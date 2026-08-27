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
	contentapp "github.com/labuda/backend/internal/social/content/application"
	contententity "github.com/labuda/backend/internal/social/content/entity"
	contentrepo "github.com/labuda/backend/internal/social/content/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type commentListHTTPRoleChecker struct{}

func (commentListHTTPRoleChecker) IsAdmin(context.Context, uuid.UUID) (bool, error) {
	return false, nil
}

func (commentListHTTPRoleChecker) HasActiveSellerCapability(context.Context, uuid.UUID) (bool, error) {
	return false, nil
}

func (commentListHTTPRoleChecker) HasSellerProfile(context.Context, uuid.UUID) (bool, error) {
	return false, nil
}

type commentListHTTPAccountChecker struct{}

func (commentListHTTPAccountChecker) EnsureActive(context.Context, uuid.UUID) error {
	return nil
}

func (commentListHTTPAccountChecker) GetStatus(context.Context, uuid.UUID) (string, error) {
	return "active", nil
}

func (commentListHTTPAccountChecker) IsBanned(context.Context, uuid.UUID) (bool, error) {
	return false, nil
}

func newCommentListHTTPHandler(pool *db.DB) *CommentHandler {
	contentService := contentapp.NewContentService(
		contentrepo.NewContentRepository(),
		nil,
		commentListHTTPRoleChecker{},
		commentListHTTPAccountChecker{},
		nil,
	)
	commentService := contentapp.NewCommentService(
		contentrepo.NewContentRepository(),
		contentrepo.NewCommentRepository(),
		nil, // fpsValidator
		nil, // auctionValidator
		nil, // visibilityChecker
		nil, // outboxRepo
		nil, // idempotencyRepo
		nil, // blockChecker
		nil, // sellerCapabilityChecker
		nil, // invariantLogger
	)
	return NewCommentHandler(
		commentService,
		contentService,
		pool,
		zap.NewNop(),
	)
}

func seedCommentListHTTPUser(t *testing.T, ctx context.Context, pool *db.DB, username string) uuid.UUID {
	t.Helper()

	userID := uuid.New()
	now := time.Now().UTC()
	usernameValue := username
	avatarURL := "https://cdn.test/avatar.png"

	err := pool.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at, role)
			VALUES ($1, $2, $3, 'active', $4, $4, 'user')
		`, userID, "fb-"+userID.String(), userID.String()+"@test.local", now)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO user_profiles (id, user_id, username, avatar_url, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $5)
		`, uuid.New(), userID, usernameValue, avatarURL, now)
		return err
	})
	require.NoError(t, err)
	return userID
}

func seedCommentListHTTPContent(
	t *testing.T,
	ctx context.Context,
	pool *db.DB,
	handler *CommentHandler,
	authorID uuid.UUID,
) uuid.UUID {
	t.Helper()

	var contentID uuid.UUID
	err := pool.WithTx(ctx, func(tx db.Tx) error {
		content, createErr := handler.contentService.CreateContent(
			ctx,
			tx,
			authorID,
			"test content",
			contententity.VisibilityPublic,
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
		return nil
	})
	require.NoError(t, err)
	return contentID
}

func seedCommentListHTTPComment(
	t *testing.T,
	ctx context.Context,
	pool *db.DB,
	handler *CommentHandler,
	contentID uuid.UUID,
	authorID uuid.UUID,
	body string,
) {
	t.Helper()

	err := pool.WithTx(ctx, func(tx db.Tx) error {
		_, createErr := handler.commentService.AddComment(
			ctx,
			tx,
			authorID,
			contentID,
			body,
			nil,
			uuid.New().String(), // unique idempotency key per seeded comment
		)
		return createErr
	})
	require.NoError(t, err)
}

func performCommentListRequest(
	t *testing.T,
	handler *CommentHandler,
	contentID uuid.UUID,
	viewerID uuid.UUID,
) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("userID", viewerID)
		c.Next()
	})
	router.GET("/api/v1/contents/:id/comments", handler.ListComments)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/contents/"+contentID.String()+"/comments?limit=20",
		nil,
	)
	router.ServeHTTP(w, req)
	return w
}

func decodeEnvelope(t *testing.T, body []byte) map[string]any {
	t.Helper()

	var env map[string]any
	require.NoError(t, json.Unmarshal(body, &env))
	return env
}

func TestCommentListHandler_EmptyPage_Integration(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	handler := newCommentListHTTPHandler(appDB)

	userID := seedCommentListHTTPUser(t, ctx, appDB, "empty-user")
	contentID := seedCommentListHTTPContent(t, ctx, appDB, handler, userID)

	w := performCommentListRequest(t, handler, contentID, userID)

	require.Equal(t, http.StatusOK, w.Code)
	env := decodeEnvelope(t, w.Body.Bytes())
	require.Equal(t, true, env["success"])

	data := env["data"].(map[string]any)
	require.Equal(t, float64(20), data["limit"])

	comments := data["comments"].([]any)
	require.Len(t, comments, 0)
	require.NotContains(t, data, "next_cursor")
}

func TestCommentListHandler_PopulatedPage_Integration(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	handler := newCommentListHTTPHandler(appDB)

	userID := seedCommentListHTTPUser(t, ctx, appDB, "populated-user")
	contentID := seedCommentListHTTPContent(t, ctx, appDB, handler, userID)
	seedCommentListHTTPComment(t, ctx, appDB, handler, contentID, userID, "hello from comments")

	w := performCommentListRequest(t, handler, contentID, userID)

	require.Equal(t, http.StatusOK, w.Code)
	env := decodeEnvelope(t, w.Body.Bytes())
	require.Equal(t, true, env["success"])

	data := env["data"].(map[string]any)
	require.Equal(t, float64(20), data["limit"])

	comments := data["comments"].([]any)
	require.Len(t, comments, 1)

	first := comments[0].(map[string]any)
	require.Equal(t, contentID.String(), first["target_id"])
	require.Equal(t, "hello from comments", first["body"])
	require.Equal(t, "normal", first["type"])

	author := first["author"].(map[string]any)
	require.Equal(t, userID.String(), author["id"])
	require.Equal(t, "populated-user", author["username"])
	require.NotContains(t, author, "full_name")
}
