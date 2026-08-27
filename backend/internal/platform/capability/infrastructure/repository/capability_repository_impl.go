// Package repository provides the PostgreSQL implementation for capability persistence.
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/platform/capability/entity"
	capabilityRepo "github.com/labuda/backend/internal/platform/capability/repository"
	"github.com/labuda/backend/pkg/db"
)

// CapabilityRepositoryImpl handles capability persistence using pgx-based DB layer.
type CapabilityRepositoryImpl struct {
	db *db.DB
}

// NewCapabilityRepository creates a new CapabilityRepository.
func NewCapabilityRepository(database *db.DB) capabilityRepo.CapabilityRepository {
	return &CapabilityRepositoryImpl{
		db: database,
	}
}

// getExecutor returns the transaction if provided, otherwise returns the pool.
// This allows read operations to work without transactions while maintaining
// transactional support for write operations.
func (r *CapabilityRepositoryImpl) getExecutor(tx interface{}) (interface{}, error) {
	if tx == nil {
		// Use pool for read operations without transaction
		return r.db.Pool(), nil
	}
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}
	return dbTx, nil
}

// Create persists a new capability grant within a transaction.
func (r *CapabilityRepositoryImpl) Create(
	ctx context.Context,
	tx interface{},
	cap *entity.UserCapability,
) error {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	var grantedBy *uuid.UUID
	if cap.GrantedBy != nil {
		grantedBy = cap.GrantedBy
	}

	_, err := dbTx.Exec(ctx, `
		INSERT INTO user_capabilities (
			id, user_id, capability, granted_by, granted_at, revoked_at
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`,
		cap.ID,
		cap.UserID,
		cap.Capability,
		grantedBy,
		cap.GrantedAt,
		cap.RevokedAt,
	)

	if err != nil {
		// Check for unique constraint violation
		if pgErr, ok := err.(*pgconn.PgError); ok {
			if pgErr.ConstraintName == "user_capabilities_unique_active_capability" {
				return &entity.ErrDuplicateCapability{
					UserID:     cap.UserID,
					Capability: cap.Capability,
				}
			}
		}
		return fmt.Errorf("create capability failed: %w", err)
	}

	return nil
}

// GetByID retrieves a capability grant by ID (including revoked).
func (r *CapabilityRepositoryImpl) GetByID(
	ctx context.Context,
	tx interface{},
	id uuid.UUID,
) (*entity.UserCapability, error) {
	executor, err := r.getExecutor(tx)
	if err != nil {
		return nil, err
	}

	var userID uuid.UUID
	var capability string
	var grantedBy *uuid.UUID
	var grantedAt time.Time
	var revokedAt *time.Time

	err = executor.(interface {
		QueryRow(context.Context, string, ...any) pgx.Row
	}).QueryRow(ctx, `
		SELECT id, user_id, capability, granted_by, granted_at, revoked_at
		FROM user_capabilities
		WHERE id = $1
	`, id).Scan(
		&id, &userID, &capability, &grantedBy, &grantedAt, &revokedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("capability not found: %s", id)
		}
		return nil, fmt.Errorf("get capability failed: %w", err)
	}

	return &entity.UserCapability{
		ID:         id,
		UserID:     userID,
		Capability: capability,
		GrantedBy:  grantedBy,
		GrantedAt:  grantedAt,
		RevokedAt:  revokedAt,
	}, nil
}

// GetActiveCapability retrieves an active capability for a user.
func (r *CapabilityRepositoryImpl) GetActiveCapability(
	ctx context.Context,
	tx interface{},
	userID uuid.UUID,
	capability string,
) (*entity.UserCapability, error) {
	executor, err := r.getExecutor(tx)
	if err != nil {
		return nil, err
	}

	var id uuid.UUID
	var grantedBy *uuid.UUID
	var grantedAt time.Time

	err = executor.(interface {
		QueryRow(context.Context, string, ...any) pgx.Row
	}).QueryRow(ctx, `
		SELECT id, user_id, capability, granted_by, granted_at, revoked_at
		FROM user_capabilities
		WHERE user_id = $1 AND capability = $2 AND revoked_at IS NULL
	`, userID, capability).Scan(
		&id, &userID, &capability, &grantedBy, &grantedAt, nil,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Not found, but not an error
		}
		return nil, fmt.Errorf("get active capability failed: %w", err)
	}

	return &entity.UserCapability{
		ID:         id,
		UserID:     userID,
		Capability: capability,
		GrantedBy:  grantedBy,
		GrantedAt:  grantedAt,
		RevokedAt:  nil,
	}, nil
}

// ListActiveCapabilities retrieves all active capabilities for a user.
func (r *CapabilityRepositoryImpl) ListActiveCapabilities(
	ctx context.Context,
	tx interface{},
	userID uuid.UUID,
) ([]*entity.UserCapability, error) {
	executor, err := r.getExecutor(tx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, user_id, capability, granted_by, granted_at, revoked_at
		FROM user_capabilities
		WHERE user_id = $1 AND revoked_at IS NULL
		ORDER BY granted_at ASC
	`

	rows, err := executor.(interface {
		Query(context.Context, string, ...any) (pgx.Rows, error)
	}).Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list active capabilities failed: %w", err)
	}
	defer rows.Close()

	var capabilities []*entity.UserCapability
	for rows.Next() {
		var id, uID uuid.UUID
		var capability string
		var grantedBy *uuid.UUID
		var grantedAt time.Time

		err := rows.Scan(&id, &uID, &capability, &grantedBy, &grantedAt, nil)
		if err != nil {
			return nil, fmt.Errorf("scan capability failed: %w", err)
		}

		capabilities = append(capabilities, &entity.UserCapability{
			ID:         id,
			UserID:     uID,
			Capability: capability,
			GrantedBy:  grantedBy,
			GrantedAt:  grantedAt,
			RevokedAt:  nil,
		})
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("list active capabilities scan failed: %w", rows.Err())
	}

	return capabilities, nil
}

// Revoke soft-deletes a capability by setting revoked_at.
func (r *CapabilityRepositoryImpl) Revoke(
	ctx context.Context,
	tx interface{},
	id uuid.UUID,
	revokedAt *interface{},
) error {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	// Use current time if not provided
	revokedTime := time.Now()
	result, err := dbTx.Exec(ctx, `
		UPDATE user_capabilities
		SET revoked_at = $1
		WHERE id = $2 AND revoked_at IS NULL
	`, revokedTime, id)

	if err != nil {
		return fmt.Errorf("revoke capability failed: %w", err)
	}

	rows := result.RowsAffected()
	if rows == 0 {
		return &entity.ErrCapabilityNotFound{}
	}

	return nil
}

// HasCapability checks if a user has an active capability.
func (r *CapabilityRepositoryImpl) HasCapability(
	ctx context.Context,
	tx interface{},
	userID uuid.UUID,
	capability string,
) (bool, error) {
	executor, err := r.getExecutor(tx)
	if err != nil {
		return false, err
	}

	var exists bool
	err = executor.(interface {
		QueryRow(context.Context, string, ...any) pgx.Row
	}).QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM user_capabilities
			WHERE user_id = $1 AND capability = $2 AND revoked_at IS NULL
		)
	`, userID, capability).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("has capability failed: %w", err)
	}

	return exists, nil
}

// HasAnyCapability checks if a user has any of the given capabilities.
func (r *CapabilityRepositoryImpl) HasAnyCapability(
	ctx context.Context,
	tx interface{},
	userID uuid.UUID,
	capabilities []string,
) (bool, error) {
	if len(capabilities) == 0 {
		return false, nil
	}

	executor, err := r.getExecutor(tx)
	if err != nil {
		return false, err
	}

	var exists bool
	err = executor.(interface {
		QueryRow(context.Context, string, ...any) pgx.Row
	}).QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM user_capabilities
			WHERE user_id = $1 AND capability = ANY($2) AND revoked_at IS NULL
		)
	`, userID, capabilities).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("has any capability failed: %w", err)
	}

	return exists, nil
}

// CountActiveCapabilities returns the number of active capabilities for a user.
func (r *CapabilityRepositoryImpl) CountActiveCapabilities(
	ctx context.Context,
	tx interface{},
	userID uuid.UUID,
) (int, error) {
	executor, err := r.getExecutor(tx)
	if err != nil {
		return 0, err
	}

	var count int
	err = executor.(interface {
		QueryRow(context.Context, string, ...any) pgx.Row
	}).QueryRow(ctx, `
		SELECT COUNT(*)
		FROM user_capabilities
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID).Scan(&count)

	if err != nil {
		return 0, fmt.Errorf("count active capabilities failed: %w", err)
	}

	return count, nil
}

// ListUsersByCapability returns distinct user IDs that hold the given active
// capability. Joins to users table to exclude soft-deleted accounts.
// Uses idx_user_capabilities_by_capability partial index.
func (r *CapabilityRepositoryImpl) ListUsersByCapability(
	ctx context.Context,
	tx interface{},
	capability string,
) ([]uuid.UUID, error) {
	executor, err := r.getExecutor(tx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT DISTINCT uc.user_id
		FROM user_capabilities uc
		JOIN users u ON uc.user_id = u.id
		WHERE uc.capability = $1
		  AND uc.revoked_at IS NULL
		  AND u.deleted_at IS NULL
		ORDER BY uc.user_id
	`

	rows, err := executor.(interface {
		Query(context.Context, string, ...any) (pgx.Rows, error)
	}).Query(ctx, query, capability)
	if err != nil {
		return nil, fmt.Errorf("list users by capability failed: %w", err)
	}
	defer rows.Close()

	var userIDs []uuid.UUID
	for rows.Next() {
		var uid uuid.UUID
		if err := rows.Scan(&uid); err != nil {
			return nil, fmt.Errorf("scan user id failed: %w", err)
		}
		userIDs = append(userIDs, uid)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("list users by capability scan failed: %w", rows.Err())
	}

	return userIDs, nil
}


