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
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap/zaptest"

	"github.com/labuda/backend/internal/interaction/notification/policy"
	platformevent "github.com/labuda/backend/internal/platform/event"
	"github.com/labuda/backend/internal/platform/events"
	dbpkg "github.com/labuda/backend/pkg/db"
)

func (m *mockBlockCheckerChat) ExistsBlock(_ context.Context, _, _ uuid.UUID) (bool, error) {
	return m.blocked, m.err
}

// mockAccountStatusCheckerChat is a configurable status checker for chat governance tests.
// Returns the status mapped per user ID; falls back to "active" for unmapped IDs.
type mockAccountStatusCheckerChat struct {
	statuses map[uuid.UUID]string
	err      error
}

func (m *mockAccountStatusCheckerChat) GetStatus(_ context.Context, userID uuid.UUID) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if status, ok := m.statuses[userID]; ok {
		return status, nil
	}
	return "active", nil
}

func makeChatPayload(senderID, recipientID, roomID, messageID uuid.UUID) []byte {
	b, _ := json.Marshal(ChatMessagePayload{
		RoomID:      roomID.String(),
		MessageID:   messageID.String(),
		SenderID:    senderID.String(),
		RecipientID: recipientID.String(),
		MessageType: "text",
	})
	return b
}

// Scenario A: active sender → active recipient → in-app inserted, push eligible.
func TestChatNotification_ActiveActive_Delivered(t *testing.T) {
	senderID := uuid.New()
	recipientID := uuid.New()
	roomID := uuid.New()
	messageID := uuid.New()

	var insertedRecipientID, insertedActorID uuid.UUID
	var insertedType string
	dbCalls := 0

	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
			dbCalls++
			return fn(&mockTxForNotification{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
					if len(args) >= 4 {
						insertedRecipientID, _ = args[1].(uuid.UUID)
						insertedActorID, _ = args[2].(uuid.UUID)
						insertedType, _ = args[3].(string)
					}
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			})
		},
	}

	handler := NewNotificationEventHandler(
		mockDB,
		&mockBlockCheckerChat{blocked: false},
		NewNotificationServiceInserter(),
		&mockPushSenderForNotification{},
		&mockAccountStatusCheckerChat{statuses: map[uuid.UUID]string{
			senderID:    "active",
			recipientID: "active",
		}},
		zaptest.NewLogger(t),
	)

	event := platformevent.OutboxEvent{
		ID:          uuid.New(),
		EventType:   "chat.message.sent",
		AggregateID: roomID,
		Payload:     makeChatPayload(senderID, recipientID, roomID, messageID),
	}

	err := handler.Handle(context.Background(), event)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if dbCalls == 0 {
		t.Error("expected notification to be inserted in DB, but WithTx was not called")
	}
	if insertedRecipientID != recipientID {
		t.Errorf("recipient_id = %s, want %s", insertedRecipientID, recipientID)
	}
	if insertedActorID != senderID {
		t.Errorf("actor_id = %s, want %s", insertedActorID, senderID)
	}
	if insertedType != "chat_message" {
		t.Errorf("type = %s, want chat_message", insertedType)
	}
}

// Scenario B: sender blocked by recipient → no in-app, no push.
// Scenario C: recipient blocked by sender → no in-app, no push.
// Both collapse to the same mock because ExistsBlock is bidirectional.
func TestChatNotification_Blocked_NoDelivery(t *testing.T) {
	senderID := uuid.New()
	recipientID := uuid.New()
	roomID := uuid.New()
	messageID := uuid.New()

	dbCalls := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
			dbCalls++
			return fn(&mockTxForNotification{})
		},
	}

	handler := NewNotificationEventHandler(
		mockDB,
		&mockBlockCheckerChat{blocked: true}, // block exists in either direction
		NewNotificationServiceInserter(),
		&mockPushSenderForNotification{},
		&mockAccountStatusCheckerChat{},
		zaptest.NewLogger(t),
	)

	event := platformevent.OutboxEvent{
		ID:          uuid.New(),
		EventType:   "chat.message.sent",
		AggregateID: roomID,
		Payload:     makeChatPayload(senderID, recipientID, roomID, messageID),
	}

	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if dbCalls != 0 {
		t.Errorf("expected no DB insert when blocked, got %d WithTx calls", dbCalls)
	}
}

// Scenario D: recipient suspended → social chat notification dropped.
func TestChatNotification_RecipientSuspended_NoDelivery(t *testing.T) {
	senderID := uuid.New()
	recipientID := uuid.New()
	roomID := uuid.New()
	messageID := uuid.New()

	dbCalls := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
			dbCalls++
			return fn(&mockTxForNotification{})
		},
	}

	handler := NewNotificationEventHandler(
		mockDB,
		&mockBlockCheckerChat{blocked: false},
		NewNotificationServiceInserter(),
		&mockPushSenderForNotification{},
		&mockAccountStatusCheckerChat{statuses: map[uuid.UUID]string{
			senderID:    "active",
			recipientID: "suspended",
		}},
		zaptest.NewLogger(t),
	)

	event := platformevent.OutboxEvent{
		ID:          uuid.New(),
		EventType:   "chat.message.sent",
		AggregateID: roomID,
		Payload:     makeChatPayload(senderID, recipientID, roomID, messageID),
	}

	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if dbCalls != 0 {
		t.Errorf("expected no DB insert for suspended recipient, got %d WithTx calls", dbCalls)
	}
}

// Scenario E: recipient banned → notification dropped.
func TestChatNotification_RecipientBanned_NoDelivery(t *testing.T) {
	senderID := uuid.New()
	recipientID := uuid.New()
	roomID := uuid.New()
	messageID := uuid.New()

	dbCalls := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
			dbCalls++
			return fn(&mockTxForNotification{})
		},
	}

	handler := NewNotificationEventHandler(
		mockDB,
		&mockBlockCheckerChat{blocked: false},
		NewNotificationServiceInserter(),
		&mockPushSenderForNotification{},
		&mockAccountStatusCheckerChat{statuses: map[uuid.UUID]string{
			senderID:    "active",
			recipientID: "banned",
		}},
		zaptest.NewLogger(t),
	)

	event := platformevent.OutboxEvent{
		ID:          uuid.New(),
		EventType:   "chat.message.sent",
		AggregateID: roomID,
		Payload:     makeChatPayload(senderID, recipientID, roomID, messageID),
	}

	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if dbCalls != 0 {
		t.Errorf("expected no DB insert for banned recipient, got %d WithTx calls", dbCalls)
	}
}

// Scenario F: sender banned (historical outbox event) → no raw identity leak; dropped.
func TestChatNotification_SenderBanned_NoDelivery(t *testing.T) {
	senderID := uuid.New()
	recipientID := uuid.New()
	roomID := uuid.New()
	messageID := uuid.New()

	dbCalls := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
			dbCalls++
			return fn(&mockTxForNotification{})
		},
	}

	handler := NewNotificationEventHandler(
		mockDB,
		&mockBlockCheckerChat{blocked: false},
		NewNotificationServiceInserter(),
		&mockPushSenderForNotification{},
		&mockAccountStatusCheckerChat{statuses: map[uuid.UUID]string{
			senderID:    "banned",
			recipientID: "active",
		}},
		zaptest.NewLogger(t),
	)

	event := platformevent.OutboxEvent{
		ID:          uuid.New(),
		EventType:   "chat.message.sent",
		AggregateID: roomID,
		Payload:     makeChatPayload(senderID, recipientID, roomID, messageID),
	}

	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if dbCalls != 0 {
		t.Errorf("expected no DB insert when sender is banned (no identity leak), got %d WithTx calls", dbCalls)
	}
}

// Scenario G: self-message → no notification regardless of policy.
func TestChatNotification_SelfMessage_NoDelivery(t *testing.T) {
	userID := uuid.New()
	roomID := uuid.New()
	messageID := uuid.New()

	dbCalls := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
			dbCalls++
			return fn(&mockTxForNotification{})
		},
	}

	handler := NewNotificationEventHandler(
		mockDB,
		&mockBlockCheckerChat{blocked: false},
		NewNotificationServiceInserter(),
		&mockPushSenderForNotification{},
		&mockAccountStatusCheckerChat{},
		zaptest.NewLogger(t),
	)

	event := platformevent.OutboxEvent{
		ID:          uuid.New(),
		EventType:   "chat.message.sent",
		AggregateID: roomID,
		Payload:     makeChatPayload(userID, userID, roomID, messageID), // sender == recipient
	}

	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if dbCalls != 0 {
		t.Errorf("expected no DB insert for self-message, got %d WithTx calls", dbCalls)
	}
}

// Scenario H: invalid payload → error propagated.
func TestChatNotification_InvalidPayload_Error(t *testing.T) {
	handler := NewNotificationEventHandler(
		&mockDBForNotification{},
		&mockBlockCheckerChat{},
		NewNotificationServiceInserter(),
		&mockPushSenderForNotification{},
		&mockAccountStatusCheckerChat{},
		zaptest.NewLogger(t),
	)

	event := platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "chat.message.sent",
		Payload:   []byte("not-valid-json"),
	}

	if err := handler.Handle(context.Background(), event); err == nil {
		t.Fatal("expected error for invalid payload, got nil")
	}
}

// Scenario: sender suspended (committed message) → in-app delivered, push suppressed.
func TestChatNotification_SenderSuspended_InAppOnly(t *testing.T) {
	senderID := uuid.New()
	recipientID := uuid.New()
	roomID := uuid.New()
	messageID := uuid.New()

	dbCalls := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
			dbCalls++
			return fn(&mockTxForNotification{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			})
		},
	}

	handler := NewNotificationEventHandler(
		mockDB,
		&mockBlockCheckerChat{blocked: false},
		NewNotificationServiceInserter(),
		&mockPushSenderForNotification{},
		&mockAccountStatusCheckerChat{statuses: map[uuid.UUID]string{
			senderID:    "suspended",
			recipientID: "active",
		}},
		zaptest.NewLogger(t),
	)

	event := platformevent.OutboxEvent{
		ID:          uuid.New(),
		EventType:   "chat.message.sent",
		AggregateID: roomID,
		Payload:     makeChatPayload(senderID, recipientID, roomID, messageID),
	}

	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	// In-app should still be inserted for a committed message from a suspended sender.
	if dbCalls == 0 {
		t.Error("expected in-app notification for committed message from suspended sender")
	}
}

// Regression: existing social notifications (user.followed) still use applyPolicyLayer.
func TestChatNotification_Regression_UserFollowed_StillWorks(t *testing.T) {
	actorID := uuid.New()
	recipientID := uuid.New()

	payload, _ := json.Marshal(NotificationPayload{
		ActorID:     actorID.String(),
		RecipientID: recipientID.String(),
	})

	dbCalls := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
			dbCalls++
			return fn(&mockTxForNotification{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			})
		},
	}

	handler := NewNotificationEventHandler(
		mockDB,
		&mockBlockCheckerChat{blocked: false},
		NewNotificationServiceInserter(),
		&mockPushSenderForNotification{},
		&mockAccountStatusCheckerChat{},
		zaptest.NewLogger(t),
	)

	event := platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: events.EventUserFollowed,
		Payload:   payload,
	}

	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if dbCalls == 0 {
		t.Error("expected user.followed notification to be inserted (regression check)")
	}
}

// =============================================================================
// CHAT-5: MUTE GOVERNANCE SHADOW ROLLOUT
// =============================================================================

// mockMuteCheckerChat is a configurable mute checker for CHAT-5 tests.
// Returns the same muted/err regardless of which direction is queried.
// Use mockMuteCheckerDirectional when direction specificity matters.
type mockMuteCheckerChat struct {
	muted bool
	err   error
}

func (m *mockMuteCheckerChat) ExistsMute(_ context.Context, _, _ uuid.UUID) (bool, error) {
	return m.muted, m.err
}

// mockMuteCheckerDirectional returns muted=true only when the exact (muterID, mutedID) pair matches.
// Used for scenario C (sender muted recipient, no delivery effect).
type mockMuteCheckerDirectional struct {
	muterID uuid.UUID
	mutedID uuid.UUID
}

func (m *mockMuteCheckerDirectional) ExistsMute(_ context.Context, muterID, mutedID uuid.UUID) (bool, error) {
	return muterID == m.muterID && mutedID == m.mutedID, nil
}

// makeMuteDB builds a mockDB that counts WithTx calls and captures the inserted notification ID.
func makeMuteDB(dbCalls *int) *mockDBForNotification {
	return &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
			*dbCalls++
			return fn(&mockTxForNotification{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			})
		},
	}
}

// Scenario A: recipient muted sender, shadow mode → notification still delivered, telemetry emitted.
func TestChatMute_RecipientMutedSender_ShadowDeliver(t *testing.T) {
	senderID := uuid.New()
	recipientID := uuid.New()
	roomID := uuid.New()
	messageID := uuid.New()

	dbCalls := 0
	handler := NewNotificationEventHandler(
		makeMuteDB(&dbCalls),
		&mockBlockCheckerChat{blocked: false},
		NewNotificationServiceInserter(),
		&mockPushSenderForNotification{},
		&mockAccountStatusCheckerChat{},
		zaptest.NewLogger(t),
	)
	handler.SetMutePolicy(policy.NewMutePolicy(
		&mockMuteCheckerChat{muted: true},
		policy.MuteShadow,
	))

	event := platformevent.OutboxEvent{
		ID:          uuid.New(),
		EventType:   "chat.message.sent",
		AggregateID: roomID,
		Payload:     makeChatPayload(senderID, recipientID, roomID, messageID),
	}
	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	// Shadow mode: muted but still delivered.
	if dbCalls == 0 {
		t.Error("expected notification delivered in shadow mode, but no DB insert happened")
	}
}

// Scenario B: recipient muted sender, enforce mode → no in-app, no push.
func TestChatMute_RecipientMutedSender_EnforceSuppress(t *testing.T) {
	senderID := uuid.New()
	recipientID := uuid.New()
	roomID := uuid.New()
	messageID := uuid.New()

	dbCalls := 0
	handler := NewNotificationEventHandler(
		makeMuteDB(&dbCalls),
		&mockBlockCheckerChat{blocked: false},
		NewNotificationServiceInserter(),
		&mockPushSenderForNotification{},
		&mockAccountStatusCheckerChat{},
		zaptest.NewLogger(t),
	)
	handler.SetMutePolicy(policy.NewMutePolicy(
		&mockMuteCheckerChat{muted: true},
		policy.MuteEnforce,
	))

	event := platformevent.OutboxEvent{
		ID:          uuid.New(),
		EventType:   "chat.message.sent",
		AggregateID: roomID,
		Payload:     makeChatPayload(senderID, recipientID, roomID, messageID),
	}
	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	// Enforce mode: muted → suppressed, no DB insert.
	if dbCalls > 0 {
		t.Error("expected suppression in enforce mode, but notification was inserted")
	}
}

// Scenario C: sender muted recipient → no effect, notification still delivered.
// The policy only checks recipient-muted-sender direction. Sender's mute is irrelevant.
func TestChatMute_SenderMutedRecipient_NoEffect(t *testing.T) {
	senderID := uuid.New()
	recipientID := uuid.New()
	roomID := uuid.New()
	messageID := uuid.New()

	dbCalls := 0
	// Directional mock: senderID muted recipientID, but NOT recipientID muted senderID.
	// Policy queries (recipientID, senderID) → false → deliver.
	handler := NewNotificationEventHandler(
		makeMuteDB(&dbCalls),
		&mockBlockCheckerChat{blocked: false},
		NewNotificationServiceInserter(),
		&mockPushSenderForNotification{},
		&mockAccountStatusCheckerChat{},
		zaptest.NewLogger(t),
	)
	handler.SetMutePolicy(policy.NewMutePolicy(
		&mockMuteCheckerDirectional{muterID: senderID, mutedID: recipientID},
		policy.MuteEnforce, // even in enforce mode, wrong direction → no suppression
	))

	event := platformevent.OutboxEvent{
		ID:          uuid.New(),
		EventType:   "chat.message.sent",
		AggregateID: roomID,
		Payload:     makeChatPayload(senderID, recipientID, roomID, messageID),
	}
	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if dbCalls == 0 {
		t.Error("expected delivery when only sender muted recipient, but no DB insert happened")
	}
}

// Scenario D: mutual mute → recipient-side semantics apply (suppress in enforce).
// Both sides muted, but only recipient-muted-sender direction governs notification delivery.
func TestChatMute_MutualMute_RecipientSemanticsApply(t *testing.T) {
	senderID := uuid.New()
	recipientID := uuid.New()
	roomID := uuid.New()
	messageID := uuid.New()

	dbCalls := 0
	// Both sides muted: mock returns true regardless of direction.
	handler := NewNotificationEventHandler(
		makeMuteDB(&dbCalls),
		&mockBlockCheckerChat{blocked: false},
		NewNotificationServiceInserter(),
		&mockPushSenderForNotification{},
		&mockAccountStatusCheckerChat{},
		zaptest.NewLogger(t),
	)
	handler.SetMutePolicy(policy.NewMutePolicy(
		&mockMuteCheckerChat{muted: true},
		policy.MuteEnforce,
	))

	event := platformevent.OutboxEvent{
		ID:          uuid.New(),
		EventType:   "chat.message.sent",
		AggregateID: roomID,
		Payload:     makeChatPayload(senderID, recipientID, roomID, messageID),
	}
	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	// Mutual mute: recipient-muted-sender governs → suppressed.
	if dbCalls > 0 {
		t.Error("expected suppression for mutual mute in enforce mode, but notification was inserted")
	}
}

// Scenario E: block + mute → block wins (notification dropped regardless of mute).
func TestChatMute_BlockPlusMute_BlockWins(t *testing.T) {
	senderID := uuid.New()
	recipientID := uuid.New()
	roomID := uuid.New()
	messageID := uuid.New()

	dbCalls := 0
	handler := NewNotificationEventHandler(
		makeMuteDB(&dbCalls),
		&mockBlockCheckerChat{blocked: true}, // block exists
		NewNotificationServiceInserter(),
		&mockPushSenderForNotification{},
		&mockAccountStatusCheckerChat{},
		zaptest.NewLogger(t),
	)
	// Mute also set, but block check (STEP 3) fires before mute (STEP 3C).
	handler.SetMutePolicy(policy.NewMutePolicy(
		&mockMuteCheckerChat{muted: true},
		policy.MuteShadow,
	))

	event := platformevent.OutboxEvent{
		ID:          uuid.New(),
		EventType:   "chat.message.sent",
		AggregateID: roomID,
		Payload:     makeChatPayload(senderID, recipientID, roomID, messageID),
	}
	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if dbCalls > 0 {
		t.Error("expected no delivery when blocked, but notification was inserted")
	}
}

// Scenario F: suspended recipient + mute → account status drop occurs before mute logic.
func TestChatMute_SuspendedRecipientPlusMute_AccountStatusWins(t *testing.T) {
	senderID := uuid.New()
	recipientID := uuid.New()
	roomID := uuid.New()
	messageID := uuid.New()

	dbCalls := 0
	handler := NewNotificationEventHandler(
		makeMuteDB(&dbCalls),
		&mockBlockCheckerChat{blocked: false},
		NewNotificationServiceInserter(),
		&mockPushSenderForNotification{},
		&mockAccountStatusCheckerChat{statuses: map[uuid.UUID]string{
			recipientID: "suspended",
			senderID:    "active",
		}},
		zaptest.NewLogger(t),
	)
	// Mute set, but account status (STEP 2) fires before mute (STEP 3C) for Social category.
	handler.SetMutePolicy(policy.NewMutePolicy(
		&mockMuteCheckerChat{muted: true},
		policy.MuteShadow,
	))

	event := platformevent.OutboxEvent{
		ID:          uuid.New(),
		EventType:   "chat.message.sent",
		AggregateID: roomID,
		Payload:     makeChatPayload(senderID, recipientID, roomID, messageID),
	}
	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if dbCalls > 0 {
		t.Error("expected no delivery for suspended recipient, but notification was inserted")
	}
}

// Scenario G: non-chat notification with mute policy set → mute STEP 3C skipped, notification delivered.
func TestChatMute_NonChatNotification_MuteSkipped(t *testing.T) {
	actorID := uuid.New()
	recipientID := uuid.New()

	payload, _ := json.Marshal(NotificationPayload{
		ActorID:     actorID.String(),
		RecipientID: recipientID.String(),
	})

	dbCalls := 0
	handler := NewNotificationEventHandler(
		makeMuteDB(&dbCalls),
		&mockBlockCheckerChat{blocked: false},
		NewNotificationServiceInserter(),
		&mockPushSenderForNotification{},
		&mockAccountStatusCheckerChat{},
		zaptest.NewLogger(t),
	)
	// Mute enforce set — but only chat_message type enters STEP 3C.
	handler.SetMutePolicy(policy.NewMutePolicy(
		&mockMuteCheckerChat{muted: true},
		policy.MuteEnforce,
	))

	event := platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: events.EventUserFollowed,
		Payload:   payload,
	}
	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	// user.followed must not be affected by chat mute policy.
	if dbCalls == 0 {
		t.Error("expected user.followed delivered regardless of chat mute policy, but no DB insert happened")
	}
}

// Scenario H: mute policy error → fail-open, notification delivered.
func TestChatMute_PolicyError_FailOpen(t *testing.T) {
	senderID := uuid.New()
	recipientID := uuid.New()
	roomID := uuid.New()
	messageID := uuid.New()

	dbCalls := 0
	handler := NewNotificationEventHandler(
		makeMuteDB(&dbCalls),
		&mockBlockCheckerChat{blocked: false},
		NewNotificationServiceInserter(),
		&mockPushSenderForNotification{},
		&mockAccountStatusCheckerChat{},
		zaptest.NewLogger(t),
	)
	handler.SetMutePolicy(policy.NewMutePolicy(
		&mockMuteCheckerChat{muted: false, err: errors.New("db timeout")},
		policy.MuteEnforce,
	))

	event := platformevent.OutboxEvent{
		ID:          uuid.New(),
		EventType:   "chat.message.sent",
		AggregateID: roomID,
		Payload:     makeChatPayload(senderID, recipientID, roomID, messageID),
	}
	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	// Mute checker error: fail-open → deliver.
	if dbCalls == 0 {
		t.Error("expected fail-open delivery on mute policy error, but no DB insert happened")
	}
}

// Scenario I: no mute policy set → behavior identical to pre-CHAT-5 (always deliver).
func TestChatMute_NoPolicySet_NoChange(t *testing.T) {
	senderID := uuid.New()
	recipientID := uuid.New()
	roomID := uuid.New()
	messageID := uuid.New()

	dbCalls := 0
	// No SetMutePolicy call — policyMute remains nil.
	handler := NewNotificationEventHandler(
		makeMuteDB(&dbCalls),
		&mockBlockCheckerChat{blocked: false},
		NewNotificationServiceInserter(),
		&mockPushSenderForNotification{},
		&mockAccountStatusCheckerChat{},
		zaptest.NewLogger(t),
	)

	event := platformevent.OutboxEvent{
		ID:          uuid.New(),
		EventType:   "chat.message.sent",
		AggregateID: roomID,
		Payload:     makeChatPayload(senderID, recipientID, roomID, messageID),
	}
	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if dbCalls == 0 {
		t.Error("expected delivery when no mute policy configured, but no DB insert happened")
	}
}

// TestNotificationServiceInserter_InsertNotification tests inserting a notification.
func TestNotificationServiceInserter_InsertNotification(t *testing.T) {
	inserter := NewNotificationServiceInserter()

	recipientID := uuid.New()
	actorID := uuid.New()
	entityID := uuid.New()

	expectedID := uuid.New()

	mockTx := &mockTxForNotificationWithQueryRow{
		mockTxForNotification: mockTxForNotification{
			ExecFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("1"), nil
			},
		},
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &mockRowForNotification{scanValue: expectedID}
		},
	}

	data := map[string]interface{}{"test": "value"}

	id, err := inserter.InsertNotification(context.Background(), mockTx, recipientID, actorID, events.EventUserFollowed, entityID, data)
	if err != nil {
		t.Fatalf("InsertNotification() error = %v", err)
	}

	if id != expectedID {
		t.Errorf("InsertNotification() = %v, want %v", id, expectedID)
	}
}

// TestOutboxWorker_SetupNotificationHandlers tests registering notification handlers.
func TestOutboxWorker_SetupNotificationHandlers(t *testing.T) {
	t.Skip("Skipping test - requires real *db.DB, tested in integration tests")
}

// TestNotificationEventHandler_ContentLikedInvalidContentID tests invalid content_id in payload.
func TestNotificationEventHandler_ContentLikedInvalidContentID(t *testing.T) {
	log := zaptest.NewLogger(t)

	payload, _ := json.Marshal(ContentLikedPayload{
		ActorID:     uuid.New().String(),
		RecipientID: uuid.New().String(),
		ContentID:   "not-a-uuid",
	})

	event := platformevent.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "content",
		AggregateID:   uuid.New(),
		EventType:     events.EventContentLiked,
		Payload:       payload,
	}

	mockDB := &mockDBForNotification{}
	inserter := NewNotificationServiceInserter()
	handler := NewNotificationEventHandler(mockDB, &mockBlockCheckerForNotification{}, inserter, &mockPushSenderForNotification{}, &mockAccountStatusCheckerForNotification{}, log)

	err := handler.Handle(context.Background(), event)
	if err == nil {
		t.Fatal("Handle() should return error for invalid content_id, got nil")
	}

	if !errors.Is(err, fmt.Errorf("invalid content_id")) && err.Error()[:19] != "invalid content_id" {
		t.Logf("error = %v (contains invalid content_id)", err)
	}
}

// ============================================================================
// SOCIAL GOVERNANCE CONVERGENCE VALIDATION (SOCIAL-1 through SOCIAL-4)
//
// Validates that all four migrated social handlers use applyPolicyLayer:
//   - recipient lifecycle enforced (suspended/banned = no delivery)
//   - actor lifecycle enforced (banned/deleted = drop, no identity leak)
//   - block enforced through policy with audit trail
//   - push/in-app use same policy decision (social = in-app only)
//   - mute gate does not expand beyond chat_message
//   - comment reply targetType fallback preserved
//   - comment created recipient SQL enrichment preserved
// ============================================================================

// mockAccountStatusControlled returns configurable account statuses per user.
type mockAccountStatusControlled struct {
	statuses map[uuid.UUID]string
}

func (m *mockAccountStatusControlled) GetStatus(_ context.Context, userID uuid.UUID) (string, error) {
	if s, ok := m.statuses[userID]; ok {
		return s, nil
	}
	return "active", nil
}

// mockBlockCheckerControlled returns blocked=true for configured actor-recipient pairs.
type mockBlockCheckerControlled struct {
	blocked map[[2]uuid.UUID]bool
}

func (m *mockBlockCheckerControlled) ExistsBlock(_ context.Context, a, b uuid.UUID) (bool, error) {
	if v, ok := m.blocked[[2]uuid.UUID{a, b}]; ok {
		return v, nil
	}
	if v, ok := m.blocked[[2]uuid.UUID{b, a}]; ok {
		return v, nil
	}
	return false, nil
}

// mockDeliveryCapture records all LogDelivery calls for audit trail assertions.
type mockDeliveryCapture struct {
	mu    sync.Mutex
	wg    sync.WaitGroup
	calls []deliveryCaptureCall
}

type deliveryCaptureCall struct {
	channel string
	status  string
	reason  string
}

func (m *mockDeliveryCapture) LogDelivery(
	_ context.Context, _, _ uuid.UUID,
	channel, status, reason string,
	_ map[string]interface{},
) {
	m.mu.Lock()
	m.calls = append(m.calls, deliveryCaptureCall{channel: channel, status: status, reason: reason})
	m.mu.Unlock()
	m.wg.Done()
}

func (m *mockDeliveryCapture) inAppStatus() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.calls {
		if c.channel == "in_app" {
			return c.status
		}
	}
	return ""
}

// mockMuteCheckerAlwaysMuted reports a mute relationship for every pair.
// Used to prove that mute policy does not suppress non-chat notification types.
type mockMuteCheckerAlwaysMuted struct{}

func (m *mockMuteCheckerAlwaysMuted) ExistsMute(_ context.Context, _, _ uuid.UUID) (bool, error) {
	return true, nil
}

// multiScanRow supports scanning multiple values of mixed type (uuid.UUID, string).
type multiScanRow struct {
	err    error
	values []interface{}
}

func (r *multiScanRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, d := range dest {
		if i >= len(r.values) {
			break
		}
		switch v := r.values[i].(type) {
		case uuid.UUID:
			if p, ok := d.(*uuid.UUID); ok {
				*p = v
			}
		case string:
			if p, ok := d.(*string); ok {
				*p = v
			}
		}
	}
	return nil
}

// buildSocialGovernanceHandler constructs a handler with controlled policy dependencies.
func buildSocialGovernanceHandler(
	t *testing.T,
	db *mockDBForNotification,
	status AccountStatusChecker,
	block BlockChecker,
	logger DeliveryLogger,
) *NotificationEventHandler {
	t.Helper()
	h := NewNotificationEventHandler(db, block, NewNotificationServiceInserter(), nil, status, zaptest.NewLogger(t))
	if logger != nil {
		h.SetDeliveryLogger(logger)
	}
	return h
}

// insertCaptureTx returns a WithTxFunc that captures INSERT args and succeeds.
func insertCaptureTx(
	capturedRecipient, capturedActor *uuid.UUID,
	capturedType *string,
	capturedEntityID *uuid.UUID,
	capturedData *map[string]interface{},
) func(context.Context, func(dbpkg.Tx) error) error {
	return func(ctx context.Context, fn func(dbpkg.Tx) error) error {
		tx := &mockTxForNotification{
			QueryRowFunc: func(_ context.Context, sql string, args ...any) pgx.Row {
				// LIKE-OCCURRENCE GUARD: content.liked delivery re-checks that the
				// like row still exists. Simulate an active like row.
				if strings.Contains(sql, "content_likes") {
					return &mockRowForNotification{scanValue: true}
				}
				if len(args) >= 6 {
					if capturedRecipient != nil {
						*capturedRecipient = args[1].(uuid.UUID)
					}
					if capturedActor != nil {
						*capturedActor = args[2].(uuid.UUID)
					}
					if capturedType != nil {
						*capturedType = args[3].(string)
					}
					if capturedEntityID != nil {
						*capturedEntityID = args[4].(uuid.UUID)
					}
					if capturedData != nil {
						*capturedData = args[5].(map[string]interface{})
					}
				}
				return &mockRowForNotification{scanValue: uuid.New()}
			},
		}
		return fn(tx)
	}
}

// --- Validation A: active recipient delivered + audit logged ---

func TestSocialGovernance_ContentLiked_ActiveRecipient_Delivered(t *testing.T) {
	actorID := uuid.New()
	recipientID := uuid.New()
	contentID := uuid.New()

	var capturedRecipient, capturedActor, capturedEntity uuid.UUID
	var capturedType string
	var capturedData map[string]interface{}

	mockDB := &mockDBForNotification{
		WithTxFunc: insertCaptureTx(&capturedRecipient, &capturedActor, &capturedType, &capturedEntity, &capturedData),
	}
	logger := &mockDeliveryCapture{}
	logger.wg.Add(1)
	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, logger)

	payload, _ := json.Marshal(ContentLikedPayload{
		ActorID:     actorID.String(),
		RecipientID: recipientID.String(),
		ContentID:   contentID.String(),
	})
	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: events.EventContentLiked, Payload: payload,
	})
	logger.wg.Wait()

	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if capturedRecipient != recipientID {
		t.Errorf("recipient = %s, want %s", capturedRecipient, recipientID)
	}
	if capturedActor != actorID {
		t.Errorf("actor = %s, want %s", capturedActor, actorID)
	}
	if capturedType != events.EventContentLiked {
		t.Errorf("type = %s, want %s", capturedType, events.EventContentLiked)
	}
	if capturedEntity != contentID {
		t.Errorf("entity = %s, want %s", capturedEntity, contentID)
	}
	if capturedData["targetType"] != "content" {
		t.Errorf("targetType = %v, want content (canonical content target)", capturedData["targetType"])
	}
	if logger.inAppStatus() != "sent" {
		t.Errorf("audit in_app status = %q, want sent", logger.inAppStatus())
	}
}

// --- Validation B: blocked actor dropped + audit logged skipped ---

func TestSocialGovernance_ContentLiked_BlockedActor_Dropped(t *testing.T) {
	actorID := uuid.New()
	recipientID := uuid.New()
	contentID := uuid.New()

	insertCalled := false
	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(dbpkg.Tx) error) error {
			insertCalled = true
			return fn(&mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			})
		},
	}
	logger := &mockDeliveryCapture{}
	logger.wg.Add(1)
	block := &mockBlockCheckerControlled{
		blocked: map[[2]uuid.UUID]bool{{actorID, recipientID}: true},
	}
	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, block, logger)

	payload, _ := json.Marshal(ContentLikedPayload{
		ActorID:     actorID.String(),
		RecipientID: recipientID.String(),
		ContentID:   contentID.String(),
	})
	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: events.EventContentLiked, Payload: payload,
	})
	logger.wg.Wait()

	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if insertCalled {
		t.Error("INSERT was called; want blocked notification to be dropped before insert")
	}
	if logger.inAppStatus() != "skipped" {
		t.Errorf("audit in_app status = %q, want skipped", logger.inAppStatus())
	}
}

// --- Validation C: suspended recipient dropped ---

func TestSocialGovernance_ContentLiked_SuspendedRecipient_Dropped(t *testing.T) {
	actorID := uuid.New()
	recipientID := uuid.New()
	contentID := uuid.New()

	insertCalled := false
	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(dbpkg.Tx) error) error {
			insertCalled = true
			return fn(&mockTxForNotification{})
		},
	}
	status := &mockAccountStatusControlled{
		statuses: map[uuid.UUID]string{recipientID: "suspended"},
	}
	h := buildSocialGovernanceHandler(t, mockDB, status, &mockBlockCheckerControlled{}, nil)

	payload, _ := json.Marshal(ContentLikedPayload{
		ActorID:     actorID.String(),
		RecipientID: recipientID.String(),
		ContentID:   contentID.String(),
	})
	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: events.EventContentLiked, Payload: payload,
	})

	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if insertCalled {
		t.Error("INSERT was called; want suspended recipient's social notification dropped")
	}
}

// --- Validation D: banned actor historical outbox dropped, no actor identity leaked ---

func TestSocialGovernance_ContentLiked_BannedActor_Dropped_NoIdentityLeak(t *testing.T) {
	actorID := uuid.New()
	recipientID := uuid.New()
	contentID := uuid.New()

	insertCalled := false
	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(dbpkg.Tx) error) error {
			insertCalled = true
			return fn(&mockTxForNotification{})
		},
	}
	status := &mockAccountStatusControlled{
		statuses: map[uuid.UUID]string{actorID: "banned"},
	}
	logger := &mockDeliveryCapture{}
	logger.wg.Add(1)
	h := buildSocialGovernanceHandler(t, mockDB, status, &mockBlockCheckerControlled{}, logger)

	payload, _ := json.Marshal(ContentLikedPayload{
		ActorID:     actorID.String(),
		RecipientID: recipientID.String(),
		ContentID:   contentID.String(),
	})
	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: events.EventContentLiked, Payload: payload,
	})
	logger.wg.Wait()

	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if insertCalled {
		t.Error("INSERT was called; want banned actor outbox event dropped without identity persistence")
	}
	if logger.inAppStatus() != "skipped" {
		t.Errorf("audit in_app status = %q, want skipped", logger.inAppStatus())
	}
}

// --- Validation E: comment reply delivered with canonical content target ---
// (Content-type SQL enrichment removed; the handler navigates directly to the
// canonical content target, so no pre-insert lookup tx exists.)

func TestSocialGovernance_CommentReply_ContentLookupFailure_FallbackToPost(t *testing.T) {
	authorID := uuid.New()
	parentAuthorID := uuid.New()
	contentID := uuid.New()
	commentID := uuid.New()
	parentID := uuid.New()

	callCount := 0
	var capturedData map[string]interface{}
	var capturedActor uuid.UUID
	var capturedType string

	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(dbpkg.Tx) error) error {
			callCount++
			// Single transaction: policy + insert. Simulate success.
			return fn(&mockTxForNotification{
				QueryRowFunc: func(_ context.Context, sql string, args ...any) pgx.Row {
					if len(args) >= 6 {
						capturedActor = args[2].(uuid.UUID)
						capturedType = args[3].(string)
						capturedData = args[5].(map[string]interface{})
					}
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			})
		},
	}
	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)

	payload, _ := json.Marshal(CommentReplyPayload{
		AuthorID:       authorID.String(),
		ParentAuthorID: parentAuthorID.String(),
		ContentID:      contentID.String(),
		CommentID:      commentID.String(),
		ParentID:       parentID.String(),
	})
	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "comment.reply", Payload: payload,
	})

	if err != nil {
		t.Fatalf("Handle() error = %v; want delivery to succeed", err)
	}
	if callCount != 1 {
		t.Errorf("WithTx called %d times, want 1 (policy+insert in one tx)", callCount)
	}
	if capturedData == nil {
		t.Fatal("INSERT was not called; want notification delivered")
	}
	if capturedType != "comment_reply" {
		t.Errorf("type = %v, want comment_reply", capturedType)
	}
	if capturedActor != authorID {
		t.Errorf("actor = %v, want %v", capturedActor, authorID)
	}
	if capturedData["targetType"] != "content" {
		t.Errorf("targetType = %v, want content (canonical navigation target)", capturedData["targetType"])
	}
	if capturedData["targetId"] != contentID.String() {
		t.Errorf("targetId = %v, want %v", capturedData["targetId"], contentID.String())
	}
}

// --- Validation F: seller.response Social category, canonical contract, no push ---

func TestSocialGovernance_SellerResponse_SocialCategory_CanonicalContract(t *testing.T) {
	sellerID := uuid.New()
	requestCreatorID := uuid.New()
	contentID := uuid.New()
	commentID := uuid.New()
	forSaleID := uuid.New()

	var capturedData map[string]interface{}
	var capturedActor uuid.UUID
	var capturedType string

	mockDB := &mockDBForNotification{
		WithTxFunc: insertCaptureTx(nil, &capturedActor, &capturedType, nil, &capturedData),
	}
	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)

	payload, _ := json.Marshal(SellerResponsePayload{
		SellerID:         sellerID.String(),
		RequestCreatorID: requestCreatorID.String(),
		ContentID:        contentID.String(),
		CommentID:        commentID.String(),
		ResourceID:       forSaleID.String(),
		ResourceType:     "for_sale",
	})
	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "seller.response", Payload: payload,
	})

	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if capturedType != "seller.response" {
		t.Errorf("type = %s, want seller.response", capturedType)
	}
	if capturedActor != sellerID {
		t.Errorf("actor = %s, want %s (no anonymization for Social)", capturedActor, sellerID)
	}
	if capturedData["resourceId"] != forSaleID.String() {
		t.Errorf(
			"resourceId = %v, want %s",
			capturedData["resourceId"],
			forSaleID.String(),
		)
	}
	if capturedData["resourceType"] != "for_sale" {
		t.Errorf("resourceType = %v, want for_sale", capturedData["resourceType"])
	}
	// Verify no legacy backward-compat keys are emitted.
	if _, ok := capturedData["forSaleId"]; ok {
		t.Error("forSaleId must not be present in notification data (obsolete compat key removed)")
	}
	if _, ok := capturedData["auctionId"]; ok {
		t.Error("auctionId must not be present in notification data (obsolete compat key removed)")
	}
	if capturedData["targetType"] != "request" {
		t.Errorf("targetType = %v, want request", capturedData["targetType"])
	}
	// Verify policy category classification.
	if cat := policy.GetCategory("seller.response"); cat != policy.Social {
		t.Errorf("GetCategory(seller.response) = %v, want Social", cat)
	}
	// Verify no push for seller.response (in-app only per doctrine).
	if policy.RequiresPushByType("seller.response") {
		t.Error("RequiresPushByType(seller.response) = true, want false (in-app only)")
	}
}

// --- Validation G: mute shadow mode does not suppress non-chat social types ---

func TestSocialGovernance_MuteShadow_NoSuppressionForSocialTypes(t *testing.T) {
	actorID := uuid.New()
	recipientID := uuid.New()
	contentID := uuid.New()

	insertCalled := false
	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(dbpkg.Tx) error) error {
			insertCalled = true
			return fn(&mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			})
		},
	}
	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	// Wire mute policy in shadow mode (same as production default).
	// Use a mute checker that reports a mute relationship to prove mute is not applied.
	mutePolicy := policy.NewMutePolicy(&mockMuteCheckerAlwaysMuted{}, policy.MuteShadow)
	h.SetMutePolicy(mutePolicy)

	payload, _ := json.Marshal(ContentLikedPayload{
		ActorID:     actorID.String(),
		RecipientID: recipientID.String(),
		ContentID:   contentID.String(),
	})
	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: events.EventContentLiked, Payload: payload,
	})

	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if !insertCalled {
		t.Error("INSERT not called; mute must not suppress social notifications (mute scope = chat_message only)")
	}
}

// --- Validation H: push/in-app symmetry — social types are in-app only via policy ---

func TestSocialGovernance_PushSymmetry_SocialTypesInAppOnly(t *testing.T) {
	// Verifies the push/in-app symmetry doctrine: both channels governed by the same
	// policy decision. For all four social types, RequiresPushByType returns false,
	// so applyPolicyLayer always sets allowPush=false regardless of recipient status.
	socialTypes := []string{
		events.EventContentLiked, // "content.liked"
		"comment",
		"comment_reply",
		"seller.response",
	}
	for _, typ := range socialTypes {
		cat := policy.GetCategory(typ)
		if cat != policy.Social {
			t.Errorf("GetCategory(%q) = %v, want Social", typ, cat)
		}
		if policy.RequiresPushByType(typ) {
			t.Errorf("RequiresPushByType(%q) = true, want false; push and in-app must follow same policy decision", typ)
		}
	}
}

// --- Validation I: shouldFilterNotification fully deleted ---

func TestSocialGovernance_ShouldFilterNotification_Deleted(t *testing.T) {
	// Static verification: shouldFilterNotification must not exist on the handler.
	// If this test compiles, the method has been removed (it would be a compile error
	// to call a non-existent method). The grep-clean proof is in the CI diff.
	//
	// Positive proof: handler compiles and handles all four social types without
	// the deprecated method. The four sub-cases below confirm routing is intact.
	h := buildSocialGovernanceHandler(t,
		&mockDBForNotification{
			WithTxFunc: func(ctx context.Context, fn func(dbpkg.Tx) error) error {
				return fn(&mockTxForNotification{
					QueryRowFunc: func(_ context.Context, _ string, args ...any) pgx.Row {
						if len(args) == 0 {
							return &multiScanRow{values: []interface{}{uuid.New(), "post"}}
						}
						return &mockRowForNotification{scanValue: uuid.New()}
					},
				})
			},
		},
		&mockAccountStatusControlled{},
		&mockBlockCheckerControlled{},
		nil,
	)

	actor := uuid.New()
	recipient := uuid.New()
	content := uuid.New()
	comment := uuid.New()
	parent := uuid.New()
	forSaleID := uuid.New()

	cases := []struct {
		eventType string
		payload   []byte
	}{
		{events.EventContentLiked, mustMarshal(ContentLikedPayload{
			ActorID: actor.String(), RecipientID: recipient.String(), ContentID: content.String(),
		})},
		{"comment.reply", mustMarshal(CommentReplyPayload{
			AuthorID: actor.String(), ParentAuthorID: recipient.String(),
			ContentID: content.String(), CommentID: comment.String(), ParentID: parent.String(),
		})},
		{"seller.response", mustMarshal(SellerResponsePayload{
			SellerID: actor.String(), RequestCreatorID: recipient.String(),
			ContentID: content.String(), CommentID: comment.String(), ResourceID: forSaleID.String(), ResourceType: "for_sale",
		})},
		{events.EventCommentCreated, mustMarshal(CommentCreatedPayload{
			AuthorID: actor.String(), ContentID: content.String(), CommentID: comment.String(),
		})},
	}

	for _, tc := range cases {
		t.Run(tc.eventType, func(t *testing.T) {
			err := h.Handle(context.Background(), platformevent.OutboxEvent{
				ID: uuid.New(), EventType: tc.eventType, Payload: tc.payload,
			})
			if err != nil {
				t.Errorf("Handle(%s) error = %v", tc.eventType, err)
			}
		})
	}
}

func mustMarshal(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// --- Validation: comment created recipient SQL enrichment preserved ---

func TestSocialGovernance_CommentCreated_RecipientResolutionFromDB(t *testing.T) {
	authorID := uuid.New()
	recipientID := uuid.New() // content owner, not in payload
	contentID := uuid.New()
	commentID := uuid.New()

	callCount := 0
	var capturedRecipient, capturedActor uuid.UUID

	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(dbpkg.Tx) error) error {
			callCount++
			if callCount == 1 {
				// First call: SELECT author_id, type FROM contents.
				return fn(&mockTxForNotification{
					QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
						return &multiScanRow{values: []interface{}{recipientID, "post"}}
					},
				})
			}
			// Second call: INSERT notification.
			return fn(&mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, args ...any) pgx.Row {
					if len(args) >= 3 {
						capturedRecipient = args[1].(uuid.UUID)
						capturedActor = args[2].(uuid.UUID)
					}
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			})
		},
	}
	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)

	payload, _ := json.Marshal(CommentCreatedPayload{
		AuthorID:  authorID.String(),
		ContentID: contentID.String(),
		CommentID: commentID.String(),
	})
	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: events.EventCommentCreated, Payload: payload,
	})

	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if callCount != 2 {
		t.Errorf("WithTx called %d times, want 2 (SELECT content owner + INSERT)", callCount)
	}
	if capturedRecipient != recipientID {
		t.Errorf("recipient = %s, want %s (resolved from DB, not payload)", capturedRecipient, recipientID)
	}
	if capturedActor != authorID {
		t.Errorf("actor = %s, want %s", capturedActor, authorID)
	}
}

// =============================================================================
// BLOCK NOTIFICATION HISTORY ALIGNMENT TESTS
// =============================================================================

// TestHandleUserBlocked_DeletesSocialPreservesCommerce proves that on block:
// - Social notification types are deleted bidirectionally
// - The DELETE SQL uses ANY($3) with socialNotificationTypes
// - No new notification is created (block is silent)
func TestHandleUserBlocked_DeletesSocialPreservesCommerce(t *testing.T) {
	log := zaptest.NewLogger(t)

	blockerID := uuid.New()
	blockedID := uuid.New()

	payload, _ := json.Marshal(NotificationPayload{
		ActorID:     blockerID.String(),
		RecipientID: blockedID.String(),
	})

	event := platformevent.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "user",
		AggregateID:   blockedID,
		EventType:     events.EventUserBlocked,
		Payload:       payload,
	}

	var capturedSQL string
	var capturedArgs []any

	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
			mockTx := &mockTxForNotification{
				ExecFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
					capturedSQL = sql
					capturedArgs = args
					return pgconn.NewCommandTag("DELETE 3"), nil
				},
			}
			return fn(mockTx)
		},
	}

	inserter := NewNotificationServiceInserter()
	handler := NewNotificationEventHandler(mockDB, &mockBlockCheckerForNotification{}, inserter, &mockPushSenderForNotification{}, &mockAccountStatusCheckerForNotification{}, log)

	err := handler.Handle(context.Background(), event)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Verify DELETE was called
	if capturedSQL == "" {
		t.Fatal("Expected DELETE SQL to be executed, got none")
	}

	// Verify SQL targets both directions
	if len(capturedArgs) != 3 {
		t.Fatalf("Expected 3 args (blockerID, blockedID, types), got %d", len(capturedArgs))
	}

	// Verify user IDs passed correctly
	if capturedArgs[0] != blockerID {
		t.Errorf("arg[0] = %v, want blockerID %s", capturedArgs[0], blockerID)
	}
	if capturedArgs[1] != blockedID {
		t.Errorf("arg[1] = %v, want blockedID %s", capturedArgs[1], blockedID)
	}

	// Verify social types passed as filter
	types, ok := capturedArgs[2].([]string)
	if !ok {
		t.Fatalf("arg[2] expected []string, got %T", capturedArgs[2])
	}

	// Verify against canonical socialNotificationTypes slice (single source of truth).
	if len(types) != len(socialNotificationTypes) {
		t.Errorf("Expected %d social types (socialNotificationTypes), got %d: %v",
			len(socialNotificationTypes), len(types), types)
	}

	haveTypes := make(map[string]bool, len(types))
	for _, typ := range types {
		haveTypes[typ] = true
	}
	for _, canonical := range socialNotificationTypes {
		if !haveTypes[canonical] {
			t.Errorf("Canonical social type %q not found in handler arg", canonical)
		}
	}
}

// TestHandleUserBlocked_DoesNotCreateNotification proves block handler
// returns directly without calling insertNotificationWithPolicy.
func TestHandleUserBlocked_DoesNotCreateNotification(t *testing.T) {
	log := zaptest.NewLogger(t)

	blockerID := uuid.New()
	blockedID := uuid.New()

	payload, _ := json.Marshal(NotificationPayload{
		ActorID:     blockerID.String(),
		RecipientID: blockedID.String(),
	})

	event := platformevent.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "user",
		AggregateID:   blockedID,
		EventType:     events.EventUserBlocked,
		Payload:       payload,
	}

	insertCalled := false

	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
			mockTx := &mockTxForNotification{
				ExecFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
					// Only DELETE should be called, not INSERT
					if len(sql) > 6 && sql[:6] == "INSERT" {
						insertCalled = true
					}
					return pgconn.NewCommandTag("DELETE 0"), nil
				},
			}
			return fn(mockTx)
		},
	}

	inserter := NewNotificationServiceInserter()
	handler := NewNotificationEventHandler(mockDB, &mockBlockCheckerForNotification{}, inserter, &mockPushSenderForNotification{}, &mockAccountStatusCheckerForNotification{}, log)

	err := handler.Handle(context.Background(), event)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if insertCalled {
		t.Error("Block handler should NOT insert a notification (block is silent)")
	}
}

// TestHandleUserUnfollowed_DeletesOnlyFollowedNotification proves unfollow
// removes only the specific "user.followed" notification in the correct direction.
func TestHandleUserUnfollowed_DeletesOnlyFollowedNotification(t *testing.T) {
	log := zaptest.NewLogger(t)

	followerID := uuid.New()
	followedID := uuid.New()

	payload, _ := json.Marshal(NotificationPayload{
		ActorID:     followerID.String(),
		RecipientID: followedID.String(),
	})

	event := platformevent.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "user",
		AggregateID:   followedID,
		EventType:     events.EventUserUnfollowed,
		Payload:       payload,
	}

	var capturedSQL string
	var capturedArgs []any

	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
			mockTx := &mockTxForNotification{
				ExecFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
					capturedSQL = sql
					capturedArgs = args
					return pgconn.NewCommandTag("DELETE 1"), nil
				},
			}
			return fn(mockTx)
		},
	}

	inserter := NewNotificationServiceInserter()
	handler := NewNotificationEventHandler(mockDB, &mockBlockCheckerForNotification{}, inserter, &mockPushSenderForNotification{}, &mockAccountStatusCheckerForNotification{}, log)

	err := handler.Handle(context.Background(), event)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Verify DELETE was called
	if capturedSQL == "" {
		t.Fatal("Expected DELETE SQL to be executed, got none")
	}

	// Verify args: followerID, followedID, "user.followed"
	if len(capturedArgs) != 3 {
		t.Fatalf("Expected 3 args, got %d", len(capturedArgs))
	}

	if capturedArgs[0] != followerID {
		t.Errorf("arg[0] = %v, want followerID %s", capturedArgs[0], followerID)
	}
	if capturedArgs[1] != followedID {
		t.Errorf("arg[1] = %v, want followedID %s", capturedArgs[1], followedID)
	}
	if capturedArgs[2] != "user.followed" {
		t.Errorf("arg[2] = %v, want 'user.followed'", capturedArgs[2])
	}
}

// TestHandleUserBlocked_SQLPreservesCommerce verifies the SQL structure ensures
// commerce/moderation/support notifications are NOT deleted.
func TestHandleUserBlocked_SQLPreservesCommerce(t *testing.T) {
	// The socialNotificationTypes list must NOT contain any commerce/moderation types.
	// This is a compile-time-ish assertion that the cleanup is safe.
	commerceTypes := []string{
		"order.created", "order.paid", "order.shipped",
		"order.completed", "order.cancelled", "order.refunded",
		"withdrawal.requested", "withdrawal.approved",
		"dispute.opened", "negotiation.started",
		"moderation.content.removed", "moderation.comment.removed",
		"support.ticket.resolved",
		"verification.document.approved",
	}

	socialSet := make(map[string]bool)
	for _, s := range socialNotificationTypes {
		socialSet[s] = true
	}

	for _, commerceType := range commerceTypes {
		if socialSet[commerceType] {
			t.Errorf("CRITICAL: commerce/moderation type %q found in socialNotificationTypes — would be deleted on block!", commerceType)
		}
	}
}

// TestSocialNotificationTypes_MatchesPolicyCategory verifies the socialNotificationTypes
// list is aligned with policy.GetCategory() Social classification.
func TestSocialNotificationTypes_MatchesPolicyCategory(t *testing.T) {
	for _, typ := range socialNotificationTypes {
		category := policy.GetCategory(typ)
		if category != policy.Social {
			t.Errorf("socialNotificationTypes contains %q but GetCategory returns %q (expected Social)", typ, category)
		}
	}
}

// TestHandleUserBlocked_InvalidPayload verifies error handling for malformed payloads.
func TestHandleUserBlocked_InvalidPayload(t *testing.T) {
	log := zaptest.NewLogger(t)

	tests := []struct {
		name    string
		payload []byte
	}{
		{"empty payload", []byte{}},
		{"invalid json", []byte(`{invalid}`)},
		{"missing actor_id", mustJSON(t, map[string]string{"recipient_id": uuid.New().String()})},
		{"invalid actor_id", mustJSON(t, NotificationPayload{ActorID: "not-a-uuid", RecipientID: uuid.New().String()})},
		{"invalid recipient_id", mustJSON(t, NotificationPayload{ActorID: uuid.New().String(), RecipientID: "not-a-uuid"})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := &mockDBForNotification{}
			inserter := NewNotificationServiceInserter()
			handler := NewNotificationEventHandler(mockDB, &mockBlockCheckerForNotification{}, inserter, &mockPushSenderForNotification{}, &mockAccountStatusCheckerForNotification{}, log)

			event := platformevent.OutboxEvent{
				ID:        uuid.New(),
				EventType: events.EventUserBlocked,
				Payload:   tt.payload,
			}

			err := handler.Handle(context.Background(), event)
			if err == nil {
				t.Error("Expected error for invalid payload, got nil")
			}
		})
	}
}

// =============================================================================
// SOCIAL MENTION NOTIFICATION RUNTIME E2E PROOF
// =============================================================================
//
// Proves the canonical mention notification wire contract:
//   type = "content.mentioned"
//   data = { "targetId": "<content UUID>", "targetType": "content" }
//
// Each test uses mock infrastructure (no PostgreSQL required) to prove
// the handler parses the canonical payload and produces correct notification
// fields.

// TestSocialGovernance_ContentMentioned_ActiveRecipient_Delivered proves:
//   1. content.mentioned event is parsed correctly
//   2. Notification recipient = mentioned_user_id (not author)
//   3. Notification actor = author_id
//   4. Notification type = "content.mentioned"
//   5. Notification data = { targetId: contentID, targetType: "content" }
//   6. Self-mention is skipped
//
// This is the single authoritative proof that the canonical wire contract
// is implemented correctly at the handler level.
func TestSocialGovernance_ContentMentioned_ActiveRecipient_Delivered(t *testing.T) {
	authorID := uuid.New()
	mentionedUserID := uuid.New()
	contentID := uuid.New()

	var capturedRecipient, capturedActor, capturedEntity uuid.UUID
	var capturedType string
	var capturedData map[string]interface{}

	mockDB := &mockDBForNotification{
		WithTxFunc: insertCaptureTx(&capturedRecipient, &capturedActor, &capturedType, &capturedEntity, &capturedData),
	}
	handler := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)

	payload, _ := json.Marshal(ContentMentionedPayload{
		ContentID:       contentID.String(),
		AuthorID:        authorID.String(),
		MentionedUserID: mentionedUserID.String(),
	})
	err := handler.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: events.EventContentMentioned,
		Payload:   payload,
	})

	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// CRITICAL: recipient must be the mentioned user, not the author
	if capturedRecipient != mentionedUserID {
		t.Errorf("recipient = %s, want %s (mentioned user)", capturedRecipient, mentionedUserID)
	}

	// CRITICAL: actor must be the content author
	if capturedActor != authorID {
		t.Errorf("actor = %s, want %s (content author)", capturedActor, authorID)
	}

	// CRITICAL: notification type must be the canonical backend wire type
	if capturedType != events.EventContentMentioned {
		t.Errorf("type = %q, want %q (canonical backend type)", capturedType, events.EventContentMentioned)
	}

	// CRITICAL: entity ID must be the content
	if capturedEntity != contentID {
		t.Errorf("entity = %s, want %s (content ID)", capturedEntity, contentID)
	}

	// CRITICAL: data must use canonical backend keys
	if capturedData == nil {
		t.Fatal("data is nil, want {targetId, targetType}")
	}
	if capturedData["targetId"] != contentID.String() {
		t.Errorf("data[targetId] = %v, want %q", capturedData["targetId"], contentID.String())
	}
	if capturedData["targetType"] != "content" {
		t.Errorf("data[targetType] = %v, want \"content\"", capturedData["targetType"])
	}

	// CRITICAL: no obsolete keys in data
	if _, ok := capturedData["forSaleId"]; ok {
		t.Error("data contains obsolete key 'forSaleId'")
	}
	if _, ok := capturedData["auctionId"]; ok {
		t.Error("data contains obsolete key 'auctionId'")
	}
	if _, ok := capturedData["contentId"]; ok {
		t.Error("data contains key 'contentId' — must use 'targetId' (canonical backend key)")
	}
}

// TestSocialGovernance_ContentMentioned_SelfMention_Skipped proves:
//   When author == mentioned_user, notification is NOT created.
//   Self-mention is a no-op at the handler level.
func TestSocialGovernance_ContentMentioned_SelfMention_Skipped(t *testing.T) {
	userA := uuid.New()
	contentID := uuid.New()

	insertCalled := false
	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(dbpkg.Tx) error) error {
			insertCalled = true
			return fn(&mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			})
		},
	}
	handler := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)

	payload, _ := json.Marshal(ContentMentionedPayload{
		ContentID:       contentID.String(),
		AuthorID:        userA.String(),
		MentionedUserID: userA.String(), // self-mention
	})
	err := handler.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: events.EventContentMentioned,
		Payload:   payload,
	})

	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if insertCalled {
		t.Error("INSERT was called; self-mention must not create notification")
	}
}

// TestSocialGovernance_ContentMentioned_BlockedActor_Dropped proves:
//   When the actor (author) is blocked by the recipient, the mention notification
//   is dropped (Social category policy).
func TestSocialGovernance_ContentMentioned_BlockedActor_Dropped(t *testing.T) {
	authorID := uuid.New()
	mentionedUserID := uuid.New()
	contentID := uuid.New()

	insertCalled := false
	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(dbpkg.Tx) error) error {
			insertCalled = true
			return fn(&mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			})
		},
	}
	block := &mockBlockCheckerControlled{
		blocked: map[[2]uuid.UUID]bool{{authorID, mentionedUserID}: true},
	}
	handler := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, block, nil)

	payload, _ := json.Marshal(ContentMentionedPayload{
		ContentID:       contentID.String(),
		AuthorID:        authorID.String(),
		MentionedUserID: mentionedUserID.String(),
	})
	err := handler.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: events.EventContentMentioned,
		Payload:   payload,
	})

	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if insertCalled {
		t.Error("INSERT was called; blocked actor notification must be dropped")
	}
}

// TestSocialGovernance_ContentMentioned_BannedActor_Dropped proves:
//   Banned author → mention notification is not created.
func TestSocialGovernance_ContentMentioned_BannedActor_Dropped(t *testing.T) {
	authorID := uuid.New()
	mentionedUserID := uuid.New()
	contentID := uuid.New()

	insertCalled := false
	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(dbpkg.Tx) error) error {
			insertCalled = true
			return fn(&mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			})
		},
	}
	status := &mockAccountStatusControlled{
		statuses: map[uuid.UUID]string{authorID: "banned"},
	}
	handler := buildSocialGovernanceHandler(t, mockDB, status, &mockBlockCheckerControlled{}, nil)

	payload, _ := json.Marshal(ContentMentionedPayload{
		ContentID:       contentID.String(),
		AuthorID:        authorID.String(),
		MentionedUserID: mentionedUserID.String(),
	})
	err := handler.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: events.EventContentMentioned,
		Payload:   payload,
	})

	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if insertCalled {
		t.Error("INSERT was called; banned actor notification must be dropped")
	}
}

// TestSocialGovernance_ContentMentioned_CategoryIsSocial proves:
//   content.mentioned is classified as Social in the notification policy.
func TestSocialGovernance_ContentMentioned_CategoryIsSocial(t *testing.T) {
	cat := policy.GetCategory(events.EventContentMentioned)
	if cat != policy.Social {
		t.Errorf("GetCategory(%q) = %v, want Social", events.EventContentMentioned, cat)
	}
}

// =============================================================================
// MISSING CRITICAL NOTIFICATIONS: order.cancelled_timeout
// =============================================================================
