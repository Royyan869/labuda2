package entity

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Canonical chat media policy
//
// These tests pin the SERVER-SIDE rules of the register → upload → attach
// pipeline: which MIME types may be attached, where their objects may live, the
// per-type ceilings, and the ownership/lifecycle predicate that stops a
// storage_key from being stolen or attached twice.
// =============================================================================

func TestMediaTypeForContentType_CanonicalVocabulary(t *testing.T) {
	images := []string{"image/jpeg", "image/png", "image/webp", "image/gif"}
	for _, contentType := range images {
		mediaType, err := MediaTypeForContentType(contentType)
		require.NoError(t, err, contentType)
		assert.Equal(t, ChatMediaAssetTypeImage, mediaType, contentType)
	}

	mediaType, err := MediaTypeForContentType("video/mp4")
	require.NoError(t, err)
	assert.Equal(t, ChatMediaAssetTypeVideo, mediaType)

	// The accepted set is exactly the general media-upload MIME vocabulary, so
	// a type the platform lets you upload is a type a message may carry.
	assert.Equal(t, []string{
		"image/gif", "image/jpeg", "image/png", "image/webp", "video/mp4",
	}, AllowedChatMediaContentTypes())
}

func TestMediaTypeForContentType_RejectsEverythingElse(t *testing.T) {
	for _, rejected := range []string{"", "  ", "application/pdf", "image/svg+xml", "video/quicktime", "text/plain"} {
		_, err := MediaTypeForContentType(rejected)
		require.Error(t, err, rejected)
		// The rejection names the vocabulary instead of guessing.
		assert.Contains(t, err.Error(), "image/jpeg")
	}
}

func TestMediaTypeForContentType_TrimsWhitespace(t *testing.T) {
	mediaType, err := MediaTypeForContentType("  image/png  ")
	require.NoError(t, err)
	assert.Equal(t, ChatMediaAssetTypeImage, mediaType)
}

func TestMediaTypeFolder_OwnsItsNamespace(t *testing.T) {
	assert.Equal(t, "images/chat", ChatMediaAssetTypeImage.Folder())
	assert.Equal(t, "videos/chat", ChatMediaAssetTypeVideo.Folder())
}

// The per-type ceilings mirror the mobile composer limits
// (MediaUploadConfig.maxImageSizeMb = 10 / maxVideoSizeMb = 100).
func TestMediaTypeMaxBytes(t *testing.T) {
	assert.Equal(t, int64(10<<20), ChatMediaAssetTypeImage.MaxBytes())
	assert.Equal(t, int64(100<<20), ChatMediaAssetTypeVideo.MaxBytes())
	assert.Equal(t, int64(10<<20), MaxChatImageBytes)
	assert.Equal(t, int64(100<<20), MaxChatVideoBytes)
}

func TestMaxMediaPerMessage_IsTheProductRule(t *testing.T) {
	// 4 foto + 1 video in one composer.
	assert.Equal(t, 5, MaxMediaPerMessage)
}

func TestStorageKeyForRoomMedia_IsRoomScopedAndOwned(t *testing.T) {
	roomID := uuid.New()
	uploaderID := uuid.New()
	now := time.UnixMilli(1_700_000_000_000)

	imageKey := StorageKeyForRoomMedia(roomID, uploaderID, ChatMediaAssetTypeImage, "image/jpeg", now)
	assert.True(t, strings.HasPrefix(imageKey, "images/chat/"+roomID.String()+"/"), imageKey)
	assert.Contains(t, imageKey, uploaderID.String(), "the uploader id is the ownership proof")
	assert.True(t, strings.HasSuffix(imageKey, ".jpg"), imageKey)

	videoKey := StorageKeyForRoomMedia(roomID, uploaderID, ChatMediaAssetTypeVideo, "video/mp4", now)
	assert.True(t, strings.HasPrefix(videoKey, "videos/chat/"+roomID.String()+"/"), videoKey)
	assert.True(t, strings.HasSuffix(videoKey, ".mp4"), videoKey)
}

func TestStorageKeyForRoomMedia_ExtensionsFollowContentType(t *testing.T) {
	roomID, uploaderID := uuid.New(), uuid.New()
	now := time.Now()

	cases := map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/webp": ".webp",
		"image/gif":  ".gif",
		"video/mp4":  ".mp4",
	}
	for contentType, ext := range cases {
		mediaType, err := MediaTypeForContentType(contentType)
		require.NoError(t, err)
		key := StorageKeyForRoomMedia(roomID, uploaderID, mediaType, contentType, now)
		assert.True(t, strings.HasSuffix(key, ext), "%s → %s", contentType, key)
	}

	// An unknown type never yields a silently-wrong extension.
	assert.Equal(t, ".bin", chatMediaExtension("application/octet-stream"))
}

func TestChatMediaAsset_Attachable(t *testing.T) {
	roomID, uploaderID := uuid.New(), uuid.New()
	now := time.Now().UTC()

	newPending := func() *ChatMediaAsset {
		asset, err := NewChatMediaAsset(
			roomID, uploaderID, ChatMediaAssetTypeImage, "image/jpeg",
			"images/chat/x/1.jpg", 1024, now.Add(PendingAssetTTL),
		)
		require.NoError(t, err)
		return asset
	}

	t.Run("pending and in window is attachable", func(t *testing.T) {
		assert.Equal(t, MediaAttachRejection(""), newPending().Attachable(roomID, uploaderID, now))
	})

	t.Run("another room is rejected", func(t *testing.T) {
		assert.Equal(t,
			MediaAttachRejectionRoomMismatch,
			newPending().Attachable(uuid.New(), uploaderID, now),
		)
	})

	t.Run("another uploader is rejected", func(t *testing.T) {
		assert.Equal(t,
			MediaAttachRejectionUploaderMismatch,
			newPending().Attachable(roomID, uuid.New(), now),
		)
	})

	t.Run("expired pending is rejected", func(t *testing.T) {
		assert.Equal(t,
			MediaAttachRejectionExpired,
			newPending().Attachable(roomID, uploaderID, now.Add(PendingAssetTTL+time.Second)),
		)
	})

	t.Run("finalized is never reusable", func(t *testing.T) {
		asset := newPending()
		asset.Finalize(now)
		assert.Equal(t,
			MediaAttachRejectionNotPending,
			asset.Attachable(roomID, uploaderID, now.Add(time.Minute)),
		)
	})
}

func TestChatMediaAsset_FinalizeMakesItPermanent(t *testing.T) {
	now := time.Now().UTC()
	asset, err := NewChatMediaAsset(
		uuid.New(), uuid.New(), ChatMediaAssetTypeVideo, "video/mp4",
		"videos/chat/x/1.mp4", 5<<20, now.Add(PendingAssetTTL),
	)
	require.NoError(t, err)
	require.Equal(t, ChatMediaAssetStatusPending, asset.Status)
	assert.Nil(t, asset.FinalizedAt)

	finalizedAt := now.Add(time.Minute)
	asset.Finalize(finalizedAt)

	assert.Equal(t, ChatMediaAssetStatusFinalized, asset.Status)
	require.NotNil(t, asset.FinalizedAt)
	assert.Equal(t, finalizedAt, *asset.FinalizedAt)
	// An attached asset outlives the pending window by design.
	assert.Equal(t, finalizedAt.Add(PermanentAssetTTL), asset.ExpiresAt)
	assert.True(t, asset.ExpiresAt.After(finalizedAt.Add(PendingAssetTTL)))
}

func TestChatMediaAsset_IsExpiredPending(t *testing.T) {
	now := time.Now().UTC()
	asset, err := NewChatMediaAsset(
		uuid.New(), uuid.New(), ChatMediaAssetTypeImage, "image/jpeg",
		"images/chat/x/1.jpg", 1024, now.Add(PendingAssetTTL),
	)
	require.NoError(t, err)

	// Inside the window: not swept.
	assert.False(t, asset.IsExpiredPending(now))
	// Past the window: swept.
	assert.True(t, asset.IsExpiredPending(now.Add(PendingAssetTTL+time.Second)))
	// Finalized rows are message content — TTL must never claim them.
	asset.Finalize(now)
	assert.False(t, asset.IsExpiredPending(now.Add(PermanentAssetTTL+time.Hour)))
}

func TestNewChatMediaAsset_RequiresItsInputs(t *testing.T) {
	roomID, uploaderID := uuid.New(), uuid.New()
	now := time.Now().UTC()

	cases := map[string]func() (*ChatMediaAsset, error){
		"room": func() (*ChatMediaAsset, error) {
			return NewChatMediaAsset(uuid.Nil, uploaderID, ChatMediaAssetTypeImage, "image/jpeg", "k", 1, now.Add(time.Hour))
		},
		"uploader": func() (*ChatMediaAsset, error) {
			return NewChatMediaAsset(roomID, uuid.Nil, ChatMediaAssetTypeImage, "image/jpeg", "k", 1, now.Add(time.Hour))
		},
		"media type": func() (*ChatMediaAsset, error) {
			return NewChatMediaAsset(roomID, uploaderID, ChatMediaAssetType("audio"), "image/jpeg", "k", 1, now.Add(time.Hour))
		},
		"content type": func() (*ChatMediaAsset, error) {
			return NewChatMediaAsset(roomID, uploaderID, ChatMediaAssetTypeImage, "  ", "k", 1, now.Add(time.Hour))
		},
		"storage key": func() (*ChatMediaAsset, error) {
			return NewChatMediaAsset(roomID, uploaderID, ChatMediaAssetTypeImage, "image/jpeg", "", 1, now.Add(time.Hour))
		},
		"byte size": func() (*ChatMediaAsset, error) {
			return NewChatMediaAsset(roomID, uploaderID, ChatMediaAssetTypeImage, "image/jpeg", "k", 0, now.Add(time.Hour))
		},
		"expiry": func() (*ChatMediaAsset, error) {
			return NewChatMediaAsset(roomID, uploaderID, ChatMediaAssetTypeImage, "image/jpeg", "k", 1, time.Time{})
		},
	}

	// Deterministic order so a failure names exactly one missing input.
	names := make([]string, 0, len(cases))
	for name := range cases {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		asset, err := cases[name]()
		require.Error(t, err, name)
		assert.Nil(t, asset, name)
	}
}
