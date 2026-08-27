package http

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	shippingApp "github.com/labuda/backend/internal/commerce/shipping/application"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// ProductShippingHandler handles HTTP requests for product-shipping operations.
type ProductShippingHandler struct {
	productShippingService *shippingApp.ProductShippingService
	db                     *db.DB
	log                    *zap.Logger
}

// NewProductShippingHandler creates a new ProductShippingHandler.
func NewProductShippingHandler(
	productShippingService *shippingApp.ProductShippingService,
	database *db.DB,
	log *zap.Logger,
) *ProductShippingHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &ProductShippingHandler{
		productShippingService: productShippingService,
		db:                     database,
		log:                    log,
	}
}

// SetProductShippingOptionsRequest holds the request body for setting shipping options.
type SetProductShippingOptionsRequest struct {
	ShippingOptionIDs []string `json:"shipping_option_ids" binding:"required"`
}

// SetProductShippingOptions handles PUT /api/v1/products/:id/shipping.
//
// Sets shipping options for a product.
//
// Request body:
// - shipping_option_ids: Array of shipping option IDs (empty = clear all)
//
// Behavior:
// 1. Parse product ID from path
// 2. Validate input (non-empty array allowed, empty means clear)
// 3. Call service to set options (overwrite model)
// 4. Return success response
func (h *ProductShippingHandler) SetProductShippingOptions(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse product ID from path
	productID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid product ID")
		return
	}

	// Get user ID from context (seller)
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	sellerID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Parse request body
	var req SetProductShippingOptionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}

	// Parse shipping option IDs
	shippingOptionIDs := make([]uuid.UUID, 0, len(req.ShippingOptionIDs))
	for _, idStr := range req.ShippingOptionIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			response.BadRequest(c, "Invalid shipping_option_id format: "+idStr)
			return
		}
		shippingOptionIDs = append(shippingOptionIDs, id)
	}

	// Execute within transaction
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.productShippingService.SetProductShippingOptions(
			ctx,
			tx,
			shippingApp.SetProductShippingOptionsInput{
				ProductID:         productID,
				ShippingOptionIDs: shippingOptionIDs,
				SellerID:          sellerID,
			},
		)
	})

	if err != nil {
		h.log.Error("Failed to set product shipping options",
			zap.String("product_id", productID.String()),
			zap.String("seller_id", sellerID.String()),
			zap.Error(err),
		)

		errMsg := err.Error()
		if contains(errMsg, "not found") {
			response.NotFound(c, errMsg)
			return
		}
		if contains(errMsg, "forbidden") {
			response.Forbidden(c, errMsg)
			return
		}
		response.InternalServerError(c, "Failed to set shipping options")
		return
	}

	response.SuccessWithMessage(c, "Shipping options updated successfully", gin.H{
		"product_id":          productID.String(),
		"shipping_option_ids": req.ShippingOptionIDs,
		"count":               len(shippingOptionIDs),
	})
}

// contains checks if a string contains a substring (case-insensitive).
// Shared across seller_shipping_handler.go and this handler for HTTP-layer error-string classification.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsIgnoreCase(s, substr))
}

func containsIgnoreCase(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
