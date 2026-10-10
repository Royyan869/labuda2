//go:build integration

package serverboot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	forsaleRepo "github.com/hishumi/backend/internal/commerce/forsale/infrastructure/repository"
	orderRepo "github.com/hishumi/backend/internal/commerce/order/infrastructure/repository"
	shippingQuoteApp "github.com/hishumi/backend/internal/commerce/shipping/quote/application"
	shippingQuoteHTTP "github.com/hishumi/backend/internal/commerce/shipping/quote/delivery/http"
	shippingQuoteEntity "github.com/hishumi/backend/internal/commerce/shipping/quote/entity"
	shippingQuoteRepo "github.com/hishumi/backend/internal/commerce/shipping/quote/infrastructure/repository"
	chatEntity "github.com/hishumi/backend/internal/interaction/chat/entity"
	chatInfraRepo "github.com/hishumi/backend/internal/interaction/chat/infrastructure/repository"
	"github.com/hishumi/backend/pkg/db"
	"github.com/hishumi/backend/pkg/money"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// LAYER 1B.2 — F-02 SHIPPING QUOTE CREATE END-TO-END.
//
// This is the real authenticated HTTP path with the REAL shipping quote
// service AND the REAL chat service wired together. It exists specifically to
// prove the transaction-boundary defect is fixed: CreateShippingQuote holds a
// FOR UPDATE lock on the chat room and the shipping_quote conversation message
// must be persisted INSIDE that same transaction. A nested independent
// transaction would self-block on the chat_messages → chat_rooms FK lock and
// hang forever.
type shippingQuoteCreateFixture struct {
	*chatProjectionHTTPFixture
	quoteService *shippingQuoteApp.Service
	handler      *shippingQuoteHTTP.Handler
	quoteRepo    *shippingQuoteRepo.ShippingQuoteRepositoryImpl
}

func newShippingQuoteCreateFixture(t *testing.T) *shippingQuoteCreateFixture {
	t.Helper()

	base := newChatProjectionHTTPFixture(t)
	chatRepository := chatInfraRepo.NewChatRepository()
	roomGetter := &shippingQuoteRoomGetterAdapter{repo: chatRepository}
	quoteRepo := shippingQuoteRepo.NewShippingQuoteRepository()

	quoteService := shippingQuoteApp.NewService(
		base.traced,
		quoteRepo,
		roomGetter,
		forsaleRepo.NewForSaleRepository(),
		nil,          // auction repo (for_sale path only)
		base.service, // REAL chat service — this is the blocker under test
		orderRepo.NewOrderRepository(),
		zap.NewNop(),
	)
	handler := shippingQuoteHTTP.NewHandler(quoteService, roomGetter, base.traced, zap.NewNop())

	return &shippingQuoteCreateFixture{
		chatProjectionHTTPFixture: base,
		quoteService:              quoteService,
		handler:                   handler,
		quoteRepo:                 quoteRepo,
	}
}

func (f *shippingQuoteCreateFixture) routerAs(userID uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Next()
	})
	router.POST("/api/v1/chat/:chat_id/shipping-quote", f.handler.CreateShippingQuote)
	return router
}

// seedActiveForSale inserts an active product + for_sale surface and returns
// both ids (the quote's ProductID contract is the PHYSICAL products.id).
func (f *shippingQuoteCreateFixture) seedActiveForSale(t *testing.T, sellerID uuid.UUID, title string) (productID, saleID uuid.UUID) {
	t.Helper()

	productID = uuid.New()
	now := time.Now().UTC()
	_, err := f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO products (
			id, seller_id, title, description, media_urls, variety,
			preparation_time, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, '[]'::jsonb, 'Kohaku', '1_3_days', $5, $5)
	`, productID, sellerID, title+" product", title+" product", now)
	require.NoError(t, err)

	saleID = uuid.New()
	_, err = f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO for_sales (
			id, product_id, seller_id, price_per_unit, negotiation_enabled,
			status, published_at, sold_at, withdrawn_at,
			quantity_available, created_at, updated_at
		)
		VALUES ($1, $2, $3, 1500000, true, 'active', $4, NULL, NULL, 1, $5, $5)
	`, saleID, productID, sellerID, now, now)
	require.NoError(t, err)

	return productID, saleID
}

// postCreateBounded issues the create request with a bounded context so a
// regression (the self-block) fails fast instead of hanging the suite.
func (f *shippingQuoteCreateFixture) postCreateBounded(t *testing.T, router *gin.Engine, path string, body any) (int, map[string]any) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	reqBody, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(reqBody)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Body.Len() == 0 {
		return w.Code, nil
	}
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &decoded))
	return w.Code, decoded
}

// TestShippingQuoteCreate_HTTP_PersistsQuoteAndMessage proves the real seller
// create path: authenticated seller POST → HTTP success → quote row committed
// AND shipping_quote conversation message committed in the correct room, for
// the correct buyer/product — with no hang.
func TestShippingQuoteCreate_HTTP_PersistsQuoteAndMessage(t *testing.T) {
	f := newShippingQuoteCreateFixture(t)
	ctx := context.Background()

	seller := f.seedActiveSeller(t, uniqueUsername("sq-create-seller"), "Toko SQ Create")
	buyer := f.seedUser(t, "active", nil, uniqueUsername("sq-create-buyer"), nil, nil, nil)
	room := f.seedRoom(t, buyer, seller)
	productID, saleID := f.seedActiveForSale(t, seller, "sq create")

	path := "/api/v1/chat/" + room.String() + "/shipping-quote"
	status, resp := f.postCreateBounded(t, f.routerAs(seller), path, map[string]any{
		"product_id":          productID.String(),
		"source_type":         "for_sale",
		"source_id":           saleID.String(),
		"cost":                25000,
		"destination_city_id": "3171",
		"note":                "ongkir manual",
	})
	require.Equal(t, http.StatusOK, status, "response: %v", resp)

	data, ok := resp["data"].(map[string]any)
	require.True(t, ok, "response data missing: %v", resp)
	quoteIDStr, ok := data["id"].(string)
	require.True(t, ok)
	quoteID, err := uuid.Parse(quoteIDStr)
	require.NoError(t, err)

	// Quote persisted with the right identity.
	var (
		status1, srcType                                 string
		chatID, sellerID, buyerID, productIDDB, sourceID uuid.UUID
		cost                                             int64
	)
	require.NoError(t, f.appDB.Pool().QueryRow(ctx, `
		SELECT status, chat_id, seller_id, buyer_id, product_id, source_type, source_id, cost
		FROM shipping_quotes
		WHERE id = $1
	`, quoteID).Scan(&status1, &chatID, &sellerID, &buyerID, &productIDDB, &srcType, &sourceID, &cost))
	require.Equal(t, string(shippingQuoteEntity.QuoteStatusActive), status1)
	require.Equal(t, room, chatID)
	require.Equal(t, seller, sellerID)
	require.Equal(t, buyer, buyerID)
	require.Equal(t, productID, productIDDB)
	require.Equal(t, "for_sale", srcType)
	require.Equal(t, saleID, sourceID)
	require.Equal(t, int64(25000), cost)

	// Conversation message persisted in the same room, seller-authored, carrying
	// the real quote id.
	var messageCount int
	require.NoError(t, f.appDB.Pool().QueryRow(ctx, `
		SELECT count(*)
		FROM chat_messages
		WHERE room_id = $1
		  AND sender_id = $2
		  AND message_type = 'shipping_quote'
		  AND attachment_json -> 'data' ->> 'offer_id' = $3
	`, room, seller, quoteID.String()).Scan(&messageCount))
	require.Equal(t, 1, messageCount, "exactly one shipping_quote message must be committed")

	// AUTHORIZATION: a chat participant who is NOT the sale-surface seller
	// cannot fabricate a seller quote. The buyer is a room participant (so the
	// handler participant gate passes) but the Commerce service rejects it
	// because the product does not belong to the requesting user.
	status, forgeResp := f.postCreateBounded(t, f.routerAs(buyer), path, map[string]any{
		"product_id":          productID.String(),
		"source_type":         "for_sale",
		"source_id":           saleID.String(),
		"cost":                25000,
		"destination_city_id": "3171",
	})
	require.NotEqual(t, http.StatusOK, status, "buyer must not be able to create a seller quote: %v", forgeResp)

	var totalQuotes int
	require.NoError(t, f.appDB.Pool().QueryRow(ctx,
		`SELECT count(*) FROM shipping_quotes WHERE chat_id = $1`, room).Scan(&totalQuotes))
	require.Equal(t, 1, totalQuotes, "a rejected create must persist no quote")
}

// failingQuoteChatSender is a ChatMessageSender that fails AFTER the quote has
// been written in the same transaction. It proves the quote write is rolled
// back when the conversation message cannot be persisted — the atomicity half
// of the transaction boundary contract.
type failingQuoteChatSender struct{}

func (failingQuoteChatSender) SendMessageInTx(
	context.Context,
	db.Tx,
	uuid.UUID,
	uuid.UUID,
	chatEntity.MessageType,
	*string,
	map[string]interface{},
	string,
) (*chatEntity.ChatMessage, error) {
	return nil, errors.New("forced message persistence failure")
}

// TestShippingQuoteCreate_MessageFailureRollsBackQuote proves quote + message
// atomicity on the failure path using the REAL database transaction.
func TestShippingQuoteCreate_MessageFailureRollsBackQuote(t *testing.T) {
	f := newShippingQuoteCreateFixture(t)
	ctx := context.Background()

	seller := f.seedActiveSeller(t, uniqueUsername("sq-atomicity-seller"), "Toko SQ Atomicity")
	buyer := f.seedUser(t, "active", nil, uniqueUsername("sq-atomicity-buyer"), nil, nil, nil)
	room := f.seedRoom(t, buyer, seller)
	productID, saleID := f.seedActiveForSale(t, seller, "sq atomicity")

	// Wire the same real service, but with a failing message sender.
	chatRepository := chatInfraRepo.NewChatRepository()
	roomGetter := &shippingQuoteRoomGetterAdapter{repo: chatRepository}
	svc := shippingQuoteApp.NewService(
		f.traced,
		f.quoteRepo,
		roomGetter,
		forsaleRepo.NewForSaleRepository(),
		nil,
		failingQuoteChatSender{},
		orderRepo.NewOrderRepository(),
		zap.NewNop(),
	)

	_, err := svc.CreateShippingQuote(ctx, shippingQuoteApp.CreateShippingQuoteInput{
		ChatID:            room,
		ProductID:         productID,
		SourceType:        "for_sale",
		SourceID:          saleID,
		SellerID:          seller,
		Cost:              money.New(25000),
		DestinationCityID: strPtr("3171"),
	})
	require.Error(t, err)

	var quoteCount int
	require.NoError(t, f.appDB.Pool().QueryRow(ctx,
		`SELECT count(*) FROM shipping_quotes WHERE chat_id = $1 AND seller_id = $2`,
		room, seller).Scan(&quoteCount))
	require.Equal(t, 0, quoteCount, "quote must not remain when the message fails")
}

// TestShippingQuoteCreate_SharedTransaction_RollsBackBoth proves that the quote
// row and the shipping_quote message are committed by ONE transaction: when the
// transaction is forced to fail after both writes, NEITHER row survives. This
// is the direct evidence that there is no nested independent message
// transaction.
func TestShippingQuoteCreate_SharedTransaction_RollsBackBoth(t *testing.T) {
	f := newShippingQuoteCreateFixture(t)
	ctx := context.Background()

	seller := f.seedActiveSeller(t, uniqueUsername("sq-sharedtx-seller"), "Toko SQ SharedTx")
	buyer := f.seedUser(t, "active", nil, uniqueUsername("sq-sharedtx-buyer"), nil, nil, nil)
	room := f.seedRoom(t, buyer, seller)
	productID, saleID := f.seedActiveForSale(t, seller, "sq shared tx")

	quote := shippingQuoteEntity.NewShippingQuote(
		room,
		productID,
		"for_sale",
		saleID,
		seller,
		buyer,
		money.New(25000),
		nil,
		strPtr("3171"),
		nil,
		time.Now().Add(24*time.Hour),
	)
	attachment := map[string]interface{}{
		"type": "shipping_quote",
		"data": map[string]interface{}{"offer_id": quote.ID.String()},
	}

	txErr := f.traced.WithTx(ctx, func(tx db.Tx) error {
		if err := f.quoteRepo.Create(ctx, tx, quote); err != nil {
			return err
		}
		if _, err := f.service.SendMessageInTx(ctx, tx, room, seller, chatEntity.MessageTypeShippingQuote, nil, attachment, "shared-tx."+quote.ID.String()); err != nil {
			return err
		}
		return errors.New("forced rollback after both writes")
	})
	require.Error(t, txErr)

	var (
		quoteCount   int
		messageCount int
	)
	require.NoError(t, f.appDB.Pool().QueryRow(ctx,
		`SELECT count(*) FROM shipping_quotes WHERE id = $1`, quote.ID).Scan(&quoteCount))
	require.NoError(t, f.appDB.Pool().QueryRow(ctx,
		`SELECT count(*) FROM chat_messages WHERE room_id = $1 AND attachment_json -> 'data' ->> 'offer_id' = $2`,
		room, quote.ID.String()).Scan(&messageCount))
	require.Equal(t, 0, quoteCount, "quote must roll back with the shared transaction")
	require.Equal(t, 0, messageCount, "message must roll back with the shared transaction")
}
