package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	shippingQuoteApp "github.com/labuda/backend/internal/commerce/shipping/quote/application"
	shippingQuoteEntity "github.com/labuda/backend/internal/commerce/shipping/quote/entity"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

// mockRoomGetter returns a predetermined room or error.
type mockRoomGetter struct {
	room *chatEntity.ChatRoom
	err  error
}

func (m *mockRoomGetter) GetRoomByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*chatEntity.ChatRoom, error) {
	return m.room, m.err
}

func (m *mockRoomGetter) GetRoomByIDForUpdate(_ context.Context, _ db.Tx, _ uuid.UUID) (*chatEntity.ChatRoom, error) {
	return m.room, m.err
}

// mockTransactor executes the function immediately with a nil tx.
type mockTransactor struct{}

func (m *mockTransactor) WithTx(_ context.Context, fn func(db.Tx) error) error {
	return fn(nil)
}

// mockQuoteRepo returns a predetermined quote for GetByID.
type mockQuoteRepo struct {
	quote *shippingQuoteEntity.ShippingQuote
	err   error
}

func (m *mockQuoteRepo) Create(context.Context, db.Tx, *shippingQuoteEntity.ShippingQuote) error {
	return nil
}
func (m *mockQuoteRepo) GetLatestByChatAndSource(context.Context, db.Tx, uuid.UUID, uuid.UUID, string, uuid.UUID, uuid.UUID, uuid.UUID) (*shippingQuoteEntity.ShippingQuote, error) {
	return m.quote, m.err
}
func (m *mockQuoteRepo) GetLatestRevisionByChatAndSource(context.Context, db.Tx, uuid.UUID, uuid.UUID, string, uuid.UUID, uuid.UUID, uuid.UUID) (*shippingQuoteEntity.ShippingQuote, error) {
	return m.quote, m.err
}
func (m *mockQuoteRepo) GetCurrentActiveByChatAndSource(context.Context, db.Tx, uuid.UUID, uuid.UUID, string, uuid.UUID, uuid.UUID, uuid.UUID) (*shippingQuoteEntity.ShippingQuote, error) {
	return m.quote, m.err
}
func (m *mockQuoteRepo) GetByID(context.Context, db.Tx, uuid.UUID) (*shippingQuoteEntity.ShippingQuote, error) {
	return m.quote, m.err
}
func (m *mockQuoteRepo) GetByIDForUpdate(context.Context, db.Tx, uuid.UUID) (*shippingQuoteEntity.ShippingQuote, error) {
	return m.quote, m.err
}
func (m *mockQuoteRepo) UpdateStatus(context.Context, db.Tx, uuid.UUID, shippingQuoteEntity.QuoteStatus, *interface{}) error {
	return nil
}
func (m *mockQuoteRepo) ReactivateQuote(context.Context, db.Tx, uuid.UUID) error { return nil }
func (m *mockQuoteRepo) SupersedeCurrentQuotes(context.Context, db.Tx, uuid.UUID, uuid.UUID, string, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) (int64, error) {
	return 0, nil
}
func (m *mockQuoteRepo) InvalidateQuotesByProduct(context.Context, db.Tx, uuid.UUID) error {
	return nil
}

// newTestService creates a minimal service wired with mocks for read-path testing.
func newTestService(quoteRepo *mockQuoteRepo) *shippingQuoteApp.Service {
	return shippingQuoteApp.NewService(
		&mockTransactor{},
		quoteRepo,
		&mockRoomGetter{}, // service-level room getter (unused in read path)
		nil,               // forSaleRepo (unused in read path)
		nil,               // auctionRepo (unused in read path)
		nil,               // chatService (unused in read path)
		nil,               // orderRepo (unused in read path)
		zap.NewNop(),
	)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func makeRoom(participantA, participantB uuid.UUID) *chatEntity.ChatRoom {
	return chatEntity.NewChatRoom(chatEntity.RoomTypeDirect, participantA, participantB)
}

func makeQuote(sellerID, buyerID uuid.UUID) *shippingQuoteEntity.ShippingQuote {
	return &shippingQuoteEntity.ShippingQuote{
		ID:         uuid.New(),
		ChatID:     uuid.New(),
		ProductID:  uuid.New(),
		SourceType: ptrString("for_sale"),
		SourceID:   ptrUUID(uuid.New()),
		SellerID:   sellerID,
		BuyerID:    buyerID,
		Cost:       money.New(10000),
		Status:     shippingQuoteEntity.QuoteStatusActive,
		CreatedAt:  time.Now(),
	}
}

func ptrString(v string) *string     { return &v }
func ptrUUID(v uuid.UUID) *uuid.UUID { return &v }

// ---------------------------------------------------------------------------
// Tests: Chat-scoped routes — participant gate
// ---------------------------------------------------------------------------

func TestB100_CreateQuote_NonParticipant_Forbidden(t *testing.T) {
	seller := uuid.New()
	buyer := uuid.New()
	outsider := uuid.New()
	chatID := uuid.New()

	room := makeRoom(seller, buyer)
	room.ID = chatID

	handler := NewHandler(nil, &mockRoomGetter{room: room}, nil, zap.NewNop())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/chat/"+chatID.String()+"/shipping-quote", nil)
	c.Set("userID", outsider)
	c.Params = gin.Params{{Key: "chat_id", Value: chatID.String()}}

	handler.CreateShippingQuote(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "not a participant")
}

// ---------------------------------------------------------------------------
// Tests: GetShippingQuoteByID — ownership gate
// ---------------------------------------------------------------------------

func TestB100_GetQuoteByID_NonOwner_Forbidden(t *testing.T) {
	seller := uuid.New()
	buyer := uuid.New()
	outsider := uuid.New()
	quote := makeQuote(seller, buyer)

	quoteRepo := &mockQuoteRepo{quote: quote}
	svc := newTestService(quoteRepo)
	handler := NewHandler(svc, &mockRoomGetter{}, nil, zap.NewNop())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/shipping-quote/"+quote.ID.String(), nil)
	c.Set("userID", outsider)
	c.Params = gin.Params{{Key: "quote_id", Value: quote.ID.String()}}

	handler.GetShippingQuoteByID(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "not authorized")
}

func TestB100_GetQuoteByID_Seller_OK(t *testing.T) {
	seller := uuid.New()
	buyer := uuid.New()
	quote := makeQuote(seller, buyer)

	quoteRepo := &mockQuoteRepo{quote: quote}
	svc := newTestService(quoteRepo)
	handler := NewHandler(svc, &mockRoomGetter{}, nil, zap.NewNop())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/shipping-quote/"+quote.ID.String(), nil)
	c.Set("userID", seller)
	c.Params = gin.Params{{Key: "quote_id", Value: quote.ID.String()}}

	handler.GetShippingQuoteByID(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestB100_GetQuoteByID_Buyer_OK(t *testing.T) {
	seller := uuid.New()
	buyer := uuid.New()
	quote := makeQuote(seller, buyer)

	quoteRepo := &mockQuoteRepo{quote: quote}
	svc := newTestService(quoteRepo)
	handler := NewHandler(svc, &mockRoomGetter{}, nil, zap.NewNop())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/shipping-quote/"+quote.ID.String(), nil)
	c.Set("userID", buyer)
	c.Params = gin.Params{{Key: "quote_id", Value: quote.ID.String()}}

	handler.GetShippingQuoteByID(c)

	assert.Equal(t, http.StatusOK, w.Code)
}
