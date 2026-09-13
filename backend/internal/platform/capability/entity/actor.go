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

// AdminRole is the single canonical value of admin membership in users.role.
//
// It lives in this lowest capability layer (entity) so that BOTH the derived
// full-access authority (capability.IsFullAccessAdmin) and the Actor authority
// predicate (Actor.IsAdmin) share exactly one definition without an import
// cycle. There is no second declaration of this value anywhere.
const AdminRole = "admin"

// Actor represents an authenticated user with their role, capabilities, and business state.
//
// DESIGN PRINCIPLES:
// - Complete: Contains ID, Role, Capabilities, and Business State
// - Immutable: All fields are read-only after creation
//
// ROLE VS CAPABILITIES (canonical Labuda authority model):
// - Role ("admin") is the coarse internal membership boundary ONLY. It never
//   grants capabilities implicitly and never authorizes a privileged
//   business action by itself.
// - Capabilities are fine-grained grants independent of role. A privileged
//   internal business action requires admin membership AND the explicit
//   required capability.
type Actor struct {
	// ID is the user's unique identifier
	ID uuid.UUID

	// Role is the user's internal membership role from users.role (user, admin).
	// It is the coarse internal boundary only — never an implicit capability
	// source. See the Actor doc comment for the canonical authority model.
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
	return a.Role == AdminRole
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
