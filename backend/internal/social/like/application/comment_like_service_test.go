package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	contententity "github.com/labuda/backend/internal/social/content/entity"
	"github.com/labuda/backend/internal/social/like/application"
	likeentity "github.com/labuda/backend/internal/social/like/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCommentRepository struct {
	getByIDFunc func(ctx context.Context, tx db.Tx, id uuid.UUID) (*contententity.Comment, error)
}

func (m *mockCommentRepository) GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*contententity.Comment, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, tx, id)
	}
	return nil, errors.New("comment not found")
}

type mockCommentLikeRepository struct {
	insertLikeFunc func(ctx context.Context, tx db.Tx, targetID uuid.UUID, targetType likeentity.TargetType, userID uuid.UUID) error
	deleteLikeFunc func(ctx context.Context, tx db.Tx, targetID uuid.UUID, targetType likeentity.TargetType, userID uuid.UUID) error
	existsLikeFunc func(ctx context.Context, tx db.Tx, targetID uuid.UUID, targetType likeentity.TargetType, userID uuid.UUID) (bool, error)
	countLikesFunc func(ctx context.Context, tx db.Tx, targetID uuid.UUID, targetType likeentity.TargetType) (int, error)
}

func (m *mockCommentLikeRepository) InsertLike(ctx context.Context, tx db.Tx, targetID uuid.UUID, targetType likeentity.TargetType, userID uuid.UUID) error {
	if m.insertLikeFunc != nil {
		return m.insertLikeFunc(ctx, tx, targetID, targetType, userID)
	}
	return nil
}

func (m *mockCommentLikeRepository) DeleteLike(ctx context.Context, tx db.Tx, targetID uuid.UUID, targetType likeentity.TargetType, userID uuid.UUID) error {
	if m.deleteLikeFunc != nil {
		return m.deleteLikeFunc(ctx, tx, targetID, targetType, userID)
	}
	return nil
}

func (m *mockCommentLikeRepository) ExistsLike(ctx context.Context, tx db.Tx, targetID uuid.UUID, targetType likeentity.TargetType, userID uuid.UUID) (bool, error) {
	if m.existsLikeFunc != nil {
		return m.existsLikeFunc(ctx, tx, targetID, targetType, userID)
	}
	return false, nil
}

func (m *mockCommentLikeRepository) CountLikes(ctx context.Context, tx db.Tx, targetID uuid.UUID, targetType likeentity.TargetType) (int, error) {
	if m.countLikesFunc != nil {
		return m.countLikesFunc(ctx, tx, targetID, targetType)
	}
	return 0, nil
}

func TestCommentLikeService_ToggleCommentLike(t *testing.T) {
	t.Run("likes active comment successfully", func(t *testing.T) {
		commentID := uuid.New()
		contentID := uuid.New()
		userID := uuid.New()
		commentAuthorID := uuid.New()
		contentAuthorID := uuid.New()
		inserted := false

		contentRepo := &mockContentRepository{
			getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*contententity.Content, error) {
				return &contententity.Content{
					ID:       contentID,
					AuthorID: contentAuthorID,
					Status:   contententity.StatusActive,
				}, nil
			},
		}

		commentRepo := &mockCommentRepository{
			getByIDFunc: func(ctx context.Context, tx db.Tx, id uuid.UUID) (*contententity.Comment, error) {
				return &contententity.Comment{
					ID:       commentID,
					AuthorID: commentAuthorID,
					TargetID: contentID,
				}, nil
			},
		}

		likeRepo := &mockCommentLikeRepository{
			existsLikeFunc: func(ctx context.Context, tx db.Tx, targetID uuid.UUID, targetType likeentity.TargetType, uid uuid.UUID) (bool, error) {
				return false, nil
			},
			insertLikeFunc: func(ctx context.Context, tx db.Tx, targetID uuid.UUID, targetType likeentity.TargetType, uid uuid.UUID) error {
				inserted = true
				return nil
			},
			countLikesFunc: func(ctx context.Context, tx db.Tx, targetID uuid.UUID, targetType likeentity.TargetType) (int, error) {
				return 1, nil
			},
		}

		service := application.NewCommentLikeService(&mockTransactor{}, contentRepo, commentRepo, likeRepo, &mockBlockChecker{})

		result, err := service.ToggleCommentLike(context.Background(), commentID, userID)

		require.NoError(t, err)
		assert.True(t, result.Liked)
		assert.Equal(t, 1, result.Count)
		assert.True(t, inserted)
	})

	t.Run("rejects deleted comment", func(t *testing.T) {
		commentID := uuid.New()
		contentID := uuid.New()
		userID := uuid.New()
		deletedAt := time.Now()

		contentRepo := &mockContentRepository{
			getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*contententity.Content, error) {
				return &contententity.Content{
					ID:     contentID,
					Status: contententity.StatusActive,
				}, nil
			},
		}

		commentRepo := &mockCommentRepository{
			getByIDFunc: func(ctx context.Context, tx db.Tx, id uuid.UUID) (*contententity.Comment, error) {
				return &contententity.Comment{
					ID:        commentID,
					AuthorID:  uuid.New(),
					TargetID:  contentID,
					DeletedAt: &deletedAt,
				}, nil
			},
		}

		service := application.NewCommentLikeService(&mockTransactor{}, contentRepo, commentRepo, &mockCommentLikeRepository{}, &mockBlockChecker{})

		_, err := service.ToggleCommentLike(context.Background(), commentID, userID)

		require.Error(t, err)
		var notFoundErr *likeentity.ErrTargetNotFound
		assert.ErrorAs(t, err, &notFoundErr)
	})

	t.Run("rejects comment under deleted content", func(t *testing.T) {
		commentID := uuid.New()
		contentID := uuid.New()
		userID := uuid.New()

		contentRepo := &mockContentRepository{
			getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*contententity.Content, error) {
				return &contententity.Content{
					ID:     contentID,
					Status: contententity.StatusDeleted,
				}, nil
			},
		}

		commentRepo := &mockCommentRepository{
			getByIDFunc: func(ctx context.Context, tx db.Tx, id uuid.UUID) (*contententity.Comment, error) {
				return &contententity.Comment{
					ID:       commentID,
					AuthorID: uuid.New(),
					TargetID: contentID,
				}, nil
			},
		}

		service := application.NewCommentLikeService(&mockTransactor{}, contentRepo, commentRepo, &mockCommentLikeRepository{}, &mockBlockChecker{})

		_, err := service.ToggleCommentLike(context.Background(), commentID, userID)

		require.Error(t, err)
		var deletedErr *likeentity.ErrContentDeleted
		assert.ErrorAs(t, err, &deletedErr)
	})

	t.Run("rejects comment under hidden content", func(t *testing.T) {
		commentID := uuid.New()
		contentID := uuid.New()
		userID := uuid.New()

		contentRepo := &mockContentRepository{
			getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*contententity.Content, error) {
				return &contententity.Content{
					ID:       contentID,
					Status:   contententity.StatusActive,
					IsHidden: true,
				}, nil
			},
		}

		commentRepo := &mockCommentRepository{
			getByIDFunc: func(ctx context.Context, tx db.Tx, id uuid.UUID) (*contententity.Comment, error) {
				return &contententity.Comment{
					ID:       commentID,
					AuthorID: uuid.New(),
					TargetID: contentID,
				}, nil
			},
		}

		service := application.NewCommentLikeService(&mockTransactor{}, contentRepo, commentRepo, &mockCommentLikeRepository{}, &mockBlockChecker{})

		_, err := service.ToggleCommentLike(context.Background(), commentID, userID)

		require.Error(t, err)
		var notFoundErr *likeentity.ErrContentNotFound
		assert.ErrorAs(t, err, &notFoundErr, "hidden parent content must block comment likes via not-found")
	})

	t.Run("rejects when blocked by comment author", func(t *testing.T) {
		commentID := uuid.New()
		contentID := uuid.New()
		userID := uuid.New()
		commentAuthorID := uuid.New()
		contentAuthorID := uuid.New()

		contentRepo := &mockContentRepository{
			getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*contententity.Content, error) {
				return &contententity.Content{
					ID:       contentID,
					AuthorID: contentAuthorID,
					Status:   contententity.StatusActive,
				}, nil
			},
		}

		commentRepo := &mockCommentRepository{
			getByIDFunc: func(ctx context.Context, tx db.Tx, id uuid.UUID) (*contententity.Comment, error) {
				return &contententity.Comment{
					ID:       commentID,
					AuthorID: commentAuthorID,
					TargetID: contentID,
				}, nil
			},
		}

		blocker := &mockBlockChecker{
			existsBlockFunc: func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (bool, error) {
				return true, nil
			},
		}

		service := application.NewCommentLikeService(&mockTransactor{}, contentRepo, commentRepo, &mockCommentLikeRepository{}, blocker)

		_, err := service.ToggleCommentLike(context.Background(), commentID, userID)

		require.Error(t, err)
		var notFoundErr *likeentity.ErrTargetNotFound
		assert.ErrorAs(t, err, &notFoundErr)
	})

	t.Run("rejects when blocked by parent content author", func(t *testing.T) {
		commentID := uuid.New()
		contentID := uuid.New()
		userID := uuid.New()
		commentAuthorID := uuid.New()
		contentAuthorID := uuid.New()
		blockCalls := 0

		contentRepo := &mockContentRepository{
			getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*contententity.Content, error) {
				return &contententity.Content{
					ID:       contentID,
					AuthorID: contentAuthorID,
					Status:   contententity.StatusActive,
				}, nil
			},
		}

		commentRepo := &mockCommentRepository{
			getByIDFunc: func(ctx context.Context, tx db.Tx, id uuid.UUID) (*contententity.Comment, error) {
				return &contententity.Comment{
					ID:       commentID,
					AuthorID: commentAuthorID,
					TargetID: contentID,
				}, nil
			},
		}

		blocker := &mockBlockChecker{
			existsBlockFunc: func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (bool, error) {
				blockCalls++
				return blockCalls == 2, nil
			},
		}

		service := application.NewCommentLikeService(&mockTransactor{}, contentRepo, commentRepo, &mockCommentLikeRepository{}, blocker)

		_, err := service.ToggleCommentLike(context.Background(), commentID, userID)

		require.Error(t, err)
		var notFoundErr *likeentity.ErrTargetNotFound
		assert.ErrorAs(t, err, &notFoundErr)
		assert.Equal(t, 2, blockCalls)
	})

	t.Run("unlike remains idempotent even if target is now deleted", func(t *testing.T) {
		commentID := uuid.New()
		userID := uuid.New()
		deleteCount := 0

		likeRepo := &mockCommentLikeRepository{
			existsLikeFunc: func(ctx context.Context, tx db.Tx, targetID uuid.UUID, targetType likeentity.TargetType, uid uuid.UUID) (bool, error) {
				return true, nil
			},
			deleteLikeFunc: func(ctx context.Context, tx db.Tx, targetID uuid.UUID, targetType likeentity.TargetType, uid uuid.UUID) error {
				deleteCount++
				return nil
			},
			countLikesFunc: func(ctx context.Context, tx db.Tx, targetID uuid.UUID, targetType likeentity.TargetType) (int, error) {
				return 0, nil
			},
		}

		contentRepo := &mockContentRepository{
			getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*contententity.Content, error) {
				t.Fatalf("content lookup should not run on unlike")
				return nil, nil
			},
		}

		commentRepo := &mockCommentRepository{
			getByIDFunc: func(ctx context.Context, tx db.Tx, id uuid.UUID) (*contententity.Comment, error) {
				t.Fatalf("comment lookup should not run on unlike")
				return nil, nil
			},
		}

		blocker := &mockBlockChecker{
			existsBlockFunc: func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (bool, error) {
				t.Fatalf("block lookup should not run on unlike")
				return false, nil
			},
		}

		service := application.NewCommentLikeService(&mockTransactor{}, contentRepo, commentRepo, likeRepo, blocker)

		result, err := service.ToggleCommentLike(context.Background(), commentID, userID)

		require.NoError(t, err)
		assert.False(t, result.Liked)
		assert.Equal(t, 1, deleteCount)
	})

	t.Run("duplicate like remains idempotent", func(t *testing.T) {
		commentID := uuid.New()
		contentID := uuid.New()
		userID := uuid.New()
		commentAuthorID := uuid.New()
		contentAuthorID := uuid.New()
		insertCount := 0

		contentRepo := &mockContentRepository{
			getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*contententity.Content, error) {
				return &contententity.Content{
					ID:       contentID,
					AuthorID: contentAuthorID,
					Status:   contententity.StatusActive,
				}, nil
			},
		}

		commentRepo := &mockCommentRepository{
			getByIDFunc: func(ctx context.Context, tx db.Tx, id uuid.UUID) (*contententity.Comment, error) {
				return &contententity.Comment{
					ID:       commentID,
					AuthorID: commentAuthorID,
					TargetID: contentID,
				}, nil
			},
		}

		likeRepo := &mockCommentLikeRepository{
			existsLikeFunc: func(ctx context.Context, tx db.Tx, targetID uuid.UUID, targetType likeentity.TargetType, uid uuid.UUID) (bool, error) {
				return false, nil
			},
			insertLikeFunc: func(ctx context.Context, tx db.Tx, targetID uuid.UUID, targetType likeentity.TargetType, uid uuid.UUID) error {
				insertCount++
				return nil
			},
			countLikesFunc: func(ctx context.Context, tx db.Tx, targetID uuid.UUID, targetType likeentity.TargetType) (int, error) {
				return 1, nil
			},
		}

		service := application.NewCommentLikeService(&mockTransactor{}, contentRepo, commentRepo, likeRepo, &mockBlockChecker{})

		first, err1 := service.ToggleCommentLike(context.Background(), commentID, userID)
		second, err2 := service.ToggleCommentLike(context.Background(), commentID, userID)

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.True(t, first.Liked)
		assert.True(t, second.Liked)
		assert.Equal(t, 2, insertCount)
	})
}
