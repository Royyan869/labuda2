//go:build integration

package testdb

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/pkg/db"
)

// TestTruncateAll_UsesFreshCleanupConnection proves teardown no longer depends
// on the live test pool being able to hand out an idle connection. We exhaust
// the original pool, then call TruncateAll; if cleanup still borrowed from the
// exhausted pool, this test would stall until the context deadline instead of
// completing.
func TestTruncateAll_UsesFreshCleanupConnection(t *testing.T) {
	loadDotEnvFromParents(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	ctx := context.Background()
	if err := runMigrations(cfg, t); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	pool, err := db.New(ctx, db.Config{ConnString: cfg.Database.GetTestDSN(), MaxConns: 1})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	tdb := &TestDB{pool: pool.Pool(), config: cfg, database: cfg.Database.TestName}
	defer tdb.Pool().Close()

	var currentDB string
	if err := tdb.Pool().QueryRow(ctx, "SELECT current_database()").Scan(&currentDB); err != nil {
		t.Fatalf("verify test database: %v", err)
	}
	if currentDB != cfg.Database.TestName {
		t.Fatalf("connected to %q, want %q", currentDB, cfg.Database.TestName)
	}

	rowID := uuid.New()
	_, execErr := tdb.Pool().Exec(ctx, `
		INSERT INTO users (
			id, firebase_uid, email, email_verified_at, phone_verified,
			account_status, created_at, updated_at, role
		)
		VALUES ($1, $2, $3, NOW(), true, 'active', NOW(), NOW(), 'user')
	`, rowID, rowID.String(), fmt.Sprintf("truncate-%s@test.invalid", rowID))
	if execErr != nil {
		t.Fatalf("seed row: %v", execErr)
	}

	maxConns := int(tdb.Pool().Stat().MaxConns())
	held := make([]*pgxpool.Conn, 0, maxConns)
	for i := 0; i < maxConns; i++ {
		conn, err := tdb.Pool().Acquire(ctx)
		if err != nil {
			t.Fatalf("acquire connection %d/%d: %v", i+1, maxConns, err)
		}
		held = append(held, conn)
	}
	defer func() {
		for i := len(held) - 1; i >= 0; i-- {
			held[i].Release()
		}
	}()

	cleanupCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	if err := tdb.TruncateAll(cleanupCtx); err != nil {
		t.Fatalf("TruncateAll with exhausted live pool: %v", err)
	}

	for i := len(held) - 1; i >= 0; i-- {
		held[i].Release()
	}
	held = nil

	var count int
	if err := tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE id = $1`, rowID).Scan(&count); err != nil {
		t.Fatalf("verify truncate: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected seeded user to be truncated, got %d rows", count)
	}
}
