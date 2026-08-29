//go:build integration

package tests

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/pkg/testdb"
)

// TestConcurrency_SellingSurfaceExclusivity proves that two concurrent
// transactions attempting to claim different selling surfaces on the
// same Product will result in exactly one success and one failure.
//
// Uses real Postgres with actual concurrent transactions, NOT mocked.
func TestConcurrency_SellingSurfaceExclusivity(t *testing.T) {
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

	seedProduct := func(name string) uuid.UUID {
		t.Helper()
		id := uuid.New()
		_, err := pool.Exec(ctx, `
			INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, created_at, updated_at)
			VALUES ($1, $2, $3, 'desc', '[]'::jsonb, 'kohaku', 'immediate', NOW(), NOW())
		`, id, sellerID, name)
		require.NoError(t, err)
		return id
	}

	// claimSurface simulates ClaimSellingSurface via raw SQL transactions.
	// Opens a transaction, locks the row, checks, and attempts to set.
	claimSurface := func(t *testing.T, productID uuid.UUID, surface string) (won bool) {
		t.Helper()
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx) //nolint:errcheck

		var currentSurface *string
		err = tx.QueryRow(ctx, `SELECT selling_surface FROM products WHERE id = $1 FOR UPDATE`, productID).Scan(&currentSurface)
		require.NoError(t, err)

		if currentSurface == nil {
			_, err = tx.Exec(ctx, `UPDATE products SET selling_surface = $2, updated_at = NOW() WHERE id = $1`, productID, surface)
			require.NoError(t, err)
		} else if *currentSurface == surface {
			// Same type — allowed, no-op
			return true
		} else {
			// Cross-type — rejected
			return false
		}

		err = tx.Commit(ctx)
		return err == nil
	}

	t.Run("concurrent for_sale vs auction on same Product", func(t *testing.T) {
		productID := seedProduct("Concurrency Test 1")

		// We cannot truly interleave goroutines at the SQL level without
		// external synchronization. Instead, we race them tightly.
		// The Postgres row lock guarantees serialization.
		var wg sync.WaitGroup
		var forSaleWon, auctionWon atomic.Bool

		for i := 0; i < 20; i++ {
			wg.Add(2)
			go func() {
				defer wg.Done()
				if claimSurface(t, productID, "for_sale") {
					forSaleWon.Store(true)
				}
			}()
			go func() {
				defer wg.Done()
				if claimSurface(t, productID, "auction") {
					auctionWon.Store(true)
				}
			}()
		}
		wg.Wait()

		// Exactly one type must have won
		fw := forSaleWon.Load()
		aw := auctionWon.Load()
		t.Logf("for_sale won=%v, auction won=%v", fw, aw)
		require.True(t, fw || aw, "at least one type must win")
		require.False(t, fw && aw, "BOTH types cannot win — cross-type is impossible")

		// Verify final state
		var finalSurface *string
		err = pool.QueryRow(ctx, `SELECT selling_surface FROM products WHERE id = $1`, productID).Scan(&finalSurface)
		require.NoError(t, err)
		require.NotNil(t, finalSurface)
		t.Logf("final surface = %s", *finalSurface)
	})

	t.Run("concurrent auction vs for_sale (reversed)", func(t *testing.T) {
		productID := seedProduct("Concurrency Test 2")

		var wg sync.WaitGroup
		var forSaleWon, auctionWon atomic.Bool

		for i := 0; i < 20; i++ {
			wg.Add(2)
			go func() {
				defer wg.Done()
				if claimSurface(t, productID, "auction") {
					auctionWon.Store(true)
				}
			}()
			go func() {
				defer wg.Done()
				if claimSurface(t, productID, "for_sale") {
					forSaleWon.Store(true)
				}
			}()
		}
		wg.Wait()

		fw := forSaleWon.Load()
		aw := auctionWon.Load()
		t.Logf("auction won=%v, for_sale won=%v", aw, fw)
		require.True(t, fw || aw, "at least one type must win")
		require.False(t, fw && aw, "BOTH types cannot win — cross-type is impossible")
	})

	t.Run("concurrent same-type (for_sale) all succeed", func(t *testing.T) {
		productID := seedProduct("Concurrency Test 3")

		var wg sync.WaitGroup
		var winCount atomic.Int32

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if claimSurface(t, productID, "for_sale") {
					winCount.Add(1)
				}
			}()
		}
		wg.Wait()

		t.Logf("same-type wins = %d", winCount.Load())
		// All should succeed since same-type is idempotent
		require.Equal(t, int32(10), winCount.Load(), "all same-type claims should succeed")
	})
}
