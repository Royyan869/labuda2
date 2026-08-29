package application

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"

	"github.com/labuda/backend/internal/pricing/discount/entity"
)

func TestValidateDiscountInput_Structure(t *testing.T) {
	userID := uuid.New()
	sellerID := uuid.New()

	input := ValidateDiscountInput{
		UserID:      userID,
		Code:        "SAVE10",
		Subtotal:    10000,
		ContextType: entity.DiscountContextForSale,
		SellerID:    &sellerID,
	}

	assert.Equal(t, userID, input.UserID)
	assert.Equal(t, entity.DiscountContextForSale, input.ContextType)
}

func TestCreateDiscountInput_Structure(t *testing.T) {
	sellerID := uuid.New()

	input := CreateDiscountInput{
		Code:        "SAVE10",
		Type:        entity.DiscountTypePercentage,
		Value:       decimal.NewFromInt(10),
		MinPurchase: decimal.NewFromInt(100000),
		AppliesTo:   entity.DiscountAppliesToForSale,
		SellerID:    &sellerID,
		ValidUntil:  time.Now().Add(24 * time.Hour),
	}

	assert.Equal(t, entity.DiscountAppliesToForSale, input.AppliesTo)
	assert.True(t, input.MinPurchase.Equal(decimal.NewFromInt(100000)))
}
