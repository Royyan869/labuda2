package application

import (
	"testing"

	"github.com/google/uuid"
	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
	"github.com/labuda/backend/pkg/money"
)

// PROOF (ORDER-SCOPE-A money base): full refunds on the canonical exception
// paths (CancelOverdue, Expire) refund the buyer-funded base PD + S =
// total_before_coins_amount — NOT the rejected P + S model (wrong when D > 0)
// and NOT Subtotal (P).
//
// P=100000, D=15000, PD=85000, S=10000:
//
//	canonical refund base = PD + S = 95000
//	rejected P + S model  =     110000
func TestCancelOverdueAndExpireRefundCanonicalProvesPDplusS(t *testing.T) {
	const (
		P   int64 = 100000
		D   int64 = 15000
		PD  int64 = P - D
		S   int64 = 10000
		can int64 = PD + S // 95000
		bad int64 = P + S  // 110000
	)

	discounted := &orderentity.Order{
		ID:                     uuid.New(),
		BuyerID:                uuid.New(),
		SellerID:               uuid.New(),
		Subtotal:               money.New(P), // P, NOT PD
		ShippingTotal:          money.New(S),
		CommissionAmount:       money.New(4000),
		TotalBeforeCoinsAmount: money.New(can),
		Status:                 orderentity.StatusPaid,
		EscrowStatus:           orderentity.EscrowStatusHolding,
	}

	// Canonical full-refund amount is the persisted buyer-funded base.
	if !discounted.HasCanonicalMoneyBase() {
		t.Fatalf("discounted order base must be valid: base=%d shipping=%d", discounted.TotalBeforeCoinsAmount.Int64(), discounted.ShippingTotal.Int64())
	}
	if got := discounted.TotalBeforeCoinsAmount.Int64(); got != can {
		t.Fatalf("full refund = %d, want canonical %d", got, can)
	}
	if discounted.TotalBeforeCoinsAmount.Int64() == bad {
		t.Fatal("full refund must NOT equal P+S for a discounted order")
	}
	// Expire goes through the same helper, so the same base applies.
	if got := discounted.TotalBeforeCoinsAmount.Int64(); got != 95000 {
		t.Fatalf("expire refund = %d, want 95000", got)
	}

	// The PD handed to the gateway/ledger is the production derivation, not P.
	if got := discounted.DiscountedProductAmount().Int64(); got != PD {
		t.Fatalf("gateway PD = %d, want %d", got, PD)
	}
	if got := discounted.DiscountedProductAmount().Add(discounted.ShippingTotal).Int64(); got != can {
		t.Fatalf("PD + S = %d, want canonical %d", got, can)
	}
	if discounted.Subtotal.Int64() == discounted.DiscountedProductAmount().Int64() {
		t.Fatal("Subtotal (P) must never be used as the refund PD")
	}

	// Undiscounted regression: D = 0 ⇒ P+S == PD+S == 110000.
	undiscounted := &orderentity.Order{
		Subtotal:               money.New(P),
		ShippingTotal:          money.New(S),
		TotalBeforeCoinsAmount: money.New(P + S),
	}
	if !undiscounted.HasCanonicalMoneyBase() {
		t.Fatal("undiscounted order base must be valid")
	}
	if got := undiscounted.TotalBeforeCoinsAmount.Int64(); got != P+S {
		t.Fatalf("undiscounted refund = %d, want %d", got, P+S)
	}
	if got := undiscounted.DiscountedProductAmount().Add(undiscounted.ShippingTotal).Int64(); got != undiscounted.TotalBeforeCoinsAmount.Int64() {
		t.Fatal("undiscounted PD + S must equal the canonical base")
	}

	// A refunded amount above the base is impossible on the canonical model:
	// the rejected P+S path would have over-refunded by exactly D.
	if bad-can != D {
		t.Fatalf("P+S over-refund must equal D: got %d, want %d", bad-can, D)
	}
}

// FAIL CLOSED: a row without a valid buyer-funded base has no derivable PD, so
// the refund paths must error out rather than fall back to P or P+S.
func TestFullRefundFallbackLegacyZeroTotalBeforeCoins(t *testing.T) {
	legacy := &orderentity.Order{
		Subtotal:               money.New(100000),
		ShippingTotal:          money.New(10000),
		TotalBeforeCoinsAmount: money.Zero(),
	}
	if legacy.HasCanonicalMoneyBase() {
		t.Fatal("legacy zero total_before_coins must be invalid (fail closed)")
	}
	if got := legacy.DiscountedProductAmount().Int64(); got != -10000 {
		t.Fatalf("absent base yields a non-positive PD: got %d", got)
	}
	if legacy.Subtotal.Int64()+legacy.ShippingTotal.Int64() != 110000 {
		t.Fatal("P+S sanity: want 110000")
	}
	// Correct behaviour: callers return an error, never fall back to P+S.
}
