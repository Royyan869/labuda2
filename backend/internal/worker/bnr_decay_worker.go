package worker

// BNRDecayWorker — forgiveness layer for buyer BNR strikes.
//
// Rule (owner-approved):
//   If a buyer has active strikes and the most-recent active strike is
//   older than 180 days, decay the OLDEST active strike (set decayed_at).
//   One decay per buyer per run. Rows are kept for audit — never deleted.
//
// Excluded rows:
//   - admin_reset = TRUE   (admin action is permanent)
//   - decayed_at IS NOT NULL (already decayed)
//
// Lifecycle: RunDecay() is the callable entry point.
// Start()/Stop() provide periodic scheduling via the standard worker
// interface. Wired in serverboot as default-ON daily worker.

import (
	"context"
	"fmt"
	"sync"
	"time"

	dbpkg "github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

const (
	// BNRDecayThreshold is the inactivity period after which strikes decay.
	BNRDecayThreshold = 180 * 24 * time.Hour

	// DefaultBNRDecayInterval is how often the worker runs when started.
	DefaultBNRDecayInterval = 24 * time.Hour
)

// BNRDecayWorker decays old BNR strikes for buyers who have not received
// a new strike in 180 days.
type BNRDecayWorker struct {
	db  dbpkg.Transactor
	log *zap.Logger

	decayInterval time.Duration

	mu          sync.RWMutex
	running     bool
	shutdownCtx context.Context
	cancelFn    context.CancelFunc
	wg          sync.WaitGroup
}

// NewBNRDecayWorker creates a decay worker. Call RunDecay() directly or
// Start() for periodic execution.
func NewBNRDecayWorker(db dbpkg.Transactor, log *zap.Logger) *BNRDecayWorker {
	if log == nil {
		log = zap.NewNop()
	}
	return &BNRDecayWorker{
		db:            db,
		log:           log,
		decayInterval: DefaultBNRDecayInterval,
	}
}

// SetDecayInterval overrides the periodic interval. Call before Start.
func (w *BNRDecayWorker) SetDecayInterval(d time.Duration) {
	w.decayInterval = d
}

// Start begins periodic decay in the background.
func (w *BNRDecayWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("BNRDecayWorker already running")
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.wg.Add(1)
	go w.run()

	w.log.Info("BNRDecayWorker started",
		zap.Duration("decay_interval", w.decayInterval),
		zap.Duration("threshold", BNRDecayThreshold),
	)
}

// Stop signals the worker to stop and waits for the current cycle to finish.
func (w *BNRDecayWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.cancelFn()
	w.wg.Wait()
	w.running = false

	w.log.Info("BNRDecayWorker stopped")
}

// IsRunning returns true if the worker loop is active.
func (w *BNRDecayWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

func (w *BNRDecayWorker) run() {
	defer w.wg.Done()

	// Run once immediately on startup.
	w.runCycle(w.shutdownCtx)

	ticker := time.NewTicker(w.decayInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.shutdownCtx.Done():
			return
		case <-ticker.C:
			w.runCycle(w.shutdownCtx)
		}
	}
}

func (w *BNRDecayWorker) runCycle(ctx context.Context) {
	start := time.Now()

	cycleCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	decayed, err := w.RunDecay(cycleCtx)
	if err != nil {
		w.log.Error("BNRDecayWorker cycle failed", zap.Error(err))
		return
	}

	w.log.Info("BNRDecayWorker cycle complete",
		zap.Int64("strikes_decayed", decayed),
		zap.Duration("elapsed", time.Since(start)),
	)
}

// RunDecay is the callable entry point. It decays at most one strike per
// eligible buyer in a single pass. Returns the number of strikes decayed.
//
// SQL logic:
//   1. Find buyers whose MAX(struck_at) among active strikes is older
//      than the threshold (180 days).
//   2. For each such buyer, UPDATE the row with the MIN(struck_at) among
//      active strikes, setting decayed_at = NOW().
//   3. One UPDATE per buyer per call (the CTE picks MIN(struck_at)).
func (w *BNRDecayWorker) RunDecay(ctx context.Context) (int64, error) {
	return w.RunDecayWithThreshold(ctx, BNRDecayThreshold)
}

// RunDecayWithThreshold allows callers (tests) to override the threshold.
func (w *BNRDecayWorker) RunDecayWithThreshold(ctx context.Context, threshold time.Duration) (int64, error) {
	cutoff := time.Now().Add(-threshold)
	var decayed int64

	err := w.db.WithTx(ctx, func(tx dbpkg.Tx) error {
		// Single statement: CTE identifies eligible buyers and their oldest
		// active strike, then UPDATE sets decayed_at on exactly those rows.
		const q = `
			WITH eligible AS (
				SELECT DISTINCT ON (buyer_id)
					id, buyer_id
				FROM buyer_bnr_strikes
				WHERE decayed_at IS NULL
				  AND admin_reset = FALSE
				GROUP BY id, buyer_id
				HAVING buyer_id IN (
					SELECT buyer_id
					FROM buyer_bnr_strikes
					WHERE decayed_at IS NULL
					  AND admin_reset = FALSE
					GROUP BY buyer_id
					HAVING MAX(struck_at) < $1
				)
				ORDER BY buyer_id, struck_at ASC
			)
			UPDATE buyer_bnr_strikes
			SET decayed_at = NOW()
			FROM eligible
			WHERE buyer_bnr_strikes.id = eligible.id
		`
		tag, err := tx.Exec(ctx, q, cutoff)
		if err != nil {
			return fmt.Errorf("bnr_decay: update: %w", err)
		}
		decayed = tag.RowsAffected()
		return nil
	})

	return decayed, err
}


