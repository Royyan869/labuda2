//go:build integration

package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	socialApp "github.com/labuda/backend/internal/social/graph/application"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mockOutbox is a no-op OutboxInserter for handler-level integration tests.
type mockOutbox struct{}

func (m *mockOutbox) InsertTx(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error {
	return nil
}

// =============================================================================
// C2E2A — Follow handler integration tests
// =============================================================================

func newFollowTestEnv(t *testing.T) (*testdb.TestDB, *FollowHandler, func()) {
	t.Helper()
	testDB, cleanup := testdb.SetupDB(t)
	svc := socialApp.NewSocialServiceWithDefaults(db.NewFromPool(testDB.Pool()), &mockOutbox{})
	handler := NewFollowHandler(svc, db.NewFromPool(testDB.Pool()), zap.NewNop())
	return testDB, handler, cleanup
}

func newFollowGin(method, path string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(method, path, nil)
	c.Request = req
	return c, w
}

func parseFollowBody(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var env map[string]any
	require.NoError(t, json.Unmarshal(body, &env))
	return env
}

// insertFollowTestUser creates a user with the given fields. username and
// avatarURL are stored in user_profiles. accountStatus goes into
// users.account_status. deletedAt (if non-nil) sets users.deleted_at.
func insertFollowTestUser(
	t *testing.T, ctx context.Context, td *testdb.TestDB,
	id uuid.UUID, username, accountStatus string,
	deletedAt *time.Time, avatarURL string,
) {
	t.Helper()
	now := time.Now().UTC()
	var avatar *string
	if avatarURL != "" {
		avatar = &avatarURL
	}
	err := td.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO users (id, firebase_uid, email, account_status, deleted_at, created_at, updated_at, role)
			VALUES ($1,$2,$3,$4,$5,$6,$6,'user')`,
			id, "fb-"+id.String(), id.String()+"@test.local",
			accountStatus, deletedAt, now)
		if err != nil {
			return err
		}
		if username != "" || avatar != nil {
			_, err = tx.Exec(ctx, `
				INSERT INTO user_profiles (id, user_id, username, avatar_url, created_at, updated_at)
				VALUES ($1,$2,$3,$4,$5,$5)`,
				uuid.New(), id, username, avatar, now)
			return err
		}
		return nil
	})
	require.NoError(t, err)
}

// insertFollowRelation creates a follow edge.
func insertFollowRelation(
	t *testing.T, ctx context.Context, td *testdb.TestDB,
	followerID, followingID uuid.UUID,
) {
	t.Helper()
	err := td.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id, created_at)
			VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`,
			followerID, followingID, time.Now().UTC())
		return err
	})
	require.NoError(t, err)
}

// =============================================================================
// hydrateFollowUserCards — PostgreSQL tests
// =============================================================================

func TestHydrateFollowUserCards_ActiveWithProfile(t *testing.T) {
	td, handler, cleanup := newFollowTestEnv(t)
	defer cleanup()
	ctx := context.Background()

	userID := uuid.New()
	insertFollowTestUser(t, ctx, td, userID, "alice", "active", nil,
		"https://cdn.example.com/a.jpg")

	cards, err := handler.hydrateFollowUserCards(ctx, []uuid.UUID{userID})
	require.NoError(t, err)
	require.Len(t, cards, 1)

	c := cards[0]
	assert.Equal(t, userID, c.ID)
	assert.Equal(t, "alice", c.Username)
	assert.Equal(t, "https://cdn.example.com/a.jpg", *c.AvatarURL)
	assert.Equal(t, "active", c.Lifecycle)
	assert.NotContains(t, c.Username, "user_")
}

func TestHydrateFollowUserCards_ActiveWithoutProfile(t *testing.T) {
	td, handler, cleanup := newFollowTestEnv(t)
	defer cleanup()
	ctx := context.Background()

	userID := uuid.New()
	// No user_profiles row — username will be empty from COALESCE.
	insertFollowTestUser(t, ctx, td, userID, "", "active", nil, "")

	cards, err := handler.hydrateFollowUserCards(ctx, []uuid.UUID{userID})
	require.NoError(t, err)
	require.Len(t, cards, 1)

	c := cards[0]
	assert.Equal(t, userID, c.ID)
	assert.Empty(t, c.Username, "active without profile: username empty")
	assert.Nil(t, c.AvatarURL)
	assert.Equal(t, "active", c.Lifecycle)
	assert.NotContains(t, c.Username, "user_")
}

func TestHydrateFollowUserCards_Suspended(t *testing.T) {
	td, handler, cleanup := newFollowTestEnv(t)
	defer cleanup()
	ctx := context.Background()

	userID := uuid.New()
	// Deliberately store live identity to prove redaction.
	insertFollowTestUser(t, ctx, td, userID, "suspended_person", "suspended", nil,
		"https://cdn.example.com/sus.jpg")

	cards, err := handler.hydrateFollowUserCards(ctx, []uuid.UUID{userID})
	require.NoError(t, err)
	require.Len(t, cards, 1)

	c := cards[0]
	assert.Equal(t, userID, c.ID)
	assert.Empty(t, c.Username, "suspended: username redacted")
	assert.Nil(t, c.AvatarURL, "suspended: avatar redacted")
	assert.Equal(t, "unavailable", c.Lifecycle)
}

func TestHydrateFollowUserCards_Banned(t *testing.T) {
	td, handler, cleanup := newFollowTestEnv(t)
	defer cleanup()
	ctx := context.Background()

	userID := uuid.New()
	insertFollowTestUser(t, ctx, td, userID, "banned_user", "banned", nil,
		"https://cdn.example.com/ban.jpg")

	cards, err := handler.hydrateFollowUserCards(ctx, []uuid.UUID{userID})
	require.NoError(t, err)
	require.Len(t, cards, 1)

	c := cards[0]
	assert.Empty(t, c.Username, "banned: username redacted")
	assert.Nil(t, c.AvatarURL)
	assert.Equal(t, "unavailable", c.Lifecycle)
}

func TestHydrateFollowUserCards_SoftDeleted(t *testing.T) {
	td, handler, cleanup := newFollowTestEnv(t)
	defer cleanup()
	ctx := context.Background()

	userID := uuid.New()
	delTime := time.Now().UTC().Add(-1 * time.Hour)
	insertFollowTestUser(t, ctx, td, userID, "deleted_person", "active", &delTime,
		"https://cdn.example.com/del.jpg")

	cards, err := handler.hydrateFollowUserCards(ctx, []uuid.UUID{userID})
	require.NoError(t, err)
	require.Len(t, cards, 1)

	c := cards[0]
	assert.Empty(t, c.Username, "soft-deleted: username redacted")
	assert.Nil(t, c.AvatarURL)
	assert.Equal(t, "removed", c.Lifecycle)
}

func TestHydrateFollowUserCards_HardMissingUser(t *testing.T) {
	td, handler, cleanup := newFollowTestEnv(t)
	defer cleanup()
	ctx := context.Background()

	// Create two users, then hydrate three IDs (one missing) — verify the
	// missing one is dropped and ordering is preserved.
	u1 := uuid.New()
	u2 := uuid.New()
	uMissing := uuid.New() // never inserted

	insertFollowTestUser(t, ctx, td, u1, "first", "active", nil, "")
	insertFollowTestUser(t, ctx, td, u2, "second", "active", nil, "")

	cards, err := handler.hydrateFollowUserCards(ctx, []uuid.UUID{u1, uMissing, u2})
	require.NoError(t, err)
	require.Len(t, cards, 2, "missing user dropped")

	assert.Equal(t, u1, cards[0].ID, "first card is first input")
	assert.Equal(t, "first", cards[0].Username)

	assert.Equal(t, u2, cards[1].ID, "second card is third input (missing skipped)")
	assert.Equal(t, "second", cards[1].Username)
}

func TestHydrateFollowUserCards_BatchOrdering(t *testing.T) {
	td, handler, cleanup := newFollowTestEnv(t)
	defer cleanup()
	ctx := context.Background()

	// Create users with controlled IDs, insert out of order to verify output
	// follows input order, not SQL result order.
	ids := make([]uuid.UUID, 5)
	for i := range ids {
		ids[i] = uuid.New()
	}
	// Insert in reverse order to ensure SQL doesn't return input order naturally.
	// Use empty username to avoid unique constraint — ordering proof doesn't
	// depend on username values.
	for i := len(ids) - 1; i >= 0; i-- {
		insertFollowTestUser(t, ctx, td, ids[i], "", "active", nil, "")
	}

	cards, err := handler.hydrateFollowUserCards(ctx, ids)
	require.NoError(t, err)
	require.Len(t, cards, 5)

	for i, c := range cards {
		assert.Equal(t, ids[i], c.ID, "position %d must match input order", i)
	}
}

func TestHydrateFollowUserCards_EmptyInput(t *testing.T) {
	_, handler, cleanup := newFollowTestEnv(t)
	defer cleanup()

	cards, err := handler.hydrateFollowUserCards(context.Background(), []uuid.UUID{})
	require.NoError(t, err)
	assert.Empty(t, cards)
}

func TestHydrateFollowUserCards_DuplicateIDs(t *testing.T) {
	td, handler, cleanup := newFollowTestEnv(t)
	defer cleanup()
	ctx := context.Background()

	u := uuid.New()
	insertFollowTestUser(t, ctx, td, u, "dup", "active", nil, "")

	// Two occurrences of the same ID — each that resolves produces one entry.
	cards, err := handler.hydrateFollowUserCards(ctx, []uuid.UUID{u, u})
	require.NoError(t, err)
	require.Len(t, cards, 2)
	assert.Equal(t, u, cards[0].ID)
	assert.Equal(t, u, cards[1].ID)
}

func TestHydrateFollowUserCards_Counts(t *testing.T) {
	td, handler, cleanup := newFollowTestEnv(t)
	defer cleanup()
	ctx := context.Background()

	u := uuid.New()
	v := uuid.New()

	insertFollowTestUser(t, ctx, td, u, "u", "active", nil, "")
	insertFollowTestUser(t, ctx, td, v, "v", "active", nil, "")

	// v follows u → u has 1 follower, v follows 1 person.
	insertFollowRelation(t, ctx, td, v, u)

	cards, err := handler.hydrateFollowUserCards(ctx, []uuid.UUID{u, v})
	require.NoError(t, err)
	require.Len(t, cards, 2)

	// u is followed by v → followers_count = 1
	assert.Equal(t, 1, cards[0].FollowersCount)
	assert.Equal(t, 0, cards[0].FollowingCount)

	// v follows u → following_count = 1
	assert.Equal(t, 0, cards[1].FollowersCount)
	assert.Equal(t, 1, cards[1].FollowingCount)
}

// =============================================================================
// Lifecycle projection — verify viewercontext.CoarsenLifecycle integration
// =============================================================================

func TestHydrateFollowUserCards_LifecycleActive(t *testing.T) {
	lifecycleStates := []string{"active"}
	for _, status := range lifecycleStates {
		t.Run("account_"+status, func(t *testing.T) {
			td, handler, cleanup := newFollowTestEnv(t)
			defer cleanup()
			ctx := context.Background()

			u := uuid.New()
			insertFollowTestUser(t, ctx, td, u, "user", status, nil, "")

			cards, err := handler.hydrateFollowUserCards(ctx, []uuid.UUID{u})
			require.NoError(t, err)
			require.Len(t, cards, 1)
			assert.Equal(t, "active", cards[0].Lifecycle)
		})
	}
}

func TestHydrateFollowUserCards_LifecycleUnavailable(t *testing.T) {
	lifecycleStates := []string{"suspended", "banned"}
	for _, status := range lifecycleStates {
		t.Run("account_"+status, func(t *testing.T) {
			td, handler, cleanup := newFollowTestEnv(t)
			defer cleanup()
			ctx := context.Background()

			u := uuid.New()
			insertFollowTestUser(t, ctx, td, u, "user", status, nil, "")

			cards, err := handler.hydrateFollowUserCards(ctx, []uuid.UUID{u})
			require.NoError(t, err)
			require.Len(t, cards, 1)
			assert.Equal(t, "unavailable", cards[0].Lifecycle)
			assert.Empty(t, cards[0].Username)
			assert.Nil(t, cards[0].AvatarURL)
		})
	}
}

func TestHydrateFollowUserCards_LifecycleRemoved(t *testing.T) {
	td, handler, cleanup := newFollowTestEnv(t)
	defer cleanup()
	ctx := context.Background()

	u := uuid.New()
	delTime := time.Now().UTC().Add(-2 * time.Hour)
	insertFollowTestUser(t, ctx, td, u, "removed_user", "active", &delTime, "")

	cards, err := handler.hydrateFollowUserCards(ctx, []uuid.UUID{u})
	require.NoError(t, err)
	require.Len(t, cards, 1)
	assert.Equal(t, "removed", cards[0].Lifecycle)
	assert.Empty(t, cards[0].Username)
	assert.Nil(t, cards[0].AvatarURL)
}

// =============================================================================
// Handler-level integration tests
// =============================================================================

func TestListFollowers_Handler_ActiveUser(t *testing.T) {
	td, handler, cleanup := newFollowTestEnv(t)
	defer cleanup()
	ctx := context.Background()

	target := uuid.New()
	follower := uuid.New()

	insertFollowTestUser(t, ctx, td, target, "target_user", "active", nil, "")
	insertFollowTestUser(t, ctx, td, follower, "follower_user", "active", nil,
		"https://cdn.example.com/f.jpg")
	insertFollowRelation(t, ctx, td, follower, target)

	c, w := newFollowGin("GET", "/api/v1/users/"+target.String()+"/followers")
	c.Params = gin.Params{{Key: "id", Value: target.String()}}

	handler.ListFollowers(c)

	assert.Equal(t, http.StatusOK, w.Code)

	body := parseFollowBody(t, w.Body.Bytes())
	assert.True(t, body["success"].(bool))

	data := body["data"].(map[string]any)
	followers := data["followers"].([]any)
	require.Len(t, followers, 1)

	f := followers[0].(map[string]any)
	assert.Equal(t, follower.String(), f["id"])
	assert.Equal(t, "follower_user", f["username"])
	assert.Equal(t, "https://cdn.example.com/f.jpg", f["avatar_url"])
	assert.Equal(t, "active", f["lifecycle"])

	// Verify no synthetic identity.
	compact, _ := json.Marshal(body)
	assert.NotContains(t, string(compact), "user_")
}

func TestListFollowers_Handler_DegradedUserRedacted(t *testing.T) {
	td, handler, cleanup := newFollowTestEnv(t)
	defer cleanup()
	ctx := context.Background()

	target := uuid.New()
	suspended := uuid.New()

	insertFollowTestUser(t, ctx, td, target, "target", "active", nil, "")
	// Suspended follower WITH live identity — must be redacted in response.
	insertFollowTestUser(t, ctx, td, suspended, "bad_actor", "suspended", nil,
		"https://cdn.example.com/bad.jpg")
	insertFollowRelation(t, ctx, td, suspended, target)

	c, w := newFollowGin("GET", "/api/v1/users/"+target.String()+"/followers")
	c.Params = gin.Params{{Key: "id", Value: target.String()}}

	handler.ListFollowers(c)

	assert.Equal(t, http.StatusOK, w.Code)

	body := parseFollowBody(t, w.Body.Bytes())
	data := body["data"].(map[string]any)
	followers := data["followers"].([]any)
	require.Len(t, followers, 1)

	f := followers[0].(map[string]any)
	assert.Equal(t, suspended.String(), f["id"])
	assert.Equal(t, "", f["username"], "degraded username must be empty")
	assert.Nil(t, f["avatar_url"])
	assert.Equal(t, "unavailable", f["lifecycle"])

	compact, _ := json.Marshal(body)
	assert.NotContains(t, string(compact), "bad_actor", "live username must not leak")
	assert.NotContains(t, string(compact), "user_")
}

func TestListFollowing_Handler_ActiveWithProfile(t *testing.T) {
	td, handler, cleanup := newFollowTestEnv(t)
	defer cleanup()
	ctx := context.Background()

	viewer := uuid.New()
	following := uuid.New()

	insertFollowTestUser(t, ctx, td, viewer, "viewer", "active", nil, "")
	insertFollowTestUser(t, ctx, td, following, "followed", "active", nil,
		"https://cdn.example.com/fd.jpg")
	insertFollowRelation(t, ctx, td, viewer, following)

	c, w := newFollowGin("GET", "/api/v1/users/"+viewer.String()+"/following")
	c.Params = gin.Params{{Key: "id", Value: viewer.String()}}

	handler.ListFollowing(c)

	assert.Equal(t, http.StatusOK, w.Code)

	body := parseFollowBody(t, w.Body.Bytes())
	data := body["data"].(map[string]any)
	followingList := data["following"].([]any)
	require.Len(t, followingList, 1)

	f := followingList[0].(map[string]any)
	assert.Equal(t, following.String(), f["id"])
	assert.Equal(t, "followed", f["username"])
	assert.Equal(t, "https://cdn.example.com/fd.jpg", f["avatar_url"])
	assert.Equal(t, "active", f["lifecycle"])
}

func TestListFollowing_Handler_SoftDeletedUserRedacted(t *testing.T) {
	td, handler, cleanup := newFollowTestEnv(t)
	defer cleanup()
	ctx := context.Background()

	viewer := uuid.New()
	gone := uuid.New()

	insertFollowTestUser(t, ctx, td, viewer, "viewer", "active", nil, "")
	delTime := time.Now().UTC().Add(-1 * time.Hour)
	insertFollowTestUser(t, ctx, td, gone, "gone_user", "active", &delTime,
		"https://cdn.example.com/gone.jpg")
	insertFollowRelation(t, ctx, td, viewer, gone)

	c, w := newFollowGin("GET", "/api/v1/users/"+viewer.String()+"/following")
	c.Params = gin.Params{{Key: "id", Value: viewer.String()}}

	handler.ListFollowing(c)

	assert.Equal(t, http.StatusOK, w.Code)

	body := parseFollowBody(t, w.Body.Bytes())
	data := body["data"].(map[string]any)
	followingList := data["following"].([]any)
	require.Len(t, followingList, 1)

	f := followingList[0].(map[string]any)
	assert.Equal(t, gone.String(), f["id"])
	assert.Equal(t, "", f["username"], "removed username must be empty")
	assert.Nil(t, f["avatar_url"])
	assert.Equal(t, "removed", f["lifecycle"])

	compact, _ := json.Marshal(body)
	assert.NotContains(t, string(compact), "gone_user")
	assert.NotContains(t, string(compact), "user_")
}

// =============================================================================
// Hydration database failure — error propagation
// =============================================================================

func TestListFollowers_HydrationDBError_Returns500(t *testing.T) {
	// Create a handler with a database that will fail on query.
	// We simulate this by closing the pool before the handler call.
	td, handler, cleanup := newFollowTestEnv(t)
	defer cleanup()
	ctx := context.Background()

	// Create a valid target user first so service layer succeeds.
	target := uuid.New()
	follower := uuid.New()
	insertFollowTestUser(t, ctx, td, target, "target", "active", nil, "")
	insertFollowTestUser(t, ctx, td, follower, "follower", "active", nil, "")
	insertFollowRelation(t, ctx, td, follower, target)

	// Close the pool so the hydration query fails.
	td.Pool().Close()

	c, w := newFollowGin("GET", "/api/v1/users/"+target.String()+"/followers")
	c.Params = gin.Params{{Key: "id", Value: target.String()}}

	handler.ListFollowers(c)

	assert.NotEqual(t, http.StatusOK, w.Code,
		"must not return 200 on hydration error")
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	body := parseFollowBody(t, w.Body.Bytes())
	assert.False(t, body["success"].(bool))
	errInfo := body["error"].(map[string]any)
	assert.NotEmpty(t, errInfo["message"])
}

// =============================================================================
// Negative contracts — full response envelope
// =============================================================================

func TestListFollowers_NegativeContracts(t *testing.T) {
	td, handler, cleanup := newFollowTestEnv(t)
	defer cleanup()
	ctx := context.Background()

	target := uuid.New()
	f1 := uuid.New() // active with profile
	f2 := uuid.New() // suspended
	f3 := uuid.New() // soft-deleted
	delTime := time.Now().UTC().Add(-3 * time.Hour)

	// Use unique usernames via ID suffix to avoid unique constraint.
	insertFollowTestUser(t, ctx, td, target, "target-"+target.String()[:12], "active", nil, "")
	insertFollowTestUser(t, ctx, td, f1, "normal-"+f1.String()[:12], "active", nil,
		"https://cdn.example.com/n.jpg")
	insertFollowTestUser(t, ctx, td, f2, "suspect-"+f2.String()[:12], "suspended", nil,
		"https://cdn.example.com/s.jpg")
	insertFollowTestUser(t, ctx, td, f3, "ghost-"+f3.String()[:12], "active", &delTime,
		"https://cdn.example.com/g.jpg")

	insertFollowRelation(t, ctx, td, f1, target)
	insertFollowRelation(t, ctx, td, f2, target)
	insertFollowRelation(t, ctx, td, f3, target)

	c, w := newFollowGin("GET", "/api/v1/users/"+target.String()+"/followers")
	c.Params = gin.Params{{Key: "id", Value: target.String()}}

	handler.ListFollowers(c)
	assert.Equal(t, http.StatusOK, w.Code)

	compact, _ := json.Marshal(parseFollowBody(t, w.Body.Bytes()))
	s := string(compact)

	// Forbidden in any form.
	forbidden := []string{
		"user_",
		"suspect", // live suspended username
		"ghost",   // live removed username
		"email",
		"phone",
		"full_name",
		"store_name",
		"farm_name",
		"firebase_uid",
		"account_status",
	}
	for _, f := range forbidden {
		assert.NotContains(t, s, f, "response must not contain %q", f)
	}

	// Verify lifecycle values are present for all three.
	assert.Contains(t, s, `"lifecycle":"active"`)
	assert.Contains(t, s, `"lifecycle":"unavailable"`)
	assert.Contains(t, s, `"lifecycle":"removed"`)

	// Verify the active user has correct identity (username with ID suffix).
	assert.Contains(t, s, f1.String())
	assert.Contains(t, s, `"lifecycle":"active"`)
	assert.Contains(t, s, `"avatar_url":"https://cdn.example.com/n.jpg"`)
}

// =============================================================================
// Cleanup proof — zero publiccard in follow response path
// =============================================================================

func TestFollowHandler_NoPubliccardInResponse(t *testing.T) {
	td, handler, cleanup := newFollowTestEnv(t)
	defer cleanup()
	ctx := context.Background()

	u := uuid.New()
	insertFollowTestUser(t, ctx, td, u, "test", "active", nil, "")

	cards, err := handler.hydrateFollowUserCards(ctx, []uuid.UUID{u})
	require.NoError(t, err)
	require.Len(t, cards, 1)

	// Verify the type is FollowUserCardResponse, not publiccard.UserCard.
	_, ok := any(cards[0]).(FollowUserCardResponse)
	assert.True(t, ok, "response type must be FollowUserCardResponse")
}
