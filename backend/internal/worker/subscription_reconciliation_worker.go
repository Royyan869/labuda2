package worker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	subscriptionapp "github.com/labuda/backend/internal/commerce/subscription/application"
	"go.uber.org/zap"
)

const (
	// DefaultSubscriptionReconciliationInterval is how often the worker runs reconciliation checks
	DefaultSubscriptionReconciliationInterval = 10 * time.Minute

	// MaxRecoveryBatches is the maximum number of batches to process per run
	MaxRecoveryBatches = 5

	// RecoveryBatchSize is the number of orphaned payments to process per batch
	RecoveryBatchSize = 50

	// AlertGraceWindow is the number of consecutive failures before escalating to CRITICAL
	AlertGraceWindow = 2
)

// orphanedPayment represents a payment that has no matching subscription record.
type orphanedPayment struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	PaymentNumber string
	GrossAmount   int64
	PaidAt        time.Time
	TransactionID string
}

// SubscriptionReconciliationWorker performs periodic subscription-payment reconciliation checks.
//
// This worker detects AND auto-recovers:
// - Payments with reference_type='subscription' and status='settlement' but no matching subscription record
// - Subscription records with no corresponding payment (data integrity issue)
// - Stale subscription payments that may need manual intervention
//
// AUTO-RECOVERY: For orphaned payments, this worker calls ProcessSuccessfulPayment to create subscriptions.
// The service is idempotent-safe, so duplicate calls are harmless.
// Alert is generated when mismatches are detected for monitoring.
//
// HARDENING (R7.4):
// - Oldest-first processing (ASC by paid_at)
// - Adaptive batching (max 5 loops per run)
// - Payment validation before recovery
// - Alert grace window (2 consecutive failures)
// - Comprehensive metrics
type SubscriptionReconciliationWorker struct {
	db                         *pgxpool.Pool
	log                        *zap.Logger
	interval                   time.Duration
	subscriptionPaymentService *subscriptionapp.SellerSubscriptionPaymentService

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

	shutdownCtx context.Context
	cancelFn    context.CancelFunc

	// HARDENING: Persistent state for alert grace window
	consecutiveFailureCount atomic.Int64

	// HARDENING: Recovery metrics
	recoveryAttemptTotal atomic.Int64
	recoverySuccessTotal atomic.Int64
	recoveryFailureTotal atomic.Int64
}

// SubscriptionReconciliationConfig holds worker configuration
type SubscriptionReconciliationConfig struct {
	Interval time.Duration // How often to run reconciliation checks
}

// DefaultSubscriptionReconciliationConfig returns default configuration
func DefaultSubscriptionReconciliationConfig() SubscriptionReconciliationConfig {
	return SubscriptionReconciliationConfig{
		Interval: DefaultSubscriptionReconciliationInterval,
	}
}

// NewSubscriptionReconciliationWorker creates a new subscription reconciliation worker
func NewSubscriptionReconciliationWorker(
	db *pgxpool.Pool,
	log *zap.Logger,
	cfg SubscriptionReconciliationConfig,
	subscriptionPaymentService *subscriptionapp.SellerSubscriptionPaymentService,
) *SubscriptionReconciliationWorker {
	if log == nil {
		log = zap.NewNop()
	}

	if cfg.Interval == 0 {
		cfg.Interval = DefaultSubscriptionReconciliationInterval
	}

	return &SubscriptionReconciliationWorker{
		db:                         db,
		log:                        log,
		interval:                   cfg.Interval,
		subscriptionPaymentService: subscriptionPaymentService,
		stopCh:                     make(chan struct{}),
	}
}

// Start begins periodic reconciliation checks in the background
func (w *SubscriptionReconciliationWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("Subscription reconciliation worker already running")
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.stopCh = make(chan struct{})

	w.wg.Add(1)
	go w.run()

	w.log.Info("Subscription reconciliation worker started",
		zap.Duration("interval", w.interval),
	)
}

// Stop gracefully shuts down the worker
func (w *SubscriptionReconciliationWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.log.Info("Stopping subscription reconciliation worker...")

	w.cancelFn()
	close(w.stopCh)

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.log.Info("Subscription reconciliation worker stopped gracefully")
	case <-time.After(10 * time.Second):
		w.log.Warn("Subscription reconciliation worker shutdown timeout")
	}

	w.running = false
}

// IsRunning returns true if the worker is currently running
func (w *SubscriptionReconciliationWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// run is the main worker loop
func (w *SubscriptionReconciliationWorker) run() {
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

// RunOnce executes a single reconciliation check.
// Can be called manually for on-demand verification.
func (w *SubscriptionReconciliationWorker) RunOnce() {
	w.runOnce()
}

// runOnce executes all reconciliation checks and logs results
// Does NOT crash on error - logs and continues
func (w *SubscriptionReconciliationWorker) runOnce() {
	ctx := context.Background()

	w.log.Debug("Starting subscription reconciliation check")

	// Run all checks
	results := w.runChecks(ctx)

	// Log results and emit alerts for critical issues
	for _, result := range results {
		switch result.Severity {
		case "CRITICAL":
			w.log.Error("RECONCILIATION ALERT",
				zap.String("check", result.Name),
				zap.String("message", result.Message),
				zap.Int("count", result.Count),
				zap.Any("details", result.Details),
			)
		case "WARNING":
			w.log.Warn("Reconciliation check warning",
				zap.String("check", result.Name),
				zap.String("message", result.Message),
				zap.Int("count", result.Count),
				zap.Any("details", result.Details),
			)
		default:
			w.log.Debug("Reconciliation check OK",
				zap.String("check", result.Name),
				zap.String("message", result.Message),
			)
		}
	}

	// HARDENING: Log metrics summary
	w.log.Debug("Recovery metrics",
		zap.Int64("total_attempts", w.recoveryAttemptTotal.Load()),
		zap.Int64("total_successes", w.recoverySuccessTotal.Load()),
		zap.Int64("total_failures", w.recoveryFailureTotal.Load()),
	)
}

// ReconciliationCheckResult represents the result of a single reconciliation check
type ReconciliationCheckResult struct {
	Name     string
	Severity string // "OK", "WARNING", "CRITICAL"
	Message  string
	Count    int
	Details  []string
}

// runChecks executes all reconciliation checks
func (w *SubscriptionReconciliationWorker) runChecks(ctx context.Context) []ReconciliationCheckResult {
	var results []ReconciliationCheckResult

	// Check 1: Orphaned Payments (payments without subscriptions) - WITH AUTO-RECOVERY
	results = append(results, w.checkAndRecoverOrphanedPayments(ctx))

	// Check 2: Subscription Lifecycle Health
	results = append(results, w.checkSubscriptionLifecycle(ctx))

	// Check 3: Conversion Rate
	results = append(results, w.checkConversionRate(ctx))

	return results
}

// checkAndRecoverOrphanedPayments finds payments with reference_type='subscription' and status='settlement'
// but no matching subscription record, then attempts auto-recovery
//
// HARDENING (R7.4):
// - Oldest-first processing (ORDER BY paid_at ASC)
// - Adaptive batching (loop until no data or max 5 iterations)
// - Payment validation before recovery
// - Duplicate processing logging
// - Alert grace window (2 consecutive cycles)
// - Comprehensive metrics
func (w *SubscriptionReconciliationWorker) checkAndRecoverOrphanedPayments(ctx context.Context) ReconciliationCheckResult {
	// HARDENING: Reset metrics for this run
	runAttemptCount := int64(0)
	runSuccessCount := int64(0)
	runFailureCount := int64(0)

	const query = `
			SELECT p.id, p.user_id, p.payment_number, p.gross_amount, p.paid_at, p.transaction_id
			FROM payments p
			WHERE p.reference_type = 'subscription'
			  AND p.status = 'settlement'
			  AND NOT EXISTS (
			    SELECT 1 FROM seller_subscriptions s WHERE s.payment_id = p.id
			  )
			ORDER BY p.paid_at ASC
			LIMIT $1;
		`


	// HARDENING: Adaptive batching - loop until no data or max iterations
	allOrphanedItems := []orphanedPayment{}
	allFailedItems := []string{}
	totalRecovered := 0

	for batchNum := 0; batchNum < MaxRecoveryBatches; batchNum++ {
		rows, err := w.db.Query(ctx, query, RecoveryBatchSize)
		if err != nil {
			w.log.Error("Orphaned payments check query failed", zap.Error(err))
			return ReconciliationCheckResult{
				Name:     "Orphaned Payments",
				Severity: "WARNING",
				Message:  fmt.Sprintf("Query failed: %v", err),
			}
		}

		var batchItems []orphanedPayment
		for rows.Next() {
			var item orphanedPayment
			if err := rows.Scan(&item.ID, &item.UserID, &item.PaymentNumber, &item.GrossAmount, &item.PaidAt, &item.TransactionID); err != nil {
				w.log.Error("Failed to scan orphaned payment row", zap.Error(err))
				continue
			}
			batchItems = append(batchItems, item)
			allOrphanedItems = append(allOrphanedItems, item)
		}
		rows.Close()

		if err := rows.Err(); err != nil {
			w.log.Error("Error iterating orphaned payments", zap.Error(err))
		}

		// No more items to process
		if len(batchItems) == 0 {
			break
		}

		w.log.Debug("Processing recovery batch",
			zap.Int("batch", batchNum+1),
			zap.Int("batch_size", len(batchItems)),
		)

		// Process this batch
		for _, item := range batchItems {
			if w.subscriptionPaymentService == nil {
				w.log.Warn("Subscription payment service not configured, skipping auto-recovery",
					zap.String("payment_id", item.ID.String()),
				)
				allFailedItems = append(allFailedItems, fmt.Sprintf("PaymentID: %s - Service not configured", item.ID.String()))
				runFailureCount++
				continue
			}

			// HARDENING: Validate payment before processing
			if err := w.validatePaymentForRecovery(ctx, item); err != nil {
				w.log.Error("Payment validation failed, skipping recovery",
					zap.String("payment_id", item.ID.String()),
					zap.String("user_id", item.UserID.String()),
					zap.Error(err),
				)
				allFailedItems = append(allFailedItems, fmt.Sprintf("PaymentID: %s - Validation failed: %v", item.ID.String(), err))
				runFailureCount++
				continue
			}

			w.log.Info("Attempting auto-recovery for orphaned payment",
				zap.String("payment_id", item.ID.String()),
				zap.String("user_id", item.UserID.String()),
				zap.String("payment_number", item.PaymentNumber),
				zap.Int64("amount", item.GrossAmount),
			)

			runAttemptCount++

			// Call ProcessSuccessfulPayment - it's idempotent-safe
			err := w.subscriptionPaymentService.ProcessSuccessfulPayment(
				ctx,
				item.ID,
				item.UserID,
				item.TransactionID,
			)

			if err != nil {
				// HARDENING: Check if this is idempotency (duplicate)
				if w.isIdempotencyError(err) {
					w.log.Info("Duplicate subscription processing prevented (idempotency)",
						zap.String("payment_id", item.ID.String()),
						zap.String("user_id", item.UserID.String()),
					)
					// Count as success - it's already processed
					totalRecovered++
					runSuccessCount++
				} else {
					w.log.Error("Failed to recover orphaned payment",
						zap.String("payment_id", item.ID.String()),
						zap.String("user_id", item.UserID.String()),
						zap.Error(err),
					)
					allFailedItems = append(allFailedItems, fmt.Sprintf("PaymentID: %s, UserID: %s - Error: %v",
						item.ID.String(), item.UserID.String(), err))
					runFailureCount++
				}
			} else {
				totalRecovered++
				runSuccessCount++
				w.log.Info("Successfully recovered orphaned payment",
					zap.String("payment_id", item.ID.String()),
					zap.String("user_id", item.UserID.String()),
				)
			}
		}
	}

	// HARDENING: Update metrics
	w.recoveryAttemptTotal.Add(runAttemptCount)
	w.recoverySuccessTotal.Add(runSuccessCount)
	w.recoveryFailureTotal.Add(runFailureCount)

	// If no orphaned payments found, return OK and reset failure counter
	if len(allOrphanedItems) == 0 {
		w.consecutiveFailureCount.Store(0)
		return ReconciliationCheckResult{
			Name:     "Orphaned Payments",
			Severity: "OK",
			Message:  "No orphaned subscription payments",
		}
	}

	// Prepare details for the result
	details := make([]string, 0, min(len(allOrphanedItems), 10))
	for i, item := range allOrphanedItems {
		if i >= 10 {
			break
		}
		details = append(details, fmt.Sprintf("PaymentID: %s, UserID: %s, Number: %s, Amount: %d, PaidAt: %s",
			item.ID.String(), item.UserID.String(), item.PaymentNumber, item.GrossAmount, item.PaidAt.Format(time.RFC3339)))
	}

	// Determine severity based on recovery results
	if totalRecovered == len(allOrphanedItems) {
		// All recovered successfully - reset failure counter
		w.consecutiveFailureCount.Store(0)
		return ReconciliationCheckResult{
			Name:     "Orphaned Payments",
			Severity: "OK",
			Message:  fmt.Sprintf("Auto-recovered %d orphaned payment(s)", totalRecovered),
			Count:    totalRecovered,
			Details:  details,
		}
	} else if totalRecovered > 0 {
		// Partial recovery
		return ReconciliationCheckResult{
			Name:     "Orphaned Payments",
			Severity: "WARNING",
			Message:  fmt.Sprintf("Partially recovered: %d/%d (failed: %d)", totalRecovered, len(allOrphanedItems), len(allOrphanedItems)-totalRecovered),
			Count:    len(allOrphanedItems) - totalRecovered,
			Details:  append(details, allFailedItems...),
		}
	} else {
		// All failed - HARDENING: Apply alert grace window
		consecutiveFailures := w.consecutiveFailureCount.Add(1)

		// Only CRITICAL after 2 consecutive failures
		if consecutiveFailures >= AlertGraceWindow {
			return ReconciliationCheckResult{
				Name:     "Orphaned Payments",
				Severity: "CRITICAL",
				Message:  fmt.Sprintf("Failed to recover %d orphaned payment(s) for %d consecutive cycles", len(allOrphanedItems), consecutiveFailures),
				Count:    len(allOrphanedItems),
				Details:  append(details, allFailedItems...),
			}
		}

		// First failure - only WARNING
		return ReconciliationCheckResult{
			Name:     "Orphaned Payments",
			Severity: "WARNING",
			Message:  fmt.Sprintf("Failed to recover %d orphaned payment(s) (consecutive failures: %d/%d - will escalate to CRITICAL on next failure)", len(allOrphanedItems), consecutiveFailures, AlertGraceWindow),
			Count:    len(allOrphanedItems),
			Details:  append(details, allFailedItems...),
		}
	}
}

// validatePaymentForRecovery validates a payment before attempting recovery
//
// HARDENING (R7.4): Ensures payment data integrity before processing
func (w *SubscriptionReconciliationWorker) validatePaymentForRecovery(ctx context.Context, item orphanedPayment) error {
	// Validate amount is positive
	if item.GrossAmount <= 0 {
		return fmt.Errorf("invalid payment amount: %d", item.GrossAmount)
	}

	// Validate transaction ID is present
	if strings.TrimSpace(item.TransactionID) == "" {
		return fmt.Errorf("empty transaction ID")
	}

	// Validate payment date is not in the future
	if item.PaidAt.After(time.Now()) {
		return fmt.Errorf("payment date is in the future: %s", item.PaidAt.Format(time.RFC3339))
	}

	// Validate payment date is not too old (more than 1 year)
	if time.Since(item.PaidAt) > 365*24*time.Hour {
		return fmt.Errorf("payment date is too old: %s", item.PaidAt.Format(time.RFC3339))
	}

	// Validate against expected subscription plan amount
	var expectedYearlyFeeRupiah int64
	err := w.db.QueryRow(ctx, `
		SELECT yearly_fee_rupiah
		FROM seller_subscription_configs
		WHERE enabled = true
		ORDER BY created_at DESC
		LIMIT 1
	`).Scan(&expectedYearlyFeeRupiah)

	if err != nil {
		// If no config found, log warning but don't fail
		w.log.Warn("No active subscription config found for amount validation",
			zap.Error(err),
			zap.String("payment_id", item.ID.String()),
		)
		return nil
	}

	// Check if payment amount matches expected yearly fee (with small tolerance)
	if item.GrossAmount != expectedYearlyFeeRupiah {
		w.log.Warn("Payment amount does not match expected subscription fee",
			zap.String("payment_id", item.ID.String()),
			zap.Int64("expected_amount", expectedYearlyFeeRupiah),
			zap.Int64("actual_amount", item.GrossAmount),
		)
		// Don't fail - just warn, as pricing might have changed
	}

	return nil
}

// isIdempotencyError reports whether err is a PostgreSQL UNIQUE constraint
// violation (SQLSTATE 23505), which indicates the payment was already processed
// by a concurrent worker tick or pod.
//
// Uses pgconn.PgError.Code instead of string matching to avoid false positives
// from error messages that happen to contain the word "duplicate".
func (w *SubscriptionReconciliationWorker) isIdempotencyError(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" // unique_violation
}

// checkSubscriptionLifecycle checks subscription lifecycle health
func (w *SubscriptionReconciliationWorker) checkSubscriptionLifecycle(ctx context.Context) ReconciliationCheckResult {
	// Check for stuck subscriptions (e.g., active but expired long ago)
	const query = `
			SELECT COUNT(*)
			FROM seller_subscriptions
			WHERE status = 'active'
			  AND expires_at < NOW() - INTERVAL '1 day';
		`

	var stuckCount int
	err := w.db.QueryRow(ctx, query).Scan(&stuckCount)
	if err != nil {
		w.log.Error("Subscription lifecycle check failed", zap.Error(err))
		return ReconciliationCheckResult{
			Name:     "Subscription Lifecycle",
			Severity: "WARNING",
			Message:  fmt.Sprintf("Query failed: %v", err),
		}
	}

	if stuckCount > 0 {
		return ReconciliationCheckResult{
			Name:     "Subscription Lifecycle",
			Severity: "WARNING",
			Message:  "Active subscriptions with expired status > 1 day (expiry worker lag)",
			Count:    stuckCount,
		}
	}

	return ReconciliationCheckResult{
		Name:     "Subscription Lifecycle",
		Severity: "OK",
		Message:  "Subscription lifecycle healthy",
	}
}

// checkConversionRate calculates subscription payment conversion rate
func (w *SubscriptionReconciliationWorker) checkConversionRate(ctx context.Context) ReconciliationCheckResult {
	const query = `
			WITH payment_stats AS (
			  SELECT
			    COUNT(*) FILTER (WHERE status = 'settlement') as total_settlement,
			    COUNT(*) FILTER (
			      WHERE status = 'settlement'
			      AND EXISTS (SELECT 1 FROM seller_subscriptions s WHERE s.payment_id = payments.id)
			    ) as converted
			  FROM payments
			  WHERE reference_type = 'subscription'
			)
			SELECT
			  COALESCE(total_settlement, 0) as total,
			  COALESCE(converted, 0) as converted
			FROM payment_stats;
		`

	var total, converted int
	err := w.db.QueryRow(ctx, query).Scan(&total, &converted)
	if err != nil {
		w.log.Error("Conversion rate check failed", zap.Error(err))
		return ReconciliationCheckResult{
			Name:     "Conversion Rate",
			Severity: "WARNING",
			Message:  fmt.Sprintf("Query failed: %v", err),
		}
	}

	if total == 0 {
		return ReconciliationCheckResult{
			Name:     "Conversion Rate",
			Severity: "OK",
			Message:  "No subscription payments yet",
		}
	}

	conversionRate := float64(converted) / float64(total)
	if conversionRate < 0.95 {
		return ReconciliationCheckResult{
			Name:     "Conversion Rate",
			Severity: "CRITICAL",
			Message:  fmt.Sprintf("Low conversion rate: %.2f%% (%d/%d)", conversionRate*100, converted, total),
			Count:    total - converted,
		}
	}

	return ReconciliationCheckResult{
		Name:     "Conversion Rate",
		Severity: "OK",
		Message:  fmt.Sprintf("Healthy conversion rate: %.2f%% (%d/%d)", conversionRate*100, converted, total),
	}
}

// GetRecoveryMetrics returns the current recovery metrics
//
// HARDENING (R7.4): Expose metrics for monitoring
func (w *SubscriptionReconciliationWorker) GetRecoveryMetrics() map[string]int64 {
	return map[string]int64{
		"subscription_recovery_attempt_total": w.recoveryAttemptTotal.Load(),
		"subscription_recovery_success_total": w.recoverySuccessTotal.Load(),
		"subscription_recovery_failure_total": w.recoveryFailureTotal.Load(),
		"consecutive_failure_count":           w.consecutiveFailureCount.Load(),
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}


