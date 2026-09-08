//go:build integration

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	forsaleApp "github.com/labuda/backend/internal/commerce/forsale/application"
	forsaleEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	forsaleHttp "github.com/labuda/backend/internal/commerce/forsale/delivery/http"
	auctionApp "github.com/labuda/backend/internal/commerce/auction/application"
	auctionHttp "github.com/labuda/backend/internal/commerce/auction/delivery/http"
	productEntity "github.com/labuda/backend/internal/commerce/product/entity"
	productRepo "github.com/labuda/backend/internal/commerce/product/infrastructure/repository"
	orderEntity "github.com/labuda/backend/internal/commerce/order/entity"
	"github.com/labuda/backend/internal/commerce/order/repository"
	shippingApp "github.com/labuda/backend/internal/commerce/shipping/application"
	shippingInfraRepo "github.com/labuda/backend/internal/commerce/shipping/infrastructure/repository"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"go.uber.org/zap"
)

type liveFakeActorResolver struct{ allow bool }

func (f liveFakeActorResolver) ResolveActor(_ context.Context, userID uuid.UUID) (*capabilityEntity.Actor, error) {
	active := "active"
	var ss *string
	if f.allow {
		ss = &active
	}
	return &capabilityEntity.Actor{ID: userID, Role: "user", AccountStatus: "active", EmailVerified: true, SellerStatus: ss}, nil
}

type liveFakeRoleChecker struct{}

func (liveFakeRoleChecker) IsAdmin(_ context.Context, _ uuid.UUID) (bool, error) { return false, nil }
func (liveFakeRoleChecker) IsSeller(_ context.Context, _ uuid.UUID) (bool, error) { return true, nil }
func (liveFakeRoleChecker) HasActiveSellerCapability(_ context.Context, _ uuid.UUID) (bool, error) { return true, nil }
func (liveFakeRoleChecker) HasSellerProfile(_ context.Context, _ uuid.UUID) (bool, error) { return true, nil }

type liveFakeOrderRepo struct{}

func (liveFakeOrderRepo) CreateOrderTx(_ context.Context, _ db.Tx, _ *orderEntity.Order) error { return nil }
func (liveFakeOrderRepo) CreateOrderItemTx(_ context.Context, _ db.Tx, _ *orderEntity.OrderItem) error { return nil }
func (liveFakeOrderRepo) GetByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*orderEntity.Order, error) { return nil, nil }
func (liveFakeOrderRepo) GetForUpdate(_ context.Context, _ db.Tx, _ uuid.UUID) (*orderEntity.Order, error) { return nil, nil }
func (liveFakeOrderRepo) UpdateStatusTx(_ context.Context, _ db.Tx, _ *orderEntity.Order) error { return nil }
func (liveFakeOrderRepo) GetByPricingTokenID(_ context.Context, _ db.Tx, _ uuid.UUID) (*orderEntity.Order, error) { return nil, nil }
func (liveFakeOrderRepo) GetByIdempotencyKey(_ context.Context, _ db.Tx, _ uuid.UUID, _ string) (*orderEntity.Order, error) {
	return nil, nil
}
func (liveFakeOrderRepo) GetByShippingQuoteID(_ context.Context, _ db.Tx, _ uuid.UUID) (*orderEntity.Order, error) {
	return nil, nil
}
func (liveFakeOrderRepo) GetBlockingOrderByShippingQuoteID(_ context.Context, _ db.Tx, _ uuid.UUID) (*orderEntity.Order, error) {
	return nil, nil
}
func (liveFakeOrderRepo) CountValidOrdersByShippingQuoteID(_ context.Context, _ db.Tx, _ uuid.UUID) (int64, error) {
	return 0, nil
}
func (liveFakeOrderRepo) GetBySource(_ context.Context, _ db.Tx, _ string, _ uuid.UUID) (*orderEntity.Order, error) {
	return nil, nil
}
func (liveFakeOrderRepo) GetOrderItems(_ context.Context, _ db.Tx, _ uuid.UUID) ([]*orderEntity.OrderItem, error) {
	return nil, nil
}
func (liveFakeOrderRepo) FindOrdersForAutoComplete(_ context.Context, _ db.Tx, _ int) ([]uuid.UUID, error) {
	return nil, nil
}
func (liveFakeOrderRepo) FindOverdueOrdersForCancel(_ context.Context, _ db.Tx, _ int) ([]uuid.UUID, error) {
	return nil, nil
}
func (liveFakeOrderRepo) GetByOrderNumber(_ context.Context, _ db.Tx, _ string) (*orderEntity.Order, error) {
	return nil, nil
}
func (liveFakeOrderRepo) CreateShippingProofTx(_ context.Context, _ db.Tx, _ *orderEntity.ShippingProof) error {
	return nil
}
func (liveFakeOrderRepo) GetShippingProofsByOrderID(_ context.Context, _ db.Tx, _ uuid.UUID) ([]*orderEntity.ShippingProof, error) {
	return nil, nil
}
func (liveFakeOrderRepo) GetOrderStats(_ context.Context, _ db.Tx, _ uuid.UUID, _ bool) (*repository.OrderStats, error) {
	return &repository.OrderStats{}, nil
}
func (liveFakeOrderRepo) CountActiveOrdersByProduct(_ context.Context, _ db.Tx, _ uuid.UUID) (int64, error) {
	return 0, nil
}
func (liveFakeOrderRepo) CountAnyOrdersByProduct(_ context.Context, _ db.Tx, _ uuid.UUID) (int64, error) {
	return 0, nil
}

func seedLiveUser(t *testing.T, ctx context.Context, tdb *testdb.TestDB) uuid.UUID {
	t.Helper()
	uid := uuid.New()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO users (id, firebase_uid, email, email_verified_at, account_status, created_at, updated_at) VALUES ($1,$2,$3,NOW(),'active',NOW(),NOW())`, uid, "fb-"+uid.String(), uid.String()+"@test.invalid")
		return err
	}))
	return uid
}

func intPtrLive(v int) *int       { return &v }
func strPtrLive(v string) *string { return &v }

func TestForSale_DraftRemainsEditable_PositiveControl(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	seller := seedLiveUser(t, ctx, tdb)
	svc := forsaleApp.NewForSaleService(liveFakeActorResolver{allow: true}, liveFakeRoleChecker{})
	handler := forsaleHttp.NewForSaleHandler(svc, appDB, zap.NewNop(), liveFakeOrderRepo{})
	// Direct SQL setup to avoid service overhead (faster, no actor/role checks)
	productID := uuid.New()
	forSaleID := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `INSERT INTO products (id, seller_id, title, description, media_urls, variety, size_cm, preparation_time, selling_surface, created_at, updated_at) VALUES ($1,$2,'draft original','draft desc','["https://cdn.test/a.jpg"]','Kohaku',30,'immediate','for_sale',NOW(),NOW())`, productID, seller)
	require.NoError(t, err)
	_, err = tdb.Pool().Exec(ctx, `INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, negotiation_enabled, status, quantity_available, created_at, updated_at) VALUES ($1,$2,$3,100000,false,'draft',2,NOW(),NOW())`, forSaleID, productID, seller)
	require.NoError(t, err)
	created := &forsaleEntity.ForSale{ID: forSaleID, ProductID: productID}
	// draft edit should succeed
	body, _ := json.Marshal(map[string]interface{}{"title": "draft edited", "price": int64(200000)})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/for-sale/"+created.ID.String(), bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: created.ID.String()}}
	c.Set("userID", seller)
	handler.UpdateForSale(c)
	require.Equal(t, http.StatusOK, w.Code)
	var prodTitle string
	var fsPrice int64
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT title FROM products WHERE id=$1`, created.ProductID).Scan(&prodTitle))
	require.Equal(t, "draft edited", prodTitle)
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT price_per_unit FROM for_sales WHERE id=$1`, created.ID).Scan(&fsPrice))
	require.Equal(t, int64(200000), fsPrice)
}

func TestForSale_ActiveImmutable_ZeroOrders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	seller := seedLiveUser(t, ctx, tdb)
	svc := forsaleApp.NewForSaleService(liveFakeActorResolver{allow: true}, liveFakeRoleChecker{})
	handler := forsaleHttp.NewForSaleHandler(svc, appDB, zap.NewNop(), liveFakeOrderRepo{})
	productID := uuid.New()
	forSaleID := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `INSERT INTO products (id, seller_id, title, description, media_urls, variety, size_cm, preparation_time, selling_surface, created_at, updated_at) VALUES ($1,$2,'live original','live desc','["https://cdn.test/live.jpg"]','Kohaku',30,'immediate','for_sale',NOW(),NOW())`, productID, seller)
	require.NoError(t, err)
	_, err = tdb.Pool().Exec(ctx, `INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, negotiation_enabled, status, quantity_available, created_at, updated_at) VALUES ($1,$2,$3,100000,false,'draft',2,NOW(),NOW())`, forSaleID, productID, seller)
	require.NoError(t, err)
	created := &forsaleEntity.ForSale{ID: forSaleID, ProductID: productID}
	// Force to active via direct SQL (simulate publish without shipping guard for test)
	_, err = tdb.Pool().Exec(ctx, `UPDATE for_sales SET status='active', published_at=NOW(), updated_at=NOW() WHERE id=$1`, created.ID)
	require.NoError(t, err)
	// Attempt seller edit — should be rejected with LIVE_IMMUTABLE
	body, _ := json.Marshal(map[string]interface{}{"title": "hacked", "price": int64(999999), "media_urls": []string{"https://cdn.test/hacked.jpg"}})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/for-sale/"+created.ID.String(), bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: created.ID.String()}}
	c.Set("userID", seller)
	handler.UpdateForSale(c)
	require.Equal(t, http.StatusConflict, w.Code)
	require.Contains(t, w.Body.String(), "LIVE_IMMUTABLE")
	// DB unchanged
	var prodTitle string
	var prodMediaRaw []byte
	var fsPrice int64
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT title, media_urls FROM products WHERE id=$1`, created.ProductID).Scan(&prodTitle, &prodMediaRaw))
	require.Equal(t, "live original", prodTitle)
	require.NotContains(t, string(prodMediaRaw), "hacked")
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT price_per_unit FROM for_sales WHERE id=$1`, created.ID).Scan(&fsPrice))
	require.Equal(t, int64(100000), fsPrice)
}

func TestForSale_ShippingImmutable_WhenActive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	seller := seedLiveUser(t, ctx, tdb)
	// Direct SQL setup
	productID := uuid.New()
	forSaleID := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, selling_surface, created_at, updated_at) VALUES ($1,$2,'ship test','desc','["https://cdn.test/a.jpg"]','Kohaku','immediate','for_sale',NOW(),NOW())`, productID, seller)
	require.NoError(t, err)
	_, err = tdb.Pool().Exec(ctx, `INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, negotiation_enabled, status, quantity_available, created_at, updated_at) VALUES ($1,$2,$3,100000,false,'draft',1,NOW(),NOW())`, forSaleID, productID, seller)
	require.NoError(t, err)
	created := &forsaleEntity.ForSale{ID: forSaleID, ProductID: productID}
	// Create two shipping options for seller
	opt1 := uuid.New()
	opt2 := uuid.New()
	_, err = tdb.Pool().Exec(ctx, `INSERT INTO shipping_options (id, seller_id, name, transport_type, is_active, created_at, updated_at) VALUES ($1,$2,'opt1','custom',true,NOW(),NOW()), ($3,$4,'opt2','custom',true,NOW(),NOW())`, opt1, seller, opt2, seller)
	require.NoError(t, err)
	_, err = tdb.Pool().Exec(ctx, `INSERT INTO shipping_coverages (id, shipping_option_id, province_code, province_name, province_rate, is_available, created_at) VALUES ($1,$2,'31','DKI Jakarta',10000,true,NOW()), ($3,$4,'31','DKI Jakarta',12000,true,NOW())`, uuid.New(), opt1, uuid.New(), opt2)
	require.NoError(t, err)
	// Initially link opt1 while draft — should succeed
	shippingSetupRepo := shippingInfraRepo.NewShippingSetupRepository()
	productShippingRepo := shippingInfraRepo.NewProductShippingSetupRepository(shippingSetupRepo)
	shippingSvc := shippingApp.NewProductShippingService(
		&liveForSaleRepoAdapter{tdb: tdb},
		shippingSetupRepo,
		productShippingRepo,
		liveFakeOrderRepo{},
	)
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		return shippingSvc.SetProductShippingSetups(ctx, tx, shippingApp.SetProductShippingSetupsInput{
			ProductID:        created.ProductID,
			SellerID:         seller,
			ShippingSetupIDs: []uuid.UUID{opt1},
		})
	}))
	var cnt int64
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM product_shipping_options WHERE product_id=$1`, created.ProductID).Scan(&cnt))
	require.Equal(t, int64(1), cnt)
	// Publish to active (direct SQL)
	_, err = tdb.Pool().Exec(ctx, `UPDATE for_sales SET status='active', published_at=NOW(), updated_at=NOW() WHERE id=$1`, created.ID)
	require.NoError(t, err)
	// Attempt to change shipping while active — must be rejected even with zero orders
	err = appDB.WithTx(ctx, func(tx db.Tx) error {
		return shippingSvc.SetProductShippingSetups(ctx, tx, shippingApp.SetProductShippingSetupsInput{
			ProductID:        created.ProductID,
			SellerID:         seller,
			ShippingSetupIDs: []uuid.UUID{opt2},
		})
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "LIVE_IMMUTABLE")
	// DB unchanged
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM product_shipping_options WHERE product_id=$1`, created.ProductID).Scan(&cnt))
	require.Equal(t, int64(1), cnt)
	var linkedOpt uuid.UUID
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT shipping_option_id FROM product_shipping_options WHERE product_id=$1`, created.ProductID).Scan(&linkedOpt))
	require.Equal(t, opt1, linkedOpt)
}

func TestForSale_ShippingMutable_WhenDraft(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	seller := seedLiveUser(t, ctx, tdb)
	productID := uuid.New()
	forSaleID := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, selling_surface, created_at, updated_at) VALUES ($1,$2,'draft ship','desc','["https://cdn.test/a.jpg"]','Kohaku','immediate','for_sale',NOW(),NOW())`, productID, seller)
	require.NoError(t, err)
	_, err = tdb.Pool().Exec(ctx, `INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, negotiation_enabled, status, quantity_available, created_at, updated_at) VALUES ($1,$2,$3,50000,false,'draft',1,NOW(),NOW())`, forSaleID, productID, seller)
	require.NoError(t, err)
	created := &forsaleEntity.ForSale{ID: forSaleID, ProductID: productID}
	opt1 := uuid.New()
	_, err = tdb.Pool().Exec(ctx, `INSERT INTO shipping_options (id, seller_id, name, transport_type, is_active, created_at, updated_at) VALUES ($1,$2,'optA','custom',true,NOW(),NOW())`, opt1, seller)
	require.NoError(t, err)
	_, err = tdb.Pool().Exec(ctx, `INSERT INTO shipping_coverages (id, shipping_option_id, province_code, province_name, province_rate, is_available, created_at) VALUES ($1,$2,'31','DKI',10000,true,NOW())`, uuid.New(), opt1)
	require.NoError(t, err)
	shippingSetupRepo := shippingInfraRepo.NewShippingSetupRepository()
	productShippingRepo := shippingInfraRepo.NewProductShippingSetupRepository(shippingSetupRepo)
	shippingSvc := shippingApp.NewProductShippingService(
		&liveForSaleRepoAdapter{tdb: tdb},
		shippingSetupRepo,
		productShippingRepo,
		liveFakeOrderRepo{},
	)
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		return shippingSvc.SetProductShippingSetups(ctx, tx, shippingApp.SetProductShippingSetupsInput{
			ProductID:        created.ProductID,
			SellerID:         seller,
			ShippingSetupIDs: []uuid.UUID{opt1},
		})
	}))
	var cnt int64
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM product_shipping_options WHERE product_id=$1`, created.ProductID).Scan(&cnt))
	require.Equal(t, int64(1), cnt)
}

func TestAuction_ShippingLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	seller := seedLiveUser(t, ctx, tdb)
	// Create product + auction draft via direct SQL (simpler than service which auto-schedules)
	productID := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, selling_surface, created_at, updated_at) VALUES ($1,$2,'auction prod','desc','[]','Kohaku','immediate','auction',NOW(),NOW())`, productID, seller)
	require.NoError(t, err)
	auctionID := uuid.New()
	startAt := time.Now().Add(2 * time.Hour)
	endAt := startAt.Add(48 * time.Hour)
	_, err = tdb.Pool().Exec(ctx, `INSERT INTO auctions (id, seller_id, product_id, start_price, bid_increment, start_at, end_at, status, created_at, updated_at) VALUES ($1,$2,$3,100000,10000,$4,$5,'draft',NOW(),NOW())`, auctionID, seller, productID, startAt, endAt)
	require.NoError(t, err)
	opt1 := uuid.New()
	opt2 := uuid.New()
	_, err = tdb.Pool().Exec(ctx, `INSERT INTO shipping_options (id, seller_id, name, transport_type, is_active, created_at, updated_at) VALUES ($1,$2,'aOpt1','custom',true,NOW(),NOW()), ($3,$4,'aOpt2','custom',true,NOW(),NOW())`, opt1, seller, opt2, seller)
	require.NoError(t, err)
	_, err = tdb.Pool().Exec(ctx, `INSERT INTO shipping_coverages (id, shipping_option_id, province_code, province_name, province_rate, is_available, created_at) VALUES ($1,$2,'31','DKI',10000,true,NOW()), ($3,$4,'31','DKI',10000,true,NOW())`, uuid.New(), opt1, uuid.New(), opt2)
	require.NoError(t, err)
	shippingSetupRepo := shippingInfraRepo.NewShippingSetupRepository()
	productShippingRepo := shippingInfraRepo.NewProductShippingSetupRepository(shippingSetupRepo)
	shippingSvc := shippingApp.NewProductShippingService(
		&liveForSaleRepoAdapter{tdb: tdb},
		shippingSetupRepo,
		productShippingRepo,
		liveFakeOrderRepo{},
	)
	// draft → shipping allowed
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		return shippingSvc.SetProductShippingSetups(ctx, tx, shippingApp.SetProductShippingSetupsInput{ProductID: productID, SellerID: seller, ShippingSetupIDs: []uuid.UUID{opt1}})
	}))
	// scheduled → shipping blocked
	_, err = tdb.Pool().Exec(ctx, `UPDATE auctions SET status='scheduled', updated_at=NOW() WHERE id=$1`, auctionID)
	require.NoError(t, err)
	err = appDB.WithTx(ctx, func(tx db.Tx) error {
		return shippingSvc.SetProductShippingSetups(ctx, tx, shippingApp.SetProductShippingSetupsInput{ProductID: productID, SellerID: seller, ShippingSetupIDs: []uuid.UUID{opt2}})
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "LIVE_IMMUTABLE")
	var cnt int64
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM product_shipping_options WHERE product_id=$1`, productID).Scan(&cnt))
	require.Equal(t, int64(1), cnt)
	// active → shipping blocked
	_, err = tdb.Pool().Exec(ctx, `UPDATE auctions SET status='active', updated_at=NOW() WHERE id=$1`, auctionID)
	require.NoError(t, err)
	err = appDB.WithTx(ctx, func(tx db.Tx) error {
		return shippingSvc.SetProductShippingSetups(ctx, tx, shippingApp.SetProductShippingSetupsInput{ProductID: productID, SellerID: seller, ShippingSetupIDs: []uuid.UUID{opt2}})
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "LIVE_IMMUTABLE")
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM product_shipping_options WHERE product_id=$1`, productID).Scan(&cnt))
	require.Equal(t, int64(1), cnt)
	// waiting_settlement → blocked
	_, err = tdb.Pool().Exec(ctx, `UPDATE auctions SET status='waiting_settlement', updated_at=NOW() WHERE id=$1`, auctionID)
	require.NoError(t, err)
	err = appDB.WithTx(ctx, func(tx db.Tx) error {
		return shippingSvc.SetProductShippingSetups(ctx, tx, shippingApp.SetProductShippingSetupsInput{ProductID: productID, SellerID: seller, ShippingSetupIDs: []uuid.UUID{}})
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "LIVE_IMMUTABLE")
}

func TestAuction_DirectEdit_ActiveRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	seller := seedLiveUser(t, ctx, tdb)
	productID := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, selling_surface, created_at, updated_at) VALUES ($1,$2,'orig','desc','[]','Kohaku','immediate','auction',NOW(),NOW())`, productID, seller)
	require.NoError(t, err)
	auctionID := uuid.New()
	startAt := time.Now().Add(-1 * time.Hour)
	endAt := time.Now().Add(48 * time.Hour)
	_, err = tdb.Pool().Exec(ctx, `INSERT INTO auctions (id, seller_id, product_id, start_price, bid_increment, start_at, end_at, status, created_at, updated_at) VALUES ($1,$2,$3,100000,10000,$4,$5,'active',NOW(),NOW())`, auctionID, seller, productID, startAt, endAt)
	require.NoError(t, err)
	auctionProdRepo := productRepo.NewProductRepository()
	auctionSvc := auctionApp.NewAuctionService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
	auctionSvc.SetProductRepo(auctionProdRepo)
	handler := auctionHttp.NewAuctionHandler(auctionSvc, auctionProdRepo, nil, appDB, zap.NewNop())
	body, _ := json.Marshal(map[string]interface{}{"title": "hacked", "start_price": int64(999999)})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/auctions/"+auctionID.String(), bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: auctionID.String()}}
	c.Set("userID", seller)
	handler.UpdateAuction(c)
	require.Equal(t, http.StatusConflict, w.Code)
	var title string
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT title FROM products WHERE id=$1`, productID).Scan(&title))
	require.Equal(t, "orig", title)
	var startPrice int64
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT start_price FROM auctions WHERE id=$1`, auctionID).Scan(&startPrice))
	require.Equal(t, int64(100000), startPrice)
}

func TestAuction_DraftFullProduct_Persists(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	seller := seedLiveUser(t, ctx, tdb)
	productID := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `INSERT INTO products (id, seller_id, title, description, media_urls, variety, size_cm, preparation_time, selling_surface, created_at, updated_at) VALUES ($1,$2,'orig','desc','[]','Kohaku',30,'immediate','auction',NOW(),NOW())`, productID, seller)
	require.NoError(t, err)
	auctionID := uuid.New()
	startAt := time.Now().Add(48 * time.Hour)
	endAt := startAt.Add(24 * time.Hour)
	_, err = tdb.Pool().Exec(ctx, `INSERT INTO auctions (id, seller_id, product_id, start_price, bid_increment, start_at, end_at, status, created_at, updated_at) VALUES ($1,$2,$3,100000,10000,$4,$5,'draft',NOW(),NOW())`, auctionID, seller, productID, startAt, endAt)
	require.NoError(t, err)
	auctionProdRepo := productRepo.NewProductRepository()
	auctionSvc := auctionApp.NewAuctionService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
	auctionSvc.SetProductRepo(auctionProdRepo)
	handler := auctionHttp.NewAuctionHandler(auctionSvc, auctionProdRepo, nil, appDB, zap.NewNop())
	body, _ := json.Marshal(map[string]interface{}{
		"title": "New Title", "description": "New Desc", "media_urls": []string{"https://a.jpg", "https://b.mp4"}, "variety": "Showa", "size_cm": 45, "age_months": 12, "gender": "male", "breeder": "Sakai", "bloodline": "Matsu", "certificates": []string{"breeder", "health"}, "preparation_time": "short", "preparation_note": "note", "start_price": int64(150000), "bid_increment": int64(15000), "start_at": startAt.Add(time.Hour).Format(time.RFC3339), "end_at": endAt.Add(time.Hour).Format(time.RFC3339),
	})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/auctions/"+auctionID.String(), bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: auctionID.String()}}
	c.Set("userID", seller)
	handler.UpdateAuction(c)
	require.Equal(t, http.StatusOK, w.Code)
	var title, desc, variety, prep, gender string
	var mediaRaw []byte
	var certs []string
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT title, description, media_urls, variety, preparation_time, gender, certificates FROM products WHERE id=$1`, productID).Scan(&title, &desc, &mediaRaw, &variety, &prep, &gender, &certs))
	require.Equal(t, "New Title", title)
	require.Equal(t, "New Desc", desc)
	require.Contains(t, string(mediaRaw), "a.jpg")
	require.Equal(t, "Showa", variety)
	require.Equal(t, "short", prep)
	require.Equal(t, "male", gender)
	require.Contains(t, certs[0], "breeder")
	var sp, bi int64
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT start_price, bid_increment FROM auctions WHERE id=$1`, auctionID).Scan(&sp, &bi))
	require.Equal(t, int64(150000), sp)
	require.Equal(t, int64(15000), bi)
}

func TestAuction_ScheduledRejectsPricingAndMedia(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	seller := seedLiveUser(t, ctx, tdb)
	productID := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, selling_surface, created_at, updated_at) VALUES ($1,$2,'orig','desc','[]','Kohaku','immediate','auction',NOW(),NOW())`, productID, seller)
	require.NoError(t, err)
	auctionID := uuid.New()
	startAt := time.Now().Add(48 * time.Hour)
	endAt := startAt.Add(24 * time.Hour)
	_, err = tdb.Pool().Exec(ctx, `INSERT INTO auctions (id, seller_id, product_id, start_price, bid_increment, start_at, end_at, status, created_at, updated_at) VALUES ($1,$2,$3,100000,10000,$4,$5,'scheduled',NOW(),NOW())`, auctionID, seller, productID, startAt, endAt)
	require.NoError(t, err)
	auctionProdRepo := productRepo.NewProductRepository()
	auctionSvc := auctionApp.NewAuctionService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
	auctionSvc.SetProductRepo(auctionProdRepo)
	handler := auctionHttp.NewAuctionHandler(auctionSvc, auctionProdRepo, nil, appDB, zap.NewNop())
	// Try to change pricing + media on scheduled — must be rejected
	body, _ := json.Marshal(map[string]interface{}{"media_urls": []string{"https://hack.jpg"}, "start_price": int64(999999)})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/auctions/"+auctionID.String(), bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: auctionID.String()}}
	c.Set("userID", seller)
	handler.UpdateAuction(c)
	require.True(t, w.Code == http.StatusConflict || w.Code == http.StatusBadRequest, "expected 409 or 400, got %d body %s", w.Code, w.Body.String())
	var mediaRaw []byte
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT media_urls FROM products WHERE id=$1`, productID).Scan(&mediaRaw))
	require.NotContains(t, string(mediaRaw), "hack")
	var sp int64
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT start_price FROM auctions WHERE id=$1`, auctionID).Scan(&sp))
	require.Equal(t, int64(100000), sp)
}

func TestAuction_ActiveFullProductRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	seller := seedLiveUser(t, ctx, tdb)
	productID := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, selling_surface, created_at, updated_at) VALUES ($1,$2,'orig','desc','[]','Kohaku','immediate','auction',NOW(),NOW())`, productID, seller)
	require.NoError(t, err)
	auctionID := uuid.New()
	startAt := time.Now().Add(-1 * time.Hour)
	endAt := time.Now().Add(48 * time.Hour)
	_, err = tdb.Pool().Exec(ctx, `INSERT INTO auctions (id, seller_id, product_id, start_price, bid_increment, start_at, end_at, status, created_at, updated_at) VALUES ($1,$2,$3,100000,10000,$4,$5,'active',NOW(),NOW())`, auctionID, seller, productID, startAt, endAt)
	require.NoError(t, err)
	auctionProdRepo := productRepo.NewProductRepository()
	auctionSvc := auctionApp.NewAuctionService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
	auctionSvc.SetProductRepo(auctionProdRepo)
	handler := auctionHttp.NewAuctionHandler(auctionSvc, auctionProdRepo, nil, appDB, zap.NewNop())
	body, _ := json.Marshal(map[string]interface{}{"title": "hacked", "media_urls": []string{"https://hack.jpg"}, "certificates": []string{"breeder"}, "preparation_time": "short", "start_price": int64(999999)})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/auctions/"+auctionID.String(), bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: auctionID.String()}}
	c.Set("userID", seller)
	handler.UpdateAuction(c)
	require.Equal(t, http.StatusConflict, w.Code)
	var title string
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT title FROM products WHERE id=$1`, productID).Scan(&title))
	require.Equal(t, "orig", title)
	var sp int64
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT start_price FROM auctions WHERE id=$1`, auctionID).Scan(&sp))
	require.Equal(t, int64(100000), sp)
}

// Minimal adapter to satisfy ProductShippingService product existence check (looks up products table)
type liveForSaleRepoAdapter struct{ tdb *testdb.TestDB }

func (a *liveForSaleRepoAdapter) GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*forsaleEntity.ForSale, error) {
	row := tx.QueryRow(ctx, `SELECT id, seller_id, title, description, media_urls, variety, preparation_time FROM products WHERE id=$1`, id)
	var p productEntity.Product
	var mediaRaw []byte
	var title, desc, variety, prep string
	var sellerID uuid.UUID
	var pid uuid.UUID
	if err := row.Scan(&pid, &sellerID, &title, &desc, &mediaRaw, &variety, &prep); err != nil {
		return nil, err
	}
	p.ID = pid
	p.SellerID = sellerID
	p.Title = title
	p.Description = desc
	p.Variety = variety
	p.PreparationTime = prep
	return &forsaleEntity.ForSale{Product: &p, SellerID: sellerID}, nil
}

