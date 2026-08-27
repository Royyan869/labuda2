package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/interaction/notification/infrastructure/repository"
)

// fakePool implements the minimal interface required by FCMTokenRepository pool
// fallback (dbQuerier). Returns empty rows for Query and success for Exec.
type fakePool struct {
	queryCalled bool
	execCalled  bool
}

func (f *fakePool) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	f.queryCalled = true
	return &fakeRows{}, nil
}

func (f *fakePool) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	f.execCalled = true
	return pgconn.NewCommandTag("UPDATE 0"), nil
}

// fakeRows implements pgx.Rows returning no data (empty result set).
type fakeRows struct {
	closed bool
}

func (r *fakeRows) Next() bool                        { return false }
func (r *fakeRows) Err() error                        { return nil }
func (r *fakeRows) Close()                            { r.closed = true }
func (r *fakeRows) Scan(dest ...any) error            { return pgx.ErrNoRows }
func (r *fakeRows) CommandTag() pgconn.CommandTag     { return pgconn.NewCommandTag("SELECT 0") }
func (r *fakeRows) Fields() []pgconn.FieldDescription { return nil }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRows) RawValues() [][]byte               { return nil }
func (r *fakeRows) Values() ([]any, error)            { return nil, nil }
func (r *fakeRows) Conn() *pgx.Conn                   { return nil }

// =============================================================================
// GetActiveTokensByUser nil-tx tests
// =============================================================================

// TestGetActiveTokensByUser_NilTx_NilPool verifies that passing tx=nil with no
// pool configured returns a controlled error and does NOT panic.
func TestGetActiveTokensByUser_NilTx_NilPool(t *testing.T) {
	repo := repository.NewFCMTokenRepository(nil)

	tokens, err := repo.GetActiveTokensByUser(context.Background(), nil, uuid.New())

	if err == nil {
		t.Fatal("expected error when tx=nil and pool=nil, got nil")
	}
	if tokens != nil {
		t.Errorf("expected nil tokens, got %v", tokens)
	}
}

// TestGetActiveTokensByUser_NilTx_WithPool verifies that passing tx=nil falls
// back to the pool, uses it for the query, and returns an empty list cleanly.
func TestGetActiveTokensByUser_NilTx_WithPool(t *testing.T) {
	pool := &fakePool{}
	repo := repository.NewFCMTokenRepository(pool)

	tokens, err := repo.GetActiveTokensByUser(context.Background(), nil, uuid.New())

	if err != nil {
		t.Fatalf("unexpected error with pool fallback: %v", err)
	}
	if !pool.queryCalled {
		t.Error("expected pool.Query to be called for nil-tx fallback")
	}
	// Empty DB → empty slice (not nil).
	if tokens == nil {
		t.Error("expected non-nil (empty) token slice, got nil")
	}
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens from empty pool, got %d", len(tokens))
	}
}

// TestGetActiveTokensByUser_WithTx verifies the happy path where a real
// (non-nil) tx is provided — pool should not be touched.
func TestGetActiveTokensByUser_WithTx(t *testing.T) {
	pool := &fakePool{}
	repo := repository.NewFCMTokenRepository(pool)

	fakeTx := &fakeTx{}
	tokens, err := repo.GetActiveTokensByUser(context.Background(), fakeTx, uuid.New())

	if err != nil {
		t.Fatalf("unexpected error with tx: %v", err)
	}
	if pool.queryCalled {
		t.Error("pool.Query must NOT be called when tx is non-nil")
	}
	if fakeTx.queryCalled != 1 {
		t.Errorf("expected tx.Query called once, got %d", fakeTx.queryCalled)
	}
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens from empty tx, got %d", len(tokens))
	}
}

// =============================================================================
// DeactivateByToken nil-tx tests
// =============================================================================

// TestDeactivateByToken_NilTx_NilPool verifies no panic and a controlled error.
func TestDeactivateByToken_NilTx_NilPool(t *testing.T) {
	repo := repository.NewFCMTokenRepository(nil)

	err := repo.DeactivateByToken(context.Background(), nil, "some-fcm-token")

	if err == nil {
		t.Fatal("expected error when tx=nil and pool=nil, got nil")
	}
}

// TestDeactivateByToken_NilTx_WithPool verifies pool fallback is used.
func TestDeactivateByToken_NilTx_WithPool(t *testing.T) {
	pool := &fakePool{}
	repo := repository.NewFCMTokenRepository(pool)

	err := repo.DeactivateByToken(context.Background(), nil, "some-fcm-token")

	if err != nil {
		t.Fatalf("unexpected error with pool fallback: %v", err)
	}
	if !pool.execCalled {
		t.Error("expected pool.Exec to be called for nil-tx fallback")
	}
}

// =============================================================================
// fakeTx — satisfies both Query and Exec for WithTx path
// =============================================================================

type fakeTx struct {
	queryCalled int
	execCalled  int
}

func (f *fakeTx) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	f.queryCalled++
	return &fakeRows{}, nil
}

func (f *fakeTx) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	f.execCalled++
	return pgconn.NewCommandTag("UPDATE 0"), nil
}
