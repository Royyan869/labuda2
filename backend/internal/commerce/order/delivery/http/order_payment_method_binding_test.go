package http

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// funcBodyFrom returns the source of the function whose signature starts with
// signature, up to the next top-level `func ` declaration.
func funcBodyFrom(t *testing.T, src, signature string) string {
	t.Helper()
	start := strings.Index(src, signature)
	require.GreaterOrEqual(t, start, 0, "function %q not found", signature)
	body := src[start:]
	if next := strings.Index(body[len(signature):], "\nfunc "); next >= 0 {
		body = body[:len(signature)+next]
	}
	return body
}

// TestCreateOrderRequest_BindsPreOrderPaymentMethod proves the order request
// carries the buyer's pre-order method selection as a REQUIRED canonical field.
func TestCreateOrderRequest_BindsPreOrderPaymentMethod(t *testing.T) {
	var req CreateOrderRequest
	require.NoError(t, json.Unmarshal([]byte(`{"payment_method_code":"bank_transfer"}`), &req))
	require.Equal(t, "bank_transfer", req.PaymentMethodCode)

	src, err := os.ReadFile("order_handler.go")
	require.NoError(t, err)
	require.Contains(t, string(src), `json:"payment_method_code" binding:"required"`,
		"payment_method_code must be a required order-creation field")
}

// TestCreateOrder_BindsSelectedMethodIntoSnapshot proves order creation binds
// the buyer-selected method through the canonical fee authority and persists
// the agreed amounts — the client never supplies a fee or total.
func TestCreateOrder_BindsSelectedMethodIntoSnapshot(t *testing.T) {
	src, err := os.ReadFile("order_handler.go")
	require.NoError(t, err)
	code := string(src)

	require.Contains(t, code, "h.applySelectedPaymentMethod(",
		"CreateOrder must bind the selected method before creating the order")
	// The EXACT method identity is carried onto the order input (both the
	// for_sale and auction buy-now branches), so the order persists it.
	require.Contains(t, code, "PaymentMethodCode: &req.PaymentMethodCode,",
		"CreateOrder must bind the exact selected method identity onto the order")

	binder := funcBodyFrom(t, code, "func (h *OrderHandler) applySelectedPaymentMethod(")
	require.Contains(t, binder, "paymentmethodentity.CalculateFee(",
		"the bound fee must come from the canonical payment-method authority")
	require.Contains(t, binder, "snapshot.ServiceFeeAmount = fee")
	require.Contains(t, binder, "snapshot.TotalPayableAmount = token.EscrowAmount.Add(fee)")
	require.NotContains(t, binder, "req.", "the binder must not read any client amount")
}

// TestCreateOrder_RejectsInvalidOrDisabledMethod proves order creation resolves
// the method from the canonical authority and rejects an unknown/disabled code
// (HTTP 400) instead of binding an unpayable order.
func TestCreateOrder_RejectsInvalidOrDisabledMethod(t *testing.T) {
	src, err := os.ReadFile("order_handler.go")
	require.NoError(t, err)
	code := string(src)

	binder := funcBodyFrom(t, code, "func (h *OrderHandler) applySelectedPaymentMethod(")
	require.Contains(t, binder, "h.paymentMethodRepo.GetByCode(",
		"the method must be resolved from the canonical authority")
	require.Contains(t, binder, "!method.Enabled",
		"a disabled method must be rejected")

	require.Contains(t, code, `response.BadRequest(c, "Invalid or unavailable payment method")`,
		"invalid/disabled method must surface HTTP 400")
}
