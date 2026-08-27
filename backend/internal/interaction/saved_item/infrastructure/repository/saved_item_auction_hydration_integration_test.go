//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

func TestGetByUserWithAuctions_HydratesRealAuction(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	sellerID := uuid.New()
	viewerID := uuid.New()
	productID := uuid.New()
	auctionID := uuid.New()

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
		`, productID, sellerID, "Auction Koi", "A live auction", `["https://cdn.example.com/auction.jpg"]`, "showa", "immediate"); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO auctions (
				id, seller_id, product_id,
				start_price, bid_increment, start_at, end_at, current_bid, status, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, NOW(), NOW() + INTERVAL '2 hour', $6, 'active', NOW(), NOW())
		`, auctionID, sellerID, productID, int64(1500000), int64(100000), int64(1750000)); err != nil {
			return err
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO saved_items (id, user_id, target_type, target_id, intent_type)
			VALUES ($1, $2, 'auction', $3, 'watch')
		`, uuid.New(), viewerID, auctionID)
		return err
	}))

	repo := NewSavedItemRepository(db.NewFromPool(tdb.Pool()))

	items, err := repo.GetByUserWithAuctions(ctx, viewerID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	item := items[0]
	require.Equal(t, auctionID, item.TargetID)
	require.Equal(t, "Auction Koi", item.AuctionTitle)
	require.Equal(t, "active", item.AuctionStatus)
	require.NotNil(t, item.StartPrice)
	require.Equal(t, int64(1500000), *item.StartPrice)
	require.NotNil(t, item.CurrentBid)
	require.Equal(t, int64(1750000), *item.CurrentBid)
	require.NotNil(t, item.EndAt)
}
