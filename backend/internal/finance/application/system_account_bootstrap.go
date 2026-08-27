package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance"
	"github.com/labuda/backend/pkg/database"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

// SystemAccountBootstrap ensures required system accounts exist.
// It is idempotent - safe to run multiple times.
type SystemAccountBootstrap struct {
	db *database.DB
}

// NewSystemAccountBootstrap creates a new SystemAccountBootstrap.
func NewSystemAccountBootstrap(db *database.DB) *SystemAccountBootstrap {
	return &SystemAccountBootstrap{
		db: db,
	}
}

// NewSystemAccountBootstrapFromPgx adapts a raw pgx-backed DB into the
// bootstrap wrapper. Useful for standalone verification binaries that do
// not construct the full database facade.
func NewSystemAccountBootstrapFromPgx(pgxDB *db.DB) *SystemAccountBootstrap {
	return &SystemAccountBootstrap{
		db: database.NewFromPgx(pgxDB),
	}
}

// systemAccountConfig holds configuration for a system account.
//
// initialBalance is the opening balance written ONLY when the row is first
// created. Subsequent bootstrap runs leave the balance untouched, so the
// seed is effectively a one-shot accounting opening entry.
type systemAccountConfig struct {
	accountType    string
	name           string
	initialBalance int64
}

// bankSettlementReserveFloat is the opening balance seeded into
// BANK_SETTLEMENT on first bootstrap.
//
// ACCOUNTING ABSTRACTION (TASK 39E, Option 1 — bank-settlement float model):
// BANK_SETTLEMENT represents the platform's external (bank-side) holding of
// gateway-settled funds. Real money lives at the payment gateway; this
// account is the internal ledger mirror so that the canonical double-entry
// can be honored at settlement:
//
//	DR GATEWAY_CLEARING  +gross
//	CR BANK_SETTLEMENT   -gross
//
// At release the GATEWAY_CLEARING balance drains into SELLER_PAYABLE and
// PLATFORM_REVENUE. BANK_SETTLEMENT is therefore expected to trend toward
// zero (or below it, conceptually) as withdrawals flow out to seller bank
// accounts. To keep the financial_accounts.balance >= 0 CHECK invariant
// satisfied across that lifecycle, we seed BANK_SETTLEMENT with a large
// reserve float on first creation.
//
// Magnitude rationale: balance is stored as a Rupiah integer (PASS_18H
// canonical unit, no cents/sen). The seed value below is Rp 9 quadrillion.
// This is several orders of magnitude larger than any plausible aggregate
// settlement volume during pre-launch and well within the int64 ceiling
// (~9.22 quintillion). Adjust only via a deliberate accounting
// reconciliation, not silently.
const bankSettlementReserveFloat int64 = 9_000_000_000_000_000

// platformBankReserveFloat is the opening balance seeded into PLATFORM_BANK
// on first bootstrap.
//
// ACCOUNTING ABSTRACTION (COIN FUNDING CONTRACT):
// PLATFORM_BANK represents the platform's own external bank holdings — the
// source of real platform money used to fund buyer benefits. Under the locked
// coin-funding contract, coin redemption K is a PLATFORM-FUNDED BUYER BENEFIT:
// the platform funds K into GATEWAY_CLEARING (DR PLATFORM_BANK -K / CR
// GATEWAY_CLEARING +K) so the seller's economic entitlement (BuyerBase = PD+S)
// is fully cash-backed without overdrawing the clearing account.
//
// The seed provides the opening balance against which platform-funded benefits
// are drawn, mirroring the BANK_SETTLEMENT reserve-float pattern: it keeps the
// financial_accounts.balance >= 0 CHECK invariant satisfied across the funding
// lifecycle. The same magnitude rationale as bankSettlementReserveFloat applies
// (Rupiah integer, well within int64, several orders of magnitude above any
// plausible aggregate coin-redemption volume during pre-launch).
const platformBankReserveFloat int64 = 9_000_000_000_000_000

// systemAccountConfigs maps account types to their readable names.
var systemAccountConfigs = map[string]systemAccountConfig{
	finance.AccountGatewayClearing: {
		accountType: finance.AccountGatewayClearing,
		name:        "Payment Gateway Clearing Account",
	},
	finance.AccountEscrow: {
		accountType: finance.AccountEscrow,
		name:        "Buyer-Seller Escrow Account",
	},
	finance.AccountSellerPayable: {
		accountType: finance.AccountSellerPayable,
		name:        "Seller Payable Account",
	},
	finance.AccountPlatformRevenue: {
		accountType: finance.AccountPlatformRevenue,
		name:        "Platform Revenue Account",
	},
	finance.AccountWithdrawalPending: {
		accountType: finance.AccountWithdrawalPending,
		name:        "Withdrawal Pending Account",
	},
	finance.AccountWithdrawalCommitted: {
		accountType: finance.AccountWithdrawalCommitted,
		name:        "Withdrawal Committed Account",
	},
	finance.AccountPlatformBank: {
		accountType:    finance.AccountPlatformBank,
		name:           "Platform Bank Account",
		initialBalance: platformBankReserveFloat,
	},
	finance.AccountBankSettlement: {
		accountType:    finance.AccountBankSettlement,
		name:           "Bank Settlement Reserve Account",
		initialBalance: bankSettlementReserveFloat,
	},
}

// EnsureSystemAccounts ensures all required system accounts exist.
// Returns the count of accounts created during this call.
// This is idempotent - safe to call multiple times.
func (b *SystemAccountBootstrap) EnsureSystemAccounts(ctx context.Context) (int, error) {
	var createdCount int

	err := b.db.WithTx(ctx, func(tx db.Tx) error {
		for _, config := range systemAccountConfigs {
			created, err := b.ensureAccount(ctx, tx, config)
			if err != nil {
				return err
			}
			if created {
				createdCount++
			}
		}
		return nil
	})

	if err != nil {
		return 0, fmt.Errorf("failed to ensure system accounts: %w", err)
	}

	return createdCount, nil
}

// ensureAccount ensures a single system account exists.
// Returns true if the account was created, false if it already existed.
func (b *SystemAccountBootstrap) ensureAccount(
	ctx context.Context,
	tx db.Tx,
	config systemAccountConfig,
) (bool, error) {
	// Check if account already exists
	var existingID uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT id FROM financial_accounts
		WHERE account_type = $1 AND user_id IS NULL
	`, config.accountType).Scan(&existingID)

	if err == nil {
		// Account already exists, skip
		return false, nil
	}

	// Account doesn't exist, create it.
	// PR-C ON CONFLICT REPAIR: The system-account uniqueness invariant is
	// enforced by a PARTIAL unique index:
	//   CREATE UNIQUE INDEX uniq_financial_accounts_system_account_type
	//     ON financial_accounts (account_type) WHERE user_id IS NULL
	// PostgreSQL cannot infer a partial unique index from a plain conflict
	// target like (account_type, user_id) — the conflict_target columns must
	// be exactly the indexed columns AND the index_predicate must match.
	// Using the wrong target raises:
	//   "there is no unique or exclusion constraint matching the
	//    ON CONFLICT specification"
	// The clause below mirrors the partial index exactly.
	accountID := uuid.New()
	openingBalance := money.New(config.initialBalance)

	_, err = tx.Exec(ctx, `
		INSERT INTO financial_accounts (id, user_id, account_type, balance, currency, name, is_active, created_at, updated_at)
		VALUES ($1, NULL, $2, $3, $4, $5, $6, NOW(), NOW())
		ON CONFLICT (account_type) WHERE user_id IS NULL DO NOTHING
	`, accountID, config.accountType, openingBalance.Int64(), "IDR", config.name, true)

	if err != nil {
		return false, fmt.Errorf("failed to create system account %s: %w", config.accountType, err)
	}

	// Check if account was actually inserted or if it existed (concurrent create)
	// Re-query to verify
	err = tx.QueryRow(ctx, `
		SELECT id FROM financial_accounts
		WHERE account_type = $1 AND user_id IS NULL
	`, config.accountType).Scan(&existingID)

	if err != nil {
		return false, fmt.Errorf("failed to verify system account creation %s: %w", config.accountType, err)
	}

	// If the ID matches our generated ID, we created it
	wasCreated := existingID == accountID
	return wasCreated, nil
}

// GetSystemAccountID retrieves a system account ID by type.
// Returns error if account not found.
func (b *SystemAccountBootstrap) GetSystemAccountID(
	ctx context.Context,
	accountType string,
) (uuid.UUID, error) {
	var accountID uuid.UUID

	err := b.db.WithTx(ctx, func(tx db.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT id FROM financial_accounts
			WHERE account_type = $1 AND user_id IS NULL
		`, accountType).Scan(&accountID)
		return err
	})

	if err != nil {
		if err.Error() == "no rows in result set" {
			return uuid.Nil, fmt.Errorf("system account not found: type=%s", accountType)
		}
		return uuid.Nil, fmt.Errorf("failed to get system account: %w", err)
	}

	return accountID, nil
}


