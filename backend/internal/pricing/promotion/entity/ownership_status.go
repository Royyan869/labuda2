package entity

import (
	"fmt"
)

// OwnershipStatus represents the status of a promotion ownership (entitlement).
// Lifecycle: available -> consumed/expired/cancelled
type OwnershipStatus string

const (
	// OwnershipStatusAvailable means the ownership is ready to use.
	// User can activate promotions with this ownership.
	OwnershipStatusAvailable OwnershipStatus = "available"

	// OwnershipStatusConsumed means the ownership's duration has been fully consumed.
	// Terminal state.
	OwnershipStatusConsumed OwnershipStatus = "consumed"

	// OwnershipStatusExpired means the ownership's validity window has passed.
	// Terminal state.
	OwnershipStatusExpired OwnershipStatus = "expired"

	// OwnershipStatusCancelled means the ownership was cancelled by admin or user.
	// Terminal state.
	OwnershipStatusCancelled OwnershipStatus = "cancelled"
)

// IsValid returns true if the ownership status is valid.
func (s OwnershipStatus) IsValid() bool {
	switch s {
	case OwnershipStatusAvailable, OwnershipStatusConsumed, OwnershipStatusExpired, OwnershipStatusCancelled:
		return true
	default:
		return false
	}
}

// IsTerminal returns true if the status is a terminal state (no further transitions possible).
func (s OwnershipStatus) IsTerminal() bool {
	return s == OwnershipStatusConsumed || s == OwnershipStatusExpired || s == OwnershipStatusCancelled
}

// CanActivate returns true if the ownership can be used to activate a promotion.
func (s OwnershipStatus) CanActivate() bool {
	return s == OwnershipStatusAvailable
}

// String returns the string representation of the ownership status.
func (s OwnershipStatus) String() string {
	return string(s)
}

// InvalidOwnershipStatusError is returned when an invalid ownership status is provided.
type InvalidOwnershipStatusError struct {
	Status OwnershipStatus
}

func (e *InvalidOwnershipStatusError) Error() string {
	return fmt.Sprintf("invalid ownership status: %s", e.Status)
}

// OwnershipTransitionError is returned when attempting an invalid status transition.
type OwnershipTransitionError struct {
	From OwnershipStatus
	To   OwnershipStatus
}

func (e *OwnershipTransitionError) Error() string {
	return fmt.Sprintf("invalid ownership status transition: %s -> %s", e.From, e.To)
}
