package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB is the single entry point to PostgreSQL.
// It manages a connection pool and provides transaction methods.
type DB struct {
	pool *pgxpool.Pool
}

// New creates a new DB instance with a connection pool.
//
// The connection string format is:
// postgres://user:password@host:port/database
//
// Example:
//
//	cfg := db.Config{ConnString: "postgres://localhost:5432/mydb"}
//	db, err := db.New(ctx, cfg)
func New(ctx context.Context, cfg Config) (*DB, error) {
	config, err := pgxpool.ParseConfig(cfg.ConnString)
	if err != nil {
		return nil, fmt.Errorf("parse conn string: %w", err)
	}

	// Apply pool configuration
	finalCfg := DefaultConfig(cfg)

	if finalCfg.MaxConns > 0 {
		config.MaxConns = finalCfg.MaxConns
	}
	if finalCfg.MinConns > 0 {
		config.MinConns = finalCfg.MinConns
	}
	if finalCfg.MaxConnLifetime > 0 {
		config.MaxConnLifetime = finalCfg.MaxConnLifetime
	}
	if finalCfg.MaxConnIdleTime > 0 {
		config.MaxConnIdleTime = finalCfg.MaxConnIdleTime
	}
	if finalCfg.HealthCheckPeriod > 0 {
		config.HealthCheckPeriod = finalCfg.HealthCheckPeriod
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &DB{pool: pool}, nil
}

// Close closes the connection pool.
// It should be called when the DB instance is no longer needed.
func (db *DB) Close() {
	if db.pool != nil {
		db.pool.Close()
	}
}

// BeginTx starts a new transaction.
// The returned Tx must be committed or rolled back.
func (db *DB) BeginTx(ctx context.Context) (Tx, error) {
	pgxTx, err := db.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &tx{tx: pgxTx}, nil
}

// WithTx executes fn within a transaction with automatic retry on retryable errors.
//
// The transaction is committed if fn returns nil.
// The transaction is rolled back if fn returns an error.
//
// Retryable errors:
//   - Serialization failure (40001)
//   - Deadlock detected (40P01)
//
// Max attempts: 3
//
// Example:
//
//	err := db.WithTx(ctx, func(tx db.Tx) error {
//	    _, err := tx.Exec(ctx, "INSERT INTO users (name) VALUES ($1)", "John")
//	    return err
//	})
func (db *DB) WithTx(ctx context.Context, fn func(Tx) error) error {
	return withRetry(ctx, db, fn, defaultMaxAttempts)
}

// Pool returns the underlying pgxpool.Pool.
// Use this for direct queries outside transactions.
// For transactional operations, use WithTx instead.
func (db *DB) Pool() *pgxpool.Pool {
	return db.pool
}

// NewFromPool wraps an existing pgxpool.Pool as a *DB.
//
// Test-only escape hatch: repositories that call db.Pool() directly (rather
// than taking a db.Tx) need a *DB to construct against pkg/testdb's pool
// (e.g. testdb.TestDB.Pool()) in integration tests. Production code should
// always go through New.
func NewFromPool(pool *pgxpool.Pool) *DB {
	return &DB{pool: pool}
}

// Ping verifies a connection to the database is still alive.
func (db *DB) Ping(ctx context.Context) error {
	return db.pool.Ping(ctx)
}

// Stats returns pool statistics.
func (db *DB) Stats() *pgxpool.Stat {
	return db.pool.Stat()
}

// defaultMaxAttempts is the maximum number of retry attempts for WithTx.
const defaultMaxAttempts = 3

// Transactor defines the interface for types that can execute transactions.
// This is used by workers and services that need transactional operations.
type Transactor interface {
	WithTx(ctx context.Context, fn func(Tx) error) error
}

// Compile-time check: DB implements Transactor.
var _ Transactor = (*DB)(nil)
