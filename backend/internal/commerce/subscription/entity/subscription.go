package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hishumi/backend/pkg/money"
)

// Status represents the subscription state machine.
// Live transitions are enforced by UpdateStatusTx (active -> expired);
// renewal inserts a new immutable active period.
type Status string

const (
	StatusActive  Status = "active"
	StatusExpired Status = "expired"
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

// IsActive returns true if the subscription status is active.
func (s *SellerSubscription) IsActive() bool {
	return s.Status == StatusActive
}

// IsExpired returns true if the subscription status is expired.
func (s *SellerSubscription) IsExpired() bool {
	return s.Status == StatusExpired
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
