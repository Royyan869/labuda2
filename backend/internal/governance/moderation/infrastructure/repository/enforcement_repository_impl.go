package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/pkg/db"
)

// EnforcementRepositoryImpl is the canonical Enforcement persistence using pgx.
type EnforcementRepositoryImpl struct{}

// NewEnforcementRepository creates the canonical Enforcement repository.
func NewEnforcementRepository() EnforcementRepository {
	return &EnforcementRepositoryImpl{}
}

// enforcementColumns is the canonical column list for enforcements queries.
// Keep SELECT and Scan in sync.
const enforcementColumns = `id, decision_id, target_type, target_id, status, attempt_count, requested_at, started_at, finished_at, last_error, next_attempt_at, created_at, updated_at`

// Create inserts a new pending Enforcement.
//
// The enforcements_decision_target_unique constraint prevents duplicate
// enforcement for the same (decision, target) pair.
func (r *EnforcementRepositoryImpl) Create(ctx context.Context, tx db.Tx, enforcement *entity.Enforcement) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO enforcements (id, decision_id, target_type, target_id, status, attempt_count, requested_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`,
		enforcement.ID,
		enforcement.DecisionID,
		string(enforcement.TargetType),
		enforcement.TargetID,
		string(enforcement.Status),
		enforcement.AttemptCount,
		enforcement.RequestedAt,
	)
	if err != nil {
		return fmt.Errorf("create enforcement failed: %w", err)
	}
	return nil
}

// GetByID retrieves an Enforcement by its ID. Returns nil when not found.
func (r *EnforcementRepositoryImpl) GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.Enforcement, error) {
	query := `SELECT ` + enforcementColumns + `
		FROM enforcements
		WHERE id = $1`

	row := tx.QueryRow(ctx, query, id)
	enforcement, err := scanEnforcement(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get enforcement failed: %w", err)
	}
	return enforcement, nil
}

// GetByDecisionAndTarget retrieves an Enforcement by decision + target.
// Returns nil when not found.
func (r *EnforcementRepositoryImpl) GetByDecisionAndTarget(ctx context.Context, tx db.Tx, decisionID uuid.UUID, targetType entity.ModerationTargetType, targetID uuid.UUID) (*entity.Enforcement, error) {
	query := `SELECT ` + enforcementColumns + `
		FROM enforcements
		WHERE decision_id = $1 AND target_type = $2 AND target_id = $3`

	row := tx.QueryRow(ctx, query, decisionID, string(targetType), targetID)
	enforcement, err := scanEnforcement(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get enforcement by decision+target failed: %w", err)
	}
	return enforcement, nil
}

// UpdateStatus transitions the Enforcement lifecycle atomically.
// Sets updated_at = now(). Returns nil if no rows affected (idempotent).
func (r *EnforcementRepositoryImpl) UpdateStatus(ctx context.Context, tx db.Tx, id uuid.UUID, status entity.EnforcementStatus, lastError *string) error {
	now := time.Now().UTC()
	_, err := tx.Exec(ctx, `
		UPDATE enforcements
		SET status = $1, last_error = $2, updated_at = $3
		WHERE id = $4
	`,
		string(status),
		lastError,
		now,
		id,
	)
	if err != nil {
		return fmt.Errorf("update enforcement status failed: %w", err)
	}
	return nil
}

// MarkProcessing transitions pending/failed → processing atomically.
// Increments attempt_count, sets started_at = now(). Returns nil if no rows affected.
func (r *EnforcementRepositoryImpl) MarkProcessing(ctx context.Context, tx db.Tx, id uuid.UUID) error {
	now := time.Now().UTC()
	result, err := tx.Exec(ctx, `
		UPDATE enforcements
		SET status = $1, attempt_count = attempt_count + 1, started_at = $2, updated_at = $3
		WHERE id = $4 AND status IN ($5, $6)
	`,
		string(entity.EnforcementStatusProcessing),
		now,
		now,
		id,
		string(entity.EnforcementStatusPending),
		string(entity.EnforcementStatusFailed),
	)
	if err != nil {
		return fmt.Errorf("mark enforcement processing failed: %w", err)
	}
	_ = result.RowsAffected() // idempotent: 0 rows = already processing/succeeded
	return nil
}

// MarkSucceeded transitions processing → succeeded atomically.
// Sets finished_at = now(). Returns nil if no rows affected (idempotent).
// GUARD: Only transitions from processing status — prevents pending→succeeded.
func (r *EnforcementRepositoryImpl) MarkSucceeded(ctx context.Context, tx db.Tx, id uuid.UUID) error {
	now := time.Now().UTC()
	result, err := tx.Exec(ctx, `
		UPDATE enforcements
		SET status = $1, finished_at = $2, updated_at = $3
		WHERE id = $4 AND status = $5
	`,
		string(entity.EnforcementStatusSucceeded),
		now,
		now,
		id,
		string(entity.EnforcementStatusProcessing),
	)
	if err != nil {
		return fmt.Errorf("mark enforcement succeeded failed: %w", err)
	}
	_ = result.RowsAffected() // 0 rows = already terminal, idempotent
	return nil
}

// MarkFailed transitions processing → failed atomically.
// Sets finished_at = now(), last_error, next_attempt_at for retry.
// Returns nil if no rows affected (idempotent).
// GUARD: Only transitions from processing status — prevents pending→failed.
func (r *EnforcementRepositoryImpl) MarkFailed(ctx context.Context, tx db.Tx, id uuid.UUID, lastError string, nextAttemptAt *time.Time) error {
	now := time.Now().UTC()
	result, err := tx.Exec(ctx, `
		UPDATE enforcements
		SET status = $1, finished_at = $2, last_error = $3, next_attempt_at = $4, updated_at = $5
		WHERE id = $6 AND status = $7
	`,
		string(entity.EnforcementStatusFailed),
		now,
		lastError,
		nextAttemptAt,
		now,
		id,
		string(entity.EnforcementStatusProcessing),
	)
	if err != nil {
		return fmt.Errorf("mark enforcement failed: %w", err)
	}
	_ = result.RowsAffected() // 0 rows = already terminal, idempotent
	return nil
}

// ListByDecision retrieves all Enforcements for a Decision, ordered by created_at ASC.
func (r *EnforcementRepositoryImpl) ListByDecision(ctx context.Context, tx db.Tx, decisionID uuid.UUID) ([]*entity.Enforcement, error) {
	query := `SELECT ` + enforcementColumns + `
		FROM enforcements
		WHERE decision_id = $1
		ORDER BY created_at ASC`

	rows, err := tx.Query(ctx, query, decisionID)
	if err != nil {
		return nil, fmt.Errorf("list enforcements by decision failed: %w", err)
	}
	defer rows.Close()

	var enforcements []*entity.Enforcement
	for rows.Next() {
		enforcement, err := scanEnforcementRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan enforcement row failed: %w", err)
		}
		enforcements = append(enforcements, enforcement)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate enforcement rows failed: %w", rows.Err())
	}
	return enforcements, nil
}

// enforcementScanner is satisfied by pgx.Row and pgx.Rows.
type enforcementScanner interface {
	Scan(dest ...any) error
}

// scanEnforcement scans one canonical Enforcement row from pgx.Row.
// Column order must match enforcementColumns.
func scanEnforcement(row enforcementScanner) (*entity.Enforcement, error) {
	var id, decisionID, targetID uuid.UUID
	var targetType, status string
	var attemptCount int
	var requestedAt, createdAt, updatedAt time.Time
	var startedAt, finishedAt, nextAttemptAt *time.Time
	var lastError *string

	err := row.Scan(
		&id, &decisionID, &targetType, &targetID, &status, &attemptCount,
		&requestedAt, &startedAt, &finishedAt, &lastError, &nextAttemptAt,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &entity.Enforcement{
		ID:            id,
		DecisionID:    decisionID,
		TargetType:    entity.ModerationTargetType(targetType),
		TargetID:      targetID,
		Status:        entity.EnforcementStatus(status),
		AttemptCount:  attemptCount,
		RequestedAt:   requestedAt,
		StartedAt:     startedAt,
		FinishedAt:    finishedAt,
		LastError:     lastError,
		NextAttemptAt: nextAttemptAt,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
	}, nil
}

// scanEnforcementRow scans one canonical Enforcement row from pgx.Rows.
func scanEnforcementRow(rows pgx.Rows) (*entity.Enforcement, error) {
	var id, decisionID, targetID uuid.UUID
	var targetType, status string
	var attemptCount int
	var requestedAt, createdAt, updatedAt time.Time
	var startedAt, finishedAt, nextAttemptAt *time.Time
	var lastError *string

	err := rows.Scan(
		&id, &decisionID, &targetType, &targetID, &status, &attemptCount,
		&requestedAt, &startedAt, &finishedAt, &lastError, &nextAttemptAt,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &entity.Enforcement{
		ID:            id,
		DecisionID:    decisionID,
		TargetType:    entity.ModerationTargetType(targetType),
		TargetID:      targetID,
		Status:        entity.EnforcementStatus(status),
		AttemptCount:  attemptCount,
		RequestedAt:   requestedAt,
		StartedAt:     startedAt,
		FinishedAt:    finishedAt,
		LastError:     lastError,
		NextAttemptAt: nextAttemptAt,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
	}, nil
}
