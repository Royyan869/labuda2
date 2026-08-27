package repository

// F2 — Feed repost target author lifecycle regression lock.
//
// Verifies that GetFeed in feed_repository_impl.go contains the additional
// repost-target author lifecycle predicate that excludes content-type reposts
// whose target author is suspended, banned, or soft-deleted.
//
// Source inspection is sufficient here because the SQL is embedded in the
// repository method and the feed integration tests cover runtime behavior.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGetFeed_RepostTargetAuthorLifecycleClauseInSQL(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	implPath := filepath.Join(filepath.Dir(thisFile), "feed_repository_impl.go")
	b, err := os.ReadFile(implPath)
	if err != nil {
		t.Fatalf("cannot read feed_repository_impl.go: %v", err)
	}
	src := string(b)

	tokens := []struct {
		description  string
		token        string
		wantMinCount int
	}{
		{
			"repost target author LEFT JOIN",
			"LEFT JOIN users orig_u ON orig_u.id = orig.author_id",
			1,
		},
		{
			"repost target author lifecycle predicate",
			"orig_u.account_status != 'active'",
			1,
		},
	}

	for _, tt := range tokens {
		count := strings.Count(src, tt.token)
		if count < tt.wantMinCount {
			t.Errorf(
				"feed repost target author token appears %d time(s) in feed_repository_impl.go, want >=%d: %s\n  token: %q",
				count, tt.wantMinCount, tt.description, tt.token,
			)
		}
	}
}


