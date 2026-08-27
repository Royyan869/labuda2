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

// NewDiscountHandler creates a new DiscountHandler.
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
	Code                 string   `json:"code" binding:"required"`
	Type                 string   `json:"type" binding:"required,oneof=percentage flat_amount free_shipping"`
	Value                string   `json:"value" binding:"required"`
	MinPurchase          string   `json:"min_purchase"`
	MaxDiscount          *string  `json:"max_discount"`
	AppliesTo            string   `json:"applies_to" binding:"required,oneof=for_sale auction both"`
	TargetMode           string   `json:"target_mode" binding:"required,oneof=seller_wide selected_items"`
	SellerID             *string  `json:"seller_id"`
	ApplicableForSaleIDs []string `json:"applicable_for_sale_ids"`
	ApplicableAuctionIDs []string `json:"applicable_auction_ids"`
	ValidFrom            string   `json:"valid_from" binding:"required"`
	ValidUntil           string   `json:"valid_until" binding:"required"`
	MaxUsagePerUser      int      `json:"max_usage_per_user"`
	TotalUsageLimit      int      `json:"total_usage_limit"`
}

// UpdateDiscountRequest represents the request body for updating a discount.
type UpdateDiscountRequest struct {
	Code                 string   `json:"code" binding:"required"`
	Type                 string   `json:"type" binding:"required,oneof=percentage flat_amount free_shipping"`
	Value                string   `json:"value" binding:"required"`
	MinPurchase          string   `json:"min_purchase"`
	MaxDiscount          *string  `json:"max_discount"`
	AppliesTo            string   `json:"applies_to" binding:"required,oneof=for_sale auction both"`
	TargetMode           string   `json:"target_mode" binding:"required,oneof=seller_wide selected_items"`
	SellerID             *string  `json:"seller_id"`
	ApplicableForSaleIDs []string `json:"applicable_for_sale_ids"`
	ApplicableAuctionIDs []string `json:"applicable_auction_ids"`
	ValidFrom            string   `json:"valid_from" binding:"required"`
	ValidUntil           string   `json:"valid_until" binding:"required"`
	MaxUsagePerUser      int      `json:"max_usage_per_user"`
	TotalUsageLimit      int      `json:"total_usage_limit"`
	IsActive             bool     `json:"is_active"`
}

// ValidateDiscountRequest represents the request body for validating a discount.
type ValidateDiscountRequest struct {
	Code        string `json:"code" binding:"required"`
	Subtotal    int64  `json:"subtotal" binding:"required,min=0"`
	ContextType string `json:"context_type" binding:"required,oneof=for_sale auction"`
	SellerID    string `json:"seller_id" binding:"required"`
	ForSaleID   string `json:"for_sale_id"`
	AuctionID   string `json:"auction_id"`
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
	var forSaleIDPtr *uuid.UUID
	if req.ForSaleID != "" {
		forSaleID, err := uuid.Parse(req.ForSaleID)
		if err != nil {
			response.BadRequest(c, "Invalid for_sale_id")
			return
		}
		forSaleIDPtr = &forSaleID
	}
	var auctionIDPtr *uuid.UUID
	if req.AuctionID != "" {
		auctionID, err := uuid.Parse(req.AuctionID)
		if err != nil {
			response.BadRequest(c, "Invalid auction_id")
			return
		}
		auctionIDPtr = &auctionID
	}

	var result *discountApp.ValidateDiscountResult
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		result, err = h.discountService.ValidateDiscount(ctx, tx, discountApp.ValidateDiscountInput{
			UserID:      userID,
			Code:        req.Code,
			Subtotal:    req.Subtotal,
			ContextType: contextType,
			SellerID:    &sellerID,
			ForSaleID:   forSaleIDPtr,
			AuctionID:   auctionIDPtr,
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

		switch e := result.ValidationError.(type) {
		case *discountEntity.DiscountNotActiveError:
			statusCode = 400
			errorCode = "DISCOUNT_INACTIVE"
		case *discountEntity.DiscountExpiredError:
			statusCode = 400
			errorCode = "DISCOUNT_EXPIRED"
		case *discountEntity.DiscountUsageLimitExceededError:
			statusCode = 400
			if e.IsUserLimit {
				errorCode = "USER_USAGE_LIMIT_EXCEEDED"
			} else {
				errorCode = "TOTAL_USAGE_LIMIT_EXCEEDED"
			}
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
		"valid":            true,
		"discount":         result.Discount,
		"discount_amount":  result.Discount.CalculateDiscountAmount(decimal.NewFromInt(req.Subtotal)).InexactFloat64(),
		"user_usage_count": result.UserUsageCount,
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
	minPurchase, _ := decimal.NewFromString(req.MinPurchase)
	var maxDiscount *decimal.Decimal
	if req.MaxDiscount != nil && *req.MaxDiscount != "" {
		md, err := decimal.NewFromString(*req.MaxDiscount)
		if err != nil {
			response.BadRequest(c, "Invalid max_discount: must be a valid decimal")
			return
		}
		maxDiscount = &md
	}

	validFrom, err := time.Parse(time.RFC3339, req.ValidFrom)
	if err != nil {
		response.BadRequest(c, "Invalid valid_from: must be ISO8601 timestamp")
		return
	}
	validUntil, err := time.Parse(time.RFC3339, req.ValidUntil)
	if err != nil {
		response.BadRequest(c, "Invalid valid_until: must be ISO8601 timestamp")
		return
	}

	appliesTo := discountEntity.DiscountAppliesTo(req.AppliesTo)
	targetMode := discountEntity.DiscountTargetMode(req.TargetMode)

	var sellerID *uuid.UUID
	if req.SellerID != nil && *req.SellerID != "" {
		parsed, err := uuid.Parse(*req.SellerID)
		if err != nil {
			response.BadRequest(c, "Invalid seller_id")
			return
		}
		sellerID = &parsed
		if *sellerID != userID {
			response.Forbidden(c, "You can only create discounts for yourself")
			return
		}
	} else {
		sellerID = &userID
	}

	if req.AppliesTo == "for_sale" && len(req.ApplicableAuctionIDs) > 0 {
		response.BadRequest(c, "applicable_auction_ids are not allowed for forSale discounts")
		return
	}
	if req.AppliesTo == "auction" && len(req.ApplicableForSaleIDs) > 0 {
		response.BadRequest(c, "applicable_for_sale_ids are not allowed for auction discounts")
		return
	}

	if req.AppliesTo == "" {
		response.BadRequest(c, "applies_to is required")
		return
	}

	if req.TargetMode == "selected_items" && len(req.ApplicableForSaleIDs)+len(req.ApplicableAuctionIDs) == 0 {
		response.BadRequest(c, "applicable_for_sale_ids or applicable_auction_ids are required for selected_items")
		return
	}

	if req.TargetMode == "seller_wide" && (len(req.ApplicableForSaleIDs) > 0 || len(req.ApplicableAuctionIDs) > 0) {
		response.BadRequest(c, "target lists must be empty for seller_wide discounts")
		return
	}

	if req.AppliesTo == "for_sale" && len(req.ApplicableAuctionIDs) > 0 {
		response.BadRequest(c, "applicable_auction_ids are not allowed for forSale discounts")
		return
	}
	if req.AppliesTo == "auction" && len(req.ApplicableForSaleIDs) > 0 {
		response.BadRequest(c, "applicable_for_sale_ids are not allowed for auction discounts")
		return
	}

	if req.TargetMode == "selected_items" {
		if len(req.ApplicableForSaleIDs) == 0 && len(req.ApplicableAuctionIDs) == 0 {
			response.BadRequest(c, "selected_items discounts require at least one target")
			return
		}
	}

	if req.TargetMode == "seller_wide" && (len(req.ApplicableForSaleIDs) > 0 || len(req.ApplicableAuctionIDs) > 0) {
		response.BadRequest(c, "seller_wide discounts cannot include selected targets")
		return
	}

	if req.AppliesTo == "for_sale" && len(req.ApplicableForSaleIDs) == 0 && req.TargetMode == "selected_items" {
		response.BadRequest(c, "applicable_for_sale_ids are required for forSale selected_items discounts")
		return
	}
	if req.AppliesTo == "auction" && len(req.ApplicableAuctionIDs) == 0 && req.TargetMode == "selected_items" {
		response.BadRequest(c, "applicable_auction_ids are required for auction selected_items discounts")
		return
	}

	if req.AppliesTo == "both" && req.TargetMode == "selected_items" && len(req.ApplicableForSaleIDs)+len(req.ApplicableAuctionIDs) == 0 {
		response.BadRequest(c, "selected_items discounts require forSale or auction targets")
		return
	}

	// MARKET AUTHORITY ENFORCEMENT:
	// Seller-owned discounts require active seller capability.
	hasCapability, err := h.roleChecker.HasActiveSellerCapability(ctx, *sellerID)
	if err != nil {
		h.log.Error("Failed to verify seller market authority", zap.String("seller_id", sellerID.String()), zap.Error(err))
		response.InternalServerError(c, "Failed to verify seller authority")
		return
	}
	if !hasCapability {
		response.Forbidden(c, "Active seller subscription required to create discounts")
		return
	}

	var forSaleIDs []uuid.UUID
	for _, forSaleIDStr := range req.ApplicableForSaleIDs {
		forSaleID, err := uuid.Parse(forSaleIDStr)
		if err != nil {
			response.BadRequest(c, "Invalid for_sale_id in applicable_for_sale_ids")
			return
		}
		forSaleIDs = append(forSaleIDs, forSaleID)
	}

	var auctionIDs []uuid.UUID
	for _, auctionIDStr := range req.ApplicableAuctionIDs {
		auctionID, err := uuid.Parse(auctionIDStr)
		if err != nil {
			response.BadRequest(c, "Invalid auction_id in applicable_auction_ids")
			return
		}
		auctionIDs = append(auctionIDs, auctionID)
	}

	var discount *discountEntity.Discount
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		discount, err = h.discountService.CreateDiscount(ctx, tx, discountApp.CreateDiscountInput{
			Code:            req.Code,
			Type:            discountEntity.DiscountType(req.Type),
			Value:           value,
			MinPurchase:     minPurchase,
			MaxDiscount:     maxDiscount,
			AppliesTo:       appliesTo,
			TargetMode:      targetMode,
			SellerID:        sellerID,
			ForSaleIDs:      forSaleIDs,
			AuctionIDs:      auctionIDs,
			ValidFrom:       validFrom,
			ValidUntil:      validUntil,
			MaxUsagePerUser: req.MaxUsagePerUser,
			TotalUsageLimit: req.TotalUsageLimit,
		})
		return err
	})
	if err != nil {
		h.log.Error("Failed to create discount", zap.String("user_id", userID.String()), zap.Error(err))
		if strings.Contains(err.Error(), "invalid discount type") || strings.Contains(err.Error(), "invalid discount applies_to") || strings.Contains(err.Error(), "invalid discount target_mode") {
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
	minPurchase, _ := decimal.NewFromString(req.MinPurchase)
	var maxDiscount *decimal.Decimal
	if req.MaxDiscount != nil && *req.MaxDiscount != "" {
		md, err := decimal.NewFromString(*req.MaxDiscount)
		if err != nil {
			response.BadRequest(c, "Invalid max_discount: must be a valid decimal")
			return
		}
		maxDiscount = &md
	}

	validFrom, err := time.Parse(time.RFC3339, req.ValidFrom)
	if err != nil {
		response.BadRequest(c, "Invalid valid_from: must be ISO8601 timestamp")
		return
	}
	validUntil, err := time.Parse(time.RFC3339, req.ValidUntil)
	if err != nil {
		response.BadRequest(c, "Invalid valid_until: must be ISO8601 timestamp")
		return
	}

	appliesTo := discountEntity.DiscountAppliesTo(req.AppliesTo)
	targetMode := discountEntity.DiscountTargetMode(req.TargetMode)

	var sellerID *uuid.UUID
	if req.SellerID != nil && *req.SellerID != "" {
		parsed, err := uuid.Parse(*req.SellerID)
		if err != nil {
			response.BadRequest(c, "Invalid seller_id")
			return
		}
		sellerID = &parsed
	}

	var forSaleIDs []uuid.UUID
	for _, forSaleIDStr := range req.ApplicableForSaleIDs {
		forSaleID, err := uuid.Parse(forSaleIDStr)
		if err != nil {
			response.BadRequest(c, "Invalid for_sale_id in applicable_for_sale_ids")
			return
		}
		forSaleIDs = append(forSaleIDs, forSaleID)
	}

	var auctionIDs []uuid.UUID
	for _, auctionIDStr := range req.ApplicableAuctionIDs {
		auctionID, err := uuid.Parse(auctionIDStr)
		if err != nil {
			response.BadRequest(c, "Invalid auction_id in applicable_auction_ids")
			return
		}
		auctionIDs = append(auctionIDs, auctionID)
	}

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
		if sellerID == nil {
			sellerID = existing.SellerID
		}

		discount, err = h.discountService.UpdateDiscount(ctx, tx, discountApp.UpdateDiscountInput{
			ID:              id,
			Code:            req.Code,
			Type:            discountEntity.DiscountType(req.Type),
			Value:           value,
			MinPurchase:     minPurchase,
			MaxDiscount:     maxDiscount,
			AppliesTo:       appliesTo,
			TargetMode:      targetMode,
			SellerID:        sellerID,
			ForSaleIDs:      forSaleIDs,
			AuctionIDs:      auctionIDs,
			ValidFrom:       validFrom,
			ValidUntil:      validUntil,
			MaxUsagePerUser: req.MaxUsagePerUser,
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


