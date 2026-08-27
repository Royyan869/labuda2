package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/discovery/search/entity"
	"github.com/labuda/backend/internal/pkg/publiccard"
)

func TestSearchProjectionAdapter_ContentTypesAndAuthorParity(t *testing.T) {
	authorID := uuid.New()
	base := &entity.ContentPreview{
		ID:             uuid.New(),
		AuthorID:       authorID,
		Type:           "post",
		Caption:        "hello world",
		MediaURLs:      []string{"https://img.example/content.jpg"},
		CreatedAt:      time.Date(2026, time.June, 2, 10, 0, 0, 0, time.UTC),
		AuthorUsername: "alice",
	}

	tests := []struct {
		name      string
		preview   *entity.ContentPreview
		wantType  string
		wantTitle string
		wantKey   string
		wantShare bool
		wantThumb string
	}{
		{
			name:      "post",
			preview:   base,
			wantType:  "post",
			wantTitle: "hello world",
		},
		{
			name: "request",
			preview: &entity.ContentPreview{
				ID:             uuid.New(),
				AuthorID:       authorID,
				Type:           "request",
				Caption:        "need a fish",
				MediaURLs:      []string{},
				CreatedAt:      base.CreatedAt,
				AuthorUsername: "alice",
			},
			wantType:  "request",
			wantTitle: "need a fish",
		},
		{
			name: "repost",
			preview: &entity.ContentPreview{
				ID:             uuid.New(),
				AuthorID:       authorID,
				Type:           "repost",
				Caption:        "repost caption",
				MediaURLs:      []string{},
				CreatedAt:      base.CreatedAt,
				AuthorUsername: "alice",
			},
			wantType:  "repost",
			wantTitle: "repost caption",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items := contentPreviewsToResponse([]*entity.ContentPreview{tt.preview}, nil, map[uuid.UUID]string{authorID: "active"})
			if len(items) != 1 {
				t.Fatalf("expected 1 item; got %d", len(items))
			}

			item := items[0]
			if got := item["type"]; got != tt.wantType {
				t.Fatalf("type = %v; want %v", got, tt.wantType)
			}

			card, ok := item["card"].(publiccard.ContentCard)
			if !ok {
				t.Fatalf("card is %T; want publiccard.ContentCard", item["card"])
			}
			if card.Type != tt.wantType {
				t.Fatalf("card.Type = %v; want %v", card.Type, tt.wantType)
			}
			if tt.preview.Caption != "" {
				if card.Caption == nil {
					t.Fatalf("card.Caption = nil; want %q", tt.preview.Caption)
				}
				if *card.Caption != tt.preview.Caption {
					t.Fatalf("card.Caption = %q; want %q", *card.Caption, tt.preview.Caption)
				}
			}

			if tt.wantShare {
				t.Fatal("legacy projection expectations were removed")
			}
			for _, key := range []string{"for_sale", "auction", "profile"} {
				if _, ok := item[key]; ok {
					t.Fatalf("unexpected %q block on non-share row", key)
				}
			}
		})
	}
}

func TestSearchProjectionAdapter_ForSaleWireParity(t *testing.T) {
	sellerID := uuid.New()
	preview := &entity.ForSalePreview{
		ID:                       uuid.New(),
		Title:                    "Showa Koi 30cm",
		Description:              "Beautiful showa",
		Variety:                  "Showa",
		Price:                    1500000,
		MediaURLs:                []string{"https://example.com/fixed-price-sale.jpg"},
		SellerID:                 sellerID,
		CreatedAt:                time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		SellerUsername:           "seller_user",
		SellerFarmName:           "Farm Name",
		SellerAvatarURL:          "https://example.com/avatar.jpg",
		SellerAccountStatus:      "active",
		SellerIsDeleted:          false,
		SellerSubscriptionStatus: "active",
	}

	resp := forSalePreviewsToResponse([]*entity.ForSalePreview{preview}, map[uuid.UUID]string{sellerID: "active"})
	if len(resp) != 1 {
		t.Fatalf("expected 1 forSale row, got %d", len(resp))
	}

	row := resp[0]
	required := []string{"id", "title", "description", "variety", "price", "media_urls", "seller_id", "created_at", "seller_username", "seller_farm_name", "seller_avatar_url", "author", "media", "for_sale"}
	for _, key := range required {
		if _, ok := row[key]; !ok {
			t.Fatalf("missing required key %q", key)
		}
	}
	if _, ok := row["seller_name"]; ok {
		t.Fatalf("seller_name should not be serialized")
	}

	if row["seller_username"] != "seller_user" {
		t.Fatalf("seller_username = %v; want seller_user", row["seller_username"])
	}
	if row["seller_farm_name"] != "Farm Name" {
		t.Fatalf("seller_farm_name = %v; want Farm Name", row["seller_farm_name"])
	}

	card, ok := row["for_sale"].(publiccard.ForSaleCard)
	if !ok {
		t.Fatalf("for_sale is %T; want publiccard.ForSaleCard", row["for_sale"])
	}
	if card.Title != preview.Title {
		t.Fatalf("fixed-price-sale card title = %v; want %v", card.Title, preview.Title)
	}
	if card.Seller == nil {
		t.Fatal("fixed-price-sale card seller = nil; want seller card")
	}
	if card.Seller.User.Username != preview.SellerUsername {
		t.Fatalf("seller username = %v; want %v", card.Seller.User.Username, preview.SellerUsername)
	}
	if card.Seller.FarmName == nil || *card.Seller.FarmName != preview.SellerFarmName {
		t.Fatalf("seller farm = %v; want %v", card.Seller.FarmName, preview.SellerFarmName)
	}
	if card.Seller.User.Lifecycle == nil || *card.Seller.User.Lifecycle != "active" {
		t.Fatalf("seller user lifecycle = %v; want active", card.Seller.User.Lifecycle)
	}
	if card.Seller.Lifecycle == nil || *card.Seller.Lifecycle != "active" {
		t.Fatalf("seller trust lifecycle = %v; want active", card.Seller.Lifecycle)
	}
}

func TestSearchProjectionAdapter_AuctionWireParity(t *testing.T) {
	sellerID := uuid.New()
	thumbnail := "https://example.com/auction.jpg"
	currentBid := int64(3000000)
	buyNow := int64(5000000)
	preview := &entity.AuctionPreview{
		ID:                       uuid.New(),
		SellerID:                 sellerID,
		ProductID:                uuid.New(),
		Title:                    "Sanke Auction",
		Description:              "Rare sanke",
		StartPrice:               2500000,
		CurrentBid:               &currentBid,
		BuyNowPrice:              &buyNow,
		StartAt:                  time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		EndAt:                    time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC),
		Status:                   "active",
		ThumbnailURL:             &thumbnail,
		BidCount:                 3,
		CreatedAt:                time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		SellerUsername:           "auction_user",
		SellerFarmName:           "Auction Farm",
		SellerAvatarURL:          "https://example.com/auction-avatar.jpg",
		SellerAccountStatus:      "active",
		SellerIsDeleted:          false,
		SellerSubscriptionStatus: "active",
	}

	resp := auctionPreviewsToResponse([]*entity.AuctionPreview{preview}, map[uuid.UUID]string{sellerID: "active"})
	if len(resp) != 1 {
		t.Fatalf("expected 1 auction row, got %d", len(resp))
	}

	row := resp[0]
	required := []string{"id", "seller_id", "product_id", "title", "description", "start_price", "current_bid", "buy_now_price", "start_at", "end_at", "status", "thumbnail_url", "bid_count", "created_at", "seller_username", "seller_farm_name", "seller_avatar_url", "author", "media", "auction"}
	for _, key := range required {
		if _, ok := row[key]; !ok {
			t.Fatalf("missing required key %q", key)
		}
	}

	card, ok := row["auction"].(publiccard.AuctionCard)
	if !ok {
		t.Fatalf("auction is %T; want publiccard.AuctionCard", row["auction"])
	}
	if card.Title != preview.Title {
		t.Fatalf("auction card title = %v; want %v", card.Title, preview.Title)
	}
	if card.Seller == nil {
		t.Fatal("auction card seller = nil; want seller card")
	}
	if card.Seller.User.Username != preview.SellerUsername {
		t.Fatalf("seller username = %v; want %v", card.Seller.User.Username, preview.SellerUsername)
	}
	if card.Seller.FarmName == nil || *card.Seller.FarmName != preview.SellerFarmName {
		t.Fatalf("seller farm = %v; want %v", card.Seller.FarmName, preview.SellerFarmName)
	}
	if card.Seller.User.Lifecycle == nil || *card.Seller.User.Lifecycle != "active" {
		t.Fatalf("seller user lifecycle = %v; want active", card.Seller.User.Lifecycle)
	}
	if card.Seller.Lifecycle == nil || *card.Seller.Lifecycle != "active" {
		t.Fatalf("seller trust lifecycle = %v; want active", card.Seller.Lifecycle)
	}
}

func TestSearchProjectionAdapter_UserWireParity(t *testing.T) {
	avatar := "https://example.com/avatar.jpg"
	resp := userPreviewsToResponse([]*entity.UserPreview{{
		ID:        uuid.New(),
		Username:  "alice",
		AvatarURL: &avatar,
	}})
	if len(resp) != 1 {
		t.Fatalf("expected 1 user row, got %d", len(resp))
	}

	row := resp[0]
	if row["username"] != "alice" {
		t.Fatalf("username = %v; want alice", row["username"])
	}
	if row["avatar_url"] != &avatar {
		t.Fatalf("avatar_url = %v; want %v", row["avatar_url"], &avatar)
	}
	if _, ok := row["profile"]; ok {
		t.Fatalf("unexpected top-level profile field in user response")
	}
}

func TestSearchProjectionAdapter_UserFollowState_False(t *testing.T) {
	resp := userPreviewsToResponse([]*entity.UserPreview{{
		ID:                      uuid.New(),
		Username:                "bob",
		IsFollowedByCurrentUser: false,
	}})
	if len(resp) != 1 {
		t.Fatalf("expected 1 user row, got %d", len(resp))
	}
	val, ok := resp[0]["is_followed_by_current_user"]
	if !ok {
		t.Fatal("is_followed_by_current_user field missing from user response")
	}
	if val != false {
		t.Fatalf("is_followed_by_current_user = %v; want false", val)
	}
}

func TestSearchProjectionAdapter_UserFollowState_True(t *testing.T) {
	resp := userPreviewsToResponse([]*entity.UserPreview{{
		ID:                      uuid.New(),
		Username:                "alice",
		IsFollowedByCurrentUser: true,
	}})
	if len(resp) != 1 {
		t.Fatalf("expected 1 user row, got %d", len(resp))
	}
	val, ok := resp[0]["is_followed_by_current_user"]
	if !ok {
		t.Fatal("is_followed_by_current_user field missing from user response")
	}
	if val != true {
		t.Fatalf("is_followed_by_current_user = %v; want true", val)
	}
}

func TestSearchProjectionAdapter_UserFollowState_AlwaysEmitted(t *testing.T) {
	// Regression: field must always be present, never omitted.
	// Mobile falls back to false only when field absent — ensure backend emits it.
	resp := userPreviewsToResponse([]*entity.UserPreview{
		{ID: uuid.New(), Username: "u1", IsFollowedByCurrentUser: true},
		{ID: uuid.New(), Username: "u2", IsFollowedByCurrentUser: false},
	})
	if len(resp) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(resp))
	}
	for i, row := range resp {
		if _, ok := row["is_followed_by_current_user"]; !ok {
			t.Fatalf("row %d: is_followed_by_current_user field missing", i)
		}
	}
	if resp[0]["is_followed_by_current_user"] != true {
		t.Fatalf("row 0: is_followed_by_current_user = %v; want true", resp[0]["is_followed_by_current_user"])
	}
	if resp[1]["is_followed_by_current_user"] != false {
		t.Fatalf("row 1: is_followed_by_current_user = %v; want false", resp[1]["is_followed_by_current_user"])
	}
}

func TestSearchProjectionAdapter_PreservesInputOrder(t *testing.T) {
	first := &entity.ForSalePreview{
		ID:        uuid.New(),
		Title:     "First",
		Variety:   "A",
		Price:     1,
		SellerID:  uuid.New(),
		CreatedAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		MediaURLs: []string{},
	}
	second := &entity.ForSalePreview{
		ID:        uuid.New(),
		Title:     "Second",
		Variety:   "B",
		Price:     2,
		SellerID:  uuid.New(),
		CreatedAt: time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC),
		MediaURLs: []string{},
	}

	rows := forSalePreviewsToResponse([]*entity.ForSalePreview{first, second}, nil)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows; got %d", len(rows))
	}
	if rows[0]["id"] != first.ID.String() || rows[1]["id"] != second.ID.String() {
		t.Fatalf("input order changed: got %v then %v", rows[0]["id"], rows[1]["id"])
	}
}
