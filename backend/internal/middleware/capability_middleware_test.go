// SLICE 2: CAPABILITY MIDDLEWARE TESTS
//
// This test file verifies the behavior of the capability-based authorization
// middleware. These middleware enforce access control based on the Actor's
// fine-grained capabilities.
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/stretchr/testify/assert"
)

// setupCapabilityTestContext creates a test gin context with optional actor
func setupCapabilityTestContext(actor *capabilityEntity.Actor) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)

	if actor != nil {
		ctx := capability.WithActor(c.Request.Context(), actor)
		c.Request = c.Request.WithContext(ctx)
	}

	return c, w
}

// ============================================================
// REQUIRECAPABILITY MIDDLEWARE TESTS
// ============================================================

func TestRequireCapability_Success_HasCapability(t *testing.T) {
	// Arrange
	userID := uuid.New()
	actor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "admin",
		Capabilities: []string{"finance.withdraw.read", "finance.withdraw.write"},
	}
	c, w := setupCapabilityTestContext(actor)

	middleware := RequireCapability("finance.withdraw.read")

	// Act
	middleware(c)

	// Assert
	assert.False(t, c.IsAborted(), "Middleware should not abort when actor has capability")
	assert.Equal(t, http.StatusOK, w.Code, "Should return 200 OK")
}

func TestRequireCapability_Forbidden_LacksCapability(t *testing.T) {
	// Arrange
	userID := uuid.New()
	actor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "admin",
		Capabilities: []string{"finance.withdraw.read"}, // Only has read, not write
	}
	c, w := setupCapabilityTestContext(actor)

	middleware := RequireCapability("finance.withdraw.write")

	// Act
	middleware(c)

	// Assert
	assert.True(t, c.IsAborted(), "Middleware should abort when actor lacks capability")
	assert.Equal(t, http.StatusForbidden, w.Code, "Should return 403 Forbidden")

	// Verify response body contains permission denied message
	assert.Contains(t, w.Body.String(), "Insufficient permissions",
		"Response should indicate insufficient permissions")
}

func TestRequireCapability_Unauthorized_NoActor(t *testing.T) {
	// Arrange
	c, w := setupCapabilityTestContext(nil) // No actor in context

	middleware := RequireCapability("any.capability")

	// Act
	middleware(c)

	// Assert
	assert.True(t, c.IsAborted(), "Middleware should abort when no actor in context")
	assert.Equal(t, http.StatusUnauthorized, w.Code, "Should return 401 Unauthorized")

	// Verify response body contains authentication required message
	assert.Contains(t, w.Body.String(), "Authentication required",
		"Response should indicate authentication is required")
}

func TestRequireCapability_EmptyCapabilitiesList(t *testing.T) {
	// Arrange
	userID := uuid.New()
	actor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "user",
		Capabilities: []string{}, // Empty capabilities
	}
	c, w := setupCapabilityTestContext(actor)

	middleware := RequireCapability("any.capability")

	// Act
	middleware(c)

	// Assert
	assert.True(t, c.IsAborted(), "Middleware should abort for empty capability list")
	assert.Equal(t, http.StatusForbidden, w.Code, "Should return 403 Forbidden")
}

func TestRequireCapability_NoAdminFallback(t *testing.T) {
	// IMPORTANT: Verify that admin role does NOT grant implicit capability access
	// Arrange
	userID := uuid.New()
	actor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "admin", // Admin role
		Capabilities: []string{}, // But NO explicit capabilities
	}
	c, w := setupCapabilityTestContext(actor)

	middleware := RequireCapability("finance.withdraw.read")

	// Act
	middleware(c)

	// Assert - admin role should NOT grant implicit access
	assert.True(t, c.IsAborted(), "Middleware should abort even for admin without explicit capability")
	assert.Equal(t, http.StatusForbidden, w.Code, "Should return 403 Forbidden")

	// This is a CRITICAL security test - ensures no implicit admin fallback
}

// ============================================================
// REQUIREANYCAPABILITY MIDDLEWARE TESTS
// ============================================================

func TestRequireAnyCapability_Success_HasFirstCapability(t *testing.T) {
	// Arrange
	userID := uuid.New()
	actor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "admin",
		Capabilities: []string{"moderation.content.remove"},
	}
	c, _ := setupCapabilityTestContext(actor)

	middleware := RequireAnyCapability("moderation.content.remove", "governance.users.suspend")

	// Act
	middleware(c)

	// Assert
	assert.False(t, c.IsAborted(), "Middleware should not abort when actor has first capability")
}

func TestRequireAnyCapability_Success_HasSecondCapability(t *testing.T) {
	// Arrange
	userID := uuid.New()
	actor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "admin",
		Capabilities: []string{"governance.users.suspend"},
	}
	c, _ := setupCapabilityTestContext(actor)

	middleware := RequireAnyCapability("moderation.content.remove", "governance.users.suspend")

	// Act
	middleware(c)

	// Assert
	assert.False(t, c.IsAborted(), "Middleware should not abort when actor has second capability")
}

func TestRequireAnyCapability_Success_HasMultipleCapabilities(t *testing.T) {
	// Arrange
	userID := uuid.New()
	actor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "admin",
		Capabilities: []string{"moderation.content.remove", "governance.users.suspend", "finance.withdraw.read"},
	}
	c, _ := setupCapabilityTestContext(actor)

	middleware := RequireAnyCapability("moderation.content.remove", "governance.users.suspend")

	// Act
	middleware(c)

	// Assert
	assert.False(t, c.IsAborted(), "Middleware should not abort when actor has multiple capabilities")
}

func TestRequireAnyCapability_Forbidden_HasNone(t *testing.T) {
	// Arrange
	userID := uuid.New()
	actor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "user",
		Capabilities: []string{"content.create"},
	}
	c, w := setupCapabilityTestContext(actor)

	middleware := RequireAnyCapability("moderation.content.remove", "governance.users.suspend")

	// Act
	middleware(c)

	// Assert
	assert.True(t, c.IsAborted(), "Middleware should abort when actor has none of the required capabilities")
	assert.Equal(t, http.StatusForbidden, w.Code, "Should return 403 Forbidden")
}

func TestRequireAnyCapability_Unauthorized_NoActor(t *testing.T) {
	// Arrange
	c, w := setupCapabilityTestContext(nil)

	middleware := RequireAnyCapability("any.capability")

	// Act
	middleware(c)

	// Assert
	assert.True(t, c.IsAborted(), "Middleware should abort when no actor in context")
	assert.Equal(t, http.StatusUnauthorized, w.Code, "Should return 401 Unauthorized")
}

func TestRequireAnyCapability_Forbidden_EmptyCapabilityList(t *testing.T) {
	// Arrange - IMPORTANT: Empty capability list should always deny
	userID := uuid.New()
	actor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "admin",
		Capabilities: []string{"any.capability"},
	}
	c, w := setupCapabilityTestContext(actor)

	middleware := RequireAnyCapability() // Empty list

	// Act
	middleware(c)

	// Assert - Defensive default: empty list should deny
	assert.True(t, c.IsAborted(), "Middleware should abort for empty capability list")
	assert.Equal(t, http.StatusForbidden, w.Code, "Should return 403 Forbidden")
}

func TestRequireAnyCapability_NoAdminFallback(t *testing.T) {
	// IMPORTANT: Verify that admin role does NOT grant implicit capability access
	// Arrange
	userID := uuid.New()
	actor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "admin",
		Capabilities: []string{}, // No explicit capabilities
	}
	c, w := setupCapabilityTestContext(actor)

	middleware := RequireAnyCapability("moderation.content.remove", "governance.users.suspend")

	// Act
	middleware(c)

	// Assert - admin role should NOT grant implicit access
	assert.True(t, c.IsAborted(), "Middleware should abort even for admin without explicit capabilities")
	assert.Equal(t, http.StatusForbidden, w.Code, "Should return 403 Forbidden")
}

// ============================================================
// REQUIREALLCAPABILITIES MIDDLEWARE TESTS
// ============================================================

func TestRequireAllCapabilities_Success_HasAll(t *testing.T) {
	// Arrange
	userID := uuid.New()
	actor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "admin",
		Capabilities: []string{"finance.withdraw.read", "finance.withdraw.approve"},
	}
	c, _ := setupCapabilityTestContext(actor)

	middleware := RequireAllCapabilities("finance.withdraw.read", "finance.withdraw.approve")

	// Act
	middleware(c)

	// Assert
	assert.False(t, c.IsAborted(), "Middleware should not abort when actor has all capabilities")
}

func TestRequireAllCapabilities_Forbidden_MissingOne(t *testing.T) {
	// Arrange
	userID := uuid.New()
	actor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "admin",
		Capabilities: []string{"finance.withdraw.read"}, // Missing approve
	}
	c, w := setupCapabilityTestContext(actor)

	middleware := RequireAllCapabilities("finance.withdraw.read", "finance.withdraw.approve")

	// Act
	middleware(c)

	// Assert
	assert.True(t, c.IsAborted(), "Middleware should abort when actor is missing a capability")
	assert.Equal(t, http.StatusForbidden, w.Code, "Should return 403 Forbidden")
}

func TestRequireAllCapabilities_Success_EmptyList(t *testing.T) {
	// Arrange - Empty list should always allow (logical AND over empty set)
	userID := uuid.New()
	actor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "user",
		Capabilities: []string{},
	}
	c, _ := setupCapabilityTestContext(actor)

	middleware := RequireAllCapabilities() // Empty list

	// Act
	middleware(c)

	// Assert - Empty set is trivially satisfied (logical AND identity)
	assert.False(t, c.IsAborted(), "Middleware should not abort for empty capability list")
}

func TestRequireAllCapabilities_Unauthorized_NoActor(t *testing.T) {
	// Arrange
	c, w := setupCapabilityTestContext(nil)

	middleware := RequireAllCapabilities("any.capability")

	// Act
	middleware(c)

	// Assert
	assert.True(t, c.IsAborted(), "Middleware should abort when no actor in context")
	assert.Equal(t, http.StatusUnauthorized, w.Code, "Should return 401 Unauthorized")
}

// ============================================================
// REQUIREACTOR MIDDLEWARE TESTS
// ============================================================

func TestRequireActor_Success_HasActor(t *testing.T) {
	// Arrange
	userID := uuid.New()
	actor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "user",
		Capabilities: []string{},
	}
	c, _ := setupCapabilityTestContext(actor)

	middleware := RequireActor()

	// Act
	middleware(c)

	// Assert
	assert.False(t, c.IsAborted(), "Middleware should not abort when actor is present")
}

func TestRequireActor_Unauthorized_NoActor(t *testing.T) {
	// Arrange
	c, w := setupCapabilityTestContext(nil)

	middleware := RequireActor()

	// Act
	middleware(c)

	// Assert
	assert.True(t, c.IsAborted(), "Middleware should abort when no actor in context")
	assert.Equal(t, http.StatusUnauthorized, w.Code, "Should return 401 Unauthorized")
}

// ============================================================
// WITHCAPABILITYCHECK MIDDLEWARE TESTS
// ============================================================

func TestWithCapabilityCheck_Success_CustomCheckPasses(t *testing.T) {
	// Arrange
	userID := uuid.New()
	actor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "seller",
		Capabilities: []string{"content.update"},
	}
	c, _ := setupCapabilityTestContext(actor)

	// Custom check: allow if user has content.update OR is accessing own resource
	check := func(a *capabilityEntity.Actor, c *gin.Context) bool {
		return a.HasCapability("content.update") || c.Param("id") == a.ID.String()
	}

	middleware := WithCapabilityCheck(check)

	// Act
	middleware(c)

	// Assert
	assert.False(t, c.IsAborted(), "Middleware should not abort when custom check passes")
}

func TestWithCapabilityCheck_Forbidden_CustomCheckFails(t *testing.T) {
	// Arrange
	userID := uuid.New()
	actor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "user",
		Capabilities: []string{}, // No capabilities
	}
	c, w := setupCapabilityTestContext(actor)

	// Custom check: require specific capability
	check := func(a *capabilityEntity.Actor, c *gin.Context) bool {
		return a.HasCapability("admin.override")
	}

	middleware := WithCapabilityCheck(check)

	// Act
	middleware(c)

	// Assert
	assert.True(t, c.IsAborted(), "Middleware should abort when custom check fails")
	assert.Equal(t, http.StatusForbidden, w.Code, "Should return 403 Forbidden")
}

func TestWithCapabilityCheck_Unauthorized_NoActor(t *testing.T) {
	// Arrange
	c, w := setupCapabilityTestContext(nil)

	check := func(a *capabilityEntity.Actor, c *gin.Context) bool {
		return true // Custom check wouldn't be called
	}

	middleware := WithCapabilityCheck(check)

	// Act
	middleware(c)

	// Assert
	assert.True(t, c.IsAborted(), "Middleware should abort when no actor in context")
	assert.Equal(t, http.StatusUnauthorized, w.Code, "Should return 401 Unauthorized")
}

// ============================================================
// RESPONSE HELPER TESTS
// ============================================================

func TestCapabilityRequiredResponse_SendsForbidden(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)

	// Act
	CapabilityRequiredResponse(c)

	// Assert
	assert.Equal(t, http.StatusForbidden, w.Code, "Should return 403 Forbidden")
	assert.Contains(t, w.Body.String(), "Insufficient permissions",
		"Response should indicate insufficient permissions")
}

func TestUnauthorizedResponse_SendsUnauthorized(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)

	// Act
	UnauthorizedResponse(c)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, w.Code, "Should return 401 Unauthorized")
	assert.Contains(t, w.Body.String(), "Authentication required",
		"Response should indicate authentication is required")
}


