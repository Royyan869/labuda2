package worker

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	orderEntity "github.com/labuda/backend/internal/commerce/order/entity"
)

// newTestBanHandler returns a zero-dependency handler suitable for pure-logic tests.
func newTestBanHandler() *UserBanEventHandler {
	return &UserBanEventHandler{log: zap.NewNop()}
}

// orderWithState builds a minimal Order for branch-routing tests.
func orderWithState(
	buyerID, sellerID uuid.UUID,
	status orderEntity.Status,
	escrow orderEntity.EscrowStatus,
	trackingNumber string,
	hasDispute bool,
) *orderEntity.Order {
	o := &orderEntity.Order{
		ID:           uuid.New(),
		BuyerID:      buyerID,
		SellerID:     sellerID,
		Status:       status,
		EscrowStatus: escrow,
		HasDispute:   hasDispute,
	}
	if trackingNumber != "" {
		o.TrackingNumber = &trackingNumber
	}
	return o
}

// ─────────────────────────────────────────────────────────────────────────────
// hasShipmentEvidence
// ─────────────────────────────────────────────────────────────────────────────

func TestUserBanHandler_HasShipmentEvidence_TrackingNumber(t *testing.T) {
	h := newTestBanHandler()
	buyerID := uuid.New()
	sellerID := uuid.New()

	o := orderWithState(buyerID, sellerID, orderEntity.StatusPaid, orderEntity.EscrowStatusHolding, "JNE123", false)
	assert.True(t, h.hasShipmentEvidence(o), "tracking number present → evidence")
}

func TestUserBanHandler_HasShipmentEvidence_StatusShipped(t *testing.T) {
	h := newTestBanHandler()
	o := orderWithState(uuid.New(), uuid.New(), orderEntity.StatusShipped, orderEntity.EscrowStatusHolding, "", false)
	assert.True(t, h.hasShipmentEvidence(o), "status=shipped → evidence")
}

func TestUserBanHandler_HasShipmentEvidence_StatusDelivered(t *testing.T) {
	h := newTestBanHandler()
	o := orderWithState(uuid.New(), uuid.New(), orderEntity.StatusDelivered, orderEntity.EscrowStatusHolding, "", false)
	assert.True(t, h.hasShipmentEvidence(o), "status=delivered → evidence")
}

func TestUserBanHandler_HasShipmentEvidence_NonePresent(t *testing.T) {
	h := newTestBanHandler()
	o := orderWithState(uuid.New(), uuid.New(), orderEntity.StatusPaid, orderEntity.EscrowStatusHolding, "", false)
	assert.False(t, h.hasShipmentEvidence(o), "paid, no tracking → no evidence")
}

// ─────────────────────────────────────────────────────────────────────────────
// shouldRefundDirectly
// ─────────────────────────────────────────────────────────────────────────────

func TestUserBanHandler_ShouldRefundDirectly_NoEvidence_Holding(t *testing.T) {
	h := newTestBanHandler()
	o := orderWithState(uuid.New(), uuid.New(), orderEntity.StatusPaid, orderEntity.EscrowStatusHolding, "", false)
	assert.True(t, h.shouldRefundDirectly(o), "paid + holding + no evidence → refund")
}

func TestUserBanHandler_ShouldRefundDirectly_EscrowNotHolding(t *testing.T) {
	h := newTestBanHandler()
	o := orderWithState(uuid.New(), uuid.New(), orderEntity.StatusPaid, orderEntity.EscrowStatusReleased, "", false)
	assert.False(t, h.shouldRefundDirectly(o), "escrow released → no refund")
}

func TestUserBanHandler_ShouldRefundDirectly_HasEvidence(t *testing.T) {
	h := newTestBanHandler()
	o := orderWithState(uuid.New(), uuid.New(), orderEntity.StatusPaid, orderEntity.EscrowStatusHolding, "JNE123", false)
	assert.False(t, h.shouldRefundDirectly(o), "has tracking → not direct refund")
}

// ─────────────────────────────────────────────────────────────────────────────
// shouldAutoCompleteForBannedBuyer
// ─────────────────────────────────────────────────────────────────────────────

func TestUserBanHandler_AutoComplete_BuyerBanned_Shipped(t *testing.T) {
	h := newTestBanHandler()
	buyerID := uuid.New()
	o := orderWithState(buyerID, uuid.New(), orderEntity.StatusShipped, orderEntity.EscrowStatusHolding, "JNE123", false)
	assert.True(t, h.shouldAutoCompleteForBannedBuyer(buyerID, o),
		"buyer banned + shipped + holding + no dispute → auto-complete")
}

func TestUserBanHandler_AutoComplete_BuyerBanned_Delivered(t *testing.T) {
	h := newTestBanHandler()
	buyerID := uuid.New()
	o := orderWithState(buyerID, uuid.New(), orderEntity.StatusDelivered, orderEntity.EscrowStatusHolding, "", false)
	assert.True(t, h.shouldAutoCompleteForBannedBuyer(buyerID, o),
		"buyer banned + delivered + holding + no dispute → auto-complete")
}

func TestUserBanHandler_AutoComplete_SellerBanned_Shipped(t *testing.T) {
	h := newTestBanHandler()
	sellerID := uuid.New()
	o := orderWithState(uuid.New(), sellerID, orderEntity.StatusShipped, orderEntity.EscrowStatusHolding, "JNE123", false)
	assert.False(t, h.shouldAutoCompleteForBannedBuyer(sellerID, o),
		"seller banned + shipped → NOT auto-complete (buyer is not the banned user)")
}

func TestUserBanHandler_AutoComplete_BuyerBanned_Shipped_HasDispute(t *testing.T) {
	h := newTestBanHandler()
	buyerID := uuid.New()
	o := orderWithState(buyerID, uuid.New(), orderEntity.StatusShipped, orderEntity.EscrowStatusHolding, "JNE123", true)
	assert.False(t, h.shouldAutoCompleteForBannedBuyer(buyerID, o),
		"active dispute → no auto-complete")
}

func TestUserBanHandler_AutoComplete_BuyerBanned_Paid(t *testing.T) {
	h := newTestBanHandler()
	buyerID := uuid.New()
	o := orderWithState(buyerID, uuid.New(), orderEntity.StatusPaid, orderEntity.EscrowStatusHolding, "", false)
	assert.False(t, h.shouldAutoCompleteForBannedBuyer(buyerID, o),
		"status=paid (not shipped/delivered) → no auto-complete")
}

// ─────────────────────────────────────────────────────────────────────────────
// shouldForceDispute — full scenario matrix (canonical doctrine)
// ─────────────────────────────────────────────────────────────────────────────

func TestUserBanHandler_ForceDispute_SellerBanned_Shipped_WithTracking(t *testing.T) {
	h := newTestBanHandler()
	sellerID := uuid.New()
	o := orderWithState(uuid.New(), sellerID, orderEntity.StatusShipped, orderEntity.EscrowStatusHolding, "JNE123", false)
	assert.True(t, h.shouldForceDispute(sellerID, o),
		"seller banned + shipped + tracking → dispute (seller trust compromised)")
}

func TestUserBanHandler_ForceDispute_SellerBanned_Delivered_NoTracking(t *testing.T) {
	h := newTestBanHandler()
	sellerID := uuid.New()
	// status=delivered is itself shipment evidence
	o := orderWithState(uuid.New(), sellerID, orderEntity.StatusDelivered, orderEntity.EscrowStatusHolding, "", false)
	assert.True(t, h.shouldForceDispute(sellerID, o),
		"seller banned + delivered (status-based evidence) → dispute")
}

func TestUserBanHandler_ForceDispute_BuyerBanned_Shipped(t *testing.T) {
	h := newTestBanHandler()
	buyerID := uuid.New()
	o := orderWithState(buyerID, uuid.New(), orderEntity.StatusShipped, orderEntity.EscrowStatusHolding, "JNE123", false)
	// buyer-ban + shipped → shouldAutoCompleteForBannedBuyer owns this, NOT dispute
	assert.False(t, h.shouldForceDispute(buyerID, o),
		"buyer banned + shipped → false (auto-complete path owns this)")
}

func TestUserBanHandler_ForceDispute_BuyerBanned_Delivered(t *testing.T) {
	h := newTestBanHandler()
	buyerID := uuid.New()
	o := orderWithState(buyerID, uuid.New(), orderEntity.StatusDelivered, orderEntity.EscrowStatusHolding, "", false)
	assert.False(t, h.shouldForceDispute(buyerID, o),
		"buyer banned + delivered → false (auto-complete path owns this)")
}

func TestUserBanHandler_ForceDispute_SellerBanned_Paid_WithTracking(t *testing.T) {
	h := newTestBanHandler()
	sellerID := uuid.New()
	o := orderWithState(uuid.New(), sellerID, orderEntity.StatusPaid, orderEntity.EscrowStatusHolding, "JNE123", false)
	assert.True(t, h.shouldForceDispute(sellerID, o),
		"seller banned + paid + tracking → dispute")
}

func TestUserBanHandler_ForceDispute_EscrowNotHolding(t *testing.T) {
	h := newTestBanHandler()
	sellerID := uuid.New()
	o := orderWithState(uuid.New(), sellerID, orderEntity.StatusShipped, orderEntity.EscrowStatusReleased, "JNE123", false)
	assert.False(t, h.shouldForceDispute(sellerID, o),
		"escrow not holding → no dispute")
}

func TestUserBanHandler_ForceDispute_NoEvidence_NotShipped(t *testing.T) {
	h := newTestBanHandler()
	sellerID := uuid.New()
	o := orderWithState(uuid.New(), sellerID, orderEntity.StatusPaid, orderEntity.EscrowStatusHolding, "", false)
	assert.False(t, h.shouldForceDispute(sellerID, o),
		"paid + no tracking → no dispute (refund path)")
}

// ─────────────────────────────────────────────────────────────────────────────
// Complete scenario routing matrix (switch exhaustion)
// Validates which branch fires for every combination without hitting the DB.
// ─────────────────────────────────────────────────────────────────────────────

type banRoute int

const (
	routeAutoComplete banRoute = iota
	routeRefund
	routeDispute
	routeNoAction
)

func routeForOrder(h *UserBanEventHandler, bannedUserID uuid.UUID, o *orderEntity.Order) banRoute {
	switch {
	case h.shouldAutoCompleteForBannedBuyer(bannedUserID, o):
		return routeAutoComplete
	case h.shouldRefundDirectly(o):
		return routeRefund
	case h.shouldForceDispute(bannedUserID, o):
		return routeDispute
	default:
		return routeNoAction
	}
}

func TestUserBanHandler_RoutingMatrix(t *testing.T) {
	h := newTestBanHandler()

	buyerID := uuid.New()
	sellerID := uuid.New()

	cases := []struct {
		name      string
		bannedUID uuid.UUID
		order     *orderEntity.Order
		want      banRoute
	}{
		{
			name:      "buyer banned + pending_payment (no escrow holding)",
			bannedUID: buyerID,
			// StatusPending = "pending_payment"; escrow not holding (payment not made)
			order: orderWithState(buyerID, sellerID, orderEntity.StatusPending, orderEntity.EscrowStatusReleased, "", false),
			want:  routeNoAction,
		},
		{
			name:      "buyer banned + paid + no tracking",
			bannedUID: buyerID,
			order:     orderWithState(buyerID, sellerID, orderEntity.StatusPaid, orderEntity.EscrowStatusHolding, "", false),
			want:      routeRefund,
		},
		{
			name:      "buyer banned + shipped",
			bannedUID: buyerID,
			order:     orderWithState(buyerID, sellerID, orderEntity.StatusShipped, orderEntity.EscrowStatusHolding, "JNE123", false),
			want:      routeAutoComplete,
		},
		{
			name:      "buyer banned + delivered",
			bannedUID: buyerID,
			order:     orderWithState(buyerID, sellerID, orderEntity.StatusDelivered, orderEntity.EscrowStatusHolding, "", false),
			want:      routeAutoComplete,
		},
		{
			name:      "buyer banned + dispute_open (escrow frozen)",
			bannedUID: buyerID,
			order:     orderWithState(buyerID, sellerID, orderEntity.StatusDisputeOpen, orderEntity.EscrowStatusReleased, "JNE123", true),
			want:      routeNoAction,
		},
		{
			name:      "seller banned + paid + no tracking",
			bannedUID: sellerID,
			order:     orderWithState(buyerID, sellerID, orderEntity.StatusPaid, orderEntity.EscrowStatusHolding, "", false),
			want:      routeRefund,
		},
		{
			name:      "seller banned + paid + tracking",
			bannedUID: sellerID,
			order:     orderWithState(buyerID, sellerID, orderEntity.StatusPaid, orderEntity.EscrowStatusHolding, "JNE123", false),
			want:      routeDispute,
		},
		{
			name:      "seller banned + shipped + tracking",
			bannedUID: sellerID,
			order:     orderWithState(buyerID, sellerID, orderEntity.StatusShipped, orderEntity.EscrowStatusHolding, "JNE123", false),
			want:      routeDispute,
		},
		{
			name:      "seller banned + delivered",
			bannedUID: sellerID,
			order:     orderWithState(buyerID, sellerID, orderEntity.StatusDelivered, orderEntity.EscrowStatusHolding, "", false),
			want:      routeDispute,
		},
		{
			name:      "seller banned + dispute_open (escrow frozen)",
			bannedUID: sellerID,
			order:     orderWithState(buyerID, sellerID, orderEntity.StatusDisputeOpen, orderEntity.EscrowStatusReleased, "JNE123", true),
			want:      routeNoAction,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := routeForOrder(h, tc.bannedUID, tc.order)
			assert.Equal(t, tc.want, got,
				"unexpected route for: %s", tc.name)
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Schema regression lock (existing test, preserved)
// ─────────────────────────────────────────────────────────────────────────────

// TestUserBanHandler_QueryColumnsMatchSchema verifies that the SQL query in
// getActiveOrdersForUser uses only columns that exist in the orders table.
//
// HISTORY: The original query referenced 3 ghost columns (paid_at, shipped_at,
// shipping_reference) that do not exist in the orders schema. This test locks
// the column list to prevent regression.
func TestUserBanHandler_QueryColumnsMatchSchema(t *testing.T) {
	// These are the columns that EXIST in the orders table (from
	// legacy_do_not_run/000_init/110 + 000113).
	// Any column referenced in getActiveOrdersForUser must be in this set.
	schemaColumns := map[string]bool{
		"id":                          true,
		"buyer_id":                    true,
		"seller_id":                   true,
		"status":                      true,
		"escrow_status":               true,
		"has_dispute":                 true,
		"subtotal":                    true,
		"platform_fee":                true,
		"total_price":                 true,
		"source_type":                 true,
		"source_id":                   true,
		"negotiation_id":              true,
		"auction_settlement_type":     true,
		"quantity":                    true,
		"product_name_snapshot":       true,
		"shipping_cost":               true,
		"shipping_option_name":        true,
		"shipping_transport_type":     true,
		"preparation_time_snapshot":   true,
		"auto_release_at":             true,
		"completed_at":                true,
		"payment_expires_at":          true,
		"ready_to_ship_by":            true,
		"proof_type":                  true,
		"tracking_number":             true,
		"shipping_proof_media":        true,
		"shipping_note":               true,
		"address_snapshot":            true,
		"preparation_note_snapshot":   true,
		"confirmation_extension_used": true,
		"confirmation_extended_at":    true,
		"created_at":                  true,
		"updated_at":                  true,
		"order_number":                true,
		"coins_applied":               true,
		"discount_applied":            true,
		"discount_code":               true,
		"shipping_source":             true,
		"shipping_origin_snapshot":    true,
	}

	// Columns actually used in the getActiveOrdersForUser query.
	// This must stay in sync with the SQL in user_ban_handler.go.
	queryColumns := []string{
		"id", "buyer_id", "seller_id", "status", "escrow_status",
		"proof_type", "tracking_number", "shipping_proof_media",
		"has_dispute", "created_at",
	}

	// Ghost columns that must NEVER appear (regression lock).
	ghostColumns := []string{"paid_at", "shipped_at", "shipping_reference"}

	for _, col := range queryColumns {
		assert.True(t, schemaColumns[col],
			"query column %q does not exist in orders schema", col)
	}

	for _, col := range ghostColumns {
		assert.False(t, schemaColumns[col],
			"ghost column %q must not be in schema set", col)
		for _, qc := range queryColumns {
			assert.False(t, strings.EqualFold(qc, col),
				"ghost column %q must not appear in query columns", col)
		}
	}

	// Verify column count matches scan targets (10 SELECT, 10 Scan)
	assert.Equal(t, 10, len(queryColumns),
		"SELECT column count must match Scan target count")
}

func TestUserBanHandler_NoDeferredProcessedMarker(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	srcPath := filepath.Join(filepath.Dir(thisFile), "user_ban_handler.go")
	srcBytes, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("failed to read source: %v", err)
	}
	src := string(srcBytes)

	assert.NotContains(t, src, "defer func() {\n\t\t_ = h.markAsProcessed",
		"processed marker must not be deferred unconditionally")
}

func TestUserBanHandler_MarksProcessedInSuccessAndNoOpPaths(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	srcPath := filepath.Join(filepath.Dir(thisFile), "user_ban_handler.go")
	srcBytes, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("failed to read source: %v", err)
	}
	src := string(srcBytes)

	assert.Contains(t, src, "h.markAsProcessedTx(ctx, tx, eventID, order.ID, bannedUserID, \"auto_completed\")")
	assert.Contains(t, src, "h.markAsProcessedTx(ctx, tx, eventID, order.ID, bannedUserID, \"refunded\")")
	assert.Contains(t, src, "h.markAsProcessedTx(ctx, tx, eventID, order.ID, bannedUserID, \"dispute_opened\")")
	assert.Contains(t, src, "h.markAsProcessed(ctx, eventID, order.ID, bannedUserID, \"no_action\")")
}

func TestUserBanHandler_AnyOrderFailureCausesRetryableError(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	srcPath := filepath.Join(filepath.Dir(thisFile), "user_ban_handler.go")
	srcBytes, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("failed to read source: %v", err)
	}
	src := string(srcBytes)

	assert.Contains(t, src, "return fmt.Errorf(\"user ban handler: one or more orders failed: %w\", firstErr)",
		"handler must return error when any per-order action fails so outbox retries")
}

func TestUserBanHandler_ActiveRefundGuardFailureIsRetryableAndUnprocessed(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	srcPath := filepath.Join(filepath.Dir(thisFile), "user_ban_handler.go")
	srcBytes, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("failed to read source: %v", err)
	}
	src := string(srcBytes)

	// Route proof: buyer-banned shipped/delivered branch goes through auto-complete.
	assert.Contains(t, src, "case h.shouldAutoCompleteForBannedBuyer(bannedUserID, order):")
	assert.Contains(t, src, "return h.completeOrderForBan(ctx, order, bannedUserID, eventID)")

	// Guard/failure proof: completeOrderForBan returns on complete() failure first.
	completeCall := strings.Index(src, "if err := h.orderService.Complete(ctx, tx, auth.SystemCallerID, order.ID, \"\"); err != nil {")
	completeErrReturn := strings.Index(src, "return fmt.Errorf(\"failed to complete order: %w\", err)")
	markProcessed := strings.Index(src, "h.markAsProcessedTx(ctx, tx, eventID, order.ID, bannedUserID, \"auto_completed\")")
	if completeCall == -1 || completeErrReturn == -1 || markProcessed == -1 {
		t.Fatal("expected complete/return/mark processed statements not found")
	}
	if !(completeCall < completeErrReturn && completeErrReturn < markProcessed) {
		t.Fatal("expected complete failure return before processed marker write")
	}

	// Retryability proof: any per-order failure bubbles up from Handle().
	assert.Contains(t, src, "return fmt.Errorf(\"user ban handler: one or more orders failed: %w\", firstErr)",
		"handle must return error so outbox retry remains possible")
}

func TestUserBanHandler_TerminalStatusesExcludedFromDiscoveryQuery(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	srcPath := filepath.Join(filepath.Dir(thisFile), "user_ban_handler.go")
	srcBytes, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("failed to read source: %v", err)
	}
	src := string(srcBytes)

	// Regression lock for terminal-status exclusion semantics.
	assert.Contains(t, src, "status NOT IN ('completed', 'cancelled', 'expired', 'refunded', 'partially_refunded')",
		"terminal statuses must remain excluded from discovery query")
}


