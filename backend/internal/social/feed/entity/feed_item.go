// DOMAIN: SOCIAL
// NOTE: Aggregated content feed for timeline display

package entity

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// FeedMedia represents a media attachment for feed items.
// Source of truth: canonical Content.media (List<MediaEntity>)
//
// PHASE C — MediaRef convergence (Option D, transitional hybrid).
// The legacy fields URL/Type/Position are preserved for mobile
// parser compatibility (FeedMediaDto in apps/mobile/lib/features/
// home/data/dto/feed_dto.dart binds them as required non-null).
// The new Kind/Width/Height fields make this struct serializable
// as a canonical-compatible MediaRef without renaming or removing
// legacy fields. See backend/internal/pkg/mediaref/mediaref.go for
// the canonical contract.
//
// Hydration rules in this layer:
//   - Kind mirrors Type (set in feed_repository_impl after JSON
//     unmarshal of the per-row aggregate). Never nil in practice
//     once hydrated; nil only if the entity is constructed without
//     going through repository hydration.
//   - Width / Height are always nil in this layer — no DB column,
//     no inference, no remote fetch.
type FeedMedia struct {
	URL      string `json:"url"`
	Type     string `json:"type"` // "image" or "video"
	Position int    `json:"position"`

	// Canonical-compatible additive fields (Phase C). Always
	// emitted (no omitempty) so the wire shape is canonical-
	// compatible; null until hydrated.
	Kind   *string `json:"kind"`
	Width  *int    `json:"width"`
	Height *int    `json:"height"`
}

// FeedItem represents a single content item in the feed.
//
// SCHEMA ALIGNMENT (Batch 3J): the canonical contents table has no
// `body` / `title` columns — only `caption`. We keep the `Body`
// field on this projection entity because the public API contract
// still carries `body` (mobile requires it); the field is populated
// from `c.caption` via SQL alias in the repository layer. The
// legacy `Title` field has been removed — it was scanned from a
// nonexistent column and never made it into any live response.
type FeedItem struct {
	ID       uuid.UUID
	AuthorID uuid.UUID
	Type     string
	// Status is always "active" for feed items
	Status    string
	Body      string  // sourced from contents.caption via SQL alias
	Caption   *string // canonical text column on contents
	City      *string // POST LOCATION: City name for posts only
	Province  *string // POST LOCATION: Province name for posts only
	IsHidden  bool
	CreatedAt time.Time
	UpdatedAt time.Time

	// Author information (optional, for display)
	AuthorUsername *string
	AuthorAvatar   *string
	AuthorCity     *string // Author's city from user_profiles
	AuthorProvince *string // Author's province from user_profiles

	// AuthorLifecycle is the coarsened public user lifecycle for the
	// content author, sourced from users.account_status + users.deleted_at
	// at projection time and coarsened via
	// viewercontext.CoarsenLifecycle (Go-side; never via SQL).
	//
	// Empty string means lifecycle was not hydrated (legacy fallback /
	// rollback safety). Non-empty values are constrained to the canonical
	// public lifecycle vocabulary: "active", "unavailable", "removed".
	//
	// Per E2 doctrine (feed identity transport activation), this field is
	// the SOLE feed-surface carrier for the embedded UserCard.Lifecycle
	// public emission. No other surface is wired through this seam yet.
	AuthorLifecycle string

	// SHARE CONTRACT V1: Repost attribution fields
	// If non-nil, this item is a repost/shares of another content
	OriginalAuthorID *uuid.UUID

	// MEDIA INTEGRATION: Media from canonical Content.media
	// Contract: Must be sourced from content_media table
	// NO hardcoded empty arrays, NO dummy data
	Media []FeedMedia `json:"media"`
}

// FeedResult contains the feed items and pagination cursor.
//
// NextCursor is the opaque, base64-encoded continuation cursor of the
// last returned row; nil when HasMore is false. HasMore is derived
// from a LIMIT+1 probe in the repository, not from len(Items).
type FeedResult struct {
	Items      []*FeedItem
	NextCursor *FeedCursor
	HasMore    bool
}

// FeedCursor is the opaque pagination cursor for the feed surface.
//
// It identifies a row in the canonical (created_at DESC, id DESC)
// ordering by the (created_at, id) tuple of the boundary row that has
// already been returned to the client. The cursor is treated as opaque
// at the HTTP boundary — clients must not parse or interpret it, and
// the wire format may change without notice.
//
// Use [EncodeFeedCursor] / [DecodeFeedCursor] for the canonical string
// form crossed at the HTTP boundary.
type FeedCursor struct {
	PriorityGroup int       `json:"pg"`
	CreatedAt     time.Time `json:"ts"`
	ID            uuid.UUID `json:"id"`
}

// EncodeFeedCursor returns the canonical opaque string form of c.
//
// The encoding is URL-safe base64 (no padding) of the JSON payload
// `{"ts":"<RFC3339Nano>","id":"<uuid>"}`. Returns "" for nil so the
// caller can decide between omitting the field or emitting an empty
// string; the handler emits the encoded value only when HasMore=true.
func EncodeFeedCursor(c *FeedCursor) string {
	if c == nil {
		return ""
	}
	payload, err := json.Marshal(c)
	if err != nil {
		// json.Marshal cannot fail for a struct with these field types,
		// but defend against future shape drift by returning empty —
		// the handler treats "" as "no cursor available".
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(payload)
}

// DecodeFeedCursor parses an opaque cursor string back into a
// FeedCursor.
//
// An empty input returns (nil, nil) — the canonical "no cursor" form
// used for the first page. A malformed cursor returns a non-nil error
// so the caller (HTTP handler) can map it to 400.
//
// BACKWARD COMPATIBILITY: the prior cursor format was an RFC3339Nano
// timestamp string. There is no production data on this surface yet
// (clean-break doctrine, MEMORY: feedback_from_zero_no_backcompat), so
// the legacy format is intentionally rejected. Mobile builds carrying
// an old cursor in memory will receive 400 once on the first page-2
// request after upgrade and recover via the standard refresh path.
func DecodeFeedCursor(s string) (*FeedCursor, error) {
	if s == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("decode feed cursor: %w", err)
	}
	var c FeedCursor
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("decode feed cursor: %w", err)
	}
	if c.CreatedAt.IsZero() {
		return nil, fmt.Errorf("decode feed cursor: missing ts")
	}
	if c.ID == uuid.Nil {
		return nil, fmt.Errorf("decode feed cursor: missing id")
	}
	return &c, nil
}
