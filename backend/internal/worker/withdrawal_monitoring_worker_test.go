//go:build integration

// Package worker provides background workers for periodic tasks.
package worker

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap/zaptest"

	"github.com/labuda/backend/pkg/db"
)

// withdrawalMockTx implements db.Tx for withdrawal monitoring worker tests
type withdrawalMockTx struct {
	queryRowCalls int
	queryCalls    int
	QueryFunc     func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	ExecFunc      func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	CommitFunc    func(ctx context.Context) error
	RollbackFunc  func(ctx context.Context) error
}

func (m *withdrawalMockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	m.queryRowCalls++
	return &withdrawalMockRow{err: errors.New("not implemented")}
}

func (m *withdrawalMockTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	m.queryCalls++
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, sql, args...)
	}
	return &withdrawalMockRows{}, nil
}

func (m *withdrawalMockTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	m.queryCalls++
	if m.ExecFunc != nil {
		return m.ExecFunc(ctx, sql, args...)
	}
	return pgconn.NewCommandTag("0"), nil
}

func (m *withdrawalMockTx) Commit(ctx context.Context) error {
	if m.CommitFunc != nil {
		return m.CommitFunc(ctx)
	}
	return nil
}

func (m *withdrawalMockTx) Rollback(ctx context.Context) error {
	if m.RollbackFunc != nil {
		return m.RollbackFunc(ctx)
	}
	return nil
}

// withdrawalMockRow implements pgx.Row for testing
type withdrawalMockRow struct {
	err error
}

func (r *withdrawalMockRow) Scan(dest ...any) error {
	return r.err
}

// withdrawalMockRows implements pgx.Rows for testing
type withdrawalMockRows struct {
	rows    [][]any
	current int
	closed  bool
	err     error
}

func (r *withdrawalMockRows) Scan(dest ...any) error {
	if r.current >= len(r.rows) {
		return pgx.ErrNoRows
	}
	row := r.rows[r.current]
	r.current++

	if len(row) != len(dest) {
		return fmt.Errorf("scan argument count mismatch: have %d, want %d", len(row), len(dest))
	}
	for i, v := range row {
		switch d := dest[i].(type) {
		case *uuid.UUID:
			if id, ok := v.(uuid.UUID); ok {
				*d = id
			}
		case *int64:
			if val, ok := v.(int64); ok {
				*d = val
			}
		case *string:
			if s, ok := v.(string); ok {
				*d = s
			}
		case *float64:
			if f, ok := v.(float64); ok {
				*d = f
			}
		case *time.Time:
			if t, ok := v.(time.Time); ok {
				*d = t
			}
		default:
			return fmt.Errorf("unsupported type %T for scan", d)
		}
	}
	return nil
}

func (r *withdrawalMockRows) Next() bool {
	return r.current < len(r.rows)
}

func (r *withdrawalMockRows) Err() error {
	return r.err
}

func (r *withdrawalMockRows) Close() {
	r.closed = true
}

func (r *withdrawalMockRows) CommandTag() pgconn.CommandTag {
	return pgconn.NewCommandTag("0")
}

func (r *withdrawalMockRows) Fields() []pgconn.FieldDescription {
	return nil
}

func (r *withdrawalMockRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (r *withdrawalMockRows) RawValues() [][]byte {
	return nil
}

func (r *withdrawalMockRows) Values() ([]any, error) {
	return nil, nil
}

func (r *withdrawalMockRows) Conn() *pgx.Conn {
	return nil
}

// withdrawalMockDB implements the Transactor interface for testing
type withdrawalMockDB struct {
	WithTxFunc func(ctx context.Context, fn func(tx db.Tx) error) error
}

func (m *withdrawalMockDB) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	if m.WithTxFunc != nil {
		return m.WithTxFunc(ctx, fn)
	}
	return fn(&withdrawalMockTx{})
}

// withdrawalMockAdminNotifier implements AdminAlertNotifier for testing
type withdrawalMockAdminNotifier struct {
	notifiedWithdrawals []StuckWithdrawal
	notifyCount         int
	notifyError         error
}

func (m *withdrawalMockAdminNotifier) NotifyWithdrawalStuck(ctx context.Context, stuck []StuckWithdrawal) error {
	m.notifyCount++
	m.notifiedWithdrawals = append(m.notifiedWithdrawals, stuck...)
	return m.notifyError
}

// TestNewWithdrawalMonitoringWorker tests creating a new WithdrawalMonitoringWorker.
func TestNewWithdrawalMonitoringWorker(t *testing.T) {
	log := zaptest.NewLogger(t)

	t.Run("with default config", func(t *testing.T) {
		worker := NewWithdrawalMonitoringWorker(nil, log, DefaultWithdrawalMonitoringConfig())

		if worker == nil {
			t.Fatal("NewWithdrawalMonitoringWorker() returned nil")
		}

		if worker.cfg.Interval != DefaultWithdrawalMonitoringInterval {
			t.Errorf("Interval = %v, want %v", worker.cfg.Interval, DefaultWithdrawalMonitoringInterval)
		}

		if worker.cfg.StuckThreshold != DefaultWithdrawalStuckThreshold {
			t.Errorf("StuckThreshold = %v, want %v", worker.cfg.StuckThreshold, DefaultWithdrawalStuckThreshold)
		}
	})

	t.Run("with custom config", func(t *testing.T) {
		customCfg := WithdrawalMonitoringConfig{
			Interval:       10 * time.Minute,
			StuckThreshold: 48 * time.Hour,
		}

		worker := NewWithdrawalMonitoringWorker(nil, log, customCfg)

		if worker.cfg.Interval != 10*time.Minute {
			t.Errorf("Interval = %v, want %v", worker.cfg.Interval, 10*time.Minute)
		}

		if worker.cfg.StuckThreshold != 48*time.Hour {
			t.Errorf("StuckThreshold = %v, want %v", worker.cfg.StuckThreshold, 48*time.Hour)
		}
	})

	t.Run("with zero config values uses defaults", func(t *testing.T) {
		cfg := WithdrawalMonitoringConfig{
			Interval:       0,
			StuckThreshold: 0,
		}

		worker := NewWithdrawalMonitoringWorker(nil, log, cfg)

		if worker.cfg.Interval != DefaultWithdrawalMonitoringInterval {
			t.Errorf("Interval = %v, want %v", worker.cfg.Interval, DefaultWithdrawalMonitoringInterval)
		}

		if worker.cfg.StuckThreshold != DefaultWithdrawalStuckThreshold {
			t.Errorf("StuckThreshold = %v, want %v", worker.cfg.StuckThreshold, DefaultWithdrawalStuckThreshold)
		}
	})
}

// TestWithdrawalMonitoringWorker_StartStop tests starting and stopping the worker.
func TestWithdrawalMonitoringWorker_StartStop(t *testing.T) {
	log := zaptest.NewLogger(t)
	cfg := WithdrawalMonitoringConfig{
		Interval:       10 * time.Millisecond,
		StuckThreshold: 24 * time.Hour,
	}

	// Mock DB that returns no stuck withdrawals
	mockDB := &withdrawalMockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
			tx := &withdrawalMockTx{
				QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
					return &withdrawalMockRows{rows: [][]any{}}, nil
				},
			}
			return fn(tx)
		},
	}

	worker := NewWithdrawalMonitoringWorker(mockDB, log, cfg)

	if worker.IsRunning() {
		t.Error("Worker should not be running initially")
	}

	worker.Start()

	if !worker.IsRunning() {
		t.Error("Worker should be running after Start()")
	}

	// Wait a bit for the worker to run
	time.Sleep(50 * time.Millisecond)

	worker.Stop()

	if worker.IsRunning() {
		t.Error("Worker should not be running after Stop()")
	}

	// Starting again should work
	worker.Start()
	if !worker.IsRunning() {
		t.Error("Worker should be running after restart")
	}
	worker.Stop()
}

// TestWithdrawalMonitoringWorker_RunOnce tests the RunOnce method.
func TestWithdrawalMonitoringWorker_RunOnce(t *testing.T) {
	log := zaptest.NewLogger(t)
	now := time.Now().UTC()
	sellerID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440001")
	withdrawalID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440002")

	t.Run("no stuck withdrawals", func(t *testing.T) {
		mockDB := &withdrawalMockDB{
			WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
				tx := &withdrawalMockTx{
					QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
						// Return empty result set
						return &withdrawalMockRows{rows: [][]any{}}, nil
					},
				}
				return fn(tx)
			},
		}

		worker := NewWithdrawalMonitoringWorker(mockDB, log, DefaultWithdrawalMonitoringConfig())
		stuck, err := worker.RunOnce()

		if err != nil {
			t.Errorf("RunOnce() error = %v", err)
		}

		if len(stuck) != 0 {
			t.Errorf("RunOnce() stuck count = %d, want 0", len(stuck))
		}
	})

	t.Run("finds stuck processing withdrawals", func(t *testing.T) {
		mockDB := &withdrawalMockDB{
			WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
				tx := &withdrawalMockTx{
					QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
						status, _ := args[0].(string)
						if status != WithdrawalStatusProcessing {
							return &withdrawalMockRows{rows: [][]any{}}, nil
						}
						rows := &withdrawalMockRows{
							rows: [][]any{
								{
									withdrawalID,
									sellerID,
									int64(100000), // amount in cents
									"PROCESSING",
									now.Add(-48 * time.Hour), // created_at
									now.Add(-26 * time.Hour), // updated_at (over 24h ago)
									26.0,                     // hours_since_update
								},
							},
						}
						return rows, nil
					},
				}
				return fn(tx)
			},
		}

		worker := NewWithdrawalMonitoringWorker(mockDB, log, DefaultWithdrawalMonitoringConfig())
		stuck, err := worker.RunOnce()

		if err != nil {
			t.Errorf("RunOnce() error = %v", err)
		}

		if len(stuck) != 1 {
			t.Errorf("RunOnce() stuck count = %d, want 1", len(stuck))
		}

		if stuck[0].Status != "PROCESSING" {
			t.Errorf("RunOnce() status = %s, want PROCESSING", stuck[0].Status)
		}

		if stuck[0].Amount != 100000 {
			t.Errorf("RunOnce() amount = %d, want 100000", stuck[0].Amount)
		}

		if stuck[0].SellerID != sellerID {
			t.Errorf("RunOnce() seller_id = %s, want %s", stuck[0].SellerID, sellerID)
		}
	})

	t.Run("finds both processing and requested stuck withdrawals", func(t *testing.T) {
		requestedID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440003")

		queryCount := 0
		mockDB := &withdrawalMockDB{
			WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
				tx := &withdrawalMockTx{
					QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
						queryCount++
						// First query for PROCESSING, second for REQUESTED
						if queryCount == 1 {
							return &withdrawalMockRows{
								rows: [][]any{
									{
										withdrawalID,
										sellerID,
										int64(50000),
										"PROCESSING",
										now.Add(-30 * time.Hour),
										now.Add(-25 * time.Hour),
										25.0,
									},
								},
							}, nil
						}
						return &withdrawalMockRows{
							rows: [][]any{
								{
									requestedID,
									sellerID,
									int64(75000),
									"REQUESTED",
										now.Add(-30 * time.Hour),
										now.Add(-26 * time.Hour),
										26.0,
								},
							},
						}, nil
					},
				}
				return fn(tx)
			},
		}

		worker := NewWithdrawalMonitoringWorker(mockDB, log, DefaultWithdrawalMonitoringConfig())
		stuck, err := worker.RunOnce()

		if err != nil {
			t.Errorf("RunOnce() error = %v", err)
		}

		if len(stuck) != 2 {
			t.Errorf("RunOnce() stuck count = %d, want 2", len(stuck))
		}
	})

	t.Run("handles database errors gracefully", func(t *testing.T) {
		mockDB := &withdrawalMockDB{
			WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
				tx := &withdrawalMockTx{
					QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
						return nil, errors.New("database connection failed")
					},
				}
				return fn(tx)
			},
		}

		worker := NewWithdrawalMonitoringWorker(mockDB, log, DefaultWithdrawalMonitoringConfig())
		stuck, err := worker.RunOnce()

		if err != nil {
			t.Errorf("RunOnce() error = %v, want nil", err)
		}
		if len(stuck) != 0 {
			t.Errorf("RunOnce() stuck count = %d, want 0 on query failure", len(stuck))
		}
	})
}

// TestWithdrawalMonitoringWorker_GetStats tests the GetStats method.
func TestWithdrawalMonitoringWorker_GetStats(t *testing.T) {
	log := zaptest.NewLogger(t)
	now := time.Now().UTC()
	sellerID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440001")
	processingID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440002")
	requestedID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440003")

	mockDB := &withdrawalMockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
			tx := &withdrawalMockTx{
				QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
					status, _ := args[0].(string)
					switch status {
					case WithdrawalStatusProcessing:
						return &withdrawalMockRows{
							rows: [][]any{
								{processingID, sellerID, int64(100000), "PROCESSING", now.Add(-48 * time.Hour), now.Add(-30 * time.Hour), 30.0},
							},
						}, nil
					case WithdrawalStatusRequested:
						return &withdrawalMockRows{
							rows: [][]any{
								{requestedID, sellerID, int64(50000), "REQUESTED", now.Add(-48 * time.Hour), now.Add(-26 * time.Hour), 26.0},
							},
						}, nil
					default:
						return &withdrawalMockRows{rows: [][]any{}}, nil
					}
				},
			}
			return fn(tx)
		},
	}

	worker := NewWithdrawalMonitoringWorker(mockDB, log, DefaultWithdrawalMonitoringConfig())
	stats, err := worker.GetStats(context.Background())

	if err != nil {
		t.Errorf("GetStats() error = %v", err)
	}

	if stats.TotalStuck != 2 {
		t.Errorf("GetStats() TotalStuck = %d, want 2", stats.TotalStuck)
	}

	if stats.TotalStuckProcessing != 1 {
		t.Errorf("GetStats() TotalStuckProcessing = %d, want 1", stats.TotalStuckProcessing)
	}

	if stats.TotalStuckRequested != 1 {
		t.Errorf("GetStats() TotalStuckRequested = %d, want 1", stats.TotalStuckRequested)
	}

	if stats.TotalAmountStuck != 150000 {
		t.Errorf("GetStats() TotalAmountStuck = %d, want 150000", stats.TotalAmountStuck)
	}

	if stats.LongestStuckHours != 30.0 {
		t.Errorf("GetStats() LongestStuckHours = %f, want 30.0", stats.LongestStuckHours)
	}
}

// TestWithdrawalMonitoringWorker_NotifyAdminDashboard tests admin notification.
func TestWithdrawalMonitoringWorker_NotifyAdminDashboard(t *testing.T) {
	log := zaptest.NewLogger(t)
	now := time.Now().UTC()
	sellerID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440001")
	withdrawalID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440002")

	mockDB := &withdrawalMockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
			tx := &withdrawalMockTx{
				QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
					status, _ := args[0].(string)
					if status != WithdrawalStatusProcessing {
						return &withdrawalMockRows{rows: [][]any{}}, nil
					}
					return &withdrawalMockRows{
						rows: [][]any{
							{withdrawalID, sellerID, int64(100000), "PROCESSING", now.Add(-48 * time.Hour), now.Add(-30 * time.Hour), 30.0},
						},
					}, nil
				},
			}
			return fn(tx)
		},
	}

	notifier := &withdrawalMockAdminNotifier{}

	worker := NewWithdrawalMonitoringWorker(mockDB, log, DefaultWithdrawalMonitoringConfig())
	err := worker.NotifyAdminDashboard(notifier)

	if err != nil {
		t.Errorf("NotifyAdminDashboard() error = %v", err)
	}

	if notifier.notifyCount != 1 {
		t.Errorf("NotifyAdminDashboard() notify count = %d, want 1", notifier.notifyCount)
	}

	if len(notifier.notifiedWithdrawals) != 1 {
		t.Errorf("NotifyAdminDashboard() notified withdrawals = %d, want 1", len(notifier.notifiedWithdrawals))
	}

	if notifier.notifiedWithdrawals[0].ID != withdrawalID {
		t.Errorf("NotifyAdminDashboard() withdrawal ID = %s, want %s", notifier.notifiedWithdrawals[0].ID, withdrawalID)
	}
}

// TestWithdrawalMonitoringWorker_HealthCheck tests the HealthCheck method.
func TestWithdrawalMonitoringWorker_HealthCheck(t *testing.T) {
	log := zaptest.NewLogger(t)

	t.Run("healthy when no stuck withdrawals", func(t *testing.T) {
		mockDB := &withdrawalMockDB{
			WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
				tx := &withdrawalMockTx{
					QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
						return &withdrawalMockRows{rows: [][]any{}}, nil
					},
				}
				return fn(tx)
			},
		}

		worker := NewWithdrawalMonitoringWorker(mockDB, log, DefaultWithdrawalMonitoringConfig())
		err := worker.HealthCheck(context.Background())

		if err != nil {
			t.Errorf("HealthCheck() should return nil when healthy, got: %v", err)
		}
	})

	t.Run("unhealthy when stuck withdrawals exist", func(t *testing.T) {
		now := time.Now().UTC()
		sellerID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440001")
		withdrawalID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440002")

		mockDB := &withdrawalMockDB{
			WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
				tx := &withdrawalMockTx{
					QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
						return &withdrawalMockRows{
							rows: [][]any{
								{withdrawalID, sellerID, int64(100000), "PROCESSING", now.Add(-48 * time.Hour), now.Add(-30 * time.Hour), 30.0},
							},
						}, nil
					},
				}
				return fn(tx)
			},
		}

		worker := NewWithdrawalMonitoringWorker(mockDB, log, DefaultWithdrawalMonitoringConfig())
		err := worker.HealthCheck(context.Background())

		if err == nil {
			t.Error("HealthCheck() should return error when stuck withdrawals exist")
		}
	})
}

// TestDefaultWithdrawalMonitoringConfig tests the default config.
func TestDefaultWithdrawalMonitoringConfig(t *testing.T) {
	cfg := DefaultWithdrawalMonitoringConfig()

	if cfg.Interval != DefaultWithdrawalMonitoringInterval {
		t.Errorf("DefaultWithdrawalMonitoringConfig() Interval = %v, want %v",
			cfg.Interval, DefaultWithdrawalMonitoringInterval)
	}

	if cfg.StuckThreshold != DefaultWithdrawalStuckThreshold {
		t.Errorf("DefaultWithdrawalMonitoringConfig() StuckThreshold = %v, want %v",
			cfg.StuckThreshold, DefaultWithdrawalStuckThreshold)
	}
}


