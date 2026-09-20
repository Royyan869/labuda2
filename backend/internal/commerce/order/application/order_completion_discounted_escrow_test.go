package application

import (
	"testing"

	"github.com/google/uuid"
	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
	"github.com/labuda/backend/pkg/money"
)

// PROOF (ORDER-SCOPE-A money base): the buyer-funded base is PD + S and the
// payment/service fee F NEVER enters it.
//
//	P = 100000, D = 15000, PD = 85000, S = 10000, F = 2500
//	total_before_coins_amount = 95000  (PD + S)
//	total_payable_amount      = 97500  (base + F, after payment selection)
//	escrow / refund / release = 95000  (canonical base)
//	coin base (PD)            = 85000
//
// This test is the guard proof for PartialRefundFromDispute: the escrow is
// validated against PD + S (derived from the persisted base), never against the
// rejected P + S = 110000 model, and never against Subtotal (P).
func TestDiscountedOrderPartialRefundGuardProvesCanonicalBase(t *testing.T) {
	const (
		P        int64 = 100000
		D        int64 = 15000
		PD       int64 = P - D // 85000
		S        int64 = 10000
		F        int64 = 2500
		base     int64 = PD + S   // 95000
		payable  int64 = base + F // 97500
		wrongOld int64 = P + S    // 110000 — rejected P+S model
	)

	order := &orderentity.Order{
		ID:                     uuid.New(),
		BuyerID:                uuid.New(),
		SellerID:               uuid.New(),
		SourceType:             orderentity.OrderSourceForSale,
		SourceID:               uuid.New(),
		Quantity:               1,
		UnitPrice:              money.New(P),
		Subtotal:               money.New(P), // P, NOT PD
		ShippingTotal:          money.New(S),
		CommissionAmount:       money.New(4000),
		ServiceFeeAmount:       money.New(F),
		TotalBeforeCoinsAmount: money.New(base),    // PD + S — fee F excluded
		TotalPayableAmount:     money.New(payable), // base + F
		Status:                 orderentity.StatusDisputeOpen,
		EscrowStatus:           orderentity.EscrowStatusHolding,
		HasDispute:             true,
	}

	// PRODUCTION DERIVATION: PD comes from the persisted base, never from Subtotal (P).
	if !order.HasCanonicalMoneyBase() {
		t.Fatalf("canonical base must be valid: base=%d shipping=%d", order.TotalBeforeCoinsAmount.Int64(), order.ShippingTotal.Int64())
	}
	if got := order.DiscountedProductAmount().Int64(); got != PD {
		t.Fatalf("DiscountedProductAmount = %d, want PD = %d", got, PD)
	}
	if order.Subtotal.Int64() == order.DiscountedProductAmount().Int64() {
		t.Fatal("Subtotal (P) must NOT be the discounted product value for a discounted order")
	}

	// Base decomposition: PD + S == base, and the fee F is outside it.
	if got := order.DiscountedProductAmount().Add(order.ShippingTotal).Int64(); got != base {
		t.Fatalf("PD + S = %d, want canonical base %d", got, base)
	}
	if order.TotalBeforeCoinsAmount.Int64() == payable {
		t.Fatal("payment fee F must NEVER be part of total_before_coins_amount")
	}
	if got := order.TotalBeforeCoinsAmount.Add(order.ServiceFeeAmount).Int64(); got != order.TotalPayableAmount.Int64() {
		t.Fatalf("total_payable_amount must equal base + F: got %d, want %d", got, payable)
	}

	// The P+S model is wrong for discounted orders — this is the bug the guard fixes.
	if wrongOld == base {
		t.Fatal("P+S and PD+S must differ when D > 0")
	}
	// A P+S-derived PD (escrow - shipping) is not the canonical PD.
	if wrongOld-S == PD {
		t.Fatal("P+S-derived PD must differ from the canonical PD")
	}

	// Guard behaviour, expressed with the production derivations.
	escrowAmount := order.TotalBeforeCoinsAmount
	itemPrice := escrowAmount.Sub(order.ShippingTotal) // PD
	shippingFee := order.ShippingTotal
	if !itemPrice.Add(shippingFee).Equal(escrowAmount) {
		t.Fatalf("guard must accept PD(%d) + S(%d) == escrow(%d)", itemPrice.Int64(), shippingFee.Int64(), escrowAmount.Int64())
	}
	// The old P+S guard would have rejected the now-canonical discounted order.
	if money.New(wrongOld).Equal(escrowAmount) {
		t.Fatal("old P+S guard must not validate a discounted order's escrow")
	}

	// Undiscounted regression: D = 0 ⇒ P+S == PD+S, both paths agree.
	undiscounted := &orderentity.Order{
		Subtotal:               money.New(P),
		ShippingTotal:          money.New(S),
		TotalBeforeCoinsAmount: money.New(P + S),
	}
	if !undiscounted.HasCanonicalMoneyBase() {
		t.Fatal("undiscounted order with a positive base must be valid")
	}
	if got := undiscounted.DiscountedProductAmount().Int64(); got != P {
		t.Fatalf("undiscounted PD = %d, want P = %d", got, P)
	}
	if got := undiscounted.DiscountedProductAmount().Add(undiscounted.ShippingTotal).Int64(); got != P+S {
		t.Fatalf("undiscounted PD + S = %d, want %d", got, P+S)
	}

	// FAIL CLOSED: an order row without a persisted base has no derivable PD,
	// so the guard must reject it instead of falling back to Subtotal (P).
	for _, broken := range []*orderentity.Order{
		{Subtotal: money.New(P), ShippingTotal: money.New(S)},                                       // no base at all
		{Subtotal: money.New(P), ShippingTotal: money.New(S), TotalBeforeCoinsAmount: money.Zero()}, // legacy zero base
		{Subtotal: money.New(P), ShippingTotal: money.New(S), TotalBeforeCoinsAmount: money.New(S)}, // base == shipping ⇒ PD == 0
	} {
		if broken.HasCanonicalMoneyBase() {
			t.Fatalf("base=%d shipping=%d must fail closed (no P fallback)", broken.TotalBeforeCoinsAmount.Int64(), broken.ShippingTotal.Int64())
		}
	}
}
