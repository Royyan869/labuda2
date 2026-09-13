//go:build integration

package repository

// Real-PostgreSQL proof for the canonical Social Content access authority at
// the search surface.
//
// Canonical authority under proof:
//   - contents.visibility is the ONLY business authority for who may access a
//     content (public / followers_only / private).
//   - search is a public discovery surface and therefore narrows to
//     visibility = 'public' — it never leaks followers_only or private rows,
//     not even when the row is not hidden.
//   - contents.is_hidden is moderation authority, not visibility: a public but
//     hidden row stays out of search.
//   - lifecycle authority (status = 'active' AND deleted_at IS NULL) plus
//     author lifecycle (active, not soft-deleted) are secondary constraints.
//
// Rows are seeded as real rows and the assertions run against real SQL
// idempotency/lifecycle behaviour, not mocks.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/internal/discovery/search/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

func seedSearchVisibilityAuthor(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	accountStatus string,
	softDeleted bool,
) uuid.UUID {
	t.Helper()

	userID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, account_status, deleted_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, CASE WHEN $5 THEN NOW() ELSE NULL END, NOW(), NOW())
	`, userID, "fb-"+userID.String(), userID.String()+"@test.invalid", accountStatus, softDeleted)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO user_profiles (id, user_id, username, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
	`, uuid.New(), userID, "search-"+strings.ReplaceAll(userID.String(), "-", ""))
	require.NoError(t, err)

	return userID
}

func seedSearchVisibilityContent(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	authorID uuid.UUID,
	visibility string,
	isHidden bool,
	status string,
	softDeleted bool,
	caption string,
) uuid.UUID {
	t.Helper()

	contentID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO contents (
			id, author_id, status, caption, visibility, is_hidden, deleted_at, created_at, updated_at
		)
		VALUES (
			$1, $2, $3::content_status_enum, $4, $5::content_visibility_enum, $6,
			CASE WHEN $7 THEN NOW() ELSE NULL END, NOW(), NOW()
		)
	`, contentID, authorID, status, caption, visibility, isHidden, softDeleted)
	require.NoError(t, err)

	return contentID
}

// TestSearchContent_RealDB_PublicOnlyNarrowing proves with real rows that the
// search reader surfaces exactly the public + visible + active-author content
// and nothing else.
func TestSearchContent_RealDB_PublicOnlyNarrowing(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	repo := NewSearchRepository()

	activeAuthor := seedSearchVisibilityAuthor(t, ctx, tdb.Pool(), "active", false)
	suspendedAuthor := seedSearchVisibilityAuthor(t, ctx, tdb.Pool(), "suspended", false)
	softDeletedAuthor := seedSearchVisibilityAuthor(t, ctx, tdb.Pool(), "active", true)

	// The only row that may appear on the public search surface.
	publicVisible := seedSearchVisibilityContent(
		t, ctx, tdb.Pool(), activeAuthor, "public", false, "active", false, "public visible",
	)
	// Visibility narrowing — excluded even though not hidden.
	seedSearchVisibilityContent(
		t, ctx, tdb.Pool(), activeAuthor, "followers_only", false, "active", false, "followers only",
	)
	seedSearchVisibilityContent(
		t, ctx, tdb.Pool(), activeAuthor, "private", false, "active", false, "private visible",
	)
	// Moderation narrowing — excluded even though public.
	seedSearchVisibilityContent(
		t, ctx, tdb.Pool(), activeAuthor, "public", true, "active", false, "public hidden",
	)
	// Lifecycle narrowing.
	seedSearchVisibilityContent(
		t, ctx, tdb.Pool(), activeAuthor, "public", false, "deleted", false, "public status deleted",
	)
	seedSearchVisibilityContent(
		t, ctx, tdb.Pool(), activeAuthor, "public", false, "active", true, "public deleted_at",
	)
	// Author lifecycle narrowing.
	seedSearchVisibilityContent(
		t, ctx, tdb.Pool(), suspendedAuthor, "public", false, "active", false, "suspended author",
	)
	seedSearchVisibilityContent(
		t, ctx, tdb.Pool(), softDeletedAuthor, "public", false, "active", false, "soft-deleted author",
	)

	var got []*entity.ContentPreview
	var total int
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		var searchErr error
		got, total, searchErr = repo.SearchContent(ctx, tx, entity.SearchFilters{Limit: 50})
		return searchErr
	})
	require.NoError(t, err)

	ids := make([]uuid.UUID, 0, len(got))
	for _, preview := range got {
		ids = append(ids, preview.ID)
	}

	require.Equal(t, []uuid.UUID{publicVisible}, ids,
		"search content must surface exactly the public + visible + active-author row, got %s",
		fmt.Sprint(ids))
	require.Equal(t, 1, total,
		"search content total must mirror the same visibility/lifecycle authority")
}
