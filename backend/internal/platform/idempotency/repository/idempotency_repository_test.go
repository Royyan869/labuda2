package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// mockTx implements db.Tx for unit testing without a real database.
type mockTx struct {
	execFunc     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	queryRowFunc func(ctx context.Context, sql string, args ...any) pgx.Row
	execCalled   int
	queryRowCalled int
}

func (m *mockTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	m.execCalled++
	if m.execFunc != nil {
		return m.execFunc(ctx, sql, args...)
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func (m *mockTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}

func (m *mockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	m.queryRowCalled++
	if m.queryRowFunc != nil {
		return m.queryRowFunc(ctx, sql, args...)
	}
	return &mockRow{scanErr: errors.New("no rows in result set")}
}

func (m *mockTx) Commit(ctx context.Context) error   { return nil }
func (m *mockTx) Rollback(ctx context.Context) error { return nil }

type mockRow struct {
	scanFunc func(dest ...any) error
	scanErr  error
}

func (r *mockRow) Scan(dest ...any) error {
	if r.scanFunc != nil {
		return r.scanFunc(dest...)
	}
	if r.scanErr != nil {
		return r.scanErr
	}
	return nil
}

// TestTryInsert_success verifies that a successful INSERT returns nil.
func TestTryInsert_success(t *testing.T) {
	repo := NewRepository()
	tx := &mockTx{
		execFunc: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		},
	}

	err := repo.TryInsert(context.Background(), tx, "key-1", "op-1", uuid.New())
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if tx.execCalled != 1 {
		t.Fatalf("expected Exec called once, got %d", tx.execCalled)
	}
}

// TestTryInsert_duplicate returns ErrAlreadyExists when RowsAffected == 0.
// ON CONFLICT DO NOTHING silently skips the INSERT and returns 0 rows affected
// without raising an error — this is the core of the P2 fix.
func TestTryInsert_duplicate_returnsErrAlreadyExists(t *testing.T) {
	repo := NewRepository()
	tx := &mockTx{
		execFunc: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			// ON CONFLICT DO NOTHING: no error, 0 rows affected
			return pgconn.NewCommandTag("INSERT 0 0"), nil
		},
	}

	err := repo.TryInsert(context.Background(), tx, "key-dup", "op-dup", uuid.New())
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

// TestTryInsert_txNotAbortedAfterDuplicate is the critical regression guard.
// With the old naive INSERT, a duplicate key caused PostgreSQL to mark the tx
// as ABORTED, making all subsequent SQL fail. With ON CONFLICT DO NOTHING,
// no error is raised and the transaction stays usable.
//
// We prove this by calling QueryRow immediately after TryInsert returns
// ErrAlreadyExists — it must be called (not short-circuited by a tx-abort check)
// and the tx is committed without error.
func TestTryInsert_txNotAbortedAfterDuplicate(t *testing.T) {
	repo := NewRepository()
	queryRowCalled := false

	tx := &mockTx{
		execFunc: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 0 0"), nil // duplicate → 0 rows
		},
		queryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
			queryRowCalled = true
			return &mockRow{scanErr: errors.New("no rows in result set")}
		},
	}

	// Step 1: TryInsert on duplicate key → ErrAlreadyExists (no tx abort)
	err := repo.TryInsert(context.Background(), tx, "key-dup", "op-dup", uuid.New())
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}

	// Step 2: QueryRow in the SAME transaction — must not fail due to "tx aborted"
	repo.Get(context.Background(), tx, "key-dup") //nolint:errcheck // error expected (no rows)

	if !queryRowCalled {
		t.Fatal("QueryRow was never called — tx may have been aborted before reaching Get")
	}
}

// TestTryInsert_dbError verifies that real database errors are propagated.
func TestTryInsert_dbError(t *testing.T) {
	repo := NewRepository()
	dbErr := errors.New("connection reset by peer")
	tx := &mockTx{
		execFunc: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, dbErr
		},
	}

	err := repo.TryInsert(context.Background(), tx, "key-err", "op-err", uuid.New())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to insert idempotency record") {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

// TestTryInsert_usesOnConflictDoNothing is a source-inspection test that ensures
// the SQL query uses ON CONFLICT DO NOTHING (not a naive INSERT that aborts tx).
func TestTryInsert_usesOnConflictDoNothing(t *testing.T) {
	repo := NewRepository()
	var capturedSQL string
	tx := &mockTx{
		execFunc: func(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		},
	}

	repo.TryInsert(context.Background(), tx, "key-inspect", "op-inspect", uuid.New()) //nolint:errcheck

	if !strings.Contains(strings.ToUpper(capturedSQL), "ON CONFLICT") {
		t.Fatalf("SQL must use ON CONFLICT DO NOTHING to avoid tx abort; got: %s", capturedSQL)
	}
	if !strings.Contains(strings.ToUpper(capturedSQL), "DO NOTHING") {
		t.Fatalf("SQL must use DO NOTHING clause; got: %s", capturedSQL)
	}
}

// TestGet_success verifies that Get returns a Record when a row exists.
func TestGet_success(t *testing.T) {
	repo := NewRepository()
	expectedID := uuid.New()
	expectedEntityID := uuid.New()

	tx := &mockTx{
		queryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{
				scanFunc: func(dest ...any) error {
					// id, idempotency_key, operation, entity_id
					*dest[0].(*uuid.UUID) = expectedID
					*dest[1].(*string) = "key-get"
					*dest[2].(*string) = "op-get"
					*dest[3].(*uuid.UUID) = expectedEntityID
					return nil
				},
			}
		},
	}

	rec, err := repo.Get(context.Background(), tx, "key-get")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if rec.ID != expectedID {
		t.Errorf("expected ID %v, got %v", expectedID, rec.ID)
	}
	if rec.IdempotencyKey != "key-get" {
		t.Errorf("expected key 'key-get', got %q", rec.IdempotencyKey)
	}
	if rec.Operation != "op-get" {
		t.Errorf("expected op 'op-get', got %q", rec.Operation)
	}
	if rec.EntityID != expectedEntityID {
		t.Errorf("expected entityID %v, got %v", expectedEntityID, rec.EntityID)
	}
}

// TestGet_notFound verifies that Get returns ErrRecordNotFound when no row exists.
func TestGet_notFound(t *testing.T) {
	repo := NewRepository()
	tx := &mockTx{
		queryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: errors.New("no rows in result set")}
		},
	}

	_, err := repo.Get(context.Background(), tx, "missing-key")
	if !errors.Is(err, ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}

// TestGetOrCreate_existingRecord verifies that GetOrCreate returns existing record
// without inserting a new one.
func TestGetOrCreate_existingRecord(t *testing.T) {
	repo := NewRepository()
	existingID := uuid.New()
	existingEntityID := uuid.New()

	tx := &mockTx{
		queryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{
				scanFunc: func(dest ...any) error {
					*dest[0].(*uuid.UUID) = existingID
					*dest[1].(*string) = "key-exists"
					*dest[2].(*string) = "op-exists"
					*dest[3].(*uuid.UUID) = existingEntityID
					return nil
				},
			}
		},
	}

	rec, created, err := repo.GetOrCreate(context.Background(), tx, "key-exists", "op-exists", existingEntityID)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if created {
		t.Fatal("expected created=false for existing record")
	}
	if rec.ID != existingID {
		t.Errorf("expected ID %v, got %v", existingID, rec.ID)
	}
	// Exec (TryInsert) must NOT be called when record already exists
	if tx.execCalled != 0 {
		t.Errorf("expected Exec not called for existing record, called %d times", tx.execCalled)
	}
}

// TestGetOrCreate_operationMismatch verifies that GetOrCreate rejects key reuse
// across different operations.
func TestGetOrCreate_operationMismatch(t *testing.T) {
	repo := NewRepository()

	tx := &mockTx{
		queryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{
				scanFunc: func(dest ...any) error {
					*dest[0].(*uuid.UUID) = uuid.New()
					*dest[1].(*string) = "key-mismatch"
					*dest[2].(*string) = "op-original"
					*dest[3].(*uuid.UUID) = uuid.New()
					return nil
				},
			}
		},
	}

	_, _, err := repo.GetOrCreate(context.Background(), tx, "key-mismatch", "op-different", uuid.New())
	if err == nil {
		t.Fatal("expected error for operation mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "different operation") {
		t.Errorf("expected 'different operation' in error, got %v", err)
	}
}

// TestGetOrCreate_createNew verifies that GetOrCreate creates a new record
// when none exists, returning created=true.
func TestGetOrCreate_createNew(t *testing.T) {
	repo := NewRepository()
	entityID := uuid.New()
	queryRowCallCount := 0

	tx := &mockTx{
		execFunc: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 0 1"), nil // success
		},
		queryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
			queryRowCallCount++
			// First call (Get): not found
			return &mockRow{scanErr: errors.New("no rows in result set")}
		},
	}

	rec, created, err := repo.GetOrCreate(context.Background(), tx, "key-new", "op-new", entityID)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !created {
		t.Fatal("expected created=true for new record")
	}
	if rec.IdempotencyKey != "key-new" {
		t.Errorf("expected key 'key-new', got %q", rec.IdempotencyKey)
	}
	if rec.Operation != "op-new" {
		t.Errorf("expected op 'op-new', got %q", rec.Operation)
	}
}
