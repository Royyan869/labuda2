//go:build integration

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	forsaleentity "github.com/labuda/backend/internal/commerce/forsale/entity"
	forsalerepo "github.com/labuda/backend/internal/commerce/forsale/infrastructure/repository"
	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
	orderrepo "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/labuda/backend/pkg/testdb"
)

func TestFPS002_OrderCompletionIdempotency(t *testing.T) {
	ctx := context.Background()
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	t.Run("Cancel same key succeeds twice", func(t *testing.T) {
		sellerID, buyerID := uuid.New(), uuid.New()
		insertOrderTestUsers(t, ctx, tdb, sellerID, buyerID)
		orderID := seedFPS002PendingOrder(t, ctx, tdb, sellerID, buyerID)
		svc := newTestOrderCompletionService(t)

		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			return svc.Cancel(ctx, tx, orderID, "fps-002-cancel-same", buyerID)
		}))
		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			return svc.Cancel(ctx, tx, orderID, "fps-002-cancel-same", buyerID)
		}))
		assertFPS002Status(t, ctx, tdb, orderID, orderentity.StatusCancelled)
		assertFPS002Quantity(t, ctx, tdb, orderID, 1)
	})

	t.Run("Cancel different key rejects second transition", func(t *testing.T) {
		sellerID, buyerID := uuid.New(), uuid.New()
		insertOrderTestUsers(t, ctx, tdb, sellerID, buyerID)
		orderID := seedFPS002PendingOrder(t, ctx, tdb, sellerID, buyerID)
		svc := newTestOrderCompletionService(t)

		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			return svc.Cancel(ctx, tx, orderID, "fps-002-cancel-first", buyerID)
		}))
		require.Error(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			return svc.Cancel(ctx, tx, orderID, "fps-002-cancel-second", buyerID)
		}))
		assertFPS002Status(t, ctx, tdb, orderID, orderentity.StatusCancelled)
		assertFPS002Quantity(t, ctx, tdb, orderID, 1)
	})

	t.Run("Expire twice rejects second transition", func(t *testing.T) {
		sellerID, buyerID := uuid.New(), uuid.New()
		insertOrderTestUsers(t, ctx, tdb, sellerID, buyerID)
		orderID := seedFPS002PendingOrder(t, ctx, tdb, sellerID, buyerID)
		svc := newTestOrderCompletionService(t)

		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error { return svc.Expire(ctx, tx, orderID) }))
		require.Error(t, tdb.WithTx(ctx, func(tx db.Tx) error { return svc.Expire(ctx, tx, orderID) }))
		assertFPS002Status(t, ctx, tdb, orderID, orderentity.StatusExpired)
		assertFPS002Quantity(t, ctx, tdb, orderID, 1)
	})
}

func seedFPS002PendingOrder(t *testing.T, ctx context.Context, tdb *testdb.TestDB, sellerID, buyerID uuid.UUID) uuid.UUID {
	t.Helper()
	listingRepo := forsalerepo.NewForSaleRepository()
	orderRepo := orderrepo.NewOrderRepository()
	var listingID uuid.UUID
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		listing, err := forsaleentity.NewForSale(sellerID, "FPS-002", "test", []byte(`[]`), "Kohaku", nil, nil, nil, nil, nil, []string{}, forsaleentity.ForSaleTypeFixedPrice, money.New(50000), 1, false, forsaleentity.ForSaleVisibilityPublic, forsaleentity.ForSaleOriginDirectCreate, nil, forsaleentity.PreparationTimeImmediate, nil)
		if err != nil {
			return err
		}
		if err := listing.Publish(); err != nil {
			return err
		}
		if err := listingRepo.Create(ctx, tx, listing); err != nil {
			return err
		}
		if err := listing.ReduceQuantity(1); err != nil {
			return err
		}
		if err := listingRepo.UpdateStock(ctx, tx, listing); err != nil {
			return err
		}
		listingID = listing.ID
		order := orderentity.NewOrderFromSource(buyerID, sellerID, orderentity.OrderSourceForSale, listingID, nil, 1, money.New(50000), money.New(50000), money.New(0), 0, money.New(0), money.New(0), money.New(50000), nil, "", "", nil, "immediate", nil, nil, nil, nil, nil, "instant", time.Now())
		if err := orderRepo.CreateOrderTx(ctx, tx, order); err != nil {
			return err
		}
		item := orderentity.NewOrderItem(order.ID, listing.ProductID, money.New(50000), 1, "FPS-002")
		if err := orderRepo.CreateOrderItemTx(ctx, tx, item); err != nil {
			return err
		}
		return nil
	}))
	var orderID uuid.UUID
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT id FROM orders WHERE source_id = $1 ORDER BY created_at DESC LIMIT 1`, listingID).Scan(&orderID)
	}))
	return orderID
}

func assertFPS002Quantity(t *testing.T, ctx context.Context, tdb *testdb.TestDB, orderID uuid.UUID, want int) {
	t.Helper()
	var got int
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT quantity_available FROM for_sales f JOIN orders o ON o.source_id = f.id WHERE o.id = $1`, orderID).Scan(&got)
	}))
	require.Equal(t, want, got)
}

func assertFPS002Status(t *testing.T, ctx context.Context, tdb *testdb.TestDB, orderID uuid.UUID, want orderentity.Status) {
	t.Helper()
	var got orderentity.Status
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&got)
	}))
	require.Equal(t, want, got)
}
