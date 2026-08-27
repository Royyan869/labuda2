package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PromotionInstance represents an active promotion on a specific target.
//
// Business truth: An instance is the active use of an ownership on a target.
// Only one instance can be active per ownership at a time.
// Duration stays at ownership level - instance is just a pointer to the target.
type PromotionInstance struct {
	// Identity
	ID uuid.UUID

	// Relations
	OwnershipID uuid.UUID
	UserID      uuid.UUID

	// Target binding
	TargetType TargetType // for_sale, auction, external_product
	TargetID   *uuid.UUID // target entity ID for all public target types

	// Status & timing
	Status      InstanceStatus // inactive, active, paused, expired, cancelled
	ActivatedAt *time.Time
	StoppedAt   *time.Time
	StopReason  *string // NULL, 'user_paused', 'target_sold', etc.

	// Pause tracking for wall-clock duration calculation
	PausedAt            *time.Time // When the instance was paused
	TotalPausedDuration int        // Total seconds spent in paused state

	// Finalization tracking for accounting safety
	// When an instance stops, its consumed duration is "baked into" the ownership's
	// consumed_duration_hours to prevent double counting and ensure durability.
	Finalized        bool       // If true, duration is already accounted for in ownership
	FinalizedAt      *time.Time // When the instance was finalized
	FinalizedSeconds int        // The consumed duration (in seconds) that was added to ownership

	// Timestamps
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewPromotionInstance creates a new promotion instance.
// For external_product, targetID should point at external_products.id.
//
// CRITICAL: dbTime parameter must be database time (from GetDBTime), not app time.
// This ensures consistency across multiple app servers and prevents clock skew issues.
func NewPromotionInstance(
	ownershipID uuid.UUID,
	userID uuid.UUID,
	targetType TargetType,
	targetID *uuid.UUID,
	dbTime time.Time,
) (*PromotionInstance, error) {
	if !targetType.IsValid() {
		return nil, &InvalidTargetTypeError{TargetType: targetType}
	}

	// Validate target_id for all promotable target types.
	if targetType.RequiresTargetID() && (targetID == nil || *targetID == uuid.Nil) {
		return nil, &ValidationError{Field: "target_id", Message: "required for this target type"}
	}

	return &PromotionInstance{
		ID:          uuid.New(),
		OwnershipID: ownershipID,
		UserID:      userID,
		TargetType:  targetType,
		TargetID:    targetID,
		Status:      InstanceStatusInactive,
		CreatedAt:   dbTime,
		UpdatedAt:   dbTime,
	}, nil
}

// Activate marks the instance as active.
//
// CRITICAL: dbTime parameter must be database time (from GetDBTime), not app time.
// This ensures consistency across multiple app servers and prevents clock skew issues.
func (i *PromotionInstance) Activate(dbTime time.Time) error {
	if !i.Status.CanActivate() {
		return &InstanceTransitionError{From: i.Status, To: InstanceStatusActive}
	}

	i.Status = InstanceStatusActive
	i.ActivatedAt = &dbTime
	i.StoppedAt = nil
	i.StopReason = nil
	i.UpdatedAt = dbTime

	return nil
}

// Pause marks the instance as paused (user-initiated).
// Records the pause time for duration calculation.
//
// CRITICAL: dbTime parameter must be database time (from GetDBTime), not app time.
// This ensures consistency across multiple app servers and prevents clock skew issues.
func (i *PromotionInstance) Pause(dbTime time.Time) error {
	if i.Status != InstanceStatusActive {
		return &InstanceTransitionError{From: i.Status, To: InstanceStatusPaused}
	}

	// IDEMPOTENCY GUARD: Prevent double pause
	// If already paused, return error instead of overwriting PausedAt
	if i.PausedAt != nil {
		return fmt.Errorf("instance already paused")
	}

	i.Status = InstanceStatusPaused
	i.PausedAt = &dbTime
	i.UpdatedAt = dbTime

	return nil
}

// Resume marks the paused instance as active again.
// Calculates and accumulates the pause duration.
//
// CRITICAL: dbTime parameter must be database time (from GetDBTime), not app time.
// This ensures consistency across multiple app servers and prevents clock skew issues.
func (i *PromotionInstance) Resume(dbTime time.Time) error {
	if i.Status != InstanceStatusPaused {
		return &InstanceTransitionError{From: i.Status, To: InstanceStatusActive}
	}

	// Calculate pause duration and add to total
	if i.PausedAt != nil {
		pauseDuration := int(dbTime.Sub(*i.PausedAt).Seconds())
		i.TotalPausedDuration += pauseDuration
	}

	i.Status = InstanceStatusActive
	i.PausedAt = nil
	i.UpdatedAt = dbTime

	return nil
}

// Stop marks the instance as cancelled with a reason.
//
// CRITICAL: dbTime parameter must be database time (from GetDBTime), not app time.
// This ensures consistency across multiple app servers and prevents clock skew issues.
func (i *PromotionInstance) Stop(reason StopReason, dbTime time.Time) error {
	if i.Status.IsTerminal() {
		return &InstanceTransitionError{From: i.Status, To: InstanceStatusCancelled}
	}

	i.Status = InstanceStatusCancelled
	i.StoppedAt = &dbTime
	i.StopReason = strPtr(string(reason))
	i.UpdatedAt = dbTime

	return nil
}

// SnapshotConsumedDuration calculates the consumed duration and prepares it for finalization.
// This should be called when stopping an instance to bake the consumed duration into the ownership.
// Returns the consumed duration in seconds.
//
// CRITICAL: This method is idempotent - if already finalized, it returns the existing value
// without modifying any state. This prevents double-counting and data corruption.
//
// CRITICAL: dbTime parameter must be database time (from transaction context), not app time.
// This ensures consistency across multiple app servers and prevents clock skew issues.
func (i *PromotionInstance) SnapshotConsumedDuration(dbTime time.Time) int {
	// IDEMPOTENCY GUARD: If already finalized, return existing value
	// This prevents double-counting if called multiple times
	if i.Finalized {
		return i.FinalizedSeconds
	}

	// Calculate consumed duration up to this point using DB time
	duration := i.GetConsumedDurationSecondsAt(dbTime)

	i.Finalized = true
	i.FinalizedAt = &dbTime
	i.FinalizedSeconds = duration
	i.UpdatedAt = dbTime

	return duration
}

// IsActive returns true if the instance is currently promoting.
func (i *PromotionInstance) IsActive() bool {
	return i.Status.IsActive()
}

// GetConsumedDurationSeconds returns the duration consumed by this instance in seconds.
// For active instances: time from activation to now (minus paused time)
// For stopped/paused instances: time from activation to stop/pause (minus paused time)
// For inactive instances: 0 (not yet consuming time)
// For finalized instances: 0 (duration already baked into ownership)
//
// CRITICAL: dbTime parameter must be database time (from GetDBTime), not app time.
// This ensures consistency across multiple app servers and prevents clock skew issues.
func (i *PromotionInstance) GetConsumedDurationSeconds(dbTime time.Time) int {
	return i.GetConsumedDurationSecondsAt(dbTime)
}

// GetConsumedDurationSecondsAt returns the duration consumed by this instance in seconds
// as of the given time. This method should be used when database time is available
// to ensure consistency across multiple app servers.
func (i *PromotionInstance) GetConsumedDurationSecondsAt(at time.Time) int {
	// Finalized instances have already been accounted for in ownership
	// Return 0 to avoid double counting
	if i.Finalized {
		return 0
	}

	// Inactive instances don't consume time
	if i.Status == InstanceStatusInactive {
		return 0
	}

	// Terminal states: calculate up to stopped_at
	if i.Status.IsTerminal() {
		if i.ActivatedAt == nil || i.StoppedAt == nil {
			return 0
		}
		duration := int(i.StoppedAt.Sub(*i.ActivatedAt).Seconds())
		return max(0, duration-i.TotalPausedDuration)
	}

	// Paused state: calculate up to paused_at
	if i.Status == InstanceStatusPaused {
		if i.ActivatedAt == nil || i.PausedAt == nil {
			return 0
		}
		duration := int(i.PausedAt.Sub(*i.ActivatedAt).Seconds())
		return max(0, duration-i.TotalPausedDuration)
	}

	// Active state: calculate up to the given time
	if i.Status == InstanceStatusActive {
		if i.ActivatedAt == nil {
			return 0
		}
		duration := int(at.Sub(*i.ActivatedAt).Seconds())
		// Subtract any accumulated pause duration
		return max(0, duration-i.TotalPausedDuration)
	}

	return 0
}

// GetStopReason returns the stop reason as a StopReason enum.
// Returns empty string if not stopped or reason is invalid.
func (i *PromotionInstance) GetStopReason() StopReason {
	if i.StopReason == nil {
		return ""
	}
	reason := StopReason(*i.StopReason)
	if reason.IsValid() {
		return reason
	}
	return ""
}

// strPtr returns a pointer to a string.
func strPtr(s string) *string {
	return &s
}
