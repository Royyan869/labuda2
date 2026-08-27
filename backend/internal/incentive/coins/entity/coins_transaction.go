// DOMAIN: INCENTIVE
// NOTE: Loyalty points system for purchase rewards

package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CoinTransactionType represents the type of coin transaction.
type CoinTransactionType string

const (
	CoinTransactionTypeEarn  CoinTransactionType = "earn"  // Points earned (rewards, purchases)
	CoinTransactionTypeSpend CoinTransactionType = "spend" // Points spent (order discount)
)

// String returns the string representation.
func (t CoinTransactionType) String() string {
	return string(t)
}

// IsValid checks if the transaction type is valid.
func (t CoinTransactionType) IsValid() bool {
	switch t {
	case CoinTransactionTypeEarn, CoinTransactionTypeSpend:
		return true
	default:
		return false
	}
}

// CoinReferenceType represents the source/reason for the transaction.
type CoinReferenceType string

const (
	CoinReferenceOrderReward CoinReferenceType = "order_reward" // Reward from completing an order
	CoinReferenceOrderSpend  CoinReferenceType = "order_spend"  // Spent on an order
	CoinReferenceRefundEarn  CoinReferenceType = "refund_earn"  // Points refunded from cancelled order
	CoinReferenceRefundSpend CoinReferenceType = "refund_spend" // Points deducted from refund
	// SECURITY: CoinReferencePromoReward and CoinReferenceAdminGrant REMOVED
	// - Promo rewards must use specific promo system, not generic coins
	// - Admin grants are a security backdoor - NOT ALLOWED
	// - Coins can ONLY be earned from order completion or refund events
)

// String returns the string representation.
func (r CoinReferenceType) String() string {
	return string(r)
}

// IsValid checks if the reference type is valid.
func (r CoinReferenceType) IsValid() bool {
	switch r {
	case CoinReferenceOrderReward, CoinReferenceOrderSpend,
		CoinReferenceRefundEarn, CoinReferenceRefundSpend:
		return true
	default:
		return false
	}
}

// CoinsTransaction represents a single loyalty points transaction.
//
// IMPORTANT: This is NOT a financial ledger entry.
// Loyalty points are non-financial rewards only, not money.
// These transactions track point movements for rewards purposes.
type CoinsTransaction struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	Type          CoinTransactionType
	Amount        int64       // Number of points (positive for earn and spend)
	ReferenceType CoinReferenceType
	ReferenceID   *uuid.UUID // ID of related entity (order_id, etc.)
	CreatedAt     time.Time
}

// IsValid checks if the transaction is valid.
func (t *CoinsTransaction) IsValid() error {
	if !t.Type.IsValid() {
		return fmt.Errorf("invalid transaction type: %s", t.Type)
	}
	if !t.ReferenceType.IsValid() {
		return fmt.Errorf("invalid reference type: %s", t.ReferenceType)
	}
	if t.Amount <= 0 {
		return fmt.Errorf("amount must be positive: got %d", t.Amount)
	}
	return nil
}

// NewCoinsTransaction creates a new loyalty points transaction.
func NewCoinsTransaction(
	userID uuid.UUID,
	transactionType CoinTransactionType,
	amount int64,
	referenceType CoinReferenceType,
	referenceID *uuid.UUID,
) (*CoinsTransaction, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be positive: got %d", amount)
	}
	if !transactionType.IsValid() {
		return nil, fmt.Errorf("invalid transaction type: %s", transactionType)
	}
	if !referenceType.IsValid() {
		return nil, fmt.Errorf("invalid reference type: %s", referenceType)
	}

	tx := &CoinsTransaction{
		ID:            uuid.New(),
		UserID:        userID,
		Type:          transactionType,
		Amount:        amount,
		ReferenceType: referenceType,
		ReferenceID:   referenceID,
		CreatedAt:     time.Now(),
	}

	return tx, nil
}

// NewEarnTransaction creates a new earn transaction.
func NewEarnTransaction(
	userID uuid.UUID,
	amount int64,
	referenceType CoinReferenceType,
	referenceID *uuid.UUID,
) (*CoinsTransaction, error) {
	return NewCoinsTransaction(userID, CoinTransactionTypeEarn, amount, referenceType, referenceID)
}

// NewSpendTransaction creates a new spend transaction.
func NewSpendTransaction(
	userID uuid.UUID,
	amount int64,
	referenceID uuid.UUID, // order_id
) (*CoinsTransaction, error) {
	return NewCoinsTransaction(userID, CoinTransactionTypeSpend, amount, CoinReferenceOrderSpend, &referenceID)
}

// CoinsTransactionPage represents a paginated list of transactions.
type CoinsTransactionPage struct {
	Transactions []*CoinsTransaction
	TotalCount   int64
	Page         int
	PageSize     int
	HasMore      bool
}

// ErrInsufficientBalance is returned when user doesn't have enough coins.
// Balance is derived from transactions using GetActiveBalance query.
type ErrInsufficientBalance struct {
	UserID          uuid.UUID
	RequestedAmount int64
	CurrentBalance  int64
}

func (e *ErrInsufficientBalance) Error() string {
	return fmt.Sprintf("insufficient coins balance for user %s: requested=%d, balance=%d",
		e.UserID, e.RequestedAmount, e.CurrentBalance)
}


