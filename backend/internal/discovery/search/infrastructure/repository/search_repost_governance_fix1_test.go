package repository

// FIX-1 — Search repost governance regression lock.
//
// Verifies that both the base query and the count query in SearchContent
// contain the NOT (... AND EXISTS ...) clause that excludes content-type
// reposts whose original content is hidden, deleted, or non-active.
//
// Uses source-file inspection (same pattern as feed_viewercontext_w3a_test.go)
// because the SQL strings are local variables built inside SearchContent and
// cannot be accessed from outside the function without extracting them.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestSearchContent_RepostGovernanceClauseInSQL verifies that the
// FIX-1 repost governance gate is present in both the base query and the
// count query. Each token must appear at least twice — once per query.
// If either token is missing, the governance clause has been removed.
func TestSearchContent_RepostGovernanceClauseInSQL(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	implPath := filepath.Join(filepath.Dir(thisFile), "search_repository_impl.go")
	b, err := os.ReadFile(implPath)
	if err != nil {
		t.Fatalf("cannot read search_repository_impl.go: %v", err)
	}
	src := string(b)

	// Each of these tokens must appear in BOTH the base query and the count
	// query, i.e. at least twice. A single occurrence means the governance
	// clause was added only to one of the two queries.
	tokens := []struct {
		description string
		token       string
	}{
		{
			"non-repost short-circuit (original_author_id IS NOT NULL)",
			"c.original_author_id IS NOT NULL",
		},
		{
			"content-source guard (occ.content_source_id IS NOT NULL)",
			"occ.content_source_id IS NOT NULL",
		},
		{
			"correlated sub-select on originals table",
			"SELECT 1 FROM contents orig",
		},
		{
			"target content status predicate on original",
			"orig.status != 'active'",
		},
	}

	for _, tt := range tokens {
		count := strings.Count(src, tt.token)
		if count < 2 {
			t.Errorf(
				"repost governance token appears %d time(s) in search_repository_impl.go, want >=2 (base+count query): %s\n  token: %q",
				count, tt.description, tt.token,
			)
		}
	}
}


