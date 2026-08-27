package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	orderEntity "github.com/labuda/backend/internal/commerce/order/entity"
	shippingQuoteEntity "github.com/labuda/backend/internal/commerce/shipping/quote/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/stretchr/testify/assert"
)

// TestReactivationLimit_CanBeReactivated tests the entity method.
func TestReactivationLimit_CanBeReactivated(t *testing.T) {
	t.Run("returns_true_when_count_less_than_max", func(t *testing.T) {
		quote := &shippingQuoteEntity.ShippingQuote{
			Status:            shippingQuoteEntity.QuoteStatusUsed,
			ReactivationCount: 1,
			MaxReuse:          2,
		}
		assert.True(t, quote.CanBeReactivated())
	})

	t.Run("returns_false_when_count_equals_max", func(t *testing.T) {
		quote := &shippingQuoteEntity.ShippingQuote{
			Status:            shippingQuoteEntity.QuoteStatusUsed,
			ReactivationCount: 2,
			MaxReuse:          2,
		}
		assert.False(t, quote.CanBeReactivated())
	})

	t.Run("returns_false_when_count_exceeds_max", func(t *testing.T) {
		quote := &shippingQuoteEntity.ShippingQuote{
			Status:            shippingQuoteEntity.QuoteStatusUsed,
			ReactivationCount: 3,
			MaxReuse:          2,
		}
		assert.False(t, quote.CanBeReactivated())
	})

	t.Run("returns_false_when_not_used_status", func(t *testing.T) {
		quote := &shippingQuoteEntity.ShippingQuote{
			Status:            shippingQuoteEntity.QuoteStatusActive,
			ReactivationCount: 0,
			MaxReuse:          2,
		}
		assert.False(t, quote.CanBeReactivated())
	})
}

// TestReactivationLimit_IncrementReactivationCount tests the entity method.
func TestReactivationLimit_IncrementReactivationCount(t *testing.T) {
	quote := &shippingQuoteEntity.ShippingQuote{
		ReactivationCount: 1,
	}

	quote.IncrementReactivationCount()
	assert.Equal(t, 2, quote.ReactivationCount)

	quote.IncrementReactivationCount()
	assert.Equal(t, 3, quote.ReactivationCount)
}

// TestReactivationLimit_NewShippingQuoteHasDefaultValues tests that new quotes
// are created with proper default values for reactivation fields.
func TestReactivationLimit_NewShippingQuoteHasDefaultValues(t *testing.T) {
	quote := shippingQuoteEntity.NewShippingQuote(
		uuid.New(),
		uuid.New(),
		"for_sale",
		uuid.New(),
		uuid.New(),
		uuid.New(),
		money.New(10000),
		nil,
		nil,
		nil,
		time.Now().Add(24*time.Hour),
	)

	assert.Equal(t, 0, quote.ReactivationCount, "new quotes should have 0 reactivations")
	assert.Equal(t, 0, quote.MaxReuse, "new quotes should default to 0 (will be set by migration default)")
	assert.NotNil(t, quote.ExpiresAt, "new quotes must always carry a non-null expiry")
}

// TestReactivationLimit_PriceIntegrity tests that the quote price is never
// recalculated or modified during reactivation.
func TestReactivationLimit_PriceIntegrity(t *testing.T) {
	t.Run("price_never_changes_on_entity_operations", func(t *testing.T) {
		originalPrice := money.New(15000)
		expiresAt := time.Now().Add(48 * time.Hour)
		quote := &shippingQuoteEntity.ShippingQuote{
			ID:                uuid.New(),
			ChatID:            uuid.New(),
			ProductID:         uuid.New(),
			SourceType:        ptrString("for_sale"),
			SourceID:          ptrUUID(uuid.New()),
			SellerID:          uuid.New(),
			BuyerID:           uuid.New(),
			Cost:              originalPrice,
			Status:            shippingQuoteEntity.QuoteStatusUsed,
			ExpiresAt:         &expiresAt,
			ReactivationCount: 0,
			MaxReuse:          2,
			CreatedAt:         time.Now(),
		}

		quote.IncrementReactivationCount()
		assert.Equal(t, originalPrice.Int64(), quote.Cost.Int64())

		err := quote.Reactivate(time.Now().Add(24 * time.Hour))
		assert.NoError(t, err)
		assert.Equal(t, originalPrice.Int64(), quote.Cost.Int64())

		err = quote.MarkUsed(time.Now())
		assert.NoError(t, err)
		assert.Equal(t, originalPrice.Int64(), quote.Cost.Int64())
	})
}

// MockOrderRepo is a minimal mock for testing.
type MockOrderRepo struct {
	order *orderEntity.Order
	count int64
	err   error
}

func (m *MockOrderRepo) GetByShippingQuoteID(ctx context.Context, tx db.Tx, quoteID uuid.UUID) (*orderEntity.Order, error) {
	return m.order, m.err
}

func (m *MockOrderRepo) CountValidOrdersByShippingQuoteID(ctx context.Context, tx db.Tx, quoteID uuid.UUID) (int64, error) {
	return m.count, m.err
}

func ptrString(v string) *string     { return &v }
func ptrUUID(v uuid.UUID) *uuid.UUID { return &v }
