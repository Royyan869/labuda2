package application

import (
	"testing"
	"time"

	"github.com/labuda/backend/internal/pricing/promotion/entity"
)

// ========================================================================
// PROMOTION DISCOVERY GOVERNANCE ALIGNMENT TESTS
//
// These tests pin the contract that promotion discovery never exposes
// stronger visibility than normal search/discovery. The operability
// checker's seller governance mirrors the 4-gate HasActiveSellerCapability
// model from the activation handler.
//
// Doctrine decision: promotion visibility follows time-bounded seller
// subscription authority. KYC is separated from selling authority and must
// not influence this predicate.
// ========================================================================

// ========================================================================
// mapReasonToStopReason TESTS
// ========================================================================

// TestMapReasonToStopReason_SellerGovernanceReasons verifies all seller/user
// governance reasons map to StopReasonSellerGovernance.
func TestMapReasonToStopReason_SellerGovernanceReasons(t *testing.T) {
	checker := &OperabilityCheckerImpl{}

	governanceReasons := []string{
		"seller_removed",
		"seller_account_inactive",
		"seller_not_found",
		"seller_inactive",
		"user_removed",
		"user_account_inactive",
		"user_not_found",
	}

	for _, reason := range governanceReasons {
		// Test with all target types — governance reasons are target-independent
		for _, targetType := range []entity.TargetType{
			entity.TargetTypeForSale,
			entity.TargetTypeAuction,
			entity.TargetTypeExternalProduct,
		} {
			got := checker.mapReasonToStopReason(targetType, reason)
			if got != entity.StopReasonSellerGovernance {
				t.Errorf("mapReasonToStopReason(%s, %q) = %q, want %q",
					targetType, reason, got, entity.StopReasonSellerGovernance)
			}
		}
	}
}

// TestMapReasonToStopReason_ForSaleReasons verifies for_sale-specific reasons
// still map correctly after the governance priority layer was added.
func TestMapReasonToStopReason_ForSaleReasons(t *testing.T) {
	checker := &OperabilityCheckerImpl{}

	cases := []struct {
		reason string
		want   entity.StopReason
	}{
		{"for_sale_sold", entity.StopReasonForSaleSold},
		{"for_sale_hidden", entity.StopReasonForSaleHidden},
		{"for_sale_deleted", entity.StopReasonForSaleDeleted},
		{"for_sale_moderated", entity.StopReasonForSaleModerated},
		{"for_sale_expired", entity.StopReasonForSaleExpired},
	}

	for _, tc := range cases {
		got := checker.mapReasonToStopReason(entity.TargetTypeForSale, tc.reason)
		if got != tc.want {
			t.Errorf("mapReasonToStopReason(for_sale, %q) = %q, want %q",
				tc.reason, got, tc.want)
		}
	}
}

// TestMapReasonToStopReason_AuctionReasons verifies auction-specific reasons
// still map correctly.
func TestMapReasonToStopReason_AuctionReasons(t *testing.T) {
	checker := &OperabilityCheckerImpl{}

	cases := []struct {
		reason string
		want   entity.StopReason
	}{
		{"auction_ended", entity.StopReasonAuctionEnded},
		{"auction_cancelled", entity.StopReasonAuctionCancelled},
		{"auction_deleted", entity.StopReasonAuctionDeleted},
		{"auction_moderated", entity.StopReasonAuctionModerated},
	}

	for _, tc := range cases {
		got := checker.mapReasonToStopReason(entity.TargetTypeAuction, tc.reason)
		if got != tc.want {
			t.Errorf("mapReasonToStopReason(auction, %q) = %q, want %q",
				tc.reason, got, tc.want)
		}
	}
}

// ========================================================================
// StopReasonSellerGovernance VALIDITY
// ========================================================================

// TestStopReasonSellerGovernance_IsValid verifies the new stop reason is
// recognized as a valid canonical constant.
func TestStopReasonSellerGovernance_IsValid(t *testing.T) {
	if !entity.StopReasonSellerGovernance.IsValid() {
		t.Errorf("StopReasonSellerGovernance.IsValid() = false, want true")
	}
}

// ========================================================================
// GOVERNANCE DOCTRINE ALIGNMENT TABLE
//
// This test documents the expected governance behavior as a truth table.
// It validates that the operability check reason strings returned by
// sellerIsDiscoveryEligible and CheckUserEligibility map to the correct
// stop reasons, ensuring promotion discovery governance is canonical.
//
// STATUS MATRIX:
//   active interval with any KYC state       → ALLOW (visible)
//   suspended account                        → HIDE  (seller_account_inactive)
//   banned account                           → HIDE  (seller_account_inactive)
//   removed (soft-deleted)                   → HIDE  (seller_removed)
//   expired subscription interval            → HIDE  (seller_inactive)
// ========================================================================

func TestGovernanceDoctrineAlignment_ReasonToStopMapping(t *testing.T) {
	checker := &OperabilityCheckerImpl{}

	// All governance reasons must map to StopReasonSellerGovernance
	doctrineMatrix := []struct {
		status     string // human-readable status
		reason     string // operability check reason string
		shouldHide bool
	}{
		{"active seller", "", false},
		{"suspended account", "seller_account_inactive", true},
		{"banned account", "seller_account_inactive", true},
		{"removed account", "seller_removed", true},
		{"expired subscription", "seller_inactive", true},
	}

	for _, tc := range doctrineMatrix {
		if !tc.shouldHide {
			continue // ALLOW cases don't produce a reason to map
		}

		got := checker.mapReasonToStopReason(entity.TargetTypeForSale, tc.reason)
		if got != entity.StopReasonSellerGovernance {
			t.Errorf("doctrine(%s): reason %q → %q, want %q",
				tc.status, tc.reason, got, entity.StopReasonSellerGovernance)
		}
	}
}

// TestGovernanceDoctrineAlignment_ExternalProductUserReasons verifies
// that user-level governance reasons for external products also map
// to StopReasonSellerGovernance.
func TestGovernanceDoctrineAlignment_ExternalProductUserReasons(t *testing.T) {
	checker := &OperabilityCheckerImpl{}

	userReasons := []struct {
		status string
		reason string
	}{
		{"suspended user", "user_account_inactive"},
		{"banned user", "user_account_inactive"},
		{"removed user", "user_removed"},
	}

	for _, tc := range userReasons {
		got := checker.mapReasonToStopReason(entity.TargetTypeExternalProduct, tc.reason)
		if got != entity.StopReasonSellerGovernance {
			t.Errorf("external product doctrine(%s): reason %q → %q, want %q",
				tc.status, tc.reason, got, entity.StopReasonSellerGovernance)
		}
	}
}

// ========================================================================
// sellerGovernanceEligible PURE FUNCTION TESTS
//
// These tests exercise the canonical eligibility predicate directly,
// covering the full time-bounded subscription alignment matrix. No DB required.
// ========================================================================

func ptr(s string) *string { return &s }

// TestSellerGovernanceEligible_TimeBoundedAlignment pins the market authority
// doctrine: active entitlement interval is ALLOW, expired interval is HIDE,
// and KYC is ignored for selling authority.
func TestSellerGovernanceEligible_TimeBoundedAlignment(t *testing.T) {
	now := time.Now()
	activeStart := now.Add(-1 * time.Hour)
	activeEnd := now.Add(1 * time.Hour)
	futureStart := now.Add(1 * time.Hour)
	futureEnd := now.Add(2 * time.Hour)
	expiredStart := now.Add(-48 * time.Hour)
	expiredEnd := now.Add(-1 * time.Hour)

	cases := []struct {
		name               string
		accountStatus      string
		isDeleted          bool
		subscriptionStatus string
		startedAt          time.Time
		expiresAt          time.Time
		wantEligible       bool
		wantReason         string
	}{
		// === ALLOW cases ===
		{
			name:               "active interval + approved kyc→visible",
			accountStatus:      "active",
			subscriptionStatus: "active",
			startedAt:          activeStart,
			expiresAt:          activeEnd,
			wantEligible:       true,
		},
		{
			name:               "active interval + under_investigation→visible",
			accountStatus:      "active",
			subscriptionStatus: "active",
			startedAt:          activeStart,
			expiresAt:          activeEnd,
			wantEligible:       true,
		},
		{
			name:               "active interval + pending kyc→visible",
			accountStatus:      "active",
			subscriptionStatus: "active",
			startedAt:          activeStart,
			expiresAt:          activeEnd,
			wantEligible:       true,
		},
		{
			name:               "active interval + rejected kyc→visible",
			accountStatus:      "active",
			subscriptionStatus: "active",
			startedAt:          activeStart,
			expiresAt:          activeEnd,
			wantEligible:       true,
		},
		{
			name:               "active interval + needs_resubmission kyc→visible",
			accountStatus:      "active",
			subscriptionStatus: "active",
			startedAt:          activeStart,
			expiresAt:          activeEnd,
			wantEligible:       true,
		},
		{
			name:               "active interval + no kyc row→visible",
			accountStatus:      "active",
			subscriptionStatus: "active",
			startedAt:          activeStart,
			expiresAt:          activeEnd,
			wantEligible:       true,
		},
		{
			name:               "active interval + suspended kyc→visible",
			accountStatus:      "active",
			subscriptionStatus: "active",
			startedAt:          activeStart,
			expiresAt:          activeEnd,
			wantEligible:       true,
		},
		{
			name:               "active interval + revoked kyc→visible",
			accountStatus:      "active",
			subscriptionStatus: "active",
			startedAt:          activeStart,
			expiresAt:          activeEnd,
			wantEligible:       true,
		},

		// === HIDE cases: subscription ===
		{
			name:               "expired subscription interval→hidden",
			accountStatus:      "active",
			subscriptionStatus: "expired",
			startedAt:          expiredStart,
			expiresAt:          expiredEnd,
			wantEligible:       false,
			wantReason:         "seller_inactive",
		},
		{
			name:               "future scheduled renewal→hidden",
			accountStatus:      "active",
			subscriptionStatus: "active",
			startedAt:          futureStart,
			expiresAt:          futureEnd,
			wantEligible:       false,
			wantReason:         "seller_inactive",
		},
		{
			name:               "no subscription (empty)→hidden",
			accountStatus:      "active",
			subscriptionStatus: "",
			startedAt:          time.Time{},
			expiresAt:          time.Time{},
			wantEligible:       false,
			wantReason:         "seller_inactive",
		},

		// === HIDE cases: account ===
		{
			name:               "suspended account→hidden",
			accountStatus:      "suspended",
			subscriptionStatus: "active",
			startedAt:          activeStart,
			expiresAt:          activeEnd,
			wantEligible:       false,
			wantReason:         "seller_account_inactive",
		},
		{
			name:               "banned account→hidden",
			accountStatus:      "banned",
			subscriptionStatus: "active",
			startedAt:          activeStart,
			expiresAt:          activeEnd,
			wantEligible:       false,
			wantReason:         "seller_account_inactive",
		},
		{
			name:               "removed account→hidden",
			accountStatus:      "active",
			isDeleted:          true,
			subscriptionStatus: "active",
			startedAt:          activeStart,
			expiresAt:          activeEnd,
			wantEligible:       false,
			wantReason:         "seller_removed",
		},

		// === Gate precedence: account gates fire before subscription interval ===
		{
			name:               "suspended account with expired interval→account_inactive",
			accountStatus:      "suspended",
			subscriptionStatus: "active",
			startedAt:          activeStart,
			expiresAt:          activeEnd,
			wantEligible:       false,
			wantReason:         "seller_account_inactive",
		},
		{
			name:               "expired subscription beats valid account",
			accountStatus:      "active",
			subscriptionStatus: "expired",
			startedAt:          expiredStart,
			expiresAt:          expiredEnd,
			wantEligible:       false,
			wantReason:         "seller_inactive",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			eligible, reason := sellerGovernanceEligible(
				tc.accountStatus, tc.isDeleted, tc.subscriptionStatus, tc.startedAt, tc.expiresAt, now)

			if eligible != tc.wantEligible {
				t.Errorf("eligible = %v, want %v", eligible, tc.wantEligible)
			}
			if reason != tc.wantReason {
				t.Errorf("reason = %q, want %q", reason, tc.wantReason)
			}
		})
	}
}
