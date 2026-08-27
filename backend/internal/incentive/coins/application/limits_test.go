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
