package entity

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscountAppliesTo_IsValid(t *testing.T) {
	assert.True(t, DiscountAppliesToForSale.IsValid())
	assert.True(t, DiscountAppliesToAuction.IsValid())
	assert.True(t, DiscountAppliesToBoth.IsValid())
	assert.False(t, DiscountAppliesTo("platform").IsValid())
}

func TestNewDiscount_Valid(t *testing.T) {
	later := time.Now().Add(24 * time.Hour)
	sellerID := uuid.New()

	discount, err := NewDiscount(
		"SALE10",
		DiscountTypePercentage,
		decimal.NewFromInt(10),
		decimal.NewFromInt(100000),
		DiscountAppliesToBoth,
		&sellerID,
		later,
		0,
	)
	require.NoError(t, err)
	assert.Equal(t, DiscountAppliesToBoth, discount.AppliesTo)
	assert.Equal(t, &sellerID, discount.SellerID)
	assert.True(t, discount.MinPurchase.Equal(decimal.NewFromInt(100000)))
}

func TestNewDiscount_ExpiryOnly(t *testing.T) {
	sellerID := uuid.New()
	later := time.Now().Add(24 * time.Hour)

	discount, err := NewDiscount(
		"TEST",
		DiscountTypePercentage,
		decimal.NewFromInt(10),
		decimal.Zero,
		DiscountAppliesToForSale,
		&sellerID,
		later,
		0,
	)
	require.NoError(t, err)
	// Discount should be active immediately (no valid_from)
	assert.True(t, discount.IsActiveNow())
}

func TestDiscount_CalculateDiscountAmount_Percentage(t *testing.T) {
	sellerID := uuid.New()
	later := time.Now().Add(24 * time.Hour)

	discount, err := NewDiscount(
		"SALE10",
		DiscountTypePercentage,
		decimal.NewFromInt(10),
		decimal.Zero,
		DiscountAppliesToForSale,
		&sellerID,
		later,
		0,
	)
	require.NoError(t, err)

	subtotal := decimal.NewFromInt(100000)
	amount := discount.CalculateDiscountAmount(subtotal)
	assert.True(t, amount.Equal(decimal.NewFromInt(10000)), "expected 10000, got %s", amount.String())
}

func TestDiscount_CalculateDiscountAmount_FlatAmount(t *testing.T) {
	sellerID := uuid.New()
	later := time.Now().Add(24 * time.Hour)

	discount, err := NewDiscount(
		"FLAT50K",
		DiscountTypeFlatAmount,
		decimal.NewFromInt(50000),
		decimal.Zero,
		DiscountAppliesToForSale,
		&sellerID,
		later,
		0,
	)
	require.NoError(t, err)

	subtotal := decimal.NewFromInt(100000)
	amount := discount.CalculateDiscountAmount(subtotal)
	assert.Equal(t, decimal.NewFromInt(50000), amount)
}

func TestDiscount_CalculateDiscountAmount_CappedAtSubtotal(t *testing.T) {
	sellerID := uuid.New()
	later := time.Now().Add(24 * time.Hour)

	discount, err := NewDiscount(
		"HUGE",
		DiscountTypeFlatAmount,
		decimal.NewFromInt(200000),
		decimal.Zero,
		DiscountAppliesToForSale,
		&sellerID,
		later,
		0,
	)
	require.NoError(t, err)

	subtotal := decimal.NewFromInt(100000)
	amount := discount.CalculateDiscountAmount(subtotal)
	// Flat amount capped at subtotal
	assert.Equal(t, decimal.NewFromInt(100000), amount)
}

func TestDiscount_CanBeUsedBy_TotalUsageLimit(t *testing.T) {
	sellerID := uuid.New()
	later := time.Now().Add(24 * time.Hour)

	discount, err := NewDiscount(
		"LIMITED",
		DiscountTypePercentage,
		decimal.NewFromInt(10),
		decimal.Zero,
		DiscountAppliesToForSale,
		&sellerID,
		later,
		2, // total usage limit = 2
	)
	require.NoError(t, err)

	// Can be used initially
	assert.NoError(t, discount.CanBeUsedBy())

	// After 1 usage
	discount.IncrementUsage()
	assert.NoError(t, discount.CanBeUsedBy())

	// After 2 usages (at limit)
	discount.IncrementUsage()
	err = discount.CanBeUsedBy()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "total usage limit exceeded")
}

func TestDiscount_CanBeUsedBy_Unlimited(t *testing.T) {
	sellerID := uuid.New()
	later := time.Now().Add(24 * time.Hour)

	discount, err := NewDiscount(
		"UNLIMITED",
		DiscountTypePercentage,
		decimal.NewFromInt(10),
		decimal.Zero,
		DiscountAppliesToForSale,
		&sellerID,
		later,
		0, // unlimited
	)
	require.NoError(t, err)

	// Can be used many times
	for i := 0; i < 100; i++ {
		assert.NoError(t, discount.CanBeUsedBy())
		discount.IncrementUsage()
	}
}

func TestDiscount_IsActiveNow_Expired(t *testing.T) {
	sellerID := uuid.New()
	past := time.Now().Add(-1 * time.Hour)

	_, err := NewDiscount(
		"EXPIRED",
		DiscountTypePercentage,
		decimal.NewFromInt(10),
		decimal.Zero,
		DiscountAppliesToForSale,
		&sellerID,
		past, // already expired
		0,
	)
	require.Error(t, err) // NewDiscount rejects past validUntil
	assert.Contains(t, err.Error(), "valid_until cannot be in the past")
}

func TestDiscount_ValidateEconomicSafety_PercentageCap(t *testing.T) {
	sellerID := uuid.New()
	later := time.Now().Add(24 * time.Hour)

	discount, err := NewDiscount(
		"HUGE",
		DiscountTypePercentage,
		decimal.NewFromInt(60), // 60% — exceeds 50% cap
		decimal.Zero,
		DiscountAppliesToForSale,
		&sellerID,
		later,
		0,
	)
	require.NoError(t, err) // NewDiscount allows it (validation is separate)

	err = discount.ValidateEconomicSafety()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum allowed")
}

func TestDiscount_AllowsContext(t *testing.T) {
	assert.True(t, DiscountAppliesToBoth.AllowsContext(DiscountContextForSale))
	assert.True(t, DiscountAppliesToBoth.AllowsContext(DiscountContextAuction))
	assert.True(t, DiscountAppliesToForSale.AllowsContext(DiscountContextForSale))
	assert.False(t, DiscountAppliesToForSale.AllowsContext(DiscountContextAuction))
	assert.True(t, DiscountAppliesToAuction.AllowsContext(DiscountContextAuction))
	assert.False(t, DiscountAppliesToAuction.AllowsContext(DiscountContextForSale))
}

func TestDiscount_MeetsMinPurchase(t *testing.T) {
	sellerID := uuid.New()
	later := time.Now().Add(24 * time.Hour)

	discount, err := NewDiscount(
		"MIN100K",
		DiscountTypePercentage,
		decimal.NewFromInt(20),
		decimal.NewFromInt(100000), // min purchase = 100,000
		DiscountAppliesToBoth,
		&sellerID,
		later,
		0,
	)
	require.NoError(t, err)

	// Below minimum — rejected
	err = discount.MeetsMinPurchase(decimal.NewFromInt(50000))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "minimum purchase not met")

	// Exactly minimum — accepted
	err = discount.MeetsMinPurchase(decimal.NewFromInt(100000))
	assert.NoError(t, err)

	// Above minimum — accepted
	err = discount.MeetsMinPurchase(decimal.NewFromInt(200000))
	assert.NoError(t, err)
}

func TestDiscount_MeetsMinPurchase_ZeroMinPurchase(t *testing.T) {
	sellerID := uuid.New()
	later := time.Now().Add(24 * time.Hour)

	discount, err := NewDiscount(
		"NO_MIN",
		DiscountTypePercentage,
		decimal.NewFromInt(10),
		decimal.Zero, // no minimum purchase
		DiscountAppliesToBoth,
		&sellerID,
		later,
		0,
	)
	require.NoError(t, err)

	// Any amount accepted when no min purchase
	err = discount.MeetsMinPurchase(decimal.NewFromInt(1))
	assert.NoError(t, err)
}

func TestDiscount_NegativeMinPurchase_Rejected(t *testing.T) {
	sellerID := uuid.New()
	later := time.Now().Add(24 * time.Hour)

	_, err := NewDiscount(
		"NEG_MIN",
		DiscountTypePercentage,
		decimal.NewFromInt(10),
		decimal.NewFromInt(-100), // negative min purchase
		DiscountAppliesToBoth,
		&sellerID,
		later,
		0,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "min_purchase cannot be negative")
}
