// ============================================================================
// COIN RESERVATION ENTITY — MODEL R RESERVATION AUTHORITY
// ============================================================================
//
// Canonical reservation lifecycle:
//   RESERVE  →  CONSUME  (payment settled)
//   RESERVE  →  RELEASE  (payment failed/expired/cancelled)
//
// INVARIANTS:
//   TotalUnspentCoins  = user_coin_balance.balance
//   ReservedCoins      = SUM(amount) WHERE status = 'reserved'
//   AvailableCoins     = TotalUnspentCoins - ReservedCoins
//
// Reserve:  does NOT decrement balance, does NOT create spend transaction.
// Consume:  will decrement balance exactly once, create exactly one spend tx.
// Release:  does NOT credit balance, does NOT create earn/refund tx.
//
// One payment. One reservation. One immutable K.
// ============================================================================

package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CoinReservationStatus represents the state of a coin reservation.
type CoinReservationStatus string

const (
	CoinReservationStatusReserved  CoinReservationStatus = "reserved"
	CoinReservationStatusConsumed  CoinReservationStatus = "consumed"
	CoinReservationStatusReleased  CoinReservationStatus = "released"
)

// String returns the string representation.
func (s CoinReservationStatus) String() string {
	return string(s)
}

// IsValid checks if the status is valid.
func (s CoinReservationStatus) IsValid() bool {
	switch s {
	case CoinReservationStatusReserved, CoinReservationStatusConsumed, CoinReservationStatusReleased:
		return true
	default:
		return false
	}
}

// IsTerminal returns true if the reservation is in a terminal state.
func (s CoinReservationStatus) IsTerminal() bool {
	return s == CoinReservationStatusConsumed || s == CoinReservationStatusReleased
}

// CoinReservation represents a coin hold for an active payment intent.
//
// This is the SINGLE SOURCE OF TRUTH for coin availability during payment.
// Coins remain in user_coin_balance throughout the reservation lifecycle;
// the reservation row makes them unavailable for other payments without
// mutating the total balance.
type CoinReservation struct {
	ID         uuid.UUID
	PaymentID  uuid.UUID
	UserID     uuid.UUID
	Amount     int64                    // Coin count (1 coin = Rp1)
	Status     CoinReservationStatus
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	ReleasedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// NewCoinReservation creates a new reservation in 'reserved' state.
func NewCoinReservation(paymentID, userID uuid.UUID, amount int64, expiresAt time.Time) (*CoinReservation, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("reservation amount must be positive: got %d", amount)
	}
	if expiresAt.IsZero() {
		return nil, fmt.Errorf("reservation expires_at is required")
	}

	now := time.Now()
	return &CoinReservation{
		ID:        uuid.New(),
		PaymentID: paymentID,
		UserID:    userID,
		Amount:    amount,
		Status:    CoinReservationStatusReserved,
		ExpiresAt: expiresAt,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// Consume transitions a reservation from reserved to consumed.
// Returns error if reservation is not in 'reserved' state.
func (r *CoinReservation) Consume() error {
	if r.Status != CoinReservationStatusReserved {
		return fmt.Errorf("cannot consume reservation in status %s (must be reserved)", r.Status)
	}
	now := time.Now()
	r.Status = CoinReservationStatusConsumed
	r.ConsumedAt = &now
	r.UpdatedAt = now
	return nil
}

// Release transitions a reservation from reserved to released.
// Returns error if reservation is not in 'reserved' state.
func (r *CoinReservation) Release() error {
	if r.Status != CoinReservationStatusReserved {
		return fmt.Errorf("cannot release reservation in status %s (must be reserved)", r.Status)
	}
	now := time.Now()
	r.Status = CoinReservationStatusReleased
	r.ReleasedAt = &now
	r.UpdatedAt = now
	return nil
}

// IsActive returns true if the reservation is currently holding coins.
func (r *CoinReservation) IsActive() bool {
	return r.Status == CoinReservationStatusReserved
}

// ErrReservationConflict is returned when a conflicting reservation exists.
type ErrReservationConflict struct {
	PaymentID          uuid.UUID
	ExistingAmount     int64
	RequestedAmount    int64
}

func (e *ErrReservationConflict) Error() string {
	return fmt.Sprintf("conflicting reservation for payment %s: existing amount=%d, requested=%d",
		e.PaymentID, e.ExistingAmount, e.RequestedAmount)
}

// ErrReservationAlreadyConsumed is returned when attempting to release
// a reservation that has already been consumed (opposite-terminal error).
type ErrReservationAlreadyConsumed struct {
	PaymentID uuid.UUID
}

func (e *ErrReservationAlreadyConsumed) Error() string {
	return fmt.Sprintf("reservation for payment %s is already consumed; cannot release", e.PaymentID)
}

// ErrReservationAlreadyReleased is returned when attempting to consume
// a reservation that has already been released (opposite-terminal error).
type ErrReservationAlreadyReleased struct {
	PaymentID uuid.UUID
}

func (e *ErrReservationAlreadyReleased) Error() string {
	return fmt.Sprintf("reservation for payment %s is already released; cannot consume", e.PaymentID)
}

// ErrReservationInsufficientBalance is returned when available coins < requested.
type ErrReservationInsufficientBalance struct {
	UserID           uuid.UUID
	RequestedAmount  int64
	AvailableBalance int64
	TotalBalance     int64
	ReservedBalance  int64
}

func (e *ErrReservationInsufficientBalance) Error() string {
	return fmt.Sprintf("insufficient available coins for user %s: requested=%d, available=%d (total=%d, reserved=%d)",
		e.UserID, e.RequestedAmount, e.AvailableBalance, e.TotalBalance, e.ReservedBalance)
}
