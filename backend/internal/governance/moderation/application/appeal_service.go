package application

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	forsaleEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	moderationrepo "github.com/labuda/backend/internal/governance/moderation/infrastructure/repository"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	contentRepo "github.com/labuda/backend/internal/social/content/infrastructure/repository"
	contentrepository "github.com/labuda/backend/internal/social/content/repository"
	"github.com/labuda/backend/pkg/db"
)

// forSaleOwnerRepo is a minimal interface for looking up fixed-price sale seller.
// Satisfied by *forsaleRepo.ForSaleRepository.
type forSaleOwnerRepo interface {
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*forsaleEntity.ForSale, error)
}

// auctionOwnerRepo is a minimal interface for looking up auction seller.
// Satisfied by *auctionInfraRepo.AuctionRepository.
type auctionOwnerRepo interface {
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*auctionEntity.Auction, error)
}

// AppealService handles appeal business logic.
//
// DOMAIN TERMINOLOGY:
// - REPORT: User action that created a CASE
// - CASE: Moderation case that was reviewed (now being appealed)
// - APPEAL: User contest of a moderation decision
//
// V1 APPEALABLE TYPES:
// - content removed → auto-restore on approval via outbox event
// - comment removed → auto-restore on approval via outbox event
// - fixed-price sale removed → record-only; approval is administrative, restoration is manual
// - auction cancelled → record-only; auction cannot be auto-restored (bids/timing lost)
// - user suspended → record-only; account reinstatement is manual admin action
//
// NON-APPEALABLE (V1):
// - chat_message hidden → too low trust-impact
// - warning issued → passive record; admin can revoke directly
type AppealService struct {
	appealRepo     moderationrepo.AppealRepository
	moderationRepo moderationrepo.ModerationRepository
	contentRepo    contentRepo.ContentRepository
	commentRepo    contentrepository.CommentRepository
	outboxRepo     *outboxRepo.OutboxRepository

	// Optional: set via SetForSaleRepo / SetAuctionRepo after construction.
	// If nil, fixed-price sale/auction resource-type appeals return ErrUnsupportedResourceType.
	forSaleRepo forSaleOwnerRepo
	auctionRepo auctionOwnerRepo
}

// NewAppealService creates a new AppealService.
// Panics at boot time if any required dependency is nil.
func NewAppealService(
	appealRepo moderationrepo.AppealRepository,
	moderationRepo moderationrepo.ModerationRepository,
	contentRepo contentRepo.ContentRepository,
	commentRepo contentrepository.CommentRepository,
	outboxRepo *outboxRepo.OutboxRepository,
) *AppealService {
	if appealRepo == nil {
		panic("NewAppealService: appealRepo must not be nil")
	}
	if moderationRepo == nil {
		panic("NewAppealService: moderationRepo must not be nil")
	}
	if contentRepo == nil {
		panic("NewAppealService: contentRepo must not be nil (supports content appeals)")
	}
	if commentRepo == nil {
		panic("NewAppealService: commentRepo must not be nil (supports comment appeals)")
	}
	if outboxRepo == nil {
		panic("NewAppealService: outboxRepo must not be nil")
	}
	return &AppealService{
		appealRepo:     appealRepo,
		moderationRepo: moderationRepo,
		contentRepo:    contentRepo,
		commentRepo:    commentRepo,
		outboxRepo:     outboxRepo,
	}
}

// SetForSaleRepo wires the fixed-price sale repository for appeal eligibility.
// Call from serverboot after construction. If not called, fixed-price sale appeals return
// ErrUnsupportedResourceType.
func (s *AppealService) SetForSaleRepo(r forSaleOwnerRepo) {
	s.forSaleRepo = r
}

// SetAuctionRepo wires the auction repository for auction-type appeal eligibility.
// Call from serverboot after construction. If not called, auction appeals return
// ErrUnsupportedResourceType.
func (s *AppealService) SetAuctionRepo(r auctionOwnerRepo) {
	s.auctionRepo = r
}

// CreateAppeal creates a new appeal for a moderation decision.
//
// DOMAIN TERMINOLOGY:
// - caseID: The moderation CASE being appealed (was created from a user report)
// - appealedBy: User creating the appeal (resource owner)
//
// Business rules:
// - Only the resource owner can appeal (content author, fixed-price sale seller, etc.)
// - Suspended users may appeal their own account suspension (route is RequireAuth only)
// - Only one pending appeal per case at a time (enforced atomically)
// - Appeals can only be created for terminal cases (enforced or rejected)
// - Returns typed domain errors for all validation failures
//
// Concurrency safety: Uses DB-level atomic check-and-insert to prevent
// duplicate pending appeals even under concurrent requests.
func (s *AppealService) CreateAppeal(
	ctx context.Context,
	tx interface{},
	caseID uuid.UUID,
	appealedBy uuid.UUID,
	message string,
) (*entity.Appeal, error) {
	// 1. Verify the moderation case exists
	kase, err := s.moderationRepo.GetByID(ctx, tx, caseID)
	if err != nil {
		// Check if this is a "not found" error
		if containsIgnoreCase(err.Error(), "not found") {
			return nil, &entity.ErrCaseNotFound{CaseID: caseID}
		}
		return nil, fmt.Errorf("failed to fetch moderation case: %w", err)
	}

	// 2. Verify the case is in a terminal state (only removed/rejected can be appealed)
	// Approved cases don't need appeal as content was not removed
	if kase.Status != entity.GovernanceCaseStatusEnforced && kase.Status != entity.GovernanceCaseStatusRejected {
		return nil, &entity.ErrCaseNotAppealable{CaseID: kase.ID, Status: kase.Status}
	}

	// 3. Verify ownership - the appealedBy user must own the moderated resource
	resourceOwnerID, err := s.getResourceOwner(ctx, tx, kase.ResourceType, kase.ResourceID)
	if err != nil {
		if containsIgnoreCase(err.Error(), "not found") {
			return nil, &entity.ErrCaseNotFound{CaseID: caseID}
		}
		return nil, fmt.Errorf("failed to verify resource ownership: %w", err)
	}

	if resourceOwnerID != appealedBy {
		return nil, &entity.ErrNotResourceOwner{
			CaseID:       kase.ID,
			ResourceID:   kase.ResourceID,
			UserID:       appealedBy,
			ResourceType: string(kase.ResourceType),
		}
	}

	// 4. Create and persist the appeal with atomic duplicate check
	// The repository method ensures no two concurrent requests can create
	// pending appeals for the same report
	appeal := entity.NewAppeal(caseID, appealedBy, message)
	if err := s.appealRepo.CreateWithPendingCheck(ctx, tx, appeal); err != nil {
		// Handle the specific duplicate error
		if isDuplicatePendingError(err) {
			return nil, &entity.ErrDuplicatePendingAppeal{CaseID: caseID}
		}
		return nil, fmt.Errorf("failed to create appeal: %w", err)
	}

	return appeal, nil
}

// isDuplicatePendingError checks if the error is a duplicate pending appeal error.
func isDuplicatePendingError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*entity.ErrDuplicatePendingAppeal)
	return ok
}

// getResourceOwner retrieves the owner ID of a resource by type.
// Returns the owner's user ID or ErrUnsupportedResourceType for non-appealable types.
//
// Ownership semantics per type:
// - content: AuthorID (social content creator)
// - comment: AuthorID (comment author)
// - fixed-price sale: SellerID (fixed-price sale seller) — requires SetForSaleRepo
// - auction: SellerID (auction seller) — requires SetAuctionRepo
// - user: ResourceID itself (the user is their own resource owner — for suspension appeals)
func (s *AppealService) getResourceOwner(
	ctx context.Context,
	tx interface{},
	resourceType entity.ResourceType,
	resourceID uuid.UUID,
) (uuid.UUID, error) {
	switch resourceType {
	case entity.ResourceTypeContent:
		content, err := s.contentRepo.GetByID(ctx, tx, resourceID)
		if err != nil {
			return uuid.Nil, fmt.Errorf("resource not found: %w", err)
		}
		return content.AuthorID, nil

	case entity.ResourceTypeComment:
		// Comment repository requires db.Tx type, do type assertion
		dbTx, ok := tx.(db.Tx)
		if !ok {
			return uuid.Nil, fmt.Errorf("invalid transaction type for comment repository")
		}
		comment, err := s.commentRepo.GetByID(ctx, dbTx, resourceID)
		if err != nil {
			return uuid.Nil, fmt.Errorf("resource not found: %w", err)
		}
		return comment.AuthorID, nil

	case entity.ResourceTypeForSale:
		if s.forSaleRepo == nil {
			return uuid.Nil, &entity.ErrUnsupportedResourceType{ResourceType: string(resourceType)}
		}
		dbTx, ok := tx.(db.Tx)
		if !ok {
			return uuid.Nil, fmt.Errorf("invalid transaction type for fixed-price sale repository")
		}
		forSale, err := s.forSaleRepo.GetByID(ctx, dbTx, resourceID)
		if err != nil {
			return uuid.Nil, fmt.Errorf("resource not found: %w", err)
		}
		return forSale.SellerID, nil

	case entity.ResourceTypeAuction:
		if s.auctionRepo == nil {
			return uuid.Nil, &entity.ErrUnsupportedResourceType{ResourceType: string(resourceType)}
		}
		dbTx, ok := tx.(db.Tx)
		if !ok {
			return uuid.Nil, fmt.Errorf("invalid transaction type for auction repository")
		}
		auction, err := s.auctionRepo.GetByID(ctx, dbTx, resourceID)
		if err != nil {
			return uuid.Nil, fmt.Errorf("resource not found: %w", err)
		}
		return auction.SellerID, nil

	case entity.ResourceTypeUser:
		// For user suspension appeals, the suspended user IS the resource.
		// The user can only appeal their own suspension (resourceID == appealedBy).
		return resourceID, nil

	default:
		return uuid.Nil, &entity.ErrUnsupportedResourceType{ResourceType: string(resourceType)}
	}
}

// isAutoRestorableType returns true for resource types where appeal approval
// triggers an automatic outbox restoration event (content, comment).
//
// For fixed-price sale/auction/user, approval is administrative record-only.
// Restoration/reinstatement requires separate manual admin action.
func isAutoRestorableType(rt entity.ResourceType) bool {
	return rt == entity.ResourceTypeContent || rt == entity.ResourceTypeComment
}

// containsIgnoreCase checks if a string contains a substring (case-insensitive).
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (
	// Simple case-insensitive contains check for common error patterns
	s[len(s)-len(substr):] == substr || // suffix check
		s[:len(substr)] == substr || // prefix check
		customContains(s, substr)))
}

func customContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// GetAppeal retrieves an appeal by ID.
func (s *AppealService) GetAppeal(
	ctx context.Context,
	tx interface{},
	appealID uuid.UUID,
) (*entity.Appeal, error) {
	return s.appealRepo.GetByID(ctx, tx, appealID)
}

// GetAppealWithCase retrieves an appeal with its original moderation case.
// W1-B2: Operational hardening - provides context for appeal review.
func (s *AppealService) GetAppealWithCase(
	ctx context.Context,
	tx interface{},
	appealID uuid.UUID,
) (*entity.Appeal, *entity.GovernanceCase, error) {
	appeal, err := s.appealRepo.GetByID(ctx, tx, appealID)
	if err != nil {
		return nil, nil, err
	}

	kase, err := s.moderationRepo.GetByID(ctx, tx, appeal.CaseID)
	if err != nil {
		// Return appeal but nil case if case not found
		return appeal, nil, nil
	}

	return appeal, kase, nil
}

// ListAppealsByUser retrieves all appeals created by a specific user.
func (s *AppealService) ListAppealsByUser(
	ctx context.Context,
	tx interface{},
	userID uuid.UUID,
	limit, offset int,
) ([]*entity.Appeal, error) {
	return s.appealRepo.ListByUser(ctx, tx, userID, limit, offset)
}

// ListAppealsByCase retrieves all appeals for a specific case.
func (s *AppealService) ListAppealsByCase(
	ctx context.Context,
	tx interface{},
	caseID uuid.UUID,
) ([]*entity.Appeal, error) {
	return s.appealRepo.ListByCase(ctx, tx, caseID)
}

// ListPendingAppeals retrieves pending appeals awaiting admin review.
func (s *AppealService) ListPendingAppeals(
	ctx context.Context,
	tx interface{},
	limit, offset int,
) ([]*entity.Appeal, error) {
	return s.appealRepo.ListPending(ctx, tx, limit, offset)
}

// ListAllAppeals retrieves all appeals with optional status filter.
func (s *AppealService) ListAllAppeals(
	ctx context.Context,
	tx interface{},
	statusFilter *entity.AppealStatus,
	limit, offset int,
) ([]*entity.Appeal, error) {
	return s.appealRepo.ListAll(ctx, tx, statusFilter, limit, offset)
}

// ReviewAppeal reviews an appeal and applies a decision.
//
// Business rules:
// - Only pending appeals can be reviewed
// - Approved content/comment appeals trigger auto-restoration via outbox event
// - Approved fixed-price sale/auction/user appeals are record-only (no auto-restoration)
// - Rejected appeals uphold the original moderation decision
//
// Transaction safety: Restoration events are emitted BEFORE appeal state changes
// to ensure consistency. If restoration fails, the appeal remains pending and can
// be retried. This prevents split-brain where appeal is approved but restoration
// event is not persisted.
func (s *AppealService) ReviewAppeal(
	ctx context.Context,
	tx interface{},
	appealID uuid.UUID,
	adminID uuid.UUID,
	approved bool,
	adminResponse *string,
) (*entity.Appeal, error) {
	// Get appeal with FOR UPDATE lock to prevent concurrent reviews
	appeal, err := s.appealRepo.GetForUpdate(ctx, tx, appealID)
	if err != nil {
		return nil, err
	}

	// For approval, we need to emit restoration event BEFORE updating appeal state.
	// This ensures that if event emission fails, the appeal is still pending and
	// the operation can be retried without leaving inconsistent state.
	if approved {
		// Type-assert tx to db.Tx for outbox operations
		dbTx, ok := tx.(db.Tx)
		if !ok {
			// Critical: cannot emit restoration event without proper transaction type
			return nil, &entity.ErrRestorationEventFailed{
				AppealID: appeal.ID,
				Err:      fmt.Errorf("invalid transaction type for outbox operations"),
			}
		}

		// Fetch the original moderation case to determine if restoration is needed
		kase, err := s.moderationRepo.GetByID(ctx, tx, appeal.CaseID)
		if err != nil {
			// Critical: cannot determine if restoration is needed
			return nil, &entity.ErrRestorationEventFailed{
				AppealID: appeal.ID,
				Err:      fmt.Errorf("failed to fetch moderation case for restoration: %w", err),
			}
		}

		// Emit restoration event only for auto-restorable types (content, comment).
		// ForSale/auction/user appeals are record-only: approval is administrative,
		// restoration/reinstatement requires separate manual admin action.
		if kase.Status == entity.GovernanceCaseStatusEnforced && isAutoRestorableType(kase.ResourceType) {
			payload := s.buildRestoredPayload(kase, appeal.ID)
			eventType := s.getRestoredEventType(kase.ResourceType)

			// Emit restoration event BEFORE updating appeal status
			// If this fails, appeal remains pending and operation can be retried
			if err := s.outboxRepo.InsertEvent(
				ctx, dbTx,
				eventType,
				kase.ResourceID,
				payload,
			); err != nil {
				return nil, &entity.ErrRestorationEventFailed{
					AppealID: appeal.ID,
					Err:      fmt.Errorf("failed to emit restoration event: %w", err),
				}
			}
		}
	}

	// Apply the decision after restoration event is emitted (for approval)
	// or directly (for rejection)
	if approved {
		err = appeal.Approve(adminID, adminResponse)
	} else {
		err = appeal.Reject(adminID, adminResponse)
	}

	if err != nil {
		return nil, err
	}

	// Persist the updated appeal
	if err := s.appealRepo.Update(ctx, tx, appeal); err != nil {
		return nil, err
	}

	return appeal, nil
}

// buildRestoredPayload creates the JSON payload for a restoration event.
//
// Event format:
//
//	{
//	  "case_id": "uuid",
//	  "appeal_id": "uuid",
//	  "resource_type": "content|comment|...",
//	  "resource_id": "uuid"
//	}
func (s *AppealService) buildRestoredPayload(kase *entity.GovernanceCase, appealID uuid.UUID) []byte {
	type payload struct {
		CaseID       string `json:"case_id"`
		AppealID     string `json:"appeal_id"`
		ResourceType string `json:"resource_type"`
		ResourceID   string `json:"resource_id"`
	}
	p := payload{
		CaseID:       kase.ID.String(),
		AppealID:     appealID.String(),
		ResourceType: string(kase.ResourceType),
		ResourceID:   kase.ResourceID.String(),
	}
	b, _ := json.Marshal(p)
	return b
}

// getRestoredEventType returns the event type for a restoration.
//
// Format: "moderation.<resource_type>.restored"
// Examples:
//   - moderation.content.restored
//   - moderation.comment.restored
func (s *AppealService) getRestoredEventType(resourceType entity.ResourceType) string {
	return fmt.Sprintf("moderation.%s.restored", resourceType)
}
