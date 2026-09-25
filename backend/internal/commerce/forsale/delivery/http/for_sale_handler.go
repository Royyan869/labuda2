package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	for_saleApp "github.com/labuda/backend/internal/commerce/forsale/application"
	"github.com/labuda/backend/internal/commerce/forsale/entity"
	for_saleRepo "github.com/labuda/backend/internal/commerce/forsale/repository"
	commerceshared "github.com/labuda/backend/internal/commerce/shared"
	orderRepo "github.com/labuda/backend/internal/commerce/order/repository"
	shippingApp "github.com/labuda/backend/internal/commerce/shipping/application"
	"github.com/labuda/backend/internal/governance/viewercontext"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/pkg/blockcheck"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/internal/pkg/sellerdisplay"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"go.uber.org/zap"
)

// ForSaleHandler handles HTTP requests for for_sale operations.
type ForSaleHandler struct {
	for_saleService *for_saleApp.ForSaleService
	db              *db.DB
	log             *zap.Logger
	orderRepo       orderRepo.OrderRepository
}

// NewForSaleHandler creates a new ForSaleHandler.
func NewForSaleHandler(
	for_saleService *for_saleApp.ForSaleService,
	database *db.DB,
	log *zap.Logger,
	orderRepo orderRepo.OrderRepository,
) *ForSaleHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &ForSaleHandler{
		for_saleService: for_saleService,
		db:              database,
		log:             log,
		orderRepo:       orderRepo,
	}
}

// CreateForSaleRequest holds the request body for creating a for_sale.
type CreateForSaleRequest struct {
	// ProductID (optional) — Product identity reuse. When set, the new
	// fixed-price sale attaches to an existing Product owned by the seller.
	// When omitted, a Product is minted inline from the item fields below.
	ProductID   *string `json:"product_id"`
	Title       string  `json:"title" binding:"required"`
	Description string  `json:"description" binding:"required"`
	Price       int64   `json:"price" binding:"required,min=1"`
	// Quantity is optional — PASS_19E: a fixed-price for_sale defaults to
	// unique-item mode (quantity=1) when omitted. Sellers with real stock
	// set it explicitly to enable multi-quantity sale.
	Quantity           *int   `json:"quantity" binding:"omitempty,min=1"`
	NegotiationEnabled bool   `json:"negotiation_enabled"`
	// Optional koi-specific fields
	MediaURLs    []string `json:"media_urls"`
	Variety      string   `json:"variety"`
	SizeCM       *int     `json:"size_cm"`
	AgeMonths    *int     `json:"age_months"`
	Gender       *string  `json:"gender"`
	Breeder      *string  `json:"breeder"`
	Bloodline    *string  `json:"bloodline"`
	Certificates []string `json:"certificates"`
	// Shipping configuration
	FarmAddressID *string `json:"farm_address_id"`
	// Shipping selection (OWNER CANONICAL: create ships WITH its options —
	// at least one shipping_setup_id is REQUIRED; create is publish.)
	ShippingSetupIDs []string `json:"shipping_setup_ids" binding:"required,min=1,dive,uuid"`
	// Shipping readiness
	PreparationTime *string `json:"preparation_time" binding:"omitempty,oneof=immediate short medium long"`
	PreparationNote *string `json:"preparation_note"`
}

// CreateForSale handles POST /api/v1/for_sales
//
// Creates a new for_sale for the authenticated seller.
//
// Request body:
//   - title: ForSale title
//   - description: ForSale description
//   - price: Price per unit (in minor currency unit)
//   - quantity: Optional available quantity (must be > 0 if provided). Defaults
//     to 1 (unique item) when omitted — sellers with real stock set this
//     explicitly to enable multi-quantity sale.
//   - negotiation_enabled: Whether negotiation is allowed
//   - visibility: "public" or "private"
//
// Headers:
// - Idempotency-Key: Optional key for safe retries (recommended for mobile clients)
//
// Returns created for_sale.
func (h *ForSaleHandler) CreateForSale(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context
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

	// Reject any request body carrying a legacy for_sale_id/for_saleId alias.
	// The legacy "attach to for_sale" shape is rejected. An explicit
	// product_id (Product identity reuse) IS supported and parsed below.
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	if bytes.Contains(rawBody, []byte(`"for_sale_id"`)) || bytes.Contains(rawBody, []byte(`"for_saleId"`)) {
		response.BadRequest(c, "legacy for_sale_id field is not supported; use product_id for Product reuse or inline product fields")
		return
	}
	if bytes.Contains(rawBody, []byte(`"listing_id"`)) || bytes.Contains(rawBody, []byte(`"listingId"`)) {
		response.BadRequest(c, "legacy listing_id/listingId field is not supported; use product_id for Product reuse or inline product fields")
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(rawBody))

	// Parse request body
	var req CreateForSaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}

	// Extract optional Idempotency-Key header for duplicate prevention
	idempotencyKey := c.GetHeader("Idempotency-Key")

	// If idempotency key is provided, check for existing operation
	if idempotencyKey != "" {
		// Generate a scoped key for this user and operation
		scopedKey := fmt.Sprintf("for_sale.create.%s.%s", sellerID.String(), idempotencyKey)

		// Check for existing idempotency record
		var existingForSale map[string]interface{}
		checkQuery := `
			SELECT response_data
			FROM idempotency_records
			WHERE idempotency_key = $1
			  AND user_id = $2
			  AND status = 'completed'
			LIMIT 1
		`
		var responseDataJSON []byte
		err := h.db.Pool().QueryRow(ctx, checkQuery, scopedKey, sellerID).Scan(&responseDataJSON)
		if err == nil {
			// Existing record found - return cached response
			json.Unmarshal(responseDataJSON, &existingForSale)
			if existingForSale != nil {
				response.Success(c, gin.H{
					"message":  "ForSale already created (idempotent request)",
					"for_sale": existingForSale,
				})
				return
			}
		}
		// If error is "no rows", proceed with creation
	}

	// Parse optional UUID fields
	var farmAddressID *uuid.UUID
	if req.FarmAddressID != nil {
		if id, err := uuid.Parse(*req.FarmAddressID); err == nil {
			farmAddressID = &id
		}
	}

	// Parse shipping selection IDs (required — create = publish).
	shippingSetupIDs := make([]uuid.UUID, 0, len(req.ShippingSetupIDs))
	for _, raw := range req.ShippingSetupIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			response.BadRequest(c, "Invalid shipping_setup_ids format")
			return
		}
		shippingSetupIDs = append(shippingSetupIDs, id)
	}

	// Parse optional product_id for Product identity reuse.
	var productID *uuid.UUID
	if req.ProductID != nil && *req.ProductID != "" {
		if id, err := uuid.Parse(*req.ProductID); err == nil {
			productID = &id
		} else {
			response.BadRequest(c, "Invalid product_id format")
			return
		}
	}

	// Execute within transaction
	var for_sale *entity.ForSale
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error

		// Parse preparation time, default to immediate if not provided
		preparationTime := entity.PreparationTimeImmediate
		if req.PreparationTime != nil {
			preparationTime = entity.PreparationTime(*req.PreparationTime)
			if !preparationTime.IsValid() {
				preparationTime = entity.PreparationTimeImmediate
			}
		}

		// Quantity defaults to 1 (unique item) when omitted — PASS_19E.
		quantity := 1
		if req.Quantity != nil {
			quantity = *req.Quantity
		}

		for_sale, err = h.for_saleService.Create(ctx, tx, for_saleApp.CreateForSaleInput{
			SellerID:           sellerID,
			ProductID:          productID,
			Title:              req.Title,
			Description:        req.Description,
			MediaURLs:          req.MediaURLs,
			Variety:            req.Variety,
			SizeCM:             req.SizeCM,
			AgeMonths:          req.AgeMonths,
			Gender:             req.Gender,
			Breeder:            req.Breeder,
			Bloodline:          req.Bloodline,
			Certificates:       req.Certificates,
			PricePerUnit:       money.New(req.Price),
			QuantityAvailable:  quantity,
			NegotiationEnabled: req.NegotiationEnabled,
			// Shipping preferences
			FarmAddressID: farmAddressID,
			// Shipping selection — create = publish; options are mandatory.
			ShippingSetupIDs: shippingSetupIDs,
			// Shipping readiness
			PreparationTime: preparationTime,
			PreparationNote: req.PreparationNote,
		})
		return err
	})

	if err != nil {
		h.log.Error("Failed to create for_sale",
			zap.String("seller_id", sellerID.String()),
			zap.Error(err),
		)
		// CREATE = PUBLISH: the publish gates fire inside create now, so the
		// typed gate errors must surface as machine-readable codes (never 500).
		if err == auth.ErrMarketAuthorityRequired {
			response.MarketAuthorityRequired(c, "Active seller subscription required to create for_sales")
			return
		}
		if errors.Is(err, shippingApp.ErrShippingNotConfigured) {
			response.Error(c, http.StatusBadRequest, "SHIPPING_NOT_CONFIGURED",
				"ForSale tidak bisa dibuat: pilih minimal satu opsi pengiriman yang punya coverage aktif.")
			return
		}
		if errors.Is(err, for_saleApp.ErrFarmAddressNotConfigured) {
			response.Error(c, http.StatusBadRequest, "FARM_ADDRESS_NOT_CONFIGURED",
				"ForSale tidak bisa dibuat: alamat pengirim (farm address) belum diatur atau tidak valid.")
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	// Store idempotency record if key was provided
	if idempotencyKey != "" {
		scopedKey := fmt.Sprintf("for_sale.create.%s.%s", sellerID.String(), idempotencyKey)
		responseData := for_saleToResponse(for_sale, sellerID)
		responseDataJSON, _ := json.Marshal(responseData)

		insertQuery := `
			INSERT INTO idempotency_records (idempotency_key, user_id, operation, entity_id, response_data, status, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, NOW())
			ON CONFLICT (idempotency_key) DO NOTHING
		`
		_, _ = h.db.Pool().Exec(ctx, insertQuery, scopedKey, sellerID, "for_sale.create", for_sale.ID, responseDataJSON, "completed")
	}

	response.Created(c, for_saleToResponse(for_sale, sellerID))
}

// UpdateForSaleRequest holds the request body for updating a for_sale.
//
// Quantity is intentionally excluded — stock mutations follow canonical paths:
//   - ReduceQuantity: order creation (via OrderCreationService)
//   - RestoreQuantity: order cancel/expire (via OrderCompletionService)
//
// The handler does NOT bypass domain authority for quantity.
type UpdateForSaleRequest struct {
	Title              *string `json:"title"`
	Description        *string `json:"description"`
	Price              *int64  `json:"price" binding:"omitempty,min=1"`
	NegotiationEnabled *bool   `json:"negotiation_enabled"`
	Status             *string `json:"status" binding:"omitempty,oneof=draft active withdrawn sold"`
	// Optional koi-specific fields
	MediaURLs    *[]string `json:"media_urls"`
	Variety      *string   `json:"variety"`
	SizeCM       *int      `json:"size_cm"`
	AgeMonths    *int      `json:"age_months"`
	Gender       *string   `json:"gender"`
	Breeder      *string   `json:"breeder"`
	Bloodline    *string   `json:"bloodline"`
	Certificates *[]string `json:"certificates"`
	// Shipping readiness
	PreparationTime *string `json:"preparation_time" binding:"omitempty,oneof=immediate short medium long"`
	PreparationNote *string `json:"preparation_note"`
}

// requiresMarketAuthorityForPublish reports whether a requested status
// transition will result in ACTIVE + PUBLIC market exposure and therefore
// requires a market-authority check before it is allowed to proceed.
//
// entity.ForSale.Publish() unconditionally forces Visibility=Public on
// every draft → active transition, regardless of the for_sale's current
// visibility or whether the update request even included a visibility field.
// This predicate intentionally depends ONLY on the status transition — never
// on current/requested visibility — so a mobile client sending a status-only
// publish update from a private draft cannot bypass the check (PASS_18M
// market-authority bypass; PASS_18O fix).
func requiresMarketAuthorityForPublish(currentStatus, newStatus entity.ForSaleStatus) bool {
	return currentStatus == entity.ForSaleStatusDraft && newStatus == entity.ForSaleStatusActive
}

// UpdateForSale handles PUT /api/v1/for_sales/:id
//
// Updates an existing for_sale.
// Only the for_sale owner can update.
// Active for_sales can be updated; sold/withdrawn for_sales cannot be modified.
func (h *ForSaleHandler) UpdateForSale(c *gin.Context) {
	ctx := c.Request.Context()

	// Get for_sale ID
	for_saleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid for_sale ID")
		return
	}

	// Get user ID from context
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	callerID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Parse request body
	var req UpdateForSaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}

	var updatedForSale *entity.ForSale
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		// Detect publish intent (draft → active) without mutating entity.
		// For non-publish status values via PUT, reject — lifecycle mutations
		// other than publish must use dedicated endpoints (e.g., DELETE for withdraw).
		if req.Status != nil {
			newStatus := entity.ForSaleStatus(*req.Status)
			if !newStatus.IsValid() {
				return fmt.Errorf("invalid status: must be 'draft', 'active', 'withdrawn', or 'sold'")
			}
			// Only draft → active is allowed as publish via this endpoint.
			// Other status transitions via PUT are not seller content edits.
			if newStatus != entity.ForSaleStatusDraft && newStatus != entity.ForSaleStatusActive {
				// For draft, only draft status is valid via content edit; active is publish.
				// Non-draft status via PUT is rejected — use dedicated lifecycle endpoints.
				if newStatus == entity.ForSaleStatusWithdrawn || newStatus == entity.ForSaleStatusSold {
					return fmt.Errorf("status transition %s via PUT not allowed; use dedicated endpoint", newStatus)
				}
			}
			if newStatus == entity.ForSaleStatusActive {
				// Defer to canonical publish authority after content edit if needed.
				// Publish will re-lock and validate draft status, ownership, restriction, market authority.
			}
		}

		// Check if request contains seller-controlled content fields.
		hasContent := req.Title != nil || req.Description != nil || req.Price != nil || req.NegotiationEnabled != nil ||
			req.MediaURLs != nil || req.Variety != nil || req.SizeCM != nil || req.AgeMonths != nil ||
			req.Gender != nil || req.Breeder != nil || req.Bloodline != nil || req.Certificates != nil ||
			req.PreparationTime != nil || req.PreparationNote != nil

		isPublishIntent := req.Status != nil && entity.ForSaleStatus(*req.Status) == entity.ForSaleStatusActive

		// Content edit via canonical authority.
		if hasContent {
			input := for_saleApp.UpdateSellerInput{
				ForSaleID:          for_saleID,
				SellerID:           callerID,
				Title:              req.Title,
				Description:        req.Description,
				Price:              req.Price,
				NegotiationEnabled: req.NegotiationEnabled,
				MediaURLs:          req.MediaURLs,
				Variety:            req.Variety,
				SizeCM:             req.SizeCM,
				AgeMonths:          req.AgeMonths,
				Gender:             req.Gender,
				Breeder:            req.Breeder,
				Bloodline:          req.Bloodline,
				Certificates:       req.Certificates,
				PreparationTime:    req.PreparationTime,
				PreparationNote:    req.PreparationNote,
			}
			saved, err := h.for_saleService.UpdateSeller(ctx, tx, input)
			if err != nil {
				return err
			}
			updatedForSale = saved
		}

		// Publish intent handling (draft → active)
		if isPublishIntent {
			if err := h.for_saleService.Publish(ctx, tx, for_saleID, callerID); err != nil {
				return err
			}
			published, reErr := h.for_saleService.GetByID(ctx, tx, for_saleID)
			if reErr == nil {
				updatedForSale = published
			} else if updatedForSale == nil {
				// No content edit before, just publish — load it
				updatedForSale = published
			}
			return nil
		}

		// If no content and no publish, but status was provided as draft (no-op), ensure we return current.
		if !hasContent && !isPublishIntent {
			// If request had only status=draft or empty, treat as no-op content edit.
			// For draft, UpdateSeller with no fields would be no-op; just load current.
			if req.Status != nil && entity.ForSaleStatus(*req.Status) == entity.ForSaleStatusDraft {
				// No mutation needed, just return current state (already draft)
				if updatedForSale == nil {
					cur, err := h.for_saleService.GetByID(ctx, tx, for_saleID)
					if err != nil {
						return err
					}
					updatedForSale = cur
				}
				return nil
			}
			if hasContent == false && req.Status == nil {
				return fmt.Errorf("no fields to update")
			}
			// For other cases with no content but status present (e.g., withdrawn via PUT), already rejected above.
		}

		return nil
	})

	if err != nil {
		h.log.Error("Failed to update for_sale",
			zap.String("for_sale_id", for_saleID.String()),
			zap.Error(err),
		)
		// Canonical live immutability
		if errors.Is(err, entity.ErrLiveImmutable) {
			response.Error(c, http.StatusConflict, "LIVE_IMMUTABLE",
				"ForSale is live (active/sold/withdrawn) — seller edit forbidden.")
			return
		}
		// Phase 0 honesty: surface the typed shipping gate error as a
		// machine-readable code so mobile can branch without string matching.
		if errors.Is(err, shippingApp.ErrShippingNotConfigured) {
			response.Error(c, http.StatusBadRequest, "SHIPPING_NOT_CONFIGURED",
				"ForSale belum bisa dipublish: belum ada opsi pengiriman terpilih untuk for_sale ini.")
			return
		}
		if errors.Is(err, for_saleApp.ErrFarmAddressNotConfigured) {
			response.Error(c, http.StatusBadRequest, "FARM_ADDRESS_NOT_CONFIGURED",
				"ForSale belum bisa dipublish: alamat pengirim (farm address) belum diatur atau tidak valid.")
			return
		}
		// Check for specific error types
		errMsg := err.Error()
		if errMsg == "forbidden: you can only update your own for_sales" {
			response.Forbidden(c, "You can only update your own for_sales")
			return
		}
		if err == auth.ErrMarketAuthorityRequired {
			response.MarketAuthorityRequired(c, "Active seller subscription required to publish for_sales")
			return
		}
		if strings.Contains(errMsg, "cannot update for_sale with status") {
			response.BadRequest(c, errMsg)
			return
		}
		if strings.Contains(errMsg, "title must be") || strings.Contains(errMsg, "description must be") || strings.Contains(errMsg, "certificate") || strings.Contains(errMsg, "preparation_time") {
			response.BadRequest(c, errMsg)
			return
		}
		if errors.Is(err, shippingApp.ErrShippingLiveImmutable) {
			response.Error(c, http.StatusConflict, "LIVE_IMMUTABLE",
				"Shipping configuration is immutable for live product.")
			return
		}
		response.InternalServerError(c, "Failed to update for_sale")
		return
	}

	response.Success(c, for_saleToResponse(updatedForSale, callerID))
}

// GetForSale handles GET /api/v1/for_sales/:id
//
// Retrieves a for_sale by ID.
// Public for_sales are accessible to all users.
// Private for_sales are only accessible to the owner.
func (h *ForSaleHandler) GetForSale(c *gin.Context) {
	ctx := c.Request.Context()

	// Get for_sale ID
	for_saleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid for_sale ID")
		return
	}

	// Get caller ID (optional - for auth check)
	var callerID *uuid.UUID
	if userIDVal, exists := c.Get("userID"); exists {
		if id, ok := userIDVal.(uuid.UUID); ok {
			callerID = &id
		}
	}

	var for_sale *entity.ForSale
	var sellerInfo sellerdisplay.Info
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		for_sale, err = h.for_saleService.GetByID(ctx, tx, for_saleID)
		if err != nil {
			return err
		}

		// Block enforcement: hide for_sale from blocked seller's viewer
		if callerID != nil && for_sale.SellerID != *callerID {
			blocked, blockErr := blockcheck.IsBidirectionallyBlocked(ctx, tx, *callerID, for_sale.SellerID)
			if blockErr != nil {
				h.log.Warn("block check failed, fail-open", zap.Error(blockErr))
			}
			if blocked {
				return fmt.Errorf("blocked")
			}
		}

		// Phase 5 Stage 1 — hydrate additive seller convergence fields
		// (seller_username/seller_farm_name/seller_avatar_url) inside
		// the same transaction. Single query; no N+1.
		sellerInfo, _ = sellerdisplay.FetchOne(ctx, tx, for_sale.SellerID)

		return nil
	})

	if err != nil {
		if err.Error() == "blocked" {
			response.NotFound(c, "ForSale not found")
			return
		}
		h.log.Error("Failed to get for_sale",
			zap.String("for_sale_id", for_saleID.String()),
			zap.Error(err),
		)
		response.NotFound(c, "ForSale not found")
		return
	}

	// Authorization check for private for_sales
	if for_sale.Visibility == entity.ForSaleVisibilityPrivate {
		if callerID == nil || for_sale.SellerID != *callerID {
			response.NotFound(c, "ForSale not found")
			return
		}
	}

	resp := forSaleToDetailResponseWithViewerCapabilities(for_sale, sellerInfo, callerID)
	response.Success(c, resp)
}

// DeleteForSale handles DELETE /api/v1/for_sales/:id
//
// Withdraws a for_sale from sale.
// Only the for_sale owner can withdraw their for_sale.
// This transitions the for_sale status to "withdrawn".
func (h *ForSaleHandler) DeleteForSale(c *gin.Context) {
	ctx := c.Request.Context()

	// Get for_sale ID
	for_saleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid for_sale ID")
		return
	}

	// Get user ID from context
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	callerID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		// Get the for_sale first for ownership check
		for_sale, err := h.for_saleService.GetByID(ctx, tx, for_saleID)
		if err != nil {
			return err
		}

		// Authorization check: only the seller can withdraw their for_sale
		if for_sale.SellerID != callerID {
			return fmt.Errorf("forbidden: you can only withdraw your own for_sales")
		}

		// Withdraw the for_sale
		return h.for_saleService.Withdraw(ctx, tx, for_saleID)
	})

	if err != nil {
		h.log.Error("Failed to withdraw for_sale",
			zap.String("for_sale_id", for_saleID.String()),
			zap.Error(err),
		)
		// Check if it's a forbidden error
		if err.Error() == "forbidden: you can only withdraw your own for_sales" {
			response.Forbidden(c, "You can only withdraw your own for_sales")
			return
		}
		response.InternalServerError(c, "Failed to withdraw for_sale")
		return
	}

	response.SuccessWithMessage(c, "ForSale withdrawn successfully", nil)
}

// ListForSalesRequest holds the query parameters for ListForSales.
type ListForSalesRequest struct {
	Page             int    `form:"page" binding:"omitempty,min=1"`
	Limit            int    `form:"limit" binding:"omitempty,min=1,max=100"`
	SellerID         string `form:"seller_id" binding:"omitempty,uuid"`
	Sort             string `form:"sort" binding:"omitempty,oneof=created_at created_at_desc price price_asc price_desc"`
	IncludeWithdrawn bool   `form:"include_withdrawn"` // Include withdrawn for_sales (default: false)
}

// ListForSales handles GET /api/v1/for_sales
//
// Query parameters:
// - page (optional): Page number for pagination (default: 1)
// - limit (optional): Number of results per page (default: 20, max: 100)
// - seller_id (optional): Filter by seller ID (UUID)
// - sort (optional): Sort order - "created_at", "created_at_desc", "price", "price_asc", "price_desc" (default: created_at_desc)
// - include_withdrawn (optional): Include withdrawn for_sales in seller view (default: false)
//
// Response:
// - for_sales: Array of for_sale items
// - page: Current page number
// - limit: Results per page
// - total: Total number of for_sales (for the current filter)
func (h *ForSaleHandler) ListForSales(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse query parameters
	var req ListForSalesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}

	// Set defaults
	page := req.Page
	if page <= 0 {
		page = 1
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	offset := (page - 1) * limit

	// Get optional seller filter
	var sellerID *uuid.UUID
	if req.SellerID != "" {
		id, err := uuid.Parse(req.SellerID)
		if err != nil {
			response.BadRequest(c, "Invalid seller_id format")
			return
		}
		sellerID = &id
	}

	// Extract viewer ID (optional — anonymous viewers see all)
	var viewerID uuid.UUID
	if userIDVal, exists := c.Get("userID"); exists {
		if id, ok := userIDVal.(uuid.UUID); ok {
			viewerID = id
		}
	}

	// Block enforcement: if filtering by seller_id and viewer has blocked that seller,
	// return empty results (the seller is invisible to this viewer).
	if sellerID != nil && viewerID != uuid.Nil && *sellerID != viewerID {
		var blocked bool
		_ = h.db.WithTx(ctx, func(tx db.Tx) error {
			var err error
			blocked, err = blockcheck.IsBidirectionallyBlocked(ctx, tx, viewerID, *sellerID)
			return err
		})
		if blocked {
			response.Success(c, gin.H{"for_sales": []map[string]interface{}{}, "page": page, "limit": limit, "total": 0})
			return
		}
	}

	// Execute query within transaction
	var for_sales []*entity.ForSale
	var sellerInfoByID map[uuid.UUID]sellerdisplay.Info
	var total int
	var err error

	txErr := h.db.WithTx(ctx, func(tx db.Tx) error {
		if sellerID != nil {
			// Owner-only inventory branch: full history (draft/active/sold;
			// withdrawn excluded by default). Anonymous/non-owner callers must
			// never reach the seller-inventory query — they get the public
			// seller page (active + in-stock only).
			isOwner := viewerID != uuid.Nil && *sellerID == viewerID
			if isOwner {
				for_sales, err = h.for_saleService.GetBySellerIDPaginated(ctx, tx, *sellerID, limit, offset, req.IncludeWithdrawn)
			} else {
				for_sales, err = h.for_saleService.GetPublicBySellerID(ctx, tx, *sellerID, limit, offset)
			}
			// Note: exact total count would require a separate count query
			// For pagination purposes, we use len(for_sales) as a minimum
			total = len(for_sales)
		} else {
			// Get public for_sales (always excludes withdrawn)
			for_sales, err = h.for_saleService.GetPublic(ctx, tx, limit, offset)
			// Note: total count would require a separate count query
			// For now, set total to len(for_sales) as a minimum
			total = len(for_sales)
		}
		if err != nil {
			return err
		}

		// Block enforcement: post-fetch filter for general for_sale browse.
		// Remove for_sales from sellers that the viewer has blocked.
		if viewerID != uuid.Nil && sellerID == nil {
			sellerIDs := make([]uuid.UUID, 0, len(for_sales))
			for _, l := range for_sales {
				sellerIDs = append(sellerIDs, l.SellerID)
			}
			blockedSet, _ := blockcheck.BlockedSet(ctx, tx, viewerID, sellerIDs)
			if len(blockedSet) > 0 {
				filtered := make([]*entity.ForSale, 0, len(for_sales))
				for _, l := range for_sales {
					if !blockedSet[l.SellerID] {
						filtered = append(filtered, l)
					}
				}
				for_sales = filtered
				total = len(for_sales)
			}
		}

		// Phase 5 Stage 1 — batch-hydrate additive seller convergence
		// fields. One query for all for_sales on the page; no N+1.
		sellerIDs := make([]uuid.UUID, 0, len(for_sales))
		for _, l := range for_sales {
			sellerIDs = append(sellerIDs, l.SellerID)
		}
		sellerInfoByID, _ = sellerdisplay.FetchMany(ctx, tx, sellerIDs)

		return nil
	})

	if txErr != nil {
		h.log.Error("Failed to list for_sales",
			zap.Error(txErr),
		)
		response.InternalServerError(c, "Failed to retrieve for_sales")
		return
	}

	// Convert to response format
	items := make([]map[string]interface{}, 0, len(for_sales))
	for _, for_sale := range for_sales {
		resp := for_saleToResponseWithSeller(for_sale, sellerInfoByID[for_sale.SellerID], &viewerID)
		items = append(items, resp)
	}

	response.Success(c, gin.H{
		"for_sales": items,
		"page":      page,
		"limit":     limit,
		"total":     total,
	})
}

// SearchForSalesRequest holds the query parameters for SearchForSales.
type SearchForSalesRequest struct {
	Query    string  `form:"q" binding:"required"`
	PriceMin *int64  `form:"price_min" binding:"omitempty,min=1"`
	PriceMax *int64  `form:"price_max" binding:"omitempty,min=1"`
	Variety  *string `form:"variety"`
	SellerID *string `form:"seller_id" binding:"omitempty,uuid"`
	Cursor   *string `form:"cursor" binding:"omitempty"`
	Limit    int     `form:"limit" binding:"omitempty,min=1,max=100"`
	SortBy   string  `form:"sort" binding:"omitempty,oneof=relevance price created_at"`
	SortDir  string  `form:"sort_dir" binding:"omitempty,oneof=asc desc"`
}

// SearchForSales handles GET /api/v1/search/for_sales
//
// Performs full-text search on for_sales with cursor pagination.
//
// Query parameters:
// - q (required): Search query string
// - price_min (optional): Minimum price
// - price_max (optional): Maximum price
// - variety (optional): Koi variety filter
// - seller_id (optional): Filter by seller ID (UUID)
// - cursor (optional): RFC3339 timestamp for pagination
// - limit (optional): Number of results (default: 20, max: 100)
// - sort (optional): Sort order - "relevance", "price", "created_at" (default: relevance)
// - sort_dir (optional): Sort direction - "asc" or "desc" (default: desc)
//
// Response:
// - for_sales: Array of for_sale items
// - next_cursor: Timestamp for next page (null if no more results)
// - has_more: Boolean indicating if there are more results
func (h *ForSaleHandler) SearchForSales(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse query parameters
	var req SearchForSalesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}

	// Extract viewer ID (optional) — Scope 3 status boundary: the exact
	// internal state crosses the wire ONLY for the owning seller.
	var viewerID uuid.UUID
	if userIDVal, exists := c.Get("userID"); exists {
		if id, ok := userIDVal.(uuid.UUID); ok {
			viewerID = id
		}
	}

	// Set defaults
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// Parse cursor if provided
	var cursor *time.Time
	if req.Cursor != nil && *req.Cursor != "" {
		parsedTime, err := time.Parse(time.RFC3339Nano, *req.Cursor)
		if err != nil {
			response.BadRequest(c, "Invalid cursor format")
			return
		}
		cursor = &parsedTime
	}

	// Parse seller ID if provided
	var sellerID *uuid.UUID
	if req.SellerID != nil && *req.SellerID != "" {
		id, err := uuid.Parse(*req.SellerID)
		if err != nil {
			response.BadRequest(c, "Invalid seller_id format")
			return
		}
		sellerID = &id
	}

	// Build filters
	filters := for_saleRepo.SearchFilters{
		Query:    req.Query,
		PriceMin: req.PriceMin,
		PriceMax: req.PriceMax,
		Variety:  req.Variety,
		SellerID: sellerID,
		Cursor:   cursor,
		Limit:    limit,
		SortBy:   req.SortBy,
		SortDir:  req.SortDir,
	}

	// Default sort to relevance if searching
	if filters.SortBy == "" {
		filters.SortBy = "relevance"
	}

	// Execute search within transaction
	var result *for_saleApp.SearchResult
	var sellerInfoByID map[uuid.UUID]sellerdisplay.Info
	txErr := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		result, err = h.for_saleService.Search(ctx, tx, filters)
		if err != nil {
			return err
		}

		// Phase 5 Stage 1 — batch-hydrate additive seller convergence
		// fields for the page.
		sellerIDs := make([]uuid.UUID, 0, len(result.ForSales))
		for _, l := range result.ForSales {
			sellerIDs = append(sellerIDs, l.SellerID)
		}
		sellerInfoByID, _ = sellerdisplay.FetchMany(ctx, tx, sellerIDs)

		return nil
	})

	if txErr != nil {
		h.log.Error("Failed to search for_sales",
			zap.Error(txErr),
		)
		response.InternalServerError(c, "Failed to search for_sales")
		return
	}

	// Convert to response format
	items := make([]map[string]interface{}, 0, len(result.ForSales))
	for _, for_sale := range result.ForSales {
		resp := for_saleToResponseWithSeller(for_sale, sellerInfoByID[for_sale.SellerID], &viewerID)
		items = append(items, resp)
	}

	response.Success(c, gin.H{
		"for_sales":   items,
		"next_cursor": result.NextCursor,
		"has_more":    result.HasMore,
	})
}

// for_saleToResponse converts a for_sale entity to API response.
//
// Phase 5 Stage 1 — additive seller convergence fields:
//   - seller_username   (= user_profiles.username)
//   - seller_farm_name  (= seller_profiles.store_name)
//   - seller_avatar_url (= user_profiles.avatar_url)
//
// These fields are ALWAYS present (empty string when absent) so the
// shape is stable; existing fields are unchanged.
//
// Owner-only write surfaces (create/update): the response carries the exact
// internal state as `seller_status` for the owning seller (Scope 3).
func for_saleToResponse(l *entity.ForSale, ownerID uuid.UUID) map[string]interface{} {
	return for_saleToResponseWithSeller(l, sellerdisplay.Info{}, &ownerID)
}

// sellerStatusForViewer returns the exact internal for_sale status only for
// the owning seller; every other viewer gets null. The raw state-machine
// value never crosses the public boundary (Scope 3 parity with auction).
func sellerStatusForViewer(l *entity.ForSale, viewerID *uuid.UUID) *string {
	if viewerID == nil || *viewerID == uuid.Nil || *viewerID != l.SellerID {
		return nil
	}
	s := string(l.Status)
	return &s
}

// soldAtForViewer / withdrawnAtForViewer gate internal transition timestamps
// to the owning seller; every other viewer reads null.
func soldAtForViewer(l *entity.ForSale, viewerID *uuid.UUID) *time.Time {
	if viewerID == nil || *viewerID == uuid.Nil || *viewerID != l.SellerID {
		return nil
	}
	return l.SoldAt
}

func withdrawnAtForViewer(l *entity.ForSale, viewerID *uuid.UUID) *time.Time {
	if viewerID == nil || *viewerID == uuid.Nil || *viewerID != l.SellerID {
		return nil
	}
	return l.WithdrawnAt
}

// for_saleToResponseWithSeller renders for_sale JSON with seller display
// fields hydrated from sellerdisplay.Info. Used by list/search/detail
// endpoints that batch-fetch seller info to avoid N+1.
//
// Scope 3 — status boundary (parity with auction): `status` carries the
// coarsened public lifecycle vocabulary via entity.ForSaleStatus
// .PublicLifecycle() ({active, unavailable}; draft never crosses the public
// boundary). The exact internal state crosses the wire ONLY via
// `seller_status`, and only when viewerID is the owning seller — for every
// other viewer it is null. Internal timestamps (sold_at/withdrawn_at) are
// likewise gated to the owning seller.
func for_saleToResponseWithSeller(
	l *entity.ForSale,
	seller sellerdisplay.Info,
	viewerID *uuid.UUID,
) map[string]interface{} {
	product := l.Product

	// Canonical media authority is Product.MediaURLs — the deprecated alias
	// fallback (l.MediaURLs) was purged with the alias fields. Typed media
	// block is the CONVERGED shared helper (same shape as auction detail).
	mediaURLs := product.MediaURLs
	var renderedMedia []map[string]interface{}
	if product != nil {
		renderedMedia = commerceshared.MediaWireItems(product.MediaURLs, l.CreatedAt)
	} else {
		renderedMedia = []map[string]interface{}{}
	}

	// Canonical PublicCard ForSaleCard (Batch 2C).
	// Carries the coarsened public lifecycle vocabulary {active, unavailable,
	// removed} via entity.ForSaleStatus.PublicLifecycle(); raw enum (draft,
	// sold, withdrawn) is intentionally NEVER read by the card.
	var thumbnail *string
	if len(mediaURLs) > 0 {
		t := mediaURLs[0]
		thumbnail = &t
	}
	var sellerAvatarPtr *string
	if seller.AvatarURL != "" {
		a := seller.AvatarURL
		sellerAvatarPtr = &a
	}
	// E8.1 / expired-seller-visibility — Coarsen both axes from raw
	// sellerdisplay.Info truth via canonical mapping sites. user-identity
	// axis covers banned/suspended/deleted; seller-trust axis covers
	// subscription expired/lapsed. Both axes are emitted independently so
	// mobile applies distinct UI policies (block/redact vs badge+CTA-disable).
	userLifecycle := string(viewercontext.CoarsenLifecycle(seller.AccountStatus, seller.IsDeleted))
	sellerTrustLifecycle := string(viewercontext.CoarsenSellerTrust(seller.SubscriptionStatus))
	sellerCard := publiccard.NewSellerCardWithBothLifecycles(
		l.SellerID, seller.Username, sellerAvatarPtr,
		seller.FarmName,
		userLifecycle,
		sellerTrustLifecycle,
		seller.Tier,
	)
	forSaleCard := publiccard.NewForSaleCard(
		l.ID,
		product.Title,
		thumbnail,
		l.PricePerUnit.Int64(),
		nil, // Currency not hydrated on this surface
		l.Status.PublicLifecycle(),
		&sellerCard,
	)

	resp := map[string]interface{}{
		"id":                  l.ID.String(),
		"product_id":          l.ProductID.String(),
		"seller_id":           l.SellerID.String(),
		"title":               product.Title,
		"description":         product.Description,
		"media":               renderedMedia,
		"media_urls":          mediaURLs,
		"variety":             product.Variety,
		"size_cm":             product.SizeCm,
		"age_months":          product.AgeMonths,
		"gender":              product.Gender,
		"breeder":             product.Breeder,
		"bloodline":           product.Bloodline,
		"certificates":        product.Certificates,
		"farm_address_id":     product.FarmAddressID,
		"price":               l.PricePerUnit.Int64(),
		"quantity":            l.QuantityAvailable,
		"negotiation_enabled": l.NegotiationEnabled,
		"visibility":       string(l.Visibility),
		// PUBLIC BOUNDARY: `status` is the coarsened public lifecycle
		// ({active, unavailable}); the raw internal enum crosses the wire
		// ONLY via `seller_status` for the owning seller (Scope 3 parity
		// with auction).
		"status":           l.Status.PublicLifecycle(),
		"lifecycle":        l.Status.PublicLifecycle(),
		"seller_status":    sellerStatusForViewer(l, viewerID),
		"preparation_time": product.PreparationTime,
		"preparation_note": product.PreparationNote,
		"published_at":     l.PublishedAt,
		// Internal transition timestamps are owner-scoped truth — hidden
		// from every other viewer (Scope 3 parity with auction).
		"sold_at":      soldAtForViewer(l, viewerID),
		"withdrawn_at": withdrawnAtForViewer(l, viewerID),
		"created_at":       l.CreatedAt.Format(time.RFC3339),
		"updated_at":       l.UpdatedAt.Format(time.RFC3339),
		// Phase 5 Stage 1 additive seller convergence fields.
		// seller_username   = user_profiles.username (NEVER store_name)
		// seller_farm_name  = seller_profiles.store_name (NEVER username)
		// seller_avatar_url = user_profiles.avatar_url
		"seller_username":   seller.Username,
		"seller_farm_name":  seller.FarmName,
		"seller_avatar_url": seller.AvatarURL,
		// Canonical PublicCard block (Batch 2C).
		"for_sale": forSaleCard,
	}
	return resp
}
