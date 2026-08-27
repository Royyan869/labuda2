package http

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/response"
	likeapp "github.com/labuda/backend/internal/social/like/application"
	likeentity "github.com/labuda/backend/internal/social/like/entity"
	likerepo "github.com/labuda/backend/internal/social/like/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// LikeHandler handles HTTP requests for like operations.
//
// Content likes route through LikeService (block check + content validation +
// outbox event emission). Comment likes route through CommentLikeService
// (comment/content lifecycle checks + block checks).
type LikeHandler struct {
	log                *zap.Logger
	repo               *likerepo.TargetLikeRepository
	db                 *db.DB
	likeService        *likeapp.Service
	commentLikeService *likeapp.CommentLikeService
}

// NewLikeHandler creates a new LikeHandler.
func NewLikeHandler(database *db.DB, log *zap.Logger, likeService *likeapp.Service, commentLikeService *likeapp.CommentLikeService) *LikeHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &LikeHandler{
		log:                log,
		repo:               likerepo.NewTargetLikeRepository(),
		db:                 database,
		likeService:        likeService,
		commentLikeService: commentLikeService,
	}
}

// ToggleLikeRequest represents the request body for toggling a like.
type ToggleLikeRequest struct {
	TargetID   string `json:"target_id" binding:"required"`
	TargetType string `json:"target_type" binding:"required,oneof=content comment"`
}

// LikeStatsResponse represents the response for like statistics.
type LikeStatsResponse struct {
	TargetID   string `json:"target_id"`
	TargetType string `json:"target_type"`
	Count      int    `json:"count"`
	IsLiked    bool   `json:"is_liked"`
}

// ToggleLike handles POST /api/v1/likes/toggle
//
// Toggles a like on supported target types (content, comment).
// Returns the new like state (true=liked, false=unliked).
//
// Content likes route through LikeService for governance:
//   - Block check (bidirectional)
//   - Deleted content guard
//   - Outbox event emission (content.liked)
//
// Comment likes route through CommentLikeService for governance:
//   - Comment existence and soft-delete guard
//   - Parent content existence and deleted-content guard
//   - Block checks against comment author and parent content author
func (h *LikeHandler) ToggleLike(c *gin.Context) {
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

	// Parse request
	var req ToggleLikeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: target_id and target_type are required")
		return
	}

	// Parse target ID
	targetID, err := uuid.Parse(req.TargetID)
	if err != nil {
		response.BadRequest(c, "Invalid target_id format")
		return
	}

	// Validate and convert target type
	targetType := likeentity.TargetType(req.TargetType)
	if !h.isValidTargetType(targetType) {
		response.BadRequest(c, "Invalid target_type. Must be one of: content, comment")
		return
	}

	switch targetType {
	case likeentity.TargetTypeContent:
		h.toggleContentLike(c, ctx, targetID, targetType, userID)
	case likeentity.TargetTypeComment:
		h.toggleCommentLike(c, ctx, targetID, targetType, userID)
	}
}

// toggleContentLike routes through LikeService for full governance.
func (h *LikeHandler) toggleContentLike(c *gin.Context, ctx context.Context, targetID uuid.UUID, targetType likeentity.TargetType, userID uuid.UUID) {
	result, err := h.likeService.ToggleContentLike(ctx, targetID, userID)
	if err != nil {
		var notFoundErr *likeentity.ErrContentNotFound
		var deletedErr *likeentity.ErrContentDeleted
		switch {
		case errors.As(err, &notFoundErr):
			response.NotFound(c, "Content not found")
		case errors.As(err, &deletedErr):
			response.Gone(c, "Content has been deleted")
		default:
			h.log.Error("Failed to toggle content like",
				zap.String("target_id", targetID.String()),
				zap.String("user_id", userID.String()),
				zap.Error(err),
			)
			response.InternalServerError(c, "Failed to toggle like")
		}
		return
	}

	response.SuccessWithMessage(c, "Like toggled successfully", gin.H{
		"target_id":   targetID,
		"target_type": targetType,
		"liked":       result.Liked,
		"count":       result.Count,
	})
}

// toggleCommentLike routes through CommentLikeService for full governance.
func (h *LikeHandler) toggleCommentLike(c *gin.Context, ctx context.Context, targetID uuid.UUID, targetType likeentity.TargetType, userID uuid.UUID) {
	result, err := h.commentLikeService.ToggleCommentLike(ctx, targetID, userID)

	if err != nil {
		var notFoundErr *likeentity.ErrTargetNotFound
		var contentNotFoundErr *likeentity.ErrContentNotFound
		var deletedErr *likeentity.ErrContentDeleted
		switch {
		case errors.As(err, &notFoundErr):
			response.NotFound(c, "Comment not found")
		case errors.As(err, &contentNotFoundErr):
			response.NotFound(c, "Content not found")
		case errors.As(err, &deletedErr):
			response.Gone(c, "Content has been deleted")
		default:
			h.log.Error("Failed to toggle comment like",
				zap.String("target_id", targetID.String()),
				zap.String("user_id", userID.String()),
				zap.Error(err),
			)
			response.InternalServerError(c, "Failed to toggle like")
		}
		return
	}

	response.SuccessWithMessage(c, "Like toggled successfully", gin.H{
		"target_id":   targetID,
		"target_type": targetType,
		"liked":       result.Liked,
		"count":       result.Count,
	})
}

// GetLikeStats handles GET /api/v1/likes/stats
//
// Query params: target_id, target_type
func (h *LikeHandler) GetLikeStats(c *gin.Context) {
	ctx := c.Request.Context()

	targetIDStr := c.Query("target_id")
	targetTypeStr := c.Query("target_type")

	if targetIDStr == "" || targetTypeStr == "" {
		response.BadRequest(c, "target_id and target_type query parameters are required")
		return
	}

	targetID, err := uuid.Parse(targetIDStr)
	if err != nil {
		response.BadRequest(c, "Invalid target_id format")
		return
	}

	targetType := likeentity.TargetType(targetTypeStr)
	if !h.isValidTargetType(targetType) {
		response.BadRequest(c, "Invalid target_type. Must be one of: content, comment")
		return
	}

	// Get stats
	var count int
	var isLiked bool

	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		// Governance: disclose like metadata only when the target is visible to
		// this viewer. Uses the SAME visibility authority as the content
		// detail / comment surfaces (deleted or hidden content, blocked
		// relationships) so /likes/stats cannot be used to bypass it.
		viewerIDVal, hasViewer := c.Get("userID")
		viewerID := uuid.Nil
		if hasViewer {
			if parsed, ok := viewerIDVal.(uuid.UUID); ok {
				viewerID = parsed
			}
		}

		var visible bool
		switch targetType {
		case likeentity.TargetTypeContent:
			visible, err = h.likeService.IsContentLikeReadable(ctx, tx, targetID, viewerID)
			if err != nil {
				return err
			}
		case likeentity.TargetTypeComment:
			visible, err = h.commentLikeService.IsCommentLikeReadable(ctx, tx, targetID, viewerID)
			if err != nil {
				return err
			}
		}

		if !visible {
			return likeentity.ErrLikeStatsInaccessible
		}

		// Get like count
		count, err = h.repo.CountLikes(ctx, tx, targetID, targetType)
		if err != nil {
			return err
		}

		// Check if current user liked this target
		if hasViewer {
			if userID, ok := viewerIDVal.(uuid.UUID); ok {
				isLiked, err = h.repo.ExistsLike(ctx, tx, targetID, targetType, userID)
				return err
			}
		}
		return nil
	})

	if err != nil {
		if errors.Is(err, likeentity.ErrLikeStatsInaccessible) {
			response.NotFound(c, "Target not found or not accessible")
			return
		}
		h.log.Error("Failed to get like stats",
			zap.String("target_type", string(targetType)),
			zap.String("target_id", targetID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to get like stats")
		return
	}

	response.Success(c, LikeStatsResponse{
		TargetID:   targetID.String(),
		TargetType: targetTypeStr,
		Count:      count,
		IsLiked:    isLiked,
	})
}

// =============================================================================
// PRIVATE METHODS
// ============================================================================

func (h *LikeHandler) isValidTargetType(targetType likeentity.TargetType) bool {
	switch targetType {
	case likeentity.TargetTypeContent,
		likeentity.TargetTypeComment:
		return true
	default:
		return false
	}
}
