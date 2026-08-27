// SLICE 2: PIPELINE INTEGRATION TESTS
//
// This test file verifies the complete request pipeline behavior when using
// the ActorContextInject and Capability middleware together.
//
// These tests ensure:
// 1. Actor is injected into context
// 2. Capability middleware reads actor from context
// 3. Requests are allowed/denied based on capabilities
// 4. The pipeline works correctly end-to-end
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================
// END-TO-END PIPELINE TESTS
// ============================================================

// TestPipeline_Success_ActorHasCapability verifies the complete pipeline:
// 1. User ID is in gin context (simulating UserLookupMiddleware)
// 2. ActorContextInject resolves and injects actor
// 3. RequireCapability checks actor's capability
// 4. Handler executes successfully
func TestPipeline_Success_ActorHasCapability(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	userID := uuid.New()

	// Create an actor with the required capability
	expectedActor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "admin",
		Capabilities: []string{"governance.reports.read"},
	}

	actorResolver := &mockActorResolver{actor: expectedActor}

	// Track if actor was available in handler
	var actorInHandler *capabilityEntity.Actor

	// Custom middleware to set user_id (simulating UserLookupMiddleware)
	setUserIDMiddleware := func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	}

	// Set up the middleware pipeline
	router.Use(setUserIDMiddleware)
	router.Use(ActorContextInject(actorResolver, ActorContextInjectOptions{}))
	router.Use(RequireCapability("governance.reports.read"))

	// Mock handler that verifies actor is available
	router.GET("/test", func(c *gin.Context) {
		actorInHandler = GetActorFromContext(c)
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Act
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code, "Should return 200 OK")
	assert.NotNil(t, actorInHandler, "Actor should be available in handler")
	assert.Equal(t, userID, actorInHandler.ID, "Actor ID should match")
	assert.True(t, actorInHandler.HasCapability("governance.reports.read"),
		"Actor should have the required capability")
}

// TestPipeline_Forbidden_ActorLacksCapability verifies that the pipeline
// correctly denies access when actor lacks the required capability.
func TestPipeline_Forbidden_ActorLacksCapability(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	userID := uuid.New()

	// Create an actor WITHOUT the required capability
	actorWithoutCapability := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "user",
		Capabilities: []string{"moderation.content.remove"}, // Different capability
	}

	actorResolver := &mockActorResolver{actor: actorWithoutCapability}

	setUserIDMiddleware := func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	}

	handlerCalled := false
	router.Use(setUserIDMiddleware)
	router.Use(ActorContextInject(actorResolver, ActorContextInjectOptions{}))
	router.Use(RequireCapability("governance.reports.read"))
	router.GET("/test", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(http.StatusOK, gin.H{"message": "should not reach here"})
	})

	// Act
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	// Assert
	assert.False(t, handlerCalled, "Handler should NOT be called")
	assert.Equal(t, http.StatusForbidden, w.Code, "Should return 403 Forbidden")
}

// TestPipeline_Unauthenticated_NoUserID verifies that the pipeline
// correctly handles unauthenticated requests.
func TestPipeline_Unauthenticated_NoUserID(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	actorResolver := &mockActorResolver{actor: &capabilityEntity.Actor{}}

	handlerCalled := false
	router.Use(ActorContextInject(actorResolver, ActorContextInjectOptions{}))
	router.Use(RequireCapability("governance.reports.read"))
	router.GET("/test", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(http.StatusOK, gin.H{"message": "should not reach here"})
	})

	// Act
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	// Assert
	assert.False(t, handlerCalled, "Handler should NOT be called")
	assert.Equal(t, http.StatusUnauthorized, w.Code, "Should return 401 Unauthorized")
}

// TestPipeline_MultipleCapabilityChecks verifies that the pipeline
// works correctly with multiple capability middleware.
func TestPipeline_MultipleCapabilityChecks(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	userID := uuid.New()

	// Create an actor with multiple capabilities
	actor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "admin",
		Capabilities: []string{
			"finance.withdraw.read",
			"finance.withdraw.approve",
			"governance.audit.view",
		},
	}

	actorResolver := &mockActorResolver{actor: actor}

	setUserIDMiddleware := func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	}

	// Set up the middleware pipeline with multiple checks
	router.Use(setUserIDMiddleware)
	router.Use(ActorContextInject(actorResolver, ActorContextInjectOptions{}))
	router.Use(RequireCapability("finance.withdraw.read"))
	router.Use(RequireCapability("finance.withdraw.approve"))

	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "withdraw approved"})
	})

	// Act
	req, _ := http.NewRequest("POST", "/test", nil)
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code, "Should return 200 OK")
}

// TestPipeline_RequireAnyCapability_Success verifies that RequireAnyCapability
// works correctly in the pipeline when actor has one of the required capabilities.
func TestPipeline_RequireAnyCapability_Success(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	userID := uuid.New()

	// Create an actor with only one of the required capabilities
	actor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "admin",
		Capabilities: []string{"moderation.content.remove"}, // Has this one
	}

	actorResolver := &mockActorResolver{actor: actor}

	setUserIDMiddleware := func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	}

	router.Use(setUserIDMiddleware)
	router.Use(ActorContextInject(actorResolver, ActorContextInjectOptions{}))
	router.Use(RequireAnyCapability(
		"moderation.content.remove",
		"governance.content.remove",
	))

	router.DELETE("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "content removed"})
	})

	// Act
	req, _ := http.NewRequest("DELETE", "/test", nil)
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code, "Should return 200 OK")
}

// TestPipeline_ActorResolutionFailure_Continues verifies that when
// actor resolution fails, the pipeline continues (does not panic).
func TestPipeline_ActorResolutionFailure_Continues(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	userID := uuid.New()

	// Actor resolver that returns an error
	actorResolver := &mockActorResolver{
		err: &capabilityEntity.ActorNotFound{UserID: userID},
	}

	setUserIDMiddleware := func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	}

	handlerCalled := false
	var actorInHandler *capabilityEntity.Actor

	router.Use(setUserIDMiddleware)
	router.Use(ActorContextInject(actorResolver, ActorContextInjectOptions{}))
	router.Use(RequireCapability("any.capability"))
	router.GET("/test", func(c *gin.Context) {
		handlerCalled = true
		actorInHandler = GetActorFromContext(c)
		c.JSON(http.StatusOK, gin.H{})
	})

	// Act
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	// Assert - The capability middleware should abort (no actor)
	assert.False(t, handlerCalled, "Handler should NOT be called (capability middleware aborts)")
	assert.Nil(t, actorInHandler, "Actor should be nil")
	assert.Equal(t, http.StatusUnauthorized, w.Code, "Should return 401 Unauthorized (no actor)")
}

// TestPipeline_ContextPropagation verifies that the actor context
// is properly propagated through the middleware chain.
func TestPipeline_ContextPropagation(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	userID := uuid.New()

	expectedActor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "seller",
		Capabilities: []string{"seller.verification.write"},
	}

	actorResolver := &mockActorResolver{actor: expectedActor}

	setUserIDMiddleware := func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	}

	var actorInFirstMiddleware *capabilityEntity.Actor
	var actorInSecondMiddleware *capabilityEntity.Actor
	var actorInHandler *capabilityEntity.Actor

	firstMiddleware := func(c *gin.Context) {
		actorInFirstMiddleware = GetActorFromContext(c)
		require.NotNil(t, actorInFirstMiddleware, "Actor should be available in first middleware")
		c.Next()
	}

	secondMiddleware := func(c *gin.Context) {
		actorInSecondMiddleware = GetActorFromContext(c)
		require.NotNil(t, actorInSecondMiddleware, "Actor should be available in second middleware")
		c.Next()
	}

	router.Use(setUserIDMiddleware)
	router.Use(ActorContextInject(actorResolver, ActorContextInjectOptions{}))
	router.Use(firstMiddleware)
	router.Use(secondMiddleware)
	router.GET("/test", func(c *gin.Context) {
		actorInHandler = GetActorFromContext(c)
		require.NotNil(t, actorInHandler, "Actor should be available in handler")
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Act
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	// Assert - Verify same actor is available throughout the pipeline
	assert.NotNil(t, actorInFirstMiddleware, "Actor should be in first middleware")
	assert.NotNil(t, actorInSecondMiddleware, "Actor should be in second middleware")
	assert.NotNil(t, actorInHandler, "Actor should be in handler")
	assert.Equal(t, userID, actorInHandler.ID, "Actor ID should match throughout pipeline")
}

// ============================================================
// SECURITY TESTS - NO IMPLICIT FALLBACK
// ============================================================

// TestPipeline_NoAdminFallback verifies that the admin role does NOT
// grant implicit capability access. This is a critical security test.
func TestPipeline_NoAdminFallback(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	userID := uuid.New()

	// Create an admin actor WITHOUT the required capability
	// This tests that there's no implicit admin→capability mapping
	adminActor := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "admin", // Admin role
		Capabilities: []string{}, // But NO explicit finance capabilities
	}

	actorResolver := &mockActorResolver{actor: adminActor}

	setUserIDMiddleware := func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	}

	handlerCalled := false
	router.Use(setUserIDMiddleware)
	router.Use(ActorContextInject(actorResolver, ActorContextInjectOptions{}))
	router.Use(RequireCapability("finance.reports.read"))
	router.GET("/test", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(http.StatusOK, gin.H{})
	})

	// Act
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	// Assert - CRITICAL: Admin role should NOT grant implicit access
	assert.False(t, handlerCalled, "Handler should NOT be called")
	assert.Equal(t, http.StatusForbidden, w.Code, "Should return 403 Forbidden")
}

// TestPipeline_CapabilityGrantedExplicitly verifies that capability
// must be explicitly granted, even for admins.
func TestPipeline_CapabilityGrantedExplicitly(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New()

	userID := uuid.New()

	// Create an admin actor WITH the required capability
	// This tests that explicit capability grants work correctly
	adminActorWithCapability := &capabilityEntity.Actor{
		ID:           userID,
		Role:         "admin",
		Capabilities: []string{"governance.users.suspend"}, // Explicitly granted
	}

	actorResolver := &mockActorResolver{actor: adminActorWithCapability}

	setUserIDMiddleware := func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	}

	handlerCalled := false
	router.Use(setUserIDMiddleware)
	router.Use(ActorContextInject(actorResolver, ActorContextInjectOptions{}))
	router.Use(RequireCapability("governance.users.suspend"))
	router.POST("/test", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(http.StatusOK, gin.H{"message": "user suspended"})
	})

	// Act
	req, _ := http.NewRequest("POST", "/test", nil)
	router.ServeHTTP(w, req)

	// Assert - Explicit capability grant should work
	assert.True(t, handlerCalled, "Handler should be called")
	assert.Equal(t, http.StatusOK, w.Code, "Should return 200 OK")
}


