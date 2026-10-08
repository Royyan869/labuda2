package http

import (
	"os"
	"strings"
	"testing"
)

// TestSellerHandler_SubscriptionFinishCallbackContract locks the Snap
// callbacks.finish delegation contract that the mobile PaymentWebviewScreen
// relies on: subscription initiation must NOT build the finish callback itself.
// It must delegate Snap creation to the ONE canonical SnapService, whose
// builder injects `{frontendURL}/payment/finish?order_id=...` for every flow.
// The canonical builder contract is proven in
// internal/integration/payment/application (TestBuildSnapRequest_FinishCallbackBuiltFromFrontendURL).
func TestSellerHandler_SubscriptionFinishCallbackContract(t *testing.T) {
	raw, err := os.ReadFile("seller_handler.go")
	if err != nil {
		t.Fatalf("failed to read seller_handler.go: %v", err)
	}
	content := string(raw)

	const fnMarker = "func (h *SellerHandler) initiateSubscriptionPaymentTx"
	fnIdx := strings.Index(content, fnMarker)
	if fnIdx < 0 {
		t.Fatal("initiateSubscriptionPaymentTx function not found — subscription payment logic may have moved")
	}

	rest := content[fnIdx+len(fnMarker):]
	nextFunc := strings.Index(rest, "\nfunc ")
	if nextFunc < 0 {
		nextFunc = len(rest)
	}
	body := rest[:nextFunc]

	if strings.Contains(body, `Finish: h.frontendURL`) {
		t.Fatal("initiateSubscriptionPaymentTx must not build the Snap finish callback; the canonical SnapService owns it")
	}
	if !strings.Contains(body, "h.createSubscriptionSnapSession(") {
		t.Fatal("initiateSubscriptionPaymentTx must delegate Snap creation to the canonical helper")
	}
	if !strings.Contains(content, "h.snapService.CreateSession(") {
		t.Fatal("subscription Snap creation must go through the canonical SnapService")
	}
}
