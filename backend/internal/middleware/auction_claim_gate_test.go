// Proves that the auction winner claim route (/auctions/:id/claim) rejects
// suspended/banned users via RequireActiveAccount middleware.
//
// Pipeline-level tests using the same mock pattern as social_account_gate_test.go.
// Locks the authority contract: claiming an auction (which resolves shipping,
// creates an order and initiates payment) requires an active, email-verified
// account — matching the identical constraint already on POST /auctions/:id/bid
// and POST /orders.
//
// Before this gate was added, a suspended or banned winner could create an
// order without being blocked.
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

// --- /auctions/:id/claim ---

func TestAuctionClaim_ActiveUser_Allowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	router.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.New())
		c.Next()
	})
	router.Use(mockAccountStatusGate(nil))

	handlerCalled := false
	router.POST("/auctions/:id/claim", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req, _ := http.NewRequest("POST", "/auctions/"+uuid.New().String()+"/claim", nil)
	router.ServeHTTP(w, req)

	assert.True(t, handlerCalled, "Handler must be called for active user")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuctionClaim_SuspendedUser_Blocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	router.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.New())
		c.Next()
	})
	router.Use(mockAccountStatusGate(auth.ErrAccountSuspended))

	handlerCalled := false
	router.POST("/auctions/:id/claim", func(c *gin.Context) {
		handlerCalled = true
	})

	req, _ := http.NewRequest("POST", "/auctions/"+uuid.New().String()+"/claim", nil)
	router.ServeHTTP(w, req)

	assert.False(t, handlerCalled, "Handler must NOT be called for suspended user")
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "ACCOUNT_SUSPENDED")
}

func TestAuctionClaim_BannedUser_Blocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	router.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.New())
		c.Next()
	})
	router.Use(mockAccountStatusGate(auth.ErrAccountBanned))

	handlerCalled := false
	router.POST("/auctions/:id/claim", func(c *gin.Context) {
		handlerCalled = true
	})

	req, _ := http.NewRequest("POST", "/auctions/"+uuid.New().String()+"/claim", nil)
	router.ServeHTTP(w, req)

	assert.False(t, handlerCalled, "Handler must NOT be called for banned user")
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "ACCOUNT_BANNED")
}


