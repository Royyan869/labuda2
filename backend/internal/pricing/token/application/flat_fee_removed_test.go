package application

import (
	"os"
	"strings"
	"testing"
)

// TestFlatBuyerServiceFee_Removed proves the PASS_18 flat Rp3.000 buyer
// checkout fee is no longer wired into pricing token generation.
//
// Before PASS_18V, every token-generation path called
// checkoutfee.BuyerServiceFee() (a hardcoded Rp3.000 constant) regardless of
// how the buyer would eventually pay. That package has been deleted; this
// test is a source-level regression guard so a future edit cannot silently
// reintroduce it (e.g. by copy-pasting an old snippet from git history).
//
// The buyer payment fee is now 0 at token/order-creation time (the buyer
// has not chosen a payment method yet) and is calculated by
// CorePaymentHandler.CreatePayment from the payment_methods table once a
// method is selected — see paymentmethod/entity.CalculateFee.
func TestFlatBuyerServiceFee_Removed(t *testing.T) {
	src, err := os.ReadFile("pricing_token_service.go")
	if err != nil {
		t.Fatalf("read pricing_token_service.go: %v", err)
	}
	code := string(src)

	if strings.Contains(code, "checkoutfee") {
		t.Fatal("REGRESSION: pricing_token_service.go must not import/reference the deleted checkoutfee package")
	}
	if strings.Contains(code, "BuyerServiceFee()") {
		t.Fatal("REGRESSION: pricing_token_service.go must not call the removed flat BuyerServiceFee()")
	}

	// The three token-generation paths (fixed-price, negotiation, auction)
	// must each derive serviceFeeAmount from money.Zero(), not a constant.
	occurrences := strings.Count(code, "serviceFeeAmount := money.Zero()")
	if occurrences < 3 {
		t.Fatalf("expected serviceFeeAmount := money.Zero() at all 3 token-generation call sites, found %d", occurrences)
	}

	if _, err := os.Stat("../../../commerce/checkoutfee"); err == nil {
		t.Fatal("REGRESSION: backend/internal/commerce/checkoutfee package must not exist (flat fee killed in PASS_18V)")
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking checkoutfee package removal: %v", err)
	}
}
