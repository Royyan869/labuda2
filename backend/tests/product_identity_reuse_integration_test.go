//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	forsaleEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	forsaleRepo "github.com/labuda/backend/internal/commerce/forsale/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/labuda/backend/pkg/testdb"
)

// Product identity & selling-surface reuse — Stage 1 runtime proof (Model B).
//
// Real Postgres (labuda_test) proves:
//   1. a ForSale can attach to an existing Product (ProductID reuse);
//   2. reuse does NOT duplicate the Product row;
//   3. a Product with an active FPS rejects a second FPS (DB trigger);
//   4. after the FPS sells, the SAME Product is reusable;
//   5. the old selling surface stays historically intact;
//   6. an Auction can attach to the SAME Product after the FPS sells;
//   7. two ACTIVE surfaces on one Product are rejected;
//   8. reusing another seller's Product is rejected without touching data.

func seedStage1User(t *testing.T, ctx context.Context, appDB *db.DB) uuid.UUID {
	t.Helper()
	userID := uuid.New()
	err := appDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO users (id, firebase_uid, email, email_verified_at, account_status, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), 'active', NOW(), NOW())
		`, userID, "fb-"+userID.String(), userID.String()+"@test.invalid")
		return err
	})
	require.NoError(t, err)
	return userID
}

func seedStage1Product(t *testing.T, ctx context.Context, appDB *db.DB, sellerID uuid.UUID) uuid.UUID {
	t.Helper()
	productID := uuid.New()
	err := appDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, created_at, updated_at)
			VALUES ($1, $2, 'Kohaku', 'desc', $3, 'kohaku', 'immediate', NOW(), NOW())
		`, productID, sellerID, `["https://example.com/koi.jpg"]`)
		return err
	})
	require.NoError(t, err)
	return productID
}

func stage1ForSale(sellerID, productID uuid.UUID, title string, quantity int) *forsaleEntity.ForSale {
	media, _ := json.Marshal([]string{"https://example.com/koi.jpg"})
	forSale, err := forsaleEntity.NewForSale(
		sellerID,
		title,
		"desc",
		media,
		"Kohaku",
		nil, nil, nil, nil, nil,
		[]string{},
		forsaleEntity.ForSaleTypeFixedPrice,
		money.New(100000),
		quantity,
		false,
		forsaleEntity.ForSaleVisibilityPrivate,
		forsaleEntity.ForSaleOriginDirectCreate,
		nil,
		forsaleEntity.PreparationTimeImmediate,
		nil,
	)
	if err != nil {
		panic(err)
	}
	forSale.ProductID = productID // explicit Product identity reuse
	return forSale
}

func stage1ProductCount(t *testing.T, ctx context.Context, appDB *db.DB, productID uuid.UUID) int {
	t.Helper()
	var count int
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT COUNT(*) FROM products WHERE id = $1`, productID).Scan(&count)
	}))
	return count
}

func stage1FPSCount(t *testing.T, ctx context.Context, appDB *db.DB, productID uuid.UUID) int {
	t.Helper()
	var count int
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT COUNT(*) FROM for_sales WHERE product_id = $1`, productID).Scan(&count)
	}))
	return count
}

func TestProductIdentityReuse_Stage1_Runtime(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	appDB := db.NewFromPool(tdb.Pool())
	ctx := context.Background()

	sellerID := seedStage1User(t, ctx, appDB)
	otherSellerID := seedStage1User(t, ctx, appDB)
	productID := seedStage1Product(t, ctx, appDB, sellerID)
	repo := forsaleRepo.NewForSaleRepository()
	require.Equal(t, 1, stage1ProductCount(t, ctx, appDB, productID))

	// 1. Create FPS #1 that REUSES the existing Product.
	fps1 := stage1ForSale(sellerID, productID, "Koi-First", 1)
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		return repo.Create(ctx, tx, fps1)
	}))
	require.Equal(t, productID, fps1.ProductID, "reuse must keep the exact product id")
	require.Equal(t, 1, stage1ProductCount(t, ctx, appDB, productID), "reuse must not duplicate the Product")
	require.Equal(t, 1, stage1FPSCount(t, ctx, appDB, productID))

	// 2. A second FPS on the same Product while FPS #1 is draft is rejected.
	fps2 := stage1ForSale(sellerID, productID, "Koi-Second", 1)
	require.Error(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		return repo.Create(ctx, tx, fps2)
	}), "two active selling surfaces on one Product must be rejected by the DB")
	require.Equal(t, 1, stage1FPSCount(t, ctx, appDB, productID), "rejected surface must not leave a row")

	// 3. End FPS #1 (sold).
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE for_sales
			SET status = 'sold', sold_at = NOW(), quantity_available = 0
			WHERE id = $1
		`, fps1.ID)
		return err
	}))

	// 4. The SAME Product is reusable after the surface sold.
	fps3 := stage1ForSale(sellerID, productID, "Koi-Third", 2)
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		return repo.Create(ctx, tx, fps3)
	}))
	require.Equal(t, productID, fps3.ProductID)
	require.Equal(t, 1, stage1ProductCount(t, ctx, appDB, productID), "reuse after sale must not duplicate the Product")
	require.Equal(t, 2, stage1FPSCount(t, ctx, appDB, productID))

	// 5. Old surface remains historically intact.
	var oldProductID uuid.UUID
	var oldStatus string
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT product_id, status FROM for_sales WHERE id = $1`, fps1.ID).
			Scan(&oldProductID, &oldStatus)
	}))
	require.Equal(t, productID, oldProductID)
	require.Equal(t, "sold", oldStatus)

	// 6. An Auction can attach to the SAME Product once the FPS is sold
	//    (sold/ended statuses are outside the active-surface guard).
	insertAuction := func(status string) error {
		return appDB.WithTx(ctx, func(tx db.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO auctions (
					id, seller_id, product_id,
					start_price, bid_increment, buy_now_price, start_at, end_at,
					current_bid, status, created_at, updated_at
				)
				VALUES ($1, $2, $3, $4, $5, NULL, NOW(), NOW() + INTERVAL '24 hours', NULL, $6, NOW(), NOW())
			`, uuid.New(), sellerID, productID, 10000, 1000, status)
			return err
		})
	}
	require.NoError(t, insertAuction("ended"), "ended auction on a reusable Product must be allowed")

	// 7. Two ACTIVE surfaces on one Product are rejected (FPS active + auction active).
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE for_sales SET status = 'active', published_at = NOW() WHERE id = $1
		`, fps3.ID)
		return err
	}))
	require.Error(t, insertAuction("active"), "active auction while the FPS is active must be rejected by the DB")

	// 8. After FPS #2 sells, an ACTIVE auction on the SAME Product is allowed.
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE for_sales SET status = 'sold', sold_at = NOW(), quantity_available = 0 WHERE id = $1
		`, fps3.ID)
		return err
	}))
	require.NoError(t, insertAuction("active"), "auction may reuse the Product once the FPS has sold")
	require.Equal(t, 1, stage1ProductCount(t, ctx, appDB, productID), "auction reuse must not duplicate the Product")

	// 9. Reusing another seller's Product is rejected (FPS path), no data touched.
	foreignProduct := seedStage1Product(t, ctx, appDB, otherSellerID)
	foreignForSale := stage1ForSale(sellerID, foreignProduct, "Theft", 1)
	err := appDB.WithTx(ctx, func(tx db.Tx) error {
		return repo.Create(ctx, tx, foreignForSale)
	})
	require.Error(t, err, "reusing a Product owned by another seller must be rejected")
	require.Contains(t, err.Error(), "owned by another seller")
	require.Equal(t, 1, stage1ProductCount(t, ctx, appDB, foreignProduct), "rejected ownership must not write any product row")
}
