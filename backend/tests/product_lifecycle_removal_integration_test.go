//go:build integration

package tests

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	auctioninfra "github.com/labuda/backend/internal/commerce/auction/infrastructure/repository"
	fpsinfra "github.com/labuda/backend/internal/commerce/forsale/infrastructure/repository"
	fpsRepo "github.com/labuda/backend/internal/commerce/forsale/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

// TestMigration000044_UpDownReplay verifies products.status/products.sold_at /
// idx_products_status / product_status_enum are gone after 000044 (up), that
// the down migration restores them, and that replaying up after down is safe.
func TestMigration000044_UpDownReplay(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t) // applies all up migrations incl. 000044
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()

	columnExists := func(column string) bool {
		var exists bool
		require.NoError(t, pool.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'products' AND column_name = $1
			)
		`, column).Scan(&exists))
		return exists
	}
	typeExists := func() bool {
		var exists bool
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM pg_type WHERE typname = 'product_status_enum')`).Scan(&exists))
		return exists
	}
	indexExists := func() bool {
		var exists bool
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE indexname = 'idx_products_status')`).Scan(&exists))
		return exists
	}

	execSQLFile := func(name string) {
		path := filepath.Join("..", "migrations", name)
		raw, err := os.ReadFile(path)
		require.NoError(t, err, "read %s", path)
		var kept []string
		for _, line := range strings.Split(string(raw), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "--") {
				continue
			}
			kept = append(kept, trimmed)
		}
		var statements []string
		for _, part := range strings.Split(strings.Join(kept, "\n"), ";") {
			if stmt := strings.TrimSpace(part); stmt != "" {
				statements = append(statements, stmt)
			}
		}
		require.NotEmpty(t, statements)
		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			for _, stmt := range statements {
				if _, err := tx.Exec(ctx, stmt); err != nil {
					return err
				}
			}
			return nil
		}), "exec %s", name)
	}

	// Post-up state (SetupDB ran 000044).
	require.False(t, columnExists("status"))
	require.False(t, columnExists("sold_at"))
	require.False(t, indexExists())
	require.False(t, typeExists())

	// Down: columns/index/type restored.
	execSQLFile("000044_product_lifecycle_removal.down.sql")
	require.True(t, columnExists("status"))
	require.True(t, columnExists("sold_at"))
	require.True(t, indexExists())
	require.True(t, typeExists())

	// Replay up: removed again.
	execSQLFile("000044_product_lifecycle_removal.up.sql")
	require.False(t, columnExists("status"))
	require.False(t, columnExists("sold_at"))
	require.False(t, indexExists())
	require.False(t, typeExists())
}

// TestFpsCatalog_SurvivesProductLifecycleRemoval proves the buyer FPS catalog
// and search visibility are driven solely by fps.status='active' (the
// redundant p.status='available' gate is gone) against real Postgres.
func TestFpsCatalog_SurvivesProductLifecycleRemoval(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	sellerID := uuid.New()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO users (id, firebase_uid, email, email_verified_at, account_status, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), 'active', NOW(), NOW())
		`, sellerID, "fb-"+sellerID.String(), sellerID.String()+"@test.invalid")
		return err
	}))

	seedProductAndSale := func(status string) (uuid.UUID, uuid.UUID) {
		productID := uuid.New()
		saleID := uuid.New()
		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			if _, err := tx.Exec(ctx, `
				INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, created_at, updated_at)
				VALUES ($1, $2, 'Kohaku', 'desc', '[]'::jsonb, 'kohaku', 'immediate', NOW(), NOW())
			`, productID, sellerID); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `
				INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, negotiation_enabled, status, quantity_available, created_at, updated_at)
				VALUES ($1, $2, $3, 100000, false, $4, 1, NOW(), NOW())
			`, saleID, productID, sellerID, status)
			return err
		}))
		return productID, saleID
	}

	activeProduct, activeSale := seedProductAndSale("active")
	seedProductAndSale("withdrawn")

	repo := fpsinfra.NewForSaleRepository()

	// GetPublic: only the active FPS is visible — no product-status gate.
	visitedActive := false
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		sales, err := repo.GetPublic(ctx, tx, 20, 0)
		if err != nil {
			return err
		}
		for _, s := range sales {
			if s.ID == activeSale {
				visitedActive = true
			} else {
				return &identityLeakError{id: s.ID.String()}
			}
		}
		return nil
	}))
	require.True(t, visitedActive, "active fixed-price sale must be listed in GetPublic")
	require.True(t, activeProduct != uuid.Nil)

	// Search: same authority — fps.status='active' only.
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		sales, _, err := repo.Search(ctx, tx, fpsRepo.SearchFilters{})
		if err != nil {
			return err
		}
		for _, s := range sales {
			if s.ID == activeSale {
				visitedActive = true
			}
		}
		return nil
	}))
	require.True(t, visitedActive, "active fixed-price sale must be returned by Search")

	// Auction marketplace still exposes an active auction (auction queries never
	// depended on product status). Uses its OWN product — the active FPS above
	// already occupies its product via the single-active-channel invariant.
	auctionProductID := uuid.New()
	auctionID := uuid.New()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, created_at, updated_at)
			VALUES ($1, $2, 'Auction Koi', 'desc', '[]'::jsonb, 'kohaku', 'immediate', NOW(), NOW())
		`, auctionProductID, sellerID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO auctions (id, seller_id, product_id,
				start_price, bid_increment, buy_now_price, start_at, end_at, current_bid, status, created_at, updated_at)
			VALUES ($1, $2, $3, 10000, 1000, NULL, NOW(), NOW() + INTERVAL '24 hours', NULL, 'active', NOW(), NOW())
		`, auctionID, sellerID, auctionProductID)
		return err
	}))
	active := auctionEntity.StatusActive
	auctionRepo := auctioninfra.NewAuctionRepository()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		auctions, err := auctionRepo.List(ctx, tx, auctioninfra.AuctionFilter{Status: &active})
		if err != nil {
			return err
		}
		found := false
		for _, a := range auctions {
			if a.ID == auctionID {
				found = true
			}
		}
		require.True(t, found, "active auction must remain visible in the auction marketplace")
		return nil
	}))
}

// identityLeakError is a sentinel for an unexpected non-active surfaced row.
type identityLeakError struct{ id string }

func (e *identityLeakError) Error() string { return "unexpected row surfaced: " + e.id }
