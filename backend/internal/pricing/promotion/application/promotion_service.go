package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/labuda/backend/internal/pricing/promotion/infrastructure/repository"
	promotionRepo "github.com/labuda/backend/internal/pricing/promotion/repository"
	"github.com/labuda/backend/pkg/db"
)

// ExternalProductOutboxEmitter is the minimal outbox surface required by
// PromotionService to emit review-decision events in the same transaction.
// Satisfied by *outboxrepo.OutboxRepository.
type ExternalProductOutboxEmitter interface {
	InsertEvent(
		ctx context.Context,
		tx db.Tx,
		eventType string,
		entityID uuid.UUID,
		payload []byte,
	) error
}

// PromotionService handles promotion business operations.
//
// This service manages:
// - Package purchase (creates ownership)
// - Promotion activation (creates instance)
// - Promotion deactivation (stops instance)
// - Promotion reassignment (moves remaining duration to new target)
// - Discovery of promoted items for search/home surfaces
//
// Business truth: Duration lives ONLY at ownership level.
// Instances are pointers that consume duration from their ownership.
type PromotionService struct {
	repo               promotionRepo.PromotionRepository
	operabilityChecker OperabilityChecker
	outboxEmitter      ExternalProductOutboxEmitter // optional; nil = no event emission
}

// ExternalProductPromotionReviewRequiredError is returned when callers try to
// activate or reassign an external product while the public review workflow is
// not yet available.
type ExternalProductPromotionReviewRequiredError struct{}

func (e *ExternalProductPromotionReviewRequiredError) Error() string {
	return "external_product promotions are temporarily blocked until review workflow is available"
}

// NewPromotionService creates a new PromotionService.
func NewPromotionService(
	operabilityChecker OperabilityChecker,
) *PromotionService {
	return &PromotionService{
		repo:               repository.NewPromotionRepository(),
		operabilityChecker: operabilityChecker,
	}
}

// NewPromotionServiceWithRepo creates a PromotionService with an injected
// repository. Intended for unit tests that need to substitute a mock or stub
// repository without relying on a live database connection.
func NewPromotionServiceWithRepo(
	repo promotionRepo.PromotionRepository,
	operabilityChecker OperabilityChecker,
) *PromotionService {
	return &PromotionService{
		repo:               repo,
		operabilityChecker: operabilityChecker,
	}
}

// SetOutboxEmitter wires the outbox emitter for external-product review-decision
// events. Must be called before any admin review action to enable notification
// delivery. Safe to call multiple times (last write wins).
func (s *PromotionService) SetOutboxEmitter(emitter ExternalProductOutboxEmitter) {
	s.outboxEmitter = emitter
}

// emitExternalProductReviewEventTx inserts a review-decision outbox event in
// the same transaction as the state change. InsertEvent failure propagates and
// rolls back the transaction — no state flip commits without its notification event.
func (s *PromotionService) emitExternalProductReviewEventTx(
	ctx context.Context,
	tx db.Tx,
	eventType string,
	product *entity.ExternalProduct,
	reason *string,
	adminID uuid.UUID,
) error {
	if s.outboxEmitter == nil {
		return nil
	}
	payload := map[string]interface{}{
		"owner_user_id":       product.OwnerUserID.String(),
		"external_product_id": product.ID.String(),
		"title":               product.Title,
		"review_status":       string(product.ReviewStatus),
		"reviewed_by":         adminID.String(),
	}
	if reason != nil && *reason != "" {
		payload["reason"] = *reason
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("promotion: marshal external_product review event payload: %w", err)
	}
	if err := s.outboxEmitter.InsertEvent(ctx, tx, eventType, product.ID, payloadBytes); err != nil {
		return fmt.Errorf("outbox %s: %w", eventType, err)
	}
	return nil
}

// externalProductReviewEventType maps an applyAdminReview action string to its
// canonical outbox event type. Returns "" if the action has no notification event.
func externalProductReviewEventType(action string) string {
	switch action {
	case "approve":
		return "external_product.review.approved"
	case "reject":
		return "external_product.review.rejected"
	case "request_changes":
		return "external_product.review.request_changes"
	case "hide":
		return "external_product.review.hidden"
	default:
		return ""
	}
}

// OperabilityChecker defines the interface for checking if targets are still operable.
// Canonical implementation: OperabilityCheckerImpl in operability_checker.go.
type OperabilityChecker interface {
	// CheckOperability checks if a target is still operable for promotion.
	// Returns (isOperable, reason, error).
	//
	// For fixed-price sale: returns false if sold, unavailable, hidden, deleted, moderated, expired
	// For auction: returns false if ended, cancelled, deleted, moderated
	// For external_product: always returns true (user must manually stop)
	CheckOperability(ctx context.Context, targetType entity.TargetType, targetID *uuid.UUID) (bool, string, error)

	// ValidateOwnership checks if the user owns the target they want to promote.
	// Returns error if user doesn't own the target.
	//
	// For fixed-price sale/auction: validates user is the seller
	// For external_product: always returns nil (no ownership check needed)
	ValidateOwnership(ctx context.Context, userID uuid.UUID, targetType entity.TargetType, targetID *uuid.UUID) error

	// CheckUserEligibility checks if a user is eligible for promotion discovery.
	// Returns (isEligible, reason, error).
	//
	// This gates ALL promotion types including external products where there is
	// no target entity to derive the seller from. Checks:
	// - Account is active (not suspended, banned, or removed)
	// - User is not soft-deleted
	CheckUserEligibility(ctx context.Context, userID uuid.UUID) (bool, string, error)
}

// ========================================================================
// PURCHASE PACKAGE
// ========================================================================

// PurchasePackageInput contains the parameters for purchasing a package.
type PurchasePackageInput struct {
	UserID    uuid.UUID
	PackageID uuid.UUID
	BillingID uuid.UUID // source billing transaction — stored on ownership for traceability
}

// PurchasePackageResult contains the result of purchasing a package.
type PurchasePackageResult struct {
	Ownership *entity.PromotionOwnership
	Package   *entity.PromotionPackage
}

// PurchasePackage creates a new promotion ownership from a package purchase.
// This is typically called after payment is completed.
func (s *PromotionService) PurchasePackage(
	ctx context.Context,
	tx db.Tx,
	input PurchasePackageInput,
) (*PurchasePackageResult, error) {
	// CRITICAL: Get database time for all accounting operations
	dbTime, err := s.repo.GetDBTime(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to get database time: %w", err)
	}

	// Get the package
	pkg, err := s.repo.GetPackageByID(ctx, tx, input.PackageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get package: %w", err)
	}
	if pkg == nil {
		return nil, &PackageNotFoundError{PackageID: input.PackageID}
	}
	if !pkg.IsActive {
		return nil, &PackageInactiveError{PackageID: input.PackageID}
	}

	// Create ownership with database time
	ownership, err := entity.NewPromotionOwnership(
		input.UserID,
		pkg.ID,
		pkg.TotalDurationHours,
		pkg.ValidityWindowHours,
		dbTime,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ownership: %w", err)
	}

	// Record source billing for traceability and duplicate-ownership prevention.
	// The DB unique index on source_billing_id rejects concurrent duplicate inserts.
	if input.BillingID != uuid.Nil {
		ownership.SourceBillingID = &input.BillingID
	}

	// Persist ownership
	err = s.repo.CreateOwnership(ctx, tx, ownership)
	if err != nil {
		return nil, fmt.Errorf("failed to persist ownership: %w", err)
	}

	return &PurchasePackageResult{
		Ownership: ownership,
		Package:   pkg,
	}, nil
}

// ========================================================================
// ACTIVATE PROMOTION
// ========================================================================

// ActivatePromotionInput contains the parameters for activating a promotion.
type ActivatePromotionInput struct {
	OwnershipID uuid.UUID
	UserID      uuid.UUID
	TargetType  entity.TargetType
	TargetID    *uuid.UUID
}

// ActivatePromotionResult contains the result of activating a promotion.
type ActivatePromotionResult struct {
	Instance  *entity.PromotionInstance
	Ownership *entity.PromotionOwnership
}

// ActivatePromotion activates a promotion on a target.
// Validates ownership, target validity, and creates an active instance.
func (s *PromotionService) ActivatePromotion(
	ctx context.Context,
	tx db.Tx,
	input ActivatePromotionInput,
) (*ActivatePromotionResult, error) {
	// CRITICAL: Get database time for all accounting operations
	dbTime, err := s.repo.GetDBTime(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to get database time: %w", err)
	}

	// Get ownership with lock
	ownership, err := s.repo.GetOwnershipForUpdate(ctx, tx, input.OwnershipID)
	if err != nil {
		return nil, fmt.Errorf("failed to get ownership: %w", err)
	}
	if ownership == nil {
		return nil, &OwnershipNotFoundError{OwnershipID: input.OwnershipID}
	}

	// Verify user owns this ownership
	if ownership.UserID != input.UserID {
		return nil, &entity.NotOwnershipOwnerError{OwnershipID: input.OwnershipID, UserID: input.UserID}
	}

	// Check if ownership can be activated
	if !ownership.CanActivate(dbTime) {
		if ownership.IsExpired(dbTime) {
			return nil, &entity.OwnershipExpiredError{OwnershipID: ownership.ID, ExpiresAt: ownership.ExpiresAt}
		}
		if ownership.IsFullyConsumed() {
			return nil, &entity.OwnershipConsumedError{OwnershipID: ownership.ID}
		}
		return nil, &OwnershipNotAvailableError{OwnershipID: ownership.ID, Status: ownership.Status}
	}

	// Get package to check allowed target types
	pkg, err := s.repo.GetPackageByID(ctx, tx, ownership.PackageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get package: %w", err)
	}
	if pkg == nil {
		return nil, &PackageNotFoundError{PackageID: ownership.PackageID}
	}

	// Validate target type is allowed
	if !pkg.AllowsTargetType(input.TargetType) {
		return nil, &TargetTypeNotAllowedError{
			TargetType:   input.TargetType,
			AllowedTypes: pkg.AllowedTargetTypes,
		}
	}

	if input.TargetType.RequiresTargetID() && input.TargetID == nil {
		return nil, &entity.ValidationError{Field: "target_id", Message: "required for this target type"}
	}

	// Validate target ownership and operability.
	if input.TargetType.RequiresTargetID() && input.TargetID != nil {
		err = s.operabilityChecker.ValidateOwnership(ctx, input.UserID, input.TargetType, input.TargetID)
		if err != nil {
			return nil, fmt.Errorf("ownership validation failed: %w", err)
		}

		// Check operability
		isOperable, reason, err := s.operabilityChecker.CheckOperability(ctx, input.TargetType, input.TargetID)
		if err != nil {
			return nil, fmt.Errorf("operability check failed: %w", err)
		}
		if !isOperable {
			return nil, &TargetNotOperableError{
				TargetType: input.TargetType,
				TargetID:   *input.TargetID,
				Reason:     reason,
			}
		}
	}

	// Check if there's already an active instance for this ownership
	// IMPORTANT: Use GetActiveInstanceByOwnershipForUpdate to lock the row
	// This prevents race conditions where two concurrent activations could both
	// see no active instance and create duplicates.
	activeInstance, err := s.repo.GetActiveInstanceByOwnershipForUpdate(ctx, tx, ownership.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to check active instance: %w", err)
	}
	if activeInstance != nil {
		// Stop the existing instance first
		err = s.DeactivatePromotion(ctx, tx, DeactivatePromotionInput{
			InstanceID: activeInstance.ID,
			UserID:     input.UserID,
			Reason:     entity.StopReasonUserCancelled,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to stop existing instance: %w", err)
		}
	}

	// CRITICAL: Check if there's already an active promotion for this target
	// This enforces the "1 fixed-price sale = 1 active promotion" rule
	// This check prevents race conditions before the DB constraint catches it
	if input.TargetType.RequiresTargetID() && input.TargetID != nil {
		hasExisting, err := s.repo.HasActivePromotionForTarget(ctx, tx, input.TargetType, *input.TargetID)
		if err != nil {
			return nil, fmt.Errorf("failed to check existing promotion for target: %w", err)
		}
		if hasExisting {
			return nil, &PromotionAlreadyActiveForTargetError{
				TargetType: input.TargetType,
				TargetID:   *input.TargetID,
			}
		}
	}

	// Create new instance with database time
	instance, err := entity.NewPromotionInstance(
		ownership.ID,
		input.UserID,
		input.TargetType,
		input.TargetID,
		dbTime,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create instance: %w", err)
	}

	// Activate the instance with database time
	err = instance.Activate(dbTime)
	if err != nil {
		return nil, fmt.Errorf("failed to activate instance: %w", err)
	}

	// Persist instance
	err = s.repo.CreateInstance(ctx, tx, instance)
	if err != nil {
		return nil, fmt.Errorf("failed to persist instance: %w", err)
	}

	return &ActivatePromotionResult{
		Instance:  instance,
		Ownership: ownership,
	}, nil
}

// ========================================================================
// DEACTIVATE PROMOTION
// ========================================================================

// DeactivatePromotionInput contains the parameters for deactivating a promotion.
type DeactivatePromotionInput struct {
	InstanceID uuid.UUID
	UserID     uuid.UUID
	Reason     entity.StopReason
}

// DeactivatePromotionResult contains the result of deactivating a promotion.
type DeactivatePromotionResult struct {
	Instance *entity.PromotionInstance
}

// DeactivatePromotion stops or pauses an active promotion.
//
// Behavior depends on reason:
//   - user_paused: PAUSES the instance (non-terminal, no finalization, no duration bake).
//     Duration clock stops. Instance can be resumed later.
//   - user_cancelled (or any other reason): STOPS the instance (terminal, finalized).
//     Duration is snapshotted and baked into ownership.
//
// The remaining duration stays in the ownership and can be reused.
func (s *PromotionService) DeactivatePromotion(
	ctx context.Context,
	tx db.Tx,
	input DeactivatePromotionInput,
) error {
	// user_paused → delegate to PausePromotion (non-terminal, no finalization)
	if input.Reason == entity.StopReasonUserPaused {
		_, err := s.PausePromotion(ctx, tx, PausePromotionInput{
			InstanceID: input.InstanceID,
			UserID:     input.UserID,
		})
		return err
	}

	// All other reasons → terminal Stop + finalization
	return s.stopAndFinalizeInstance(ctx, tx, input.InstanceID, input.UserID, input.Reason)
}

// stopAndFinalizeInstance is the canonical terminal stop path.
// Stop → Snapshot → Bake → Persist.
func (s *PromotionService) stopAndFinalizeInstance(
	ctx context.Context,
	tx db.Tx,
	instanceID uuid.UUID,
	userID uuid.UUID,
	reason entity.StopReason,
) error {
	// CRITICAL: Get database time for all accounting operations
	dbTime, err := s.repo.GetDBTime(ctx, tx)
	if err != nil {
		return fmt.Errorf("failed to get database time: %w", err)
	}

	// Get instance with row lock to serialize concurrent finalizers.
	instance, err := s.repo.GetInstanceForUpdate(ctx, tx, instanceID)
	if err != nil {
		return fmt.Errorf("failed to get instance: %w", err)
	}
	if instance == nil {
		return &InstanceNotFoundError{InstanceID: instanceID}
	}

	// Verify user owns this instance
	if instance.UserID != userID {
		return &NotInstanceOwnerError{InstanceID: instanceID, UserID: userID}
	}

	// Check if instance can be stopped
	if instance.Status.IsTerminal() {
		return &InstanceAlreadyStoppedError{InstanceID: instanceID, Status: instance.Status}
	}

	// CRITICAL: Prevent modification of already finalized instances
	// Finalized instances have their duration baked into ownership and cannot be modified
	if instance.Finalized {
		return &InstanceAlreadyFinalizedError{InstanceID: instanceID}
	}

	// Stop the instance with database time
	err = instance.Stop(reason, dbTime)
	if err != nil {
		return fmt.Errorf("failed to stop instance: %w", err)
	}

	// Snapshot consumed duration and mark as finalized with database time
	consumedSeconds := instance.SnapshotConsumedDuration(dbTime)

	// Add consumed duration to ownership (bake it in)
	err = s.repo.AddConsumedDurationToOwnership(ctx, tx, instance.OwnershipID, consumedSeconds)
	if err != nil {
		return fmt.Errorf("failed to add consumed duration to ownership: %w", err)
	}

	// Persist instance changes (including finalized flag)
	err = s.repo.UpdateInstance(ctx, tx, instance)
	if err != nil {
		return fmt.Errorf("failed to update instance: %w", err)
	}

	return nil
}

// ========================================================================
// PAUSE PROMOTION
// ========================================================================

// PausePromotionInput contains the parameters for pausing a promotion.
type PausePromotionInput struct {
	InstanceID uuid.UUID
	UserID     uuid.UUID
}

// PausePromotionResult contains the result of pausing a promotion.
type PausePromotionResult struct {
	Instance *entity.PromotionInstance
}

// PausePromotion pauses an active promotion.
//
// Business truth: Promotion time must not burn while paused.
//   - Sets status to paused (NON-TERMINAL)
//   - Records PausedAt for duration calculation
//   - Does NOT snapshot consumed duration
//   - Does NOT bake into ownership
//   - Does NOT finalize
//   - Instance can be resumed later via ResumePromotion
func (s *PromotionService) PausePromotion(
	ctx context.Context,
	tx db.Tx,
	input PausePromotionInput,
) (*PausePromotionResult, error) {
	// CRITICAL: Get database time for all accounting operations
	dbTime, err := s.repo.GetDBTime(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to get database time: %w", err)
	}

	// Get instance
	instance, err := s.repo.GetInstanceByID(ctx, tx, input.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}
	if instance == nil {
		return nil, &InstanceNotFoundError{InstanceID: input.InstanceID}
	}

	// Verify user owns this instance
	if instance.UserID != input.UserID {
		return nil, &NotInstanceOwnerError{InstanceID: input.InstanceID, UserID: input.UserID}
	}

	// Check if instance can be paused (must be active)
	if instance.Status.IsTerminal() {
		return nil, &InstanceAlreadyStoppedError{InstanceID: input.InstanceID, Status: instance.Status}
	}

	if instance.Finalized {
		return nil, &InstanceAlreadyFinalizedError{InstanceID: input.InstanceID}
	}

	// Pause the instance — NO finalization, NO duration bake
	err = instance.Pause(dbTime)
	if err != nil {
		return nil, fmt.Errorf("failed to pause instance: %w", err)
	}

	// Persist instance changes (paused_at set, status=paused)
	err = s.repo.UpdateInstance(ctx, tx, instance)
	if err != nil {
		return nil, fmt.Errorf("failed to update instance: %w", err)
	}

	return &PausePromotionResult{Instance: instance}, nil
}

// ========================================================================
// RESUME PROMOTION
// ========================================================================

// ResumePromotionInput contains the parameters for resuming a paused promotion.
type ResumePromotionInput struct {
	InstanceID uuid.UUID
	UserID     uuid.UUID
}

// ResumePromotionResult contains the result of resuming a promotion.
type ResumePromotionResult struct {
	Instance *entity.PromotionInstance
}

// ResumePromotion resumes a paused promotion.
//
// Business truth: Only resume if the target is still operable.
//   - Re-checks target operability before resume
//   - Calls entity Resume() which accumulates pause duration
//   - Does NOT bake ownership duration
//   - Instance continues consuming from where it was paused
func (s *PromotionService) ResumePromotion(
	ctx context.Context,
	tx db.Tx,
	input ResumePromotionInput,
) (*ResumePromotionResult, error) {
	// CRITICAL: Get database time for all accounting operations
	dbTime, err := s.repo.GetDBTime(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to get database time: %w", err)
	}

	// Get instance
	instance, err := s.repo.GetInstanceByID(ctx, tx, input.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}
	if instance == nil {
		return nil, &InstanceNotFoundError{InstanceID: input.InstanceID}
	}

	// Verify user owns this instance
	if instance.UserID != input.UserID {
		return nil, &NotInstanceOwnerError{InstanceID: input.InstanceID, UserID: input.UserID}
	}

	// Must be paused to resume
	if instance.Status != entity.InstanceStatusPaused {
		return nil, &InstanceNotPausedError{InstanceID: input.InstanceID, Status: instance.Status}
	}

	if instance.Finalized {
		return nil, &InstanceAlreadyFinalizedError{InstanceID: input.InstanceID}
	}

	// Check ownership is still usable (not expired, not fully consumed)
	ownership, err := s.repo.GetOwnershipByID(ctx, tx, instance.OwnershipID)
	if err != nil {
		return nil, fmt.Errorf("failed to get ownership: %w", err)
	}
	if ownership == nil {
		return nil, &OwnershipNotFoundError{OwnershipID: instance.OwnershipID}
	}
	if !ownership.CanActivate(dbTime) {
		if ownership.IsExpired(dbTime) {
			return nil, &entity.OwnershipExpiredError{OwnershipID: ownership.ID, ExpiresAt: ownership.ExpiresAt}
		}
		if ownership.IsFullyConsumed() {
			return nil, &entity.OwnershipConsumedError{OwnershipID: ownership.ID}
		}
		return nil, &OwnershipNotAvailableError{OwnershipID: ownership.ID, Status: ownership.Status}
	}

	// Re-check target operability before resume
	if instance.TargetType.RequiresTargetID() && instance.TargetID != nil {
		isOperable, reason, err := s.operabilityChecker.CheckOperability(ctx, instance.TargetType, instance.TargetID)
		if err != nil {
			return nil, fmt.Errorf("operability check failed: %w", err)
		}
		if !isOperable {
			return nil, &TargetNotOperableError{
				TargetType: instance.TargetType,
				TargetID:   *instance.TargetID,
				Reason:     reason,
			}
		}
	}

	// Resume the instance — accumulates pause duration, clears PausedAt
	err = instance.Resume(dbTime)
	if err != nil {
		return nil, fmt.Errorf("failed to resume instance: %w", err)
	}

	// Persist instance changes (status=active, paused_at=nil, total_paused_duration updated)
	err = s.repo.UpdateInstance(ctx, tx, instance)
	if err != nil {
		return nil, fmt.Errorf("failed to update instance: %w", err)
	}

	return &ResumePromotionResult{Instance: instance}, nil
}

// ApplyOperabilityRecommendation executes a lifecycle recommendation produced
// by the operability checker.
func (s *PromotionService) ApplyOperabilityRecommendation(
	ctx context.Context,
	tx db.Tx,
	recommendation OperabilityRecommendation,
) error {
	if err := recommendation.Validate(); err != nil {
		return err
	}

	switch recommendation.Action {
	case OperabilityRecommendationNoAction:
		return nil
	case OperabilityRecommendationPause:
		_, err := s.PausePromotion(ctx, tx, PausePromotionInput{
			InstanceID: recommendation.InstanceID,
			UserID:     recommendation.UserID,
		})
		return err
	case OperabilityRecommendationResume:
		_, err := s.ResumePromotion(ctx, tx, ResumePromotionInput{
			InstanceID: recommendation.InstanceID,
			UserID:     recommendation.UserID,
		})
		return err
	case OperabilityRecommendationStop:
		return s.stopAndFinalizeInstance(ctx, tx, recommendation.InstanceID, recommendation.UserID, entity.StopReason(recommendation.Reason))
	default:
		return fmt.Errorf("unknown operability recommendation action: %s", recommendation.Action)
	}
}

// ========================================================================
// REASSIGN PROMOTION
// ========================================================================

// ReassignPromotionInput contains the parameters for reassigning a promotion.
type ReassignPromotionInput struct {
	InstanceID    uuid.UUID
	UserID        uuid.UUID
	NewTargetType entity.TargetType
	NewTargetID   *uuid.UUID
}

// ReassignPromotionResult contains the result of reassigning a promotion.
type ReassignPromotionResult struct {
	NewInstance *entity.PromotionInstance
	Ownership   *entity.PromotionOwnership
}

// ReassignPromotion moves a promotion from one target to another.
// The old instance is stopped, and a new one is created with the same ownership.
// Remaining duration is preserved at ownership level.
func (s *PromotionService) ReassignPromotion(
	ctx context.Context,
	tx db.Tx,
	input ReassignPromotionInput,
) (*ReassignPromotionResult, error) {
	// CRITICAL: Get database time for all accounting operations
	dbTime, err := s.repo.GetDBTime(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to get database time: %w", err)
	}

	// Get old instance with row lock to serialize concurrent finalizers.
	oldInstance, err := s.repo.GetInstanceForUpdate(ctx, tx, input.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}
	if oldInstance == nil {
		return nil, &InstanceNotFoundError{InstanceID: input.InstanceID}
	}

	// Verify user owns this instance
	if oldInstance.UserID != input.UserID {
		return nil, &NotInstanceOwnerError{InstanceID: input.InstanceID, UserID: input.UserID}
	}

	// Get ownership
	ownership, err := s.repo.GetOwnershipForUpdate(ctx, tx, oldInstance.OwnershipID)
	if err != nil {
		return nil, fmt.Errorf("failed to get ownership: %w", err)
	}
	if ownership == nil {
		return nil, &OwnershipNotFoundError{OwnershipID: oldInstance.OwnershipID}
	}

	// Verify user owns this ownership
	if ownership.UserID != input.UserID {
		return nil, &entity.NotOwnershipOwnerError{OwnershipID: ownership.ID, UserID: input.UserID}
	}

	// Check if ownership can still be used
	if !ownership.CanActivate(dbTime) {
		if ownership.IsExpired(dbTime) {
			return nil, &entity.OwnershipExpiredError{OwnershipID: ownership.ID, ExpiresAt: ownership.ExpiresAt}
		}
		if ownership.IsFullyConsumed() {
			return nil, &entity.OwnershipConsumedError{OwnershipID: ownership.ID}
		}
		return nil, &OwnershipNotAvailableError{OwnershipID: ownership.ID, Status: ownership.Status}
	}

	// Get package to validate target type
	pkg, err := s.repo.GetPackageByID(ctx, tx, ownership.PackageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get package: %w", err)
	}
	if pkg == nil {
		return nil, &PackageNotFoundError{PackageID: ownership.PackageID}
	}

	// Validate new target type is allowed
	if !pkg.AllowsTargetType(input.NewTargetType) {
		return nil, &TargetTypeNotAllowedError{
			TargetType:   input.NewTargetType,
			AllowedTypes: pkg.AllowedTargetTypes,
		}
	}

	if input.NewTargetType.RequiresTargetID() && input.NewTargetID == nil {
		return nil, &entity.ValidationError{Field: "target_id", Message: "required for this target type"}
	}

	// Validate target ownership and operability.
	if input.NewTargetType.RequiresTargetID() && input.NewTargetID != nil {
		err = s.operabilityChecker.ValidateOwnership(ctx, input.UserID, input.NewTargetType, input.NewTargetID)
		if err != nil {
			return nil, fmt.Errorf("ownership validation failed: %w", err)
		}

		// Check operability
		isOperable, reason, err := s.operabilityChecker.CheckOperability(ctx, input.NewTargetType, input.NewTargetID)
		if err != nil {
			return nil, fmt.Errorf("operability check failed: %w", err)
		}
		if !isOperable {
			return nil, &TargetNotOperableError{
				TargetType: input.NewTargetType,
				TargetID:   *input.NewTargetID,
				Reason:     reason,
			}
		}
	}

	// ========================================================================
	// CRITICAL: STOP OLD INSTANCE FIRST (before creating new)
	// ========================================================================
	// This prevents a window where 2 instances are active simultaneously
	// If crash occurs after new instance creation but before old instance stops,
	// both would be active in DB, violating invariants

	// CRITICAL: Prevent modification of already finalized instances
	if oldInstance.Finalized {
		return nil, &InstanceAlreadyFinalizedError{InstanceID: oldInstance.ID}
	}

	// STEP 1: Stop the old instance FIRST with database time
	err = oldInstance.Stop(entity.StopReasonUserCancelled, dbTime)
	if err != nil {
		return nil, fmt.Errorf("failed to stop old instance: %w", err)
	}

	// STEP 2: Snapshot consumed duration and mark as finalized with database time
	consumedSeconds := oldInstance.SnapshotConsumedDuration(dbTime)

	// STEP 3: Add consumed duration to ownership (bake it in)
	err = s.repo.AddConsumedDurationToOwnership(ctx, tx, oldInstance.OwnershipID, consumedSeconds)
	if err != nil {
		return nil, fmt.Errorf("failed to add consumed duration to ownership: %w", err)
	}

	// STEP 4: Persist old instance changes FIRST
	// This ensures old instance is stopped in DB before new instance is created
	err = s.repo.UpdateInstance(ctx, tx, oldInstance)
	if err != nil {
		return nil, fmt.Errorf("failed to update old instance: %w", err)
	}

	// ========================================================================
	// NOW CREATE NEW INSTANCE (after old is stopped and persisted)
	// ========================================================================

	// STEP 5: Create new instance with database time
	newInstance, err := entity.NewPromotionInstance(
		ownership.ID,
		input.UserID,
		input.NewTargetType,
		input.NewTargetID,
		dbTime,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create instance: %w", err)
	}

	// STEP 6: Activate the new instance with database time
	err = newInstance.Activate(dbTime)
	if err != nil {
		return nil, fmt.Errorf("failed to activate instance: %w", err)
	}

	// STEP 7: Persist new instance LAST
	err = s.repo.CreateInstance(ctx, tx, newInstance)
	if err != nil {
		return nil, fmt.Errorf("failed to persist new instance: %w", err)
	}

	return &ReassignPromotionResult{
		NewInstance: newInstance,
		Ownership:   ownership,
	}, nil
}

// ========================================================================
// QUERIES
// ========================================================================

// ListPackages retrieves all available promotion packages.
func (s *PromotionService) ListPackages(
	ctx context.Context,
	tx db.Tx,
	includeInactive bool,
) ([]*entity.PromotionPackage, error) {
	return s.repo.ListPackages(ctx, tx, includeInactive)
}

// ListMyOwnerships retrieves ownerships for a user.
func (s *PromotionService) ListMyOwnerships(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	status entity.OwnershipStatus,
	limit, offset int,
) ([]*entity.PromotionOwnership, error) {
	if limit > 0 {
		return s.repo.ListOwnershipsByUserPaginated(ctx, tx, userID, status, limit, offset)
	}
	return s.repo.ListOwnershipsByUser(ctx, tx, userID, status)
}

// ListMyInstances retrieves instances for a user.
func (s *PromotionService) ListMyInstances(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	status entity.InstanceStatus,
) ([]*entity.PromotionInstance, error) {
	return s.repo.ListInstancesByUser(ctx, tx, userID, status)
}

// GetOwnership retrieves a single ownership by ID.
func (s *PromotionService) GetOwnership(
	ctx context.Context,
	tx db.Tx,
	ownershipID uuid.UUID,
) (*entity.PromotionOwnership, error) {
	return s.repo.GetOwnershipByID(ctx, tx, ownershipID)
}

// GetInstance retrieves a single instance by ID.
func (s *PromotionService) GetInstance(
	ctx context.Context,
	tx db.Tx,
	instanceID uuid.UUID,
) (*entity.PromotionInstance, error) {
	return s.repo.GetInstanceByID(ctx, tx, instanceID)
}

// ========================================================================
// AUTO-STOP (Event Hook)
// ========================================================================

// OnTargetStatusChanged is called by target domains when status changes.
// This is the PRIMARY mechanism for auto-stopping promotions on dead targets.
//
// Example:
//   - When fixed-price sale is sold -> all active instances for that sale are stopped
//   - When auction ends -> all active instances for that auction are stopped
//
// This method snapshots consumed duration for each stopped instance to ensure
// accounting safety (no double counting, no lost duration).
func (s *PromotionService) OnTargetStatusChanged(
	ctx context.Context,
	tx db.Tx,
	targetType entity.TargetType,
	targetID uuid.UUID,
	newStatus string,
) error {
	// CRITICAL: Get database time for all accounting operations
	dbTime, err := s.repo.GetDBTime(ctx, tx)
	if err != nil {
		return fmt.Errorf("failed to get database time: %w", err)
	}

	reason := s.mapStatusToStopReason(targetType, newStatus)

	// Classify: reversible reasons → pause, permanent → stop+finalize.
	// The stop reason string matches the operability reason format, so
	// IsReversibleReason can classify it directly.
	reversible := IsReversibleReason(string(reason))

	// Find all active instances for this target
	instances, err := s.repo.GetActiveInstancesByTarget(ctx, tx, targetType, targetID)
	if err != nil {
		return fmt.Errorf("failed to get active instances for target: %w", err)
	}

	for _, instance := range instances {
		if instance.Finalized {
			continue
		}

		if reversible {
			// PAUSE — non-terminal, no finalization
			if instance.Status == entity.InstanceStatusPaused {
				continue // already paused
			}
			if err := instance.Pause(dbTime); err != nil {
				_ = err
				continue
			}
			if err := s.repo.UpdateInstance(ctx, tx, instance); err != nil {
				_ = err
				continue
			}
		} else {
			// STOP — terminal, canonical 4-step finalization
			if instance.Status.IsTerminal() {
				continue
			}
			lockedInstance, err := s.repo.GetInstanceForUpdate(ctx, tx, instance.ID)
			if err != nil {
				_ = err
				continue
			}
			if lockedInstance == nil {
				continue
			}
			instance = lockedInstance
			if err := instance.Stop(reason, dbTime); err != nil {
				_ = err
				continue
			}
			consumedSeconds := instance.SnapshotConsumedDuration(dbTime)
			if err := s.repo.AddConsumedDurationToOwnership(ctx, tx, instance.OwnershipID, consumedSeconds); err != nil {
				_ = err
				continue
			}
			if err := s.repo.UpdateInstance(ctx, tx, instance); err != nil {
				_ = err
				continue
			}
		}
	}

	return nil
}

// ResumeByTarget resumes all paused promotion instances for a target
// that is now operable again. Checks ownership validity before resume.
func (s *PromotionService) ResumeByTarget(
	ctx context.Context,
	tx db.Tx,
	targetType entity.TargetType,
	targetID uuid.UUID,
) (int, error) {
	dbTime, err := s.repo.GetDBTime(ctx, tx)
	if err != nil {
		return 0, fmt.Errorf("failed to get database time: %w", err)
	}

	instances, err := s.repo.GetPausedInstancesByTarget(ctx, tx, targetType, targetID)
	if err != nil {
		return 0, fmt.Errorf("failed to get paused instances for target: %w", err)
	}

	resumedCount := 0
	for _, instance := range instances {
		if instance.Finalized || instance.Status != entity.InstanceStatusPaused {
			continue
		}

		// Check ownership is still usable
		ownership, err := s.repo.GetOwnershipByID(ctx, tx, instance.OwnershipID)
		if err != nil || ownership == nil || !ownership.CanActivate(dbTime) {
			continue // ownership expired or consumed — leave paused for safety worker to finalize
		}

		if err := instance.Resume(dbTime); err != nil {
			_ = err
			continue
		}
		if err := s.repo.UpdateInstance(ctx, tx, instance); err != nil {
			_ = err
			continue
		}
		resumedCount++
	}

	return resumedCount, nil
}

// PauseByUser pauses all active promotion instances for a user.
// Used when seller-level governance conditions change (subscription expired, etc.).
func (s *PromotionService) PauseByUser(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) (int, error) {
	dbTime, err := s.repo.GetDBTime(ctx, tx)
	if err != nil {
		return 0, fmt.Errorf("failed to get database time: %w", err)
	}

	instances, err := s.repo.ListInstancesByUser(ctx, tx, userID, entity.InstanceStatusActive)
	if err != nil {
		return 0, fmt.Errorf("failed to list active instances for user: %w", err)
	}

	pausedCount := 0
	for _, instance := range instances {
		if instance.Finalized || instance.Status != entity.InstanceStatusActive {
			continue
		}
		lockedInstance, err := s.repo.GetInstanceForUpdate(ctx, tx, instance.ID)
		if err != nil {
			_ = err
			continue
		}
		if lockedInstance == nil {
			continue
		}
		instance = lockedInstance
		if err := instance.Pause(dbTime); err != nil {
			_ = err
			continue
		}
		if err := s.repo.UpdateInstance(ctx, tx, instance); err != nil {
			_ = err
			continue
		}
		pausedCount++
	}

	return pausedCount, nil
}

// StopByUser terminally stops all active and paused promotion instances for a user.
// Used when permanent seller-level conditions change.
func (s *PromotionService) StopByUser(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	reason entity.StopReason,
) (int, error) {
	dbTime, err := s.repo.GetDBTime(ctx, tx)
	if err != nil {
		return 0, fmt.Errorf("failed to get database time: %w", err)
	}

	// Stop active instances
	activeInstances, err := s.repo.ListInstancesByUser(ctx, tx, userID, entity.InstanceStatusActive)
	if err != nil {
		return 0, fmt.Errorf("failed to list active instances for user: %w", err)
	}

	// Stop paused instances
	pausedInstances, err := s.repo.ListInstancesByUser(ctx, tx, userID, entity.InstanceStatusPaused)
	if err != nil {
		return 0, fmt.Errorf("failed to list paused instances for user: %w", err)
	}

	allInstances := append(activeInstances, pausedInstances...)
	stoppedCount := 0

	for _, instance := range allInstances {
		if instance.Finalized || instance.Status.IsTerminal() {
			continue
		}
		lockedInstance, err := s.repo.GetInstanceForUpdate(ctx, tx, instance.ID)
		if err != nil {
			_ = err
			continue
		}
		if lockedInstance == nil {
			continue
		}
		instance = lockedInstance
		if err := instance.Stop(reason, dbTime); err != nil {
			_ = err
			continue
		}
		consumedSeconds := instance.SnapshotConsumedDuration(dbTime)
		if err := s.repo.AddConsumedDurationToOwnership(ctx, tx, instance.OwnershipID, consumedSeconds); err != nil {
			_ = err
			continue
		}
		if err := s.repo.UpdateInstance(ctx, tx, instance); err != nil {
			_ = err
			continue
		}
		stoppedCount++
	}

	return stoppedCount, nil
}

// ResumeByUser resumes all paused promotion instances for a user whose
// seller-level governance condition has been resolved.
// Re-checks target operability before each resume.
func (s *PromotionService) ResumeByUser(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) (int, error) {
	dbTime, err := s.repo.GetDBTime(ctx, tx)
	if err != nil {
		return 0, fmt.Errorf("failed to get database time: %w", err)
	}

	instances, err := s.repo.ListInstancesByUser(ctx, tx, userID, entity.InstanceStatusPaused)
	if err != nil {
		return 0, fmt.Errorf("failed to list paused instances for user: %w", err)
	}

	resumedCount := 0
	for _, instance := range instances {
		if instance.Finalized || instance.Status != entity.InstanceStatusPaused {
			continue
		}

		// Check ownership is still usable
		ownership, err := s.repo.GetOwnershipByID(ctx, tx, instance.OwnershipID)
		if err != nil || ownership == nil || !ownership.CanActivate(dbTime) {
			continue
		}

		// Re-check target operability before resume
		if instance.TargetType.RequiresTargetID() && instance.TargetID != nil {
			isOperable, _, err := s.operabilityChecker.CheckOperability(ctx, instance.TargetType, instance.TargetID)
			if err != nil || !isOperable {
				continue // target still inoperable — keep paused
			}
		}

		if err := instance.Resume(dbTime); err != nil {
			_ = err
			continue
		}
		if err := s.repo.UpdateInstance(ctx, tx, instance); err != nil {
			_ = err
			continue
		}
		resumedCount++
	}

	return resumedCount, nil
}

// StopByTarget synchronously stops all active promotions for a target.
// This is called DIRECTLY by target domains (e.g., when fixed-price sale is sold).
// This is the PRIMARY synchronous stop mechanism - outbox events are secondary.
//
// This should be called within a transaction to ensure atomicity.
// This method snapshots consumed duration for each stopped instance to ensure
// accounting safety (no double counting, no lost duration).
func (s *PromotionService) StopByTarget(
	ctx context.Context,
	tx db.Tx,
	targetType entity.TargetType,
	targetID uuid.UUID,
	reason entity.StopReason,
) (int, error) {
	// CRITICAL: Get database time for all accounting operations
	dbTime, err := s.repo.GetDBTime(ctx, tx)
	if err != nil {
		return 0, fmt.Errorf("failed to get database time: %w", err)
	}

	// Find all active instances for this target
	instances, err := s.repo.GetActiveInstancesByTarget(ctx, tx, targetType, targetID)
	if err != nil {
		return 0, fmt.Errorf("failed to get active instances for target: %w", err)
	}

	stoppedCount := 0

	// Stop all instances and snapshot consumption
	for _, instance := range instances {
		// CRITICAL: Prevent modification of already finalized instances
		if instance.Finalized {
			continue
		}
		lockedInstance, err := s.repo.GetInstanceForUpdate(ctx, tx, instance.ID)
		if err != nil {
			_ = err
			continue
		}
		if lockedInstance == nil {
			continue
		}
		instance = lockedInstance

		// Stop the instance with database time
		err = instance.Stop(reason, dbTime)
		if err != nil {
			// Log but continue
			_ = err
			continue
		}

		// Snapshot consumed duration and mark as finalized with database time
		consumedSeconds := instance.SnapshotConsumedDuration(dbTime)

		// Add consumed duration to ownership (bake it in)
		err = s.repo.AddConsumedDurationToOwnership(ctx, tx, instance.OwnershipID, consumedSeconds)
		if err != nil {
			// Log but continue
			_ = err
			continue
		}

		// Persist instance changes (including finalized flag)
		err = s.repo.UpdateInstance(ctx, tx, instance)
		if err != nil {
			// Log but continue
			_ = err
			continue
		}

		stoppedCount++
	}

	return stoppedCount, nil
}

// mapStatusToStopReason maps a target status change to a canonical stop reason.
func (s *PromotionService) mapStatusToStopReason(targetType entity.TargetType, status string) entity.StopReason {
	switch targetType {
	case entity.TargetTypeForSale:
		switch status {
		case "sold", "unavailable":
			return entity.StopReasonForSaleSold
		case "hidden", "withdrawn":
			return entity.StopReasonForSaleHidden
		case "deleted":
			return entity.StopReasonForSaleDeleted
		case "moderated":
			return entity.StopReasonForSaleModerated
		case "expired":
			return entity.StopReasonForSaleExpired
		default:
			return entity.StopReasonForSaleSold
		}
	case entity.TargetTypeAuction:
		switch status {
		case "ended":
			return entity.StopReasonAuctionEnded
		case "cancelled":
			return entity.StopReasonAuctionCancelled
		case "deleted":
			return entity.StopReasonAuctionDeleted
		case "moderated":
			return entity.StopReasonAuctionModerated
		default:
			return entity.StopReasonAuctionEnded
		}
	default:
		return entity.StopReasonUserCancelled
	}
}

// ========================================================================
// WORKER METHODS
// ========================================================================

// ProcessExpiredOwnerships marks ownerships as expired if their validity window has passed.
// Active instances are finalized with canonical accounting (Stop → Snapshot → Bake → Persist).
// This is called by a periodic worker.
func (s *PromotionService) ProcessExpiredOwnerships(
	ctx context.Context,
	tx db.Tx,
	limit int,
) (int, error) {
	// CRITICAL: Get database time for all accounting operations
	dbTime, err := s.repo.GetDBTime(ctx, tx)
	if err != nil {
		return 0, fmt.Errorf("failed to get database time: %w", err)
	}

	ownerships, err := s.repo.ListExpiredOwnerships(ctx, tx, limit)
	if err != nil {
		return 0, fmt.Errorf("failed to list expired ownerships: %w", err)
	}

	count := 0
	for _, ownership := range ownerships {
		// Finalize any active/paused instances BEFORE marking ownership expired.
		// This ensures consumed duration is baked into the ownership.
		instances, err := s.repo.ListInstancesByOwnership(ctx, tx, ownership.ID)
		if err != nil {
			continue
		}
		for _, instance := range instances {
			if instance.Finalized || instance.Status.IsTerminal() {
				continue
			}
			if !instance.IsActive() && instance.Status != entity.InstanceStatusPaused {
				continue
			}
			lockedInstance, err := s.repo.GetInstanceForUpdate(ctx, tx, instance.ID)
			if err != nil {
				continue
			}
			if lockedInstance == nil {
				continue
			}
			instance = lockedInstance

			// Canonical 4-step finalization sequence:
			// 1. Stop the instance
			if err := instance.Stop(entity.StopReasonValidityExpired, dbTime); err != nil {
				continue
			}
			// 2. Snapshot consumed duration
			consumedSeconds := instance.SnapshotConsumedDuration(dbTime)
			// 3. Bake into ownership
			if consumedSeconds > 0 {
				if err := s.repo.AddConsumedDurationToOwnership(ctx, tx, instance.OwnershipID, consumedSeconds); err != nil {
					continue
				}
			}
			// 4. Persist finalized instance
			if err := s.repo.UpdateInstance(ctx, tx, instance); err != nil {
				continue
			}
		}

		// Now mark the ownership as expired
		ownership.MarkAsExpired(dbTime)
		if err := s.repo.UpdateOwnership(ctx, tx, ownership); err != nil {
			continue
		}

		count++
	}

	return count, nil
}

// ProcessDurationExhaustedInstances detects active instances whose consumed
// duration meets or exceeds their ownership's remaining duration and finalizes
// them. This prevents promotions from running beyond purchased hours.
//
// Called by the expiration worker alongside ownership expiry processing.
func (s *PromotionService) ProcessDurationExhaustedInstances(
	ctx context.Context,
	tx db.Tx,
	limit int,
) (int, error) {
	dbTime, err := s.repo.GetDBTime(ctx, tx)
	if err != nil {
		return 0, fmt.Errorf("failed to get database time: %w", err)
	}

	instances, err := s.repo.GetAllActiveInstances(ctx, tx, limit)
	if err != nil {
		return 0, fmt.Errorf("failed to get active instances: %w", err)
	}

	count := 0
	for _, instance := range instances {
		if instance.Finalized {
			continue
		}
		lockedInstance, err := s.repo.GetInstanceForUpdate(ctx, tx, instance.ID)
		if err != nil {
			continue
		}
		if lockedInstance == nil {
			continue
		}
		instance = lockedInstance

		// Load ownership to check remaining duration
		ownership, err := s.repo.GetOwnershipByID(ctx, tx, instance.OwnershipID)
		if err != nil {
			continue
		}

		// Calculate how much this instance has consumed so far (wall-clock)
		currentConsumed := instance.GetConsumedDurationSecondsAt(dbTime)
		remainingSeconds := (ownership.TotalDurationHours - ownership.ConsumedDurationHours) * 3600

		if currentConsumed < remainingSeconds {
			continue // Still has remaining duration
		}

		// Duration exhausted — canonical 4-step finalization
		if err := instance.Stop(entity.StopReasonDurationExhausted, dbTime); err != nil {
			continue
		}
		consumedSeconds := instance.SnapshotConsumedDuration(dbTime)
		if consumedSeconds > 0 {
			if err := s.repo.AddConsumedDurationToOwnership(ctx, tx, instance.OwnershipID, consumedSeconds); err != nil {
				continue
			}
		}
		if err := s.repo.UpdateInstance(ctx, tx, instance); err != nil {
			continue
		}
		count++
	}

	return count, nil
}

// ========================================================================
// ERROR TYPES
// ========================================================================

// PackageNotFoundError is returned when a package is not found.
type PackageNotFoundError struct {
	PackageID uuid.UUID
}

func (e *PackageNotFoundError) Error() string {
	return fmt.Sprintf("package not found: %s", e.PackageID)
}

// PackageInactiveError is returned when attempting to purchase an inactive package.
type PackageInactiveError struct {
	PackageID uuid.UUID
}

func (e *PackageInactiveError) Error() string {
	return fmt.Sprintf("package is not active: %s", e.PackageID)
}

// OwnershipNotFoundError is returned when an ownership is not found.
type OwnershipNotFoundError struct {
	OwnershipID uuid.UUID
}

func (e *OwnershipNotFoundError) Error() string {
	return fmt.Sprintf("ownership not found: %s", e.OwnershipID)
}

// OwnershipNotAvailableError is returned when ownership is not available for activation.
type OwnershipNotAvailableError struct {
	OwnershipID uuid.UUID
	Status      entity.OwnershipStatus
}

func (e *OwnershipNotAvailableError) Error() string {
	return fmt.Sprintf("ownership not available: %s (status: %s)", e.OwnershipID, e.Status)
}

// InstanceNotFoundError is returned when an instance is not found.
type InstanceNotFoundError struct {
	InstanceID uuid.UUID
}

func (e *InstanceNotFoundError) Error() string {
	return fmt.Sprintf("instance not found: %s", e.InstanceID)
}

// InstanceAlreadyStoppedError is returned when attempting to stop a stopped instance.
type InstanceAlreadyStoppedError struct {
	InstanceID uuid.UUID
	Status     entity.InstanceStatus
}

func (e *InstanceAlreadyStoppedError) Error() string {
	return fmt.Sprintf("instance already stopped: %s (status: %s)", e.InstanceID, e.Status)
}

// InstanceAlreadyFinalizedError is returned when attempting to modify a finalized instance.
type InstanceAlreadyFinalizedError struct {
	InstanceID uuid.UUID
}

func (e *InstanceAlreadyFinalizedError) Error() string {
	return fmt.Sprintf("instance already finalized: %s", e.InstanceID)
}

// NotInstanceOwnerError is returned when a user attempts to access another user's instance.
type NotInstanceOwnerError struct {
	InstanceID uuid.UUID
	UserID     uuid.UUID
}

func (e *NotInstanceOwnerError) Error() string {
	return "not the owner of this instance"
}

// TargetTypeNotAllowedError is returned when attempting to promote a disallowed target type.
type TargetTypeNotAllowedError struct {
	TargetType   entity.TargetType
	AllowedTypes []entity.TargetType
}

func (e *TargetTypeNotAllowedError) Error() string {
	return fmt.Sprintf("target type %s not allowed by package", e.TargetType)
}

// TargetNotOperableError is returned when attempting to promote a non-operable target.
type TargetNotOperableError struct {
	TargetType entity.TargetType
	TargetID   uuid.UUID
	Reason     string
}

func (e *TargetNotOperableError) Error() string {
	return fmt.Sprintf("target not operable: %s %s (reason: %s)", e.TargetType, e.TargetID, e.Reason)
}

// DurationExpiredError is returned when attempting to use an expired ownership.
type DurationExpiredError struct {
	OwnershipID uuid.UUID
	ExpiresAt   time.Time
}

func (e *DurationExpiredError) Error() string {
	return "ownership validity window has expired"
}

// InstanceNotPausedError is returned when attempting to resume a non-paused instance.
type InstanceNotPausedError struct {
	InstanceID uuid.UUID
	Status     entity.InstanceStatus
}

func (e *InstanceNotPausedError) Error() string {
	return fmt.Sprintf("instance is not paused: %s (status: %s)", e.InstanceID, e.Status)
}

// PromotionAlreadyActiveForTargetError is returned when attempting to activate
// a promotion for a target that already has an active promotion.
type PromotionAlreadyActiveForTargetError struct {
	TargetType entity.TargetType
	TargetID   uuid.UUID
}

func (e *PromotionAlreadyActiveForTargetError) Error() string {
	return fmt.Sprintf("target already has an active promotion: %s %s", e.TargetType, e.TargetID)
}

// ========================================================================
// DISCOVERY METHODS - For Search, Home, and other discovery surfaces
// ========================================================================

// GetPromotedItemsForDiscovery returns active promoted items for discovery surfaces.
// This is a convenience wrapper that creates the DiscoveryService on demand.
// The dbConn parameter should be the database pool (from db.Pgx()).
func (s *PromotionService) GetPromotedItemsForDiscovery(
	ctx context.Context,
	dbConn *db.DB,
	limit int,
) ([]*entity.PromotionInstance, error) {
	if dbConn == nil {
		return []*entity.PromotionInstance{}, nil
	}
	discoveryService := NewDiscoveryService(dbConn, s.operabilityChecker)
	return discoveryService.GetPromotedItems(ctx, limit)
}

// GetPromotedItemsByTargetForDiscovery returns promoted items filtered by target type.
func (s *PromotionService) GetPromotedItemsByTargetForDiscovery(
	ctx context.Context,
	dbConn *db.DB,
	targetType entity.TargetType,
	limit int,
) ([]*entity.PromotionInstance, error) {
	if dbConn == nil {
		return []*entity.PromotionInstance{}, nil
	}
	discoveryService := NewDiscoveryService(dbConn, s.operabilityChecker)
	return discoveryService.GetPromotedItemsByTargetType(ctx, targetType, limit)
}

// IsTargetPromoted checks if a specific target has an active promotion.
func (s *PromotionService) IsTargetPromoted(
	ctx context.Context,
	dbConn *db.DB,
	targetType entity.TargetType,
	targetID uuid.UUID,
) (bool, error) {
	if dbConn == nil {
		return false, nil
	}
	discoveryService := NewDiscoveryService(dbConn, s.operabilityChecker)
	return discoveryService.IsTargetPromoted(ctx, targetType, targetID)
}

// IsTargetPromotedInTx checks whether a target is currently promoted using the
// caller's transaction. This is used by read-models that already have a tx and
// need an accurate public_visible flag without opening a second handle.
func (s *PromotionService) IsTargetPromotedInTx(
	ctx context.Context,
	tx db.Tx,
	targetType entity.TargetType,
	targetID uuid.UUID,
) (bool, error) {
	if !targetType.IsPublicPromotable() {
		return false, nil
	}

	instance, err := s.repo.GetActiveInstanceByTargetForUpdate(ctx, tx, targetType, targetID)
	if err != nil {
		return false, fmt.Errorf("failed to check promoted target: %w", err)
	}
	if instance == nil {
		return false, nil
	}

	isOperable, _, err := s.operabilityChecker.CheckOperability(ctx, targetType, &targetID)
	if err != nil {
		return false, err
	}
	return isOperable, nil
}

// ========================================================================
// ADMIN PACKAGE MANAGEMENT
// ========================================================================

// CreatePackageInput holds parameters for creating a promotion package.
type CreatePackageInput struct {
	Name                string
	TotalDurationHours  int
	ValidityWindowHours int
	PriceAmount         int
	AllowedTargetTypes  []entity.TargetType
}

// AdminCreatePackage creates a new promotion package (admin-only).
func (s *PromotionService) AdminCreatePackage(
	ctx context.Context,
	tx db.Tx,
	input CreatePackageInput,
) (*entity.PromotionPackage, error) {
	pkg, err := entity.NewPromotionPackage(
		input.Name,
		input.TotalDurationHours,
		input.ValidityWindowHours,
		input.PriceAmount,
		input.AllowedTargetTypes,
	)
	if err != nil {
		return nil, err
	}

	if err := s.repo.CreatePackage(ctx, tx, pkg); err != nil {
		return nil, fmt.Errorf("failed to create package: %w", err)
	}
	return pkg, nil
}

// UpdatePackageInput holds parameters for updating a promotion package.
type UpdatePackageInput struct {
	Name                string
	TotalDurationHours  int
	ValidityWindowHours int
	PriceAmount         int
	AllowedTargetTypes  []entity.TargetType
	IsActive            bool
}

// AdminUpdatePackage updates an existing promotion package (admin-only).
// Returns PackageNotFoundError if the package does not exist.
func (s *PromotionService) AdminUpdatePackage(
	ctx context.Context,
	tx db.Tx,
	packageID uuid.UUID,
	input UpdatePackageInput,
) (*entity.PromotionPackage, error) {
	pkg, err := s.repo.GetPackageByID(ctx, tx, packageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get package: %w", err)
	}
	if pkg == nil {
		return nil, &PackageNotFoundError{PackageID: packageID}
	}

	// Validate via entity constructor (reuses all validation logic)
	if _, err := entity.NewPromotionPackage(
		input.Name,
		input.TotalDurationHours,
		input.ValidityWindowHours,
		input.PriceAmount,
		input.AllowedTargetTypes,
	); err != nil {
		return nil, err
	}

	pkg.Name = input.Name
	pkg.TotalDurationHours = input.TotalDurationHours
	pkg.ValidityWindowHours = input.ValidityWindowHours
	pkg.PriceAmount = input.PriceAmount
	pkg.AllowedTargetTypes = input.AllowedTargetTypes
	pkg.IsActive = input.IsActive

	if err := s.repo.UpdatePackage(ctx, tx, pkg); err != nil {
		return nil, fmt.Errorf("failed to update package: %w", err)
	}
	return pkg, nil
}

// AdminSetPackageActive enables or disables a promotion package.
// Returns PackageNotFoundError if the package does not exist.
func (s *PromotionService) AdminSetPackageActive(
	ctx context.Context,
	tx db.Tx,
	packageID uuid.UUID,
	active bool,
) (*entity.PromotionPackage, error) {
	pkg, err := s.repo.GetPackageByID(ctx, tx, packageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get package: %w", err)
	}
	if pkg == nil {
		return nil, &PackageNotFoundError{PackageID: packageID}
	}

	pkg.IsActive = active
	if err := s.repo.UpdatePackage(ctx, tx, pkg); err != nil {
		return nil, fmt.Errorf("failed to update package: %w", err)
	}
	return pkg, nil
}

// ========================================================================
// ADMIN CAMPAIGN VISIBILITY
// ========================================================================

// AdminListCampaigns retrieves promotion instances for admin visibility.
func (s *PromotionService) AdminListCampaigns(
	ctx context.Context,
	tx db.Tx,
	filter promotionRepo.AdminCampaignFilter,
) ([]*promotionRepo.AdminCampaignRow, int, error) {
	return s.repo.ListCampaignsAdmin(ctx, tx, filter)
}

// ========================================================================
// ADMIN FORCE-STOP
// ========================================================================

// ForceStopInstanceAdmin terminates a promotion instance on behalf of an admin.
// Unlike the user-facing DeactivatePromotion, this skips the ownership check and
// always uses StopReasonAdminCancelled. No automatic refund is issued.
// Returns InstanceAlreadyStoppedError if the instance is already in a terminal state.
func (s *PromotionService) ForceStopInstanceAdmin(
	ctx context.Context,
	tx db.Tx,
	instanceID uuid.UUID,
) error {
	dbTime, err := s.repo.GetDBTime(ctx, tx)
	if err != nil {
		return fmt.Errorf("failed to get database time: %w", err)
	}

	instance, err := s.repo.GetInstanceForUpdate(ctx, tx, instanceID)
	if err != nil {
		return fmt.Errorf("failed to get instance: %w", err)
	}
	if instance == nil {
		return &InstanceNotFoundError{InstanceID: instanceID}
	}

	if instance.Status.IsTerminal() {
		return &InstanceAlreadyStoppedError{InstanceID: instanceID, Status: instance.Status}
	}

	if instance.Finalized {
		return &InstanceAlreadyFinalizedError{InstanceID: instanceID}
	}

	if err := instance.Stop(entity.StopReasonAdminCancelled, dbTime); err != nil {
		return fmt.Errorf("failed to stop instance: %w", err)
	}

	consumedSeconds := instance.SnapshotConsumedDuration(dbTime)

	if err := s.repo.AddConsumedDurationToOwnership(ctx, tx, instance.OwnershipID, consumedSeconds); err != nil {
		return fmt.Errorf("failed to add consumed duration to ownership: %w", err)
	}

	if err := s.repo.UpdateInstance(ctx, tx, instance); err != nil {
		return fmt.Errorf("failed to update instance: %w", err)
	}

	return nil
}
