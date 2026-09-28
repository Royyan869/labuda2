package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/auction/entity"
	productEntity "github.com/labuda/backend/internal/commerce/product/entity"
	commerceshared "github.com/labuda/backend/internal/commerce/shared"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/internal/pkg/sellerdisplay"
	"github.com/labuda/backend/internal/platform/mediaresolve"
	"github.com/labuda/backend/internal/platform/s3presign"
)

// CANONICAL TRUTH (owner-locked): the auction payload — list and detail —
// carries the same Product content block as for_sale. Discovery cards render
// media from `media_urls` / `media` on the SAME wire slot regardless of sale
// channel. A list payload without media is a contract violation, not a
// tolerated shape.
func TestAuctionListResponse_CarriesCanonicalProductContentBlock(t *testing.T) {
	mediaresolve.SetDefaultConfig(mediaresolve.Config{
		PresignCfg: s3presign.Config{
			Region:    "us-east-1",
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
			Bucket:    "labuda-uploads",
		},
		CDNBaseURL: "https://cdn.example.test",
		ReadTTL:    time.Minute,
	})

	now := time.Now().UTC()
	auction := &entity.Auction{
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
	product := &productEntity.Product{
		ID:          auction.ProductID,
		SellerID:    auction.SellerID,
		Title:       "Showa Koi 30cm",
		Description: "Premium showa",
		MediaURLs: []string{
			"https://labuda-uploads.s3.us-east-1.amazonaws.com/auctions/koi.jpg",
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
	seller := sellerdisplay.Info{
		Username:           "seller_user",
		FarmName:           "Acme Farm",
		AccountStatus:      "active",
		SubscriptionStatus: "active",
	}

	// LIST payload — the discovery surface that feeds marketplace cards.
	resp := auctionToResponseWithSeller(auction, product, seller, nil)

	for _, key := range commerceshared.ProductContentWireKeys {
		if _, ok := resp[key]; !ok {
			t.Fatalf("auction LIST payload missing canonical Product content key %q", key)
		}
	}

	mediaURLs, ok := resp["media_urls"].([]string)
	if !ok || len(mediaURLs) != 1 {
		t.Fatalf("media_urls = %#v, want 1 item", resp["media_urls"])
	}
	if mediaURLs[0] != "https://cdn.example.test/auctions/koi.jpg" {
		t.Fatalf("media_urls[0] = %q, want canonical CDN URL", mediaURLs[0])
	}

	media, ok := resp["media"].([]map[string]interface{})
	if !ok {
		t.Fatalf("media = %#v, want []map[string]interface{}", resp["media"])
	}
	if len(media) != 1 {
		t.Fatalf("media length = %d, want 1", len(media))
	}
	if media[0]["type"] != "image" {
		t.Fatalf("media[0].type = %v, want image", media[0]["type"])
	}
	if media[0]["url"] != mediaURLs[0] {
		t.Fatalf("media[0].url = %q, want %q (single read authority)", media[0]["url"], mediaURLs[0])
	}

	if resp["farm_address_id"] == nil {
		t.Fatalf("farm_address_id = nil, want the Product farm address pointer")
	}

	// viewer_capabilities is DETAIL-ONLY by contract (both channels).
	if _, ok := resp["viewer_capabilities"]; ok {
		t.Fatalf("LIST payload must not carry viewer_capabilities: %#v", resp["viewer_capabilities"])
	}

	detail := auctionToDetailResponseWithSeller(
		auction,
		publiccard.SellerCard{User: publiccard.UserCard{ID: auction.SellerID}},
		seller,
		product,
		nil,
	)
	for _, key := range commerceshared.ProductContentWireKeys {
		if _, ok := detail[key]; !ok {
			t.Fatalf("auction DETAIL payload missing canonical Product content key %q", key)
		}
	}
	if _, ok := detail["viewer_capabilities"]; !ok {
		t.Fatalf("DETAIL payload must carry viewer_capabilities")
	}
}
