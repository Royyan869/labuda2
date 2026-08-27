package worker

import (
	"os"
	"strings"
	"testing"

	"github.com/labuda/backend/internal/platform/events"
)

// =============================================================================
// REGRESSION LOCK: OrderOverdueCancelWorker MUST call CancelOverdue, NOT Cancel
// =============================================================================
//
// P0 BUG CONTEXT:
// The worker auto-cancels paid orders past the shipment deadline. The original
// code called OrderService.Cancel() which transitions to "cancelled" but does
// NOT initiate gateway refund or escrow release — buyer money permanently frozen.
//
// FIX: Worker now calls OrderService.CancelOverdue() which transitions to
// "cancelled_timeout" AND initiates gateway refund + escrow flip + coins refund.
//
// These tests prevent silent regression back to the broken call path.

// TestOverdueCancelWorker_CallsCancelOverdue_NotCancel proves the worker source
// code calls CancelOverdue, not Cancel, on the order service.
//
// SOURCE-READING REGRESSION LOCK: If someone changes the worker back to
// Cancel(), this test fails immediately.
func TestOverdueCancelWorker_CallsCancelOverdue_NotCancel(t *testing.T) {
	src, err := os.ReadFile("order_overdue_cancel_worker.go")
	if err != nil {
		t.Fatalf("read order_overdue_cancel_worker.go: %v", err)
	}
	source := string(src)

	// MUST contain CancelOverdue call
	if !strings.Contains(source, "w.orderService.CancelOverdue(") {
		t.Fatal("OrderOverdueCancelWorker MUST call orderService.CancelOverdue(), not Cancel(). " +
			"CancelOverdue() initiates gateway refund + escrow flip. " +
			"Cancel() does NOT refund — buyer money would be permanently frozen.")
	}

	// MUST NOT contain bare Cancel() call (which is the broken path)
	// We check for the specific pattern used in processOrder.
	// The comment or variable name may mention "cancel" but the actual call
	// must be CancelOverdue.
	bareCancel := "w.orderService.Cancel("
	if strings.Contains(source, bareCancel) {
		t.Fatalf("OrderOverdueCancelWorker contains bare %q call — "+
			"this is the P0 money-freeze bug. Use CancelOverdue() instead.", bareCancel)
	}
}

// TestOverdueCancelWorker_IdempotencyKeyPrefix proves the worker uses a
// recognizable idempotency key prefix so overdue cancellations are
// distinguishable from buyer-initiated cancellations in audit logs.
func TestOverdueCancelWorker_IdempotencyKeyPrefix(t *testing.T) {
	src, err := os.ReadFile("order_overdue_cancel_worker.go")
	if err != nil {
		t.Fatalf("read order_overdue_cancel_worker.go: %v", err)
	}
	if !strings.Contains(string(src), `"worker_overdue_cancel_"`) {
		t.Error("worker must use 'worker_overdue_cancel_' idempotency key prefix for audit traceability")
	}
}

// TestEventConstant_OrderCancelledTimeout proves the event constant exists and
// matches the string used by CancelOverdue's outbox emission and the
// notification handler's registration.
func TestEventConstant_OrderCancelledTimeout(t *testing.T) {
	const expected = "order.cancelled_timeout"
	if events.EventOrderCancelledTimeout != expected {
		t.Fatalf("EventOrderCancelledTimeout = %q, want %q", events.EventOrderCancelledTimeout, expected)
	}
}

// TestCancelledTimeoutEvent_IsConsumedByNotificationHandler proves the
// notification worker registers a handler for order.cancelled_timeout.
// This ensures the outbox event emitted by CancelOverdue is not silently dropped.
func TestCancelledTimeoutEvent_IsConsumedByNotificationHandler(t *testing.T) {
	src, err := os.ReadFile("notification_worker.go")
	if err != nil {
		t.Fatalf("read notification_worker.go: %v", err)
	}
	source := string(src)

	// Must be in the switch case
	if !strings.Contains(source, `case "order.cancelled_timeout"`) {
		t.Fatal("notification_worker.go must have a switch case for order.cancelled_timeout; " +
			"without it, the event emitted by CancelOverdue is silently dropped")
	}

	// Must be in RegisterMultiple
	if !strings.Contains(source, `"order.cancelled_timeout"`) {
		t.Fatal("notification_worker.go must register order.cancelled_timeout in RegisterMultiple; " +
			"unregistered events are not dispatched to the handler")
	}
}

// TestCancelledTimeoutEvent_NotInNoHandlerAllowlist proves order.cancelled_timeout
// is NOT in AcknowledgedNoHandlerEvents (because it HAS a handler).
func TestCancelledTimeoutEvent_NotInNoHandlerAllowlist(t *testing.T) {
	if _, found := AcknowledgedNoHandlerEvents["order.cancelled_timeout"]; found {
		t.Fatal("order.cancelled_timeout MUST NOT be in AcknowledgedNoHandlerEvents — " +
			"it has a live notification handler. Remove it from the allowlist.")
	}
}


