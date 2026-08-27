package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// mockRoleChecker implements auth.RoleChecker for testing
type mockRoleChecker struct {
	isAdmin             bool
	hasSellerCapability bool
	hasSellerProfileVal bool
	adminErr            error
	capabilityErr       error
	sellerProfileErr    error
}

func (m *mockRoleChecker) IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error) {
	return m.isAdmin, m.adminErr
}

func (m *mockRoleChecker) HasActiveSellerCapability(ctx context.Context, userID uuid.UUID) (bool, error) {
	return m.hasSellerCapability, m.capabilityErr
}

func (m *mockRoleChecker) HasSellerProfile(ctx context.Context, userID uuid.UUID) (bool, error) {
	return m.hasSellerProfileVal, m.sellerProfileErr
}

// setupTestContext creates a test gin context with user_id set
func setupTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)
	return c, w
}

func TestRequireAdminMiddleware_Success_AdminUser(t *testing.T) {
	c, w := setupTestContext()
	userID := uuid.New()
	c.Set("user_id", userID)

	roleChecker := &mockRoleChecker{isAdmin: true}
	middleware := RequireAdminMiddleware(roleChecker)
	middleware(c)

	assert.False(t, c.IsAborted(), "Middleware should not abort for admin user")
	assert.Equal(t, http.StatusOK, w.Code)

	isAdmin, exists := c.Get("is_admin")
	assert.True(t, exists)
	assert.True(t, isAdmin.(bool))
}

func TestRequireAdminMiddleware_Forbidden_NonAdminUser(t *testing.T) {
	c, w := setupTestContext()
	userID := uuid.New()
	c.Set("user_id", userID)

	roleChecker := &mockRoleChecker{isAdmin: false}
	middleware := RequireAdminMiddleware(roleChecker)
	middleware(c)

	assert.True(t, c.IsAborted(), "Middleware should abort for non-admin user")
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequireAdminMiddleware_Forbidden_RegularUser(t *testing.T) {
	c, w := setupTestContext()
	userID := uuid.New()
	c.Set("user_id", userID)

	roleChecker := &mockRoleChecker{isAdmin: false}
	middleware := RequireAdminMiddleware(roleChecker)
	middleware(c)

	assert.True(t, c.IsAborted(), "Middleware should abort for regular user")
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequireAdminMiddleware_Unauthorized_NoUserID(t *testing.T) {
	c, w := setupTestContext()
	// Don't set user_id in context

	roleChecker := &mockRoleChecker{isAdmin: true}
	middleware := RequireAdminMiddleware(roleChecker)
	middleware(c)

	assert.True(t, c.IsAborted(), "Middleware should abort when user_id not in context")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireAdminMiddleware_InternalError_AdminCheckFailed(t *testing.T) {
	c, w := setupTestContext()
	userID := uuid.New()
	c.Set("user_id", userID)

	roleChecker := &mockRoleChecker{
		isAdmin:  false,
		adminErr: errors.New("database error"),
	}
	middleware := RequireAdminMiddleware(roleChecker)
	middleware(c)

	assert.True(t, c.IsAborted(), "Middleware should abort on admin check error")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ============================================================================
// RequireSellerProfileMiddleware tests
//
// Doctrine: workspace/payout-prep gate — requires seller profile existence only.
// Expired sellers (hasSellerProfile=true) MUST pass.
// Non-sellers (hasSellerProfile=false) MUST be rejected with 403.
// ============================================================================

func TestRequireSellerProfileMiddleware_Success_SellerWithProfile(t *testing.T) {
	c, w := setupTestContext()
	userID := uuid.New()
	c.Set("user_id", userID)

	roleChecker := &mockRoleChecker{hasSellerProfileVal: true}
	mw := RequireSellerProfileMiddleware(roleChecker)
	mw(c)

	assert.False(t, c.IsAborted(), "Should not abort for seller with profile")
	assert.Equal(t, http.StatusOK, w.Code)

	val, exists := c.Get("has_seller_profile")
	assert.True(t, exists)
	assert.True(t, val.(bool))
	_, sellerKeyExists := c.Get("is_seller")
	assert.False(t, sellerKeyExists, "legacy ambiguous is_seller key must not be emitted")
}

func TestRequireSellerProfileMiddleware_Success_ExpiredSellerAllowed(t *testing.T) {
	// REGRESSION LOCK: expired seller MUST pass profile middleware.
	// hasSellerProfile=true regardless of subscription status.
	c, w := setupTestContext()
	userID := uuid.New()
	c.Set("user_id", userID)

	// Simulate: hasSellerProfile=true, hasSellerCapability=false (expired)
	roleChecker := &mockRoleChecker{
		hasSellerProfileVal: true,
		hasSellerCapability: false,
	}
	mw := RequireSellerProfileMiddleware(roleChecker)
	mw(c)

	assert.False(t, c.IsAborted(), "Expired seller with profile must be allowed through workspace gate")
	assert.Equal(t, http.StatusOK, w.Code)
	_, sellerKeyExists := c.Get("is_seller")
	assert.False(t, sellerKeyExists, "legacy ambiguous is_seller key must not be emitted")
}

func TestRequireSellerProfileMiddleware_Forbidden_NoSellerProfile(t *testing.T) {
	c, w := setupTestContext()
	userID := uuid.New()
	c.Set("user_id", userID)

	roleChecker := &mockRoleChecker{hasSellerProfileVal: false}
	mw := RequireSellerProfileMiddleware(roleChecker)
	mw(c)

	assert.True(t, c.IsAborted(), "Should abort for user without seller profile")
	assert.Equal(t, http.StatusForbidden, w.Code)
	_, sellerKeyExists := c.Get("is_seller")
	assert.False(t, sellerKeyExists, "legacy ambiguous is_seller key must not be emitted")
}

func TestRequireSellerProfileMiddleware_Unauthorized_NoUserID(t *testing.T) {
	c, w := setupTestContext()
	// user_id NOT set in context — simulates unauthenticated request

	roleChecker := &mockRoleChecker{hasSellerProfileVal: true}
	mw := RequireSellerProfileMiddleware(roleChecker)
	mw(c)

	assert.True(t, c.IsAborted(), "Should abort when user_id not in context")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireSellerProfileMiddleware_InternalError_ProfileCheckFailed(t *testing.T) {
	c, w := setupTestContext()
	userID := uuid.New()
	c.Set("user_id", userID)

	roleChecker := &mockRoleChecker{
		hasSellerProfileVal: false,
		sellerProfileErr:    errors.New("database error"),
	}
	mw := RequireSellerProfileMiddleware(roleChecker)
	mw(c)

	assert.True(t, c.IsAborted(), "Should abort on profile check error")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.NotEmpty(t, c.Errors, "internal error should be retained in gin context")
}

// RequireSellerMiddleware still requires active subscription -
// expired seller is rejected by the market gate.
func TestRequireSellerMiddleware_Forbidden_ExpiredSellerRejected(t *testing.T) {
	c, w := setupTestContext()
	userID := uuid.New()
	c.Set("user_id", userID)

	// Simulate expired seller: hasProfile=true but hasCapability=false
	roleChecker := &mockRoleChecker{
		hasSellerProfileVal: true,
		hasSellerCapability: false,
	}
	mw := RequireSellerMiddleware(roleChecker)
	mw(c)

	assert.True(t, c.IsAborted(), "Market gate must reject expired seller")
	assert.Equal(t, http.StatusForbidden, w.Code)
	_, sellerKeyExists := c.Get("is_seller")
	assert.False(t, sellerKeyExists, "legacy ambiguous is_seller key must not be emitted")
}

func TestRequireSellerMiddleware_Success_ActiveSeller(t *testing.T) {
	c, w := setupTestContext()
	userID := uuid.New()
	c.Set("user_id", userID)

	roleChecker := &mockRoleChecker{hasSellerCapability: true}
	mw := RequireSellerMiddleware(roleChecker)
	mw(c)

	assert.False(t, c.IsAborted(), "Active seller must pass market gate")
	assert.Equal(t, http.StatusOK, w.Code)
	val, exists := c.Get("has_market_authority")
	assert.True(t, exists, "market authority key should be emitted")
	assert.True(t, val.(bool), "market authority key must be true")
	_, sellerKeyExists := c.Get("is_seller")
	assert.False(t, sellerKeyExists, "legacy ambiguous is_seller key must not be emitted")
}
