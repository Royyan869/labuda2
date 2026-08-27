package application

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/internal/governance/moderation/infrastructure/repository"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
)

// ModerationService handles moderation case operations.
//
// DOMAIN TERMINOLOGY:
// - REPORT: User action (ingested via handler → creates CASE)
// - CASE: Internal moderation object managed by this service
// - APPEAL: User contest (handled by AppealService)
//
// STRICT BOUNDARY RULES:
// - NO direct financial mutations
// - NO ledger modifications
// - NO trade/offer/withdraw mutations
// - NO verification/rating modifications
// - Only creates cases and emits events
// - Downstream domains react to "moderation.removed" events
type ModerationService struct {
	db         Transactor
	repo       repository.ModerationRepository
	outboxRepo *outboxRepo.OutboxRepository
}

// Transactor represents the ability to execute functions within transactions.
type Transactor interface {
	WithTx(ctx context.Context, fn func(tx db.Tx) error) error
}

// NewModerationService creates a new ModerationService.
func NewModerationService(
	db Transactor,
	repo repository.ModerationRepository,
	outboxRepo *outboxRepo.OutboxRepository,
) *ModerationService {
	return &ModerationService{
		db:         db,
		repo:       repo,
		outboxRepo: outboxRepo,
	}
}

// CreateCase creates a new moderation case from a user report.
//
// DOMAIN TERMINOLOGY:
// - Input: User REPORT (reporterID, resourceType, resourceID, reason)
// - Output: Moderation CASE (GovernanceCase entity)
//
// Business rules:
// - Any user can report any resource (creates a CASE)
// - User cannot report the same resource twice (returns error)
// - Initial status is always "pending"
//
// Supported resource types:
// - content, comment, for_sale, auction, user, chat_message
//
// Report uniqueness: a user can only report a specific resource once.
// Two-layer guard:
//   1. Application check: HasUserReportedResource() (early UX rejection)
//   2. DB unique index: idx_moderation_cases_one_report_per_user on
//      (reported_by, resource_type, resource_id) — migration 000207.
//      This is the final guard against concurrent duplicate inserts.
//
// Validation:
// - Resource must exist (for content, comment, user, chat_message)
// - For chat_message: reporter must be room participant, room must not be support type
// - User must not have already reported this resource (duplicate check)
func (s *ModerationService) CreateCase(
	ctx context.Context,
	resourceType entity.ResourceType,
	resourceID uuid.UUID,
	reporterID uuid.UUID,
	reason string,
) (*entity.GovernanceCase, error) {
	var kase *entity.GovernanceCase

	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		// Validate resource type
		if !resourceType.IsValid() {
			return fmt.Errorf("invalid resource type: %s. Supported types: content, comment, for_sale, auction, user, chat_message", resourceType)
		}

		// Supported: content, comment, for_sale, auction, user, chat_message
		if resourceType != entity.ResourceTypeContent &&
			resourceType != entity.ResourceTypeComment &&
			resourceType != entity.ResourceTypeForSale &&
			resourceType != entity.ResourceTypeAuction &&
			resourceType != entity.ResourceTypeUser &&
			resourceType != entity.ResourceTypeChatMessage {
			return fmt.Errorf("resource type '%s' is not yet supported. Supported: content, comment, for_sale, auction, user, chat_message", resourceType)
		}

		// Check if resource exists (for supported types)
		resourceExists, err := s.repo.ResourceExists(ctx, tx, resourceType, resourceID)
		if err != nil {
			return fmt.Errorf("failed to check resource existence: %w", err)
		}
		if !resourceExists {
			return fmt.Errorf("resource not found: %s with id %s", resourceType, resourceID)
		}

		// chat_message: validate reporter is room participant + not support room
		if resourceType == entity.ResourceTypeChatMessage {
			authorized, rejectReason, err := s.repo.ValidateChatMessageReporter(ctx, tx, resourceID, reporterID)
			if err != nil {
				return fmt.Errorf("failed to validate chat message reporter: %w", err)
			}
			if !authorized {
				return fmt.Errorf("chat message report rejected: %s", rejectReason)
			}
		}

		// Check if user has already reported this resource
		hasReported, err := s.repo.HasUserReportedResource(ctx, tx, reporterID, resourceType, resourceID)
		if err != nil {
			return fmt.Errorf("failed to check existing reports: %w", err)
		}
		if hasReported {
			return fmt.Errorf("you have already reported this %s", resourceType)
		}

		// Create new case
		kase = entity.NewGovernanceCase(resourceType, resourceID, reporterID, reason)

		// Persist
		if err := s.repo.Create(ctx, tx, kase); err != nil {
			return fmt.Errorf("failed to create moderation case: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return kase, nil
}

// ReviewCase reviews a pending moderation case.
//
// Transaction flow:
// 1. BEGIN
// 2. Lock case FOR UPDATE (prevents double-review)
// 3. Apply decision via entity transition
// 4. If decision == removed: emit outbox event
// 5. Update case
// 6. COMMIT
//
// Business rules:
// - Only pending cases can be reviewed
// - Approved/Rejected: no downstream action
// - Removed: emits "moderation.removed" event
func (s *ModerationService) ReviewCase(
	ctx context.Context,
	caseID uuid.UUID,
	adminID uuid.UUID,
	decision entity.Decision,
	note *string,
) (*entity.GovernanceCase, error) {
	var kase *entity.GovernanceCase

	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		// Lock case for update (prevents double-review)
		reviewedCase, err := s.repo.GetForUpdate(ctx, tx, caseID)
		if err != nil {
			return fmt.Errorf("failed to get case for review: %w", err)
		}

		// Apply decision
		switch decision {
		case entity.DecisionApprove:
			if err := reviewedCase.Approve(adminID, note); err != nil {
				return err
			}
		case entity.DecisionReject:
			if err := reviewedCase.Reject(adminID, note); err != nil {
				return err
			}
		case entity.DecisionEnforce:
			if err := reviewedCase.Enforce(adminID, note); err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid decision: %s", decision)
		}

		// If removed, emit outbox event for downstream soft-delete
		if reviewedCase.ShouldEmitEnforcementEvents() {
			payload := s.buildRemovedPayload(reviewedCase)
			eventType := s.getRemovedEventType(reviewedCase.ResourceType)

			if err := s.outboxRepo.InsertEvent(
				ctx, tx,
				eventType,
				reviewedCase.ResourceID,
				payload,
			); err != nil {
				return fmt.Errorf("failed to insert removal event: %w", err)
			}
		}

		// Update case
		if err := s.repo.Update(ctx, tx, reviewedCase); err != nil {
			return fmt.Errorf("failed to update moderation case: %w", err)
		}

		kase = reviewedCase
		return nil
	})

	if err != nil {
		return nil, err
	}

	return kase, nil
}

// ListPendingCases retrieves pending cases awaiting review.
// Ordered by created_at ASC (oldest first).
func (s *ModerationService) ListPendingCases(
	ctx context.Context,
	limit, offset int,
) ([]*entity.GovernanceCase, error) {
	var cases []*entity.GovernanceCase

	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		cases, err = s.repo.ListPending(ctx, tx, limit, offset)
		return err
	})

	if err != nil {
		return nil, err
	}

	return cases, nil
}

// GetCasesByResource retrieves all cases for a specific resource.
// Useful for checking moderation history.
func (s *ModerationService) GetCasesByResource(
	ctx context.Context,
	resourceType entity.ResourceType,
	resourceID uuid.UUID,
) ([]*entity.GovernanceCase, error) {
	var cases []*entity.GovernanceCase

	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		cases, err = s.repo.ListByResource(ctx, tx, resourceType, resourceID)
		return err
	})

	if err != nil {
		return nil, err
	}

	return cases, nil
}

// GetCase retrieves a case by ID.
func (s *ModerationService) GetCase(
	ctx context.Context,
	caseID uuid.UUID,
) (*entity.GovernanceCase, error) {
	var kase *entity.GovernanceCase

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

// GetCasesByUser retrieves all moderation cases created by a specific user.
// Ordered by created_at DESC (newest first).
// Supports pagination with limit and offset.
func (s *ModerationService) GetCasesByUser(
	ctx context.Context,
	reporterID uuid.UUID,
	limit, offset int,
) ([]*entity.GovernanceCase, error) {
	var cases []*entity.GovernanceCase

	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		cases, err = s.repo.ListByReporter(ctx, tx, reporterID, limit, offset)
		return err
	})

	if err != nil {
		return nil, err
	}

	return cases, nil
}

// ListCases retrieves moderation cases with optional status and resource type filters.
// If a filter is nil, that dimension is not applied.
// Ordered by created_at ASC (oldest first).
// Supports pagination with limit and offset.
func (s *ModerationService) ListCases(
	ctx context.Context,
	statusFilter *entity.GovernanceCaseStatus,
	resourceTypeFilter *entity.ResourceType,
	limit, offset int,
) ([]*entity.GovernanceCase, int64, error) {
	var cases []*entity.GovernanceCase
	var total int64

	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		cases, total, err = s.repo.ListWithStatus(ctx, tx, statusFilter, resourceTypeFilter, limit, offset)
		return err
	})

	if err != nil {
		return nil, 0, err
	}

	return cases, total, nil
}

// buildRemovedPayload creates the JSON payload for a removal event.
//
// Event format:
//
//	{
//	  "case_id": "uuid",
//	  "resource_type": "content|comment|...",
//	  "resource_id": "uuid",
//	  "decision_note": "optional note"
//	}
func (s *ModerationService) buildRemovedPayload(kase *entity.GovernanceCase) []byte {
	type payload struct {
		CaseID       string  `json:"case_id"`
		ResourceType string  `json:"resource_type"`
		ResourceID   string  `json:"resource_id"`
		DecisionNote *string `json:"decision_note,omitempty"`
	}
	p := payload{
		CaseID:       kase.ID.String(),
		ResourceType: string(kase.ResourceType),
		ResourceID:   kase.ResourceID.String(),
		DecisionNote: kase.DecisionNote,
	}
	b, _ := json.Marshal(p)
	return b
}

// getRemovedEventType returns the event type for an enforcement action.
//
// Format: "moderation.<resource_type>.removed" for content/comment/for_sale/auction.
// Special case: user enforcement emits "moderation.user.suspended" because
// suspension is semantically distinct from removal and the outbox handler
// (outbox_worker.go) is registered under that event type.
//
// Examples:
//   - moderation.content.removed
//   - moderation.comment.removed
//   - moderation.user.suspended
func (s *ModerationService) getRemovedEventType(resourceType entity.ResourceType) string {
	switch resourceType {
	case entity.ResourceTypeUser:
		return "moderation.user.suspended"
	case entity.ResourceTypeChatMessage:
		return "moderation.chat_message.hidden"
	default:
		return fmt.Sprintf("moderation.%s.removed", resourceType)
	}
}


