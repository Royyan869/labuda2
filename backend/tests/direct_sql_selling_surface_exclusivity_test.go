//go:build integration

package tests

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/pkg/testdb"
)

// TestDirectSQL_SellingSurfaceExclusivity performs direct SQL negative proofs
// against real Postgres to prove the database-level invariant:
//
// A Product may belong to EXACTLY ONE selling surface TYPE (for_sale OR auction)
// for its entire lifecycle. Once claimed, selling_surface is IMMUTABLE.
//
// NO application layer. NO ClaimSellingSurface. Pure SQL only.
func TestDirectSQL_SellingSurfaceExclusivity(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()

	// Seed seller
	sellerID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, email_verified_at, account_status, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), 'active', NOW(), NOW())
	`, sellerID, "fb-"+sellerID.String(), sellerID.String()+"@test.invalid")
	require.NoError(t, err)

	seedProduct := func(surface string) uuid.UUID {
		t.Helper()
		id := uuid.New()
		var execErr error
		if surface == "" {
			_, execErr = pool.Exec(ctx, `
				INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, created_at, updated_at)
				VALUES ($1, $2, 'Koi', 'desc', '[]'::jsonb, 'kohaku', 'immediate', NOW(), NOW())
			`, id, sellerID)
		} else {
			_, execErr = pool.Exec(ctx, `
				INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, selling_surface, created_at, updated_at)
				VALUES ($1, $2, 'Koi', 'desc', '[]'::jsonb, 'kohaku', 'immediate', $3, NOW(), NOW())
			`, id, sellerID, surface)
		}
		require.NoError(t, execErr)
		return id
	}

	insertForSale := func(productID uuid.UUID, status string) error {
		t.Helper()
		_, err := pool.Exec(ctx, `
			INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, negotiation_enabled, status, quantity_available, created_at, updated_at)
			VALUES ($1, $2, $3, 100000, false, $4, 1, NOW(), NOW())
		`, uuid.New(), productID, sellerID, status)
		return err
	}

	insertAuction := func(productID uuid.UUID, status string) error {
		t.Helper()
		_, err := pool.Exec(ctx, `
			INSERT INTO auctions (id, seller_id, product_id, start_price, bid_increment, buy_now_price, start_at, end_at, status, created_at, updated_at)
			VALUES ($1, $2, $3, 100000, 10000, NULL, NOW(), NOW() + INTERVAL '24 hours', $4, NOW(), NOW())
		`, uuid.New(), sellerID, productID, status)
		return err
	}

	assertCheckViolation := func(t *testing.T, err error, msg string) {
		t.Helper()
		require.Error(t, err, msg)
		var pgErr *pgconn.PgError
		require.ErrorAs(t, err, &pgErr, msg+": expected pg error")
		require.Equal(t, "23514", pgErr.Code, msg+": expected check_violation (23514), got "+pgErr.Code+": "+pgErr.Message)
	}

	// A: for_sale Product → Auction INSERT rejected
	t.Run("A: for_sale Product → Auction INSERT rejected", func(t *testing.T) {
		productID := seedProduct("for_sale")
		err := insertAuction(productID, "draft")
		assertCheckViolation(t, err, "Auction on for_sale Product")
	})

	// B: auction Product → ForSale INSERT rejected
	t.Run("B: auction Product → ForSale INSERT rejected", func(t *testing.T) {
		productID := seedProduct("auction")
		err := insertForSale(productID, "draft")
		assertCheckViolation(t, err, "ForSale on auction Product")
	})

	// C: sold ForSale does not free Product for Auction
	t.Run("C: sold ForSale → Auction still rejected", func(t *testing.T) {
		productID := seedProduct("for_sale")
		forSaleID := uuid.New()
		_, err := pool.Exec(ctx, `
			INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, negotiation_enabled, status, quantity_available, created_at, updated_at)
			VALUES ($1, $2, $3, 100000, false, 'draft', 1, NOW(), NOW())
		`, forSaleID, productID, sellerID)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `UPDATE for_sales SET status = 'sold', sold_at = NOW(), quantity_available = 0 WHERE id = $1`, forSaleID)
		require.NoError(t, err)

		var surface string
		require.NoError(t, pool.QueryRow(ctx, `SELECT selling_surface FROM products WHERE id = $1`, productID).Scan(&surface))
		require.Equal(t, "for_sale", surface, "selling_surface must remain after sold")

		err = insertAuction(productID, "active")
		assertCheckViolation(t, err, "Auction after ForSale sold")
	})

	// D: ended Auction does not free Product for ForSale
	t.Run("D: ended Auction → ForSale still rejected", func(t *testing.T) {
		productID := seedProduct("auction")
		auctionID := uuid.New()
		_, err := pool.Exec(ctx, `
			INSERT INTO auctions (id, seller_id, product_id, start_price, bid_increment, buy_now_price, start_at, end_at, status, created_at, updated_at)
			VALUES ($1, $2, $3, 100000, 10000, NULL, NOW(), NOW() + INTERVAL '24 hours', 'draft', NOW(), NOW())
		`, auctionID, sellerID, productID)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `UPDATE auctions SET status = 'ended', updated_at = NOW() WHERE id = $1`, auctionID)
		require.NoError(t, err)

		var surface string
		require.NoError(t, pool.QueryRow(ctx, `SELECT selling_surface FROM products WHERE id = $1`, productID).Scan(&surface))
		require.Equal(t, "auction", surface, "selling_surface must remain after ended")

		err = insertForSale(productID, "active")
		assertCheckViolation(t, err, "ForSale after Auction ended")
	})

	// E: selling_surface revert to NULL rejected
	t.Run("E: for_sale → NULL rejected", func(t *testing.T) {
		productID := seedProduct("for_sale")
		_, err := pool.Exec(ctx, `UPDATE products SET selling_surface = NULL WHERE id = $1`, productID)
		assertCheckViolation(t, err, "for_sale → NULL")
	})

	t.Run("E2: auction → NULL rejected", func(t *testing.T) {
		productID := seedProduct("auction")
		_, err := pool.Exec(ctx, `UPDATE products SET selling_surface = NULL WHERE id = $1`, productID)
		assertCheckViolation(t, err, "auction → NULL")
	})

	// F: selling_surface type change rejected
	t.Run("F: for_sale → auction rejected", func(t *testing.T) {
		productID := seedProduct("for_sale")
		_, err := pool.Exec(ctx, `UPDATE products SET selling_surface = 'auction' WHERE id = $1`, productID)
		assertCheckViolation(t, err, "for_sale → auction")
	})

	t.Run("F2: auction → for_sale rejected", func(t *testing.T) {
		productID := seedProduct("auction")
		_, err := pool.Exec(ctx, `UPDATE products SET selling_surface = 'for_sale' WHERE id = $1`, productID)
		assertCheckViolation(t, err, "auction → for_sale")
	})

	// G: NULL → for_sale allowed (normal claiming path)
	t.Run("G: NULL → for_sale allowed", func(t *testing.T) {
		productID := seedProduct("")
		_, err := pool.Exec(ctx, `UPDATE products SET selling_surface = 'for_sale' WHERE id = $1`, productID)
		require.NoError(t, err, "NULL → for_sale should succeed")
	})

	// H: NULL → auction allowed
	t.Run("H: NULL → auction allowed", func(t *testing.T) {
		productID := seedProduct("")
		_, err := pool.Exec(ctx, `UPDATE products SET selling_surface = 'auction' WHERE id = $1`, productID)
		require.NoError(t, err, "NULL → auction should succeed")
	})

	// I: same-type update (for_sale → for_sale) allowed (idempotent)
	t.Run("I: for_sale → for_sale allowed (idempotent)", func(t *testing.T) {
		productID := seedProduct("for_sale")
		_, err := pool.Exec(ctx, `UPDATE products SET selling_surface = 'for_sale' WHERE id = $1`, productID)
		require.NoError(t, err, "for_sale → for_sale should be allowed (idempotent)")
	})
}
