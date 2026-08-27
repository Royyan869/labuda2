package models

import (
	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/money"
)

// LedgerTransactionDB represents the ledger_transactions table.
// WARNING: Internal to finance domain. Use LedgerService for all ledger operations.
// Direct writes to this table bypass double-entry accounting validation.
// GORM tags removed - migration now handled via SQL
type LedgerTransactionDB struct {
	ID             uuid.UUID  `json:"id"`
	IdempotencyKey string     `json:"idempotency_key"`
	ReferenceType  string     `json:"reference_type"`
	ReferenceID    uuid.UUID  `json:"reference_id"`
	TotalDebit     money.Money `json:"total_debit"`
	TotalCredit    money.Money `json:"total_credit"`
	CreatedAt      int64       `json:"created_at"`
}

// TableName specifies the table name for LedgerTransactionDB.
func (LedgerTransactionDB) TableName() string {
	return "ledger_transactions"
}

// LedgerEntryDB represents the ledger_entries table.
// WARNING: Internal to finance domain. Use LedgerService for all ledger operations.
// Direct writes to this table bypass double-entry accounting validation.
// GORM tags removed - migration now handled via SQL
type LedgerEntryDB struct {
	ID            uuid.UUID  `json:"id"`
	TransactionID uuid.UUID  `json:"transaction_id"`
	AccountID     uuid.UUID  `json:"account_id"`
	EntryType     string     `json:"entry_type"` // "debit" or "credit"
	Amount        money.Money `json:"amount"`
	BalanceAfter  money.Money `json:"balance_after"`
	CreatedAt     int64       `json:"created_at"`
}

// TableName specifies the table name for LedgerEntryDB.
func (LedgerEntryDB) TableName() string {
	return "ledger_entries"
}


