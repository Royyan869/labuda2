package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	contententity "github.com/labuda/backend/internal/social/content/entity"
	"github.com/labuda/backend/internal/social/like/entity"
	"github.com/labuda/backend/pkg/db"
)

// CommentRepository defines the minimum comment persistence needed for comment likes.
type CommentRepository interface {
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*contententity.Comment, error)
}

// CommentLikeRepository defines the target-like persistence needed for comment likes.
type CommentLikeRepository interface {
	InsertLike(ctx context.Context, tx db.Tx, targetID uuid.UUID, targetType entity.TargetType, userID uuid.UUID) error
	DeleteLike(ctx context.Context, tx db.Tx, targetID uuid.UUID, targetType entity.TargetType, userID uuid.UUID) error
	ExistsLike(ctx context.Context, tx db.Tx, targetID uuid.UUID, targetType entity.TargetType, userID uuid.UUID) (bool, error)
	CountLikes(ctx context.Context, tx db.Tx, targetID uuid.UUID, targetType entity.TargetType) (int, error)
}

// CommentLikeService handles governed like toggles for comments.
//
// Comment likes share the same mutation pattern as content likes, but the
// validation surface is narrower:
//   - comment must exist
//   - comment must not be soft-deleted
//   - parent content must exist and not be deleted
//   - block checks must be enforced against the comment author and parent
//     content author
//   - unlike remains idempotent and skips validation
type CommentLikeService struct {
	db           Transactor
	likeRepo     CommentLikeRepository
	contentRepo  ContentRepository
	commentRepo  CommentRepository
	blockChecker BlockChecker
}

// NewCommentLikeService creates a new comment like service with the provided dependencies.
func NewCommentLikeService(
	db Transactor,
	contentRepo ContentRepository,
	commentRepo CommentRepository,
	likeRepo CommentLikeRepository,
	blockChecker BlockChecker,
) *CommentLikeService {
	return &CommentLikeService{
		db:           db,
		likeRepo:     likeRepo,
		contentRepo:  contentRepo,
		commentRepo:  commentRepo,
		blockChecker: blockChecker,
	}
}

// ToggleCommentLike atomically toggles a like on a comment.
//
// The unlike path intentionally skips validation so a user can remove a like
// even if the comment or parent content has since been deleted or blocked.
func (s *CommentLikeService) ToggleCommentLike(ctx context.Context, commentID, userID uuid.UUID) (ToggleResult, error) {
	var result ToggleResult

	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		isLiked, err := s.likeRepo.ExistsLike(ctx, tx, commentID, entity.TargetTypeComment, userID)
		if err != nil {
			return fmt.Errorf("check like exists failed: %w", err)
		}

		if isLiked {
			if err := s.likeRepo.DeleteLike(ctx, tx, commentID, entity.TargetTypeComment, userID); err != nil {
				return fmt.Errorf("delete like failed: %w", err)
			}
			result.Liked = false
		} else {
			if err := s.validateCommentLikeTarget(ctx, tx, commentID, userID); err != nil {
				return err
			}

			if err := s.likeRepo.InsertLike(ctx, tx, commentID, entity.TargetTypeComment, userID); err != nil {
				return fmt.Errorf("insert like failed: %w", err)
			}
			result.Liked = true
		}

		count, err := s.likeRepo.CountLikes(ctx, tx, commentID, entity.TargetTypeComment)
		if err != nil {
			return fmt.Errorf("count likes failed: %w", err)
		}
		result.Count = count

		return nil
	})

	return result, err
}

func (s *CommentLikeService) validateCommentLikeTarget(ctx context.Context, tx db.Tx, commentID, userID uuid.UUID) error {
	comment, err := s.commentRepo.GetByID(ctx, tx, commentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &entity.ErrTargetNotFound{TargetID: commentID, TargetType: entity.TargetTypeComment}
		}
		return fmt.Errorf("get comment failed: %w", err)
	}

	if comment.DeletedAt != nil {
		return &entity.ErrTargetNotFound{TargetID: commentID, TargetType: entity.TargetTypeComment}
	}

	content, err := s.contentRepo.GetByID(ctx, tx, comment.TargetID)
	if err != nil {
		if strings.HasPrefix(err.Error(), "content not found:") {
			return &entity.ErrContentNotFound{ContentID: comment.TargetID}
		}
		return fmt.Errorf("get content failed: %w", err)
	}

	if content.Status == contententity.StatusDeleted {
		return &entity.ErrContentDeleted{ContentID: comment.TargetID}
	}

	// Hidden (moderated/private) parent content is not publicly accessible:
	// mirror the comment-list / content-detail authority by treating it as
	// not-found so no existence info leaks through the like surface.
	if content.IsHidden {
		return &entity.ErrContentNotFound{ContentID: comment.TargetID}
	}

	if s.blockChecker != nil {
		if userID != comment.AuthorID {
			blocked, err := s.blockChecker.ExistsBlock(ctx, tx, userID, comment.AuthorID)
			if err != nil {
				return fmt.Errorf("failed to check block status: %w", err)
			}
			if blocked {
				return &entity.ErrTargetNotFound{TargetID: commentID, TargetType: entity.TargetTypeComment}
			}
		}

		if comment.AuthorID != content.AuthorID && userID != content.AuthorID {
			blocked, err := s.blockChecker.ExistsBlock(ctx, tx, userID, content.AuthorID)
			if err != nil {
				return fmt.Errorf("failed to check block status: %w", err)
			}
			if blocked {
				return &entity.ErrTargetNotFound{TargetID: commentID, TargetType: entity.TargetTypeComment}
			}
		}
	}

	return nil
}

// IsCommentLikeReadable reports whether like metadata for a comment may be
// disclosed to a viewer. Uses the same visibility authority as
// validateCommentLikeTarget (comment exists, not soft-deleted, parent content
// visible, no blocks) but read-oriented with viewerID (uuid.Nil = anonymous).
// Used to gate GET /likes/stats.
func (s *CommentLikeService) IsCommentLikeReadable(ctx context.Context, tx db.Tx, commentID, viewerID uuid.UUID) (bool, error) {
	comment, err := s.commentRepo.GetByID(ctx, tx, commentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("get comment failed: %w", err)
	}

	if comment.DeletedAt != nil {
		return false, nil
	}

	content, err := s.contentRepo.GetByID(ctx, tx, comment.TargetID)
	if err != nil {
		if strings.HasPrefix(err.Error(), "content not found:") {
			return false, nil
		}
		return false, fmt.Errorf("get content failed: %w", err)
	}

	if content.Status == contententity.StatusDeleted || content.IsHidden {
		return false, nil
	}

	if viewerID == uuid.Nil || (viewerID == comment.AuthorID && viewerID == content.AuthorID) {
		return true, nil
	}

	if s.blockChecker != nil {
		if viewerID != comment.AuthorID {
			blocked, err := s.blockChecker.ExistsBlock(ctx, tx, viewerID, comment.AuthorID)
			if err != nil {
				return false, fmt.Errorf("failed to check block status: %w", err)
			}
			if blocked {
				return false, nil
			}
		}

		if comment.AuthorID != content.AuthorID && viewerID != content.AuthorID {
			blocked, err := s.blockChecker.ExistsBlock(ctx, tx, viewerID, content.AuthorID)
			if err != nil {
				return false, fmt.Errorf("failed to check block status: %w", err)
			}
			if blocked {
				return false, nil
			}
		}
	}

	return true, nil
}
