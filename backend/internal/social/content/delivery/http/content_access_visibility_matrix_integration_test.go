//go:build integration

package http

// Real-PostgreSQL proof for the canonical Social Content access authority at
// the HTTP boundary. One test binary, one schema bootstrap, real rows.
//
// Canonical authority under proof:
//   - contents.visibility (public / followers_only / private) is the only
//     business authority deciding who may access a content. A private row is
//     denied to everyone but the owner even when is_hidden = false.
//   - No access to the parent content means no access to its comments, on BOTH
//     the comment list and comment create surfaces.
//   - Bidirectional block (viewer blocks author OR author blocks viewer) is
//     denied consistently.
//   - Reposting / sharing must never create a carrier row pointing at a source
//     the viewer cannot access.

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	contententity "github.com/labuda/backend/internal/social/content/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

func seedMatrixContent(
	t *testing.T,
	ctx context.Context,
	pool *db.DB,
	authorID uuid.UUID,
	visibility contententity.Visibility,
	isHidden bool,
) uuid.UUID {
	t.Helper()

	contentID := uuid.New()
	_, err := pool.Pool().Exec(ctx, `
		INSERT INTO contents (id, author_id, status, caption, visibility, is_hidden, created_at, updated_at)
		VALUES ($1, $2, 'active', 'matrix content', $3::content_visibility_enum, $4, NOW(), NOW())
	`, contentID, authorID, string(visibility), isHidden)
	require.NoError(t, err)

	return contentID
}

func seedMatrixFollow(t *testing.T, ctx context.Context, pool *db.DB, followerID, followingID uuid.UUID) {
	t.Helper()

	_, err := pool.Pool().Exec(ctx, `
		INSERT INTO user_follows (follower_id, following_id, created_at)
		VALUES ($1, $2, NOW())
	`, followerID, followingID)
	require.NoError(t, err)
}

func seedMatrixBlock(t *testing.T, ctx context.Context, pool *db.DB, blockerID, blockedID uuid.UUID) {
	t.Helper()

	_, err := pool.Pool().Exec(ctx, `
		INSERT INTO user_blocks (blocker_id, blocked_id, created_at)
		VALUES ($1, $2, NOW())
	`, blockerID, blockedID)
	require.NoError(t, err)
}

func getContentAsViewer(
	t *testing.T,
	handler *ContentHandler,
	contentID uuid.UUID,
	viewerID uuid.UUID,
) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/contents/"+contentID.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: contentID.String()}}
	c.Set("userID", viewerID)

	handler.GetContent(c)
	return w
}

func postCommentAsViewer(
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
	router.POST("/api/v1/contents/:id/comments", handler.CreateComment)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/contents/"+contentID.String()+"/comments",
		strings.NewReader(`{"body":"matrix comment"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", uuid.New().String())
	router.ServeHTTP(w, req)
	return w
}

func postRepostAsViewer(
	t *testing.T,
	handler *ContentHandler,
	targetContentID uuid.UUID,
	viewerID uuid.UUID,
) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("userID", viewerID)
		c.Next()
	})
	router.POST("/api/v1/contents/:id/repost", handler.RepostContent)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/contents/"+targetContentID.String()+"/repost",
		strings.NewReader(fmt.Sprintf(
			`{"original_content_id":%q,"caption":"matrix repost"}`, targetContentID.String(),
		)),
	)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	return w
}

func countRepostCarriers(t *testing.T, ctx context.Context, pool *db.DB, originalAuthorID uuid.UUID) int {
	t.Helper()

	var count int
	require.NoError(t, pool.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM contents WHERE original_author_id = $1`, originalAuthorID,
	).Scan(&count))
	return count
}

// TestContentAccessVisibilityMatrix_RealDB proves the access matrix with real
// rows across the detail, comment list, comment create and repost surfaces.
func TestContentAccessVisibilityMatrix_RealDB(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	contentHandler := newVisibilityHTTPHandlerFromPool(appDB)
	commentHandler := newCommentListHTTPHandler(appDB)

	author := seedCommentListHTTPUser(t, ctx, appDB, "matrix-author")
	follower := seedCommentListHTTPUser(t, ctx, appDB, "matrix-follower")
	stranger := seedCommentListHTTPUser(t, ctx, appDB, "matrix-stranger")
	blockedViewer := seedCommentListHTTPUser(t, ctx, appDB, "matrix-blocked")

	seedMatrixFollow(t, ctx, appDB, follower, author)
	seedMatrixBlock(t, ctx, appDB, author, blockedViewer)

	privateContent := seedMatrixContent(t, ctx, appDB, author, contententity.VisibilityPrivate, false)
	followersOnlyContent := seedMatrixContent(t, ctx, appDB, author, contententity.VisibilityFollowersOnly, false)
	publicContent := seedMatrixContent(t, ctx, appDB, author, contententity.VisibilityPublic, false)

	t.Run("detail honors visibility", func(t *testing.T) {
		require.Equal(t, http.StatusNotFound, getContentAsViewer(t, contentHandler, privateContent, stranger).Code,
			"private detail must be hidden from a non-owner even when is_hidden = false")
		require.Equal(t, http.StatusOK, getContentAsViewer(t, contentHandler, privateContent, author).Code,
			"owner must still read own private content")
		require.Equal(t, http.StatusNotFound, getContentAsViewer(t, contentHandler, followersOnlyContent, stranger).Code,
			"followers_only detail must be hidden from a non-follower")
		require.Equal(t, http.StatusOK, getContentAsViewer(t, contentHandler, followersOnlyContent, follower).Code,
			"followers_only detail must be readable by a follower")
	})

	t.Run("comment list uses the same parent authority", func(t *testing.T) {
		require.Equal(t, http.StatusNotFound, performCommentListRequest(t, commentHandler, privateContent, stranger).Code,
			"non-owner must not enumerate comments of a private parent")
		require.Equal(t, http.StatusNotFound, performCommentListRequest(t, commentHandler, followersOnlyContent, stranger).Code,
			"non-follower must not enumerate comments of a followers_only parent")
		require.Equal(t, http.StatusOK, performCommentListRequest(t, commentHandler, followersOnlyContent, follower).Code,
			"follower must keep access to comments of a followers_only parent")
		require.Equal(t, http.StatusNotFound, performCommentListRequest(t, commentHandler, publicContent, blockedViewer).Code,
			"bidirectional block must deny the comment list on a public parent")
	})

	t.Run("comment create uses the same parent authority", func(t *testing.T) {
		require.Equal(t, http.StatusBadRequest, postCommentAsViewer(t, commentHandler, privateContent, stranger).Code,
			"non-owner must not comment on a private parent")
		require.Equal(t, http.StatusBadRequest, postCommentAsViewer(t, commentHandler, followersOnlyContent, stranger).Code,
			"non-follower must not comment on a followers_only parent")
		require.Equal(t, http.StatusCreated, postCommentAsViewer(t, commentHandler, followersOnlyContent, follower).Code,
			"follower must be able to comment on a followers_only parent")
		require.Equal(t, http.StatusBadRequest, postCommentAsViewer(t, commentHandler, publicContent, blockedViewer).Code,
			"bidirectional block must deny comment creation via the canonical service authorization")
	})

	t.Run("repost target authorization is viewer-aware", func(t *testing.T) {
		before := countRepostCarriers(t, ctx, appDB, author)

		// Control case first: the same endpoint, same caller, same payload —
		// only the source's access class differs. If this is not 201 the gate
		// is not proven, only broken.
		allowedRepost := postRepostAsViewer(t, contentHandler, publicContent, stranger)
		require.Equal(t, http.StatusCreated, allowedRepost.Code,
			"an accessible public source must be repostable (gate is not blanket-deny), body=%s", allowedRepost.Body.String())
		require.Equal(t, before+1, countRepostCarriers(t, ctx, appDB, author),
			"the accessible repost must create exactly one carrier row")

		for name, tc := range map[string]struct {
			contentID uuid.UUID
			viewerID  uuid.UUID
		}{
			"private source, non-owner":           {contentID: privateContent, viewerID: stranger},
			"followers_only source, non-follower": {contentID: followersOnlyContent, viewerID: stranger},
			"public source, blocked viewer":       {contentID: publicContent, viewerID: blockedViewer},
		} {
			w := postRepostAsViewer(t, contentHandler, tc.contentID, tc.viewerID)
			require.NotEqual(t, http.StatusCreated, w.Code,
				"%s must not be repostable (status=%d), body=%s", name, w.Code, w.Body.String())
		}

		require.Equal(t, before+1, countRepostCarriers(t, ctx, appDB, author),
			"no repost carrier row may be created for an inaccessible source")
	})
}
