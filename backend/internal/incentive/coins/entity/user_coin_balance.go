package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// UserCoinBalance represents the aggregate coin balance for a user.
//
// This is the SINGLE SOURCE OF TRUTH for concurrent spend operations.
// The coins_transactions table remains as the audit trail, but all
// balance checks and modifications go through this entity first.
//
// RACE CONDITION PREVENTION:
// - Single row per user
// - Atomic UPDATE with WHERE clause
// - No SELECT FOR UPDATE needed (deadlock-free)
type UserCoinBalance struct {
	UserID    uuid.UUID
	Balance   int64     // Current balance (never negative)
	Version   int64     // Optimistic locking for reconciliation
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewUserCoinBalance creates a new balance entity for a user.
func NewUserCoinBalance(userID uuid.UUID, initialBalance int64) (*UserCoinBalance, error) {
	if initialBalance < 0 {
		return nil, fmt.Errorf("initial balance cannot be negative: got %d", initialBalance)
	}

	return &UserCoinBalance{
		UserID:    userID,
		Balance:   initialBalance,
		Version:   1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

// CanSpend checks if the user has sufficient balance for a spend operation.
// This is a LOCAL check - the actual atomic check happens in the database.
func (b *UserCoinBalance) CanSpend(amount int64) bool {
	return b.Balance >= amount
}

// CalculateNewBalance returns the balance after a spend, without modifying it.
// Returns error if spend would result in negative balance.
func (b *UserCoinBalance) CalculateNewBalance(spendAmount int64) (int64, error) {
	newBalance := b.Balance - spendAmount
	if newBalance < 0 {
		return 0, fmt.Errorf("spend would result in negative balance: current=%d, spend=%d", b.Balance, spendAmount)
	}
	return newBalance, nil
}

// CalculateNewBalanceWithEarn returns the balance after earning coins.
func (b *UserCoinBalance) CalculateNewBalanceWithEarn(earnAmount int64) int64 {
	return b.Balance + earnAmount
}

// ReconcileDifference checks if this balance matches the expected derived balance.
// Returns the difference (positive = aggregate is higher, negative = aggregate is lower).
func (b *UserCoinBalance) ReconcileDifference(derivedBalance int64) int64 {
	return b.Balance - derivedBalance
}


