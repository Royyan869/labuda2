package application

import "testing"

func TestMaxCoinsAllowedForDiscountedProduct(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name              string
		discountedProduct int64
		want              int64
	}{
		{name: "positive_pd", discountedProduct: 90000, want: 18000},
		{name: "zero_pd", discountedProduct: 0, want: 0},
		{name: "negative_pd", discountedProduct: -1, want: 0},
		{name: "flooring", discountedProduct: 9999, want: 1999},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := MaxCoinsAllowedForDiscountedProduct(tc.discountedProduct); got != tc.want {
				t.Fatalf("MaxCoinsAllowedForDiscountedProduct(%d) = %d, want %d", tc.discountedProduct, got, tc.want)
			}
		})
	}
}

// TestResolveOrderRedemption pins the single order-time coin authority: the
// buyer's `use_coins` intent is resolved into K exactly once, at Order creation,
// as min(live balance, token.MaxCoinsAllowed). Downstream payment derives K from
// the persisted token snapshot and has no independent coin authority.
func TestResolveOrderRedemption(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		availableBalance int64
		maxCoinsAllowed  int64
		useCoins         bool
		want             int64
	}{
		// Coins are optional: no intent means no redemption.
		{name: "intent_false_is_zero_even_with_balance", availableBalance: 50000, maxCoinsAllowed: 18000, useCoins: false, want: 0},
		// Ceiling binds when the buyer holds more than 20% of PD.
		{name: "ceiling_binds", availableBalance: 50000, maxCoinsAllowed: 18000, useCoins: true, want: 18000},
		// Balance binds when the buyer holds less than the ceiling.
		{name: "balance_binds", availableBalance: 12000, maxCoinsAllowed: 18000, useCoins: true, want: 12000},
		// Exact balance == ceiling.
		{name: "balance_equals_ceiling", availableBalance: 18000, maxCoinsAllowed: 18000, useCoins: true, want: 18000},
		// Zero balance can never fund a redemption.
		{name: "zero_balance", availableBalance: 0, maxCoinsAllowed: 18000, useCoins: true, want: 0},
		// A zero ceiling (e.g. coins disabled for the order) yields zero.
		{name: "zero_ceiling", availableBalance: 18000, maxCoinsAllowed: 0, useCoins: true, want: 0},
		// Defensive: a negative balance can never become a negative K.
		{name: "negative_balance_clamped", availableBalance: -1, maxCoinsAllowed: 18000, useCoins: true, want: 0},
		// Defensive: a negative ceiling can never become a negative K.
		{name: "negative_ceiling_clamped", availableBalance: 18000, maxCoinsAllowed: -1, useCoins: true, want: 0},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ResolveOrderRedemption(tc.availableBalance, tc.maxCoinsAllowed, tc.useCoins)
			if got != tc.want {
				t.Fatalf(
					"ResolveOrderRedemption(balance=%d, max=%d, useCoins=%v) = %d, want %d",
					tc.availableBalance, tc.maxCoinsAllowed, tc.useCoins, got, tc.want,
				)
			}
			if got < 0 {
				t.Fatalf("K must never be negative, got %d", got)
			}
			if tc.want > 0 && got > tc.maxCoinsAllowed {
				t.Fatalf("K must never exceed maxCoinsAllowed (%d), got %d", tc.maxCoinsAllowed, got)
			}
			if tc.want > 0 && got > tc.availableBalance {
				t.Fatalf("K must never exceed the buyer's balance (%d), got %d", tc.availableBalance, got)
			}
		})
	}
}
