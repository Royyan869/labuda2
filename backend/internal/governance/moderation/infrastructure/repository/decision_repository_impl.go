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

// DecisionRepositoryImpl is the canonical Decision persistence using pgx.
type DecisionRepositoryImpl struct{}

// NewDecisionRepository creates the canonical Decision repository.
func NewDecisionRepository() DecisionRepository {
	return &DecisionRepositoryImpl{}
}

// decisionColumns is the canonical column list for decisions queries.
// Keep SELECT and Scan in sync.
const decisionColumns = `id, case_id, decided_by, outcome, decision_note, created_at`

// Create inserts a new immutable Decision.
//
// Decision is append-only: no uniqueness constraint on case_id (multiple
// Decisions per Case are canonical). The trg_decisions_immutable trigger
// blocks any UPDATE at the DB level.
func (r *DecisionRepositoryImpl) Create(ctx context.Context, tx db.Tx, decision *entity.Decision) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO decisions (id, case_id, decided_by, outcome, decision_note)
		VALUES ($1, $2, $3, $4, $5)
	`,
		decision.ID,
		decision.CaseID,
		decision.DecidedBy,
		string(decision.Outcome),
		decision.DecisionNote,
	)
	if err != nil {
		return fmt.Errorf("create decision failed: %w", err)
	}
	return nil
}

// GetByID retrieves a Decision by its ID. Returns nil when not found.
func (r *DecisionRepositoryImpl) GetByID(ctx context.Context, tx db.Tx, decisionID uuid.UUID) (*entity.Decision, error) {
	query := `SELECT ` + decisionColumns + `
		FROM decisions
		WHERE id = $1`

	row := tx.QueryRow(ctx, query, decisionID)
	decision, err := scanDecision(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get decision failed: %w", err)
	}
	return decision, nil
}

// ListByCase retrieves all Decisions for a Case, ordered by created_at DESC.
func (r *DecisionRepositoryImpl) ListByCase(ctx context.Context, tx db.Tx, caseID uuid.UUID, limit, offset int) ([]*entity.Decision, error) {
	query := `SELECT ` + decisionColumns + `
		FROM decisions
		WHERE case_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := tx.Query(ctx, query, caseID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list decisions by case failed: %w", err)
	}
	defer rows.Close()

	var decisions []*entity.Decision
	for rows.Next() {
		decision, err := scanDecisionRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan decision row failed: %w", err)
		}
		decisions = append(decisions, decision)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate decision rows failed: %w", rows.Err())
	}
	return decisions, nil
}

// decisionScanner is satisfied by pgx.Row and pgx.Rows.
type decisionScanner interface {
	Scan(dest ...any) error
}

// scanDecision scans one canonical Decision row from pgx.Row.
// Column order must match decisionColumns.
func scanDecision(row decisionScanner) (*entity.Decision, error) {
	var id, caseID, decidedBy uuid.UUID
	var outcome string
	var decisionNote *string
	var createdAt time.Time

	err := row.Scan(
		&id, &caseID, &decidedBy, &outcome, &decisionNote, &createdAt,
	)
	if err != nil {
		return nil, err
	}

	return &entity.Decision{
		ID:           id,
		CaseID:       caseID,
		DecidedBy:    decidedBy,
		Outcome:      entity.DecisionOutcome(outcome),
		DecisionNote: decisionNote,
		CreatedAt:    createdAt,
	}, nil
}

// scanDecisionRow scans one canonical Decision row from pgx.Rows.
func scanDecisionRow(rows pgx.Rows) (*entity.Decision, error) {
	var id, caseID, decidedBy uuid.UUID
	var outcome string
	var decisionNote *string
	var createdAt time.Time

	err := rows.Scan(
		&id, &caseID, &decidedBy, &outcome, &decisionNote, &createdAt,
	)
	if err != nil {
		return nil, err
	}

	return &entity.Decision{
		ID:           id,
		CaseID:       caseID,
		DecidedBy:    decidedBy,
		Outcome:      entity.DecisionOutcome(outcome),
		DecisionNote: decisionNote,
		CreatedAt:    createdAt,
	}, nil
}
