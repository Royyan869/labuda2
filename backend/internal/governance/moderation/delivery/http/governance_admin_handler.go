// DOMAIN: Moderation Domain (governance/moderation/)
// RESPONSIBILITY: Canonical Admin Governance HTTP API
//
// SLICE 6: Admin-facing endpoints for the canonical governance workflow:
//   GET    /admin/governance/cases              — list Cases (open/resolved)
//   GET    /admin/governance/cases/:id          — Case detail with Reports + Decisions + Enforcement
//   POST   /admin/governance/cases/:id/decisions — create Decision for Case
//   GET    /admin/governance/decisions/:id      — Decision detail
//   GET    /admin/governance/decisions/:id/enforcement — Enforcement status
//
// These are pure adapters to the canonical application services.
// No duplicate Decision authority. No legacy moderation_cases.

package http

import (
	"context"
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	auditentity "github.com/labuda/backend/internal/governance/audit/entity"
	"github.com/labuda/backend/internal/governance/moderation/application"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/internal/governance/moderation/infrastructure/repository"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// GovernanceAuditQuerier is the minimal interface for querying governance audit events.
// Satisfied by *auditApp.AuditService.
type GovernanceAuditQuerier interface {
	// GetByEntity retrieves audit events for a specific entity type and ID.
	GetByEntity(ctx context.Context, entityType string, entityID uuid.UUID, limit int) ([]*auditentity.AuditEvent, error)

	// GetByEntityIDs retrieves audit events for multiple entity IDs of the same type.
	GetByEntityIDs(ctx context.Context, entityType string, entityIDs []uuid.UUID, limit int) ([]*auditentity.AuditEvent, error)
}

// GovernanceAdminHandler handles canonical admin governance HTTP requests.
type GovernanceAdminHandler struct {
	db          db.Transactor
	caseRepo    repository.CaseRepository
	reportRepo  repository.ReportRepository
	decisionSvc *application.DecisionService
	enfRepo     repository.EnforcementRepository
	decRepo     repository.DecisionRepository
	auditQuerier GovernanceAuditQuerier
	log         *zap.Logger
}

// NewGovernanceAdminHandler creates the canonical admin governance handler.
func NewGovernanceAdminHandler(
	dbConn db.Transactor,
	caseRepo repository.CaseRepository,
	reportRepo repository.ReportRepository,
	decisionSvc *application.DecisionService,
	enfRepo repository.EnforcementRepository,
	decRepo repository.DecisionRepository,
	auditQuerier GovernanceAuditQuerier,
	log *zap.Logger,
) *GovernanceAdminHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &GovernanceAdminHandler{
		db:           dbConn,
		caseRepo:     caseRepo,
		reportRepo:   reportRepo,
		decisionSvc:  decisionSvc,
		enfRepo:      enfRepo,
		decRepo:      decRepo,
		auditQuerier: auditQuerier,
		log:          log,
	}
}

// ============================================================================
// REQUEST / RESPONSE DTOs
// ============================================================================

// CreateDecisionRequest is the admin request body for creating a Decision.
type CreateDecisionRequest struct {
	Outcome      string  `json:"outcome" binding:"required,oneof=no_violation violation"`
	TargetType   string  `json:"target_type" binding:"omitempty"`
	TargetID     string  `json:"target_id" binding:"omitempty"`
	DecisionNote *string `json:"decision_note" binding:"omitempty,max=2000"`
}

// ============================================================================
// CASE ENDPOINTS
// ============================================================================

// ListCases handles GET /admin/governance/cases
//
// Returns all Cases, ordered by created_at DESC.
// Query parameters:
//   - status: filter by status ("open" | "resolved"), empty = all
//   - page: page number (default 1)
//   - limit: items per page (default 20, max 100)
func (h *GovernanceAdminHandler) ListCases(c *gin.Context) {
	ctx := c.Request.Context()

	_, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	// Parse status filter
	var statusFilter *entity.CaseStatus
	if statusRaw := c.Query("status"); statusRaw != "" {
		status := entity.CaseStatus(statusRaw)
		if !status.IsValid() {
			response.BadRequest(c, "Invalid status filter. Valid: open, resolved")
			return
		}
		statusFilter = &status
	}

	var cases []*entity.CanonicalCase
	var count int
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		cases, err = h.caseRepo.ListAll(ctx, tx, statusFilter, limit, offset)
		if err != nil {
			return err
		}
		count, err = h.caseRepo.CountAll(ctx, tx, statusFilter)
		return err
	})

	if err != nil {
		h.log.Error("Failed to list cases", zap.Error(err))
		response.InternalServerError(c, "Failed to retrieve cases")
		return
	}

	items := make([]gin.H, len(cases))
	for i, kase := range cases {
		items[i] = caseToListResponse(kase)
	}

	response.Success(c, gin.H{
		"cases": items,
		"page":  page,
		"limit": limit,
		"count": count,
	})
}

// GetCase handles GET /admin/governance/cases/:id
//
// Returns Case detail with related Reports, Decisions, and Enforcement status.
func (h *GovernanceAdminHandler) GetCase(c *gin.Context) {
	ctx := c.Request.Context()

	_, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	caseID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid case ID")
		return
	}

	var kase *entity.CanonicalCase
	var reports []*entity.Report
	var decisions []*entity.Decision

	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		// Fetch case
		kase, err = h.caseRepo.GetByID(ctx, tx, caseID)
		if err != nil {
			return err
		}
		if kase == nil {
			return &errNotFound{resource: "case"}
		}

		// Fetch related reports
		reports, err = h.reportRepo.ListByCaseID(ctx, tx, caseID)
		if err != nil {
			return err
		}

		// Fetch decisions for this case
		decisions, err = h.decisionSvc.ListDecisionsByCase(ctx, caseID, 50, 0)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		var nf *errNotFound
		if errors.As(err, &nf) {
			response.NotFound(c, "Case not found")
			return
		}
		h.log.Error("Failed to get case",
			zap.String("case_id", caseID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve case")
		return
	}

	// Build response with enforcement info for violation decisions
	decisionItems := make([]gin.H, len(decisions))
	for i, d := range decisions {
		dResp := decisionToResponse(d)

		// If violation, fetch enforcement(s) for this decision
		if d.Outcome == entity.DecisionOutcomeViolation {
			enforcements, enfErr := h.enfRepo.ListByDecision(ctx, nil, d.ID)
			if enfErr == nil && len(enforcements) > 0 {
				enfItems := make([]gin.H, len(enforcements))
				for j, e := range enforcements {
					enfItems[j] = enforcementToResponse(e)
				}
				dResp["enforcements"] = enfItems
			}
		}

		decisionItems[i] = dResp
	}

	reportItems := make([]gin.H, len(reports))
	for i, r := range reports {
		reportItems[i] = reportToResponse(r)
	}

	response.Success(c, gin.H{
		"case":      caseToDetailResponse(kase),
		"reports":   reportItems,
		"decisions": decisionItems,
	})
}

// ============================================================================
// AUDIT ENDPOINTS
// ============================================================================

// GetCaseAudit handles GET /admin/governance/cases/:id/audit
//
// Returns governance audit events for all Decisions belonging to this Case.
// Requires: moderation.case.read capability (same as Case Detail).
func (h *GovernanceAdminHandler) GetCaseAudit(c *gin.Context) {
	ctx := c.Request.Context()

	_, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	caseID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid case ID")
		return
	}

	// Verify case exists
	var kase *entity.CanonicalCase
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		kase, err = h.caseRepo.GetByID(ctx, tx, caseID)
		return err
	})
	if err != nil {
		h.log.Error("Failed to get case for audit", zap.String("case_id", caseID.String()), zap.Error(err))
		response.InternalServerError(c, "Failed to retrieve case")
		return
	}
	if kase == nil {
		response.NotFound(c, "Case not found")
		return
	}

	// No audit querier configured
	if h.auditQuerier == nil {
		response.Success(c, gin.H{"events": []gin.H{}, "count": 0})
		return
	}

	// Fetch all Decisions for this Case
	var decisions []*entity.Decision
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		decisions, err = h.decRepo.ListByCase(ctx, tx, caseID, 100, 0)
		return err
	})
	if err != nil {
		h.log.Error("Failed to list decisions for audit", zap.String("case_id", caseID.String()), zap.Error(err))
		response.InternalServerError(c, "Failed to retrieve audit events")
		return
	}

	if len(decisions) == 0 {
		response.Success(c, gin.H{"events": []gin.H{}, "count": 0})
		return
	}

	// Collect decision IDs for bulk audit query
	decisionIDs := make([]uuid.UUID, len(decisions))
	for i, d := range decisions {
		decisionIDs[i] = d.ID
	}

	// Fetch audit events for all decisions in one query
	auditEvents, err := h.auditQuerier.GetByEntityIDs(ctx, "governance.decision", decisionIDs, 100)
	if err != nil {
		h.log.Error("Failed to query audit events", zap.String("case_id", caseID.String()), zap.Error(err))
		response.InternalServerError(c, "Failed to retrieve audit events")
		return
	}

	items := make([]gin.H, len(auditEvents))
	for i, event := range auditEvents {
		items[i] = auditEventToResponse(event)
	}

	response.Success(c, gin.H{
		"events": items,
		"count":  len(items),
	})
}

// ============================================================================
// DECISION ENDPOINTS
// ============================================================================

// CreateDecision handles POST /admin/governance/cases/:id/decisions
//
// Creates a Decision against a Case through the canonical DecisionService.
// Business rules (all enforced by DecisionService):
//   - Case must exist
//   - outcome must be "no_violation" or "violation"
//   - If violation: target_type and target_id are required
//   - Decision is immutable
//   - Case is resolved if open
//   - Enforcement + outbox event created atomically for violation
func (h *GovernanceAdminHandler) CreateDecision(c *gin.Context) {
	ctx := c.Request.Context()

	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	caseID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid case ID")
		return
	}

	var req CreateDecisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	input := application.CreateDecisionInput{
		CaseID:       caseID,
		DecidedBy:    adminID,
		Outcome:      entity.DecisionOutcome(req.Outcome),
		DecisionNote: req.DecisionNote,
	}

	// Validate and parse enforcement target for violation decisions.
	if req.Outcome == "violation" {
		if req.TargetType == "" {
			response.BadRequest(c, "target_type is required for violation decisions")
			return
		}
		if req.TargetID == "" {
			response.BadRequest(c, "target_id is required for violation decisions")
			return
		}

		targetType := entity.ModerationTargetType(req.TargetType)
		if !targetType.IsValid() {
			response.BadRequest(c, "Invalid target_type. Valid: content, comment, for_sale, auction, user")
			return
		}

		targetID, err := uuid.Parse(req.TargetID)
		if err != nil {
			response.BadRequest(c, "Invalid target_id")
			return
		}

		input.TargetType = targetType
		input.TargetID = targetID
	}

	// Delegate to the canonical DecisionService — single authority.
	decision, err := h.decisionSvc.CreateDecision(ctx, input)
	if err != nil {
		// Map domain errors to HTTP responses.
		var caseNotFound *entity.ErrDecisionCaseNotFound
		var invalidOutcome *entity.ErrInvalidDecisionOutcome
		var invalidTarget *entity.ErrInvalidEnforcementTargetType

		switch {
		case errors.As(err, &caseNotFound):
			response.NotFound(c, "Case not found")
			return
		case errors.As(err, &invalidOutcome):
			response.BadRequest(c, err.Error())
			return
		case errors.As(err, &invalidTarget):
			response.BadRequest(c, err.Error())
			return
		}

		h.log.Error("Failed to create decision",
			zap.String("case_id", caseID.String()),
			zap.String("admin_id", adminID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to create decision")
		return
	}

	response.Created(c, gin.H{
		"decision": decisionToResponse(decision),
	})
}

// GetDecision handles GET /admin/governance/decisions/:id
//
// Returns truthful immutable Decision information.
func (h *GovernanceAdminHandler) GetDecision(c *gin.Context) {
	ctx := c.Request.Context()

	_, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	decisionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid decision ID")
		return
	}

	decision, err := h.decisionSvc.GetDecision(ctx, decisionID)
	if err != nil {
		h.log.Error("Failed to get decision",
			zap.String("decision_id", decisionID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve decision")
		return
	}
	if decision == nil {
		response.NotFound(c, "Decision not found")
		return
	}

	// Fetch enforcements if violation
	var enforcementsResp []gin.H
	if decision.Outcome == entity.DecisionOutcomeViolation {
		enforcements, enfErr := h.enfRepo.ListByDecision(ctx, nil, decision.ID)
		if enfErr == nil {
			enforcementsResp = make([]gin.H, len(enforcements))
			for i, e := range enforcements {
				enforcementsResp[i] = enforcementToResponse(e)
			}
		}
	}

	resp := decisionToResponse(decision)
	if enforcementsResp != nil {
		resp["enforcements"] = enforcementsResp
	}

	response.Success(c, gin.H{
		"decision": resp,
	})
}

// ============================================================================
// ENFORCEMENT ENDPOINTS
// ============================================================================

// GetEnforcement handles GET /admin/governance/decisions/:id/enforcement
//
// Returns Enforcement status for a Decision's enforcement(s).
func (h *GovernanceAdminHandler) GetEnforcement(c *gin.Context) {
	ctx := c.Request.Context()

	_, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	decisionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid decision ID")
		return
	}

	// Verify decision exists
	decision, err := h.decisionSvc.GetDecision(ctx, decisionID)
	if err != nil {
		h.log.Error("Failed to get decision for enforcement lookup",
			zap.String("decision_id", decisionID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve enforcement")
		return
	}
	if decision == nil {
		response.NotFound(c, "Decision not found")
		return
	}

	if decision.Outcome != entity.DecisionOutcomeViolation {
		response.Success(c, gin.H{
			"enforcements": []gin.H{},
			"message":       "No enforcement for no_violation decisions",
		})
		return
	}

	// Fetch enforcements
	enforcements, err := h.enfRepo.ListByDecision(ctx, nil, decisionID)
	if err != nil {
		h.log.Error("Failed to list enforcements",
			zap.String("decision_id", decisionID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve enforcement status")
		return
	}

	items := make([]gin.H, len(enforcements))
	for i, e := range enforcements {
		items[i] = enforcementToResponse(e)
	}

	response.Success(c, gin.H{
		"enforcements": items,
	})
}

// ============================================================================
// RESPONSE MAPPERS
// ============================================================================

// caseToListResponse converts a CanonicalCase to admin list response.
func caseToListResponse(kase *entity.CanonicalCase) gin.H {
	resp := gin.H{
		"id":           kase.ID,
		"subject_type": string(kase.SubjectType),
		"subject_id":   kase.SubjectID,
		"status":       string(kase.Status),
		"created_at":   kase.CreatedAt,
		"updated_at":   kase.UpdatedAt,
	}
	if kase.ClosedAt != nil {
		resp["closed_at"] = *kase.ClosedAt
	}
	return resp
}

// caseToDetailResponse converts a CanonicalCase to admin detail response.
func caseToDetailResponse(kase *entity.CanonicalCase) gin.H {
	return caseToListResponse(kase)
}

// decisionToResponse converts a Decision to response.
func decisionToResponse(d *entity.Decision) gin.H {
	resp := gin.H{
		"id":         d.ID,
		"case_id":    d.CaseID,
		"decided_by": d.DecidedBy,
		"outcome":    string(d.Outcome),
		"created_at": d.CreatedAt,
	}
	if d.DecisionNote != nil {
		resp["decision_note"] = *d.DecisionNote
	}
	return resp
}

// enforcementToResponse converts an Enforcement to response.
func enforcementToResponse(e *entity.Enforcement) gin.H {
	resp := gin.H{
		"id":            e.ID,
		"decision_id":   e.DecisionID,
		"target_type":   string(e.TargetType),
		"target_id":     e.TargetID,
		"status":        string(e.Status),
		"attempt_count": e.AttemptCount,
		"requested_at":  e.RequestedAt,
		"created_at":    e.CreatedAt,
		"updated_at":    e.UpdatedAt,
	}
	if e.StartedAt != nil {
		resp["started_at"] = *e.StartedAt
	}
	if e.FinishedAt != nil {
		resp["finished_at"] = *e.FinishedAt
	}
	if e.LastError != nil {
		resp["last_error"] = *e.LastError
	}
	if e.NextAttemptAt != nil {
		resp["next_attempt_at"] = *e.NextAttemptAt
	}
	return resp
}

// ============================================================================
// AUDIT RESPONSE MAPPER
// ============================================================================

// auditEventToResponse converts an audit event to a clean admin-facing DTO.
// Exposes only governance-relevant fields; raw payload is structured into
// meaningful top-level fields.
func auditEventToResponse(event *auditentity.AuditEvent) gin.H {
	resp := gin.H{
		"id":         event.ID,
		"event_type": event.EventType,
		"actor_type": event.ActorType,
		"created_at": event.CreatedAt,
	}

	if event.ActorID != nil {
		resp["actor_id"] = *event.ActorID
	}

	// Extract governance-relevant payload fields into top-level response
	if payload, ok := event.PayloadJSON.(map[string]interface{}); ok {
		if outcome, ok := payload["outcome"]; ok {
			resp["outcome"] = outcome
		}
		if caseID, ok := payload["case_id"]; ok {
			resp["case_id"] = caseID
		}
		if targetType, ok := payload["target_type"]; ok {
			resp["target_type"] = targetType
		}
		if targetID, ok := payload["target_id"]; ok {
			resp["target_id"] = targetID
		}
		if decisionNote, ok := payload["decision_note"]; ok {
			resp["decision_note"] = decisionNote
		}
		if actorName, ok := payload["actor_name"]; ok {
			resp["actor_name"] = actorName
		}
	}

	return resp
}

// ============================================================================
// ERROR TYPES
// ============================================================================

// errNotFound is a simple domain error for resource not found.
type errNotFound struct {
	resource string
}

func (e *errNotFound) Error() string {
	return e.resource + " not found"
}
