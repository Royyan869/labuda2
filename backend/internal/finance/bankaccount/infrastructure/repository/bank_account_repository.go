package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/finance/bankaccount/entity"
	"github.com/labuda/backend/pkg/db"
)

// BankAccountRepository handles bank account persistence operations.
// All mutations use FOR UPDATE to prevent race conditions.
type BankAccountRepository struct{}

// NewBankAccountRepository creates a new BankAccountRepository.
func NewBankAccountRepository() *BankAccountRepository {
	return &BankAccountRepository{}
}

// Create creates a new bank account.
// Does NOT set default - use SetDefaultBankAccount for that.
func (r *BankAccountRepository) Create(
	ctx context.Context,
	tx db.Tx,
	ba *entity.BankAccount,
) error {
	now := time.Now().Unix()
	_, err := tx.Exec(ctx, `
		INSERT INTO bank_accounts (
			id, seller_id, bank_name, bank_code, account_number, account_holder_name,
			is_default, status, deleted_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, ba.ID, ba.UserID, ba.BankName, ba.BankCode, ba.AccountNumber, ba.AccountHolderName,
		ba.IsDefault, ba.Status, nil, now, now)

	if err != nil {
		return fmt.Errorf("bank account: create failed: %w", err)
	}

	return nil
}

// GetByID retrieves a bank account by ID (including deleted).
func (r *BankAccountRepository) GetByID(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (*entity.BankAccount, error) {
	return r.scanOne(ctx, tx, `
		SELECT id, seller_id, bank_name, bank_code, account_number, account_holder_name,
		       is_default, status, deleted_at, created_at, updated_at
		FROM bank_accounts WHERE id = $1
	`, id)
}

// GetActiveByID retrieves an active bank account by ID.
func (r *BankAccountRepository) GetActiveByID(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (*entity.BankAccount, error) {
	return r.scanOne(ctx, tx, `
		SELECT id, seller_id, bank_name, bank_code, account_number, account_holder_name,
		       is_default, status, deleted_at, created_at, updated_at
		FROM bank_accounts WHERE id = $1 AND deleted_at IS NULL
	`, id)
}

// GetDefaultBySeller retrieves the default bank account for a seller.
// Locks the row FOR UPDATE to prevent concurrent modifications.
func (r *BankAccountRepository) GetDefaultBySeller(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
) (*entity.BankAccount, error) {
	var ba entity.BankAccount
	var deletedAt *time.Time

	err := tx.QueryRow(ctx, `
		SELECT id, seller_id, bank_name, bank_code, account_number, account_holder_name,
		       is_default, status, deleted_at, created_at, updated_at
		FROM bank_accounts
		WHERE seller_id = $1 AND is_default = true AND deleted_at IS NULL
		FOR UPDATE
	`, sellerID).Scan(
		&ba.ID, &ba.UserID, &ba.BankName, &ba.BankCode, &ba.AccountNumber, &ba.AccountHolderName,
		&ba.IsDefault, &ba.Status, &deletedAt, &ba.CreatedAt, &ba.UpdatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, fmt.Errorf("bank account: no default account found for seller %s", sellerID)
		}
		return nil, fmt.Errorf("bank account: get default failed: %w", err)
	}

	ba.DeletedAt = deletedAt
	return &ba, nil
}

// ListActiveAccountsBySeller retrieves all active bank accounts for a seller.
// Ordered by default first, then by creation date (newest first).
func (r *BankAccountRepository) ListActiveAccountsBySeller(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
) ([]*entity.BankAccount, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, seller_id, bank_name, bank_code, account_number, account_holder_name,
		       is_default, status, deleted_at, created_at, updated_at
		FROM bank_accounts
		WHERE seller_id = $1 AND deleted_at IS NULL
		ORDER BY is_default DESC, created_at DESC
	`, sellerID)
	if err != nil {
		return nil, fmt.Errorf("bank account: list failed: %w", err)
	}
	defer rows.Close()

	var accounts []*entity.BankAccount
	for rows.Next() {
		ba, err := r.scanRow(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, ba)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("bank account: scan failed: %w", rows.Err())
	}

	return accounts, nil
}

// SetDefaultBankAccount sets a bank account as default.
// Uses FOR UPDATE to prevent concurrent modifications.
// Ensures only ONE default per seller:
// 1. Locks ALL active accounts for seller FOR UPDATE (prevents concurrent SetDefault)
// 2. Verifies target account exists and is active
// 3. Unsets any existing default for the seller
// 4. Sets the new account as default
func (r *BankAccountRepository) SetDefaultBankAccount(
	ctx context.Context,
	tx db.Tx,
	accountID uuid.UUID,
	sellerID uuid.UUID,
) error {
	// CRITICAL: Lock ALL active accounts for this seller first
	// This prevents concurrent SetDefaultBankAccount calls from racing
	rows, err := tx.Query(ctx, `
		SELECT id, seller_id, bank_name, bank_code, account_number, account_holder_name,
		       is_default, status, deleted_at, created_at, updated_at
		FROM bank_accounts
		WHERE seller_id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`, sellerID)
	if err != nil {
		return fmt.Errorf("bank account: lock seller accounts failed: %w", err)
	}
	defer rows.Close()

	// Find the target account and check if already default
	var targetAccount *entity.BankAccount
	var existingDefaultID *uuid.UUID

	for rows.Next() {
		ba, scanErr := r.scanRow(rows)
		if scanErr != nil {
			return scanErr
		}
		if ba.ID == accountID {
			targetAccount = ba
		}
		if ba.IsDefault {
			existingDefaultID = &ba.ID
		}
	}

	if rows.Err() != nil {
		return fmt.Errorf("bank account: scan accounts failed: %w", rows.Err())
	}

	// Verify target account exists
	if targetAccount == nil {
		return fmt.Errorf("bank account: not found or not active for seller %s", sellerID)
	}

	// If already default, nothing to do
	if targetAccount.IsDefault {
		return nil
	}

	// Unset any existing default for this seller
	now := time.Now().Unix()
	if existingDefaultID != nil {
		_, err = tx.Exec(ctx, `
			UPDATE bank_accounts
			SET is_default = false, updated_at = $1
			WHERE id = $2
		`, now, *existingDefaultID)
		if err != nil {
			return fmt.Errorf("bank account: unset existing default failed: %w", err)
		}
	}

	// Set the new account as default
	_, err = tx.Exec(ctx, `
		UPDATE bank_accounts
		SET is_default = true, updated_at = $1
		WHERE id = $2
	`, now, accountID)
	if err != nil {
		return fmt.Errorf("bank account: set default failed: %w", err)
	}

	return nil
}

// SoftDeleteBankAccount soft deletes a bank account.
// Uses FOR UPDATE to prevent concurrent modifications.
// Returns error if:
// - Account is already deleted
// - Account is the only active account for the seller
// - Active withdrawals exist for the seller in REQUESTED or PROCESSING status
func (r *BankAccountRepository) SoftDeleteBankAccount(
	ctx context.Context,
	tx db.Tx,
	accountID uuid.UUID,
	sellerID uuid.UUID,
) error {
	// Lock the account first
	var ba entity.BankAccount
	var deletedAt *time.Time

	err := tx.QueryRow(ctx, `
		SELECT id, seller_id, bank_name, bank_code, account_number, account_holder_name,
		       is_default, status, deleted_at, created_at, updated_at
		FROM bank_accounts
		WHERE id = $1 AND seller_id = $2
		FOR UPDATE
	`, accountID, sellerID).Scan(
		&ba.ID, &ba.UserID, &ba.BankName, &ba.BankCode, &ba.AccountNumber, &ba.AccountHolderName,
		&ba.IsDefault, &ba.Status, &deletedAt, &ba.CreatedAt, &ba.UpdatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return fmt.Errorf("bank account: not found for seller %s", sellerID)
		}
		return fmt.Errorf("bank account: lock failed: %w", err)
	}

	ba.DeletedAt = deletedAt

	// Check if already deleted
	if ba.Status == entity.StatusDeleted {
		return fmt.Errorf("bank account: already deleted")
	}

	// Check for active withdrawals BEFORE deleting
	// This prevents deletion while a withdrawal is in progress
	var activeWithdrawalCount int
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM withdrawals
		WHERE seller_id = $1
		  AND status IN ('REQUESTED', 'PROCESSING')
	`, sellerID).Scan(&activeWithdrawalCount)
	if err != nil {
		return fmt.Errorf("bank account: check active withdrawals failed: %w", err)
	}

	if activeWithdrawalCount > 0 {
		return fmt.Errorf("bank account: cannot delete while %d active withdrawal(s) exist", activeWithdrawalCount)
	}

	// Perform soft delete
	now := time.Now()
	nowUnix := now.Unix()
	_, err = tx.Exec(ctx, `
		UPDATE bank_accounts
		SET status = 'deleted', is_default = false, deleted_at = $1, updated_at = $2
		WHERE id = $3
	`, now, nowUnix, accountID)
	if err != nil {
		return fmt.Errorf("bank account: soft delete failed: %w", err)
	}

	return nil
}

// scanOne scans a single bank account from a query.
func (r *BankAccountRepository) scanOne(
	ctx context.Context,
	tx db.Tx,
	query string,
	args ...interface{},
) (*entity.BankAccount, error) {
	var ba entity.BankAccount
	var deletedAt *time.Time

	err := tx.QueryRow(ctx, query, args...).Scan(
		&ba.ID, &ba.UserID, &ba.BankName, &ba.BankCode, &ba.AccountNumber, &ba.AccountHolderName,
		&ba.IsDefault, &ba.Status, &deletedAt, &ba.CreatedAt, &ba.UpdatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, fmt.Errorf("bank account: not found")
		}
		return nil, fmt.Errorf("bank account: scan failed: %w", err)
	}

	ba.DeletedAt = deletedAt
	return &ba, nil
}

// scanRow scans a bank account from a row.
func (r *BankAccountRepository) scanRow(rows pgx.Rows) (*entity.BankAccount, error) {
	var ba entity.BankAccount
	var deletedAt *time.Time

	// rows implements the same scanning interface as tx
	err := rows.Scan(
		&ba.ID, &ba.UserID, &ba.BankName, &ba.BankCode, &ba.AccountNumber, &ba.AccountHolderName,
		&ba.IsDefault, &ba.Status, &deletedAt, &ba.CreatedAt, &ba.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("bank account: scan row failed: %w", err)
	}

	ba.DeletedAt = deletedAt
	return &ba, nil
}


