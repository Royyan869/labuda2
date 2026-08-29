package http

import (
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/platform/response"
	discountApp "github.com/labuda/backend/internal/pricing/discount/application"
	discountEntity "github.com/labuda/backend/internal/pricing/discount/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// DiscountHandler handles HTTP requests for discount operations.
type DiscountHandler struct {
	discountService *discountApp.DiscountService
	db              *db.DB
	roleChecker     auth.RoleChecker
	log             *zap.Logger
}

// NewDiscountHandler creates a NewDiscountHandler.
func NewDiscountHandler(db *db.DB, roleChecker auth.RoleChecker, log *zap.Logger) *DiscountHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &DiscountHandler{
		discountService: discountApp.NewDiscountService(),
		db:              db,
		roleChecker:     roleChecker,
		log:             log,
	}
}

// CreateDiscountRequest represents the request body for creating a discount.
type CreateDiscountRequest struct {
	Code            string  `json:"code" binding:"required"`
	Type            string  `json:"type" binding:"required,oneof=percentage flat_amount"`
	Value           string  `json:"value" binding:"required"`
	MinPurchase     string  `json:"min_purchase"`
	AppliesTo       string  `json:"applies_to" binding:"required,oneof=for_sale auction both"`
	ValidUntil      string  `json:"valid_until" binding:"required"`
	TotalUsageLimit int     `json:"total_usage_limit"`
}

// UpdateDiscountRequest represents the request body for updating a discount.
type UpdateDiscountRequest struct {
	Code            string  `json:"code" binding:"required"`
	Type            string  `json:"type" binding:"required,oneof=percentage flat_amount"`
	Value           string  `json:"value" binding:"required"`
	MinPurchase     string  `json:"min_purchase"`
	AppliesTo       string  `json:"applies_to" binding:"required,oneof=for_sale auction both"`
	ValidUntil      string  `json:"valid_until" binding:"required"`
	TotalUsageLimit int     `json:"total_usage_limit"`
	IsActive        bool    `json:"is_active"`
}

// ValidateDiscountRequest represents the request body for validating a discount.
type ValidateDiscountRequest struct {
	Code        string `json:"code" binding:"required"`
	Subtotal    int64  `json:"subtotal" binding:"required,min=0"`
	ContextType string `json:"context_type" binding:"required,oneof=for_sale auction"`
	SellerID    string `json:"seller_id" binding:"required"`
}

func (h *DiscountHandler) GetDiscountByCode(c *gin.Context) {
	ctx := c.Request.Context()
	code := c.Param("code")
	if code == "" {
		response.BadRequest(c, "Code parameter is required")
		return
	}

	var discount *discountEntity.Discount
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		discount, err = h.discountService.GetDiscountByCode(ctx, tx, code)
		return err
	})
	if err != nil {
		h.log.Error("Failed to get discount by code", zap.String("code", code), zap.Error(err))
		response.NotFound(c, "Discount not found")
		return
	}

	response.Success(c, discount)
}

func (h *DiscountHandler) ValidateDiscount(c *gin.Context) {
	ctx := c.Request.Context()

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

	var req ValidateDiscountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	sellerID, err := uuid.Parse(req.SellerID)
	if err != nil {
		response.BadRequest(c, "Invalid seller_id")
		return
	}

	contextType := discountEntity.DiscountContextType(req.ContextType)

	var result *discountApp.ValidateDiscountResult
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		result, err = h.discountService.ValidateDiscount(ctx, tx, discountApp.ValidateDiscountInput{
			UserID:      userID,
			Code:        req.Code,
			Subtotal:    req.Subtotal,
			ContextType: contextType,
			SellerID:    &sellerID,
		})
		return err
	})
	if err != nil {
		h.log.Error("Failed to validate discount", zap.String("user_id", userID.String()), zap.String("code", req.Code), zap.Error(err))
		response.InternalServerError(c, "Failed to validate discount")
		return
	}

	if !result.Valid {
		var statusCode int
		var errorCode string

		switch result.ValidationError.(type) {
		case *discountEntity.DiscountNotActiveError:
			statusCode = 400
			errorCode = "DISCOUNT_INACTIVE"
		case *discountEntity.DiscountExpiredError:
			statusCode = 400
			errorCode = "DISCOUNT_EXPIRED"
		case *discountEntity.DiscountUsageLimitExceededError:
			statusCode = 400
			errorCode = "TOTAL_USAGE_LIMIT_EXCEEDED"
		case *discountEntity.MinPurchaseNotMetError:
			statusCode = 400
			errorCode = "MIN_PURCHASE_NOT_MET"
		default:
			statusCode = 400
			errorCode = "DISCOUNT_INVALID"
		}

		response.ErrorWithDetails(c, statusCode, errorCode, result.ValidationError.Error(), []string{result.ValidationError.Error()})
		return
	}

	response.Success(c, gin.H{
		"valid":           true,
		"discount":        result.Discount,
		"discount_amount": result.Discount.CalculateDiscountAmount(decimal.NewFromInt(req.Subtotal)).InexactFloat64(),
	})
}

func (h *DiscountHandler) CreateDiscount(c *gin.Context) {
	ctx := c.Request.Context()

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

	var req CreateDiscountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	value, err := decimal.NewFromString(req.Value)
	if err != nil {
		response.BadRequest(c, "Invalid value: must be a valid decimal")
		return
	}

	minPurchase := decimal.Zero
	if req.MinPurchase != "" {
		minPurchase, err = decimal.NewFromString(req.MinPurchase)
		if err != nil {
			response.BadRequest(c, "Invalid min_purchase: must be a valid decimal")
			return
		}
	}

	validUntil, err := time.Parse(time.RFC3339, req.ValidUntil)
	if err != nil {
		response.BadRequest(c, "Invalid valid_until: must be ISO8601 timestamp")
		return
	}

	appliesTo := discountEntity.DiscountAppliesTo(req.AppliesTo)
	sellerID := userID

	// MARKET AUTHORITY ENFORCEMENT:
	// Seller-owned discounts require active seller capability.
	hasCapability, err := h.roleChecker.HasActiveSellerCapability(ctx, sellerID)
	if err != nil {
		h.log.Error("Failed to verify seller market authority", zap.String("seller_id", sellerID.String()), zap.Error(err))
		response.InternalServerError(c, "Failed to verify seller authority")
		return
	}
	if !hasCapability {
		response.Forbidden(c, "Active seller subscription required to create discounts")
		return
	}

	var discount *discountEntity.Discount
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		discount, err = h.discountService.CreateDiscount(ctx, tx, discountApp.CreateDiscountInput{
			Code:            req.Code,
			Type:            discountEntity.DiscountType(req.Type),
			Value:           value,
			MinPurchase:     minPurchase,
			AppliesTo:       appliesTo,
			SellerID:        &sellerID,
			ValidUntil:      validUntil,
			TotalUsageLimit: req.TotalUsageLimit,
		})
		return err
	})
	if err != nil {
		h.log.Error("Failed to create discount", zap.String("user_id", userID.String()), zap.Error(err))
		if strings.Contains(err.Error(), "invalid discount type") || strings.Contains(err.Error(), "invalid discount applies_to") {
			response.BadRequest(c, "Invalid request")
			return
		}
		response.InternalServerError(c, "Failed to create discount")
		return
	}

	response.Created(c, discount)
}

func (h *DiscountHandler) UpdateDiscount(c *gin.Context) {
	ctx := c.Request.Context()

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

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid discount ID")
		return
	}

	var req UpdateDiscountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	value, err := decimal.NewFromString(req.Value)
	if err != nil {
		response.BadRequest(c, "Invalid value: must be a valid decimal")
		return
	}

	minPurchase := decimal.Zero
	if req.MinPurchase != "" {
		minPurchase, err = decimal.NewFromString(req.MinPurchase)
		if err != nil {
			response.BadRequest(c, "Invalid min_purchase: must be a valid decimal")
			return
		}
	}

	validUntil, err := time.Parse(time.RFC3339, req.ValidUntil)
	if err != nil {
		response.BadRequest(c, "Invalid valid_until: must be ISO8601 timestamp")
		return
	}

	appliesTo := discountEntity.DiscountAppliesTo(req.AppliesTo)
	sellerID := userID

	var discount *discountEntity.Discount
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		existing, err := h.discountService.GetDiscountByID(ctx, tx, id)
		if err != nil {
			return err
		}
		if existing == nil {
			return errors.New("discount not found")
		}
		if existing.SellerID == nil || *existing.SellerID != userID {
			return errors.New("forbidden: you can only update your own discounts")
		}

		discount, err = h.discountService.UpdateDiscount(ctx, tx, discountApp.UpdateDiscountInput{
			ID:              id,
			Code:            req.Code,
			Type:            discountEntity.DiscountType(req.Type),
			Value:           value,
			MinPurchase:     minPurchase,
			AppliesTo:       appliesTo,
			SellerID:        &sellerID,
			ValidUntil:      validUntil,
			TotalUsageLimit: req.TotalUsageLimit,
			IsActive:        req.IsActive,
		})
		return err
	})
	if err != nil {
		h.log.Error("Failed to update discount", zap.String("user_id", userID.String()), zap.String("discount_id", id.String()), zap.Error(err))
		if strings.Contains(err.Error(), "forbidden") {
			response.Forbidden(c, "Access denied")
			return
		}
		if strings.Contains(err.Error(), "not found") {
			response.NotFound(c, "Discount not found")
			return
		}
		response.InternalServerError(c, "Failed to update discount")
		return
	}

	response.Success(c, discount)
}

func (h *DiscountHandler) DeactivateDiscount(c *gin.Context) {
	ctx := c.Request.Context()

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

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid discount ID")
		return
	}

	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		existing, err := h.discountService.GetDiscountByID(ctx, tx, id)
		if err != nil {
			return err
		}
		if existing == nil {
			return errors.New("discount not found")
		}
		if existing.SellerID == nil || *existing.SellerID != userID {
			return errors.New("forbidden: you can only deactivate your own discounts")
		}
		return h.discountService.DeactivateDiscount(ctx, tx, id)
	})
	if err != nil {
		h.log.Error("Failed to deactivate discount", zap.String("user_id", userID.String()), zap.String("discount_id", id.String()), zap.Error(err))
		if strings.Contains(err.Error(), "forbidden") {
			response.Forbidden(c, "Access denied")
			return
		}
		if strings.Contains(err.Error(), "not found") {
			response.NotFound(c, "Discount not found")
			return
		}
		response.InternalServerError(c, "Failed to deactivate discount")
		return
	}

	response.SuccessWithMessage(c, "Discount deactivated successfully", nil)
}

func (h *DiscountHandler) GetDiscountsBySeller(c *gin.Context) {
	ctx := c.Request.Context()

	sellerIDStr := c.Param("sellerId")
	sellerID, err := uuid.Parse(sellerIDStr)
	if err != nil {
		response.BadRequest(c, "Invalid seller ID")
		return
	}

	var discounts []*discountEntity.Discount
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		discounts, err = h.discountService.GetDiscountsBySeller(ctx, tx, sellerID)
		return err
	})
	if err != nil {
		h.log.Error("Failed to get seller discounts", zap.String("seller_id", sellerID.String()), zap.Error(err))
		response.InternalServerError(c, "Failed to retrieve discounts")
		return
	}

	response.Success(c, gin.H{
		"discounts": discounts,
		"count":     len(discounts),
	})
}

func (h *DiscountHandler) ListActiveDiscounts(c *gin.Context) {
	ctx := c.Request.Context()

	var discounts []*discountEntity.Discount
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		discounts, err = h.discountService.ListActiveDiscounts(ctx, tx)
		return err
	})
	if err != nil {
		h.log.Error("Failed to list active discounts", zap.Error(err))
		response.InternalServerError(c, "Failed to retrieve active discounts")
		return
	}

	response.Success(c, gin.H{
		"discounts": discounts,
		"count":     len(discounts),
	})
}
