package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	shippingQuoteEntity "github.com/labuda/backend/internal/commerce/shipping/quote/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// consumeQuoteRepoStub is a minimal ShippingQuoteRepository stub controlling
// the two methods ConsumeQuoteForCheckout uses: GetByIDForUpdate (lock + read)
// and UpdateStatus (persist USED). Every other method panics loudly so a future
// change that starts using one is caught here instead of silently passing.
type consumeQuoteRepoStub struct {
	quote         *shippingQuoteEntity.ShippingQuote
	updatedStatus shippingQuoteEntity.QuoteStatus
	updatedUsedAt *interface{}
	updateCalls   int
}

func (r *consumeQuoteRepoStub) Create(context.Context, db.Tx, *shippingQuoteEntity.ShippingQuote) error {
	panic("not exercised")
}
func (r *consumeQuoteRepoStub) GetLatestByChatAndSource(context.Context, db.Tx, uuid.UUID, uuid.UUID, string, uuid.UUID, uuid.UUID, uuid.UUID) (*shippingQuoteEntity.ShippingQuote, error) {
	panic("not exercised")
}
func (r *consumeQuoteRepoStub) GetLatestRevisionByChatAndSource(context.Context, db.Tx, uuid.UUID, uuid.UUID, string, uuid.UUID, uuid.UUID, uuid.UUID) (*shippingQuoteEntity.ShippingQuote, error) {
	panic("not exercised")
}
func (r *consumeQuoteRepoStub) GetByID(context.Context, db.Tx, uuid.UUID) (*shippingQuoteEntity.ShippingQuote, error) {
	panic("not exercised")
}
func (r *consumeQuoteRepoStub) GetByIDs(context.Context, db.Tx, []uuid.UUID) (map[uuid.UUID]*shippingQuoteEntity.ShippingQuote, error) {
	panic("not exercised")
}
func (r *consumeQuoteRepoStub) GetByIDForUpdate(context.Context, db.Tx, uuid.UUID) (*shippingQuoteEntity.ShippingQuote, error) {
	return r.quote, nil
}
func (r *consumeQuoteRepoStub) UpdateStatus(_ context.Context, _ db.Tx, _ uuid.UUID, status shippingQuoteEntity.QuoteStatus, usedAt *interface{}) error {
	r.updatedStatus = status
	r.updatedUsedAt = usedAt
	r.updateCalls++
	return nil
}
func (r *consumeQuoteRepoStub) ReactivateQuote(context.Context, db.Tx, uuid.UUID) error {
	panic("not exercised")
}
func (r *consumeQuoteRepoStub) GetCurrentActiveByChatAndSource(context.Context, db.Tx, uuid.UUID, uuid.UUID, string, uuid.UUID, uuid.UUID, uuid.UUID) (*shippingQuoteEntity.ShippingQuote, error) {
	panic("not exercised")
}
func (r *consumeQuoteRepoStub) SupersedeCurrentQuotes(context.Context, db.Tx, uuid.UUID, uuid.UUID, string, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) (int64, error) {
	panic("not exercised")
}
func (r *consumeQuoteRepoStub) InvalidateQuotesByProduct(context.Context, db.Tx, uuid.UUID) error {
	panic("not exercised")
}

type consumeCtx struct {
	buyer   uuid.UUID
	seller  uuid.UUID
	product uuid.UUID
	source  uuid.UUID
	chat    uuid.UUID
}

func newConsumableQuote(expiresAt time.Time, c consumeCtx) *shippingQuoteEntity.ShippingQuote {
	sourceType := "for_sale"
	province := "31"
	city := "3171"
	return &shippingQuoteEntity.ShippingQuote{
		ID:                    uuid.New(),
		ChatID:                c.chat,
		ProductID:             c.product,
		SourceType:            &sourceType,
		SourceID:              &c.source,
		SellerID:              c.seller,
		BuyerID:               c.buyer,
		Status:                shippingQuoteEntity.QuoteStatusActive,
		DestinationProvinceID: &province,
		DestinationCityID:     &city,
		ExpiresAt:             &expiresAt,
		CreatedAt:             time.Now(),
	}
}

func newConsumeService(repo *consumeQuoteRepoStub) *Service {
	return NewService(nil, repo, nil, nil, nil, nil, nil, zap.NewNop())
}

func checkoutContextFor(q *shippingQuoteEntity.ShippingQuote) shippingQuoteEntity.CheckoutContext {
	return shippingQuoteEntity.CheckoutContext{
		QuoteID:            q.ID,
		BuyerID:            q.BuyerID,
		ProductID:          q.ProductID,
		SourceType:         "for_sale",
		SourceID:           *q.SourceID,
		SellerID:           q.SellerID,
		ChatID:             &q.ChatID,
		ShippingProvinceID: "31",
		ShippingCityID:     "3171",
	}
}

// TestConsumeQuoteForCheckout_ValidQuote_MarksUsed proves the ONE authority
// validates a matching quote and transitions it ACTIVE -> USED.
func TestConsumeQuoteForCheckout_ValidQuote_MarksUsed(t *testing.T) {
	c := consumeCtx{buyer: uuid.New(), seller: uuid.New(), product: uuid.New(), source: uuid.New(), chat: uuid.New()}
	quote := newConsumableQuote(time.Now().Add(24*time.Hour), c)
	repo := &consumeQuoteRepoStub{quote: quote}
	svc := newConsumeService(repo)

	got, err := svc.ConsumeQuoteForCheckout(context.Background(), nil, checkoutContextFor(quote))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, shippingQuoteEntity.QuoteStatusUsed, got.Status)
	require.Equal(t, shippingQuoteEntity.QuoteStatusUsed, repo.updatedStatus)
	require.NotNil(t, repo.updatedUsedAt)
	require.Equal(t, 1, repo.updateCalls)
}

func TestConsumeQuoteForCheckout_RejectsExpired(t *testing.T) {
	c := consumeCtx{buyer: uuid.New(), seller: uuid.New(), product: uuid.New(), source: uuid.New(), chat: uuid.New()}
	quote := newConsumableQuote(time.Now().Add(-1*time.Hour), c)
	repo := &consumeQuoteRepoStub{quote: quote}
	svc := newConsumeService(repo)

	_, err := svc.ConsumeQuoteForCheckout(context.Background(), nil, checkoutContextFor(quote))
	requireRejection(t, err, "expired")
	require.Equal(t, 0, repo.updateCalls, "rejected quote must not be persisted")
}

func TestConsumeQuoteForCheckout_RejectsWrongBuyer(t *testing.T) {
	c := consumeCtx{buyer: uuid.New(), seller: uuid.New(), product: uuid.New(), source: uuid.New(), chat: uuid.New()}
	quote := newConsumableQuote(time.Now().Add(24*time.Hour), c)
	repo := &consumeQuoteRepoStub{quote: quote}
	svc := newConsumeService(repo)

	in := checkoutContextFor(quote)
	in.BuyerID = uuid.New()
	_, err := svc.ConsumeQuoteForCheckout(context.Background(), nil, in)
	requireRejection(t, err, "buyer_mismatch")
}

func TestConsumeQuoteForCheckout_RejectsWrongSource(t *testing.T) {
	c := consumeCtx{buyer: uuid.New(), seller: uuid.New(), product: uuid.New(), source: uuid.New(), chat: uuid.New()}
	quote := newConsumableQuote(time.Now().Add(24*time.Hour), c)
	repo := &consumeQuoteRepoStub{quote: quote}
	svc := newConsumeService(repo)

	in := checkoutContextFor(quote)
	in.SourceID = uuid.New()
	_, err := svc.ConsumeQuoteForCheckout(context.Background(), nil, in)
	requireRejection(t, err, "source_mismatch")
}

func TestConsumeQuoteForCheckout_RejectsWrongChat(t *testing.T) {
	c := consumeCtx{buyer: uuid.New(), seller: uuid.New(), product: uuid.New(), source: uuid.New(), chat: uuid.New()}
	quote := newConsumableQuote(time.Now().Add(24*time.Hour), c)
	repo := &consumeQuoteRepoStub{quote: quote}
	svc := newConsumeService(repo)

	in := checkoutContextFor(quote)
	otherChat := uuid.New()
	in.ChatID = &otherChat
	_, err := svc.ConsumeQuoteForCheckout(context.Background(), nil, in)
	requireRejection(t, err, "chat_mismatch")
}

func TestConsumeQuoteForCheckout_RejectsSuperseded(t *testing.T) {
	c := consumeCtx{buyer: uuid.New(), seller: uuid.New(), product: uuid.New(), source: uuid.New(), chat: uuid.New()}
	quote := newConsumableQuote(time.Now().Add(24*time.Hour), c)
	now := time.Now()
	superseder := uuid.New()
	quote.SupersededAt = &now
	quote.SupersededByID = &superseder
	repo := &consumeQuoteRepoStub{quote: quote}
	svc := newConsumeService(repo)

	_, err := svc.ConsumeQuoteForCheckout(context.Background(), nil, checkoutContextFor(quote))
	requireRejection(t, err, "superseded")
}

func TestConsumeQuoteForCheckout_RejectsAddressMismatch(t *testing.T) {
	c := consumeCtx{buyer: uuid.New(), seller: uuid.New(), product: uuid.New(), source: uuid.New(), chat: uuid.New()}
	quote := newConsumableQuote(time.Now().Add(24*time.Hour), c)
	repo := &consumeQuoteRepoStub{quote: quote}
	svc := newConsumeService(repo)

	in := checkoutContextFor(quote)
	in.ShippingCityID = "9999"
	_, err := svc.ConsumeQuoteForCheckout(context.Background(), nil, in)
	requireRejection(t, err, "address_mismatch")
}

func requireRejection(t *testing.T, err error, field string) {
	t.Helper()
	require.Error(t, err)
	var rejection *shippingQuoteEntity.CheckoutRejectionError
	require.True(t, errors.As(err, &rejection), "expected CheckoutRejectionError, got %T: %v", err, err)
	require.Equal(t, field, rejection.Field)
}
