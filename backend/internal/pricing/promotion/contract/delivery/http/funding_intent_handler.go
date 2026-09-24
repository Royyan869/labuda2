// Package http exposes the canonical Promotion Contract lifecycle over HTTP.
//
// This file implements the exact-shortage payment intent endpoint and the
// payment initiation endpoint that bridges FundingIntent to the canonical
// billing→Midtrans payment engine.
package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/response"
	contractApp "github.com/labuda/backend/internal/pricing/promotion/contract/application"
	contractEntity "github.com/labuda/backend/internal/pricing/promotion/contract/entity"
	"go.uber.org/zap"
)

// InitiateBillingPaymentFunc is the function signature for the canonical
// billing→payment engine. Implemented by CorePaymentHandler.InitiateBillingPayment.
// Uses simple return types to avoid cross-package type dependencies.
type InitiateBillingPaymentFunc func(
	ctx context.Context,
	userID uuid.UUID,
	billingID uuid.UUID,
	paymentMethodCode string,
) (paymentID uuid.UUID, paymentURL string, grossAmount int64, err error)

// FundingIntentHandler owns the canonical funding intent HTTP surface.
type FundingIntentHandler struct {
	service          *contractApp.FundingIntentService
	initiatePayment  InitiateBillingPaymentFunc
	log              *zap.Logger
}

// NewFundingIntentHandler wires the canonical funding intent handler.
func NewFundingIntentHandler(service *contractApp.FundingIntentService, log *zap.Logger) *FundingIntentHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &FundingIntentHandler{service: service, log: log}
}

// SetPaymentInitiator wires the canonical billing→payment engine.
// Must be called before any payment initiation endpoint is reachable.
func (h *FundingIntentHandler) SetPaymentInitiator(fn InitiateBillingPaymentFunc) {
	h.initiatePayment = fn
}

// CreateFundingIntentRequest captures the promotion creation params for
// computing the exact shortage payment. The same inputs as CreateContractRequest.
type CreateFundingIntentRequest struct {
	Kind         string   `json:"kind" binding:"required,oneof=internal external"`
	BudgetRupiah int64    `json:"budget_rupiah" binding:"required,min=1"`
	DurationDays int64    `json:"duration_days" binding:"required,min=1"`
	CityIDs      []string `json:"city_ids"`
}

// CreateFundingIntent handles POST /api/v1/promotions/contracts/payment-intent.
//
// It returns a read-only FundingIntentResult with the exact shortage the seller
// would face. When shortage > 0, a billing transaction is created for exactly
// that amount. The promotion parameters are a snapshot used for shortage
// calculation — they do NOT bind the payment to a specific future promotion.
//
// AUTHORITY: single canonical funding intent endpoint. There is no second
// calculation or payment path.
func (h *FundingIntentHandler) CreateFundingIntent(c *gin.Context) {
	callerID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		response.Unauthorized(c, "Authentication required")
		return
	}

	var req CreateFundingIntentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	result, err := h.service.CreateFundingIntent(c.Request.Context(), contractApp.CreatePromotionInput{
		SellerID:     callerID,
		Kind:         contractEntity.Kind(req.Kind),
		BudgetRupiah: req.BudgetRupiah,
		DurationDays: req.DurationDays,
		CityIDs:      req.CityIDs,
	})
	if err != nil {
		h.writeError(c, "create funding intent", err)
		return
	}

	response.Success(c, gin.H{
		"intent": result,
	})
}

// InitiatePaymentRequest is the request body for payment initiation.
type InitiatePaymentRequest struct {
	PaymentMethodCode string `json:"payment_method_code" binding:"required"`
}

// InitiatePaymentResponse is the response from payment initiation.
type InitiatePaymentResponse struct {
	PaymentID   string `json:"payment_id"`
	PaymentURL  string `json:"payment_url"`
	GrossAmount int64  `json:"gross_amount"`
	Shortage    int64  `json:"shortage"`
	IntentID    string `json:"intent_id"`
}

// InitiatePayment handles POST /api/v1/promotions/contracts/payment-intent/:id/pay.
//
// It loads the FundingIntent, verifies the linked billing transaction's amount
// matches the immutable shortage, then delegates to the canonical
// CorePaymentHandler.InitiateBillingPayment engine.
//
// FLOW:
//  1. Load FundingIntent by ID
//  2. Verify caller owns the intent (seller_id == caller)
//  3. Verify billing transaction exists and is linked
//  4. Verify billing.gross_amount == intent.shortage_amount (immutable)
//  5. Delegate to CorePaymentHandler.InitiateBillingPayment (canonical engine)
//  6. Return payment_url
//
// AUTHORITY: this is a thin orchestration layer. The canonical payment engine
// (CorePaymentHandler.InitiateBillingPayment) is the sole authority for payment
// creation, Midtrans Snap, and payment record persistence.
func (h *FundingIntentHandler) InitiatePayment(c *gin.Context) {
	callerID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		response.Unauthorized(c, "Authentication required")
		return
	}

	intentIDStr := c.Param("id")
	intentID, err := uuid.Parse(intentIDStr)
	if err != nil {
		response.BadRequest(c, "Invalid intent ID")
		return
	}

	var req InitiatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if h.initiatePayment == nil {
		response.InternalServerError(c, "Payment engine not configured")
		return
	}

	// Step 1-4: Load intent and verify billing amount matches
	verification, err := h.service.InitiatePayment(c.Request.Context(), contractApp.InitiatePaymentInput{
		IntentID:          intentID,
		CallerID:          callerID,
		PaymentMethodCode: req.PaymentMethodCode,
	})
	if err != nil {
		h.writePaymentError(c, "initiate payment", err)
		return
	}

	// Step 5: Delegate to canonical billing→payment engine
	paymentID, paymentURL, grossAmount, err := h.initiatePayment(
		c.Request.Context(),
		callerID,
		verification.BillingID,
		req.PaymentMethodCode,
	)
	if err != nil {
		h.writePaymentError(c, "initiate billing payment", err)
		return
	}

	// Step 6: Return payment URL
	response.Success(c, InitiatePaymentResponse{
		PaymentID:   paymentID.String(),
		PaymentURL:  paymentURL,
		GrossAmount: grossAmount,
		Shortage:    verification.Shortage,
		IntentID:    intentIDStr,
	})
}

// GetFundingPaymentMethods handles
// GET /api/v1/promotions/contracts/payment-intent/:id/payment-methods.
//
// READ-ONLY disclosure: returns the enabled payment methods with the canonical
// fee and gross total for this intent's exact shortage obligation. The client
// picks a method_code from this response and sends it to
// POST /payment-intent/:id/pay. The client never computes a fee.
//
// AUTHORITY: no payment is created, no Midtrans call is made, no ledger row is
// written, no FundingIntent is created or mutated. Both the method list and the
// fee come from the same canonical payment-method authority initiation uses.
//
// Rejections: unknown intent → 404, another seller's intent → 403, unlinked or
// non-pending billing → 409 (see writePaymentError).
func (h *FundingIntentHandler) GetFundingPaymentMethods(c *gin.Context) {
	callerID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		response.Unauthorized(c, "Authentication required")
		return
	}

	intentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid intent ID")
		return
	}

	result, err := h.service.PaymentMethods(c.Request.Context(), contractApp.PaymentMethodsInput{
		IntentID: intentID,
		CallerID: callerID,
	})
	if err != nil {
		h.writePaymentError(c, "list funding payment methods", err)
		return
	}

	response.Success(c, result)
}

// writePaymentError maps domain errors to HTTP error responses for payment initiation.
func (h *FundingIntentHandler) writePaymentError(c *gin.Context, op string, err error) {
	switch {
	case errors.Is(err, contractApp.ErrFundingIntentNotFound):
		response.Error(c, http.StatusNotFound, "FUNDING_INTENT_NOT_FOUND", "Funding intent not found")
	case errors.Is(err, contractApp.ErrFundingIntentNotOwnedByCaller):
		response.Forbidden(c, "You can only pay for your own funding intent")
	case errors.Is(err, contractApp.ErrFundingIntentNoBilling):
		response.Error(c, http.StatusConflict, "FUNDING_INTENT_NO_BILLING", "Funding intent has no linked billing transaction")
	case errors.Is(err, contractApp.ErrFundingIntentBillingAmountMismatch):
		response.Error(c, http.StatusConflict, "BILLING_AMOUNT_MISMATCH", "Billing amount does not match intent shortage")
	case errors.Is(err, contractApp.ErrFundingIntentBillingNotPending):
		response.Error(c, http.StatusConflict, "BILLING_NOT_PENDING", "Billing transaction is not pending")
	default:
		h.log.Error(op+" failed", zap.Error(err))
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Sprintf("Failed to %s", op))
	}
}

// writeError maps domain errors to HTTP error responses.
func (h *FundingIntentHandler) writeError(c *gin.Context, op string, err error) {
	var budgetErr *contractApp.ErrPromotionBudgetBelowMinimum
	if errors.As(err, &budgetErr) {
		response.Error(c, http.StatusBadRequest, "PROMOTION_BUDGET_BELOW_MINIMUM",
			"Promotion budget is below the minimum required for the chosen duration")
		return
	}

	switch {
	case errors.Is(err, contractApp.ErrPromotionKindInvalid):
		response.Error(c, http.StatusBadRequest, "INVALID_PROMOTION_KIND", "Promotion kind must be internal or external")
	case errors.Is(err, contractApp.ErrPromotionBudgetInvalid):
		response.Error(c, http.StatusBadRequest, "INVALID_PROMOTION_BUDGET", "Promotion budget must be a positive Rupiah integer")
	case errors.Is(err, contractApp.ErrPromotionDurationInvalid):
		response.Error(c, http.StatusBadRequest, "INVALID_PROMOTION_DURATION", "Promotion duration is invalid")
	default:
		h.log.Error(op+" failed", zap.Error(err))
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Sprintf("Failed to %s", op))
	}
}
