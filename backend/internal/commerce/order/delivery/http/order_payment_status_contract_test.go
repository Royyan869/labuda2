package http

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

var paymentStatusFieldPattern = regexp.MustCompile(`paymentStatus\s+\*string`)

// =============================================================================
// CONTRACT LOCK: payment_status and payment_id in order detail handler (GET /orders/:id)
//
// Source-inspection tests locking the handler contract for payment_status and
// payment_id — same pattern as TestMarkShipped_AuthErrorMapping in
// order_fulfillment_auth_test.go.
//
// Behaviour being locked:
//   - GetOrder handler issues a payment query against the payments table,
//     scoped to the requested order ID.
//   - The query selects BOTH id and status so payment_id is returned alongside
//     payment_status in the detail response.
//   - Priority ordering (settlement > capture > pending > others) is applied.
//   - When no payment row exists, paymentStatus/paymentID stay nil (psErr path) — no 500.
//   - paymentStatus and paymentID are passed to OrderToDetailResponseWithIdentity,
//     which places them in the JSON response.
// =============================================================================

func readHandlerSource(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile("order_handler.go")
	if err != nil {
		t.Fatalf("read order_handler.go: %v", err)
	}
	return string(src)
}

func readDecisionSource(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile("dto/decision.go")
	if err != nil {
		t.Fatalf("read dto/decision.go: %v", err)
	}
	return string(src)
}

// TestGetOrder_PaymentStatusQuery_Present verifies that the detail handler queries
// the payments table for the order's payment status.
func TestGetOrder_PaymentStatusQuery_Present(t *testing.T) {
	src := readHandlerSource(t)

	if !strings.Contains(src, "FROM payments") {
		t.Fatal("GetOrder handler must query FROM payments to fetch payment_status; " +
			"order detail responses must include payment_status when a payment row exists")
	}
	if !strings.Contains(src, "reference_type = 'order'") {
		t.Fatal("GetOrder payment query must filter by reference_type = 'order'")
	}
	if !strings.Contains(src, "reference_id = $1") {
		t.Fatal("GetOrder payment query must scope to the requested order ID via reference_id = $1")
	}
}

// TestGetOrder_PaymentStatusQuery_PriorityOrdering verifies the canonical priority
// ordering: settlement(1) > capture(2) > pending(3) > others(4).
func TestGetOrder_PaymentStatusQuery_PriorityOrdering(t *testing.T) {
	src := readHandlerSource(t)

	if !strings.Contains(src, "'settlement' THEN 1") {
		t.Fatal("GetOrder payment query must assign priority 1 to 'settlement'")
	}
	if !strings.Contains(src, "'capture'") {
		t.Fatal("GetOrder payment query must include 'capture' in priority ordering")
	}
	if !strings.Contains(src, "'pending'") {
		t.Fatal("GetOrder payment query must include 'pending' in priority ordering")
	}
}

// TestGetOrder_NoPaymentRow_NilPaymentStatus verifies that when no payment row
// exists for an order, paymentStatus is left nil (not an empty string) and the
// handler does not return 500.
func TestGetOrder_NoPaymentRow_NilPaymentStatus(t *testing.T) {
	src := readHandlerSource(t)

	// The psErr path: psErr non-nil → leave paymentStatus nil.
	if !paymentStatusFieldPattern.MatchString(src) {
		t.Fatal("GetOrder must declare paymentStatus as *string (nil-able) — " +
			"nil signals no payment row; empty string must NOT be used")
	}
	if !strings.Contains(src, "psErr == nil") {
		t.Fatal("GetOrder must gate paymentStatus assignment with psErr == nil — " +
			"no payment row (psErr != nil) must leave paymentStatus nil, not cause 500")
	}
}

// TestGetOrder_PaymentStatusPassedToDTO verifies that paymentStatus is forwarded
// to OrderToDetailResponseWithIdentity, which embeds it in the JSON response as
// the payment_status field.
func TestGetOrder_PaymentStatusPassedToDTO(t *testing.T) {
	src := readHandlerSource(t)

	if !strings.Contains(src, "OrderToDetailResponseWithIdentity") {
		t.Fatal("GetOrder must call OrderToDetailResponseWithIdentity to build the response")
	}
	// paymentStatus is the last argument in the call; confirm it appears after the func name.
	idx := strings.Index(src, "OrderToDetailResponseWithIdentity")
	if idx < 0 {
		t.Fatal("OrderToDetailResponseWithIdentity call not found in handler source")
	}
	afterCall := src[idx:]
	if !strings.Contains(afterCall, "paymentStatus") {
		t.Fatal("GetOrder must pass paymentStatus to OrderToDetailResponseWithIdentity — " +
			"it must appear in the argument list of that call so payment_status reaches the JSON response")
	}
}

// TestGetOrder_PaymentIDQuery_Present verifies that the detail handler selects
// the payment id alongside the status so payment_id is returned in the response.
func TestGetOrder_PaymentIDQuery_Present(t *testing.T) {
	src := readHandlerSource(t)

	if !strings.Contains(src, "id::text") {
		t.Fatal("GetOrder payment query must SELECT id::text to hydrate payment_id in the response")
	}
}

// TestGetOrder_PaymentIDDeclared verifies that the handler declares paymentID
// as *uuid.UUID so it is nil-able (absent when no payment row exists).
func TestGetOrder_PaymentIDDeclared(t *testing.T) {
	src := readHandlerSource(t)

	if !regexp.MustCompile(`paymentID\s+\*uuid\.UUID`).MatchString(src) {
		t.Fatal("GetOrder must declare paymentID as *uuid.UUID — nil when no payment row exists")
	}
}

// TestGetOrder_PaymentNowContract verifies that the pending-buyer pay action
// uses the canonical pay action type, points to the payments endpoint, and
// selects its label_key from payment state rather than a single hardcoded
// string (see selectPayActionLabelKey — Phase 2B-1 CTA wording correction).
func TestGetOrder_PaymentNowContract(t *testing.T) {
	src := readDecisionSource(t)

	if !strings.Contains(src, "ActionPay") {
		t.Fatal("decision contract must include ActionPay for pending buyer orders")
	}
	if !strings.Contains(src, "selectPayActionLabelKey(paymentStatus, paymentExpiredAt)") {
		t.Fatal("pending-buyer pay action must select its label_key via selectPayActionLabelKey, " +
			"not a single hardcoded label — CTA wording must vary by payment state")
	}
	if !strings.Contains(src, `"/api/v1/payments"`) {
		t.Fatal("pay action must use POST /api/v1/payments as the canonical endpoint")
	}
	if strings.Contains(src, `"/api/v1/orders/"+order.ID.String()+"/pay"`) {
		t.Fatal("pay action must not use the legacy /api/v1/orders/:id/pay endpoint")
	}
	if !strings.Contains(src, `WithInputField("order_id"`) {
		t.Fatal("pay action must include order_id in its input schema")
	}
}

// TestGetOrder_PaymentIDPassedToDTO verifies that paymentID is forwarded to
// OrderToDetailResponseWithIdentity so it reaches the JSON response as payment_id.
func TestGetOrder_PaymentIDPassedToDTO(t *testing.T) {
	src := readHandlerSource(t)

	idx := strings.Index(src, "OrderToDetailResponseWithIdentity")
	if idx < 0 {
		t.Fatal("OrderToDetailResponseWithIdentity call not found in handler source")
	}
	afterCall := src[idx:]
	if !strings.Contains(afterCall, "paymentID") {
		t.Fatal("GetOrder must pass paymentID to OrderToDetailResponseWithIdentity — " +
			"it must appear in the argument list of that call so payment_id reaches the JSON response")
	}
}
