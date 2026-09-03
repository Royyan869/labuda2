package entity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	ID                 uuid.UUID
	RoomID             uuid.UUID
	SenderID           uuid.UUID
	MessageType        MessageType
	Body               *string
	AttachmentJSON     map[string]interface{}
	IdempotencyKey     string
	CommandFingerprint string
	CreatedAt          time.Time

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
// - command_fingerprint is computed server-side as a canonical SHA-256
//   of the normalized send-message command fields.
func NewChatMessage(
	roomID, senderID uuid.UUID,
	messageType MessageType,
	body *string,
	attachmentJSON map[string]interface{},
	idempotencyKey string,
) *ChatMessage {
	now := time.Now()

	return &ChatMessage{
		ID:                 uuid.New(),
		RoomID:             roomID,
		SenderID:           senderID,
		MessageType:        messageType,
		Body:               body,
		AttachmentJSON:     attachmentJSON,
		IdempotencyKey:     idempotencyKey,
		CommandFingerprint: ComputeCommandFingerprint(senderID, messageType, body, attachmentJSON),
		CreatedAt:          now,
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

// ComputeCommandFingerprint computes the canonical server-side SHA-256
// fingerprint of a normalized send-message command.
//
// Inputs (the full set of fields a client controls when sending):
//   - senderID: the authenticated sender
//   - messageType: text, negotiation_proposal, or system
//   - body: optional message body (may be nil)
//   - attachmentJSON: optional structured attachment (may be nil)
//
// The fingerprint is deterministic, idempotent, and changes only when the
// command inputs change. It does NOT depend on the message ID or timestamp.
// This makes it suitable for replay validation as documented in migration
// 000032.
//
// No fallback, no sentinel, no optional bypass — every message MUST carry
// a non-empty canonical fingerprint per migration 000033.
func ComputeCommandFingerprint(
	senderID uuid.UUID,
	messageType MessageType,
	body *string,
	attachmentJSON map[string]interface{},
) string {
	fingerprintInput := map[string]interface{}{
		"sender_id":       senderID.String(),
		"message_type":    string(messageType),
		"body":            body,
		"attachment_json": attachmentJSON,
	}

	normalized, err := json.Marshal(fingerprintInput)
	if err != nil {
		// This should never fail — all values are JSON-serializable.
		panic(fmt.Sprintf("chat: failed to normalize fingerprint input: %v", err))
	}

	sum := sha256.Sum256(normalized)
	return hex.EncodeToString(sum[:])
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


