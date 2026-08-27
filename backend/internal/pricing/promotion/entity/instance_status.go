package entity

import (
	"fmt"
)

// InstanceStatus represents the status of a promotion instance.
// Lifecycle: inactive -> active -> paused/expired/cancelled
type InstanceStatus string

const (
	// InstanceStatusInactive means the instance was created but not yet activated.
	InstanceStatusInactive InstanceStatus = "inactive"

	// InstanceStatusActive means the instance is currently promoting the target.
	InstanceStatusActive InstanceStatus = "active"

	// InstanceStatusPaused means the instance was paused by the user.
	// Can be reactivated.
	InstanceStatusPaused InstanceStatus = "paused"

	// InstanceStatusExpired means the instance's duration was consumed.
	// Terminal state.
	InstanceStatusExpired InstanceStatus = "expired"

	// InstanceStatusCancelled means the instance was cancelled.
	// Terminal state.
	InstanceStatusCancelled InstanceStatus = "cancelled"
)

// IsValid returns true if the instance status is valid.
func (s InstanceStatus) IsValid() bool {
	switch s {
	case InstanceStatusInactive, InstanceStatusActive, InstanceStatusPaused, InstanceStatusExpired, InstanceStatusCancelled:
		return true
	default:
		return false
	}
}

// IsTerminal returns true if the status is a terminal state.
func (s InstanceStatus) IsTerminal() bool {
	return s == InstanceStatusExpired || s == InstanceStatusCancelled
}

// IsActive returns true if the instance is currently promoting.
func (s InstanceStatus) IsActive() bool {
	return s == InstanceStatusActive
}

// CanActivate returns true if the instance can be activated.
func (s InstanceStatus) CanActivate() bool {
	return s == InstanceStatusInactive || s == InstanceStatusPaused
}

// String returns the string representation of the instance status.
func (s InstanceStatus) String() string {
	return string(s)
}

// InvalidInstanceStatusError is returned when an invalid instance status is provided.
type InvalidInstanceStatusError struct {
	Status InstanceStatus
}

func (e *InvalidInstanceStatusError) Error() string {
	return fmt.Sprintf("invalid instance status: %s", e.Status)
}

// InstanceTransitionError is returned when attempting an invalid status transition.
type InstanceTransitionError struct {
	From InstanceStatus
	To   InstanceStatus
}

func (e *InstanceTransitionError) Error() string {
	return fmt.Sprintf("invalid instance status transition: %s -> %s", e.From, e.To)
}
