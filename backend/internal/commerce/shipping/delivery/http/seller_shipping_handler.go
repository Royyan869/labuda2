package http

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	sellerShippingApp "github.com/labuda/backend/internal/commerce/shipping/application"
	shippingEntity "github.com/labuda/backend/internal/commerce/shipping/entity"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// SellerShippingHandler handles HTTP requests for seller shipping option management.
//
// CANONICAL CONTRACT (Owner-locked):
//   - POST   /seller/shipping/options          → create ONE package (identity + destinations)
//   - GET    /seller/shipping/options          → list options
//   - GET    /seller/shipping/options/:id      → get full package (option + coverages + city qualifications)
//   - PUT    /seller/shipping/options/:id      → replace ONE package in a single transaction
//   - PATCH  /seller/shipping/options/:id/active → toggle active (canonical retire path)
//   - DELETE /seller/shipping/options/:id      → hard delete, refused while linked to any listing
//
// Killed design (must not be reintroduced): bare metadata-only creation and
// per-coverage CRUD endpoints (POST/PUT/DELETE on coverages) as seller flows.
type SellerShippingHandler struct {
	sellerShippingService *sellerShippingApp.SellerShippingService
	db                    *db.DB
	log                   *zap.Logger
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
		db:                    database,
		log:                   log,
	}
}

// ============================================================================
// Request/Response DTOs
// ============================================================================

// CityQualificationRequest is a city-level override inside a province payload.
// Pointer fields: nil = inherit the province default.
type CityQualificationRequest struct {
	CityCode    string `json:"city_code" binding:"required"`
	CityName    string `json:"city_name" binding:"required"`
	Rate        *int64 `json:"rate"`
	IsAvailable *bool  `json:"is_available"`
}

// ProvinceDestinationRequest is a destination province inside the package.
type ProvinceDestinationRequest struct {
	ProvinceCode       string                      `json:"province_code" binding:"required"`
	ProvinceName       string                      `json:"province_name" binding:"required"`
	Rate               int64                       `json:"rate" binding:"required"`
	IsAvailable        bool                        `json:"is_available"`
	CityQualifications []CityQualificationRequest  `json:"city_qualifications"`
}

// ShippingPackageRequest is the full seller-authored shipping option payload.
// The rate is ALL-IN (shipping + packing) — there is no separate packing
// field by design; the seller form communicates that via a hint, not a column.
type ShippingPackageRequest struct {
	Name            string                       `json:"name" binding:"required"`
	TransportType   string                       `json:"transport_type" binding:"required,oneof=train bus travel plane custom"`
	InternalPurpose string                       `json:"internal_purpose"`
	IsActive        *bool                        `json:"is_active"`
	Destinations    []ProvinceDestinationRequest `json:"destinations" binding:"required,min=1,dive"`
}

// UpdateShippingPackageRequest is the full-replace payload for updates.
// Destinations are optional here only so that identity-only edits do not
// force the client to resend destinations; when present they fully replace.
type UpdateShippingPackageRequest struct {
	Name            string                       `json:"name" binding:"required"`
	TransportType   string                       `json:"transport_type" binding:"required,oneof=train bus travel plane custom"`
	InternalPurpose string                       `json:"internal_purpose"`
	IsActive        *bool                        `json:"is_active"`
	Destinations    []ProvinceDestinationRequest `json:"destinations"`
}

// SetActiveRequest toggles option availability.
type SetActiveRequest struct {
	IsActive bool `json:"is_active" binding:"required"`
}

// ============================================================================
// Helpers
// ============================================================================

func (h *SellerShippingHandler) sellerIDFromContext(c *gin.Context) (uuid.UUID, bool) {
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return uuid.Nil, false
	}
	sellerID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return uuid.Nil, false
	}
	return sellerID, true
}

func (h *SellerShippingHandler) parseOptionID(c *gin.Context) (uuid.UUID, bool) {
	shippingSetupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid shipping option ID")
		return uuid.Nil, false
	}
	return shippingSetupID, true
}

func packageRequestToInput(sellerID uuid.UUID, req ShippingPackageRequest) sellerShippingApp.ShippingPackageInput {
	destinations := make([]sellerShippingApp.ProvinceDestinationInput, 0, len(req.Destinations))
	for _, dest := range req.Destinations {
		cities := make([]sellerShippingApp.CityQualificationInput, 0, len(dest.CityQualifications))
		for _, city := range dest.CityQualifications {
			cities = append(cities, sellerShippingApp.CityQualificationInput{
				CityCode:    city.CityCode,
				CityName:    city.CityName,
				Rate:        city.Rate,
				IsAvailable: city.IsAvailable,
			})
		}
		destinations = append(destinations, sellerShippingApp.ProvinceDestinationInput{
			ProvinceCode:       dest.ProvinceCode,
			ProvinceName:       dest.ProvinceName,
			Rate:               dest.Rate,
			IsAvailable:        dest.IsAvailable,
			CityQualifications: cities,
		})
	}
	return sellerShippingApp.ShippingPackageInput{
		SellerID:        sellerID,
		Name:            req.Name,
		TransportType:   shippingEntity.TransportType(req.TransportType),
		InternalPurpose: req.InternalPurpose,
		IsActive:        req.IsActive,
		Destinations:    destinations,
	}
}

// writeServiceError maps service errors to honest HTTP responses.
func (h *SellerShippingHandler) writeServiceError(c *gin.Context, logFields []zap.Field, err error, generic string) {
	h.log.Error(generic, append(logFields, zap.Error(err))...)

	switch {
	case errors.Is(err, sellerShippingApp.ErrShippingPackageIncomplete):
		response.BadRequest(c, err.Error())
	case errors.Is(err, sellerShippingApp.ErrShippingLinkedOptionUndeletable):
		response.ConflictWithLog(c, h.log, "SHIPPING_OPTION_LINKED", err.Error(), err)
	case contains(err.Error(), "already exists"):
		response.ConflictWithLog(c, h.log, "DUPLICATE_NAME", err.Error(), err)
	case contains(err.Error(), "invalid transport type"):
		response.BadRequest(c, err.Error())
	case contains(err.Error(), "not found"):
		response.NotFound(c, err.Error())
	case contains(err.Error(), "forbidden"):
		response.Forbidden(c, err.Error())
	default:
		response.InternalServerError(c, generic)
	}
}

// ============================================================================
// One-package handlers
// ============================================================================

// CreateShippingSetup handles POST /api/v1/seller/shipping/options
//
// Creates the shipping option AND all of its destinations (provinces with
// rates + city qualifications) in ONE transaction. An option without at
// least one destination-with-rate is rejected (SHIPPING_PACKAGE_INCOMPLETE).
// internal_purpose is stored seller-private and is never echoed to buyers.
func (h *SellerShippingHandler) CreateShippingSetup(c *gin.Context) {
	ctx := c.Request.Context()

	sellerID, ok := h.sellerIDFromContext(c)
	if !ok {
		return
	}

	var req ShippingPackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	var option *shippingEntity.ShippingSetup
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		option, err = h.sellerShippingService.CreateShippingPackage(
			ctx, tx, packageRequestToInput(sellerID, req),
		)
		return err
	})

	if err != nil {
		h.writeServiceError(c, []zap.Field{
			zap.String("seller_id", sellerID.String()),
		}, err, "Failed to create shipping option")
		return
	}

	response.Created(c, gin.H{
		"shipping_option": shippingSetupToResponse(option),
	})
}

// ListShippingSetups handles GET /api/v1/seller/shipping/options
func (h *SellerShippingHandler) ListShippingSetups(c *gin.Context) {
	ctx := c.Request.Context()

	sellerID, ok := h.sellerIDFromContext(c)
	if !ok {
		return
	}

	includeInactive := c.DefaultQuery("include_inactive", "false") == "true"

	var options []*shippingEntity.ShippingSetup
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		options, err = h.sellerShippingService.ListSellerShippingSetups(
			ctx, tx, sellerID, includeInactive,
		)
		return err
	})

	if err != nil {
		h.log.Error("Failed to list shipping options",
			zap.String("seller_id", sellerID.String()), zap.Error(err))
		response.InternalServerError(c, "Failed to list shipping options")
		return
	}

	optionResponses := make([]map[string]interface{}, len(options))
	for i, opt := range options {
		optionResponses[i] = shippingSetupToResponse(opt)
	}

	response.Success(c, gin.H{
		"shipping_options": optionResponses,
		"count":            len(options),
	})
}

// GetShippingSetup handles GET /api/v1/seller/shipping/options/:id
//
// Returns the full package: option + coverages + city qualifications.
func (h *SellerShippingHandler) GetShippingSetup(c *gin.Context) {
	ctx := c.Request.Context()

	sellerID, ok := h.sellerIDFromContext(c)
	if !ok {
		return
	}
	shippingSetupID, ok := h.parseOptionID(c)
	if !ok {
		return
	}

	var result *sellerShippingApp.GetShippingSetupWithCoveragesResult
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		result, err = h.sellerShippingService.GetShippingSetupWithCoverages(
			ctx, tx, shippingSetupID, sellerID,
		)
		return err
	})

	if err != nil {
		h.writeServiceError(c, []zap.Field{
			zap.String("shipping_option_id", shippingSetupID.String()),
			zap.String("seller_id", sellerID.String()),
		}, err, "Failed to get shipping option")
		return
	}

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

// UpdateShippingSetup handles PUT /api/v1/seller/shipping/options/:id
//
// Replaces the option identity and (when destinations are provided) its full
// destination set in ONE transaction. Editing is allowed at any time — even
// while linked to live listings: existing orders keep their checkout
// snapshot, and order creation re-validates coverage at the gate.
func (h *SellerShippingHandler) UpdateShippingSetup(c *gin.Context) {
	ctx := c.Request.Context()

	sellerID, ok := h.sellerIDFromContext(c)
	if !ok {
		return
	}
	shippingSetupID, ok := h.parseOptionID(c)
	if !ok {
		return
	}

	var req UpdateShippingPackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	input := packageRequestToInput(sellerID, ShippingPackageRequest{
		Name:            req.Name,
		TransportType:   req.TransportType,
		InternalPurpose: req.InternalPurpose,
		IsActive:        req.IsActive,
		Destinations:    req.Destinations,
	})

	var option *shippingEntity.ShippingSetup
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		option, err = h.sellerShippingService.UpdateShippingPackage(
			ctx, tx, shippingSetupID, input,
		)
		return err
	})

	if err != nil {
		h.writeServiceError(c, []zap.Field{
			zap.String("shipping_option_id", shippingSetupID.String()),
			zap.String("seller_id", sellerID.String()),
		}, err, "Failed to update shipping option")
		return
	}

	response.SuccessWithMessage(c, "Shipping option updated successfully", gin.H{
		"shipping_option": shippingSetupToResponse(option),
	})
}

// SetShippingSetupActive handles PATCH /api/v1/seller/shipping/options/:id/active
//
// Canonical retire/restore path. Linked options must be deactivated, never
// deleted.
func (h *SellerShippingHandler) SetShippingSetupActive(c *gin.Context) {
	ctx := c.Request.Context()

	sellerID, ok := h.sellerIDFromContext(c)
	if !ok {
		return
	}
	shippingSetupID, ok := h.parseOptionID(c)
	if !ok {
		return
	}

	var req SetActiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	var option *shippingEntity.ShippingSetup
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		option, err = h.sellerShippingService.SetShippingSetupActive(
			ctx, tx, shippingSetupID, sellerID, req.IsActive,
		)
		return err
	})

	if err != nil {
		h.writeServiceError(c, []zap.Field{
			zap.String("shipping_option_id", shippingSetupID.String()),
			zap.String("seller_id", sellerID.String()),
		}, err, "Failed to set shipping option active state")
		return
	}

	response.SuccessWithMessage(c, "Shipping option active state updated", gin.H{
		"shipping_option": shippingSetupToResponse(option),
	})
}

// DeleteShippingSetup handles DELETE /api/v1/seller/shipping/options/:id
//
// Hard delete is refused while the option is linked to any listing
// (order history references it): the seller must deactivate instead.
func (h *SellerShippingHandler) DeleteShippingSetup(c *gin.Context) {
	ctx := c.Request.Context()

	sellerID, ok := h.sellerIDFromContext(c)
	if !ok {
		return
	}
	shippingSetupID, ok := h.parseOptionID(c)
	if !ok {
		return
	}

	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.sellerShippingService.DeleteShippingSetup(
			ctx, tx, shippingSetupID, sellerID,
		)
	})

	if err != nil {
		h.writeServiceError(c, []zap.Field{
			zap.String("shipping_option_id", shippingSetupID.String()),
			zap.String("seller_id", sellerID.String()),
		}, err, "Failed to delete shipping option")
		return
	}

	response.SuccessWithMessage(c, "Shipping option deleted successfully", gin.H{
		"shipping_option_id": shippingSetupID.String(),
	})
}

// ============================================================================
// Response Converters
// ============================================================================

// shippingSetupToResponse converts a ShippingSetup entity to API response format.
// internal_purpose is seller-private data — this converter is ONLY used on
// seller-facing endpoints; buyer-facing payloads must never include it.
func shippingSetupToResponse(opt *shippingEntity.ShippingSetup) map[string]interface{} {
	return map[string]interface{}{
		"id":               opt.ID.String(),
		"name":             opt.Name,
		"transport_type":   string(opt.TransportType),
		"internal_purpose": opt.InternalPurpose,
		"is_active":        opt.IsActive,
		"created_at":       opt.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"updated_at":       opt.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// coverageToResponse converts a ShippingCoverage entity (with hydrated city
// qualifications) to API response format.
func coverageToResponse(cov *shippingEntity.ShippingCoverage) map[string]interface{} {
	overrides := make([]map[string]interface{}, len(cov.CityOverrides))
	for i, o := range cov.CityOverrides {
		resp := map[string]interface{}{
			"id":           o.ID.String(),
			"city_code":    o.CityCode,
			"city_name":    o.CityName,
			"created_at":   o.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			"updated_at":   o.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if o.Rate != nil {
			resp["rate"] = o.Rate.Int64()
		}
		if o.IsAvailable != nil {
			resp["is_available"] = *o.IsAvailable
		}
		overrides[i] = resp
	}

	return map[string]interface{}{
		"id":                 cov.ID.String(),
		"shipping_option_id": cov.ShippingSetupID.String(),
		"province_code":      cov.ProvinceCode,
		"province_name":      cov.ProvinceName,
		"rate":               cov.ProvinceRate.Int64(),
		"is_available":       cov.IsAvailable,
		"city_qualifications": overrides,
		"created_at":         cov.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
