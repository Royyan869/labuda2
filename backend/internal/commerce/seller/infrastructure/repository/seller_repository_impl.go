package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	sellerEntity "github.com/labuda/backend/internal/commerce/seller/entity"
	"github.com/labuda/backend/pkg/db"
)

// SellerRepositoryImpl handles seller domain persistence using pgx-based DB layer.
// No business logic - all SQL, all data access.
type SellerRepositoryImpl struct{}

// NewSellerRepository creates a new SellerRepositoryImpl.
func NewSellerRepository() *SellerRepositoryImpl {
	return &SellerRepositoryImpl{}
}

// InsertProfileTx creates a new seller profile within a transaction.
func (r *SellerRepositoryImpl) InsertProfileTx(
	ctx context.Context,
	tx db.Tx,
	p *sellerEntity.SellerProfile,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO seller_profiles (
			id, user_id, store_name, store_image_url, store_image_updated_at,
			tier,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`,
		p.ID,
		p.UserID,
		p.StoreName,
		p.StoreImageURL,
		p.StoreImageUpdatedAt,
		p.Tier,
		p.CreatedAt,
		p.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("insert seller profile failed: %w", err)
	}

	return nil
}

// GetByUserID retrieves a seller profile by user ID without locking.
// Returns nil if not found.
func (r *SellerRepositoryImpl) GetByUserID(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) (*sellerEntity.SellerProfile, error) {
	var id uuid.UUID
	var storeName string
	var storeImageURL *string
	var storeImageUpdatedAt *time.Time
	var tier sellerEntity.Tier
	var createdAt, updatedAt time.Time

	err := tx.QueryRow(ctx, `
		SELECT id, store_name, store_image_url, store_image_updated_at, tier, created_at, updated_at
		FROM seller_profiles
		WHERE user_id = $1
	`, userID).Scan(
		&id,
		&storeName,
		&storeImageURL,
		&storeImageUpdatedAt,
		&tier,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("get seller profile by user id failed: %w", err)
	}

	return &sellerEntity.SellerProfile{
		ID:                  id,
		UserID:              userID,
		StoreName:           storeName,
		StoreImageURL:       storeImageURL,
		StoreImageUpdatedAt: storeImageUpdatedAt,
		Tier:                tier,
		CreatedAt:           createdAt,
		UpdatedAt:           updatedAt,
	}, nil
}

// GetByUserIDForUpdate retrieves a seller profile by user ID with row-level lock.
// Uses FOR UPDATE to prevent concurrent modifications during onboarding.
func (r *SellerRepositoryImpl) GetByUserIDForUpdate(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) (*sellerEntity.SellerProfile, error) {
	var id uuid.UUID
	var storeName string
	var storeImageURL *string
	var storeImageUpdatedAt *time.Time
	var tier sellerEntity.Tier
	var createdAt, updatedAt time.Time

	err := tx.QueryRow(ctx, `
		SELECT id, store_name, store_image_url, store_image_updated_at, tier, created_at, updated_at
		FROM seller_profiles
		WHERE user_id = $1
		FOR UPDATE
	`, userID).Scan(
		&id,
		&storeName,
		&storeImageURL,
		&storeImageUpdatedAt,
		&tier,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("get seller profile by user id for update failed: %w", err)
	}

	return &sellerEntity.SellerProfile{
		ID:                  id,
		UserID:              userID,
		StoreName:           storeName,
		StoreImageURL:       storeImageURL,
		StoreImageUpdatedAt: storeImageUpdatedAt,
		Tier:                tier,
		CreatedAt:           createdAt,
		UpdatedAt:           updatedAt,
	}, nil
}

// EnsureProfileExistsTx safely creates or retrieves a seller profile within a transaction.
// Behavior:
// - If profile exists for userID: returns existing profile
// - If not exists: creates new profile with basic tier
// Atomic inside tx - uses INSERT ... ON CONFLICT (user_id) DO NOTHING for race protection.
func (r *SellerRepositoryImpl) EnsureProfileExistsTx(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	storeName string,
) (*sellerEntity.SellerProfile, error) {
	return r.EnsureProfileExistsTxWithImage(ctx, tx, userID, storeName, nil)
}

// EnsureProfileExistsTxWithImage is the canonical variant that also persists the optional store image.
// If storeImageURL is nil or empty, the column remains NULL (placeholder contract).
func (r *SellerRepositoryImpl) EnsureProfileExistsTxWithImage(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	storeName string,
	storeImageURL *string,
) (*sellerEntity.SellerProfile, error) {
	now := time.Now()
	var imageURL *string
	var imageUpdatedAt *time.Time
	if storeImageURL != nil && *storeImageURL != "" {
		imageURL = storeImageURL
		imageUpdatedAt = &now
	}
	newProfile := &sellerEntity.SellerProfile{
		ID:                  uuid.New(),
		UserID:              userID,
		StoreName:           storeName,
		StoreImageURL:       imageURL,
		StoreImageUpdatedAt: imageUpdatedAt,
		Tier:                sellerEntity.TierBasic,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO seller_profiles (
			id, user_id, store_name, store_image_url, store_image_updated_at,
			tier,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id) DO NOTHING
	`,
		newProfile.ID,
		newProfile.UserID,
		newProfile.StoreName,
		newProfile.StoreImageURL,
		newProfile.StoreImageUpdatedAt,
		newProfile.Tier,
		newProfile.CreatedAt,
		newProfile.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("insert seller profile failed: %w", err)
	}

	// Always fetch and return the profile (either the one we just inserted or existing)
	profile, err := r.GetByUserID(ctx, tx, userID)
	if err != nil {
		return nil, fmt.Errorf("get seller profile failed: %w", err)
	}

	if profile == nil {
		// This should never happen if the database is working correctly
		return nil, fmt.Errorf("profile not found after insert/fetch")
	}

	return profile, nil
}

// GetByIDForUpdate retrieves a seller profile by ID with row-level lock.
// Uses FOR UPDATE to prevent concurrent modifications.
func (r *SellerRepositoryImpl) GetByIDForUpdate(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (*sellerEntity.SellerProfile, error) {
	var userID uuid.UUID
	var storeName string
	var storeImageURL *string
	var storeImageUpdatedAt *time.Time
	var tier sellerEntity.Tier
	var createdAt, updatedAt time.Time

	err := tx.QueryRow(ctx, `
		SELECT user_id, store_name, store_image_url, store_image_updated_at, tier, created_at, updated_at
		FROM seller_profiles
		WHERE id = $1
		FOR UPDATE
	`, id).Scan(
		&userID,
		&storeName,
		&storeImageURL,
		&storeImageUpdatedAt,
		&tier,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("get seller profile by id for update failed: %w", err)
	}

	return &sellerEntity.SellerProfile{
		ID:                  id,
		UserID:              userID,
		StoreName:           storeName,
		StoreImageURL:       storeImageURL,
		StoreImageUpdatedAt: storeImageUpdatedAt,
		Tier:                tier,
		CreatedAt:           createdAt,
		UpdatedAt:           updatedAt,
	}, nil
}

// UpdateTierTx updates the tier of a seller profile.
func (r *SellerRepositoryImpl) UpdateTierTx(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
	tier sellerEntity.Tier,
) error {
	_, err := tx.Exec(ctx, `
		UPDATE seller_profiles
		SET tier = $1, updated_at = NOW()
		WHERE id = $2
	`, tier, id)

	if err != nil {
		return fmt.Errorf("update seller tier failed: %w", err)
	}

	return nil
}

// UpdateSellerProfileTx atomically updates store_name and/or store_image_url.
// Nil pointer means "no change" for that column; empty string for image means clear to NULL.
// Only bump store_image_updated_at when image column changes.
func (r *SellerRepositoryImpl) UpdateSellerProfileTx(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	storeName *string,
	storeImageURL *string,
) (*sellerEntity.SellerProfile, error) {
	var existing *sellerEntity.SellerProfile
	var err error
	existing, err = r.GetByUserIDForUpdate(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("seller profile not found")
	}
	// Determine new values
	newStoreName := existing.StoreName
	if storeName != nil {
		trimmed := *storeName
		if trimmed == "" {
			return nil, fmt.Errorf("store_name is required")
		}
		newStoreName = trimmed
	}
	// storeImageURL semantics: nil=no change, ""=clear, valid URL=set
	var newImageURL *string = existing.StoreImageURL
	var newImageUpdatedAt *time.Time = existing.StoreImageUpdatedAt
	imageChanged := false
	if storeImageURL != nil {
		if *storeImageURL == "" {
			if existing.StoreImageURL != nil {
				imageChanged = true
			}
			newImageURL = nil
		} else {
			if existing.StoreImageURL == nil || *existing.StoreImageURL != *storeImageURL {
				imageChanged = true
			}
			newImageURL = storeImageURL
		}
		if imageChanged {
			now := time.Now()
			newImageUpdatedAt = &now
		}
	}
	// Validate image URL if being set
	if newImageURL != nil {
		if err := validateStoreImageURL(*newImageURL); err != nil {
			return nil, err
		}
	}
	// Only update if something changed
	if newStoreName == existing.StoreName && !imageChanged {
		return existing, nil
	}
	_, err = tx.Exec(ctx, `
		UPDATE seller_profiles
		SET store_name = $1, store_image_url = $2, store_image_updated_at = $3, updated_at = NOW()
		WHERE user_id = $4
	`, newStoreName, newImageURL, newImageUpdatedAt, userID)
	if err != nil {
		return nil, fmt.Errorf("update seller profile failed: %w", err)
	}
	return r.GetByUserID(ctx, tx, userID)
}

func validateStoreImageURL(raw string) error {
	if raw == "" {
		return fmt.Errorf("store_image_url must be images/stores/{user_id}.jpg")
	}
	// Canonical persisted form is a raw storage key, not an absolute URL.
	if len(raw) >= 8 && (raw[:8] == "https://" || raw[:7] == "http://") {
		return fmt.Errorf("store_image_url must be images/stores/{user_id}.jpg (got absolute URL)")
	}
	if raw[:14] != "images/stores/" || raw[len(raw)-4:] != ".jpg" {
		return fmt.Errorf("store_image_url must be images/stores/{user_id}.jpg")
	}
	if containsDotDot(raw) {
		return fmt.Errorf("store_image_url must be images/stores/{user_id}.jpg")
	}
	return nil
}

func containsDotDot(s string) bool {
	for i := 0; i+1 < len(s); i++ {
		if s[i] == '.' && s[i+1] == '.' {
			return true
		}
	}
	return false
}

// InsertMonthlyMetricTx creates a new monthly metric snapshot within a transaction.
func (r *SellerRepositoryImpl) InsertMonthlyMetricTx(
	ctx context.Context,
	tx db.Tx,
	m *sellerEntity.SellerMonthlyMetric,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO seller_monthly_metrics (
			id, seller_id, year, month,
			total_items_sold, average_rating,
			fulfilled_count, cancelled_timeout_count,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`,
		m.ID,
		m.SellerID,
		m.Year,
		m.Month,
		m.TotalItemsSold,
		m.AverageRating,
		m.FulfilledCount,
		m.CancelledTimeoutCount,
		m.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("insert seller monthly metric failed: %w", err)
	}

	return nil
}



