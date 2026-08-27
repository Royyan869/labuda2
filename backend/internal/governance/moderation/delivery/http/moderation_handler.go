package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	moderationApp "github.com/labuda/backend/internal/governance/moderation/application"
	moderationEntity "github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/capability"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// ModerationHandler handles HTTP requests for moderation operations.
//
// DOMAIN TERMINOLOGY:
// - REPORT: User action of flagging content (POST /api/v1/moderation/cases)
// - CASE: Internal moderation object created from a report
// - APPEAL: User contest of a moderation decision (handled by AppealHandler)
//
// Moderation handles user reports and moderation actions on platform content.
// This handler ONLY calls ModerationService - NO moderation logic is implemented here.
//
// MODERATION BOUNDARY: This handler does NOT modify orders, ledger, escrow,
// or financial balances. Moderation actions only affect social or fixed-price sale visibility.
//
// W14-B3: Added audit logging for all moderation actions.
type ModerationHandler struct {
	moderationService *moderationApp.ModerationService
	db                db.Transactor
	log               *zap.Logger
	// W14-B3: Audit logger for tracking all admin moderation actions
	adminAuditLogger AdminAuditLogger
}

// AdminAuditLogger defines interface for logging admin actions.
// W14-B3: Extracted from audit package to avoid circular dependency.
type AdminAuditLogger interface {
	LogSafe(ctx context.Context, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{})
	LogTx(ctx context.Context, tx db.Tx, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{}) error
}

// NewModerationHandler creates a new ModerationHandler.
// W14-B3: Added adminAuditLogger parameter for audit logging.
func NewModerationHandler(
	moderationService *moderationApp.ModerationService,
	db db.Transactor,
	log *zap.Logger,
	adminAuditLogger AdminAuditLogger,
) *ModerationHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &ModerationHandler{
		moderationService: moderationService,
		db:                db,
		log:               log,
		adminAuditLogger:  adminAuditLogger,
	}
}

// ============================================================================
// REQUEST/RESPONSE DTOs
// ============================================================================

// CreateCaseRequest represents the request body for creating a moderation case.
// All 6 resource types are supported: content, comment, for_sale, auction, user, chat_message.
type CreateCaseRequest struct {
	EntityType string `json:"entity_type" binding:"required,oneof=content comment for_sale auction user chat_message"`
	EntityID   string `json:"entity_id" binding:"required,uuid"`
	Reason     string `json:"reason" binding:"required,min=1,max=500"`
}

// ApplyActionRequest represents the request body for applying moderation action.
// Notes is optional for approve/reject but REQUIRED (non-empty) for enforce.
type ApplyActionRequest struct {
	Action string `json:"action" binding:"required,oneof=approve reject enforce"`
	Notes  string `json:"notes" binding:"omitempty,max=1000"`
}

// ResourcePreview contains minimal resource information for moderation review.
// This allows moderators to see what they're reviewing without additional API calls.
// W1-B2: Operational hardening - provides honest preview instead of blind resource_id.
type ResourcePreview struct {
	AuthorID                   string     `json:"author_id"`
	AuthorUsername             string     `json:"author_username,omitempty"` // Username from user_profiles
	Title                      string     `json:"title,omitempty"`           // Resource title or subject line
	Status                     string     `json:"status,omitempty"`          // Resource lifecycle/status
	ContentText                string     `json:"content_text,omitempty"`    // Truncated text preview (max 200 chars)
	ContentType                string     `json:"content_type,omitempty"`    // post/request for content, comment type for comments
	IsDeleted                  bool       `json:"is_deleted"`
	DeletedAt                  *time.Time `json:"deleted_at,omitempty"`
	DeletionReason             *string    `json:"deletion_reason,omitempty"`
	EvidenceAvailable          bool       `json:"evidence_available,omitempty"`
	EvidenceRequiresCapability string     `json:"evidence_requires_capability,omitempty"`

	// Chat-message-specific fields (omitted for other resource types)
	RoomID   string `json:"room_id,omitempty"`
	RoomType string `json:"room_type,omitempty"` // normal, support, negotiation
	SentAt   string `json:"sent_at,omitempty"`   // ISO 8601 timestamp of the message
}

// ModerationCaseEvidence contains the original hidden chat evidence returned
// by the dedicated evidence endpoint.
type ModerationCaseEvidence struct {
	CaseID             string                 `json:"case_id"`
	ResourceType       string                 `json:"resource_type"`
	ResourceID         string                 `json:"resource_id"`
	MessageID          string                 `json:"message_id"`
	RoomID             string                 `json:"room_id"`
	RoomType           string                 `json:"room_type"`
	SenderID           string                 `json:"sender_id"`
	AuthorUsername     string                 `json:"author_username,omitempty"`
	CreatedAt          string                 `json:"created_at"`
	DeletedAt          *time.Time             `json:"deleted_at,omitempty"`
	DeletionReason     *string                `json:"deletion_reason,omitempty"`
	OriginalBody       *string                `json:"original_body,omitempty"`
	OriginalAttachment map[string]interface{} `json:"original_attachment,omitempty"`
}

// ============================================================================
// USER ENDPOINTS - Create Case
// ============================================================================

// CreateCase handles POST /api/v1/moderation/cases
//
// DOMAIN TERMINOLOGY:
// - This endpoint creates a moderation CASE from a user REPORT
// - User reports content → System creates case → Admin reviews case
//
// Allows authenticated users to report content for moderation.
//
// Request body:
//   - entity_type: "content" | "comment" | "for_sale" | "auction" | "user" | "chat_message" (V1 supported)
//   - entity_id: UUID of the entity being reported
//   - reason: description of why this content is being reported
//
// Response:
//   - 201 Created: Case created successfully
//   - 409 Conflict: User has already reported this resource
//   - 404 Not Found: Resource does not exist
//   - 400 Bad Request: Unsupported resource type or invalid input
func (h *ModerationHandler) CreateCase(c *gin.Context) {
	ctx := c.Request.Context()

	// Get authenticated user ID from context
	userID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse request body
	var req CreateCaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Parse entity ID
	entityID, err := uuid.Parse(req.EntityID)
	if err != nil {
		response.BadRequest(c, "Invalid entity_id")
		return
	}

	// Convert entity_type to ResourceType
	resourceType := moderationEntity.ResourceType(req.EntityType)
	if !resourceType.IsValid() {
		response.BadRequest(c, "Invalid entity_type")
		return
	}

	// V1: content, comment, for_sale, auction, user, and chat_message are supported
	if resourceType != moderationEntity.ResourceTypeContent &&
		resourceType != moderationEntity.ResourceTypeComment &&
		resourceType != moderationEntity.ResourceTypeForSale &&
		resourceType != moderationEntity.ResourceTypeAuction &&
		resourceType != moderationEntity.ResourceTypeUser &&
		resourceType != moderationEntity.ResourceTypeChatMessage {
		response.BadRequest(c, "Resource type '"+req.EntityType+"' is not yet supported. Supported: content, comment, for_sale, auction, user, chat_message")
		return
	}

	// Call moderation service to create case (service owns its own transaction)
	kase, err := h.moderationService.CreateCase(ctx, resourceType, entityID, userID, req.Reason)
	if err != nil {
		// Check for duplicate report error
		if strings.Contains(err.Error(), "already reported") {
			response.Conflict(c, err.Error())
			return
		}

		// Check for PostgreSQL unique constraint violation (23505)
		// This handles potential future DB-level unique constraints
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			response.Conflict(c, "You have already reported this resource")
			return
		}

		// Check for resource not found error
		if strings.Contains(err.Error(), "resource not found") {
			response.NotFound(c, err.Error())
			return
		}

		// Check for unsupported type error
		if strings.Contains(err.Error(), "not yet supported") {
			response.BadRequest(c, err.Error())
			return
		}

		// Check for chat message report authorization errors
		if strings.Contains(err.Error(), "chat message report rejected") {
			response.BadRequest(c, err.Error())
			return
		}

		h.log.Error("Failed to create moderation case",
			zap.String("user_id", userID.String()),
			zap.String("entity_type", req.EntityType),
			zap.String("entity_id", req.EntityID),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to create case")
		return
	}

	response.Created(c, gin.H{
		"case_id":       kase.ID,
		"resource_type": string(kase.ResourceType),
		"resource_id":   kase.ResourceID,
		"status":        string(kase.Status),
		"created_at":    kase.CreatedAt,
	})
}

// ============================================================================
// USER ENDPOINTS - Get My Case
// ============================================================================

// GetMyCase handles GET /api/v1/moderation/cases/:id
//
// Returns a specific case ONLY if the authenticated user is the reporter.
// Does NOT include resource_preview (admin-only data).
func (h *ModerationHandler) GetMyCase(c *gin.Context) {
	ctx := c.Request.Context()

	userID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	caseIDStr := c.Param("id")
	caseID, err := uuid.Parse(caseIDStr)
	if err != nil {
		response.BadRequest(c, "Invalid case ID")
		return
	}

	kase, err := h.moderationService.GetCase(ctx, caseID)
	if err != nil {
		response.NotFound(c, "Moderation case not found")
		return
	}

	// Ownership check: user can only see their own reported cases
	if kase.ReportedBy != userID {
		response.NotFound(c, "Moderation case not found")
		return
	}

	// Return case without resource_preview (preview is admin-only)
	response.Success(c, gin.H{
		"case": h.caseToResponse(kase),
	})
}

// ============================================================================
// USER ENDPOINTS - My Cases
// ============================================================================

// GetMyCases handles GET /api/v1/moderation/my-cases
//
// Returns all moderation cases created by the authenticated user.
//
// Query parameters:
//   - page: page number (default: 1)
//   - limit: items per page (default: 20, max: 100)
func (h *ModerationHandler) GetMyCases(c *gin.Context) {
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

	// Get cases from service (service owns its own transaction)
	cases, err := h.moderationService.GetCasesByUser(ctx, userID, limit, offset)
	if err != nil {
		h.log.Error("Failed to get user cases",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve cases")
		return
	}

	// Convert to response format
	items := make([]gin.H, len(cases))
	for i, c := range cases {
		items[i] = h.caseToResponse(c)
	}

	response.Success(c, gin.H{
		"cases": items,
		"page":  page,
		"limit": limit,
		"count": len(items),
	})
}

// ============================================================================
// ADMIN ENDPOINTS - List Cases
// ============================================================================

// ListCases handles GET /api/v1/admin/moderation/cases
//
// Returns moderation cases for admin review.
//
// Query parameters:
//   - status: filter by status (optional: "pending", "approved", "rejected", "enforced")
//   - resource_type: optional resource type filter ("content", "comment", "for_sale", "auction", "user", "chat_message")
//   - page: page number (default: 1)
//   - limit: items per page (default: 20, max: 100)
func (h *ModerationHandler) ListCases(c *gin.Context) {
	ctx := c.Request.Context()

	// Get authenticated admin ID from context
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse query parameters
	statusFilter := c.Query("status")
	resourceTypeFilter := c.Query("resource_type")
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
	var statusFilterPtr *moderationEntity.GovernanceCaseStatus
	if statusFilter != "" {
		status := moderationEntity.GovernanceCaseStatus(statusFilter)
		if status != moderationEntity.GovernanceCaseStatusPending &&
			status != moderationEntity.GovernanceCaseStatusApproved &&
			status != moderationEntity.GovernanceCaseStatusRejected &&
			status != moderationEntity.GovernanceCaseStatusEnforced {
			response.BadRequest(c, "Invalid status filter")
			return
		}
		statusFilterPtr = &status
	}

	var resourceTypeFilterPtr *moderationEntity.ResourceType
	if resourceTypeFilter != "" {
		resourceType := moderationEntity.ResourceType(resourceTypeFilter)
		if !resourceType.IsValid() {
			response.BadRequest(c, "Invalid resource_type filter")
			return
		}
		resourceTypeFilterPtr = &resourceType
	}

	// Get cases from service (service owns its own transaction)
	cases, total, err := h.moderationService.ListCases(ctx, statusFilterPtr, resourceTypeFilterPtr, limit, offset)
	if err != nil {
		h.log.Error("Failed to list moderation cases",
			zap.String("admin_id", adminID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve cases")
		return
	}

	// Convert to response format
	items := make([]gin.H, len(cases))
	for i, c := range cases {
		items[i] = h.caseToResponse(c)
	}

	response.Success(c, gin.H{
		"cases": items,
		"page":  page,
		"limit": limit,
		"count": total,
	})
}

// ============================================================================
// ADMIN ENDPOINTS - Get Case
// ============================================================================

// GetCase handles GET /api/v1/admin/moderation/cases/:id
//
// Returns details of a specific moderation case.
// W1-B2: Enhanced to include resource_preview for operational honesty.
func (h *ModerationHandler) GetCase(c *gin.Context) {
	ctx := c.Request.Context()

	// Get authenticated admin ID from context
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse case ID from URL
	caseIDStr := c.Param("id")
	caseID, err := uuid.Parse(caseIDStr)
	if err != nil {
		response.BadRequest(c, "Invalid case ID")
		return
	}

	// Get case from service (service owns its own transaction)
	kase, err := h.moderationService.GetCase(ctx, caseID)
	if err != nil {
		h.log.Error("Failed to get moderation case",
			zap.String("admin_id", adminID.String()),
			zap.String("case_id", caseIDStr),
			zap.Error(err),
		)
		response.NotFound(c, "Moderation case not found")
		return
	}

	// Fetch resource preview for operational honesty
	preview := h.fetchResourcePreview(ctx, kase.ResourceType, kase.ResourceID)
	if preview == nil {
		h.log.Warn("Failed to fetch resource preview or resource not found",
			zap.String("case_id", caseIDStr),
		)
		// Continue without preview - UI should handle missing preview gracefully
	}

	resp := h.caseToResponse(kase)
	if preview != nil {
		resp["resource_preview"] = preview
	}

	response.Success(c, gin.H{
		"case": resp,
	})
}

// GetCaseEvidence handles GET /api/v1/admin/moderation/cases/:id/evidence.
//
// The dedicated evidence endpoint returns original hidden chat evidence only
// for chat_message cases. The case ID determines the message ID; callers cannot
// supply an arbitrary message ID.
func (h *ModerationHandler) GetCaseEvidence(c *gin.Context) {
	ctx := c.Request.Context()

	if !capability.HasCapability(ctx, capability.CapModerationEvidenceRead.String()) {
		response.Forbidden(c, "Insufficient permissions: moderation.evidence.read capability required")
		return
	}

	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	if h.moderationService == nil || h.db == nil || h.adminAuditLogger == nil {
		response.InternalServerError(c, "Evidence access is unavailable")
		return
	}

	caseIDStr := c.Param("id")
	caseID, err := uuid.Parse(caseIDStr)
	if err != nil {
		response.BadRequest(c, "Invalid case ID")
		return
	}

	kase, err := h.moderationService.GetCase(ctx, caseID)
	if err != nil {
		h.log.Error("Failed to get moderation case evidence",
			zap.String("admin_id", adminID.String()),
			zap.String("case_id", caseIDStr),
			zap.Error(err),
		)
		response.NotFound(c, "Moderation case not found")
		return
	}

	if kase.ResourceType != moderationEntity.ResourceTypeChatMessage {
		response.BadRequest(c, "Moderation evidence is only available for chat_message cases")
		return
	}

	var evidence *ModerationCaseEvidence
	if err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var loadErr error
		evidence, loadErr = h.fetchChatMessageEvidenceTx(ctx, tx, kase)
		if loadErr != nil {
			return loadErr
		}

		metadata := map[string]interface{}{
			"case_id":       kase.ID.String(),
			"resource_type": string(kase.ResourceType),
			"resource_id":   kase.ResourceID.String(),
			"message_id":    evidence.MessageID,
			"room_id":       evidence.RoomID,
			"room_type":     evidence.RoomType,
			"sender_id":     evidence.SenderID,
		}
		if evidence.AuthorUsername != "" {
			metadata["author_username"] = evidence.AuthorUsername
		}
		if evidence.DeletedAt != nil {
			metadata["deleted_at"] = evidence.DeletedAt.Format(time.RFC3339)
		}
		if evidence.DeletionReason != nil && *evidence.DeletionReason != "" {
			metadata["deletion_reason"] = *evidence.DeletionReason
		}

		return h.adminAuditLogger.LogTx(ctx, tx, adminID, "moderation.evidence.read", "moderation_case", kase.ID, metadata)
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.NotFound(c, "Chat message evidence not found")
			return
		}
		h.log.Error("Failed to read moderation evidence",
			zap.String("admin_id", adminID.String()),
			zap.String("case_id", caseIDStr),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to read evidence")
		return
	}

	response.Success(c, gin.H{
		"evidence": evidence,
	})
}

// ============================================================================
// ADMIN ENDPOINTS - Apply Action
// ============================================================================

// ApplyAction handles POST /api/v1/admin/moderation/cases/:id/action
//
// Allows admins to apply moderation decisions to a case.
//
// Request body:
//   - action: "approve" | "reject" | "enforce"
//   - notes: optional for approve/reject; REQUIRED (non-empty) for enforce
//
// Action meanings:
//   - approve: content complies, no action needed
//   - reject: case dismissed as false positive
//   - enforce: content violates guidelines, triggers soft-delete event (notes required)
//
// SLICE 6: Handler-level defense - requires moderation.case.resolve capability
func (h *ModerationHandler) ApplyAction(c *gin.Context) {
	ctx := c.Request.Context()

	// SLICE 6: Defense-in-depth - Check capability at handler level
	// This provides defense even if middleware is bypassed
	if !capability.HasCapability(ctx, capability.CapModerationCaseResolve.String()) {
		response.Forbidden(c, "Insufficient permissions: moderation.case.resolve capability required")
		return
	}

	// Get authenticated admin ID from context
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse case ID from URL
	caseIDStr := c.Param("id")
	caseID, err := uuid.Parse(caseIDStr)
	if err != nil {
		response.BadRequest(c, "Invalid case ID")
		return
	}

	// Parse request body
	var req ApplyActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Convert action to Decision
	var decision moderationEntity.Decision
	var notes *string

	switch req.Action {
	case "approve":
		decision = moderationEntity.DecisionApprove
	case "reject":
		decision = moderationEntity.DecisionReject
	case "enforce":
		decision = moderationEntity.DecisionEnforce
		// Handler-level guard: enforce requires non-empty notes.
		// Entity also validates this; this check avoids unnecessary DB work on invalid input.
		if strings.TrimSpace(req.Notes) == "" {
			response.BadRequest(c, "Enforce action requires non-empty notes for audit trail")
			return
		}
	default:
		response.BadRequest(c, "Invalid action")
		return
	}

	if req.Notes != "" {
		notes = &req.Notes
	}

	// Fetch current status before review for audit trail
	var previousStatus moderationEntity.GovernanceCaseStatus
	if existing, getErr := h.moderationService.GetCase(ctx, caseID); getErr == nil {
		previousStatus = existing.Status
	}

	// Apply action (service owns its own transaction: lock → transition → outbox → update)
	kase, err := h.moderationService.ReviewCase(ctx, caseID, adminID, decision, notes)
	if err != nil {
		// Check for specific error types
		if _, ok := err.(*moderationEntity.ErrAlreadyReviewed); ok {
			response.BadRequest(c, "Case has already been reviewed")
			return
		}
		if _, ok := err.(*moderationEntity.ErrInvalidTransition); ok {
			response.BadRequest(c, "Invalid case transition")
			return
		}
		if _, ok := err.(*moderationEntity.ErrEnforceRequiresNote); ok {
			response.BadRequest(c, "Enforce action requires non-empty notes for audit trail")
			return
		}

		h.log.Error("Failed to apply moderation action",
			zap.String("admin_id", adminID.String()),
			zap.String("case_id", caseIDStr),
			zap.String("action", req.Action),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to apply action")
		return
	}

	// W14-B3: Log moderation action to audit trail
	h.adminAuditLogger.LogSafe(ctx, adminID,
		"moderation_action_applied", "moderation_case", caseID,
		map[string]interface{}{
			"action":          req.Action,
			"previous_status": string(previousStatus),
			"new_status":      string(kase.Status),
			"resource_type":   string(kase.ResourceType),
			"resource_id":     kase.ResourceID.String(),
			"notes":           req.Notes,
		},
	)

	response.Success(c, gin.H{
		"case_id":        kase.ID,
		"status":         string(kase.Status),
		"action_applied": req.Action,
		"reviewed_at":    kase.ReviewedAt,
	})
}

// ============================================================================
// HELPER METHODS
// ============================================================================

// fetchResourcePreview fetches minimal resource information for moderation review.
// W1-B2: Operational hardening - provides honest preview instead of blind resource_id.
// Returns nil if resource not found (not an error - UI should handle gracefully).
func (h *ModerationHandler) fetchResourcePreview(
	ctx context.Context,
	resourceType moderationEntity.ResourceType,
	resourceID uuid.UUID,
) *ResourcePreview {
	var preview *ResourcePreview

	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		preview, err = h.fetchResourcePreviewTx(ctx, tx, resourceType, resourceID)
		return err
	})

	if err != nil {
		return nil
	}
	return preview
}

// fetchResourcePreviewTx fetches resource preview within a transaction.
func (h *ModerationHandler) fetchResourcePreviewTx(
	ctx context.Context,
	tx db.Tx,
	resourceType moderationEntity.ResourceType,
	resourceID uuid.UUID,
) (*ResourcePreview, error) {
	switch resourceType {
	case moderationEntity.ResourceTypeContent:
		return h.fetchContentPreview(ctx, tx, resourceID)
	case moderationEntity.ResourceTypeComment:
		return h.fetchCommentPreview(ctx, tx, resourceID)
	case moderationEntity.ResourceTypeForSale:
		return h.fetchForSalePreview(ctx, tx, resourceID)
	case moderationEntity.ResourceTypeAuction:
		return h.fetchAuctionPreview(ctx, tx, resourceID)
	case moderationEntity.ResourceTypeChatMessage:
		return h.fetchChatMessagePreview(ctx, tx, resourceID)
	case moderationEntity.ResourceTypeUser:
		return h.fetchUserPreview(ctx, tx, resourceID)
	default:
		// Future types not yet implemented
		return nil, nil
	}
}

// fetchContentPreview fetches minimal content preview data.
// Uses direct SQL for efficiency - only fetches what's needed for moderation display.
//
// SCHEMA ALIGNMENT (Batch 3K): the canonical contents table has no
// `body` column under the 000100_initial_schema head — only
// `caption`. The previous COALESCE(c.caption, c.body, ”) fallback
// referenced a column that has never existed in the runtime schema
// and would surface as SQLSTATE 42703 on any content moderation
// preview lookup. content_text continues to be emitted on the
// `ResourcePreview.ContentText` JSON field; only the source column
// changes.
func (h *ModerationHandler) fetchContentPreview(ctx context.Context, tx db.Tx, contentID uuid.UUID) (*ResourcePreview, error) {
	query := `
		SELECT
			c.author_id,
			COALESCE(p.username, '') as author_username,
			COALESCE(c.caption, '') as content_text,
			CASE WHEN c.original_author_id IS NOT NULL THEN 'repost' ELSE 'post' END as content_type,
			c.deleted_at IS NOT NULL as is_deleted
		FROM contents c
		LEFT JOIN user_profiles p ON p.user_id = c.author_id
		WHERE c.id = $1
		LIMIT 1
	`

	var authorID string
	var authorName, contentText, contentType string
	var isDeleted bool

	err := tx.QueryRow(ctx, query, contentID).Scan(
		&authorID,
		&authorName,
		&contentText,
		&contentType,
		&isDeleted,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Content not found - return nil preview
	}
	if err != nil {
		return nil, err
	}

	// Truncate content text to max 200 chars for preview
	if len(contentText) > 200 {
		contentText = contentText[:200] + "..."
	}

	return &ResourcePreview{
		AuthorID:       authorID,
		AuthorUsername: authorName,
		ContentText:    contentText,
		ContentType:    contentType, // post, request
		IsDeleted:      isDeleted,
	}, nil
}

// fetchCommentPreview fetches minimal comment preview data.
// Uses direct SQL for efficiency - only fetches what's needed for moderation display.
func (h *ModerationHandler) fetchCommentPreview(ctx context.Context, tx db.Tx, commentID uuid.UUID) (*ResourcePreview, error) {
	query := `
		SELECT
			c.author_id,
			COALESCE(p.username, '') as author_username,
			COALESCE(c.body, '') as content_text,
			CASE WHEN ccr.comment_id IS NULL THEN 'normal' ELSE 'commerce_reference' END as content_type,
			c.deleted_at IS NOT NULL as is_deleted
		FROM comments c
		LEFT JOIN user_profiles p ON p.user_id = c.author_id
		LEFT JOIN comment_commerce_references ccr ON ccr.comment_id = c.id
		WHERE c.id = $1
		LIMIT 1
	`

	var authorID string
	var authorName, contentText, contentType string
	var isDeleted bool

	err := tx.QueryRow(ctx, query, commentID).Scan(
		&authorID,
		&authorName,
		&contentText,
		&contentType,
		&isDeleted,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Comment not found - return nil preview
	}
	if err != nil {
		return nil, err
	}

	// Truncate content text to max 200 chars for preview
	if len(contentText) > 200 {
		contentText = contentText[:200] + "..."
	}

	return &ResourcePreview{
		AuthorID:       authorID,
		AuthorUsername: authorName,
		ContentText:    contentText,
		ContentType:    contentType, // normal, commerce_reference
		IsDeleted:      isDeleted,
	}, nil
}

// fetchForSalePreview fetches minimal fixed-price sale preview data.
//
// Title/description live on products (canonical item authority);
// seller_id/status live on for_sales. The legacy `listings` table is
// write-dead — nothing inserts or updates it anymore.
func (h *ModerationHandler) fetchForSalePreview(ctx context.Context, tx db.Tx, forSaleID uuid.UUID) (*ResourcePreview, error) {
	query := `
		SELECT
			fps.seller_id,
			COALESCE(p.username, '') as seller_username,
			COALESCE(prod.title, '') as title,
			COALESCE(prod.description, '') as content_text,
			fps.status::text as status
		FROM for_sales fps
		JOIN products prod ON prod.id = fps.product_id
		LEFT JOIN user_profiles p ON p.user_id = fps.seller_id
		WHERE fps.id = $1
		LIMIT 1
	`

	var authorID string
	var authorName, title, contentText, status string

	err := tx.QueryRow(ctx, query, forSaleID).Scan(
		&authorID,
		&authorName,
		&title,
		&contentText,
		&status,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if len(contentText) > 200 {
		contentText = contentText[:200] + "..."
	}

	return &ResourcePreview{
		AuthorID:       authorID,
		AuthorUsername: authorName,
		Title:          title,
		Status:         status,
		ContentText:    contentText,
		ContentType:    "for_sale",
		IsDeleted:      false,
	}, nil
}

// fetchAuctionPreview fetches minimal auction preview data.
func (h *ModerationHandler) fetchAuctionPreview(ctx context.Context, tx db.Tx, auctionID uuid.UUID) (*ResourcePreview, error) {
	query := `
		SELECT
			a.seller_id,
			COALESCE(p.username, '') as seller_username,
			COALESCE(prod.title, '') as title,
			COALESCE(prod.description, '') as content_text,
			a.status::text as status
		FROM auctions a
		LEFT JOIN products prod ON prod.id = a.product_id
		LEFT JOIN user_profiles p ON p.user_id = a.seller_id
		WHERE a.id = $1
		LIMIT 1
	`

	var authorID string
	var authorName, title, contentText, status string

	err := tx.QueryRow(ctx, query, auctionID).Scan(
		&authorID,
		&authorName,
		&title,
		&contentText,
		&status,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if len(contentText) > 200 {
		contentText = contentText[:200] + "..."
	}

	return &ResourcePreview{
		AuthorID:       authorID,
		AuthorUsername: authorName,
		Title:          title,
		Status:         status,
		ContentText:    contentText,
		ContentType:    "auction",
		IsDeleted:      false,
	}, nil
}

// fetchChatMessagePreview fetches minimal chat message preview data.
// Uses direct SQL for efficiency - only fetches what's needed for moderation display.
func (h *ModerationHandler) fetchChatMessagePreview(ctx context.Context, tx db.Tx, messageID uuid.UUID) (*ResourcePreview, error) {
	query := `
		SELECT
			cm.sender_id,
			COALESCE(p.username, '') as author_username,
			cm.message_type::text as content_type,
			cm.deleted_at IS NOT NULL as is_deleted,
			cm.deleted_at,
			cm.deletion_reason,
			cm.room_id::text,
			cr.room_type::text as room_type,
			cm.created_at
		FROM chat_messages cm
		LEFT JOIN user_profiles p ON p.user_id = cm.sender_id
		LEFT JOIN chat_rooms cr ON cr.id = cm.room_id
		WHERE cm.id = $1
		LIMIT 1
	`

	var authorID string
	var authorName, contentType string
	var isDeleted bool
	var deletedAt sql.NullTime
	var deletionReason sql.NullString
	var roomID, roomType string
	var sentAt time.Time

	err := tx.QueryRow(ctx, query, messageID).Scan(
		&authorID,
		&authorName,
		&contentType,
		&isDeleted,
		&deletedAt,
		&deletionReason,
		&roomID,
		&roomType,
		&sentAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &ResourcePreview{
		AuthorID:                   authorID,
		AuthorUsername:             authorName,
		ContentType:                contentType, // text, image, system, etc.
		IsDeleted:                  isDeleted,
		DeletedAt:                  nullTimePtr(deletedAt),
		DeletionReason:             nullStringPtr(deletionReason),
		EvidenceAvailable:          true,
		EvidenceRequiresCapability: capability.CapModerationEvidenceRead.String(),
		RoomID:                     roomID,
		RoomType:                   roomType,
		SentAt:                     sentAt.Format(time.RFC3339),
	}, nil
}

// fetchChatMessageEvidenceTx loads original hidden chat evidence for an admin
// who has already passed capability and case-type checks.
func (h *ModerationHandler) fetchChatMessageEvidenceTx(
	ctx context.Context,
	tx db.Tx,
	kase *moderationEntity.GovernanceCase,
) (*ModerationCaseEvidence, error) {
	query := `
		SELECT
			cm.id::text,
			cm.room_id::text,
			cr.room_type::text,
			cm.sender_id::text,
			COALESCE(p.username, '') as author_username,
			cm.created_at,
			cm.deleted_at,
			cm.deletion_reason,
			cm.body,
			cm.attachment_json
		FROM chat_messages cm
		LEFT JOIN chat_rooms cr ON cr.id = cm.room_id
		LEFT JOIN user_profiles p ON p.user_id = cm.sender_id
		WHERE cm.id = $1
		LIMIT 1
	`

	var messageID string
	var roomID string
	var roomType string
	var senderID string
	var authorUsername string
	var createdAt time.Time
	var deletedAt sql.NullTime
	var deletionReason sql.NullString
	var originalBody sql.NullString
	var originalAttachmentBytes []byte

	if err := tx.QueryRow(ctx, query, kase.ResourceID).Scan(
		&messageID,
		&roomID,
		&roomType,
		&senderID,
		&authorUsername,
		&createdAt,
		&deletedAt,
		&deletionReason,
		&originalBody,
		&originalAttachmentBytes,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("load moderation evidence failed: %w", err)
	}

	var originalAttachment map[string]interface{}
	if len(originalAttachmentBytes) > 0 {
		if err := json.Unmarshal(originalAttachmentBytes, &originalAttachment); err != nil {
			return nil, fmt.Errorf("decode moderation evidence attachment failed: %w", err)
		}
	}

	return &ModerationCaseEvidence{
		CaseID:             kase.ID.String(),
		ResourceType:       string(kase.ResourceType),
		ResourceID:         kase.ResourceID.String(),
		MessageID:          messageID,
		RoomID:             roomID,
		RoomType:           roomType,
		SenderID:           senderID,
		AuthorUsername:     authorUsername,
		CreatedAt:          createdAt.Format(time.RFC3339),
		DeletedAt:          nullTimePtr(deletedAt),
		DeletionReason:     nullStringPtr(deletionReason),
		OriginalBody:       nullStringPtr(originalBody),
		OriginalAttachment: originalAttachment,
	}, nil
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time
	return &t
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	s := value.String
	return &s
}

// fetchUserPreview fetches minimal user account preview data for moderation review.
// Only exposes fields already visible on existing admin user surfaces:
// username, account_status, and deletion state — no email or phone.
func (h *ModerationHandler) fetchUserPreview(ctx context.Context, tx db.Tx, userID uuid.UUID) (*ResourcePreview, error) {
	query := `
		SELECT
			u.id::text as user_id,
			COALESCE(p.username, '') as username,
			u.account_status as account_status,
			u.deleted_at IS NOT NULL as is_deleted
		FROM users u
		LEFT JOIN user_profiles p ON p.user_id = u.id
		WHERE u.id = $1
		LIMIT 1
	`

	var userIDStr string
	var username, accountStatus string
	var isDeleted bool

	err := tx.QueryRow(ctx, query, userID).Scan(
		&userIDStr,
		&username,
		&accountStatus,
		&isDeleted,
	)

	if err == sql.ErrNoRows {
		return nil, nil // User not found - return nil preview
	}
	if err != nil {
		return nil, err
	}

	return &ResourcePreview{
		AuthorID:       userIDStr,
		AuthorUsername: username,
		Status:         accountStatus,
		ContentType:    "user",
		ContentText:    "", // User accounts have no content body
		IsDeleted:      isDeleted,
	}, nil
}

// caseToResponse converts a GovernanceCase entity to a response map.
func (h *ModerationHandler) caseToResponse(kase *moderationEntity.GovernanceCase) gin.H {
	resp := gin.H{
		"id":            kase.ID,
		"resource_type": string(kase.ResourceType),
		"resource_id":   kase.ResourceID,
		"status":        string(kase.Status),
		"reported_by":   kase.ReportedBy,
		"reason":        kase.Reason,
		"created_at":    kase.CreatedAt,
	}

	if kase.ReviewedBy != nil {
		resp["reviewed_by"] = *kase.ReviewedBy
	}

	if kase.DecisionNote != nil {
		resp["decision_note"] = *kase.DecisionNote
	}

	if kase.ReviewedAt != nil {
		resp["reviewed_at"] = *kase.ReviewedAt
	}

	return resp
}
