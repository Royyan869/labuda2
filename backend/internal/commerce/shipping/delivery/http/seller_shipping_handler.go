package http

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	sellerShippingApp "github.com/labuda/backend/internal/commerce/shipping/application"
	shippingEntity "github.com/labuda/backend/internal/commerce/shipping/entity"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// SellerShippingHandler handles HTTP requests for seller shipping option management.
type SellerShippingHandler struct {
	sellerShippingService *sellerShippingApp.SellerShippingService
	db                     *db.DB
	log                    *zap.Logger
}

// NewSellerShippingHandler creates a new SellerShippingHandler.
func NewSellerShippingHandler(
	sellerShippingService *sellerShippingApp.SellerShippingService,
	database *db.DB,
	log *zap.Logger,
) *SellerShippingHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &SellerShippingHandler{
		sellerShippingService: sellerShippingService,
		db:                     database,
		log:                    log,
	}
}

// ============================================================================
// Request/Response DTOs
// ============================================================================

// CreateShippingSetupRequest holds the request body for creating a shipping option.
type CreateShippingSetupRequest struct {
	Name          string `json:"name" binding:"required"`
	TransportType string `json:"transport_type" binding:"required,oneof=train bus travel plane custom"`
}

// UpdateShippingSetupRequest holds the request body for updating a shipping option.
type UpdateShippingSetupRequest struct {
	Name          string `json:"name"`
	TransportType string `json:"transport_type" binding:"omitempty,oneof=train bus travel plane custom"`
	IsActive      *bool  `json:"is_active"`
}

// CreateCoverageRequest holds the request body for creating a shipping coverage.
type CreateCoverageRequest struct {
	ProvinceCode  string `json:"province_code" binding:"required"`
	ProvinceName  string `json:"province_name" binding:"required"`
	Rate          int64  `json:"rate" binding:"required"`
	IsAvailable   bool   `json:"is_available"`
}

// UpdateCoverageRequest holds the request body for updating a shipping coverage.
type UpdateCoverageRequest struct {
	ProvinceName string  `json:"province_name"`
	Rate         *int64  `json:"rate"`
	IsAvailable  *bool   `json:"is_available"`
}

// ============================================================================
// Shipping Option Handlers
// ============================================================================

// CreateShippingSetup handles POST /api/v1/shipping/options
//
// Creates a new shipping option for the authenticated seller.
//
// Request body:
// - name: Display name for the shipping option (required)
// - transport_type: Type of transport - train, bus, travel, plane, custom (required)
func (h *SellerShippingHandler) CreateShippingSetup(c *gin.Context) {
	ctx := c.Request.Context()

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
	var req CreateShippingSetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Execute within transaction
	var option *shippingEntity.ShippingSetup
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		option, err = h.sellerShippingService.CreateShippingSetup(
				ctx,
				tx,
				sellerShippingApp.CreateShippingSetupInput{
					SellerID:      sellerID,
					Name:          req.Name,
					TransportType: shippingEntity.TransportType(req.TransportType),
				},
			)
		return err
	})

	if err != nil {
		h.log.Error("Failed to create shipping option",
			zap.String("seller_id", sellerID.String()),
			zap.Error(err),
		)

		errMsg := err.Error()
		if contains(errMsg, "already exists") {
			response.ConflictWithLog(c, h.log, "DUPLICATE_NAME", errMsg, err)
			return
		}
		if contains(errMsg, "invalid transport type") {
			response.BadRequest(c, errMsg)
			return
		}
		response.InternalServerError(c, "Failed to create shipping option")
		return
	}

	response.Created(c, gin.H{
		"shipping_option": shippingSetupToResponse(option),
	})
}

// ListShippingSetups handles GET /api/v1/shipping/options
//
// Lists all shipping options for the authenticated seller.
//
// Query parameters:
// - include_inactive: Include inactive options (default: false)
func (h *SellerShippingHandler) ListShippingSetups(c *gin.Context) {
	ctx := c.Request.Context()

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

	// Parse query parameters
	includeInactive := c.DefaultQuery("include_inactive", "false") == "true"

	// Execute within transaction
	var options []*shippingEntity.ShippingSetup
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		options, err = h.sellerShippingService.ListSellerShippingSetups(
			ctx,
			tx,
			sellerID,
			includeInactive,
		)
		return err
	})

	if err != nil {
		h.log.Error("Failed to list shipping options",
			zap.String("seller_id", sellerID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to list shipping options")
		return
	}

	// Convert to response format
	optionResponses := make([]map[string]interface{}, len(options))
	for i, opt := range options {
		optionResponses[i] = shippingSetupToResponse(opt)
	}

	response.Success(c, gin.H{
		"shipping_options": optionResponses,
		"count":            len(options),
	})
}

// GetShippingSetup handles GET /api/v1/shipping/options/:id
//
// Retrieves a shipping option with its coverages by ID.
func (h *SellerShippingHandler) GetShippingSetup(c *gin.Context) {
	ctx := c.Request.Context()

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

	// Parse shipping option ID
	shippingSetupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid shipping option ID")
		return
	}

	// Execute within transaction
	var result *sellerShippingApp.GetShippingSetupWithCoveragesResult
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		result, err = h.sellerShippingService.GetShippingSetupWithCoverages(
			ctx,
			tx,
			shippingSetupID,
			sellerID,
		)
		return err
	})

	if err != nil {
		h.log.Error("Failed to get shipping option",
			zap.String("shipping_option_id", shippingSetupID.String()),
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
		response.InternalServerError(c, "Failed to get shipping option")
		return
	}

	// Convert coverages to response format
	coverages := make([]map[string]interface{}, len(result.Coverages))
	for i, cov := range result.Coverages {
		coverages[i] = coverageToResponse(cov)
	}

	response.Success(c, gin.H{
		"shipping_option": shippingSetupToResponse(result.ShippingSetup),
		"coverages":       coverages,
		"coverage_count":  len(coverages),
	})
}

// UpdateShippingSetup handles PUT /api/v1/shipping/options/:id
//
// Updates an existing shipping option.
//
// Request body:
// - name: New display name (optional)
// - transport_type: New transport type (optional)
// - is_active: Active status (optional)
func (h *SellerShippingHandler) UpdateShippingSetup(c *gin.Context) {
	ctx := c.Request.Context()

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

	// Parse shipping option ID
	shippingSetupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid shipping option ID")
		return
	}

	// Parse request body
	var req UpdateShippingSetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Execute within transaction
	var option *shippingEntity.ShippingSetup
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		input := sellerShippingApp.UpdateShippingSetupInput{
			ShippingSetupID: shippingSetupID,
			SellerID:         sellerID,
			Name:             req.Name,
			IsActive:         req.IsActive,
		}
		if req.TransportType != "" {
			input.TransportType = shippingEntity.TransportType(req.TransportType)
		}
		option, err = h.sellerShippingService.UpdateShippingSetup(ctx, tx, input)
		return err
	})

	if err != nil {
		h.log.Error("Failed to update shipping option",
			zap.String("shipping_option_id", shippingSetupID.String()),
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
		if contains(errMsg, "already exists") {
			response.ConflictWithLog(c, h.log, "DUPLICATE_NAME", errMsg, err)
			return
		}
		response.InternalServerError(c, "Failed to update shipping option")
		return
	}

	response.SuccessWithMessage(c, "Shipping option updated successfully", gin.H{
		"shipping_option": shippingSetupToResponse(option),
	})
}

// DeleteShippingSetup handles DELETE /api/v1/shipping/options/:id
//
// Deletes a shipping option and its associated coverages.
func (h *SellerShippingHandler) DeleteShippingSetup(c *gin.Context) {
	ctx := c.Request.Context()

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

	// Parse shipping option ID
	shippingSetupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid shipping option ID")
		return
	}

	// Execute within transaction
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.sellerShippingService.DeleteShippingSetup(
			ctx,
			tx,
			shippingSetupID,
			sellerID,
		)
	})

	if err != nil {
		h.log.Error("Failed to delete shipping option",
			zap.String("shipping_option_id", shippingSetupID.String()),
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
		response.InternalServerError(c, "Failed to delete shipping option")
		return
	}

	response.SuccessWithMessage(c, "Shipping option deleted successfully", gin.H{
		"shipping_option_id": shippingSetupID.String(),
	})
}

// ============================================================================
// Coverage Handlers
// ============================================================================

// CreateCoverage handles POST /api/v1/shipping/options/:id/coverages
//
// Creates a new shipping coverage for a shipping option.
//
// Request body:
// - province_code: 2-digit BPS province code (required)
// - province_name: Province name (required)
// - rate: Shipping rate in smallest currency unit (required)
// - is_available: Whether shipping is available (default: true)
func (h *SellerShippingHandler) CreateCoverage(c *gin.Context) {
	ctx := c.Request.Context()

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

	// Parse shipping option ID
	shippingSetupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid shipping option ID")
		return
	}

	// Parse request body
	var req CreateCoverageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Execute within transaction
	var coverage *shippingEntity.ShippingCoverage
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		coverage, err = h.sellerShippingService.CreateCoverage(
				ctx,
				tx,
				sellerShippingApp.CreateCoverageInput{
					ShippingSetupID: shippingSetupID,
					SellerID:         sellerID,
					ProvinceCode:     req.ProvinceCode,
					ProvinceName:     req.ProvinceName,
					Rate:             req.Rate,
					IsAvailable:      req.IsAvailable,
				},
			)
		return err
	})

	if err != nil {
		h.log.Error("Failed to create coverage",
			zap.String("shipping_option_id", shippingSetupID.String()),
			zap.String("seller_id", sellerID.String()),
			zap.String("province_code", req.ProvinceCode),
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
		if contains(errMsg, "already exists") {
			response.ConflictWithLog(c, h.log, "DUPLICATE_COVERAGE", errMsg, err)
			return
		}
		response.InternalServerError(c, "Failed to create coverage")
		return
	}

	response.Created(c, gin.H{
		"coverage": coverageToResponse(coverage),
	})
}

// ListCoverages handles GET /api/v1/shipping/options/:id/coverages
//
// Lists all coverages for a shipping option.
func (h *SellerShippingHandler) ListCoverages(c *gin.Context) {
	ctx := c.Request.Context()

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

	// Parse shipping option ID
	shippingSetupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid shipping option ID")
		return
	}

	// Execute within transaction
	var coverages []*shippingEntity.ShippingCoverage
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		coverages, err = h.sellerShippingService.ListCoverages(
			ctx,
			tx,
			shippingSetupID,
			sellerID,
		)
		return err
	})

	if err != nil {
		h.log.Error("Failed to list coverages",
			zap.String("shipping_option_id", shippingSetupID.String()),
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
		response.InternalServerError(c, "Failed to list coverages")
		return
	}

	// Convert to response format
	coverageResponses := make([]map[string]interface{}, len(coverages))
	for i, cov := range coverages {
		coverageResponses[i] = coverageToResponse(cov)
	}

	response.Success(c, gin.H{
		"coverages": coverageResponses,
		"count":     len(coverages),
	})
}

// UpdateCoverage handles PUT /api/v1/shipping/coverages/:id
//
// Updates an existing shipping coverage.
//
// Request body:
// - province_name: New province name (optional)
// - rate: New shipping rate (optional)
// - is_available: New availability status (optional)
func (h *SellerShippingHandler) UpdateCoverage(c *gin.Context) {
	ctx := c.Request.Context()

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

	// Parse coverage ID
	coverageID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid coverage ID")
		return
	}

	// Parse request body
	var req UpdateCoverageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Execute within transaction
	var coverage *shippingEntity.ShippingCoverage
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		coverage, err = h.sellerShippingService.UpdateCoverage(
			ctx,
			tx,
			sellerShippingApp.UpdateCoverageInput{
				CoverageID:     coverageID,
				SellerID:     sellerID,
				ProvinceName: req.ProvinceName,
				Rate:         req.Rate,
				IsAvailable:  req.IsAvailable,
			},
		)
		return err
	})

	if err != nil {
		h.log.Error("Failed to update coverage",
			zap.String("coverage_id", coverageID.String()),
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
		response.InternalServerError(c, "Failed to update coverage")
		return
	}

	response.SuccessWithMessage(c, "Coverage updated successfully", gin.H{
		"coverage": coverageToResponse(coverage),
	})
}

// DeleteCoverage handles DELETE /api/v1/shipping/coverages/:id
//
// Deletes a shipping coverage.
func (h *SellerShippingHandler) DeleteCoverage(c *gin.Context) {
	ctx := c.Request.Context()

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

	// Parse coverage ID
	coverageID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid coverage ID")
		return
	}

	// Execute within transaction
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.sellerShippingService.DeleteCoverage(
			ctx,
			tx,
			coverageID,
			sellerID,
		)
	})

	if err != nil {
		h.log.Error("Failed to delete coverage",
			zap.String("coverage_id", coverageID.String()),
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
		response.InternalServerError(c, "Failed to delete coverage")
		return
	}

	response.SuccessWithMessage(c, "Coverage deleted successfully", gin.H{
		"coverage_id": coverageID.String(),
	})
}

// ============================================================================
// Response Converters
// ============================================================================

// shippingSetupToResponse converts a ShippingSetup entity to API response format.
func shippingSetupToResponse(opt *shippingEntity.ShippingSetup) map[string]interface{} {
	resp := map[string]interface{}{
		"id":             opt.ID.String(),
		"name":           opt.Name,
		"transport_type": string(opt.TransportType),
		"is_active":      opt.IsActive,
		"created_at":     opt.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"updated_at":     opt.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	return resp
}

// coverageToResponse converts a ShippingCoverage entity to API response format.
func coverageToResponse(cov *shippingEntity.ShippingCoverage) map[string]interface{} {
	resp := map[string]interface{}{
		"id":               cov.ID.String(),
		"shipping_option_id": cov.ShippingSetupID.String(),
		"province_code":    cov.ProvinceCode,
		"province_name":    cov.ProvinceName,
		"rate":             cov.ProvinceRate.Int64(),
		"is_available":     cov.IsAvailable,
		"created_at":       cov.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	return resp
}


