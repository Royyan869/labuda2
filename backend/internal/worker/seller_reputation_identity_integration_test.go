//go:build integration

package worker

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	sellerRepo "github.com/hishumi/backend/internal/commerce/seller/infrastructure/repository"
	outboxRepo "github.com/hishumi/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/hishumi/backend/pkg/db"
	"github.com/hishumi/backend/pkg/testdb"
)

// TestReputationIdentity_ProfileKeyIsNotCommerceKey is the real-DB regression
// that locks the canonical identity lineage. It proves that reputation
// aggregation and seller_reputation_state use the canonical commerce seller
// identity (users.id = seller_profiles.user_id), never the seller_profiles
// surrogate primary key.
//
// The fixture deliberately creates seller_profiles.id != seller_profiles.user_id
// and writes orders.seller_id = user_id. It runs the REAL productionAggregator
// (no mock aggregator) through the real recompute path and asserts the rolling
// metrics land on the user-keyed state row and drive the canonical tier badge.
//
// This is the regression the previous mock-only unit tests could not catch:
// they stubbed the aggregator, so the profile-vs-user key mismatch was
// invisible.
func TestReputationIdentity_ProfileKeyIsNotCommerceKey(t *testing.T) {
	ctx := context.Background()

	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	appDB := db.NewFromPool(tdb.Pool())
	sellerRepository := sellerRepo.NewSellerRepository()
	outboxRepository := outboxRepo.NewOutboxRepository(appDB)

	buyerID := uuid.New()
	sellerUserID := uuid.New() // canonical commerce seller identity (users.id)
	profileID := uuid.New()    // seller_profiles surrogate key — must differ
	require.NotEqual(t, profileID, sellerUserID,
		"fixture must exercise distinct seller_profiles.id and users.id")

	now := time.Now().UTC()
	withinWindow := now.Add(-10 * 24 * time.Hour) // inside the rolling 90-day window

	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO users (
				id, firebase_uid, email, email_verified_at, phone_verified,
				account_status, created_at, updated_at
			)
			VALUES
				($1, $2, $3, NOW(), true, 'active', NOW(), NOW()),
				($4, $5, $6, NOW(), true, 'active', NOW(), NOW())
		`,
			buyerID, buyerID.String(), buyerID.String()+"@buyer.test.invalid",
			sellerUserID, sellerUserID.String(), sellerUserID.String()+"@seller.test.invalid",
		); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO seller_profiles (id, user_id, store_name, tier, status, created_at, updated_at)
			VALUES ($1, $2, 'Reputation Identity Store', 'basic', 'active', NOW(), NOW())
		`, profileID, sellerUserID); err != nil {
			return err
		}

		// 100 completed orders keyed by the canonical seller identity (users.id).
		if _, err := tx.Exec(ctx, `
			INSERT INTO orders (
				id, buyer_id, seller_id, source_type, source_id,
				quantity, unit_price, subtotal, shipping_total,
				commission_percent, commission_amount, status,
				completed_at, created_at, updated_at
			)
			SELECT gen_random_uuid(), $1, $2, 'for_sale', gen_random_uuid(),
			       1, 100000, 100000, 0, 0, 0, 'completed',
			       $3, $3, $3
			FROM generate_series(1, 100)
		`, buyerID, sellerUserID, withinWindow); err != nil {
			return err
		}

		// 3 shipping-timeout orders (cancelled_timeout), keyed by users.id.
		if _, err := tx.Exec(ctx, `
			INSERT INTO orders (
				id, buyer_id, seller_id, source_type, source_id,
				quantity, unit_price, subtotal, shipping_total,
				commission_percent, commission_amount, status,
				created_at, updated_at
			)
			SELECT gen_random_uuid(), $1, $2, 'for_sale', gen_random_uuid(),
			       1, 100000, 100000, 0, 0, 0, 'cancelled_timeout',
			       $3, $3
			FROM generate_series(1, 3)
		`, buyerID, sellerUserID, withinWindow); err != nil {
			return err
		}

		// 15 five-star ratings on distinct completed orders (Pro needs >= 15).
		if _, err := tx.Exec(ctx, `
			INSERT INTO order_ratings (id, order_id, buyer_id, seller_id, rating_value, created_at)
			SELECT gen_random_uuid(), o.id, o.buyer_id, o.seller_id, 5, $2
			FROM orders o
			WHERE o.seller_id = $1 AND o.status = 'completed'
			LIMIT 15
		`, sellerUserID, withinWindow); err != nil {
			return err
		}

		// 2 admin-decided refund losses (seller at fault), keyed by users.id.
		// Proves the refunds aggregation also uses the canonical user id.
		if _, err := tx.Exec(ctx, `
			INSERT INTO refunds (
				id, order_id, buyer_id, seller_id, reason, status,
				requested_amount, admin_reviewed_at, created_at, updated_at
			)
			SELECT gen_random_uuid(), o.id, o.buyer_id, o.seller_id,
			       'item_not_received', 'admin_refunded',
			       100000, $2, $2, $2
			FROM orders o
			WHERE o.seller_id = $1 AND o.status = 'completed'
			LIMIT 2
		`, sellerUserID, withinWindow); err != nil {
			return err
		}

		return nil
	}))

	// Run the REAL recompute path (productionAggregator + real repositories),
	// no mock aggregator.
	w := NewSellerReputationRecomputeWorker(
		appDB, sellerRepository, outboxRepository, zaptest.NewLogger(t),
	)
	w.RecomputeAllSellers(ctx)

	// POSITIVE: state row is keyed by users.id and carries the real counts.
	var (
		completed   int
		cancelled   int
		ratingCount int
		ratingAvg   float64
		disputeLoss int
		currentTier string
	)
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT
				rolling_completed_orders,
				rolling_cancelled_timeout,
				rolling_rating_count,
				rolling_rating_average,
				rolling_dispute_loss_count,
				current_tier
			FROM seller_reputation_state
			WHERE seller_id = $1
		`, sellerUserID).Scan(
			&completed, &cancelled, &ratingCount, &ratingAvg, &disputeLoss, &currentTier,
		)
	}))

	require.Equal(t, 100, completed,
		"rolling_completed_orders must count orders keyed by users.id")
	require.Equal(t, 3, cancelled,
		"rolling_cancelled_timeout must count orders keyed by users.id")
	require.Equal(t, 15, ratingCount,
		"rolling_rating_count must count order_ratings keyed by users.id")
	require.InDelta(t, 5.0, ratingAvg, 0.001,
		"rolling_rating_average must aggregate order_ratings keyed by users.id")
	require.Equal(t, 2, disputeLoss,
		"rolling_dispute_loss_count must count refunds keyed by users.id")
	require.Equal(t, "pro", currentTier,
		"seller meeting the Pro threshold must evaluate to pro")

	// Canonical tier badge (seller_profiles.tier) reflects the evaluation.
	var badgeTier string
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT tier FROM seller_profiles WHERE id = $1`, profileID,
		).Scan(&badgeTier)
	}))
	require.Equal(t, "pro", badgeTier,
		"seller_profiles.tier is the canonical current tier and must be updated")

	// NEGATIVE: no reputation-state row may exist under the surrogate key.
	var profileKeyed int
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT COUNT(*) FROM seller_reputation_state WHERE seller_id = $1`, profileID,
		).Scan(&profileKeyed)
	}))
	require.Zero(t, profileKeyed,
		"reputation state must never be keyed by seller_profiles.id")
}
