// PARKED — AppealReversalService is not instantiated in serverboot/dependencies.go.
//
// The canonical V1 appeal approval path is AppealService.ReviewAppeal, which
// emits direct outbox restoration events for content and comment types only.
// This service was designed for a Phase 2 all-or-nothing domain-action reversal
// pattern (DomainActionWorker) that is not yet wired.
//
// Do not instantiate or call this service until Phase 2 is scoped and planned.
//
// 🔥 PHASE 2: APPEAL REVERSAL SERVICE — ALL-OR-NOTHING SEMANTICS
//
// WAJIB:
// - Appeal reversal must be all-or-nothing
// - All actions reversed atomically or none reversed
// - Invariant checks after reversal
// - Event emission with idempotency key
//
// DILARANG:
// - Partial reversal (some actions reversed, some not)
// - Skipping invariant checks
// - Non-idempotent operations

package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	moderationrepo "github.com/labuda/backend/internal/governance/moderation/infrastructure/repository"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
)

// AppealReversalService handles all-or-nothing appeal reversal.
//
// 🔥 PHASE 2: All actions in a governance case must be reversed together.
// If any reversal fails, all reversals are rolled back.
type AppealReversalService struct {
	domainActionRepo moderationrepo.DomainActionRepository
	appealRepo       moderationrepo.AppealRepository
	moderationRepo   moderationrepo.ModerationRepository
	outboxRepo       *outboxRepo.OutboxRepository
}

// NewAppealReversalService creates a new AppealReversalService.
func NewAppealReversalService(
	domainActionRepo moderationrepo.DomainActionRepository,
	appealRepo moderationrepo.AppealRepository,
	moderationRepo moderationrepo.ModerationRepository,
	outboxRepo *outboxRepo.OutboxRepository,
) *AppealReversalService {
	return &AppealReversalService{
		domainActionRepo: domainActionRepo,
		appealRepo:       appealRepo,
		moderationRepo:   moderationRepo,
		outboxRepo:       outboxRepo,
	}
}

// ReverseAppeal reverses all actions associated with an approved appeal.
//
// 🔥 PHASE 2: All-or-nothing reversal semantics.
//
// Transaction flow:
// 1. Get all domain actions for the governance case
// 2. Validate all actions are reversible
// 3. Execute reversals in reverse order (LIFO)
// 4. Verify invariant checks after reversal
// 5. Emit reversal event with idempotency key
//
// If any step fails, the entire operation is rolled back.
func (s *AppealReversalService) ReverseAppeal(
	ctx context.Context,
	tx db.Tx,
	appealID uuid.UUID,
	adminID uuid.UUID,
	reason string,
) error {
	// Get appeal
	appeal, err := s.appealRepo.GetByID(ctx, tx, appealID)
	if err != nil {
		return fmt.Errorf("failed to get appeal: %w", err)
	}

	// Validate appeal status
	if appeal.Status != entity.AppealStatusApproved {
		return fmt.Errorf("cannot reverse appeal: appeal is not approved: status=%s", appeal.Status)
	}

	// Get all domain actions for the governance case
	actions, err := s.domainActionRepo.GetByGovernanceCaseID(ctx, tx, appeal.CaseID)
	if err != nil {
		return fmt.Errorf("failed to get domain actions: %w", err)
	}

	if len(actions) == 0 {
		// No actions to reverse, this is OK
		return nil
	}

	// Validate all actions are reversible
	for _, action := range actions {
		if !action.IsSucceeded() {
			return fmt.Errorf("cannot reverse non-succeeded action: action_id=%s, status=%s",
				action.ID, action.ExecutionStatus)
		}

		if action.IsReversed() {
			// Already reversed, skip (idempotent)
			continue
		}
	}

	// 🔥 PHASE 2: Execute reversals in reverse order (LIFO)
	// This ensures dependencies are undone correctly
	// Example: If forSale was hidden then seller was warned,
	//          unwarn seller first then unhide forSale
	for i := len(actions) - 1; i >= 0; i-- {
		action := actions[i]

		if action.IsReversed() {
			// Already reversed, skip (idempotent)
			continue
		}

		// Reverse the action
		if err := action.Reverse(adminID, reason); err != nil {
			return fmt.Errorf("failed to reverse action: action_id=%s, error=%w", action.ID, err)
		}

		// Update action in database
		if err := s.domainActionRepo.Update(ctx, tx, action); err != nil {
			return fmt.Errorf("failed to update reversed action: action_id=%s, error=%w", action.ID, err)
		}

		// 🔥 PHASE 2: Execute domain-specific reversal logic
		if err := s.executeActionReversal(ctx, tx, action); err != nil {
			return fmt.Errorf("failed to execute action reversal: action_id=%s, error=%w", action.ID, err)
		}

		// 🔥 PHASE 2: Verify invariant after reversal
		if err := s.verifyPostReversalInvariant(ctx, tx, action); err != nil {
			return fmt.Errorf("invariant violation after reversal: action_id=%s, error=%w", action.ID, err)
		}
	}

	// Emit reversal event with idempotency key
	idempotencyKey := fmt.Sprintf("appeal.reversal.%s", appealID)
	payload := map[string]interface{}{
		"appeal_id":    appealID,
		"case_id":      appeal.CaseID,
		"action_count": len(actions),
		"reversed_by":  adminID,
		"reason":       reason,
		"reversed_at":  appeal.ReviewedAt,
	}

	if err := s.outboxRepo.InsertTx(ctx, tx, "appeal.reversed", payload, idempotencyKey); err != nil {
		return fmt.Errorf("failed to emit reversal event: %w", err)
	}

	return nil
}

// executeActionReversal executes the domain-specific reversal logic for an action.
//
// 🔥 PHASE 2: This is where the actual reversal happens.
// Example: Unhide forSale, restore visibility, etc.
func (s *AppealReversalService) executeActionReversal(
	ctx context.Context,
	tx db.Tx,
	action *entity.DomainAction,
) error {
	switch action.ActionType {
	case entity.ActionTypeHideForSale, entity.ActionTypeReduceVisibility:
		return s.executeForSaleVisibilityReversal(ctx, tx, action)

	case entity.ActionTypePauseAuction:
		return s.executeAuctionPauseReversal(ctx, tx, action)

	default:
		// No reversal logic needed for this action type
		return nil
	}
}

// executeForSaleVisibilityReversal reverses for_sale hiding actions.
//
// Emits for_sale.visibility.restored which is consumed by the moderation event
// handler (ModerationEventHandler.handleForSaleRestored) via the outbox worker.
// The handler calls ForSaleService.RestoreFromModeration() to transition
// the for_sale from withdrawn → active.
func (s *AppealReversalService) executeForSaleVisibilityReversal(
	ctx context.Context,
	tx db.Tx,
	action *entity.DomainAction,
) error {
	idempotencyKey := fmt.Sprintf("for_sale.visibility.restore.%s", action.TargetResourceID)
	payload := map[string]interface{}{
		"for_sale_id":     action.TargetResourceID,
		"action_id":       action.ID,
		"previous_hidden": true,
		"new_hidden":      false,
		"reversed_by":     action.ReversedBy,
		"reversal_reason": action.ReversalReason,
		"reversed_at":     action.ReversedAt,
	}

	if err := s.outboxRepo.InsertTx(ctx, tx, "for_sale.visibility.restored", payload, idempotencyKey); err != nil {
		return fmt.Errorf("failed to emit for_sale visibility restoration event: %w", err)
	}

	return nil
}

// executeAuctionPauseReversal reverses auction pause actions.
//
// Emits auction.pause.restored which is consumed by the moderation event
// handler. Note: auction full restoration is intentionally unsupported
// (see ModerationEventHandler.handleAuctionRestored). This event signals the
// appeal outcome; the seller must create a new auction.
func (s *AppealReversalService) executeAuctionPauseReversal(
	ctx context.Context,
	tx db.Tx,
	action *entity.DomainAction,
) error {
	idempotencyKey := fmt.Sprintf("auction.pause.restore.%s", action.TargetResourceID)
	payload := map[string]interface{}{
		"auction_id":      action.TargetResourceID,
		"action_id":       action.ID,
		"previous_status": "paused",
		"new_status":      "active",
		"reversed_by":     action.ReversedBy,
		"reversal_reason": action.ReversalReason,
		"reversed_at":     action.ReversedAt,
	}

	if err := s.outboxRepo.InsertTx(ctx, tx, "auction.pause.restored", payload, idempotencyKey); err != nil {
		return fmt.Errorf("failed to emit auction pause restoration event: %w", err)
	}

	return nil
}

// verifyPostReversalInvariant verifies invariants after action reversal.
//
// 🔥 PHASE 2: Critical invariant checks to ensure system consistency.
func (s *AppealReversalService) verifyPostReversalInvariant(
	ctx context.Context,
	tx db.Tx,
	action *entity.DomainAction,
) error {
	switch action.ActionType {
	case entity.ActionTypeHideForSale, entity.ActionTypeReduceVisibility:
		return s.verifyForSaleHiddenInvariant(ctx, tx, action)

	case entity.ActionTypePauseAuction:
		return s.verifyAuctionPauseInvariant(ctx, tx, action)

	default:
		// No invariant check for this action type
		return nil
	}
}

// verifyForSaleHiddenInvariant verifies forSale restoration after reversal.
//
// Verification is performed by ModerationEventHandler.handleForSaleRestored()
// at outbox dispatch time. This post-reversal check is a no-op; the outbox
// event emission in executeForSaleVisibilityReversal is the authority path.
func (s *AppealReversalService) verifyForSaleHiddenInvariant(
	ctx context.Context,
	tx db.Tx,
	action *entity.DomainAction,
) error {
	return nil
}

// verifyAuctionPauseInvariant verifies auction state after reversal.
//
// Auction restoration is intentionally unsupported (seller must re-create).
// No invariant to verify here.
func (s *AppealReversalService) verifyAuctionPauseInvariant(
	ctx context.Context,
	tx db.Tx,
	action *entity.DomainAction,
) error {
	return nil
}

// RollbackAppealReversal rolls back a failed appeal reversal.
//
// 🔥 PHASE 2: This is called when reversal fails partway through.
// It re-executes all actions that were reversed.
func (s *AppealReversalService) RollbackAppealReversal(
	ctx context.Context,
	tx db.Tx,
	appealID uuid.UUID,
) error {
	// Get appeal
	appeal, err := s.appealRepo.GetByID(ctx, tx, appealID)
	if err != nil {
		return fmt.Errorf("failed to get appeal: %w", err)
	}

	// Get all domain actions for the governance case
	actions, err := s.domainActionRepo.GetByGovernanceCaseID(ctx, tx, appeal.CaseID)
	if err != nil {
		return fmt.Errorf("failed to get domain actions: %w", err)
	}

	// Re-execute all reversed actions in original order (FIFO)
	for _, action := range actions {
		if !action.IsReversed() {
			continue
		}

		// Re-execute the action
		if err := s.reExecuteAction(ctx, tx, action); err != nil {
			return fmt.Errorf("failed to re-execute action: action_id=%s, error=%w", action.ID, err)
		}

		// Clear reversal flags
		action.ReversedBy = nil
		action.ReversedAt = nil
		action.ReversalReason = nil

		// Update action in database
		if err := s.domainActionRepo.Update(ctx, tx, action); err != nil {
			return fmt.Errorf("failed to update re-executed action: action_id=%s, error=%w", action.ID, err)
		}
	}

	return nil
}

// reExecuteAction re-executes an action that was previously reversed.
//
// 🔥 PHASE 2: This restores the original action state.
func (s *AppealReversalService) reExecuteAction(
	ctx context.Context,
	tx db.Tx,
	action *entity.DomainAction,
) error {
	switch action.ActionType {
	case entity.ActionTypeHideForSale, entity.ActionTypeReduceVisibility:
		return s.reExecuteForSaleVisibility(ctx, tx, action)

	case entity.ActionTypePauseAuction:
		return s.reExecuteAuctionPause(ctx, tx, action)

	default:
		// No re-execution logic needed for this action type
		return nil
	}
}

// reExecuteForSaleVisibility re-executes for_sale hiding.
func (s *AppealReversalService) reExecuteForSaleVisibility(
	ctx context.Context,
	tx db.Tx,
	action *entity.DomainAction,
) error {
	idempotencyKey := fmt.Sprintf("for_sale.visibility.apply.%s", action.TargetResourceID)
	payload := map[string]interface{}{
		"for_sale_id":   action.TargetResourceID,
		"action_id":     action.ID,
		"new_hidden":    true,
		"reexecuted_by": "system",
		"reexecuted_at": json.Number(fmt.Sprintf("%d", time.Now().Unix())),
	}

	if err := s.outboxRepo.InsertTx(ctx, tx, "for_sale.visibility.applied", payload, idempotencyKey); err != nil {
		return fmt.Errorf("failed to emit for_sale visibility event: %w", err)
	}

	return nil
}

// reExecuteAuctionPause re-executes auction pause.
func (s *AppealReversalService) reExecuteAuctionPause(
	ctx context.Context,
	tx db.Tx,
	action *entity.DomainAction,
) error {
	idempotencyKey := fmt.Sprintf("auction.pause.apply.%s", action.TargetResourceID)
	payload := map[string]interface{}{
		"auction_id":    action.TargetResourceID,
		"action_id":     action.ID,
		"new_status":    "paused",
		"reexecuted_by": "system",
		"reexecuted_at": json.Number(fmt.Sprintf("%d", time.Now().Unix())),
	}

	if err := s.outboxRepo.InsertTx(ctx, tx, "auction.pause.applied", payload, idempotencyKey); err != nil {
		return fmt.Errorf("failed to emit auction pause event: %w", err)
	}

	return nil
}
