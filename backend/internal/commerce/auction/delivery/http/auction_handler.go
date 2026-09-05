package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	auctionApp "github.com/labuda/backend/internal/commerce/auction/application"
	"github.com/labuda/backend/internal/commerce/auction/entity"
	forSaleEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	productEntity "github.com/labuda/backend/internal/commerce/product/entity"
	productRepo "github.com/labuda/backend/internal/commerce/product/repository"
	orderApp "github.com/labuda/backend/internal/commerce/order/application"
	orderEntity "github.com/labuda/backend/internal/commerce/order/entity"
	shippingApp "github.com/labuda/backend/internal/commerce/shipping/application"
	"github.com/labuda/backend/internal/governance/viewercontext"
	addressEntity "github.com/labuda/backend/internal/identity/address/entity"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/pkg/blockcheck"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/internal/pkg/sellerdisplay"
	"github.com/labuda/backend/internal/platform/response"
	pricingtokenapp "github.com/labuda/backend/internal/pricing/token/application"
	pricingtokenentity "github.com/labuda/backend/internal/pricing/token/entity"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// AuctionHandler handles HTTP requests for auction operations.
type AuctionHandler struct {
	auctionService      *auctionApp.AuctionService
	productRepo         productRepo.ProductRepository
	pricingTokenService *pricingtokenapp.PricingTokenService
	db                  *db.DB
	log                 *zap.Logger
}

// NewAuctionHandler creates a new AuctionHandler.
func NewAuctionHandler(
	auctionService *auctionApp.AuctionService,
	productRepo productRepo.ProductRepository,
	pricingTokenService *pricingtokenapp.PricingTokenService,
	database *db.DB,
	log *zap.Logger,
) *AuctionHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &AuctionHandler{
		auctionService:      auctionService,
		productRepo:         productRepo,
		pricingTokenService: pricingTokenService,
		db:                  database,
		log:                 log,
	}
}

// CreateAuctionRequest holds the request body for creating an auction.
// A product is created inline — no pre-existing product_id is required.
type CreateAuctionRequest struct {
	// ProductID (optional) — Product identity reuse. When set, the auction
	// attaches to an existing Product owned by the seller. When omitted, a
	// Product is minted inline from the item fields below.
	ProductID *string `json:"product_id"`
	// Product fields (inline — created atomically with the auction unless reused)
	Title             string   `json:"title" binding:"required,min=1,max=200"`
	Description       string   `json:"description" binding:"required,max=5000"`
	MediaURLs         []string `json:"media_urls"`
	Variety           string   `json:"variety"`
	SizeCM            *int     `json:"size_cm"`
	AgeMonths         *int     `json:"age_months"`
	Gender            *string  `json:"gender"`
	Breeder           *string  `json:"breeder"`
	Bloodline         *string  `json:"bloodline"`
	Certificates      []string `json:"certificates"`
	FarmAddressID     *string  `json:"farm_address_id"`
	ShippingSetupIDs []string `json:"shipping_option_ids" binding:"required,min=1"`
	// Auction-specific fields
	StartPrice   int64  `json:"start_price" binding:"required,min=0"`
	BidIncrement int64  `json:"bid_increment" binding:"required,min=1"`
	BuyNowPrice  *int64 `json:"buy_now_price" binding:"omitempty,min=0"`
	// Timing (PASS_18C): seller picks how the auction starts and how long it
	// runs. Backend (entity.ResolveAuctionTiming) is the source of truth for
	// the 1-7 day duration bound — this binding is a client-experience
	// nicety, not the enforcement point.
	StartMode        string  `json:"start_mode" binding:"required,oneof=now scheduled"`
	ScheduledStartAt *string `json:"scheduled_start_at" binding:"omitempty"` // RFC3339; required when start_mode=scheduled
	DurationHours    int     `json:"duration_hours" binding:"required,min=1"`
	// Shipping readiness
	PreparationTime *string `json:"preparation_time" binding:"omitempty,oneof=immediate short medium long"`
	PreparationNote *string `json:"preparation_note"`
}

// CreateAuction handles POST /api/v1/auctions
//
// Creates a new draft auction.
//
// The request must NOT contain a legacy for_sale_id/forSaleId (auction is
// never sourced from a ForSale). An optional product_id (Product identity
// reuse) IS supported: when set, the auction attaches to an existing Product
// owned by the seller; when absent, a Product is minted inline.
//
// Request body:
// - product_id: Optional existing Product id to reuse (Product identity)
// - title: Auction title
// - description: Auction description
// - start_price: Starting bid amount (in minor unit)
// - bid_increment: Minimum increment between bids (in minor unit)
// - buy_now_price: Optional price for immediate purchase
// - start_mode: "now" (start immediately) or "scheduled" (custom future start)
// - scheduled_start_at: Required when start_mode=scheduled (RFC3339)
// - duration_hours: How long the auction runs; backend enforces 24-168h (1-7 days)
//
// Response: Created auction. status=active for start_mode=now,
// status=scheduled for start_mode=scheduled — never draft.
func (h *AuctionHandler) CreateAuction(c *gin.Context) {
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

	// Reject any request body carrying a legacy for_sale_id/forSaleId field.
	// Auction must never be sourced from a ForSale — this is a hard guard
	// against the rejected "auction created from forSale" design, not a
	// silently-ignored field. (An explicit product_id for Product identity
	// reuse IS supported and parsed below.)
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	if bytes.Contains(rawBody, []byte(`"for_sale_id"`)) || bytes.Contains(rawBody, []byte(`"forSaleId"`)) ||
		bytes.Contains(rawBody, []byte(`"listing_id"`)) || bytes.Contains(rawBody, []byte(`"listingId"`)) {
		response.BadRequest(c, "auction cannot be created from a forSale/listing; use product_id for Product reuse or inline product fields")
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(rawBody))

	// Parse request body
	var req CreateAuctionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var startMode entity.StartMode
	switch req.StartMode {
	case "now":
		startMode = entity.StartModeNow
	case "scheduled":
		startMode = entity.StartModeScheduled
	}

	var scheduledStartAt *time.Time
	if req.ScheduledStartAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ScheduledStartAt)
		if err != nil {
			response.BadRequest(c, "Invalid scheduled_start_at format, use RFC3339")
			return
		}
		scheduledStartAt = &t
	}
	duration := time.Duration(req.DurationHours) * time.Hour

	// Parse optional farm_address_id
	var farmAddressID *uuid.UUID
	shippingSetupIDs := make([]uuid.UUID, 0, len(req.ShippingSetupIDs))
	if req.FarmAddressID != nil {
		fid, err := uuid.Parse(*req.FarmAddressID)
		if err != nil {
			response.BadRequest(c, "Invalid farm_address_id format")
			return
		}
		farmAddressID = &fid
	}
	for _, rawID := range req.ShippingSetupIDs {
		optionID, err := uuid.Parse(rawID)
		if err != nil {
			response.BadRequest(c, "Invalid shipping_option_ids value")
			return
		}
		shippingSetupIDs = append(shippingSetupIDs, optionID)
	}

	// Parse optional product_id for Product identity reuse.
	var productID *uuid.UUID
	if req.ProductID != nil && *req.ProductID != "" {
		id, err := uuid.Parse(*req.ProductID)
		if err != nil {
			response.BadRequest(c, "Invalid product_id format")
			return
		}
		productID = &id
	}

	// Execute within transaction
	var auction *entity.Auction
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error

		// Parse preparation time, default to immediate if not provided
		preparationTime := forSaleEntity.PreparationTimeImmediate
		if req.PreparationTime != nil {
			preparationTime = forSaleEntity.PreparationTime(*req.PreparationTime)
			if !preparationTime.IsValid() {
				preparationTime = forSaleEntity.PreparationTimeImmediate
			}
		}

		auction, err = h.auctionService.CreateDraft(ctx, tx, auctionApp.CreateDraftInput{
			SellerID: sellerID,
			// Product identity reuse (optional)
			ProductID: productID,
			// Product fields
			Title:             req.Title,
			Description:       req.Description,
			MediaURLs:         req.MediaURLs,
			Variety:           req.Variety,
			SizeCM:            req.SizeCM,
			AgeMonths:         req.AgeMonths,
			Gender:            req.Gender,
			Breeder:           req.Breeder,
			Bloodline:         req.Bloodline,
			Certificates:      req.Certificates,
			FarmAddressID:     farmAddressID,
			ShippingSetupIDs: shippingSetupIDs,
			// Auction-specific fields
			StartPrice:   req.StartPrice,
			BidIncrement: req.BidIncrement,
			BuyNowPrice:  req.BuyNowPrice,
			// Timing
			StartMode:        startMode,
			ScheduledStartAt: scheduledStartAt,
			Duration:         duration,
			// Shipping readiness
			PreparationTime: preparationTime,
			PreparationNote: req.PreparationNote,
		})
		return err
	})

	if err != nil {
		h.log.Error("Failed to create auction",
			zap.String("seller_id", sellerID.String()),
			zap.Error(err),
		)
		if errors.Is(err, shippingApp.ErrInvalidSellableCreateShippingSelection) {
			response.BadRequest(c, err.Error())
			return
		}
		if isAuctionTimingValidationError(err) {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalServerError(c, "Failed to create auction")
		return
	}

	// Fetch the associated Product for the response (Product owns title/description/media)
	var product *productEntity.Product
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		product, err = h.productRepo.GetByID(ctx, tx, auction.ProductID)
		return err
	})
	if err != nil {
		h.log.Error("Failed to fetch product for response",
			zap.String("product_id", auction.ProductID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to fetch product")
		return
	}

	response.Created(c, auctionToResponse(auction, product))
}

// isAuctionTimingValidationError reports whether err is one of the
// PASS_18C auction timing/start-mode validation errors, which the client
// can fix by changing the request (400), rather than a server fault (500).
func isAuctionTimingValidationError(err error) bool {
	if errors.Is(err, entity.ErrEndAtNotAfterStartAt) ||
		errors.Is(err, entity.ErrScheduledStartRequired) ||
		errors.Is(err, entity.ErrInvalidStartMode) {
		return true
	}
	var durationErr *entity.ErrAuctionDurationOutOfRange
	if errors.As(err, &durationErr) {
		return true
	}
	var futureErr *entity.ErrScheduledStartMustBeFuture
	if errors.As(err, &futureErr) {
		return true
	}
	var horizonErr *entity.ErrScheduledStartBeyondHorizon
	return errors.As(err, &horizonErr)
}

// UpdateAuctionRequest holds the request body for updating an auction.
//
// Canonical update contract (F2.2B):
//   Draft:     title, description, start_price, bid_increment, buy_now_price, start_at, end_at
//   Scheduled: title, description, start_at, end_at
// Product content (title/description) is persisted via ProductRepository;
// Auction surface (pricing/timing) via AuctionRepository — ONE transaction.
// Unsupported fields (images/category/condition/auto_extend*) are NOT bound
// and are explicitly rejected when present (see UpdateAuction guard).
type UpdateAuctionRequest struct {
	Title       *string `json:"title" binding:"omitempty,min=1,max=200"`
	Description *string `json:"description" binding:"omitempty,max=5000"`
	StartPrice   *int64  `json:"start_price" binding:"omitempty,min=0"`
	BidIncrement *int64  `json:"bid_increment" binding:"omitempty,min=1"`
	BuyNowPrice  *int64  `json:"buy_now_price" binding:"omitempty,min=0"`
	StartAt      *string `json:"start_at" binding:"omitempty"` // RFC3339
	EndAt        *string `json:"end_at" binding:"omitempty"`   // RFC3339
}

// UpdateAuction handles PATCH /api/v1/auctions/:id
//
// Updates an auction. Allowed fields depend on auction status:
// - Draft: All fields except product_id
// - Scheduled: title, description, start_at, end_at only
// - Active: No updates allowed
// - Ended/Cancelled: Terminal, no updates
func (h *AuctionHandler) UpdateAuction(c *gin.Context) {
	ctx := c.Request.Context()

	// Get auction ID
	auctionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid auction ID")
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

	// Parse request body — raw read first to enforce unsupported-field guard
	// before ShouldBindJSON silently ignores them.
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	// Explicitly reject unsupported update fields that were carried by the
	// stale mobile DTO (images/category/condition/auto_extend*).
	// Canonical contract is title/description/start_price/bid_increment/buy_now_price/start_at/end_at only.
	if bytes.Contains(rawBody, []byte(`"images"`)) ||
		bytes.Contains(rawBody, []byte(`"category"`)) ||
		bytes.Contains(rawBody, []byte(`"condition"`)) ||
		bytes.Contains(rawBody, []byte(`"auto_extend"`)) ||
		bytes.Contains(rawBody, []byte(`"autoExtend"`)) {
		response.BadRequest(c, "unsupported field: images/category/condition/auto_extend are not supported for auction update")
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(rawBody))

	var req UpdateAuctionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// First get the auction to check status
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		auction, err := h.auctionService.GetAuction(ctx, tx, auctionID)
		if err != nil {
			return err
		}

		// Execute update based on status
		if auction.Status == entity.StatusDraft {
			startPrice := auction.StartPrice
			bidIncrement := auction.BidIncrement
			buyNowPrice := auction.BuyNowPrice
			startAt := auction.StartAt
			endAt := auction.EndAt

			if req.StartPrice != nil {
				startPrice = *req.StartPrice
			}
			if req.BidIncrement != nil {
				bidIncrement = *req.BidIncrement
			}
			if req.BuyNowPrice != nil {
				buyNowPrice = req.BuyNowPrice
			}
			if req.StartAt != nil {
				startAt, err = time.Parse(time.RFC3339, *req.StartAt)
				if err != nil {
					return err
				}
			}
			if req.EndAt != nil {
				endAt, err = time.Parse(time.RFC3339, *req.EndAt)
				if err != nil {
					return err
				}
			}

			return h.auctionService.UpdateDraft(ctx, tx, auctionApp.UpdateDraftInput{
				AuctionID:   auctionID,
				CallerID:    callerID,
				Title:       req.Title,
				Description: req.Description,
				StartPrice:   startPrice,
				BidIncrement: bidIncrement,
				BuyNowPrice:  buyNowPrice,
				StartAt:      startAt,
				EndAt:        endAt,
			})

		} else if auction.Status == entity.StatusScheduled {
			startAt := auction.StartAt
			endAt := auction.EndAt

			if req.StartAt != nil {
				startAt, err = time.Parse(time.RFC3339, *req.StartAt)
				if err != nil {
					return err
				}
			}
			if req.EndAt != nil {
				endAt, err = time.Parse(time.RFC3339, *req.EndAt)
				if err != nil {
					return err
				}
			}

			return h.auctionService.UpdateScheduled(ctx, tx, auctionApp.UpdateScheduledInput{
				AuctionID:   auctionID,
				CallerID:    callerID,
				Title:       req.Title,
				Description: req.Description,
				StartAt:   startAt,
				EndAt:     endAt,
			})

		} else {
			// Active, Ended, Cancelled, WaitingSettlement: cannot update.
			// Covers waiting_settlement as well — settlement lifecycle is immutable via edit.
			return &entity.InvalidOperationError{Status: auction.Status, Reason: "can only update draft or scheduled auctions"}
		}
	})

	if err != nil {
		h.log.Error("Failed to update auction",
			zap.String("auction_id", auctionID.String()),
			zap.Error(err),
		)
		if isAuctionTimingValidationError(err) {
			response.BadRequest(c, err.Error())
			return
		}
		var opErr *entity.InvalidOperationError
		var transErr *entity.InvalidTransitionError
		if errors.As(err, &opErr) || errors.As(err, &transErr) {
			response.Conflict(c, err.Error())
			return
		}
		response.InternalServerError(c, "Failed to update auction")
		return
	}

	// Fetch updated auction — Product is joined into the auction entity
	var updatedAuction *entity.Auction
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		updatedAuction, err = h.auctionService.GetAuction(ctx, tx, auctionID)
		return err
	})

	if err != nil {
		response.InternalServerError(c, "Auction updated but failed to retrieve")
		return
	}

	response.Success(c, auctionToResponse(updatedAuction, updatedAuction.Product))
}

// ScheduleAuction handles POST /api/v1/auctions/:id/schedule
//
// Transitions an auction from draft to scheduled.
func (h *AuctionHandler) ScheduleAuction(c *gin.Context) {
	ctx := c.Request.Context()

	auctionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid auction ID")
		return
	}

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
		return h.auctionService.Schedule(ctx, tx, auctionApp.ScheduleInput{
			AuctionID: auctionID,
			CallerID:  callerID,
		})
	})

	if err != nil {
		h.log.Error("Failed to schedule auction",
			zap.String("auction_id", auctionID.String()),
			zap.Error(err),
		)
		if err == auth.ErrMarketAuthorityRequired {
			response.Forbidden(c, "Active seller subscription required to schedule auctions")
			return
		}
		if err == auth.ErrSellerRequired {
			response.Forbidden(c, "Only the auction owner can schedule this auction")
			return
		}
		response.InternalServerError(c, "Failed to schedule auction")
		return
	}

	response.SuccessWithMessage(c, "Auction scheduled", nil)
}

// CancelAuction handles POST /api/v1/auctions/:id/cancel
//
// Cancels an auction.
// - Draft/Scheduled: Always allowed
// - Active: Only if no bids
func (h *AuctionHandler) CancelAuction(c *gin.Context) {
	ctx := c.Request.Context()

	auctionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid auction ID")
		return
	}

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
		return h.auctionService.Cancel(ctx, tx, auctionApp.CancelInput{
			AuctionID: auctionID,
			CallerID:  callerID,
		})
	})

	if err != nil {
		h.log.Error("Failed to cancel auction",
			zap.String("auction_id", auctionID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to cancel auction")
		return
	}

	response.SuccessWithMessage(c, "Auction cancelled", nil)
}

// PlaceBidRequest holds the request body for placing a bid.
type PlaceBidRequest struct {
	Amount         int64  `json:"amount" binding:"required,min=1"`
	IdempotencyKey string `json:"idempotency_key" binding:"required,min=1,max=100"`
}

// PlaceBid handles POST /api/v1/auctions/:id/bid
//
// Places a bid on an active auction.
//
// Request body:
// - amount: Bid amount (in minor unit)
// - idempotency_key: Unique key for idempotency
func (h *AuctionHandler) PlaceBid(c *gin.Context) {
	ctx := c.Request.Context()

	auctionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid auction ID")
		return
	}

	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	bidderID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	var req PlaceBidRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var bid *entity.AuctionBid
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		bid, err = h.auctionService.PlaceBid(ctx, tx, auctionApp.PlaceBidInput{
			AuctionID:      auctionID,
			BidderID:       bidderID,
			Amount:         req.Amount,
			IdempotencyKey: req.IdempotencyKey,
		})
		return err
	})

	if err != nil {
		if err == auth.ErrMarketAuthorityRequired {
			response.Forbidden(c, "Seller does not have active market authority")
			return
		}

		h.log.Error("Failed to place bid",
			zap.String("auction_id", auctionID.String()),
			zap.String("bidder_id", bidderID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to place bid")
		return
	}

	response.Created(c, bidToResponse(bid))
}

// BuyNowRequest holds the request body for buy now.
// ClaimAuctionRequest holds the request body for the canonical claim endpoint.
type ClaimAuctionRequest struct {
	AddressID        uuid.UUID `json:"address_id" binding:"required"`
	ShippingSetupID uuid.UUID `json:"shipping_option_id" binding:"required"`
	DiscountCode     *string   `json:"discount_code"`
	UseCoins         *bool     `json:"use_coins,omitempty"` // Optional: buyer coin-use intent; backend decides actual amount
}

// ClaimAuction handles POST /api/v1/auctions/:id/claim
//
// Canonical winner shipping-resolution + order-creation action. In a single
// atomic transaction:
//  1. Validate winner, shipping deadline (end_at + 24h), not-settled,
//     not-already-resolved. Locks auction FOR UPDATE.
//  2. Resolve shipping: set auction.shipping_resolved_at = now (first
//     resolution wins).
//  3. Generate + validate the pricing token, create the order, bind
//     auction.OrderID = order.ID.
//  4. The auction STAYS in waiting_settlement — it only transitions to ended
//     when payment succeeds. On payment expiry the auction returns to DRAFT
//     (settlement failure) rather than remaining terminal-ended.
//
// Request body:
//   - address_id:        Buyer's shipping address (UUID, required)
//   - shipping_option_id: Selected shipping option (UUID, required)
//
// Response: { "order_id": "<uuid>" }
func (h *AuctionHandler) ClaimAuction(c *gin.Context) {
	ctx := c.Request.Context()

	auctionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid auction ID")
		return
	}

	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	winnerID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	var req ClaimAuctionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var orderID uuid.UUID
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		// Step 1: Validate winner, deadline, not-settled, not-resolved.
		// Locks auction FOR UPDATE.
		auction, err := h.auctionService.GeneratePricingTokenForAuctionClaim(ctx, tx, auctionApp.GeneratePricingTokenForAuctionInput{
			AuctionID:        auctionID,
			WinnerID:         winnerID,
			AddressID:        req.AddressID,
			ShippingSetupID: req.ShippingSetupID,
		})
		if err != nil {
			return fmt.Errorf("claim validation failed: %w", err)
		}

		// Step 1b: Mark shipping resolved (first-resolution-wins). Anchors the
		// payment deadline: shipping_resolved_at + 24h.
		if err := auction.ResolveShipping(time.Now()); err != nil {
			return fmt.Errorf("shipping resolution failed: %w", err)
		}

		// Step 2: Generate pricing token within the same transaction.
		useCoins := req.UseCoins != nil && *req.UseCoins
		tokenResp, err := h.pricingTokenService.GenerateForAuction(ctx, tx, &pricingtokenapp.GenerateForAuctionRequest{
			UserID:           winnerID,
			AuctionID:        auctionID,
			AddressID:        req.AddressID,
			ShippingSetupID: req.ShippingSetupID,
			DiscountCode:     req.DiscountCode,
			UseCoins:         useCoins,
		})
		if err != nil {
			return fmt.Errorf("pricing token generation failed: %w", err)
		}

		// Step 3: Validate and lock the token for consumption.
		validatedToken, err := h.pricingTokenService.ValidateForOrderLocked(
			ctx, tx,
			tokenResp.Token,
			winnerID,
			auction.ProductID,
			"auction",
			auction.ID,
			0, // Quantity from token
			req.AddressID,
			req.ShippingSetupID,
		)
		if err != nil {
			return fmt.Errorf("pricing token validation failed: %w", err)
		}

		// Step 4: Build pricing snapshot from validated token.
		pricingSnapshot := buildClaimPricingSnapshot(validatedToken)

		// Determine settlement type from token response.
		settlementType := orderEntity.AuctionSettlementBidWin
		if tokenResp.AuctionSettlementType == "buy_now" {
			settlementType = orderEntity.AuctionSettlementBuyNow
		}

		// Step 5: Create order from auction.
		order, err := h.auctionService.CreateOrderFromAuction(ctx, tx, auctionApp.CreateOrderFromAuctionInput{
			Auction:               auction,
			BuyerID:               winnerID,
			WinningBid:            *auction.CurrentBid,
			AddressID:             req.AddressID,
			ShippingSetupID:      req.ShippingSetupID,
			AuctionSettlementType: settlementType,
			PricingSnapshot:       pricingSnapshot,
			UseCoins:              useCoins,
		})
		if err != nil {
			return fmt.Errorf("order creation failed: %w", err)
		}

		// Step 6: Mark pricing token as consumed.
		if err := h.pricingTokenService.FinalizeOrderConsumption(ctx, tx, validatedToken, order.ID); err != nil {
			return fmt.Errorf("pricing token consume failed: %w", err)
		}

		// Step 7: Bind the order and persist shipping resolution + OrderID.
		// The auction STAYS in waiting_settlement until payment succeeds
		// (payment success settles it to ended; payment expiry returns it to
		// draft with the order binding released).
		auction.OrderID = &order.ID
		if err := h.auctionService.PersistAuctionUpdate(ctx, tx, auction); err != nil {
			return fmt.Errorf("auction persist failed: %w", err)
		}

		orderID = order.ID
		return nil
	})

	if err != nil {
		h.log.Error("Failed to claim auction",
			zap.String("auction_id", auctionID.String()),
			zap.String("winner_id", winnerID.String()),
			zap.Error(err),
		)

		switch {
		case errors.Is(err, entity.ErrAlreadySettled):
			response.Conflict(c, "Auction has already been claimed")
		case errors.Is(err, entity.ErrNotClaimable):
			response.Conflict(c, "Auction is not claimable")
		case errors.Is(err, entity.ErrSettlementDeadlinePassed):
			response.Gone(c, "Auction settlement deadline has passed")
		case errors.Is(err, entity.ErrNoWinner):
			response.Conflict(c, "Auction has no winner")
		case errors.Is(err, entity.ErrNotWinner):
			response.Forbidden(c, "Caller is not the auction winner")
		case errors.Is(err, entity.ErrShippingAlreadyResolved):
			response.Conflict(c, "Auction shipping has already been resolved")
		default:
			response.InternalServerError(c, "Failed to claim auction")
		}
		return
	}

	response.Created(c, gin.H{
		"order_id": orderID.String(),
	})
}

// buildClaimPricingSnapshot converts a validated PricingToken to a PricingSnapshot
// for auction claim order creation. Mirrors the order handler's
// buildPricingSnapshotFromToken but lives in the auction handler package to
// avoid cross-package coupling.
func buildClaimPricingSnapshot(token *pricingtokenentity.PricingToken) *orderApp.PricingSnapshot {
	var shippingSource *string
	if token.ShippingQuoteID != nil {
		source := "shipping_quote"
		shippingSource = &source
	} else {
		source := "for_sale"
		shippingSource = &source
	}

	var addressSnapshot *addressEntity.AddressSnapshot
	if len(token.AddressSnapshot) > 0 {
		var snapshot addressEntity.AddressSnapshot
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
		ShippingSetupName:    token.ShippingSetupName,
		ShippingTransportType: token.ShippingTransportType,
		ShippingDestination:   addressSnapshot,
		ShippingSource:         shippingSource,
		ShippingQuoteID:        token.ShippingQuoteID,
		AuctionID:              token.AuctionID,
		TokenID:                token.Token,
		PaymentMethod:          "default",
	}
}

// ListAuctionsRequest holds query parameters for forSale auctions.
type ListAuctionsRequest struct {
	Status   string `form:"status" binding:"omitempty,oneof=draft scheduled active ended cancelled"`
	SellerID string `form:"seller_id" binding:"omitempty"`
	Limit    int    `form:"limit" binding:"omitempty,min=1,max=50"`
	Cursor   string `form:"cursor" binding:"omitempty"` // RFC3339 timestamp
}

// ListAuctions handles GET /api/v1/auctions
//
// Lists auctions with filtering and cursor-based pagination.
// Query parameters:
// - status: Filter by status (draft, scheduled, active, ended, cancelled)
// - seller_id: Filter by seller ID
// - limit: Results per page (default 20, max 50)
// - cursor: RFC3339 timestamp for pagination
func (h *AuctionHandler) ListAuctions(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse query parameters
	var req ListAuctionsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Build filter
	filter := auctionApp.ListAuctionsFilter{
		Limit: req.Limit,
	}

	// Extract viewer ID (optional)
	var viewerID uuid.UUID
	if userIDVal, exists := c.Get("userID"); exists {
		if id, ok := userIDVal.(uuid.UUID); ok {
			viewerID = id
		}
	}

	// Parse seller_id if provided (needed by the status gate below to decide
	// whether a non-public status query is the owning seller's own request).
	if req.SellerID != "" {
		sellerID, err := uuid.Parse(req.SellerID)
		if err != nil {
			response.BadRequest(c, "Invalid seller_id format")
			return
		}
		filter.SellerID = &sellerID
	}

	// Parse status if provided.
	// Non-public statuses (draft, cancelled, ended, ...) are owner-history
	// surfaces: an anonymous or non-owner caller must never query them.
	if req.Status != "" {
		status := entity.Status(req.Status)
		if !status.IsPublicDiscoverable() {
			isOwner := filter.SellerID != nil && viewerID != uuid.Nil && *filter.SellerID == viewerID
			if !isOwner {
				response.Success(c, gin.H{"data": []map[string]interface{}{}, "next_cursor": "", "has_more": false})
				return
			}
		}
		filter.Status = &status
	}

	// Parse cursor if provided
	if req.Cursor != "" {
		cursorTime, err := time.Parse(time.RFC3339Nano, req.Cursor)
		if err != nil {
			response.BadRequest(c, "Invalid cursor format, use RFC3339")
			return
		}
		filter.Cursor = &cursorTime
	}

	// Block enforcement: if filtering by seller_id and viewer has blocked that seller,
	// return empty results.
	if filter.SellerID != nil && viewerID != uuid.Nil && *filter.SellerID != viewerID {
		var blocked bool
		_ = h.db.WithTx(ctx, func(tx db.Tx) error {
			var err error
			blocked, err = blockcheck.IsBidirectionallyBlocked(ctx, tx, viewerID, *filter.SellerID)
			return err
		})
		if blocked {
			response.Success(c, gin.H{"data": []map[string]interface{}{}, "next_cursor": "", "has_more": false})
			return
		}
	}

	// Execute query
	var result auctionApp.ListAuctionsResult
	var sellerInfoByID map[uuid.UUID]sellerdisplay.Info
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		result, err = h.auctionService.ListAuctions(ctx, tx, filter)
		if err != nil {
			return err
		}

		// Block enforcement: post-fetch filter for general auction browse.
		if viewerID != uuid.Nil && filter.SellerID == nil {
			sellerIDs := make([]uuid.UUID, 0, len(result.Auctions))
			for _, a := range result.Auctions {
				sellerIDs = append(sellerIDs, a.SellerID)
			}
			blockedSet, _ := blockcheck.BlockedSet(ctx, tx, viewerID, sellerIDs)
			if len(blockedSet) > 0 {
				filtered := make([]*entity.Auction, 0, len(result.Auctions))
				for _, a := range result.Auctions {
					if !blockedSet[a.SellerID] {
						filtered = append(filtered, a)
					}
				}
				result.Auctions = filtered
			}
		}

		// Phase 5 Stage 1 — batch-hydrate additive seller convergence
		// fields for all auctions on the page.
		sellerIDs := make([]uuid.UUID, 0, len(result.Auctions))
		for _, a := range result.Auctions {
			sellerIDs = append(sellerIDs, a.SellerID)
		}
		sellerInfoByID, _ = sellerdisplay.FetchMany(ctx, tx, sellerIDs)
		return nil
	})

	if err != nil {
		h.log.Error("Failed to list auctions",
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve auctions")
		return
	}

	// Convert to response — Product is joined and attached to each auction
	data := make([]map[string]interface{}, len(result.Auctions))
	for i, auction := range result.Auctions {
		data[i] = auctionToResponseWithSeller(auction, auction.Product, sellerInfoByID[auction.SellerID])
	}

	response.Success(c, gin.H{
		"data":        data,
		"next_cursor": result.NextCursor,
		"has_more":    result.HasMore,
	})
}

// GetAuction handles GET /api/v1/auctions/:id
//
// Retrieves an auction by ID.
func (h *AuctionHandler) GetAuction(c *gin.Context) {
	ctx := c.Request.Context()

	auctionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid auction ID")
		return
	}

	// Extract viewer ID (optional)
	var viewerID uuid.UUID
	if userIDVal, exists := c.Get("userID"); exists {
		if id, ok := userIDVal.(uuid.UUID); ok {
			viewerID = id
		}
	}

	var auction *entity.Auction
	var sellerInfo sellerdisplay.Info
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		auction, err = h.auctionService.GetAuction(ctx, tx, auctionID)
		if err != nil {
			return err
		}

		// Block enforcement: hide auction from blocked seller's viewer
		if viewerID != uuid.Nil && auction.SellerID != viewerID {
			blocked, blockErr := blockcheck.IsBidirectionallyBlocked(ctx, tx, viewerID, auction.SellerID)
			if blockErr != nil {
				h.log.Warn("block check failed, fail-open", zap.Error(blockErr))
			}
			if blocked {
				return fmt.Errorf("blocked")
			}
		}

		// Phase 5 Stage 1 — hydrate additive seller convergence fields
		// inside the same transaction.
		sellerInfo, _ = sellerdisplay.FetchOne(ctx, tx, auction.SellerID)
		return nil
	})

	if err != nil {
		if err.Error() == "blocked" {
			response.NotFound(c, "Auction not found")
			return
		}
		h.log.Error("Failed to get auction",
			zap.String("auction_id", auctionID.String()),
			zap.Error(err),
		)
		response.NotFound(c, "Auction not found")
		return
	}

	response.Success(c, auctionToResponseWithSeller(auction, auction.Product, sellerInfo))
}

// ListBidsRequest holds query parameters for forSale bids.
type ListBidsRequest struct {
	Limit int `form:"limit" binding:"omitempty,min=1,max=100"`
}

// ListBids handles GET /api/v1/auctions/:id/bids
//
// Lists bids for an auction, ordered by creation time (newest first).
//
// D14 — Auction bid discovery governance convergence.
//
// Pattern A — Public Discovery ViewerContext propagation (mirrors
// /search/content per docs/05-rollout/search-content-viewercontext-
// runtime-threading-task-design.md):
//
//   - ViewerContext is constructed at the HTTP boundary
//     (constructAuctionBidsViewerContext): AuthenticatedViewer when
//     user_id is present (the route is auth-middleware-gated, so this is
//     the production path), AnonymousViewer otherwise (defensive).
//   - Bidder identity is batched through publiccard.UserCard with
//     coarsened lifecycle via hydrateBidderLifecycleCards. The raw
//     account_status enum NEVER reaches the wire.
//   - Bidirectional user_blocks is enforced caller-side via
//     hydrateBidsBlockedSet; rows whose bidder is in the blocked set are
//     dropped before the response is written.
//   - The response wire shape removes the previously-leaked fields
//     (idempotency_key, flat bidder_username) and emits the canonical
//     publiccard.UserCard nested under `bidder`. Mobile already parses
//     `bidder` as UserBriefDto; the new shape is forward-compatible with
//     the additive identity fields landed in this batch.
//
// No evaluator is introduced on this surface — this is bounded visibility
// convergence per D14 spec, not an evaluator rollout.
func (h *AuctionHandler) ListBids(c *gin.Context) {
	ctx := c.Request.Context()

	auctionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid auction ID")
		return
	}

	var req ListBidsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if req.Limit <= 0 {
		req.Limit = 50
	}

	var bids []*entity.AuctionBid
	var bidderCardsByID map[uuid.UUID]publiccard.UserCard
	var blockedBidderSet map[uuid.UUID]struct{}
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		// Canonical Pattern A ViewerContext construction inside the
		// transaction scope so overlay hydration reuses the request tx.
		vc := constructAuctionBidsViewerContext(c, tx)

		var err error
		bids, err = h.auctionService.ListBids(ctx, tx, auctionID, req.Limit)
		if err != nil {
			return err
		}

		// D14 — caller-batched bidder identity + lifecycle hydration.
		bidderCardsByID = hydrateBidderLifecycleCards(ctx, tx, bids)

		// D14 — caller-batched bidirectional block resolution. Anonymous
		// viewers receive an empty set (block semantics are viewer-
		// relative); DB hydration failures fail-OPEN per the D14 spec.
		blockedBidderSet = hydrateBidsBlockedSet(ctx, tx, vc, bids)
		return nil
	})

	if err != nil {
		h.log.Error("Failed to list bids",
			zap.String("auction_id", auctionID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve bids")
		return
	}

	bidResponses := make([]map[string]interface{}, 0, len(bids))
	for _, bid := range bids {
		if _, blocked := blockedBidderSet[bid.BidderID]; blocked {
			continue
		}
		bidResponses = append(bidResponses, bidToResponseWithBidderCard(bid, bidderCardsByID[bid.BidderID]))
	}

	response.Success(c, gin.H{
		"auction_id": auctionID.String(),
		"bids":       bidResponses,
		"count":      len(bidResponses),
	})
}

// auctionToResponse converts an auction entity to API response.
// Used for create/update/cancel responses. Caller is responsible for
// passing the associated Product entity so title/description/media are
// included in the response.
func auctionToResponse(a *entity.Auction, product *productEntity.Product) map[string]interface{} {
	return auctionToResponseWithSeller(a, product, sellerdisplay.Info{})
}

// auctionToResponseWithSeller renders auction JSON with seller display
// fields hydrated from sellerdisplay.Info. Used by detail/list endpoints
// that batch-fetch seller info to avoid N+1.
//
// Product content (title, description, media) is read from the Product
// entity — the auction entity no longer carries duplicate content fields.
func auctionToResponseWithSeller(
	a *entity.Auction,
	product *productEntity.Product,
	seller sellerdisplay.Info,
) map[string]interface{} {
	var auctionSellerAvatar *string
	if seller.AvatarURL != "" {
		av := seller.AvatarURL
		auctionSellerAvatar = &av
	}
	auctionUserLifecycle := string(viewercontext.CoarsenLifecycle(seller.AccountStatus, seller.IsDeleted))
	auctionSellerTrustLifecycle := string(viewercontext.CoarsenSellerTrust(seller.SubscriptionStatus))
	auctionSellerCard := publiccard.NewSellerCardWithBothLifecycles(
		a.SellerID, seller.Username, auctionSellerAvatar,
		seller.FarmName,
		auctionUserLifecycle,
		auctionSellerTrustLifecycle,
		seller.Tier,
	)

	title := ""
	description := ""
	var thumbnail *string
	if product != nil {
		title = product.Title
		description = product.Description
		if len(product.MediaURLs) > 0 {
			t := product.MediaURLs[0]
			thumbnail = &t
		}
	}

	auctionCard := publiccard.NewAuctionCard(
		a.ID,
		title,
		thumbnail,
		a.CurrentBid,
		a.BuyNowPrice,
		a.EndAt.Format(time.RFC3339),
		a.Status.PublicLifecycle(),
		&auctionSellerCard,
	)

	resp := map[string]interface{}{
		"id":            a.ID.String(),
		"seller_id":     a.SellerID.String(),
		"product_id":    a.ProductID.String(),
		"title":         title,
		"description":   description,
		"start_price":   a.StartPrice,
		"bid_increment": a.BidIncrement,
		"buy_now_price": a.BuyNowPrice,
		"start_at":      a.StartAt.Format(time.RFC3339),
		"end_at":        a.EndAt.Format(time.RFC3339),
		"current_bid":   a.CurrentBid,
		"current_winner_id": func() *string {
			if a.CurrentWinnerID != nil {
				s := a.CurrentWinnerID.String()
				return &s
			}
			return nil
		}(),
		"status":     string(a.Status),
		"lifecycle":  a.Status.PublicLifecycle(),
		"created_at": a.CreatedAt.Format(time.RFC3339),
		"updated_at": a.UpdatedAt.Format(time.RFC3339),
		"seller_username":   seller.Username,
		"seller_farm_name":  seller.FarmName,
		"seller_avatar_url": seller.AvatarURL,
		"auction": auctionCard,
	}
	return resp
}

// bidToResponse converts an auction bid entity to the public API
// response shape.
//
// D14 — Auction bid discovery governance convergence.
//
// The PlaceBid write path does NOT pre-fetch the bidder identity for
// the response (the bid was just created by the caller themselves, so
// the caller already has their own identity). The public response shape
// matches ListBids — the `bidder` nested card is emitted without
// lifecycle (caller can render their own identity from local state),
// and the previously-leaked fields (idempotency_key, flat
// bidder_username) are NEVER emitted.
//
// Removed (D14):
//   - "idempotency_key" — client-supplied token; was enumerable via the
//     read path and is not part of the public bid history contract.
//   - "bidder_username" (flat scalar) — superseded by the nested
//     `bidder` UserCard whose `username` field is the canonical
//     identity surface per public-card-boundary.md.
func bidToResponse(b *entity.AuctionBid) map[string]interface{} {
	return bidToResponseWithBidderCard(b, publiccard.UserCard{})
}

// bidToResponseWithBidderCard renders the canonical public bid JSON
// with the bidder identity nested as a publiccard.UserCard (carrying
// coarsened lifecycle when hydrated).
//
// The caller MUST batch-hydrate the bidder card via
// hydrateBidderLifecycleCards before invoking this builder. An empty
// (zero-value) card is acceptable for write paths (PlaceBid) where the
// caller is themselves the bidder — the card's `id` field defaults to
// the entity's BidderID via the fallback below so the wire shape stays
// uniform.
//
// Wire fields:
//   - id, auction_id, bidder_id, amount, created_at (unchanged from
//     pre-D14)
//   - bidder: nested UserCard {id, username, display_name, avatar_url,
//     lifecycle}
//
// Public boundary: raw account_status, deleted_at, email,
// idempotency_key NEVER reach this builder. UserCard fields are the
// only identity surface.
func bidToResponseWithBidderCard(
	b *entity.AuctionBid,
	bidder publiccard.UserCard,
) map[string]interface{} {
	if bidder.ID == uuid.Nil {
		// Write-path fallback: caller did not batch-hydrate. Synthesize
		// an anonymous-safe card so the wire shape is uniform; the
		// caller already knows their own identity and does not need it
		// echoed back here.
		bidder = publiccard.Anonymous(b.BidderID)
	}
	return map[string]interface{}{
		"id":         b.ID.String(),
		"auction_id": b.AuctionID.String(),
		"bidder_id":  b.BidderID.String(),
		"amount":     b.Amount,
		"created_at": b.CreatedAt.Format(time.RFC3339),
		"bidder":     bidder,
	}
}
