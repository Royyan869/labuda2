// Package http: admin-only gateway refund trigger (TASK 34 / Phase 2a).
//
// This handler EXISTS so we can prove the gateway refund pipeline end-to-end
// in a sandbox without exposing the flow to buyers or sellers. It is gated
// by:
//
//	1. Admin role (RequireAdminMiddleware) at the route level.
//	2. Capability "finance.refund.gateway.initiate" at the route level.
//	3. Server-side feature flag ENABLE_GATEWAY_REFUND_PHASE2 inside the
//	   handler — toggling the flag flips the endpoint between 503
//	   FEATURE_DISABLED and live behavior without redeploying.
//
// PHASE 2a INVARIANT: this handler dispatches to the gateway and persists
// the refund row state, but performs ZERO financial mutation. All escrows
// are gateway-funded; the legacy wallet-hold kill-switch has been demolished.
package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/labuda/backend/internal/audit"
	refundapp "github.com/labuda/backend/internal/finance/refund/application"
	"github.com/labuda/backend/internal/finance/refund/entity"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
)

// gatewayRefundInitiator is the minimal RefundService surface this handler
// depends on. Defined as an interface — mirroring GatewayRefundClient /
// FinanceReverser in the application layer — so tests can inject a fake
// without a live database or gateway.
type gatewayRefundInitiator interface {
	InitiateGatewayRefund(ctx context.Context, tx db.Tx, input refundapp.InitiateGatewayRefundInput) (*entity.Refund, error)
}

// AdminRefundHandler exposes the gateway refund trigger to administrators.
//
// flagEnabled is captured by-value at construction so toggling the env var
// requires a process restart — that is intentional for Phase 2a since each
// activation should be deliberate.
type AdminRefundHandler struct {
	refundService    gatewayRefundInitiator
	database         db.Transactor
	flagEnabled      bool
	adminAuditLogger audit.AdminAuditLogger
	log              *zap.Logger
}

// NewAdminRefundHandler builds the handler. refundService MUST be the
// singleton constructed in dependencies_core.go and MUST already have
// SetGatewayClient called on it; otherwise InitiateGatewayRefund will
// return ErrGatewayClientNotConfigured at request time.
func NewAdminRefundHandler(
	refundService *refundapp.RefundService,
	database *db.DB,
	flagEnabled bool,
	log *zap.Logger,
	adminAuditLogger audit.AdminAuditLogger,
) *AdminRefundHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &AdminRefundHandler{
		refundService:    refundService,
		database:         database,
		flagEnabled:      flagEnabled,
		adminAuditLogger: adminAuditLogger,
		log:              log,
	}
}

// initiateGatewayRefundRequest is the JSON body expected on POST.
//
// IdempotencyKey MUST be supplied by the caller. The orchestration layer
// stores it in refunds.gateway_idempotency_key (UNIQUE), so a duplicate
// POST with the same key against the same refund returns the existing row
// without re-dispatching to Midtrans.
type initiateGatewayRefundRequest struct {
	Amount         int64  `json:"amount" binding:"required,gt=0"`
	Reason         string `json:"reason" binding:"required"`
	IdempotencyKey string `json:"idempotency_key" binding:"required"`
}

// initiateGatewayRefundResponse is the trimmed view returned to the admin.
// We deliberately do NOT echo the legacy buyer/seller-negotiation columns —
// this surface is about the gateway pipeline, nothing else.
type initiateGatewayRefundResponse struct {
	RefundID              uuid.UUID `json:"refund_id"`
	OrderID               uuid.UUID `json:"order_id"`
	GatewayStatus         string    `json:"gateway_status"`
	GatewayAttempts       int       `json:"gateway_attempts"`
	GatewayRefundID       *string   `json:"gateway_refund_id,omitempty"`
	GatewayIdempotencyKey *string   `json:"gateway_idempotency_key,omitempty"`
	LastGatewayError      *string   `json:"last_gateway_error,omitempty"`
}

// InitiateGatewayRefund handles POST /api/v1/admin/refunds/:refund_id/gateway/initiate.
//
// Flow:
//  1. Reject with 503 if the feature flag is off.
//  2. Parse path parameter (refund_id) and JSON body.
//  3. Open a transaction and call refundService.InitiateGatewayRefund.
//  4. Translate domain errors to HTTP responses.
//
// Translation rules:
//   - ErrGatewayClientNotConfigured              → 503 FEATURE_DISABLED (wiring bug)
//   - ErrRefundAlreadySettledByGateway           → 409 CONFLICT
//   - validation errors (amount, escrow, etc.)  → 400 BAD_REQUEST
//   - everything else                            → 500 INTERNAL_SERVER_ERROR
//
// On success the refund row is returned with its gateway_status reflecting
// either 'pending' (gateway accepted) or 'failed' (gateway rejected). A
// 'failed' outcome is still a 200 — the orchestration ran correctly, the
// gateway just declined; the admin needs that detail to decide whether to
// retry, not a server-error code.
func (h *AdminRefundHandler) InitiateGatewayRefund(c *gin.Context) {
	if !h.flagEnabled {
		response.FeatureDisabled(c, "gateway_refund_initiate")
		return
	}

	refundIDStr := c.Param("refund_id")
	refundID, err := uuid.Parse(refundIDStr)
	if err != nil {
		response.BadRequest(c, "invalid refund_id: must be uuid")
		return
	}

	var req initiateGatewayRefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid body: "+err.Error())
		return
	}
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()
	var refundOut struct {
		ID                    uuid.UUID
		OrderID               uuid.UUID
		GatewayStatus         string
		GatewayAttempts       int
		GatewayRefundID       *string
		GatewayIdempotencyKey *string
		LastGatewayError      *string
	}
	dispatchErr := h.database.WithTx(ctx, func(tx db.Tx) error {
		refund, err := h.refundService.InitiateGatewayRefund(ctx, tx, refundapp.InitiateGatewayRefundInput{
			RefundID:       refundID,
			Amount:         req.Amount,
			Reason:         req.Reason,
			IdempotencyKey: req.IdempotencyKey,
			CallerID:       adminID,
			CallerType:     refundapp.GatewayRefundCallerTypeAdmin,
		})
		// We capture refund state for both success AND failure-of-gateway
		// (which is still a successful orchestration) so the admin can see
		// the row's transition outcome.
		if refund != nil {
			refundOut.ID = refund.ID
			refundOut.OrderID = refund.OrderID
			refundOut.GatewayStatus = string(refund.GatewayStatus)
			refundOut.GatewayAttempts = refund.GatewayAttempts
			refundOut.GatewayRefundID = refund.GatewayRefundID
			refundOut.GatewayIdempotencyKey = refund.GatewayIdempotencyKey
			refundOut.LastGatewayError = refund.LastGatewayError
		}
		return err
	})

	if dispatchErr != nil {
		// Distinguish orchestration validation errors (admin's fault) from
		// runtime/wiring errors (operator's fault) so the admin gets a
		// useful response.
		var callerErr *refundapp.ErrGatewayRefundCallerProvenanceRequired
		switch {
		case errors.Is(dispatchErr, refundapp.ErrGatewayClientNotConfigured):
			h.log.Error("gateway_refund_handler_unwired",
				zap.String("refund_id", refundID.String()),
				zap.Error(dispatchErr),
			)
			response.FeatureDisabled(c, "gateway_refund_initiate (gateway client not wired)")
			return
		case errors.As(dispatchErr, &callerErr):
			response.Forbidden(c, dispatchErr.Error())
			return
		case errors.Is(dispatchErr, refundapp.ErrRefundAlreadySettledByGateway):
			h.log.Warn("gateway_refund_already_settled",
				zap.String("refund_id", refundID.String()),
			)
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "REFUND_ALREADY_SETTLED",
					"message": dispatchErr.Error(),
				},
			})
			return
		default:
			// Surface the synchronous Midtrans HTTP error too — when the
			// gateway returns 4xx, refundOut is populated AND dispatchErr
			// is non-nil. Treat that as a 200 (the orchestration ran).
			if refundOut.ID != uuid.Nil &&
				refundOut.GatewayStatus == "failed" {
				h.log.Warn("gateway_refund_dispatch_failed",
					zap.String("refund_id", refundOut.ID.String()),
					zap.Error(dispatchErr),
				)
				h.logGatewayRefundInitiated(ctx, adminID, refundOut, req)
				response.Success(c, toAdminResponse(refundOut))
				return
			}
			h.log.Error("gateway_refund_validation_or_internal_error",
				zap.String("refund_id", refundID.String()),
				zap.Error(dispatchErr),
			)
			response.BadRequest(c, dispatchErr.Error())
			return
		}
	}

	h.logGatewayRefundInitiated(ctx, adminID, refundOut, req)
	response.Success(c, toAdminResponse(refundOut))
}

// logGatewayRefundInitiated writes an admin audit log entry for a completed
// gateway refund orchestration — this fires whether the gateway accepted
// the refund (gateway_status=pending) or declined it (gateway_status=failed),
// since both are a completed admin-triggered dispatch, not a wiring error.
//
// Best-effort (LogSafe): an audit-logging failure must not roll back or
// otherwise corrupt the refund row that was already committed by
// InitiateGatewayRefund — this mirrors the LogSafe pattern used by
// AdminPayoutHandler for the same class of money-moving admin action.
//
// Metadata is limited to business identifiers (order id, amount, reason,
// caller-supplied idempotency key, gateway status/attempts, gateway's own
// refund id) — never gateway credentials, tokens, or raw webhook payloads.
func (h *AdminRefundHandler) logGatewayRefundInitiated(
	ctx context.Context,
	adminID uuid.UUID,
	refundOut struct {
		ID                    uuid.UUID
		OrderID               uuid.UUID
		GatewayStatus         string
		GatewayAttempts       int
		GatewayRefundID       *string
		GatewayIdempotencyKey *string
		LastGatewayError      *string
	},
	req initiateGatewayRefundRequest,
) {
	if h.adminAuditLogger == nil {
		return
	}
	h.adminAuditLogger.LogSafe(ctx, adminID,
		audit.ActionRefundGatewayInitiated, audit.TargetTypeRefund, refundOut.ID,
		map[string]interface{}{
			"order_id":          refundOut.OrderID.String(),
			"amount":            req.Amount,
			"reason":            req.Reason,
			"idempotency_key":   req.IdempotencyKey,
			"gateway_status":    refundOut.GatewayStatus,
			"gateway_attempts":  refundOut.GatewayAttempts,
			"gateway_refund_id": refundOut.GatewayRefundID,
		},
	)
}

func toAdminResponse(r struct {
	ID                    uuid.UUID
	OrderID               uuid.UUID
	GatewayStatus         string
	GatewayAttempts       int
	GatewayRefundID       *string
	GatewayIdempotencyKey *string
	LastGatewayError      *string
}) *initiateGatewayRefundResponse {
	return &initiateGatewayRefundResponse{
		RefundID:              r.ID,
		OrderID:               r.OrderID,
		GatewayStatus:         r.GatewayStatus,
		GatewayAttempts:       r.GatewayAttempts,
		GatewayRefundID:       r.GatewayRefundID,
		GatewayIdempotencyKey: r.GatewayIdempotencyKey,
		LastGatewayError:      r.LastGatewayError,
	}
}


