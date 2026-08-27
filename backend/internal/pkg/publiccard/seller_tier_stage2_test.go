package publiccard

// Stage 2 seller tier — tests for GatedSellerTier gate logic and SellerCard
// wire shape with tier field populated / absent.
//
// Coverage:
//   - FIX-7 feature flag gate: disabled flag → nil tier on every surface.
//   - Lifecycle gates: suspended / expired-subscription → nil tier.
//   - Basic tier → nil (never publicly shown).
//   - Pro + Elite with all gates passing → tier string emitted.
//   - SellerCard.Tier omitempty: nil tier absent from JSON wire.
//   - SellerCard.Tier present: tier field on wire when all gates pass.
//   - NewSellerCardWithBothLifecycles wires GatedSellerTier correctly.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// setTierFlag sets ENABLE_PUBLIC_SELLER_TIER_PROFILE and returns a cleanup fn.
func setTierFlag(t *testing.T, val string) func() {
	t.Helper()
	prev := os.Getenv("ENABLE_PUBLIC_SELLER_TIER_PROFILE")
	os.Setenv("ENABLE_PUBLIC_SELLER_TIER_PROFILE", val)
	return func() { os.Setenv("ENABLE_PUBLIC_SELLER_TIER_PROFILE", prev) }
}

// ── GatedSellerTier unit tests ──────────────────────────────────────────────

func TestGatedSellerTier_FlagDisabled_ReturnsNil(t *testing.T) {
	for _, flag := range []string{"", "false", "0", "no", "FALSE"} {
		t.Run("flag="+flag, func(t *testing.T) {
			cleanup := setTierFlag(t, flag)
			defer cleanup()
			if got := GatedSellerTier("pro", "active", "active"); got != nil {
				t.Fatalf("flag=%q: expected nil, got %q", flag, *got)
			}
		})
	}
}

func TestGatedSellerTier_FlagEnabled_Variants(t *testing.T) {
	for _, flag := range []string{"true", "1", "yes", "TRUE", "True"} {
		t.Run("flag="+flag, func(t *testing.T) {
			cleanup := setTierFlag(t, flag)
			defer cleanup()
			got := GatedSellerTier("pro", "active", "active")
			if got == nil {
				t.Fatalf("flag=%q: expected non-nil tier", flag)
			}
			if *got != "pro" {
				t.Fatalf("flag=%q: expected \"pro\", got %q", flag, *got)
			}
		})
	}
}

func TestGatedSellerTier_UserIdentityDegraded_ReturnsNil(t *testing.T) {
	cleanup := setTierFlag(t, "true")
	defer cleanup()
	for _, lc := range []string{"unavailable", "removed", ""} {
		t.Run("userLifecycle="+lc, func(t *testing.T) {
			if got := GatedSellerTier("pro", lc, "active"); got != nil {
				t.Fatalf("userLifecycle=%q: expected nil, got %q", lc, *got)
			}
		})
	}
}

func TestGatedSellerTier_SellerTrustDegraded_ReturnsNil(t *testing.T) {
	cleanup := setTierFlag(t, "true")
	defer cleanup()
	for _, lc := range []string{"unavailable", "removed", ""} {
		t.Run("trustLifecycle="+lc, func(t *testing.T) {
			if got := GatedSellerTier("pro", "active", lc); got != nil {
				t.Fatalf("trustLifecycle=%q: expected nil, got %q", lc, *got)
			}
		})
	}
}

func TestGatedSellerTier_BasicTier_ReturnsNil(t *testing.T) {
	cleanup := setTierFlag(t, "true")
	defer cleanup()
	for _, tier := range []string{"basic", "", "unknown", "legend"} {
		t.Run("tier="+tier, func(t *testing.T) {
			if got := GatedSellerTier(tier, "active", "active"); got != nil {
				t.Fatalf("tier=%q: expected nil, got %q", tier, *got)
			}
		})
	}
}

func TestGatedSellerTier_ProAllGatesPass(t *testing.T) {
	cleanup := setTierFlag(t, "true")
	defer cleanup()
	got := GatedSellerTier("pro", "active", "active")
	if got == nil {
		t.Fatal("expected non-nil tier for pro")
	}
	if *got != "pro" {
		t.Fatalf("expected \"pro\", got %q", *got)
	}
}

func TestGatedSellerTier_EliteAllGatesPass(t *testing.T) {
	cleanup := setTierFlag(t, "true")
	defer cleanup()
	got := GatedSellerTier("elite", "active", "active")
	if got == nil {
		t.Fatal("expected non-nil tier for elite")
	}
	if *got != "elite" {
		t.Fatalf("expected \"elite\", got %q", *got)
	}
}

// ── SellerCard wire shape ────────────────────────────────────────────────────

func TestSellerCard_NilTier_AbsentFromWire(t *testing.T) {
	// When Tier is nil the json:"tier,omitempty" tag must suppress the field.
	cleanup := setTierFlag(t, "") // flag off → GatedSellerTier returns nil
	defer cleanup()
	card := NewSellerCardWithBothLifecycles(
		uuid.New(), "alice", nil, "Acme Farm",
		"active", "active", "pro",
	)
	if card.Tier != nil {
		t.Fatalf("flag off: expected nil Tier, got %q", *card.Tier)
	}
	b, _ := json.Marshal(card)
	if strings.Contains(string(b), `"tier"`) {
		t.Fatalf("nil Tier must be omitted from wire; got: %s", b)
	}
}

func TestSellerCard_ProTier_PresentOnWire(t *testing.T) {
	cleanup := setTierFlag(t, "true")
	defer cleanup()
	card := NewSellerCardWithBothLifecycles(
		uuid.New(), "alice", nil, "Acme Farm",
		"active", "active", "pro",
	)
	if card.Tier == nil {
		t.Fatal("expected non-nil Tier")
	}
	if *card.Tier != "pro" {
		t.Fatalf("expected \"pro\", got %q", *card.Tier)
	}
	b, _ := json.Marshal(card)
	if !strings.Contains(string(b), `"tier":"pro"`) {
		t.Fatalf("expected tier on wire; got: %s", b)
	}
}

func TestSellerCard_SuspendedSeller_TierNil(t *testing.T) {
	cleanup := setTierFlag(t, "true")
	defer cleanup()
	card := NewSellerCardWithBothLifecycles(
		uuid.New(), "alice", nil, "Acme Farm",
		"unavailable", // suspended — user-identity axis degraded
		"active",
		"pro",
	)
	if card.Tier != nil {
		t.Fatalf("suspended seller: Tier must be nil, got %q", *card.Tier)
	}
}

func TestSellerCard_ExpiredSubscription_TierNil(t *testing.T) {
	cleanup := setTierFlag(t, "true")
	defer cleanup()
	card := NewSellerCardWithBothLifecycles(
		uuid.New(), "alice", nil, "Acme Farm",
		"active",
		"unavailable", // expired subscription — seller-trust axis degraded
		"elite",
	)
	if card.Tier != nil {
		t.Fatalf("expired subscription: Tier must be nil, got %q", *card.Tier)
	}
}

func TestSellerCard_SearchSurface_EmptyTier_NilOnWire(t *testing.T) {
	// Search passes "" as tier — must produce nil Tier regardless of flag.
	cleanup := setTierFlag(t, "true")
	defer cleanup()
	card := NewSellerCardWithBothLifecycles(
		uuid.New(), "alice", nil, "Acme Farm",
		"active", "active",
		"", // search: Stage 2 deferred
	)
	if card.Tier != nil {
		t.Fatalf("search surface empty tier: expected nil, got %q", *card.Tier)
	}
	b, _ := json.Marshal(card)
	if strings.Contains(string(b), `"tier"`) {
		t.Fatalf("empty tier must be absent from wire; got: %s", b)
	}
}

func TestSellerCard_BothLifecycles_TierPreservesAxisBoundary(t *testing.T) {
	// Adding Tier to SellerCard must NOT break the existing axis-boundary
	// contract: top-level Lifecycle is populated, User.Lifecycle is populated,
	// and they remain independent.
	cleanup := setTierFlag(t, "true")
	defer cleanup()
	card := NewSellerCardWithBothLifecycles(
		uuid.New(), "alice", nil, "Farm",
		"active", "active", "elite",
	)
	if card.Lifecycle == nil || *card.Lifecycle != "active" {
		t.Fatalf("top-level Lifecycle must be \"active\"; got %v", card.Lifecycle)
	}
	if card.User.Lifecycle == nil || *card.User.Lifecycle != "active" {
		t.Fatalf("User.Lifecycle must be \"active\"; got %v", card.User.Lifecycle)
	}
	if card.Tier == nil || *card.Tier != "elite" {
		t.Fatalf("Tier must be \"elite\"; got %v", card.Tier)
	}
}


