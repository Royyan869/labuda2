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
			id, user_id, store_name,
			tier,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`,
		p.ID,
		p.UserID,
		p.StoreName,
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
	var tier sellerEntity.Tier
	var createdAt, updatedAt time.Time

	err := tx.QueryRow(ctx, `
		SELECT id, store_name, tier, created_at, updated_at
		FROM seller_profiles
		WHERE user_id = $1
	`, userID).Scan(
		&id,
		&storeName,
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
		ID:        id,
		UserID:    userID,
		StoreName: storeName,
		Tier:      tier,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
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
	var tier sellerEntity.Tier
	var createdAt, updatedAt time.Time

	err := tx.QueryRow(ctx, `
		SELECT id, store_name, tier, created_at, updated_at
		FROM seller_profiles
		WHERE user_id = $1
		FOR UPDATE
	`, userID).Scan(
		&id,
		&storeName,
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
		ID:        id,
		UserID:    userID,
		StoreName: storeName,
		Tier:      tier,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
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
	// Try to insert new profile with ON CONFLICT DO NOTHING
	// This is deterministic and doesn't rely on string error matching
	now := time.Now()
	newProfile := &sellerEntity.SellerProfile{
		ID:        uuid.New(),
		UserID:    userID,
		StoreName: storeName,
		Tier:      sellerEntity.TierBasic,
		CreatedAt: now,
		UpdatedAt: now,
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO seller_profiles (
			id, user_id, store_name,
			tier,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id) DO NOTHING
	`,
		newProfile.ID,
		newProfile.UserID,
		newProfile.StoreName,
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
	var tier sellerEntity.Tier
	var createdAt, updatedAt time.Time

	err := tx.QueryRow(ctx, `
		SELECT user_id, store_name, tier, created_at, updated_at
		FROM seller_profiles
		WHERE id = $1
		FOR UPDATE
	`, id).Scan(
		&userID,
		&storeName,
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
		ID:        id,
		UserID:    userID,
		StoreName: storeName,
		Tier:      tier,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
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



