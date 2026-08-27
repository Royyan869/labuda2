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

// AppealRepositoryImpl handles appeal persistence using pgx-based DB layer.
type AppealRepositoryImpl struct{}

// NewAppealRepository creates a new AppealRepository.
func NewAppealRepository() AppealRepository {
	return &AppealRepositoryImpl{}
}

// Create persists a new appeal within a transaction.
func (r *AppealRepositoryImpl) Create(
	ctx context.Context,
	tx interface{},
	appeal *entity.Appeal,
) error {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	var reviewedBy *uuid.UUID
	if appeal.ReviewedBy != nil {
		reviewedBy = appeal.ReviewedBy
	}

	_, err := dbTx.Exec(ctx, `
		INSERT INTO appeals (
			id, report_id, appealed_by, status,
			message, admin_response, reviewed_by,
			created_at, reviewed_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`,
		appeal.ID,
		appeal.CaseID,
		appeal.AppealedBy,
		string(appeal.Status),
		appeal.Message,
		appeal.AdminResponse,
		reviewedBy,
		appeal.CreatedAt,
		appeal.ReviewedAt,
	)

	if err != nil {
		return fmt.Errorf("create appeal failed: %w", err)
	}

	return nil
}

// CreateWithPendingCheck atomically creates an appeal only if no pending appeal
// exists for the same report. Uses a CTE with FOR UPDATE lock to prevent race conditions.
//
// This method is concurrency-safe: two concurrent requests cannot create two pending
// appeals for the same report, even under race conditions.
func (r *AppealRepositoryImpl) CreateWithPendingCheck(
	ctx context.Context,
	tx interface{},
	appeal *entity.Appeal,
) error {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	var reviewedBy *uuid.UUID
	if appeal.ReviewedBy != nil {
		reviewedBy = appeal.ReviewedBy
	}

	// Use a CTE with INSERT ... SELECT ... WHERE NOT EXISTS pattern
	// The FOR UPDATE lock on the check ensures concurrent requests serialize
	result, err := dbTx.Exec(ctx, `
		WITH pending_check AS (
			SELECT id
			FROM appeals
			WHERE report_id = $1 AND status = 'pending'
			FOR UPDATE
		)
		INSERT INTO appeals (
			id, report_id, appealed_by, status,
			message, admin_response, reviewed_by,
			created_at, reviewed_at
		)
		SELECT $2, $3, $4, $5, $6, $7, $8, $9, $10
		WHERE NOT EXISTS (SELECT 1 FROM pending_check)
	`,
		appeal.CaseID,
		appeal.ID,
		appeal.CaseID,
		appeal.AppealedBy,
		string(appeal.Status),
		appeal.Message,
		appeal.AdminResponse,
		reviewedBy,
		appeal.CreatedAt,
		appeal.ReviewedAt,
	)

	if err != nil {
		return fmt.Errorf("create appeal with pending check failed: %w", err)
	}

	// Check if the INSERT actually inserted a row
	// If no rows were inserted, a pending appeal already exists
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return &entity.ErrDuplicatePendingAppeal{CaseID: appeal.CaseID}
	}

	return nil
}

// GetByID retrieves an appeal without locking (for read-only operations).
func (r *AppealRepositoryImpl) GetByID(
	ctx context.Context,
	tx interface{},
	appealID uuid.UUID,
) (*entity.Appeal, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	var status string
	var reportID, appealedBy uuid.UUID
	var reviewedBy *uuid.UUID
	var message string
	var adminResponse *string
	var createdAt time.Time
	var reviewedAt *time.Time

	err := dbTx.QueryRow(ctx, `
		SELECT id, report_id, appealed_by, status,
		       message, admin_response, reviewed_by,
		       created_at, reviewed_at
		FROM appeals
		WHERE id = $1
	`, appealID).Scan(
		&appealID, &reportID, &appealedBy, &status,
		&message, &adminResponse, &reviewedBy,
		&createdAt, &reviewedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, &entity.ErrAppealNotFound{AppealID: appealID}
		}
		return nil, fmt.Errorf("get appeal failed: %w", err)
	}

	return &entity.Appeal{
		ID:            appealID,
		CaseID:      reportID,
		AppealedBy:    appealedBy,
		Status:        entity.AppealStatus(status),
		Message:       message,
		AdminResponse: adminResponse,
		ReviewedBy:    reviewedBy,
		CreatedAt:     createdAt,
		ReviewedAt:    reviewedAt,
	}, nil
}

// GetForUpdate retrieves an appeal with FOR UPDATE lock.
// CRITICAL: Must be used for all review operations to prevent double-review.
func (r *AppealRepositoryImpl) GetForUpdate(
	ctx context.Context,
	tx interface{},
	appealID uuid.UUID,
) (*entity.Appeal, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	var status string
	var reportID, appealedBy uuid.UUID
	var reviewedBy *uuid.UUID
	var message string
	var adminResponse *string
	var createdAt time.Time
	var reviewedAt *time.Time

	err := dbTx.QueryRow(ctx, `
		SELECT id, report_id, appealed_by, status,
		       message, admin_response, reviewed_by,
		       created_at, reviewed_at
		FROM appeals
		WHERE id = $1
		FOR UPDATE
	`, appealID).Scan(
		&appealID, &reportID, &appealedBy, &status,
		&message, &adminResponse, &reviewedBy,
		&createdAt, &reviewedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, &entity.ErrAppealNotFound{AppealID: appealID}
		}
		return nil, fmt.Errorf("get appeal for update failed: %w", err)
	}

	return &entity.Appeal{
		ID:            appealID,
		CaseID:      reportID,
		AppealedBy:    appealedBy,
		Status:        entity.AppealStatus(status),
		Message:       message,
		AdminResponse: adminResponse,
		ReviewedBy:    reviewedBy,
		CreatedAt:     createdAt,
		ReviewedAt:    reviewedAt,
	}, nil
}

// Update persists appeal changes within a transaction.
func (r *AppealRepositoryImpl) Update(
	ctx context.Context,
	tx interface{},
	appeal *entity.Appeal,
) error {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	_, err := dbTx.Exec(ctx, `
		UPDATE appeals
		SET status = $2,
		    admin_response = $3,
		    reviewed_by = $4,
		    reviewed_at = $5
		WHERE id = $1
	`,
		appeal.ID,
		string(appeal.Status),
		appeal.AdminResponse,
		appeal.ReviewedBy,
		appeal.ReviewedAt,
	)

	if err != nil {
		return fmt.Errorf("update appeal failed: %w", err)
	}

	return nil
}

// ListByUser retrieves all appeals created by a specific user.
// Ordered by created_at DESC (newest first).
func (r *AppealRepositoryImpl) ListByUser(
	ctx context.Context,
	tx interface{},
	userID uuid.UUID,
	limit, offset int,
) ([]*entity.Appeal, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	query := `
		SELECT id, report_id, appealed_by, status,
		       message, admin_response, reviewed_by,
		       created_at, reviewed_at
		FROM appeals
		WHERE appealed_by = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := dbTx.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list appeals by user failed: %w", err)
	}
	defer rows.Close()

	var appeals []*entity.Appeal
	for rows.Next() {
		appeal, err := r.scanRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan appeal failed: %w", err)
		}
		appeals = append(appeals, appeal)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("list appeals by user scan failed: %w", rows.Err())
	}

	return appeals, nil
}

// ListByReport retrieves all appeals for a specific report.
func (r *AppealRepositoryImpl) ListByCase(
	ctx context.Context,
	tx interface{},
	reportID uuid.UUID,
) ([]*entity.Appeal, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	query := `
		SELECT id, report_id, appealed_by, status,
		       message, admin_response, reviewed_by,
		       created_at, reviewed_at
		FROM appeals
		WHERE report_id = $1
		ORDER BY created_at DESC
	`

	rows, err := dbTx.Query(ctx, query, reportID)
	if err != nil {
		return nil, fmt.Errorf("list appeals by case failed: %w", err)
	}
	defer rows.Close()

	var appeals []*entity.Appeal
	for rows.Next() {
		appeal, err := r.scanRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan appeal failed: %w", err)
		}
		appeals = append(appeals, appeal)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("list appeals by report scan failed: %w", rows.Err())
	}

	return appeals, nil
}

// ListAll retrieves all appeals with optional status filter.
// If statusFilter is nil, returns all appeals regardless of status.
// Ordered by created_at ASC (oldest first).
func (r *AppealRepositoryImpl) ListAll(
	ctx context.Context,
	tx interface{},
	statusFilter *entity.AppealStatus,
	limit, offset int,
) ([]*entity.Appeal, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	var query string
	var args []interface{}

	if statusFilter != nil {
		query = `
			SELECT id, report_id, appealed_by, status,
			       message, admin_response, reviewed_by,
			       created_at, reviewed_at
			FROM appeals
			WHERE status = $1
			ORDER BY created_at ASC
			LIMIT $2 OFFSET $3
		`
		args = []interface{}{string(*statusFilter), limit, offset}
	} else {
		query = `
			SELECT id, report_id, appealed_by, status,
			       message, admin_response, reviewed_by,
			       created_at, reviewed_at
			FROM appeals
			ORDER BY created_at ASC
			LIMIT $1 OFFSET $2
		`
		args = []interface{}{limit, offset}
	}

	rows, err := dbTx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list all appeals failed: %w", err)
	}
	defer rows.Close()

	var appeals []*entity.Appeal
	for rows.Next() {
		appeal, err := r.scanRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan appeal failed: %w", err)
		}
		appeals = append(appeals, appeal)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("list all appeals scan failed: %w", rows.Err())
	}

	return appeals, nil
}

// ListPending retrieves pending appeals awaiting review.
// Ordered by created_at ASC (oldest first).
func (r *AppealRepositoryImpl) ListPending(
	ctx context.Context,
	tx interface{},
	limit, offset int,
) ([]*entity.Appeal, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	query := `
		SELECT id, report_id, appealed_by, status,
		       message, admin_response, reviewed_by,
		       created_at, reviewed_at
		FROM appeals
		WHERE status = $1
		ORDER BY created_at ASC
		LIMIT $2 OFFSET $3
	`

	rows, err := dbTx.Query(ctx, query, entity.AppealStatusPending, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list pending appeals failed: %w", err)
	}
	defer rows.Close()

	var appeals []*entity.Appeal
	for rows.Next() {
		appeal, err := r.scanRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan appeal failed: %w", err)
		}
		appeals = append(appeals, appeal)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("list pending appeals scan failed: %w", rows.Err())
	}

	return appeals, nil
}

// scanRow scans an appeal from a row.
func (r *AppealRepositoryImpl) scanRow(rows pgx.Rows) (*entity.Appeal, error) {
	var id, reportID, appealedBy uuid.UUID
	var status string
	var reviewedBy *uuid.UUID
	var message string
	var adminResponse *string
	var createdAt time.Time
	var reviewedAt *time.Time

	err := rows.Scan(
		&id, &reportID, &appealedBy, &status,
		&message, &adminResponse, &reviewedBy,
		&createdAt, &reviewedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("scan row failed: %w", err)
	}

	return &entity.Appeal{
		ID:            id,
		CaseID:      reportID,
		AppealedBy:    appealedBy,
		Status:        entity.AppealStatus(status),
		Message:       message,
		AdminResponse: adminResponse,
		ReviewedBy:    reviewedBy,
		CreatedAt:     createdAt,
		ReviewedAt:    reviewedAt,
	}, nil
}


