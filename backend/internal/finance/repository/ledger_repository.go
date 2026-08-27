package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

// LedgerRepository handles double-entry ledger operations.
type LedgerRepository interface {
	// CreateTransaction creates a new ledger transaction with its entries.
	//
	// Idempotency: If a transaction with the same idempotency key exists,
	// returns nil (success without duplicate execution).
	//
	// Double-entry invariant: Σ(entries) must equal zero.
	CreateTransaction(
		ctx context.Context,
		tx db.Tx,
		idempotencyKey string,
		referenceType string,
		referenceID uuid.UUID,
		orderID *uuid.UUID,
		paymentID *uuid.UUID,
		entries []Entry,
	) error

	// GetAccountBalance retrieves the current balance of an account.
	GetAccountBalance(ctx context.Context, tx db.Tx, accountID uuid.UUID) (money.Money, error)

	// GetAccountBalanceForUpdate retrieves the current balance of an account with FOR UPDATE lock.
	// This MUST be used before balance checks to prevent race conditions.
	GetAccountBalanceForUpdate(ctx context.Context, tx db.Tx, accountID uuid.UUID) (money.Money, error)

	// GetSystemAccountID retrieves a system account by type.
	GetSystemAccountID(ctx context.Context, tx db.Tx, accountType string) (uuid.UUID, error)

	// GetUserAccountID retrieves a user account by type and owner ID.
	GetUserAccountID(ctx context.Context, tx db.Tx, accountType string, userID uuid.UUID) (uuid.UUID, error)

	// GetOrCreateUserAccount retrieves or creates a user account.
	GetOrCreateUserAccount(ctx context.Context, tx db.Tx, accountType string, userID uuid.UUID) (uuid.UUID, error)

	// CountTransactionsByEntityID returns the number of ledger transactions for a given entity ID.
	CountTransactionsByEntityID(ctx context.Context, tx db.Tx, entityID uuid.UUID) (int, error)

	// GetTotalCreditToUserAccount returns the total amount of credits to a user's account.
	GetTotalCreditToUserAccount(ctx context.Context, tx db.Tx, accountType string, userID uuid.UUID) (int64, error)
}

// Entry represents a single ledger entry.
// Positive amount = debit
// Negative amount = credit
type Entry struct {
	AccountID uuid.UUID
	Amount    money.Money
}

// Constants for system account types — re-exported aliases of the canonical
// values defined in package finance (internal/finance/account_types.go).
//
// SINGLE SOURCE OF TRUTH (TASK 40): financial_accounts.account_type values
// are stored as canonical UPPERCASE enum-style strings ("GATEWAY_CLEARING",
// "SELLER_PAYABLE", …). The bootstrap inserts uppercase; every lookup must
// query with uppercase. Earlier copies of this block held lowercase string
// literals which silently broke any GetSystemAccountID call routed through
// these symbols. Do NOT reintroduce lowercase string literals here.
const (
	AccountGatewayClearing = finance.AccountGatewayClearing
	// AccountEscrow is used by EscrowIntegrityChecker and TotalMoneyInvariantChecker
	// for system-level escrow balance verification.
	AccountEscrow          = finance.AccountEscrow
	AccountSellerPayable   = finance.AccountSellerPayable
	AccountBuyerRefundable = finance.AccountBuyerRefundable
	AccountPlatformRevenue = finance.AccountPlatformRevenue
	AccountBankSettlement  = finance.AccountBankSettlement
)


