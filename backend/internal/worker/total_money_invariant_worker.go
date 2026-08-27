package worker

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	walletApp "github.com/labuda/backend/internal/core/wallet/application"
	"go.uber.org/zap"
)

const (
	// DefaultTotalMoneyInvariantInterval is how often the worker runs the check.
	DefaultTotalMoneyInvariantInterval = 15 * time.Minute
)

// TotalMoneyInvariantConfig holds configuration parsed from environment variables.
// Constructed via ParseTotalMoneyInvariantConfig.
type TotalMoneyInvariantConfig struct {
	Disabled   bool
	Interval   time.Duration
	ShadowMode bool
}

// ParseTotalMoneyInvariantConfig reads total money invariant worker configuration from env.
//
// PASS_18R: this detector protects a money/ledger invariant and must not be
// silently dormant, so it now defaults to ENABLED (in shadow mode) rather
// than disabled. An operator must explicitly opt out via
// DISABLE_TOTAL_MONEY_INVARIANT_WORKER=true.
//
// Environment variables:
//   - DISABLE_TOTAL_MONEY_INVARIANT_WORKER     default "false" (enabled)
//   - TOTAL_MONEY_INVARIANT_INTERVAL_MINUTES   default 15
//   - TOTAL_MONEY_INVARIANT_SHADOW_MODE        default "true" (shadow)
func ParseTotalMoneyInvariantConfig() TotalMoneyInvariantConfig {
	cfg := TotalMoneyInvariantConfig{
		Disabled:   false,
		Interval:   DefaultTotalMoneyInvariantInterval,
		ShadowMode: true,
	}

	if v := strings.TrimSpace(os.Getenv("DISABLE_TOTAL_MONEY_INVARIANT_WORKER")); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes", "on":
			cfg.Disabled = true
		case "0", "false", "no", "off":
			cfg.Disabled = false
		}
	}

	if v := strings.TrimSpace(os.Getenv("TOTAL_MONEY_INVARIANT_INTERVAL_MINUTES")); v != "" {
		if mins, err := strconv.Atoi(v); err == nil && mins > 0 {
			cfg.Interval = time.Duration(mins) * time.Minute
		}
	}

	if v := strings.TrimSpace(os.Getenv("TOTAL_MONEY_INVARIANT_SHADOW_MODE")); v != "" {
		switch strings.ToLower(v) {
		case "0", "false", "no", "off":
			cfg.ShadowMode = false
		}
	}

	return cfg
}

// TotalMoneyInvariantWorker is a shadow-rollout wrapper around TotalMoneyInvariantChecker.
//
// ENABLED-BY-DEFAULT, SHADOW-FIRST (PASS_18R):
//
//	This worker is enabled by default (DISABLE_TOTAL_MONEY_INVARIANT_WORKER=false) and
//	runs in shadow mode by default (TOTAL_MONEY_INVARIANT_SHADOW_MODE=true). Shadow mode
//	logs check results but does NOT create alerts — the underlying checker already
//	respects its own shadowMode flag for alert suppression. This is a read-only
//	detector: it must never be silently dormant, so it defaults ON rather than OFF.
//
// WIRED IN STARTUP (enabled by default):
//
//	This worker is constructed and conditionally started in dependencies.go behind
//	the workerEnabled("TOTAL_MONEY_INVARIANT_WORKER", true) gate. To deactivate:
//	  1. Set DISABLE_TOTAL_MONEY_INVARIANT_WORKER=true
//	  2. Set TOTAL_MONEY_INVARIANT_SHADOW_MODE=false to promote shadow logging to live alerts
type TotalMoneyInvariantWorker struct {
	checker    *walletApp.TotalMoneyInvariantChecker
	logger     *zap.Logger
	interval   time.Duration
	shadowMode bool

	mu          sync.RWMutex
	running     bool
	shutdownCtx context.Context
	cancelFn    context.CancelFunc
	wg          sync.WaitGroup
}

// NewTotalMoneyInvariantWorker creates a new total money invariant worker.
//
// Dependencies:
//   - checker: the invariant checker from wallet/application
//   - logger: nil is safe (falls back to zap.NewNop)
//   - interval: tick interval between checks
//   - shadowMode: must match the value passed to the checker (used only for
//     accurate per-cycle log reporting; the checker independently enforces
//     its own alert-suppression behavior)
func NewTotalMoneyInvariantWorker(
	checker *walletApp.TotalMoneyInvariantChecker,
	logger *zap.Logger,
	interval time.Duration,
	shadowMode bool,
) *TotalMoneyInvariantWorker {
	if logger == nil {
		logger = zap.NewNop()
	}
	if interval <= 0 {
		interval = DefaultTotalMoneyInvariantInterval
	}
	return &TotalMoneyInvariantWorker{
		checker:    checker,
		logger:     logger,
		interval:   interval,
		shadowMode: shadowMode,
	}
}

// Start begins the total money invariant check loop.
// Idempotent: calling Start on an already-running worker is a no-op.
func (w *TotalMoneyInvariantWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.logger.Warn("TotalMoneyInvariantWorker already running")
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.wg.Add(1)
	go w.run()

	w.logger.Info("TotalMoneyInvariantWorker started",
		zap.Duration("interval", w.interval),
	)
}

// Stop signals the worker to stop and waits for the current cycle to finish.
// Safe to call before Start or after Stop.
func (w *TotalMoneyInvariantWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.cancelFn()
	w.wg.Wait()
	w.running = false

	w.logger.Info("TotalMoneyInvariantWorker stopped")
}

// IsRunning returns true if the worker is currently active.
func (w *TotalMoneyInvariantWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

func (w *TotalMoneyInvariantWorker) run() {
	defer w.wg.Done()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Immediate check on startup to detect any drift accumulated before this
	// worker instance was started (e.g., after a redeploy).
	w.checkOnce(w.shutdownCtx)

	for {
		select {
		case <-w.shutdownCtx.Done():
			return
		case <-ticker.C:
			w.checkOnce(w.shutdownCtx)
		}
	}
}

// checkOnce performs a single total money invariant check cycle.
func (w *TotalMoneyInvariantWorker) checkOnce(ctx context.Context) {
	if w.checker == nil {
		w.logger.Warn("total money invariant check skipped: checker is nil")
		return
	}

	start := time.Now()

	violationFound, err := w.checker.CheckTotalMoneyInvariant(ctx)
	duration := time.Since(start)

	if err != nil {
		w.logger.Warn("total money invariant check failed",
			zap.Error(err),
			zap.Duration("duration", duration),
		)
		return
	}

	w.logCheckCompleted(violationFound, duration)
}

// logCheckCompleted logs the outcome of a completed check cycle. Extracted
// as its own method so the alert_suppressed field's correctness (it must
// reflect w.shadowMode, never be hardcoded — PASS_18R) can be regression
// tested directly without needing a database-backed checker run.
func (w *TotalMoneyInvariantWorker) logCheckCompleted(violationFound bool, duration time.Duration) {
	w.logger.Info("total money invariant check completed",
		zap.Bool("violation_found", violationFound),
		zap.Duration("duration", duration),
		zap.Bool("alert_suppressed", w.shadowMode),
	)
}


