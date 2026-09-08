package entity

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/money"
)

func TestNewPricingTokenFromNegotiation_WithQuote(t *testing.T) {
	userID := uuid.New()
	productID := uuid.New()
	negotiationID := uuid.New()
	addressID := uuid.New()
	quoteID := uuid.New()
	addrSnap := []byte(`{"city":"Jakarta"}`)

	token := NewPricingTokenFromNegotiation(
		userID, productID, negotiationID, 1,
		money.New(5_000_000), money.New(200_000),
		5, money.New(250_000), money.New(5_200_000), money.Zero(),
		uuid.Nil, "Manual Quote", "manual",
		addressID, addrSnap,
		nil, nil, nil, nil, money.Zero(),
		0, 1_000_000, 5_000_000,
		&quoteID,
	)

	if token.ShippingQuoteID == nil || *token.ShippingQuoteID != quoteID {
		t.Fatalf("ShippingQuoteID = %v, want %s", token.ShippingQuoteID, quoteID)
	}
	if token.ShippingSetupID != uuid.Nil {
		t.Fatalf("ShippingSetupID = %s, want Nil for quote", token.ShippingSetupID)
	}
	if token.NegotiationID == nil || *token.NegotiationID != negotiationID {
		t.Fatalf("NegotiationID mismatch")
	}
	if token.UnitPrice.Int64() != 5_000_000 {
		t.Fatalf("UnitPrice = %d, want 5000000", token.UnitPrice.Int64())
	}
	if token.ShippingTotal.Int64() != 200_000 {
		t.Fatalf("ShippingTotal = %d, want 200000", token.ShippingTotal.Int64())
	}
	if token.SourceType != "negotiation" || token.SourceID != negotiationID {
		t.Fatalf("SourceType/ID = %s/%s, want negotiation/%s", token.SourceType, token.SourceID, negotiationID)
	}
}

func TestNewPricingTokenFromNegotiation_WithOption(t *testing.T) {
	userID := uuid.New()
	productID := uuid.New()
	negotiationID := uuid.New()
	addressID := uuid.New()
	setupID := uuid.New()

	token := NewPricingTokenFromNegotiation(
		userID, productID, negotiationID, 1,
		money.New(5_000_000), money.New(15000),
		5, money.New(250_000), money.New(5_015_000), money.Zero(),
		setupID, "Regular", "train",
		addressID, []byte(`{}`),
		nil, nil, nil, nil, money.Zero(),
		0, 1_000_000, 5_000_000,
		nil,
	)

	if token.ShippingQuoteID != nil {
		t.Fatalf("ShippingQuoteID = %v, want nil for option", token.ShippingQuoteID)
	}
	if token.ShippingSetupID != setupID {
		t.Fatalf("ShippingSetupID = %s, want %s", token.ShippingSetupID, setupID)
	}
}

func TestPricingToken_ValidateForOrder_NegotiationQuoteMode(t *testing.T) {
	userID := uuid.New()
	productID := uuid.New()
	negotiationID := uuid.New()
	quoteID := uuid.New()
	addressID := uuid.New()
	token := &PricingToken{
		UserID:          userID,
		ProductID:       productID,
		SourceType:      "negotiation",
		SourceID:        negotiationID,
		Quantity:        1,
		AddressID:       addressID,
		ShippingQuoteID: &quoteID,
		ShippingSetupID: uuid.Nil,
		ExpiresAt:       time.Now().Add(10 * time.Minute),
		IsUsed:          false,
	}

	// Quote mode: shippingSetupID must be Nil — valid
	if err := token.ValidateForOrder(userID, productID, "negotiation", negotiationID, 1, addressID, uuid.Nil); err != nil {
		t.Fatalf("ValidateForOrder quote mode failed: %v", err)
	}
	// Quote mode: shippingSetupID non-Nil must fail
	if err := token.ValidateForOrder(userID, productID, "negotiation", negotiationID, 1, addressID, uuid.New()); err == nil {
		t.Fatal("expected error for quote mode with setup id")
	}
}


