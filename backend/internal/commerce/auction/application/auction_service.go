package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/commerce/auction/entity"
	auctionRepo "github.com/labuda/backend/internal/commerce/auction/infrastructure/repository"
	forsaleEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	orderApp "github.com/labuda/backend/internal/commerce/order/application"
	orderEntity "github.com/labuda/backend/internal/commerce/order/entity"
	orderRepo "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	productEntity "github.com/labuda/backend/internal/commerce/product/entity"
	shippingApp "github.com/labuda/backend/internal/commerce/shipping/application"
	shippingRepo "github.com/labuda/backend/internal/commerce/shipping/infrastructure/repository"
	"github.com/labuda/backend/internal/identity/auth"
	addressEntity "github.com/labuda/backend/internal/identity/address/entity"
	addressRepo "github.com/labuda/backend/internal/identity/address/repository"
	platformconfigApp "github.com/labuda/backend/internal/platform/config/application"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// ProductCreator creates a canonical product record within a transaction.
// Defined locally so auction/application does not import product/infrastructure.
// dependencies.go wires the concrete productRepoImpl.ProductRepositoryImpl.
type ProductCreator interface {
	Create(ctx context.Context, tx db.Tx, product *productEntity.Product) error
	ClaimSellingSurface(ctx context.Context, tx db.Tx, productID uuid.UUID, surface productEntity.SellingSurface) error
}

// productReusableGetter resolves an existing Product for reuse. The concrete
// product repo (ProductRepositoryImpl) implements GetByID; the type assertion
// keeps existing ProductCreator fakes (which only implement Create) compiling.
type productReusableGetter interface {
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*productEntity.Product, error)
}

// AuctionService handles auction state transitions and operations.
// It enforces state machine rules and persists changes with proper locking.
// Commission is read from PlatformConfigService at order creation time (snapshot model).
//
// All order creation operations are delegated to OrderService to maintain
// single responsibility principle.
//
// MARKET AUTHORITY ENFORCEMENT (PHASE 1B):
// - CreateDraft: Allowed without active subscription (workspace safety)
// - Schedule: Requires active seller subscription (hasMarketAuthority)
type AuctionService struct {
	auctionRepo          *auctionRepo.AuctionRepository
	bidRepo              *auctionRepo.AuctionBidRepository
	orderRepo            *orderRepo.OrderRepository
	orderService         *orderApp.OrderService
	shippingSvc          *shippingApp.ShippingService
	shippingSetupRepo   shippingRepo.ShippingSetupRepository
	shippingCoverageRepo shippingRepo.ShippingCoverageRepository
	productShippingRepo  shippingRepo.ProductShippingSetupRepository
	addressRepo          addressRepo.AddressRepository
	outboxRepo           *outboxRepo.OutboxRepository
	ownership            *auth.OwnershipValidator
	accountStatus        auth.AccountStatusChecker
	roleChecker          auth.RoleChecker
	configService  *platformconfigApp.ConfigService
	productRepo    ProductCreator
	log            *zap.Logger
}

// NewAuctionService creates a new AuctionService.
// Commission is read from PlatformConfigService at order creation time.
func NewAuctionService(
	accountStatus auth.AccountStatusChecker,
	shippingService *shippingApp.ShippingService,
	shippingSetupRepo shippingRepo.ShippingSetupRepository,
	shippingCoverageRepo shippingRepo.ShippingCoverageRepository,
	productShippingRepo shippingRepo.ProductShippingSetupRepository,
	outboxRepo *outboxRepo.OutboxRepository,
	configService *platformconfigApp.ConfigService,
	orderService *orderApp.OrderService,
	roleChecker auth.RoleChecker,
	addressRepository addressRepo.AddressRepository,
	log *zap.Logger,
) *AuctionService {
	if log == nil {
		log = zap.NewNop()
	}

	return &AuctionService{
		auctionRepo:          auctionRepo.NewAuctionRepository(),
		bidRepo:              auctionRepo.NewAuctionBidRepository(),
		orderRepo:            orderRepo.NewOrderRepository(),
		orderService:         orderService,
		shippingSvc:          shippingService,
		shippingSetupRepo:   shippingSetupRepo,
		shippingCoverageRepo: shippingCoverageRepo,
		productShippingRepo:  productShippingRepo,
		addressRepo:          addressRepository,
		outboxRepo:           outboxRepo,
		ownership:            auth.NewOwnershipValidator(),
		accountStatus:        accountStatus,
		roleChecker:          roleChecker,
		configService:  configService,
		log:            log,
	}
}

// SetProductRepo attaches the product creator for inline product creation
// during CreateDraft. Must be called before any CreateDraft invocation.
func (s *AuctionService) SetProductRepo(repo ProductCreator) {
	s.productRepo = repo
}

// buildAuctionPayload creates a JSON payload for auction events.
func buildAuctionPayload(auction *entity.Auction) []byte {
	type payload struct {
		AuctionID     string  `json:"auction_id"`
		SellerID      string  `json:"seller_id"`
		ProductID     string  `json:"product_id"`
		Status        string  `json:"status"`
		StartPrice    int64   `json:"start_price"`
		CurrentBid    *int64  `json:"current_bid,omitempty"`
		CurrentWinner *string `json:"current_winner,omitempty"`
	}
	p := payload{
		AuctionID:  auction.ID.String(),
		SellerID:   auction.SellerID.String(),
		ProductID:  auction.ProductID.String(),
		Status:     string(auction.Status),
		StartPrice: auction.StartPrice,
		CurrentBid: auction.CurrentBid,
	}
	if auction.CurrentWinnerID != nil {
		winner := auction.CurrentWinnerID.String()
		p.CurrentWinner = &winner
	}
	b, _ := json.Marshal(p)
	return b
}

// buildAuctionExtendedPayload creates a JSON payload for the auction.extended
// soft-close event (PASS_18C).
func buildAuctionExtendedPayload(auction *entity.Auction, extension time.Duration) []byte {
	type payload struct {
		AuctionID             string `json:"auction_id"`
		NewEndAt              string `json:"new_end_at"`
		ExtensionSeconds      int64  `json:"extension_seconds"`
		TotalExtensionSeconds int64  `json:"total_extension_seconds"`
	}
	p := payload{
		AuctionID:             auction.ID.String(),
		NewEndAt:              auction.EndAt.Format(time.RFC3339),
		ExtensionSeconds:      int64(extension / time.Second),
		TotalExtensionSeconds: int64(auction.AntiSnipeExtensionTotal / time.Second),
	}
	b, _ := json.Marshal(p)
	return b
}

// buildAuctionBidPayload creates a JSON payload for auction bid events.
func buildAuctionBidPayload(bid *entity.AuctionBid) []byte {
	type payload struct {
		BidID     string `json:"bid_id"`
		AuctionID string `json:"auction_id"`
		BidderID  string `json:"bidder_id"`
		Amount    int64  `json:"amount"`
	}
	p := payload{
		BidID:     bid.ID.String(),
		AuctionID: bid.AuctionID.String(),
		BidderID:  bid.BidderID.String(),
		Amount:    bid.Amount,
	}
	b, _ := json.Marshal(p)
	return b
}

// CreateDraftInput contains parameters for creating a draft auction.
// By default a Product is created inline from the product fields. When
// ProductID is set (Product identity reuse), the auction attaches to that
// existing Product instead of minting a new one; the product must exist and
// belong to the seller.
type CreateDraftInput struct {
	SellerID uuid.UUID
	// ProductID (optional) — Product identity reuse target.
	ProductID *uuid.UUID
	// Product fields — created atomically with the auction unless reused
	Title             string
	Description       string
	MediaURLs         []string
	Variety           string
	SizeCM            *int
	AgeMonths         *int
	Gender            *string
	Breeder           *string
	Bloodline         *string
	Certificates      []string
	FarmAddressID     *uuid.UUID
	ShippingSetupIDs []uuid.UUID
	// Auction-specific fields
	StartPrice   int64
	BidIncrement int64
	BuyNowPrice  *int64
	// Timing (PASS_18C): the seller picks a start mode and duration; the
	// service computes and validates start_at/end_at server-side so backend
	// remains the source of truth regardless of client input.
	StartMode        entity.StartMode
	ScheduledStartAt *time.Time // required only when StartMode == StartModeScheduled
	Duration         time.Duration
	// Shipping readiness
	PreparationTime forsaleEntity.PreparationTime
	PreparationNote *string
}

// CreateDraft creates a new draft auction.
//
// LOCK DISCIPLINE:
// - Validate product reference
// - Create auction
// - Emit outbox event
//
// All operations happen within the same transaction for atomicity.
func (s *AuctionService) CreateDraft(
	ctx context.Context,
	tx db.Tx,
	input CreateDraftInput,
) (*entity.Auction, error) {
	// Validate seller account status
	if err := s.accountStatus.EnsureActive(ctx, input.SellerID); err != nil {
		return nil, fmt.Errorf("seller account not active: %w", err)
	}

	// Resolve and validate start_at/end_at from the seller's chosen start
	// mode + duration. Server time is the only trustworthy "now" — this is
	// the sole source of truth for the 1-7 day duration bound (PASS_18C).
	startAt, endAt, err := entity.ResolveAuctionTiming(input.StartMode, input.ScheduledStartAt, input.Duration, time.Now())
	if err != nil {
		return nil, err
	}

	// Validate buy_now_price >= start_price + bid_increment
	if input.BuyNowPrice != nil && *input.BuyNowPrice < input.StartPrice+input.BidIncrement {
		return nil, fmt.Errorf("buy_now_price must be >= start_price + bid_increment")
	}

	shippingSetupIDs, err := shippingApp.ValidateSellableCreateShippingSelection(
		ctx,
		tx,
		s.shippingSetupRepo,
		s.shippingCoverageRepo,
		input.SellerID,
		input.ShippingSetupIDs,
	)
	if err != nil {
		return nil, err
	}

	// Resolve the auctioned Product: reuse an existing Product (stable
	// Identity) when ProductID is supplied, otherwise mint one inline —
	// atomically with the auction in the same transaction (mirrors
	// ForSaleRepositoryImpl.Create()).
	var productID uuid.UUID
	if input.ProductID != nil {
		lookup, ok := s.productRepo.(productReusableGetter)
		if !ok {
			return nil, fmt.Errorf("product repo does not support product reuse")
		}
		existing, err := lookup.GetByID(ctx, tx, *input.ProductID)
		if err != nil {
			return nil, fmt.Errorf("reuse product failed: %w", err)
		}
		if existing.SellerID != input.SellerID {
			return nil, fmt.Errorf("cannot create auction on product owned by another seller")
		}
		// INVARIANT: Product must not already belong to any selling surface.
		// ClaimSellingSurface uses SELECT ... FOR UPDATE to prevent concurrent
		// attachment to both ForSale and Auction.
		if err := s.productRepo.ClaimSellingSurface(ctx, tx, existing.ID, productEntity.SellingSurfaceAuction); err != nil {
			return nil, fmt.Errorf("cannot attach auction to product: %w", err)
		}
		productID = existing.ID
	} else {
		product := &productEntity.Product{
			SellerID:        input.SellerID,
			Title:           input.Title,
			Description:     input.Description,
			MediaURLs:       input.MediaURLs,
			Variety:         input.Variety,
			SizeCm:          input.SizeCM,
			AgeMonths:       input.AgeMonths,
			Gender:          input.Gender,
			Breeder:         input.Breeder,
			Bloodline:       input.Bloodline,
			Certificates:    input.Certificates,
			FarmAddressID:   input.FarmAddressID,
			PreparationTime: string(input.PreparationTime),
			PreparationNote: input.PreparationNote,
			SellingSurface:  productEntity.SellingSurfaceAuction,
		}
		if err := s.productRepo.Create(ctx, tx, product); err != nil {
			return nil, fmt.Errorf("failed to create product: %w", err)
		}
		productID = product.ID
	}

	// Create draft auction — Product content (title, description, koi attributes,
	// preparation, media) is owned by Product entity, not by Auction.
	auction := entity.NewDraft(
		input.SellerID,
		productID,
		input.StartPrice,
		input.BidIncrement,
		input.BuyNowPrice,
		startAt,
		endAt,
	)

	// Persist auction
	if err := s.auctionRepo.CreateTx(ctx, tx, auction); err != nil {
		return nil, fmt.Errorf("failed to create auction: %w", err)
	}

	if err := shippingApp.LinkSellableCreateShippingSelection(
		ctx,
		tx,
		s.productShippingRepo,
		productID,
		shippingSetupIDs,
	); err != nil {
		return nil, err
	}

	// Emit outbox event
	if err := s.outboxRepo.InsertEvent(
		ctx, tx,
		"auction.created",
		auction.ID,
		buildAuctionPayload(auction),
	); err != nil {
		return nil, fmt.Errorf("failed to insert outbox event: %w", err)
	}

	// PASS_18C: create no longer leaves the auction sitting in draft waiting
	// on a separate seller action. Progress it to scheduled — and, for an
	// immediate start, straight through to active — in the same transaction,
	// reusing the exact market-authority + shipping-coverage gate Schedule()
	// enforces so nothing can bypass it via the create path.
	if err := s.scheduleAuctionInternal(ctx, tx, auction, input.SellerID); err != nil {
		return nil, err
	}
	if err := s.auctionRepo.UpdateTx(ctx, tx, auction); err != nil {
		return nil, fmt.Errorf("failed to schedule auction: %w", err)
	}
	if err := s.outboxRepo.InsertEvent(
		ctx, tx,
		"auction.scheduled",
		auction.ID,
		buildAuctionPayload(auction),
	); err != nil {
		return nil, fmt.Errorf("failed to insert outbox event: %w", err)
	}

	if input.StartMode == entity.StartModeNow {
		if err := auction.Activate(); err != nil {
			return nil, err
		}
		if err := s.auctionRepo.UpdateTx(ctx, tx, auction); err != nil {
			return nil, fmt.Errorf("failed to activate auction: %w", err)
		}
		if err := s.outboxRepo.InsertEvent(
			ctx, tx,
			"auction.activated",
			auction.ID,
			buildAuctionPayload(auction),
		); err != nil {
			return nil, fmt.Errorf("failed to insert outbox event: %w", err)
		}
	}

	s.log.Info("Auction created",
		zap.String("auction_id", auction.ID.String()),
		zap.String("seller_id", auction.SellerID.String()),
		zap.String("product_id", auction.ProductID.String()),
		zap.String("status", string(auction.Status)),
	)

	return auction, nil
}

// ScheduleInput contains parameters for scheduling an auction.
type ScheduleInput struct {
	AuctionID uuid.UUID
	CallerID  uuid.UUID
}

// Schedule transitions an auction from draft to scheduled.
//
// MARKET AUTHORITY ENFORCEMENT (PHASE 1B):
// Requires active seller subscription to schedule auctions.
// Expired sellers can create drafts but cannot schedule them.
//
// LOCK DISCIPLINE:
// - Lock Auction (FOR UPDATE)
// - Validate ownership
// - MARKET AUTHORITY CHECK
// - SHIPPING COVERAGE CHECK
// - Validate state transition
// - Update auction
// - Emit outbox event
func (s *AuctionService) Schedule(
	ctx context.Context,
	tx db.Tx,
	input ScheduleInput,
) error {
	// Lock auction
	auction, err := s.auctionRepo.GetForUpdate(ctx, tx, input.AuctionID)
	if err != nil {
		return err
	}

	if err := s.scheduleAuctionInternal(ctx, tx, auction, input.CallerID); err != nil {
		return err
	}

	// Persist
	if err := s.auctionRepo.UpdateTx(ctx, tx, auction); err != nil {
		return err
	}

	// Emit outbox event
	if err := s.outboxRepo.InsertEvent(
		ctx, tx,
		"auction.scheduled",
		auction.ID,
		buildAuctionPayload(auction),
	); err != nil {
		return fmt.Errorf("failed to insert outbox event: %w", err)
	}

	return nil
}

// scheduleAuctionInternal performs the shared ownership + MARKET AUTHORITY +
// SHIPPING COVERAGE checks and transitions auction from draft to scheduled.
// Shared by the explicit Schedule() service method and CreateDraft's
// immediate/scheduled auto-progression (PASS_18C), so both paths enforce the
// exact same gate before an auction ever becomes market-visible.
func (s *AuctionService) scheduleAuctionInternal(
	ctx context.Context,
	tx db.Tx,
	auction *entity.Auction,
	callerID uuid.UUID,
) error {
	// Validate ownership
	if !s.ownership.IsSeller(callerID, auction.SellerID) {
		return auth.ErrSellerRequired
	}

	// MARKET AUTHORITY CHECK: Scheduling requires active seller subscription
	hasCapability, err := s.roleChecker.HasActiveSellerCapability(ctx, callerID)
	if err != nil {
		return fmt.Errorf("failed to verify market authority: %w", err)
	}
	if !hasCapability {
		return auth.ErrMarketAuthorityRequired
	}

	// SHIPPING COVERAGE CHECK: Auction must have at least one shipping option
	// with at least one active coverage before going live. Buyers cannot
	// checkout an auction with no coverable address, so we block here rather
	// than surprising them at claim time.
	if err := s.ensureShippingCoverage(ctx, tx, auction.ProductID); err != nil {
		return err
	}

	// Validate state transition
	return auction.Schedule()
}

// ensureShippingCoverage verifies that the product has at least one shipping
// option with at least one active (is_available=true) coverage row. Used as
// a pre-flight before transitioning an auction to a market-visible state.
//
// Returns shippingApp.ErrShippingNotConfigured when no coverable option exists,
// so upstream handlers can surface the canonical SHIPPING_NOT_CONFIGURED code.
func (s *AuctionService) ensureShippingCoverage(
	ctx context.Context,
	tx db.Tx,
	productID uuid.UUID,
) error {
	options, err := s.productShippingRepo.GetByProduct(ctx, tx, productID)
	if err != nil {
		return fmt.Errorf("failed to load shipping options: %w", err)
	}
	for _, opt := range options {
		coverages, err := s.shippingCoverageRepo.GetByShippingSetup(ctx, tx, opt.ID)
		if err != nil {
			return fmt.Errorf("failed to load coverage for option %s: %w", opt.ID, err)
		}
		for _, c := range coverages {
			if c.IsAvailable {
				return nil // At least one option can serve buyers — schedule is safe.
			}
		}
	}
	return shippingApp.ErrShippingNotConfigured
}

// UpdateDraftInput contains parameters for updating a draft auction.
// Product content (title, description, koi attributes) is updated via
// the Product entity — this struct only carries surface-specific fields.
type UpdateDraftInput struct {
	AuctionID    uuid.UUID
	CallerID     uuid.UUID
	StartPrice   int64
	BidIncrement int64
	BuyNowPrice  *int64
	StartAt      time.Time
	EndAt        time.Time
}

// UpdateDraft updates a draft auction.
func (s *AuctionService) UpdateDraft(
	ctx context.Context,
	tx db.Tx,
	input UpdateDraftInput,
) error {
	// Lock auction
	auction, err := s.auctionRepo.GetForUpdate(ctx, tx, input.AuctionID)
	if err != nil {
		return err
	}

	// Validate ownership
	if !s.ownership.IsSeller(input.CallerID, auction.SellerID) {
		return auth.ErrSellerRequired
	}

	// Update draft
	if err := auction.UpdateDraft(
		input.StartPrice,
		input.BidIncrement,
		input.BuyNowPrice,
		input.StartAt,
		input.EndAt,
	); err != nil {
		return err
	}

	// Persist
	if err := s.auctionRepo.UpdateTx(ctx, tx, auction); err != nil {
		return err
	}

	return nil
}

// UpdateScheduledInput contains parameters for updating a scheduled auction.
// Only timing can be changed at this stage. Product content is updated via
// the Product entity.
type UpdateScheduledInput struct {
	AuctionID uuid.UUID
	CallerID  uuid.UUID
	StartAt   time.Time
	EndAt     time.Time
}

// UpdateScheduled updates a scheduled auction (restricted fields).
func (s *AuctionService) UpdateScheduled(
	ctx context.Context,
	tx db.Tx,
	input UpdateScheduledInput,
) error {
	// Lock auction
	auction, err := s.auctionRepo.GetForUpdate(ctx, tx, input.AuctionID)
	if err != nil {
		return err
	}

	// Validate ownership
	if !s.ownership.IsSeller(input.CallerID, auction.SellerID) {
		return auth.ErrSellerRequired
	}

	// Update scheduled
	if err := auction.UpdateScheduled(
		input.StartAt,
		input.EndAt,
	); err != nil {
		return err
	}

	// Persist
	if err := s.auctionRepo.UpdateTx(ctx, tx, auction); err != nil {
		return err
	}

	return nil
}

// CancelInput contains parameters for cancelling an auction.
type CancelInput struct {
	AuctionID uuid.UUID
	CallerID  uuid.UUID
}

// Cancel cancels an auction.
//
// Can cancel from:
// - Draft: Always allowed
// - Scheduled: Always allowed
// - Active: Only if no bids
//
// LOCK DISCIPLINE:
// - Lock Auction (FOR UPDATE)
// - Validate ownership
// - Validate can cancel
// - Update auction
// - Emit outbox event
func (s *AuctionService) Cancel(
	ctx context.Context,
	tx db.Tx,
	input CancelInput,
) error {
	// Lock auction
	auction, err := s.auctionRepo.GetForUpdate(ctx, tx, input.AuctionID)
	if err != nil {
		return err
	}

	// Validate ownership
	if !s.ownership.IsSeller(input.CallerID, auction.SellerID) {
		return auth.ErrSellerRequired
	}

	// Validate can cancel
	if !auction.CanCancel() {
		return fmt.Errorf("auction cannot be cancelled in status %s with existing bids", auction.Status)
	}

	// Cancel
	if err := auction.Cancel(); err != nil {
		return err
	}

	// Persist
	if err := s.auctionRepo.UpdateTx(ctx, tx, auction); err != nil {
		return err
	}

	// Emit outbox event
	if err := s.outboxRepo.InsertEvent(
		ctx, tx,
		"auction.cancelled",
		auction.ID,
		buildAuctionPayload(auction),
	); err != nil {
		return fmt.Errorf("failed to insert outbox event: %w", err)
	}

	return nil
}

// PlaceBidInput contains parameters for placing a bid.
type PlaceBidInput struct {
	AuctionID      uuid.UUID
	BidderID       uuid.UUID
	Amount         int64
	IdempotencyKey string
}

// PlaceBid places a bid on an active auction.
//
// LOCK DISCIPLINE:
// - Check idempotency (get existing bid by key)
// - Lock Auction (FOR UPDATE)
// - Validate via entity.PlaceBid()
// - Insert auction_bid
// - Update auction.current_bid
// - Emit outbox event
//
// Must use idempotency_key from request for retry safety.
func (s *AuctionService) PlaceBid(
	ctx context.Context,
	tx db.Tx,
	input PlaceBidInput,
) (*entity.AuctionBid, error) {
	// Validate idempotency key
	if input.IdempotencyKey == "" {
		return nil, fmt.Errorf("idempotency key is required")
	}

	// Validate bidder account status
	if err := s.accountStatus.EnsureActive(ctx, input.BidderID); err != nil {
		return nil, fmt.Errorf("bidder account not active: %w", err)
	}

	// Check for existing bid with same idempotency key scoped to this bidder.
	// Scoping to bidder prevents cross-actor collision: two different bidders
	// using the same key string on the same auction are independent.
	existingBid, err := s.bidRepo.GetByAuctionAndIdempotencyKey(ctx, tx, input.AuctionID, input.BidderID, input.IdempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("failed to check idempotency: %w", err)
	}
	if existingBid != nil {
		// Idempotent replay for this bidder: return their existing bid.
		return existingBid, nil
	}

	// Lock auction
	auction, err := s.auctionRepo.GetForUpdate(ctx, tx, input.AuctionID)
	if err != nil {
		return nil, err
	}

	// Seller market authority gate: bids against an expired seller's auction
	// are rejected up-front so buyers do not waste a bid that the downstream
	// order-creation Guard 6 would block at win-checkout anyway.
	hasCapability, err := s.roleChecker.HasActiveSellerCapability(ctx, auction.SellerID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify seller market authority: %w", err)
	}
	if !hasCapability {
		return nil, auth.ErrMarketAuthorityRequired
	}

	// Validate and place bid via entity
	now := time.Now()
	endAtBeforeBid := auction.EndAt
	if err := auction.PlaceBid(input.BidderID, input.Amount, now); err != nil {
		return nil, err
	}
	extended := auction.EndAt.After(endAtBeforeBid)

	// Create bid entity
	bid, err := entity.NewAuctionBid(
		input.AuctionID,
		input.BidderID,
		input.Amount,
		input.IdempotencyKey,
	)
	if err != nil {
		return nil, err
	}

	// Insert bid
	if err := s.bidRepo.CreateTx(ctx, tx, bid); err != nil {
		// Handle UNIQUE constraint violation for idempotency (per-bidder race).
		if isUniqueViolationError(err) {
			// Load and return this bidder's existing bid.
			existing, loadErr := s.bidRepo.GetByAuctionAndIdempotencyKey(ctx, tx, input.AuctionID, input.BidderID, input.IdempotencyKey)
			if loadErr != nil {
				return nil, fmt.Errorf("idempotency conflict and failed to load existing bid: %w", loadErr)
			}
			return existing, nil
		}
		return nil, fmt.Errorf("failed to create bid: %w", err)
	}

	// Update auction with new current bid
	if err := s.auctionRepo.UpdateTx(ctx, tx, auction); err != nil {
		return nil, fmt.Errorf("failed to update auction: %w", err)
	}

	// Emit outbox event for bid placed
	if err := s.outboxRepo.InsertEvent(
		ctx, tx,
		"auction.bid.placed",
		bid.ID,
		buildAuctionBidPayload(bid),
	); err != nil {
		return nil, fmt.Errorf("failed to insert outbox event: %w", err)
	}

	// Emit outbox event for auction updated (with new current bid)
	if err := s.outboxRepo.InsertEvent(
		ctx, tx,
		"auction.bid.updated",
		auction.ID,
		buildAuctionPayload(auction),
	); err != nil {
		return nil, fmt.Errorf("failed to insert outbox event: %w", err)
	}

	// Soft-close (anti-sniping): emit a distinct event when the bid landed in
	// the closing window and extended EndAt, atomically with bid acceptance,
	// so a future notification consumer can tell bidders/watchers the clock
	// moved (PASS_18C).
	if extended {
		if err := s.outboxRepo.InsertEvent(
			ctx, tx,
			"auction.extended",
			auction.ID,
			buildAuctionExtendedPayload(auction, auction.EndAt.Sub(endAtBeforeBid)),
		); err != nil {
			return nil, fmt.Errorf("failed to insert outbox event: %w", err)
		}
		s.log.Info("Auction soft-close extended",
			zap.String("auction_id", auction.ID.String()),
			zap.Time("new_end_at", auction.EndAt),
			zap.Duration("total_extension", auction.AntiSnipeExtensionTotal),
		)
	}

	s.log.Info("Bid placed",
		zap.String("bid_id", bid.ID.String()),
		zap.String("auction_id", auction.ID.String()),
		zap.String("bidder_id", input.BidderID.String()),
		zap.Int64("amount", input.Amount),
	)

	return bid, nil
}

// CreateOrderFromAuctionInput contains parameters for creating an order from auction.
type CreateOrderFromAuctionInput struct {
	Auction               *entity.Auction
	BuyerID               uuid.UUID
	WinningBid            int64
	AddressID             uuid.UUID // Buyer's shipping address ID
	ShippingSetupID      uuid.UUID
	ProvinceCode          string                            // Deprecated: Use AddressID instead
	CityCode              string                            // Deprecated: Use AddressID instead
	DiscountCode          *string                           // Optional discount code
	UseCoins              bool                              // Whether to use coins
	AuctionSettlementType orderEntity.AuctionSettlementType // buy_now vs bid_win
	PricingSnapshot       *orderApp.PricingSnapshot         // REQUIRED: Pricing snapshot from validated pricing token
	IdempotencyKey        *string                           // Optional: HTTP idempotency key for safe retries
}

// CreateOrderFromAuction creates an order from an ended auction.
// This is called by the auction end worker.
//
// All business logic is delegated to OrderService.CreateFromAuction().
// This method only provides orchestration and parameter passing.
//
// ⚠️ CRITICAL AUCTION SETTLEMENT INVARIANT ⚠️
// This function MUST maintain atomicity to prevent:
// 1. Double-spending: Same auction creating multiple orders
// 2. Inventory leaks: Auction not locked, listing sold elsewhere
// 3. Price manipulation: Bid prices changed after auction ends
//
// ✅ ALWAYS use transactions (tx parameter required)
// ✅ ALWAYS lock auction with FOR UPDATE before calling
// ✅ ALWAYS validate PricingSnapshot is present and valid
// ❌ NEVER call this without proper locking - it WILL cause races
// ❌ NEVER bypass OrderService validation logic
//
// DB CONSTRAINTS ARE FINAL GUARD:
// - UNIQUE(auction_id) on orders prevents duplicate orders
// - CHECK(for_sale_status = 'sold') prevents double-spending
// - NEVER disable these constraints for "performance"
//
// ATOMICITY: All operations happen within the caller-provided transaction.
func (s *AuctionService) CreateOrderFromAuction(
	ctx context.Context,
	tx db.Tx,
	input CreateOrderFromAuctionInput,
) (*orderEntity.Order, error) {
	// CRITICAL: PricingSnapshot is REQUIRED
	if input.PricingSnapshot == nil {
		return nil, fmt.Errorf("pricing_snapshot is required for auction order creation: all orders must use pricing token")
	}

	// Delegate to OrderService for all order creation logic
	// OrderService handles:
	// - Listing locking and validation
	// - Quantity reduction
	// - Order and order item creation
	// - Outbox event emission
	order, err := s.orderService.CreateFromAuction(ctx, tx, orderApp.CreateFromAuctionInput{
		AuctionID:             input.Auction.ID,
		AuctionSellerID:       input.Auction.SellerID,
		ProductID:             input.Auction.ProductID,
		BuyerID:               input.BuyerID,
		WinningBid:            input.WinningBid,
		AddressID:             input.AddressID,
		ShippingSetupID:      input.ShippingSetupID,
		ProvinceCode:          input.ProvinceCode,
		CityCode:              input.CityCode,
		DiscountCode:          input.DiscountCode,
		AuctionSettlementType: input.AuctionSettlementType,
		PricingSnapshot:       input.PricingSnapshot,
		IdempotencyKey:        input.IdempotencyKey,
		ShippingResolvedAt:     auctionShippingResolvedAt(input.Auction),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create order from auction: %w", err)
	}

	s.log.Info("Order created from auction",
		zap.String("order_id", order.ID.String()),
		zap.String("auction_id", input.Auction.ID.String()),
		zap.String("buyer_id", input.BuyerID.String()),
		zap.Int64("winning_bid", input.WinningBid),
	)

	return order, nil
}

// auctionShippingResolvedAt returns the auction's shipping resolution time.
// Order creation requires shipping to be resolved first; if the field is
// unexpectedly nil the caller is about to create an order without a resolution
// anchor — fail closed by returning the zero time (the order layer then treats
// payment expiry conservatively from now).
func auctionShippingResolvedAt(auction *entity.Auction) time.Time {
	if auction.ShippingResolvedAt != nil {
		return *auction.ShippingResolvedAt
	}
	return time.Time{}
}

// sellerQuoteRequiredForWinner determines whether the seller must provide a
// private shipping quote before the winner can resolve shipping (Case A).
//
// The winner's PRIMARY shipping address (purpose='shipping',
// is_available_for_checkout=true) is resolved; when no shipping setup linked
// to the auctioned product covers that destination, a private quote is
// required. Returns false when the winner has no usable primary address yet
// (the buyer resolves shipping — and provides an address — at claim time).
func (s *AuctionService) sellerQuoteRequiredForWinner(
	ctx context.Context,
	tx db.Tx,
	auction *entity.Auction,
) (bool, error) {
	if auction.WinnerID() == nil {
		return false, nil
	}
	winnerID := *auction.WinnerID()

	primary, err := s.addressRepo.GetPrimaryByUserIDFiltered(ctx, tx, winnerID, string(addressEntity.AddressPurposeShipping))
	if err != nil {
		return false, err
	}
	if primary == nil || !primary.IsAvailableForCheckout {
		// Winner has no usable primary address yet. Fail-open to Case B — the
		// winner supplies an address when resolving shipping at claim time.
		return false, nil
	}

	// A shipping setup covers the winner's destination when at least one
	// delivery option is available for the winner's province/city. No usable
	// option means the seller must provide a private quote.
	options, err := s.shippingSvc.CheckDeliveryAvailabilityForProduct(ctx, tx, auction.ProductID, primary.ProvinceID, primary.CityID)
	if err != nil {
		return false, err
	}
	return len(options) == 0, nil
}

// ReturnToDraftOnSettlementFailure atomically returns a waiting_settlement
// auction to DRAFT after a settlement failure. Callers must already hold the
// auction FOR UPDATE. The auction's settlement context (order binding,
// shipping resolution, seller flags, current bid/winner) is cleared on the
// entity; persist via auctionRepo.UpdateTx within the same transaction.
func (s *AuctionService) ReturnToDraftOnSettlementFailure(
	ctx context.Context,
	tx db.Tx,
	auction *entity.Auction,
) error {
	if err := auction.TransitionToDraftOnSettlementFailure(); err != nil {
		return err
	}
	if err := s.auctionRepo.UpdateTx(ctx, tx, auction); err != nil {
		return fmt.Errorf("failed to persist auction return to draft: %w", err)
	}
	return nil
}

// EndAuctionInput contains parameters for ending an auction internally.
// Used by the auction end worker.
type EndAuctionInput struct {
	AuctionID        uuid.UUID
	ShippingSetupID uuid.UUID
	ProvinceCode     string
	CityCode         string
}

// EndAuctionInternal ends an auction and prepares it for settlement.
// This is called by the auction end worker.
//
// For auctions with no bids: simply ends the auction (no order_id set).
// For auctions with bids:
//   - Transitions to waiting_settlement status
//   - SellerActionRequired is classified from the winner's primary-address
//     coverage (seller must provide a private quote when no shipping setup
//     covers the winner's destination)
//   - Winner must resolve shipping (claim) to create the order within the
//     canonical shipping deadline: end_at + 24h
//
// CRITICAL CHANGE: Worker NO LONGER creates orders directly.
// ALL auction orders must be created by the winner via the claim flow.
//
// LOCK DISCIPLINE:
// - Lock Auction (FOR UPDATE)
// - Validate status = active
// - Check NOT already settled (order_id is NULL)
// - Transition status based on winner existence
// - Emit outbox events
//
// SETTLEMENT SAFETY: Once order_id is set, no further order can be created.
func (s *AuctionService) EndAuctionInternal(
	ctx context.Context,
	tx db.Tx,
	input EndAuctionInput,
) error {
	// Lock auction
	auction, err := s.auctionRepo.GetForUpdate(ctx, tx, input.AuctionID)
	if err != nil {
		return err
	}

	// Validate auction is active
	if auction.Status != entity.StatusActive {
		// Already processed, skip
		return nil
	}

	// NOTE: Shipping details are NO LONGER used by worker
	// Worker only transitions auction state
	// Winner must use pricing token flow to create order

	if auction.HasWinner() {
		// Winner exists - transition to waiting_settlement.
		// Winner will resolve shipping (claim) to create the order.
		if err := auction.TransitionToWaitingSettlement(); err != nil {
			return err
		}

		// Canonical Case A/B classification: determine whether the seller must
		// provide a private shipping quote before the winner can resolve
		// shipping. The winner's primary shipping address is resolved and
		// checked against the product's shipping coverage. When no selected
		// shipping setup covers the winner's destination, the seller must act
		// (seller_action_required = true). Fail-open (false) if the winner has
		// no primary address yet or coverage cannot be determined — the
		// buyer-side deadline then applies, and the winner resolves shipping
		// at claim time.
		requiresSellerQuote, err := s.sellerQuoteRequiredForWinner(ctx, tx, auction)
		if err != nil {
			s.log.Warn("seller_action_required determination failed, defaulting false",
				zap.String("auction_id", auction.ID.String()),
				zap.Error(err),
			)
		}
		auction.SellerActionRequired = requiresSellerQuote

		s.log.Info("Auction entered waiting_settlement state",
			zap.String("auction_id", auction.ID.String()),
			zap.String("winner_id", auction.WinnerID().String()),
			zap.Int64("winning_bid", *auction.WinningBid()),
			zap.Bool("seller_action_required", auction.SellerActionRequired),
		)
	} else {
		// No winner - transition to ended
		if err := auction.End(); err != nil {
			return err
		}
		s.log.Info("Auction ended without winner",
			zap.String("auction_id", auction.ID.String()),
		)
	}

	// Persist auction
	if err := s.auctionRepo.UpdateTx(ctx, tx, auction); err != nil {
		return err
	}

	// Emit outbox event for auction status change
	eventType := "auction.ended"
	if auction.Status == entity.StatusWaitingSettlement {
		eventType = "auction.waiting_settlement"
	}
	if err := s.outboxRepo.InsertEvent(
		ctx, tx,
		eventType,
		auction.ID,
		buildAuctionPayload(auction),
	); err != nil {
		return fmt.Errorf("failed to insert outbox event: %w", err)
	}

	return nil
}

// PersistAuctionUpdate persists an already-mutated auction entity within the
// caller's transaction. Used by the one-shot claim handler after setting
// OrderID and calling Settle() on the entity.
func (s *AuctionService) PersistAuctionUpdate(
	ctx context.Context,
	tx db.Tx,
	auction *entity.Auction,
) error {
	return s.auctionRepo.UpdateTx(ctx, tx, auction)
}

// GetAuction retrieves an auction without locking.
func (s *AuctionService) GetAuction(
	ctx context.Context,
	tx db.Tx,
	auctionID uuid.UUID,
) (*entity.Auction, error) {
	return s.auctionRepo.GetByID(ctx, tx, auctionID)
}

// ListBids retrieves bids for an auction.
func (s *AuctionService) ListBids(
	ctx context.Context,
	tx db.Tx,
	auctionID uuid.UUID,
	limit int,
) ([]*entity.AuctionBid, error) {
	return s.bidRepo.ListByAuction(ctx, tx, auctionID, limit)
}

// ListAuctionsFilter holds filter criteria for listing auctions.
type ListAuctionsFilter struct {
	Status   *entity.Status // Filter by status (optional)
	SellerID *uuid.UUID     // Filter by seller ID (optional)
	Cursor   *time.Time     // Cursor for pagination (created_at based)
	Limit    int            // Max results (default 20, max 50)
}

// ListAuctionsResult holds the result of ListAuctions with pagination metadata.
type ListAuctionsResult struct {
	Auctions   []*entity.Auction
	NextCursor *string // RFC3339 timestamp of last item's created_at
	HasMore    bool    // True if there are more results
}

// ListAuctions retrieves auctions with filtering and cursor-based pagination.
// This is a read-only query - no business logic, no locks.
func (s *AuctionService) ListAuctions(
	ctx context.Context,
	tx db.Tx,
	filter ListAuctionsFilter,
) (ListAuctionsResult, error) {
	// Map filter to repository filter
	repoFilter := auctionRepo.AuctionFilter{
		Status:   filter.Status,
		SellerID: filter.SellerID,
		Cursor:   filter.Cursor,
		Limit:    filter.Limit,
	}

	// Fetch from repository (with limit+1 to detect has_more)
	auctions, err := s.auctionRepo.List(ctx, tx, repoFilter)
	if err != nil {
		return ListAuctionsResult{}, err
	}

	// Determine if there are more results
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	hasMore := len(auctions) > limit
	if hasMore {
		// Remove the extra item used only for has_more detection
		auctions = auctions[:limit]
	}

	// Generate next cursor from last item
	var nextCursor *string
	if len(auctions) > 0 {
		lastCreatedAt := auctions[len(auctions)-1].CreatedAt
		cursorStr := lastCreatedAt.Format(time.RFC3339Nano)
		nextCursor = &cursorStr
	}

	return ListAuctionsResult{
		Auctions:   auctions,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// GeneratePricingTokenForAuctionInput contains parameters for generating pricing token for auction claim.
type GeneratePricingTokenForAuctionInput struct {
	AuctionID        uuid.UUID
	WinnerID         uuid.UUID
	AddressID        uuid.UUID
	ShippingSetupID uuid.UUID
}

// GeneratePricingTokenForAuctionClaim generates a pricing token for auction claim.
//
// This is the NEW flow for auction claims:
// 1. Winner validates and gets pricing token
// 2. Winner proceeds to checkout with pricing token
// 3. Order is created using the pricing token
//
// This ensures ALL orders go through pricing token validation.
//
// VALIDATIONS:
// - Auction is in waiting_settlement state
// - Caller is the winner
// - Settlement deadline has not passed
// - NOT already settled (order_id is NULL)
//
// Returns auction data for pricing token generation.
func (s *AuctionService) GeneratePricingTokenForAuctionClaim(
	ctx context.Context,
	tx db.Tx,
	input GeneratePricingTokenForAuctionInput,
) (*entity.Auction, error) {
	// Lock auction for validation
	auction, err := s.auctionRepo.GetForUpdate(ctx, tx, input.AuctionID)
	if err != nil {
		return nil, err
	}

	// SETTLEMENT GUARD: Check if already settled
	if auction.OrderID != nil {
		return nil, entity.ErrAlreadySettled
	}

	// Validate auction is in waiting_settlement state
	if auction.Status != entity.StatusWaitingSettlement {
		return nil, fmt.Errorf("%w: status=%s (expected waiting_settlement)", entity.ErrNotClaimable, auction.Status)
	}

	// Validate the canonical shipping deadline (auction.end_at + 24h) has not
	// passed. Deadline authority is DERIVED — never stored, never extended.
	now := time.Now()
	if now.After(auction.SettlementDeadline()) {
		return nil, fmt.Errorf("%w: deadline=%s", entity.ErrSettlementDeadlinePassed, auction.SettlementDeadline().Format(time.RFC3339))
	}

	// Shipping resolution guard: shipping must be resolved before an order can
	// be created. First-resolution-wins — a claim cannot proceed after shipping
	// has already been resolved by another path.
	if auction.ShippingResolvedAt != nil {
		return nil, entity.ErrShippingAlreadyResolved
	}

	// Validate caller is the winner
	if !auction.HasWinner() {
		return nil, entity.ErrNoWinner
	}
	if auction.WinnerID() == nil || *auction.WinnerID() != input.WinnerID {
		return nil, entity.ErrNotWinner
	}

	// Return auction for pricing token generation
	// The pricing token service will use this to generate the token
	return auction, nil
}

// ActivateScheduledAuctionInput contains parameters for activating a scheduled auction.
type ActivateScheduledAuctionInput struct {
	AuctionID uuid.UUID
}

// ActivateScheduledAuction transitions a scheduled auction to active state.
//
// MARKET AUTHORITY ENFORCEMENT (PHASE 1D):
// - Re-verifies seller subscription at activation time
// - If seller subscription expired: cancels auction instead of activating
// - If seller subscription active: proceeds with activation
//
// This enforces the business rule: "Auction yang baru SCHEDULED saat seller
// expire: tidak boleh masuk live"
//
// LOCK DISCIPLINE:
// - Lock Auction (FOR UPDATE)
// - Verify seller still has market authority
// - Activate or cancel based on authority
// - Emit outbox event
func (s *AuctionService) ActivateScheduledAuction(
	ctx context.Context,
	tx db.Tx,
	input ActivateScheduledAuctionInput,
) error {
	// Lock auction
	auction, err := s.auctionRepo.GetForUpdate(ctx, tx, input.AuctionID)
	if err != nil {
		return err
	}

	// Double-check status is still scheduled (idempotent)
	if auction.Status != entity.StatusScheduled {
		return nil // Already processed
	}

	// MARKET AUTHORITY CHECK: Re-verify seller has active subscription
	// This prevents scheduled auctions from going live if seller expired
	hasCapability, err := s.roleChecker.HasActiveSellerCapability(ctx, auction.SellerID)
	if err != nil {
		return fmt.Errorf("failed to verify market authority: %w", err)
	}

	if !hasCapability {
		// Seller subscription expired - cancel auction instead of activating
		s.log.Info("Seller subscription expired, cancelling scheduled auction",
			zap.String("auction_id", auction.ID.String()),
			zap.String("seller_id", auction.SellerID.String()),
		)

		if err := auction.Cancel(); err != nil {
			return err
		}

		if err := s.auctionRepo.UpdateTx(ctx, tx, auction); err != nil {
			return err
		}

		// Emit outbox event for cancellation
		if err := s.outboxRepo.InsertEvent(
			ctx, tx,
			"auction.cancelled",
			auction.ID,
			buildAuctionPayload(auction),
		); err != nil {
			return fmt.Errorf("failed to insert outbox event: %w", err)
		}

		return nil
	}

	// Seller has active subscription - proceed with activation
	if err := auction.Activate(); err != nil {
		return err
	}

	// Persist activation
	if err := s.auctionRepo.UpdateTx(ctx, tx, auction); err != nil {
		return err
	}

	// Emit outbox event for activation
	if err := s.outboxRepo.InsertEvent(
		ctx, tx,
		"auction.activated",
		auction.ID,
		buildAuctionPayload(auction),
	); err != nil {
		return fmt.Errorf("failed to insert outbox event: %w", err)
	}

	s.log.Info("Auction activated",
		zap.String("auction_id", auction.ID.String()),
		zap.String("seller_id", auction.SellerID.String()),
	)

	return nil
}

// CancelForModeration cancels an auction under governance enforcement authority.
//
// GOVERNANCE BYPASS: Skips IsSeller ownership check and CanCancel bid check.
// This method is only callable from moderation/governance workers — never from
// seller-facing API handlers.
//
// Handles all non-terminal states: draft, scheduled, active, waiting_settlement.
// Terminal states (ended, cancelled) return InvalidTransitionError;
// callers must treat that as idempotent success.
//
// Emits auction.cancelled outbox event for downstream audit trail.
func (s *AuctionService) CancelForModeration(
	ctx context.Context,
	tx db.Tx,
	auctionID uuid.UUID,
) error {
	auction, err := s.auctionRepo.GetForUpdate(ctx, tx, auctionID)
	if err != nil {
		return err
	}

	if err := auction.Cancel(); err != nil {
		// InvalidTransitionError for terminal states — caller handles idempotency
		return err
	}

	if err := s.auctionRepo.UpdateTx(ctx, tx, auction); err != nil {
		return fmt.Errorf("update auction failed: %w", err)
	}

	if s.outboxRepo != nil {
		if err := s.outboxRepo.InsertEvent(
			ctx, tx,
			"auction.cancelled",
			auction.ID,
			buildAuctionPayload(auction),
		); err != nil {
			return fmt.Errorf("insert outbox event failed: %w", err)
		}
	}

	return nil
}

// AdminCancelInput contains parameters for an admin emergency auction cancellation.
type AdminCancelInput struct {
	AuctionID uuid.UUID
	Reason    string
}

// ErrAuctionCancelReasonRequired is returned when an admin cancel request has
// no (or only whitespace) reason. Admin cancel must always be attributable.
var ErrAuctionCancelReasonRequired = fmt.Errorf("reason is required for admin auction cancellation")

// ErrAuctionCancelConflict is returned when an admin cancel is attempted on
// an auction whose current state cannot be safely cancelled without a money,
// order, or dispute-domain reversal (or is already terminal). The caller
// must be told to use the canonical order/dispute/refund path instead.
type ErrAuctionCancelConflict struct {
	AuctionID     uuid.UUID
	CurrentStatus entity.Status
	Reason        string
}

func (e *ErrAuctionCancelConflict) Error() string {
	return fmt.Sprintf("auction %s cannot be admin-cancelled from status %s: %s", e.AuctionID, e.CurrentStatus, e.Reason)
}

// applyAdminCancel is the pure, in-memory decision-and-mutation core of
// AdminCancel, extracted so the safe/conflict state contract is unit
// testable without a repository or transaction. It mutates auction in
// place (via entity.Cancel()) on success and returns an error otherwise.
//
// See AdminCancel's doc comment for the full safe/conflict state contract.
func applyAdminCancel(auction *entity.Auction) error {
	// Defense-in-depth: an order already exists for this auction — money/
	// order resolution must go through the canonical order/dispute/refund
	// path, not this endpoint. In practice unreachable because OrderID is
	// only ever set in the same transaction that transitions status to the
	// terminal `ended` state (see auction_handler.go claim flow), but this
	// guard is cheap insurance against that invariant ever drifting.
	if auction.OrderID != nil {
		return &ErrAuctionCancelConflict{
			AuctionID:     auction.ID,
			CurrentStatus: auction.Status,
			Reason:        "auction already has an order; use the order/dispute/refund path",
		}
	}

	if err := auction.Cancel(); err != nil {
		var ite *entity.InvalidTransitionError
		if errors.As(err, &ite) {
			return &ErrAuctionCancelConflict{
				AuctionID:     auction.ID,
				CurrentStatus: auction.Status,
				Reason:        "auction is already in a terminal state",
			}
		}
		return err
	}

	return nil
}

// AdminCancel cancels an auction under admin governance authority.
//
// GOVERNANCE AUTHORITY, NOT SELLER AUTHORITY: unlike Cancel (seller-facing),
// this does not check auction ownership — any caller holding the
// governance.auction.cancel capability (enforced at the HTTP layer) may
// cancel any seller's auction. This is the emergency-intervention path for
// an unreachable/abusive seller or a trust-and-safety stop, not a
// replacement for the seller's own Cancel.
//
// Unlike CancelForModeration (automated moderation-worker enforcement,
// which treats a terminal-state InvalidTransitionError as idempotent
// success because retries are safe there), this is a human-triggered action
// that must surface a clear, stable conflict instead of silently
// succeeding — an admin clicking "cancel" needs to know whether it worked.
//
// SAFE STATES (bypasses the bid-count restriction in CanCancel(), matching
// CancelForModeration's precedent — cancelling never mutates money, escrow,
// or order state at this stage: PlaceBid only ever writes bid rows and
// auction.current_bid; no ledger/order side effect exists until an order is
// actually created via claim/buy-now):
//   - draft, scheduled, active (with or without bids), waiting_settlement
//     (winner determined; safe only while no order is bound — the non-nil
//     OrderID conflict guard below covers the claimed-but-unpaid case).
//
// CONFLICT STATES (fail closed, return ErrAuctionCancelConflict):
//   - ended, cancelled (already terminal)
//   - any auction with a non-nil OrderID (defense-in-depth: an order
//     already exists — go through the order/dispute/refund domain instead;
//     in practice this is unreachable because a bid-win auction keeps its
//     OrderID bound in waiting_settlement until payment, but this guard is
//     cheap insurance against that invariant ever drifting).
//
// Does NOT delete bids, does NOT delete the auction/product, does NOT touch
// the ledger, does NOT create or cancel an order, does NOT issue a refund.
// Bid history and audit traceability are fully preserved — only the
// auction's own status column changes.
func (s *AuctionService) AdminCancel(
	ctx context.Context,
	tx db.Tx,
	input AdminCancelInput,
) (*entity.Auction, entity.Status, error) {
	if strings.TrimSpace(input.Reason) == "" {
		return nil, "", ErrAuctionCancelReasonRequired
	}

	auction, err := s.auctionRepo.GetForUpdate(ctx, tx, input.AuctionID)
	if err != nil {
		return nil, "", err
	}

	previousStatus := auction.Status

	if err := applyAdminCancel(auction); err != nil {
		return nil, previousStatus, err
	}

	if err := s.auctionRepo.UpdateTx(ctx, tx, auction); err != nil {
		return nil, previousStatus, fmt.Errorf("update auction failed: %w", err)
	}

	if s.outboxRepo != nil {
		if err := s.outboxRepo.InsertEvent(
			ctx, tx,
			"auction.cancelled",
			auction.ID,
			buildAuctionPayload(auction),
		); err != nil {
			return nil, previousStatus, fmt.Errorf("insert outbox event failed: %w", err)
		}
	}

	return auction, previousStatus, nil
}

// isUniqueViolationError checks if the error is a PostgreSQL UNIQUE constraint violation.
func isUniqueViolationError(err error) bool {
	if err == nil {
		return false
	}
	pgErr, ok := err.(*pgconn.PgError)
	return ok && pgErr.Code == "23505" // UNIQUE_VIOLATION
}
