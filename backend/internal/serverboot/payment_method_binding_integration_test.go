//go:build integration

package serverboot

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/stretchr/testify/require"
)

// seedBindingPaymentMethods adds methods needed to prove the EXACT identity
// invariant, including two methods with an identical (non-zero) fee and two
// zero-fee methods.
func seedBindingPaymentMethods(t *testing.T, h *paymentIntentHarness) {
	t.Helper()
	require.NoError(t, h.tdb.WithTx(context.Background(), func(tx db.Tx) error {
		_, err := tx.Exec(context.Background(), `
			INSERT INTO payment_methods
				(method_code, display_name, enabled, fee_type, flat_amount_rupiah, percent_bps,
				 midtrans_channels, sort_order, rate_source, rate_source_note)
			VALUES
				('flat_4000_b', 'Flat 4000 B', true, 'flat', 4000, 0,
					ARRAY['other_va'], 30, 'public_baseline', 'test seed'),
				('zero_a', 'Zero A', true, 'flat', 0, 0,
					ARRAY['other_va'], 40, 'public_baseline', 'test seed'),
				('zero_b', 'Zero B', true, 'flat', 0, 0,
					ARRAY['other_va'], 50, 'public_baseline', 'test seed')
			ON CONFLICT (method_code) DO NOTHING
		`)
		return err
	}))
}

// createBoundOrder inserts an order whose canonical payment-method binding is
// methodCode (or nil for an unbound / auction bid-win order).
func createBoundOrder(t *testing.T, h *paymentIntentHarness, methodCode *string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	tokenID := seedPaymentIntentPricingToken(t, ctx, h.tdb, h.buyerID, canonicalShipping, canonicalBuyerBase)

	order := orderentity.NewOrderFromSource(
		h.buyerID,
		h.sellerID,
		orderentity.OrderSourceForSale,
		uuid.New(),
		nil,
		1,
		money.New(100000),
		money.New(100000),
		money.New(canonicalShipping),
		5,
		money.New(4500),
		money.New(0),
		money.New(canonicalBuyerBase),
		nil,
		"JNE Reguler",
		"reguler",
		"1_3_days",
		nil,
		nil,
		nil,
		&tokenID,
		time.Now().Add(1*time.Hour),
	)
	order.PaymentMethodCode = methodCode

	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		return h.orderRepo.CreateOrderTx(ctx, tx, order)
	}))
	return order.ID
}

func loadOrderPaymentMethodCode(t *testing.T, h *paymentIntentHarness, orderID uuid.UUID) *string {
	t.Helper()
	var code *string
	require.NoError(t, h.tdb.WithTx(context.Background(), func(tx db.Tx) error {
		return tx.QueryRow(context.Background(),
			`SELECT payment_method_code FROM orders WHERE id = $1`, orderID).Scan(&code)
	}))
	return code
}

// TestOrderPaymentMethodBinding_Persisted proves the exact selected method is
// persisted on the order and read back by the canonical reader.
func TestOrderPaymentMethodBinding_Persisted(t *testing.T) {
	h := newPaymentIntentHarness(t, 0, &recordingSnapGateway{}, nil)
	seedBindingPaymentMethods(t, h)

	method := "bank_transfer"
	orderID := createBoundOrder(t, h, &method)

	// Persistence proof (raw SQL).
	require.NotNil(t, loadOrderPaymentMethodCode(t, h, orderID))
	require.Equal(t, "bank_transfer", *loadOrderPaymentMethodCode(t, h, orderID))

	// Canonical reader proof.
	var order *orderentity.Order
	require.NoError(t, h.tdb.WithTx(context.Background(), func(tx db.Tx) error {
		var err error
		order, err = h.orderRepo.GetByID(context.Background(), tx, orderID)
		return err
	}))
	require.NotNil(t, order.PaymentMethodCode)
	require.Equal(t, "bank_transfer", *order.PaymentMethodCode)
}

// TestPaymentMethodEnforcement_ExactIdentity is the DB-backed proof of the
// business invariant: the method requested by POST /payments must be the EXACT
// method bound to the order — equal fees do NOT make methods interchangeable.
func TestPaymentMethodEnforcement_ExactIdentity(t *testing.T) {
	h := newPaymentIntentHarness(t, 0, &recordingSnapGateway{}, nil)
	seedBindingPaymentMethods(t, h)

	// Case B — payment matches the bound method → accepted.
	bank := "bank_transfer"
	orderB := createBoundOrder(t, h, &bank)
	respB := h.runCreatePayment(t, orderB, "bank_transfer", 0)
	require.Equal(t, http.StatusOK, respB.Code,
		"payment with the bound method must be accepted: %s", respB.Body.String())

	// Case C — a different method → rejected.
	orderC := createBoundOrder(t, h, &bank)
	respC := h.runCreatePayment(t, orderC, "qris", 0)
	require.Equal(t, http.StatusConflict, respC.Code,
		"payment with a different method must be rejected: %s", respC.Body.String())

	// Case D — same fee (4000), different method → MUST be rejected.
	orderD := createBoundOrder(t, h, &bank)
	respD := h.runCreatePayment(t, orderD, "flat_4000_b", 0)
	require.Equal(t, http.StatusConflict, respD.Code,
		"same-fee different method must be rejected: %s", respD.Body.String())

	// Case E — zero-fee bound method vs another zero-fee method → MUST be rejected.
	zeroA := "zero_a"
	orderE := createBoundOrder(t, h, &zeroA)
	respE := h.runCreatePayment(t, orderE, "zero_b", 0)
	require.Equal(t, http.StatusConflict, respE.Code,
		"zero-fee different method must be rejected: %s", respE.Body.String())

	// Bound zero-fee method used with its own identity → accepted.
	orderE2 := createBoundOrder(t, h, &zeroA)
	respE2 := h.runCreatePayment(t, orderE2, "zero_a", 0)
	require.Equal(t, http.StatusOK, respE2.Code,
		"bound zero-fee method with its own identity must be accepted: %s", respE2.Body.String())

	// Unbound order (auction bid-win) → first payment binds the method.
	orderF := createBoundOrder(t, h, nil)
	require.Nil(t, loadOrderPaymentMethodCode(t, h, orderF))
	respF := h.runCreatePayment(t, orderF, "bank_transfer", 0)
	require.Equal(t, http.StatusOK, respF.Code,
		"unbound order must accept the first payment method: %s", respF.Body.String())
	require.Equal(t, "bank_transfer", *loadOrderPaymentMethodCode(t, h, orderF),
		"unbound order must bind the method used at first payment")

	// A second payment on the now-bound order using a different method → rejected.
	respF2 := h.runCreatePayment(t, orderF, "qris", 0)
	require.Equal(t, http.StatusConflict, respF2.Code,
		"once bound, a different method must be rejected: %s", respF2.Body.String())
}
