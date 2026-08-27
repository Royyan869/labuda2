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
	shippingApp "github.com/labuda/backend/internal/commerce/shipping/application"
	shippingRepo "github.com/labuda/backend/internal/commerce/shipping/infrastructure/repository"
	shippingquoteRepo "github.com/labuda/backend/internal/commerce/shipping/quote/repository"
	addressEntity "github.com/labuda/backend/internal/identity/address/entity"
	addressRepoInterface "github.com/labuda/backend/internal/identity/address/repository"
	"github.com/labuda/backend/internal/identity/auth"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
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
type ForSaleService struct {
	repo                for_saleRepo.ForSaleRepository
	outboxRepo          *outboxRepo.OutboxRepository
	roleChecker         auth.RoleChecker
	actorResolver       capabilityEntity.ActorResolver
	productShippingRepo shippingRepo.ProductShippingOptionRepository
	coverageRepo        shippingRepo.ShippingCoverageRepository
	shippingQuoteRepo   shippingquoteRepo.ShippingQuoteRepository
	addressRepo         addressRepoInterface.AddressRepository
}

// NewForSaleService creates a new ForSaleService.
func NewForSaleService(args ...any) *ForSaleService {
	svc := &ForSaleService{
		repo: repository.NewForSaleRepository(),
	}

	for _, arg := range args {
		switch v := arg.(type) {
		case *outboxRepo.OutboxRepository:
			svc.outboxRepo = v
		case auth.RoleChecker:
			svc.roleChecker = v
		case capabilityEntity.ActorResolver:
			svc.actorResolver = v
		case shippingRepo.ProductShippingOptionRepository:
			svc.productShippingRepo = v
		case shippingRepo.ShippingCoverageRepository:
			svc.coverageRepo = v
		case shippingquoteRepo.ShippingQuoteRepository:
			svc.shippingQuoteRepo = v
		case addressRepoInterface.AddressRepository:
			svc.addressRepo = v
		}
	}

	return svc
}

// buildForSaleEventPayload creates a JSON payload for fixed-price sale events.
func buildForSaleEventPayload(for_sale *entity.ForSale) []byte {
	type payload struct {
		ForSaleID string `json:"for_sale_id"`
		SellerID         string `json:"seller_id"`
		Status           string `json:"status,omitempty"`
		Title            string `json:"title,omitempty"`
		Variety          string `json:"variety,omitempty"`
		Price            int64  `json:"price,omitempty"`
	}
	p := payload{
		ForSaleID: for_sale.ID.String(),
		SellerID:         for_sale.SellerID.String(),
		Status:           string(for_sale.Status),
		Title:            for_sale.Title,
		Variety:          for_sale.Variety,
		Price:            for_sale.PricePerUnit.Int64(),
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
	ForSaleType entity.ForSaleType
	PricePerUnit       money.Money
	QuantityAvailable  int
	NegotiationEnabled bool
	Visibility         entity.ForSaleVisibility
	Origin             entity.ForSaleOrigin // Source/context of for_sale creation
	// Location
	CityID     *uuid.UUID
	ProvinceID *uuid.UUID
	Latitude   *float64
	Longitude  *float64
	// Shipping preferences
	FarmAddressID *uuid.UUID
	// Shipping readiness
	PreparationTime entity.PreparationTime
	PreparationNote *string
}

// Create creates a new for_sale.
//
// MARKET AUTHORITY ENFORCEMENT (PHASE 1B):
// - Public visibility requires active seller subscription (hasMarketAuthority)
// - Private visibility can be created without active subscription (workspace safety)
// Expired sellers can create drafts but cannot publish to market.
//
// HARD RULE VALIDATION:
// - New for_sales are always created in draft status
// - Draft for_sales should be private (workspace-only)
// - Active + private combination is rejected
func (s *ForSaleService) Create(
	ctx context.Context,
	tx db.Tx,
	input CreateForSaleInput,
) (*entity.ForSale, error) {
	// SERVICE LAYER ENFORCEMENT: Check seller can create for_sales
	// This checks: account active, email verified, seller subscription active
	actor, err := s.actorResolver.ResolveActor(ctx, input.SellerID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve actor: %w", err)
	}
	if !actor.CanCreateForSale() {
		return nil, auth.ErrSellerNotReady
	}

	// MARKET AUTHORITY CHECK: Public for_sales require active seller subscription
	if input.Visibility == entity.ForSaleVisibilityPublic {
		hasCapability, err := s.roleChecker.HasActiveSellerCapability(ctx, input.SellerID)
		if err != nil {
			return nil, fmt.Errorf("failed to verify market authority: %w", err)
		}
		if !hasCapability {
			return nil, auth.ErrMarketAuthorityRequired
		}
	}

	// Convert media URLs to JSONB
	// Guard: Ensure media URLs is never nil (database requires array)
	mediaURLs := input.MediaURLs
	if mediaURLs == nil {
		mediaURLs = []string{}
	}
	mediaURLsJSON, err := json.Marshal(mediaURLs)
	if err != nil {
		return nil, fmt.Errorf("marshal media urls failed: %w", err)
	}

	// Create the for_sale entity
	// Default origin to DirectCreate if not specified
	origin := input.Origin
	if origin == "" {
		origin = entity.ForSaleOriginDirectCreate
	}

	// Guard: Ensure certificates is never nil (entity expects non-nil slice)
	certificates := input.Certificates
	if certificates == nil {
		certificates = []string{}
	}

	for_sale, err := entity.NewForSale(
		input.SellerID,
		input.Title,
		input.Description,
		mediaURLsJSON,
		input.Variety,
		input.SizeCM,
		input.AgeMonths,
		input.Gender,
		input.Breeder,
		input.Bloodline,
		certificates,
		input.ForSaleType,
		input.PricePerUnit,
		input.QuantityAvailable,
		input.NegotiationEnabled,
		input.Visibility,
		origin,
		// Shipping preferences
		input.FarmAddressID,
		// Shipping readiness
		input.PreparationTime,
		input.PreparationNote,
	)
	if err != nil {
		return nil, fmt.Errorf("create for_sale entity failed: %w", err)
	}

	// HARD RULE: Validate that for_sale was not created in invalid state
	// (entity.NewForSale creates for_sales in draft status, so this is a sanity check)
	if for_sale.Status == entity.ForSaleStatusActive && for_sale.Visibility == entity.ForSaleVisibilityPrivate {
		return nil, fmt.Errorf("invalid for_sale: active status requires public visibility")
	}

	// Product identity reuse: when the caller supplies an existing ProductID,
	// attach this sale to that Product instead of minting a new one. The
	// repository resolves + ownership-checks the product and skips the mint.
	if input.ProductID != nil {
		for_sale.ProductID = *input.ProductID
	}

	// Persist the for_sale
	if err := s.repo.Create(ctx, tx, for_sale); err != nil {
		return nil, fmt.Errorf("persist for_sale failed: %w", err)
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

// Update updates an existing for_sale.
//
// HARD RULE VALIDATION: Rejects active + private combination.
// Active for_sales MUST be public (enforced invariant).
func (s *ForSaleService) Update(
	ctx context.Context,
	tx db.Tx,
	for_sale *entity.ForSale,
) error {
	// HARD RULE: Active for_sales cannot be private
	if for_sale.Status == entity.ForSaleStatusActive && for_sale.Visibility == entity.ForSaleVisibilityPrivate {
		return fmt.Errorf("invalid for_sale: active status requires public visibility")
	}

	for_sale.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, tx, for_sale); err != nil {
		return err
	}

	// Emit for_sale.updated event
	if s.outboxRepo != nil {
		if err := s.outboxRepo.InsertEvent(
			ctx, tx,
			events.EventForSaleUpdated,
			for_sale.ID,
			buildForSaleEventPayload(for_sale),
		); err != nil {
			return fmt.Errorf("failed to insert for_sale.updated event: %w", err)
		}
	}

	return nil
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
		coverages, err := s.coverageRepo.GetByShippingOption(ctx, tx, opt.ID)
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

// EnsureFarmAddressValid validates that the for_sale has a valid farm/sender
// address configured before publish. Checks:
//   - FarmAddressID is set (not nil)
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
	if for_sale.FarmAddressID == nil {
		return fmt.Errorf("farm_address_id is required: %w", ErrFarmAddressNotConfigured)
	}

	address, err := s.addressRepo.GetByID(ctx, tx, *for_sale.FarmAddressID)
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

// ReduceStock reduces the quantity of a for_sale.
// This is used when an order is created.
// All stock mutations must use this method within a transaction.
func (s *ForSaleService) ReduceStock(
	ctx context.Context,
	tx db.Tx,
	for_saleID uuid.UUID,
	quantity int,
) error {
	// Lock the for_sale for update
	for_sale, err := s.repo.GetForUpdate(ctx, tx, for_saleID)
	if err != nil {
		return err
	}

	// Check if for_sale will transition to sold after this reduction
	previousQuantity := for_sale.QuantityAvailable
	willTransitionToSold := previousQuantity > 0 && (previousQuantity-quantity) <= 0

	// Reduce quantity (this auto-transitions to sold if quantity reaches 0)
	if err := for_sale.ReduceQuantity(quantity); err != nil {
		return err
	}

	// Persist the changes
	if err := s.repo.UpdateStock(ctx, tx, for_sale); err != nil {
		return err
	}

	// Emit for_sale.sold event when the sale transitions to sold status
	if willTransitionToSold && for_sale.Status == entity.ForSaleStatusSold {
		if s.outboxRepo != nil {
			if err := s.outboxRepo.InsertEvent(
				ctx, tx,
				events.EventForSaleSold,
				for_sale.ID,
				buildForSaleEventPayload(for_sale),
			); err != nil {
				return fmt.Errorf("failed to insert for_sale.sold event: %w", err)
			}
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
	ForSales []*entity.ForSale
	NextCursor      *string // RFC3339 timestamp for next page
	HasMore         bool    // True if there are more results
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
		ForSales: for_sales,
		NextCursor:      nextCursorStr,
		HasMore:         nextCursor != nil,
	}, nil
}
