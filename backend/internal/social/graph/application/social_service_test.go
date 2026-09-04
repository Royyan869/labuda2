package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/platform/events"
	"github.com/labuda/backend/internal/social/graph/application"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSocialTransactor is a mock implementation of Transactor for testing.
type mockSocialTransactor struct {
	executeFn func(ctx context.Context, fn func(tx db.Tx) error) error
}

func (m *mockSocialTransactor) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	if m.executeFn != nil {
		return m.executeFn(ctx, fn)
	}
	return fn(&mockSocialTxImpl{})
}

// mockSocialTx is a mock implementation of db.Tx for testing.
type mockSocialTxImpl struct{}

func (m *mockSocialTxImpl) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (m *mockSocialTxImpl) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return &mockSocialRows{}, nil
}

func (m *mockSocialTxImpl) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return &mockSocialRow{}
}

func (m *mockSocialTxImpl) Commit(ctx context.Context) error {
	return nil
}

func (m *mockSocialTxImpl) Rollback(ctx context.Context) error {
	return nil
}

// mockSocialRows is a mock implementation of pgx.Rows.
type mockSocialRows struct{}

func (m *mockSocialRows) Close() {
}

func (m *mockSocialRows) Err() error {
	return nil
}

func (m *mockSocialRows) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}

func (m *mockSocialRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (m *mockSocialRows) Next() bool {
	return false
}

func (m *mockSocialRows) Scan(dest ...any) error {
	return nil
}

func (m *mockSocialRows) Values() ([]any, error) {
	return nil, nil
}

func (m *mockSocialRows) RawValues() [][]byte {
	return nil
}

func (m *mockSocialRows) Conn() *pgx.Conn {
	return nil
}

// mockSocialRow is a mock implementation of pgx.Row.
type mockSocialRow struct {
	scanFunc func(dest ...any) error
}

func (m *mockSocialRow) Scan(dest ...any) error {
	if m.scanFunc != nil {
		return m.scanFunc(dest...)
	}
	return nil
}

// mockSocialRepository is a mock implementation of SocialRepository.
type mockSocialRepository struct {
	insertFollowFunc                func(ctx context.Context, tx interface{}, followerID, followingID uuid.UUID) error
	deleteFollowFunc                func(ctx context.Context, tx interface{}, followerID, followingID uuid.UUID) error
	deleteFollowBothDirectionsFunc  func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) error
	existsFollowFunc                func(ctx context.Context, tx interface{}, followerID, followingID uuid.UUID) (bool, error)
	listFollowersFunc               func(ctx context.Context, tx interface{}, userID uuid.UUID, limit int, cursor *time.Time) ([]uuid.UUID, error)
	listFollowingFunc               func(ctx context.Context, tx interface{}, userID uuid.UUID, limit int, cursor *time.Time) ([]uuid.UUID, error)
	insertBlockFunc                 func(ctx context.Context, tx interface{}, blockerID, blockedID uuid.UUID) error
	deleteBlockFunc                 func(ctx context.Context, tx interface{}, blockerID, blockedID uuid.UUID) error
	existsBlockFunc                 func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (bool, error)
	acquireFollowLockFunc           func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) error
}

func (m *mockSocialRepository) InsertFollow(ctx context.Context, tx interface{}, followerID, followingID uuid.UUID) error {
	if m.insertFollowFunc != nil {
		return m.insertFollowFunc(ctx, tx, followerID, followingID)
	}
	return nil
}

func (m *mockSocialRepository) DeleteFollow(ctx context.Context, tx interface{}, followerID, followingID uuid.UUID) error {
	if m.deleteFollowFunc != nil {
		return m.deleteFollowFunc(ctx, tx, followerID, followingID)
	}
	return nil
}

func (m *mockSocialRepository) DeleteFollowBothDirections(ctx context.Context, tx interface{}, userA, userB uuid.UUID) error {
	if m.deleteFollowBothDirectionsFunc != nil {
		return m.deleteFollowBothDirectionsFunc(ctx, tx, userA, userB)
	}
	return nil
}

func (m *mockSocialRepository) ExistsFollow(ctx context.Context, tx interface{}, followerID, followingID uuid.UUID) (bool, error) {
	if m.existsFollowFunc != nil {
		return m.existsFollowFunc(ctx, tx, followerID, followingID)
	}
	return false, nil
}

func (m *mockSocialRepository) ListFollowers(ctx context.Context, tx interface{}, userID uuid.UUID, limit int, cursor *time.Time) ([]uuid.UUID, error) {
	if m.listFollowersFunc != nil {
		return m.listFollowersFunc(ctx, tx, userID, limit, cursor)
	}
	return nil, nil
}

func (m *mockSocialRepository) ListFollowing(ctx context.Context, tx interface{}, userID uuid.UUID, limit int, cursor *time.Time) ([]uuid.UUID, error) {
	if m.listFollowingFunc != nil {
		return m.listFollowingFunc(ctx, tx, userID, limit, cursor)
	}
	return nil, nil
}

func (m *mockSocialRepository) InsertBlock(ctx context.Context, tx interface{}, blockerID, blockedID uuid.UUID) error {
	if m.insertBlockFunc != nil {
		return m.insertBlockFunc(ctx, tx, blockerID, blockedID)
	}
	return nil
}

func (m *mockSocialRepository) DeleteMute(ctx context.Context, tx interface{}, userID, targetID uuid.UUID) error {	return nil}
func (m *mockSocialRepository) DeleteBlock(ctx context.Context, tx interface{}, blockerID, blockedID uuid.UUID) error {
	if m.deleteBlockFunc != nil {
		return m.deleteBlockFunc(ctx, tx, blockerID, blockedID)
	}
	return nil
}

func (m *mockSocialRepository) ExistsBlock(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (bool, error) {
	if m.existsBlockFunc != nil {
		return m.existsBlockFunc(ctx, tx, userA, userB)
	}
	return false, nil
}

func (m *mockSocialRepository) AcquireFollowLock(ctx context.Context, tx interface{}, userA, userB uuid.UUID) error {
	if m.acquireFollowLockFunc != nil {
		return m.acquireFollowLockFunc(ctx, tx, userA, userB)
	}
	return nil
}

func (m *mockSocialRepository) InsertMute(ctx context.Context, tx interface{}, muterID, mutedID uuid.UUID) error {
	return nil
}

func (m *mockSocialRepository) IsBlockedBy(ctx context.Context, tx interface{}, blockerID, targetID uuid.UUID) (bool, error) {
	return false, nil
}

func (m *mockSocialRepository) ListMuted(ctx context.Context, tx interface{}, userID uuid.UUID, limit int, cursor *time.Time) ([]uuid.UUID, error) {
	return []uuid.UUID{}, nil
}

func (m *mockSocialRepository) ListBlocked(ctx context.Context, tx interface{}, userID uuid.UUID, limit int, cursor *time.Time) ([]uuid.UUID, error) {
	return []uuid.UUID{}, nil
}

func (m *mockSocialRepository) ExistsMute(ctx context.Context, tx interface{}, muterID, mutedID uuid.UUID) (bool, error) {
	return false, nil
}

// TestSocialService_Follow tests follow operations.
func TestSocialService_Follow(t *testing.T) {
	// Shared mock outbox for tests that don't verify outbox behavior
	mockOutbox := &mockSocialOutboxInserter{}

	t.Run("creates follow relationship successfully", func(t *testing.T) {
		followerID := uuid.New()
		followingID := uuid.New()
		var insertedFollow bool

		repo := &mockSocialRepository{
			acquireFollowLockFunc: func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) error {
				return nil
			},
			existsBlockFunc: func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (bool, error) {
				return false, nil
			},
			insertFollowFunc: func(ctx context.Context, tx interface{}, fid, tid uuid.UUID) error {
				insertedFollow = true
				return nil
			},
		}
		transactor := &mockSocialTransactor{}

		service := application.NewSocialService(transactor, repo, mockOutbox)

		err := service.Follow(context.Background(), followerID, followingID)

		require.NoError(t, err)
		assert.True(t, insertedFollow)
	})

	t.Run("rejects self follow", func(t *testing.T) {
		userID := uuid.New()

		repo := &mockSocialRepository{}
		transactor := &mockSocialTransactor{}

		service := application.NewSocialService(transactor, repo, mockOutbox)

		err := service.Follow(context.Background(), userID, userID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot follow self")
	})

	t.Run("rejects follow when blocked", func(t *testing.T) {
		followerID := uuid.New()
		followingID := uuid.New()

		repo := &mockSocialRepository{
			acquireFollowLockFunc: func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) error {
				return nil
			},
			existsBlockFunc: func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (bool, error) {
				return true, nil
			},
		}
		transactor := &mockSocialTransactor{}

		service := application.NewSocialService(transactor, repo, mockOutbox)

		err := service.Follow(context.Background(), followerID, followingID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot follow: block exists")
	})
}

// TestSocialService_Unfollow tests unfollow operations.
func TestSocialService_Unfollow(t *testing.T) {
	mockOutbox := &mockSocialOutboxInserter{}

	t.Run("deletes follow relationship successfully", func(t *testing.T) {
		followerID := uuid.New()
		followingID := uuid.New()
		var deletedFollow bool

		repo := &mockSocialRepository{
			deleteFollowFunc: func(ctx context.Context, tx interface{}, fid, tid uuid.UUID) error {
				deletedFollow = true
				return nil
			},
		}
		transactor := &mockSocialTransactor{}

		service := application.NewSocialService(transactor, repo, mockOutbox)

		err := service.Unfollow(context.Background(), followerID, followingID)

		require.NoError(t, err)
		assert.True(t, deletedFollow)
	})

	t.Run("unfollow is idempotent", func(t *testing.T) {
		followerID := uuid.New()
		followingID := uuid.New()

		repo := &mockSocialRepository{
			deleteFollowFunc: func(ctx context.Context, tx interface{}, fid, tid uuid.UUID) error {
				return nil // No error even if follow doesn't exist
			},
		}
		transactor := &mockSocialTransactor{}

		service := application.NewSocialService(transactor, repo, mockOutbox)

		// Unfollow twice - should not error
		err1 := service.Unfollow(context.Background(), followerID, followingID)
		err2 := service.Unfollow(context.Background(), followerID, followingID)

		assert.NoError(t, err1)
		assert.NoError(t, err2)
	})
}

// TestSocialService_Block tests block operations.
func TestSocialService_Block(t *testing.T) {
	mockOutbox := &mockSocialOutboxInserter{}

	t.Run("creates block and removes follow relationships", func(t *testing.T) {
		blockerID := uuid.New()
		blockedID := uuid.New()
		var insertedBlock bool
		var deletedFollows bool

		repo := &mockSocialRepository{
			insertBlockFunc: func(ctx context.Context, tx interface{}, bid, tid uuid.UUID) error {
				insertedBlock = true
				return nil
			},
			deleteFollowBothDirectionsFunc: func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) error {
				deletedFollows = true
				return nil
			},
		}
		transactor := &mockSocialTransactor{}

		service := application.NewSocialService(transactor, repo, mockOutbox)

		err := service.Block(context.Background(), blockerID, blockedID)

		require.NoError(t, err)
		assert.True(t, insertedBlock)
		assert.True(t, deletedFollows)
	})

	t.Run("rejects self block", func(t *testing.T) {
		userID := uuid.New()

		repo := &mockSocialRepository{}
		transactor := &mockSocialTransactor{}

		service := application.NewSocialService(transactor, repo, mockOutbox)

		err := service.Block(context.Background(), userID, userID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot block self")
	})

	t.Run("block is idempotent - duplicate block safe", func(t *testing.T) {
		blockerID := uuid.New()
		blockedID := uuid.New()

		repo := &mockSocialRepository{
			insertBlockFunc: func(ctx context.Context, tx interface{}, bid, tid uuid.UUID) error {
				return nil // ON CONFLICT DO NOTHING
			},
			deleteFollowBothDirectionsFunc: func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) error {
				return nil
			},
		}
		transactor := &mockSocialTransactor{}

		service := application.NewSocialService(transactor, repo, mockOutbox)

		// Block twice - should not error
		err1 := service.Block(context.Background(), blockerID, blockedID)
		err2 := service.Block(context.Background(), blockerID, blockedID)

		assert.NoError(t, err1)
		assert.NoError(t, err2)
	})

	t.Run("block removes existing follow", func(t *testing.T) {
		blockerID := uuid.New()
		blockedID := uuid.New()
		var followCleanedUp bool

		repo := &mockSocialRepository{
			insertBlockFunc: func(ctx context.Context, tx interface{}, bid, tid uuid.UUID) error {
				return nil
			},
			deleteFollowBothDirectionsFunc: func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) error {
				followCleanedUp = true
				return nil
			},
		}
		transactor := &mockSocialTransactor{}

		service := application.NewSocialService(transactor, repo, mockOutbox)

		err := service.Block(context.Background(), blockerID, blockedID)

		require.NoError(t, err)
		assert.True(t, followCleanedUp)
	})
}

// TestSocialService_Unblock tests unblock operations.
func TestSocialService_Unblock(t *testing.T) {
	mockOutbox := &mockSocialOutboxInserter{}

	t.Run("deletes block relationship successfully", func(t *testing.T) {
		blockerID := uuid.New()
		blockedID := uuid.New()
		var deletedBlock bool

		repo := &mockSocialRepository{
			deleteBlockFunc: func(ctx context.Context, tx interface{}, bid, tid uuid.UUID) error {
				deletedBlock = true
				return nil
			},
		}
		transactor := &mockSocialTransactor{}

		service := application.NewSocialService(transactor, repo, mockOutbox)

		err := service.Unblock(context.Background(), blockerID, blockedID)

		require.NoError(t, err)
		assert.True(t, deletedBlock)
	})

	t.Run("unblock is idempotent", func(t *testing.T) {
		blockerID := uuid.New()
		blockedID := uuid.New()

		repo := &mockSocialRepository{
			deleteBlockFunc: func(ctx context.Context, tx interface{}, bid, tid uuid.UUID) error {
				return nil // No error even if block doesn't exist
			},
		}
		transactor := &mockSocialTransactor{}

		service := application.NewSocialService(transactor, repo, mockOutbox)

		// Unblock twice - should not error
		err1 := service.Unblock(context.Background(), blockerID, blockedID)
		err2 := service.Unblock(context.Background(), blockerID, blockedID)

		assert.NoError(t, err1)
		assert.NoError(t, err2)
	})

	t.Run("unblock does not restore follow", func(t *testing.T) {
		// This is a critical test - unblock should NOT restore the follow
		// The follow must be explicitly re-created
		blockerID := uuid.New()
		blockedID := uuid.New()
		followRestored := false

		repo := &mockSocialRepository{
			deleteBlockFunc: func(ctx context.Context, tx interface{}, bid, tid uuid.UUID) error {
				return nil
			},
			insertFollowFunc: func(ctx context.Context, tx interface{}, fid, tid uuid.UUID) error {
				followRestored = true
				return nil
			},
		}
		transactor := &mockSocialTransactor{}

		service := application.NewSocialService(transactor, repo, mockOutbox)

		err := service.Unblock(context.Background(), blockerID, blockedID)

		require.NoError(t, err)
		assert.False(t, followRestored, "Unblock should not restore follow")
	})
}

// TestSocialService_IsFollowing tests checking follow status.
func TestSocialService_IsFollowing(t *testing.T) {
	mockOutbox := &mockSocialOutboxInserter{}

	t.Run("returns true when following", func(t *testing.T) {
		followerID := uuid.New()
		followingID := uuid.New()

		repo := &mockSocialRepository{
			existsFollowFunc: func(ctx context.Context, tx interface{}, fid, tid uuid.UUID) (bool, error) {
				return true, nil
			},
		}
		transactor := &mockSocialTransactor{}

		service := application.NewSocialService(transactor, repo, mockOutbox)

		following, err := service.IsFollowing(context.Background(), followerID, followingID)

		require.NoError(t, err)
		assert.True(t, following)
	})

	t.Run("returns false when not following", func(t *testing.T) {
		followerID := uuid.New()
		followingID := uuid.New()

		repo := &mockSocialRepository{
			existsFollowFunc: func(ctx context.Context, tx interface{}, fid, tid uuid.UUID) (bool, error) {
				return false, nil
			},
		}
		transactor := &mockSocialTransactor{}

		service := application.NewSocialService(transactor, repo, mockOutbox)

		following, err := service.IsFollowing(context.Background(), followerID, followingID)

		require.NoError(t, err)
		assert.False(t, following)
	})
}

// TestSocialService_IsBlocked tests checking block status.
func TestSocialService_IsBlocked(t *testing.T) {
	mockOutbox := &mockSocialOutboxInserter{}

	t.Run("returns true when blocked", func(t *testing.T) {
		userA := uuid.New()
		userB := uuid.New()

		repo := &mockSocialRepository{
			existsBlockFunc: func(ctx context.Context, tx interface{}, a, b uuid.UUID) (bool, error) {
				return true, nil
			},
		}
		transactor := &mockSocialTransactor{}

		service := application.NewSocialService(transactor, repo, mockOutbox)

		blocked, err := service.IsBlocked(context.Background(), userA, userB)

		require.NoError(t, err)
		assert.True(t, blocked)
	})

	t.Run("returns false when not blocked", func(t *testing.T) {
		userA := uuid.New()
		userB := uuid.New()

		repo := &mockSocialRepository{
			existsBlockFunc: func(ctx context.Context, tx interface{}, a, b uuid.UUID) (bool, error) {
				return false, nil
			},
		}
		transactor := &mockSocialTransactor{}

		service := application.NewSocialService(transactor, repo, mockOutbox)

		blocked, err := service.IsBlocked(context.Background(), userA, userB)

		require.NoError(t, err)
		assert.False(t, blocked)
	})
}

// TestSocialService_ListFollowers tests listing followers.
func TestSocialService_ListFollowers(t *testing.T) {
	mockOutbox := &mockSocialOutboxInserter{}

	t.Run("returns list of followers", func(t *testing.T) {
		userID := uuid.New()
		expectedFollowers := []uuid.UUID{
			uuid.New(),
			uuid.New(),
			uuid.New(),
		}

		repo := &mockSocialRepository{
			listFollowersFunc: func(ctx context.Context, tx interface{}, uid uuid.UUID, limit int, cursor *time.Time) ([]uuid.UUID, error) {
				return expectedFollowers, nil
			},
		}
		transactor := &mockSocialTransactor{}

		service := application.NewSocialService(transactor, repo, mockOutbox)

		followers, err := service.ListFollowers(context.Background(), userID, 10, nil)

		require.NoError(t, err)
		assert.Equal(t, expectedFollowers, followers)
	})

	t.Run("supports pagination with cursor", func(t *testing.T) {
		userID := uuid.New()
		cursorTime := time.Now().Add(-1 * time.Hour)
		var passedCursor *time.Time

		repo := &mockSocialRepository{
			listFollowersFunc: func(ctx context.Context, tx interface{}, uid uuid.UUID, limit int, cursor *time.Time) ([]uuid.UUID, error) {
				passedCursor = cursor
				return []uuid.UUID{}, nil
			},
		}
		transactor := &mockSocialTransactor{}

		service := application.NewSocialService(transactor, repo, mockOutbox)

		_, err := service.ListFollowers(context.Background(), userID, 10, &cursorTime)

		require.NoError(t, err)
		assert.NotNil(t, passedCursor)
		assert.Equal(t, cursorTime, *passedCursor)
	})
}

// TestSocialService_ListFollowing tests listing following.
func TestSocialService_ListFollowing(t *testing.T) {
	mockOutbox := &mockSocialOutboxInserter{}

	t.Run("returns list of following", func(t *testing.T) {
		userID := uuid.New()
		expectedFollowing := []uuid.UUID{
			uuid.New(),
			uuid.New(),
			uuid.New(),
		}

		repo := &mockSocialRepository{
			listFollowingFunc: func(ctx context.Context, tx interface{}, uid uuid.UUID, limit int, cursor *time.Time) ([]uuid.UUID, error) {
				return expectedFollowing, nil
			},
		}
		transactor := &mockSocialTransactor{}

		service := application.NewSocialService(transactor, repo, mockOutbox)

		following, err := service.ListFollowing(context.Background(), userID, 10, nil)

		require.NoError(t, err)
		assert.Equal(t, expectedFollowing, following)
	})

	t.Run("supports pagination with cursor", func(t *testing.T) {
		userID := uuid.New()
		cursorTime := time.Now().Add(-1 * time.Hour)
		var passedCursor *time.Time

		repo := &mockSocialRepository{
			listFollowingFunc: func(ctx context.Context, tx interface{}, uid uuid.UUID, limit int, cursor *time.Time) ([]uuid.UUID, error) {
				passedCursor = cursor
				return []uuid.UUID{}, nil
			},
		}
		transactor := &mockSocialTransactor{}

		service := application.NewSocialService(transactor, repo, mockOutbox)

		_, err := service.ListFollowing(context.Background(), userID, 10, &cursorTime)

		require.NoError(t, err)
		assert.NotNil(t, passedCursor)
		assert.Equal(t, cursorTime, *passedCursor)
	})
}

// TestSocialService_ErrorHandling tests error handling.
func TestSocialService_ErrorHandling(t *testing.T) {
	t.Run("returns error when repository fails", func(t *testing.T) {
		followerID := uuid.New()
		followingID := uuid.New()

		repo := &mockSocialRepository{
			acquireFollowLockFunc: func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) error {
				return errors.New("database error")
			},
		}
		transactor := &mockSocialTransactor{}
		mockOutbox := &mockSocialOutboxInserter{}

		service := application.NewSocialService(transactor, repo, mockOutbox)

		err := service.Follow(context.Background(), followerID, followingID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to acquire follow lock")
	})
}

// TestSocialService_RaceConditionProtection tests that race conditions are prevented.
func TestSocialService_RaceConditionProtection(t *testing.T) {
	mockOutbox := &mockSocialOutboxInserter{}

	t.Run("follow uses AcquireFollowLock to prevent race conditions", func(t *testing.T) {
		followerID := uuid.New()
		followingID := uuid.New()
		acquireLockCalled := false
		var capturedUserA, capturedUserB uuid.UUID

		repo := &mockSocialRepository{
			acquireFollowLockFunc: func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) error {
				acquireLockCalled = true
				capturedUserA = userA
				capturedUserB = userB
				return nil
			},
			existsBlockFunc: func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (bool, error) {
				return false, nil // No block
			},
			insertFollowFunc: func(ctx context.Context, tx interface{}, fid, tid uuid.UUID) error {
				return nil
			},
		}
		transactor := &mockSocialTransactor{}

		service := application.NewSocialService(transactor, repo, mockOutbox)

		err := service.Follow(context.Background(), followerID, followingID)

		require.NoError(t, err)
		assert.True(t, acquireLockCalled, "AcquireFollowLock should be called to prevent race conditions")
		assert.Equal(t, followerID, capturedUserA)
		assert.Equal(t, followingID, capturedUserB)
	})

	t.Run("follow rejected when ExistsBlock detects existing block after lock", func(t *testing.T) {
		followerID := uuid.New()
		followingID := uuid.New()
		insertFollowCalled := false

		repo := &mockSocialRepository{
			acquireFollowLockFunc: func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) error {
				return nil // Lock acquired successfully
			},
			existsBlockFunc: func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (bool, error) {
				return true, nil // Block exists
			},
			insertFollowFunc: func(ctx context.Context, tx interface{}, fid, tid uuid.UUID) error {
				insertFollowCalled = true
				return nil
			},
		}
		transactor := &mockSocialTransactor{}

		service := application.NewSocialService(transactor, repo, mockOutbox)

		err := service.Follow(context.Background(), followerID, followingID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot follow: block exists")
		assert.False(t, insertFollowCalled, "InsertFollow should NOT be called when block exists")
	})

	t.Run("AcquireFollowLock error prevents follow insertion", func(t *testing.T) {
		followerID := uuid.New()
		followingID := uuid.New()
		insertFollowCalled := false

		repo := &mockSocialRepository{
			acquireFollowLockFunc: func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) error {
				return errors.New("lock acquisition failed")
			},
			insertFollowFunc: func(ctx context.Context, tx interface{}, fid, tid uuid.UUID) error {
				insertFollowCalled = true
				return nil
			},
		}
		transactor := &mockSocialTransactor{}

		service := application.NewSocialService(transactor, repo, mockOutbox)

		err := service.Follow(context.Background(), followerID, followingID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to acquire follow lock")
		assert.False(t, insertFollowCalled, "InsertFollow should NOT be called when lock acquisition fails")
	})
}

// mockSocialOutboxInserter is a mock implementation of OutboxInserter for testing.
type mockSocialOutboxInserter struct {
	insertTxFunc func(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error
}

func (m *mockSocialOutboxInserter) InsertTx(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error {
	if m.insertTxFunc != nil {
		return m.insertTxFunc(ctx, tx, eventType, payload, idempotencyKey)
	}
	return nil
}

// TestSocialService_IdempotencyKey tests that idempotency keys follow the correct format.
func TestSocialService_IdempotencyKey(t *testing.T) {
	t.Run("idempotency key format is user.followed.{recipientID}.{actorID}", func(t *testing.T) {
		followerID := uuid.New()  // actor
		followingID := uuid.New() // recipient
		var capturedIdempotencyKey string
		var capturedEventType string

		repo := &mockSocialRepository{
			existsBlockFunc: func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (bool, error) {
				return false, nil
			},
			insertFollowFunc: func(ctx context.Context, tx interface{}, fid, tid uuid.UUID) error {
				return nil
			},
		}

		outboxRepo := &mockSocialOutboxInserter{
			insertTxFunc: func(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error {
				capturedEventType = eventType
				capturedIdempotencyKey = idempotencyKey
				return nil
			},
		}

		transactor := &mockSocialTransactor{}

		service := application.NewSocialService(transactor, repo, outboxRepo)

		err := service.Follow(context.Background(), followerID, followingID)

		require.NoError(t, err)
		assert.Equal(t, events.EventUserFollowed, capturedEventType)
		// Format: user.followed.{recipientID}.{actorID}
		assert.Equal(t, "user.followed."+followingID.String()+"."+followerID.String(), capturedIdempotencyKey)
	})
}

// TestSocialService_EventEmission tests that all social mutations emit the correct events.
func TestSocialService_EventEmission(t *testing.T) {
	t.Run("Follow emits user.followed event", func(t *testing.T) {
		followerID := uuid.New()
		followingID := uuid.New()
		var capturedEventType string
		var capturedPayload map[string]any

		repo := &mockSocialRepository{
			existsBlockFunc: func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (bool, error) {
				return false, nil
			},
			insertFollowFunc: func(ctx context.Context, tx interface{}, fid, tid uuid.UUID) error {
				return nil
			},
		}

		outboxRepo := &mockSocialOutboxInserter{
			insertTxFunc: func(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error {
				capturedEventType = eventType
				capturedPayload = payload.(map[string]any)
				return nil
			},
		}

		transactor := &mockSocialTransactor{}
		service := application.NewSocialService(transactor, repo, outboxRepo)

		err := service.Follow(context.Background(), followerID, followingID)

		require.NoError(t, err)
		assert.Equal(t, events.EventUserFollowed, capturedEventType)
		assert.Equal(t, followerID, capturedPayload["actor_id"])
		assert.Equal(t, followingID, capturedPayload["recipient_id"])
	})

	t.Run("Unfollow emits user.unfollowed event", func(t *testing.T) {
		followerID := uuid.New()
		followingID := uuid.New()
		var capturedEventType string
		var capturedPayload map[string]any

		repo := &mockSocialRepository{
			deleteFollowFunc: func(ctx context.Context, tx interface{}, fid, tid uuid.UUID) error {
				return nil
			},
		}

		outboxRepo := &mockSocialOutboxInserter{
			insertTxFunc: func(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error {
				capturedEventType = eventType
				capturedPayload = payload.(map[string]any)
				return nil
			},
		}

		transactor := &mockSocialTransactor{}
		service := application.NewSocialService(transactor, repo, outboxRepo)

		err := service.Unfollow(context.Background(), followerID, followingID)

		require.NoError(t, err)
		assert.Equal(t, events.EventUserUnfollowed, capturedEventType)
		assert.Equal(t, followerID, capturedPayload["actor_id"])
		assert.Equal(t, followingID, capturedPayload["recipient_id"])
	})

	t.Run("Block emits user.blocked event", func(t *testing.T) {
		blockerID := uuid.New()
		blockedID := uuid.New()
		var capturedEventType string
		var capturedPayload map[string]any

		repo := &mockSocialRepository{
			insertBlockFunc: func(ctx context.Context, tx interface{}, bid, tid uuid.UUID) error {
				return nil
			},
			deleteFollowBothDirectionsFunc: func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) error {
				return nil
			},
		}

		outboxRepo := &mockSocialOutboxInserter{
			insertTxFunc: func(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error {
				capturedEventType = eventType
				capturedPayload = payload.(map[string]any)
				return nil
			},
		}

		transactor := &mockSocialTransactor{}
		service := application.NewSocialService(transactor, repo, outboxRepo)

		err := service.Block(context.Background(), blockerID, blockedID)

		require.NoError(t, err)
		assert.Equal(t, events.EventUserBlocked, capturedEventType)
		assert.Equal(t, blockerID, capturedPayload["actor_id"])
		assert.Equal(t, blockedID, capturedPayload["recipient_id"])
	})

	t.Run("Unblock is silent no outbox", func(t *testing.T) {
		blockerID := uuid.New()
		blockedID := uuid.New()
		insertCalled := false

		repo := &mockSocialRepository{
			deleteBlockFunc: func(ctx context.Context, tx interface{}, bid, tid uuid.UUID) error {
				return nil
			},
		}

		outboxRepo := &mockSocialOutboxInserter{
			insertTxFunc: func(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error {
				insertCalled = true
				return nil
			},
		}

		transactor := &mockSocialTransactor{}
		service := application.NewSocialService(transactor, repo, outboxRepo)

		err := service.Unblock(context.Background(), blockerID, blockedID)

		require.NoError(t, err)
		assert.False(t, insertCalled, "unblock must not emit outbox (silent, no consumer) Phase 2A")
	})
}

// TestSocialService_EventIdempotency tests that retry requests don't create duplicate events.
func TestSocialService_EventIdempotency(t *testing.T) {
	t.Run("retry follow request uses same idempotency key", func(t *testing.T) {
		followerID := uuid.New()
		followingID := uuid.New()
		idempotencyKeys := []string{}

		repo := &mockSocialRepository{
			existsBlockFunc: func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (bool, error) {
				return false, nil
			},
			insertFollowFunc: func(ctx context.Context, tx interface{}, fid, tid uuid.UUID) error {
				return nil
			},
		}

		outboxRepo := &mockSocialOutboxInserter{
			insertTxFunc: func(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error {
				idempotencyKeys = append(idempotencyKeys, idempotencyKey)
				return nil
			},
		}

		transactor := &mockSocialTransactor{}
		service := application.NewSocialService(transactor, repo, outboxRepo)

		// Follow twice with same IDs
		_ = service.Follow(context.Background(), followerID, followingID)
		_ = service.Follow(context.Background(), followerID, followingID)

		// Both requests should use the same idempotency key
		assert.Equal(t, 2, len(idempotencyKeys))
		assert.Equal(t, idempotencyKeys[0], idempotencyKeys[1])
	})

	t.Run("retry unfollow request uses same idempotency key", func(t *testing.T) {
		followerID := uuid.New()
		followingID := uuid.New()
		idempotencyKeys := []string{}

		repo := &mockSocialRepository{
			deleteFollowFunc: func(ctx context.Context, tx interface{}, fid, tid uuid.UUID) error {
				return nil
			},
		}

		outboxRepo := &mockSocialOutboxInserter{
			insertTxFunc: func(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error {
				idempotencyKeys = append(idempotencyKeys, idempotencyKey)
				return nil
			},
		}

		transactor := &mockSocialTransactor{}
		service := application.NewSocialService(transactor, repo, outboxRepo)

		// Unfollow twice with same IDs
		_ = service.Unfollow(context.Background(), followerID, followingID)
		_ = service.Unfollow(context.Background(), followerID, followingID)

		// Both requests should use the same idempotency key
		assert.Equal(t, 2, len(idempotencyKeys))
		assert.Equal(t, idempotencyKeys[0], idempotencyKeys[1])
	})

	t.Run("retry block request uses same idempotency key", func(t *testing.T) {
		blockerID := uuid.New()
		blockedID := uuid.New()
		idempotencyKeys := []string{}

		repo := &mockSocialRepository{
			insertBlockFunc: func(ctx context.Context, tx interface{}, bid, tid uuid.UUID) error {
				return nil
			},
			deleteFollowBothDirectionsFunc: func(ctx context.Context, tx interface{}, userA, userB uuid.UUID) error {
				return nil
			},
		}

		outboxRepo := &mockSocialOutboxInserter{
			insertTxFunc: func(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error {
				idempotencyKeys = append(idempotencyKeys, idempotencyKey)
				return nil
			},
		}

		transactor := &mockSocialTransactor{}
		service := application.NewSocialService(transactor, repo, outboxRepo)

		// Block twice with same IDs
		_ = service.Block(context.Background(), blockerID, blockedID)
		_ = service.Block(context.Background(), blockerID, blockedID)

		// Both requests should use the same idempotency key
		assert.Equal(t, 2, len(idempotencyKeys))
		assert.Equal(t, idempotencyKeys[0], idempotencyKeys[1])
	})

	t.Run("retry unblock request uses same idempotency key", func(t *testing.T) {
		blockerID := uuid.New()
		blockedID := uuid.New()
		idempotencyKeys := []string{}

		repo := &mockSocialRepository{
			deleteBlockFunc: func(ctx context.Context, tx interface{}, bid, tid uuid.UUID) error {
				return nil
			},
		}

		outboxRepo := &mockSocialOutboxInserter{
			insertTxFunc: func(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error {
				idempotencyKeys = append(idempotencyKeys, idempotencyKey)
				return nil
			},
		}

		transactor := &mockSocialTransactor{}
		service := application.NewSocialService(transactor, repo, outboxRepo)

		// Unblock twice with same IDs - no outbox per Phase 2A silent convergence
		_ = service.Unblock(context.Background(), blockerID, blockedID)
		_ = service.Unblock(context.Background(), blockerID, blockedID)

		// No events expected
		assert.Equal(t, 0, len(idempotencyKeys))
	})
}


