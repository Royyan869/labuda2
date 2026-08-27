package application

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/internal/pricing/discount/entity"
)

func TestValidateDiscountInput_Structure(t *testing.T) {
	userID := uuid.New()
	sellerID := uuid.New()
	forSaleID := uuid.New()

	input := ValidateDiscountInput{
		UserID:      userID,
		Code:        "SAVE10",
		Subtotal:    10000,
		ContextType: entity.DiscountContextForSale,
		SellerID:    &sellerID,
		ForSaleID:   &forSaleID,
	}

	assert.Equal(t, userID, input.UserID)
	assert.Equal(t, entity.DiscountContextForSale, input.ContextType)
}

func TestCreateDiscountInput_Structure(t *testing.T) {
	sellerID := uuid.New()
	forSaleID := uuid.New()

	input := CreateDiscountInput{
		Code:            "SAVE10",
		Type:            entity.DiscountTypePercentage,
		Value:           decimal.NewFromInt(10),
		MinPurchase:     decimal.Zero,
		AppliesTo:       entity.DiscountAppliesToForSale,
		TargetMode:      entity.DiscountTargetModeSelectedItems,
		SellerID:        &sellerID,
		ForSaleIDs:      []uuid.UUID{forSaleID},
		ValidFrom:       time.Now(),
		ValidUntil:      time.Now().Add(24 * time.Hour),
		MaxUsagePerUser: 1,
		TotalUsageLimit: 1,
	}

	assert.Equal(t, entity.DiscountAppliesToForSale, input.AppliesTo)
	assert.Equal(t, entity.DiscountTargetModeSelectedItems, input.TargetMode)
}

func TestDiscountService_SelectionPrefersSelectedItems(t *testing.T) {
	now := time.Now()
	later := now.Add(24 * time.Hour)
	sellerID := uuid.New()
	forSaleID := uuid.New()

	selected, err := entity.NewDiscount(
		"SELECTED",
		entity.DiscountTypePercentage,
		decimal.NewFromInt(10),
		decimal.Zero,
		nil,
		entity.DiscountAppliesToForSale,
		entity.DiscountTargetModeSelectedItems,
		&sellerID,
		[]uuid.UUID{forSaleID},
		nil,
		now,
		later,
		1,
		10,
	)
	require.NoError(t, err)

	wide, err := entity.NewDiscount(
		"WIDE",
		entity.DiscountTypePercentage,
		decimal.NewFromInt(20),
		decimal.Zero,
		nil,
		entity.DiscountAppliesToBoth,
		entity.DiscountTargetModeSellerWide,
		&sellerID,
		nil,
		nil,
		now,
		later,
		1,
		10,
	)
	require.NoError(t, err)

	assert.True(t, selected.IsBetterThan(wide, decimal.NewFromInt(10000), entity.DiscountContextForSale))
}


