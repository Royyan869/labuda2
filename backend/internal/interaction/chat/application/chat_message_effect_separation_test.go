package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/rate"
	"go.uber.org/zap"
)

// countingTransactor records how many transaction boundaries the service opens.
type countingTransactor struct {
	tx    db.Tx
	calls int
}

func (m *countingTransactor) WithTx(_ context.Context, fn func(db.Tx) error) error {
	m.calls++
	return fn(m.tx)
}

// failingNotificationOutbox fails the notification durable effect while letting
// every other insert through, so the send transaction must abort.
type failingNotificationOutbox struct {
	roomUpdatedMockOutbox
	failOn    string
	failCount int
}

func (m *failingNotificationOutbox) InsertTx(
	ctx context.Context,
	tx db.Tx,
	eventType string,
	payload any,
	idempotencyKey string,
) error {
	if eventType == m.failOn {
		m.failCount++
		return errors.New("injected outbox failure")
	}
	return m.roomUpdatedMockOutbox.InsertTx(ctx, tx, eventType, payload, idempotencyKey)
}

func newEffectSeparationService(t *testing.T, room *chatEntity.ChatRoom, outbox OutboxInserter) (*Service, *countingTransactor) {
	t.Helper()

	repo := &roomUpdatedMockRepo{room: room}
	transactor := &countingTransactor{tx: &roomUpdatedMockTx{}}

	return &Service{
		db:          transactor,
		repo:        repo,
		socialRepo:  &roomUpdatedMockSocialRepo{},
		outboxRepo:  outbox,
		rateLimiter: rate.NewRateLimiter(),
		log:         zap.NewNop(),
	}, transactor
}

// TestSendMessage_BothDurableEffectsInOneTransaction proves the transactional
// requirement: message persistence, the realtime effect and the notification
// effect all happen inside ONE transaction, and each effect is produced exactly
// once with its own owning consumer.
func TestSendMessage_BothDurableEffectsInOneTransaction(t *testing.T) {
	sender := uuid.New()
	recipient := uuid.New()
	room := &chatEntity.ChatRoom{
		ID:           uuid.New(),
		RoomType:     chatEntity.RoomTypeDirect,
		ParticipantA: sender,
		ParticipantB: recipient,
	}

	outbox := &roomUpdatedMockOutbox{}
	svc, transactor := newEffectSeparationService(t, room, outbox)

	body := "hello"
	msg, err := svc.SendMessage(context.Background(), room.ID, sender, chatEntity.MessageTypeText, &body, nil, "idem-effects")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if msg == nil {
		t.Fatal("expected message result")
	}

	if transactor.calls != 1 {
		t.Fatalf("transaction boundaries=%d want 1 (both durable effects must be atomic with persistence)", transactor.calls)
	}

	var realtimeInserts, notificationInserts int
	for _, insert := range outbox.inserts {
		switch insert.eventType {
		case "chat.message.sent":
			realtimeInserts++
			requireOutboxFieldString(t, insert.payload, "message_id", msg.ID.String())
			requireOutboxFieldString(t, insert.payload, "room_id", room.ID.String())
		case "chat.message.notification":
			notificationInserts++
			requireOutboxFieldString(t, insert.payload, "message_id", msg.ID.String())
			requireOutboxFieldString(t, insert.payload, "room_id", room.ID.String())
			requireOutboxFieldString(t, insert.payload, "sender_id", sender.String())
			requireOutboxFieldString(t, insert.payload, "recipient_id", recipient.String())
		}
	}

	if realtimeInserts != 1 {
		t.Fatalf("realtime durable events=%d want exactly 1", realtimeInserts)
	}
	if notificationInserts != 1 {
		t.Fatalf("notification durable events=%d want exactly 1", notificationInserts)
	}
}

// TestSendMessage_NotificationEffectFailureAbortsSend proves the failure
// contract: if the notification durable effect cannot be written, the send does
// NOT succeed — so in the real transaction no message can be persisted without
// its required effects.
func TestSendMessage_NotificationEffectFailureAbortsSend(t *testing.T) {
	sender := uuid.New()
	recipient := uuid.New()
	room := &chatEntity.ChatRoom{
		ID:           uuid.New(),
		RoomType:     chatEntity.RoomTypeDirect,
		ParticipantA: sender,
		ParticipantB: recipient,
	}

	outbox := &failingNotificationOutbox{failOn: "chat.message.notification"}
	svc, _ := newEffectSeparationService(t, room, outbox)

	body := "hello"
	msg, err := svc.SendMessage(context.Background(), room.ID, sender, chatEntity.MessageTypeText, &body, nil, "idem-fail-notif")

	if err == nil {
		t.Fatal("expected SendMessage to fail when the notification effect cannot be written")
	}
	if msg != nil {
		t.Fatal("expected no message result when the notification effect cannot be written")
	}
	if outbox.failCount != 1 {
		t.Fatalf("notification insert attempts=%d want 1", outbox.failCount)
	}
	if !strings.Contains(err.Error(), "chat.message.notification") {
		t.Fatalf("error must name the failed durable effect, got: %v", err)
	}
}

// TestSendMessage_RealtimeEffectFailureAbortsSend is the mirror of the
// notification failure test: losing the realtime effect must abort the send too.
func TestSendMessage_RealtimeEffectFailureAbortsSend(t *testing.T) {
	sender := uuid.New()
	recipient := uuid.New()
	room := &chatEntity.ChatRoom{
		ID:           uuid.New(),
		RoomType:     chatEntity.RoomTypeDirect,
		ParticipantA: sender,
		ParticipantB: recipient,
	}

	outbox := &failingNotificationOutbox{failOn: "chat.message.sent"}
	svc, _ := newEffectSeparationService(t, room, outbox)

	body := "hello"
	msg, err := svc.SendMessage(context.Background(), room.ID, sender, chatEntity.MessageTypeText, &body, nil, "idem-fail-realtime")

	if err == nil {
		t.Fatal("expected SendMessage to fail when the realtime effect cannot be written")
	}
	if msg != nil {
		t.Fatal("expected no message result when the realtime effect cannot be written")
	}
	if outbox.failCount != 1 {
		t.Fatalf("realtime insert attempts=%d want 1", outbox.failCount)
	}
	if !strings.Contains(err.Error(), "chat.message.sent") {
		t.Fatalf("error must name the failed durable effect, got: %v", err)
	}
}
