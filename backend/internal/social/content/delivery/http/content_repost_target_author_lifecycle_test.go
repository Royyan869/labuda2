package http

// F2 — Content detail repost target author lifecycle regression lock.
//
// Verifies that the repost target author lifecycle check exists in the
// service layer (validatePublicContentVisibility in content_service.go).
// The check was originally in the handler (GetContentPublic for origID)
// but was refactored into the recursive validatePublicContentVisibility
// function which handles both the content itself and its repost target.

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGetContent_RepostTargetAuthorLifecycleUsesPublicLookup(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	// Check content_service.go for the recursive repost target validation
	servicePath := filepath.Join(filepath.Dir(thisFile), "..", "..", "application", "content_service.go")
	b, err := os.ReadFile(servicePath)
	if err != nil {
		t.Fatalf("cannot read content_service.go: %v", err)
	}
	src := string(b)

	tokens := []struct {
		description  string
		token        string
		wantMinCount int
	}{
		{
			"recursive repost target visibility check",
			"s.validatePublicContentVisibility(ctx, tx, targetContent)",
			1,
		},
		{
			"target content existence check for reposts",
			"s.contentRepo.GetByID(ctx, tx, targetID)",
			1,
		},
	}

	for _, tt := range tokens {
		count := 0
		start := 0
		for {
			idx := searchSubstring(src[start:], tt.token)
			if idx < 0 {
				break
			}
			count++
			start += idx + 1
		}
		if count < tt.wantMinCount {
			t.Errorf(
				"content_service.go token appears %d time(s), want >=%d: %s\n  token: %q",
				count, tt.wantMinCount, tt.description, tt.token,
			)
		}
	}
}

// searchSubstring is a simple substring search (no regex).
func searchSubstring(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
