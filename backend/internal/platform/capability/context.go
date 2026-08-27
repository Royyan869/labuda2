// Package capability provides context helpers for actor injection and retrieval.
//
// SLICE 2: ACTOR CONTEXT INJECTION
// - Context helpers are used by ActorContextInject middleware
// - Actor is injected into request context after authentication
// - Safe access patterns are enforced at compile time
//
// DESIGN PRINCIPLES:
// - Safe access: All public helpers return nil instead of panicking
// - Explicit functions: No magic string keys
// - Type-safe: Uses private struct type as context key to prevent collisions
//
// ACCESS PATTERNS:
// 1. PREFERRED: GetActor() - Returns nil if not found, always safe
// 2. PREFERRED: HasCapability() / HasAnyCapability() - Safe convenience wrappers
// 3. AVOID: MustGetActor() - Panics if not found, use only in protected code paths
package capability

import (
	"context"

	"github.com/labuda/backend/internal/platform/capability/entity"
)

// actorKey is the context key type for storing Actor.
// Using a private struct type prevents key collisions.
type actorKey struct{}

// ActorContextKey is the canonical context key for storing Actor.
// Use WithActor to set and GetActor to retrieve.
var ActorContextKey = actorKey{}

// WithActor returns a new context with the Actor stored.
//
// This is used by ActorContextInject middleware to inject actor
// into the request context after authentication.
//
// Example:
//   ctx := capability.WithActor(c.Request.Context(), actor)
//   c.Request = c.Request.WithContext(ctx)
func WithActor(ctx context.Context, actor *entity.Actor) context.Context {
	return context.WithValue(ctx, ActorContextKey, actor)
}

// GetActor retrieves the Actor from the context.
//
// Returns nil if actor is not present in context.
// This is safe to call even if actor hasn't been injected yet.
//
// This is the PREFERRED way to access actor from context.
// It never panics and always returns a valid pointer (or nil).
//
// Example:
//   actor := capability.GetActor(c.Request.Context())
//   if actor != nil {
//       if actor.HasCapability("finance.withdraw.read") {
//           // Allow access
//       }
//   }
func GetActor(ctx context.Context) *entity.Actor {
	if actor, ok := ctx.Value(ActorContextKey).(*entity.Actor); ok {
		return actor
	}
	return nil
}

// MustGetActor retrieves the Actor from context or panics.
//
// AVOID USING THIS FUNCTION in most cases.
// This should ONLY be used in code paths that are GUARANTEED to have actor,
// such as after RequireActor or RequireCapability middleware has run.
//
// For most code, use GetActor() instead and handle the nil case.
//
// Example (ONLY after RequireActor middleware):
//   actor := capability.MustGetActor(c.Request.Context())
//   // At this point, we know actor exists because RequireActor already checked
func MustGetActor(ctx context.Context) *entity.Actor {
	actor := GetActor(ctx)
	if actor == nil {
		panic("actor not found in context: middleware chain may be misconfigured")
	}
	return actor
}

// HasCapability is a convenience function that checks if the actor
// in the context has a specific capability.
//
// Returns false if no actor is present in context.
//
// Example:
//   if capability.HasCapability(ctx, "finance.withdraw.read") {
//       // Allow access
//   }
func HasCapability(ctx context.Context, cap string) bool {
	actor := GetActor(ctx)
	if actor == nil {
		return false
	}
	return actor.HasCapability(cap)
}

// HasAnyCapability is a convenience function that checks if the actor
// in the context has any of the specified capabilities.
//
// Returns false if no actor is present in context.
func HasAnyCapability(ctx context.Context, caps ...string) bool {
	actor := GetActor(ctx)
	if actor == nil {
		return false
	}
	return actor.HasAnyCapability(caps...)
}


