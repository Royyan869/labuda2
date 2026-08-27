package repository

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGetFeed_AuctionClauseDoesNotReferenceDeletedAt(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	implPath := filepath.Join(filepath.Dir(thisFile), "feed_repository_impl.go")
	b, err := os.ReadFile(implPath)
	if err != nil {
		t.Fatalf("cannot read feed_repository_impl.go: %v", err)
	}
	src := string(b)

	if strings.Contains(src, "a.deleted_at") {
		t.Fatal("feed SQL still references nonexistent auctions.deleted_at")
	}

	want := "a.status NOT IN ('scheduled', 'active')"
	if !strings.Contains(src, want) {
		t.Fatalf("feed SQL missing canonical auction lifecycle predicate %q", want)
	}
}
