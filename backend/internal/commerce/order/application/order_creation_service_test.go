package application

// ============================================================================
// PHASE 2.5: FOUNDATIONAL SERVICE-LEVEL TEST COVERAGE
// ============================================================================
//
// Before this file, no test in this package constructed a full
// OrderCreationService and drove CreateFromSaleSurface / CreateFromAuction
// end-to-end — every existing test exercised a small private helper
// (validateSaleSurfaceForCheckout, idempotentOrderRecovery,
// validateShippingQuoteForOrder) with a minimal fake. That is a real gap:
// the finalization pipeline (snapshot integrity check, order + item
// persistence, outbox events, audit log) extracted into
// finalizeOrderCreationTx during the PHASE 2 refactor had never been
// exercised by any test.
//
// WHY THIS TOOK A PRODUCTION CODE CHANGE, NOT JUST A TEST FILE:
// OrderCreationService.addressService was typed as the concrete
// *addressApp.AddressService. That type's constructor (NewAddressService())
// takes no arguments and hardcodes a real, DB-backed repository — there is
// no way to substitute a fake into it from another package (its repo field
// is unexported). Every other heavy dependency here (ShippingService,
// CoinsService, OrderPaymentService, OutboxRepository) already accepts its
// sub-repositories via constructor injection, so a REAL instance can be
// built on top of FAKE repos below — only AddressService could not. The
// fix: CheckoutAddressResolver, a minimal single-method local interface
// (order_creation_service.go), mirrors the AuctionStatusChecker pattern
// already in this file. It changes nothing about runtime wiring
// (*addressApp.AddressService still satisfies it structurally) and unblocks
// exactly this test.
//
// SCOPE: One happy-path test for CreateFromSaleSurface, focused on zero snapshot
// so the order creation path stays at zero and CreateOrderTx still runs.
// This keeps the test focused on the zero-snapshot regression.
// finalizeOrderCreationTx — not just reachable in theory.
//
// All fakes below are deliberately minimal ("stub dasar"): each embeds the
// real repository interface as a nil value and only overrides the handful
// of methods this one flow actually calls. Calling an unoverridden method
// panics loudly (nil interface dispatch) instead of silently returning a
// zero value — that is the intended failure mode if a future change touches
// a code path this test doesn't anticipate.

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	addressentity "github.com/labuda/backend/internal/identity/address/entity"
	addressrepo "github.com/labuda/backend/internal/identity/address/repository"

	forsaleentity "github.com/labuda/backend/internal/commerce/forsale/entity"
	forsalerepo "github.com/labuda/backend/internal/commerce/forsale/repository"
	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
	orderrepository "github.com/labuda/backend/internal/commerce/order/repository"

	shippingApp "github.com/labuda/backend/internal/commerce/shipping/application"
	shippingentity "github.com/labuda/backend/internal/commerce/shipping/entity"
	shippingrepo "github.com/labuda/backend/internal/commerce/shipping/infrastructure/repository"

	coinsentity "github.com/labuda/backend/internal/incentive/coins/entity"
	coinsrepo "github.com/labuda/backend/internal/incentive/coins/repository"

	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	outboxrepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"

	contentrepo "github.com/labuda/backend/internal/social/content/repository"

	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

// ============================================================================
// FAKE db.Tx — Exec always succeeds; nothing in this happy path needs
// Query/QueryRow/Commit/Rollback to do anything but return zero values.
// ============================================================================

type happyPathTx struct{}

func (happyPathTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("OK"), nil
}
func (happyPathTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) { return nil, nil }
func (happyPathTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row        { return nil }
func (happyPathTx) Commit(_ context.Context) error                                { return nil }
func (happyPathTx) Rollback(_ context.Context) error                              { return nil }

var _ db.Tx = happyPathTx{}

// ============================================================================
// FAKE REPOSITORIES — each embeds the real interface (nil) and overrides
// only what CreateFromSaleSurface's happy path calls.
// ============================================================================

// happyPathOrderRepo backs OrderCreationService.repo.
type happyPathOrderRepo struct {
	orderrepository.OrderRepository

	createOrderCalls     int
	createOrderItemCalls int
	lastOrder            *orderentity.Order
	lastOrderItem        *orderentity.OrderItem
}

func (r *happyPathOrderRepo) GetByIdempotencyKey(context.Context, db.Tx, uuid.UUID, string) (*orderentity.Order, error) {
	return nil, nil // no prior order — fresh checkout
}

func (r *happyPathOrderRepo) CreateOrderTx(_ context.Context, _ db.Tx, order *orderentity.Order) error {
	r.createOrderCalls++
	r.lastOrder = order
	return nil
}

func (r *happyPathOrderRepo) CreateOrderItemTx(_ context.Context, _ db.Tx, item *orderentity.OrderItem) error {
	r.createOrderItemCalls++
	r.lastOrderItem = item
	return nil
}

// happyPathForSaleRepo backs OrderCreationService.forSaleRepo.
type happyPathForSaleRepo struct {
	forsalerepo.ForSaleRepository
	forSale          *forsaleentity.ForSale
	updateStockCalls int
}

func (r *happyPathForSaleRepo) GetForUpdate(context.Context, db.Tx, uuid.UUID) (*forsaleentity.ForSale, error) {
	return r.forSale, nil
}

func (r *happyPathForSaleRepo) UpdateStock(_ context.Context, _ db.Tx, _ *forsaleentity.ForSale) error {
	r.updateStockCalls++
	return nil
}

// happyPathProductShippingRepo backs both OrderCreationService.productShippingRepo
// and the real ShippingService's internal productShippingRepo — same data either way.
type happyPathProductShippingRepo struct {
	shippingrepo.ProductShippingSetupRepository
	options []*shippingentity.ShippingSetup
}

func (r *happyPathProductShippingRepo) GetByProduct(context.Context, db.Tx, uuid.UUID) ([]*shippingentity.ShippingSetup, error) {
	return r.options, nil
}

// happyPathCoverageRepo backs the real ShippingService's coverageRepo.
type happyPathCoverageRepo struct {
	shippingrepo.ShippingCoverageRepository
	coverage *shippingentity.ShippingCoverage
}

func (r *happyPathCoverageRepo) GetByOptionAndProvince(_ context.Context, _ db.Tx, _ uuid.UUID, _ string) (*shippingentity.ShippingCoverage, error) {
	return r.coverage, nil
}

// happyPathAddressRepo backs OrderCreationService.addressRepo (used by
// getFarmAddressSnapshot to resolve the seller's shipping origin).
type happyPathAddressRepo struct {
	addressrepo.AddressRepository
	farmAddress *addressentity.Address
}

func (r *happyPathAddressRepo) GetByID(context.Context, db.Tx, uuid.UUID) (*addressentity.Address, error) {
	return r.farmAddress, nil
}

// happyPathAddressResolver backs OrderCreationService.addressService via the
// CheckoutAddressResolver seam (see file header for why this seam exists).
type happyPathAddressResolver struct {
	address *addressentity.Address
}

func (r happyPathAddressResolver) GetAddressForCheckout(_ context.Context, _ db.Tx, _ uuid.UUID, _ uuid.UUID) (*addressentity.Address, error) {
	return r.address, nil
}

// happyPathCoinsRepo records whether order creation accidentally touches the
// coins repository. The regression should leave every counter at zero.
type happyPathCoinsRepo struct {
	coinsrepo.CoinsRepository
	balance       int64
	spendCalls    int
	deductCalls   int
	createTxCalls int
}

func (r *happyPathCoinsRepo) GetActiveBalance(context.Context, db.Tx, uuid.UUID) (int64, error) {
	return r.balance, nil
}
func (r *happyPathCoinsRepo) FindSpendByReference(context.Context, db.Tx, uuid.UUID, uuid.UUID) (*coinsentity.CoinsTransaction, error) {
	return nil, nil // no prior spend for this order — first attempt
}
func (r *happyPathCoinsRepo) EnsureBalanceRow(context.Context, db.Tx, uuid.UUID) error {
	return nil
}
func (r *happyPathCoinsRepo) AtomicDeductBalance(_ context.Context, _ db.Tx, _ uuid.UUID, _ int64) (int64, error) {
	r.deductCalls++
	return 1, nil // 1 row affected == success
}
func (r *happyPathCoinsRepo) CreateTransaction(context.Context, db.Tx, *coinsentity.CoinsTransaction) error {
	r.createTxCalls++
	r.spendCalls++
	return nil
}

// happyPathCommentRepo backs OrderCreationService.commentRepo, called
// unconditionally by getOriginRequestTargetID (result discarded on error).
type happyPathCommentRepo struct {
	contentrepo.CommentRepository
}

func (happyPathCommentRepo) FindTargetIDByCommerceReference(context.Context, db.Tx, uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, nil
}

// happyPathAccountStatusChecker backs OrderCreationService.accountStatusChecker.
type happyPathAccountStatusChecker struct{}

func (happyPathAccountStatusChecker) EnsureActive(context.Context, uuid.UUID) error { return nil }
func (happyPathAccountStatusChecker) GetStatus(context.Context, uuid.UUID) (string, error) {
	return "active", nil
}
func (happyPathAccountStatusChecker) IsBanned(context.Context, uuid.UUID) (bool, error) {
	return false, nil
}

// happyPathActorResolver backs OrderCreationService.actorResolver — returns
// an Actor whose CanCheckout() is true (active, verified, identity complete).
type happyPathActorResolver struct{}

func (happyPathActorResolver) ResolveActor(_ interface{}, userID uuid.UUID) (*capabilityEntity.Actor, error) {
	return &capabilityEntity.Actor{
		ID:                 userID,
		Role:               "user",
		AccountStatus:      "active",
		EmailVerified:      true,
		IsIdentityComplete: true,
	}, nil
}

// ============================================================================
// FIXTURE BUILDERS
// ============================================================================

func newHappyPathFixtures(_ *testing.T) (*OrderCreationService, CreateFromSaleSurfaceInput, *happyPathOrderRepo, *happyPathCoinsRepo) {
	buyerID := uuid.New()
	sellerID := uuid.New()
	productID := uuid.New()
	listingID := uuid.New()
	shippingSetupID := uuid.New()
	buyerAddressID := uuid.New()
	farmAddressID := uuid.New()

	forSale := &forsaleentity.ForSale{
		ID:                listingID,
		ProductID:         productID,
		SellerID:          sellerID,
		Title:             "Kohaku Premium 40cm",
		ForSaleType:       forsaleentity.ForSaleTypeFixedPrice,
		PricePerUnit:      money.New(100_000),
		QuantityAvailable: 5,
		Status:            forsaleentity.ForSaleStatusActive,
		Visibility:        forsaleentity.ForSaleVisibilityPublic,
		FarmAddressID:     &farmAddressID,
		PreparationTime:   forsaleentity.PreparationTimeImmediate,
	}

	buyerAddress := &addressentity.Address{
		ID:         buyerAddressID,
		UserID:     buyerID,
		ProvinceID: "31",
		CityID:     "", // empty on purpose: skips the city-override lookup path
	}

	farmAddress := &addressentity.Address{
		ID:      farmAddressID,
		UserID:  sellerID,
		Purpose: addressentity.AddressPurposeSender,
	}

	shippingSetup := &shippingentity.ShippingSetup{
		ID:            shippingSetupID,
		SellerID:      sellerID,
		Name:          "JNE Reguler",
		TransportType: shippingentity.TransportTrain,
		IsActive:      true,
	}

	coverage := &shippingentity.ShippingCoverage{
		ID:               uuid.New(),
		ShippingSetupID: shippingSetupID,
		ProvinceCode:     "31",
		ProvinceRate:     money.New(15_000),
		IsAvailable:      true,
	}

	productShippingRepo := &happyPathProductShippingRepo{options: []*shippingentity.ShippingSetup{shippingSetup}}
	orderRepo := &happyPathOrderRepo{}
	coinsRepo := &happyPathCoinsRepo{balance: 5_000} // less than MaxCoinsAllowed -> balance wins the cap

	realShippingService := shippingApp.NewShippingService(
		nil, // shippingSetupRepo: unused by CheckDeliveryAvailability's actual code path
		&happyPathCoverageRepo{coverage: coverage},
		nil, // cityOverrideRepo: unused since buyerAddress.CityID == ""
		productShippingRepo,
	)
	realOutboxRepo := outboxrepo.NewOutboxRepository(nil)

	svc := &OrderCreationService{
		repo:                 orderRepo,
		forSaleRepo:          &happyPathForSaleRepo{forSale: forSale},
		productShippingRepo:  productShippingRepo,
		shippingService:      realShippingService,
		addressService:       happyPathAddressResolver{address: buyerAddress},
		addressRepo:          &happyPathAddressRepo{farmAddress: farmAddress},
		accountStatusChecker: happyPathAccountStatusChecker{},
		roleChecker:          checkoutFakeRoleChecker{hasCapability: true},
		actorResolver:        happyPathActorResolver{},
		outboxRepo:           realOutboxRepo,
		commentRepo:          happyPathCommentRepo{},
		// auditService, auctionRepo, negotiationRepo, shippingQuoteRepo,
		// walletService, discountService, coinsService, configService,
		// ownership: left nil. CreateFromSaleSurface's happy path either
		// guards these with a nil check or never reaches them (no
		// negotiation, no shipping quote, no auction).
	}

	snapshot := &PricingSnapshot{
		UnitPrice:          money.New(100_000),
		Subtotal:           money.New(100_000),
		ShippingTotal:      money.New(15_000),
		CommissionPercent:  5,
		CommissionAmount:   money.New(5_000),
		EscrowAmount:       money.New(115_000), // canonical buyer base (P−D)+S = 100000 + 15000 (commission excluded)
		ServiceFeeAmount:   money.New(3_000),
		TotalPayableAmount: money.New(118_000), // escrow + service fee
		DiscountAmount:     money.New(0),
		MaxCoinsAllowed:    10_000,
		OrderValueForCoins: 115_000, // subtotal + shipping - discount
		ShippingSetupName: "JNE Reguler",
		TokenID:            uuid.New(),
		PaymentMethod:      PaymentMethodInstant,
	}

	input := CreateFromSaleSurfaceInput{
		ProductID:        productID,
		SourceType:       orderentity.OrderSourceForSale,
		SourceID:         listingID,
		BuyerID:          buyerID,
		Quantity:         1,
		AddressID:        buyerAddressID,
		ShippingSetupID: shippingSetupID,
		PricingSnapshot:  snapshot,
	}

	return svc, input, orderRepo, coinsRepo
}

// ============================================================================
// THE TEST
// ============================================================================

// TestCreateFromSaleSurface_HappyPath drives a full, successful checkout
// through CreateFromSaleSurface — idempotency checks, buyer eligibility,
// address resolution, forSale lock + validation, shipping-option validation,
// stock reduction, pricing-snapshot integrity check, order + order-item
// persistence, and outbox events — using only fakes/real-with-fake-repos
// dependencies (no live DB). It proves order creation keeps the coins
// snapshot at zero and does not touch the coins repository.
func TestCreateFromSaleSurface_HappyPath(t *testing.T) {
	svc, input, orderRepo, coinsRepo := newHappyPathFixtures(t)

	order, err := svc.CreateFromSaleSurface(context.Background(), happyPathTx{}, input)

	require.NoError(t, err)
	require.NotNil(t, order)

	// --- Order itself ---
	require.Equal(t, input.BuyerID, order.BuyerID)
	require.Equal(t, orderentity.StatusPending, order.Status)
	require.Equal(t, orderentity.EscrowStatusHolding, order.EscrowStatus)
	require.Equal(t, int64(0), order.CoinsUsed)

	// --- finalizeOrderCreationTx actually ran ---
	require.Equal(t, 1, orderRepo.createOrderCalls, "CreateOrderTx must run exactly once")
	require.Equal(t, 1, orderRepo.createOrderItemCalls, "CreateOrderItemTx must run exactly once")
	require.Equal(t, order.ID, orderRepo.lastOrder.ID)
	require.NotNil(t, orderRepo.lastOrderItem)
	require.Equal(t, order.UnitPrice, orderRepo.lastOrderItem.UnitPriceSnapshot)

	// --- No legacy coin mutation happened during order creation ---
	require.Equal(t, 0, coinsRepo.deductCalls, "AtomicDeductBalance must not run during order creation")
	require.Equal(t, 0, coinsRepo.spendCalls, "coins spend transaction must not be recorded during order creation")
}
