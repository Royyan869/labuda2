package tests

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMigrationAuthorityDocsAndRuntimeStayAligned pins the single-authority
// contract of the migration concern:
//
//   - pkg/migration is the only executor (ResolveDir / Split / LoadMigrations /
//     Run / CurrentVersion).
//   - cmd/migrate is a thin CLI over it and owns no SQL parsing or version
//     bookkeeping of its own.
//   - cmd/core_server refuses to boot behind the chain, and never applies
//     migrations itself.
//   - no golang-migrate driver exists anywhere: its schema_migrations shape is
//     incompatible with the canonical table.
//   - the operator-facing docs keep pointing at the single command.
//
// These assertions follow the codebase. They exist so a second migration
// authority cannot be reintroduced silently — not to prescribe how the code is
// written.
func TestMigrationAuthorityDocsAndRuntimeStayAligned(t *testing.T) {
	checks := []struct {
		path           string
		mustContain    []string
		mustNotContain []string
	}{
		{
			path: "../cmd/core_server/main.go",
			mustContain: []string{
				"Database migrations are not applied automatically; run `go run ./cmd/migrate` from backend/ before starting the server",
				"finance_bootstrap_failed: required finance table missing - run `go run ./cmd/migrate` from backend/ before starting the server",
				"migration.ResolveDir(",
				"migration.CurrentVersion(",
				"schemaVersionGate(",
			},
			mustNotContain: []string{
				"database.AutoMigrate(",
				"database.SeedDefaultData(",
				"Running database migrations (CORE domains only)",
				"migration.Run(",
			},
		},
		{
			path: "../cmd/migrate/main.go",
			mustContain: []string{
				"github.com/labuda/backend/pkg/migration",
				"migration.ResolveDir(",
				"migration.Run(",
				"migration.CurrentVersion(",
			},
			mustNotContain: []string{
				"splitSQLStatements",
				"cleanupStatement",
				"func loadMigrations",
				"golang-migrate",
			},
		},
		{
			path: "../pkg/testdb/testdb.go",
			mustContain: []string{
				"migration.ResolveDir(",
				"migration.Run(",
			},
			mustNotContain: []string{
				"../migrations",
				"skipping auto-migrate",
			},
		},
		{
			path: "../README.md",
			mustContain: []string{
				"go run ./cmd/migrate",
				"go run ./cmd/core_server",
				"Run this before starting `core_server`.",
			},
		},
		{
			path: "../migrations/README.md",
			mustContain: []string{
				"The server does not auto-run migrations.",
				"000001_canonical_schema.up.sql",
				"go run ./cmd/migrate",
				"pkg/migration",
			},
			mustNotContain: []string{
				"backend/migrations/000_init/",
				"legacy_do_not_run",
			},
		},
		{
			path: "../.env.example",
			mustContain: []string{
				"RUN_MIGRATIONS_AT_STARTUP=false",
			},
		},
		{
			path: "../cmd/README.md",
			mustContain: []string{
				"Start after `go run ./cmd/migrate`.",
				"Explicit manual migration command.",
			},
		},
	}

	for _, check := range checks {
		t.Run(check.path, func(t *testing.T) {
			t.Helper()

			data := readFile(t, check.path)

			for _, want := range check.mustContain {
				if !strings.Contains(data, want) {
					t.Fatalf("%s is missing required text %q", check.path, want)
				}
			}

			for _, ban := range check.mustNotContain {
				if strings.Contains(data, ban) {
					t.Fatalf("%s still contains forbidden text %q", check.path, ban)
				}
			}
		})
	}

	// The golang-migrate helper is gone for good: resurrecting it would give
	// the database a second, incompatible migration ledger.
	t.Run("golang-migrate helper stays deleted", func(t *testing.T) {
		if _, err := os.Stat("../pkg/database/migrate.go"); err == nil {
			t.Fatal("../pkg/database/migrate.go exists again - migrations must have exactly one executor (pkg/migration)")
		}
	})

	// Stronger than a text check: no Go file may import the golang-migrate
	// driver at all, wherever it hides.
	t.Run("no golang-migrate import anywhere", func(t *testing.T) {
		const needle = "github.com/golang-migrate/migrate"
		for _, root := range []string{"../cmd", "../internal", "../pkg", "../tests"} {
			err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() || !strings.HasSuffix(path, ".go") {
					return nil
				}
				if d.Name() == "migration_authority_guard_test.go" {
					// This file carries the forbidden import path as data, not as
					// an import of its own.
					return nil
				}
				data, readErr := os.ReadFile(path)
				if readErr != nil {
					return readErr
				}
				if strings.Contains(string(data), needle) {
					t.Errorf("%s imports %s - pkg/migration is the only migration executor", path, needle)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("walk %s: %v", root, err)
			}
		}
	})
}

func readFile(t *testing.T, rel string) string {
	t.Helper()

	abs := filepath.Clean(rel)
	data, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}
