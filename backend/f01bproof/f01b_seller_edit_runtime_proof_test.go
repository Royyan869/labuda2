//go:build integration

package f01bproof

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	forsaleApp "github.com/labuda/backend/internal/commerce/forsale/application"
	forsaleEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	forsaleHttp "github.com/labuda/backend/internal/commerce/forsale/delivery/http"
	productEntity "github.com/labuda/backend/internal/commerce/product/entity"
	orderEntity "github.com/labuda/backend/internal/commerce/order/entity"
	"github.com/labuda/backend/internal/commerce/order/repository"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/labuda/backend/pkg/testdb"
	"go.uber.org/zap"
)

type fakeActorResolver struct{ allow bool }

func (f fakeActorResolver) ResolveActor(_ interface{}, userID uuid.UUID) (*capabilityEntity.Actor, error) {
	active := "active"
	var ss *string
	if f.allow {
		ss = &active
	}
	return &capabilityEntity.Actor{ID: userID, Role: "user", AccountStatus: "active", EmailVerified: true, SellerStatus: ss}, nil
}

type fakeRoleChecker struct{}

func (fakeRoleChecker) IsAdmin(_ context.Context, _ uuid.UUID) (bool, error) { return false, nil }
func (fakeRoleChecker) IsSeller(_ context.Context, _ uuid.UUID) (bool, error) { return true, nil }
func (fakeRoleChecker) HasActiveSellerCapability(_ context.Context, _ uuid.UUID) (bool, error) { return true, nil }
func (fakeRoleChecker) HasSellerProfile(_ context.Context, _ uuid.UUID) (bool, error) { return true, nil }

type fakeOrderRepo struct{}

func (fakeOrderRepo) CreateOrderTx(_ context.Context, _ db.Tx, _ *orderEntity.Order) error { return nil }
func (fakeOrderRepo) CreateOrderItemTx(_ context.Context, _ db.Tx, _ *orderEntity.OrderItem) error { return nil }
func (fakeOrderRepo) GetByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*orderEntity.Order, error) { return nil, nil }
func (fakeOrderRepo) GetForUpdate(_ context.Context, _ db.Tx, _ uuid.UUID) (*orderEntity.Order, error) { return nil, nil }
func (fakeOrderRepo) UpdateStatusTx(_ context.Context, _ db.Tx, _ *orderEntity.Order) error { return nil }
func (fakeOrderRepo) GetByPricingTokenID(_ context.Context, _ db.Tx, _ uuid.UUID) (*orderEntity.Order, error) { return nil, nil }
func (fakeOrderRepo) GetByIdempotencyKey(_ context.Context, _ db.Tx, _ uuid.UUID, _ string) (*orderEntity.Order, error) {
	return nil, nil
}
func (fakeOrderRepo) GetByShippingQuoteID(_ context.Context, _ db.Tx, _ uuid.UUID) (*orderEntity.Order, error) {
	return nil, nil
}
func (fakeOrderRepo) GetBlockingOrderByShippingQuoteID(_ context.Context, _ db.Tx, _ uuid.UUID) (*orderEntity.Order, error) {
	return nil, nil
}
func (fakeOrderRepo) CountValidOrdersByShippingQuoteID(_ context.Context, _ db.Tx, _ uuid.UUID) (int64, error) {
	return 0, nil
}
func (fakeOrderRepo) GetBySource(_ context.Context, _ db.Tx, _ string, _ uuid.UUID) (*orderEntity.Order, error) {
	return nil, nil
}
func (fakeOrderRepo) GetOrderItems(_ context.Context, _ db.Tx, _ uuid.UUID) ([]*orderEntity.OrderItem, error) {
	return nil, nil
}
func (fakeOrderRepo) FindOrdersForAutoComplete(_ context.Context, _ db.Tx, _ int) ([]uuid.UUID, error) {
	return nil, nil
}
func (fakeOrderRepo) FindOverdueOrdersForCancel(_ context.Context, _ db.Tx, _ int) ([]uuid.UUID, error) {
	return nil, nil
}
func (fakeOrderRepo) GetByOrderNumber(_ context.Context, _ db.Tx, _ string) (*orderEntity.Order, error) {
	return nil, nil
}
func (fakeOrderRepo) CreateShippingProofTx(_ context.Context, _ db.Tx, _ *orderEntity.ShippingProof) error {
	return nil
}
func (fakeOrderRepo) GetShippingProofsByOrderID(_ context.Context, _ db.Tx, _ uuid.UUID) ([]*orderEntity.ShippingProof, error) {
	return nil, nil
}
func (fakeOrderRepo) GetOrderStats(_ context.Context, _ db.Tx, _ uuid.UUID, _ bool) (*repository.OrderStats, error) {
	return &repository.OrderStats{}, nil
}
func (fakeOrderRepo) CountActiveOrdersByProduct(_ context.Context, _ db.Tx, _ uuid.UUID) (int64, error) {
	return 0, nil
}
func (fakeOrderRepo) CountAnyOrdersByProduct(_ context.Context, _ db.Tx, _ uuid.UUID) (int64, error) {
	return 0, nil
}

func seedUser(t *testing.T, ctx context.Context, tdb *testdb.TestDB) uuid.UUID {
	t.Helper()
	uid := uuid.New()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO users (id, firebase_uid, email, email_verified_at, account_status, created_at, updated_at) VALUES ($1,$2,$3,NOW(),'active',NOW(),NOW())`, uid, "fb-"+uid.String(), uid.String()+"@test.invalid")
		return err
	}))
	return uid
}

func TestF01B_SellerCanEditForSale_RealRuntimeProof(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())

	sellerA := seedUser(t, ctx, tdb)
	sellerB := seedUser(t, ctx, tdb)

	svc := forsaleApp.NewForSaleService(fakeActorResolver{allow: true}, fakeRoleChecker{})
	handler := forsaleHttp.NewForSaleHandler(svc, appDB, zap.NewNop(), fakeOrderRepo{})

	originalTitle := "F01B original title"
	originalDesc := "F01B original description"
	originalMedia := []string{"https://cdn.test/f01b-original.jpg"}
	originalPrice := int64(100_000)
	originalQty := 3

	var created *forsaleEntity.ForSale
	require.NoError(t, appDB.WithTx(ctx, func(tx db.Tx) error {
		var err error
		created, err = svc.Create(ctx, tx, forsaleApp.CreateForSaleInput{
			SellerID:           sellerA,
			Title:              originalTitle,
			Description:        originalDesc,
			MediaURLs:          originalMedia,
			Variety:            "Kohaku",
			SizeCM:             intPtr(30),
			AgeMonths:          intPtr(12),
			Gender:             strPtr("female"),
			Breeder:            strPtr("Acme Breeder"),
			Bloodline:          strPtr("Ogata"),
			Certificates:       []string{},
			ForSaleType:        forsaleEntity.ForSaleTypeFixedPrice,
			PricePerUnit:       money.New(originalPrice),
			QuantityAvailable:  originalQty,
			NegotiationEnabled: false,
			Visibility:         forsaleEntity.ForSaleVisibilityPrivate,
			PreparationTime:    forsaleEntity.PreparationTimeImmediate,
		})
		return err
	}))
	require.NotNil(t, created)
	saleID := created.ID
	productID := created.ProductID
	t.Logf("CREATE PROOF: seller=%s sale=%s product=%s", sellerA, saleID, productID)

	var pTitle, pDesc, pVariety string
	var pMediaRaw json.RawMessage
	var pSizeCM *int
	var dbPrice int64
	var dbQty int
	var dbStatus string
	var dbSellerID uuid.UUID
	var dbProductID uuid.UUID
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT title, description, media_urls, variety, size_cm FROM products WHERE id=$1`, productID).Scan(&pTitle, &pDesc, &pMediaRaw, &pVariety, &pSizeCM))
	require.Equal(t, originalTitle, pTitle)
	require.Equal(t, originalDesc, pDesc)
	require.Contains(t, string(pMediaRaw), "f01b-original.jpg")
	require.Equal(t, "Kohaku", pVariety)
	require.Equal(t, 30, *pSizeCM)
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT price_per_unit, quantity_available, status, seller_id, product_id FROM for_sales WHERE id=$1`, saleID).Scan(&dbPrice, &dbQty, &dbStatus, &dbSellerID, &dbProductID))
	require.Equal(t, originalPrice, dbPrice)
	require.Equal(t, originalQty, dbQty)
	require.Equal(t, "draft", dbStatus)
	require.Equal(t, sellerA, dbSellerID)
	require.Equal(t, productID, dbProductID)
	t.Logf("PERSISTENCE AFTER CREATE: title=%q media=%s price=%d qty=%d", pTitle, string(pMediaRaw), dbPrice, dbQty)

	var hasTitleCol bool
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='for_sales' AND column_name='title')`).Scan(&hasTitleCol))
	require.False(t, hasTitleCol, "for_sales must NOT have duplicate title column")

	editedTitle := "F01B edited title"
	editedDesc := "F01B edited description"
	editedMedia := []string{"https://cdn.test/f01b-edited.jpg", "https://cdn.test/f01b-edited2.mp4"}
	editedPrice := int64(250_000)
	editedVariety := "Showa"
	editedSizeCM := 45

	putBody := map[string]interface{}{"title": editedTitle, "description": editedDesc, "media_urls": editedMedia, "variety": editedVariety, "size_cm": editedSizeCM, "price": editedPrice, "negotiation_enabled": true}
	bodyBytes, _ := json.Marshal(putBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/for-sale/"+saleID.String(), bytes.NewReader(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: saleID.String()}}
	c.Set("userID", sellerA)
	handler.UpdateForSale(c)
	t.Logf("PUT status=%d body=%s", w.Code, w.Body.String())
	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
	var dataPart json.RawMessage
	if d, ok := raw["data"]; ok {
		dataPart = d
	} else {
		dataPart = w.Body.Bytes()
	}
	require.NoError(t, json.Unmarshal(dataPart, &resp))
	require.Equal(t, editedTitle, resp["title"])
	require.Equal(t, editedDesc, resp["description"])
	require.Equal(t, float64(editedPrice), resp["price"])

	var afterTitle, afterDesc, afterVariety string
	var afterMediaRaw json.RawMessage
	var afterSizeCM *int
	var afterPrice int64
	var afterQty int
	var afterNeg bool
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT title, description, media_urls, variety, size_cm FROM products WHERE id=$1`, productID).Scan(&afterTitle, &afterDesc, &afterMediaRaw, &afterVariety, &afterSizeCM))
	require.Equal(t, editedTitle, afterTitle)
	require.Equal(t, editedDesc, afterDesc)
	require.Contains(t, string(afterMediaRaw), "f01b-edited.jpg")
	require.Contains(t, string(afterMediaRaw), "f01b-edited2.mp4")
	require.Equal(t, editedVariety, afterVariety)
	require.Equal(t, editedSizeCM, *afterSizeCM)
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT price_per_unit, quantity_available, negotiation_enabled FROM for_sales WHERE id=$1`, saleID).Scan(&afterPrice, &afterQty, &afterNeg))
	require.Equal(t, editedPrice, afterPrice)
	require.Equal(t, originalQty, afterQty)
	require.True(t, afterNeg)
	t.Logf("DATABASE AFTER PUT: title=%q variety=%q price=%d qty=%d", afterTitle, afterVariety, afterPrice, afterQty)

	preFailTitle := afterTitle
	preFailPrice := afterPrice
	faultErr := appDB.WithTx(ctx, func(tx db.Tx) error {
		prod, err := svc.GetByID(ctx, tx, saleID)
		if err != nil {
			return err
		}
		prod.Product.Title = "F01B fault title should rollback"
		if err := svc.UpdateProduct(ctx, tx, prod.Product); err != nil {
			return err
		}
		prod.Status = forsaleEntity.ForSaleStatusSold
		prod.PricePerUnit = money.New(999_999)
		if err := svc.Update(ctx, tx, prod); err != nil {
			return err
		}
		return nil
	})
	require.Error(t, faultErr)
	t.Logf("FAULT err=%v", faultErr)
	var rolledTitle string
	var rolledPrice int64
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT title FROM products WHERE id=$1`, productID).Scan(&rolledTitle))
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT price_per_unit FROM for_sales WHERE id=$1`, saleID).Scan(&rolledPrice))
	require.Equal(t, preFailTitle, rolledTitle)
	require.Equal(t, preFailPrice, rolledPrice)

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/api/v1/for-sale/"+saleID.String(), nil)
	c2.Params = gin.Params{{Key: "id", Value: saleID.String()}}
	c2.Set("userID", sellerA)
	handler.GetForSale(c2)
	require.Equal(t, http.StatusOK, w2.Code)
	var raw2 map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &raw2))
	var data2 json.RawMessage
	if d, ok := raw2["data"]; ok {
		data2 = d
	} else {
		data2 = w2.Body.Bytes()
	}
	var getResp map[string]interface{}
	require.NoError(t, json.Unmarshal(data2, &getResp))
	require.Equal(t, editedTitle, getResp["title"])
	require.Equal(t, editedDesc, getResp["description"])
	mediaUrlsVal, ok := getResp["media_urls"]
	require.True(t, ok)
	mediaBytes, _ := json.Marshal(mediaUrlsVal)
	require.Contains(t, string(mediaBytes), "f01b-edited.jpg")
	require.Equal(t, float64(editedPrice), getResp["price"])
	t.Logf("GET PROOF: title=%q", getResp["title"])

	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	c3.Request = httptest.NewRequest(http.MethodPut, "/api/v1/for-sale/"+saleID.String(), bytes.NewReader(mustJSON(map[string]interface{}{"title": "hacked"})))
	c3.Request.Header.Set("Content-Type", "application/json")
	c3.Params = gin.Params{{Key: "id", Value: saleID.String()}}
	c3.Set("userID", sellerB)
	handler.UpdateForSale(c3)
	require.Equal(t, http.StatusForbidden, w3.Code)
	var afterHackTitle string
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT title FROM products WHERE id=$1`, productID).Scan(&afterHackTitle))
	require.Equal(t, editedTitle, afterHackTitle)

	qtyBefore := afterQty
	w4 := httptest.NewRecorder()
	c4, _ := gin.CreateTestContext(w4)
	quantityPayload := fmt.Sprintf(`{"quantity":9999,"title":%q}`, editedTitle)
	c4.Request = httptest.NewRequest(http.MethodPut, "/api/v1/for-sale/"+saleID.String(), bytes.NewReader([]byte(quantityPayload)))
	c4.Request.Header.Set("Content-Type", "application/json")
	c4.Params = gin.Params{{Key: "id", Value: saleID.String()}}
	c4.Set("userID", sellerA)
	handler.UpdateForSale(c4)
	var qtyAfter int
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT quantity_available FROM for_sales WHERE id=$1`, saleID).Scan(&qtyAfter))
	require.Equal(t, qtyBefore, qtyAfter)
	_ = w4
	t.Logf("F01B PROOF COMPLETE")
}

func intPtr(v int) *int       { return &v }
func strPtr(v string) *string { return &v }
func mustJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

var _ = productEntity.SellingSurfaceForSale
