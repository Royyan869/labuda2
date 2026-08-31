// DOMAIN: Moderation Domain (governance/moderation/)
// RESPONSIBILITY: Canonical Report intake HTTP API
//
// SLICE 2: User-facing Report contract:
//   POST   /api/v1/reports        — create a Report (canonical intake)
//   GET    /api/v1/reports/mine   — list own Reports
//   GET    /api/v1/reports/:id    — get own Report by ID
//
// These replace the legacy POST /moderation/cases, GET /moderation/my-cases,
// and GET /moderation/cases/:id intake endpoints. The handler calls ONLY the
// canonical ReportService — no GovernanceCase, no moderation_cases.

package http

import (
	"context"
	"errors"
	"strconv"

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
func NewReportHandler(reportService *moderationApp.ReportService, log *zap.Logger) *ReportHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &ReportHandler{reportService: reportService, log: log}
}

// CreateReportRequest is the canonical Report create request.
//
// subject_type: content|comment|for_sale|auction|user (canonical targets only).
// reason_code:  locked taxonomy (scam_or_fraud, prohibited_content,
//               harassment_or_abuse, impersonation, misleading_information,
//               commerce_violation, other).
// reason_note:  optional free text (NOT a reason_code replacement).
type CreateReportRequest struct {
	SubjectType string  `json:"subject_type" binding:"required"`
	SubjectID   string  `json:"subject_id" binding:"required,uuid"`
	ReasonCode  string  `json:"reason_code" binding:"required"`
	ReasonNote  *string `json:"reason_note" binding:"omitempty,max=2000"`
}

// CreateReport handles POST /api/v1/reports.
//
// Response: 201 Created with the immutable Report record.
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

	// Backend is the authority for target + reason validation.
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

	response.Created(c, reportToResponse(report))
}

// GetMyReport handles GET /api/v1/reports/:id.
//
// Returns a Report ONLY if the authenticated user is the reporter.
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

	response.Success(c, gin.H{"report": reportToResponse(report)})
}

// ListMyReports handles GET /api/v1/reports/mine.
//
// Returns all Reports created by the authenticated user, newest first.
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
		items[i] = reportToResponse(r)
	}

	response.Success(c, gin.H{
		"reports": items,
		"page":    page,
		"limit":   limit,
		"count":   len(items),
	})
}

// reportToResponse converts a canonical Report entity to its wire shape.
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
	if r.EvidenceSnapshot != nil {
		snapshot := gin.H{}
		if r.EvidenceSnapshot.AuthorID != "" {
			snapshot["author_id"] = r.EvidenceSnapshot.AuthorID
		}
		if r.EvidenceSnapshot.AuthorUsername != "" {
			snapshot["author_username"] = r.EvidenceSnapshot.AuthorUsername
		}
		if r.EvidenceSnapshot.Title != "" {
			snapshot["title"] = r.EvidenceSnapshot.Title
		}
		if r.EvidenceSnapshot.Text != "" {
			snapshot["text"] = r.EvidenceSnapshot.Text
		}
		if r.EvidenceSnapshot.Status != "" {
			snapshot["status"] = r.EvidenceSnapshot.Status
		}
		if r.EvidenceSnapshot.ContentType != "" {
			snapshot["content_type"] = r.EvidenceSnapshot.ContentType
		}
		if r.EvidenceSnapshot.IsDeleted {
			snapshot["is_deleted"] = true
		}
		resp["evidence_snapshot"] = snapshot
	}
	if r.CaseID != nil {
		resp["case_id"] = *r.CaseID
	}

	return resp
}
