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
	// DefaultEscrowIntegrityInterval is how often the worker runs the escrow check.
	DefaultEscrowIntegrityInterval = 15 * time.Minute
)

// EscrowIntegrityConfig holds configuration parsed from environment variables.
// Constructed via ParseEscrowIntegrityConfig.
type EscrowIntegrityConfig struct {
	Disabled       bool
	Interval       time.Duration
	ShadowMode     bool
}

// ParseEscrowIntegrityConfig reads escrow integrity worker configuration from env.
//
// PASS_18R: this detector protects a money/escrow invariant and must not be
// silently dormant, so it now defaults to ENABLED (in shadow mode) rather
// than disabled. An operator must explicitly opt out via
// DISABLE_ESCROW_INTEGRITY_WORKER=true.
//
// Environment variables:
//   - DISABLE_ESCROW_INTEGRITY_WORKER     default "false" (enabled)
//   - ESCROW_INTEGRITY_INTERVAL_MINUTES   default 15
//   - ESCROW_INTEGRITY_SHADOW_MODE        default "true" (shadow)
func ParseEscrowIntegrityConfig() EscrowIntegrityConfig {
	cfg := EscrowIntegrityConfig{
		Disabled:   false,
		Interval:   DefaultEscrowIntegrityInterval,
		ShadowMode: true,
	}

	if v := strings.TrimSpace(os.Getenv("DISABLE_ESCROW_INTEGRITY_WORKER")); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes", "on":
			cfg.Disabled = true
		case "0", "false", "no", "off":
			cfg.Disabled = false
		}
	}

	if v := strings.TrimSpace(os.Getenv("ESCROW_INTEGRITY_INTERVAL_MINUTES")); v != "" {
		if mins, err := strconv.Atoi(v); err == nil && mins > 0 {
			cfg.Interval = time.Duration(mins) * time.Minute
		}
	}

	if v := strings.TrimSpace(os.Getenv("ESCROW_INTEGRITY_SHADOW_MODE")); v != "" {
		switch strings.ToLower(v) {
		case "0", "false", "no", "off":
			cfg.ShadowMode = false
		}
	}

	return cfg
}

// EscrowIntegrityWorker is a shadow-rollout wrapper around EscrowIntegrityChecker.
//
// ENABLED-BY-DEFAULT, SHADOW-FIRST (PASS_18R):
//
//	This worker is enabled by default (DISABLE_ESCROW_INTEGRITY_WORKER=false) and
//	runs in shadow mode by default (ESCROW_INTEGRITY_SHADOW_MODE=true). Shadow mode
//	logs check results but does NOT create alerts — the underlying checker already
//	respects its own shadowMode flag for alert suppression. This is a read-only
//	detector: it must never be silently dormant, so it defaults ON rather than OFF.
//
// WIRED IN STARTUP (enabled by default):
//
//	This worker is constructed and conditionally started in dependencies.go behind
//	the workerEnabled("ESCROW_INTEGRITY_WORKER", true) gate. To deactivate:
//	  1. Set DISABLE_ESCROW_INTEGRITY_WORKER=true
//	  2. Set ESCROW_INTEGRITY_SHADOW_MODE=false to promote shadow logging to live alerts
type EscrowIntegrityWorker struct {
	checker    *walletApp.EscrowIntegrityChecker
	logger     *zap.Logger
	interval   time.Duration
	shadowMode bool

	mu          sync.RWMutex
	running     bool
	shutdownCtx context.Context
	cancelFn    context.CancelFunc
	wg          sync.WaitGroup
}

// NewEscrowIntegrityWorker creates a new escrow integrity worker.
//
// Dependencies:
//   - checker: the reconciliation checker from wallet/application
//   - logger: nil is safe (falls back to zap.NewNop)
//   - interval: tick interval between checks
//   - shadowMode: must match the value passed to the checker (used only for
//     accurate per-cycle log reporting; the checker independently enforces
//     its own alert-suppression behavior)
func NewEscrowIntegrityWorker(
	checker *walletApp.EscrowIntegrityChecker,
	logger *zap.Logger,
	interval time.Duration,
	shadowMode bool,
) *EscrowIntegrityWorker {
	if logger == nil {
		logger = zap.NewNop()
	}
	if interval <= 0 {
		interval = DefaultEscrowIntegrityInterval
	}
	return &EscrowIntegrityWorker{
		checker:    checker,
		logger:     logger,
		interval:   interval,
		shadowMode: shadowMode,
	}
}

// Start begins the escrow integrity check loop.
// Idempotent: calling Start on an already-running worker is a no-op.
func (w *EscrowIntegrityWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.logger.Warn("EscrowIntegrityWorker already running")
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.wg.Add(1)
	go w.run()

	w.logger.Info("EscrowIntegrityWorker started",
		zap.Duration("interval", w.interval),
	)
}

// Stop signals the worker to stop and waits for the current cycle to finish.
// Safe to call before Start or after Stop.
func (w *EscrowIntegrityWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.cancelFn()
	w.wg.Wait()
	w.running = false

	w.logger.Info("EscrowIntegrityWorker stopped")
}

// IsRunning returns true if the worker is currently active.
func (w *EscrowIntegrityWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

func (w *EscrowIntegrityWorker) run() {
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

// checkOnce performs a single escrow integrity check cycle.
func (w *EscrowIntegrityWorker) checkOnce(ctx context.Context) {
	if w.checker == nil {
		w.logger.Warn("escrow integrity check skipped: checker is nil")
		return
	}

	start := time.Now()

	mismatchCount, err := w.checker.CheckEscrowIntegrity(ctx)
	duration := time.Since(start)

	if err != nil {
		w.logger.Warn("escrow integrity check failed",
			zap.Error(err),
			zap.Duration("duration", duration),
		)
		return
	}

	w.logCheckCompleted(mismatchCount, duration)
}

// logCheckCompleted logs the outcome of a completed check cycle. Extracted
// as its own method so the alert_suppressed field's correctness (it must
// reflect w.shadowMode, never be hardcoded — PASS_18R) can be regression
// tested directly without needing a database-backed checker run.
func (w *EscrowIntegrityWorker) logCheckCompleted(mismatchCount int, duration time.Duration) {
	w.logger.Info("escrow integrity check completed",
		zap.Int("mismatch_count", mismatchCount),
		zap.Duration("duration", duration),
		zap.Bool("alert_suppressed", w.shadowMode),
	)
}


