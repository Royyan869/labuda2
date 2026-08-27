package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	ledgerrepo "github.com/labuda/backend/internal/finance/repository"
	"github.com/labuda/backend/pkg/money"
)

// TestDbTx is a mock that implements db.Tx interface for testing.
// It uses function callbacks to return controlled results.
type TestDbTx struct {
	QueryRowFunc func(ctx context.Context, sql string, args ...any) pgx.Row
	QueryFunc    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	ExecFunc     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	CommitFunc   func(ctx context.Context) error
	RollbackFunc func(ctx context.Context) error
}

func (m *TestDbTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.QueryRowFunc != nil {
		return m.QueryRowFunc(ctx, sql, args...)
	}
	return &mockRow{err: errors.New("no mock configured")}
}

func (m *TestDbTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, sql, args...)
	}
	return &mockRows{}, nil
}

func (m *TestDbTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if m.ExecFunc != nil {
		return m.ExecFunc(ctx, sql, args...)
	}
	return pgconn.NewCommandTag("1"), nil
}

func (m *TestDbTx) Commit(ctx context.Context) error {
	if m.CommitFunc != nil {
		return m.CommitFunc(ctx)
	}
	return nil
}

func (m *TestDbTx) Rollback(ctx context.Context) error {
	if m.RollbackFunc != nil {
		return m.RollbackFunc(ctx)
	}
	return nil
}

// mockRow implements pgx.Row for testing
type mockRow struct {
	values []any
	err    error
}

func (r *mockRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(r.values) != len(dest) {
		return errors.New("scan argument count mismatch")
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
		}
	}
	return nil
}

// mockRows implements pgx.Rows for testing
type mockRows struct {
	nextFunc func() bool
	scanFunc func(dest ...any) error
	errFunc  func() error
}

func (r *mockRows) Next() bool {
	if r.nextFunc != nil {
		return r.nextFunc()
	}
	return false
}

func (r *mockRows) Scan(dest ...any) error {
	if r.scanFunc != nil {
		return r.scanFunc(dest...)
	}
	return nil
}

func (r *mockRows) Close() {
	// pgx.Rows Close() has no return value
}

func (r *mockRows) Err() error {
	if r.errFunc != nil {
		return r.errFunc()
	}
	return nil
}

func (r *mockRows) CommandTag() pgconn.CommandTag { return pgconn.NewCommandTag("") }
func (r *mockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *mockRows) Values() ([]any, error) { return nil, nil }
func (r *mockRows) Conn() *pgx.Conn { return nil } // Required by pgx.Rows interface
func (r *mockRows) RawValues() [][]byte { return nil }

func TestNewLedgerRepository(t *testing.T) {
	repo := NewLedgerRepository()
	if repo == nil {
		t.Error("NewLedgerRepository() returned nil")
	}
}

func TestLedgerRepository_CreateTransaction_Unbalanced(t *testing.T) {
	repo := NewLedgerRepository()
	ctx := context.Background()

	account1 := uuid.New()
	account2 := uuid.New()

	// Unbalanced transaction: debit 1000, credit 500
	entries := []ledgerrepo.Entry{
		{AccountID: account1, Amount: money.New(1000)}, // debit
		{AccountID: account2, Amount: money.New(-500)}, // credit
	}

	mockedTx := &TestDbTx{}

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for unbalanced transaction")
		}
	}()

	_ = repo.CreateTransaction(ctx, mockedTx, "test-key-2", "payment", uuid.New(), nil, nil, entries)
}

func TestLedgerRepository_CreateTransaction_Idempotent(t *testing.T) {
	repo := NewLedgerRepository()
	ctx := context.Background()

	account1 := uuid.New()
	account2 := uuid.New()

	entries := []ledgerrepo.Entry{
		{AccountID: account1, Amount: money.New(1000)},
		{AccountID: account2, Amount: money.New(-1000)},
	}

	existingID := uuid.New()
	mockedTx := &TestDbTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &mockRow{values: []any{existingID}}
		},
	}

	err := repo.CreateTransaction(ctx, mockedTx, "existing-key", "payment", uuid.New(), nil, nil, entries)
	if err != nil {
		t.Errorf("Expected nil for idempotent call, got %v", err)
	}
}

func TestLedgerRepository_CreateTransaction_EmptyEntries(t *testing.T) {
	repo := NewLedgerRepository()
	ctx := context.Background()

	mockedTx := &TestDbTx{}

	err := repo.CreateTransaction(ctx, mockedTx, "test-key", "payment", uuid.New(), nil, nil, []ledgerrepo.Entry{})
	if err == nil {
		t.Error("Expected error for empty entries")
	}
	if err.Error() != "ledger: at least one entry required" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestEntry_BalanceValidation(t *testing.T) {
	tests := []struct {
		name      string
		entries   []ledgerrepo.Entry
		wantPanic bool
	}{
		{
			name: "balanced transaction",
			entries: []ledgerrepo.Entry{
				{Amount: money.New(100)},
				{Amount: money.New(-50)},
				{Amount: money.New(-50)},
			},
			wantPanic: false,
		},
		{
			name: "unbalanced positive",
			entries: []ledgerrepo.Entry{
				{Amount: money.New(100)},
				{Amount: money.New(-50)},
			},
			wantPanic: true,
		},
		{
			name: "unbalanced negative",
			entries: []ledgerrepo.Entry{
				{Amount: money.New(50)},
				{Amount: money.New(-100)},
			},
			wantPanic: true,
		},
		{
			name: "all zeros",
			entries: []ledgerrepo.Entry{
				{Amount: money.Zero()},
				{Amount: money.Zero()},
			},
			wantPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewLedgerRepository()
			ctx := context.Background()
			mockedTx := &TestDbTx{}

			if tt.wantPanic {
				defer func() {
					if r := recover(); r == nil {
						t.Error("Expected panic for unbalanced entries")
					}
				}()
			}

			_ = repo.CreateTransaction(ctx, mockedTx, "key", "type", uuid.New(), nil, nil, tt.entries)
		})
	}
}

func TestMoneyImmutability(t *testing.T) {
	// Verify that money.Money is immutable
	original := money.New(100)
	amount := original

	_ = amount.Add(money.New(50))

	if original.Int64() != 100 {
		t.Errorf("Money was mutated: expected 100, got %d", original.Int64())
	}
}

func TestIsUniqueViolation(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "duplicate key error",
			err:  errors.New("duplicate key value violates unique constraint"),
			want: true,
		},
		{
			name: "UNIQUE constraint error",
			err:  errors.New("UNIQUE constraint failed"),
			want: true,
		},
		{
			name: "23505 in error",
			err:  errors.New("error code 23505"),
			want: true,
		},
		{
			name: "other error",
			err:  errors.New("some other error"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "no rows error",
			err:  errors.New("no rows in result set"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsUniqueViolation(tt.err)
			if got != tt.want {
				t.Errorf("IsUniqueViolation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		substr string
		want   bool
	}{
		{"contains", "hello world", "world", true},
		{"not contains", "hello world", "goodbye", false},
		{"empty substr", "hello", "", true},
		{"same string", "hello", "hello", true},
		{"case sensitive", "Hello", "hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contains(tt.s, tt.substr)
			if got != tt.want {
				t.Errorf("contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestLedgerRepository_CreateTransaction_SummaryFields verifies that
// total_debit and total_credit are computed from entries and stored in the
// ledger_transactions INSERT (not hardcoded 0).
//
// Regression guard for the monitoring blind spot where SUM(total_debit)-
// SUM(total_credit) always returned 0 regardless of actual movements.
func TestLedgerRepository_CreateTransaction_SummaryFields(t *testing.T) {
	tests := []struct {
		name          string
		entries       []ledgerrepo.Entry
		wantDebit     int64
		wantCredit    int64
	}{
		{
			name: "simple two-account transfer",
			entries: []ledgerrepo.Entry{
				{AccountID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), Amount: money.New(1000)},
				{AccountID: uuid.MustParse("00000000-0000-0000-0000-000000000002"), Amount: money.New(-1000)},
			},
			wantDebit:  1000,
			wantCredit: 1000,
		},
		{
			name: "split debit to two credit accounts",
			entries: []ledgerrepo.Entry{
				{AccountID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), Amount: money.New(500)},
				{AccountID: uuid.MustParse("00000000-0000-0000-0000-000000000002"), Amount: money.New(-300)},
				{AccountID: uuid.MustParse("00000000-0000-0000-0000-000000000003"), Amount: money.New(-200)},
			},
			wantDebit:  500,
			wantCredit: 500,
		},
		{
			name: "large withdrawal amount",
			entries: []ledgerrepo.Entry{
				{AccountID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), Amount: money.New(50_000_000)},
				{AccountID: uuid.MustParse("00000000-0000-0000-0000-000000000002"), Amount: money.New(-50_000_000)},
			},
			wantDebit:  50_000_000,
			wantCredit: 50_000_000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewLedgerRepository()
			ctx := context.Background()

			var capturedDebit, capturedCredit int64
			execCallCount := 0

			mockedTx := &TestDbTx{
				// Idempotency check: return no rows (new transaction)
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
					return &mockRow{err: errors.New("no rows in result set")}
				},
				// Account lock query: return all accounts with balance 0
				QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
					ids, _ := args[0].([]uuid.UUID)
					i := 0
					return &mockRows{
						nextFunc: func() bool {
							return i < len(ids)
						},
						scanFunc: func(dest ...any) error {
							if i < len(ids) {
								*(dest[0].(*uuid.UUID)) = ids[i]
								*(dest[1].(*int64)) = 0
								i++
							}
							return nil
						},
					}, nil
				},
				ExecFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
					execCallCount++
					if execCallCount == 1 {
						// First Exec is the INSERT into ledger_transactions.
						// Args: $1=id $2=idemKey $3=refType $4=refID $5=orderID $6=paymentID $7=totalDebit $8=totalCredit $9=createdAt
						if len(args) >= 9 {
							capturedDebit, _ = args[6].(int64)
							capturedCredit, _ = args[7].(int64)
						}
					}
					return pgconn.NewCommandTag("1"), nil
				},
			}

			err := repo.CreateTransaction(ctx, mockedTx, "test-summary-key", "withdrawal_request", uuid.New(), nil, nil, tt.entries)
			if err != nil {
				t.Fatalf("CreateTransaction() unexpected error: %v", err)
			}
			if capturedDebit != tt.wantDebit {
				t.Errorf("total_debit = %d, want %d", capturedDebit, tt.wantDebit)
			}
			if capturedCredit != tt.wantCredit {
				t.Errorf("total_credit = %d, want %d", capturedCredit, tt.wantCredit)
			}
			if capturedDebit != capturedCredit {
				t.Errorf("total_debit (%d) != total_credit (%d): transaction would fail DB CHECK constraint", capturedDebit, capturedCredit)
			}
		})
	}
}

// TestLedgerRepository_CreateTransaction_SummaryFields_Imbalance verifies that
// an unbalanced entry set panics before any Exec is called (existing invariant).
func TestLedgerRepository_CreateTransaction_SummaryFields_Imbalance(t *testing.T) {
	repo := NewLedgerRepository()
	ctx := context.Background()
	execCalled := false
	mockedTx := &TestDbTx{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			execCalled = true
			return pgconn.NewCommandTag("1"), nil
		},
	}
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic for unbalanced entries, got none")
		}
		if execCalled {
			t.Error("Exec must not be called before balance check panics")
		}
	}()
	entries := []ledgerrepo.Entry{
		{AccountID: uuid.New(), Amount: money.New(1000)},
		{AccountID: uuid.New(), Amount: money.New(-500)}, // unbalanced: +500 net
	}
	_ = repo.CreateTransaction(ctx, mockedTx, "imbalance-key", "test", uuid.New(), nil, nil, entries)
}

// TestDeterministicOrdering verifies that account IDs are sorted before locking.
// This is critical for preventing deadlocks in high-concurrency scenarios.
func TestDeterministicOrdering(t *testing.T) {
	// Create UUIDs in reverse order to test sorting
	uuid1 := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	uuid2 := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	uuid3 := uuid.MustParse("00000000-0000-0000-0000-000000000003")

	// Entries in reverse order (should be sorted before locking)
	entries := []ledgerrepo.Entry{
		{AccountID: uuid3, Amount: money.New(100)},
		{AccountID: uuid1, Amount: money.New(-50)},
		{AccountID: uuid2, Amount: money.New(-50)},
	}

	// Mock to capture the locked account order
	var lockedOrder []uuid.UUID
	mockedTx := &TestDbTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &mockRow{err: errors.New("no rows in result set")}
		},
		QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			// Capture the account IDs passed to FOR UPDATE query
			if ids, ok := args[0].([]uuid.UUID); ok {
				lockedOrder = ids
			}
			return &mockRows{}, nil
		},
	}

	repo := NewLedgerRepository()
	ctx := context.Background()
	_ = repo.CreateTransaction(ctx, mockedTx, "test-key", "test", uuid.New(), nil, nil, entries)

	// Verify the order is sorted
	if len(lockedOrder) != 3 {
		t.Fatalf("Expected 3 accounts to be locked, got %d", len(lockedOrder))
	}

	// Check if sorted: uuid1 < uuid2 < uuid3
	if lockedOrder[0] != uuid1 || lockedOrder[1] != uuid2 || lockedOrder[2] != uuid3 {
		t.Errorf("Accounts not sorted before locking: got %v, want [%v, %v, %v]",
			lockedOrder, uuid1, uuid2, uuid3)
	}
}


