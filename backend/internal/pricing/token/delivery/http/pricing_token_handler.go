package http

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	platformconfigApp "github.com/labuda/backend/internal/platform/config/application"
	"github.com/labuda/backend/internal/platform/response"
	pricingtokenapp "github.com/labuda/backend/internal/pricing/token/application"
	pricingtokenentity "github.com/labuda/backend/internal/pricing/token/entity"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

type pricingTokenGenerator interface {
	GenerateForForSale(ctx context.Context, tx db.Tx, req *pricingtokenapp.GenerateForForSaleRequest) (*pricingtokenapp.GenerateForForSaleResponse, error)
	GenerateForAuction(ctx context.Context, tx db.Tx, req *pricingtokenapp.GenerateForAuctionRequest) (*pricingtokenapp.GenerateForAuctionResponse, error)
	GenerateForNegotiation(ctx context.Context, tx db.Tx, req *pricingtokenapp.GenerateForNegotiationRequest) (*pricingtokenapp.GenerateForNegotiationResponse, error)
	ValidateForOrder(ctx context.Context, tx db.Tx, req *pricingtokenapp.ValidateForOrderRequest) (*pricingtokenentity.PricingToken, error)
	GetSnapshot(ctx context.Context, tx db.Tx, token uuid.UUID) (*pricingtokenentity.PricingToken, error)
}

// PricingTokenHandler handles HTTP requests for pricing token operations.
type PricingTokenHandler struct {
	tokenService pricingTokenGenerator
	db           db.Transactor
	log          *zap.Logger
}

// NewPricingTokenHandler creates a new PricingTokenHandler.
func NewPricingTokenHandler(
	configService *platformconfigApp.ConfigService,
	db db.Transactor,
	log *zap.Logger,
) *PricingTokenHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &PricingTokenHandler{
		tokenService: pricingtokenapp.NewPricingTokenService(configService),
		db:           db,
		log:          log,
	}
}

// GeneratePreviewRequest contains the request body for generating a pricing preview token.
//
// SHIPPING SOURCE (REPLACE MODE):
// - Exactly one of shipping_option_id or shipping_quote_id must be provided
// - If shipping_quote_id is set, shipping cost comes from manual quote
// - If shipping_option_id is set, shipping cost comes from product shipping options
type GeneratePreviewRequest struct {
	ProductID        uuid.UUID  `json:"product_id" binding:"required"`
	SourceType       string     `json:"source_type" binding:"required"`
	SourceID         uuid.UUID  `json:"source_id" binding:"required"`
	NegotiationID    *uuid.UUID `json:"negotiation_id,omitempty"`
	Quantity         int        `json:"quantity" binding:"required,min=1"`
	ShippingOptionID *uuid.UUID `json:"shipping_option_id,omitempty"` // Optional: when using shipping_quote_id
	ShippingQuoteID  *uuid.UUID `json:"shipping_quote_id,omitempty"`  // Optional: when using manual quote
	AddressID        uuid.UUID  `json:"address_id" binding:"required"`
	DiscountCode     *string    `json:"discount_code,omitempty"`
}

// GeneratePreview generates a pricing preview token.
//
// POST /api/v1/pricing/preview
//
// This endpoint generates a single-use pricing token that captures the complete
// pricing snapshot at preview time. The token must be provided during order creation
// to ensure the pricing hasn't been manipulated.
//
// Request body:
// - product_id: The product to purchase
// - source_type: for_sale | auction | negotiation
// - source_id: The sale-surface UUID
// - quantity: Quantity to purchase (must be >= 1)
// - shipping_option_id: Optional, selected shipping option (required if not using shipping_quote_id)
// - shipping_quote_id: Optional, manual shipping quote ID (required if not using shipping_option_id)
// - address_id: Shipping address ID
// - discount_code: Optional promo code
//
// Response:
// - token: The pricing token UUID (must be provided during order creation)
// - expires_at: Token expiration timestamp
// - pricing_snapshot: Complete pricing breakdown (without coins - coins are applied at order layer)
func (h *PricingTokenHandler) GeneratePreview(c *gin.Context) {
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

	// Parse request body
	var req GeneratePreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		h.log.Debug("Failed to parse preview request", zap.Error(err))
		return
	}

	// Route based on source_type
	switch req.SourceType {
	case "auction":
		// Auction buy-now or bid-win preview via GenerateForAuction
		if req.ShippingOptionID == nil {
			response.BadRequest(c, "shipping_option_id is required for auction pricing preview")
			return
		}
		auctionReq := &pricingtokenapp.GenerateForAuctionRequest{
			UserID:           userID,
			AuctionID:        req.SourceID,
			AddressID:        req.AddressID,
			ShippingOptionID: *req.ShippingOptionID,
			DiscountCode:     req.DiscountCode,
		}
		var auctionResult *pricingtokenapp.GenerateForAuctionResponse
		err := h.db.WithTx(ctx, func(tx db.Tx) error {
			var err error
			auctionResult, err = h.tokenService.GenerateForAuction(ctx, tx, auctionReq)
			return err
		})
		if err != nil {
			h.log.Error("Failed to generate auction pricing token",
				zap.String("user_id", userID.String()),
				zap.String("auction_id", req.SourceID.String()),
				zap.Error(err),
			)
			response.InternalServerError(c, "Failed to generate pricing preview")
			return
		}
		response.Success(c, gin.H{
			"token":                   auctionResult.Token,
			"expires_at":              auctionResult.ExpiresAt,
			"pricing_snapshot":        auctionResult.PricingSnapshot,
			"auction_settlement_type": auctionResult.AuctionSettlementType,
		})
	case "for_sale":
		if req.NegotiationID != nil {
			if req.ShippingOptionID == nil {
				response.BadRequest(c, "shipping_option_id is required for negotiation pricing preview")
				return
			}
			negotiationReq := &pricingtokenapp.GenerateForNegotiationRequest{
				UserID:           userID,
				NegotiationID:    *req.NegotiationID,
				AddressID:        req.AddressID,
				ShippingOptionID: *req.ShippingOptionID,
				DiscountCode:     req.DiscountCode,
			}
			var negotiationResult *pricingtokenapp.GenerateForNegotiationResponse
			err := h.db.WithTx(ctx, func(tx db.Tx) error {
				var err error
				negotiationResult, err = h.tokenService.GenerateForNegotiation(ctx, tx, negotiationReq)
				return err
			})
			if err != nil {
				h.log.Error("Failed to generate negotiation pricing token",
					zap.String("user_id", userID.String()),
					zap.String("negotiation_id", req.NegotiationID.String()),
					zap.Error(err),
				)
				response.InternalServerError(c, "Failed to generate pricing preview")
				return
			}
			response.Success(c, gin.H{
				"token":            negotiationResult.Token,
				"expires_at":       negotiationResult.ExpiresAt,
				"pricing_snapshot": negotiationResult.PricingSnapshot,
			})
			return
		}

		// fixed-price-sale direct preview via GenerateForForSale
		serviceReq := &pricingtokenapp.GenerateForForSaleRequest{
			UserID:           userID,
			ProductID:        req.ProductID,
			SourceType:       req.SourceType,
			SourceID:         req.SourceID,
			Quantity:         req.Quantity,
			ShippingOptionID: req.ShippingOptionID,
			ShippingQuoteID:  req.ShippingQuoteID,
			AddressID:        req.AddressID,
			DiscountCode:     req.DiscountCode,
		}
		var result *pricingtokenapp.GenerateForForSaleResponse
		err := h.db.WithTx(ctx, func(tx db.Tx) error {
			var err error
			result, err = h.tokenService.GenerateForForSale(ctx, tx, serviceReq)
			return err
		})
		if err != nil {
			h.log.Error("Failed to generate pricing token",
				zap.String("user_id", userID.String()),
				zap.String("product_id", req.ProductID.String()),
				zap.Error(err),
			)
			response.InternalServerError(c, "Failed to generate pricing preview")
			return
		}
		response.Success(c, gin.H{
			"token":            result.Token,
			"expires_at":       result.ExpiresAt,
			"pricing_snapshot": result.PricingSnapshot,
		})

	default:
		response.BadRequest(c, "source_type must be for_sale or auction")
	}
}

// ValidateTokenRequest contains the request body for validating a pricing token.
type ValidateTokenRequest struct {
	Token            uuid.UUID  `json:"token" binding:"required"`
	ProductID        uuid.UUID  `json:"product_id" binding:"required"`
	SourceType       string     `json:"source_type" binding:"required"`
	SourceID         uuid.UUID  `json:"source_id" binding:"required"`
	Quantity         int        `json:"quantity" binding:"required,min=1"`
	ShippingOptionID *uuid.UUID `json:"shipping_option_id,omitempty"`
	AddressID        uuid.UUID  `json:"address_id" binding:"required"`
}

// ValidateToken validates a pricing token without consuming it.
//
// POST /api/v1/pricing/validate
//
// This endpoint validates a pricing token and returns the pricing snapshot
// if valid. Useful for re-displaying order details before final confirmation.
//
// Request body:
// - token: The pricing token to validate
// - product_id/source_type/source_id: Must match the token's sale surface binding
// - quantity: Must match the token's quantity
// - shipping_option_id: Must match the token's shipping_option_id
// - address_id: Must match the token's address_id
//
// Response:
// - Returns the pricing snapshot if valid
// - Returns error with validation code if invalid
func (h *PricingTokenHandler) ValidateToken(c *gin.Context) {
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

	// Parse request body
	var req ValidateTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	// Build validation request
	validateReq := &pricingtokenapp.ValidateForOrderRequest{
		Token:            req.Token,
		RequesterID:      userID,
		ProductID:        req.ProductID,
		SourceType:       req.SourceType,
		SourceID:         req.SourceID,
		Quantity:         req.Quantity,
		AddressID:        req.AddressID,
		ShippingOptionID: req.ShippingOptionID,
	}

	// Validate token within transaction
	var token *pricingtokenentity.PricingToken
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		token, err = h.tokenService.ValidateForOrder(ctx, tx, validateReq)
		return err
	})

	if err != nil {
		// Check if it's a validation error
		validationErr, isValidationErr := err.(*pricingtokenentity.ValidationError)
		if isValidationErr {
			response.BadRequest(c, validationErr.Message)
			return
		}

		h.log.Error("Failed to validate pricing token",
			zap.String("user_id", userID.String()),
			zap.String("token", req.Token.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to validate pricing token")
		return
	}

	// Return pricing snapshot
	response.Success(c, gin.H{
		"valid":            true,
		"pricing_snapshot": pricingSnapshotFromEntity(token),
		"expires_at":       token.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// GetToken retrieves a pricing token's snapshot without validation.
//
// GET /api/v1/pricing/tokens/:token
//
// This endpoint retrieves the pricing snapshot for a token without consuming it.
// Useful for displaying order details when the user returns to the checkout page.
func (h *PricingTokenHandler) GetToken(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse token UUID from URL
	tokenStr := c.Param("token")
	token, err := uuid.Parse(tokenStr)
	if err != nil {
		response.BadRequest(c, "Invalid token UUID")
		return
	}

	// Get token snapshot within transaction
	var pricingToken *pricingtokenentity.PricingToken
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		pricingToken, err = h.tokenService.GetSnapshot(ctx, tx, token)
		return err
	})

	if err != nil {
		if err == pricingtokenentity.ErrTokenNotFound {
			response.NotFound(c, "Pricing token not found")
			return
		}

		h.log.Error("Failed to get pricing token",
			zap.String("token", token.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve pricing token")
		return
	}

	// Return pricing snapshot
	response.Success(c, gin.H{
		"pricing_snapshot": pricingSnapshotFromEntity(pricingToken),
		"is_used":          pricingToken.IsUsed,
		"expires_at":       pricingToken.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// Helper function to convert entity to response format
func pricingSnapshotFromEntity(token *pricingtokenentity.PricingToken) gin.H {
	var discountType *string
	if token.DiscountType != nil {
		dt := string(*token.DiscountType)
		discountType = &dt
	}

	var discountValue *string
	if token.DiscountValue != nil {
		dv := token.DiscountValue.String()
		discountValue = &dv
	}

	return gin.H{
		"unit_price":           token.UnitPrice.Int64(),
		"quantity":             token.Quantity,
		"subtotal":             token.Subtotal.Int64(),
		"shipping_total":       token.ShippingTotal.Int64(),
		"commission_percent":   token.CommissionPercent,
		"commission_amount":    token.CommissionAmount.Int64(),
		"service_fee_amount":   token.ServiceFeeAmount.Int64(),
		"total_payable_amount": token.TotalPayableAmount.Int64(),
		"discount_amount":      token.DiscountAmount.Int64(),
		"discount_code":        token.DiscountCode,
		"discount_type":        discountType,
		"discount_value":       discountValue,
		"escrow_amount":        token.EscrowAmount.Int64(),
		"shipping_option": gin.H{
			"id":              token.ShippingOptionID,
			"name":            token.ShippingOptionName,
			"transport_type":  token.ShippingTransportType,
			"expedition_name": token.ShippingExpeditionName,
			"estimated_days":  token.ShippingEstimatedDays,
		},
		"product_id":  token.ProductID,
		"source_type": token.SourceType,
		"source_id":   token.SourceID,
		"address_id":  token.AddressID,
	}
}
