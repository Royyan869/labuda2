//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	forsaleEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	forsaleRepo "github.com/labuda/backend/internal/commerce/forsale/infrastructure/repository"
	orderEntity "github.com/labuda/backend/internal/commerce/order/entity"
	quoteApp "github.com/labuda/backend/internal/commerce/shipping/quote/application"
	quoteEntity "github.com/labuda/backend/internal/commerce/shipping/quote/entity"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	chatInfraRepo "github.com/labuda/backend/internal/interaction/chat/infrastructure/repository"
	chatRepo "github.com/labuda/backend/internal/interaction/chat/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type roomGetterAdapter struct {
	repo chatRepo.Repository
}

func (a roomGetterAdapter) GetRoomByID(ctx context.Context, tx db.Tx, roomID uuid.UUID) (*chatEntity.ChatRoom, error) {
	return a.repo.GetRoomByID(ctx, tx, roomID)
}

func (a roomGetterAdapter) GetRoomByIDForUpdate(ctx context.Context, tx db.Tx, roomID uuid.UUID) (*chatEntity.ChatRoom, error) {
	return a.repo.GetRoomByIDForUpdate(ctx, tx, roomID)
}

type blockingGate struct {
	entered     chan struct{}
	release     chan struct{}
	enteredOnce sync.Once
	releaseOnce sync.Once
}

func newBlockingGate() *blockingGate {
	return &blockingGate{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (g *blockingGate) markEntered() {
	g.enteredOnce.Do(func() {
		close(g.entered)
	})
}

func (g *blockingGate) wait(ctx context.Context) error {
	select {
	case <-g.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *blockingGate) releaseNow() {
	g.releaseOnce.Do(func() {
		close(g.release)
	})
}

type blockingChatSender struct {
	gate    *blockingGate
	hit     int32
	blockOn int32
}

func (s *blockingChatSender) EnableBlocking() {
	atomic.StoreInt32(&s.blockOn, 1)
	atomic.StoreInt32(&s.hit, 0)
}

func (s *blockingChatSender) SendMessage(
	ctx context.Context,
	roomID, senderID uuid.UUID,
	messageType chatEntity.MessageType,
	body *string,
	attachmentJSON map[string]interface{},
	idempotencyKey string,
) (*chatEntity.ChatMessage, error) {
	if atomic.LoadInt32(&s.blockOn) == 1 && atomic.CompareAndSwapInt32(&s.hit, 0, 1) {
		s.gate.markEntered()
		if err := s.gate.wait(ctx); err != nil {
			return nil, err
		}
	}

	return &chatEntity.ChatMessage{
		ID:             uuid.New(),
		RoomID:         roomID,
		SenderID:       senderID,
		MessageType:    messageType,
		Body:           body,
		AttachmentJSON: attachmentJSON,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      time.Now(),
	}, nil
}

type blockingOrderRepo struct {
	gate    *blockingGate
	hit     int32
	blockOn int32
}

func (r *blockingOrderRepo) EnableBlocking() {
	atomic.StoreInt32(&r.blockOn, 1)
	atomic.StoreInt32(&r.hit, 0)
}

func (r *blockingOrderRepo) GetByShippingQuoteID(context.Context, db.Tx, uuid.UUID) (*orderEntity.Order, error) {
	return nil, nil
}

func (r *blockingOrderRepo) CountValidOrdersByShippingQuoteID(ctx context.Context, tx db.Tx, shippingQuoteID uuid.UUID) (int64, error) {
	if atomic.LoadInt32(&r.blockOn) == 1 && atomic.CompareAndSwapInt32(&r.hit, 0, 1) {
		r.gate.markEntered()
		if err := r.gate.wait(ctx); err != nil {
			return 0, err
		}
	}
	return 0, nil
}

func waitForSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for blocking gate")
	}
}

func insertTestUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()

	userID := uuid.New()
	email := fmt.Sprintf("%s@test.invalid", userID.String())

	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at)
		VALUES ($1, $2, $3, 'active', NOW(), NOW())
	`, userID, userID.String(), email)
	require.NoError(t, err)

	return userID
}

func setupShippingQuoteIntegration(t *testing.T) (*testdb.TestDB, *quoteApp.Service, *blockingChatSender, *blockingOrderRepo, *forsaleRepo.ForSaleRepositoryImpl, chatRepo.Repository) {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	chatRepository := chatInfraRepo.NewChatRepository()
	quoteRepository := NewShippingQuoteRepository()
	forSaleRepository := forsaleRepo.NewForSaleRepository()
	roomGetter := roomGetterAdapter{repo: chatRepository}

	sender := &blockingChatSender{gate: newBlockingGate()}
	orderRepo := &blockingOrderRepo{gate: newBlockingGate()}

	service := quoteApp.NewService(
		tdb,
		quoteRepository,
		roomGetter,
		forSaleRepository,
		nil,
		sender,
		orderRepo,
		zap.NewNop(),
	)

	return tdb, service, sender, orderRepo, forSaleRepository, chatRepository
}

func seedChatRoom(t *testing.T, ctx context.Context, tdb *testdb.TestDB, repo chatRepo.Repository, roomType chatEntity.RoomType, userA, userB uuid.UUID) *chatEntity.ChatRoom {
	t.Helper()

	room := chatEntity.NewChatRoom(roomType, userA, userB)
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		return repo.CreateRoom(ctx, tx, room)
	}))

	return room
}

func seedPublishedForSale(t *testing.T, ctx context.Context, tdb *testdb.TestDB, repo *forsaleRepo.ForSaleRepositoryImpl, sellerID uuid.UUID, title string) *forsaleEntity.ForSale {
	t.Helper()

	forSale, err := forsaleEntity.NewForSale(
		sellerID,
		title,
		"fixture forSale",
		json.RawMessage(`[]`),
		"kohaku",
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		forsaleEntity.ForSaleTypeFixedPrice,
		money.New(25000),
		1,
		false,
		forsaleEntity.ForSaleVisibilityPrivate,
		forsaleEntity.ForSaleOriginDirectCreate,
		nil,
		forsaleEntity.PreparationTimeImmediate,
		nil,
	)
	require.NoError(t, err)

	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		if err := repo.Create(ctx, tx, forSale); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE for_sales
			SET status = 'active',
			    published_at = NOW(),
			    updated_at = NOW()
			WHERE id = $1
		`, forSale.ID); err != nil {
			return err
		}
		return nil
	}))

	forSale.Status = forsaleEntity.ForSaleStatusActive
	now := time.Now()
	forSale.PublishedAt = &now
	forSale.Visibility = forsaleEntity.ForSaleVisibilityPublic
	forSale.UpdatedAt = now

	return forSale
}

func createQuoteInput(chatID uuid.UUID, forSale *forsaleEntity.ForSale, sellerID, buyerID uuid.UUID, note string, cost int64) quoteApp.CreateShippingQuoteInput {
	return quoteApp.CreateShippingQuoteInput{
		ChatID:         chatID,
		ProductID:      forSale.ID,
		SourceType:     "for_sale",
		SourceID:       forSale.ID,
		SellerID:       sellerID,
		Cost:           money.New(cost),
		Note:           &note,
		ExpiresInHours: nil,
	}
}

func seedUsedQuote(t *testing.T, ctx context.Context, tdb *testdb.TestDB, quoteID uuid.UUID, reactivationCount, maxReuse int) {
	t.Helper()

	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE shipping_quotes
			SET status = 'USED',
			    used_at = NOW(),
			    superseded_at = NULL,
			    superseded_by_id = NULL,
			    reactivation_count = $2,
			    max_reuse = $3
			WHERE id = $1
		`, quoteID, reactivationCount, maxReuse)
		return err
	}))
}

func TestShippingQuote_CreateConcurrentReplacement_SupersedesPriorRevision(t *testing.T) {
	tdb, service, sender, _, forSaleRepo, chatRepository := setupShippingQuoteIntegration(t)

	ctx := context.Background()
	sellerID := insertTestUser(t, ctx, tdb.Pool())
	buyerID := insertTestUser(t, ctx, tdb.Pool())

	directRoom := seedChatRoom(t, ctx, tdb, chatRepository, chatEntity.RoomTypeDirect, sellerID, buyerID)
	forSale := seedPublishedForSale(t, ctx, tdb, forSaleRepo, sellerID, "replacement fixture")

	primaryInput := createQuoteInput(directRoom.ID, forSale, sellerID, buyerID, "first", 15000)
	secondaryInput := createQuoteInput(directRoom.ID, forSale, sellerID, buyerID, "second", 17500)
	sender.EnableBlocking()

	type createResult struct {
		quote *quoteEntity.ShippingQuote
		err   error
	}

	firstResultCh := make(chan createResult, 1)
	secondResultCh := make(chan createResult, 1)

	go func() {
		quote, err := service.CreateShippingQuote(ctx, primaryInput)
		firstResultCh <- createResult{quote: quote, err: err}
	}()

	waitForSignal(t, sender.gate.entered)

	go func() {
		quote, err := service.CreateShippingQuote(ctx, secondaryInput)
		secondResultCh <- createResult{quote: quote, err: err}
	}()

	time.Sleep(250 * time.Millisecond)
	sender.gate.releaseNow()

	firstResult := <-firstResultCh
	secondResult := <-secondResultCh
	require.NoError(t, firstResult.err)
	require.NoError(t, secondResult.err)
	require.NotNil(t, firstResult.quote)
	require.NotNil(t, secondResult.quote)

	latest, err := service.GetLatestByChatAndSource(ctx, directRoom.ID, forSale.ID, "for_sale", forSale.ID, sellerID, buyerID)
	require.NoError(t, err)
	require.NotNil(t, latest)
	require.Equal(t, "second", *latest.Note)
	require.True(t, latest.SupersededAt == nil)
	require.Equal(t, secondResult.quote.ID, latest.ID)

	firstPersisted, err := service.GetByID(ctx, firstResult.quote.ID)
	require.NoError(t, err)
	require.NotNil(t, firstPersisted)
	require.NotNil(t, firstPersisted.SupersededAt)
	require.NotNil(t, firstPersisted.SupersededByID)
	require.Equal(t, secondResult.quote.ID, *firstPersisted.SupersededByID)

	secondPersisted, err := service.GetByID(ctx, secondResult.quote.ID)
	require.NoError(t, err)
	require.NotNil(t, secondPersisted)
	require.Nil(t, secondPersisted.SupersededAt)
}

func TestShippingQuote_ReactivationVsReplacement_ReactivationFailsClosedAfterReplacement(t *testing.T) {
	tdb, service, sender, _, forSaleRepo, chatRepository := setupShippingQuoteIntegration(t)

	ctx := context.Background()
	sellerID := insertTestUser(t, ctx, tdb.Pool())
	buyerID := insertTestUser(t, ctx, tdb.Pool())

	directRoom := seedChatRoom(t, ctx, tdb, chatRepository, chatEntity.RoomTypeDirect, sellerID, buyerID)
	forSale := seedPublishedForSale(t, ctx, tdb, forSaleRepo, sellerID, "reactivation fixture")

	initialQuote, err := service.CreateShippingQuote(ctx, createQuoteInput(directRoom.ID, forSale, sellerID, buyerID, "original", 15000))
	require.NoError(t, err)
	require.NotNil(t, initialQuote)

	seedUsedQuote(t, ctx, tdb, initialQuote.ID, 0, 2)
	sender.EnableBlocking()

	type createResult struct {
		quote *quoteEntity.ShippingQuote
		err   error
	}

	replacementCh := make(chan createResult, 1)
	reactivationCh := make(chan error, 1)

	go func() {
		quote, err := service.CreateShippingQuote(ctx, createQuoteInput(directRoom.ID, forSale, sellerID, buyerID, "replacement", 17500))
		replacementCh <- createResult{quote: quote, err: err}
	}()

	waitForSignal(t, sender.gate.entered)

	go func() {
		reactivationCh <- tdb.WithTx(ctx, func(tx db.Tx) error {
			return service.ReactivateQuoteIfEligible(ctx, tx, initialQuote.ID)
		})
	}()

	time.Sleep(250 * time.Millisecond)
	sender.gate.releaseNow()

	replacementResult := <-replacementCh
	require.NoError(t, replacementResult.err)
	require.NotNil(t, replacementResult.quote)

	reactivationErr := <-reactivationCh
	require.Error(t, reactivationErr)
	require.Contains(t, reactivationErr.Error(), "superseded")

	originalPersisted, err := service.GetByID(ctx, initialQuote.ID)
	require.NoError(t, err)
	require.NotNil(t, originalPersisted)
	require.NotNil(t, originalPersisted.SupersededAt)
	require.Equal(t, 0, originalPersisted.ReactivationCount)

	latest, err := service.GetLatestByChatAndSource(ctx, directRoom.ID, forSale.ID, "for_sale", forSale.ID, sellerID, buyerID)
	require.NoError(t, err)
	require.NotNil(t, latest)
	require.Equal(t, replacementResult.quote.ID, latest.ID)
	require.Equal(t, "replacement", *latest.Note)
}

func TestShippingQuote_DuplicateReactivation_SecondAttemptNoopsWithoutDoubleIncrement(t *testing.T) {
	tdb, service, _, orderRepo, forSaleRepo, chatRepository := setupShippingQuoteIntegration(t)

	ctx := context.Background()
	sellerID := insertTestUser(t, ctx, tdb.Pool())
	buyerID := insertTestUser(t, ctx, tdb.Pool())

	room := seedChatRoom(t, ctx, tdb, chatRepository, chatEntity.RoomTypeDirect, sellerID, buyerID)
	forSale := seedPublishedForSale(t, ctx, tdb, forSaleRepo, sellerID, "duplicate reactivation fixture")

	quote, err := service.CreateShippingQuote(ctx, createQuoteInput(room.ID, forSale, sellerID, buyerID, "used", 15000))
	require.NoError(t, err)
	require.NotNil(t, quote)

	seedUsedQuote(t, ctx, tdb, quote.ID, 0, 2)
	orderRepo.EnableBlocking()

	firstErrCh := make(chan error, 1)
	secondErrCh := make(chan error, 1)

	go func() {
		firstErrCh <- tdb.WithTx(ctx, func(tx db.Tx) error {
			return service.ReactivateQuoteIfEligible(ctx, tx, quote.ID)
		})
	}()

	waitForSignal(t, orderRepo.gate.entered)

	go func() {
		secondErrCh <- tdb.WithTx(ctx, func(tx db.Tx) error {
			return service.ReactivateQuoteIfEligible(ctx, tx, quote.ID)
		})
	}()

	time.Sleep(250 * time.Millisecond)
	orderRepo.gate.releaseNow()

	require.NoError(t, <-firstErrCh)
	require.NoError(t, <-secondErrCh)

	persisted, err := service.GetByID(ctx, quote.ID)
	require.NoError(t, err)
	require.NotNil(t, persisted)
	require.Equal(t, quoteEntity.QuoteStatusActive, persisted.Status)
	require.Equal(t, 1, persisted.ReactivationCount)
	require.Nil(t, persisted.UsedAt)
	require.NotNil(t, persisted.ExpiresAt)
}

func TestShippingQuote_ReuseCap_RejectsQuoteAtLimit(t *testing.T) {
	tdb, service, _, _, forSaleRepo, chatRepository := setupShippingQuoteIntegration(t)

	ctx := context.Background()
	sellerID := insertTestUser(t, ctx, tdb.Pool())
	buyerID := insertTestUser(t, ctx, tdb.Pool())

	room := seedChatRoom(t, ctx, tdb, chatRepository, chatEntity.RoomTypeDirect, sellerID, buyerID)
	forSale := seedPublishedForSale(t, ctx, tdb, forSaleRepo, sellerID, "reuse cap fixture")

	quote, err := service.CreateShippingQuote(ctx, createQuoteInput(room.ID, forSale, sellerID, buyerID, "used", 15000))
	require.NoError(t, err)
	require.NotNil(t, quote)

	seedUsedQuote(t, ctx, tdb, quote.ID, 1, 1)

	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		return service.ReactivateQuoteIfEligible(ctx, tx, quote.ID)
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "reuse limit")

	persisted, err := service.GetByID(ctx, quote.ID)
	require.NoError(t, err)
	require.NotNil(t, persisted)
	require.Equal(t, quoteEntity.QuoteStatusUsed, persisted.Status)
	require.Equal(t, 1, persisted.ReactivationCount)
}

func TestShippingQuote_ContextSeparation_AllowsDistinctCanonicalContexts(t *testing.T) {
	tdb, service, sender, _, forSaleRepo, chatRepository := setupShippingQuoteIntegration(t)

	ctx := context.Background()
	sellerID := insertTestUser(t, ctx, tdb.Pool())
	buyerID := insertTestUser(t, ctx, tdb.Pool())

	directRoom := seedChatRoom(t, ctx, tdb, chatRepository, chatEntity.RoomTypeDirect, sellerID, buyerID)
	negotiationRoom := seedChatRoom(t, ctx, tdb, chatRepository, chatEntity.RoomTypeNegotiation, sellerID, buyerID)
	forSale := seedPublishedForSale(t, ctx, tdb, forSaleRepo, sellerID, "separation fixture")

	firstQuote, err := service.CreateShippingQuote(ctx, createQuoteInput(directRoom.ID, forSale, sellerID, buyerID, "direct", 15000))
	require.NoError(t, err)
	require.NotNil(t, firstQuote)

	// Release the blocking sender now so the second call can proceed normally.
	sender.gate.releaseNow()

	secondQuote, err := service.CreateShippingQuote(ctx, createQuoteInput(negotiationRoom.ID, forSale, sellerID, buyerID, "negotiation", 16000))
	require.NoError(t, err)
	require.NotNil(t, secondQuote)

	firstCurrent, err := service.GetLatestByChatAndSource(ctx, directRoom.ID, forSale.ID, "for_sale", forSale.ID, sellerID, buyerID)
	require.NoError(t, err)
	require.NotNil(t, firstCurrent)
	require.Equal(t, firstQuote.ID, firstCurrent.ID)
	require.Equal(t, "direct", *firstCurrent.Note)

	secondCurrent, err := service.GetLatestByChatAndSource(ctx, negotiationRoom.ID, forSale.ID, "for_sale", forSale.ID, sellerID, buyerID)
	require.NoError(t, err)
	require.NotNil(t, secondCurrent)
	require.Equal(t, secondQuote.ID, secondCurrent.ID)
	require.Equal(t, "negotiation", *secondCurrent.Note)
}
