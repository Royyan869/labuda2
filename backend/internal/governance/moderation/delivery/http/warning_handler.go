package http

import (
	"errors"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/audit"
	warningApp "github.com/labuda/backend/internal/governance/moderation/application"
	warningEntity "github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// WarningHandler handles HTTP requests for warning operations.
//
// Warnings are issued to users for policy violations.
type WarningHandler struct {
	warningService   *warningApp.WarningService
	db               db.Transactor
	log              *zap.Logger
	adminAuditLogger audit.AdminAuditLogger
}

// NewWarningHandler creates a new WarningHandler.
func NewWarningHandler(
	warningService *warningApp.WarningService,
	db db.Transactor,
	log *zap.Logger,
	adminAuditLogger audit.AdminAuditLogger,
) *WarningHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &WarningHandler{
		warningService:   warningService,
		db:               db,
		log:              log,
		adminAuditLogger: adminAuditLogger,
	}
}

// ============================================================================
// REQUEST/RESPONSE DTOs
// ============================================================================

// CreateWarningRequest represents the request body for issuing a warning.
type CreateWarningRequest struct {
	UserID    string `json:"user_id" binding:"required,uuid"`
	Level     string `json:"level" binding:"required,oneof=info warning severe"`
	Reason    string `json:"reason" binding:"required,min=1,max=500"`
	ExpiresAt *int64 `json:"expires_at,omitempty"` // Unix timestamp, optional
}

// ============================================================================
// USER ENDPOINTS - Get Warnings
// ============================================================================

// GetWarning handles GET /api/v1/warnings/:id
//
// Returns details of a specific warning.
func (h *WarningHandler) GetWarning(c *gin.Context) {
	ctx := c.Request.Context()

	// Get authenticated user ID from context
	userID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse warning ID from URL
	warningIDStr := c.Param("id")
	warningID, err := uuid.Parse(warningIDStr)
	if err != nil {
		response.BadRequest(c, "Invalid warning ID")
		return
	}

	// Get warning from service
	var warning *warningEntity.UserWarning
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		warning, err = h.warningService.GetWarning(ctx, tx, warningID)
		return err
	})

	if err != nil {
		h.log.Error("Failed to get warning",
			zap.String("user_id", userID.String()),
			zap.String("warning_id", warningIDStr),
			zap.Error(err),
		)
		response.NotFound(c, "Warning not found")
		return
	}

	// Verify the warning belongs to the user
	if warning.UserID != userID {
		response.Forbidden(c, "You do not have access to this warning")
		return
	}

	response.Success(c, gin.H{
		"warning": h.warningToResponse(warning),
	})
}

// ListWarnings handles GET /api/v1/warnings
//
// Returns all warnings for the authenticated user.
//
// Query parameters:
//   - page: page number (default: 1)
//   - limit: items per page (default: 20, max: 100)
func (h *WarningHandler) ListWarnings(c *gin.Context) {
	ctx := c.Request.Context()

	// Get authenticated user ID from context
	userID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse pagination parameters
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "20")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit

	// Get warnings from service
	var warnings []*warningEntity.UserWarning
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		warnings, err = h.warningService.ListWarningsByUser(ctx, tx, userID, limit, offset)
		return err
	})

	if err != nil {
		h.log.Error("Failed to get user warnings",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve warnings")
		return
	}

	// Convert to response format
	items := make([]gin.H, len(warnings))
	for i, w := range warnings {
		items[i] = h.warningToResponse(w)
	}

	response.Success(c, gin.H{
		"warnings": items,
		"page":     page,
		"limit":    limit,
		"count":    len(items),
	})
}

// GetActiveWarnings handles GET /api/v1/users/:id/warnings/active
//
// Returns active warnings for a specific user.
func (h *WarningHandler) GetActiveWarnings(c *gin.Context) {
	ctx := c.Request.Context()

	// Get authenticated user ID from context
	requesterID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse user ID from URL
	userIDStr := c.Param("id")
	targetUserID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}

	// Users can only view their own active warnings
	if requesterID != targetUserID {
		response.Forbidden(c, "You can only view your own active warnings")
		return
	}

	// Get active warnings from service
	var warnings []*warningEntity.UserWarning
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		warnings, err = h.warningService.ListActiveWarningsByUser(ctx, tx, targetUserID)
		return err
	})

	if err != nil {
		h.log.Error("Failed to get active warnings",
			zap.String("user_id", targetUserID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve active warnings")
		return
	}

	// Convert to response format
	items := make([]gin.H, len(warnings))
	for i, w := range warnings {
		items[i] = h.warningToResponse(w)
	}

	response.Success(c, gin.H{
		"warnings": items,
		"count":    len(items),
	})
}

// GetUserWarnings handles GET /api/v1/users/:id/warnings
//
// Returns all warnings for a specific user.
func (h *WarningHandler) GetUserWarnings(c *gin.Context) {
	ctx := c.Request.Context()

	// Get authenticated user ID from context
	requesterID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse user ID from URL
	userIDStr := c.Param("id")
	targetUserID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}

	// Parse pagination parameters
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "20")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit

	// Users can only view their own warnings
	if requesterID != targetUserID {
		response.Forbidden(c, "You can only view your own warnings")
		return
	}

	// Get warnings from service
	var warnings []*warningEntity.UserWarning
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		warnings, err = h.warningService.ListWarningsByUser(ctx, tx, targetUserID, limit, offset)
		return err
	})

	if err != nil {
		h.log.Error("Failed to get user warnings",
			zap.String("user_id", targetUserID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve warnings")
		return
	}

	// Convert to response format
	items := make([]gin.H, len(warnings))
	for i, w := range warnings {
		items[i] = h.warningToResponse(w)
	}

	response.Success(c, gin.H{
		"warnings": items,
		"page":     page,
		"limit":    limit,
		"count":    len(items),
	})
}

// ============================================================================
// ADMIN ENDPOINTS
// ============================================================================

// AdminListWarnings handles GET /api/v1/admin/warnings
//
// Lists all warnings with optional filters.
//
// Query parameters:
//   - user_id:   optional UUID to filter by target user
//   - is_active: optional bool ("true"/"false") to filter by active state
//   - page:      page number (default: 1)
//   - limit:     items per page (default: 20, max: 100)
func (h *WarningHandler) AdminListWarnings(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse optional user_id filter
	var userID *uuid.UUID
	if raw := c.Query("user_id"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			response.BadRequest(c, "Invalid user_id")
			return
		}
		userID = &parsed
	}

	// Parse is_active filter (optional)
	var isActiveFilter *bool
	if raw := c.Query("is_active"); raw != "" {
		active := raw == "true"
		isActiveFilter = &active
	}

	// Parse pagination
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "20")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	// Fetch from service
	var (
		warnings []*warningEntity.UserWarning
		total    int64
	)
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		warnings, total, err = h.warningService.ListAllWarnings(ctx, tx, userID, isActiveFilter, limit, offset)
		return err
	})
	if err != nil {
		h.log.Error("Failed to list admin warnings", zap.Error(err))
		response.InternalServerError(c, "Failed to retrieve warnings")
		return
	}

	items := make([]gin.H, len(warnings))
	for i, w := range warnings {
		items[i] = h.warningToResponse(w)
	}

	response.Success(c, gin.H{
		"warnings": items,
		"page":     page,
		"limit":    limit,
		"count":    total,
	})
}

// AdminIssueWarning handles POST /api/v1/warnings
//
// Allows admins to issue warnings to users.
func (h *WarningHandler) AdminIssueWarning(c *gin.Context) {
	ctx := c.Request.Context()

	// Get authenticated admin ID from context
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse request body
	var req CreateWarningRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Parse user ID
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		response.BadRequest(c, "Invalid user_id")
		return
	}

	// Convert level
	level := warningEntity.WarningLevel(req.Level)
	if !level.IsValid() {
		response.BadRequest(c, "Invalid level")
		return
	}

	// Issue warning
	var warning *warningEntity.UserWarning
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		warning, err = h.warningService.IssueWarning(ctx, tx, userID, adminID, level, req.Reason, req.ExpiresAt)
		return err
	})

	if err != nil {
		var targetErr *warningEntity.ErrWarningTargetNotFound
		if errors.As(err, &targetErr) {
			response.NotFound(c, "Warning target user not found")
			return
		}
		h.log.Error("Failed to issue warning",
			zap.String("admin_id", adminID.String()),
			zap.String("user_id", req.UserID),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to issue warning")
		return
	}

	response.Created(c, gin.H{
		"id":         warning.ID,
		"user_id":    warning.UserID,
		"level":      string(warning.Level),
		"reason":     warning.Reason,
		"is_active":  warning.IsActive,
		"created_at": warning.CreatedAt,
		"expires_at": warning.ExpiresAt,
	})

	if h.adminAuditLogger != nil {
		h.adminAuditLogger.LogSafe(ctx, adminID,
			"admin_warning_issued", "warning", warning.ID,
			map[string]interface{}{
				"user_id": warning.UserID.String(),
				"level":   string(warning.Level),
			},
		)
	}
}

// AdminRevokeWarning handles DELETE /api/v1/warnings/:id/revoke
//
// Allows admins to revoke an active warning.
func (h *WarningHandler) AdminRevokeWarning(c *gin.Context) {
	ctx := c.Request.Context()

	// Get authenticated admin ID from context
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse warning ID from URL
	warningIDStr := c.Param("id")
	warningID, err := uuid.Parse(warningIDStr)
	if err != nil {
		response.BadRequest(c, "Invalid warning ID")
		return
	}

	// Revoke warning
	var warning *warningEntity.UserWarning
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		warning, err = h.warningService.RevokeWarning(ctx, tx, warningID, adminID)
		return err
	})

	if err != nil {
		// Check for specific error types
		if _, ok := err.(*warningEntity.ErrWarningNotFound); ok {
			response.NotFound(c, "Warning not found")
			return
		}
		if _, ok := err.(*warningEntity.ErrWarningAlreadyRevoked); ok {
			response.BadRequest(c, "Warning has already been revoked")
			return
		}

		h.log.Error("Failed to revoke warning",
			zap.String("admin_id", adminID.String()),
			zap.String("warning_id", warningIDStr),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to revoke warning")
		return
	}

	response.Success(c, gin.H{
		"id":         warning.ID,
		"is_active":  warning.IsActive,
		"revoked_at": warning.RevokedAt,
	})

	if h.adminAuditLogger != nil {
		h.adminAuditLogger.LogSafe(ctx, adminID,
			"admin_warning_revoked", "warning", warning.ID,
			map[string]interface{}{
				"user_id": warning.UserID.String(),
			},
		)
	}
}

// ============================================================================
// HELPER METHODS
// ============================================================================

// warningToResponse converts a UserWarning entity to a response map.
func (h *WarningHandler) warningToResponse(warning *warningEntity.UserWarning) gin.H {
	resp := gin.H{
		"id":         warning.ID,
		"user_id":    warning.UserID,
		"level":      string(warning.Level),
		"reason":     warning.Reason,
		"is_active":  warning.IsActive,
		"status":     string(warning.GetStatus()),
		"created_at": warning.CreatedAt,
	}

	if warning.ExpiresAt != nil {
		resp["expires_at"] = warning.ExpiresAt
	}

	if warning.RevokedAt != nil {
		resp["revoked_at"] = *warning.RevokedAt
	}

	if warning.RevokedBy != nil {
		resp["revoked_by"] = *warning.RevokedBy
	}

	return resp
}

// parseExpiresAt converts an optional Unix timestamp to *time.Time.
func parseExpiresAt(timestamp *int64) *time.Time {
	if timestamp == nil {
		return nil
	}
	t := time.Unix(*timestamp, 0)
	return &t
}


