package worker

// CleanupWorker — hygiene layer for notification infrastructure tables.
//
// Responsibilities (daily, non-blocking):
//   1. Delete notification_delivery_log rows older than the retention window
//      (default 30 days). Logs are observability artifacts; deletion does NOT
//      affect notification records or delivery semantics.
//   2. Delete expired push_retry_queue rows (expires_at < NOW()) as a safety
//      net for any entries that the PushRetryWorker missed (e.g. crash before
//      delete). Active retry entries are never touched.
//
// Lifecycle: Start() / Stop() / IsRunning() satisfy serverboot.Worker.

import (
	"context"
	"sync"
	"time"

	dbpkg "github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

const (
	// DefaultCleanupRetention is the default log retention period.
	DefaultCleanupRetention = 30 * 24 * time.Hour

	// DefaultCleanupInterval is how often cleanup runs.
	DefaultCleanupInterval = 24 * time.Hour
)

// CleanupWorker handles periodic cleanup of old notification delivery logs
// and expired push retry queue entries.
type CleanupWorker struct {
	db  *dbpkg.DB
	log *zap.Logger

	retention       time.Duration
	cleanupInterval time.Duration

	mu          sync.RWMutex
	running     bool
	shutdownCtx context.Context
	cancelFn    context.CancelFunc
	wg          sync.WaitGroup
}

// NewCleanupWorker creates a new CleanupWorker with default retention (30d) and
// interval (24h).
func NewCleanupWorker(db *dbpkg.DB, log *zap.Logger) *CleanupWorker {
	if log == nil {
		log = zap.NewNop()
	}
	return &CleanupWorker{
		db:              db,
		log:             log,
		retention:       DefaultCleanupRetention,
		cleanupInterval: DefaultCleanupInterval,
	}
}

// SetRetention overrides the log retention period. Call before Start.
func (w *CleanupWorker) SetRetention(retention time.Duration) {
	w.retention = retention
}

// SetCleanupInterval overrides how often cleanup runs. Call before Start.
func (w *CleanupWorker) SetCleanupInterval(interval time.Duration) {
	w.cleanupInterval = interval
}

// Start begins the cleanup worker loop in the background.
// Idempotent: calling Start on an already-running worker is a no-op.
func (w *CleanupWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("CleanupWorker already running")
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.wg.Add(1)
	go w.run()

	w.log.Info("CleanupWorker started",
		zap.Duration("retention", w.retention),
		zap.Duration("cleanup_interval", w.cleanupInterval),
	)
}

// Stop signals the worker to stop and waits for the current cycle to finish.
func (w *CleanupWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.cancelFn()
	w.wg.Wait()
	w.running = false

	w.log.Info("CleanupWorker stopped")
}

// IsRunning returns true if the worker loop is active.
func (w *CleanupWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

func (w *CleanupWorker) run() {
	defer w.wg.Done()

	// Run once immediately on startup.
	w.runCleanup(w.shutdownCtx)

	ticker := time.NewTicker(w.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.shutdownCtx.Done():
			return
		case <-ticker.C:
			w.runCleanup(w.shutdownCtx)
		}
	}
}

// runCleanup executes all cleanup operations and logs the summary.
func (w *CleanupWorker) runCleanup(ctx context.Context) {
	start := time.Now()

	cleanupCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	logCount := w.cleanupDeliveryLogs(cleanupCtx)
	retryCount := w.cleanupPushRetryQueue(cleanupCtx)

	w.log.Info("CleanupWorker cycle complete",
		zap.Int64("delivery_logs_deleted", logCount),
		zap.Int64("retry_entries_expired", retryCount),
		zap.Duration("elapsed", time.Since(start)),
	)
}

// cleanupDeliveryLogs removes notification_delivery_log rows older than retention.
// Returns number of rows deleted (negative on error).
func (w *CleanupWorker) cleanupDeliveryLogs(ctx context.Context) int64 {
	cutoff := time.Now().Add(-w.retention)

	result, err := w.db.Pool().Exec(ctx,
		`DELETE FROM notification_delivery_log WHERE created_at < $1`,
		cutoff,
	)
	if err != nil {
		w.log.Error("CleanupWorker: failed to delete delivery logs",
			zap.Time("cutoff", cutoff),
			zap.Error(err),
		)
		return -1
	}

	return result.RowsAffected()
}

// cleanupPushRetryQueue removes expired push_retry_queue entries.
// Safety net: PushRetryWorker removes entries on terminal outcome; this
// catches any rows that were missed (server crash, etc.).
// Active rows (expires_at > NOW()) are never touched.
func (w *CleanupWorker) cleanupPushRetryQueue(ctx context.Context) int64 {
	result, err := w.db.Pool().Exec(ctx,
		`DELETE FROM push_retry_queue WHERE expires_at < NOW()`,
	)
	if err != nil {
		w.log.Error("CleanupWorker: failed to delete expired retry entries", zap.Error(err))
		return -1
	}

	return result.RowsAffected()
}

// RunFullCleanup runs all cleanup operations synchronously.
// Useful for testing or ad-hoc ops runs. Returns row counts per table.
func (w *CleanupWorker) RunFullCleanup(ctx context.Context) map[string]int64 {
	return map[string]int64{
		"delivery_logs":    w.cleanupDeliveryLogs(ctx),
		"push_retry_queue": w.cleanupPushRetryQueue(ctx),
	}
}

// CleanupOldLogs satisfies the DeliveryLogCleaner interface used in tests.
func (w *CleanupWorker) CleanupOldLogs(ctx context.Context, retention time.Duration) (int64, error) {
	orig := w.retention
	w.retention = retention
	defer func() { w.retention = orig }()

	count := w.cleanupDeliveryLogs(ctx)
	if count < 0 {
		return 0, nil // error already logged
	}
	return count, nil
}


