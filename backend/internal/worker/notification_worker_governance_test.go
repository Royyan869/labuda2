package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap/zaptest"

	platformevent "github.com/labuda/backend/internal/platform/event"
	"github.com/labuda/backend/internal/platform/events"
	dbpkg "github.com/labuda/backend/pkg/db"
)

func TestOrderDisputeOpen_SellerNotified_NoLister(t *testing.T) {
	// Without capabilityLister, only seller gets notified (backward compat).
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	var capturedRecipient uuid.UUID
	mockDB := &mockDBForNotification{
		WithTxFunc: insertCaptureTx(&capturedRecipient, nil, nil, nil, nil),
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	// No SetCapabilityLister — backward compat path.

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "order.dispute_open", Payload: makeDisputeOpenPayload(orderID, buyerID, sellerID),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if capturedRecipient != sellerID {
		t.Errorf("recipient = %s, want seller %s", capturedRecipient, sellerID)
	}
}

func TestOrderDisputeOpen_SellerAndAdminFanout(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	admin1 := uuid.New()
	admin2 := uuid.New()

	var recipients []uuid.UUID
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			tx := &mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, args ...any) pgx.Row {
					if len(args) >= 6 {
						recipients = append(recipients, args[1].(uuid.UUID))
					}
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			}
			return fn(tx)
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{
			"finance.dispute.resolve": {admin1, admin2},
		},
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "order.dispute_open", Payload: makeDisputeOpenPayload(orderID, buyerID, sellerID),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// 1 seller + 2 admins = 3 inserts
	if len(recipients) != 3 {
		t.Fatalf("insert count = %d, want 3 (1 seller + 2 admins)", len(recipients))
	}
	if recipients[0] != sellerID {
		t.Errorf("first recipient = %s, want seller %s", recipients[0], sellerID)
	}
	adminSet := map[uuid.UUID]bool{admin1: true, admin2: true}
	for _, r := range recipients[1:] {
		if !adminSet[r] {
			t.Errorf("unexpected admin recipient %s", r)
		}
	}
}

func TestOrderDisputeOpen_ZeroReviewers_NoError(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	dbCalls := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			dbCalls++
			tx := &mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			}
			return fn(tx)
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{}, // no reviewers
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "order.dispute_open", Payload: makeDisputeOpenPayload(orderID, buyerID, sellerID),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	// Only seller notification — no admin inserts.
	if dbCalls != 1 {
		t.Errorf("WithTx called %d times, want 1 (seller only)", dbCalls)
	}
}

func TestOrderDisputeOpen_CapabilityLookupError_ReturnsError(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			tx := &mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			}
			return fn(tx)
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		err: fmt.Errorf("connection refused"),
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "order.dispute_open", Payload: makeDisputeOpenPayload(orderID, buyerID, sellerID),
	})
	if err == nil {
		t.Fatal("expected error for capability lookup failure, got nil")
	}
	if !strings.Contains(err.Error(), "list admins by capability") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOrderDisputeOpen_TotalAdminFanoutFailure_ReturnsError(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	admin1 := uuid.New()

	callCount := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			callCount++
			if callCount == 1 {
				// Seller insert succeeds
				tx := &mockTxForNotification{
					QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
						return &mockRowForNotification{scanValue: uuid.New()}
					},
				}
				return fn(tx)
			}
			// Admin insert fails
			return fmt.Errorf("db connection lost")
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{
			"finance.dispute.resolve": {admin1},
		},
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "order.dispute_open", Payload: makeDisputeOpenPayload(orderID, buyerID, sellerID),
	})
	if err == nil {
		t.Fatal("expected error for total admin fanout failure, got nil")
	}
	if !strings.Contains(err.Error(), "all 1 admin fanout inserts failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOrderDisputeOpen_PartialAdminFanoutFailure_Succeeds(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	admin1 := uuid.New()
	admin2 := uuid.New()

	callCount := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			callCount++
			if callCount == 2 {
				// First admin insert fails
				return fmt.Errorf("transient error")
			}
			// Seller + second admin succeed
			tx := &mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			}
			return fn(tx)
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{
			"finance.dispute.resolve": {admin1, admin2},
		},
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "order.dispute_open", Payload: makeDisputeOpenPayload(orderID, buyerID, sellerID),
	})
	if err != nil {
		t.Fatalf("partial failure should not return error, got: %v", err)
	}
	// 3 WithTx calls: 1 seller + 2 admin attempts (1 failed, 1 succeeded)
	if callCount != 3 {
		t.Errorf("WithTx calls = %d, want 3", callCount)
	}
}

func TestOrderDisputeOpen_ReplayIdempotent(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	admin1 := uuid.New()

	insertCount := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			tx := &mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					insertCount++
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			}
			return fn(tx)
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{
			"finance.dispute.resolve": {admin1},
		},
	})

	event := platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "order.dispute_open", Payload: makeDisputeOpenPayload(orderID, buyerID, sellerID),
	}

	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("first Handle() error = %v", err)
	}
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("replay Handle() error = %v", err)
	}

	// 2 deliveries × (1 seller + 1 admin) = 4 inserts. DB dedup handles actual duplicates.
	if insertCount != 4 {
		t.Errorf("insertCount = %d, want 4", insertCount)
	}
}

func TestOrderDisputeOpen_RefundEscalatedNoAdminDuplicate(t *testing.T) {
	// Proves that refund.escalated does NOT fanout to admins — only
	// order.dispute_open does. This prevents duplicate admin notifications
	// when both events fire in the refund escalation path.
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	refundPayload, _ := json.Marshal(struct {
		RefundID string `json:"refund_id"`
		OrderID  string `json:"order_id"`
		BuyerID  string `json:"buyer_id"`
		SellerID string `json:"seller_id"`
		Status   string `json:"status"`
	}{
		RefundID: uuid.New().String(),
		OrderID:  orderID.String(),
		BuyerID:  buyerID.String(),
		SellerID: sellerID.String(),
		Status:   "escalated_to_admin",
	})

	var recipients []uuid.UUID
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			tx := &mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, args ...any) pgx.Row {
					if len(args) >= 6 {
						recipients = append(recipients, args[1].(uuid.UUID))
					}
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			}
			return fn(tx)
		},
	}

	admin1 := uuid.New()
	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{
			"finance.dispute.resolve": {admin1},
		},
	})

	// Fire refund.escalated — should notify seller + buyer only, NOT admins.
	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "refund.escalated", Payload: refundPayload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// refund.escalated notifies seller + buyer = 2 recipients. No admin.
	if len(recipients) != 2 {
		t.Fatalf("refund.escalated insert count = %d, want 2 (seller + buyer, no admin)", len(recipients))
	}
	for _, r := range recipients {
		if r == admin1 {
			t.Errorf("admin %s should NOT receive refund.escalated notification — admin fanout is on order.dispute_open only", admin1)
		}
	}
}

func TestRefundEscalated_SellerFail_BuyerSuccess_ReturnsError(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	refundPayload, _ := json.Marshal(struct {
		RefundID string `json:"refund_id"`
		OrderID  string `json:"order_id"`
		BuyerID  string `json:"buyer_id"`
		SellerID string `json:"seller_id"`
		Status   string `json:"status"`
	}{
		RefundID: uuid.New().String(),
		OrderID:  orderID.String(),
		BuyerID:  buyerID.String(),
		SellerID: sellerID.String(),
		Status:   "escalated_to_admin",
	})

	call := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
			return fn(&mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					call++
					if call == 1 {
						return &mockRowForNotification{err: errors.New("seller insert failed")}
					}
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			})
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "refund.escalated", Payload: refundPayload,
	})
	if err == nil {
		t.Fatal("expected error for seller insert failure, got nil")
	}
	if !strings.Contains(err.Error(), "seller insert failed") {
		t.Errorf("error = %v, want seller insert failure", err)
	}
}

// =============================================================================
// DISPUTE.OPENED — POST-RELEASE SELLER + ADMIN NOTIFICATION (D1B)
// =============================================================================

func makeDisputeOpenedPayload(orderID, buyerID, sellerID uuid.UUID, isPostRelease bool) []byte {
	b, _ := json.Marshal(map[string]interface{}{
		"dispute_id":      uuid.New().String(),
		"order_id":        orderID.String(),
		"buyer_id":        buyerID.String(),
		"seller_id":       sellerID.String(),
		"caller_id":       buyerID.String(),
		"reason":          "item not as described",
		"is_post_release": isPostRelease,
	})
	return b
}

func TestDisputeOpened_PostRelease_SellerNotified(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	var capturedRecipient uuid.UUID
	mockDB := &mockDBForNotification{
		WithTxFunc: insertCaptureTx(&capturedRecipient, nil, nil, nil, nil),
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	// No SetCapabilityLister — seller-only path.

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "dispute.opened", Payload: makeDisputeOpenedPayload(orderID, buyerID, sellerID, true),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if capturedRecipient != sellerID {
		t.Errorf("recipient = %s, want seller %s", capturedRecipient, sellerID)
	}
}

func TestDisputeOpened_PreRelease_NoNotification(t *testing.T) {
	// is_post_release=false → order.dispute_open already fired → no-op.
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	insertCalled := false
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			insertCalled = true
			return fn(&mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			})
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "dispute.opened", Payload: makeDisputeOpenedPayload(orderID, buyerID, sellerID, false),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if insertCalled {
		t.Error("expected no DB insert for pre-release dispute.opened, but insert was called")
	}
}

func TestDisputeOpened_PostRelease_AdminFanout(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	admin1 := uuid.New()
	admin2 := uuid.New()

	var recipients []uuid.UUID
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			tx := &mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, args ...any) pgx.Row {
					if len(args) >= 6 {
						recipients = append(recipients, args[1].(uuid.UUID))
					}
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			}
			return fn(tx)
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{
			"finance.dispute.resolve": {admin1, admin2},
		},
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "dispute.opened", Payload: makeDisputeOpenedPayload(orderID, buyerID, sellerID, true),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// 1 seller + 2 admins = 3 inserts
	if len(recipients) != 3 {
		t.Fatalf("insert count = %d, want 3 (1 seller + 2 admins)", len(recipients))
	}
	if recipients[0] != sellerID {
		t.Errorf("first recipient = %s, want seller %s", recipients[0], sellerID)
	}
	adminSet := map[uuid.UUID]bool{admin1: true, admin2: true}
	for _, r := range recipients[1:] {
		if !adminSet[r] {
			t.Errorf("unexpected admin recipient %s", r)
		}
	}
}

func TestDisputeOpened_PostRelease_Idempotent(t *testing.T) {
	// Replaying the same event must not return an error (DB dedup handles uniqueness).
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	admin1 := uuid.New()

	insertCount := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			insertCount++
			return fn(&mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			})
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{
			"finance.dispute.resolve": {admin1},
		},
	})

	event := platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "dispute.opened", Payload: makeDisputeOpenedPayload(orderID, buyerID, sellerID, true),
	}

	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("first Handle() error = %v", err)
	}
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("replay Handle() error = %v", err)
	}

	// 2 deliveries × (1 seller + 1 admin) = 4 inserts. DB dedup handles actual duplicates.
	if insertCount != 4 {
		t.Errorf("insertCount = %d, want 4", insertCount)
	}
}

func TestDisputeOpened_InvalidPayload(t *testing.T) {
	h := buildSocialGovernanceHandler(t, &mockDBForNotification{}, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "dispute.opened", Payload: []byte("{invalid json"),
	})
	if err == nil {
		t.Fatal("expected error for invalid payload, got nil")
	}
	if !strings.Contains(err.Error(), "unmarshal payload failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDisputeResolved_SellerFail_BuyerSuccess_ReturnsError(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	payload, _ := json.Marshal(map[string]interface{}{
		"dispute_id": uuid.New().String(),
		"order_id":   orderID.String(),
		"buyer_id":   buyerID.String(),
		"seller_id":  sellerID.String(),
		"resolution": "refund",
		"status":     "resolved",
	})

	call := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
			return fn(&mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					call++
					if call == 1 {
						return &mockRowForNotification{err: errors.New("seller insert failed")}
					}
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			})
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: events.EventDisputeResolved, Payload: payload,
	})
	if err == nil {
		t.Fatal("expected error for seller insert failure, got nil")
	}
	if !strings.Contains(err.Error(), "seller insert failed") {
		t.Errorf("error = %v, want seller insert failure", err)
	}
}

// =============================================================================
// MONEY.REFUND_FAILED ADMIN FANOUT
// =============================================================================

func makeRefundFailedPayload(refundID, orderID uuid.UUID, amount int64) []byte {
	b, _ := json.Marshal(map[string]interface{}{
		"refund_id":      refundID,
		"order_id":       orderID,
		"gateway_status": "failed",
		"amount":         amount,
		"error":          "gateway timeout",
	})
	return b
}

func TestMoneyRefundFailed_AdminFanout(t *testing.T) {
	refundID := uuid.New()
	orderID := uuid.New()
	admin1 := uuid.New()
	admin2 := uuid.New()

	var recipients []uuid.UUID
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			tx := &mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, args ...any) pgx.Row {
					if len(args) >= 6 {
						recipients = append(recipients, args[1].(uuid.UUID))
					}
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			}
			return fn(tx)
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{
			"governance.alert.read": {admin1, admin2},
		},
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "money.refund_failed", Payload: makeRefundFailedPayload(refundID, orderID, 150000),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// 2 admins (no user notification — admin-only)
	if len(recipients) != 2 {
		t.Fatalf("insert count = %d, want 2 admins", len(recipients))
	}
	adminSet := map[uuid.UUID]bool{admin1: true, admin2: true}
	for _, r := range recipients {
		if !adminSet[r] {
			t.Errorf("unexpected recipient %s", r)
		}
	}
}

func TestMoneyRefundFailed_ZeroReviewers_NoError(t *testing.T) {
	refundID := uuid.New()
	orderID := uuid.New()

	dbCalls := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			dbCalls++
			tx := &mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			}
			return fn(tx)
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{}, // no reviewers
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "money.refund_failed", Payload: makeRefundFailedPayload(refundID, orderID, 50000),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if dbCalls != 0 {
		t.Errorf("WithTx called %d times, want 0 (no reviewers = no inserts)", dbCalls)
	}
}

func TestMoneyRefundFailed_CapabilityLookupError_ReturnsError(t *testing.T) {
	refundID := uuid.New()
	orderID := uuid.New()

	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			tx := &mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			}
			return fn(tx)
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		err: fmt.Errorf("connection refused"),
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "money.refund_failed", Payload: makeRefundFailedPayload(refundID, orderID, 100000),
	})
	if err == nil {
		t.Fatal("expected error for capability lookup failure, got nil")
	}
	if !strings.Contains(err.Error(), "list admins by capability") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMoneyRefundFailed_TotalFanoutFailure_ReturnsError(t *testing.T) {
	refundID := uuid.New()
	orderID := uuid.New()
	admin1 := uuid.New()

	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, _ func(dbpkg.Tx) error) error {
			return fmt.Errorf("db connection lost")
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{
			"governance.alert.read": {admin1},
		},
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "money.refund_failed", Payload: makeRefundFailedPayload(refundID, orderID, 100000),
	})
	if err == nil {
		t.Fatal("expected error for total admin fanout failure, got nil")
	}
	if !strings.Contains(err.Error(), "all 1 admin fanout inserts failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMoneyRefundFailed_PartialFanoutFailure_Succeeds(t *testing.T) {
	refundID := uuid.New()
	orderID := uuid.New()
	admin1 := uuid.New()
	admin2 := uuid.New()

	callCount := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			callCount++
			if callCount == 1 {
				// First admin insert fails
				return fmt.Errorf("transient error")
			}
			// Second admin succeeds
			tx := &mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			}
			return fn(tx)
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{
			"governance.alert.read": {admin1, admin2},
		},
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "money.refund_failed", Payload: makeRefundFailedPayload(refundID, orderID, 75000),
	})
	if err != nil {
		t.Fatalf("partial failure should not return error, got: %v", err)
	}
	if callCount != 2 {
		t.Errorf("WithTx calls = %d, want 2", callCount)
	}
}

func TestMoneyRefundFailed_ReplayIdempotent(t *testing.T) {
	refundID := uuid.New()
	orderID := uuid.New()
	admin1 := uuid.New()

	insertCount := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			tx := &mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					insertCount++
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			}
			return fn(tx)
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{
			"governance.alert.read": {admin1},
		},
	})

	event := platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "money.refund_failed", Payload: makeRefundFailedPayload(refundID, orderID, 30000),
	}

	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("first Handle() error = %v", err)
	}
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("replay Handle() error = %v", err)
	}

	// 2 deliveries × 1 admin = 2 inserts. DB dedup handles actual duplicates.
	if insertCount != 2 {
		t.Errorf("insertCount = %d, want 2", insertCount)
	}
}

func TestMoneyRefundFailed_OtherMoneyEventsNoFanout(t *testing.T) {
	// Proves that other money.* events (money.released, money.refund_pending,
	// money.refund_succeeded) are NOT handled by the notification handler.
	// Only money.refund_failed gets admin fanout.
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			t.Fatal("should not attempt DB insert for unhandled money.* event")
			return nil
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{
			"governance.alert.read": {uuid.New()},
		},
	})

	otherPayload, _ := json.Marshal(map[string]interface{}{
		"order_id": uuid.New(),
		"amount":   100000,
	})

	for _, eventType := range []string{"money.released", "money.refund_pending", "money.refund_succeeded"} {
		err := h.Handle(context.Background(), platformevent.OutboxEvent{
			ID: uuid.New(), EventType: eventType, Payload: otherPayload,
		})
		if err != nil {
			t.Fatalf("Handle(%s) error = %v", eventType, err)
		}
	}
}

func TestInsertNotification_DedupReturnsNil(t *testing.T) {
	// Verify that ON CONFLICT DO NOTHING (ErrNoRows from Scan) is treated as
	// success, not as an error. This is the dedup safety net for outbox replay.
	inserter := NewNotificationServiceInserter()

	dedupTx := &mockTxForNotification{
		QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRowForNotification{err: pgx.ErrNoRows}
		},
	}

	id, err := inserter.InsertNotification(
		context.Background(), dedupTx,
		uuid.New(), uuid.Nil, "withdrawal.requested", uuid.New(),
		map[string]interface{}{"test": true},
	)
	if err != nil {
		t.Fatalf("dedup insert should return nil error, got: %v", err)
	}
	if id != uuid.Nil {
		t.Errorf("dedup insert should return uuid.Nil, got: %v", id)
	}
}

// =============================================================================
// MISSING CRITICAL NOTIFICATIONS: auction.bid.placed
// =============================================================================

func TestNotificationEventHandler_HandleRefundApproved(t *testing.T) {
	log := zaptest.NewLogger(t)

	buyerID := uuid.New()
	sellerID := uuid.New()
	orderID := uuid.New()
	refundID := uuid.New()

	payload, _ := json.Marshal(map[string]interface{}{
		"refund_id": refundID.String(),
		"order_id":  orderID.String(),
		"buyer_id":  buyerID.String(),
		"seller_id": sellerID.String(),
		"status":    "seller_approved",
	})

	var insertedRecipientID, insertedActorID uuid.UUID
	var insertedType string
	var insertedEntityID uuid.UUID

	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
			mockTx := &mockTxForNotification{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
					if len(args) >= 6 {
						insertedRecipientID = args[1].(uuid.UUID)
						insertedActorID = args[2].(uuid.UUID)
						insertedType = args[3].(string)
						insertedEntityID = args[4].(uuid.UUID)
					}
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			}
			return fn(mockTx)
		},
	}

	inserter := NewNotificationServiceInserter()
	handler := NewNotificationEventHandler(mockDB, &mockBlockCheckerForNotification{}, inserter, &mockPushSenderForNotification{}, &mockAccountStatusCheckerForNotification{}, log)

	err := handler.Handle(context.Background(), platformevent.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "refund",
		AggregateID:   refundID,
		EventType:     "refund.approved",
		Payload:       payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if insertedRecipientID != buyerID {
		t.Errorf("recipient_id = %s, want buyer %s", insertedRecipientID, buyerID)
	}
	if insertedActorID != sellerID {
		t.Errorf("actor_id = %s, want seller %s", insertedActorID, sellerID)
	}
	if insertedType != "refund.approved" {
		t.Errorf("type = %s, want refund.approved", insertedType)
	}
	if insertedEntityID != orderID {
		t.Errorf("entity_id = %s, want order %s", insertedEntityID, orderID)
	}
}

// TestNotificationEventHandler_HandleRefundRejected tests handling refund.rejected events.
// Seller rejects refund → buyer gets notified.
func TestNotificationEventHandler_HandleRefundRejected(t *testing.T) {
	log := zaptest.NewLogger(t)

	buyerID := uuid.New()
	sellerID := uuid.New()
	orderID := uuid.New()
	refundID := uuid.New()

	payload, _ := json.Marshal(map[string]interface{}{
		"refund_id": refundID.String(),
		"order_id":  orderID.String(),
		"buyer_id":  buyerID.String(),
		"seller_id": sellerID.String(),
		"status":    "seller_rejected",
	})

	var insertedRecipientID, insertedActorID uuid.UUID
	var insertedType string
	var insertedEntityID uuid.UUID

	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
			mockTx := &mockTxForNotification{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
					if len(args) >= 6 {
						insertedRecipientID = args[1].(uuid.UUID)
						insertedActorID = args[2].(uuid.UUID)
						insertedType = args[3].(string)
						insertedEntityID = args[4].(uuid.UUID)
					}
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			}
			return fn(mockTx)
		},
	}

	inserter := NewNotificationServiceInserter()
	handler := NewNotificationEventHandler(mockDB, &mockBlockCheckerForNotification{}, inserter, &mockPushSenderForNotification{}, &mockAccountStatusCheckerForNotification{}, log)

	err := handler.Handle(context.Background(), platformevent.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "refund",
		AggregateID:   refundID,
		EventType:     "refund.rejected",
		Payload:       payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if insertedRecipientID != buyerID {
		t.Errorf("recipient_id = %s, want buyer %s", insertedRecipientID, buyerID)
	}
	if insertedActorID != sellerID {
		t.Errorf("actor_id = %s, want seller %s", insertedActorID, sellerID)
	}
	if insertedType != "refund.rejected" {
		t.Errorf("type = %s, want refund.rejected", insertedType)
	}
	if insertedEntityID != orderID {
		t.Errorf("entity_id = %s, want order %s", insertedEntityID, orderID)
	}
}

// TestNotificationEventHandler_RefundApproved_InvalidPayload tests invalid JSON.
func TestNotificationEventHandler_RefundApproved_InvalidPayload(t *testing.T) {
	log := zaptest.NewLogger(t)

	mockDB := &mockDBForNotification{}
	inserter := NewNotificationServiceInserter()
	handler := NewNotificationEventHandler(mockDB, &mockBlockCheckerForNotification{}, inserter, nil, &mockAccountStatusCheckerForNotification{}, log)

	err := handler.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "refund.approved",
		Payload:   []byte(`{invalid json`),
	})
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// TestNotificationEventHandler_RefundRejected_InvalidOrderID tests invalid order_id.
func TestNotificationEventHandler_RefundRejected_InvalidOrderID(t *testing.T) {
	log := zaptest.NewLogger(t)

	payload, _ := json.Marshal(map[string]interface{}{
		"refund_id": uuid.New().String(),
		"order_id":  "not-a-uuid",
		"buyer_id":  uuid.New().String(),
		"seller_id": uuid.New().String(),
		"status":    "seller_rejected",
	})

	mockDB := &mockDBForNotification{}
	inserter := NewNotificationServiceInserter()
	handler := NewNotificationEventHandler(mockDB, &mockBlockCheckerForNotification{}, inserter, nil, &mockAccountStatusCheckerForNotification{}, log)

	err := handler.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "refund.rejected",
		Payload:   payload,
	})
	if err == nil {
		t.Fatal("expected error for invalid order_id, got nil")
	}
}

// =============================================================================
// N4-A1: ORDER LIFECYCLE BYPASS MIGRATION TESTS
// Proves that migrated handlers correctly use insertNotificationWithPolicy:
//   - dual-notification handlers still notify both parties
//   - dual-push for order.created and order.completed (buyer push was missing pre-N4)
//   - blocked actor relationship anonymizes actorID (CommerceCritical)
//   - unblocked actor preserved
//   - delivery log invoked
//   - support.ticket.user_responded routes to admin recipient
// =============================================================================

// multiInsertDB captures every InsertNotification call in order, for multi-recipient handlers.
type multiInsertDB struct {
	mu      sync.Mutex
	records []n4InsertRecord
}

type n4InsertRecord struct {
	recipient uuid.UUID
	actor     uuid.UUID
	notifType string
	data      map[string]any
}

func (d *multiInsertDB) WithTx(_ context.Context, fn func(dbpkg.Tx) error) error {
	tx := &mockTxForNotification{
		QueryRowFunc: func(_ context.Context, _ string, args ...any) pgx.Row {
			if len(args) >= 4 {
				d.mu.Lock()
				var data map[string]any
				if len(args) >= 6 {
					if v, ok := args[5].(map[string]any); ok {
						data = v
					}
				}
				d.records = append(d.records, n4InsertRecord{
					recipient: args[1].(uuid.UUID),
					actor:     args[2].(uuid.UUID),
					notifType: args[3].(string),
					data:      data,
				})
				d.mu.Unlock()
			}
			return &mockRowForNotification{scanValue: uuid.New()}
		},
	}
	return fn(tx)
}

func (d *multiInsertDB) count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.records)
}

func (d *multiInsertDB) at(i int) n4InsertRecord {
	d.mu.Lock()
	defer d.mu.Unlock()
	if i >= len(d.records) {
		return n4InsertRecord{}
	}
	return d.records[i]
}

// pushCountSender counts SendNotification calls. Uses WaitGroup for synchronisation
// because order.created / order.completed fire the buyer push in a goroutine.
type pushCountSender struct {
	mu    sync.Mutex
	wg    sync.WaitGroup
	count int
}

func (p *pushCountSender) SendNotification(_ context.Context, _ interface{}, _ interface{}, _, _ string) error {
	p.mu.Lock()
	p.count++
	p.mu.Unlock()
	p.wg.Done()
	return nil
}

func (p *pushCountSender) pushCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.count
}

// buildN4Handler constructs a handler with active account status and configurable block/push/logger.
func buildN4Handler(t *testing.T, db *multiInsertDB, block BlockChecker, push PushSender, logger DeliveryLogger) *NotificationEventHandler {
	t.Helper()
	h := NewNotificationEventHandler(db, block, NewNotificationServiceInserter(), push, &mockAccountStatusCheckerForNotification{}, zaptest.NewLogger(t))
	if logger != nil {
		h.SetDeliveryLogger(logger)
	}
	return h
}

func makeOrderPayloadN4(orderID, buyerID, sellerID uuid.UUID) []byte {
	b, _ := json.Marshal(OrderPayload{
		OrderID:  orderID.String(),
		BuyerID:  buyerID.String(),
		SellerID: sellerID.String(),
	})
	return b
}

// TestN4A1_OrderCreated_DualNotification_DualPush proves:
//   - 2 DB inserts: seller (actor=buyer) + buyer (actor=buyer, type=order.created.buyer)
//   - 2 push calls: seller via Handle() dispatch + buyer via inline goroutine
func TestRefundEscalated_DedupSeller_BuyerInserted_PushOnce_NoFailure(t *testing.T) {
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	refundID := uuid.New()

	payload, _ := json.Marshal(map[string]interface{}{
		"refund_id": refundID.String(),
		"order_id":  orderID.String(),
		"buyer_id":  buyerID.String(),
		"seller_id": sellerID.String(),
		"status":    "escalated_to_admin",
	})

	call := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
			return fn(&mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					call++
					if call == 1 {
						// Seller dedup (no insert, but success).
						return &mockRowForNotification{err: pgx.ErrNoRows}
					}
					// Buyer fresh insert.
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			})
		},
	}

	push := &pushCountSender{}
	push.wg.Add(1) // buyer push only
	h := NewNotificationEventHandler(
		mockDB,
		&mockBlockCheckerForNotification{},
		NewNotificationServiceInserter(),
		push,
		&mockAccountStatusCheckerForNotification{},
		zaptest.NewLogger(t),
	)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "refund.escalated", Payload: payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	push.wg.Wait()
	if got := push.pushCount(); got != 1 {
		t.Errorf("push count = %d, want 1 (dedup seller should not push; buyer inserted should push)", got)
	}
}

// =============================================================================
// B1: SELLER TIER + NEGOTIATION.CANCELLED HANDLER TESTS
// =============================================================================

func makeSellerTierPayload(sellerID uuid.UUID, previousTier, newTier string) []byte {
	b, _ := json.Marshal(SellerTierChangedPayload{
		SellerID:     sellerID.String(),
		PreviousTier: previousTier,
		NewTier:      newTier,
		EvaluatedAt:  "2026-05-28T00:00:00Z",
		WindowDays:   90,
	})
	return b
}

func makeNegotiationCancelledPayload(sessionID, buyerID, sellerID, chatRoomID uuid.UUID) []byte {
	b, _ := json.Marshal(NegotiationPayload{
		SessionID:  sessionID.String(),
		ChatRoomID: chatRoomID.String(),
		BuyerID:    buyerID.String(),
		SellerID:   sellerID.String(),
	})
	return b
}

// TestB1_SellerTierUpgraded_SellerNotified proves:
//   - seller.tier.upgraded notifies the seller
//   - CommerceCritical category → allowPush=true
//   - actor is uuid.Nil (system-initiated)
func TestDisputeOverdue_AdminFanout(t *testing.T) {
	disputeID := uuid.New()
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	admin1 := uuid.New()
	admin2 := uuid.New()

	var recipients []uuid.UUID
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			tx := &mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, args ...any) pgx.Row {
					if len(args) >= 6 {
						recipients = append(recipients, args[1].(uuid.UUID))
					}
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			}
			return fn(tx)
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{
			"finance.dispute.resolve": {admin1, admin2},
		},
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "dispute.overdue",
		Payload: makeDisputeOverduePayload(disputeID, orderID, buyerID, sellerID, 4),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Admin-only: 2 admins notified, 0 buyer/seller
	if len(recipients) != 2 {
		t.Fatalf("insert count = %d, want 2 admins", len(recipients))
	}
	adminSet := map[uuid.UUID]bool{admin1: true, admin2: true}
	for _, r := range recipients {
		if !adminSet[r] {
			t.Errorf("unexpected recipient %s — buyer/seller must NOT receive dispute.overdue", r)
		}
	}
}

func TestDisputeOverdue_ZeroReviewers_NoError(t *testing.T) {
	disputeID := uuid.New()
	orderID := uuid.New()

	dbCalls := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			dbCalls++
			tx := &mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			}
			return fn(tx)
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{}, // no reviewers
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "dispute.overdue",
		Payload: makeDisputeOverduePayload(disputeID, orderID, uuid.New(), uuid.New(), 4),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if dbCalls != 0 {
		t.Errorf("WithTx called %d times, want 0 (no reviewers = no inserts)", dbCalls)
	}
}

func TestDisputeOverdue_NoCapabilityLister_NoError(t *testing.T) {
	disputeID := uuid.New()
	orderID := uuid.New()

	dbCalls := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			dbCalls++
			tx := &mockTxForNotification{}
			return fn(tx)
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	// No SetCapabilityLister — simulates nil lister

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "dispute.overdue",
		Payload: makeDisputeOverduePayload(disputeID, orderID, uuid.New(), uuid.New(), 4),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if dbCalls != 0 {
		t.Errorf("WithTx called %d times, want 0 (nil lister = no inserts)", dbCalls)
	}
}

func TestDisputeTimeoutEscalation_AdminFanout(t *testing.T) {
	disputeID := uuid.New()
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	admin1 := uuid.New()
	admin2 := uuid.New()
	admin3 := uuid.New()

	var recipients []uuid.UUID
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			tx := &mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, args ...any) pgx.Row {
					if len(args) >= 6 {
						recipients = append(recipients, args[1].(uuid.UUID))
					}
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			}
			return fn(tx)
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{
			"finance.dispute.resolve": {admin1, admin2, admin3},
		},
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "dispute.timeout_escalation",
		Payload: makeDisputeTimeoutEscalationPayload(disputeID, orderID, buyerID, sellerID, 15, 14),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Admin-only: 3 admins notified, 0 buyer/seller
	if len(recipients) != 3 {
		t.Fatalf("insert count = %d, want 3 admins", len(recipients))
	}
	adminSet := map[uuid.UUID]bool{admin1: true, admin2: true, admin3: true}
	for _, r := range recipients {
		if !adminSet[r] {
			t.Errorf("unexpected recipient %s — buyer/seller must NOT receive dispute.timeout_escalation", r)
		}
	}
}

func TestDisputeTimeoutEscalation_TotalFanoutFailure_ReturnsError(t *testing.T) {
	disputeID := uuid.New()
	orderID := uuid.New()
	admin1 := uuid.New()

	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			tx := &mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return &mockRowForNotification{err: errors.New("db error")}
				},
			}
			return fn(tx)
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{
			"finance.dispute.resolve": {admin1},
		},
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "dispute.timeout_escalation",
		Payload: makeDisputeTimeoutEscalationPayload(disputeID, orderID, uuid.New(), uuid.New(), 15, 14),
	})
	if err == nil {
		t.Fatal("Handle() should return error on total fanout failure")
	}
	if !strings.Contains(err.Error(), "all") || !strings.Contains(err.Error(), "admin fanout") {
		t.Errorf("error message = %q, want 'all ... admin fanout ...'", err.Error())
	}
}

func TestDisputeOverdue_CapabilityLookupError_ReturnsError(t *testing.T) {
	disputeID := uuid.New()
	orderID := uuid.New()

	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			tx := &mockTxForNotification{}
			return fn(tx)
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		err: errors.New("capability lookup failure"),
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "dispute.overdue",
		Payload: makeDisputeOverduePayload(disputeID, orderID, uuid.New(), uuid.New(), 4),
	})
	if err == nil {
		t.Fatal("Handle() should return error on capability lookup failure")
	}
	if !strings.Contains(err.Error(), "list admins by capability") {
		t.Errorf("error message = %q, want to contain 'list admins by capability'", err.Error())
	}
}

func TestDisputeTimeoutEscalation_ZeroReviewers_NoError(t *testing.T) {
	disputeID := uuid.New()
	orderID := uuid.New()

	dbCalls := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			dbCalls++
			tx := &mockTxForNotification{}
			return fn(tx)
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{}, // no reviewers
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "dispute.timeout_escalation",
		Payload: makeDisputeTimeoutEscalationPayload(disputeID, orderID, uuid.New(), uuid.New(), 15, 14),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if dbCalls != 0 {
		t.Errorf("WithTx called %d times, want 0 (no reviewers = no inserts)", dbCalls)
	}
}



