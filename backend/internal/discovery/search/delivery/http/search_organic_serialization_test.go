package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/discovery/search/entity"
)

func TestForSalePreviewsToResponse_OmitsSellerName(t *testing.T) {
	forSales := []*entity.ForSalePreview{
		{
			ID:              uuid.New(),
			Title:           "Showa Koi 30cm",
			Description:     "Beautiful showa",
			Variety:         "Showa",
			Price:           1500000,
			MediaURLs:       []string{"https://example.com/forSale.jpg"},
			SellerID:        uuid.New(),
			SellerUsername:  "seller_user",
			SellerFarmName:  "Farm Name",
			SellerAvatarURL: "https://example.com/avatar.jpg",
			CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	resp := forSalePreviewsToResponse(forSales, map[uuid.UUID]string{})
	if len(resp) != 1 {
		t.Fatalf("expected 1 forSale row, got %d", len(resp))
	}

	row := resp[0]
	if _, ok := row["seller_name"]; ok {
		t.Fatalf("seller_name should not be serialized: %v", row["seller_name"])
	}
	if row["seller_username"] != "seller_user" {
		t.Fatalf("seller_username = %v, want seller_user", row["seller_username"])
	}
	if row["seller_farm_name"] != "Farm Name" {
		t.Fatalf("seller_farm_name = %v, want Farm Name", row["seller_farm_name"])
	}
}

func TestAuctionPreviewsToResponse_OmitsSellerName(t *testing.T) {
	auctions := []*entity.AuctionPreview{
		{
			ID:              uuid.New(),
			SellerID:        uuid.New(),
			ProductID:       uuid.New(),
			Title:           "Sanke Auction",
			Description:     "Rare sanke",
			StartPrice:      2500000,
			StartAt:         time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			EndAt:           time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
			Status:          "active",
			BidCount:        3,
			SellerUsername:  "auction_user",
			SellerFarmName:  "Auction Farm",
			SellerAvatarURL: "https://example.com/auction-avatar.jpg",
			CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	resp := auctionPreviewsToResponse(auctions, map[uuid.UUID]string{})
	if len(resp) != 1 {
		t.Fatalf("expected 1 auction row, got %d", len(resp))
	}

	row := resp[0]
	if _, ok := row["seller_name"]; ok {
		t.Fatalf("seller_name should not be serialized: %v", row["seller_name"])
	}
	if row["seller_username"] != "auction_user" {
		t.Fatalf("seller_username = %v, want auction_user", row["seller_username"])
	}
	if row["seller_farm_name"] != "Auction Farm" {
		t.Fatalf("seller_farm_name = %v, want Auction Farm", row["seller_farm_name"])
	}
}
