//go:build integration

package serverboot

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	fpsEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	commerceshared "github.com/labuda/backend/internal/commerce/shared"
	"github.com/labuda/backend/internal/governance/viewercontext"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
)

type auctionProjectionFixture struct {
	appDB    *db.DB
	traced   *db.DB
	tracer   *queryCountingTracer
	resolver *auctionProjectionBatchResolver
	cleanup  func()
}

func newAuctionProjectionFixture(t *testing.T) *auctionProjectionFixture {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	ctx := context.Background()

	baseCfg := *tdb.Pool().Config()
	tracer := &queryCountingTracer{}
	baseCfg.ConnConfig.Tracer = tracer

	tracedPool, err := pgxpool.NewWithConfig(ctx, &baseCfg)
	require.NoError(t, err)

	fx := &auctionProjectionFixture{
		appDB:    db.NewFromPool(tdb.Pool()),
		traced:   db.NewFromPool(tracedPool),
		tracer:   tracer,
		resolver: newAuctionProjectionBatchResolver(db.NewFromPool(tracedPool)),
		cleanup: func() {
			tracedPool.Close()
			cleanup()
		},
	}

	t.Cleanup(fx.cleanup)
	return fx
}

func (f *auctionProjectionFixture) seedUser(
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
			INSERT INTO seller_profiles (id, user_id, store_name, store_image_url, tier, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'basic', 'active', NOW(), NOW())
		`, uuid.New(), id, *storeName, image)
		require.NoError(t, err)
	}

	return id
}

func (f *auctionProjectionFixture) seedSellerWithSubscriptionStatus(
	t *testing.T,
	accountStatus string,
	deletedAt *time.Time,
	username string,
	storeName string,
	storeImageURL *string,
	subscriptionStatus string,
) uuid.UUID {
	t.Helper()

	sellerID := f.seedUser(t, accountStatus, deletedAt, username, &storeName, storeImageURL)
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
		VALUES ($1, $2, $3, NOW(), NOW() + INTERVAL '1 year', 365, 0, $4, NOW(), NOW())
	`, uuid.New(), sellerID, subscriptionStatus, paymentID)
	require.NoError(t, err)

	return sellerID
}

func uniqueAuctionUsername(prefix string) string {
	return prefix + "-" + uuid.NewString()
}

func (f *auctionProjectionFixture) seedProduct(
	t *testing.T,
	sellerID uuid.UUID,
	title string,
	description string,
	mediaURLs []string,
) uuid.UUID {
	t.Helper()

	if mediaURLs == nil {
		mediaURLs = []string{}
	}
	raw, err := json.Marshal(mediaURLs)
	require.NoError(t, err)

	id := uuid.New()
	now := time.Now().UTC()
	_, err = f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO products (
			id, seller_id, title, description, media_urls, variety,
			preparation_time, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
	`, id, sellerID, title, description, raw, "Kohaku", string(fpsEntity.PreparationTime1To3Days), now)
	require.NoError(t, err)

	return id
}

func (f *auctionProjectionFixture) seedAuction(
	t *testing.T,
	sellerID uuid.UUID,
	status auctionEntity.Status,
	buyNowPrice *int64,
	title string,
	description string,
	productMediaURLs []string,
) uuid.UUID {
	t.Helper()

	productID := f.seedProduct(t, sellerID, title+" product", description+" product", productMediaURLs)
	id := uuid.New()
	now := time.Now().UTC()
	startAt := now.Add(-1 * time.Hour)
	endAt := now.Add(1 * time.Hour)
	_, err := f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO auctions (
			id, seller_id, order_id,
			start_price, bid_increment, buy_now_price,
			start_at, end_at, current_bid, current_winner_id, status, created_at, updated_at,
			product_id, anti_snipe_extension_seconds
		)
		VALUES (
			$1, $2, NULL,
			$3, $4, $5,
			$6, $7, NULL, NULL, $8, $9, $9,
			$10, 0
		)
	`, id, sellerID, int64(100000), int64(5000), buyNowPrice, startAt, endAt, string(status), now, productID)
	require.NoError(t, err)

	return id
}

func (f *auctionProjectionFixture) seedBlock(t *testing.T, blockerID, blockedID uuid.UUID) {
	t.Helper()

	_, err := f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO user_blocks (blocker_id, blocked_id, created_at)
		VALUES ($1, $2, NOW())
	`, blockerID, blockedID)
	require.NoError(t, err)
}

func (f *auctionProjectionFixture) resolve(
	ctx context.Context,
	viewerID uuid.UUID,
	occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
) (map[uuid.UUID]*chatApp.ResourceProjection, error) {
	return f.resolver.ResolveAuctions(ctx, viewerID, occurrences)
}

func newAuctionOccurrenceWithOperation(
	messageID, auctionID uuid.UUID,
	op chatEntity.ResourceOccurrenceOperation,
) *chatEntity.ChatMessageResourceOccurrence {
	return chatEntity.NewChatMessageResourceOccurrence(
		messageID,
		op,
		chatEntity.ResourceOccurrenceResourceTypeAuction,
		auctionID,
		json.RawMessage(`{}`),
	)
}

func makeRepeatedAuctionOccurrences(
	auctionID uuid.UUID,
	count int,
	op chatEntity.ResourceOccurrenceOperation,
) map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence {
	occurrences := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, count)
	for i := 0; i < count; i++ {
		occurrences[uuid.New()] = newAuctionOccurrenceWithOperation(uuid.New(), auctionID, op)
	}
	return occurrences
}

func requireLiveAuctionProjection(t *testing.T, proj *chatApp.ResourceProjection) commerceshared.AuctionLivePayload {
	t.Helper()
	require.NotNil(t, proj)
	require.Equal(t, chatApp.ProjectionStateLive, proj.State)
	require.Equal(t, string(chatEntity.ResourceOccurrenceResourceTypeAuction), string(proj.ResourceType))
	require.NotNil(t, proj.Auction)
	require.True(t, proj.ViewerCapabilities.CanView)
	require.False(t, proj.ViewerCapabilities.BlockedByTombstone)

	return *proj.Auction
}

func requireTombstoneAuctionProjection(t *testing.T, proj *chatApp.ResourceProjection) {
	t.Helper()
	require.NotNil(t, proj)
	require.Equal(t, chatApp.ProjectionStateTombstone, proj.State)
	require.Equal(t, string(chatEntity.ResourceOccurrenceResourceTypeAuction), string(proj.ResourceType))
	// Canonical contract: the resource id survives death.
	require.NotEqual(t, uuid.Nil, proj.ResourceID)
	require.Nil(t, proj.Auction)
	require.Nil(t, proj.ForSale)
	require.Nil(t, proj.Profile)
	require.Nil(t, proj.Content)
	require.False(t, proj.ViewerCapabilities.CanView)
	require.False(t, proj.ViewerCapabilities.CanInteract)
	require.True(t, proj.ViewerCapabilities.BlockedByTombstone)
}

func assertAuctionProjectionMatchesAuthority(
	t *testing.T,
	proj *chatApp.ResourceProjection,
	viewerID uuid.UUID,
	auctionID uuid.UUID,
	sellerID uuid.UUID,
	status auctionEntity.Status,
	sellerAccountStatus string,
	sellerDeleted bool,
	subscriptionStatus string,
	blocked bool,
	buyNowPrice *int64,
) {
	t.Helper()

	viewAllowed := commerceshared.EvaluateAuctionViewAccess(commerceshared.AuctionViewAccessInput{
		ViewerID: viewerID,
		SellerID: sellerID,
		Status:   string(status),
		Blocked:  blocked,
		Seller: commerceshared.SellerAccessSnapshot{
			AccountStatus:      sellerAccountStatus,
			IsDeleted:          sellerDeleted,
			SubscriptionStatus: subscriptionStatus,
		},
	})
	if !viewAllowed {
		requireTombstoneAuctionProjection(t, proj)
		return
	}

	_ = requireLiveAuctionProjection(t, proj)
	require.Equal(t, auctionID, proj.ResourceID)

	expectedCaps := commerceshared.EvaluateAuctionViewerCapabilities(commerceshared.AuctionViewerCapabilitiesInput{
		ViewerID:          viewerID,
		SellerID:          sellerID,
		Status:            string(status),
		SellerTrustActive: viewercontext.CoarsenSellerTrust(subscriptionStatus) == viewercontext.PublicLifecycleStateActive,
		BuyNowPrice:       buyNowPrice,
	})
	require.Equal(t, expectedCaps.CanBid || expectedCaps.CanBuyNow, proj.ViewerCapabilities.CanInteract)
	// Canonical Commerce ownership projected for the viewer (owner vs buyer).
	require.Equal(t, expectedCaps.CanManage, proj.ViewerCapabilities.CanManage)
}

func assertAuctionLivePayloadContract(
	t *testing.T,
	payload commerceshared.AuctionLivePayload,
	wantTitle string,
	wantThumbnail string,
	wantCurrentBid *int64,
	wantBuyNowPrice *int64,
	wantEndAt string,
	wantSellerID uuid.UUID,
	wantSellerUsername string,
	wantSellerFarmName string,
	wantAuctionLifecycle string,
	wantHasWinner bool,
	wantSellerLifecycle string,
	wantUserLifecycle string,
) {
	t.Helper()

	require.Equal(t, wantTitle, payload.Title)
	require.Equal(t, wantCurrentBid, payload.CurrentBid)
	require.Equal(t, wantBuyNowPrice, payload.BuyNowPrice)
	require.Equal(t, wantEndAt, payload.EndAt)

	if wantThumbnail == "" {
		require.Nil(t, payload.ThumbnailURL)
		require.Empty(t, payload.Media)
	} else {
		require.NotNil(t, payload.ThumbnailURL)
		require.Equal(t, wantThumbnail, *payload.ThumbnailURL)
		require.Len(t, payload.Media, 1)
		require.Equal(t, wantThumbnail, payload.Media[0].URL)
	}

	require.Equal(t, wantAuctionLifecycle, payload.Lifecycle)
	require.NotNil(t, payload.HasWinner)
	require.Equal(t, wantHasWinner, *payload.HasWinner)

	require.Equal(t, wantSellerID, payload.Seller.User.ID)
	require.Equal(t, wantSellerUsername, payload.Seller.User.Username)
	if wantUserLifecycle == "" {
		require.Nil(t, payload.Seller.User.Lifecycle)
	} else {
		require.NotNil(t, payload.Seller.User.Lifecycle)
		require.Equal(t, wantUserLifecycle, *payload.Seller.User.Lifecycle)
	}
	if wantSellerFarmName == "" {
		require.Nil(t, payload.Seller.FarmName)
	} else {
		require.NotNil(t, payload.Seller.FarmName)
		require.Equal(t, wantSellerFarmName, *payload.Seller.FarmName)
	}
	require.Equal(t, wantSellerLifecycle, derefString(payload.Seller.Lifecycle))
	require.Equal(t, wantUserLifecycle, derefString(payload.Seller.User.Lifecycle))
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func TestAuctionProjectionResolver_OperationParity(t *testing.T) {
	fx := newAuctionProjectionFixture(t)
	ctx := context.Background()

	viewerID := fx.seedSellerWithSubscriptionStatus(t, "active", nil, uniqueAuctionUsername("viewer"), "Viewer Farm", nil, "active")
	sellerID := fx.seedSellerWithSubscriptionStatus(t, "active", nil, uniqueAuctionUsername("seller"), "Seller Farm", nil, "active")
	buyNow := int64(1500000)
	auctionID := fx.seedAuction(
		t,
		sellerID,
		auctionEntity.StatusActive,
		&buyNow,
		"Showa Koi 30cm",
		"Showa Koi 30cm",
		[]string{"https://cdn.example.test/product.jpg"},
	)

	shareProjection, err := fx.resolve(ctx, viewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newAuctionOccurrenceWithOperation(uuid.New(), auctionID, chatEntity.ResourceOccurrenceOperationShareToChat),
	})
	require.NoError(t, err)

	directProjection, err := fx.resolve(ctx, viewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newAuctionOccurrenceWithOperation(uuid.New(), auctionID, chatEntity.ResourceOccurrenceOperationDirectCommerceInsertChat),
	})
	require.NoError(t, err)

	var shareProj *chatApp.ResourceProjection
	for _, proj := range shareProjection {
		shareProj = proj
	}
	var directProj *chatApp.ResourceProjection
	for _, proj := range directProjection {
		directProj = proj
	}

	require.NotNil(t, shareProj)
	require.NotNil(t, directProj)
	require.Equal(t, shareProj.State, directProj.State)
	require.Equal(t, shareProj.ResourceType, directProj.ResourceType)
	require.Equal(t, shareProj.ResourceID, directProj.ResourceID)
	require.Equal(t, shareProj.ViewerCapabilities, directProj.ViewerCapabilities)

	shareJSON, err := json.Marshal(shareProj)
	require.NoError(t, err)
	directJSON, err := json.Marshal(directProj)
	require.NoError(t, err)
	require.JSONEq(t, string(shareJSON), string(directJSON))
}

func TestAuctionProjectionResolver_LivePayloadContract(t *testing.T) {
	fx := newAuctionProjectionFixture(t)
	ctx := context.Background()

	viewerID := fx.seedSellerWithSubscriptionStatus(t, "active", nil, uniqueAuctionUsername("viewer"), "Viewer Farm", nil, "active")
	sellerUsername := uniqueAuctionUsername("seller-live")
	sellerID := fx.seedSellerWithSubscriptionStatus(t, "active", nil, sellerUsername, "Seller Live Farm", nil, "active")
	avatarURL := "https://cdn.example.test/seller-avatar.jpg"
	_, err := fx.appDB.Pool().Exec(ctx, `
		UPDATE user_profiles
		SET avatar_url = $2, updated_at = NOW()
		WHERE user_id = $1
	`, sellerID, avatarURL)
	require.NoError(t, err)

	buyNow := int64(1750000)
	currentBid := int64(1425000)
	auctionID := fx.seedAuction(
		t,
		sellerID,
		auctionEntity.StatusActive,
		&buyNow,
		"Showa Koi 30cm",
		"Showa Koi 30cm",
		[]string{"https://cdn.example.test/product.jpg"},
	)
	_, err = fx.appDB.Pool().Exec(ctx, `
		UPDATE auctions
		SET current_bid = $2, updated_at = NOW()
		WHERE id = $1
	`, auctionID, currentBid)
	require.NoError(t, err)

	var endAt time.Time
	require.NoError(t, fx.appDB.Pool().QueryRow(ctx, `SELECT end_at FROM auctions WHERE id = $1`, auctionID).Scan(&endAt))

	projections, err := fx.resolve(ctx, viewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newAuctionOccurrenceWithOperation(uuid.New(), auctionID, chatEntity.ResourceOccurrenceOperationShareToChat),
	})
	require.NoError(t, err)
	require.Len(t, projections, 1)

	var proj *chatApp.ResourceProjection
	for _, p := range projections {
		proj = p
	}
	payload := requireLiveAuctionProjection(t, proj)
	assertAuctionLivePayloadContract(
		t,
		payload,
		// seedAuction mints the product title with a " product" suffix; the
		// canonical projection Title sources p.title (parity with FPS).
		"Showa Koi 30cm product",
		"https://cdn.example.test/product.jpg",
		&currentBid,
		&buyNow,
		endAt.Format(time.RFC3339),
		sellerID,
		sellerUsername,
		"Seller Live Farm",
		"active",
		false, // active phase → no winner yet
		"active",
		"active",
	)
	require.NotNil(t, payload.Seller.User.AvatarURL)
	require.Equal(t, avatarURL, *payload.Seller.User.AvatarURL)
	require.Equal(t, avatarURL, derefString(payload.Seller.AvatarURL))

	require.Equal(t, chatApp.ProjectionStateLive, proj.State)
	require.True(t, proj.ViewerCapabilities.CanInteract)

	expiredSellerUsername := uniqueAuctionUsername("seller-expired")
	expiredSellerID := fx.seedSellerWithSubscriptionStatus(t, "active", nil, expiredSellerUsername, "Seller Expired Farm", nil, "expired")
	expiredAuctionID := fx.seedAuction(
		t,
		expiredSellerID,
		auctionEntity.StatusActive,
		nil,
		"Expired Trust Auction",
		"Expired Trust Auction",
		nil,
	)
	var expiredEndAt time.Time
	require.NoError(t, fx.appDB.Pool().QueryRow(ctx, `SELECT end_at FROM auctions WHERE id = $1`, expiredAuctionID).Scan(&expiredEndAt))

	projections, err = fx.resolve(ctx, viewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newAuctionOccurrenceWithOperation(uuid.New(), expiredAuctionID, chatEntity.ResourceOccurrenceOperationShareToChat),
	})
	require.NoError(t, err)
	require.Len(t, projections, 1)

	for _, p := range projections {
		proj = p
	}
	payload = requireLiveAuctionProjection(t, proj)
	assertAuctionLivePayloadContract(
		t,
		payload,
		"Expired Trust Auction product",
		"",
		nil,
		nil,
		expiredEndAt.Format(time.RFC3339),
		expiredSellerID,
		expiredSellerUsername,
		"Seller Expired Farm",
		"active",
		false, // active phase → no winner yet
		"unavailable",
		"active",
	)
	// Seller trust expired: the viewer has no actionable capability, so the
	// envelope-level can_interact is false.
	require.False(t, proj.ViewerCapabilities.CanInteract)
}

func TestAuctionProjectionResolver_TombstonePrivacy(t *testing.T) {
	fx := newAuctionProjectionFixture(t)
	ctx := context.Background()

	viewerID := fx.seedSellerWithSubscriptionStatus(t, "active", nil, uniqueAuctionUsername("viewer"), "Viewer Farm", nil, "active")
	sellerID := fx.seedSellerWithSubscriptionStatus(t, "active", nil, uniqueAuctionUsername("seller"), "Seller Farm", nil, "active")
	auctionID := fx.seedAuction(
		t,
		sellerID,
		auctionEntity.StatusLapsed,
		nil,
		"Private Auction",
		"Private Auction",
		nil,
	)

	projections, err := fx.resolve(ctx, viewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newAuctionOccurrenceWithOperation(uuid.New(), auctionID, chatEntity.ResourceOccurrenceOperationShareToChat),
	})
	require.NoError(t, err)

	var proj *chatApp.ResourceProjection
	for _, p := range projections {
		proj = p
	}
	requireTombstoneAuctionProjection(t, proj)

	got := projectionJSONMap(t, proj)
	requireTopLevelKeys(t, got, []string{"state", "resource_type", "resource_id", "viewer_capabilities"})
	requireAbsentKeys(t, got, []string{"canonical_url", "commerce_actions", "auction", "for_sale", "profile", "content"})
	// Canonical contract: identity survives death (chat's omission rule died).
	require.Equal(t, auctionID.String(), mustStringValue(t, got["resource_id"]))
}

func TestAuctionProjectionResolver_AuthorityParityMatrix(t *testing.T) {
	fx := newAuctionProjectionFixture(t)
	ctx := context.Background()

	viewerID := fx.seedSellerWithSubscriptionStatus(t, "active", nil, uniqueAuctionUsername("viewer"), "Viewer Farm", nil, "active")
	blockedViewerID := fx.seedSellerWithSubscriptionStatus(t, "active", nil, uniqueAuctionUsername("blocked-viewer"), "Blocked Viewer Farm", nil, "active")
	sellerActiveID := fx.seedSellerWithSubscriptionStatus(t, "active", nil, uniqueAuctionUsername("seller-active"), "Seller Active Farm", nil, "active")
	sellerInactiveTrustID := fx.seedSellerWithSubscriptionStatus(t, "active", nil, uniqueAuctionUsername("seller-inactive"), "Seller Inactive Farm", nil, "expired")
	sellerRemovedDeletedAt := time.Now().UTC()
	sellerRemovedID := fx.seedSellerWithSubscriptionStatus(t, "active", &sellerRemovedDeletedAt, uniqueAuctionUsername("seller-removed"), "Seller Removed Farm", nil, "active")
	fx.seedBlock(t, viewerID, blockedViewerID)

	buyNow := int64(1750000)
	cases := []struct {
		name             string
		viewerID         uuid.UUID
		sellerID         uuid.UUID
		status           auctionEntity.Status
		buyNowPrice      *int64
		blocked          bool
		sellerAccount    string
		sellerDeleted    bool
		subscriptionStat string
		wantLive         bool
	}{
		{
			name:             "live active auction",
			viewerID:         viewerID,
			sellerID:         sellerActiveID,
			status:           auctionEntity.StatusActive,
			buyNowPrice:      &buyNow,
			sellerAccount:    "active",
			subscriptionStat: "active",
			wantLive:         true,
		},
		{
			name:             "live scheduled auction owner",
			viewerID:         sellerActiveID,
			sellerID:         sellerActiveID,
			status:           auctionEntity.StatusScheduled,
			buyNowPrice:      &buyNow,
			sellerAccount:    "active",
			subscriptionStat: "active",
			wantLive:         true,
		},
		{
			name:             "live guest active auction",
			viewerID:         uuid.Nil,
			sellerID:         sellerActiveID,
			status:           auctionEntity.StatusActive,
			buyNowPrice:      nil,
			sellerAccount:    "active",
			subscriptionStat: "active",
			wantLive:         true,
		},
		{
			name:             "tombstone lapsed non-owner",
			viewerID:         viewerID,
			sellerID:         sellerActiveID,
			status:           auctionEntity.StatusLapsed,
			buyNowPrice:      &buyNow,
			sellerAccount:    "active",
			subscriptionStat: "active",
			wantLive:         false,
		},
		{
			name:             "tombstone blocked viewer",
			viewerID:         blockedViewerID,
			sellerID:         sellerActiveID,
			status:           auctionEntity.StatusActive,
			buyNowPrice:      &buyNow,
			blocked:          true,
			sellerAccount:    "active",
			subscriptionStat: "active",
			wantLive:         false,
		},
		{
			name:             "tombstone removed seller",
			viewerID:         viewerID,
			sellerID:         sellerRemovedID,
			status:           auctionEntity.StatusActive,
			buyNowPrice:      &buyNow,
			sellerAccount:    "active",
			sellerDeleted:    true,
			subscriptionStat: "active",
			wantLive:         false,
		},
		{
			name:             "live trust inactive buyer with no buy now",
			viewerID:         viewerID,
			sellerID:         sellerInactiveTrustID,
			status:           auctionEntity.StatusActive,
			buyNowPrice:      nil,
			sellerAccount:    "active",
			subscriptionStat: "expired",
			wantLive:         true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			auctionID := fx.seedAuction(
				t,
				tc.sellerID,
				tc.status,
				tc.buyNowPrice,
				tc.name,
				tc.name,
				nil,
			)
			if tc.blocked {
				fx.seedBlock(t, tc.viewerID, tc.sellerID)
			}

			projections, err := fx.resolve(ctx, tc.viewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
				uuid.New(): newAuctionOccurrenceWithOperation(uuid.New(), auctionID, chatEntity.ResourceOccurrenceOperationShareToChat),
			})
			require.NoError(t, err)
			require.Len(t, projections, 1)

			var proj *chatApp.ResourceProjection
			for _, p := range projections {
				proj = p
			}
			require.NotNil(t, proj)
			assertAuctionProjectionMatchesAuthority(
				t,
				proj,
				tc.viewerID,
				auctionID,
				tc.sellerID,
				tc.status,
				tc.sellerAccount,
				tc.sellerDeleted,
				tc.subscriptionStat,
				tc.blocked,
				tc.buyNowPrice,
			)

			if tc.wantLive {
				requireLiveAuctionProjection(t, proj)
			} else {
				requireTombstoneAuctionProjection(t, proj)
			}
		})
	}
}

func TestAuctionProjectionResolver_QueryCount_MatrixQ1ToQ7(t *testing.T) {
	fx := newAuctionProjectionFixture(t)
	ctx := context.Background()

	viewerID := fx.seedSellerWithSubscriptionStatus(t, "active", nil, uniqueAuctionUsername("viewer"), "Viewer Farm", nil, "active")
	buyNow := int64(1500000)
	q1Seller := fx.seedSellerWithSubscriptionStatus(t, "active", nil, uniqueAuctionUsername("q1-seller"), "Q1 Seller Farm", nil, "active")
	q1Auction := fx.seedAuction(t, q1Seller, auctionEntity.StatusActive, &buyNow, "Q1", "Q1", nil)

	sameAuctionSeller := fx.seedSellerWithSubscriptionStatus(t, "active", nil, uniqueAuctionUsername("same-auction"), "Same Auction Farm", nil, "active")
	sameAuctionID := fx.seedAuction(t, sameAuctionSeller, auctionEntity.StatusActive, &buyNow, "Q2", "Q2", nil)

	q3 := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, 20)
	for i := 0; i < 20; i++ {
		username := "q3-seller-" + uuid.NewString()
		sellerID := fx.seedSellerWithSubscriptionStatus(t, "active", nil, username, "Q3 Seller Farm", nil, "active")
		auctionID := fx.seedAuction(t, sellerID, auctionEntity.StatusActive, &buyNow, "Q3", "Q3", nil)
		q3[uuid.New()] = newAuctionOccurrenceWithOperation(uuid.New(), auctionID, chatEntity.ResourceOccurrenceOperationShareToChat)
	}

	q4Seller := fx.seedSellerWithSubscriptionStatus(t, "active", nil, uniqueAuctionUsername("q4-seller"), "Q4 Seller Farm", nil, "active")
	q4 := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, 20)
	for i := 0; i < 20; i++ {
		auctionID := fx.seedAuction(t, q4Seller, auctionEntity.StatusActive, &buyNow, "Q4", "Q4", nil)
		q4[uuid.New()] = newAuctionOccurrenceWithOperation(uuid.New(), auctionID, chatEntity.ResourceOccurrenceOperationShareToChat)
	}

	q5 := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, 20)
	for i := 0; i < 20; i++ {
		username := "q5-seller-" + uuid.NewString()
		sellerID := fx.seedSellerWithSubscriptionStatus(t, "active", nil, username, "Q5 Seller Farm", nil, "active")
		auctionID := fx.seedAuction(t, sellerID, auctionEntity.StatusActive, &buyNow, "Q5", "Q5", nil)
		q5[uuid.New()] = newAuctionOccurrenceWithOperation(uuid.New(), auctionID, chatEntity.ResourceOccurrenceOperationShareToChat)
	}

	mixedActiveSeller := fx.seedSellerWithSubscriptionStatus(t, "active", nil, uniqueAuctionUsername("mixed-active"), "Mixed Active Farm", nil, "active")
	mixedDraftSeller := fx.seedSellerWithSubscriptionStatus(t, "active", nil, uniqueAuctionUsername("mixed-draft"), "Mixed Draft Farm", nil, "active")
	mixedActiveAuction := fx.seedAuction(t, mixedActiveSeller, auctionEntity.StatusActive, &buyNow, "Q6 active", "Q6 active", nil)
	mixedDraftAuction := fx.seedAuction(t, mixedDraftSeller, auctionEntity.StatusLapsed, &buyNow, "Q6 draft", "Q6 draft", nil)
	q6 := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, 20)
	for i := 0; i < 10; i++ {
		q6[uuid.New()] = newAuctionOccurrenceWithOperation(uuid.New(), mixedActiveAuction, chatEntity.ResourceOccurrenceOperationShareToChat)
	}
	for i := 0; i < 10; i++ {
		q6[uuid.New()] = newAuctionOccurrenceWithOperation(uuid.New(), mixedDraftAuction, chatEntity.ResourceOccurrenceOperationShareToChat)
	}

	mixedShareDirectSeller := fx.seedSellerWithSubscriptionStatus(t, "active", nil, uniqueAuctionUsername("mixed-ops"), "Mixed Ops Farm", nil, "active")
	mixedShareDirectAuction := fx.seedAuction(t, mixedShareDirectSeller, auctionEntity.StatusActive, &buyNow, "Q7", "Q7", nil)
	q7 := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, 20)
	for i := 0; i < 10; i++ {
		q7[uuid.New()] = newAuctionOccurrenceWithOperation(uuid.New(), mixedShareDirectAuction, chatEntity.ResourceOccurrenceOperationShareToChat)
	}
	for i := 0; i < 10; i++ {
		q7[uuid.New()] = newAuctionOccurrenceWithOperation(uuid.New(), mixedShareDirectAuction, chatEntity.ResourceOccurrenceOperationDirectCommerceInsertChat)
	}

	cases := []struct {
		name     string
		viewerID uuid.UUID
		input    map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence
		want     int64
	}{
		{name: "Q1 one auction occurrence", viewerID: viewerID, input: map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{uuid.New(): newAuctionOccurrenceWithOperation(uuid.New(), q1Auction, chatEntity.ResourceOccurrenceOperationShareToChat)}, want: 5},
		{name: "Q2 20 occurrences same auction", viewerID: viewerID, input: makeRepeatedAuctionOccurrences(sameAuctionID, 20, chatEntity.ResourceOccurrenceOperationShareToChat), want: 5},
		{name: "Q3 20 distinct auctions", viewerID: viewerID, input: q3, want: 5},
		{name: "Q4 20 auctions same seller", viewerID: viewerID, input: q4, want: 5},
		{name: "Q5 20 auctions distinct sellers", viewerID: viewerID, input: q5, want: 5},
		{name: "Q6 mixed states and access", viewerID: viewerID, input: q6, want: 5},
		{name: "Q7 mixed share and direct", viewerID: viewerID, input: q7, want: 5},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx.tracer.reset()
			projections, err := fx.resolve(ctx, tc.viewerID, tc.input)
			require.NoError(t, err)
			require.Len(t, projections, len(tc.input))
			require.Equal(t, tc.want, fx.tracer.value())
		})
	}
}

// TestAuctionProjectionResolver_LifecycleOutcomeRuntime proves the REAL DB →
// auction projection resolver → AuctionLivePayload path for the three
// outcome-critical phases, plus viewer independence.
//
// It seeds real persisted auctions (with a real persisted current_winner_id)
// and asserts the resolver reads that DB state — never a fabricated payload.
func TestAuctionProjectionResolver_LifecycleOutcomeRuntime(t *testing.T) {
	fx := newAuctionProjectionFixture(t)
	ctx := context.Background()

	viewerID := fx.seedUser(t, "active", nil, uniqueAuctionUsername("viewer-outcome"), nil, nil)
	otherViewerID := fx.seedUser(t, "active", nil, uniqueAuctionUsername("other-outcome"), nil, nil)
	sellerID := fx.seedSellerWithSubscriptionStatus(t, "active", nil, uniqueAuctionUsername("seller-outcome"), "Outcome Farm", nil, "active")
	bidderID := fx.seedUser(t, "active", nil, uniqueAuctionUsername("bidder-outcome"), nil, nil)

	buyNow := int64(1500000)

	cases := []struct {
		name          string
		status        auctionEntity.Status
		winner        *uuid.UUID
		wantLifecycle string
		wantHasWinner bool
	}{
		{name: "waiting_settlement + winner", status: auctionEntity.StatusWaitingSettlement, winner: &bidderID, wantLifecycle: "waiting_settlement", wantHasWinner: true},
		{name: "ended + winner", status: auctionEntity.StatusEnded, winner: &bidderID, wantLifecycle: "ended", wantHasWinner: true},
		{name: "ended + no winner", status: auctionEntity.StatusEnded, winner: nil, wantLifecycle: "ended", wantHasWinner: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			auctionID := fx.seedAuction(t, sellerID, tc.status, &buyNow, tc.name, tc.name, nil)
			if tc.winner != nil {
				fx.setAuctionWinner(t, auctionID, *tc.winner)
			}

			projections, err := fx.resolve(ctx, viewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
				uuid.New(): newAuctionOccurrenceWithOperation(uuid.New(), auctionID, chatEntity.ResourceOccurrenceOperationShareToChat),
			})
			require.NoError(t, err)
			require.Len(t, projections, 1)

			var proj *chatApp.ResourceProjection
			for _, p := range projections {
				proj = p
			}
			payload := requireLiveAuctionProjection(t, proj)

			// Phase + outcome from the actual resolver output.
			require.Equal(t, tc.wantLifecycle, payload.Lifecycle)
			require.NotNil(t, payload.HasWinner)
			require.Equal(t, tc.wantHasWinner, *payload.HasWinner)

			// JSON wire shape: phase + outcome reach the serialized envelope.
			got := projectionJSONMap(t, proj)
			var auctionJSON map[string]any
			require.NoError(t, json.Unmarshal(got["auction"], &auctionJSON))
			require.Equal(t, tc.wantLifecycle, auctionJSON["lifecycle"])
			require.Equal(t, tc.wantHasWinner, auctionJSON["has_winner"])

			// Viewer independence: a second viewer sees the identical projection.
			otherProjections, err := fx.resolve(ctx, otherViewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
				uuid.New(): newAuctionOccurrenceWithOperation(uuid.New(), auctionID, chatEntity.ResourceOccurrenceOperationShareToChat),
			})
			require.NoError(t, err)
			require.Len(t, otherProjections, 1)
			var otherProj *chatApp.ResourceProjection
			for _, p := range otherProjections {
				otherProj = p
			}
			otherPayload := requireLiveAuctionProjection(t, otherProj)
			require.Equal(t, payload.Lifecycle, otherPayload.Lifecycle)
			require.NotNil(t, otherPayload.HasWinner)
			require.Equal(t, *payload.HasWinner, *otherPayload.HasWinner)
		})
	}
}

// setAuctionWinner writes the canonical winner column directly so the resolver
// reads persisted DB state (never a fabricated projection).
func (f *auctionProjectionFixture) setAuctionWinner(t *testing.T, auctionID, winnerID uuid.UUID) {
	t.Helper()

	_, err := f.appDB.Pool().Exec(context.Background(), `
		UPDATE auctions
		SET current_winner_id = $2, updated_at = NOW()
		WHERE id = $1
	`, auctionID, winnerID)
	require.NoError(t, err)
}
