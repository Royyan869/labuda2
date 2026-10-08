//go:build integration

// PASS_21B regression test: fixed-price sale promotion operability reads
// from for_sales (+ products for the derived-visibility check), not
// the legacy `listings` table. Before PASS_21B this read the dead table —
// every real for_sale reported "for_sale_not_found", meaning
// no seller could ever successfully promote a real fixed-price sale.
package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

func TestCheckOperability_ForSale_RealRowIsOperable(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	sellerID := uuid.New()
	productID := uuid.New()
	forSaleID := uuid.New()

	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		if _, err := tx.Exec(ctx,
			`INSERT INTO users (id, firebase_uid, email) VALUES ($1, $2, $3)`,
			sellerID, "fb-"+sellerID.String(), sellerID.String()+"@operability.test",
		); err != nil {
			return err
		}

		now := time.Now()
		if _, err := tx.Exec(ctx, `
			INSERT INTO seller_subscriptions (
				id, user_id, status, started_at, expires_at,
				duration_days, amount_paid, payment_id
			) VALUES ($1, $2, 'active', $3, $4, 365, 0, $5)
		`, uuid.New(), sellerID, now, now.Add(365*24*time.Hour), uuid.New()); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, productID, sellerID, "Sanke Koi", "A fine sanke", `["https://cdn.example.com/sanke.jpg"]`, "sanke", "1_3_days"); err != nil {
			return err
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, status, published_at, quantity_available)
			VALUES ($1, $2, $3, $4, 'active', NOW(), $5)
		`, forSaleID, productID, sellerID, int64(200000), 1)
		return err
	}))

	checker := NewOperabilityCheckerImpl(db.NewFromPool(tdb.Pool()))

	operable, reason, err := checker.CheckOperability(ctx, entity.TargetTypeForSale, &forSaleID)
	require.NoError(t, err)
	require.True(t, operable, "expected a real active for_sale row to be operable, got reason=%q", reason)
	require.Empty(t, reason)

	require.NoError(t, checker.ValidateOwnership(ctx, sellerID, entity.TargetTypeForSale, &forSaleID))
}

// SCOPE 2 (E): the canonical Auction eligibility authority treats scheduled
// (not-yet-started) and active auctions as promotable, and ended auctions as
// ineligible. Promotion consumes this verdict; it owns no auction lifecycle.
func TestCheckOperability_Auction_ScheduledAndActiveOperable(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	sellerID := uuid.New()
	scheduledID := uuid.New()
	activeID := uuid.New()
	endedID := uuid.New()

	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		if _, err := tx.Exec(ctx,
			`INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at, role)
			 VALUES ($1, $2, $3, 'active', NOW(), NOW(), 'user')`,
			sellerID, "fb-"+sellerID.String()[:8], sellerID.String()+"@auction.test"); err != nil {
			return err
		}
		now := time.Now()
		if _, err := tx.Exec(ctx, `
			INSERT INTO seller_subscriptions (id, user_id, status, started_at, expires_at, duration_days, amount_paid, payment_id)
			VALUES ($1, $2, 'active', $3, $4, 365, 0, $5)
		`, uuid.New(), sellerID, now, now.Add(365*24*time.Hour), uuid.New()); err != nil {
			return err
		}
		insertAuction := func(auctionID uuid.UUID, status string) error {
			productID := uuid.New()
			if _, err := tx.Exec(ctx, `
				INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`, productID, sellerID, "Koi", "d", `["u"]`, "koi", "1_3_days"); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `
				INSERT INTO auctions (id, seller_id, product_id, start_price, bid_increment, start_at, end_at, status, created_at, updated_at)
				VALUES ($1, $2, $3, 100000, 10000, NOW(), NOW() + INTERVAL '2 days', $4, NOW(), NOW())
			`, auctionID, sellerID, productID, status)
			return err
		}
		if err := insertAuction(scheduledID, "scheduled"); err != nil {
			return err
		}
		if err := insertAuction(activeID, "active"); err != nil {
			return err
		}
		return insertAuction(endedID, "ended")
	}))

	checker := NewOperabilityCheckerImpl(db.NewFromPool(tdb.Pool()))

	op, reason, err := checker.CheckOperability(ctx, entity.TargetTypeAuction, &scheduledID)
	require.NoError(t, err)
	require.True(t, op, "scheduled auction must be promotable, reason=%q", reason)

	op, reason, err = checker.CheckOperability(ctx, entity.TargetTypeAuction, &activeID)
	require.NoError(t, err)
	require.True(t, op, "active auction must be promotable, reason=%q", reason)

	op, _, err = checker.CheckOperability(ctx, entity.TargetTypeAuction, &endedID)
	require.NoError(t, err)
	require.False(t, op, "ended auction must not be promotable")
}
