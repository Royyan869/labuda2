package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/internal/social/feed/infrastructure/repository"
	"github.com/labuda/backend/pkg/testdb"
)

func TestFeedRepository_CanonicalContentQuerySmoke(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	pool := tdb.Pool()

	viewerID := uuid.New()
	authorID := uuid.New()
	now := time.Date(2026, time.August, 11, 9, 30, 0, 0, time.UTC)

	_, err := pool.Exec(ctx, `
		INSERT INTO users (
			id, firebase_uid, email, email_verified_at, phone_verified, account_status, created_at, updated_at
		)
		VALUES
			($1, $2, $3, NOW(), true, 'active', NOW(), NOW()),
			($4, $5, $6, NOW(), true, 'active', NOW(), NOW())
	`, viewerID, viewerID.String(), viewerID.String()+"@example.com", authorID, authorID.String(), authorID.String()+"@example.com")
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO user_profiles (id, user_id, username, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW()), ($4, $5, $6, NOW(), NOW())
	`, uuid.New(), viewerID, "viewer", uuid.New(), authorID, "author")
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO user_follows (follower_id, following_id, created_at)
		VALUES ($1, $2, NOW())
	`, viewerID, authorID)
	require.NoError(t, err)

	contentID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO contents (
			id, author_id, status, caption, visibility, is_hidden, created_at, updated_at
		)
		VALUES ($1, $2, 'active', $3, 'public', false, $4, $4)
	`, contentID, authorID, "feed smoke content", now)
	require.NoError(t, err)

	repo := repository.NewFeedRepository()
	result, err := repo.GetFeed(ctx, pool, viewerID, nil, 20)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Items, 1)
	require.Equal(t, contentID, result.Items[0].ID)
	require.Equal(t, "active", result.Items[0].Status)
	require.NotNil(t, result.Items[0].Caption)
	require.Equal(t, "feed smoke content", *result.Items[0].Caption)
	require.NotNil(t, result.Items[0].AuthorUsername)
	require.Equal(t, "author", *result.Items[0].AuthorUsername)
}
