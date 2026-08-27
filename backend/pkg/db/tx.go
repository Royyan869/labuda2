package db

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Tx represents a database transaction.
// Domain layer uses this interface for all database operations.
// This makes testing easier and hides implementation details.
type Tx interface {
	// Exec executes a query without returning any rows.
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)

	// Query executes a query that returns rows.
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)

	// QueryRow executes a query that returns at most one row.
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row

	// Commit commits the transaction.
	Commit(ctx context.Context) error

	// Rollback rolls back the transaction.
	Rollback(ctx context.Context) error
}

// tx is the concrete implementation of Tx using pgxpool.
type tx struct {
	tx pgx.Tx
}

// Exec implements Tx.
func (t *tx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return t.tx.Exec(ctx, sql, args...)
}

// Query implements Tx.
func (t *tx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return t.tx.Query(ctx, sql, args...)
}

// QueryRow implements Tx.
func (t *tx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return t.tx.QueryRow(ctx, sql, args...)
}

// Commit implements Tx.
func (t *tx) Commit(ctx context.Context) error {
	return t.tx.Commit(ctx)
}

// Rollback implements Tx.
func (t *tx) Rollback(ctx context.Context) error {
	return t.tx.Rollback(ctx)
}

// Verify tx implements Tx at compile time.
var _ Tx = (*tx)(nil)
