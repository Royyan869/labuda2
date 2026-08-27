package realtime

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestParseClientControlMessage_CanonicalSubscribe(t *testing.T) {
	roomID := uuid.New()
	raw, _ := json.Marshal(map[string]any{
		"id":   "client-msg-1",
		"type": "subscribe",
		"data": map[string]any{
			"room_id": roomID.String(),
		},
	})

	msg, messageID, ok := parseClientControlMessage(raw)
	if !ok {
		t.Fatal("expected canonical subscribe to parse")
	}
	if msg.Action != "subscribe" {
		t.Fatalf("action=%q want subscribe", msg.Action)
	}
	if msg.RoomID != roomID {
		t.Fatalf("room_id=%s want %s", msg.RoomID, roomID)
	}
	if messageID != "client-msg-1" {
		t.Fatalf("message_id=%q want client-msg-1", messageID)
	}
}

func TestParseClientControlMessage_CanonicalUnsubscribe(t *testing.T) {
	roomID := uuid.New()
	raw, _ := json.Marshal(map[string]any{
		"id":   "client-msg-2",
		"type": "unsubscribe",
		"data": map[string]any{
			"room_id": roomID.String(),
		},
	})

	msg, messageID, ok := parseClientControlMessage(raw)
	if !ok {
		t.Fatal("expected canonical unsubscribe to parse")
	}
	if msg.Action != "unsubscribe" {
		t.Fatalf("action=%q want unsubscribe", msg.Action)
	}
	if msg.RoomID != roomID {
		t.Fatalf("room_id=%s want %s", msg.RoomID, roomID)
	}
	if messageID != "client-msg-2" {
		t.Fatalf("message_id=%q want client-msg-2", messageID)
	}
}

func TestParseClientControlMessage_LegacyStillAccepted(t *testing.T) {
	roomID := uuid.New()
	raw, _ := json.Marshal(map[string]any{
		"action":  "subscribe",
		"room_id": roomID.String(),
	})

	msg, messageID, ok := parseClientControlMessage(raw)
	if !ok {
		t.Fatal("expected legacy message to parse")
	}
	if msg.Action != "subscribe" {
		t.Fatalf("action=%q want subscribe", msg.Action)
	}
	if msg.RoomID != roomID {
		t.Fatalf("room_id=%s want %s", msg.RoomID, roomID)
	}
	if messageID != "" {
		t.Fatalf("legacy message_id=%q want empty", messageID)
	}
}

func TestParseClientControlMessage_CanonicalPing_NoRoomRequired(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"id":   "client-msg-ping-1",
		"type": "ping",
		"data": map[string]any{},
	})

	msg, messageID, ok := parseClientControlMessage(raw)
	if !ok {
		t.Fatal("expected canonical ping to parse")
	}
	if msg.Action != "ping" {
		t.Fatalf("action=%q want ping", msg.Action)
	}
	if msg.RoomID != uuid.Nil {
		t.Fatalf("room_id=%s want nil UUID for ping", msg.RoomID)
	}
	if messageID != "client-msg-ping-1" {
		t.Fatalf("message_id=%q want client-msg-ping-1", messageID)
	}
}


