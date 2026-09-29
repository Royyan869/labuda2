package entity

import (
	"fmt"
	"sort"
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

// ============================================================================
// MEDIA POLICY — the canonical rules for attaching media to a message.
//
// Every limit here is enforced SERVER-SIDE. The mobile composer limits
// (MediaUploadConfig.forChat) are a client experience; this is the authority.
// ============================================================================

const (
	// MaxMediaPerMessage is the canonical cap for one message's media array:
	// the product rule is 4 foto + 1 video in one composer (forChat).
	MaxMediaPerMessage = 5

	// MaxChatImageBytes / MaxChatVideoBytes mirror the mobile composer limits
	// (MediaUploadConfig.maxImageSizeMb = 10, maxVideoSizeMb = 100).
	MaxChatImageBytes int64 = 10 << 20 // 10 MB
	MaxChatVideoBytes int64 = 100 << 20 // 100 MB

	// PendingAssetTTL bounds how long an asset may sit between the presigned
	// upload and the message that references it. An asset that is never
	// attached expires and is swept by ChatMediaCleanupWorker — that is what
	// makes "upload then abandon" cost nothing.
	PendingAssetTTL = 24 * time.Hour

	// PermanentAssetTTL is the read lifetime of an ATTACHED asset: attached
	// assets are content referenced by a message, so they outlive any pending
	// window. chat_media_assets.expires_at is NOT NULL (migration 000027), so
	// the attached state carries an explicit far-future value rather than
	// NULL; the cleanup worker only ever sweeps status = 'pending'.
	PermanentAssetTTL = 100 * 365 * 24 * time.Hour
)

// allowedChatMediaContentTypes is the single MIME vocabulary for chat
// uploads. It is deliberately IDENTICAL to mediaupload.allowedContentTypes:
// a type the platform accepts for upload is a type a message may carry.
var allowedChatMediaContentTypes = map[string]ChatMediaAssetType{
	"image/jpeg": ChatMediaAssetTypeImage,
	"image/png":  ChatMediaAssetTypeImage,
	"image/webp": ChatMediaAssetTypeImage,
	"image/gif":  ChatMediaAssetTypeImage,
	"video/mp4":  ChatMediaAssetTypeVideo,
}

// AllowedChatMediaContentTypes returns the accepted MIME types as a stable
// (sorted) list for error messages and contract tests.
func AllowedChatMediaContentTypes() []string {
	types := make([]string, 0, len(allowedChatMediaContentTypes))
	for contentType := range allowedChatMediaContentTypes {
		types = append(types, contentType)
	}
	sort.Strings(types)
	return types
}

// MediaTypeForContentType maps an upload MIME type to its canonical chat
// media type. No sniffing, no fallback: an unknown type is an error.
func MediaTypeForContentType(contentType string) (ChatMediaAssetType, error) {
	contentType = strings.TrimSpace(contentType)
	mediaType, ok := allowedChatMediaContentTypes[contentType]
	if !ok {
		return "", fmt.Errorf("content_type must be one of %s", strings.Join(AllowedChatMediaContentTypes(), ", "))
	}
	return mediaType, nil
}

// Folder is the owned S3 namespace for this media type. Chat media never
// lands outside these two prefixes, so bucket layout mirrors ownership.
func (t ChatMediaAssetType) Folder() string {
	if t == ChatMediaAssetTypeVideo {
		return "videos/chat"
	}
	return "images/chat"
}

// MaxBytes is the per-file byte ceiling for this media type.
func (t ChatMediaAssetType) MaxBytes() int64 {
	if t == ChatMediaAssetTypeVideo {
		return MaxChatVideoBytes
	}
	return MaxChatImageBytes
}

// StorageKeyForRoomMedia mints the canonical room-scoped object key:
//
//	images/chat/{room_id}/{unix_ms}_{uploader_id}{ext}
//
// The room segment is the ownership scope: a chat object is addressable as
// (room, uploader) from the bucket alone, and the sender's id inside the key
// is what the attach path proves belongs to them.
func StorageKeyForRoomMedia(roomID, uploaderID uuid.UUID, mediaType ChatMediaAssetType, contentType string, now time.Time) string {
	return fmt.Sprintf(
		"%s/%s/%d_%s%s",
		mediaType.Folder(),
		roomID.String(),
		now.UnixMilli(),
		uploaderID.String(),
		chatMediaExtension(contentType),
	)
}

// chatMediaExtension maps an allowed MIME type to its object extension. The
// caller has already validated the content type via MediaTypeForContentType.
func chatMediaExtension(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "video/mp4":
		return ".mp4"
	default:
		return ".bin"
	}
}

// MediaAttachRejection is the machine-readable reason a media asset cannot be
// attached to a message. It is a closed vocabulary so the transport layer maps
// ownership problems to FORBIDDEN and lifecycle problems to NOT ATTACHABLE
// without parsing prose.
//
// The zero value ("") means the asset IS attachable.
type MediaAttachRejection string

const (
	// MediaAttachRejectionRoomMismatch: the asset was minted for another room.
	MediaAttachRejectionRoomMismatch MediaAttachRejection = "room_mismatch"
	// MediaAttachRejectionUploaderMismatch: someone else uploaded the asset.
	MediaAttachRejectionUploaderMismatch MediaAttachRejection = "uploader_mismatch"
	// MediaAttachRejectionNotPending: already attached (or swept); a finalized
	// asset belongs to a message and must never be reused.
	MediaAttachRejectionNotPending MediaAttachRejection = "not_pending"
	// MediaAttachRejectionExpired: the upload window closed before the message
	// referencing it was sent.
	MediaAttachRejectionExpired MediaAttachRejection = "expired"
)

// Attachable reports whether this asset may be attached to a message of
// (roomID, uploaderID) at time now. The zero value means "yes".
func (a *ChatMediaAsset) Attachable(roomID, uploaderID uuid.UUID, now time.Time) MediaAttachRejection {
	if a.RoomID != roomID {
		return MediaAttachRejectionRoomMismatch
	}
	if a.UploaderID != uploaderID {
		return MediaAttachRejectionUploaderMismatch
	}
	if a.Status != ChatMediaAssetStatusPending {
		return MediaAttachRejectionNotPending
	}
	if !now.Before(a.ExpiresAt) {
		return MediaAttachRejectionExpired
	}
	return ""
}

// Finalize marks an attached asset as permanent content: status flips
// pending → finalized and the read lifetime extends beyond the pending window.
func (a *ChatMediaAsset) Finalize(now time.Time) {
	a.Status = ChatMediaAssetStatusFinalized
	a.FinalizedAt = &now
	a.ExpiresAt = now.Add(PermanentAssetTTL)
}

// IsExpiredPending reports whether a pending asset is past its upload window
// (the cleanup worker's sweep predicate).
func (a *ChatMediaAsset) IsExpiredPending(now time.Time) bool {
	return a.Status == ChatMediaAssetStatusPending && !now.Before(a.ExpiresAt)
}
