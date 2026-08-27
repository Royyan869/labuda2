// P4C3: ENABLED — periodic safety sweep for stale active promotion instances.
// Canonical entity-based finalization (Stop → Snapshot → Bake → Persist) verified in P4B.
// Disable via DISABLE_PROMOTION_SAFETY_WORKER=true.
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

// PromotionSafetyWorker is a periodic worker that sweeps active promotions
// and stops those whose targets have become non-operable.
//
// This is a SAFETY NET for:
// - Missed events (event-driven auto-stop failed)
// - Stale active instances
// - Data inconsistencies
//
// The PRIMARY mechanism for auto-stop is event-driven via outbox events.
// This worker ensures that even if events are missed, promotions are stopped.
type PromotionSafetyWorker struct {
	operabilityChecker    application.OperabilityRecommendationSource
	recommendationApplier application.OperabilityRecommendationApplier
	txManager             db.Transactor
	log                   *zap.Logger
	pollInterval          time.Duration
	batchSize             int

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

// PromotionSafetyWorkerConfig holds worker configuration
type PromotionSafetyWorkerConfig struct {
	PollInterval time.Duration // How often to sweep (default: 5 minutes)
	BatchSize    int           // Max instances to process per sweep (default: 100)
}

// DefaultPromotionSafetyWorkerConfig returns default configuration
func DefaultPromotionSafetyWorkerConfig() PromotionSafetyWorkerConfig {
	return PromotionSafetyWorkerConfig{
		PollInterval: 5 * time.Minute,
		BatchSize:    100,
	}
}

// NewPromotionSafetyWorker creates a new promotion safety worker
func NewPromotionSafetyWorker(
	operabilityChecker application.OperabilityRecommendationSource,
	recommendationApplier application.OperabilityRecommendationApplier,
	txManager db.Transactor,
	log *zap.Logger,
	cfg PromotionSafetyWorkerConfig,
) *PromotionSafetyWorker {
	if log == nil {
		log = zap.NewNop()
	}

	if cfg.PollInterval == 0 {
		cfg.PollInterval = DefaultPromotionSafetyWorkerConfig().PollInterval
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = DefaultPromotionSafetyWorkerConfig().BatchSize
	}

	workerID := fmt.Sprintf("promotion-safety-worker-%s", uuid.New().String()[:8])

	return &PromotionSafetyWorker{
		operabilityChecker:    operabilityChecker,
		recommendationApplier: recommendationApplier,
		txManager:             txManager,
		log:                   log,
		pollInterval:          cfg.PollInterval,
		batchSize:             cfg.BatchSize,
		stopCh:                make(chan struct{}),
		workerID:              workerID,
	}
}

// Start begins processing promotion safety sweeps in the background
func (w *PromotionSafetyWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("PromotionSafetyWorker already running",
			zap.String("worker_id", w.workerID),
		)
		return
	}

	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.running = true

	w.wg.Add(1)
	go w.run()

	w.log.Info("PromotionSafetyWorker started",
		zap.String("worker_id", w.workerID),
		zap.Duration("poll_interval", w.pollInterval),
		zap.Int("batch_size", w.batchSize),
	)
}

// Stop gracefully shuts down the worker
func (w *PromotionSafetyWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.log.Info("PromotionSafetyWorker stopping",
		zap.String("worker_id", w.workerID),
	)

	// Signal shutdown
	w.cancelFn()
	close(w.stopCh)

	// Wait for run loop to exit
	w.wg.Wait()

	w.running = false

	w.log.Info("PromotionSafetyWorker stopped",
		zap.String("worker_id", w.workerID),
	)
}

// IsRunning returns true if the worker is currently running
func (w *PromotionSafetyWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// run is the main worker loop
func (w *PromotionSafetyWorker) run() {
	defer w.wg.Done()

	// Create ticker for periodic sweeps
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	// Do immediate sweep on startup
	w.processSweep()

	for {
		select {
		case <-w.shutdownCtx.Done():
			w.log.Debug("PromotionSafetyWorker shutdown requested",
				zap.String("worker_id", w.workerID),
			)
			return

		case <-ticker.C:
			w.processSweep()

		case <-w.stopCh:
			w.log.Debug("PromotionSafetyWorker stop signal received",
				zap.String("worker_id", w.workerID),
			)
			return
		}
	}
}

// processSweep executes one full sweep cycle:
// Phase 1: Active instances → pause (reversible) or stop (permanent)
// Phase 2: Paused instances → resume (operable again) or stop (permanent detected)
func (w *PromotionSafetyWorker) processSweep() {
	w.log.Debug("PromotionSafetyWorker starting sweep",
		zap.String("worker_id", w.workerID),
	)

	startTime := time.Now()

	// Phase 1: Sweep active instances (pause or stop)
	activeRecommendations, err := w.operabilityChecker.SweepInactivePromotions(
		w.shutdownCtx,
		w.batchSize,
	)
	if err != nil {
		w.log.Error("PromotionSafetyWorker active sweep failed",
			zap.String("worker_id", w.workerID),
			zap.Duration("duration", time.Since(startTime)),
			zap.Error(err),
		)
		return
	}
	activePauseCount, _, activeStopCount, activeReasons := w.applyRecommendations(activeRecommendations)
	activeActionCount := activePauseCount + activeStopCount

	// Phase 2: Sweep paused instances (resume or stop)
	pausedRecommendations, err := w.operabilityChecker.SweepPausedPromotions(
		w.shutdownCtx,
		w.batchSize,
	)
	if err != nil {
		w.log.Error("PromotionSafetyWorker paused sweep failed",
			zap.String("worker_id", w.workerID),
			zap.Duration("duration", time.Since(startTime)),
			zap.Int("active_actions", activeActionCount),
			zap.Error(err),
		)
		return
	}
	_, resumedCount, pausedStoppedCount, pausedReasons := w.applyRecommendations(pausedRecommendations)

	duration := time.Since(startTime)
	allReasons := append(activeReasons, pausedReasons...)

	w.log.Info("PromotionSafetyWorker sweep completed",
		zap.String("worker_id", w.workerID),
		zap.Duration("duration", duration),
		zap.Int("active_actions", activeActionCount),
		zap.Int("resumed_count", resumedCount),
		zap.Int("paused_stopped_count", pausedStoppedCount),
		zap.Int("batch_size", w.batchSize),
		zap.Strings("reasons", allReasons),
	)
}

// applyRecommendations executes actionable recommendations through the
// PromotionService command surface.
func (w *PromotionSafetyWorker) applyRecommendations(
	recommendations []application.OperabilityRecommendation,
) (int, int, int, []string) {
	pauseCount := 0
	resumeCount := 0
	stopCount := 0
	var reasons []string

	for _, recommendation := range recommendations {
		if !recommendation.HasAction() {
			if recommendation.Reason != "" {
				reasons = append(reasons, fmt.Sprintf("no action for %s: %s", recommendation.InstanceID, recommendation.Reason))
			}
			continue
		}

		err := w.txManager.WithTx(w.shutdownCtx, func(tx db.Tx) error {
			return w.recommendationApplier.ApplyOperabilityRecommendation(w.shutdownCtx, tx, recommendation)
		})
		if err != nil {
			reasons = append(reasons, fmt.Sprintf("failed to apply %s for %s: %v", recommendation.Action, recommendation.InstanceID, err))
			continue
		}

		switch recommendation.Action {
		case application.OperabilityRecommendationPause, application.OperabilityRecommendationStop:
			if recommendation.Action == application.OperabilityRecommendationStop {
				stopCount++
				reasons = append(reasons, fmt.Sprintf("stopped %s: %s", recommendation.InstanceID, recommendation.Reason))
			} else {
				pauseCount++
				reasons = append(reasons, fmt.Sprintf("paused %s: %s", recommendation.InstanceID, recommendation.Reason))
			}
		case application.OperabilityRecommendationResume:
			resumeCount++
			reasons = append(reasons, fmt.Sprintf("resumed %s", recommendation.InstanceID))
		}
	}

	return pauseCount, resumeCount, stopCount, reasons
}


