package http

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/forsale/entity"
	"github.com/labuda/backend/internal/pkg/sellerdisplay"
)

// SCOPE 3 — for_sale status boundary.
//
// The raw internal state machine value ("draft", "sold", "withdrawn", …)
// must never cross the public wire as `status`. The public wire vocabulary
// is Status.PublicLifecycle() — {active, unavailable; draft coarsens
// defensively to "unavailable"}. The exact internal state crosses the wire
// ONLY as `seller_status`, and only for the owning seller; every other
// viewer — including anonymous — reads null there. Internal transition
// timestamps (sold_at / withdrawn_at) are likewise owner-scoped.

func boundarySellerInfo() sellerdisplay.Info {
	return sellerdisplay.Info{
		Username:           "seller_user",
		AccountStatus:      "active",
		SubscriptionStatus: "active",
	}
}

func decodeForSaleBoundary(t *testing.T, resp map[string]interface{}) map[string]interface{} {
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

// TestForSalePublicStatusVocabulary_StatusNeverCarriesRawInternalState locks
// the public `status` vocabulary across all four internal states, for the
// anonymous viewer. Draft MUST coarsen (defensively) and MUST NOT appear in
// the emitted value set.
func TestForSalePublicStatusVocabulary_StatusNeverCarriesRawInternalState(t *testing.T) {
	cases := []struct {
		internal entity.ForSaleStatus
		wantPub  string
	}{
		{entity.ForSaleStatusDraft, "unavailable"}, // conservative defensive mapping — never "draft"
		{entity.ForSaleStatusActive, "active"},
		{entity.ForSaleStatusSold, "unavailable"},
		{entity.ForSaleStatusWithdrawn, "unavailable"},
	}

	for _, tc := range cases {
		l := testForSale(uuid.New())
		l.Status = tc.internal
		resp := decodeForSaleBoundary(t, for_saleToResponseWithSeller(l, boundarySellerInfo(), nil))

		if got := resp["status"]; got != tc.wantPub {
			t.Errorf("internal=%q: status = %v, want %q", tc.internal, got, tc.wantPub)
		}
		if got := resp["lifecycle"]; got != tc.wantPub {
			t.Errorf("internal=%q: lifecycle = %v, want %q", tc.internal, got, tc.wantPub)
		}
		if got := resp["seller_status"]; got != nil {
			t.Errorf("internal=%q: anonymous seller_status = %v, want nil", tc.internal, got)
		}
	}
}

// TestForSaleSellerStatusOwnerOnly locks the owner-only gate: the owning
// seller reads the exact internal state via `seller_status`; every other
// viewer (other users, anonymous) reads null.
func TestForSaleSellerStatusOwnerOnly(t *testing.T) {
	sellerID := uuid.New()
	otherID := uuid.New()
	l := testForSale(sellerID)
	l.Status = entity.ForSaleStatusSold

	if got := sellerStatusForViewer(l, &sellerID); got == nil || *got != "sold" {
		t.Errorf("owner seller_status = %v, want \"sold\"", got)
	}
	if got := sellerStatusForViewer(l, &otherID); got != nil {
		t.Errorf("non-owner seller_status = %v, want nil", got)
	}
	if got := sellerStatusForViewer(l, nil); got != nil {
		t.Errorf("anonymous seller_status = %v, want nil", got)
	}
	if got := sellerStatusForViewer(l, &uuid.Nil); got != nil {
		t.Errorf("uuid.Nil seller_status = %v, want nil", got)
	}

	// Wire-level: owner sees raw state in seller_status while public status
	// stays coarsened.
	resp := decodeForSaleBoundary(t, for_saleToResponseWithSeller(l, boundarySellerInfo(), &sellerID))
	if got := resp["status"]; got != "unavailable" {
		t.Errorf("owner public status = %v, want coarsened \"unavailable\"", got)
	}
	if got := resp["seller_status"]; got != "sold" {
		t.Errorf("owner seller_status = %v, want \"sold\"", got)
	}

	// Wire-level: a non-owner viewer gets neither.
	respOther := decodeForSaleBoundary(t, for_saleToResponseWithSeller(l, boundarySellerInfo(), &otherID))
	if got := respOther["seller_status"]; got != nil {
		t.Errorf("non-owner seller_status = %v, want nil", got)
	}
}

// TestForSaleInternalTimestampsOwnerOnly locks sold_at/withdrawn_at gating:
// the owning seller reads the exact transition timestamps; every other
// viewer reads null.
func TestForSaleInternalTimestampsOwnerOnly(t *testing.T) {
	sellerID := uuid.New()
	otherID := uuid.New()
	now := time.Now().UTC()
	l := testForSale(sellerID)
	l.Status = entity.ForSaleStatusSold
	l.SoldAt = &now

	ownerResp := decodeForSaleBoundary(t, for_saleToResponseWithSeller(l, boundarySellerInfo(), &sellerID))
	if got := ownerResp["sold_at"]; got == nil {
		t.Errorf("owner sold_at = nil, want the timestamp")
	}

	otherResp := decodeForSaleBoundary(t, for_saleToResponseWithSeller(l, boundarySellerInfo(), &otherID))
	if got := otherResp["sold_at"]; got != nil {
		t.Errorf("non-owner sold_at = %v, want nil", got)
	}

	anonResp := decodeForSaleBoundary(t, for_saleToResponseWithSeller(l, boundarySellerInfo(), nil))
	if got := anonResp["sold_at"]; got != nil {
		t.Errorf("anonymous sold_at = %v, want nil", got)
	}
	if got := anonResp["withdrawn_at"]; got != nil {
		t.Errorf("anonymous withdrawn_at = %v, want nil", got)
	}
}
