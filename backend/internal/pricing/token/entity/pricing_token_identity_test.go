package entity

import (
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/money"
)

func TestNewPricingToken_BindsProductAndSource(t *testing.T) {
	userID := uuid.New()
	productID := uuid.New()
	sourceID := uuid.New()
	token := NewPricingToken(
		userID,
		productID,
		"for_sale",
		sourceID,
		2,
		money.New(1000),
		money.New(250),
		10,
		money.New(100),
		money.New(1150),
		money.New(50),
		uuid.New(),
		"JNE",
		"courier",
		uuid.New(),
		[]byte(`{}`),
		nil,
		nil,
		nil,
		nil,
		money.Zero(),
		nil,
		0,
		0,
		0,
	)

	if token.ProductID != productID {
		t.Fatalf("expected product_id %s, got %s", productID, token.ProductID)
	}
	if token.SourceType != "for_sale" {
		t.Fatalf("expected source_type for_sale, got %s", token.SourceType)
	}
	if token.SourceID != sourceID {
		t.Fatalf("expected source_id %s, got %s", sourceID, token.SourceID)
	}

	if err := token.ValidateForOrder(userID, productID, "for_sale", sourceID, 2, token.AddressID, token.ShippingSetupID); err != nil {
		t.Fatalf("expected validation to pass, got error: %v", err)
	}

	if err := token.ValidateForOrder(userID, uuid.New(), "for_sale", sourceID, 2, token.AddressID, token.ShippingSetupID); err == nil {
		t.Fatal("expected product mismatch to fail")
	}
}

func TestNewPricingTokenFromAuction_BindsAuctionAndProduct(t *testing.T) {
	userID := uuid.New()
	productID := uuid.New()
	auctionID := uuid.New()
	token := NewPricingTokenFromAuction(
		userID,
		productID,
		auctionID,
		1,
		money.New(2500),
		money.New(100),
		12,
		money.New(100),
		money.New(2520),
		money.New(25),
		uuid.New(),
		"JNE",
		"courier",
		uuid.New(),
		[]byte(`{}`),
		nil,
		nil,
		nil,
		nil,
		money.Zero(),
		0,
		0,
		0,
	)

	if token.ProductID != productID {
		t.Fatalf("expected product_id %s, got %s", productID, token.ProductID)
	}
	if token.SourceType != "auction" {
		t.Fatalf("expected source_type auction, got %s", token.SourceType)
	}
	if token.SourceID != auctionID {
		t.Fatalf("expected source_id %s, got %s", auctionID, token.SourceID)
	}
}


