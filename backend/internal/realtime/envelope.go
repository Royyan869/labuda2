package realtime

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const wsServerSender = "server"

const (
	EventTypeChatRoomCreated = "chat.room.created"
	EventTypeChatRoomUpdated = "chat.room.updated"
	EventTypeChatRoomRemoved = "chat.room.removed"
)

// WSEnvelope is the canonical outbound WS contract for Labuda.
type WSEnvelope struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Timestamp string         `json:"timestamp"`
	From      string         `json:"from"`
	Data      map[string]any `json:"data"`
}

// ChatRoomSummaryPayload mirrors the room-list REST item used by `/chat/rooms`.
// It is the canonical WS payload for room-created and room-updated events.
type ChatRoomSummaryPayload struct {
	RoomID        string `json:"room_id"`
	RoomType      string `json:"room_type"`
	OtherUserID   string `json:"other_user_id,omitempty"`
	OtherUser     any    `json:"other_user,omitempty"`
	Context       any    `json:"context,omitempty"`
	ContextSetBy  string `json:"context_set_by,omitempty"`
	LinkedOrderID string `json:"linked_order_id,omitempty"`
	LastMessage   any    `json:"last_message,omitempty"`
	UnreadCount   int    `json:"unread_count"`
	CreatedAt     string `json:"created_at,omitempty"`
	UpdatedAt     string `json:"updated_at"`
	LastMessageAt string `json:"last_message_at"`
}

// ChatRoomRemovedPayload is the canonical WS tombstone for room removal.
// Keep it minimal so removed events do not leak room content.
type ChatRoomRemovedPayload struct {
	RoomID    string `json:"room_id"`
	Reason    string `json:"reason,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

func marshalWSEnvelope(messageType string, data map[string]any) []byte {
	env := WSEnvelope{
		ID:        uuid.NewString(),
		Type:      messageType,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		From:      wsServerSender,
		Data:      data,
	}
	payload, _ := json.Marshal(env)
	return payload
}

func (p ChatRoomSummaryPayload) toMap() map[string]any {
	data := map[string]any{
		"room_id":         p.RoomID,
		"room_type":       p.RoomType,
		"unread_count":    p.UnreadCount,
		"updated_at":      p.UpdatedAt,
		"last_message_at": p.LastMessageAt,
	}
	if p.OtherUserID != "" {
		data["other_user_id"] = p.OtherUserID
	}
	if p.OtherUser != nil {
		data["other_user"] = p.OtherUser
	}
	if p.Context != nil {
		data["context"] = p.Context
	}
	if p.ContextSetBy != "" {
		data["context_set_by"] = p.ContextSetBy
	}
	if p.LinkedOrderID != "" {
		data["linked_order_id"] = p.LinkedOrderID
	}
	if p.LastMessage != nil {
		data["last_message"] = p.LastMessage
	}
	if p.CreatedAt != "" {
		data["created_at"] = p.CreatedAt
	}
	return data
}

func (p ChatRoomRemovedPayload) toMap() map[string]any {
	data := map[string]any{
		"room_id": p.RoomID,
	}
	if p.Reason != "" {
		data["reason"] = p.Reason
	}
	if p.UpdatedAt != "" {
		data["updated_at"] = p.UpdatedAt
	}
	return data
}

func marshalChatMessageSent(roomID, messageID uuid.UUID) []byte {
	return marshalWSEnvelope("chat.message.sent", map[string]any{
		"room_id":    roomID.String(),
		"message_id": messageID.String(),
	})
}

func marshalChatMessageHidden(roomID, messageID uuid.UUID) []byte {
	return marshalWSEnvelope("chat.message.hidden", map[string]any{
		"room_id":    roomID.String(),
		"message_id": messageID.String(),
	})
}

func marshalChatMessageRestored(roomID, messageID uuid.UUID) []byte {
	return marshalWSEnvelope("chat.message.restored", map[string]any{
		"room_id":    roomID.String(),
		"message_id": messageID.String(),
	})
}

func marshalChatRoomCreated(payload ChatRoomSummaryPayload) []byte {
	return marshalWSEnvelope(EventTypeChatRoomCreated, payload.toMap())
}

func marshalChatRoomUpdated(payload ChatRoomSummaryPayload) []byte {
	return marshalWSEnvelope(EventTypeChatRoomUpdated, payload.toMap())
}

func marshalChatRoomRemoved(payload ChatRoomRemovedPayload) []byte {
	return marshalWSEnvelope(EventTypeChatRoomRemoved, payload.toMap())
}

func marshalWSError(messageID, code, action string) []byte {
	data := map[string]any{
		"code":   code,
		"action": action,
	}
	if messageID != "" {
		data["message_id"] = messageID
	}
	return marshalWSEnvelope("error", data)
}

func marshalWSAck(messageID, action string, roomID uuid.UUID) []byte {
	data := map[string]any{
		"action":  action,
		"room_id": roomID.String(),
	}
	if messageID != "" {
		data["message_id"] = messageID
	}
	return marshalWSEnvelope("ack", data)
}

func marshalWSPong(messageID string) []byte {
	data := map[string]any{}
	if messageID != "" {
		data["message_id"] = messageID
	}
	return marshalWSEnvelope("pong", data)
}

func marshalWSHeartbeat() []byte {
	return marshalWSEnvelope("heartbeat", map[string]any{})
}


