package application

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/labuda/backend/pkg/money"
)

// FIN-R01E-D — POST /pricing/preview money contract.
//
// The preview snapshot must emit NUMERIC money fields, not `{}`. money.Money is
// a struct with an unexported int64 and no json.Marshaler, so it serializes to
// `{}` unless the snapshot marshals its amounts explicitly. Regression: the
// preview emitted `{}` for every amount, so the mobile preview parser silently
// produced 0 for subtotal/shipping/total_payable/escrow.
//
// This also proves the semantic distinction the pricing token owns:
//
//	escrow_amount        = (P−D)+S            — buyer gross escrow, before fee
//	total_payable_amount = escrow + service   — buyer payable after fee
//
// Both are canonical pricing_tokens columns. `total_before_coins_amount` is the
// persisted ORDER column name and MUST NOT appear in the preview snapshot.
func TestPricingSnapshot_WireContract(t *testing.T) {
	snap := PricingSnapshot{
		UnitPrice:          money.New(100000),
		Quantity:           1,
		Subtotal:           money.New(100000),
		ShippingTotal:      money.New(10000),
		CommissionPercent:  5,
		CommissionAmount:   money.New(4250),
		DiscountAmount:     money.New(15000),
		ServiceFeeAmount:   money.New(5000),
		TotalPayableAmount: money.New(100000), // escrow (95000) + fee (5000)
		EscrowAmount:       money.New(95000),  // (P−D)+S
		ShippingMode:       "standard",
	}

	raw, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal pricing snapshot: %v", err)
	}
	body := string(raw)

	// Every money field must be a JSON number, not an object.
	for _, want := range []string{
		`"unit_price":100000`,
		`"subtotal":100000`,
		`"shipping_total":10000`,
		`"commission_amount":4250`,
		`"discount_amount":15000`,
		`"service_fee_amount":5000`,
		`"total_payable_amount":100000`,
		`"escrow_amount":95000`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("MISSING numeric preview money field %s (got %s)", want, body)
		}
	}
	if strings.Contains(body, `":{}`) {
		t.Fatalf("REGRESSION: preview snapshot must not serialize money fields as objects (got %s)", body)
	}
	if strings.Contains(body, "total_before_coins_amount") {
		t.Fatalf("REGRESSION: preview snapshot must not emit the persisted ORDER column name total_before_coins_amount (got %s)", body)
	}
}
