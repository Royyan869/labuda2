package database

import (
	"context"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/labuda/backend/internal/platform/logger"
	"go.uber.org/zap"
)

// RunMigrations applies the numbered migration chain using golang-migrate.
// It is a tooling helper, not an auto-run runtime path: core_server does not
// invoke this function in the current release flow.
func RunMigrations(databaseURL string, log *logger.Logger) error {
	log.Info("Starting database migrations with golang-migrate")

	// Create migrate instance
	// Path relative to working directory (backend/)
	m, err := migrate.New(
		"file://migrations",
		databaseURL,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	// Get current migration version
	version, dirty, err := m.Version()
	if err != nil {
		if err == migrate.ErrNilVersion {
			// No migrations have been run yet - this is a new database
			log.Info("No migration version found - running all migrations from start")
		} else {
			return fmt.Errorf("failed to get migration version: %w", err)
		}
	} else {
		log.Info("Current migration version",
			zap.Uint32("version", uint32(version)),
			zap.Bool("dirty", dirty))

		if dirty {
			return fmt.Errorf("database is in dirty state at version %d, manual intervention required", version)
		}
	}

	// Run any pending migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Get final version
	version, dirty, err = m.Version()
	if err != nil {
		log.Warn("Could not get final migration version", zap.Error(err))
	} else {
		log.Info("Database migrations completed successfully",
			zap.Uint32("version", uint32(version)),
			zap.Bool("dirty", dirty))
	}

	return nil
}

// CreateOutboxIdempotencyConstraints creates unique indexes for outbox idempotency
// This uses raw SQL via pkg/db
func CreateOutboxIdempotencyConstraints(ctx context.Context, db *DB, log *logger.Logger) error {
	log.Info("Creating outbox idempotency constraints")

	// Get the underlying pool
	pool := db.Pool()

	// Partial unique index for subscription_expired events
	const subscriptionExpiredIndex = `
		CREATE UNIQUE INDEX IF NOT EXISTS idx_outbox_subscription_expired_idempotent
		ON outbox (event_type, aggregate_id)
		WHERE event_type = 'subscription_expired';
	`
	_, err := pool.Exec(ctx, subscriptionExpiredIndex)
	if err != nil {
		return fmt.Errorf("failed to create subscription_expired idempotency index: %w", err)
	}
	log.Info("Created subscription_expired idempotency index")

	return nil
}
