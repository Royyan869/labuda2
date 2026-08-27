package http

import (
	"context"
	"testing"

	"github.com/google/uuid"
	promoentity "github.com/labuda/backend/internal/pricing/promotion/entity"
)

// ---------- interleavePromotions tests ----------

func makeOrganic(n int) []map[string]interface{} {
	items := make([]map[string]interface{}, n)
	for i := range items {
		items[i] = map[string]interface{}{"type": "post", "index": i}
	}
	return items
}

func makeHydrated(n int) []hydratedPromotion {
	var items []hydratedPromotion
	for i := 0; i < n; i++ {
		tid := uuid.New()
		items = append(items, hydratedPromotion{
			Instance: &promoentity.PromotionInstance{
				ID:         uuid.New(),
				TargetType: promoentity.TargetTypeForSale,
				TargetID:   &tid,
			},
			SellerID: uuid.New(),
			Response: map[string]interface{}{"type": "promoted_for_sale", "promo_index": i},
		})
	}
	return items
}

func TestInterleavePromotions_SinglePromo(t *testing.T) {
	organic := makeOrganic(6)
	promoted := makeHydrated(1)

	result := interleavePromotions(organic, promoted)

	// Expect promo at index 2 (before organic[2])
	if len(result) != 7 {
		t.Fatalf("expected 7 items, got %d", len(result))
	}
	if result[2]["type"] != "promoted_for_sale" {
		t.Errorf("expected promoted_for_sale at index 2, got %v", result[2]["type"])
	}
	// Organic items should be in order around the promo
	if result[0]["index"] != 0 || result[1]["index"] != 1 {
		t.Error("organic items before promo slot are wrong")
	}
	if result[3]["index"] != 2 || result[4]["index"] != 3 {
		t.Error("organic items after promo slot are wrong")
	}
}

func TestInterleavePromotions_TwoPromos(t *testing.T) {
	organic := makeOrganic(10)
	promoted := makeHydrated(2)

	result := interleavePromotions(organic, promoted)

	if len(result) != 12 {
		t.Fatalf("expected 12 items, got %d", len(result))
	}
	// First promo at index 2
	if result[2]["type"] != "promoted_for_sale" {
		t.Errorf("expected promoted_for_sale at index 2, got %v", result[2]["type"])
	}
	// Second promo at index 6 (organic[5] is at output position 6 due to first insertion)
	if result[6]["type"] != "promoted_for_sale" {
		t.Errorf("expected promoted_for_sale at index 6, got %v", result[6]["type"])
	}
}

func TestInterleavePromotions_EmptyPromoted(t *testing.T) {
	organic := makeOrganic(5)
	result := interleavePromotions(organic, nil)
	if len(result) != 5 {
		t.Fatalf("expected 5 items, got %d", len(result))
	}
}

func TestInterleavePromotions_ShortOrganic(t *testing.T) {
	// Organic has 3 items, promo should be inserted at position 2
	organic := makeOrganic(3)
	promoted := makeHydrated(2)

	result := interleavePromotions(organic, promoted)

	// First promo at index 2, second appended at end (organic too short for slot 5)
	if len(result) != 5 {
		t.Fatalf("expected 5 items, got %d", len(result))
	}
	if result[2]["type"] != "promoted_for_sale" {
		t.Errorf("expected promoted_for_sale at index 2, got %v", result[2]["type"])
	}
	// Second promo appended after organic ends
	if result[4]["type"] != "promoted_for_sale" {
		t.Errorf("expected promoted_for_sale at index 4, got %v", result[4]["type"])
	}
}

// ---------- applySlotPolicy tests ----------

func TestApplySlotPolicy_DedupTarget(t *testing.T) {
	targetID := uuid.New()
	items := []hydratedPromotion{
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

	result := applySlotPolicy(items)
	if len(result) != 1 {
		t.Fatalf("expected 1 after target dedup, got %d", len(result))
	}
}

func TestApplySlotPolicy_DedupSeller(t *testing.T) {
	sellerID := uuid.New()
	t1 := uuid.New()
	t2 := uuid.New()
	items := []hydratedPromotion{
		{
			Instance: &promoentity.PromotionInstance{
				ID:         uuid.New(),
				TargetType: promoentity.TargetTypeForSale,
				TargetID:   &t1,
			},
			SellerID: sellerID,
			Response: map[string]interface{}{"type": "promoted_for_sale"},
		},
		{
			Instance: &promoentity.PromotionInstance{
				ID:         uuid.New(),
				TargetType: promoentity.TargetTypeAuction,
				TargetID:   &t2,
			},
			SellerID: sellerID, // same seller
			Response: map[string]interface{}{"type": "promoted_auction"},
		},
	}

	result := applySlotPolicy(items)
	if len(result) != 1 {
		t.Fatalf("expected 1 after seller dedup, got %d", len(result))
	}
}

func TestApplySlotPolicy_MaxCap(t *testing.T) {
	items := makeHydrated(5)
	result := applySlotPolicy(items)
	if len(result) != maxPromotedPerPage {
		t.Fatalf("expected %d (cap), got %d", maxPromotedPerPage, len(result))
	}
}

// ---------- InjectPromotions nil-safety tests ----------

func TestInjectPromotions_NilInjector(t *testing.T) {
	var inj *FeedPromotionInjector
	organic := makeOrganic(5)
	result := inj.InjectPromotions(nil, organic)
	if len(result) != 5 {
		t.Fatalf("nil injector should return organic unchanged, got %d", len(result))
	}
}

func TestInjectPromotions_TooFewOrganic(t *testing.T) {
	inj := &FeedPromotionInjector{}
	organic := makeOrganic(2)
	result := inj.InjectPromotions(nil, organic)
	if len(result) != 2 {
		t.Fatalf("should return organic unchanged when < %d items, got %d",
			minOrganicForInjection, len(result))
	}
}

func TestBuildPromotedForSaleResponse_AddsSplitIdentity(t *testing.T) {
	inst := &promoentity.PromotionInstance{ID: uuid.New()}
	card := &forSaleCardData{ID: uuid.New(), Title: "t", PricePerUnit: 1000, ImageURL: "https://example.com/a.jpg"}
	resp := buildPromotedForSaleResponse(inst, card, "seller-user", "Farm Name", "active")

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

// ---------- extractFirstMediaURL tests ----------

func TestExtractFirstMediaURL_StringArray(t *testing.T) {
	raw := []byte(`["https://img.example.com/1.jpg", "https://img.example.com/2.jpg"]`)
	url := extractFirstMediaURL(raw)
	if url != "https://img.example.com/1.jpg" {
		t.Errorf("expected first URL, got %q", url)
	}
}

func TestExtractFirstMediaURL_ObjectArray(t *testing.T) {
	raw := []byte(`[{"url":"https://img.example.com/1.jpg","type":"image"}]`)
	url := extractFirstMediaURL(raw)
	if url != "https://img.example.com/1.jpg" {
		t.Errorf("expected first URL, got %q", url)
	}
}

func TestExtractFirstMediaURL_Empty(t *testing.T) {
	url := extractFirstMediaURL(nil)
	if url != "" {
		t.Errorf("expected empty, got %q", url)
	}
	url = extractFirstMediaURL([]byte("[]"))
	if url != "" {
		t.Errorf("expected empty for empty array, got %q", url)
	}
}

func TestHydratePromotedItems_ExcludesExternalProduct(t *testing.T) {
	inj := &FeedPromotionInjector{}
	instances := []*promoentity.PromotionInstance{
		{
			ID:         uuid.New(),
			TargetType: promoentity.TargetTypeExternalProduct,
			UserID:     uuid.New(),
		},
	}

	hydrated, err := inj.hydratePromotedItems(context.Background(), instances)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(hydrated) != 0 {
		t.Fatalf("expected external product to be excluded, got %d items", len(hydrated))
	}
}
