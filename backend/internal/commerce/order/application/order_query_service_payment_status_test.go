package application

import (
	"os"
	"strings"
	"testing"
)

// =============================================================================
// CONTRACT LOCK: payment_status batch hydration in OrderQueryService.ListMyOrders
//
// These tests use source inspection to lock the contract without requiring a
// live database — consistent with the source-inspection pattern used throughout
// this package (see order_fulfillment_auth_test.go, order_cancel_overdue_test.go).
//
// Behaviour being locked:
//   - batchFetchPaymentStatuses issues ONE query per list call (no N+1).
//   - The batch query uses DISTINCT ON (reference_id) to pick the highest-
//     priority payment per order: settlement(1) > capture(2) > pending(3) > others(4).
//   - ListMyOrders calls batchFetchPaymentStatuses and assigns the result to
//     item.PaymentStatus and item.PaymentID on each matching list item.
//   - An order with no payment row is absent from the map → PaymentStatus stays nil
//     on the list item (no 500, no crash).
// =============================================================================

func readQueryServiceSource(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile("order_query_service.go")
	if err != nil {
		t.Fatalf("read order_query_service.go: %v", err)
	}
	return string(src)
}

// TestListOrders_BatchPaymentQuery_NotNPlus1 verifies that payment statuses are
// fetched in a single batch query (DISTINCT ON + ANY($1::uuid[])) — not in a
// per-order loop.
func TestListOrders_BatchPaymentQuery_NotNPlus1(t *testing.T) {
	src := readQueryServiceSource(t)

	if !strings.Contains(src, "batchFetchPaymentStatuses") {
		t.Fatal("ListMyOrders must call batchFetchPaymentStatuses for batch payment hydration")
	}

	// Single-query pattern: DISTINCT ON (reference_id) with array parameter.
	if !strings.Contains(src, "DISTINCT ON (reference_id)") {
		t.Fatal("batchFetchPaymentStatuses must use DISTINCT ON (reference_id) to pick " +
			"the highest-priority payment per order in a single query")
	}
	if !strings.Contains(src, "ANY($1::uuid[])") {
		t.Fatal("batchFetchPaymentStatuses must use ANY($1::uuid[]) array parameter — " +
			"calling the DB once for all order IDs, not once per order (no N+1)")
	}
}

// TestListOrders_BatchPaymentQuery_PriorityOrdering verifies that the batch query
// applies the canonical priority: settlement(1) > capture(2) > pending(3) > others(4).
func TestListOrders_BatchPaymentQuery_PriorityOrdering(t *testing.T) {
	src := readQueryServiceSource(t)

	if !strings.Contains(src, "'settlement' THEN 1") {
		t.Fatal("batch payment query must assign priority 1 to 'settlement'")
	}
	if !strings.Contains(src, "'capture'") {
		t.Fatal("batch payment query must include 'capture' in priority ordering")
	}
	if !strings.Contains(src, "'pending'") {
		t.Fatal("batch payment query must include 'pending' in priority ordering")
	}
}

// TestListOrders_BatchPaymentQuery_SelectsPaymentID verifies that the batch
// payment query selects the payment row ID alongside the status so list items
// can hydrate PaymentID from the same chosen row.
func TestListOrders_BatchPaymentQuery_SelectsPaymentID(t *testing.T) {
	src := readQueryServiceSource(t)

	if !strings.Contains(src, "SELECT DISTINCT ON (reference_id) reference_id, id, status::text") {
		t.Fatal("batchFetchPaymentStatuses must select payment id and status from the same chosen row")
	}
}

// TestListOrders_BatchPaymentQuery_ReferenceTypeFilter verifies that the batch
// query filters by reference_type = 'order' to avoid cross-domain payment rows.
func TestListOrders_BatchPaymentQuery_ReferenceTypeFilter(t *testing.T) {
	src := readQueryServiceSource(t)

	if !strings.Contains(src, "reference_type = 'order'") {
		t.Fatal("batchFetchPaymentStatuses must filter by reference_type = 'order' " +
			"to avoid returning payment rows from other domains")
	}
}

// TestListOrders_PaymentStatus_AssignedToListItem verifies that the result of
// batchFetchPaymentStatuses is written to item.PaymentStatus on each list item
// where a payment row exists.
func TestListOrders_PaymentStatus_AssignedToListItem(t *testing.T) {
	src := readQueryServiceSource(t)

	if !strings.Contains(src, "item.PaymentStatus = &ps") {
		t.Fatal("ListMyOrders must assign &ps (payment status string pointer) to " +
			"item.PaymentStatus for orders that have a payment row")
	}
}

// TestListOrders_PaymentID_AssignedToListItem verifies that the chosen payment
// row ID is written to item.PaymentID on each matching list item.
func TestListOrders_PaymentID_AssignedToListItem(t *testing.T) {
	src := readQueryServiceSource(t)

	if !strings.Contains(src, "item.PaymentID = &pm.ID") {
		t.Fatal("ListMyOrders must assign &pm.ID to item.PaymentID for orders that have a payment row")
	}
}

// TestListOrders_NoPayment_PaymentStatusNil verifies that the code path for an
// order without a payment row leaves PaymentStatus nil (not set to empty string,
// not causing a 500).
func TestListOrders_NoPayment_PaymentStatusNil(t *testing.T) {
	src := readQueryServiceSource(t)

	// The map lookup `if ps, ok := paymentStatuses[item.ID]; ok` means absent
	// orders are skipped → PaymentStatus stays nil.  Verify the ok-idiom is used.
	if !strings.Contains(src, "paymentStatuses[item.ID]") {
		t.Fatal("ListMyOrders must look up payment status by item.ID in the batch map; " +
			"missing orders (no payment row) must be left with nil PaymentStatus")
	}

	// psErr path: if batch query fails, items render without payment_status (non-fatal).
	if !strings.Contains(src, "psErr == nil") {
		t.Fatal("ListMyOrders must guard batch assignment with psErr == nil — " +
			"a failed batch query must be non-fatal (items render without payment_status)")
	}
}

// TestListOrders_EmptyOrderList_NoBatchQuery verifies that the batch payment query
// is guarded by len(summaries) > 0 so it is never issued for empty result sets.
func TestListOrders_EmptyOrderList_NoBatchQuery(t *testing.T) {
	src := readQueryServiceSource(t)

	if !strings.Contains(src, "len(summaries) > 0") {
		t.Fatal("ListMyOrders must guard batchFetchPaymentStatuses with len(summaries) > 0 " +
			"to avoid issuing a vacuous query for empty result sets")
	}
}
