package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	forSaleApp "github.com/labuda/backend/internal/commerce/forsale/application"
	"github.com/labuda/backend/internal/governance/viewercontext"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/platform/response"
	contentApp "github.com/labuda/backend/internal/social/content/application"
	"github.com/labuda/backend/internal/social/content/entity"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

type blockQueryRunner interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// CommentHandler handles HTTP requests for comment operations.
type CommentHandler struct {
	commentService     *contentApp.CommentService
	contentService     *contentApp.ContentService
	forSaleService     *forSaleApp.ForSaleService
	roleChecker        auth.RoleChecker
	db                 *db.DB
	log                *zap.Logger
	blockQueryOverride blockQueryRunner
}

// NewCommentHandler creates a new CommentHandler.
func NewCommentHandler(
	commentService *contentApp.CommentService,
	contentService *contentApp.ContentService,
	extra ...any,
) *CommentHandler {
	handler := &CommentHandler{
		commentService: commentService,
		contentService: contentService,
	}

	switch len(extra) {
	case 2:
		if v, ok := extra[0].(*db.DB); ok {
			handler.db = v
		}
		if v, ok := extra[1].(*zap.Logger); ok {
			handler.log = v
		}
	case 4:
		if v, ok := extra[0].(*forSaleApp.ForSaleService); ok {
			handler.forSaleService = v
		}
		if v, ok := extra[1].(auth.RoleChecker); ok {
			handler.roleChecker = v
		}
		if v, ok := extra[2].(*db.DB); ok {
			handler.db = v
		}
		if v, ok := extra[3].(*zap.Logger); ok {
			handler.log = v
		}
	default:
		// Best effort for partial wiring in unit tests.
		for _, arg := range extra {
			switch v := arg.(type) {
			case *forSaleApp.ForSaleService:
				handler.forSaleService = v
			case auth.RoleChecker:
				handler.roleChecker = v
			case *db.DB:
				handler.db = v
			case *zap.Logger:
				handler.log = v
			}
		}
	}

	if handler.log == nil {
		handler.log = zap.NewNop()
	}
	return handler
}

// CreateCommentRequest holds the request body for creating a comment.
type CreateCommentRequest struct {
	Body     string  `json:"body" binding:"required"`
	ParentID *string `json:"parent_id,omitempty"`
}

// CreateComment handles POST /api/v1/contents/{id}/comments
//
// Authorization:
// - Any active user can comment
// - Cannot comment on deleted content
//
// Requires Idempotency-Key header for safe retries.
func (h *CommentHandler) CreateComment(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse content ID
	idStr := c.Param("id")
	contentID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid content ID")
		return
	}

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
	var req CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Parse parent_id if provided
	var parentID *uuid.UUID
	if req.ParentID != nil && *req.ParentID != "" {
		parsedID, err := uuid.Parse(*req.ParentID)
		if err != nil {
			response.BadRequest(c, "Invalid parent ID")
			return
		}
		parentID = &parsedID
	}

	// A2 — Bidirectional block check between commenter and content author.
	// Mirrors the pattern used in GetUserContent (content_handler.go) and
	// filterBlockedComments (read surface). Fail-open: if the block query or
	// the author lookup errors we log and allow the comment to proceed so
	// transient DB issues do not silently break comment creation.
	var contentAuthorID uuid.UUID
	if bcErr := h.db.Pool().QueryRow(ctx,
		`SELECT author_id FROM contents WHERE id = $1 AND deleted_at IS NULL`,
		contentID,
	).Scan(&contentAuthorID); bcErr == nil && contentAuthorID != userID {
		var blocked bool
		if qErr := h.db.Pool().QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM user_blocks
				WHERE (blocker_id = $1 AND blocked_id = $2)
				   OR (blocker_id = $2 AND blocked_id = $1)
			)
		`, userID, contentAuthorID).Scan(&blocked); qErr != nil {
			h.log.Warn("block check query failed for CreateComment, allowing proceed",
				zap.String("content_id", contentID.String()),
				zap.String("user_id", userID.String()),
				zap.Error(qErr),
			)
		} else if blocked {
			response.Forbidden(c, "Cannot comment on this content")
			return
		}
	}

	// Execute create within transaction
	var newComment *entity.Comment
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		newComment, err = h.commentService.AddComment(ctx, tx, userID, contentID, req.Body, parentID, idempotencyKey)
		return err
	})

	if err != nil {
		h.log.Error("Failed to create comment",
			zap.String("content_id", contentID.String()),
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)

		// Check for specific errors
		if invalidErr, ok := err.(*entity.ErrInvalidComment); ok {
			response.BadRequest(c, invalidErr.Error())
			return
		}
		if errors.Is(err, entity.ErrIdempotencyConflict) {
			response.Error(c, http.StatusConflict, "IDEMPOTENCY_CONFLICT", "Idempotency key already used for a different operation")
			return
		}

		response.InternalServerError(c, "Failed to create comment")
		return
	}

	// C-RESP — POST returns the canonical snake_case CommentResponse (same
	// wire shape as the list endpoint), never the raw entity.
	response.Created(c, h.buildCreateCommentResponse(ctx, newComment))
}

// buildCreateCommentResponse hydrates the canonical CommentResponse for a
// freshly created or idempotently replayed comment. C-RESP: create endpoints
// return the same snake_case wire shape as the list endpoint, with the
// embedded author card and (for FPS commerce references) forSale preview.
func (h *CommentHandler) buildCreateCommentResponse(ctx context.Context, comment *entity.Comment) *contentApp.CommentResponse {
	var preview *contentApp.ForSalePreview
	if comment.IsCommerceReference() && comment.Reference != nil && comment.Reference.TargetType == entity.ShareTargetTypeForSale {
		forSaleID, err := uuid.Parse(comment.Reference.TargetID)
		if err == nil && h.forSaleService != nil {
			perr := h.db.WithTx(ctx, func(tx db.Tx) error {
				forSale, lerr := h.forSaleService.GetByID(ctx, tx, forSaleID)
				if lerr != nil {
					return lerr
				}
				lp, lperr := contentApp.GetForSalePreviewFromForSale(forSale.ID, forSale.Title, forSale.PricePerUnit, forSale.MediaURLs, forSale.Status.String())
				if lperr != nil {
					return lperr
				}
				preview = lp
				return nil
			})
			if perr != nil {
				h.log.Warn("Failed to fetch forSale preview for created comment", zap.Error(perr))
			}
		}
	}

	var username, lifecycle string
	var avatarURL *string
	info := h.fetchCommentAuthorsInfo(ctx, map[uuid.UUID]bool{comment.AuthorID: true})
	if info != nil {
		if author := info[comment.AuthorID]; author != nil {
			username = author.Username
			avatarURL = author.AvatarURL
			lifecycle = author.Lifecycle
		}
	}

	return contentApp.NewCommentResponse(comment, preview, username, avatarURL, lifecycle)
}

// DeleteComment handles DELETE /api/v1/comments/{id}
//
// Authorization:
// - Public HTTP delete is author-only
// - Moderator removal is handled separately by the moderation worker
// - Soft delete only
func (h *CommentHandler) DeleteComment(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse comment ID
	idStr := c.Param("id")
	commentID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid comment ID")
		return
	}

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

	// Execute delete within transaction
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.commentService.DeleteComment(ctx, tx, commentID, userID)
	})

	if err != nil {
		h.log.Error("Failed to delete comment",
			zap.String("comment_id", commentID.String()),
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)

		// Check for specific errors
		if err.Error() == "only comment author or moderator can delete comment" {
			response.Forbidden(c, "Access denied")
			return
		}

		response.InternalServerError(c, "Failed to delete comment")
		return
	}

	response.SuccessWithMessage(c, "Comment deleted successfully", nil)
}

// ListCommentsRequest holds the query parameters for forSale comments.
type ListCommentsRequest struct {
	Limit  int    `form:"limit" binding:"omitempty,min=1,max=50"`
	Cursor string `form:"cursor"`
}

// ListComments handles GET /api/v1/contents/{id}/comments
//
// Query parameters:
// - limit (optional): Number of results per page (default: 20, max: 50)
// - cursor (optional): ISO timestamp for cursor-based pagination
//
// Returns paginated list of non-deleted comments for the content.
// Cursor-based pagination: next_cursor returned in response if more results exist.
// For commerce-reference comments, includes lightweight resource preview (id, title, price, media_urls).
// For all comments, includes embedded author info (name, username, avatar) for proper UI rendering.
// Block filtering: authenticated viewers will not see comments from users they blocked or who blocked them.
// Anonymous viewers see all comments unfiltered.
func (h *CommentHandler) ListComments(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse content ID
	idStr := c.Param("id")
	contentID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid content ID")
		return
	}

	// Extract optional viewer ID for block filtering. List is public so auth is
	// not required; we extract it opportunistically when the token is present.
	var viewerID *uuid.UUID
	if userIDVal, exists := c.Get("userID"); exists {
		if uid, ok := userIDVal.(uuid.UUID); ok {
			viewerID = &uid
		}
	}

	// Parse query parameters
	var req ListCommentsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}

	// Set default limit if not provided
	if req.Limit <= 0 {
		req.Limit = 20
	}

	// Execute list within transaction.
	// SC-3: Before forSale comments, enforce the same parent-content visibility
	// policy as GET /contents/:id. If the parent is deleted or hidden (moderated),
	// comments must not be publicly accessible. Admin callers bypass this gate.
	var parentNotVisible bool
	var comments []*entity.Comment
	var nextCursor string
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		// Load parent content to check visibility.
		parentContent, contentErr := h.contentService.GetContent(ctx, tx, contentID)
		if contentErr != nil {
			// Content not found at all — treat as not visible.
			parentNotVisible = true
			return nil
		}
		if !isParentContentPubliclyListable(parentContent) {
			isAdmin := false
			if viewerID != nil {
				isAdmin, _ = h.roleChecker.IsAdmin(ctx, *viewerID)
			}
			if !isAdmin {
				parentNotVisible = true
				return nil
			}
		}

		var err error
		comments, nextCursor, err = h.commentService.ListComments(ctx, tx, contentID, req.Limit, req.Cursor)
		return err
	})

	if parentNotVisible {
		response.NotFound(c, "Content not found")
		return
	}

	// Apply bidirectional block filter for authenticated viewers. Anonymous
	// viewers always see the full unfiltered list. Fail-open: if the block
	// query itself errors, we log and return the unfiltered list rather than
	// surfacing an error. Pagination note: filtered results may be fewer than
	// limit; next_cursor from the repository is still forwarded so the client
	// can continue paginating.
	if err == nil && viewerID != nil {
		comments = h.filterBlockedComments(ctx, *viewerID, comments)
	}

	if err != nil {
		h.log.Error("Failed to list comments",
			zap.String("content_id", contentID.String()),
			zap.Int("limit", req.Limit),
			zap.String("cursor", req.Cursor),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve comments")
		return
	}

	// Build enriched response with forSale previews and author info
	commentResponses := make([]*contentApp.CommentResponse, 0, len(comments))

	// Collect commerce target IDs and author IDs that need to be fetched.
	forSaleIDsMap := make(map[uuid.UUID]bool)
	authorIDsMap := make(map[uuid.UUID]bool)
	for _, comment := range comments {
		if comment.IsCommerceReference() && comment.Reference != nil && comment.Reference.TargetType == entity.ShareTargetTypeForSale {
			forSaleID, err := uuid.Parse(comment.Reference.TargetID)
			if err == nil {
				forSaleIDsMap[forSaleID] = true
			}
		}
		authorIDsMap[comment.AuthorID] = true
	}

	// Fetch forSale data in a separate transaction
	forSalesMap := make(map[uuid.UUID]*contentApp.ForSalePreview)
	if len(forSaleIDsMap) > 0 {
		err = h.db.WithTx(ctx, func(tx db.Tx) error {
			for forSaleID := range forSaleIDsMap {
				forSale, err := h.forSaleService.GetByID(ctx, tx, forSaleID)
				if err != nil {
					// Log but continue - forSale might have been deleted
					h.log.Warn("Failed to fetch forSale for comment preview",
						zap.String("for_sale_id", forSaleID.String()),
						zap.Error(err),
					)
					continue
				}

				// Convert to preview
				preview, err := contentApp.GetForSalePreviewFromForSale(
					forSale.ID,
					forSale.Title,
					forSale.PricePerUnit,
					forSale.MediaURLs,
					forSale.Status.String(),
				)
				if err != nil {
					h.log.Warn("Failed to create forSale preview",
						zap.String("for_sale_id", forSaleID.String()),
						zap.Error(err),
					)
					continue
				}
				forSalesMap[forSaleID] = preview
			}
			return nil
		})
		if err != nil {
			h.log.Warn("Failed to fetch forSale previews", zap.Error(err))
		}
	}

	// Fetch author info for all comment authors
	authorInfoMap := h.fetchCommentAuthorsInfo(ctx, authorIDsMap)

	// Build comment responses
	for _, comment := range comments {
		var preview *contentApp.ForSalePreview
		if comment.IsCommerceReference() && comment.Reference != nil && comment.Reference.TargetType == entity.ShareTargetTypeForSale {
			forSaleID, err := uuid.Parse(comment.Reference.TargetID)
			if err == nil {
				preview = forSalesMap[forSaleID]
			}
		}

		// Get author info
		authorInfo := authorInfoMap[comment.AuthorID]
		var authorUsername, authorLifecycle string
		var authorAvatarURL *string
		if authorInfo != nil {
			authorUsername = authorInfo.Username
			authorAvatarURL = authorInfo.AvatarURL
			authorLifecycle = authorInfo.Lifecycle
		}

		commentResponses = append(commentResponses, contentApp.NewCommentResponse(
			comment,
			preview,
			authorUsername,
			authorAvatarURL,
			authorLifecycle,
		))
	}

	// Build response with next_cursor
	result := gin.H{
		"comments": commentResponses,
		"limit":    req.Limit,
	}
	if nextCursor != "" {
		result["next_cursor"] = nextCursor
	}

	response.Success(c, result)
}

// isParentContentPubliclyListable reports whether a content entity is visible
// enough for its comments to be publicly served. Mirrors the legacy gate in
// GET /contents/:id (content_handler.go): deleted or hidden (admin-moderated)
// content is not publicly accessible. This is a pure function — admin bypass
// is enforced by the caller (ListComments handler) before invoking.
//
// SC-3 regression lock: any change to this predicate must be reflected in
// TestIsParentContentPubliclyListable in comment_list_s4_test.go.
func isParentContentPubliclyListable(content *entity.Content) bool {
	return content.Status != entity.StatusDeleted && !content.IsHidden
}

// CommentAuthorInfo holds author information for comments.
//
// E3.2 — Lifecycle field carries the coarsened public lifecycle state for
// the comment author, sourced from users.account_status + users.deleted_at
// and materialised via viewercontext.CoarsenLifecycle in Go (NEVER in SQL).
// Empty string when not hydrated. Non-empty values are constrained to the
// canonical public lifecycle vocabulary: "active", "unavailable",
// "removed".
type CommentAuthorInfo struct {
	Username  string
	AvatarURL *string
	Lifecycle string
}

// fetchCommentAuthorsInfo fetches author info for all comment authors
// Uses a single query with ANY clause for efficiency
func (h *CommentHandler) fetchCommentAuthorsInfo(ctx context.Context, authorIDs map[uuid.UUID]bool) map[uuid.UUID]*CommentAuthorInfo {
	if len(authorIDs) == 0 {
		return nil
	}

	// Build slice of IDs for ANY clause
	ids := make([]uuid.UUID, 0, len(authorIDs))
	for id := range authorIDs {
		ids = append(ids, id)
	}

	// Query all author info in a single query.
	// PRIVACY: public comment identity is username only. p.full_name is KYC/private
	// data and MUST NOT be emitted on this surface.
	//
	// S4 — deleted_at IS NULL filter removed so soft-deleted authors are
	// returned with is_deleted=true. CoarsenLifecycle maps them to "removed".
	// This does NOT change comment visibility; only the author card lifecycle
	// field changes from empty-string (missing map entry) to "removed".
	query := `
		SELECT
			u.id,
			COALESCE(p.username, '') as username,
			p.avatar_url,
			u.account_status,
			(u.deleted_at IS NOT NULL) AS is_deleted
		FROM users u
		LEFT JOIN user_profiles p ON u.id = p.user_id
		WHERE u.id = ANY($1)
	`

	rows, err := h.db.Pool().Query(ctx, query, ids)
	if err != nil {
		h.log.Warn("Failed to fetch comment authors info", zap.Error(err))
		return nil
	}
	defer rows.Close()

	result := make(map[uuid.UUID]*CommentAuthorInfo)
	for rows.Next() {
		var userID uuid.UUID
		var username string
		var avatarURL *string
		var accountStatus string
		var isDeleted bool

		if err := rows.Scan(&userID, &username, &avatarURL, &accountStatus, &isDeleted); err != nil {
			h.log.Warn("Failed to scan author info", zap.Error(err))
			continue
		}

		// Coarsen raw lifecycle truth via the canonical site. The raw
		// account_status enum never leaves this layer. With deleted_at IS NULL
		// removed (S4), isDeleted can now be true for soft-deleted authors,
		// yielding lifecycle="removed". Suspended/banned authors yield
		// "unavailable"; all others yield "active".
		lifecycle := string(viewercontext.CoarsenLifecycle(accountStatus, isDeleted))

		result[userID] = &CommentAuthorInfo{
			Username:  username,
			AvatarURL: avatarURL,
			Lifecycle: lifecycle,
		}
	}

	return result
}

// filterBlockedComments resolves the bidirectional block set for viewerID
// against comment authors, then delegates to applyCommentBlockFilter.
// Fail-open: block query errors are logged and the unfiltered slice is returned.
//
// Pagination note: the returned slice may be shorter than the original; the
// caller forwards next_cursor from the repository so the client can keep
// paginating through remaining unblocked results.
func (h *CommentHandler) filterBlockedComments(
	ctx context.Context,
	viewerID uuid.UUID,
	comments []*entity.Comment,
) []*entity.Comment {
	if len(comments) == 0 {
		return comments
	}

	// Collect unique author IDs.
	seen := make(map[uuid.UUID]bool, len(comments))
	authorIDs := make([]uuid.UUID, 0, len(comments))
	for _, c := range comments {
		if !seen[c.AuthorID] {
			seen[c.AuthorID] = true
			authorIDs = append(authorIDs, c.AuthorID)
		}
	}

	// Single bulk query: find which candidate author IDs are in a bidirectional
	// block relationship with the viewer.
	rows, err := h.db.Pool().Query(ctx, `
		SELECT DISTINCT
			CASE
				WHEN blocker_id = $1 THEN blocked_id
				ELSE blocker_id
			END AS blocked_author
		FROM user_blocks
		WHERE (blocker_id = $1 AND blocked_id = ANY($2))
		   OR (blocker_id = ANY($2) AND blocked_id = $1)
	`, viewerID, authorIDs)
	if err != nil {
		h.log.Warn("block filter query failed, returning unfiltered comments", zap.Error(err))
		return comments
	}
	defer rows.Close()

	blockedSet := make(map[uuid.UUID]bool)
	for rows.Next() {
		var blockedAuthorID uuid.UUID
		if err := rows.Scan(&blockedAuthorID); err != nil {
			h.log.Warn("failed to scan blocked author ID in comment filter", zap.Error(err))
			continue
		}
		blockedSet[blockedAuthorID] = true
	}
	if rows.Err() != nil {
		h.log.Warn("block filter row iteration error, returning unfiltered comments", zap.Error(rows.Err()))
		return comments
	}

	return applyCommentBlockFilter(comments, blockedSet)
}

// applyCommentBlockFilter removes comments whose author is in blockedSet.
// Package-level for unit-testability. Returns the original slice unchanged
// when blockedSet is empty.
func applyCommentBlockFilter(comments []*entity.Comment, blockedSet map[uuid.UUID]bool) []*entity.Comment {
	if len(blockedSet) == 0 {
		return comments
	}
	filtered := make([]*entity.Comment, 0, len(comments))
	for _, c := range comments {
		if !blockedSet[c.AuthorID] {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

// CreateCommerceReferenceCommentRequest holds the request body for creating a
// commerce reference comment.
type CreateCommerceReferenceCommentRequest struct {
	ResourceReference *CreateCommerceReferenceRequest `json:"resource_reference" binding:"required"`
	Body              string                          `json:"body,omitempty"`
}

// CreateCommerceReferenceRequest carries the typed commerce identity.
type CreateCommerceReferenceRequest struct {
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	Preview      any    `json:"preview,omitempty"`
}

// CreateCommerceReferenceComment handles POST /api/v1/contents/{id}/comments/reference
//
// Authorization:
// - Only sellers can add commerce reference comments
// - Backend enforces ownership and market authority
//
// Requires Idempotency-Key header for safe retries.
func (h *CommentHandler) CreateCommerceReferenceComment(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse content ID
	idStr := c.Param("id")
	contentID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid content ID")
		return
	}

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
	var req CreateCommerceReferenceCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Validate resource_reference
	if req.ResourceReference == nil {
		response.BadRequest(c, "resource_reference is required")
		return
	}

	resourceType := entity.ResourceType(req.ResourceReference.ResourceType)
	if !resourceType.IsValid() {
		response.BadRequest(c, "Invalid resource_type")
		return
	}

	if !resourceType.CanDirectCommerceInsert() {
		response.BadRequest(c, "Only fixed-price sale or auction resource types are supported for commerce reference comments")
		return
	}

	// Validate resource ID format
	resourceID, err := uuid.Parse(req.ResourceReference.ResourceID)
	if err != nil {
		response.BadRequest(c, "Invalid resource_id format")
		return
	}

	// Prepare input with canonical commerce identity.
	input := contentApp.CommerceReferenceInput{
		TargetID:     contentID,
		ResourceType: resourceType,
		ResourceID:   resourceID,
	}

	// Add optional body
	if req.Body != "" {
		body := req.Body
		input.Body = &body
	}

	// Execute create within transaction
	var newComment *entity.Comment
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		newComment, err = h.commentService.AddCommerceReferenceComment(ctx, tx, userID, input, idempotencyKey)
		return err
	})

	if err != nil {
		h.log.Error("Failed to create commerce reference comment",
			zap.String("content_id", contentID.String()),
			zap.String("resource_id", req.ResourceReference.ResourceID),
			zap.String("resource_type", req.ResourceReference.ResourceType),
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)

		// Check for specific errors
		if _, ok := err.(*entity.ErrInvalidComment); ok {
			response.BadRequest(c, "Invalid request")
			return
		}
		if _, ok := err.(*entity.ErrCommerceReferenceOnPost); ok {
			response.BadRequest(c, "Invalid request")
			return
		}
		if errors.Is(err, entity.ErrIdempotencyConflict) {
			response.Error(c, http.StatusConflict, "IDEMPOTENCY_CONFLICT", "Idempotency key already used for a different operation")
			return
		}

		response.InternalServerError(c, "Failed to create commerce reference comment")
		return
	}

	// C-RESP — POST .../comments/reference returns the canonical snake_case
	// CommentResponse, consistent with the normal-comment create and list.
	response.Created(c, h.buildCreateCommentResponse(ctx, newComment))
}
