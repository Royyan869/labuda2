package shared

import "testing"

// The canonical PRODUCT-LEVEL negotiation attribute surfaced on the generic
// resource projection: negotiationEnabled && active && quantityAvailable > 0.
// It is viewer-independent — the derivation has no viewer input at all.
func TestForSaleNegotiationEnabled_ProductStateMatrix(t *testing.T) {
	cases := []struct {
		name               string
		status             string
		quantityAvailable  int
		negotiationEnabled bool
		want               bool
	}{
		{"active negotiable in stock", "active", 3, true, true},
		{"active not negotiable", "active", 3, false, false},
		{"active out of stock", "active", 0, true, false},
		{"sold negotiable", "sold", 3, true, false},
		{"withdrawn negotiable", "withdrawn", 3, true, false},
		{"unavailable negotiable", "unavailable", 3, true, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ForSaleNegotiationEnabled(tc.status, tc.quantityAvailable, tc.negotiationEnabled); got != tc.want {
				t.Fatalf("ForSaleNegotiationEnabled(%q, %d, %v) = %v, want %v",
					tc.status, tc.quantityAvailable, tc.negotiationEnabled, got, tc.want)
			}
		})
	}
}
