package realtime

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

func TestDispatcher_ChatRoomCreated_BroadcastsToRecipientConnectionsOnly(t *testing.T) {
	hub := NewHub(zap.NewNop())
	roomID := uuid.New()
	recipientID := uuid.New()
	otherUserID := uuid.New()

	recipientConn1 := &Connection{
		ID:     "recipient-1",
		UserID: recipientID,
		Send:   make(chan []byte, 1),
		Rooms:  map[uuid.UUID]struct{}{},
	}
	recipientConn2 := &Connection{
		ID:     "recipient-2",
		UserID: recipientID,
		Send:   make(chan []byte, 1),
		Rooms:  map[uuid.UUID]struct{}{},
	}
	otherConn := &Connection{
		ID:     "other-1",
		UserID: otherUserID,
		Send:   make(chan []byte, 1),
		Rooms:  map[uuid.UUID]struct{}{},
	}

	hub.Register(recipientConn1)
	hub.Register(recipientConn2)
	hub.Register(otherConn)
	hub.Subscribe(otherConn, roomID)

	dispatcher := NewDispatcherWithRoomResolver(hub, testStatusChecker{}, nil, zap.NewNop())
	payload := mustRoomEventPayload(t, map[string]any{
		"recipient_id":    recipientID.String(),
		"room_id":         roomID.String(),
		"room_type":       "direct",
		"other_user_id":   uuid.NewString(),
		"unread_count":    0,
		"created_at":      "2026-06-14T00:00:00Z",
		"updated_at":      "2026-06-14T00:00:00Z",
		"last_message_at": "2026-06-14T00:00:00Z",
	})

	if err := dispatcher.Dispatch(EventTypeChatRoomCreated, payload); err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	assertSingleEnvelope(t, recipientConn1.Send, EventTypeChatRoomCreated, roomID.String(), false)
	assertSingleEnvelope(t, recipientConn2.Send, EventTypeChatRoomCreated, roomID.String(), false)
	assertNoEnvelope(t, otherConn.Send)
}

func TestDispatcher_ChatRoomUpdated_BroadcastsToRecipientConnectionsOnly(t *testing.T) {
	hub := NewHub(zap.NewNop())
	roomID := uuid.New()
	recipientID := uuid.New()
	otherUserID := uuid.New()

	recipientConn := &Connection{
		ID:     "recipient-1",
		UserID: recipientID,
		Send:   make(chan []byte, 1),
		Rooms:  map[uuid.UUID]struct{}{},
	}
	otherConn := &Connection{
		ID:     "other-1",
		UserID: otherUserID,
		Send:   make(chan []byte, 1),
		Rooms:  map[uuid.UUID]struct{}{},
	}

	hub.Register(recipientConn)
	hub.Register(otherConn)
	hub.Subscribe(otherConn, roomID)

	dispatcher := NewDispatcherWithRoomResolver(hub, testStatusChecker{}, nil, zap.NewNop())
	payload := mustRoomEventPayload(t, map[string]any{
		"recipient_id":    recipientID.String(),
		"room_id":         roomID.String(),
		"room_type":       "direct",
		"other_user_id":   uuid.NewString(),
		"unread_count":    4,
		"created_at":      "2026-06-14T00:00:00Z",
		"updated_at":      "2026-06-14T00:01:00Z",
		"last_message_at": "2026-06-14T00:01:00Z",
		"last_message": map[string]any{
			"id":           uuid.NewString(),
			"room_id":      roomID.String(),
			"sender_id":    uuid.NewString(),
			"message_type": "text",
			"body":         "hello",
			"created_at":   "2026-06-14T00:01:00Z",
		},
	})

	if err := dispatcher.Dispatch(EventTypeChatRoomUpdated, payload); err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	assertSingleEnvelope(t, recipientConn.Send, EventTypeChatRoomUpdated, roomID.String(), true)
	assertNoEnvelope(t, otherConn.Send)
}

func TestDispatcher_ChatRoomCreated_MissingRecipientFailsClosed(t *testing.T) {
	hub := NewHub(zap.NewNop())
	recipientID := uuid.New()
	conn := &Connection{
		ID:     "recipient-1",
		UserID: recipientID,
		Send:   make(chan []byte, 1),
		Rooms:  map[uuid.UUID]struct{}{},
	}
	hub.Register(conn)

	dispatcher := NewDispatcherWithRoomResolver(hub, testStatusChecker{}, nil, zap.NewNop())
	payload := mustRoomEventPayload(t, map[string]any{
		"room_id":         uuid.NewString(),
		"room_type":       "direct",
		"other_user_id":   uuid.NewString(),
		"unread_count":    0,
		"created_at":      "2026-06-14T00:00:00Z",
		"updated_at":      "2026-06-14T00:00:00Z",
		"last_message_at": "2026-06-14T00:00:00Z",
	})

	if err := dispatcher.Dispatch(EventTypeChatRoomCreated, payload); err == nil {
		t.Fatal("expected error for missing recipient_id")
	}
	assertNoEnvelope(t, conn.Send)
}

func mustRoomEventPayload(t *testing.T, payload map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload failed: %v", err)
	}
	return raw
}

func assertSingleEnvelope(t *testing.T, ch <-chan []byte, wantType, wantRoomID string, wantLastMessage bool) {
	t.Helper()
	select {
	case raw := <-ch:
		var env WSEnvelope
		if err := json.Unmarshal(raw, &env); err != nil {
			t.Fatalf("invalid ws envelope: %v", err)
		}
		if env.Type != wantType {
			t.Fatalf("type=%q want %q", env.Type, wantType)
		}
		if got := env.Data["room_id"]; got != wantRoomID {
			t.Fatalf("room_id=%v want %s", got, wantRoomID)
		}
		if _, ok := env.Data["recipient_id"]; ok {
			t.Fatal("recipient_id must not be included in outbound WS payload")
		}
		if wantLastMessage {
			if env.Data["last_message"] == nil {
				t.Fatal("last_message must be present")
			}
		} else if _, ok := env.Data["last_message"]; ok {
			t.Fatal("last_message must be omitted for created payload")
		}
	default:
		t.Fatal("expected websocket frame")
	}
}

func assertNoEnvelope(t *testing.T, ch <-chan []byte) {
	t.Helper()
	select {
	case raw := <-ch:
		t.Fatalf("unexpected websocket frame: %s", string(raw))
	default:
	}
}


