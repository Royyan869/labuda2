package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Split tests (moved from cmd/migrate/main_test.go) ---

func TestSplit_EscapedQuote(t *testing.T) {
	sql := `COMMENT ON COLUMN foo.bar IS 'It''s a label';`
	stmts := Split(sql)
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d: %v", len(stmts), stmts)
	}
	stmt := stmts[0]
	if !strings.Contains(stmt, "It''s") {
		t.Errorf("escaped quote '' not preserved in statement: %q", stmt)
	}
}

func TestSplit_EscapedQuoteMultiple(t *testing.T) {
	sql := `INSERT INTO t (a) VALUES ('it''s a ''test''');`
	stmts := Split(sql)
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	stmt := stmts[0]
	if !strings.Contains(stmt, "it''s") {
		t.Errorf("first '' not preserved: %q", stmt)
	}
	if !strings.Contains(stmt, "''test''") {
		t.Errorf("second '' not preserved: %q", stmt)
	}
}

func TestSplit_DollarQuote(t *testing.T) {
	sql := `DO $$ BEGIN RAISE NOTICE 'hello'; END $$;`
	stmts := Split(sql)
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], "RAISE NOTICE") {
		t.Errorf("dollar-quoted body not preserved: %q", stmts[0])
	}
}

func TestSplit_MultiStatement(t *testing.T) {
	sql := "ALTER TABLE a ADD COLUMN x INT;\nALTER TABLE b ADD COLUMN y TEXT;\n"
	stmts := Split(sql)
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d: %v", len(stmts), stmts)
	}
}

func TestSplit_CommentStripped(t *testing.T) {
	sql := "-- This is a comment\nSELECT 1;"
	stmts := Split(sql)
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d: %v", len(stmts), stmts)
	}
	if strings.Contains(stmts[0], "comment") {
		t.Errorf("comment was not stripped from statement: %q", stmts[0])
	}
}

func TestSplit_EnumAddValueWithIndex(t *testing.T) {
	sql := `ALTER TYPE payment_webhook_status_enum ADD VALUE IF NOT EXISTS 'manual_review';
CREATE INDEX IF NOT EXISTS idx_foo ON payment_webhook_events USING btree (status) WHERE (status = 'manual_review'::payment_webhook_status_enum);`
	stmts := Split(sql)
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

// --- New split tests ---

func TestSplit_DoBlockWithBegin(t *testing.T) {
	// Regression: a DO $$ block containing its own BEGIN/END must stay as
	// one statement — the semicolons inside $$ are NOT statement terminators.
	sql := `DO $$
DECLARE
    duplicate_count integer;
BEGIN
    SELECT COUNT(*) INTO duplicate_count FROM users;
    IF duplicate_count > 0 THEN
        RAISE EXCEPTION 'duplicates exist';
    END IF;
END
$$;
CREATE UNIQUE INDEX idx_foo ON users (email);`
	stmts := Split(sql)
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements (DO block + CREATE INDEX), got %d: %v", len(stmts), stmts)
	}
	if !strings.Contains(stmts[0], "RAISE EXCEPTION") {
		t.Errorf("DO block body not preserved in first statement: %q", stmts[0])
	}
	if !strings.Contains(stmts[1], "CREATE UNIQUE INDEX") {
		t.Errorf("CREATE INDEX should be second statement, got: %q", stmts[1])
	}
}

func TestSplit_CommentBeforeStatement(t *testing.T) {
	sql := "-- comment line\n/* block comment */\nSELECT 1;"
	stmts := Split(sql)
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if strings.Contains(stmts[0], "comment") {
		t.Errorf("comments should be stripped: %q", stmts[0])
	}
}

func TestSplit_EmptyResult(t *testing.T) {
	sql := "-- only a comment\n/* and a block */\n   \n"
	stmts := Split(sql)
	if len(stmts) != 0 {
		t.Fatalf("expected 0 statements, got %d: %v", len(stmts), stmts)
	}
}

func TestSplit_FunctionWithDollarQuote(t *testing.T) {
	// CREATE FUNCTION with $$ body — the semicolons inside the function
	// must not split the statement.
	sql := `CREATE OR REPLACE FUNCTION enforce_rule()
RETURNS trigger AS $$
BEGIN
    IF NEW.status = 'active' THEN
        RAISE EXCEPTION 'cannot activate';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER trg_enforce BEFORE INSERT ON tbl FOR EACH ROW EXECUTE FUNCTION enforce_rule();`
	stmts := Split(sql)
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements (function + trigger), got %d: %v", len(stmts), stmts)
	}
	if !strings.Contains(stmts[0], "RETURNS trigger") {
		t.Errorf("first statement should be CREATE FUNCTION, got: %q", stmts[0])
	}
	if !strings.Contains(stmts[1], "CREATE TRIGGER") {
		t.Errorf("second statement should be CREATE TRIGGER, got: %q", stmts[1])
	}
}

// --- LoadMigrations tests ---

func TestLoadMigrations_IgnoresSubfolders(t *testing.T) {
	tempRoot, err := os.MkdirTemp("", "mig-load-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tempRoot) })

	migrationsDir := filepath.Join(tempRoot, "migrations")
	if err := os.Mkdir(migrationsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	write := func(rel string) {
		full := filepath.Join(tempRoot, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("SELECT 1;"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("migrations/000100_alpha.up.sql")
	write("migrations/000101_bravo.up.sql")
	write("migrations/legacy_do_not_run/000_init/000102_wrong.up.sql")
	write("migrations/legacy_do_not_run/archive/000103_wrong.up.sql")

	migrations, err := LoadMigrations(migrationsDir)
	if err != nil {
		t.Fatalf("LoadMigrations returned error: %v", err)
	}

	if len(migrations) != 2 {
		t.Fatalf("expected 2 root migrations, got %d: %#v", len(migrations), migrations)
	}
	if migrations[0].Version != 100 || migrations[1].Version != 101 {
		t.Fatalf("unexpected migration versions: %#v", migrations)
	}
}

func TestLoadMigrations_SortedOrder(t *testing.T) {
	tempRoot, err := os.MkdirTemp("", "mig-sort-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tempRoot) })

	dir := filepath.Join(tempRoot, "migrations")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write out of numeric order
	files := []string{
		"000005_fifth.up.sql",
		"000001_first.up.sql",
		"000010_tenth.up.sql",
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("SELECT 1;"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	migrations, err := LoadMigrations(dir)
	if err != nil {
		t.Fatalf("LoadMigrations: %v", err)
	}
	if len(migrations) != 3 {
		t.Fatalf("expected 3 migrations, got %d", len(migrations))
	}
	if migrations[0].Version != 1 || migrations[1].Version != 5 || migrations[2].Version != 10 {
		t.Fatalf("expected sorted order 1,5,10 got %d,%d,%d",
			migrations[0].Version, migrations[1].Version, migrations[2].Version)
	}
}

func TestLoadMigrations_NonexistentDir(t *testing.T) {
	_, err := LoadMigrations("/nonexistent/path/to/migrations")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestLoadMigrations_UnparseableVersion(t *testing.T) {
	tempRoot, err := os.MkdirTemp("", "mig-badver-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tempRoot) })

	dir := filepath.Join(tempRoot, "migrations")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "bad_version.up.sql"), []byte("SELECT 1;"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = LoadMigrations(dir)
	if err == nil {
		t.Fatal("expected error for unparseable version")
	}
}

// --- isStandaloneTxControl tests ---

func TestIsStandaloneTxControl(t *testing.T) {
	cases := []struct {
		stmt     string
		expected bool
	}{
		{"BEGIN;", true},
		{"begin;", true},
		{"  BEGIN;  ", true},
		{"COMMIT;", true},
		{"commit;", true},
		{"  COMMIT;  ", true},
		{"COMMIT", false},       // no semicolon — not a standalone statement as split
		{"SELECT 1;", false},
		{"BEGIN", false},        // no semicolon
		{"BEGIN WORK;", false},  // not standalone
		{"", false},
	}
	for _, tc := range cases {
		got := isStandaloneTxControl(tc.stmt)
		if got != tc.expected {
			t.Errorf("isStandaloneTxControl(%q) = %v, want %v", tc.stmt, got, tc.expected)
		}
	}
}
