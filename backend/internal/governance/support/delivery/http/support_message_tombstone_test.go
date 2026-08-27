package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/stretchr/testify/assert"
)

func TestSupportMessageToResponse_HiddenMessage_SuppressesBodyAndAttachment(t *testing.T) {
	now := time.Now()
	body := "sensitive content"

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
		CreatedAt: now,
		DeletedAt: &now,
	}

	resp := supportMessageToResponse(msg, uuid.New())

	assert.Equal(t, true, resp["is_hidden"])
	_, hasBody := resp["body"]
	_, hasAttachment := resp["attachment_json"]
	assert.False(t, hasBody, "hidden message must not expose body")
	assert.False(t, hasAttachment, "hidden message must not expose attachment")
}

func TestSupportMessageToResponse_VisibleMessage_StillEmitsBodyAndAttachment(t *testing.T) {
	body := "regular support message"
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
		CreatedAt: time.Now(),
	}

	resp := supportMessageToResponse(msg, uuid.New())

	assert.Equal(t, body, resp["body"])
	assert.NotNil(t, resp["attachment_json"])
	_, hasHidden := resp["is_hidden"]
	assert.False(t, hasHidden, "visible message must not be marked hidden")
}
