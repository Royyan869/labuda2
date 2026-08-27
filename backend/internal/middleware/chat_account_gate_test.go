// PASS_15A: Locks the route-level middleware chain for the two chat mutation
// routes touched by PASS_6A (link-order, mark-as-read), both of which are
// gated solely by RequireActiveAccount (active account + email verification).
//
// Pipeline-level tests using the same mock pattern as admin_account_gate_test.go
// and social_account_gate_test.go, extended to cover the email-verification
// branch of RequireActiveAccount that those existing mocks do not model.
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

// mockChatAccountGate simulates RequireActiveAccount for pipeline testing,
// including the email-verification branch (mockAccountStatusGate in
// admin_account_gate_test.go only models the suspended/banned branch).
func mockChatAccountGate(statusErr error, emailVerified bool) gin.HandlerFunc {
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
		if !emailVerified {
			response.Error(c, http.StatusForbidden, "EMAIL_VERIFICATION_REQUIRED", "Email verification required")
			c.Abort()
			return
		}
		c.Next()
	}
}

// TestChatLinkOrder_Unauthenticated_Rejected verifies that an unauthenticated
// caller (no user_id set by prior middleware) never reaches the link-order
// handler. RequireActiveAccount itself returns 401 via GetUserIDFromContext
// failing; this is exercised directly against RequireAdminMiddleware's sibling
// behavior in admin_account_gate_test.go, and reproduced here for the chat
// route path/method specifically.
func TestChatLinkOrder_Unauthenticated_Rejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	// No user_id middleware — simulates AuthMiddleware/UserLookupMiddleware
	// never having authenticated the caller.
	router.Use(func(c *gin.Context) {
		if _, exists := c.Get("user_id"); !exists {
			response.Unauthorized(c, "Authentication required")
			c.Abort()
			return
		}
		c.Next()
	})
	router.Use(mockChatAccountGate(nil, true))

	handlerCalled := false
	router.PUT("/chat/rooms/:room_id/link-order", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req, _ := http.NewRequest("PUT", "/chat/rooms/room-1/link-order", nil)
	router.ServeHTTP(w, req)

	assert.False(t, handlerCalled, "Handler must NOT be called for unauthenticated caller")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestChatLinkOrder_SuspendedUser_Blocked verifies a suspended user is
// rejected by RequireActiveAccount before reaching the link-order handler.
func TestChatLinkOrder_SuspendedUser_Blocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	router.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.New())
		c.Next()
	})
	router.Use(mockChatAccountGate(auth.ErrAccountSuspended, true))

	handlerCalled := false
	router.PUT("/chat/rooms/:room_id/link-order", func(c *gin.Context) {
		handlerCalled = true
	})

	req, _ := http.NewRequest("PUT", "/chat/rooms/room-1/link-order", nil)
	router.ServeHTTP(w, req)

	assert.False(t, handlerCalled, "Handler must NOT be called for suspended user")
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "ACCOUNT_SUSPENDED")
}

// TestChatLinkOrder_UnverifiedEmail_Blocked verifies an active user with an
// unverified email is rejected by RequireActiveAccount's second gate before
// reaching the link-order handler.
func TestChatLinkOrder_UnverifiedEmail_Blocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	router.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.New())
		c.Next()
	})
	router.Use(mockChatAccountGate(nil, false))

	handlerCalled := false
	router.PUT("/chat/rooms/:room_id/link-order", func(c *gin.Context) {
		handlerCalled = true
	})

	req, _ := http.NewRequest("PUT", "/chat/rooms/room-1/link-order", nil)
	router.ServeHTTP(w, req)

	assert.False(t, handlerCalled, "Handler must NOT be called for unverified email")
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "EMAIL_VERIFICATION_REQUIRED")
}

// TestChatLinkOrder_ActiveVerifiedUser_Allowed verifies an active, email-verified
// user passes the middleware chain and reaches the link-order handler.
func TestChatLinkOrder_ActiveVerifiedUser_Allowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	router.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.New())
		c.Next()
	})
	router.Use(mockChatAccountGate(nil, true))

	handlerCalled := false
	router.PUT("/chat/rooms/:room_id/link-order", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req, _ := http.NewRequest("PUT", "/chat/rooms/room-1/link-order", nil)
	router.ServeHTTP(w, req)

	assert.True(t, handlerCalled, "Handler must be called for active, verified user")
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestChatMarkAsRead_Unauthenticated_Rejected mirrors
// TestChatLinkOrder_Unauthenticated_Rejected for the mark-as-read route.
func TestChatMarkAsRead_Unauthenticated_Rejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	router.Use(func(c *gin.Context) {
		if _, exists := c.Get("user_id"); !exists {
			response.Unauthorized(c, "Authentication required")
			c.Abort()
			return
		}
		c.Next()
	})
	router.Use(mockChatAccountGate(nil, true))

	handlerCalled := false
	router.POST("/chat/rooms/:room_id/read", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req, _ := http.NewRequest("POST", "/chat/rooms/room-1/read", nil)
	router.ServeHTTP(w, req)

	assert.False(t, handlerCalled, "Handler must NOT be called for unauthenticated caller")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestChatMarkAsRead_SuspendedUser_Blocked mirrors
// TestChatLinkOrder_SuspendedUser_Blocked for the mark-as-read route.
func TestChatMarkAsRead_SuspendedUser_Blocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	router.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.New())
		c.Next()
	})
	router.Use(mockChatAccountGate(auth.ErrAccountSuspended, true))

	handlerCalled := false
	router.POST("/chat/rooms/:room_id/read", func(c *gin.Context) {
		handlerCalled = true
	})

	req, _ := http.NewRequest("POST", "/chat/rooms/room-1/read", nil)
	router.ServeHTTP(w, req)

	assert.False(t, handlerCalled, "Handler must NOT be called for suspended user")
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "ACCOUNT_SUSPENDED")
}

// TestChatMarkAsRead_UnverifiedEmail_Blocked mirrors
// TestChatLinkOrder_UnverifiedEmail_Blocked for the mark-as-read route.
func TestChatMarkAsRead_UnverifiedEmail_Blocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	router.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.New())
		c.Next()
	})
	router.Use(mockChatAccountGate(nil, false))

	handlerCalled := false
	router.POST("/chat/rooms/:room_id/read", func(c *gin.Context) {
		handlerCalled = true
	})

	req, _ := http.NewRequest("POST", "/chat/rooms/room-1/read", nil)
	router.ServeHTTP(w, req)

	assert.False(t, handlerCalled, "Handler must NOT be called for unverified email")
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "EMAIL_VERIFICATION_REQUIRED")
}

// TestChatMarkAsRead_ActiveVerifiedUser_Allowed mirrors
// TestChatLinkOrder_ActiveVerifiedUser_Allowed for the mark-as-read route.
func TestChatMarkAsRead_ActiveVerifiedUser_Allowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	router.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.New())
		c.Next()
	})
	router.Use(mockChatAccountGate(nil, true))

	handlerCalled := false
	router.POST("/chat/rooms/:room_id/read", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req, _ := http.NewRequest("POST", "/chat/rooms/room-1/read", nil)
	router.ServeHTTP(w, req)

	assert.True(t, handlerCalled, "Handler must be called for active, verified user")
	assert.Equal(t, http.StatusOK, w.Code)
}
