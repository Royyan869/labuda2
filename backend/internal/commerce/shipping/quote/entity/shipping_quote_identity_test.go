package entity

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/money"
)

func TestNewShippingQuote_BindsProductAndSource(t *testing.T) {
	productID := uuid.New()
	sourceID := uuid.New()
	expiresAt := time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)
	quote := NewShippingQuote(
		uuid.New(),
		productID,
		"for_sale",
		sourceID,
		uuid.New(),
		uuid.New(),
		money.New(5000),
		nil,
		nil,
		nil,
		expiresAt,
	)

	if quote.ProductID != productID {
		t.Fatalf("expected product_id %s, got %s", productID, quote.ProductID)
	}
	if quote.SourceType == nil || *quote.SourceType != "for_sale" {
		t.Fatalf("expected source_type for_sale, got %#v", quote.SourceType)
	}
	if quote.SourceID == nil || *quote.SourceID != sourceID {
		t.Fatalf("expected source_id %s, got %#v", sourceID, quote.SourceID)
	}
	if quote.ExpiresAt == nil || !quote.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("expected expires_at %s, got %#v", expiresAt, quote.ExpiresAt)
	}
	if id, isAuction := quote.GetItemReference(); isAuction || id != sourceID {
		t.Fatalf("expected item reference to resolve fixed-price source, got %s auction=%v", id, isAuction)
	}
}

func TestNewAuctionShippingQuote_BindsProductAndSource(t *testing.T) {
	productID := uuid.New()
	auctionID := uuid.New()
	expiresAt := time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)
	quote := NewAuctionShippingQuote(
		uuid.New(),
		productID,
		auctionID,
		"auction",
		auctionID,
		uuid.New(),
		uuid.New(),
		money.New(5000),
		nil,
		nil,
		nil,
		expiresAt,
	)

	if quote.ProductID != productID {
		t.Fatalf("expected product_id %s, got %s", productID, quote.ProductID)
	}
	if quote.SourceType == nil || *quote.SourceType != "auction" {
		t.Fatalf("expected source_type auction, got %#v", quote.SourceType)
	}
	if quote.SourceID == nil || *quote.SourceID != auctionID {
		t.Fatalf("expected source_id %s, got %#v", auctionID, quote.SourceID)
	}
	if quote.ExpiresAt == nil || !quote.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("expected expires_at %s, got %#v", expiresAt, quote.ExpiresAt)
	}
	if id, isAuction := quote.GetItemReference(); !isAuction || id != auctionID {
		t.Fatalf("expected auction item reference, got %s auction=%v", id, isAuction)
	}
}
