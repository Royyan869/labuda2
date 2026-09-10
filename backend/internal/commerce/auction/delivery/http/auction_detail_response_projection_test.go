package http

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/auction/entity"
	productEntity "github.com/labuda/backend/internal/commerce/product/entity"
	commerceshared "github.com/labuda/backend/internal/commerce/shared"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/internal/pkg/sellerdisplay"
)

// TestAuctionToDetailResponseWithSeller_SellerIdentityAbsent locks the
// canonical detail contract: seller identity is carried by the flat
// seller_username / seller_farm_name / seller_avatar_url scalars, and the
// seller_identity block (duplicate + dead transport) MUST NOT be emitted.
func TestAuctionToDetailResponseWithSeller_SellerIdentityAbsent(t *testing.T) {
	auction := &entity.Auction{
		ID:           uuid.New(),
		SellerID:     uuid.New(),
		ProductID:    uuid.New(),
		StartPrice:   50000,
		BidIncrement: 5000,
		StartAt:      time.Now().UTC(),
		EndAt:        time.Now().UTC().Add(24 * time.Hour),
		Status:       entity.StatusActive,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	sellerCard := publiccard.SellerCard{
		User: publiccard.UserCard{
			ID:       auction.SellerID,
			Username: "user_deadbeef",
		},
	}
	sellerInfo := sellerdisplay.Info{
		Username:           "  user_deadbeef  ",
		FarmName:           "  Acme Farm  ",
		AvatarURL:          "  https://example.com/avatar.jpg  ",
		AccountStatus:      "active",
		SubscriptionStatus: "active",
		Tier:               "pro",
	}

	viewerID := auction.SellerID
	resp := auctionToDetailResponseWithSeller(auction, sellerCard, sellerInfo, nil, nil, &viewerID)
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if _, ok := decoded["seller_identity"]; ok {
		t.Fatalf("seller_identity unexpectedly present: %#v", decoded["seller_identity"])
	}

	// Canonical seller identity authority = flat scalars (raw passthrough
	// from sellerdisplay.Info — identical to the list/write surface).
	if decoded["seller_username"] != "  user_deadbeef  " {
		t.Fatalf("seller_username = %v, want raw user_deadbeef", decoded["seller_username"])
	}
	if decoded["seller_farm_name"] != "  Acme Farm  " {
		t.Fatalf("seller_farm_name = %v, want raw Acme Farm", decoded["seller_farm_name"])
	}
	if decoded["seller_avatar_url"] != "  https://example.com/avatar.jpg  " {
		t.Fatalf("seller_avatar_url = %v, want raw avatar url", decoded["seller_avatar_url"])
	}

	requireAuctionViewerCapabilitiesMap(t, decoded, commerceshared.EvaluateAuctionViewerCapabilities(commerceshared.AuctionViewerCapabilitiesInput{
		ViewerID:          viewerID,
		SellerID:          auction.SellerID,
		Status:            string(auction.Status),
		SellerTrustActive: false,
		BuyNowPrice:       auction.BuyNowPrice,
	}))
}

func TestAuctionToDetailResponseWithSeller_EmitsCanonicalProductFields(t *testing.T) {
	auction := &entity.Auction{
		ID:           uuid.New(),
		SellerID:     uuid.New(),
		ProductID:    uuid.New(),
		StartPrice:   50000,
		BidIncrement: 5000,
		StartAt:      time.Now().UTC(),
		EndAt:        time.Now().UTC().Add(24 * time.Hour),
		Status:       entity.StatusActive,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	sellerCard := publiccard.SellerCard{
		User: publiccard.UserCard{
			ID:       auction.SellerID,
			Username: "user_deadbeef",
		},
	}
	sellerInfo := sellerdisplay.Info{
		Username:           "seller_user",
		FarmName:           "Acme Farm",
		AccountStatus:      "active",
		IsDeleted:          false,
		SubscriptionStatus: "active",
		Tier:               "pro",
	}
	product := &productEntity.Product{
		ID:          uuid.New(),
		SellerID:    auction.SellerID,
		Title:       "Showa Koi 30cm",
		Description: "Premium showa",
		MediaURLs: []string{
			"https://cdn.example.com/koi-1.jpg",
			"https://cdn.example.com/koi-2.jpg",
		},
		Variety:         "Showa",
		SizeCm:          ptrInt(30),
		AgeMonths:       ptrInt(8),
		Gender:          ptrString("female"),
		Breeder:         ptrString("Acme Farm"),
		Bloodline:       ptrString("Ogata"),
		Certificates:    []string{"cert-a"},
		PreparationTime: "short",
		PreparationNote: ptrString("Pack carefully"),
	}

	resp := auctionToDetailResponseWithSeller(auction, sellerCard, sellerInfo, nil, product, nil)
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if _, ok := decoded["origin"]; ok {
		t.Fatalf("origin unexpectedly present: %#v", decoded["origin"])
	}
	if _, ok := decoded["shipping_options"]; ok {
		t.Fatalf("shipping_options unexpectedly present: %#v", decoded["shipping_options"])
	}
	if decoded["title"] != "Showa Koi 30cm" {
		t.Fatalf("title = %v, want Showa Koi 30cm", decoded["title"])
	}
	if decoded["description"] != "Premium showa" {
		t.Fatalf("description = %v, want Premium showa", decoded["description"])
	}
	if decoded["variety"] != "Showa" {
		t.Fatalf("variety = %v, want Showa", decoded["variety"])
	}
	if decoded["size_cm"] != float64(30) {
		t.Fatalf("size_cm = %v, want 30", decoded["size_cm"])
	}
	if decoded["age_months"] != float64(8) {
		t.Fatalf("age_months = %v, want 8", decoded["age_months"])
	}
	if decoded["preparation_time"] != "short" {
		t.Fatalf("preparation_time = %v, want short", decoded["preparation_time"])
	}
	if decoded["preparation_note"] != "Pack carefully" {
		t.Fatalf("preparation_note = %v, want Pack carefully", decoded["preparation_note"])
	}
	mediaURLs, ok := decoded["media_urls"].([]interface{})
	if !ok || len(mediaURLs) != 2 {
		t.Fatalf("media_urls = %#v, want 2 items", decoded["media_urls"])
	}
	if mediaURLs[0] != "https://cdn.example.com/koi-1.jpg" || mediaURLs[1] != "https://cdn.example.com/koi-2.jpg" {
		t.Fatalf("media_urls = %#v, want both CDN urls", mediaURLs)
	}
	if decoded["gender"] != "female" {
		t.Fatalf("gender = %v, want female", decoded["gender"])
	}
	if decoded["breeder"] != "Acme Farm" {
		t.Fatalf("breeder = %v, want Acme Farm", decoded["breeder"])
	}
	if decoded["bloodline"] != "Ogata" {
		t.Fatalf("bloodline = %v, want Ogata", decoded["bloodline"])
	}
	certificates, ok := decoded["certificates"].([]interface{})
	if !ok || len(certificates) != 1 || certificates[0] != "cert-a" {
		t.Fatalf("certificates = %#v, want [cert-a]", decoded["certificates"])
	}
}

func TestAuctionToDetailResponseWithSeller_MapsSharedViewerCapabilities(t *testing.T) {
	auction := &entity.Auction{
		ID:           uuid.New(),
		SellerID:     uuid.New(),
		ProductID:    uuid.New(),
		StartPrice:   50000,
		BidIncrement: 5000,
		StartAt:      time.Now().UTC(),
		EndAt:        time.Now().UTC().Add(24 * time.Hour),
		Status:       entity.StatusActive,
		BuyNowPrice:  ptrInt64(75000),
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	sellerCard := publiccard.SellerCard{
		Lifecycle: ptrString("active"),
		User: publiccard.UserCard{
			ID:       auction.SellerID,
			Username: "user_deadbeef",
		},
	}
	sellerInfo := sellerdisplay.Info{
		Username:           "seller_user",
		FarmName:           "Acme Farm",
		AccountStatus:      "active",
		IsDeleted:          false,
		SubscriptionStatus: "active",
		Tier:               "pro",
	}
	viewerID := uuid.New()
	resp := auctionToDetailResponseWithSeller(auction, sellerCard, sellerInfo, nil, nil, &viewerID)
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	requireAuctionViewerCapabilitiesMap(t, decoded, commerceshared.EvaluateAuctionViewerCapabilities(commerceshared.AuctionViewerCapabilitiesInput{
		ViewerID:          viewerID,
		SellerID:          auction.SellerID,
		Status:            string(auction.Status),
		SellerTrustActive: true,
		BuyNowPrice:       auction.BuyNowPrice,
	}))
}

func requireAuctionViewerCapabilitiesMap(t *testing.T, decoded map[string]interface{}, want commerceshared.ViewerCapabilities) {
	t.Helper()

	capabilities, ok := decoded["viewer_capabilities"].(map[string]interface{})
	if !ok {
		t.Fatalf("viewer_capabilities = %#v, want object", decoded["viewer_capabilities"])
	}
	if capabilities["role"] != want.Role {
		t.Fatalf("role = %v, want %s", capabilities["role"], want.Role)
	}
	if capabilities["can_manage"] != want.CanManage {
		t.Fatalf("can_manage = %v, want %v", capabilities["can_manage"], want.CanManage)
	}
	if capabilities["can_edit"] != want.CanEdit {
		t.Fatalf("can_edit = %v, want %v", capabilities["can_edit"], want.CanEdit)
	}
	if capabilities["can_promote"] != want.CanPromote {
		t.Fatalf("can_promote = %v, want %v", capabilities["can_promote"], want.CanPromote)
	}
	if capabilities["can_chat"] != want.CanChat {
		t.Fatalf("can_chat = %v, want %v", capabilities["can_chat"], want.CanChat)
	}
	if capabilities["can_negotiate"] != want.CanNegotiate {
		t.Fatalf("can_negotiate = %v, want %v", capabilities["can_negotiate"], want.CanNegotiate)
	}
	if capabilities["can_buy"] != want.CanBuy {
		t.Fatalf("can_buy = %v, want %v", capabilities["can_buy"], want.CanBuy)
	}
	if capabilities["can_bid"] != want.CanBid {
		t.Fatalf("can_bid = %v, want %v", capabilities["can_bid"], want.CanBid)
	}
	if capabilities["can_buy_now"] != want.CanBuyNow {
		t.Fatalf("can_buy_now = %v, want %v", capabilities["can_buy_now"], want.CanBuyNow)
	}
}

func ptrInt64(v int64) *int64 {
	return &v
}

func ptrInt(v int) *int {
	return &v
}

func ptrString(v string) *string {
	return &v
}
