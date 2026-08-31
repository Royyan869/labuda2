// SLICE 3: Canonical Case repository interface.
// The only producer of Case rows is the Case service via this repository.
// There is no bridge to GovernanceCase or moderation_cases.

package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/pkg/db"
)

// CaseRepository defines the persistence contract for canonical Cases.
type CaseRepository interface {
	// FindOrCreateOpenCase finds an existing open Case for the subject,
	// or creates a new one if none exists. Returns the Case.
	//
	// Race safety: The partial unique index uniq_active_case_per_subject
	// is the final guard. Under concurrent requests, exactly one INSERT
	// succeeds; the other gets 23505 and is handled by retrying the SELECT.
	FindOrCreateOpenCase(ctx context.Context, tx db.Tx, subjectType entity.ReportTargetType, subjectID uuid.UUID) (*entity.CanonicalCase, error)

	// GetByID retrieves a Case by its ID. Returns (nil, nil) when not found.
	GetByID(ctx context.Context, tx db.Tx, caseID uuid.UUID) (*entity.CanonicalCase, error)

	// ListBySubject retrieves all Cases for a subject, ordered by created_at DESC.
	ListBySubject(ctx context.Context, tx db.Tx, subjectType entity.ReportTargetType, subjectID uuid.UUID, limit, offset int) ([]*entity.CanonicalCase, error)

	// ResolveCase marks a Case as resolved (used when a Decision is made).
	// Returns ErrCaseAlreadyResolved if the Case is not open.
	ResolveCase(ctx context.Context, tx db.Tx, caseID uuid.UUID) error

	// ListAll retrieves all Cases for admin governance view, ordered by created_at DESC.
	// If statusFilter is non-nil, only cases with that status are returned.
	ListAll(ctx context.Context, tx db.Tx, statusFilter *entity.CaseStatus, limit, offset int) ([]*entity.CanonicalCase, error)

	// CountAll returns the total number of Cases for admin pagination.
	// If statusFilter is non-nil, only cases with that status are counted.
	CountAll(ctx context.Context, tx db.Tx, statusFilter *entity.CaseStatus) (int, error)
}
