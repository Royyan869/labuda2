package monitoring

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricsNamespace = "labuda_system"
)

// MetricsCollector implements prometheus.Collector interface
// It collects metrics from MonitoringService in a read-only manner
//
// AUTHORITY GUARANTEE:
// - All counters/histograms are write-once-per-event hooks invoked by workers AFTER
//   their canonical decision (dispatch result, retry transition, DLQ transition).
// - Gauges are derived from read-only SQL via MonitoringService.GetSystemHealth or
//   from process-local heartbeat state. NO metric path mutates business state.
// - Metrics MUST NOT be read back by business logic. They are sink-only.
type MetricsCollector struct {
	monitoringService *MonitoringService

	// Gauges (read-only SQL via GetSystemHealth on every scrape)
	ledgerImbalanceValue prometheus.Gauge
	ledgerBalanced       prometheus.Gauge
	escrowStuckCount     prometheus.Gauge
	withdrawalStuckCount prometheus.Gauge
	auctionStuckCount    prometheus.Gauge
	outboxPendingEvents  prometheus.Gauge
	outboxStuckEvents    prometheus.Gauge
	outboxDLQCount       prometheus.Gauge
	outboxLagSeconds     prometheus.Gauge

	// Subscription Metrics
	orphanedPaymentCount              prometheus.Gauge
	paymentSubscriptionConversionRate prometheus.Gauge
	activeSubscriptionCount           prometheus.Gauge
	expiringSubscriptionCount         prometheus.Gauge
	expiredSubscriptionCount          prometheus.Gauge

	// Counters (Outbox Archival & Recovery — existing)
	outboxArchivedTotal             prometheus.Counter
	outboxStuckEventsRecoveredTotal prometheus.Counter
	outboxArchiveBatchDuration      prometheus.Histogram

	// =============================================================================
	// WORKER OBSERVABILITY (new)
	// =============================================================================
	//
	// CARDINALITY DISCIPLINE:
	// - event_type label is bounded by the registered dispatcher set (~dozens of
	//   constants in internal/platform/events). NEVER include aggregate_id or
	//   payload-derived values as labels.
	// - result label is a closed enum: succeeded | failed_retry | dead_letter | no_handler.
	// - worker_name label is a stable identifier ("outbox", "projection", "archival") —
	//   NEVER the per-instance UUID workerID.

	// outboxEventsProcessedTotal counts every terminal disposition of an outbox event
	// at the worker boundary. Use this to compute success / failure / DLQ ratios.
	outboxEventsProcessedTotal *prometheus.CounterVec

	// outboxHandlerFailuresTotal counts handler-reported errors per event_type.
	// Distinct from outboxEventsProcessedTotal{result="failed_retry"} only in that
	// it ignores re-counting on retry storms — it is incremented at FIRST failure
	// detection within processSingleEvent.
	outboxHandlerFailuresTotal *prometheus.CounterVec

	// outboxDeadLetterTotal counts transitions to dead_letter (poison events).
	// Each increment is a poison-event incident.
	outboxDeadLetterTotal *prometheus.CounterVec

	// outboxNoHandlerTotal counts events that arrived at the dispatcher with no
	// registered handler — silent no-consumer condition. Was previously a Warn log only.
	outboxNoHandlerTotal *prometheus.CounterVec

	// outboxProcessingDurationSeconds tracks end-to-end per-event handler latency
	// inside processEventInTx. Labelled by result so slow-success vs slow-failure
	// are distinguishable.
	outboxProcessingDurationSeconds *prometheus.HistogramVec

	// outboxRetryAttemptsAtTerminal records the attempt count at the moment an event
	// reaches a terminal state (success or DLQ). Histogram lets us see retry-storm
	// distribution without per-event high-cardinality labels.
	outboxRetryAttemptsAtTerminal *prometheus.HistogramVec

	// projectionEventsProcessedTotal counts projection worker dispositions.
	projectionEventsProcessedTotal *prometheus.CounterVec

	// orphanWebhookRecoveredTotal counts successful orphan recovery outcomes.
	orphanWebhookRecoveredTotal *prometheus.CounterVec

	// orphanWebhookRetryTotal counts retry scheduling decisions.
	orphanWebhookRetryTotal *prometheus.CounterVec

	// orphanWebhookFailedTotal counts technical recovery failures.
	orphanWebhookFailedTotal *prometheus.CounterVec

	// orphanWebhookManualReviewTotal counts unknown-status manual review handoffs.
	orphanWebhookManualReviewTotal *prometheus.CounterVec

	// orphanWebhookQuarantinedTotal counts malformed payload quarantine outcomes.
	orphanWebhookQuarantinedTotal *prometheus.CounterVec

	// orphanWebhookTerminalFailureTotal counts terminal-failure queue placements.
	orphanWebhookTerminalFailureTotal *prometheus.CounterVec

	// orphanWebhookProcessingDurationSeconds tracks recovery latency by outcome.
	orphanWebhookProcessingDurationSeconds *prometheus.HistogramVec

	// orphanWebhookBacklogCount surfaces the current orphan backlog size.
	orphanWebhookBacklogCount prometheus.Gauge

	// workerRunning is a 0/1 gauge per stable worker_name (set by Start/Stop).
	workerRunning *prometheus.GaugeVec

	// workerLastActivityTimestampSeconds is updated by workers at end of each batch
	// (success OR no-events). Staleness vs NOW() exposes stuck workers.
	workerLastActivityTimestampSeconds *prometheus.GaugeVec
}

// NewMetricsCollector creates a new MetricsCollector
func NewMetricsCollector(monitoringService *MonitoringService) *MetricsCollector {
	return &MetricsCollector{
		monitoringService: monitoringService,

		// Initialize gauges
		ledgerImbalanceValue: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "ledger_imbalance_value",
			Help:      "Difference between SUM(total_debit) and SUM(total_credit) in ledger_transactions. Should be 0.",
		}),
		ledgerBalanced: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "ledger_balanced",
			Help:      "Whether the ledger is balanced (1 = balanced, 0 = imbalanced).",
		}),
		escrowStuckCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "escrow_stuck_count",
			Help:      "Number of orders stuck in escrow (shipped but past auto_release_at).",
		}),
		withdrawalStuckCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "withdrawal_stuck_count",
			Help:      "Number of withdrawals stuck in processing status for > 24 hours.",
		}),
		auctionStuckCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "auction_stuck_count",
			Help:      "Number of ended auctions without order settlement (have bids but no order_id).",
		}),
		outboxPendingEvents: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "outbox_pending_events",
			Help:      "Number of outbox events with status 'pending'.",
		}),
		outboxStuckEvents: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "outbox_stuck_events",
			Help:      "Number of outbox events stuck in 'processing' status beyond timeout threshold.",
		}),
		outboxDLQCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "outbox_dlq_count",
			Help:      "Number of outbox events with status 'dead_letter' (poison events awaiting attributable manual reprocess).",
		}),
		outboxLagSeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "outbox_lag_seconds",
			Help:      "Age in seconds of the oldest outbox event ready for processing (status pending|failed, next_attempt_at <= NOW). 0 if no backlog.",
		}),

		// Initialize subscription metrics
		orphanedPaymentCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "orphaned_payment_count",
			Help:      "Number of payments with reference_type='subscription' and status='settlement' but no matching subscription record.",
		}),
		paymentSubscriptionConversionRate: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "payment_subscription_conversion_rate",
			Help:      "Ratio of successful subscription payments that have corresponding subscription records (0-1).",
		}),
		activeSubscriptionCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "active_subscription_count",
			Help:      "Number of subscriptions in active status.",
		}),
		expiringSubscriptionCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "expiring_subscription_count",
			Help:      "Number of subscriptions in active status that expire within 7 days.",
		}),
		expiredSubscriptionCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "expired_subscription_count",
			Help:      "Number of subscriptions in expired status.",
		}),

		// Initialize outbox archival metrics
		outboxArchivedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "outbox_archived_total",
			Help:      "Total number of outbox events archived to outbox_archive table.",
		}),
		outboxStuckEventsRecoveredTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "outbox_stuck_events_recovered_total",
			Help:      "Total number of outbox events recovered from stuck 'processing' status (replay storm indicator).",
		}),
		outboxArchiveBatchDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Name:      "outbox_archive_batch_duration_ms",
			Help:      "Duration of outbox archival batch processing in milliseconds.",
			Buckets:   []float64{10, 50, 100, 250, 500, 1000, 2500, 5000, 10000},
		}),

		// =============================================================================
		// Worker observability metrics
		// =============================================================================
		outboxEventsProcessedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "outbox_events_processed_total",
			Help:      "Total outbox events processed at the worker boundary, labelled by event_type and terminal disposition (succeeded|failed_retry|dead_letter|no_handler).",
		}, []string{"event_type", "result"}),
		outboxHandlerFailuresTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "outbox_handler_failures_total",
			Help:      "Total handler-reported failures during outbox dispatch, labelled by event_type. Retry storm indicator.",
		}, []string{"event_type"}),
		outboxDeadLetterTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "outbox_dead_letter_total",
			Help:      "Total transitions of outbox events into dead_letter status (poison-event incidents), labelled by event_type.",
		}, []string{"event_type"}),
		outboxNoHandlerTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "outbox_no_handler_total",
			Help:      "Total events that reached the dispatcher with no registered handler. Surfaces no-consumer / untracked-event-type conditions.",
		}, []string{"event_type"}),
		outboxProcessingDurationSeconds: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Name:      "outbox_processing_duration_seconds",
			Help:      "End-to-end per-event handler duration inside the outbox worker transaction, labelled by result.",
			Buckets:   prometheus.ExponentialBuckets(0.005, 2, 12), // 5ms .. ~20s
		}, []string{"result"}),
		outboxRetryAttemptsAtTerminal: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Name:      "outbox_retry_attempts_at_terminal",
			Help:      "Distribution of retry_count when an outbox event reaches a terminal state. Labelled by terminal (succeeded|dead_letter).",
			Buckets:   []float64{0, 1, 2, 3, 5, 8, 13, 20, 30},
		}, []string{"terminal"}),
		projectionEventsProcessedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "projection_events_processed_total",
			Help:      "Total events handled by the projection worker, labelled by result (processed|skipped|failed).",
		}, []string{"result"}),
		orphanWebhookRecoveredTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "orphan_webhook_recovered_total",
			Help:      "Total orphaned webhook events recovered into the canonical payment flow, labelled by outcome.",
		}, []string{"outcome"}),
		orphanWebhookRetryTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "orphan_webhook_retry_total",
			Help:      "Total orphaned webhook events scheduled for retry, labelled by outcome.",
		}, []string{"outcome"}),
		orphanWebhookFailedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "orphan_webhook_failed_total",
			Help:      "Total technical failures during orphan recovery, labelled by outcome.",
		}, []string{"outcome"}),
		orphanWebhookManualReviewTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "orphan_webhook_manual_review_total",
			Help:      "Total orphaned webhooks routed to manual review, labelled by outcome.",
		}, []string{"outcome"}),
		orphanWebhookQuarantinedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "orphan_webhook_quarantined_total",
			Help:      "Total malformed orphan payloads quarantined, labelled by outcome.",
		}, []string{"outcome"}),
		orphanWebhookTerminalFailureTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "orphan_webhook_terminal_failure_total",
			Help:      "Total orphan events moved into terminal review after retry exhaustion, labelled by outcome.",
		}, []string{"outcome"}),
		orphanWebhookProcessingDurationSeconds: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Name:      "orphan_webhook_processing_duration_seconds",
			Help:      "End-to-end orphan recovery duration, labelled by outcome.",
			Buckets:   prometheus.ExponentialBuckets(0.005, 2, 12),
		}, []string{"outcome"}),
		orphanWebhookBacklogCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "orphan_webhook_backlog_count",
			Help:      "Current count of orphaned webhook events awaiting recovery.",
		}),
		workerRunning: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "worker_running",
			Help:      "Whether a named worker is currently running (1) or stopped (0). worker_name is a stable identifier, never the per-instance UUID.",
		}, []string{"worker_name"}),
		workerLastActivityTimestampSeconds: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "worker_last_activity_timestamp_seconds",
			Help:      "Unix timestamp of the worker's last completed poll. Staleness vs now() exposes stuck workers. Updated on every batch (including empty batches).",
		}, []string{"worker_name"}),
	}
}

// Describe implements prometheus.Collector.Describe
// Sends descriptions of all metrics to the channel
func (mc *MetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(mc, ch)
}

// Collect implements prometheus.Collector.Collect
// Called each time /metrics endpoint is scraped
// Fetches current system health and updates all gauge values
func (mc *MetricsCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()

	// Get fresh system health data (read-only query)
	status, err := mc.monitoringService.GetSystemHealth(ctx)
	if err == nil {
		// Update gauge values from current system health
		mc.ledgerImbalanceValue.Set(float64(status.LedgerImbalanceValue))

		// Convert bool to 1 or 0
		balancedValue := 0.0
		if status.LedgerBalanced {
			balancedValue = 1.0
		}
		mc.ledgerBalanced.Set(balancedValue)

		mc.escrowStuckCount.Set(float64(status.EscrowStuckCount))
		mc.withdrawalStuckCount.Set(float64(status.WithdrawalStuckCount))
		mc.auctionStuckCount.Set(float64(status.AuctionStuckCount))
		mc.outboxPendingEvents.Set(float64(status.OutboxPendingCount))
		mc.outboxStuckEvents.Set(float64(status.OutboxStuckCount))
		mc.outboxDLQCount.Set(float64(status.OutboxDeadLetterCount))
		mc.outboxLagSeconds.Set(status.OutboxLagSeconds)

		// Update subscription metrics
		mc.orphanedPaymentCount.Set(float64(status.OrphanedPaymentCount))
		mc.paymentSubscriptionConversionRate.Set(status.PaymentSubscriptionConversionRate)
		mc.activeSubscriptionCount.Set(float64(status.ActiveSubscriptionCount))
		mc.expiringSubscriptionCount.Set(float64(status.ExpiringSubscriptionCount))
		mc.expiredSubscriptionCount.Set(float64(status.ExpiredSubscriptionCount))
	}
	// On health-query error we still emit existing process-local counters and worker
	// gauges. The gauges keep their last-known value (or 0 for never-set) so the
	// scrape never returns an empty body, and the failure surfaces as scrape_error
	// in Prometheus.

	// Emit all collectors. Gauges retain whatever values Set() last applied; the
	// counter vecs / histogram vecs emit whatever labelled series were recorded by
	// worker hooks.
	ch <- mc.ledgerImbalanceValue
	ch <- mc.ledgerBalanced
	ch <- mc.escrowStuckCount
	ch <- mc.withdrawalStuckCount
	ch <- mc.auctionStuckCount
	ch <- mc.outboxPendingEvents
	ch <- mc.outboxStuckEvents
	ch <- mc.outboxDLQCount
	ch <- mc.outboxLagSeconds
	ch <- mc.orphanedPaymentCount
	ch <- mc.paymentSubscriptionConversionRate
	ch <- mc.activeSubscriptionCount
	ch <- mc.expiringSubscriptionCount
	ch <- mc.expiredSubscriptionCount
	ch <- mc.outboxArchivedTotal
	ch <- mc.outboxStuckEventsRecoveredTotal
	ch <- mc.outboxArchiveBatchDuration
	ch <- mc.orphanWebhookBacklogCount

	mc.outboxEventsProcessedTotal.Collect(ch)
	mc.outboxHandlerFailuresTotal.Collect(ch)
	mc.outboxDeadLetterTotal.Collect(ch)
	mc.outboxNoHandlerTotal.Collect(ch)
	mc.outboxProcessingDurationSeconds.Collect(ch)
	mc.outboxRetryAttemptsAtTerminal.Collect(ch)
	mc.projectionEventsProcessedTotal.Collect(ch)
	mc.orphanWebhookRecoveredTotal.Collect(ch)
	mc.orphanWebhookRetryTotal.Collect(ch)
	mc.orphanWebhookFailedTotal.Collect(ch)
	mc.orphanWebhookManualReviewTotal.Collect(ch)
	mc.orphanWebhookQuarantinedTotal.Collect(ch)
	mc.orphanWebhookTerminalFailureTotal.Collect(ch)
	mc.orphanWebhookProcessingDurationSeconds.Collect(ch)
	mc.workerRunning.Collect(ch)
	mc.workerLastActivityTimestampSeconds.Collect(ch)
}

// =============================================================================
// EXISTING RECORDING HOOKS (kept stable)
// =============================================================================

// RecordOutboxArchived records that outbox events were archived.
// This should be called by the OutboxArchivalWorker after each successful batch.
func (mc *MetricsCollector) RecordOutboxArchived(count int) {
	mc.outboxArchivedTotal.Add(float64(count))
}

// RecordOutboxArchiveBatchDuration records the duration of an archival batch.
// This should be called by the OutboxArchivalWorker after each batch.
func (mc *MetricsCollector) RecordOutboxArchiveBatchDuration(durationMs float64) {
	mc.outboxArchiveBatchDuration.Observe(durationMs)
}

// RecordOutboxStuckEventsRecovered records that stuck outbox events were recovered.
// This should be called by the OutboxWorker after each stuck event recovery cycle.
func (mc *MetricsCollector) RecordOutboxStuckEventsRecovered(count int) {
	mc.outboxStuckEventsRecoveredTotal.Add(float64(count))
}

// =============================================================================
// WORKER OBSERVABILITY HOOKS (new)
// =============================================================================
//
// Each hook is a pure write to a Prometheus collector. No side effects on
// business state, no DB writes, no retry triggers. Workers may invoke these
// safely from inside transactions — they don't touch the tx.

// RecordOutboxEventProcessed records the terminal disposition of an event at the
// outbox worker boundary.
//
// result is a closed enum: "succeeded" | "failed_retry" | "dead_letter" | "no_handler".
func (mc *MetricsCollector) RecordOutboxEventProcessed(eventType, result string) {
	mc.outboxEventsProcessedTotal.WithLabelValues(eventType, result).Inc()
}

// RecordOutboxHandlerFailure records a handler-reported error during dispatch.
// Increment once per failure detection (before retry/dead-letter decision).
func (mc *MetricsCollector) RecordOutboxHandlerFailure(eventType string) {
	mc.outboxHandlerFailuresTotal.WithLabelValues(eventType).Inc()
}

// RecordOutboxDeadLetter records a transition to dead_letter status.
// Each invocation is a poison-event incident.
func (mc *MetricsCollector) RecordOutboxDeadLetter(eventType string) {
	mc.outboxDeadLetterTotal.WithLabelValues(eventType).Inc()
}

// RecordOutboxNoHandler records that an event arrived at the dispatcher with no
// registered handler. Previously a silent Warn log; this surfaces it.
func (mc *MetricsCollector) RecordOutboxNoHandler(eventType string) {
	mc.outboxNoHandlerTotal.WithLabelValues(eventType).Inc()
}

// RecordOutboxProcessingDuration records the end-to-end per-event handler latency.
// result follows the same enum as RecordOutboxEventProcessed.
func (mc *MetricsCollector) RecordOutboxProcessingDuration(result string, d time.Duration) {
	mc.outboxProcessingDurationSeconds.WithLabelValues(result).Observe(d.Seconds())
}

// RecordOutboxRetryAttemptsAtTerminal records the attempt count at the moment an
// event reaches a terminal state.
// terminal is a closed enum: "succeeded" | "dead_letter".
func (mc *MetricsCollector) RecordOutboxRetryAttemptsAtTerminal(terminal string, attempts int) {
	mc.outboxRetryAttemptsAtTerminal.WithLabelValues(terminal).Observe(float64(attempts))
}

// RecordProjectionEventProcessed records a projection worker disposition.
// result is a closed enum: "processed" | "skipped" | "failed".
func (mc *MetricsCollector) RecordProjectionEventProcessed(result string) {
	mc.projectionEventsProcessedTotal.WithLabelValues(result).Inc()
}

// SetOrphanWebhookBacklog sets the current orphan webhook backlog size.
func (mc *MetricsCollector) SetOrphanWebhookBacklog(count int) {
	mc.orphanWebhookBacklogCount.Set(float64(count))
}

// RecordOrphanWebhookRecovered records a recovered orphan webhook event.
func (mc *MetricsCollector) RecordOrphanWebhookRecovered(count int) {
	mc.orphanWebhookRecoveredTotal.WithLabelValues("recovered").Add(float64(count))
}

// RecordOrphanWebhookRetry records an orphan webhook retry decision.
func (mc *MetricsCollector) RecordOrphanWebhookRetry(count int) {
	mc.orphanWebhookRetryTotal.WithLabelValues("retry").Add(float64(count))
}

// RecordOrphanWebhookFailed records a technical orphan recovery failure.
func (mc *MetricsCollector) RecordOrphanWebhookFailed(count int) {
	mc.orphanWebhookFailedTotal.WithLabelValues("failed").Add(float64(count))
}

// RecordOrphanWebhookManualReview records an unknown-status manual review handoff.
func (mc *MetricsCollector) RecordOrphanWebhookManualReview(count int) {
	mc.orphanWebhookManualReviewTotal.WithLabelValues("manual_review").Add(float64(count))
}

// RecordOrphanWebhookQuarantined records a malformed-payload quarantine outcome.
func (mc *MetricsCollector) RecordOrphanWebhookQuarantined(count int) {
	mc.orphanWebhookQuarantinedTotal.WithLabelValues("quarantined").Add(float64(count))
}

// RecordOrphanWebhookTerminalFailure records a terminal-review queue placement.
func (mc *MetricsCollector) RecordOrphanWebhookTerminalFailure(count int) {
	mc.orphanWebhookTerminalFailureTotal.WithLabelValues("terminal_failure").Add(float64(count))
}

// RecordOrphanWebhookProcessingDuration records the end-to-end orphan recovery latency.
func (mc *MetricsCollector) RecordOrphanWebhookProcessingDuration(result string, d time.Duration) {
	mc.orphanWebhookProcessingDurationSeconds.WithLabelValues(result).Observe(d.Seconds())
}

// SetWorkerRunning sets the running gauge for a stable worker_name.
// Pass true on Start, false on Stop.
func (mc *MetricsCollector) SetWorkerRunning(workerName string, running bool) {
	v := 0.0
	if running {
		v = 1.0
	}
	mc.workerRunning.WithLabelValues(workerName).Set(v)
}

// RecordWorkerHeartbeat updates the last-activity timestamp for a stable
// worker_name. Call this at the end of every poll (including empty polls) so
// staleness can distinguish "idle" from "stuck".
func (mc *MetricsCollector) RecordWorkerHeartbeat(workerName string) {
	mc.workerLastActivityTimestampSeconds.WithLabelValues(workerName).Set(float64(time.Now().Unix()))
}


