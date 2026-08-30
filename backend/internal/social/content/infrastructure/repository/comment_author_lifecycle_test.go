package repository

// F1-B1 — Comment author lifecycle regression lock.
//
// Verifies that ListByTarget (content and non-content branches) in
// comment_repository_impl.go applies the governance filter that excludes
// comments from suspended/banned/deleted authors.
//
// Comments are a public discovery surface: any user can view comments on a
// content. An author whose account is suspended, banned, or deleted must not
// have their comments surfaced.
//
// Replies are flat rows returned by the same ListByTarget list (parent_id);
// there is no separate ListReplies reader.
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

func readCommentImplFile(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	implPath := filepath.Join(filepath.Dir(thisFile), "comment_repository_impl.go")
	b, err := os.ReadFile(implPath)
	if err != nil {
		t.Fatalf("cannot read comment_repository_impl.go: %v", err)
	}
	// Normalize CRLF to LF so source-inspection assertions are
	// resilient to platform line-ending differences.
	normalized := strings.ReplaceAll(string(b), "\r\n", "\n")
	return normalized
}

// TestCommentListByTarget_AuthorLifecycleFilterInSQL verifies the F1-B1 filter
// is present in both query branches of ListByTarget (the canonical flat list;
// replies are rows in the same list and inherit the same filter).
func TestCommentListByTarget_AuthorLifecycleFilterInSQL(t *testing.T) {
	src := readCommentImplFile(t)

	tokens := []struct {
		description  string
		token        string
		wantMinCount int
	}{
		{
			"F1-B1: users JOIN in ListByTarget (content branch + non-content branch = 2 min)",
			"JOIN users u ON u.id = c.author_id",
			2,
		},
		{
			"F1-B1: account_status='active' filter in ListByTarget (content + non-content = 2 min)",
			"u.account_status = 'active' AND u.deleted_at IS NULL",
			2,
		},
	}

	for _, tt := range tokens {
		count := strings.Count(src, tt.token)
		if count < tt.wantMinCount {
			t.Errorf(
				"F1-B1 governance token appears %d time(s) in comment_repository_impl.go, want >=%d: %s\n  token: %q",
				count, tt.wantMinCount, tt.description, tt.token,
			)
		}
	}
}

// TestCommentRepository_NoSeparateReplyReader locks the canonical flat-list
// contract: there is no ListReplies reader to drift from the parent-filtered
// semantics of the canonical list.
func TestCommentRepository_NoSeparateReplyReader(t *testing.T) {
	src := readCommentImplFile(t)

	if strings.Contains(src, "func (r *CommentRepositoryImpl) ListReplies(") {
		t.Error("ListReplies was reintroduced; replies are flat rows served by ListByTarget")
	}
}