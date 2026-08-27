package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	platformevent "github.com/labuda/backend/internal/platform/event"
	"go.uber.org/zap/zaptest"
)

// mockSupportUserReplyService implements SupportUserReplyService for testing.
type mockSupportUserReplyService struct {
	calls   []handleUserReplyCall
	err     error
}

type handleUserReplyCall struct {
	ChatRoomID uuid.UUID
	SenderID   uuid.UUID
}

func (m *mockSupportUserReplyService) HandleUserReply(ctx context.Context, chatRoomID uuid.UUID, senderID uuid.UUID) error {
	m.calls = append(m.calls, handleUserReplyCall{ChatRoomID: chatRoomID, SenderID: senderID})
	return m.err
}

// TestSupportUserReplyHandler_HappyPath verifies that a well-formed
// support.user_replied event delegates to HandleUserReply with correct UUIDs.
func TestSupportUserReplyHandler_HappyPath(t *testing.T) {
	mock := &mockSupportUserReplyService{}
	handler := &supportUserReplyHandler{
		supportService: mock,
		log:            zaptest.NewLogger(t),
	}

	roomID := uuid.New()
	senderID := uuid.New()

	payload, _ := json.Marshal(map[string]interface{}{
		"room_id":    roomID.String(),
		"sender_id":  senderID.String(),
		"message_id": uuid.New().String(),
	})

	event := platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "support.user_replied",
		Payload:   payload,
	}

	err := handler.Handle(context.Background(), event)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 call to HandleUserReply, got %d", len(mock.calls))
	}
	if mock.calls[0].ChatRoomID != roomID {
		t.Errorf("ChatRoomID = %s, want %s", mock.calls[0].ChatRoomID, roomID)
	}
	if mock.calls[0].SenderID != senderID {
		t.Errorf("SenderID = %s, want %s", mock.calls[0].SenderID, senderID)
	}
}

// TestSupportUserReplyHandler_MalformedPayload verifies that invalid JSON
// returns an error (triggering outbox retry).
func TestSupportUserReplyHandler_MalformedPayload(t *testing.T) {
	mock := &mockSupportUserReplyService{}
	handler := &supportUserReplyHandler{
		supportService: mock,
		log:            zaptest.NewLogger(t),
	}

	event := platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "support.user_replied",
		Payload:   []byte(`{invalid json`),
	}

	err := handler.Handle(context.Background(), event)
	if err == nil {
		t.Fatal("expected error for malformed payload")
	}

	if len(mock.calls) != 0 {
		t.Error("HandleUserReply should not be called for malformed payload")
	}
}

// TestSupportUserReplyHandler_InvalidRoomID verifies that a non-UUID room_id
// returns an error.
func TestSupportUserReplyHandler_InvalidRoomID(t *testing.T) {
	mock := &mockSupportUserReplyService{}
	handler := &supportUserReplyHandler{
		supportService: mock,
		log:            zaptest.NewLogger(t),
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"room_id":    "not-a-uuid",
		"sender_id":  uuid.New().String(),
		"message_id": uuid.New().String(),
	})

	event := platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "support.user_replied",
		Payload:   payload,
	}

	err := handler.Handle(context.Background(), event)
	if err == nil {
		t.Fatal("expected error for invalid room_id")
	}

	if len(mock.calls) != 0 {
		t.Error("HandleUserReply should not be called for invalid room_id")
	}
}

// TestSupportUserReplyHandler_InvalidSenderID verifies that a non-UUID sender_id
// returns an error.
func TestSupportUserReplyHandler_InvalidSenderID(t *testing.T) {
	mock := &mockSupportUserReplyService{}
	handler := &supportUserReplyHandler{
		supportService: mock,
		log:            zaptest.NewLogger(t),
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"room_id":    uuid.New().String(),
		"sender_id":  "not-a-uuid",
		"message_id": uuid.New().String(),
	})

	event := platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "support.user_replied",
		Payload:   payload,
	}

	err := handler.Handle(context.Background(), event)
	if err == nil {
		t.Fatal("expected error for invalid sender_id")
	}

	if len(mock.calls) != 0 {
		t.Error("HandleUserReply should not be called for invalid sender_id")
	}
}

// TestSupportUserReplyHandler_ServiceError verifies that service errors propagate
// (triggering outbox retry).
func TestSupportUserReplyHandler_ServiceError(t *testing.T) {
	mock := &mockSupportUserReplyService{
		err: errors.New("database unavailable"),
	}
	handler := &supportUserReplyHandler{
		supportService: mock,
		log:            zaptest.NewLogger(t),
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"room_id":    uuid.New().String(),
		"sender_id":  uuid.New().String(),
		"message_id": uuid.New().String(),
	})

	event := platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "support.user_replied",
		Payload:   payload,
	}

	err := handler.Handle(context.Background(), event)
	if err == nil {
		t.Fatal("expected error when service fails")
	}

	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 call to HandleUserReply, got %d", len(mock.calls))
	}
}


