//go:build integration

package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	auctionentity "github.com/labuda/backend/internal/commerce/auction/entity"
	auctionRepoImpl "github.com/labuda/backend/internal/commerce/auction/infrastructure/repository"
	fpsentity "github.com/labuda/backend/internal/commerce/forsale/entity"
	forsaleRepoImpl "github.com/labuda/backend/internal/commerce/forsale/infrastructure/repository"
	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
	orderRepoImpl "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	productentity "github.com/labuda/backend/internal/commerce/product/entity"
	productRepoImpl "github.com/labuda/backend/internal/commerce/product/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/labuda/backend/pkg/testdb"
)

// ============================================================================
// STAGE 5 (7): ORDER COMPLETION / STOCK RESTORATION — SOURCE-RESOLUTION PROOF
//
// Proves against real Postgres that restoreForSaleStock resolves the selling
// surface from orders.source_type + orders.source_id and never interprets
// order_items.product_id as a selling-surface ID:
//   - FPS/negotiation order: item.product_id is products.id; the listing is
//     locked via order.source_id; quantity is restored.
//   - Auction order: item.product_id is products.id; the auction binding is
//     released via order.source_id (existing PASS_20B behavior preserved).
// ============================================================================

func TestStage5_RestoreListingStock_ResolvesSurfaceFromOrderSource(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	sellerID := uuid.New()
	buyerID := uuid.New()
	seedStage5CompletionUsers(t, ctx, tdb, sellerID, buyerID)

	orderRepo := orderRepoImpl.NewOrderRepository()
	forSaleRepo := forsaleRepoImpl.NewForSaleRepository()
	productRepo := productRepoImpl.NewProductRepository()
	auctionRepo := auctionRepoImpl.NewAuctionRepository()
	svc := &OrderCompletionService{
		repo:        orderRepo,
		forSaleRepo: forSaleRepo,
		auctionRepo: auctionRepo,
	}

	// --- Seed: product + active FPS with 2 units ---
	var productID, fpsID uuid.UUID
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		product := &productentity.Product{SellerID: sellerID, Title: "Kohaku", Description: "d", Variety: "Kohaku", PreparationTime: "immediate"}
		if err := productRepo.Create(ctx, tx, product); err != nil {
			return err
		}
		productID = product.ID
		listing, err := fpsentity.NewForSale(
			sellerID, "Kohaku", "d", []byte(`[]`), "Kohaku",
			nil, nil, nil, nil, nil, []string{},
			fpsentity.ForSaleTypeFixedPrice, money.New(50000), 2, false,
			fpsentity.ForSaleVisibilityPublic, fpsentity.ForSaleOriginDirectCreate,
			nil, fpsentity.PreparationTimeImmediate, nil,
		)
		if err != nil {
			return err
		}
		listing.ProductID = product.ID
		if err := listing.Publish(); err != nil {
			return err
		}
		if err := forSaleRepo.Create(ctx, tx, listing); err != nil {
			return err
		}
		fpsID = listing.ID
		return nil
	}))

	// --- Seed: reduce stock to 1 (as order creation would) ---
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		listing, err := forSaleRepo.GetForUpdate(ctx, tx, fpsID)
		if err != nil {
			return err
		}
		if err := listing.ReduceQuantity(1); err != nil {
			return err
		}
		return forSaleRepo.UpdateStock(ctx, tx, listing)
	}))

	// --- Seed: FPS order whose item.product_id is products.id (new contract) ---
	order := orderentity.NewOrderFromSource(
		buyerID, sellerID, orderentity.OrderSourceForSale, fpsID, nil,
		1, money.New(50000), money.New(50000), money.New(0),
		0, money.New(0), money.New(0), money.New(50000),
		nil, "", "", nil, "immediate", nil, nil, nil, nil, nil,
		"instant", time.Now(),
	)
	order.ID = uuid.New()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		if err := orderRepo.CreateOrderTx(ctx, tx, order); err != nil {
			return err
		}
		item := orderentity.NewOrderItem(order.ID, productID, money.New(50000), 1, "Kohaku")
		return orderRepo.CreateOrderItemTx(ctx, tx, item)
	}))

	// --- PROOF: restore resolves the listing via order.source_id, not item.product_id ---
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		if err := svc.restoreForSaleStock(ctx, tx, order); err != nil {
			return err
		}
		listing, err := forSaleRepo.GetByID(ctx, tx, fpsID)
		if err != nil {
			return err
		}
		require.Equal(t, 2, listing.QuantityAvailable,
			"restore must return stock to the sourcing listing via order.source_id")
		return nil
	}))

	// --- AUCTION SIDE (preserved behavior): release binding via order.source_id ---
	// Uses its own product: the product above still has an ACTIVE fixed-price
	// sale (rule 9 forbids a simultaneous active auction on the same product).
	var auctionProductID uuid.UUID
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		product := &productentity.Product{SellerID: sellerID, Title: "Auction Koi", Description: "d", Variety: "Showa", PreparationTime: "immediate"}
		if err := productRepo.Create(ctx, tx, product); err != nil {
			return err
		}
		auctionProductID = product.ID
		return nil
	}))
	var auctionOrderID uuid.UUID
	auctionOrder := orderentity.NewOrderFromSource(
		buyerID, sellerID, orderentity.OrderSourceAuction, uuid.Nil, nil,
		1, money.New(40000), money.New(40000), money.New(0),
		0, money.New(0), money.New(0), money.New(40000),
		nil, "", "", nil, "immediate", nil, nil, nil, nil, nil,
		"instant", time.Now(),
	)
	auctionOrder.ID = uuid.New()
	auctionOrderID = auctionOrder.ID

	var auctionID uuid.UUID
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		auction := auctionentity.NewDraft(
			sellerID, auctionProductID, 40000, 1000, nil,
			time.Now().Add(-time.Hour), time.Now().Add(time.Hour),
		)
		if err := auction.Schedule(); err != nil {
			return err
		}
		if err := auction.Activate(); err != nil {
			return err
		}
		if err := auctionRepo.CreateTx(ctx, tx, auction); err != nil {
			return err
		}
		auctionID = auction.ID
		return nil
	}))

	auctionOrder.SourceID = auctionID
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		if err := orderRepo.CreateOrderTx(ctx, tx, auctionOrder); err != nil {
			return err
		}
		item := orderentity.NewOrderItem(auctionOrder.ID, auctionProductID, money.New(40000), 1, "A")
		return orderRepo.CreateOrderItemTx(ctx, tx, item)
	}))

	// Simulate settled auction bound to its order, status ended (order row must
	// exist first — auctions.order_id FK references orders).
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		auction, err := auctionRepo.GetForUpdate(ctx, tx, auctionID)
		if err != nil {
			return err
		}
		auction.OrderID = &auctionOrderID
		if err := auction.End(); err != nil {
			return err
		}
		return auctionRepo.UpdateTx(ctx, tx, auction)
	}))
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		if err := svc.restoreForSaleStock(ctx, tx, auctionOrder); err != nil {
			return err
		}
		auction, err := auctionRepo.GetByID(ctx, tx, auctionID)
		if err != nil {
			return err
		}
		require.Nil(t, auction.OrderID,
			"auction order binding must be released via order.source_id")
		return nil
	}))
}

func seedStage5CompletionUsers(t *testing.T, ctx context.Context, tdb *testdb.TestDB, sellerID, buyerID uuid.UUID) {
	t.Helper()
	for _, id := range []uuid.UUID{sellerID, buyerID} {
		id := id
		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO users (id, firebase_uid, email, email_verified_at, account_status, created_at, updated_at)
				VALUES ($1, $2, $3, NOW(), 'active', NOW(), NOW())
			`, id, "fb-"+id.String(), id.String()+"@stage5completion.invalid")
			return err
		}))
	}
}
