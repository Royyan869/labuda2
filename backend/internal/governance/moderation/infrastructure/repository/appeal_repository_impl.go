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
//
// SLICE A: All SQL references decision_id (canonical: appeals.decision_id → decisions.id).
// Legacy report_id references have been removed. The appeals table schema
// was already migrated by migration 000055.
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
			id, decision_id, appealed_by, status,
			message, admin_response, reviewed_by,
			created_at, reviewed_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`,
		appeal.ID,
		appeal.DecisionID,
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
// exists for the same Decision. Uses a CTE with FOR UPDATE lock to prevent race conditions.
//
// Canonical: one pending appeal per Decision (Design §35 concurrency constraint).
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
	// Check for existing pending appeal on the same Decision
	result, err := dbTx.Exec(ctx, `
		WITH pending_check AS (
			SELECT id
			FROM appeals
			WHERE decision_id = $1 AND status = 'pending'
			FOR UPDATE
		)
		INSERT INTO appeals (
			id, decision_id, appealed_by, status,
			message, admin_response, reviewed_by,
			created_at, reviewed_at
		)
		SELECT $2, $3, $4, $5, $6, $7, $8, $9, $10
		WHERE NOT EXISTS (SELECT 1 FROM pending_check)
	`,
		appeal.DecisionID,
		appeal.ID,
		appeal.DecisionID,
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
		return &entity.ErrDuplicatePendingAppeal{DecisionID: appeal.DecisionID}
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
	var decisionID, appealedBy uuid.UUID
	var reviewedBy *uuid.UUID
	var message string
	var adminResponse *string
	var createdAt time.Time
	var reviewedAt *time.Time

	err := dbTx.QueryRow(ctx, `
		SELECT id, decision_id, appealed_by, status,
		       message, admin_response, reviewed_by,
		       created_at, reviewed_at
		FROM appeals
		WHERE id = $1
	`, appealID).Scan(
		&appealID, &decisionID, &appealedBy, &status,
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
		DecisionID:    decisionID,
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
	var decisionID, appealedBy uuid.UUID
	var reviewedBy *uuid.UUID
	var message string
	var adminResponse *string
	var createdAt time.Time
	var reviewedAt *time.Time

	err := dbTx.QueryRow(ctx, `
		SELECT id, decision_id, appealed_by, status,
		       message, admin_response, reviewed_by,
		       created_at, reviewed_at
		FROM appeals
		WHERE id = $1
		FOR UPDATE
	`, appealID).Scan(
		&appealID, &decisionID, &appealedBy, &status,
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
		DecisionID:    decisionID,
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
		SELECT id, decision_id, appealed_by, status,
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

// ListByDecisionID retrieves all appeals for a specific Decision.
// Canonical: Decision 1 → 0..N Appeal (Design §5).
func (r *AppealRepositoryImpl) ListByDecisionID(
	ctx context.Context,
	tx interface{},
	decisionID uuid.UUID,
) ([]*entity.Appeal, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	query := `
		SELECT id, decision_id, appealed_by, status,
		       message, admin_response, reviewed_by,
		       created_at, reviewed_at
		FROM appeals
		WHERE decision_id = $1
		ORDER BY created_at DESC
	`

	rows, err := dbTx.Query(ctx, query, decisionID)
	if err != nil {
		return nil, fmt.Errorf("list appeals by decision failed: %w", err)
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
		return nil, fmt.Errorf("list appeals by decision scan failed: %w", rows.Err())
	}

	return appeals, nil
}

// ListByCase retrieves all appeals for a specific Case.
// Joins through decisions table: appeals → decisions → case.
func (r *AppealRepositoryImpl) ListByCase(
	ctx context.Context,
	tx interface{},
	caseID uuid.UUID,
) ([]*entity.Appeal, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	query := `
		SELECT a.id, a.decision_id, a.appealed_by, a.status,
		       a.message, a.admin_response, a.reviewed_by,
		       a.created_at, a.reviewed_at
		FROM appeals a
		JOIN decisions d ON d.id = a.decision_id
		WHERE d.case_id = $1
		ORDER BY a.created_at DESC
	`

	rows, err := dbTx.Query(ctx, query, caseID)
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
		return nil, fmt.Errorf("list appeals by case scan failed: %w", rows.Err())
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
			SELECT id, decision_id, appealed_by, status,
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
			SELECT id, decision_id, appealed_by, status,
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
		SELECT id, decision_id, appealed_by, status,
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
	var id, decisionID, appealedBy uuid.UUID
	var status string
	var reviewedBy *uuid.UUID
	var message string
	var adminResponse *string
	var createdAt time.Time
	var reviewedAt *time.Time

	err := rows.Scan(
		&id, &decisionID, &appealedBy, &status,
		&message, &adminResponse, &reviewedBy,
		&createdAt, &reviewedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("scan row failed: %w", err)
	}

	return &entity.Appeal{
		ID:            id,
		DecisionID:    decisionID,
		AppealedBy:    appealedBy,
		Status:        entity.AppealStatus(status),
		Message:       message,
		AdminResponse: adminResponse,
		ReviewedBy:    reviewedBy,
		CreatedAt:     createdAt,
		ReviewedAt:    reviewedAt,
	}, nil
}
