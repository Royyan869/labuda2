package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	orderEntity "github.com/labuda/backend/internal/commerce/order/entity"
	shippingQuoteEntity "github.com/labuda/backend/internal/commerce/shipping/quote/entity"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// =============================================================================
// MOCKS FOR REACTIVATION TESTS
// =============================================================================

type mockQuoteRepo struct {
	quote            *shippingQuoteEntity.ShippingQuote
	reactivateCalled bool
	err              error
}

func (m *mockQuoteRepo) Create(_ context.Context, _ db.Tx, _ *shippingQuoteEntity.ShippingQuote) error {
	return nil
}
func (m *mockQuoteRepo) GetLatestByChatAndSource(_ context.Context, _ db.Tx, _, _ uuid.UUID, _ string, _, _, _ uuid.UUID) (*shippingQuoteEntity.ShippingQuote, error) {
	return nil, nil
}
func (m *mockQuoteRepo) GetLatestRevisionByChatAndSource(_ context.Context, _ db.Tx, _, _ uuid.UUID, _ string, _, _, _ uuid.UUID) (*shippingQuoteEntity.ShippingQuote, error) {
	return nil, nil
}
func (m *mockQuoteRepo) GetCurrentActiveByChatAndSource(_ context.Context, _ db.Tx, _, _ uuid.UUID, _ string, _, _, _ uuid.UUID) (*shippingQuoteEntity.ShippingQuote, error) {
	return nil, nil
}
func (m *mockQuoteRepo) GetByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*shippingQuoteEntity.ShippingQuote, error) {
	return m.quote, m.err
}
func (m *mockQuoteRepo) GetByIDForUpdate(_ context.Context, _ db.Tx, _ uuid.UUID) (*shippingQuoteEntity.ShippingQuote, error) {
	return m.quote, m.err
}
func (m *mockQuoteRepo) UpdateStatus(_ context.Context, _ db.Tx, _ uuid.UUID, _ shippingQuoteEntity.QuoteStatus, _ *interface{}) error {
	return nil
}
func (m *mockQuoteRepo) ReactivateQuote(_ context.Context, _ db.Tx, _ uuid.UUID) error {
	m.reactivateCalled = true
	return nil
}
func (m *mockQuoteRepo) SupersedeCurrentQuotes(_ context.Context, _ db.Tx, _, _ uuid.UUID, _ string, _, _, _, _ uuid.UUID) (int64, error) {
	return 0, nil
}
func (m *mockQuoteRepo) InvalidateQuotesByProduct(_ context.Context, _ db.Tx, _ uuid.UUID) error {
	return nil
}

type mockRoomLockGetter struct {
	room *chatEntity.ChatRoom
	err  error
}

func (m *mockRoomLockGetter) GetRoomByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*chatEntity.ChatRoom, error) {
	return m.room, m.err
}

func (m *mockRoomLockGetter) GetRoomByIDForUpdate(_ context.Context, _ db.Tx, _ uuid.UUID) (*chatEntity.ChatRoom, error) {
	return m.room, m.err
}

type mockOrderRepoForReactivation struct {
	order *orderEntity.Order
	count int64
	err   error
}

func (m *mockOrderRepoForReactivation) GetByShippingQuoteID(_ context.Context, _ db.Tx, _ uuid.UUID) (*orderEntity.Order, error) {
	return m.order, m.err
}

func (m *mockOrderRepoForReactivation) CountValidOrdersByShippingQuoteID(_ context.Context, _ db.Tx, _ uuid.UUID) (int64, error) {
	return m.count, m.err
}

// =============================================================================
// TEST: STATUS WHITELIST FOR REACTIVATION
// =============================================================================

// TestReactivateQuoteIfEligible_StatusWhitelist validates that reactivation is
// allowed for terminal failure statuses and blocked for all other statuses.
func TestReactivateQuoteIfEligible_StatusWhitelist(t *testing.T) {
	quoteID := uuid.New()

	tests := []struct {
		name             string
		orderStatus      orderEntity.Status
		shouldReactivate bool
	}{
		// SHOULD reactivate — terminal failure, shipping never consumed
		{"expired_reactivates", orderEntity.StatusExpired, true},
		{"refunded_reactivates", orderEntity.StatusRefunded, true},
		{"cancelled_reactivates", orderEntity.StatusCancelled, true},
		{"cancelled_timeout_reactivates", orderEntity.StatusCancelledTimeout, true},

		// SHOULD NOT reactivate — shipping consumed or order still active
		{"completed_does_not_reactivate", orderEntity.StatusCompleted, false},
		{"shipped_does_not_reactivate", orderEntity.StatusShipped, false},
		{"paid_does_not_reactivate", orderEntity.StatusPaid, false},
		{"pending_does_not_reactivate", orderEntity.StatusPending, false},
		{"partially_refunded_does_not_reactivate", orderEntity.StatusPartiallyRefunded, false},
		{"dispute_open_does_not_reactivate", orderEntity.StatusDisputeOpen, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quoteRepo := &mockQuoteRepo{
				quote: &shippingQuoteEntity.ShippingQuote{
					ID:                quoteID,
					ChatID:            uuid.New(),
					ProductID:         uuid.New(),
					SourceType:        ptrString("for_sale"),
					SourceID:          ptrUUID(uuid.New()),
					SellerID:          uuid.New(),
					BuyerID:           uuid.New(),
					Status:            shippingQuoteEntity.QuoteStatusUsed,
					ReactivationCount: 0,
					MaxReuse:          2,
				},
			}
			orderRepo := &mockOrderRepoForReactivation{
				order: &orderEntity.Order{
					ID:     uuid.New(),
					Status: tt.orderStatus,
				},
				count: 0, // no other valid orders
			}

			svc := &Service{
				quoteRepo:  quoteRepo,
				orderRepo:  orderRepo,
				roomGetter: &mockRoomLockGetter{room: &chatEntity.ChatRoom{}},
				log:        zap.NewNop(),
			}

			err := svc.ReactivateQuoteIfEligible(context.Background(), nil, quoteID)

			if tt.shouldReactivate {
				require.NoError(t, err)
				assert.True(t, quoteRepo.reactivateCalled,
					"expected ReactivateQuote to be called for status %s", tt.orderStatus)
			} else {
				// For non-reactivable statuses, service returns nil (no-op) without error
				assert.NoError(t, err)
				assert.False(t, quoteRepo.reactivateCalled,
					"expected ReactivateQuote NOT to be called for status %s", tt.orderStatus)
			}
		})
	}
}

// TestReactivateQuoteIfEligible_NilShippingQuoteID verifies that when order has
// no shipping quote, the service is never called (caller guards with nil check).
// This test documents the inline pattern used in OrderCompletionService.
func TestReactivateQuoteIfEligible_QuoteNotUsed(t *testing.T) {
	quoteID := uuid.New()

	quoteRepo := &mockQuoteRepo{
		quote: &shippingQuoteEntity.ShippingQuote{
			ID:                quoteID,
			ChatID:            uuid.New(),
			ProductID:         uuid.New(),
			SourceType:        ptrString("for_sale"),
			SourceID:          ptrUUID(uuid.New()),
			SellerID:          uuid.New(),
			BuyerID:           uuid.New(),
			Status:            shippingQuoteEntity.QuoteStatusActive, // Not USED
			ReactivationCount: 0,
			MaxReuse:          2,
		},
	}
	orderRepo := &mockOrderRepoForReactivation{}

	svc := &Service{
		quoteRepo:  quoteRepo,
		orderRepo:  orderRepo,
		roomGetter: &mockRoomLockGetter{room: &chatEntity.ChatRoom{}},
		log:        zap.NewNop(),
	}

	err := svc.ReactivateQuoteIfEligible(context.Background(), nil, quoteID)
	assert.NoError(t, err)
	assert.False(t, quoteRepo.reactivateCalled,
		"should not reactivate a quote that is not in USED status")
}

// TestReactivateQuoteIfEligible_ReuseLimitBlocks verifies that once
// reactivation_count reaches max_reuse, further reactivation is blocked.
func TestReactivateQuoteIfEligible_ReuseLimitBlocks(t *testing.T) {
	quoteID := uuid.New()

	quoteRepo := &mockQuoteRepo{
		quote: &shippingQuoteEntity.ShippingQuote{
			ID:                quoteID,
			ChatID:            uuid.New(),
			ProductID:         uuid.New(),
			SourceType:        ptrString("for_sale"),
			SourceID:          ptrUUID(uuid.New()),
			SellerID:          uuid.New(),
			BuyerID:           uuid.New(),
			Status:            shippingQuoteEntity.QuoteStatusUsed,
			ReactivationCount: 2,
			MaxReuse:          2, // limit reached
		},
	}
	orderRepo := &mockOrderRepoForReactivation{
		order: &orderEntity.Order{
			ID:     uuid.New(),
			Status: orderEntity.StatusCancelled,
		},
		count: 0,
	}

	svc := &Service{
		quoteRepo:  quoteRepo,
		orderRepo:  orderRepo,
		roomGetter: &mockRoomLockGetter{room: &chatEntity.ChatRoom{}},
		log:        zap.NewNop(),
	}

	err := svc.ReactivateQuoteIfEligible(context.Background(), nil, quoteID)
	assert.Error(t, err, "should return error when reuse limit exceeded")
	assert.Contains(t, err.Error(), "reuse limit exceeded")
	assert.False(t, quoteRepo.reactivateCalled)
}

// TestReactivateQuoteIfEligible_OtherValidOrdersBlocks verifies that if another
// valid order is using the same quote, reactivation is blocked.
func TestReactivateQuoteIfEligible_OtherValidOrdersBlocks(t *testing.T) {
	quoteID := uuid.New()

	quoteRepo := &mockQuoteRepo{
		quote: &shippingQuoteEntity.ShippingQuote{
			ID:                quoteID,
			ChatID:            uuid.New(),
			ProductID:         uuid.New(),
			SourceType:        ptrString("for_sale"),
			SourceID:          ptrUUID(uuid.New()),
			SellerID:          uuid.New(),
			BuyerID:           uuid.New(),
			Status:            shippingQuoteEntity.QuoteStatusUsed,
			ReactivationCount: 0,
			MaxReuse:          2,
		},
	}
	orderRepo := &mockOrderRepoForReactivation{
		order: &orderEntity.Order{
			ID:     uuid.New(),
			Status: orderEntity.StatusCancelled,
		},
		count: 1, // another valid order exists
	}

	svc := &Service{
		quoteRepo:  quoteRepo,
		orderRepo:  orderRepo,
		roomGetter: &mockRoomLockGetter{room: &chatEntity.ChatRoom{}},
		log:        zap.NewNop(),
	}

	err := svc.ReactivateQuoteIfEligible(context.Background(), nil, quoteID)
	assert.Error(t, err, "should block reactivation when other valid orders exist")
	assert.Contains(t, err.Error(), "valid order(s) still using this quote")
	assert.False(t, quoteRepo.reactivateCalled)
}
