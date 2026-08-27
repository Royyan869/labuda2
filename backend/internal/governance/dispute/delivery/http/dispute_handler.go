package http

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/audit"
	"github.com/labuda/backend/internal/governance/dispute/application"
	"github.com/labuda/backend/internal/governance/dispute/entity"
	disputeRepo "github.com/labuda/backend/internal/governance/dispute/repository"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// disputeAuditLogger defines the audit logging interface used by DisputeHandler.
// Extracted locally to mirror the pattern used in moderation and support handlers.
type disputeAuditLogger interface {
	LogSafe(ctx context.Context, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{})
}

// DisputeHandler handles HTTP requests for dispute operations (admin only).
type DisputeHandler struct {
	disputeService   *application.DisputeService
	slaService       *application.DisputeSLAService
	db               *db.DB
	log              *zap.Logger
	adminAuditLogger disputeAuditLogger
}

// NewDisputeHandler creates a new DisputeHandler.
func NewDisputeHandler(
	disputeService *application.DisputeService,
	db *db.DB,
	log *zap.Logger,
	auditLogger audit.AdminAuditLogger,
) *DisputeHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &DisputeHandler{
		disputeService:   disputeService,
		slaService:       application.NewDisputeSLAService(),
		db:               db,
		log:              log,
		adminAuditLogger: auditLogger,
	}
}

// ResolveDisputeRequest holds the request body for resolving a dispute.
// The resolution type is determined by the endpoint (approve/reject),
// but admin notes can be provided for audit trail purposes.
type ResolveDisputeRequest struct {
	// Notes are the admin's reasoning for the resolution decision.
	// Stored for audit trail and dispute history.
	Notes *string `json:"notes"`
}

// ResolveDisputeApprove handles POST /api/v1/admin/disputes/{id}/approve
//
// Authorization: Admin only
// Service: DisputeService.ResolveDispute() with ResolutionRefund
// This resolves the dispute in favor of the buyer (refund).
// Admin attribution and notes are stored for audit trail.
func (h *DisputeHandler) ResolveDisputeApprove(c *gin.Context) {
	ctx := c.Request.Context()

	// Get authenticated admin ID from context
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse dispute ID
	idStr := c.Param("id")
	disputeID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid dispute ID")
		return
	}

	// Parse request body for optional notes
	var req ResolveDisputeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Notes are optional, don't fail on empty body
		// req.Notes will be nil
	}

	// Execute dispute resolution within transaction
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.disputeService.ResolveDispute(
			ctx,
			tx,
			disputeID,
			application.ResolutionRefund,
			adminID,
			req.Notes,
		)
	})

	if err != nil {
		h.log.Error("Failed to resolve dispute",
			zap.String("dispute_id", disputeID.String()),
			zap.String("admin_id", adminID.String()),
			zap.Error(err),
		)

		if handled := h.writeResolveDisputeError(c, err); handled {
			return
		}
		response.InternalServerError(c, "Failed to resolve dispute")
		return
	}

	h.adminAuditLogger.LogSafe(ctx, adminID,
		audit.ActionDisputeResolvedApproved,
		audit.TargetTypeDispute,
		disputeID,
		map[string]interface{}{
			"resolution":    "refund",
			"notes_present": req.Notes != nil,
		},
	)

	response.SuccessWithMessage(c, "Dispute resolved in favor of buyer (refund)", gin.H{
		"dispute_id": disputeID,
		"resolution": "refund",
	})
}

// ResolveDisputeReject handles POST /api/v1/admin/disputes/{id}/reject
//
// Authorization: Admin only
// Service: DisputeService.ResolveDispute() with ResolutionRelease
// This resolves the dispute in favor of the seller (release escrow).
// Admin attribution and notes are stored for audit trail.
func (h *DisputeHandler) ResolveDisputeReject(c *gin.Context) {
	ctx := c.Request.Context()

	// Get authenticated admin ID from context
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse dispute ID
	idStr := c.Param("id")
	disputeID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid dispute ID")
		return
	}

	// Parse request body for optional notes
	var req ResolveDisputeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Notes are optional, don't fail on empty body
		// req.Notes will be nil
	}

	// Execute dispute resolution within transaction
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.disputeService.ResolveDispute(
			ctx,
			tx,
			disputeID,
			application.ResolutionRelease,
			adminID,
			req.Notes,
		)
	})

	if err != nil {
		h.log.Error("Failed to resolve dispute",
			zap.String("dispute_id", disputeID.String()),
			zap.String("admin_id", adminID.String()),
			zap.Error(err),
		)

		if handled := h.writeResolveDisputeError(c, err); handled {
			return
		}
		response.InternalServerError(c, "Failed to resolve dispute")
		return
	}

	h.adminAuditLogger.LogSafe(ctx, adminID,
		audit.ActionDisputeResolvedRejected,
		audit.TargetTypeDispute,
		disputeID,
		map[string]interface{}{
			"resolution":    "release",
			"notes_present": req.Notes != nil,
		},
	)

	response.SuccessWithMessage(c, "Dispute resolved in favor of seller (release escrow)", gin.H{
		"dispute_id": disputeID,
		"resolution": "release",
	})
}

// ResolveDisputePartialSplit handles POST /api/v1/admin/disputes/{id}/partial-split
//
// Authorization: Admin only
// Service: DisputeService.ResolveDispute() with ResolutionPartialSplit
// This resolves the dispute with partial split:
// - Buyer gets refund for item price (subtotal)
// - Seller gets release for shipping fee (shipping_total)
// Admin attribution and notes are stored for audit trail.
func (h *DisputeHandler) ResolveDisputePartialSplit(c *gin.Context) {
	ctx := c.Request.Context()

	// Get authenticated admin ID from context
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse dispute ID
	idStr := c.Param("id")
	disputeID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid dispute ID")
		return
	}

	// Parse request body for optional notes
	var req ResolveDisputeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Notes are optional, don't fail on empty body
		// req.Notes will be nil
	}

	// Execute dispute resolution within transaction
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.disputeService.ResolveDispute(
			ctx,
			tx,
			disputeID,
			application.ResolutionPartialSplit,
			adminID,
			req.Notes,
		)
	})

	if err != nil {
		h.log.Error("Failed to resolve dispute with partial split",
			zap.String("dispute_id", disputeID.String()),
			zap.String("admin_id", adminID.String()),
			zap.Error(err),
		)

		if handled := h.writeResolveDisputeError(c, err); handled {
			return
		}
		response.InternalServerError(c, "Failed to resolve dispute")
		return
	}

	h.adminAuditLogger.LogSafe(ctx, adminID,
		audit.ActionDisputeResolvedPartialSplit,
		audit.TargetTypeDispute,
		disputeID,
		map[string]interface{}{
			"resolution":    "partial_split",
			"notes_present": req.Notes != nil,
		},
	)

	response.SuccessWithMessage(c, "Dispute resolved with partial split (buyer: item refund, seller: shipping fee)", gin.H{
		"dispute_id": disputeID,
		"resolution": "partial_split",
	})
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr ||
		s[len(s)-len(substr):] == substr ||
		containsMiddle(s, substr)))
}

func (h *DisputeHandler) writeResolveDisputeError(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		response.NotFound(c, "Dispute not found")
		return true
	case errors.Is(err, application.ErrDisputeResolutionCapabilityRequired):
		response.Forbidden(c, "finance.dispute.resolve capability required")
		return true
	case errors.Is(err, application.ErrDisputeResolveInvalidState):
		response.Error(c, 409, "INVALID_STATE", "Dispute cannot be resolved in current state")
		return true
	case errors.Is(err, application.ErrDisputeResolveAfterCompletion):
		response.Error(c, 409, "DISPUTE_CLOSED", "Cannot resolve dispute after order completion. Please negotiate directly with the seller outside the app.")
		return true
	default:
		return false
	}
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ============================================================================
// ADMIN QUERY ENDPOINTS
// ============================================================================

// DisputeListResponse contains the response for admin dispute listing.
type DisputeListResponse struct {
	Disputes []*DisputeListItem `json:"disputes"`
	Total    int64              `json:"total"`
}

// DisputeListItem represents a dispute in the admin list view.
type DisputeListItem struct {
	ID              uuid.UUID  `json:"id"`
	OrderID         uuid.UUID  `json:"order_id"`
	BuyerID         uuid.UUID  `json:"buyer_id"`
	SellerID        uuid.UUID  `json:"seller_id"`
	Reason          string     `json:"reason"`
	Description     *string    `json:"description,omitempty"`
	Status          string     `json:"status"`
	OpenedAt        time.Time  `json:"opened_at"`
	ResolvedAt      *time.Time `json:"resolved_at,omitempty"`
	ResolvedBy      *uuid.UUID `json:"resolved_by,omitempty"`
	ResolutionNotes *string    `json:"resolution_notes,omitempty"`

	// Computed fields for display
	BuyerUsername  *string `json:"buyer_username,omitempty"`
	BuyerAvatar    *string `json:"buyer_avatar,omitempty"`
	SellerUsername *string `json:"seller_username,omitempty"`
	SellerFarmName *string `json:"seller_farm_name,omitempty"`
	SellerAvatar   *string `json:"seller_avatar,omitempty"`

	// SLA Metrics
	NextAction           *string `json:"next_action,omitempty"`
	SLASummary           *string `json:"sla_summary,omitempty"`
	AdminResponseOverdue bool    `json:"admin_response_overdue"`
	ResolutionOverdue    bool    `json:"resolution_overdue"`
}

// DisputeDetailResponse contains the full dispute details for admin.
type DisputeDetailResponse struct {
	ID              uuid.UUID  `json:"id"`
	OrderID         uuid.UUID  `json:"order_id"`
	BuyerID         uuid.UUID  `json:"buyer_id"`
	SellerID        uuid.UUID  `json:"seller_id"`
	Reason          string     `json:"reason"`
	Description     *string    `json:"description,omitempty"`
	Status          string     `json:"status"`
	OpenedAt        time.Time  `json:"opened_at"`
	ResolvedAt      *time.Time `json:"resolved_at,omitempty"`
	ResolvedBy      *uuid.UUID `json:"resolved_by,omitempty"`
	ResolutionNotes *string    `json:"resolution_notes,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`

	// Evidence
	Evidence []string `json:"evidence,omitempty"`

	// Related order info (for context)
	OrderStatus *string `json:"order_status,omitempty"`
	OrderEscrow *string `json:"order_escrow_status,omitempty"`

	// Order financial and shipping info (for dispute resolution)
	EscrowAmount      *int64  `json:"escrow_amount,omitempty"`      // Total escrow amount (subtotal + shipping)
	ShippingReference *string `json:"shipping_reference,omitempty"` // Tracking number or phone reference
	ShippingCarrier   *string `json:"shipping_carrier,omitempty"`   // Shipping option name (JNE, J&T, etc.)

	// Computed fields for display
	BuyerUsername  *string `json:"buyer_username,omitempty"`
	BuyerAvatar    *string `json:"buyer_avatar,omitempty"`
	SellerUsername *string `json:"seller_username,omitempty"`
	SellerFarmName *string `json:"seller_farm_name,omitempty"`
	SellerAvatar   *string `json:"seller_avatar,omitempty"`

	// SLA Metrics
	NextAction                   *string `json:"next_action,omitempty"`
	SLASummary                   *string `json:"sla_summary,omitempty"`
	AdminResponseTime            *string `json:"admin_response_time,omitempty"`
	ResolutionTime               *string `json:"resolution_time,omitempty"`
	WaitingBuyerTime             *string `json:"waiting_buyer_time,omitempty"`
	WaitingSellerTime            *string `json:"waiting_seller_time,omitempty"`
	ActiveTime                   *string `json:"active_time,omitempty"`
	AdminResponseOverdue         bool    `json:"admin_response_overdue"`
	ResolutionOverdue            bool    `json:"resolution_overdue"`
	AdminResponseOverdueDuration *string `json:"admin_response_overdue_duration,omitempty"`
	ResolutionOverdueDuration    *string `json:"resolution_overdue_duration,omitempty"`
}

// ListDisputes handles GET /api/v1/admin/disputes
//
// Returns all disputes with optional filtering and pagination.
//
// Query parameters:
//   - status: Filter by dispute status (opened, resolved_refund, resolved_release)
//   - date_from: Filter by opening date (RFC3339 format)
//   - date_to: Filter by opening date (RFC3339 format)
//   - page: Page number (default: 1)
//   - page_size: Items per page (default: 20, max: 100)
//
// Authorization: Admin only
func (h *DisputeHandler) ListDisputes(c *gin.Context) {
	ctx := c.Request.Context()

	// Get admin ID from context for audit logging
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")
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
	filters := disputeRepo.DisputeListFilters{
		Page:     page,
		PageSize: pageSize,
	}

	if status != "" {
		filters.Status = &status
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

	// Execute query within transaction
	var disputes []*entity.Dispute
	var total int64
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		disputes, total, err = h.disputeService.ListAll(ctx, tx, filters)
		return err
	})

	if err != nil {
		h.log.Error("Failed to list disputes",
			zap.String("admin_id", adminID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to fetch disputes")
		return
	}

	// Convert to response DTOs
	items := make([]*DisputeListItem, len(disputes))
	for i, d := range disputes {
		items[i] = &DisputeListItem{
			ID:              d.ID,
			OrderID:         d.OrderID,
			BuyerID:         d.BuyerID,
			SellerID:        d.SellerID,
			Reason:          d.Reason,
			Description:     d.Description,
			Status:          string(d.Status),
			OpenedAt:        d.OpenedAt,
			ResolvedAt:      d.ResolvedAt,
			ResolvedBy:      d.ResolvedBy,
			ResolutionNotes: d.ResolutionNotes,
		}

		// Fetch buyer and seller info
		var buyerUsername, buyerAvatar, sellerUsername, sellerFarmName, sellerAvatar string
		_ = h.db.Pool().QueryRow(ctx, `
			SELECT COALESCE(username, ''), COALESCE(avatar_url, '')
			FROM user_profiles WHERE user_id = $1
		`, d.BuyerID).Scan(&buyerUsername, &buyerAvatar)

		_ = h.db.Pool().QueryRow(ctx, `
			SELECT COALESCE(up.username, '') as seller_username,
			       COALESCE(sp.store_name, '') as seller_farm_name,
			       COALESCE(up.avatar_url, '') as avatar_url
			FROM seller_profiles sp
			LEFT JOIN user_profiles up ON up.user_id = sp.user_id
			WHERE sp.user_id = $1
		`, d.SellerID).Scan(&sellerUsername, &sellerFarmName, &sellerAvatar)

		if buyerUsername != "" {
			items[i].BuyerUsername = &buyerUsername
		}
		if buyerAvatar != "" {
			items[i].BuyerAvatar = &buyerAvatar
		}
		if sellerUsername != "" {
			items[i].SellerUsername = &sellerUsername
		}
		if sellerFarmName != "" {
			items[i].SellerFarmName = &sellerFarmName
		}
		if sellerAvatar != "" {
			items[i].SellerAvatar = &sellerAvatar
		}
		// Compute SLA metrics
		slaMetrics := h.slaService.ComputeMetrics(d)
		items[i].NextAction = &slaMetrics.NextAction
		slaSummary := h.slaService.GetSLASummary(slaMetrics)
		items[i].SLASummary = &slaSummary
		items[i].AdminResponseOverdue = slaMetrics.AdminResponseOverdue
		items[i].ResolutionOverdue = slaMetrics.ResolutionOverdue
	}

	response.SuccessWithMeta(c, gin.H{
		"disputes": items,
	}, &response.Meta{
		Page:       page,
		PerPage:    pageSize,
		Total:      int(total),
		TotalPages: int((total + int64(pageSize) - 1) / int64(pageSize)),
	})
}

// GetDisputeDetail handles GET /api/v1/admin/disputes/:id
//
// Returns full dispute details including evidence and order context.
//
// Authorization: Admin only
func (h *DisputeHandler) GetDisputeDetail(c *gin.Context) {
	ctx := c.Request.Context()

	// Get admin ID from context
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse dispute ID
	disputeID, err := middleware.GetUUIDParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid dispute ID")
		return
	}

	// Execute query within transaction
	var detail *DisputeDetailResponse
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		// Get dispute
		dispute, err := h.disputeService.GetDisputeByID(ctx, tx, disputeID)
		if err != nil {
			return err
		}
		if dispute == nil {
			return nil // Will return 404
		}

		// Get evidence
		evidence, _ := h.disputeService.GetDisputeMedia(ctx, tx, disputeID)

		// Get order context
		var orderStatus, orderEscrow *string
		var escrowAmount *int64
		var shippingReference, shippingCarrier *string

		err = tx.QueryRow(ctx, `
			SELECT status, escrow_status,
			       (subtotal + shipping_total) as escrow_amount,
			       tracking_number, shipping_option_name
			FROM orders WHERE id = $1
		`, dispute.OrderID).Scan(&orderStatus, &orderEscrow, &escrowAmount, &shippingReference, &shippingCarrier)

		if err != nil {
			// Order might not exist, continue with nil values
		}

		// Fetch buyer and seller info
		var buyerUsername, buyerAvatar, sellerUsername, sellerFarmName, sellerAvatar string
		_ = tx.QueryRow(ctx, `
			SELECT COALESCE(username, ''), COALESCE(avatar_url, '')
			FROM user_profiles WHERE user_id = $1
		`, dispute.BuyerID).Scan(&buyerUsername, &buyerAvatar)

		_ = tx.QueryRow(ctx, `
			SELECT COALESCE(up.username, '') as seller_username,
			       COALESCE(sp.store_name, '') as seller_farm_name,
			       COALESCE(up.avatar_url, '') as avatar_url
			FROM seller_profiles sp
			LEFT JOIN user_profiles up ON up.user_id = sp.user_id
			WHERE sp.user_id = $1
		`, dispute.SellerID).Scan(&sellerUsername, &sellerFarmName, &sellerAvatar)

		detail = &DisputeDetailResponse{
			ID:              dispute.ID,
			OrderID:         dispute.OrderID,
			BuyerID:         dispute.BuyerID,
			SellerID:        dispute.SellerID,
			Reason:          dispute.Reason,
			Description:     dispute.Description,
			Status:          string(dispute.Status),
			OpenedAt:        dispute.OpenedAt,
			ResolvedAt:      dispute.ResolvedAt,
			ResolvedBy:      dispute.ResolvedBy,
			ResolutionNotes: dispute.ResolutionNotes,
			CreatedAt:       dispute.CreatedAt,
			UpdatedAt:       dispute.UpdatedAt,
			Evidence:        evidence,
			OrderStatus:     orderStatus,
			OrderEscrow:     orderEscrow,
		}
		detail.EscrowAmount = escrowAmount
		detail.ShippingReference = shippingReference
		detail.ShippingCarrier = shippingCarrier

		if buyerUsername != "" {
			detail.BuyerUsername = &buyerUsername
		}
		if buyerAvatar != "" {
			detail.BuyerAvatar = &buyerAvatar
		}
		if sellerUsername != "" {
			detail.SellerUsername = &sellerUsername
		}
		if sellerFarmName != "" {
			detail.SellerFarmName = &sellerFarmName
		}
		if sellerAvatar != "" {
			detail.SellerAvatar = &sellerAvatar
		}
		// Compute SLA metrics
		slaMetrics := h.slaService.ComputeMetrics(dispute)
		detail.NextAction = &slaMetrics.NextAction
		slaSummary := h.slaService.GetSLASummary(slaMetrics)
		detail.SLASummary = &slaSummary
		if slaMetrics.AdminResponseTime != nil {
			formatted := application.FormatDuration(slaMetrics.AdminResponseTime)
			detail.AdminResponseTime = &formatted
		}
		if slaMetrics.ResolutionTime != nil {
			formatted := application.FormatDuration(slaMetrics.ResolutionTime)
			detail.ResolutionTime = &formatted
		}
		if slaMetrics.WaitingBuyerTime != nil {
			formatted := application.FormatDuration(slaMetrics.WaitingBuyerTime)
			detail.WaitingBuyerTime = &formatted
		}
		if slaMetrics.WaitingSellerTime != nil {
			formatted := application.FormatDuration(slaMetrics.WaitingSellerTime)
			detail.WaitingSellerTime = &formatted
		}
		if slaMetrics.ActiveTime != nil {
			formatted := application.FormatDuration(slaMetrics.ActiveTime)
			detail.ActiveTime = &formatted
		}
		detail.AdminResponseOverdue = slaMetrics.AdminResponseOverdue
		detail.ResolutionOverdue = slaMetrics.ResolutionOverdue
		if slaMetrics.AdminResponseOverdueDuration != nil {
			formatted := application.FormatDuration(slaMetrics.AdminResponseOverdueDuration)
			detail.AdminResponseOverdueDuration = &formatted
		}
		if slaMetrics.ResolutionOverdueDuration != nil {
			formatted := application.FormatDuration(slaMetrics.ResolutionOverdueDuration)
			detail.ResolutionOverdueDuration = &formatted
		}

		return nil
	})

	if err != nil {
		h.log.Error("Failed to get dispute detail",
			zap.String("dispute_id", disputeID.String()),
			zap.String("admin_id", adminID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to fetch dispute")
		return
	}

	if detail == nil {
		response.NotFound(c, "Dispute not found")
		return
	}

	response.Success(c, detail)
}
