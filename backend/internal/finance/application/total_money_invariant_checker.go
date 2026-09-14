// ⚠️ RECONCILIATION LAYER:
// This module detects total money invariant violations using finance ledger authority.
// It does NOT modify business data - detection and alerting only.
//
// CANONICAL INVARIANT (account-class-aware):
//
//	For every account: stored_balance == computed_economic_balance
//
// Every ledger transaction is balanced (Σ entries = 0) and the engine applies
// account-class-aware balance formulas (Asset/Expense: +entry.Amount,
// Liability/Revenue: -entry.Amount). Any deviation indicates a bug in ledger
// bookkeeping or a direct DB mutation.
//
// WIRED IN STARTUP (disabled by default):
//
//	This checker is wrapped by TotalMoneyInvariantWorker and conditionally started
//	in dependencies.go behind the workerEnabled("TOTAL_MONEY_INVARIANT_WORKER", false) gate.
//	To activate: set DISABLE_TOTAL_MONEY_INVARIANT_WORKER=false.
//	To enable alerts: also set TOTAL_MONEY_INVARIANT_SHADOW_MODE=false.
package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	alertapp "github.com/labuda/backend/internal/platform/alert/application"
	alertentity "github.com/labuda/backend/internal/platform/alert/entity"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

const (
	// BankSettlementInitialSeed is the balance bootstrapped into BANK_SETTLEMENT
	// without a corresponding ledger entry. Under canonical sign architecture
	// (Option A), the global SUM(balance) is no longer a meaningful invariant
	// because liability and asset balances accumulate in the same direction.
	// The per-account reconciliation invariant (stored == computed) is the
	// canonical correctness check.
	BankSettlementInitialSeed int64 = 9_000_000_000_000_000 // retained for bootstrap; not used in invariant check
)

// TotalMoneyInvariantChecker verifies the total money invariant using
// finance ledger authority only.
//
// INVARIANT: For every account, stored_balance == computed_economic_balance.
//
// Under canonical sign architecture (Option A), the global SUM(balance) is not
// invariant because liability and asset balances both increase with balanced
// cross-class transactions. The correct invariant is per-account reconciliation:
// each account's stored balance must equal its computed balance from ledger entries.
//
// This checker verifies the invariant by reconstructing balances from ledger entries
// and comparing against stored balances.
//
// FINANCIAL SAFETY LAYER:
//   - Detects ledger bookkeeping bugs or unauthorized DB mutations
//   - Single read-only snapshot query on financial_accounts
//   - NO wallet, payment, order, or refund table queries
//   - NO AUTO-FIX — detection and alerting only
type TotalMoneyInvariantChecker struct {
	alertService *alertapp.AlertService
	db           db.Transactor
	log          *zap.Logger
	shadowMode   bool
}

// NewTotalMoneyInvariantChecker creates a new total money invariant checker.
//
// When shadowMode is true, the checker logs findings but does NOT create alerts.
func NewTotalMoneyInvariantChecker(
	alertService *alertapp.AlertService,
	database db.Transactor,
	log *zap.Logger,
	shadowMode bool,
) *TotalMoneyInvariantChecker {
	if log == nil {
		log = zap.NewNop()
	}

	return &TotalMoneyInvariantChecker{
		alertService: alertService,
		db:           database,
		log:          log,
		shadowMode:   shadowMode,
	}
}

// CheckTotalMoneyInvariant validates per-account balance integrity.
// Under canonical sign architecture (Option A), each account's stored balance
// must equal its computed economic balance from ledger entries.
// Returns true if a violation is detected, false otherwise.
func (c *TotalMoneyInvariantChecker) CheckTotalMoneyInvariant(ctx context.Context) (bool, error) {
	c.log.Debug("Starting total money invariant check",
		zap.Bool("shadow_mode", c.shadowMode),
	)

	mismatches, err := c.getBalanceMismatches(ctx)
	if err != nil {
		c.log.Error("Failed to check account balances", zap.Error(err))
		return false, fmt.Errorf("failed to check account balances: %w", err)
	}

	if len(mismatches) > 0 {
		c.log.Error("Total money invariant violation - CRITICAL",
			zap.Int("mismatched_accounts", len(mismatches)),
			zap.Bool("shadow_mode", c.shadowMode),
		)

		if !c.shadowMode {
			c.createViolationAlert(ctx, int64(len(mismatches)), 0, int64(len(mismatches)))
		}

		return true, nil
	}

	c.log.Info("Total money invariant check passed")
	return false, nil
}

// getBalanceMismatches returns accounts where stored balance != computed balance.
// Under canonical sign architecture, the computed balance is account-class-aware.
func (c *TotalMoneyInvariantChecker) getBalanceMismatches(ctx context.Context) ([]string, error) {
	var mismatches []string

	err := c.db.WithTx(ctx, func(tx db.Tx) error {
		query := `
			SELECT fa.account_type,
			       fa.balance as stored,
			       COALESCE(SUM(CASE
			         WHEN fa.account_type IN ('SELLER_PAYABLE','BUYER_REFUNDABLE','PLATFORM_REVENUE',
			           'WITHDRAWAL_PENDING','WITHDRAWAL_COMMITTED','GATEWAY_CLEARING','ESCROW',
			           'USER_SERVICE_CREDIT','AD_REVENUE','PROMOTE_BALANCE','PROMOTION_ALLOCATION',
			           'BANK_SETTLEMENT')
			         THEN CASE WHEN le.entry_type = 'credit' THEN le.amount ELSE -le.amount END
			         ELSE CASE WHEN le.entry_type = 'debit' THEN le.amount ELSE -le.amount END
			       END), 0) as computed
			FROM financial_accounts fa
			LEFT JOIN ledger_entries le ON le.account_id = fa.id
			GROUP BY fa.id, fa.account_type, fa.balance
			HAVING fa.balance != COALESCE(SUM(CASE
			  WHEN fa.account_type IN ('SELLER_PAYABLE','BUYER_REFUNDABLE','PLATFORM_REVENUE',
			    'WITHDRAWAL_PENDING','WITHDRAWAL_COMMITTED','GATEWAY_CLEARING','ESCROW',
			    'USER_SERVICE_CREDIT','AD_REVENUE','PROMOTE_BALANCE','PROMOTION_ALLOCATION',
			    'BANK_SETTLEMENT')
			  THEN CASE WHEN le.entry_type = 'credit' THEN le.amount ELSE -le.amount END
			  ELSE CASE WHEN le.entry_type = 'debit' THEN le.amount ELSE -le.amount END
			END), 0)
		`
		rows, err := tx.Query(ctx, query)
		if err != nil {
			return fmt.Errorf("mismatch query failed: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var accountType string
			var stored, computed int64
			if err := rows.Scan(&accountType, &stored, &computed); err != nil {
				continue
			}
			mismatches = append(mismatches, fmt.Sprintf("%s: stored=%d computed=%d drift=%d", accountType, stored, computed, stored-computed))
		}
		return rows.Err()
	})

	if err != nil {
		return nil, err
	}
	return mismatches, nil
}

// createViolationAlert creates a CRITICAL alert for total money invariant violation.
func (c *TotalMoneyInvariantChecker) createViolationAlert(
	ctx context.Context,
	actualTotal, expectedTotal, difference int64,
) {
	metadata := alertentity.AlertMetadata{
		"actual_total":    actualTotal,
		"expected_total":  expectedTotal,
		"difference":      difference,
		"required_action": "emergency_forensic_accounting_audit",
		"reason":          "total_money_invariant_violation",
		"systemic_issue":  "true",
	}

	message := fmt.Sprintf(
		"CRITICAL: Total money invariant violated. SUM(financial_accounts.balance) = %d, expected %d. Difference: %d.",
		actualTotal,
		expectedTotal,
		difference,
	)

	groupKey := "total-money-invariant-violation"
	_, err := c.alertService.CreateAlert(
		ctx,
		alertentity.AlertTypeReconciliationDrift,
		alertentity.SeverityCritical,
		"system",
		uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		message,
		metadata,
		&groupKey,
	)

	if err != nil {
		c.log.Error("Failed to create total money violation alert",
			zap.Int64("actual_total", actualTotal),
			zap.Int64("expected_total", expectedTotal),
			zap.Int64("difference", difference),
			zap.Error(err),
		)
	}
}
