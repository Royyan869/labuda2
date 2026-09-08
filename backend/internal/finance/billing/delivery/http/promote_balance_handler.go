package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	billingApp "github.com/labuda/backend/internal/finance/billing/application"
	billingentity "github.com/labuda/backend/internal/finance/billing/entity"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"go.uber.org/zap"
)

// PromoteBalanceFundingHandler owns the canonical Promote Balance funding entry.
//
// It creates ONLY a billing transaction of type promote_balance_top_up.
// Funding (BANK_SETTLEMENT -> PROMOTE_BALANCE) happens later via the
// existing payment settlement path (POST /payments/billing + webhook +
// billingService.MarkPaid + finance funding path). This handler
// MUST NOT directly touch ledger state.
type PromoteBalanceFundingHandler struct {
	billingService *billingApp.BillingService
	roleChecker    auth.RoleChecker
	db             db.Transactor
	log            *zap.Logger
}

// NewPromoteBalanceFundingHandler wires the canonical funding handler.
func NewPromoteBalanceFundingHandler(
	billingService *billingApp.BillingService,
	roleChecker auth.RoleChecker,
	db db.Transactor,
	log *zap.Logger,
) *PromoteBalanceFundingHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &PromoteBalanceFundingHandler{
		billingService: billingService,
		roleChecker:    roleChecker,
		db:             db,
		log:            log,
	}
}

// CreateTopUpRequest captures the seller-supplied funding amount.
//
// Amount is a positive Rupiah integer (canonical money unit). No cap is
// enforced here — NO_CANONICAL_TOPUP_CAP_FOUND (audit 4B). Zero/negative
// and overflow are fail-closed via validation below and the downstream
// settlement guards (finance funding path + billing top-up guard).
type CreateTopUpRequest struct {
	Amount int64 `json:"amount" binding:"required"`
}

// CreateTopUp handles POST /api/v1/promote-balance/topup.
//
// Production graph:
//   authenticated eligible seller
//     -> CreateTopUp (this handler)
//     -> CreateBillingTransaction(TypePromoteBalanceTopUp, payer=caller, fee=0)
//     -> billing row (pending)
//     -> existing POST /payments/billing
//     -> gateway webhook -> MarkPaid -> funding settlement -> PROMOTE_BALANCE
//
// Forbidden (enforced):
//   - does NOT create platform revenue
//   - does NOT create promotion allocation / promotion contract
//   - does NOT directly credit balance
func (h *PromoteBalanceFundingHandler) CreateTopUp(c *gin.Context) {
	callerID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		response.Unauthorized(c, "Authentication required")
		return
	}

	var req CreateTopUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Canonical amount validation: positive Rupiah integer.
	// Existing billing/settlement authorities also enforce positivity, but
	// fail-closed early here so the client gets a 400 before any DB write.
	if req.Amount <= 0 {
		response.Error(c, http.StatusBadRequest, "INVALID_AMOUNT", "Top-up amount must be a positive Rupiah integer")
		return
	}

	// Seller eligibility: canonical market authority
	// auth.RoleChecker.HasActiveSellerCapability (same authority used by
	// PromotionContractService via RoleCheckerSellerEligibilityGate).
	if h.roleChecker == nil {
		h.log.Error("promote balance funding: role checker not wired")
		response.InternalServerError(c, "Seller eligibility authority not configured")
		return
	}
	ok, err := h.roleChecker.HasActiveSellerCapability(c.Request.Context(), callerID)
	if err != nil {
		h.log.Error("promote balance funding: seller capability check failed", zap.Error(err), zap.String("seller_id", callerID.String()))
		response.InternalServerError(c, "Failed to verify seller authority")
		return
	}
	if !ok {
		response.Error(c, http.StatusForbidden, "SELLER_NOT_ELIGIBLE", "Active seller subscription required to top up promote balance")
		return
	}

	// Create billing row: TypePromoteBalanceTopUp, fee=0, payer=caller.
	// TargetID is a fresh UUID — it is not a package id and is not
	// interpreted by settlement (only the billing ID is the idempotency key).
	var billing *billingentity.BillingTransaction
	err = h.db.WithTx(c.Request.Context(), func(tx db.Tx) error {
		var txErr error
		billing, txErr = h.billingService.CreateBillingTransaction(
			c.Request.Context(),
			tx,
			callerID,                          // caller
			callerID,                          // payer (must equal caller)
			uuid.New(),                        // target_id: opaque funding reference
			billingentity.TypePromoteBalanceTopUp,
			money.New(req.Amount),
			0, // platform fee MUST be 0 — funding is not revenue
		)
		return txErr
	})
	if err != nil {
		h.log.Error("promote balance funding: create billing transaction failed", zap.Error(err), zap.String("seller_id", callerID.String()), zap.Int64("amount", req.Amount))
		// Surface account-status / ownership errors as 403/400 where appropriate.
		// BillingService uses auth.ErrOwnerRequired / account status errors.
		// The middleware auth layer already gates account status, but we
		// preserve the canonical error mapping here.
		if err == auth.ErrOwnerRequired {
			response.Forbidden(c, "You can only create billing for yourself")
			return
		}
		response.Error(c, http.StatusInternalServerError, "BILLING_CREATE_FAILED", "Failed to create top-up billing transaction")
		return
	}

	response.Created(c, gin.H{
		"message":    "Top-up billing transaction created. Proceed to payment via POST /api/v1/payments/billing.",
		"billing_id": billing.ID.String(),
		"amount":     billing.GrossAmount.Int64(),
		"status":     string(billing.Status),
		"type":       string(billing.Type),
	})
}
