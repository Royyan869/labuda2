package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// LedgerEntryType represents the type of ledger entry.
type LedgerEntryType string

const (
	LedgerEntryTypeCredit LedgerEntryType = "credit" // Increases wallet balance
	LedgerEntryTypeDebit  LedgerEntryType = "debit"  // Decreases wallet balance
)

// String returns the string representation.
func (t LedgerEntryType) String() string {
	return string(t)
}

// IsValid checks if the entry type is valid.
func (t LedgerEntryType) IsValid() bool {
	switch t {
	case LedgerEntryTypeCredit, LedgerEntryTypeDebit:
		return true
	default:
		return false
	}
}

// LedgerReferenceType represents the source/reason for the ledger entry.
type LedgerReferenceType string

const (
	LedgerReferenceOrder            LedgerReferenceType = "order"              // Order-related transaction
	LedgerReferencePayment          LedgerReferenceType = "payment"            // Payment-related transaction
	LedgerReferenceRefund           LedgerReferenceType = "refund"             // Refund-related transaction
	LedgerReferenceWithdrawal       LedgerReferenceType = "withdrawal"         // Seller withdrawal (deprecated, use specific types)
	LedgerReferenceDeposit          LedgerReferenceType = "deposit"            // User deposit
	LedgerReferenceEscrowHold       LedgerReferenceType = "escrow_hold"        // Escrow hold for order
	LedgerReferenceEscrowRelease    LedgerReferenceType = "escrow_release"     // Escrow release to seller
	LedgerReferenceEscrowRefund     LedgerReferenceType = "escrow_refund"      // Escrow refund to buyer
	// Withdrawal-specific reference types (Phase 3)
	LedgerReferenceWithdrawalRequest  LedgerReferenceType = "withdrawal_request"  // Initial deduction on withdrawal request
	LedgerReferenceWithdrawalRefund   LedgerReferenceType = "withdrawal_refund"   // Refund on withdrawal rejection
	LedgerReferenceWithdrawalComplete LedgerReferenceType = "withdrawal_complete" // Completion record (optional)
	// Payout-specific reference types (Minimal manual payout system)
	LedgerReferencePayoutRequested  LedgerReferenceType = "payout_requested"  // Payout requested (available -> pending_withdrawal)
	LedgerReferencePayoutCompleted  LedgerReferenceType = "payout_completed"  // Payout completed (pending_withdrawal deducted)
	LedgerReferencePayoutFailed     LedgerReferenceType = "payout_failed"     // Payout failed (pending_withdrawal -> available)
)

// String returns the string representation.
func (t LedgerReferenceType) String() string {
	return string(t)
}

// IsValid checks if the reference type is valid.
func (t LedgerReferenceType) IsValid() bool {
	switch t {
	case LedgerReferenceOrder, LedgerReferencePayment, LedgerReferenceRefund,
		LedgerReferenceWithdrawal, LedgerReferenceDeposit,
		LedgerReferenceEscrowHold, LedgerReferenceEscrowRelease, LedgerReferenceEscrowRefund,
		LedgerReferenceWithdrawalRequest, LedgerReferenceWithdrawalRefund, LedgerReferenceWithdrawalComplete,
		LedgerReferencePayoutRequested, LedgerReferencePayoutCompleted, LedgerReferencePayoutFailed:
		return true
	default:
		return false
	}
}

// LedgerEntry represents an immutable record of a wallet balance change.
//
// IMPORTANT: Ledger entries are NEVER updated or deleted.
// Every balance change MUST create a corresponding ledger entry.
//
// This provides a complete audit trail of all money movements.
type LedgerEntry struct {
	ID            uuid.UUID
	WalletID      uuid.UUID
	EntryType     LedgerEntryType
	Amount        int64             // In cents (e.g., 1000 = $10.00)
	ReferenceType LedgerReferenceType
	ReferenceID   *uuid.UUID        // ID of related entity (order_id, payment_id, etc.)
	Description   *string           // Optional human-readable description
	CreatedAt     time.Time
}

// IsValid checks if the ledger entry is valid.
func (e *LedgerEntry) IsValid() error {
	if !e.EntryType.IsValid() {
		return fmt.Errorf("invalid entry type: %s", e.EntryType)
	}
	if !e.ReferenceType.IsValid() {
		return fmt.Errorf("invalid reference type: %s", e.ReferenceType)
	}
	if e.Amount < 0 {
		return fmt.Errorf("amount cannot be negative: got %d", e.Amount)
	}
	return nil
}

// NewLedgerEntry creates a new ledger entry.
func NewLedgerEntry(
	walletID uuid.UUID,
	entryType LedgerEntryType,
	amount int64,
	referenceType LedgerReferenceType,
	referenceID *uuid.UUID,
	description *string,
) (*LedgerEntry, error) {
	if amount < 0 {
		return nil, fmt.Errorf("amount cannot be negative: got %d", amount)
	}
	if !entryType.IsValid() {
		return nil, fmt.Errorf("invalid entry type: %s", entryType)
	}
	if !referenceType.IsValid() {
		return nil, fmt.Errorf("invalid reference type: %s", referenceType)
	}

	return &LedgerEntry{
		ID:            uuid.New(),
		WalletID:      walletID,
		EntryType:     entryType,
		Amount:        amount,
		ReferenceType: referenceType,
		ReferenceID:   referenceID,
		Description:   description,
		CreatedAt:     time.Now(),
	}, nil
}

// NewCreditEntry creates a new credit ledger entry (balance increase).
func NewCreditEntry(
	walletID uuid.UUID,
	amount int64,
	referenceType LedgerReferenceType,
	referenceID *uuid.UUID,
	description *string,
) (*LedgerEntry, error) {
	return NewLedgerEntry(walletID, LedgerEntryTypeCredit, amount, referenceType, referenceID, description)
}

// NewDebitEntry creates a new debit ledger entry (balance decrease).
func NewDebitEntry(
	walletID uuid.UUID,
	amount int64,
	referenceType LedgerReferenceType,
	referenceID *uuid.UUID,
	description *string,
) (*LedgerEntry, error) {
	return NewLedgerEntry(walletID, LedgerEntryTypeDebit, amount, referenceType, referenceID, description)
}


