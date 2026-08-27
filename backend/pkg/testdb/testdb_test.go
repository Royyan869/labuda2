// Package testdb tests verify that test database isolation works correctly.
package testdb

import (
	"testing"

	"github.com/labuda/backend/internal/config"
)

// TestDatabaseIsolation verifies that test database is isolated from main database.
//
// This test ensures:
// 1. Test database name is different from main database name
// 2. Test DSN connects to test database, not main database
// 3. GetTestDSN() returns valid connection string
func TestDatabaseIsolation(t *testing.T) {
	// go test sets the working directory to this package's directory, not
	// backend/, so config.Load()'s godotenv.Load() can't find backend/.env
	// from here. loadDotEnvFromParents (already used by SetupDB, see
	// testdb.go) walks up to find it, same as every other caller in this
	// codebase relies on.
	loadDotEnvFromParents(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// SAFETY CHECK: Test DB must be different from main DB
	if cfg.Database.TestName == cfg.Database.Name {
		t.Fatalf("TEST DB SAFETY FAIL: Test database name '%s' is the same as main database '%s'",
			cfg.Database.TestName, cfg.Database.Name)
	}

	t.Logf("Main database: %s", cfg.Database.Name)
	t.Logf("Test database: %s", cfg.Database.TestName)

	// Verify GetTestDSN returns non-empty string
	testDSN := cfg.Database.GetTestDSN()
	if testDSN == "" {
		t.Fatal("GetTestDSN() returned empty string")
	}

	// Verify GetTestDatabaseURL returns non-empty string
	testURL := cfg.Database.GetTestDatabaseURL()
	if testURL == "" {
		t.Fatal("GetTestDatabaseURL() returned empty string")
	}

	t.Logf("Test DSN generated successfully (password redacted in logs)")

	// Verify test database name is set
	if cfg.Database.TestName == "" {
		t.Fatal("DB_TEST_NAME is not set - test database name is empty")
	}
}

// TestMainDatabaseNotAffected verifies tests cannot accidentally use main database.
//
// This is a compile-time check that the testdb package provides proper isolation.
func TestMainDatabaseNotAffected(t *testing.T) {
	t.Run("GetDSN vs GetTestDSN", func(t *testing.T) {
		loadDotEnvFromParents(t)

		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		mainDSN := cfg.Database.GetDSN()
		testDSN := cfg.Database.GetTestDSN()

		t.Logf("Main DB DSN: %s", mainDSN)
		t.Logf("Test DB DSN: %s", testDSN)

		// The DSNs should be different (they point to different databases)
		if mainDSN == testDSN {
			t.Fatal("Main DSN and Test DSN are identical - this indicates a configuration error")
		}

		// Test DSN should contain test database's dbname parameter
		// Use word boundary matching to avoid false positives (e.g., "labuda" in "labuda_test")
		testDBPattern := "dbname=" + cfg.Database.TestName
		if !containsWord(testDSN, testDBPattern) {
			t.Errorf("Test DSN does not contain test database pattern '%s'", testDBPattern)
		}

		// Main DSN should contain main database's dbname parameter
		mainDBPattern := "dbname=" + cfg.Database.Name
		if !containsWord(mainDSN, mainDBPattern) {
			t.Errorf("Main DSN does not contain main database pattern '%s'", mainDBPattern)
		}

		t.Logf("✓ Isolation verified: main DB uses '%s', test DB uses '%s'",
			cfg.Database.Name, cfg.Database.TestName)
	})
}

// containsWord checks if s contains substr as a whole word (not as substring of another word)
// For DSN, this means checking the pattern is followed by space or end of string
func containsWord(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			// Check if pattern ends at space, end of string, or another delimiter
			nextIdx := i + len(substr)
			if nextIdx >= len(s) {
				return true // Pattern is at end of string
			}
			nextChar := s[nextIdx]
			if nextChar == ' ' || nextChar == '?' || nextChar == '&' || nextChar == ';' {
				return true // Pattern is followed by delimiter
			}
		}
	}
	return false
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
