package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
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
	Tags     []string // non-empty subset of entity.AllAddressTags
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

	Tags     []string
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
// - Tags must be a non-empty subset of "shipping"/"sender"
// - Required fields are present
// - If is_primary is true, unsets existing primary address
func (s *AddressService) CreateAddress(
	ctx context.Context,
	tx db.Tx,
	input CreateAddressInput,
) (*addressEntity.Address, error) {
	// Validate + canonically order tags
	tags, err := addressEntity.NormalizeTags(input.Tags)
	if err != nil {
		return nil, err
	}

	// Reconciler, count==1 half (contract A2), applied BEFORE persisting so
	// the response matches the stored row: an account's first address is its
	// everything — both roles and the primary flag.
	tags, isPrimary, err := s.enforceSingleAddressRule(ctx, tx, input.UserID, uuid.Nil, tags, input.IsPrimary)
	if err != nil {
		return nil, err
	}

	// Create the address entity
	address, err := addressEntity.NewAddress(
		input.UserID,
		tags,
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
		isPrimary,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create address entity: %w", err)
	}

	// If setting as primary, unset existing primary addresses
	if isPrimary {
		if err := s.repo.UnsetAllPrimary(ctx, tx, input.UserID); err != nil {
			return nil, fmt.Errorf("failed to unset existing primary: %w", err)
		}
	}

	// Persist the address. Losing the first-address race means another
	// create committed the primary between our check and our insert —
	// recreate as non-primary; the reconcile below keeps the count at one.
	if err := s.repo.Create(ctx, tx, address); err != nil {
		if isPrimary && isPrimaryUniqueViolation(err) {
			address.IsPrimary = false
			if retryErr := s.repo.Create(ctx, tx, address); retryErr != nil {
				return nil, fmt.Errorf("failed to persist address: %w", retryErr)
			}
		} else {
			return nil, fmt.Errorf("failed to persist address: %w", err)
		}
	}

	s.log.Info("Address created",
		zap.String("address_id", address.ID.String()),
		zap.String("user_id", input.UserID.String()),
		zap.Strings("tags", address.TagStrings()),
		zap.Bool("is_primary", isPrimary),
	)

	if err := s.reconcile(ctx, tx, input.UserID); err != nil {
		return nil, err
	}

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

// ListUserAddressesFiltered retrieves addresses for a user carrying the tag.
func (s *AddressService) ListUserAddressesFiltered(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	tag string,
) ([]*addressEntity.Address, error) {
	addresses, err := s.repo.GetByUserIDFiltered(ctx, tx, userID, tag)
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
	tags, err := addressEntity.NormalizeTags(input.Tags)
	if err != nil {
		return nil, err
	}
	address.Tags = tags
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

	// Reconciler, count==1 half (contract A2): the sole address of an
	// account is its everything — both roles and the primary flag.
	address.Tags, address.IsPrimary, err = s.enforceSingleAddressRule(
		ctx, tx, input.UserID, input.AddressID, address.Tags, address.IsPrimary,
	)
	if err != nil {
		return nil, err
	}

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

	if err := s.reconcile(ctx, tx, input.UserID); err != nil {
		return nil, err
	}

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

	return s.reconcile(ctx, tx, userID)
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

	return s.reconcile(ctx, tx, userID)
}

// ============================================================================
// RECONCILER (contract A2 — the ONE write-side invariant)
// ============================================================================

// reconcile runs after every write to the address book:
//
//	0 active addresses -> nothing is forced (an address book is optional)
//	1 active address    -> both tags + primary (the account's everything)
//	2+ active addresses -> tags are the user's choice, but EXACTLY ONE
//	                      primary exists (the oldest is promoted when none
//	                      is flagged)
//
// It lives here — never in a UI — so mobile, web and direct API writes
// cannot disagree about what an address book means. The read side has its
// own law (tag fallback in the repository): reads degrade gracefully, this
// write side keeps the data coherent.
func (s *AddressService) reconcile(ctx context.Context, tx db.Tx, userID uuid.UUID) error {
	addresses, err := s.repo.GetByUserID(ctx, tx, userID)
	if err != nil {
		return fmt.Errorf("failed to reconcile addresses: %w", err)
	}

	switch len(addresses) {
	case 0:
		// Optional address book: nothing to force.
		return nil

	case 1:
		only := addresses[0]
		needsTags := !only.HasTag(addressEntity.TagShipping) ||
			!only.HasTag(addressEntity.TagSender)
		if needsTags {
			only.Tags = []addressEntity.AddressTag{
				addressEntity.TagShipping,
				addressEntity.TagSender,
			}
		}
		if !only.IsPrimary {
			if err := s.repo.SetPrimary(ctx, tx, only.ID); err != nil {
				return fmt.Errorf("failed to reconcile primary address: %w", err)
			}
			only.IsPrimary = true
		}
		if needsTags {
			if err := s.repo.Update(ctx, tx, only); err != nil {
				return fmt.Errorf("failed to reconcile address tags: %w", err)
			}
		}
		return nil

	default:
		// ≥2 addresses: tags stay the user's choice.
		for _, address := range addresses {
			if address.IsPrimary {
				return nil // exactly one primary (the unique index caps it)
			}
		}
		// No primary flagged: promote the oldest active address.
		oldest := addresses[0]
		for _, address := range addresses[1:] {
			if address.CreatedAt.Before(oldest.CreatedAt) {
				oldest = address
			}
		}
		if err := s.repo.SetPrimary(ctx, tx, oldest.ID); err != nil {
			return fmt.Errorf("failed to promote primary address: %w", err)
		}
		return nil
	}
}

// enforceSingleAddressRule applies the count==1 half of the reconciler to a
// row that is ABOUT to be written: when userID owns no other active address,
// the row becomes the account's everything — both tags and the primary flag.
// existingID is the row being edited (uuid.Nil on create).
//
// Applied before persisting (not only after) so the response the caller
// receives already matches the stored row.
func (s *AddressService) enforceSingleAddressRule(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	existingID uuid.UUID,
	tags []addressEntity.AddressTag,
	isPrimary bool,
) ([]addressEntity.AddressTag, bool, error) {
	active, err := s.repo.GetByUserID(ctx, tx, userID)
	if err != nil {
		return nil, false, fmt.Errorf("failed to list active addresses: %w", err)
	}

	for _, address := range active {
		if address.ID != existingID {
			return tags, isPrimary, nil // ≥2: the user's tags, the user's choice
		}
	}

	return []addressEntity.AddressTag{
		addressEntity.TagShipping,
		addressEntity.TagSender,
	}, true, nil
}

// ============================================================================
// COUNT
// ============================================================================

// CountByUserID returns address counts grouped by tag.
func (s *AddressService) CountByUserID(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) (*addressRepoInterface.AddressCount, error) {
	return s.repo.CountByUserID(ctx, tx, userID)
}

// ============================================================================
// GET PRIMARY BY TAG
// ============================================================================

// GetPrimaryFiltered retrieves the account's primary address, narrowed to
// rows carrying the given tag. The primary flag is account-wide — there is
// exactly one per account, never one per tag.
func (s *AddressService) GetPrimaryFiltered(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	tag string,
) (*addressEntity.Address, error) {
	return s.repo.GetPrimaryByTag(ctx, tx, userID, tag)
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

// isPrimaryUniqueViolation reports whether err is the unique-index rejection
// of a second active primary (idx_addresses_user_active_primary_unique).
func isPrimaryUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
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


