//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	feedentity "github.com/labuda/backend/internal/social/feed/entity"
	"github.com/labuda/backend/internal/social/feed/infrastructure/repository"
	"github.com/labuda/backend/pkg/testdb"
)

// setupTestDB creates test database with required tables and data.
func setupTestDB(t *testing.T) (*testdb.TestDB, func()) {
	t.Helper()
	testDB, cleanup := testdb.SetupDB(t)
	return testDB, cleanup
}

// createTestUser creates a test user in the database.
func createTestUser(t *testing.T, ctx context.Context, testDB *testdb.TestDB) uuid.UUID {
	t.Helper()
	userID := uuid.New()
	pool := testDB.Pool()

	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, email_verified_at, phone_verified, account_status, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), true, 'active', NOW(), NOW())
	`, userID, userID.String(), userID.String()+"@test.com")
	require.NoError(t, err)

	return userID
}

// createTestContent creates a test content in the database.
//
// SCHEMA ALIGNMENT (Batch 3J): contents has no `body` column under the
// canonical 000100_initial_schema head — only `caption`. Fixture
// previously inserted both, which silently broke the moment any
// `c.body` SELECT ran. Insert canonical columns only.
func createTestContent(t *testing.T, ctx context.Context, testDB *testdb.TestDB, authorID uuid.UUID, status string, createdAt time.Time) uuid.UUID {
	t.Helper()
	contentID := uuid.New()
	pool := testDB.Pool()

	caption := "test content"
	_, err := pool.Exec(ctx, `
		INSERT INTO contents (id, author_id, status, caption, is_hidden, created_at, updated_at)
		VALUES ($1, $2, $3, $4, false, $5, $5)
	`, contentID, authorID, status, caption, createdAt)
	require.NoError(t, err)

	return contentID
}

func createTestProduct(
	t *testing.T,
	ctx context.Context,
	testDB *testdb.TestDB,
	sellerID uuid.UUID,
) uuid.UUID {
	t.Helper()
	productID := uuid.New()
	pool := testDB.Pool()

	_, err := pool.Exec(ctx, `
		INSERT INTO products (
			id, seller_id, title, description, media_urls, variety,
			preparation_time, created_at, updated_at
		)
		VALUES ($1, $2, 'test product', 'test product', '[]'::jsonb, 'goat',
			'short', NOW(), NOW())
	`, productID, sellerID)
	require.NoError(t, err)

	return productID
}

func createTestAuction(
	t *testing.T,
	ctx context.Context,
	testDB *testdb.TestDB,
	sellerID uuid.UUID,
	productID uuid.UUID,
	status string,
	startAt time.Time,
	endAt time.Time,
) uuid.UUID {
	t.Helper()
	auctionID := uuid.New()
	pool := testDB.Pool()

	_, err := pool.Exec(ctx, `
		INSERT INTO auctions (
			id, seller_id, product_id,
			start_price, bid_increment, start_at, end_at, status,
			created_at, updated_at
		)
		VALUES ($1, $2, $3,
			1000, 100, $4, $5, $6, NOW(), NOW())
	`, auctionID, sellerID, productID, startAt, endAt, status)
	require.NoError(t, err)

	return auctionID
}

// TestFeedRepository_GetFeed tests the feed query.
func TestFeedRepository_GetFeed(t *testing.T) {
	t.Run("returns only followed users active content", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()

		ctx := context.Background()

		// Create users
		viewer := createTestUser(t, ctx, testDB)
		followedUser := createTestUser(t, ctx, testDB)
		otherUser := createTestUser(t, ctx, testDB)

		// Create follows: viewer follows followedUser
		pool := testDB.Pool()
		_, err := pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, followedUser)
		require.NoError(t, err)

		// Create content
		now := time.Now()
		followedContent1 := createTestContent(t, ctx, testDB, followedUser, "active", now)
		followedContent2 := createTestContent(t, ctx, testDB, followedUser, "active", now.Add(-1*time.Minute))

		// otherContent uses visibility='private' so it is excluded from
		// the feed by the global-discovery clause (only public content
		// from unfollowed users is discoverable).
		otherContent := uuid.New()
		_, err = pool.Exec(ctx, `
			INSERT INTO contents (id, author_id, status, caption, visibility, is_hidden, created_at, updated_at)
			VALUES ($1, $2, 'active', 'other content', 'private', false, $3, $3)
		`, otherContent, otherUser, now.Add(-2*time.Minute))
		require.NoError(t, err)

		// Get feed
		repo := repository.NewFeedRepository()
		result, err := repo.GetFeed(ctx, pool, viewer, nil, 20)

		require.NoError(t, err)
		assert.Len(t, result.Items, 2)

		// Verify content IDs - should only include followed user's content
		contentIDs := make([]uuid.UUID, len(result.Items))
		for i, item := range result.Items {
			contentIDs[i] = item.ID
		}

		assert.Contains(t, contentIDs, followedContent1)
		assert.Contains(t, contentIDs, followedContent2)
		assert.NotContains(t, contentIDs, otherContent)
	})

	t.Run("excludes blocked users content", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()

		ctx := context.Background()

		// Create users
		viewer := createTestUser(t, ctx, testDB)
		followedUser := createTestUser(t, ctx, testDB)
		blockedUser := createTestUser(t, ctx, testDB)

		pool := testDB.Pool()

		// Create follows
		_, err := pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, followedUser)
		require.NoError(t, err)

		_, err = pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, blockedUser)
		require.NoError(t, err)

		// Create block: viewer blocks blockedUser
		_, err = pool.Exec(ctx, `
			INSERT INTO user_blocks (blocker_id, blocked_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, blockedUser)
		require.NoError(t, err)

		// Create content
		now := time.Now()
		followedContent := createTestContent(t, ctx, testDB, followedUser, "active", now)
		blockedContent := createTestContent(t, ctx, testDB, blockedUser, "active", now.Add(-1*time.Minute))

		// Get feed
		repo := repository.NewFeedRepository()
		result, err := repo.GetFeed(ctx, pool, viewer, nil, 20)

		require.NoError(t, err)

		// Should only include followed user's content, not blocked user's
		contentIDs := make([]uuid.UUID, len(result.Items))
		for i, item := range result.Items {
			contentIDs[i] = item.ID
		}

		assert.Contains(t, contentIDs, followedContent)
		assert.NotContains(t, contentIDs, blockedContent)
	})

	t.Run("excludes content from users who blocked viewer", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()

		ctx := context.Background()

		// Create users
		viewer := createTestUser(t, ctx, testDB)
		followedUser := createTestUser(t, ctx, testDB)
		blockerUser := createTestUser(t, ctx, testDB)

		pool := testDB.Pool()

		// Create follows
		_, err := pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, followedUser)
		require.NoError(t, err)

		_, err = pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, blockerUser)
		require.NoError(t, err)

		// Create block: blockerUser blocks viewer
		_, err = pool.Exec(ctx, `
			INSERT INTO user_blocks (blocker_id, blocked_id, created_at)
			VALUES ($1, $2, NOW())
		`, blockerUser, viewer)
		require.NoError(t, err)

		// Create content
		now := time.Now()
		followedContent := createTestContent(t, ctx, testDB, followedUser, "active", now)
		blockerContent := createTestContent(t, ctx, testDB, blockerUser, "active", now.Add(-1*time.Minute))

		// Get feed
		repo := repository.NewFeedRepository()
		result, err := repo.GetFeed(ctx, pool, viewer, nil, 20)

		require.NoError(t, err)

		// Should only include followed user's content, not blocker's content
		contentIDs := make([]uuid.UUID, len(result.Items))
		for i, item := range result.Items {
			contentIDs[i] = item.ID
		}

		assert.Contains(t, contentIDs, followedContent)
		assert.NotContains(t, contentIDs, blockerContent)
	})

	t.Run("excludes deleted content", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()

		ctx := context.Background()

		// Create users
		viewer := createTestUser(t, ctx, testDB)
		followedUser := createTestUser(t, ctx, testDB)

		pool := testDB.Pool()

		// Create follow
		_, err := pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, followedUser)
		require.NoError(t, err)

		// Create content
		now := time.Now()
		activeContent := createTestContent(t, ctx, testDB, followedUser, "active", now)
		deletedContent := createTestContent(t, ctx, testDB, followedUser, "deleted", now.Add(-1*time.Minute))
		_ = deletedContent

		// Get feed
		repo := repository.NewFeedRepository()
		result, err := repo.GetFeed(ctx, pool, viewer, nil, 20)

		require.NoError(t, err)

		// Should only include active content
		contentIDs := make([]uuid.UUID, len(result.Items))
		for i, item := range result.Items {
			contentIDs[i] = item.ID
		}

		assert.Contains(t, contentIDs, activeContent)
		assert.Len(t, contentIDs, 1)
	})

	// F1-W1 — Regression: hidden (moderation-flagged) content must not
	// surface on the wire, even in default shadow mode. Prior to the
	// SQL filter addition this row would be selected and the enforce-
	// mode evaluator was the only thing dropping it.
	t.Run("excludes hidden content (F1-W1)", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()

		ctx := context.Background()

		viewer := createTestUser(t, ctx, testDB)
		followedUser := createTestUser(t, ctx, testDB)
		pool := testDB.Pool()

		_, err := pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, followedUser)
		require.NoError(t, err)

		now := time.Now()
		activeContent := createTestContent(t, ctx, testDB, followedUser, "active", now)

		// Insert a row with is_hidden=true directly (bypasses the
		// createTestContent helper's hardcoded false).
		hiddenID := uuid.New()
		_, err = pool.Exec(ctx, `
			INSERT INTO contents (id, author_id, status, caption, is_hidden, created_at, updated_at)
			VALUES ($1, $2, 'active', 'hidden content', true, $3, $3)
		`, hiddenID, followedUser, now.Add(-1*time.Minute))
		require.NoError(t, err)

		repo := repository.NewFeedRepository()
		result, err := repo.GetFeed(ctx, pool, viewer, nil, 20)
		require.NoError(t, err)

		contentIDs := make([]uuid.UUID, len(result.Items))
		for i, item := range result.Items {
			contentIDs[i] = item.ID
		}
		assert.Contains(t, contentIDs, activeContent)
		assert.NotContains(t, contentIDs, hiddenID, "hidden content must not reach the feed wire")
		assert.Len(t, contentIDs, 1)
	})

	// F1-W1 — Regression: soft-deleted content (deleted_at IS NOT NULL)
	// with status='active' must not surface. The status filter alone
	// did not catch this row because the application code clears
	// status to 'deleted' on hard delete but soft-delete via deleted_at
	// timestamp may co-exist with stale status='active'. The new SQL
	// filter aligns with /search/content (search_repository_impl.go:265).
	t.Run("excludes soft-deleted content (F1-W1)", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()

		ctx := context.Background()

		viewer := createTestUser(t, ctx, testDB)
		followedUser := createTestUser(t, ctx, testDB)
		pool := testDB.Pool()

		_, err := pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, followedUser)
		require.NoError(t, err)

		now := time.Now()
		activeContent := createTestContent(t, ctx, testDB, followedUser, "active", now)

		// Soft-delete a row: status='active' but deleted_at populated.
		softDeletedID := createTestContent(t, ctx, testDB, followedUser, "active", now.Add(-1*time.Minute))
		_, err = pool.Exec(ctx, `
			UPDATE contents SET deleted_at = NOW() WHERE id = $1
		`, softDeletedID)
		require.NoError(t, err)

		repo := repository.NewFeedRepository()
		result, err := repo.GetFeed(ctx, pool, viewer, nil, 20)
		require.NoError(t, err)

		contentIDs := make([]uuid.UUID, len(result.Items))
		for i, item := range result.Items {
			contentIDs[i] = item.ID
		}
		assert.Contains(t, contentIDs, activeContent)
		assert.NotContains(t, contentIDs, softDeletedID, "soft-deleted content must not reach the feed wire")
		assert.Len(t, contentIDs, 1)
	})

	t.Run("cursor-based pagination works", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()

		ctx := context.Background()

		// Create users
		viewer := createTestUser(t, ctx, testDB)
		followedUser := createTestUser(t, ctx, testDB)

		pool := testDB.Pool()

		// Create follow
		_, err := pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, followedUser)
		require.NoError(t, err)

		// Create content with different timestamps
		now := time.Now()
		content1 := createTestContent(t, ctx, testDB, followedUser, "active", now)
		content2 := createTestContent(t, ctx, testDB, followedUser, "active", now.Add(-1*time.Minute))
		content3 := createTestContent(t, ctx, testDB, followedUser, "active", now.Add(-2*time.Minute))
		content4 := createTestContent(t, ctx, testDB, followedUser, "active", now.Add(-3*time.Minute))
		content5 := createTestContent(t, ctx, testDB, followedUser, "active", now.Add(-4*time.Minute))

		repo := repository.NewFeedRepository()

		// First page - limit 3
		result1, err := repo.GetFeed(ctx, pool, viewer, nil, 3)
		require.NoError(t, err)
		require.Len(t, result1.Items, 3)
		assert.True(t, result1.HasMore)
		assert.NotNil(t, result1.NextCursor)

		// First page should have content1, content2, content3
		page1IDs := make([]uuid.UUID, len(result1.Items))
		for i, item := range result1.Items {
			page1IDs[i] = item.ID
		}
		assert.Contains(t, page1IDs, content1)
		assert.Contains(t, page1IDs, content2)
		assert.Contains(t, page1IDs, content3)
		assert.NotContains(t, page1IDs, content4)
		assert.NotContains(t, page1IDs, content5)

		// Second page with cursor
		result2, err := repo.GetFeed(ctx, pool, viewer, result1.NextCursor, 3)
		require.NoError(t, err)
		assert.Len(t, result2.Items, 2) // content4, content5

		page2IDs := make([]uuid.UUID, len(result2.Items))
		for i, item := range result2.Items {
			page2IDs[i] = item.ID
		}
		assert.Contains(t, page2IDs, content4)
		assert.Contains(t, page2IDs, content5)
		assert.False(t, result2.HasMore)
	})

	t.Run("limit cap enforced - max 50", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()

		ctx := context.Background()

		// Create users
		viewer := createTestUser(t, ctx, testDB)
		followedUser := createTestUser(t, ctx, testDB)

		pool := testDB.Pool()

		// Create follow
		_, err := pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, followedUser)
		require.NoError(t, err)

		// Create 60 content items — enough to satisfy any valid limit.
		now := time.Now()
		for i := 0; i < 60; i++ {
			createTestContent(t, ctx, testDB, followedUser, "active", now.Add(-time.Duration(i)*time.Minute))
		}

		repo := repository.NewFeedRepository()

		// Request the maximum valid limit (50). The HTTP binding
		// enforces max=50 at the boundary; values above 50 are
		// invalid and fall back to the default (20) in the repository.
		result, err := repo.GetFeed(ctx, pool, viewer, nil, 50)
		require.NoError(t, err)
		assert.Len(t, result.Items, 50)
	})

	t.Run("invalid limit defaults to 20", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()

		ctx := context.Background()

		// Create users
		viewer := createTestUser(t, ctx, testDB)
		followedUser := createTestUser(t, ctx, testDB)

		pool := testDB.Pool()

		// Create follow
		_, err := pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, followedUser)
		require.NoError(t, err)

		// Create 30 content items
		now := time.Now()
		for i := 0; i < 30; i++ {
			createTestContent(t, ctx, testDB, followedUser, "active", now.Add(-time.Duration(i)*time.Minute))
		}

		repo := repository.NewFeedRepository()

		// Request limit of 0 - should default to 20
		result, err := repo.GetFeed(ctx, pool, viewer, nil, 0)
		require.NoError(t, err)
		assert.Len(t, result.Items, 20)
	})

	t.Run("returns empty result when no follows", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()

		ctx := context.Background()

		// Create user
		viewer := createTestUser(t, ctx, testDB)

		repo := repository.NewFeedRepository()

		// Get feed without any follows
		result, err := repo.GetFeed(ctx, testDB.Pool(), viewer, nil, 20)

		require.NoError(t, err)
		assert.Len(t, result.Items, 0)
		assert.False(t, result.HasMore)
		assert.Nil(t, result.NextCursor)
	})

	t.Run("returns empty result when followed users have no content", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()

		ctx := context.Background()

		// Create users
		viewer := createTestUser(t, ctx, testDB)
		followedUser := createTestUser(t, ctx, testDB)

		pool := testDB.Pool()

		// Create follow but no content
		_, err := pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, followedUser)
		require.NoError(t, err)

		repo := repository.NewFeedRepository()

		// Get feed
		result, err := repo.GetFeed(ctx, pool, viewer, nil, 20)

		require.NoError(t, err)
		assert.Len(t, result.Items, 0)
		assert.False(t, result.HasMore)
	})
}

// TestFeedRepository_RepostGovernance verifies FIX-1: content-type reposts whose
// original content is hidden, deleted, or non-active must not appear in the feed.
// ForSale/auction/profile reposts are unaffected.
func TestFeedRepository_RepostGovernance(t *testing.T) {
	t.Run("repost of hidden original content excluded", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()
		ctx := context.Background()

		viewer := createTestUser(t, ctx, testDB)
		poster := createTestUser(t, ctx, testDB)
		pool := testDB.Pool()

		_, err := pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, poster)
		require.NoError(t, err)

		now := time.Now()

		// Create the original content that will be hidden (moderated)
		origID := uuid.New()
		_, err = pool.Exec(ctx, `
			INSERT INTO contents (id, author_id, status, caption, is_hidden, created_at, updated_at)
			VALUES ($1, $2, 'active', 'original', true, $3, $3)
		`, origID, poster, now.Add(-2*time.Minute))
		require.NoError(t, err)

		// Create a repost of the hidden original
		repostID := uuid.New()
		_ = fmt.Sprintf(`{"targetType":"content","targetId":"%s","preview":{"title":"original","isAvailable":true}}`, origID)
		_, err = pool.Exec(ctx, `
			INSERT INTO contents (id, author_id, status, caption, is_hidden, original_author_id, created_at, updated_at)
			VALUES ($1, $2, 'active', 'repost', false, $3, $4, $4)
		`, repostID, poster, poster, now.Add(-1*time.Minute))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `
			INSERT INTO content_resource_occurrences (
				content_id, actor_id, operation, content_source_id, created_at
			)
			VALUES ($1, $2, 'share_to_feed', $3, NOW())
		`, repostID, poster, origID)
		require.NoError(t, err)

		// Create a normal post that should appear
		normalID := createTestContent(t, ctx, testDB, poster, "active", now)

		repo := repository.NewFeedRepository()
		result, err := repo.GetFeed(ctx, pool, viewer, nil, 20)
		require.NoError(t, err)

		ids := make([]uuid.UUID, len(result.Items))
		for i, item := range result.Items {
			ids[i] = item.ID
		}
		assert.Contains(t, ids, normalID, "normal post must appear")
		assert.NotContains(t, ids, repostID, "repost of hidden original must be excluded")
		assert.NotContains(t, ids, origID, "hidden original must be excluded")
	})

	t.Run("repost of deleted original content excluded", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()
		ctx := context.Background()

		viewer := createTestUser(t, ctx, testDB)
		poster := createTestUser(t, ctx, testDB)
		pool := testDB.Pool()

		_, err := pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, poster)
		require.NoError(t, err)

		now := time.Now()

		// Create the original content and then soft-delete it
		origID := uuid.New()
		_, err = pool.Exec(ctx, `
			INSERT INTO contents (id, author_id, status, caption, is_hidden, deleted_at, created_at, updated_at)
			VALUES ($1, $2, 'deleted', 'original deleted', false, NOW(), $3, $3)
		`, origID, poster, now.Add(-2*time.Minute))
		require.NoError(t, err)

		// Create a repost pointing at the deleted original
		repostID := uuid.New()
		_ = fmt.Sprintf(`{"targetType":"content","targetId":"%s","preview":{"title":"original","isAvailable":true}}`, origID)
		_, err = pool.Exec(ctx, `
			INSERT INTO contents (id, author_id, status, caption, is_hidden, original_author_id, created_at, updated_at)
			VALUES ($1, $2, 'active', 'repost', false, $3, $4, $4)
		`, repostID, poster, poster, now.Add(-1*time.Minute))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `
			INSERT INTO content_resource_occurrences (
				content_id, actor_id, operation, content_source_id, created_at
			)
			VALUES ($1, $2, 'share_to_feed', $3, NOW())
		`, repostID, poster, origID)
		require.NoError(t, err)

		normalID := createTestContent(t, ctx, testDB, poster, "active", now)

		repo := repository.NewFeedRepository()
		result, err := repo.GetFeed(ctx, pool, viewer, nil, 20)
		require.NoError(t, err)

		ids := make([]uuid.UUID, len(result.Items))
		for i, item := range result.Items {
			ids[i] = item.ID
		}
		assert.Contains(t, ids, normalID)
		assert.NotContains(t, ids, repostID, "repost of deleted original must be excluded")
	})

	t.Run("repost of active original content included", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()
		ctx := context.Background()

		viewer := createTestUser(t, ctx, testDB)
		poster := createTestUser(t, ctx, testDB)
		pool := testDB.Pool()

		_, err := pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, poster)
		require.NoError(t, err)

		now := time.Now()

		// Active, visible original
		origID := uuid.New()
		_, err = pool.Exec(ctx, `
			INSERT INTO contents (id, author_id, status, caption, is_hidden, created_at, updated_at)
			VALUES ($1, $2, 'active', 'original active', false, $3, $3)
		`, origID, poster, now.Add(-2*time.Minute))
		require.NoError(t, err)

		// Repost of the active original — must appear
		repostID := uuid.New()
		_ = fmt.Sprintf(`{"targetType":"content","targetId":"%s","preview":{"title":"original","isAvailable":true}}`, origID)
		_, err = pool.Exec(ctx, `
			INSERT INTO contents (id, author_id, status, caption, is_hidden, original_author_id, created_at, updated_at)
			VALUES ($1, $2, 'active', 'repost', false, $3, $4, $4)
		`, repostID, poster, poster, now.Add(-1*time.Minute))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `
			INSERT INTO content_resource_occurrences (
				content_id, actor_id, operation, content_source_id, created_at
			)
			VALUES ($1, $2, 'share_to_feed', $3, NOW())
		`, repostID, poster, origID)
		require.NoError(t, err)

		repo := repository.NewFeedRepository()
		result, err := repo.GetFeed(ctx, pool, viewer, nil, 20)
		require.NoError(t, err)

		ids := make([]uuid.UUID, len(result.Items))
		for i, item := range result.Items {
			ids[i] = item.ID
		}
		assert.Contains(t, ids, repostID, "repost of active original must be included")
		assert.Contains(t, ids, origID, "active original must also be included")
	})

	t.Run("for_sale repost unaffected by content repost governance", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()
		ctx := context.Background()

		viewer := createTestUser(t, ctx, testDB)
		poster := createTestUser(t, ctx, testDB)
		pool := testDB.Pool()

		_, err := pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, poster)
		require.NoError(t, err)

		now := time.Now()

		// A repost of a for_sale (targetType='for_sale', not 'content') — must always pass through
		forSaleRepostID := uuid.New()
		fakeForSaleID := uuid.New()
		_ = fmt.Sprintf(`{"targetType":"for_sale","targetId":"%s","preview":{"title":"some for_sale","isAvailable":false,"isSold":true}}`, fakeForSaleID)
		_, err = pool.Exec(ctx, `
			INSERT INTO contents (id, author_id, status, caption, is_hidden, original_author_id, created_at, updated_at)
			VALUES ($1, $2, 'active', 'for_sale repost', false, $3, $4, $4)
		`, forSaleRepostID, poster, poster, now)
		require.NoError(t, err)

		repo := repository.NewFeedRepository()
		result, err := repo.GetFeed(ctx, pool, viewer, nil, 20)
		require.NoError(t, err)

		ids := make([]uuid.UUID, len(result.Items))
		for i, item := range result.Items {
			ids[i] = item.ID
		}
		assert.Contains(t, ids, forSaleRepostID, "for_sale repost must not be excluded by content repost governance")
	})
}

// TestFeedRepository_RepostTargetAuthorLifecycle verifies that reposts are
// excluded when the original target author is suspended, banned, or deleted,
// and still included when the original target author is active.
func TestFeedRepository_RepostTargetAuthorLifecycle(t *testing.T) {
	createUserWithStatus := func(t *testing.T, ctx context.Context, testDB *testdb.TestDB, status string, deleted bool) uuid.UUID {
		t.Helper()
		userID := uuid.New()
		pool := testDB.Pool()
		if deleted {
			_, err := pool.Exec(ctx, `
				INSERT INTO users (id, firebase_uid, email, email_verified_at, phone_verified, account_status, deleted_at, created_at, updated_at)
				VALUES ($1, $2, $3, NOW(), true, 'active', NOW(), NOW(), NOW())
			`, userID, userID.String(), userID.String()+"@test.com")
			require.NoError(t, err)
			return userID
		}

		_, err := pool.Exec(ctx, `
			INSERT INTO users (id, firebase_uid, email, email_verified_at, phone_verified, account_status, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), true, $4, NOW(), NOW())
		`, userID, userID.String(), userID.String()+"@test.com", status)
		require.NoError(t, err)
		return userID
	}

	t.Run("repost of suspended original author excluded", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()
		ctx := context.Background()
		pool := testDB.Pool()

		viewer := createTestUser(t, ctx, testDB)
		poster := createTestUser(t, ctx, testDB)
		originalAuthor := createUserWithStatus(t, ctx, testDB, "suspended", false)

		_, err := pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, poster)
		require.NoError(t, err)

		now := time.Now()
		origID := createTestContent(t, ctx, testDB, originalAuthor, "active", now.Add(-2*time.Minute))
		repostID := uuid.New()
		_ = fmt.Sprintf(`{"targetType":"content","targetId":"%s","preview":{"title":"original","isAvailable":true}}`, origID)
		_, err = pool.Exec(ctx, `
			INSERT INTO contents (id, author_id, status, caption, is_hidden, original_author_id, created_at, updated_at)
			VALUES ($1, $2, 'active', 'repost', false, $3, $4, $4)
		`, repostID, poster, originalAuthor, now.Add(-1*time.Minute))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `
			INSERT INTO content_resource_occurrences (
				content_id, actor_id, operation, content_source_id, created_at
			)
			VALUES ($1, $2, 'share_to_feed', $3, NOW())
		`, repostID, poster, origID)
		require.NoError(t, err)

		repo := repository.NewFeedRepository()
		result, err := repo.GetFeed(ctx, pool, viewer, nil, 20)
		require.NoError(t, err)

		ids := make([]uuid.UUID, len(result.Items))
		for i, item := range result.Items {
			ids[i] = item.ID
		}
		assert.NotContains(t, ids, repostID, "repost of suspended original author must be excluded")
	})

	t.Run("repost of banned original author excluded", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()
		ctx := context.Background()
		pool := testDB.Pool()

		viewer := createTestUser(t, ctx, testDB)
		poster := createTestUser(t, ctx, testDB)
		originalAuthor := createUserWithStatus(t, ctx, testDB, "banned", false)

		_, err := pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, poster)
		require.NoError(t, err)

		now := time.Now()
		origID := createTestContent(t, ctx, testDB, originalAuthor, "active", now.Add(-2*time.Minute))
		repostID := uuid.New()
		_ = fmt.Sprintf(`{"targetType":"content","targetId":"%s","preview":{"title":"original","isAvailable":true}}`, origID)
		_, err = pool.Exec(ctx, `
			INSERT INTO contents (id, author_id, status, caption, is_hidden, original_author_id, created_at, updated_at)
			VALUES ($1, $2, 'active', 'repost', false, $3, $4, $4)
		`, repostID, poster, originalAuthor, now.Add(-1*time.Minute))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `
			INSERT INTO content_resource_occurrences (
				content_id, actor_id, operation, content_source_id, created_at
			)
			VALUES ($1, $2, 'share_to_feed', $3, NOW())
		`, repostID, poster, origID)
		require.NoError(t, err)

		repo := repository.NewFeedRepository()
		result, err := repo.GetFeed(ctx, pool, viewer, nil, 20)
		require.NoError(t, err)

		ids := make([]uuid.UUID, len(result.Items))
		for i, item := range result.Items {
			ids[i] = item.ID
		}
		assert.NotContains(t, ids, repostID, "repost of banned original author must be excluded")
	})

	t.Run("repost of soft-deleted original author excluded", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()
		ctx := context.Background()
		pool := testDB.Pool()

		viewer := createTestUser(t, ctx, testDB)
		poster := createTestUser(t, ctx, testDB)
		originalAuthor := createUserWithStatus(t, ctx, testDB, "active", true)

		_, err := pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, poster)
		require.NoError(t, err)

		now := time.Now()
		origID := createTestContent(t, ctx, testDB, originalAuthor, "active", now.Add(-2*time.Minute))
		repostID := uuid.New()
		_ = fmt.Sprintf(`{"targetType":"content","targetId":"%s","preview":{"title":"original","isAvailable":true}}`, origID)
		_, err = pool.Exec(ctx, `
			INSERT INTO contents (id, author_id, status, caption, is_hidden, original_author_id, created_at, updated_at)
			VALUES ($1, $2, 'active', 'repost', false, $3, $4, $4)
		`, repostID, poster, originalAuthor, now.Add(-1*time.Minute))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `
			INSERT INTO content_resource_occurrences (
				content_id, actor_id, operation, content_source_id, created_at
			)
			VALUES ($1, $2, 'share_to_feed', $3, NOW())
		`, repostID, poster, origID)
		require.NoError(t, err)

		repo := repository.NewFeedRepository()
		result, err := repo.GetFeed(ctx, pool, viewer, nil, 20)
		require.NoError(t, err)

		ids := make([]uuid.UUID, len(result.Items))
		for i, item := range result.Items {
			ids[i] = item.ID
		}
		assert.NotContains(t, ids, repostID, "repost of soft-deleted original author must be excluded")
	})

	t.Run("repost of active original author included", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()
		ctx := context.Background()
		pool := testDB.Pool()

		viewer := createTestUser(t, ctx, testDB)
		poster := createTestUser(t, ctx, testDB)
		originalAuthor := createUserWithStatus(t, ctx, testDB, "active", false)

		_, err := pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, poster)
		require.NoError(t, err)

		now := time.Now()
		origID := createTestContent(t, ctx, testDB, originalAuthor, "active", now.Add(-2*time.Minute))
		repostID := uuid.New()
		_ = fmt.Sprintf(`{"targetType":"content","targetId":"%s","preview":{"title":"original","isAvailable":true}}`, origID)
		_, err = pool.Exec(ctx, `
			INSERT INTO contents (id, author_id, status, caption, is_hidden, original_author_id, created_at, updated_at)
			VALUES ($1, $2, 'active', 'repost', false, $3, $4, $4)
		`, repostID, poster, originalAuthor, now.Add(-1*time.Minute))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `
			INSERT INTO content_resource_occurrences (
				content_id, actor_id, operation, content_source_id, created_at
			)
			VALUES ($1, $2, 'share_to_feed', $3, NOW())
		`, repostID, poster, origID)
		require.NoError(t, err)

		repo := repository.NewFeedRepository()
		result, err := repo.GetFeed(ctx, pool, viewer, nil, 20)
		require.NoError(t, err)

		ids := make([]uuid.UUID, len(result.Items))
		for i, item := range result.Items {
			ids[i] = item.ID
		}
		assert.Contains(t, ids, repostID, "repost of active original author must remain visible")
	})

	t.Run("repost of active auction included", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()
		ctx := context.Background()
		pool := testDB.Pool()

		viewer := createTestUser(t, ctx, testDB)
		seller := createTestUser(t, ctx, testDB)

		_, err := pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, seller)
		require.NoError(t, err)

		now := time.Now()
		productID := createTestProduct(t, ctx, testDB, seller)
		auctionID := createTestAuction(
			t,
			ctx,
			testDB,
			seller,
			productID,
			"active",
			now.Add(-1*time.Hour),
			now.Add(1*time.Hour),
		)

		repostID := uuid.New()
		_ = fmt.Sprintf(
			`{"targetType":"auction","targetId":"%s","preview":{"title":"auction","isAvailable":true}}`,
			auctionID,
		)
		_, err = pool.Exec(ctx, `
			INSERT INTO contents (id, author_id, status, caption, is_hidden, original_author_id, created_at, updated_at)
			VALUES ($1, $2, 'active', 'auction repost', false, $3, $4, $4)
		`, repostID, seller, seller, now)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `
			INSERT INTO content_resource_occurrences (
				content_id, actor_id, operation, auction_source_id, created_at
			)
			VALUES ($1, $2, 'share_to_feed', $3, NOW())
		`, repostID, seller, auctionID)
		require.NoError(t, err)

		repo := repository.NewFeedRepository()
		result, err := repo.GetFeed(ctx, pool, viewer, nil, 20)
		require.NoError(t, err)

		ids := make([]uuid.UUID, len(result.Items))
		for i, item := range result.Items {
			ids[i] = item.ID
		}
		assert.Contains(t, ids, repostID, "repost of active auction must remain visible")
	})

	t.Run("repost of ended auction excluded", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()
		ctx := context.Background()
		pool := testDB.Pool()

		viewer := createTestUser(t, ctx, testDB)
		seller := createTestUser(t, ctx, testDB)

		_, err := pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, seller)
		require.NoError(t, err)

		now := time.Now()
		productID := createTestProduct(t, ctx, testDB, seller)
		auctionID := createTestAuction(
			t,
			ctx,
			testDB,
			seller,
			productID,
			"ended",
			now.Add(-2*time.Hour),
			now.Add(-1*time.Hour),
		)

		repostID := uuid.New()
		_ = fmt.Sprintf(
			`{"targetType":"auction","targetId":"%s","preview":{"title":"auction","isAvailable":false}}`,
			auctionID,
		)
		_, err = pool.Exec(ctx, `
			INSERT INTO contents (id, author_id, status, caption, is_hidden, original_author_id, created_at, updated_at)
			VALUES ($1, $2, 'active', 'ended auction repost', false, $3, $4, $4)
		`, repostID, seller, seller, now)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `
			INSERT INTO content_resource_occurrences (
				content_id, actor_id, operation, auction_source_id, created_at
			)
			VALUES ($1, $2, 'share_to_feed', $3, NOW())
		`, repostID, seller, auctionID)
		require.NoError(t, err)

		normalID := createTestContent(t, ctx, testDB, seller, "active", now.Add(-3*time.Hour))

		repo := repository.NewFeedRepository()
		result, err := repo.GetFeed(ctx, pool, viewer, nil, 20)
		require.NoError(t, err)

		ids := make([]uuid.UUID, len(result.Items))
		for i, item := range result.Items {
			ids[i] = item.ID
		}
		assert.Contains(t, ids, normalID, "normal content should remain visible")
		assert.NotContains(t, ids, repostID, "repost of ended auction must be excluded")
	})
}

// TestFeedRepository_AuthorLifecycle verifies F1-B1: content from suspended, banned, or
// soft-deleted authors must not appear in the feed, regardless of evaluator mode.
// These tests prove that canonical enforcement lives at the SQL/repository layer —
// EVALUATOR_SHADOW_FEED_ENABLED does not need to be true for safety to hold.
func TestFeedRepository_AuthorLifecycle(t *testing.T) {
	// createTestUserWithStatus creates a user with the given account_status.
	// Valid values: 'active', 'suspended', 'banned'.
	createTestUserWithStatus := func(t *testing.T, ctx context.Context, testDB *testdb.TestDB, status string) uuid.UUID {
		t.Helper()
		userID := uuid.New()
		pool := testDB.Pool()
		_, err := pool.Exec(ctx, `
			INSERT INTO users (id, firebase_uid, email, email_verified_at, phone_verified, account_status, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), true, $4, NOW(), NOW())
		`, userID, userID.String(), userID.String()+"@test.com", status)
		require.NoError(t, err)
		return userID
	}

	// createDeletedUser creates a user with deleted_at set (soft-deleted account).
	// account_status is 'active' — deletion is represented by deleted_at IS NOT NULL.
	createDeletedUser := func(t *testing.T, ctx context.Context, testDB *testdb.TestDB) uuid.UUID {
		t.Helper()
		userID := uuid.New()
		pool := testDB.Pool()
		_, err := pool.Exec(ctx, `
			INSERT INTO users (id, firebase_uid, email, email_verified_at, phone_verified, account_status, deleted_at, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), true, 'active', NOW(), NOW(), NOW())
		`, userID, userID.String(), userID.String()+"@test.com")
		require.NoError(t, err)
		return userID
	}

	t.Run("excludes content from suspended author (F1-B1)", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()
		ctx := context.Background()
		pool := testDB.Pool()

		viewer := createTestUser(t, ctx, testDB)
		activeAuthor := createTestUser(t, ctx, testDB)
		suspendedAuthor := createTestUserWithStatus(t, ctx, testDB, "suspended")

		// Both authors followed by viewer
		_, err := pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW()), ($1, $3, NOW())
		`, viewer, activeAuthor, suspendedAuthor)
		require.NoError(t, err)

		now := time.Now()
		activeContent := createTestContent(t, ctx, testDB, activeAuthor, "active", now)
		suspendedContent := createTestContent(t, ctx, testDB, suspendedAuthor, "active", now.Add(-1*time.Minute))

		repo := repository.NewFeedRepository()
		result, err := repo.GetFeed(ctx, pool, viewer, nil, 20)
		require.NoError(t, err)

		ids := make([]uuid.UUID, len(result.Items))
		for i, item := range result.Items {
			ids[i] = item.ID
		}
		assert.Contains(t, ids, activeContent, "active author content must appear")
		assert.NotContains(t, ids, suspendedContent, "suspended author content must be excluded (F1-B1)")
	})

	t.Run("excludes content from banned author (F1-B1)", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()
		ctx := context.Background()
		pool := testDB.Pool()

		viewer := createTestUser(t, ctx, testDB)
		activeAuthor := createTestUser(t, ctx, testDB)
		bannedAuthor := createTestUserWithStatus(t, ctx, testDB, "banned")

		_, err := pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW()), ($1, $3, NOW())
		`, viewer, activeAuthor, bannedAuthor)
		require.NoError(t, err)

		now := time.Now()
		activeContent := createTestContent(t, ctx, testDB, activeAuthor, "active", now)
		bannedContent := createTestContent(t, ctx, testDB, bannedAuthor, "active", now.Add(-1*time.Minute))

		repo := repository.NewFeedRepository()
		result, err := repo.GetFeed(ctx, pool, viewer, nil, 20)
		require.NoError(t, err)

		ids := make([]uuid.UUID, len(result.Items))
		for i, item := range result.Items {
			ids[i] = item.ID
		}
		assert.Contains(t, ids, activeContent, "active author content must appear")
		assert.NotContains(t, ids, bannedContent, "banned author content must be excluded (F1-B1)")
	})

	t.Run("excludes content from soft-deleted author (F1-B1)", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()
		ctx := context.Background()
		pool := testDB.Pool()

		viewer := createTestUser(t, ctx, testDB)
		activeAuthor := createTestUser(t, ctx, testDB)
		deletedAuthor := createDeletedUser(t, ctx, testDB)

		_, err := pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW()), ($1, $3, NOW())
		`, viewer, activeAuthor, deletedAuthor)
		require.NoError(t, err)

		now := time.Now()
		activeContent := createTestContent(t, ctx, testDB, activeAuthor, "active", now)
		deletedAuthorContent := createTestContent(t, ctx, testDB, deletedAuthor, "active", now.Add(-1*time.Minute))

		repo := repository.NewFeedRepository()
		result, err := repo.GetFeed(ctx, pool, viewer, nil, 20)
		require.NoError(t, err)

		ids := make([]uuid.UUID, len(result.Items))
		for i, item := range result.Items {
			ids[i] = item.ID
		}
		assert.Contains(t, ids, activeContent, "active author content must appear")
		assert.NotContains(t, ids, deletedAuthorContent, "deleted author content must be excluded (F1-B1)")
	})

	t.Run("includes content from active author (F1-B1 negative case)", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()
		ctx := context.Background()
		pool := testDB.Pool()

		viewer := createTestUser(t, ctx, testDB)
		author1 := createTestUser(t, ctx, testDB)
		author2 := createTestUser(t, ctx, testDB)

		_, err := pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW()), ($1, $3, NOW())
		`, viewer, author1, author2)
		require.NoError(t, err)

		now := time.Now()
		c1 := createTestContent(t, ctx, testDB, author1, "active", now)
		c2 := createTestContent(t, ctx, testDB, author2, "active", now.Add(-1*time.Minute))

		repo := repository.NewFeedRepository()
		result, err := repo.GetFeed(ctx, pool, viewer, nil, 20)
		require.NoError(t, err)

		ids := make([]uuid.UUID, len(result.Items))
		for i, item := range result.Items {
			ids[i] = item.ID
		}
		assert.Contains(t, ids, c1, "active author 1 content must appear")
		assert.Contains(t, ids, c2, "active author 2 content must appear")
		assert.Len(t, ids, 2, "exactly two items from two active authors")
	})
}

// TestFeedRepository_NoOffset verifies no OFFSET is used in the query.
func TestFeedRepository_NoOffset(t *testing.T) {
	t.Run("cursor pagination does not use OFFSET", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()

		ctx := context.Background()

		// Create users
		viewer := createTestUser(t, ctx, testDB)
		followedUser := createTestUser(t, ctx, testDB)

		pool := testDB.Pool()

		// Create follow
		_, err := pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, followedUser)
		require.NoError(t, err)

		// Create content
		now := time.Now()
		for i := 0; i < 5; i++ {
			createTestContent(t, ctx, testDB, followedUser, "active", now.Add(-time.Duration(i)*time.Minute))
		}

		repo := repository.NewFeedRepository()

		// Get multiple pages - verify cursor pagination works
		result1, err := repo.GetFeed(ctx, pool, viewer, nil, 2)
		require.NoError(t, err)
		assert.Len(t, result1.Items, 2)

		result2, err := repo.GetFeed(ctx, pool, viewer, result1.NextCursor, 2)
		require.NoError(t, err)
		assert.Len(t, result2.Items, 2)

		result3, err := repo.GetFeed(ctx, pool, viewer, result2.NextCursor, 2)
		require.NoError(t, err)
		assert.Len(t, result3.Items, 1) // Last item

		// Verify no duplicates across pages
		allIDs := make([]uuid.UUID, 0)
		for _, item := range result1.Items {
			allIDs = append(allIDs, item.ID)
		}
		for _, item := range result2.Items {
			allIDs = append(allIDs, item.ID)
		}
		for _, item := range result3.Items {
			allIDs = append(allIDs, item.ID)
		}

		// Check for duplicates
		uniqueIDs := make(map[uuid.UUID]bool)
		for _, id := range allIDs {
			if uniqueIDs[id] {
				t.Errorf("Duplicate content ID found across pages: %s", id)
			}
			uniqueIDs[id] = true
		}

		assert.Len(t, uniqueIDs, 5)
	})
}

// TestFeedRepository_CompositeCursor exercises the (created_at, id)
// composite cursor + LIMIT+1 honesty introduced in Batch 3I.
func TestFeedRepository_CompositeCursor(t *testing.T) {
	// Equal-created_at rows must paginate without skip or duplicate
	// across page boundaries. Pre-3I behaviour (`created_at < cursor`,
	// no secondary key) silently skipped the tied peer.
	t.Run("equal created_at rows paginate without skip or duplicate", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()

		ctx := context.Background()

		viewer := createTestUser(t, ctx, testDB)
		followedUser := createTestUser(t, ctx, testDB)

		pool := testDB.Pool()

		_, err := pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, followedUser)
		require.NoError(t, err)

		// Three contents at the SAME created_at timestamp — the
		// pathology that previously caused skip/duplicate at the page
		// boundary.
		sharedTS := time.Now().Add(-1 * time.Hour)
		c1 := createTestContent(t, ctx, testDB, followedUser, "active", sharedTS)
		c2 := createTestContent(t, ctx, testDB, followedUser, "active", sharedTS)
		c3 := createTestContent(t, ctx, testDB, followedUser, "active", sharedTS)

		repo := repository.NewFeedRepository()

		// Page 1 — limit 2 against 3 tied rows.
		page1, err := repo.GetFeed(ctx, pool, viewer, nil, 2)
		require.NoError(t, err)
		require.Len(t, page1.Items, 2)
		assert.True(t, page1.HasMore, "page1 hasMore must be true (third tied row remains)")
		require.NotNil(t, page1.NextCursor, "page1 nextCursor must be set when hasMore=true")
		lastReturned := page1.Items[len(page1.Items)-1]
		assert.Equal(t, lastReturned.CreatedAt.UTC(), page1.NextCursor.CreatedAt.UTC(),
			"nextCursor.CreatedAt must point at the last returned row")

		// Page 2 — must return the tied peer that was NOT returned on
		// page 1, never one of the page-1 rows.
		page2, err := repo.GetFeed(ctx, pool, viewer, page1.NextCursor, 2)
		require.NoError(t, err)
		require.Len(t, page2.Items, 1, "page2 must return the third tied row")
		assert.False(t, page2.HasMore, "page2 hasMore must be false (only one tied row left)")
		assert.Nil(t, page2.NextCursor, "terminal page must emit nil cursor")

		// Aggregate: every tied row appears exactly once.
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

	// LIMIT+1 honesty: HasMore must be derived from a probe, not from
	// `len(items) == limit`. With exactly `limit` rows available, the
	// old heuristic returned has_more=true; the new contract returns
	// has_more=false because the probe returns no extra row.
	t.Run("has_more false when exactly limit rows remain", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()

		ctx := context.Background()

		viewer := createTestUser(t, ctx, testDB)
		followedUser := createTestUser(t, ctx, testDB)

		pool := testDB.Pool()
		_, err := pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, followedUser)
		require.NoError(t, err)

		// Exactly 2 contents, request limit=2.
		now := time.Now()
		createTestContent(t, ctx, testDB, followedUser, "active", now)
		createTestContent(t, ctx, testDB, followedUser, "active", now.Add(-1*time.Minute))

		repo := repository.NewFeedRepository()
		page, err := repo.GetFeed(ctx, pool, viewer, nil, 2)
		require.NoError(t, err)
		require.Len(t, page.Items, 2)
		assert.False(t, page.HasMore, "has_more must be false when exactly limit rows remain")
		assert.Nil(t, page.NextCursor, "next cursor must be nil on terminal page")
	})

	// Symmetry assertion: NextCursor must be non-nil iff HasMore=true,
	// and the cursor's tuple must match the last returned row exactly.
	t.Run("next_cursor symmetry with has_more", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()

		ctx := context.Background()

		viewer := createTestUser(t, ctx, testDB)
		followedUser := createTestUser(t, ctx, testDB)

		pool := testDB.Pool()
		_, err := pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, followedUser)
		require.NoError(t, err)

		now := time.Now()
		for i := 0; i < 4; i++ {
			createTestContent(t, ctx, testDB, followedUser, "active", now.Add(-time.Duration(i)*time.Minute))
		}

		repo := repository.NewFeedRepository()
		page, err := repo.GetFeed(ctx, pool, viewer, nil, 2)
		require.NoError(t, err)
		require.True(t, page.HasMore)
		require.NotNil(t, page.NextCursor)

		lastReturned := page.Items[len(page.Items)-1]
		assert.Equal(t, lastReturned.ID, page.NextCursor.ID,
			"NextCursor.ID must equal the last returned row's id")
		assert.Equal(t, lastReturned.CreatedAt.UTC(), page.NextCursor.CreatedAt.UTC(),
			"NextCursor.CreatedAt must equal the last returned row's created_at")
	})

	// Type roundtrip — a FeedCursor produced by one page must feed
	// back into the next call unchanged. Guards against accidental
	// shape drift between repository and codec.
	t.Run("cursor roundtrip via encode/decode preserves identity", func(t *testing.T) {
		testDB, cleanup := setupTestDB(t)
		defer cleanup()

		ctx := context.Background()

		viewer := createTestUser(t, ctx, testDB)
		followedUser := createTestUser(t, ctx, testDB)

		pool := testDB.Pool()
		_, err := pool.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1, $2, NOW())
		`, viewer, followedUser)
		require.NoError(t, err)

		now := time.Now()
		for i := 0; i < 3; i++ {
			createTestContent(t, ctx, testDB, followedUser, "active", now.Add(-time.Duration(i)*time.Minute))
		}

		repo := repository.NewFeedRepository()
		page1, err := repo.GetFeed(ctx, pool, viewer, nil, 2)
		require.NoError(t, err)
		require.NotNil(t, page1.NextCursor)

		encoded := feedentity.EncodeFeedCursor(page1.NextCursor)
		require.NotEmpty(t, encoded, "encoded cursor must be non-empty on a non-terminal page")

		decoded, err := feedentity.DecodeFeedCursor(encoded)
		require.NoError(t, err)
		require.NotNil(t, decoded)

		page2, err := repo.GetFeed(ctx, pool, viewer, decoded, 2)
		require.NoError(t, err)
		require.Len(t, page2.Items, 1)
		assert.False(t, page2.HasMore)
		assert.Nil(t, page2.NextCursor)
	})
}
