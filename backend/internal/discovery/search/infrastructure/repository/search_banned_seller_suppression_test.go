package repository

// E2B1 — Banned/deleted seller discovery suppression regression lock.
//
// Verifies that both the base query and the count query in SearchForSales
// and SearchAuctions contain the governance filter that excludes forSales
// and auctions whose seller is banned (account_status='banned') or deleted
// (deleted_at IS NOT NULL).
//
// Policy:
//   - BANNED  → excluded from discovery (permanent governance enforcement)
//   - DELETED → excluded from discovery (account removal)
//   - SUSPENDED → NOT excluded (reversible; inventory preserved)
//   - EXPIRED subscription → NOT excluded (demotion-only, existing doctrine)
//
// Uses source-file inspection (same pattern as search_repost_governance_fix1_test.go)
// because the SQL strings are local variables built inside the repository
// methods and cannot be accessed from outside without extracting them.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestSearchForSales_BannedDeletedSellerFilterInSQL verifies that the
// E2B1 governance filter is present in BOTH the base query and the count
// query of SearchForSales. Each token must appear at least twice.
func TestSearchForSales_BannedDeletedSellerFilterInSQL(t *testing.T) {
	src := readImplFile(t)

	tokens := []struct {
		description  string
		token        string
		wantMinCount int
	}{
		{
			"banned/deleted filter predicate in SearchForSales (base+count = 2 occurrences)",
			`u.account_status != 'banned' AND u.deleted_at IS NULL`,
			// Must appear in SearchForSales base query AND count query.
			// SearchAuctions also uses this token, so total >= 4 but we
			// only assert >= 2 to avoid coupling to auction count.
			2,
		},
		{
			"fail-open orphan guard (u.id IS NULL) in SearchForSales",
			"u.id IS NULL OR (u.account_status",
			2,
		},
		{
			"count query LEFT JOIN users for forSales (fps table after Phase 2A)",
			"LEFT JOIN users u ON u.id = fps.seller_id",
			1,
		},
	}

	for _, tt := range tokens {
		count := strings.Count(src, tt.token)
		if count < tt.wantMinCount {
			t.Errorf(
				"E2B1 governance token appears %d time(s) in search_repository_impl.go, want >=%d: %s\n  token: %q",
				count, tt.wantMinCount, tt.description, tt.token,
			)
		}
	}
}

// TestSearchAuctions_BannedDeletedSellerFilterInSQL verifies that the
// E2B1 governance filter is present in BOTH the base query and the count
// query of SearchAuctions.
func TestSearchAuctions_BannedDeletedSellerFilterInSQL(t *testing.T) {
	src := readImplFile(t)

	tokens := []struct {
		description  string
		token        string
		wantMinCount int
	}{
		{
			"count query LEFT JOIN users for auctions",
			"LEFT JOIN users u ON u.id = a.seller_id",
			1,
		},
		{
			"banned/deleted predicate present in auction base+count queries (total >=4 across both functions)",
			`u.account_status != 'banned' AND u.deleted_at IS NULL`,
			4, // 2 from SearchForSales + 2 from SearchAuctions
		},
	}

	for _, tt := range tokens {
		count := strings.Count(src, tt.token)
		if count < tt.wantMinCount {
			t.Errorf(
				"E2B1 governance token appears %d time(s) in search_repository_impl.go, want >=%d: %s\n  token: %q",
				count, tt.wantMinCount, tt.description, tt.token,
			)
		}
	}
}

// TestSearchForSales_SuspendedSellerNotFiltered verifies that the SUSPENDED
// account_status is NOT added as an exclusion predicate. Suspension is a
// reversible governance state — inventory must remain discoverable.
func TestSearchForSales_SuspendedSellerNotFiltered(t *testing.T) {
	src := readImplFile(t)

	// These patterns would indicate accidental suppression of suspended sellers.
	forbiddenPatterns := []struct {
		description string
		token       string
	}{
		{
			"suspended exclusion in for_sale WHERE clause",
			`account_status != 'suspended'`,
		},
		{
			"suspended exclusion via NOT IN",
			`account_status NOT IN ('suspended'`,
		},
	}

	for _, tt := range forbiddenPatterns {
		if strings.Contains(src, tt.token) {
			t.Errorf(
				"E2B1 forbidden pattern found — SUSPENDED sellers must NOT be filtered from discovery: %s\n  token: %q",
				tt.description, tt.token,
			)
		}
	}
}

// readImplFile reads search_repository_impl.go relative to this test file.
func readImplFile(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	implPath := filepath.Join(filepath.Dir(thisFile), "search_repository_impl.go")
	b, err := os.ReadFile(implPath)
	if err != nil {
		t.Fatalf("cannot read search_repository_impl.go: %v", err)
	}
	return string(b)
}
