//go:build integration

// PASS_21B regression test: GetByUserWithForSales hydrates a saved
// fixed-price forSale from for_sales JOIN products. Before PASS_21B
// this query read from the legacy `forSales` table, which nothing writes to
// anymore — every real saved forSale hydrated with a null title/price/media.
package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

func TestGetByUserWithForSales_HydratesRealForSale(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	sellerID := uuid.New()
	viewerID := uuid.New()
	productID := uuid.New()
	forSaleID := uuid.New()

	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		if _, err := tx.Exec(ctx,
			`INSERT INTO users (id, firebase_uid, email) VALUES ($1, $2, $3), ($4, $5, $6)`,
			sellerID, "fb-"+sellerID.String(), sellerID.String()+"@saveditem.test",
			viewerID, "fb-"+viewerID.String(), viewerID.String()+"@saveditem.test",
		); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, productID, sellerID, "Showa Koi", "A fine showa", `["https://cdn.example.com/koi.jpg"]`, "showa", "immediate"); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, status, published_at, quantity_available)
			VALUES ($1, $2, $3, $4, 'active', NOW(), $5)
		`, forSaleID, productID, sellerID, int64(150000), 3); err != nil {
			return err
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO saved_items (id, user_id, target_type, target_id, intent_type, seller_id)
			VALUES ($1, $2, 'for_sale', $3, 'bookmark', $4)
		`, uuid.New(), viewerID, forSaleID, sellerID)
		return err
	}))

	repo := NewSavedItemRepository(db.NewFromPool(tdb.Pool()))

	items, err := repo.GetByUserWithForSales(ctx, viewerID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	item := items[0]
	require.Equal(t, forSaleID, item.TargetID)
	require.Equal(t, "Showa Koi", item.ForSaleTitle)
	require.Equal(t, int64(150000), item.ForSalePrice)
	require.Equal(t, "fixed_price", item.ForSaleType)
	require.Equal(t, 3, item.QuantityAvailable)
	require.Equal(t, "active", item.ForSaleStatus)
	require.Equal(t, "public", item.ForSaleVisibility)
	require.Contains(t, string(item.ForSaleMediaURLs), "cdn.example.com/koi.jpg")
}
