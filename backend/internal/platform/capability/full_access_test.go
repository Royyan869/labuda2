package capability

import (
	"testing"

	"github.com/labuda/backend/internal/platform/capability/entity"
)

// TestFullAccess_RequiresAdminRole proves role is half of the derived state:
// every capability in the universe still is not full access without admin
// membership.
func TestFullAccess_RequiresAdminRole(t *testing.T) {
	all := AllCapabilityStrings()

	if IsFullAccessAdmin(entity.AdminRole, all) != true {
		t.Fatal("admin + entire universe must be full access")
	}
	for _, role := range []string{"user", "", "seller", "super_admin"} {
		if IsFullAccessAdmin(role, all) {
			t.Fatalf("role %q with the entire universe must NOT be full access", role)
		}
	}
}

// TestFullAccess_RequiresFullCoverage proves one missing capability is enough to
// lose full access.
func TestFullAccess_RequiresFullCoverage(t *testing.T) {
	all := AllCapabilityStrings()

	for _, drop := range []int{0, len(all) / 2, len(all) - 1} {
		subset := make([]string, 0, len(all)-1)
		for i, c := range all {
			if i == drop {
				continue
			}
			subset = append(subset, c)
		}
		if IsFullAccessAdmin(entity.AdminRole, subset) {
			t.Fatalf("missing %q must not be full access", all[drop])
		}
		missing := MissingCapabilityStrings(subset)
		if len(missing) != 1 || missing[0] != all[drop] {
			t.Fatalf("missing set = %v, want [%s]", missing, all[drop])
		}
	}
}

// TestFullAccess_DuplicatesCannotFakeCoverage proves coverage is set-based, not
// row-count based: duplicated rows (a real defect found in fixture data) must
// not be mistaken for breadth.
func TestFullAccess_DuplicatesCannotFakeCoverage(t *testing.T) {
	all := AllCapabilityStrings()

	// Every capability repeated 5x — the exact shape the duplicate-grant defect
	// produced. Still full access, because coverage is real.
	duplicated := make([]string, 0, len(all)*5)
	for i := 0; i < 5; i++ {
		duplicated = append(duplicated, all...)
	}
	if !IsFullAccessAdmin(entity.AdminRole, duplicated) {
		t.Fatal("duplicated full coverage is still full coverage")
	}

	// Conversely, a single capability repeated 46 times is NOT full access even
	// though it has more rows than the universe.
	oneRepeated := make([]string, 0, len(all)+10)
	for i := 0; i < len(all)+10; i++ {
		oneRepeated = append(oneRepeated, all[0])
	}
	if IsFullAccessAdmin(entity.AdminRole, oneRepeated) {
		t.Fatal("row count must never substitute for coverage")
	}
}

// TestFullAccess_EmptyCapabilitySet documents the degenerate case.
func TestFullAccess_EmptyCapabilitySet(t *testing.T) {
	if IsFullAccessAdmin(entity.AdminRole, nil) {
		t.Fatal("admin with no capabilities is not full access")
	}
	if got := len(MissingCapabilityStrings(nil)); got != len(AllCapabilities()) {
		t.Fatalf("missing count = %d, want %d", got, len(AllCapabilities()))
	}
}
