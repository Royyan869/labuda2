// Package entity defines the domain entities for the capability system.
package entity

import (
	"time"

	"github.com/google/uuid"
)

// UserCapability represents a capability granted to a user.
//
// INVARIANTS:
// - A user can only have one active instance of each capability
// - Capabilities are soft-revoked via revoked_at timestamp
// - History is preserved for audit trail
//
// LIFECYCLE:
// - Created: granted_at set, revoked_at is NULL
// - Revoked: revoked_at set to timestamp
// - Expired: revoked_at is NOT NULL
type UserCapability struct {
	// ID is the unique identifier for this capability grant
	ID uuid.UUID

	// UserID is the user who has been granted this capability
	UserID uuid.UUID

	// Capability is the capability string (e.g., "finance.withdraw.read")
	// These constants are defined in capability package
	Capability string

	// GrantedBy is the user who granted this capability (nullable for system grants)
	GrantedBy *uuid.UUID

	// GrantedAt is when this capability was granted
	GrantedAt time.Time

	// RevokedAt is when this capability was revoked (NULL if active)
	// A capability is considered ACTIVE if and only if revoked_at is NULL
	RevokedAt *time.Time
}

// IsActive returns true if this capability is currently active.
// A capability is active when it has not been revoked.
func (uc *UserCapability) IsActive() bool {
	return uc.RevokedAt == nil
}

// IsRevoked returns true if this capability has been revoked.
func (uc *UserCapability) IsRevoked() bool {
	return uc.RevokedAt != nil
}

// NewCapabilityGrant creates a new active capability grant.
//
// Parameters:
// - userID: The user receiving the capability
// - capability: The capability string (must be a valid capability constant)
// - grantedBy: The user granting the capability (nil for system grants)
//
// Returns a UserCapability with:
// - ID: generated UUID
// - GrantedAt: set to current time
// - RevokedAt: nil (active)
func NewCapabilityGrant(userID uuid.UUID, capability string, grantedBy *uuid.UUID) *UserCapability {
	return &UserCapability{
		ID:         uuid.New(),
		UserID:     userID,
		Capability: capability,
		GrantedBy:  grantedBy,
		GrantedAt:  time.Now(),
		RevokedAt:  nil,
	}
}

// Revoke marks this capability as revoked at the specified time.
// This is a soft revoke - the record is preserved for audit.
func (uc *UserCapability) Revoke(revokedAt time.Time) {
	uc.RevokedAt = &revokedAt
}

// ============================================================
// ERRORS
// ============================================================

// ErrDuplicateCapability is returned when attempting to grant a capability
// that the user already has active.
type ErrDuplicateCapability struct {
	UserID     uuid.UUID
	Capability string
}

func (e *ErrDuplicateCapability) Error() string {
	return "user already has this active capability"
}

// ErrCapabilityNotFound is returned when attempting to revoke a capability
// that the user does not have.
type ErrCapabilityNotFound struct {
	UserID     uuid.UUID
	Capability string
}

func (e *ErrCapabilityNotFound) Error() string {
	return "capability not found for user"
}

// ErrInvalidCapability is returned when an invalid capability string is used.
type ErrInvalidCapability struct {
	Capability string
}

func (e *ErrInvalidCapability) Error() string {
	return "invalid capability string"
}


