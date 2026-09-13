//go:build integration

package repository_test

// Real-PostgreSQL proof for the canonical Social Content access authority at
// the feed surface. One schema bootstrap, one transaction, real rows.
//
// Canonical authority under proof:
//   - contents.visibility decides who may reach a row: owner always,
//     follower for followers_only, everyone for public.
//   - is_hidden, status/deleted_at and author lifecycle are secondary
//     constraints that remove an otherwise reachable row.
//   - Block parity is bidirectional: viewer blocks author OR author blocks
//     viewer both exclude.
//   - Mute is DIRECTIONAL and feed-only: viewer mutes author excludes, while
//     the author muting the viewer has no effect on the viewer's feed.
//
// The feed is the only surface audited here; mute never denies access on the
// detail/comment/search surfaces (proved by the absence of any user_mutes
// reference in those readers).

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/internal/social/feed/infrastructure/repository"
	"github.com/labuda/backend/pkg/testdb"
)

func TestFeedAccessVisibilityMatrix_RealDB(t *testing.T) {
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()

	tx, err := testDB.Pool().Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	viewer := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
	followedAuthor := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
	blockedByViewer := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
	blockingViewer := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
	mutedByViewer := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
	mutingViewer := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
	strangerAuthor := createFeedTestUserWithStatusTx(t, ctx, tx, "active", false)
	suspendedAuthor := createFeedTestUserWithStatusTx(t, ctx, tx, "suspended", false)
	deletedAuthor := createDeletedFeedTestUserTx(t, ctx, tx)

	// The viewer follows every author except strangerAuthor, so the followed
	// set is the only place followers_only becomes reachable.
	_, err = tx.Exec(ctx, `
		INSERT INTO user_follows (follower_id, following_id, created_at)
		VALUES ($1, $2, NOW()), ($1, $3, NOW()), ($1, $4, NOW()), ($1, $5, NOW()),
		       ($1, $6, NOW()), ($1, $7, NOW()), ($1, $8, NOW())
	`, viewer, followedAuthor, blockedByViewer, blockingViewer, mutedByViewer, mutingViewer, suspendedAuthor, deletedAuthor)
	require.NoError(t, err)

	// Block parity — both directions must exclude.
	_, err = tx.Exec(ctx, `
		INSERT INTO user_blocks (blocker_id, blocked_id, created_at)
		VALUES ($1, $2, NOW()), ($3, $1, NOW())
	`, viewer, blockedByViewer, blockingViewer)
	require.NoError(t, err)

	// Mute is directional — only the viewer's own mute may suppress.
	_, err = tx.Exec(ctx, `
		INSERT INTO user_mutes (muter_id, muted_id, created_at)
		VALUES ($1, $2, NOW()), ($3, $1, NOW())
	`, viewer, mutedByViewer, mutingViewer)
	require.NoError(t, err)

	base := time.Date(2099, 1, 1, 12, 0, 0, 0, time.UTC)
	at := func(minutes int) time.Time { return base.Add(-time.Duration(minutes) * time.Minute) }

	// Reachable rows.
	followedPublic := createFeedTestContentTxWithVisibility(t, ctx, tx, followedAuthor, "active", "public", false, at(0))
	followedFollowersOnly := createFeedTestContentTxWithVisibility(t, ctx, tx, followedAuthor, "active", "followers_only", false, at(1))
	strangerPublicDiscovery := createFeedTestContentTxWithVisibility(t, ctx, tx, strangerAuthor, "active", "public", false, at(2))
	mutingViewerContent := createFeedTestContentTxWithVisibility(t, ctx, tx, mutingViewer, "active", "public", false, at(3))

	// Visibility denials.
	followedPrivate := createFeedTestContentTxWithVisibility(t, ctx, tx, followedAuthor, "active", "private", false, at(4))
	strangerFollowersOnly := createFeedTestContentTxWithVisibility(t, ctx, tx, strangerAuthor, "active", "followers_only", false, at(5))

	// Secondary-constraint denials.
	followedHidden := createFeedTestContentTxWithVisibility(t, ctx, tx, followedAuthor, "active", "public", true, at(6))
	followedStatusDeleted := createFeedTestContentTxWithVisibility(t, ctx, tx, followedAuthor, "deleted", "public", false, at(7))
	followedSoftDeletedContent := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO contents (id, author_id, status, caption, visibility, is_hidden, deleted_at, created_at, updated_at)
		VALUES ($1, $2, 'active', 'soft deleted', 'public', false, NOW(), $3, $3)
	`, followedSoftDeletedContent, followedAuthor, at(8))
	require.NoError(t, err)

	// Relationship denials.
	blockedByViewerContent := createFeedTestContentTxWithVisibility(t, ctx, tx, blockedByViewer, "active", "public", false, at(9))
	blockingViewerContent := createFeedTestContentTxWithVisibility(t, ctx, tx, blockingViewer, "active", "public", false, at(10))
	mutedContent := createFeedTestContentTxWithVisibility(t, ctx, tx, mutedByViewer, "active", "public", false, at(11))

	// Author lifecycle denials.
	suspendedContent := createFeedTestContentTxWithVisibility(t, ctx, tx, suspendedAuthor, "active", "public", false, at(12))
	deletedAuthorContent := createFeedTestContentTxWithVisibility(t, ctx, tx, deletedAuthor, "active", "public", false, at(13))

	repo := repository.NewFeedRepository()
	result, err := repo.GetFeed(ctx, tx, viewer, nil, 50)
	require.NoError(t, err)

	ids := feedItemIDs(result.Items)

	// Followed rows rank above discovery rows; inside a group, newest first.
	assert.Equal(t, []uuid.UUID{
		followedPublic,
		followedFollowersOnly,
		mutingViewerContent,
		strangerPublicDiscovery,
	}, ids)

	for name, excluded := range map[string]uuid.UUID{
		"followers_only author the viewer does not follow": strangerFollowersOnly,
		"private row owned by a followed author":           followedPrivate,
		"hidden row":                                       followedHidden,
		"status-deleted row":                               followedStatusDeleted,
		"deleted_at row":                                   followedSoftDeletedContent,
		"author blocked by the viewer":                     blockedByViewerContent,
		"author who blocks the viewer":                     blockingViewerContent,
		"author muted by the viewer":                       mutedContent,
		"suspended author":                                 suspendedContent,
		"soft-deleted author":                              deletedAuthorContent,
	} {
		assert.NotContains(t, ids, excluded, "feed must exclude %s", name)
	}
}
