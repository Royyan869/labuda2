package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// UserWarning represents a warning issued to a user for policy violations.
//
// Warnings are issued by admins when users violate platform policies.
// Multiple warnings can lead to account restrictions or bans.
type UserWarning struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	IssuedBy  uuid.UUID // Admin who issued the warning
	Level     WarningLevel
	Reason    string     // Explanation for the warning
	IsActive  bool       // Whether the warning is currently active
	RevokedAt *time.Time // When the warning was revoked (if applicable)
	RevokedBy *uuid.UUID // Admin who revoked the warning (if applicable)
	CreatedAt time.Time
	ExpiresAt *time.Time // Optional expiration date for temporary warnings
}

// WarningLevel represents the severity of a warning.
type WarningLevel string

const (
	// WarningLevelInfo is for minor policy reminders.
	WarningLevelInfo WarningLevel = "info"

	// WarningLevelWarning is for moderate policy violations.
	WarningLevelWarning WarningLevel = "warning"

	// WarningLevelSevere is for serious policy violations.
	WarningLevelSevere WarningLevel = "severe"
)

// IsValid returns true if the warning level is valid.
func (w WarningLevel) IsValid() bool {
	return w == WarningLevelInfo || w == WarningLevelWarning || w == WarningLevelSevere
}

// WarningStatus represents the current status of a warning.
type WarningStatus string

const (
	// WarningStatusActive means the warning is currently active.
	WarningStatusActive WarningStatus = "active"

	// WarningStatusRevoked means the warning has been revoked.
	WarningStatusRevoked WarningStatus = "revoked"

	// WarningStatusExpired means the warning has expired.
	WarningStatusExpired WarningStatus = "expired"
)

// ErrWarningNotFound is returned when a warning cannot be found.
type ErrWarningNotFound struct {
	WarningID uuid.UUID
}

func (e *ErrWarningNotFound) Error() string {
	return fmt.Sprintf("warning not found: %s", e.WarningID)
}

// ErrWarningTargetNotFound is returned when attempting to issue a warning to a missing user.
type ErrWarningTargetNotFound struct {
	UserID uuid.UUID
}

func (e *ErrWarningTargetNotFound) Error() string {
	return fmt.Sprintf("warning target user not found: %s", e.UserID)
}

// ErrWarningAlreadyRevoked is returned when attempting to revoke an already revoked warning.
type ErrWarningAlreadyRevoked struct {
	WarningID uuid.UUID
}

func (e *ErrWarningAlreadyRevoked) Error() string {
	return fmt.Sprintf("warning already revoked: %s", e.WarningID)
}

// Revoke marks the warning as revoked.
func (w *UserWarning) Revoke(revokedBy uuid.UUID) error {
	if !w.IsActive {
		return &ErrWarningAlreadyRevoked{WarningID: w.ID}
	}

	now := time.Now()
	w.IsActive = false
	w.RevokedAt = &now
	w.RevokedBy = &revokedBy

	return nil
}

// GetStatus returns the current status of the warning.
func (w *UserWarning) GetStatus() WarningStatus {
	if !w.IsActive {
		if w.RevokedAt != nil {
			return WarningStatusRevoked
		}
	}
	if w.ExpiresAt != nil && time.Now().After(*w.ExpiresAt) {
		return WarningStatusExpired
	}
	return WarningStatusActive
}

// IsExpired returns true if the warning has expired.
func (w *UserWarning) IsExpired() bool {
	if w.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*w.ExpiresAt)
}


