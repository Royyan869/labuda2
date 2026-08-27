//go:build integration

package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	notificationrepoimpl "github.com/labuda/backend/internal/interaction/notification/infrastructure/repository"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	contentrepo "github.com/labuda/backend/internal/social/content/infrastructure/repository"
	socialrepo "github.com/labuda/backend/internal/social/graph/infrastructure/repository"
	likeApp "github.com/labuda/backend/internal/social/like/application"
	likeHTTP "github.com/labuda/backend/internal/social/like/delivery/http"
	likerepo "github.com/labuda/backend/internal/social/like/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type likeStatsFixture struct {
	appDB   *db.DB
	handler *likeHTTP.LikeHandler
}

func newLikeStatsFixture(t *testing.T) *likeStatsFixture {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	appDB := db.NewFromPool(tdb.Pool())
	blockChecker := socialrepo.NewSocialRepository()

	likeService := likeApp.NewService(
		appDB,
		contentrepo.NewContentRepository(),
		likerepo.NewLikeRepository(),
		outboxRepo.NewOutboxRepository(appDB),
		blockChecker,
		nil,
		&notificationrepoimpl.NotificationRepository{},
	)
	commentLikeService := likeApp.NewCommentLikeService(
		appDB,
		contentrepo.NewContentRepository(),
		contentrepo.NewCommentRepository(),
		likerepo.NewTargetLikeRepository(),
		blockChecker,
	)

	return &likeStatsFixture{
		appDB:   appDB,
		handler: likeHTTP.NewLikeHandler(appDB, zap.NewNop(), likeService, commentLikeService),
	}
}

func seedLikeStatsUser(t *testing.T, ctx context.Context, pool *db.DB, username string) uuid.UUID {
	t.Helper()
	userID := uuid.New()
	_, err := pool.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, email_verified_at, phone_verified, account_status, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), true, 'active', NOW(), NOW())
	`, userID, userID.String(), username+"@"+"test.invalid")
	require.NoError(t, err)
	_, err = pool.Pool().Exec(ctx, `
		INSERT INTO user_profiles (id, user_id, username, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
	`, uuid.New(), userID, username)
	require.NoError(t, err)
	return userID
}

func seedLikeStatsContent(t *testing.T, ctx context.Context, pool *db.DB, authorID uuid.UUID, status, caption string, hidden bool) uuid.UUID {
	t.Helper()
	contentID := uuid.New()
	_, err := pool.Pool().Exec(ctx, `
		INSERT INTO contents (id, author_id, status, caption, visibility, is_hidden, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'public', $5, NOW(), NOW())
	`, contentID, authorID, status, caption, hidden)
	require.NoError(t, err)
	return contentID
}

func seedLikeStatsComment(t *testing.T, ctx context.Context, pool *db.DB, authorID, contentID uuid.UUID) uuid.UUID {
	t.Helper()
	commentID := uuid.New()
	_, err := pool.Pool().Exec(ctx, `
		INSERT INTO comments (id, author_id, body, target_id, target_type, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'content', NOW(), NOW())
	`, commentID, authorID, "stats comment", contentID)
	require.NoError(t, err)
	return commentID
}

func seedLike(t *testing.T, ctx context.Context, pool *db.DB, contentID, userID uuid.UUID) {
	t.Helper()
	_, err := pool.Pool().Exec(ctx, `
		INSERT INTO content_likes (content_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (content_id, user_id) DO NOTHING
	`, contentID, userID)
	require.NoError(t, err)
}

func seedBlock(t *testing.T, ctx context.Context, pool *db.DB, blockerID, blockedID uuid.UUID) {
	t.Helper()
	_, err := pool.Pool().Exec(ctx, `
		INSERT INTO user_blocks (blocker_id, blocked_id, created_at)
		VALUES ($1, $2, NOW())
	`, blockerID, blockedID)
	require.NoError(t, err)
}

func doLikeStatsRequest(t *testing.T, handler *likeHTTP.LikeHandler, targetID uuid.UUID, targetType string, viewerID *uuid.UUID) (int, map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/likes/stats?target_id="+targetID.String()+"&target_type="+targetType, nil)
	c.Request = req
	if viewerID != nil {
		c.Set("userID", *viewerID)
	}
	handler.GetLikeStats(c)

	var body struct {
		Data map[string]any `json:"data"`
	}
	if len(w.Body.Bytes()) > 0 {
		_ = json.Unmarshal(w.Body.Bytes(), &body)
	}
	return w.Code, body.Data
}

// TestLikeToggle_HTTPLifecycleProof exercises POST /likes/toggle +
// GET /likes/stats over the real HTTP+DB stack and cross-checks every step
// against content_likes row state:
//
//	LIKE → 200 liked=true count=1 → 1 DB row → stats is_liked=true count=1
//	UNLIKE → 200 liked=false count=0 → 0 DB row → stats is_liked=false count=0
//	LIKE AGAIN → 200 liked=true count=1 → 1 DB row → stats is_liked=true count=1
func TestLikeToggle_HTTPLifecycleProof(t *testing.T) {
	fixture := newLikeStatsFixture(t)
	ctx := context.Background()

	authorID := seedLikeStatsUser(t, ctx, fixture.appDB, "http-cycle-author")
	viewerID := seedLikeStatsUser(t, ctx, fixture.appDB, "http-cycle-viewer")
	contentID := seedLikeStatsContent(t, ctx, fixture.appDB, authorID, "active", "http cycle post", false)

	doToggle := func() (bool, float64) {
		t.Helper()
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body := `{"target_id":"` + contentID.String() + `","target_type":"content"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/likes/toggle", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req
		c.Set("userID", viewerID)
		fixture.handler.ToggleLike(c)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var resp struct {
			Data struct {
				Liked bool    `json:"liked"`
				Count float64 `json:"count"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		return resp.Data.Liked, resp.Data.Count
	}

	dbLikeRows := func() int {
		t.Helper()
		var n int
		require.NoError(t, fixture.appDB.Pool().QueryRow(ctx, `
			SELECT COUNT(*) FROM content_likes WHERE content_id = $1 AND user_id = $2
		`, contentID, viewerID).Scan(&n))
		return n
	}

	t.Run("LIKE", func(t *testing.T) {
		liked, count := doToggle()
		require.True(t, liked)
		require.Equal(t, float64(1), count)
		require.Equal(t, 1, dbLikeRows(), "LIKE must leave exactly one content_likes row")

		code, data := doLikeStatsRequest(t, fixture.handler, contentID, "content", &viewerID)
		require.Equal(t, http.StatusOK, code)
		require.Equal(t, float64(1), data["count"])
		require.Equal(t, true, data["is_liked"])
	})

	t.Run("UNLIKE", func(t *testing.T) {
		liked, count := doToggle()
		require.False(t, liked)
		require.Equal(t, float64(0), count)
		require.Equal(t, 0, dbLikeRows(), "UNLIKE must remove the content_likes row")

		code, data := doLikeStatsRequest(t, fixture.handler, contentID, "content", &viewerID)
		require.Equal(t, http.StatusOK, code)
		require.Equal(t, float64(0), data["count"])
		require.Equal(t, false, data["is_liked"])
	})

	t.Run("LIKE AGAIN after UNLIKE", func(t *testing.T) {
		liked, count := doToggle()
		require.True(t, liked)
		require.Equal(t, float64(1), count)
		require.Equal(t, 1, dbLikeRows(), "re-like must create exactly one current like row")

		code, data := doLikeStatsRequest(t, fixture.handler, contentID, "content", &viewerID)
		require.Equal(t, http.StatusOK, code)
		require.Equal(t, float64(1), data["count"])
		require.Equal(t, true, data["is_liked"])
	})
}

// TestLikeStats_VisibilityGovernance proves GET /likes/stats uses the same
// visibility authority as content detail / comment surfaces: no like metadata
// for deleted, hidden, or blocked targets, and is_liked is only disclosed for
// visible content to the requesting viewer.
func TestLikeStats_VisibilityGovernance(t *testing.T) {
	fixture := newLikeStatsFixture(t)
	ctx := context.Background()

	authorID := seedLikeStatsUser(t, ctx, fixture.appDB, "stats-author")
	viewerID := seedLikeStatsUser(t, ctx, fixture.appDB, "stats-viewer")
	likerID := seedLikeStatsUser(t, ctx, fixture.appDB, "stats-liker")

	visibleContent := seedLikeStatsContent(t, ctx, fixture.appDB, authorID, "active", "visible stats post", false)
	deletedContent := seedLikeStatsContent(t, ctx, fixture.appDB, authorID, "deleted", "deleted stats post", false)
	hiddenContent := seedLikeStatsContent(t, ctx, fixture.appDB, authorID, "active", "hidden stats post", true)

	seedLike(t, ctx, fixture.appDB, visibleContent, authorID)
	seedLike(t, ctx, fixture.appDB, visibleContent, likerID)
	seedLike(t, ctx, fixture.appDB, deletedContent, likerID)
	seedLike(t, ctx, fixture.appDB, hiddenContent, likerID)

	t.Run("visible content discloses stats to authed viewer", func(t *testing.T) {
		code, data := doLikeStatsRequest(t, fixture.handler, visibleContent, "content", &viewerID)
		require.Equal(t, http.StatusOK, code)
		require.Equal(t, float64(2), data["count"])
		require.Equal(t, false, data["is_liked"])
	})

	t.Run("visible content discloses stats to viewer who liked it", func(t *testing.T) {
		code, data := doLikeStatsRequest(t, fixture.handler, visibleContent, "content", &likerID)
		require.Equal(t, http.StatusOK, code)
		require.Equal(t, float64(2), data["count"])
		require.Equal(t, true, data["is_liked"])
	})

	t.Run("anonymous viewer sees count but never a true is_liked", func(t *testing.T) {
		code, data := doLikeStatsRequest(t, fixture.handler, visibleContent, "content", nil)
		require.Equal(t, http.StatusOK, code)
		require.Equal(t, float64(2), data["count"])
		require.Equal(t, false, data["is_liked"])
	})

	t.Run("deleted content leaks no like stats", func(t *testing.T) {
		code, data := doLikeStatsRequest(t, fixture.handler, deletedContent, "content", &likerID)
		require.Equal(t, http.StatusNotFound, code)
		require.Nil(t, data)
	})

	t.Run("hidden content leaks no like stats", func(t *testing.T) {
		code, data := doLikeStatsRequest(t, fixture.handler, hiddenContent, "content", &authorID)
		require.Equal(t, http.StatusNotFound, code)
		require.Nil(t, data)
	})

	t.Run("nonexistent content leaks no like stats", func(t *testing.T) {
		code, data := doLikeStatsRequest(t, fixture.handler, uuid.New(), "content", &viewerID)
		require.Equal(t, http.StatusNotFound, code)
		require.Nil(t, data)
	})

	t.Run("comment under hidden content leaks nothing", func(t *testing.T) {
		hiddenComment := seedLikeStatsComment(t, ctx, fixture.appDB, authorID, hiddenContent)
		code, data := doLikeStatsRequest(t, fixture.handler, hiddenComment, "comment", &viewerID)
		require.Equal(t, http.StatusNotFound, code)
		require.Nil(t, data)
	})

	t.Run("comment under visible content discloses stats", func(t *testing.T) {
		visibleComment := seedLikeStatsComment(t, ctx, fixture.appDB, authorID, visibleContent)
		code, data := doLikeStatsRequest(t, fixture.handler, visibleComment, "comment", &viewerID)
		require.Equal(t, http.StatusOK, code)
		require.Equal(t, float64(0), data["count"])
		require.Equal(t, false, data["is_liked"])
	})

	t.Run("blocked viewer cannot bypass via /likes/stats", func(t *testing.T) {
		seedBlock(t, ctx, fixture.appDB, viewerID, authorID)
		code, data := doLikeStatsRequest(t, fixture.handler, visibleContent, "content", &viewerID)
		require.Equal(t, http.StatusNotFound, code)
		require.Nil(t, data)
	})

	t.Run("comment under visible content hidden from blocked viewer", func(t *testing.T) {
		visibleComment := seedLikeStatsComment(t, ctx, fixture.appDB, authorID, visibleContent)
		code, data := doLikeStatsRequest(t, fixture.handler, visibleComment, "comment", &viewerID)
		require.Equal(t, http.StatusNotFound, code)
		require.Nil(t, data)
	})
}
