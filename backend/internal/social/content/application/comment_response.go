package application

import (
	"time"

	"github.com/google/uuid"
	commerceshared "github.com/labuda/backend/internal/commerce/shared"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/internal/social/content/entity"
)

// CommentMediaResponse represents a foto+video attachment on a comment.
type CommentMediaResponse struct {
	ID         uuid.UUID `json:"id"`
	StorageKey string    `json:"storage_key"`
	MediaURL   string    `json:"media_url"`
	MediaType  string    `json:"media_type"`
	Position   int       `json:"position"`
}

// CommentResponse represents a comment with its optional canonical resource
// projection (commerce-reference comments only).
// This is the response format for comment list and detail endpoints.
//
// PUBLIC BOUNDARY (Phase 2A):
//   - `author` is the canonical CommentAuthorCard (publiccard.UserCard).
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
	// ResourceProjection is the viewer-aware envelope for a commerce-reference
	// comment (for_sale / auction), resolved by the canonical projection
	// authority — LIVE payload or TOMBSTONE. The legacy `forSale` snapshot
	// preview has no consumer and is deleted; comments answer exactly like
	// chat, content detail, feed and search.
	ResourceProjection *commerceshared.ResourceProjection `json:"resource_projection,omitempty"`
	Media              []CommentMediaResponse             `json:"media,omitempty"` // foto+video attachments (max 5)
	CreatedAt          time.Time                          `json:"created_at"`
	DeletedAt          *time.Time                         `json:"deleted_at,omitempty"`
}

// NewCommentResponse creates a comment response from a comment entity.
// For commerce-reference comments, the viewer-aware resource projection should
// be provided separately by the caller (canonical projection authority).
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
	projection *commerceshared.ResourceProjection,
	authorUsername string,
	authorAvatarURL *string,
	authorLifecycle string,
) *CommentResponse {
	return NewCommentResponseWithMedia(comment, projection, nil, authorUsername, authorAvatarURL, authorLifecycle)
}

// NewCommentResponseWithMedia is the foto+video-aware variant.
func NewCommentResponseWithMedia(
	comment *entity.Comment,
	projection *commerceshared.ResourceProjection,
	media []*entity.CommentMedia,
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

	// Commerce-reference comments carry the canonical viewer-aware envelope
	// (LIVE or TOMBSTONE) instead of a snapshot preview.
	if comment.IsCommerceReference() && projection != nil {
		resp.ResourceProjection = projection
	}

	if len(media) > 0 {
		resp.Media = make([]CommentMediaResponse, 0, len(media))
		for _, m := range media {
			resp.Media = append(resp.Media, CommentMediaResponse{
				ID:         m.ID,
				StorageKey: m.StorageKey,
				MediaURL:   m.MediaURL,
				MediaType:  string(m.MediaType),
				Position:   m.Position,
			})
		}
	}

	return resp
}
