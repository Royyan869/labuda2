package finance_test

import (
	"math"
	"testing"

	"github.com/labuda/backend/internal/finance"
)

// TestPromotionCumulativeSpend_CPM7500 locks the canonical worked example from
// the promotion contract audit: CPM Rp7.500 does not divide evenly into 1000
// (per-impression = Rp7,5). The cumulative model floors at the *total* so no
// per-impression rounding drift exists: S(N) = floor(N*7500/1000).
func TestPromotionCumulativeSpend_CPM7500(t *testing.T) {
	want := []int64{0, 7, 15, 22, 30, 37, 45, 52, 60, 67, 75}
	for n := int64(0); n < int64(len(want)); n++ {
		got, err := finance.PromotionCumulativeSpend(n, 7500)
		if err != nil {
			t.Fatalf("PromotionCumulativeSpend(%d, 7500) error: %v", n, err)
		}
		if got != want[n] {
			t.Fatalf("S(%d) = %d, want %d", n, got, want[n])
		}
	}
}

// TestPromotionCharge_CPM7500 locks charge(N) = S(N) - S(N-1): successive
// charges alternate 7/8 Rupiah but their SUM is exactly S(N).
func TestPromotionCharge_CPM7500(t *testing.T) {
	wantCharges := []int64{7, 8, 7, 8, 7, 8, 7, 8, 7, 8}
	var cumulative int64
	for n := int64(1); n <= int64(len(wantCharges)); n++ {
		charge, err := finance.PromotionCharge(n, 7500)
		if err != nil {
			t.Fatalf("PromotionCharge(%d, 7500) error: %v", n, err)
		}
		if charge != wantCharges[n-1] {
			t.Fatalf("charge(%d) = %d, want %d", n, charge, wantCharges[n-1])
		}
		cumulative += charge
		sn, err := finance.PromotionCumulativeSpend(n, 7500)
		if err != nil {
			t.Fatalf("PromotionCumulativeSpend(%d) error: %v", n, err)
		}
		if cumulative != sn {
			t.Fatalf("sum of charges up to N=%d = %d, want S(N) = %d", n, cumulative, sn)
		}
	}
}

// TestPromotionCharge_NoDrift sums per-impression charges over a large N at a
// deliberately awkward CPM and proves the total equals S(N) exactly.
func TestPromotionCharge_NoDrift(t *testing.T) {
	const cpm = 9999 // 9999/1000 = 9.999 Rupiah per impression
	const n = int64(50000)
	var sum int64
	for i := int64(1); i <= n; i++ {
		charge, err := finance.PromotionCharge(i, cpm)
		if err != nil {
			t.Fatalf("PromotionCharge(%d, %d) error: %v", i, cpm, err)
		}
		if charge < 0 {
			t.Fatalf("negative charge at N=%d: %d", i, charge)
		}
		sum += charge
	}
	sn, err := finance.PromotionCumulativeSpend(n, cpm)
	if err != nil {
		t.Fatalf("PromotionCumulativeSpend error: %v", err)
	}
	if sum != sn {
		t.Fatalf("sum of %d charges = %d, want S(%d) = %d", n, sum, n, sn)
	}
}

func TestPromotionCharge_ZeroCPMAndZeroN(t *testing.T) {
	// Zero CPM: impressions are free — cumulative spend and charge stay 0.
	for _, n := range []int64{0, 1, 5, 100} {
		s, err := finance.PromotionCumulativeSpend(n, 0)
		if err != nil || s != 0 {
			t.Fatalf("PromotionCumulativeSpend(%d, 0) = %d, %v; want 0, nil", n, s, err)
		}
		c, err := finance.PromotionCharge(n, 0)
		if err != nil || c != 0 {
			t.Fatalf("PromotionCharge(%d, 0) = %d, %v; want 0, nil", n, c, err)
		}
	}
	// Charge for N=0 is defined as 0.
	c, err := finance.PromotionCharge(0, 7500)
	if err != nil || c != 0 {
		t.Fatalf("PromotionCharge(0, 7500) = %d, %v; want 0, nil", c, err)
	}
}

func TestPromotionArithmetic_InvalidInputs(t *testing.T) {
	if _, err := finance.PromotionCumulativeSpend(-1, 7500); err == nil {
		t.Fatal("expected error for negative impression count")
	}
	if _, err := finance.PromotionCumulativeSpend(10, -1); err == nil {
		t.Fatal("expected error for negative cpm")
	}
	if _, err := finance.PromotionCharge(1, -5); err == nil {
		t.Fatal("expected error for negative cpm in charge")
	}
	if _, err := finance.PromotionEstimatedImpressions(10, -1); err == nil {
		t.Fatal("expected error for negative cpm in estimation")
	}
	// Overflow guard: n*cpm would exceed int64.
	if _, err := finance.PromotionCumulativeSpend(math.MaxInt64, 2); err == nil {
		t.Fatal("expected overflow error for MaxInt64 impressions at cpm 2")
	}
	if _, err := finance.PromotionEstimatedImpressions(math.MaxInt64, 1); err == nil {
		t.Fatal("expected overflow error for MaxInt64 budget")
	}
}

func TestPromotionEstimatedImpressions(t *testing.T) {
	cases := []struct {
		budget int64
		cpm    int64
		want   int64
	}{
		{30000, 7500, 4000}, // S(4000)=30000 exactly exhausts budget
		{30001, 7500, 4000}, // S(4000)<=budget<S(4001)
		{0, 7500, 0},
		{100000, 1000, 100000},
		{50000, 0, 0}, // zero CPM: unlimited free impressions
	}
	for _, c := range cases {
		got, err := finance.PromotionEstimatedImpressions(c.budget, c.cpm)
		if err != nil {
			t.Fatalf("PromotionEstimatedImpressions(%d, %d) error: %v", c.budget, c.cpm, err)
		}
		if got != c.want {
			t.Fatalf("PromotionEstimatedImpressions(%d, %d) = %d, want %d", c.budget, c.cpm, got, c.want)
		}
	}
	// Round-trip: the estimate never exceeds the budget.
	sn, err := finance.PromotionCumulativeSpend(4000, 7500)
	if err != nil || sn != 30000 {
		t.Fatalf("round-trip S(4000, 7500) = %d, %v; want 30000", sn, err)
	}
}
