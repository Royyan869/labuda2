//go:build integration

package http

// Real-PostgreSQL proof for the canonical Social Content access authority at
// the public OpenGraph embedding surface.
//
// Canonical authority under proof:
//   - /og/content/:id and /content/:id are unauthenticated public share
//     endpoints. They have no viewer identity, so the only visibility class
//     they may project is public.
//   - A private or followers_only row must therefore answer 404 with
//     generic metadata instead of disclosing caption, media or author —
//     and must never be reported as a misleading generic 200 (H.4.6-C).
//   - Moderation (is_hidden) and lifecycle (status/deleted_at, author
//     lifecycle) narrow the public set further.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	pkgdb "github.com/hishumi/backend/pkg/db"
	"github.com/hishumi/backend/pkg/testdb"
)

func seedOGAuthor(t *testing.T, ctx context.Context, pool *pkgdb.DB) uuid.UUID {
	t.Helper()

	userID := uuid.New()
	_, err := pool.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at)
		VALUES ($1, $2, $3, 'active', NOW(), NOW())
	`, userID, "fb-"+userID.String(), userID.String()+"@test.invalid")
	require.NoError(t, err)

	_, err = pool.Pool().Exec(ctx, `
		INSERT INTO user_profiles (id, user_id, username, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
	`, uuid.New(), userID, "og-"+strings.ReplaceAll(userID.String(), "-", "")[:12])
	require.NoError(t, err)

	return userID
}

func seedOGContent(
	t *testing.T,
	ctx context.Context,
	pool *pkgdb.DB,
	authorID uuid.UUID,
	visibility string,
	isHidden bool,
	caption string,
	firstMediaURL string,
) uuid.UUID {
	t.Helper()

	contentID := uuid.New()
	_, err := pool.Pool().Exec(ctx, `
		INSERT INTO contents (id, author_id, status, caption, visibility, is_hidden, created_at, updated_at)
		VALUES ($1, $2, 'active', $3, $4::content_visibility_enum, $5, NOW(), NOW())
	`, contentID, authorID, caption, visibility, isHidden)
	require.NoError(t, err)

	if firstMediaURL != "" {
		_, err = pool.Pool().Exec(ctx, `
			INSERT INTO content_media (content_id, media_url, media_type, position, created_at)
			VALUES ($1, $2, 'image'::media_type_enum, 0, NOW())
		`, contentID, firstMediaURL)
		require.NoError(t, err)
	}

	return contentID
}

func ogContentPreview(t *testing.T, handler *Handler, contentID uuid.UUID) (int, string) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/og/content/:id", handler.GetContent)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/og/content/"+contentID.String(), nil)
	req.Host = "hishumi.com"
	router.ServeHTTP(w, req)

	return w.Code, w.Body.String()
}

// TestOGContentPreview_RealDB_PublicOnly proves the unauthenticated embedding
// surface projects public content only.
func TestOGContentPreview_RealDB_PublicOnly(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	appDB := pkgdb.NewFromPool(tdb.Pool())
	handler := NewHandler(appDB)

	author := seedOGAuthor(t, ctx, appDB)

	const (
		publicCaption         = "PUBLIC-CAPTION-VISIBLE"
		privateCaption        = "PRIVATE-CAPTION-SECRET"
		followersOnlyCaption  = "FOLLOWERS-ONLY-CAPTION-SECRET"
		hiddenCaption         = "HIDDEN-CAPTION-SECRET"
		publicMediaURL        = "https://cdn.test.invalid/public.jpg"
		privateMediaURL       = "https://cdn.test.invalid/private.jpg"
		followersOnlyMediaURL = "https://cdn.test.invalid/followers.jpg"
	)

	publicContent := seedOGContent(t, ctx, appDB, author, "public", false, publicCaption, publicMediaURL)
	privateContent := seedOGContent(t, ctx, appDB, author, "private", false, privateCaption, privateMediaURL)
	followersOnlyContent := seedOGContent(t, ctx, appDB, author, "followers_only", false, followersOnlyCaption, followersOnlyMediaURL)
	hiddenContent := seedOGContent(t, ctx, appDB, author, "public", true, hiddenCaption, "")

	// Public content is genuinely projected (the gate is not blanket-deny).
	publicCode, publicBody := ogContentPreview(t, handler, publicContent)
	require.Equal(t, http.StatusOK, publicCode,
		"a public content must still answer 200 on the OG surface")
	require.Contains(t, publicBody, publicCaption,
		"a public content must still project its caption on the OG surface")
	require.Contains(t, publicBody, publicMediaURL,
		"a public content must still project its first media URL on the OG surface")

	// Non-public content must answer 404 with no disclosure.
	for name, tc := range map[string]struct {
		id      uuid.UUID
		secrets []string
	}{
		"private": {
			id:      privateContent,
			secrets: []string{privateCaption, privateMediaURL},
		},
		"followers_only": {
			id:      followersOnlyContent,
			secrets: []string{followersOnlyCaption, followersOnlyMediaURL},
		},
		"hidden public": {
			id:      hiddenContent,
			secrets: []string{hiddenCaption},
		},
	} {
		code, body := ogContentPreview(t, handler, tc.id)
		require.Equal(t, http.StatusNotFound, code,
			"%s must answer 404, never a misleading generic 200", name)
		require.Contains(t, body, defaultTitle,
			"%s must keep the generic fallback metadata", name)
		for _, secret := range tc.secrets {
			require.NotContains(t, body, secret,
				"%s content must not leak through the unauthenticated OG surface", name)
		}
	}
}
