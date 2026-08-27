package application

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/internal/social/content/entity"
	"github.com/labuda/backend/pkg/money"
)

// ForSalePreview represents a lightweight preview of a commerce resource for comment responses.
// Only contains essential fields for display purposes.
//
// PHASE C — ForSaleRef convergence (third horizontal layer, inline hybrid).
// The legacy fields ID/Title/Price/MediaURLs are preserved for existing consumers.
// The new Currency/Thumbnail/Seller/Status fields make this struct canonical-
// compatible with the ForSaleRef contract without renaming or removing legacy
// fields. See backend/internal/pkg/publiccard/for_sale_card.go for the canonical
// contract (Batch 2C — forSaleref collapsed onto publiccard.ForSaleCard).
//
// Hydration rules in this layer:
//   - Thumbnail mirrors the first element of MediaURLs (set in
//     GetForSalePreviewFromForSale after JSONB unmarshal). Nil when no media.
//   - Status mirrors forSale.Status.String() from the full ForSale entity
//     fetched in the comment handler. Empty string maps to nil.
//   - Currency is nil — the comment surface does not hydrate currency today.
//   - Seller is nil — seller identity is not hydrated in the comment path.
//     (Batch 2B: type collapsed onto *publiccard.UserCard; same JSON shape.)
type ForSalePreview struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	Price     int64     `json:"price"`      // Price in minor currency units (e.g., cents)
	MediaURLs []string  `json:"media_urls"` // Array of media URLs

	// Canonical-compatible additive fields (Phase C, third horizontal layer).
	// Always emitted (no omitempty) so the wire shape is canonical-compatible;
	// null until hydrated.
	Currency  *string              `json:"currency"`
	Thumbnail *string              `json:"thumbnail"`
	Seller    *publiccard.UserCard `json:"seller"`
	Status    *string              `json:"status"`
}

// CommentResponse represents a comment with optional embedded resource preview.
// This is the response format for comment list and detail endpoints.
//
// PUBLIC BOUNDARY (Phase 2A):
//   - `author` is now the canonical CommentAuthorCard (publiccard.UserCard).
//     JSON shape matches the previous authorref.AuthorRef so this is a
//     drop-in replacement; the difference is doctrinal — the card is the
//     canonical exposure type, not an "additive ref".
type CommentResponse struct {
	ID       uuid.UUID `json:"id"`
	TargetID uuid.UUID `json:"target_id"`
	AuthorID uuid.UUID `json:"author_id"`
	// Author info embedded for proper UI rendering (legacy flat fields).
	AuthorUsername  string  `json:"author_username,omitempty"`
	AuthorAvatarURL *string `json:"author_avatar_url,omitempty"`
	// Canonical CommentAuthorCard (Phase 2A PublicCard landing).
	Author    *publiccard.UserCard   `json:"author,omitempty"`
	Body      *string                `json:"body,omitempty"`
	Type      string                 `json:"type"`
	ParentID  *uuid.UUID             `json:"parent_id,omitempty"` // Set for replies
	Reference *entity.ShareReference `json:"reference,omitempty"`
	ForSale   *ForSalePreview        `json:"forSale,omitempty"` // Populated only for commerce-reference comments
	CreatedAt time.Time              `json:"created_at"`
	DeletedAt *time.Time             `json:"deleted_at,omitempty"`
}

// NewCommentResponse creates a comment response from a comment entity.
// For commerce-reference comments, resource data should be provided separately.
// Author info (username, avatar) should be provided for proper UI rendering.
//
// E3.2 — authorLifecycle is the coarsened public user lifecycle for the
// comment author, sourced upstream from users.account_status +
// users.deleted_at via viewercontext.CoarsenLifecycle. Pass an empty
// string when the surface has not hydrated lifecycle truth; the embedded
// UserCard will then carry a nil Lifecycle slot (legacy / rollback-safe
// shape). Non-empty values MUST be one of {"active", "unavailable",
// "removed"}; raw enum strings (e.g. "suspended", "banned") MUST NEVER
// flow into this parameter — coarsening is the caller's responsibility.
func NewCommentResponse(
	comment *entity.Comment,
	forSale *ForSalePreview,
	authorUsername string,
	authorAvatarURL *string,
	authorLifecycle string,
) *CommentResponse {
	// Canonical CommentAuthorCard mirrors the hydrated values that populate
	// the canonical author card.
	//
	// PUBLIC BOUNDARY: DisplayName is always nil on this surface. The
	// upstream query in fetchCommentAuthorsInfo previously COALESCE'd
	// p.full_name (KYC/private data) into the name column, which leaked
	// through DisplayName whenever a user had filled in their legal name.
	// publiccard.NewWithLifecycle does not accept a display_name source —
	// DisplayName stays nil by construction.
	//
	// E3.2 — Lifecycle is now populated from the coarsened public state
	// computed at the comment_handler prosection layer. The wire slot
	// flips from null → {"active" | "unavailable"} for live users.
	// "removed" remains structurally unreachable for now because the
	// comment SQL still filters `WHERE u.deleted_at IS NULL` (a
	// deliberate scope boundary — relaxing that filter is a separate
	// doctrine decision and is out of scope for E3.2).
	authorCard := publiccard.NewWithLifecycle(
		comment.AuthorID,
		authorUsername,
		authorAvatarURL,
		authorLifecycle,
	)
	resp := &CommentResponse{
		ID:              comment.ID,
		TargetID:        comment.TargetID,
		AuthorID:        comment.AuthorID,
		AuthorUsername:  authorUsername,
		AuthorAvatarURL: authorAvatarURL,
		Author:          &authorCard,
		Body:            comment.Body,
		Type:            string(comment.Type),
		ParentID:        comment.ParentID, // Include parent_id for reply threading
		Reference:       comment.Reference,
		CreatedAt:       comment.CreatedAt,
		DeletedAt:       comment.DeletedAt,
	}

	// Only include forSale preview if this is a commerce reference comment
	if comment.IsCommerceReference() && forSale != nil {
		resp.ForSale = forSale
	}

	return resp
}

// GetForSalePreviewFromForSale converts a forSale entity to a lightweight preview.
// This is used when embedding commerce-resource data in comment responses.
//
// status is the string representation of the forSale's lifecycle status
// (e.g. "active", "sold"). Pass an empty string when unavailable; it maps to nil.
func GetForSalePreviewFromForSale(
	forSaleID uuid.UUID,
	title string,
	price money.Money,
	mediaURLs json.RawMessage,
	status string,
) (*ForSalePreview, error) {
	// Parse media URLs from JSONB
	var urls []string
	if mediaURLs != nil {
		if err := json.Unmarshal(mediaURLs, &urls); err != nil {
			return nil, err
		}
	}

	// Canonical additive thumbnail: first element of media_urls, nil when absent.
	var thumbnail *string
	if len(urls) > 0 {
		t := urls[0]
		thumbnail = &t
	}

	// Canonical additive status: nil when unavailable.
	var statusPtr *string
	if status != "" {
		statusPtr = &status
	}

	return &ForSalePreview{
		ID:        forSaleID,
		Title:     title,
		Price:     price.Int64(),
		MediaURLs: urls,
		// Additive ForSaleRef fields:
		Currency:  nil,       // not hydrated on this surface
		Thumbnail: thumbnail, // first of media_urls
		Seller:    nil,       // seller identity not hydrated in comment path
		Status:    statusPtr,
	}, nil
}
