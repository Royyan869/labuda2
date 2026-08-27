// Package http provides HTTP handlers for admin payout operations.
package http

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/audit"
	"github.com/labuda/backend/internal/finance/application"
	withdrawrepo "github.com/labuda/backend/internal/finance/infrastructure/repository"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// WhitelistAuditRow is the read-only DTO returned by the whitelist audit endpoint.
type WhitelistAuditRow struct {
	ID        uuid.UUID  `json:"id"`
	SellerID  *uuid.UUID `json:"seller_id,omitempty"`
	Action    string     `json:"action"`
	ActorID   string     `json:"actor_id"`
	Reason    string     `json:"reason"`
	Source    string     `json:"source"`
	CreatedAt string     `json:"created_at"`
}

// AdminPayoutHandler handles HTTP requests for admin payout operations.
//
// This handler provides endpoints for:
// - Listing pending/approved/failed withdrawals
// - Viewing withdrawal details
// - Approving withdrawals (PROCESSING -> SUBMITTED)
// - Rejecting withdrawals (PROCESSING/REQUESTED -> FAILED)
// - Manual completion for exceptional cases
//
// All endpoints require admin role authentication and are audit logged.
type AdminPayoutHandler struct {
	withdrawService    *application.WithdrawService
	withdrawRepo       *withdrawrepo.WithdrawRepository
	whitelistAuditRepo withdrawrepo.WhitelistAuditRepository // nil when not wired
	db                 *db.DB
	adminAuditLogger   audit.AdminAuditLogger
	log                *zap.Logger
}

// NewAdminPayoutHandler creates a new admin payout handler.
// whitelistAuditRepo may be nil; the whitelist audit endpoint will return 503
// rather than panicking when it is not wired.
func NewAdminPayoutHandler(
	withdrawService *application.WithdrawService,
	database *db.DB,
	adminAuditLogger audit.AdminAuditLogger,
	log *zap.Logger,
	whitelistAuditRepo withdrawrepo.WhitelistAuditRepository,
) *AdminPayoutHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &AdminPayoutHandler{
		withdrawService:    withdrawService,
		withdrawRepo:       withdrawrepo.NewWithdrawRepository(),
		whitelistAuditRepo: whitelistAuditRepo,
		db:                 database,
		adminAuditLogger:   adminAuditLogger,
		log:                log,
	}
}

// ============================================================================
// REQUEST/RESPONSE DTOs
// ============================================================================

// ApproveWithdrawalRequest represents the request body for approving a withdrawal.
type ApproveWithdrawalRequest struct {
	// Notes is optional admin notes about the approval
	Notes string `json:"notes"`
}

// RejectWithdrawalRequest represents the request body for rejecting a withdrawal.
type RejectWithdrawalRequest struct {
	// Reason is required - why the withdrawal is being rejected
	Reason string `json:"reason" binding:"required,min=1,max=500"`
}

// MarkWithdrawalProcessedRequest allows an optional manual payout reference.
type MarkWithdrawalProcessedRequest struct {
	ReferenceCode string `json:"reference_code"`
}

// WithdrawalDetail represents a withdrawal with full details for admin view.
type WithdrawalDetail struct {
	ID                    uuid.UUID `json:"id"`
	SellerID              uuid.UUID `json:"seller_id"`
	Amount                int64     `json:"amount"`             // Requested withdrawal amount (reserved from seller payable)
	FeeAmount             int64     `json:"fee_amount"`         // Flat withdrawal fee (PASS_18H: deducted from Amount, not added on top)
	NetPayoutAmount       int64     `json:"net_payout_amount"`  // Amount - FeeAmount — what actually reaches the seller's bank
	TotalDebitAmount      int64     `json:"total_debit_amount"` // Equal to Amount — the full reservation debited from seller payable
	Status                string    `json:"status"`
	BankNameSnapshot      string    `json:"bank_name_snapshot"`
	BankCodeSnapshot      string    `json:"bank_code_snapshot"`
	AccountNumberSnapshot string    `json:"account_number_snapshot"`
	AccountHolderSnapshot string    `json:"account_holder_snapshot"`
	ExternalReferenceID   string    `json:"external_reference_id,omitempty"`
	GatewayReferenceID    string    `json:"gateway_reference_id,omitempty"`
	RetryCount            int       `json:"retry_count"`
	FailureReason         string    `json:"failure_reason,omitempty"`
	CreatedAt             string    `json:"created_at"`
	UpdatedAt             string    `json:"updated_at"`
	SubmittedAt           *string   `json:"submitted_at,omitempty"`
	SettledAt             *string   `json:"settled_at,omitempty"`

	// Seller info (denormalized for admin view)
	SellerEmail    *string `json:"seller_email,omitempty"`
	SellerUsername *string `json:"seller_username,omitempty"`
	SellerFarmName *string `json:"seller_farm_name,omitempty"`
}

// WithdrawalListResponse represents a paginated list of withdrawals.
type WithdrawalListResponse struct {
	Withdrawals []WithdrawalDetail `json:"withdrawals"`
	Page        int                `json:"page"`
	PerPage     int                `json:"per_page"`
	Total       int64              `json:"total"`
	TotalPages  int                `json:"total_pages"`
}

// ============================================================================
// LIST WITHDRAWALS
// ============================================================================

// ListWithdrawals handles GET /api/v1/admin/payouts/withdrawals
//
// Returns a paginated list of withdrawals with optional filtering.
//
// Query parameters:
//   - page: page number (default: 1)
//   - page_size: items per page (default: 20, max: 100)
//   - status: filter by status (REQUESTED, PROCESSING, SUBMITTED, SETTLING, SETTLED, COMPLETED, FAILED, FAILED_RETRYABLE, FAILED_FINAL, PILOT_BLOCKED)
//   - seller_id: filter by seller ID
//   - sort_by: field to sort by (default: created_at)
//   - sort_order: "asc" or "desc" (default: "desc")
//   - stuck: "true" to return only SUBMITTED/SETTLING payouts older than 30 minutes (read-only, no auto-fix)
func (h *AdminPayoutHandler) ListWithdrawals(c *gin.Context) {
	ctx := c.Request.Context()

	// Get admin ID from context for audit logging
	actorID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// stuck=true path: returns SUBMITTED/SETTLING payouts older than threshold, no pagination.
	if c.Query("stuck") == "true" {
		h.listStuckWithdrawals(c, actorID)
		return
	}

	// Parse query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	statusStr := c.Query("status")
	sellerIDStr := c.Query("seller_id")
	sortBy := c.DefaultQuery("sort_by", "created_at")
	sortOrder := c.DefaultQuery("sort_order", "desc")

	// Validate pagination
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// Parse seller ID if provided
	var sellerID *uuid.UUID
	if sellerIDStr != "" {
		if id, err := uuid.Parse(sellerIDStr); err == nil {
			sellerID = &id
		}
	}

	// Parse status filter
	var status *string
	if statusStr != "" {
		status = &statusStr
	}

	// Build filters
	filters := withdrawrepo.WithdrawalListFilters{
		Status:   status,
		SellerID: sellerID,
		SortBy:   sortBy,
		SortDesc: sortOrder == "desc",
		Page:     page,
		PageSize: pageSize,
	}

	// Query withdrawals
	var withdrawals []*withdrawrepo.Withdrawal
	var total int64
	var err error

	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		withdrawals, err = h.withdrawRepo.ListWithFilters(ctx, tx, filters)
		if err != nil {
			return err
		}
		total, err = h.withdrawRepo.CountWithFilters(ctx, tx, filters)
		return err
	})

	if err != nil {
		h.log.Error("Failed to list withdrawals", zap.Error(err))
		response.InternalServerError(c, "Failed to fetch withdrawals")
		return
	}

	// Convert to response DTOs
	items := make([]WithdrawalDetail, len(withdrawals))
	for i, w := range withdrawals {
		items[i] = h.withdrawalToDetail(w)
	}

	// Log admin action
	h.adminAuditLogger.LogSafe(ctx, actorID,
		"admin_withdrawals_listed", "withdrawal_list", uuid.Nil,
		map[string]interface{}{
			"page":      page,
			"page_size": pageSize,
			"total":     total,
			"status":    status,
		},
	)

	response.SuccessWithMeta(c, gin.H{
		"withdrawals": items,
	}, &response.Meta{
		Page:       page,
		PerPage:    pageSize,
		Total:      int(total),
		TotalPages: int((total + int64(pageSize) - 1) / int64(pageSize)),
	})
}

// listStuckWithdrawals handles the stuck=true branch of ListWithdrawals.
// Returns SUBMITTED/SETTLING payouts older than 30 minutes. Read-only; no auto-fix.
func (h *AdminPayoutHandler) listStuckWithdrawals(c *gin.Context, actorID uuid.UUID) {
	reqCtx := c.Request.Context()
	const stuckThreshold = 30 * time.Minute
	cutoff := time.Now().Add(-stuckThreshold)

	var stuck []*withdrawrepo.Withdrawal
	err := h.db.WithTx(reqCtx, func(tx db.Tx) error {
		var e error
		stuck, e = h.withdrawRepo.GetStuckPayouts(reqCtx, tx, cutoff, 200)
		return e
	})
	if err != nil {
		h.log.Error("Failed to query stuck withdrawals", zap.Error(err))
		response.InternalServerError(c, "Failed to fetch stuck withdrawals")
		return
	}

	items := make([]WithdrawalDetail, len(stuck))
	for i, w := range stuck {
		items[i] = h.withdrawalToDetail(w)
	}

	h.adminAuditLogger.LogSafe(reqCtx, actorID,
		"admin_stuck_withdrawals_listed", "withdrawal_list", uuid.Nil,
		map[string]interface{}{
			"stuck_threshold_minutes": int(stuckThreshold.Minutes()),
			"count":                   len(stuck),
		},
	)

	response.Success(c, gin.H{
		"withdrawals":             items,
		"stuck_threshold_minutes": int(stuckThreshold.Minutes()),
		"count":                   len(stuck),
		"note":                    "read-only visibility — no auto-fix applied",
	})
}

// ============================================================================
// WITHDRAWAL DETAILS
// ============================================================================

// GetWithdrawalDetails handles GET /api/v1/admin/payouts/withdrawals/:id
//
// Returns detailed information about a specific withdrawal.
func (h *AdminPayoutHandler) GetWithdrawalDetails(c *gin.Context) {
	ctx := c.Request.Context()

	// Get admin ID from context
	actorID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse withdrawal ID
	withdrawalID, err := middleware.GetUUIDParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid withdrawal ID")
		return
	}

	// Get withdrawal
	var withdrawal *withdrawrepo.Withdrawal
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		withdrawal, err = h.withdrawRepo.GetByID(ctx, tx, withdrawalID)
		return err
	})

	if err != nil {
		h.log.Error("Failed to get withdrawal", zap.Error(err))
		response.InternalServerError(c, "Failed to fetch withdrawal")
		return
	}

	if withdrawal == nil {
		response.NotFound(c, "Withdrawal not found")
		return
	}

	// Log admin action
	h.adminAuditLogger.LogSafe(ctx, actorID,
		"admin_withdrawal_viewed", "withdrawal", withdrawalID,
		map[string]interface{}{
			"status":    withdrawal.Status,
			"amount":    withdrawal.Amount,
			"seller_id": withdrawal.SellerID,
		},
	)

	response.Success(c, gin.H{
		"withdrawal": h.withdrawalToDetail(withdrawal),
	})
}

// ============================================================================
// APPROVE WITHDRAWAL
// ============================================================================

// ApproveWithdrawal handles POST /api/v1/admin/payouts/withdrawals/:id/approve
//
// Approves a withdrawal request, transitioning it from pending to approved
// in the canonical wallet+finance flow. The payout worker remains dormant;
// approved means the reserved amount has been committed for manual payout.
//
// SLICE 3: MIGRATED to capability-based auth with finance.withdraw.review
// This action is audit logged.
func (h *AdminPayoutHandler) ApproveWithdrawal(c *gin.Context) {
	ctx := c.Request.Context()

	// SLICE 3: Handler-level defense - check capability explicitly
	// This provides defense-in-depth even if middleware is bypassed
	actor := middleware.GetActorFromContext(c)
	if actor == nil {
		response.Unauthorized(c, "Authentication required")
		return
	}
	if !actor.HasCapability("finance.withdraw.review") {
		response.Forbidden(c, "Insufficient permissions: finance.withdraw.review required")
		return
	}

	// Get admin ID from context
	actorID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse withdrawal ID
	withdrawalID, err := middleware.GetUUIDParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid withdrawal ID")
		return
	}

	// Parse request body (notes are optional)
	var req ApproveWithdrawalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Allow empty body for approval
		req = ApproveWithdrawalRequest{Notes: ""}
	}

	withdrawal, err := h.withdrawService.GetWithdrawal(ctx, withdrawalID)
	if err != nil || withdrawal == nil {
		response.NotFound(c, "Withdrawal not found")
		return
	}
	statusBefore := withdrawal.Status

	if err := h.withdrawService.ApproveWithdraw(ctx, actorID, withdrawalID); err != nil {
		if strings.Contains(err.Error(), "cannot approve") {
			h.log.Warn("Invalid status for approval",
				zap.String("withdrawal_id", withdrawalID.String()),
				zap.String("status", string(withdrawal.Status)),
			)
			response.BadRequest(c, "Cannot approve withdrawal in current status: "+string(withdrawal.Status))
			return
		}
		h.log.Error("Failed to approve withdrawal", zap.Error(err))
		response.InternalServerError(c, "Failed to approve withdrawal")
		return
	}

	// Log admin action (non-transactional; service logs atomically within tx)
	h.adminAuditLogger.LogSafe(ctx, actorID,
		"admin_withdrawal_approved", "withdrawal", withdrawalID,
		map[string]interface{}{
			"amount":        withdrawal.Amount,
			"seller_id":     withdrawal.SellerID.String(),
			"admin_notes":   req.Notes,
			"status_before": statusBefore,
			"status_after":  withdrawrepo.WithdrawalStatusProcessing,
		},
	)

	response.Success(c, gin.H{
		"message":       "Withdrawal approved and payout committed for manual execution",
		"withdrawal_id": withdrawalID.String(),
		"status":        string(withdrawrepo.WithdrawalStatusProcessing),
	})
}

// ============================================================================
// REJECT WITHDRAWAL
// ============================================================================

// RejectWithdrawal handles POST /api/v1/admin/payouts/withdrawals/:id/reject
//
// Rejects a withdrawal request, returning funds to the seller's payable
// account. Works for both pending reservations and approved-but-not-completed
// manual payouts.
//
// SLICE 3: MIGRATED to capability-based auth with finance.withdraw.review
// This action is audit logged and requires a reason.
func (h *AdminPayoutHandler) RejectWithdrawal(c *gin.Context) {
	ctx := c.Request.Context()

	// SLICE 3: Handler-level defense - check capability explicitly
	// This provides defense-in-depth even if middleware is bypassed
	actor := middleware.GetActorFromContext(c)
	if actor == nil {
		response.Unauthorized(c, "Authentication required")
		return
	}
	if !actor.HasCapability("finance.withdraw.review") {
		response.Forbidden(c, "Insufficient permissions: finance.withdraw.review required")
		return
	}

	// Get admin ID from context
	actorID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse withdrawal ID
	withdrawalID, err := middleware.GetUUIDParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid withdrawal ID")
		return
	}

	// Parse request body
	var req RejectWithdrawalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	withdrawal, err := h.withdrawService.GetWithdrawal(ctx, withdrawalID)
	if err != nil || withdrawal == nil {
		response.NotFound(c, "Withdrawal not found")
		return
	}
	statusBefore := withdrawal.Status

	if err := h.withdrawService.RejectWithdraw(ctx, actorID, withdrawalID); err != nil {
		if strings.Contains(err.Error(), "cannot reject") {
			h.log.Warn("Invalid status for rejection",
				zap.String("withdrawal_id", withdrawalID.String()),
				zap.String("status", string(withdrawal.Status)),
			)
			response.BadRequest(c, "Cannot reject withdrawal in current status: "+string(withdrawal.Status))
			return
		}
		h.log.Error("Failed to reject withdrawal", zap.Error(err))
		response.InternalServerError(c, "Failed to reject withdrawal")
		return
	}

	// Log admin action
	h.adminAuditLogger.LogSafe(ctx, actorID,
		"admin_withdrawal_rejected", "withdrawal", withdrawalID,
		map[string]interface{}{
			"amount":        withdrawal.Amount,
			"seller_id":     withdrawal.SellerID.String(),
			"reason":        req.Reason,
			"status_before": statusBefore,
			"status_after":  withdrawrepo.WithdrawalStatusFailed,
		},
	)

	response.Success(c, gin.H{
		"message":       "Withdrawal rejected and funds returned to seller",
		"withdrawal_id": withdrawalID.String(),
		"status":        string(withdrawrepo.WithdrawalStatusFailed),
	})
}

// ============================================================================
// MARK WITHDRAWAL AS PROCESSED (MANUAL COMPLETION)
// ============================================================================

// MarkWithdrawalProcessed handles POST /api/v1/admin/payouts/withdrawals/:id/mark-processed
//
// Marks a withdrawal as processed after manual bank transfer.
//
// CRITICAL: This endpoint is for MANUAL COMPLETION ONLY, used when:
// 1. External bank transfer was done manually (outside the payment gateway)
// 2. Exceptional recovery requiring admin intervention
//
// SLICE 3: MIGRATED to capability-based auth with finance.withdraw.review
// This action is audit logged.
func (h *AdminPayoutHandler) MarkWithdrawalProcessed(c *gin.Context) {
	ctx := c.Request.Context()

	// SLICE 3: Handler-level defense - check capability explicitly
	actor := middleware.GetActorFromContext(c)
	if actor == nil {
		response.Unauthorized(c, "Authentication required")
		return
	}
	if !actor.HasCapability("finance.withdraw.review") {
		response.Forbidden(c, "Insufficient permissions: finance.withdraw.review required")
		return
	}

	// Get admin ID from context
	actorID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse withdrawal ID
	withdrawalID, err := middleware.GetUUIDParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid withdrawal ID")
		return
	}

	withdrawal, err := h.withdrawService.GetWithdrawal(ctx, withdrawalID)
	if err != nil || withdrawal == nil {
		response.NotFound(c, "Withdrawal not found")
		return
	}

	var req MarkWithdrawalProcessedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req = MarkWithdrawalProcessedRequest{}
	}

	if err := h.withdrawService.MarkProcessed(ctx, actorID, withdrawalID); err != nil {
		if strings.Contains(err.Error(), "cannot process") || strings.Contains(err.Error(), "cannot manually complete") {
			h.log.Warn("Invalid status for manual completion",
				zap.String("withdrawal_id", withdrawalID.String()),
				zap.String("status", string(withdrawal.Status)),
			)
			response.BadRequest(c, "Cannot mark withdrawal as processed in current status: "+string(withdrawal.Status))
			return
		}
		h.log.Error("Failed to mark withdrawal as processed", zap.Error(err))
		response.InternalServerError(c, "Failed to mark withdrawal as processed")
		return
	}

	// Log admin action
	h.adminAuditLogger.LogSafe(ctx, actorID,
		"admin_withdrawal_marked_processed", "withdrawal", withdrawalID,
		map[string]interface{}{
			"amount":            withdrawal.Amount,
			"seller_id":         withdrawal.SellerID.String(),
			"completion_method": "manual",
		},
	)

	response.Success(c, gin.H{
		"message":       "Withdrawal marked as processed (manually completed)",
		"withdrawal_id": withdrawalID.String(),
		"status":        string(withdrawrepo.WithdrawalStatusCompleted),
	})
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// ListWhitelistAudit handles GET /admin/payouts/whitelist/audit
//
// Returns paginated, read-only history of every whitelist mutation:
// WHITELIST_INITIALIZED, SELLER_ADDED, SELLER_REMOVED.
// Optional query param: seller_id=<uuid> to filter by seller.
// Pagination: limit (default 50, max 200) and offset.
// Requires: finance.withdraw.read capability.
func (h *AdminPayoutHandler) ListWhitelistAudit(c *gin.Context) {
	if !middleware.HasCapability(c, "finance.withdraw.read") {
		response.Forbidden(c, "finance.withdraw.read capability required")
		return
	}
	if h.whitelistAuditRepo == nil {
		response.Error(c, 503, "service_unavailable", "whitelist audit repository not available in this environment")
		return
	}

	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}
	offset := 0
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	var (
		records []withdrawrepo.WhitelistAuditRecord
		err     error
	)

	sellerStr := strings.TrimSpace(c.Query("seller_id"))
	if sellerStr != "" {
		sellerID, parseErr := uuid.Parse(sellerStr)
		if parseErr != nil {
			response.BadRequest(c, "invalid seller_id: must be a valid UUID")
			return
		}
		records, err = h.whitelistAuditRepo.ListBySeller(c.Request.Context(), sellerID, limit, offset)
	} else {
		records, err = h.whitelistAuditRepo.List(c.Request.Context(), limit, offset)
	}

	if err != nil {
		h.log.Error("Failed to query whitelist audit log", zap.Error(err))
		response.InternalServerError(c, "failed to retrieve whitelist audit log")
		return
	}

	rows := make([]WhitelistAuditRow, 0, len(records))
	for _, rec := range records {
		rows = append(rows, WhitelistAuditRow{
			ID:        rec.ID,
			SellerID:  rec.SellerID,
			Action:    rec.Action,
			ActorID:   rec.ActorID,
			Reason:    rec.Reason,
			Source:    rec.Source,
			CreatedAt: rec.CreatedAt.Format(time.RFC3339),
		})
	}

	c.JSON(200, gin.H{
		"audit_log": rows,
		"limit":     limit,
		"offset":    offset,
		"count":     len(rows),
	})
}

// withdrawalToDetail converts a repository Withdrawal to a handler WithdrawalDetail.
func (h *AdminPayoutHandler) withdrawalToDetail(w *withdrawrepo.Withdrawal) WithdrawalDetail {
	detail := WithdrawalDetail{
		ID:                    w.ID,
		SellerID:              w.SellerID,
		Amount:                w.Amount,
		FeeAmount:             w.FeeAmount,
		NetPayoutAmount:       w.Amount - w.FeeAmount,
		TotalDebitAmount:      w.Amount,
		Status:                string(w.Status),
		BankNameSnapshot:      w.BankNameSnapshot,
		BankCodeSnapshot:      w.BankCodeSnapshot,
		AccountNumberSnapshot: w.AccountNumberSnapshot,
		AccountHolderSnapshot: w.AccountHolderSnapshot,
		ExternalReferenceID:   w.ExternalReferenceID,
		GatewayReferenceID:    w.ExternalReferenceID, // ExternalReferenceID is the gateway ref
		RetryCount:            w.RetryCount,
		FailureReason:         w.FailureReason,
		CreatedAt:             time.Unix(w.CreatedAt, 0).Format(time.RFC3339),
		UpdatedAt:             time.Unix(w.UpdatedAt, 0).Format(time.RFC3339),
	}

	if w.SellerUsername != "" {
		sellerUsername := w.SellerUsername
		detail.SellerUsername = &sellerUsername
	}

	if w.SellerFarmName != "" {
		sellerFarmName := w.SellerFarmName
		detail.SellerFarmName = &sellerFarmName
	}

	if w.SubmittedAt > 0 {
		submittedAt := time.Unix(w.SubmittedAt, 0).Format(time.RFC3339)
		detail.SubmittedAt = &submittedAt
	}

	if w.SettledAt > 0 {
		settledAt := time.Unix(w.SettledAt, 0).Format(time.RFC3339)
		detail.SettledAt = &settledAt
	}

	return detail
}
