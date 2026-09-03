package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	addressEntity "github.com/labuda/backend/internal/identity/address/entity"
	addressRepo "github.com/labuda/backend/internal/identity/address/infrastructure/repository"
	addressRepoInterface "github.com/labuda/backend/internal/identity/address/repository"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// AddressService handles address business logic.
//
// IMPORTANT DESIGN NOTES:
// - Order must store address snapshot, NOT address_id
// - Address is soft-deleted (is_available_for_checkout = false)
// - Only one primary address per user is allowed
type AddressService struct {
	repo addressRepoInterface.AddressRepository
	log  *zap.Logger
}

// NewAddressService creates a new AddressService.
func NewAddressService() *AddressService {
	return &AddressService{
		repo: addressRepo.NewAddressRepository(),
		log:  zap.NewNop(),
	}
}

// SetLogger sets the logger for the service.
func (s *AddressService) SetLogger(log *zap.Logger) {
	s.log = log
}

// ============================================================================
// INPUT TYPES
// ============================================================================

// CreateAddressInput contains parameters for creating an address.
type CreateAddressInput struct {
	UserID   uuid.UUID
	Purpose  string // "shipping" or "sender"
	Nickname string

	RecipientName string
	Phone         string

	ProvinceID   string
	ProvinceName string
	CityID       string
	CityName     string
	DistrictID   string
	DistrictName string
	VillageID    string
	VillageName  string

	StreetAddress string
	PostalCode    string

	Latitude  *float64
	Longitude *float64

	Notes string

	IsPrimary bool
}

// UpdateAddressInput contains parameters for updating an address.
type UpdateAddressInput struct {
	AddressID uuid.UUID
	UserID    uuid.UUID

	Purpose  string
	Nickname string

	RecipientName string
	Phone         string

	ProvinceID   string
	ProvinceName string
	CityID       string
	CityName     string
	DistrictID   string
	DistrictName string
	VillageID    string
	VillageName  string

	StreetAddress string
	PostalCode    string

	Latitude  *float64
	Longitude *float64

	Notes string
}

// ============================================================================
// CREATE ADDRESS
// ============================================================================

// CreateAddress creates a new address for a user.
//
// Validation:
// - Purpose must be valid ("shipping" or "sender")
// - Required fields are present
// - If is_primary is true, unsets existing primary address
func (s *AddressService) CreateAddress(
	ctx context.Context,
	tx db.Tx,
	input CreateAddressInput,
) (*addressEntity.Address, error) {
	// Parse purpose
	purpose := addressEntity.AddressPurpose(input.Purpose)

	// Create the address entity
	address, err := addressEntity.NewAddress(
		input.UserID,
		purpose,
		input.Nickname,
		input.RecipientName,
		input.Phone,
		input.ProvinceID,
		input.ProvinceName,
		input.CityID,
		input.CityName,
		input.DistrictID,
		input.DistrictName,
		input.VillageID,
		input.VillageName,
		input.StreetAddress,
		input.PostalCode,
		input.Latitude,
		input.Longitude,
		input.Notes,
		input.IsPrimary,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create address entity: %w", err)
	}

	// If setting as primary, unset existing primary addresses
	if input.IsPrimary {
		if err := s.repo.UnsetAllPrimary(ctx, tx, input.UserID); err != nil {
			return nil, fmt.Errorf("failed to unset existing primary: %w", err)
		}
	}

	// Persist the address
	if err := s.repo.Create(ctx, tx, address); err != nil {
		return nil, fmt.Errorf("failed to persist address: %w", err)
	}

	s.log.Info("Address created",
		zap.String("address_id", address.ID.String()),
		zap.String("user_id", input.UserID.String()),
		zap.String("purpose", input.Purpose),
		zap.Bool("is_primary", input.IsPrimary),
	)

	return address, nil
}

// ============================================================================
// GET ADDRESS
// ============================================================================

// GetAddress retrieves an address by ID.
// Validates that the user owns the address.
func (s *AddressService) GetAddress(
	ctx context.Context,
	tx db.Tx,
	addressID uuid.UUID,
	userID uuid.UUID,
) (*addressEntity.Address, error) {
	address, err := s.repo.GetByID(ctx, tx, addressID)
	if err != nil {
		return nil, err
	}

	// Verify ownership
	if address.UserID != userID {
		return nil, &addressEntity.AddressNotOwnedError{
			AddressID: addressID,
			UserID:    userID,
		}
	}

	return address, nil
}

// ============================================================================
// LIST USER ADDRESSES
// ============================================================================

// ListUserAddresses retrieves all addresses for a user.
func (s *AddressService) ListUserAddresses(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) ([]*addressEntity.Address, error) {
	addresses, err := s.repo.GetByUserID(ctx, tx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list user addresses: %w", err)
	}

	return addresses, nil
}

// ListUserAddressesFiltered retrieves addresses for a user filtered by purpose.
func (s *AddressService) ListUserAddressesFiltered(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	purpose string,
) ([]*addressEntity.Address, error) {
	addresses, err := s.repo.GetByUserIDFiltered(ctx, tx, userID, purpose)
	if err != nil {
		return nil, fmt.Errorf("failed to list user addresses filtered: %w", err)
	}

	return addresses, nil
}

// ============================================================================
// UPDATE ADDRESS
// ============================================================================

// UpdateAddress updates an existing address.
//
// Validation:
// - Address exists and is owned by the user
// - If updating to primary, unsets existing primary addresses
func (s *AddressService) UpdateAddress(
	ctx context.Context,
	tx db.Tx,
	input UpdateAddressInput,
) (*addressEntity.Address, error) {
	// Get the existing address with lock
	address, err := s.repo.GetForUpdate(ctx, tx, input.AddressID)
	if err != nil {
		return nil, fmt.Errorf("address not found: %w", err)
	}

	// Verify ownership
	if address.UserID != input.UserID {
		return nil, &addressEntity.AddressNotOwnedError{
			AddressID: input.AddressID,
			UserID:    input.UserID,
		}
	}

	// Update fields
	address.Purpose = addressEntity.AddressPurpose(input.Purpose)
	address.Nickname = input.Nickname
	address.RecipientName = input.RecipientName
	address.Phone = input.Phone
	address.ProvinceID = input.ProvinceID
	address.ProvinceName = input.ProvinceName
	address.CityID = input.CityID
	address.CityName = input.CityName
	address.DistrictID = input.DistrictID
	address.DistrictName = input.DistrictName
	address.VillageID = input.VillageID
	address.VillageName = input.VillageName
	address.StreetAddress = input.StreetAddress
	address.PostalCode = input.PostalCode
	address.Latitude = input.Latitude
	address.Longitude = input.Longitude
	address.Notes = input.Notes
	address.UpdatedAt = time.Now()

	// If setting as primary, unset other primary addresses
	if address.IsPrimary {
		if err := s.repo.UnsetAllPrimary(ctx, tx, input.UserID); err != nil {
			return nil, fmt.Errorf("failed to unset existing primary: %w", err)
		}
	}

	// Persist changes
	if err := s.repo.Update(ctx, tx, address); err != nil {
		return nil, fmt.Errorf("failed to update address: %w", err)
	}

	s.log.Info("Address updated",
		zap.String("address_id", address.ID.String()),
		zap.String("user_id", input.UserID.String()),
	)

	return address, nil
}

// ============================================================================
// DELETE ADDRESS
// ============================================================================

// DeleteAddress soft-deletes an address (marks as unavailable for checkout).
//
// Validation:
// - Address exists and is owned by the user
func (s *AddressService) DeleteAddress(
	ctx context.Context,
	tx db.Tx,
	addressID uuid.UUID,
	userID uuid.UUID,
) error {
	// Verify ownership first
	address, err := s.repo.GetByID(ctx, tx, addressID)
	if err != nil {
		return fmt.Errorf("address not found: %w", err)
	}

	if address.UserID != userID {
		return &addressEntity.AddressNotOwnedError{
			AddressID: addressID,
			UserID:    userID,
		}
	}

	// Soft delete
	if err := s.repo.Delete(ctx, tx, addressID); err != nil {
		return fmt.Errorf("failed to delete address: %w", err)
	}

	s.log.Info("Address deleted",
		zap.String("address_id", addressID.String()),
		zap.String("user_id", userID.String()),
	)

	return nil
}

// ============================================================================
// SET PRIMARY
// ============================================================================

// SetPrimary sets an address as the primary address for the user.
//
// Validation:
// - Address exists and is owned by the user
// - Unsets all other primary addresses for the user
func (s *AddressService) SetPrimary(
	ctx context.Context,
	tx db.Tx,
	addressID uuid.UUID,
	userID uuid.UUID,
) error {
	// Verify ownership first
	address, err := s.repo.GetByID(ctx, tx, addressID)
	if err != nil {
		return fmt.Errorf("address not found: %w", err)
	}

	if address.UserID != userID {
		return &addressEntity.AddressNotOwnedError{
			AddressID: addressID,
			UserID:    userID,
		}
	}

	// Set as primary (also unsets other primary addresses)
	if err := s.repo.SetPrimary(ctx, tx, addressID); err != nil {
		return fmt.Errorf("failed to set primary address: %w", err)
	}

	s.log.Info("Primary address set",
		zap.String("address_id", addressID.String()),
		zap.String("user_id", userID.String()),
	)

	return nil
}

// ============================================================================
// COUNT
// ============================================================================

// CountByUserID returns address counts grouped by purpose.
func (s *AddressService) CountByUserID(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) (*addressRepoInterface.AddressCount, error) {
	return s.repo.CountByUserID(ctx, tx, userID)
}

// ============================================================================
// GET PRIMARY FILTERED
// ============================================================================

// GetPrimaryFiltered retrieves the user's primary address filtered by purpose.
func (s *AddressService) GetPrimaryFiltered(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	purpose string,
) (*addressEntity.Address, error) {
	return s.repo.GetPrimaryByUserIDFiltered(ctx, tx, userID, purpose)
}

// ============================================================================
// CHECKOUT INTEGRATION
// ============================================================================

// GetAddressForCheckout retrieves an address for use during checkout.
//
// Validation:
// - Address exists and is owned by the user
// - Address is available for checkout (is_available_for_checkout = true)
func (s *AddressService) GetAddressForCheckout(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	addressID uuid.UUID,
) (*addressEntity.Address, error) {
	address, err := s.repo.GetByID(ctx, tx, addressID)
	if err != nil {
		return nil, fmt.Errorf("address not found: %w", err)
	}

	// Verify ownership
	if address.UserID != userID {
		return nil, &addressEntity.AddressNotOwnedError{
			AddressID: addressID,
			UserID:    userID,
		}
	}

	// Check if available for checkout
	if !address.CanBeUsedForCheckout() {
		return nil, &addressEntity.AddressUnavailableError{ID: addressID}
	}

	return address, nil
}

// GetPrimaryAddressForCheckout retrieves the user's primary address for checkout.
// Returns nil if no primary address is set.
func (s *AddressService) GetPrimaryAddressForCheckout(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) (*addressEntity.Address, error) {
	address, err := s.repo.GetPrimaryByUserID(ctx, tx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get primary address: %w", err)
	}

	// No primary address set
	if address == nil {
		return nil, nil
	}

	// Check if available for checkout
	if !address.CanBeUsedForCheckout() {
		return nil, nil // Primary address exists but not available
	}

	return address, nil
}

// ============================================================================
// ERROR HELPERS
// ============================================================================

// IsAddressNotOwnedError checks if an error is an AddressNotOwnedError.
func IsAddressNotOwnedError(err error) bool {
	var notOwnedErr *addressEntity.AddressNotOwnedError
	return errors.As(err, &notOwnedErr)
}

// IsAddressUnavailableError checks if an error is an AddressUnavailableError.
func IsAddressUnavailableError(err error) bool {
	var unavailableErr *addressEntity.AddressUnavailableError
	return errors.As(err, &unavailableErr)
}

// IsAddressNotFoundError checks if an error is an AddressNotFoundError.
func IsAddressNotFoundError(err error) bool {
	var notFoundErr *addressEntity.AddressNotFoundError
	return errors.As(err, &notFoundErr)
}


