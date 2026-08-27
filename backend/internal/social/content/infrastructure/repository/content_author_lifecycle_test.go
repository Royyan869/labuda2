package repository

// F1-B1 — Content ListByAuthor (profile feed) author lifecycle regression lock.
//
// Verifies that ListByAuthor in content_repository_impl.go contains the
// governance filter that excludes content from suspended/banned/deleted authors.
//
// The profile feed (/users/:id/contents) is a public discovery surface.
// Viewing another user's profile must not expose content from suspended,
// banned, or soft-deleted account holders.
//
// Uses source-file inspection (no DB required). Pattern mirrors
// search_banned_seller_suppression_test.go.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func readContentImplFile(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	implPath := filepath.Join(filepath.Dir(thisFile), "content_repository_impl.go")
	b, err := os.ReadFile(implPath)
	if err != nil {
		t.Fatalf("cannot read content_repository_impl.go: %v", err)
	}
	return string(b)
}

// TestListByAuthor_AuthorLifecycleFilterInSQL verifies that ListByAuthor
// carries the F1-B1 governance filter.
func TestListByAuthor_AuthorLifecycleFilterInSQL(t *testing.T) {
	src := readContentImplFile(t)

	tokens := []struct {
		description  string
		token        string
		wantMinCount int
	}{
		{
			"F1-B1: users JOIN in ListByAuthor",
			"JOIN users u ON u.id = c.author_id",
			1,
		},
		{
			"F1-B1: account_status='active' filter in ListByAuthor",
			"u.account_status = 'active' AND u.deleted_at IS NULL",
			1,
		},
		{
			"F2: repost target author lifecycle check in ListByAuthor",
			"orig_u.account_status != 'active'",
			1,
		},
		{
			"F1-B1: c. table alias used (required after JOIN to avoid column ambiguity)",
			"FROM contents c",
			1,
		},
	}

	for _, tt := range tokens {
		count := strings.Count(src, tt.token)
		if count < tt.wantMinCount {
			t.Errorf(
				"F1-B1 governance token appears %d time(s) in content_repository_impl.go, want >=%d: %s\n  token: %q",
				count, tt.wantMinCount, tt.description, tt.token,
			)
		}
	}
}


