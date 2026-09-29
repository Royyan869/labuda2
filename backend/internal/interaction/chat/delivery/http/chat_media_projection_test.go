package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =====================================================================
// Chat media projection contract
//
// Media is ORTHOGONAL to message_type: the asset rows carry image|video and the
// message carries the ordered list. The wire shape therefore is
// `text + media_urls + has_media` for a photo/video message, and an absent
// media block for every message that has none.
// =====================================================================

func TestMessageToResponse_MediaUrlsAreProjectedInOrder(t *testing.T) {
	body := "lihat ini"
	msg := &chatEntity.ChatMessage{
		ID:          uuid.New(),
		RoomID:      uuid.New(),
		SenderID:    uuid.New(),
		MessageType: chatEntity.MessageTypeText,
		Body:        &body,
		CreatedAt:   time.Now(),
	}

	// ListMessages/GetRoom/Send produce the SAME shape: an ordered url slice.
	urls := []string{
		"https://cdn.labuda.app/images/chat/room/1_user.jpg",
		"https://cdn.labuda.app/videos/chat/room/2_user.mp4",
	}

	resp := messageToResponse(msg, nil, nil, urls)

	assert.Equal(t, urls, resp["media_urls"])
	assert.Equal(t, true, resp["has_media"])
	assert.Equal(t, "text", resp["message_type"], "media does not change the stored message type")
	assert.Equal(t, body, resp["body"])
}

func TestMessageToResponse_MediaOnlyMessageHasNoBody(t *testing.T) {
	// A caption-less foto/video message: the body field is simply absent.
	msg := &chatEntity.ChatMessage{
		ID:          uuid.New(),
		RoomID:      uuid.New(),
		SenderID:    uuid.New(),
		MessageType: chatEntity.MessageTypeText,
		CreatedAt:   time.Now(),
	}

	resp := messageToResponse(msg, nil, nil, []string{"https://cdn.labuda.app/images/chat/room/1_user.jpg"})

	assert.NotContains(t, resp, "body")
	assert.Equal(t, true, resp["has_media"])
	assert.Len(t, resp["media_urls"], 1)
}

func TestMessageToResponse_NoMediaMeansNoMediaKeys(t *testing.T) {
	body := "pesan biasa"
	msg := &chatEntity.ChatMessage{
		ID:          uuid.New(),
		RoomID:      uuid.New(),
		SenderID:    uuid.New(),
		MessageType: chatEntity.MessageTypeText,
		Body:        &body,
		CreatedAt:   time.Now(),
	}

	resp := messageToResponse(msg, nil, nil, nil)

	// Negative proof: a text message must never grow a phantom media block.
	assert.NotContains(t, resp, "media_urls")
	assert.NotContains(t, resp, "has_media")
}

func TestMessageToResponse_TombstoneNeverLeaksMedia(t *testing.T) {
	// A hidden (moderated) message keeps its timeline identity but must not leak
	// body, attachment, or media urls.
	now := time.Now()
	adminID := uuid.New()
	reason := "Moderation: hidden by admin"
	body := "media pesan yang disembunyikan"

	msg := &chatEntity.ChatMessage{
		ID:             uuid.New(),
		RoomID:         uuid.New(),
		SenderID:       uuid.New(),
		MessageType:    chatEntity.MessageTypeText,
		Body:           &body,
		CreatedAt:      now,
		DeletedAt:      &now,
		DeletedBy:      &adminID,
		DeletionReason: &reason,
	}

	resp := messageToResponse(msg, nil, nil, []string{"https://cdn.labuda.app/images/chat/room/1_user.jpg"})

	require.Equal(t, true, resp["is_hidden"])
	assert.NotContains(t, resp, "media_urls")
	assert.NotContains(t, resp, "has_media")
	assert.NotContains(t, resp, "body")
}

// =====================================================================
// Attach policy vocabulary
// =====================================================================

func TestChatMediaPolicy_IsServerEnforcedNotClientHinted(t *testing.T) {
	// The caps the register/attach paths enforce. If one of these moves, the
	// contract changed and this test is the gate that says so.
	assert.Equal(t, 5, chatEntity.MaxMediaPerMessage)
	assert.Equal(t, int64(10<<20), chatEntity.MaxChatImageBytes)
	assert.Equal(t, int64(100<<20), chatEntity.MaxChatVideoBytes)
	assert.Equal(t, 24*time.Hour, chatEntity.PendingAssetTTL)

	// The pending window is what makes \"upload lalu batal\" free: an unattached
	// asset is swept instead of lingering as attachable authority.
	assert.True(t, chatEntity.PermanentAssetTTL > chatEntity.PendingAssetTTL)
}
