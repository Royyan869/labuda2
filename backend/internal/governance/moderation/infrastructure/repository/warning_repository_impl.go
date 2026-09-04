package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/pkg/db"
)

// WarningRepositoryImpl handles warning persistence using pgx-based DB layer.
type WarningRepositoryImpl struct{}

// NewWarningRepository creates a new WarningRepository.
func NewWarningRepository() WarningRepository {
	return &WarningRepositoryImpl{}
}

// GetByID retrieves a warning without locking (for read-only operations).
func (r *WarningRepositoryImpl) GetByID(
	ctx context.Context,
	tx interface{},
	warningID uuid.UUID,
) (*entity.UserWarning, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	var level string
	var userID, issuedBy uuid.UUID
	var reason string
	var isActive bool
	var revokedAt *time.Time
	var revokedBy *uuid.UUID
	var createdAt time.Time
	var expiresAt *time.Time

	err := dbTx.QueryRow(ctx, `
		SELECT id, user_id, issued_by, level, reason,
		       is_active, revoked_at, revoked_by,
		       created_at, expires_at
		FROM user_warnings
		WHERE id = $1
	`, warningID).Scan(
		&warningID, &userID, &issuedBy, &level, &reason,
		&isActive, &revokedAt, &revokedBy,
		&createdAt, &expiresAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, &entity.ErrWarningNotFound{WarningID: warningID}
		}
		return nil, fmt.Errorf("get warning failed: %w", err)
	}

	return &entity.UserWarning{
		ID:        warningID,
		UserID:    userID,
		IssuedBy:  issuedBy,
		Level:     entity.WarningLevel(level),
		Reason:    reason,
		IsActive:  isActive,
		RevokedAt: revokedAt,
		RevokedBy: revokedBy,
		CreatedAt: createdAt,
		ExpiresAt: expiresAt,
	}, nil
}

// GetForUpdate retrieves a warning with FOR UPDATE lock.
// CRITICAL: Must be used for all revoke operations to prevent concurrent modifications.
func (r *WarningRepositoryImpl) GetForUpdate(
	ctx context.Context,
	tx interface{},
	warningID uuid.UUID,
) (*entity.UserWarning, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	var level string
	var userID, issuedBy uuid.UUID
	var reason string
	var isActive bool
	var revokedAt *time.Time
	var revokedBy *uuid.UUID
	var createdAt time.Time
	var expiresAt *time.Time

	err := dbTx.QueryRow(ctx, `
		SELECT id, user_id, issued_by, level, reason,
		       is_active, revoked_at, revoked_by,
		       created_at, expires_at
		FROM user_warnings
		WHERE id = $1
		FOR UPDATE
	`, warningID).Scan(
		&warningID, &userID, &issuedBy, &level, &reason,
		&isActive, &revokedAt, &revokedBy,
		&createdAt, &expiresAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, &entity.ErrWarningNotFound{WarningID: warningID}
		}
		return nil, fmt.Errorf("get warning for update failed: %w", err)
	}

	return &entity.UserWarning{
		ID:        warningID,
		UserID:    userID,
		IssuedBy:  issuedBy,
		Level:     entity.WarningLevel(level),
		Reason:    reason,
		IsActive:  isActive,
		RevokedAt: revokedAt,
		RevokedBy: revokedBy,
		CreatedAt: createdAt,
		ExpiresAt: expiresAt,
	}, nil
}

// Update persists warning changes within a transaction.
func (r *WarningRepositoryImpl) Update(
	ctx context.Context,
	tx interface{},
	warning *entity.UserWarning,
) error {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	_, err := dbTx.Exec(ctx, `
		UPDATE user_warnings
		SET is_active = $2,
		    revoked_at = $3,
		    revoked_by = $4
		WHERE id = $1
	`,
		warning.ID,
		warning.IsActive,
		warning.RevokedAt,
		warning.RevokedBy,
	)

	if err != nil {
		return fmt.Errorf("update warning failed: %w", err)
	}

	return nil
}

// ListByUser retrieves all warnings for a specific user.
// Ordered by created_at DESC (newest first).
func (r *WarningRepositoryImpl) ListByUser(
	ctx context.Context,
	tx interface{},
	userID uuid.UUID,
	limit, offset int,
) ([]*entity.UserWarning, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	query := `
		SELECT id, user_id, issued_by, level, reason,
		       is_active, revoked_at, revoked_by,
		       created_at, expires_at
		FROM user_warnings
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := dbTx.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list warnings by user failed: %w", err)
	}
	defer rows.Close()

	var warnings []*entity.UserWarning
	for rows.Next() {
		warning, err := r.scanRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan warning failed: %w", err)
		}
		warnings = append(warnings, warning)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("list warnings by user scan failed: %w", rows.Err())
	}

	return warnings, nil
}

// ListActiveByUser retrieves active warnings for a specific user.
func (r *WarningRepositoryImpl) ListActiveByUser(
	ctx context.Context,
	tx interface{},
	userID uuid.UUID,
) ([]*entity.UserWarning, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	query := `
		SELECT id, user_id, issued_by, level, reason,
		       is_active, revoked_at, revoked_by,
		       created_at, expires_at
		FROM user_warnings
		WHERE user_id = $1 AND is_active = true
		ORDER BY created_at DESC
	`

	rows, err := dbTx.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list active warnings by user failed: %w", err)
	}
	defer rows.Close()

	var warnings []*entity.UserWarning
	for rows.Next() {
		warning, err := r.scanRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan warning failed: %w", err)
		}
		warnings = append(warnings, warning)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("list active warnings by user scan failed: %w", rows.Err())
	}

	return warnings, nil
}

// ListAll retrieves all warnings with optional user filter.
// If userID is nil, returns all warnings.
// Ordered by created_at DESC (newest first).
func (r *WarningRepositoryImpl) ListAll(
	ctx context.Context,
	tx interface{},
	userID *uuid.UUID,
	isActive *bool,
	limit, offset int,
) ([]*entity.UserWarning, int64, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, 0, fmt.Errorf("invalid transaction type")
	}

	where := " WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	if userID != nil {
		where += fmt.Sprintf(" AND user_id = $%d", argIdx)
		args = append(args, *userID)
		argIdx++
	}

	if isActive != nil {
		where += fmt.Sprintf(" AND is_active = $%d", argIdx)
		args = append(args, *isActive)
		argIdx++
	}

	var total int64
	countQuery := "SELECT COUNT(*) FROM user_warnings" + where
	if err := dbTx.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count warnings failed: %w", err)
	}
	if total == 0 {
		return nil, 0, nil
	}

	query := `
		SELECT id, user_id, issued_by, level, reason,
		       is_active, revoked_at, revoked_by,
		       created_at, expires_at
		FROM user_warnings` + where + fmt.Sprintf(`
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, argIdx, argIdx+1)
	dataArgs := append(args, limit, offset)

	rows, err := dbTx.Query(ctx, query, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list all warnings failed: %w", err)
	}
	defer rows.Close()

	var warnings []*entity.UserWarning
	for rows.Next() {
		warning, err := r.scanRow(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan warning failed: %w", err)
		}
		warnings = append(warnings, warning)
	}

	if rows.Err() != nil {
		return nil, 0, fmt.Errorf("list all warnings scan failed: %w", rows.Err())
	}

	return warnings, total, nil
}

// scanRow scans a warning from a row.
func (r *WarningRepositoryImpl) scanRow(rows pgx.Rows) (*entity.UserWarning, error) {
	var id, userID, issuedBy uuid.UUID
	var level string
	var reason string
	var isActive bool
	var revokedAt *time.Time
	var revokedBy *uuid.UUID
	var createdAt time.Time
	var expiresAt *time.Time

	err := rows.Scan(
		&id, &userID, &issuedBy, &level, &reason,
		&isActive, &revokedAt, &revokedBy,
		&createdAt, &expiresAt,
	)

	if err != nil {
		return nil, fmt.Errorf("scan row failed: %w", err)
	}

	return &entity.UserWarning{
		ID:        id,
		UserID:    userID,
		IssuedBy:  issuedBy,
		Level:     entity.WarningLevel(level),
		Reason:    reason,
		IsActive:  isActive,
		RevokedAt: revokedAt,
		RevokedBy: revokedBy,
		CreatedAt: createdAt,
		ExpiresAt: expiresAt,
	}, nil
}


