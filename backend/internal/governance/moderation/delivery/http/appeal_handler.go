package http

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/capability"
	appealApp "github.com/labuda/backend/internal/governance/moderation/application"
	appealEntity "github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// AppealHandler handles HTTP requests for appeal operations.
//
// Appeals allow users to contest governance Decisions.
// SLICE A: Canonical alignment — Appeal → Decision.
type AppealHandler struct {
	appealService    *appealApp.AppealService
	db               db.Transactor
	log              *zap.Logger
	adminAuditLogger AdminAuditLogger
}

// NewAppealHandler creates a new AppealHandler.
func NewAppealHandler(
	appealService *appealApp.AppealService,
	db db.Transactor,
	log *zap.Logger,
	adminAuditLogger AdminAuditLogger,
) *AppealHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &AppealHandler{
		appealService:    appealService,
		db:               db,
		log:              log,
		adminAuditLogger: adminAuditLogger,
	}
}

// ============================================================================
// REQUEST/RESPONSE DTOs
// ============================================================================

// CreateAppealRequest represents the request body for creating an appeal.
// SLICE A: The field is documented as decision_id but kept as case_id in the
// JSON contract for mobile/Admin frontend compatibility during Slice A.
// A later API slice will rename the field.
type CreateAppealRequest struct {
	DecisionID string `json:"decision_id" binding:"required,uuid"`
	Message    string `json:"message" binding:"required,min=1,max=2000"`
}

// ReviewAppealRequest represents the request body for reviewing an appeal.
type ReviewAppealRequest struct {
	Decision      string `json:"decision" binding:"required,oneof=approve reject approved rejected"`
	AdminResponse string `json:"admin_response" binding:"omitempty,max=2000"`
}

// ============================================================================
// USER ENDPOINTS - Create Appeal
// ============================================================================

// CreateAppeal handles POST /api/v1/appeals
//
// Allows authenticated users to create an appeal for a governance Decision.
//
// SLICE A: Canonical alignment — appeal targets Decision, not Case.
//
// Suspension note: This route is RequireAuth only (NOT RequireActiveAccount).
// Suspended users must be allowed to appeal their own account suspension.
func (h *AppealHandler) CreateAppeal(c *gin.Context) {
	ctx := c.Request.Context()

	// Get authenticated user ID from context
	userID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse request body
	var req CreateAppealRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Parse decision ID
	decisionID, err := uuid.Parse(req.DecisionID)
	if err != nil {
		response.BadRequest(c, "Invalid decision_id")
		return
	}

	// Create appeal
	var appeal *appealEntity.Appeal
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		appeal, err = h.appealService.CreateAppeal(ctx, tx, decisionID, userID, req.Message)
		return err
	})

	if err != nil {
		switch err.(type) {
		case *appealEntity.ErrDecisionNotFound:
			response.NotFound(c, "Decision not found")
		case *appealEntity.ErrDecisionNotAppealable:
			response.BadRequest(c, "This decision is not appealable (no consequences)")
		case *appealEntity.ErrNotResourceOwner:
			response.Forbidden(c, "You can only appeal decisions on your own content")
		case *appealEntity.ErrDuplicatePendingAppeal:
			response.Conflict(c, "An appeal for this decision is already pending")
		case *appealEntity.ErrUnsupportedResourceType:
			response.BadRequest(c, "Appeals are not supported for this resource type. Supported: content, comment, for_sale, auction, user")
		default:
			h.log.Error("Failed to create appeal",
				zap.String("user_id", userID.String()),
				zap.String("decision_id", req.DecisionID),
				zap.Error(err),
			)
			response.InternalServerError(c, "Failed to create appeal")
		}
		return
	}

	response.Created(c, gin.H{
		"id":          appeal.ID,
		"decision_id": appeal.DecisionID,
		"status":      string(appeal.Status),
		"message":     appeal.Message,
		"created_at":  appeal.CreatedAt,
	})
}

// GetAppeal handles GET /api/v1/appeals/:id
//
// Returns details of a specific appeal. Only the appeal owner can access
// this route. Admins use GET /api/v1/admin/appeals/:id instead.
func (h *AppealHandler) GetAppeal(c *gin.Context) {
	ctx := c.Request.Context()

	userID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	appealIDStr := c.Param("id")
	appealID, err := uuid.Parse(appealIDStr)
	if err != nil {
		response.BadRequest(c, "Invalid appeal ID")
		return
	}

	var appeal *appealEntity.Appeal
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		appeal, err = h.appealService.GetAppeal(ctx, tx, appealID)
		return err
	})

	if err != nil {
		h.log.Error("Failed to get appeal",
			zap.String("user_id", userID.String()),
			zap.String("appeal_id", appealIDStr),
			zap.Error(err),
		)
		response.NotFound(c, "Appeal not found")
		return
	}

	// Ownership check: only the appeal owner may read it.
	if appeal.AppealedBy != userID {
		response.NotFound(c, "Appeal not found")
		return
	}

	response.Success(c, gin.H{
		"appeal": h.appealToResponse(appeal),
	})
}

// ListMyAppeals handles GET /api/v1/appeals/me
func (h *AppealHandler) ListMyAppeals(c *gin.Context) {
	ctx := c.Request.Context()

	userID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

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

	var appeals []*appealEntity.Appeal
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		appeals, err = h.appealService.ListAppealsByUser(ctx, tx, userID, limit, offset)
		return err
	})

	if err != nil {
		h.log.Error("Failed to get user appeals",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve appeals")
		return
	}

	items := make([]gin.H, len(appeals))
	for i, a := range appeals {
		items[i] = h.appealToResponse(a)
	}

	response.Success(c, gin.H{
		"appeals": items,
		"page":    page,
		"limit":   limit,
		"count":   len(items),
	})
}

// ============================================================================
// ADMIN ENDPOINTS
// ============================================================================

// AdminListAppeals handles GET /api/v1/admin/appeals
func (h *AppealHandler) AdminListAppeals(c *gin.Context) {
	ctx := c.Request.Context()

	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	statusFilter := c.Query("status")
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

	var statusFilterPtr *appealEntity.AppealStatus
	if statusFilter != "" {
		status := appealEntity.AppealStatus(statusFilter)
		if status != appealEntity.AppealStatusPending &&
			status != appealEntity.AppealStatusApproved &&
			status != appealEntity.AppealStatusRejected {
			response.BadRequest(c, "Invalid status filter")
			return
		}
		statusFilterPtr = &status
	}

	var appeals []*appealEntity.Appeal
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		appeals, err = h.appealService.ListAllAppeals(ctx, tx, statusFilterPtr, limit, offset)
		return err
	})

	if err != nil {
		h.log.Error("Failed to list appeals",
			zap.String("admin_id", adminID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve appeals")
		return
	}

	items := make([]gin.H, len(appeals))
	for i, a := range appeals {
		items[i] = h.appealToResponse(a)
	}

	response.Success(c, gin.H{
		"appeals": items,
		"page":    page,
		"limit":   limit,
		"count":   len(items),
	})
}

// AdminListPendingAppeals handles GET /api/v1/admin/appeals/pending
func (h *AppealHandler) AdminListPendingAppeals(c *gin.Context) {
	ctx := c.Request.Context()

	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

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

	var appeals []*appealEntity.Appeal
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		appeals, err = h.appealService.ListPendingAppeals(ctx, tx, limit, offset)
		return err
	})

	if err != nil {
		h.log.Error("Failed to list pending appeals",
			zap.String("admin_id", adminID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve pending appeals")
		return
	}

	items := make([]gin.H, len(appeals))
	for i, a := range appeals {
		items[i] = h.appealToResponse(a)
	}

	response.Success(c, gin.H{
		"appeals": items,
		"page":    page,
		"limit":   limit,
		"count":   len(items),
	})
}

// AdminGetAppeal handles GET /api/v1/admin/appeals/:id
//
// SLICE A: Uses canonical AppealContext (Decision→Case) instead of GovernanceCase.
func (h *AppealHandler) AdminGetAppeal(c *gin.Context) {
	ctx := c.Request.Context()

	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	appealIDStr := c.Param("id")
	appealID, err := uuid.Parse(appealIDStr)
	if err != nil {
		response.BadRequest(c, "Invalid appeal ID")
		return
	}

	// SLICE A: Use GetAppealWithContext (canonical Decision→Case context)
	var appeal *appealEntity.Appeal
	var appealCtx *appealApp.AppealContext
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		appeal, appealCtx, err = h.appealService.GetAppealWithContext(ctx, tx, appealID)
		return err
	})

	if err != nil {
		h.log.Error("Failed to get appeal",
			zap.String("admin_id", adminID.String()),
			zap.String("appeal_id", appealIDStr),
			zap.Error(err),
		)
		response.NotFound(c, "Appeal not found")
		return
	}

	resp := h.appealToResponse(appeal)
	if appealCtx != nil {
		resp["original_case"] = h.appealContextToCaseResponse(appealCtx)
	}

	response.Success(c, gin.H{
		"appeal": resp,
	})
}

// AdminReviewAppeal handles PUT /api/v1/admin/appeals/:id/review
func (h *AppealHandler) AdminReviewAppeal(c *gin.Context) {
	ctx := c.Request.Context()

	// Defense-in-depth: Check capability at handler level
	if !capability.HasCapability(ctx, capability.CapModerationAppealReview.String()) {
		response.Forbidden(c, "Insufficient permissions: moderation.appeal.review capability required")
		return
	}

	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	appealIDStr := c.Param("id")
	appealID, err := uuid.Parse(appealIDStr)
	if err != nil {
		response.BadRequest(c, "Invalid appeal ID")
		return
	}

	var req ReviewAppealRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	approved := req.Decision == "approve" || req.Decision == "approved"

	var adminResponse *string
	if req.AdminResponse != "" {
		adminResponse = &req.AdminResponse
	}

	var appeal *appealEntity.Appeal
	var previousStatus appealEntity.AppealStatus
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		appeal, err = h.appealService.GetAppeal(ctx, tx, appealID)
		if err == nil {
			previousStatus = appeal.Status
		}

		var err error
		appeal, err = h.appealService.ReviewAppeal(ctx, tx, appealID, adminID, approved, adminResponse)
		return err
	})

	if err != nil {
		if _, ok := err.(*appealEntity.ErrAppealNotFound); ok {
			response.NotFound(c, "Appeal not found")
			return
		}
		if _, ok := err.(*appealEntity.ErrAppealAlreadyReviewed); ok {
			response.BadRequest(c, "Appeal has already been reviewed")
			return
		}

		h.log.Error("Failed to review appeal",
			zap.String("admin_id", adminID.String()),
			zap.String("appeal_id", appealIDStr),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to review appeal")
		return
	}

	// Log appeal review to audit trail
	h.adminAuditLogger.LogSafe(ctx, adminID,
		"appeal_reviewed", "appeal", appealID,
		map[string]interface{}{
			"decision":        req.Decision,
			"approved":        approved,
			"previous_status": string(previousStatus),
			"new_status":      string(appeal.Status),
			"decision_id":     appeal.DecisionID.String(),
			"admin_response":  req.AdminResponse,
		},
	)

	response.Success(c, gin.H{
		"id":          appeal.ID,
		"status":      string(appeal.Status),
		"reviewed_at": appeal.ReviewedAt,
	})
}

// ============================================================================
// HELPER METHODS
// ============================================================================

// appealContextToCaseResponse converts an AppealContext to the original_case
// response format. SLICE A: Uses canonical Decision→Case instead of GovernanceCase.
func (h *AppealHandler) appealContextToCaseResponse(appealCtx *appealApp.AppealContext) gin.H {
	if appealCtx == nil {
		return nil
	}

	resp := gin.H{}

	if appealCtx.Case != nil {
		resp["id"] = appealCtx.Case.ID
		resp["resource_type"] = string(appealCtx.Case.SubjectType)
		resp["resource_id"] = appealCtx.Case.SubjectID
		resp["status"] = string(appealCtx.Case.Status)
		resp["created_at"] = appealCtx.Case.CreatedAt
	}

	if appealCtx.Decision != nil {
		resp["decision_outcome"] = string(appealCtx.Decision.Outcome)
		resp["decision_id"] = appealCtx.Decision.ID
	}

	return resp
}

// appealToResponse converts an Appeal entity to a response map.
// SLICE A: Uses DecisionID instead of CaseID.
func (h *AppealHandler) appealToResponse(appeal *appealEntity.Appeal) gin.H {
	resp := gin.H{
		"id":          appeal.ID,
		"decision_id": appeal.DecisionID,
		"status":      string(appeal.Status),
		"message":     appeal.Message,
		"created_at":  appeal.CreatedAt,
	}

	if appeal.AdminResponse != nil {
		resp["admin_response"] = *appeal.AdminResponse
	}

	if appeal.ReviewedBy != nil {
		resp["reviewed_by"] = *appeal.ReviewedBy
	}

	if appeal.ReviewedAt != nil {
		resp["reviewed_at"] = *appeal.ReviewedAt
	}

	return resp
}
