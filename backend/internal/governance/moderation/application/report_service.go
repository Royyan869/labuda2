package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/internal/governance/moderation/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
)

// Transactor represents the ability to execute functions within transactions.
type Transactor interface {
	WithTx(ctx context.Context, fn func(tx db.Tx) error) error
}

// ReportService is the canonical Report intake service.
//
// SLICE 2: The single authority for Report creation. It replaces the rejected
// CreateCase → GovernanceCase → moderation_cases intake path.
//
// Report is an immutable historical intake record: after creation there is no
// update path for reporter_id, subject_type, subject_id, reason_code,
// reason_note, evidence_snapshot, or created_at.
type ReportService struct {
	db   Transactor
	repo repository.ReportRepository
}

// NewReportService creates the canonical Report service.
func NewReportService(db Transactor, repo repository.ReportRepository) *ReportService {
	return &ReportService{db: db, repo: repo}
}

// CreateReportInput is the validated input for creating a Report.
type CreateReportInput struct {
	ReporterID  uuid.UUID
	SubjectType entity.ReportTargetType
	SubjectID   uuid.UUID
	ReasonCode  entity.ReportReasonCode
	ReasonNote  *string
}

// CreateReport creates a new immutable Report.
//
// Business rules:
//   - Subject type must be canonical (content|comment|for_sale|auction|user).
//     chat_message and fixed_price_sale are rejected.
//   - reason_code must be from the locked taxonomy.
//   - reason_note is optional free text (NOT a replacement for reason_code).
//   - The subject must exist in its canonical target domain
//     (application-level existence validation — polymorphic, no single FK).
//   - Self-report is DENIED (Owner decision, Business Truth §6): a user may
//     not report their own content/comment/for_sale/auction/profile.
//   - Same reporter + same subject → duplicate rejected (DB unique index is
//     the race-safe final guard; ErrDuplicateReport for early UX).
//   - Different reporter + same subject → valid.
func (s *ReportService) CreateReport(ctx context.Context, input CreateReportInput) (*entity.Report, error) {
	// Canonical target type validation (backend is the authority).
	if !input.SubjectType.IsValid() {
		return nil, &entity.ErrInvalidReportTarget{SubjectType: string(input.SubjectType)}
	}

	// Locked reason taxonomy validation (backend is the authority).
	if !input.ReasonCode.IsValid() {
		return nil, &entity.ErrInvalidReasonCode{ReasonCode: string(input.ReasonCode)}
	}

	var report *entity.Report
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		// Validate subject existence + build the minimal immutable evidence
		// snapshot of the subject at report time.
		snapshot, err := s.repo.ValidateTarget(ctx, tx, input.SubjectType, input.SubjectID)
		if err != nil {
			var notFound *repository.ErrReportTargetNotFound
			if errors.As(err, &notFound) {
				return err
			}
			return fmt.Errorf("failed to validate report target: %w", err)
		}

		// Self-report DENY (Owner decision, Business Truth §6).
		// A user may not report their own subject.
		if snapshot.AuthorID != "" && snapshot.AuthorID == input.ReporterID.String() {
			return &ErrSelfReportDenied{SubjectType: input.SubjectType}
		}

		// Early duplicate check for UX; the unique index remains the final guard.
		hasReported, err := s.repo.HasUserReported(ctx, tx, input.ReporterID, input.SubjectType, input.SubjectID)
		if err != nil {
			return fmt.Errorf("failed to check existing report: %w", err)
		}
		if hasReported {
			return &repository.ErrDuplicateReport{
				ReporterID:  input.ReporterID,
				SubjectType: input.SubjectType,
				SubjectID:   input.SubjectID,
			}
		}

		report = entity.NewReport(
			input.ReporterID,
			input.SubjectType,
			input.SubjectID,
			input.ReasonCode,
			input.ReasonNote,
			snapshot,
		)

		if err := s.repo.Create(ctx, tx, report); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return report, nil
}

// GetReport retrieves a Report by ID. Returns (nil, nil) when not found.
func (s *ReportService) GetReport(ctx context.Context, reportID uuid.UUID) (*entity.Report, error) {
	var report *entity.Report
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		report, err = s.repo.GetByID(ctx, tx, reportID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return report, nil
}

// ListReportsByReporter retrieves Reports created by a specific reporter.
func (s *ReportService) ListReportsByReporter(ctx context.Context, reporterID uuid.UUID, limit, offset int) ([]*entity.Report, error) {
	var reports []*entity.Report
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		reports, err = s.repo.ListByReporter(ctx, tx, reporterID, limit, offset)
		return err
	})
	if err != nil {
		return nil, err
	}
	return reports, nil
}

// ErrSelfReportDenied is returned when a user tries to report their own subject.
// Owner decision (Business Truth §6): self-report is DENIED for v1.
type ErrSelfReportDenied struct {
	SubjectType entity.ReportTargetType
}

func (e *ErrSelfReportDenied) Error() string {
	return "you cannot report your own " + string(e.SubjectType)
}

// IsDuplicateReport reports whether err is a duplicate-report domain error.
func IsDuplicateReport(err error) bool {
	if err == nil {
		return false
	}
	var dup *repository.ErrDuplicateReport
	if errors.As(err, &dup) {
		return true
	}
	return strings.Contains(err.Error(), "already reported")
}
