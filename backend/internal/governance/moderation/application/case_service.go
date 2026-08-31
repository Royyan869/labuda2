package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/internal/governance/moderation/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
)

// CaseService is the canonical Case service.
//
// SLICE 3: The single authority for Case creation and lifecycle.
// Case is a governance investigation unit for one moderation subject.
//
// Canonical invariant: at most one OPEN Case per (subject_type, subject_id).
// The partial unique index uniq_active_case_per_subject is the final guard.
type CaseService struct {
	db   Transactor
	repo repository.CaseRepository
}

// NewCaseService creates the canonical Case service.
func NewCaseService(db Transactor, repo repository.CaseRepository) *CaseService {
	return &CaseService{db: db, repo: repo}
}

// CorrelateOrCreateCase finds or creates an open Case for the given subject.
// This is called within the Report creation transaction.
//
// Business rules:
//   - If an open Case exists for the subject, the new Report is correlated to it
//   - If no open Case exists, a new one is created
//   - The partial unique index ensures at most one open Case per subject
//   - Race safety: DB constraint is the final guard
//
// Returns the Case (either existing or newly created).
func (s *CaseService) CorrelateOrCreateCase(ctx context.Context, tx db.Tx, subjectType entity.ReportTargetType, subjectID uuid.UUID) (*entity.CanonicalCase, error) {
	kase, err := s.repo.FindOrCreateOpenCase(ctx, tx, subjectType, subjectID)
	if err != nil {
		return nil, fmt.Errorf("correlate or create case failed: %w", err)
	}
	return kase, nil
}

// GetCase retrieves a Case by ID.
func (s *CaseService) GetCase(ctx context.Context, caseID uuid.UUID) (*entity.CanonicalCase, error) {
	var kase *entity.CanonicalCase
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		kase, err = s.repo.GetByID(ctx, tx, caseID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return kase, nil
}

// ListCasesBySubject retrieves all Cases for a subject.
func (s *CaseService) ListCasesBySubject(ctx context.Context, subjectType entity.ReportTargetType, subjectID uuid.UUID, limit, offset int) ([]*entity.CanonicalCase, error) {
	var cases []*entity.CanonicalCase
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		cases, err = s.repo.ListBySubject(ctx, tx, subjectType, subjectID, limit, offset)
		return err
	})
	if err != nil {
		return nil, err
	}
	return cases, nil
}

// ResolveCase marks a Case as resolved.
// This is called when a Decision is made against the Case.
func (s *CaseService) ResolveCase(ctx context.Context, caseID uuid.UUID) error {
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		return s.repo.ResolveCase(ctx, tx, caseID)
	})
}
