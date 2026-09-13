package capability

import "testing"

// TestCatalog_ExactCoverageOfUniverse proves the governance catalog is derived
// from the canonical universe with EXACT set equality (not count equality).
//
// This is the anti-drift guarantee: the API catalog, the dashboard, and the
// derived full-access universe cannot diverge, because there is only one list.
func TestCatalog_ExactCoverageOfUniverse(t *testing.T) {
	universe := AllCapabilityStrings()
	catalog := Catalog()

	if len(catalog) != len(universe) {
		t.Fatalf("catalog size %d != universe size %d", len(catalog), len(universe))
	}

	fromCatalog := make(map[string]bool, len(catalog))
	for _, d := range catalog {
		if fromCatalog[d.Capability] {
			t.Fatalf("catalog contains %q twice", d.Capability)
		}
		fromCatalog[d.Capability] = true
	}

	for _, c := range universe {
		if !fromCatalog[c] {
			t.Fatalf("capability %q is in the universe but MISSING from the catalog", c)
		}
	}
	for _, d := range catalog {
		if !IsValid(d.Capability) {
			t.Fatalf("catalog contains %q which is not a valid capability", d.Capability)
		}
	}
}

// TestUniverse_HasNoDuplicates proves the one source of truth is itself clean.
func TestUniverse_HasNoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range AllCapabilities() {
		if seen[c.String()] {
			t.Fatalf("duplicate capability in canonical universe: %q", c)
		}
		seen[c.String()] = true
	}
}

// TestCatalog_NoClusterIsDropped proves every capability surfaces in a named
// cluster. A cluster can never be silently hidden by a missing grouping entry,
// because the category is derived from the capability string.
func TestCatalog_NoClusterIsDropped(t *testing.T) {
	clusters := map[string]bool{}
	for _, d := range Catalog() {
		if d.Category == "" {
			t.Fatalf("%q has an empty category", d.Capability)
		}
		if d.Description == "" {
			t.Fatalf("%q has an empty description", d.Capability)
		}
		clusters[d.Category] = true
	}

	// The clusters that exist in the canonical universe. If a new cluster is
	// introduced, this assertion is the deliberate reminder that the dashboard
	// will render it dynamically.
	want := []string{"Config", "Finance", "Governance", "Moderation", "Order", "Promotion", "Seller", "Support"}
	for _, c := range want {
		if !clusters[c] {
			t.Fatalf("cluster %q is absent from the catalog", c)
		}
	}
}

// TestCatalog_CriticalityHasOneAuthority proves criticality is keyed by the
// capability itself and matches the catalog's reported flag.
func TestCatalog_CriticalityHasOneAuthority(t *testing.T) {
	for _, d := range Catalog() {
		if IsCritical(Capability(d.Capability)) != d.Critical {
			t.Fatalf("criticality mismatch for %q", d.Capability)
		}
	}

	// The governance capabilities that gate admin recruitment must be critical.
	for _, c := range []Capability{CapGovernanceRoleAssign, CapGovernanceCapabilityAssign} {
		if !IsCritical(c) {
			t.Fatalf("%q must be critical", c)
		}
	}
}

// TestCategory_DerivedFromCluster documents the derivation.
func TestCategory_DerivedFromCluster(t *testing.T) {
	cases := map[Capability]string{
		CapFinanceWithdrawRead:     "Finance",
		CapGovernanceRoleAssign:    "Governance",
		CapModerationAppealReview:  "Moderation",
		CapPromotionCampaignStop:   "Promotion",
		CapSellerSubscriptionRecover: "Seller",
		CapOrderRead:               "Order",
		CapConfigUpdateFinancial:   "Config",
		CapSupportTicketEscalate:   "Support",
	}
	for c, want := range cases {
		if got := Category(c); got != want {
			t.Fatalf("Category(%q) = %q, want %q", c, got, want)
		}
	}
}
