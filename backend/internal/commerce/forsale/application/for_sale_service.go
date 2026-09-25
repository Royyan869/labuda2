package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/forsale/entity"
	"github.com/labuda/backend/internal/commerce/forsale/infrastructure/repository"
	for_saleRepo "github.com/labuda/backend/internal/commerce/forsale/repository"
	"github.com/labuda/backend/internal/commerce/governance/commercegov"
	productEntity "github.com/labuda/backend/internal/commerce/product/entity"
	productInfraRepo "github.com/labuda/backend/internal/commerce/product/infrastructure/repository"
	productRepo "github.com/labuda/backend/internal/commerce/product/repository"
	shippingApp "github.com/labuda/backend/internal/commerce/shipping/application"
	shippingRepo "github.com/labuda/backend/internal/commerce/shipping/infrastructure/repository"
	shippingquoteRepo "github.com/labuda/backend/internal/commerce/shipping/quote/repository"
	addressEntity "github.com/labuda/backend/internal/identity/address/entity"
	addressRepoInterface "github.com/labuda/backend/internal/identity/address/repository"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/platform/events"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

// ErrFarmAddressNotConfigured is returned when a for_sale is published without
// a valid farm/sender address. The seller must set farm_address_id to an
// address they own with purpose="sender" before publishing.
var ErrFarmAddressNotConfigured = errors.New("FARM_ADDRESS_NOT_CONFIGURED: for_sale requires a valid sender address before publishing")

// ForSaleService handles for_sale business operations.
//
// STOCK RESTORATION POLICY:
// RestoreStock is intentionally NOT exposed here.
// Stock restoration is ONLY allowed through:
// - OrderService.Cancel() - when order is cancelled
// - OrderService.Expire() - when payment expires
//
// This ensures stock restoration is always paired with order lifecycle events.
//
// MARKET AUTHORITY ENFORCEMENT (PHASE 1B):
// Creating or updating for_sales with PUBLIC visibility requires active seller subscription.
// Private for_sales can be created/updated without active subscription (workspace safety).
//
// PRODUCT AUTHORITY (FASE 2.1): Product is the sole persistence authority for
// title/description/media/koi/farm/preparation. ForSale never stores a copy.
// Unified PUT /for-sale edits Product content via ProductRepository and
// ForSale surface via ForSaleRepository in the same transaction — no bridge.
type ForSaleService struct {
	repo                for_saleRepo.ForSaleRepository
	productRepo         productRepo.ProductRepository
	outboxRepo          *outboxRepo.OutboxRepository
	roleChecker         auth.RoleChecker
	shippingSetupRepo   shippingRepo.ShippingSetupRepository
	productShippingRepo shippingRepo.ProductShippingSetupRepository
	coverageRepo        shippingRepo.ShippingCoverageRepository
	shippingQuoteRepo   shippingquoteRepo.ShippingQuoteRepository
	addressRepo         addressRepoInterface.AddressRepository
	commerceGovRepo     commercegov.Repository // COMMERCE RESTRICTION: canonical restriction checker
}

// NewForSaleService creates a new ForSaleService.
func NewForSaleService(args ...any) *ForSaleService {
	svc := &ForSaleService{
		repo:        repository.NewForSaleRepository(),
		productRepo: productInfraRepo.NewProductRepository(),
	}

	for _, arg := range args {
		switch v := arg.(type) {
		case *outboxRepo.OutboxRepository:
			svc.outboxRepo = v
		case auth.RoleChecker:
			svc.roleChecker = v
		case shippingRepo.ShippingSetupRepository:
			svc.shippingSetupRepo = v
		case shippingRepo.ProductShippingSetupRepository:
			svc.productShippingRepo = v
		case shippingRepo.ShippingCoverageRepository:
			svc.coverageRepo = v
		case shippingquoteRepo.ShippingQuoteRepository:
			svc.shippingQuoteRepo = v
		case addressRepoInterface.AddressRepository:
			svc.addressRepo = v
		case commercegov.Repository:
			svc.commerceGovRepo = v
		case productRepo.ProductRepository:
			svc.productRepo = v
		case *productInfraRepo.ProductRepositoryImpl:
			svc.productRepo = v
		}
	}

	return svc
}

// SetCommerceGovRepository wires the canonical commerce restriction repository
// into the for-sale service so seller restrictions are enforced at the create/
// publish mutation boundaries (TOCTOU prevention).
func (s *ForSaleService) SetCommerceGovRepository(repo commercegov.Repository) {
	s.commerceGovRepo = repo
}

// requireSellerNotRestricted checks whether the given seller has an active
// commerce restriction. Must be called inside the same transaction as the
// commerce mutation. Returns auth.ErrCommerceRestricted when restricted.
func (s *ForSaleService) requireSellerNotRestricted(ctx context.Context, tx db.Tx, sellerID uuid.UUID) error {
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

// ensureWorkspaceAuthorityTx enforces workspace authority for private/draft creation:
// active account (not suspended/banned/removed) + verified email + seller profile exists.
// Uses tx for TOCTOU safety; does NOT require active subscription.
func (s *ForSaleService) ensureWorkspaceAuthorityTx(ctx context.Context, tx db.Tx, sellerID uuid.UUID) error {
	if sellerID == uuid.Nil {
		return auth.ErrInvalidCaller
	}
	var accountStatus string
	var deletedAt *time.Time
	var emailVerifiedAt *time.Time
	err := tx.QueryRow(ctx, `SELECT account_status, deleted_at, email_verified_at FROM users WHERE id = $1`, sellerID).Scan(&accountStatus, &deletedAt, &emailVerifiedAt)
	if err != nil {
		return fmt.Errorf("failed to verify account: %w", err)
	}
	if deletedAt != nil {
		return auth.ErrAccountRemoved
	}
	switch accountStatus {
	case "active":
	default:
		if accountStatus == "suspended" {
			return auth.ErrAccountSuspended
		}
		if accountStatus == "banned" {
			return auth.ErrAccountBanned
		}
		return auth.ErrAccountInactive
	}
	if emailVerifiedAt == nil {
		return auth.ErrSellerNotReady
	}
	var hasProfile bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM seller_profiles WHERE user_id = $1)`, sellerID).Scan(&hasProfile)
	if err != nil {
		return fmt.Errorf("failed to check seller profile: %w", err)
	}
	if !hasProfile {
		return auth.ErrSellerNotReady
	}
	return nil
}

// buildForSaleEventPayload creates a JSON payload for fixed-price sale events.
// Product is the sole authority for title/variety — read from for_sale.Product.
func buildForSaleEventPayload(for_sale *entity.ForSale) []byte {
	type payload struct {
		ForSaleID string `json:"for_sale_id"`
		SellerID  string `json:"seller_id"`
		Status    string `json:"status,omitempty"`
		Title     string `json:"title,omitempty"`
		Variety   string `json:"variety,omitempty"`
		Price     int64  `json:"price,omitempty"`
	}
	title, variety := "", ""
	if for_sale.Product != nil {
		title = for_sale.Product.Title
		variety = for_sale.Product.Variety
	}
	p := payload{
		ForSaleID: for_sale.ID.String(),
		SellerID:  for_sale.SellerID.String(),
		Status:    string(for_sale.Status),
		Title:     title,
		Variety:   variety,
		Price:     for_sale.PricePerUnit.Int64(),
	}
	b, _ := json.Marshal(p)
	return b
}

// CreateForSaleInput contains the parameters for creating a for_sale.
type CreateForSaleInput struct {
	SellerID uuid.UUID
	// ProductID (optional) — Product identity reuse. When set, the new
	// fixed-price sale attaches to this existing Product instead of minting
	// a new one. The Product must exist and belong to the seller. When nil,
	// a Product is minted inline (legacy per-attempt behavior).
	ProductID          *uuid.UUID
	Title              string
	Description        string
	MediaURLs          []string
	Variety            string
	SizeCM             *int
	AgeMonths          *int
	Gender             *string
	Breeder            *string
	Bloodline          *string
	Certificates       []string
	PricePerUnit       money.Money
	QuantityAvailable  int
	NegotiationEnabled bool
	// Shipping preferences
	FarmAddressID *uuid.UUID
	// Shipping selection (OWNER CANONICAL: create ships WITH its options —
	// validated ownership + ≥1 active coverage, then linked to the product
	// inside the same transaction. Create without options is rejected.)
	ShippingSetupIDs []uuid.UUID
	// Shipping readiness
	PreparationTime entity.PreparationTime
	PreparationNote *string
}

// Create creates a new for_sale that is IMMEDIATELY LIVE (status=active,
// visibility=public) — OWNER CANONICAL: a seller who completes the create
// form is publishing; there is no draft-first creation path.
//
// AUTHORITY MODEL (OWNER CANONICAL):
// - MARKET AUTHORITY: HasActiveSellerCapability (active + not deleted +
//   profile + active subscription interval) is REQUIRED at create. An
//   expired seller cannot create at all.
// - SHIPPING: at least one shipping option (validated for ownership +
//   active coverage) must be selected and is linked to the product in the
//   same transaction. Farm/sender address must be valid.
// Draft exists ONLY as a system-imposed demotion state when a subscription
// lapses — never as a seller-chosen creation outcome.
func (s *ForSaleService) Create(
	ctx context.Context,
	tx db.Tx,
	input CreateForSaleInput,
) (*entity.ForSale, error) {
	// WORKSPACE AUTHORITY: verified email + active account + seller profile.
	// Transactional checks via tx (TOCTOU-safe) – not via stale Actor projection.
	if err := s.ensureWorkspaceAuthorityTx(ctx, tx, input.SellerID); err != nil {
		return nil, err
	}

	// COMMERCE RESTRICTION: Reject restricted seller at creation boundary.
	// Checked inside the same transaction as the for_sale mutation (TOCTOU prevention).
	if err := s.requireSellerNotRestricted(ctx, tx, input.SellerID); err != nil {
		return nil, err
	}

	// MARKET AUTHORITY CHECK: canonical create requires market eligibility.
	// Expired sellers are rejected up-front — they cannot create at all.
	hasCapability, err := s.roleChecker.HasActiveSellerCapability(ctx, input.SellerID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify market authority: %w", err)
	}
	if !hasCapability {
		return nil, auth.ErrMarketAuthorityRequired
	}

	// SHIPPING SELECTION: canonical create ships WITH its options. Reject
	// empty selections and validate ownership + ≥1 active coverage per option.
	if len(input.ShippingSetupIDs) == 0 {
		return nil, shippingApp.ErrShippingNotConfigured
	}
	validatedShippingIDs, err := shippingApp.ValidateSellableCreateShippingSelection(
		ctx, tx, s.shippingSetupRepo, s.coverageRepo,
		input.SellerID, input.ShippingSetupIDs,
	)
	if err != nil {
		return nil, err
	}

	// Create ForSale surface entity — surface-only, no hidden Product creation.
	// Product content is handled explicitly below via ProductRepository.
	for_sale, err := entity.NewForSaleSurface(
		input.SellerID,
		input.PricePerUnit,
		input.QuantityAvailable,
		input.NegotiationEnabled,
		entity.ForSaleVisibilityPublic,
	)
	if err != nil {
		return nil, fmt.Errorf("create for_sale entity failed: %w", err)
	}

	// CREATE = PUBLISH: transition draft → active immediately. The entity
	// still owns the transition (and the ACTIVE=PUBLIC hard rule); the seller
	// never observes a draft stage during create.
	if err := for_sale.Publish(); err != nil {
		return nil, fmt.Errorf("publish-on-create failed: %w", err)
	}

	// Product handling — Product is the sole persistence authority for content.
	// Explicit split: mint a new Product or reuse an existing one, then attach ForSale.
	mediaURLs := input.MediaURLs
	if mediaURLs == nil {
		mediaURLs = []string{}
	}
	certificates := input.Certificates
	if certificates == nil {
		certificates = []string{}
	}
	if input.ProductID != nil {
		// Reuse path: attach to existing Product
		product, err := s.productRepo.GetByID(ctx, tx, *input.ProductID)
		if err != nil {
			return nil, fmt.Errorf("reuse product failed: %w", err)
		}
		if product.SellerID != input.SellerID {
			return nil, fmt.Errorf("cannot attach for_sale to product owned by another seller")
		}
		if err := s.productRepo.ClaimSellingSurface(ctx, tx, product.ID, productEntity.SellingSurfaceForSale); err != nil {
			return nil, fmt.Errorf("cannot attach for_sale to product: %w", err)
		}
		for_sale.ProductID = product.ID
		for_sale.Product = product
	} else {
		// Mint path: create Product inline with content fields
		product := &productEntity.Product{
			SellerID:        input.SellerID,
			Title:           input.Title,
			Description:     input.Description,
			MediaURLs:       mediaURLs,
			Variety:         input.Variety,
			SizeCm:          input.SizeCM,
			AgeMonths:       input.AgeMonths,
			Gender:          input.Gender,
			Breeder:         input.Breeder,
			Bloodline:       input.Bloodline,
			Certificates:    certificates,
			FarmAddressID:   input.FarmAddressID,
			PreparationTime: string(input.PreparationTime),
			PreparationNote: input.PreparationNote,
			SellingSurface:  productEntity.SellingSurfaceForSale,
		}
		if err := s.productRepo.Create(ctx, tx, product); err != nil {
			return nil, fmt.Errorf("create product failed: %w", err)
		}
		for_sale.ProductID = product.ID
		for_sale.Product = product
	}

	// Persist the for_sale surface (product already persisted)
	if err := s.repo.Create(ctx, tx, for_sale); err != nil {
		return nil, fmt.Errorf("persist for_sale failed: %w", err)
	}

	// Link validated shipping options to the product in the same transaction.
	if err := s.productShippingRepo.CreateBulk(ctx, tx, for_sale.ProductID, validatedShippingIDs); err != nil {
		return nil, fmt.Errorf("link shipping options failed: %w", err)
	}

	// Publish gates enforced at the create boundary too (defense in depth):
	// active for_sales must be purchasable and shippable the moment they exist.
	if err := s.EnsureShippingConfigured(ctx, tx, for_sale.ProductID); err != nil {
		return nil, err
	}
	if err := s.EnsureFarmAddressValid(ctx, tx, for_sale); err != nil {
		return nil, err
	}

	// Emit for_sale.created event
	if s.outboxRepo != nil {
		if err := s.outboxRepo.InsertEvent(
			ctx, tx,
			events.EventForSaleCreated,
			for_sale.ID,
			buildForSaleEventPayload(for_sale),
		); err != nil {
			return nil, fmt.Errorf("failed to insert for_sale.created event: %w", err)
		}
	}

	return for_sale, nil
}

// GetByID retrieves a for_sale by ID (read-only).
func (s *ForSaleService) GetByID(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (*entity.ForSale, error) {
	return s.repo.GetByID(ctx, tx, id)
}

// CheckMarketAuthorityForForSale checks if the seller has authority to publish for_sales.
// This is used by the Publish method since ACTIVE = PUBLIC ONLY (all active for_sales are public).
//
// Returns nil if seller has authority, ErrMarketAuthorityRequired otherwise.
func (s *ForSaleService) CheckMarketAuthorityForForSale(ctx context.Context, sellerID uuid.UUID) error {
	hasCapability, err := s.roleChecker.HasActiveSellerCapability(ctx, sellerID)
	if err != nil {
		return fmt.Errorf("failed to verify market authority: %w", err)
	}
	if !hasCapability {
		return auth.ErrMarketAuthorityRequired
	}
	return nil
}

// GetForUpdate retrieves a for_sale with FOR UPDATE lock.
// This must be used within a transaction for stock mutations.
func (s *ForSaleService) GetForUpdate(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (*entity.ForSale, error) {
	return s.repo.GetForUpdate(ctx, tx, id)
}

// UpdateSellerInput contains the seller-controlled content fields that may be
// mutated only while the ForSale is in draft. All fields are optional — only
// non-nil fields are applied. Quantity is intentionally absent (stock-only path).
type UpdateSellerInput struct {
	ForSaleID          uuid.UUID
	SellerID           uuid.UUID
	Title              *string
	Description        *string
	Price              *int64
	NegotiationEnabled *bool
	MediaURLs          *[]string
	Variety            *string
	SizeCM             *int
	AgeMonths          *int
	Gender             *string
	Breeder            *string
	Bloodline          *string
	Certificates       *[]string
	PreparationTime    *string
	PreparationNote    *string
}

// UpdateSeller is the canonical seller-edit authority for ForSale.
// It enforces draft-only mutability inside a single transaction with row lock.
//
// Flow: GetForUpdate → ownership → status==draft → commerce restriction →
// apply Product + ForSale mutations → persist both → commit.
func (s *ForSaleService) UpdateSeller(
	ctx context.Context,
	tx db.Tx,
	input UpdateSellerInput,
) (*entity.ForSale, error) {
	// Acquire canonical lock on for_sale + joined product.
	forSale, err := s.repo.GetForUpdate(ctx, tx, input.ForSaleID)
	if err != nil {
		return nil, err
	}
	if forSale.Product == nil {
		return nil, fmt.Errorf("for_sale product not loaded")
	}
	// Ownership
	if forSale.SellerID != input.SellerID {
		return nil, fmt.Errorf("forbidden: you can only update your own for_sales")
	}
	// Lifecycle: seller edit allowed IFF status == draft
	if forSale.Status != entity.ForSaleStatusDraft {
		return nil, fmt.Errorf("%w: status=%s", entity.ErrLiveImmutable, forSale.Status)
	}
	// Commerce restriction inside same tx
	if err := s.requireSellerNotRestricted(ctx, tx, input.SellerID); err != nil {
		return nil, err
	}
	// Canonical Product validation (title 1-200, desc 5000, certs, prep)
	patch := productEntity.ProductContentPatch{
		Title:           input.Title,
		Description:     input.Description,
		MediaURLs:       input.MediaURLs,
		Variety:         input.Variety,
		SizeCM:          input.SizeCM,
		AgeMonths:       input.AgeMonths,
		Gender:          input.Gender,
		Breeder:         input.Breeder,
		Bloodline:       input.Bloodline,
		Certificates:    input.Certificates,
		PreparationTime: input.PreparationTime,
		PreparationNote: input.PreparationNote,
	}
	if err := patch.Validate(); err != nil {
		return nil, err
	}
	// Apply Product-owned mutations via canonical helper
	product := forSale.Product
	patch.ApplyTo(product)
	// Apply ForSale-owned mutations
	if input.Price != nil {
		forSale.PricePerUnit = money.New(*input.Price)
	}
	if input.NegotiationEnabled != nil {
		forSale.NegotiationEnabled = *input.NegotiationEnabled
	}
	// Persist both authorities atomically — product first, then surface.
	product.UpdatedAt = time.Now()
	if err := s.productRepo.Update(ctx, tx, product); err != nil {
		return nil, fmt.Errorf("update product failed: %w", err)
	}
	forSale.UpdatedAt = time.Now()
	// Use narrow persistence that does not re-validate status transition (already done).
	// Call repo.Update directly to avoid double restriction/transition checks that
	// would interfere with draft-only guarantee. Emit event via repo.Update-like path.
	if err := s.repo.Update(ctx, tx, forSale); err != nil {
		return nil, err
	}
	if s.outboxRepo != nil {
		if err := s.outboxRepo.InsertEvent(
			ctx, tx,
			events.EventForSaleUpdated,
			forSale.ID,
			buildForSaleEventPayload(forSale),
		); err != nil {
			return nil, fmt.Errorf("failed to insert for_sale.updated event: %w", err)
		}
	}
	return forSale, nil
}

// Withdraw withdraws a for_sale from sale.
func (s *ForSaleService) Withdraw(
	ctx context.Context,
	tx db.Tx,
	for_saleID uuid.UUID,
) error {
	for_sale, err := s.repo.GetForUpdate(ctx, tx, for_saleID)
	if err != nil {
		return err
	}

	if err := for_sale.MarkWithdrawn(); err != nil {
		return err
	}

	if err := s.repo.UpdateStatus(ctx, tx, for_sale); err != nil {
		return err
	}

	// Invalidate all active shipping quotes for this for_sale
	if s.shippingQuoteRepo != nil {
		if err := s.shippingQuoteRepo.InvalidateQuotesByProduct(ctx, tx, for_sale.ProductID); err != nil {
			return fmt.Errorf("failed to invalidate shipping quotes: %w", err)
		}
	}

	// Emit for_sale.withdrawn event
	if s.outboxRepo != nil {
		if err := s.outboxRepo.InsertEvent(
			ctx, tx,
			events.EventForSaleWithdrawn,
			for_sale.ID,
			buildForSaleEventPayload(for_sale),
		); err != nil {
			return fmt.Errorf("failed to insert for_sale.withdrawn event: %w", err)
		}
	}

	return nil
}

// RestoreFromModeration restores a for_sale to active after a successful
// moderation appeal. This is the canonical moderation-authority restoration
// path and must only be called from the moderation event handler.
//
// GUARD: delegates to ForSale.MarkActiveFromModeration() which rejects sold
// for_sales and is a no-op on already-active for_sales.
//
// IDEMPOTENT: safe to retry — MarkActiveFromModeration() is idempotent for
// already-active for_sales.
func (s *ForSaleService) RestoreFromModeration(
	ctx context.Context,
	tx db.Tx,
	for_saleID uuid.UUID,
) error {
	for_sale, err := s.repo.GetForUpdate(ctx, tx, for_saleID)
	if err != nil {
		return fmt.Errorf("for_sale not found for moderation restore: %w", err)
	}

	if err := for_sale.MarkActiveFromModeration(); err != nil {
		return err
	}

	if err := s.repo.UpdateStatus(ctx, tx, for_sale); err != nil {
		return fmt.Errorf("failed to persist for_sale restore: %w", err)
	}

	return nil
}

// EnsureShippingConfigured blocks publish when the product has zero linked
// shipping options, or when every linked option has zero active coverage rows.
//
// Two-level guard:
//  1. At least one shipping option must be linked to the product.
//  2. At least one linked option must have at least one coverage with
//     is_available=true. An option with only inactive coverages cannot serve
//     any buyer address, making the for_sale effectively non-purchasable.
//
// Returns shippingApp.ErrShippingNotConfigured in both failure cases so that
// the handler can surface a single SHIPPING_NOT_CONFIGURED error code.
// Repository transport errors bubble up wrapped with %w.
func (s *ForSaleService) EnsureShippingConfigured(
	ctx context.Context,
	tx db.Tx,
	productID uuid.UUID,
) error {
	count, err := s.productShippingRepo.CountByProduct(ctx, tx, productID)
	if err != nil {
		return fmt.Errorf("failed to check shipping options: %w", err)
	}
	if count == 0 {
		return shippingApp.ErrShippingNotConfigured
	}

	// Verify at least one linked option has active coverage.
	options, err := s.productShippingRepo.GetByProduct(ctx, tx, productID)
	if err != nil {
		return fmt.Errorf("failed to load shipping options for coverage check: %w", err)
	}
	for _, opt := range options {
		coverages, err := s.coverageRepo.GetByShippingSetup(ctx, tx, opt.ID)
		if err != nil {
			return fmt.Errorf("failed to load coverage for option %s: %w", opt.ID, err)
		}
		for _, c := range coverages {
			if c.IsAvailable {
				return nil // At least one option can serve buyers — publish is safe.
			}
		}
	}
	return shippingApp.ErrShippingNotConfigured
}

// EnsureFarmAddressValid validates that the for_sale's Product has a valid farm/sender
// address configured before publish. Checks:
//   - Product.FarmAddressID is set (not nil)
//   - The referenced address exists
//   - The address belongs to the seller (ownership)
//   - The address has purpose="sender"
//
// Returns ErrFarmAddressNotConfigured (wrapped) so handlers can branch via
// errors.Is and surface the FARM_ADDRESS_NOT_CONFIGURED error code.
func (s *ForSaleService) EnsureFarmAddressValid(
	ctx context.Context,
	tx db.Tx,
	for_sale *entity.ForSale,
) error {
	if for_sale.Product == nil || for_sale.Product.FarmAddressID == nil {
		return fmt.Errorf("farm_address_id is required: %w", ErrFarmAddressNotConfigured)
	}

	address, err := s.addressRepo.GetByID(ctx, tx, *for_sale.Product.FarmAddressID)
	if err != nil {
		return fmt.Errorf("farm address not found: %w", ErrFarmAddressNotConfigured)
	}

	if address.UserID != for_sale.SellerID {
		return fmt.Errorf("farm address does not belong to seller: %w", ErrFarmAddressNotConfigured)
	}

	if address.Purpose != addressEntity.AddressPurposeSender {
		return fmt.Errorf("farm address must have purpose 'sender': %w", ErrFarmAddressNotConfigured)
	}

	return nil
}

// Publish publishes a for_sale from draft to active (market-visible).
// This is the EXPLICIT publish boundary - for_sales do NOT auto-publish.
//
// HARD RULE: ACTIVE = PUBLIC ONLY
// Publishing automatically sets visibility to public (enforced by entity.Publish()).
// Market authority check is always required since active for_sales are always public.
func (s *ForSaleService) Publish(
	ctx context.Context,
	tx db.Tx,
	for_saleID uuid.UUID,
	callerID uuid.UUID,
) error {
	// Lock the for_sale for update
	for_sale, err := s.repo.GetForUpdate(ctx, tx, for_saleID)
	if err != nil {
		return err
	}

	// Ownership check
	if for_sale.SellerID != callerID {
		return fmt.Errorf("for_sale does not belong to caller")
	}

	// COMMERCE RESTRICTION: Reject restricted seller at publish boundary.
	// Checked inside the same transaction as the status mutation (TOCTOU prevention).
	if err := s.requireSellerNotRestricted(ctx, tx, callerID); err != nil {
		return err
	}

	// HARD RULE: Market authority check is ALWAYS required for publish
	// because ACTIVE = PUBLIC ONLY (no such thing as private active for_sale)
	if err := s.CheckMarketAuthorityForForSale(ctx, callerID); err != nil {
		return err
	}

	// Shipping options check: product must have at least one shipping option configured.
	// Returns the typed shippingApp.ErrShippingNotConfigured so handlers can branch
	// via errors.Is and surface the SHIPPING_NOT_CONFIGURED error code.
	if err := s.EnsureShippingConfigured(ctx, tx, for_sale.ProductID); err != nil {
		return err
	}

	// Farm address check: for_sale must have a valid sender address configured.
	// Returns the typed ErrFarmAddressNotConfigured so handlers can branch
	// via errors.Is and surface the FARM_ADDRESS_NOT_CONFIGURED error code.
	if err := s.EnsureFarmAddressValid(ctx, tx, for_sale); err != nil {
		return err
	}

	// Transition to published (automatically sets visibility to public)
	if err := for_sale.Publish(); err != nil {
		return err
	}

	// Persist the status change
	if err := s.repo.UpdateStatus(ctx, tx, for_sale); err != nil {
		return err
	}

	// Emit for_sale.published event
	if s.outboxRepo != nil {
		if err := s.outboxRepo.InsertEvent(
			ctx, tx,
			events.EventForSalePublished,
			for_sale.ID,
			buildForSaleEventPayload(for_sale),
		); err != nil {
			return fmt.Errorf("failed to insert for_sale.published event: %w", err)
		}
	}

	return nil
}

// GetBySellerIDPaginated retrieves for_sales for a seller with SQL-based pagination.
// When includeWithdrawn is false, withdrawn for_sales are excluded from results.
func (s *ForSaleService) GetBySellerIDPaginated(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
	limit, offset int,
	includeWithdrawn bool,
) ([]*entity.ForSale, error) {
	return s.repo.GetBySellerIDPaginated(ctx, tx, sellerID, limit, offset, includeWithdrawn)
}

// GetPublic retrieves public active for_sales.
func (s *ForSaleService) GetPublic(
	ctx context.Context,
	tx db.Tx,
	limit, offset int,
) ([]*entity.ForSale, error) {
	return s.repo.GetPublic(ctx, tx, limit, offset)
}

// GetPublicBySellerID retrieves public discoverable for_sales of one seller
// (active + in-stock). Non-owner public seller page lookups only — never the
// seller inventory/owner surface.
func (s *ForSaleService) GetPublicBySellerID(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
	limit, offset int,
) ([]*entity.ForSale, error) {
	return s.repo.GetPublicBySellerID(ctx, tx, sellerID, limit, offset)
}

// SearchResult holds the search results with pagination metadata.
type SearchResult struct {
	ForSales   []*entity.ForSale
	NextCursor *string // RFC3339 timestamp for next page
	HasMore    bool    // True if there are more results
}

// Search performs full-text search on for_sales.
func (s *ForSaleService) Search(
	ctx context.Context,
	tx db.Tx,
	filters for_saleRepo.SearchFilters,
) (*SearchResult, error) {
	for_sales, nextCursor, err := s.repo.Search(ctx, tx, filters)
	if err != nil {
		return nil, err
	}

	var nextCursorStr *string
	if nextCursor != nil {
		cursor := nextCursor.Format(time.RFC3339Nano)
		nextCursorStr = &cursor
	}

	return &SearchResult{
		ForSales:   for_sales,
		NextCursor: nextCursorStr,
		HasMore:    nextCursor != nil,
	}, nil
}
