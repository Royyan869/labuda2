package http

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =====================================================================
// CHAT MESSAGE PREVIEW SHAPE TESTS
// =====================================================================

func TestResourcePreview_ChatMessageFields_Serialized(t *testing.T) {
	preview := &ResourcePreview{
		AuthorID:       "sender-uuid-123",
		AuthorUsername: "testuser",
		ContentText:    "hello world",
		ContentType:    "text",
		IsDeleted:      false,
		RoomID:         "room-uuid-456",
		RoomType:       "normal",
		SentAt:         "2026-05-27T10:30:00Z",
	}

	data, err := json.Marshal(preview)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	assert.Equal(t, "sender-uuid-123", m["author_id"])
	assert.Equal(t, "testuser", m["author_username"])
	assert.Equal(t, "hello world", m["content_text"])
	assert.Equal(t, "text", m["content_type"])
	assert.Equal(t, false, m["is_deleted"])
	assert.Equal(t, "room-uuid-456", m["room_id"])
	assert.Equal(t, "normal", m["room_type"])
	assert.Equal(t, "2026-05-27T10:30:00Z", m["sent_at"])
}

func TestResourcePreview_ChatFields_OmittedForContent(t *testing.T) {
	// Content/comment previews should NOT emit chat-specific fields
	preview := &ResourcePreview{
		AuthorID:       "author-uuid-789",
		AuthorUsername: "poster",
		ContentText:    "my post",
		ContentType:    "post",
		IsDeleted:      false,
		// RoomID, RoomType, SentAt all zero-value (empty string)
	}

	data, err := json.Marshal(preview)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	// Chat fields should be omitted (omitempty)
	_, hasRoomID := m["room_id"]
	_, hasRoomType := m["room_type"]
	_, hasSentAt := m["sent_at"]
	assert.False(t, hasRoomID, "room_id should be omitted for non-chat resources")
	assert.False(t, hasRoomType, "room_type should be omitted for non-chat resources")
	assert.False(t, hasSentAt, "sent_at should be omitted for non-chat resources")
}

func TestResourcePreview_NilPreview_SafeForJSON(t *testing.T) {
	// Nil preview should not panic
	var preview *ResourcePreview
	assert.Nil(t, preview)

	// Simulates what the handler does: if preview != nil { resp["resource_preview"] = preview }
	resp := map[string]interface{}{}
	if preview != nil {
		resp["resource_preview"] = preview
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	_, hasPreview := m["resource_preview"]
	assert.False(t, hasPreview, "nil preview should not appear in response")
}

func TestResourcePreview_DeletedMessage_FlagTrue(t *testing.T) {
	preview := &ResourcePreview{
		AuthorID:       "sender-uuid",
		AuthorUsername: "user1",
		ContentText:    "",
		ContentType:    "text",
		IsDeleted:      true,
		RoomID:         "room-uuid",
		RoomType:       "normal",
		SentAt:         "2026-05-27T10:30:00Z",
	}

	data, err := json.Marshal(preview)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, true, m["is_deleted"])
}

func TestResourcePreview_ChatMessageRedactedBody_OmitsContentText(t *testing.T) {
	preview := &ResourcePreview{
		AuthorID:                   "sender-uuid",
		AuthorUsername:             "user1",
		ContentType:                "text",
		ContentText:                "",
		IsDeleted:                  true,
		EvidenceAvailable:          true,
		EvidenceRequiresCapability: "moderation.evidence.read",
		RoomID:                     "room-uuid",
		RoomType:                   "normal",
		SentAt:                     "2026-05-27T10:30:00Z",
	}

	data, err := json.Marshal(preview)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	_, hasContentText := m["content_text"]
	assert.False(t, hasContentText, "content_text should be omitted for redacted chat previews")
	assert.Equal(t, true, m["evidence_available"])
	assert.Equal(t, "moderation.evidence.read", m["evidence_requires_capability"])
}

func TestResourcePreview_TruncationBoundary(t *testing.T) {
	// Verify truncation works at exactly 200 chars
	longText := ""
	for i := 0; i < 201; i++ {
		longText += "x"
	}

	// Simulate what fetchChatMessagePreview does
	contentText := longText
	if len(contentText) > 200 {
		contentText = contentText[:200] + "..."
	}

	assert.Len(t, contentText, 203) // 200 + "..."
	assert.True(t, contentText[200:] == "...")
}

func TestResourcePreview_ExactlyAtTruncationLimit(t *testing.T) {
	text200 := ""
	for i := 0; i < 200; i++ {
		text200 += "a"
	}

	contentText := text200
	if len(contentText) > 200 {
		contentText = contentText[:200] + "..."
	}

	assert.Len(t, contentText, 200, "exactly 200 chars should not be truncated")
}
