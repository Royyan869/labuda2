//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	feedentity "github.com/labuda/backend/internal/social/feed/entity"
	"github.com/labuda/backend/internal/social/feed/infrastructure/repository"
	"github.com/labuda/backend/pkg/testdb"
)

func beginIsolatedFeedTx(t *testing.T, ctx context.Context, testDB *testdb.TestDB) pgx.Tx {
	t.Helper()

	// Truncate all tables before each subtest to guarantee isolation.
	// TruncateAll is the canonical cleanup authority in testdb.TestDB.
	if err := testDB.TruncateAll(ctx); err != nil {
		t.Fatalf("truncate all tables: %v", err)
	}

	tx, err := testDB.Pool().Begin(ctx)
	require.NoError(t, err)

	return tx
}

func createFeedTestUserWithStatusTx(
	t *testing.T,
	ctx context.Context,
	tx pgx.Tx,
	status string,
	deleted bool,
) uuid.UUID {
	t.Helper()

	userID := uuid.New()
	if deleted {
		_, err := tx.Exec(ctx, `
			INSERT INTO users (id, firebase_uid, email, email_verified_at, phone_verified, account_status, deleted_at, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), true, $4, NOW(), NOW(), NOW())
		`, userID, userID.String(), userID.String()+"@test.com", status)
		require.NoError(t, err)
		return userID
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, email_verified_at, phone_verified, account_status, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), true, $4, NOW(), NOW())
	`, userID, userID.String(), userID.String()+"@test.com", status)
	require.NoError(t, err)
	return userID
}

func createDeletedFeedTestUserTx(
	t *testing.T,
	ctx context.Context,
	tx pgx.Tx,
) uuid.UUID {
	return createFeedTestUserWithStatusTx(t, ctx, tx, "active", true)
}

func createFeedTestContentTx(
	t *testing.T,
	ctx context.Context,
	tx pgx.Tx,
	authorID uuid.UUID,
	status string,
	createdAt time.Time,
) uuid.UUID {
	return createFeedTestContentTxWithVisibility(t, ctx, tx, authorID, status, "public", false, createdAt)
}

func createFeedTestContentTxWithVisibility(
	t *testing.T,
	ctx context.Context,
	tx pgx.Tx,
	authorID uuid.UUID,
	status string,
	visibility string,
	isHidden bool,
	createdAt time.Time,
) uuid.UUID {
	t.Helper()

	contentID := uuid.New()
	_, err := tx.Exec(ctx, `
		INSERT INTO contents (id, author_id, status, caption, visibility, is_hidden, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
	`, contentID, authorID, status, "test content", visibility, isHidden, createdAt)
	require.NoError(t, err)

	return contentID
}

func feedItemIDs(items []*feedentity.FeedItem) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func assertNoDuplicateIDs(ids []uuid.UUID) error {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			return fmt.Errorf("duplicate feed id: %s", id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func TestFeedRepository_FollowFirstDiscoveryBootstrap(t *testing.T) {
	testDB, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("no-follow viewer receives public discovery", func(t *testing.T) {
		tx := beginIsolatedFeedTx(t, ctx, testDB)
		defer func() { _ = tx.Rollback(ctx) }()

		viewer := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
		publicAuthor := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
		secondPublicAuthor := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
		followersOnlyAuthor := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
		privateAuthor := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
		hiddenAuthor := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
		suspendedAuthor := createFeedTestUserWithStatusTx(t, ctx, tx, "suspended", false)
		blockedAuthor := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
		deletedAuthor := createDeletedFeedTestUserTx(t, ctx, tx)

		base := time.Date(2099, 1, 1, 12, 0, 0, 0, time.UTC)
		public1 := createFeedTestContentTx(t, ctx, tx, publicAuthor, "active", base)
		public2 := createFeedTestContentTx(t, ctx, tx, secondPublicAuthor, "active", base.Add(-1*time.Minute))
		followersOnlyOther := createFeedTestContentTxWithVisibility(t, ctx, tx, followersOnlyAuthor, "active", "followers_only", false, base.Add(-2*time.Minute))
		privateOther := createFeedTestContentTxWithVisibility(t, ctx, tx, privateAuthor, "active", "private", false, base.Add(-3*time.Minute))

		hiddenID := uuid.New()
		_, err := tx.Exec(ctx, `
			INSERT INTO contents (id, author_id, status, caption, visibility, is_hidden, created_at, updated_at)
			VALUES ($1, $2, 'active', 'hidden', 'public', true, $3, $3)
		`, hiddenID, hiddenAuthor, base.Add(-2*time.Minute))
		require.NoError(t, err)

		suspendedContent := createFeedTestContentTx(t, ctx, tx, suspendedAuthor, "active", base.Add(-4*time.Minute))
		deletedContent := createFeedTestContentTx(t, ctx, tx, deletedAuthor, "deleted", base.Add(-5*time.Minute))
		blockedContent := createFeedTestContentTx(t, ctx, tx, blockedAuthor, "active", base.Add(-6*time.Minute))

		_, err = tx.Exec(ctx, `
			INSERT INTO user_blocks (blocker_id, blocked_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, blockedAuthor)
		require.NoError(t, err)

		repo := repository.NewFeedRepository()
		result, err := repo.GetFeed(ctx, tx, viewer, nil, 20)
		require.NoError(t, err)

		ids := feedItemIDs(result.Items)
		assert.Equal(t, []uuid.UUID{public1, public2}, ids)
		assert.NotContains(t, ids, hiddenID)
		assert.NotContains(t, ids, followersOnlyOther)
		assert.NotContains(t, ids, privateOther)
		assert.NotContains(t, ids, suspendedContent)
		assert.NotContains(t, ids, deletedContent)
		assert.NotContains(t, ids, blockedContent)
		assert.False(t, result.HasMore)
		assert.Nil(t, result.NextCursor)
	})

	t.Run("followed content ranks before discovery", func(t *testing.T) {
		tx := beginIsolatedFeedTx(t, ctx, testDB)
		defer func() { _ = tx.Rollback(ctx) }()

		viewer := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
		followedAuthor := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
		discoveryAuthor := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)

		_, err := tx.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, followedAuthor)
		require.NoError(t, err)

		base := time.Date(2099, 1, 1, 12, 0, 0, 0, time.UTC)
		followed1 := createFeedTestContentTx(t, ctx, tx, followedAuthor, "active", base.Add(-2*time.Minute))
		followed2 := createFeedTestContentTx(t, ctx, tx, followedAuthor, "active", base.Add(-3*time.Minute))
		discovery := createFeedTestContentTx(t, ctx, tx, discoveryAuthor, "active", base)

		repo := repository.NewFeedRepository()
		result, err := repo.GetFeed(ctx, tx, viewer, nil, 20)
		require.NoError(t, err)

		ids := feedItemIDs(result.Items)
		assert.Equal(t, []uuid.UUID{followed1, followed2, discovery}, ids)
		assert.Equal(t, 3, len(ids))
		assert.False(t, result.HasMore)
		assert.Nil(t, result.NextCursor)
	})

	t.Run("partial followed page fills with discovery", func(t *testing.T) {
		tx := beginIsolatedFeedTx(t, ctx, testDB)
		defer func() { _ = tx.Rollback(ctx) }()

		viewer := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
		followedAuthor := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
		discoveryAuthor1 := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
		discoveryAuthor2 := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
		discoveryAuthor3 := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)

		_, err := tx.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, followedAuthor)
		require.NoError(t, err)

		base := time.Date(2099, 1, 1, 12, 0, 0, 0, time.UTC)
		followed1 := createFeedTestContentTx(t, ctx, tx, followedAuthor, "active", base)
		followed2 := createFeedTestContentTx(t, ctx, tx, followedAuthor, "active", base.Add(-1*time.Minute))
		discovery1 := createFeedTestContentTx(t, ctx, tx, discoveryAuthor1, "active", base.Add(-2*time.Minute))
		discovery2 := createFeedTestContentTx(t, ctx, tx, discoveryAuthor2, "active", base.Add(-3*time.Minute))
		discovery3 := createFeedTestContentTx(t, ctx, tx, discoveryAuthor3, "active", base.Add(-4*time.Minute))

		repo := repository.NewFeedRepository()
		result, err := repo.GetFeed(ctx, tx, viewer, nil, 4)
		require.NoError(t, err)

		ids := feedItemIDs(result.Items)
		require.Len(t, ids, 4)
		assert.Equal(t, []uuid.UUID{followed1, followed2, discovery1, discovery2}, ids)
		assert.NotContains(t, ids, discovery3)
		assert.NoError(t, assertNoDuplicateIDs(ids))
		require.NotNil(t, result.NextCursor)
		assert.Equal(t, 1, result.NextCursor.PriorityGroup)
		assert.True(t, result.HasMore)
	})

	t.Run("followers-only authorization", func(t *testing.T) {
		tx := beginIsolatedFeedTx(t, ctx, testDB)
		defer func() { _ = tx.Rollback(ctx) }()

		author := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
		followerViewer := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
		strangerViewer := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)

		_, err := tx.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, followerViewer, author)
		require.NoError(t, err)

		base := time.Date(2099, 1, 1, 12, 0, 0, 0, time.UTC)
		followersOnly := createFeedTestContentTxWithVisibility(t, ctx, tx, author, "active", "followers_only", false, base)
		publicContent := createFeedTestContentTxWithVisibility(t, ctx, tx, author, "active", "public", false, base.Add(-1*time.Minute))

		repo := repository.NewFeedRepository()

		followerPage, err := repo.GetFeed(ctx, tx, followerViewer, nil, 20)
		require.NoError(t, err)
		assert.Equal(t, []uuid.UUID{followersOnly, publicContent}, feedItemIDs(followerPage.Items))
		assert.False(t, followerPage.HasMore)
		assert.Nil(t, followerPage.NextCursor)

		strangerPage, err := repo.GetFeed(ctx, tx, strangerViewer, nil, 20)
		require.NoError(t, err)
		assert.Equal(t, []uuid.UUID{publicContent}, feedItemIDs(strangerPage.Items))
		assert.False(t, strangerPage.HasMore)
		assert.Nil(t, strangerPage.NextCursor)

		ownerPage, err := repo.GetFeed(ctx, tx, author, nil, 20)
		require.NoError(t, err)
		assert.Equal(t, []uuid.UUID{followersOnly, publicContent}, feedItemIDs(ownerPage.Items))
		assert.False(t, ownerPage.HasMore)
		assert.Nil(t, ownerPage.NextCursor)
	})

	t.Run("private owner-only authorization", func(t *testing.T) {
		tx := beginIsolatedFeedTx(t, ctx, testDB)
		defer func() { _ = tx.Rollback(ctx) }()

		author := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
		stranger := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)

		privateContent := createFeedTestContentTxWithVisibility(t, ctx, tx, author, "active", "private", false, time.Date(2099, 1, 1, 12, 0, 0, 0, time.UTC))

		repo := repository.NewFeedRepository()

		authorPage, err := repo.GetFeed(ctx, tx, author, nil, 20)
		require.NoError(t, err)
		assert.Equal(t, []uuid.UUID{privateContent}, feedItemIDs(authorPage.Items))
		assert.False(t, authorPage.HasMore)
		assert.Nil(t, authorPage.NextCursor)

		strangerPage, err := repo.GetFeed(ctx, tx, stranger, nil, 20)
		require.NoError(t, err)
		assert.Empty(t, strangerPage.Items)
		assert.False(t, strangerPage.HasMore)
		assert.Nil(t, strangerPage.NextCursor)
	})

	t.Run("blocked and unavailable rows excluded", func(t *testing.T) {
		tx := beginIsolatedFeedTx(t, ctx, testDB)
		defer func() { _ = tx.Rollback(ctx) }()

		viewer := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
		blockedAuthor := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
		suspendedAuthor := createFeedTestUserWithStatusTx(t, ctx, tx, "suspended", false)
		deletedAuthor := createDeletedFeedTestUserTx(t, ctx, tx)
		activeAuthor := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)

		_, err := tx.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW()), ($1, $3, NOW()), ($1, $4, NOW()), ($1, $5, NOW())
		`, viewer, blockedAuthor, suspendedAuthor, deletedAuthor, activeAuthor)
		require.NoError(t, err)

		_, err = tx.Exec(ctx, `
			INSERT INTO user_blocks (blocker_id, blocked_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, blockedAuthor)
		require.NoError(t, err)

		base := time.Date(2099, 1, 1, 12, 0, 0, 0, time.UTC)
		blockedContent := createFeedTestContentTxWithVisibility(t, ctx, tx, blockedAuthor, "active", "public", false, base)
		suspendedContent := createFeedTestContentTx(t, ctx, tx, suspendedAuthor, "active", base.Add(-1*time.Minute))
		deletedContent := createFeedTestContentTx(t, ctx, tx, deletedAuthor, "deleted", base.Add(-2*time.Minute))
		activeContent := createFeedTestContentTx(t, ctx, tx, activeAuthor, "active", base.Add(-3*time.Minute))

		repo := repository.NewFeedRepository()
		result, err := repo.GetFeed(ctx, tx, viewer, nil, 20)
		require.NoError(t, err)

		ids := feedItemIDs(result.Items)
		assert.Equal(t, []uuid.UUID{activeContent}, ids)
		assert.NotContains(t, ids, blockedContent)
		assert.NotContains(t, ids, suspendedContent)
		assert.NotContains(t, ids, deletedContent)
		assert.False(t, result.HasMore)
		assert.Nil(t, result.NextCursor)
	})

	t.Run("mixed pagination reaches discovery", func(t *testing.T) {
		tx := beginIsolatedFeedTx(t, ctx, testDB)
		defer func() { _ = tx.Rollback(ctx) }()

		viewer := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
		followedAuthor := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
		discoveryAuthor1 := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
		discoveryAuthor2 := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)

		_, err := tx.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, followedAuthor)
		require.NoError(t, err)

		base := time.Date(2099, 1, 1, 12, 0, 0, 0, time.UTC)
		followed1 := createFeedTestContentTx(t, ctx, tx, followedAuthor, "active", base)
		followed2 := createFeedTestContentTx(t, ctx, tx, followedAuthor, "active", base.Add(-1*time.Minute))
		followed3 := createFeedTestContentTx(t, ctx, tx, followedAuthor, "active", base.Add(-2*time.Minute))
		followed4 := createFeedTestContentTx(t, ctx, tx, followedAuthor, "active", base.Add(-3*time.Minute))
		discovery1 := createFeedTestContentTx(t, ctx, tx, discoveryAuthor1, "active", base.Add(-4*time.Minute))
		discovery2 := createFeedTestContentTx(t, ctx, tx, discoveryAuthor2, "active", base.Add(-5*time.Minute))

		repo := repository.NewFeedRepository()

		page1, err := repo.GetFeed(ctx, tx, viewer, nil, 3)
		require.NoError(t, err)
		require.Len(t, page1.Items, 3)
		assert.Equal(t, []uuid.UUID{followed1, followed2, followed3}, feedItemIDs(page1.Items))
		require.NotNil(t, page1.NextCursor)
		assert.Equal(t, 0, page1.NextCursor.PriorityGroup)
		assert.True(t, page1.HasMore)

		page2, err := repo.GetFeed(ctx, tx, viewer, page1.NextCursor, 3)
		require.NoError(t, err)
		require.Len(t, page2.Items, 3)
		assert.Equal(t, []uuid.UUID{followed4, discovery1, discovery2}, feedItemIDs(page2.Items))
		assert.False(t, page2.HasMore)
		assert.Nil(t, page2.NextCursor)

		merged := append(feedItemIDs(page1.Items), feedItemIDs(page2.Items)...)
		assert.NoError(t, assertNoDuplicateIDs(merged))
		assert.Equal(t, 6, len(merged))
	})

	t.Run("no-follow pagination", func(t *testing.T) {
		tx := beginIsolatedFeedTx(t, ctx, testDB)
		defer func() { _ = tx.Rollback(ctx) }()

		viewer := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
		author1 := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
		author2 := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
		author3 := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
		author4 := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)

		base := time.Date(2099, 1, 1, 12, 0, 0, 0, time.UTC)
		c1 := createFeedTestContentTx(t, ctx, tx, author1, "active", base)
		c2 := createFeedTestContentTx(t, ctx, tx, author2, "active", base.Add(-1*time.Minute))
		c3 := createFeedTestContentTx(t, ctx, tx, author3, "active", base.Add(-2*time.Minute))
		c4 := createFeedTestContentTx(t, ctx, tx, author4, "active", base.Add(-3*time.Minute))

		repo := repository.NewFeedRepository()

		page1, err := repo.GetFeed(ctx, tx, viewer, nil, 2)
		require.NoError(t, err)
		require.Len(t, page1.Items, 2)
		assert.Equal(t, []uuid.UUID{c1, c2}, feedItemIDs(page1.Items))
		require.NotNil(t, page1.NextCursor)
		assert.Equal(t, 1, page1.NextCursor.PriorityGroup)
		assert.True(t, page1.HasMore)

		page2, err := repo.GetFeed(ctx, tx, viewer, page1.NextCursor, 2)
		require.NoError(t, err)
		require.Len(t, page2.Items, 2)
		assert.Equal(t, []uuid.UUID{c3, c4}, feedItemIDs(page2.Items))
		assert.False(t, page2.HasMore)
		assert.Nil(t, page2.NextCursor)

		merged := append(feedItemIDs(page1.Items), feedItemIDs(page2.Items)...)
		assert.NoError(t, assertNoDuplicateIDs(merged))
	})

	t.Run("exact-limit terminal page", func(t *testing.T) {
		tx := beginIsolatedFeedTx(t, ctx, testDB)
		defer func() { _ = tx.Rollback(ctx) }()

		viewer := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
		followedAuthor := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
		discoveryAuthor := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)

		_, err := tx.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, followedAuthor)
		require.NoError(t, err)

		base := time.Date(2099, 1, 1, 12, 0, 0, 0, time.UTC)
		followed := createFeedTestContentTx(t, ctx, tx, followedAuthor, "active", base)
		discovery := createFeedTestContentTx(t, ctx, tx, discoveryAuthor, "active", base.Add(-1*time.Minute))

		repo := repository.NewFeedRepository()
		page, err := repo.GetFeed(ctx, tx, viewer, nil, 2)
		require.NoError(t, err)
		require.Len(t, page.Items, 2)
		assert.Equal(t, []uuid.UUID{followed, discovery}, feedItemIDs(page.Items))
		assert.False(t, page.HasMore)
		assert.Nil(t, page.NextCursor)
	})

	t.Run("equal-timestamp no-skip/no-duplicate", func(t *testing.T) {
		tx := beginIsolatedFeedTx(t, ctx, testDB)
		defer func() { _ = tx.Rollback(ctx) }()

		viewer := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
		followedAuthor := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)

		_, err := tx.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, followedAuthor)
		require.NoError(t, err)

		sharedTS := time.Date(2099, 1, 1, 12, 0, 0, 0, time.UTC)
		c1 := createFeedTestContentTx(t, ctx, tx, followedAuthor, "active", sharedTS)
		c2 := createFeedTestContentTx(t, ctx, tx, followedAuthor, "active", sharedTS)
		c3 := createFeedTestContentTx(t, ctx, tx, followedAuthor, "active", sharedTS)

		repo := repository.NewFeedRepository()

		page1, err := repo.GetFeed(ctx, tx, viewer, nil, 2)
		require.NoError(t, err)
		require.Len(t, page1.Items, 2)
		assert.True(t, page1.HasMore)
		require.NotNil(t, page1.NextCursor)
		assert.Equal(t, 0, page1.NextCursor.PriorityGroup)
		assert.Equal(t, sharedTS.Truncate(time.Microsecond).UTC(), page1.NextCursor.CreatedAt.UTC())

		page2, err := repo.GetFeed(ctx, tx, viewer, page1.NextCursor, 2)
		require.NoError(t, err)
		require.Len(t, page2.Items, 1)
		assert.False(t, page2.HasMore)
		assert.Nil(t, page2.NextCursor)

		seen := map[uuid.UUID]int{}
		for _, item := range page1.Items {
			seen[item.ID]++
		}
		for _, item := range page2.Items {
			seen[item.ID]++
		}
		assert.Equal(t, 1, seen[c1])
		assert.Equal(t, 1, seen[c2])
		assert.Equal(t, 1, seen[c3])
	})
}
