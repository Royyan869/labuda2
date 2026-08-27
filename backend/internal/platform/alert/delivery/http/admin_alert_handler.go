// Package http provides HTTP handlers for admin alert operations.
package http

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/audit"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/alert/application"
	alertentity "github.com/labuda/backend/internal/platform/alert/entity"
	"github.com/labuda/backend/internal/platform/alert/repository"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// AdminAlertHandler handles HTTP requests for admin alert operations.
type AdminAlertHandler struct {
	alertService     *application.AlertService
	db               *db.DB
	log              *zap.Logger
	adminAuditLogger audit.AdminAuditLogger
}

// NewAdminAlertHandler creates a new AdminAlertHandler.
func NewAdminAlertHandler(
	alertService *application.AlertService,
	db *db.DB,
	log *zap.Logger,
	adminAuditLogger audit.AdminAuditLogger,
) *AdminAlertHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &AdminAlertHandler{
		alertService:     alertService,
		db:               db,
		log:              log,
		adminAuditLogger: adminAuditLogger,
	}
}

// ============================================================================
// ADMIN ALERT LIST ENDPOINT
// ============================================================================

// ListAlerts handles GET /api/v1/admin/alerts
//
// Returns ALL alerts with optional filtering and pagination.
//
// Query parameters:
//   - status: Filter by status (active, acknowledged, resolved, false_positive)
//   - severity: Filter by severity (low, medium, high, critical)
//   - alert_type: Filter by alert type
//   - entity_type: Filter by entity type
//   - entity_id: Filter by entity ID
//   - date_from: Filter by creation date (RFC3339 format)
//   - date_to: Filter by creation date (RFC3339 format)
//   - page: Page number (default: 1)
//   - page_size: Items per page (default: 20, max: 100)
//
// Authorization: Admin only
func (h *AdminAlertHandler) ListAlerts(c *gin.Context) {
	ctx := c.Request.Context()

	// Get admin ID from context for audit logging
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	statusStr := c.Query("status")
	severityStr := c.Query("severity")
	alertTypeStr := c.Query("alert_type")
	entityType := c.Query("entity_type")
	entityIDStr := c.Query("entity_id")
	dateFromStr := c.Query("date_from")
	dateToStr := c.Query("date_to")

	// Validate pagination
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// Build filters
	filters := repository.AlertFilters{
		Limit:  pageSize,
		Offset: (page - 1) * pageSize,
	}

	if statusStr != "" {
		status := alertentity.AlertStatus(statusStr)
		filters.Status = &status
	}

	if severityStr != "" {
		severity := alertentity.AlertSeverity(severityStr)
		filters.Severity = &severity
	}

	if alertTypeStr != "" {
		alertType := alertentity.AlertType(alertTypeStr)
		filters.AlertType = &alertType
	}

	if entityType != "" {
		filters.EntityType = &entityType
	}

	if entityIDStr != "" {
		if entityID, err := uuid.Parse(entityIDStr); err == nil {
			filters.EntityID = &entityID
		}
	}

	if dateFromStr != "" {
		if t, err := time.Parse(time.RFC3339, dateFromStr); err == nil {
			filters.DateFrom = &t
		}
	}

	if dateToStr != "" {
		if t, err := time.Parse(time.RFC3339, dateToStr); err == nil {
			filters.DateTo = &t
		}
	}

	// Execute query
	result, err := h.alertService.ListAlerts(ctx, filters)
	if err != nil {
		h.log.Error("Failed to list alerts for admin",
			zap.String("admin_id", adminID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to fetch alerts")
		return
	}

	response.SuccessWithMeta(c, gin.H{
		"alerts": result.Alerts,
	}, &response.Meta{
		Page:       page,
		PerPage:    pageSize,
		Total:      int(result.Total),
		TotalPages: int((result.Total + int64(pageSize) - 1) / int64(pageSize)),
	})
}

// ============================================================================
// ADMIN ALERT DETAIL ENDPOINT
// ============================================================================

// GetAlertDetail handles GET /api/v1/admin/alerts/:id
//
// Returns FULL alert detail including metadata.
//
// Authorization: Admin only
func (h *AdminAlertHandler) GetAlertDetail(c *gin.Context) {
	ctx := c.Request.Context()

	// Get admin ID from context
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse alert ID
	alertID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid alert ID")
		return
	}

	// Get alert
	alert, err := h.alertService.GetAlert(ctx, alertID)
	if err != nil {
		h.log.Error("Failed to get alert detail for admin",
			zap.String("alert_id", alertID.String()),
			zap.String("admin_id", adminID.String()),
			zap.Error(err),
		)
		response.NotFound(c, "Alert not found")
		return
	}

	response.Success(c, alert)
}

// ============================================================================
// ALERT ACTION ENDPOINTS
// ============================================================================

// AcknowledgeAlertRequest represents the request body for acknowledging an alert.
type AcknowledgeAlertRequest struct {
	// Optional reason for acknowledging
	Reason *string `json:"reason"`
}

// AcknowledgeAlert handles POST /api/v1/admin/alerts/:id/acknowledge
//
// Marks an alert as acknowledged.
//
// Authorization: Admin only
func (h *AdminAlertHandler) AcknowledgeAlert(c *gin.Context) {
	ctx := c.Request.Context()

	// Get admin ID from context
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse alert ID
	alertID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid alert ID")
		return
	}

	// Acknowledge alert
	if err := h.alertService.AcknowledgeAlert(ctx, alertID, adminID); err != nil {
		h.log.Error("Failed to acknowledge alert",
			zap.String("alert_id", alertID.String()),
			zap.String("admin_id", adminID.String()),
			zap.Error(err),
		)
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"id":     alertID,
		"status": "acknowledged",
	})

	if h.adminAuditLogger != nil {
		h.adminAuditLogger.LogSafe(ctx, adminID,
			"admin_alert_acknowledged", "alert", alertID,
			nil,
		)
	}
}

// ResolveAlertRequest represents the request body for resolving an alert.
type ResolveAlertRequest struct {
	// Optional reason for resolving
	Reason *string `json:"reason"`
}

// ResolveAlert handles POST /api/v1/admin/alerts/:id/resolve
//
// Marks an alert as resolved.
//
// Authorization: Admin only
func (h *AdminAlertHandler) ResolveAlert(c *gin.Context) {
	ctx := c.Request.Context()

	// Get admin ID from context
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse alert ID
	alertID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid alert ID")
		return
	}

	// Resolve alert
	if err := h.alertService.ResolveAlert(ctx, alertID, adminID); err != nil {
		h.log.Error("Failed to resolve alert",
			zap.String("alert_id", alertID.String()),
			zap.String("admin_id", adminID.String()),
			zap.Error(err),
		)
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"id":     alertID,
		"status": "resolved",
	})

	if h.adminAuditLogger != nil {
		h.adminAuditLogger.LogSafe(ctx, adminID,
			"admin_alert_resolved", "alert", alertID,
			nil,
		)
	}
}

// MarkAsFalsePositiveRequest represents the request body for marking an alert as false positive.
type MarkAsFalsePositiveRequest struct {
	// Optional reason for marking as false positive
	Reason *string `json:"reason"`
}

// MarkAsFalsePositive handles POST /api/v1/admin/alerts/:id/false-positive
//
// Marks an alert as a false positive.
//
// Authorization: Admin only
func (h *AdminAlertHandler) MarkAsFalsePositive(c *gin.Context) {
	ctx := c.Request.Context()

	// Get admin ID from context
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse alert ID
	alertID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid alert ID")
		return
	}

	// Mark as false positive
	if err := h.alertService.MarkAsFalsePositive(ctx, alertID, adminID); err != nil {
		h.log.Error("Failed to mark alert as false positive",
			zap.String("alert_id", alertID.String()),
			zap.String("admin_id", adminID.String()),
			zap.Error(err),
		)
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"id":     alertID,
		"status": "false_positive",
	})

	if h.adminAuditLogger != nil {
		h.adminAuditLogger.LogSafe(ctx, adminID,
			"admin_alert_false_positive", "alert", alertID,
			nil,
		)
	}
}

// ============================================================================
// ALERT STATS ENDPOINT
// ============================================================================

// AlertStatsResponse represents alert statistics.
type AlertStatsResponse struct {
	Total         int64                `json:"total"`
	Active        int64                `json:"active"`
	Acknowledged  int64                `json:"acknowledged"`
	Resolved      int64                `json:"resolved"`
	FalsePositive int64                `json:"false_positive"`
	BySeverity    map[string]int64     `json:"by_severity"`
	ByType        map[string]int64     `json:"by_type"`
	RecentAlerts  []*alertentity.Alert `json:"recent_alerts,omitempty"`
}

// GetAlertStats handles GET /api/v1/admin/alerts/stats
//
// Returns alert statistics summary.
//
// Authorization: Admin only
func (h *AdminAlertHandler) GetAlertStats(c *gin.Context) {
	ctx := c.Request.Context()

	// Get admin ID from context
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	stats := &AlertStatsResponse{
		BySeverity: make(map[string]int64),
		ByType:     make(map[string]int64),
	}

	// Get counts for each status. open is the entity-layer alias for active
	// (migration 000177 added it to the DB CHECK constraint); both count toward Active.
	statuses := []alertentity.AlertStatus{
		alertentity.StatusActive,
		alertentity.StatusOpen,
		alertentity.StatusAcknowledged,
		alertentity.StatusResolved,
		alertentity.StatusFalsePositive,
	}

	for _, status := range statuses {
		result, err := h.alertService.ListAlerts(ctx, repository.AlertFilters{
			Status: &status,
			Limit:  1, // We only need the count
		})
		if err != nil {
			h.log.Error("Failed to get alert stats",
				zap.String("admin_id", adminID.String()),
				zap.String("status", string(status)),
				zap.Error(err),
			)
			continue
		}

		switch status {
		case alertentity.StatusActive:
			stats.Active = result.Total
		case alertentity.StatusOpen:
			stats.Active += result.Total // open is alias for active
		case alertentity.StatusAcknowledged:
			stats.Acknowledged = result.Total
		case alertentity.StatusResolved:
			stats.Resolved = result.Total
		case alertentity.StatusFalsePositive:
			stats.FalsePositive = result.Total
		}

		stats.Total += result.Total
	}

	// Active statuses for severity/type/recent queries — includes both open and active.
	activeStatuses := []alertentity.AlertStatus{alertentity.StatusActive, alertentity.StatusOpen}

	// Get counts by severity
	severities := []alertentity.AlertSeverity{
		alertentity.SeverityLow,
		alertentity.SeverityMedium,
		alertentity.SeverityHigh,
		alertentity.SeverityCritical,
	}

	for _, severity := range severities {
		result, err := h.alertService.ListAlerts(ctx, repository.AlertFilters{
			Severity: &severity,
			Statuses: activeStatuses,
			Limit:    1,
		})
		if err == nil {
			stats.BySeverity[string(severity)] = result.Total
		}
	}

	// Get counts by type
	alertTypes := []alertentity.AlertType{
		alertentity.AlertTypePaymentFailureSpike,
		alertentity.AlertTypePaymentStuck,
		alertentity.AlertTypeDisputeSpike,
		alertentity.AlertTypeSellerRisk,
		alertentity.AlertTypeCoinsAnomaly,
		alertentity.AlertTypeWithdrawalAnomaly,
	}

	for _, alertType := range alertTypes {
		result, err := h.alertService.ListAlerts(ctx, repository.AlertFilters{
			AlertType: &alertType,
			Statuses:  activeStatuses,
			Limit:     1,
		})
		if err == nil {
			stats.ByType[string(alertType)] = result.Total
		}
	}

	// Get recent alerts (limit 10)
	recentResult, err := h.alertService.ListAlerts(ctx, repository.AlertFilters{
		Statuses: activeStatuses,
		Limit:    10,
	})
	if err == nil && len(recentResult.Alerts) > 0 {
		stats.RecentAlerts = recentResult.Alerts
	}

	response.Success(c, stats)
}

// ============================================================================
// ALERT CLEANUP ENDPOINT
// ============================================================================

// CleanupOldAlerts handles POST /api/v1/admin/alerts/cleanup
//
// Manually triggers cleanup of old resolved alerts.
//
// Authorization: Admin only
func (h *AdminAlertHandler) CleanupOldAlerts(c *gin.Context) {
	ctx := c.Request.Context()

	// Get admin ID from context
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse retention days (default 90)
	retentionDaysStr := c.DefaultQuery("retention_days", "90")
	retentionDays, _ := strconv.Atoi(retentionDaysStr)
	if retentionDays < 1 {
		retentionDays = 90
	}

	// Cleanup
	deleted, err := h.alertService.CleanupOldAlerts(ctx, retentionDays)
	if err != nil {
		h.log.Error("Failed to cleanup old alerts",
			zap.String("admin_id", adminID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to cleanup alerts")
		return
	}

	h.log.Info("Manual alert cleanup completed",
		zap.String("admin_id", adminID.String()),
		zap.Int("deleted", deleted),
		zap.Int("retention_days", retentionDays),
	)

	response.Success(c, gin.H{
		"deleted":        deleted,
		"retention_days": retentionDays,
	})
}


