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

// checkoutFakeRoleChecker is a minimal auth.RoleChecker stub controlling only
// HasActiveSellerCapability, which is all Guard 6 (market authority) consumes.
type checkoutFakeRoleChecker struct {
	hasCapability bool
}

func (f checkoutFakeRoleChecker) IsAdmin(context.Context, uuid.UUID) (bool, error) {
	return false, nil
}
func (f checkoutFakeRoleChecker) IsSeller(context.Context, uuid.UUID) (bool, error) {
	return f.hasCapability, nil
}
func (f checkoutFakeRoleChecker) HasActiveSellerCapability(context.Context, uuid.UUID) (bool, error) {
	return f.hasCapability, nil
}
func (f checkoutFakeRoleChecker) HasSellerProfile(context.Context, uuid.UUID) (bool, error) {
	return f.hasCapability, nil
}

var _ auth.RoleChecker = checkoutFakeRoleChecker{}

func newPublicActiveForSaleForCheckout(sellerID uuid.UUID) *entity.ForSale {
	return &entity.ForSale{
		ID:                uuid.New(),
		SellerID:          sellerID,
		Status:            entity.ForSaleStatusActive,
		Visibility:        entity.ForSaleVisibilityPublic,
		QuantityAvailable: 5,
		ForSaleType:       entity.ForSaleTypeFixedPrice,
	}
}

// TestValidateSaleSurfaceForCheckout_Guard6_BlocksSellerWithoutMarketAuthority
// is the PASS_18O regression proof that Guard 6 (defense-in-depth) still
// blocks checkout against a public/active for_sale whose seller has since
// lost market authority (expired subscription, suspended verification,
// etc.) — even if a for_sale somehow reached public/active state without a
// valid authority check (the exact PASS_18M bypass this pass closes at the
// publish boundary). This guard must remain in place unweakened.
func TestValidateSaleSurfaceForCheckout_Guard6_BlocksSellerWithoutMarketAuthority(t *testing.T) {
	sellerID := uuid.New()
	buyerID := uuid.New()
	svc := &OrderCreationService{
		roleChecker: checkoutFakeRoleChecker{hasCapability: false},
	}
	forSale := newPublicActiveForSaleForCheckout(sellerID)

	err := svc.validateSaleSurfaceForCheckout(context.Background(), ValidateSaleSurfaceForCheckoutInput{
		SaleSurface: forSale,
		BuyerID:     buyerID,
		Quantity:    1,
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrMarketAuthorityRequired))
}

// TestValidateSaleSurfaceForCheckout_Guard6_AllowsSellerWithMarketAuthority
// proves checkout proceeds normally for a public/active for_sale whose seller
// currently holds market authority.
func TestValidateSaleSurfaceForCheckout_Guard6_AllowsSellerWithMarketAuthority(t *testing.T) {
	sellerID := uuid.New()
	buyerID := uuid.New()
	svc := &OrderCreationService{
		roleChecker: checkoutFakeRoleChecker{hasCapability: true},
	}
	forSale := newPublicActiveForSaleForCheckout(sellerID)

	err := svc.validateSaleSurfaceForCheckout(context.Background(), ValidateSaleSurfaceForCheckoutInput{
		SaleSurface: forSale,
		BuyerID:     buyerID,
		Quantity:    1,
	})

	assert.NoError(t, err)
}
