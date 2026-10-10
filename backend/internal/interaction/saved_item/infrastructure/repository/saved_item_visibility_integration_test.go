//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	savedItemEntity "github.com/hishumi/backend/internal/interaction/saved_item/entity"
	"github.com/hishumi/backend/pkg/db"
	"github.com/hishumi/backend/pkg/testdb"
)

func TestSavedVisibility_OnlyActiveExistingCommerce(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	sellerID, viewerID := uuid.New(), uuid.New()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		if _, err := tx.Exec(ctx,
			`INSERT INTO users (id, firebase_uid, email) VALUES ($1, $2, $3), ($4, $5, $6)`,
			sellerID, "fb-"+sellerID.String(), sellerID.String()+"@saved.test",
			viewerID, "fb-"+viewerID.String(), viewerID.String()+"@saved.test",
		); err != nil {
			return err
		}

		productID := uuid.New()
		if _, err := tx.Exec(ctx, `INSERT INTO products
			(id, seller_id, title, description, media_urls, variety, preparation_time)
			VALUES ($1, $2, 'Saved fixture', 'fixture', '[]', 'showa', '1_3_days')`,
			productID, sellerID); err != nil {
			return err
		}

		activeSale, soldSale, withdrawnSale := uuid.New(), uuid.New(), uuid.New()
		for id, status := range map[uuid.UUID]string{
			activeSale: "active", soldSale: "sold", withdrawnSale: "withdrawn",
		} {
			productID := uuid.New()
			if _, err := tx.Exec(ctx, `INSERT INTO products
				(id, seller_id, title, description, media_urls, variety, preparation_time)
				VALUES ($1, $2, 'Saved sale fixture', 'fixture', '[]', 'showa', '1_3_days')`,
				productID, sellerID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO for_sales
				(id, product_id, seller_id, price_per_unit, status, published_at, quantity_available)
				VALUES ($1, $2, $3, 100000, $4, NOW(), $5)`,
				id, productID, sellerID, status, func() int {
					if status == "active" {
						return 1
					}
					return 0
				}()); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO saved_items
				(id, user_id, target_type, target_id, intent_type)
				VALUES ($1, $2, 'for_sale', $3, 'bookmark')`,
				uuid.New(), viewerID, id); err != nil {
				return err
			}
		}

		activeAuction, endedAuction, cancelledAuction, lapsedAuction, waitingAuction := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
		statuses := map[uuid.UUID]string{
			activeAuction: "active", endedAuction: "ended", cancelledAuction: "cancelled",
			lapsedAuction: "lapsed", waitingAuction: "waiting_settlement",
		}
		for id, status := range statuses {
			productID := uuid.New()
			if _, err := tx.Exec(ctx, `INSERT INTO products
				(id, seller_id, title, description, media_urls, variety, preparation_time)
				VALUES ($1, $2, 'Saved auction fixture', 'fixture', '[]', 'showa', '1_3_days')`,
				productID, sellerID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO auctions
				(id, seller_id, product_id, start_price, bid_increment, start_at, end_at, status)
				VALUES ($1, $2, $3, 100000, 10000, NOW() - INTERVAL '1 hour', NOW() + INTERVAL '1 hour', $4)`,
				id, sellerID, productID, status); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO saved_items
				(id, user_id, target_type, target_id, intent_type)
				VALUES ($1, $2, 'auction', $3, 'watch')`,
				uuid.New(), viewerID, id); err != nil {
				return err
			}
		}

		missing := uuid.New()
		_, err := tx.Exec(ctx, `INSERT INTO saved_items
			(id, user_id, target_type, target_id, intent_type)
			VALUES ($1, $2, 'auction', $3, 'watch')`, uuid.New(), viewerID, missing)
		return err
	}))

	repo := NewSavedItemRepository(db.NewFromPool(tdb.Pool()))
	forSales, err := repo.GetByUserWithForSales(ctx, viewerID)
	require.NoError(t, err)
	require.Len(t, forSales, 1)
	require.Equal(t, "active", forSales[0].ForSaleStatus)

	auctions, err := repo.GetByUserWithAuctions(ctx, viewerID)
	require.NoError(t, err)
	require.Len(t, auctions, 1)
	require.Equal(t, "active", auctions[0].AuctionStatus)

	total, err := repo.Count(ctx, viewerID)
	require.NoError(t, err)
	require.Equal(t, 2, total)
	forSaleCount, err := repo.CountByType(ctx, viewerID, savedItemEntity.TargetTypeForSale)
	require.NoError(t, err)
	require.Equal(t, 1, forSaleCount)
	auctionCount, err := repo.CountByType(ctx, viewerID, savedItemEntity.TargetTypeAuction)
	require.NoError(t, err)
	require.Equal(t, 1, auctionCount)
}
