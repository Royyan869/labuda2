package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	forSaleApp "github.com/labuda/backend/internal/commerce/forsale/application"
	forSaleEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	commerceResponse "github.com/labuda/backend/internal/commerce/response"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/platform/events"
	idempotencyRepo "github.com/labuda/backend/internal/platform/idempotency/repository"
	"github.com/labuda/backend/internal/social/content/entity"
	contentrepo "github.com/labuda/backend/internal/social/content/infrastructure/repository"
	"github.com/labuda/backend/internal/social/content/repository"
	"github.com/labuda/backend/pkg/db"
)

// BlockChecker defines the interface for checking block relationships.
type BlockChecker interface {
	ExistsBlock(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (bool, error)
}

// AuctionValidator validates and loads auction state for commerce references.
type AuctionValidator interface {
	GetAuction(ctx context.Context, tx db.Tx, auctionID uuid.UUID) (*auctionEntity.Auction, error)
}

// ContentVisibilityChecker resolves content visibility for commerce references.
type ContentVisibilityChecker interface {
	GetContentVisibleToViewer(ctx context.Context, tx db.Tx, viewerID uuid.UUID, contentID uuid.UUID) (*entity.Content, error)
}

// CommentService handles comment business logic.
//
// BOUNDARY RULES:
// - Does NOT create Offer (OfferService is the only entry point)
// - Does NOT modify offer state
// - Does NOT create forSale (forSale references are read-only)
// - ForSale validation only (fetches forSale data for reference)
type CommentService struct {
	contentRepo             contentrepo.ContentRepository
	commentRepo             repository.CommentRepository
	forSaleService          *forSaleApp.ForSaleService
	auctionValidator        AuctionValidator
	visibilityChecker       ContentVisibilityChecker
	outboxRepo              OutboxInserter
	idempotencyRepo         *idempotencyRepo.Repository
	blockChecker            BlockChecker // For filtering comments from blocked users
	invariantLogger         InvariantLogger // Logs invariant violations for monitoring
	commerceRefValidator    commerceResponse.Validator // Validates commerce resource references for display
}

// OutboxInserter defines the interface for inserting outbox events.
type OutboxInserter interface {
	InsertTx(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error
}

// NewCommentService creates a new CommentService.
// blockChecker can be nil - if nil, no block filtering will be performed.
// invariantLogger is optional - if nil, no invariant violation logging will occur.
func NewCommentService(
	contentRepo contentrepo.ContentRepository,
	commentRepo repository.CommentRepository,
	forSaleService *forSaleApp.ForSaleService,
	extra ...any,
) *CommentService {
	svc := &CommentService{
		contentRepo:    contentRepo,
		commentRepo:    commentRepo,
		forSaleService: forSaleService,
	}

	switch len(extra) {
	case 0:
		// Allow nil wiring in tests that only exercise constructor retention.
	case 3:
		svc.outboxRepo, _ = extra[0].(OutboxInserter)
		svc.blockChecker, _ = extra[1].(BlockChecker)
		svc.invariantLogger, _ = extra[2].(InvariantLogger)
	case 4:
		svc.outboxRepo, _ = extra[0].(OutboxInserter)
		svc.blockChecker, _ = extra[1].(BlockChecker)
		svc.invariantLogger, _ = extra[2].(InvariantLogger)
		svc.idempotencyRepo, _ = extra[3].(*idempotencyRepo.Repository)
	case 7:
		svc.auctionValidator, _ = extra[0].(AuctionValidator)
		svc.visibilityChecker, _ = extra[1].(ContentVisibilityChecker)
		svc.outboxRepo, _ = extra[2].(OutboxInserter)
		svc.idempotencyRepo, _ = extra[3].(*idempotencyRepo.Repository)
		svc.blockChecker, _ = extra[4].(BlockChecker)
		svc.invariantLogger, _ = extra[5].(InvariantLogger)
	default:
		// Best-effort compatibility: interpret the legacy six-argument wiring
		// or the newer ten-argument wiring when call sites drift.
		if len(extra) > 0 {
			if v, ok := extra[0].(AuctionValidator); ok {
				svc.auctionValidator = v
			}
		}
		if len(extra) > 1 {
			if v, ok := extra[1].(ContentVisibilityChecker); ok {
				svc.visibilityChecker = v
			}
		}
		if len(extra) > 2 {
			svc.outboxRepo, _ = extra[2].(OutboxInserter)
		}
		if len(extra) > 3 {
			svc.idempotencyRepo, _ = extra[3].(*idempotencyRepo.Repository)
		}
		if len(extra) > 4 {
			svc.blockChecker, _ = extra[4].(BlockChecker)
		}
		if len(extra) > 5 {
			svc.invariantLogger, _ = extra[5].(InvariantLogger)
		}
	}

	return svc
}

// SetCommerceReferenceValidator injects the canonical Commerce Response resource
// reference validator. Must be called before any AddCommerceReferenceComment requests.
func (s *CommentService) SetCommerceReferenceValidator(v commerceResponse.Validator) {
	s.commerceRefValidator = v
}

// CommerceReferenceInput carries the canonical commerce-reference payload.
type CommerceReferenceInput struct {
	TargetID     uuid.UUID
	ResourceType entity.ResourceType
	ResourceID   uuid.UUID
	Body         *string
}

// AddComment adds a normal comment to content.
// AUTHORIZATION: Any active user can comment.
// ENFORCES:
// - Cannot comment on deleted content
// - Reply max depth = 1 (top-level comments can be replied, replies cannot be replied)
// - Idempotency-Key: when idempotencyRepo is wired and a key is supplied,
//   the same key + same operation replays the existing comment (no new row);
//   the same key + different operation returns ErrIdempotencyConflict.
func (s *CommentService) AddComment(
	ctx context.Context,
	tx db.Tx,
	callerID uuid.UUID,
	contentID uuid.UUID,
	body string,
	parentID *uuid.UUID,
	idempotencyKey string,
) (*entity.Comment, error) {
	// Validate caller
	if err := auth.ValidateCaller(callerID); err != nil {
		return nil, err
	}

	// Validate body
	if body == "" {
		return nil, &entity.ErrInvalidComment{Reason: "body cannot be empty"}
	}

	// Get content to validate status
	content, err := s.contentRepo.GetByID(ctx, tx, contentID)
	if err != nil {
		return nil, fmt.Errorf("get content failed: %w", err)
	}

	// Validate: Cannot comment on deleted content
	if content.Status == entity.StatusDeleted {
		return nil, &entity.ErrInvalidComment{Reason: "cannot comment on deleted content"}
	}

	// BLOCK FILTERING: Check if caller is blocked before allowing comment
	// Determine the target user to check against (content owner or parent comment author)
	var targetUserID uuid.UUID
	if parentID != nil {
		// For replies, get parent comment author
		parentComment, err := s.commentRepo.GetByID(ctx, tx, *parentID)
		if err != nil {
			return nil, &entity.ErrInvalidComment{Reason: "parent comment not found"}
		}
		targetUserID = parentComment.AuthorID
	} else {
		// For top-level comments, use content owner
		targetUserID = content.AuthorID
	}

	// Check for block relationship (if blockChecker is available)
	if s.blockChecker != nil && callerID != targetUserID {
		blocked, err := s.blockChecker.ExistsBlock(ctx, tx, callerID, targetUserID)
		if err != nil {
			return nil, fmt.Errorf("failed to check block status: %w", err)
		}
		if blocked {
			return nil, &entity.ErrInvalidComment{Reason: "cannot comment: block exists between users"}
		}
	}

	// Validate reply rules if parent_id is provided
	if parentID != nil {
		// Get parent comment (we may have already fetched it above, but need to validate all rules)
		parentComment, err := s.commentRepo.GetByID(ctx, tx, *parentID)
		if err != nil {
			return nil, &entity.ErrInvalidComment{Reason: "parent comment not found"}
		}

		// Validate: Parent must belong to the same content
		if parentComment.TargetID != contentID {
			return nil, &entity.ErrInvalidComment{Reason: "parent comment belongs to different content"}
		}

		// Validate: Parent must not be deleted
		if parentComment.DeletedAt != nil {
			return nil, &entity.ErrInvalidComment{Reason: "cannot reply to deleted comment"}
		}

		// VALIDATE MAX DEPTH = 1: Parent must be top-level (no parent_id)
		if parentComment.ParentID != nil {
			return nil, &entity.ErrInvalidComment{Reason: "replies can only be one level deep"}
		}
	}

	// IDEMPOTENCY ENFORCEMENT (comment wire contract C-IPC): the
	// Idempotency-Key header is mandatory on POST /contents/:id/comments.
	// Same key + same operation must replay the existing comment instead of
	// inserting another row; same key + different operation (actor, target,
	// body, or parent) must return ErrIdempotencyConflict. The enforcement is
	// transactional: the idempotency record and the comment row are written in
	// the same tx, so a conflict can never be half-applied.
	commentID := uuid.New()
	if s.idempotencyRepo != nil && idempotencyKey != "" {
		op := s.normalCommentOperationFingerprint(callerID, contentID, body, parentID)
		rec, created, idemErr := s.idempotencyRepo.GetOrCreate(ctx, tx, idempotencyKey, op, commentID)
		if idemErr != nil {
			if isIdempotencyOperationConflict(idemErr) {
				return nil, fmt.Errorf("%w: %v", entity.ErrIdempotencyConflict, idemErr)
			}
			return nil, fmt.Errorf("idempotency lookup failed: %w", idemErr)
		}
		commentID = rec.EntityID
		if !created {
			return s.commentRepo.GetByID(ctx, tx, rec.EntityID)
		}
	}

	// Create comment
	comment, err := entity.NewComment(contentID, callerID, body)
	if err != nil {
		return nil, err
	}
	comment.ID = commentID

	// Set parent_id if provided
	if parentID != nil {
		comment.ParentID = parentID
	}

	// Persist using repository
	if err := s.commentRepo.Create(ctx, tx, comment); err != nil {
		return nil, fmt.Errorf("create comment failed: %w", err)
	}

	// Emit notification events
	if parentID != nil {
		// This is a reply - notify parent comment author
		// Get parent comment author (we already fetched it above)
		parentComment, err := s.commentRepo.GetByID(ctx, tx, *parentID)
		if err == nil {
			// Don't notify if replying to own comment
			if parentComment.AuthorID != callerID {
				payload := map[string]any{
					"comment_id":       comment.ID.String(),
					"content_id":       contentID.String(),
					"author_id":        callerID.String(),
					"parent_author_id": parentComment.AuthorID.String(),
					"parent_id":        parentID.String(),
					"created_at":       comment.CreatedAt.UTC().Format(time.RFC3339),
				}
				idempotencyKey := fmt.Sprintf("comment.reply.%s", comment.ID.String())
				if err := s.outboxRepo.InsertTx(ctx, tx, events.EventCommentReply, payload, idempotencyKey); err != nil {
					return nil, fmt.Errorf("insert outbox event failed: %w", err)
				}
			}

			// Also notify content owner if different from parent comment author
			// This ensures the content owner is aware of replies on their content
			if content.AuthorID != callerID && content.AuthorID != parentComment.AuthorID {
				payload := map[string]any{
					"comment_id": comment.ID.String(),
					"content_id": contentID.String(),
					"author_id":  callerID.String(),
					"parent_id":  parentID.String(),
					"created_at": comment.CreatedAt.UTC().Format(time.RFC3339),
				}
				idempotencyKey := fmt.Sprintf("comment.created.%s.%s", comment.ID.String(), content.AuthorID.String())
				if err := s.outboxRepo.InsertTx(ctx, tx, events.EventCommentCreated, payload, idempotencyKey); err != nil {
					return nil, fmt.Errorf("insert outbox event failed: %w", err)
				}
			}
		}
	} else {
		// This is a top-level comment - notify content owner
		// Don't notify if commenting on own content
		if content.AuthorID != callerID {
			payload := map[string]any{
				"comment_id": comment.ID.String(),
				"content_id": contentID.String(),
				"author_id":  callerID.String(),
				"created_at": comment.CreatedAt.UTC().Format(time.RFC3339),
			}
			idempotencyKey := fmt.Sprintf("comment.created.%s", comment.ID.String())
			if err := s.outboxRepo.InsertTx(ctx, tx, events.EventCommentCreated, payload, idempotencyKey); err != nil {
				return nil, fmt.Errorf("insert outbox event failed: %w", err)
			}
		}
	}

	return comment, nil
}

// ListComments retrieves comments for a content with cursor-based pagination.
// No authorization required - comments are public.
func (s *CommentService) ListComments(
	ctx context.Context,
	tx db.Tx,
	contentID uuid.UUID,
	limit int,
	cursor string,
) ([]*entity.Comment, string, error) {
	if limit <= 0 {
		limit = 20 // Default limit
	}
	if limit > 100 {
		limit = 100 // Max limit
	}

	comments, nextCursor, err := s.commentRepo.ListByTarget(ctx, tx, entity.TargetContent, contentID, limit, cursor)
	if err != nil {
		return nil, "", fmt.Errorf("list comments failed: %w", err)
	}

	// Comments are already entities from repository
	return comments, nextCursor, nil
}

// AddCommerceReferenceComment adds a commerce-reference comment to a content.
// AUTHORIZATION: Any active user can add commerce refs.
// ENFORCES:
// - Cannot comment on deleted content
// - Commerce resource must exist and be displayable (active for sale, scheduled/active auction)
// - Any user may reference any displayable commerce resource (no ownership or seller-capability gate)
func (s *CommentService) AddCommerceReferenceComment(
	ctx context.Context,
	tx db.Tx,
	callerID uuid.UUID,
	input CommerceReferenceInput,
	idempotencyKey string,
) (*entity.Comment, error) {
	if err := auth.ValidateCaller(callerID); err != nil {
		return nil, err
	}

	if input.TargetID == uuid.Nil || input.ResourceID == uuid.Nil {
		return nil, &entity.ErrInvalidComment{Reason: "target_id and resource_id are required"}
	}

	content, err := s.loadVisibleContentForComment(ctx, tx, callerID, input.TargetID)
	if err != nil {
		return nil, err
	}
	if content.Status == entity.StatusDeleted {
		return nil, &entity.ErrInvalidComment{Reason: "cannot comment on deleted content"}
	}

	commentID := uuid.New()
	if s.idempotencyRepo != nil && idempotencyKey != "" {
		op := s.commerceReferenceOperationFingerprint(callerID, input)
		rec, created, idemErr := s.idempotencyRepo.GetOrCreate(ctx, tx, idempotencyKey, op, commentID)
		if idemErr != nil {
			if isIdempotencyOperationConflict(idemErr) {
				return nil, fmt.Errorf("%w: %v", entity.ErrIdempotencyConflict, idemErr)
			}
			return nil, fmt.Errorf("idempotency lookup failed: %w", idemErr)
		}
		commentID = rec.EntityID
		if !created {
			return s.commentRepo.GetByID(ctx, tx, rec.EntityID)
		}
	}

	// Canonical Commerce Response validation: existence + displayability only.
	// Any user may reference any displayable commerce resource.
	// Ownership and capability authority remain in the commerce domain.
	if err := s.validateCommerceReference(ctx, tx, input); err != nil {
		return nil, err
	}

	// Load the full entity for building the ShareReference response data.
	// The canonical authority already validated; this is for response hydration.
	var shareReference *entity.ShareReference
	switch input.ResourceType {
	case entity.ResourceTypeForSale:
		if s.forSaleService == nil {
			return nil, fmt.Errorf("fixed-price sale service is required")
		}
		forSale, err := s.forSaleService.GetByID(ctx, tx, input.ResourceID)
		if err != nil {
			return nil, &entity.ErrInvalidComment{Reason: "fixed-price sale not found"}
		}

		// Product is the sole canonical authority for fixed-price sale
		// content (title, description, media). ForSale mirrors are
		// in-memory read-throughs; read directly from the canonical Product.
		var imageURL string
		if forSale.Product != nil && len(forSale.Product.MediaURLs) > 0 {
			imageURL = forSale.Product.MediaURLs[0]
		}
		saleTitle := forSale.Title
		if forSale.Product != nil {
			saleTitle = forSale.Product.Title
		}
		shareReference = entity.NewShareReferenceFromForSale(
			forSale.ID.String(),
			saleTitle,
			imageURL,
			true,
			forSale.Status == forSaleEntity.ForSaleStatusSold,
			forSale.Status == forSaleEntity.ForSaleStatusWithdrawn,
		)
	case entity.ResourceTypeAuction:
		if s.auctionValidator == nil {
			return nil, fmt.Errorf("auction validator is required")
		}
		auction, err := s.auctionValidator.GetAuction(ctx, tx, input.ResourceID)
		if err != nil {
			return nil, &entity.ErrInvalidComment{Reason: "auction not found"}
		}
		// Product is the sole canonical authority for auction content (title,
		// description, media). Auction never carries its own content copy.
		auctionTitle := ""
		auctionImageURL := ""
		if auction.Product != nil {
			auctionTitle = auction.Product.Title
			if len(auction.Product.MediaURLs) > 0 {
				auctionImageURL = auction.Product.MediaURLs[0]
			}
		}
		shareReference = entity.NewShareReferenceFromAuction(
			auction.ID.String(),
			auctionTitle,
			auctionImageURL,
			true,
			!auction.Status.IsRepostable(),
			false,
		)
	default:
		return nil, &entity.ErrInvalidComment{Reason: "unsupported commerce resource type"}
	}

	comment, err := entity.NewCommerceReferenceComment(input.TargetID, callerID, shareReference, input.Body)
	if err != nil {
		return nil, err
	}
	comment.ID = commentID

	if err := s.commentRepo.Create(ctx, tx, comment); err != nil {
		return nil, fmt.Errorf("create commerce reference comment failed: %w", err)
	}

	if content.AuthorID != callerID && s.outboxRepo != nil {
		eventType := events.EventSellerResponse
		if input.ResourceType == entity.ResourceTypeAuction {
			eventType = events.EventAuctionResponse
		}
		payload := map[string]any{
			"comment_id":         comment.ID.String(),
			"content_id":         input.TargetID.String(),
			"seller_id":          callerID.String(),
			"request_creator_id": content.AuthorID.String(),
			"resource_id":        input.ResourceID.String(),
			"resource_type":      string(input.ResourceType),
			"created_at":         comment.CreatedAt.UTC().Format(time.RFC3339),
		}
		outboxKey := fmt.Sprintf("%s.%s", eventType, comment.ID.String())
		if err := s.outboxRepo.InsertTx(ctx, tx, eventType, payload, outboxKey); err != nil {
			return nil, fmt.Errorf("insert outbox event failed: %w", err)
		}
	}

	return comment, nil
}

func (s *CommentService) loadVisibleContentForComment(ctx context.Context, tx db.Tx, viewerID, contentID uuid.UUID) (*entity.Content, error) {
	if s.visibilityChecker != nil {
		content, err := s.visibilityChecker.GetContentVisibleToViewer(ctx, tx, viewerID, contentID)
		if err != nil {
			return nil, fmt.Errorf("get content failed: %w", err)
		}
		return content, nil
	}
	// Fail-closed: visibility checker must be wired. Permissive fallback would expose private/followers_only content.
	return nil, fmt.Errorf("content visibility checker not configured")
}

// validateCommerceReference validates that the referenced commerce resource
// exists and is in a valid state for display/reference. This is displayability
// validation, NOT ownership authorization — any user may reference any displayable
// commerce resource. Ownership and capability authority remain in the commerce domain.
func (s *CommentService) validateCommerceReference(
	ctx context.Context,
	tx db.Tx,
	input CommerceReferenceInput,
) error {
	if s.commerceRefValidator == nil {
		return fmt.Errorf("commerce resource validator not configured")
	}

	var resourceType commerceResponse.ResourceType
	switch input.ResourceType {
	case entity.ResourceTypeForSale:
		resourceType = commerceResponse.ResourceTypeForSale
	case entity.ResourceTypeAuction:
		resourceType = commerceResponse.ResourceTypeAuction
	default:
		return &entity.ErrInvalidComment{Reason: "unsupported commerce resource type"}
	}

	err := s.commerceRefValidator.ValidateReference(ctx, tx, resourceType, input.ResourceID)
	if err != nil {
		switch err {
		case commerceResponse.ErrResourceNotFound:
			return &entity.ErrInvalidComment{Reason: "commerce resource not found"}
		case commerceResponse.ErrResourceNotDisplayable:
			return &entity.ErrInvalidComment{Reason: "commerce resource is not valid for response"}
		default:
			return err
		}
	}
	return nil
}

func (s *CommentService) commerceReferenceOperationFingerprint(actorID uuid.UUID, input CommerceReferenceInput) string {
	bodyToken := "<nil>"
	if input.Body != nil {
		bodyToken = *input.Body
	}
	return fmt.Sprintf(
		"comment.commerce:%s:%s:%s:%s:%s",
		actorID.String(),
		input.TargetID.String(),
		string(input.ResourceType),
		input.ResourceID.String(),
		bodyToken,
	)
}

// normalCommentOperationFingerprint derives the idempotency operation key for
// a normal comment create. It uniquely identifies the logical operation so a
// replayed key with a different actor / target / body / parent is rejected as
// a conflict rather than silently replayed.
func (s *CommentService) normalCommentOperationFingerprint(actorID, targetID uuid.UUID, body string, parentID *uuid.UUID) string {
	parentToken := "<nil>"
	if parentID != nil {
		parentToken = parentID.String()
	}
	return fmt.Sprintf(
		"comment.normal:%s:%s:%s:%s",
		actorID.String(),
		targetID.String(),
		body,
		parentToken,
	)
}

func isIdempotencyOperationConflict(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "different operation")
}

// DeleteComment soft deletes a comment.
// AUTHORIZATION: Only comment author can delete.
func (s *CommentService) DeleteComment(
	ctx context.Context,
	tx db.Tx,
	commentID, callerID uuid.UUID,
) error {
	// Get comment to validate ownership
	model, err := s.commentRepo.GetByID(ctx, tx, commentID)
	if err != nil {
		return fmt.Errorf("get comment failed: %w", err)
	}

	// Validate: Only author can delete
	if model.AuthorID != callerID {
		return fmt.Errorf("only comment author can delete comment")
	}

	// Soft delete by setting deleted_at
	now := time.Now()
	if err := s.commentRepo.SoftDelete(ctx, tx, commentID, now); err != nil {
		return fmt.Errorf("delete comment failed: %w", err)
	}

	return nil
}

// SoftDeleteForModeration soft deletes a comment due to moderation action.
//
// STRICT BOUNDARY RULES:
// - NO ownership check (moderation overrides ownership)
// - NO auth check (called from outbox worker)
// - Idempotent: safe to call on already-deleted comments
//
// This method is called by the moderation event handler when a comment
// is removed via moderation.case with decision=removed.
func (s *CommentService) SoftDeleteForModeration(
	ctx context.Context,
	tx db.Tx,
	commentID uuid.UUID,
) error {
	// Get comment to check if already deleted
	model, err := s.commentRepo.GetByID(ctx, tx, commentID)
	if err != nil {
		// If comment not found, treat as success (idempotent)
		// This can happen if comment was already deleted
		return nil
	}

	// Already deleted? Idempotent - return success
	if model.DeletedAt != nil {
		return nil
	}

	// Soft delete by setting deleted_at
	now := time.Now()
	if err := s.commentRepo.SoftDelete(ctx, tx, commentID, now); err != nil {
		return fmt.Errorf("moderation soft delete comment failed: %w", err)
	}

	return nil
}

// RestoreFromModeration restores a comment that was soft-deleted due to moderation.
//
// STRICT BOUNDARY RULES:
// - NO ownership check (moderation overrides ownership)
// - NO auth check (called from outbox worker)
// - Idempotent: safe to call on already-restored comments
//
// This method is called by the moderation event handler when an appeal
// is approved, restoring a comment that was previously removed.
func (s *CommentService) RestoreFromModeration(
	ctx context.Context,
	tx db.Tx,
	commentID uuid.UUID,
) error {
	// Get comment to check if already restored
	model, err := s.commentRepo.GetByID(ctx, tx, commentID)
	if err != nil {
		// If comment not found, treat as success (idempotent)
		return nil
	}

	// Already restored (deleted_at is NULL)? Idempotent - return success
	if model.DeletedAt == nil {
		return nil
	}

	// Restore by setting deleted_at to NULL
	if err := s.commentRepo.Restore(ctx, tx, commentID); err != nil {
		return fmt.Errorf("moderation restore comment failed: %w", err)
	}

	return nil
}
