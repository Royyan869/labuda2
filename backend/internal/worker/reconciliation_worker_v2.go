package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	finance "github.com/labuda/backend/internal/finance"
	"github.com/labuda/backend/internal/finance/entity"
	"github.com/labuda/backend/internal/finance/repository"
	alertapp "github.com/labuda/backend/internal/platform/alert/application"
	alertentity "github.com/labuda/backend/internal/platform/alert/entity"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

const (
	// DefaultReconciliationIntervalV2 is how often the v2 worker runs reconciliation checks
	DefaultReconciliationIntervalV2 = 5 * time.Minute

	// DefaultEscalationThreshold is the difference (in cents) at which a non-critical
	// drift escalates from MEDIUM to HIGH severity. It does NOT enable mutation.
	DefaultEscalationThreshold = 100

	// bankSettlementSeedBalance is the initial balance bootstrapped into BANK_SETTLEMENT
	// without a corresponding ledger entry. checkAccountBalances must offset by this
	// value to avoid a false-positive mismatch.
	// Canonical source: system_account_bootstrap.go → bankSettlementReserveFloat.
	bankSettlementSeedBalance int64 = 9_000_000_000_000_000
)

// ReconciliationAlertService is the interface used by ReconciliationWorkerV2 for alerting.
type ReconciliationAlertService interface {
	CreateReconciliationDriftAlert(ctx context.Context, severity alertentity.AlertSeverity, mismatchedAccounts int, totalAccounts int, details alertentity.AlertMetadata) (*alertapp.CreateAlertResult, error)
}

// ReconciliationWorkerV2 performs periodic integrity checks with:
// - Persistent logging of all results
// - Alert creation on mismatches
// - Severity-graded escalation
//
// CONSTITUTIONAL ROLE (RUNTIME-INVARIANTS §7.1 + ADR-002):
// Reconciliation is VERIFICATION + ESCALATION only. It MUST NOT mutate the
// ledger, wallet, or any canonical authority. Corrective journal entries are
// the exclusive responsibility of canonical FinanceService methods invoked by
// an attributable operator. There is no "auto-repair" path — silently
// reshaping truth to match the ledger sum violates ADR-002 and §7.7.
type ReconciliationWorkerV2 struct {
	db       Transactor
	log      *zap.Logger
	interval time.Duration
	strict   bool // panic on critical error if true

	reconcileRepo repository.ReconciliationRepository
	alertService  ReconciliationAlertService

	enableAlerting      bool
	escalationThreshold int64

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

	shutdownCtx context.Context
	cancelFn    context.CancelFunc
}

// ReconciliationConfigV2 holds worker configuration.
//
// Note: there is intentionally no auto-repair toggle. Reconciliation is
// constitutionally read-only; the configuration surface does not expose a way
// to flip it.
type ReconciliationConfigV2 struct {
	Interval            time.Duration
	Strict              bool
	EnableAlerting      bool
	EscalationThreshold int64 // Drift above this escalates non-critical mismatches from MEDIUM to HIGH (cents)
}

// DefaultReconciliationConfigV2 returns the constitutional default: detection
// and escalation only, no mutation.
func DefaultReconciliationConfigV2() ReconciliationConfigV2 {
	return ReconciliationConfigV2{
		Interval:            DefaultReconciliationIntervalV2,
		Strict:              false,
		EnableAlerting:      true,
		EscalationThreshold: DefaultEscalationThreshold,
	}
}

// NewReconciliationWorkerV2 creates a verification-only reconciliation worker.
func NewReconciliationWorkerV2(
	db Transactor,
	log *zap.Logger,
	reconcileRepo repository.ReconciliationRepository,
	alertService ReconciliationAlertService,
	cfg ReconciliationConfigV2,
) *ReconciliationWorkerV2 {
	if log == nil {
		log = zap.NewNop()
	}

	if cfg.Interval == 0 {
		cfg.Interval = DefaultReconciliationIntervalV2
	}
	if cfg.EscalationThreshold == 0 {
		cfg.EscalationThreshold = DefaultEscalationThreshold
	}

	return &ReconciliationWorkerV2{
		db:                  db,
		log:                 log,
		interval:            cfg.Interval,
		strict:              cfg.Strict,
		reconcileRepo:       reconcileRepo,
		alertService:        alertService,
		enableAlerting:      cfg.EnableAlerting,
		escalationThreshold: cfg.EscalationThreshold,
		stopCh:              make(chan struct{}),
	}
}

// Start begins periodic reconciliation checks in the background
func (w *ReconciliationWorkerV2) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("Reconciliation worker already running")
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.stopCh = make(chan struct{})

	w.wg.Add(1)
	go w.run()

	w.log.Info("Reconciliation worker V2 started (verification-only)",
		zap.Duration("interval", w.interval),
		zap.Bool("strict", w.strict),
		zap.Bool("alerting_enabled", w.enableAlerting),
		zap.Int64("escalation_threshold_cents", w.escalationThreshold),
	)
}

// Stop gracefully shuts down the worker
func (w *ReconciliationWorkerV2) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.log.Info("Stopping reconciliation worker V2...")

	w.cancelFn()
	close(w.stopCh)

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.log.Info("Reconciliation worker V2 stopped gracefully")
	case <-time.After(10 * time.Second):
		w.log.Warn("Reconciliation worker V2 shutdown timeout")
	}

	w.running = false
}

// IsRunning returns true if the worker is currently running
func (w *ReconciliationWorkerV2) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// RunOnce executes a single reconciliation check
func (w *ReconciliationWorkerV2) RunOnce() error {
	return w.runOnce()
}

// run is the main worker loop
func (w *ReconciliationWorkerV2) run() {
	defer w.wg.Done()

	w.runOnce()

	for {
		select {
		case <-w.shutdownCtx.Done():
			w.log.Info("Worker shutdown requested")
			return

		case <-time.After(w.interval):
			w.runOnce()

		case <-w.stopCh:
			return
		}
	}
}

// runOnce executes all reconciliation checks (read-only).
func (w *ReconciliationWorkerV2) runOnce() error {
	ctx := context.Background()
	checkedAt := time.Now()

	w.log.Debug("Starting reconciliation check (V2 — verification only)")

	var allIssues []ReconciliationIssue

	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		if issues := w.checkTransactionBalance(ctx, tx); len(issues) > 0 {
			allIssues = append(allIssues, issues...)
		}
		if issues := w.checkAccountBalances(ctx, tx); len(issues) > 0 {
			allIssues = append(allIssues, issues...)
		}
		if issues := w.checkCriticalAccounts(ctx, tx); len(issues) > 0 {
			allIssues = append(allIssues, issues...)
		}
		if issues := w.checkWithdrawalConsistency(ctx, tx); len(issues) > 0 {
			allIssues = append(allIssues, issues...)
		}
		return nil
	})

	if err != nil {
		w.log.Error("Reconciliation check transaction failed", zap.Error(err))
		result := entity.NewReconciliationResult(
			checkedAt,
			0,
			0,
			entity.SeverityReconcileMedium,
			entity.ReconcileDetails{"error": err.Error()},
		)
		w.persistResult(ctx, result)
		return err
	}

	severity, totalAccounts, mismatchedAccounts := w.categorizeIssues(allIssues)

	details := w.buildResultDetails(allIssues)
	result := entity.NewReconciliationResult(
		checkedAt,
		totalAccounts,
		mismatchedAccounts,
		severity,
		details,
	)

	if severity == entity.SeverityReconcilePassed {
		result.WithAction(entity.ActionNone)
		w.log.Info("Reconciliation passed: no issues detected")
	} else {
		w.handleIssues(ctx, result, allIssues)
	}

	w.persistResult(ctx, result)

	if severity == entity.SeverityReconcileCritical && w.strict {
		panic(fmt.Sprintf("critical reconciliation failure: %d issues", len(allIssues)))
	}

	return nil
}

// ReconciliationIssue represents a detected issue
type ReconciliationIssue struct {
	Type     string // "transaction_balance", "account_mismatch", "critical_account", "withdrawal"
	Severity entity.ReconcileSeverity
	Message  string
	Details  interface{}
}

// categorizeIssues determines overall severity from collected issues
func (w *ReconciliationWorkerV2) categorizeIssues(issues []ReconciliationIssue) (entity.ReconcileSeverity, int, int) {
	if len(issues) == 0 {
		return entity.SeverityReconcilePassed, 0, 0
	}

	totalAccounts := 0
	mismatchedAccounts := 0

	hasCritical := false
	hasHigh := false

	for _, issue := range issues {
		if issue.Severity == entity.SeverityReconcileCritical {
			hasCritical = true
		} else if issue.Severity == entity.SeverityReconcileHigh {
			hasHigh = true
		}

		if issue.Type == "account_mismatch" {
			mismatchedAccounts++
		}
		if issue.Type == "account_mismatch" || issue.Type == "critical_account" {
			totalAccounts++
		}
	}

	if hasCritical {
		return entity.SeverityReconcileCritical, totalAccounts, mismatchedAccounts
	}
	if hasHigh {
		return entity.SeverityReconcileHigh, totalAccounts, mismatchedAccounts
	}
	return entity.SeverityReconcileMedium, totalAccounts, mismatchedAccounts
}

// buildResultDetails constructs the details JSONB for persistence
func (w *ReconciliationWorkerV2) buildResultDetails(issues []ReconciliationIssue) entity.ReconcileDetails {
	details := entity.ReconcileDetails{
		"total_issues": len(issues),
		"checked_at":   time.Now(),
	}

	var accountMismatches []entity.AccountMismatch
	var transactionImbalances []entity.TransactionImbalance
	var otherIssues []map[string]interface{}

	for _, issue := range issues {
		switch issue.Type {
		case "account_mismatch":
			if am, ok := issue.Details.(entity.AccountMismatch); ok {
				accountMismatches = append(accountMismatches, am)
			}
		case "transaction_balance":
			if ti, ok := issue.Details.(entity.TransactionImbalance); ok {
				transactionImbalances = append(transactionImbalances, ti)
			}
		default:
			otherIssues = append(otherIssues, map[string]interface{}{
				"type":    issue.Type,
				"message": issue.Message,
				"details": issue.Details,
			})
		}
	}

	if len(accountMismatches) > 0 {
		details["account_mismatches"] = accountMismatches
	}
	if len(transactionImbalances) > 0 {
		details["transaction_imbalances"] = transactionImbalances
	}
	if len(otherIssues) > 0 {
		details["other_issues"] = otherIssues
	}

	return details
}

// handleIssues escalates detected issues via alerting. No mutation path.
//
// Correction of any detected mismatch is the responsibility of an attributable
// operator invoking canonical FinanceService methods. This worker never writes
// to the ledger.
func (w *ReconciliationWorkerV2) handleIssues(ctx context.Context, result *entity.ReconciliationResult, issues []ReconciliationIssue) {
	needsEscalation := result.NeedsEscalation()

	w.log.Warn("Reconciliation issues detected — escalating (no mutation)",
		zap.Int("total_issues", len(issues)),
		zap.String("severity", string(result.Severity)),
		zap.Bool("needs_escalation", needsEscalation),
	)

	alertSeverity := reconcileSeverityToAlertSeverity(result.Severity)
	if needsEscalation {
		w.log.Error("CRITICAL/HIGH reconciliation issue — ESCALATION REQUIRED",
			zap.Int("issues", len(issues)),
		)
	}

	result.WithAction(entity.ActionEscalated)

	if w.enableAlerting {
		w.createAlert(ctx, result, alertSeverity)
	}
}

// reconcileSeverityToAlertSeverity maps the reconciliation domain's own
// severity taxonomy onto the platform-wide AlertSeverity, so admins can
// distinguish a MEDIUM ledger nuisance from a HIGH/CRITICAL drift directly
// from the alert badge/filter instead of every non-passed result reading as
// "High".
func reconcileSeverityToAlertSeverity(severity entity.ReconcileSeverity) alertentity.AlertSeverity {
	switch severity {
	case entity.SeverityReconcileCritical:
		return alertentity.SeverityCritical
	case entity.SeverityReconcileHigh:
		return alertentity.SeverityHigh
	case entity.SeverityReconcileMedium:
		return alertentity.SeverityMedium
	case entity.SeverityReconcileLow:
		return alertentity.SeverityLow
	default:
		return alertentity.SeverityMedium
	}
}

// createAlert creates an alert for reconciliation issues
func (w *ReconciliationWorkerV2) createAlert(ctx context.Context, result *entity.ReconciliationResult, severity alertentity.AlertSeverity) {
	metadata := make(alertentity.AlertMetadata)
	for k, v := range result.Details {
		metadata[k] = v
	}

	_, err := w.alertService.CreateReconciliationDriftAlert(
		ctx,
		severity,
		result.MismatchedAccounts,
		result.TotalAccounts,
		metadata,
	)

	if err != nil {
		w.log.Error("Failed to create reconciliation alert", zap.Error(err))
	}
}

// persistResult saves the reconciliation result to the database
func (w *ReconciliationWorkerV2) persistResult(ctx context.Context, result *entity.ReconciliationResult) {
	if w.reconcileRepo == nil {
		w.log.Debug("Reconciliation repository not configured - skipping persistence")
		return
	}

	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		return w.reconcileRepo.Create(ctx, tx, result)
	})

	if err != nil {
		w.log.Error("Failed to persist reconciliation result", zap.Error(err))
	} else {
		w.log.Debug("Reconciliation result persisted",
			zap.String("result_id", result.ID.String()),
			zap.String("severity", string(result.Severity)),
			zap.String("action_taken", string(result.ActionTaken)),
		)
	}
}

// checkTransactionBalance verifies double-entry invariant.
//
// ledger_entries.amount is always positive (CHECK amount > 0);
// entry_type distinguishes debit (+) from credit (-). The signed
// sum reconstructs the net movement per transaction, which must be zero.
func (w *ReconciliationWorkerV2) checkTransactionBalance(ctx context.Context, tx db.Tx) []ReconciliationIssue {
	query := `
		SELECT transaction_id
		FROM ledger_entries
		GROUP BY transaction_id
		HAVING SUM(CASE WHEN entry_type = 'debit' THEN amount ELSE -amount END) != 0;
	`

	rows, err := tx.Query(ctx, query)
	if err != nil {
		w.log.Error("Failed to query unbalanced transactions", zap.Error(err))
		return []ReconciliationIssue{{
			Type:     "transaction_balance",
			Severity: entity.SeverityReconcileHigh,
			Message:  fmt.Sprintf("Failed to query: %v", err),
			Details:  err.Error(),
		}}
	}
	defer rows.Close()

	var issues []ReconciliationIssue
	for rows.Next() {
		var txID uuid.UUID
		if err := rows.Scan(&txID); err != nil {
			continue
		}

		issues = append(issues, ReconciliationIssue{
			Type:     "transaction_balance",
			Severity: entity.SeverityReconcileCritical,
			Message:  "Unbalanced transaction detected",
			Details: entity.TransactionImbalance{
				TransactionID: txID,
			},
		})
	}

	return issues
}

// checkAccountBalances verifies stored vs calculated balances.
//
// ledger_entries.amount is always positive (CHECK amount > 0);
// entry_type distinguishes debit (+) from credit (-). The signed
// sum reconstructs the net movement per account.
//
// BANK_SETTLEMENT is bootstrapped with an initial balance of
// 9,000,000,000,000,000 (Rp 90 T reserve float) without a ledger
// entry, so the comparison offsets by this seed value.
// See: system_account_bootstrap.go → bankSettlementReserveFloat.
func (w *ReconciliationWorkerV2) checkAccountBalances(ctx context.Context, tx db.Tx) []ReconciliationIssue {
	ledgerQuery := `
		SELECT account_id,
		       SUM(CASE WHEN entry_type = 'debit' THEN amount ELSE -amount END) as calculated_balance
		FROM ledger_entries
		GROUP BY account_id;
	`

	rows, err := tx.Query(ctx, ledgerQuery)
	if err != nil {
		return []ReconciliationIssue{{
			Type:     "account_balance",
			Severity: entity.SeverityReconcileMedium,
			Message:  fmt.Sprintf("Failed to query ledger: %v", err),
		}}
	}
	defer rows.Close()

	type ledgerBalance struct {
		accountID         uuid.UUID
		calculatedBalance int64
	}

	var ledgerBalances []ledgerBalance
	for rows.Next() {
		var lb ledgerBalance
		if err := rows.Scan(&lb.accountID, &lb.calculatedBalance); err != nil {
			continue
		}
		ledgerBalances = append(ledgerBalances, lb)
	}

	var issues []ReconciliationIssue

	for _, lb := range ledgerBalances {
		var storedBalance int64
		var accountType string
		var userID *uuid.UUID

		err := tx.QueryRow(ctx, `
			SELECT balance, account_type, user_id
			FROM financial_accounts
			WHERE id = $1;
		`, lb.accountID).Scan(&storedBalance, &accountType, &userID)

		if err != nil {
			issues = append(issues, ReconciliationIssue{
				Type:     "account_mismatch",
				Severity: entity.SeverityReconcileHigh,
				Message:  "Account in ledger but not in financial_accounts",
				Details: entity.AccountMismatch{
					AccountID:         lb.accountID,
					StoredBalance:     0,
					CalculatedBalance: lb.calculatedBalance,
					Difference:        -lb.calculatedBalance,
					IsCritical:        entity.IsCriticalAccountType(accountType),
					OwnerID:           userID,
				},
			})
			continue
		}

		// BANK_SETTLEMENT is bootstrapped with a seed balance that has no
		// corresponding ledger entry, so the expected balance is seed + net movement.
		expectedBalance := lb.calculatedBalance
		if accountType == finance.AccountBankSettlement {
			expectedBalance += bankSettlementSeedBalance
		}

		if storedBalance != expectedBalance {
			difference := storedBalance - expectedBalance
			isCritical := entity.IsCriticalAccountType(accountType)

			severity := entity.SeverityReconcileMedium
			if isCritical {
				severity = entity.SeverityReconcileCritical
			} else if abs(difference) > w.escalationThreshold {
				severity = entity.SeverityReconcileHigh
			}

			issues = append(issues, ReconciliationIssue{
				Type:     "account_mismatch",
				Severity: severity,
				Message:  fmt.Sprintf("Balance drift: %d", difference),
				Details: entity.AccountMismatch{
					AccountID:         lb.accountID,
					AccountType:       accountType,
					StoredBalance:     storedBalance,
					CalculatedBalance: lb.calculatedBalance,
					Difference:        difference,
					IsCritical:        isCritical,
					OwnerID:           userID,
				},
			})
		}
	}

	return issues
}

// checkCriticalAccounts verifies critical accounts have non-negative balances
func (w *ReconciliationWorkerV2) checkCriticalAccounts(ctx context.Context, tx db.Tx) []ReconciliationIssue {
	criticalTypes := []string{
		"ESCROW",
		"WITHDRAWAL_PENDING",
		"WITHDRAWAL_COMMITTED",
		"GATEWAY_CLEARING",
	}

	query := `
		SELECT id, account_type, balance
		FROM financial_accounts
		WHERE account_type = ANY($1)
		  AND balance < 0;
	`

	rows, err := tx.Query(ctx, query, criticalTypes)
	if err != nil {
		return []ReconciliationIssue{{
			Type:     "critical_account",
			Severity: entity.SeverityReconcileMedium,
			Message:  fmt.Sprintf("Failed to query critical accounts: %v", err),
		}}
	}
	defer rows.Close()

	var issues []ReconciliationIssue
	for rows.Next() {
		var id uuid.UUID
		var accountType string
		var balance int64

		if err := rows.Scan(&id, &accountType, &balance); err != nil {
			continue
		}

		issues = append(issues, ReconciliationIssue{
			Type:     "critical_account",
			Severity: entity.SeverityReconcileCritical,
			Message:  fmt.Sprintf("Negative balance in critical account: %s", accountType),
			Details: map[string]interface{}{
				"account_id":   id,
				"account_type": accountType,
				"balance":      balance,
			},
		})
	}

	return issues
}

// checkWithdrawalConsistency verifies withdrawal integrity
func (w *ReconciliationWorkerV2) checkWithdrawalConsistency(ctx context.Context, tx db.Tx) []ReconciliationIssue {
	var issues []ReconciliationIssue

	sumQuery := `
		SELECT COALESCE(SUM(amount), 0)
		FROM withdrawals
		WHERE status IN ('REQUESTED', 'PROCESSING');
	`

	var expectedSum int64
	err := tx.QueryRow(ctx, sumQuery).Scan(&expectedSum)
	if err != nil {
		return []ReconciliationIssue{{
			Type:     "withdrawal",
			Severity: entity.SeverityReconcileMedium,
			Message:  fmt.Sprintf("Failed to sum pending withdrawals: %v", err),
		}}
	}

	balanceQuery := `
		SELECT COALESCE(balance, 0)
		FROM financial_accounts
		WHERE account_type = 'WITHDRAWAL_PENDING';
	`

	var actualBalance int64
	err = tx.QueryRow(ctx, balanceQuery).Scan(&actualBalance)
	if err != nil {
		return []ReconciliationIssue{{
			Type:     "withdrawal",
			Severity: entity.SeverityReconcileMedium,
			Message:  fmt.Sprintf("Failed to query WITHDRAWAL_PENDING balance: %v", err),
		}}
	}

	if expectedSum != actualBalance {
		severity := entity.SeverityReconcileHigh
		if abs(expectedSum-actualBalance) > 1000 {
			severity = entity.SeverityReconcileCritical
		}

		issues = append(issues, ReconciliationIssue{
			Type:     "withdrawal",
			Severity: severity,
			Message:  "Withdrawal pending balance drift",
			Details: map[string]interface{}{
				"expected_sum":   expectedSum,
				"actual_balance": actualBalance,
				"difference":     expectedSum - actualBalance,
			},
		})
	}

	return issues
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
