package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// EscrowStatus represents the status of an escrow.
type EscrowStatus string

const (
	EscrowStatusHolding  EscrowStatus = "holding"  // Funds held at gateway clearing for pending order
	EscrowStatusReleased EscrowStatus = "released" // Released to seller (order complete)
	EscrowStatusRefunded EscrowStatus = "refunded" // Refunded to buyer (order cancelled)
)

// String returns the string representation.
func (s EscrowStatus) String() string {
	return string(s)
}

// IsValid checks if the status is valid.
func (s EscrowStatus) IsValid() bool {
	switch s {
	case EscrowStatusHolding, EscrowStatusReleased, EscrowStatusRefunded:
		return true
	default:
		return false
	}
}

// IsFinal checks if the escrow is in a final state (cannot be modified).
func (s EscrowStatus) IsFinal() bool {
	return s == EscrowStatusReleased || s == EscrowStatusRefunded
}

// Escrow represents the platform's obligation against a settled gateway payment.
//
// Money physically lives at the payment gateway clearing account; the escrow
// row is the platform-side record of "this order has a settled payment held
// pending release-to-seller or refund-to-buyer". Buyer has no internal
// monetary balance — the seller's withdrawable surface is the finance ledger's
// SELLER_PAYABLE account, and refunds flow through the gateway refund pipeline.
//
// LIFECYCLE:
//  1. HOLDING   — payment settled at gateway, escrow recorded
//  2. RELEASED  — order completed, finance ledger drains GATEWAY_CLEARING into
//                 SELLER_PAYABLE + PLATFORM_REVENUE
//  3. REFUNDED  — order cancelled / disputed, gateway refund pipeline reverses
//                 the settlement
//
// IMPORTANT: Only ONE escrow per order (enforced by UNIQUE constraint).
type Escrow struct {
	ID         uuid.UUID
	OrderID    uuid.UUID
	Amount     int64 // In rupiah units (canonical money unit)
	Status     EscrowStatus
	PaymentID  *uuid.UUID // Nullable link to payments(id) for audit
	CreatedAt  time.Time
	ReleasedAt *time.Time
	RefundedAt *time.Time
}

// IsValid checks if the escrow is valid.
func (e *Escrow) IsValid() error {
	if !e.Status.IsValid() {
		return fmt.Errorf("invalid escrow status: %s", e.Status)
	}
	if e.Amount < 0 {
		return fmt.Errorf("amount cannot be negative: got %d", e.Amount)
	}
	return nil
}

// NewEscrow creates a new escrow for an order.
func NewEscrow(orderID uuid.UUID, amount int64) (*Escrow, error) {
	if orderID == uuid.Nil {
		return nil, fmt.Errorf("order_id cannot be nil")
	}
	if amount < 0 {
		return nil, fmt.Errorf("amount cannot be negative: got %d", amount)
	}

	return &Escrow{
		ID:        uuid.New(),
		OrderID:   orderID,
		Amount:    amount,
		Status:    EscrowStatusHolding,
		CreatedAt: time.Now(),
	}, nil
}

// Release marks the escrow as released to the seller.
func (e *Escrow) Release() error {
	if e.Status.IsFinal() {
		return &ErrEscrowAlreadyFinalized{
			EscrowID: e.ID,
			Status:   e.Status,
		}
	}
	e.Status = EscrowStatusReleased
	now := time.Now()
	e.ReleasedAt = &now
	return nil
}

// Refund marks the escrow as refunded to the buyer.
func (e *Escrow) Refund() error {
	if e.Status.IsFinal() {
		return &ErrEscrowAlreadyFinalized{
			EscrowID: e.ID,
			Status:   e.Status,
		}
	}
	e.Status = EscrowStatusRefunded
	now := time.Now()
	e.RefundedAt = &now
	return nil
}

// ErrEscrowAlreadyFinalized is returned when trying to modify a finalized escrow.
type ErrEscrowAlreadyFinalized struct {
	EscrowID uuid.UUID
	Status   EscrowStatus
}

func (e *ErrEscrowAlreadyFinalized) Error() string {
	return fmt.Sprintf("escrow already finalized: id=%s status=%s", e.EscrowID, e.Status)
}

// ErrEscrowNotFound is returned when an escrow is not found.
type ErrEscrowNotFound struct {
	OrderID uuid.UUID
}

func (e *ErrEscrowNotFound) Error() string {
	return fmt.Errorf("escrow not found for order: %s", e.OrderID).Error()
}

// ErrEscrowAlreadyExists is returned when trying to create a duplicate escrow for an order.
type ErrEscrowAlreadyExists struct {
	OrderID uuid.UUID
}

func (e *ErrEscrowAlreadyExists) Error() string {
	return fmt.Errorf("escrow already exists for order: %s", e.OrderID).Error()
}
