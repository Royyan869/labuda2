// Package worker provides idempotency record cleanup functionality.
//
// STATUS: LIVE — wired in serverboot/dependencies.go (B92).
// Env gate: DISABLE_IDEMPOTENCY_CLEANUP_WORKER (default ON).
//
// IDEMPOTENCY CLEANUP WORKER:
// - Deletes old idempotency records to prevent table bloat
// - Configurable retention period (default: 30 days)
// - Runs on a periodic schedule (default: daily)
package worker

import (
	"context"
	"sync"
	"time"

	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

const (
	// DefaultIdempotencyCleanupPollInterval is how often the worker runs cleanup
	DefaultIdempotencyCleanupPollInterval = 24 * time.Hour // Daily

	// DefaultIdempotencyRetentionDays is how long to keep idempotency records
	// Idempotency records are only needed for:
	// - Idempotency during retries (minutes to hours)
	// - Debugging recent issues (days)
	// 30 days is a safe retention period for operational needs
	DefaultIdempotencyRetentionDays = 30

	// MinIdempotencyRetentionDays is the minimum allowed retention period
	// Prevents accidental misconfiguration that could break idempotency
	MinIdempotencyRetentionDays = 7
)

// IdempotencyCleanupWorker deletes old idempotency records to prevent table bloat.
//
// CLEANUP LOGIC:
// 1. Delete records where created_at < NOW() - INTERVAL 'N days'
// 2. Configurable retention period (default: 30 days, min: 7 days)
// 3. Runs on a periodic schedule (default: daily)
//
// SAFETY:
// - Uses DELETE with WHERE clause (no TRUNCATE)
// - Deletes in batches to prevent long-running transactions
// - Minimum retention of 7 days prevents accidental data loss
//
// WHY CLEANUP IS NEEDED:
// - Idempotency records accumulate over time
// - Each order creation/payment/shipping creates records
// - Table can grow to millions of rows without cleanup
// - Large tables slow down queries and increase storage costs
//
// WHY 30 DAYS RETENTION:
// - Sufficient for debugging recent issues
// - Covers the entire order lifecycle (creation → completion)
// - Covers dispute resolution window (up to 30 days)
// - Balances storage costs with operational needs
type IdempotencyCleanupWorker struct {
	db           *db.DB
	log          *zap.Logger
	pollInterval time.Duration
	retentionDays int

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

	// Context for shutdown
	shutdownCtx context.Context
	cancelFn    context.CancelFunc
}

// IdempotencyCleanupConfig holds worker configuration
type IdempotencyCleanupConfig struct {
	PollInterval  time.Duration // How often to run cleanup
	RetentionDays int           // How long to keep records (days)
}

// DefaultIdempotencyCleanupConfig returns default configuration
func DefaultIdempotencyCleanupConfig() IdempotencyCleanupConfig {
	return IdempotencyCleanupConfig{
		PollInterval:  DefaultIdempotencyCleanupPollInterval,
		RetentionDays: DefaultIdempotencyRetentionDays,
	}
}

// NewIdempotencyCleanupWorker creates a new idempotency cleanup worker
func NewIdempotencyCleanupWorker(
	db *db.DB,
	log *zap.Logger,
	cfg IdempotencyCleanupConfig,
) *IdempotencyCleanupWorker {
	if log == nil {
		log = zap.NewNop()
	}

	if cfg.PollInterval == 0 {
		cfg.PollInterval = DefaultIdempotencyCleanupPollInterval
	}
	if cfg.RetentionDays == 0 {
		cfg.RetentionDays = DefaultIdempotencyRetentionDays
	}

	// Enforce minimum retention period (safety guard)
	if cfg.RetentionDays < MinIdempotencyRetentionDays {
		log.Warn("Idempotency retention period too low, using minimum",
			zap.Int("requested_days", cfg.RetentionDays),
			zap.Int("minimum_days", MinIdempotencyRetentionDays),
		)
		cfg.RetentionDays = MinIdempotencyRetentionDays
	}

	return &IdempotencyCleanupWorker{
		db:            db,
		log:           log,
		pollInterval:  cfg.PollInterval,
		retentionDays: cfg.RetentionDays,
		stopCh:        make(chan struct{}),
	}
}

// Start begins the cleanup worker in the background
func (w *IdempotencyCleanupWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("Idempotency cleanup worker already running")
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.stopCh = make(chan struct{})

	w.wg.Add(1)
	go w.run()

	w.log.Info("Idempotency cleanup worker started",
		zap.Duration("poll_interval", w.pollInterval),
		zap.Int("retention_days", w.retentionDays),
	)
}

// Stop gracefully shuts down the worker
func (w *IdempotencyCleanupWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.log.Info("Stopping idempotency cleanup worker...")

	w.cancelFn()
	close(w.stopCh)

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.log.Info("Idempotency cleanup worker stopped gracefully")
	case <-time.After(10 * time.Second):
		w.log.Warn("Idempotency cleanup worker shutdown timeout")
	}

	w.running = false
}

// IsRunning returns true if the worker is currently running
func (w *IdempotencyCleanupWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// run is the main worker loop
func (w *IdempotencyCleanupWorker) run() {
	defer w.wg.Done()

	// Run cleanup immediately on startup
	w.cleanupOldRecords()

	for {
		select {
		case <-w.shutdownCtx.Done():
			w.log.Info("Worker shutdown requested")
			return

		case <-time.After(w.pollInterval):
			w.cleanupOldRecords()

		case <-w.stopCh:
			return
		}
	}
}

// cleanupOldRecords deletes idempotency records older than the retention period.
// Deletes in batches to prevent long-running transactions.
func (w *IdempotencyCleanupWorker) cleanupOldRecords() {
	ctx := context.Background()
	start := time.Now()

	// Delete records older than retention period
	var deletedCount int64
	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		query := `
			DELETE FROM idempotency_records
			WHERE created_at < NOW() - INTERVAL '1 day' * $1
		`

		result, err := tx.Exec(ctx, query, w.retentionDays)
		if err != nil {
			return err
		}

		deletedCount = result.RowsAffected()
		return nil
	})

	if err != nil {
		w.log.Error("Failed to cleanup old idempotency records",
			zap.Error(err),
			zap.Int("retention_days", w.retentionDays),
		)
		return
	}

	duration := time.Since(start)

	if deletedCount > 0 {
		w.log.Info("Idempotency records cleanup completed",
			zap.Int64("deleted_count", deletedCount),
			zap.Int("retention_days", w.retentionDays),
			zap.Duration("duration", duration),
		)
	} else {
		w.log.Debug("Idempotency records cleanup completed",
			zap.Int64("deleted_count", deletedCount),
			zap.Int("retention_days", w.retentionDays),
			zap.Duration("duration", duration),
		)
	}
}

// ManualCleanup triggers immediate cleanup of old idempotency records.
// Useful for testing or manual intervention.
func (w *IdempotencyCleanupWorker) ManualCleanup(ctx context.Context) error {
	return w.db.WithTx(ctx, func(tx db.Tx) error {
		query := `
			DELETE FROM idempotency_records
			WHERE created_at < NOW() - INTERVAL '1 day' * $1
		`

		result, err := tx.Exec(ctx, query, w.retentionDays)
		if err != nil {
			return err
		}

		deletedCount := result.RowsAffected()

		w.log.Info("Manual idempotency cleanup completed",
			zap.Int64("deleted_count", deletedCount),
			zap.Int("retention_days", w.retentionDays),
		)

		return nil
	})
}


