package application

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	commerceResponse "github.com/labuda/backend/internal/commerce/response"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/social/content/entity"
	contentrepo "github.com/labuda/backend/internal/social/content/infrastructure/repository"
	likedomain "github.com/labuda/backend/internal/social/like"
	"github.com/labuda/backend/pkg/db"
)

// ContentService handles content business logic and state transitions.
type ContentService struct {
	contentRepo            contentrepo.ContentRepository
	likeRepo               likedomain.Repository // For read-only queries - canonical like domain
	roleChecker            auth.RoleChecker
	accountStatusChecker   auth.AccountStatusChecker
	ownership              *auth.OwnershipValidator
	invariantLogger        InvariantLogger // Logs invariant violations for monitoring
	internalShareAuthority internalShareAuthority
	commerceRefValidator   commerceResponse.Validator // Validates commerce resource references for display
}

type contentResourceOccurrenceWriter interface {
	CreateResourceOccurrence(ctx context.Context, tx interface{}, occurrence *entity.ContentResourceOccurrence) error
}

type contentResourceOccurrenceReader interface {
	GetResourceOccurrenceByContentID(ctx context.Context, tx interface{}, contentID uuid.UUID) (*entity.ContentResourceOccurrence, error)
}

// NewContentService creates a new ContentService.
// invariantLogger is optional - if nil, no invariant violation logging will occur.
func NewContentService(
	contentRepo contentrepo.ContentRepository,
	likeRepo likedomain.Repository,
	roleChecker auth.RoleChecker,
	accountStatusChecker auth.AccountStatusChecker,
	invariantLogger InvariantLogger,
) *ContentService {
	svc := &ContentService{
		contentRepo:          contentRepo,
		likeRepo:             likeRepo,
		roleChecker:          roleChecker,
		accountStatusChecker: accountStatusChecker,
		ownership:            auth.NewOwnershipValidator(),
		invariantLogger:      invariantLogger,
	}
	svc.internalShareAuthority = svc
	return svc
}

// SetCommerceReferenceValidator injects the canonical Commerce Response resource
// reference validator. Must be called before any CreateContentWithResourceOccurrence
// requests that reference ForSale or Auction resources.
func (s *ContentService) SetCommerceReferenceValidator(v commerceResponse.Validator) {
	s.commerceRefValidator = v
}

// CreateContent creates a new active content.
// AUTHORIZATION: Any active user can create content.
// ENFORCES: Caption must not be empty.
//
// Location is preserved when provided.
func (s *ContentService) CreateContent(
	ctx context.Context,
	tx db.Tx,
	callerID uuid.UUID,
	caption string,
	visibility entity.Visibility,
	city *string,
	province *string,
	originalAuthorID *uuid.UUID,
	tags []string,
	mentionedUserIDs []uuid.UUID,
) (*entity.Content, error) {

	// Validate caller
	if err := auth.ValidateCaller(callerID); err != nil {
		return nil, err
	}

	// ACCOUNT STATUS: Check if caller's account is active
	if err := s.accountStatusChecker.EnsureActive(ctx, callerID); err != nil {
		return nil, err
	}

	// Validate caption
	if caption == "" {
		return nil, fmt.Errorf("content caption cannot be empty")
	}

	// Create new active content
	content := entity.NewContent(callerID, caption)
	content.Visibility = visibility.Normalize()

	// Preserve location when provided.
	content.City = city
	content.Province = province
	_ = originalAuthorID

	// Persist within transaction
	if err := s.contentRepo.Create(ctx, tx, content); err != nil {
		return nil, fmt.Errorf("create content failed: %w", err)
	}

	// Persist hashtags (fail-open: tag write failure does not abort the content creation).
	if len(tags) > 0 {
		if err := s.contentRepo.InsertTags(ctx, tx, content.ID, tags); err != nil {
			// Non-fatal: content is created successfully; tags can be retried.
			// Log at warn level — no zap here, rely on caller observability.
			_ = err
		} else {
			content.Tags = tags
		}
	}

	// Persist mentioned users — validate each user ID exists, then insert.
	// Deterministic: if any mentioned user ID is invalid, creation fails.
	// The transaction rolls back content creation if mention persistence fails.
	if len(mentionedUserIDs) > 0 {
		validIDs := make([]uuid.UUID, 0, len(mentionedUserIDs))
		for _, uid := range mentionedUserIDs {
			if uid == uuid.Nil {
				continue
			}
			if tx != nil {
				var exists bool
				if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, uid).Scan(&exists); err != nil {
					return nil, fmt.Errorf("mention validation failed for user %s: %w", uid, err)
				}
				if !exists {
					return nil, fmt.Errorf("mentioned user %s does not exist", uid)
				}
			}
			validIDs = append(validIDs, uid)
		}
		if len(validIDs) > 0 {
			if err := s.contentRepo.InsertMentionedUsers(ctx, tx, content.ID, validIDs); err != nil {
				return nil, fmt.Errorf("mention persistence failed: %w", err)
			}
		}
	}

	return content, nil
}

// CreateContentWithResourceOccurrence creates content and persists the
// canonical resource occurrence in the same transaction.
func (s *ContentService) CreateContentWithResourceOccurrence(
	ctx context.Context,
	tx db.Tx,
	callerID uuid.UUID,
	caption string,
	visibility entity.Visibility,
	city *string,
	province *string,
	occurrence *entity.ContentResourceOccurrenceIdentity,
	tags []string,
	mentionedUserIDs []uuid.UUID,
) (*entity.Content, error) {
	if err := auth.ValidateCaller(callerID); err != nil {
		return nil, err
	}
	if err := s.accountStatusChecker.EnsureActive(ctx, callerID); err != nil {
		return nil, err
	}
	if caption == "" {
		return nil, fmt.Errorf("content caption cannot be empty")
	}
	if !visibility.IsValid() {
		visibility = entity.VisibilityPublic
	}
	if occurrence != nil && !occurrence.Operation.IsValid() {
		return nil, fmt.Errorf("invalid operation: %s", occurrence.Operation)
	}
	if occurrence != nil && occurrence.Operation == entity.ContentResourceOccurrenceOperationDirectCommerceInsertContent && !occurrence.ResourceType.CanDirectCommerceInsert() {
		return nil, fmt.Errorf("invalid resource type for direct commerce insert: %s", occurrence.ResourceType)
	}
	if occurrence != nil && !occurrence.ResourceType.IsValid() {
		return nil, fmt.Errorf("invalid resource type: %s", occurrence.ResourceType)
	}
	if occurrence != nil && occurrence.ResourceID == uuid.Nil {
		return nil, fmt.Errorf("resource id is required")
	}
	if occurrence != nil {
		// Commerce Response authorization (ForSale / Auction) is delegated to
		// the canonical Commerce Response boundary for direct_commerce_insert_content
		// operations. Non-commerce resource types and share_to_feed operations
		// retain their own validation.
		if occurrence.Operation == entity.ContentResourceOccurrenceOperationDirectCommerceInsertContent {
			switch occurrence.ResourceType {
			case entity.ContentResourceOccurrenceResourceTypeForSale:
				if err := s.validateCommerceReference(ctx, tx, commerceResponse.ResourceTypeForSale, occurrence.ResourceID); err != nil {
					return nil, err
				}
			case entity.ContentResourceOccurrenceResourceTypeAuction:
				if err := s.validateCommerceReference(ctx, tx, commerceResponse.ResourceTypeAuction, occurrence.ResourceID); err != nil {
					return nil, err
				}
			case entity.ContentResourceOccurrenceResourceTypeContent:
				if err := s.validateContentTarget(ctx, tx, occurrence.ResourceID.String()); err != nil {
					return nil, err
				}
			case entity.ContentResourceOccurrenceResourceTypeProfile:
				if err := s.validateProfileTarget(ctx, tx, occurrence.ResourceID.String()); err != nil {
					return nil, err
				}
			default:
				return nil, fmt.Errorf("invalid resource type for direct commerce insert: %s", occurrence.ResourceType)
			}			} else {
				// share_to_feed and other non-commerce operations: validate resource
				// existence and displayability. Commerce resource types (ForSale, Auction)
				// route through the canonical Commerce Response validator. Non-commerce
				// types use their own validation.
				switch occurrence.ResourceType {
				case entity.ContentResourceOccurrenceResourceTypeContent:
					if err := s.validateContentTarget(ctx, tx, occurrence.ResourceID.String()); err != nil {
						return nil, err
					}
				case entity.ContentResourceOccurrenceResourceTypeForSale:
					if err := s.validateCommerceReference(ctx, tx, commerceResponse.ResourceTypeForSale, occurrence.ResourceID); err != nil {
						return nil, err
					}
				case entity.ContentResourceOccurrenceResourceTypeAuction:
					if err := s.validateCommerceReference(ctx, tx, commerceResponse.ResourceTypeAuction, occurrence.ResourceID); err != nil {
						return nil, err
					}
				case entity.ContentResourceOccurrenceResourceTypeProfile:
					if err := s.validateProfileTarget(ctx, tx, occurrence.ResourceID.String()); err != nil {
						return nil, err
					}
				default:
					return nil, fmt.Errorf("invalid resource type: %s", occurrence.ResourceType)
				}
			}
	}

	content := entity.NewContent(callerID, caption)
	content.Visibility = visibility
	content.IsHidden = visibility == entity.VisibilityPrivate
	content.City = city
	content.Province = province

	if err := s.contentRepo.Create(ctx, tx, content); err != nil {
		return nil, fmt.Errorf("create content failed: %w", err)
	}

	if occurrence != nil {
		occ := entity.NewContentResourceOccurrence(content.ID, callerID, occurrence)
		if err := createContentResourceOccurrence(ctx, tx, s.contentRepo, occ); err != nil {
			return nil, err
		}
	}

	if len(tags) > 0 {
		if err := s.contentRepo.InsertTags(ctx, tx, content.ID, tags); err != nil {
			_ = err
		} else {
			content.Tags = tags
		}
	}

	// Persist mentioned users — same deterministic contract as CreateContent.
	if len(mentionedUserIDs) > 0 {
		validIDs := make([]uuid.UUID, 0, len(mentionedUserIDs))
		for _, uid := range mentionedUserIDs {
			if uid == uuid.Nil {
				continue
			}
			if tx != nil {
				var exists bool
				if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, uid).Scan(&exists); err != nil {
					return nil, fmt.Errorf("mention validation failed for user %s: %w", uid, err)
				}
				if !exists {
					return nil, fmt.Errorf("mentioned user %s does not exist", uid)
				}
			}
			validIDs = append(validIDs, uid)
		}
		if len(validIDs) > 0 {
			if err := s.contentRepo.InsertMentionedUsers(ctx, tx, content.ID, validIDs); err != nil {
				return nil, fmt.Errorf("mention persistence failed: %w", err)
			}
		}
	}

	return content, nil
}

// validateContentTarget validates that content exists and is in a shareable state
func (s *ContentService) validateContentTarget(
	ctx context.Context,
	tx db.Tx,
	contentID string,
) error {
	targetID, err := uuid.Parse(contentID)
	if err != nil {
		return fmt.Errorf("invalid target_id for content: %w", err)
	}

	_, err = s.GetContentPublic(ctx, tx, targetID)
	if err != nil {
		return fmt.Errorf("content not found: %w", err)
	}

	return nil
}

// validateCommerceReference validates that the referenced commerce resource
// exists and is in a valid state for display/reference. This is displayability
// validation, NOT ownership authorization — any user may reference any displayable
// commerce resource. Ownership and capability authority remain in the commerce domain.
func (s *ContentService) validateCommerceReference(
	ctx context.Context,
	tx db.Tx,
	resourceType commerceResponse.ResourceType,
	resourceID uuid.UUID,
) error {
	if s.commerceRefValidator == nil {
		// Fallback: canonical validator not wired. Return an explicit error
		// rather than silently passing.
		return fmt.Errorf("commerce resource validator not configured")
	}
	if err := s.commerceRefValidator.ValidateReference(ctx, tx, resourceType, resourceID); err != nil {
		switch err {
		case commerceResponse.ErrResourceNotFound:
			return shareTargetNotFoundError(string(resourceType), resourceID.String(), err)
		case commerceResponse.ErrResourceNotDisplayable:
			return shareTargetStatusError(string(resourceType), "invalid", resourceID.String())
		default:
			return err
		}
	}
	return nil
}

// validateProfileTarget validates that a user profile exists and is in a shareable state
func (s *ContentService) validateProfileTarget(
	ctx context.Context,
	tx db.Tx,
	profileID string,
) error {
	query := `
		SELECT id, account_status, deleted_at
		FROM users
		WHERE id = $1
	`

	var id uuid.UUID
	var accountStatus string
	var deletedAt sql.NullTime

	err := tx.QueryRow(ctx, query, profileID).Scan(&id, &accountStatus, &deletedAt)
	if err != nil {
		return shareTargetNotFoundError("user profile", profileID, err)
	}

	// Reject if deleted
	if deletedAt.Valid {
		return shareTargetDeletedError("user profile", profileID)
	}

	// Reject if banned
	if accountStatus == "banned" {
		return fmt.Errorf("cannot share banned user profile: %s", profileID)
	}

	return nil
}

// DeleteContent soft-deletes content.
// AUTHORIZATION: Only the author can delete their content (admin can override).
// ENFORCES: Deleted content cannot transition back (terminal state).
func (s *ContentService) DeleteContent(
	ctx context.Context,
	tx db.Tx,
	callerID uuid.UUID,
	contentID uuid.UUID,
) error {
	// Validate caller
	if err := auth.ValidateCaller(callerID); err != nil {
		return err
	}

	// Lock content for update
	content, err := s.contentRepo.GetForUpdate(ctx, tx, contentID)
	if err != nil {
		return err
	}

	// AUTHORIZATION: Only author or admin can delete
	if !auth.IsSystemCaller(callerID) && content.AuthorID != callerID {
		return auth.ErrOwnerRequired
	}

	// Transition state
	if err := content.Delete(); err != nil {
		return fmt.Errorf("delete content failed: %w", err)
	}

	// Persist changes
	if err := s.contentRepo.Update(ctx, tx, content); err != nil {
		return fmt.Errorf("update content failed: %w", err)
	}

	return nil
}

// HideContent marks content as hidden without changing status.
// AUTHORIZATION: Only the author can hide their content (admin can override).
func (s *ContentService) HideContent(
	ctx context.Context,
	tx db.Tx,
	callerID uuid.UUID,
	contentID uuid.UUID,
) error {
	// Validate caller
	if err := auth.ValidateCaller(callerID); err != nil {
		return err
	}

	// Lock content for update
	content, err := s.contentRepo.GetForUpdate(ctx, tx, contentID)
	if err != nil {
		return err
	}

	// AUTHORIZATION: Only author or admin can hide
	if !auth.IsSystemCaller(callerID) && content.AuthorID != callerID {
		return auth.ErrOwnerRequired
	}

	// Hide content
	if err := content.Hide(); err != nil {
		return fmt.Errorf("hide content failed: %w", err)
	}

	// Persist changes
	if err := s.contentRepo.Update(ctx, tx, content); err != nil {
		return fmt.Errorf("update content failed: %w", err)
	}

	return nil
}

// UnhideContent marks content as visible without changing status.
// AUTHORIZATION: Only the author can unhide their content (admin can override).
func (s *ContentService) UnhideContent(
	ctx context.Context,
	tx db.Tx,
	callerID uuid.UUID,
	contentID uuid.UUID,
) error {
	// Validate caller
	if err := auth.ValidateCaller(callerID); err != nil {
		return err
	}

	// Lock content for update
	content, err := s.contentRepo.GetForUpdate(ctx, tx, contentID)
	if err != nil {
		return err
	}

	// AUTHORIZATION: Only author or admin can unhide
	if !auth.IsSystemCaller(callerID) && content.AuthorID != callerID {
		return auth.ErrOwnerRequired
	}

	// Unhide content
	if err := content.Unhide(); err != nil {
		return fmt.Errorf("unhide content failed: %w", err)
	}

	// Persist changes
	if err := s.contentRepo.Update(ctx, tx, content); err != nil {
		return fmt.Errorf("update content failed: %w", err)
	}

	return nil
}

// GetContent retrieves content without locking.
func (s *ContentService) GetContent(
	ctx context.Context,
	tx db.Tx,
	contentID uuid.UUID,
) (*entity.Content, error) {
	return s.contentRepo.GetByID(ctx, tx, contentID)
}

// GetContentPublic retrieves content for public-facing reads with visibility hardening.
//
// VISIBILITY INTEGRITY V1:
// - Returns "not found" error for deleted content (status = 'deleted')
// - This ensures public-facing reads don't leak deleted/moderated content
// - Internal/admin flows should use GetContent() directly for full access
//
// Use this method for:
// - Public API endpoints where users read content
// - Feed generation and content discovery
// - Cross-domain references where visibility should be enforced
func (s *ContentService) GetContentPublic(
	ctx context.Context,
	tx db.Tx,
	contentID uuid.UUID,
) (*entity.Content, error) {
	if s == nil || s.contentRepo == nil {
		return nil, fmt.Errorf("content not found")
	}

	content, err := s.contentRepo.GetByID(ctx, tx, contentID)
	if err != nil {
		return nil, err
	}

	if err := s.validatePublicContentVisibility(ctx, tx, content); err != nil {
		return nil, err
	}

	return content, nil
}

// validatePublicContentVisibility enforces the public-read boundary for a
// content row. The caller decides whether to surface the content or hide it as
// "not found".
//
// Rules:
//   - hide deleted or hidden content
//   - hide content whose own author is not active
//   - if the content is a content-type repost, also validate the repost target
//     content and its author lifecycle
func (s *ContentService) validatePublicContentVisibility(
	ctx context.Context,
	tx db.Tx,
	content *entity.Content,
) error {
	if content == nil {
		return fmt.Errorf("content not found")
	}

	if content.Status == entity.StatusDeleted || content.IsHidden {
		return fmt.Errorf("content not found")
	}

	if err := s.ensurePublicAuthorActive(ctx, tx, content.AuthorID); err != nil {
		return err
	}

	if !content.IsRepost() {
		return nil
	}

	occurrence, err := getContentResourceOccurrenceByContentID(ctx, tx, s.contentRepo, content.ID)
	if err != nil {
		return fmt.Errorf("content not found: %w", err)
	}
	if occurrence == nil || occurrence.ResourceType() != entity.ContentResourceOccurrenceResourceTypeContent {
		return nil
	}

	targetID := occurrence.SourceID()
	targetContent, err := s.contentRepo.GetByID(ctx, tx, targetID)
	if err != nil {
		return err
	}

	if targetContent.Status == entity.StatusDeleted || targetContent.IsHidden {
		return fmt.Errorf("content not found")
	}

	return s.validatePublicContentVisibility(ctx, tx, targetContent)
}

// ensurePublicAuthorActive returns "content not found" unless the author row
// exists, is not soft-deleted, and is active.
func (s *ContentService) ensurePublicAuthorActive(ctx context.Context, tx db.Tx, authorID uuid.UUID) error {
	if authorID == uuid.Nil {
		return fmt.Errorf("content not found")
	}

	var status string
	var deleted bool
	err := tx.QueryRow(ctx, `
		SELECT account_status, (deleted_at IS NOT NULL)
		FROM users
		WHERE id = $1
	`, authorID).Scan(&status, &deleted)
	if err != nil {
		return fmt.Errorf("content not found")
	}

	if status != "active" || deleted {
		return fmt.Errorf("content not found")
	}

	return nil
}

// ListByAuthor retrieves content by author ID with cursor-based pagination.
// Returns active, non-deleted content by the author.
func (s *ContentService) ListByAuthor(
	ctx context.Context,
	tx db.Tx,
	authorID uuid.UUID,
	viewerID uuid.UUID,
	limit int,
	cursor string,
) ([]*entity.Content, string, error) {
	return s.contentRepo.ListByAuthor(ctx, tx, authorID, viewerID, limit, cursor)
}

// GetContentMedia retrieves all media for a content.
func (s *ContentService) GetContentMedia(
	ctx context.Context,
	tx db.Tx,
	contentID uuid.UUID,
) ([]*entity.ContentMedia, error) {
	return s.contentRepo.GetMedia(ctx, tx, contentID)
}

// AddMedia adds media attachments to content.
// AUTHORIZATION: Only the author can add media to their content.
func (s *ContentService) AddMedia(
	ctx context.Context,
	tx db.Tx,
	callerID uuid.UUID,
	contentID uuid.UUID,
	mediaItems []struct {
		MediaURL  string
		MediaType entity.MediaType
		Position  int
	},
) error {
	// Validate caller
	if err := auth.ValidateCaller(callerID); err != nil {
		return err
	}

	// Get content to verify ownership
	content, err := s.contentRepo.GetByID(ctx, tx, contentID)
	if err != nil {
		return err
	}

	// AUTHORIZATION: Only author can add media
	if !auth.IsSystemCaller(callerID) && content.AuthorID != callerID {
		return auth.ErrOwnerRequired
	}

	// Build media entities
	media := make([]*entity.ContentMedia, len(mediaItems))
	for i, item := range mediaItems {
		media[i] = entity.NewContentMedia(contentID, item.MediaURL, item.MediaType, item.Position)
	}

	// Persist media
	if err := s.contentRepo.CreateMedia(ctx, tx, media); err != nil {
		return fmt.Errorf("create media failed: %w", err)
	}

	return nil
}

// GetLikeCount returns the number of likes for a content.
func (s *ContentService) GetLikeCount(
	ctx context.Context,
	tx db.Tx,
	contentID uuid.UUID,
) (int, error) {
	return s.likeRepo.CountLikes(ctx, tx, contentID)
}

// GetCommentCount returns the number of non-deleted top-level comments for a content.
//
// C7C — canonical CommentCount source for EngagementResponse.
// Delegates to CommentRepositoryImpl.CountTopLevelCommentsByContent.
// CommentRepositoryImpl is stateless (zero-value struct); constructing it
// here is zero-cost and avoids adding a commentRepo field to ContentService.
func (s *ContentService) GetCommentCount(
	ctx context.Context,
	tx db.Tx,
	contentID uuid.UUID,
) (int, error) {
	repo := contentrepo.NewCommentRepository()
	return repo.CountTopLevelCommentsByContent(ctx, tx, contentID)
}

// IsLiked checks if a user has liked a content.
func (s *ContentService) IsLiked(
	ctx context.Context,
	tx db.Tx,
	userID, contentID uuid.UUID,
) (bool, error) {
	return s.likeRepo.ExistsLike(ctx, tx, contentID, userID)
}

// GetMentionedUserIDs retrieves the canonical mentioned user IDs for a content item.
// Returns an empty slice when the content has no mentions.
func (s *ContentService) GetMentionedUserIDs(
	ctx context.Context,
	tx db.Tx,
	contentID uuid.UUID,
) ([]uuid.UUID, error) {
	return s.contentRepo.GetMentionedUserIDs(ctx, tx, contentID)
}

// ============================================================================
// MODERATION INTEGRATION
// ============================================================================

// SoftDeleteForModeration soft-deletes content due to moderation removal.
//
// STRICT BOUNDARY RULES:
// - NO ownership check (moderation overrides ownership)
// - NO auth check (called from outbox worker)
// - Idempotent: safe to call on already-deleted content
//
// This method is called by the moderation event handler when a content
// is removed via moderation.case with decision=removed.
func (s *ContentService) SoftDeleteForModeration(
	ctx context.Context,
	tx db.Tx,
	contentID uuid.UUID,
) error {
	// Lock content for update
	content, err := s.contentRepo.GetForUpdate(ctx, tx, contentID)
	if err != nil {
		// If content not found, treat as success (idempotent)
		// This can happen if content was already deleted
		return nil
	}

	// Already deleted? Idempotent - return success
	if content.Status == entity.StatusDeleted {
		return nil
	}

	// Transition to deleted state (bypasses ownership check)
	if err := content.Delete(); err != nil {
		return fmt.Errorf("moderation delete content failed: %w", err)
	}

	// Persist changes
	if err := s.contentRepo.Update(ctx, tx, content); err != nil {
		return fmt.Errorf("moderation update content failed: %w", err)
	}

	return nil
}

// RestoreFromModeration restores content that was soft-deleted due to moderation.
//
// STRICT BOUNDARY RULES:
// - NO ownership check (moderation overrides ownership)
// - NO auth check (called from outbox worker)
// - Idempotent: safe to call on already-restored content
//
// This method is called by the moderation event handler when an appeal
// is approved, restoring content that was previously removed.
func (s *ContentService) RestoreFromModeration(
	ctx context.Context,
	tx db.Tx,
	contentID uuid.UUID,
) error {
	// Lock content for update
	content, err := s.contentRepo.GetForUpdate(ctx, tx, contentID)
	if err != nil {
		// If content not found, treat as success (idempotent)
		return nil
	}

	// Already restored? Idempotent - return success.
	//
	// We key idempotency off the deletion marker rather than status alone so
	// a partially restored row (status flipped but deleted_at still set) can
	// be repaired by the moderation restore path.
	if content.DeletedAt == nil && content.Status != entity.StatusDeleted {
		return nil
	}

	// Restore content to the active public state.
	content.Status = entity.StatusActive

	// Clear the moderation delete marker so public repositories can surface
	// the row again. Preserve hidden state and all ownership/content fields.
	content.DeletedAt = nil
	content.UpdatedAt = time.Now()

	// Persist changes
	if err := s.contentRepo.Update(ctx, tx, content); err != nil {
		return fmt.Errorf("moderation restore content failed: %w", err)
	}

	return nil
}

// ============================================================================
// SHARE CONTRACT V1: Repost Methods
// ============================================================================

// CreateRepostRequest holds the parameters for creating a repost.
type CreateRepostRequest struct {
	// OriginalContentID is the canonical ID of the content being reposted
	OriginalContentID uuid.UUID

	// Caption is the optional caption from the reposter
	// If empty, the repost will have no caption
	Caption string

	// OriginalContentAuthorID is the author ID of the original content
	OriginalContentAuthorID uuid.UUID

	// OriginalContentTitle is retained for legacy callers but no longer
	// participates in backend share authority.
	OriginalContentTitle string

	// OriginalContentImageURL is retained for legacy callers but no longer
	// participates in backend share authority.
	OriginalContentImageURL string
}

// CreateRepost creates a repost of existing content.
//
// SHARE CONTRACT V1:
// - Creates NEW Content as a repost of the original content
// - Original author attribution is preserved via originalAuthorId
// - Source object is NOT mutated
//
// AUTHORIZATION: Any active user can repost content.
// ENFORCES: Original content must exist.
// VALIDATION: Uses the canonical content visibility path for repost source
// verification.
func (s *ContentService) CreateRepost(
	ctx context.Context,
	tx db.Tx,
	callerID uuid.UUID,
	req *CreateRepostRequest,
) (*entity.Content, error) {
	// Validate caller
	if err := auth.ValidateCaller(callerID); err != nil {
		return nil, err
	}

	// ACCOUNT STATUS: Check if caller's account is active
	if err := s.accountStatusChecker.EnsureActive(ctx, callerID); err != nil {
		return nil, err
	}

	return s.internalShareAuthorityOrSelf().CreateInternalShare(ctx, tx, &CreateInternalShareRequest{
		ActorID:          callerID,
		TargetType:       entity.ShareTargetTypeContent,
		TargetID:         req.OriginalContentID.String(),
		Caption:          req.Caption,
		SourceEntrypoint: internalShareSourceEntrypointRepostEndpoint,
		OriginalAuthorID: &req.OriginalContentAuthorID,
	})
}

// UpdateCaptionAndVisibility updates content caption and visibility.
// AUTHORIZATION: Only the author can update their content.
// ENFORCES: Content must exist and caller must be the author.
func (s *ContentService) UpdateCaptionAndVisibility(
	ctx context.Context,
	tx db.Tx,
	callerID uuid.UUID,
	contentID uuid.UUID,
	caption *string,
	visibility *string,
) error {
	// Validate caller
	if err := auth.ValidateCaller(callerID); err != nil {
		return err
	}

	// Get content with lock for update
	content, err := s.contentRepo.GetForUpdate(ctx, tx, contentID)
	if err != nil {
		return fmt.Errorf("get content for update failed: %w", err)
	}

	// AUTHORIZATION: Only author can update
	if !auth.IsSystemCaller(callerID) && content.AuthorID != callerID {
		return auth.ErrOwnerRequired
	}

	// Update caption if provided
	if caption != nil {
		content.Caption = caption
	}

	// Update visibility/hide status
	if visibility != nil {
		content.Visibility = entity.Visibility(*visibility).Normalize()
		switch content.Visibility {
		case entity.VisibilityPrivate:
			content.IsHidden = true
		default:
			content.IsHidden = false
		}
	}

	// Persist changes
	if err := s.contentRepo.Update(ctx, tx, content); err != nil {
		return fmt.Errorf("update content failed: %w", err)
	}

	return nil
}

func createContentResourceOccurrence(
	ctx context.Context,
	tx interface{},
	repo any,
	occurrence *entity.ContentResourceOccurrence,
) error {
	if repo == nil {
		return fmt.Errorf("content repository does not support resource occurrences")
	}
	writer, ok := repo.(contentResourceOccurrenceWriter)
	if !ok {
		return fmt.Errorf("content repository does not support resource occurrences")
	}
	return writer.CreateResourceOccurrence(ctx, tx, occurrence)
}

func getContentResourceOccurrenceByContentID(
	ctx context.Context,
	tx interface{},
	repo any,
	contentID uuid.UUID,
) (*entity.ContentResourceOccurrence, error) {
	if repo == nil {
		return nil, fmt.Errorf("content repository does not support resource occurrences")
	}
	reader, ok := repo.(contentResourceOccurrenceReader)
	if !ok {
		return nil, fmt.Errorf("content repository does not support resource occurrences")
	}
	return reader.GetResourceOccurrenceByContentID(ctx, tx, contentID)
}
