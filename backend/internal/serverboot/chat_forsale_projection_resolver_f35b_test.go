//go:build integration

package serverboot

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	fpsEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	commerceshared "github.com/labuda/backend/internal/commerce/shared"
	"github.com/labuda/backend/internal/governance/viewercontext"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (f *forSaleProjectionFixture) seedSellerWithSubscriptionStatus(
	t *testing.T,
	accountStatus string,
	deletedAt *time.Time,
	username string,
	storeName string,
	storeImageURL string,
	subscriptionStatus string,
) uuid.UUID {
	t.Helper()

	sellerID := uuid.New()
	now := time.Now().UTC()
	_, err := f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO users (id, firebase_uid, email, account_status, deleted_at, created_at, updated_at, role)
		VALUES ($1, $2, $3, $4, $5, $6, $6, 'user')
	`, sellerID, sellerID.String(), sellerID.String()+"@test.local", accountStatus, deletedAt, now)
	require.NoError(t, err)

	_, err = f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO user_profiles (id, user_id, username, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
	`, uuid.New(), sellerID, username)
	require.NoError(t, err)

	_, err = f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO seller_profiles (id, user_id, store_name, store_image_url, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'active', NOW(), NOW())
	`, uuid.New(), sellerID, storeName, storeImageURL)
	require.NoError(t, err)

	paymentID := uuid.New()
	paymentNumber := "sub-" + uuid.NewString()
	midtransOrderID := uuid.NewString()
	referenceID := uuid.New()
	_, err = f.appDB.Pool().Exec(context.Background(), `
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
		VALUES ($1, $2, $3, NOW(), NOW() + INTERVAL '1 year', 365, 0, $4, NOW(), NOW())
	`, uuid.New(), sellerID, subscriptionStatus, paymentID)
	require.NoError(t, err)

	return sellerID
}

func newFPSOccurrenceWithOperation(
	messageID, saleID uuid.UUID,
	op chatEntity.ResourceOccurrenceOperation,
) *chatEntity.ChatMessageResourceOccurrence {
	return chatEntity.NewChatMessageResourceOccurrence(
		messageID,
		op,
		chatEntity.ResourceOccurrenceResourceTypeForSale,
		saleID,
		json.RawMessage(`{}`),
	)
}

func projectionJSONMap(t *testing.T, proj *chatApp.ResourceProjection) map[string]json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(proj)
	require.NoError(t, err)

	var out map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

func mustStringValue(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var out string
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

func mustInt64Value(t *testing.T, raw json.RawMessage) int64 {
	t.Helper()
	var out int64
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

func requireTopLevelKeys(t *testing.T, got map[string]json.RawMessage, want []string) {
	t.Helper()
	require.ElementsMatch(t, want, projectionKeys(got))
}

func requireAbsentKeys(t *testing.T, got map[string]json.RawMessage, keys []string) {
	t.Helper()
	for _, key := range keys {
		_, ok := got[key]
		assert.False(t, ok, "forbidden key %s must be absent", key)
	}
}

func projectionKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func resolveSingleFPS(
	t *testing.T,
	fx *forSaleProjectionFixture,
	viewerID uuid.UUID,
	occ *chatEntity.ChatMessageResourceOccurrence,
) *chatApp.ResourceProjection {
	t.Helper()
	projections, err := fx.resolve(context.Background(), viewerID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): occ,
	})
	require.NoError(t, err)
	require.Len(t, projections, 1)
	for _, proj := range projections {
		return proj
	}
	t.Fatalf("missing projection")
	return nil
}

func assertLiveFPSProjectionMatchesAuthority(
	t *testing.T,
	proj *chatApp.ResourceProjection,
	viewInput commerceshared.ForSaleViewAccessInput,
	capsInput commerceshared.ForSaleViewerCapabilitiesInput,
	wantTitle string,
	wantImageURL string,
	wantAmount int64,
	wantCurrency string,
	wantQuantity int,
	wantSellerID uuid.UUID,
	wantStoreName string,
	wantStoreImage string,
	wantUsername string,
	wantSellerLifecycle string,
) {
	t.Helper()

	expectedView := commerceshared.EvaluateForSaleViewAccess(viewInput)
	if !expectedView {
		requireTombstoneFPSProjection(t, proj)
		return
	}

	payload := requireLiveFPSProjection(t, proj)
	expectedCaps := commerceshared.EvaluateForSaleViewerCapabilities(capsInput)

	require.Equal(t, expectedCaps.Role, proj.CommerceActions.Role)
	require.Equal(t, expectedCaps.CanChat, proj.CommerceActions.CanChat)
	require.Equal(t, expectedCaps.CanNegotiate, proj.CommerceActions.CanNegotiate)
	require.Equal(t, expectedCaps.CanBuy, proj.CommerceActions.CanBuy)
	require.False(t, proj.CommerceActions.CanBid)
	require.Equal(t, expectedCaps.CanManage, proj.CommerceActions.CanManage)
	require.Equal(t, expectedCaps.CanBuy || expectedCaps.CanNegotiate, proj.ViewerCapabilities.CanInteract)
	require.True(t, proj.ViewerCapabilities.CanView)
	require.False(t, proj.ViewerCapabilities.BlockedByTombstone)

	require.Equal(t, wantTitle, payload.Title)
	require.Equal(t, wantAmount, payload.Price.Amount)
	require.Equal(t, wantCurrency, payload.Price.Currency)
	require.Equal(t, wantQuantity, payload.QuantityAvailable)
	require.Equal(t, wantSellerID, payload.Seller.ID)
	require.Equal(t, wantStoreName, payload.Seller.StoreName)
	require.Equal(t, wantUsername, payload.Seller.Username)
	require.Equal(t, wantSellerLifecycle, payload.Seller.Lifecycle)

	if wantStoreImage == "" {
		require.Nil(t, payload.Seller.StoreImage)
	} else {
		require.NotNil(t, payload.Seller.StoreImage)
		require.Equal(t, wantStoreImage, *payload.Seller.StoreImage)
	}

	if wantImageURL == "" {
		require.Nil(t, payload.ImageURL)
	} else {
		require.NotNil(t, payload.ImageURL)
		require.Equal(t, wantImageURL, *payload.ImageURL)
	}
}

func TestForSaleProjectionResolver_AuthorityMatrix(t *testing.T) {
	t.Setenv("ENABLE_PUBLIC_SELLER_TIER_PROFILE", "1")
	fx := newForSaleProjectionFixture(t)

	viewerID := fx.seedViewer(t, "viewer")
	blockedSellerID := fx.seedSellerWithSubscriptionStatus(t, "active", nil, "blocked_seller", "Blocked Farm", "https://cdn.example.test/blocked-store.jpg", "active")
	fx.seedBlock(t, viewerID, blockedSellerID)

	blockingSellerID := fx.seedSellerWithSubscriptionStatus(t, "active", nil, "blocking_seller", "Blocking Farm", "https://cdn.example.test/blocking-store.jpg", "active")
	fx.seedBlock(t, blockingSellerID, viewerID)

	activeSellerID := fx.seedSellerWithSubscriptionStatus(t, "active", nil, "active_seller", "Active Farm", "https://cdn.example.test/active-store.jpg", "active")
	expiredSellerID := fx.seedSellerWithSubscriptionStatus(t, "active", nil, "expired_seller", "Expired Farm", "https://cdn.example.test/expired-store.jpg", "expired")
	suspendedSellerID := fx.seedSellerWithSubscriptionStatus(t, "suspended", nil, "suspended_seller", "Suspended Farm", "https://cdn.example.test/suspended-store.jpg", "active")
	bannedSellerID := fx.seedSellerWithSubscriptionStatus(t, "banned", nil, "banned_seller", "Banned Farm", "https://cdn.example.test/banned-store.jpg", "active")
	removedAt := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	removedSellerID := fx.seedSellerWithSubscriptionStatus(t, "active", &removedAt, "removed_seller", "Removed Farm", "https://cdn.example.test/removed-store.jpg", "active")

	type caseInput struct {
		name                string
		viewerID            uuid.UUID
		sellerID            uuid.UUID
		accountStatus       string
		deletedAt           *time.Time
		subscriptionStatus  string
		status              fpsEntity.ForSaleStatus
		visibility          fpsEntity.ForSaleVisibility
		negotiationEnabled  bool
		quantityAvailable   int
		pricePerUnit        int64
		title               string
		productMediaURLs    []string
		wantLive            bool
		wantImageURL        string
		wantStoreImage      string
		wantStoreName       string
		wantUsername        string
		wantSellerLifecycle string
		blocked             bool
	}

	cases := []caseInput{
		{
			name:                "F1 active public buyer",
			viewerID:            viewerID,
			sellerID:            activeSellerID,
			accountStatus:       "active",
			subscriptionStatus:  "active",
			status:              fpsEntity.ForSaleStatusActive,
			visibility:          fpsEntity.ForSaleVisibilityPublic,
			negotiationEnabled:  true,
			quantityAvailable:   3,
			pricePerUnit:        1500000,
			title:               "Showa Koi 30cm",
			productMediaURLs:    []string{"https://cdn.example.test/product-thumb.jpg"},
			wantLive:            true,
			wantImageURL:        "https://cdn.example.test/product-thumb.jpg",
			wantStoreImage:      "https://cdn.example.test/active-store.jpg",
			wantStoreName:       "Active Farm",
			wantUsername:        "active_seller",
			wantSellerLifecycle: string(viewercontext.PublicLifecycleStateActive),
		},
		{
			name:                "F4 owner viewing own FPS",
			viewerID:            activeSellerID,
			sellerID:            activeSellerID,
			accountStatus:       "active",
			subscriptionStatus:  "active",
			status:              fpsEntity.ForSaleStatusActive,
			visibility:          fpsEntity.ForSaleVisibilityPublic,
			negotiationEnabled:  true,
			quantityAvailable:   3,
			pricePerUnit:        1500000,
			title:               "Owner Active Koi",
			wantLive:            true,
			wantStoreImage:      "https://cdn.example.test/active-store.jpg",
			wantStoreName:       "Active Farm",
			wantUsername:        "active_seller",
			wantSellerLifecycle: string(viewercontext.PublicLifecycleStateActive),
		},
		{
			name:                "F5 draft FPS owner",
			viewerID:            activeSellerID,
			sellerID:            activeSellerID,
			accountStatus:       "active",
			subscriptionStatus:  "active",
			status:              fpsEntity.ForSaleStatusDraft,
			visibility:          fpsEntity.ForSaleVisibilityPrivate,
			negotiationEnabled:  false,
			quantityAvailable:   1,
			pricePerUnit:        900000,
			title:               "Draft Owner Koi",
			wantLive:            true,
			wantStoreImage:      "https://cdn.example.test/active-store.jpg",
			wantStoreName:       "Active Farm",
			wantUsername:        "active_seller",
			wantSellerLifecycle: string(viewercontext.PublicLifecycleStateActive),
		},
		{
			name:               "F6 draft FPS non-owner",
			viewerID:           viewerID,
			sellerID:           activeSellerID,
			accountStatus:      "active",
			subscriptionStatus: "active",
			status:             fpsEntity.ForSaleStatusDraft,
			visibility:         fpsEntity.ForSaleVisibilityPrivate,
			negotiationEnabled: false,
			quantityAvailable:  1,
			pricePerUnit:       900000,
			title:              "Draft Non-owner Koi",
			wantLive:           false,
		},
		{
			name:               "F7 viewer blocks seller",
			viewerID:           viewerID,
			sellerID:           blockedSellerID,
			accountStatus:      "active",
			subscriptionStatus: "active",
			status:             fpsEntity.ForSaleStatusActive,
			visibility:         fpsEntity.ForSaleVisibilityPublic,
			negotiationEnabled: true,
			quantityAvailable:  1,
			pricePerUnit:       700000,
			title:              "Blocked Viewer Koi",
			wantLive:           false,
			blocked:            true,
		},
		{
			name:               "F8 seller blocks viewer",
			viewerID:           viewerID,
			sellerID:           blockingSellerID,
			accountStatus:      "active",
			subscriptionStatus: "active",
			status:             fpsEntity.ForSaleStatusActive,
			visibility:         fpsEntity.ForSaleVisibilityPublic,
			negotiationEnabled: true,
			quantityAvailable:  1,
			pricePerUnit:       700000,
			title:              "Blocking Seller Koi",
			wantLive:           false,
			blocked:            true,
		},
		{
			name:               "F9 seller suspended",
			viewerID:           viewerID,
			sellerID:           suspendedSellerID,
			accountStatus:      "suspended",
			subscriptionStatus: "active",
			status:             fpsEntity.ForSaleStatusActive,
			visibility:         fpsEntity.ForSaleVisibilityPublic,
			negotiationEnabled: true,
			quantityAvailable:  2,
			pricePerUnit:       800000,
			title:              "Suspended Seller Koi",
			wantLive:           false,
		},
		{
			name:               "F10 seller banned",
			viewerID:           viewerID,
			sellerID:           bannedSellerID,
			accountStatus:      "banned",
			subscriptionStatus: "active",
			status:             fpsEntity.ForSaleStatusActive,
			visibility:         fpsEntity.ForSaleVisibilityPublic,
			negotiationEnabled: true,
			quantityAvailable:  2,
			pricePerUnit:       800000,
			title:              "Banned Seller Koi",
			wantLive:           false,
		},
		{
			name:               "F10 seller removed",
			viewerID:           viewerID,
			sellerID:           removedSellerID,
			accountStatus:      "active",
			deletedAt:          &removedAt,
			subscriptionStatus: "active",
			status:             fpsEntity.ForSaleStatusActive,
			visibility:         fpsEntity.ForSaleVisibilityPublic,
			negotiationEnabled: true,
			quantityAvailable:  2,
			pricePerUnit:       800000,
			title:              "Removed Seller Koi",
			wantLive:           false,
		},
		{
			name:                "F11 expired subscription retains identity",
			viewerID:            viewerID,
			sellerID:            expiredSellerID,
			accountStatus:       "active",
			subscriptionStatus:  "expired",
			status:              fpsEntity.ForSaleStatusActive,
			visibility:          fpsEntity.ForSaleVisibilityPublic,
			negotiationEnabled:  true,
			quantityAvailable:   4,
			pricePerUnit:        1200000,
			title:               "Expired Trust Koi",
			wantLive:            true,
			wantStoreImage:      "https://cdn.example.test/expired-store.jpg",
			wantStoreName:       "Expired Farm",
			wantUsername:        "expired_seller",
			wantSellerLifecycle: string(viewercontext.PublicLifecycleStateActive),
		},
		{
			name:                "F12 sold terminal owner-visible",
			viewerID:            activeSellerID,
			sellerID:            activeSellerID,
			accountStatus:       "active",
			subscriptionStatus:  "active",
			status:              fpsEntity.ForSaleStatusSold,
			visibility:          fpsEntity.ForSaleVisibilityPrivate,
			negotiationEnabled:  false,
			quantityAvailable:   0,
			pricePerUnit:        1500000,
			title:               "Sold Owner Koi",
			wantLive:            true,
			wantStoreImage:      "https://cdn.example.test/active-store.jpg",
			wantStoreName:       "Active Farm",
			wantUsername:        "active_seller",
			wantSellerLifecycle: string(viewercontext.PublicLifecycleStateActive),
		},
		{
			name:                "F13 withdrawn terminal owner-visible",
			viewerID:            activeSellerID,
			sellerID:            activeSellerID,
			accountStatus:       "active",
			subscriptionStatus:  "active",
			status:              fpsEntity.ForSaleStatusWithdrawn,
			visibility:          fpsEntity.ForSaleVisibilityPrivate,
			negotiationEnabled:  false,
			quantityAvailable:   0,
			pricePerUnit:        1500000,
			title:               "Withdrawn Owner Koi",
			wantLive:            true,
			wantStoreImage:      "https://cdn.example.test/active-store.jpg",
			wantStoreName:       "Active Farm",
			wantUsername:        "active_seller",
			wantSellerLifecycle: string(viewercontext.PublicLifecycleStateActive),
		},
		{
			name:                "F14/F15/F17/F19/F20 active buyer negotiable and buyable",
			viewerID:            viewerID,
			sellerID:            activeSellerID,
			accountStatus:       "active",
			subscriptionStatus:  "active",
			status:              fpsEntity.ForSaleStatusActive,
			visibility:          fpsEntity.ForSaleVisibilityPublic,
			negotiationEnabled:  true,
			quantityAvailable:   2,
			pricePerUnit:        1500000,
			title:               "Active Negotiable Koi",
			wantLive:            true,
			wantStoreImage:      "https://cdn.example.test/active-store.jpg",
			wantStoreName:       "Active Farm",
			wantUsername:        "active_seller",
			wantSellerLifecycle: string(viewercontext.PublicLifecycleStateActive),
		},
		{
			name:                "F16 negotiation disabled",
			viewerID:            viewerID,
			sellerID:            activeSellerID,
			accountStatus:       "active",
			subscriptionStatus:  "active",
			status:              fpsEntity.ForSaleStatusActive,
			visibility:          fpsEntity.ForSaleVisibilityPublic,
			negotiationEnabled:  false,
			quantityAvailable:   2,
			pricePerUnit:        1500000,
			title:               "Active Nonnegotiable Koi",
			wantLive:            true,
			wantStoreImage:      "https://cdn.example.test/active-store.jpg",
			wantStoreName:       "Active Farm",
			wantUsername:        "active_seller",
			wantSellerLifecycle: string(viewercontext.PublicLifecycleStateActive),
		},
		{
			name:                "F18 owner management",
			viewerID:            activeSellerID,
			sellerID:            activeSellerID,
			accountStatus:       "active",
			subscriptionStatus:  "active",
			status:              fpsEntity.ForSaleStatusActive,
			visibility:          fpsEntity.ForSaleVisibilityPublic,
			negotiationEnabled:  true,
			quantityAvailable:   2,
			pricePerUnit:        1500000,
			title:               "Owner Management Koi",
			wantLive:            true,
			wantStoreImage:      "https://cdn.example.test/active-store.jpg",
			wantStoreName:       "Active Farm",
			wantUsername:        "active_seller",
			wantSellerLifecycle: string(viewercontext.PublicLifecycleStateActive),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			saleID := fx.seedSale(
				t,
				tc.sellerID,
				tc.status,
				tc.visibility,
				tc.negotiationEnabled,
				tc.quantityAvailable,
				tc.pricePerUnit,
				tc.title,
				tc.productMediaURLs,
			)

			proj := resolveSingleFPS(t, fx, tc.viewerID, newFPSOccurrence(uuid.New(), saleID))

			viewInput := commerceshared.ForSaleViewAccessInput{
				ViewerID:   tc.viewerID,
				SellerID:   tc.sellerID,
				Status:     string(tc.status),
				Visibility: string(tc.visibility),
				Blocked:    tc.blocked,
				Seller: commerceshared.SellerAccessSnapshot{
					AccountStatus:      tc.accountStatus,
					IsDeleted:          tc.deletedAt != nil,
					SubscriptionStatus: tc.subscriptionStatus,
				},
			}
			capsInput := commerceshared.ForSaleViewerCapabilitiesInput{
				ViewerID:           tc.viewerID,
				SellerID:           tc.sellerID,
				ProductID:          uuid.New(),
				Status:             string(tc.status),
				QuantityAvailable:  tc.quantityAvailable,
				NegotiationEnabled: tc.negotiationEnabled,
				SellerTrustActive:  tc.subscriptionStatus == "active",
			}

			if !tc.wantLive {
				requireTombstoneFPSProjection(t, proj)
				require.False(t, commerceshared.EvaluateForSaleViewAccess(viewInput))
				return
			}

			assertLiveFPSProjectionMatchesAuthority(
				t,
				proj,
				viewInput,
				capsInput,
				tc.title,
				func() string {
					if len(tc.productMediaURLs) > 0 {
						return tc.productMediaURLs[0]
					}
					return ""
				}(),
				tc.pricePerUnit,
				"IDR",
				tc.quantityAvailable,
				tc.sellerID,
				tc.wantStoreName,
				tc.wantStoreImage,
				tc.wantUsername,
				tc.wantSellerLifecycle,
			)
		})
	}
}

func TestForSaleProjectionResolver_OperationConvergence(t *testing.T) {
	fx := newForSaleProjectionFixture(t)

	viewerID := fx.seedViewer(t, "viewer")
	sellerID := fx.seedSellerWithSubscriptionStatus(t, "active", nil, "active_seller", "Active Farm", "https://cdn.example.test/active-store.jpg", "active")
	saleID := fx.seedSale(
		t,
		sellerID,
		fpsEntity.ForSaleStatusActive,
		fpsEntity.ForSaleVisibilityPublic,
		true,
		2,
		1500000,
		"Operation Koi",
		[]string{"https://cdn.example.test/product-thumb.jpg"},
	)

	occurrences := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newFPSOccurrenceWithOperation(uuid.New(), saleID, chatEntity.ResourceOccurrenceOperationShareToChat),
		uuid.New(): newFPSOccurrenceWithOperation(uuid.New(), saleID, chatEntity.ResourceOccurrenceOperationDirectCommerceInsertChat),
	}

	projections, err := fx.resolve(context.Background(), viewerID, occurrences)
	require.NoError(t, err)
	require.Len(t, projections, 2)

	var first, second *chatApp.ResourceProjection
	for _, proj := range projections {
		if first == nil {
			first = proj
			continue
		}
		second = proj
	}
	require.NotNil(t, first)
	require.NotNil(t, second)
	require.Equal(t, projectionJSONMap(t, first), projectionJSONMap(t, second))
}

func TestForSaleProjectionResolver_JSONContracts(t *testing.T) {
	fx := newForSaleProjectionFixture(t)

	viewerID := fx.seedViewer(t, "viewer")
	sellerID := fx.seedSellerWithSubscriptionStatus(t, "active", nil, "active_seller", "Active Farm", "https://cdn.example.test/active-store.jpg", "active")
	saleID := fx.seedSale(
		t,
		sellerID,
		fpsEntity.ForSaleStatusActive,
		fpsEntity.ForSaleVisibilityPublic,
		true,
		3,
		1500000,
		"JSON Koi",
		[]string{"https://cdn.example.test/product-thumb.jpg"},
	)

	liveProj := resolveSingleFPS(t, fx, viewerID, newFPSOccurrence(uuid.New(), saleID))
	liveJSON := projectionJSONMap(t, liveProj)
	requireTopLevelKeys(t, liveJSON, []string{"state", "resource_type", "resource_id", "canonical_url", "viewer_capabilities", "commerce_actions", "for_sale"})
	requireAbsentKeys(t, liveJSON, []string{"profile", "content", "auction"})
	require.Equal(t, "LIVE", mustStringValue(t, liveJSON["state"]))
	require.Equal(t, "for_sale", mustStringValue(t, liveJSON["resource_type"]))
	require.Equal(t, liveProj.Identity.ResourceID.String(), mustStringValue(t, liveJSON["resource_id"]))
	require.Equal(t, "/for-sale/"+saleID.String(), mustStringValue(t, liveJSON["canonical_url"]))

	var livePayload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(liveJSON["for_sale"], &livePayload))
	requireTopLevelKeys(t, livePayload, []string{"title", "image_url", "price", "status", "seller", "quantity_available"})
	require.Equal(t, "JSON Koi", mustStringValue(t, livePayload["title"]))
	require.Equal(t, "https://cdn.example.test/product-thumb.jpg", mustStringValue(t, livePayload["image_url"]))
	require.Equal(t, "active", mustStringValue(t, livePayload["status"]))
	require.Equal(t, int64(3), mustInt64Value(t, livePayload["quantity_available"]))

	var price map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(livePayload["price"], &price))
	requireTopLevelKeys(t, price, []string{"amount", "currency"})
	require.Equal(t, int64(1500000), mustInt64Value(t, price["amount"]))
	require.Equal(t, "IDR", mustStringValue(t, price["currency"]))

	var seller map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(livePayload["seller"], &seller))
	requireTopLevelKeys(t, seller, []string{"id", "store_name", "store_image", "username", "lifecycle"})
	require.Equal(t, sellerID.String(), mustStringValue(t, seller["id"]))
	require.Equal(t, "Active Farm", mustStringValue(t, seller["store_name"]))
	require.Equal(t, "https://cdn.example.test/active-store.jpg", mustStringValue(t, seller["store_image"]))
	require.Equal(t, "active_seller", mustStringValue(t, seller["username"]))
	require.Equal(t, string(viewercontext.PublicLifecycleStateActive), mustStringValue(t, seller["lifecycle"]))

	tombstoneSellerID := fx.seedSellerWithSubscriptionStatus(t, "active", nil, "draft_owner", "Draft Farm", "https://cdn.example.test/draft-store.jpg", "expired")
	tombstoneSaleID := fx.seedSale(
		t,
		tombstoneSellerID,
		fpsEntity.ForSaleStatusDraft,
		fpsEntity.ForSaleVisibilityPrivate,
		false,
		1,
		900000,
		"Draft JSON Koi",
		nil,
	)
	tombstoneProj := resolveSingleFPS(t, fx, viewerID, newFPSOccurrence(uuid.New(), tombstoneSaleID))
	tombstoneJSON := projectionJSONMap(t, tombstoneProj)
	requireTopLevelKeys(t, tombstoneJSON, []string{"state", "resource_type", "viewer_capabilities"})
	requireAbsentKeys(t, tombstoneJSON, []string{"resource_id", "canonical_url", "commerce_actions", "for_sale", "profile", "content", "auction"})
	require.Equal(t, "TOMBSTONE", mustStringValue(t, tombstoneJSON["state"]))
	require.Equal(t, "for_sale", mustStringValue(t, tombstoneJSON["resource_type"]))
}

func TestForSaleProjectionResolver_FailurePropagationAndIntegrity(t *testing.T) {
	fx := newForSaleProjectionFixture(t)

	viewerID := fx.seedViewer(t, "viewer")
	sellerID := fx.seedSellerWithSubscriptionStatus(t, "active", nil, "active_seller", "Active Farm", "https://cdn.example.test/active-store.jpg", "active")
	saleID := fx.seedSale(
		t,
		sellerID,
		fpsEntity.ForSaleStatusActive,
		fpsEntity.ForSaleVisibilityPublic,
		true,
		2,
		100000,
		"Failure Koi",
		nil,
	)

	occurrences := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newFPSOccurrence(uuid.New(), saleID),
	}

	t.Run("F24 source batch failure", func(t *testing.T) {
		resolver := newForSaleProjectionBatchResolver(&forSaleProjectionFailingDB{
			base:        fx.appDB,
			failOnQuery: 1,
			err:         assert.AnError,
		})
		projections, err := resolver.ResolveForSales(context.Background(), viewerID, occurrences)
		require.Error(t, err)
		require.Nil(t, projections)
		assert.Contains(t, err.Error(), "fixed price sale source batch query failed")
	})

	t.Run("F25 seller/access hydration shares the source batch", func(t *testing.T) {
		resolver := newForSaleProjectionBatchResolver(&forSaleProjectionFailingDB{
			base:        fx.appDB,
			failOnQuery: 1,
			err:         assert.AnError,
		})
		projections, err := resolver.ResolveForSales(context.Background(), viewerID, occurrences)
		require.Error(t, err)
		require.Nil(t, projections)
		assert.Contains(t, err.Error(), "fixed price sale source batch query failed")
	})

	t.Run("F26 block batch failure", func(t *testing.T) {
		resolver := newForSaleProjectionBatchResolver(&forSaleProjectionFailingDB{
			base:        fx.appDB,
			failOnQuery: 2,
			err:         assert.AnError,
		})
		projections, err := resolver.ResolveForSales(context.Background(), viewerID, occurrences)
		require.Error(t, err)
		require.Nil(t, projections)
		assert.Contains(t, err.Error(), "fixed price sale block batch query failed")
	})

	t.Run("F28 missing source row is integrity error", func(t *testing.T) {
		missingOccurrences := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
			uuid.New(): newFPSOccurrence(uuid.New(), uuid.New()),
		}
		projections, err := fx.resolve(context.Background(), viewerID, missingOccurrences)
		require.Error(t, err)
		require.Nil(t, projections)
		assert.Contains(t, err.Error(), "fixed price sale source row missing")
	})
}

func TestForSaleProjectionResolver_QueryCountMatrix(t *testing.T) {
	fx := newForSaleProjectionFixture(t)
	viewerID := fx.seedViewer(t, "viewer")

	makeSale := func(t *testing.T, sellerStatus string, subscriptionStatus string, title string) uuid.UUID {
		t.Helper()
		sellerID := fx.seedSellerWithSubscriptionStatus(t, sellerStatus, nil, title+" seller", title+" Farm", "https://cdn.example.test/"+title+".jpg", subscriptionStatus)
		return fx.seedSale(t, sellerID, fpsEntity.ForSaleStatusActive, fpsEntity.ForSaleVisibilityPublic, true, 2, 250000, title, nil)
	}

	resolveCount := func(occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence) int64 {
		fx.tracer.reset()
		_, err := fx.resolve(context.Background(), viewerID, occurrences)
		require.NoError(t, err)
		return fx.tracer.value()
	}

	q1SaleID := makeSale(t, "active", "active", "Q1")
	q1 := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newFPSOccurrence(uuid.New(), q1SaleID),
	}
	q1Count := resolveCount(q1)

	q2 := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, 20)
	for i := 0; i < 20; i++ {
		q2[uuid.New()] = newFPSOccurrence(uuid.New(), q1SaleID)
	}
	q2Count := resolveCount(q2)

	q3 := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, 20)
	for i := 0; i < 20; i++ {
		q3SaleID := makeSale(t, "active", "active", "Q3-"+uuid.NewString())
		q3[uuid.New()] = newFPSOccurrence(uuid.New(), q3SaleID)
	}
	q3Count := resolveCount(q3)

	q4 := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, 20)
	q4SellerID := fx.seedSellerWithSubscriptionStatus(t, "active", nil, "q4_seller", "Q4 Farm", "https://cdn.example.test/q4.jpg", "active")
	for i := 0; i < 20; i++ {
		saleID := fx.seedSale(t, q4SellerID, fpsEntity.ForSaleStatusActive, fpsEntity.ForSaleVisibilityPublic, true, 2, 250000, "Q4-"+uuid.NewString(), nil)
		q4[uuid.New()] = newFPSOccurrence(uuid.New(), saleID)
	}
	q4Count := resolveCount(q4)

	q5 := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, 20)
	for i := 0; i < 20; i++ {
		saleID := makeSale(t, "active", "active", "Q5-"+uuid.NewString())
		q5[uuid.New()] = newFPSOccurrence(uuid.New(), saleID)
	}
	q5Count := resolveCount(q5)

	activeSellerID := fx.seedSellerWithSubscriptionStatus(t, "active", nil, "q6_active", "Q6 Active", "https://cdn.example.test/q6a.jpg", "active")
	draftSellerID := fx.seedSellerWithSubscriptionStatus(t, "active", nil, "q6_draft", "Q6 Draft", "https://cdn.example.test/q6d.jpg", "active")
	blockedSellerID := fx.seedSellerWithSubscriptionStatus(t, "active", nil, "q6_blocked", "Q6 Blocked", "https://cdn.example.test/q6b.jpg", "active")
	fx.seedBlock(t, viewerID, blockedSellerID)
	expiredSellerID := fx.seedSellerWithSubscriptionStatus(t, "active", nil, "q6_expired", "Q6 Expired", "https://cdn.example.test/q6e.jpg", "expired")
	suspendedSellerID := fx.seedSellerWithSubscriptionStatus(t, "suspended", nil, "q6_suspended", "Q6 Suspended", "https://cdn.example.test/q6s.jpg", "active")

	q6 := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newFPSOccurrence(uuid.New(), fx.seedSale(t, activeSellerID, fpsEntity.ForSaleStatusActive, fpsEntity.ForSaleVisibilityPublic, true, 2, 1000, "Q6 Active", nil)),
		uuid.New(): newFPSOccurrence(uuid.New(), fx.seedSale(t, draftSellerID, fpsEntity.ForSaleStatusDraft, fpsEntity.ForSaleVisibilityPrivate, false, 1, 1000, "Q6 Draft", nil)),
		uuid.New(): newFPSOccurrence(uuid.New(), fx.seedSale(t, activeSellerID, fpsEntity.ForSaleStatusSold, fpsEntity.ForSaleVisibilityPrivate, false, 0, 1000, "Q6 Sold", nil)),
		uuid.New(): newFPSOccurrence(uuid.New(), fx.seedSale(t, blockedSellerID, fpsEntity.ForSaleStatusActive, fpsEntity.ForSaleVisibilityPublic, true, 1, 1000, "Q6 Blocked", nil)),
		uuid.New(): newFPSOccurrence(uuid.New(), fx.seedSale(t, expiredSellerID, fpsEntity.ForSaleStatusActive, fpsEntity.ForSaleVisibilityPublic, true, 1, 1000, "Q6 Expired", nil)),
		uuid.New(): newFPSOccurrence(uuid.New(), fx.seedSale(t, suspendedSellerID, fpsEntity.ForSaleStatusActive, fpsEntity.ForSaleVisibilityPublic, true, 1, 1000, "Q6 Suspended", nil)),
	}
	q6Count := resolveCount(q6)

	q7SaleID := fx.seedSale(t, activeSellerID, fpsEntity.ForSaleStatusActive, fpsEntity.ForSaleVisibilityPublic, true, 2, 1000, "Q7 Operation", nil)
	q7 := map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence{
		uuid.New(): newFPSOccurrenceWithOperation(uuid.New(), q7SaleID, chatEntity.ResourceOccurrenceOperationShareToChat),
		uuid.New(): newFPSOccurrenceWithOperation(uuid.New(), q7SaleID, chatEntity.ResourceOccurrenceOperationDirectCommerceInsertChat),
	}
	q7Count := resolveCount(q7)

	t.Logf("Q1=%d Q2=%d Q3=%d Q4=%d Q5=%d Q6=%d Q7=%d", q1Count, q2Count, q3Count, q4Count, q5Count, q6Count, q7Count)
	require.Equal(t, q1Count, q2Count)
	require.Equal(t, q2Count, q3Count)
	require.Equal(t, q3Count, q4Count)
	require.Equal(t, q4Count, q5Count)
	require.Equal(t, q5Count, q6Count)
	require.Equal(t, q6Count, q7Count)
}
