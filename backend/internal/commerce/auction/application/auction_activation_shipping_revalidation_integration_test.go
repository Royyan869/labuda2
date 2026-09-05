//go:build integration

package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/internal/commerce/auction/entity"
	shippingRepo "github.com/labuda/backend/internal/commerce/shipping/infrastructure/repository"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"go.uber.org/zap"
)

// seedShippingValidForAuction creates a shipping option with active coverage and product link.
// Shared helpers seedRevalidationAuction / seedRevalidationUser / noopAccountStatusForRevalidation
// are defined in auction_worker_revalidation_integration_test.go (same package, same build tag).

func seedShippingValidForAuction(t *testing.T, ctx context.Context, pool *testdb.TestDB, sellerID, productID uuid.UUID) uuid.UUID {
	t.Helper()
	optionID := uuid.New()
	optionName := "JNE-" + optionID.String()[:8]
	require.NoError(t, pool.WithTx(ctx, func(tx db.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO shipping_options (id, seller_id, name, transport_type, is_active, created_at, updated_at, internal_purpose)
			VALUES ($1, $2, $3, 'train', true, NOW(), NOW(), NULL)
		`, optionID, sellerID, optionName); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO shipping_coverages (id, shipping_option_id, province_code, province_name, province_rate, is_available, created_at)
			VALUES ($1, $2, '31', 'DKI Jakarta', $3, true, NOW())
		`, uuid.New(), optionID, int64(15_000)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO product_shipping_options (product_id, shipping_option_id, sort_order, created_at)
			VALUES ($1, $2, 0, NOW())
		`, productID, optionID); err != nil {
			return err
		}
		return nil
	}))
	return optionID
}

func newShippingRevalidationService(t *testing.T) *AuctionService {
	t.Helper()
	shippingSetupRepo := shippingRepo.NewShippingSetupRepository()
	coverageRepo := shippingRepo.NewShippingCoverageRepository()
	productShippingRepo := shippingRepo.NewProductShippingSetupRepository(shippingSetupRepo)
	svc := NewAuctionService(
		noopAccountStatusForRevalidation{},
		nil,
		shippingSetupRepo,
		coverageRepo,
		productShippingRepo,
		outboxRepo.NewOutboxRepository(nil),
		nil, nil,
		auctionRoleCheckerStub{hasCapability: true},
		nil,
		zap.NewNop(),
	)
	return svc
}

// 1. Valid shipping -> activation succeeds
func TestActivationShipping_ValidCoverage_Activates(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()
	dbWrap := db.NewFromPool(pool)

	var sellerID, productID, auctionID uuid.UUID
	require.NoError(t, dbWrap.WithTx(ctx, func(tx db.Tx) error {
		sellerID = seedRevalidationUser(t, ctx, tx)
		productID = uuid.New()
		_, err := tx.Exec(ctx, `INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, created_at, updated_at) VALUES ($1,$2,'Koi','desc','[]','kohaku','immediate',NOW(),NOW())`, productID, sellerID)
		return err
	}))
	seedShippingValidForAuction(t, ctx, tdb, sellerID, productID)

	startPast := time.Now().Add(-10 * time.Second)
	end := startPast.Add(24 * time.Hour)
	auctionID = seedRevalidationAuction(t, ctx, tdb, sellerID, productID, entity.StatusScheduled, startPast, end, nil, nil)

	svc := newShippingRevalidationService(t)
	require.NoError(t, dbWrap.WithTx(ctx, func(tx db.Tx) error {
		return svc.ActivateScheduledAuction(ctx, tx, ActivateScheduledAuctionInput{AuctionID: auctionID})
	}))

	var status string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM auctions WHERE id=$1`, auctionID).Scan(&status))
	assert.Equal(t, string(entity.StatusActive), status)

	var outboxCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM outbox WHERE aggregate_id=$1 AND event_type='auction.activated'`, auctionID).Scan(&outboxCount))
	assert.Equal(t, 1, outboxCount, "activation must emit auction.activated")
}

// 2. Shipping becomes invalid before activation -> remains scheduled, no event
func TestActivationShipping_InvalidCoverage_Skipped(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()
	dbWrap := db.NewFromPool(pool)

	var sellerID, productID, auctionID, optionID uuid.UUID
	require.NoError(t, dbWrap.WithTx(ctx, func(tx db.Tx) error {
		sellerID = seedRevalidationUser(t, ctx, tx)
		productID = uuid.New()
		_, err := tx.Exec(ctx, `INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, created_at, updated_at) VALUES ($1,$2,'Koi','desc','[]','kohaku','immediate',NOW(),NOW())`, productID, sellerID)
		return err
	}))
	optionID = seedShippingValidForAuction(t, ctx, tdb, sellerID, productID)

	startPast := time.Now().Add(-10 * time.Second)
	end := startPast.Add(24 * time.Hour)
	auctionID = seedRevalidationAuction(t, ctx, tdb, sellerID, productID, entity.StatusScheduled, startPast, end, nil, nil)

	// Invalidate: remove coverage and unlink (simulates seller deleting coverage after scheduling)
	_, err := pool.Exec(ctx, `DELETE FROM shipping_coverages WHERE shipping_option_id=$1`, optionID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM product_shipping_options WHERE product_id=$1 AND shipping_option_id=$2`, productID, optionID)
	require.NoError(t, err)

	svc := newShippingRevalidationService(t)
	require.NoError(t, dbWrap.WithTx(ctx, func(tx db.Tx) error {
		return svc.ActivateScheduledAuction(ctx, tx, ActivateScheduledAuctionInput{AuctionID: auctionID})
	}))

	var status string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM auctions WHERE id=$1`, auctionID).Scan(&status))
	assert.Equal(t, string(entity.StatusScheduled), status, "must remain scheduled when coverage missing")

	var outboxCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM outbox WHERE aggregate_id=$1 AND event_type='auction.activated'`, auctionID).Scan(&outboxCount))
	assert.Equal(t, 0, outboxCount, "no activation event when coverage invalid")

	// also test is_available=false case: re-add link but with only inactive coverage
	_, err = pool.Exec(ctx, `INSERT INTO product_shipping_options (product_id, shipping_option_id, sort_order, created_at) VALUES ($1,$2,1,NOW())`, productID, optionID)
	require.NoError(t, err)
	newCoverageID := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO shipping_coverages (id, shipping_option_id, province_code, province_name, province_rate, is_available, created_at) VALUES ($1,$2,'31','DKI Jakarta',$3,false,NOW())`, newCoverageID, optionID, int64(15_000))
	require.NoError(t, err)

	// Try again – still invalid because only inactive coverage exists
	require.NoError(t, dbWrap.WithTx(ctx, func(tx db.Tx) error {
		return svc.ActivateScheduledAuction(ctx, tx, ActivateScheduledAuctionInput{AuctionID: auctionID})
	}))
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM auctions WHERE id=$1`, auctionID).Scan(&status))
	assert.Equal(t, string(entity.StatusScheduled), status)
}

// 3. Shipping becomes valid again -> activation succeeds
func TestActivationShipping_RestoredCoverage_Activates(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()
	dbWrap := db.NewFromPool(pool)

	var sellerID, productID, auctionID, optionID uuid.UUID
	require.NoError(t, dbWrap.WithTx(ctx, func(tx db.Tx) error {
		sellerID = seedRevalidationUser(t, ctx, tx)
		productID = uuid.New()
		_, err := tx.Exec(ctx, `INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, created_at, updated_at) VALUES ($1,$2,'Koi','desc','[]','kohaku','immediate',NOW(),NOW())`, productID, sellerID)
		return err
	}))
	optionID = seedShippingValidForAuction(t, ctx, tdb, sellerID, productID)
	startPast := time.Now().Add(-10 * time.Second)
	end := startPast.Add(24 * time.Hour)
	auctionID = seedRevalidationAuction(t, ctx, tdb, sellerID, productID, entity.StatusScheduled, startPast, end, nil, nil)

	// Invalidate
	_, err := pool.Exec(ctx, `DELETE FROM shipping_coverages WHERE shipping_option_id=$1`, optionID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM product_shipping_options WHERE product_id=$1`, productID)
	require.NoError(t, err)

	svc := newShippingRevalidationService(t)
	require.NoError(t, dbWrap.WithTx(ctx, func(tx db.Tx) error {
		return svc.ActivateScheduledAuction(ctx, tx, ActivateScheduledAuctionInput{AuctionID: auctionID})
	}))
	var status string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM auctions WHERE id=$1`, auctionID).Scan(&status))
	require.Equal(t, string(entity.StatusScheduled), status)

	// Restore: re-insert coverage + link
	_, err = pool.Exec(ctx, `INSERT INTO shipping_coverages (id, shipping_option_id, province_code, province_name, province_rate, is_available, created_at) VALUES ($1,$2,'31','DKI Jakarta',$3,true,NOW())`, uuid.New(), optionID, int64(15_000))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO product_shipping_options (product_id, shipping_option_id, sort_order, created_at) VALUES ($1,$2,0,NOW())`, productID, optionID)
	require.NoError(t, err)

	require.NoError(t, dbWrap.WithTx(ctx, func(tx db.Tx) error {
		return svc.ActivateScheduledAuction(ctx, tx, ActivateScheduledAuctionInput{AuctionID: auctionID})
	}))
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM auctions WHERE id=$1`, auctionID).Scan(&status))
	assert.Equal(t, string(entity.StatusActive), status)
}

// 4. Existing guards remain intact
func TestActivationShipping_GuardsRemainIntact(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()
	dbWrap := db.NewFromPool(pool)

	var sellerID, productID uuid.UUID
	require.NoError(t, dbWrap.WithTx(ctx, func(tx db.Tx) error {
		sellerID = seedRevalidationUser(t, ctx, tx)
		productID = uuid.New()
		_, err := tx.Exec(ctx, `INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, created_at, updated_at) VALUES ($1,$2,'Koi','desc','[]','kohaku','immediate',NOW(),NOW())`, productID, sellerID)
		return err
	}))
	seedShippingValidForAuction(t, ctx, tdb, sellerID, productID)

	// Future start -> remains scheduled (timing guard)
	futureStart := time.Now().Add(5 * time.Hour)
	futureEnd := futureStart.Add(24 * time.Hour)
	auctionFuture := seedRevalidationAuction(t, ctx, tdb, sellerID, productID, entity.StatusScheduled, futureStart, futureEnd, nil, nil)
	svc := newShippingRevalidationService(t)
	require.NoError(t, dbWrap.WithTx(ctx, func(tx db.Tx) error {
		return svc.ActivateScheduledAuction(ctx, tx, ActivateScheduledAuctionInput{AuctionID: auctionFuture})
	}))
	var status string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM auctions WHERE id=$1`, auctionFuture).Scan(&status))
	assert.Equal(t, string(entity.StatusScheduled), status, "timing guard must still work")

	// Cancelled -> no activation (cancellation guard) - use fresh product to avoid uniq_active_auction_per_product
	cancelProductID := uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, created_at, updated_at) VALUES ($1,$2,'Koi2','desc','[]','kohaku','immediate',NOW(),NOW())`, cancelProductID, sellerID)
	require.NoError(t, err)
	seedShippingValidForAuction(t, ctx, tdb, sellerID, cancelProductID)
	startPast := time.Now().Add(-10 * time.Second)
	end := startPast.Add(24 * time.Hour)
	auctionCancelled := seedRevalidationAuction(t, ctx, tdb, sellerID, cancelProductID, entity.StatusScheduled, startPast, end, nil, nil)
	_, err = pool.Exec(ctx, `UPDATE auctions SET status='cancelled' WHERE id=$1`, auctionCancelled)
	require.NoError(t, err)
	require.NoError(t, dbWrap.WithTx(ctx, func(tx db.Tx) error {
		return svc.ActivateScheduledAuction(ctx, tx, ActivateScheduledAuctionInput{AuctionID: auctionCancelled})
	}))
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM auctions WHERE id=$1`, auctionCancelled).Scan(&status))
	assert.Equal(t, string(entity.StatusCancelled), status)

	// Seller capability false -> cancels instead of activating (market authority guard)
	// Use service with hasCapability=false, fresh product
	svcNoCap := NewAuctionService(
		noopAccountStatusForRevalidation{},
		nil,
		shippingRepo.NewShippingSetupRepository(),
		shippingRepo.NewShippingCoverageRepository(),
		shippingRepo.NewProductShippingSetupRepository(shippingRepo.NewShippingSetupRepository()),
		outboxRepo.NewOutboxRepository(nil),
		nil, nil,
		auctionRoleCheckerStub{hasCapability: false},
		nil,
		zap.NewNop(),
	)
	noCapProductID := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, created_at, updated_at) VALUES ($1,$2,'Koi3','desc','[]','kohaku','immediate',NOW(),NOW())`, noCapProductID, sellerID)
	require.NoError(t, err)
	seedShippingValidForAuction(t, ctx, tdb, sellerID, noCapProductID)
	auctionNoCap := seedRevalidationAuction(t, ctx, tdb, sellerID, noCapProductID, entity.StatusScheduled, startPast, end, nil, nil)
	// need fresh valid shipping for this product already exists, but product is same; ensure at least one valid
	require.NoError(t, dbWrap.WithTx(ctx, func(tx db.Tx) error {
		return svcNoCap.ActivateScheduledAuction(ctx, tx, ActivateScheduledAuctionInput{AuctionID: auctionNoCap})
	}))
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM auctions WHERE id=$1`, auctionNoCap).Scan(&status))
	assert.Equal(t, string(entity.StatusCancelled), status, "market authority must still cancel when capability missing")
}

// 5. Fail-closed: missing required shipping dependency must not silently activate
func TestActivationShipping_FailClosed_MissingDependency(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()
	dbWrap := db.NewFromPool(pool)

	var sellerID, productID, auctionID uuid.UUID
	require.NoError(t, dbWrap.WithTx(ctx, func(tx db.Tx) error {
		sellerID = seedRevalidationUser(t, ctx, tx)
		productID = uuid.New()
		_, err := tx.Exec(ctx, `INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, created_at, updated_at) VALUES ($1,$2,'Koi','desc','[]','kohaku','immediate',NOW(),NOW())`, productID, sellerID)
		return err
	}))
	seedShippingValidForAuction(t, ctx, tdb, sellerID, productID)
	startPast := time.Now().Add(-10 * time.Second)
	end := startPast.Add(24 * time.Hour)
	auctionID = seedRevalidationAuction(t, ctx, tdb, sellerID, productID, entity.StatusScheduled, startPast, end, nil, nil)

	// Service with missing required shipping repos – must fail closed
	svc := NewAuctionService(
		noopAccountStatusForRevalidation{},
		nil,
		nil, // shippingSetupRepo nil
		nil, // shippingCoverageRepo nil – required
		nil, // productShippingRepo nil – required
		outboxRepo.NewOutboxRepository(nil),
		nil, nil,
		auctionRoleCheckerStub{hasCapability: true},
		nil,
		zap.NewNop(),
	)
	err := dbWrap.WithTx(ctx, func(tx db.Tx) error {
		return svc.ActivateScheduledAuction(ctx, tx, ActivateScheduledAuctionInput{AuctionID: auctionID})
	})
	require.Error(t, err, "missing shipping dependencies must surface error, not silent skip")
	assert.Contains(t, err.Error(), "shipping")

	var status string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM auctions WHERE id=$1`, auctionID).Scan(&status))
	assert.Equal(t, string(entity.StatusScheduled), status, "must remain scheduled when dependency missing")
	var outboxCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM outbox WHERE aggregate_id=$1 AND event_type='auction.activated'`, auctionID).Scan(&outboxCount))
	assert.Equal(t, 0, outboxCount)
}
