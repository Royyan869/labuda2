package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/stretchr/testify/assert"
)

// =====================================================================
// Phase 3 — messageToResponse tombstone tests
// =====================================================================

func TestMessageToResponse_HiddenMessage_SuppressesBody(t *testing.T) {
	now := time.Now()
	adminID := uuid.New()
	reason := "Moderation: hidden by admin"
	body := "this message should be hidden"

	msg := &chatEntity.ChatMessage{
		ID:          uuid.New(),
		RoomID:      uuid.New(),
		SenderID:    uuid.New(),
		MessageType: chatEntity.MessageTypeText,
		Body:        &body,
		AttachmentJSON: map[string]interface{}{
			"type": "reference",
			"data": map[string]interface{}{
				"target_type": "for_sale",
				"target_id":   "abc",
			},
		},
		CreatedAt:      now,
		DeletedAt:      &now,
		DeletedBy:      &adminID,
		DeletionReason: &reason,
	}

	resp := messageToResponse(msg, nil, nil)

	// Tombstone fields present
	assert.Equal(t, true, resp["is_hidden"])

	// Timeline structure preserved
	assert.Equal(t, msg.ID.String(), resp["id"])
	assert.Equal(t, msg.RoomID.String(), resp["room_id"])
	assert.Equal(t, msg.SenderID.String(), resp["sender_id"])
	assert.Equal(t, "text", resp["message_type"])
	assert.NotEmpty(t, resp["created_at"])

	// Body and attachment suppressed
	_, hasBody := resp["body"]
	_, hasAttachment := resp["attachment_json"]
	_, hasAttachmentMetadata := resp["attachment_metadata"]
	assert.False(t, hasBody, "body must be suppressed for hidden messages")
	assert.False(t, hasAttachment, "attachment_json must be suppressed for hidden messages")
	assert.False(t, hasAttachmentMetadata, "attachment_metadata must be suppressed for hidden messages")
}

func TestMessageToResponse_NormalMessage_EmitsBody(t *testing.T) {
	body := "hello world"
	msg := &chatEntity.ChatMessage{
		ID:          uuid.New(),
		RoomID:      uuid.New(),
		SenderID:    uuid.New(),
		MessageType: chatEntity.MessageTypeText,
		Body:        &body,
		CreatedAt:   time.Now(),
		// DeletedAt is nil — not hidden
	}

	resp := messageToResponse(msg, nil, nil)

	// Body present
	assert.Equal(t, "hello world", resp["body"])

	// is_hidden should NOT be present
	_, hasHidden := resp["is_hidden"]
	assert.False(t, hasHidden, "is_hidden must not be present for normal messages")
}

func TestMessageToResponse_HiddenMessage_NoSenderCard(t *testing.T) {
	now := time.Now()
	body := "hidden"
	msg := &chatEntity.ChatMessage{
		ID:          uuid.New(),
		RoomID:      uuid.New(),
		SenderID:    uuid.New(),
		MessageType: chatEntity.MessageTypeText,
		Body:        &body,
		CreatedAt:   now,
		DeletedAt:   &now,
	}

	resp := messageToResponse(msg, nil, nil)

	// Sender card should not be hydrated for hidden messages (early return)
	_, hasSender := resp["sender"]
	assert.False(t, hasSender, "sender card must not be present for hidden messages")
	assert.Equal(t, true, resp["is_hidden"])
}
