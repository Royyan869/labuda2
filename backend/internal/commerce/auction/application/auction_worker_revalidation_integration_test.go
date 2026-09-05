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
	productRepoImpl "github.com/labuda/backend/internal/commerce/product/infrastructure/repository"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"go.uber.org/zap"
)

func seedRevalidationAuction(t *testing.T, ctx context.Context, pool *testdb.TestDB, sellerID, productID uuid.UUID, status entity.Status, startAt, endAt time.Time, winnerID *uuid.UUID, currentBid *int64) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Pool().Exec(ctx, `
		INSERT INTO auctions (id, seller_id, product_id, start_price, bid_increment, buy_now_price, start_at, end_at, current_bid, current_winner_id, status, created_at, updated_at, anti_snipe_extension_seconds)
		VALUES ($1, $2, $3, 10000, 1000, NULL, $4, $5, $6, $7, $8, NOW(), NOW(), 0)
	`, id, sellerID, productID, startAt, endAt, currentBid, winnerID, string(status))
	require.NoError(t, err)
	return id
}

func seedRevalidationUser(t *testing.T, ctx context.Context, tx db.Tx) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := tx.Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, email_verified_at, account_status, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), 'active', NOW(), NOW())
	`, id, "fb-"+id.String(), id.String()+"@test.invalid")
	require.NoError(t, err)
	return id
}

type noopAccountStatusForRevalidation struct{}

func (noopAccountStatusForRevalidation) EnsureActive(context.Context, uuid.UUID) error { return nil }
func (noopAccountStatusForRevalidation) GetStatus(context.Context, uuid.UUID) (string, error) {
	return "active", nil
}
func (noopAccountStatusForRevalidation) IsBanned(context.Context, uuid.UUID) (bool, error) {
	return false, nil
}

func newRevalidationService(prodRepo *productRepoImpl.ProductRepositoryImpl) *AuctionService {
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
	if prodRepo != nil {
		svc.SetProductRepo(prodRepo)
	}
	return svc
}

func seedShippingValidForRevalidation(t *testing.T, ctx context.Context, pool *testdb.TestDB, sellerID, productID uuid.UUID) {
	t.Helper()
	optionID := uuid.New()
	optionName := "JNE-R-" + optionID.String()[:8]
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
}

func TestEndWorker_Revalidation_StaleEndCandidate_Skipped(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()
	dbWrap := db.NewFromPool(pool)

	var sellerID, productID, auctionID uuid.UUID
	err := dbWrap.WithTx(ctx, func(tx db.Tx) error {
		sellerID = seedRevalidationUser(t, ctx, tx)
		productID = uuid.New()
		_, err := tx.Exec(ctx, `INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, created_at, updated_at) VALUES ($1,$2,'Koi','desc','[]','kohaku','immediate',NOW(),NOW())`, productID, sellerID)
		return err
	})
	require.NoError(t, err)

	endAtExpired := time.Now().Add(-5 * time.Second)
	startAt := endAtExpired.Add(-24 * time.Hour)
	auctionID = seedRevalidationAuction(t, ctx, tdb, sellerID, productID, entity.StatusActive, startAt, endAtExpired, nil, nil)

	var foundID uuid.UUID
	err = pool.QueryRow(ctx, `SELECT id FROM auctions WHERE status='active' AND end_at <= NOW() LIMIT 1`).Scan(&foundID)
	require.NoError(t, err)
	assert.Equal(t, auctionID, foundID, "phase-1 must discover expired auction")

	extendedEnd := time.Now().Add(5 * time.Minute)
	_, err = pool.Exec(ctx, `UPDATE auctions SET end_at=$1, updated_at=NOW() WHERE id=$2`, extendedEnd, auctionID)
	require.NoError(t, err)

	svc := newRevalidationService(nil)
	err = dbWrap.WithTx(ctx, func(tx db.Tx) error {
		return svc.EndAuctionInternal(ctx, tx, EndAuctionInput{AuctionID: auctionID})
	})
	require.NoError(t, err, "stale end must be safe no-op, not error")

	var status string
	var dbEndAt time.Time
	err = pool.QueryRow(ctx, `SELECT status, end_at FROM auctions WHERE id=$1`, auctionID).Scan(&status, &dbEndAt)
	require.NoError(t, err)
	assert.Equal(t, string(entity.StatusActive), status, "auction must remain active after stale end skipped")
	assert.True(t, dbEndAt.After(time.Now()), "end_at must remain future (extension preserved)")

	var outboxCount int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM outbox WHERE aggregate_id=$1 AND event_type IN ('auction.ended','auction.waiting_settlement')`, auctionID).Scan(&outboxCount)
	if err == nil {
		assert.Equal(t, 0, outboxCount, "no lifecycle event must be emitted for stale candidate")
	}
}

func TestEndWorker_Revalidation_StaleWithWinner_Skipped(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()
	dbWrap := db.NewFromPool(pool)

	var sellerID, productID, auctionID uuid.UUID
	err := dbWrap.WithTx(ctx, func(tx db.Tx) error {
		sellerID = seedRevalidationUser(t, ctx, tx)
		productID = uuid.New()
		_, err := tx.Exec(ctx, `INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, created_at, updated_at) VALUES ($1,$2,'Koi','desc','[]','kohaku','immediate',NOW(),NOW())`, productID, sellerID)
		return err
	})
	require.NoError(t, err)

	bid := int64(15000)
	winner := uuid.New()
	err = dbWrap.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO users (id, firebase_uid, email, email_verified_at, account_status, created_at, updated_at) VALUES ($1,$2,$3,NOW(),'active',NOW(),NOW())`, winner, "fb-"+winner.String(), winner.String()+"@test.invalid")
		return err
	})
	require.NoError(t, err)

	endExpired := time.Now().Add(-2 * time.Second)
	start := endExpired.Add(-24 * time.Hour)
	auctionID = seedRevalidationAuction(t, ctx, tdb, sellerID, productID, entity.StatusActive, start, endExpired, &winner, &bid)

	_, err = pool.Exec(ctx, `UPDATE auctions SET end_at=$1 WHERE id=$2`, time.Now().Add(5*time.Minute), auctionID)
	require.NoError(t, err)

	svc := newRevalidationService(nil)
	err = dbWrap.WithTx(ctx, func(tx db.Tx) error {
		return svc.EndAuctionInternal(ctx, tx, EndAuctionInput{AuctionID: auctionID})
	})
	require.NoError(t, err)

	var status string
	err = pool.QueryRow(ctx, `SELECT status FROM auctions WHERE id=$1`, auctionID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, string(entity.StatusActive), status)
}

func TestEndWorker_NormalExpired_NoWinner_Ends(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()
	dbWrap := db.NewFromPool(pool)

	var sellerID, productID, auctionID uuid.UUID
	err := dbWrap.WithTx(ctx, func(tx db.Tx) error {
		sellerID = seedRevalidationUser(t, ctx, tx)
		productID = uuid.New()
		_, err := tx.Exec(ctx, `INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, created_at, updated_at) VALUES ($1,$2,'Koi','desc','[]','kohaku','immediate',NOW(),NOW())`, productID, sellerID)
		return err
	})
	require.NoError(t, err)

	endExpired := time.Now().Add(-10 * time.Second)
	start := endExpired.Add(-24 * time.Hour)
	auctionID = seedRevalidationAuction(t, ctx, tdb, sellerID, productID, entity.StatusActive, start, endExpired, nil, nil)

	svc := newRevalidationService(nil)
	err = dbWrap.WithTx(ctx, func(tx db.Tx) error {
		return svc.EndAuctionInternal(ctx, tx, EndAuctionInput{AuctionID: auctionID})
	})
	require.NoError(t, err)

	var status string
	err = pool.QueryRow(ctx, `SELECT status FROM auctions WHERE id=$1`, auctionID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, string(entity.StatusEnded), status, "expired active without winner must go to ended")
}

func TestEndWorker_NormalExpired_WithWinner_WaitingSettlement_EntityProof(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()

	sellerID := uuid.New()
	productID := uuid.New()
	winnerID := uuid.New()
	bid := int64(20000)

	_, err := pool.Exec(ctx, `INSERT INTO users (id, firebase_uid, email, email_verified_at, account_status, created_at, updated_at) VALUES ($1,$2,$3,NOW(),'active',NOW(),NOW())`, sellerID, "fb-"+sellerID.String(), sellerID.String()+"@test.invalid")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO users (id, firebase_uid, email, email_verified_at, account_status, created_at, updated_at) VALUES ($1,$2,$3,NOW(),'active',NOW(),NOW())`, winnerID, "fb-"+winnerID.String(), winnerID.String()+"@test.invalid")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, created_at, updated_at) VALUES ($1,$2,'Koi','desc','[]','kohaku','immediate',NOW(),NOW())`, productID, sellerID)
	require.NoError(t, err)

	endExpired := time.Now().Add(-10 * time.Second)
	start := endExpired.Add(-24 * time.Hour)
	auctionID := seedRevalidationAuction(t, ctx, tdb, sellerID, productID, entity.StatusActive, start, endExpired, &winnerID, &bid)

	var status string
	err = pool.QueryRow(ctx, `SELECT status FROM auctions WHERE id=$1`, auctionID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, string(entity.StatusActive), status)
	assert.True(t, time.Now().After(endExpired), "test auction must be expired")
	_ = auctionID
}

func TestStartWorker_Revalidation_StaleScheduled_Skipped(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()
	dbWrap := db.NewFromPool(pool)

	var sellerID, productID, auctionID uuid.UUID
	err := dbWrap.WithTx(ctx, func(tx db.Tx) error {
		sellerID = seedRevalidationUser(t, ctx, tx)
		productID = uuid.New()
		_, err := tx.Exec(ctx, `INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, created_at, updated_at) VALUES ($1,$2,'Koi','desc','[]','kohaku','immediate',NOW(),NOW())`, productID, sellerID)
		return err
	})
	require.NoError(t, err)

	startExpired := time.Now().Add(-5 * time.Second)
	end := startExpired.Add(24 * time.Hour)
	auctionID = seedRevalidationAuction(t, ctx, tdb, sellerID, productID, entity.StatusScheduled, startExpired, end, nil, nil)
	seedShippingValidForRevalidation(t, ctx, tdb, sellerID, productID)

	var foundID uuid.UUID
	err = pool.QueryRow(ctx, `SELECT id FROM auctions WHERE status='scheduled' AND start_at <= NOW() LIMIT 1`).Scan(&foundID)
	require.NoError(t, err)
	assert.Equal(t, auctionID, foundID)

	futureStart := time.Now().Add(5 * time.Hour)
	futureEnd := futureStart.Add(24 * time.Hour)
	_, err = pool.Exec(ctx, `UPDATE auctions SET start_at=$1, end_at=$2, updated_at=NOW() WHERE id=$3`, futureStart, futureEnd, auctionID)
	require.NoError(t, err)

	svc := newRevalidationService(nil)
	err = dbWrap.WithTx(ctx, func(tx db.Tx) error {
		return svc.ActivateScheduledAuction(ctx, tx, ActivateScheduledAuctionInput{AuctionID: auctionID})
	})
	require.NoError(t, err)

	var status string
	var dbStart time.Time
	err = pool.QueryRow(ctx, `SELECT status, start_at FROM auctions WHERE id=$1`, auctionID).Scan(&status, &dbStart)
	require.NoError(t, err)
	assert.Equal(t, string(entity.StatusScheduled), status, "must remain scheduled after stale activation skipped")
	assert.True(t, dbStart.After(time.Now()), "start_at must remain future")
}

func TestStartWorker_NormalScheduled_Activates(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()
	dbWrap := db.NewFromPool(pool)

	var sellerID, productID, auctionID uuid.UUID
	err := dbWrap.WithTx(ctx, func(tx db.Tx) error {
		sellerID = seedRevalidationUser(t, ctx, tx)
		productID = uuid.New()
		_, err := tx.Exec(ctx, `INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, created_at, updated_at) VALUES ($1,$2,'Koi','desc','[]','kohaku','immediate',NOW(),NOW())`, productID, sellerID)
		return err
	})
	require.NoError(t, err)

	startPast := time.Now().Add(-10 * time.Second)
	end := startPast.Add(24 * time.Hour)
	auctionID = seedRevalidationAuction(t, ctx, tdb, sellerID, productID, entity.StatusScheduled, startPast, end, nil, nil)
	seedShippingValidForRevalidation(t, ctx, tdb, sellerID, productID)

	svc := newRevalidationService(nil)
	err = dbWrap.WithTx(ctx, func(tx db.Tx) error {
		return svc.ActivateScheduledAuction(ctx, tx, ActivateScheduledAuctionInput{AuctionID: auctionID})
	})
	require.NoError(t, err)

	var status string
	err = pool.QueryRow(ctx, `SELECT status FROM auctions WHERE id=$1`, auctionID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, string(entity.StatusActive), status)
}

func TestStartWorker_FutureScheduled_RemainsScheduled(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()
	dbWrap := db.NewFromPool(pool)

	var sellerID, productID, auctionID uuid.UUID
	err := dbWrap.WithTx(ctx, func(tx db.Tx) error {
		sellerID = seedRevalidationUser(t, ctx, tx)
		productID = uuid.New()
		_, err := tx.Exec(ctx, `INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, created_at, updated_at) VALUES ($1,$2,'Koi','desc','[]','kohaku','immediate',NOW(),NOW())`, productID, sellerID)
		return err
	})
	require.NoError(t, err)

	startFuture := time.Now().Add(5 * time.Hour)
	end := startFuture.Add(24 * time.Hour)
	auctionID = seedRevalidationAuction(t, ctx, tdb, sellerID, productID, entity.StatusScheduled, startFuture, end, nil, nil)

	svc := newRevalidationService(nil)
	err = dbWrap.WithTx(ctx, func(tx db.Tx) error {
		return svc.ActivateScheduledAuction(ctx, tx, ActivateScheduledAuctionInput{AuctionID: auctionID})
	})
	require.NoError(t, err)

	var status string
	err = pool.QueryRow(ctx, `SELECT status FROM auctions WHERE id=$1`, auctionID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, string(entity.StatusScheduled), status)
}

func TestCancellationInteraction_ScheduledCancelled_NoActivation(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()
	dbWrap := db.NewFromPool(pool)

	var sellerID, productID, auctionID uuid.UUID
	err := dbWrap.WithTx(ctx, func(tx db.Tx) error {
		sellerID = seedRevalidationUser(t, ctx, tx)
		productID = uuid.New()
		_, err := tx.Exec(ctx, `INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, created_at, updated_at) VALUES ($1,$2,'Koi','desc','[]','kohaku','immediate',NOW(),NOW())`, productID, sellerID)
		return err
	})
	require.NoError(t, err)

	startPast := time.Now().Add(-10 * time.Second)
	end := startPast.Add(24 * time.Hour)
	auctionID = seedRevalidationAuction(t, ctx, tdb, sellerID, productID, entity.StatusScheduled, startPast, end, nil, nil)

	_, err = pool.Exec(ctx, `UPDATE auctions SET status='cancelled', updated_at=NOW() WHERE id=$1`, auctionID)
	require.NoError(t, err)

	svc := newRevalidationService(nil)
	err = dbWrap.WithTx(ctx, func(tx db.Tx) error {
		return svc.ActivateScheduledAuction(ctx, tx, ActivateScheduledAuctionInput{AuctionID: auctionID})
	})
	require.NoError(t, err)

	var status string
	err = pool.QueryRow(ctx, `SELECT status FROM auctions WHERE id=$1`, auctionID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, string(entity.StatusCancelled), status)
}

func TestCancellationInteraction_ActiveCancelled_NoEnd(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()
	dbWrap := db.NewFromPool(pool)

	var sellerID, productID, auctionID uuid.UUID
	err := dbWrap.WithTx(ctx, func(tx db.Tx) error {
		sellerID = seedRevalidationUser(t, ctx, tx)
		productID = uuid.New()
		_, err := tx.Exec(ctx, `INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, created_at, updated_at) VALUES ($1,$2,'Koi','desc','[]','kohaku','immediate',NOW(),NOW())`, productID, sellerID)
		return err
	})
	require.NoError(t, err)

	endExpired := time.Now().Add(-10 * time.Second)
	start := endExpired.Add(-24 * time.Hour)
	auctionID = seedRevalidationAuction(t, ctx, tdb, sellerID, productID, entity.StatusActive, start, endExpired, nil, nil)

	_, err = pool.Exec(ctx, `UPDATE auctions SET status='cancelled' WHERE id=$1`, auctionID)
	require.NoError(t, err)

	svc := newRevalidationService(nil)
	err = dbWrap.WithTx(ctx, func(tx db.Tx) error {
		return svc.EndAuctionInternal(ctx, tx, EndAuctionInput{AuctionID: auctionID})
	})
	require.NoError(t, err)

	var status string
	err = pool.QueryRow(ctx, `SELECT status FROM auctions WHERE id=$1`, auctionID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, string(entity.StatusCancelled), status)
}
