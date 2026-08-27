// Package infrastructure provides the implementation for actor resolution.
package infrastructure

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/labuda/backend/internal/platform/capability/repository"
)

// ActorResolverImpl resolves an Actor from a user ID using database lookups.
//
// DESIGN PRINCIPLES:
// - Loads role, capabilities, and business state (user + seller)
// - Returns explicit Actor with all data loaded
// - No caching in Slice 2 (can be added later)
// - All values come from database - NO HARDCODING
type ActorResolverImpl struct {
	capabilityRepo   repository.CapabilityRepository
	userStateQuerier UserStateQuerier
}

// UserStateQuerier defines how to get a user's complete state.
// This abstraction avoids circular dependency with other packages.
//
// CRITICAL: All state must be loaded atomically in a SINGLE query to prevent
// race conditions and inconsistency.
type UserStateQuerier interface {
	// GetUserState returns the user's role, account status, identity state, and seller subscription status.
	// All values come from a SINGLE atomic query - no separate queries allowed.
	// sellerStatus is nil if the user has no seller subscription, otherwise one of: active, expired.
	GetUserState(ctx context.Context, userID uuid.UUID) (role, accountStatus string, emailVerified, isIdentityComplete bool, sellerStatus *string, err error)
}

// NewActorResolver creates a new ActorResolver.
func NewActorResolver(
	capabilityRepo repository.CapabilityRepository,
	userStateQuerier UserStateQuerier,
) entity.ActorResolver {
	return &ActorResolverImpl{
		capabilityRepo:   capabilityRepo,
		userStateQuerier: userStateQuerier,
	}
}

// ResolveActor builds an Actor with role, capabilities, and business state for the given user.
//
// CRITICAL: User state (role, account status, profile, seller subscription) is loaded
// atomically from a SINGLE database query to prevent race conditions and inconsistency.
func (r *ActorResolverImpl) ResolveActor(ctx interface{}, userID uuid.UUID) (*entity.Actor, error) {
	// Extract context from interface if needed
	var contextCtx context.Context
	switch v := ctx.(type) {
	case context.Context:
		contextCtx = v
	default:
		contextCtx = context.Background()
	}

	// Step 1: Load user state atomically from SINGLE query (role, account status, profile state, seller subscription status)
	role, accountStatus, emailVerified, isIdentityComplete, sellerStatus, err := r.userStateQuerier.GetUserState(contextCtx, userID)
	if err != nil {
		// If user not found, return ActorNotFound error
		if err.Error() == "no rows in result set" || err.Error() == "user not found" {
			return nil, &entity.ActorNotFound{UserID: userID}
		}
		return nil, fmt.Errorf("get user state: %w", err)
	}

	// Step 2: Load active capabilities
	userCaps, err := r.capabilityRepo.ListActiveCapabilities(contextCtx, nil, userID)
	if err != nil {
		return nil, fmt.Errorf("list active capabilities: %w", err)
	}

	// Step 3: Build capability strings
	capabilities := make([]string, len(userCaps))
	for i, uc := range userCaps {
		capabilities[i] = uc.Capability
	}

	// Step 4: Return Actor with all business state loaded atomically
	return &entity.Actor{
		ID:                 userID,
		Role:               role,
		Capabilities:       capabilities,
		EmailVerified:      emailVerified,
		IsIdentityComplete: isIdentityComplete,
		AccountStatus:      accountStatus,
		SellerStatus:       sellerStatus,
	}, nil
}
