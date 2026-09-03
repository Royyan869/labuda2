package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	forsaleEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	shippingQuoteEntity "github.com/labuda/backend/internal/commerce/shipping/quote/entity"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type auctionQuoteTx struct{}

func (m *auctionQuoteTx) WithTx(_ context.Context, fn func(db.Tx) error) error {
	return fn(nil)
}

type auctionQuoteRoomGetter struct {
	room *chatEntity.ChatRoom
	err  error
}

func (m *auctionQuoteRoomGetter) GetRoomByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*chatEntity.ChatRoom, error) {
	return m.room, m.err
}

func (m *auctionQuoteRoomGetter) GetRoomByIDForUpdate(_ context.Context, _ db.Tx, _ uuid.UUID) (*chatEntity.ChatRoom, error) {
	return m.room, m.err
}

type auctionQuoteSender struct {
	called bool
}

func (m *auctionQuoteSender) SendMessage(
	_ context.Context,
	_, _ uuid.UUID,
	_ chatEntity.MessageType,
	_ *string,
	_ map[string]interface{},
	_ string,
) (*chatEntity.ChatMessage, error) {
	m.called = true
	return &chatEntity.ChatMessage{ID: uuid.New()}, nil
}

type auctionQuoteRepoStub struct {
	auction *auctionEntity.Auction
	err     error
	called  bool
}

func (m *auctionQuoteRepoStub) GetByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*auctionEntity.Auction, error) {
	m.called = true
	return m.auction, m.err
}

func (m *auctionQuoteRepoStub) MarkSellerQuoteProvided(_ context.Context, _ db.Tx, _ uuid.UUID) error {
	return nil
}

type forSaleQuoteRepoStub struct {
	called   bool
	sellerID uuid.UUID
}

func (m *forSaleQuoteRepoStub) GetByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*forsaleEntity.ForSale, error) {
	m.called = true
	return &forsaleEntity.ForSale{
		ID:       uuid.New(),
		SellerID: m.sellerID,
		Status:   forsaleEntity.ForSaleStatusActive,
	}, nil
}

type shippingQuoteRepoStub struct {
	supersedeCalled bool
	createCalled    bool
}

func (m *shippingQuoteRepoStub) Create(_ context.Context, _ db.Tx, _ *shippingQuoteEntity.ShippingQuote) error {
	m.createCalled = true
	return nil
}

func (m *shippingQuoteRepoStub) GetLatestByChatAndSource(context.Context, db.Tx, uuid.UUID, uuid.UUID, string, uuid.UUID, uuid.UUID, uuid.UUID) (*shippingQuoteEntity.ShippingQuote, error) {
	return nil, nil
}

func (m *shippingQuoteRepoStub) GetLatestRevisionByChatAndSource(context.Context, db.Tx, uuid.UUID, uuid.UUID, string, uuid.UUID, uuid.UUID, uuid.UUID) (*shippingQuoteEntity.ShippingQuote, error) {
	return nil, nil
}

func (m *shippingQuoteRepoStub) GetByID(context.Context, db.Tx, uuid.UUID) (*shippingQuoteEntity.ShippingQuote, error) {
	return nil, nil
}

func (m *shippingQuoteRepoStub) GetByIDForUpdate(context.Context, db.Tx, uuid.UUID) (*shippingQuoteEntity.ShippingQuote, error) {
	return nil, nil
}

func (m *shippingQuoteRepoStub) UpdateStatus(context.Context, db.Tx, uuid.UUID, shippingQuoteEntity.QuoteStatus, *interface{}) error {
	return nil
}

func (m *shippingQuoteRepoStub) ReactivateQuote(context.Context, db.Tx, uuid.UUID) error {
	return nil
}

func (m *shippingQuoteRepoStub) GetCurrentActiveByChatAndSource(context.Context, db.Tx, uuid.UUID, uuid.UUID, string, uuid.UUID, uuid.UUID, uuid.UUID) (*shippingQuoteEntity.ShippingQuote, error) {
	return nil, nil
}

func (m *shippingQuoteRepoStub) SupersedeCurrentQuotes(_ context.Context, _ db.Tx, _ uuid.UUID, _ uuid.UUID, _ string, _ uuid.UUID, _ uuid.UUID, _ uuid.UUID, _ uuid.UUID) (int64, error) {
	m.supersedeCalled = true
	return 0, nil
}

func (m *shippingQuoteRepoStub) InvalidateQuotesByProduct(context.Context, db.Tx, uuid.UUID) error {
	return nil
}

func newAuctionQuoteService(
	quoteRepo *shippingQuoteRepoStub,
	room *chatEntity.ChatRoom,
	forSaleRepo ForSaleRepository,
	auctionRepo AuctionQuoteReader,
	sender *auctionQuoteSender,
) *Service {
	if sender == nil {
		sender = &auctionQuoteSender{}
	}
	return &Service{
		db:          &auctionQuoteTx{},
		quoteRepo:   quoteRepo,
		roomGetter:  &auctionQuoteRoomGetter{room: room},
		forSaleRepo: forSaleRepo,
		auctionRepo: auctionRepo,
		chatService: sender,
		log:         zap.NewNop(),
	}
}

func newAuctionChatRoom(sellerID, winnerID uuid.UUID) *chatEntity.ChatRoom {
	room := chatEntity.NewChatRoom(chatEntity.RoomTypeDirect, sellerID, winnerID)
	room.ID = uuid.New()
	return room
}

func newWaitingSettlementAuction(sellerID, winnerID uuid.UUID) *auctionEntity.Auction {
	productID := uuid.New()
	return &auctionEntity.Auction{
		ID:              uuid.New(),
		SellerID:        sellerID,
		ProductID:       productID,
		Status:          auctionEntity.StatusWaitingSettlement,
		CurrentWinnerID: &winnerID,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
}

func TestCreateShippingQuote_AuctionWaitingSettlement_WinnerAllowed(t *testing.T) {
	sellerID := uuid.New()
	winnerID := uuid.New()
	chatRoom := newAuctionChatRoom(sellerID, winnerID)
	auction := newWaitingSettlementAuction(sellerID, winnerID)

	auctionRepo := &auctionQuoteRepoStub{auction: auction}
	quoteRepo := &shippingQuoteRepoStub{}
	sender := &auctionQuoteSender{}
	forSaleRepo := &forSaleQuoteRepoStub{sellerID: sellerID}

	svc := newAuctionQuoteService(quoteRepo, chatRoom, forSaleRepo, auctionRepo, sender)

	quote, err := svc.CreateShippingQuote(context.Background(), CreateShippingQuoteInput{
		ChatID:         chatRoom.ID,
		ProductID:      auction.ProductID,
		SourceType:     "auction",
		SourceID:       auction.ID,
		SellerID:       sellerID,
		Cost:           money.New(15000),
		ExpiresInHours: nil,
	})

	require.NoError(t, err)
	require.NotNil(t, quote)
	require.True(t, auctionRepo.called)
	require.True(t, quoteRepo.supersedeCalled)
	require.True(t, quoteRepo.createCalled)
	require.True(t, sender.called)
	require.False(t, forSaleRepo.called)
}

func TestCreateShippingQuote_AuctionMissingRejected(t *testing.T) {
	sellerID := uuid.New()
	winnerID := uuid.New()
	chatRoom := newAuctionChatRoom(sellerID, winnerID)
	auctionID := uuid.New()

	auctionRepo := &auctionQuoteRepoStub{auction: nil}
	quoteRepo := &shippingQuoteRepoStub{}
	sender := &auctionQuoteSender{}
	forSaleRepo := &forSaleQuoteRepoStub{sellerID: sellerID}

	svc := newAuctionQuoteService(quoteRepo, chatRoom, forSaleRepo, auctionRepo, sender)

	quote, err := svc.CreateShippingQuote(context.Background(), CreateShippingQuoteInput{
		ChatID:         chatRoom.ID,
		ProductID:      uuid.New(),
		SourceType:     "auction",
		SourceID:       auctionID,
		SellerID:       sellerID,
		Cost:           money.New(15000),
		ExpiresInHours: nil,
	})

	require.Error(t, err)
	require.Nil(t, quote)
	require.Contains(t, err.Error(), "auction not found")
	require.True(t, auctionRepo.called)
	require.False(t, quoteRepo.supersedeCalled)
	require.False(t, quoteRepo.createCalled)
	require.False(t, sender.called)
	require.False(t, forSaleRepo.called)
}

func TestCreateShippingQuote_AuctionWrongSellerRejected(t *testing.T) {
	sellerID := uuid.New()
	winnerID := uuid.New()
	chatRoom := newAuctionChatRoom(sellerID, winnerID)
	auction := newWaitingSettlementAuction(uuid.New(), winnerID)

	auctionRepo := &auctionQuoteRepoStub{auction: auction}
	quoteRepo := &shippingQuoteRepoStub{}
	sender := &auctionQuoteSender{}
	forSaleRepo := &forSaleQuoteRepoStub{sellerID: sellerID}

	svc := newAuctionQuoteService(quoteRepo, chatRoom, forSaleRepo, auctionRepo, sender)

	quote, err := svc.CreateShippingQuote(context.Background(), CreateShippingQuoteInput{
		ChatID:         chatRoom.ID,
		ProductID:      auction.ProductID,
		SourceType:     "auction",
		SourceID:       auction.ID,
		SellerID:       sellerID,
		Cost:           money.New(15000),
		ExpiresInHours: nil,
	})

	require.Error(t, err)
	require.Nil(t, quote)
	require.Contains(t, err.Error(), "does not belong to seller")
	require.True(t, auctionRepo.called)
	require.False(t, quoteRepo.supersedeCalled)
	require.False(t, quoteRepo.createCalled)
	require.False(t, sender.called)
	require.False(t, forSaleRepo.called)
}

func TestCreateShippingQuote_AuctionInvalidStatusesRejected(t *testing.T) {
	sellerID := uuid.New()
	winnerID := uuid.New()
	chatRoom := newAuctionChatRoom(sellerID, winnerID)

	statuses := []auctionEntity.Status{
		auctionEntity.StatusDraft,
		auctionEntity.StatusScheduled,
		auctionEntity.StatusActive,
		auctionEntity.StatusEnded,
		auctionEntity.StatusCancelled,
	}

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			auction := &auctionEntity.Auction{
				ID:              uuid.New(),
				SellerID:        sellerID,
				Status:          status,
				CurrentWinnerID: &winnerID,
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			}

			auctionRepo := &auctionQuoteRepoStub{auction: auction}
			quoteRepo := &shippingQuoteRepoStub{}
			sender := &auctionQuoteSender{}
			forSaleRepo := &forSaleQuoteRepoStub{sellerID: sellerID}

			svc := newAuctionQuoteService(quoteRepo, chatRoom, forSaleRepo, auctionRepo, sender)

			quote, err := svc.CreateShippingQuote(context.Background(), CreateShippingQuoteInput{
				ChatID:         chatRoom.ID,
				ProductID:      auction.ProductID,
				SourceType:     "auction",
				SourceID:       auction.ID,
				SellerID:       sellerID,
				Cost:           money.New(15000),
				ExpiresInHours: nil,
			})

			require.Error(t, err)
			require.Nil(t, quote)
			require.Contains(t, err.Error(), "waiting_settlement")
			require.True(t, auctionRepo.called)
			require.False(t, quoteRepo.supersedeCalled)
			require.False(t, quoteRepo.createCalled)
			require.False(t, sender.called)
			require.False(t, forSaleRepo.called)
		})
	}
}

func TestCreateShippingQuote_AuctionRecipientMismatchRejected(t *testing.T) {
	sellerID := uuid.New()
	winnerID := uuid.New()
	otherParticipant := uuid.New()
	chatRoom := newAuctionChatRoom(sellerID, otherParticipant)
	auction := newWaitingSettlementAuction(sellerID, winnerID)

	auctionRepo := &auctionQuoteRepoStub{auction: auction}
	quoteRepo := &shippingQuoteRepoStub{}
	sender := &auctionQuoteSender{}
	forSaleRepo := &forSaleQuoteRepoStub{sellerID: sellerID}

	svc := newAuctionQuoteService(quoteRepo, chatRoom, forSaleRepo, auctionRepo, sender)

	quote, err := svc.CreateShippingQuote(context.Background(), CreateShippingQuoteInput{
		ChatID:         chatRoom.ID,
		ProductID:      auction.ProductID,
		SourceType:     "auction",
		SourceID:       auction.ID,
		SellerID:       sellerID,
		Cost:           money.New(15000),
		ExpiresInHours: nil,
	})

	require.Error(t, err)
	require.Nil(t, quote)
	require.Contains(t, err.Error(), "auction winner")
	require.True(t, auctionRepo.called)
	require.False(t, quoteRepo.supersedeCalled)
	require.False(t, quoteRepo.createCalled)
	require.False(t, sender.called)
	require.False(t, forSaleRepo.called)
}

func TestCreateShippingQuote_ForSalePathUnaffected(t *testing.T) {
	sellerID := uuid.New()
	buyerID := uuid.New()
	chatRoom := newAuctionChatRoom(sellerID, buyerID)
	forSaleID := uuid.New()

	auctionRepo := &auctionQuoteRepoStub{}
	quoteRepo := &shippingQuoteRepoStub{}
	sender := &auctionQuoteSender{}
	forSaleRepo := &forSaleQuoteRepoStub{sellerID: sellerID}

	svc := newAuctionQuoteService(quoteRepo, chatRoom, forSaleRepo, auctionRepo, sender)

	quote, err := svc.CreateShippingQuote(context.Background(), CreateShippingQuoteInput{
		ChatID:         chatRoom.ID,
		ProductID:      forSaleID,
		SourceType:     "for_sale",
		SourceID:       forSaleID,
		SellerID:       sellerID,
		Cost:           money.New(15000),
		ExpiresInHours: nil,
	})

	require.NoError(t, err)
	require.NotNil(t, quote)
	require.False(t, auctionRepo.called)
	require.True(t, forSaleRepo.called)
	require.True(t, quoteRepo.supersedeCalled)
	require.True(t, quoteRepo.createCalled)
	require.True(t, sender.called)
}
