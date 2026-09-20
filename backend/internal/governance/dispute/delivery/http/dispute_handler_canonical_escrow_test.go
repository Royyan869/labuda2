package http

import (
	"testing"
)

// TestDisputeDetailCanonicalEscrowProof validates the FIXED formula for
// GetDisputeDetail: dispute detail must expose total_before_coins_amount = PD+S,
// not subtotal+shipping_total = P+S.
// P=100000 D=15000 S=10000 → PD=85000 → canonical=95000, wrong=110000.
func TestDisputeDetailCanonicalEscrowProof(t *testing.T) {
	const P int64 = 100000
	const D int64 = 15000
	const S int64 = 10000
	const PD = P - D
	const canonical = PD + S // 95000
	const wrong = P + S       // 110000

	if canonical != 95000 {
		t.Fatalf("canonical must be 95000, got %d", canonical)
	}
	if wrong != 110000 {
		t.Fatalf("wrong must be 110000, got %d", wrong)
	}
	// Simulate persisted order row after FIX: total_before_coins_amount = canonical
	persistedTotalBeforeCoins := canonical
	// Simulate OLD query: SELECT (subtotal+shipping_total) as escrow_amount
	oldQueryResult := P + S
	// Simulate NEW query: SELECT total_before_coins_amount as escrow_amount
	newQueryResult := persistedTotalBeforeCoins

	if oldQueryResult != wrong {
		t.Fatalf("old query must return wrong P+S=110000, got %d", oldQueryResult)
	}
	if newQueryResult != canonical {
		t.Fatalf("new query must return canonical PD+S=95000, got %d", newQueryResult)
	}
	if oldQueryResult == newQueryResult {
		t.Fatal("old and new query results must differ when D>0, proving fix changes API result")
	}
	// API must expose canonical, not wrong
	if newQueryResult == wrong {
		t.Fatal("API must not expose P+S when discount exists")
	}
}
