package repository

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance"
	ledgerrepo "github.com/labuda/backend/internal/finance/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

// LedgerRepository handles double-entry ledger operations using pgx-based DB layer.
// It enforces idempotency via unique constraint and double-entry accounting.
type LedgerRepository struct {
	// No DB field needed - repository uses db.Tx passed as parameter
	// This follows the spec: domain only knows db.Tx interface
}

// NewLedgerRepository creates a new LedgerRepository.
func NewLedgerRepository() *LedgerRepository {
	return &LedgerRepository{}
}

// CreateTransaction creates a new ledger transaction with its entries.
//
// Idempotency: If a transaction with the same idempotency key exists,
// returns nil (success without duplicate execution).
//
// Double-entry invariant: Σ(entries) must equal zero.
// Panics if the transaction is unbalanced.
//
// This method MUST be called within a transaction for atomicity:
//
//	db.WithTx(ctx, func(tx db.Tx) error {
//	    return repo.CreateTransaction(ctx, tx, idempotencyKey, referenceType, referenceID, orderID, paymentID, entries)
//	})
func (r *LedgerRepository) CreateTransaction(
	ctx context.Context,
	tx db.Tx,
	idempotencyKey string,
	referenceType string,
	referenceID uuid.UUID,
	orderID *uuid.UUID,
	paymentID *uuid.UUID,
	entries []ledgerrepo.Entry,
) error {
	// Validate entries
	if len(entries) == 0 {
		return fmt.Errorf("ledger: at least one entry required")
	}

	// Step 1: Validate balanced transaction (Σ(debit) = Σ(credit))
	// In our model: positive = debit, negative = credit
	// So Σ(all amounts) must equal zero
	total := money.Zero()
	var totalDebit, totalCredit int64
	for _, entry := range entries {
		total = total.Add(entry.Amount)
		if entry.Amount.Int64() > 0 {
			totalDebit += entry.Amount.Int64()
		} else {
			totalCredit += -entry.Amount.Int64()
		}
	}
	if !total.IsZero() {
		panic(fmt.Sprintf("ledger: unbalanced transaction, total=%d", total.Int64()))
	}

	// Collect unique account IDs
	accountIDs := make([]uuid.UUID, 0, len(entries))
	seen := make(map[uuid.UUID]bool)
	for _, entry := range entries {
		if !seen[entry.AccountID] {
			accountIDs = append(accountIDs, entry.AccountID)
			seen[entry.AccountID] = true
		}
	}

	// CRITICAL: Sort account IDs BEFORE locking to prevent deadlock
	// When concurrent transactions lock accounts in the same order,
	// deadlock cannot occur. This is essential for high concurrency.
	sort.Slice(accountIDs, func(i, j int) bool {
		return accountIDs[i].String() < accountIDs[j].String()
	})

	// Step 2: Idempotency check
	// If transaction with this key exists, return nil (success)
	var existingID uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT id FROM ledger_transactions
		WHERE idempotency_key = $1
	`, idempotencyKey).Scan(&existingID)

	if err == nil {
		// Transaction already exists - idempotent success
		return nil
	}

	// If error is not "no rows", return it
	// pgx returns "no rows" as pgx.ErrNoRows
	if err.Error() != "no rows in result set" {
		return fmt.Errorf("ledger: idempotency check failed: %w", err)
	}

	// Step 3: Create ledger transaction record
	transactionID := uuid.New()
	now := time.Now().Unix()

	_, err = tx.Exec(ctx, `
		INSERT INTO ledger_transactions (id, idempotency_key, reference_type, reference_id, order_id, payment_id, total_debit, total_credit, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, transactionID, idempotencyKey, referenceType, referenceID, orderID, paymentID,
		totalDebit, totalCredit, now)

	if err != nil {
		// Check for unique constraint violation (23505)
		if IsUniqueViolation(err) {
			// Concurrent insert with same idempotency key - treat as success
			return nil
		}
		return fmt.Errorf("ledger: insert transaction failed: %w", err)
	}

	// Step 4: Lock accounts with FOR UPDATE
	// This prevents concurrent modifications to the same accounts
	rows, err := tx.Query(ctx, `
		SELECT id, balance FROM financial_accounts
		WHERE id = ANY($1)
		FOR UPDATE
	`, accountIDs)

	if err != nil {
		return fmt.Errorf("ledger: lock accounts failed: %w", err)
	}
	defer rows.Close()

	// Load account balances
	accountBalances := make(map[uuid.UUID]int64)
	for rows.Next() {
		var id uuid.UUID
		var balance int64
		if err := rows.Scan(&id, &balance); err != nil {
			return fmt.Errorf("ledger: scan account failed: %w", err)
		}
		accountBalances[id] = balance
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("ledger: iterate accounts failed: %w", err)
	}

	// Verify all accounts exist
	if len(accountBalances) != len(accountIDs) {
		return fmt.Errorf("ledger: some accounts not found: expected %d, found %d",
			len(accountIDs), len(accountBalances))
	}

	// Step 5: Update account balances and create ledger entries
	for _, entry := range entries {
		newBalance := accountBalances[entry.AccountID] + entry.Amount.Int64()

		// Update account balance
		_, err = tx.Exec(ctx, `
			UPDATE financial_accounts
			SET balance = $1, updated_at = NOW()
			WHERE id = $2
		`, newBalance, entry.AccountID)

		if err != nil {
			return fmt.Errorf("ledger: update balance failed for account %s: %w",
				entry.AccountID, err)
		}

		// Insert ledger entry
		entryID := uuid.New()
		entryType := "debit"
		amount := entry.Amount.Int64()
		if amount < 0 {
			entryType = "credit"
			amount = -amount // Store as positive for entries
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO ledger_entries (id, transaction_id, account_id, entry_type, amount, balance_after, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, entryID, transactionID, entry.AccountID, entryType, amount, newBalance, now)

		if err != nil {
			return fmt.Errorf("ledger: insert entry failed: %w", err)
		}

		// Update local balance for subsequent entries to same account
		accountBalances[entry.AccountID] = newBalance
	}

	return nil
}

// GetAccountBalance retrieves the current balance of an account.
func (r *LedgerRepository) GetAccountBalance(
	ctx context.Context,
	tx db.Tx,
	accountID uuid.UUID,
) (money.Money, error) {
	var balance int64
	err := tx.QueryRow(ctx, `
		SELECT balance FROM financial_accounts WHERE id = $1
	`, accountID).Scan(&balance)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return money.Zero(), fmt.Errorf("ledger: account not found: %s", accountID)
		}
		return money.Zero(), fmt.Errorf("ledger: get balance failed: %w", err)
	}

	return money.New(balance), nil
}

// GetAccountBalanceForUpdate retrieves the current balance of an account with FOR UPDATE lock.
// This MUST be used before balance checks to prevent race conditions.
// The lock is held until the transaction commits or rolls back.
func (r *LedgerRepository) GetAccountBalanceForUpdate(
	ctx context.Context,
	tx db.Tx,
	accountID uuid.UUID,
) (money.Money, error) {
	var balance int64
	err := tx.QueryRow(ctx, `
		SELECT balance FROM financial_accounts WHERE id = $1 FOR UPDATE
	`, accountID).Scan(&balance)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return money.Zero(), fmt.Errorf("ledger: account not found: %s", accountID)
		}
		return money.Zero(), fmt.Errorf("ledger: get balance for update failed: %w", err)
	}

	return money.New(balance), nil
}

// GetSystemAccountID retrieves a system account by type.
func (r *LedgerRepository) GetSystemAccountID(
	ctx context.Context,
	tx db.Tx,
	accountType string,
) (uuid.UUID, error) {
	var accountID uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT id FROM financial_accounts
		WHERE account_type = $1 AND user_id IS NULL
	`, accountType).Scan(&accountID)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return uuid.Nil, fmt.Errorf("ledger: system account not found: type=%s", accountType)
		}
		return uuid.Nil, fmt.Errorf("ledger: get system account failed: %w", err)
	}

	return accountID, nil
}

// GetUserAccountID retrieves a user account by type and owner ID.
func (r *LedgerRepository) GetUserAccountID(
	ctx context.Context,
	tx db.Tx,
	accountType string,
	userID uuid.UUID,
) (uuid.UUID, error) {
	var accountID uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT id FROM financial_accounts
		WHERE account_type = $1 AND user_id = $2
	`, accountType, userID).Scan(&accountID)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return uuid.Nil, fmt.Errorf("ledger: user account not found: type=%s, user_id=%s",
				accountType, userID)
		}
		return uuid.Nil, fmt.Errorf("ledger: get user account failed: %w", err)
	}

	return accountID, nil
}

// GetOrCreateUserAccount retrieves or creates a user account.
func (r *LedgerRepository) GetOrCreateUserAccount(
	ctx context.Context,
	tx db.Tx,
	accountType string,
	userID uuid.UUID,
) (uuid.UUID, error) {
	// Try to get existing account first
	accountID, err := r.GetUserAccountID(ctx, tx, accountType, userID)
	if err == nil {
		return accountID, nil
	}

	// If error is "not found", create new account
	errStr := err.Error()
	if contains(errStr, "not found") {
		newID := uuid.New()
		_, err = tx.Exec(ctx, `
			INSERT INTO financial_accounts (id, user_id, account_type, balance, currency, name, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		`, newID, userID, accountType, 0, "IDR", accountType+" Account", true)

		if err != nil {
			if IsUniqueViolation(err) {
				// Concurrent insert - retry get
				return r.GetUserAccountID(ctx, tx, accountType, userID)
			}
			return uuid.Nil, fmt.Errorf("ledger: create account failed: %w", err)
		}
		return newID, nil
	}

	return uuid.Nil, err
}

// IsUniqueViolation checks if error is PostgreSQL unique constraint violation (23505).
func IsUniqueViolation(err error) bool {
	if err == nil || err.Error() == "no rows in result set" {
		return false
	}
	// Try to extract error code from error message
	// This is a simple check - in production, use proper error wrapping
	errStr := err.Error()
	return contains(errStr, "23505") ||
		contains(errStr, "duplicate key") ||
		contains(errStr, "UNIQUE constraint")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// GetTotalCreditToUserAccount returns the total amount of credits to a user's account.
// This is used for calculating total earnings for sellers.
func (r *LedgerRepository) GetTotalCreditToUserAccount(
	ctx context.Context,
	tx db.Tx,
	accountType string,
	userID uuid.UUID,
) (int64, error) {
	// Get the user account ID first
	accountID, err := r.GetUserAccountID(ctx, tx, accountType, userID)
	if err != nil {
		return 0, fmt.Errorf("get user account: %w", err)
	}

	// Sum all credit (positive) entries for this account
	var total int64
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(
			CASE WHEN entry_type = 'debit' THEN amount ELSE 0 END
		), 0)
		FROM ledger_entries
		WHERE account_id = $1
	`, accountID).Scan(&total)

	if err != nil {
		return 0, fmt.Errorf("sum credit entries: %w", err)
	}

	return total, nil
}

// Constants for system account types - exported for convenience
const (
	AccountGatewayClearing = finance.AccountGatewayClearing
	AccountEscrow          = finance.AccountEscrow
	AccountSellerPayable   = finance.AccountSellerPayable
	AccountBuyerRefundable = finance.AccountBuyerRefundable
	AccountPlatformRevenue = finance.AccountPlatformRevenue
	AccountBankSettlement  = finance.AccountBankSettlement
)

// CountTransactionsByEntityID returns the number of ledger transactions for a given entity ID.
func (r *LedgerRepository) CountTransactionsByEntityID(
	ctx context.Context,
	tx db.Tx,
	entityID uuid.UUID,
) (int, error) {
	var count int
	err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM ledger_transactions
		WHERE reference_id = $1
	`, entityID).Scan(&count)

	if err != nil {
		return 0, fmt.Errorf("count transactions by entity id: %w", err)
	}

	return count, nil
}



