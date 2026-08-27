// Package middleware provides capability-based authorization middleware.
//
// SLICE 2: CAPABILITY MIDDLEWARE FOUNDATION
//
// This package implements capability enforcement middleware that uses the Actor
// from the request context (injected by ActorContextInject middleware) to make
// authorization decisions based on fine-grained capabilities.
//
// DESIGN PRINCIPLES:
// - Explicit: Only grants access based on explicitly granted capabilities
// - No fallback: Does NOT fall back to admin role or any implicit logic
// - Context-first: Reads Actor from context, does not resolve actor
// - Safe: Returns appropriate error responses without panicking
//
// USAGE:
// These middleware should be placed AFTER ActorContextInject in the middleware chain.
// They will reject requests that don't have the required capabilities.
//
// NOTE: This is the foundation for Slice 3 where existing routes will be migrated
// to use capability-based authorization. In Slice 2, these middleware are prepared
// and tested but NOT yet applied to production routes.
package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/labuda/backend/internal/platform/response"
)

// RequireCapability creates middleware that requires a specific capability.
//
// The middleware:
// 1. Extracts Actor from the request context (injected by ActorContextInject)
// 2. Checks if the actor has the required capability
// 3. Returns 403 Forbidden if capability is missing
// 4. Returns 401 Unauthorized if no actor is present
// 5. Continues to next handler if capability is present
//
// IMPORTANT SECURITY NOTES:
// - Does NOT fall back to admin role checking
// - Does NOT perform any implicit role→capability mapping
// - Only checks capabilities explicitly granted to the actor
// - Authorization is purely based on the actor's capability list
//
// This middleware must be used AFTER ActorContextInject in the middleware chain.
//
// Example:
//   router.Use(middleware.ActorContextInject(actorResolver, opts))
//   router.GET("/admin/reports",
//       middleware.RequireCapability("governance.reports.read"),
//       reportsHandler,
//   )
func RequireCapability(requiredCapability string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Step 1: Get actor from context
		actor := capability.GetActor(ctx)
		if actor == nil {
			// No actor in context - user is not authenticated
			// or ActorContextInject middleware is not in the chain
			response.Unauthorized(c, "Authentication required")
			c.Abort()
			return
		}

		// Step 2: Check if actor has the required capability
		// This is an explicit check - no fallback logic
		if !actor.HasCapability(requiredCapability) {
			// Actor doesn't have the capability - forbid access
			response.Forbidden(c, "Insufficient permissions")
			c.Abort()
			return
		}

		// Actor has the capability - continue to handler
		c.Next()
	}
}

// RequireAnyCapability creates middleware that requires at least one of the
// specified capabilities.
//
// The middleware:
// 1. Extracts Actor from the request context (injected by ActorContextInject)
// 2. Checks if the actor has ANY of the required capabilities
// 3. Returns 403 Forbidden if none of the capabilities are present
// 4. Returns 401 Unauthorized if no actor is present
// 5. Continues to next handler if at least one capability is present
//
// This is useful for endpoints that can be accessed by multiple capability
// paths. For example, a content moderation endpoint might be accessible by
// both moderators and admins:
//
//   router.DELETE("/content/:id",
//       middleware.RequireAnyCapability(
//           "moderation.content.remove",
//           "governance.content.remove",
//       ),
//       removeContentHandler,
//   )
//
// IMPORTANT SECURITY NOTES:
// - Does NOT fall back to admin role checking
// - Only grants access if actor has AT LEAST ONE of the listed capabilities
// - Empty capability list will always deny access (defensive default)
func RequireAnyCapability(requiredCapabilities ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Defensive: empty capability list should always deny
		if len(requiredCapabilities) == 0 {
			response.Forbidden(c, "Insufficient permissions")
			c.Abort()
			return
		}

		// Step 1: Get actor from context
		actor := capability.GetActor(ctx)
		if actor == nil {
			// No actor in context - user is not authenticated
			response.Unauthorized(c, "Authentication required")
			c.Abort()
			return
		}

		// Step 2: Check if actor has any of the required capabilities
		if !actor.HasAnyCapability(requiredCapabilities...) {
			// Actor doesn't have any of the capabilities - forbid access
			response.Forbidden(c, "Insufficient permissions")
			c.Abort()
			return
		}

		// Actor has at least one of the capabilities - continue to handler
		c.Next()
	}
}

// RequireAllCapabilities creates middleware that requires ALL of the
// specified capabilities.
//
// The middleware:
// 1. Extracts Actor from the request context (injected by ActorContextInject)
// 2. Checks if the actor has ALL of the required capabilities
// 3. Returns 403 Forbidden if any capability is missing
// 4. Returns 401 Unauthorized if no actor is present
// 5. Continues to next handler if all capabilities are present
//
// This is useful for endpoints that require multiple capabilities. For example,
// a financial approval might require both read and write access:
//
//   router.POST("/finance/approve",
//       middleware.RequireAllCapabilities(
//           "finance.withdraw.read",
//           "finance.withdraw.approve",
//       ),
//       approveWithdrawalHandler,
//   )
//
// IMPORTANT SECURITY NOTES:
// - Does NOT fall back to admin role checking
// - Only grants access if actor has ALL of the listed capabilities
// - Empty capability list will always allow (logical AND over empty set is true)
func RequireAllCapabilities(requiredCapabilities ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Step 1: Get actor from context
		actor := capability.GetActor(ctx)
		if actor == nil {
			// No actor in context - user is not authenticated
			response.Unauthorized(c, "Authentication required")
			c.Abort()
			return
		}

		// Step 2: Check if actor has all of the required capabilities
		if !actor.HasAllCapabilities(requiredCapabilities...) {
			// Actor doesn't have all the capabilities - forbid access
			response.Forbidden(c, "Insufficient permissions")
			c.Abort()
			return
		}

		// Actor has all the capabilities - continue to handler
		c.Next()
	}
}

// ============================================================
// ADVANCED MIDDLEWARE: CUSTOM AUTHORIZATION LOGIC
// ============================================================

// RequireActor creates middleware that requires an authenticated actor
// but doesn't check for any specific capability.
//
// This is useful for endpoints that need to know the user's identity
// but don't require specific capabilities. For example, a profile
// update endpoint might only require that the user is authenticated.
//
// Use this when you need authentication but not authorization.
func RequireActor() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		actor := capability.GetActor(ctx)
		if actor == nil {
			response.Unauthorized(c, "Authentication required")
			c.Abort()
			return
		}

		c.Next()
	}
}

// WithCapabilityCheck creates middleware that runs a custom authorization
// function against the actor.
//
// This allows for more complex authorization logic that can't be expressed
// with simple capability checking. For example, checking if the actor can
// access a specific resource based on resource ownership:
//
//   router.GET("/documents/:id",
//       middleware.WithCapabilityCheck(func(actor *entity.Actor, c *gin.Context) bool {
//           docID := c.Param("id")
//           return actor.HasCapability("document.read") ||
//               isDocumentOwner(docID, actor.ID)
//       }),
//       documentHandler,
//   )
func WithCapabilityCheck(check func(*capabilityEntity.Actor, *gin.Context) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		actor := capability.GetActor(ctx)
		if actor == nil {
			response.Unauthorized(c, "Authentication required")
			c.Abort()
			return
		}

		if !check(actor, c) {
			response.Forbidden(c, "Insufficient permissions")
			c.Abort()
			return
		}

		c.Next()
	}
}

// ============================================================
// RESPONSE HELPERS
// ============================================================

// CapabilityRequiredResponse sends a 403 response for missing capabilities.
// This is a helper that can be used in handlers for more complex authorization logic.
//
// Example:
//   func MyHandler(c *gin.Context) {
//       actor := middleware.GetActorFromContext(c)
//       if actor == nil {
//           middleware.UnauthorizedResponse(c)
//           return
//       }
//       if !actor.HasCapability("some.capability") {
//           middleware.CapabilityRequiredResponse(c)
//           return
//       }
//       // ... handler logic
//   }
func CapabilityRequiredResponse(c *gin.Context) {
	response.Forbidden(c, "Insufficient permissions")
}

// UnauthorizedResponse sends a 401 response for missing authentication.
func UnauthorizedResponse(c *gin.Context) {
	response.Unauthorized(c, "Authentication required")
}


