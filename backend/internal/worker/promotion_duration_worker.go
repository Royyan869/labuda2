package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pricing/promotion/application"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// PromotionExpirationWorker is a periodic worker that handles expiration checks.
//
// This worker runs every hour (configurable) and:
// - Marks ownerships as expired if their validity window has passed
// - Marks instances as expired if their duration is fully consumed (based on wall-clock)
//
// IMPORTANT: With wall-clock time tracking, duration is calculated dynamically
// at read time. This worker only handles status updates for expiration.
//
// IMPORTANT: This is an EXPIRATION worker, NOT a consumption worker.
// The safety worker (PromotionSafetyWorker) handles non-operable targets.
// This worker ONLY handles validity window expiration.
type PromotionExpirationWorker struct {
	promotionService *application.PromotionService
	db               *db.DB
	log              *zap.Logger
	pollInterval     time.Duration

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

	// Context for shutdown
	shutdownCtx context.Context
	cancelFn    context.CancelFunc

	// Worker identifier for logging
	workerID string
}

// PromotionExpirationWorkerConfig holds worker configuration
type PromotionExpirationWorkerConfig struct {
	PollInterval time.Duration // How often to check for expiration (default: 1 hour)
}

// DefaultPromotionExpirationWorkerConfig returns default configuration
func DefaultPromotionExpirationWorkerConfig() PromotionExpirationWorkerConfig {
	return PromotionExpirationWorkerConfig{
		PollInterval: 1 * time.Hour,
	}
}

// NewPromotionExpirationWorker creates a new promotion expiration worker
func NewPromotionExpirationWorker(
	promotionService *application.PromotionService,
	dbConn *db.DB,
	log *zap.Logger,
	cfg PromotionExpirationWorkerConfig,
) *PromotionExpirationWorker {
	if log == nil {
		log = zap.NewNop()
	}

	if cfg.PollInterval == 0 {
		cfg.PollInterval = DefaultPromotionExpirationWorkerConfig().PollInterval
	}

	workerID := fmt.Sprintf("promotion-expiration-worker-%s", uuid.New().String()[:8])

	return &PromotionExpirationWorker{
		promotionService: promotionService,
		db:               dbConn,
		log:              log,
		pollInterval:     cfg.PollInterval,
		stopCh:           make(chan struct{}),
		workerID:         workerID,
	}
}

// Start begins processing expiration checks in the background
func (w *PromotionExpirationWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("PromotionExpirationWorker already running",
			zap.String("worker_id", w.workerID),
		)
		return
	}

	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.running = true

	w.wg.Add(1)
	go w.run()

	w.log.Info("PromotionExpirationWorker started",
		zap.String("worker_id", w.workerID),
		zap.Duration("poll_interval", w.pollInterval),
	)
}

// Stop gracefully shuts down the worker
func (w *PromotionExpirationWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.log.Info("PromotionExpirationWorker stopping",
		zap.String("worker_id", w.workerID),
	)

	// Signal shutdown
	w.cancelFn()
	close(w.stopCh)

	// Wait for run loop to exit
	w.wg.Wait()

	w.running = false

	w.log.Info("PromotionExpirationWorker stopped",
		zap.String("worker_id", w.workerID),
	)
}

// IsRunning returns true if the worker is currently running
func (w *PromotionExpirationWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// run is the main worker loop
func (w *PromotionExpirationWorker) run() {
	defer w.wg.Done()

	// Create ticker for periodic expiration checks
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	// Do initial expiration check on startup
	w.processExpiration()

	for {
		select {
		case <-w.shutdownCtx.Done():
			w.log.Debug("PromotionExpirationWorker shutdown requested",
				zap.String("worker_id", w.workerID),
			)
			return

		case <-ticker.C:
			w.processExpiration()

		case <-w.stopCh:
			w.log.Debug("PromotionExpirationWorker stop signal received",
				zap.String("worker_id", w.workerID),
			)
			return
		}
	}
}

// processExpiration executes one round of expiration checks
func (w *PromotionExpirationWorker) processExpiration() {
	w.log.Debug("PromotionExpirationWorker starting expiration check",
		zap.String("worker_id", w.workerID),
	)

	startTime := time.Now()

	// Create transaction for expiration processing
	tx, err := w.db.BeginTx(w.shutdownCtx)
	if err != nil {
		w.log.Error("PromotionExpirationWorker failed to begin transaction",
			zap.String("worker_id", w.workerID),
			zap.Error(err),
		)
		return
	}
	defer tx.Rollback(w.shutdownCtx)

	// Phase 1: Process expired ownerships (validity window expired)
	expiredCount, err := w.promotionService.ProcessExpiredOwnerships(w.shutdownCtx, tx, 100)
	if err != nil {
		w.log.Error("PromotionExpirationWorker expiration check failed",
			zap.String("worker_id", w.workerID),
			zap.Error(err),
		)
		return
	}

	// Phase 2: Process duration-exhausted instances (consumed >= remaining)
	exhaustedCount, err := w.promotionService.ProcessDurationExhaustedInstances(w.shutdownCtx, tx, 100)
	if err != nil {
		w.log.Error("PromotionExpirationWorker duration exhaustion check failed",
			zap.String("worker_id", w.workerID),
			zap.Error(err),
		)
		return
	}

	// Commit transaction
	if err := tx.Commit(w.shutdownCtx); err != nil {
		w.log.Error("PromotionExpirationWorker failed to commit transaction",
			zap.String("worker_id", w.workerID),
			zap.Error(err),
		)
		return
	}

	duration := time.Since(startTime)

	w.log.Info("PromotionExpirationWorker expiration check completed",
		zap.String("worker_id", w.workerID),
		zap.Duration("duration", duration),
		zap.Int("expired_ownership_count", expiredCount),
		zap.Int("exhausted_instance_count", exhaustedCount),
	)
}


