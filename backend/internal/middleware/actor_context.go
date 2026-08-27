// Package middleware provides actor context injection for the capability system.
//
// SLICE 2: ACTOR CONTEXT INJECTION MIDDLEWARE
//
// This middleware is responsible for injecting the Actor entity into the request context
// after authentication has been performed. It works safely with both authenticated and
// unauthenticated requests.
//
// DESIGN PRINCIPLES:
// - Safe: Never panics, always continues the request chain
// - Non-blocking: Does not reject requests based on actor resolution
// - Minimal: Only reads from existing context, does not validate tokens
// - Clear: Uses explicit context helpers for actor injection
//
// USAGE:
// The middleware should be placed AFTER AuthMiddleware/UserLookupMiddleware but BEFORE
// any capability-protected handlers or middleware.
package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	"go.uber.org/zap"
)

// ActorContextInjectOptions configures the actor context injection middleware.
type ActorContextInjectOptions struct {
	// Log is the logger for debugging actor resolution failures.
	// If nil, failures will be silent (recommended for production).
	Log *zap.Logger

	// EnableDebugLogging enables verbose logging for actor injection.
	// This should be false in production to avoid log spam.
	EnableDebugLogging bool
}

// ActorContextInject creates middleware that injects Actor into the request context.
//
// This middleware:
// 1. Extracts user ID from the existing request context (set by UserLookupMiddleware)
// 2. Resolves the Actor using ActorResolver (role + capabilities)
// 3. Injects the Actor into the request context using capability.WithActor()
//
// IMPORTANT:
// - Does NOT perform authentication (that's done by AuthMiddleware)
// - Does NOT perform user lookup (that's done by UserLookupMiddleware)
// - Does NOT block requests if actor resolution fails
// - Unauthenticated requests simply have no actor in context
//
// EXPECTED BEHAVIOR:
// - Authenticated request with valid user → actor injected, request continues
// - Unauthenticated request → no actor, request continues
// - Actor resolution failure → no actor, request continues (logged if enabled)
//
// This middleware MUST be placed after UserLookupMiddleware in the middleware chain.
func ActorContextInject(actorResolver capabilityEntity.ActorResolver, opts ActorContextInjectOptions) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Step 1: Try to get user ID from existing context
		// The user ID is set by UserLookupMiddleware
		userID, exists := getUserIDFromGinContext(c)
		if !exists || userID == uuid.Nil {
			// User is not authenticated - this is fine, just continue
			// No actor will be available in context
			if opts.Log != nil && opts.EnableDebugLogging {
				opts.Log.Debug("[ACTOR CONTEXT] No user ID in context, skipping actor injection")
			}
			c.Next()
			return
		}

		// Step 2: Resolve actor from user ID
		// This loads both role and capabilities
		actor, err := actorResolver.ResolveActor(ctx, userID)
		if err != nil {
			// Actor resolution failed - continue anyway
			// This is intentional: we don't want to block requests if the capability
			// system has issues. The capability middleware will handle authorization.
			if opts.Log != nil {
				// Log the failure but don't expose internal details
				opts.Log.Warn("[ACTOR CONTEXT] Failed to resolve actor, continuing without actor",
					zap.String("user_id", userID.String()),
					zap.Error(err),
				)
			}
			c.Next()
			return
		}

		// Step 3: Inject actor into request context
		// Use the canonical context helper from capability package
		ctxWithActor := capability.WithActor(ctx, actor)
		c.Request = c.Request.WithContext(ctxWithActor)

		if opts.Log != nil && opts.EnableDebugLogging {
			opts.Log.Debug("[ACTOR CONTEXT] Actor injected",
				zap.String("user_id", userID.String()),
				zap.String("role", actor.Role),
				zap.Int("capabilities", len(actor.Capabilities)),
			)
		}

		c.Next()
	}
}

// getUserIDFromGinContext safely extracts user ID from gin context.
// This mirrors the logic in GetUserIDFromContext but returns a boolean
// to indicate presence instead of an error.
func getUserIDFromGinContext(c *gin.Context) (uuid.UUID, bool) {
	// Try user_id key first (set by UserLookupMiddleware)
	if userIDVal, exists := c.Get("user_id"); exists {
		switch v := userIDVal.(type) {
		case uuid.UUID:
			if v != uuid.Nil {
				return v, true
			}
		case string:
			if id, err := uuid.Parse(v); err == nil {
				return id, true
			}
		}
	}

	// Try userID key (alternative)
	if userIDVal, exists := c.Get("userID"); exists {
		switch v := userIDVal.(type) {
		case uuid.UUID:
			if v != uuid.Nil {
				return v, true
			}
		case string:
			if id, err := uuid.Parse(v); err == nil {
				return id, true
			}
		}
	}

	return uuid.Nil, false
}

// ============================================================
// GIN-SPECIFIC ACTOR HELPERS
// ============================================================

// GetActorFromContext retrieves the Actor from the gin request context.
//
// This is a convenience wrapper around capability.GetActor() that works
// directly with gin.Context.
//
// Returns nil if actor is not present in context.
//
// Example:
//   actor := GetActorFromContext(c)
//   if actor != nil && actor.HasCapability("finance.withdraw.read") {
//       // Allow access
//   }
func GetActorFromContext(c *gin.Context) *capabilityEntity.Actor {
	return capability.GetActor(c.Request.Context())
}

// ActorFromContext is an alias for GetActorFromContext for consistency
// with other context helpers in this package.
func ActorFromContext(c *gin.Context) *capabilityEntity.Actor {
	return GetActorFromContext(c)
}

// HasCapability is a convenience function that checks if the current
// request's actor has a specific capability.
//
// Returns false if no actor is present in context.
//
// Example:
//   if HasCapability(c, "finance.withdraw.read") {
//       // Show withdrawal button
//   }
func HasCapability(c *gin.Context, cap string) bool {
	return capability.HasCapability(c.Request.Context(), cap)
}

// HasAnyCapability is a convenience function that checks if the current
// request's actor has any of the specified capabilities.
//
// Returns false if no actor is present in context.
func HasAnyCapability(c *gin.Context, caps ...string) bool {
	return capability.HasAnyCapability(c.Request.Context(), caps...)
}

// ============================================================
// CONTEXT HELPERS FOR GIN COMPATIBILITY
// ============================================================

// The following helpers provide gin-specific convenience functions
// that mirror the capability package helpers but work directly with
// gin.Context instead of context.Context.

// ActorContextKey is the canonical context key for Actor.
// Use GetActorFromContext() to retrieve.
var ActorContextKey = capability.ActorContextKey

// WithActor returns a new gin.Context with the Actor stored in its request context.
//
// This is a convenience wrapper around capability.WithActor() for gin middleware.
//
// Example:
//   ctx := WithActor(c.Request.Context(), actor)
//   c.Request = c.Request.WithContext(ctx)
func WithActor(ctx context.Context, actor *capabilityEntity.Actor) context.Context {
	return capability.WithActor(ctx, actor)
}


