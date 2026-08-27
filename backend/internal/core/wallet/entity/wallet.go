package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Wallet represents a user's wallet - single source of truth for money.
//
// IMPORTANT: This wallet stores REAL MONEY, not coins (loyalty points).
// Coins are managed separately in user_coin_balance table.
//
// BALANCE STRUCTURE:
// - AvailableBalance: money that can be spent now
// - HeldBalance: money held in escrow for pending orders
// - PendingWithdrawal: money being processed for manual payout
// - TotalBalance = AvailableBalance + HeldBalance + PendingWithdrawal
type Wallet struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	AvailableBalance int64  // In cents (e.g., 1000 = $10.00)
	HeldBalance      int64  // In cents (e.g., 500 = $5.00)
	PendingWithdrawal int64  // In cents - for manual payout processing
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// TotalBalance returns the sum of available, held, and pending withdrawal balances.
func (w *Wallet) TotalBalance() int64 {
	return w.AvailableBalance + w.HeldBalance + w.PendingWithdrawal
}

// CanAfford checks if the wallet has sufficient available balance.
func (w *Wallet) CanAfford(amount int64) bool {
	return w.AvailableBalance >= amount
}

// HasSufficientAvailable checks if the wallet has sufficient available balance for an amount.
// Returns error if insufficient.
func (w *Wallet) HasSufficientAvailable(amount int64) error {
	if amount < 0 {
		return fmt.Errorf("amount cannot be negative: %d", amount)
	}
	if w.AvailableBalance < amount {
		return &ErrInsufficientBalance{
			WalletID:         w.ID,
			UserID:           w.UserID,
			RequestedAmount:  amount,
			AvailableBalance: w.AvailableBalance,
		}
	}
	return nil
}

// NewWallet creates a new wallet for a user with zero balance.
func NewWallet(userID uuid.UUID) *Wallet {
	now := time.Now()
	return &Wallet{
		ID:               uuid.New(),
		UserID:           userID,
		AvailableBalance: 0,
		HeldBalance:      0,
		PendingWithdrawal: 0,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// ErrInsufficientBalance is returned when a wallet has insufficient available balance.
type ErrInsufficientBalance struct {
	WalletID         uuid.UUID
	UserID           uuid.UUID
	RequestedAmount  int64
	AvailableBalance int64
}

func (e *ErrInsufficientBalance) Error() string {
	return fmt.Sprintf("insufficient wallet balance for user %s: requested=%d, available=%d",
		e.UserID, e.RequestedAmount, e.AvailableBalance)
}

// ErrWalletNotFound is returned when a wallet is not found.
type ErrWalletNotFound struct {
	UserID uuid.UUID
}

func (e *ErrWalletNotFound) Error() string {
	return fmt.Errorf("wallet not found for user: %s", e.UserID).Error()
}


