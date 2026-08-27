package worker

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap/zaptest"
)

// emptyAuctionMockDB returns a Transactor whose Query always returns zero rows.
func emptyAuctionMockDB() *mockDB {
	return &mockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
			return fn(&mockTx{
				QueryFunc: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
					return &mockRows{rows: [][]any{}}, nil
				},
			})
		},
	}
}

// =============================================================================
// Construction tests
// =============================================================================

func TestAuctionStartWorker_Construction(t *testing.T) {
	log := zaptest.NewLogger(t)

	w := NewAuctionStartWorker(nil, nil, log, DefaultAuctionStartWorkerConfig())
	if w == nil {
		t.Fatal("NewAuctionStartWorker() returned nil")
	}
	if w.pollInterval != DefaultAuctionStartPollInterval {
		t.Errorf("pollInterval = %v, want %v", w.pollInterval, DefaultAuctionStartPollInterval)
	}
	if w.batchSize != DefaultAuctionStartBatchSize {
		t.Errorf("batchSize = %v, want %v", w.batchSize, DefaultAuctionStartBatchSize)
	}
}

func TestAuctionStartWorker_NilLogFallback(t *testing.T) {
	w := NewAuctionStartWorker(nil, nil, nil, DefaultAuctionStartWorkerConfig())
	if w == nil {
		t.Fatal("NewAuctionStartWorker with nil logger returned nil")
	}
}

func TestAuctionStartWorker_DefaultConfig_Sane(t *testing.T) {
	cfg := DefaultAuctionStartWorkerConfig()

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

func TestAuctionStartWorker_Lifecycle(t *testing.T) {
	log := zaptest.NewLogger(t)

	w := NewAuctionStartWorker(emptyAuctionMockDB(), nil, log, AuctionStartWorkerConfig{
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


