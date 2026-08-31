package application

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/internal/governance/moderation/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
)

// OutboxInserter is the minimal interface for emitting outbox events.
// Satisfied by *outboxRepo.OutboxRepository.
type OutboxInserter interface {
	InsertEvent(
		ctx context.Context,
		tx db.Tx,
		eventType string,
		aggregateID uuid.UUID,
		payload []byte,
	) error
}

// DecisionService is the canonical Decision service.
//
// SLICE 5: The single authority for Decision creation and lifecycle.
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
//	  if outcome = violation: INSERT Enforcement + INSERT outbox event
//	  if Case is open → resolve Case (open → resolved)
//	  if Case is already resolved → no-op on Case
//	COMMIT
type DecisionService struct {
	db          Transactor
	caseRepo    repository.CaseRepository
	decRepo     repository.DecisionRepository
	enfRepo     repository.EnforcementRepository
	outboxRepo  OutboxInserter
}

// NewDecisionService creates the canonical Decision service.
func NewDecisionService(
	db Transactor,
	caseRepo repository.CaseRepository,
	decRepo repository.DecisionRepository,
	enfRepo repository.EnforcementRepository,
	outboxRepo OutboxInserter,
) *DecisionService {
	return &DecisionService{
		db:         db,
		caseRepo:   caseRepo,
		decRepo:    decRepo,
		enfRepo:    enfRepo,
		outboxRepo: outboxRepo,
	}
}

// CreateDecisionInput contains the parameters for creating a Decision.
type CreateDecisionInput struct {
	CaseID       uuid.UUID
	DecidedBy    uuid.UUID
	Outcome      entity.DecisionOutcome
	DecisionNote *string

	// Enforcement target — required when Outcome = violation.
	// The Enforcement is created atomically with the Decision.
	TargetType entity.ModerationTargetType
	TargetID   uuid.UUID
}

// CreateDecision creates an immutable Decision against a Case.
//
// Business rules:
//   - Case must exist (ErrDecisionCaseNotFound if not)
//   - Outcome must be valid (no_violation or violation)
//   - If outcome = violation: TargetType and TargetID are required
//   - Decision is created regardless of Case status (open or resolved)
//   - If Case is open → resolved atomically with Decision creation
//   - If Case is already resolved → Case stays resolved (no-op)
//   - Decision is immutable: no update, no delete
//   - If outcome = violation: Enforcement + outbox event created atomically
//
// Transaction boundary:
// Decision + Enforcement + Outbox + Case resolution (if needed) are atomic.
// If any insert fails, everything rolls back.
func (s *DecisionService) CreateDecision(ctx context.Context, input CreateDecisionInput) (*entity.Decision, error) {
	// Validate outcome before entering transaction.
	if !input.Outcome.IsValid() {
		return nil, &entity.ErrInvalidDecisionOutcome{Outcome: input.Outcome}
	}

	// Validate enforcement target for violation decisions.
	if input.Outcome == entity.DecisionOutcomeViolation {
		if !input.TargetType.IsValid() {
			return nil, &entity.ErrInvalidEnforcementTargetType{TargetType: input.TargetType}
		}
		if input.TargetID == uuid.Nil {
			return nil, fmt.Errorf("target_id is required for violation decisions")
		}
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

		// 3. If violation: create Enforcement + outbox event atomically.
		if input.Outcome == entity.DecisionOutcomeViolation {
			enforcement, err := entity.NewEnforcement(
				decision.ID,
				input.TargetType,
				input.TargetID,
			)
			if err != nil {
				return err
			}

			if err := s.enfRepo.Create(ctx, tx, enforcement); err != nil {
				return fmt.Errorf("insert enforcement failed: %w", err)
			}				// Emit outbox event for enforcement execution.
				if s.outboxRepo != nil {
					payload, err := buildModerationEventPayload(
						decision.ID,
						enforcement.ID,
						input.CaseID,
						string(input.TargetType),
						input.TargetID,
						input.DecisionNote,
					)
					if err != nil {
						return fmt.Errorf("build moderation event payload failed: %w", err)
					}

					eventType, err := buildModerationEventType(input.TargetType)
					if err != nil {
						return fmt.Errorf("build moderation event type failed: %w", err)
					}
					if err := s.outboxRepo.InsertEvent(ctx, tx, eventType, input.TargetID, payload); err != nil {
						return fmt.Errorf("insert outbox event failed: %w", err)
					}
				}
		}

		// 4. Resolve Case if open (no-op if already resolved).
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

// moderationEventPayload is the canonical payload for moderation.outbox events.
// The worker (ModerationEventHandler) reads this to execute the target mutation.
type moderationEventPayload struct {
	DecisionID   string  `json:"decision_id"`
	EnforcementID string `json:"enforcement_id"`
	CaseID       string  `json:"case_id"`
	ResourceType string  `json:"resource_type"`
	ResourceID   string  `json:"resource_id"`
	DecisionNote *string `json:"decision_note,omitempty"`
}

// targetEventSuffix maps ModerationTargetType to the canonical outbox event suffix.
// This mapping MUST stay in sync with SetupModerationHandlers event registration.
// content/comment/for_sale/auction use "removed"; user uses "suspended".
var targetEventSuffix = map[entity.ModerationTargetType]string{
	entity.ModerationTargetTypeContent: "removed",
	entity.ModerationTargetTypeComment: "removed",
	entity.ModerationTargetTypeForSale: "removed",
	entity.ModerationTargetTypeAuction: "removed",
	entity.ModerationTargetTypeUser:    "suspended",
}

// buildModerationEventType constructs the canonical outbox event type for a given target.
func buildModerationEventType(targetType entity.ModerationTargetType) (string, error) {
	suffix, ok := targetEventSuffix[targetType]
	if !ok {
		return "", fmt.Errorf("no event suffix for target type: %s", targetType)
	}
	return "moderation." + string(targetType) + "." + suffix, nil
}

// buildModerationEventPayload constructs the JSON payload for a moderation outbox event.
func buildModerationEventPayload(
	decisionID, enforcementID, caseID uuid.UUID,
	resourceType string,
	resourceID uuid.UUID,
	decisionNote *string,
) ([]byte, error) {
	payload := moderationEventPayload{
		DecisionID:    decisionID.String(),
		EnforcementID: enforcementID.String(),
		CaseID:        caseID.String(),
		ResourceType:  resourceType,
		ResourceID:    resourceID.String(),
		DecisionNote:  decisionNote,
	}
	return json.Marshal(payload)
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
