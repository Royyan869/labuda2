package http

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/auction/entity"
	productEntity "github.com/labuda/backend/internal/commerce/product/entity"
	"github.com/labuda/backend/internal/pkg/sellerdisplay"
)

// SCOPE 3 — auction status boundary.
//
// The raw internal state machine value ("draft", "waiting_settlement", …)
// must never cross the public wire as `status`. The public wire vocabulary
// is Status.PublicPhase() — a closed set that excludes draft entirely. The
// exact internal state crosses the wire ONLY as `seller_status`, and only
// for the owning seller (owner workspace surfaces); every other viewer —
// including anonymous — reads null there.

func ownerAuction(status entity.Status) *entity.Auction {
	return &entity.Auction{
		ID:           uuid.New(),
		SellerID:     uuid.New(),
		ProductID:    uuid.New(),
		StartPrice:   50000,
		BidIncrement: 5000,
		StartAt:      time.Now().UTC(),
		EndAt:        time.Now().UTC().Add(24 * time.Hour),
		Status:       status,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		Product: &productEntity.Product{
			ID:        uuid.New(),
			SellerID:  uuid.New(),
			Title:     "Boundary Auction",
			MediaURLs: []string{"https://cdn.example.com/a.jpg"},
		},
	}
}

func sellerOf(a *entity.Auction) sellerdisplay.Info {
	return sellerdisplay.Info{
		Username:           "seller_user",
		AccountStatus:      "active",
		SubscriptionStatus: "active",
	}
}

func decodeAuction(t *testing.T, resp map[string]interface{}) map[string]interface{} {
	t.Helper()
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	return decoded
}

// TestPublicStatusVocabulary_StatusNeverCarriesRawInternalState locks the
// public `status` vocabulary across all six internal states, for the
// anonymous viewer. Draft MUST coarsen (defensively) and MUST NOT appear in
// the emitted value set.
func TestPublicStatusVocabulary_StatusNeverCarriesRawInternalState(t *testing.T) {
	cases := []struct {
		internal entity.Status
		wantPub  string
	}{
		{entity.StatusDraft, "cancelled"}, // conservative defensive mapping — never "draft"
		{entity.StatusScheduled, "scheduled"},
		{entity.StatusActive, "active"},
		{entity.StatusWaitingSettlement, "waiting_settlement"},
		{entity.StatusEnded, "ended"},
		{entity.StatusCancelled, "cancelled"},
	}
	for _, tc := range cases {
		a := ownerAuction(tc.internal)
		resp := auctionToResponseWithSeller(a, a.Product, sellerOf(a), nil)
		decoded := decodeAuction(t, resp)

		if decoded["status"] != tc.wantPub {
			t.Fatalf("internal=%s status = %v, want %s", tc.internal, decoded["status"], tc.wantPub)
		}
		if decoded["status"] == string(tc.internal) && tc.internal == entity.StatusDraft {
			t.Fatalf("raw internal draft leaked to public wire: %v", decoded["status"])
		}
		if decoded["seller_status"] != nil {
			t.Fatalf("internal=%s seller_status must be null for anonymous viewer, got %v", tc.internal, decoded["seller_status"])
		}
	}
}

// TestPublicStatusVocabulary_SellerStatusOwnerOnly locks the owner axis: the
// owning seller reads the exact internal state via `seller_status`; any other
// authenticated viewer reads null.
func TestPublicStatusVocabulary_SellerStatusOwnerOnly(t *testing.T) {
	a := ownerAuction(entity.StatusDraft)
	seller := sellerOf(a)

	// Owner sees the exact internal state (seller workspace correctness).
	ownerResp := auctionToResponseWithSeller(a, a.Product, seller, &a.SellerID)
	owner := decodeAuction(t, ownerResp)
	if owner["seller_status"] != "draft" {
		t.Fatalf("owner seller_status = %v, want draft", owner["seller_status"])
	}
	// The public `status` still coarsens even for the owner — one public
	// vocabulary for everyone, owner detail lives only in seller_status.
	if owner["status"] == "draft" {
		t.Fatalf("public status must never carry raw draft, even for owner")
	}

	// Another authenticated viewer does NOT see the internal state.
	other := uuid.New()
	otherResp := auctionToResponseWithSeller(a, a.Product, seller, &other)
	otherDecoded := decodeAuction(t, otherResp)
	if otherDecoded["seller_status"] != nil {
		t.Fatalf("non-owner seller_status = %v, want null", otherDecoded["seller_status"])
	}
}

// TestPublicStatusVocabulary_OwnerSurfacesCarrySellerStatus locks the owner
// write surfaces (create/update responses via auctionToResponse): they pass
// the caller's identity, so seller_status is always populated there.
func TestPublicStatusVocabulary_OwnerSurfacesCarrySellerStatus(t *testing.T) {
	a := ownerAuction(entity.StatusScheduled)
	resp := auctionToResponse(a, a.Product, a.SellerID)
	decoded := decodeAuction(t, resp)
	if decoded["seller_status"] != "scheduled" {
		t.Fatalf("owner write surface seller_status = %v, want scheduled", decoded["seller_status"])
	}
	if decoded["status"] != "scheduled" {
		t.Fatalf("public status = %v, want scheduled", decoded["status"])
	}
}
