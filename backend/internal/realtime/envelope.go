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

	// EventTypeNotificationCreated is the user-targeted realtime signal that a
	// notification row was committed for the recipient (TASK_5). The database
	// remains the notification authority; this event is a delivery mechanism
	// only. The payload carries the canonical post-insert unread_count so the
	// client can reconcile its badge without treating the WS frame as truth.
	EventTypeNotificationCreated = "notification.created"

	// EventTypeNotificationUpdated is the user-targeted realtime signal that
	// a committed mutation CHANGED the recipient's notification state
	// (TASK_4.2): mark-read, mark-all-read, delete, or the chat-room-read
	// notification sync. One event represents the whole user-level state
	// change — never one event per affected row. The database remains the
	// authority; consumers invalidate their canonical providers and re-read.
	EventTypeNotificationUpdated = "notification.updated"
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
	LinkedOrderID string `json:"linked_order_id,omitempty"`
	LastMessage   any    `json:"last_message,omitempty"`
	UnreadCount   int    `json:"unread_count"`
	CreatedAt     string `json:"created_at,omitempty"`
	UpdatedAt     string `json:"updated_at"`
	LastMessageAt string `json:"last_message_at"`
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

func marshalChatMessageSent(roomID, messageID uuid.UUID) []byte {
	return marshalWSEnvelope("chat.message.sent", map[string]any{
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

// marshalNotificationCreated builds the canonical WS frame for a committed
// notification. Minimal by design (ADR-005): the frame carries only what the
// client needs to reconcile — the created notification's id/type and the
// canonical unread_count measured in the creation transaction. The client
// re-fetches the list over REST; the frame is never notification truth.
func marshalNotificationCreated(notificationID uuid.UUID, notifyType string, unreadCount int) []byte {
	return marshalWSEnvelope(EventTypeNotificationCreated, map[string]any{
		"notification_id": notificationID.String(),
		"type":            notifyType,
		"unread_count":    unreadCount,
	})
}

// marshalNotificationUpdated builds the canonical WS frame for a committed
// notification-state mutation (TASK_4.2). Deliberately minimal: one frame per
// user-level state change with no per-row semantics — the client treats it
// as an invalidation signal and re-reads the canonical count/list. The
// unread_count is a supplemental hint, never truth.
func marshalNotificationUpdated(unreadCount int) []byte {
	return marshalWSEnvelope(EventTypeNotificationUpdated, map[string]any{
		"unread_count": unreadCount,
	})
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
