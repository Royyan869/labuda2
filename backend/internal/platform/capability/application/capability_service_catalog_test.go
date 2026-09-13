package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
)

func mustUUID(t *testing.T, raw string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(raw)
	if err != nil {
		t.Fatalf("invalid uuid %q: %v", raw, err)
	}
	return id
}

func newActiveGrant(userID uuid.UUID, capabilityStr string) *capabilityEntity.UserCapability {
	return &capabilityEntity.UserCapability{
		ID:         uuid.New(),
		UserID:     userID,
		Capability: capabilityStr,
		GrantedAt:  time.Now(),
	}
}

// TestListAllCapabilities_ExactSetEquality proves the governance API catalog is
// exactly the canonical universe — no capability is hidden from the dashboard,
// and no capability exists in the API that is not canonical.
//
// Set equality, deliberately not a count comparison: a count check would pass
// with one capability missing and one extra.
func TestListAllCapabilities_ExactSetEquality(t *testing.T) {
	svc := NewCapabilityService(newCapabilityServiceRepoMock(), capabilityServiceAuditLogger{})

	defs := svc.ListAllCapabilities(context.Background())

	fromAPI := make(map[string]bool, len(defs))
	for _, d := range defs {
		if fromAPI[d.Capability] {
			t.Fatalf("API catalog lists %q twice", d.Capability)
		}
		fromAPI[d.Capability] = true
		if d.Category == "" || d.Description == "" {
			t.Fatalf("%q is missing presentation metadata", d.Capability)
		}
	}

	universe := capability.AllCapabilityStrings()
	if len(fromAPI) != len(universe) {
		t.Fatalf("API catalog size %d != universe size %d", len(fromAPI), len(universe))
	}
	for _, c := range universe {
		if !fromAPI[c] {
			t.Fatalf("canonical capability %q is MISSING from the governance API catalog", c)
		}
	}
}

// TestIsCriticalCapability_MatchesCanonicalAuthority proves the service
// adapter delegates to the single criticality authority.
func TestIsCriticalCapability_MatchesCanonicalAuthority(t *testing.T) {
	for _, c := range capability.AllCapabilities() {
		if got, want := isCriticalCapability(c.String()), capability.IsCritical(c); got != want {
			t.Fatalf("isCriticalCapability(%q) = %v, want %v", c, got, want)
		}
	}
}

// TestGetUserAuthoritySummary_DerivesFullAccess proves full access is derived
// from role + active capability coverage, and that a role-only change flips it.
func TestGetUserAuthoritySummary_DerivesFullAccess(t *testing.T) {
	repo := newCapabilityServiceRepoMock()
	svc := NewCapabilityService(repo, capabilityServiceAuditLogger{})

	id := mustUUID(t, "11111111-1111-1111-1111-111111111111")
	for _, c := range capability.AllCapabilityStrings() {
		repo.activeCaps[repo.capKey(id, c)] = newActiveGrant(id, c)
	}

	// Capabilities alone are not full access.
	summary, err := svc.GetUserAuthoritySummary(context.Background(), id)
	if err != nil {
		t.Fatalf("summary failed: %v", err)
	}
	if summary.FullAccess {
		t.Fatal("user with all capabilities but role != admin must not be full access")
	}
	if len(summary.MissingCapabilities) != 0 {
		t.Fatalf("coverage should be complete, missing = %v", summary.MissingCapabilities)
	}

	// Adding admin membership makes it full access, with no stored flag.
	repo.roles[id] = capabilityEntity.AdminRole
	summary, err = svc.GetUserAuthoritySummary(context.Background(), id)
	if err != nil {
		t.Fatalf("summary failed: %v", err)
	}
	if !summary.FullAccess {
		t.Fatal("admin with entire universe must be full access")
	}
	if !summary.IsAdmin {
		t.Fatal("IsAdmin must mirror role == admin")
	}
}
