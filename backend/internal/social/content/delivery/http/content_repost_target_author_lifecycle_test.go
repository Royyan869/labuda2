package http

// F2 — Content detail repost target author lifecycle regression lock.
//
// Verifies that the public content-detail path uses GetContentPublic for the
// requested row and for the repost target row, so suspended/banned/deleted
// target authors cannot bypass visibility via a repost wrapper.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGetContent_RepostTargetAuthorLifecycleUsesPublicLookup(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	handlerPath := filepath.Join(filepath.Dir(thisFile), "content_handler.go")
	b, err := os.ReadFile(handlerPath)
	if err != nil {
		t.Fatalf("cannot read content_handler.go: %v", err)
	}
	src := string(b)

	tokens := []struct {
		description  string
		token        string
		wantMinCount int
	}{
		{
			"public lookup for requested content",
			"h.contentService.GetContentPublic(ctx, tx, contentID)",
			1,
		},
		{
			"public lookup for repost target",
			"h.contentService.GetContentPublic(ctx, tx, origID)",
			1,
		},
	}

	for _, tt := range tokens {
		count := strings.Count(src, tt.token)
		if count < tt.wantMinCount {
			t.Errorf(
				"content detail target-author token appears %d time(s) in content_handler.go, want >=%d: %s\n  token: %q",
				count, tt.wantMinCount, tt.description, tt.token,
			)
		}
	}
}


