//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Fixtures ---

func insertUser(t *testing.T, ctx context.Context, tx db.Tx, id uuid.UUID, username string, accountStatus string, deletedAt *time.Time, avatarURL string) {
	t.Helper()
	now := time.Now().UTC()
	uniqueUsername := username + "-" + id.String()[:12]
	var avatar *string
	if avatarURL != "" {
		avatar = &avatarURL
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, account_status, deleted_at, created_at, updated_at, role)
		VALUES ($1, $2, $3, $4, $5, $6, $6, 'user')
	`, id, "fb-"+id.String(), id.String()+"@test.local", accountStatus, deletedAt, now)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `
		INSERT INTO user_profiles (id, user_id, username, avatar_url, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $5)
	`, uuid.New(), id, uniqueUsername, avatar, now)
	require.NoError(t, err)
}

func insertOrder(t *testing.T, ctx context.Context, tx db.Tx, orderID, buyerID, sellerID uuid.UUID) {
	t.Helper()
	_, err := tx.Exec(ctx, `
		INSERT INTO orders (id, buyer_id, seller_id, source_type, source_id, quantity, unit_price, subtotal, shipping_total, commission_percent, commission_amount, escrow_amount, refunded_amount, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'for_sale', $4, 1, 100000, 100000, 0, 0, 0, 100000, 0, 'completed', NOW(), NOW())
	`, orderID, buyerID, sellerID, uuid.New())
	require.NoError(t, err)
}

func insertRating(t *testing.T, ctx context.Context, tx db.Tx, id, orderID, buyerID, sellerID uuid.UUID, ratingValue int, createdAt time.Time) {
	t.Helper()
	_, err := tx.Exec(ctx, `
		INSERT INTO order_ratings (id, order_id, buyer_id, seller_id, rating_value, comment, created_at, invalidated_at)
		VALUES ($1, $2, $3, $4, $5, 'test', $6, NULL)
	`, id, orderID, buyerID, sellerID, ratingValue, createdAt)
	require.NoError(t, err)
}

// --- Pagination Tests ---

func TestListBySeller_FirstPage(t *testing.T) {
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	repo := NewOrderRatingRepository()

	sellerID := uuid.New()
	buyerIDs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	ratingIDs := make([]uuid.UUID, 5)
	base := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		insertUser(t, ctx, tx, sellerID, "seller", "active", nil, "")
		for _, b := range buyerIDs {
			insertUser(t, ctx, tx, b, "buyer_"+b.String()[:8], "active", nil, "")
		}
		for i := 0; i < 5; i++ {
			ratingIDs[i] = uuid.New()
			oid := uuid.New()
			insertOrder(t, ctx, tx, oid, buyerIDs[i], sellerID)
			insertRating(t, ctx, tx, ratingIDs[i], oid, buyerIDs[i], sellerID, 4+i%2, base.Add(-time.Duration(i)*time.Hour))
		}
		return nil
	})
	require.NoError(t, err)

	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		ratings, err := repo.ListBySeller(ctx, tx, sellerID, 2, 0)
		require.NoError(t, err)
		assert.Len(t, ratings, 2)
		assert.Equal(t, ratingIDs[0], ratings[0].ID, "newest first")
		assert.Equal(t, ratingIDs[1], ratings[1].ID, "second newest")
		return nil
	})
	require.NoError(t, err)
}

func TestListBySeller_SecondPage(t *testing.T) {
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	repo := NewOrderRatingRepository()

	sellerID := uuid.New()
	buyerIDs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	ratingIDs := make([]uuid.UUID, 4)
	base := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)

	var cursorTS time.Time

	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		insertUser(t, ctx, tx, sellerID, "seller2", "active", nil, "")
		for _, b := range buyerIDs {
			insertUser(t, ctx, tx, b, "buyer_"+b.String()[:8], "active", nil, "")
		}
		for i := 0; i < 4; i++ {
			ratingIDs[i] = uuid.New()
			oid := uuid.New()
			insertOrder(t, ctx, tx, oid, buyerIDs[i], sellerID)
			insertRating(t, ctx, tx, ratingIDs[i], oid, buyerIDs[i], sellerID, 4, base.Add(-time.Duration(i)*time.Hour))
		}
		return nil
	})
	require.NoError(t, err)

	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		ratings, err := repo.ListBySeller(ctx, tx, sellerID, 2, 0)
		require.NoError(t, err)
		assert.Len(t, ratings, 2)
		assert.Equal(t, ratingIDs[0], ratings[0].ID)
		assert.Equal(t, ratingIDs[1], ratings[1].ID)
		cursorTS = ratings[1].CreatedAt
		return nil
	})
	require.NoError(t, err)

	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		ratings, err := repo.ListBySeller(ctx, tx, sellerID, 2, cursorTS.UnixNano())
		require.NoError(t, err)
		assert.Len(t, ratings, 2)
		assert.Equal(t, ratingIDs[2], ratings[0].ID)
		assert.Equal(t, ratingIDs[3], ratings[1].ID)
		return nil
	})
	require.NoError(t, err)
}

func TestListBySeller_InvalidatedRatingExcluded(t *testing.T) {
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	repo := NewOrderRatingRepository()

	sellerID := uuid.New()
	b1 := uuid.New()
	b2 := uuid.New()
	base := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	var validID uuid.UUID

	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		insertUser(t, ctx, tx, sellerID, "seller_inv", "active", nil, "")
		insertUser(t, ctx, tx, b1, "buyer1", "active", nil, "")
		insertUser(t, ctx, tx, b2, "buyer2", "active", nil, "")

		validID = uuid.New()
		oid1 := uuid.New()
		oid2 := uuid.New()
		insertOrder(t, ctx, tx, oid1, b1, sellerID)
		insertOrder(t, ctx, tx, oid2, b2, sellerID)
		insertRating(t, ctx, tx, validID, oid1, b1, sellerID, 5, base)
		// Insert invalidated rating.
		_, err := tx.Exec(ctx, `
			INSERT INTO order_ratings (id, order_id, buyer_id, seller_id, rating_value, comment, created_at, invalidated_at)
			VALUES ($1, $2, $3, $4, 5, 'test', $5, NOW())
		`, uuid.New(), oid2, b2, sellerID, base.Add(-time.Hour))
		require.NoError(t, err)
		return nil
	})
	require.NoError(t, err)

	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		ratings, err := repo.ListBySeller(ctx, tx, sellerID, 10, 0)
		require.NoError(t, err)
		assert.Len(t, ratings, 1, "only valid rating")
		assert.Equal(t, validID, ratings[0].ID)
		return nil
	})
	require.NoError(t, err)
}

func TestListBySeller_EmptyResult(t *testing.T) {
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	repo := NewOrderRatingRepository()

	sellerID := uuid.New()
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		insertUser(t, ctx, tx, sellerID, "seller_empty", "active", nil, "")
		return nil
	})
	require.NoError(t, err)

	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		ratings, err := repo.ListBySeller(ctx, tx, sellerID, 10, 0)
		require.NoError(t, err)
		assert.Empty(t, ratings)
		return nil
	})
	require.NoError(t, err)
}

func TestListByBuyer_BuyerFilterCorrect(t *testing.T) {
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	repo := NewOrderRatingRepository()

	seller := uuid.New()
	buyerA := uuid.New()
	buyerB := uuid.New()
	base := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	var ratingA uuid.UUID

	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		insertUser(t, ctx, tx, seller, "seller_bf", "active", nil, "")
		insertUser(t, ctx, tx, buyerA, "buyer_a", "active", nil, "")
		insertUser(t, ctx, tx, buyerB, "buyer_b", "active", nil, "")

		ratingA = uuid.New()
		oidA := uuid.New()
		oidB := uuid.New()
		insertOrder(t, ctx, tx, oidA, buyerA, seller)
		insertOrder(t, ctx, tx, oidB, buyerB, seller)
		insertRating(t, ctx, tx, ratingA, oidA, buyerA, seller, 5, base)
		insertRating(t, ctx, tx, uuid.New(), oidB, buyerB, seller, 4, base.Add(-time.Hour))
		return nil
	})
	require.NoError(t, err)

	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		ratings, err := repo.ListByBuyer(ctx, tx, buyerA, 10, 0)
		require.NoError(t, err)
		assert.Len(t, ratings, 1)
		assert.Equal(t, ratingA, ratings[0].ID)
		assert.Equal(t, buyerA, ratings[0].BuyerID)
		return nil
	})
	require.NoError(t, err)
}

func TestListBySeller_TerminalPage(t *testing.T) {
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	repo := NewOrderRatingRepository()

	sellerID := uuid.New()
	buyerIDs := []uuid.UUID{uuid.New(), uuid.New()}
	base := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)

	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		insertUser(t, ctx, tx, sellerID, "seller_term", "active", nil, "")
		for idx, b := range buyerIDs {
			insertUser(t, ctx, tx, b, "buyer_"+b.String()[:8], "active", nil, "")
			oid := uuid.New()
			insertOrder(t, ctx, tx, oid, b, sellerID)
			insertRating(t, ctx, tx, uuid.New(), oid, b, sellerID, 5, base.Add(-time.Duration(idx)*time.Hour))
		}
		return nil
	})
	require.NoError(t, err)

	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		ratings, err := repo.ListBySeller(ctx, tx, sellerID, 10, 0)
		require.NoError(t, err)
		assert.Len(t, ratings, 2)
		return nil
	})
	require.NoError(t, err)
}
