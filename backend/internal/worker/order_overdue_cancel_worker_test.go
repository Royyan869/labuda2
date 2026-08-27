package worker

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap/zaptest"

	"github.com/labuda/backend/pkg/db"
)

// TestOrderOverdueCancelWorker_NewWorker proves construction succeeds with valid deps.
func TestOrderOverdueCancelWorker_NewWorker(t *testing.T) {
	log := zaptest.NewLogger(t)
	cfg := DefaultOrderOverdueCancelConfig()

	w := NewOrderOverdueCancelWorker(nil, nil, log, cfg)

	if w == nil {
		t.Fatal("NewOrderOverdueCancelWorker() returned nil")
	}
	if w.pollInterval != DefaultOverdueCancelPollInterval {
		t.Errorf("pollInterval = %v, want %v", w.pollInterval, DefaultOverdueCancelPollInterval)
	}
	if w.batchSize != DefaultOverdueCancelBatchSize {
		t.Errorf("batchSize = %d, want %d", w.batchSize, DefaultOverdueCancelBatchSize)
	}
	if w.workerID == "" {
		t.Error("workerID must not be empty")
	}
}

// TestOrderOverdueCancelWorker_StartStop proves Start/Stop lifecycle is safe.
func TestOrderOverdueCancelWorker_StartStop(t *testing.T) {
	log := zaptest.NewLogger(t)

	// Mock DB that returns empty overdue list — no orders to cancel.
	emptyMock := &mockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
			return fn(&overdueMockTx{
				QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
					return &mockRows{rows: [][]any{}}, nil
				},
			})
		},
	}

	w := NewOrderOverdueCancelWorker(emptyMock, nil, log, OrderOverdueCancelConfig{
		PollInterval: 10 * time.Millisecond,
		BatchSize:    10,
	})

	if w.IsRunning() {
		t.Fatal("should not be running before Start()")
	}

	w.Start()
	if !w.IsRunning() {
		t.Fatal("should be running after Start()")
	}

	time.Sleep(30 * time.Millisecond) // let one poll cycle fire

	w.Stop()
	if w.IsRunning() {
		t.Fatal("should not be running after Stop()")
	}
}

// TestOrderOverdueCancelWorker_DoubleStart proves duplicate Start() is a no-op.
func TestOrderOverdueCancelWorker_DoubleStart(t *testing.T) {
	log := zaptest.NewLogger(t)

	emptyMock := &mockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
			return fn(&overdueMockTx{
				QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
					return &mockRows{rows: [][]any{}}, nil
				},
			})
		},
	}

	w := NewOrderOverdueCancelWorker(emptyMock, nil, log, OrderOverdueCancelConfig{
		PollInterval: 100 * time.Millisecond,
		BatchSize:    10,
	})

	w.Start()
	w.Start() // second call must not panic or spawn a second goroutine

	if !w.IsRunning() {
		t.Fatal("should still be running after double Start()")
	}

	w.Stop()
}

// TestOrderOverdueCancelWorker_EmptyResult proves no-op when no overdue orders found.
func TestOrderOverdueCancelWorker_EmptyResult(t *testing.T) {
	log := zaptest.NewLogger(t)
	pollCount := 0

	emptyMock := &mockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
			pollCount++
			return fn(&overdueMockTx{
				QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
					return &mockRows{rows: [][]any{}}, nil
				},
			})
		},
	}

	w := NewOrderOverdueCancelWorker(emptyMock, nil, log, OrderOverdueCancelConfig{
		PollInterval: 10 * time.Millisecond,
		BatchSize:    10,
	})

	w.Start()
	time.Sleep(50 * time.Millisecond)
	w.Stop()

	// Must have polled at least once (initial + at least one interval tick)
	if pollCount == 0 {
		t.Error("expected at least one poll cycle, got 0")
	}
}

// TestOrderOverdueCancelWorker_DBErrorDoesNotPanic proves DB failure is logged, not panicked.
func TestOrderOverdueCancelWorker_DBErrorDoesNotPanic(t *testing.T) {
	log := zaptest.NewLogger(t)

	errorMock := &mockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
			return context.DeadlineExceeded // simulate DB timeout
		},
	}

	w := NewOrderOverdueCancelWorker(errorMock, nil, log, OrderOverdueCancelConfig{
		PollInterval: 10 * time.Millisecond,
		BatchSize:    10,
	})

	// Must not panic — error is logged and worker continues
	w.Start()
	time.Sleep(40 * time.Millisecond)
	w.Stop()
}

// TestOrderOverdueCancelWorker_StopBeforeStart proves Stop() on un-started worker is safe.
func TestOrderOverdueCancelWorker_StopBeforeStart(t *testing.T) {
	log := zaptest.NewLogger(t)
	w := NewOrderOverdueCancelWorker(nil, nil, log, DefaultOrderOverdueCancelConfig())

	// Must not panic
	w.Stop()

	if w.IsRunning() {
		t.Error("IsRunning() should be false after Stop() on un-started worker")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Minimal transaction mock for overdue cancel tests
// ─────────────────────────────────────────────────────────────────────────────

type overdueMockTx struct {
	QueryFunc func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (m *overdueMockTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &mockRow{err: nil}
}

func (m *overdueMockTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, sql, args...)
	}
	return &mockRows{rows: [][]any{}}, nil
}

func (m *overdueMockTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("0"), nil
}

func (m *overdueMockTx) Commit(_ context.Context) error   { return nil }
func (m *overdueMockTx) Rollback(_ context.Context) error { return nil }


