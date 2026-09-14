package http

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	financeApp "github.com/labuda/backend/internal/finance/application"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// WithdrawalHandlerUnified handles HTTP requests for withdrawal operations.
//
// The HTTP entry (POST /api/v1/withdraw) drives the CANONICAL finance-shape
// withdrawal lifecycle via WithdrawService.RequestWithdrawal. The resulting
// row is compatible with AdminPayoutHandler, PayoutWorker, and
// PayoutWebhookHandler without any status translation.
//
// ListWithdrawals reads from the canonical finance withdrawal repository via
// WithdrawService.ListWithdrawalsBySeller.
type WithdrawalHandlerUnified struct {
	withdrawService *financeApp.WithdrawService
	db              *db.DB
	logger          *zap.Logger
}

// NewWithdrawalHandlerUnified creates a new unified withdrawal handler.
//
// withdrawService is required and drives the canonical request path.
func NewWithdrawalHandlerUnified(
	withdrawService *financeApp.WithdrawService,
	database *db.DB,
	logger *zap.Logger,
) *WithdrawalHandlerUnified {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &WithdrawalHandlerUnified{
		withdrawService: withdrawService,
		db:              database,
		logger:          logger,
	}
}

// RequestWithdrawRequest holds the request body for requesting a withdrawal.
type RequestWithdrawRequest struct {
	Amount int64 `json:"amount" binding:"required,min=1"`
}

// RequestWithdraw handles POST /api/v1/withdraw (canonical finance-shape flow).
//
// CANONICAL FLOW:
//  1. Get seller_id from auth context.
//  2. Delegate to WithdrawService.RequestWithdrawal — single tx covers:
//     - Verified-seller gate
//     - Dispute-aware withdrawable gate (SELLER_PAYABLE − dispute_freeze)
//     - Single-in-flight withdrawal guard
//     - Default bank account snapshot (FOR UPDATE)
//     - withdrawals row insert with status=REQUESTED and idempotency_key set
//     - Ledger DR SELLER_PAYABLE / CR WITHDRAWAL_PENDING
//     - withdrawal.requested outbox event
//  3. Returned row is directly consumable by AdminPayoutHandler / PayoutWorker
//     / PayoutWebhookHandler without status translation.
func (h *WithdrawalHandlerUnified) RequestWithdraw(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context (set by auth middleware)
	userIDVal, exists := c.Get("userID")
	if !exists {
		h.logger.Warn("withdrawal_unified_missing_user_id")
		response.Unauthorized(c, "User not authenticated")
		return
	}
	sellerID, ok := userIDVal.(uuid.UUID)
	if !ok {
		h.logger.Error("withdrawal_unified_invalid_user_id_type")
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Parse request body
	var req RequestWithdrawRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("withdrawal_unified_invalid_request",
			zap.String("user_id", sellerID.String()),
			zap.Error(err),
		)
		response.BadRequest(c, "Invalid request body")
		return
	}

	h.logger.Info("withdrawal_unified_request_received",
		zap.String("user_id", sellerID.String()),
		zap.Int64("amount", req.Amount),
	)

	if h.withdrawService == nil {
		h.logger.Error("withdrawal_unified_service_not_configured",
			zap.String("severity", "critical"),
		)
		response.InternalServerError(c, "Withdrawal service not configured")
		return
	}

	withdrawal, err := h.withdrawService.RequestWithdrawal(ctx, financeApp.CanonicalRequestWithdrawalInput{
		SellerID: sellerID,
		Amount:   req.Amount,
	})

	if err != nil {
		switch e := err.(type) {
		case *financeApp.ErrSellerNotVerified:
			h.logger.Warn("withdrawal_unified_seller_not_verified",
				zap.String("seller_id", sellerID.String()),
			)
			response.Forbidden(c, "Seller must be verified to request a withdrawal")
			return

		case *financeApp.ErrWithdrawalPendingExists:
			h.logger.Warn("withdrawal_unified_existing_pending",
				zap.String("seller_id", sellerID.String()),
				zap.String("existing_withdrawal_id", e.ExistingWithdrawalID.String()),
				zap.String("existing_status", string(e.ExistingStatus)),
			)
			response.Conflict(c, fmt.Sprintf(
				"In-flight withdrawal already exists: %s (status %s). Please wait for it to settle.",
				e.ExistingWithdrawalID.String(), e.ExistingStatus,
			))
			return

		case *financeApp.ErrNoDefaultBankAccount:
			h.logger.Warn("withdrawal_unified_no_default_bank",
				zap.String("seller_id", sellerID.String()),
			)
			response.BadRequest(c, "No default bank account configured. Please add a bank account before requesting a withdrawal.")
			return

		case *financeApp.ErrBankAccountNotReviewed:
			h.logger.Warn("withdrawal_bank_account_not_reviewed",
				zap.String("seller_id", sellerID.String()),
				zap.String("bank_account_id", e.BankAccountID.String()),
			)
			response.Error(c, 422, "BANK_ACCOUNT_NOT_REVIEWED",
				"Rekening bank Anda belum ditinjau oleh admin untuk pencairan dana. "+
					"Silakan hubungi admin atau gunakan rekening yang sudah terdaftar sebelum perubahan terakhir.")
			return

		case *financeApp.ErrWithdrawalAmountOutOfRange:
			h.logger.Warn("withdrawal_unified_amount_out_of_range",
				zap.String("seller_id", sellerID.String()),
				zap.Int64("amount", e.Amount),
				zap.Int64("min", e.Min),
				zap.Int64("max", e.Max),
			)
			response.BadRequest(c, fmt.Sprintf(
				"Amount %d out of range. Allowed: %d..%d.",
				e.Amount, e.Min, e.Max,
			))
			return

		case *financeApp.ErrWithdrawalBlockedByWithdrawableBalance:
			h.logger.Warn("withdrawal_blocked_by_withdrawable_balance",
				zap.String("seller_id", sellerID.String()),
				zap.Int64("requested_amount", e.RequestedAmount),
				zap.Int64("payable_balance", e.PayableBalance),
				zap.Int64("active_dispute_freeze", e.ActiveDisputeFreeze),
				zap.Int64("withdrawable", e.Withdrawable),
			)
			response.BadRequest(c, fmt.Sprintf(
				"Withdrawal blocked by dispute freeze or balance ceiling. Requested: %d, Withdrawable: %d (Payable: %d − Freeze: %d)",
				e.RequestedAmount, e.Withdrawable, e.PayableBalance, e.ActiveDisputeFreeze,
			))
			return

		default:
			h.logger.Error("withdrawal_unified_failed",
				zap.String("seller_id", sellerID.String()),
				zap.Error(err),
			)
			response.InternalServerError(c, "Failed to request withdrawal")
			return
		}
	}

	h.logger.Info("withdrawal_unified_success",
		zap.String("withdrawal_id", withdrawal.WithdrawalID.String()),
		zap.String("seller_id", sellerID.String()),
		zap.Int64("amount", withdrawal.Amount),
		zap.String("status", string(withdrawal.Status)),
	)

	response.Created(c, gin.H{
		"withdrawal_id":      withdrawal.WithdrawalID.String(),
		"status":             string(withdrawal.Status),
		"amount":             withdrawal.Amount,
		"fee_amount":         withdrawal.FeeAmount,
		"net_payout_amount":  withdrawal.Amount - withdrawal.FeeAmount,
		"total_debit_amount": withdrawal.TotalDebitAmount,
	})
}

// ListWithdrawals retrieves withdrawal history for the authenticated seller from
// the canonical finance withdrawal repository.
//
// GET /api/v1/withdraw/history
//
// JSON shape is backward-compatible with the prior wallet-projection response:
//   - withdrawal_id, amount: unchanged
//   - status: canonical uppercase finance status (REQUESTED, SETTLED, etc.)
//   - reference_code: mapped from ExternalReferenceID (null when empty)
//   - requested_at: finance created_at formatted as RFC3339
//   - processed_at: finance settled_at as RFC3339 when >0, else null
//
// Additive fields included when non-empty:
//
//	bank_name_snapshot, bank_code_snapshot,
//	account_number_snapshot, account_holder_snapshot
func (h *WithdrawalHandlerUnified) ListWithdrawals(c *gin.Context) {
	ctx := c.Request.Context()

	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	sellerID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	limit := 20
	offset := 0
	if limitStr := c.Query("limit"); limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		fmt.Sscanf(offsetStr, "%d", &offset)
	}

	withdrawals, total, err := h.withdrawService.ListWithdrawalsBySeller(ctx, sellerID, limit, offset)
	if err != nil {
		h.logger.Error("withdrawal_list_finance_failed",
			zap.String("user_id", sellerID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve withdrawals")
		return
	}

	items := make([]gin.H, len(withdrawals))
	for i, w := range withdrawals {
		var referenceCode interface{}
		if w.ExternalReferenceID != "" {
			referenceCode = w.ExternalReferenceID
		}

		var processedAt interface{}
		if w.SettledAt > 0 {
			processedAt = time.Unix(w.SettledAt, 0).UTC().Format(time.RFC3339)
		}

		// MONEY MODEL (PASS_18H, owner-confirmed): amount is the requested/
		// reserved amount; net_payout_amount = amount - fee is what actually
		// reaches the seller's bank; total_debit_amount == amount (the fee is
		// deducted FROM it, never added on top).
		item := gin.H{
			"withdrawal_id":      w.ID.String(),
			"amount":             w.Amount,
			"fee_amount":         w.FeeAmount,
			"net_payout_amount":  w.Amount - w.FeeAmount,
			"total_debit_amount": w.Amount,
			"status":             string(w.Status),
			"reference_code":     referenceCode,
			"requested_at":       time.Unix(w.CreatedAt, 0).UTC().Format(time.RFC3339),
			"processed_at":       processedAt,
		}
		if w.BankNameSnapshot != "" {
			item["bank_name_snapshot"] = w.BankNameSnapshot
		}
		if w.BankCodeSnapshot != "" {
			item["bank_code_snapshot"] = w.BankCodeSnapshot
		}
		if w.AccountNumberSnapshot != "" {
			item["account_number_snapshot"] = w.AccountNumberSnapshot
		}
		if w.AccountHolderSnapshot != "" {
			item["account_holder_snapshot"] = w.AccountHolderSnapshot
		}
		items[i] = item
	}

	response.Success(c, gin.H{
		"withdrawals": items,
		"total":       total,
		"limit":       limit,
		"offset":      offset,
	})
}
