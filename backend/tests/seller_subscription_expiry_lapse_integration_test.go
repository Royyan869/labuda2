//go:build integration

package tests

// Stage 7 — seller market-authority lapse (owner decisions, Oct 2026).
//
// Real Postgres runtime proof that:
//  1. the seller.subscription.expired handler LAPSES scheduled auctions
//     (status 'lapsed') — it never writes 'cancelled';
//  2. running auctions and ForSale surfaces are deliberately untouched:
//     they run to completion / stay active (no draft demotion, no
//     bidless-cancel);
//  3. the expired seller's ACTIVE inventory disappears from viewer surfaces
//     (public auction browse, public FPS catalog, public seller page) while
//     the seller's own inventory view still shows every row;
//  4. a control seller with live market authority is unaffected — the hide
//     is selective, not a broken query.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	auctioninfra "github.com/labuda/backend/internal/commerce/auction/infrastructure/repository"
	fpsinfra "github.com/labuda/backend/internal/commerce/forsale/infrastructure/repository"
	platformevent "github.com/labuda/backend/internal/platform/event"
	"github.com/labuda/backend/internal/worker"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

// expireSellerSubscription flips a fixture seller's (stage6b seeds an active
// one) into the lapsed state the expiry worker would leave behind.
func expireSellerSubscription(t *testing.T, ctx context.Context, tdb *testdb.TestDB, userID uuid.UUID) {
	t.Helper()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE seller_subscriptions
			SET status = 'expired', expires_at = NOW() - INTERVAL '1 hour', updated_at = NOW()
			WHERE user_id = $1
		`, userID)
		return err
	}))
}

func auctionStatusOf(t *testing.T, ctx context.Context, tdb *testdb.TestDB, auctionID uuid.UUID) string {
	t.Helper()
	var status string
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT status FROM auctions WHERE id = $1`, auctionID).Scan(&status)
	}))
	return status
}

func forSaleStatusOf(t *testing.T, ctx context.Context, tdb *testdb.TestDB, saleID uuid.UUID) string {
	t.Helper()
	var status string
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT status FROM for_sales WHERE id = $1`, saleID).Scan(&status)
	}))
	return status
}

func TestSellerSubscriptionExpiry_LapsesScheduledHidesInventory_PreservesRunningAndForSale(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	// Fixtures: an expiring seller (stage6b seeds an active subscription,
	// flipped here) and a control seller with live authority.
	expiredSeller := seedStage6BUser(t, ctx, tdb)
	expireSellerSubscription(t, ctx, tdb, expiredSeller)
	controlSeller := seedStage6BUser(t, ctx, tdb)

	scheduledID := seedStage6BAuction(t, ctx, tdb, expiredSeller, "scheduled")
	runningID := seedStage6BAuction(t, ctx, tdb, expiredSeller, "active")
	controlAuctionID := seedStage6BAuction(t, ctx, tdb, controlSeller, "scheduled")

	fpsProduct := seedStage6BProduct(t, ctx, tdb, expiredSeller)
	expiredFPS := seedStage6BFPS(t, ctx, tdb, fpsProduct, expiredSeller, "active", 5, true)
	controlFPSProduct := seedStage6BProduct(t, ctx, tdb, controlSeller)
	controlFPS := seedStage6BFPS(t, ctx, tdb, controlFPSProduct, controlSeller, "active", 5, true)

	// 1+2. Run the REAL outbox handler on the expiry event.
	payload, err := json.Marshal(map[string]string{"user_id": expiredSeller.String()})
	require.NoError(t, err)
	h := worker.NewSellerSubscriptionExpiredHandler(db.NewFromPool(tdb.Pool()), nil)
	require.NoError(t, h.Handle(ctx, platformevent.OutboxEvent{
		ID:      uuid.New(),
		Payload: payload,
	}))

	// 1. scheduled → lapsed (never cancelled).
	require.Equal(t, "lapsed", auctionStatusOf(t, ctx, tdb, scheduledID),
		"scheduled auction must LAPSE, not cancel")
	// 2. Running auction untouched — it finishes on its own schedule.
	require.Equal(t, "active", auctionStatusOf(t, ctx, tdb, runningID),
		"running auction must continue to completion (bidless ends as 'ended', relistable)")
	// 2. ForSale stays active — no active→draft demotion.
	require.Equal(t, "active", forSaleStatusOf(t, ctx, tdb, expiredFPS),
		"forsale must stay active; hiding is read-side, not a status demotion")
	// 2. Control seller untouched.
	require.Equal(t, "scheduled", auctionStatusOf(t, ctx, tdb, controlAuctionID))

	// 3. Read-side hide: the expired seller's ACTIVE auction and FPS vanish
	// from viewer surfaces...
	auctionRepo := auctioninfra.NewAuctionRepository()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		got, err := auctionRepo.List(ctx, tx, auctioninfra.AuctionFilter{Limit: 50})
		if err != nil {
			return err
		}
		for _, a := range got {
			require.NotEqual(t, expiredSeller, a.SellerID,
				"expired seller's auction must be hidden from public browse")
		}
		return nil
	}))

	fpsRepo := fpsinfra.NewForSaleRepository()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		got, err := fpsRepo.GetPublic(ctx, tx, 50, 0)
		if err != nil {
			return err
		}
		gotSellerPage, err := fpsRepo.GetPublicBySellerID(ctx, tx, expiredSeller, 50, 0)
		if err != nil {
			return err
		}
		for _, l := range got {
			require.NotEqual(t, expiredFPS, l.ID, "expired seller's FPS must be hidden from public catalog")
		}
		for _, l := range gotSellerPage {
			require.NotEqual(t, expiredFPS, l.ID, "expired seller's FPS must be hidden from their public seller page")
		}
		// 4. Selectivity: the control seller stays fully visible.
		var controlVisible bool
		for _, l := range got {
			if l.ID == controlFPS {
				controlVisible = true
			}
		}
		require.True(t, controlVisible, "control seller with live authority must stay discoverable")
		return nil
	}))

	// 3b. ...while the owner still sees their own hidden inventory.
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		got, err := auctionRepo.List(ctx, tx, auctioninfra.AuctionFilter{
			SellerID:       &expiredSeller,
			OwnerInventory: true,
			Limit:          50,
		})
		if err != nil {
			return err
		}
		ids := make(map[uuid.UUID]bool, len(got))
		for _, a := range got {
			ids[a.ID] = true
		}
		require.True(t, ids[runningID], "owner must see their own running auction")
		require.True(t, ids[scheduledID], "owner must see their own lapsed auction")
		return nil
	}))
}
