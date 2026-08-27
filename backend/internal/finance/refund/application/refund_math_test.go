package application

import "testing"

// Canonical values for all S2C2 examples:
//
//	PD=90000, S=20000, C=4500, K=18000, F=4000
const (
	testPD = 90_000
	testS  = 20_000
	testC  = 4_500
	testK  = 18_000
)

func assertFields(t *testing.T, label string, b *ProportionalRefundBreakdown, wantCash, wantCoin, wantComm, wantSeller int64) {
	t.Helper()
	if b.CashRefund != wantCash {
		t.Errorf("%s: CashRefund=%d want %d", label, b.CashRefund, wantCash)
	}
	if b.CoinDelta != wantCoin {
		t.Errorf("%s: CoinDelta=%d want %d", label, b.CoinDelta, wantCoin)
	}
	if b.CommissionDelta != wantComm {
		t.Errorf("%s: CommissionDelta=%d want %d", label, b.CommissionDelta, wantComm)
	}
	if b.SellerComponent != wantSeller {
		t.Errorf("%s: SellerComponent=%d want %d", label, b.SellerComponent, wantSeller)
	}
	// Accounting identity: cash + coins == rpd + rs
	if b.CashRefund+b.CoinDelta != b.Rpd+b.Rs {
		t.Errorf("%s: INVARIANT BROKEN: cash(%d)+coins(%d) != rpd(%d)+rs(%d)",
			label, b.CashRefund, b.CoinDelta, b.Rpd, b.Rs)
	}
	// Gateway cap: cumulative cash <= PD + S - K
	cumCashAfter := b.CumProductRefundAfter + b.CumShippingRefundAfter - b.CumCoinsRestoredAfter
	if cumCashAfter+b.CumCoinsRestoredAfter != b.CumProductRefundAfter+b.CumShippingRefundAfter {
		t.Errorf("%s: cumulative identity broken", label)
	}
	if cumCashAfter > testPD+testS-testK {
		t.Errorf("%s: GATEWAY CAP BROKEN", label)
	}
}

func assertCumulativeCash(t *testing.T, label string, b *ProportionalRefundBreakdown, wantCumCash int64) {
	t.Helper()
	gotCumCash := b.CumProductRefundAfter + b.CumShippingRefundAfter - b.CumCoinsRestoredAfter
	if gotCumCash != wantCumCash {
		t.Errorf("%s: CumCash=%d want %d", label, gotCumCash, wantCumCash)
	}
	if gotCumCash+b.CumCoinsRestoredAfter != b.CumProductRefundAfter+b.CumShippingRefundAfter {
		t.Errorf("%s: cumulative identity broken", label)
	}
}

func TestS2C2_FullRefund(t *testing.T) {
	// Rpd=90000, Rs=20000
	b, err := CalculateProportionalRefundBreakdown(
		testPD, testS, testC, testK, 90_000, 20_000,
		0, 0, 0, 0,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// CashRefund = 90000+20000-18000 = 92000
	// CoinDelta = floor(18000*90000/90000) - 0 = 18000
	// CommissionDelta = floor(4500*90000/90000) - 0 = 4500
	// SellerComponent = 90000+20000-4500 = 105500
	assertFields(t, "full", b, 92_000, 18_000, 4_500, 105_500)
	assertCumulativeCash(t, "full", b, 92_000)
	if b.CumProductRefundAfter != 90_000 {
		t.Errorf("cumProductAfter=%d want 90000", b.CumProductRefundAfter)
	}
	if b.CumShippingRefundAfter != 20_000 {
		t.Errorf("cumShippingAfter=%d want 20000", b.CumShippingRefundAfter)
	}
}

func TestS2C2_ProductOnlyFullRefund(t *testing.T) {
	// Rpd=90000, Rs=0
	b, err := CalculateProportionalRefundBreakdown(
		testPD, testS, testC, testK, 90_000, 0,
		0, 0, 0, 0,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// CashRefund = 90000+0-18000 = 72000
	assertFields(t, "product_only", b, 72_000, 18_000, 4_500, 85_500)
	assertCumulativeCash(t, "product_only", b, 72_000)
}

func TestS2C2_FiftyPercentProduct(t *testing.T) {
	// Rpd=45000, Rs=0
	b, err := CalculateProportionalRefundBreakdown(
		testPD, testS, testC, testK, 45_000, 0,
		0, 0, 0, 0,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// CashRefund = 45000+0-9000 = 36000
	// CoinDelta = floor(18000*45000/90000) = 9000
	// CommissionDelta = floor(4500*45000/90000) = 2250
	assertFields(t, "50pct_product", b, 36_000, 9_000, 2_250, 42_750)
	assertCumulativeCash(t, "50pct_product", b, 36_000)
}

func TestS2C2_ShippingOnly(t *testing.T) {
	// Rpd=0, Rs=20000
	b, err := CalculateProportionalRefundBreakdown(
		testPD, testS, testC, testK, 0, 20_000,
		0, 0, 0, 0,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// CashRefund = 0+20000-0 = 20000
	// CoinDelta = 0 (product refund is 0)
	// CommissionDelta = 0 (product refund is 0)
	assertFields(t, "shipping_only", b, 20_000, 0, 0, 20_000)
	assertCumulativeCash(t, "shipping_only", b, 20_000)
}

func TestS2C2_TwoFiftyPercentPartials(t *testing.T) {
	// Event 1: Rpd=45000, Rs=0
	b1, err := CalculateProportionalRefundBreakdown(
		testPD, testS, testC, testK, 45_000, 0,
		0, 0, 0, 0,
	)
	if err != nil {
		t.Fatalf("event1: %v", err)
	}
	assertFields(t, "event1", b1, 36_000, 9_000, 2_250, 42_750)
	assertCumulativeCash(t, "event1", b1, 36_000)

	// Event 2: Rpd=45000, Rs=0, cumulative state from event1
	b2, err := CalculateProportionalRefundBreakdown(
		testPD, testS, testC, testK, 45_000, 0,
		b1.CumProductRefundAfter, 0, b1.CumCoinsRestoredAfter, b1.CumCommissionReversedAfter,
	)
	if err != nil {
		t.Fatalf("event2: %v", err)
	}
	assertFields(t, "event2", b2, 36_000, 9_000, 2_250, 42_750)
	assertCumulativeCash(t, "event2", b2, 72_000)

	// Cumulative
	if b2.CumProductRefundAfter != 90_000 {
		t.Errorf("cumProduct total=%d want 90000", b2.CumProductRefundAfter)
	}
	if b2.CumCoinsRestoredAfter != 18_000 {
		t.Errorf("cumCoins total=%d want 18000", b2.CumCoinsRestoredAfter)
	}
	if b2.CumCommissionReversedAfter != 4_500 {
		t.Errorf("cumCommission total=%d want 4500", b2.CumCommissionReversedAfter)
	}
	// Total cash: 36000+36000 = 72000, coins: 18000, sum=90000 = PD
}

func TestS2C2_OverRefundRejected(t *testing.T) {
	// Rpd=91000 exceeds PD
	if _, err := CalculateProportionalRefundBreakdown(
		testPD, testS, testC, testK, 91_000, 0,
		0, 0, 0, 0,
	); err == nil {
		t.Fatal("expected over-refund error")
	}

	// Rs=21000 exceeds S
	if _, err := CalculateProportionalRefundBreakdown(
		testPD, testS, testC, testK, 0, 21_000,
		0, 0, 0, 0,
	); err == nil {
		t.Fatal("expected over-shipping-refund error")
	}

	// Cumulative exceeds PD
	if _, err := CalculateProportionalRefundBreakdown(
		testPD, testS, testC, testK, 50_000, 0,
		50_000, 0, 10_000, 2_500,
	); err == nil {
		t.Fatal("expected cumulative over-refund error")
	}
}

func TestS2C2_CorruptedCumulativeCoinsRejected(t *testing.T) {
	if _, err := CalculateProportionalRefundBreakdown(
		testPD, testS, testC, testK, 45_000, 0,
		45_000, 0, 9_999, 2_250,
	); err == nil {
		t.Fatal("expected corrupted cumulative coin state to be rejected")
	}
}

func TestS2C2_CorruptedCumulativeCommissionRejected(t *testing.T) {
	if _, err := CalculateProportionalRefundBreakdown(
		testPD, testS, testC, testK, 45_000, 0,
		45_000, 0, 9_000, 2_249,
	); err == nil {
		t.Fatal("expected corrupted cumulative commission state to be rejected")
	}
}

func TestS2C2_RoundingAcrossPartials(t *testing.T) {
	// Small numbers to test rounding: PD=100, K=50, C=33
	// Event 1: Rpd=33
	b1, err := CalculateProportionalRefundBreakdown(100, 0, 33, 50, 33, 0, 0, 0, 0, 0)
	if err != nil {
		t.Fatalf("event1: %v", err)
	}
	// floor(50*33/100)=16, floor(33*33/100)=10, cash=33-16=17
	if b1.CoinDelta != 16 {
		t.Errorf("coins1=%d want 16", b1.CoinDelta)
	}
	if b1.CommissionDelta != 10 {
		t.Errorf("comm1=%d want 10", b1.CommissionDelta)
	}

	// Event 2: Rpd=33
	b2, err := CalculateProportionalRefundBreakdown(100, 0, 33, 50, 33, 0,
		b1.CumProductRefundAfter, 0, b1.CumCoinsRestoredAfter, b1.CumCommissionReversedAfter)
	if err != nil {
		t.Fatalf("event2: %v", err)
	}
	// floor(50*66/100)=33, delta=33-16=17
	if b2.CoinDelta != 17 {
		t.Errorf("coins2=%d want 17", b2.CoinDelta)
	}

	// Event 3: Rpd=34 (final)
	b3, err := CalculateProportionalRefundBreakdown(100, 0, 33, 50, 34, 0,
		b2.CumProductRefundAfter, 0, b2.CumCoinsRestoredAfter, b2.CumCommissionReversedAfter)
	if err != nil {
		t.Fatalf("event3: %v", err)
	}
	// floor(50*100/100)=50, delta=50-33=17
	if b3.CoinDelta != 17 {
		t.Errorf("coins3=%d want 17", b3.CoinDelta)
	}
	// Cumulative coins: 16+17+17=50=K exactly
	if b3.CumCoinsRestoredAfter != 50 {
		t.Errorf("totalCoins=%d want 50", b3.CumCoinsRestoredAfter)
	}
	// Cumulative commission: 10+10+13=33=C exactly
	if b3.CumCommissionReversedAfter != 33 {
		t.Errorf("totalCommission=%d want 33", b3.CumCommissionReversedAfter)
	}
	assertCumulativeCash(t, "rounding", b3, 50)
}

func TestParseMidtransRefundAmount(t *testing.T) {
	cases := []struct {
		raw  string
		want int64
	}{
		{raw: "100000.00", want: 100_000},
		{raw: "10000.00", want: 10_000},
		{raw: "123", want: 123},
		{raw: "7", want: 7},
	}
	for _, tc := range cases {
		got, err := parseMidtransRefundAmount(tc.raw)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.raw, err)
		}
		if got != tc.want {
			t.Fatalf("%s: got %d want %d", tc.raw, got, tc.want)
		}
	}
}

func TestParseMidtransRefundAmount_RejectsNegativeAndInvalid(t *testing.T) {
	for _, raw := range []string{"", "-100.00", "not-a-number"} {
		if _, err := parseMidtransRefundAmount(raw); err == nil {
			t.Errorf("%q: expected error, got nil", raw)
		}
	}
}
