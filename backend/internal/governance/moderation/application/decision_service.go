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

// GovernanceAuditEmitter is the minimal interface for emitting governance audit events.
// Satisfied by *auditApp.AuditService.
//
// DESIGN: The audit event MUST be emitted within the same transaction as the
// Decision creation. If the audit write fails, the entire transaction rolls back.
// This ensures the invariant: either both Decision and audit event persist, or neither.
type GovernanceAuditEmitter interface {
	// GovernanceDecisionCreated emits an audit event when an admin creates a Decision.
	// Called within the same DB transaction as the Decision insert.
	// Returns an error if the audit INSERT fails — caller must propagate to roll back TX.
	// payload contains: case_id, outcome, target_type (if violation), target_id (if violation),
	// decision_note (if present).
	GovernanceDecisionCreated(
		ctx context.Context,
		tx db.Tx,
		decisionID, caseID, adminID uuid.UUID,
		outcome string,
		payload map[string]interface{},
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
	db           Transactor
	caseRepo     repository.CaseRepository
	decRepo      repository.DecisionRepository
	enfRepo      repository.EnforcementRepository
	outboxRepo   OutboxInserter
	auditEmitter GovernanceAuditEmitter
}

// NewDecisionService creates the canonical Decision service.
func NewDecisionService(
	db Transactor,
	caseRepo repository.CaseRepository,
	decRepo repository.DecisionRepository,
	enfRepo repository.EnforcementRepository,
	outboxRepo OutboxInserter,
	auditEmitter GovernanceAuditEmitter,
) *DecisionService {
	return &DecisionService{
		db:           db,
		caseRepo:     caseRepo,
		decRepo:      decRepo,
		enfRepo:      enfRepo,
		outboxRepo:   outboxRepo,
		auditEmitter: auditEmitter,
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

		// 5. Emit governance audit event within the same transaction.
		// This is MANDATORY for Decision creation — the audit event must persist
		// atomically with the Decision. If the audit write fails, the entire
		// transaction rolls back, ensuring: either both Decision + audit persist, or neither.
		if s.auditEmitter != nil {
			auditPayload := map[string]interface{}{
				"case_id": input.CaseID.String(),
				"outcome": string(input.Outcome),
			}
			if input.Outcome == entity.DecisionOutcomeViolation {
				auditPayload["target_type"] = string(input.TargetType)
				auditPayload["target_id"] = input.TargetID.String()
			}
			if input.DecisionNote != nil {
				auditPayload["decision_note"] = *input.DecisionNote
			}
			if err := s.auditEmitter.GovernanceDecisionCreated(
				ctx, tx,
				decision.ID, input.CaseID, input.DecidedBy,
				string(input.Outcome),
				auditPayload,
			); err != nil {
				return fmt.Errorf("governance audit event failed: %w", err)
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

// ============================================================================
// APPEAL DECISION #2
// ============================================================================

// CreateAppealDecisionInput contains the parameters for creating an Appeal Decision #2.
// SLICE B: Appeal review produces Decision #2 atomically.
type CreateAppealDecisionInput struct {
	CaseID       uuid.UUID   // Same Case as Decision #1
	DecidedBy    uuid.UUID   // Reviewing admin
	Outcome      entity.DecisionOutcome // no_violation (reversal) or violation (upheld)
	DecisionNote *string
	AppealID     uuid.UUID   // For audit trail

	// Enforcement target — required for reversal (no_violation outcome).
	// For upheld (violation outcome), no new Enforcement is created.
	TargetType entity.ModerationTargetType
	TargetID   uuid.UUID
}

// CreateAppealDecision creates Decision #2 for an Appeal review.
//
// SLICE B: Canonical Appeal reversal path.
//
// Transaction boundary: Accepts an existing db.Tx from the caller.
// Decision #2 + Enforcement #2 + Outbox + Audit + Appeal status update
// MUST all execute within the SAME transaction to guarantee atomicity.
// This method does NOT open a nested transaction.
//
// For reversal (outcome = no_violation):
//   - Decision #2 created (same Case, outcome = no_violation)
//   - Enforcement #2 created (for restoration)
//   - Outbox event = moderation.<type>.restored
//
// For upheld (outcome = violation):
//   - Decision #2 created (same Case, outcome = violation)
//   - NO new Enforcement (original enforcement already applied)
//   - NO outbox event
func (s *DecisionService) CreateAppealDecision(ctx context.Context, tx db.Tx, input CreateAppealDecisionInput) (*entity.Decision, error) {
	if !input.Outcome.IsValid() {
		return nil, &entity.ErrInvalidDecisionOutcome{Outcome: input.Outcome}
	}

	// For reversal, enforcement target is required.
	if input.Outcome == entity.DecisionOutcomeNoViolation {
		if !input.TargetType.IsValid() {
			return nil, &entity.ErrInvalidEnforcementTargetType{TargetType: input.TargetType}
		}
		if input.TargetID == uuid.Nil {
			return nil, fmt.Errorf("target_id is required for appeal reversal decisions")
		}
	}

	// 1. Validate Case exists.
	kase, err := s.caseRepo.GetByID(ctx, tx, input.CaseID)
	if err != nil {
		return nil, fmt.Errorf("fetch case for appeal decision failed: %w", err)
	}
	if kase == nil {
		return nil, &entity.ErrDecisionCaseNotFound{CaseID: input.CaseID}
	}

	// 2. Create immutable Decision #2.
	decision, err := entity.NewDecision(
		input.CaseID,
		input.DecidedBy,
		input.Outcome,
		input.DecisionNote,
	)
	if err != nil {
		return nil, err
	}

	if err := s.decRepo.Create(ctx, tx, decision); err != nil {
		return nil, fmt.Errorf("insert appeal decision failed: %w", err)
	}

	// 3. For reversal: create Enforcement #2 + outbox event.
	if input.Outcome == entity.DecisionOutcomeNoViolation {
		enforcement, err := entity.NewEnforcement(
			decision.ID,
			input.TargetType,
			input.TargetID,
		)
		if err != nil {
			return nil, err
		}

		if err := s.enfRepo.Create(ctx, tx, enforcement); err != nil {
			return nil, fmt.Errorf("insert appeal enforcement failed: %w", err)
		}

		// Emit outbox event for restoration.
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
				return nil, fmt.Errorf("build appeal event payload failed: %w", err)
			}

			eventType := buildRestorationEventType(input.TargetType)
			if err := s.outboxRepo.InsertEvent(ctx, tx, eventType, input.TargetID, payload); err != nil {
				return nil, fmt.Errorf("insert appeal outbox event failed: %w", err)
			}
		}
	}

	// 4. Emit governance audit event within the same transaction.
	if s.auditEmitter != nil {
		auditPayload := map[string]interface{}{
			"case_id":   input.CaseID.String(),
			"outcome":   string(input.Outcome),
			"appeal_id": input.AppealID.String(),
		}
		if input.Outcome == entity.DecisionOutcomeNoViolation {
			auditPayload["target_type"] = string(input.TargetType)
			auditPayload["target_id"] = input.TargetID.String()
		}
		if input.DecisionNote != nil {
			auditPayload["decision_note"] = *input.DecisionNote
		}
		if err := s.auditEmitter.GovernanceDecisionCreated(
			ctx, tx,
			decision.ID, input.CaseID, input.DecidedBy,
			string(input.Outcome),
			auditPayload,
		); err != nil {
			return nil, fmt.Errorf("governance audit event failed: %w", err)
		}
	}

	return decision, nil
}

// targetRestorationSuffix maps ModerationTargetType to the restoration event suffix.
var targetRestorationSuffix = map[entity.ModerationTargetType]string{
	entity.ModerationTargetTypeContent: "restored",
	entity.ModerationTargetTypeComment: "restored",
	entity.ModerationTargetTypeForSale: "restored",
	entity.ModerationTargetTypeAuction: "restored",
	entity.ModerationTargetTypeUser:    "restored",
}

// buildRestorationEventType constructs the canonical outbox event type for restoration.
func buildRestorationEventType(targetType entity.ModerationTargetType) string {
	suffix, ok := targetRestorationSuffix[targetType]
	if !ok {
		suffix = "restored"
	}
	return "moderation." + string(targetType) + "." + suffix
}
