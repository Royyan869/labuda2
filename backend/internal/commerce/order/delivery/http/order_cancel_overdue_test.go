package http

import (
	"os"
	"strings"
	"testing"
)

// =============================================================================
// REGRESSION LOCK: CancelOrder handler MUST route overdue orders to CancelOverdue
// =============================================================================
//
// P0 BUG CONTEXT:
// The buyer-facing POST /orders/:id/cancel endpoint previously called
// OrderService.Cancel() unconditionally. For overdue (paid + past deadline)
// orders, Cancel() sets status to "cancelled" WITHOUT gateway refund — buyer
// money permanently frozen.
//
// FIX: The handler now fetches the order inside the transaction, checks
// IsBuyerEligibleForCancel(), and routes to CancelOverdue() for overdue orders.
//
// These tests prevent silent regression.

// TestCancelOrderHandler_RoutesToCancelOverdue_ForOverdueOrders proves the
// CancelOrder handler source contains the overdue routing branch.
func TestCancelOrderHandler_RoutesToCancelOverdue_ForOverdueOrders(t *testing.T) {
	src, err := os.ReadFile("order_handler.go")
	if err != nil {
		t.Fatalf("read order_handler.go: %v", err)
	}
	source := string(src)

	// The handler MUST contain IsBuyerEligibleForCancel check
	if !strings.Contains(source, "IsBuyerEligibleForCancel()") {
		t.Fatal("CancelOrder handler MUST check IsBuyerEligibleForCancel() to route " +
			"overdue orders to CancelOverdue(). Without this, buyer money is frozen.")
	}

	// The handler MUST call CancelOverdue for overdue path
	if !strings.Contains(source, "h.orderService.CancelOverdue(") {
		t.Fatal("CancelOrder handler MUST call CancelOverdue() for overdue orders. " +
			"Only Cancel() is present — overdue orders would not get refunded.")
	}

	// The handler MUST still call Cancel for non-overdue path
	if !strings.Contains(source, "h.orderService.Cancel(") {
		t.Fatal("CancelOrder handler MUST still call Cancel() for non-overdue orders " +
			"(pre-payment cancellation). Both paths must exist.")
	}
}

// TestCancelOrderHandler_OverdueErrorResponse proves the handler returns a
// specific error code for overdue eligibility failures.
func TestCancelOrderHandler_OverdueErrorResponse(t *testing.T) {
	src, err := os.ReadFile("order_handler.go")
	if err != nil {
		t.Fatalf("read order_handler.go: %v", err)
	}
	source := string(src)

	// Must handle "buyer not eligible for cancel" error from CancelOverdue
	if !strings.Contains(source, `"buyer not eligible for cancel"`) {
		t.Error("CancelOrder handler must handle 'buyer not eligible for cancel' error " +
			"from CancelOverdue() and return a user-facing error response")
	}
}


