// ⚠️ STANDBY WORKER
// DO NOT ENABLE WITHOUT BUSINESS VALIDATION
// This worker is intentionally disabled pending business validation of system monitoring.
// Worker is initialized but not started in dependencies_core.go.
//
// To enable: Remove this comment and uncomment .Start() call in dependencies_core.go
package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/labuda/backend/internal/monitoring"
)

const (
	// DefaultSystemMonitoringInterval is how often the worker runs monitoring checks
	DefaultSystemMonitoringInterval = 5 * time.Minute
)

// SystemMonitoringWorker performs periodic production monitoring checks.
// All checks are READ-ONLY - no mutations to database state.
// Worker does NOT crash on error - logs and continues.
type SystemMonitoringWorker struct {
	db       *pgxpool.Pool
	log      *zap.Logger
	service  *monitoring.MonitoringService
	interval time.Duration

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

	shutdownCtx context.Context
	cancelFn    context.CancelFunc
}

// SystemMonitoringConfig holds worker configuration
type SystemMonitoringConfig struct {
	Interval time.Duration // How often to run monitoring checks
}

// DefaultSystemMonitoringConfig returns default configuration
func DefaultSystemMonitoringConfig() SystemMonitoringConfig {
	return SystemMonitoringConfig{
		Interval: DefaultSystemMonitoringInterval,
	}
}

// NewSystemMonitoringWorker creates a new system monitoring worker
func NewSystemMonitoringWorker(
	db *pgxpool.Pool,
	log *zap.Logger,
	cfg SystemMonitoringConfig,
) *SystemMonitoringWorker {
	if log == nil {
		log = zap.NewNop()
	}

	if cfg.Interval == 0 {
		cfg.Interval = DefaultSystemMonitoringInterval
	}

	// Create the monitoring service
	monitoringService := monitoring.NewMonitoringService(db, log)

	return &SystemMonitoringWorker{
		db:       db,
		log:      log,
		service:  monitoringService,
		interval: cfg.Interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins periodic monitoring checks in the background
func (w *SystemMonitoringWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("System monitoring worker already running")
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.stopCh = make(chan struct{})

	w.wg.Add(1)
	go w.run()

	w.log.Info("System monitoring worker started",
		zap.Duration("interval", w.interval),
	)
}

// Stop gracefully shuts down the worker
func (w *SystemMonitoringWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.log.Info("Stopping system monitoring worker...")

	w.cancelFn()
	close(w.stopCh)

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.log.Info("System monitoring worker stopped gracefully")
	case <-time.After(10 * time.Second):
		w.log.Warn("System monitoring worker shutdown timeout")
	}

	w.running = false
}

// IsRunning returns true if the worker is currently running
func (w *SystemMonitoringWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// run is the main worker loop
func (w *SystemMonitoringWorker) run() {
	defer w.wg.Done()

	w.runOnce()

	for {
		select {
		case <-w.shutdownCtx.Done():
			w.log.Info("Worker shutdown requested")
			return

		case <-time.After(w.interval):
			w.runOnce()

		case <-w.stopCh:
			return
		}
	}
}

// RunOnce executes a single monitoring check.
// Can be called manually for on-demand verification.
func (w *SystemMonitoringWorker) RunOnce() {
	w.runOnce()
}

// runOnce executes all monitoring checks and logs results
// Does NOT crash on error - logs and continues
func (w *SystemMonitoringWorker) runOnce() {
	ctx := context.Background()

	w.log.Debug("Starting system monitoring check")

	// Run all checks - service handles individual check errors
	results := w.service.RunChecks(ctx)

	// Log results
	errorCount := 0
	warningCount := 0

	for _, result := range results {
		switch result.Status {
		case "ERROR":
			errorCount++
			w.log.Error("Monitoring check FAILED",
				zap.String("check", result.Name),
				zap.String("status", result.Status),
				zap.String("message", result.Message),
				zap.Int("count", result.Count),
				zap.Any("details", result.Details),
			)
		case "WARNING":
			warningCount++
			w.log.Warn("Monitoring check WARNING",
				zap.String("check", result.Name),
				zap.String("status", result.Status),
				zap.String("message", result.Message),
				zap.Int("count", result.Count),
				zap.Any("details", result.Details),
			)
		default:
			w.log.Debug("Monitoring check OK",
				zap.String("check", result.Name),
				zap.String("status", result.Status),
				zap.String("message", result.Message),
			)
		}
	}

	// Summary log
	if errorCount > 0 || warningCount > 0 {
		w.log.Warn("System monitoring check completed with issues",
			zap.Int("errors", errorCount),
			zap.Int("warnings", warningCount),
		)
	} else {
		w.log.Info("System monitoring check completed - all checks OK")
	}
}

// GetResults returns the current monitoring check results
// This is a convenience method for external callers
func (w *SystemMonitoringWorker) GetResults(ctx context.Context) []monitoring.CheckResult {
	return w.service.RunChecks(ctx)
}

// HealthCheck returns the current health status of the monitored system
func (w *SystemMonitoringWorker) HealthCheck(ctx context.Context) error {
	results := w.service.RunChecks(ctx)

	for _, result := range results {
		if result.Status == "ERROR" {
			return fmt.Errorf("monitoring check failed: %s - %s", result.Name, result.Message)
		}
	}

	return nil
}


