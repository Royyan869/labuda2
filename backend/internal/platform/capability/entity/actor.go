// Package entity defines the Actor entity for the capability system.
package entity

import (
	"github.com/google/uuid"
)

// SellerStatus represents the status of a seller subscription.
type SellerStatus string

const (
	// SellerStatusActive indicates the seller subscription is active and can perform seller actions.
	SellerStatusActive SellerStatus = "active"
	// SellerStatusExpired indicates the seller subscription has expired.
	SellerStatusExpired SellerStatus = "expired"
	// SellerStatusNone indicates the user has no seller subscription.
	SellerStatusNone SellerStatus = "none"
)

// Actor represents an authenticated user with their role, capabilities, and business state.
//
// DESIGN PRINCIPLES:
// - Complete: Contains ID, Role, Capabilities, and Business State
// - Immutable: All fields are read-only after creation
// - Foundation: Prepared for context injection in future slices
//
// SLICE 2 SCOPE:
// - Actor is now aware of business state (user + seller)
// - Actor is the SINGLE SOURCE of capability logic
type Actor struct {
	// ID is the user's unique identifier
	ID uuid.UUID

	// Role is the user's role from users.role (user, admin)
	// This is kept separate from capabilities for backward compatibility
	Role string

	// Capabilities is the list of active capabilities granted to this user
	// These are fine-grained permissions independent of role
	Capabilities []string

	// Business State - loaded from database

	// EmailVerified indicates the user has verified their email address
	EmailVerified bool

	// IsIdentityComplete indicates the user has established Layer B identity.
	IsIdentityComplete bool

	// AccountStatus is the user's account status (active, suspended, banned)
	AccountStatus string

	// SellerStatus is the status of the seller subscription (active, expired, nil if not a seller)
	SellerStatus *string
}

// ActorResolver defines the interface for resolving an Actor from a user ID.
type ActorResolver interface {
	// ResolveActor builds an Actor with role and capabilities for the given user.
	// Returns an error if the user doesn't exist.
	ResolveActor(ctx interface{}, userID uuid.UUID) (*Actor, error)
}

// ActorNotFound is returned when a user cannot be found during actor resolution.
type ActorNotFound struct {
	UserID uuid.UUID
}

func (e *ActorNotFound) Error() string {
	return "actor not found: user does not exist"
}

// HasCapability checks if the actor has a specific capability.
func (a *Actor) HasCapability(capability string) bool {
	for _, c := range a.Capabilities {
		if c == capability {
			return true
		}
	}
	return false
}

// HasAnyCapability checks if the actor has any of the specified capabilities.
func (a *Actor) HasAnyCapability(capabilities ...string) bool {
	if len(capabilities) == 0 {
		return false
	}

	capMap := make(map[string]bool, len(a.Capabilities))
	for _, c := range a.Capabilities {
		capMap[c] = true
	}

	for _, cap := range capabilities {
		if capMap[cap] {
			return true
		}
	}
	return false
}

// HasAllCapabilities checks if the actor has all of the specified capabilities.
func (a *Actor) HasAllCapabilities(capabilities ...string) bool {
	if len(capabilities) == 0 {
		return true
	}

	capMap := make(map[string]bool, len(a.Capabilities))
	for _, c := range a.Capabilities {
		capMap[c] = true
	}

	for _, cap := range capabilities {
		if !capMap[cap] {
			return false
		}
	}
	return true
}

// IsAdmin returns true if the actor has admin role.
func (a *Actor) IsAdmin() bool {
	return a.Role == "admin"
}

// ============================================================================
// STATE HELPERS
// ============================================================================

// IsProfileReady returns true when the user has established Layer B identity.
func (a *Actor) IsProfileReady() bool {
	return a.IsIdentityComplete
}

// IsSellerReady returns true if the user has an active seller subscription.
// Active subscription means:
// - SellerStatus is not nil (has a subscription)
// - Status is "active" (can perform seller actions)
// - Account status is active
func (a *Actor) IsSellerReady() bool {
	if a.AccountStatus != "active" {
		return false
	}
	if a.SellerStatus == nil {
		return false
	}
	status := *a.SellerStatus
	return status == string(SellerStatusActive)
}

// ============================================================================
// CAPABILITY METHODS
// ============================================================================

// CanCreateForSale returns true if the actor can create a for_sale item.
//
// Requirements:
// - Account must be active
// - Email must be verified
// - Seller subscription must be active (not nil and not expired)
func (a *Actor) CanCreateForSale() bool {
	if a.AccountStatus != "active" {
		return false
	}
	if !a.EmailVerified {
		return false
	}
	if !a.IsSellerReady() {
		return false
	}
	return true
}

// CanCheckout returns true if the actor can checkout.
//
// Requirements:
// - Account must be active
// - Email must be verified
// - Profile must be completed with custom username
func (a *Actor) CanCheckout() bool {
	if a.AccountStatus != "active" {
		return false
	}
	if !a.EmailVerified {
		return false
	}
	if !a.IsProfileReady() {
		return false
	}
	return true
}
