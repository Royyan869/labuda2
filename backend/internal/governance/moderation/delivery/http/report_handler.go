// DOMAIN: Moderation Domain (governance/moderation/)
// RESPONSIBILITY: Canonical Report intake HTTP API
//
// User-facing Report contract:
//   POST   /api/v1/reports        — create a Report (canonical intake)
//   GET    /api/v1/reports/mine   — list own Reports with safe Case+Decision projection
//   GET    /api/v1/reports/:id    — get own Report by ID with safe Case+Decision projection
//
// These replace the legacy POST /moderation/cases, GET /moderation/my-cases,
// and GET /moderation/cases/:id intake endpoints.
//
// Evidence snapshot is INTERNAL GOVERNANCE DATA.
// Raw evidence_snapshot MUST NOT be exposed to reporters.
// User-facing response carries only: safe target title from snapshot (if any),
// the report fields, the safe case projection, and the safe decision projection.

package http

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	moderationApp "github.com/labuda/backend/internal/governance/moderation/application"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	moderationRepo "github.com/labuda/backend/internal/governance/moderation/infrastructure/repository"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// ReportHandler handles canonical Report intake HTTP requests.
type ReportHandler struct {
	reportService *moderationApp.ReportService
	db            db.Transactor
	caseRepo      moderationRepo.CaseRepository
	decRepo       moderationRepo.DecisionRepository
	log           *zap.Logger
}

// AdminAuditLogger defines the interface for logging admin actions.
// Kept in this package for the out-of-scope Appeal/Warning handlers that still
// reference it (defined here after the legacy moderation_handler.go removal).
type AdminAuditLogger interface {
	LogSafe(ctx context.Context, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{})
	LogTx(ctx context.Context, tx db.Tx, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{}) error
}

// NewReportHandler creates a canonical Report handler.
// Accepts variadic args to preserve backward-compatibility for existing test callsites
// that pass only (service, logger). Production wiring must use NewReportHandlerWithDeps.
func NewReportHandler(reportService *moderationApp.ReportService, args ...interface{}) *ReportHandler {
	var (
		dbVal    db.Transactor                    = nil
		caseRepo moderationRepo.CaseRepository    = nil
		decRepo  moderationRepo.DecisionRepository = nil
		logVal   *zap.Logger                      = zap.NewNop()
	)
	for _, a := range args {
		switch v := a.(type) {
		case db.Transactor:
			dbVal = v
		case moderationRepo.CaseRepository:
			caseRepo = v
		case moderationRepo.DecisionRepository:
			decRepo = v
		case *zap.Logger:
			if v != nil {
				logVal = v
			}
		}
	}
	return &ReportHandler{reportService: reportService, db: dbVal, caseRepo: caseRepo, decRepo: decRepo, log: logVal}
}

// NewReportHandlerWithDeps is the explicit canonical constructor used in production wiring.
func NewReportHandlerWithDeps(
	reportService *moderationApp.ReportService,
	dbConn db.Transactor,
	caseRepo moderationRepo.CaseRepository,
	decRepo moderationRepo.DecisionRepository,
	log *zap.Logger,
) *ReportHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &ReportHandler{
		reportService: reportService,
		db:            dbConn,
		caseRepo:      caseRepo,
		decRepo:       decRepo,
		log:           log,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// REQUEST DTOs
// ─────────────────────────────────────────────────────────────────────────────

// CreateReportRequest is the canonical Report create request.
//
// subject_type: content|comment|for_sale|auction|user (canonical targets only).
// reason_code:  locked taxonomy (scam_or_fraud, prohibited_content,
//
//	harassment_or_abuse, impersonation, misleading_information,
//	commerce_violation, other).
//
// reason_note:  optional free text (NOT a reason_code replacement).
type CreateReportRequest struct {
	SubjectType string  `json:"subject_type" binding:"required"`
	SubjectID   string  `json:"subject_id" binding:"required,uuid"`
	ReasonCode  string  `json:"reason_code" binding:"required"`
	ReasonNote  *string `json:"reason_note" binding:"omitempty,max=2000"`
}

// ─────────────────────────────────────────────────────────────────────────────
// HANDLERS
// ─────────────────────────────────────────────────────────────────────────────

// CreateReport handles POST /api/v1/reports.
//
// Response: 201 Created with the immutable Report record + safe Case projection.
// Errors:
//   - 400 invalid target type / invalid reason code / invalid request
//   - 404 subject does not exist in its canonical target domain
//   - 409 duplicate report (same reporter + same subject)
//   - 400 self-report denied (Owner decision)
func (h *ReportHandler) CreateReport(c *gin.Context) {
	ctx := c.Request.Context()

	reporterID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	var req CreateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	subjectID, err := uuid.Parse(req.SubjectID)
	if err != nil {
		response.BadRequest(c, "Invalid subject_id")
		return
	}

	subjectType := entity.ReportTargetType(req.SubjectType)
	reasonCode := entity.ReportReasonCode(req.ReasonCode)

	report, err := h.reportService.CreateReport(ctx, moderationApp.CreateReportInput{
		ReporterID:  reporterID,
		SubjectType: subjectType,
		SubjectID:   subjectID,
		ReasonCode:  reasonCode,
		ReasonNote:  req.ReasonNote,
	})
	if err != nil {
		var invalidTarget *entity.ErrInvalidReportTarget
		var invalidReason *entity.ErrInvalidReasonCode
		var notFound *moderationRepo.ErrReportTargetNotFound
		var dup *moderationRepo.ErrDuplicateReport
		var selfReport *moderationApp.ErrSelfReportDenied

		switch {
		case errors.As(err, &invalidTarget):
			response.BadRequest(c, err.Error())
			return
		case errors.As(err, &invalidReason):
			response.BadRequest(c, err.Error())
			return
		case errors.As(err, &notFound):
			response.NotFound(c, err.Error())
			return
		case errors.As(err, &dup):
			response.Conflict(c, err.Error())
			return
		case errors.As(err, &selfReport):
			response.BadRequest(c, err.Error())
			return
		}

		h.log.Error("Failed to create report",
			zap.String("reporter_id", reporterID.String()),
			zap.String("subject_type", req.SubjectType),
			zap.String("subject_id", req.SubjectID),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to create report")
		return
	}

	// Return the canonical report shape. On creation the Case was correlated
	// atomically so case_id is present; full Case projection is available via
	// GET /reports/:id.
	response.Created(c, reportToUserResponse(report, nil, nil))
}

// GetMyReport handles GET /api/v1/reports/:id.
//
// Returns a Report ONLY if the authenticated user is the reporter.
// Includes safe Case and Decision projections.
// Raw evidence_snapshot is NEVER exposed to the reporter.
func (h *ReportHandler) GetMyReport(c *gin.Context) {
	ctx := c.Request.Context()

	userID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	reportID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid report ID")
		return
	}

	report, err := h.reportService.GetReport(ctx, reportID)
	if err != nil {
		h.log.Error("Failed to get report",
			zap.String("user_id", userID.String()),
			zap.String("report_id", reportID.String()),
			zap.Error(err),
		)
		response.NotFound(c, "Report not found")
		return
	}
	if report == nil {
		response.NotFound(c, "Report not found")
		return
	}

	// Ownership: user can only see their own reports.
	if report.ReporterID != userID {
		response.NotFound(c, "Report not found")
		return
	}

	kase, decision := h.fetchCaseAndDecision(ctx, report)
	response.Success(c, gin.H{"report": reportToUserResponse(report, kase, decision)})
}

// ListMyReports handles GET /api/v1/reports/mine.
//
// Returns all Reports created by the authenticated user, newest first.
// Each report includes safe Case and Decision projections.
// Raw evidence_snapshot is NEVER exposed to reporters.
// Query parameters: page (default 1), limit (default 20, max 100).
func (h *ReportHandler) ListMyReports(c *gin.Context) {
	ctx := c.Request.Context()

	userID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	reports, err := h.reportService.ListReportsByReporter(ctx, userID, limit, offset)
	if err != nil {
		h.log.Error("Failed to list my reports",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve reports")
		return
	}

	items := make([]gin.H, len(reports))
	for i, r := range reports {
		kase, decision := h.fetchCaseAndDecision(ctx, r)
		items[i] = reportToUserResponse(r, kase, decision)
	}

	response.Success(c, gin.H{
		"reports": items,
		"page":    page,
		"limit":   limit,
		"count":   len(items),
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// PRIVATE HELPERS
// ─────────────────────────────────────────────────────────────────────────────

// fetchCaseAndDecision retrieves the Case and most-recent Decision correlated to
// this Report. Returns (nil, nil) if repos are not wired or the Case does not exist.
// Errors are logged and suppressed — projection is best-effort so a read failure
// does not block the reporter from seeing their own Report.
func (h *ReportHandler) fetchCaseAndDecision(ctx context.Context, r *entity.Report) (*entity.CanonicalCase, *entity.Decision) {
	if h.db == nil || h.caseRepo == nil || r.CaseID == nil {
		return nil, nil
	}

	var kase *entity.CanonicalCase
	var decision *entity.Decision

	_ = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		kase, err = h.caseRepo.GetByID(ctx, tx, *r.CaseID)
		if err != nil || kase == nil {
			return err
		}

		if h.decRepo == nil {
			return nil
		}
		decisions, err := h.decRepo.ListByCase(ctx, tx, kase.ID, 1, 0)
		if err != nil || len(decisions) == 0 {
			return err
		}
		decision = decisions[0]
		return nil
	})

	return kase, decision
}

// ─────────────────────────────────────────────────────────────────────────────
// RESPONSE BUILDERS
// ─────────────────────────────────────────────────────────────────────────────

// reportToUserResponse builds the safe user-facing Report response.
//
// SECURITY INVARIANT: raw evidence_snapshot is NEVER serialized into this response.
// Only a safe target projection (subject_type, subject_id, title from snapshot if present)
// is exposed to the reporter. Internal governance fields (author_id, author_username,
// lifecycle status, content_type, is_deleted) remain server-side only.
func reportToUserResponse(r *entity.Report, kase *entity.CanonicalCase, decision *entity.Decision) gin.H {
	resp := gin.H{
		"id":           r.ID,
		"reporter_id":  r.ReporterID,
		"subject_type": string(r.SubjectType),
		"subject_id":   r.SubjectID,
		"reason_code":  string(r.ReasonCode),
		"created_at":   r.CreatedAt,
	}

	if r.ReasonNote != nil {
		resp["reason_note"] = *r.ReasonNote
	}

	if r.CaseID != nil {
		resp["case_id"] = *r.CaseID
	}

	// Safe target projection: only title is exposed (safe display name from snapshot).
	// MUST NOT expose: author_id, author_username, status, content_type, is_deleted.
	if r.EvidenceSnapshot != nil && r.EvidenceSnapshot.Title != "" {
		resp["target"] = gin.H{
			"subject_type": string(r.SubjectType),
			"subject_id":   r.SubjectID,
			"title":        r.EvidenceSnapshot.Title,
		}
	}

	// Safe Case projection: status + timestamps only. No reporter, no admin fields.
	if kase != nil {
		caseProj := gin.H{
			"id":         kase.ID,
			"status":     string(kase.Status),
			"created_at": kase.CreatedAt,
		}
		if kase.ClosedAt != nil {
			caseProj["closed_at"] = *kase.ClosedAt
		}
		resp["case"] = caseProj
	}

	// Safe Decision projection: outcome + created_at only.
	// MUST NOT expose: decided_by (admin identity), decision_note (internal).
	if decision != nil {
		resp["decision"] = gin.H{
			"outcome":    string(decision.Outcome),
			"created_at": decision.CreatedAt,
		}
	}

	return resp
}

// ─────────────────────────────────────────────────────────────────────────────
// ADMIN-INTERNAL MAPPER (used by governance_admin_handler.go)
// ─────────────────────────────────────────────────────────────────────────────

// reportToResponse is the ADMIN-ONLY full Report response (used internally by
// GovernanceAdminHandler). It includes case_id for correlation but still omits
// the raw evidence_snapshot to keep the response shaped consistently.
// Admin full evidence access is via the DB / audit trail, not this endpoint.
func reportToResponse(r *entity.Report) gin.H {
	resp := gin.H{
		"id":           r.ID,
		"reporter_id":  r.ReporterID,
		"subject_type": string(r.SubjectType),
		"subject_id":   r.SubjectID,
		"reason_code":  string(r.ReasonCode),
		"created_at":   r.CreatedAt,
	}

	if r.ReasonNote != nil {
		resp["reason_note"] = *r.ReasonNote
	}
	if r.CaseID != nil {
		resp["case_id"] = *r.CaseID
	}

	// Admin sees the safe target title from the snapshot (same as user-facing).
	// The raw snapshot blob (author_id etc.) is NOT serialized into HTTP responses;
	// it stays in DB governance storage.
	if r.EvidenceSnapshot != nil && r.EvidenceSnapshot.Title != "" {
		resp["target_title"] = r.EvidenceSnapshot.Title
	}

	return resp
}

// ─────────────────────────────────────────────────────────────────────────────
// UNUSED — kept here to satisfy Go compiler for time import used elsewhere
// ─────────────────────────────────────────────────────────────────────────────

var _ = time.Time{}
