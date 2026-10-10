package serverboot

import (
	"os"
	"strings"
	"testing"

	paymentmethodentity "github.com/hishumi/backend/internal/commerce/paymentmethod/entity"
	"github.com/hishumi/backend/pkg/money"
)

// funcBodyFrom returns the source of the function whose signature starts with
// signature, up to the next top-level `func ` declaration. It is the same
// bounded-source technique used by the payment wire-contract tests: it pins a
// contract that a behavior test cannot reach without a live DB.
func funcBodyFrom(t *testing.T, src, signature string) string {
	t.Helper()
	start := strings.Index(src, signature)
	if start < 0 {
		t.Fatalf("function %q not found", signature)
	}
	body := src[start:]
	if next := strings.Index(body[len(signature):], "\nfunc "); next >= 0 {
		body = body[:len(signature)+next]
	}
	return body
}

// TestBuildCanonicalPaymentMethodOptions_MoneyIntegrity proves the shared
// pre-order/order fee builder is backend money authority: fee is always
// CalculateFee(cash, method) and final payable is always cash + fee. A method
// with an invalid fee formula is skipped (never silently priced at zero).
func TestBuildCanonicalPaymentMethodOptions_MoneyIntegrity(t *testing.T) {
	cash := money.New(100_000)
	flat := paymentmethodentity.Method{
		Code:        "bank_transfer",
		DisplayName: "Transfer Bank",
		Enabled:     true,
		FeeType:     paymentmethodentity.FeeTypeFlat,
		FlatAmount:  money.New(4_000),
	}
	percent := paymentmethodentity.Method{
		Code:        "qris",
		DisplayName: "QRIS",
		Enabled:     true,
		FeeType:     paymentmethodentity.FeeTypePercent,
		PercentBps:  200, // 2%
	}
	broken := paymentmethodentity.Method{
		Code:    "broken",
		Enabled: true,
		FeeType: paymentmethodentity.FeeType("not_a_real_fee_type"),
	}

	var invalid []string
	opts := buildCanonicalPaymentMethodOptions(
		cash,
		[]paymentmethodentity.Method{flat, percent, broken},
		func(code string, _ error) { invalid = append(invalid, code) },
	)

	if len(opts) != 2 {
		t.Fatalf("want 2 priced methods, got %d", len(opts))
	}
	if opts[0].MethodCode != "bank_transfer" || opts[0].BuyerPaymentFeeAmount != 4_000 || opts[0].FinalPayableAmount != 104_000 {
		t.Fatalf("flat method: got %+v, want fee=4000 final=104000", opts[0])
	}
	if opts[1].MethodCode != "qris" || opts[1].BuyerPaymentFeeAmount != 2_000 || opts[1].FinalPayableAmount != 102_000 {
		t.Fatalf("percent method: got %+v, want fee=2000 final=102000", opts[1])
	}
	if len(invalid) != 1 || invalid[0] != "broken" {
		t.Fatalf("invalid fee formula must be reported once, got %v", invalid)
	}
}

// TestBuildCanonicalPaymentMethodOptions_UsesCanonicalFeeAuthority pins that the
// shared builder never re-derives a fee: it calls the one canonical
// paymentmethodentity.CalculateFee and adds the result to the caller's cash.
func TestBuildCanonicalPaymentMethodOptions_UsesCanonicalFeeAuthority(t *testing.T) {
	src, err := os.ReadFile("dependencies.go")
	if err != nil {
		t.Fatalf("read dependencies.go: %v", err)
	}
	body := funcBodyFrom(t, string(src), "func buildCanonicalPaymentMethodOptions(")

	if !strings.Contains(body, "paymentmethodentity.CalculateFee(") {
		t.Error("shared method-option builder must use the canonical paymentmethodentity.CalculateFee")
	}
	if !strings.Contains(body, "cash.Add(fee)") {
		t.Error("final payable must be cash + fee (fee-on-base, never fee-on-fee)")
	}
}

// TestPreOrderPaymentMethods_ReadOnlyCanonicalContract proves the Phase 1
// pre-order pricing disclosure is the canonical, read-only contract that makes
// the final payable amount knowable before any durable order exists.
func TestPreOrderPaymentMethods_ReadOnlyCanonicalContract(t *testing.T) {
	src, err := os.ReadFile("dependencies.go")
	if err != nil {
		t.Fatalf("read dependencies.go: %v", err)
	}
	body := funcBodyFrom(t, string(src), "func (h *CorePaymentHandler) ListPreOrderPaymentMethods(")

	// Positive: the obligation is identified by the pricing token, its
	// lifecycle is validated server-side, and K/fee come from canonical
	// authorities.
	for _, want := range []string{
		`c.Query("pricing_token")`,
		"GetSnapshot(",
		"token.UserID != userID",
		"token.IsUsed",
		"token.IsExpired()",
		"coinsApp.ResolveOrderRedemption(",
		"buildCanonicalPaymentMethodOptions(",
		`"final_payable_amount"`,
		`"escrow_amount"`,
		`"cash_amount"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("pre-order disclosure is missing %q", want)
		}
	}

	// Negative: read-only. It must never create an order, create a payment, or
	// reach the gateway — the pricing token is consumed only by POST /orders.
	for _, forbidden := range []string{
		"h.orderRepo.CreateOrderTx(",
		"h.orderService.",
		"h.paymentRepo.CreatePayment(",
		"createMidtransTransaction(",
		"UpdatePaymentSelectionTx(",
	} {
		if strings.Contains(body, forbidden) {
			t.Errorf("pre-order disclosure must be read-only, but it references %q", forbidden)
		}
	}

	// Semantic boundary: this surface must emit the post-fee final amount and
	// must NOT reuse the ambiguous pre-fee `total_payable_amount` key.
	if strings.Contains(body, `"total_payable_amount"`) {
		t.Error("pre-order disclosure must emit final_payable_amount, never total_payable_amount")
	}
}

// TestPreOrderPaymentMethods_RouteRegistered proves the canonical pre-order
// disclosure is actually reachable at GET /api/v1/payments/pre-order-methods.
func TestPreOrderPaymentMethods_RouteRegistered(t *testing.T) {
	raw, err := os.ReadFile("../../cmd/core_server/routes_core.go")
	if err != nil {
		t.Fatalf("read routes_core.go: %v", err)
	}
	if !strings.Contains(
		string(raw),
		`paymentRoutes.GET("/pre-order-methods", deps.PaymentHandler.ListPreOrderPaymentMethods)`,
	) {
		t.Error("GET /payments/pre-order-methods is not registered to ListPreOrderPaymentMethods")
	}
}

// TestCreatePayment_RejectsMethodIdentityMismatch proves the payment endpoint
// enforces the pre-order method binding by IDENTITY — not merely by fee — so
// two methods with the same fee (including two zero-fee methods) can never be
// substituted for the method the buyer selected.
func TestCreatePayment_RejectsMethodIdentityMismatch(t *testing.T) {
	src, err := os.ReadFile("dependencies.go")
	if err != nil {
		t.Fatalf("read dependencies.go: %v", err)
	}
	body := funcBodyFrom(t, string(src), "func (h *CorePaymentHandler) CreatePayment(")

	if !strings.Contains(body, "order.PaymentMethodCode != nil") {
		t.Error("CreatePayment must read the order's bound payment method identity")
	}
	if !strings.Contains(body, "req.PaymentMethodCode != *order.PaymentMethodCode") {
		t.Error("CreatePayment must reject a requested method that differs by IDENTITY from the order's bound method")
	}
	if !strings.Contains(body, `"Payment method does not match the method selected at checkout"`) {
		t.Error("CreatePayment mismatch rejection copy is missing")
	}
}

// TestPreOrderPaymentMethods_SingleFeeAuthority proves there is exactly one
// pre-order fee producer: both the pre-order and post-order disclosures build
// their options through the same shared builder. A second inline
// CalculateFee loop on the pre-order path would be a competing authority.
func TestPreOrderPaymentMethods_SingleFeeAuthority(t *testing.T) {
	src, err := os.ReadFile("dependencies.go")
	if err != nil {
		t.Fatalf("read dependencies.go: %v", err)
	}
	code := string(src)

	preOrderBody := funcBodyFrom(t, code, "func (h *CorePaymentHandler) ListPreOrderPaymentMethods(")
	if strings.Contains(preOrderBody, "CalculateFee(") {
		t.Error("pre-order disclosure must call the shared builder, not CalculateFee directly")
	}

	postOrderBody := funcBodyFrom(t, code, "func (h *CorePaymentHandler) ListPaymentMethods(")
	if !strings.Contains(postOrderBody, "buildCanonicalPaymentMethodOptions(") {
		t.Error("post-order disclosure must share the canonical method-option builder")
	}
	// The pre-refactor inline per-method loop called
	// CalculateFee(cashAmount, m) directly. Its absence proves the post-order
	// path now shares the one builder instead of owning a second fee loop.
	if strings.Contains(postOrderBody, "paymentmethodentity.CalculateFee(cashAmount, m)") {
		t.Error("post-order disclosure must not re-derive a fee inline")
	}
}
