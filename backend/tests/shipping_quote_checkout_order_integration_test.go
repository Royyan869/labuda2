//go:build integration

package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	fpsentity "github.com/labuda/backend/internal/commerce/forsale/entity"
	fpsinfra "github.com/labuda/backend/internal/commerce/forsale/infrastructure/repository"
	orderApp "github.com/labuda/backend/internal/commerce/order/application"
	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
	productentity "github.com/labuda/backend/internal/commerce/product/entity"
	productinfra "github.com/labuda/backend/internal/commerce/product/infrastructure/repository"
	shippingApp "github.com/labuda/backend/internal/commerce/shipping/application"
	shippingquoteEntity "github.com/labuda/backend/internal/commerce/shipping/quote/entity"
	shippingquoteRepo "github.com/labuda/backend/internal/commerce/shipping/quote/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/labuda/backend/pkg/testdb"
)

// ============================================================================
// F-02 SHIPPING QUOTE — ACCEPT/USE → CHECKOUT → ORDER → USED (RUNTIME PROOF)
//
// Proves, against real Postgres through the PRODUCTION order-creation path
// (OrderCreationService.CreateFromSaleSurface), the actual business completion
// of a manual shipping quote:
//
//   - an ACTIVE quote is consumed by checkout: order created, quote → USED;
//   - the created order snapshots the quote identity + source;
//   - a USED quote cannot produce a second order (idempotent reuse);
//   - an EXPIRED quote is rejected;
//   - a quote owned by a DIFFERENT buyer is rejected.
//
// The shipping quote is validated and marked USED by the ONE Commerce authority
// (ShippingQuoteService.ConsumeQuoteForCheckout) that order creation delegates
// to, inside the order transaction. Only identity-irrelevant gates are stubbed (account
// status, seller capability, actor resolution); every persistence touch uses
// real repositories against the disposable labuda_test database.
// ============================================================================

func sqStrPtr(v string) *string { return &v }

func sqAddress(t *testing.T, ctx context.Context, tdb *testdb.TestDB, id, userID uuid.UUID, provinceID, cityID string) {
	t.Helper()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO addresses (
				id, user_id, nickname,
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
				$1, $2, 'Addr', 'Name', '08123',
				$3, 'DKI Jakarta',
				$4, 'Jakarta', '', '', '', '',
				'St.', '12345',
				NULL, NULL, 'sq', true, true, NOW(), NOW()
			)
		`, id, userID, provinceID, cityID)
		return err
	}))
}

func sqSeedRoom(t *testing.T, ctx context.Context, tdb *testdb.TestDB, userA, userB uuid.UUID) uuid.UUID {
	t.Helper()
	if userA.String() > userB.String() {
		userA, userB = userB, userA
	}
	roomID := uuid.New()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO chat_rooms (id, room_type, participant_a, participant_b, created_at, updated_at, last_message_at)
			VALUES ($1, 'direct', $2, $3, NOW(), NOW(), NOW())
		`, roomID, userA, userB)
		return err
	}))
	return roomID
}

func sqActiveForSale(t *testing.T, ctx context.Context, tdb *testdb.TestDB, sellerID uuid.UUID, title string, qty int) (productID, saleID uuid.UUID) {
	t.Helper()
	product := &productentity.Product{
		SellerID:        sellerID,
		Title:           title,
		Description:     "desc",
		Variety:         "Kohaku",
		PreparationTime: "1_3_days",
	}
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		return productinfra.NewProductRepository().Create(ctx, tx, product)
	}))

	forSale, err := fpsentity.NewForSaleSurface(sellerID, money.New(100_000), qty, false)
	require.NoError(t, err)
	forSale.ProductID = product.ID
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		if err := fpsinfra.NewForSaleRepository().Create(ctx, tx, forSale); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			UPDATE for_sales
			SET status = 'active', published_at = NOW(), quantity_available = $2, updated_at = NOW()
			WHERE id = $1
		`, forSale.ID, qty)
		return err
	}))
	return product.ID, forSale.ID
}

func sqCreateQuote(t *testing.T, ctx context.Context, tdb *testdb.TestDB, q *shippingquoteEntity.ShippingQuote) {
	t.Helper()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		return shippingquoteRepo.NewShippingQuoteRepository().Create(ctx, tx, q)
	}))
}

func sqQuoteStatus(t *testing.T, ctx context.Context, tdb *testdb.TestDB, quoteID uuid.UUID) (string, *time.Time) {
	t.Helper()
	var status string
	var usedAt *time.Time
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT status, used_at FROM shipping_quotes WHERE id = $1`, quoteID).Scan(&status, &usedAt))
	return status, usedAt
}

func TestShippingQuote_UseCheckout_OrderCreated_QuoteUsed(t *testing.T) {
	ctx := context.Background()
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	sellerID := uuid.New()
	buyerID := uuid.New()
	otherBuyerID := uuid.New()
	stage5User(t, ctx, tdb, sellerID)
	stage5User(t, ctx, tdb, buyerID)
	stage5User(t, ctx, tdb, otherBuyerID)

	// Seller origin (primary) + buyer destination city lock.
	stage5Address(t, ctx, tdb, uuid.New(), sellerID, "31")
	buyerAddressID := uuid.New()
	sqAddress(t, ctx, tdb, buyerAddressID, buyerID, "31", "3171")

	productID, saleID := sqActiveForSale(t, ctx, tdb, sellerID, "SQ Checkout Koi", 2)
	// The one-current-active-quote-per-context unique index includes source_id,
	// so the EXPIRED case needs its own sale context to be seeded alongside the
	// ACTIVE one.
	expiredProductID, expiredSaleID := sqActiveForSale(t, ctx, tdb, sellerID, "SQ Checkout Expired Koi", 2)
	roomID := sqSeedRoom(t, ctx, tdb, sellerID, buyerID)

	// A shipping option is needed only to satisfy pricing_tokens.shipping_option_id
	// (the quote path does not use it for coverage).
	optionID := stage5Shipping(t, ctx, tdb, sellerID, productID)

	destCity := "3171"
	destProvince := "31"
	note := "ongkir manual"

	activeQuote := shippingquoteEntity.NewShippingQuote(
		roomID, productID, "for_sale", saleID, sellerID, buyerID,
		money.New(25_000), &note, &destCity, &destProvince, time.Now().Add(24*time.Hour),
	)
	expiredQuote := shippingquoteEntity.NewShippingQuote(
		roomID, expiredProductID, "for_sale", expiredSaleID, sellerID, buyerID,
		money.New(25_000), &note, &destCity, &destProvince, time.Now().Add(-1*time.Hour),
	)
	wrongBuyerQuote := shippingquoteEntity.NewShippingQuote(
		roomID, productID, "for_sale", saleID, sellerID, otherBuyerID,
		money.New(25_000), &note, &destCity, &destProvince, time.Now().Add(24*time.Hour),
	)
	sqCreateQuote(t, ctx, tdb, activeQuote)
	sqCreateQuote(t, ctx, tdb, expiredQuote)
	sqCreateQuote(t, ctx, tdb, wrongBuyerQuote)

	shippingSource := "shipping_quote"
	sqSnapshot := func(quoteID uuid.UUID) *orderApp.PricingSnapshot {
		return &orderApp.PricingSnapshot{
			UnitPrice:             money.New(100_000),
			Subtotal:              money.New(100_000),
			ShippingTotal:         money.New(25_000),
			CommissionPercent:     5,
			CommissionAmount:      money.New(5_000),
			EscrowAmount:          money.New(125_000), // (P-D)+S
			ServiceFeeAmount:      money.New(3_000),
			TotalPayableAmount:    money.New(128_000), // escrow + fee
			DiscountAmount:        money.New(0),
			OrderValueForCoins:    125_000,
			ShippingSetupName:     "Ongkir Manual",
			ShippingTransportType: "manual",
			ShippingSource:        &shippingSource,
			ShippingQuoteID:       &quoteID,
			ChatID:                &roomID,
			TokenID:               uuid.New(),
			PaymentMethod:         orderApp.PaymentMethodInstant,
		}
	}
	input := func(product, sale uuid.UUID, snapshot *orderApp.PricingSnapshot) orderApp.CreateFromSaleSurfaceInput {
		return orderApp.CreateFromSaleSurfaceInput{
			ProductID:       product,
			SourceType:      orderentity.OrderSourceForSale,
			SourceID:        sale,
			BuyerID:         buyerID,
			Quantity:        1,
			AddressID:       buyerAddressID,
			PricingSnapshot: snapshot,
		}
	}

	svc := newStage5OrderService()

	// ---- EXPIRED quote is rejected --------------------------------------
	expiredSnapshot := sqSnapshot(expiredQuote.ID)
	stage5PricingToken(t, ctx, tdb, expiredSnapshot, buyerID, buyerAddressID, optionID)
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := svc.CreateFromSaleSurface(ctx, tx, input(expiredProductID, expiredSaleID, expiredSnapshot))
		return err
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "expired")
	status, usedAt := sqQuoteStatus(t, ctx, tdb, expiredQuote.ID)
	require.Equal(t, string(shippingquoteEntity.QuoteStatusActive), status)
	require.Nil(t, usedAt)

	// ---- WRONG-BUYER quote is rejected ----------------------------------
	wrongBuyerSnapshot := sqSnapshot(wrongBuyerQuote.ID)
	stage5PricingToken(t, ctx, tdb, wrongBuyerSnapshot, buyerID, buyerAddressID, optionID)
	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := svc.CreateFromSaleSurface(ctx, tx, input(productID, saleID, wrongBuyerSnapshot))
		return err
	})
	require.Error(t, err)
	var buyerRejection *shippingquoteEntity.CheckoutRejectionError
	require.ErrorAs(t, err, &buyerRejection)
	require.Equal(t, "buyer_mismatch", buyerRejection.Field)

	// ---- ACTIVE quote is consumed → order created, quote USED -----------
	activeSnapshot := sqSnapshot(activeQuote.ID)
	stage5PricingToken(t, ctx, tdb, activeSnapshot, buyerID, buyerAddressID, optionID)

	var orderID uuid.UUID
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		order, err := svc.CreateFromSaleSurface(ctx, tx, input(productID, saleID, activeSnapshot))
		if err != nil {
			return err
		}
		orderID = order.ID
		return nil
	}))

	status, usedAt = sqQuoteStatus(t, ctx, tdb, activeQuote.ID)
	require.Equal(t, string(shippingquoteEntity.QuoteStatusUsed), status)
	require.NotNil(t, usedAt, "consuming a quote at checkout must stamp used_at")

	var (
		shipSourceDB, sourceTypeDB   string
		quoteIDDB, sellerDB, buyerDB uuid.UUID
		activeShipOptionID           *uuid.UUID
	)
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT shipping_source, source_type, shipping_quote_id, seller_id, buyer_id, shipping_option_id
		FROM orders WHERE id = $1
	`, orderID).Scan(&shipSourceDB, &sourceTypeDB, &quoteIDDB, &sellerDB, &buyerDB, &activeShipOptionID))
	require.Equal(t, "shipping_quote", shipSourceDB)
	require.Equal(t, quoteIDDB, activeQuote.ID)
	require.Equal(t, sellerDB, sellerID)
	require.Equal(t, buyerDB, buyerID)
	require.Nil(t, activeShipOptionID, "CASE C: shipping quote must replace the normal shipping option")
	// ...and normal shipping genuinely exists for this product/destination.
	var normalCoverage int
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT count(*)
		FROM product_shipping_options pso
		JOIN shipping_coverages sc ON sc.shipping_option_id = pso.shipping_option_id
		WHERE pso.product_id = $1 AND sc.province_code = '31' AND sc.is_available = true
	`, productID).Scan(&normalCoverage))
	require.GreaterOrEqual(t, normalCoverage, 1, "normal shipping must be available for the covered destination")

	// ---- USED quote cannot create a second order ------------------------
	reuseSnapshot := sqSnapshot(activeQuote.ID)
	stage5PricingToken(t, ctx, tdb, reuseSnapshot, buyerID, buyerAddressID, optionID)
	var reusedOrderID uuid.UUID
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		order, err := svc.CreateFromSaleSurface(ctx, tx, input(productID, saleID, reuseSnapshot))
		if err != nil {
			return err
		}
		reusedOrderID = order.ID
		return nil
	}))
	require.Equal(t, orderID, reusedOrderID, "reusing a USED quote must converge on the existing order")

	var orderCount int
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT count(*) FROM orders WHERE shipping_quote_id = $1`, activeQuote.ID).Scan(&orderCount))
	require.Equal(t, 1, orderCount, "a USED quote must never produce a second order")

	// =====================================================================
	// CASE A — COVERED destination: NORMAL shipping checkout succeeds.
	// =====================================================================
	normalProductID, normalSaleID := sqActiveForSale(t, ctx, tdb, sellerID, "SQ Normal Koi", 2)
	normalOptionID := stage5Shipping(t, ctx, tdb, sellerID, normalProductID)
	normalSnapshot := stage5Snapshot(uuid.New(), 100_000)
	stage5PricingToken(t, ctx, tdb, normalSnapshot, buyerID, buyerAddressID, normalOptionID)
	var normalOrderID uuid.UUID
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		order, err := svc.CreateFromSaleSurface(ctx, tx, orderApp.CreateFromSaleSurfaceInput{
			ProductID:       normalProductID,
			SourceType:      orderentity.OrderSourceForSale,
			SourceID:        normalSaleID,
			BuyerID:         buyerID,
			Quantity:        1,
			AddressID:       buyerAddressID,
			ShippingSetupID: normalOptionID,
			PricingSnapshot: normalSnapshot,
		})
		if err != nil {
			return err
		}
		normalOrderID = order.ID
		return nil
	}))
	var normalShipOptionID, normalQuoteID *uuid.UUID
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT shipping_option_id, shipping_quote_id FROM orders WHERE id = $1`, normalOrderID).
		Scan(&normalShipOptionID, &normalQuoteID))
	require.NotNil(t, normalShipOptionID, "CASE A: covered destination must produce a normal shipping option")
	require.Equal(t, normalOptionID, *normalShipOptionID)
	require.Nil(t, normalQuoteID)

	// =====================================================================
	// CASE B — NOT COVERED destination.
	//  (B1) checkout without a quote must NOT invent an invalid option.
	//  (B2) the Chat shipping quote path works.
	// =====================================================================
	uncoveredProductID, uncoveredSaleID := sqActiveForSale(t, ctx, tdb, sellerID, "SQ Uncovered Koi", 2)
	uncoveredNormalSnap := stage5Snapshot(uuid.New(), 100_000)
	stage5PricingToken(t, ctx, tdb, uncoveredNormalSnap, buyerID, buyerAddressID, optionID)
	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := svc.CreateFromSaleSurface(ctx, tx, orderApp.CreateFromSaleSurfaceInput{
			ProductID:       uncoveredProductID,
			SourceType:      orderentity.OrderSourceForSale,
			SourceID:        uncoveredSaleID,
			BuyerID:         buyerID,
			Quantity:        1,
			AddressID:       buyerAddressID,
			ShippingSetupID: optionID, // client may send an option, but none exists for the product
			PricingSnapshot: uncoveredNormalSnap,
		})
		return err
	})
	require.Error(t, err, "CASE B1: checkout without valid shipping must fail")
	require.True(t, errors.Is(err, shippingApp.ErrNoShippingSetups),
		"CASE B1: expected ErrNoShippingSetups, got: %v", err)

	uncoveredQuote := shippingquoteEntity.NewShippingQuote(
		roomID, uncoveredProductID, "for_sale", uncoveredSaleID, sellerID, buyerID,
		money.New(25_000), &note, &destCity, &destProvince, time.Now().Add(24*time.Hour),
	)
	sqCreateQuote(t, ctx, tdb, uncoveredQuote)
	uncoveredQuoteSnap := sqSnapshot(uncoveredQuote.ID)
	stage5PricingToken(t, ctx, tdb, uncoveredQuoteSnap, buyerID, buyerAddressID, optionID)
	var uncoveredOrderID uuid.UUID
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		order, err := svc.CreateFromSaleSurface(ctx, tx, input(uncoveredProductID, uncoveredSaleID, uncoveredQuoteSnap))
		if err != nil {
			return err
		}
		uncoveredOrderID = order.ID
		return nil
	}), "CASE B2: uncovered destination must complete via a Chat shipping quote")
	uncoveredStatus, _ := sqQuoteStatus(t, ctx, tdb, uncoveredQuote.ID)
	require.Equal(t, string(shippingquoteEntity.QuoteStatusUsed), uncoveredStatus)
	require.NotEqual(t, uuid.Nil, uncoveredOrderID)

	// =====================================================================
	// CHAT BINDING — same buyer + wrong chat id is denied.
	// =====================================================================
	chatProductID, chatSaleID := sqActiveForSale(t, ctx, tdb, sellerID, "SQ Chat Binding Koi", 2)
	chatQuote := shippingquoteEntity.NewShippingQuote(
		roomID, chatProductID, "for_sale", chatSaleID, sellerID, buyerID,
		money.New(25_000), &note, &destCity, &destProvince, time.Now().Add(24*time.Hour),
	)
	sqCreateQuote(t, ctx, tdb, chatQuote)
	otherRoom := uuid.New()
	wrongChatSnap := sqSnapshot(chatQuote.ID)
	wrongChatSnap.ChatID = &otherRoom
	stage5PricingToken(t, ctx, tdb, wrongChatSnap, buyerID, buyerAddressID, optionID)
	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := svc.CreateFromSaleSurface(ctx, tx, input(chatProductID, chatSaleID, wrongChatSnap))
		return err
	})
	var chatRejection *shippingquoteEntity.CheckoutRejectionError
	require.ErrorAs(t, err, &chatRejection)
	require.Equal(t, "chat_mismatch", chatRejection.Field,
		"a quote must only be usable by the chat it was issued in")
}
