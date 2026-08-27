package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// StrictBrowseAuthMiddleware unit tests.
//
// Firebase token verification (cases 3-5 in the middleware spec) is integration-
// tested via the existing AuthMiddleware tests and the live Firebase emulator.
// Here we focus on:
//
//   - Case 1: No Authorization header → 200 (anonymous pass-through), no claims.
//   - Case 2a: Non-Bearer Authorization header → 401.
//   - Case 2b: Authorization: Bearer (no token, only one word) → 401.
//   - Case 2c: Authorization: Bearer foo bar (too many parts) → 401.
//   - Case 5b: Bearer token present but firebase client is nil → 500.

func makeStrictBrowseEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	// nil firebase.Client — safe for cases that never reach VerifyIDToken
	e.Use(StrictBrowseAuthMiddleware(nil))
	e.GET("/browse", func(c *gin.Context) {
		_, exists := GetUserFromContext(c)
		c.JSON(http.StatusOK, gin.H{"claims_present": exists})
	})
	return e
}

// TC-1: No Authorization header → anonymous pass-through (HTTP 200, no claims).
func TestStrictBrowseAuth_NoAuthHeader_AnonymousPassThrough(t *testing.T) {
	engine := makeStrictBrowseEngine()

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/browse", nil)
	require.NoError(t, err)

	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "no Authorization header must pass through (anonymous)")
	assert.Contains(t, w.Body.String(), `"claims_present":false`, "context must not contain user claims for anonymous request")
}

// TC-2a: Non-Bearer scheme in Authorization header → 401.
func TestStrictBrowseAuth_NonBearerScheme_Returns401(t *testing.T) {
	engine := makeStrictBrowseEngine()

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/browse", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")

	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "non-Bearer Authorization must return 401")
}

// TC-2b: "Authorization: Bearer" with no token (single word) → 401.
func TestStrictBrowseAuth_BearerNoToken_Returns401(t *testing.T) {
	engine := makeStrictBrowseEngine()

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/browse", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer")

	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "Authorization: Bearer (no token) must return 401")
}

// TC-2c: "Authorization: Bearer foo bar" (three parts) → 401.
func TestStrictBrowseAuth_BearerThreeParts_Returns401(t *testing.T) {
	engine := makeStrictBrowseEngine()

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/browse", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer foo bar")

	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "Authorization: Bearer foo bar (three parts) must return 401")
}

// TC-5b: Valid Bearer format but nil Firebase client → 500 (not configured).
// This proves the nil-client guard fires before VerifyIDToken.
func TestStrictBrowseAuth_NilFirebaseClient_Returns500(t *testing.T) {
	engine := makeStrictBrowseEngine() // already uses nil firebase client

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/browse", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer looks.like.a.valid.token")

	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code, "nil Firebase client with a token must return 500")
}

// TC-abort: ensure middleware calls c.Abort() on rejection (subsequent handlers not called).
func TestStrictBrowseAuth_Abort_OnMalformedHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handlerCalled := false

	e := gin.New()
	e.Use(StrictBrowseAuthMiddleware(nil))
	e.GET("/abort-test", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/abort-test", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "NotBearer sometoken")

	e.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, handlerCalled, "downstream handler must not be called when middleware aborts")
}
