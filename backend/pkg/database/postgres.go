package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/internal/platform/logger"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// DB is the unified database interface for the application
type DB struct {
	pgxDB *db.DB
}

// NewPostgresDB creates a new PostgreSQL database connection using pgx
func NewPostgresDB(cfg *config.DatabaseConfig, log *logger.Logger) (*DB, error) {
	dsn := cfg.GetDSN()

	log.Info("Connecting to PostgreSQL",
		zap.String("host", cfg.Host),
		zap.String("port", cfg.Port),
		zap.String("database", cfg.Name),
	)

	// Create pgx connection pool
	dbCfg := db.Config{
		ConnString:        dsn,
		MaxConns:          int32(cfg.MaxConnections),
		MinConns:          int32(cfg.MaxIdle),
		MaxConnLifetime:   cfg.ConnMaxLifetime,
		MaxConnIdleTime:   5 * time.Minute,
		HealthCheckPeriod: 30 * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pgxDB, err := db.New(ctx, dbCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	log.Info("Database connected successfully",
		zap.String("host", cfg.Host),
		zap.String("database", cfg.Name),
		zap.Int("max_connections", cfg.MaxConnections),
	)

	return &DB{pgxDB: pgxDB}, nil
}

// NewFromPgx wraps an already-constructed low-level DB in the higher-level
// database facade.
func NewFromPgx(pgxDB *db.DB) *DB {
	return &DB{pgxDB: pgxDB}
}

// CloseDB closes the database connection
func CloseDB(db *DB, log *logger.Logger) error {
	if db.pgxDB != nil {
		db.pgxDB.Close()
	}
	log.Info("Database connection closed")
	return nil
}

// HealthCheck performs a database health check
func HealthCheck(database *DB) error {
	if database.pgxDB == nil {
		return fmt.Errorf("database not initialized")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return database.pgxDB.Ping(ctx)
}

// Pgx returns the underlying pgx database for direct access
func (d *DB) Pgx() *db.DB {
	return d.pgxDB
}

// BeginTx starts a new transaction
func (d *DB) BeginTx(ctx context.Context) (db.Tx, error) {
	return d.pgxDB.BeginTx(ctx)
}

// WithTx executes fn within a transaction with automatic retry
func (d *DB) WithTx(ctx context.Context, fn func(db.Tx) error) error {
	return d.pgxDB.WithTx(ctx, fn)
}

// Pool returns the underlying pgx pool for direct queries
func (d *DB) Pool() *pgxpool.Pool {
	return d.pgxDB.Pool()
}
