package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

	// Optional seller-subscription status reader (RF-02 residual). When
	// subscriptionStatusSet is false, the mock does NOT implement the reader
	// interface, exercising the fail-closed activation copy path.
	subscriptionStatusSet bool
	subscriptionStatus    string
	subscriptionStatusErr error
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

// GetSellerSubscriptionStatus is only promoted onto the concrete type when the
// test opts in via subscriptionStatusSet, so type-assertion fail-closed is
// still exercised by the default mock.

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

// ============================================================================
// RF-02 residual — market-authority denial message must distinguish
// never-paid ('none') from ended ('expired'). Never-paid must never be
// told to renew.
// ============================================================================

// statusReportingRoleChecker wraps mockRoleChecker and promotes the optional
// GetSellerSubscriptionStatus reader so type-assertion succeeds.
type statusReportingRoleChecker struct {
	*mockRoleChecker
	status    string
	statusErr error
}

func (r *statusReportingRoleChecker) GetSellerSubscriptionStatus(_ context.Context, _ uuid.UUID) (string, error) {
	return r.status, r.statusErr
}

func TestRequireSellerMiddleware_NeverPaid_IsToldToActivateNotRenew(t *testing.T) {
	c, w := setupTestContext()
	userID := uuid.New()
	c.Set("user_id", userID)

	// Freshly onboarded seller: profile exists, no subscription row → 'none'.
	roleChecker := &statusReportingRoleChecker{
		mockRoleChecker: &mockRoleChecker{
			hasSellerProfileVal: true,
			hasSellerCapability: false,
		},
		status: "none",
	}
	mw := RequireSellerMiddleware(roleChecker)
	mw(c)

	assert.True(t, c.IsAborted(), "Market gate must reject never-paid seller")
	assert.Equal(t, http.StatusForbidden, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "activate", "never-paid seller must be told to ACTIVATE")
	assert.NotContains(t, body, "renew", "never-paid seller must NEVER be told to RENEW")
	assert.NotContains(t, body, "Renew", "never-paid seller must NEVER be told to RENEW")
}

func TestRequireSellerMiddleware_ExpiredSeller_IsToldToRenew(t *testing.T) {
	c, w := setupTestContext()
	userID := uuid.New()
	c.Set("user_id", userID)

	// Previously active then expired seller.
	roleChecker := &statusReportingRoleChecker{
		mockRoleChecker: &mockRoleChecker{
			hasSellerProfileVal: true,
			hasSellerCapability: false,
		},
		status: "expired",
	}
	mw := RequireSellerMiddleware(roleChecker)
	mw(c)

	assert.True(t, c.IsAborted(), "Market gate must reject expired seller")
	assert.Equal(t, http.StatusForbidden, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "renew", "expired seller must still be told to RENEW")
	assert.NotContains(t, body, "activate your subscription", "expired seller must not be told to activate")
}

func TestRequireSellerMiddleware_StatusReaderError_FailsClosedToActivate(t *testing.T) {
	c, w := setupTestContext()
	userID := uuid.New()
	c.Set("user_id", userID)

	// Status unreadable → fail closed to activation copy, never renewal.
	roleChecker := &statusReportingRoleChecker{
		mockRoleChecker: &mockRoleChecker{
			hasSellerProfileVal: true,
			hasSellerCapability: false,
		},
		status:    "",
		statusErr: errors.New("db down"),
	}
	mw := RequireSellerMiddleware(roleChecker)
	mw(c)

	assert.True(t, c.IsAborted())
	body := w.Body.String()
	assert.Contains(t, body, "activate")
	assert.False(t, strings.Contains(body, "renew"), "unreadable status must never say renew")
}

func TestMarketAuthorityDenialMessage(t *testing.T) {
	assert.Contains(t, marketAuthorityDenialMessage("expired"), "renew")
	assert.NotContains(t, marketAuthorityDenialMessage("expired"), "activate your subscription")

	assert.Contains(t, marketAuthorityDenialMessage("none"), "activate")
	assert.NotContains(t, marketAuthorityDenialMessage("none"), "renew")

	assert.Contains(t, marketAuthorityDenialMessage("active"), "activate")
	assert.NotContains(t, marketAuthorityDenialMessage("active"), "renew")

	assert.Contains(t, marketAuthorityDenialMessage(""), "activate")
	assert.NotContains(t, marketAuthorityDenialMessage(""), "renew")
}
