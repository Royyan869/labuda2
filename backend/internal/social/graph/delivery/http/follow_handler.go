package http

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/response"
	socialApp "github.com/labuda/backend/internal/social/graph/application"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// FollowHandler handles HTTP requests for follow operations.
type FollowHandler struct {
	socialService *socialApp.SocialService
	db            *db.DB
	log           *zap.Logger
}

// NewFollowHandler creates a new FollowHandler.
func NewFollowHandler(
	socialService *socialApp.SocialService,
	db *db.DB,
	log *zap.Logger,
) *FollowHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &FollowHandler{
		socialService: socialService,
		db:            db,
		log:           log,
	}
}

// FollowUser handles POST /api/v1/users/{id}/follow
//
// Authorization:
// - Any active user can follow another user
// - Cannot follow self
//
// Idempotent behavior:
// - No error if already following
func (h *FollowHandler) FollowUser(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse target user ID
	idStr := c.Param("id")
	targetUserID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
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

	// Validate: cannot follow self
	if userID == targetUserID {
		response.BadRequest(c, "Cannot follow yourself")
		return
	}

	// Execute follow (service handles idempotency within its own transaction)
	err = h.socialService.Follow(ctx, userID, targetUserID)
	if err != nil {
		h.log.Error("Failed to follow user",
			zap.String("follower_id", userID.String()),
			zap.String("following_id", targetUserID.String()),
			zap.Error(err),
		)

		// Check for block exists error
		if err.Error() == "cannot follow: block exists between users" {
			response.Forbidden(c, "Cannot follow this user")
			return
		}

		response.InternalServerError(c, "Failed to follow user")
		return
	}

	// Idempotent: Already following returns success
	response.SuccessWithMessage(c, "User followed successfully", gin.H{
		"user_id": targetUserID,
	})
}

// UnfollowUser handles DELETE /api/v1/users/{id}/follow
//
// Authorization:
// - Any active user can unfollow another user
//
// Idempotent behavior:
// - No error if not already following
func (h *FollowHandler) UnfollowUser(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse target user ID
	idStr := c.Param("id")
	targetUserID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
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

	// Execute unfollow (service handles idempotency within its own transaction)
	err = h.socialService.Unfollow(ctx, userID, targetUserID)
	if err != nil {
		h.log.Error("Failed to unfollow user",
			zap.String("follower_id", userID.String()),
			zap.String("following_id", targetUserID.String()),
			zap.Error(err),
		)

		response.InternalServerError(c, "Failed to unfollow user")
		return
	}

	// Idempotent: Not following returns success
	response.SuccessWithMessage(c, "User unfollowed successfully", gin.H{
		"user_id": targetUserID,
	})
}

// ListFollowersRequest holds the query parameters for listing followers.
type ListFollowersRequest struct {
	Limit  int     `form:"limit" binding:"omitempty,min=1,max=50"`
	Cursor *string `form:"cursor" binding:"omitempty"`
}

// ListFollowers handles GET /api/v1/users/{id}/followers
//
// Query parameters:
// - limit (optional): Number of results per page (default: 20, max: 50)
// - cursor (optional): RFC3339 timestamp for pagination
//
// Returns lifecycle-aware UserCard list for each follower. Deleted/suspended
// followers surface with lifecycle="removed"/"unavailable" rather than being
// dropped, preserving an accurate count signal for the caller.
func (h *FollowHandler) ListFollowers(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse user ID
	idStr := c.Param("id")
	userID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}

	// Parse query parameters
	var req ListFollowersRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}

	// Set default limit if not provided
	if req.Limit <= 0 {
		req.Limit = 20
	}

	// Parse cursor timestamp
	var cursorTime *time.Time
	if req.Cursor != nil && *req.Cursor != "" {
		parsedTime, err := time.Parse(time.RFC3339, *req.Cursor)
		if err == nil {
			cursorTime = &parsedTime
		}
	}

	followerIDs, err := h.socialService.ListFollowers(ctx, userID, req.Limit, cursorTime)
	if err != nil {
		h.log.Error("Failed to list followers",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve followers")
		return
	}

	followers, err := h.hydrateFollowUserCards(ctx, followerIDs)
	if err != nil {
		h.log.Error("Failed to hydrate followers",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve followers")
		return
	}

	response.Success(c, gin.H{
		"followers": followers,
		"limit":     req.Limit,
	})
}

// ListFollowingRequest holds the query parameters for listing following.
type ListFollowingRequest struct {
	Limit  int     `form:"limit" binding:"omitempty,min=1,max=50"`
	Cursor *string `form:"cursor" binding:"omitempty"`
}

// ListFollowing handles GET /api/v1/users/{id}/following
//
// Query parameters:
// - limit (optional): Number of results per page (default: 20, max: 50)
// - cursor (optional): RFC3339 timestamp for pagination
//
// Returns lifecycle-aware UserCard list for each followed user.
func (h *FollowHandler) ListFollowing(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse user ID
	idStr := c.Param("id")
	userID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}

	// Parse query parameters
	var req ListFollowingRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}

	// Set default limit if not provided
	if req.Limit <= 0 {
		req.Limit = 20
	}

	// Parse cursor timestamp
	var cursorTime *time.Time
	if req.Cursor != nil && *req.Cursor != "" {
		parsedTime, err := time.Parse(time.RFC3339, *req.Cursor)
		if err == nil {
			cursorTime = &parsedTime
		}
	}

	followingIDs, err := h.socialService.ListFollowing(ctx, userID, req.Limit, cursorTime)
	if err != nil {
		h.log.Error("Failed to list following",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve following")
		return
	}

	following, err := h.hydrateFollowUserCards(ctx, followingIDs)
	if err != nil {
		h.log.Error("Failed to hydrate following",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve following")
		return
	}

	response.Success(c, gin.H{
		"following": following,
		"limit":     req.Limit,
	})
}

// GetFollowStatus handles GET /api/v1/follows/status/:userId
//
// Returns the follow status between the authenticated user and the target user.
// Includes following, followed_by, muted, and blocked status.
func (h *FollowHandler) GetFollowStatus(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse target user ID
	idStr := c.Param("userId")
	targetUserID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}

	// Get authenticated user ID
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

	// Check following status (am I following them?)
	following, err := h.socialService.IsFollowing(ctx, userID, targetUserID)
	if err != nil {
		h.log.Error("Failed to check following status",
			zap.String("user_id", userID.String()),
			zap.String("target_id", targetUserID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to check follow status")
		return
	}

	// Check followed by status (are they following me?)
	followedBy, err := h.socialService.IsFollowing(ctx, targetUserID, userID)
	if err != nil {
		h.log.Error("Failed to check followed_by status",
			zap.String("user_id", userID.String()),
			zap.String("target_id", targetUserID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to check follow status")
		return
	}

	// Check blocked status (either direction)
	blocked, err := h.socialService.IsBlocked(ctx, userID, targetUserID)
	if err != nil {
		h.log.Error("Failed to check block status",
			zap.String("user_id", userID.String()),
			zap.String("target_id", targetUserID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to check block status")
		return
	}

	// Get mute status
	muted, err := h.socialService.IsMuted(ctx, userID, targetUserID)
	if err != nil {
		h.log.Error("Failed to check mute status",
			zap.String("user_id", userID.String()),
			zap.String("target_id", targetUserID.String()),
			zap.Error(err),
		)
		// Continue without mute status on error
		muted = false
	}

	response.Success(c, gin.H{
		"following":   following,
		"followed_by": followedBy,
		"mutual":      following && followedBy,
		"muted":       muted,
		"blocked":     blocked,
	})
}

// BlockUser handles POST /api/v1/users/:id/block
//
// Authorization:
// - Any active user can block another user
// - Cannot block self
//
// Idempotent behavior:
// - No error if already blocked
// - Removes follow relationships in both directions
func (h *FollowHandler) BlockUser(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse target user ID
	idStr := c.Param("id")
	targetUserID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}

	// Get authenticated user ID
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

	// Validate: cannot block self
	if userID == targetUserID {
		response.BadRequest(c, "Cannot block yourself")
		return
	}

	// Execute block (service handles follow cleanup)
	err = h.socialService.Block(ctx, userID, targetUserID)
	if err != nil {
		h.log.Error("Failed to block user",
			zap.String("blocker_id", userID.String()),
			zap.String("blocked_id", targetUserID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to block user")
		return
	}

	response.SuccessWithMessage(c, "User blocked successfully", gin.H{
		"user_id": targetUserID,
	})
}

// UnblockUser handles DELETE /api/v1/users/:id/block
//
// Authorization:
// - Any active user can unblock another user
//
// Idempotent behavior:
// - No error if not blocking
func (h *FollowHandler) UnblockUser(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse target user ID
	idStr := c.Param("id")
	targetUserID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}

	// Get authenticated user ID
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

	// Execute unblock
	err = h.socialService.Unblock(ctx, userID, targetUserID)
	if err != nil {
		h.log.Error("Failed to unblock user",
			zap.String("blocker_id", userID.String()),
			zap.String("blocked_id", targetUserID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to unblock user")
		return
	}

	response.SuccessWithMessage(c, "User unblocked successfully", gin.H{
		"user_id": targetUserID,
	})
}

// GetBlockedUsersRequest holds the query parameters for listing blocked users.
type GetBlockedUsersRequest struct {
	Limit  int     `form:"limit" binding:"omitempty,min=1,max=50"`
	Cursor *string `form:"cursor" binding:"omitempty"`
}

// GetBlockedUsers handles GET /api/v1/blocks
//
// Returns list of user IDs that the authenticated user has blocked.
func (h *FollowHandler) GetBlockedUsers(c *gin.Context) {
	ctx := c.Request.Context()

	// Get authenticated user ID
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

	// Parse query parameters
	var req GetBlockedUsersRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}

	// Set default limit if not provided
	if req.Limit <= 0 {
		req.Limit = 20
	}

	// Parse cursor timestamp
	var cursorTime *time.Time
	if req.Cursor != nil && *req.Cursor != "" {
		parsedTime, err := time.Parse(time.RFC3339, *req.Cursor)
		if err == nil {
			cursorTime = &parsedTime
		}
	}

	// Get blocked users
	blocked, err := h.socialService.ListBlocked(ctx, userID, req.Limit, cursorTime)
	if err != nil {
		h.log.Error("Failed to list blocked users",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve blocked users")
		return
	}

	response.Success(c, gin.H{
		"blocked": blocked,
		"limit":   req.Limit,
	})
}

// MuteUser handles POST /api/v1/users/:id/mute
//
// Authorization:
// - Any active user can mute another user
// - Cannot mute self
//
// Business rules:
// - Mute hides content but does NOT prevent interactions
// - Duplicate mute is safe (no error)
func (h *FollowHandler) MuteUser(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse target user ID
	idStr := c.Param("id")
	targetUserID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}

	// Get authenticated user ID
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

	// Validate: cannot mute self
	if userID == targetUserID {
		response.BadRequest(c, "Cannot mute yourself")
		return
	}

	// Execute mute
	err = h.socialService.Mute(ctx, userID, targetUserID)
	if err != nil {
		h.log.Error("Failed to mute user",
			zap.String("muter_id", userID.String()),
			zap.String("muted_id", targetUserID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to mute user")
		return
	}

	response.SuccessWithMessage(c, "User muted successfully", gin.H{
		"user_id": targetUserID,
	})
}

// UnmuteUser handles DELETE /api/v1/users/:id/mute
//
// Authorization:
// - Any active user can unmute another user
//
// Idempotent behavior:
// - No error if not muting
func (h *FollowHandler) UnmuteUser(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse target user ID
	idStr := c.Param("id")
	targetUserID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}

	// Get authenticated user ID
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

	// Execute unmute
	err = h.socialService.Unmute(ctx, userID, targetUserID)
	if err != nil {
		h.log.Error("Failed to unmute user",
			zap.String("muter_id", userID.String()),
			zap.String("muted_id", targetUserID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to unmute user")
		return
	}

	response.SuccessWithMessage(c, "User unmuted successfully", gin.H{
		"user_id": targetUserID,
	})
}

// GetMutedUsers handles GET /api/v1/mutes
//
// Returns list of user IDs that the authenticated user has muted.
func (h *FollowHandler) GetMutedUsers(c *gin.Context) {
	ctx := c.Request.Context()

	// Get authenticated user ID
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

	// Parse query parameters
	var req GetBlockedUsersRequest // Reuse the same struct
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}

	// Set default limit if not provided
	if req.Limit <= 0 {
		req.Limit = 20
	}

	// Parse cursor timestamp
	var cursorTime *time.Time
	if req.Cursor != nil && *req.Cursor != "" {
		parsedTime, err := time.Parse(time.RFC3339, *req.Cursor)
		if err == nil {
			cursorTime = &parsedTime
		}
	}

	// Get muted users
	muted, err := h.socialService.ListMuted(ctx, userID, req.Limit, cursorTime)
	if err != nil {
		h.log.Error("Failed to list muted users",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve muted users")
		return
	}

	response.Success(c, gin.H{
		"muted": muted,
		"limit": req.Limit,
	})
}
