package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	addressEntity "github.com/hishumi/backend/internal/identity/address/entity"
	addressRepo "github.com/hishumi/backend/internal/identity/address/repository"
	"github.com/hishumi/backend/pkg/db"
)

// AddressRepositoryImpl handles address persistence using pgx-based DB layer.
type AddressRepositoryImpl struct{}

// NewAddressRepository creates a new AddressRepositoryImpl.
func NewAddressRepository() *AddressRepositoryImpl {
	return &AddressRepositoryImpl{}
}

// addressColumns is the canonical column list for SELECT queries.
const addressColumns = `id, user_id, nickname,
	recipient_name, phone,
	province_id, province_name,
	city_id, city_name,
	district_id, district_name,
	village_id, village_name,
	street_address, postal_code,
	latitude, longitude, notes,
	is_primary, is_available_for_checkout,
	created_at, updated_at`

// Create persists a new address within a transaction.
func (r *AddressRepositoryImpl) Create(
	ctx context.Context,
	tx db.Tx,
	address *addressEntity.Address,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO addresses (
			id, user_id, nickname,
			recipient_name, phone,
			province_id, province_name,
			city_id, city_name,
			district_id, district_name,
			village_id, village_name,
			street_address, postal_code,
			latitude, longitude, notes,
			is_primary, is_available_for_checkout,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)
	`,
		address.ID,
		address.UserID,
		address.Nickname,
		address.RecipientName,
		address.Phone,
		address.ProvinceID,
		address.ProvinceName,
		address.CityID,
		address.CityName,
		address.DistrictID,
		address.DistrictName,
		address.VillageID,
		address.VillageName,
		address.StreetAddress,
		address.PostalCode,
		address.Latitude,
		address.Longitude,
		address.Notes,
		address.IsPrimary,
		address.IsAvailableForCheckout,
		address.CreatedAt,
		address.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("create address failed: %w", err)
	}

	return nil
}

// GetByID retrieves an address without locking (for read-only operations).
func (r *AddressRepositoryImpl) GetByID(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (*addressEntity.Address, error) {
	var address addressEntity.Address
	err := tx.QueryRow(ctx, `
		SELECT `+addressColumns+`
		FROM addresses
		WHERE id = $1
	`, id).Scan(
		&address.ID,
		&address.UserID,
		&address.Nickname,
		&address.RecipientName,
		&address.Phone,
		&address.ProvinceID,
		&address.ProvinceName,
		&address.CityID,
		&address.CityName,
		&address.DistrictID,
		&address.DistrictName,
		&address.VillageID,
		&address.VillageName,
		&address.StreetAddress,
		&address.PostalCode,
		&address.Latitude,
		&address.Longitude,
		&address.Notes,
		&address.IsPrimary,
		&address.IsAvailableForCheckout,
		&address.CreatedAt,
		&address.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("address not found: %s", id)
		}
		return nil, fmt.Errorf("get address failed: %w", err)
	}


	return &address, nil
}

// GetForUpdate retrieves an address with FOR UPDATE lock.
func (r *AddressRepositoryImpl) GetForUpdate(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (*addressEntity.Address, error) {
	var address addressEntity.Address
	err := tx.QueryRow(ctx, `
		SELECT `+addressColumns+`
		FROM addresses
		WHERE id = $1
		FOR UPDATE
	`, id).Scan(
		&address.ID,
		&address.UserID,
		&address.Nickname,
		&address.RecipientName,
		&address.Phone,
		&address.ProvinceID,
		&address.ProvinceName,
		&address.CityID,
		&address.CityName,
		&address.DistrictID,
		&address.DistrictName,
		&address.VillageID,
		&address.VillageName,
		&address.StreetAddress,
		&address.PostalCode,
		&address.Latitude,
		&address.Longitude,
		&address.Notes,
		&address.IsPrimary,
		&address.IsAvailableForCheckout,
		&address.CreatedAt,
		&address.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("address not found: %s", id)
		}
		return nil, fmt.Errorf("get address for update failed: %w", err)
	}


	return &address, nil
}

// Update persists changes to an address within a transaction.
func (r *AddressRepositoryImpl) Update(
	ctx context.Context,
	tx db.Tx,
	address *addressEntity.Address,
) error {
	_, err := tx.Exec(ctx, `
		UPDATE addresses
		SET nickname = $2,
		    recipient_name = $3,
		    phone = $4,
		    province_id = $5,
		    province_name = $6,
		    city_id = $7,
		    city_name = $8,
		    district_id = $9,
		    district_name = $10,
		    village_id = $11,
		    village_name = $12,
		    street_address = $13,
		    postal_code = $14,
		    latitude = $15,
		    longitude = $16,
		    notes = $17,
		    is_primary = $18,
		    is_available_for_checkout = $19,
		    updated_at = $20
		WHERE id = $1
	`,
		address.ID,
		address.Nickname,
		address.RecipientName,
		address.Phone,
		address.ProvinceID,
		address.ProvinceName,
		address.CityID,
		address.CityName,
		address.DistrictID,
		address.DistrictName,
		address.VillageID,
		address.VillageName,
		address.StreetAddress,
		address.PostalCode,
		address.Latitude,
		address.Longitude,
		address.Notes,
		address.IsPrimary,
		address.IsAvailableForCheckout,
		address.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("update address failed: %w", err)
	}

	return nil
}

// Delete soft-deletes an address by making it unavailable for checkout.
func (r *AddressRepositoryImpl) Delete(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) error {
	_, err := tx.Exec(ctx, `
		UPDATE addresses
		SET is_available_for_checkout = false,
		    updated_at = NOW()
		WHERE id = $1
	`, id)

	if err != nil {
		return fmt.Errorf("delete address failed: %w", err)
	}

	return nil
}

// GetByUserID retrieves all addresses for a user (read-only).
func (r *AddressRepositoryImpl) GetByUserID(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) ([]*addressEntity.Address, error) {
	rows, err := tx.Query(ctx, `
		SELECT `+addressColumns+`
		FROM addresses
		WHERE user_id = $1 AND is_available_for_checkout = true
		ORDER BY is_primary DESC, created_at DESC
	`, userID)

	if err != nil {
		return nil, fmt.Errorf("get addresses by user failed: %w", err)
	}
	defer rows.Close()

	return r.scanRows(rows)
}

// GetByUserIDForDisplay retrieves every address of a user for public display
// (profile origin line, seller card origin). No is_available_for_checkout
// filter — see the interface contract.
func (r *AddressRepositoryImpl) GetByUserIDForDisplay(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) ([]*addressEntity.Address, error) {
	rows, err := tx.Query(ctx, `
		SELECT `+addressColumns+`
		FROM addresses
		WHERE user_id = $1
		ORDER BY is_primary DESC, created_at DESC
	`, userID)

	if err != nil {
		return nil, fmt.Errorf("get addresses for display failed: %w", err)
	}
	defer rows.Close()

	return r.scanRows(rows)
}

// GetPrimaryByUserID retrieves the primary address for a user.
func (r *AddressRepositoryImpl) GetPrimaryByUserID(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) (*addressEntity.Address, error) {
	var address addressEntity.Address
	err := tx.QueryRow(ctx, `
		SELECT `+addressColumns+`
		FROM addresses
		WHERE user_id = $1 AND is_primary = true AND is_available_for_checkout = true
		LIMIT 1
	`, userID).Scan(
		&address.ID,
		&address.UserID,
		&address.Nickname,
		&address.RecipientName,
		&address.Phone,
		&address.ProvinceID,
		&address.ProvinceName,
		&address.CityID,
		&address.CityName,
		&address.DistrictID,
		&address.DistrictName,
		&address.VillageID,
		&address.VillageName,
		&address.StreetAddress,
		&address.PostalCode,
		&address.Latitude,
		&address.Longitude,
		&address.Notes,
		&address.IsPrimary,
		&address.IsAvailableForCheckout,
		&address.CreatedAt,
		&address.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // No primary address set
		}
		return nil, fmt.Errorf("get primary address failed: %w", err)
	}

	return &address, nil
}

// SetPrimary sets an address as primary and unsets all other primary addresses.
func (r *AddressRepositoryImpl) SetPrimary(
	ctx context.Context,
	tx db.Tx,
	addressID uuid.UUID,
) error {
	// First, get the user ID of this address
	var userID uuid.UUID
	err := tx.QueryRow(ctx, `SELECT user_id FROM addresses WHERE id = $1`, addressID).Scan(&userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("address not found: %s", addressID)
		}
		return fmt.Errorf("failed to get address user: %w", err)
	}

	// Unset all primary addresses for this user
	if err := r.UnsetAllPrimary(ctx, tx, userID); err != nil {
		return fmt.Errorf("failed to unset existing primary: %w", err)
	}

	// Set this address as primary
	_, err = tx.Exec(ctx, `
		UPDATE addresses
		SET is_primary = true, updated_at = NOW()
		WHERE id = $1
	`, addressID)

	if err != nil {
		return fmt.Errorf("set primary address failed: %w", err)
	}

	return nil
}

// UnsetAllPrimary removes the primary flag from all addresses for a user.
func (r *AddressRepositoryImpl) UnsetAllPrimary(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) error {
	_, err := tx.Exec(ctx, `
		UPDATE addresses
		SET is_primary = false, updated_at = NOW()
		WHERE user_id = $1 AND is_primary = true
	`, userID)

	if err != nil {
		return fmt.Errorf("unset all primary addresses failed: %w", err)
	}

	return nil
}

// CountByUserID returns address counts grouped by tag.
func (r *AddressRepositoryImpl) CountByUserID(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) (*addressRepo.AddressCount, error) {
	var total int64
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM addresses
		WHERE user_id = $1 AND is_available_for_checkout = true
	`, userID).Scan(&total); err != nil {
		return nil, fmt.Errorf("count addresses by user failed: %w", err)
	}

	return &addressRepo.AddressCount{Total: total}, nil
}

// scanRows is a helper to scan addresses from rows.
func (r *AddressRepositoryImpl) scanRows(rows pgx.Rows) ([]*addressEntity.Address, error) {
	var addresses []*addressEntity.Address

	for rows.Next() {
		var address addressEntity.Address

		err := rows.Scan(
			&address.ID,
			&address.UserID,
			&address.Nickname,
			&address.RecipientName,
			&address.Phone,
			&address.ProvinceID,
			&address.ProvinceName,
			&address.CityID,
			&address.CityName,
			&address.DistrictID,
			&address.DistrictName,
			&address.VillageID,
			&address.VillageName,
			&address.StreetAddress,
			&address.PostalCode,
			&address.Latitude,
			&address.Longitude,
			&address.Notes,
			&address.IsPrimary,
			&address.IsAvailableForCheckout,
			&address.CreatedAt,
			&address.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan address failed: %w", err)
		}

		addresses = append(addresses, &address)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate addresses failed: %w", rows.Err())
	}

	if addresses == nil {
		return []*addressEntity.Address{}, nil
	}

	return addresses, nil
}

// Ensure AddressRepositoryImpl implements the interface
var _ addressRepo.AddressRepository = (*AddressRepositoryImpl)(nil)


