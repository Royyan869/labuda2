package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/internal/governance/moderation/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
)

// DecisionService is the canonical Decision service.
//
// SLICE 4: The single authority for Decision creation and lifecycle.
// Decision is an immutable historical governance record.
//
// Canonical invariant: Decision is append-only (immutable).
// The trg_decisions_immutable trigger is the final DB guard.
//
// Transaction boundary for CreateDecision:
//
//	BEGIN
//	  validate Case exists
//	  INSERT immutable Decision
//	  if Case is open → resolve Case (open → resolved)
//	  if Case is already resolved → no-op on Case
//	COMMIT
type DecisionService struct {
	db       Transactor
	caseRepo repository.CaseRepository
	decRepo  repository.DecisionRepository
}

// NewDecisionService creates the canonical Decision service.
func NewDecisionService(db Transactor, caseRepo repository.CaseRepository, decRepo repository.DecisionRepository) *DecisionService {
	return &DecisionService{
		db:       db,
		caseRepo: caseRepo,
		decRepo:  decRepo,
	}
}

// CreateDecisionInput contains the parameters for creating a Decision.
type CreateDecisionInput struct {
	CaseID       uuid.UUID
	DecidedBy    uuid.UUID
	Outcome      entity.DecisionOutcome
	DecisionNote *string
}

// CreateDecision creates an immutable Decision against a Case.
//
// Business rules:
//   - Case must exist (ErrDecisionCaseNotFound if not)
//   - Outcome must be valid (no_violation or violation)
//   - Decision is created regardless of Case status (open or resolved)
//   - If Case is open → resolved atomically with Decision creation
//   - If Case is already resolved → Case stays resolved (no-op)
//   - Decision is immutable: no update, no delete
//
// Transaction boundary:
// Decision + Case resolution (if needed) are in the same transaction.
// If Decision insert fails, no Case mutation occurs.
func (s *DecisionService) CreateDecision(ctx context.Context, input CreateDecisionInput) (*entity.Decision, error) {
	// Validate outcome before entering transaction.
	if !input.Outcome.IsValid() {
		return nil, &entity.ErrInvalidDecisionOutcome{Outcome: input.Outcome}
	}

	var decision *entity.Decision

	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		// 1. Validate Case exists.
		kase, err := s.caseRepo.GetByID(ctx, tx, input.CaseID)
		if err != nil {
			return fmt.Errorf("fetch case for decision failed: %w", err)
		}
		if kase == nil {
			return &entity.ErrDecisionCaseNotFound{CaseID: input.CaseID}
		}

		// 2. Create immutable Decision.
		decision, err = entity.NewDecision(
			input.CaseID,
			input.DecidedBy,
			input.Outcome,
			input.DecisionNote,
		)
		if err != nil {
			return err
		}

		if err := s.decRepo.Create(ctx, tx, decision); err != nil {
			return fmt.Errorf("insert decision failed: %w", err)
		}

		// 3. Resolve Case if open (no-op if already resolved).
		// ResolveCase uses WHERE status='open' — idempotent and safe.
		if kase.IsOpen() {
			if err := s.caseRepo.ResolveCase(ctx, tx, input.CaseID); err != nil {
				return fmt.Errorf("resolve case failed: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	return decision, nil
}

// GetDecision retrieves a Decision by ID.
func (s *DecisionService) GetDecision(ctx context.Context, decisionID uuid.UUID) (*entity.Decision, error) {
	var decision *entity.Decision
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		decision, err = s.decRepo.GetByID(ctx, tx, decisionID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return decision, nil
}

// ListDecisionsByCase retrieves all Decisions for a Case, newest first.
func (s *DecisionService) ListDecisionsByCase(ctx context.Context, caseID uuid.UUID, limit, offset int) ([]*entity.Decision, error) {
	var decisions []*entity.Decision
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		decisions, err = s.decRepo.ListByCase(ctx, tx, caseID, limit, offset)
		return err
	})
	if err != nil {
		return nil, err
	}
	return decisions, nil
}
