package application

import (
	"testing"
	"time"

	"github.com/google/uuid"
	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	forsaleEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	productEntity "github.com/labuda/backend/internal/commerce/product/entity"
	shippingQuoteEntity "github.com/labuda/backend/internal/commerce/shipping/quote/entity"
	chatvalidator "github.com/labuda/backend/internal/interaction/chat/attachmentvalidator"
	"github.com/labuda/backend/pkg/money"
)

func TestBuildShippingQuoteAttachmentJSON_Canonical(t *testing.T) {
	productID := uuid.New()
	sourceID := uuid.New()
	quote := shippingQuoteEntity.NewShippingQuote(
		uuid.New(),
		productID,
		"for_sale",
		sourceID,
		uuid.New(),
		uuid.New(),
		money.New(42000),
		nil,
		nil,
		nil,
		time.Now().Add(2*time.Hour),
	)
	expiresAt := time.Now().Add(2 * time.Hour)
	quote.ExpiresAt = &expiresAt

	forSale := &forsaleEntity.ForSale{
		ID:           productID,
		Title:        "ForSale Title",
		PricePerUnit: money.New(125000),
		MediaURLs:    []byte(`["https://example.com/for_sale.jpg"]`),
	}

	att := buildShippingQuoteAttachmentJSONV2(quote, forSale, nil)

	if got, ok := att["type"].(string); !ok || got != "shipping_quote" {
		t.Fatalf("expected type shipping_quote, got %#v", att["type"])
	}
	data, ok := att["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data object, got %#v", att["data"])
	}
	if _, ok := att["offer_id"]; ok {
		t.Fatal("flat root key offer_id must not exist")
	}
	if _, ok := att["linked_item_id"]; ok {
		t.Fatal("flat root key linked_item_id must not exist")
	}
	required := []string{"offer_id", "linked_item_id", "linked_item_type", "status", "rate", "seller_id", "linked_item_name", "linked_item_price"}
	for _, key := range required {
		if _, ok := data[key]; !ok {
			t.Fatalf("expected data.%s", key)
		}
	}
	if got := data["linked_item_type"]; got != "for_sale" {
		t.Fatalf("expected for_sale linked_item_type, got %#v", got)
	}
	if got := data["linked_item_id"]; got != sourceID.String() {
		t.Fatalf("expected linked_item_id %s, got %#v", sourceID, got)
	}
	if errs := chatvalidator.ValidateAttachmentJSON(att); chatvalidator.HasValidationErrors(errs) {
		t.Fatalf("expected canonical attachment to pass validator, got: %+v", errs)
	}
	if err := validateCanonicalAttachmentJSON(att); err != nil {
		t.Fatalf("expected guard to pass, got: %v", err)
	}
}

func TestBuildShippingQuoteAttachmentJSON_Auction(t *testing.T) {
	auctionID := uuid.New()
	productID := uuid.New()
	quote := shippingQuoteEntity.NewAuctionShippingQuote(
		uuid.New(),
		productID,
		auctionID,
		"auction",
		auctionID,
		uuid.New(),
		uuid.New(),
		money.New(42000),
		nil,
		nil,
		nil,
		time.Now().Add(2*time.Hour),
	)
	expiresAt := time.Now().Add(2 * time.Hour)
	quote.ExpiresAt = &expiresAt

	currentBid := int64(125000)
	buyNowPrice := int64(150000)
	auction := &auctionEntity.Auction{
		ID:          auctionID,
		CurrentBid:  &currentBid,
		BuyNowPrice: &buyNowPrice,
		Product: &productEntity.Product{
			Title: "Auction Title",
		},
	}

	att := buildShippingQuoteAttachmentJSONV2(quote, nil, auction)
	data, ok := att["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data object, got %#v", att["data"])
	}

	if got := data["linked_item_type"]; got != "auction" {
		t.Fatalf("expected auction linked_item_type, got %#v", got)
	}
	if got := data["linked_item_id"]; got != auctionID.String() {
		t.Fatalf("expected linked_item_id %s, got %#v", auctionID, got)
	}
	if got := data["auction_id"]; got != auctionID.String() {
		t.Fatalf("expected auction_id %s, got %#v", auctionID, got)
	}
	if _, ok := data["product_id"]; ok {
		t.Fatal("auction quote must not emit product_id in linked data")
	}
	if _, ok := data["linked_item_price"]; !ok {
		t.Fatal("expected linked_item_price for auction quote")
	}
	if _, ok := data["linked_item_buy_now_price"]; !ok {
		t.Fatal("expected linked_item_buy_now_price for auction quote")
	}
	if errs := chatvalidator.ValidateAttachmentJSON(att); chatvalidator.HasValidationErrors(errs) {
		t.Fatalf("expected canonical auction attachment to pass validator, got: %+v", errs)
	}
}
