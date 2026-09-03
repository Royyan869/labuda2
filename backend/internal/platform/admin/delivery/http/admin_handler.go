// Package http provides HTTP handlers for admin operations.
package http

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/audit"
	notifservice "github.com/labuda/backend/internal/interaction/notification/service"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/admin/application"
	"github.com/labuda/backend/internal/platform/admin/repository"
	capabilityApp "github.com/labuda/backend/internal/platform/capability/application"
	"github.com/labuda/backend/internal/platform/response"
)

// FailedDeliveryQuerier is the minimal interface for querying failed notification
// deliveries. Satisfied by *service.DeliveryLogger.
type FailedDeliveryQuerier interface {
	GetFailedDeliveriesPaginated(ctx context.Context, page, pageSize int, since time.Time) (*notifservice.FailedDeliveryResult, error)
}

// AdminHandler handles HTTP requests for admin operations.
//
// This handler provides endpoints for:
// - User management (list, suspend, ban, activate)
// - Dashboard metrics
// - Audit log queries
// - SLA metrics
// - Notification delivery failure monitoring (O4)
//
// All endpoints require admin role authentication.
type AdminHandler struct {
	service           *application.AdminService
	adminAuditLogger  audit.AdminAuditLogger
	capabilityService *capabilityApp.CapabilityService
	slaService        *application.SLAService
	deliveryQuerier   FailedDeliveryQuerier // O4: notification delivery failure monitoring
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(
	service *application.AdminService,
	adminAuditLogger audit.AdminAuditLogger,
	capabilityService *capabilityApp.CapabilityService,
	slaService *application.SLAService,
) *AdminHandler {
	return &AdminHandler{
		service:           service,
		adminAuditLogger:  adminAuditLogger,
		capabilityService: capabilityService,
		slaService:        slaService,
	}
}

// SetDeliveryQuerier sets the notification delivery querier for O4 admin endpoint.
func (h *AdminHandler) SetDeliveryQuerier(q FailedDeliveryQuerier) {
	h.deliveryQuerier = q
}

// ============================================================================
// REQUEST/RESPONSE DTOs
// ============================================================================

// SuspendUserRequest represents the request body for suspending a user.
type SuspendUserRequest struct {
	Reason       string `json:"reason" binding:"required,min=1,max=500"`
	DurationDays *int   `json:"duration_days"`
}

// BanUserRequest represents the request body for banning a user.
type BanUserRequest struct {
	Reason string `json:"reason" binding:"required,min=1,max=500"`
}

// UnbanUserRequest represents the request body for unbanning a user.
type UnbanUserRequest struct {
	Reason string `json:"reason" binding:"required,min=1,max=500"`
}

// UserSummary represents a simplified user for list views.
type UserSummary struct {
	ID            uuid.UUID `json:"id"`
	FirebaseUID   string    `json:"firebase_uid"`
	Email         string    `json:"email"`
	PhoneNumber   *string   `json:"phone_number,omitempty"`
	EmailVerified bool      `json:"email_verified"`
	PhoneVerified bool      `json:"phone_verified"`
	AccountStatus string    `json:"account_status"`
	Role          string    `json:"role"`
	IsAdmin       bool      `json:"is_admin"`
	IsSuspended   bool      `json:"is_suspended"`
	CoinBalance   *int64    `json:"coin_balance,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	// Profile info (nullable)
	Username   *string `json:"username,omitempty"`
	IsVerified bool    `json:"is_verified"`
}

// UserDetails represents a complete user with all information.
type UserDetails struct {
	ID            uuid.UUID `json:"id"`
	FirebaseUID   string    `json:"firebase_uid"`
	Email         string    `json:"email"`
	PhoneNumber   *string   `json:"phone_number,omitempty"`
	EmailVerified bool      `json:"email_verified"`
	PhoneVerified bool      `json:"phone_verified"`
	AccountStatus string    `json:"account_status"`
	Role          string    `json:"role"`
	IsAdmin       bool      `json:"is_admin"`
	IsSeller      bool      `json:"is_seller"`
	IsSuspended   bool      `json:"is_suspended"`
	CoinBalance   *int64    `json:"coin_balance,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	// Profile
	Username   *string `json:"username,omitempty"`
	Bio        *string `json:"bio,omitempty"`
	AvatarURL  *string `json:"avatar_url,omitempty"`
	IsVerified bool    `json:"is_verified"`

	// Stats
	FollowersCount *int `json:"followers_count,omitempty"`
	FollowingCount *int `json:"following_count,omitempty"`

	// Location
	City     *string `json:"city,omitempty"`
	Province *string `json:"province,omitempty"`

	// Seller authority (Batch 52: canonical seller state visibility)
	SubscriptionStatus *string `json:"subscription_status,omitempty"`
	VerificationStatus *string `json:"verification_status,omitempty"`
	SellerPayable      *int64  `json:"seller_payable,omitempty"`
	SellerTier         *string `json:"seller_tier,omitempty"` // "basic"/"pro"/"elite"; nil if no reputation row

	// RecoverableSubscriptionPaymentID is set when a settled subscription payment
	// exists but has no seller_subscriptions row (webhook miss scenario).
	// Non-null signals the UI to show the "Recover Subscription" action.
	RecoverableSubscriptionPaymentID *string `json:"recoverable_subscription_payment_id,omitempty"`

	// Capabilities (PHASE 4: Added for capability management)
	Capabilities []string `json:"capabilities,omitempty"`

	// Warning counts (governance visibility, read-only).
	// WarningCount is the total warnings ever issued.
	// ActiveWarningCount is warnings currently in effect (is_active AND not expired).
	// SevereWarningCount is active warnings with level='severe'.
	WarningCount       int `json:"warning_count"`
	ActiveWarningCount int `json:"active_warning_count"`
	SevereWarningCount int `json:"severe_warning_count"`
}

// AdminMeResponse is the response for GET /api/v1/admin/me.
type AdminMeResponse struct {
	ID           string   `json:"id"`
	Email        string   `json:"email"`
	Username     string   `json:"username"`
	Role         string   `json:"role"`
	IsAdmin      bool     `json:"is_admin"`
	Capabilities []string `json:"capabilities"`
}

// GetAdminMe returns the current admin's identity from request context (zero extra DB calls).
func (h *AdminHandler) GetAdminMe(c *gin.Context) {
	actor := middleware.GetActorFromContext(c)
	if actor == nil {
		response.Unauthorized(c, "Authentication required")
		return
	}

	email := ""
	if claims, ok := middleware.GetUserFromContext(c); ok && claims != nil {
		email = claims.Email
	}

	username := ""
	if h.service != nil {
		if userDetails, err := h.service.GetUserDetails(c.Request.Context(), actor.ID); err == nil && userDetails != nil {
			if userDetails.Username != nil {
				username = *userDetails.Username
			}
		}
	}

	caps := actor.Capabilities
	if caps == nil {
		caps = []string{}
	}

	response.Success(c, AdminMeResponse{
		ID:           actor.ID.String(),
		Email:        email,
		Username:     username,
		Role:         actor.Role,
		IsAdmin:      actor.IsAdmin(),
		Capabilities: caps,
	})
}

// DashboardMetrics represents the dashboard summary.
type DashboardMetrics struct {
	Summary struct {
		TotalUsers       int64 `json:"total_users"`
		ActiveUsersToday int64 `json:"active_users_today"`
		ActiveSellers    int64 `json:"active_sellers"`
		TotalOrders      int64 `json:"total_orders"`
		OrdersToday      int64 `json:"orders_today"`
		PendingReports   int64 `json:"pending_reports"`
		TotalRevenue     int64 `json:"total_revenue"`
	} `json:"summary"`
	GeneratedAt time.Time `json:"generated_at"`
}

// AuditLogEntry represents a single audit log entry.
type AuditLogEntry struct {
	ID         uuid.UUID              `json:"id"`
	ActorID    uuid.UUID              `json:"actor_id"`
	ActionType string                 `json:"action_type"`
	TargetType string                 `json:"target_type"`
	TargetID   uuid.UUID              `json:"target_id"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
}

// ============================================================================
// USER MANAGEMENT ENDPOINTS
// ============================================================================

// ListUsers handles GET /api/v1/admin/users
//
// Returns a paginated list of users with optional filtering.
//
// Query parameters:
//   - page: page number (default: 1)
//   - page_size: items per page (default: 20, max: 100)
//   - q: search query (searches email, username)
//   - status: filter by account_status (active, suspended, banned)
//   - role: filter by role (user, admin)
//   - is_admin: filter by admin status (true/false)
//   - is_suspended: filter by suspension status (true/false)
//   - sort_by: field to sort by (default: created_at)
//   - sort_order: "asc" or "desc" (default: "desc")
func (h *AdminHandler) ListUsers(c *gin.Context) {
	ctx := c.Request.Context()

	// Get admin ID from context for audit logging
	actorID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	searchQuery := c.Query("q")
	status := c.Query("status")
	role := c.Query("role")
	isAdminStr := c.Query("is_admin")
	isSuspendedStr := c.Query("is_suspended")
	sortBy := c.DefaultQuery("sort_by", "created_at")
	sortOrder := c.DefaultQuery("sort_order", "desc")

	// Validate pagination
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// Build filters
	filters := repository.UserListFilters{
		SearchQuery: searchQuery,
		Status:      status,
		Role:        role,
		SortBy:      sortBy,
		SortOrder:   sortOrder,
		Page:        page,
		PageSize:    pageSize,
	}

	if isAdminStr == "true" {
		val := true
		filters.IsAdmin = &val
	} else if isAdminStr == "false" {
		val := false
		filters.IsAdmin = &val
	}

	if isSuspendedStr == "true" {
		val := true
		filters.IsSuspended = &val
	} else if isSuspendedStr == "false" {
		val := false
		filters.IsSuspended = &val
	}

	// Call service
	result, err := h.service.ListUsers(ctx, filters)
	if err != nil {
		response.InternalServerError(c, "Failed to fetch users")
		return
	}

	// Convert to response DTOs
	users := make([]UserSummary, len(result.Users))
	for i, u := range result.Users {
		users[i] = userSummaryFromRepo(u)
	}

	// Log admin action
	h.adminAuditLogger.LogSafe(ctx, actorID,
		"admin_users_listed", "user_list", uuid.Nil,
		map[string]interface{}{
			"page":      page,
			"page_size": pageSize,
			"total":     result.Total,
		},
	)

	response.SuccessWithMeta(c, gin.H{
		"users": users,
	}, &response.Meta{
		Page:       page,
		PerPage:    pageSize,
		Total:      result.Total,
		TotalPages: (result.Total + pageSize - 1) / pageSize,
	})
}

// GetUserDetails handles GET /api/v1/admin/users/:id
//
// Returns detailed information about a specific user.
func (h *AdminHandler) GetUserDetails(c *gin.Context) {
	ctx := c.Request.Context()

	// Get admin ID from context
	actorID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse user ID
	userID, err := middleware.GetUUIDParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}

	// Call service
	userDetails, err := h.service.GetUserDetails(ctx, userID)
	if err != nil {
		response.InternalServerError(c, "Failed to fetch user")
		return
	}

	if userDetails == nil {
		response.NotFound(c, "User not found")
		return
	}

	// PHASE 4: Fetch user capabilities
	var capabilities []string
	if h.capabilityService != nil {
		userCaps, err := h.capabilityService.GetUserCapabilities(ctx, userID)
		if err == nil {
			capabilities = make([]string, len(userCaps))
			for i, cap := range userCaps {
				capabilities[i] = cap.Capability
			}
		}
		// If capability fetch fails, continue without capabilities (non-breaking)
	}

	// Log admin action
	h.adminAuditLogger.LogSafe(ctx, actorID,
		"admin_user_viewed", "user", userID,
		nil,
	)

	response.Success(c, userDetailsFromRepo(*userDetails, capabilities))
}

// SuspendUser handles POST /api/v1/admin/users/:id/suspend
//
// Suspends a user account for a specified duration or indefinitely.
func (h *AdminHandler) SuspendUser(c *gin.Context) {
	ctx := c.Request.Context()

	// Get admin ID from context
	actorID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse user ID
	targetUserID, err := middleware.GetUUIDParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}

	// Parse request body
	var req SuspendUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Call service
	suspendReq := application.SuspendUserRequest{
		Reason:       req.Reason,
		DurationDays: req.DurationDays,
	}

	if err := h.service.SuspendUser(ctx, actorID, targetUserID, suspendReq); err != nil {
		response.InternalServerError(c, "Failed to suspend user")
		return
	}

	response.NoContent(c)
}

// ActivateUser handles POST /api/v1/admin/users/:id/activate
//
// Activates a suspended user account. Banned accounts require the
// explicit /unban endpoint with governance.user.unban capability.
func (h *AdminHandler) ActivateUser(c *gin.Context) {
	ctx := c.Request.Context()

	// Get admin ID from context
	actorID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse user ID
	targetUserID, err := middleware.GetUUIDParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}

	// Call service
	if err := h.service.ActivateUser(ctx, actorID, targetUserID); err != nil {
		if _, isBannedGuard := err.(*application.ErrCannotActivateBannedUser); isBannedGuard {
			response.Error(c, 409, "BANNED_USER", "Cannot activate banned user: use POST /unban with governance.user.unban capability")
			return
		}
		response.InternalServerError(c, "Failed to activate user")
		return
	}

	response.NoContent(c)
}

// BanUser handles POST /api/v1/admin/users/:id/ban
//
// Permanently bans a user account.
func (h *AdminHandler) BanUser(c *gin.Context) {
	ctx := c.Request.Context()

	// Get admin ID from context
	actorID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse user ID
	targetUserID, err := middleware.GetUUIDParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}

	// Parse request body
	var req BanUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Call service
	banReq := application.BanUserRequest{
		Reason: req.Reason,
	}

	if err := h.service.BanUser(ctx, actorID, targetUserID, banReq); err != nil {
		// Check if it's a BanSelfError
		if _, isBanSelf := err.(*application.BanSelfError); isBanSelf {
			response.BadRequest(c, "Cannot ban yourself")
			return
		}
		response.InternalServerError(c, "Failed to ban user")
		return
	}

	response.NoContent(c)
}

// UnbanUser handles POST /api/v1/admin/users/:id/unban
//
// Explicitly reverses a ban. This is the ONLY path that can revive a banned
// account. Requires governance.user.unban capability (separate from activate).
func (h *AdminHandler) UnbanUser(c *gin.Context) {
	ctx := c.Request.Context()

	// Get admin ID from context
	actorID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse user ID
	targetUserID, err := middleware.GetUUIDParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}

	// Parse request body
	var req UnbanUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Call service
	unbanReq := application.UnbanUserRequest{
		Reason: req.Reason,
	}

	if err := h.service.UnbanUser(ctx, actorID, targetUserID, unbanReq); err != nil {
		if _, isNotBanned := err.(*application.ErrCannotUnbanNonBannedUser); isNotBanned {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalServerError(c, "Failed to unban user")
		return
	}

	response.NoContent(c)
}

// ============================================================================
// DASHBOARD ENDPOINTS
// ============================================================================

// GetDashboard handles GET /api/v1/admin/dashboard
//
// Returns platform metrics and summary statistics for the admin dashboard.
func (h *AdminHandler) GetDashboard(c *gin.Context) {
	ctx := c.Request.Context()

	// Get admin ID from context
	actorID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Call service
	dashboardData, err := h.service.GetDashboard(ctx)
	if err != nil {
		response.InternalServerError(c, "Failed to fetch dashboard metrics")
		return
	}

	// Log admin action
	h.adminAuditLogger.LogSafe(ctx, actorID,
		"admin_dashboard_viewed", "dashboard", uuid.Nil,
		nil,
	)

	response.Success(c, gin.H{
		"data": DashboardMetrics{
			Summary: struct {
				TotalUsers       int64 "json:\"total_users\""
				ActiveUsersToday int64 "json:\"active_users_today\""
				ActiveSellers    int64 "json:\"active_sellers\""
				TotalOrders      int64 "json:\"total_orders\""
				OrdersToday      int64 "json:\"orders_today\""
				PendingReports   int64 "json:\"pending_reports\""
				TotalRevenue     int64 "json:\"total_revenue\""
			}{
				TotalUsers:       dashboardData.Metrics.TotalUsers,
				ActiveUsersToday: dashboardData.Metrics.ActiveUsersToday,
				ActiveSellers:    dashboardData.Metrics.ActiveSellers,
				TotalOrders:      dashboardData.Metrics.TotalOrders,
				OrdersToday:      dashboardData.Metrics.OrdersToday,
				PendingReports:   dashboardData.Metrics.PendingReports,
				TotalRevenue:     dashboardData.Metrics.TotalRevenue,
			},
			GeneratedAt: dashboardData.GeneratedAt,
		},
	})
}

// ============================================================================
// AUDIT LOG ENDPOINTS
// ============================================================================

// GetAuditLogs handles GET /api/v1/admin/audit-logs
//
// Returns a paginated list of audit logs with optional filtering.
//
// Query parameters:
//   - page: page number (default: 1)
//   - page_size: items per page (default: 20, max: 100)
//   - admin_id: filter by actor ID
//   - action: filter by action type
//   - target_type: filter by target type
//   - target_id: filter by target ID
func (h *AdminHandler) GetAuditLogs(c *gin.Context) {
	ctx := c.Request.Context()

	// Get admin ID from context
	actorID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	adminIDStr := c.Query("admin_id")
	action := c.Query("action")
	targetType := c.Query("target_type")
	targetIDStr := c.Query("target_id")

	// Validate pagination
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// Build filters
	filters := repository.AuditLogFilters{
		Action:     action,
		TargetType: targetType,
		Page:       page,
		PageSize:   pageSize,
	}

	if adminIDStr != "" {
		if adminID, err := uuid.Parse(adminIDStr); err == nil {
			filters.AdminID = &adminID
		}
	}

	if targetIDStr != "" {
		if targetID, err := uuid.Parse(targetIDStr); err == nil {
			filters.TargetID = &targetID
		}
	}

	// Call service
	result, err := h.service.ListAuditLogs(ctx, filters)
	if err != nil {
		response.InternalServerError(c, "Failed to fetch audit logs")
		return
	}

	// Convert to response DTOs
	logs := make([]AuditLogEntry, len(result.Logs))
	for i, log := range result.Logs {
		logs[i] = auditLogEntryFromRepo(log)
	}

	// Log admin action
	h.adminAuditLogger.LogSafe(ctx, actorID,
		"admin_audit_logs_viewed", "audit_logs", uuid.Nil,
		map[string]interface{}{
			"page":      page,
			"page_size": pageSize,
		},
	)

	response.SuccessWithMeta(c, gin.H{
		"logs": logs,
	}, &response.Meta{
		Page:       page,
		PerPage:    pageSize,
		Total:      result.Total,
		TotalPages: (result.Total + pageSize - 1) / pageSize,
	})
}

// ============================================================================
// SLA METRICS ENDPOINTS
// ============================================================================

// SLAMetricsResponse represents the SLA metrics response.
type SLAMetricsResponse struct {
	Support          *application.SLAMetricBreakdown       `json:"support"`
	Dispute          *application.SLAMetricBreakdown       `json:"dispute"`
	AdminPerformance []application.AdminPerformanceMetrics `json:"admin_performance"`
	GeneratedAt      time.Time                             `json:"generated_at"`
}

// FormatDuration formats a duration for display.
func FormatDuration(d *time.Duration) *string {
	if d == nil {
		return nil
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	var result string
	if hours > 48 {
		days := hours / 24
		remainingHours := hours % 24
		result = fmt.Sprintf("%dd %dh", days, remainingHours)
	} else if hours > 0 {
		result = fmt.Sprintf("%dh %dm", hours, minutes)
	} else {
		result = fmt.Sprintf("%dm", minutes)
	}
	return &result
}

// GetSLAMetrics handles GET /api/v1/admin/sla/metrics
//
// Returns aggregated SLA metrics for support and disputes.
func (h *AdminHandler) GetSLAMetrics(c *gin.Context) {
	ctx := c.Request.Context()

	// Get admin ID from context
	actorID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Call service
	metrics, err := h.slaService.GetSLAMetrics(ctx)
	if err != nil {
		response.InternalServerError(c, "Failed to fetch SLA metrics")
		return
	}

	// Log admin action
	h.adminAuditLogger.LogSafe(ctx, actorID,
		"admin_sla_metrics_viewed", "sla_metrics", uuid.Nil,
		nil,
	)

	response.Success(c, gin.H{
		"data": SLAMetricsResponse{
			Support:          metrics.Support,
			Dispute:          metrics.Dispute,
			AdminPerformance: metrics.AdminPerformance,
			GeneratedAt:      metrics.GeneratedAt,
		},
	})
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// userSummaryFromRepo converts a repository UserSummary to a handler UserSummary.
func userSummaryFromRepo(u repository.UserSummary) UserSummary {
	return UserSummary{
		ID:            u.ID,
		FirebaseUID:   u.FirebaseUID,
		Email:         u.Email,
		PhoneNumber:   u.PhoneNumber,
		EmailVerified: u.EmailVerified,
		PhoneVerified: u.PhoneVerified,
		AccountStatus: u.AccountStatus,
		Role:          u.Role,
		Username:      u.Username,
		IsVerified:    u.IsVerified,
		CoinBalance:   u.CoinBalance,
		IsAdmin:       u.Role == "admin",
		IsSuspended:   u.AccountStatus == "suspended",
		CreatedAt:     toTime(u.CreatedAt),
		UpdatedAt:     toTime(u.UpdatedAt),
	}
}

// userDetailsFromRepo converts a repository UserDetails to a handler UserDetails.
func userDetailsFromRepo(u repository.UserDetails, capabilities []string) UserDetails {
	return UserDetails{
		ID:                 u.ID,
		FirebaseUID:        u.FirebaseUID,
		Email:              u.Email,
		PhoneNumber:        u.PhoneNumber,
		EmailVerified:      u.EmailVerified,
		PhoneVerified:      u.PhoneVerified,
		AccountStatus:      u.AccountStatus,
		Role:               u.Role,
		Username:           u.Username,
		Bio:                u.Bio,
		AvatarURL:          u.AvatarURL,
		IsVerified:         u.IsVerified,
		FollowersCount:     u.FollowersCount,
		FollowingCount:     u.FollowingCount,
		City:               u.City,
		Province:           u.Province,
		CoinBalance:        u.CoinBalance,
		IsAdmin:            u.Role == "admin",
		IsSeller:           u.HasSellerProfile,
		IsSuspended:        u.AccountStatus == "suspended",
		CreatedAt:          toTime(u.CreatedAt),
		UpdatedAt:          toTime(u.UpdatedAt),
		SubscriptionStatus: u.SubscriptionStatus,
		VerificationStatus: u.VerificationStatus,
		SellerPayable:      u.SellerPayable,
		SellerTier:         u.SellerTier,
		Capabilities:       capabilities,
		RecoverableSubscriptionPaymentID: func() *string {
			if u.RecoverableSubscriptionPaymentID == nil {
				return nil
			}
			s := u.RecoverableSubscriptionPaymentID.String()
			return &s
		}(),
		WarningCount:       u.WarningCount,
		ActiveWarningCount: u.ActiveWarningCount,
		SevereWarningCount: u.SevereWarningCount,
	}
}

// auditLogEntryFromRepo converts a repository AuditLogEntry to a handler AuditLogEntry.
func auditLogEntryFromRepo(log repository.AuditLogEntry) AuditLogEntry {
	return AuditLogEntry{
		ID:         log.ID,
		ActorID:    log.ActorID,
		ActionType: log.ActionType,
		TargetType: log.TargetType,
		TargetID:   log.TargetID,
		Metadata:   log.Metadata,
		CreatedAt:  toTime(log.CreatedAt),
	}
}

// toTime converts interface{} to time.Time.
func toTime(t interface{}) time.Time {
	switch v := t.(type) {
	case time.Time:
		return v
	default:
		return time.Time{}
	}
}

// ============================================================================
// O4: NOTIFICATION DELIVERY FAILURE MONITORING
// ============================================================================

// FailedDeliveryResponse is the JSON shape for a single failed delivery entry.
type FailedDeliveryResponse struct {
	ID             uuid.UUID              `json:"id"`
	NotificationID uuid.UUID              `json:"notification_id"`
	RecipientID    uuid.UUID              `json:"recipient_id"`
	Channel        string                 `json:"channel"`
	Status         string                 `json:"status"`
	Reason         string                 `json:"reason"`
	Metadata       map[string]interface{} `json:"metadata"`
	CreatedAt      time.Time              `json:"created_at"`
}

// GetFailedDeliveries handles GET /api/v1/admin/notifications/failed-deliveries
//
// Returns paginated list of failed notification delivery attempts for operator
// monitoring. Read-only.
//
// Query parameters:
//   - page (default 1)
//   - page_size (default 20, max 100)
//   - since (RFC3339, default 24h ago)
func (h *AdminHandler) GetFailedDeliveries(c *gin.Context) {
	if h.deliveryQuerier == nil {
		response.InternalServerError(c, "Delivery monitoring not configured")
		return
	}

	ctx := c.Request.Context()

	actorID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// Parse time filter
	since := time.Now().Add(-24 * time.Hour)
	if sinceStr := c.Query("since"); sinceStr != "" {
		if parsed, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = parsed
		}
	}

	result, err := h.deliveryQuerier.GetFailedDeliveriesPaginated(ctx, page, pageSize, since)
	if err != nil {
		response.InternalServerError(c, "Failed to fetch delivery failures")
		return
	}

	entries := make([]FailedDeliveryResponse, len(result.Entries))
	for i, e := range result.Entries {
		entries[i] = FailedDeliveryResponse{
			ID:             e.ID,
			NotificationID: e.NotificationID,
			RecipientID:    e.UserID,
			Channel:        e.Channel,
			Status:         e.Status,
			Reason:         e.Reason,
			Metadata:       e.Metadata,
			CreatedAt:      e.CreatedAt,
		}
	}

	h.adminAuditLogger.LogSafe(ctx, actorID,
		"admin_failed_deliveries_viewed", "notification_delivery", uuid.Nil,
		map[string]interface{}{
			"page":      page,
			"page_size": pageSize,
			"total":     result.Total,
		},
	)

	response.SuccessWithMeta(c, gin.H{
		"deliveries": entries,
	}, &response.Meta{
		Page:       page,
		PerPage:    pageSize,
		Total:      result.Total,
		TotalPages: (result.Total + pageSize - 1) / pageSize,
	})
}

