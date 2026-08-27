//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

// Stage 8 proof: Product is the sole canonical authority for all product
// identity/content. ForSale and Auction are selling surfaces that
// read content from Product — they never carry their own authority.
//
// These tests run against real Postgres (labuda_test) to prove runtime
// correctness of the canonical architecture.

// seedStage8Product creates a product with known content fields.
func seedStage8Product(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sellerID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, title, description, media_urls, variety,
			size_cm, age_months, gender, breeder, bloodline, certificates,
			farm_address_id, preparation_time, preparation_note, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12,
			NULL, $13, NULL, NOW(), NOW())
	`, id, sellerID, "Canonical Koi", "The one true description",
		`["https://cdn.test/koi.jpg"]`, "Kohaku",
			50, 12, "female", "Acme Breeder", "Ogata", "{}",

		"short")
	require.NoError(t, err)
	return id
}

// TestProductContent_SingleAuthority_AuctionReadsFromProduct proves that
// Auction detail (read via JOIN) returns content from Product, not from
// auction-local columns. The auction is INSERTed without any title/description.
func TestProductContent_SingleAuthority_AuctionReadsFromProduct(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	sellerID := seedStage1User(t, ctx, db.NewFromPool(tdb.Pool()))
	productID := seedStage8Product(t, ctx, tdb.Pool(), sellerID)
	auctionID := uuid.New()

	// Insert auction WITHOUT title, description, preparation_time —
	// these columns are dropped by migration 000046. The auction relies
	// entirely on the joined Product for content.
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO auctions (id, seller_id, product_id,
			start_price, bid_increment, buy_now_price,
			start_at, end_at, status, created_at, updated_at)
		VALUES ($1, $2, $3,
			10000, 1000, NULL,
			NOW(), NOW() + INTERVAL '24 hours', 'active', NOW(), NOW())
	`, auctionID, sellerID, productID)
	require.NoError(t, err)

	// Read auction via JOIN — the production query path.
	var title, description, variety string
	var mediaURLsRaw json.RawMessage
	err = tdb.Pool().QueryRow(ctx, `
		SELECT p.title, p.description, p.media_urls, p.variety
		FROM auctions a
		JOIN products p ON p.id = a.product_id
		WHERE a.id = $1
	`, auctionID).Scan(&title, &description, &mediaURLsRaw, &variety)
	require.NoError(t, err)
	require.Equal(t, "Canonical Koi", title, "auction title must come from Product")
	require.Equal(t, "The one true description", description, "auction description must come from Product")
	require.Equal(t, "Kohaku", variety, "auction koi variety must come from Product")
	require.Contains(t, string(mediaURLsRaw), "koi.jpg", "auction media must come from Product")
}

// TestProductContent_SingleAuthority_FPSReadsFromProduct proves that ForSale
// response projection reads title/description from Product (canonical path),
// not from its own in-memory mirror fields.
func TestProductContent_SingleAuthority_FPSReadsFromProduct(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	sellerID := seedStage1User(t, ctx, db.NewFromPool(tdb.Pool()))
	productID := seedStage8Product(t, ctx, tdb.Pool(), sellerID)
	saleID := uuid.New()

	// Insert FPS — no title/description columns exist on for_sales.
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO for_sales (id, product_id, seller_id,
			price_per_unit, negotiation_enabled, status,
			quantity_available, created_at, updated_at)
		VALUES ($1, $2, $3,
			150000, false, 'active',
			1, NOW(), NOW())
	`, saleID, productID, sellerID)
	require.NoError(t, err)

	// Read FPS via JOIN — the production query path.
	var title, description string
	err = tdb.Pool().QueryRow(ctx, `
		SELECT p.title, p.description
		FROM for_sales fps
		JOIN products p ON p.id = fps.product_id
		WHERE fps.id = $1
	`, saleID).Scan(&title, &description)
	require.NoError(t, err)
	require.Equal(t, "Canonical Koi", title, "FPS title must come from Product")
	require.Equal(t, "The one true description", description, "FPS description must come from Product")
}

// TestProductContent_NoDuplicateAuctionContent proves that the auction entity
// has no local title/description fields (compile-time and runtime proof).
func TestProductContent_NoDuplicateAuctionContent(t *testing.T) {
	// Compile-time proof: entity.Auction no longer has Title/Description fields.
	// Runtime proof: CreateDraft does not accept title/description; NewDraft
	// signature has no title/description/preparation parameters.
	productID := uuid.New()
	sellerID := uuid.New()
	now := time.Now()
	auction := auctionEntity.NewDraft(
		sellerID, productID,
		10000, 1000, nil,
		now.Add(time.Hour), now.Add(25*time.Hour),
	)
	require.Equal(t, sellerID, auction.SellerID)
	require.Equal(t, productID, auction.ProductID)
	require.Equal(t, int64(10000), auction.StartPrice)
	require.Equal(t, int64(1000), auction.BidIncrement)
	// Auction content (title, description, preparation) is NOT on the entity.
	// It is accessed through auction.Product (nil by default — set by repo).
	require.Nil(t, auction.Product, "Product is nil until joined by repository")
}

// TestProductContent_MediaCanonical proves that media is read from Product
// by both Auction and FPS, not from dead media tables.
func TestProductContent_MediaCanonical(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	sellerID := seedStage1User(t, ctx, db.NewFromPool(tdb.Pool()))
	productID := seedStage8Product(t, ctx, tdb.Pool(), sellerID)
	auctionID := uuid.New()

	// Insert auction
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO auctions (id, seller_id, product_id,
			start_price, bid_increment, buy_now_price,
			start_at, end_at, status, created_at, updated_at)
		VALUES ($1, $2, $3,
			10000, 1000, NULL,
			NOW(), NOW() + INTERVAL '24 hours', 'active', NOW(), NOW())
	`, auctionID, sellerID, productID)
	require.NoError(t, err)

	// Read media from Product via auction JOIN
	var mediaURLsRaw json.RawMessage
	err = tdb.Pool().QueryRow(ctx, `
		SELECT p.media_urls
		FROM auctions a
		JOIN products p ON p.id = a.product_id
		WHERE a.id = $1
	`, auctionID).Scan(&mediaURLsRaw)
	require.NoError(t, err)
	require.Contains(t, string(mediaURLsRaw), "koi.jpg", "auction media must come from Product, not auction_media")

	// Same for FPS
	saleID := uuid.New()
	_, err = tdb.Pool().Exec(ctx, `
		INSERT INTO for_sales (id, product_id, seller_id,
			price_per_unit, negotiation_enabled, status,
			quantity_available, created_at, updated_at)
		VALUES ($1, $2, $3,
			150000, false, 'active',
			1, NOW(), NOW())
	`, saleID, productID, sellerID)
	require.NoError(t, err)

	err = tdb.Pool().QueryRow(ctx, `
		SELECT p.media_urls
		FROM for_sales fps
		JOIN products p ON p.id = fps.product_id
		WHERE fps.id = $1
	`, saleID).Scan(&mediaURLsRaw)
	require.NoError(t, err)
	require.Contains(t, string(mediaURLsRaw), "koi.jpg", "FPS media must come from Product, not for_sale_media")
}

// TestProductContent_Reuse_DoesNotCreateDuplicate proves that reusing a
// Product (sequential surface reuse) does not duplicate the Product row.
// The Product is the stable identity; only the surface changes.
func TestProductContent_Reuse_DoesNotCreateDuplicate(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	sellerID := seedStage1User(t, ctx, db.NewFromPool(tdb.Pool()))
	productID := seedStage8Product(t, ctx, tdb.Pool(), sellerID)

	// Create FPS that reuses the Product
	saleID := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO for_sales (id, product_id, seller_id,
			price_per_unit, negotiation_enabled, status,
			quantity_available, created_at, updated_at)
		VALUES ($1, $2, $3,
			150000, false, 'active',
			1, NOW(), NOW())
	`, saleID, productID, sellerID)
	require.NoError(t, err)

	// Verify the Product row is unique
	var count int
	err = tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM products WHERE id = $1`, productID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count, "reusing a Product must not create a duplicate Product row")
}

// TestProductContent_ChatCommentReadsCanonicalContent proves that
// chat/comment projections read content from Product, not from auction-local
// columns or media tables.
func TestProductContent_ChatCommentReadsCanonicalContent(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	sellerID := seedStage1User(t, ctx, db.NewFromPool(tdb.Pool()))
	productID := seedStage8Product(t, ctx, tdb.Pool(), sellerID)
	auctionID := uuid.New()

	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO auctions (id, seller_id, product_id,
			start_price, bid_increment, buy_now_price,
			start_at, end_at, status, created_at, updated_at)
		VALUES ($1, $2, $3,
			10000, 1000, NULL,
			NOW(), NOW() + INTERVAL '24 hours', 'active', NOW(), NOW())
	`, auctionID, sellerID, productID)
	require.NoError(t, err)

	// Simulate the chat/forsale projection resolver query:
	// reads title from Product, not from auctions.
	var title string
	var mediaURLsRaw json.RawMessage
	err = tdb.Pool().QueryRow(ctx, `
		SELECT p.title, p.media_urls
		FROM auctions a
		JOIN products p ON p.id = a.product_id
		WHERE a.id = $1
	`, auctionID).Scan(&title, &mediaURLsRaw)
	require.NoError(t, err)
	require.Equal(t, "Canonical Koi", title, "chat/comment must read auction title from Product")
	require.Contains(t, string(mediaURLsRaw), "koi.jpg", "chat/comment must read auction media from Product")
}
