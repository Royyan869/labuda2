//go:build integration

package serverboot

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	fpsEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/internal/social/content/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
)

type aggregateQueryProofFixture struct {
	appDB    *db.DB
	traced   *db.DB
	tracer   *queryCountingTracer
	resolver *resourceProjectionAggregateResolver
	cleanup  func()
}

func newAggregateQueryProofFixture(t *testing.T) *aggregateQueryProofFixture {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	ctx := context.Background()

	baseCfg := *tdb.Pool().Config()
	tracer := &queryCountingTracer{}
	baseCfg.ConnConfig.Tracer = tracer

	tracedPool, err := pgxpool.NewWithConfig(ctx, &baseCfg)
	require.NoError(t, err)

	tracedDB := db.NewFromPool(tracedPool)
	fx := &aggregateQueryProofFixture{
		appDB:  db.NewFromPool(tdb.Pool()),
		traced: tracedDB,
		tracer: tracer,
		resolver: newResourceProjectionAggregateResolver(
			newProfileProjectionBatchResolver(tracedDB),
			newContentProjectionBatchResolver(tracedDB),
			newForSaleProjectionBatchResolver(tracedDB),
			newAuctionProjectionBatchResolver(tracedDB),
		),
		cleanup: func() {
			tracedPool.Close()
			cleanup()
		},
	}

	t.Cleanup(fx.cleanup)
	return fx
}

func (f *aggregateQueryProofFixture) seedUser(
	t *testing.T,
	accountStatus string,
	deletedAt *time.Time,
	username string,
	avatarURL *string,
	storeName *string,
	storeImageURL *string,
) uuid.UUID {
	t.Helper()

	id := uuid.New()
	now := time.Now().UTC()
	_, err := f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO users (id, firebase_uid, email, account_status, deleted_at, created_at, updated_at, role)
		VALUES ($1, $2, $3, $4, $5, $6, $6, 'user')
	`, id, id.String(), id.String()+"@test.local", accountStatus, deletedAt, now)
	require.NoError(t, err)

	if username != "" {
		_, err = f.appDB.Pool().Exec(context.Background(), `
			INSERT INTO user_profiles (id, user_id, username, avatar_url, created_at, updated_at)
			VALUES ($1, $2, $3, $4, NOW(), NOW())
		`, uuid.New(), id, username, avatarURL)
		require.NoError(t, err)
	}

	if storeName != nil {
		var image any
		if storeImageURL != nil {
			image = *storeImageURL
		}
		_, err = f.appDB.Pool().Exec(context.Background(), `
			INSERT INTO seller_profiles (id, user_id, store_name, store_image_url, tier, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'basic', 'active', NOW(), NOW())
		`, uuid.New(), id, *storeName, image)
		require.NoError(t, err)
	}

	return id
}

func (f *aggregateQueryProofFixture) seedActiveSeller(t *testing.T, username, storeName string) uuid.UUID {
	t.Helper()

	sellerID := f.seedUser(t, "active", nil, username, nil, &storeName, nil)
	paymentID := uuid.New()
	paymentNumber := "sub-" + uuid.NewString()
	midtransOrderID := uuid.NewString()
	referenceID := uuid.New()
	now := time.Now().UTC()
	_, err := f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO payments (
			id, user_id, payment_number, midtrans_order_id, gross_amount,
			service_fee_amount, coin_discount_amount, coins_to_use,
			reference_type, reference_id, expired_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, 0, 0, 0, 0, 'subscription', $5, NOW() + INTERVAL '1 year', $6, $6)
	`, paymentID, sellerID, paymentNumber, midtransOrderID, referenceID, now)
	require.NoError(t, err)

	_, err = f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO seller_subscriptions (
			id, user_id, status, started_at, expires_at, duration_days,
			amount_paid, payment_id, created_at, updated_at
		)
		VALUES ($1, $2, 'active', NOW(), NOW() + INTERVAL '1 year', 365, 0, $3, NOW(), NOW())
	`, uuid.New(), sellerID, paymentID)
	require.NoError(t, err)

	return sellerID
}

func (f *aggregateQueryProofFixture) seedContent(
	t *testing.T,
	authorID uuid.UUID,
	title string,
	visibility entity.Visibility,
) uuid.UUID {
	t.Helper()

	contentID := uuid.New()
	now := time.Now().UTC()
	caption := title
	_, err := f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO contents (
			id, author_id, status, caption, visibility, is_hidden,
			original_author_id, created_at, updated_at, deleted_at
		)
		VALUES ($1, $2, $3, $4, $5, false, NULL, $6, $6, NULL)
	`, contentID, authorID, string(entity.StatusActive), &caption, string(visibility), now)
	require.NoError(t, err)

	return contentID
}

func (f *aggregateQueryProofFixture) seedSale(
	t *testing.T,
	sellerID uuid.UUID,
	title string,
) uuid.UUID {
	t.Helper()

	productID := uuid.New()
	now := time.Now().UTC()
	_, err := f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO products (
			id, seller_id, title, description, media_urls, variety,
			preparation_time, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, '[]'::jsonb, 'Kohaku', $5, $6, $6)
	`, productID, sellerID, title+" product", title+" product", string(fpsEntity.PreparationTimeImmediate), now)
	require.NoError(t, err)

	saleID := uuid.New()
	_, err = f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO for_sales (
			id, product_id, seller_id, price_per_unit, negotiation_enabled,
			status, published_at, sold_at, withdrawn_at,
			quantity_available, created_at, updated_at
		)
		VALUES ($1, $2, $3, 1500000, true, 'active', $4, NULL, NULL, 1, $5, $5)
	`, saleID, productID, sellerID, now, now)
	require.NoError(t, err)

	return saleID
}

func (f *aggregateQueryProofFixture) seedAuction(
	t *testing.T,
	sellerID uuid.UUID,
	title string,
) uuid.UUID {
	t.Helper()

	productID := uuid.New()
	now := time.Now().UTC()
	_, err := f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO products (
			id, seller_id, title, description, media_urls, variety,
			preparation_time, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, '[]'::jsonb, 'Kohaku', $5, $6, $6)
	`, productID, sellerID, title+" product", title+" product", string(fpsEntity.PreparationTimeImmediate), now)
	require.NoError(t, err)

	buyNow := int64(1750000)
	startAt := now.Add(-1 * time.Hour)
	endAt := now.Add(1 * time.Hour)
	auctionID := uuid.New()
	_, err = f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO auctions (
			id, seller_id, product_id, order_id, settlement_deadline,
			start_price, bid_increment, buy_now_price, start_at, end_at, current_bid,
			current_winner_id, status, created_at,
			updated_at, anti_snipe_extension_seconds
		)
		VALUES ($1, $2, $3, NULL, NULL, 100000, 5000, $4, $5, $6, NULL, NULL, $7, $8, $8, 0)
	`, auctionID, sellerID, productID, buyNow, startAt, endAt, string(auctionEntity.StatusActive), now)
	require.NoError(t, err)

	return auctionID
}

func (f *aggregateQueryProofFixture) resolve(
	ctx context.Context,
	viewerID uuid.UUID,
	occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
) (map[uuid.UUID]*chatApp.ResourceProjection, error) {
	return f.resolver.ResolveResourceProjections(ctx, viewerID, occurrences)
}

func uniqueUsername(prefix string) string {
	return prefix + "-" + uuid.NewString()
}

func TestResourceProjectionAggregateResolver_IntegrationQueryCounts(t *testing.T) {
	fx := newAggregateQueryProofFixture(t)
	ctx := context.Background()
	viewerID := fx.seedUser(t, "active", nil, uniqueUsername("viewer"), nil, nil, nil)

	profileTargetID := fx.seedUser(t, "active", nil, uniqueUsername("profile-target"), nil, nil, nil)
	contentAuthorID := fx.seedUser(t, "active", nil, uniqueUsername("content-author"), nil, nil, nil)
	fpsSellerID := fx.seedActiveSeller(t, uniqueUsername("fps-seller"), "FPS Farm")
	auctionSellerID := fx.seedActiveSeller(t, uniqueUsername("auction-seller"), "Auction Farm")

	sharedProfileID := fx.seedUser(t, "active", nil, uniqueUsername("shared-profile"), nil, nil, nil)
	sharedContentAuthorID := fx.seedUser(t, "active", nil, uniqueUsername("shared-content-author"), nil, nil, nil)
	sharedFpsSellerID := fx.seedActiveSeller(t, uniqueUsername("shared-fps-seller"), "Shared FPS Farm")
	sharedAuctionSellerID := fx.seedActiveSeller(t, uniqueUsername("shared-auction-seller"), "Shared Auction Farm")

	q0 := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{}

	q1 := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeProfile, []uuid.UUID{profileTargetID})
	q2 := buildRepeatedOccurrences(chatEntity.ResourceOccurrenceResourceTypeProfile, sharedProfileID, 20)

	q3 := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeContent, []uuid.UUID{
		fx.seedContent(t, contentAuthorID, "content-one", entity.VisibilityPublic),
	})
	q4 := buildRepeatedOccurrences(chatEntity.ResourceOccurrenceResourceTypeContent, fx.seedContent(t, sharedContentAuthorID, "content-shared", entity.VisibilityPublic), 20)

	q5SaleID := fx.seedSale(t, fpsSellerID, "fps-one")
	q5 := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeForSale, []uuid.UUID{q5SaleID})
	q6 := buildRepeatedOccurrences(chatEntity.ResourceOccurrenceResourceTypeForSale, fx.seedSale(t, sharedFpsSellerID, "fps-shared"), 20)

	q7AuctionID := fx.seedAuction(t, auctionSellerID, "auction-one")
	q7 := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeAuction, []uuid.UUID{q7AuctionID})
	q8 := buildRepeatedOccurrences(chatEntity.ResourceOccurrenceResourceTypeAuction, fx.seedAuction(t, sharedAuctionSellerID, "auction-shared"), 20)

	q9 := combineOccurrenceMaps(q1, q3, q5, q7)

	q10ProfileIDs := make([]uuid.UUID, 20)
	q10ContentIDs := make([]uuid.UUID, 20)
	q10FpsIDs := make([]uuid.UUID, 20)
	q10AuctionIDs := make([]uuid.UUID, 20)

	for i := range q10ProfileIDs {
		q10ProfileIDs[i] = fx.seedUser(t, "active", nil, uniqueUsername("q10-profile"), nil, nil, nil)
	}
	q10ProfileOccurrences := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeProfile, q10ProfileIDs)

	for i := range q10ContentIDs {
		authorID := fx.seedUser(t, "active", nil, uniqueUsername("q10-content-author"), nil, nil, nil)
		contentID := fx.seedContent(t, authorID, uniqueUsername("q10-content"), entity.VisibilityPublic)
		q10ContentIDs[i] = contentID
	}
	q10ContentOccurrences := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeContent, q10ContentIDs)

	for i := range q10FpsIDs {
		sellerID := fx.seedActiveSeller(t, uniqueUsername("q10-fps-seller"), "Q10 FPS Farm")
		q10FpsIDs[i] = fx.seedSale(t, sellerID, uniqueUsername("q10-fps"))
	}
	q10FpsOccurrences := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeForSale, q10FpsIDs)

	for i := range q10AuctionIDs {
		sellerID := fx.seedActiveSeller(t, uniqueUsername("q10-auction-seller"), "Q10 Auction Farm")
		q10AuctionIDs[i] = fx.seedAuction(t, sellerID, uniqueUsername("q10-auction"))
	}
	q10AuctionOccurrences := buildOccurrences(chatEntity.ResourceOccurrenceResourceTypeAuction, q10AuctionIDs)

	q10 := combineOccurrenceMaps(q10ProfileOccurrences, q10ContentOccurrences, q10FpsOccurrences, q10AuctionOccurrences)

	q11 := combineOccurrenceMaps(
		buildRepeatedOccurrences(chatEntity.ResourceOccurrenceResourceTypeProfile, q1SourceID(q1), 20),
		buildRepeatedOccurrences(chatEntity.ResourceOccurrenceResourceTypeContent, q3SourceID(q3), 20),
		buildRepeatedOccurrences(chatEntity.ResourceOccurrenceResourceTypeForSale, q5SourceID(q5), 20),
		buildRepeatedOccurrences(chatEntity.ResourceOccurrenceResourceTypeAuction, q7SourceID(q7), 20),
	)

	measure := func(name string, occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence) int64 {
		t.Helper()
		fx.tracer.reset()
		got, err := fx.resolve(ctx, viewerID, occurrences)
		require.NoError(t, err, name)
		require.Len(t, got, len(occurrences), name)
		for messageID, projection := range got {
			assertProjectionMatchesOccurrence(t, projection, occurrences[messageID])
		}
		count := fx.tracer.value()
		t.Logf("%s=%d", name, count)
		return count
	}

	countQ0 := measure("Q0", q0)
	countQ1 := measure("Q1", q1)
	countQ2 := measure("Q2", q2)
	countQ3 := measure("Q3", q3)
	countQ4 := measure("Q4", q4)
	countQ5 := measure("Q5", q5)
	countQ6 := measure("Q6", q6)
	countQ7 := measure("Q7", q7)
	countQ8 := measure("Q8", q8)
	countQ9 := measure("Q9", q9)
	countQ10 := measure("Q10", q10)
	countQ11 := measure("Q11", q11)

	require.Equal(t, int64(0), countQ0)
	require.Equal(t, countQ1, countQ2)
	require.Equal(t, countQ3, countQ4)
	require.Equal(t, countQ5, countQ6)
	require.Equal(t, countQ7, countQ8)
	require.Equal(t, countQ9, countQ10)
	require.Equal(t, countQ10, countQ11)
}

func q1SourceID(occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence) uuid.UUID {
	for _, occurrence := range occurrences {
		return occurrence.SourceID()
	}
	return uuid.Nil
}

func q3SourceID(occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence) uuid.UUID {
	return q1SourceID(occurrences)
}

func q5SourceID(occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence) uuid.UUID {
	return q1SourceID(occurrences)
}

func q7SourceID(occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence) uuid.UUID {
	return q1SourceID(occurrences)
}
