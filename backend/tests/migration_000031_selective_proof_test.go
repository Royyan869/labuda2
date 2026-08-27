//go:build integration

package tests

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/joho/godotenv"
	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/pkg/migration"
)

// testOnlyRunTo is a TEST-ONLY helper that applies migrations up to maxVersion.
func testOnlyRunTo(ctx context.Context, pool *pgxpool.Pool, dir string, maxVer int) error {
	ms, err := migration.LoadMigrations(dir)
	if err != nil {
		return err
	}
	for _, m := range ms {
		if m.Version > maxVer {
			continue
		}
		var exists bool
		if err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)", m.Version).Scan(&exists); err != nil {
			return fmt.Errorf("check %d: %w", m.Version, err)
		}
		if exists {
			continue
		}
		stmts := migration.Split(m.UpSQL)
		var filtered []string
		for _, s := range stmts {
			s = strings.TrimSpace(s)
			if s == "" || strings.EqualFold(s, "begin;") || strings.EqualFold(s, "commit;") {
				continue
			}
			filtered = append(filtered, s)
		}
		if strings.Contains(strings.ToUpper(m.UpSQL), "ADD VALUE") {
			for i, s := range filtered {
				if _, err := pool.Exec(ctx, s); err != nil {
					return fmt.Errorf("mig %d stmt %d: %w", m.Version, i+1, err)
				}
			}
			if _, err := pool.Exec(ctx, "INSERT INTO schema_migrations (version,name) VALUES ($1,$2)", m.Version, m.Name); err != nil {
				return err
			}
		} else {
			tx, err := pool.Begin(ctx)
			if err != nil {
				return err
			}
			ok := false
			defer func() { if !ok { tx.Rollback(ctx) } }()
			for i, s := range filtered {
				if _, err := tx.Exec(ctx, s); err != nil {
					return fmt.Errorf("mig %d stmt %d: %w", m.Version, i+1, err)
				}
			}
			if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version,name) VALUES ($1,$2)", m.Version, m.Name); err != nil {
				return err
			}
			if err := tx.Commit(ctx); err != nil {
				return err
			}
			ok = true
		}
	}
	return nil
}

func newIsolatedPool(t *testing.T, suffix string) (*pgxpool.Pool, func()) {
	t.Helper()
	for _, p := range []string{".env", "../.env", "../../.env", "../../../.env", "../../../../.env"} {
		if _, err := os.Stat(p); err == nil {
			_ = godotenv.Load(p)
			break
		}
	}
	cfg, err := config.Load()
	require.NoError(t, err)
	baseDSN := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password, cfg.Database.Name, cfg.Database.SSLMode)
	poolCfg, _ := pgxpool.ParseConfig(baseDSN)
	poolCfg.MaxConns = 5
	basePool, _ := pgxpool.NewWithConfig(context.Background(), poolCfg)
	testDB := fmt.Sprintf("labuda_test_mig_%s_%d", suffix, time.Now().UnixNano())
	_, _ = basePool.Exec(context.Background(), fmt.Sprintf("CREATE DATABASE %s", testDB))
	basePool.Close()
	testDSN := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password, testDB, cfg.Database.SSLMode)
	testCfg, _ := pgxpool.ParseConfig(testDSN)
	testCfg.MaxConns = 5
	testPool, _ := pgxpool.NewWithConfig(context.Background(), testCfg)
	cleanup := func() {
		testPool.Close()
		cp, _ := pgxpool.NewWithConfig(context.Background(), poolCfg)
		defer cp.Close()
		cp.Exec(context.Background(), fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", testDB))
	}
	return testPool, cleanup
}

func findMigrationsDir(t *testing.T) string {
	t.Helper()
	for _, d := range []string{"migrations", "../migrations", "../../migrations", "../../../migrations", "../../../../migrations"} {
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			return d
		}
	}
	t.Skip("migrations dir not found")
	return ""
}

func setupTo000030(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()
	dir := findMigrationsDir(t)
	require.NoError(t, migration.EnsureSchemaMigrationsTable(ctx, pool))
	_, err := pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS pgcrypto")
	require.NoError(t, err)
	require.NoError(t, testOnlyRunTo(ctx, pool, dir, 30))
	var v int
	require.NoError(t, pool.QueryRow(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&v))
	require.Equal(t, 30, v)
	return dir
}

// ── VALID ─────────────────────────────────────────────────────────────

func TestTrueSelectiveMigration_ValidTransition(t *testing.T) {
	pool, cleanup := newIsolatedPool(t, "valid")
	defer cleanup()
	ctx := context.Background()
	dir := setupTo000030(t, pool)

	user1 := seedMigrationUser(t, ctx, pool, "active")
	user2 := seedMigrationUser(t, ctx, pool, "active")
	p1 := seedMigrationProduct(t, ctx, pool, user1)
	p2 := seedMigrationProduct(t, ctx, pool, user1)
	fps1 := seedMigrationFPS(t, ctx, pool, user1, p1)
	fps2 := seedMigrationFPS(t, ctx, pool, user1, p2)
	content := seedMigrationContent(t, ctx, pool, user2, "public")

	n1, n2 := uuid.New(), uuid.New()
	r1, r2 := uuid.New(), uuid.New()

	_, err := pool.Exec(ctx, `INSERT INTO comments(id,author_id,body,type,target_id,target_type,created_at,updated_at) VALUES($1,$2,$3,'normal',$4,'content',NOW(),NOW())`, n1, user2, "n1", content)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO comments(id,author_id,body,type,target_id,target_type,created_at,updated_at) VALUES($1,$2,$3,'normal',$4,'content',NOW(),NOW())`, n2, user2, "n2", content)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO comments(id,author_id,body,type,for_sale_id,target_id,target_type,created_at,updated_at) VALUES($1,$2,$3,'listing_reference',$4,$5,'content',NOW(),NOW())`, r1, user1, "ref1", fps1, content)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO comments(id,author_id,type,for_sale_id,target_id,target_type,created_at,updated_at) VALUES($1,$2,'listing_reference',$3,$4,'content',NOW(),NOW())`, r2, user1, fps2, content)
	require.NoError(t, err)

	var preCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM comments WHERE deleted_at IS NULL`).Scan(&preCount))
	require.Equal(t, 4, preCount)

	require.NoError(t, testOnlyRunTo(ctx, pool, dir, 31))

	var postCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM comments WHERE deleted_at IS NULL`).Scan(&postCount))
	require.Equal(t, preCount, postCount)

	var assoc int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM comment_commerce_references`).Scan(&assoc))
	require.Equal(t, 2, assoc)

	for _, cid := range []uuid.UUID{n1, n2, r1, r2} {
		var ex bool
		require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM comments WHERE id=$1)`, cid).Scan(&ex))
		require.True(t, ex)
	}
	for _, fps := range []uuid.UUID{fps1, fps2} {
		var ex bool
		require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM comment_commerce_references WHERE for_sale_id=$1)`, fps).Scan(&ex))
		require.True(t, ex)
	}
	for _, cid := range []uuid.UUID{n1, n2} {
		var has bool
		require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM comment_commerce_references WHERE comment_id=$1)`, cid).Scan(&has))
		require.False(t, has)
	}
	var tc, fc bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='comments' AND column_name='type')`).Scan(&tc))
	require.False(t, tc)
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='comments' AND column_name='for_sale_id')`).Scan(&fc))
	require.False(t, fc)
	var en bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_type WHERE typname='comment_type_enum')`).Scan(&en))
	require.False(t, en)
}

// ── INVALID ───────────────────────────────────────────────────────────

// disableLegacyConsistencyCheck removes the old CHECK constraint that prevents
// inserting corrupt legacy data. TEST-ONLY — emulates a truly corrupted legacy
// database that bypassed the constraint.
func disableLegacyConsistencyCheck(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(ctx, `ALTER TABLE comments DROP CONSTRAINT IF EXISTS comments_listing_ref_consistency_check`)
	require.NoError(t, err)
}

func TestInvalidTransition_OrphanAborts(t *testing.T) {
	pool, cleanup := newIsolatedPool(t, "orphan")
	defer cleanup()
	ctx := context.Background()
	dir := setupTo000030(t, pool)

	// Drop old CHECK to emulate bypassed constraint in corrupt legacy DB
	disableLegacyConsistencyCheck(t, ctx, pool)

	u := seedMigrationUser(t, ctx, pool, "active")
	c := seedMigrationContent(t, ctx, pool, u, "public")
	_, err := pool.Exec(ctx, `INSERT INTO comments(id,author_id,body,type,for_sale_id,target_id,target_type,created_at,updated_at) VALUES($1,$2,$3,'listing_reference',NULL,$4,'content',NOW(),NOW())`, uuid.New(), u, "orphan", c)
	require.NoError(t, err)

	err = testOnlyRunTo(ctx, pool, dir, 31)
	require.Error(t, err, "orphan must abort 000031")

	var rec bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=31)`).Scan(&rec))
	require.False(t, rec)
	var tc bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='comments' AND column_name='type')`).Scan(&tc))
	require.True(t, tc)
}

func TestInvalidTransition_LeakAborts(t *testing.T) {
	pool, cleanup := newIsolatedPool(t, "leak")
	defer cleanup()
	ctx := context.Background()
	dir := setupTo000030(t, pool)

	// Drop old CHECK to emulate bypassed constraint in corrupt legacy DB
	disableLegacyConsistencyCheck(t, ctx, pool)

	u := seedMigrationUser(t, ctx, pool, "active")
	p := seedMigrationProduct(t, ctx, pool, u)
	f := seedMigrationFPS(t, ctx, pool, u, p)
	c := seedMigrationContent(t, ctx, pool, u, "public")
	_, err := pool.Exec(ctx, `INSERT INTO comments(id,author_id,body,type,for_sale_id,target_id,target_type,created_at,updated_at) VALUES($1,$2,$3,'normal',$4,$5,'content',NOW(),NOW())`, uuid.New(), u, "leak", f, c)
	require.NoError(t, err)

	err = testOnlyRunTo(ctx, pool, dir, 31)
	require.Error(t, err, "leak must abort 000031")

	var rec bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=31)`).Scan(&rec))
	require.False(t, rec)
}

func TestInvalidTransition_MissingSourceAborts(t *testing.T) {
	pool, cleanup := newIsolatedPool(t, "missing")
	defer cleanup()
	ctx := context.Background()
	dir := setupTo000030(t, pool)

	u := seedMigrationUser(t, ctx, pool, "active")
	c := seedMigrationContent(t, ctx, pool, u, "public")
	missingFPS := uuid.New()

	// TEST-ONLY: drop old constraints to emulate corrupt legacy state
	disableLegacyConsistencyCheck(t, ctx, pool)
	_, err := pool.Exec(ctx, `ALTER TABLE comments DROP CONSTRAINT IF EXISTS comments_for_sale_id_fkey`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO comments(id,author_id,type,for_sale_id,target_id,target_type,created_at,updated_at) VALUES($1,$2,'listing_reference',$3,$4,'content',NOW(),NOW())`, uuid.New(), u, missingFPS, c)
	require.NoError(t, err)

	err = testOnlyRunTo(ctx, pool, dir, 31)
	require.Error(t, err, "missing source must abort 000031")

	var rec bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=31)`).Scan(&rec))
	require.False(t, rec)
	var tc bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='comments' AND column_name='type')`).Scan(&tc))
	require.True(t, tc)
}
