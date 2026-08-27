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
		`, productID, sellerID, "Sanke Koi", "A fine sanke", `["https://cdn.example.com/sanke.jpg"]`, "sanke", "immediate"); err != nil {
			return err
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, status, published_at, quantity_available)
			VALUES ($1, $2, $3, $4, 'active', NOW(), $5)
		`, forSaleID, productID, sellerID, int64(200000), 1)
		return err
	}))

	checker := NewOperabilityCheckerImpl(db.NewFromPool(tdb.Pool()), nil)

	operable, reason, err := checker.CheckOperability(ctx, entity.TargetTypeForSale, &forSaleID)
	require.NoError(t, err)
	require.True(t, operable, "expected a real active for_sale row to be operable, got reason=%q", reason)
	require.Empty(t, reason)

	require.NoError(t, checker.ValidateOwnership(ctx, sellerID, entity.TargetTypeForSale, &forSaleID))
}
