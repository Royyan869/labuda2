//go:build integration

package application_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	contentapp "github.com/labuda/backend/internal/social/content/application"
	contenthttp "github.com/labuda/backend/internal/social/content/delivery/http"
	contententity "github.com/labuda/backend/internal/social/content/entity"
	contentrepo "github.com/labuda/backend/internal/social/content/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

type visibilityAccountChecker struct{}

func (visibilityAccountChecker) EnsureActive(ctx context.Context, userID uuid.UUID) error {
	return nil
}

func (visibilityAccountChecker) GetStatus(ctx context.Context, userID uuid.UUID) (string, error) {
	return "active", nil
}

func (visibilityAccountChecker) IsBanned(ctx context.Context, userID uuid.UUID) (bool, error) {
	return false, nil
}

type visibilityRoleChecker struct{}

func (visibilityRoleChecker) IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error) {
	return false, nil
}

func (visibilityRoleChecker) HasActiveSellerCapability(ctx context.Context, userID uuid.UUID) (bool, error) {
	return false, nil
}

func (visibilityRoleChecker) HasSellerProfile(ctx context.Context, userID uuid.UUID) (bool, error) {
	return false, nil
}

func newVisibilityService() *contentapp.ContentService {
	return contentapp.NewContentService(
		contentrepo.NewContentRepository(),
		nil,
		visibilityRoleChecker{},
		visibilityAccountChecker{},
		nil,
	)
}

func seedVisibilityUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, status string) uuid.UUID {
	t.Helper()
	userID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, email_verified_at, phone_verified, account_status, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), true, $4, NOW(), NOW())
	`, userID, userID.String(), fmt.Sprintf("%s@test.invalid", userID), status)
	if err == nil {
		_, err = pool.Exec(ctx, `
			INSERT INTO user_profiles (id, user_id, username, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), NOW())
		`, uuid.New(), userID, "user-"+strings.ReplaceAll(userID.String(), "-", ""))
	}
	require.NoError(t, err)
	return userID
}

func seedVisibilityFollow(t *testing.T, ctx context.Context, pool *pgxpool.Pool, followerID, followingID uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(ctx, `
		INSERT INTO user_follows (follower_id, following_id, created_at)
		VALUES ($1, $2, NOW())
	`, followerID, followingID)
	require.NoError(t, err)
}

func seedVisibilityContent(
	t *testing.T,
	ctx context.Context,
	tx db.Tx,
	authorID uuid.UUID,
	visibility contententity.Visibility,
	isHidden bool,
	caption string,
) uuid.UUID {
	t.Helper()
	contentID := uuid.New()
	_, err := tx.Exec(ctx, `
		INSERT INTO contents (
			id, author_id,status, caption,
			visibility, is_hidden, created_at, updated_at
		)
		VALUES ($1, $2, 'active', $3, $4, $5, NOW(), NOW())
	`, contentID, authorID, caption, string(visibility), isHidden)
	require.NoError(t, err)
	return contentID
}

func TestCreateContent_PersistsVisibilityAndDefaultsPublic(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	service := newVisibilityService()
	authorID := seedVisibilityUser(t, ctx, tdb.Pool(), "active")

	cases := []struct {
		name              string
		requestVisibility contententity.Visibility
		wantVisibility    contententity.Visibility
		explicit          bool
	}{
		{name: "public", requestVisibility: contententity.VisibilityPublic, wantVisibility: contententity.VisibilityPublic, explicit: true},
		{name: "followers_only", requestVisibility: contententity.VisibilityFollowersOnly, wantVisibility: contententity.VisibilityFollowersOnly, explicit: true},
		{name: "private", requestVisibility: contententity.VisibilityPrivate, wantVisibility: contententity.VisibilityPrivate, explicit: true},
		{name: "omitted defaults to public", requestVisibility: "", wantVisibility: contententity.VisibilityPublic, explicit: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var createdID uuid.UUID
			err := tdb.WithTx(ctx, func(tx db.Tx) error {
				visibility := tc.requestVisibility
				if !tc.explicit {
					visibility = ""
				}
				content, createErr := service.CreateContent(
					ctx,
					tx,
					authorID,
					"visibility test",
					visibility,
					nil,
					nil,
					nil,
					nil,
					nil,
				)
				if createErr != nil {
					return createErr
				}
				createdID = content.ID
				require.Equal(t, string(tc.wantVisibility), string(content.Visibility))

				resp := contenthttp.ToContentResponse(content, nil)
				require.Equal(t, string(tc.wantVisibility), string(resp.Visibility))
				return nil
			})
			require.NoError(t, err)

			var storedVisibility string
			var isHidden bool
			require.NoError(t, tdb.Pool().QueryRow(ctx, `
				SELECT visibility, is_hidden
				FROM contents
				WHERE id = $1
			`, createdID).Scan(&storedVisibility, &isHidden))
			require.Equal(t, string(tc.wantVisibility), storedVisibility)
			require.False(t, isHidden)
		})
	}
}

func TestUpdateCaptionAndVisibility_TransitionsVisibilityWithoutTouchingIsHidden(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	service := newVisibilityService()
	authorID := seedVisibilityUser(t, ctx, tdb.Pool(), "active")

	cases := []struct {
		name          string
		from          contententity.Visibility
		to            contententity.Visibility
		initialHidden bool
	}{
		{name: "public to followers_only", from: contententity.VisibilityPublic, to: contententity.VisibilityFollowersOnly},
		{name: "followers_only to private", from: contententity.VisibilityFollowersOnly, to: contententity.VisibilityPrivate},
		{name: "private to public", from: contententity.VisibilityPrivate, to: contententity.VisibilityPublic},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var contentID uuid.UUID
			err := tdb.WithTx(ctx, func(tx db.Tx) error {
				content, createErr := service.CreateContent(
					ctx,
					tx,
					authorID,
					"update visibility",
					tc.from,
					nil,
					nil,
					nil,
					nil,
					nil,
				)
				if createErr != nil {
					return createErr
				}
				contentID = content.ID

				visibility := string(tc.to)
				if updateErr := service.UpdateCaptionAndVisibility(ctx, tx, authorID, contentID, nil, &visibility); updateErr != nil {
					return updateErr
				}

				updated, loadErr := contentrepo.NewContentRepository().GetByID(ctx, tx, contentID)
				if loadErr != nil {
					return loadErr
				}
				require.Equal(t, string(tc.to), string(updated.Visibility))
				require.Equal(t, tc.to == contententity.VisibilityPrivate, updated.IsHidden)

				resp := contenthttp.ToContentResponse(updated, nil)
				require.Equal(t, string(tc.to), string(resp.Visibility))
				return nil
			})
			require.NoError(t, err)

			var storedVisibility string
			var isHidden bool
			require.NoError(t, tdb.Pool().QueryRow(ctx, `
				SELECT visibility, is_hidden
				FROM contents
				WHERE id = $1
			`, contentID).Scan(&storedVisibility, &isHidden))
			require.Equal(t, string(tc.to), storedVisibility)
			require.Equal(t, tc.to == contententity.VisibilityPrivate, isHidden)
		})
	}
}

func TestListByAuthor_RespectsVisibilityAndExcludesHiddenAndDeleted(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	service := newVisibilityService()

	authorID := seedVisibilityUser(t, ctx, tdb.Pool(), "active")
	base := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	mustInsert := func(tx db.Tx, visibility contententity.Visibility, isHidden bool, status string, createdAt time.Time, caption string) uuid.UUID {
		t.Helper()
		contentID := uuid.New()
		_, err := tx.Exec(ctx, `
			INSERT INTO contents (
				id, author_id,status, caption,
				visibility, is_hidden, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		`, contentID, authorID, status, caption, string(visibility), isHidden, createdAt)
		require.NoError(t, err)
		return contentID
	}

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		publicID := mustInsert(tx, contententity.VisibilityPublic, false, "active", base, "public")
		followersOnlyID := mustInsert(tx, contententity.VisibilityFollowersOnly, false, "active", base.Add(-1*time.Minute), "followers")
		privateID := mustInsert(tx, contententity.VisibilityPrivate, false, "active", base.Add(-2*time.Minute), "private")
		_ = mustInsert(tx, contententity.VisibilityPublic, true, "active", base.Add(-3*time.Minute), "hidden")
		_ = mustInsert(tx, contententity.VisibilityPublic, false, "deleted", base.Add(-4*time.Minute), "deleted")

		got, nextCursor, err := service.ListByAuthor(ctx, tx, authorID, 20, "")
		require.NoError(t, err)
		require.Empty(t, nextCursor)
		require.Len(t, got, 3)
		require.Equal(t, publicID, got[0].ID)
		require.Equal(t, followersOnlyID, got[1].ID)
		require.Equal(t, privateID, got[2].ID)

		return nil
	})
	require.NoError(t, err)
}
