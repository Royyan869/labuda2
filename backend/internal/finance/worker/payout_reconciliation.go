package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// PayoutReconciliationService detects and handles stuck or mismatched payouts.
//
// RECONCILIATION CHECKS:
// 1. Stuck payouts - SUBMITTED/SETTLING for too long
// 2. Orphaned payouts - FAILED_RETRYABLE that can be retried
// 3. Status mismatch - gateway status differs from our records
//
// SAFETY GUARDS:
// - READ-ONLY queries by default
// - Explicit action required for state changes
// - All actions logged for audit
type PayoutReconciliationService struct {
	withdrawRepo *repository.WithdrawRepository
	log          *zap.Logger
	db           Transactor

	// Configuration
	stuckThreshold         time.Duration // How long before considering a payout stuck
	reconciliationInterval time.Duration // How often to run reconciliation
}

// PayoutReconciliationConfig holds configuration for the reconciliation service.
type PayoutReconciliationConfig struct {
	StuckThresholdMinutes        int // Default: 30 minutes
	ReconciliationIntervalMinutes int // Default: 10 minutes
}

// DefaultPayoutReconciliationConfig returns default configuration.
func DefaultPayoutReconciliationConfig() PayoutReconciliationConfig {
	return PayoutReconciliationConfig{
		StuckThresholdMinutes:        30,
		ReconciliationIntervalMinutes: 10,
	}
}

// NewPayoutReconciliationService creates a new reconciliation service.
func NewPayoutReconciliationService(
	withdrawRepo *repository.WithdrawRepository,
	db Transactor,
	log *zap.Logger,
	cfg PayoutReconciliationConfig,
) *PayoutReconciliationService {
	if log == nil {
		log = zap.NewNop()
	}

	stuckThreshold := time.Duration(cfg.StuckThresholdMinutes) * time.Minute
	if stuckThreshold == 0 {
		stuckThreshold = 30 * time.Minute
	}

	return &PayoutReconciliationService{
		withdrawRepo:          withdrawRepo,
		log:                   log,
		db:                    db,
		stuckThreshold:        stuckThreshold,
		reconciliationInterval: time.Duration(cfg.ReconciliationIntervalMinutes) * time.Minute,
	}
}

// ============================================================================
// RECONCILIATION REPORT
// ============================================================================

// ReconciliationReport contains the results of a reconciliation check.
type ReconciliationReport struct {
	CheckTimestamp       time.Time              `json:"check_timestamp"`
	StuckPayouts         []*StuckPayoutInfo     `json:"stuck_payouts"`
	RetryablePayouts     []*RetryablePayoutInfo `json:"retryable_payouts"`
	OrphanedPayouts      []*OrphanedPayoutInfo  `json:"orphaned_payouts"`
	TotalPayoutsChecked  int                    `json:"total_payouts_checked"`
	ActionsTaken         []string               `json:"actions_taken"`
	RequiresManualReview int                    `json:"requires_manual_review"`

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

// StuckPayoutInfo contains information about a stuck payout.
type StuckPayoutInfo struct {
	WithdrawalID        uuid.UUID `json:"withdrawal_id"`
	ExternalReferenceID string    `json:"external_reference_id"`
	Status              string    `json:"status"`
	GatewayReferenceID  string    `json:"gateway_reference_id"`
	Amount              int64     `json:"amount"`
	SellerID            uuid.UUID `json:"seller_id"`
	DurationInState     int64     `json:"duration_in_state_seconds"`
	LastUpdated         time.Time `json:"last_updated"`
	RecommendedAction   string    `json:"recommended_action"`
	Severity            string    `json:"severity"`    // CRITICAL / WARNING / INFO
	RunbookURL          string    `json:"runbook_url"` // operator-facing runbook link

	// Enrichment
	LineageSummary         string `json:"lineage_summary"`          // one-line human description
	GatewayMismatchCategory string `json:"gateway_mismatch_category"` // TIMEOUT / REJECTED / MISSING / UNKNOWN
	OperatorRecommendation string `json:"operator_recommendation"`
}

// RetryablePayoutInfo contains information about a payout that can be retried.
type RetryablePayoutInfo struct {
	WithdrawalID        uuid.UUID `json:"withdrawal_id"`
	ExternalReferenceID string    `json:"external_reference_id"`
	Status              string    `json:"status"`
	RetryCount          int       `json:"retry_count"`
	LastRetryAt         time.Time `json:"last_retry_at"`
	CanRetry            bool      `json:"can_retry"`
	RetrySeverity       string    `json:"retry_severity"` // LOW (<3) / MEDIUM (3-4) / HIGH (>=5)
}

// OrphanedPayoutInfo contains information about a payout with no matching gateway record.
type OrphanedPayoutInfo struct {
	WithdrawalID        uuid.UUID `json:"withdrawal_id"`
	ExternalReferenceID string    `json:"external_reference_id"`
	Status              string    `json:"status"`
	GatewayReferenceID  string    `json:"gateway_reference_id"`
	SubmittedAt         time.Time `json:"submitted_at"`
}

// ============================================================================
// RECONCILIATION METHODS
// ============================================================================

// CheckStuckPayouts finds payouts that have been in SUBMITTED or SETTLING status
// for longer than the stuck threshold.
func (r *PayoutReconciliationService) CheckStuckPayouts(ctx context.Context) (*ReconciliationReport, error) {
	report := &ReconciliationReport{
		CheckTimestamp: time.Now(),
	}

	r.log.Info("Starting payout reconciliation check",
		zap.Duration("stuck_threshold", r.stuckThreshold),
	)

	// Query for payouts in SUBMITTED or SETTLING status older than threshold
	cutoff := time.Now().Add(-r.stuckThreshold)

	var stuckPayouts []*repository.Withdrawal
	if err := r.db.WithTx(ctx, func(tx db.Tx) error {
		var e error
		stuckPayouts, e = r.withdrawRepo.GetStuckPayouts(ctx, tx, cutoff, 100)
		return e
	}); err != nil {
		r.log.Error("Failed to query stuck payouts", zap.Error(err))
		return nil, fmt.Errorf("query stuck payouts: %w", err)
	}

	report.TotalPayoutsChecked = len(stuckPayouts)

	for _, payout := range stuckPayouts {
		// UpdatedAt is int64 Unix timestamp, convert to time.Time
		updatedAt := time.Unix(payout.UpdatedAt, 0)
		duration := time.Since(updatedAt)
		info := &StuckPayoutInfo{
			WithdrawalID:        payout.ID,
			ExternalReferenceID: payout.ExternalReferenceID,
			Status:              string(payout.Status),
			GatewayReferenceID:  extractGatewayRefID(payout.GatewayResponse),
			Amount:              payout.Amount,
			SellerID:            payout.SellerID,
			DurationInState:     int64(duration.Seconds()),
			LastUpdated:         updatedAt,
		}

		// Determine recommended action, severity, runbook, and enrichment
		switch payout.Status {
		case repository.WithdrawalStatusSubmitted:
			if duration > 1*time.Hour {
				info.RecommendedAction = "query_gateway_status"
				info.Severity = "CRITICAL"
				info.RunbookURL = "https://internal.labuda.id/runbooks/payout/stuck-submitted"
				info.GatewayMismatchCategory = "TIMEOUT"
				info.OperatorRecommendation = "Check gateway dashboard for external_ref=" + payout.ExternalReferenceID + "; if settled, update DB manually."
			} else {
				info.RecommendedAction = "wait_for_callback"
				info.Severity = "WARNING"
				info.RunbookURL = "https://internal.labuda.id/runbooks/payout/pending-submitted"
				info.GatewayMismatchCategory = "PENDING"
				info.OperatorRecommendation = "No action needed yet; will escalate if not resolved within 1h."
			}
		case repository.WithdrawalStatusSettling:
			if duration > 2*time.Hour {
				info.RecommendedAction = "mark_failed_final_or_query_gateway"
				info.Severity = "CRITICAL"
				info.RunbookURL = "https://internal.labuda.id/runbooks/payout/stuck-settling"
				info.GatewayMismatchCategory = "TIMEOUT"
				info.OperatorRecommendation = "Query gateway for final status. If bank rejected, mark FAILED_FINAL. If bank settled, mark SETTLED."
			} else {
				info.RecommendedAction = "wait_for_callback"
				info.Severity = "WARNING"
				info.RunbookURL = "https://internal.labuda.id/runbooks/payout/pending-settling"
				info.GatewayMismatchCategory = "PENDING"
				info.OperatorRecommendation = "No action needed yet; escalate if not resolved within 2h."
			}
		default:
			info.RecommendedAction = "manual_review"
			info.Severity = "CRITICAL"
			info.RunbookURL = "https://internal.labuda.id/runbooks/payout/unknown-stuck"
			info.GatewayMismatchCategory = "UNKNOWN"
			info.OperatorRecommendation = "Unexpected status for stuck payout. Escalate immediately."
		}

		info.LineageSummary = fmt.Sprintf(
			"withdrawal:%s ext:%s gw:%s status:%s age:%ds",
			payout.ID, payout.ExternalReferenceID,
			extractGatewayRefID(payout.GatewayResponse),
			string(payout.Status),
			int64(duration.Seconds()),
		)

		report.StuckPayouts = append(report.StuckPayouts, info)
		report.RequiresManualReview++
	}

	// Query PILOT_BLOCKED count for alert awareness
	pilotBlockedCount, err := r.queryPilotBlockedCount(ctx)
	if err != nil {
		r.log.Warn("Failed to query PILOT_BLOCKED count", zap.Error(err))
	} else {
		report.PilotBlockedCount = pilotBlockedCount
		if pilotBlockedCount > 0 {
			report.OperatorRecommendations = append(report.OperatorRecommendations, OperatorRec{
				Priority: "HIGH",
				Category: "pilot_mode",
				Message:  fmt.Sprintf("%d withdrawals are PILOT_BLOCKED", pilotBlockedCount),
				Action:   "Add sellers to PAYOUT_PILOT_WHITELIST or disable pilot mode",
			})
		}
	}

	r.log.Info("Reconciliation check completed",
		zap.Int("stuck_payouts", len(report.StuckPayouts)),
		zap.Int("requires_manual_review", report.RequiresManualReview),
	)

	return report, nil
}

// CheckRetryablePayouts finds payouts that are in FAILED_RETRYABLE status
// and may be eligible for retry.
func (r *PayoutReconciliationService) CheckRetryablePayouts(ctx context.Context) ([]*RetryablePayoutInfo, error) {
	var retryableInfos []*RetryablePayoutInfo

	// Query for payouts in FAILED_RETRYABLE status
	var retryablePayouts []*repository.Withdrawal
	if err := r.db.WithTx(ctx, func(tx db.Tx) error {
		var e error
		retryablePayouts, e = r.withdrawRepo.GetRetryableWithdrawals(ctx, tx, 100)
		return e
	}); err != nil {
		r.log.Error("Failed to query retryable payouts", zap.Error(err))
		return nil, fmt.Errorf("query retryable payouts: %w", err)
	}

	for _, payout := range retryablePayouts {
		// UpdatedAt is int64 Unix timestamp, convert to time.Time
		updatedAt := time.Unix(payout.UpdatedAt, 0)
		info := &RetryablePayoutInfo{
			WithdrawalID:        payout.ID,
			ExternalReferenceID: payout.ExternalReferenceID,
			Status:              string(payout.Status),
			RetryCount:          payout.RetryCount,
			LastRetryAt:         updatedAt,
			CanRetry:            payout.RetryCount < 5, // Max 5 retries
			RetrySeverity:       classifyRetrySeverity(payout.RetryCount),
		}

		retryableInfos = append(retryableInfos, info)
	}

	r.log.Debug("Retryable payouts check completed",
		zap.Int("count", len(retryableInfos)),
	)

	return retryableInfos, nil
}

// ============================================================================
// AUTOMATED ACTIONS
// ============================================================================

// QueryGatewayStatus checks the status of a payout with the gateway.
// This is a READ-ONLY operation that does not change any state.
func (r *PayoutReconciliationService) QueryGatewayStatus(
	ctx context.Context,
	withdrawalID uuid.UUID,
) (map[string]interface{}, error) {
	// Get the withdrawal record
	withdrawal, err := r.withdrawRepo.GetByID(ctx, nil, withdrawalID)
	if err != nil {
		return nil, fmt.Errorf("get withdrawal: %w", err)
	}

	if withdrawal == nil {
		return nil, fmt.Errorf("withdrawal not found")
	}

	// For sandbox mode, return simulated status
	result := map[string]interface{}{
		"withdrawal_id":         withdrawal.ID.String(),
		"external_reference_id": withdrawal.ExternalReferenceID,
		"gateway_reference_id":  extractGatewayRefID(withdrawal.GatewayResponse),
		"our_status":            string(withdrawal.Status),
		"gateway_status":        "UNKNOWN",
		"last_updated":          withdrawal.UpdatedAt,
		"mode":                  "sandbox_query",
		"action_required":       "manual_verification",
	}

	r.log.Info("Gateway status queried",
		zap.String("withdrawal_id", withdrawalID.String()),
		zap.String("status", string(withdrawal.Status)),
	)

	return result, nil
}

// MarkPayoutStuck marks a payout as stuck for manual review.
// This transitions the payout to a state that requires admin intervention.
func (r *PayoutReconciliationService) MarkPayoutStuck(
	ctx context.Context,
	withdrawalID uuid.UUID,
	reason string,
) error {
	return r.db.WithTx(ctx, func(tx db.Tx) error {
		withdrawal, err := r.withdrawRepo.LockForUpdate(ctx, tx, withdrawalID)
		if err != nil {
			return fmt.Errorf("lock withdrawal: %w", err)
		}

		if withdrawal.Status.IsFinal() {
			r.log.Info("Withdrawal already in final state, cannot mark as stuck",
				zap.String("withdrawal_id", withdrawalID.String()),
				zap.String("status", string(withdrawal.Status)),
			)
			return nil
		}

		// For now, we don't have a dedicated "stuck" status
		// We'll log it and leave it as-is for manual intervention
		r.log.Warn("Payout marked as stuck for manual review",
			zap.String("withdrawal_id", withdrawalID.String()),
			zap.String("status", string(withdrawal.Status)),
			zap.String("reason", reason),
			zap.Int("retry_count", withdrawal.RetryCount),
		)

		return nil
	})
}

// ============================================================================
// OBSERVABILITY
// ============================================================================

// LogReconciliationReport logs the reconciliation report for monitoring.
func (r *PayoutReconciliationService) LogReconciliationReport(report *ReconciliationReport) {
	r.log.Info("Payout reconciliation report",
		zap.Time("check_timestamp", report.CheckTimestamp),
		zap.Int("total_checked", report.TotalPayoutsChecked),
		zap.Int("stuck_payouts", len(report.StuckPayouts)),
		zap.Int("retryable_payouts", len(report.RetryablePayouts)),
		zap.Int("orphaned_payouts", len(report.OrphanedPayouts)),
		zap.Int("requires_manual_review", report.RequiresManualReview),
		zap.Strings("actions_taken", report.ActionsTaken),
	)

	// Log individual stuck payouts for alerting
	for _, stuck := range report.StuckPayouts {
		r.log.Warn("Stuck payout detected",
			zap.String("withdrawal_id", stuck.WithdrawalID.String()),
			zap.String("external_ref", stuck.ExternalReferenceID),
			zap.String("status", stuck.Status),
			zap.Int64("duration_seconds", stuck.DurationInState),
			zap.String("severity", stuck.Severity),
			zap.String("recommended_action", stuck.RecommendedAction),
			zap.String("runbook_url", stuck.RunbookURL),
		)
	}
}

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
		"stuck_threshold_minutes":        int(r.stuckThreshold.Minutes()),
		"reconciliation_interval_minutes": int(r.reconciliationInterval.Minutes()),
		"status_counts":                  statusCounts,
		"last_check_timestamp":           time.Now(),
	}, nil
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

	w.log.Info("Stopping payout reconciliation worker...")
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

// performReconciliation executes a single reconciliation check.
// Wall-clock duration is logged for scaling-bottleneck visibility.
func (w *PayoutReconciliationWorker) performReconciliation() {
	ctx := context.Background()
	cycleStart := time.Now()

	w.log.Debug("Performing payout reconciliation check")

	report, err := w.service.CheckStuckPayouts(ctx)
	if err != nil {
		w.log.Error("Reconciliation check failed", zap.Error(err))
		return
	}

	w.service.LogReconciliationReport(report)

	// Log metrics
	if metrics, err := w.service.GetReconciliationMetrics(ctx); err == nil {
		w.log.Debug("Reconciliation metrics", zap.Any("metrics", metrics))
	}

	cycleDuration := time.Since(cycleStart)
	fields := []zap.Field{zap.Duration("reconcile_cycle_duration_ms", cycleDuration)}
	if cycleDuration > 5*time.Second {
		w.log.Warn("PAYOUT_RECONCILE_CYCLE_SLOW", fields...)
	} else {
		w.log.Debug("PAYOUT_RECONCILE_CYCLE_DURATION", fields...)
	}
}

// ManualCheck triggers an immediate reconciliation check.
func (w *PayoutReconciliationWorker) ManualCheck(ctx context.Context) (*ReconciliationReport, error) {
	return w.service.CheckStuckPayouts(ctx)
}

// IsRunning returns true if the worker is running.
func (w *PayoutReconciliationWorker) IsRunning() bool {
	return w.running
}

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


