//go:build integration

package tests

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/pkg/testdb"
)

// TestMigration000047_Rule9_SingleActiveChannelInvariant proves that after
// migration 000047 UP the cross-selling-channel trigger still enforces
// Rule 9 (one active selling channel per product), operating on the
// canonical `for_sales` table:
//
//	1. a for_sale whose product already has an active auction is rejected;
//	2. an auction whose product already has an active for_sale is rejected;
//	3. inserting a for_sale for a product with no competing active surface
//	   succeeds (control case).
func TestMigration000047_Rule9_SingleActiveChannelInvariant(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()

	user := seedRule9User(ctx, t, pool)
	product := seedRule9Product(ctx, t, pool, user)

	// The canonical trigger must exist on for_sales.
	var triggerExists bool
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pg_trigger WHERE NOT tgisinternal AND tgname='trg_for_sales_single_active_channel')`,
	).Scan(&triggerExists))
	require.True(t, triggerExists, "Rule 9 trigger trg_for_sales_single_active_channel must exist on for_sales")

	// Direction 1: product already has an active auction -> for_sale rejected.
	require.NoError(t, insertRule9Auction(ctx, pool, product, user, "active"))
	err := insertRule9ForSale(ctx, pool, product, user, "active")
	requireRule9Error(t, err, "for_sale insert with existing active auction must be rejected")

// Clear the auction so we can test the opposite direction.
	_, err = pool.Exec(ctx, `DELETE FROM auctions WHERE product_id = $1`, product)
	require.NoError(t, err)

	// Direction 2: product now has an active for_sale -> auction rejected.
	require.NoError(t, insertRule9ForSale(ctx, pool, product, user, "active"))
	err = insertRule9Auction(ctx, pool, product, user, "active")
	requireRule9Error(t, err, "auction insert with existing active for_sale must be rejected")

	// Control: a fresh product has no competing active surface -> for_sale allowed.
	cleanProduct := seedRule9Product(ctx, t, pool, user)
	require.NoError(t, insertRule9ForSale(ctx, pool, cleanProduct, user, "active"),
		"for_sale insert on a clean product must succeed")
}

func requireRule9Error(t *testing.T, err error, msg string) {
	t.Helper()
	require.Error(t, err, msg)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("%s: expected a pg error, got %T: %v", msg, err, err)
	}
	require.Equal(t, "23514", pgErr.Code,
		"%s: expected check_violation (23514) from Rule 9 trigger, got %s: %s", msg, pgErr.Code, pgErr.Message)
}

func seedRule9User(ctx context.Context, t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, email_verified_at, account_status, created_at, updated_at)
		VALUES ($1, $2, $3, now(), 'active', now(), now())
	`, id, "fb-rule9-"+id.String(), id.String()+"@rule9.invalid")
	require.NoError(t, err)
	return id
}

func seedRule9Product(ctx context.Context, t *testing.T, pool *pgxpool.Pool, sellerID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, created_at, updated_at)
		VALUES ($1, $2, 'Rule9 Koi', 'desc', '[]'::jsonb, 'kohaku', 'immediate', now(), now())
	`, id, sellerID)
	require.NoError(t, err)
	return id
}

func insertRule9ForSale(ctx context.Context, pool *pgxpool.Pool, product, seller uuid.UUID, status string) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, negotiation_enabled, status, published_at, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, $2, 100000, false, $3::for_sale_status_enum, now(), now(), now())
	`, product, seller, status)
	return err
}

func insertRule9Auction(ctx context.Context, pool *pgxpool.Pool, product, seller uuid.UUID, status string) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO auctions (id, seller_id, product_id, start_price, bid_increment, buy_now_price, start_at, end_at, status, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, $2, 100000, 10000, 200000, now(), now() + interval '24 hours', $3::auction_status_enum, now(), now())
	`, seller, product, status)
	return err
}
