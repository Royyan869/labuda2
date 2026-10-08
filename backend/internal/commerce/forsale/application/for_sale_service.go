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
	commerceshared "github.com/labuda/backend/internal/commerce/shared"
	shippingApp "github.com/labuda/backend/internal/commerce/shipping/application"
	shippingRepo "github.com/labuda/backend/internal/commerce/shipping/infrastructure/repository"
	shippingquoteRepo "github.com/labuda/backend/internal/commerce/shipping/quote/repository"
	addressRepoInterface "github.com/labuda/backend/internal/identity/address/repository"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/platform/events"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

// ErrSellerOriginNotConfigured is returned when a seller publishes without a
// primary address. Every product's origin is the account's primary address, so
// a seller with no address yet cannot publish.
var ErrSellerOriginNotConfigured = errors.New("SELLER_ORIGIN_NOT_CONFIGURED: seller requires a primary address before publishing")

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

// ensureWorkspaceAuthorityTx enforces workspace authority for create:
// active account (not suspended/banned/removed) + verified email + seller profile exists.
// Uses tx for TOCTOU safety; does NOT require active subscription (that is the
// market-authority gate, checked separately inside the same transaction).
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
	Media              []productEntity.ProductMedia
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
	// Shipping selection (OWNER CANONICAL: create ships WITH its options —
	// validated ownership + ≥1 active coverage, then linked to the product
	// inside the same transaction. Create without options is rejected.)
	ShippingSetupIDs []uuid.UUID
	// Shipping readiness
	PreparationTime entity.PreparationTime
}

// Create creates a new for_sale that is IMMEDIATELY LIVE (status=active,
// visibility=public) — OWNER CANONICAL: a seller who completes the create
// form is publishing; there is no draft-first creation path.
//
// AUTHORITY MODEL (OWNER CANONICAL):
// - MARKET AUTHORITY: HasActiveSellerCapability (active + not deleted +
//   profile + active subscription interval) is REQUIRED at create. An
//   expired seller cannot create at all.//   - SHIPPING: at least one shipping option (validated for ownership +
//   active coverage) must be selected and is linked to the product in the
//   same transaction. Farm/sender address must be valid.
//
// There is no draft state: a created for_sale is active and public
// immediately, so the seller never observes a workspace stage.
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
	if err := s.CheckMarketAuthorityForForSale(ctx, input.SellerID); err != nil {
		return nil, err
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
	)
	if err != nil {
		return nil, fmt.Errorf("create for_sale entity failed: %w", err)
	}

	// CREATE = PUBLISH: the constructor already produced an ACTIVE + PUBLIC
	// for_sale (there is no draft stage to transition from), so the seller
	// never observes a draft during create.

	// Product handling — Product is the sole persistence authority for content.
	// Explicit split: mint a new Product or reuse an existing one, then attach ForSale.
	mediaURLs := input.Media
	if mediaURLs == nil {
		mediaURLs = []productEntity.ProductMedia{}
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
			PreparationTime: string(input.PreparationTime),
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
	if err := s.EnsureSellerOriginValid(ctx, tx, for_sale.SellerID); err != nil {
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

// CheckMarketAuthorityForForSale checks if the seller has authority to hold a
// market-visible for_sale. Used at the create boundary — ACTIVE = PUBLIC ONLY
// (all for_sales are public from the moment they exist).
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

// UpdateSellerInput carries the canonical seller-edit vocabulary for a
// for_sale (content + price/stock-adjacent fields). Quantity is intentionally
// absent (stock-only path). No field is mutable anymore — see UpdateSeller.
type UpdateSellerInput struct {
	ForSaleID          uuid.UUID
	SellerID           uuid.UUID
	Title              *string
	Description        *string
	Price              *int64
	NegotiationEnabled *bool
	Media              *[]productEntity.ProductMedia
	Variety            *string
	SizeCM             *int
	AgeMonths          *int
	Gender             *string
	Breeder            *string
	Bloodline          *string
	Certificates       *[]string
	PreparationTime    *string
}

// UpdateSeller is the canonical seller-edit authority for ForSale.
//
// CREATE = PUBLISH: there is no draft state, so every persisted for_sale is
// live and seller content edits are rejected (live immutability invariant).
// The canonical lock + ownership + commerce-restriction checks still run
// first, so the caller always receives the most specific domain error.
//
// Flow: GetForUpdate → ownership → commerce restriction → LIVE_IMMUTABLE.
func (s *ForSaleService) UpdateSeller(
	ctx context.Context,
	tx db.Tx,
	input UpdateSellerInput,
) (*entity.ForSale, error) {
	// Acquire canonical lock on for_sale.
	forSale, err := s.repo.GetForUpdate(ctx, tx, input.ForSaleID)
	if err != nil {
		return nil, err
	}
	// Ownership
	if forSale.SellerID != input.SellerID {
		return nil, fmt.Errorf("forbidden: you can only update your own for_sales")
	}
	// Commerce restriction inside same tx
	if err := s.requireSellerNotRestricted(ctx, tx, input.SellerID); err != nil {
		return nil, err
	}
	// LIVE IMMUTABILITY: a for_sale is born published (create = publish), so
	// no status is an editable state. This is the single rejection authority
	// for PUT /for-sale/:id content edits.
	return nil, fmt.Errorf("%w: status=%s", entity.ErrLiveImmutable, forSale.Status)
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

// EnsureSellerOriginValid validates that the seller has a primary address
// configured before publish. Every product's origin is the account's primary
// address, so a seller with no address cannot publish.
//
// Returns ErrSellerOriginNotConfigured (wrapped) so handlers can branch via
// errors.Is and surface the SELLER_ORIGIN_NOT_CONFIGURED error code.
func (s *ForSaleService) EnsureSellerOriginValid(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
) error {
	address, err := s.addressRepo.GetPrimaryByUserID(ctx, tx, sellerID)
	if err != nil {
		return fmt.Errorf("failed to resolve seller primary address: %w", ErrSellerOriginNotConfigured)
	}
	if address == nil {
		return fmt.Errorf("seller has no primary address: %w", ErrSellerOriginNotConfigured)
	}

	return nil
}

// PublicOriginLine returns the buyer-facing origin summary ("City, Province")
// for a for_sale detail read. The rule itself lives once in
// commerceshared.PublicListingOrigin (account primary address, city+province
// only) so the for_sale and auction surfaces cannot drift.
func (s *ForSaleService) PublicOriginLine(
	ctx context.Context,
	tx db.Tx,
	for_sale *entity.ForSale,
) string {
	if for_sale == nil {
		return ""
	}
	return commerceshared.PublicListingOrigin(ctx, tx, s.addressRepo, for_sale.Product)
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
