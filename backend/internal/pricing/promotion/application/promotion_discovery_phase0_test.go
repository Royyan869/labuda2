package application

import (
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/stretchr/testify/require"
)

func TestFilterPublicPromotionInstances_IncludesExternalProduct(t *testing.T) {
	forSaleID := uuid.New()
	auctionID := uuid.New()
	externalProductID := uuid.New()

	instances := []*entity.PromotionInstance{
		{
			ID:         uuid.New(),
			TargetType: entity.TargetTypeForSale,
			TargetID:   &forSaleID,
		},
		{
			ID:         uuid.New(),
			TargetType: entity.TargetTypeExternalProduct,
			TargetID:   &externalProductID,
		},
		{
			ID:         uuid.New(),
			TargetType: entity.TargetTypeAuction,
			TargetID:   &auctionID,
		},
	}

	filtered := filterPublicPromotionInstances(instances)
	require.Len(t, filtered, 3)
	for _, inst := range filtered {
		require.True(t, inst.TargetType.IsPublicPromotable())
	}
}
