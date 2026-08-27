package application

import (
	"testing"

	coinsapp "github.com/labuda/backend/internal/incentive/coins/application"
)

func TestCanonicalFormula_CoinsMaxBasedOnPDOnly(t *testing.T) {
	const (
		subtotal  = int64(100000)
		discount  = int64(10000)
		shippingB = int64(50000)
	)

	discountedProduct := subtotal - discount
	got := coinsapp.MaxCoinsAllowedForDiscountedProduct(discountedProduct)
	if got != 18000 {
		t.Fatalf("max coins = %d, want 18000 for PD=90000", got)
	}

	if gotWithShippingB := coinsapp.MaxCoinsAllowedForDiscountedProduct(discountedProduct); gotWithShippingB != got {
		t.Fatalf("max coins changed with shipping B: got %d, want %d", gotWithShippingB, got)
	}

	wrongInclusiveB := (subtotal + shippingB - discount) / 5
	if wrongInclusiveB != 28000 {
		t.Fatalf("shipping-inclusive formula with shippingB = 50000 should be 28000, got %d", wrongInclusiveB)
	}
	if got == wrongInclusiveB {
		t.Fatalf("canonical max coins unexpectedly matched shipping-inclusive result: %d", got)
	}
}
