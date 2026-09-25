package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/labuda/backend/internal/finance/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// IrisTimingBuffer is the minimum time after gateway submission before querying status.
// Midtrans Iris requires a 10-minute buffer after payout creation.
const IrisTimingBuffer = 10 * time.Minute

// MaxReconciliationBatchSize is the maximum number of payouts to reconcile per cycle.
const MaxReconciliationBatchSize = 50

// PayoutReconciliationService detects and handles stuck or mismatched payouts.
//
// PAYOUT-03: This service now performs real automated reconciliation by:
// 1. Finding eligible payouts (SUBMITTED/SETTLING, age >= 10 minutes)
// 2. Querying Midtrans Iris for actual gateway status
// 3. Mapping gateway status to WebhookCallback
// 4. Calling WebhookHandler.HandleCallback() for canonical transition
//
// SAFETY GUARDS:
// - Observe-only: never calls CreatePayout/SubmitPayout
// - 10-minute buffer: respects Iris timing requirement
// - Canonical transition: uses same path as webhook handler
// - Idempotent: duplicate reconciliations produce no duplicate effects
// - Unknown status: no financial transition, logged for investigation
type PayoutReconciliationService struct {
	withdrawRepo   *repository.WithdrawRepository
	ledgerRepo     *LedgerRepositoryWrapper
	log            *zap.Logger
	db             Transactor
	gateway        PayoutGateway
	webhookHandler *WebhookHandler

	// Configuration
	reconciliationInterval time.Duration // How often to run reconciliation
}

// LedgerRepositoryWrapper wraps the repository ledger repo for dependency injection.
type LedgerRepositoryWrapper struct {
	*repository.LedgerRepository
}

// PayoutReconciliationConfig holds configuration for the reconciliation service.
type PayoutReconciliationConfig struct {
	ReconciliationIntervalMinutes int // Default: 10 minutes
}

// DefaultPayoutReconciliationConfig returns default configuration.
func DefaultPayoutReconciliationConfig() PayoutReconciliationConfig {
	return PayoutReconciliationConfig{
		ReconciliationIntervalMinutes: 10,
	}
}

// NewPayoutReconciliationService creates a new reconciliation service.
// PAYOUT-03: Now requires gateway and webhook handler for real reconciliation.
func NewPayoutReconciliationService(
	withdrawRepo *repository.WithdrawRepository,
	db Transactor,
	log *zap.Logger,
	cfg PayoutReconciliationConfig,
	gateway PayoutGateway,
	webhookHandler *WebhookHandler,
) *PayoutReconciliationService {
	if log == nil {
		log = zap.NewNop()
	}

	reconciliationInterval := time.Duration(cfg.ReconciliationIntervalMinutes) * time.Minute
	if reconciliationInterval == 0 {
		reconciliationInterval = 10 * time.Minute
	}

	return &PayoutReconciliationService{
		withdrawRepo:          withdrawRepo,
		log:                   log,
		db:                    db,
		gateway:               gateway,
		webhookHandler:        webhookHandler,
		reconciliationInterval: reconciliationInterval,
	}
}

// ============================================================================
// RECONCILIATION REPORT
// ============================================================================

// ReconciliationReport contains the results of a reconciliation check.
type ReconciliationReport struct {
	CheckTimestamp        time.Time              `json:"check_timestamp"`
	CandidatesFound       int                    `json:"candidates_found"`
	QueriesAttempted      int                    `json:"queries_attempted"`
	TransitionsApplied    int                    `json:"transitions_applied"`
	QueryFailures         int                    `json:"query_failures"`
	UnknownStatuses       int                    `json:"unknown_statuses"`
	TerminalSkipped       int                    `json:"terminal_skipped"`
	MissingReferenceNo    int                    `json:"missing_reference_no"`
	TooYoung              int                    `json:"too_young"`
	ActionsTaken          []string               `json:"actions_taken"`

	// Enrichment fields
	PilotBlockedCount       int                  `json:"pilot_blocked_count"`
	PilotBlockedAlert       *PilotBlockedAlert   `json:"pilot_blocked_alert,omitempty"`
	OperatorRecommendations []OperatorRec        `json:"operator_recommendations,omitempty"`
}

// OperatorRec is an actionable recommendation for an operator.
type OperatorRec struct {
	Priority string `json:"priority"` // "CRITICAL", "HIGH", "MEDIUM", "LOW"
	Category string `json:"category"`
	Message  string `json:"message"`
	Action   string `json:"action"`
}

// ============================================================================
// RECONCILIATION CANDIDATE SELECTION
// ============================================================================

// GetReconciliationCandidates finds payouts eligible for reconciliation.
// PAYOUT-03: Eligibility requires:
//   - status IN ('SUBMITTED', 'SETTLING')
//   - submitted_at > 0 (gateway submission timestamp known)
//   - gateway_reference_no != '' (can query gateway)
//   - submitted_at <= now - 10 minutes (Iris timing requirement)
func (r *PayoutReconciliationService) GetReconciliationCandidates(ctx context.Context) ([]*repository.Withdrawal, error) {
	// Calculate the maximum submitted_at for eligibility (now - 10 minutes)
	maxSubmittedAt := time.Now().Add(-IrisTimingBuffer).Unix()

	var candidates []*repository.Withdrawal
	if err := r.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		candidates, err = r.withdrawRepo.GetReconciliationCandidates(ctx, tx, maxSubmittedAt, MaxReconciliationBatchSize)
		return err
	}); err != nil {
		return nil, fmt.Errorf("query reconciliation candidates: %w", err)
	}

	return candidates, nil
}

// ============================================================================
// GATEWAY STATUS QUERY
// ============================================================================

// QueryGatewayStatus queries the Midtrans Iris gateway for payout status.
// PAYOUT-03: This is observe-only - never submits or resubmits payouts.
func (r *PayoutReconciliationService) QueryGatewayStatus(
	ctx context.Context,
	referenceNo string,
) (*PayoutStatusCheck, error) {
	if r.gateway == nil {
		return nil, fmt.Errorf("gateway not configured")
	}

	status, err := r.gateway.GetPayoutStatus(ctx, referenceNo)
	if err != nil {
		return nil, fmt.Errorf("query gateway status: %w", err)
	}

	return status, nil
}

// ============================================================================
// STATUS MAPPING
// ============================================================================

// MapGatewayStatusToCallback maps Iris gateway status to WebhookCallback.
// PAYOUT-03: Correct mapping based on Iris documentation:
//   - queued → WebhookStatusPending (non-terminal)
//   - processed → WebhookStatusPending (non-terminal, bank acknowledged)
//   - completed → WebhookStatusSuccess (terminal success)
//   - failed → WebhookStatusFailed (terminal failure)
//   - unknown → WebhookStatusUnknown (no transition)
func MapGatewayStatusToCallback(
	externalReferenceID string,
	gatewayReferenceNo string,
	gatewayStatus string,
	errorMessage string,
) WebhookCallback {
	var status WebhookStatus
	switch gatewayStatus {
	case "completed":
		status = WebhookStatusSuccess // Terminal success
	case "processed":
		status = WebhookStatusPending // Non-terminal, bank acknowledged
	case "queued":
		status = WebhookStatusPending // Non-terminal, waiting
	case "failed":
		status = WebhookStatusFailed // Terminal failure
	default:
		status = WebhookStatusUnknown // Unknown, do not transition
	}

	return WebhookCallback{
		ExternalReferenceID: externalReferenceID,
		GatewayReferenceID:  gatewayReferenceNo,
		Status:              status,
		Message:             errorMessage,
		Timestamp:           time.Now().Unix(),
		RawPayload:          "", // Empty for synthetic callbacks
	}
}

// ============================================================================
// CANONICAL TRANSITION
// ============================================================================

// ReconcilePayout queries gateway status and applies canonical transition.
// PAYOUT-03: This is the core reconciliation logic:
// 1. Query Midtrans Iris for actual status
// 2. Map to WebhookCallback
// 3. Call WebhookHandler.HandleCallback() for canonical transition
//
// SAFETY:
// - Gateway query occurs OUTSIDE database transaction
// - Canonical transition occurs WITHIN database transaction
// - Never calls CreatePayout/SubmitPayout
// - Unknown status produces no financial transition
func (r *PayoutReconciliationService) ReconcilePayout(
	ctx context.Context,
	withdrawal *repository.Withdrawal,
) (string, error) {
	// Step 1: Query gateway (outside DB transaction)
	r.log.Info("Querying gateway for payout status",
		zap.String("withdrawal_id", withdrawal.ID.String()),
		zap.String("external_ref", withdrawal.ExternalReferenceID),
		zap.String("gateway_ref_no", withdrawal.GatewayReferenceNo),
		zap.String("current_status", string(withdrawal.Status)),
	)

	status, err := r.QueryGatewayStatus(ctx, withdrawal.GatewayReferenceNo)
	if err != nil {
		r.log.Warn("Gateway query failed",
			zap.String("withdrawal_id", withdrawal.ID.String()),
			zap.Error(err),
		)
		return "query_failed", fmt.Errorf("gateway query: %w", err)
	}

	// Step 2: Map to WebhookCallback
	callback := MapGatewayStatusToCallback(
		withdrawal.ExternalReferenceID,
		withdrawal.GatewayReferenceNo,
		status.Status,
		status.RawResponse,
	)

	r.log.Info("Gateway status observed",
		zap.String("withdrawal_id", withdrawal.ID.String()),
		zap.String("gateway_status", status.Status),
		zap.String("mapped_status", string(callback.Status)),
	)

	// Step 3: Handle unknown status (no transition)
	if callback.Status == WebhookStatusUnknown {
		r.log.Warn("Unknown gateway status - no transition applied",
			zap.String("withdrawal_id", withdrawal.ID.String()),
			zap.String("gateway_status", status.Status),
		)
		return "unknown_status", nil
	}

	// Step 4: Apply canonical transition (within DB transaction)
	err = r.db.WithTx(ctx, func(tx db.Tx) error {
		return r.webhookHandler.HandleCallback(ctx, tx, callback)
	})

	if err != nil {
		// ErrDuplicateCallback is not an error - it means already processed
		if err == ErrDuplicateCallback {
			r.log.Info("Payout already in terminal state, skipping",
				zap.String("withdrawal_id", withdrawal.ID.String()),
			)
			return "terminal_skipped", nil
		}
		return "transition_failed", fmt.Errorf("canonical transition: %w", err)
	}

	return "transition_applied", nil
}

// ============================================================================
// RECONCILIATION WORKER
// ============================================================================

// PayoutReconciliationWorker runs periodic reconciliation checks.
type PayoutReconciliationWorker struct {
	service *PayoutReconciliationService
	log     *zap.Logger
	running bool
	stopCh  chan struct{}
}

// NewPayoutReconciliationWorker creates a new reconciliation worker.
func NewPayoutReconciliationWorker(
	service *PayoutReconciliationService,
	log *zap.Logger,
) *PayoutReconciliationWorker {
	if log == nil {
		log = zap.NewNop()
	}
	return &PayoutReconciliationWorker{
		service: service,
		log:     log,
		stopCh:  make(chan struct{}),
	}
}

// Start begins the reconciliation worker loop.
func (w *PayoutReconciliationWorker) Start() {
	if w.running {
		w.log.Warn("Reconciliation worker already running")
		return
	}

	w.running = true
	w.log.Info("Starting payout reconciliation worker",
		zap.Duration("interval", w.service.reconciliationInterval),
	)

	go w.run()
}

// Stop stops the reconciliation worker.
func (w *PayoutReconciliationWorker) Stop() {
	if !w.running {
		return
	}

	w.log.Info("Stopping reconciliation worker...")
	close(w.stopCh)
	w.running = false
}

// run is the main worker loop.
func (w *PayoutReconciliationWorker) run() {
	ticker := time.NewTicker(w.service.reconciliationInterval)
	defer ticker.Stop()

	// Run immediately on start
	w.performReconciliation()

	for {
		select {
		case <-ticker.C:
			w.performReconciliation()
		case <-w.stopCh:
			w.log.Info("Reconciliation worker stopped")
			return
		}
	}
}

// performReconciliation executes a single reconciliation cycle.
// PAYOUT-03: Now performs real gateway queries and canonical transitions.
func (w *PayoutReconciliationWorker) performReconciliation() {
	ctx := context.Background()
	cycleStart := time.Now()

	w.log.Debug("Starting reconciliation cycle")

	// Step 1: Find eligible candidates
	candidates, err := w.service.GetReconciliationCandidates(ctx)
	if err != nil {
		w.log.Error("Failed to query reconciliation candidates", zap.Error(err))
		return
	}

	if len(candidates) == 0 {
		w.log.Debug("No reconciliation candidates found")
		return
	}

	w.log.Info("Reconciliation candidates found", zap.Int("count", len(candidates)))

	// Step 2: Reconcile each candidate
	report := &ReconciliationReport{
		CheckTimestamp:  time.Now(),
		CandidatesFound: len(candidates),
	}

	for _, withdrawal := range candidates {
		action, err := w.service.ReconcilePayout(ctx, withdrawal)
		if err != nil {
			w.log.Error("Reconciliation failed for payout",
				zap.String("withdrawal_id", withdrawal.ID.String()),
				zap.Error(err),
			)
			report.QueryFailures++
			continue
		}

		report.QueriesAttempted++
		switch action {
		case "transition_applied":
			report.TransitionsApplied++
			report.ActionsTaken = append(report.ActionsTaken,
				fmt.Sprintf("transitioned %s", withdrawal.ID.String()))
		case "terminal_skipped":
			report.TerminalSkipped++
		case "unknown_status":
			report.UnknownStatuses++
			report.ActionsTaken = append(report.ActionsTaken,
				fmt.Sprintf("unknown status for %s", withdrawal.ID.String()))
		case "query_failed":
			// Already counted above
		}
	}

	// Log report
	w.log.Info("Reconciliation cycle completed",
		zap.Int("candidates", report.CandidatesFound),
		zap.Int("queries_attempted", report.QueriesAttempted),
		zap.Int("transitions_applied", report.TransitionsApplied),
		zap.Int("query_failures", report.QueryFailures),
		zap.Int("unknown_statuses", report.UnknownStatuses),
		zap.Duration("cycle_duration", time.Since(cycleStart)),
	)
}

// ManualCheck triggers an immediate reconciliation check.
func (w *PayoutReconciliationWorker) ManualCheck(ctx context.Context) (*ReconciliationReport, error) {
	// Find candidates
	candidates, err := w.service.GetReconciliationCandidates(ctx)
	if err != nil {
		return nil, err
	}

	report := &ReconciliationReport{
		CheckTimestamp:  time.Now(),
		CandidatesFound: len(candidates),
	}

	// Reconcile each candidate
	for _, withdrawal := range candidates {
		action, err := w.service.ReconcilePayout(ctx, withdrawal)
		if err != nil {
			report.QueryFailures++
			continue
		}

		report.QueriesAttempted++
		switch action {
		case "transition_applied":
			report.TransitionsApplied++
		case "terminal_skipped":
			report.TerminalSkipped++
		case "unknown_status":
			report.UnknownStatuses++
		}
	}

	return report, nil
}

// IsRunning returns true if the worker is running.
func (w *PayoutReconciliationWorker) IsRunning() bool {
	return w.running
}

// ============================================================================
// OBSERVABILITY
// ============================================================================

// GetReconciliationMetrics returns metrics for monitoring.
func (r *PayoutReconciliationService) GetReconciliationMetrics(ctx context.Context) (map[string]interface{}, error) {
	// Get counts by status
	var statusCounts []repository.StatusCount
	if err := r.db.WithTx(ctx, func(tx db.Tx) error {
		var e error
		statusCounts, e = r.withdrawRepo.GetStatusCounts(ctx, tx)
		return e
	}); err != nil {
		return nil, fmt.Errorf("get status counts: %w", err)
	}

	return map[string]interface{}{
		"reconciliation_interval_minutes": int(r.reconciliationInterval.Minutes()),
		"status_counts":                   statusCounts,
		"last_check_timestamp":            time.Now(),
	}, nil
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// extractGatewayRefID parses the raw Midtrans gateway response JSON and returns
// the gateway-assigned payout reference ID ("id" field). Returns empty string
// if the response is absent or the field is missing — never conflates with
// our own ExternalReferenceID.
func extractGatewayRefID(rawJSON string) string {
	if rawJSON == "" {
		return ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(rawJSON), &m); err != nil {
		return ""
	}
	if id, ok := m["id"].(string); ok {
		return id
	}
	return ""
}

// queryPilotBlockedCount returns the number of withdrawals currently in PILOT_BLOCKED status.
func (r *PayoutReconciliationService) queryPilotBlockedCount(ctx context.Context) (int, error) {
	var count int
	if err := r.db.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT COUNT(*) FROM withdrawals WHERE status = 'PILOT_BLOCKED'`,
		).Scan(&count)
	}); err != nil {
		return 0, fmt.Errorf("query pilot_blocked count: %w", err)
	}
	return count, nil
}

// classifyRetrySeverity returns LOW / MEDIUM / HIGH based on retry count.
// HIGH means the payout is approaching the max retry limit and needs attention.
func classifyRetrySeverity(retryCount int) string {
	switch {
	case retryCount >= 5:
		return "HIGH"
	case retryCount >= 3:
		return "MEDIUM"
	default:
		return "LOW"
	}
}
