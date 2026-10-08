//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/internal/commerce/productview/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

func seedPVUser(t *testing.T, ctx context.Context, tdb *testdb.TestDB) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, email_verified_at, account_status, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), 'active', NOW(), NOW())
	`, id, "fb-"+id.String(), id.String()+"@test.invalid")
	require.NoError(t, err)
	return id
}

func seedPVProduct(t *testing.T, ctx context.Context, tdb *testdb.TestDB) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, selling_surface, created_at, updated_at)
		VALUES ($1, $2, 'Koi', 'desc', '[]', 'kohaku', '1_3_days', 'for_sale', NOW(), NOW())
	`, id, seedPVUser(t, ctx, tdb))
	require.NoError(t, err)
	return id
}

func (r *ProductViewRepositoryImpl) countDirect(t *testing.T, tdb *testdb.TestDB, productID uuid.UUID) int64 {
	t.Helper()
	var n int64
	require.NoError(t, tdb.WithTx(context.Background(), func(tx db.Tx) error {
		var err error
		n, err = r.CountByProduct(context.Background(), tx, productID)
		return err
	}))
	return n
}

// TestProductViewRepository_AnonymousThenAuthenticated verifies that Product
// View is an append-only event log: anonymous views persist with a NULL viewer,
// authenticated views persist with the viewer id, and every open adds a row
// (no dedup).
func TestProductViewRepository_AnonymousThenAuthenticated(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	repo := NewProductViewRepository()
	productID := seedPVProduct(t, ctx, tdb)
	buyer := seedPVUser(t, ctx, tdb)

	// 1 anonymous view.
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		return repo.Record(ctx, tx, &entity.ProductViewEvent{ProductID: productID, ViewerUserID: nil})
	}))
	// 2 authenticated views by the same viewer (repeated viewing = multiple rows).
	for i := 0; i < 2; i++ {
		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			return repo.Record(ctx, tx, &entity.ProductViewEvent{ProductID: productID, ViewerUserID: &buyer})
		}))
	}

	require.Equal(t, int64(3), repo.countDirect(t, tdb, productID))

	// The anonymous row carries a NULL viewer; the two authenticated rows carry
	// the buyer id.
	var anonymous, authenticated int64
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM product_view_events WHERE product_id = $1 AND viewer_user_id IS NULL`, productID).Scan(&anonymous); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `SELECT COUNT(*) FROM product_view_events WHERE product_id = $1 AND viewer_user_id = $2`, productID, buyer).Scan(&authenticated)
	}))
	require.Equal(t, int64(1), anonymous)
	require.Equal(t, int64(2), authenticated)
}

// TestProductViewRepository_CountIsPerProduct verifies the aggregate read path
// that a future Seller Analytics consumer uses: COUNT GROUP BY product_id.
func TestProductViewRepository_CountIsPerProduct(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	repo := NewProductViewRepository()
	productA := seedPVProduct(t, ctx, tdb)
	productB := seedPVProduct(t, ctx, tdb)

	for i := 0; i < 3; i++ {
		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			return repo.Record(ctx, tx, &entity.ProductViewEvent{ProductID: productA})
		}))
	}
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		return repo.Record(ctx, tx, &entity.ProductViewEvent{ProductID: productB})
	}))

	require.Equal(t, int64(3), repo.countDirect(t, tdb, productA))
	require.Equal(t, int64(1), repo.countDirect(t, tdb, productB))
}
