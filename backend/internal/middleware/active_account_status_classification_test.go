// PASS_17B: locks the HTTP classification RequireActiveAccount produces for
// each canonical account-state error. These tests exercise the real production
// classifier (respondAccountStatusError), not a mock replica: every non-active
// account state must fail closed with a 4xx auth-class response, and only
// genuine lookup/internal failures may remain 500.
package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/stretchr/testify/assert"
)

func runAccountStatusClassification(t *testing.T, err error) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/protected", nil)
	respondAccountStatusError(c, err)
	return w
}

func TestRespondAccountStatusError_Suspended_403(t *testing.T) {
	w := runAccountStatusClassification(t, auth.ErrAccountSuspended)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "ACCOUNT_SUSPENDED")
}

func TestRespondAccountStatusError_Banned_403(t *testing.T) {
	w := runAccountStatusClassification(t, auth.ErrAccountBanned)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "ACCOUNT_BANNED")
}

func TestRespondAccountStatusError_Removed_403(t *testing.T) {
	w := runAccountStatusClassification(t, auth.ErrAccountRemoved)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "ACCOUNT_REMOVED")
}

func TestRespondAccountStatusError_Inactive_403(t *testing.T) {
	w := runAccountStatusClassification(t, auth.ErrAccountInactive)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "ACCOUNT_INACTIVE")
}

func TestRespondAccountStatusError_InvalidCaller_401(t *testing.T) {
	w := runAccountStatusClassification(t, auth.ErrInvalidCaller)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_CALLER")
}

// TestRespondAccountStatusError_WrappedSentinel_Classified proves the switch
// sees through %w wrapping (EnsureActive consumers wrap sentinels).
func TestRespondAccountStatusError_WrappedSentinel_Classified(t *testing.T) {
	w := runAccountStatusClassification(t, fmt.Errorf("guard: %w", auth.ErrAccountRemoved))
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "ACCOUNT_REMOVED")
}

// TestRespondAccountStatusError_InternalError_Remains500 proves a genuine
// DB/internal failure is not reclassified as an account-state rejection.
func TestRespondAccountStatusError_InternalError_Remains500(t *testing.T) {
	w := runAccountStatusClassification(t, errors.New("connection refused"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
