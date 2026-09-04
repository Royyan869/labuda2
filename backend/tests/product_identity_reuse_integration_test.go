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

// Product identity & selling-surface exclusivity — integration proof.
//
// Real Postgres proves:
//   1. a ForSale can attach to an existing Product (ProductID reuse);
//   2. reuse does NOT duplicate the Product row;
//   3. a second ForSale on the same Product is rejected after prior sale;
//   4. cross-type reuse is PERMANENTLY rejected (ForSale→Auction);
//   5. cross-type reuse is PERMANENTLY rejected (Auction→ForSale);
//   6. terminal surface does NOT free Product for opposite type;
//   7. reusing another seller's Product is rejected;
//   8. selling_surface is immutable after claim.

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

func stage1AuctionCount(t *testing.T, ctx context.Context, appDB *db.DB, productID uuid.UUID) int {
	t.Helper()
	var count int
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT COUNT(*) FROM auctions WHERE product_id = $1`, productID).Scan(&count)
	}))
	return count
}

func TestProductSellingSurfaceExclusivity(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	appDB := db.NewFromPool(tdb.Pool())
	ctx := context.Background()

	sellerID := seedStage1User(t, ctx, appDB)
	otherSellerID := seedStage1User(t, ctx, appDB)
	productID := seedStage1Product(t, ctx, appDB, sellerID)
	repo := forsaleRepo.NewForSaleRepository()
	require.Equal(t, 1, stage1ProductCount(t, ctx, appDB, productID))

	t.Run("direct first ForSale insert claims selling surface", func(t *testing.T) {
		directProductID := seedStage1Product(t, ctx, appDB, sellerID)
		require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, negotiation_enabled, status, quantity_available, created_at, updated_at)
				VALUES ($1, $2, $3, 100000, false, 'active', 1, NOW(), NOW())
			`, uuid.New(), directProductID, sellerID)
			return err
		}))

		var surface string
		require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
			return tx.QueryRow(ctx, `SELECT selling_surface FROM products WHERE id = $1`, directProductID).Scan(&surface)
		}))
		require.Equal(t, "for_sale", surface)
	})

	t.Run("withdrawn ForSale with stock rejects second ForSale", func(t *testing.T) {
		withdrawnProductID := seedStage1Product(t, ctx, appDB, sellerID)
		firstSaleID := uuid.New()
		require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, negotiation_enabled, status, quantity_available, created_at, updated_at)
				VALUES ($1, $2, $3, 100000, false, 'withdrawn', 1, NOW(), NOW())
			`, firstSaleID, withdrawnProductID, sellerID)
			return err
		}))
		err := appDB.WithTx(ctx, func(tx db.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, negotiation_enabled, status, quantity_available, created_at, updated_at)
				VALUES ($1, $2, $3, 100000, false, 'active', 1, NOW(), NOW())
			`, uuid.New(), withdrawnProductID, sellerID)
			return err
		})
		require.Error(t, err)
		require.Equal(t, 1, stage1FPSCount(t, ctx, appDB, withdrawnProductID))
	})

	// ---- SCENARIO 1: ForSale creation succeeds, Auction on same Product rejected ----

	// 1a. Create ForSale reusing the Product.
	fps1 := stage1ForSale(sellerID, productID, "Koi-First", 1)
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		return repo.Create(ctx, tx, fps1)
	}))
	require.Equal(t, productID, fps1.ProductID, "reuse must keep the exact product id")
	require.Equal(t, 1, stage1ProductCount(t, ctx, appDB, productID), "reuse must not duplicate the Product")
	require.Equal(t, 1, stage1FPSCount(t, ctx, appDB, productID))

	// 1b. Verify selling_surface is set to 'for_sale'.
	var surface string
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT selling_surface FROM products WHERE id = $1`, productID).Scan(&surface)
	}))
	require.Equal(t, "for_sale", surface, "selling_surface must be 'for_sale' after ForSale creation")

	// 1c. Direct SQL Auction insert MUST be rejected (cross-type).
	err := appDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO auctions (
				id, seller_id, product_id,
				start_price, bid_increment, buy_now_price, start_at, end_at,
				status, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, NULL, NOW(), NOW() + INTERVAL '24 hours', 'draft', NOW(), NOW())
		`, uuid.New(), sellerID, productID, 10000, 1000)
		return err
	})
	require.Error(t, err, "Auction on for_sale Product MUST be rejected by DB trigger")
	require.Contains(t, err.Error(), "permanently claimed as for_sale")
	require.Equal(t, 0, stage1AuctionCount(t, ctx, appDB, productID), "rejected Auction must not leave a row")

	// ---- SCENARIO 2: Auction creation succeeds, ForSale on same Product rejected ----

	// Create a fresh Product for Auction.
	auctionProductID := seedStage1Product(t, ctx, appDB, sellerID)

	// 2a. Direct SQL Auction insert succeeds (selling_surface is NULL, allowed).
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO auctions (
				id, seller_id, product_id,
				start_price, bid_increment, buy_now_price, start_at, end_at,
				status, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, NULL, NOW(), NOW() + INTERVAL '24 hours', 'draft', NOW(), NOW())
		`, uuid.New(), sellerID, auctionProductID, 10000, 1000)
		return err
	}))

	// Note: We can't use ClaimSellingSurface via direct SQL, so we need to set selling_surface
	// via the service layer. For this test, we verify the trigger by setting it directly.
	// In production, ClaimSellingSurface is the only way to set it.
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE products SET selling_surface = 'auction' WHERE id = $1`, auctionProductID)
		return err
	}))

	// 2b. ForSale creation on the auction Product MUST be rejected.
	err = appDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, negotiation_enabled, status, quantity_available, created_at, updated_at)
			VALUES ($1, $2, $3, 100000, false, 'draft', 1, NOW(), NOW())
		`, uuid.New(), auctionProductID, sellerID)
		return err
	})
	require.Error(t, err, "ForSale on auction Product MUST be rejected by DB trigger")
	require.Contains(t, err.Error(), "permanently claimed as auction")

	// ---- SCENARIO 3: ForSale row remains unique after sale ----

	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE for_sales
			SET status = 'sold', sold_at = NOW(), quantity_available = 0
			WHERE id = $1
		`, fps1.ID)
		return err
	}))

	fps2 := stage1ForSale(sellerID, productID, "Koi-Second", 2)
	require.Error(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		return repo.Create(ctx, tx, fps2)
	}), "sold ForSale must prevent a second ForSale row")
	require.Equal(t, 1, stage1ProductCount(t, ctx, appDB, productID))
	require.Equal(t, 1, stage1FPSCount(t, ctx, appDB, productID))

	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT selling_surface FROM products WHERE id = $1`, productID).Scan(&surface)
	}))
	require.Equal(t, "for_sale", surface)

	// ---- SCENARIO 4: Terminal ForSale does NOT free Product for Auction ----

	// 4a. ForSale remains sold.
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE for_sales SET status = 'sold', sold_at = NOW(), quantity_available = 0 WHERE id = $1
		`, fps1.ID)
		return err
	}))

	// 4b. Auction on the Product with sold ForSales MUST still be rejected.
	err = appDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO auctions (
				id, seller_id, product_id,
				start_price, bid_increment, buy_now_price, start_at, end_at,
				status, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, NULL, NOW(), NOW() + INTERVAL '24 hours', 'active', NOW(), NOW())
		`, uuid.New(), sellerID, productID, 10000, 1000)
		return err
	})
	require.Error(t, err, "Auction after ForSale sold MUST be rejected — terminal does not free Product")

	// ---- SCENARIO 5: selling_surface is immutable after claim ----

	// 5a. Try to clear selling_surface back to NULL — MUST be rejected.
	err = appDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE products SET selling_surface = NULL WHERE id = $1`, productID)
		return err
	})
	require.Error(t, err, "selling_surface must not revert to NULL after claim")

	// 5b. Try to change selling_surface from 'for_sale' to 'auction' — MUST be rejected.
	err = appDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE products SET selling_surface = 'auction' WHERE id = $1`, productID)
		return err
	})
	require.Error(t, err, "selling_surface must not change from 'for_sale' to 'auction'")

	// ---- SCENARIO 6: Foreign seller Product reuse is rejected ----

	foreignProduct := seedStage1Product(t, ctx, appDB, otherSellerID)
	foreignForSale := stage1ForSale(sellerID, foreignProduct, "Theft", 1)
	err = appDB.WithTx(ctx, func(tx db.Tx) error {
		return repo.Create(ctx, tx, foreignForSale)
	})
	require.Error(t, err, "reusing a Product owned by another seller must be rejected")
	require.Contains(t, err.Error(), "owned by another seller")
	require.Equal(t, 1, stage1ProductCount(t, ctx, appDB, foreignProduct), "rejected ownership must not write any product row")

	// ---- SCENARIO 7: Second ForSale while first is draft is rejected (active surface per Product) ----

	freshProduct := seedStage1Product(t, ctx, appDB, sellerID)
	fps3 := stage1ForSale(sellerID, freshProduct, "Koi-Draft1", 1)
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		return repo.Create(ctx, tx, fps3)
	}))

	// A second ForSale on same Product while first is draft is rejected.
	fps4 := stage1ForSale(sellerID, freshProduct, "Koi-Draft2", 1)
	require.Error(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		return repo.Create(ctx, tx, fps4)
	}), "two active selling surfaces on one Product must be rejected")
}
