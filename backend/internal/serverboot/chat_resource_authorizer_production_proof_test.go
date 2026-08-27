//go:build integration

package serverboot

import (
	"context"
	"testing"

	"github.com/google/uuid"
	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	fpsEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	"github.com/labuda/backend/internal/identity/auth"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	chatRepo "github.com/labuda/backend/internal/interaction/chat/repository"
	contentEntity "github.com/labuda/backend/internal/social/content/entity"
	socialInfraRepo "github.com/labuda/backend/internal/social/graph/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Production ResourceAuthorizer proof using the concrete serverboot adapter
// ============================================================================

func seedUser(t *testing.T, ctx context.Context, pool *db.DB, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Pool().Exec(ctx, `INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at) VALUES ($1,$2,$3,'active',NOW(),NOW())`, id, id.String(), id.String()+"@test.local")
	require.NoError(t, err)
	_, err = pool.Pool().Exec(ctx, `INSERT INTO user_profiles (user_id, username, created_at, updated_at) VALUES ($1,$2,NOW(),NOW())`, id, id.String())
	require.NoError(t, err)
	return id
}

// repository adapters (satisfy production interfaces with real SQL)

type contentQuerierAdapter struct{ db *db.DB }

func (a contentQuerierAdapter) GetByID(ctx context.Context, tx interface{}, id uuid.UUID) (*contentEntity.Content, error) {
	return nil, nil
}

type fpsRepoAdapter struct{ db *db.DB }

func (a *fpsRepoAdapter) GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*fpsEntity.ForSale, error) {
	var f fpsEntity.ForSale
	err := tx.QueryRow(ctx, `SELECT id, product_id, seller_id, price_per_unit, status FROM for_sales WHERE id=$1`, id).
		Scan(&f.ID, &f.ProductID, &f.SellerID, &f.PricePerUnit, &f.Status)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

type auctionRepoAdapter struct{ db *db.DB }

func (a *auctionRepoAdapter) GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*auctionEntity.Auction, error) {
	var auc auctionEntity.Auction
	err := tx.QueryRow(ctx, `SELECT id, product_id, seller_id, start_price, bid_increment, start_at, end_at, status FROM auctions WHERE id=$1`, id).
		Scan(&auc.ID, &auc.ProductID, &auc.SellerID, &auc.StartPrice, &auc.BidIncrement, &auc.StartAt, &auc.EndAt, &auc.Status)
	if err != nil {
		return nil, err
	}
	return &auc, nil
}

func TestProductionAuthorizer_DirectFPS_Success(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	appDB := db.NewFromPool(tdb.Pool())
	ctx := context.Background()

	seller := seedUser(t, ctx, appDB, "prod-seller")

	pid := uuid.New()
	_, err := appDB.Pool().Exec(ctx, `INSERT INTO payments (id, user_id, payment_number, midtrans_order_id, gross_amount, service_fee_amount, coin_discount_amount, coins_to_use, reference_type, reference_id, expired_at, created_at, updated_at) VALUES ($1,$2,'sub','mid',0,0,0,0,'subscription',$3,NOW()+INTERVAL'1y',NOW(),NOW())`, pid, seller, uuid.New())
	require.NoError(t, err)
	_, err = appDB.Pool().Exec(ctx, `INSERT INTO seller_subscriptions (id, user_id, status, started_at, expires_at, duration_days, amount_paid, payment_id, created_at, updated_at) VALUES ($1,$2,'active',NOW(),NOW()+INTERVAL'1y',365,0,$3,NOW(),NOW())`, uuid.New(), seller, pid)
	require.NoError(t, err)
	_, err = appDB.Pool().Exec(ctx, `INSERT INTO seller_profiles (user_id, store_name, status, created_at, updated_at) VALUES ($1,'prod store','active',NOW(),NOW())`, seller)
	require.NoError(t, err)

	productID := uuid.New()
	_, err = appDB.Pool().Exec(ctx, `INSERT INTO products (id, seller_id, title, description, variety, preparation_time, created_at, updated_at) VALUES ($1,$2,'p','','','same_day',NOW(),NOW())`, productID, seller)
	require.NoError(t, err)
	fpsID := uuid.New()
	_, err = appDB.Pool().Exec(ctx, `INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, status, created_at, updated_at) VALUES ($1,$2,$3,10000,'active',NOW(),NOW())`, fpsID, productID, seller)
	require.NoError(t, err)

	roleChecker := auth.NewRoleCheckerDB(appDB, nil)
	authorizer := newChatResourceAuthorizer(
		appDB,
		contentQuerierAdapter{db: appDB},
		socialInfraRepo.NewSocialRepository(),
		roleChecker,
		&fpsRepoAdapter{db: appDB},
		&auctionRepoAdapter{db: appDB},
	)

	var fallback []byte
	err = appDB.WithTx(ctx, func(tx db.Tx) error {
		var authErr error
		fallback, authErr = authorizer.AuthorizeDirect(ctx, tx, seller, chatEntity.ResourceOccurrenceResourceTypeForSale, fpsID)
		return authErr
	})
	require.NoError(t, err)
	assert.NotEmpty(t, fallback)
	assert.NotEqual(t, "{}", string(fallback))

	t.Logf("Production HasActiveSellerCapability reached and passed for direct FPS authorization")
}

func TestProductionAuthorizer_NoSellerCapability_Rejected(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	appDB := db.NewFromPool(tdb.Pool())
	ctx := context.Background()

	seller := seedUser(t, ctx, appDB, "nocap-seller")

	productID := uuid.New()
	_, err := appDB.Pool().Exec(ctx, `INSERT INTO products (id, seller_id, title, description, variety, preparation_time, created_at, updated_at) VALUES ($1,$2,'p','','','same_day',NOW(),NOW())`, productID, seller)
	require.NoError(t, err)
	fpsID := uuid.New()
	_, err = appDB.Pool().Exec(ctx, `INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, status, created_at, updated_at) VALUES ($1,$2,$3,10000,'active',NOW(),NOW())`, fpsID, productID, seller)
	require.NoError(t, err)

	roleChecker := auth.NewRoleCheckerDB(appDB, nil)
	authorizer := newChatResourceAuthorizer(
		appDB,
		contentQuerierAdapter{db: appDB},
		socialInfraRepo.NewSocialRepository(),
		roleChecker,
		&fpsRepoAdapter{db: appDB},
		&auctionRepoAdapter{db: appDB},
	)

	err = appDB.WithTx(ctx, func(tx db.Tx) error {
		_, authErr := authorizer.AuthorizeDirect(ctx, tx, seller, chatEntity.ResourceOccurrenceResourceTypeForSale, fpsID)
		return authErr
	})
	require.ErrorIs(t, err, chatRepo.ErrMarketAuthorityRequired)
}
