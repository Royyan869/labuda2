package worker

import (
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
)

// =============================================================================
// Construction tests
// =============================================================================

func TestAuctionSettlementWorker_Construction(t *testing.T) {
	log := zaptest.NewLogger(t)

	w := NewAuctionSettlementWorker(nil, nil, nil, nil, log, DefaultAuctionSettlementWorkerConfig())
	if w == nil {
		t.Fatal("NewAuctionSettlementWorker() returned nil")
	}
	if w.pollInterval != DefaultAuctionSettlementPollInterval {
		t.Errorf("pollInterval = %v, want %v", w.pollInterval, DefaultAuctionSettlementPollInterval)
	}
	if w.batchSize != DefaultAuctionSettlementBatchSize {
		t.Errorf("batchSize = %v, want %v", w.batchSize, DefaultAuctionSettlementBatchSize)
	}
}

func TestAuctionSettlementWorker_NilLogFallback(t *testing.T) {
	w := NewAuctionSettlementWorker(nil, nil, nil, nil, nil, DefaultAuctionSettlementWorkerConfig())
	if w == nil {
		t.Fatal("NewAuctionSettlementWorker with nil logger returned nil")
	}
}

func TestAuctionSettlementWorker_DefaultConfig_Sane(t *testing.T) {
	cfg := DefaultAuctionSettlementWorkerConfig()

	if cfg.PollInterval < 1*time.Minute || cfg.PollInterval > 30*time.Minute {
		t.Errorf("PollInterval = %v, want [1m, 30m]", cfg.PollInterval)
	}
	if cfg.BatchSize < 1 || cfg.BatchSize > 1000 {
		t.Errorf("BatchSize = %d, want [1, 1000]", cfg.BatchSize)
	}
}

// =============================================================================
// Lifecycle tests
// =============================================================================

func TestAuctionSettlementWorker_Lifecycle(t *testing.T) {
	log := zaptest.NewLogger(t)

	w := NewAuctionSettlementWorker(nil, nil, nil, nil, log, AuctionSettlementWorkerConfig{
		PollInterval: 10 * time.Second, // long enough to never fire
		BatchSize:    50,
	})

	if w.IsRunning() {
		t.Fatal("worker should not be running before Start")
	}

	// Stop before Start must not panic
	w.Stop()
	if w.IsRunning() {
		t.Fatal("worker should not be running after Stop on un-started worker")
	}
}


