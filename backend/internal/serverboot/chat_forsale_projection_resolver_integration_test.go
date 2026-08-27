//go:build integration

package serverboot

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	fpsEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type forSaleProjectionFixture struct {
	appDB    *db.DB
	traced   *db.DB
	tracer   *queryCountingTracer
	resolver *forSaleProjectionBatchResolver
	cleanup  func()
}

func newForSaleProjectionFixture(t *testing.T) *forSaleProjectionFixture {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	ctx := context.Background()

	baseCfg := *tdb.Pool().Config()
	tracer := &queryCountingTracer{}
	baseCfg.ConnConfig.Tracer = tracer

	tracedPool, err := pgxpool.NewWithConfig(ctx, &baseCfg)
	require.NoError(t, err)

	fx := &forSaleProjectionFixture{
		appDB:    db.NewFromPool(tdb.Pool()),
		traced:   db.NewFromPool(tracedPool),
		tracer:   tracer,
		resolver: newForSaleProjectionBatchResolver(db.NewFromPool(tracedPool)),
		cleanup: func() {
			tracedPool.Close()
			cleanup()
		},
	}

	t.Cleanup(fx.cleanup)
	return fx
}

func (f *forSaleProjectionFixture) seedViewer(t *testing.T, username string) uuid.UUID {
	t.Helper()
	return f.seedUser(t, "active", nil, username, nil, nil)
}

func (f *forSaleProjectionFixture) seedUser(
	t *testing.T,
	accountStatus string,
	deletedAt *time.Time,
	username string,
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
			INSERT INTO user_profiles (id, user_id, username, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), NOW())
		`, uuid.New(), id, username)
		require.NoError(t, err)
	}

	if storeName != nil {
		var image any
		if storeImageURL != nil {
			image = *storeImageURL
		}
		_, err = f.appDB.Pool().Exec(context.Background(), `
			INSERT INTO seller_profiles (id, user_id, store_name, store_image_url, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'active', NOW(), NOW())
		`, uuid.New(), id, *storeName, image)
		require.NoError(t, err)
	}

	return id
}

func (f *forSaleProjectionFixture) seedActiveSeller(
	t *testing.T,
	username string,
	storeName string,
	storeImageURL string,
) uuid.UUID {
	t.Helper()

	sellerID := f.seedUser(t, "active", nil, username, &storeName, &storeImageURL)
	paymentID := uuid.New()
	paymentNumber := "sub-" + uuid.NewString()
	midtransOrderID := uuid.NewString()
	referenceID := uuid.New()
	_, err := f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO payments (
			id, user_id, payment_number, midtrans_order_id, gross_amount,
			service_fee_amount, coin_discount_amount, coins_to_use,
			reference_type, reference_id, expired_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, 0, 0, 0, 0, 'subscription', $5, NOW() + INTERVAL '1 year', NOW(), NOW())
	`, paymentID, sellerID, paymentNumber, midtransOrderID, referenceID)
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

func (f *forSaleProjectionFixture) seedProduct(
	t *testing.T,
	sellerID uuid.UUID,
	title string,
	description string,
	mediaURLs []string,
) uuid.UUID {
	t.Helper()

	id := uuid.New()
	now := time.Now().UTC()
	if mediaURLs == nil {
		mediaURLs = []string{}
	}
	raw, err := json.Marshal(mediaURLs)
	require.NoError(t, err)

	_, err = f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO products (
			id, seller_id, title, description, media_urls, variety,
			preparation_time, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
	`, id, sellerID, title, description, raw, "Kohaku", string(fpsEntity.PreparationTimeImmediate), now)
	require.NoError(t, err)

	return id
}

func (f *forSaleProjectionFixture) seedSale(
	t *testing.T,
	sellerID uuid.UUID,
	status fpsEntity.ForSaleStatus,
	visibility fpsEntity.ForSaleVisibility,
	negotiationEnabled bool,
	quantityAvailable int,
	pricePerUnit int64,
	title string,
	productMediaURLs []string,
) uuid.UUID {
	t.Helper()

	productID := f.seedProduct(t, sellerID, title, "fixture description", productMediaURLs)
	id := uuid.New()
	now := time.Now().UTC()
	var err error

	var publishedAt any
	if status == fpsEntity.ForSaleStatusActive && visibility == fpsEntity.ForSaleVisibilityPublic {
		publishedAt = now
	}

	_, err = f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO for_sales (
			id, product_id, seller_id, price_per_unit, negotiation_enabled,
			status, published_at, sold_at, withdrawn_at,
			quantity_available, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULL, NULL, $8, $9, $9)
	`, id, productID, sellerID, pricePerUnit, negotiationEnabled, string(status), publishedAt, quantityAvailable, now)
	require.NoError(t, err)

	return id
}

func (f *forSaleProjectionFixture) seedBlock(t *testing.T, blockerID, blockedID uuid.UUID) {
	t.Helper()

	_, err := f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO user_blocks (blocker_id, blocked_id, created_at)
		VALUES ($1, $2, NOW())
	`, blockerID, blockedID)
	require.NoError(t, err)
}

func (f *forSaleProjectionFixture) resolve(
	ctx context.Context,
	viewerID uuid.UUID,
	occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
) (map[uuid.UUID]*chatApp.ResourceProjection, error) {
	return f.resolver.ResolveForSales(ctx, viewerID, occurrences)
}

func newFPSOccurrence(messageID, saleID uuid.UUID) *chatEntity.ChatMessageResourceOccurrence {
	return chatEntity.NewChatMessageResourceOccurrence(
		messageID,
		chatEntity.ResourceOccurrenceOperationShareToChat,
		chatEntity.ResourceOccurrenceResourceTypeForSale,
		saleID,
		json.RawMessage(`{}`),
	)
}

func requireLiveFPSProjection(t *testing.T, proj *chatApp.ResourceProjection) chatApp.ForSaleLivePayload {
	t.Helper()
	require.NotNil(t, proj)
	require.Equal(t, chatApp.ProjectionStateLive, proj.State)
	require.Equal(t, chatEntity.ResourceOccurrenceResourceTypeForSale, proj.Identity.ResourceType)
	require.NotNil(t, proj.Payload)
	require.NotNil(t, proj.CommerceActions)

	payload, ok := proj.Payload.(chatApp.ForSaleLivePayload)
	require.True(t, ok, "expected ForSaleLivePayload, got %T", proj.Payload)
	require.NotNil(t, payload.Price)
	require.NotNil(t, payload.Seller)
	return payload
}

func requireTombstoneFPSProjection(t *testing.T, proj *chatApp.ResourceProjection) {
	t.Helper()
	require.NotNil(t, proj)
	require.Equal(t, chatApp.ProjectionStateTombstone, proj.State)
	require.Equal(t, chatEntity.ResourceOccurrenceResourceTypeForSale, proj.Identity.ResourceType)
	require.Nil(t, proj.Payload)
	require.Nil(t, proj.CommerceActions)
	assert.False(t, proj.ViewerCapabilities.CanView)
	assert.False(t, proj.ViewerCapabilities.CanInteract)
	assert.True(t, proj.ViewerCapabilities.BlockedByTombstone)
	assert.Equal(t, uuid.Nil, proj.Identity.ResourceID)
}

func TestForSaleProjectionResolver_MixedStatesAndPayloadContract(t *testing.T) {
	fx := newForSaleProjectionFixture(t)
	ctx := context.Background()

	viewerID := fx.seedViewer(t, "viewer")
	blockedByViewerID := fx.seedActiveSeller(t, "blocked-by-viewer", "Blocked Farm", "https://cdn.example.test/blocked-store.jpg")
	fx.seedBlock(t, viewerID, blockedByViewerID)

	activeSellerID := fx.seedActiveSeller(t, "active-seller", "Active Farm", "https://cdn.example.test/active-store.jpg")
	draftSellerID := fx.seedActiveSeller(t, "draft-owner", "Draft Farm", "https://cdn.example.test/draft-store.jpg")
	suspendedSellerID := fx.seedUser(t, "suspended", nil, "suspended-seller", nil, nil)

	activeSaleID := fx.seedSale(
		t,
		activeSellerID,
		fpsEntity.ForSaleStatusActive,
		fpsEntity.ForSaleVisibilityPublic,
		true,
		3,
		1500000,
		"Showa Koi 30cm",
		[]string{"https://cdn.example.test/product-thumb.jpg"},
	)
	soldSaleID := fx.seedSale(
		t,
		activeSellerID,
		fpsEntity.ForSaleStatusSold,
		fpsEntity.ForSaleVisibilityPrivate,
		false,
		0,
		1500000,
		"Sold Koi",
		nil,
	)
	draftSaleID := fx.seedSale(
		t,
		draftSellerID,
		fpsEntity.ForSaleStatusDraft,
		fpsEntity.ForSaleVisibilityPrivate,
		false,
		1,
		900000,
		"Draft Koi",
		nil,
	)
	blockedSaleID := fx.seedSale(
		t,
		blockedByViewerID,
		fpsEntity.ForSaleStatusActive,
		fpsEntity.ForSaleVisibilityPublic,
		true,
		1,
		700000,
		"Blocked Koi",
		nil,
	)
	suspendedSaleID := fx.seedSale(
		t,
		suspendedSellerID,
		fpsEntity.ForSaleStatusActive,
		fpsEntity.ForSaleVisibilityPublic,
		true,
		2,
		800000,
		"Suspended Koi",
		nil,
	)

	occurrences := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newFPSOccurrence(uuid.New(), activeSaleID),
		uuid.New(): newFPSOccurrence(uuid.New(), soldSaleID),
		uuid.New(): newFPSOccurrence(uuid.New(), draftSaleID),
		uuid.New(): newFPSOccurrence(uuid.New(), blockedSaleID),
		uuid.New(): newFPSOccurrence(uuid.New(), suspendedSaleID),
	}

	projections, err := fx.resolve(ctx, viewerID, occurrences)
	require.NoError(t, err)
	require.Len(t, projections, len(occurrences))

	for msgID, occ := range occurrences {
		proj := projections[msgID]
		switch occ.SourceID() {
		case activeSaleID:
			payload := requireLiveFPSProjection(t, proj)
			require.Equal(t, "Showa Koi 30cm", payload.Title)
			require.NotNil(t, payload.ImageURL)
			require.Equal(t, "https://cdn.example.test/product-thumb.jpg", *payload.ImageURL)
			require.Equal(t, int64(1500000), payload.Price.Amount)
			require.Equal(t, "IDR", payload.Price.Currency)
			require.Equal(t, "active", payload.Status)
			require.Equal(t, 3, payload.QuantityAvailable)
			require.Equal(t, activeSellerID, payload.Seller.ID)
			require.Equal(t, "Active Farm", payload.Seller.StoreName)
			require.Equal(t, "active", payload.Seller.Lifecycle)
			require.True(t, proj.CommerceActions.CanChat)
			require.True(t, proj.CommerceActions.CanBuy)
			require.True(t, proj.CommerceActions.CanNegotiate)
			require.True(t, proj.ViewerCapabilities.CanInteract)
		case soldSaleID:
			requireTombstoneFPSProjection(t, proj)
		case draftSaleID:
			requireTombstoneFPSProjection(t, proj)
		case blockedSaleID, suspendedSaleID:
			requireTombstoneFPSProjection(t, proj)
		default:
			t.Fatalf("unexpected source id %s", occ.SourceID())
		}
	}
}

func TestForSaleProjectionResolver_QueryCount_BoundedByDistinctSales(t *testing.T) {
	fx := newForSaleProjectionFixture(t)
	ctx := context.Background()

	viewerID := fx.seedViewer(t, "viewer")
	sellerID := fx.seedActiveSeller(t, "query-seller", "Query Farm", "https://cdn.example.test/query-store.jpg")
	saleID := fx.seedSale(
		t,
		sellerID,
		fpsEntity.ForSaleStatusActive,
		fpsEntity.ForSaleVisibilityPublic,
		true,
		5,
		222000,
		"Query Koi",
		[]string{"https://cdn.example.test/query-product.jpg"},
	)

	one := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newFPSOccurrence(uuid.New(), saleID),
	}
	twentySame := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, 20)
	for i := 0; i < 20; i++ {
		twentySame[uuid.New()] = newFPSOccurrence(uuid.New(), saleID)
	}

	fx.tracer.reset()
	_, err := fx.resolve(ctx, viewerID, one)
	require.NoError(t, err)
	countOne := fx.tracer.value()

	fx.tracer.reset()
	_, err = fx.resolve(ctx, viewerID, twentySame)
	require.NoError(t, err)
	countSame := fx.tracer.value()

	require.Equal(t, countOne, countSame)
	require.Equal(t, int64(4), countOne)
	t.Logf("query counts: one=%d same=%d", countOne, countSame)
}

type forSaleProjectionFailingDB struct {
	base        *db.DB
	failOnQuery int
	err         error
}

func (d *forSaleProjectionFailingDB) WithTx(ctx context.Context, fn func(db.Tx) error) error {
	return d.base.WithTx(ctx, func(tx db.Tx) error {
		return fn(&forSaleProjectionFailingTx{
			Tx:          tx,
			failOnQuery: d.failOnQuery,
			err:         d.err,
		})
	})
}

type forSaleProjectionFailingTx struct {
	db.Tx
	failOnQuery int
	queryCount  int
	err         error
}

func (t *forSaleProjectionFailingTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	t.queryCount++
	if t.queryCount == t.failOnQuery {
		return nil, t.err
	}
	return t.Tx.Query(ctx, sql, args...)
}

var _ forSaleProjectionDB = (*forSaleProjectionFailingDB)(nil)

func TestForSaleProjectionResolver_QueryFailures_Propagate(t *testing.T) {
	fx := newForSaleProjectionFixture(t)
	ctx := context.Background()

	viewerID := fx.seedViewer(t, "viewer")
	sellerID := fx.seedActiveSeller(t, "seller", "Seller Farm", "https://cdn.example.test/seller-store.jpg")
	saleID := fx.seedSale(
		t,
		sellerID,
		fpsEntity.ForSaleStatusActive,
		fpsEntity.ForSaleVisibilityPublic,
		true,
		2,
		100000,
		"Boom Koi",
		nil,
	)

	occurrences := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newFPSOccurrence(uuid.New(), saleID),
	}

	t.Run("sale source query failure", func(t *testing.T) {
		resolver := newForSaleProjectionBatchResolver(&forSaleProjectionFailingDB{
			base:        fx.appDB,
			failOnQuery: 1,
			err:         errors.New("source query boom"),
		})

		projections, err := resolver.ResolveForSales(ctx, viewerID, occurrences)
		require.Error(t, err)
		require.Nil(t, projections)
		assert.Contains(t, err.Error(), "fixed price sale source batch query failed")
	})

	t.Run("block query failure", func(t *testing.T) {
		resolver := newForSaleProjectionBatchResolver(&forSaleProjectionFailingDB{
			base:        fx.appDB,
			failOnQuery: 2,
			err:         errors.New("block query boom"),
		})

		projections, err := resolver.ResolveForSales(ctx, viewerID, occurrences)
		require.Error(t, err)
		require.Nil(t, projections)
		assert.Contains(t, err.Error(), "fixed price sale block batch query failed")
	})
}

func TestForSaleProjectionResolver_MissingSourceRow_IsIntegrityError(t *testing.T) {
	fx := newForSaleProjectionFixture(t)
	ctx := context.Background()

	viewerID := fx.seedViewer(t, "viewer")
	missingSaleID := uuid.New()
	occurrences := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newFPSOccurrence(uuid.New(), missingSaleID),
	}

	projections, err := fx.resolve(ctx, viewerID, occurrences)
	require.Error(t, err)
	require.Nil(t, projections)
	assert.Contains(t, err.Error(), "fixed price sale source row missing")
}
