package migration

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// writeChain materializes a migration chain directory containing the canonical
// baseline file, so ResolveDir can recognise it.
func writeChain(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	baseline := filepath.Join(dir, canonicalBaselineFile)
	if err := os.WriteFile(baseline, []byte("SELECT 1;\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", baseline, err)
	}
}

func TestResolveDir_WalksUpFromNestedDir(t *testing.T) {
	root := t.TempDir()
	chain := filepath.Join(root, DefaultDir)
	writeChain(t, chain)

	nested := filepath.Join(root, "cmd", "core_server")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	got, err := ResolveDir(nested)
	if err != nil {
		t.Fatalf("ResolveDir(%s): %v", nested, err)
	}
	if got != chain {
		t.Fatalf("ResolveDir(%s) = %s, want %s", nested, got, chain)
	}
}

func TestResolveDir_AcceptsChainDirItself(t *testing.T) {
	root := t.TempDir()
	chain := filepath.Join(root, DefaultDir)
	writeChain(t, chain)

	got, err := ResolveDir(chain)
	if err != nil {
		t.Fatalf("ResolveDir(%s): %v", chain, err)
	}
	if got != chain {
		t.Fatalf("ResolveDir(%s) = %s, want %s", chain, got, chain)
	}
}

// TestResolveDir_FindsBackendChainFromRepoRoot covers the layout where the
// process starts at the repository root and the chain lives in backend/.
func TestResolveDir_FindsBackendChainFromRepoRoot(t *testing.T) {
	root := t.TempDir()
	chain := filepath.Join(root, "backend", DefaultDir)
	writeChain(t, chain)

	got, err := ResolveDir(root)
	if err != nil {
		t.Fatalf("ResolveDir(%s): %v", root, err)
	}
	if got != chain {
		t.Fatalf("ResolveDir(%s) = %s, want %s", root, got, chain)
	}
}

func TestResolveDir_RequiresCanonicalBaseline(t *testing.T) {
	root := t.TempDir()
	almost := filepath.Join(root, DefaultDir)
	if err := os.MkdirAll(almost, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(almost, "000002_other.up.sql"), []byte("SELECT 1;\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := ResolveDir(filepath.Join(root, "deeper")); !errors.Is(err, ErrMigrationsDirNotFound) {
		t.Fatalf("ResolveDir without %s = %v, want ErrMigrationsDirNotFound", canonicalBaselineFile, err)
	}
}

func TestResolveDir_NotFound(t *testing.T) {
	root := t.TempDir()
	deeper := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(deeper, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, err := ResolveDir(deeper)
	if !errors.Is(err, ErrMigrationsDirNotFound) {
		t.Fatalf("ResolveDir(%s) = %v, want ErrMigrationsDirNotFound", deeper, err)
	}
}
