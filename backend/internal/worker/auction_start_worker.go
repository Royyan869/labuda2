package worker

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/auction/application"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

const (
	// DefaultAuctionStartPollInterval is how often the worker checks for auctions ready to start
	DefaultAuctionStartPollInterval = 30 * time.Second

	// DefaultAuctionStartBatchSize is max auctions to start per poll
	DefaultAuctionStartBatchSize = 50
)

// AuctionStartWorker detects and activates scheduled auctions whose start time has arrived.
// It processes each auction atomically within a transaction using SKIP LOCKED
// for concurrent-safe batch processing.
type AuctionStartWorker struct {
	db             Transactor
	auctionService *application.AuctionService
	log            *zap.Logger
	pollInterval   time.Duration
	batchSize      int

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

	// Context for shutdown
	shutdownCtx context.Context
	cancelFn    context.CancelFunc
}

// AuctionStartWorkerConfig holds worker configuration
type AuctionStartWorkerConfig struct {
	PollInterval time.Duration // How often to check for auctions ready to start
	BatchSize    int           // Max auctions to process per poll
}

// DefaultAuctionStartWorkerConfig returns default configuration
func DefaultAuctionStartWorkerConfig() AuctionStartWorkerConfig {
	return AuctionStartWorkerConfig{
		PollInterval: DefaultAuctionStartPollInterval,
		BatchSize:    DefaultAuctionStartBatchSize,
	}
}

// NewAuctionStartWorker creates a new auction start worker
func NewAuctionStartWorker(
	db Transactor,
	auctionService *application.AuctionService,
	log *zap.Logger,
	cfg AuctionStartWorkerConfig,
) *AuctionStartWorker {
	if log == nil {
		log = zap.NewNop()
	}

	if cfg.PollInterval == 0 {
		cfg.PollInterval = DefaultAuctionStartPollInterval
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = DefaultAuctionStartBatchSize
	}

	return &AuctionStartWorker{
		db:             db,
		auctionService: auctionService,
		log:            log,
		pollInterval:   cfg.PollInterval,
		batchSize:      cfg.BatchSize,
		stopCh:         make(chan struct{}),
	}
}

// Start begins detecting scheduled auctions ready to start in the background
func (w *AuctionStartWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("Auction start worker already running")
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.stopCh = make(chan struct{}) // Always create a new stopCh

	w.wg.Add(1)
	go w.run()

	w.log.Info("Auction start worker started",
		zap.Duration("poll_interval", w.pollInterval),
		zap.Int("batch_size", w.batchSize),
	)
}

// Stop gracefully shuts down the worker
func (w *AuctionStartWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.log.Info("Stopping auction start worker...")

	w.cancelFn()
	close(w.stopCh)

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.log.Info("Auction start worker stopped gracefully")
	case <-time.After(10 * time.Second):
		w.log.Warn("Auction start worker shutdown timeout")
	}

	w.running = false
}

// IsRunning returns true if the worker is currently running
func (w *AuctionStartWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// run is the main worker loop
func (w *AuctionStartWorker) run() {
	defer w.wg.Done()

	w.checkStartingAuctions()

	for {
		select {
		case <-w.shutdownCtx.Done():
			w.log.Info("Worker shutdown requested")
			return

		case <-time.After(w.pollInterval):
			w.checkStartingAuctions()

		case <-w.stopCh:
			return
		}
	}
}

// checkStartingAuctions finds and activates scheduled auctions ready to start.
//
// PATTERN:
// Phase 1: Short transaction to fetch IDs with SKIP LOCKED
// Phase 2: Process each auction in its own transaction
func (w *AuctionStartWorker) checkStartingAuctions() {
	ctx := context.Background()

	// Phase 1: Short tx just to fetch IDs with SKIP LOCKED
	// Locks are released immediately after this transaction commits
	auctionIDs, err := w.findStartingAuctionIDs(ctx, w.batchSize)
	if err != nil {
		w.log.Error("Failed to find starting auctions", zap.Error(err))
		return
	}

	if len(auctionIDs) == 0 {
		return
	}

	w.log.Info("Found auctions ready to start", zap.Int("count", len(auctionIDs)))

	// Phase 2: Process each auction in its own transaction
	// Failure is isolated per entity - no cascade rollback
	for _, auctionID := range auctionIDs {
		if err := w.startAuction(ctx, auctionID); err != nil {
			w.log.Error("Failed to start auction",
				zap.String("auction_id", auctionID.String()),
				zap.Error(err),
			)
		}
	}
}

// findStartingAuctionIDs returns IDs of scheduled auctions ready to start.
// Phase 1: Short transaction to fetch IDs with SKIP LOCKED.
func (w *AuctionStartWorker) findStartingAuctionIDs(
	ctx context.Context,
	limit int,
) ([]uuid.UUID, error) {
	var auctionIDs []uuid.UUID

	// Short transaction to fetch and lock IDs
	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		query := `
			SELECT id
			FROM auctions
			WHERE status = 'scheduled'
			  AND start_at <= NOW()
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		`

		rows, err := tx.Query(ctx, query, limit)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				return err
			}
			auctionIDs = append(auctionIDs, id)
		}

		return rows.Err()
	})

	return auctionIDs, err
}

// ActivateAuctionInput is a minimal input struct for activation
// Note: Activation is done directly in the worker for simplicity

// startAuction starts a single scheduled auction within its own transaction.
// Phase 2: Each auction is processed in a separate transaction.
//
// BOUNDARY NORMALIZATION (PHASE 1D):
// - Auction status is the authoritative source of truth
// - Time boundary (start_at) is the trigger, but status determines operability
// - Service's ActivateScheduledAuction() enforces state transition + market authority rules
//
// MARKET AUTHORITY ENFORCEMENT:
// - Re-verifies seller subscription at activation time
// - Cancels auction if seller subscription expired
// - Only activates if seller still has active subscription
func (w *AuctionStartWorker) startAuction(
	ctx context.Context,
	auctionID uuid.UUID,
) error {
	return w.db.WithTx(ctx, func(tx db.Tx) error {
		// Use service method which includes market authority check
		return w.auctionService.ActivateScheduledAuction(ctx, tx, application.ActivateScheduledAuctionInput{
			AuctionID: auctionID,
		})
	})
}


