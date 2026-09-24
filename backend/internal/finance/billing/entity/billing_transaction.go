package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/money"
)

// BillingTransaction represents a non-order payment transaction.
// Canonical usage: Promote Balance top-up (TypePromoteBalanceTopUp).
type BillingTransaction struct {
	ID                   uuid.UUID
	PayerID              uuid.UUID
	TargetID             uuid.UUID
	Type                 Type
	GrossAmount          money.Money
	PlatformFeePercent   int64 // e.g., 5 for 5%
	PlatformFeeAmount    money.Money
	NetAmount            money.Money
	Status               Status
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// Type represents the billing transaction type.
type Type string

const (
	// TypePromoteBalanceTopUp is a verified seller top-up into PROMOTE_BALANCE.
	// Canonical funding: the settled payment credits the seller's Promote
	// Balance via FinanceService.RecordPromoteBalanceFunding. It is NOT a
	// service purchase — PLATFORM_REVENUE must never be credited by a top-up.
	TypePromoteBalanceTopUp Type = "promote_balance_top_up"
)

// Status represents the billing transaction status.
type Status string

const (
	StatusPending Status = "pending"
	StatusPaid    Status = "paid"
	StatusFailed  Status = "failed"
)

// InvalidTransitionError is returned when attempting an invalid state transition.
type InvalidTransitionError struct {
	CurrentStatus Status
	TargetStatus  Status
}

func (e *InvalidTransitionError) Error() string {
	return fmt.Sprintf("invalid billing status transition: %s -> %s", e.CurrentStatus, e.TargetStatus)
}

// IsValidType returns true if the type is valid.
func IsValidType(t Type) bool {
	switch t {
	case TypePromoteBalanceTopUp:
		return true
	default:
		return false
	}
}


// MarkPaid transitions the transaction from pending to paid.
func (bt *BillingTransaction) MarkPaid() error {
	if !canTransition(bt.Status, StatusPaid) {
		return &InvalidTransitionError{
			CurrentStatus: bt.Status,
			TargetStatus:  StatusPaid,
		}
	}

	bt.Status = StatusPaid
	bt.UpdatedAt = time.Now()
	return nil
}

// MarkFailed transitions the transaction from pending to failed.
func (bt *BillingTransaction) MarkFailed() error {
	if !canTransition(bt.Status, StatusFailed) {
		return &InvalidTransitionError{
			CurrentStatus: bt.Status,
			TargetStatus:  StatusFailed,
		}
	}

	bt.Status = StatusFailed
	bt.UpdatedAt = time.Now()
	return nil
}

// canTransition checks if a status transition is valid.
func canTransition(from, to Status) bool {
	switch from {
	case StatusPending:
		return to == StatusPaid || to == StatusFailed
	case StatusPaid, StatusFailed:
		return false // Terminal states
	default:
		return false
	}
}

// NewBillingTransaction creates a new pending billing transaction.
// Platform fee is calculated as: (grossAmount * platformFeePercent) / 100
// NetAmount = GrossAmount - PlatformFeeAmount
func NewBillingTransaction(
	payerID uuid.UUID,
	targetID uuid.UUID,
	billingType Type,
	grossAmount money.Money,
	platformFeePercent int64,
) (*BillingTransaction, error) {
	if !IsValidType(billingType) {
		return nil, fmt.Errorf("invalid billing type: %s", billingType)
	}

	now := time.Now()

	// Calculate platform fee amount: (grossAmount * percent) / 100
	platformFeeAmount := money.New((grossAmount.Int64() * platformFeePercent) / 100)

	// Net amount = gross - platform fee
	netAmount := grossAmount.Sub(platformFeeAmount)

	return &BillingTransaction{
		ID:                 uuid.New(),
		PayerID:            payerID,
		TargetID:           targetID,
		Type:               billingType,
		GrossAmount:        grossAmount,
		PlatformFeePercent: platformFeePercent,
		PlatformFeeAmount:  platformFeeAmount,
		NetAmount:          netAmount,
		Status:             StatusPending,
		CreatedAt:          now,
		UpdatedAt:          now,
	}, nil
}


