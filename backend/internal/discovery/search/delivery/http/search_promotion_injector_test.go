package http

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	promoentity "github.com/labuda/backend/internal/pricing/promotion/entity"
)

// ---------- helpers ----------

func makeSearchHydrated(n int) []searchHydratedPromotion {
	var items []searchHydratedPromotion
	for i := 0; i < n; i++ {
		tid := uuid.New()
		items = append(items, searchHydratedPromotion{
			Instance: &promoentity.PromotionInstance{
				ID:         uuid.New(),
				TargetType: promoentity.TargetTypeForSale,
				TargetID:   &tid,
			},
			SellerID: uuid.New(),
			Response: map[string]interface{}{"type": "promoted_for_sale", "index": i},
		})
	}
	return items
}

// ---------- GetPromotedSidecar nil-safety ----------

func TestSearchGetPromotedSidecar_NilInjector(t *testing.T) {
	var inj *SearchPromotionInjector
	result := inj.GetPromotedSidecar(nil, make([]uuid.UUID, 5), make([]uuid.UUID, 5))
	if result != nil {
		t.Fatalf("nil injector should return nil, got %d items", len(result))
	}
}

func TestSearchGetPromotedSidecar_TooFewOrganic(t *testing.T) {
	inj := &SearchPromotionInjector{}
	result := inj.GetPromotedSidecar(nil, make([]uuid.UUID, 2), make([]uuid.UUID, 2))
	if result != nil {
		t.Fatalf("should return nil when < %d organic items, got %d",
			searchMinOrganicForInjection, len(result))
	}
}

// ---------- searchApplySlotPolicy ----------

func TestSearchSlotPolicy_OrganicTargetDedup(t *testing.T) {
	targetID := uuid.New()
	items := []searchHydratedPromotion{
		{
			Instance: &promoentity.PromotionInstance{
				ID:         uuid.New(),
				TargetType: promoentity.TargetTypeForSale,
				TargetID:   &targetID,
			},
			SellerID: uuid.New(),
			Response: map[string]interface{}{"type": "promoted_for_sale"},
		},
	}

	// The promoted target is in organic results → should be skipped.
	organicIDs := map[uuid.UUID]bool{targetID: true}
	organicSellers := map[uuid.UUID]bool{}

	result := searchApplySlotPolicy(items, organicIDs, organicSellers)
	if len(result) != 0 {
		t.Fatalf("expected 0 after organic target dedup, got %d", len(result))
	}
}

func TestSearchSlotPolicy_OrganicSellerDedup(t *testing.T) {
	sellerID := uuid.New()
	tid := uuid.New()
	items := []searchHydratedPromotion{
		{
			Instance: &promoentity.PromotionInstance{
				ID:         uuid.New(),
				TargetType: promoentity.TargetTypeForSale,
				TargetID:   &tid,
			},
			SellerID: sellerID,
			Response: map[string]interface{}{"type": "promoted_for_sale"},
		},
	}

	// The promoted seller is in organic results → should be skipped.
	organicIDs := map[uuid.UUID]bool{}
	organicSellers := map[uuid.UUID]bool{sellerID: true}

	result := searchApplySlotPolicy(items, organicIDs, organicSellers)
	if len(result) != 0 {
		t.Fatalf("expected 0 after organic seller dedup, got %d", len(result))
	}
}

func TestSearchSlotPolicy_WithinPromotedTargetDedup(t *testing.T) {
	targetID := uuid.New()
	items := []searchHydratedPromotion{
		{
			Instance: &promoentity.PromotionInstance{
				ID:         uuid.New(),
				TargetType: promoentity.TargetTypeForSale,
				TargetID:   &targetID,
			},
			SellerID: uuid.New(),
			Response: map[string]interface{}{"type": "promoted_for_sale"},
		},
		{
			Instance: &promoentity.PromotionInstance{
				ID:         uuid.New(),
				TargetType: promoentity.TargetTypeForSale,
				TargetID:   &targetID, // same target
			},
			SellerID: uuid.New(),
			Response: map[string]interface{}{"type": "promoted_for_sale"},
		},
	}

	result := searchApplySlotPolicy(items, map[uuid.UUID]bool{}, map[uuid.UUID]bool{})
	if len(result) != 1 {
		t.Fatalf("expected 1 after within-promoted target dedup, got %d", len(result))
	}
}

func TestSearchSlotPolicy_MaxCap(t *testing.T) {
	items := makeSearchHydrated(5)
	result := searchApplySlotPolicy(items, map[uuid.UUID]bool{}, map[uuid.UUID]bool{})
	if len(result) != searchMaxPromotedPerPage {
		t.Fatalf("expected %d (cap), got %d", searchMaxPromotedPerPage, len(result))
	}
}

func TestSearchSlotPolicy_PassesNonConflicting(t *testing.T) {
	items := makeSearchHydrated(1)
	result := searchApplySlotPolicy(items, map[uuid.UUID]bool{}, map[uuid.UUID]bool{})
	if len(result) != 1 {
		t.Fatalf("expected 1 non-conflicting item, got %d", len(result))
	}
}

// ---------- inject_at field ----------

func TestSearchSidecar_InjectAtField(t *testing.T) {
	items := makeSearchHydrated(1)
	filtered := searchApplySlotPolicy(items, map[uuid.UUID]bool{}, map[uuid.UUID]bool{})
	if len(filtered) == 0 {
		t.Fatal("expected at least 1 item")
	}

	// Simulate what GetPromotedSidecar does.
	for i := range filtered {
		filtered[i].Response["inject_at"] = searchInjectAtIndex
	}

	injectAt, ok := filtered[0].Response["inject_at"].(int)
	if !ok {
		t.Fatal("inject_at should be an int")
	}
	if injectAt != searchInjectAtIndex {
		t.Errorf("expected inject_at=%d, got %d", searchInjectAtIndex, injectAt)
	}
}

func TestSearchBuildForSaleResponse_AddsSplitIdentity(t *testing.T) {
	inst := &promoentity.PromotionInstance{ID: uuid.New()}
	card := &searchForSaleCard{ID: uuid.New(), Title: "t", PricePerUnit: 1000, ImageURL: "https://example.com/a.jpg"}
	resp := searchBuildForSaleResponse(inst, card, "seller-user", "Farm Name", "active")

	if _, ok := resp["seller_name"]; ok {
		t.Fatalf("seller_name should not be present: %v", resp["seller_name"])
	}
	if resp["seller_username"] != "seller-user" {
		t.Fatalf("seller_username = %v, want seller-user", resp["seller_username"])
	}
	if resp["seller_farm_name"] != "Farm Name" {
		t.Fatalf("seller_farm_name = %v, want Farm Name", resp["seller_farm_name"])
	}
	if resp["seller_lifecycle"] != "active" {
		t.Fatalf("seller_lifecycle = %v, want active", resp["seller_lifecycle"])
	}
}

// ---------- searchExtractFirstMediaURL ----------

func TestSearchExtractFirstMediaURL_StringArray(t *testing.T) {
	raw := json.RawMessage(`["https://img.example.com/1.jpg", "https://img.example.com/2.jpg"]`)
	url := searchExtractFirstMediaURL(raw)
	if url != "https://img.example.com/1.jpg" {
		t.Errorf("expected first URL, got %q", url)
	}
}

func TestSearchExtractFirstMediaURL_ObjectArray(t *testing.T) {
	raw := json.RawMessage(`[{"url":"https://img.example.com/1.jpg","type":"image"}]`)
	url := searchExtractFirstMediaURL(raw)
	if url != "https://img.example.com/1.jpg" {
		t.Errorf("expected first URL, got %q", url)
	}
}

func TestSearchExtractFirstMediaURL_Empty(t *testing.T) {
	url := searchExtractFirstMediaURL(nil)
	if url != "" {
		t.Errorf("expected empty, got %q", url)
	}
	url = searchExtractFirstMediaURL(json.RawMessage("[]"))
	if url != "" {
		t.Errorf("expected empty for empty array, got %q", url)
	}
}

func TestHydrateSearchPromotedItems_ExcludesExternalProduct(t *testing.T) {
	inj := &SearchPromotionInjector{}
	instances := []*promoentity.PromotionInstance{
		{
			ID:         uuid.New(),
			TargetType: promoentity.TargetTypeExternalProduct,
			UserID:     uuid.New(),
		},
	}

	hydrated, err := inj.hydrateSearchPromotedItems(context.Background(), instances)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(hydrated) != 0 {
		t.Fatalf("expected external product to be excluded, got %d items", len(hydrated))
	}
}
