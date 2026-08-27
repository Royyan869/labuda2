// Package migration provides the canonical PostgreSQL migration executor.
//
// It is the single authority for applying numbered *.up.sql migration files.
// Both the production CLI (cmd/migrate) and the test infrastructure (pkg/testdb)
// use this package so that every path through the codebase executes migrations
// with identical semantics.
//
// The SQL statement splitter handles:
//   - $$ dollar-quoted strings (PL/pgSQL function bodies, DO blocks)
//   - -- single-line comments
//   - /* */ multi-line comments
//   - '' escaped quotes inside string literals
//   - ; statement terminators
//
// Migration tracking uses the production-canonical public.schema_migrations table:
//
//	CREATE TABLE IF NOT EXISTS public.schema_migrations (
//	    version    INTEGER PRIMARY KEY,
//	    name       TEXT NOT NULL,
//	    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
//	);
//
// This package does NOT use golang-migrate. It uses pgx directly.
package migration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Migration is one numbered .up.sql file.
type Migration struct {
	Version int
	Name    string
	UpSQL   string
}

// Split splits raw SQL into individual executable statements.
//
// It correctly handles:
//   - $$ dollar-quoted strings (including PL/pgSQL function bodies with
//     embedded BEGIN/END and semicolons)
//   - -- single-line comments (stripped)
//   - /* */ multi-line comments (stripped)
//   - '' escaped quotes inside string literals (doubled single-quote preserved)
//   - ; statement terminators
//
// This is the canonical SQL splitter, extracted from cmd/migrate/main.go.
func Split(sql string) []string {
	var statements []string
	var current strings.Builder

	runes := []rune(sql)
	n := len(runes)

	i := 0
	for i < n {
		r := runes[i]

		// Single-line comment --
		if r == '-' && i+1 < n && runes[i+1] == '-' {
			// Skip until newline
			for i < n && runes[i] != '\n' {
				i++
			}
			continue
		}

		// Multi-line comment /* */
		if r == '/' && i+1 < n && runes[i+1] == '*' {
			i += 2
			for i < n-1 && !(runes[i] == '*' && runes[i+1] == '/') {
				i++
			}
			i += 2
			continue
		}

		// String literal
		if r == '\'' {
			current.WriteRune(r)
			i++
			for i < n {
				current.WriteRune(runes[i])
				if runes[i] == '\'' {
					// Check for escaped quote ''
					if i+1 < n && runes[i+1] == '\'' {
						current.WriteRune(runes[i+1]) // write second ' of ''
						i += 2
						continue
					}
					break
				}
				i++
			}
			i++
			continue
		}

		// Dollar-quoted string $$
		if r == '$' && i+1 < n && runes[i+1] == '$' {
			// Find the closing $$
			current.WriteString("$$")
			i += 2
			for i < n-1 && !(runes[i] == '$' && runes[i+1] == '$') {
				current.WriteRune(runes[i])
				i++
			}
			if i < n-1 {
				current.WriteString("$$")
				i += 2
			}
			continue
		}

		// Statement terminator (semicolon)
		if r == ';' {
			current.WriteRune(r)
			stmt := current.String()
			stmt = cleanupStatement(stmt)
			stmt = strings.TrimSpace(stmt)
			if stmt != "" {
				statements = append(statements, stmt)
			}
			current.Reset()
			i++
			continue
		}

		current.WriteRune(r)
		i++
	}

	// Add remaining content
	if stmt := strings.TrimSpace(current.String()); stmt != "" {
		stmt = cleanupStatement(stmt)
		if strings.TrimSpace(stmt) != "" {
			statements = append(statements, strings.TrimSpace(stmt))
		}
	}

	return statements
}

// cleanupStatement removes trailing commas before closing parentheses
// and other common SQL cleanup issues.
func cleanupStatement(stmt string) string {
	// Remove trailing commas before closing parentheses
	// Pattern: comma + whitespace + closing paren + optional whitespace + semicolon
	// This handles: "column TYPE, );" -> "column TYPE);"

	// First, handle ",);" -> ");"
	re1 := regexp.MustCompile(`,\s*\);`)
	stmt = re1.ReplaceAllString(stmt, ");")

	// Then handle ",\n)" -> "\n)" (comma before newline before paren)
	re2 := regexp.MustCompile(`,\s*\n\s*\)`)
	stmt = re2.ReplaceAllString(stmt, "\n)")

	// Handle ", )" at end of statement
	re3 := regexp.MustCompile(`,(\s*\))\s*;`)
	stmt = re3.ReplaceAllString(stmt, "$1;")

	// Clean up multiple empty lines
	re4 := regexp.MustCompile(`\n\s*\n\s*\n+`)
	stmt = re4.ReplaceAllString(stmt, "\n")

	return stmt
}

// CurrentVersion returns the highest applied migration version.
// Returns 0 when no migrations have been applied.
func CurrentVersion(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	var version int
	err := pool.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0) FROM public.schema_migrations`).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("query max migration version: %w", err)
	}
	return version, nil
}

// ErrLegacyMigrationTable is returned when a golang-migrate-format
// schema_migrations table is detected on a non-empty database.
var ErrLegacyMigrationTable = fmt.Errorf("unexpected legacy schema_migrations table (golang-migrate format) — this database may have been migrated by an incompatible tool; recreate the database or manually drop the legacy schema_migrations table and re-run")

// EnsureSchemaMigrationsTable creates the production-canonical
// schema_migrations table if it does not exist. If a canonical table
// already exists this is a no-op.
//
// Safety contract:
//   - Empty database (no table) → CREATE canonical table.
//   - Canonical table present → no-op.
//   - Unexpected table shape (legacy golang-migrate format without a
//     "name" column) → fail closed with ErrLegacyMigrationTable.
//     No schema mutation, no migration execution.
//
// It NEVER drops or alters an existing table. Destructive recovery is
// the caller's responsibility (e.g. pkg/testdb can drop the table before
// calling Run because it owns the disposable database).
func EnsureSchemaMigrationsTable(ctx context.Context, pool *pgxpool.Pool) error {
	var tableExists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_tables
			WHERE schemaname = 'public' AND tablename = 'schema_migrations'
		)
	`).Scan(&tableExists)
	if err != nil {
		return fmt.Errorf("check schema_migrations table existence: %w", err)
	}

	if tableExists {
		var hasNameColumn bool
		err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_schema = 'public'
				  AND table_name = 'schema_migrations'
				  AND column_name = 'name'
			)
		`).Scan(&hasNameColumn)
		if err != nil {
			return fmt.Errorf("check schema_migrations columns: %w", err)
		}

		if hasNameColumn {
			// Canonical table already present — no-op.
			return nil
		}

		// Legacy golang-migrate format — fail closed. No DROP, no mutation.
		return ErrLegacyMigrationTable
	}

	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS public.schema_migrations (
			version    INTEGER PRIMARY KEY,
			name       TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	return nil
}

// isStandaloneTxControl returns true when stmt is exactly "BEGIN;" or
// "COMMIT;" (case-insensitive, after trimming). Migrations 000019 and
// 000026 wrap their content in explicit BEGIN/COMMIT, which would
// prematurely commit the outer transaction that Run creates.
func isStandaloneTxControl(stmt string) bool {
	s := strings.ToUpper(strings.TrimSpace(stmt))
	return s == "BEGIN;" || s == "COMMIT;"
}

// LoadMigrations reads all top-level *.up.sql files from dir, parses the
// leading version number, and returns them sorted in ascending version
// order. Subdirectories are never recursed into.
func LoadMigrations(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations directory %q: %w", dir, err)
	}

	var migrationFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".up.sql") {
			migrationFiles = append(migrationFiles, filepath.Join(dir, name))
		}
	}

	sort.Strings(migrationFiles)

	var migrations []Migration
	for _, filePath := range migrationFiles {
		filename := filepath.Base(filePath)

		versionStr := strings.Split(filename, "_")[0]
		var version int
		if _, err := fmt.Sscanf(versionStr, "%d", &version); err != nil {
			return nil, fmt.Errorf("parse version from %q: %w", filename, err)
		}

		name := strings.TrimPrefix(filename, versionStr+"_")
		name = strings.TrimSuffix(name, ".up.sql")

		upSQL, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("read migration file %q: %w", filePath, err)
		}

		migrations = append(migrations, Migration{
			Version: version,
			Name:    name,
			UpSQL:   string(upSQL),
		})
	}

	return migrations, nil
}

// Run executes all pending migrations in version order against pool.
//
// Migrations whose version already appears in schema_migrations are
// skipped. Each new migration is split into individual statements via
// Split and executed either transactionally (the default) or
// non-transactionally when the file contains ADD VALUE (PostgreSQL
// forbids using a newly-added enum value in the same transaction).
//
// Standalone BEGIN;/COMMIT; statements embedded in migration files
// (000019, 000026) are filtered out — Run manages its own transaction
// boundaries.
//
// Run is idempotent: calling it against an already-migrated database is a
// no-op.
func Run(ctx context.Context, pool *pgxpool.Pool, migrationsDir string) error {
	if err := EnsureSchemaMigrationsTable(ctx, pool); err != nil {
		return err
	}

	// Required by gen_random_uuid() references in 000001 and later
	// migrations. The migration files themselves create uuid-ossp and
	// btree_gist, but pgcrypto is only guaranteed by the runner preamble.
	if _, err := pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS pgcrypto"); err != nil {
		return fmt.Errorf("enable pgcrypto extension: %w", err)
	}

	migrations, err := LoadMigrations(migrationsDir)
	if err != nil {
		return err
	}

	for _, m := range migrations {
		var exists bool
		err := pool.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM public.schema_migrations WHERE version = $1)",
			m.Version,
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check migration %d: %w", m.Version, err)
		}
		if exists {
			continue
		}

		statements := Split(m.UpSQL)

		// Filter standalone BEGIN;/COMMIT; — the runner manages its own
		// transaction boundaries (see isStandaloneTxControl godoc).
		filtered := statements[:0]
		for _, stmt := range statements {
			if strings.TrimSpace(stmt) == "" || isStandaloneTxControl(stmt) {
				continue
			}
			filtered = append(filtered, stmt)
		}

		hasAddValue := strings.Contains(strings.ToUpper(m.UpSQL), "ADD VALUE")

		if hasAddValue {
			// Non-transactional: each statement executes independently so
			// that ADD VALUE commits before any dependent CREATE INDEX.
			for i, stmt := range filtered {
				if _, err := pool.Exec(ctx, stmt); err != nil {
					return fmt.Errorf("migration %d_%s statement %d (no-tx): %w\nSQL: %s",
						m.Version, m.Name, i+1, err, stmt)
				}
			}
			if _, err := pool.Exec(ctx,
				"INSERT INTO public.schema_migrations (version, name) VALUES ($1, $2)",
				m.Version, m.Name,
			); err != nil {
				return fmt.Errorf("record migration %d_%s: %w", m.Version, m.Name, err)
			}
		} else {
			tx, err := pool.Begin(ctx)
			if err != nil {
				return fmt.Errorf("begin tx for migration %d_%s: %w", m.Version, m.Name, err)
			}

			ok := false
			defer func() {
				if !ok {
					tx.Rollback(ctx)
				}
			}()

			for i, stmt := range filtered {
				if _, err := tx.Exec(ctx, stmt); err != nil {
					return fmt.Errorf("migration %d_%s statement %d: %w\nSQL: %s",
						m.Version, m.Name, i+1, err, stmt)
				}
			}

			if _, err := tx.Exec(ctx,
				"INSERT INTO public.schema_migrations (version, name) VALUES ($1, $2)",
				m.Version, m.Name,
			); err != nil {
				return fmt.Errorf("record migration %d_%s: %w", m.Version, m.Name, err)
			}

			if err := tx.Commit(ctx); err != nil {
				return fmt.Errorf("commit migration %d_%s: %w", m.Version, m.Name, err)
			}
			ok = true
		}
	}

	return nil
}
