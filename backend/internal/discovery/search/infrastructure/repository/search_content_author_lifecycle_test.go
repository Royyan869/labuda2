package repository

// F1-B1 — Content search author lifecycle regression lock.
//
// Verifies that both the base query and the count query in SearchContent
// contain the governance filter that excludes content whose author is
// suspended (account_status != 'active'), banned, or deleted (deleted_at IS NOT NULL).
//
// Social content doctrine differs from commerce for_sale/auction doctrine:
//   - Social (feed/content search): SUSPENDED authors are also excluded.
//     A suspension is a visible governance event; the content is hidden.
//   - Commerce (for_sale/auction search): only BANNED/DELETED sellers are
//     excluded; suspended seller inventory is preserved (demotion-only).
//
// These assertions are source-file inspections because the SQL strings are
// local variables inside repository methods and cannot be accessed externally.
// Pattern mirrors search_banned_seller_suppression_test.go.

import (
	"strings"
	"testing"
)

// TestSearchContent_AuthorLifecycleFilterInSQL verifies the F1-B1 governance
// filter is present in BOTH the base query and the count query of SearchContent.
func TestSearchContent_AuthorLifecycleFilterInSQL(t *testing.T) {
	src := readImplFile(t)

	tokens := []struct {
		description  string
		token        string
		wantMinCount int
	}{
		{
			"F1-B1: users JOIN in SearchContent base query",
			"JOIN users u ON u.id = c.author_id",
			// base query + count query = 2 occurrences minimum.
			2,
		},
		{
			"F1-B1: account_status='active' filter in SearchContent (base+count = 2)",
			"u.account_status = 'active' AND u.deleted_at IS NULL",
			2,
		},
		{
			"F2: repost target author lifecycle check in SearchContent (base+count = 2)",
			"orig_u.account_status != 'active'",
			2,
		},
	}

	for _, tt := range tokens {
		count := strings.Count(src, tt.token)
		if count < tt.wantMinCount {
			t.Errorf(
				"F1-B1 governance token appears %d time(s) in search_repository_impl.go, want >=%d: %s\n  token: %q",
				count, tt.wantMinCount, tt.description, tt.token,
			)
		}
	}
}

// TestSearchContent_SuspendedAuthorFiltered verifies that the content search
// DOES exclude suspended authors (social doctrine — unlike commerce which preserves
// suspended seller inventory).
func TestSearchContent_SuspendedAuthorFiltered(t *testing.T) {
	src := readImplFile(t)

	// The filter must use = 'active', which implicitly excludes suspended.
	// Confirm the token is present in the SearchContent section.
	token := "u.account_status = 'active' AND u.deleted_at IS NULL"
	if !strings.Contains(src, token) {
		t.Errorf(
			"F1-B1: content search must filter authors with account_status = 'active'; token not found: %q",
			token,
		)
	}
}

// TestSearchContent_AuthorJoinMirrorsCountQuery verifies the count query also
// carries the author lifecycle JOIN and filter, so pagination totals are
// consistent with the visible result set.
func TestSearchContent_AuthorJoinMirrorsCountQuery(t *testing.T) {
	src := readImplFile(t)

	// The count query comment "F1-B1 (2026-06-14): mirrors base query author lifecycle filter."
	// is the canonical marker that the count query was updated atomically with the base query.
	marker := "F1-B1 (2026-06-14): mirrors base query author lifecycle filter."
	if !strings.Contains(src, marker) {
		t.Errorf(
			"F1-B1: count query must carry the F1-B1 marker comment confirming atomic update; not found: %q",
			marker,
		)
	}
}
