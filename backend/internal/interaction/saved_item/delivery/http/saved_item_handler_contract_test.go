package http

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
	auctionRepo "github.com/labuda/backend/internal/commerce/auction/infrastructure/repository"
	savedItemApp "github.com/labuda/backend/internal/interaction/saved_item/application"
	savedItemRepo "github.com/labuda/backend/internal/interaction/saved_item/infrastructure/repository"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type savedItemCreatedEnvelope struct {
	Success bool                 `json:"success"`
	Data    savedItemCreatedData `json:"data"`
	Error   *response.ErrorInfo  `json:"error,omitempty"`
}

type savedItemCreatedData struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	TargetType string    `json:"target_type"`
	TargetID   string    `json:"target_id"`
	IntentType string    `json:"intent_type"`
	SellerID   *string   `json:"seller_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type savedItemsEnvelope struct {
	Success bool                `json:"success"`
	Data    savedItemsListData  `json:"data"`
	Error   *response.ErrorInfo `json:"error,omitempty"`
}

type savedItemsListData struct {
	UserID   string                 `json:"user_id"`
	Items    []savedItemForSaleData `json:"items"`
	Auctions []savedItemAuctionData `json:"auctions"`
	Total    int                    `json:"total"`
	Page     int                    `json:"page"`
	PerPage  int                    `json:"per_page"`
}

type savedItemForSaleData struct {
	savedItemCreatedData
	ForSaleTitle      *string  `json:"for_sale_title,omitempty"`
	ForSalePrice      *int64   `json:"for_sale_price,omitempty"`
	ForSaleType       *string  `json:"for_sale_type,omitempty"`
	QuantityAvailable *int     `json:"quantity_available,omitempty"`
	ForSaleStatus     *string  `json:"for_sale_status,omitempty"`
	ForSaleVisibility *string  `json:"for_sale_visibility,omitempty"`
	ForSaleMediaURLs  []string `json:"for_sale_media_urls,omitempty"`
}

type savedItemAuctionData struct {
	savedItemCreatedData
	AuctionTitle  *string    `json:"auction_title,omitempty"`
	AuctionStatus *string    `json:"auction_status,omitempty"`
	StartPrice    *int64     `json:"start_price,omitempty"`
	CurrentBid    *int64     `json:"current_bid,omitempty"`
	EndAt         *time.Time `json:"end_at,omitempty"`
}

func setupSavedItemHandlerTest(t *testing.T) (*testdb.TestDB, *SavedItemHandler, uuid.UUID, uuid.UUID) {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	viewerID := uuid.New()
	sellerID := uuid.New()

	ctx := context.Background()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO users (id, firebase_uid, email)
			VALUES ($1, $2, $3), ($4, $5, $6)
		`, viewerID, "fb-"+viewerID.String(), viewerID.String()+"@saved-item.test",
			sellerID, "fb-"+sellerID.String(), sellerID.String()+"@saved-item.test")
		return err
	}))

	savedRepo := savedItemRepo.NewSavedItemRepository(db.NewFromPool(tdb.Pool()))
	service := savedItemApp.NewSavedItemService(nil)
	service.SetDB(db.NewFromPool(tdb.Pool()))
	service.SetSavedItemRepository(savedRepo)
	service.SetAuctionRepository(*auctionRepo.NewAuctionRepository())

	handler := NewSavedItemHandler(service, db.NewFromPool(tdb.Pool()), zap.NewNop())
	return tdb, handler, viewerID, sellerID
}

func insertForSaleFixture(t *testing.T, tdb *testdb.TestDB, sellerID uuid.UUID) (uuid.UUID, uuid.UUID) {
	t.Helper()

	productID := uuid.New()
	forSaleID := uuid.New()
	ctx := context.Background()

	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, productID, sellerID, "Showa Koi", "A fine showa", `["https://cdn.example.com/koi.jpg"]`, "showa", "immediate"); err != nil {
			return err
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, status, published_at, quantity_available)
			VALUES ($1, $2, $3, $4, 'active', NOW(), $5)
		`, forSaleID, productID, sellerID, int64(1250000), 3)
		return err
	}))

	return forSaleID, productID
}

func insertAuctionFixture(t *testing.T, tdb *testdb.TestDB, sellerID uuid.UUID) uuid.UUID {
	t.Helper()

	auctionID := uuid.New()
	productID := uuid.New()
	now := time.Now().UTC().Add(-time.Minute)
	ctx := context.Background()

	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, productID, sellerID, "Auction Product", "A live auction product", `["https://cdn.example.com/auction.jpg"]`, "showa", "immediate"); err != nil {
			return err
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO auctions (
				id, seller_id, product_id,
				start_price, bid_increment, start_at, end_at, status, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 'active', NOW(), NOW())
		`, auctionID, sellerID, productID, int64(1500000), int64(100000), now, now.Add(2*time.Hour))
		return err
	}))

	return auctionID
}

func addSavedItemRequest(t *testing.T, handler *SavedItemHandler, userID uuid.UUID, targetType, targetID string) *httptest.ResponseRecorder {
	t.Helper()

	body := []byte(`{"target_type":"` + targetType + `","target_id":"` + targetID + `"}`)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/saved-items", bytes.NewReader(body))
	c.Set("userID", userID)

	handler.AddSavedItem(c)
	return w
}

func getSavedItemsRequest(t *testing.T, handler *SavedItemHandler, userID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/saved-items", nil)
	c.Set("userID", userID)

	handler.GetSavedItems(c)
	return w
}

func removeSavedItemRequest(t *testing.T, handler *SavedItemHandler, userID uuid.UUID, targetType, targetID string) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/saved-items/"+targetID+"?type="+targetType, nil)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: targetID}}
	c.Set("userID", userID)

	handler.RemoveSavedItem(c)
	return w
}

func TestSavedItemHandler_PersistsAndFetchesMixedForSaleAndAuctionContracts(t *testing.T) {
	tdb, handler, viewerID, sellerID := setupSavedItemHandlerTest(t)
	forSaleID, _ := insertForSaleFixture(t, tdb, sellerID)
	auctionID := insertAuctionFixture(t, tdb, sellerID)

	forSaleResp := addSavedItemRequest(t, handler, viewerID, "for_sale", forSaleID.String())
	require.Equal(t, http.StatusCreated, forSaleResp.Code)

	var createdForSale savedItemCreatedEnvelope
	require.NoError(t, json.Unmarshal(forSaleResp.Body.Bytes(), &createdForSale))
	require.True(t, createdForSale.Success)
	require.Equal(t, "for_sale", createdForSale.Data.TargetType)
	require.Equal(t, forSaleID.String(), createdForSale.Data.TargetID)
	require.Equal(t, "bookmark", createdForSale.Data.IntentType)
	require.NotNil(t, createdForSale.Data.SellerID)
	require.Equal(t, sellerID.String(), *createdForSale.Data.SellerID)

	require.Equal(t, http.StatusCreated, addSavedItemRequest(t, handler, viewerID, "for_sale", forSaleID.String()).Code)

	auctionResp := addSavedItemRequest(t, handler, viewerID, "auction", auctionID.String())
	require.Equal(t, http.StatusCreated, auctionResp.Code)

	var createdAuction savedItemCreatedEnvelope
	require.NoError(t, json.Unmarshal(auctionResp.Body.Bytes(), &createdAuction))
	require.True(t, createdAuction.Success)
	require.Equal(t, "auction", createdAuction.Data.TargetType)
	require.Equal(t, auctionID.String(), createdAuction.Data.TargetID)
	require.Equal(t, "watch", createdAuction.Data.IntentType)
	require.Nil(t, createdAuction.Data.SellerID)

	require.Equal(t, http.StatusCreated, addSavedItemRequest(t, handler, viewerID, "auction", auctionID.String()).Code)

	getResp := getSavedItemsRequest(t, handler, viewerID)
	require.Equal(t, http.StatusOK, getResp.Code)

	var fetched savedItemsEnvelope
	require.NoError(t, json.Unmarshal(getResp.Body.Bytes(), &fetched))
	require.True(t, fetched.Success)
	require.Equal(t, viewerID.String(), fetched.Data.UserID)
	require.Equal(t, 2, fetched.Data.Total)
	require.Len(t, fetched.Data.Items, 1)
	require.Len(t, fetched.Data.Auctions, 1)

	fetchedForSale := fetched.Data.Items[0]
	require.Equal(t, "for_sale", fetchedForSale.TargetType)
	require.Equal(t, forSaleID.String(), fetchedForSale.TargetID)
	require.NotNil(t, fetchedForSale.SellerID)
	require.Equal(t, sellerID.String(), *fetchedForSale.SellerID)
	require.NotNil(t, fetchedForSale.ForSaleTitle)
	require.Equal(t, "Showa Koi", *fetchedForSale.ForSaleTitle)
	require.NotNil(t, fetchedForSale.ForSalePrice)
	require.EqualValues(t, 1250000, *fetchedForSale.ForSalePrice)
	require.NotNil(t, fetchedForSale.ForSaleType)
	require.Equal(t, "fixed_price", *fetchedForSale.ForSaleType)
	require.NotNil(t, fetchedForSale.QuantityAvailable)
	require.Equal(t, 3, *fetchedForSale.QuantityAvailable)
	require.NotNil(t, fetchedForSale.ForSaleStatus)
	require.Equal(t, "active", *fetchedForSale.ForSaleStatus)
	require.NotNil(t, fetchedForSale.ForSaleVisibility)
	require.Equal(t, "public", *fetchedForSale.ForSaleVisibility)
	require.Equal(t, []string{"https://cdn.example.com/koi.jpg"}, fetchedForSale.ForSaleMediaURLs)

	fetchedAuction := fetched.Data.Auctions[0]
	require.Equal(t, "auction", fetchedAuction.TargetType)
	require.Equal(t, auctionID.String(), fetchedAuction.TargetID)
	require.Equal(t, "watch", fetchedAuction.IntentType)
	require.Nil(t, fetchedAuction.SellerID)
	require.NotNil(t, fetchedAuction.AuctionTitle)
	require.Equal(t, "Auction Koi", *fetchedAuction.AuctionTitle)
	require.NotNil(t, fetchedAuction.AuctionStatus)
	require.Equal(t, "active", *fetchedAuction.AuctionStatus)

	removeForSaleResp := removeSavedItemRequest(t, handler, viewerID, "for_sale", forSaleID.String())
	require.Equal(t, http.StatusOK, removeForSaleResp.Code)

	getAfterForSaleRemove := getSavedItemsRequest(t, handler, viewerID)
	require.Equal(t, http.StatusOK, getAfterForSaleRemove.Code)
	var afterForSaleRemove savedItemsEnvelope
	require.NoError(t, json.Unmarshal(getAfterForSaleRemove.Body.Bytes(), &afterForSaleRemove))
	require.Equal(t, 1, afterForSaleRemove.Data.Total)
	require.Len(t, afterForSaleRemove.Data.Items, 0)
	require.Len(t, afterForSaleRemove.Data.Auctions, 1)

	removeAuctionResp := removeSavedItemRequest(t, handler, viewerID, "auction", auctionID.String())
	require.Equal(t, http.StatusOK, removeAuctionResp.Code)

	getAfterAuctionRemove := getSavedItemsRequest(t, handler, viewerID)
	require.Equal(t, http.StatusOK, getAfterAuctionRemove.Code)
	var afterAuctionRemove savedItemsEnvelope
	require.NoError(t, json.Unmarshal(getAfterAuctionRemove.Body.Bytes(), &afterAuctionRemove))
	require.Equal(t, 0, afterAuctionRemove.Data.Total)
	require.Len(t, afterAuctionRemove.Data.Items, 0)
	require.Len(t, afterAuctionRemove.Data.Auctions, 0)
}

func TestSavedItemHandler_FailClosedOnOwnForSaleAndEndedAuction(t *testing.T) {
	tdb, handler, viewerID, sellerID := setupSavedItemHandlerTest(t)
	ownForSaleID, _ := insertForSaleFixture(t, tdb, viewerID)
	endedAuctionID := insertAuctionFixture(t, tdb, sellerID)

	ctx := context.Background()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE auctions SET status = 'ended' WHERE id = $1
		`, endedAuctionID)
		return err
	}))

	ownForSaleResp := addSavedItemRequest(t, handler, viewerID, "for_sale", ownForSaleID.String())
	require.Equal(t, http.StatusBadRequest, ownForSaleResp.Code)
	var ownForSaleError savedItemCreatedEnvelope
	require.NoError(t, json.Unmarshal(ownForSaleResp.Body.Bytes(), &ownForSaleError))
	require.False(t, ownForSaleError.Success)
	require.NotNil(t, ownForSaleError.Error)
	require.Equal(t, "OWN_FOR_SALE", ownForSaleError.Error.Code)

	endedAuctionResp := addSavedItemRequest(t, handler, viewerID, "auction", endedAuctionID.String())
	require.Equal(t, http.StatusBadRequest, endedAuctionResp.Code)
	var endedAuctionError savedItemCreatedEnvelope
	require.NoError(t, json.Unmarshal(endedAuctionResp.Body.Bytes(), &endedAuctionError))
	require.False(t, endedAuctionError.Success)
	require.NotNil(t, endedAuctionError.Error)
	require.Equal(t, "AUCTION_ENDED", endedAuctionError.Error.Code)

	repo := savedItemRepo.NewSavedItemRepository(db.NewFromPool(tdb.Pool()))
	count, err := repo.Count(ctx, viewerID)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}
