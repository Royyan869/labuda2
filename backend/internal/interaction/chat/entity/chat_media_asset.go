package entity

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ChatMediaAssetStatus represents the lifecycle of an uploaded chat media asset.
type ChatMediaAssetStatus string

const (
	ChatMediaAssetStatusPending   ChatMediaAssetStatus = "pending"
	ChatMediaAssetStatusFinalized ChatMediaAssetStatus = "finalized"
	ChatMediaAssetStatusDeleted   ChatMediaAssetStatus = "deleted"
)

func (s ChatMediaAssetStatus) IsValid() bool {
	switch s {
	case ChatMediaAssetStatusPending, ChatMediaAssetStatusFinalized, ChatMediaAssetStatusDeleted:
		return true
	default:
		return false
	}
}

// ChatMediaAssetType identifies the canonical media type for chat uploads.
type ChatMediaAssetType string

const (
	ChatMediaAssetTypeImage ChatMediaAssetType = "image"
	ChatMediaAssetTypeVideo ChatMediaAssetType = "video"
)

func (t ChatMediaAssetType) IsValid() bool {
	switch t {
	case ChatMediaAssetTypeImage, ChatMediaAssetTypeVideo:
		return true
	default:
		return false
	}
}

// ChatMediaAsset is a canonical room-scoped media object.
type ChatMediaAsset struct {
	ID                  uuid.UUID
	RoomID              uuid.UUID
	UploaderID          uuid.UUID
	MediaType           ChatMediaAssetType
	ContentType         string
	StorageKey          string
	ThumbnailStorageKey *string
	ByteSize            int64
	SortOrder           int
	Width               *int
	Height              *int
	DurationMs          *int
	Status              ChatMediaAssetStatus
	ExpiresAt           time.Time
	CreatedAt           time.Time
	FinalizedAt         *time.Time
	DeletedAt           *time.Time
	DeletedBy           *uuid.UUID
	DeletionReason      *string
}

func NewChatMediaAsset(
	roomID, uploaderID uuid.UUID,
	mediaType ChatMediaAssetType,
	contentType string,
	storageKey string,
	byteSize int64,
	expiresAt time.Time,
) (*ChatMediaAsset, error) {
	if roomID == uuid.Nil {
		return nil, fmt.Errorf("room_id is required")
	}
	if uploaderID == uuid.Nil {
		return nil, fmt.Errorf("uploader_id is required")
	}
	if !mediaType.IsValid() {
		return nil, fmt.Errorf("invalid media type: %s", mediaType)
	}
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		return nil, fmt.Errorf("content_type is required")
	}
	storageKey = strings.TrimSpace(storageKey)
	if storageKey == "" {
		return nil, fmt.Errorf("storage_key is required")
	}
	if byteSize <= 0 {
		return nil, fmt.Errorf("byte_size is required")
	}
	if expiresAt.IsZero() {
		return nil, fmt.Errorf("expires_at is required")
	}
	now := time.Now().UTC()
	return &ChatMediaAsset{
		ID:          uuid.New(),
		RoomID:      roomID,
		UploaderID:  uploaderID,
		MediaType:   mediaType,
		ContentType: contentType,
		StorageKey:  storageKey,
		ByteSize:    byteSize,
		Status:      ChatMediaAssetStatusPending,
		ExpiresAt:   expiresAt,
		CreatedAt:   now,
	}, nil
}

// ChatReplyPreview is the canonical reply snapshot emitted by the server.
type ChatReplyPreview struct {
	MessageID  uuid.UUID
	Content    string
	SenderName string
	Type       string
	IsHidden   bool
}

func (p ChatReplyPreview) IsZero() bool {
	return p.MessageID == uuid.Nil && p.Content == "" && p.SenderName == "" && p.Type == "" && !p.IsHidden
}
