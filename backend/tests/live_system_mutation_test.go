//go:build integration

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/internal/commerce/forsale/entity"
	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	"github.com/labuda/backend/pkg/testdb"
)

func TestSystemMutation_ActiveForSale_ReduceQuantityStillAllowed(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	seller := seedLiveUser(t, ctx, tdb)
	productID := uuid.New()
	forSaleID := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, selling_surface, created_at, updated_at) VALUES ($1,$2,'prod','desc','[]','Kohaku','immediate','for_sale',NOW(),NOW())`, productID, seller)
	require.NoError(t, err)
	_, err = tdb.Pool().Exec(ctx, `INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, negotiation_enabled, status, quantity_available, created_at, updated_at) VALUES ($1,$2,$3,100000,false,'active',2,NOW(),NOW())`, forSaleID, productID, seller)
	require.NoError(t, err)
	// Simulate system ReduceQuantity (order creation) — direct entity test but via DB
	// Load for_sale via handler's GetForUpdate would lock, but we just test entity logic still works when active
	forSale := &entity.ForSale{ID: forSaleID, SellerID: seller, ProductID: productID, Status: entity.ForSaleStatusActive, QuantityAvailable: 2}
	require.NoError(t, forSale.ReduceQuantity(1))
	require.Equal(t, 1, forSale.QuantityAvailable)
	require.Equal(t, entity.ForSaleStatusActive, forSale.Status)
	require.NoError(t, forSale.ReduceQuantity(1))
	require.Equal(t, 0, forSale.QuantityAvailable)
	require.Equal(t, entity.ForSaleStatusSold, forSale.Status)
}

func TestSystemMutation_ActiveAuction_PlaceBidStillAllowed(t *testing.T) {
	seller := uuid.New()
	bidder := uuid.New()
	productID := uuid.New()
	auction := &auctionEntity.Auction{
		ID:        uuid.New(),
		SellerID:  seller,
		ProductID: productID,
		StartPrice: 100000,
		BidIncrement: 10000,
		StartAt:   time.Now().Add(-1 * time.Hour),
		EndAt:     time.Now().Add(2 * time.Hour),
		Status:    auctionEntity.StatusActive,
	}
	require.NoError(t, auction.PlaceBid(bidder, 110000, time.Now()))
	require.NotNil(t, auction.CurrentBid)
	require.Equal(t, int64(110000), *auction.CurrentBid)
}
