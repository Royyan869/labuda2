package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Public browse route boundary tests.
//
// These tests prove the middleware boundary behaviour without needing a real
// database or Firebase connection.  They build a minimal gin router that mirrors
// the v1Browse group pattern from routes_core.go and verify:
//
//   - Anonymous GET (no header) → stub handler runs → 200
//   - Malformed Authorization header → middleware aborts → 401
//   - Non-Bearer scheme → middleware aborts → 401

func buildBrowseBoundaryRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	e := gin.New()

	// Simulate v1Browse group: StrictBrowseAuthMiddleware (nil firebase client)
	// plus a stub handler that returns 200.
	browse := e.Group("/api/v1")
	browse.Use(StrictBrowseAuthMiddleware(nil))
	browse.GET("/listings", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"route": "listings"})
	})
	browse.GET("/listings/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"route": "listing-detail"})
	})
	browse.GET("/auctions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"route": "auctions"})
	})
	browse.GET("/auctions/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"route": "auction-detail"})
	})
	browse.GET("/search/listings", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"route": "search-listings"})
	})
	browse.GET("/search/auctions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"route": "search-auctions"})
	})
	browse.GET("/search/content", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"route": "search-content"})
	})
	browse.GET("/search/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"route": "search-users"})
	})
	browse.GET("/users/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"route": "user-profile"})
	})
	browse.GET("/users/:id/contents", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"route": "user-contents"})
	})
	browse.GET("/contents/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"route": "content-detail"})
	})
	browse.GET("/likes/stats", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"route": "like-stats"})
	})
	return e
}

type browseCase struct {
	name   string
	path   string
	header string // empty = no Authorization header
	want   int
}

var browseCases = []browseCase{
	// Anonymous pass-through cases (no token → 200)
	{"listings no-auth", "/api/v1/listings", "", http.StatusOK},
	{"listing-detail no-auth", "/api/v1/listings/some-id", "", http.StatusOK},
	{"auctions no-auth", "/api/v1/auctions", "", http.StatusOK},
	{"auction-detail no-auth", "/api/v1/auctions/some-uuid", "", http.StatusOK},
	{"search-listings no-auth", "/api/v1/search/listings", "", http.StatusOK},
	{"search-auctions no-auth", "/api/v1/search/auctions", "", http.StatusOK},
	{"search-content no-auth", "/api/v1/search/content", "", http.StatusOK},
	{"search-users no-auth", "/api/v1/search/users", "", http.StatusOK},
	{"user-profile no-auth", "/api/v1/users/some-uuid", "", http.StatusOK},
	{"user-contents no-auth", "/api/v1/users/some-uuid/contents", "", http.StatusOK},
	{"content-detail no-auth", "/api/v1/contents/some-uuid", "", http.StatusOK},
	{"like-stats no-auth", "/api/v1/likes/stats", "", http.StatusOK},

	// Malformed token → 401 (client must clear credentials)
	{"listings invalid-token", "/api/v1/listings", "NotBearer token", http.StatusUnauthorized},
	{"auctions invalid-token", "/api/v1/auctions", "Basic abc123", http.StatusUnauthorized},
	{"user-profile invalid-token", "/api/v1/users/some-uuid", "Bearer", http.StatusUnauthorized},
	{"content-detail invalid-token", "/api/v1/contents/some-uuid", "Bearer foo bar", http.StatusUnauthorized},
}

func TestPublicBrowseRouteBoundary(t *testing.T) {
	engine := buildBrowseBoundaryRouter()

	for _, tc := range browseCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, err := http.NewRequest(http.MethodGet, tc.path, nil)
			require.NoError(t, err)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			engine.ServeHTTP(w, req)
			assert.Equal(t, tc.want, w.Code, "path=%s header=%q", tc.path, tc.header)
		})
	}
}
