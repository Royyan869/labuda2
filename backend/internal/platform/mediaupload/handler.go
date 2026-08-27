// Package mediaupload provides the general-purpose presigned S3 upload-URL
// endpoint for non-KYC media (avatars, content images, product photos, videos).
//
// KYC documents use a separate endpoint in the governance/verification domain
// that applies stricter validation (doc-type allow-list, kyc/ key prefix,
// shorter TTL, private bucket).
package mediaupload

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/internal/platform/s3presign"
)

// MediaUploadTTL is the lifetime of a general presigned PUT URL.
const MediaUploadTTL = 15 * time.Minute

// allowedContentTypes maps supported MIME types to their file extensions.
var allowedContentTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
	"video/mp4":  ".mp4",
}

// allowedFolders is the set of top-level S3 key prefixes the mobile client
// may request. Prevents path traversal via unexpected folder values.
var allowedFolders = map[string]bool{
	"images": true,
	"videos": true,
}

// UploadURLRequest is the request body for POST /api/v1/media/upload-url.
type UploadURLRequest struct {
	// ContentType is the MIME type of the file being uploaded.
	ContentType string `json:"content_type" binding:"required"`
	// Folder is the top-level S3 key prefix: "images" or "videos".
	Folder string `json:"folder" binding:"required"`
	// StorageKey, when provided, must be a canonical owned fixed key
	// (images/avatars/{user_id}.jpg|.png, images/stores/{user_id}.jpg,
	// images/profile-covers/{user_id}.jpg, or an owned commerce media key).
	// When omitted, a timestamped key is minted server-side.
	StorageKey string `json:"storage_key"`
}

// UploadURLResponse is returned by the upload-url endpoint.
type UploadURLResponse struct {
	// StorageKey is the S3 object key. Store this in the DB.
	StorageKey string `json:"storage_key"`
	// UploadURL is the presigned PUT URL. PUT the file bytes here.
	UploadURL string `json:"upload_url"`
	// ExpiresAt is the presigned URL expiry.
	ExpiresAt time.Time `json:"expires_at"`
	// PublicURL is the CDN or S3 URL for display after a successful upload.
	PublicURL string `json:"public_url"`
	// ReadURL is the canonical read URL (CDN when configured, else raw S3).
	ReadURL string `json:"read_url"`
}

// Handler handles the general media upload-url endpoint.
type Handler struct {
	presignCfg s3presign.Config
	cdnBase    string // optional CloudFront prefix; empty → raw S3 URL
	log        *zap.Logger
}

// NewHandler creates a media upload handler.
func NewHandler(cfg s3presign.Config, cdnBaseURL string, log *zap.Logger) *Handler {
	if log == nil {
		log = zap.NewNop()
	}
	return &Handler{
		presignCfg: cfg,
		cdnBase:    strings.TrimRight(cdnBaseURL, "/"),
		log:        log,
	}
}

// RequestUploadURL handles POST /api/v1/media/upload-url.
//
// Returns a presigned PUT URL the client can use to upload media directly to
// S3. AWS credentials never leave the server.
//
// Returns:
//   - 200: {storage_key, upload_url, expires_at, public_url}
//   - 400: invalid content_type or folder
//   - 503: AWS not configured
func (h *Handler) RequestUploadURL(c *gin.Context) {
	userID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	if h.presignCfg.AccessKey == "" {
		h.log.Error("media upload-url: AWS not configured")
		response.Error(c, 503, "UPLOAD_NOT_CONFIGURED", "Media upload service not configured")
		return
	}

	var req UploadURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	if _, ok := allowedContentTypes[req.ContentType]; !ok {
		response.Error(c, 400, "INVALID_CONTENT_TYPE",
			"content_type must be image/jpeg, image/png, image/webp, image/gif, or video/mp4")
		return
	}

	if !allowedFolders[req.Folder] {
		response.Error(c, 400, "INVALID_FOLDER", "folder must be 'images' or 'videos'")
		return
	}

	storageKey := req.StorageKey
	if storageKey != "" {
		// Fixed-key upload: validate ownership against the canonical allowlist
		// so a caller can never claim another user's avatar/store/cover key.
		if err := validateFixedStorageKey(storageKey, req.ContentType, userID); err != nil {
			response.Error(c, 400, "INVALID_STORAGE_KEY", err.Error())
			return
		}
	} else {
		ts := time.Now().UnixMilli()
		ext := allowedContentTypes[req.ContentType]
		storageKey = fmt.Sprintf("%s/%d_%s%s", req.Folder, ts, userID, ext)
	}

	uploadURL, err := s3presign.PresignPUT(h.presignCfg, storageKey, req.ContentType, MediaUploadTTL)
	if err != nil {
		h.log.Error("media upload-url: presign failed",
			zap.String("user_id", userID.String()), zap.Error(err))
		response.InternalServerError(c, "Failed to generate upload URL")
		return
	}

	publicURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s",
		h.presignCfg.Bucket, h.presignCfg.Region, storageKey)
	if h.cdnBase != "" {
		publicURL = h.cdnBase + "/" + storageKey
	}

	response.Success(c, UploadURLResponse{
		StorageKey: storageKey,
		UploadURL:  uploadURL,
		ExpiresAt:  time.Now().Add(MediaUploadTTL),
		PublicURL:  publicURL,
		ReadURL:    publicURL,
	})
}

// canonicalFixedKeyMessage is the shared rejection message pinned by the
// mediaupload contract tests.
const canonicalFixedKeyMessage = "storage_key must match images/avatars/{user_id}.jpg or .png, images/stores/{user_id}.jpg, images/profile-covers/{user_id}.jpg, or an owned commerce media key"

// validateFixedStorageKey enforces the canonical fixed-key contract:
//   - images/avatars/{user_id}.jpg|.png   — jpeg or png, caller-owned
//   - images/stores/{user_id}.jpg         — jpeg only, caller-owned
//   - images/profile-covers/{user_id}.jpg — jpeg only, caller-owned
//   - owned commerce media key            — videos/{ts}_{user_id}.(mp4|ext) with
//     optional _poster.jpg suffix, caller-owned
//
// Any other key (cross-user, legacy images/covers/, traversal, wrong prefix,
// wrong MIME for a fixed image) is rejected.
func validateFixedStorageKey(key, contentType string, userID uuid.UUID) error {
	id := userID.String()

	switch {
	case strings.HasPrefix(key, "images/avatars/") && (strings.HasSuffix(key, ".jpg") || strings.HasSuffix(key, ".png")):
		if key != "images/avatars/"+id+".jpg" && key != "images/avatars/"+id+".png" {
			return fmt.Errorf("%s", canonicalFixedKeyMessage)
		}
		if contentType != "image/jpeg" && contentType != "image/png" {
			return fmt.Errorf("fixed image uploads must use image/jpeg")
		}
		return nil

	case strings.HasPrefix(key, "images/stores/") && strings.HasSuffix(key, ".jpg"):
		if key != "images/stores/"+id+".jpg" {
			return fmt.Errorf("%s", canonicalFixedKeyMessage)
		}
		if contentType != "image/jpeg" {
			return fmt.Errorf("fixed image uploads must use image/jpeg")
		}
		return nil

	case strings.HasPrefix(key, "images/profile-covers/") && strings.HasSuffix(key, ".jpg"):
		if key != "images/profile-covers/"+id+".jpg" {
			return fmt.Errorf("%s", canonicalFixedKeyMessage)
		}
		if contentType != "image/jpeg" {
			return fmt.Errorf("fixed image uploads must use image/jpeg")
		}
		return nil

	case isOwnedCommerceMediaKey(key, id):
		return nil

	default:
		return fmt.Errorf("%s", canonicalFixedKeyMessage)
	}
}

// isOwnedCommerceMediaKey matches owned commerce media keys minted by the
// generic upload path: {folder}/{ts}_{userID}{ext} with an optional
// _poster.jpg suffix (video poster frame).
func isOwnedCommerceMediaKey(key, userID string) bool {
	base := key
	if strings.HasSuffix(base, "_poster.jpg") {
		base = strings.TrimSuffix(base, "_poster.jpg")
	}
	// {folder}/{millis}_{userID}{ext}
	idx := strings.LastIndex(base, "_")
	if idx <= 0 {
		return false
	}
	prefix := base[:idx]
	suffix := base[idx+1:]
	if !strings.HasPrefix(suffix, userID) {
		return false
	}
	if !strings.Contains(prefix, "/") {
		return false
	}
	// Extension must be one of the allowed media extensions.
	for _, ext := range []string{".jpg", ".png", ".webp", ".gif", ".mp4"} {
		if strings.HasSuffix(suffix, ext) {
			return true
		}
	}
	return false
}


