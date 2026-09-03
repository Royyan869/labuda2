package http

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	shippingApp "github.com/labuda/backend/internal/commerce/shipping/application"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// ShippingHandler handles HTTP requests for shipping operations.
type ShippingHandler struct {
	shippingService *shippingApp.ShippingService
	db              *db.DB
	log             *zap.Logger
}

// NewShippingHandler creates a new ShippingHandler.
func NewShippingHandler(
	shippingService *shippingApp.ShippingService,
	database *db.DB,
	log *zap.Logger,
) *ShippingHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &ShippingHandler{
		shippingService: shippingService,
		db:              database,
		log:             log,
	}
}

// GetDeliveryOptionsRequest holds query parameters for checking delivery availability.
type GetDeliveryOptionsRequest struct {
	ProductID    string `form:"product_id" binding:"required"`
	ProvinceCode string `form:"province_code" binding:"required"`
	CityCode     string `form:"city_code" binding:"omitempty"`
}

// CheckDeliveryRequest holds JSON body for POST /api/v1/shipping/check.
type CheckDeliveryRequest struct {
	ProductID    string `json:"product_id" binding:"required"`
	ProvinceCode string `json:"province_code" binding:"required"`
	CityCode     string `json:"city_code,omitempty"`
}

// GetDeliveryOptions handles GET /api/v1/shipping/options.
//
// Checks which delivery options are available for a product to a specific location.
//
// Query parameters:
// - product_id: The product to check shipping options for (required)
// - province_code: 2-digit BPS province code e.g., "31" for DKI Jakarta (required)
// - city_code: 4-digit BPS city code e.g., "3171" for Jakarta Selatan (optional)
func (h *ShippingHandler) GetDeliveryOptions(c *gin.Context) {
	ctx := c.Request.Context()

	var req GetDeliveryOptionsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}

	productID, err := uuid.Parse(req.ProductID)
	if err != nil {
		response.BadRequest(c, "Invalid product_id format")
		return
	}

	var options []shippingApp.DeliveryOption
	var productConfigured bool
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var innerErr error
		options, innerErr = h.shippingService.CheckDeliveryAvailability(
			ctx,
			tx,
			shippingApp.CheckDeliveryAvailabilityInput{
				ProductID:    productID,
				ProvinceCode: req.ProvinceCode,
				CityCode:     req.CityCode,
			},
		)
		if innerErr != nil {
			return innerErr
		}
		productConfigured, innerErr = h.shippingService.HasAnyShippingSetupsForProduct(ctx, tx, productID)
		return innerErr
	})

	if err != nil {
		h.log.Error("Failed to check delivery availability",
			zap.String("product_id", productID.String()),
			zap.String("province_code", req.ProvinceCode),
			zap.String("city_code", req.CityCode),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to check delivery availability")
		return
	}

	optionResponses := make([]map[string]interface{}, len(options))
	for i, opt := range options {
		optionResponses[i] = deliveryOptionToResponse(opt)
	}

	response.Success(c, gin.H{
		"product_id":         productID.String(),
		"province":           req.ProvinceCode,
		"city":               req.CityCode,
		"options":            optionResponses,
		"count":              len(options),
		"product_configured": productConfigured,
	})
}

// deliveryOptionToResponse converts a DeliveryOption to API response format.
func deliveryOptionToResponse(opt shippingApp.DeliveryOption) map[string]interface{} {
	resp := map[string]interface{}{
		"shipping_option_id": opt.ShippingSetupID.String(),
		"name":               opt.Name,
		"transport_type":     string(opt.TransportType),
		"rate":               opt.Rate,
		"is_available":       opt.IsAvailable,
	}

	return resp
}

// CheckDelivery handles POST /api/v1/shipping/check.
//
// Flutter-compatible endpoint for checking delivery availability.
// This is equivalent to GET /api/v1/shipping/options but uses POST with JSON body.
//
// Request body:
// - product_id: The product to check shipping options for (required)
// - province_code: 2-digit BPS province code e.g., "31" for DKI Jakarta (required)
// - city_code: 4-digit BPS city code e.g., "3171" for Jakarta Selatan (optional)
func (h *ShippingHandler) CheckDelivery(c *gin.Context) {
	ctx := c.Request.Context()

	var req CheckDeliveryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	productID, err := uuid.Parse(req.ProductID)
	if err != nil {
		response.BadRequest(c, "Invalid product_id format")
		return
	}

	var options []shippingApp.DeliveryOption
	var productConfigured bool
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var innerErr error
		options, innerErr = h.shippingService.CheckDeliveryAvailability(
			ctx,
			tx,
			shippingApp.CheckDeliveryAvailabilityInput{
				ProductID:    productID,
				ProvinceCode: req.ProvinceCode,
				CityCode:     req.CityCode,
			},
		)
		if innerErr != nil {
			return innerErr
		}
		productConfigured, innerErr = h.shippingService.HasAnyShippingSetupsForProduct(ctx, tx, productID)
		return innerErr
	})

	if err != nil {
		h.log.Error("Failed to check delivery availability",
			zap.String("product_id", productID.String()),
			zap.String("province_code", req.ProvinceCode),
			zap.String("city_code", req.CityCode),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to check delivery availability")
		return
	}

	optionResponses := make([]map[string]interface{}, len(options))
	for i, opt := range options {
		optionResponses[i] = deliveryOptionToResponse(opt)
	}

	response.Success(c, gin.H{
		"product_id":         productID.String(),
		"province":           req.ProvinceCode,
		"city":               req.CityCode,
		"options":            optionResponses,
		"count":              len(options),
		"product_configured": productConfigured,
	})
}
