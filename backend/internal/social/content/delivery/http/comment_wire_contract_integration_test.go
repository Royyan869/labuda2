//go:build integration

package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	contentapp "github.com/labuda/backend/internal/social/content/application"
	contentrepo "github.com/labuda/backend/internal/social/content/infrastructure/repository"
	idempotencyRepo "github.com/labuda/backend/internal/platform/idempotency/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// commentWireTestOutbox is a no-op OutboxInserter so AddComment's event
// emission does not require real outbox rows in these integration tests.
type commentWireTestOutbox struct{}

func (commentWireTestOutbox) InsertTx(
	ctx context.Context,
	tx db.Tx,
	eventType string,
	payload any,
	idempotencyKey string,
) error {
	return nil
}

// newCommentWireTestHandler builds a CommentHandler whose CommentService is
// wired with the production idempotency repository (C-IPC) so the canonical
// Idempotency-Key enforcement is exercised end-to-end.
func newCommentWireTestHandler(appDB *db.DB) *CommentHandler {
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
		nil,                             // fpsValidator
		commentWireTestOutbox{},         // outboxRepo (no-op)
		nil,                             // blockChecker
		nil,                             // invariantLogger
		idempotencyRepo.NewRepository(), // idempotencyRepo — wired (C-IPC)
	)
	return NewCommentHandler(commentService, contentService, appDB, zap.NewNop())
}

// newCommentWireRouter registers the canonical comment routes with a fixed
// authenticated user, bypassing AuthMiddleware (handlers read userID from ctx).
func newCommentWireRouter(handler *CommentHandler, userID uuid.UUID) *gin.Engine {
	router := gin.New()
	router.POST("/contents/:id/comments", func(c *gin.Context) {
		c.Set("userID", userID)
		handler.CreateComment(c)
	})
	router.GET("/contents/:id/comments", handler.ListComments)
	router.DELETE("/comments/:id", func(c *gin.Context) {
		c.Set("userID", userID)
		handler.DeleteComment(c)
	})
	return router
}

func performWireCreateComment(
	t *testing.T,
	router *gin.Engine,
	contentID, body, parentID, key string,
) *httptest.ResponseRecorder {
	t.Helper()
	reqBody := map[string]any{"body": body}
	if parentID != "" {
		reqBody["parent_id"] = parentID
	}
	raw, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/contents/"+contentID+"/comments", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", key)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func performWireDeleteComment(t *testing.T, router *gin.Engine, commentID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, "/comments/"+commentID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func performWireListComments(t *testing.T, router *gin.Engine, contentID, cursor string, limit int) *httptest.ResponseRecorder {
	t.Helper()
	path := "/contents/" + contentID + "/comments"
	req := httptest.NewRequest(http.MethodGet, path, nil)
	q := req.URL.Query()
	q.Set("limit", strconv.Itoa(limit))
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	req.URL.RawQuery = q.Encode()
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// decodeWireCreateResponse parses the create response envelope and returns
// the comment payload as a generic map plus the canonical fields.
func decodeWireCreateResponse(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var env struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
		Error   *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	var data map[string]any
	require.NoError(t, json.Unmarshal(env.Data, &data))
	return data
}

func wireCommentID(t *testing.T, data map[string]any) string {
	t.Helper()
	id, ok := data["id"].(string)
	require.True(t, ok, "created comment missing id: %v", data)
	return id
}

func wireCommentCount(t *testing.T, appDB *db.DB, contentID uuid.UUID) int {
	t.Helper()
	var count int
	err := appDB.WithTx(context.Background(), func(tx db.Tx) error {
		var cErr error
		count, cErr = contentrepo.NewCommentRepository().CountTopLevelCommentsByContent(context.Background(), tx, contentID)
		return cErr
	})
	require.NoError(t, err)
	return count
}

func TestCommentWire_CreateReturnsSnakeCaseCommentResponse_AndEnforcesIdempotency(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	appDB := db.NewFromPool(tdb.Pool())

	ctx := context.Background()
	authorID := seedCommentListHTTPUser(t, ctx, appDB, "wire-author")
	contentID := seedCommentListHTTPContent(t, ctx, appDB, newCommentWireTestHandler(appDB), authorID)

	handler := newCommentWireTestHandler(appDB)
	router := newCommentWireRouter(handler, authorID)

	// 1) Top-level create → 201 + snake_case CommentResponse.
	w := performWireCreateComment(t, router, contentID.String(), "hello wire", "", "key-create-1")
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	data := decodeWireCreateResponse(t, w)
	require.NotEmpty(t, data["id"])
	require.Equal(t, contentID.String(), data["target_id"])
	require.Equal(t, authorID.String(), data["author_id"])
	require.Equal(t, "normal", data["type"])
	require.Nil(t, data["parent_id"])
	// Wire contract: no raw entity PascalCase keys leak.
	for _, pascal := range []string{"TargetID", "AuthorID", "CreatedAt", "Body"} {
		_, present := data[pascal]
		require.False(t, present, "PascalCase key leaked into wire: %s", pascal)
	}
	createdID := wireCommentID(t, data)

	// 2) Same key + same operation → replay, no new row.
	w2 := performWireCreateComment(t, router, contentID.String(), "hello wire", "", "key-create-1")
	require.Equal(t, http.StatusCreated, w2.Code, "body=%s", w2.Body.String())
	replayed := decodeWireCreateResponse(t, w2)
	require.Equal(t, createdID, wireCommentID(t, replayed), "replay must return the original comment")
	require.Equal(t, 1, wireCommentCount(t, appDB, contentID))

	// 3) Same key + different payload → idempotency conflict (409).
	w3 := performWireCreateComment(t, router, contentID.String(), "different body", "", "key-create-1")
	require.Equal(t, http.StatusConflict, w3.Code, "body=%s", w3.Body.String())
	require.Equal(t, 1, wireCommentCount(t, appDB, contentID))

	// 4) Same key + different actor → idempotency conflict (409).
	otherID := seedCommentListHTTPUser(t, ctx, appDB, "wire-other")
	otherRouter := newCommentWireRouter(handler, otherID)
	w4 := performWireCreateComment(t, otherRouter, contentID.String(), "hello wire", "", "key-create-1")
	require.Equal(t, http.StatusConflict, w4.Code, "body=%s", w4.Body.String())
	require.Equal(t, 1, wireCommentCount(t, appDB, contentID))
}

func TestCommentWire_ListCursorAsc_NoDupNoOmission_DeleteAffectsListAndCount(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	appDB := db.NewFromPool(tdb.Pool())

	ctx := context.Background()
	authorID := seedCommentListHTTPUser(t, ctx, appDB, "wire-list-author")
	handler := newCommentWireTestHandler(appDB)
	contentID := seedCommentListHTTPContent(t, ctx, appDB, handler, authorID)
	router := newCommentWireRouter(handler, authorID)

	// Create 5 top-level comments in a known order (sequential keys, spaced
	// timestamps so created_at ASC is deterministic).
	ids := make([]string, 0, 5)
	for i := 0; i < 5; i++ {
		w := performWireCreateComment(t, router, contentID.String(), "comment-"+strconv.Itoa(i), "", "key-list-"+strconv.Itoa(i))
		require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
		ids = append(ids, wireCommentID(t, decodeWireCreateResponse(t, w)))
		time.Sleep(15 * time.Millisecond)
	}

	// Cursor walk with limit=2 must surface every comment once, in ASC order.
	var collected []string
	cursor := ""
	for {
		w := performWireListComments(t, router, contentID.String(), cursor, 2)
		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		var env struct {
			Data struct {
				Comments []struct {
					ID string `json:"id"`
				} `json:"comments"`
				NextCursor *string `json:"next_cursor"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
		for _, c := range env.Data.Comments {
			collected = append(collected, c.ID)
		}
		cursor = ""
		if env.Data.NextCursor != nil {
			cursor = *env.Data.NextCursor
		}
		if cursor == "" {
			break
		}
	}
	require.Equal(t, ids, collected, "cursor walk must return all comments once, in ASC order, no dup/omission")

	require.Equal(t, 5, wireCommentCount(t, appDB, contentID))

	// Delete the middle comment (author-only soft delete).
	wDel := performWireDeleteComment(t, router, ids[2])
	require.Equal(t, http.StatusOK, wDel.Code, "body=%s", wDel.Body.String())

	// List must omit the deleted comment.
	wAfter := performWireListComments(t, router, contentID.String(), "", 50)
	require.Equal(t, http.StatusOK, wAfter.Code)
	var envAfter struct {
		Data struct {
			Comments []struct {
				ID string `json:"id"`
			} `json:"comments"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(wAfter.Body.Bytes(), &envAfter))
	seen := make(map[string]bool, len(envAfter.Data.Comments))
	for _, c := range envAfter.Data.Comments {
		seen[c.ID] = true
		require.NotEqual(t, ids[2], c.ID, "deleted comment must disappear from list")
	}
	require.Len(t, envAfter.Data.Comments, 4)

	// Count equals top-level non-deleted rows → decrement after delete.
	require.Equal(t, 4, wireCommentCount(t, appDB, contentID))

	// Replay the deleted key: idempotent replay returns the original row; the
	// row count must not increase.
	wReplay := performWireCreateComment(t, router, contentID.String(), "comment-2", "", "key-list-2")
	require.Equal(t, http.StatusCreated, wReplay.Code, "body=%s", wReplay.Body.String())
	require.Equal(t, ids[2], wireCommentID(t, decodeWireCreateResponse(t, wReplay)))
	require.Equal(t, 4, wireCommentCount(t, appDB, contentID))
}

func TestCommentWire_ReplyRules_GroupingWire_CountExcludesReplies(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	appDB := db.NewFromPool(tdb.Pool())

	ctx := context.Background()
	authorID := seedCommentListHTTPUser(t, ctx, appDB, "wire-reply-author")
	otherID := seedCommentListHTTPUser(t, ctx, appDB, "wire-reply-other")
	handler := newCommentWireTestHandler(appDB)
	contentID := seedCommentListHTTPContent(t, ctx, appDB, handler, authorID)
	secondContentID := seedCommentListHTTPContent(t, ctx, appDB, handler, authorID)
	authorRouter := newCommentWireRouter(handler, authorID)
	otherRouter := newCommentWireRouter(handler, otherID)

	// Top-level comment from author.
	wTop := performWireCreateComment(t, authorRouter, contentID.String(), "top", "", "key-reply-top")
	require.Equal(t, http.StatusCreated, wTop.Code)
	topID := wireCommentID(t, decodeWireCreateResponse(t, wTop))

	// Reply from another user → parent_id present on the wire (grouping hook).
	wReply := performWireCreateComment(t, otherRouter, contentID.String(), "reply", topID, "key-reply-1")
	require.Equal(t, http.StatusCreated, wReply.Code, "body=%s", wReply.Body.String())
	replyData := decodeWireCreateResponse(t, wReply)
	require.Equal(t, topID, replyData["parent_id"])

	// Reply-to-reply is rejected (depth max = 1).
	wReply2 := performWireCreateComment(t, otherRouter, contentID.String(), "reply2depth", wireCommentID(t, replyData), "key-reply-2")
	require.Equal(t, http.StatusBadRequest, wReply2.Code, "body=%s", wReply2.Body.String())

	// Cross-content parent is rejected.
	wCross := performWireCreateComment(t, otherRouter, secondContentID.String(), "cross", topID, "key-reply-3")
	require.Equal(t, http.StatusBadRequest, wCross.Code, "body=%s", wCross.Body.String())

	// Count excludes replies: 1 top-level (top) counted, reply not counted.
	require.Equal(t, 1, wireCommentCount(t, appDB, contentID))

	// Reply to deleted parent is rejected.
	wDel := performWireDeleteComment(t, authorRouter, topID)
	require.Equal(t, http.StatusOK, wDel.Code)
	wDelReply := performWireCreateComment(t, otherRouter, contentID.String(), "deletedparent", topID, "key-reply-4")
	require.Equal(t, http.StatusBadRequest, wDelReply.Code, "body=%s", wDelReply.Body.String())
	// Deleting the parent removes the top-level row from the count.
	require.Equal(t, 0, wireCommentCount(t, appDB, contentID))
}