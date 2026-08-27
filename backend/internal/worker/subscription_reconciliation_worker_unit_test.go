package worker

import (
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
)

// TestSubscriptionReconciliationWorker_Construction proves nil-safe construction.
func TestSubscriptionReconciliationWorker_Construction(t *testing.T) {
	log := zaptest.NewLogger(t)
	cfg := DefaultSubscriptionReconciliationConfig()

	w := NewSubscriptionReconciliationWorker(nil, log, cfg, nil)

	if w == nil {
		t.Fatal("NewSubscriptionReconciliationWorker() returned nil")
	}
	if w.interval != DefaultSubscriptionReconciliationInterval {
		t.Errorf("interval = %v, want %v", w.interval, DefaultSubscriptionReconciliationInterval)
	}
}

// TestSubscriptionReconciliationWorker_DefaultInterval proves default is 10 minutes.
func TestSubscriptionReconciliationWorker_DefaultInterval(t *testing.T) {
	cfg := DefaultSubscriptionReconciliationConfig()
	if cfg.Interval != 10*time.Minute {
		t.Errorf("DefaultSubscriptionReconciliationConfig().Interval = %v, want 10m", cfg.Interval)
	}
}

// TestSubscriptionReconciliationWorker_ZeroIntervalFallsBack proves zero interval uses default.
func TestSubscriptionReconciliationWorker_ZeroIntervalFallsBack(t *testing.T) {
	log := zaptest.NewLogger(t)
	w := NewSubscriptionReconciliationWorker(nil, log, SubscriptionReconciliationConfig{Interval: 0}, nil)
	if w.interval != DefaultSubscriptionReconciliationInterval {
		t.Errorf("zero interval should fall back to default %v, got %v",
			DefaultSubscriptionReconciliationInterval, w.interval)
	}
}

// TestSubscriptionReconciliationWorker_IsRunningInitiallyFalse proves initial state.
func TestSubscriptionReconciliationWorker_IsRunningInitiallyFalse(t *testing.T) {
	log := zaptest.NewLogger(t)
	w := NewSubscriptionReconciliationWorker(nil, log, DefaultSubscriptionReconciliationConfig(), nil)
	if w.IsRunning() {
		t.Fatal("must not be running before Start()")
	}
}

// TestSubscriptionReconciliationWorker_StopBeforeStartIsSafe proves Stop() on idle worker is safe.
func TestSubscriptionReconciliationWorker_StopBeforeStartIsSafe(t *testing.T) {
	log := zaptest.NewLogger(t)
	w := NewSubscriptionReconciliationWorker(nil, log, DefaultSubscriptionReconciliationConfig(), nil)

	// Must not panic
	w.Stop()
	if w.IsRunning() {
		t.Error("IsRunning() should be false")
	}
}

// TestSubscriptionReconciliationWorker_MetricsInitiallyZero proves atomic counters start at zero.
// This proves idempotency tracking infrastructure is clean on construction.
func TestSubscriptionReconciliationWorker_MetricsInitiallyZero(t *testing.T) {
	log := zaptest.NewLogger(t)
	w := NewSubscriptionReconciliationWorker(nil, log, DefaultSubscriptionReconciliationConfig(), nil)

	if n := w.recoveryAttemptTotal.Load(); n != 0 {
		t.Errorf("recoveryAttemptTotal should be 0, got %d", n)
	}
	if n := w.recoverySuccessTotal.Load(); n != 0 {
		t.Errorf("recoverySuccessTotal should be 0, got %d", n)
	}
	if n := w.recoveryFailureTotal.Load(); n != 0 {
		t.Errorf("recoveryFailureTotal should be 0, got %d", n)
	}
	if n := w.consecutiveFailureCount.Load(); n != 0 {
		t.Errorf("consecutiveFailureCount should be 0, got %d", n)
	}
}


