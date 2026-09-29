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
// - body can be nil for non-text messages (and for a media-only message)
// - attachmentJSON stores structured data (attachments, proposals, etc.)
// - mediaAssetIDs are the chat media assets this message carries, in attach
//   order; they are part of the command and therefore of the fingerprint
// - command_fingerprint is computed server-side as a canonical SHA-256
//   of the normalized send-message command fields.
func NewChatMessage(
	roomID, senderID uuid.UUID,
	messageType MessageType,
	body *string,
	attachmentJSON map[string]interface{},
	idempotencyKey string,
	mediaAssetIDs []uuid.UUID,
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
		CommandFingerprint: ComputeCommandFingerprint(senderID, messageType, body, attachmentJSON, mediaAssetIDs),
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
		nil,
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
//   - mediaAssetIDs: optional chat media assets, in attach order (may be nil)
//
// The fingerprint is deterministic, idempotent, and changes only when the
// command inputs change. It does NOT depend on the message ID or timestamp.
// This makes it suitable for replay validation as documented in migration
// 000032. Media participates on the same rule as the body: the same
// idempotency key with a DIFFERENT media set is a conflict, never a silent
// replay of the older message.
//
// No fallback, no sentinel, no optional bypass — every message MUST carry
// a non-empty canonical fingerprint per migration 000033.
func ComputeCommandFingerprint(
	senderID uuid.UUID,
	messageType MessageType,
	body *string,
	attachmentJSON map[string]interface{},
	mediaAssetIDs []uuid.UUID,
) string {
	fingerprintInput := map[string]interface{}{
		"sender_id":       senderID.String(),
		"message_type":    string(messageType),
		"body":            body,
		"attachment_json": attachmentJSON,
	}

	// Media is folded in only when present so the canonical fingerprint of a
	// plain text message stays byte-identical to the one migration 000032/33
	// rows were stored with (replay must keep working across this deploy).
	if len(mediaAssetIDs) > 0 {
		ids := make([]string, len(mediaAssetIDs))
		for i, id := range mediaAssetIDs {
			ids[i] = id.String()
		}
		fingerprintInput["media_asset_ids"] = ids
	}

	normalized, err := json.Marshal(fingerprintInput)
	if err != nil {
		// This should never fail — all values are JSON-serializable.
		panic(fmt.Sprintf("chat: failed to normalize fingerprint input: %v", err))
	}

	sum := sha256.Sum256(normalized)
	return hex.EncodeToString(sum[:])
}

// NewSystemMessage creates a system-generated message attributed to a REAL
// actor. chat_messages.sender_id is NOT NULL with an FK to users — uuid.Nil
// violates the constraint, so the historical "system messages don't have a
// sender" contract was unwritable and every call failed at FK enforcement.
//
// The actor is the participant on whose behalf the system speaks (e.g. the
// seller for for_sale.sold notifications in the seller↔buyer room).
// Renderers must treat message_type='system' as the authoritative signal for
// system-authored content, never sender_id.
func NewSystemMessage(roomID uuid.UUID, actorID uuid.UUID, body string, idempotencyKey string) *ChatMessage {
	if actorID == uuid.Nil {
		panic("chat: NewSystemMessage requires a real actor id (sender_id is NOT NULL + FK to users)")
	}
	return NewChatMessage(
		roomID,
		actorID,
		MessageTypeSystem,
		&body,
		nil,
		idempotencyKey,
		nil,
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


