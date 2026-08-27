// Package http: seller refund decision endpoints (H2-A).
//
// These endpoints allow the seller to approve or reject a buyer's refund
// request. The seller's identity comes from the auth context — never from
// the request body.
//
// Routes:
//
//	POST /api/v1/refunds/:id/approve  — seller approves, dispatches gateway refund
//	POST /api/v1/refunds/:id/reject   — seller rejects, buyer may escalate
package http

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	refundapp "github.com/labuda/backend/internal/finance/refund/application"
	"github.com/labuda/backend/internal/finance/refund/entity"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
)

// SellerRefundHandler handles seller refund decision HTTP requests.
type SellerRefundHandler struct {
	refundService *refundapp.RefundService
	database      *db.DB
	log           *zap.Logger
}

// NewSellerRefundHandler creates a new SellerRefundHandler.
func NewSellerRefundHandler(
	refundService *refundapp.RefundService,
	database *db.DB,
	log *zap.Logger,
) *SellerRefundHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &SellerRefundHandler{
		refundService: refundService,
		database:      database,
		log:           log,
	}
}

// sellerRefundDecisionRequest is the JSON body for approve/reject.
type sellerRefundDecisionRequest struct {
	Notes *string `json:"notes,omitempty"`
}

// ApproveRefund handles POST /api/v1/refunds/:id/approve.
//
// Seller approves the buyer's refund request. This triggers the canonical
// gateway refund dispatch. The seller ID is extracted from the auth context.
func (h *SellerRefundHandler) ApproveRefund(c *gin.Context) {
	ctx := c.Request.Context()

	refundID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid refund ID")
		return
	}

	sellerID, ok := extractUserID(c)
	if !ok {
		return
	}

	var req sellerRefundDecisionRequest
	// Body is optional (notes only), so ignore bind errors for empty body
	_ = c.ShouldBindJSON(&req)

	var result map[string]interface{}
	txErr := h.database.WithTx(ctx, func(tx db.Tx) error {
		refund, err := h.refundService.ApproveRefund(ctx, tx, refundID, sellerID, refundapp.ApproveRefundInput{
			Notes: req.Notes,
		})
		if err != nil {
			return err
		}

		result = buildRefundResponse(refund)
		return nil
	})

	if txErr != nil {
		h.handleError(c, txErr, refundID, sellerID, "approve")
		return
	}

	response.Success(c, result)
}

// RejectRefund handles POST /api/v1/refunds/:id/reject.
//
// Seller rejects the buyer's refund request. No money movement occurs.
// The buyer may escalate to a dispute after rejection.
func (h *SellerRefundHandler) RejectRefund(c *gin.Context) {
	ctx := c.Request.Context()

	refundID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid refund ID")
		return
	}

	sellerID, ok := extractUserID(c)
	if !ok {
		return
	}

	var req sellerRefundDecisionRequest
	_ = c.ShouldBindJSON(&req)

	var result map[string]interface{}
	txErr := h.database.WithTx(ctx, func(tx db.Tx) error {
		refund, err := h.refundService.RejectRefund(ctx, tx, refundID, sellerID, refundapp.RejectRefundInput{
			Notes: req.Notes,
		})
		if err != nil {
			return err
		}

		result = buildRefundResponse(refund)
		return nil
	})

	if txErr != nil {
		h.handleError(c, txErr, refundID, sellerID, "reject")
		return
	}

	response.Success(c, result)
}

// extractUserID pulls the authenticated user ID from the gin context.
func extractUserID(c *gin.Context) (uuid.UUID, bool) {
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return uuid.Nil, false
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalError(c, "Invalid user ID in context")
		return uuid.Nil, false
	}
	return userID, true
}

// buildRefundResponse builds a response map from a refund entity.
func buildRefundResponse(refund *entity.Refund) map[string]interface{} {
	resp := map[string]interface{}{
		"id":               refund.ID,
		"order_id":         refund.OrderID,
		"buyer_id":         refund.BuyerID,
		"seller_id":        refund.SellerID,
		"reason":           string(refund.Reason),
		"status":           string(refund.Status),
		"requested_amount": refund.RequestedAmount,
		"opened_at":        refund.OpenedAt,
		"created_at":       refund.CreatedAt,
		"updated_at":       refund.UpdatedAt,
	}
	if refund.Description != nil {
		resp["description"] = *refund.Description
	}
	if refund.SellerApprovedAmount != nil {
		resp["seller_approved_amount"] = *refund.SellerApprovedAmount
	}
	if refund.SellerApprovedPercent != nil {
		resp["seller_approved_percent"] = *refund.SellerApprovedPercent
	}
	if refund.SellerNotes != nil {
		resp["seller_notes"] = *refund.SellerNotes
	}
	if refund.SellerReviewedAt != nil {
		resp["seller_reviewed_at"] = *refund.SellerReviewedAt
	}
	if refund.ApprovedAt != nil {
		resp["approved_at"] = *refund.ApprovedAt
	}
	if refund.RejectedAt != nil {
		resp["rejected_at"] = *refund.RejectedAt
	}
	return resp
}

// handleError translates service errors to HTTP responses.
func (h *SellerRefundHandler) handleError(c *gin.Context, err error, refundID, sellerID uuid.UUID, action string) {
	h.log.Error("seller_refund_"+action+"_failed",
		zap.String("refund_id", refundID.String()),
		zap.String("seller_id", sellerID.String()),
		zap.Error(err),
	)

	errMsg := err.Error()
	switch {
	case strings.Contains(errMsg, "requires admin review"):
		response.Error(c, 422, "ADMIN_REVIEW_REQUIRED", "This refund reason requires admin review; seller cannot auto-approve. Reject and let buyer escalate to dispute.")
	case strings.Contains(errMsg, "only the seller of this order can"):
		response.Forbidden(c, "Only the seller of this order can "+action+" the refund")
	case strings.Contains(errMsg, "invalid refund status transition"):
		response.Error(c, 409, "INVALID_STATE", "Refund cannot be "+action+"d in its current state")
	case strings.Contains(errMsg, "refund already resolved"):
		response.Error(c, 409, "ALREADY_RESOLVED", "Refund has already been resolved")
	case strings.Contains(errMsg, "refund not found"):
		response.NotFound(c, "Refund not found")
	case strings.Contains(errMsg, "gateway refund client not configured"):
		response.FeatureDisabled(c, "gateway_refund")
	default:
		response.InternalServerError(c, "Failed to "+action+" refund")
	}
}


