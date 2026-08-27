package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	addressEntity "github.com/labuda/backend/internal/identity/address/entity"
	addressRepo "github.com/labuda/backend/internal/identity/address/repository"
	"github.com/labuda/backend/pkg/db"
)

// AddressRepositoryImpl handles address persistence using pgx-based DB layer.
type AddressRepositoryImpl struct{}

// NewAddressRepository creates a new AddressRepositoryImpl.
func NewAddressRepository() *AddressRepositoryImpl {
	return &AddressRepositoryImpl{}
}

// addressColumns is the canonical column list for SELECT queries.
const addressColumns = `id, user_id, purpose, nickname,
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
			id, user_id, purpose, nickname,
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
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23)
	`,
		address.ID,
		address.UserID,
		string(address.Purpose),
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
	var purpose string

	err := tx.QueryRow(ctx, `
		SELECT `+addressColumns+`
		FROM addresses
		WHERE id = $1
	`, id).Scan(
		&address.ID,
		&address.UserID,
		&purpose,
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

	address.Purpose = addressEntity.AddressPurpose(purpose)

	return &address, nil
}

// GetForUpdate retrieves an address with FOR UPDATE lock.
func (r *AddressRepositoryImpl) GetForUpdate(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (*addressEntity.Address, error) {
	var address addressEntity.Address
	var purpose string

	err := tx.QueryRow(ctx, `
		SELECT `+addressColumns+`
		FROM addresses
		WHERE id = $1
		FOR UPDATE
	`, id).Scan(
		&address.ID,
		&address.UserID,
		&purpose,
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

	address.Purpose = addressEntity.AddressPurpose(purpose)

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
		SET purpose = $2,
		    nickname = $3,
		    recipient_name = $4,
		    phone = $5,
		    province_id = $6,
		    province_name = $7,
		    city_id = $8,
		    city_name = $9,
		    district_id = $10,
		    district_name = $11,
		    village_id = $12,
		    village_name = $13,
		    street_address = $14,
		    postal_code = $15,
		    latitude = $16,
		    longitude = $17,
		    notes = $18,
		    is_primary = $19,
		    is_available_for_checkout = $20,
		    updated_at = $21
		WHERE id = $1
	`,
		address.ID,
		string(address.Purpose),
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

// GetByUserIDFiltered retrieves addresses for a user filtered by purpose.
func (r *AddressRepositoryImpl) GetByUserIDFiltered(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	purpose string,
) ([]*addressEntity.Address, error) {
	if purpose == "" {
		return r.GetByUserID(ctx, tx, userID)
	}

	rows, err := tx.Query(ctx, `
		SELECT `+addressColumns+`
		FROM addresses
		WHERE user_id = $1 AND purpose = $2 AND is_available_for_checkout = true
		ORDER BY is_primary DESC, created_at DESC
	`, userID, purpose)

	if err != nil {
		return nil, fmt.Errorf("get addresses by user filtered failed: %w", err)
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
	var purpose string

	err := tx.QueryRow(ctx, `
		SELECT `+addressColumns+`
		FROM addresses
		WHERE user_id = $1 AND is_primary = true AND is_available_for_checkout = true
		LIMIT 1
	`, userID).Scan(
		&address.ID,
		&address.UserID,
		&purpose,
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

	address.Purpose = addressEntity.AddressPurpose(purpose)

	return &address, nil
}

// GetPrimaryByUserIDFiltered retrieves the primary address for a user filtered by purpose.
func (r *AddressRepositoryImpl) GetPrimaryByUserIDFiltered(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	purpose string,
) (*addressEntity.Address, error) {
	if purpose == "" {
		return r.GetPrimaryByUserID(ctx, tx, userID)
	}

	var address addressEntity.Address
	var purposeStr string

	err := tx.QueryRow(ctx, `
		SELECT `+addressColumns+`
		FROM addresses
		WHERE user_id = $1 AND is_primary = true AND purpose = $2 AND is_available_for_checkout = true
		LIMIT 1
	`, userID, purpose).Scan(
		&address.ID,
		&address.UserID,
		&purposeStr,
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
			return nil, nil
		}
		return nil, fmt.Errorf("get primary address filtered failed: %w", err)
	}

	address.Purpose = addressEntity.AddressPurpose(purposeStr)

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

// CountByUserID returns address counts grouped by purpose.
func (r *AddressRepositoryImpl) CountByUserID(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) (*addressRepo.AddressCount, error) {
	rows, err := tx.Query(ctx, `
		SELECT purpose, COUNT(*)
		FROM addresses
		WHERE user_id = $1 AND is_available_for_checkout = true
		GROUP BY purpose
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("count addresses by user failed: %w", err)
	}
	defer rows.Close()

	result := &addressRepo.AddressCount{}
	for rows.Next() {
		var purpose string
		var count int64
		if err := rows.Scan(&purpose, &count); err != nil {
			return nil, fmt.Errorf("scan address count failed: %w", err)
		}
		switch purpose {
		case "shipping":
			result.ShippingCount = count
		case "sender":
			result.SenderCount = count
		}
		result.Total += count
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate address counts failed: %w", rows.Err())
	}

	return result, nil
}

// scanRows is a helper to scan addresses from rows.
func (r *AddressRepositoryImpl) scanRows(rows pgx.Rows) ([]*addressEntity.Address, error) {
	var addresses []*addressEntity.Address

	for rows.Next() {
		var address addressEntity.Address
		var purpose string

		err := rows.Scan(
			&address.ID,
			&address.UserID,
			&purpose,
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

		address.Purpose = addressEntity.AddressPurpose(purpose)
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


