package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	dbpkg "github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

const (
	// DefaultArchivalPollInterval is how often the worker checks for events to archive
	DefaultArchivalPollInterval = 5 * time.Minute

	// DefaultArchivalBatchSize is max events to archive per batch
	DefaultArchivalBatchSize = 500

	// DefaultRetentionDays is default days before archiving succeeded events
	DefaultRetentionDays = 30
)

// =============================================================================
// OUTBOX ARCHIVAL WORKER
// =============================================================================

// OutboxArchivalWorker archives old succeeded outbox events to outbox_archive.
//
// SCOPE: Safe retention & archival of succeeded outbox events only.
//
// PROCESSING FLOW (SINGLE TRANSACTION - FULLY ATOMIC):
// BEGIN
//   SELECT id FROM outbox
//   WHERE status = 'succeeded'
//   AND created_at < cutoff
//   FOR UPDATE SKIP LOCKED
//   LIMIT batch_size
//
//   DELETE FROM outbox
//   WHERE id IN (selected_ids)
//   RETURNING *
//
//   INSERT INTO outbox_archive (...)
// COMMIT
//
// SAFETY:
// - Only archives events with status='succeeded'
// - Does NOT archive pending/failed/dead_letter events
// - Uses FOR UPDATE SKIP LOCKED for concurrent worker safety
// - Fully atomic: if insert fails, delete is rolled back
// - Single transaction: no intermediate commit
// - Separate worker name: "outbox_archival_worker"
// - Does NOT modify financial logic
// - Does NOT interfere with main OutboxWorker
type OutboxArchivalWorker struct {
	db            Transactor
	archiveRepo   *repository.OutboxArchiveRepository
	log           *zap.Logger
	pollInterval  time.Duration
	batchSize     int
	retentionDays int

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

	// Context for shutdown
	shutdownCtx context.Context
	cancelFn    context.CancelFunc

	// Worker identifier for logging
	workerID string

	// Metrics collector (optional, set via SetMetricsCollector)
	metricsCollector MetricsRecorder

	// Internal metrics
	archivedCount      int64
	totalArchivedCount int64
}

// MetricsRecorder is an interface for recording metrics.
// This allows the worker to work with or without a metrics collector.
type MetricsRecorder interface {
	RecordOutboxArchived(count int)
	RecordOutboxArchiveBatchDuration(durationMs float64)
}

// OutboxArchivalWorkerConfig holds worker configuration
type OutboxArchivalWorkerConfig struct {
	PollInterval  time.Duration // How often to check for events to archive
	BatchSize     int           // Max events to archive per batch
	RetentionDays int           // Days before archiving succeeded events
}

// DefaultOutboxArchivalWorkerConfig returns default configuration
func DefaultOutboxArchivalWorkerConfig() OutboxArchivalWorkerConfig {
	return OutboxArchivalWorkerConfig{
		PollInterval:  DefaultArchivalPollInterval,
		BatchSize:     DefaultArchivalBatchSize,
		RetentionDays: DefaultRetentionDays,
	}
}

// NewOutboxArchivalWorker creates a new outbox archival worker
func NewOutboxArchivalWorker(
	db Transactor,
	database *dbpkg.DB,
	log *zap.Logger,
	cfg OutboxArchivalWorkerConfig,
) *OutboxArchivalWorker {
	if log == nil {
		log = zap.NewNop()
	}

	if cfg.PollInterval == 0 {
		cfg.PollInterval = DefaultArchivalPollInterval
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = DefaultArchivalBatchSize
	}
	if cfg.RetentionDays == 0 {
		cfg.RetentionDays = DefaultRetentionDays
	}

	workerID := fmt.Sprintf("outbox_archival_worker-%s", uuid.New().String()[:8])

	return &OutboxArchivalWorker{
		db:            db,
		archiveRepo:   repository.NewOutboxArchiveRepository(database),
		log:           log,
		pollInterval:  cfg.PollInterval,
		batchSize:     cfg.BatchSize,
		retentionDays: cfg.RetentionDays,
		stopCh:        make(chan struct{}),
		workerID:      workerID,
	}
}

// NewOutboxArchivalWorkerFromConfig creates a worker from application config
func NewOutboxArchivalWorkerFromConfig(
	db Transactor,
	database *dbpkg.DB,
	log *zap.Logger,
	appCfg *config.Config,
) *OutboxArchivalWorker {
	return NewOutboxArchivalWorker(db, database, log, OutboxArchivalWorkerConfig{
		PollInterval:  DefaultArchivalPollInterval,
		BatchSize:     appCfg.Outbox.ArchiveBatchSize,
		RetentionDays: appCfg.Outbox.RetentionDays,
	})
}

// Start begins processing archival in the background
func (w *OutboxArchivalWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("Outbox archival worker already running",
			zap.String("worker_id", w.workerID),
		)
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.stopCh = make(chan struct{})

	w.wg.Add(1)
	go w.run()

	w.log.Info("Outbox archival worker started",
		zap.Duration("poll_interval", w.pollInterval),
		zap.Int("batch_size", w.batchSize),
		zap.Int("retention_days", w.retentionDays),
		zap.String("worker_id", w.workerID),
	)
}

// Stop gracefully shuts down the worker
func (w *OutboxArchivalWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.log.Info("Stopping outbox archival worker...",
		zap.String("worker_id", w.workerID),
	)

	w.cancelFn()
	close(w.stopCh)

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.log.Info("Outbox archival worker stopped gracefully",
			zap.String("worker_id", w.workerID),
		)
	case <-time.After(10 * time.Second):
		w.log.Warn("Outbox archival worker shutdown timeout",
			zap.String("worker_id", w.workerID),
		)
	}

	w.running = false
}

// IsRunning returns true if the worker is currently running
func (w *OutboxArchivalWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// run is the main worker loop
func (w *OutboxArchivalWorker) run() {
	defer w.wg.Done()

	// Process immediately on start
	w.processArchivalBatch()

	for {
		select {
		case <-w.shutdownCtx.Done():
			w.log.Info("Archival worker shutdown requested",
				zap.String("worker_id", w.workerID),
			)
			return

		case <-time.After(w.pollInterval):
			w.processArchivalBatch()

		case <-w.stopCh:
			return
		}
	}
}

// processArchivalBatch processes a batch of old succeeded events.
//
// TRANSACTION MODEL: SINGLE TRANSACTION - FULLY ATOMIC
// - Fetch IDs with locks
// - Delete ... RETURNING
// - Insert into archive
// All in ONE db.WithTx call - no intermediate commits
func (w *OutboxArchivalWorker) processArchivalBatch() {
	ctx := context.Background()
	startTime := time.Now()

	// Single transaction: fetch, delete, and insert all atomically
	var archivedCount int
	err := w.db.WithTx(ctx, func(tx dbpkg.Tx) error {
		var err error
		archivedCount, err = w.archiveRepo.ArchiveBatch(ctx, tx, w.retentionDays, w.batchSize)
		return err
	})

	if err != nil {
		w.log.Error("Failed to archive events",
			zap.Error(err),
			zap.String("worker_id", w.workerID),
		)
		return
	}

	if archivedCount == 0 {
		return
	}

	duration := time.Since(startTime)

	// Update internal metrics
	w.archivedCount = int64(archivedCount)
	w.totalArchivedCount += int64(archivedCount)

	// Record Prometheus metrics if collector is set
	if w.metricsCollector != nil {
		w.metricsCollector.RecordOutboxArchived(archivedCount)
		w.metricsCollector.RecordOutboxArchiveBatchDuration(float64(duration.Milliseconds()))
	}

	// Log batch stats
	w.log.Info("Outbox archival batch completed",
		zap.Int("archived_count", archivedCount),
		zap.Duration("duration_ms", duration),
		zap.Int("retention_days", w.retentionDays),
		zap.String("worker_id", w.workerID),
	)
}

// SetMetricsCollector sets the metrics collector for this worker.
// This is optional and allows the worker to record metrics to Prometheus.
func (w *OutboxArchivalWorker) SetMetricsCollector(collector MetricsRecorder) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.metricsCollector = collector
}

// GetArchivedCount returns the count of events archived in the last batch
func (w *OutboxArchivalWorker) GetArchivedCount() int64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.archivedCount
}

// GetTotalArchivedCount returns the total count of events archived since worker start
func (w *OutboxArchivalWorker) GetTotalArchivedCount() int64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.totalArchivedCount
}

// ManualProcess triggers immediate archival processing.
// Useful for testing or manual intervention.
func (w *OutboxArchivalWorker) ManualProcess(ctx context.Context) error {
	w.processArchivalBatch()
	return nil
}


