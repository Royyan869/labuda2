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

// ModerationRepositoryImpl is the LEGACY GovernanceCase read path.
//
// SLICE 2 TEARDOWN: Report intake and admin Case review methods were removed.
// Only GetByID remains, solely to keep the out-of-scope Appeal domain (Slice 9)
// compiling. It reads the moderation_cases table, which migration 000056
// dropped — so every call fails at runtime. The Appeal domain must be rebuilt
// against the canonical Decision schema in its own slice.
type ModerationRepositoryImpl struct{}

// NewModerationRepository creates the legacy read-only repository.
func NewModerationRepository() ModerationRepository {
	return &ModerationRepositoryImpl{}
}

// GetByID retrieves a legacy governance case by ID.
//
// RUNTIME-DEAD: moderation_cases was dropped in migration 000056. This method
// exists only for Appeal-domain compilation and always fails with a relation
// not found error at runtime.
func (r *ModerationRepositoryImpl) GetByID(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	query := `SELECT id, resource_type, resource_id, status,
		       reported_by, reviewed_by, reason, decision_note,
		       created_at, reviewed_at
		FROM moderation_cases
		WHERE id = $1`

	row := dbTx.QueryRow(ctx, query, caseID)

	var id, resourceID, reportedBy uuid.UUID
	var resourceType, status string
	var reviewedBy *uuid.UUID
	var reason string
	var decisionNote *string
	var createdAt time.Time
	var reviewedAt *time.Time

	err := row.Scan(
		&id, &resourceType, &resourceID, &status,
		&reportedBy, &reviewedBy, &reason, &decisionNote,
		&createdAt, &reviewedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("governance case not found: %s", caseID)
		}
		return nil, fmt.Errorf("get governance case failed: %w", err)
	}

	return &entity.GovernanceCase{
		ID:           id,
		ResourceType: entity.ResourceType(resourceType),
		ResourceID:   resourceID,
		Status:       entity.GovernanceCaseStatus(status),
		ReportedBy:   reportedBy,
		ReviewedBy:   reviewedBy,
		Reason:       reason,
		DecisionNote: decisionNote,
		CreatedAt:    createdAt,
		ReviewedAt:   reviewedAt,
	}, nil
}
