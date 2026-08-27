package http

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/auction/entity"
	"github.com/labuda/backend/internal/pkg/sellerdisplay"
)

// Auction browse seller governance tests.
//
// These tests pin the contract that browse auctions (GET /api/v1/auctions)
// only returns auctions from active, non-deleted sellers. The SQL JOIN with
// users table enforces this at the query level. The response builder then
// emits the coarsened lifecycle on the SellerCard for any surviving results.
//
// This mirrors the for_sale browse pattern (for_sale_repository_impl.go):
//   - JOIN users u ON u.id = a.seller_id
//   - WHERE u.account_status = 'active' AND u.deleted_at IS NULL

func newBrowseAuction(t *testing.T) *entity.Auction {
	t.Helper()
	now := time.Now().UTC()
	return &entity.Auction{
		ID:           uuid.New(),
		SellerID:     uuid.New(),
		ProductID:    uuid.New(),
		StartPrice:   50000,
		BidIncrement: 5000,
		StartAt:      now,
		EndAt:        now.Add(24 * time.Hour),
		Status:       entity.StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// extractSellerCard digs into the auction card JSON to get the nested seller
// card. Path: resp["auction"]["seller"].
func extractSellerCard(t *testing.T, wire map[string]interface{}) map[string]interface{} {
	t.Helper()
	auctionCard, ok := wire["auction"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing auction card in response")
	}
	sellerCard, ok := auctionCard["seller"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing seller card in auction card")
	}
	return sellerCard
}

// extractUserLifecycle gets the user-identity lifecycle from the nested
// seller.user.lifecycle path.
func extractUserLifecycle(t *testing.T, sellerCard map[string]interface{}) interface{} {
	t.Helper()
	user, ok := sellerCard["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing user card in seller card")
	}
	return user["lifecycle"]
}

// TestBrowseAuction_ActiveSeller_Visible verifies that auctions from active
// sellers produce a valid response with lifecycle="active" on both axes.
func TestBrowseAuction_ActiveSeller_Visible(t *testing.T) {
	auction := newBrowseAuction(t)
	seller := sellerdisplay.Info{
		Username:           "active_seller",
		FarmName:           "Active Farm",
		AvatarURL:          "https://example.com/avatar.jpg",
		AccountStatus:      "active",
		IsDeleted:          false,
		SubscriptionStatus: "active",
	}

	resp := auctionToResponseWithSeller(auction, nil, seller)
	wire := marshalRoundtrip(t, resp)

	// Verify seller identity fields populated
	if wire["seller_username"] != "active_seller" {
		t.Errorf("seller_username = %v, want active_seller", wire["seller_username"])
	}
	if wire["seller_farm_name"] != "Active Farm" {
		t.Errorf("seller_farm_name = %v, want Active Farm", wire["seller_farm_name"])
	}

	// Verify lifecycle on the canonical auction card
	sellerCard := extractSellerCard(t, wire)
	userLifecycle := extractUserLifecycle(t, sellerCard)
	if userLifecycle != "active" {
		t.Errorf("seller.user.lifecycle = %v, want active", userLifecycle)
	}
	// Seller-trust axis (top-level seller.lifecycle)
	if sellerCard["lifecycle"] != "active" {
		t.Errorf("seller.lifecycle = %v, want active", sellerCard["lifecycle"])
	}
}

// TestBrowseAuction_SuspendedSeller_LifecycleUnavailable verifies that if a
// suspended seller's auction somehow survives the SQL filter (e.g. race with
// status change), the response still coarsens lifecycle to "unavailable".
//
// In production, the SQL JOIN prevents this from happening (suspended sellers
// are filtered at query time). This test validates defense-in-depth at the
// response builder layer.
func TestBrowseAuction_SuspendedSeller_LifecycleUnavailable(t *testing.T) {
	auction := newBrowseAuction(t)
	seller := sellerdisplay.Info{
		Username:           "suspended_seller",
		FarmName:           "Suspended Farm",
		AvatarURL:          "",
		AccountStatus:      "suspended",
		IsDeleted:          false,
		SubscriptionStatus: "active",
	}

	resp := auctionToResponseWithSeller(auction, nil, seller)
	wire := marshalRoundtrip(t, resp)

	sellerCard := extractSellerCard(t, wire)
	userLifecycle := extractUserLifecycle(t, sellerCard)
	if userLifecycle != "unavailable" {
		t.Errorf("seller.user.lifecycle = %v, want unavailable", userLifecycle)
	}
}

// TestBrowseAuction_BannedSeller_LifecycleUnavailable verifies banned sellers
// coarsen to "unavailable" on the user-identity axis.
func TestBrowseAuction_BannedSeller_LifecycleUnavailable(t *testing.T) {
	auction := newBrowseAuction(t)
	seller := sellerdisplay.Info{
		Username:           "banned_seller",
		FarmName:           "Banned Farm",
		AvatarURL:          "",
		AccountStatus:      "banned",
		IsDeleted:          false,
		SubscriptionStatus: "active",
	}

	resp := auctionToResponseWithSeller(auction, nil, seller)
	wire := marshalRoundtrip(t, resp)

	sellerCard := extractSellerCard(t, wire)
	userLifecycle := extractUserLifecycle(t, sellerCard)
	if userLifecycle != "unavailable" {
		t.Errorf("seller.user.lifecycle = %v, want unavailable", userLifecycle)
	}
}

// TestBrowseAuction_RemovedSeller_LifecycleRemoved verifies deleted sellers
// coarsen to "removed" on the user-identity axis.
func TestBrowseAuction_RemovedSeller_LifecycleRemoved(t *testing.T) {
	auction := newBrowseAuction(t)
	seller := sellerdisplay.Info{
		Username:           "removed_seller",
		FarmName:           "Removed Farm",
		AvatarURL:          "",
		AccountStatus:      "active",
		IsDeleted:          true,
		SubscriptionStatus: "active",
	}

	resp := auctionToResponseWithSeller(auction, nil, seller)
	wire := marshalRoundtrip(t, resp)

	sellerCard := extractSellerCard(t, wire)
	userLifecycle := extractUserLifecycle(t, sellerCard)
	if userLifecycle != "removed" {
		t.Errorf("seller.user.lifecycle = %v, want removed", userLifecycle)
	}
}

// TestBrowseAuction_ExpiredSeller_TrustLifecycleUnavailable verifies expired
// subscription coarsens to "unavailable" on the seller-trust axis.
func TestBrowseAuction_ExpiredSeller_TrustLifecycleUnavailable(t *testing.T) {
	auction := newBrowseAuction(t)
	seller := sellerdisplay.Info{
		Username:           "expired_seller",
		FarmName:           "Expired Farm",
		AvatarURL:          "",
		AccountStatus:      "active",
		IsDeleted:          false,
		SubscriptionStatus: "expired",
	}

	resp := auctionToResponseWithSeller(auction, nil, seller)
	wire := marshalRoundtrip(t, resp)

	sellerCard := extractSellerCard(t, wire)

	// User-identity axis remains active (account is fine)
	userLifecycle := extractUserLifecycle(t, sellerCard)
	if userLifecycle != "active" {
		t.Errorf("seller.user.lifecycle = %v, want active", userLifecycle)
	}
	// Seller-trust axis is unavailable (subscription expired)
	if sellerCard["lifecycle"] != "unavailable" {
		t.Errorf("seller.lifecycle = %v, want unavailable", sellerCard["lifecycle"])
	}
}

// TestBrowseAuction_EmptySeller_FailOpen verifies that when sellerdisplay
// returns a zero-value Info (e.g. batch fetch missed this seller_id), the
// lifecycle defaults to "active" for user-identity. This is the fail-open
// behavior that the SQL JOIN now backstops — in production, the JOIN
// prevents suspended/deleted sellers from reaching this path.
func TestBrowseAuction_EmptySeller_FailOpen(t *testing.T) {
	auction := newBrowseAuction(t)
	seller := sellerdisplay.Info{} // Zero value — simulates fetch miss

	resp := auctionToResponseWithSeller(auction, nil, seller)
	wire := marshalRoundtrip(t, resp)

	sellerCard := extractSellerCard(t, wire)
	userLifecycle := extractUserLifecycle(t, sellerCard)

	// Empty account_status + not deleted → CoarsenLifecycle returns "active"
	if userLifecycle != "active" {
		t.Errorf("seller.user.lifecycle = %v, want active (fail-open)", userLifecycle)
	}
	// Empty subscription → CoarsenSellerTrust returns "unavailable"
	if sellerCard["lifecycle"] != "unavailable" {
		t.Errorf("seller.lifecycle = %v, want unavailable", sellerCard["lifecycle"])
	}
}

// marshalRoundtrip serializes a response map to JSON and back, simulating
// what mobile receives on the wire.
func marshalRoundtrip(t *testing.T, resp map[string]interface{}) map[string]interface{} {
	t.Helper()
	buf, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(buf, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	return out
}
