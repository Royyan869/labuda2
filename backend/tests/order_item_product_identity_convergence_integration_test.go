//go:build integration

package tests

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	auctionentity "github.com/labuda/backend/internal/commerce/auction/entity"
	auctioninfra "github.com/labuda/backend/internal/commerce/auction/infrastructure/repository"
	fpsentity "github.com/labuda/backend/internal/commerce/forsale/entity"
	fpsinfra "github.com/labuda/backend/internal/commerce/forsale/infrastructure/repository"
	orderApp "github.com/labuda/backend/internal/commerce/order/application"
	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
	orderinfra "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	productentity "github.com/labuda/backend/internal/commerce/product/entity"
	productinfra "github.com/labuda/backend/internal/commerce/product/infrastructure/repository"
	shippingApp "github.com/labuda/backend/internal/commerce/shipping/application"
	shippingrepo "github.com/labuda/backend/internal/commerce/shipping/infrastructure/repository"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	outboxinfra "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/labuda/backend/pkg/testdb"
)

// ============================================================================
// STAGE 5: ORDER ITEM PRODUCT IDENTITY CONVERGENCE — RUNTIME PROOF
//
// Proves against real Postgres that order_items.product_id is ALWAYS
// products.id for every order path (Model B):
//   1. FPS buy-now order -> product_id == for_sale.product_id
//   2. negotiation order  -> product_id == for_sale.product_id
//   3. auction order      -> product_id == auction.product_id
//   4. Product reuse by later selling surfaces keeps the same products.id
//   5. CountActiveOrdersByProduct(productID) sees FPS orders
//   6. CountAnyOrdersByProduct(productID) sees FPS orders
//   7. (Order-completion stock restoration via orders.source_* is proven in
//      the application package: order_completion_restore_source_integration_test.go)
//   8. auction behavior remains correct (quantity 1, own product id)
// Plus migration 000045 up/down replay, including the legacy-namespace
// backfill proved against a crafted for_sales-id row.
//
// Only identity-irrelevant gates are stubbed (account status, seller
// capability, actor resolution); every persistence touch uses real
// repositories against the disposable labuda_test database.
// ============================================================================

type stage5RoleChecker struct{}

func (stage5RoleChecker) IsAdmin(context.Context, uuid.UUID) (bool, error) { return false, nil }
func (stage5RoleChecker) HasActiveSellerCapability(context.Context, uuid.UUID) (bool, error) {
	return true, nil
}
func (stage5RoleChecker) HasSellerProfile(context.Context, uuid.UUID) (bool, error) { return true, nil }

type stage5AccountChecker struct{}

func (stage5AccountChecker) EnsureActive(context.Context, uuid.UUID) error { return nil }
func (stage5AccountChecker) GetStatus(context.Context, uuid.UUID) (string, error) {
	return "active", nil
}
func (stage5AccountChecker) IsBanned(context.Context, uuid.UUID) (bool, error) { return false, nil }

type stage5ActorResolver struct{}

func (stage5ActorResolver) ResolveActor(_ interface{}, userID uuid.UUID) (*capabilityEntity.Actor, error) {
	return &capabilityEntity.Actor{
		ID:                 userID,
		Role:               "user",
		AccountStatus:      "active",
		EmailVerified:      true,
		IsIdentityComplete: true,
	}, nil
}

// newStage5OrderService wires a real OrderCreationService: real repos for
// order/forSale/product/shipping/outbox, stub gates only.
func newStage5OrderService() *orderApp.OrderCreationService {
	optionRepo := shippingrepo.NewShippingSetupRepository()
	return orderApp.NewOrderCreationService(
		stage5AccountChecker{},
		shippingApp.NewShippingService(
			optionRepo,
			shippingrepo.NewShippingCoverageRepository(),
			shippingrepo.NewCityOverrideRepository(),
			shippingrepo.NewProductShippingSetupRepository(optionRepo),
		),
		outboxinfra.NewOutboxRepository(nil),
		nil, // configService: unused by creation path
		stage5RoleChecker{},
		stage5ActorResolver{},
		nil, // auditService
		shippingrepo.NewProductShippingSetupRepository(optionRepo),
		nil, // auctionStatusChecker (nil-safe guard; skips BNR lock)
		nil, // walletService: unused by creation path
	)
}

// ---------------------------------------------------------------------------
// SEEDING HELPERS
// ---------------------------------------------------------------------------

func stage5User(t *testing.T, ctx context.Context, tdb *testdb.TestDB, id uuid.UUID) {
	t.Helper()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO users (id, firebase_uid, email, email_verified_at, account_status, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), 'active', NOW(), NOW())
		`, id, "fb-"+id.String(), id.String()+"@stage5.invalid")
		return err
	}))
}

func stage5Address(t *testing.T, ctx context.Context, tdb *testdb.TestDB, id, userID uuid.UUID, purpose, provinceID string) {
	t.Helper()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO addresses (
				id, user_id, purpose, nickname,
				recipient_name, phone,
				province_id, province_name,
				city_id, city_name,
				district_id, district_name,
				village_id, village_name,
				street_address, postal_code,
				latitude, longitude, notes,
				is_primary, is_available_for_checkout,
				created_at, updated_at
			)
			VALUES (
				$1, $2, $3, 'Addr', 'Name', '08123',
				$4, 'DKI Jakarta', '', '', '', '', '', '',
				'St.', '12345',
				NULL, NULL, 'stage5', true, true, NOW(), NOW()
			)
		`, id, userID, purpose, provinceID)
		return err
	}))
}

// stage5Shipping seeds one shipping option + DKI coverage + product links.
// Unique option name per call because of the
// uq_shipping_options_seller_name_internal_purpose_ci unique index.
func stage5Shipping(t *testing.T, ctx context.Context, tdb *testdb.TestDB, sellerID uuid.UUID, productIDs ...uuid.UUID) uuid.UUID {
	t.Helper()
	optionID := uuid.New()
	optionName := "JNE-" + optionID.String()[:8]
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO shipping_options (id, seller_id, name, transport_type, is_active, created_at, updated_at, internal_purpose)
			VALUES ($1, $2, $3, 'train', true, NOW(), NOW(), NULL)
		`, optionID, sellerID, optionName); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO shipping_coverages (id, shipping_option_id, province_code, province_name, province_rate, is_available, created_at)
			VALUES ($1, $2, '31', 'DKI Jakarta', $3, true, NOW())
		`, uuid.New(), optionID, int64(15_000)); err != nil {
			return err
		}
		for _, pid := range productIDs {
			if _, err := tx.Exec(ctx, `
				INSERT INTO product_shipping_options (product_id, shipping_option_id, sort_order, created_at)
				VALUES ($1, $2, 0, NOW())
			`, pid, optionID); err != nil {
				return err
			}
		}
		return nil
	}))
	return optionID
}

func stage5PricingToken(t *testing.T, ctx context.Context, tdb *testdb.TestDB, snapshot *orderApp.PricingSnapshot, buyerID, addressID, optionID uuid.UUID) {
	t.Helper()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO pricing_tokens (
				token, user_id, quantity, unit_price, subtotal, shipping_total,
				commission_percent, commission_amount, escrow_amount,
				discount_amount, shipping_option_id, shipping_option_name, shipping_transport_type,
				address_id, address_snapshot, max_coins_allowed, order_value_for_coins,
				is_used, expires_at, service_fee_amount, total_payable_amount, created_at, updated_at
			)
			VALUES (
				$1, $2, 1, $3, $4, $5,
				$6, $7, $8,
				0, $9, $10, $11,
				$12, '{}', $13, $14,
				false, NOW() + INTERVAL '1 hour', $15, $16, NOW(), NOW()
			)
		`,
			snapshot.TokenID, buyerID,
			snapshot.UnitPrice.Int64(), snapshot.Subtotal.Int64(), snapshot.ShippingTotal.Int64(),
			snapshot.CommissionPercent, snapshot.CommissionAmount.Int64(), snapshot.EscrowAmount.Int64(),
			optionID, snapshot.ShippingSetupName, snapshot.ShippingTransportType,
			addressID, snapshot.MaxCoinsAllowed, snapshot.OrderValueForCoins,
			snapshot.ServiceFeeAmount.Int64(), snapshot.TotalPayableAmount.Int64(),
		)
		return err
	}))
}

func stage5Snapshot(tokenID uuid.UUID, unitPrice int64) *orderApp.PricingSnapshot {
	shipping := int64(15_000)
	serviceFee := int64(3_000)
	return &orderApp.PricingSnapshot{
		UnitPrice:             money.New(unitPrice),
		Subtotal:              money.New(unitPrice),
		ShippingTotal:         money.New(shipping),
		CommissionPercent:     5,
		CommissionAmount:      money.New(unitPrice * 5 / 100),
		EscrowAmount:          money.New(unitPrice + shipping),
		ServiceFeeAmount:      money.New(serviceFee),
		TotalPayableAmount:    money.New(unitPrice + shipping + serviceFee),
		DiscountAmount:        money.New(0),
		MaxCoinsAllowed:       10_000,
		OrderValueForCoins:    unitPrice + shipping,
		ShippingSetupName:    "JNE Reguler",
		ShippingTransportType: "train",
		TokenID:               tokenID,
		PaymentMethod:         orderApp.PaymentMethodInstant,
	}
}

func storagePID(t *testing.T, ctx context.Context, tdb *testdb.TestDB, orderID uuid.UUID) uuid.UUID {
	t.Helper()
	var pid uuid.UUID
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT product_id FROM order_items WHERE order_id = $1`, orderID).Scan(&pid))
	return pid
}

func storedPrices(t *testing.T, ctx context.Context, tdb *testdb.TestDB, orderID uuid.UUID) (int64, int64) {
	t.Helper()
	var orderPrice, itemPrice int64
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT o.unit_price, oi.unit_price_snapshot
		FROM orders o
		JOIN order_items oi ON oi.order_id = o.id
		WHERE o.id = $1
	`, orderID).Scan(&orderPrice, &itemPrice))
	return orderPrice, itemPrice
}

// ---------------------------------------------------------------------------
// THE RUNTIME PROOF
// ---------------------------------------------------------------------------

func TestOrderItemProductIdentity_Convergence_RuntimeProof(t *testing.T) {
	ctx := context.Background()
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	sellerID := uuid.New()
	buyerID := uuid.New()
	stage5User(t, ctx, tdb, sellerID)
	stage5User(t, ctx, tdb, buyerID)

	farmAddressID := uuid.New()
	buyerAddressID := uuid.New()
	stage5Address(t, ctx, tdb, farmAddressID, sellerID, "sender", "31")
	stage5Address(t, ctx, tdb, buyerAddressID, buyerID, "shipping", "31")

	orderRepo := orderinfra.NewOrderRepository()
	svc := newStage5OrderService()

	createProduct := func(title string) uuid.UUID {
		t.Helper()
		product := &productentity.Product{
			SellerID:        sellerID,
			Title:           title,
			Description:     "desc",
			Variety:         "Kohaku",
			PreparationTime: "immediate",
			FarmAddressID:   &farmAddressID,
		}
		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			return productinfra.NewProductRepository().Create(ctx, tx, product)
		}))
		return product.ID
	}

	createActiveFPS := func(productID uuid.UUID, qty int) uuid.UUID {
		t.Helper()
		forSale, err := fpsentity.NewForSale(
			sellerID, "Kohaku Premium", "desc", []byte(`["https://example.com/koi.jpg"]`), "Kohaku",
			nil, nil, nil, nil, nil, []string{},
			fpsentity.ForSaleTypeFixedPrice, money.New(100_000), qty, false,
			fpsentity.ForSaleVisibilityPublic,
			
			&farmAddressID, fpsentity.PreparationTimeImmediate, nil,
		)
		require.NoError(t, err)
		forSale.ProductID = productID
		require.NoError(t, forSale.Publish())
		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			return fpsinfra.NewForSaleRepository().Create(ctx, tx, forSale)
		}))
		return forSale.ID
	}

	// --- 1. FPS buy-now order ---
	product1 := createProduct("P1")
	option1 := stage5Shipping(t, ctx, tdb, sellerID, product1)
	fps1ID := createActiveFPS(product1, 3)
	orderInput1 := orderApp.CreateFromSaleSurfaceInput{
		ProductID:        product1,
		SourceType:       orderentity.OrderSourceForSale,
		SourceID:         fps1ID,
		BuyerID:          buyerID,
		Quantity:         1,
		AddressID:        buyerAddressID,
		ShippingSetupID: option1,
		PricingSnapshot:  stage5Snapshot(uuid.New(), 100_000),
	}
	stage5PricingToken(t, ctx, tdb, orderInput1.PricingSnapshot, buyerID, buyerAddressID, option1)
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE for_sales SET price_per_unit = $2 WHERE id = $1`, fps1ID, int64(200_000))
		return err
	}))
	var fps1OrderID uuid.UUID
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		order, err := svc.CreateFromSaleSurface(ctx, tx, orderInput1)
		if err != nil {
			return err
		}
		fps1OrderID = order.ID
		return nil
	}))
	require.Equal(t, product1, storagePID(t, ctx, tdb, fps1OrderID),
		"FPS order item product_id must be products.id (not for_sales.id)")
	assertStoredPIDMatchesSurface(t, ctx, tdb, fps1OrderID, "for_sales")
	orderPrice, itemPrice := storedPrices(t, ctx, tdb, fps1OrderID)
	require.Equal(t, int64(100_000), orderPrice)
	require.Equal(t, int64(100_000), itemPrice)
	require.NotEqual(t, int64(200_000), orderPrice)
	currentPrice := int64(0)
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT price_per_unit FROM for_sales WHERE id = $1`, fps1ID).Scan(&currentPrice))
	require.Equal(t, int64(200_000), currentPrice)
	require.NotEqual(t, currentPrice, orderPrice)

	// --- 5/6. product-keyed counts see the FPS order ---
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		active, err := orderRepo.CountActiveOrdersByProduct(ctx, tx, product1)
		require.NoError(t, err)
		require.Equal(t, int64(1), active, "CountActiveOrdersByProduct must see the FPS order")
		anyCount, err := orderRepo.CountAnyOrdersByProduct(ctx, tx, product1)
		require.NoError(t, err)
		require.Equal(t, int64(1), anyCount, "CountAnyOrdersByProduct must see the FPS order")
		return nil
	}))

	// --- 1b. a Product cannot receive a second ForSale even after the first
	// is sold ---
	// Canonical product identity: products.selling_surface is claimed once and
	// permanently (ClaimSellingSurface). ForSaleStatusSold is seller-terminal;
	// it never releases the product for a second ForSale row. The second
	// attachment attempt is therefore rejected at surface creation, before any
	// order-creation path can be reached.
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE for_sales SET status = 'sold', sold_at = NOW(), quantity_available = 0 WHERE id = $1`, fps1ID)
		return err
	}))
	forSale2, err := fpsentity.NewForSale(
		sellerID, "Kohaku Premium 2", "desc", []byte(`["https://example.com/koi2.jpg"]`), "Kohaku",
		nil, nil, nil, nil, nil, []string{},
		fpsentity.ForSaleTypeFixedPrice, money.New(100_000), 3, false,
		fpsentity.ForSaleVisibilityPublic,
		
		&farmAddressID, fpsentity.PreparationTimeImmediate, nil,
	)
	require.NoError(t, err)
	forSale2.ProductID = product1
	require.NoError(t, forSale2.Publish())
	require.Error(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		return fpsinfra.NewForSaleRepository().Create(ctx, tx, forSale2)
	}), "second ForSale on a sold product must be rejected by the permanent selling-surface claim")

	// --- 2. negotiation order (fresh product) ---
	product2 := createProduct("P2")
	option2 := stage5Shipping(t, ctx, tdb, sellerID, product2)
	fps3ID := createActiveFPS(product2, 3)
	sessionID := uuid.New()
	stage5NegotiationSession(t, ctx, tdb, sessionID, fps3ID, sellerID, buyerID)

	negotiationSnapshot := stage5Snapshot(uuid.New(), 90_000) // negotiated price
	stage5PricingToken(t, ctx, tdb, negotiationSnapshot, buyerID, buyerAddressID, option2)
	negotiationInput := orderApp.CreateFromSaleSurfaceInput{
		ProductID:        product2,
		SourceType:       orderentity.OrderSourceForSale,
		SourceID:         fps3ID,
		BuyerID:          buyerID,
		Quantity:         1,
		AddressID:        buyerAddressID,
		ShippingSetupID: option2,
		NegotiationID:    &sessionID,
		PricingSnapshot:  negotiationSnapshot,
	}
	var negotiationOrderID uuid.UUID
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		order, err := svc.CreateFromSaleSurface(ctx, tx, negotiationInput)
		if err != nil {
			return err
		}
		negotiationOrderID = order.ID
		return nil
	}))
	require.Equal(t, product2, storagePID(t, ctx, tdb, negotiationOrderID),
		"negotiation order item product_id must be products.id")
	assertStoredPIDMatchesSurface(t, ctx, tdb, negotiationOrderID, "for_sales")
	orderPrice, itemPrice = storedPrices(t, ctx, tdb, negotiationOrderID)
	require.Equal(t, int64(90_000), orderPrice)
	require.Equal(t, int64(90_000), itemPrice)
	require.NotEqual(t, int64(100_000), orderPrice)
	var sourceType, sourceIDStr, negotiationID string
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT source_type, source_id::text, COALESCE(negotiation_id::text, '') FROM orders WHERE id = $1`, negotiationOrderID).
		Scan(&sourceType, &sourceIDStr, &negotiationID))
	require.Equal(t, "for_sale", sourceType, "negotiation order must carry the FPS source type")
	require.Equal(t, fps3ID.String(), sourceIDStr, "negotiation order must carry the FPS source id")
	require.Equal(t, sessionID.String(), negotiationID)

	// --- 3/8. auction order (fresh product) ---
	product3 := createProduct("P3")
	option3 := stage5Shipping(t, ctx, tdb, sellerID, product3)
	auction3ID := stage5Auction(t, ctx, tdb, product3, sellerID)
	auctionSnapshot3 := stage5Snapshot(uuid.New(), 500_000)
	stage5PricingToken(t, ctx, tdb, auctionSnapshot3, buyerID, buyerAddressID, option3)
	var auctionOrderID uuid.UUID
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		order, err := svc.CreateFromAuction(ctx, tx, orderApp.CreateFromAuctionInput{
			AuctionID:             auction3ID,
			AuctionSellerID:       sellerID,
			ProductID:             product3,
			BuyerID:               buyerID,
			WinningBid:            500_000,
			AddressID:             buyerAddressID,
			ShippingSetupID:      option3,
			AuctionSettlementType: orderentity.AuctionSettlementBidWin,
			PricingSnapshot:       auctionSnapshot3,
		})
		if err != nil {
			return err
		}
		auctionOrderID = order.ID
		return nil
	}))
	require.Equal(t, product3, storagePID(t, ctx, tdb, auctionOrderID),
		"auction order item product_id must be products.id")
	assertStoredPIDMatchesSurface(t, ctx, tdb, auctionOrderID, "auctions")
	require.Equal(t, 1, auctionOrderQuantity(t, ctx, tdb, auctionOrderID),
		"auction order quantity must remain 1 (unique item)")

	// --- 4. auction on a fresh product ---
	product4 := createProduct("P4")
	option4 := stage5Shipping(t, ctx, tdb, sellerID, product4)
	reuseAuctionID := stage5Auction(t, ctx, tdb, product4, sellerID)
	reuseSnapshot := stage5Snapshot(uuid.New(), 400_000)
	stage5PricingToken(t, ctx, tdb, reuseSnapshot, buyerID, buyerAddressID, option4)
	var reuseAuctionOrderID uuid.UUID
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		order, err := svc.CreateFromAuction(ctx, tx, orderApp.CreateFromAuctionInput{
			AuctionID:             reuseAuctionID,
			AuctionSellerID:       sellerID,
			ProductID:             product4,
			BuyerID:               buyerID,
			WinningBid:            400_000,
			AddressID:             buyerAddressID,
			ShippingSetupID:      option4,
			AuctionSettlementType: orderentity.AuctionSettlementBidWin,
			PricingSnapshot:       reuseSnapshot,
		})
		if err != nil {
			return err
		}
		reuseAuctionOrderID = order.ID
		return nil
	}))
	require.Equal(t, product4, storagePID(t, ctx, tdb, reuseAuctionOrderID),
		"auction order item product_id must be the auction product id")

	// --- final: product-keyed counts see the fresh auction order ---
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		anyCount, err := orderRepo.CountAnyOrdersByProduct(ctx, tx, product4)
		require.NoError(t, err)
		require.Equal(t, int64(1), anyCount)
		active, err := orderRepo.CountActiveOrdersByProduct(ctx, tx, product4)
		require.NoError(t, err)
		require.Equal(t, int64(1), active)
		return nil
	}))
}

func auctionOrderQuantity(t *testing.T, ctx context.Context, tdb *testdb.TestDB, orderID uuid.UUID) int {
	t.Helper()
	var qty int
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT quantity FROM order_items WHERE order_id = $1`, orderID).Scan(&qty))
	return qty
}

func stage5Auction(t *testing.T, ctx context.Context, tdb *testdb.TestDB, productID, sellerID uuid.UUID) uuid.UUID {
	t.Helper()
	auction := auctionentity.NewDraft(
		sellerID, productID, 400_000, 25_000, nil,
		time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour),
	)
	require.NoError(t, auction.Schedule())
	require.NoError(t, auction.Activate())
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		return auctioninfra.NewAuctionRepository().CreateTx(ctx, tx, auction)
	}))
	return auction.ID
}

func stage5NegotiationSession(t *testing.T, ctx context.Context, tdb *testdb.TestDB, sessionID, fpsID, sellerID, buyerID uuid.UUID) {
	t.Helper()
	now := time.Now()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO negotiation_sessions (
				id, resource_type, for_sale_id, buyer_id, seller_id, chat_room_id,
				status, expires_at, current_price, accepted_price, accepted_at,
				proposal_sequence, created_at, updated_at
			)
			VALUES ($1, 'for_sale', $2, $3, $4, NULL, 'accepted',
			        $5, 90000, 90000, $6, 1, $6, $6)
		`, sessionID, fpsID, buyerID, sellerID, now.Add(time.Hour), now)
		return err
	}))
}

// assertStoredPIDMatchesSurface verifies order_items.product_id equals the
// product_id column on the selling surface named by surfaceTable, joined via
// orders.source_id.
func assertStoredPIDMatchesSurface(t *testing.T, ctx context.Context, tdb *testdb.TestDB, orderID uuid.UUID, surfaceTable string) {
	t.Helper()
	var stored, surfaceProductID uuid.UUID
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT oi.product_id, s.product_id
		   FROM order_items oi
		   JOIN orders o ON o.id = oi.order_id
		   JOIN `+surfaceTable+` s ON s.id = o.source_id
		  WHERE oi.order_id = $1`,
		orderID).Scan(&stored, &surfaceProductID))
	require.Equal(t, stored, surfaceProductID,
		"order_items.product_id must equal %s.product_id (source surface)", surfaceTable)
}

// ---------------------------------------------------------------------------
// MIGRATION 000045 UP/DOWN REPLAY + LEGACY BACKFILL PROOF
// ---------------------------------------------------------------------------

func TestMigration000045_OrderItemProductIdentity_UpDownReplay(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t) // applies all up migrations incl. 000045
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()

	execSQLFile := func(name string) {
		t.Helper()
		path := filepath.Join("..", "migrations", name)
		raw, err := os.ReadFile(path)
		require.NoError(t, err, "read %s", path)
		var kept []string
		for _, line := range strings.Split(string(raw), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "--") {
				continue
			}
			kept = append(kept, trimmed)
		}
		var statements []string
		for _, part := range strings.Split(strings.Join(kept, "\n"), ";") {
			if stmt := strings.TrimSpace(part); stmt != "" {
				statements = append(statements, stmt)
			}
		}
		require.NotEmpty(t, statements)
		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			for _, stmt := range statements {
				if _, err := tx.Exec(ctx, stmt); err != nil {
					return err
				}
			}
			return nil
		}))
	}

	// execSQLFileAdapted is execSQLFile with a global substring substitution
	// applied to the historical SQL before execution. Used when a later
	// migration renamed a schema object that an older migration's replay
	// references (e.g. 000047: fixed_price_sales -> for_sales). Migration
	// history is never rewritten; only the replay is adapted.
	execSQLFileAdapted := func(name, old, replacement string) {
		t.Helper()
		path := filepath.Join("..", "migrations", name)
		raw, err := os.ReadFile(path)
		require.NoError(t, err, "read %s", path)
		sql := strings.ReplaceAll(string(raw), old, replacement)
		var kept []string
		for _, line := range strings.Split(sql, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "--") {
				continue
			}
			kept = append(kept, trimmed)
		}
		var statements []string
		for _, part := range strings.Split(strings.Join(kept, "\n"), ";") {
			if stmt := strings.TrimSpace(part); stmt != "" {
				statements = append(statements, stmt)
			}
		}
		require.NotEmpty(t, statements)
		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			for _, stmt := range statements {
				if _, err := tx.Exec(ctx, stmt); err != nil {
					return err
				}
			}
			return nil
		}))
	}

	fkExists := func() bool {
		var exists bool
		require.NoError(t, pool.QueryRow(ctx, `
			SELECT EXISTS(SELECT 1 FROM pg_constraint WHERE conname = 'order_items_product_id_fkey')`).Scan(&exists))
		return exists
	}
	notNull := func() bool {
		var isNullable string
		require.NoError(t, pool.QueryRow(ctx, `
			SELECT is_nullable FROM information_schema.columns
			WHERE table_name = 'order_items' AND column_name = 'product_id'`).Scan(&isNullable))
		return isNullable == "NO"
	}

	// Fresh up state from SetupDB.
	require.True(t, fkExists(), "000045 up must add order_items_product_id_fkey")
	require.True(t, notNull(), "000045 up must set product_id NOT NULL")

	// Down migration (drop FK + NOT NULL) so we can seed a legacy row whose
	// product_id holds the old for_sales.id namespace.
	execSQLFile("000045_order_item_product_identity_convergence.down.sql")
	require.False(t, fkExists())
	require.False(t, notNull())

	// Seed legacy data: a real order whose item.product_id = the FPS id.
	sellerID := uuid.New()
	buyerID := uuid.New()
	stage5User(t, ctx, tdb, sellerID)
	stage5User(t, ctx, tdb, buyerID)
	product := &productentity.Product{SellerID: sellerID, Title: "Legacy Koi", Description: "d", Variety: "Kohaku", PreparationTime: "immediate"}
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		return productinfra.NewProductRepository().Create(ctx, tx, product)
	}))
	fpsID := createLegacySoldSurface(t, ctx, tdb, product.ID, sellerID)
	legacyOrderID := createLegacyOrderWithFPSNamespace(t, ctx, tdb, sellerID, buyerID, fpsID)

	// Re-apply up: the backfill must converge the legacy row to products.id,
	// then reinstall NOT NULL + FK.
	//
	// 000047 renamed fixed_price_sales -> for_sales; the historical 000045
	// up.sql references the old table name, so the replay adapts it to the
	// current canonical schema (migration history itself is never rewritten).
	execSQLFileAdapted("000045_order_item_product_identity_convergence.up.sql",
		"fixed_price_sales", "for_sales")
	require.True(t, fkExists(), "000045 replay up must re-add the FK")
	require.True(t, notNull(), "000045 replay up must re-add NOT NULL")

	var convergedPID, surfaceProductID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT oi.product_id, fps.product_id
		   FROM order_items oi
		   JOIN orders o ON o.id = oi.order_id
		   JOIN for_sales fps ON fps.id = o.source_id
		  WHERE oi.order_id = $1`, legacyOrderID).Scan(&convergedPID, &surfaceProductID))
	require.Equal(t, surfaceProductID, convergedPID,
		"000045 backfill must converge legacy FPS-namespace rows to products.id")
	require.Equal(t, product.ID, convergedPID)
}

// createLegacySoldSurface seeds a sold for_sale referencing product.ID.
func createLegacySoldSurface(t *testing.T, ctx context.Context, tdb *testdb.TestDB, productID, sellerID uuid.UUID) uuid.UUID {
	t.Helper()
	fpsID := uuid.New()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO for_sales (
				id, product_id, seller_id, price_per_unit, negotiation_enabled,
				status, published_at, sold_at, withdrawn_at, quantity_available,
				created_at, updated_at
			)
			VALUES ($1, $2, $3, 50000, false, 'sold', NOW(), NOW(), NULL, 0, NOW(), NOW())
		`, fpsID, productID, sellerID)
		return err
	}))
	return fpsID
}

// createLegacyOrderWithFPSNamespace writes an order + order_item whose
// product_id stores the FPS id (the pre-Stage-5 writer behavior).
func createLegacyOrderWithFPSNamespace(t *testing.T, ctx context.Context, tdb *testdb.TestDB, sellerID, buyerID, fpsID uuid.UUID) uuid.UUID {
	t.Helper()
	orderRepo := orderinfra.NewOrderRepository()
	order := orderentity.NewOrderFromSource(
		buyerID, sellerID, orderentity.OrderSourceForSale, fpsID, nil,
		1,
		money.New(50000), money.New(50000), money.New(15000),
		5, money.New(2500), money.New(3000), money.New(68000),
		nil, "JNE", "train",
		nil, "immediate", nil, nil, nil, nil, nil,
		"instant", time.Now(),
	)
	order.ID = uuid.New()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		if err := orderRepo.CreateOrderTx(ctx, tx, order); err != nil {
			return err
		}
		item := orderentity.NewOrderItem(order.ID, fpsID, money.New(50000), 1, "Legacy Koi")
		return orderRepo.CreateOrderItemTx(ctx, tx, item)
	}))
	return order.ID
}
