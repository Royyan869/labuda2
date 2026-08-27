package worker

import (
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
)

// =============================================================================
// Construction tests
// =============================================================================

func TestIdempotencyCleanupWorker_DefaultConfig(t *testing.T) {
	cfg := DefaultIdempotencyCleanupConfig()
	if cfg.PollInterval != DefaultIdempotencyCleanupPollInterval {
		t.Errorf("PollInterval = %v, want %v", cfg.PollInterval, DefaultIdempotencyCleanupPollInterval)
	}
	if cfg.RetentionDays != DefaultIdempotencyRetentionDays {
		t.Errorf("RetentionDays = %d, want %d", cfg.RetentionDays, DefaultIdempotencyRetentionDays)
	}
}

func TestIdempotencyCleanupWorker_Construction(t *testing.T) {
	log := zaptest.NewLogger(t)
	cfg := DefaultIdempotencyCleanupConfig()

	w := NewIdempotencyCleanupWorker(nil, log, cfg)
	if w == nil {
		t.Fatal("NewIdempotencyCleanupWorker() returned nil")
	}
	if w.retentionDays != DefaultIdempotencyRetentionDays {
		t.Errorf("retentionDays = %d, want %d", w.retentionDays, DefaultIdempotencyRetentionDays)
	}
	if w.pollInterval != DefaultIdempotencyCleanupPollInterval {
		t.Errorf("pollInterval = %v, want %v", w.pollInterval, DefaultIdempotencyCleanupPollInterval)
	}
}

func TestIdempotencyCleanupWorker_NilLogFallback(t *testing.T) {
	cfg := DefaultIdempotencyCleanupConfig()
	w := NewIdempotencyCleanupWorker(nil, nil, cfg)
	if w == nil {
		t.Fatal("NewIdempotencyCleanupWorker(nil, nil, cfg) returned nil")
	}
}

// =============================================================================
// Retention guard tests
// =============================================================================

func TestIdempotencyCleanupWorker_MinRetentionEnforced(t *testing.T) {
	log := zaptest.NewLogger(t)
	cfg := IdempotencyCleanupConfig{
		PollInterval:  1 * time.Hour,
		RetentionDays: 1, // below minimum
	}

	w := NewIdempotencyCleanupWorker(nil, log, cfg)
	if w.retentionDays != MinIdempotencyRetentionDays {
		t.Errorf("retentionDays = %d, want minimum %d", w.retentionDays, MinIdempotencyRetentionDays)
	}
}

func TestIdempotencyCleanupWorker_ZeroConfigUsesDefaults(t *testing.T) {
	log := zaptest.NewLogger(t)
	cfg := IdempotencyCleanupConfig{} // all zeros

	w := NewIdempotencyCleanupWorker(nil, log, cfg)
	if w.retentionDays != DefaultIdempotencyRetentionDays {
		t.Errorf("retentionDays = %d, want %d", w.retentionDays, DefaultIdempotencyRetentionDays)
	}
	if w.pollInterval != DefaultIdempotencyCleanupPollInterval {
		t.Errorf("pollInterval = %v, want %v", w.pollInterval, DefaultIdempotencyCleanupPollInterval)
	}
}

// =============================================================================
// Lifecycle tests
// =============================================================================

func TestIdempotencyCleanupWorker_IsRunning_BeforeStart(t *testing.T) {
	log := zaptest.NewLogger(t)
	cfg := DefaultIdempotencyCleanupConfig()
	w := NewIdempotencyCleanupWorker(nil, log, cfg)

	if w.IsRunning() {
		t.Fatal("worker should not be running before Start")
	}
}

// =============================================================================
// Constants sanity
// =============================================================================

func TestIdempotencyCleanupWorker_Constants(t *testing.T) {
	if DefaultIdempotencyCleanupPollInterval <= 0 {
		t.Errorf("DefaultIdempotencyCleanupPollInterval = %v, want > 0", DefaultIdempotencyCleanupPollInterval)
	}
	if DefaultIdempotencyRetentionDays <= 0 {
		t.Errorf("DefaultIdempotencyRetentionDays = %d, want > 0", DefaultIdempotencyRetentionDays)
	}
	if MinIdempotencyRetentionDays <= 0 {
		t.Errorf("MinIdempotencyRetentionDays = %d, want > 0", MinIdempotencyRetentionDays)
	}
	if MinIdempotencyRetentionDays > DefaultIdempotencyRetentionDays {
		t.Errorf("min %d should be <= default %d", MinIdempotencyRetentionDays, DefaultIdempotencyRetentionDays)
	}
	// Poll interval should be at least 1 hour (no aggressive polling).
	if DefaultIdempotencyCleanupPollInterval < time.Hour {
		t.Errorf("poll interval %v should be >= 1h", DefaultIdempotencyCleanupPollInterval)
	}
}


