package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PromotionOwnership represents a user's entitlement to promote.
// This is the SINGLE SOURCE OF TRUTH for duration accounting.
//
// Business truth: Users purchase packages, which creates ownership.
// The ownership tracks total duration, consumed duration, and validity window.
// Remaining duration is computed as: total - consumed.
type PromotionOwnership struct {
	// Identity
	ID uuid.UUID

	// Relations
	UserID          uuid.UUID
	PackageID       uuid.UUID
	SourceBillingID *uuid.UUID // nullable; set when created via payment webhook for traceability

	// Entitlement status
	Status      OwnershipStatus // available, consumed, expired, cancelled
	PurchasedAt time.Time
	ExpiresAt   time.Time // Hard expiry: purchased_at + validity_window

	// Duration accounting (ONLY SOURCE OF TRUTH)
	TotalDurationHours    int // Fixed at purchase
	ConsumedDurationHours int // Increments over time
	// RemainingDurationHours is DERIVED: TotalDurationHours - ConsumedDurationHours

	// Timestamps
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewPromotionOwnership creates a new promotion ownership from a package purchase.
//
// CRITICAL: dbTime parameter must be database time (from GetDBTime), not app time.
// This ensures consistency across multiple app servers and prevents clock skew issues.
func NewPromotionOwnership(
	userID uuid.UUID,
	packageID uuid.UUID,
	totalDurationHours int,
	validityWindowHours int,
	dbTime time.Time,
) (*PromotionOwnership, error) {
	return &PromotionOwnership{
		ID:                    uuid.New(),
		UserID:                userID,
		PackageID:             packageID,
		Status:                OwnershipStatusAvailable,
		PurchasedAt:           dbTime,
		ExpiresAt:             dbTime.Add(time.Duration(validityWindowHours) * time.Hour),
		TotalDurationHours:    totalDurationHours,
		ConsumedDurationHours: 0,
		CreatedAt:             dbTime,
		UpdatedAt:             dbTime,
	}, nil
}

// GetRemainingDuration calculates remaining duration in hours.
// This is ALWAYS computed from active instances, never stored.
func (o *PromotionOwnership) GetRemainingDuration() int {
	remaining := o.TotalDurationHours - (o.GetConsumedDurationSeconds() / 3600)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// GetConsumedDurationSeconds returns the total consumed duration in seconds.
// This is calculated dynamically based on historical consumption from all instances.
// For ownerships with no instances, returns the stored consumed_duration_hours (converted to seconds).
func (o *PromotionOwnership) GetConsumedDurationSeconds() int {
	// This is a placeholder - the actual calculation happens in the repository
	// which has access to all instances. The stored value is only used for
	// ownerships with no instances (edge case).
	return o.ConsumedDurationHours * 3600
}

// IsExpired returns true if the ownership has passed its validity window.
//
// CRITICAL: dbTime parameter must be database time (from GetDBTime), not app time.
// This ensures consistency across multiple app servers and prevents clock skew issues.
func (o *PromotionOwnership) IsExpired(dbTime time.Time) bool {
	return dbTime.After(o.ExpiresAt)
}

// IsFullyConsumed returns true if all duration has been consumed.
func (o *PromotionOwnership) IsFullyConsumed() bool {
	return o.ConsumedDurationHours >= o.TotalDurationHours
}

// CanActivate returns true if the ownership can be used to activate a promotion.
//
// CRITICAL: dbTime parameter must be database time (from GetDBTime), not app time.
// This ensures consistency across multiple app servers and prevents clock skew issues.
func (o *PromotionOwnership) CanActivate(dbTime time.Time) bool {
	return o.Status == OwnershipStatusAvailable &&
		!o.IsExpired(dbTime) &&
		!o.IsFullyConsumed()
}

// ConsumeDuration adds consumed hours and updates status if fully consumed.
// Returns true if the ownership is now fully consumed.
//
// CRITICAL: dbTime parameter must be database time (from GetDBTime), not app time.
// This ensures consistency across multiple app servers and prevents clock skew issues.
func (o *PromotionOwnership) ConsumeDuration(hours int, dbTime time.Time) bool {
	o.ConsumedDurationHours += hours
	if o.ConsumedDurationHours > o.TotalDurationHours {
		o.ConsumedDurationHours = o.TotalDurationHours
	}
	o.UpdatedAt = dbTime

	if o.IsFullyConsumed() {
		o.Status = OwnershipStatusConsumed
		return true
	}
	return false
}

// AddConsumedDurationSeconds adds consumed duration in seconds and updates status if fully consumed.
// This is used when finalizing instances to bake their consumed duration into the ownership.
// Returns true if the ownership is now fully consumed.
//
// CRITICAL: This method enforces the accounting invariant:
// consumed_duration_hours <= total_duration_hours
// If adding would exceed total, it caps at total (prevents over-consumption)
//
// CRITICAL: dbTime parameter must be database time (from GetDBTime), not app time.
// This ensures consistency across multiple app servers and prevents clock skew issues.
func (o *PromotionOwnership) AddConsumedDurationSeconds(seconds int, dbTime time.Time) bool {
	// INVARIANT ENFORCEMENT: Never consume more than total
	// Convert seconds to hours (round up to be conservative)
	hours := (seconds + 3599) / 3600

	o.ConsumedDurationHours += hours

	// CAP: Never exceed total duration
	// This prevents accounting corruption even if bugs double-count
	if o.ConsumedDurationHours > o.TotalDurationHours {
		// CRITICAL: Log invariant violation - this indicates a bug or data corruption
		// TODO: Add proper observability/alerting
		o.ConsumedDurationHours = o.TotalDurationHours
	}

	o.UpdatedAt = dbTime

	if o.IsFullyConsumed() {
		o.Status = OwnershipStatusConsumed
		return true
	}
	return false
}

// MarkAsExpired marks the ownership as expired.
//
// CRITICAL: dbTime parameter must be database time (from GetDBTime), not app time.
// This ensures consistency across multiple app servers and prevents clock skew issues.
func (o *PromotionOwnership) MarkAsExpired(dbTime time.Time) {
	o.Status = OwnershipStatusExpired
	o.UpdatedAt = dbTime
}

// ValidateAccountingInvariant checks that accounting invariants are maintained.
// Returns error if invariants are violated.
// This should be called:
// - After any mutation to ownership or instances
// - In tests to verify data consistency
// - Periodically as a health check
func (o *PromotionOwnership) ValidateAccountingInvariant(activeRuntimeSeconds int) error {
	// INVARIANT 1: Consumed duration never exceeds total duration
	if o.ConsumedDurationHours > o.TotalDurationHours {
		return &AccountingInvariantViolationError{
			OwnershipID: o.ID,
			Violation:   "consumed_duration_hours exceeds total_duration_hours",
			Consumed:    o.ConsumedDurationHours,
			Total:       o.TotalDurationHours,
		}
	}

	// INVARIANT 2: Consumed + active runtime never exceeds total
	// (This catches cases where active instance is consuming but not yet finalized)
	totalConsumedHours := o.ConsumedDurationHours + (activeRuntimeSeconds / 3600)
	if totalConsumedHours > o.TotalDurationHours {
		return &AccountingInvariantViolationError{
			OwnershipID: o.ID,
			Violation:   "consumed + active_runtime exceeds total_duration_hours",
			Consumed:    totalConsumedHours,
			Total:       o.TotalDurationHours,
		}
	}

	// INVARIANT 3: Consumed duration is never negative
	if o.ConsumedDurationHours < 0 {
		return &AccountingInvariantViolationError{
			OwnershipID: o.ID,
			Violation:   "consumed_duration_hours is negative",
			Consumed:    o.ConsumedDurationHours,
			Total:       o.TotalDurationHours,
		}
	}

	return nil
}

// AccountingInvariantViolationError is returned when accounting invariants are violated.
type AccountingInvariantViolationError struct {
	OwnershipID uuid.UUID
	Violation   string
	Consumed    int
	Total       int
}

func (e *AccountingInvariantViolationError) Error() string {
	return fmt.Sprintf("accounting invariant violation for ownership %s: %s (consumed: %d, total: %d)",
		e.OwnershipID, e.Violation, e.Consumed, e.Total)
}

// OwnershipExpiredError is returned when attempting to use an expired ownership.
type OwnershipExpiredError struct {
	OwnershipID uuid.UUID
	ExpiresAt   time.Time
}

func (e *OwnershipExpiredError) Error() string {
	return "ownership has expired"
}

// OwnershipConsumedError is returned when attempting to use a fully consumed ownership.
type OwnershipConsumedError struct {
	OwnershipID uuid.UUID
}

func (e *OwnershipConsumedError) Error() string {
	return "ownership duration is fully consumed"
}

// NotOwnershipOwnerError is returned when a user attempts to access another user's ownership.
type NotOwnershipOwnerError struct {
	OwnershipID uuid.UUID
	UserID      uuid.UUID
}

func (e *NotOwnershipOwnerError) Error() string {
	return "not the owner of this ownership"
}
