// SLICE 2: ACTOR CONTEXT INJECTION MIDDLEWARE TESTS
//
// This test file verifies the behavior of the ActorContextInject middleware.
// The middleware is responsible for injecting the Actor entity into the request
// context after authentication has been performed.
package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/stretchr/testify/assert"
)

// mockActorResolver implements capabilityEntity.ActorResolver for testing
type mockActorResolver struct {
	actor *capabilityEntity.Actor
	err   error
}

func (m *mockActorResolver) ResolveActor(ctx interface{}, userID uuid.UUID) (*capabilityEntity.Actor, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.actor == nil {
		return nil, &capabilityEntity.ActorNotFound{UserID: userID}
	}
	return m.actor, nil
}

// setupTestContext creates a test gin context
func setupActorTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)
	return c, w
}

// ============================================================
// ACTOR CONTEXT INJECTION MIDDLEWARE TESTS
// ============================================================

func TestActorContextInject_Success_AuthenticatedUser(t *testing.T) {
	// Arrange
	c, w := setupActorTestContext()
	userID := uuid.New()
	c.Set("user_id", userID)

	expectedActor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "seller",
		Capabilities: []string{"seller.verification.read", "seller.verification.write"},
	}

	resolver := &mockActorResolver{actor: expectedActor}
	middleware := ActorContextInject(resolver, ActorContextInjectOptions{})

	// Act
	middleware(c)

	// Assert
	assert.False(t, c.IsAborted(), "Middleware should not abort for authenticated user")
	assert.Equal(t, http.StatusOK, w.Code, "Should return 200 OK")

	// Verify actor was injected into context
	actor := GetActorFromContext(c)
	assert.NotNil(t, actor, "Actor should be in context")
	assert.Equal(t, userID, actor.ID, "Actor ID should match user ID")
	assert.Equal(t, "seller", actor.Role, "Actor role should match")
	assert.Equal(t, []string{"seller.verification.read", "seller.verification.write"}, actor.Capabilities, "Actor capabilities should match")
}

func TestActorContextInject_UnauthenticatedRequest_NoUserID(t *testing.T) {
	// Arrange
	c, w := setupActorTestContext()
	// Don't set user_id in context - simulating unauthenticated request

	resolver := &mockActorResolver{actor: &capabilityEntity.Actor{}}
	middleware := ActorContextInject(resolver, ActorContextInjectOptions{})

	// Act
	middleware(c)

	// Assert
	assert.False(t, c.IsAborted(), "Middleware should not abort for unauthenticated request")
	assert.Equal(t, http.StatusOK, w.Code, "Should return 200 OK")

	// Verify no actor was injected
	actor := GetActorFromContext(c)
	assert.Nil(t, actor, "Actor should not be in context for unauthenticated request")
}

func TestActorContextInject_NilUserID(t *testing.T) {
	// Arrange
	c, w := setupActorTestContext()
	c.Set("user_id", uuid.Nil) // Explicitly set to nil

	resolver := &mockActorResolver{actor: &capabilityEntity.Actor{}}
	middleware := ActorContextInject(resolver, ActorContextInjectOptions{})

	// Act
	middleware(c)

	// Assert
	assert.False(t, c.IsAborted(), "Middleware should not abort for nil user ID")
	assert.Equal(t, http.StatusOK, w.Code, "Should return 200 OK")

	// Verify no actor was injected
	actor := GetActorFromContext(c)
	assert.Nil(t, actor, "Actor should not be in context for nil user ID")
}

func TestActorContextInject_ActorResolutionFailure(t *testing.T) {
	// Arrange
	c, w := setupActorTestContext()
	userID := uuid.New()
	c.Set("user_id", userID)

	resolver := &mockActorResolver{
		err: errors.New("database connection failed"),
	}
	middleware := ActorContextInject(resolver, ActorContextInjectOptions{})

	// Act
	middleware(c)

	// Assert - IMPORTANT: middleware should NOT abort on actor resolution failure
	assert.False(t, c.IsAborted(), "Middleware should not abort on actor resolution failure")
	assert.Equal(t, http.StatusOK, w.Code, "Should return 200 OK")

	// Verify no actor was injected
	actor := GetActorFromContext(c)
	assert.Nil(t, actor, "Actor should not be in context when resolution fails")
}

func TestActorContextInject_ActorNotFound(t *testing.T) {
	// Arrange
	c, w := setupActorTestContext()
	userID := uuid.New()
	c.Set("user_id", userID)

	resolver := &mockActorResolver{
		actor: nil, // Simulates actor not found
	}
	middleware := ActorContextInject(resolver, ActorContextInjectOptions{})

	// Act
	middleware(c)

	// Assert - IMPORTANT: middleware should NOT abort when actor is not found
	assert.False(t, c.IsAborted(), "Middleware should not abort when actor not found")
	assert.Equal(t, http.StatusOK, w.Code, "Should return 200 OK")

	// Verify no actor was injected
	actor := GetActorFromContext(c)
	assert.Nil(t, actor, "Actor should not be in context when not found")
}

func TestActorContextInject_AltUserIDKey(t *testing.T) {
	// Arrange
	c, _ := setupActorTestContext()
	userID := uuid.New()
	c.Set("userID", userID) // Alternative key

	expectedActor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "admin",
		Capabilities: []string{"governance.users.suspend"},
	}

	resolver := &mockActorResolver{actor: expectedActor}
	middleware := ActorContextInject(resolver, ActorContextInjectOptions{})

	// Act
	middleware(c)

	// Assert
	assert.False(t, c.IsAborted(), "Middleware should not abort")

	// Verify actor was injected using the alternative key
	actor := GetActorFromContext(c)
	assert.NotNil(t, actor, "Actor should be in context")
	assert.Equal(t, userID, actor.ID, "Actor ID should match")
}

func TestActorContextInject_StringUserID(t *testing.T) {
	// Arrange
	c, _ := setupActorTestContext()
	userID := uuid.New()
	c.Set("user_id", userID.String()) // String instead of UUID

	expectedActor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "user",
		Capabilities: []string{},
	}

	resolver := &mockActorResolver{actor: expectedActor}
	middleware := ActorContextInject(resolver, ActorContextInjectOptions{})

	// Act
	middleware(c)

	// Assert
	assert.False(t, c.IsAborted(), "Middleware should not abort")

	// Verify actor was injected - the helper should parse the string
	actor := GetActorFromContext(c)
	assert.NotNil(t, actor, "Actor should be in context")
	assert.Equal(t, userID, actor.ID, "Actor ID should match")
}

func TestActorContextInject_ContextWithActor(t *testing.T) {
	// Arrange
	c, _ := setupActorTestContext()
	userID := uuid.New()
	c.Set("user_id", userID)

	expectedActor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "admin",
		Capabilities: []string{"moderation.content.remove"},
	}

	resolver := &mockActorResolver{actor: expectedActor}
	middleware := ActorContextInject(resolver, ActorContextInjectOptions{})

	// Act
	middleware(c)

	// Assert - verify actor is accessible via capability package helpers
	actor := capability.GetActor(c.Request.Context())
	assert.NotNil(t, actor, "Actor should be accessible via capability.GetActor")
	assert.Equal(t, userID, actor.ID, "Actor ID should match")
}

// ============================================================
// GIN CONTEXT ACTOR HELPER TESTS
// ============================================================

func TestGetActorFromContext_ActorPresent(t *testing.T) {
	// Arrange
	c, _ := setupActorTestContext()
	userID := uuid.New()

	actor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "admin",
		Capabilities: []string{"test.capability"},
	}

	ctx := capability.WithActor(c.Request.Context(), actor)
	c.Request = c.Request.WithContext(ctx)

	// Act
	result := GetActorFromContext(c)

	// Assert
	assert.Same(t, actor, result, "Should return the same actor instance")
}

func TestGetActorFromContext_ActorAbsent(t *testing.T) {
	// Arrange
	c, _ := setupActorTestContext()
	// No actor in context

	// Act
	result := GetActorFromContext(c)

	// Assert
	assert.Nil(t, result, "Should return nil when actor not in context")
}

func TestHasCapability_ActorHasCapability(t *testing.T) {
	// Arrange
	c, _ := setupActorTestContext()
	userID := uuid.New()

	actor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "admin",
		Capabilities: []string{"finance.withdraw.read", "finance.withdraw.write"},
	}

	ctx := capability.WithActor(c.Request.Context(), actor)
	c.Request = c.Request.WithContext(ctx)

	// Act
	result := HasCapability(c, "finance.withdraw.read")

	// Assert
	assert.True(t, result, "Should return true when actor has capability")
}

func TestHasCapability_ActorLacksCapability(t *testing.T) {
	// Arrange
	c, _ := setupActorTestContext()
	userID := uuid.New()

	actor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "admin",
		Capabilities: []string{"finance.withdraw.read"},
	}

	ctx := capability.WithActor(c.Request.Context(), actor)
	c.Request = c.Request.WithContext(ctx)

	// Act
	result := HasCapability(c, "finance.withdraw.write")

	// Assert
	assert.False(t, result, "Should return false when actor lacks capability")
}

func TestHasCapability_NoActor(t *testing.T) {
	// Arrange
	c, _ := setupActorTestContext()
	// No actor in context

	// Act
	result := HasCapability(c, "any.capability")

	// Assert
	assert.False(t, result, "Should return false when no actor in context")
}

func TestHasAnyCapability_MultipleCapabilities(t *testing.T) {
	// Arrange
	c, _ := setupActorTestContext()
	userID := uuid.New()

	actor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "admin",
		Capabilities: []string{"moderation.content.remove"},
	}

	ctx := capability.WithActor(c.Request.Context(), actor)
	c.Request = c.Request.WithContext(ctx)

	// Act - actor has one of the required capabilities
	result := HasAnyCapability(c, "moderation.content.remove", "governance.users.suspend")

	// Assert
	assert.True(t, result, "Should return true when actor has at least one capability")
}

func TestActorContextKey_IsCanonical(t *testing.T) {
	// Verify ActorContextKey is the same as capability.ActorContextKey
	assert.Equal(t, capability.ActorContextKey, ActorContextKey,
		"ActorContextKey should match capability.ActorContextKey for consistency")
}

// ============================================================
// PIPELINE BEHAVIOR TESTS
// ============================================================

func TestActorContextInject_Pipeline_ContinuesAfterInjection(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	userID := uuid.New()
	expectedActor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "seller",
		Capabilities: []string{"test.capability"},
	}

	resolver := &mockActorResolver{actor: expectedActor}
	middleware := ActorContextInject(resolver, ActorContextInjectOptions{})

	// Track if actor was available in handler
	var actorInHandler *capabilityEntity.Actor

	// Set up a route with the middleware and a handler
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		actorInHandler = GetActorFromContext(c)
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Set up request with user_id in context
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-User-ID", userID.String())

	// We need to use a different approach - set user_id in gin context manually
	// by using a custom middleware before the actor inject middleware
	var customMiddleware gin.HandlerFunc = func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	}

	// Rebuild router with custom middleware first
	router = gin.New()
	router.Use(customMiddleware)
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		actorInHandler = GetActorFromContext(c)
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req, _ = http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code, "Request should succeed")
	assert.NotNil(t, actorInHandler, "Actor should be available in handler")
	assert.Equal(t, userID, actorInHandler.ID, "Actor ID should match")
}


