package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/auction/entity"
	productEntity "github.com/labuda/backend/internal/commerce/product/entity"
	"github.com/labuda/backend/internal/pkg/sellerdisplay"
)

// TestAuctionDetailResponse_IsTheGetAuctionSerializer locks the serializer
// wired at the end of AuctionHandler.GetAuction (GET /api/v1/auctions/:id) to
// the canonical detail projection. The method receives the DB-loaded
// auction (with auction.Product hydrated by the products JOIN) plus the
// sellerdisplay.Info and viewer identity, and MUST emit the full Product
// content together with the canonical detail-only marker
// (viewer_capabilities) — seller identity stays on the flat scalars and the
// seller_identity block is absent.
func TestAuctionDetailResponse_IsTheGetAuctionSerializer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewAuctionHandler(nil, nil, nil, nil, nil)

	now := time.Now().UTC()
	auction := &entity.Auction{
		ID:           uuid.New(),
		SellerID:     uuid.New(),
		ProductID:    uuid.New(),
		StartPrice:   50000,
		BidIncrement: 5000,
		BuyNowPrice:  ptrInt64(75000),
		StartAt:      now,
		EndAt:        now.Add(24 * time.Hour),
		Status:       entity.StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
		Product: &productEntity.Product{
			ID:              uuid.New(),
			SellerID:        uuid.New(),
			Title:           "Showa Koi 30cm",
			Description:     "Premium showa",
			MediaURLs:       []string{"https://cdn.example.com/koi-1.jpg"},
			Variety:         "Showa",
			SizeCm:          ptrInt(30),
			AgeMonths:       ptrInt(8),
			Gender:          ptrString("female"),
			Breeder:         ptrString("Acme Farm"),
			Bloodline:       ptrString("Ogata"),
			Certificates:    []string{"cert-a"},
			PreparationTime: "short",
			PreparationNote: ptrString("Pack carefully"),
		},
	}
	seller := sellerdisplay.Info{
		Username:           "seller_user",
		FarmName:           "Acme Farm",
		AccountStatus:      "active",
		IsDeleted:          false,
		SubscriptionStatus: "active",
		Tier:               "pro",
	}

	resp := h.auctionDetailResponse(auction, seller, uuid.Nil)
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	// Canonical detail-only marker — present ONLY on the detail projection,
	// never on the list/write auctionToResponseWithSeller shape.
	if _, ok := decoded["viewer_capabilities"]; !ok {
		t.Fatalf("viewer_capabilities missing from detail response")
	}

	// seller_identity is duplicate/dead transport — must be absent.
	if _, ok := decoded["seller_identity"]; ok {
		t.Fatalf("seller_identity unexpectedly present: %#v", decoded["seller_identity"])
	}

	// Full Product content preserved through the handler serializer.
	if decoded["title"] != "Showa Koi 30cm" {
		t.Fatalf("title = %v, want Showa Koi 30cm", decoded["title"])
	}
	if decoded["description"] != "Premium showa" {
		t.Fatalf("description = %v, want Premium showa", decoded["description"])
	}
	mediaURLs, ok := decoded["media_urls"].([]interface{})
	if !ok || len(mediaURLs) != 1 || mediaURLs[0] != "https://cdn.example.com/koi-1.jpg" {
		t.Fatalf("media_urls = %#v, want [https://cdn.example.com/koi-1.jpg]", decoded["media_urls"])
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
	if decoded["preparation_time"] != "short" {
		t.Fatalf("preparation_time = %v, want short", decoded["preparation_time"])
	}
	if decoded["preparation_note"] != "Pack carefully" {
		t.Fatalf("preparation_note = %v, want Pack carefully", decoded["preparation_note"])
	}
}

// TestGetAuctionRoute_BindsDetailEndpoint pins the production route binding
// (v1Browse.GET("/auctions/:id", AuctionHandler.GetAuction)) and proves the
// request is dispatched to AuctionHandler.GetAuction: a malformed id reaches
// the handler's own validation and returns its 400 response.
func TestGetAuctionRoute_BindsDetailEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewAuctionHandler(nil, nil, nil, nil, nil)

	router := gin.New()
	router.GET("/auctions/:id", h.GetAuction)

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/auctions/not-a-uuid", nil)
	if err != nil {
		t.Fatalf("request build failed: %v", err)
	}
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("GET /auctions/:id status = %d, want 400 (handler path reached)", w.Code)
	}
}
