package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
			},
			mustNotContain: []string{
				"database.AutoMigrate(",
				"database.SeedDefaultData(",
				"Running database migrations (CORE domains only)",
			},
		},
		{
			path: "../pkg/database/migrate.go",
			mustContain: []string{
				"RunMigrations applies the numbered migration chain using golang-migrate.",
				"core_server does not",
			},
			mustNotContain: []string{
				"func AutoMigrate(",
				"func SeedDefaultData(",
				"PRODUCTION-RECOMMENDED",
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
