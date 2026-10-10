//go:build integration

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/hishumi/backend/internal/audit"
	auctionApp "github.com/hishumi/backend/internal/commerce/auction/application"
	auctionHTTP "github.com/hishumi/backend/internal/commerce/auction/delivery/http"
	auctionEntity "github.com/hishumi/backend/internal/commerce/auction/entity"
	"github.com/hishumi/backend/internal/identity/auth"
	biddingApp "github.com/hishumi/backend/internal/interaction/bidding/application"
	biddingHTTP "github.com/hishumi/backend/internal/interaction/bidding/delivery/http"
	"github.com/hishumi/backend/pkg/db"
	"github.com/hishumi/backend/pkg/testdb"
)

type auctionBidRuntimeAuditLogger struct{}

var _ audit.AdminAuditLogger = auctionBidRuntimeAuditLogger{}

func (auctionBidRuntimeAuditLogger) Log(context.Context, uuid.UUID, string, string, uuid.UUID, map[string]interface{}) error {
	return nil
}

func (auctionBidRuntimeAuditLogger) LogSafe(context.Context, uuid.UUID, string, string, uuid.UUID, map[string]interface{}) {
}

func (auctionBidRuntimeAuditLogger) LogTx(context.Context, db.Tx, uuid.UUID, string, string, uuid.UUID, map[string]interface{}) error {
	return nil
}

func TestAuctionBidHistorySameUserRuntimeFixture(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())

	sellerID := seedStage6BUser(t, ctx, tdb)
	buyerID := seedStage6BUser(t, ctx, tdb)
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		INSERT INTO seller_profiles (id, user_id, store_name, tier, status, created_at, updated_at)
		VALUES ($1, $2, 'Runtime Fixture Store', 'basic', 'active', NOW(), NOW())
		RETURNING user_id
	`, uuid.New(), sellerID).Scan(new(uuid.UUID)))

	productID := seedStage6BProduct(t, ctx, tdb, sellerID)
	shippingID := stage5Shipping(t, ctx, tdb, sellerID, productID)
	auctionID := stage5Auction(t, ctx, tdb, productID, sellerID)

	var status string
	var storedSeller, storedProduct uuid.UUID
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT status, seller_id, product_id FROM auctions WHERE id = $1
	`, auctionID).Scan(&status, &storedSeller, &storedProduct))
	require.Equal(t, string(auctionEntity.StatusActive), status)
	require.Equal(t, sellerID, storedSeller)
	require.Equal(t, productID, storedProduct)
	var linkedShipping uuid.UUID
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT shipping_option_id FROM product_shipping_options WHERE product_id = $1
	`, productID).Scan(&linkedShipping))
	require.Equal(t, shippingID, linkedShipping)

	roleChecker := auth.NewRoleCheckerDB(appDB, auctionBidRuntimeAuditLogger{})
	auctionService := auctionApp.NewAuctionService(
		auth.NewAccountStatusCheckerDB(appDB), nil, nil, nil, nil, nil, nil, roleChecker, nil, zap.NewNop(),
	)
	auctionHandler := auctionHTTP.NewAuctionHandler(auctionService, nil, nil, appDB, zap.NewNop(), nil)
	biddingHandler := biddingHTTP.NewBiddingHandler(biddingApp.NewBiddingService(), appDB, zap.NewNop())

	route := func(method, path string, userID uuid.UUID, payload interface{}, handler gin.HandlerFunc) *httptest.ResponseRecorder {
		var body *bytes.Reader
		if payload == nil {
			body = bytes.NewReader(nil)
		} else {
			encoded, err := json.Marshal(payload)
			require.NoError(t, err)
			body = bytes.NewReader(encoded)
		}
		request := httptest.NewRequest(method, path, body)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(response)
		context.Request = request
		context.Params = gin.Params{{Key: "id", Value: auctionID.String()}}
		context.Set("userID", userID)
		handler(context)
		return response
	}

	bidResponse := route(http.MethodPost, "/api/v1/auctions/"+auctionID.String()+"/bid", buyerID, map[string]interface{}{
		"amount":          int64(410_000),
		"idempotency_key": "runtime-fixture-" + uuid.NewString(),
	}, auctionHandler.PlaceBid)
	require.Equal(t, http.StatusCreated, bidResponse.Code, bidResponse.Body.String())

	var bidID, persistedAuction, persistedBidder uuid.UUID
	var amount int64
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT id, auction_id, bidder_id, amount FROM auction_bids WHERE auction_id = $1
	`, auctionID).Scan(&bidID, &persistedAuction, &persistedBidder, &amount))
	require.Equal(t, auctionID, persistedAuction)
	require.Equal(t, buyerID, persistedBidder)
	require.Equal(t, int64(410_000), amount)

	myBidsResponse := route(http.MethodGet, "/api/v1/bidding", buyerID, nil, biddingHandler.GetMyBidding)
	require.Equal(t, http.StatusOK, myBidsResponse.Code, myBidsResponse.Body.String())
	var myBids struct {
		Data struct {
			Items []struct {
				AuctionID   uuid.UUID `json:"auction_id"`
				YourLastBid int64     `json:"your_last_bid"`
				CurrentBid  int64     `json:"current_bid"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(myBidsResponse.Body.Bytes(), &myBids))
	require.Len(t, myBids.Data.Items, 1)
	require.Equal(t, auctionID, myBids.Data.Items[0].AuctionID)
	require.Equal(t, amount, myBids.Data.Items[0].YourLastBid)
	require.Equal(t, amount, myBids.Data.Items[0].CurrentBid)

	historyResponse := route(http.MethodGet, "/api/v1/auctions/"+auctionID.String()+"/bids", buyerID, nil, auctionHandler.ListBids)
	require.Equal(t, http.StatusOK, historyResponse.Code, historyResponse.Body.String())
	var history struct {
		Data struct {
			AuctionID uuid.UUID `json:"auction_id"`
			Bids      []struct {
				ID        uuid.UUID `json:"id"`
				AuctionID uuid.UUID `json:"auction_id"`
				BidderID  uuid.UUID `json:"bidder_id"`
				Amount    int64     `json:"amount"`
			} `json:"bids"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(historyResponse.Body.Bytes(), &history))
	require.Equal(t, auctionID, history.Data.AuctionID)
	require.Len(t, history.Data.Bids, 1)
	require.Equal(t, bidID, history.Data.Bids[0].ID)
	require.Equal(t, auctionID, history.Data.Bids[0].AuctionID)
	require.Equal(t, buyerID, history.Data.Bids[0].BidderID)
	require.Equal(t, amount, history.Data.Bids[0].Amount)
}
