package application_test

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/platform/events"
	"github.com/labuda/backend/internal/social/content/entity"
	"github.com/labuda/backend/internal/social/like/application"
	likeentity "github.com/labuda/backend/internal/social/like/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLikeTx is a mock implementation of db.Tx for testing.
type mockLikeTx struct{}

func (m *mockLikeTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (m *mockLikeTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return &mockLikeRows{}, nil
}

func (m *mockLikeTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return &mockLikeRow{}
}

func (m *mockLikeTx) Commit(ctx context.Context) error {
	return nil
}

func (m *mockLikeTx) Rollback(ctx context.Context) error {
	return nil
}

// mockTransactor is a mock implementation of Transactor for testing.
type mockTransactor struct{}

func (m *mockTransactor) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	return fn(&mockLikeTx{})
}

// mockLikeRows is a mock implementation of pgx.Rows.
type mockLikeRows struct {
	values    [][]any
	nextCall  int
	scanCalls int
	closeCall bool
}

func (m *mockLikeRows) Close() {
	m.closeCall = true
}

func (m *mockLikeRows) Err() error {
	return nil
}

func (m *mockLikeRows) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}

func (m *mockLikeRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (m *mockLikeRows) Next() bool {
	if m.values == nil {
		return false
	}
	if m.nextCall >= len(m.values) {
		return false
	}
	m.nextCall++
	return true
}

func (m *mockLikeRows) Scan(dest ...any) error {
	m.scanCalls++
	if m.values == nil || m.nextCall > len(m.values) {
		return nil
	}
	row := m.values[m.nextCall-1]
	for i, val := range row {
		if dest[i] != nil {
			if ptr, ok := dest[i].(*uuid.UUID); ok {
				if uid, ok := val.(uuid.UUID); ok {
					*ptr = uid
				}
			}
			if ptr, ok := dest[i].(*int); ok {
				if count, ok := val.(int); ok {
					*ptr = count
				}
			}
		}
	}
	return nil
}

func (m *mockLikeRows) Values() ([]any, error) {
	return nil, nil
}

func (m *mockLikeRows) RawValues() [][]byte {
	return nil
}

func (m *mockLikeRows) Conn() *pgx.Conn {
	return nil
}

// mockLikeRow is a mock implementation of pgx.Row.
type mockLikeRow struct {
	scanFunc func(dest ...any) error
}

func (m *mockLikeRow) Scan(dest ...any) error {
	if m.scanFunc != nil {
		return m.scanFunc(dest...)
	}
	return nil
}

// mockContentRepository is a mock implementation of ContentRepository.
type mockContentRepository struct {
	getByIDFunc func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error)
}

func (m *mockContentRepository) Create(ctx context.Context, tx interface{}, content *entity.Content) error {
	return nil
}

func (m *mockContentRepository) CreateMedia(ctx context.Context, tx interface{}, media []*entity.ContentMedia) error {
	return nil
}

func (m *mockContentRepository) GetByID(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, tx, id)
	}
	return nil, errors.New("content not found")
}

func (m *mockContentRepository) GetForUpdate(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, tx, id)
	}
	return nil, errors.New("content not found")
}

func (m *mockContentRepository) Update(ctx context.Context, tx interface{}, content *entity.Content) error {
	return nil
}

func (m *mockContentRepository) ListByAuthor(ctx context.Context, tx interface{}, authorID uuid.UUID, limit int, cursor string) ([]*entity.Content, string, error) {
	return nil, "", nil
}

func (m *mockContentRepository) GetMedia(ctx context.Context, tx interface{}, contentID uuid.UUID) ([]*entity.ContentMedia, error) {
	return nil, nil
}

// mockLikeRepository is a mock implementation of LikeRepository.
type mockLikeRepository struct {
	insertLikeFunc       func(ctx context.Context, tx interface{}, contentID, userID uuid.UUID) error
	deleteLikeFunc       func(ctx context.Context, tx interface{}, contentID, userID uuid.UUID) error
	existsLikeFunc       func(ctx context.Context, tx interface{}, contentID, userID uuid.UUID) (bool, error)
	countLikesFunc       func(ctx context.Context, tx interface{}, contentID uuid.UUID) (int, error)
	getLikeCreatedAtFunc func(ctx context.Context, tx interface{}, contentID, userID uuid.UUID) (time.Time, error)
}

var mockLikeCreatedAt = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

func (m *mockLikeRepository) InsertLike(ctx context.Context, tx interface{}, contentID, userID uuid.UUID) error {
	if m.insertLikeFunc != nil {
		return m.insertLikeFunc(ctx, tx, contentID, userID)
	}
	return nil
}

func (m *mockLikeRepository) DeleteLike(ctx context.Context, tx interface{}, contentID, userID uuid.UUID) error {
	if m.deleteLikeFunc != nil {
		return m.deleteLikeFunc(ctx, tx, contentID, userID)
	}
	return nil
}

func (m *mockLikeRepository) ExistsLike(ctx context.Context, tx interface{}, contentID, userID uuid.UUID) (bool, error) {
	if m.existsLikeFunc != nil {
		return m.existsLikeFunc(ctx, tx, contentID, userID)
	}
	return false, nil
}

func (m *mockLikeRepository) CountLikes(ctx context.Context, tx interface{}, contentID uuid.UUID) (int, error) {
	if m.countLikesFunc != nil {
		return m.countLikesFunc(ctx, tx, contentID)
	}
	return 0, nil
}

func (m *mockLikeRepository) GetLikeCreatedAt(ctx context.Context, tx interface{}, contentID, userID uuid.UUID) (time.Time, error) {
	if m.getLikeCreatedAtFunc != nil {
		return m.getLikeCreatedAtFunc(ctx, tx, contentID, userID)
	}
	return mockLikeCreatedAt, nil
}

// mockLikeNotificationScrubber is a mock implementation of LikeNotificationScrubber.
type mockLikeNotificationScrubber struct {
	deleteFunc func(ctx context.Context, tx interface{}, likerID, contentID uuid.UUID) error
	deleted    []uuid.UUID
}

func (m *mockLikeNotificationScrubber) DeleteContentLikedNotification(ctx context.Context, tx interface{}, likerID, contentID uuid.UUID) error {
	m.deleted = append(m.deleted, contentID)
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, tx, likerID, contentID)
	}
	return nil
}

// mockOutboxInserter is a mock implementation of OutboxInserter for testing.
type mockOutboxInserter struct {
	insertTxFunc func(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error
}

func (m *mockOutboxInserter) InsertTx(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error {
	if m.insertTxFunc != nil {
		return m.insertTxFunc(ctx, tx, eventType, payload, idempotencyKey)
	}
	return nil
}

// mockBlockChecker is a mock implementation of BlockChecker.
type mockBlockChecker struct {
	existsBlockFunc func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (bool, error)
}

func (m *mockBlockChecker) ExistsBlock(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (bool, error) {
	if m.existsBlockFunc != nil {
		return m.existsBlockFunc(ctx, tx, userA, userB)
	}
	return false, nil
}

// mockInvariantLogger is a mock implementation of InvariantLogger.
type mockInvariantLogger struct{}

func (m *mockInvariantLogger) LogViolation(ctx context.Context, violation application.InvariantViolation) {
	// No-op for tests
}

// TestLikeService_Like tests like operations.
func TestLikeService_Like(t *testing.T) {
	// Shared mock outbox for tests that don't verify outbox behavior
	mockOutbox := &mockOutboxInserter{}

	t.Run("likes content successfully", func(t *testing.T) {
		contentID := uuid.New()
		userID := uuid.New()
		authorID := uuid.New()
		var insertedLike bool

		contentRepo := &mockContentRepository{
			getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error) {
				return &entity.Content{
					ID:       contentID,
					AuthorID: authorID,
					Status:   entity.StatusActive,
				}, nil
			},
		}

		likeRepo := &mockLikeRepository{
			insertLikeFunc: func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) error {
				insertedLike = true
				return nil
			},
		}

		service := application.NewService(&mockTransactor{}, contentRepo, likeRepo, mockOutbox, &mockBlockChecker{}, &mockInvariantLogger{}, &mockLikeNotificationScrubber{})

		err := service.Like(context.Background(), contentID, userID)

		require.NoError(t, err)
		assert.True(t, insertedLike)
	})

	t.Run("cannot like deleted content", func(t *testing.T) {
		contentID := uuid.New()
		userID := uuid.New()

		contentRepo := &mockContentRepository{
			getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error) {
				return &entity.Content{
					ID:     contentID,
					Status: entity.StatusDeleted,
				}, nil
			},
		}

		likeRepo := &mockLikeRepository{}

		service := application.NewService(&mockTransactor{}, contentRepo, likeRepo, mockOutbox, &mockBlockChecker{}, &mockInvariantLogger{}, &mockLikeNotificationScrubber{})

		err := service.Like(context.Background(), contentID, userID)

		assert.Error(t, err)
		var deletedErr *likeentity.ErrContentDeleted
		assert.ErrorAs(t, err, &deletedErr)
	})

	t.Run("returns error when content not found", func(t *testing.T) {
		contentID := uuid.New()
		userID := uuid.New()

		contentRepo := &mockContentRepository{
			getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error) {
				return nil, errors.New("content not found: " + id.String())
			},
		}

		likeRepo := &mockLikeRepository{}

		service := application.NewService(&mockTransactor{}, contentRepo, likeRepo, mockOutbox, &mockBlockChecker{}, &mockInvariantLogger{}, &mockLikeNotificationScrubber{})

		err := service.Like(context.Background(), contentID, userID)

		assert.Error(t, err)
		var notFoundErr *likeentity.ErrContentNotFound
		assert.ErrorAs(t, err, &notFoundErr)
	})

	t.Run("duplicate like is idempotent", func(t *testing.T) {
		contentID := uuid.New()
		userID := uuid.New()
		authorID := uuid.New()
		insertCount := 0

		contentRepo := &mockContentRepository{
			getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error) {
				return &entity.Content{
					ID:       contentID,
					AuthorID: authorID,
					Status:   entity.StatusActive,
				}, nil
			},
		}

		likeRepo := &mockLikeRepository{
			insertLikeFunc: func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) error {
				insertCount++
				return nil // ON CONFLICT DO NOTHING makes this idempotent
			},
		}

		service := application.NewService(&mockTransactor{}, contentRepo, likeRepo, mockOutbox, &mockBlockChecker{}, &mockInvariantLogger{}, &mockLikeNotificationScrubber{})

		// Like twice
		err1 := service.Like(context.Background(), contentID, userID)
		err2 := service.Like(context.Background(), contentID, userID)

		assert.NoError(t, err1)
		assert.NoError(t, err2)
		assert.Equal(t, 2, insertCount, "Insert is called twice (ON CONFLICT handles dupes)")
	})
}

// TestLikeService_Unlike tests unlike operations.
func TestLikeService_Unlike(t *testing.T) {
	t.Run("unlikes content successfully", func(t *testing.T) {
		contentID := uuid.New()
		userID := uuid.New()
		var deletedLike bool

		likeRepo := &mockLikeRepository{
			deleteLikeFunc: func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) error {
				deletedLike = true
				return nil
			},
		}

		contentRepo := &mockContentRepository{}

		service := application.NewService(&mockTransactor{}, contentRepo, likeRepo, nil, &mockBlockChecker{}, &mockInvariantLogger{}, &mockLikeNotificationScrubber{})

		err := service.Unlike(context.Background(), contentID, userID)

		require.NoError(t, err)
		assert.True(t, deletedLike)
	})

	t.Run("unlike is idempotent", func(t *testing.T) {
		contentID := uuid.New()
		userID := uuid.New()
		deleteCount := 0

		likeRepo := &mockLikeRepository{
			deleteLikeFunc: func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) error {
				deleteCount++
				return nil // No error even if like doesn't exist
			},
		}

		contentRepo := &mockContentRepository{}

		service := application.NewService(&mockTransactor{}, contentRepo, likeRepo, nil, &mockBlockChecker{}, &mockInvariantLogger{}, &mockLikeNotificationScrubber{})

		// Unlike twice
		err1 := service.Unlike(context.Background(), contentID, userID)
		err2 := service.Unlike(context.Background(), contentID, userID)

		assert.NoError(t, err1)
		assert.NoError(t, err2)
		assert.Equal(t, 2, deleteCount)
	})
}

// TestLikeRepository_CountLikes tests counting likes.
func TestLikeRepository_CountLikes(t *testing.T) {
	t.Run("returns accurate count", func(t *testing.T) {
		contentID := uuid.New()
		expectedCount := 42

		likeRepo := &mockLikeRepository{
			countLikesFunc: func(ctx context.Context, tx interface{}, cid uuid.UUID) (int, error) {
				return expectedCount, nil
			},
		}

		count, err := likeRepo.CountLikes(context.Background(), &mockLikeTx{}, contentID)

		require.NoError(t, err)
		assert.Equal(t, expectedCount, count)
	})

	t.Run("returns zero for content with no likes", func(t *testing.T) {
		contentID := uuid.New()

		likeRepo := &mockLikeRepository{
			countLikesFunc: func(ctx context.Context, tx interface{}, cid uuid.UUID) (int, error) {
				return 0, nil
			},
		}

		count, err := likeRepo.CountLikes(context.Background(), &mockLikeTx{}, contentID)

		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

// TestLikeRepository_ExistsLike tests checking if like exists.
func TestLikeRepository_ExistsLike(t *testing.T) {
	t.Run("returns true when like exists", func(t *testing.T) {
		contentID := uuid.New()
		userID := uuid.New()

		likeRepo := &mockLikeRepository{
			existsLikeFunc: func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) (bool, error) {
				return true, nil
			},
		}

		exists, err := likeRepo.ExistsLike(context.Background(), &mockLikeTx{}, contentID, userID)

		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("returns false when like does not exist", func(t *testing.T) {
		contentID := uuid.New()
		userID := uuid.New()

		likeRepo := &mockLikeRepository{
			existsLikeFunc: func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) (bool, error) {
				return false, nil
			},
		}

		exists, err := likeRepo.ExistsLike(context.Background(), &mockLikeTx{}, contentID, userID)

		require.NoError(t, err)
		assert.False(t, exists)
	})
}

// TestLikeRepository_InsertLike tests inserting likes with ON CONFLICT.
func TestLikeRepository_InsertLike(t *testing.T) {
	t.Run("inserts like successfully", func(t *testing.T) {
		contentID := uuid.New()
		userID := uuid.New()
		var inserted bool

		likeRepo := &mockLikeRepository{
			insertLikeFunc: func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) error {
				inserted = true
				return nil
			},
		}

		err := likeRepo.InsertLike(context.Background(), &mockLikeTx{}, contentID, userID)

		require.NoError(t, err)
		assert.True(t, inserted)
	})

	t.Run("handles duplicate via ON CONFLICT DO NOTHING", func(t *testing.T) {
		contentID := uuid.New()
		userID := uuid.New()

		likeRepo := &mockLikeRepository{
			insertLikeFunc: func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) error {
				return nil // ON CONFLICT DO NOTHING prevents error
			},
		}

		// Insert twice
		err1 := likeRepo.InsertLike(context.Background(), &mockLikeTx{}, contentID, userID)
		err2 := likeRepo.InsertLike(context.Background(), &mockLikeTx{}, contentID, userID)

		assert.NoError(t, err1)
		assert.NoError(t, err2)
	})
}

// TestLikeRepository_DeleteLike tests deleting likes.
func TestLikeRepository_DeleteLike(t *testing.T) {
	t.Run("deletes like successfully", func(t *testing.T) {
		contentID := uuid.New()
		userID := uuid.New()
		var deleted bool

		likeRepo := &mockLikeRepository{
			deleteLikeFunc: func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) error {
				deleted = true
				return nil
			},
		}

		err := likeRepo.DeleteLike(context.Background(), &mockLikeTx{}, contentID, userID)

		require.NoError(t, err)
		assert.True(t, deleted)
	})

	t.Run("delete is idempotent", func(t *testing.T) {
		contentID := uuid.New()
		userID := uuid.New()

		likeRepo := &mockLikeRepository{
			deleteLikeFunc: func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) error {
				return nil // No error even if like doesn't exist
			},
		}

		// Delete twice
		err1 := likeRepo.DeleteLike(context.Background(), &mockLikeTx{}, contentID, userID)
		err2 := likeRepo.DeleteLike(context.Background(), &mockLikeTx{}, contentID, userID)

		assert.NoError(t, err1)
		assert.NoError(t, err2)
	})
}

// TestLikeService_ErrorHandling tests error handling in service.
func TestLikeService_ErrorHandling(t *testing.T) {
	t.Run("returns database error on repository failure", func(t *testing.T) {
		contentID := uuid.New()
		userID := uuid.New()

		contentRepo := &mockContentRepository{
			getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error) {
				return &entity.Content{
					ID:     contentID,
					Status: entity.StatusActive,
				}, nil
			},
		}

		likeRepo := &mockLikeRepository{
			insertLikeFunc: func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) error {
				return errors.New("database connection failed")
			},
		}

		mockOutbox := &mockOutboxInserter{}

		service := application.NewService(&mockTransactor{}, contentRepo, likeRepo, mockOutbox, &mockBlockChecker{}, &mockInvariantLogger{}, &mockLikeNotificationScrubber{})

		err := service.Like(context.Background(), contentID, userID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "insert like failed")
	})
}

// TestLikeService_IdempotencyKey tests that idempotency keys follow the correct format.
func TestLikeService_IdempotencyKey(t *testing.T) {
	t.Run("idempotency key format is content.liked.{contentID}.{userID}.{likeCreatedAt}", func(t *testing.T) {
		contentID := uuid.New()
		userID := uuid.New()
		authorID := uuid.New()
		var capturedIdempotencyKey string
		var capturedEventType string

		contentRepo := &mockContentRepository{
			getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error) {
				return &entity.Content{
					ID:       contentID,
					AuthorID: authorID,
					Status:   entity.StatusActive,
				}, nil
			},
		}

		likeRepo := &mockLikeRepository{}

		outboxRepo := &mockOutboxInserter{
			insertTxFunc: func(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error {
				capturedEventType = eventType
				capturedIdempotencyKey = idempotencyKey
				return nil
			},
		}

		service := application.NewService(&mockTransactor{}, contentRepo, likeRepo, outboxRepo, nil, nil, nil)

		err := service.Like(context.Background(), contentID, userID)

		require.NoError(t, err)
		assert.Equal(t, events.EventContentLiked, capturedEventType)
		// The occurrence-scoped key: events after UNLIKE carry a fresh created_at,
		// so a re-like is never suppressed by the prior (retained) outbox key.
		assert.Equal(t, "content.liked."+contentID.String()+"."+userID.String()+"."+strconv.FormatInt(mockLikeCreatedAt.UnixNano(), 10), capturedIdempotencyKey)
	})

	t.Run("LIKE after UNLIKE produces a NEW occurrence key (not the prior one)", func(t *testing.T) {
		contentID := uuid.New()
		userID := uuid.New()
		authorID := uuid.New()
		var capturedKeys []string

		contentRepo := &mockContentRepository{
			getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error) {
				return &entity.Content{
					ID:       contentID,
					AuthorID: authorID,
					Status:   entity.StatusActive,
				}, nil
			},
		}

		firstLike := mockLikeCreatedAt
		secondLike := mockLikeCreatedAt.Add(time.Nanosecond)
		createdAtCalls := 0
		likeRepo := &mockLikeRepository{
			getLikeCreatedAtFunc: func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) (time.Time, error) {
				createdAtCalls++
				if createdAtCalls == 1 {
					return firstLike, nil
				}
				return secondLike, nil
			},
		}

		outboxRepo := &mockOutboxInserter{
			insertTxFunc: func(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error {
				capturedKeys = append(capturedKeys, idempotencyKey)
				return nil
			},
		}

		scrubber := &mockLikeNotificationScrubber{}
		service := application.NewService(&mockTransactor{}, contentRepo, likeRepo, outboxRepo, nil, nil, scrubber)

		// LIKE
		r1, err := service.ToggleContentLike(context.Background(), contentID, userID)
		require.NoError(t, err)
		assert.True(t, r1.Liked)

		// Unlike path: ExistsLike returns true, DeleteLike removes, scrubber fires.
		likeRepo.existsLikeFunc = func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) (bool, error) {
			return true, nil
		}
		r2, err := service.ToggleContentLike(context.Background(), contentID, userID)
		require.NoError(t, err)
		assert.False(t, r2.Liked)
		require.Len(t, scrubber.deleted, 1, "unlike must scrub the content.liked notification")

		// LIKE AGAIN (new occurrence, new created_at => new key)
		likeRepo.existsLikeFunc = func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) (bool, error) {
			return false, nil
		}
		r3, err := service.ToggleContentLike(context.Background(), contentID, userID)
		require.NoError(t, err)
		assert.True(t, r3.Liked)

		require.Len(t, capturedKeys, 2, "each LIKE occurrence must emit exactly one outbox event")
		assert.NotEqual(t, capturedKeys[0], capturedKeys[1], "re-like after unlike must use a different idempotency key")
		assert.Equal(t, "content.liked."+contentID.String()+"."+userID.String()+"."+strconv.FormatInt(secondLike.UnixNano(), 10), capturedKeys[1])
	})
}

// =============================================================================
// ToggleContentLike tests
// =============================================================================

func TestToggleContentLike_ActiveContentCanBeLiked(t *testing.T) {
	contentID := uuid.New()
	userID := uuid.New()
	authorID := uuid.New()

	contentRepo := &mockContentRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error) {
			return &entity.Content{
				ID:       contentID,
				AuthorID: authorID,
				Status:   entity.StatusActive,
			}, nil
		},
	}

	likeCount := 0
	likeRepo := &mockLikeRepository{
		existsLikeFunc: func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) (bool, error) {
			return false, nil
		},
		insertLikeFunc: func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) error {
			likeCount++
			return nil
		},
		countLikesFunc: func(ctx context.Context, tx interface{}, cid uuid.UUID) (int, error) {
			return likeCount, nil
		},
	}

	service := application.NewService(
		&mockTransactor{}, contentRepo, likeRepo,
		&mockOutboxInserter{}, &mockBlockChecker{}, &mockInvariantLogger{},
		&mockLikeNotificationScrubber{},
	)

	result, err := service.ToggleContentLike(context.Background(), contentID, userID)

	require.NoError(t, err)
	assert.True(t, result.Liked)
	assert.Equal(t, 1, result.Count)
}

func TestToggleContentLike_UnlikeWhenAlreadyLiked(t *testing.T) {
	contentID := uuid.New()
	userID := uuid.New()

	likeRepo := &mockLikeRepository{
		existsLikeFunc: func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) (bool, error) {
			return true, nil // already liked
		},
		deleteLikeFunc: func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) error {
			return nil
		},
		countLikesFunc: func(ctx context.Context, tx interface{}, cid uuid.UUID) (int, error) {
			return 0, nil
		},
	}

	service := application.NewService(
		&mockTransactor{}, &mockContentRepository{}, likeRepo,
		&mockOutboxInserter{}, &mockBlockChecker{}, &mockInvariantLogger{},
		&mockLikeNotificationScrubber{},
	)

	result, err := service.ToggleContentLike(context.Background(), contentID, userID)

	require.NoError(t, err)
	assert.False(t, result.Liked)
	assert.Equal(t, 0, result.Count)
}

func TestToggleContentLike_BlockedUserCannotLike(t *testing.T) {
	contentID := uuid.New()
	userID := uuid.New()
	authorID := uuid.New()

	contentRepo := &mockContentRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error) {
			return &entity.Content{
				ID:       contentID,
				AuthorID: authorID,
				Status:   entity.StatusActive,
			}, nil
		},
	}

	likeRepo := &mockLikeRepository{
		existsLikeFunc: func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) (bool, error) {
			return false, nil
		},
	}

	blocker := &mockBlockChecker{
		existsBlockFunc: func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (bool, error) {
			return true, nil // blocked
		},
	}

	service := application.NewService(
		&mockTransactor{}, contentRepo, likeRepo,
		&mockOutboxInserter{}, blocker, &mockInvariantLogger{},
		&mockLikeNotificationScrubber{},
	)

	_, err := service.ToggleContentLike(context.Background(), contentID, userID)

	require.Error(t, err)
	var notFoundErr *likeentity.ErrContentNotFound
	assert.ErrorAs(t, err, &notFoundErr, "block should surface as content-not-found to avoid leaking block info")
}

func TestToggleContentLike_DeletedContentCannotBeLiked(t *testing.T) {
	contentID := uuid.New()
	userID := uuid.New()

	contentRepo := &mockContentRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error) {
			return &entity.Content{
				ID:     contentID,
				Status: entity.StatusDeleted,
			}, nil
		},
	}

	likeRepo := &mockLikeRepository{
		existsLikeFunc: func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) (bool, error) {
			return false, nil
		},
	}

	service := application.NewService(
		&mockTransactor{}, contentRepo, likeRepo,
		&mockOutboxInserter{}, &mockBlockChecker{}, &mockInvariantLogger{},
		&mockLikeNotificationScrubber{},
	)

	_, err := service.ToggleContentLike(context.Background(), contentID, userID)

	require.Error(t, err)
	var deletedErr *likeentity.ErrContentDeleted
	assert.ErrorAs(t, err, &deletedErr)
}

func TestToggleContentLike_HiddenContentCannotBeLiked(t *testing.T) {
	contentID := uuid.New()
	userID := uuid.New()

	contentRepo := &mockContentRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error) {
			return &entity.Content{
				ID:       contentID,
				Status:   entity.StatusActive,
				IsHidden: true,
			}, nil
		},
	}

	likeRepo := &mockLikeRepository{
		existsLikeFunc: func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) (bool, error) {
			return false, nil
		},
	}

	service := application.NewService(
		&mockTransactor{}, contentRepo, likeRepo,
		&mockOutboxInserter{}, &mockBlockChecker{}, &mockInvariantLogger{},
		&mockLikeNotificationScrubber{},
	)

	_, err := service.ToggleContentLike(context.Background(), contentID, userID)

	require.Error(t, err)
	var notFoundErr *likeentity.ErrContentNotFound
	assert.ErrorAs(t, err, &notFoundErr, "hidden (moderated/private) content must not be likeable, surfaced as not-found")
}

func TestToggleContentLike_UnlikeScrubsLikeNotification(t *testing.T) {
	contentID := uuid.New()
	userID := uuid.New()

	likeRepo := &mockLikeRepository{
		existsLikeFunc: func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) (bool, error) {
			return true, nil // already liked → unlike path
		},
		deleteLikeFunc: func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) error {
			return nil
		},
		countLikesFunc: func(ctx context.Context, tx interface{}, cid uuid.UUID) (int, error) {
			return 0, nil
		},
	}

	scrubber := &mockLikeNotificationScrubber{}
	service := application.NewService(
		&mockTransactor{}, &mockContentRepository{}, likeRepo,
		&mockOutboxInserter{}, &mockBlockChecker{}, &mockInvariantLogger{},
		scrubber,
	)

	result, err := service.ToggleContentLike(context.Background(), contentID, userID)

	require.NoError(t, err)
	assert.False(t, result.Liked)
	require.Len(t, scrubber.deleted, 1, "unlike must scrub the stale content.liked notification so a re-like can notify")
	assert.Equal(t, contentID, scrubber.deleted[0])
}

func TestLikeService_IsContentLikeReadable(t *testing.T) {
	contentID := uuid.New()
	authorID := uuid.New()
	viewerID := uuid.New()

	activeContent := func() *entity.Content {
		return &entity.Content{ID: contentID, AuthorID: authorID, Status: entity.StatusActive}
	}

	t.Run("active content is readable", func(t *testing.T) {
		service := application.NewService(
			&mockTransactor{}, &mockContentRepository{
				getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error) {
					return activeContent(), nil
				},
			}, &mockLikeRepository{}, nil, &mockBlockChecker{}, &mockInvariantLogger{},
			&mockLikeNotificationScrubber{},
		)
		ok, err := service.IsContentLikeReadable(context.Background(), &mockLikeTx{}, contentID, viewerID)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("deleted content is NOT readable", func(t *testing.T) {
		service := application.NewService(
			&mockTransactor{}, &mockContentRepository{
				getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error) {
					return &entity.Content{ID: contentID, AuthorID: authorID, Status: entity.StatusDeleted}, nil
				},
			}, &mockLikeRepository{}, nil, &mockBlockChecker{}, &mockInvariantLogger{},
			&mockLikeNotificationScrubber{},
		)
		ok, err := service.IsContentLikeReadable(context.Background(), &mockLikeTx{}, contentID, viewerID)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("hidden content is NOT readable", func(t *testing.T) {
		service := application.NewService(
			&mockTransactor{}, &mockContentRepository{
				getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error) {
					return &entity.Content{ID: contentID, AuthorID: authorID, Status: entity.StatusActive, IsHidden: true}, nil
				},
			}, &mockLikeRepository{}, nil, &mockBlockChecker{}, &mockInvariantLogger{},
			&mockLikeNotificationScrubber{},
		)
		ok, err := service.IsContentLikeReadable(context.Background(), &mockLikeTx{}, contentID, viewerID)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("blocked relationship is NOT readable", func(t *testing.T) {
		service := application.NewService(
			&mockTransactor{}, &mockContentRepository{
				getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error) {
					return activeContent(), nil
				},
			}, &mockLikeRepository{}, nil, &mockBlockChecker{
				existsBlockFunc: func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (bool, error) {
					return true, nil
				},
			}, &mockInvariantLogger{},
			&mockLikeNotificationScrubber{},
		)
		ok, err := service.IsContentLikeReadable(context.Background(), &mockLikeTx{}, contentID, viewerID)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("content author is always readable", func(t *testing.T) {
		service := application.NewService(
			&mockTransactor{}, &mockContentRepository{
				getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error) {
					return activeContent(), nil
				},
			}, &mockLikeRepository{}, nil, &mockBlockChecker{
				existsBlockFunc: func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (bool, error) {
					return true, nil
				},
			}, &mockInvariantLogger{},
			&mockLikeNotificationScrubber{},
		)
		ok, err := service.IsContentLikeReadable(context.Background(), &mockLikeTx{}, contentID, authorID)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("anonymous viewer on active content is readable", func(t *testing.T) {
		service := application.NewService(
			&mockTransactor{}, &mockContentRepository{
				getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error) {
					return activeContent(), nil
				},
			}, &mockLikeRepository{}, nil, &mockBlockChecker{}, &mockInvariantLogger{},
			&mockLikeNotificationScrubber{},
		)
		ok, err := service.IsContentLikeReadable(context.Background(), &mockLikeTx{}, contentID, uuid.Nil)
		require.NoError(t, err)
		assert.True(t, ok)
	})
}

func TestLikeService_Like_HiddenContentRejected(t *testing.T) {
	contentID := uuid.New()
	userID := uuid.New()

	contentRepo := &mockContentRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error) {
			return &entity.Content{
				ID:       contentID,
				Status:   entity.StatusActive,
				IsHidden: true,
			}, nil
		},
	}

	service := application.NewService(
		&mockTransactor{}, contentRepo, &mockLikeRepository{},
		&mockOutboxInserter{}, &mockBlockChecker{}, &mockInvariantLogger{},
		&mockLikeNotificationScrubber{},
	)

	err := service.Like(context.Background(), contentID, userID)

	require.Error(t, err)
	var notFoundErr *likeentity.ErrContentNotFound
	assert.ErrorAs(t, err, &notFoundErr)
}

func TestToggleContentLike_OutboxEmittedOnLike(t *testing.T) {
	contentID := uuid.New()
	userID := uuid.New()
	authorID := uuid.New()
	var capturedEventType string
	var capturedKey string

	contentRepo := &mockContentRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error) {
			return &entity.Content{
				ID:       contentID,
				AuthorID: authorID,
				Status:   entity.StatusActive,
			}, nil
		},
	}

	likeRepo := &mockLikeRepository{
		existsLikeFunc: func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) (bool, error) {
			return false, nil
		},
		countLikesFunc: func(ctx context.Context, tx interface{}, cid uuid.UUID) (int, error) {
			return 1, nil
		},
	}

	outbox := &mockOutboxInserter{
		insertTxFunc: func(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error {
			capturedEventType = eventType
			capturedKey = idempotencyKey
			return nil
		},
	}

	service := application.NewService(
		&mockTransactor{}, contentRepo, likeRepo,
		outbox, &mockBlockChecker{}, &mockInvariantLogger{},
		&mockLikeNotificationScrubber{},
	)

	result, err := service.ToggleContentLike(context.Background(), contentID, userID)

	require.NoError(t, err)
	assert.True(t, result.Liked)
	assert.Equal(t, events.EventContentLiked, capturedEventType)
	assert.Equal(t, "content.liked."+contentID.String()+"."+userID.String()+"."+strconv.FormatInt(mockLikeCreatedAt.UnixNano(), 10), capturedKey)
}

func TestToggleContentLike_NoOutboxOnUnlike(t *testing.T) {
	contentID := uuid.New()
	userID := uuid.New()
	outboxCalled := false

	likeRepo := &mockLikeRepository{
		existsLikeFunc: func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) (bool, error) {
			return true, nil // already liked → unlike path
		},
		deleteLikeFunc: func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) error {
			return nil
		},
		countLikesFunc: func(ctx context.Context, tx interface{}, cid uuid.UUID) (int, error) {
			return 0, nil
		},
	}

	outbox := &mockOutboxInserter{
		insertTxFunc: func(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error {
			outboxCalled = true
			return nil
		},
	}

	service := application.NewService(
		&mockTransactor{}, &mockContentRepository{}, likeRepo,
		outbox, &mockBlockChecker{}, &mockInvariantLogger{},
		&mockLikeNotificationScrubber{},
	)

	result, err := service.ToggleContentLike(context.Background(), contentID, userID)

	require.NoError(t, err)
	assert.False(t, result.Liked)
	assert.False(t, outboxCalled, "outbox must NOT be called on unlike")
}

func TestToggleContentLike_UnlikeSkipsValidation(t *testing.T) {
	// A user who previously liked content should be able to unlike even if
	// the block checker would now deny a like. The unlike path intentionally
	// skips content existence and block checks.
	contentID := uuid.New()
	userID := uuid.New()

	likeRepo := &mockLikeRepository{
		existsLikeFunc: func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) (bool, error) {
			return true, nil // already liked
		},
		deleteLikeFunc: func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) error {
			return nil
		},
		countLikesFunc: func(ctx context.Context, tx interface{}, cid uuid.UUID) (int, error) {
			return 0, nil
		},
	}

	blocker := &mockBlockChecker{
		existsBlockFunc: func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (bool, error) {
			return true, nil // blocked — should not matter for unlike
		},
	}

	service := application.NewService(
		&mockTransactor{}, &mockContentRepository{}, likeRepo,
		&mockOutboxInserter{}, blocker, &mockInvariantLogger{},
		&mockLikeNotificationScrubber{},
	)

	result, err := service.ToggleContentLike(context.Background(), contentID, userID)

	require.NoError(t, err)
	assert.False(t, result.Liked)
}

func TestToggleContentLike_DuplicateLikeIdempotent(t *testing.T) {
	contentID := uuid.New()
	userID := uuid.New()
	authorID := uuid.New()
	insertCount := 0

	contentRepo := &mockContentRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error) {
			return &entity.Content{
				ID:       contentID,
				AuthorID: authorID,
				Status:   entity.StatusActive,
			}, nil
		},
	}

	likeRepo := &mockLikeRepository{
		existsLikeFunc: func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) (bool, error) {
			return false, nil // always "not liked" to force insert path
		},
		insertLikeFunc: func(ctx context.Context, tx interface{}, cid, uid uuid.UUID) error {
			insertCount++
			return nil // ON CONFLICT DO NOTHING
		},
		countLikesFunc: func(ctx context.Context, tx interface{}, cid uuid.UUID) (int, error) {
			return 1, nil
		},
	}

	service := application.NewService(
		&mockTransactor{}, contentRepo, likeRepo,
		&mockOutboxInserter{}, &mockBlockChecker{}, &mockInvariantLogger{},
		&mockLikeNotificationScrubber{},
	)

	r1, err1 := service.ToggleContentLike(context.Background(), contentID, userID)
	r2, err2 := service.ToggleContentLike(context.Background(), contentID, userID)

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.True(t, r1.Liked)
	assert.True(t, r2.Liked)
	assert.Equal(t, 2, insertCount, "insert called twice; ON CONFLICT handles dedup at DB level")
}
