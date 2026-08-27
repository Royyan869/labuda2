package schemaguard

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// forbiddenListingsRead matches SQL that reads from the legacy `listings`
// table (FROM listings / JOIN listings, case-insensitive, at a word
// boundary so it does not false-positive on for_sales or
// listing_shipping_options-style names).
var forbiddenListingsRead = regexp.MustCompile(`(?i)\b(from|join)\s+listings\b`)

// allowedHistoricalCommentFiles contains files whose only match is a
// historical SQL comment documenting a *past* fix (e.g. "-- Fixed
// 2026-06-21: was querying FROM listings ..."), not a live query. These are
// intentionally allowed so the guard doesn't force deleting the paper trail
// of the original bug.
var allowedHistoricalCommentFiles = map[string]bool{
	filepath.FromSlash("internal/discovery/search/infrastructure/repository/search_repository_impl.go"): true,
	filepath.FromSlash("internal/social/feed/infrastructure/repository/feed_repository_impl.go"):         true,
}

// TestNoProductionCodeReadsLegacyListingsTable is a PASS_21B negative guard.
//
// `listings` is the pre-refactor unified Product/Listing/Auction parent
// table (owner-rejected design: Auction sourced from Listing, Listing as
// generic sale parent). It is write-dead — no Go code inserts or updates it
// — but PASS_21A found several handlers still silently reading stale/empty
// data from it (moderation ResourceExists, seller dashboard, promotion
// operability, OG previews, saved-item hydration, notification worker, and
// the moderation fixed-price-sale preview). All known instances were fixed
// in PASS_21B. This test fails if any production .go file under
// backend/internal re-introduces a query against `listings`, so the
// rejected design cannot silently return.
//
// Test files are excluded: they may contain historical/spec-only SQL in
// comments (e.g. monitoring_service_test.go documents a query shape without
// executing it against a real table).
func TestNoProductionCodeReadsLegacyListingsTable(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve test file path")
	}
	// thisFile = .../backend/internal/platform/schemaguard/legacy_table_guard_test.go
	internalRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	backendRoot := filepath.Dir(internalRoot)

	var violations []string
	err := filepath.Walk(internalRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		relPath, relErr := filepath.Rel(backendRoot, path)
		if relErr != nil {
			return relErr
		}
		if allowedHistoricalCommentFiles[relPath] {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if forbiddenListingsRead.Match(content) {
			violations = append(violations, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk backend/internal: %v", err)
	}

	if len(violations) > 0 {
		t.Fatalf("found production code reading from the legacy `listings` table "+
			"(write-dead since the products/for_sales split) in:\n%s",
			strings.Join(violations, "\n"))
	}
}
