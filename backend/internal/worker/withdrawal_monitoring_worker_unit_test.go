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

// withdrawalEmptyMockDB returns a mockDB whose WithTx runs the callback with a
// mock Tx that returns empty rows — so runOnce() completes without panic.
func withdrawalEmptyMockDB() *mockDB {
	return &mockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
			return fn(&withdrawalUnitMockTx{})
		},
	}
}

// withdrawalUnitMockTx implements db.Tx, returning empty rows for every Query.
type withdrawalUnitMockTx struct{}

func (m *withdrawalUnitMockTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &mockRow{err: nil}
}
func (m *withdrawalUnitMockTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return &mockRows{rows: [][]any{}}, nil
}
func (m *withdrawalUnitMockTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("0"), nil
}
func (m *withdrawalUnitMockTx) Commit(_ context.Context) error   { return nil }
func (m *withdrawalUnitMockTx) Rollback(_ context.Context) error { return nil }

// TestWithdrawalMonitoringWorker_Construction proves nil-safe construction.
func TestWithdrawalMonitoringWorker_Construction(t *testing.T) {
	log := zaptest.NewLogger(t)
	cfg := DefaultWithdrawalMonitoringConfig()

	w := NewWithdrawalMonitoringWorker(nil, log, cfg)
	if w == nil {
		t.Fatal("NewWithdrawalMonitoringWorker() returned nil")
	}
	if w.cfg.Interval != DefaultWithdrawalMonitoringInterval {
		t.Errorf("interval = %v, want %v", w.cfg.Interval, DefaultWithdrawalMonitoringInterval)
	}
	if w.cfg.StuckThreshold != DefaultWithdrawalStuckThreshold {
		t.Errorf("stuckThreshold = %v, want %v", w.cfg.StuckThreshold, DefaultWithdrawalStuckThreshold)
	}
}

// TestWithdrawalMonitoringWorker_DefaultConfig proves default values.
func TestWithdrawalMonitoringWorker_DefaultConfig(t *testing.T) {
	cfg := DefaultWithdrawalMonitoringConfig()
	if cfg.Interval != 30*time.Minute {
		t.Errorf("Interval = %v, want 30m", cfg.Interval)
	}
	if cfg.StuckThreshold != 24*time.Hour {
		t.Errorf("StuckThreshold = %v, want 24h", cfg.StuckThreshold)
	}
}

// TestWithdrawalMonitoringWorker_ZeroIntervalFallsBack proves zero interval uses default.
func TestWithdrawalMonitoringWorker_ZeroIntervalFallsBack(t *testing.T) {
	log := zaptest.NewLogger(t)
	w := NewWithdrawalMonitoringWorker(nil, log, WithdrawalMonitoringConfig{Interval: 0, StuckThreshold: 0})
	if w.cfg.Interval != DefaultWithdrawalMonitoringInterval {
		t.Errorf("zero interval should fall back to default %v, got %v",
			DefaultWithdrawalMonitoringInterval, w.cfg.Interval)
	}
	if w.cfg.StuckThreshold != DefaultWithdrawalStuckThreshold {
		t.Errorf("zero StuckThreshold should fall back to default %v, got %v",
			DefaultWithdrawalStuckThreshold, w.cfg.StuckThreshold)
	}
}

// TestWithdrawalMonitoringWorker_IsRunningInitiallyFalse proves initial state.
func TestWithdrawalMonitoringWorker_IsRunningInitiallyFalse(t *testing.T) {
	log := zaptest.NewLogger(t)
	w := NewWithdrawalMonitoringWorker(nil, log, DefaultWithdrawalMonitoringConfig())
	if w.IsRunning() {
		t.Fatal("must not be running before Start()")
	}
}

// TestWithdrawalMonitoringWorker_StopBeforeStartIsSafe proves Stop() on idle worker is safe.
func TestWithdrawalMonitoringWorker_StopBeforeStartIsSafe(t *testing.T) {
	log := zaptest.NewLogger(t)
	w := NewWithdrawalMonitoringWorker(nil, log, DefaultWithdrawalMonitoringConfig())
	w.Stop() // must not panic
	if w.IsRunning() {
		t.Error("IsRunning() should be false")
	}
}

// TestWithdrawalMonitoringWorker_StartStopIsRunning proves lifecycle transitions
// using a mockDB so runOnce() completes without panic.
func TestWithdrawalMonitoringWorker_StartStopIsRunning(t *testing.T) {
	log := zaptest.NewLogger(t)

	w := NewWithdrawalMonitoringWorker(withdrawalEmptyMockDB(), log, WithdrawalMonitoringConfig{
		Interval:       50 * time.Millisecond,
		StuckThreshold: DefaultWithdrawalStuckThreshold,
	})

	if w.IsRunning() {
		t.Fatal("must not be running before Start()")
	}

	w.Start()
	if !w.IsRunning() {
		t.Fatal("must be running after Start()")
	}

	time.Sleep(20 * time.Millisecond)

	w.Stop()
	if w.IsRunning() {
		t.Fatal("must not be running after Stop()")
	}
}

// TestWithdrawalMonitoringWorker_DoubleStartIsSafe proves double-start is a no-op.
func TestWithdrawalMonitoringWorker_DoubleStartIsSafe(t *testing.T) {
	log := zaptest.NewLogger(t)
	w := NewWithdrawalMonitoringWorker(withdrawalEmptyMockDB(), log, WithdrawalMonitoringConfig{
		Interval:       50 * time.Millisecond,
		StuckThreshold: DefaultWithdrawalStuckThreshold,
	})

	w.Start()
	w.Start() // must not panic or spawn second goroutine
	if !w.IsRunning() {
		t.Fatal("must still be running")
	}
	w.Stop()
}

// TestWithdrawalMonitoringWorker_RunOnceWithEmptyDB proves RunOnce returns empty slice and no error
// when the DB has no stuck withdrawals.
func TestWithdrawalMonitoringWorker_RunOnceWithEmptyDB(t *testing.T) {
	log := zaptest.NewLogger(t)
	w := NewWithdrawalMonitoringWorker(withdrawalEmptyMockDB(), log, DefaultWithdrawalMonitoringConfig())

	stuck, err := w.RunOnce()
	if err != nil {
		t.Errorf("RunOnce() error = %v, want nil", err)
	}
	if len(stuck) != 0 {
		t.Errorf("RunOnce() returned %d stuck withdrawals, want 0", len(stuck))
	}
}

// TestWithdrawalMonitoringWorker_NotifyAdminDashboard_NilNotifierDoesNotPanic proves
// the nil-notifier guard added in Z4. Prior to this fix, a nil notifier would panic.
func TestWithdrawalMonitoringWorker_NotifyAdminDashboard_NilNotifierDoesNotPanic(t *testing.T) {
	log := zaptest.NewLogger(t)
	w := NewWithdrawalMonitoringWorker(withdrawalEmptyMockDB(), log, DefaultWithdrawalMonitoringConfig())

	// Must not panic with nil notifier — returns nil after logging a warning.
	err := w.NotifyAdminDashboard(nil)
	if err != nil {
		t.Errorf("NotifyAdminDashboard(nil) error = %v, want nil", err)
	}
}


