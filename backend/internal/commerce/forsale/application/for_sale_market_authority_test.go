package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/forsale/entity"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRoleChecker is a minimal auth.RoleChecker stub controlling only
// HasActiveSellerCapability, which is all CheckMarketAuthorityForForSale
// consumes.
type fakeRoleChecker struct {
	hasCapability bool
	err           error
}

func (f fakeRoleChecker) IsAdmin(context.Context, uuid.UUID) (bool, error) { return false, nil }
func (f fakeRoleChecker) IsSeller(context.Context, uuid.UUID) (bool, error) {
	return f.hasCapability, f.err
}
func (f fakeRoleChecker) HasActiveSellerCapability(context.Context, uuid.UUID) (bool, error) {
	return f.hasCapability, f.err
}
func (f fakeRoleChecker) HasSellerProfile(context.Context, uuid.UUID) (bool, error) {
	return f.hasCapability, f.err
}

var _ auth.RoleChecker = fakeRoleChecker{}

func newTestForSaleForPublish(sellerID uuid.UUID) *entity.ForSale {
	return &entity.ForSale{
		ID:         uuid.New(),
		SellerID:   sellerID,
		Status:     entity.ForSaleStatusDraft,
		Visibility: entity.ForSaleVisibilityPrivate,
	}
}

// TestCheckMarketAuthorityForForSale_RejectsSellerWithoutCapability
// proves an unauthorized seller (no active subscription / seller capability)
// is rejected with auth.ErrMarketAuthorityRequired.
func TestCheckMarketAuthorityForForSale_RejectsSellerWithoutCapability(t *testing.T) {
	svc := &ForSaleService{
		roleChecker: fakeRoleChecker{hasCapability: false},
	}

	err := svc.CheckMarketAuthorityForForSale(context.Background(), uuid.New())

	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrMarketAuthorityRequired))
}

// TestCheckMarketAuthorityForForSale_AllowsSellerWithCapability proves
// an authorized seller (active subscription / seller capability) passes.
func TestCheckMarketAuthorityForForSale_AllowsSellerWithCapability(t *testing.T) {
	svc := &ForSaleService{
		roleChecker: fakeRoleChecker{hasCapability: true},
	}

	err := svc.CheckMarketAuthorityForForSale(context.Background(), uuid.New())

	assert.NoError(t, err)
}

// TestCheckMarketAuthorityForForSale_PropagatesCheckerError proves a
// role-checker transport error is surfaced (fail-closed), not swallowed as
// "no capability".
func TestCheckMarketAuthorityForForSale_PropagatesCheckerError(t *testing.T) {
	svc := &ForSaleService{
		roleChecker: fakeRoleChecker{err: errors.New("db unavailable")},
	}

	err := svc.CheckMarketAuthorityForForSale(context.Background(), uuid.New())

	require.Error(t, err)
	assert.False(t, errors.Is(err, auth.ErrMarketAuthorityRequired), "transport errors must not be reported as a market-authority denial")
}

// TestPublishFlow_EntityBecomesActivePublicOnlyAfterAuthorityPasses composes
// the exact sequence the fixed handler now uses (authority check, then
// entity.Publish()) and proves:
//  1. An unauthorized seller's for_sale never transitions to active/public —
//     the entity is untouched because the caller must check authority BEFORE
//     calling Publish().
//  2. An authorized seller's for_sale transitions to active + public only
//     after CheckMarketAuthorityForForSale succeeds.
func TestPublishFlow_EntityBecomesActivePublicOnlyAfterAuthorityPasses(t *testing.T) {
	sellerID := uuid.New()

	t.Run("unauthorized seller: authority check fails, Publish is never reached", func(t *testing.T) {
		svc := &ForSaleService{roleChecker: fakeRoleChecker{hasCapability: false}}
		for_sale := newTestForSaleForPublish(sellerID)

		err := svc.CheckMarketAuthorityForForSale(context.Background(), sellerID)
		require.Error(t, err)

		// Caller must not proceed to Publish() when the authority check fails —
		// assert the for_sale remains in its pre-publish state.
		assert.Equal(t, entity.ForSaleStatusDraft, for_sale.Status)
		assert.Equal(t, entity.ForSaleVisibilityPrivate, for_sale.Visibility)
	})

	t.Run("authorized seller: authority check passes, Publish sets active+public", func(t *testing.T) {
		svc := &ForSaleService{roleChecker: fakeRoleChecker{hasCapability: true}}
		for_sale := newTestForSaleForPublish(sellerID)

		err := svc.CheckMarketAuthorityForForSale(context.Background(), sellerID)
		require.NoError(t, err)

		require.NoError(t, for_sale.Publish())
		assert.Equal(t, entity.ForSaleStatusActive, for_sale.Status)
		assert.Equal(t, entity.ForSaleVisibilityPublic, for_sale.Visibility)
	})
}
