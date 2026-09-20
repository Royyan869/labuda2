// Proves that social mutation routes (like toggle, for_sale-reference comment)
// reject suspended/banned users via RequireActiveAccount middleware.
//
// Pipeline-level tests using the same mock pattern as admin_account_gate_test.go.
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/stretchr/testify/assert"
)

// TestLikeToggle_ActiveUser_Allowed verifies an active user reaches the like handler.
func TestLikeToggle_ActiveUser_Allowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	router.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.New())
		c.Next()
	})
	router.Use(mockAccountStatusGate(nil))

	handlerCalled := false
	router.POST("/likes/toggle", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req, _ := http.NewRequest("POST", "/likes/toggle", nil)
	router.ServeHTTP(w, req)

	assert.True(t, handlerCalled, "Handler must be called for active user")
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestLikeToggle_SuspendedUser_Blocked verifies a suspended user is rejected with 403.
func TestLikeToggle_SuspendedUser_Blocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	router.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.New())
		c.Next()
	})
	router.Use(mockAccountStatusGate(auth.ErrAccountSuspended))

	handlerCalled := false
	router.POST("/likes/toggle", func(c *gin.Context) {
		handlerCalled = true
	})

	req, _ := http.NewRequest("POST", "/likes/toggle", nil)
	router.ServeHTTP(w, req)

	assert.False(t, handlerCalled, "Handler must NOT be called for suspended user")
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "ACCOUNT_SUSPENDED")
}

// TestLikeToggle_BannedUser_Blocked verifies a banned user is rejected with 403.
func TestLikeToggle_BannedUser_Blocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	router.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.New())
		c.Next()
	})
	router.Use(mockAccountStatusGate(auth.ErrAccountBanned))

	handlerCalled := false
	router.POST("/likes/toggle", func(c *gin.Context) {
		handlerCalled = true
	})

	req, _ := http.NewRequest("POST", "/likes/toggle", nil)
	router.ServeHTTP(w, req)

	assert.False(t, handlerCalled, "Handler must NOT be called for banned user")
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "ACCOUNT_BANNED")
}

// TestForSaleComment_ActiveUser_Allowed verifies an active user reaches the for_sale comment handler.
func TestForSaleComment_ActiveUser_Allowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	router.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.New())
		c.Next()
	})
	router.Use(mockAccountStatusGate(nil))

	handlerCalled := false
	router.POST("/contents/:id/comments/for_sale", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req, _ := http.NewRequest("POST", "/contents/abc/comments/listing", nil)
	router.ServeHTTP(w, req)

	assert.True(t, handlerCalled, "Handler must be called for active user")
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestForSaleComment_SuspendedUser_Blocked verifies a suspended user is rejected with 403.
func TestForSaleComment_SuspendedUser_Blocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	router.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.New())
		c.Next()
	})
	router.Use(mockAccountStatusGate(auth.ErrAccountSuspended))

	handlerCalled := false
	router.POST("/contents/:id/comments/for_sale", func(c *gin.Context) {
		handlerCalled = true
	})

	req, _ := http.NewRequest("POST", "/contents/abc/comments/listing", nil)
	router.ServeHTTP(w, req)

	assert.False(t, handlerCalled, "Handler must NOT be called for suspended user")
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "ACCOUNT_SUSPENDED")
}

// TestForSaleComment_BannedUser_Blocked verifies a banned user is rejected with 403.
func TestForSaleComment_BannedUser_Blocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	router.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.New())
		c.Next()
	})
	router.Use(mockAccountStatusGate(auth.ErrAccountBanned))

	handlerCalled := false
	router.POST("/contents/:id/comments/for_sale", func(c *gin.Context) {
		handlerCalled = true
	})

	req, _ := http.NewRequest("POST", "/contents/abc/comments/listing", nil)
	router.ServeHTTP(w, req)

	assert.False(t, handlerCalled, "Handler must NOT be called for banned user")
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "ACCOUNT_BANNED")
}


