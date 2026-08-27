package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/evaluator"
	"github.com/labuda/backend/internal/governance/viewercontext"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/internal/platform/response"
	contentApp "github.com/labuda/backend/internal/social/content/application"
	"github.com/labuda/backend/internal/social/content/delivery/http/dto"
	"github.com/labuda/backend/internal/social/content/entity"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// buildContentAuthorCardWithLifecycle hydrates a lifecycle-aware author
// UserCard for the content_handler emit sites (CreateContent, UpdateContent,
// GetContent, RepostContent).
//
// E6 — bounded content-detail-only activation. Mirrors the E4.2 chat-handler
// recipe: single SQL projection that reads username + avatar_url +
// account_status + deleted_at, coarsen via viewercontext.CoarsenLifecycle,
// emit through publiccard.NewWithLifecycle. The slot-persistence carve-out
// from chat (no `u.deleted_at IS NULL` filter) is preserved here so a
// hard-deleted author surfaces Lifecycle="removed" — the canonical tombstone
// — rather than falling through to the anonymous-safe nil-lifecycle path
// that the legacy publiccard.BuildOne returned.
//
// Returns publiccard.Anonymous(id) with Lifecycle nil when no row matches
// (id never existed). Errors propagate so the call sites can apply their
// existing degradation strategy (silent fall-through to a zero-value
// UserCard, identical to pre-E6 wire shape).
//
// PUBLIC BOUNDARY: Raw account_status enum strings never leave this
// function. Only the coarsened public lifecycle vocabulary {active,
// unavailable, removed} crosses into publiccard.NewWithLifecycle.
func (h *ContentHandler) buildContentAuthorCardWithLifecycle(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (publiccard.UserCard, error) {
	if id == uuid.Nil {
		return publiccard.UserCard{}, nil
	}
	const query = `
		SELECT
			u.id,
			COALESCE(p.username, '') AS username,
			p.avatar_url,
			u.account_status,
			(u.deleted_at IS NOT NULL) AS is_deleted
		FROM users u
		LEFT JOIN user_profiles p ON p.user_id = u.id
		WHERE u.id = $1
	`
	rows, err := tx.Query(ctx, query, id)
	if err != nil {
		return publiccard.UserCard{}, fmt.Errorf("content author lifecycle hydration: query failed: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		var (
			userID        uuid.UUID
			username      string
			avatarURL     *string
			accountStatus string
			isDeleted     bool
		)
		if err := rows.Scan(&userID, &username, &avatarURL, &accountStatus, &isDeleted); err != nil {
			return publiccard.UserCard{}, fmt.Errorf("content author lifecycle hydration: scan failed: %w", err)
		}
		return contentAuthorCardFromRow(userID, username, avatarURL, accountStatus, isDeleted), nil
	}
	if err := rows.Err(); err != nil {
		return publiccard.UserCard{}, fmt.Errorf("content author lifecycle hydration: rows iteration failed: %w", err)
	}
	// No row — author id never matched users; return anonymous-safe card
	// with Lifecycle nil (rollback-safe; no truth available).
	return publiccard.Anonymous(id), nil
}

// contentAuthorCardFromRow is the pure, DB-free per-row builder. Extracted
// for unit testability so the lifecycle coarsening rule + the wire shape can
// be exercised without a database. Mirrors chatParticipantCardFromRow (E4.2).
func contentAuthorCardFromRow(
	id uuid.UUID,
	username string,
	avatarURL *string,
	accountStatus string,
	isDeleted bool,
) publiccard.UserCard {
	var avatar *string
	if avatarURL != nil && *avatarURL != "" {
		v := *avatarURL
		avatar = &v
	}
	lifecycle := string(viewercontext.CoarsenLifecycle(accountStatus, isDeleted))
	return publiccard.NewWithLifecycle(id, username, avatar, lifecycle)
}

// checkBidirectionalBlock reports whether a block exists in either direction
// between viewerID and targetID. Queries user_blocks directly, following the
// same inline-SQL pattern as buildContentAuthorCardWithLifecycle. Returns
// false (no block) when viewerID is uuid.Nil (anonymous viewer).
func (h *ContentHandler) checkBidirectionalBlock(
	ctx context.Context,
	tx db.Tx,
	viewerID uuid.UUID,
	targetID uuid.UUID,
) (bool, error) {
	if viewerID == uuid.Nil {
		return false, nil
	}
	const query = `
		SELECT EXISTS(
			SELECT 1 FROM user_blocks
			WHERE (blocker_id = $1 AND blocked_id = $2)
			   OR (blocker_id = $2 AND blocked_id = $1)
		)
	`
	rows, err := tx.Query(ctx, query, viewerID, targetID)
	if err != nil {
		return false, fmt.Errorf("block check query failed: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		var blocked bool
		if err := rows.Scan(&blocked); err != nil {
			return false, fmt.Errorf("block check scan failed: %w", err)
		}
		return blocked, nil
	}
	return false, rows.Err()
}

// derefOrEmpty safely dereferences a string pointer, returning empty string if nil.
// Used to prevent empty string leaks in location responses.
func derefOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// hasNonEmptyValue returns true if the pointer is non-nil and points to a non-empty string.
// Used to guard against empty string leaks in conditional logic.
func hasNonEmptyValue(s *string) bool {
	return s != nil && *s != ""
}

// ContentHandler handles HTTP requests for content operations.
type ContentHandler struct {
	contentService *contentApp.ContentService
	roleChecker    auth.RoleChecker
	db             *db.DB
	log            *zap.Logger
	// contentDetailShadowRunner is the optional /contents/:id shadow
	// evaluator seam (Batch 3Q). When non-nil the GetContent handler
	// dispatches a fire-and-forget shadow run AFTER the response is
	// written. nil disables shadow telemetry; the runner is never on
	// the request-critical path.
	contentDetailShadowRunner *evaluator.ContentDetailShadowRunner
}

// NewContentHandler creates a new ContentHandler.
//
// contentDetailShadowRunner is optional. When non-nil, GetContent
// dispatches a /contents/:id shadow evaluator run post-response per
// docs/contracts/content-detail-visibility-doctrine.md §8. The shadow
// path is observability-only and never alters the response.
func NewContentHandler(
	contentService *contentApp.ContentService,
	roleChecker auth.RoleChecker,
	db *db.DB,
	log *zap.Logger,
	contentDetailShadowRunner *evaluator.ContentDetailShadowRunner,
) *ContentHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &ContentHandler{
		contentService:            contentService,
		roleChecker:               roleChecker,
		db:                        db,
		log:                       log,
		contentDetailShadowRunner: contentDetailShadowRunner,
	}
}

// CreateContentRequest holds the request body for creating content.
// This aligns with the Flutter CreateContent screen DTO.
type CreateContentRequest struct {
	Caption            string                         `json:"caption" binding:"required,min=1,max=5000"`
	Visibility         string                         `json:"visibility"` // Optional: public, followers_only, private
	AllowComments      bool                           `json:"allow_comments"`
	Media              []MediaInput                   `json:"media"`
	Tags               []string                       `json:"tags"`
	MentionedUserIDs   []string                       `json:"mentioned_user_ids"`
	Location           *LocationInput                 `json:"location"`
	ResourceOccurrence *dto.ResourceOccurrenceRequest `json:"resource_occurrence"`
}

// MediaInput represents a media attachment in the request.
type MediaInput struct {
	URL  string `json:"url" binding:"required"`
	Type string `json:"type" binding:"required,oneof=image video"`
}

// LocationInput represents location data in the request.
type LocationInput struct {
	City     string `json:"city"`
	Province string `json:"province"`
}

// ContentResponse represents the response DTO for content.
// This aligns with the Flutter content model.
//
// PUBLIC BOUNDARY:
//   - `status` is the COARSENED public lifecycle ("active" / "removed").
//     The raw internal enum (active/deleted) is never emitted — see
//     entity.Status.PublicLifecycle.
//   - `is_hidden` is intentionally absent. Hidden/moderated items are filtered
//     by the handler before serialization; the flag itself never crosses the
//     boundary. Visibility coarsens into the `status` lifecycle.
type ContentResponse struct {
	ID         uuid.UUID         `json:"id"`
	AuthorID   uuid.UUID         `json:"author_id"`
	Caption    string            `json:"caption"`
	Media      []MediaResponse   `json:"media"`
	Tags       []string          `json:"tags,omitempty"`
	Location   *LocationResponse `json:"location,omitempty"`
	Visibility string            `json:"visibility,omitempty"`
	Status     string            `json:"status"` // coarsened: active|unavailable|removed
	// D1 — top-level canonical governance lifecycle field, identical to
	// card.lifecycle. Mirror of the feed handler's top-level `lifecycle`
	// key (feed_handler.go). The mobile content DTO reads this directly
	// via the new optional `lifecycle` field added in D1. Vocabulary:
	// {active, unavailable, removed}. The pre-existing `status` field
	// already carries the same coarsened value; `lifecycle` is the
	// canonical convergence-aligned name.
	Lifecycle string `json:"lifecycle,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`

	// SHARE CONTRACT V1: Repost attribution fields
	OriginalAuthorID   *uuid.UUID                            `json:"original_author_id,omitempty"`
	ResourceProjection *contentApp.ContentResourceProjection `json:"resource_projection,omitempty"`

	// C7C — Engagement stats for content detail responses.
	// Always non-nil on content detail/create/update responses.
	// Populated with available data (like count); zeros for untracked metrics.
	Engagement *EngagementResponse `json:"engagement,omitempty"`

	// C7C — Per-viewer engagement flags (authenticated callers only).
	IsLiked *bool `json:"is_liked,omitempty"`
	IsSaved *bool `json:"is_saved,omitempty"`

	// PUBLICCARD BATCH 2D: Canonical ContentCard exposure (additive).
	// New clients SHOULD consume the nested card, which is the canonical
	// PublicCard seam shared with feed / search.
	Card *publiccard.ContentCard `json:"card,omitempty"`
}

// EngagementResponse holds engagement metrics for a content item.
// C7C: Fields use camelCase JSON keys to match mobile ContentEngagementDto.
type EngagementResponse struct {
	ViewCount    int `json:"viewCount"`
	LikeCount    int `json:"likeCount"`
	CommentCount int `json:"commentCount"`
	ShareCount   int `json:"shareCount"`
	SaveCount    int `json:"saveCount"`
	ReportCount  int `json:"reportCount"`
}

// UserContentListResponse is the paginated envelope for GET /users/:id/contents.
// Cursor is opaque to callers; next_cursor is null on the terminal page.
type UserContentListResponse struct {
	Data       []ContentResponse `json:"data"`
	NextCursor *string           `json:"next_cursor"`
	HasMore    bool              `json:"has_more"`
}

// MediaResponse represents a media attachment in the response.
type MediaResponse struct {
	ID       uuid.UUID `json:"id"`
	URL      string    `json:"url"`
	Type     string    `json:"type"`
	Position int       `json:"position"`
}

// LocationResponse represents location data in the response.
type LocationResponse struct {
	City     string `json:"city"`
	Province string `json:"province"`
}

// ToContentResponse converts an entity.Content to ContentResponse.
//
// PUBLIC BOUNDARY (Batch 2D):
//   - The legacy flat fields (`author_id`, `status`, …) are retained for
//     mobile compat.
//   - The canonical PublicCard `card` block is populated only when the
//     caller supplies a hydrated author UserCard via
//     ToContentResponseWithAuthor (the variant below). Callers without
//     transaction access fall through to a card-less response — the
//     wire shape stays valid because `card` is omitempty.
func ToContentResponse(content *entity.Content, media []*entity.ContentMedia) ContentResponse {
	// Derive caption from Caption field
	caption := ""
	if content.Caption != nil {
		caption = *content.Caption
	}

	// Derive visibility from is_hidden status
	visibility := string(content.Visibility.Normalize())
	if content.Visibility == "" {
		if content.IsHidden {
			visibility = "private"
		} else {
			visibility = "public"
		}
	}

	publicLifecycle := content.Status.PublicLifecycle()
	resp := ContentResponse{
		ID:         content.ID,
		AuthorID:   content.AuthorID,
		Caption:    caption,
		Media:      make([]MediaResponse, len(media)),
		Tags:       content.Tags, // populated from content_hashtags via GetByID
		Visibility: visibility,
		Status:     publicLifecycle, // coarsened: active|unavailable|removed
		// D1 — canonical lifecycle field, identical to status today.
		// Mirrors feed_handler.go's top-level `lifecycle` emission so
		// mobile DTOs can consume the same wire shape across surfaces.
		Lifecycle: publicLifecycle,
		CreatedAt: content.CreatedAt.Format(time.RFC3339),
		UpdatedAt: content.UpdatedAt.Format(time.RFC3339),
	}

	// Location is included when present.
	hasCity := hasNonEmptyValue(content.City)
	hasProvince := hasNonEmptyValue(content.Province)

	if hasCity || hasProvince {
		resp.Location = &LocationResponse{
			City:     derefOrEmpty(content.City),
			Province: derefOrEmpty(content.Province),
		}
	}

	for i, m := range media {
		resp.Media[i] = MediaResponse{
			ID:       m.ID,
			URL:      m.MediaURL,
			Type:     string(m.MediaType),
			Position: m.Position,
		}
	}

	attribution := contentApp.BuildContentAttributionContext(content)
	if attribution.OriginalAuthorID != nil {
		resp.OriginalAuthorID = attribution.OriginalAuthorID
	}
	return resp
}

// ToContentResponseWithProjection attaches the canonical resource projection.
func ToContentResponseWithProjection(content *entity.Content, media []*entity.ContentMedia, projection *contentApp.ContentResourceProjection) ContentResponse {
	resp := ToContentResponse(content, media)
	resp.ResourceProjection = projection
	return resp
}

// ToContentResponseWithAuthor builds a ContentResponse and attaches the
// canonical PublicCard ContentCard (Batch 2D). The author UserCard SHOULD be
// pre-hydrated by the caller via publiccard.BuildOne inside the same
// transaction that loaded the content; pass nil to emit a card-less response
// (the legacy flat-field shape).
//
// SharedForSale / SharedAuction are intentionally left nil on this surface.
// The response only carries the canonical resource_projection for resource-
// bearing content; live commerce-card hydration is a future batch.
func ToContentResponseWithAuthor(
	content *entity.Content,
	media []*entity.ContentMedia,
	author *publiccard.UserCard,
) ContentResponse {
	resp := ToContentResponse(content, media)

	var caption *string
	if content.Caption != nil && *content.Caption != "" {
		c := *content.Caption
		caption = &c
	}
	mediaURLs := make([]string, 0, len(media))
	for _, m := range media {
		if m.MediaURL != "" {
			mediaURLs = append(mediaURLs, m.MediaURL)
		}
	}

	card := publiccard.NewContentCard(
		content.ID,
		"content",
		caption,
		mediaURLs,
		content.Status.PublicLifecycle(),
		content.CreatedAt,
		author,
	)
	resp.Card = &card
	return resp
}

// ToContentResponseWithAuthorAndProjection attaches both the author card and
// the canonical resource projection.
func ToContentResponseWithAuthorAndProjection(
	content *entity.Content,
	media []*entity.ContentMedia,
	author *publiccard.UserCard,
	projection *contentApp.ContentResourceProjection,
) ContentResponse {
	resp := ToContentResponseWithProjection(content, media, projection)

	var caption *string
	if content.Caption != nil && *content.Caption != "" {
		c := *content.Caption
		caption = &c
	}
	mediaURLs := make([]string, 0, len(media))
	for _, m := range media {
		if m.MediaURL != "" {
			mediaURLs = append(mediaURLs, m.MediaURL)
		}
	}

	card := publiccard.NewContentCard(
		content.ID,
		"content",
		caption,
		mediaURLs,
		content.Status.PublicLifecycle(),
		content.CreatedAt,
		author,
	)
	resp.Card = &card
	return resp
}

func (h *ContentHandler) loadContentResourceProjection(
	ctx context.Context,
	tx db.Tx,
	viewerID uuid.UUID,
	contentID uuid.UUID,
) (*contentApp.ContentResourceProjection, error) {
	resolver := contentApp.NewContentResourceProjectionResolver()
	return resolver.ResolveContentResourceProjection(ctx, tx, viewerID, contentID)
}

// CreateContent handles POST /api/v1/contents
//
// Authorization:
// - Any active user can create content
//
// Requires Idempotency-Key header for safe retries.
// Idempotency is handled in the service layer within the same transaction.
func (h *ContentHandler) CreateContent(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context (set by auth middleware)
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Extract Idempotency-Key header
	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		response.BadRequest(c, "Idempotency-Key header required")
		return
	}

	// Parse request body
	var req CreateContentRequest
	if err := bindStrictContentJSON(c, &req, "share_reference", "original_author_id", "type"); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Execute create within transaction
	// Note: Idempotency handling is currently deferred to future implementation
	// The service layer should handle idempotency within the transaction
	var newContent *entity.Content
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error

		// Extract and validate location from request
		var city, province *string
		if req.Location != nil {
			// DATA HYGIENE: Trim whitespace from input
			cityStr := strings.TrimSpace(req.Location.City)
			provinceStr := strings.TrimSpace(req.Location.Province)

			// VALIDATION: Reject city/province longer than 100 characters
			const maxLocationLength = 100
			if len(cityStr) > maxLocationLength {
				response.BadRequest(c, fmt.Sprintf("city exceeds maximum length of %d characters", maxLocationLength))
				return fmt.Errorf("city too long: %d characters", len(cityStr))
			}
			if len(provinceStr) > maxLocationLength {
				response.BadRequest(c, fmt.Sprintf("province exceeds maximum length of %d characters", maxLocationLength))
				return fmt.Errorf("province too long: %d characters", len(provinceStr))
			}

			// Only set non-empty values (empty strings become nil)
			if cityStr != "" {
				city = &cityStr
			}
			if provinceStr != "" {
				province = &provinceStr
			}
		}

		var occurrence *entity.ContentResourceOccurrenceIdentity
		if req.ResourceOccurrence != nil {
			if req.ResourceOccurrence.Preview != nil {
				response.BadRequest(c, "Invalid request: resource_occurrence.preview is not supported")
				return fmt.Errorf("resource_occurrence.preview is not supported")
			}
			op := entity.ContentResourceOccurrenceOperation(req.ResourceOccurrence.Operation)
			rt := entity.ContentResourceOccurrenceResourceType(req.ResourceOccurrence.ResourceType)
			if !op.IsValid() || !rt.IsValid() {
				response.BadRequest(c, "Invalid request: invalid resource occurrence")
				return fmt.Errorf("invalid resource occurrence")
			}
			if op == entity.ContentResourceOccurrenceOperationDirectCommerceInsertContent && !rt.CanDirectCommerceInsert() {
				response.BadRequest(c, "Invalid request: invalid resource occurrence")
				return fmt.Errorf("invalid resource occurrence")
			}
			occurrence = &entity.ContentResourceOccurrenceIdentity{
				Operation:    op,
				ResourceType: rt,
				ResourceID:   req.ResourceOccurrence.ResourceID,
			}
		}

		visibility := entity.Visibility(req.Visibility).Normalize()
		if req.ResourceOccurrence != nil {
			newContent, err = h.contentService.CreateContentWithResourceOccurrence(ctx, tx, userID, req.Caption, visibility, city, province, occurrence, req.Tags)
		} else {
			newContent, err = h.contentService.CreateContent(ctx, tx, userID, req.Caption, city, province, nil, req.Tags)
		}
		if err != nil {
			return err
		}

		// Add media if provided
		if len(req.Media) > 0 {
			// Convert media inputs to entity media items
			mediaItems := make([]struct {
				MediaURL  string
				MediaType entity.MediaType
				Position  int
			}, len(req.Media))
			for i, m := range req.Media {
				mediaItems[i] = struct {
					MediaURL  string
					MediaType entity.MediaType
					Position  int
				}{
					MediaURL:  m.URL,
					MediaType: entity.MediaType(m.Type),
					Position:  i,
				}
			}

			if err := h.contentService.AddMedia(ctx, tx, userID, newContent.ID, mediaItems); err != nil {
				return fmt.Errorf("add media failed: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		h.log.Error("Failed to create content",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to create content")
		return
	}

	// Fetch media and author card for the response in a single tx.
	var media []*entity.ContentMedia
	var authorCard publiccard.UserCard
	var projection *contentApp.ContentResourceProjection
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		media, err = h.contentService.GetContentMedia(ctx, tx, newContent.ID)
		if err != nil {
			return err
		}
		authorCard, err = h.buildContentAuthorCardWithLifecycle(ctx, tx, newContent.AuthorID)
		if err != nil {
			return err
		}
		projection, err = h.loadContentResourceProjection(ctx, tx, userID, newContent.ID)
		return err
	})
	if err != nil {
		h.log.Warn("Failed to fetch media/author for response",
			zap.String("content_id", newContent.ID.String()),
			zap.Error(err),
		)
		// Continue without media/author card - non-fatal error
		media = []*entity.ContentMedia{}
	}

	// Convert to response DTO
	contentResp := ToContentResponseWithAuthorAndProjection(newContent, media, &authorCard, projection)
	contentResp.Engagement = &EngagementResponse{} // C7C: new content, all counts zero
	response.Created(c, contentResp)
}

// UpdateContentRequest holds the request body for updating content.
// Only caption can be updated - legacy title and body fields removed.
type UpdateContentRequest struct {
	Caption    *string `json:"caption,omitempty"`
	Visibility *string `json:"visibility,omitempty"` // public, followers_only, private
}

func bindStrictContentJSON(c *gin.Context, dst any, deprecatedKeys ...string) error {
	raw, err := c.GetRawData()
	if err != nil {
		return err
	}

	if len(raw) > 0 {
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(raw, &payload); err != nil {
			return err
		}
		for _, key := range deprecatedKeys {
			if _, ok := payload[key]; ok {
				return fmt.Errorf("legacy field %s is not supported", key)
			}
		}
	}

	c.Request.Body = io.NopCloser(bytes.NewReader(raw))
	return c.ShouldBindJSON(dst)
}

// UpdateContent handles PUT /api/v1/contents/{id}
//
// Authorization:
// - Only author can update their content
//
// Requires Idempotency-Key header for safe retries.
func (h *ContentHandler) UpdateContent(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse content ID
	idStr := c.Param("id")
	contentID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid content ID")
		return
	}

	// Get user ID from context
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Extract Idempotency-Key header
	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		response.BadRequest(c, "Idempotency-Key header required")
		return
	}

	// Parse request body
	var req UpdateContentRequest
	if err := bindStrictContentJSON(c, &req, "share_reference", "resource_occurrence", "type"); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Get content to verify ownership
	var content *entity.Content
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		content, err = h.contentService.GetContent(ctx, tx, contentID)
		return err
	})

	if err != nil {
		h.log.Error("Failed to get content",
			zap.String("content_id", contentID.String()),
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.NotFound(c, "Content not found")
		return
	}

	// Authorization: only author can update
	if content.AuthorID != userID {
		isAdmin, _ := h.roleChecker.IsAdmin(ctx, userID)
		if !isAdmin {
			response.Forbidden(c, "You can only update your own content")
			return
		}
	}

	// Execute update within transaction

	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.contentService.UpdateCaptionAndVisibility(ctx, tx, userID, contentID, req.Caption, req.Visibility)
	})

	if err != nil {
		h.log.Error("Failed to update content",
			zap.String("content_id", contentID.String()),
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to update content")
		return
	}

	// Fetch updated content for response
	var updatedContent *entity.Content
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		updatedContent, err = h.contentService.GetContent(ctx, tx, contentID)
		return err
	})

	if err != nil {
		h.log.Error("Failed to fetch updated content",
			zap.String("content_id", contentID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Content updated but failed to fetch")
		return
	}

	// Fetch media and author card for the response in a single tx.
	var media []*entity.ContentMedia
	var authorCard publiccard.UserCard
	var projection *contentApp.ContentResourceProjection
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		media, err = h.contentService.GetContentMedia(ctx, tx, contentID)
		if err != nil {
			return err
		}
		authorCard, err = h.buildContentAuthorCardWithLifecycle(ctx, tx, updatedContent.AuthorID)
		if err != nil {
			return err
		}
		projection, err = h.loadContentResourceProjection(ctx, tx, userID, contentID)
		return err
	})
	if err != nil {
		media = []*entity.ContentMedia{}
	}

	contentResp := ToContentResponseWithAuthorAndProjection(updatedContent, media, &authorCard, projection)
	contentResp.Engagement = &EngagementResponse{} // C7C: best-effort zero (update doesn't re-query counts)
	response.Success(c, contentResp)
}

// DeleteContent handles DELETE /api/v1/contents/{id}
//
// Authorization:
// - Only author can delete their content (admin can override)
func (h *ContentHandler) DeleteContent(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse content ID
	idStr := c.Param("id")
	contentID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid content ID")
		return
	}

	// Get user ID from context
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Execute delete within transaction
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.contentService.DeleteContent(ctx, tx, userID, contentID)
	})

	if err != nil {
		h.log.Error("Failed to delete content",
			zap.String("content_id", contentID.String()),
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)

		// Check for authorization error
		if err == auth.ErrOwnerRequired {
			response.Forbidden(c, "You can only delete your own content")
			return
		}

		response.InternalServerError(c, "Failed to delete content")
		return
	}

	response.SuccessWithMessage(c, "Content deleted successfully", gin.H{
		"content_id": contentID,
	})
}

// GetContent retrieves a single content by ID.
//
// GET /api/v1/contents/{id}
//
// Authorization:
// - Deleted/moderated content is hidden unless admin
func (h *ContentHandler) GetContent(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse content ID
	idStr := c.Param("id")
	contentID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid content ID")
		return
	}

	// F1-W3B — Canonical Pattern A ViewerContext construction at the
	// HTTP boundary. /contents/:id is anonymous-permissive per
	// content-detail-visibility-doctrine §2.6; AnonymousViewer is a
	// legitimate outcome here and the derived userID = uuid.Nil
	// preserves the existing handler semantics exactly. The pre-tx
	// invocation passes nil tx for the cheap construction path; the
	// post-tx re-invocation (inside the transaction below) wires inline
	// viewer-lifecycle hydration.
	vc := constructContentDetailViewerContext(c, nil)
	userID := vc.Identity().CanonicalUserID

	// F1-W3B — load content + build canonical TargetContext +
	// relationship overlay inside ONE tx so the evaluator + shadow
	// runner consume pre-hydrated canonical types. Order:
	//   1. Re-construct the ViewerContext WITH tx — inline viewer
	//      lifecycle hydration via hydrateContentDetailViewerLifecycle.
	//   2. Load the requested content row.
	//   3. Build the canonical TargetContext (owner lifecycle +
	//      content moderation).
	//   4. Attach the bidirectional block overlay to the VC.
	//   5. Hydrate media + author card for the response payload.
	var content *entity.Content
	var media []*entity.ContentMedia
	var authorCard publiccard.UserCard
	var projection *contentApp.ContentResourceProjection
	var tc *viewercontext.TargetContext
	var likeCount int
	var commentCount int
	var isLiked bool
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		vc = constructContentDetailViewerContext(c, tx)
		var loadErr error
		content, loadErr = h.contentService.GetContentPublic(ctx, tx, contentID)
		if loadErr != nil {
			return loadErr
		}

		tc = hydrateContentDetailTargetContext(ctx, tx, content)
		vc = hydrateContentDetailRelationship(ctx, tx, vc, content)

		// Best-effort response hydration. Errors here degrade the
		// response shape (empty media / anonymous-safe author card)
		// without failing the request; mirrors pre-W3B semantics.
		if m, mediaErr := h.contentService.GetContentMedia(ctx, tx, contentID); mediaErr == nil {
			media = m
		}
		if card, cardErr := h.buildContentAuthorCardWithLifecycle(ctx, tx, content.AuthorID); cardErr == nil {
			authorCard = card
		}
		if proj, projErr := h.loadContentResourceProjection(ctx, tx, userID, content.ID); projErr == nil {
			projection = proj
		}
		// C7C — best-effort engagement hydration.
		if lc, lcErr := h.contentService.GetLikeCount(ctx, tx, contentID); lcErr == nil {
			likeCount = lc
		}
		if cc, ccErr := h.contentService.GetCommentCount(ctx, tx, contentID); ccErr == nil {
			commentCount = cc
		}
		if userID != uuid.Nil {
			if liked, likedErr := h.contentService.IsLiked(ctx, tx, userID, contentID); likedErr == nil {
				isLiked = liked
			}
		}
		return nil
	})

	if err != nil {
		h.log.Error("Failed to get content",
			zap.String("content_id", contentID.String()),
			zap.Error(err),
		)
		response.NotFound(c, "Content not found")
		return
	}
	if media == nil {
		media = []*entity.ContentMedia{}
	}

	// Hide deleted/moderated content unless admin. The legacy gate runs
	// FIRST so the canonical enforce path can only further-restrict
	// (per the F1-W3B further-restrict contract). IsHidden is gated
	// here — it must never reach a non-admin caller, nor cross the
	// boundary as a field.
	if content.Status == entity.StatusDeleted || content.IsHidden {
		isAdmin := false
		if userID != uuid.Nil {
			isAdmin, _ = h.roleChecker.IsAdmin(ctx, userID)
		}
		if !isAdmin {
			response.NotFound(c, "Content not found")
			// BATCH 3Q / F1-W3B — dispatch /contents/:id shadow
			// evaluator on the 404 gate path. The runner is
			// observability-only; the 404 response above is already
			// written. Pre-hydrated canonical (vc, tc, content) are
			// passed in; the runner does NO DB work.
			h.contentDetailShadowRunner.Run(vc, tc, content, evaluator.LegacyContentDetailOutcome404)
			return
		}
	}

	// D1 / F1-W3B — synchronous fail-CLOSED enforcement.
	//
	// After the legacy gate passes, in ContentDetailEvaluatorModeEnforce
	// the handler runs EvaluateContentDetail + AdaptContentDetailDecision
	// and converts any non-ALLOW outcome (DENY / TOMBSTONE / REDACT /
	// UNKNOWN) into HTTP 404. This implements doctrine §8.5 (fail-CLOSED
	// on UNKNOWN; detail surface never silently passes an unhydrated
	// decision). In shadow mode the helper short-circuits to allow=true
	// and the legacy gate (already passed) decides — wire shape is
	// byte-identical to pre-D1.
	//
	// F1-W3B — the helper consumes the same pre-hydrated canonical
	// (vc, tc, content) the handler built inside WithTx. The evaluator
	// package owns NO SQL, NO pool, NO hydration helpers.
	if h.contentDetailShadowRunner.Mode() == evaluator.ContentDetailEvaluatorModeEnforce {
		enf := evaluator.EnforceContentDetail(
			evaluator.ContentDetailEvaluatorModeEnforce, vc, tc, content,
		)
		if !enf.Allow {
			response.NotFound(c, "Content not found")
			h.log.Info("content_detail enforce 404",
				zap.String("content_id", content.ID.String()),
				zap.String("reason", string(enf.Reason)),
				zap.String("shadow_decision", string(enf.ShadowDecision)),
			)
			// Dispatch the async shadow runner with LegacyOutcome=200
			// because the legacy gate authorized the response BEFORE
			// enforcement converted it to 404. This preserves shadow
			// telemetry's denominator across the shadow→enforce flip
			// (the shadow seam continues to observe what the legacy gate
			// allowed, not what enforce converted).
			h.contentDetailShadowRunner.Run(vc, tc, content, evaluator.LegacyContentDetailOutcome200)
			return
		}
	}

	contentResp := ToContentResponseWithAuthorAndProjection(content, media, &authorCard, projection)
	// C7C — attach engagement stats to content detail response.
	contentResp.Engagement = &EngagementResponse{LikeCount: likeCount, CommentCount: commentCount}
	if userID != uuid.Nil {
		contentResp.IsLiked = &isLiked
	}
	response.Success(c, contentResp)

	// BATCH 3Q / F1-W3B — dispatch /contents/:id shadow evaluator on
	// the success path. Fired AFTER response.Success so the response
	// is fully written before the goroutine spawns. Pre-hydrated
	// canonical (vc, tc, content) — the runner does NO DB work.
	// nil-safe when shadow is disabled.
	h.contentDetailShadowRunner.Run(vc, tc, content, evaluator.LegacyContentDetailOutcome200)
}

// GetUserContent handles GET /api/v1/users/:id/contents
//
// Returns the public active content for a target user. Auth is optional;
// anonymous callers receive the same view as authenticated non-blocked viewers.
//
// Governance overlays applied:
//   - is_hidden = false enforced at repository (hidden/moderated items excluded).
//   - status = 'active' + deleted_at IS NULL enforced at repository.
//   - Bidirectional block: viewer↔target block denies with 403.
//   - Author lifecycle coarsened via viewercontext.CoarsenLifecycle; raw
//     account_status never leaves the handler.
//   - PublicCard emitted on every item via card.author.lifecycle.
//
// Pagination: cursor-based (opaque created_at timestamp). Callers pass the
// next_cursor value from a prior response verbatim as ?cursor=. A null
// next_cursor signals the terminal page.
func (h *ContentHandler) GetUserContent(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse target user UUID.
	targetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}

	// Parse pagination params; clamp limit to [1, 50], default 20.
	limit := 20
	if s := c.Query("limit"); s != "" {
		if n, pErr := strconv.Atoi(s); pErr == nil && n > 0 {
			limit = n
		}
	}
	if limit > 50 {
		limit = 50
	}
	cursor := c.Query("cursor")

	// F1-W2 — Canonical Pattern A ViewerContext construction at the
	// HTTP boundary. Replaces the prior silent `var viewerID
	// uuid.UUID; if exists ...` zero-UUID fallback (F6 forbidden
	// pattern). /users/:id/contents is anonymous-permissive per the
	// docstring above; AnonymousViewer is a legitimate outcome and
	// the derived viewerID = uuid.Nil preserves the existing handler
	// semantics — block governance below correctly short-circuits
	// when viewerID is uuid.Nil (anonymous viewers cannot be in a
	// user_blocks relationship by definition).
	vc := constructUserContentViewerContext(c, nil)
	viewerID := vc.Identity().CanonicalUserID

	// Block governance: deny if a bidirectional block exists between viewer
	// and target. Fail-open on infra error (do not deny on transient DB failure).
	if viewerID != uuid.Nil {
		var blocked bool
		if bErr := h.db.WithTx(ctx, func(tx db.Tx) error {
			var e error
			blocked, e = h.checkBidirectionalBlock(ctx, tx, viewerID, targetID)
			return e
		}); bErr != nil {
			h.log.Warn("block check failed for GetUserContent",
				zap.String("viewer_id", viewerID.String()),
				zap.String("target_id", targetID.String()),
				zap.Error(bErr),
			)
		} else if blocked {
			response.Forbidden(c, "Content not accessible")
			return
		}
	}

	// Fetch content list via existing service / repository layer.
	// Hidden, deleted, and non-active content excluded at the repository.
	var contents []*entity.Content
	var nextCursorStr string
	if lErr := h.db.WithTx(ctx, func(tx db.Tx) error {
		var e error
		contents, nextCursorStr, e = h.contentService.ListByAuthor(ctx, tx, targetID, limit, cursor)
		return e
	}); lErr != nil {
		h.log.Error("ListByAuthor failed for GetUserContent",
			zap.String("target_id", targetID.String()),
			zap.Error(lErr),
		)
		response.InternalServerError(c, "Failed to fetch user content")
		return
	}

	if len(contents) == 0 {
		response.Success(c, UserContentListResponse{
			Data:       []ContentResponse{},
			NextCursor: nil,
			HasMore:    false,
		})
		return
	}

	// Hydrate the author card once — all items share the same author.
	var authorCard publiccard.UserCard
	if hErr := h.db.WithTx(ctx, func(tx db.Tx) error {
		var e error
		authorCard, e = h.buildContentAuthorCardWithLifecycle(ctx, tx, targetID)
		return e
	}); hErr != nil {
		h.log.Warn("author card hydration failed for GetUserContent",
			zap.String("target_id", targetID.String()),
			zap.Error(hErr),
		)
		// Non-fatal: authorCard stays zero-value (anonymous-safe fallback).
	}

	// Batch-fetch media and build response items in one transaction.
	// A single item's media failure is non-fatal; it degrades to empty media
	// so the rest of the list remains visible.
	items := make([]ContentResponse, 0, len(contents))
	if mErr := h.db.WithTx(ctx, func(tx db.Tx) error {
		for _, ct := range contents {
			media, fetchErr := h.contentService.GetContentMedia(ctx, tx, ct.ID)
			if fetchErr != nil {
				h.log.Warn("media fetch failed for content item in GetUserContent",
					zap.String("content_id", ct.ID.String()),
					zap.Error(fetchErr),
				)
				media = []*entity.ContentMedia{}
			}
			var projection *contentApp.ContentResourceProjection
			if proj, projErr := h.loadContentResourceProjection(ctx, tx, viewerID, ct.ID); projErr == nil {
				projection = proj
			}
			resp := ToContentResponseWithAuthorAndProjection(ct, media, &authorCard, projection)

			// C7C — canonical engagement hydration, same live-count authority
			// as GET /contents/:id (no stored/denormalized counters).
			var likeCount, commentCount int
			if lc, lcErr := h.contentService.GetLikeCount(ctx, tx, ct.ID); lcErr == nil {
				likeCount = lc
			}
			if cc, ccErr := h.contentService.GetCommentCount(ctx, tx, ct.ID); ccErr == nil {
				commentCount = cc
			}
			resp.Engagement = &EngagementResponse{LikeCount: likeCount, CommentCount: commentCount}

			// Per-viewer is_liked only for authenticated viewers (anonymous
			// callers omit the field, mirroring content detail).
			if viewerID != uuid.Nil {
				if liked, likedErr := h.contentService.IsLiked(ctx, tx, viewerID, ct.ID); likedErr == nil {
					resp.IsLiked = &liked
				}
			}

			items = append(items, resp)
		}
		return nil
	}); mErr != nil {
		h.log.Warn("media batch transaction failed for GetUserContent",
			zap.String("target_id", targetID.String()),
			zap.Error(mErr),
		)
	}

	var nextCursorPtr *string
	if nextCursorStr != "" {
		nextCursorPtr = &nextCursorStr
	}

	response.Success(c, UserContentListResponse{
		Data:       items,
		NextCursor: nextCursorPtr,
		HasMore:    nextCursorStr != "",
	})
}

// ============================================================================
// SHARE CONTRACT V1: Repost Endpoints
// ============================================================================

// CreateRepostRequest holds the request body for creating a repost.
type CreateRepostRequest struct {
	OriginalContentID string `json:"original_content_id" binding:"required"`
	Caption           string `json:"caption"`
	// OriginalAuthorID is optional - backend will fetch from original content if not provided
	OriginalAuthorID        string `json:"original_author_id"`
	OriginalContentTitle    string `json:"original_content_title"`
	OriginalContentImageURL string `json:"original_content_image_url"`
}

// RepostContent handles POST /api/v1/contents/{id}/repost
//
// SHARE CONTRACT V1:
// - Creates NEW Content as a repost of the original content
// - Original author attribution is preserved via originalAuthorId
// - Source object is NOT mutated
//
// Authorization:
// - Any active user can repost content
func (h *ContentHandler) RepostContent(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Parse request body
	var req CreateRepostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Parse original content ID
	originalContentID, err := uuid.Parse(req.OriginalContentID)
	if err != nil {
		response.BadRequest(c, "Invalid original content ID")
		return
	}

	// Parse optional original author ID (backend will fetch from content if not provided)
	var originalAuthorID uuid.UUID
	if req.OriginalAuthorID != "" {
		originalAuthorID, err = uuid.Parse(req.OriginalAuthorID)
		if err != nil {
			response.BadRequest(c, "Invalid original author ID")
			return
		}
	}

	// Build service request
	serviceReq := &contentApp.CreateRepostRequest{
		OriginalContentID:       originalContentID,
		Caption:                 req.Caption,
		OriginalContentAuthorID: originalAuthorID,
		OriginalContentTitle:    req.OriginalContentTitle,
		OriginalContentImageURL: req.OriginalContentImageURL,
	}

	// Execute repost within transaction
	var repost *entity.Content
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		repost, err = h.contentService.CreateRepost(ctx, tx, userID, serviceReq)
		return err
	})

	if err != nil {
		// Map service-level EnsureActive errors to typed 403s so the mobile
		// client can react with the canonical blocked-action UX instead of a
		// generic 500. Route middleware already gates email-verified; this
		// branch covers the account-status half of the doctrine.
		switch {
		case errors.Is(err, auth.ErrAccountSuspended):
			response.Error(c, http.StatusForbidden, "ACCOUNT_SUSPENDED", "Your account has been suspended.")
			return
		case errors.Is(err, auth.ErrAccountBanned):
			response.Error(c, http.StatusForbidden, "ACCOUNT_BANNED", "Your account has been banned.")
			return
		}
		h.log.Error("Failed to create repost",
			zap.String("user_id", userID.String()),
			zap.String("original_content_id", req.OriginalContentID),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to create repost")
		return
	}

	// Fetch the author card and resource projection so the response carries
	// the canonical PublicCard and resource_projection blocks.
	var media []*entity.ContentMedia
	var authorCard publiccard.UserCard
	var projection *contentApp.ContentResourceProjection
	if hydrateErr := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		authorCard, err = h.buildContentAuthorCardWithLifecycle(ctx, tx, repost.AuthorID)
		if err != nil {
			return err
		}
		projection, err = h.loadContentResourceProjection(ctx, tx, userID, repost.ID)
		return err
	}); hydrateErr != nil {
		// Card hydration is non-critical; the wire stays valid with a
		// zero-value UserCard (anonymous-safe username fallback).
	}

	// Convert to response DTO
	contentResp := ToContentResponseWithAuthorAndProjection(repost, media, &authorCard, projection)
	contentResp.Engagement = &EngagementResponse{} // C7C: new repost, all counts zero
	response.Created(c, contentResp)
}
