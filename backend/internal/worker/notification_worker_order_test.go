package worker

import (
	"context"
	"encoding/json"
	"errors"
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

func TestOrderCancelledTimeout_BothPartiesNotified(t *testing.T) {
	buyerID := uuid.New()
	sellerID := uuid.New()
	orderID := uuid.New()

	payload, _ := json.Marshal(OrderPayload{
		OrderID:  orderID.String(),
		BuyerID:  buyerID.String(),
		SellerID: sellerID.String(),
		Status:   "cancelled",
	})

	var recipients []uuid.UUID
	var actors []uuid.UUID
	var types []string
	var entityIDs []uuid.UUID

	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(dbpkg.Tx) error) error {
			return fn(&mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, args ...any) pgx.Row {
					if len(args) >= 6 {
						recipients = append(recipients, args[1].(uuid.UUID))
						actors = append(actors, args[2].(uuid.UUID))
						types = append(types, args[3].(string))
						entityIDs = append(entityIDs, args[4].(uuid.UUID))
					}
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			})
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "order.cancelled_timeout", Payload: payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Both seller and buyer should be notified.
	if len(recipients) != 2 {
		t.Fatalf("got %d notifications, want 2 (seller + buyer)", len(recipients))
	}
	if recipients[0] != sellerID {
		t.Errorf("first recipient = %s, want seller %s", recipients[0], sellerID)
	}
	if recipients[1] != buyerID {
		t.Errorf("second recipient = %s, want buyer %s", recipients[1], buyerID)
	}
	// Both should be system-initiated (uuid.Nil actor).
	for i, a := range actors {
		if a != uuid.Nil {
			t.Errorf("actor[%d] = %s, want uuid.Nil (system-initiated)", i, a)
		}
	}
	for i, typ := range types {
		if typ != "order.cancelled_timeout" {
			t.Errorf("type[%d] = %s, want order.cancelled_timeout", i, typ)
		}
	}
	for i, eid := range entityIDs {
		if eid != orderID {
			t.Errorf("entityID[%d] = %s, want %s", i, eid, orderID)
		}
	}
}

func TestOrderCancelledTimeout_InvalidPayload(t *testing.T) {
	h := buildSocialGovernanceHandler(t,
		&mockDBForNotification{},
		&mockAccountStatusControlled{},
		&mockBlockCheckerControlled{},
		nil,
	)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "order.cancelled_timeout", Payload: []byte("invalid json"),
	})
	if err == nil {
		t.Fatal("expected error for invalid payload, got nil")
	}
}

func TestOrderCancelledTimeout_InvalidOrderID(t *testing.T) {
	payload, _ := json.Marshal(OrderPayload{
		OrderID:  "not-a-uuid",
		BuyerID:  uuid.New().String(),
		SellerID: uuid.New().String(),
	})

	h := buildSocialGovernanceHandler(t,
		&mockDBForNotification{},
		&mockAccountStatusControlled{},
		&mockBlockCheckerControlled{},
		nil,
	)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "order.cancelled_timeout", Payload: payload,
	})
	if err == nil {
		t.Fatal("expected error for invalid order_id, got nil")
	}
}

func TestOrderCancelledTimeout_SellerFail_BuyerSuccess_ReturnsError(t *testing.T) {
	buyerID := uuid.New()
	sellerID := uuid.New()
	orderID := uuid.New()

	payload, _ := json.Marshal(OrderPayload{
		OrderID:  orderID.String(),
		BuyerID:  buyerID.String(),
		SellerID: sellerID.String(),
		Status:   "cancelled",
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
		ID: uuid.New(), EventType: "order.cancelled_timeout", Payload: payload,
	})
	if err == nil {
		t.Fatal("expected error for seller insert failure, got nil")
	}
	if !strings.Contains(err.Error(), "seller insert failed") {
		t.Errorf("error = %v, want seller insert failure", err)
	}
}

// =============================================================================
// MISSING CRITICAL NOTIFICATIONS: support.ticket.created
// =============================================================================

func TestN4A1_OrderCreated_DualNotification_DualPush(t *testing.T) {
	orderID, buyerID, sellerID := uuid.New(), uuid.New(), uuid.New()
	db := &multiInsertDB{}
	push := &pushCountSender{}
	push.wg.Add(2) // seller push (Handle dispatch) + buyer push (inline goroutine)

	h := buildN4Handler(t, db, &mockBlockCheckerForNotification{}, push, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: events.EventOrderCreated,
		Payload:   makeOrderPayloadN4(orderID, buyerID, sellerID),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	push.wg.Wait()

	if db.count() != 2 {
		t.Fatalf("insert count = %d, want 2 (seller + buyer)", db.count())
	}
	s := db.at(0)
	if s.recipient != sellerID {
		t.Errorf("insert[0].recipient = %s, want sellerID", s.recipient)
	}
	if s.actor != buyerID {
		t.Errorf("insert[0].actor = %s, want buyerID", s.actor)
	}
	if s.notifType != events.EventOrderCreated {
		t.Errorf("insert[0].type = %s, want %s", s.notifType, events.EventOrderCreated)
	}
	b := db.at(1)
	if b.recipient != buyerID {
		t.Errorf("insert[1].recipient = %s, want buyerID", b.recipient)
	}
	if b.notifType != "order.created.buyer" {
		t.Errorf("insert[1].type = %s, want order.created.buyer", b.notifType)
	}
	if push.pushCount() != 2 {
		t.Errorf("push count = %d, want 2", push.pushCount())
	}
}

// TestN4A1_OrderCompleted_DualNotification_BuyerPushRestored proves:
//   - 2 DB inserts: seller (actor=buyer) + buyer (actor=seller)
//   - 2 push calls — buyer push was MISSING before N4-A1 migration
func TestN4A1_OrderCompleted_DualNotification_BuyerPushRestored(t *testing.T) {
	orderID, buyerID, sellerID := uuid.New(), uuid.New(), uuid.New()
	db := &multiInsertDB{}
	push := &pushCountSender{}
	push.wg.Add(2) // seller push (Handle dispatch) + buyer push (inline goroutine)

	h := buildN4Handler(t, db, &mockBlockCheckerForNotification{}, push, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: events.EventOrderCompleted,
		Payload:   makeOrderPayloadN4(orderID, buyerID, sellerID),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	push.wg.Wait()

	if db.count() != 2 {
		t.Fatalf("insert count = %d, want 2 (seller + buyer)", db.count())
	}
	s := db.at(0)
	if s.recipient != sellerID {
		t.Errorf("insert[0].recipient = %s, want sellerID", s.recipient)
	}
	if s.notifType != events.EventOrderCompleted {
		t.Errorf("insert[0].type = %s, want %s", s.notifType, events.EventOrderCompleted)
	}
	b := db.at(1)
	if b.recipient != buyerID {
		t.Errorf("insert[1].recipient = %s, want buyerID", b.recipient)
	}
	if b.notifType != events.EventOrderCompleted {
		t.Errorf("insert[1].type = %s, want %s", b.notifType, events.EventOrderCompleted)
	}
	if push.pushCount() != 2 {
		t.Errorf("push count = %d, want 2 (buyer push was missing before N4-A1)", push.pushCount())
	}
}

// TestN4A1_OrderShipped_Blocked_ActorAnonymized proves:
//   - CommerceCritical: delivery is NOT blocked even when buyer blocked seller
//   - Actor is anonymized to uuid.Nil in the INSERT when a block relationship exists
func TestN4A1_OrderShipped_Blocked_ActorAnonymized(t *testing.T) {
	orderID, buyerID, sellerID := uuid.New(), uuid.New(), uuid.New()
	db := &multiInsertDB{}

	h := buildN4Handler(t, db, &mockBlockCheckerBlocked{}, &mockPushSenderForNotification{}, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "order.shipped",
		Payload:   makeOrderPayloadN4(orderID, buyerID, sellerID),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if db.count() != 1 {
		t.Fatalf("insert count = %d, want 1", db.count())
	}
	ins := db.at(0)
	if ins.recipient != buyerID {
		t.Errorf("recipient = %s, want buyerID %s", ins.recipient, buyerID)
	}
	// Block present → actor anonymized to uuid.Nil for CommerceCritical
	if ins.actor != uuid.Nil {
		t.Errorf("actor = %s, want uuid.Nil (anonymized due to block)", ins.actor)
	}
}

// TestN4A1_OrderShipped_Unblocked_ActorPreserved proves:
//   - Without a block relationship sellerID is preserved as actor
func TestN4A1_OrderShipped_Unblocked_ActorPreserved(t *testing.T) {
	orderID, buyerID, sellerID := uuid.New(), uuid.New(), uuid.New()
	db := &multiInsertDB{}

	h := buildN4Handler(t, db, &mockBlockCheckerForNotification{}, nil, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "order.shipped",
		Payload:   makeOrderPayloadN4(orderID, buyerID, sellerID),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if db.count() != 1 {
		t.Fatalf("insert count = %d, want 1", db.count())
	}
	ins := db.at(0)
	if ins.actor != sellerID {
		t.Errorf("actor = %s, want sellerID %s (preserved when no block)", ins.actor, sellerID)
	}
}

// TestN4A1_SupportTicketUserResponded_DeliversToAdmin proves:
//   - admin is the recipient
//   - userID is the actor
func TestN4A1_SupportTicketUserResponded_DeliversToAdmin(t *testing.T) {
	ticketID, userID, adminID := uuid.New(), uuid.New(), uuid.New()
	db := &multiInsertDB{}

	payload, _ := json.Marshal(SupportTicketPayload{
		TicketID: ticketID.String(),
		UserID:   userID.String(),
		AdminID:  adminID.String(),
	})

	h := buildN4Handler(t, db, &mockBlockCheckerForNotification{}, nil, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "support.ticket.user_responded",
		Payload:   payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if db.count() != 1 {
		t.Fatalf("insert count = %d, want 1", db.count())
	}
	ins := db.at(0)
	if ins.recipient != adminID {
		t.Errorf("recipient = %s, want adminID %s", ins.recipient, adminID)
	}
	if ins.actor != userID {
		t.Errorf("actor = %s, want userID %s", ins.actor, userID)
	}
	if ins.notifType != "support.ticket.user_responded" {
		t.Errorf("type = %s, want support.ticket.user_responded", ins.notifType)
	}
}

// TestN4A1_OrderShipped_DeliveryLogInvoked proves:
//   - insertNotificationWithPolicy writes an in_app "sent" event to delivery log
//   - migrated order handlers now have an audit trail (were bypass before N4-A1)
func TestN4A1_OrderShipped_DeliveryLogInvoked(t *testing.T) {
	orderID, buyerID, sellerID := uuid.New(), uuid.New(), uuid.New()
	db := &multiInsertDB{}

	logger := &mockDeliveryCapture{}
	logger.wg.Add(1) // 1 in_app log for the single insert

	h := buildN4Handler(t, db, &mockBlockCheckerForNotification{}, nil, logger)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "order.shipped",
		Payload:   makeOrderPayloadN4(orderID, buyerID, sellerID),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	logger.wg.Wait()

	status := logger.inAppStatus()
	if status != "sent" {
		t.Errorf("in_app delivery status = %q, want %q", status, "sent")
	}
}

// TestNotificationEventHandler_RefundDecision_TitleAndBody tests title/body for refund decision types.
func TestNotificationEventHandler_RefundDecision_TitleAndBody(t *testing.T) {
	log := zaptest.NewLogger(t)
	mockDB := &mockDBForNotification{}
	inserter := NewNotificationServiceInserter()
	handler := NewNotificationEventHandler(mockDB, nil, inserter, nil, nil, log)

	tests := []struct {
		eventType string
		wantTitle string
		wantBody  string
	}{
		{"refund.approved", "Refund Disetujui Penjual", "Refund disetujui. Proses pengembalian dana sedang berjalan."},
		{"refund.rejected", "Refund Ditolak Penjual", "Anda dapat mengajukan sengketa ke admin."},
		{"refund.escalated", "Refund Dieskalasi", "Pembeli mengajukan sengketa ke admin."},
	}

	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			title, body := handler.getTitleAndBody(tt.eventType)
			if title != tt.wantTitle {
				t.Errorf("title = %q, want %q", title, tt.wantTitle)
			}
			if body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}

// =============================================================================
// N4-A2: WITHDRAWAL + VERIFICATION HANDLER MIGRATION TESTS
// =============================================================================

// capturePushSender records the title/body of the last push call.
// Used when we need to assert push content, not just count.
type capturePushSender struct {
	mu    sync.Mutex
	wg    sync.WaitGroup
	title string
	body  string
}

func (c *capturePushSender) SendNotification(_ context.Context, _ interface{}, _ interface{}, title, body string) error {
	c.mu.Lock()
	c.title = title
	c.body = body
	c.mu.Unlock()
	c.wg.Done()
	return nil
}

func (c *capturePushSender) lastTitle() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.title
}

func (c *capturePushSender) lastBody() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.body
}

func (c *capturePushSender) pushCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.title == "" && c.body == "" {
		return 0
	}
	return 1
}

// capturePushPayloadSender records the last push notification payload.
type capturePushPayloadSender struct {
	mu    sync.Mutex
	wg    sync.WaitGroup
	count int
	last  interface{}
}

func (c *capturePushPayloadSender) SendNotification(_ context.Context, _ interface{}, notification interface{}, _, _ string) error {
	c.mu.Lock()
	c.count++
	c.last = notification
	c.mu.Unlock()
	c.wg.Done()
	return nil
}

func (c *capturePushPayloadSender) pushCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.count
}

func (c *capturePushPayloadSender) lastPayload() map[string]interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	p, _ := c.last.(map[string]interface{})
	return p
}

func makeWithdrawalPayloadN4(withdrawalID, sellerID uuid.UUID) []byte {
	b, _ := json.Marshal(WithdrawalPayload{
		WithdrawalID: withdrawalID.String(),
		SellerID:     sellerID.String(),
		Amount:       50000,
	})
	return b
}

func makeVerificationDocPayloadN4(documentID, userID uuid.UUID, docType string) []byte {
	b, _ := json.Marshal(VerificationDocumentPayload{
		DocumentID:   documentID.String(),
		UserID:       userID.String(),
		DocumentType: docType,
	})
	return b
}

func makeSellerVerificationPayloadN4(sellerID uuid.UUID, status string) []byte {
	b, _ := json.Marshal(SellerVerificationPayload{
		SellerID: sellerID.String(),
		Status:   status,
	})
	return b
}

// TestN4A2_WithdrawalRequested_WrapperAllowPushLog proves:
//   - withdrawal.requested uses insertNotificationWithPolicy (multiInsertDB captures insert)
//   - allowPush=true: Handle() fires push goroutine
//   - delivery log written: in_app status "sent"


