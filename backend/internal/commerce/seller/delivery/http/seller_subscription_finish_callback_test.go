package http

import (
	"os"
	"strings"
	"testing"
)

// TestSellerHandler_SubscriptionFinishCallbackContract locks the Snap
// callbacks.finish contract that the mobile PaymentWebviewScreen relies on:
// BOTH branches of initiateSubscriptionPaymentTx (fresh payment + idempotent
// reuse) must set `callbacks.finish = {frontendURL}/payment/finish`. The
// webview auto-closes on that path — if a branch stops sending it, the user is
// stranded on the Snap result page (or worse, the old dead-route error page).
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

	// Function body ends at the next top-level func declaration.
	rest := content[fnIdx+len(fnMarker):]
	nextFunc := strings.Index(rest, "\nfunc ")
	if nextFunc < 0 {
		nextFunc = len(rest)
	}
	body := rest[:nextFunc]

	const finishAssignment = `Finish: h.frontendURL + "/payment/finish"`
	canonical := strings.Count(body, finishAssignment)
	if canonical != 2 {
		t.Fatalf("want exactly 2 canonical finish callback assignments (fresh + reuse branch) in initiateSubscriptionPaymentTx, got %d", canonical)
	}
}
