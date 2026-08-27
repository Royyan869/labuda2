package entity

import "testing"

// TestNewPromotionPackage_PriceAmountRoundTripsUnchanged locks the PASS_18N
// fix: PriceAmount is a Rupiah integer end to end. A package created with
// price 50000 (Rp 50,000) must carry exactly 50000 — no /100 or *100
// scaling anywhere in construction. Before the fix this field was named
// PriceCents but was never actually divided anywhere in the purchase flow,
// so admins entering a display price had buyers charged that raw value
// unscaled (a de facto 100x overcharge relative to the "cents" label).
func TestNewPromotionPackage_PriceAmountRoundTripsUnchanged(t *testing.T) {
	const priceAmount = 50000 // Rp 50,000

	pkg, err := NewPromotionPackage(
		"Promote Basic (3 Days)",
		72,
		336,
		priceAmount,
		[]TargetType{TargetTypeForSale},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pkg.PriceAmount != priceAmount {
		t.Errorf("PriceAmount: want %d (no scaling), got %d", priceAmount, pkg.PriceAmount)
	}
}

func TestNewPromotionPackage_NegativePriceAmountRejected(t *testing.T) {
	_, err := NewPromotionPackage(
		"Invalid",
		72,
		336,
		-1,
		[]TargetType{TargetTypeForSale},
	)
	if err == nil {
		t.Fatal("expected validation error for negative price_amount, got nil")
	}
}
