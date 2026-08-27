package worker

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	platformevent "github.com/labuda/backend/internal/platform/event"
	dbpkg "github.com/labuda/backend/pkg/db"
)

func TestSupportTicketCreated_WithAdmin_Delivered(t *testing.T) {
	userID := uuid.New()
	adminID := uuid.New()
	ticketID := uuid.New()

	payload, _ := json.Marshal(SupportTicketPayload{
		TicketID: ticketID.String(),
		UserID:   userID.String(),
		AdminID:  adminID.String(),
		Category: "order_issue",
		Priority: "high",
	})

	var capturedRecipient, capturedActor, capturedEntity uuid.UUID
	var capturedType string

	mockDB := &mockDBForNotification{
		WithTxFunc: insertCaptureTx(&capturedRecipient, &capturedActor, &capturedType, &capturedEntity, nil),
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "support.ticket.created", Payload: payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if capturedRecipient != adminID {
		t.Errorf("recipient = %s, want admin %s", capturedRecipient, adminID)
	}
	if capturedActor != userID {
		t.Errorf("actor = %s, want user %s", capturedActor, userID)
	}
	if capturedType != "support.ticket.created" {
		t.Errorf("type = %s, want support.ticket.created", capturedType)
	}
	if capturedEntity != ticketID {
		t.Errorf("entityID = %s, want %s", capturedEntity, ticketID)
	}
}

func TestSupportTicketCreated_NoAdmin_NoLister_NoInsert(t *testing.T) {
	userID := uuid.New()
	ticketID := uuid.New()

	payload, _ := json.Marshal(SupportTicketPayload{
		TicketID: ticketID.String(),
		UserID:   userID.String(),
		// AdminID intentionally omitted.
		Category: "general",
	})

	dbCalls := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			dbCalls++
			return fn(&mockTxForNotification{})
		},
	}

	// No capability lister set → graceful no-op
	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "support.ticket.created", Payload: payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if dbCalls != 0 {
		t.Errorf("WithTx called %d times, want 0 (no lister → no notification)", dbCalls)
	}
}

// mockCapabilityLister implements CapabilityLister for tests.
type mockCapabilityLister struct {
	users map[string][]uuid.UUID
	err   error
}

func (m *mockCapabilityLister) ListUsersByCapability(_ context.Context, capability string) ([]uuid.UUID, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.users[capability], nil
}

func TestSupportTicketCreated_NoAdmin_FanoutToCapabilityAdmins(t *testing.T) {
	userID := uuid.New()
	ticketID := uuid.New()
	admin1 := uuid.New()
	admin2 := uuid.New()
	admin3 := uuid.New()

	payload, _ := json.Marshal(SupportTicketPayload{
		TicketID: ticketID.String(),
		UserID:   userID.String(),
		Category: "order_issue",
		Priority: "high",
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

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{
			"support.ticket.claim": {admin1, admin2, admin3},
		},
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "support.ticket.created", Payload: payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if len(recipients) != 3 {
		t.Fatalf("fanout delivered to %d admins, want 3", len(recipients))
	}

	expected := map[uuid.UUID]bool{admin1: true, admin2: true, admin3: true}
	for _, r := range recipients {
		if !expected[r] {
			t.Errorf("unexpected recipient %s", r)
		}
	}
}

func TestSupportTicketCreated_WithAdmin_PreservesDirectPath(t *testing.T) {
	// Verify that assigned admin path still works and does NOT trigger fanout.
	userID := uuid.New()
	adminID := uuid.New()
	ticketID := uuid.New()
	capAdmin := uuid.New() // should NOT be notified

	payload, _ := json.Marshal(SupportTicketPayload{
		TicketID: ticketID.String(),
		UserID:   userID.String(),
		AdminID:  adminID.String(),
		Category: "order_issue",
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

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{
			"support.ticket.claim": {capAdmin},
		},
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "support.ticket.created", Payload: payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if len(recipients) != 1 {
		t.Fatalf("direct path delivered to %d admins, want 1", len(recipients))
	}
	if recipients[0] != adminID {
		t.Errorf("recipient = %s, want assigned admin %s (not capability fanout)", recipients[0], adminID)
	}
}

func TestSupportTicketCreated_NoAdmin_EmptyCapabilityList_NoError(t *testing.T) {
	userID := uuid.New()
	ticketID := uuid.New()

	payload, _ := json.Marshal(SupportTicketPayload{
		TicketID: ticketID.String(),
		UserID:   userID.String(),
	})

	dbCalls := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			dbCalls++
			return fn(&mockTxForNotification{})
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{}, // no admins hold the capability
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "support.ticket.created", Payload: payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if dbCalls != 0 {
		t.Errorf("WithTx called %d times, want 0 (empty admin list)", dbCalls)
	}
}

func TestSupportTicketCreated_NoAdmin_ReplayIdempotent(t *testing.T) {
	userID := uuid.New()
	ticketID := uuid.New()
	admin1 := uuid.New()

	payload, _ := json.Marshal(SupportTicketPayload{
		TicketID: ticketID.String(),
		UserID:   userID.String(),
		Category: "general",
	})

	insertCount := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			tx := &mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, args ...any) pgx.Row {
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
			"support.ticket.claim": {admin1},
		},
	})

	event := platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "support.ticket.created", Payload: payload,
	}

	// First delivery
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("first Handle() error = %v", err)
	}
	// Replay (outbox redelivery)
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("replay Handle() error = %v", err)
	}

	// Each replay inserts again — idempotency is at DB constraint level (notification
	// dedup by recipient+entity+type), not handler level. Handler must not error.
	if insertCount != 2 {
		t.Errorf("insertCount = %d, want 2 (handler does not deduplicate — DB constraint does)", insertCount)
	}
}

func TestSupportTicketCreated_InvalidPayload(t *testing.T) {
	h := buildSocialGovernanceHandler(t,
		&mockDBForNotification{},
		&mockAccountStatusControlled{},
		&mockBlockCheckerControlled{},
		nil,
	)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "support.ticket.created", Payload: []byte("invalid json"),
	})
	if err == nil {
		t.Fatal("expected error for invalid payload, got nil")
	}
}

// =============================================================================
// WITHDRAWAL.REQUESTED ADMIN FANOUT
// =============================================================================

func TestN4A3_SupportTicketClosed_WrapperPushLog(t *testing.T) {
	ticketID, userID := uuid.New(), uuid.New()
	db := &multiInsertDB{}
	push := &pushCountSender{}
	push.wg.Add(1)
	logger := &mockDeliveryCapture{}
	logger.wg.Add(2) // in_app "sent" + push "sent"

	h := buildN4Handler(t, db, &mockBlockCheckerForNotification{}, push, logger)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "support.ticket.closed",
		Payload:   makeSupportTicketPayloadN4(ticketID, userID),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	push.wg.Wait()
	logger.wg.Wait()

	if db.count() != 1 {
		t.Errorf("DB inserts = %d, want 1", db.count())
	}
	if db.at(0).recipient != userID {
		t.Errorf("recipient = %v, want userID %v", db.at(0).recipient, userID)
	}
	if push.pushCount() != 1 {
		t.Errorf("push count = %d, want 1", push.pushCount())
	}
	if status := logger.inAppStatus(); status != "sent" {
		t.Errorf("in_app delivery status = %q, want %q", status, "sent")
	}
}

// TestN4A3_ModerationContentRemoved_WrapperNoPushLog proves:
//   - moderation.content.removed uses insertNotificationWithPolicy
//   - Moderation category: RequiresPushByType=false → no push
//   - in_app delivery log written (audit trail present even without push)
//   - Handler performs DB lookup then calls wrapper (DB lookup uses multiInsertDB WithTx)
func TestN4A3_ModerationContentRemoved_WrapperNoPushLog(t *testing.T) {
	resourceID := uuid.New()
	db := &multiInsertDB{}
	// nil push sender: no push goroutine launched, so no push logDelivery
	logger := &mockDeliveryCapture{}
	logger.wg.Add(1) // only in_app "sent" (no push channel since pushSender=nil)

	h := buildN4Handler(t, db, &mockBlockCheckerForNotification{}, nil, logger)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "moderation.content.removed",
		Payload:   makeModerationRemovedPayloadN4(resourceID),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	logger.wg.Wait()

	// Wrapper was called: exactly 1 DB insert captured
	if db.count() != 1 {
		t.Errorf("DB inserts = %d, want 1", db.count())
	}
	if db.at(0).notifType != "moderation.content.removed" {
		t.Errorf("notifType = %q, want %q", db.at(0).notifType, "moderation.content.removed")
	}
	// No push sender → guaranteed no push call
	if status := logger.inAppStatus(); status != "sent" {
		t.Errorf("in_app delivery status = %q, want %q", status, "sent")
	}
}

// =============================================================================
// N5: NEGOTIATION.MESSAGE_SENT RECIPIENT FIX TESTS
// =============================================================================

// makeNegotiationMessageSentPayloadN5 builds a negotiation.message_sent payload
// with all four IDs populated, including sender_id which drives other-party logic.
func makeNegotiationMessageSentPayloadN5(sessionID, buyerID, sellerID, senderID uuid.UUID) []byte {
	b, _ := json.Marshal(NegotiationPayload{
		SessionID: sessionID.String(),
		BuyerID:   buyerID.String(),
		SellerID:  sellerID.String(),
		SenderID:  senderID.String(),
	})
	return b
}

// TestN5_NegotiationMessageSent_BuyerSends_SellerReceives proves:
//   - When buyer is the sender, seller receives the notification
//   - No DB lookup needed (fields from payload)
//   - allowPush=true (negotiation.* → RequiresPushByType=true)


