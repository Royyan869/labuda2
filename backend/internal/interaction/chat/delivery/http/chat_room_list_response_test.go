package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/stretchr/testify/assert"
)

func TestRoomListItemResponse_IncludesLastMessage(t *testing.T) {
	room := chatEntity.NewChatRoom(chatEntity.RoomTypeDirect, uuid.New(), uuid.New())
	room.LastMessageAt = time.Now()
	userID := room.ParticipantA
	body := "hello room list"
	msg := &chatEntity.ChatMessage{
		ID:          uuid.New(),
		RoomID:      room.ID,
		SenderID:    room.ParticipantB,
		MessageType: chatEntity.MessageTypeText,
		Body:        &body,
		CreatedAt:   time.Now(),
	}

	resp := roomListItemResponse(room, userID, nil, msg, 3)
	last, ok := resp["last_message"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "text", last["message_type"])
	assert.Equal(t, "hello room list", last["body"])
	assert.Equal(t, 3, resp["unread_count"])
}

func TestRoomListItemResponse_HiddenLastMessage_Tombstone(t *testing.T) {
	room := chatEntity.NewChatRoom(chatEntity.RoomTypeDirect, uuid.New(), uuid.New())
	room.LastMessageAt = time.Now()
	userID := room.ParticipantA
	now := time.Now()
	body := "should be hidden"
	msg := &chatEntity.ChatMessage{
		ID:          uuid.New(),
		RoomID:      room.ID,
		SenderID:    room.ParticipantB,
		MessageType: chatEntity.MessageTypeText,
		Body:        &body,
		CreatedAt:   now,
		DeletedAt:   &now,
	}

	resp := roomListItemResponse(room, userID, nil, msg, 0)
	last := resp["last_message"].(map[string]interface{})
	assert.Equal(t, true, last["is_hidden"])
	_, hasBody := last["body"]
	_, hasAttachment := last["attachment_json"]
	_, hasAttachmentMeta := last["attachment_metadata"]
	assert.False(t, hasBody)
	assert.False(t, hasAttachment)
	assert.False(t, hasAttachmentMeta)
}

func TestRoomListItemResponse_NoMessage_NullLastMessage(t *testing.T) {
	room := chatEntity.NewChatRoom(chatEntity.RoomTypeDirect, uuid.New(), uuid.New())
	room.LastMessageAt = time.Now()
	resp := roomListItemResponse(room, room.ParticipantA, nil, nil, 0)
	assert.Nil(t, resp["last_message"])
	assert.Equal(t, 0, resp["unread_count"])
}



