package application

import (
	"testing"

	"github.com/google/uuid"
	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
	orderrepository "github.com/labuda/backend/internal/commerce/order/repository"
	"github.com/stretchr/testify/require"
)

func TestIdempotentOrderRecovery_PreservesExistingOrder(t *testing.T) {
	pricingTokenID := uuid.New()
	existing := &orderentity.Order{
		PricingTokenID: &pricingTokenID,
	}

	recovered, err := idempotentOrderRecovery(existing, &pricingTokenID)
	require.NoError(t, err)
	require.Same(t, existing, recovered)
}

func TestIdempotentOrderRecovery_AllowsAuctionStyleRetryWithoutPricingToken(t *testing.T) {
	existing := &orderentity.Order{}

	recovered, err := idempotentOrderRecovery(existing, nil)
	require.NoError(t, err)
	require.Same(t, existing, recovered)
}

func TestIdempotentOrderRecovery_RejectsDifferentPricingToken(t *testing.T) {
	existingTokenID := uuid.New()
	requestedTokenID := uuid.New()
	existing := &orderentity.Order{
		PricingTokenID: &existingTokenID,
	}

	recovered, err := idempotentOrderRecovery(existing, &requestedTokenID)
	require.ErrorIs(t, err, orderrepository.ErrDuplicateIdempotencyKey)
	require.Nil(t, recovered)
}


