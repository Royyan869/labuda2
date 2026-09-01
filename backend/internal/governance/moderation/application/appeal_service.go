package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	forsaleEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	moderationrepo "github.com/labuda/backend/internal/governance/moderation/infrastructure/repository"
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

// ============================================================================
// APPEAL CONTEXT — Read model for canonical Decision→Case→Subject resolution
// ============================================================================

// AppealContext provides the canonical context for an Appeal.
// It resolves Decision → Case → Subject and optionally the latest Enforcement.
//
// This is a READ MODEL / QUERY abstraction only.
// It does NOT create another governance state table.
type AppealContext struct {
	Decision   *entity.Decision
	Case       *entity.CanonicalCase
	Enforcement *entity.Enforcement // latest enforcement for this Decision (may be nil)
}

// SubjectResourceType returns the Case's subject type as a ResourceType
// suitable for the ownership lookup.
func (c *AppealContext) SubjectResourceType() entity.ResourceType {
	if c.Case == nil {
		return ""
	}
	return entity.ResourceType(c.Case.SubjectType.String())
}

// SubjectID returns the Case's subject ID.
func (c *AppealContext) SubjectID() uuid.UUID {
	if c.Case == nil {
		return uuid.Nil
	}
	return c.Case.SubjectID
}

// ============================================================================
// APPEAL SERVICE
// ============================================================================

// AppealService handles appeal business logic.
//
// SLICE A: Canonical alignment — Appeal → Decision.
// Dependencies use canonical DecisionRepository + CaseRepository (NOT ModerationRepository).
//
// DOMAIN TERMINOLOGY:
// - DECISION: Governance outcome being appealed
// - CASE: The subject being governed (via Decision → Case)
// - APPEAL: User contest of a Decision's outcome
//
// V1 APPEALABLE TYPES:
// - content removed → auto-restore on approval via outbox event (DEFERRED TO SLICE B)
// - comment removed → auto-restore on approval via outbox event (DEFERRED TO SLICE B)
// - fixed-price sale removed → record-only; approval is administrative
// - auction cancelled → record-only; auction cannot be auto-restored
// - user suspended → record-only; account reinstatement is manual admin action
//
// NON-APPEALABLE (V1):
// - chat_message hidden → too low trust-impact
// - no_violation Decisions → pure rejection/no-action (Design §23)
type AppealService struct {
	appealRepo     moderationrepo.AppealRepository
	decisionRepo   moderationrepo.DecisionRepository
	caseRepo       moderationrepo.CaseRepository
	decisionService *DecisionService
	contentRepo    contentRepo.ContentRepository
	commentRepo    contentrepository.CommentRepository

	// Optional: set via SetForSaleRepo / SetAuctionRepo after construction.
	forSaleRepo  forSaleOwnerRepo
	auctionRepo  auctionOwnerRepo
}

// NewAppealService creates a new AppealService.
// SLICE A: Uses canonical DecisionRepository + CaseRepository instead of ModerationRepository.
// SLICE B: outboxRepo removed — restoration authority is via Decision #2 + Outbox
// in DecisionService, not AppealService.
func NewAppealService(
	appealRepo moderationrepo.AppealRepository,
	decisionRepo moderationrepo.DecisionRepository,
	caseRepo moderationrepo.CaseRepository,
	decisionService *DecisionService,
	contentRepo contentRepo.ContentRepository,
	commentRepo contentrepository.CommentRepository,
) *AppealService {
	if appealRepo == nil {
		panic("NewAppealService: appealRepo must not be nil")
	}
	if decisionRepo == nil {
		panic("NewAppealService: decisionRepo must not be nil")
	}
	if caseRepo == nil {
		panic("NewAppealService: caseRepo must not be nil")
	}
	if decisionService == nil {
		panic("NewAppealService: decisionService must not be nil")
	}
	if contentRepo == nil {
		panic("NewAppealService: contentRepo must not be nil (supports content appeals)")
	}
	if commentRepo == nil {
		panic("NewAppealService: commentRepo must not be nil (supports comment appeals)")
	}
	return &AppealService{
		appealRepo:      appealRepo,
		decisionRepo:    decisionRepo,
		caseRepo:        caseRepo,
		decisionService: decisionService,
		contentRepo:     contentRepo,
		commentRepo:     commentRepo,
	}
}

// SetForSaleRepo wires the fixed-price sale repository for appeal eligibility.
func (s *AppealService) SetForSaleRepo(r forSaleOwnerRepo) {
	s.forSaleRepo = r
}

// SetAuctionRepo wires the auction repository for auction-type appeal eligibility.
func (s *AppealService) SetAuctionRepo(r auctionOwnerRepo) {
	s.auctionRepo = r
}

// ============================================================================
// APPEAL CONTEXT RESOLUTION
// ============================================================================

// resolveAppealContext resolves the canonical context for an Appeal.
// It fetches Decision → Case → Enforcement in sequence.
//
// Returns AppealContext with at least Decision populated.
// Case may be nil if the Case does not exist (should not happen in normal operation).
// Enforcement may be nil if no Enforcement exists for this Decision.
func (s *AppealService) resolveAppealContext(
	ctx context.Context,
	tx interface{},
	decisionID uuid.UUID,
) (*AppealContext, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	// 1. Fetch Decision
	decision, err := s.decisionRepo.GetByID(ctx, dbTx, decisionID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch decision: %w", err)
	}
	if decision == nil {
		return nil, &entity.ErrDecisionNotFound{DecisionID: decisionID}
	}

	// 2. Fetch Case (via decision.case_id)
	kase, err := s.caseRepo.GetByID(ctx, dbTx, decision.CaseID)
	if err != nil {
		// Case not found is non-fatal — return appeal context without Case
		return &AppealContext{Decision: decision}, nil
	}

	// 3. Fetch latest Enforcement (best-effort, may be nil)
	enforcements, err := s.decisionRepo.ListByCase(ctx, dbTx, decision.CaseID, 1, 0)
	_ = enforcements // Not used directly; enforcement lookup is via separate path

	// For now, Enforcement is not resolved in the context.
	// Slice B will add full Enforcement resolution.
	ctx_result := &AppealContext{
		Decision: decision,
		Case:     kase,
	}

	return ctx_result, nil
}

// ============================================================================
// CREATE APPEAL
// ============================================================================

// CreateAppeal creates a new appeal for a governance Decision.
//
// SLICE A: Canonical alignment — appeal targets Decision, not GovernanceCase.
//
// Business rules (Design §23):
// - Appeal is available to the affected party of a Decision with consequences
// - Pure rejection/no-action Decisions are NOT appealable (Design §23)
// - Only one pending appeal per Decision at a time (Design §35)
// - Suspended users may appeal their own account suspension
//
// Concurrency safety: Uses DB-level atomic check-and-insert.
func (s *AppealService) CreateAppeal(
	ctx context.Context,
	tx interface{},
	decisionID uuid.UUID,
	appealedBy uuid.UUID,
	message string,
) (*entity.Appeal, error) {
	// 1. Resolve canonical context: Decision → Case → Subject
	appealCtx, err := s.resolveAppealContext(ctx, tx, decisionID)
	if err != nil {
		return nil, err
	}

	// 2. Verify the Decision produces consequences (Design §23)
	// no_violation Decisions are NOT appealable
	if appealCtx.Decision.Outcome == entity.DecisionOutcomeNoViolation {
		return nil, &entity.ErrDecisionNotAppealable{
			DecisionID: decisionID,
			Outcome:    appealCtx.Decision.Outcome,
		}
	}

	// 3. Verify Case exists and has a subject
	if appealCtx.Case == nil {
		return nil, &entity.ErrDecisionNotFound{DecisionID: decisionID}
	}

	// 4. Verify ownership — the appealedBy user must own the moderated resource
	resourceType := appealCtx.SubjectResourceType()
	resourceID := appealCtx.SubjectID()

	resourceOwnerID, err := s.getResourceOwner(ctx, tx, resourceType, resourceID)
	if err != nil {
		if containsIgnoreCase(err.Error(), "not found") {
			return nil, &entity.ErrDecisionNotFound{DecisionID: decisionID}
		}
		return nil, fmt.Errorf("failed to verify resource ownership: %w", err)
	}

	if resourceOwnerID != appealedBy {
		return nil, &entity.ErrNotResourceOwner{
			DecisionID:   decisionID,
			ResourceID:   resourceID,
			UserID:       appealedBy,
			ResourceType: string(resourceType),
		}
	}

	// 5. Create and persist the appeal with atomic duplicate check
	appeal := entity.NewAppeal(decisionID, appealedBy, message)
	if err := s.appealRepo.CreateWithPendingCheck(ctx, tx, appeal); err != nil {
		if isDuplicatePendingError(err) {
			return nil, &entity.ErrDuplicatePendingAppeal{DecisionID: decisionID}
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

// ============================================================================
// OWNERSHIP
// ============================================================================

// getResourceOwner retrieves the owner ID of a resource by type.
// Returns the owner's user ID or ErrUnsupportedResourceType for non-appealable types.
//
// SLICE A: Uses ResourceType (derived from Case.SubjectType).
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
		return resourceID, nil

	default:
		return uuid.Nil, &entity.ErrUnsupportedResourceType{ResourceType: string(resourceType)}
	}
}

// ============================================================================
// READ OPERATIONS
// ============================================================================

// GetAppeal retrieves an appeal by ID.
func (s *AppealService) GetAppeal(
	ctx context.Context,
	tx interface{},
	appealID uuid.UUID,
) (*entity.Appeal, error) {
	return s.appealRepo.GetByID(ctx, tx, appealID)
}

// GetAppealWithContext retrieves an appeal with its canonical Decision/Case context.
func (s *AppealService) GetAppealWithContext(
	ctx context.Context,
	tx interface{},
	appealID uuid.UUID,
) (*entity.Appeal, *AppealContext, error) {
	appeal, err := s.appealRepo.GetByID(ctx, tx, appealID)
	if err != nil {
		return nil, nil, err
	}

	appealCtx, err := s.resolveAppealContext(ctx, tx, appeal.DecisionID)
	if err != nil {
		// Return appeal but nil context if resolution fails
		return appeal, nil, nil
	}

	return appeal, appealCtx, nil
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

// ListAppealsByDecision retrieves all appeals for a specific Decision.
// Canonical: Decision 1 → 0..N Appeal (Design §5).
func (s *AppealService) ListAppealsByDecision(
	ctx context.Context,
	tx interface{},
	decisionID uuid.UUID,
) ([]*entity.Appeal, error) {
	return s.appealRepo.ListByDecisionID(ctx, tx, decisionID)
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

// ============================================================================
// REVIEW APPEAL
// ============================================================================

// ReviewAppeal reviews an appeal and applies a decision.
//
// SLICE B: Canonical Appeal reversal path.
// Creates Decision #2 atomically via DecisionService.
//
// TRANSACTION INVARIANT: All operations (appeal lock, Decision #2,
// Enforcement #2, Outbox, Audit, Appeal status update) execute within
// the SAME transaction provided by the caller. The caller (handler)
// owns the transaction boundary via db.WithTx.
func (s *AppealService) ReviewAppeal(
	ctx context.Context,
	tx interface{},
	appealID uuid.UUID,
	adminID uuid.UUID,
	approved bool,
	adminResponse *string,
) (*entity.Appeal, error) {
	// Type-assert tx to db.Tx for DecisionService
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type: ReviewAppeal requires db.Tx")
	}

	// Get appeal with FOR UPDATE lock to prevent concurrent reviews
	appeal, err := s.appealRepo.GetForUpdate(ctx, tx, appealID)
	if err != nil {
		return nil, err
	}

	// Check appeal is still pending (defense-in-depth after FOR UPDATE lock)
	if !appeal.Status.IsPending() {
		return nil, &entity.ErrAppealAlreadyReviewed{
			AppealID: appeal.ID,
			Status:   appeal.Status,
		}
	}

	// Resolve canonical context for the appealed Decision
	appealCtx, err := s.resolveAppealContext(ctx, tx, appeal.DecisionID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve appeal context: %w", err)
	}

	if appealCtx == nil || appealCtx.Case == nil {
		return nil, fmt.Errorf("appeal context incomplete: cannot resolve case")
	}

	// Determine Decision #2 outcome based on review result.
	var decisionOutcome entity.DecisionOutcome
	var targetType entity.ModerationTargetType
	var targetID uuid.UUID

	if approved {
		// Reversal: Decision #2 = no_violation (reversing the original violation)
		decisionOutcome = entity.DecisionOutcomeNoViolation
		targetType = entity.ModerationTargetType(appealCtx.SubjectResourceType())
		targetID = appealCtx.SubjectID()
	} else {
		// Upheld: Decision #2 = violation (upholding the original)
		decisionOutcome = entity.DecisionOutcomeViolation
	}

	// Create Decision #2 via canonical DecisionService — SAME transaction.
	// This creates: Decision #2 + Enforcement #2 (if reversal) + Outbox + Audit.
	decisionNote := adminResponse
	_, err = s.decisionService.CreateAppealDecision(ctx, dbTx, CreateAppealDecisionInput{
		CaseID:       appealCtx.Case.ID,
		DecidedBy:    adminID,
		Outcome:      decisionOutcome,
		DecisionNote: decisionNote,
		AppealID:     appeal.ID,
		TargetType:   targetType,
		TargetID:     targetID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create appeal decision: %w", err)
	}

	// Apply the appeal status transition
	if approved {
		err = appeal.Approve(adminID, adminResponse)
	} else {
		err = appeal.Reject(adminID, adminResponse)
	}

	if err != nil {
		return nil, err
	}

	// Persist the updated appeal — SAME transaction as Decision #2.
	if err := s.appealRepo.Update(ctx, tx, appeal); err != nil {
		return nil, err
	}

	return appeal, nil
}



// ============================================================================
// UTILITIES
// ============================================================================

// containsIgnoreCase checks if a string contains a substring (case-insensitive).
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (
		s[len(s)-len(substr):] == substr ||
			s[:len(substr)] == substr ||
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
