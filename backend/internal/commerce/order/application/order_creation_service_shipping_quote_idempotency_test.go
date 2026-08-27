package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
	orderrepository "github.com/labuda/backend/internal/commerce/order/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/require"
)

type shippingQuoteIdempotencyRepoStub struct {
	orderrepository.OrderRepository

	blockingOrder   *orderentity.Order
	blockingErr     error
	broadLookupSeen bool
}

func (s *shippingQuoteIdempotencyRepoStub) GetBlockingOrderByShippingQuoteID(
	_ context.Context,
	_ db.Tx,
	_ uuid.UUID,
) (*orderentity.Order, error) {
	return s.blockingOrder, s.blockingErr
}

func (s *shippingQuoteIdempotencyRepoStub) GetByShippingQuoteID(
	_ context.Context,
	_ db.Tx,
	_ uuid.UUID,
) (*orderentity.Order, error) {
	s.broadLookupSeen = true
	panic("unexpected broad shipping-quote history lookup")
}

func TestCheckShippingQuoteIdempotency_ReturnsBlockingOrder(t *testing.T) {
	buyerID := uuid.New()
	quoteID := uuid.New()
	blockingOrder := &orderentity.Order{
		ID:      uuid.New(),
		BuyerID: buyerID,
		Status:  orderentity.StatusPaid,
	}

	svc := &OrderCreationService{
		repo: &shippingQuoteIdempotencyRepoStub{
			blockingOrder: blockingOrder,
		},
	}

	existing, err := svc.checkShippingQuoteIdempotency(context.Background(), nil, quoteID, buyerID)
	require.NoError(t, err)
	require.Same(t, blockingOrder, existing)
}

func TestCheckShippingQuoteIdempotency_IgnoresHistoricalNonBlockingOrders(t *testing.T) {
	buyerID := uuid.New()
	quoteID := uuid.New()
	repo := &shippingQuoteIdempotencyRepoStub{
		blockingOrder: nil,
	}

	svc := &OrderCreationService{
		repo: repo,
	}

	existing, err := svc.checkShippingQuoteIdempotency(context.Background(), nil, quoteID, buyerID)
	require.NoError(t, err)
	require.Nil(t, existing)
	require.False(t, repo.broadLookupSeen, "service should not use broad historical shipping-quote lookup")
}


