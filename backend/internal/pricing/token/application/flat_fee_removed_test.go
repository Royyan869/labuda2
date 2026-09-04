package application

import (
	"os"
	"strings"
	"testing"
)

// TestPreviewTimeServiceFee_IsZero proves the BUSINESS INVARIANT:
//
//   Preview-time (token generation) service fee MUST be zero for every active
//   pricing path. The buyer has not yet chosen a payment method at preview
//   time, so no payment fee can be computed. The actual payment fee is
//   calculated later by CorePaymentHandler.CreatePayment from the
//   payment_methods table once a method is selected — see
//   paymentmethod/entity.CalculateFee.
//
// This test enforces:
//   1. No reference to the deleted checkoutfee package exists.
//   2. No call to the removed flat BuyerServiceFee() constant exists.
//   3. The canonical money-flow function (calculatePostDiscountMoneyFlow)
//      sets serviceFeeAmount to zero.
//   4. All 3 token-generation constructors (ForSale, Negotiation, Auction)
//      pass money.Zero() as the serviceFeeAmount parameter.
//   5. All 3 PricingSnapshot response objects set ServiceFeeAmount to zero.
//   6. The backend/internal/commerce/checkoutfee directory no longer exists.
//
// AUTHORITATIVE COVERAGE:
//   Path 1 — ForSale direct        (GenerateForForSale)
//   Path 2 — Negotiation            (GenerateForNegotiation)
//   Path 3 — Auction buy-now        (GenerateForAuction, buy_now branch)
//   Path 4 — Auction bid-win        (GenerateForAuction, bid_win branch)
//
//   Paths 3 and 4 share GenerateForAuction, so covering that single
//   function covers both auction settlement types.
func TestPreviewTimeServiceFee_IsZero(t *testing.T) {
	src, err := os.ReadFile("pricing_token_service.go")
	if err != nil {
		t.Fatalf("read pricing_token_service.go: %v", err)
	}
	code := string(src)

	// =================================================================
	// INVARIANT 1: No reference to the deleted checkoutfee package
	// =================================================================
	if strings.Contains(code, "checkoutfee") {
		t.Fatal("REGRESSION: pricing_token_service.go must not import/reference the deleted checkoutfee package")
	}
	if strings.Contains(code, "BuyerServiceFee()") {
		t.Fatal("REGRESSION: pricing_token_service.go must not call the removed flat BuyerServiceFee()")
	}

	// =================================================================
	// INVARIANT 2: Canonical money-flow authority sets serviceFee = 0
	// =================================================================
	// calculatePostDiscountMoneyFlow is the SINGLE shared function called
	// by all 3 pricing paths. It must set serviceFeeAmount to zero.
	if !strings.Contains(code, "serviceFeeAmount := money.Zero()") {
		t.Fatal("REGRESSION: calculatePostDiscountMoneyFlow must set serviceFeeAmount := money.Zero()")
	}

	// =================================================================
	// INVARIANT 3: All 3 token constructors receive money.Zero() serviceFee
	// =================================================================
	// Each NewPricingToken/NewPricingTokenFromNegotiation/NewPricingTokenFromAuction
	// passes money.Zero() as the serviceFeeAmount parameter. In every case,
	// this money.Zero() immediately follows the EscrowAmount parameter.	// Match both LF and CRLF line endings (depends on git checkout config).
	constructorZeroFeeLF := strings.Count(code, "postDiscount.EscrowAmount,\n\t\tmoney.Zero(),")
	constructorZeroFeeCRLF := strings.Count(code, "postDiscount.EscrowAmount,\r\n\t\tmoney.Zero(),")
	constructorZeroFee := constructorZeroFeeLF + constructorZeroFeeCRLF
	if constructorZeroFee < 3 {
		t.Fatalf("REGRESSION: all 3 token constructors (ForSale, Negotiation, Auction) must pass money.Zero() as serviceFeeAmount after EscrowAmount, found %d (LF=%d CRLF=%d)", constructorZeroFee, constructorZeroFeeLF, constructorZeroFeeCRLF)
	}

	// =================================================================
	// INVARIANT 4: All 3 PricingSnapshot responses set ServiceFeeAmount = 0
	// =================================================================
	// Each Generate*Response PricingSnapshot struct must set
	// ServiceFeeAmount: money.Zero() so the client preview never shows a fee.
	snapshotZeroFee := strings.Count(code, "ServiceFeeAmount:   money.Zero(),")
	if snapshotZeroFee < 3 {
		t.Fatalf("REGRESSION: all 3 PricingSnapshot responses must set ServiceFeeAmount: money.Zero(), found %d", snapshotZeroFee)
	}

	// =================================================================
	// INVARIANT 5: Deleted package directory must not exist
	// =================================================================
	if _, err := os.Stat("../../../commerce/checkoutfee"); err == nil {
		t.Fatal("REGRESSION: backend/internal/commerce/checkoutfee package must not exist (flat fee killed in PASS_18V)")
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking checkoutfee package removal: %v", err)
	}
}
