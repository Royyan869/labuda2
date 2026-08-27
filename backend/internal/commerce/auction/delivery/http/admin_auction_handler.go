// Package http: admin-only auction emergency cancel/override (PASS_5B).
//
// This handler exists so an admin can stop a time-bound, bid-based auction
// under governance authority — e.g. an unreachable/abusive seller, or a
// trust-and-safety stop — without going through the seller-facing cancel
// route (which requires ownership and blocks cancellation once bids exist).
//
// It is gated by:
//  1. Admin role (RequireAdminMiddleware) at the route level.
//  2. Capability "governance.auction.cancel" at the route level.
//
// GOVERNANCE INVARIANT: this handler never mutates money, escrow, payout,
// or order state, never creates or cancels an order, never deletes bids,
// the auction, or the underlying product. See AuctionService.AdminCancel
// for the full safe/conflict state contract.
package http

import (
	"context"
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/labuda/backend/internal/audit"
	auctionApp "github.com/labuda/backend/internal/commerce/auction/application"
	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
)

// adminAuctionCanceller is the minimal AuctionService surface this handler
// depends on. Defined as an interface — mirroring GatewayRefundClient /
// gatewayRefundInitiator elsewhere in the codebase — so tests can inject a
// fake without a live database.
type adminAuctionCanceller interface {
	AdminCancel(ctx context.Context, tx db.Tx, input auctionApp.AdminCancelInput) (*auctionEntity.Auction, auctionEntity.Status, error)
}

// AdminAuctionHandler exposes the admin emergency auction cancel/override.
type AdminAuctionHandler struct {
	auctionService   adminAuctionCanceller
	database         db.Transactor
	adminAuditLogger audit.AdminAuditLogger
	log              *zap.Logger
}

// NewAdminAuctionHandler builds the handler. auctionService MUST be the
// singleton AuctionService constructed in dependencies.go.
func NewAdminAuctionHandler(
	auctionService *auctionApp.AuctionService,
	database *db.DB,
	adminAuditLogger audit.AdminAuditLogger,
	log *zap.Logger,
) *AdminAuctionHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &AdminAuctionHandler{
		auctionService:   auctionService,
		database:         database,
		adminAuditLogger: adminAuditLogger,
		log:              log,
	}
}

// adminCancelAuctionRequest is the JSON body expected on POST.
type adminCancelAuctionRequest struct {
	Reason string `json:"reason" binding:"required"`
}

// adminCancelAuctionResponse is returned to the admin on success.
type adminCancelAuctionResponse struct {
	AuctionID    uuid.UUID `json:"auction_id"`
	StatusBefore string    `json:"status_before"`
	StatusAfter  string    `json:"status_after"`
	Reason       string    `json:"reason"`
}

// CancelAuction handles POST /api/v1/admin/auctions/:id/cancel.
//
// Translation rules:
//   - invalid auction id                          -> 400 BAD_REQUEST
//   - empty/whitespace-only reason                 -> 400 BAD_REQUEST
//   - auction not found                            -> 404 NOT_FOUND
//   - auction already terminal / already has order -> 409 CONFLICT
//   - everything else                              -> 500 INTERNAL_SERVER_ERROR
func (h *AdminAuctionHandler) CancelAuction(c *gin.Context) {
	auctionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid auction id: must be uuid")
		return
	}

	var req adminCancelAuctionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid body: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Reason) == "" {
		response.BadRequest(c, "reason is required")
		return
	}

	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()
	var auction *auctionEntity.Auction
	var statusBefore auctionEntity.Status
	dispatchErr := h.database.WithTx(ctx, func(tx db.Tx) error {
		var err error
		auction, statusBefore, err = h.auctionService.AdminCancel(ctx, tx, auctionApp.AdminCancelInput{
			AuctionID: auctionID,
			Reason:    req.Reason,
		})
		return err
	})

	if dispatchErr != nil {
		var conflictErr *auctionApp.ErrAuctionCancelConflict
		switch {
		case errors.Is(dispatchErr, auctionApp.ErrAuctionCancelReasonRequired):
			response.BadRequest(c, dispatchErr.Error())
			return
		case errors.As(dispatchErr, &conflictErr):
			h.log.Warn("admin_auction_cancel_conflict",
				zap.String("auction_id", auctionID.String()),
				zap.String("admin_id", adminID.String()),
				zap.Error(dispatchErr),
			)
			response.Error(c, 409, "AUCTION_CANCEL_CONFLICT", dispatchErr.Error())
			return
		case strings.Contains(dispatchErr.Error(), "auction not found"):
			response.NotFound(c, "auction not found")
			return
		default:
			h.log.Error("admin_auction_cancel_failed",
				zap.String("auction_id", auctionID.String()),
				zap.String("admin_id", adminID.String()),
				zap.Error(dispatchErr),
			)
			response.InternalServerError(c, "failed to cancel auction")
			return
		}
	}

	if h.adminAuditLogger != nil {
		h.adminAuditLogger.LogSafe(ctx, adminID,
			audit.ActionAuctionAdminCancelled, audit.TargetTypeAuction, auction.ID,
			map[string]interface{}{
				"reason":        req.Reason,
				"status_before": string(statusBefore),
				"status_after":  string(auction.Status),
				"seller_id":     auction.SellerID.String(),
			},
		)
	}

	response.Success(c, adminCancelAuctionResponse{
		AuctionID:    auction.ID,
		StatusBefore: string(statusBefore),
		StatusAfter:  string(auction.Status),
		Reason:       req.Reason,
	})
}
