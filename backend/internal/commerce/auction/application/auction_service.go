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
	"github.com/labuda/backend/internal/commerce/governance/commercegov"
	productEntity "github.com/labuda/backend/internal/commerce/product/entity"
	commerceshared "github.com/labuda/backend/internal/commerce/shared"
	shippingApp "github.com/labuda/backend/internal/commerce/shipping/application"
	shippingRepo "github.com/labuda/backend/internal/commerce/shipping/infrastructure/repository"
	addressRepo "github.com/labuda/backend/internal/identity/address/repository"
	"github.com/labuda/backend/internal/identity/auth"
	platformconfigApp "github.com/labuda/backend/internal/platform/config/application"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// ProductCreator creates and mutates canonical product records within a
// transaction. Defined locally so auction/application does not import
// product/infrastructure; dependencies.go wires the concrete
// productRepoImpl.ProductRepositoryImpl.
type ProductCreator interface {
	Create(ctx context.Context, tx db.Tx, product *productEntity.Product) error
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*productEntity.Product, error)
	Update(ctx context.Context, tx db.Tx, product *productEntity.Product) error
	ClaimSellingSurface(ctx context.Context, tx db.Tx, productID uuid.UUID, surface productEntity.SellingSurface) error
}

// AuctionService handles auction state transitions and operations.
// It enforces state machine rules and persists changes with proper locking.
//
// ORDER CREATION IS NOT AN AUCTION CONCERN: auction-sourced orders (buy-now
// and bid-win) are created exclusively by POST /orders → OrderCreationService.
// The auction side only owns settlement eligibility, the settlement window,
// shipping prerequisite flags, and the order binding (auction.OrderID).
//
// MARKET AUTHORITY ENFORCEMENT (PHASE 1B):
//   - Create (create = publish): requires an active seller subscription — the
//     shared market-entry gate runs inside the create transaction
//   - Relist (republish): the identical gate, no exceptions
type AuctionService struct {
	auctionRepo              *auctionRepo.AuctionRepository
	bidRepo                  *auctionRepo.AuctionBidRepository
	shippingSvc              *shippingApp.ShippingService
	shippingSetupRepo        shippingRepo.ShippingSetupRepository
	shippingCoverageRepo     shippingRepo.ShippingCoverageRepository
	productShippingRepo      shippingRepo.ProductShippingSetupRepository
	addressRepo              addressRepo.AddressRepository
	outboxRepo               *outboxRepo.OutboxRepository
	ownership                *auth.OwnershipValidator
	accountStatus            auth.AccountStatusChecker
	roleChecker              auth.RoleChecker
	configService            *platformconfigApp.ConfigService
	productRepo              ProductCreator
	commerceGovRepo          commercegov.Repository   // COMMERCE RESTRICTION: canonical restriction checker
	shippingQuoteInvalidator ShippingQuoteInvalidator // CROSS-LIFECYCLE: invalidate stale quotes on settlement failure
	log                      *zap.Logger
}

// NewAuctionService creates a new AuctionService.
func NewAuctionService(
	accountStatus auth.AccountStatusChecker,
	shippingService *shippingApp.ShippingService,
	shippingSetupRepo shippingRepo.ShippingSetupRepository,
	shippingCoverageRepo shippingRepo.ShippingCoverageRepository,
	productShippingRepo shippingRepo.ProductShippingSetupRepository,
	outboxRepo *outboxRepo.OutboxRepository,
	configService *platformconfigApp.ConfigService,
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
		shippingSvc:          shippingService,
		shippingSetupRepo:    shippingSetupRepo,
		shippingCoverageRepo: shippingCoverageRepo,
		productShippingRepo:  productShippingRepo,
		addressRepo:          addressRepository,
		outboxRepo:           outboxRepo,
		ownership:            auth.NewOwnershipValidator(),
		accountStatus:        accountStatus,
		roleChecker:          roleChecker,
		configService:        configService,
		log:                  log,
	}
}

// SetProductRepo attaches the product creator for inline product creation
// during create. Must be called before any AuctionService.Create invocation.
func (s *AuctionService) SetProductRepo(repo ProductCreator) {
	s.productRepo = repo
}

// SetCommerceGovRepository wires the canonical commerce restriction repository
// into the auction service so seller/bidder restrictions are enforced inside the
// same transaction as the mutation (TOCTOU prevention).
func (s *AuctionService) SetCommerceGovRepository(repo commercegov.Repository) {
	s.commerceGovRepo = repo
}

// ShippingQuoteInvalidator marks ACTIVE shipping quotes for a product as INVALID.
// Used during settlement failure (auto-reschedule) to prevent stale quotes from
// being usable in the next settlement lifecycle.
type ShippingQuoteInvalidator interface {
	InvalidateQuotesByProduct(ctx context.Context, tx db.Tx, productID uuid.UUID) error
}

// SetShippingQuoteInvalidator wires the shipping quote invalidation capability
// for cross-lifecycle isolation on settlement failure.
func (s *AuctionService) SetShippingQuoteInvalidator(invalidator ShippingQuoteInvalidator) {
	s.shippingQuoteInvalidator = invalidator
}

// requireSellerNotRestricted checks whether the given seller has an active
// commerce restriction. Must be called inside the same transaction as the
// commerce mutation. Returns auth.ErrCommerceRestricted when restricted.
func (s *AuctionService) requireSellerNotRestricted(ctx context.Context, tx db.Tx, sellerID uuid.UUID) error {
	if s.commerceGovRepo == nil {
		return nil // repo not wired — fail-open for backward compat
	}
	restricted, _, err := commercegov.IsUserRestricted(ctx, tx, s.commerceGovRepo, sellerID)
	if err != nil {
		return fmt.Errorf("commerce restriction check failed: %w", err)
	}
	if restricted {
		return auth.ErrCommerceRestricted
	}
	return nil
}

// requireUserNotRestricted checks whether the given user has an active
// commerce restriction. Used for bidder/buyer restriction checks.
// Must be called inside the same transaction as the commerce mutation.
func (s *AuctionService) requireUserNotRestricted(ctx context.Context, tx db.Tx, userID uuid.UUID) error {
	if s.commerceGovRepo == nil {
		return nil // repo not wired — fail-open for backward compat
	}
	restricted, _, err := commercegov.IsUserRestricted(ctx, tx, s.commerceGovRepo, userID)
	if err != nil {
		return fmt.Errorf("commerce restriction check failed: %w", err)
	}
	if restricted {
		return auth.ErrCommerceRestricted
	}
	return nil
}

// buildAuctionPayload creates a JSON payload for auction events.
//
// cancel_reason is stamped ONLY on auction.cancelled events (empty/omitted
// for every other lifecycle event — matching entity.CancelReasonLegacy).
// See entity.CancelReason for the authority vocabulary — an internal
// outbox-audit field, never a public wire field and never a notification
// routing key.
func buildAuctionPayload(auction *entity.Auction, cancelReason ...entity.CancelReason) []byte {
	type payload struct {
		AuctionID     string  `json:"auction_id"`
		SellerID      string  `json:"seller_id"`
		ProductID     string  `json:"product_id"`
		Status        string  `json:"status"`
		StartPrice    int64   `json:"start_price"`
		CurrentBid    *int64  `json:"current_bid,omitempty"`
		CurrentWinner *string `json:"current_winner,omitempty"`
		CancelReason  string  `json:"cancel_reason,omitempty"`
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
	if len(cancelReason) > 0 {
		p.CancelReason = string(cancelReason[0])
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

// ErrBuyNowBelowFloor is returned when buy_now_price is below
// start_price + bid_increment. The canonical pricing floor for BOTH create and
// relist-republish — a client-fixable request error (handlers map it to 400),
// never a server fault.
var ErrBuyNowBelowFloor = fmt.Errorf("buy_now_price must be >= start_price + bid_increment")

// CreateAuctionInput contains parameters for creating an auction.
// By default a Product is created inline from the product fields. When
// ProductID is set (Product identity reuse), the auction attaches to that
// existing Product instead of minting a new one; the product must exist and
// belong to the seller.
type CreateAuctionInput struct {
	SellerID uuid.UUID
	// ProductID (optional) — Product identity reuse target.
	ProductID *uuid.UUID
	// Product fields — created atomically with the auction unless reused
	Title            string
	Description      string
	Media            []productEntity.ProductMedia
	Variety          string
	SizeCM           *int
	AgeMonths        *int
	Gender           *string
	Breeder          *string
	Bloodline        *string
	Certificates     []string
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
}

// Create creates a new auction. CREATE = PUBLISH: there is no draft state —
// the auction is constructed already scheduled (or activated immediately for
// an immediate start) after clearing the shared market-entry gate.
//
// LOCK DISCIPLINE:
// - Validate product reference
// - Create auction
// - Emit outbox event
//
// All operations happen within the same transaction for atomicity.
func (s *AuctionService) Create(
	ctx context.Context,
	tx db.Tx,
	input CreateAuctionInput,
) (*entity.Auction, error) {
	// Validate seller account status
	if err := s.accountStatus.EnsureActive(ctx, input.SellerID); err != nil {
		return nil, fmt.Errorf("seller account not active: %w", err)
	}

	// COMMERCE RESTRICTION: Reject restricted seller before any auction creation.
	// Checked at the creation boundary; scheduleAuctionInternal re-checks inside
	// the same transaction for activation paths.
	if err := s.requireSellerNotRestricted(ctx, tx, input.SellerID); err != nil {
		return nil, err
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
		return nil, ErrBuyNowBelowFloor
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
		existing, err := s.productRepo.GetByID(ctx, tx, *input.ProductID)
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
		// Canonical Product validation for mint
		if err := productEntity.ValidateTitle(&input.Title); err != nil {
			return nil, err
		}
		if err := productEntity.ValidateDescription(&input.Description); err != nil {
			return nil, err
		}
		if input.Certificates != nil {
			if err := productEntity.ValidateCertificates(&input.Certificates); err != nil {
				return nil, err
			}
		}
		if pt := string(input.PreparationTime); pt != "" {
			if err := productEntity.ValidatePreparationTime(&pt); err != nil {
				return nil, err
			}
		}
		media := input.Media
		if media == nil {
			media = []productEntity.ProductMedia{}
		}
		product := &productEntity.Product{
			SellerID:        input.SellerID,
			Title:           input.Title,
			Description:     input.Description,
			MediaURLs:       media,
			Variety:         input.Variety,
			SizeCm:          input.SizeCM,
			AgeMonths:       input.AgeMonths,
			Gender:          input.Gender,
			Breeder:         input.Breeder,
			Bloodline:       input.Bloodline,
			Certificates:    input.Certificates,
			PreparationTime: string(input.PreparationTime),
			SellingSurface:  productEntity.SellingSurfaceAuction,
		}
		if err := s.productRepo.Create(ctx, tx, product); err != nil {
			return nil, fmt.Errorf("failed to create product: %w", err)
		}
		productID = product.ID
	}

	// Create the auction — already in its initial market state (scheduled);
	// Product content (title, description, koi attributes,
	// preparation, media) is owned by Product entity, not by Auction.
	auction := entity.NewScheduled(
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

	// CREATE = PUBLISH: the auction reaches the single market-entry gate
	// (ownership + restriction + market authority + shipping coverage) in this
	// same transaction, so no path can bypass it. It persists as scheduled and,
	// for an immediate start, progresses straight through to active below.
	if err := s.validateScheduleGates(ctx, tx, auction, input.SellerID); err != nil {
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

// validateScheduleGates performs the shared ownership + COMMERCE RESTRICTION +
// MARKET AUTHORITY + SHIPPING COVERAGE checks required before an auction may
// (re)enter the market. It performs no state mutation, so every entry path —
// create and relist-republish — runs the identical gate. There is no separate
// schedule step: create IS the schedule (create = publish).
func (s *AuctionService) validateScheduleGates(
	ctx context.Context,
	tx db.Tx,
	auction *entity.Auction,
	callerID uuid.UUID,
) error {
	// Validate ownership
	if !s.ownership.IsSeller(callerID, auction.SellerID) {
		return auth.ErrSellerRequired
	}

	// COMMERCE RESTRICTION: Reject restricted seller before schedule/activation.
	// Checked inside the same transaction as the state mutation (TOCTOU prevention).
	if err := s.requireSellerNotRestricted(ctx, tx, callerID); err != nil {
		return err
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
	// than surprising them at checkout time.
	return s.ensureShippingCoverage(ctx, tx, auction.ProductID)
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

// UpdateScheduledInput contains parameters for updating a scheduled auction.
// Allowed: Product content (title/description) + timing (start_at/end_at).
// Pricing is immutable once scheduled.
type UpdateScheduledInput struct {
	AuctionID   uuid.UUID
	CallerID    uuid.UUID
	Title       *string
	Description *string
	StartAt     time.Time
	EndAt       time.Time
}

// UpdateScheduled updates a scheduled auction (restricted fields) and its
// Product content atomically.
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

	// Lifecycle guard before any Product mutation
	if auction.Status != entity.StatusScheduled {
		return &entity.InvalidOperationError{Status: auction.Status, Reason: "can only update scheduled auctions"}
	}

	// Canonical Product validation (title/description only for scheduled)
	patch := productEntity.ProductContentPatch{
		Title:       input.Title,
		Description: input.Description,
	}
	if err := patch.Validate(); err != nil {
		return err
	}

	// Product content authority: update products.title/description when provided.
	if input.Title != nil || input.Description != nil {
		if s.productRepo == nil {
			return fmt.Errorf("product repo not wired for auction scheduled update")
		}
		product, err := s.productRepo.GetByID(ctx, tx, auction.ProductID)
		if err != nil {
			return fmt.Errorf("failed to load product for auction update: %w", err)
		}
		if product.SellerID != auction.SellerID {
			return fmt.Errorf("product ownership mismatch")
		}
		patch.ApplyTo(product)
		product.UpdatedAt = time.Now()
		if err := s.productRepo.Update(ctx, tx, product); err != nil {
			return fmt.Errorf("failed to update product: %w", err)
		}
	}

	// Update scheduled auction timing
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
		buildAuctionPayload(auction, entity.CancelReasonSeller),
	); err != nil {
		return fmt.Errorf("failed to insert outbox event: %w", err)
	}

	return nil
}

// RelistInput contains parameters for REPUBLISHING a finished auction.
//
// Create-like payload by owner decision: relist IS the create form run again
// (autofilled, duration re-chosen), so it carries the same run-shaping
// authorities as create — timing (start mode + duration) and pricing — plus an
// optional Product content patch. Product identity and shipping selection are
// deliberately NOT part of it: the auction's Product is already bound and its
// shipping coverage is re-validated by the schedule gate, not re-selected.
type RelistInput struct {
	AuctionID uuid.UUID
	CallerID  uuid.UUID
	// Timing (create authority: entity.ResolveAuctionTiming)
	StartMode        entity.StartMode
	ScheduledStartAt *time.Time // required only when StartMode == StartModeScheduled
	Duration         time.Duration
	// Pricing (create authority: start price, increment, buy-now floor)
	StartPrice   int64
	BidIncrement int64
	BuyNowPrice  *int64
	// Product content patch — nil fields keep the Product's current values.
	Title           *string
	Description     *string
	Media           *[]productEntity.ProductMedia
	Variety         *string
	SizeCM          *int
	AgeMonths       *int
	Gender          *string
	Breeder         *string
	Bloodline       *string
	Certificates    *[]string
	PreparationTime *string
}

// Relist REPUBLISHES a finished auction: ended (no bid/winner/order) or
// lapsed -> scheduled (or active for start_mode=now), with a fresh run.
//
// OWNER BUSINESS TRUTH: relist = republish via the create form. Same record,
// new run, NO draft detour — the create-form UX must behave identically the
// second time around.
//
// LOCK DISCIPLINE:
//   - Lock Auction (FOR UPDATE)
//   - Shared schedule gates (ownership, restriction, market authority, coverage)
//   - Canonical timing + pricing validation (identical to create)
//   - Optional Product content patch (identical to update authority)
//   - Entity gate + lifecycle reset (entity.Relist)
//   - Invalidate stale shipping quotes, persist, emit auction.scheduled
//     (+ auction.activated for start_mode=now — same events as create)
func (s *AuctionService) Relist(
	ctx context.Context,
	tx db.Tx,
	input RelistInput,
) (*entity.Auction, error) {
	auction, err := s.auctionRepo.GetForUpdate(ctx, tx, input.AuctionID)
	if err != nil {
		return nil, err
	}

	// SAME GATE AS CREATE/SCHEDULE: an auction may only (re)enter the market
	// with live market authority, an unrestricted seller, and coverable shipping.
	if err := s.validateScheduleGates(ctx, tx, auction, input.CallerID); err != nil {
		return nil, err
	}

	// Canonical timing authority (identical to create): server clock is the
	// only "now", 1-7 day duration bound, future scheduled start, 30-day horizon.
	startAt, endAt, err := entity.ResolveAuctionTiming(input.StartMode, input.ScheduledStartAt, input.Duration, time.Now())
	if err != nil {
		return nil, err
	}

	// Canonical pricing floor (identical to create).
	if input.BuyNowPrice != nil && *input.BuyNowPrice < input.StartPrice+input.BidIncrement {
		return nil, ErrBuyNowBelowFloor
	}

	// Product content authority: apply the provided patch before the run commits.
	patch := productEntity.ProductContentPatch{
		Title:           input.Title,
		Description:     input.Description,
		MediaURLs:       input.Media,
		Variety:         input.Variety,
		SizeCM:          input.SizeCM,
		AgeMonths:       input.AgeMonths,
		Gender:          input.Gender,
		Breeder:         input.Breeder,
		Bloodline:       input.Bloodline,
		Certificates:    input.Certificates,
		PreparationTime: input.PreparationTime,
	}
	if err := patch.Validate(); err != nil {
		return nil, err
	}
	hasProductContent := input.Title != nil || input.Description != nil || input.Media != nil || input.Variety != nil ||
		input.SizeCM != nil || input.AgeMonths != nil || input.Gender != nil || input.Breeder != nil ||
		input.Bloodline != nil || input.Certificates != nil || input.PreparationTime != nil
	if hasProductContent {
		if s.productRepo == nil {
			return nil, fmt.Errorf("product repo not wired for auction relist")
		}
		product, err := s.productRepo.GetByID(ctx, tx, auction.ProductID)
		if err != nil {
			return nil, fmt.Errorf("failed to load product for auction relist: %w", err)
		}
		if product.SellerID != auction.SellerID {
			return nil, fmt.Errorf("product ownership mismatch")
		}
		patch.ApplyTo(product)
		product.UpdatedAt = time.Now()
		if err := s.productRepo.Update(ctx, tx, product); err != nil {
			return nil, fmt.Errorf("failed to update product on relist: %w", err)
		}
	}

	// ENTITY: gate (ended-no-outcome | lapsed) + lifecycle reset + new run.
	if err := auction.Relist(startAt, endAt, input.StartPrice, input.BidIncrement, input.BuyNowPrice); err != nil {
		return nil, err
	}

	// CROSS-LIFECYCLE: drop quotes produced by the finished run so no stale
	// quote can price the republished auction.
	if s.shippingQuoteInvalidator != nil {
		if err := s.shippingQuoteInvalidator.InvalidateQuotesByProduct(ctx, tx, auction.ProductID); err != nil {
			return nil, fmt.Errorf("failed to invalidate shipping quotes for relist: %w", err)
		}
	}

	if err := s.auctionRepo.UpdateTx(ctx, tx, auction); err != nil {
		return nil, fmt.Errorf("failed to persist auction relist: %w", err)
	}
	if err := s.outboxRepo.InsertEvent(
		ctx, tx,
		"auction.scheduled",
		auction.ID,
		buildAuctionPayload(auction),
	); err != nil {
		return nil, fmt.Errorf("failed to insert outbox event: %w", err)
	}

	// start_mode=now republishes straight to live — same event pair as create.
	if input.StartMode == entity.StartModeNow {
		if err := auction.Activate(); err != nil {
			return nil, err
		}
		if err := s.auctionRepo.UpdateTx(ctx, tx, auction); err != nil {
			return nil, fmt.Errorf("failed to activate relisted auction: %w", err)
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

	s.log.Info("Auction republished",
		zap.String("auction_id", auction.ID.String()),
		zap.String("seller_id", auction.SellerID.String()),
		zap.String("status", string(auction.Status)),
	)

	return auction, nil
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

	// COMMERCE RESTRICTION: Reject restricted bidder before any bid mutation.
	// Checked inside the same transaction as the bid to prevent TOCTOU bypass.
	if err := s.requireUserNotRestricted(ctx, tx, input.BidderID); err != nil {
		return nil, err
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

// sellerQuoteRequiredForWinner determines whether the seller must provide a
// private shipping quote before the winner can resolve shipping (Case A).
//
// The winner's PRIMARY shipping address (purpose='shipping',
// is_available_for_checkout=true) is resolved; when no shipping setup linked
// to the auctioned product covers that destination, a private quote is
// required. Returns false when the winner has no usable primary address yet
// (the buyer resolves shipping — and provides an address — at checkout time).
func (s *AuctionService) sellerQuoteRequiredForWinner(
	ctx context.Context,
	tx db.Tx,
	auction *entity.Auction,
) (bool, error) {
	if auction.WinnerID() == nil {
		return false, nil
	}
	winnerID := *auction.WinnerID()

	primary, err := s.addressRepo.GetPrimaryByUserID(ctx, tx, winnerID)
	if err != nil {
		return false, err
	}
	if primary == nil || !primary.IsAvailableForCheckout {
		// Winner has no usable primary address yet. Fail-open to Case B — the
		// winner supplies an address when resolving shipping at checkout time.
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

// RescheduleAfterSettlementFailure atomically auto-reschedules a
// waiting_settlement auction after a settlement failure: settlement context is
// cleared on the entity and the auction re-enters the market at start=now,
// end=now+previous duration. Callers must already hold the auction FOR UPDATE.
// Persist via auctionRepo.UpdateTx within the same transaction; the caller
// emits auction.settlement_failed as the audit event for this transition.
func (s *AuctionService) RescheduleAfterSettlementFailure(
	ctx context.Context,
	tx db.Tx,
	auction *entity.Auction,
) error {
	if err := auction.RescheduleAfterSettlementFailure(); err != nil {
		return err
	}
	// CROSS-LIFECYCLE ISOLATION: invalidate all ACTIVE shipping quotes
	// for this product so no stale quote from the previous settlement
	// lifecycle can be used in the next lifecycle.
	if s.shippingQuoteInvalidator != nil {
		if err := s.shippingQuoteInvalidator.InvalidateQuotesByProduct(ctx, tx, auction.ProductID); err != nil {
			return fmt.Errorf("failed to invalidate shipping quotes on settlement failure: %w", err)
		}
	}
	if err := s.auctionRepo.UpdateTx(ctx, tx, auction); err != nil {
		return fmt.Errorf("failed to persist auction reschedule after settlement failure: %w", err)
	}
	return nil
}

// EndAuctionInput contains parameters for ending an auction internally.
// Used by the auction end worker. Shipping details are NOT worker concerns:
// the winner supplies them in the shared Checkout (POST /orders bid-win).
type EndAuctionInput struct {
	AuctionID uuid.UUID
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
//   - Winner must complete the shared Checkout (POST /orders bid-win) inside
//     the canonical settlement window: end_at + 24h
//
// The worker NEVER creates orders: auction-sourced orders are created only by
// POST /orders, which binds auction.OrderID in the same transaction.
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

	// REVALIDATION (F22D-001): phase-1 discovery used DB NOW() but the
	// authoritative eligibility must be proven after the row is locked.
	// If a concurrent PlaceBid extended EndAt (anti-sniping) after phase-1,
	// the auction is no longer expired and must NOT be ended here.
	if time.Now().Before(auction.EndAt) {
		s.log.Info("Auction no longer expired, skipping stale end",
			zap.String("auction_id", auction.ID.String()),
			zap.Time("end_at", auction.EndAt),
		)
		return nil
	}

	// NOTE: Shipping details are NOT worker concerns: the worker only
	// transitions auction state. The winner supplies address + shipping in the
	// shared Checkout (POST /orders bid-win).
	if auction.HasWinner() {
		// Winner exists - transition to waiting_settlement. The winner
		// completes the shared Checkout to create the order.
		if err := auction.TransitionToWaitingSettlement(); err != nil {
			return err
		}

		// Canonical Case A/B classification: determine whether the seller must
		// provide a private shipping quote before the winner can complete
		// checkout. The winner's primary shipping address is resolved and
		// checked against the product's shipping coverage. When no selected
		// shipping setup covers the winner's destination, the seller must act
		// (seller_action_required = true). Fail-open (false) if the winner has
		// no primary address yet or coverage cannot be determined — the
		// settlement-window deadline then applies, and the winner resolves
		// shipping at checkout time.
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

// GetAuction retrieves an auction without locking.
func (s *AuctionService) GetAuction(
	ctx context.Context,
	tx db.Tx,
	auctionID uuid.UUID,
) (*entity.Auction, error) {
	return s.auctionRepo.GetByID(ctx, tx, auctionID)
}

// PublicOriginLine returns the buyer-facing origin summary ("City, Province")
// for an auction detail read. The rule itself lives once in
// commerceshared.PublicListingOrigin (account primary address,
// city+province only) so the auction and for_sale surfaces cannot drift.
func (s *AuctionService) PublicOriginLine(
	ctx context.Context,
	tx db.Tx,
	auction *entity.Auction,
) string {
	if auction == nil {
		return ""
	}
	return commerceshared.PublicListingOrigin(ctx, tx, s.addressRepo, auction.Product)
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

	// OwnerInventory marks a seller reading their OWN list with no explicit
	// status: every status is returned, including waiting_settlement, ended,
	// cancelled and lapsed. Set only by the handler when seller_id == viewer.
	// It never widens public browse — that keeps the public discovery default.
	OwnerInventory bool
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
		Status:         filter.Status,
		SellerID:       filter.SellerID,
		Cursor:         filter.Cursor,
		Limit:          filter.Limit,
		OwnerInventory: filter.OwnerInventory,
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

	// REVALIDATION (F22D-003): phase-1 discovery used DB NOW() but the
	// authoritative eligibility must be proven after the row is locked.
	// If a concurrent UpdateScheduled moved start_at into the future after
	// phase-1, the auction is no longer eligible and must NOT be activated.
	if time.Now().Before(auction.StartAt) {
		s.log.Info("Auction start_at in future, skipping stale activation",
			zap.String("auction_id", auction.ID.String()),
			zap.Time("start_at", auction.StartAt),
		)
		return nil
	}

	// COMMERCE RESTRICTION: Reject restricted seller at activation boundary.
	// Checked inside the same transaction as the state mutation.
	if err := s.requireSellerNotRestricted(ctx, tx, auction.SellerID); err != nil {
		return err
	}

	// MARKET AUTHORITY CHECK: Re-verify seller has active subscription
	// This prevents scheduled auctions from going live if seller expired
	hasCapability, err := s.roleChecker.HasActiveSellerCapability(ctx, auction.SellerID)
	if err != nil {
		return fmt.Errorf("failed to verify market authority: %w", err)
	}

	if !hasCapability {
		// Seller market authority expired before activation. Owner decision
		// (Oct 2026): LAPSE, not cancel — the seller took no action. The
		// auction never went live; it is hidden from viewer surfaces and
		// relistable after renewal. Deliberately emits no outbox event: the
		// seller already receives the global seller.subscription.expired
		// notification, and this outcome is not an auction.cancelled.
		s.log.Info("Seller market authority lapsed, holding scheduled auction",
			zap.String("auction_id", auction.ID.String()),
			zap.String("seller_id", auction.SellerID.String()),
		)

		if err := auction.Lapse(); err != nil {
			return err
		}

		if err := s.auctionRepo.UpdateTx(ctx, tx, auction); err != nil {
			return err
		}

		return nil
	}

	// SHIPPING COVERAGE REVALIDATION (F22D-002 / F2.2F - fail-closed):
	// Scheduling requires at least one active coverage. Seller may deactivate
	// or delete coverages / unlink options after scheduling but before start_at.
	// Revalidate against authoritative current state after row lock; if no longer
	// shippable, keep scheduled and allow future retry when seller restores coverage.
	// Uses canonical ensureShippingCoverage (at least one option has is_available=true).
	// Fail-closed: required shipping dependencies must be present; missing repo is a
	// system error, not a silent skip.
	if s.productShippingRepo == nil || s.shippingCoverageRepo == nil {
		return fmt.Errorf("auction shipping validation requires productShippingRepo and shippingCoverageRepo")
	}
	if err := s.ensureShippingCoverage(ctx, tx, auction.ProductID); err != nil {
		if errors.Is(err, shippingApp.ErrShippingNotConfigured) {
			s.log.Info("Auction shipping coverage not available, skipping activation",
				zap.String("auction_id", auction.ID.String()),
				zap.String("product_id", auction.ProductID.String()),
				zap.Error(err),
			)
			return nil
		}
		return fmt.Errorf("failed to validate shipping coverage: %w", err)
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
// Handles all non-cancellable states: scheduled, active, waiting_settlement.
// Non-cancellable states (ended, cancelled) return InvalidTransitionError;
// callers must treat that as idempotent success.
//
//	// Emits auction.cancelled outbox event for downstream audit trail.
//
// Reason: governance enforcement (outcome also travels the canonical
// moderation channel).
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
			buildAuctionPayload(auction, entity.CancelReasonModeration),
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
	// settlement state in the POST /orders transaction (bid-win keeps waiting_settlement; buy-now ends), but this
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
// actually created via bid-win checkout / buy-now):
//   - scheduled, active (with or without bids), waiting_settlement
//     (winner determined; safe only while no order is bound — the non-nil
//     OrderID conflict guard below covers the bound-but-unpaid case).
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
			buildAuctionPayload(auction, entity.CancelReasonAdmin),
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
