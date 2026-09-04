// Package testdb provides isolated database testing infrastructure.
//
// It ensures tests use a separate test database (labuda_test) instead of
// the development database, preventing test data from polluting development.
//
// USAGE:
//
//	testDB, cleanup := testdb.Setup(t)
//	defer cleanup()
//
//	// Use testDB in your tests
//	testDB.WithTx(ctx, func(tx db.Tx) error {
//	    // ... test code
//	    return nil
//	})
package testdb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/migration"
)

var (
	migrateOnce sync.Once
	migrateErr  error
)

const testDBMigrationLockKey = "labuda:testdb:migrations"

// testDBBootstrapBeforeReset is a test hook that fires after the advisory lock
// has been acquired and before the disposable schema is reset.
var testDBBootstrapBeforeReset func()

// TestDB wraps a database connection for testing with cleanup utilities.
type TestDB struct {
	pool     *pgxpool.Pool
	config   *config.Config
	database string
}

// Setup creates and initializes a test database connection.
//
// It:
//  1. Connects to the test database (labuda_test by default)
//  2. Runs migrations if not already run (cached per test run)
//  3. Returns a cleanup function that truncates all tables
//
// The cleanup function ensures tests don't leave data behind.
//
// IMPORTANT: This will NEVER connect to the development database.
// If test database cannot be used, it will fail the test immediately.
//
// LIFECYCLE LOCK: Setup acquires a PostgreSQL advisory lock that is held for
// the entire lifetime of the test binary (migration + tests + cleanup).
// This prevents a concurrent test binary from dropping the public schema
// while this binary's tests are actively using it. The lock is released
// during cleanup, after TruncateAll completes.
func Setup(t *testing.T, cfg *config.Config) (*TestDB, func()) {
	t.Helper()

	// SAFETY CHECK: Verify we're not accidentally using dev database
	testDBName := cfg.Database.TestName
	if testDBName == "" || testDBName == cfg.Database.Name {
		t.Fatalf("TEST DB SAFETY FAIL: Test database name '%s' conflicts with main database '%s'. Set DB_TEST_NAME environment variable.",
			testDBName, cfg.Database.Name)
	}

	// Parse test DSN
	dsn := cfg.Database.GetTestDSN()

	// Acquire the lifecycle advisory lock BEFORE migrations.
	// This lock is held through migration, test execution, and cleanup,
	// preventing concurrent test binaries from dropping the public schema
	// while this binary's tests are active.
	if err := acquireLifecycleLock(dsn); err != nil {
		t.Fatalf("Failed to acquire lifecycle advisory lock: %v", err)
	}

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		releaseLifecycleLock()
		t.Fatalf("Failed to parse test DSN: %v\nDSN: %s", err, redactPassword(dsn))
	}

	// Limit connections for tests
	poolConfig.MaxConns = 5
	poolConfig.MinConns = 0

	// Run migrations (once per test run)
	migrateOnce.Do(func() {
		migrateErr = runMigrationsRaw(cfg, t.Logf)
	})
	if migrateErr != nil {
		releaseLifecycleLock()
		t.Fatalf("Failed to run test database migrations: %v", migrateErr)
	}

	poolCtx, poolCancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer poolCancel()

	pool, err := pgxpool.NewWithConfig(poolCtx, poolConfig)
	if err != nil {
		releaseLifecycleLock()
		t.Fatalf("Failed to connect to test database '%s': %v\n\nHINT: Ensure test database exists:\n  createdb -U %s -h %s -p %s %s\n  Or use docker exec:\n  docker exec -it labuda-postgres createdb -U labuda labuda_test",
			testDBName, err, cfg.Database.User, cfg.Database.Host, cfg.Database.Port, testDBName)
	}

	// Verify we're actually connected to test DB (not dev!)
	var currentDB string
	err = pool.QueryRow(poolCtx, "SELECT current_database()").Scan(&currentDB)
	if err != nil {
		releaseLifecycleLock()
		pool.Close()
		t.Fatalf("Failed to verify test database: %v", err)
	}

	if currentDB != testDBName {
		releaseLifecycleLock()
		pool.Close()
		t.Fatalf("TEST DB SAFETY FAIL: Connected to '%s' but expected '%s'. This indicates a configuration error.", currentDB, testDBName)
	}

	tdb := &TestDB{
		pool:     pool,
		config:   cfg,
		database: currentDB,
	}

	// Return cleanup function
	cleanup := func() {
		if t.Failed() {
			// On test failure, keep data for debugging
			t.Logf("Test failed - database '%s' NOT cleaned up for inspection", currentDB)
		} else {
			// On success, truncate all tables.
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cleanupCancel()
			pool.Close()
			if err := tdb.TruncateAll(cleanupCtx); err != nil {
				t.Logf("Warning: Failed to truncate test tables: %v", err)
			}
			// Release the lifecycle advisory lock after cleanup.
			// This allows a waiting concurrent test binary to proceed.
			releaseLifecycleLock()
			return
		}
		pool.Close()
		releaseLifecycleLock()
	}

	return tdb, cleanup
}

// SetupDB is a convenience wrapper that loads config and calls Setup.
//
// It walks up the directory tree from the test binary's working directory to
// find the backend .env file. When running tests from a nested package such
// as internal/identity/auth/delivery/http, the working directory is that
// package's directory, not backend/. Without this walk the godotenv.Load()
// inside config.Load() silently skips the file, causing DB_NAME to be unset
// and config.Load() to fail with "DB_NAME is required but not set".
func SetupDB(t *testing.T) (*TestDB, func()) {
	t.Helper()

	loadDotEnvFromParents(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Check for TEST_MODE env var
	if os.Getenv("TEST_MODE") == "true" {
		t.Log("TEST_MODE=true - using isolated test database")
	}

	return Setup(t, cfg)
}

// loadDotEnvFromParents walks up from the current working directory looking
// for a .env file (up to 8 levels). This is needed because go test sets the
// working directory to the package directory, not the module/backend root where
// .env lives.
//
// Only the first .env found is loaded. If none is found, the function returns
// silently — callers rely on explicit environment variables in that case (e.g.
// CI).
func loadDotEnvFromParents(t *testing.T) {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		return
	}

	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, ".env")
		if _, statErr := os.Stat(candidate); statErr == nil {
			if loadErr := godotenv.Load(candidate); loadErr != nil {
				t.Logf("testdb: loaded .env from %s", candidate)
			}
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
}

// WithTx executes a function within a transaction.
//
// Goexit-safe (PASS_19D): fn commonly calls testify's require.* (or
// t.Fatal), which aborts the calling goroutine via runtime.Goexit rather
// than a normal return. When that happens mid-call, the assignment that
// would have captured fn's returned error never executes — so a rollback
// guarded by checking that outer error variable never fires, and the
// transaction (and its pooled connection) leaks forever. That leak then
// hangs any later pgxpool.Pool.Close() in test cleanup, waiting forever for
// a connection that will never be released (observed as a 600s test hang in
// TestForSaleStockRoundTrip_MultiQty). Guarding the deferred rollback with a
// "committed" flag set only after a successful Commit fixes this: the defer
// still runs during Goexit unwinding (it's registered on this function's own
// stack frame, which Goexit unwinds through), and "committed" is false on
// every exit path except an actual commit — normal error return, panic, or
// Goexit alike.
func (tdb *TestDB) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	pgxTx, err := tdb.pool.Begin(ctx)
	if err != nil {
		return err
	}
	tx := &dbTx{tx: pgxTx}

	committed := false
	defer func() {
		if !committed {
			pgxTx.Rollback(ctx)
		}
	}()

	if err := fn(tx); err != nil {
		return err
	}

	if err := pgxTx.Commit(ctx); err != nil {
		return err
	}
	committed = true
	return nil
}

// Pool returns the underlying pgxpool.Pool for direct queries.
func (tdb *TestDB) Pool() *pgxpool.Pool {
	return tdb.pool
}

// DatabaseName returns the name of the test database.
func (tdb *TestDB) DatabaseName() string {
	return tdb.database
}

// TruncateAll truncates all tables in the test database in a single SQL statement.
//
// This is called automatically by the cleanup function on test success.
//
// PERFORMANCE NOTE: Using a single TRUNCATE with a comma-separated table list is
// significantly faster than individual TRUNCATE statements because PostgreSQL can
// resolve FK dependencies in one pass and acquire all locks atomically. On a schema
// with 100+ tables, individual truncates can take > 30s; a single bulk truncate
// typically completes in < 5s.
//
// session_replication_role='replica' suppresses FK trigger checks and deferred
// constraint enforcement, further reducing per-row overhead.
func (tdb *TestDB) TruncateAll(ctx context.Context) error {
	// Use a fresh cleanup connection so teardown does not depend on the
	// live test pool being able to hand out an idle connection. Repeated
	// integration runs can briefly exhaust the test pool if a test leaks a
	// connection; cleanup still needs to succeed so the next iteration starts
	// from a clean database.
	cleanupDB, err := db.New(ctx, db.Config{ConnString: tdb.config.Database.GetTestDSN()})
	if err != nil {
		return fmt.Errorf("open cleanup database: %w", err)
	}
	defer cleanupDB.Close()

	pool := cleanupDB.Pool()

	// Collect all user table names (excluding schema_migrations)
	rows, err := pool.Query(ctx, `
		SELECT tablename
		FROM pg_tables
		WHERE schemaname = 'public'
		  AND tablename <> 'schema_migrations'
		ORDER BY tablename;
	`)
	if err != nil {
		return fmt.Errorf("query tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return fmt.Errorf("scan table name: %w", err)
		}
		tables = append(tables, table)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate tables: %w", err)
	}

	if len(tables) == 0 {
		return nil
	}

	// Disable FK trigger enforcement for the session so cascade resolution is fast.
	_, err = pool.Exec(ctx, "SET session_replication_role = 'replica';")
	if err != nil {
		return fmt.Errorf("disable triggers: %w", err)
	}

	// Build a single TRUNCATE for all tables — orders of magnitude faster than N
	// individual statements because lock acquisition and FK graph traversal happen
	// once. CASCADE is still specified to handle any remaining dependency edges.
	tableList := ""
	for i, t := range tables {
		if i > 0 {
			tableList += ", "
		}
		tableList += t
	}
	_, err = pool.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE;", tableList))
	if err != nil {
		return fmt.Errorf("truncate all tables: %w", err)
	}

	// Re-enable triggers
	_, err = pool.Exec(ctx, "SET session_replication_role = 'origin';")
	if err != nil {
		return fmt.Errorf("enable triggers: %w", err)
	}

	return nil
}

// dbTx wraps pgx.Tx to implement db.Tx interface
type dbTx struct {
	tx pgx.Tx
}

func (t *dbTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return t.tx.QueryRow(ctx, sql, args...)
}

func (t *dbTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return t.tx.Query(ctx, sql, args...)
}

func (t *dbTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return t.tx.Exec(ctx, sql, args...)
}

func (t *dbTx) Commit(ctx context.Context) error {
	return t.tx.Commit(ctx)
}

func (t *dbTx) Rollback(ctx context.Context) error {
	return t.tx.Rollback(ctx)
}

// runMigrations runs database migrations on the test database using the
// canonical migration executor.
func runMigrations(cfg *config.Config, t *testing.T) error {
	return runMigrationsWithLogger(cfg, t.Logf)
}

// runMigrationsWithLogger resets the test database schema and applies all
// pending migrations. It acquires its own advisory lock to serialize
// concurrent callers across test binaries that share labuda_test.
//
// NOTE: This function is also called directly by
// TestConcurrentBootstrapSerialization, so it must retain its own lock
// acquisition. For the normal Setup path, the lifecycle lock in Setup
// provides broader protection (see runMigrationsRaw).
func runMigrationsWithLogger(cfg *config.Config, logf func(string, ...any)) error {
	// Acquire a per-call advisory lock so concurrent callers serialize.
	resetPool, err := pgxpool.New(context.Background(), cfg.Database.GetTestDSN())
	if err != nil {
		return fmt.Errorf("open migration lock pool: %w", err)
	}
	defer resetPool.Close()

	lockConn, err := resetPool.Acquire(context.Background())
	if err != nil {
		return fmt.Errorf("acquire migration lock connection: %w", err)
	}
	defer lockConn.Release()

	if _, err := lockConn.Exec(context.Background(), `SELECT pg_advisory_lock(hashtextextended($1, 0))`, testDBMigrationLockKey); err != nil {
		return fmt.Errorf("acquire migration bootstrap lock: %w", err)
	}
	defer func() {
		_, _ = lockConn.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtextextended($1, 0))`, testDBMigrationLockKey)
	}()

	return runMigrationsRaw(cfg, logf)
}

// lifecycleLockPool holds the pool created by acquireLifecycleLock.
// It is closed in releaseLifecycleLock after the connection is released,
// preventing a connection leak per Setup() call.
var lifecycleLockPool *pgxpool.Pool

// lifecycleLockConn holds the advisory lock connection for the entire
// test binary lifecycle (migration + test execution + cleanup).
var lifecycleLockConn *pgxpool.Conn

// acquireLifecycleLock acquires the advisory lock that spans the entire
// test binary lifecycle (migration + test execution + cleanup).
// This prevents a concurrent test binary from dropping the public schema
// while this binary's tests are actively using it.
func acquireLifecycleLock(dsn string) error {
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return fmt.Errorf("open lifecycle lock connection: %w", err)
	}
	conn, err := pool.Acquire(context.Background())
	if err != nil {
		pool.Close()
		return fmt.Errorf("acquire lifecycle lock connection: %w", err)
	}

	if _, err := conn.Exec(context.Background(), `SELECT pg_advisory_lock(hashtextextended($1, 0))`, testDBMigrationLockKey); err != nil {
		conn.Release()
		pool.Close()
		return fmt.Errorf("acquire lifecycle advisory lock: %w", err)
	}
	lifecycleLockPool = pool
	lifecycleLockConn = conn
	return nil
}

// releaseLifecycleLock releases the advisory lock acquired by
// acquireLifecycleLock. It is safe to call even if no lock is held.
func releaseLifecycleLock() {
	if lifecycleLockConn != nil {
		_, _ = lifecycleLockConn.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtextextended($1, 0))`, testDBMigrationLockKey)
		lifecycleLockConn.Release()
		lifecycleLockConn = nil
	}
	if lifecycleLockPool != nil {
		lifecycleLockPool.Close()
		lifecycleLockPool = nil
	}
}

// runMigrationsRaw resets the test database schema and applies all pending
// migrations. It does NOT manage advisory locks — the caller is
// responsible for serialization.
func runMigrationsRaw(cfg *config.Config, logf func(string, ...any)) error {
	// Try progressively deeper parent directories to locate the migrations dir.
	candidates := []string{
		"migrations",
		"../migrations",
		"../../migrations",
		"../../../migrations",
		"../../../../migrations",
		"../../../../../migrations",
		"../../../../../../migrations",
	}

	var migrationsDir string
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			migrationsDir = candidate
			break
		}
	}

	if migrationsDir == "" {
		if logf != nil {
			logf("WARN: migration source not found, skipping auto-migrate")
		}
		return nil
	}

	// Discard any stale public-schema objects and the extensions that the
	// canonical migration chain recreates. This keeps the disposable test
	// database alive while still giving us a clean bootstrap state.
	resetPool, err := pgxpool.New(context.Background(), cfg.Database.GetTestDSN())
	if err != nil {
		return fmt.Errorf("open migration cleanup connection: %w", err)
	}
	defer resetPool.Close()

	conn, err := resetPool.Acquire(context.Background())
	if err != nil {
		return fmt.Errorf("acquire migration cleanup connection: %w", err)
	}
	defer conn.Release()

	if testDBBootstrapBeforeReset != nil {
		testDBBootstrapBeforeReset()
	}

	for _, stmt := range []string{
		`DROP EXTENSION IF EXISTS btree_gist CASCADE`,
		`DROP EXTENSION IF EXISTS "uuid-ossp" CASCADE`,
		`DROP EXTENSION IF EXISTS pgcrypto CASCADE`,
		`DROP SCHEMA IF EXISTS public CASCADE`,
		`CREATE SCHEMA IF NOT EXISTS public`,
	} {
		if _, err := conn.Exec(context.Background(), stmt); err != nil {
			return fmt.Errorf("reset statement %q: %w", stmt, err)
		}
	}
	resetPool.Reset()

	// Start a fresh pool for the actual migration run so the executor sees
	// the rebuilt schema through brand-new sessions.
	runPool, err := pgxpool.New(context.Background(), cfg.Database.GetTestDSN())
	if err != nil {
		return fmt.Errorf("open migration run connection: %w", err)
	}
	defer runPool.Close()

	if err := migration.Run(context.Background(), runPool, migrationsDir); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	if logf != nil {
		logf("Test database migrations completed using canonical runner")
	}
	return nil
}

// Connection timeout for test database
const connectionTimeout = 10 * time.Second

// redactPassword removes password from DSN for safe logging
func redactPassword(dsn string) string {
	// Simple regex replacement for password field
	result := ""
	passIdx := indexOf(dsn, "password=")
	if passIdx == -1 {
		return dsn
	}
	result = dsn[:passIdx+9] // "password="

	// Find end of password value
	endIdx := passIdx + 9
	for endIdx < len(dsn) && dsn[endIdx] != ' ' && dsn[endIdx] != '\'' {
		endIdx++
	}

	result += "*****"
	if endIdx < len(dsn) {
		result += dsn[endIdx:]
	}
	return result
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
