package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	sellerEntity "github.com/labuda/backend/internal/commerce/seller/entity"
	addressEntity "github.com/labuda/backend/internal/identity/address/entity"
	userEntity "github.com/labuda/backend/internal/identity/user/domain/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockOnboardingUserRepo struct {
	user    *userEntity.User
	profile *userEntity.UserProfile
	err     error
}

func (m *mockOnboardingUserRepo) GetByIDForUpdate(
	_ context.Context,
	_ db.Tx,
	_ uuid.UUID,
) (*userEntity.User, error) {
	return m.user, m.err
}

func (m *mockOnboardingUserRepo) GetProfileByID(
	_ context.Context,
	_ db.Tx,
	_ uuid.UUID,
) (*userEntity.UserProfile, error) {
	return m.profile, m.err
}

type mockOnboardingSellerRepo struct {
	profile *sellerEntity.SellerProfile
	err     error
}

func (m *mockOnboardingSellerRepo) GetByUserID(
	_ context.Context,
	_ db.Tx,
	_ uuid.UUID,
) (*sellerEntity.SellerProfile, error) {
	return m.profile, m.err
}

type mockOnboardingAddressRepo struct {
	primary *addressEntity.Address
	err     error
}

func (m *mockOnboardingAddressRepo) GetPrimaryByUserID(
	_ context.Context,
	_ db.Tx,
	_ uuid.UUID,
) (*addressEntity.Address, error) {
	return m.primary, m.err
}

func TestValidateOnboardingWithoutProfile_RequiresPrimaryAddress(t *testing.T) {
	svc := NewSellerOnboardingService(
		&mockOnboardingUserRepo{
			user: &userEntity.User{
				ID:            uuid.New(),
				EmailVerified: true,
				PhoneNumber:   strPtr("+628123456789"),
			},
			profile: &userEntity.UserProfile{
				Username: strPtr("seller-1"),
				Bio:      strPtr("bio"),
			},
		},
		&mockOnboardingSellerRepo{},
		&mockOnboardingAddressRepo{},
	)

	missing := svc.ValidateOnboardingWithoutProfile(context.Background(), nil, uuid.New())
	assert.Contains(t, missing, "primary_address")
	assert.NotContains(t, missing, "location")
}

func TestValidateOnboardingWithoutProfile_PassesWithPrimaryAddress(t *testing.T) {
	primaryAddress := &addressEntity.Address{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		IsPrimary: true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	svc := NewSellerOnboardingService(
		&mockOnboardingUserRepo{
			user: &userEntity.User{
				ID:            primaryAddress.UserID,
				EmailVerified: true,
				PhoneNumber:   strPtr("+628123456789"),
			},
			profile: &userEntity.UserProfile{
				Username: strPtr("seller-1"),
				Bio:      strPtr("bio"),
			},
		},
		&mockOnboardingSellerRepo{},
		&mockOnboardingAddressRepo{primary: primaryAddress},
	)

	missing := svc.ValidateOnboardingWithoutProfile(context.Background(), nil, primaryAddress.UserID)
	require.Empty(t, missing)
}

func TestValidateOnboarding_RequiresPrimaryAddress(t *testing.T) {
	userID := uuid.New()
	svc := NewSellerOnboardingService(
		&mockOnboardingUserRepo{
			user: &userEntity.User{
				ID:            userID,
				EmailVerified: true,
				PhoneNumber:   strPtr("+628123456789"),
			},
			profile: &userEntity.UserProfile{
				Username: strPtr("seller-1"),
				Bio:      strPtr("bio"),
			},
		},
		&mockOnboardingSellerRepo{
			profile: &sellerEntity.SellerProfile{
				ID:        uuid.New(),
				UserID:    userID,
				StoreName: "Store",
			},
		},
		&mockOnboardingAddressRepo{},
	)

	err := svc.ValidateOnboarding(context.Background(), nil, userID)
	require.Error(t, err)

	var onboardingErr *ErrOnboardingIncomplete
	require.True(t, errors.As(err, &onboardingErr))
	assert.Contains(t, onboardingErr.MissingRequirements, "primary_address")
	assert.NotContains(t, onboardingErr.MissingRequirements, "location")
}

func strPtr(v string) *string {
	return &v
}


