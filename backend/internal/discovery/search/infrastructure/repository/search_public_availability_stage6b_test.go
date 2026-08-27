package repository

// Stage 6B — public availability convergence regression lock.
//
// Source-file inspection (same pattern as search_banned_seller_suppression_test.go):
// verifies that the public FPS discovery queries enforce quantity_available > 0
// and that auction discovery only admits public (scheduled/active) states.
// These predicates live inside SQL built in the repository methods, so the
// lock asserts their presence in both the base and the count queries.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestSearchForSales_QuantityAvailablePredicateInSQL verifies that BOTH the
// base query and the count query of SearchForSales filter out-of-stock
// (quantity_available = 0) fixed-price sales.
func TestSearchForSales_QuantityAvailablePredicateInSQL(t *testing.T) {
	src := readSearchImplFile(t)
	tokens := []struct {
		description  string
		token        string
		wantMinCount int
	}{
		{
			"availability predicate in SearchForSales (base + count = 2 occurrences)",
			"fps.quantity_available > 0",
			2,
		},
	}
	for _, tt := range tokens {
		count := strings.Count(src, tt.token)
		if count < tt.wantMinCount {
			t.Errorf(
				"public availability token appears %d time(s) in search_repository_impl.go, want >=%d: %s\n  token: %q",
				count, tt.wantMinCount, tt.description, tt.token,
			)
		}
	}
}

// TestSearchAuctions_PublicDiscoverableStatesInSQL verifies that BOTH the
// base query and the count query of SearchAuctions restrict discovery to the
// canonical public auction states (scheduled, active) and no longer admit
// ended/draft/cancelled surfaces.
func TestSearchAuctions_PublicDiscoverableStatesInSQL(t *testing.T) {
	src := readSearchImplFile(t)
	tokens := []struct {
		description  string
		token        string
		wantMinCount int
	}{
		{
			"auction discovery state predicate (base + count = 2 occurrences)",
			"a.status IN ('scheduled', 'active')",
			2,
		},
		{
			"discovery must not admit ended auctions (base + count both rewritten)",
			"'scheduled', 'active', 'ended'",
			0,
		},
	}
	for _, tt := range tokens {
		count := strings.Count(src, tt.token)
		if tt.wantMinCount == 0 && count > 0 {
			t.Errorf(
				"rejected auction discovery token still present %d time(s): %s\n  token: %q",
				count, tt.description, tt.token,
			)
		}
		if tt.wantMinCount > 0 && count < tt.wantMinCount {
			t.Errorf(
				"auction discovery state token appears %d time(s), want >=%d: %s\n  token: %q",
				count, tt.wantMinCount, tt.description, tt.token,
			)
		}
	}
}

func readSearchImplFile(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	implPath := filepath.Join(filepath.Dir(thisFile), "search_repository_impl.go")
	b, err := os.ReadFile(implPath)
	if err != nil {
		t.Fatalf("cannot read search_repository_impl.go: %v", err)
	}
	return string(b)
}
