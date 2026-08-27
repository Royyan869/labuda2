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

func TestAuctionToDetailResponseWithSeller_EmitsCanonicalSellerIdentity(t *testing.T) {
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
		Username:         "  user_deadbeef  ",
		FarmName:         "  Acme Farm  ",
		StoreImageURL:    "  https://example.com/store.jpg  ",
		AvatarURL:        "  https://example.com/avatar.jpg  ",
		PublicOriginLine: "  Magelang, Jawa Tengah  ",
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

	sellerIdentity, ok := decoded["seller_identity"].(map[string]interface{})
	if !ok {
		t.Fatalf("seller_identity = %#v, want object", decoded["seller_identity"])
	}
	if sellerIdentity["store_name"] != "Acme Farm" {
		t.Fatalf("store_name = %v, want Acme Farm", sellerIdentity["store_name"])
	}
	if sellerIdentity["store_image_url"] != "https://example.com/store.jpg" {
		t.Fatalf(
			"store_image_url = %v, want https://example.com/store.jpg",
			sellerIdentity["store_image_url"],
		)
	}
	if sellerIdentity["username"] != "user_deadbeef" {
		t.Fatalf("username = %v, want user_deadbeef", sellerIdentity["username"])
	}
	if sellerIdentity["avatar_url"] != "https://example.com/avatar.jpg" {
		t.Fatalf(
			"avatar_url = %v, want https://example.com/avatar.jpg",
			sellerIdentity["avatar_url"],
		)
	}
	if sellerIdentity["public_origin_line"] != "Magelang, Jawa Tengah" {
		t.Fatalf(
			"public_origin_line = %v, want Magelang, Jawa Tengah",
			sellerIdentity["public_origin_line"],
		)
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
		ID:              uuid.New(),
		SellerID:        auction.SellerID,
		Title:           "Showa Koi 30cm",
		Description:     "Premium showa",
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
