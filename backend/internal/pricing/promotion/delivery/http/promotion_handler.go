package http

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/audit"
	billingApp "github.com/labuda/backend/internal/finance/billing/application"
	billingentity "github.com/labuda/backend/internal/finance/billing/entity"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/response"
	promotionApp "github.com/labuda/backend/internal/pricing/promotion/application"
	"github.com/labuda/backend/internal/pricing/promotion/entity"
	promotionRepo "github.com/labuda/backend/internal/pricing/promotion/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"go.uber.org/zap"
)

// PromotionHandler handles HTTP requests for promotion operations.
//
// Promotion domain provides duration-based promotion entitlement for for_sale items,
// auctions, and external products. This is NOT an ad platform - it's a simple
// promotion system where users purchase packages and activate them on targets.
type PromotionHandler struct {
	promotionService *promotionApp.PromotionService
	billingService   *billingApp.BillingService
	roleChecker      auth.RoleChecker
	db               db.Transactor
	log              *zap.Logger
	adminAuditLogger audit.AdminAuditLogger
	eventRepo        promotionRepo.PromotionEventRepository // optional; nil = tracking disabled
}

// NewPromotionHandler creates a new PromotionHandler.
func NewPromotionHandler(
	promotionService *promotionApp.PromotionService,
	billingService *billingApp.BillingService,
	roleChecker auth.RoleChecker,
	db db.Transactor,
	log *zap.Logger,
	adminAuditLogger ...audit.AdminAuditLogger,
) *PromotionHandler {
	if log == nil {
		log = zap.NewNop()
	}
	var auditLogger audit.AdminAuditLogger
	if len(adminAuditLogger) > 0 {
		auditLogger = adminAuditLogger[0]
	}
	return &PromotionHandler{
		promotionService: promotionService,
		billingService:   billingService,
		roleChecker:      roleChecker,
		db:               db,
		log:              log,
		adminAuditLogger: auditLogger,
	}
}

// SetEventRepo wires the analytics event repository for click tracking.
// Must be called after NewPromotionHandler before serving requests.
// Safe to skip — if nil, POST /promotions/events returns 204 and summary reads
// return 503 (analytics disabled).
func (h *PromotionHandler) SetEventRepo(r promotionRepo.PromotionEventRepository) {
	h.eventRepo = r
}

// ============================================================================
// REQUEST DTOs
// ============================================================================

// ActivatePromotionRequest represents the request body for activating a promotion.
type ActivatePromotionRequest struct {
	OwnershipID uuid.UUID `json:"ownership_id" binding:"required"`
	TargetType  string    `json:"target_type" binding:"required,oneof=for_sale auction external_product"`
	TargetID    *string   `json:"target_id"`
}

// DeactivatePromotionRequest represents the request body for deactivating a promotion.
type DeactivatePromotionRequest struct {
	Reason string `json:"reason" binding:"required,oneof=user_paused user_cancelled"`
}

// ReassignPromotionRequest represents the request body for reassigning a promotion.
type ReassignPromotionRequest struct {
	NewTargetType string  `json:"new_target_type" binding:"required,oneof=for_sale auction external_product"`
	NewTargetID   *string `json:"new_target_id"`
}

// PurchasePackageRequest represents the request body for purchasing a promotion package.
// IMPORTANT: The price is NOT provided by the client - it is always derived from the package.
type PurchasePackageRequest struct {
	PackageID uuid.UUID `json:"package_id" binding:"required"`
}

// PurchasePackageResponse represents the response after initiating a package purchase.
type PurchasePackageResponse struct {
	BillingID uuid.UUID `json:"billing_id"`
	PaymentID uuid.UUID `json:"payment_id"`
	// PaymentURL is the Midtrans redirect URL for payment completion
	PaymentURL string `json:"payment_url"`
	// Amount is the total amount due (server-derived)
	Amount int64 `json:"amount"`
}

// ============================================================================
// RESPONSE DTOs
// ============================================================================

// PackageResponse represents a promotion package in API responses.
type PackageResponse struct {
	ID                  uuid.UUID `json:"id"`
	Name                string    `json:"name"`
	TotalDurationHours  int       `json:"total_duration_hours"`
	ValidityWindowHours int       `json:"validity_window_hours"`
	PriceAmount         int       `json:"price_amount"`
	AllowedTargetTypes  []string  `json:"allowed_target_types"`
	IsActive            bool      `json:"is_active"`
	CreatedAt           string    `json:"created_at"`
}

// OwnershipResponse represents a promotion ownership in API responses.
type OwnershipResponse struct {
	ID                     uuid.UUID `json:"id"`
	UserID                 uuid.UUID `json:"user_id"`
	PackageID              uuid.UUID `json:"package_id"`
	Status                 string    `json:"status"`
	PurchasedAt            string    `json:"purchased_at"`
	ExpiresAt              string    `json:"expires_at"`
	TotalDurationHours     int       `json:"total_duration_hours"`
	ConsumedDurationHours  int       `json:"consumed_duration_hours"`
	RemainingDurationHours int       `json:"remaining_duration_hours"`
	CreatedAt              string    `json:"created_at"`
	UpdatedAt              string    `json:"updated_at"`
}

// InstanceResponse represents a promotion instance in API responses.
type InstanceResponse struct {
	ID          uuid.UUID  `json:"id"`
	OwnershipID uuid.UUID  `json:"ownership_id"`
	UserID      uuid.UUID  `json:"user_id"`
	TargetType  string     `json:"target_type"`
	TargetID    *uuid.UUID `json:"target_id"`
	Status      string     `json:"status"`
	ActivatedAt *string    `json:"activated_at,omitempty"`
	StoppedAt   *string    `json:"stopped_at,omitempty"`
	StopReason  *string    `json:"stop_reason,omitempty"`
	CreatedAt   string     `json:"created_at"`
	UpdatedAt   string     `json:"updated_at"`
}

// ============================================================================
// HTTP HANDLERS - PUBLIC (Authenticated)
// ============================================================================

// ListPackages handles GET /api/v1/promotions/packages
//
// Returns all available promotion packages.
func (h *PromotionHandler) ListPackages(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse query parameters
	includeInactive := c.DefaultQuery("include_inactive", "false") == "true"

	// List packages within a transaction
	var packages []*entity.PromotionPackage
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		packages, err = h.promotionService.ListPackages(ctx, tx, includeInactive)
		return err
	})

	if err != nil {
		h.log.Error("Failed to list promotion packages", zap.Error(err))
		response.InternalServerError(c, "Failed to retrieve packages")
		return
	}

	// Convert to response
	resp := make([]PackageResponse, len(packages))
	for i, pkg := range packages {
		targetTypes := make([]string, len(pkg.AllowedTargetTypes))
		for j, tt := range pkg.AllowedTargetTypes {
			targetTypes[j] = string(tt)
		}
		resp[i] = PackageResponse{
			ID:                  pkg.ID,
			Name:                pkg.Name,
			TotalDurationHours:  pkg.TotalDurationHours,
			ValidityWindowHours: pkg.ValidityWindowHours,
			PriceAmount:         pkg.PriceAmount,
			AllowedTargetTypes:  targetTypes,
			IsActive:            pkg.IsActive,
			CreatedAt:           pkg.CreatedAt.Format(time.RFC3339),
		}
	}

	response.Success(c, gin.H{
		"packages": resp,
	})
}

// PurchasePackage handles POST /api/v1/promotions/packages/purchase
//
// Initiates the purchase of a promotion package.
//
// PURCHASE TRUTH ENFORCEMENT:
// 1. Client provides ONLY package_id
// 2. Price is ALWAYS derived from server package data
// 3. Creates billing transaction + payment
// 4. Ownership is created ONLY after verified payment success (via webhook)
func (h *PromotionHandler) PurchasePackage(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context
	userID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	// Parse request
	var req PurchasePackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Get caller ID from context (same as user ID for this endpoint)
	callerID := userID

	// Get package to derive price
	var pkg *entity.PromotionPackage
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		packages, err := h.promotionService.ListPackages(ctx, tx, false)
		if err != nil {
			return err
		}
		// Find the specific package
		for _, p := range packages {
			if p.ID == req.PackageID {
				pkg = p
				return nil
			}
		}
		// Package not found
		pkg = nil
		return nil
	})

	if err != nil {
		h.log.Error("Failed to get package", zap.Error(err))
		response.InternalServerError(c, "Failed to retrieve package")
		return
	}

	if pkg == nil {
		response.NotFound(c, "Package not found")
		return
	}

	if !pkg.IsActive {
		response.BadRequest(c, "Package is not available for purchase")
		return
	}

	// SERVER-DERIVED PRICE: Amount comes from package, NOT client
	// PriceAmount is a Rupiah integer; no conversion needed.
	grossAmount := money.New(int64(pkg.PriceAmount))

	// Platform fee: typically 0% for promotion packages (optional)
	// The full amount goes to platform revenue
	platformFeePercent := int64(0)

	// Create billing transaction (creates payment intent)
	var billing *billingentity.BillingTransaction

	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		// Step 1: Create billing transaction
		billing, err = h.billingService.CreateBillingTransaction(
			ctx,
			tx,
			callerID,
			userID,
			req.PackageID, // Package ID stored as target_id
			billingentity.TypePromotionPackage,
			grossAmount,
			platformFeePercent,
		)
		if err != nil {
			return fmt.Errorf("failed to create billing: %w", err)
		}

		// Step 2: Payment initiation is handled by payment domain endpoint:
		// POST /api/v1/payments/billing with billing_id.
		// For this endpoint, we return billing_id so client can continue payment flow.

		return nil
	})

	if err != nil {
		h.log.Error("Failed to create purchase",
			zap.String("user_id", userID.String()),
			zap.String("package_id", req.PackageID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to create purchase")
		return
	}

	response.Success(c, gin.H{
		"billing_id": billing.ID,
		"message":    "Billing transaction created. Proceed to payment.",
		"amount":     grossAmount.Int64(),
	})
}

// ListMyOwnerships handles GET /api/v1/promotions/my/ownerships
//
// Returns the current user's promotion ownerships.
func (h *PromotionHandler) ListMyOwnerships(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context
	userID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	// Parse query parameters
	statusStr := c.DefaultQuery("status", "")
	var status entity.OwnershipStatus
	if statusStr != "" {
		status = entity.OwnershipStatus(statusStr)
	}

	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "20")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize

	// List ownerships within a transaction
	var ownerships []*entity.PromotionOwnership
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		ownerships, err = h.promotionService.ListMyOwnerships(ctx, tx, userID, status, pageSize, offset)
		return err
	})

	if err != nil {
		h.log.Error("Failed to list ownerships",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve ownerships")
		return
	}

	// Convert to response
	resp := make([]OwnershipResponse, len(ownerships))
	for i, o := range ownerships {
		resp[i] = ownershipToResponse(o)
	}

	response.Success(c, gin.H{
		"ownerships": resp,
		"page":       page,
		"page_size":  pageSize,
	})
}

// ListMyInstances handles GET /api/v1/promotions/my/instances
//
// Returns the current user's promotion instances.
func (h *PromotionHandler) ListMyInstances(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context
	userID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	// Parse query parameters
	statusStr := c.DefaultQuery("status", "")
	var status entity.InstanceStatus
	if statusStr != "" {
		status = entity.InstanceStatus(statusStr)
	}

	// List instances within a transaction
	var instances []*entity.PromotionInstance
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		instances, err = h.promotionService.ListMyInstances(ctx, tx, userID, status)
		return err
	})

	if err != nil {
		h.log.Error("Failed to list instances",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve instances")
		return
	}

	// Convert to response
	resp := make([]InstanceResponse, len(instances))
	for i, inst := range instances {
		resp[i] = instanceToResponse(inst)
	}

	response.Success(c, gin.H{
		"instances": resp,
	})
}

// GetOwnership handles GET /api/v1/promotions/ownerships/:id
//
// Returns a single ownership by ID.
func (h *PromotionHandler) GetOwnership(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context
	userID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	// Parse ownership ID
	ownershipID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid ownership ID")
		return
	}

	// Get ownership within a transaction
	var ownership *entity.PromotionOwnership
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		ownership, err = h.promotionService.GetOwnership(ctx, tx, ownershipID)
		return err
	})

	if err != nil {
		h.log.Error("Failed to get ownership",
			zap.String("ownership_id", ownershipID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve ownership")
		return
	}

	if ownership == nil {
		response.NotFound(c, "Ownership not found")
		return
	}

	// Verify ownership
	if ownership.UserID != userID {
		response.Forbidden(c, "Not your ownership")
		return
	}

	response.Success(c, ownershipToResponse(ownership))
}

// GetInstance handles GET /api/v1/promotions/instances/:id
//
// Returns a single instance by ID.
func (h *PromotionHandler) GetInstance(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context
	userID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	// Parse instance ID
	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid instance ID")
		return
	}

	// Get instance within a transaction
	var instance *entity.PromotionInstance
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		instance, err = h.promotionService.GetInstance(ctx, tx, instanceID)
		return err
	})

	if err != nil {
		h.log.Error("Failed to get instance",
			zap.String("instance_id", instanceID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve instance")
		return
	}

	if instance == nil {
		response.NotFound(c, "Instance not found")
		return
	}

	// Verify ownership
	if instance.UserID != userID {
		response.Forbidden(c, "Not your instance")
		return
	}

	response.Success(c, instanceToResponse(instance))
}

// ActivatePromotion handles POST /api/v1/promotions/activate
//
// Activates a promotion on a target.
func (h *PromotionHandler) ActivatePromotion(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context
	userID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	// Parse request
	var req ActivatePromotionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	targetType := entity.TargetType(req.TargetType)

	// Parse target_id if provided
	var targetID *uuid.UUID
	if req.TargetID != nil && *req.TargetID != "" {
		id, err := uuid.Parse(*req.TargetID)
		if err != nil {
			response.BadRequest(c, "Invalid target_id")
			return
		}
		targetID = &id
	}
	if targetType.RequiresTargetID() && targetID == nil {
		response.BadRequest(c, "target_id is required for this target_type")
		return
	}

	// MARKET AUTHORITY ENFORCEMENT (PHASE 1B):
	// Promoting fixed-price-sale/auction requires active seller subscription.
	// Expired sellers cannot promote their market objects.
	if targetType.RequiresTargetID() && targetID != nil {
		// For fixed-price sale/auction, verify the user is the seller AND has active subscription
		if targetType == entity.TargetTypeForSale {
			// Get fixed-price sale to verify ownership and check seller capability
			var sellerID uuid.UUID
			err = h.db.WithTx(ctx, func(tx db.Tx) error {
				return tx.QueryRow(ctx, `
					SELECT seller_id FROM for_sales WHERE id = $1 LIMIT 1
				`, targetID).Scan(&sellerID)
			})
			if err != nil {
				h.log.Error("Failed to verify fixed-price sale ownership",
					zap.String("for_sale_id", targetID.String()),
					zap.Error(err),
				)
				response.InternalServerError(c, "Failed to verify fixed-price sale")
				return
			}
			if sellerID != userID {
				response.Forbidden(c, "You can only promote your own fixed-price sales")
				return
			}
			// Check seller capability
			hasCapability, err := h.roleChecker.HasActiveSellerCapability(ctx, sellerID)
			if err != nil {
				h.log.Error("Failed to verify seller market authority",
					zap.String("seller_id", sellerID.String()),
					zap.Error(err),
				)
				response.InternalServerError(c, "Failed to verify seller authority")
				return
			}
			if !hasCapability {
				response.Forbidden(c, "Active seller subscription required to promote fixed-price sales")
				return
			}
		} else if targetType == entity.TargetTypeAuction {
			// Get auction to verify ownership and check seller capability
			var sellerID uuid.UUID
			err = h.db.WithTx(ctx, func(tx db.Tx) error {
				return tx.QueryRow(ctx, `
					SELECT seller_id FROM auctions WHERE id = $1 LIMIT 1
				`, targetID).Scan(&sellerID)
			})
			if err != nil {
				h.log.Error("Failed to verify auction ownership",
					zap.String("auction_id", targetID.String()),
					zap.Error(err),
				)
				response.InternalServerError(c, "Failed to verify auction")
				return
			}
			if sellerID != userID {
				response.Forbidden(c, "You can only promote your own auctions")
				return
			}
			// Check seller capability
			hasCapability, err := h.roleChecker.HasActiveSellerCapability(ctx, sellerID)
			if err != nil {
				h.log.Error("Failed to verify seller market authority",
					zap.String("seller_id", sellerID.String()),
					zap.Error(err),
				)
				response.InternalServerError(c, "Failed to verify seller authority")
				return
			}
			if !hasCapability {
				response.Forbidden(c, "Active seller subscription required to promote auctions")
				return
			}
		} else if targetType == entity.TargetTypeExternalProduct {
			// Get external product to verify ownership and check seller capability
			var ownerUserID uuid.UUID
			err = h.db.WithTx(ctx, func(tx db.Tx) error {
				return tx.QueryRow(ctx, `
					SELECT owner_user_id FROM external_products WHERE id = $1 AND deleted_at IS NULL LIMIT 1
				`, targetID).Scan(&ownerUserID)
			})
			if err != nil {
				h.log.Error("Failed to verify external product ownership",
					zap.String("external_product_id", targetID.String()),
					zap.Error(err),
				)
				response.InternalServerError(c, "Failed to verify external product")
				return
			}
			if ownerUserID != userID {
				response.Forbidden(c, "You can only promote your own external products")
				return
			}
			// Check seller capability
			hasCapability, err := h.roleChecker.HasActiveSellerCapability(ctx, ownerUserID)
			if err != nil {
				h.log.Error("Failed to verify seller market authority",
					zap.String("user_id", ownerUserID.String()),
					zap.Error(err),
				)
				response.InternalServerError(c, "Failed to verify seller authority")
				return
			}
			if !hasCapability {
				response.Forbidden(c, "Active seller subscription required to promote external products")
				return
			}
		}
	}

	// Execute within transaction
	var result *promotionApp.ActivatePromotionResult
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		result, err = h.promotionService.ActivatePromotion(ctx, tx, promotionApp.ActivatePromotionInput{
			OwnershipID: req.OwnershipID,
			UserID:      userID,
			TargetType:  targetType,
			TargetID:    targetID,
		})
		return err
	})

	// Handle specific errors
	if handlePromotionError(c, err) {
		return
	}

	response.Success(c, gin.H{
		"instance":  instanceToResponse(result.Instance),
		"ownership": ownershipToResponse(result.Ownership),
	})
}

// DeactivatePromotion handles POST /api/v1/promotions/instances/:id/deactivate
//
// Deactivates an active promotion.
// If reason is "user_paused", the instance is paused (non-terminal, resumable).
// If reason is "user_cancelled", the instance is stopped (terminal, finalized).
func (h *PromotionHandler) DeactivatePromotion(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context
	userID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	// Parse instance ID
	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid instance ID")
		return
	}

	// Parse request
	var req DeactivatePromotionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Map reason string to StopReason
	var reason entity.StopReason
	switch req.Reason {
	case "user_paused":
		reason = entity.StopReasonUserPaused
	case "user_cancelled":
		reason = entity.StopReasonUserCancelled
	default:
		reason = entity.StopReasonUserCancelled
	}

	// Execute within transaction
	// DeactivatePromotion internally delegates to PausePromotion for user_paused
	// and to stopAndFinalizeInstance for user_cancelled
	var updatedInstance *entity.PromotionInstance
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		txErr := h.promotionService.DeactivatePromotion(ctx, tx, promotionApp.DeactivatePromotionInput{
			InstanceID: instanceID,
			UserID:     userID,
			Reason:     reason,
		})
		if txErr != nil {
			return txErr
		}
		// Re-read to get updated state for response
		updatedInstance, txErr = h.promotionService.GetInstance(ctx, tx, instanceID)
		return txErr
	})

	// Handle specific errors
	if handlePromotionError(c, err) {
		return
	}

	if updatedInstance != nil {
		response.Success(c, gin.H{
			"instance": instanceToResponse(updatedInstance),
		})
	} else {
		response.Success(c, gin.H{
			"message": "Promotion deactivated successfully",
		})
	}
}

// ResumePromotion handles POST /api/v1/promotions/instances/:id/resume
//
// Resumes a paused promotion. Re-checks target operability before resume.
func (h *PromotionHandler) ResumePromotion(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context
	userID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	// Parse instance ID
	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid instance ID")
		return
	}

	// Execute within transaction
	var result *promotionApp.ResumePromotionResult
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var txErr error
		result, txErr = h.promotionService.ResumePromotion(ctx, tx, promotionApp.ResumePromotionInput{
			InstanceID: instanceID,
			UserID:     userID,
		})
		return txErr
	})

	// Handle specific errors
	if handlePromotionError(c, err) {
		return
	}

	response.Success(c, gin.H{
		"instance": instanceToResponse(result.Instance),
	})
}

// ReassignPromotion handles POST /api/v1/promotions/instances/:id/reassign
//
// Reassigns a promotion to a new target.
func (h *PromotionHandler) ReassignPromotion(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context
	userID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	// Parse instance ID
	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid instance ID")
		return
	}

	// Parse request
	var req ReassignPromotionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	newTargetType := entity.TargetType(req.NewTargetType)

	// Parse new_target_id if provided
	var newTargetID *uuid.UUID
	if req.NewTargetID != nil && *req.NewTargetID != "" {
		id, err := uuid.Parse(*req.NewTargetID)
		if err != nil {
			response.BadRequest(c, "Invalid new_target_id")
			return
		}
		newTargetID = &id
	}
	if newTargetType.RequiresTargetID() && newTargetID == nil {
		response.BadRequest(c, "new_target_id is required for this target_type")
		return
	}

	// MARKET AUTHORITY ENFORCEMENT (PHASE 1B):
	// Reassigning promotion to fixed-price-sale/auction requires active seller subscription.
	if newTargetType.RequiresTargetID() && newTargetID != nil {
		if newTargetType == entity.TargetTypeForSale {
			// Get fixed-price sale to verify ownership and check seller capability
			var sellerID uuid.UUID
			err = h.db.WithTx(ctx, func(tx db.Tx) error {
				return tx.QueryRow(ctx, `
					SELECT seller_id FROM for_sales WHERE id = $1 LIMIT 1
				`, newTargetID).Scan(&sellerID)
			})
			if err != nil {
				h.log.Error("Failed to verify fixed-price sale ownership",
					zap.String("for_sale_id", newTargetID.String()),
					zap.Error(err),
				)
				response.InternalServerError(c, "Failed to verify fixed-price sale")
				return
			}
			if sellerID != userID {
				response.Forbidden(c, "You can only promote your own fixed-price sales")
				return
			}
			// Check seller capability
			hasCapability, err := h.roleChecker.HasActiveSellerCapability(ctx, sellerID)
			if err != nil {
				h.log.Error("Failed to verify seller market authority",
					zap.String("seller_id", sellerID.String()),
					zap.Error(err),
				)
				response.InternalServerError(c, "Failed to verify seller authority")
				return
			}
			if !hasCapability {
				response.Forbidden(c, "Active seller subscription required to promote fixed-price sales")
				return
			}
		} else if newTargetType == entity.TargetTypeAuction {
			// Get auction to verify ownership and check seller capability
			var sellerID uuid.UUID
			err = h.db.WithTx(ctx, func(tx db.Tx) error {
				return tx.QueryRow(ctx, `
					SELECT seller_id FROM auctions WHERE id = $1 LIMIT 1
				`, newTargetID).Scan(&sellerID)
			})
			if err != nil {
				h.log.Error("Failed to verify auction ownership",
					zap.String("auction_id", newTargetID.String()),
					zap.Error(err),
				)
				response.InternalServerError(c, "Failed to verify auction")
				return
			}
			if sellerID != userID {
				response.Forbidden(c, "You can only promote your own auctions")
				return
			}
			// Check seller capability
			hasCapability, err := h.roleChecker.HasActiveSellerCapability(ctx, sellerID)
			if err != nil {
				h.log.Error("Failed to verify seller market authority",
					zap.String("seller_id", sellerID.String()),
					zap.Error(err),
				)
				response.InternalServerError(c, "Failed to verify seller authority")
				return
			}
			if !hasCapability {
				response.Forbidden(c, "Active seller subscription required to promote auctions")
				return
			}
		} else if newTargetType == entity.TargetTypeExternalProduct {
			// Get external product to verify ownership and check seller capability
			var ownerUserID uuid.UUID
			err = h.db.WithTx(ctx, func(tx db.Tx) error {
				return tx.QueryRow(ctx, `
					SELECT owner_user_id FROM external_products WHERE id = $1 AND deleted_at IS NULL LIMIT 1
				`, newTargetID).Scan(&ownerUserID)
			})
			if err != nil {
				h.log.Error("Failed to verify external product ownership",
					zap.String("external_product_id", newTargetID.String()),
					zap.Error(err),
				)
				response.InternalServerError(c, "Failed to verify external product")
				return
			}
			if ownerUserID != userID {
				response.Forbidden(c, "You can only promote your own external products")
				return
			}
			// Check seller capability
			hasCapability, err := h.roleChecker.HasActiveSellerCapability(ctx, ownerUserID)
			if err != nil {
				h.log.Error("Failed to verify seller market authority",
					zap.String("user_id", ownerUserID.String()),
					zap.Error(err),
				)
				response.InternalServerError(c, "Failed to verify seller authority")
				return
			}
			if !hasCapability {
				response.Forbidden(c, "Active seller subscription required to promote external products")
				return
			}
		}
	}

	// Execute within transaction
	var result *promotionApp.ReassignPromotionResult
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		result, err = h.promotionService.ReassignPromotion(ctx, tx, promotionApp.ReassignPromotionInput{
			InstanceID:    instanceID,
			UserID:        userID,
			NewTargetType: newTargetType,
			NewTargetID:   newTargetID,
		})
		return err
	})

	// Handle specific errors
	if handlePromotionError(c, err) {
		return
	}

	response.Success(c, gin.H{
		"instance":  instanceToResponse(result.NewInstance),
		"ownership": ownershipToResponse(result.Ownership),
	})
}

// ============================================================================
// HTTP HANDLERS - DISCOVERY (Public - for Search, Home surfaces)
// ============================================================================

// PromotedItemResponse represents a promoted item for discovery surfaces.
// This is a simplified view for external consumption by search/home.
type PromotedItemResponse struct {
	InstanceID       string  `json:"instance_id"`
	TargetType       string  `json:"target_type"`
	TargetID         *string `json:"target_id,omitempty"`
	Title            *string `json:"title,omitempty"`
	Description      *string `json:"description,omitempty"`
	ExternalURL      *string `json:"external_url,omitempty"`
	ExternalMediaURL *string `json:"external_media_url,omitempty"`
	MediaType        *string `json:"media_type,omitempty"`
	ThumbnailURL     *string `json:"thumbnail_url,omitempty"`
	SellerUsername   *string `json:"seller_username,omitempty"`
	SellerFarmName   *string `json:"seller_farm_name,omitempty"`
	SellerLifecycle  *string `json:"seller_lifecycle,omitempty"`
	Promoted         bool    `json:"promoted"`
}

// GetPromotedItems handles GET /api/v1/promotions/discover
//
// Returns active promoted items for discovery surfaces (search, home, etc.).
// This endpoint:
// - Only returns active instances with operable targets
// - Returns simple, minimal data needed for discovery
func (h *PromotionHandler) GetPromotedItems(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse query parameters
	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 50 {
		limit = 10
	}

	// Get promoted items via discovery service
	dbConn, ok := h.db.(*db.DB)
	if !ok {
		response.InternalServerError(c, "Promotion discovery database is not configured")
		return
	}

	instances, err := h.promotionService.GetPromotedItemsForDiscovery(ctx, dbConn, limit)
	if err != nil {
		h.log.Error("Failed to get promoted items for discovery", zap.Error(err))
		response.InternalServerError(c, "Failed to retrieve promoted items")
		return
	}

	resp := make([]PromotedItemResponse, 0, len(instances))
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		for _, inst := range instances {
			item, buildErr := h.promotedItemResponseForDiscovery(ctx, tx, inst)
			if buildErr != nil {
				continue
			}
			resp = append(resp, item)
		}
		return nil
	})
	if err != nil {
		h.log.Error("Failed to build promoted discovery response", zap.Error(err))
		response.InternalServerError(c, "Failed to retrieve promoted items")
		return
	}

	response.Success(c, gin.H{
		"promoted_items": resp,
		"count":          len(resp),
	})
}

// GetPromotedItemsByTarget handles GET /api/v1/promotions/discover/:target_type
//
// Returns promoted items filtered by public discovery target type.
func (h *PromotionHandler) GetPromotedItemsByTarget(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse target type
	targetTypeStr := c.Param("target_type")
	var queryTargetType entity.TargetType
	switch targetTypeStr {
	case "for_sale":
		queryTargetType = entity.TargetTypeForSale
	case string(entity.TargetTypeAuction):
		queryTargetType = entity.TargetTypeAuction
	case string(entity.TargetTypeExternalProduct):
		queryTargetType = entity.TargetTypeExternalProduct
	default:
		response.BadRequest(c, "Invalid target_type. Must be: for_sale, auction, or external_product")
		return
	}

	// Parse query parameters
	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 50 {
		limit = 10
	}

	// Get promoted items via discovery service
	dbConn, ok := h.db.(*db.DB)
	if !ok {
		response.InternalServerError(c, "Promotion discovery database is not configured")
		return
	}

	instances, err := h.promotionService.GetPromotedItemsByTargetForDiscovery(ctx, dbConn, queryTargetType, limit)
	if err != nil {
		h.log.Error("Failed to get promoted items by target", zap.Error(err))
		response.InternalServerError(c, "Failed to retrieve promoted items")
		return
	}

	resp := make([]PromotedItemResponse, 0, len(instances))
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		for _, inst := range instances {
			item, buildErr := h.promotedItemResponseForDiscovery(ctx, tx, inst)
			if buildErr != nil {
				continue
			}
			resp = append(resp, item)
		}
		return nil
	})
	if err != nil {
		h.log.Error("Failed to build promoted discovery response", zap.Error(err))
		response.InternalServerError(c, "Failed to retrieve promoted items")
		return
	}

	response.Success(c, gin.H{
		"promoted_items": resp,
		"count":          len(resp),
	})
}

func (h *PromotionHandler) promotedItemResponseForDiscovery(
	ctx context.Context,
	tx db.Tx,
	inst *entity.PromotionInstance,
) (PromotedItemResponse, error) {
	if inst == nil {
		return PromotedItemResponse{}, fmt.Errorf("promotion instance is nil")
	}

	resp := PromotedItemResponse{
		InstanceID: inst.ID.String(),
		TargetType: func() string {
			switch inst.TargetType {
			case entity.TargetTypeForSale:
				return "for_sale"
			default:
				return string(inst.TargetType)
			}
		}(),
		TargetID: uuidPtrToStr(inst.TargetID),
		Promoted: true,
	}

	switch inst.TargetType {
	case entity.TargetTypeExternalProduct:
		if inst.TargetID == nil {
			return PromotedItemResponse{}, fmt.Errorf("external product target_id is nil")
		}
		product, err := h.promotionService.GetExternalProduct(ctx, tx, *inst.TargetID)
		if err != nil || product == nil {
			return PromotedItemResponse{}, fmt.Errorf("external product not found")
		}
		media, err := h.promotionService.ListExternalProductMedia(ctx, tx, product.ID)
		if err != nil {
			return PromotedItemResponse{}, err
		}
		title := product.Title
		resp.Title = &title
		resp.Description = product.Description
		externalURL := product.NormalizedExternalURL
		resp.ExternalURL = &externalURL
		if mediaURL, mediaType, thumbnailURL := firstPublicExternalProductMedia(media); mediaURL != nil {
			resp.ExternalMediaURL = mediaURL
			resp.MediaType = mediaType
			resp.ThumbnailURL = thumbnailURL
		}
	case entity.TargetTypeForSale, entity.TargetTypeAuction:
		// fixed-price-sale/auction discovery does not use inline external fields
	default:
		return PromotedItemResponse{}, fmt.Errorf("unsupported target type: %s", inst.TargetType)
	}

	return resp, nil
}

// uuidPtrToStr converts a UUID pointer to string pointer, returns nil if input is nil.
func uuidPtrToStr(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	str := id.String()
	return &str
}

func firstPublicExternalProductMedia(media []*entity.ExternalProductMedia) (*string, *string, *string) {
	for _, item := range media {
		if item == nil {
			continue
		}
		if item.URL == "" {
			continue
		}
		mediaURL := item.URL
		mediaType := string(item.MediaType)
		var thumbnailURL *string
		if item.ThumbnailURL != nil {
			value := *item.ThumbnailURL
			thumbnailURL = &value
		}
		return &mediaURL, &mediaType, thumbnailURL
	}
	return nil, nil, nil
}

// ============================================================================
// ANALYTICS / TRACKING
// ============================================================================

// RecordEventRequest is the request body for POST /api/v1/promotions/events.
type RecordEventRequest struct {
	PromotionInstanceID string `json:"promotion_instance_id" binding:"required"`
	EventType           string `json:"event_type"           binding:"required"`
	Surface             string `json:"surface"              binding:"required"`
}

// CampaignAnalyticsResponse is the minimal promotion analytics summary read model.
type CampaignAnalyticsResponse struct {
	InstanceID         uuid.UUID `json:"instance_id"`
	WindowFrom         *string   `json:"window_from,omitempty"`
	WindowTo           *string   `json:"window_to,omitempty"`
	ImpressionsTotal   int       `json:"impressions_total"`
	ClicksTotal        int       `json:"clicks_total"`
	CTR                float64   `json:"ctr"`
	FeedImpressions    int       `json:"feed_impressions"`
	FeedClicks         int       `json:"feed_clicks"`
	SearchImpressions  int       `json:"search_impressions"`
	SearchClicks       int       `json:"search_clicks"`
	ExploreImpressions int       `json:"explore_impressions"`
	ExploreClicks      int       `json:"explore_clicks"`
}

// RecordEvent handles POST /api/v1/promotions/events.
//
// Records a promotion analytics event (e.g. a viewer click on a promoted card).
// The endpoint is analytics-only — it has no effect on finance, escrow, or promotion lifecycle.
//
// Auth: required (all promotion surfaces are authenticated).
// Response: 204 No Content on success, 400 on invalid input.
//
// Instance validation: if the promotion_instance_id does not exist in the database,
// the request is silently accepted (204) so stale mobile state does not error.
//
// Metadata (target_type, target_id, owner_user_id) is resolved server-side
// from the promotion instance — clients cannot forge attribution.
func (h *PromotionHandler) RecordEvent(c *gin.Context) {
	if h.eventRepo == nil {
		// Tracking disabled (not wired); accept silently.
		c.Status(204)
		return
	}

	viewerID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req RecordEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	instanceID, err := uuid.Parse(req.PromotionInstanceID)
	if err != nil {
		response.BadRequest(c, "Invalid promotion_instance_id")
		return
	}

	eventType := entity.PromotionEventType(req.EventType)
	if !eventType.IsValid() {
		response.BadRequest(c, "Invalid event_type")
		return
	}

	surface := entity.PromotionEventSurface(req.Surface)
	if !surface.IsValid() {
		response.BadRequest(c, "Invalid surface")
		return
	}

	ctx := c.Request.Context()

	// Validate instance exists and resolve metadata server-side.
	// Use a read-only transaction; no locks needed for analytics.
	var inst *entity.PromotionInstance
	if txErr := h.db.WithTx(ctx, func(tx db.Tx) error {
		var e error
		inst, e = h.promotionService.GetInstance(ctx, tx, instanceID)
		return e
	}); txErr != nil {
		// Instance not found → silent 204 (stale mobile state is not an error).
		h.log.Debug("promotion RecordEvent: instance lookup failed, accepting silently",
			zap.String("instance_id", instanceID.String()),
			zap.Error(txErr),
		)
		c.Status(204)
		return
	}

	ev, err := entity.NewPromotionEvent(inst, viewerID, eventType, surface)
	if err != nil {
		// Should not happen given prior validation, but guard defensively.
		h.log.Warn("promotion RecordEvent: event construction failed", zap.Error(err))
		response.BadRequest(c, "Event construction failed")
		return
	}

	if txErr := h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.eventRepo.RecordEvent(ctx, tx, ev)
	}); txErr != nil {
		h.log.Error("promotion RecordEvent: insert failed",
			zap.String("instance_id", instanceID.String()),
			zap.Error(txErr),
		)
		response.InternalServerError(c, "Failed to record event")
		return
	}

	c.Status(204)
}

// AdminGetCampaignAnalytics handles GET /api/v1/admin/promotions/campaigns/:id/analytics.
// Capability gate is enforced at the route layer (promotion.campaign.view).
func (h *PromotionHandler) AdminGetCampaignAnalytics(c *gin.Context) {
	if h.eventRepo == nil {
		response.ServiceUnavailable(c, "Promotion analytics is not wired")
		return
	}

	campaignID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid campaign ID")
		return
	}

	from, to, err := parsePromotionAnalyticsWindow(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	ctx := c.Request.Context()

	var exists int
	if txErr := h.db.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT 1
			FROM promotion_instances
			WHERE id = $1
		`, campaignID).Scan(&exists)
	}); txErr != nil {
		if txErr == pgx.ErrNoRows {
			response.NotFound(c, "Campaign not found")
			return
		}
		h.log.Error("promotion analytics: failed to verify campaign",
			zap.String("campaign_id", campaignID.String()),
			zap.Error(txErr),
		)
		response.InternalServerError(c, "Failed to retrieve campaign analytics")
		return
	}

	var summary *promotionRepo.PromotionEventAnalyticsSummary
	if txErr := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		summary, err = h.eventRepo.GetCampaignAnalytics(ctx, tx, campaignID, from, to)
		return err
	}); txErr != nil {
		h.log.Error("promotion analytics: failed to aggregate campaign metrics",
			zap.String("campaign_id", campaignID.String()),
			zap.Error(txErr),
		)
		response.InternalServerError(c, "Failed to retrieve campaign analytics")
		return
	}

	response.Success(c, gin.H{
		"analytics": campaignAnalyticsSummaryToResponse(summary),
	})
}

func parsePromotionAnalyticsWindow(c *gin.Context) (*time.Time, *time.Time, error) {
	var from *time.Time
	if raw := strings.TrimSpace(c.Query("from")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid from timestamp")
		}
		from = &parsed
	}

	var to *time.Time
	if raw := strings.TrimSpace(c.Query("to")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid to timestamp")
		}
		to = &parsed
	}

	if from != nil && to != nil && from.After(*to) {
		return nil, nil, fmt.Errorf("from must be before or equal to to")
	}

	return from, to, nil
}

func campaignAnalyticsSummaryToResponse(summary *promotionRepo.PromotionEventAnalyticsSummary) CampaignAnalyticsResponse {
	resp := CampaignAnalyticsResponse{
		InstanceID:         summary.InstanceID,
		ImpressionsTotal:   summary.ImpressionsTotal,
		ClicksTotal:        summary.ClicksTotal,
		CTR:                summary.CTR,
		FeedImpressions:    summary.FeedImpressions,
		FeedClicks:         summary.FeedClicks,
		SearchImpressions:  summary.SearchImpressions,
		SearchClicks:       summary.SearchClicks,
		ExploreImpressions: summary.ExploreImpressions,
		ExploreClicks:      summary.ExploreClicks,
	}
	if summary.WindowFrom != nil {
		value := summary.WindowFrom.UTC().Format(time.RFC3339)
		resp.WindowFrom = &value
	}
	if summary.WindowTo != nil {
		value := summary.WindowTo.UTC().Format(time.RFC3339)
		resp.WindowTo = &value
	}
	return resp
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// getUserID extracts the user ID from the gin context.
func (h *PromotionHandler) getUserID(c *gin.Context) (uuid.UUID, error) {
	return middleware.GetUserIDFromContext(c)
}

// instanceToResponse converts an entity to a response DTO.
func instanceToResponse(inst *entity.PromotionInstance) InstanceResponse {
	var activatedAt, stoppedAt *string
	if inst.ActivatedAt != nil {
		s := inst.ActivatedAt.Format(time.RFC3339)
		activatedAt = &s
	}
	if inst.StoppedAt != nil {
		s := inst.StoppedAt.Format(time.RFC3339)
		stoppedAt = &s
	}

	return InstanceResponse{
		ID:          inst.ID,
		OwnershipID: inst.OwnershipID,
		UserID:      inst.UserID,
		TargetType:  string(inst.TargetType),
		TargetID:    inst.TargetID,
		Status:      string(inst.Status),
		ActivatedAt: activatedAt,
		StoppedAt:   stoppedAt,
		StopReason:  inst.StopReason,
		CreatedAt:   inst.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   inst.UpdatedAt.Format(time.RFC3339),
	}
}

// ownershipToResponse converts an entity to a response DTO.
func ownershipToResponse(o *entity.PromotionOwnership) OwnershipResponse {
	return OwnershipResponse{
		ID:                     o.ID,
		UserID:                 o.UserID,
		PackageID:              o.PackageID,
		Status:                 string(o.Status),
		PurchasedAt:            o.PurchasedAt.Format(time.RFC3339),
		ExpiresAt:              o.ExpiresAt.Format(time.RFC3339),
		TotalDurationHours:     o.TotalDurationHours,
		ConsumedDurationHours:  o.ConsumedDurationHours,
		RemainingDurationHours: o.GetRemainingDuration(),
		CreatedAt:              o.CreatedAt.Format(time.RFC3339),
		UpdatedAt:              o.UpdatedAt.Format(time.RFC3339),
	}
}

// handlePromotionError handles promotion-specific errors and returns true if an error was handled.
func handlePromotionError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}

	// Check for specific error types and return appropriate responses
	switch e := err.(type) {
	case *promotionApp.OwnershipNotFoundError:
		response.NotFound(c, e.Error())
		return true
	case *promotionApp.InstanceNotFoundError:
		response.NotFound(c, e.Error())
		return true
	case *promotionApp.PackageNotFoundError:
		response.NotFound(c, e.Error())
		return true
	case *promotionApp.PackageInactiveError:
		response.BadRequest(c, e.Error())
		return true
	case *promotionApp.OwnershipNotAvailableError:
		response.BadRequest(c, e.Error())
		return true
	case *promotionApp.InstanceAlreadyStoppedError:
		response.BadRequest(c, e.Error())
		return true
	case *promotionApp.InstanceNotPausedError:
		response.BadRequest(c, e.Error())
		return true
	case *promotionApp.NotInstanceOwnerError:
		response.Forbidden(c, e.Error())
		return true
	case *promotionApp.TargetTypeNotAllowedError:
		response.BadRequest(c, e.Error())
		return true
	case *promotionApp.TargetNotOperableError:
		response.BadRequest(c, e.Error())
		return true
	case *promotionApp.ExternalProductPromotionReviewRequiredError:
		response.Forbidden(c, e.Error())
		return true
	case *entity.OwnershipExpiredError:
		response.BadRequest(c, "Ownership has expired")
		return true
	case *entity.OwnershipConsumedError:
		response.BadRequest(c, "Ownership duration is fully consumed")
		return true
	case *entity.NotOwnershipOwnerError:
		response.Forbidden(c, e.Error())
		return true
	case *entity.InvalidTargetTypeError:
		response.BadRequest(c, e.Error())
		return true
	case *entity.ValidationError:
		response.BadRequest(c, e.Error())
		return true
	default:
		// Log unexpected errors but don't return specific message
		// The caller should handle logging
		return false
	}
}

// ============================================================================
// ADMIN HANDLERS — PROMOTION PACKAGES
// Capability: promotion.package.manage
// ============================================================================

// AdminListPackages handles GET /api/v1/admin/promotions/packages
//
// Lists all packages including inactive ones (admin view).
func (h *PromotionHandler) AdminListPackages(c *gin.Context) {
	ctx := c.Request.Context()

	var packages []*entity.PromotionPackage
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		// Admin always sees all packages including inactive
		packages, err = h.promotionService.ListPackages(ctx, tx, true)
		return err
	})
	if err != nil {
		h.log.Error("Admin: failed to list promotion packages", zap.Error(err))
		response.InternalServerError(c, "Failed to retrieve packages")
		return
	}

	resp := make([]PackageResponse, len(packages))
	for i, pkg := range packages {
		resp[i] = packageToResponse(pkg)
	}
	response.Success(c, gin.H{"packages": resp})
}

// AdminCreatePackageRequest is the request body for creating a package.
type AdminCreatePackageRequest struct {
	Name                string   `json:"name" binding:"required"`
	TotalDurationHours  int      `json:"total_duration_hours" binding:"required,min=1"`
	ValidityWindowHours int      `json:"validity_window_hours" binding:"required,min=1"`
	PriceAmount         int      `json:"price_amount" binding:"min=0"`
	AllowedTargetTypes  []string `json:"allowed_target_types" binding:"required,min=1"`
}

// AdminCreatePackage handles POST /api/v1/admin/promotions/packages
func (h *PromotionHandler) AdminCreatePackage(c *gin.Context) {
	ctx := c.Request.Context()

	adminID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req AdminCreatePackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	targetTypes := make([]entity.TargetType, len(req.AllowedTargetTypes))
	for i, tt := range req.AllowedTargetTypes {
		targetTypes[i] = entity.TargetType(tt)
	}

	var pkg *entity.PromotionPackage
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var txErr error
		pkg, txErr = h.promotionService.AdminCreatePackage(ctx, tx, promotionApp.CreatePackageInput{
			Name:                req.Name,
			TotalDurationHours:  req.TotalDurationHours,
			ValidityWindowHours: req.ValidityWindowHours,
			PriceAmount:         req.PriceAmount,
			AllowedTargetTypes:  targetTypes,
		})
		return txErr
	})
	if handlePromotionError(c, err) {
		return
	}
	if err != nil {
		h.log.Error("Admin: failed to create promotion package", zap.Error(err))
		response.InternalServerError(c, "Failed to create package")
		return
	}

	if h.adminAuditLogger != nil {
		h.adminAuditLogger.LogSafe(ctx, adminID, "promotion_package.created", "promotion_package", pkg.ID, map[string]interface{}{
			"name": pkg.Name,
		})
	}
	response.Success(c, gin.H{"package": packageToResponse(pkg)})
}

// AdminUpdatePackageRequest is the request body for updating a package.
type AdminUpdatePackageRequest struct {
	Name                string   `json:"name" binding:"required"`
	TotalDurationHours  int      `json:"total_duration_hours" binding:"required,min=1"`
	ValidityWindowHours int      `json:"validity_window_hours" binding:"required,min=1"`
	PriceAmount         int      `json:"price_amount" binding:"min=0"`
	AllowedTargetTypes  []string `json:"allowed_target_types" binding:"required,min=1"`
	IsActive            bool     `json:"is_active"`
}

// AdminUpdatePackage handles PATCH /api/v1/admin/promotions/packages/:id
func (h *PromotionHandler) AdminUpdatePackage(c *gin.Context) {
	ctx := c.Request.Context()

	adminID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	pkgID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid package ID")
		return
	}

	var req AdminUpdatePackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	targetTypes := make([]entity.TargetType, len(req.AllowedTargetTypes))
	for i, tt := range req.AllowedTargetTypes {
		targetTypes[i] = entity.TargetType(tt)
	}

	var pkg *entity.PromotionPackage
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var txErr error
		pkg, txErr = h.promotionService.AdminUpdatePackage(ctx, tx, pkgID, promotionApp.UpdatePackageInput{
			Name:                req.Name,
			TotalDurationHours:  req.TotalDurationHours,
			ValidityWindowHours: req.ValidityWindowHours,
			PriceAmount:         req.PriceAmount,
			AllowedTargetTypes:  targetTypes,
			IsActive:            req.IsActive,
		})
		return txErr
	})
	if handlePromotionError(c, err) {
		return
	}
	if err != nil {
		h.log.Error("Admin: failed to update promotion package", zap.String("package_id", pkgID.String()), zap.Error(err))
		response.InternalServerError(c, "Failed to update package")
		return
	}

	if h.adminAuditLogger != nil {
		h.adminAuditLogger.LogSafe(ctx, adminID, "promotion_package.updated", "promotion_package", pkg.ID, map[string]interface{}{
			"name":      pkg.Name,
			"is_active": pkg.IsActive,
		})
	}
	response.Success(c, gin.H{"package": packageToResponse(pkg)})
}

// AdminEnablePackage handles POST /api/v1/admin/promotions/packages/:id/enable
func (h *PromotionHandler) AdminEnablePackage(c *gin.Context) {
	h.adminSetPackageActive(c, true)
}

// AdminDisablePackage handles POST /api/v1/admin/promotions/packages/:id/disable
func (h *PromotionHandler) AdminDisablePackage(c *gin.Context) {
	h.adminSetPackageActive(c, false)
}

func (h *PromotionHandler) adminSetPackageActive(c *gin.Context, active bool) {
	ctx := c.Request.Context()

	adminID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	pkgID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid package ID")
		return
	}

	var pkg *entity.PromotionPackage
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var txErr error
		pkg, txErr = h.promotionService.AdminSetPackageActive(ctx, tx, pkgID, active)
		return txErr
	})
	if handlePromotionError(c, err) {
		return
	}
	if err != nil {
		h.log.Error("Admin: failed to set package active state",
			zap.String("package_id", pkgID.String()),
			zap.Bool("active", active),
			zap.Error(err))
		response.InternalServerError(c, "Failed to update package")
		return
	}

	action := "promotion_package.enabled"
	if !active {
		action = "promotion_package.disabled"
	}
	if h.adminAuditLogger != nil {
		h.adminAuditLogger.LogSafe(ctx, adminID, action, "promotion_package", pkg.ID, nil)
	}
	response.Success(c, gin.H{"package": packageToResponse(pkg)})
}

// ============================================================================
// ADMIN HANDLERS — PROMOTION CAMPAIGNS
// Capability: promotion.campaign.view / promotion.campaign.stop
// ============================================================================

// AdminCampaignResponse is the response DTO for a campaign row.
type AdminCampaignResponse struct {
	InstanceResponse
	PackageID              uuid.UUID `json:"package_id"`
	PackageName            string    `json:"package_name"`
	OwnershipTotalHours    int       `json:"ownership_total_hours"`
	OwnershipConsumedHours int       `json:"ownership_consumed_hours"`
}

// AdminListCampaigns handles GET /api/v1/admin/promotions/campaigns
func (h *PromotionHandler) AdminListCampaigns(c *gin.Context) {
	ctx := c.Request.Context()

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	var ownerUserID *uuid.UUID
	if ownerStr := c.Query("owner_user_id"); ownerStr != "" {
		id, err := uuid.Parse(ownerStr)
		if err != nil {
			response.BadRequest(c, "Invalid owner_user_id")
			return
		}
		ownerUserID = &id
	}

	var packageID *uuid.UUID
	if pkgStr := c.Query("package_id"); pkgStr != "" {
		id, err := uuid.Parse(pkgStr)
		if err != nil {
			response.BadRequest(c, "Invalid package_id")
			return
		}
		packageID = &id
	}

	filter := promotionRepo.AdminCampaignFilter{
		Status:      c.Query("status"),
		TargetType:  c.Query("target_type"),
		OwnerUserID: ownerUserID,
		PackageID:   packageID,
		Limit:       limit,
		Offset:      offset,
	}

	var rows []*promotionRepo.AdminCampaignRow
	var total int
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var txErr error
		rows, total, txErr = h.promotionService.AdminListCampaigns(ctx, tx, filter)
		return txErr
	})
	if err != nil {
		h.log.Error("Admin: failed to list campaigns", zap.Error(err))
		response.InternalServerError(c, "Failed to retrieve campaigns")
		return
	}

	resp := make([]AdminCampaignResponse, len(rows))
	for i, row := range rows {
		resp[i] = AdminCampaignResponse{
			InstanceResponse:       instanceToResponse(row.Instance),
			PackageID:              row.PackageID,
			PackageName:            row.PackageName,
			OwnershipTotalHours:    row.OwnershipTotalHours,
			OwnershipConsumedHours: row.OwnershipConsumedHours,
		}
	}

	response.Success(c, gin.H{
		"campaigns": resp,
		"total":     total,
		"limit":     limit,
		"offset":    offset,
	})
}

// AdminForceStopCampaignRequest is the request body for force-stopping a campaign.
type AdminForceStopCampaignRequest struct {
	Reason string `json:"reason" binding:"required"`
}

// AdminForceStopCampaign handles POST /api/v1/admin/promotions/campaigns/:id/stop
func (h *PromotionHandler) AdminForceStopCampaign(c *gin.Context) {
	ctx := c.Request.Context()

	adminID, err := h.getUserID(c)
	if err != nil {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid instance ID")
		return
	}

	var req AdminForceStopCampaignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.promotionService.ForceStopInstanceAdmin(ctx, tx, instanceID)
	})

	if err != nil {
		switch err.(type) {
		case *promotionApp.InstanceNotFoundError:
			response.NotFound(c, "Campaign not found")
		case *promotionApp.InstanceAlreadyStoppedError:
			response.BadRequest(c, "Campaign is already stopped")
		case *promotionApp.InstanceAlreadyFinalizedError:
			response.BadRequest(c, "Campaign is already finalized")
		default:
			h.log.Error("Admin: failed to force-stop campaign",
				zap.String("instance_id", instanceID.String()),
				zap.Error(err))
			response.InternalServerError(c, "Failed to stop campaign")
		}
		return
	}

	if h.adminAuditLogger != nil {
		h.adminAuditLogger.LogSafe(ctx, adminID, "promotion_campaign.force_stopped", "promotion_instance", instanceID, map[string]interface{}{
			"reason": req.Reason,
		})
	}
	response.Success(c, gin.H{"message": "Campaign stopped successfully"})
}

// packageToResponse converts a PromotionPackage entity to a response DTO.
func packageToResponse(pkg *entity.PromotionPackage) PackageResponse {
	targetTypes := make([]string, len(pkg.AllowedTargetTypes))
	for j, tt := range pkg.AllowedTargetTypes {
		targetTypes[j] = string(tt)
	}
	return PackageResponse{
		ID:                  pkg.ID,
		Name:                pkg.Name,
		TotalDurationHours:  pkg.TotalDurationHours,
		ValidityWindowHours: pkg.ValidityWindowHours,
		PriceAmount:         pkg.PriceAmount,
		AllowedTargetTypes:  targetTypes,
		IsActive:            pkg.IsActive,
		CreatedAt:           pkg.CreatedAt.Format(time.RFC3339),
	}
}
