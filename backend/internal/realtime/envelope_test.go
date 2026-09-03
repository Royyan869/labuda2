package realtime

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func decodeEnvelope(t *testing.T, payload []byte) WSEnvelope {
	t.Helper()
	if len(payload) == 0 {
		t.Fatal("payload is empty")
	}
	var env WSEnvelope
	if err := json.Unmarshal(payload, &env); err != nil {
		t.Fatalf("unmarshal envelope failed: %v", err)
	}
	return env
}

func requireStringField(t *testing.T, data map[string]any, key, want string) {
	t.Helper()
	got, ok := data[key]
	if !ok {
		t.Fatalf("missing field %q", key)
	}
	if got != want {
		t.Fatalf("%s=%v want %s", key, got, want)
	}
}

func requireIntField(t *testing.T, data map[string]any, key string, want int) {
	t.Helper()
	got, ok := data[key]
	if !ok {
		t.Fatalf("missing field %q", key)
	}
	gotFloat, ok := got.(float64)
	if !ok {
		t.Fatalf("%s type=%T want float64", key, got)
	}
	if int(gotFloat) != want {
		t.Fatalf("%s=%v want %d", key, gotFloat, want)
	}
}

func TestMarshalChatMessageSent_CanonicalEnvelope(t *testing.T) {
	roomID := uuid.New()
	messageID := uuid.New()

	env := decodeEnvelope(t, marshalChatMessageSent(roomID, messageID))

	if env.Type != "chat.message.sent" {
		t.Fatalf("type=%q want chat.message.sent", env.Type)
	}
	if env.From != wsServerSender {
		t.Fatalf("from=%q want %q", env.From, wsServerSender)
	}
	if env.ID == "" || env.Timestamp == "" {
		t.Fatalf("id/timestamp must be non-empty: id=%q ts=%q", env.ID, env.Timestamp)
	}
	if got := env.Data["room_id"]; got != roomID.String() {
		t.Fatalf("room_id=%v want %s", got, roomID.String())
	}
	if got := env.Data["message_id"]; got != messageID.String() {
		t.Fatalf("message_id=%v want %s", got, messageID.String())
	}
}

func TestMarshalChatMessageHidden_CanonicalEnvelope(t *testing.T) {
	roomID := uuid.New()
	messageID := uuid.New()

	env := decodeEnvelope(t, marshalChatMessageHidden(roomID, messageID))

	if env.Type != "chat.message.hidden" {
		t.Fatalf("type=%q want chat.message.hidden", env.Type)
	}
	if got := env.Data["room_id"]; got != roomID.String() {
		t.Fatalf("room_id=%v want %s", got, roomID.String())
	}
	if got := env.Data["message_id"]; got != messageID.String() {
		t.Fatalf("message_id=%v want %s", got, messageID.String())
	}
	if _, ok := env.Data["body"]; ok {
		t.Fatal("hidden message envelope must not include body")
	}
	if _, ok := env.Data["attachment"]; ok {
		t.Fatal("hidden message envelope must not include attachment")
	}
}

func TestMarshalChatMessageRestored_CanonicalEnvelope(t *testing.T) {
	roomID := uuid.New()
	messageID := uuid.New()

	env := decodeEnvelope(t, marshalChatMessageRestored(roomID, messageID))

	if env.Type != "chat.message.restored" {
		t.Fatalf("type=%q want chat.message.restored", env.Type)
	}
	if got := env.Data["room_id"]; got != roomID.String() {
		t.Fatalf("room_id=%v want %s", got, roomID.String())
	}
	if got := env.Data["message_id"]; got != messageID.String() {
		t.Fatalf("message_id=%v want %s", got, messageID.String())
	}
	if _, ok := env.Data["body"]; ok {
		t.Fatal("restored message envelope must not include body")
	}
	if _, ok := env.Data["attachment"]; ok {
		t.Fatal("restored message envelope must not include attachment")
	}
}

func TestMarshalChatRoomCreated_CanonicalEnvelope(t *testing.T) {
	roomID := uuid.NewString()
	otherUserID := uuid.NewString()
	linkedOrderID := uuid.NewString()
	lastMessageID := uuid.NewString()
	env := decodeEnvelope(t, marshalChatRoomCreated(ChatRoomSummaryPayload{
		RoomID:        roomID,
		RoomType:      "direct",
		OtherUserID:   otherUserID,
		OtherUser:     map[string]any{"id": uuid.NewString(), "display_name": "Dana"},
		LinkedOrderID: linkedOrderID,
		LastMessage:   map[string]any{"id": lastMessageID, "body": "hello"},
		UnreadCount:   3,
		CreatedAt:     "2026-06-14T00:00:00Z",
		UpdatedAt:     "2026-06-14T00:01:00Z",
		LastMessageAt: "2026-06-14T00:01:00Z",
	}))

	if env.Type != "chat.room.created" {
		t.Fatalf("type=%q want chat.room.created", env.Type)
	}
	requireStringField(t, env.Data, "room_id", roomID)
	requireStringField(t, env.Data, "room_type", "direct")
	requireStringField(t, env.Data, "other_user_id", otherUserID)
	requireIntField(t, env.Data, "unread_count", 3)
	requireStringField(t, env.Data, "created_at", "2026-06-14T00:00:00Z")
	requireStringField(t, env.Data, "updated_at", "2026-06-14T00:01:00Z")
	requireStringField(t, env.Data, "last_message_at", "2026-06-14T00:01:00Z")
	requireStringField(t, env.Data, "linked_order_id", linkedOrderID)
	if env.Data["other_user"] == nil {
		t.Fatal("other_user must be present")
	}
	if env.Data["last_message"] == nil {
		t.Fatal("last_message must be present")
	}
}

func TestMarshalChatRoomUpdated_CanonicalEnvelope(t *testing.T) {
	env := decodeEnvelope(t, marshalChatRoomUpdated(ChatRoomSummaryPayload{
		RoomID:        uuid.NewString(),
		RoomType:      "support",
		OtherUserID:   uuid.NewString(),
		UnreadCount:   0,
		UpdatedAt:     "2026-06-14T00:02:00Z",
		LastMessageAt: "2026-06-14T00:02:00Z",
	}))

	if env.Type != "chat.room.updated" {
		t.Fatalf("type=%q want chat.room.updated", env.Type)
	}
	requireStringField(t, env.Data, "room_type", "support")
	requireIntField(t, env.Data, "unread_count", 0)
	requireStringField(t, env.Data, "updated_at", "2026-06-14T00:02:00Z")
	requireStringField(t, env.Data, "last_message_at", "2026-06-14T00:02:00Z")
}

func TestMarshalChatRoomRemoved_CanonicalEnvelope(t *testing.T) {
	env := decodeEnvelope(t, marshalChatRoomRemoved(ChatRoomRemovedPayload{
		RoomID:    uuid.NewString(),
		Reason:    "visibility_changed",
		UpdatedAt: "2026-06-14T00:03:00Z",
	}))

	if env.Type != "chat.room.removed" {
		t.Fatalf("type=%q want chat.room.removed", env.Type)
	}
	requireStringField(t, env.Data, "reason", "visibility_changed")
	requireStringField(t, env.Data, "updated_at", "2026-06-14T00:03:00Z")
	if _, ok := env.Data["context"]; ok {
		t.Fatal("removed payload must not include context")
	}
	if _, ok := env.Data["last_message"]; ok {
		t.Fatal("removed payload must not include last_message")
	}
	if _, ok := env.Data["unread_count"]; ok {
		t.Fatal("removed payload must not include unread_count")
	}
}

func TestMarshalWSError_CanonicalEnvelope(t *testing.T) {
	env := decodeEnvelope(t, marshalWSError("client-msg-err", "rate_limit_exceeded", "subscribe"))

	if env.Type != "error" {
		t.Fatalf("type=%q want error", env.Type)
	}
	if got := env.Data["code"]; got != "rate_limit_exceeded" {
		t.Fatalf("code=%v want rate_limit_exceeded", got)
	}
	if got := env.Data["action"]; got != "subscribe" {
		t.Fatalf("action=%v want subscribe", got)
	}
	if got := env.Data["message_id"]; got != "client-msg-err" {
		t.Fatalf("message_id=%v want client-msg-err", got)
	}
}

func TestMarshalWSAck_Subscribe_CanonicalEnvelope(t *testing.T) {
	roomID := uuid.New()
	env := decodeEnvelope(t, marshalWSAck("client-msg-ack", "subscribe", roomID))

	if env.Type != "ack" {
		t.Fatalf("type=%q want ack", env.Type)
	}
	if got := env.Data["action"]; got != "subscribe" {
		t.Fatalf("action=%v want subscribe", got)
	}
	if got := env.Data["room_id"]; got != roomID.String() {
		t.Fatalf("room_id=%v want %s", got, roomID.String())
	}
	if got := env.Data["message_id"]; got != "client-msg-ack" {
		t.Fatalf("message_id=%v want client-msg-ack", got)
	}
}

func TestMarshalWSHeartbeat_JSONEnvelope(t *testing.T) {
	payload := marshalWSHeartbeat()
	if string(payload) == "" || string(payload) == "{}" {
		t.Fatalf("heartbeat payload must be non-empty canonical JSON envelope, got=%q", string(payload))
	}

	env := decodeEnvelope(t, payload)
	if env.Type != "heartbeat" {
		t.Fatalf("type=%q want heartbeat", env.Type)
	}
	if env.Data == nil {
		t.Fatal("heartbeat data must not be nil")
	}
	if len(env.Data) != 0 {
		t.Fatalf("heartbeat data must be empty object, got=%v", env.Data)
	}
}

func TestMarshalWSPong_CanonicalEnvelope(t *testing.T) {
	env := decodeEnvelope(t, marshalWSPong("client-msg-ping-1"))

	if env.Type != "pong" {
		t.Fatalf("type=%q want pong", env.Type)
	}
	if got := env.Data["message_id"]; got != "client-msg-ping-1" {
		t.Fatalf("message_id=%v want client-msg-ping-1", got)
	}
}
