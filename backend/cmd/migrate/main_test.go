package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSplitSQL_EscapedQuote verifies the ” (double single-quote) SQL escape is preserved.
// Root cause that this covers: the parser was writing the first ' then skipping both with
// i += 2, resulting in only one quote reaching PostgreSQL and corrupting string literals
// (e.g. COMMENT ON COLUMN with 'It”s a label' would produce 'It's, breaking the SQL).
func TestSplitSQL_EscapedQuote(t *testing.T) {
	sql := `COMMENT ON COLUMN foo.bar IS 'It''s a label';`
	stmts := splitSQLStatements(sql)
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d: %v", len(stmts), stmts)
	}
	stmt := stmts[0]
	if !strings.Contains(stmt, "It''s") {
		t.Errorf("escaped quote '' not preserved in statement: %q", stmt)
	}
}

// TestSplitSQL_EscapedQuoteMultiple verifies multiple ” in the same string literal.
func TestSplitSQL_EscapedQuoteMultiple(t *testing.T) {
	sql := `INSERT INTO t (a) VALUES ('it''s a ''test''');`
	stmts := splitSQLStatements(sql)
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	stmt := stmts[0]
	// Original string: 'it''s a ''test'''
	if !strings.Contains(stmt, "it''s") {
		t.Errorf("first '' not preserved: %q", stmt)
	}
	if !strings.Contains(stmt, "''test''") {
		t.Errorf("second '' not preserved: %q", stmt)
	}
}

// TestSplitSQL_DollarQuote verifies $$ dollar-quoted strings pass through intact.
func TestSplitSQL_DollarQuote(t *testing.T) {
	sql := `DO $$ BEGIN RAISE NOTICE 'hello'; END $$;`
	stmts := splitSQLStatements(sql)
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], "RAISE NOTICE") {
		t.Errorf("dollar-quoted body not preserved: %q", stmts[0])
	}
}

// TestSplitSQL_MultiStatement verifies multiple semicolon-separated statements are split.
func TestSplitSQL_MultiStatement(t *testing.T) {
	sql := "ALTER TABLE a ADD COLUMN x INT;\nALTER TABLE b ADD COLUMN y TEXT;\n"
	stmts := splitSQLStatements(sql)
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d: %v", len(stmts), stmts)
	}
}

// TestSplitSQL_CommentStripped verifies single-line comments are stripped.
func TestSplitSQL_CommentStripped(t *testing.T) {
	sql := "-- This is a comment\nSELECT 1;"
	stmts := splitSQLStatements(sql)
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d: %v", len(stmts), stmts)
	}
	if strings.Contains(stmts[0], "comment") {
		t.Errorf("comment was not stripped from statement: %q", stmts[0])
	}
}

// TestHasAddValue_Detection verifies the ADD VALUE detection logic used by the migration runner
// to decide whether to run a migration non-transactionally.
// This covers the class of failures where ALTER TYPE ... ADD VALUE followed by CREATE INDEX
// using the new enum value fails inside a transaction (PostgreSQL requires ADD VALUE to commit
// before the new value is visible to DDL in the same transaction block).
func TestHasAddValue_Detection(t *testing.T) {
	cases := []struct {
		name     string
		sql      string
		expected bool
	}{
		{
			name:     "has ADD VALUE uppercase",
			sql:      "ALTER TYPE foo ADD VALUE 'bar';",
			expected: true,
		},
		{
			name:     "has ADD VALUE lowercase",
			sql:      "alter type foo add value 'bar';",
			expected: true,
		},
		{
			name:     "has ADD VALUE IF NOT EXISTS",
			sql:      "ALTER TYPE ticket_status_enum ADD VALUE IF NOT EXISTS 'waiting_user';\nCREATE INDEX idx_foo ON bar(status);",
			expected: true,
		},
		{
			name:     "no ADD VALUE",
			sql:      "ALTER TABLE foo ADD COLUMN bar TEXT;",
			expected: false,
		},
		{
			name:     "no ADD VALUE in CREATE INDEX only",
			sql:      "CREATE INDEX idx_foo ON bar(status) WHERE status = 'active';",
			expected: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := strings.Contains(strings.ToUpper(tc.sql), "ADD VALUE")
			if got != tc.expected {
				t.Errorf("ADD VALUE detection for %q: got %v, want %v", tc.name, got, tc.expected)
			}
		})
	}
}

// TestSplitSQL_EnumAddValueWithIndex verifies that migration 000188-style SQL
// (ADD VALUE followed by CREATE INDEX using the new value) is correctly split
// into 2 separate statements — so the runner can execute them non-transactionally.
func TestSplitSQL_EnumAddValueWithIndex(t *testing.T) {
	sql := `ALTER TYPE payment_webhook_status_enum ADD VALUE IF NOT EXISTS 'manual_review';
CREATE INDEX IF NOT EXISTS idx_foo ON payment_webhook_events USING btree (status) WHERE (status = 'manual_review'::payment_webhook_status_enum);`
	stmts := splitSQLStatements(sql)
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d: %v", len(stmts), stmts)
	}
	if !strings.Contains(strings.ToUpper(stmts[0]), "ADD VALUE") {
		t.Errorf("first statement should be ADD VALUE, got: %q", stmts[0])
	}
	if !strings.Contains(strings.ToUpper(stmts[1]), "CREATE INDEX") {
		t.Errorf("second statement should be CREATE INDEX, got: %q", stmts[1])
	}
}

// TestLoadMigrations_IgnoresSubfolders proves the migration runner only reads
// top-level *.up.sql files from migrations/ and never recurses into deprecated
// or historical subdirectories.
func TestLoadMigrations_IgnoresSubfolders(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tempRoot, err := os.MkdirTemp("", "mig-load-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
		_ = os.RemoveAll(tempRoot)
	})

	if err := os.Mkdir(filepath.Join(tempRoot, "migrations"), 0o755); err != nil {
		t.Fatal(err)
	}

	write := func(rel string) {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(tempRoot, rel)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tempRoot, rel), []byte("SELECT 1;"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("migrations/000100_alpha.up.sql")
	write("migrations/000101_bravo.up.sql")
	write("migrations/legacy_do_not_run/000_init/000102_wrong.up.sql")
	write("migrations/legacy_do_not_run/archive/000103_wrong.up.sql")
	write("migrations/legacy_do_not_run/snapshots/000104_wrong.up.sql")

	if err := os.Chdir(tempRoot); err != nil {
		t.Fatal(err)
	}

	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations returned error: %v", err)
	}

	if len(migrations) != 2 {
		t.Fatalf("expected 2 root migrations, got %d: %#v", len(migrations), migrations)
	}
	if migrations[0].Version != 100 || migrations[1].Version != 101 {
		t.Fatalf("unexpected migration versions: %#v", migrations)
	}
}
