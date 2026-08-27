package entity

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ChatMessage represents a message in a chat room.
//
// STRICT RULES:
// - Immutable after creation (no Update method)
// - No Delete method (append-only for creation)
// - IdempotencyKey required for duplicate prevention
// - No financial state in this entity
//
// Deletion metadata (DeletedAt/DeletedBy/DeletionReason) is write-once
// by the moderation enforcement path only. The entity does not expose
// a Delete() method — enforcement writes directly via repository SQL.
type ChatMessage struct {
	ID             uuid.UUID
	RoomID         uuid.UUID
	SenderID       uuid.UUID
	MessageType    MessageType
	Body           *string
	AttachmentJSON map[string]interface{}
	IdempotencyKey string
	CreatedAt      time.Time

	// Moderation soft-hide fields (populated from DB, read-only on entity)
	DeletedAt      *time.Time
	DeletedBy      *uuid.UUID
	DeletionReason *string
}

// NewChatMessage creates a new chat message.
//
// Rules:
// - idempotencyKey is required for duplicate prevention
// - messageType must be valid
// - body can be nil for non-text messages
// - attachmentJSON stores structured data (attachments, proposals, etc.)
func NewChatMessage(
	roomID, senderID uuid.UUID,
	messageType MessageType,
	body *string,
	attachmentJSON map[string]interface{},
	idempotencyKey string,
) *ChatMessage {
	now := time.Now()

	return &ChatMessage{
		ID:             uuid.New(),
		RoomID:         roomID,
		SenderID:       senderID,
		MessageType:    messageType,
		Body:           body,
		AttachmentJSON: attachmentJSON,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      now,
	}
}

// NewTextMessage creates a new text message.
func NewTextMessage(roomID, senderID uuid.UUID, body string, idempotencyKey string) *ChatMessage {
	return NewChatMessage(
		roomID,
		senderID,
		MessageTypeText,
		&body,
		nil,
		idempotencyKey,
	)
}

// NewSystemMessage creates a new system-generated message.
func NewSystemMessage(roomID uuid.UUID, body string, idempotencyKey string) *ChatMessage {
	// System messages don't have a sender - use nil UUID
	return NewChatMessage(
		roomID,
		uuid.Nil,
		MessageTypeSystem,
		&body,
		nil,
		idempotencyKey,
	)
}

// GetAttachmentJSON returns the attachment JSON as raw bytes.
// Returns nil if there's no attachment.
func (m *ChatMessage) GetAttachmentJSON() []byte {
	if m.AttachmentJSON == nil {
		return nil
	}
	data, err := json.Marshal(m.AttachmentJSON)
	if err != nil {
		return nil
	}
	return data
}

// IsSystem returns true if this is a system-generated message.
func (m *ChatMessage) IsSystem() bool {
	return m.MessageType == MessageTypeSystem
}


