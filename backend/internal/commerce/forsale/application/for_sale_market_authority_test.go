package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/hishumi/backend/internal/commerce/forsale/entity"
	"github.com/hishumi/backend/internal/identity/auth"
	money "github.com/hishumi/backend/pkg/money"
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

// TestCreateGate_AuthorityCheckedBeforeSurfaceIsBorn locks the create =
// publish sequence: the market-authority gate runs BEFORE any for_sale
// surface exists (there is no draft workspace stage to fall back to), and a
// passing gate is what allows the constructor-produced active + public
// surface to be persisted (covered by the constructor test in the canonical
// authority suite).
func TestCreateGate_AuthorityCheckedBeforeSurfaceIsBorn(t *testing.T) {
	sellerID := uuid.New()

	t.Run("unauthorized seller: authority check fails before any surface exists", func(t *testing.T) {
		svc := &ForSaleService{roleChecker: fakeRoleChecker{hasCapability: false}}

		err := svc.CheckMarketAuthorityForForSale(context.Background(), sellerID)
		require.Error(t, err)
		assert.ErrorIs(t, err, auth.ErrMarketAuthorityRequired)
	})

	t.Run("authorized seller: authority check passes, constructor yields active + public", func(t *testing.T) {
		svc := &ForSaleService{roleChecker: fakeRoleChecker{hasCapability: true}}
		require.NoError(t, svc.CheckMarketAuthorityForForSale(context.Background(), sellerID))

		for_sale, err := entity.NewForSaleSurface(sellerID, money.New(100000), 1, false)
		require.NoError(t, err)
		assert.Equal(t, entity.ForSaleStatusActive, for_sale.Status)
		assert.Equal(t, entity.ForSaleVisibilityPublic, for_sale.Visibility)
	})
}
