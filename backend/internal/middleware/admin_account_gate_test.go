// ID1C: Admin account status gate tests.
//
// Proves that the admin middleware chain rejects suspended/banned users
// BEFORE reaching RequireAdminMiddleware or capability checks.
//
// These are pipeline-level tests that simulate the RequireActiveAccount
// behavior without a real database, using a mock account status gate.
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/stretchr/testify/assert"
)

// mockAccountStatusGate simulates RequireActiveAccount for pipeline testing.
// Returns the same HTTP responses as the real middleware.
func mockAccountStatusGate(statusErr error) gin.HandlerFunc {
	return func(c *gin.Context) {
		if statusErr != nil {
			switch statusErr {
			case auth.ErrAccountSuspended:
				response.Error(c, http.StatusForbidden, "ACCOUNT_SUSPENDED", "Your account has been suspended.")
			case auth.ErrAccountBanned:
				response.Error(c, http.StatusForbidden, "ACCOUNT_BANNED", "Your account has been banned.")
			default:
				response.InternalServerError(c, "Failed to verify account status")
			}
			c.Abort()
			return
		}
		c.Next()
	}
}

// TestAdminChain_ActiveAdmin_Allowed verifies an active admin reaches the handler.
func TestAdminChain_ActiveAdmin_Allowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	userID := uuid.New()

	// Simulate: UserLookup sets user_id
	router.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	})
	// RequireActiveAccount (active → passes)
	router.Use(mockAccountStatusGate(nil))
	// RequireAdminMiddleware
	router.Use(RequireAdminMiddleware(&mockRoleChecker{isAdmin: true}))

	handlerCalled := false
	router.GET("/admin/test", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req, _ := http.NewRequest("GET", "/admin/test", nil)
	router.ServeHTTP(w, req)

	assert.True(t, handlerCalled, "Handler should be called for active admin")
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestAdminChain_SuspendedAdmin_Rejected verifies a suspended admin is
// blocked by RequireActiveAccount BEFORE RequireAdminMiddleware runs.
func TestAdminChain_SuspendedAdmin_Rejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	userID := uuid.New()

	router.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	})
	// RequireActiveAccount (suspended → rejects)
	router.Use(mockAccountStatusGate(auth.ErrAccountSuspended))
	// RequireAdminMiddleware should NEVER be reached
	router.Use(RequireAdminMiddleware(&mockRoleChecker{isAdmin: true}))

	handlerCalled := false
	router.GET("/admin/test", func(c *gin.Context) {
		handlerCalled = true
	})

	req, _ := http.NewRequest("GET", "/admin/test", nil)
	router.ServeHTTP(w, req)

	assert.False(t, handlerCalled, "Handler must NOT be called for suspended admin")
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "ACCOUNT_SUSPENDED")
}

// TestAdminChain_BannedAdmin_Rejected verifies a banned admin is
// blocked by RequireActiveAccount BEFORE RequireAdminMiddleware runs.
func TestAdminChain_BannedAdmin_Rejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	userID := uuid.New()

	router.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	})
	// RequireActiveAccount (banned → rejects)
	router.Use(mockAccountStatusGate(auth.ErrAccountBanned))
	// RequireAdminMiddleware should NEVER be reached
	router.Use(RequireAdminMiddleware(&mockRoleChecker{isAdmin: true}))

	handlerCalled := false
	router.GET("/admin/test", func(c *gin.Context) {
		handlerCalled = true
	})

	req, _ := http.NewRequest("GET", "/admin/test", nil)
	router.ServeHTTP(w, req)

	assert.False(t, handlerCalled, "Handler must NOT be called for banned admin")
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "ACCOUNT_BANNED")
}

// TestAdminChain_NonAdmin_StillRejected verifies that a non-admin active user
// passes RequireActiveAccount but is stopped by RequireAdminMiddleware.
func TestAdminChain_NonAdmin_StillRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	userID := uuid.New()

	router.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	})
	// RequireActiveAccount (active → passes)
	router.Use(mockAccountStatusGate(nil))
	// RequireAdminMiddleware (not admin → rejects)
	router.Use(RequireAdminMiddleware(&mockRoleChecker{isAdmin: false}))

	handlerCalled := false
	router.GET("/admin/test", func(c *gin.Context) {
		handlerCalled = true
	})

	req, _ := http.NewRequest("GET", "/admin/test", nil)
	router.ServeHTTP(w, req)

	assert.False(t, handlerCalled, "Handler must NOT be called for non-admin user")
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestAdminChain_NoUserID_Rejected verifies that missing user_id
// causes RequireActiveAccount to fail (no user_id → 401).
func TestAdminChain_NoUserID_Rejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	// No user_id middleware — simulates deleted user (UserLookup rejects)
	// In production, UserLookupMiddleware would abort before this point.
	// This test ensures RequireAdminMiddleware also handles missing user_id.
	router.Use(RequireAdminMiddleware(&mockRoleChecker{isAdmin: true}))

	handlerCalled := false
	router.GET("/admin/test", func(c *gin.Context) {
		handlerCalled = true
	})

	req, _ := http.NewRequest("GET", "/admin/test", nil)
	router.ServeHTTP(w, req)

	assert.False(t, handlerCalled, "Handler must NOT be called without user_id")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}


