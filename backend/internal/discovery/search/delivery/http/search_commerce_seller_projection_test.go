package http

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/discovery/search/entity"
	"github.com/labuda/backend/internal/pkg/publiccard"
)

func TestSearchCommerceSellerProjection_ActiveCanonicalSeller_ForSaleAndAuctionParity(t *testing.T) {
	sellerID := uuid.New()
	avatar := "  https://example.com/avatar.jpg  "
	farmName := "  Koi Farm  "

	forSale := &entity.ForSalePreview{
		ID:                       uuid.New(),
		Title:                    "Showa Koi 30cm",
		Description:              "Beautiful showa",
		Variety:                  "Showa",
		Price:                    1500000,
		MediaURLs:                []string{"https://example.com/forSale.jpg"},
		SellerID:                 sellerID,
		CreatedAt:                time.Date(2026, time.July, 24, 10, 0, 0, 0, time.UTC),
		SellerUsername:           "  Seller_User  ",
		SellerFarmName:           farmName,
		SellerAvatarURL:          avatar,
		SellerAccountStatus:      "active",
		SellerIsDeleted:          false,
		SellerSubscriptionStatus: "active",
	}
	auction := &entity.AuctionPreview{
		ID:                       uuid.New(),
		SellerID:                 sellerID,
		ProductID:                uuid.New(),
		Title:                    "Sanke Auction",
		Description:              "Rare sanke",
		StartPrice:               2500000,
		StartAt:                  time.Date(2026, time.July, 24, 11, 0, 0, 0, time.UTC),
		EndAt:                    time.Date(2026, time.July, 25, 11, 0, 0, 0, time.UTC),
		Status:                   "active",
		ThumbnailURL:             ptrString("https://example.com/auction.jpg"),
		BidCount:                 3,
		CreatedAt:                time.Date(2026, time.July, 24, 10, 5, 0, 0, time.UTC),
		SellerUsername:           "  Seller_User  ",
		SellerFarmName:           farmName,
		SellerAvatarURL:          avatar,
		SellerAccountStatus:      "active",
		SellerIsDeleted:          false,
		SellerSubscriptionStatus: "active",
	}

	forSaleRows := forSalePreviewsToResponse([]*entity.ForSalePreview{forSale}, nil)
	if len(forSaleRows) != 1 {
		t.Fatalf("forSale rows = %d; want 1", len(forSaleRows))
	}
	assertSearchCommerceSellerRow(t, forSaleRows[0], "for_sale", sellerID.String(), "seller_user", strings.TrimSpace(farmName), ptrString("https://example.com/avatar.jpg"), "active", "active")

	auctionRows := auctionPreviewsToResponse([]*entity.AuctionPreview{auction}, nil)
	if len(auctionRows) != 1 {
		t.Fatalf("auction rows = %d; want 1", len(auctionRows))
	}
	assertSearchCommerceSellerRow(t, auctionRows[0], "auction", sellerID.String(), "seller_user", strings.TrimSpace(farmName), ptrString("https://example.com/avatar.jpg"), "active", "active")
}

func TestSearchCommerceSellerProjection_BlankAndMissingUsernameRedactWithoutAnonymousFallback(t *testing.T) {
	sellerID := uuid.New()
	tests := []struct {
		name     string
		username string
		avatar   string
	}{
		{
			name:     "blank username with avatar",
			username: "   ",
			avatar:   "https://example.com/avatar.jpg",
		},
		{
			name:     "missing profile row",
			username: "",
			avatar:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			forSale := &entity.ForSalePreview{
				ID:                       uuid.New(),
				Title:                    "Showa Koi 30cm",
				Description:              "Beautiful showa",
				Variety:                  "Showa",
				Price:                    1500000,
				MediaURLs:                []string{},
				SellerID:                 sellerID,
				CreatedAt:                time.Date(2026, time.July, 24, 10, 0, 0, 0, time.UTC),
				SellerUsername:           tt.username,
				SellerFarmName:           "Farm Name",
				SellerAvatarURL:          tt.avatar,
				SellerAccountStatus:      "active",
				SellerIsDeleted:          false,
				SellerSubscriptionStatus: "active",
			}
			auction := &entity.AuctionPreview{
				ID:                       uuid.New(),
				SellerID:                 sellerID,
				ProductID:                uuid.New(),
				Title:                    "Sanke Auction",
				Description:              "Rare sanke",
				StartPrice:               2500000,
				StartAt:                  time.Date(2026, time.July, 24, 11, 0, 0, 0, time.UTC),
				EndAt:                    time.Date(2026, time.July, 25, 11, 0, 0, 0, time.UTC),
				Status:                   "active",
				ThumbnailURL:             nil,
				BidCount:                 3,
				CreatedAt:                time.Date(2026, time.July, 24, 10, 5, 0, 0, time.UTC),
				SellerUsername:           tt.username,
				SellerFarmName:           "Farm Name",
				SellerAvatarURL:          tt.avatar,
				SellerAccountStatus:      "active",
				SellerIsDeleted:          false,
				SellerSubscriptionStatus: "active",
			}

			forSaleRows := forSalePreviewsToResponse([]*entity.ForSalePreview{forSale}, nil)
			if len(forSaleRows) != 1 {
				t.Fatalf("forSale rows = %d; want 1", len(forSaleRows))
			}
			assertSearchCommerceSellerRow(t, forSaleRows[0], "for_sale", sellerID.String(), "", "Farm Name", nil, "unavailable", "active")

			auctionRows := auctionPreviewsToResponse([]*entity.AuctionPreview{auction}, nil)
			if len(auctionRows) != 1 {
				t.Fatalf("auction rows = %d; want 1", len(auctionRows))
			}
			assertSearchCommerceSellerRow(t, auctionRows[0], "auction", sellerID.String(), "", "Farm Name", nil, "unavailable", "active")
		})
	}
}

func TestSearchCommerceSellerProjection_SuspendedUserRedactsIdentity(t *testing.T) {
	sellerID := uuid.New()
	forSale := &entity.ForSalePreview{
		ID:                       uuid.New(),
		Title:                    "Showa Koi 30cm",
		Description:              "Beautiful showa",
		Variety:                  "Showa",
		Price:                    1500000,
		MediaURLs:                []string{},
		SellerID:                 sellerID,
		CreatedAt:                time.Date(2026, time.July, 24, 10, 0, 0, 0, time.UTC),
		SellerUsername:           "seller_user",
		SellerFarmName:           "Farm Name",
		SellerAvatarURL:          "https://example.com/avatar.jpg",
		SellerAccountStatus:      "suspended",
		SellerIsDeleted:          false,
		SellerSubscriptionStatus: "active",
	}
	auction := &entity.AuctionPreview{
		ID:                       uuid.New(),
		SellerID:                 sellerID,
		ProductID:                uuid.New(),
		Title:                    "Sanke Auction",
		Description:              "Rare sanke",
		StartPrice:               2500000,
		StartAt:                  time.Date(2026, time.July, 24, 11, 0, 0, 0, time.UTC),
		EndAt:                    time.Date(2026, time.July, 25, 11, 0, 0, 0, time.UTC),
		Status:                   "active",
		ThumbnailURL:             nil,
		BidCount:                 3,
		CreatedAt:                time.Date(2026, time.July, 24, 10, 5, 0, 0, time.UTC),
		SellerUsername:           "seller_user",
		SellerFarmName:           "Farm Name",
		SellerAvatarURL:          "https://example.com/avatar.jpg",
		SellerAccountStatus:      "suspended",
		SellerIsDeleted:          false,
		SellerSubscriptionStatus: "active",
	}

	forSaleRows := forSalePreviewsToResponse([]*entity.ForSalePreview{forSale}, nil)
	if len(forSaleRows) != 1 {
		t.Fatalf("forSale rows = %d; want 1", len(forSaleRows))
	}
	assertSearchCommerceSellerRow(t, forSaleRows[0], "for_sale", sellerID.String(), "", "Farm Name", nil, "unavailable", "active")

	auctionRows := auctionPreviewsToResponse([]*entity.AuctionPreview{auction}, nil)
	if len(auctionRows) != 1 {
		t.Fatalf("auction rows = %d; want 1", len(auctionRows))
	}
	assertSearchCommerceSellerRow(t, auctionRows[0], "auction", sellerID.String(), "", "Farm Name", nil, "unavailable", "active")
}

func TestSearchCommerceSellerProjection_MissingSellerProfileKeepsUserIdentityAndRedactsSellerLifecycle(t *testing.T) {
	sellerID := uuid.New()
	avatar := "https://example.com/avatar.jpg"
	forSale := &entity.ForSalePreview{
		ID:                       uuid.New(),
		Title:                    "Showa Koi 30cm",
		Description:              "Beautiful showa",
		Variety:                  "Showa",
		Price:                    1500000,
		MediaURLs:                []string{},
		SellerID:                 sellerID,
		CreatedAt:                time.Date(2026, time.July, 24, 10, 0, 0, 0, time.UTC),
		SellerUsername:           "seller_user",
		SellerFarmName:           "",
		SellerAvatarURL:          avatar,
		SellerAccountStatus:      "active",
		SellerIsDeleted:          false,
		SellerSubscriptionStatus: "active",
	}
	auction := &entity.AuctionPreview{
		ID:                       uuid.New(),
		SellerID:                 sellerID,
		ProductID:                uuid.New(),
		Title:                    "Sanke Auction",
		Description:              "Rare sanke",
		StartPrice:               2500000,
		StartAt:                  time.Date(2026, time.July, 24, 11, 0, 0, 0, time.UTC),
		EndAt:                    time.Date(2026, time.July, 25, 11, 0, 0, 0, time.UTC),
		Status:                   "active",
		ThumbnailURL:             nil,
		BidCount:                 3,
		CreatedAt:                time.Date(2026, time.July, 24, 10, 5, 0, 0, time.UTC),
		SellerUsername:           "seller_user",
		SellerFarmName:           "",
		SellerAvatarURL:          avatar,
		SellerAccountStatus:      "active",
		SellerIsDeleted:          false,
		SellerSubscriptionStatus: "active",
	}

	forSaleRows := forSalePreviewsToResponse([]*entity.ForSalePreview{forSale}, nil)
	if len(forSaleRows) != 1 {
		t.Fatalf("forSale rows = %d; want 1", len(forSaleRows))
	}
	assertSearchCommerceSellerRow(t, forSaleRows[0], "for_sale", sellerID.String(), "seller_user", "", ptrString(avatar), "active", "unavailable")

	auctionRows := auctionPreviewsToResponse([]*entity.AuctionPreview{auction}, nil)
	if len(auctionRows) != 1 {
		t.Fatalf("auction rows = %d; want 1", len(auctionRows))
	}
	assertSearchCommerceSellerRow(t, auctionRows[0], "auction", sellerID.String(), "seller_user", "", ptrString(avatar), "active", "unavailable")
}

func TestSearchCommerceSellerProjection_NilSellerIDFailsClosed(t *testing.T) {
	forSale := &entity.ForSalePreview{
		ID:                       uuid.New(),
		Title:                    "Showa Koi 30cm",
		Description:              "Beautiful showa",
		Variety:                  "Showa",
		Price:                    1500000,
		MediaURLs:                []string{},
		SellerID:                 uuid.Nil,
		CreatedAt:                time.Date(2026, time.July, 24, 10, 0, 0, 0, time.UTC),
		SellerUsername:           "seller_user",
		SellerFarmName:           "Farm Name",
		SellerAvatarURL:          "https://example.com/avatar.jpg",
		SellerAccountStatus:      "active",
		SellerIsDeleted:          false,
		SellerSubscriptionStatus: "active",
	}
	auction := &entity.AuctionPreview{
		ID:                       uuid.New(),
		SellerID:                 uuid.Nil,
		ProductID:                uuid.New(),
		Title:                    "Sanke Auction",
		Description:              "Rare sanke",
		StartPrice:               2500000,
		StartAt:                  time.Date(2026, time.July, 24, 11, 0, 0, 0, time.UTC),
		EndAt:                    time.Date(2026, time.July, 25, 11, 0, 0, 0, time.UTC),
		Status:                   "active",
		ThumbnailURL:             nil,
		BidCount:                 3,
		CreatedAt:                time.Date(2026, time.July, 24, 10, 5, 0, 0, time.UTC),
		SellerUsername:           "seller_user",
		SellerFarmName:           "Farm Name",
		SellerAvatarURL:          "https://example.com/avatar.jpg",
		SellerAccountStatus:      "active",
		SellerIsDeleted:          false,
		SellerSubscriptionStatus: "active",
	}

	if got := forSalePreviewsToResponse([]*entity.ForSalePreview{forSale}, nil); len(got) != 0 {
		t.Fatalf("forSale rows = %d; want 0", len(got))
	}
	if got := auctionPreviewsToResponse([]*entity.AuctionPreview{auction}, nil); len(got) != 0 {
		t.Fatalf("auction rows = %d; want 0", len(got))
	}
	if _, ok := buildSearchCommerceSellerProjection(uuid.Nil, "seller_user", "https://example.com/avatar.jpg", "Farm Name", "active", false, "active"); ok {
		t.Fatal("nil seller ID should fail closed")
	}
}

func TestSearchCommerceSellerProjection_RealStoredUserPrefixUsernamePreserved(t *testing.T) {
	sellerID := uuid.New()
	username := "user_deadbeef"
	avatar := "https://example.com/avatar.jpg"
	forSale := &entity.ForSalePreview{
		ID:                       uuid.New(),
		Title:                    "Showa Koi 30cm",
		Description:              "Beautiful showa",
		Variety:                  "Showa",
		Price:                    1500000,
		MediaURLs:                []string{},
		SellerID:                 sellerID,
		CreatedAt:                time.Date(2026, time.July, 24, 10, 0, 0, 0, time.UTC),
		SellerUsername:           username,
		SellerFarmName:           "Farm Name",
		SellerAvatarURL:          avatar,
		SellerAccountStatus:      "active",
		SellerIsDeleted:          false,
		SellerSubscriptionStatus: "active",
	}

	rows := forSalePreviewsToResponse([]*entity.ForSalePreview{forSale}, nil)
	if len(rows) != 1 {
		t.Fatalf("forSale rows = %d; want 1", len(rows))
	}
	assertSearchCommerceSellerRow(t, rows[0], "for_sale", sellerID.String(), username, "Farm Name", ptrString(avatar), "active", "active")

	auction := &entity.AuctionPreview{
		ID:                       uuid.New(),
		SellerID:                 sellerID,
		ProductID:                uuid.New(),
		Title:                    "Sanke Auction",
		Description:              "Rare sanke",
		StartPrice:               2500000,
		StartAt:                  time.Date(2026, time.July, 24, 11, 0, 0, 0, time.UTC),
		EndAt:                    time.Date(2026, time.July, 25, 11, 0, 0, 0, time.UTC),
		Status:                   "active",
		ThumbnailURL:             nil,
		BidCount:                 3,
		CreatedAt:                time.Date(2026, time.July, 24, 10, 5, 0, 0, time.UTC),
		SellerUsername:           username,
		SellerFarmName:           "Farm Name",
		SellerAvatarURL:          avatar,
		SellerAccountStatus:      "active",
		SellerIsDeleted:          false,
		SellerSubscriptionStatus: "active",
	}
	auctionRows := auctionPreviewsToResponse([]*entity.AuctionPreview{auction}, nil)
	if len(auctionRows) != 1 {
		t.Fatalf("auction rows = %d; want 1", len(auctionRows))
	}
	assertSearchCommerceSellerRow(t, auctionRows[0], "auction", sellerID.String(), username, "Farm Name", ptrString(avatar), "active", "active")
}

func assertSearchCommerceSellerRow(
	t *testing.T,
	row map[string]interface{},
	cardKey string,
	wantSellerID string,
	wantUsername string,
	wantFarmName string,
	wantAvatar *string,
	wantUserLifecycle string,
	wantSellerLifecycle string,
) {
	t.Helper()

	if got := row["seller_id"]; got != wantSellerID {
		t.Fatalf("seller_id = %v; want %v", got, wantSellerID)
	}
	if got := row["seller_username"]; got != wantUsername {
		t.Fatalf("seller_username = %v; want %v", got, wantUsername)
	}
	if got := row["seller_farm_name"]; got != wantFarmName {
		t.Fatalf("seller_farm_name = %v; want %v", got, wantFarmName)
	}
	if !reflect.DeepEqual(row["seller_avatar_url"], wantAvatar) {
		t.Fatalf("seller_avatar_url = %#v; want %#v", row["seller_avatar_url"], wantAvatar)
	}
	if !reflect.DeepEqual(row["seller_lifecycle"], ptrString(wantSellerLifecycle)) {
		t.Fatalf("seller_lifecycle = %#v; want %#v", row["seller_lifecycle"], ptrString(wantSellerLifecycle))
	}

	author, ok := row["author"].(publiccard.UserCard)
	if !ok {
		t.Fatalf("author is %T; want publiccard.UserCard", row["author"])
	}
	if author.Username != wantUsername {
		t.Fatalf("author.Username = %q; want %q", author.Username, wantUsername)
	}
	if !reflect.DeepEqual(author.AvatarURL, wantAvatar) {
		t.Fatalf("author.AvatarURL = %#v; want %#v", author.AvatarURL, wantAvatar)
	}
	if !reflect.DeepEqual(author.Lifecycle, ptrString(wantUserLifecycle)) {
		t.Fatalf("author.Lifecycle = %#v; want %#v", author.Lifecycle, ptrString(wantUserLifecycle))
	}

	var seller *publiccard.SellerCard
	switch cardKey {
	case "for_sale":
		card, ok := row[cardKey].(publiccard.ForSaleCard)
		if !ok {
			t.Fatalf("%s is %T; want publiccard.ForSaleCard", cardKey, row[cardKey])
		}
		seller = card.Seller
	case "auction":
		card, ok := row[cardKey].(publiccard.AuctionCard)
		if !ok {
			t.Fatalf("%s is %T; want publiccard.AuctionCard", cardKey, row[cardKey])
		}
		seller = card.Seller
	default:
		t.Fatalf("unsupported card key %q", cardKey)
	}

	if seller == nil {
		t.Fatal("seller card = nil; want seller card")
	}
	if seller.User.Username != wantUsername {
		t.Fatalf("seller.User.Username = %q; want %q", seller.User.Username, wantUsername)
	}
	if !reflect.DeepEqual(seller.User.AvatarURL, wantAvatar) {
		t.Fatalf("seller.User.AvatarURL = %#v; want %#v", seller.User.AvatarURL, wantAvatar)
	}
	if !reflect.DeepEqual(seller.User.Lifecycle, ptrString(wantUserLifecycle)) {
		t.Fatalf("seller.User.Lifecycle = %#v; want %#v", seller.User.Lifecycle, ptrString(wantUserLifecycle))
	}
	if got := sellerFarmNameString(seller); got != wantFarmName {
		t.Fatalf("seller.FarmName = %q; want %q", got, wantFarmName)
	}
	if !reflect.DeepEqual(seller.Lifecycle, ptrString(wantSellerLifecycle)) {
		t.Fatalf("seller.Lifecycle = %#v; want %#v", seller.Lifecycle, ptrString(wantSellerLifecycle))
	}
	if !reflect.DeepEqual(row["seller_username"], seller.User.Username) {
		t.Fatalf("flat seller_username and nested seller.user.username diverged: %v vs %v", row["seller_username"], seller.User.Username)
	}
	if !reflect.DeepEqual(row["seller_avatar_url"], seller.User.AvatarURL) {
		t.Fatalf("flat seller_avatar_url and nested seller.user.avatar_url diverged: %#v vs %#v", row["seller_avatar_url"], seller.User.AvatarURL)
	}
	if !reflect.DeepEqual(row["seller_lifecycle"], seller.Lifecycle) {
		t.Fatalf("flat seller_lifecycle and nested seller.lifecycle diverged: %#v vs %#v", row["seller_lifecycle"], seller.Lifecycle)
	}
}

func sellerFarmNameString(s *publiccard.SellerCard) string {
	if s == nil || s.FarmName == nil {
		return ""
	}
	return *s.FarmName
}

func ptrString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
