//go:build integration

package migration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/labuda/backend/internal/config"
)

// --- helpers (unchanged) ---

func loadConfig(t *testing.T) *config.Config {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, ".env")
		if _, statErr := os.Stat(candidate); statErr == nil {
			if loadErr := godotenv.Load(candidate); loadErr == nil {
				t.Logf("loaded .env from %s", candidate)
			}
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg
}

func resolveMigrationsDir(t *testing.T) string {
	t.Helper()
	candidates := []string{
		"migrations",
		"../migrations",
		"../../migrations",
		"../../../migrations",
		"../../../../migrations",
		"../../../../../migrations",
		"../../../../../../migrations",
	}
	for _, d := range candidates {
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			return d
		}
	}
	t.Fatal("migrations directory not found")
	return ""
}

func connectTestDB(t *testing.T, cfg *config.Config) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, cfg.Database.GetTestDSN())
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	var currentDB string
	if err := pool.QueryRow(ctx, "SELECT current_database()").Scan(&currentDB); err != nil {
		t.Fatalf("verify database: %v", err)
	}
	if currentDB != cfg.Database.TestName {
		t.Fatalf("connected to %q, want %q", currentDB, cfg.Database.TestName)
	}
	return pool
}

func resetPublicSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("reset public schema acquire: %v", err)
	}
	defer conn.Release()

	for _, stmt := range []string{
		`DROP EXTENSION IF EXISTS btree_gist CASCADE`,
		`DROP EXTENSION IF EXISTS "uuid-ossp" CASCADE`,
		`DROP EXTENSION IF EXISTS pgcrypto CASCADE`,
		`DROP SCHEMA IF EXISTS public CASCADE`,
		`CREATE SCHEMA IF NOT EXISTS public`,
	} {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			t.Fatalf("reset public schema (%s): %v", stmt, err)
		}
	}
	pool.Reset()
}

// --- Test 1: fresh empty database creates canonical migration table ---

func TestIntegration_FreshDBCreation(t *testing.T) {
	cfg := loadConfig(t)
	pool := connectTestDB(t, cfg)
	ctx := context.Background()
	resetPublicSchema(t, ctx, pool)

	// Before: no table
	var exists bool
	_ = pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_tables WHERE schemaname='public' AND tablename='schema_migrations')`).Scan(&exists)
	if exists {
		t.Fatal("table state BEFORE: schema_migrations already exists — expected clean DB")
	}

	if err := EnsureSchemaMigrationsTable(ctx, pool); err != nil {
		t.Fatalf("EnsureSchemaMigrationsTable: %v", err)
	}

	// After: table exists with correct columns
	var hasName, hasVersion, hasAppliedAt bool
	err := pool.QueryRow(ctx, `
		SELECT
			EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='schema_migrations' AND column_name='name'),
			EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='schema_migrations' AND column_name='version'),
			EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='schema_migrations' AND column_name='applied_at')
	`).Scan(&hasName, &hasVersion, &hasAppliedAt)
	if err != nil {
		t.Fatalf("verify columns: %v", err)
	}
	t.Logf("table state AFTER: name=%v version=%v applied_at=%v", hasName, hasVersion, hasAppliedAt)
	if !hasName || !hasVersion || !hasAppliedAt {
		t.Fatal("schema_migrations missing expected columns")
	}
}

// --- Test 2: canonical existing table is preserved ---

func TestIntegration_CanonicalTablePreserved(t *testing.T) {
	cfg := loadConfig(t)
	pool := connectTestDB(t, cfg)
	ctx := context.Background()

	// Ensure canonical table exists from previous tests or create it
	if err := EnsureSchemaMigrationsTable(ctx, pool); err != nil {
		t.Fatalf("initial EnsureSchemaMigrationsTable: %v", err)
	}

	// Record the oid of the existing table before the call
	var oidBefore uint32
	if err := pool.QueryRow(ctx, `SELECT relfilenode FROM pg_class WHERE relname = 'schema_migrations' AND relnamespace = 'public'::regnamespace`).Scan(&oidBefore); err != nil {
		t.Fatalf("get oid before: %v", err)
	}
	t.Logf("table state BEFORE: oid=%d", oidBefore)

	// Second call — must be no-op
	if err := EnsureSchemaMigrationsTable(ctx, pool); err != nil {
		t.Fatalf("second EnsureSchemaMigrationsTable: %v", err)
	}

	var oidAfter uint32
	if err := pool.QueryRow(ctx, `SELECT relfilenode FROM pg_class WHERE relname = 'schema_migrations' AND relnamespace = 'public'::regnamespace`).Scan(&oidAfter); err != nil {
		t.Fatalf("get oid after: %v", err)
	}
	t.Logf("table state AFTER: oid=%d", oidAfter)
	if oidBefore != oidAfter {
		t.Fatalf("canonical table was NOT preserved: oid changed from %d to %d", oidBefore, oidAfter)
	}
}

// --- Test 3: existing canonical current schema results in no-op ---

func TestIntegration_CanonicalSchemaIsNoOp(t *testing.T) {
	cfg := loadConfig(t)
	pool := connectTestDB(t, cfg)
	ctx := context.Background()

	dir := resolveMigrationsDir(t)

	// First run applies all 41 migrations
	if err := Run(ctx, pool, dir); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	// Record state after first run
	var countBefore int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&countBefore); err != nil {
		t.Fatalf("count before: %v", err)
	}

	// Second run must be no-op
	if err := Run(ctx, pool, dir); err != nil {
		t.Fatalf("second Run: %v", err)
	}

	var countAfter int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&countAfter); err != nil {
		t.Fatalf("count after: %v", err)
	}
	t.Logf("migration count BEFORE second Run: %d, AFTER: %d", countBefore, countAfter)
	if countBefore != countAfter {
		t.Fatalf("second Run mutated migration count: %d → %d", countBefore, countAfter)
	}
}

// --- Test 4: unexpected legacy migration table on non-empty schema fails closed ---

func TestIntegration_LegacyTableOnNonEmptySchemaFailsClosed(t *testing.T) {
	cfg := loadConfig(t)
	pool := connectTestDB(t, cfg)
	ctx := context.Background()

	resetPublicSchema(t, ctx, pool)

	// Create a user table to simulate non-empty schema
	if _, err := pool.Exec(ctx, `CREATE TABLE test_tbl (id serial primary key, val text)`); err != nil {
		t.Fatalf("create test table: %v", err)
	}

	// Create a legacy golang-migrate table
	if _, err := pool.Exec(ctx, `CREATE TABLE schema_migrations (version BIGINT PRIMARY KEY, dirty BOOLEAN NOT NULL)`); err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	// Insert a row to simulate dirty state
	if _, err := pool.Exec(ctx, `INSERT INTO schema_migrations (version, dirty) VALUES (1, true)`); err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}

	// Verify pre-condition: schema has data + other tables
	var nonMigrationTableCount int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM pg_tables WHERE schemaname='public' AND tablename<>'schema_migrations'`).Scan(&nonMigrationTableCount)
	t.Logf("table state BEFORE: non-migration tables=%d, schema_migrations exists with legacy format", nonMigrationTableCount)

	// EnsureSchemaMigrationsTable must fail closed
	err := EnsureSchemaMigrationsTable(ctx, pool)
	if err == nil {
		t.Fatal("expected error on legacy table, got nil")
	}
	if err != ErrLegacyMigrationTable {
		t.Fatalf("expected ErrLegacyMigrationTable, got: %v", err)
	}
	t.Logf("correctly failed closed: %v", err)

	// Verify post-condition: legacy table is still there (no DROP)
	var legacyStillExists bool
	_ = pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_tables WHERE schemaname='public' AND tablename='schema_migrations')`).Scan(&legacyStillExists)
	t.Logf("table state AFTER: schema_migrations still exists=%v (no destructive mutation)", legacyStillExists)
	if !legacyStillExists {
		t.Fatal("legacy table was dropped — destructive mutation occurred!")
	}

	// Other tables are untouched
	var testTableStillExists bool
	_ = pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_tables WHERE schemaname='public' AND tablename='test_tbl')`).Scan(&testTableStillExists)
	if !testTableStillExists {
		t.Fatal("test table was dropped — destructive mutation occurred!")
	}
}

// --- Test 5: dirty legacy table fails closed ---

func TestIntegration_DirtyLegacyTableFailsClosed(t *testing.T) {
	cfg := loadConfig(t)
	pool := connectTestDB(t, cfg)
	ctx := context.Background()

	resetPublicSchema(t, ctx, pool)

	// Create legacy table with dirty=true (mimics the exact stuck state)
	if _, err := pool.Exec(ctx, `CREATE TABLE schema_migrations (version BIGINT PRIMARY KEY, dirty BOOLEAN NOT NULL)`); err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO schema_migrations (version, dirty) VALUES (1, true)`); err != nil {
		t.Fatalf("insert dirty row: %v", err)
	}

	var version int
	var dirty bool
	_ = pool.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations`).Scan(&version, &dirty)
	t.Logf("table state BEFORE: version=%d dirty=%v", version, dirty)

	err := EnsureSchemaMigrationsTable(ctx, pool)
	if err == nil {
		t.Fatal("expected error on dirty legacy table, got nil")
	}
	if err != ErrLegacyMigrationTable {
		t.Fatalf("expected ErrLegacyMigrationTable, got: %v", err)
	}

	var exists bool
	_ = pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_tables WHERE schemaname='public' AND tablename='schema_migrations')`).Scan(&exists)
	t.Logf("table state AFTER: schema_migrations still exists=%v (no destructive mutation)", exists)
	if !exists {
		t.Fatal("dirty legacy table was dropped — destructive mutation occurred!")
	}
}

// --- Test 6: no migration metadata is deleted on failure ---

func TestIntegration_NoMetadataDeletedOnFailure(t *testing.T) {
	cfg := loadConfig(t)
	pool := connectTestDB(t, cfg)
	ctx := context.Background()

	resetPublicSchema(t, ctx, pool)

	// Create a legacy table with some "applied" data
	if _, err := pool.Exec(ctx, `CREATE TABLE schema_migrations (version BIGINT PRIMARY KEY, dirty BOOLEAN NOT NULL)`); err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO schema_migrations (version, dirty) VALUES (1, false),(2, false),(3, false)`); err != nil {
		t.Fatalf("insert legacy rows: %v", err)
	}

	var rowCountBefore int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&rowCountBefore)
	t.Logf("table state BEFORE: %d metadata rows in legacy table", rowCountBefore)

	// This must fail
	err := EnsureSchemaMigrationsTable(ctx, pool)
	if err == nil {
		t.Fatal("expected error on legacy table")
	}

	// Metadata rows are untouched
	var rowCountAfter int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&rowCountAfter)
	t.Logf("table state AFTER: %d metadata rows (must equal BEFORE: %d)", rowCountAfter, rowCountBefore)
	if rowCountAfter != rowCountBefore {
		t.Fatalf("metadata was deleted: %d → %d rows", rowCountBefore, rowCountAfter)
	}
}

// --- Test 7: testdb can recover by recreating the disposable database ---

// This test simulates the exact testdb recovery path: drop legacy table,
// then let EnsureSchemaMigrationsTable create the canon table, then Run.

func TestIntegration_TestdbRecoveryPath(t *testing.T) {
	cfg := loadConfig(t)
	pool := connectTestDB(t, cfg)
	ctx := context.Background()

	resetPublicSchema(t, ctx, pool)

	// Step 1: Create the stuck dirty-state legacy table (exactly like labuda_test was)
	if _, err := pool.Exec(ctx, `CREATE TABLE schema_migrations (version BIGINT PRIMARY KEY, dirty BOOLEAN NOT NULL)`); err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO schema_migrations (version, dirty) VALUES (1, true)`); err != nil {
		t.Fatalf("insert dirty row: %v", err)
	}
	t.Logf("table state STEP 1 (stuck legacy): version=1, dirty=true")

	// Step 2: testdb's recovery — detect legacy format and drop it
	var hasNameCol bool
	_ = pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM information_schema.columns
		               WHERE table_schema='public' AND table_name='schema_migrations' AND column_name='name')
	`).Scan(&hasNameCol)
	if !hasNameCol {
		var legacyExists bool
		_ = pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_tables WHERE schemaname='public' AND tablename='schema_migrations')`).Scan(&legacyExists)
		if legacyExists {
			t.Logf("STEP 2 (testdb recovery): dropping legacy schema_migrations table")
			if _, err := pool.Exec(ctx, `DROP TABLE schema_migrations`); err != nil {
				t.Fatalf("drop legacy: %v", err)
			}
		}
	}

	// Step 3: Now the shared executor should succeed
	if err := EnsureSchemaMigrationsTable(ctx, pool); err != nil {
		t.Fatalf("EnsureSchemaMigrationsTable after recovery: %v", err)
	}

	// Step 4: Run all migrations
	dir := resolveMigrationsDir(t)
	if err := Run(ctx, pool, dir); err != nil {
		t.Fatalf("Run after recovery: %v", err)
	}

	var count int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&count)
	t.Logf("table state STEP 4 (recovered): %d migrations applied", count)
	if count != 41 {
		t.Fatalf("expected 41 migrations after recovery, got %d", count)
	}
}

// --- Test 8: second clean testdb run succeeds ---

func TestIntegration_SecondRunIsIdempotent(t *testing.T) {
	cfg := loadConfig(t)
	pool := connectTestDB(t, cfg)
	ctx := context.Background()
	dir := resolveMigrationsDir(t)

	if err := Run(ctx, pool, dir); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	var countAfterFirst int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&countAfterFirst)
	t.Logf("table state AFTER first Run: %d migrations", countAfterFirst)

	if err := Run(ctx, pool, dir); err != nil {
		t.Fatalf("second Run: %v", err)
	}
	var countAfterSecond int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&countAfterSecond)
	t.Logf("table state AFTER second Run: %d migrations", countAfterSecond)

	if countAfterSecond != countAfterFirst {
		t.Fatalf("second Run changed migration count: %d → %d", countAfterFirst, countAfterSecond)
	}

	// No dirty column
	var hasDirty bool
	_ = pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='schema_migrations' AND column_name='dirty')`).Scan(&hasDirty)
	if hasDirty {
		t.Fatal("dirty column found — legacy golang-migrate table is present")
	}

	// Key tables exist
	expectedTables := []string{
		"users", "user_profiles", "auctions", "for_sales",
		"orders", "order_items", "payments", "escrows",
		"contents", "chat_rooms", "chat_messages",
		"shipping_quotes", "bank_accounts",
	}
	for _, tbl := range expectedTables {
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name=$1)`, tbl).Scan(&exists); err != nil {
			t.Fatalf("check table %q: %v", tbl, err)
		}
		if !exists {
			t.Errorf("expected table %q does not exist", tbl)
		}
	}
}

// --- Test 9: failed migration rolls back without recording the version ---

func TestIntegration_FailedMigrationDoesNotPoisonNextRun(t *testing.T) {
	cfg := loadConfig(t)
	pool := connectTestDB(t, cfg)
	ctx := context.Background()
	resetPublicSchema(t, ctx, pool)

	dir := resolveMigrationsDir(t)

	// Build a fake migration set with the first real migration + a bad one.
	fakeDir := t.TempDir()
	realMigrations, err := LoadMigrations(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(realMigrations) == 0 {
		t.Fatal("no migrations found")
	}

	m1 := realMigrations[0]
	dst := filepath.Join(fakeDir, fmt.Sprintf("%06d_%s.up.sql", m1.Version, m1.Name))
	if err := os.WriteFile(dst, []byte(m1.UpSQL), 0o644); err != nil {
		t.Fatal(err)
	}

	badFile := filepath.Join(fakeDir, "000999_failing_test.up.sql")
	if err := os.WriteFile(badFile, []byte("THIS IS NOT VALID SQL;"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Run against fake dir — migration 1 should succeed, 999 fail
	err = Run(ctx, pool, fakeDir)
	if err == nil {
		t.Fatal("expected Run to fail on injected bad migration")
	}
	t.Logf("expected failure: %v", err)

	// Migration 1 should be recorded
	var m1Exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, m1.Version).Scan(&m1Exists); err != nil {
		t.Fatalf("check migration 1: %v", err)
	}
	if !m1Exists {
		t.Fatal("migration 1 was not recorded — successful migration was lost")
	}
	t.Logf("table state AFTER failure: migration 1 recorded, 999 NOT recorded (correct)")

	// Bad migration 999 should NOT be recorded
	var badExists bool
	_ = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=999)`).Scan(&badExists)
	if badExists {
		t.Fatal("bad migration 999 was incorrectly recorded as applied")
	}

	// Only 1 migration should be recorded
	var count int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&count)
	if count != 1 {
		t.Fatalf("expected 1 applied migration, got %d", count)
	}

	// Fresh Run against real dir should succeed
	if err := Run(ctx, pool, dir); err != nil {
		t.Fatalf("second Run after failed migration: %v", err)
	}

	var finalCount int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&finalCount)
	t.Logf("table state AFTER recovery Run: %d migrations applied", finalCount)
	if finalCount != 41 {
		t.Fatalf("expected 41 applied migrations after recovery, got %d", finalCount)
	}
}

// --- Test: all 41 migrations apply ---

func TestIntegration_AllMigrationsApply(t *testing.T) {
	cfg := loadConfig(t)
	pool := connectTestDB(t, cfg)
	ctx := context.Background()
	resetPublicSchema(t, ctx, pool)

	dir := resolveMigrationsDir(t)
	if err := Run(ctx, pool, dir); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var count int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&count)
	t.Logf("table state AFTER: %d migrations applied", count)
	if count != 41 {
		t.Fatalf("expected 41 migrations, got %d", count)
	}

	var maxVersion int
	_ = pool.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&maxVersion)
	if maxVersion != 41 {
		t.Fatalf("expected max version 41, got %d", maxVersion)
	}
}

var _ = fmt.Sprintf
