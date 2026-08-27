package application

import (
	"os"
	"testing"

	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// publicSellerTier — 4-gate visibility tests
// =============================================================================

func setTierFlag(t *testing.T, val string) {
	t.Helper()
	t.Setenv("ENABLE_PUBLIC_SELLER_TIER_PROFILE", val)
}

func strPtr(s string) *string { return &s }

func TestPublicSellerTier_FlagDisabled_ReturnsNil(t *testing.T) {
	os.Unsetenv("ENABLE_PUBLIC_SELLER_TIER_PROFILE")
	seller := &SellerState{HasMarketAuthority: true, Tier: strPtr("pro")}
	assert.Nil(t, publicSellerTier("active", seller))
}

func TestPublicSellerTier_FlagFalse_ReturnsNil(t *testing.T) {
	setTierFlag(t, "false")
	seller := &SellerState{HasMarketAuthority: true, Tier: strPtr("pro")}
	assert.Nil(t, publicSellerTier("active", seller))
}

func TestPublicSellerTier_FlagTrue_ProEmitted(t *testing.T) {
	setTierFlag(t, "true")
	seller := &SellerState{HasMarketAuthority: true, Tier: strPtr("pro")}
	result := publicSellerTier("active", seller)
	assert.NotNil(t, result)
	assert.Equal(t, "pro", *result)
}

func TestPublicSellerTier_Flag1_EliteEmitted(t *testing.T) {
	setTierFlag(t, "1")
	seller := &SellerState{HasMarketAuthority: true, Tier: strPtr("elite")}
	result := publicSellerTier("active", seller)
	assert.NotNil(t, result)
	assert.Equal(t, "elite", *result)
}

func TestPublicSellerTier_FlagYes_Works(t *testing.T) {
	setTierFlag(t, "yes")
	seller := &SellerState{HasMarketAuthority: true, Tier: strPtr("pro")}
	result := publicSellerTier("active", seller)
	assert.NotNil(t, result)
	assert.Equal(t, "pro", *result)
}

// Gate 2: user lifecycle

func TestPublicSellerTier_SuspendedLifecycle_ReturnsNil(t *testing.T) {
	setTierFlag(t, "true")
	seller := &SellerState{HasMarketAuthority: true, Tier: strPtr("elite")}
	assert.Nil(t, publicSellerTier("unavailable", seller))
}

func TestPublicSellerTier_RemovedLifecycle_ReturnsNil(t *testing.T) {
	setTierFlag(t, "true")
	seller := &SellerState{HasMarketAuthority: true, Tier: strPtr("pro")}
	assert.Nil(t, publicSellerTier("removed", seller))
}

// Gate 3: market authority

func TestPublicSellerTier_NilSeller_ReturnsNil(t *testing.T) {
	setTierFlag(t, "true")
	assert.Nil(t, publicSellerTier("active", nil))
}

func TestPublicSellerTier_NoMarketAuthority_ReturnsNil(t *testing.T) {
	setTierFlag(t, "true")
	seller := &SellerState{HasMarketAuthority: false, Tier: strPtr("pro")}
	assert.Nil(t, publicSellerTier("active", seller))
}

// Gate 4: tier value

func TestPublicSellerTier_BasicTier_ReturnsNil(t *testing.T) {
	setTierFlag(t, "true")
	seller := &SellerState{HasMarketAuthority: true, Tier: strPtr("basic")}
	assert.Nil(t, publicSellerTier("active", seller))
}

func TestPublicSellerTier_NilTier_ReturnsNil(t *testing.T) {
	setTierFlag(t, "true")
	seller := &SellerState{HasMarketAuthority: true, Tier: nil}
	assert.Nil(t, publicSellerTier("active", seller))
}

func TestPublicSellerTier_UnknownTier_ReturnsNil(t *testing.T) {
	setTierFlag(t, "true")
	seller := &SellerState{HasMarketAuthority: true, Tier: strPtr("legendary")}
	assert.Nil(t, publicSellerTier("active", seller))
}

// =============================================================================
// Gate unification regression — publicSellerTier must produce identical results
// to publiccard.GatedSellerTier for equivalent inputs.
// Prevents future policy divergence between the profile surface and the
// for_sale/auction surfaces that call GatedSellerTier directly.
// =============================================================================

func TestGateUnification_ProfileMatchesForSaleAuction(t *testing.T) {
	cases := []struct {
		name              string
		flag              string
		tier              *string
		lifecycle         string
		hasMarketAuth     bool
		wantProfileResult *string
	}{
		{"flag off", "", strPtr("pro"), "active", true, nil},
		{"suspended user", "true", strPtr("pro"), "unavailable", true, nil},
		{"no market authority", "true", strPtr("pro"), "active", false, nil},
		{"basic tier", "true", strPtr("basic"), "active", true, nil},
		{"nil tier", "true", nil, "active", true, nil},
		{"pro all pass", "true", strPtr("pro"), "active", true, strPtr("pro")},
		{"elite all pass", "true", strPtr("elite"), "active", true, strPtr("elite")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ENABLE_PUBLIC_SELLER_TIER_PROFILE", tc.flag)

			seller := &SellerState{HasMarketAuthority: tc.hasMarketAuth, Tier: tc.tier}
			profileResult := publicSellerTier(tc.lifecycle, seller)

			// Construct the equivalent GatedSellerTier inputs:
			// - tier string: "" when nil, else raw value
			// - trustLifecycle: "active" when HasMarketAuthority, else "unavailable"
			tierStr := ""
			if tc.tier != nil {
				tierStr = *tc.tier
			}
			trustLifecycle := "unavailable"
			if tc.hasMarketAuth {
				trustLifecycle = "active"
			}
			gatedResult := publiccard.GatedSellerTier(tierStr, tc.lifecycle, trustLifecycle)

			// Both must agree.
			assert.Equal(t, tc.wantProfileResult, profileResult,
				"publicSellerTier mismatch for case %q", tc.name)
			assert.Equal(t, profileResult, gatedResult,
				"publicSellerTier vs GatedSellerTier diverged for case %q", tc.name)
		})
	}
}
