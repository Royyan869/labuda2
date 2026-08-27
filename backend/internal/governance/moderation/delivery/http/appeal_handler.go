package http

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/capability"
	appealApp "github.com/labuda/backend/internal/governance/moderation/application"
	appealEntity "github.com/labuda/backend/internal/governance/moderation/entity"
	moderationEntity "github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// AppealHandler handles HTTP requests for appeal operations.
//
// Appeals allow users to contest moderation decisions.
// W14-B3: Added audit logging for all appeal review actions.
type AppealHandler struct {
	appealService *appealApp.AppealService
	db            db.Transactor
	log           *zap.Logger
	// W14-B3: Audit logger for tracking admin appeal reviews
	adminAuditLogger AdminAuditLogger
}

// NewAppealHandler creates a new AppealHandler.
// W14-B3: Added adminAuditLogger parameter for audit logging.
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
type CreateAppealRequest struct {
	CaseID string `json:"case_id" binding:"required,uuid"`
	Message  string `json:"message" binding:"required,min=1,max=2000"`
}

// ReviewAppealRequest represents the request body for reviewing an appeal.
type ReviewAppealRequest struct {
	Decision       string `json:"decision" binding:"required,oneof=approve reject approved rejected"`
	AdminResponse  string `json:"admin_response" binding:"omitempty,max=2000"`
}

// ============================================================================
// USER ENDPOINTS - Create Appeal
// ============================================================================

// CreateAppeal handles POST /api/v1/appeals
//
// Allows authenticated users to create an appeal for a moderation decision.
//
// Terminology:
//   - User REPORTS content → Creates a moderation CASE
//   - Admin reviews CASE → Makes decision (approve/reject/enforce)
//   - User contests decision → Creates an APPEAL
//
// Suspension note: This route is RequireAuth only (NOT RequireActiveAccount).
// Suspended users must be allowed to appeal their own account suspension.
// Adding RequireActiveAccount here would block legitimate suspension appeals.
//
// Request body:
//   - case_id: UUID of the moderation case being appealed
//   - message: User's explanation for the appeal
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

	// Parse case ID
	caseID, err := uuid.Parse(req.CaseID)
	if err != nil {
		response.BadRequest(c, "Invalid case_id")
		return
	}

	// Create appeal
	var appeal *appealEntity.Appeal
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		appeal, err = h.appealService.CreateAppeal(ctx, tx, caseID, userID, req.Message)
		return err
	})

	if err != nil {
		switch err.(type) {
		case *appealEntity.ErrCaseNotFound:
			response.NotFound(c, "Moderation case not found")
		case *appealEntity.ErrCaseNotAppealable:
			response.BadRequest(c, "This case is not in an appealable state")
		case *appealEntity.ErrNotResourceOwner:
			response.Forbidden(c, "You can only appeal decisions on your own content")
		case *appealEntity.ErrDuplicatePendingAppeal:
			response.Conflict(c, "An appeal for this case is already pending")
		case *appealEntity.ErrUnsupportedResourceType:
			response.BadRequest(c, "Appeals are not supported for this resource type. Supported: content, comment, for_sale, auction, user")
		default:
			h.log.Error("Failed to create appeal",
				zap.String("user_id", userID.String()),
				zap.String("case_id", req.CaseID),
				zap.Error(err),
			)
			response.InternalServerError(c, "Failed to create appeal")
		}
		return
	}

	response.Created(c, gin.H{
		"id":         appeal.ID,
		"case_id":  appeal.CaseID,
		"status":     string(appeal.Status),
		"message":    appeal.Message,
		"created_at": appeal.CreatedAt,
	})
}

// GetAppeal handles GET /api/v1/appeals/:id
//
// Returns details of a specific appeal. Only the appeal owner can access
// this route. Admins use GET /api/v1/admin/appeals/:id instead.
//
// Security: ownership is verified after fetch. Returns 404 (not 403) to
// not reveal the existence of other users' appeals (IDOR prevention).
func (h *AppealHandler) GetAppeal(c *gin.Context) {
	ctx := c.Request.Context()

	// Get authenticated user ID from context
	userID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse appeal ID from URL
	appealIDStr := c.Param("id")
	appealID, err := uuid.Parse(appealIDStr)
	if err != nil {
		response.BadRequest(c, "Invalid appeal ID")
		return
	}

	// Get appeal from service
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
	// Use 404 (not 403) to not reveal existence of other users' appeals.
	if appeal.AppealedBy != userID {
		response.NotFound(c, "Appeal not found")
		return
	}

	response.Success(c, gin.H{
		"appeal": h.appealToResponse(appeal),
	})
}

// ListMyAppeals handles GET /api/v1/appeals/me
//
// Returns all appeals created by the authenticated user.
//
// Query parameters:
//   - page: page number (default: 1)
//   - limit: items per page (default: 20, max: 100)
func (h *AppealHandler) ListMyAppeals(c *gin.Context) {
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

	// Get appeals from service
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

	// Convert to response format
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
//
// Returns all appeals for admin review.
//
// Query parameters:
//   - status: filter by status (optional: "pending", "approved", "rejected")
//   - page: page number (default: 1)
//   - limit: items per page (default: 20, max: 100)
func (h *AppealHandler) AdminListAppeals(c *gin.Context) {
	ctx := c.Request.Context()

	// Get authenticated admin ID from context
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse query parameters
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

	// Parse status filter
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

	// Get appeals from service
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

	// Convert to response format
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
//
// Returns pending appeals awaiting admin review.
//
// Query parameters:
//   - page: page number (default: 1)
//   - limit: items per page (default: 20, max: 100)
func (h *AppealHandler) AdminListPendingAppeals(c *gin.Context) {
	ctx := c.Request.Context()

	// Get authenticated admin ID from context
	adminID, ok := middleware.MustGetUserIDFromContext(c)
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

	// Get appeals from service
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

	// Convert to response format
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
// Returns details of a specific appeal with original moderation case context.
// W1-B2: Operational hardening - provides original case context for appeal review.
func (h *AppealHandler) AdminGetAppeal(c *gin.Context) {
	ctx := c.Request.Context()

	// Get authenticated admin ID from context
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse appeal ID from URL
	appealIDStr := c.Param("id")
	appealID, err := uuid.Parse(appealIDStr)
	if err != nil {
		response.BadRequest(c, "Invalid appeal ID")
		return
	}

	// Get appeal with original case from service
	var appeal *appealEntity.Appeal
	var originalCase *moderationEntity.GovernanceCase
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		appeal, originalCase, err = h.appealService.GetAppealWithCase(ctx, tx, appealID)
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
	if originalCase != nil {
		resp["original_case"] = h.governanceCaseToContext(originalCase)
	}

	response.Success(c, gin.H{
		"appeal": resp,
	})
}

// AdminReviewAppeal handles PUT /api/v1/admin/appeals/:id/review
//
// Allows admins to review an appeal and apply a decision.
//
// Request body:
//   - decision: "approve" | "reject"
//   - admin_response: optional admin response
//
// SLICE 7: Handler-level defense - requires moderation.appeal.review capability
func (h *AppealHandler) AdminReviewAppeal(c *gin.Context) {
	ctx := c.Request.Context()

	// SLICE 7: Defense-in-depth - Check capability at handler level
	// This provides defense even if middleware is bypassed
	if !capability.HasCapability(ctx, capability.CapModerationAppealReview.String()) {
		response.Forbidden(c, "Insufficient permissions: moderation.appeal.review capability required")
		return
	}

	// Get authenticated admin ID from context
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse appeal ID from URL
	appealIDStr := c.Param("id")
	appealID, err := uuid.Parse(appealIDStr)
	if err != nil {
		response.BadRequest(c, "Invalid appeal ID")
		return
	}

	// Parse request body
	var req ReviewAppealRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Convert decision to boolean
	approved := req.Decision == "approve" || req.Decision == "approved"

	var adminResponse *string
	if req.AdminResponse != "" {
		adminResponse = &req.AdminResponse
	}

	// Review appeal
	var appeal *appealEntity.Appeal
	var previousStatus appealEntity.AppealStatus
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		// Fetch current status before review for audit trail
		appeal, err = h.appealService.GetAppeal(ctx, tx, appealID)
		if err == nil {
			previousStatus = appeal.Status
		}

		var err error
		appeal, err = h.appealService.ReviewAppeal(ctx, tx, appealID, adminID, approved, adminResponse)
		return err
	})

	if err != nil {
		// Check for specific error types
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

	// W14-B3: Log appeal review to audit trail
	h.adminAuditLogger.LogSafe(ctx, adminID,
		"appeal_reviewed", "appeal", appealID,
		map[string]interface{}{
			"decision":        req.Decision,
			"approved":        approved,
			"previous_status": string(previousStatus),
			"new_status":      string(appeal.Status),
			"case_id":       appeal.CaseID.String(),
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

// governanceCaseToContext converts a GovernanceCase to minimal context for appeal review.
// W1-B2: Provides just enough context for admins to make informed appeal decisions.
func (h *AppealHandler) governanceCaseToContext(kase *moderationEntity.GovernanceCase) gin.H {
	return gin.H{
		"id":            kase.ID,
		"resource_type": string(kase.ResourceType),
		"resource_id":   kase.ResourceID,
		"status":        string(kase.Status),
		"reason":        kase.Reason,
		"created_at":    kase.CreatedAt,
		// Decision status helps appeal reviewer understand original outcome
		"decision_status": mapStatusToDecision(kase.Status),
	}
}

// mapStatusToDecision converts moderation case status to human-readable decision.
// Note: "dismissed" refers to the CASE being dismissed (not the original report)
func mapStatusToDecision(status moderationEntity.GovernanceCaseStatus) string {
	switch status {
	case moderationEntity.GovernanceCaseStatusApproved:
		return "approved" // Content was kept
	case moderationEntity.GovernanceCaseStatusRejected:
		return "dismissed" // Case was dismissed (false positive)
	case moderationEntity.GovernanceCaseStatusEnforced:
		return "enforced" // Content was removed
	default:
		return string(status)
	}
}

// appealToResponse converts an Appeal entity to a response map.
func (h *AppealHandler) appealToResponse(appeal *appealEntity.Appeal) gin.H {
	resp := gin.H{
		"id":         appeal.ID,
		"case_id":  appeal.CaseID,
		"status":     string(appeal.Status),
		"message":    appeal.Message,
		"created_at": appeal.CreatedAt,
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


