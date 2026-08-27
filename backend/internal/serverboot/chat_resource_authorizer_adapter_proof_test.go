//go:build integration

package serverboot

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	auctionRepo "github.com/labuda/backend/internal/commerce/auction/infrastructure/repository"
	fpsRepo "github.com/labuda/backend/internal/commerce/forsale/infrastructure/repository"
	"github.com/labuda/backend/internal/identity/auth"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	chatRepo "github.com/labuda/backend/internal/interaction/chat/repository"
	contentRepo "github.com/labuda/backend/internal/social/content/infrastructure/repository"
	socialRepo "github.com/labuda/backend/internal/social/graph/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
)

func TestChatResourceAuthorizerAdapter_SentinelErrors(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	appDB := db.NewFromPool(tdb.Pool())
	roleChecker := auth.NewRoleCheckerDB(appDB, nil)
	authz := newChatResourceAuthorizer(
		appDB,
		contentRepo.NewContentRepository(),
		socialRepo.NewSocialRepository(),
		roleChecker,
		fpsRepo.NewForSaleRepository(),
		auctionRepo.NewAuctionRepository(),
	)

	ctx := context.Background()
	viewerID := seedTestUser(t, ctx, appDB, "viewer")
	blockedProfileID := seedTestUser(t, ctx, appDB, "blocked-profile")
	blockedContentAuthorID := seedTestUser(t, ctx, appDB, "blocked-content-author")
	fpsOwnerID := seedTestUser(t, ctx, appDB, "fps-owner")
	auctionOwnerID := seedTestUser(t, ctx, appDB, "auction-owner")

	seedBlock(t, ctx, appDB, viewerID, blockedProfileID)
	privateContentID := seedContent(t, ctx, appDB, blockedContentAuthorID, "private", false, false)

	seedSellerProfile(t, ctx, appDB, fpsOwnerID)
	seedSellerProfile(t, ctx, appDB, auctionOwnerID)
	seedSellerSubscription(t, ctx, appDB, fpsOwnerID)
	seedSellerSubscription(t, ctx, appDB, auctionOwnerID)

	activeFPSID := seedForSale(t, ctx, appDB, fpsOwnerID, string(chatEntity.ResourceOccurrenceResourceTypeForSale), "active")
	draftFPSID := seedForSale(t, ctx, appDB, fpsOwnerID, string(chatEntity.ResourceOccurrenceResourceTypeForSale), "draft")
	activeAuctionID := seedAuction(t, ctx, appDB, auctionOwnerID, "active")
	draftAuctionID := seedAuction(t, ctx, appDB, auctionOwnerID, "draft")

	t.Run("share profile blocked", func(t *testing.T) {
		err := withAuthorizerTx(t, appDB, func(tx db.Tx) error {
			_, err := authz.AuthorizeShare(ctx, tx, viewerID, chatEntity.ResourceOccurrenceResourceTypeProfile, blockedProfileID)
			return err
		})
		require.ErrorIs(t, err, chatRepo.ErrResourceNotAccessible)
	})

	t.Run("share content private", func(t *testing.T) {
		err := withAuthorizerTx(t, appDB, func(tx db.Tx) error {
			_, err := authz.AuthorizeShare(ctx, tx, viewerID, chatEntity.ResourceOccurrenceResourceTypeContent, privateContentID)
			return err
		})
		require.ErrorIs(t, err, chatRepo.ErrResourceNotAccessible)
	})

	t.Run("direct fps missing", func(t *testing.T) {
		err := withAuthorizerTx(t, appDB, func(tx db.Tx) error {
			_, err := authz.AuthorizeDirect(ctx, tx, fpsOwnerID, chatEntity.ResourceOccurrenceResourceTypeForSale, uuid.New())
			return err
		})
		require.ErrorIs(t, err, chatRepo.ErrResourceNotFound)
	})

	t.Run("direct fps foreign owner", func(t *testing.T) {
		err := withAuthorizerTx(t, appDB, func(tx db.Tx) error {
			_, err := authz.AuthorizeDirect(ctx, tx, viewerID, chatEntity.ResourceOccurrenceResourceTypeForSale, activeFPSID)
			return err
		})
		require.ErrorIs(t, err, chatRepo.ErrNotResourceOwner)
	})

	t.Run("direct fps no market capability", func(t *testing.T) {
		noCapSellerID := seedTestUser(t, ctx, appDB, "no-cap-seller")
		seedSellerProfile(t, ctx, appDB, noCapSellerID)
		noCapFPSID := seedForSale(t, ctx, appDB, noCapSellerID, string(chatEntity.ResourceOccurrenceResourceTypeForSale), "active")

		err := withAuthorizerTx(t, appDB, func(tx db.Tx) error {
			_, err := authz.AuthorizeDirect(ctx, tx, noCapSellerID, chatEntity.ResourceOccurrenceResourceTypeForSale, noCapFPSID)
			return err
		})
		require.ErrorIs(t, err, chatRepo.ErrMarketAuthorityRequired)
	})

	t.Run("direct fps not promotable", func(t *testing.T) {
		err := withAuthorizerTx(t, appDB, func(tx db.Tx) error {
			_, err := authz.AuthorizeDirect(ctx, tx, fpsOwnerID, chatEntity.ResourceOccurrenceResourceTypeForSale, draftFPSID)
			return err
		})
		require.ErrorIs(t, err, chatRepo.ErrResourceNotPromotable)
	})

	t.Run("direct auction foreign owner", func(t *testing.T) {
		err := withAuthorizerTx(t, appDB, func(tx db.Tx) error {
			_, err := authz.AuthorizeDirect(ctx, tx, viewerID, chatEntity.ResourceOccurrenceResourceTypeAuction, activeAuctionID)
			return err
		})
		require.ErrorIs(t, err, chatRepo.ErrNotResourceOwner)
	})

	t.Run("direct auction no market capability", func(t *testing.T) {
		noCapAuctionSellerID := seedTestUser(t, ctx, appDB, "no-cap-auction-seller")
		seedSellerProfile(t, ctx, appDB, noCapAuctionSellerID)
		noCapAuctionID := seedAuction(t, ctx, appDB, noCapAuctionSellerID, "active")

		err := withAuthorizerTx(t, appDB, func(tx db.Tx) error {
			_, err := authz.AuthorizeDirect(ctx, tx, noCapAuctionSellerID, chatEntity.ResourceOccurrenceResourceTypeAuction, noCapAuctionID)
			return err
		})
		require.ErrorIs(t, err, chatRepo.ErrMarketAuthorityRequired)
	})

	t.Run("direct auction not promotable", func(t *testing.T) {
		err := withAuthorizerTx(t, appDB, func(tx db.Tx) error {
			_, err := authz.AuthorizeDirect(ctx, tx, auctionOwnerID, chatEntity.ResourceOccurrenceResourceTypeAuction, draftAuctionID)
			return err
		})
		require.ErrorIs(t, err, chatRepo.ErrResourceNotPromotable)
	})
}

func withAuthorizerTx(t *testing.T, appDB *db.DB, fn func(tx db.Tx) error) error {
	t.Helper()

	var err error
	require.NoError(t, appDB.WithTx(context.Background(), func(tx db.Tx) error {
		err = fn(tx)
		return nil
	}))
	return err
}

func seedTestUser(t *testing.T, ctx context.Context, appDB *db.DB, suffix string) uuid.UUID {
	t.Helper()

	id := uuid.New()
	_, err := appDB.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at)
		VALUES ($1, $2, $3, 'active', NOW(), NOW())
	`, id, "firebase-"+suffix+"-"+id.String(), suffix+"@test.local")
	require.NoError(t, err)

	_, err = appDB.Pool().Exec(ctx, `
		INSERT INTO user_profiles (user_id, username, created_at, updated_at)
		VALUES ($1, $2, NOW(), NOW())
	`, id, suffix)
	require.NoError(t, err)

	return id
}

func seedBlock(t *testing.T, ctx context.Context, appDB *db.DB, blockerID, blockedID uuid.UUID) {
	t.Helper()

	_, err := appDB.Pool().Exec(ctx, `
		INSERT INTO user_blocks (blocker_id, blocked_id, created_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT DO NOTHING
	`, blockerID, blockedID)
	require.NoError(t, err)
}

func seedContent(t *testing.T, ctx context.Context, appDB *db.DB, authorID uuid.UUID, visibility string, hidden, deleted bool) uuid.UUID {
	t.Helper()

	id := uuid.New()
	_, err := appDB.Pool().Exec(ctx, `
		INSERT INTO contents (
			id, author_id, status, caption, city, province,
			is_hidden, original_author_id, visibility, created_at, updated_at, deleted_at
		)
		VALUES ($1, $2, 'active', 'test content', NULL, NULL, $3, NULL, $4::content_visibility_enum, NOW(), NOW(), NULL)
	`, id, authorID, hidden, visibility)
	require.NoError(t, err)

	if deleted {
		_, err = appDB.Pool().Exec(ctx, `UPDATE contents SET deleted_at = NOW() WHERE id = $1`, id)
		require.NoError(t, err)
	}

	return id
}

func seedSellerProfile(t *testing.T, ctx context.Context, appDB *db.DB, userID uuid.UUID) {
	t.Helper()

	_, err := appDB.Pool().Exec(ctx, `
		INSERT INTO seller_profiles (user_id, store_name, status, created_at, updated_at)
		VALUES ($1, $2, 'active', NOW(), NOW())
		ON CONFLICT (user_id) DO NOTHING
	`, userID, "store-"+userID.String()[:8])
	require.NoError(t, err)
}

func seedSellerSubscription(t *testing.T, ctx context.Context, appDB *db.DB, userID uuid.UUID) {
	t.Helper()

	paymentID := uuid.New()
	_, err := appDB.Pool().Exec(ctx, `
		INSERT INTO payments (
			id, user_id, payment_number, midtrans_order_id, gross_amount,
			service_fee_amount, coin_discount_amount, coins_to_use,
			reference_type, reference_id, expired_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, 0, 0, 0, 0, 'subscription', $5, NOW() + INTERVAL '1 year', NOW(), NOW())
	`, paymentID, userID, "sub-"+paymentID.String(), "mid-"+paymentID.String(), uuid.New())
	require.NoError(t, err)

	_, err = appDB.Pool().Exec(ctx, `
		INSERT INTO seller_subscriptions (
			id, user_id, status, started_at, expires_at, duration_days,
			amount_paid, payment_id, created_at, updated_at
		)
		VALUES ($1, $2, 'active', NOW() - INTERVAL '1 day', NOW() + INTERVAL '1 year', 365, 0, $3, NOW(), NOW())
	`, uuid.New(), userID, paymentID)
	require.NoError(t, err)
}

func seedProduct(t *testing.T, ctx context.Context, appDB *db.DB, sellerID uuid.UUID, title string) uuid.UUID {
	t.Helper()

	id := uuid.New()
	_, err := appDB.Pool().Exec(ctx, `
		INSERT INTO products (
			id, seller_id, title, description, media_urls, variety, preparation_time, created_at, updated_at
		)
		VALUES ($1, $2, $3, 'test description', '[]', 'koi', 'same_day', NOW(), NOW())
	`, id, sellerID, title)
	require.NoError(t, err)

	return id
}

func seedForSale(t *testing.T, ctx context.Context, appDB *db.DB, sellerID uuid.UUID, title, status string) uuid.UUID {
	t.Helper()

	productID := seedProduct(t, ctx, appDB, sellerID, title)
	saleID := uuid.New()
	_, err := appDB.Pool().Exec(ctx, `
		INSERT INTO for_sales (
			id, product_id, seller_id, price_per_unit, negotiation_enabled, status,
			published_at, sold_at, withdrawn_at, quantity_available, created_at, updated_at
		)
		VALUES ($1, $2, $3, 10000, false, $4::for_sale_status_enum, NOW(), NULL, NULL, 1, NOW(), NOW())
	`, saleID, productID, sellerID, status)
	require.NoError(t, err)

	return saleID
}

func seedAuction(t *testing.T, ctx context.Context, appDB *db.DB, sellerID uuid.UUID, status string) uuid.UUID {
	t.Helper()

	productID := seedProduct(t, ctx, appDB, sellerID, "auction-"+status)
	auctionID := uuid.New()
	now := time.Now().UTC()
	_, err := appDB.Pool().Exec(ctx, `
		INSERT INTO auctions (
			id, seller_id, product_id, order_id, settlement_deadline,
			start_price, bid_increment, buy_now_price, start_at, end_at,
			current_bid, current_winner_id,
			status, created_at, updated_at, anti_snipe_extension_seconds
		)
		VALUES ($1, $2, $3, NULL, NULL,
			10000, 1000, NULL, $4, $5, NULL, NULL,
			$6::auction_status_enum, NOW(), NOW(), 0)
	`, auctionID, sellerID, productID, now.Add(time.Hour), now.Add(2*time.Hour), status)
	require.NoError(t, err)

	return auctionID
}
