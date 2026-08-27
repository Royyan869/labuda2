// Package http: buyer refund escalation endpoint (H2-B).
//
// When a seller rejects a refund, the buyer may escalate to admin/dispute.
// This handler orchestrates two atomic operations in a single transaction:
//   1. RefundService.EscalateToDispute  — transitions refund to escalated_to_admin
//   2. DisputeService.OpenDisputeFromEscalation — creates a linked dispute
//
// Route:
//
//	POST /api/v1/refunds/:id/escalate  — buyer escalates rejected refund
package http

import (
	"context"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	disputeEntity "github.com/labuda/backend/internal/governance/dispute/entity"
	refundapp "github.com/labuda/backend/internal/finance/refund/application"
	"github.com/labuda/backend/internal/finance/refund/entity"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
)

// DisputeServiceForEscalation is the interface the handler needs from DisputeService.
// Implemented by *disputeApp.DisputeService.
type DisputeServiceForEscalation interface {
	OpenDisputeFromEscalation(
		ctx context.Context,
		tx db.Tx,
		orderID uuid.UUID,
		callerID uuid.UUID,
		reason string,
		description *string,
		reasonCode string,
		evidenceURLs []string,
	) (*disputeEntity.Dispute, error)
}

// BuyerEscalationHandler handles buyer refund escalation HTTP requests.
type BuyerEscalationHandler struct {
	refundService  *refundapp.RefundService
	disputeService DisputeServiceForEscalation
	database       *db.DB
	log            *zap.Logger
}

// NewBuyerEscalationHandler creates a new BuyerEscalationHandler.
func NewBuyerEscalationHandler(
	refundService *refundapp.RefundService,
	disputeService DisputeServiceForEscalation,
	database *db.DB,
	log *zap.Logger,
) *BuyerEscalationHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &BuyerEscalationHandler{
		refundService:  refundService,
		disputeService: disputeService,
		database:       database,
		log:            log,
	}
}

// EscalateRefund handles POST /api/v1/refunds/:id/escalate.
//
// Buyer escalates a seller-rejected refund to a dispute. Both the refund
// state transition and the dispute creation happen in a single transaction.
func (h *BuyerEscalationHandler) EscalateRefund(c *gin.Context) {
	ctx := c.Request.Context()

	refundID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid refund ID")
		return
	}

	buyerID, ok := extractUserID(c)
	if !ok {
		return
	}

	var result map[string]interface{}
	txErr := h.database.WithTx(ctx, func(tx db.Tx) error {
		// Step 1: Escalate refund (ownership + state transition + event)
		refund, err := h.refundService.EscalateToDispute(ctx, tx, refundID, buyerID)
		if err != nil {
			return err
		}

		// Step 2: Create linked dispute (order lock + escrow check + persist + event)
		reasonCode := mapRefundReasonToDisputeCode(refund.Reason)
		dispute, err := h.disputeService.OpenDisputeFromEscalation(
			ctx, tx,
			refund.OrderID,
			buyerID,
			string(refund.Reason),
			refund.Description,
			reasonCode,
			refund.EvidenceURLs,
		)
		if err != nil {
			return fmt.Errorf("failed to create dispute: %w", err)
		}

		result = map[string]interface{}{
			"refund_id":  refund.ID,
			"status":     string(refund.Status),
			"dispute_id": dispute.ID,
		}
		return nil
	})

	if txErr != nil {
		h.handleError(c, txErr, refundID, buyerID)
		return
	}

	response.Success(c, result)
}

// handleError translates service errors to HTTP responses.
func (h *BuyerEscalationHandler) handleError(c *gin.Context, err error, refundID, buyerID uuid.UUID) {
	h.log.Error("buyer_escalation_failed",
		zap.String("refund_id", refundID.String()),
		zap.String("buyer_id", buyerID.String()),
		zap.Error(err),
	)

	errMsg := err.Error()
	switch {
	case strings.Contains(errMsg, "only the buyer can escalate"):
		response.Forbidden(c, "Only the buyer can escalate this refund")
	case strings.Contains(errMsg, "invalid refund status transition"):
		response.Error(c, 409, "INVALID_STATE", "Refund can only be escalated when seller has rejected it")
	case strings.Contains(errMsg, "refund already resolved"):
		response.Error(c, 409, "ALREADY_RESOLVED", "Refund has already been resolved")
	case strings.Contains(errMsg, "already has an active dispute"):
		response.Error(c, 409, "ALREADY_ESCALATED", "This order already has an active dispute")
	case strings.Contains(errMsg, "refund not found"):
		response.NotFound(c, "Refund not found")
	case strings.Contains(errMsg, "escrow not in holding"):
		response.Error(c, 409, "ESCROW_NOT_HOLDING", "Cannot escalate: escrow is no longer in holding state")
	default:
		response.InternalServerError(c, "Failed to escalate refund")
	}
}

// mapRefundReasonToDisputeCode maps a refund reason to the closest dispute reason code.
// Refund escalations always come from the buyer, so we use BuyerReasonCodes.
func mapRefundReasonToDisputeCode(reason entity.RefundReason) string {
	switch reason {
	case entity.RefundReasonItemNotReceived:
		return disputeEntity.ReasonCodeItemNotReceived
	case entity.RefundReasonItemNotAsDescribed:
		return disputeEntity.ReasonCodeItemNotAsDescribed
	case entity.RefundReasonItemDamaged:
		return disputeEntity.ReasonCodeShippingDamage
	default:
		return disputeEntity.ReasonCodeOther
	}
}


