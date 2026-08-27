package worker

import (
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
)

// =============================================================================
// Construction tests
// =============================================================================

func TestCleanupWorker_Construction(t *testing.T) {
	log := zaptest.NewLogger(t)

	w := NewCleanupWorker(nil, log)
	if w == nil {
		t.Fatal("NewCleanupWorker() returned nil")
	}
	if w.retention != DefaultCleanupRetention {
		t.Errorf("retention = %v, want %v", w.retention, DefaultCleanupRetention)
	}
	if w.cleanupInterval != DefaultCleanupInterval {
		t.Errorf("cleanupInterval = %v, want %v", w.cleanupInterval, DefaultCleanupInterval)
	}
}

func TestCleanupWorker_NilLogFallback(t *testing.T) {
	// nil logger must not panic
	w := NewCleanupWorker(nil, nil)
	if w == nil {
		t.Fatal("NewCleanupWorker(nil, nil) returned nil")
	}
}

func TestCleanupWorker_SetRetention(t *testing.T) {
	log := zaptest.NewLogger(t)
	w := NewCleanupWorker(nil, log)

	custom := 7 * 24 * time.Hour
	w.SetRetention(custom)
	if w.retention != custom {
		t.Errorf("retention = %v, want %v", w.retention, custom)
	}
}

func TestCleanupWorker_SetCleanupInterval(t *testing.T) {
	log := zaptest.NewLogger(t)
	w := NewCleanupWorker(nil, log)

	custom := 6 * time.Hour
	w.SetCleanupInterval(custom)
	if w.cleanupInterval != custom {
		t.Errorf("cleanupInterval = %v, want %v", w.cleanupInterval, custom)
	}
}

// =============================================================================
// Lifecycle tests
// =============================================================================

// TestCleanupWorker_Lifecycle verifies IsRunning before Start.
// We do NOT call Start here because the worker runs runCleanup immediately,
// which requires a real DB. Lifecycle state before Start is safe to verify.
func TestCleanupWorker_IsRunning_BeforeStart(t *testing.T) {
	log := zaptest.NewLogger(t)
	w := NewCleanupWorker(nil, log)

	if w.IsRunning() {
		t.Fatal("worker should not be running before Start")
	}
}

// TestCleanupWorker_Defaults verifies DefaultCleanupRetention and
// DefaultCleanupInterval are non-zero.
func TestCleanupWorker_Defaults(t *testing.T) {
	if DefaultCleanupRetention <= 0 {
		t.Errorf("DefaultCleanupRetention = %v, want > 0", DefaultCleanupRetention)
	}
	if DefaultCleanupInterval <= 0 {
		t.Errorf("DefaultCleanupInterval = %v, want > 0", DefaultCleanupInterval)
	}
	// Retention must be longer than interval (logs should survive at least one cleanup cycle).
	if DefaultCleanupRetention <= DefaultCleanupInterval {
		t.Errorf("retention %v should be > interval %v", DefaultCleanupRetention, DefaultCleanupInterval)
	}
}

// =============================================================================
// EnqueuePushRetry idempotency constant check
// =============================================================================

func TestPushRetry_Constants(t *testing.T) {
	if maxPushRetryAttempts <= 0 {
		t.Errorf("maxPushRetryAttempts = %d, want > 0", maxPushRetryAttempts)
	}
	if pushRetryWindow <= 0 {
		t.Errorf("pushRetryWindow = %v, want > 0", pushRetryWindow)
	}
	if pushBatchSize <= 0 {
		t.Errorf("pushBatchSize = %d, want > 0", pushBatchSize)
	}
	// Business constraint: retry window must be meaningful (≥ 1h).
	if pushRetryWindow < time.Hour {
		t.Errorf("pushRetryWindow = %v, want ≥ 1h", pushRetryWindow)
	}
}


