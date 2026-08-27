package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/money"
)

// Status represents the subscription state machine.
// Transitions: inactive -> active -> expired.
// Renewal from expired creates a new active period.
type Status string

const (
	StatusInactive Status = "inactive"
	StatusActive   Status = "active"
	StatusExpired  Status = "expired"
)

// SellerSubscription represents a seller's subscription record.
// This entity uses an immutable record model:
// - Renewals insert new rows instead of updating existing records
// - Non-refundable, revenue-backed
// - No escrow involvement
//
// Invariants:
// - One user can have only one active subscription (enforced by DB partial unique index)
// - expires_at > started_at
// - Revenue is recognized immediately upon payment (no escrow)
type SellerSubscription struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	Status       Status
	StartedAt    time.Time
	ExpiresAt    time.Time
	DurationDays int
	AmountPaid   money.Money
	Currency     string
	PaymentID    uuid.UUID
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Domain Errors

// ErrInvalidStatusTransition is returned when attempting an invalid status transition.
type ErrInvalidStatusTransition struct {
	From Status
	To   Status
}

func (e *ErrInvalidStatusTransition) Error() string {
	return fmt.Sprintf("invalid status transition: %s -> %s", e.From, e.To)
}

// ErrDuplicateActiveSubscription is returned when attempting to create a new active
// subscription for a user who already has an active subscription.
type ErrDuplicateActiveSubscription struct {
	UserID uuid.UUID
}

func (e *ErrDuplicateActiveSubscription) Error() string {
	return fmt.Sprintf("user %s already has an active subscription", e.UserID)
}

// ErrTransitionGuardFailed is returned when status transition fails because
// the current status doesn't match the expected fromStatus.
type ErrTransitionGuardFailed struct {
	ID           uuid.UUID
	ExpectedFrom Status
	ActualFrom   Status
	To           Status
}

func (e *ErrTransitionGuardFailed) Error() string {
	return fmt.Sprintf("transition guard failed for subscription %s: expected status %s, got %s (attempting: %s -> %s)",
		e.ID, e.ExpectedFrom, e.ActualFrom, e.ExpectedFrom, e.To)
}

// ValidateTransition checks if the status transition is valid.
//
// Valid transitions:
// - inactive -> active
// - active -> expired
// - expired -> active (renewal only, creates new record)
//
// Returns nil for valid transitions, ErrInvalidStatusTransition otherwise.
func (s *SellerSubscription) ValidateTransition(to Status) error {
	// Define valid transitions
	validTransitions := map[Status][]Status{
		StatusInactive: {StatusActive},
		StatusActive:   {StatusExpired},
		StatusExpired:  {}, // Terminal until renewal (new record)
	}

	allowed, exists := validTransitions[s.Status]
	if !exists {
		return &ErrInvalidStatusTransition{From: s.Status, To: to}
	}

	for _, allowedStatus := range allowed {
		if allowedStatus == to {
			return nil
		}
	}

	return &ErrInvalidStatusTransition{From: s.Status, To: to}
}

// IsActive returns true if the subscription status is active.
func (s *SellerSubscription) IsActive() bool {
	return s.Status == StatusActive
}

// IsExpired returns true if the subscription status is expired.
func (s *SellerSubscription) IsExpired() bool {
	return s.Status == StatusExpired
}

// IsInactive returns true if the subscription status is inactive.
func (s *SellerSubscription) IsInactive() bool {
	return s.Status == StatusInactive
}

// HasMarketAuthority returns true if the user has active seller market authority.
// Authority is derived from subscription status, not a stored role flag.
func (s *SellerSubscription) HasMarketAuthority() bool {
	return s.IsActive()
}

// IsExpiredByTime returns true if the subscription has passed expiration.
func (s *SellerSubscription) IsExpiredByTime(now time.Time) bool {
	return now.After(s.ExpiresAt)
}

// SellerSubscriptionConfig represents the canonical seller subscription config.
// Changes affect new payments only - values are snapshotted into subscription records.
type SellerSubscriptionConfig struct {
	ID                  uuid.UUID
	YearlyFeeRupiah     int64
	DurationDays        int
	RenewalReminderDays int
	Enabled             bool
	CreatedAt           time.Time
}
