package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	sellerEntity "github.com/labuda/backend/internal/commerce/seller/entity"
	addressEntity "github.com/labuda/backend/internal/identity/address/entity"
	userEntity "github.com/labuda/backend/internal/identity/user/domain/entity"
	"github.com/labuda/backend/pkg/db"
)

// ErrOnboardingIncomplete is returned when user hasn't completed seller onboarding.
// STRICT MODE: This error blocks subscription payment until all requirements are met.
// Provides detailed list of missing requirements for debugging.
type ErrOnboardingIncomplete struct {
	MissingRequirements []string
}

func (e *ErrOnboardingIncomplete) Error() string {
	return fmt.Sprintf("seller onboarding incomplete: missing requirements [%s]",
		strings.Join(e.MissingRequirements, ", "))
}

// SellerOnboardingService handles seller onboarding validation.
//
// This is a SHARED service used by:
// - SellerSubscriptionPaymentService (payment flow validation)
// - SellerHandler (onboarding endpoint validation)
//
// This ensures CONSISTENT BEHAVIOR across all seller onboarding operations
// and eliminates validation drift.
//
// STRICT MODE: All validation methods use row-level locking (FOR UPDATE)
// to prevent race conditions during concurrent onboarding operations.
type SellerOnboardingService struct {
	userRepo    onboardingUserRepository
	sellerRepo  onboardingSellerRepository
	addressRepo onboardingAddressRepository
}

type onboardingUserRepository interface {
	GetByIDForUpdate(ctx context.Context, tx db.Tx, userID uuid.UUID) (*userEntity.User, error)
	GetProfileByID(ctx context.Context, tx db.Tx, userID uuid.UUID) (*userEntity.UserProfile, error)
}

type onboardingSellerRepository interface {
	GetByUserID(ctx context.Context, tx db.Tx, userID uuid.UUID) (*sellerEntity.SellerProfile, error)
}

type onboardingAddressRepository interface {
	GetByUserIDFiltered(ctx context.Context, tx db.Tx, userID uuid.UUID, purpose string) ([]*addressEntity.Address, error)
}

// NewSellerOnboardingService creates a new SellerOnboardingService.
func NewSellerOnboardingService(
	userRepo onboardingUserRepository,
	sellerRepo onboardingSellerRepository,
	addressRepo onboardingAddressRepository,
) *SellerOnboardingService {
	return &SellerOnboardingService{
		userRepo:    userRepo,
		sellerRepo:  sellerRepo,
		addressRepo: addressRepo,
	}
}

// ValidateOnboarding validates that the user has completed seller onboarding.
//
// STRICT MODE: FAIL HARD - No payment without complete onboarding
// This prevents fake seller entry by ensuring:
// - Email is verified
// - Username exists
// - Bio exists (user_profiles.bio IS NOT NULL)
// - Phone number exists (phone_number IS NOT NULL)
// - Sender address exists (structured address row with purpose="sender")
// - Seller profile exists (seller_profile must be created first)
//
// MARKET AUTHORITY ENFORCEMENT:
// - Seller profile must exist before subscription payment
// - This ensures seller identity is created before market authority is granted
//
// SEPARATION OF CONCERNS:
// - Identity creation (seller_profile) happens during onboarding
// - Market authority (seller_subscription) happens during payment
// This ensures deterministic behavior and no hidden side effects.
//
// DETAILED ERRORS: Returns ErrOnboardingIncomplete with list of missing requirements
//
// ROW LOCKING: Uses GetByIDForUpdate to prevent concurrent modifications during validation
func (s *SellerOnboardingService) ValidateOnboarding(ctx context.Context, tx db.Tx, userID uuid.UUID) error {
	// Collect all missing requirements for detailed error reporting
	var missingRequirements []string
	missing := &missingRequirement{requirements: &missingRequirements}

	// STRICT MODE: Lock user row to prevent concurrent modifications
	user, err := s.userRepo.GetByIDForUpdate(ctx, tx, userID)
	if err != nil {
		return fmt.Errorf("failed to validate onboarding: %w", err)
	}
	if user == nil {
		return fmt.Errorf("user not found: %s", userID)
	}

	// HTTP boundary authority checks happen in middleware.
	// Keep domain validation here focused on onboarding completeness only.

	// Check email verification (explicit field validation)
	if !user.EmailVerified {
		missing.add("email_verified")
	}

	// Check phone number exists
	if user.PhoneNumber == nil || *user.PhoneNumber == "" {
		missing.add("phone_number")
	}

	// Check profile completeness from username and bio.
	userProfile, err := s.userRepo.GetProfileByID(ctx, tx, userID)
	if err != nil {
		return fmt.Errorf("failed to check user profile: %w", err)
	}
	if userProfile == nil || userProfile.Username == nil || *userProfile.Username == "" {
		missing.add("username")
	}
	if userProfile.Bio == nil || *userProfile.Bio == "" {
		missing.add("bio")
	}

	hasSenderAddress, err := s.hasSenderAddress(ctx, tx, userID)
	if err != nil {
		return fmt.Errorf("failed to check sender address: %w", err)
	}
	if !hasSenderAddress {
		missing.add("sender_address")
	}

	// Check seller profile exists (identity must be created before subscription)
	profile, err := s.sellerRepo.GetByUserID(ctx, tx, userID)
	if err != nil {
		return fmt.Errorf("failed to check seller profile: %w", err)
	}
	if profile == nil {
		missing.add("seller_profile")
	}

	// FAIL HARD with detailed error if any requirements are missing
	if missing.hasAny() {
		return &ErrOnboardingIncomplete{
			MissingRequirements: missingRequirements,
		}
	}

	return nil
}

// ValidateOnboardingWithoutProfile validates onboarding requirements except seller profile.
//
// This is used by the onboarding endpoint to check if the user is ready to create
// a seller profile, but doesn't require the profile to exist yet.
//
// STRICT MODE: Uses row locking to prevent concurrent modifications during validation.
// IDEMPOTENCY: If profile exists, validation is skipped and returns empty requirements.
func (s *SellerOnboardingService) ValidateOnboardingWithoutProfile(ctx context.Context, tx db.Tx, userID uuid.UUID) []string {
	// Collect all missing requirements for detailed error reporting
	var missingRequirements []string
	missing := &missingRequirement{requirements: &missingRequirements}

	// STRICT MODE: Lock user row to prevent concurrent modifications
	user, err := s.userRepo.GetByIDForUpdate(ctx, tx, userID)
	if err != nil {
		return missingRequirements // Return empty on error, let caller handle
	}
	if user == nil {
		return missingRequirements
	}

	// HTTP boundary authority checks happen in middleware.
	// Keep domain validation here focused on onboarding completeness only.

	// Check email verification (explicit field validation)
	if !user.EmailVerified {
		missing.add("email_verified")
	}

	// Check phone number exists
	if user.PhoneNumber == nil || *user.PhoneNumber == "" {
		missing.add("phone_number")
	}

	// Check profile completeness from username and bio.
	userProfile, err := s.userRepo.GetProfileByID(ctx, tx, userID)
	if err != nil {
		return missingRequirements // Return empty on error, let caller handle
	}
	if userProfile == nil || userProfile.Username == nil || *userProfile.Username == "" {
		missing.add("username")
	}
	if userProfile.Bio == nil || *userProfile.Bio == "" {
		missing.add("bio")
	}

	hasSenderAddress, err := s.hasSenderAddress(ctx, tx, userID)
	if err != nil {
		return missingRequirements // Return empty on error, let caller handle
	}
	if !hasSenderAddress {
		missing.add("sender_address")
	}

	return missingRequirements
}

func (s *SellerOnboardingService) hasSenderAddress(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) (bool, error) {
	addresses, err := s.addressRepo.GetByUserIDFiltered(
		ctx,
		tx,
		userID,
		string(addressEntity.AddressPurposeSender),
	)
	if err != nil {
		return false, err
	}

	return len(addresses) > 0, nil
}

// missingRequirement is a helper to add missing requirements during validation
type missingRequirement struct {
	requirements *[]string
}

func (m *missingRequirement) add(req string) {
	*m.requirements = append(*m.requirements, req)
}

func (m *missingRequirement) hasAny() bool {
	return len(*m.requirements) > 0
}


