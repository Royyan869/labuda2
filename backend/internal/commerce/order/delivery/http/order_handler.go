package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	auctionRepoImpl "github.com/labuda/backend/internal/commerce/auction/infrastructure/repository"
	orderApp "github.com/labuda/backend/internal/commerce/order/application"
	orderDTO "github.com/labuda/backend/internal/commerce/order/delivery/http/dto"
	orderEntity "github.com/labuda/backend/internal/commerce/order/entity"
	orderRepo "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	orderrepository "github.com/labuda/backend/internal/commerce/order/repository"
	shippingApp "github.com/labuda/backend/internal/commerce/shipping/application"
	refundApp "github.com/labuda/backend/internal/finance/refund/application"
	"github.com/labuda/backend/internal/governance/dispute/application"
	disputeEntity "github.com/labuda/backend/internal/governance/dispute/entity"
	addressentity "github.com/labuda/backend/internal/identity/address/entity"
	"github.com/labuda/backend/internal/identity/auth"
	idempotencyRepo "github.com/labuda/backend/internal/platform/idempotency/repository"
	"github.com/labuda/backend/internal/platform/response"
	pricingtokenapp "github.com/labuda/backend/internal/pricing/token/application"
	pricingtokenentity "github.com/labuda/backend/internal/pricing/token/entity"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// OrderHandler handles HTTP requests for order operations.
type OrderHandler struct {
	queryService        *orderApp.OrderQueryService
	orderService        *orderApp.OrderService
	disputeService      *application.DisputeService
	refundService       *refundApp.RefundService
	orderRepository     orderrepository.OrderRepository
	pricingTokenService *pricingtokenapp.PricingTokenService
	auctionRepo         *auctionRepoImpl.AuctionRepository
	roleChecker         auth.RoleChecker
	idempotencyRepo     *idempotencyRepo.Repository
	db                  *db.DB
	log                 *zap.Logger
}

// NewOrderHandler creates a new OrderHandler.
func NewOrderHandler(
	queryService *orderApp.OrderQueryService,
	orderService *orderApp.OrderService,
	disputeService *application.DisputeService,
	refundService *refundApp.RefundService,
	pricingTokenService *pricingtokenapp.PricingTokenService,
	roleChecker auth.RoleChecker,
	db *db.DB,
	log *zap.Logger,
) *OrderHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &OrderHandler{
		queryService:        queryService,
		orderService:        orderService,
		disputeService:      disputeService,
		refundService:       refundService,
		orderRepository:     orderRepo.NewOrderRepository(),
		pricingTokenService: pricingTokenService,
		auctionRepo:         auctionRepoImpl.NewAuctionRepository(),
		roleChecker:         roleChecker,
		idempotencyRepo:     idempotencyRepo.NewRepository(),
		db:                  db,
		log:                 log,
	}
}

// ListMyOrdersRequest holds the query parameters for ListMyOrders.
type ListMyOrdersRequest struct {
	Role   string  `form:"role" binding:"required,oneof=buyer seller"`
	Status *string `form:"status"`
	Limit  int     `form:"limit" binding:"omitempty,min=1,max=50"`
	Cursor int64   `form:"cursor" binding:"omitempty,min=0"`
}

// ListMyOrders handles GET /api/v1/orders
//
// Query parameters:
// - role (required): "buyer" or "seller" - filters orders by the caller's role
// - status (optional): Filter by order status (e.g., "pending", "paid", "shipped", "delivered", "completed", "cancelled", "expired", "refunded")
// - limit (optional): Number of results per page (default: 20, max: 50)
// - cursor (optional): Unix timestamp for pagination - returns orders created before this time
//
// Response:
// - orders: Array of order items with computed role and allowed actions
// - next_cursor: Unix timestamp for fetching the next page (only if more results exist)
// - limit: The limit used for this request
func (h *OrderHandler) ListMyOrders(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context (set by auth middleware)
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Parse query parameters
	var req ListMyOrdersRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}

	// Set default limit if not provided
	if req.Limit <= 0 {
		req.Limit = 20
	}

	// Build query service input
	input := orderApp.ListMyOrdersInput{
		CallerID:  userID,
		RoleParam: req.Role,
		Status:    req.Status,
		Limit:     req.Limit,
		Cursor:    req.Cursor,
	}

	// Execute query within transaction
	var result *orderApp.OrderListResponse
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		result, err = h.queryService.ListMyOrders(ctx, tx, input)
		return err
	})

	if err != nil {
		h.log.Error("Failed to list orders",
			zap.String("user_id", userID.String()),
			zap.String("role", req.Role),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve orders")
		return
	}

	response.Success(c, result)
}

// GetOrder retrieves a single order by ID.
//
// GET /api/v1/orders/{id}
//
// Authorization:
// - Caller must be a participant (buyer or seller) OR admin
// - Returns 403 if not participant
// - Returns 404 if not found
func (h *OrderHandler) GetOrder(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse order ID
	idStr := c.Param("id")
	orderID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid order ID")
		return
	}

	// Get user ID from context
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Get order from database
	var order *orderEntity.Order
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		order, err = h.orderService.GetOrder(ctx, tx, orderID)
		return err
	})

	if err != nil {
		h.log.Error("Failed to get order",
			zap.String("order_id", orderID.String()),
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.NotFound(c, "Order not found")
		return
	}

	// Authorization: only buyer, seller, or admin can view
	isAdmin, err := h.roleChecker.IsAdmin(ctx, userID)
	if err != nil {
		response.InternalServerError(c, "Failed to verify permissions")
		return
	}
	if !isAdmin && order.BuyerID != userID && order.SellerID != userID {
		response.Forbidden(c, "You are not a participant in this order")
		return
	}

	// Fetch buyer and seller display info.
	//
	// Phase 5 Stage 1 — SELLER/FARM CONTRACT CONVERGENCE:
	// We fetch the canonical components separately (username, store_name,
	// avatar_url) so they can be exposed under the explicit additive
	// fields (buyer_username, seller_username, seller_farm_name,
	// seller_avatar_url) WITHOUT corruption.
	var (
		buyerName, buyerAvatar, sellerAvatar string
		// Additive convergence identity fields (strict separation).
		buyerUsername, sellerUsername, sellerFarmName, sellerAvatarURL string
		// paymentStatus/paymentID/paymentExpiredAt track the latest/active
		// payment for this order. Nil when no payment row exists (normal for
		// orders in pending_payment state before the buyer initiates payment).
		paymentStatus    *string
		paymentID        *uuid.UUID
		paymentExpiredAt *time.Time
	)
	// C1B: Query active refund state for decision builder.
	var hasActiveRefund bool
	var activeRefundStatus *string
	var activeRefundSummary *orderDTO.ActiveRefundSummary
	var orderItems []*orderEntity.OrderItem
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		// Check for active (non-terminal) refund on this order
		if h.refundService != nil {
			existingRefund, _ := h.refundService.GetRefundByOrderID(ctx, tx, order.ID)
			if existingRefund != nil && !existingRefund.IsTerminal() {
				hasActiveRefund = true
				s := string(existingRefund.Status)
				activeRefundStatus = &s

				var resolvedAt *int64
				if existingRefund.RefundedAt != nil {
					ts := existingRefund.RefundedAt.Unix()
					resolvedAt = &ts
				} else if existingRefund.RejectedAt != nil {
					ts := existingRefund.RejectedAt.Unix()
					resolvedAt = &ts
				} else if existingRefund.ApprovedAt != nil {
					ts := existingRefund.ApprovedAt.Unix()
					resolvedAt = &ts
				} else if existingRefund.AdminReviewedAt != nil {
					ts := existingRefund.AdminReviewedAt.Unix()
					resolvedAt = &ts
				}

				var gatewayStatus *string
				if existingRefund.GatewayStatus.IsValid() {
					gs := string(existingRefund.GatewayStatus)
					gatewayStatus = &gs
				}

				activeRefundSummary = &orderDTO.ActiveRefundSummary{
					ID:              existingRefund.ID,
					OrderID:         existingRefund.OrderID,
					BuyerID:         existingRefund.BuyerID,
					SellerID:        existingRefund.SellerID,
					Status:          string(existingRefund.Status),
					Reason:          string(existingRefund.Reason),
					Description:     existingRefund.Description,
					RequestedAmount: existingRefund.RequestedAmount,
					SellerNotes:     existingRefund.SellerNotes,
					EvidenceURLs:    existingRefund.EvidenceURLs,
					CreatedAt:       existingRefund.CreatedAt.Unix(),
					UpdatedAt:       existingRefund.UpdatedAt.Unix(),
					AdminNotes:      existingRefund.AdminNotes,
					ResolvedAt:      resolvedAt,
					GatewayStatus:   gatewayStatus,
				}
			}
		}
		// Fetch buyer profile (canonical public identity).
		err := tx.QueryRow(ctx, `
			SELECT COALESCE(username, ''), COALESCE(avatar_url, '')
			FROM user_profiles WHERE user_id = $1
		`, order.BuyerID).Scan(&buyerName, &buyerAvatar)
		if err != nil {
			// Profile may not exist, use empty strings
			buyerName = ""
			buyerAvatar = ""
		}
		// buyer_username = user_profiles.username (canonical, identical
		// to the buyer_name field on this surface).
		buyerUsername = buyerName

		// Fetch seller profile (user_profiles + seller_profiles).
		// We retrieve username, store_name, and avatar_url SEPARATELY
		// so we can expose them under explicit additive fields without
		// COALESCE corruption.
		var storeName, username, sellerAvatarUrl string
		err = tx.QueryRow(ctx, `
			SELECT COALESCE(sp.store_name, ''), COALESCE(up.username, ''), COALESCE(up.avatar_url, '')
			FROM seller_profiles sp
			LEFT JOIN user_profiles up ON up.user_id = sp.user_id
			WHERE sp.user_id = $1
		`, order.SellerID).Scan(&storeName, &username, &sellerAvatarUrl)
		if err != nil {
			// Try the user_profiles-only path.
			err = tx.QueryRow(ctx, `
				SELECT COALESCE(username, ''), COALESCE(avatar_url, '')
				FROM user_profiles WHERE user_id = $1
			`, order.SellerID).Scan(&username, &sellerAvatarUrl)
			if err != nil {
				username = ""
				sellerAvatarUrl = ""
			}
		}

		// (store_name preferred, falls back to username) — preserved
		sellerAvatar = sellerAvatarUrl

		// Phase 5 Stage 1 additive convergence — strict source separation:
		//   seller_username   = user_profiles.username   (NEVER store_name)
		//   seller_farm_name  = seller_profiles.store_name (NEVER username)
		//   seller_avatar_url = user_profiles.avatar_url
		sellerUsername = username
		sellerFarmName = storeName
		sellerAvatarURL = sellerAvatarUrl

		// Fetch order items with snapshot data
		orderItems, err = h.orderRepository.GetOrderItems(ctx, tx, orderID)
		if err != nil {
			return fmt.Errorf("failed to fetch order items: %w", err)
		}

		// Fetch payment status, ID, and expiry for this order.
		// Priority: settlement(1) > capture(2) > pending(3) > others(4).
		// No row = no payment yet (normal for pending_payment orders).
		// expired_at is threaded into the decision builder so the buyer CTA
		// can distinguish an active pending payment from one whose window
		// has lapsed (see selectPayActionLabelKey).
		var psIDStr, psStr string
		var psExpiredAt time.Time
		psErr := tx.QueryRow(ctx, `
			SELECT id::text, status::text, expired_at
			FROM payments
			WHERE reference_type = 'order'
			  AND reference_id = $1
			ORDER BY
			  CASE status::text
			    WHEN 'settlement' THEN 1
			    WHEN 'capture'    THEN 2
			    WHEN 'pending'    THEN 3
			    ELSE 4
			  END ASC,
			  created_at DESC
			LIMIT 1
		`, orderID).Scan(&psIDStr, &psStr, &psExpiredAt)
		if psErr == nil {
			paymentStatus = &psStr
			paymentExpiredAt = &psExpiredAt
			if pid, parseErr := uuid.Parse(psIDStr); parseErr == nil {
				paymentID = &pid
			}
		}
		// psErr non-nil = no payment row; leave paymentStatus/paymentID/paymentExpiredAt nil.

		return nil
	})

	if err != nil {
		h.log.Warn("Failed to fetch user profiles for order",
			zap.String("order_id", orderID.String()),
			zap.Error(err),
		)
		// Continue with empty display fields
	}

	// Convert to response DTO with decision contract.
	// Phase 5 Stage 1 — emit additive identity fields alongside the established fields.
	orderResp := orderDTO.OrderToDetailResponseWithIdentity(
		order, userID,
		buyerName, buyerAvatar, sellerAvatar,
		buyerUsername, sellerUsername, sellerFarmName, sellerAvatarURL,
		orderItems,
		activeRefundSummary,
		hasActiveRefund, activeRefundStatus,
		paymentStatus,
		paymentID,
		paymentExpiredAt,
	)

	response.Success(c, orderResp)
}

// MarkShippedRequest holds the request body for mark shipped.
//
// SHIPPING PROOF REQUIREMENTS (STRICT - NO FAKE SHIPMENT):
// - proof_type: REQUIRED - "tracking" | "phone" | "manual"
// - tracking_number: REQUIRED for tracking/phone types
// - shipping_proof_media: REQUIRED for manual type
// - note: Optional shipping note for buyer
type MarkShippedRequest struct {
	ProofType          *string `json:"proof_type" binding:"required"`  // Required: "tracking" | "phone" | "manual"
	TrackingNumber     *string `json:"tracking_number,omitempty"`      // Required for tracking/phone
	ShippingProofMedia *string `json:"shipping_proof_media,omitempty"` // Required for manual
	Note               *string `json:"note,omitempty"`                 // Optional shipping note
}

// MarkShipped handles POST /api/v1/orders/{id}/ship
//
// Authorization:
// - Only seller can mark as shipped
// - Only allowed if status = paid
//
// Request body:
// - proof_type (required): Type of proof - "tracking" | "phone" | "manual"
// - shipping_reference (conditional): Required for tracking/phone types
// - shipping_proof_media (conditional): Required for manual type
// - shipping_note (optional): Shipping note from seller to buyer
//
// Supports Idempotency-Key header for safe retries.
// Idempotency is handled in the service layer within the same transaction.
func (h *OrderHandler) MarkShipped(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse order ID
	idStr := c.Param("id")
	orderID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid order ID")
		return
	}

	// Extract Idempotency-Key header
	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		response.BadRequest(c, "Idempotency-Key header required")
		return
	}

	// Get user ID from context
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Parse request body
	var req MarkShippedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: proof_type is required")
		return
	}

	// Validate proof_type is provided
	if req.ProofType == nil || *req.ProofType == "" {
		response.BadRequest(c, "proof_type is required")
		return
	}

	// Validate proof_type value
	proofType := *req.ProofType
	if proofType != "tracking" && proofType != "phone" && proofType != "manual" {
		response.BadRequest(c, "proof_type must be one of: tracking, phone, manual")
		return
	}

	// Validate tracking_number for tracking/phone types
	if (proofType == "tracking" || proofType == "phone") && (req.TrackingNumber == nil || *req.TrackingNumber == "") {
		response.BadRequest(c, "tracking_number is required for proof_type="+proofType)
		return
	}

	// Validate shipping_proof_media for manual type
	if proofType == "manual" && (req.ShippingProofMedia == nil || *req.ShippingProofMedia == "") {
		response.BadRequest(c, "shipping_proof_media is required for proof_type=manual")
		return
	}

	// Execute mark shipped within transaction
	// Idempotency check is performed inside the service method
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.orderService.MarkShipped(ctx, tx, userID, orderID, idempotencyKey, req.ProofType, req.TrackingNumber, req.ShippingProofMedia, req.Note)
	})

	if err != nil {
		h.log.Error("Failed to mark order as shipped",
			zap.String("order_id", orderID.String()),
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)

		// Check for authorization error
		if errors.Is(err, auth.ErrSellerRequired) {
			response.Forbidden(c, "Only seller can mark order as shipped")
			return
		}

		// Check for shipping proof validation error
		if strings.Contains(err.Error(), "invalid shipping proof") {
			response.Error(c, 400, "INVALID_PROOF", err.Error())
			return
		}

		// Check for invalid state error
		if err.Error() == "cannot mark shipped: current status does not allow this transition" {
			response.Error(c, 409, "INVALID_STATE", "Order cannot be marked shipped in current state")
			return
		}

		response.InternalServerError(c, fmt.Sprintf("Failed to mark order as shipped: %v", err))
		return
	}

	response.SuccessWithMessage(c, "Order marked as shipped", gin.H{
		"order_id": orderID,
	})
}

// CreateDisputeRequest holds the request body for creating a dispute.
type CreateDisputeRequest struct {
	Reason       string   `json:"reason" binding:"required"`
	ReasonCode   string   `json:"reason_code,omitempty"` // Standardized reason code (required by service)
	Description  *string  `json:"description,omitempty"`
	MediaURLs    []string `json:"media_urls,omitempty"`
	EvidenceURLs []string `json:"evidence_urls,omitempty"` // Alias accepted from mobile
	VideoURL     *string  `json:"video_url,omitempty"`     // Video evidence (required for buyer disputes)
}

// CreateDispute handles POST /api/v1/orders/{id}/disputes
//
// 🔥 COMPREHENSIVE UPDATE: PHASES 3-7
//
// Authorization:
// - Caller must be a participant (buyer or seller)
//
// Supports Idempotency-Key header for safe retries.
// Service: DisputeService.OpenDispute()
//
// 🔥 PHASE 4: Pre-ship disputes allowed when order is overdue
// 🔥 PHASE 5: Authorization enforced (only buyer/seller)
// 🔥 PHASE 6: Post-ship disputes limited to 12 hours window
// 🔥 PHASE 7: Video evidence required for buyer disputes
func (h *OrderHandler) CreateDispute(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse order ID
	idStr := c.Param("id")
	orderID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid order ID")
		return
	}

	// Extract Idempotency-Key header
	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		response.BadRequest(c, "Idempotency-Key header required")
		return
	}

	// Get user ID from context
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Parse request body
	var req CreateDisputeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Merge media_urls and evidence_urls (mobile sends evidence_urls)
	mergedMediaURLs := req.MediaURLs
	for _, url := range req.EvidenceURLs {
		mergedMediaURLs = append(mergedMediaURLs, url)
	}

	// Default reason_code to reason if not provided
	reasonCode := req.ReasonCode
	if reasonCode == "" {
		reasonCode = req.Reason
	}

	// Execute dispute creation within transaction
	var dispute *disputeEntity.Dispute
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		// C1B: Coexistence guard — block direct dispute if active non-rejected refund exists.
		// Buyer must wait for seller refund decision, or escalate from rejected refund.
		if h.refundService != nil {
			existingRefund, _ := h.refundService.GetRefundByOrderID(ctx, tx, orderID)
			if existingRefund != nil && !existingRefund.IsTerminal() && !existingRefund.IsRejected() {
				return fmt.Errorf("cannot open dispute: order has an active refund request (status: %s). Wait for seller decision or escalate after rejection", existingRefund.Status)
			}
		}

		input := application.OpenDisputeInput{
			Reason:      req.Reason,
			Description: req.Description,
			MediaURLs:   mergedMediaURLs,
			VideoURL:    req.VideoURL,
			ReasonCode:  reasonCode,
		}
		d, err := h.disputeService.OpenDispute(ctx, tx, orderID, userID, input)
		if err != nil {
			return err
		}
		dispute = d
		return nil
	})

	if err != nil {
		h.log.Error("Failed to create dispute",
			zap.String("order_id", orderID.String()),
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)

		if handled := h.writeCreateDisputeError(c, err); handled {
			return
		}

		response.InternalServerError(c, "Failed to create dispute")
		return
	}

	response.Created(c, dispute)
}

func (h *OrderHandler) writeCreateDisputeError(c *gin.Context, err error) bool {
	var unauthorized *orderEntity.ErrUnauthorizedDisputeAccess
	var videoRequired *orderEntity.ErrVideoRequiredForBuyerDispute
	var missingReasonCode *disputeEntity.ErrMissingReasonCode
	var invalidReasonCode *disputeEntity.ErrInvalidReasonCode
	var reasonCodeNotAuthorized *disputeEntity.ErrReasonCodeNotAuthorized
	var insufficientEvidence *disputeEntity.ErrInsufficientEvidence
	switch {
	case errors.As(err, &unauthorized):
		response.Forbidden(c, "You are not authorized to access this dispute")
		return true
	case errors.As(err, &videoRequired):
		response.Error(c, 400, "VIDEO_REQUIRED", "Buyer disputes require video evidence or attached evidence URLs")
		return true
	case errors.As(err, &insufficientEvidence):
		response.Error(c, 400, "INSUFFICIENT_EVIDENCE", "Buyer disputes require evidence")
		return true
	case errors.As(err, &missingReasonCode):
		response.BadRequest(c, "Reason code is required")
		return true
	case errors.As(err, &invalidReasonCode):
		response.BadRequest(c, "Invalid dispute reason code")
		return true
	case errors.As(err, &reasonCodeNotAuthorized):
		response.BadRequest(c, "Reason code is not allowed for this caller")
		return true
	case errors.Is(err, application.ErrDisputeOpenAlreadyHasActive):
		response.Error(c, 409, "DISPUTE_EXISTS", "Order already has an active dispute")
		return true
	case errors.Is(err, application.ErrDisputeOpenAfterCompletion):
		response.Error(c, 409, "DISPUTE_CLOSED", "Cannot open dispute after order completion. Please negotiate directly with the seller outside the app.")
		return true
	case errors.Is(err, application.ErrDisputeOpenNoEscrow), errors.Is(err, application.ErrDisputeOpenInvalidEscrowState):
		response.Error(c, 409, "INVALID_ESCROW_STATE", "Cannot open dispute in current escrow state")
		return true
	case errors.Is(err, application.ErrDisputeOpenPreShipNotEligible), errors.Is(err, application.ErrDisputeOpenPostShipWindowExpired):
		response.Error(c, 409, "INVALID_STATE", "Dispute can only be opened for eligible shipped or overdue orders")
		return true
	default:
		return false
	}
}

// CreateRefundRequest holds the request body for buyer-initiated refund.
type CreateRefundRequest struct {
	Reason          string   `json:"reason" binding:"required"`
	Description     *string  `json:"description,omitempty"`
	EvidenceURLs    []string `json:"evidence_urls,omitempty"`
	Evidence        []string `json:"evidence,omitempty"`         // Alias accepted from mobile
	RequestedAmount *int64   `json:"requested_amount,omitempty"` // Defaults to full order amount
}

// CreateRefund handles POST /api/v1/orders/{id}/refund
//
// Buyer-initiated refund request (tier 1 protection).
// Requires Idempotency-Key header for safe retries.
// Service: RefundService.CreateRefund()
func (h *OrderHandler) CreateRefund(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse order ID
	idStr := c.Param("id")
	orderID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid order ID")
		return
	}

	// Extract Idempotency-Key header
	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		response.BadRequest(c, "Idempotency-Key header required")
		return
	}

	// Get user ID from context
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Parse request body
	var req CreateRefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Merge evidence_urls and evidence (mobile sends "evidence")
	evidenceURLs := req.EvidenceURLs
	for _, url := range req.Evidence {
		evidenceURLs = append(evidenceURLs, url)
	}

	// Execute refund creation within transaction
	var refundResult map[string]interface{}
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		// If requested_amount not specified, default to full order amount
		requestedAmount := int64(0)
		if req.RequestedAmount != nil && *req.RequestedAmount > 0 {
			requestedAmount = *req.RequestedAmount
		} else {
			order, err := h.orderRepository.GetByID(ctx, tx, orderID)
			if err != nil {
				return fmt.Errorf("order not found: %w", err)
			}
			requestedAmount = order.Subtotal.Int64() + order.ShippingTotal.Int64()
		}

		input := refundApp.CreateRefundInput{
			IdempotencyKey:  idempotencyKey,
			Reason:          req.Reason,
			Description:     req.Description,
			EvidenceURLs:    evidenceURLs,
			RequestedAmount: requestedAmount,
		}
		refund, err := h.refundService.CreateRefund(ctx, tx, orderID, userID, input)
		if err != nil {
			return err
		}

		// Build response matching RefundResponse shape for mobile
		refundResult = map[string]interface{}{
			"id":               refund.ID,
			"order_id":         refund.OrderID,
			"buyer_id":         refund.BuyerID,
			"seller_id":        refund.SellerID,
			"reason":           string(refund.Reason),
			"description":      refund.Description,
			"evidence_urls":    refund.EvidenceURLs,
			"status":           string(refund.Status),
			"requested_amount": refund.RequestedAmount,
			"opened_at":        refund.OpenedAt,
			"created_at":       refund.CreatedAt,
			"updated_at":       refund.UpdatedAt,
		}
		return nil
	})

	if err != nil {
		h.log.Error("Failed to create refund",
			zap.String("order_id", orderID.String()),
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)

		errMsg := err.Error()
		if strings.Contains(errMsg, "only buyer can request refund") {
			response.Error(c, 403, "BUYER_REQUIRED", "Only the buyer can request a refund")
			return
		}
		if strings.Contains(errMsg, "cannot request refund: order must be shipped") {
			response.Error(c, 409, "INVALID_STATE", "Refund can only be requested for shipped orders")
			return
		}
		if strings.Contains(errMsg, "cannot request refund: escrow must be in holding state") {
			response.Error(c, 409, "INVALID_ESCROW_STATE", "Cannot request refund in current escrow state")
			return
		}
		if strings.Contains(errMsg, "order already has an active refund request") {
			response.Error(c, 409, "REFUND_EXISTS", "Order already has an active refund request")
			return
		}
		if strings.Contains(errMsg, "order has an active dispute") {
			response.Error(c, 409, "DISPUTE_ACTIVE", "Cannot request refund while dispute is active")
			return
		}
		if strings.Contains(errMsg, "invalid refund reason") {
			response.BadRequest(c, "Invalid refund reason: "+req.Reason)
			return
		}
		if strings.Contains(errMsg, "requested amount") {
			response.BadRequest(c, errMsg)
			return
		}
		if strings.Contains(errMsg, "order not found") {
			response.NotFound(c, "Order not found")
			return
		}

		response.InternalServerError(c, "Failed to create refund")
		return
	}

	response.Created(c, refundResult)
}

// CreateOrderRequest holds the request body for unified order creation.
type CreateOrderRequest struct {
	// REQUIRED: Canonical sale surface identity
	ProductID  string `json:"product_id" binding:"required,uuid"`
	SourceType string `json:"source_type" binding:"required,oneof=for_sale auction"`
	SourceID   string `json:"source_id" binding:"required,uuid"`

	// OPTIONAL: Negotiation session ID for price override
	// If provided, the negotiated price will be used instead of fixed-price sale price
	NegotiationID *string `json:"negotiation_id,omitempty"`

	// Common fields (required)
	Quantity         int     `json:"quantity" binding:"required,min=1"`
	AddressID        string  `json:"address_id" binding:"required"`
	ShippingOptionID *string `json:"shipping_option_id,omitempty"` // Optional: when using shipping_quote_id
	ShippingQuoteID  *string `json:"shipping_quote_id,omitempty"`  // Optional: when using manual quote
	ProvinceCode     string  `json:"province_code"`                // Deprecated
	CityCode         string  `json:"city_code"`                    // Deprecated

	// Pricing token (required for ALL orders to prevent price manipulation)
	// The token must have been obtained from the pricing preview endpoint
	PricingToken *string `json:"pricing_token,omitempty"`
}

// CreateOrder handles POST /api/v1/orders
//
// UNIFIED ORDER CREATION ENDPOINT (Auction Closure Fix Pack V1):
// This is the PRIMARY client-facing endpoint for creating ALL order types.
//
// Creates an order from various source types using a unified interface:
// - "for_sale": Direct purchase from a fixed-price sale
// - "negotiation": Purchase from an accepted negotiation
// - "auction": Purchase from an auction (buy-now or winner claim)
//
// Request body:
// - source_type: "for_sale" or "auction"
// - source_id: UUID of the source entity
// - quantity: Quantity to order
// - address_id: Shipping address ID
// - shipping_option_id: Selected shipping option ID
// - pricing_token: REQUIRED - token from pricing preview endpoint
//
// Idempotency:
// - Supports Idempotency-Key header for safe retries (recommended)
// - If provided, subsequent requests with same key return the existing order
// - Key is scoped per user (different users can use same key)
//
// Authorization:
// - Caller must be authenticated
// - Fixed-price sale must be active and available
// - If negotiation_id provided: must be the buyer, negotiation must be in 'accepted' status
func (h *OrderHandler) CreateOrder(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Extract Idempotency-Key header (for safe retries)
	// REQUIRED per RUNTIME-INVARIANTS §2.1 — order creation is a retry-prone
	// mutation; missing key would leave a buyer-side double-tap relying solely on
	// pricing-token uniqueness for dedup. Match the reject pattern used by other
	// mutation handlers in this file (e.g., CompleteOrder, MarkShipped).
	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		response.Error(c, 400, "IDEMPOTENCY_KEY_REQUIRED", "Idempotency-Key header is required")
		return
	}

	// Parse request body
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	sourceFieldMarker := []byte{'"', 'l', 'i', 's', 't', 'i', 'n', 'g', '_', 'i', 'd', '"'}
	if bytes.Contains(rawBody, sourceFieldMarker) {
		response.BadRequest(c, "old order source field is not supported; use product_id, source_type, and source_id")
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(rawBody))
	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}

	// ============================================================
	// PRICING TOKEN REQUIRED (HARD REQUIREMENT)
	// ============================================================
	// All order creation MUST provide a valid pricing token.
	// The token contains the authoritative pricing snapshot from preview time.
	// NO pricing data from the frontend is trusted.
	if req.PricingToken == nil || *req.PricingToken == "" {
		response.BadRequest(c, "pricing_token is required")
		return
	}

	// Parse pricing token
	tokenID, err := uuid.Parse(*req.PricingToken)
	if err != nil {
		response.BadRequest(c, "invalid pricing token format")
		return
	}

	// Parse canonical product and source identities.
	productID, err := uuid.Parse(req.ProductID)
	if err != nil {
		response.BadRequest(c, "Invalid product_id")
		return
	}
	sourceID, err := uuid.Parse(req.SourceID)
	if err != nil {
		response.BadRequest(c, "Invalid source_id")
		return
	}
	sourceType := orderEntity.OrderSourceType(req.SourceType)
	if !sourceType.IsValid() {
		response.BadRequest(c, "source_type must be for_sale or auction")
		return
	}

	// Parse optional negotiation ID
	var negotiationID *uuid.UUID
	if req.NegotiationID != nil && *req.NegotiationID != "" {
		parsed, err := uuid.Parse(*req.NegotiationID)
		if err != nil {
			response.BadRequest(c, "Invalid negotiation_id")
			return
		}
		negotiationID = &parsed
	}

	// Parse address ID
	addressID, err := uuid.Parse(req.AddressID)
	if err != nil {
		response.BadRequest(c, "Invalid address ID")
		return
	}

	// Parse shipping option ID (optional - can use shipping quote instead)
	shippingOptionID := uuid.Nil
	if req.ShippingOptionID != nil && *req.ShippingOptionID != "" {
		shippingOptionID, err = uuid.Parse(*req.ShippingOptionID)
		if err != nil {
			response.BadRequest(c, "Invalid shipping option ID")
			return
		}
	}

	// Validate: either shipping_option_id or shipping_quote_id must be provided
	hasShippingQuote := req.ShippingQuoteID != nil && *req.ShippingQuoteID != ""
	if shippingOptionID == uuid.Nil && !hasShippingQuote {
		response.BadRequest(c, "Either shipping_option_id or shipping_quote_id must be provided")
		return
	}
	if shippingOptionID != uuid.Nil && hasShippingQuote {
		response.BadRequest(c, "Cannot provide both shipping_option_id and shipping_quote_id")
		return
	}

	// ============================================================
	// PRICING TOKEN VALIDATION AND SNAPSHOT CREATION
	// ============================================================
	// The pricing token is validated WITHIN the transaction to ensure atomicity.
	// This prevents race conditions and ensures the token is used exactly once.
	//
	// IMPORTANT: All pricing data (quantity, prices, fees) comes from the token.
	// The frontend CANNOT manipulate pricing - the token is the single source of truth.
	var pricingSnapshot *orderApp.PricingSnapshot

	// Execute within transaction
	var order *orderEntity.Order
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		// Step 1: Validate pricing token under row lock before order creation.
		// Consumption happens only after a real order ID exists in this same transaction.
		validatedToken, err := h.pricingTokenService.ValidateForOrderLocked(
			ctx,
			tx,
			tokenID,
			userID,
			productID,
			req.SourceType,
			sourceID,
			0, // Quantity from token, not request
			addressID,
			shippingOptionID,
		)
		if err != nil {
			return fmt.Errorf("pricing token validation failed: %w", err)
		}

		// Step 2: Build pricing snapshot from validated token
		// This snapshot is the ONLY source of truth for order pricing
		pricingSnapshot = buildPricingSnapshotFromToken(validatedToken)

		switch sourceType {
		case orderEntity.OrderSourceForSale:
			// Step 3: Prepare input with pricing snapshot
			// CRITICAL: Quantity comes from token, NOT from request
			// Frontend cannot manipulate quantity or pricing
			// Idempotency key is guaranteed non-empty by the header check above.
			input := orderApp.CreateFromSaleSurfaceInput{
				ProductID:        productID,
				SourceType:       sourceType,
				SourceID:         sourceID,
				BuyerID:          userID,
				Quantity:         validatedToken.Quantity, // From token, not request
				AddressID:        addressID,
				ShippingOptionID: shippingOptionID,
				ProvinceCode:     req.ProvinceCode,
				CityCode:         req.CityCode,
				IdempotencyKey:   &idempotencyKey,
				NegotiationID:    negotiationID,   // Optional negotiation context
				PricingSnapshot:  pricingSnapshot, // PRICING FROM TOKEN
				PricingTokenID:   &tokenID,        // Store token ID (prevents double-ordering)
			}

			// Step 4: Create order using pricing snapshot
			order, err = h.orderService.CreateFromSaleSurface(ctx, tx, input)
			if err != nil {
				return err
			}
		case orderEntity.OrderSourceAuction:
			auction, err := h.auctionRepo.GetForUpdate(ctx, tx, sourceID)
			if err != nil {
				return fmt.Errorf("failed to load auction for checkout: %w", err)
			}

			if auction.OrderID != nil {
				return fmt.Errorf("auction already settled: order_id=%s", *auction.OrderID)
			}
			if auction.Status != auctionEntity.StatusActive || auction.BuyNowPrice == nil {
				return fmt.Errorf("auction is not available for buy now checkout: status=%s", auction.Status)
			}
			if auction.BuyNowPrice != nil && validatedToken.UnitPrice.Int64() != *auction.BuyNowPrice {
				return fmt.Errorf("auction price changed after preview: preview=%d current=%d", validatedToken.UnitPrice.Int64(), *auction.BuyNowPrice)
			}

			order, err = h.orderService.CreateFromAuction(ctx, tx, orderApp.CreateFromAuctionInput{
				AuctionID:             auction.ID,
				AuctionSellerID:       auction.SellerID,
				ProductID:             productID,
				BuyerID:               userID,
				WinningBid:            validatedToken.UnitPrice.Int64(),
				AddressID:             addressID,
				ShippingOptionID:      shippingOptionID,
				ProvinceCode:          req.ProvinceCode,
				CityCode:              req.CityCode,
				DiscountCode:          nil,
				AuctionSettlementType: orderEntity.AuctionSettlementBuyNow,
				PricingSnapshot:       pricingSnapshot,
				IdempotencyKey:        &idempotencyKey,
			})
			if err != nil {
				return err
			}

			// PASS_20B (D1): bind and close the auction in the same
			// transaction as order creation. Without this, the auction row
			// is never updated — it stays order_id=NULL, status=active — so
			// a second buyer could buy-now or bid on the same unique item
			// again. This mirrors what the dedicated /auctions/:id/claim
			// handler already does for bid-win settlement.
			auction.OrderID = &order.ID
			if err := auction.End(); err != nil {
				return fmt.Errorf("failed to settle auction after buy-now: %w", err)
			}
			if err := h.auctionRepo.UpdateTx(ctx, tx, auction); err != nil {
				return fmt.Errorf("failed to persist auction settlement: %w", err)
			}
		default:
			return fmt.Errorf("source_type must be for_sale or auction")
		}

		// Step 5: Mark token used with the real order ID in the same transaction.
		if err := h.pricingTokenService.FinalizeOrderConsumption(ctx, tx, validatedToken, order.ID); err != nil {
			return fmt.Errorf("pricing token consume failed: %w", err)
		}

		return nil
	})

	if err != nil {
		var tokenValidationErr *pricingtokenentity.ValidationError
		if errors.As(err, &tokenValidationErr) && tokenValidationErr.Code == pricingtokenentity.CodeTokenAlreadyUsed {
			if tokenValidationErr.OrderID == nil || *tokenValidationErr.OrderID == uuid.Nil {
				response.Error(c, 409, "PRICING_TOKEN_ALREADY_USED", "Pricing token already used but not linked to an order")
				return
			}

			existingOrder, recoverErr := h.recoverOrderFromUsedPricingToken(ctx, tokenID, *tokenValidationErr.OrderID)
			if recoverErr != nil || existingOrder == nil {
				h.log.Warn("Used pricing token detected but recovery fetch failed",
					zap.String("pricing_token_id", tokenID.String()),
					zap.String("order_id", tokenValidationErr.OrderID.String()),
					zap.String("user_id", userID.String()),
					zap.Error(recoverErr),
				)
				response.Error(c, 409, "PRICING_TOKEN_ALREADY_USED", "Pricing token already used for an existing order")
				return
			}

			response.Created(c, existingOrder)
			return
		}

		if errors.Is(err, orderrepository.ErrDuplicatePricingToken) {
			var existingOrder *orderEntity.Order
			fetchErr := h.db.WithTx(ctx, func(tx db.Tx) error {
				var lookupErr error
				existingOrder, lookupErr = h.orderRepository.GetByPricingTokenID(ctx, tx, tokenID)
				return lookupErr
			})
			if fetchErr != nil || existingOrder == nil {
				h.log.Warn("Duplicate pricing token detected but recovery fetch failed",
					zap.String("pricing_token_id", tokenID.String()),
					zap.String("user_id", userID.String()),
					zap.Error(fetchErr),
				)
				response.InternalServerError(c, "Failed to create order")
				return
			}

			response.Created(c, existingOrder)
			return
		}

		if errors.Is(err, orderrepository.ErrDuplicateIdempotencyKey) {
			h.log.Info("Duplicate buyer idempotency key on order create",
				zap.String("user_id", userID.String()),
			)
			response.Error(c, 409, "DUPLICATE_IDEMPOTENCY_KEY", "Idempotency key already used by a different request")
			return
		}

		h.log.Error("Failed to create order",
			zap.String("product_id", productID.String()),
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)

		// Phase 0 honesty: surface typed shipping gate errors as machine-readable
		// codes so mobile can branch into the canonical uncovered-area UX instead
		// of falling back to substring matching on opaque sentences.
		if errors.Is(err, shippingApp.ErrNoShippingOptions) {
			response.Error(c, 400, "NO_SHIPPING_OPTIONS",
				"Penjual belum mengatur pengiriman untuk produk ini.")
			return
		}
		if errors.Is(err, shippingApp.ErrShippingOptionUnavailable) {
			response.Error(c, 400, "SHIPPING_OPTION_UNAVAILABLE",
				"Produk ini di luar area pengiriman untuk alamat Anda.")
			return
		}

		// Handle specific errors
		errMsg := err.Error()
		if strings.Contains(errMsg, "sale surface not active") || strings.Contains(errMsg, "sale surface not available") {
			response.Error(c, 400, "SALE_SURFACE_NOT_AVAILABLE", "Sale surface is not available for purchase")
			return
		}
		if strings.Contains(errMsg, "insufficient stock") {
			response.Error(c, 400, "INSUFFICIENT_STOCK", "Not enough stock available")
			return
		}
		if strings.Contains(errMsg, "negotiation is not accepted") {
			response.Error(c, 400, "NEGOTIATION_NOT_ACCEPTED", "Negotiation is not accepted")
			return
		}
		if strings.Contains(errMsg, "negotiation expired") {
			response.Error(c, 400, "NEGOTIATION_EXPIRED", "Negotiation has expired")
			return
		}
		if strings.Contains(errMsg, "negotiation buyer mismatch") {
			response.Forbidden(c, "You are not the buyer of this negotiation")
			return
		}
		if strings.Contains(errMsg, "negotiation already settled") {
			response.Error(c, 400, "NEGOTIATION_ALREADY_USED", "Negotiation has already been used to create an order")
			return
		}
		if strings.Contains(errMsg, "negotiation session not found") {
			response.NotFound(c, "Negotiation session not found")
			return
		}

		response.InternalServerError(c, "Failed to create order")
		return
	}

	response.Created(c, order)
}

func (h *OrderHandler) recoverOrderFromUsedPricingToken(
	ctx context.Context,
	tokenID uuid.UUID,
	orderID uuid.UUID,
) (*orderEntity.Order, error) {
	var recoveredOrder *orderEntity.Order
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var lookupErr error
		recoveredOrder, lookupErr = h.orderRepository.GetByID(ctx, tx, orderID)
		if lookupErr == nil && recoveredOrder != nil {
			return nil
		}

		recoveredOrder, lookupErr = h.orderRepository.GetByPricingTokenID(ctx, tx, tokenID)
		return lookupErr
	})
	if err != nil {
		return nil, err
	}
	return recoveredOrder, nil
}

// ========================================================================
// COMPLETE ORDER (Seller marks order as complete)
// ========================================================================
func (h *OrderHandler) CompleteOrder(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse order ID
	idStr := c.Param("id")
	orderID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid order ID")
		return
	}

	// Extract Idempotency-Key header
	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		response.BadRequest(c, "Idempotency-Key header required")
		return
	}

	// Get user ID from context
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Execute complete within transaction
	// All business validation happens inside OrderService.Complete() under row lock
	var updatedOrder *orderEntity.Order
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		// B4A: Complete() is the canonical "Terima Barang" path.
		// Pass idempotency key for buyer-facing request safety.
		if err := h.orderService.Complete(ctx, tx, userID, orderID, idempotencyKey); err != nil {
			return err
		}

		// Fetch updated order for response
		updatedOrder, err = h.orderRepository.GetByID(ctx, tx, orderID)
		return err
	})

	if err != nil {
		h.log.Error("Failed to complete order",
			zap.String("order_id", orderID.String()),
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)

		// Check for authorization error
		if errors.Is(err, auth.ErrBuyerRequired) {
			response.Forbidden(c, "Only buyer can complete order")
			return
		}

		// Check for invalid state error
		if strings.Contains(err.Error(), "cannot complete order with active dispute") {
			response.Error(c, 409, "DISPUTE_ACTIVE", "Cannot complete order with active dispute")
			return
		}
		if strings.Contains(err.Error(), "invalid order status transition") {
			response.Error(c, 409, "INVALID_STATE", "Order cannot be completed in current state")
			return
		}
		if strings.Contains(err.Error(), "invalid escrow status") {
			response.Error(c, 409, "INVALID_ESCROW_STATE", "Cannot complete order in current escrow state")
			return
		}

		response.InternalServerError(c, "Failed to complete order")
		return
	}

	// HARDENING: Safety check - verify EscrowStatus is consistent
	// After order completion, EscrowStatus should be "released"
	// If not, log warning but return value anyway (resilience over correctness)
	if updatedOrder.EscrowStatus != orderEntity.EscrowStatusReleased {
		h.log.Warn("Order completed but EscrowStatus is not 'released' - possible staleness",
			zap.String("order_id", updatedOrder.ID.String()),
			zap.String("status", string(updatedOrder.Status)),
			zap.String("escrow_status", string(updatedOrder.EscrowStatus)),
			zap.String("expected_escrow_status", string(orderEntity.EscrowStatusReleased)),
		)
	}

	// Return updated order with relevant fields
	// NOTE: EscrowStatus is cached from Wallet state
	response.Success(c, gin.H{
		"id":            updatedOrder.ID,
		"status":        updatedOrder.Status,
		"escrow_status": updatedOrder.EscrowStatus,
		"completed_at":  updatedOrder.CompletedAt,
	})
}

// ExtendConfirmationRequest holds the request body for extend confirmation.
type ExtendConfirmationRequest struct {
	// No body required for extend confirmation - idempotency is handled via header
}

// ExtendConfirmation handles POST /api/v1/orders/{id}/extend-confirmation
//
// Authorization:
// - Only buyer can extend confirmation
// - Order status must be 'shipped'
// - Order must not have dispute
// - Extension must not have been used before
// - auto_release_at must be in the future
//
// Action:
// - Extends auto_release_at by 3 days
// - Sets confirmation_extension_used = true
// - Sets confirmation_extended_at = now()
//
// Supports Idempotency-Key header for safe retries.
// Idempotency is handled in the service layer within the same transaction.
func (h *OrderHandler) ExtendConfirmation(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse order ID
	idStr := c.Param("id")
	orderID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid order ID")
		return
	}

	// Extract Idempotency-Key header
	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		response.BadRequest(c, "Idempotency-Key header required")
		return
	}

	// Get user ID from context
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Execute extend confirmation within transaction
	// Idempotency check is performed inside the service method
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.orderService.ExtendConfirmation(ctx, tx, userID, orderID, idempotencyKey)
	})

	if err != nil {
		errMsg := err.Error()

		// HARDENING LOGS: Log specific rejection reasons for debugging
		if strings.Contains(errMsg, "extension only allowed near expiry") {
			// ESCROW HARDENING: Extension rejected (too early)
			h.log.Warn("Extend confirmation rejected (too early)",
				zap.String("order_id", orderID.String()),
				zap.String("user_id", userID.String()),
				zap.String("reason", "extension_only_allowed_last_24h"),
				zap.Error(err),
			)
		} else if strings.Contains(errMsg, "cannot extend expired order") {
			// ESCROW HARDENING: Extension rejected (expired)
			h.log.Warn("Extend confirmation rejected (expired)",
				zap.String("order_id", orderID.String()),
				zap.String("user_id", userID.String()),
				zap.String("reason", "order_already_expired"),
				zap.Error(err),
			)
		} else {
			h.log.Error("Failed to extend confirmation period",
				zap.String("order_id", orderID.String()),
				zap.String("user_id", userID.String()),
				zap.Error(err),
			)
		}

		// Check for authorization error
		if errors.Is(err, auth.ErrBuyerRequired) {
			response.Forbidden(c, "Only buyer can extend confirmation period")
			return
		}

		// Check for invalid state error
		if strings.Contains(errMsg, "cannot be extended") || strings.Contains(errMsg, "already used") {
			response.Error(c, 409, "INVALID_STATE", "Confirmation period cannot be extended in current state")
			return
		}
		if strings.Contains(errMsg, "has already been used") {
			response.Error(c, 409, "EXTENSION_ALREADY_USED", "Confirmation extension has already been used")
			return
		}

		// Check for timing restriction errors (HARDENING)
		if strings.Contains(errMsg, "extension only allowed near expiry") {
			response.Error(c, 400, "TOO_EARLY", "Extension only allowed within 24 hours of expiry")
			return
		}
		if strings.Contains(errMsg, "cannot extend expired order") {
			response.Error(c, 400, "EXPIRED", "Cannot extend expired order")
			return
		}

		response.InternalServerError(c, "Failed to extend confirmation period")
		return
	}

	response.SuccessWithMessage(c, "Confirmation period extended successfully", gin.H{
		"order_id":                    orderID,
		"extension_days":              3,
		"confirmation_extension_used": true,
	})
}

// CancelOrder handles POST /api/v1/orders/{id}/cancel
//
// Authorization: Only buyer can cancel. Cannot cancel after shipped.
// State: pending_payment → cancelled (only valid transition via entity.Cancel()).
// Requires Idempotency-Key header.
func (h *OrderHandler) CancelOrder(c *gin.Context) {
	ctx := c.Request.Context()

	idStr := c.Param("id")
	orderID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid order ID")
		return
	}

	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		response.BadRequest(c, "Idempotency-Key header required")
		return
	}

	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	var updatedOrder *orderEntity.Order
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		// Fetch order to determine correct cancel path.
		// Paid + overdue → CancelOverdue (gateway refund + escrow flip).
		// Otherwise → Cancel (pre-payment cancellation, no money movement).
		order, fetchErr := h.orderRepository.GetByID(ctx, tx, orderID)
		if fetchErr != nil {
			return fetchErr
		}

		if order.Status == orderEntity.StatusPaid && order.IsBuyerEligibleForCancel() {
			if err := h.orderService.CancelOverdue(ctx, tx, orderID, idempotencyKey, userID); err != nil {
				return err
			}
		} else {
			if err := h.orderService.Cancel(ctx, tx, orderID, idempotencyKey, userID); err != nil {
				return err
			}
		}

		updatedOrder, err = h.orderRepository.GetByID(ctx, tx, orderID)
		return err
	})

	if err != nil {
		h.log.Error("Failed to cancel order",
			zap.String("order_id", orderID.String()),
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		errMsg := err.Error()
		if errors.Is(err, auth.ErrBuyerRequired) {
			response.Forbidden(c, "Only buyer can cancel order")
			return
		}
		if strings.Contains(errMsg, "buyer not eligible for cancel") {
			response.Error(c, 409, "NOT_ELIGIBLE", "Order is not yet eligible for overdue cancellation")
			return
		}
		if strings.Contains(errMsg, "cannot cancel order: order already shipped") {
			response.Error(c, 409, "ALREADY_SHIPPED", "Cannot cancel order that has already been shipped")
			return
		}
		if strings.Contains(errMsg, "invalid order status transition") {
			response.Error(c, 409, "INVALID_STATE", "Order cannot be cancelled in current state")
			return
		}
		response.InternalServerError(c, "Failed to cancel order")
		return
	}

	response.Success(c, gin.H{
		"id":           updatedOrder.ID,
		"status":       updatedOrder.Status,
		"cancelled_at": updatedOrder.UpdatedAt,
	})
}

// buildPricingSnapshotFromToken converts a validated PricingToken to a PricingSnapshot.
//
// This helper function extracts pricing data from the validated pricing token
// and converts it to the format expected by the order creation service.
//
// CRITICAL: The token is the SINGLE SOURCE OF TRUTH for all pricing data.
// No frontend values are used in pricing calculations.
func buildPricingSnapshotFromToken(token *pricingtokenentity.PricingToken) *orderApp.PricingSnapshot {
	// Determine shipping source
	var shippingSource *string
	if token.ShippingQuoteID != nil {
		source := "shipping_quote"
		shippingSource = &source
	} else {
		source := "for_sale"
		shippingSource = &source
	}

	// Parse address snapshot from JSONB
	var addressSnapshot *addressentity.AddressSnapshot
	if len(token.AddressSnapshot) > 0 {
		var snapshot addressentity.AddressSnapshot
		if err := json.Unmarshal(token.AddressSnapshot, &snapshot); err == nil {
			addressSnapshot = &snapshot
		}
	}

	return &orderApp.PricingSnapshot{
		UnitPrice:              token.UnitPrice,
		Subtotal:               token.Subtotal,
		ShippingTotal:          token.ShippingTotal,
		CommissionPercent:      token.CommissionPercent,
		CommissionAmount:       token.CommissionAmount,
		EscrowAmount:           token.EscrowAmount,
		ServiceFeeAmount:       token.ServiceFeeAmount,
		TotalPayableAmount:     token.TotalPayableAmount,
		DiscountAmount:         token.DiscountAmount,
		MaxCoinsAllowed:        token.MaxCoinsAllowed,
		CoinsUsed:              token.CoinsUsed,
		OrderValueForCoins:     token.OrderValueForCoins,
		ShippingOptionName:     token.ShippingOptionName,
		ShippingTransportType:  token.ShippingTransportType,
		ShippingExpeditionName: token.ShippingExpeditionName,
		ShippingEstimatedDays:  token.ShippingEstimatedDays,
		ShippingDestination:    addressSnapshot,
		ShippingSource:         shippingSource,
		ShippingQuoteID:        token.ShippingQuoteID,
		ChatID:                 nil, // Set during chat checkout if needed
		AuctionID:              token.AuctionID,
		TokenID:                token.Token, // Store token ID to prevent double-ordering
		PaymentMethod:          "default",   // TODO: Add payment method to token
	}
}
