package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/pkg/db"
)

// ReportRepository defines the persistence contract for canonical Reports.
//
// SLICE 2: Canonical Report authority. The only producer of Report rows is the
// Report service via this repository. There is no fallback to moderation_cases
// (dropped in migration 000056) and no bridge to GovernanceCase.
type ReportRepository interface {
	// Create persists a new immutable Report within a transaction.
	// Returns ErrDuplicateReport if (reporter_id, subject_type, subject_id)
	// already exists — the unique index is the race-safe final guard.
	Create(ctx context.Context, tx db.Tx, report *entity.Report) error

	// ValidateTarget validates that a polymorphic subject exists in its
	// canonical target domain table and returns the minimal immutable evidence
	// snapshot of the subject at report time.
	// Returns ErrReportTargetNotFound when the subject does not exist.
	ValidateTarget(ctx context.Context, tx db.Tx, subjectType entity.ReportTargetType, subjectID uuid.UUID) (*entity.EvidenceSnapshot, error)

	// GetByID retrieves a Report by its ID. Returns nil when not found.
	GetByID(ctx context.Context, tx db.Tx, reportID uuid.UUID) (*entity.Report, error)

	// ListByReporter retrieves Reports created by a specific reporter,
	// ordered by created_at DESC, with pagination.
	ListByReporter(ctx context.Context, tx db.Tx, reporterID uuid.UUID, limit, offset int) ([]*entity.Report, error)

	// HasUserReported returns true if the reporter already has a Report for
	// the same subject. Used for early UX feedback; the unique index remains
	// the final guard.
	HasUserReported(ctx context.Context, tx db.Tx, reporterID uuid.UUID, subjectType entity.ReportTargetType, subjectID uuid.UUID) (bool, error)
}

// ErrDuplicateReport is returned when a report already exists for the same
// reporter + subject.
type ErrDuplicateReport struct {
	ReporterID  uuid.UUID
	SubjectType entity.ReportTargetType
	SubjectID   uuid.UUID
}

func (e *ErrDuplicateReport) Error() string {
	return "you have already reported this subject"
}

// ErrReportTargetNotFound is returned when the subject does not exist in its
// canonical target domain.
type ErrReportTargetNotFound struct {
	SubjectType entity.ReportTargetType
	SubjectID   uuid.UUID
}

func (e *ErrReportTargetNotFound) Error() string {
	return "report target not found"
}
