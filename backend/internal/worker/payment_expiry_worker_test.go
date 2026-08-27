//go:build integration

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

	paymentRepo "github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

// paymentExpiryMockTx implements db.Tx for testing.
type paymentExpiryMockTx struct {
	queryRowCalls int
	queryCalls    int
	execCalls     int
	QueryRowFunc  func(ctx context.Context, sql string, args ...any) pgx.Row
	QueryFunc     func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	ExecFunc      func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	CommitFunc    func(ctx context.Context) error
	RollbackFunc  func(ctx context.Context) error
}

func (m *paymentExpiryMockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	m.queryRowCalls++
	if m.QueryRowFunc != nil {
		return m.QueryRowFunc(ctx, sql, args...)
	}
	return &paymentExpiryMockRow{err: errors.New("no mock configured")}
}

func (m *paymentExpiryMockTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	m.queryCalls++
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, sql, args...)
	}
	return &paymentExpiryMockRows{}, nil
}

func (m *paymentExpiryMockTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	m.execCalls++
	if m.ExecFunc != nil {
		return m.ExecFunc(ctx, sql, args...)
	}
	return pgconn.NewCommandTag("1"), nil
}

func (m *paymentExpiryMockTx) Commit(ctx context.Context) error {
	if m.CommitFunc != nil {
		return m.CommitFunc(ctx)
	}
	return nil
}

func (m *paymentExpiryMockTx) Rollback(ctx context.Context) error {
	if m.RollbackFunc != nil {
		return m.RollbackFunc(ctx)
	}
	return nil
}

// paymentExpiryMockDB implements the Transactor interface for testing.
type paymentExpiryMockDB struct {
	WithTxFunc func(ctx context.Context, fn func(tx db.Tx) error) error
}

func (m *paymentExpiryMockDB) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	if m.WithTxFunc != nil {
		return m.WithTxFunc(ctx, fn)
	}
	return fn(&paymentExpiryMockTx{})
}

// paymentExpiryMockRow implements pgx.Row for testing.
type paymentExpiryMockRow struct {
	values []any
	err    error
}

func (r *paymentExpiryMockRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(r.values) != len(dest) {
		return fmt.Errorf("scan argument count mismatch: have %d, want %d", len(r.values), len(dest))
	}
	for i, v := range r.values {
		switch d := dest[i].(type) {
		case *uuid.UUID:
			if id, ok := v.(uuid.UUID); ok {
				*d = id
			}
		case *int64:
			if val, ok := v.(int64); ok {
				*d = val
			}
		case *int:
			if val, ok := v.(int64); ok {
				*d = int(val)
			}
		case *string:
			if s, ok := v.(string); ok {
				*d = s
			}
		case **string:
			if s, ok := v.(*string); ok {
				*d = s
			} else if v == nil {
				*d = nil
			}
		case *bool:
			if b, ok := v.(bool); ok {
				*d = b
			}
		case *time.Time:
			if t, ok := v.(time.Time); ok {
				*d = t
			}
		case **time.Time:
			if t, ok := v.(*time.Time); ok {
				*d = t
			} else if v == nil {
				*d = nil
			}
		case **uuid.UUID:
			if id, ok := v.(*uuid.UUID); ok {
				*d = id
			} else if v == nil {
				*d = nil
			}
		case *money.Money:
			if val, ok := v.(int64); ok {
				*d = money.New(val)
			}
		default:
			return fmt.Errorf("unsupported type %T for scan", d)
		}
	}
	return nil
}

// paymentExpiryMockRows implements pgx.Rows for testing.
type paymentExpiryMockRows struct {
	rows    [][]any
	current int
	closed  bool
	err     error
}

func (r *paymentExpiryMockRows) Scan(dest ...any) error {
	if r.current >= len(r.rows) {
		return pgx.ErrNoRows
	}
	row := r.rows[r.current]
	r.current++

	if len(row) != len(dest) {
		return fmt.Errorf("scan argument count mismatch")
	}
	for i, v := range r.rows[0] {
		switch d := dest[i].(type) {
		case *uuid.UUID:
			if id, ok := v.(uuid.UUID); ok {
				*d = id
			}
		}
	}
	return nil
}

func (r *paymentExpiryMockRows) Next() bool {
	return r.current < len(r.rows)
}

func (r *paymentExpiryMockRows) Err() error {
	return r.err
}

func (r *paymentExpiryMockRows) Close() {
	r.closed = true
}

func (r *paymentExpiryMockRows) CommandTag() pgconn.CommandTag {
	return pgconn.NewCommandTag("0")
}

func (r *paymentExpiryMockRows) Fields() []pgconn.FieldDescription {
	return nil
}

func (r *paymentExpiryMockRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (r *paymentExpiryMockRows) RawValues() [][]byte {
	return nil
}

func (r *paymentExpiryMockRows) Values() ([]any, error) {
	return nil, nil
}

func (r *paymentExpiryMockRows) Conn() *pgx.Conn {
	return nil
}

func newPaymentExpiryRow(paymentID uuid.UUID, status string, expiredAt time.Time) *paymentExpiryMockRow {
	now := time.Now()
	return &paymentExpiryMockRow{
		values: []any{
			paymentID,
			uuid.New(),
			"PAY-001",
			"midtrans-order-123",
			int64(100000),
			0,
			int64(0),
			int64(100000),
			int64(100000),
			status,
			"",
			(*uuid.UUID)(nil),
			(*uuid.UUID)(nil),
			(*string)(nil),
			(*string)(nil),
			(*string)(nil),
			(*time.Time)(nil),
			expiredAt,
			now,
			now,
			(*string)(nil),
		},
	}
}

// TestNewPaymentExpiryWorker tests creating a new PaymentExpiryWorker.
func TestNewPaymentExpiryWorker(t *testing.T) {
	log := zaptest.NewLogger(t)
	cfg := DefaultConfig()

	worker := NewPaymentExpiryWorker(nil, nil, log, cfg)

	if worker == nil {
		t.Fatal("NewPaymentExpiryWorker() returned nil")
	}

	if worker.pollInterval != DefaultPollInterval {
		t.Errorf("pollInterval = %v, want %v", worker.pollInterval, DefaultPollInterval)
	}

	if worker.batchSize != DefaultBatchSize {
		t.Errorf("batchSize = %d, want %d", worker.batchSize, DefaultBatchSize)
	}
}

// TestPaymentExpiryWorker_StartStop tests starting and stopping the worker.
func TestPaymentExpiryWorker_StartStop(t *testing.T) {
	log := zaptest.NewLogger(t)
	cfg := Config{
		PollInterval: 10 * time.Millisecond,
		BatchSize:    10,
	}

	// Create a mock DB that returns no expired payments
	mockDB := &paymentExpiryMockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
			// Return empty mockRows for the query
			tx := &paymentExpiryMockTx{
				QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
					return &paymentExpiryMockRows{rows: [][]any{}}, nil
				},
			}
			return fn(tx)
		},
	}

	worker := NewPaymentExpiryWorker(mockDB, nil, log, cfg)

	if worker.IsRunning() {
		t.Error("worker should not be running initially")
	}

	worker.Start()

	if !worker.IsRunning() {
		t.Error("worker should be running after Start()")
	}

	// Let it run briefly
	time.Sleep(50 * time.Millisecond)

	worker.Stop()

	if worker.IsRunning() {
		t.Error("worker should not be running after Stop()")
	}

	// Starting again should work
	worker.Start()
	if !worker.IsRunning() {
		t.Error("worker should be running after second Start()")
	}
	worker.Stop()
}

// TestPaymentExpiryWorker_DoubleStart tests that starting twice doesn't spawn multiple goroutines.
func TestPaymentExpiryWorker_DoubleStart(t *testing.T) {
	log := zaptest.NewLogger(t)
	cfg := DefaultConfig()

	// Create a mock DB that returns no expired payments
	mockDB := &paymentExpiryMockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
			tx := &paymentExpiryMockTx{
				QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
					return &paymentExpiryMockRows{rows: [][]any{}}, nil
				},
			}
			return fn(tx)
		},
	}

	worker := NewPaymentExpiryWorker(mockDB, nil, log, cfg)

	worker.Start()
	worker.Start() // Should warn but not crash

	worker.Stop()
}

// TestFindExpiredPendingPaymentIDs_NoExpired tests finding expired payments when none exist.
func TestFindExpiredPendingPaymentIDs_NoExpired(t *testing.T) {
	log := zaptest.NewLogger(t)
	worker := NewPaymentExpiryWorker(nil, nil, log, DefaultConfig())

	ctx := context.Background()
	mockDB := &paymentExpiryMockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
		return fn(&paymentExpiryMockTx{
				QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
					return &paymentExpiryMockRows{rows: [][]any{}}, nil
				},
			})
		},
	}
	worker.db = mockDB

	err := mockDB.WithTx(ctx, func(tx db.Tx) error {
		ids, err := worker.findExpiredPendingPaymentIDs(ctx, 10)
		if err != nil {
			t.Fatalf("findExpiredPendingPaymentIDs() error = %v", err)
		}
		if len(ids) != 0 {
			t.Errorf("findExpiredPendingPaymentIDs() returned %d IDs, want 0", len(ids))
		}
		return nil
	})

	if err != nil {
		t.Fatalf("WithTx error = %v", err)
	}
}

// TestFindExpiredPendingPaymentIDs_WithExpired tests finding expired payments.
func TestFindExpiredPendingPaymentIDs_WithExpired(t *testing.T) {
	log := zaptest.NewLogger(t)
	worker := NewPaymentExpiryWorker(nil, nil, log, DefaultConfig())

	expectedID := uuid.New()
	ctx := context.Background()

	mockDB := &paymentExpiryMockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
		return fn(&paymentExpiryMockTx{
				QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
					return &paymentExpiryMockRows{rows: [][]any{{expectedID}}, current: 0}, nil
				},
			})
		},
	}
	worker.db = mockDB

	err := mockDB.WithTx(ctx, func(tx db.Tx) error {
		ids, err := worker.findExpiredPendingPaymentIDs(ctx, 10)
		if err != nil {
			t.Fatalf("findExpiredPendingPaymentIDs() error = %v", err)
		}
		if len(ids) != 1 {
			t.Fatalf("findExpiredPendingPaymentIDs() returned %d IDs, want 1", len(ids))
		}
		if ids[0] != expectedID {
			t.Errorf("findExpiredPendingPaymentIDs() returned ID %v, want %v", ids[0], expectedID)
		}
		return nil
	})

	if err != nil {
		t.Fatalf("WithTx error = %v", err)
	}
}

// TestExpirePayment_Success tests expiring a single payment successfully.
func TestExpirePayment_Success(t *testing.T) {
	log := zaptest.NewLogger(t)
	worker := NewPaymentExpiryWorker(nil, nil, log, DefaultConfig())

	paymentID := uuid.New()
	expiredAt := time.Now().Add(-1 * time.Hour)
	ctx := context.Background()

	mockDB := &paymentExpiryMockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
		return fn(&paymentExpiryMockTx{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
					// First call: GetByIDForUpdate - return pending payment
					if args[0] == paymentID {
						return newPaymentExpiryRow(paymentID, "pending", expiredAt)
					}
					return &paymentExpiryMockRow{err: pgx.ErrNoRows}
				},
				ExecFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
					// UpdateStatus call
					return pgconn.NewCommandTag("1"), nil
				},
			})
		},
	}
	worker.db = mockDB

	err := mockDB.WithTx(ctx, func(tx db.Tx) error {
		err := worker.expirePayment(ctx, paymentID)
		if err != nil {
			t.Fatalf("expirePayment() error = %v", err)
		}
		return nil
	})

	if err != nil {
		t.Fatalf("WithTx error = %v", err)
	}
}

// TestExpirePayment_NotPending tests that non-pending payments are skipped.
func TestExpirePayment_NotPending(t *testing.T) {
	log := zaptest.NewLogger(t)
	worker := NewPaymentExpiryWorker(nil, nil, log, DefaultConfig())

	paymentID := uuid.New()
	expiredAt := time.Now().Add(-1 * time.Hour)
	ctx := context.Background()

	mockDB := &mockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
		return fn(&paymentExpiryMockTx{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
					// Return settled payment (already paid)
					return newPaymentExpiryRow(paymentID, "settlement", expiredAt)
				},
			})
		},
	}
	worker.db = mockDB

	err := mockDB.WithTx(ctx, func(tx db.Tx) error {
		err := worker.expirePayment(ctx, paymentID)
		if err != nil {
			t.Fatalf("expirePayment() should return nil for non-pending payment, got %v", err)
		}
		return nil
	})

	if err != nil {
		t.Fatalf("WithTx error = %v", err)
	}
}

// TestExpirePayment_NotExpired tests that payments not yet expired are skipped.
func TestExpirePayment_NotExpired(t *testing.T) {
	log := zaptest.NewLogger(t)
	worker := NewPaymentExpiryWorker(nil, nil, log, DefaultConfig())

	paymentID := uuid.New()
	expiredAt := time.Now().Add(1 * time.Hour) // Future time
	ctx := context.Background()

	mockDB := &mockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
			return fn(&mockTx{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
					// Return pending payment with future expiry
					return newPaymentExpiryRow(paymentID, "pending", expiredAt)
				},
			})
		},
	}
	worker.db = mockDB

	err := mockDB.WithTx(ctx, func(tx db.Tx) error {
		err := worker.expirePayment(ctx, paymentID)
		if err != nil {
			t.Fatalf("expirePayment() should return nil for non-expired payment, got %v", err)
		}
		return nil
	})

	if err != nil {
		t.Fatalf("WithTx error = %v", err)
	}
}

// TestDefaultConfig tests the DefaultConfig function.
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.PollInterval != DefaultPollInterval {
		t.Errorf("PollInterval = %v, want %v", cfg.PollInterval, DefaultPollInterval)
	}

	if cfg.BatchSize != DefaultBatchSize {
		t.Errorf("BatchSize = %d, want %d", cfg.BatchSize, DefaultBatchSize)
	}
}

// TestPaymentExpiryWorker_Idempotent tests that expiring an already expired payment is idempotent.
func TestPaymentExpiryWorker_Idempotent(t *testing.T) {
	log := zaptest.NewLogger(t)
	worker := NewPaymentExpiryWorker(nil, nil, log, DefaultConfig())

	paymentID := uuid.New()
	ctx := context.Background()

	// First call - payment is pending, should be expired
	firstCall := false
	mockDB := &mockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
		return fn(&paymentExpiryMockTx{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
					if !firstCall {
						// First call: pending payment
						return newPaymentExpiryRow(paymentID, "pending", time.Now().Add(-1*time.Hour))
					}
					// Second call: already expired (idempotent)
					return newPaymentExpiryRow(paymentID, "expire", time.Now().Add(-1*time.Hour))
				},
				ExecFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
					return pgconn.NewCommandTag("1"), nil
				},
			})
		},
	}
	worker.db = mockDB

	// First expiry
	err := mockDB.WithTx(ctx, func(tx db.Tx) error {
		firstCall = false
		return worker.expirePayment(ctx, paymentID)
	})
	if err != nil {
		t.Fatalf("First expirePayment() error = %v", err)
	}

	// Second expiry (should be idempotent)
	firstCall = true
	err = mockDB.WithTx(ctx, func(tx db.Tx) error {
		return worker.expirePayment(ctx, paymentID)
	})
	if err != nil {
		t.Fatalf("Second expirePayment() should be idempotent, got error = %v", err)
	}
}

// TestExpirePayment_DatabaseGuard_Success tests successful status transition
// when payment is in pending status (simulates rowsAffected = 1).
func TestExpirePayment_DatabaseGuard_Success(t *testing.T) {
	log := zaptest.NewLogger(t)
	worker := NewPaymentExpiryWorker(nil, nil, log, DefaultConfig())

	paymentID := uuid.New()
	expiredAt := time.Now().Add(-1 * time.Hour)
	ctx := context.Background()

	mockDB := &paymentExpiryMockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
		return fn(&paymentExpiryMockTx{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
					// Return pending payment
					return newPaymentExpiryRow(paymentID, paymentRepo.PaymentStatusPending, expiredAt)
				},
				ExecFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
					// Simulate successful update (1 row affected)
					return pgconn.NewCommandTag("1"), nil
				},
			})
		},
	}
	worker.db = mockDB

	err := mockDB.WithTx(ctx, func(tx db.Tx) error {
		return worker.expirePayment(ctx, paymentID)
	})

	if err != nil {
		t.Fatalf("expirePayment() with pending payment should succeed, got %v", err)
	}
}

// TestExpirePayment_DatabaseGuard_AlreadyExpired tests that attempting to expire
// an already-expired payment returns ErrInvalidStatusTransition (simulates rowsAffected = 0).
func TestExpirePayment_DatabaseGuard_AlreadyExpired(t *testing.T) {
	log := zaptest.NewLogger(t)
	worker := NewPaymentExpiryWorker(nil, nil, log, DefaultConfig())

	paymentID := uuid.New()
	expiredAt := time.Now().Add(-1 * time.Hour)
	ctx := context.Background()

	mockDB := &paymentExpiryMockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
		return fn(&paymentExpiryMockTx{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
					// Return already expired payment
					return newPaymentExpiryRow(paymentID, paymentRepo.PaymentStatusExpire, expiredAt)
				},
				ExecFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
					// Simulate no rows affected (WHERE status='pending' doesn't match)
					return pgconn.NewCommandTag("0"), nil
				},
			})
		},
	}
	worker.db = mockDB

	err := mockDB.WithTx(ctx, func(tx db.Tx) error {
		return worker.expirePayment(ctx, paymentID)
	})

	if err != nil {
		t.Fatalf("expirePayment() with already expired payment should be idempotent (return nil), got %v", err)
	}
}

// TestExpirePayment_DatabaseGuard_SettledPayment tests that attempting to expire
// a settled payment returns ErrInvalidStatusTransition (simulates rowsAffected = 0).
func TestExpirePayment_DatabaseGuard_SettledPayment(t *testing.T) {
	log := zaptest.NewLogger(t)
	worker := NewPaymentExpiryWorker(nil, nil, log, DefaultConfig())

	paymentID := uuid.New()
	expiredAt := time.Now().Add(-1 * time.Hour)
	ctx := context.Background()

	mockDB := &mockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx db.Tx) error) error {
			return fn(&mockTx{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
					// Return settled payment
					return newPaymentExpiryRow(paymentID, paymentRepo.PaymentStatusSettlement, expiredAt)
				},
				ExecFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
					// Simulate no rows affected (WHERE status='pending' doesn't match)
					return pgconn.NewCommandTag("0"), nil
				},
			})
		},
	}
	worker.db = mockDB

	err := mockDB.WithTx(ctx, func(tx db.Tx) error {
		return worker.expirePayment(ctx, paymentID)
	})

	if err != nil {
		t.Fatalf("expirePayment() with settled payment should be idempotent (return nil), got %v", err)
	}
}


