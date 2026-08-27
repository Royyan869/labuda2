// ⚠️ RECONCILIATION LAYER:
// This module detects total money invariant violations using finance ledger authority.
// It does NOT modify business data - detection and alerting only.
//
// CANONICAL INVARIANT:
//
//	SUM(financial_accounts.balance) == BankSettlementInitialSeed
//
// Every ledger transaction is balanced (Σ entries = 0), so the sum of all account
// balances can only equal the seed value injected at bootstrap time. Any deviation
// indicates a bug in ledger bookkeeping or a direct DB mutation.
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
	// without a corresponding ledger entry. Since every ledger transaction is
	// balanced (Σ entries = 0), the sum of all financial_accounts.balance must
	// always equal this value.
	//
	// Canonical source: system_account_bootstrap.go → bankSettlementReserveFloat.
	BankSettlementInitialSeed int64 = 9_000_000_000_000_000
)

// TotalMoneyInvariantChecker verifies the total money invariant using
// finance ledger authority only.
//
// INVARIANT: SUM(financial_accounts.balance) == BankSettlementInitialSeed
//
// This is a trivial consequence of double-entry bookkeeping:
//   - BANK_SETTLEMENT is bootstrapped with BankSettlementInitialSeed
//   - Every subsequent ledger transaction has entries that sum to zero
//   - Therefore the total across all accounts is always the seed value
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
	db db.Transactor,
	log *zap.Logger,
	shadowMode bool,
) *TotalMoneyInvariantChecker {
	if log == nil {
		log = zap.NewNop()
	}

	return &TotalMoneyInvariantChecker{
		alertService: alertService,
		db:           db,
		log:          log,
		shadowMode:   shadowMode,
	}
}

// CheckTotalMoneyInvariant validates the total money invariant.
// Returns true if a violation is detected, false otherwise.
func (c *TotalMoneyInvariantChecker) CheckTotalMoneyInvariant(ctx context.Context) (bool, error) {
	c.log.Debug("Starting total money invariant check",
		zap.Bool("shadow_mode", c.shadowMode),
	)

	actualTotal, err := c.getTotalAccountBalance(ctx)
	if err != nil {
		c.log.Error("Failed to get total account balance", zap.Error(err))
		return false, fmt.Errorf("failed to get total account balance: %w", err)
	}

	expectedTotal := BankSettlementInitialSeed
	difference := actualTotal - expectedTotal

	if difference != 0 {
		// Invariant violation detected
		c.log.Error("Total money invariant violation - CRITICAL",
			zap.Int64("actual_total", actualTotal),
			zap.Int64("expected_total", expectedTotal),
			zap.Int64("difference", difference),
			zap.Bool("shadow_mode", c.shadowMode),
		)

		if !c.shadowMode {
			c.createViolationAlert(ctx, actualTotal, expectedTotal, difference)
		}

		return true, nil
	}

	c.log.Info("Total money invariant check passed",
		zap.Int64("actual_total", actualTotal),
		zap.Int64("expected_total", expectedTotal),
	)

	return false, nil
}

// getTotalAccountBalance calculates SUM(financial_accounts.balance) in a single
// read-only snapshot query.
func (c *TotalMoneyInvariantChecker) getTotalAccountBalance(ctx context.Context) (int64, error) {
	var totalBalance int64

	err := c.db.WithTx(ctx, func(tx db.Tx) error {
		query := `SELECT COALESCE(SUM(balance), 0) FROM financial_accounts`
		return tx.QueryRow(ctx, query).Scan(&totalBalance)
	})

	if err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}

	return totalBalance, nil
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


