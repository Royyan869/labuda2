package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/money"
)

// FinancialAccountDB represents the financial_accounts table.
// GORM tags removed - migration now handled via SQL
type FinancialAccountDB struct {
	ID          uuid.UUID       `json:"id"`
	UserID      *uuid.UUID      `json:"user_id,omitempty"`
	AccountType string          `json:"account_type"`
	Balance     money.Money `json:"balance"`
	Currency    string          `json:"currency"`
	Name        string          `json:"name"`
	IsActive    bool            `json:"is_active"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// TableName specifies the table name for FinancialAccountDB.
func (FinancialAccountDB) TableName() string {
	return "financial_accounts"
}


