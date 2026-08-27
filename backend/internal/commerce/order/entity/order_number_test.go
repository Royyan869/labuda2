package entity_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/order/entity"
	"github.com/labuda/backend/pkg/money"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// ORDER NUMBER TESTS
// ============================================================================

// TestGenerateOrderNumber_Format verifies format: ORD-YYYYMMDD-XXXXXXXX
func TestGenerateOrderNumber_Format(t *testing.T) {
	num := entity.GenerateOrderNumber()

	require.NotEmpty(t, num)
	assert.True(t, strings.HasPrefix(num, "ORD-"), "must start with ORD-")

	parts := strings.Split(num, "-")
	require.Len(t, parts, 3, "format must be ORD-YYYYMMDD-XXXXXXXX")

	dateStr := parts[1]
	assert.Len(t, dateStr, 8, "date part must be 8 chars")
	today := time.Now().Format("20060102")
	assert.Equal(t, today, dateStr, "date part must be today's date")

	suffix := parts[2]
	assert.Len(t, suffix, 8, "suffix part must be 8 chars")
}

// TestGenerateOrderNumber_Unique verifies two calls return different values.
func TestGenerateOrderNumber_Unique(t *testing.T) {
	a := entity.GenerateOrderNumber()
	b := entity.GenerateOrderNumber()
	assert.NotEqual(t, a, b, "two generated order numbers must differ")
}

// TestNewOrderFromSource_SetsOrderNumber verifies that NewOrderFromSource always
// populates OrderNumber so every newly created order has a human-readable identifier.
//
// REGRESSION LOCK: OrderNumber must never be nil after construction.
func TestNewOrderFromSource_SetsOrderNumber(t *testing.T) {
	order := entity.NewOrderFromSource(
		uuid.New(),
		uuid.New(),
		entity.OrderSourceForSale,
		uuid.New(),
		nil,
		1,
		money.New(100000),
		money.New(100000),
		money.Zero(),
		5,
		money.New(5000),
		money.New(3000),
		money.New(108000),
		nil,
		"JNE",
		"truck",
		nil,
		nil,
		nil,
		"immediate",
		nil,
		nil,
		nil,
		nil,
		nil,
		"instant",
		time.Now().Add(15*time.Minute),
	)

	require.NotNil(t, order.OrderNumber, "OrderNumber must not be nil")
	assert.True(t, strings.HasPrefix(*order.OrderNumber, "ORD-"), "OrderNumber must start with ORD-")
	assert.Greater(t, len(*order.OrderNumber), 10, "OrderNumber must have reasonable length")
}

// TestNewOrderFromSource_OrderNumberMatchesFormat verifies the exact
// ORD-YYYYMMDD-XXXXXXXX format is honoured by NewOrderFromSource.
func TestNewOrderFromSource_OrderNumberMatchesFormat(t *testing.T) {
	order := entity.NewOrderFromSource(
		uuid.New(), uuid.New(),
		entity.OrderSourceAuction, uuid.New(),
		nil, 1,
		money.New(200000), money.New(200000), money.Zero(),
		5, money.New(10000),
		money.New(3000),
		money.New(213000),
		nil, "TIKI", "plane",
		nil, nil,
		&[]entity.AuctionSettlementType{entity.AuctionSettlementBidWin}[0],
		"short", nil,
		nil, nil, nil, nil,
		"va",
		time.Now().Add(time.Hour),
	)

	require.NotNil(t, order.OrderNumber)
	parts := strings.Split(*order.OrderNumber, "-")
	require.Len(t, parts, 3)
	assert.Equal(t, "ORD", parts[0])
	assert.Len(t, parts[1], 8)
	assert.Len(t, parts[2], 8)
}


