package request

import (
	"fmt"
	"strings"
	"time"

	mediaentity "github.com/labuda/backend/internal/commerce/media/entity"
	productentity "github.com/labuda/backend/internal/commerce/product/entity"
	"github.com/labuda/backend/internal/pkg/mediaref"
	"github.com/labuda/backend/internal/platform/mediaresolve"
)

const (
	ErrCodeAmbiguousPayload    = "COMMERCE_TYPED_MEDIA_CONFLICT"
	ErrCodeInvalidType         = "COMMERCE_TYPED_MEDIA_INVALID_TYPE"
	ErrCodeInvalidURL          = "COMMERCE_TYPED_MEDIA_INVALID_URL"
	ErrCodeInvalidDimensions   = "COMMERCE_TYPED_MEDIA_INVALID_DIMENSIONS"
	ErrCodeInvalidDuration     = "COMMERCE_TYPED_MEDIA_INVALID_DURATION"
	ErrCodeInvalidThumbnailURL = "COMMERCE_TYPED_MEDIA_INVALID_THUMBNAIL_URL"
)

// ValidationError reports a typed-media request failure that should be surfaced
// as a stable HTTP 400 response code.
type ValidationError struct {
	Code    string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// MediaRequest is the canonical public HTTP typed-media item.
type MediaRequest struct {
	Type         string  `json:"type"`
	URL          string  `json:"url"`
	Width        *int    `json:"width,omitempty"`
	Height       *int    `json:"height,omitempty"`
	Duration     *int    `json:"duration,omitempty"` // milliseconds
	ThumbnailURL *string `json:"thumbnail_url,omitempty"`
	Blurhash     *string `json:"blurhash,omitempty"`
}

// NormalizeSelection validates and canonicalizes a typed-media selection.
// Typed media preserves caller order exactly; position 0 is canonical cover
// media, regardless of image/video type.
//
// Legacy media_urls are still accepted for compatibility, but they are always
// converted into typed image rows.
func NormalizeSelection(media []MediaRequest, mediaURLs []string) ([]mediaentity.Media, error) {
	hasTyped := media != nil
	hasLegacy := mediaURLs != nil

	if hasTyped && hasLegacy && len(media) > 0 && len(mediaURLs) > 0 {
		return nil, &ValidationError{
			Code:    ErrCodeAmbiguousPayload,
			Message: "media and media_urls cannot both be non-empty",
		}
	}

	if hasTyped && len(media) > 0 {
		return normalizeTyped(media)
	}

	if hasLegacy && len(mediaURLs) > 0 {
		return normalizeLegacy(mediaURLs)
	}

	if hasTyped || hasLegacy {
		return []mediaentity.Media{}, nil
	}

	return nil, nil
}

func normalizeTyped(media []MediaRequest) ([]mediaentity.Media, error) {
	now := time.Now().UTC()
	out := make([]mediaentity.Media, 0, len(media))

	for i, item := range media {
		normalized, err := normalizeTypedItem(item, now)
		if err != nil {
			return nil, fmt.Errorf("media[%d]: %w", i, err)
		}
		// Preserve client order exactly. Position 0 is canonical cover media;
		// media type never outranks the owner's explicit arrangement.
		normalized.Position = i
		out = append(out, *normalized)
	}
	return out, nil
}

func normalizeLegacy(mediaURLs []string) ([]mediaentity.Media, error) {
	now := time.Now().UTC()
	normalized := make([]string, 0, len(mediaURLs))
	for i, raw := range mediaURLs {
		ref, err := normalizeReference(raw)
		if err != nil {
			return nil, fmt.Errorf("media_urls[%d]: %w", i, err)
		}
		normalized = append(normalized, ref)
	}

	typed, err := mediaentity.NewLegacyImageListFromReferences(normalized, now)
	if err != nil {
		return nil, err
	}
	return typed, nil
}

func normalizeTypedItem(item MediaRequest, createdAt time.Time) (*mediaentity.Media, error) {
	mediaType := mediaentity.MediaType(strings.ToLower(strings.TrimSpace(item.Type)))
	if !mediaType.IsValid() {
		return nil, &ValidationError{
			Code:    ErrCodeInvalidType,
			Message: "invalid media type: must be image or video",
		}
	}

	reference, err := normalizeReference(item.URL)
	if err != nil {
		return nil, err
	}

	if item.Width != nil && *item.Width <= 0 {
		return nil, &ValidationError{
			Code:    ErrCodeInvalidDimensions,
			Message: "width and height must be positive when provided",
		}
	}
	if item.Height != nil && *item.Height <= 0 {
		return nil, &ValidationError{
			Code:    ErrCodeInvalidDimensions,
			Message: "width and height must be positive when provided",
		}
	}

	if item.Duration != nil {
		if *item.Duration < 0 {
			return nil, &ValidationError{
				Code:    ErrCodeInvalidDuration,
				Message: "duration must be non-negative in milliseconds when provided",
			}
		}
		if mediaType == mediaentity.MediaTypeVideo && *item.Duration == 0 {
			return nil, &ValidationError{
				Code:    ErrCodeInvalidDuration,
				Message: "video duration must be positive in milliseconds when provided",
			}
		}
	}

	if item.ThumbnailURL != nil {
		if _, err := normalizeReference(*item.ThumbnailURL); err != nil {
			return nil, &ValidationError{
				Code:    ErrCodeInvalidThumbnailURL,
				Message: "thumbnail_url must be a non-blank media reference when provided",
			}
		}
	}

	media, err := mediaentity.NewMedia(reference, mediaType, 0, createdAt)
	if err != nil {
		return nil, &ValidationError{
			Code:    ErrCodeInvalidURL,
			Message: err.Error(),
		}
	}
	media.Width = cloneInt(item.Width)
	media.Height = cloneInt(item.Height)
	media.Duration = cloneInt(item.Duration)
	media.ThumbnailURL = cloneString(item.ThumbnailURL)
	media.Blurhash = cloneString(item.Blurhash)
	return media, nil
}

func normalizeReference(reference string) (string, error) {
	trimmed := strings.TrimSpace(reference)
	if trimmed == "" {
		return "", &ValidationError{
			Code:    ErrCodeInvalidURL,
			Message: "media url is required",
		}
	}

	normalized, err := mediaresolve.NormalizeStorageReference(trimmed)
	if err != nil {
		return "", &ValidationError{
			Code:    ErrCodeInvalidURL,
			Message: "invalid media url",
		}
	}
	return normalized, nil
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

// ResolveProductMedia is the single entry for write-side product media:
// typed `media[]` (validated, blurhash-carrying) XOR legacy `media_urls`
// (bare strings). Both non-empty is a 400 conflict. Order is preserved
// exactly as sent — position 0 is canonical cover media.
func ResolveProductMedia(media []MediaRequest, mediaURLs []string) ([]productentity.ProductMedia, *ValidationError) {
	items, err := NormalizeSelection(media, mediaURLs)
	if err != nil {
		if verr, ok := err.(*ValidationError); ok {
			return nil, verr
		}
		return nil, &ValidationError{Code: ErrCodeInvalidURL, Message: err.Error()}
	}
	if items == nil {
		return nil, nil
	}
	return ToProductMedia(items), nil
}

// ToProductMedia converts normalized items into ordered product media slots,
// preserving URL order plus the client-provisional video metadata
// (thumbnail, dimensions, duration) and blurhash. Nothing is derived here —
// what the client sent (validated) is what persists. Position authority
// stays the slice.
func ToProductMedia(items []mediaentity.Media) []productentity.ProductMedia {
	out := make([]productentity.ProductMedia, 0, len(items))
	for _, item := range items {
		url := strings.TrimSpace(item.URL)
		if url == "" {
			continue
		}
		out = append(out, productentity.ProductMedia{
			URL:          url,
			Blurhash:     item.Blurhash,
			ThumbnailURL: item.ThumbnailURL,
			Width:        item.Width,
			Height:       item.Height,
			DurationMs:   item.Duration,
			Status:       string(mediaref.StatusForNewRow(item.Type.String())),
		})
	}
	return out
}

// ProductMediaFromLegacy converts legacy URL strings into product media
// slots without hashes.
func ProductMediaFromLegacy(mediaURLs []string) []productentity.ProductMedia {
	out := make([]productentity.ProductMedia, 0, len(mediaURLs))
	for _, raw := range mediaURLs {
		if trimmed := strings.TrimSpace(raw); trimmed != "" {
			out = append(out, productentity.ProductMedia{URL: trimmed})
		}
	}
	return out
}
