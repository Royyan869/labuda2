package application

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
)

func buildChatRoomUpdatedOutboxPayload(
	room *chatEntity.ChatRoom,
	viewerID uuid.UUID,
	recipientID uuid.UUID,
	lastMessage *chatEntity.ChatMessage,
	unreadCount int,
	preserveRoomTimestamps bool,
) map[string]any {
	payload := buildChatRoomSummaryOutboxPayload(room, viewerID, recipientID, unreadCount)
	if lastMessage != nil {
		payload["last_message"] = buildChatRoomLastMessagePayload(lastMessage)
		if !preserveRoomTimestamps {
			payload["updated_at"] = lastMessage.CreatedAt.UTC().Format(time.RFC3339)
			payload["last_message_at"] = lastMessage.CreatedAt.UTC().Format(time.RFC3339)
		}
	} else {
		payload["last_message"] = nil
	}
	return payload
}

func buildChatRoomCreatedOutboxPayload(
	room *chatEntity.ChatRoom,
	viewerID uuid.UUID,
	recipientID uuid.UUID,
) map[string]any {
	payload := buildChatRoomSummaryOutboxPayload(room, viewerID, recipientID, 0)
	payload["last_message"] = nil
	return payload
}

func buildChatRoomSummaryOutboxPayload(
	room *chatEntity.ChatRoom,
	viewerID uuid.UUID,
	recipientID uuid.UUID,
	unreadCount int,
) map[string]any {
	payload := map[string]any{
		"recipient_id":    recipientID.String(),
		"room_id":         room.ID.String(),
		"room_type":       string(room.RoomType),
		"other_user_id":   room.OtherParticipant(viewerID).String(),
		"unread_count":    unreadCount,
		"created_at":      room.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at":      room.UpdatedAt.UTC().Format(time.RFC3339),
		"last_message_at": room.LastMessageAt.UTC().Format(time.RFC3339),
	}

	if room.HasContext() {
		var contextData any
		if err := json.Unmarshal(room.ContextJSON, &contextData); err == nil {
			payload["context"] = contextData
		}
	}

	if room.HasContext() && room.ContextSetBy != nil {
		payload["context_set_by"] = room.ContextSetBy.String()
	}

	if room.HasLinkedOrder() && room.LinkedOrderID != nil {
		payload["linked_order_id"] = room.LinkedOrderID.String()
	}

	return payload
}

func buildChatRoomLastMessagePayload(msg *chatEntity.ChatMessage) map[string]any {
	payload := map[string]any{
		"id":           msg.ID.String(),
		"room_id":      msg.RoomID.String(),
		"sender_id":    msg.SenderID.String(),
		"message_type": string(msg.MessageType),
		"created_at":   msg.CreatedAt.UTC().Format(time.RFC3339),
	}

	if msg.DeletedAt != nil {
		payload["is_hidden"] = true
		return payload
	}

	if msg.Body != nil {
		payload["body"] = *msg.Body
	}
	if msg.AttachmentJSON != nil {
		payload["attachment_json"] = msg.AttachmentJSON
	}

	return payload
}


