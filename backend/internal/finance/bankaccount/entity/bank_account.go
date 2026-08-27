package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Status represents the bank account status.
// Values MUST match database bank_account_status_enum exactly.
type Status string

const (
	// StatusActive indicates the bank account is active and can be used
	StatusActive Status = "active"
	// StatusDeleted indicates the bank account has been soft deleted
	StatusDeleted Status = "deleted"
)

// BankAccount represents a user's bank account for withdrawals.
type BankAccount struct {
	ID                uuid.UUID
	UserID            uuid.UUID
	BankName          string
	BankCode          string // Canonical bank code for payout rail integration (e.g., "BCA", "BNI")
	AccountNumber     string
	AccountHolderName string
	IsDefault         bool
	Status            Status
	DeletedAt         *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// InvalidTransitionError is returned when attempting an invalid state transition.
type InvalidTransitionError struct {
	CurrentStatus Status
	TargetStatus  Status
}

func (e *InvalidTransitionError) Error() string {
	return fmt.Sprintf("invalid bank account status transition: %s -> %s", e.CurrentStatus, e.TargetStatus)
}

// IsValidStatus returns true if the status is valid.
func IsValidStatus(s Status) bool {
	switch s {
	case StatusActive, StatusDeleted:
		return true
	default:
		return false
	}
}

// IsActive returns true if the bank account is active (not deleted).
func (ba *BankAccount) IsActive() bool {
	return ba.Status == StatusActive && ba.DeletedAt == nil
}

// SoftDelete marks the bank account as deleted.
// This is a soft delete - the record remains but is filtered from queries.
func (ba *BankAccount) SoftDelete() error {
	if ba.Status == StatusDeleted {
		return &InvalidTransitionError{
			CurrentStatus: ba.Status,
			TargetStatus:  StatusDeleted,
		}
	}

	now := time.Now()
	ba.Status = StatusDeleted
	ba.DeletedAt = &now
	ba.IsDefault = false // Cannot be default when deleted
	ba.UpdatedAt = now
	return nil
}

// SetDefault marks this account as the default.
// Note: Repository must ensure only one default per user.
func (ba *BankAccount) SetDefault() {
	ba.IsDefault = true
	ba.UpdatedAt = time.Now()
}

// UnsetDefault removes default status from this account.
func (ba *BankAccount) UnsetDefault() {
	ba.IsDefault = false
	ba.UpdatedAt = time.Now()
}

// NewBankAccount creates a new bank account.
// By default, new accounts are NOT set as default.
// The caller must explicitly call SetDefault() if this should be the default.
func NewBankAccount(
	userID uuid.UUID,
	bankName string,
	bankCode string,
	accountNumber string,
	accountHolderName string,
) (*BankAccount, error) {
	if userID == uuid.Nil {
		return nil, fmt.Errorf("bank account: user_id cannot be empty")
	}
	if bankName == "" {
		return nil, fmt.Errorf("bank account: bank_name cannot be empty")
	}
	if bankCode == "" {
		return nil, fmt.Errorf("bank account: bank_code cannot be empty")
	}
	if accountNumber == "" {
		return nil, fmt.Errorf("bank account: account_number cannot be empty")
	}
	if accountHolderName == "" {
		return nil, fmt.Errorf("bank account: account_holder_name cannot be empty")
	}

	now := time.Now()
	return &BankAccount{
		ID:                uuid.New(),
		UserID:            userID,
		BankName:          bankName,
		BankCode:          bankCode,
		AccountNumber:     accountNumber,
		AccountHolderName: accountHolderName,
		IsDefault:         false,
		Status:            StatusActive,
		DeletedAt:         nil,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}


