package worker

import (
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
)

// =============================================================================
// Construction tests
// =============================================================================

func TestAuctionEndWorker_Construction(t *testing.T) {
	log := zaptest.NewLogger(t)

	w := NewAuctionEndWorker(nil, nil, log, DefaultAuctionEndWorkerConfig())
	if w == nil {
		t.Fatal("NewAuctionEndWorker() returned nil")
	}
	if w.pollInterval != DefaultAuctionPollInterval {
		t.Errorf("pollInterval = %v, want %v", w.pollInterval, DefaultAuctionPollInterval)
	}
	if w.batchSize != DefaultAuctionBatchSize {
		t.Errorf("batchSize = %v, want %v", w.batchSize, DefaultAuctionBatchSize)
	}
}

func TestAuctionEndWorker_NilLogFallback(t *testing.T) {
	w := NewAuctionEndWorker(nil, nil, nil, DefaultAuctionEndWorkerConfig())
	if w == nil {
		t.Fatal("NewAuctionEndWorker with nil logger returned nil")
	}
}

func TestAuctionEndWorker_DefaultConfig_Sane(t *testing.T) {
	cfg := DefaultAuctionEndWorkerConfig()

	if cfg.PollInterval < 10*time.Second || cfg.PollInterval > 5*time.Minute {
		t.Errorf("PollInterval = %v, want [10s, 5m]", cfg.PollInterval)
	}
	if cfg.BatchSize < 1 || cfg.BatchSize > 1000 {
		t.Errorf("BatchSize = %d, want [1, 1000]", cfg.BatchSize)
	}
}

// =============================================================================
// Lifecycle tests
// =============================================================================

func TestAuctionEndWorker_Lifecycle(t *testing.T) {
	log := zaptest.NewLogger(t)

	w := NewAuctionEndWorker(emptyAuctionMockDB(), nil, log, AuctionEndWorkerConfig{
		PollInterval: 10 * time.Second, // long enough to never fire
		BatchSize:    50,
	})

	if w.IsRunning() {
		t.Fatal("worker should not be running before Start")
	}

	w.Start()
	time.Sleep(20 * time.Millisecond) // let initial poll complete
	if !w.IsRunning() {
		t.Fatal("worker should be running after Start")
	}

	// Idempotent: second Start is a no-op
	w.Start()
	if !w.IsRunning() {
		t.Fatal("worker should still be running after double Start")
	}

	w.Stop()
	if w.IsRunning() {
		t.Fatal("worker should not be running after Stop")
	}

	// Idempotent: second Stop is a no-op
	w.Stop()
	if w.IsRunning() {
		t.Fatal("worker should not be running after double Stop")
	}
}


