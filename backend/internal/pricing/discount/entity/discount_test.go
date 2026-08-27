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

func TestDiscountTargetMode_IsValid(t *testing.T) {
	assert.True(t, DiscountTargetModeSellerWide.IsValid())
	assert.True(t, DiscountTargetModeSelectedItems.IsValid())
	assert.False(t, DiscountTargetMode("variety").IsValid())
}

func TestNewDiscount_Valid(t *testing.T) {
	now := time.Now()
	later := now.Add(24 * time.Hour)
	sellerID := uuid.New()

	discount, err := NewDiscount(
		"SALE10",
		DiscountTypePercentage,
		decimal.NewFromInt(10),
		decimal.Zero,
		nil,
		DiscountAppliesToBoth,
		DiscountTargetModeSellerWide,
		&sellerID,
		nil,
		nil,
		now,
		later,
		0,
		0,
	)
	require.NoError(t, err)
	assert.Equal(t, DiscountAppliesToBoth, discount.AppliesTo)
	assert.Equal(t, DiscountTargetModeSellerWide, discount.TargetMode)
	assert.Equal(t, &sellerID, discount.SellerID)
}

func TestNewDiscount_SelectedItemsRequiresTargets(t *testing.T) {
	now := time.Now()
	later := now.Add(24 * time.Hour)
	sellerID := uuid.New()

	_, err := NewDiscount(
		"SALE10",
		DiscountTypePercentage,
		decimal.NewFromInt(10),
		decimal.Zero,
		nil,
		DiscountAppliesToForSale,
		DiscountTargetModeSelectedItems,
		&sellerID,
		nil,
		nil,
		now,
		later,
		0,
		0,
	)
	require.Error(t, err)
}

func TestDiscount_MatchPriority(t *testing.T) {
	discounts := []*Discount{
		{AppliesTo: DiscountAppliesToBoth, TargetMode: DiscountTargetModeSellerWide},
		{AppliesTo: DiscountAppliesToForSale, TargetMode: DiscountTargetModeSellerWide},
		{AppliesTo: DiscountAppliesToBoth, TargetMode: DiscountTargetModeSelectedItems},
		{AppliesTo: DiscountAppliesToForSale, TargetMode: DiscountTargetModeSelectedItems},
	}

	assert.Equal(t, 1, discounts[0].MatchPriority(DiscountContextForSale))
	assert.Equal(t, 3, discounts[1].MatchPriority(DiscountContextForSale))
	assert.Equal(t, 2, discounts[2].MatchPriority(DiscountContextForSale))
	assert.Equal(t, 4, discounts[3].MatchPriority(DiscountContextForSale))
}

func TestDiscount_IsBetterThan(t *testing.T) {
	subtotal := decimal.NewFromInt(10000)
	better := &Discount{
		AppliesTo:  DiscountAppliesToForSale,
		TargetMode: DiscountTargetModeSelectedItems,
		Type:       DiscountTypePercentage,
		Value:      decimal.NewFromInt(20),
	}
	worse := &Discount{
		AppliesTo:  DiscountAppliesToBoth,
		TargetMode: DiscountTargetModeSellerWide,
		Type:       DiscountTypePercentage,
		Value:      decimal.NewFromInt(30),
	}

	assert.True(t, better.IsBetterThan(worse, subtotal, DiscountContextForSale))
}


