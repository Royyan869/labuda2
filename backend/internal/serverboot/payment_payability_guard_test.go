package serverboot

import (
	"os"
	"strings"
	"testing"
)

// TestPaymentPayabilityGuard_PositionedBeforeMidtrans proves that the
// CreatePayment handler rejects non-payable orders BEFORE any payment row
// INSERT or Midtrans Snap API call.
//
// The test reads dependencies.go source and verifies:
//   1. The status guard (StatusPending check) exists in CreatePayment.
//   2. The expiry guard (PaymentExpiresAt check) exists in CreatePayment.
//   3. Both guards appear BEFORE the first paymentRepo.CreatePayment call.
//   4. Both guards appear BEFORE the first createMidtransTransaction call.
//
// This is the same source-grep pattern used by TestN4A3_FinalBypassCount_Zero
// in notification_worker_test.go.
func TestPaymentPayabilityGuard_PositionedBeforeMidtrans(t *testing.T) {
	src, err := os.ReadFile("dependencies.go")
	if err != nil {
		t.Fatalf("read dependencies.go: %v", err)
	}
	code := string(src)

	// Locate the CreatePayment function body start.
	funcStart := strings.Index(code, "func (h *CorePaymentHandler) CreatePayment(")
	if funcStart < 0 {
		t.Fatal("CreatePayment handler not found in dependencies.go")
	}
	body := code[funcStart:]

	// --- Guard 1: order status must be pending_payment ---
	statusGuard := strings.Index(body, "order.Status != orderEntity.StatusPending")
	if statusGuard < 0 {
		t.Fatal("MISSING: order status guard (order.Status != orderEntity.StatusPending) not found in CreatePayment")
	}

	// --- Guard 2: payment window must not be expired ---
	expiryGuard := strings.Index(body, "time.Now().After(order.PaymentExpiresAt)")
	if expiryGuard < 0 {
		t.Fatal("MISSING: expiry guard (time.Now().After(order.PaymentExpiresAt)) not found in CreatePayment")
	}

	// --- Payment row INSERT site ---
	paymentInsert := strings.Index(body, "h.paymentRepo.CreatePayment(")
	if paymentInsert < 0 {
		t.Fatal("paymentRepo.CreatePayment call not found in CreatePayment (structure changed?)")
	}

	// --- Midtrans call site ---
	midtransCall := strings.Index(body, "h.createMidtransTransaction(")
	if midtransCall < 0 {
		t.Fatal("createMidtransTransaction call not found in CreatePayment (structure changed?)")
	}

	// --- Position assertions: guards must precede both payment write sites ---
	if statusGuard >= paymentInsert {
		t.Errorf("REGRESSION: status guard (pos %d) appears AFTER paymentRepo.CreatePayment (pos %d); "+
			"non-payable orders can reach payment INSERT", statusGuard, paymentInsert)
	}
	if statusGuard >= midtransCall {
		t.Errorf("REGRESSION: status guard (pos %d) appears AFTER createMidtransTransaction (pos %d); "+
			"non-payable orders can reach Midtrans", statusGuard, midtransCall)
	}
	if expiryGuard >= paymentInsert {
		t.Errorf("REGRESSION: expiry guard (pos %d) appears AFTER paymentRepo.CreatePayment (pos %d); "+
			"expired orders can reach payment INSERT", expiryGuard, paymentInsert)
	}
	if expiryGuard >= midtransCall {
		t.Errorf("REGRESSION: expiry guard (pos %d) appears AFTER createMidtransTransaction (pos %d); "+
			"expired orders can reach Midtrans", expiryGuard, midtransCall)
	}

	// --- Verify response semantics ---
	conflictResp := strings.Index(body, `response.Conflict(c,`)
	if conflictResp < 0 {
		t.Fatal("MISSING: response.Conflict call for status guard rejection")
	}
	if conflictResp < statusGuard || conflictResp > paymentInsert {
		t.Error("response.Conflict is not between status guard and payment INSERT")
	}

	goneResp := strings.Index(body, `response.Gone(c,`)
	if goneResp < 0 {
		t.Fatal("MISSING: response.Gone call for expiry guard rejection")
	}
	if goneResp < expiryGuard || goneResp > paymentInsert {
		t.Error("response.Gone is not between expiry guard and payment INSERT")
	}
}

// TestPaymentReuseGuard_HandlesLookupErrors verifies that the existing-payment
// lookup fails closed on real DB errors but still allows the no-rows case to
// proceed to a fresh payment creation.
func TestPaymentReuseGuard_HandlesLookupErrors(t *testing.T) {
	src, err := os.ReadFile("dependencies.go")
	if err != nil {
		t.Fatalf("read dependencies.go: %v", err)
	}
	code := string(src)
	funcStart := strings.Index(code, "func (h *CorePaymentHandler) CreatePayment(")
	if funcStart < 0 {
		t.Fatal("CreatePayment handler not found in dependencies.go")
	}
	body := code[funcStart:]

	if !strings.Contains(body, `err.Error() != "no rows in result set"`) {
		t.Fatal("reuse guard must treat no rows in result set as a non-fatal miss")
	}
	if !strings.Contains(body, `response.InternalServerError(c, "Failed to verify existing payment")`) {
		t.Fatal("reuse guard must fail closed with an internal error when the existing payment lookup fails")
	}
	if strings.Contains(body, "existingPayment.IsPending()") {
		t.Fatal("reuse guard must not require existingPayment.IsPending(); non-expired active payments should be reused regardless of status")
	}
	if !strings.Contains(body, `"status":               existingPayment.Status`) {
		t.Fatal("reuse response must include the existing payment status")
	}
}
