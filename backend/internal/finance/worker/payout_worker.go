package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance"
	"github.com/labuda/backend/internal/finance/infrastructure/repository"
	ledgerintf "github.com/labuda/backend/internal/finance/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"go.uber.org/zap"
)

const (
	// DefaultPayoutPollInterval is how often the worker checks for withdrawals to process
	DefaultPayoutPollInterval = 30 * time.Second

	// DefaultPayoutBatchSize is max withdrawals to process per batch
	DefaultPayoutBatchSize = 10

	// DefaultRetryBatchSize is max retryable withdrawals to process per batch
	DefaultRetryBatchSize = 5

	// gatewayRestoreKeyFmt is the idempotency key format used when a gateway
	// payout failure triggers a ledger reversal (WITHDRAWAL_COMMITTED → SELLER_PAYABLE).
	// Both WebhookHandler.handleFailedCallback and PayoutWorker.markSubmissionFailed
	// use this same format so the DB unique constraint prevents double-restoration
	// regardless of which path fires first.
	gatewayRestoreKeyFmt = "withdrawal_gateway_restore_%s"
)

// PayoutGateway defines the interface for payment gateway integration.
type PayoutGateway interface {
	SubmitPayout(ctx context.Context, req PayoutGatewayRequest) (*PayoutGatewayResponse, error)
}

// PayoutGatewayRequest represents a payout request to the gateway.
type PayoutGatewayRequest struct {
	ExternalReferenceID string
	Amount              int64
	Currency            string
	BankName            string
	AccountNumber       string
	AccountHolder       string
	Metadata            map[string]string
}

// PayoutGatewayResponse represents the gateway's response.
type PayoutGatewayResponse struct {
	Status             PayoutResponseStatus
	GatewayReferenceID string
	Message            string
	RawResponse        string
	ErrorType          PayoutErrorType
}

// PayoutResponseStatus represents the outcome of a payout submission.
type PayoutResponseStatus string

const (
	PayoutResponseStatusSuccess  PayoutResponseStatus = "SUCCESS"
	PayoutResponseStatusPending  PayoutResponseStatus = "PENDING"
	PayoutResponseStatusFailed   PayoutResponseStatus = "FAILED"
	PayoutResponseStatusRejected PayoutResponseStatus = "REJECTED"
)

// PayoutErrorType indicates the nature of a payout error.
type PayoutErrorType string

const (
	ErrorTypeRetryable PayoutErrorType = "RETRYABLE"
	ErrorTypePermanent PayoutErrorType = "PERMANENT"
)

// IsSuccess returns true if the payout was successful.
func (p *PayoutGatewayResponse) IsSuccess() bool {
	return p.Status == PayoutResponseStatusSuccess || p.Status == PayoutResponseStatusPending
}

// IsRetryable returns true if the error is temporary.
func (p *PayoutGatewayResponse) IsRetryable() bool {
	return p.Status == PayoutResponseStatusFailed && p.ErrorType == ErrorTypeRetryable
}

// PayoutWorker processes withdrawal payouts by submitting them to the payment gateway.
//
// SAFETY GUARDS:
// - Uses FOR UPDATE locking for concurrent worker support
// - Idempotent submission via external_reference_id
// - Controlled retry with exponential backoff
// - Max retry limit to prevent infinite loops
// - Per-item transaction isolation
// - PILOT MODE: Optional whitelist restriction for production safety
//
// WORKER FLOW:
// 1. Poll for PROCESSING withdrawals (ready for submission)
// 2. Poll for FAILED_RETRYABLE withdrawals (ready for retry)
// 3. Check pilot mode whitelist (if enabled)
// 4. Submit to gateway with idempotency key
// 5. Handle success → MarkSettled
// 6. Handle temporary error → MarkFailed(FAILED_RETRYABLE)
// 7. Handle permanent error → MarkFailed(FAILED_FINAL)
type PayoutWorker struct {
	db             Transactor
	withdrawRepo   *repository.WithdrawRepository
	ledgerRepo     *repository.LedgerRepository
	gateway        PayoutGateway
	log            *zap.Logger
	pollInterval   time.Duration
	batchSize      int
	retryBatchSize int

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

	// Context for shutdown
	shutdownCtx context.Context
	cancelFn    context.CancelFunc

	// PILOT MODE CONFIGURATION
	enablePilotMode       bool              // When true, only whitelisted sellers can process payouts
	whitelistMgr          *WhitelistManager // Audited whitelist — replaces raw map
	pilotBlockedThreshold int64             // Emit CRITICAL alert when accumulated blocks reach this

	// OBSERVABILITY
	metrics *PayoutMetrics
}

// Transactor interface for database transactions
type Transactor interface {
	WithTx(ctx context.Context, fn func(tx db.Tx) error) error
}

// PayoutWorkerConfig holds worker configuration
type PayoutWorkerConfig struct {
	PollInterval   time.Duration // How often to check for withdrawals to process
	BatchSize      int           // Max withdrawals to process per batch
	RetryBatchSize int           // Max retryable withdrawals to process per batch

	// PILOT MODE SETTINGS
	EnablePilotMode       bool        // When true, only whitelisted sellers can process payouts
	PilotWhitelist        []uuid.UUID // List of seller IDs allowed in pilot mode
	PilotBlockedThreshold int64       // Emit CRITICAL alert when cumulative PILOT_BLOCKED >= this (0 = disabled)
}

// DefaultPayoutWorkerConfig returns default configuration
func DefaultPayoutWorkerConfig() PayoutWorkerConfig {
	return PayoutWorkerConfig{
		PollInterval:    DefaultPayoutPollInterval,
		BatchSize:       DefaultPayoutBatchSize,
		RetryBatchSize:  DefaultRetryBatchSize,
		EnablePilotMode: true, // Default TRUE for safety
		PilotWhitelist:  nil,  // Empty means no one is whitelisted
	}
}

// ValidatePayoutWorkerConfig returns an error if the configuration is unsafe to start.
// Must be called before NewPayoutWorker so the process can fail fast at startup
// rather than silently blocking every seller at runtime.
//
// LANDMINE GUARD: EnablePilotMode=true with an empty PilotWhitelist means zero
// sellers can receive payouts. This is intentional as a safety gate but must be
// an explicit, conscious choice — not an accidental default.
func ValidatePayoutWorkerConfig(cfg PayoutWorkerConfig) error {
	if cfg.EnablePilotMode && len(cfg.PilotWhitelist) == 0 {
		return fmt.Errorf(
			"PAYOUT WORKER CONFIG INVALID: EnablePilotMode=true but PilotWhitelist is empty. " +
				"All sellers would be silently blocked. " +
				"Either populate PAYOUT_PILOT_WHITELIST or set PAYOUT_ENABLE_PILOT_MODE=false",
		)
	}
	return nil
}

// NewPayoutWorker creates a new payout worker.
//
// whitelistAuditRepo may be nil (local dev: audit to in-process log + structured
// logger only). When non-nil (staging/production) the initial whitelist audit
// record is written to the DB; failure returns an error so the caller can
// terminate startup fail-closed.
func NewPayoutWorker(
	db Transactor,
	withdrawRepo *repository.WithdrawRepository,
	ledgerRepo *repository.LedgerRepository,
	gateway PayoutGateway,
	log *zap.Logger,
	cfg PayoutWorkerConfig,
	whitelistAuditRepo repository.WhitelistAuditRepository,
) (*PayoutWorker, error) {
	if log == nil {
		log = zap.NewNop()
	}

	if cfg.PollInterval == 0 {
		cfg.PollInterval = DefaultPayoutPollInterval
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = DefaultPayoutBatchSize
	}
	if cfg.RetryBatchSize == 0 {
		cfg.RetryBatchSize = DefaultRetryBatchSize
	}

	// Build audited whitelist manager when pilot mode is enabled.
	// When whitelistAuditRepo is non-nil, the INITIALIZED event is persisted to
	// DB and a failure is treated as fatal (fail-closed for staging/production).
	// When whitelistAuditRepo is nil (dev), audit goes to in-process + logs only.
	var wm *WhitelistManager
	if cfg.EnablePilotMode {
		auditLog := NewWhitelistAuditLog(log, whitelistAuditRepo)
		var err error
		wm, err = NewWhitelistManager(
			context.Background(),
			cfg.PilotWhitelist,
			"system:startup",
			"loaded from PAYOUT_PILOT_WHITELIST env var",
			auditLog,
		)
		if err != nil {
			if whitelistAuditRepo != nil {
				// Fail-closed: DB repo is wired, persistence is required.
				return nil, fmt.Errorf("payout worker: whitelist audit init: %w", err)
			}
			// Dev mode: warn and continue with in-process log only.
			log.Warn("Whitelist audit persistence failed (dev mode, continuing without DB)",
				zap.Error(err),
			)
			// Retry init without repo so the manager is always non-nil when pilot mode is on.
			devAuditLog := NewWhitelistAuditLog(log, nil)
			wm, _ = NewWhitelistManager(
				context.Background(),
				cfg.PilotWhitelist,
				"system:startup",
				"loaded from PAYOUT_PILOT_WHITELIST env var (dev fallback)",
				devAuditLog,
			)
		}
		log.Info("Pilot whitelist initialized",
			zap.Int("size", wm.Size()),
			zap.Bool("db_audit_enabled", whitelistAuditRepo != nil),
		)
	}

	return &PayoutWorker{
		db:                    db,
		withdrawRepo:          withdrawRepo,
		ledgerRepo:            ledgerRepo,
		gateway:               gateway,
		log:                   log,
		pollInterval:          cfg.PollInterval,
		batchSize:             cfg.BatchSize,
		retryBatchSize:        cfg.RetryBatchSize,
		stopCh:                make(chan struct{}),
		enablePilotMode:       cfg.EnablePilotMode,
		whitelistMgr:          wm,
		pilotBlockedThreshold: cfg.PilotBlockedThreshold,
		metrics:               NewPayoutMetrics(log),
	}, nil
}

// Start begins processing withdrawals in the background
func (w *PayoutWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("Payout worker already running")
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.stopCh = make(chan struct{})

	w.wg.Add(1)
	go w.run()

	w.log.Info("Payout worker started",
		zap.Duration("poll_interval", w.pollInterval),
		zap.Int("batch_size", w.batchSize),
		zap.Int("retry_batch_size", w.retryBatchSize),
	)
}

// Stop gracefully shuts down the worker
func (w *PayoutWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.log.Info("Stopping payout worker...")

	w.cancelFn()
	close(w.stopCh)

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.log.Info("Payout worker stopped gracefully")
	case <-time.After(10 * time.Second):
		w.log.Warn("Payout worker shutdown timeout")
	}

	w.running = false
}

// IsRunning returns true if the worker is currently running
func (w *PayoutWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// run is the main worker loop
func (w *PayoutWorker) run() {
	defer w.wg.Done()

	// Process immediately on start
	w.processPendingWithdrawals()
	w.processRetryableWithdrawals()

	for {
		select {
		case <-w.shutdownCtx.Done():
			w.log.Info("Worker shutdown requested")
			return

		case <-time.After(w.pollInterval):
			w.processPendingWithdrawals()
			w.processRetryableWithdrawals()

		case <-w.stopCh:
			return
		}
	}
}

// processPendingWithdrawals finds and submits withdrawals in PROCESSING status
func (w *PayoutWorker) processPendingWithdrawals() {
	ctx := context.Background()
	pollStart := time.Now()

	// Find withdrawals to process (with locking)
	withdrawals, err := w.findEligibleWithdrawals(ctx, w.batchSize)
	if err != nil {
		w.log.Error("Failed to find eligible withdrawals", zap.Error(err))
		return
	}

	if len(withdrawals) == 0 {
		return
	}

	w.log.Debug("Processing pending withdrawals", zap.Int("count", len(withdrawals)))

	// Process each withdrawal in its own transaction for isolation
	for _, withdrawal := range withdrawals {
		if err := w.submitWithdrawal(ctx, withdrawal); err != nil {
			w.log.Error("Failed to submit withdrawal",
				zap.String("withdrawal_id", withdrawal.ID.String()),
				zap.Error(err),
			)
		}
	}

	w.metrics.RecordPollCycle(time.Since(pollStart))
	w.metrics.EmitAggregateSnapshot()
}

// processRetryableWithdrawals finds and retries FAILED_RETRYABLE withdrawals
func (w *PayoutWorker) processRetryableWithdrawals() {
	ctx := context.Background()

	// Find retryable withdrawals (with backoff check)
	withdrawals, err := w.findRetryableWithdrawals(ctx, w.retryBatchSize)
	if err != nil {
		w.log.Error("Failed to find retryable withdrawals", zap.Error(err))
		return
	}

	if len(withdrawals) == 0 {
		return
	}

	w.log.Debug("Processing retryable withdrawals", zap.Int("count", len(withdrawals)))

	// Process each withdrawal in its own transaction for isolation
	for _, withdrawal := range withdrawals {
		if err := w.submitWithdrawal(ctx, withdrawal); err != nil {
			w.log.Error("Failed to retry withdrawal",
				zap.String("withdrawal_id", withdrawal.ID.String()),
				zap.Int("retry_count", withdrawal.RetryCount),
				zap.Error(err),
			)
		}
	}
}

// findEligibleWithdrawals returns PROCESSING withdrawals ready for submission.
// Sets external_reference_id atomically within the locked transaction.
// Wall-clock duration of the whole call (including FOR UPDATE lock wait) is
// recorded in metrics.RecordLockWait for operator visibility.
func (w *PayoutWorker) findEligibleWithdrawals(
	ctx context.Context,
	limit int,
) ([]*repository.Withdrawal, error) {
	var withdrawals []*repository.Withdrawal

	queryStart := time.Now()
	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		withdrawals, err = w.withdrawRepo.GetEligibleForSubmission(ctx, tx, limit)
		if err != nil {
			return err
		}

		// Ensure external_reference_id is set for each withdrawal
		// This happens within the same transaction that has the row locked
		for _, withdrawal := range withdrawals {
			if withdrawal.ExternalReferenceID == "" {
				extRef, err := w.withdrawRepo.EnsureExternalReference(ctx, tx, withdrawal.ID)
				if err != nil {
					return fmt.Errorf("ensure external reference for %s: %w", withdrawal.ID, err)
				}
				withdrawal.ExternalReferenceID = extRef
			}
		}

		return nil
	})
	w.metrics.RecordLockWait("GetEligibleForSubmission", time.Since(queryStart))

	return withdrawals, err
}

// findRetryableWithdrawals returns FAILED_RETRYABLE withdrawals ready for retry.
// Duration is recorded for lock-contention visibility.
func (w *PayoutWorker) findRetryableWithdrawals(
	ctx context.Context,
	limit int,
) ([]*repository.Withdrawal, error) {
	var withdrawals []*repository.Withdrawal

	queryStart := time.Now()
	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		withdrawals, err = w.withdrawRepo.GetRetryableWithdrawals(ctx, tx, limit)
		return err
	})
	w.metrics.RecordLockWait("GetRetryableWithdrawals", time.Since(queryStart))

	return withdrawals, err
}

// submitWithdrawal submits a single withdrawal to the gateway
func (w *PayoutWorker) submitWithdrawal(ctx context.Context, withdrawal *repository.Withdrawal) error {
	// PILOT MODE CHECK: If pilot mode is enabled, only whitelisted sellers can process payouts.
	if w.enablePilotMode && (w.whitelistMgr == nil || !w.whitelistMgr.IsWhitelisted(withdrawal.SellerID)) {
		// Seller not in pilot whitelist - mark as PILOT_BLOCKED for operational honesty.
		// This makes the blocking explicit rather than leaving withdrawals "stuck" in PROCESSING.
		if err := w.markPilotBlocked(ctx, withdrawal.ID); err != nil {
			w.log.Error("Failed to mark withdrawal as pilot blocked",
				zap.String("withdrawal_id", withdrawal.ID.String()),
				zap.String("seller_id", withdrawal.SellerID.String()),
				zap.Error(err),
			)
			return err
		}
		n := w.metrics.RecordPilotBlocked(withdrawal.SellerID)
		w.log.Info("Withdrawal marked as PILOT_BLOCKED - seller not in pilot whitelist",
			zap.String("withdrawal_id", withdrawal.ID.String()),
			zap.String("seller_id", withdrawal.SellerID.String()),
			zap.Int64("cumulative_pilot_blocked", n),
		)
		// Threshold alert: emit CRITICAL log if too many sellers are accumulating.
		if w.pilotBlockedThreshold > 0 {
			w.metrics.CheckPilotBlockedThreshold(w.pilotBlockedThreshold)
		}
		// Return nil so the worker continues processing other withdrawals.
		return nil
	}

	// The withdrawal already has external_reference_id set by GetEligibleForSubmission
	// which locks the row. Use it directly.
	externalRef := withdrawal.ExternalReferenceID
	if externalRef == "" {
		// This should not happen since GetEligibleForSubmission ensures it
		// But as a safety net, generate one
		externalRef = fmt.Sprintf("WD_%s_%d", withdrawal.ID.String(), time.Now().Unix())
	}

	// MONEY MODEL (PASS_18H, owner-confirmed): the gateway transfer amount is
	// the NET payout (requested amount minus the withdrawal fee) — the fee
	// is retained as platform revenue, never sent to the seller's bank.
	netPayoutAmount := withdrawal.Amount - withdrawal.FeeAmount

	// Build gateway request
	req := PayoutGatewayRequest{
		ExternalReferenceID: externalRef,
		Amount:              netPayoutAmount,
		Currency:            "IDR",
		BankName:            withdrawal.BankNameSnapshot,
		AccountNumber:       withdrawal.AccountNumberSnapshot,
		AccountHolder:       withdrawal.AccountHolderSnapshot,
		Metadata: map[string]string{
			"withdrawal_id": withdrawal.ID.String(),
			"seller_id":     withdrawal.SellerID.String(),
		},
	}

	// Submit to gateway
	w.metrics.RecordSubmission(withdrawal.ID)
	submitStart := time.Now()

	w.log.Info("Submitting payout to gateway",
		zap.String("withdrawal_id", withdrawal.ID.String()),
		zap.String("external_ref", externalRef),
		zap.Int64("requested_amount", withdrawal.Amount),
		zap.Int64("fee_amount", withdrawal.FeeAmount),
		zap.Int64("net_payout_amount", netPayoutAmount),
		zap.Int("retry_count", withdrawal.RetryCount),
		zap.Bool("pilot_mode", w.enablePilotMode),
	)

	resp, err := w.gateway.SubmitPayout(ctx, req)
	if err != nil {
		// Gateway call failed - mark as retryable
		w.metrics.RecordFailedRetryable(withdrawal.ID, withdrawal.RetryCount)
		w.log.Warn("Gateway submission error",
			zap.String("withdrawal_id", withdrawal.ID.String()),
			zap.Error(err),
		)
		return w.markSubmissionFailed(ctx, withdrawal.ID, ErrorTypeRetryable, err.Error())
	}

	latency := PayoutLatencyRecord{
		WithdrawalID:          withdrawal.ID,
		ProcessingToSubmitted: time.Since(submitStart),
		Timestamp:             time.Now(),
	}

	// Process gateway response
	if resp.IsSuccess() {
		w.metrics.RecordSettled(withdrawal.ID, latency)
		w.log.Info("Payout submitted successfully",
			zap.String("withdrawal_id", withdrawal.ID.String()),
			zap.String("gateway_ref", resp.GatewayReferenceID),
			zap.Duration("submit_latency_ms", latency.ProcessingToSubmitted),
		)
		return w.markSubmissionSuccess(ctx, withdrawal.ID, externalRef, resp.RawResponse)
	}

	// Gateway rejected the payout
	if resp.IsRetryable() {
		w.metrics.RecordFailedRetryable(withdrawal.ID, withdrawal.RetryCount)
		w.log.Warn("Payout failed with retryable error",
			zap.String("withdrawal_id", withdrawal.ID.String()),
			zap.String("reason", resp.Message),
		)
		return w.markSubmissionFailed(ctx, withdrawal.ID, ErrorTypeRetryable, resp.Message)
	}

	w.metrics.RecordFailedPermanent(withdrawal.ID)
	w.log.Error("Payout failed with permanent error",
		zap.String("withdrawal_id", withdrawal.ID.String()),
		zap.String("reason", resp.Message),
	)
	return w.markSubmissionFailed(ctx, withdrawal.ID, ErrorTypePermanent, resp.Message)
}

// markSubmissionSuccess marks a withdrawal as successfully submitted to the gateway
func (w *PayoutWorker) markSubmissionSuccess(
	ctx context.Context,
	withdrawalID uuid.UUID,
	externalRef string,
	gatewayResponse string,
) error {
	return w.db.WithTx(ctx, func(tx db.Tx) error {
		// Update for submission (PROCESSING -> SUBMITTED)
		if err := w.withdrawRepo.UpdateForSubmission(ctx, tx, withdrawalID, externalRef, gatewayResponse); err != nil {
			return fmt.Errorf("update for submission: %w", err)
		}

		// Reset retry count if this was a retry
		if err := w.withdrawRepo.ResetRetryCount(ctx, tx, withdrawalID); err != nil {
			return fmt.Errorf("reset retry count: %w", err)
		}

		w.log.Info("Withdrawal marked as submitted",
			zap.String("withdrawal_id", withdrawalID.String()),
			zap.String("external_ref", externalRef),
		)

		return nil
	})
}

// markSubmissionFailed marks a withdrawal as failed after gateway rejection
func (w *PayoutWorker) markSubmissionFailed(
	ctx context.Context,
	withdrawalID uuid.UUID,
	errorType PayoutErrorType,
	reason string,
) error {
	return w.db.WithTx(ctx, func(tx db.Tx) error {
		var newStatus repository.WithdrawalStatus

		if errorType == ErrorTypeRetryable {
			newStatus = repository.WithdrawalStatusFailedRetryable
			// Increment retry count for retryable failures
			if err := w.withdrawRepo.IncrementRetryCount(ctx, tx, withdrawalID); err != nil {
				return fmt.Errorf("increment retry count: %w", err)
			}
		} else {
			newStatus = repository.WithdrawalStatusFailedFinal
		}

		// Get current withdrawal to determine current status
		withdrawal, err := w.withdrawRepo.LockForUpdate(ctx, tx, withdrawalID)
		if err != nil {
			return fmt.Errorf("lock withdrawal: %w", err)
		}

		// Only update if not already in a terminal state
		if withdrawal.Status.IsFinal() {
			w.log.Debug("Withdrawal already in terminal state",
				zap.String("withdrawal_id", withdrawalID.String()),
				zap.String("status", string(withdrawal.Status)),
			)
			return nil
		}

		// WITHDRAWAL CONSISTENCY: permanent failure → return funds to seller.
		// Mirrors webhook_handler.handleFailedCallback ledger semantics.
		// Uses same idempotency key so late webhook duplicates are no-ops.
		//
		// MONEY MODEL (PASS_18H): the full reserved `amount` (not amount+fee)
		// is what was committed and is what returns to SELLER_PAYABLE — the
		// fee is only ever split off at successful settlement.
		if errorType == ErrorTypePermanent {
			amountMoney := money.New(withdrawal.Amount)
			idempotencyKey := fmt.Sprintf(gatewayRestoreKeyFmt, withdrawalID.String())

			withdrawalCommittedID, err := w.ledgerRepo.GetSystemAccountID(ctx, tx, finance.AccountWithdrawalCommitted)
			if err != nil {
				return fmt.Errorf("get withdrawal committed account: %w", err)
			}

			sellerPayableID, err := w.ledgerRepo.GetOrCreateUserAccount(ctx, tx, finance.AccountSellerPayable, withdrawal.SellerID)
			if err != nil {
				return fmt.Errorf("get seller payable account: %w", err)
			}

			entries := []ledgerintf.Entry{
				{AccountID: withdrawalCommittedID, Amount: amountMoney.Neg()},
				{AccountID: sellerPayableID, Amount: amountMoney},
			}
			if err := w.ledgerRepo.CreateTransaction(ctx, tx, idempotencyKey, "WITHDRAWAL_FAIL_RETURN", withdrawalID, nil, nil, entries); err != nil {
				return fmt.Errorf("return funds to seller: %w", err)
			}

			w.log.Info("Permanent failure: funds returned to seller",
				zap.String("withdrawal_id", withdrawalID.String()),
				zap.String("seller_id", withdrawal.SellerID.String()),
				zap.Int64("amount", withdrawal.Amount),
				zap.Int64("fee_amount", withdrawal.FeeAmount),
			)
		}

		if err := w.withdrawRepo.MarkFailed(ctx, tx, withdrawalID, newStatus, reason, ""); err != nil {
			return fmt.Errorf("mark failed: %w", err)
		}

		w.log.Info("Withdrawal marked as failed",
			zap.String("withdrawal_id", withdrawalID.String()),
			zap.String("status", string(newStatus)),
			zap.String("reason", reason),
			zap.Int("retry_count", withdrawal.RetryCount),
		)

		return nil
	})
}

// markPilotBlocked marks a withdrawal as blocked due to pilot mode restrictions.
// This provides operational honesty - withdrawals are explicitly marked as blocked
// rather than appearing "stuck" in PROCESSING status.
func (w *PayoutWorker) markPilotBlocked(
	ctx context.Context,
	withdrawalID uuid.UUID,
) error {
	return w.db.WithTx(ctx, func(tx db.Tx) error {
		if err := w.withdrawRepo.MarkPilotBlocked(ctx, tx, withdrawalID); err != nil {
			return fmt.Errorf("mark pilot blocked: %w", err)
		}
		return nil
	})
}

// ManualProcess triggers immediate processing of pending withdrawals.
// Useful for testing or manual intervention.
func (w *PayoutWorker) ManualProcess(ctx context.Context) error {
	withdrawals, err := w.findEligibleWithdrawals(ctx, w.batchSize)
	if err != nil {
		return fmt.Errorf("failed to find eligible withdrawals: %w", err)
	}

	if len(withdrawals) == 0 {
		w.log.Info("No pending withdrawals found")
		return nil
	}

	w.log.Info("Manual payout processing", zap.Int("count", len(withdrawals)))

	for _, withdrawal := range withdrawals {
		if err := w.submitWithdrawal(ctx, withdrawal); err != nil {
			w.log.Error("Failed to submit withdrawal",
				zap.String("withdrawal_id", withdrawal.ID.String()),
				zap.Error(err),
			)
		}
	}

	return nil
}
