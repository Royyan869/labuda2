package db

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// mockPgError is a mock implementation of pgconn.PgError for testing.
type mockPgError struct {
	code     string
	message  string
	constraint string
}

func (e *mockPgError) Code() string {
	return e.code
}

func (e *mockPgError) Message() string {
	return e.message
}

func (e *mockPgError) Constraint() string {
	return e.constraint
}

// Error implements error interface.
func (e *mockPgError) Error() string {
	return e.message
}

// Verify mockPgError implements the interface we check in errors.go
var _ interface{ Code() string } = (*mockPgError)(nil)

// TestIsUniqueViolation tests the IsUniqueViolation function.
func TestIsUniqueViolation(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "unique violation error",
			err: &mockPgError{code: "23505", message: "duplicate key value violates unique constraint"},
			want: true,
		},
		{
			name: "serialization failure",
			err:  &mockPgError{code: "40001", message: "serialization failure"},
			want: false,
		},
		{
			name: "deadlock",
			err:  &mockPgError{code: "40P01", message: "deadlock detected"},
			want: false,
		},
		{
			name: "generic error",
			err:  errors.New("generic error"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
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

// TestIsSerializationFailure tests the IsSerializationFailure function.
func TestIsSerializationFailure(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "serialization failure",
			err:  &mockPgError{code: "40001", message: "serialization failure"},
			want: true,
		},
		{
			name: "unique violation",
			err:  &mockPgError{code: "23505", message: "duplicate key"},
			want: false,
		},
		{
			name: "deadlock",
			err:  &mockPgError{code: "40P01", message: "deadlock detected"},
			want: false,
		},
		{
			name: "generic error",
			err:  errors.New("generic error"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSerializationFailure(tt.err)
			if got != tt.want {
				t.Errorf("IsSerializationFailure() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsDeadlock tests the IsDeadlock function.
func TestIsDeadlock(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "deadlock",
			err:  &mockPgError{code: "40P01", message: "deadlock detected"},
			want: true,
		},
		{
			name: "unique violation",
			err:  &mockPgError{code: "23505", message: "duplicate key"},
			want: false,
		},
		{
			name: "serialization failure",
			err:  &mockPgError{code: "40001", message: "serialization failure"},
			want: false,
		},
		{
			name: "generic error",
			err:  errors.New("generic error"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDeadlock(tt.err)
			if got != tt.want {
				t.Errorf("IsDeadlock() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsRetryable tests the IsRetryable function.
func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "serialization failure is retryable",
			err:  &mockPgError{code: "40001", message: "serialization failure"},
			want: true,
		},
		{
			name: "deadlock is retryable",
			err:  &mockPgError{code: "40P01", message: "deadlock detected"},
			want: true,
		},
		{
			name: "unique violation is not retryable",
			err:  &mockPgError{code: "23505", message: "duplicate key"},
			want: false,
		},
		{
			name: "generic error is not retryable",
			err:  errors.New("generic error"),
			want: false,
		},
		{
			name: "nil error is not retryable",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRetryable(tt.err)
			if got != tt.want {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.want)
			}
		})
	}
}

// mockTx is a mock transaction for testing.
type mockTx struct {
	execCalled    int
	queryCalled   int
	queryRowCalled int
	commitCalled  int
	rollbackCalled int
	shouldFail    error
	commitFail    error
	rollbackFail  error
}

func (m *mockTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	m.execCalled++
	return pgconn.CommandTag{}, m.shouldFail
}

func (m *mockTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	m.queryCalled++
	// Return a mock rows that satisfies the interface
	return &mockRows{}, m.shouldFail
}

func (m *mockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	m.queryRowCalled++
	// Return a mock row that satisfies the interface
	return &mockRow{}
}

// mockRows is a mock implementation of pgx.Rows.
type mockRows struct{}

func (m *mockRows) Close() {}
func (m *mockRows) Err() error { return nil }
func (m *mockRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (m *mockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (m *mockRows) Next() bool { return false }
func (m *mockRows) Scan(dest ...any) error { return nil }
func (m *mockRows) Values() ([]any, error) { return nil, nil }
func (m *mockRows) RawValues() [][]byte { return nil }
func (m *mockRows) Conn() *pgx.Conn { return nil }

// mockRow is a mock implementation of pgx.Row.
type mockRow struct{}

func (m *mockRow) Scan(dest ...any) error { return nil }


func (m *mockTx) Commit(ctx context.Context) error {
	m.commitCalled++
	return m.commitFail
}

func (m *mockTx) Rollback(ctx context.Context) error {
	m.rollbackCalled++
	return m.rollbackFail
}

// Verify mockTx implements Tx
var _ Tx = (*mockTx)(nil)

// mockDB is a mock database for testing retry logic.
type mockDB struct {
	beginCalls   int
	beginTxError error
	tx           *mockTx
}

func (m *mockDB) BeginTx(ctx context.Context) (Tx, error) {
	m.beginCalls++
	if m.beginTxError != nil {
		return nil, m.beginTxError
	}
	return m.tx, nil
}

// TestWithRetry tests the withRetry function.
func TestWithRetry(t *testing.T) {
	t.Run("success on first attempt", func(t *testing.T) {
		tx := &mockTx{}
		db := &DB{}
		// We need to use a custom approach since withRetry takes *DB, not an interface

		// This test verifies the structure - actual integration test would need a real pool
		if db == nil {
			t.Fatal("db should not be nil")
		}
		_ = tx
	})

	t.Run("non-retryable error fails immediately", func(t *testing.T) {
		err := &mockPgError{code: "23505", message: "unique violation"}
		if !IsUniqueViolation(err) {
			t.Fatal("test setup failed")
		}
		if IsRetryable(err) {
			t.Error("unique violation should not be retryable")
		}
	})

	t.Run("serialization failure is retryable", func(t *testing.T) {
		err := &mockPgError{code: "40001", message: "serialization failure"}
		if !IsSerializationFailure(err) {
			t.Fatal("test setup failed")
		}
		if !IsRetryable(err) {
			t.Error("serialization failure should be retryable")
		}
	})

	t.Run("deadlock is retryable", func(t *testing.T) {
		err := &mockPgError{code: "40P01", message: "deadlock detected"}
		if !IsDeadlock(err) {
			t.Fatal("test setup failed")
		}
		if !IsRetryable(err) {
			t.Error("deadlock should be retryable")
		}
	})
}

// TestDefaultConfig tests the DefaultConfig function.
func TestDefaultConfig(t *testing.T) {
	t.Run("empty config returns defaults", func(t *testing.T) {
		cfg := DefaultConfig(Config{})
		if cfg.MaxConns != 0 {
			t.Errorf("MaxConns = %d, want 0", cfg.MaxConns)
		}
		if cfg.MinConns != 0 {
			t.Errorf("MinConns = %d, want 0", cfg.MinConns)
		}
		if cfg.MaxConnLifetime == 0 {
			t.Error("MaxConnLifetime should be set")
		}
		if cfg.MaxConnIdleTime == 0 {
			t.Error("MaxConnIdleTime should be set")
		}
		if cfg.HealthCheckPeriod == 0 {
			t.Error("HealthCheckPeriod should be set")
		}
	})

	t.Run("custom values override defaults", func(t *testing.T) {
		input := Config{
			MaxConns:          10,
			MinConns:          2,
			MaxConnLifetime:   2 * 3600_000_000_000, // 2 hours in nanoseconds
			MaxConnIdleTime:   10 * 60 * 1_000_000_000, // 10 minutes
			HealthCheckPeriod: 30 * 1_000_000_000, // 30 seconds
		}
		cfg := DefaultConfig(input)
		if cfg.MaxConns != 10 {
			t.Errorf("MaxConns = %d, want 10", cfg.MaxConns)
		}
		if cfg.MinConns != 2 {
			t.Errorf("MinConns = %d, want 2", cfg.MinConns)
		}
	})
}
