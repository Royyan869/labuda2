package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/events"
	"github.com/labuda/backend/internal/social/content/entity"
	"github.com/labuda/backend/internal/social/like"
	likeentity "github.com/labuda/backend/internal/social/like/entity"
	"github.com/labuda/backend/pkg/db"
)

// Transactor represents the ability to execute functions within transactions.
type Transactor interface {
	WithTx(ctx context.Context, fn func(tx db.Tx) error) error
}

// BlockChecker defines the interface for checking block relationships.
type BlockChecker interface {
	ExistsBlock(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (bool, error)
}

// ContentRepository interface for dependency injection.
type ContentRepository interface {
	GetByID(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error)
}

// OutboxInserter defines the interface for inserting outbox events.
type OutboxInserter interface {
	InsertTx(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error
}

// LikeNotificationScrubber removes the content.liked notification a liker
// produced on a content. Called on UNLIKE within the same transaction as the
// like-row deletion so a later LIKE is a new occurrence and can re-notify.
// Nil-safe: a nil scrubber skips the scrub (notification correctness requires
// the real dependency at the composition root).
type LikeNotificationScrubber interface {
	DeleteContentLikedNotification(ctx context.Context, tx interface{}, likerID, contentID uuid.UUID) error
}

// likeLikedIdempotencyKey scopes the content.liked outbox key to a LIKE
// occurrence. The created_at is the (content, user) like row's singleton
// timestamp; within one occurrence concurrent/retry inserts read the same
// value (dedup preserved) while a LIKE after an UNLIKE has a fresh created_at
// (new key => new delivery).
func likeLikedIdempotencyKey(contentID, userID uuid.UUID, createdAt time.Time) string {
	return fmt.Sprintf("content.liked.%s.%s.%d", contentID, userID, createdAt.UnixNano())
}

// Service handles like business logic.
type Service struct {
	db                   Transactor
	likeRepo             like.Repository
	contentRepo          ContentRepository
	outboxRepo           OutboxInserter
	blockChecker         BlockChecker             // For filtering likes from blocked users
	invariantLogger      InvariantLogger          // Logs invariant violations for monitoring
	notificationScrubber LikeNotificationScrubber // Removes stale like notifications on unlike
}

// NewService creates a new LikeService with explicit dependencies.
// blockChecker can be nil - if nil, no block filtering will be performed.
// invariantLogger is optional - if nil, no invariant violation logging will occur.
// notificationScrubber is optional but required for re-like notification
// correctness (see LikeNotificationScrubber).
func NewService(db Transactor, contentRepo ContentRepository, likeRepo like.Repository, outboxRepo OutboxInserter, blockChecker BlockChecker, invariantLogger InvariantLogger, notificationScrubber LikeNotificationScrubber) *Service {
	return &Service{
		db:                   db,
		likeRepo:             likeRepo,
		contentRepo:          contentRepo,
		outboxRepo:           outboxRepo,
		blockChecker:         blockChecker,
		invariantLogger:      invariantLogger,
		notificationScrubber: notificationScrubber,
	}
}

// Like creates a like on content by a user.
//
// Transaction flow:
// 1. BEGIN
// 2. Validate content exists
// 3. Reject if deleted
// 4. Check block relationship
// 5. Insert like
// 6. Insert outbox event for notification
// 7. COMMIT
//
// Validates:
// - Content exists
// - Content status is not deleted
// - No block exists between user and content author
// Returns nil on success (idempotent - can be called multiple times).
func (s *Service) Like(ctx context.Context, contentID, userID uuid.UUID) error {
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		// Validate content exists and is not deleted
		content, err := s.contentRepo.GetByID(ctx, tx, contentID)
		if err != nil {
			if err.Error() == fmt.Sprintf("content not found: %s", contentID) {
				return &likeentity.ErrContentNotFound{ContentID: contentID}
			}
			return fmt.Errorf("get content failed: %w", err)
		}

		// Validate content is not deleted
		if content.Status == entity.StatusDeleted {
			LikeOnDeletedContentViolation(ctx, s.invariantLogger, userID, contentID)
			return &likeentity.ErrContentDeleted{ContentID: contentID}
		}

		// Validate content is not hidden (moderated/private). Mirrors the
		// canonical content visibility authority: hidden content is treated as
		// not-found so its existence is not leaked through the like surface.
		if content.IsHidden {
			return &likeentity.ErrContentNotFound{ContentID: contentID}
		}

		// BLOCK FILTERING: Check if user is blocked by content author
		// Don't allow liking own content (that's fine), but check for blocks
		if s.blockChecker != nil && userID != content.AuthorID {
			blocked, err := s.blockChecker.ExistsBlock(ctx, tx, userID, content.AuthorID)
			if err != nil {
				return fmt.Errorf("failed to check block status: %w", err)
			}
			if blocked {
				return &likeentity.ErrContentNotFound{ContentID: contentID} // Treat as not found to avoid leaking block info
			}
		}

		// Insert like (idempotent via ON CONFLICT DO NOTHING)
		if err := s.likeRepo.InsertLike(ctx, tx, contentID, userID); err != nil {
			return fmt.Errorf("insert like failed: %w", err)
		}

		// Read back the like occurrence timestamp. Under concurrent double
		// toggles both transactions settle on the same committed row, so
		// retries dedup to a single outbox event.
		createdAt, err := s.likeRepo.GetLikeCreatedAt(ctx, tx, contentID, userID)
		if err != nil {
			return fmt.Errorf("get like created_at failed: %w", err)
		}

		// Insert outbox event for notification delivery
		// EventType: events.EventContentLiked
		// AggregateType: "content"
		// AggregateID: contentID
		payload := map[string]any{
			"actor_id":      userID,
			"recipient_id":  content.AuthorID,
			"content_id":    contentID,
			"occurrence_at": createdAt.Format(time.RFC3339Nano),
		}
		// Idempotency key: content.liked.{contentID}.{userID}.{likeCreatedAt}
		idempotencyKey := likeLikedIdempotencyKey(contentID, userID, createdAt)
		if err := s.outboxRepo.InsertTx(ctx, tx, events.EventContentLiked, payload, idempotencyKey); err != nil {
			return fmt.Errorf("insert outbox event failed: %w", err)
		}

		return nil
	})
}

// Unlike removes a like on content by a user.
// Idempotent - returns nil even if like doesn't exist.
//
// Notification lifecycle: removing the like also removes any content.liked
// notification this user produced on the content, so a later LIKE is a new
// occurrence that can notify again. The scrub is atomic with the row delete.
func (s *Service) Unlike(ctx context.Context, contentID, userID uuid.UUID) error {
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		if err := s.likeRepo.DeleteLike(ctx, tx, contentID, userID); err != nil {
			return fmt.Errorf("delete like failed: %w", err)
		}

		if s.notificationScrubber != nil {
			if err := s.notificationScrubber.DeleteContentLikedNotification(ctx, tx, userID, contentID); err != nil {
				return fmt.Errorf("delete like notification failed: %w", err)
			}
		}

		return nil
	})
}

// ToggleResult holds the outcome of a content like toggle.
type ToggleResult struct {
	Liked bool
	Count int
}

// ToggleContentLike atomically toggles a like on content.
//
// Transaction flow:
// 1. BEGIN
// 2. Check if user already liked this content
// 3. If liked → delete like (unlike) — no validation needed
// 4. If not liked → validate content exists + not deleted + not blocked → insert like → emit outbox
// 5. Count likes
// 6. COMMIT
//
// The unlike path skips content/block validation intentionally: a user who
// previously liked content that was later deleted or whose author blocked them
// should still be able to unlike.
func (s *Service) ToggleContentLike(ctx context.Context, contentID, userID uuid.UUID) (ToggleResult, error) {
	var result ToggleResult
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		isLiked, err := s.likeRepo.ExistsLike(ctx, tx, contentID, userID)
		if err != nil {
			return fmt.Errorf("check like exists failed: %w", err)
		}

		if isLiked {
			// Unlike path: no validation needed (idempotent delete).
			if err := s.likeRepo.DeleteLike(ctx, tx, contentID, userID); err != nil {
				return fmt.Errorf("delete like failed: %w", err)
			}

			// Release the like occurrence: scrub the content.liked notification
			// so a later LIKE can notify again (atomic with the row delete).
			if s.notificationScrubber != nil {
				if err := s.notificationScrubber.DeleteContentLikedNotification(ctx, tx, userID, contentID); err != nil {
					return fmt.Errorf("delete like notification failed: %w", err)
				}
			}

			result.Liked = false
		} else {
			// Like path: full validation.
			content, err := s.contentRepo.GetByID(ctx, tx, contentID)
			if err != nil {
				if err.Error() == fmt.Sprintf("content not found: %s", contentID) {
					return &likeentity.ErrContentNotFound{ContentID: contentID}
				}
				return fmt.Errorf("get content failed: %w", err)
			}

			if content.Status == entity.StatusDeleted {
				LikeOnDeletedContentViolation(ctx, s.invariantLogger, userID, contentID)
				return &likeentity.ErrContentDeleted{ContentID: contentID}
			}

			if content.IsHidden {
				return &likeentity.ErrContentNotFound{ContentID: contentID}
			}

			if s.blockChecker != nil && userID != content.AuthorID {
				blocked, err := s.blockChecker.ExistsBlock(ctx, tx, userID, content.AuthorID)
				if err != nil {
					return fmt.Errorf("failed to check block status: %w", err)
				}
				if blocked {
					return &likeentity.ErrContentNotFound{ContentID: contentID}
				}
			}

			if err := s.likeRepo.InsertLike(ctx, tx, contentID, userID); err != nil {
				return fmt.Errorf("insert like failed: %w", err)
			}

			createdAt, err := s.likeRepo.GetLikeCreatedAt(ctx, tx, contentID, userID)
			if err != nil {
				return fmt.Errorf("get like created_at failed: %w", err)
			}

			payload := map[string]any{
				"actor_id":      userID,
				"recipient_id":  content.AuthorID,
				"content_id":    contentID,
				"occurrence_at": createdAt.Format(time.RFC3339Nano),
			}
			idempotencyKey := likeLikedIdempotencyKey(contentID, userID, createdAt)
			if err := s.outboxRepo.InsertTx(ctx, tx, events.EventContentLiked, payload, idempotencyKey); err != nil {
				return fmt.Errorf("insert outbox event failed: %w", err)
			}

			result.Liked = true
		}

		count, err := s.likeRepo.CountLikes(ctx, tx, contentID)
		if err != nil {
			return fmt.Errorf("count likes failed: %w", err)
		}
		result.Count = count

		return nil
	})
	return result, err
}

// IsContentLikeReadable reports whether like metadata (count, is_liked) for a
// content may be disclosed to a viewer. Mirrors the canonical content
// visibility authority used by content detail / feed / profile: visible iff
// the content exists, is not deleted, is not hidden, and no block exists
// between the viewer and the author. viewerID may be uuid.Nil (anonymous
// viewers are never in a block relationship). Used to gate GET /likes/stats.
func (s *Service) IsContentLikeReadable(ctx context.Context, tx db.Tx, contentID, viewerID uuid.UUID) (bool, error) {
	content, err := s.contentRepo.GetByID(ctx, tx, contentID)
	if err != nil {
		if strings.HasPrefix(err.Error(), "content not found:") {
			return false, nil
		}
		return false, fmt.Errorf("get content failed: %w", err)
	}

	if content.Status == entity.StatusDeleted || content.IsHidden {
		return false, nil
	}

	if viewerID == uuid.Nil || viewerID == content.AuthorID {
		return true, nil
	}

	if s.blockChecker != nil {
		blocked, err := s.blockChecker.ExistsBlock(ctx, tx, viewerID, content.AuthorID)
		if err != nil {
			return false, fmt.Errorf("failed to check block status: %w", err)
		}
		if blocked {
			return false, nil
		}
	}

	return true, nil
}
