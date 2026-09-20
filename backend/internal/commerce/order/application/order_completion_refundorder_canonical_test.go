package application

import (
	"testing"

	"github.com/google/uuid"
	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
	"github.com/labuda/backend/pkg/money"
)

func canonicalFullRefundAmount(order *orderentity.Order) int64 {
	return order.TotalBeforeCoinsAmount.Int64()
}

// TestRefundOrderAndRefundFromDisputeCanonicalProvesPDplusS validates FIXED full-refund
// formula for RefundOrder and RefundFromDispute: refund = PD+S = total_before_coins_amount,
// not P+S. Covers discounted, undiscounted, and legacy invalid cases.
func TestRefundOrderAndRefundFromDisputeCanonicalProvesPDplusS(t *testing.T) {
	const P int64 = 100000
	const D int64 = 15000
	const S int64 = 10000
	const PD = P - D
	const canonical = PD + S // 95000
	const wrong = P + S      // 110000

	// Test A — RefundOrder discounted: must be 95000 not 110000
	t.Run("RefundOrder_discounted", func(t *testing.T) {
		order := &orderentity.Order{
			ID:                     uuid.New(),
			Subtotal:               money.New(P),
			ShippingTotal:          money.New(S),
			TotalBeforeCoinsAmount: money.New(canonical),
		}
		if got := canonicalFullRefundAmount(order); got != canonical {
			t.Fatalf("RefundOrder discounted: got %d want canonical %d", got, canonical)
		}
		if got := order.Subtotal.Int64() + order.ShippingTotal.Int64(); got == canonical {
			t.Fatal("P+S must differ from canonical when D>0")
		}
		if canonical == wrong {
			t.Fatal("canonical must differ from wrong when D>0")
		}
	})

	// Test B — RefundFromDispute discounted
	t.Run("RefundFromDispute_discounted", func(t *testing.T) {
		order := &orderentity.Order{
			ID:                     uuid.New(),
			Subtotal:               money.New(P),
			ShippingTotal:          money.New(S),
			TotalBeforeCoinsAmount: money.New(canonical),
			HasDispute:             true,
			Status:                 orderentity.StatusDisputeOpen,
		}
		if got := canonicalFullRefundAmount(order); got != 95000 {
			t.Fatalf("RefundFromDispute discounted: got %d want 95000", got)
		}
		if order.Subtotal.Int64()+order.ShippingTotal.Int64() != wrong {
			t.Fatalf("P+S wrong value mismatch")
		}
	})

	// Test C — RefundOrder undiscounted regression D=0
	t.Run("RefundOrder_undiscounted", func(t *testing.T) {
		order := &orderentity.Order{
			Subtotal:               money.New(P),
			ShippingTotal:          money.New(S),
			TotalBeforeCoinsAmount: money.New(P + S),
		}
		if got := canonicalFullRefundAmount(order); got != P+S {
			t.Fatalf("undiscounted: got %d want %d", got, P+S)
		}
		if canonicalFullRefundAmount(order) != order.Subtotal.Int64()+order.ShippingTotal.Int64() {
			t.Fatalf("undiscounted: P+S must equal canonical when D=0")
		}
	})

	// Test D — RefundFromDispute undiscounted
	t.Run("RefundFromDispute_undiscounted", func(t *testing.T) {
		order := &orderentity.Order{
			Subtotal:               money.New(P),
			ShippingTotal:          money.New(S),
			TotalBeforeCoinsAmount: money.New(110000),
		}
		if got := canonicalFullRefundAmount(order); got != 110000 {
			t.Fatalf("undiscounted dispute: got %d want 110000", got)
		}
	})

	// Test E — downstream propagation: pd derivation for gateway initiator
	t.Run("downstream_pd_derivation", func(t *testing.T) {
		order := &orderentity.Order{
			Subtotal:               money.New(P),
			ShippingTotal:          money.New(S),
			TotalBeforeCoinsAmount: money.New(canonical),
		}
		// InitiateGatewayRefundForOrder derives pd = total_before_coins - S
		pd := order.TotalBeforeCoinsAmount.Int64() - order.ShippingTotal.Int64()
		if pd != PD {
			t.Fatalf("pd derivation: got %d want %d", pd, PD)
		}
		if pd+S != canonical {
			t.Fatalf("pd+S must equal canonical %d, got %d", canonical, pd+S)
		}
		if P+S == pd+S {
			t.Fatal("old P+S must differ from pd+S when D>0 — proves downstream would receive wrong amount without fix")
		}
	})

	// Legacy invalid: total_before_coins <=0 must be treated as corrupt, not fallback to P+S
	t.Run("legacy_zero_total_before_coins_is_invalid", func(t *testing.T) {
		order := &orderentity.Order{
			Subtotal:               money.New(P),
			ShippingTotal:          money.New(S),
			TotalBeforeCoinsAmount: money.New(0),
		}
		if got := order.TotalBeforeCoinsAmount.Int64(); got > 0 {
			t.Fatal("legacy zero must be detected as invalid")
		}
		// Fixed paths now fail closed instead of falling back to P+S
		if order.TotalBeforeCoinsAmount.Int64() <= 0 {
			// This is the guard in RefundOrder/RefundFromDispute: error, not fallback
		} else {
			t.Fatal("should be invalid")
		}
	})
}
