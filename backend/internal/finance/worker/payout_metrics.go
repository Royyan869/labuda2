package worker

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// PayoutMetrics tracks payout timing and aggregate counters.
// All counters use atomic operations for lock-free reads; timing fields are
// protected by a mutex. Metrics are emitted as structured log events —
// no external monitoring integration required.
type PayoutMetrics struct {
	log *zap.Logger

	// Aggregate counters (atomic)
	totalSubmitted    int64
	totalSettled      int64
	totalFailedPerm   int64
	totalFailedRetry  int64
	totalPilotBlocked int64
	totalRetries      int64

	// Rate window
	mu          sync.Mutex
	windowStart time.Time
	windowCount int64

	// Last timing observations
	lastPollDuration      time.Duration
	lastLockWaitDuration  time.Duration
	lastReconcileDuration time.Duration
}

// NewPayoutMetrics creates a metrics tracker.
func NewPayoutMetrics(log *zap.Logger) *PayoutMetrics {
	if log == nil {
		log = zap.NewNop()
	}
	return &PayoutMetrics{log: log, windowStart: time.Now()}
}

// RecordSubmission records that a payout was attempted.
func (m *PayoutMetrics) RecordSubmission(withdrawalID uuid.UUID) {
	atomic.AddInt64(&m.totalSubmitted, 1)
	m.mu.Lock()
	m.windowCount++
	m.mu.Unlock()
	m.log.Debug("PAYOUT_METRIC_SUBMISSION",
		zap.String("withdrawal_id", withdrawalID.String()),
		zap.Int64("total_submitted", atomic.LoadInt64(&m.totalSubmitted)),
	)
}

// RecordSettled records a successful settlement with per-payout latency.
func (m *PayoutMetrics) RecordSettled(withdrawalID uuid.UUID, latency PayoutLatencyRecord) {
	n := atomic.AddInt64(&m.totalSettled, 1)
	m.log.Info("PAYOUT_METRIC_SETTLED",
		zap.String("withdrawal_id", withdrawalID.String()),
		zap.Duration("processing_to_submitted_ms", latency.ProcessingToSubmitted),
		zap.Duration("total_latency_ms", latency.TotalLatency),
		zap.Int64("total_settled", n),
	)
}

// RecordFailedPermanent records a permanent failure.
func (m *PayoutMetrics) RecordFailedPermanent(withdrawalID uuid.UUID) {
	n := atomic.AddInt64(&m.totalFailedPerm, 1)
	m.log.Warn("PAYOUT_METRIC_FAILED_PERMANENT",
		zap.String("withdrawal_id", withdrawalID.String()),
		zap.Int64("total_failed_permanent", n),
	)
}

// RecordFailedRetryable records a retryable failure.
func (m *PayoutMetrics) RecordFailedRetryable(withdrawalID uuid.UUID, retryCount int) {
	atomic.AddInt64(&m.totalFailedRetry, 1)
	atomic.AddInt64(&m.totalRetries, 1)
	m.log.Debug("PAYOUT_METRIC_FAILED_RETRYABLE",
		zap.String("withdrawal_id", withdrawalID.String()),
		zap.Int("retry_count", retryCount),
	)
}

// RecordPilotBlocked records a PILOT_BLOCKED event and returns the running total.
func (m *PayoutMetrics) RecordPilotBlocked(sellerID uuid.UUID) int64 {
	n := atomic.AddInt64(&m.totalPilotBlocked, 1)
	m.log.Info("PAYOUT_METRIC_PILOT_BLOCKED",
		zap.String("seller_id", sellerID.String()),
		zap.Int64("total_pilot_blocked", n),
	)
	return n
}

// RecordPollCycle records worker poll cycle duration and emits if above threshold.
func (m *PayoutMetrics) RecordPollCycle(duration time.Duration) {
	m.mu.Lock()
	m.lastPollDuration = duration
	m.mu.Unlock()
	m.log.Debug("PAYOUT_METRIC_POLL_CYCLE",
		zap.Duration("duration_ms", duration),
	)
}

// RecordLockWait records the wall-clock duration of a DB query that uses FOR UPDATE.
// Durations above 500ms are logged at WARN to surface lock contention.
func (m *PayoutMetrics) RecordLockWait(queryLabel string, duration time.Duration) {
	m.mu.Lock()
	m.lastLockWaitDuration = duration
	m.mu.Unlock()
	fields := []zap.Field{
		zap.String("query", queryLabel),
		zap.Duration("duration_ms", duration),
	}
	if duration > 500*time.Millisecond {
		m.log.Warn("PAYOUT_METRIC_LOCK_CONTENTION", fields...)
	} else {
		m.log.Debug("PAYOUT_METRIC_LOCK_WAIT", fields...)
	}
}

// RecordReconcileCycle records reconciliation cycle duration.
func (m *PayoutMetrics) RecordReconcileCycle(duration time.Duration) {
	m.mu.Lock()
	m.lastReconcileDuration = duration
	m.mu.Unlock()
	fields := []zap.Field{zap.Duration("duration_ms", duration)}
	if duration > 5*time.Second {
		m.log.Warn("PAYOUT_METRIC_RECONCILE_SLOW", fields...)
	} else {
		m.log.Debug("PAYOUT_METRIC_RECONCILE_CYCLE", fields...)
	}
}

// EmitAggregateSnapshot logs a point-in-time snapshot of all aggregate metrics.
// Call this periodically (e.g., each poll cycle) for throughput observability.
func (m *PayoutMetrics) EmitAggregateSnapshot() {
	submitted := atomic.LoadInt64(&m.totalSubmitted)
	settled := atomic.LoadInt64(&m.totalSettled)
	failedPerm := atomic.LoadInt64(&m.totalFailedPerm)
	failedRetry := atomic.LoadInt64(&m.totalFailedRetry)
	blocked := atomic.LoadInt64(&m.totalPilotBlocked)

	m.mu.Lock()
	windowDur := time.Since(m.windowStart)
	windowCount := m.windowCount
	lastPoll := m.lastPollDuration
	lastLock := m.lastLockWaitDuration
	lastReconcile := m.lastReconcileDuration
	m.mu.Unlock()

	var successRate, failRate, blockRate, payoutsPerMin float64
	if submitted > 0 {
		successRate = float64(settled) / float64(submitted) * 100
		failRate = float64(failedPerm+failedRetry) / float64(submitted) * 100
		blockRate = float64(blocked) / float64(submitted) * 100
	}
	if windowDur.Minutes() > 0 {
		payoutsPerMin = float64(windowCount) / windowDur.Minutes()
	}

	m.log.Info("PAYOUT_AGGREGATE_METRICS",
		zap.Int64("total_submitted", submitted),
		zap.Int64("total_settled", settled),
		zap.Int64("total_failed_permanent", failedPerm),
		zap.Int64("total_failed_retryable", failedRetry),
		zap.Int64("total_pilot_blocked", blocked),
		zap.Float64("settlement_success_rate_pct", successRate),
		zap.Float64("failure_rate_pct", failRate),
		zap.Float64("pilot_block_rate_pct", blockRate),
		zap.Float64("payouts_per_min", payoutsPerMin),
		zap.Duration("last_poll_duration_ms", lastPoll),
		zap.Duration("last_lock_wait_ms", lastLock),
		zap.Duration("last_reconcile_duration_ms", lastReconcile),
	)
}

// ============================================================================
// PILOT_BLOCKED ALERT
// ============================================================================

// PilotBlockedAlert is a threshold-triggered alert for PILOT_BLOCKED accumulation.
// Emitted as a CRITICAL log when blocked count exceeds the configured threshold.
type PilotBlockedAlert struct {
	Timestamp      time.Time `json:"timestamp"`
	BlockedCount   int64     `json:"blocked_count"`
	Threshold      int64     `json:"threshold"`
	Message        string    `json:"message"`
	OperatorAction string    `json:"operator_action"`
}

// CheckPilotBlockedThreshold emits a CRITICAL alert log if the PILOT_BLOCKED count
// meets or exceeds the threshold. Returns a non-nil alert when triggered.
// Does NOT auto-heal or change any state.
func (m *PayoutMetrics) CheckPilotBlockedThreshold(threshold int64) *PilotBlockedAlert {
	if threshold <= 0 {
		return nil
	}
	blocked := atomic.LoadInt64(&m.totalPilotBlocked)
	if blocked < threshold {
		return nil
	}
	alert := &PilotBlockedAlert{
		Timestamp:    time.Now(),
		BlockedCount: blocked,
		Threshold:    threshold,
		Message:      "PILOT_BLOCKED accumulation exceeds operator threshold",
		OperatorAction: "Add eligible sellers to PAYOUT_PILOT_WHITELIST or " +
			"set PAYOUT_ENABLE_PILOT_MODE=false to unblock",
	}
	m.log.Error("CRITICAL_PILOT_BLOCKED_THRESHOLD_EXCEEDED",
		zap.Int64("blocked_count", blocked),
		zap.Int64("threshold", threshold),
		zap.String("operator_action", alert.OperatorAction),
		zap.String("alert_level", "CRITICAL"),
	)
	return alert
}

// ============================================================================
// PER-PAYOUT LATENCY
// ============================================================================

// PayoutLatencyRecord holds per-payout latency phases.
type PayoutLatencyRecord struct {
	WithdrawalID          uuid.UUID     `json:"withdrawal_id"`
	ProcessingToSubmitted time.Duration `json:"processing_to_submitted"`
	TotalLatency          time.Duration `json:"total_latency"`
	Timestamp             time.Time     `json:"timestamp"`
}


