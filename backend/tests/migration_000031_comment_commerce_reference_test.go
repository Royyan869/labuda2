//go:build integration

package tests

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/pkg/testdb"
)

// TestMigration00031_SchemaState verifies the post-migration schema.
// testdb.SetupDB applies ALL migrations including 000031, so we verify
// the final expected state.
func TestMigration00031_SchemaState(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()

	// comments.type absent
	var typeColExists bool
	err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='comments' AND column_name='type')`).Scan(&typeColExists)
	require.NoError(t, err)
	require.False(t, typeColExists, "comments.type must be dropped by migration 000031")

	// comments.for_sale_id absent
	var fpsColExists bool
	err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='comments' AND column_name='for_sale_id')`).Scan(&fpsColExists)
	require.NoError(t, err)
	require.False(t, fpsColExists, "comments.for_sale_id must be dropped")

	// comment_type_enum absent
	var enumExists bool
	err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_type WHERE typname='comment_type_enum')`).Scan(&enumExists)
	require.NoError(t, err)
	require.False(t, enumExists, "comment_type_enum must be dropped")

	// comment_commerce_references table exists
	var ccrExists bool
	err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name='comment_commerce_references')`).Scan(&ccrExists)
	require.NoError(t, err)
	require.True(t, ccrExists, "comment_commerce_references must exist")

	// Partial indexes exist (canonical For Sale names after 000047)
	for _, idx := range []string{"idx_comment_commerce_ref_for_sale", "idx_comment_commerce_ref_auction"} {
		var idxExists bool
		err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE indexname=$1)`, idx).Scan(&idxExists)
		require.NoError(t, err)
		require.True(t, idxExists, "index %s must exist", idx)
	}

	// FPS FK exists on comment_commerce_references
	var fpsFKExists bool
	err = pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM information_schema.table_constraints tc
		JOIN information_schema.referential_constraints rc ON tc.constraint_name = rc.constraint_name
		WHERE tc.table_name='comment_commerce_references' AND rc.unique_constraint_name LIKE '%for_sales%')
	`).Scan(&fpsFKExists)
	require.NoError(t, err)
	require.True(t, fpsFKExists, "FPS FK must exist")
}

// TestMigration00031_Constraint_ZeroSourceRejected verifies the exactly-one-source CHECK.
func TestMigration00031_Constraint_ZeroSourceRejected(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()

	userA := seedMigrationUser(t, ctx, pool, "active")
	product := seedMigrationProduct(t, ctx, pool, userA)
	fps := seedMigrationFPS(t, ctx, pool, userA, product)
	content := seedMigrationContent(t, ctx, pool, userA, "public")

	// Create a normal comment
	var commentID uuid.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO comments (id, author_id, body, target_id, target_type, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, 'test', $2, 'content', NOW(), NOW()) RETURNING id
	`, userA, content).Scan(&commentID)
	require.NoError(t, err)

	// Zero-source association rejected
	_, err = pool.Exec(ctx, `
		INSERT INTO comment_commerce_references (comment_id, for_sale_id, auction_id)
		VALUES ($1, NULL, NULL)
	`, commentID)
	require.Error(t, err, "zero-source must be rejected by CHECK constraint")
	require.Contains(t, err.Error(), "comment_commerce_reference_exactly_one_source")

	// Two-source association rejected
	_, err = pool.Exec(ctx, `
		INSERT INTO comment_commerce_references (comment_id, for_sale_id, auction_id)
		VALUES ($1, $2, uuid_nil())
	`, commentID, fps)
	// Second source (uuid_nil is not valid so this might fail FK, not CHECK)
	// Let's test properly:
	_, err = pool.Exec(ctx, `
		INSERT INTO comment_commerce_references (comment_id, for_sale_id, auction_id)
		VALUES ($1, $2, $2)
	`, commentID, fps)
	require.Error(t, err, "two-source must be rejected")
}

// TestMigration00031_Constraint_NonexistentFKRejected verifies FK constraints.
func TestMigration00031_Constraint_NonexistentFKRejected(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()

	userA := seedMigrationUser(t, ctx, pool, "active")
	content := seedMigrationContent(t, ctx, pool, userA, "public")

	var commentID uuid.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO comments (id, author_id, body, target_id, target_type, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, 'test', $2, 'content', NOW(), NOW()) RETURNING id
	`, userA, content).Scan(&commentID)
	require.NoError(t, err)

	// Nonexistent FPS rejected
	_, err = pool.Exec(ctx, `
		INSERT INTO comment_commerce_references (comment_id, for_sale_id, auction_id)
		VALUES ($1, gen_random_uuid(), NULL)
	`, commentID)
	require.Error(t, err, "nonexistent FPS must be rejected by FK")
	require.Contains(t, err.Error(), "violates foreign key")

	// Nonexistent Auction rejected
	_, err = pool.Exec(ctx, `
		INSERT INTO comment_commerce_references (comment_id, for_sale_id, auction_id)
		VALUES ($1, NULL, gen_random_uuid())
	`, commentID)
	require.Error(t, err, "nonexistent Auction must be rejected by FK")
	require.Contains(t, err.Error(), "violates foreign key")
}

// TestMigration00031_DeleteCascadeRestrict verifies cascade/restrict semantics.
func TestMigration00031_DeleteCascadeRestrict(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()

	userA := seedMigrationUser(t, ctx, pool, "active")
	product := seedMigrationProduct(t, ctx, pool, userA)
	fps := seedMigrationFPS(t, ctx, pool, userA, product)
	content := seedMigrationContent(t, ctx, pool, userA, "public")

	var commentID uuid.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO comments (id, author_id, body, target_id, target_type, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, 'test', $2, 'content', NOW(), NOW()) RETURNING id
	`, userA, content).Scan(&commentID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO comment_commerce_references (comment_id, for_sale_id, auction_id)
		VALUES ($1, $2, NULL)
	`, commentID, fps)
	require.NoError(t, err)

	// Comment delete cascades association
	_, err = pool.Exec(ctx, `DELETE FROM comments WHERE id=$1`, commentID)
	require.NoError(t, err)
	var assocExists bool
	err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM comment_commerce_references WHERE comment_id=$1)`, commentID).Scan(&assocExists)
	require.NoError(t, err)
	require.False(t, assocExists, "association must cascade-delete with comment")
}

// ── Helpers ──

func seedMigrationUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, status string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, firebase_uid, email, email_verified_at, phone_verified, account_status, created_at, updated_at) VALUES ($1,$2,$3,NOW(),true,$4,NOW(),NOW())`,
		id, id.String(), id.String()+"@test.invalid", status)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO user_profiles (id, user_id, username, created_at, updated_at) VALUES ($1,$2,$3,NOW(),NOW())`,
		uuid.New(), id, "user-"+id.String()[:8])
	require.NoError(t, err)
	return id
}

func seedMigrationProduct(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sellerID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		id, sellerID, "Product", "desc", `["https://example.com/1.jpg"]`, "kohaku", "immediate")
	require.NoError(t, err)
	return id
}

func seedMigrationFPS(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sellerID, productID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, status, published_at, quantity_available) VALUES ($1,$2,$3,$4,'active',NOW(),1)`,
		id, productID, sellerID, int64(100000))
	require.NoError(t, err)
	return id
}

func seedMigrationContent(t *testing.T, ctx context.Context, pool *pgxpool.Pool, authorID uuid.UUID, visibility string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO contents (id, author_id, status, caption, is_hidden, visibility, created_at, updated_at) VALUES ($1,$2,'active',$3,false,$4,NOW(),NOW())`,
		id, authorID, "test", visibility)
	require.NoError(t, err)
	return id
}
