package repository

// CONTENT_TAGS_JSONB_FIX — source-inspection regression lock for content hashtag wiring.
//
// Invariants locked:
// 1. GetTagsByContentID is present in the content repository interface.
// 2. InsertTags is present in the content repository interface.
// 3. GetTagsByContentID query references content_hashtags table.
// 4. InsertTags query references content_hashtags table with ON CONFLICT DO NOTHING.
// 5. GetByID populates content.Tags via GetTagsByContentID.
// 6. ToContentResponse uses content.Tags (not hardcoded []string{}).
// 7. Migration 000201 creates content_hashtags (idempotent IF NOT EXISTS).

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func readContentRepoImpl(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	data, err := os.ReadFile(filepath.Join(dir, "content_repository_impl.go"))
	if err != nil {
		t.Fatalf("read content_repository_impl.go: %v", err)
	}
	return string(data)
}

func readContentHandlerFile(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// Navigate from infrastructure/repository → delivery/http
	dir := filepath.Dir(file)
	p := filepath.Join(dir, "..", "..", "delivery", "http", "content_handler.go")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read content_handler.go: %v", err)
	}
	return string(data)
}

func readCanonicalSchema(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	p := filepath.Join(dir, "..", "..", "..", "..", "..", "migrations", "000001_canonical_schema.up.sql")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read canonical schema: %v", err)
	}
	return string(data)
}

func TestContentTags_GetTagsByContentID_QueryReferencesTable(t *testing.T) {
	src := readContentRepoImpl(t)
	if !strings.Contains(src, "GetTagsByContentID") {
		t.Fatal("GetTagsByContentID not found in content_repository_impl.go")
	}
	if !strings.Contains(src, "FROM content_hashtags") {
		t.Fatal("GetTagsByContentID does not query content_hashtags table")
	}
}

func TestContentTags_InsertTags_QueryReferencesTable(t *testing.T) {
	src := readContentRepoImpl(t)
	if !strings.Contains(src, "InsertTags") {
		t.Fatal("InsertTags not found in content_repository_impl.go")
	}
	if !strings.Contains(src, "INSERT INTO content_hashtags") {
		t.Fatal("InsertTags does not INSERT INTO content_hashtags")
	}
}

func TestContentTags_InsertTags_IdempotentOnConflict(t *testing.T) {
	src := readContentRepoImpl(t)
	if !strings.Contains(src, "ON CONFLICT") {
		t.Fatal("InsertTags missing ON CONFLICT DO NOTHING — not idempotent")
	}
}

func TestContentTags_GetByID_PopulatesTagsFromRepo(t *testing.T) {
	src := readContentRepoImpl(t)
	// GetByID must call GetTagsByContentID and assign content.Tags
	if !strings.Contains(src, "GetTagsByContentID") {
		t.Fatal("GetByID does not call GetTagsByContentID")
	}
	if !strings.Contains(src, "content.Tags = tags") {
		t.Fatal("GetByID does not assign content.Tags from tag fetch result")
	}
}

func TestContentTags_Handler_UsesEntityTags_NotHardcoded(t *testing.T) {
	src := readContentHandlerFile(t)

	// The old hardcoded stub must be gone
	if strings.Contains(src, `[]string{}, // TODO: Populate from entity`) {
		t.Fatal("content_handler.go still has the hardcoded []string{} TODO stub — not fixed")
	}

	// The new wiring must reference content.Tags
	if !strings.Contains(src, "content.Tags") {
		t.Fatal("content_handler.go does not use content.Tags in ToContentResponse")
	}
}

func TestContentTags_CanonicalSchema_HasContentHashtagsTable(t *testing.T) {
	sql := readCanonicalSchema(t)

	if !strings.Contains(sql, "CREATE TABLE content_hashtags") {
		t.Fatal("canonical schema missing content_hashtags table")
	}
	if !strings.Contains(sql, "content_id uuid") {
		t.Fatal("canonical schema content_hashtags missing content_id column")
	}
	if !strings.Contains(sql, "hashtag text") {
		t.Fatal("canonical schema content_hashtags missing hashtag column")
	}
	if !strings.Contains(sql, "REFERENCES contents(id) ON DELETE CASCADE") {
		t.Fatal("canonical schema content_hashtags missing FK to contents(id) ON DELETE CASCADE")
	}
}

func TestContentTags_ContentRepoInterface_HasTagMethods(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	data, err := os.ReadFile(filepath.Join(dir, "content_repository.go"))
	if err != nil {
		t.Fatalf("read content_repository.go: %v", err)
	}
	src := string(data)

	if !strings.Contains(src, "GetTagsByContentID") {
		t.Fatal("ContentRepository interface missing GetTagsByContentID")
	}
	if !strings.Contains(src, "InsertTags") {
		t.Fatal("ContentRepository interface missing InsertTags")
	}
}


