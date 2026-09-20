// Package worker.
//
// worker_metrics.go defines the narrow recorder interfaces the outbox and
// projection workers depend on. These interfaces are deliberately small so
// that workers compile without a metrics collector and so test code can pass
// nil or a no-op recorder.
//
// AUTHORITY GUARANTEE:
// The recorder is sink-only. No method on these interfaces mutates DB state,
// reads back business truth, or affects retry/dead-letter decisions. Workers
// call these hooks AFTER their canonical decision has already been made.
package worker

import "time"

// OutboxMetricsRecorder is the subset of metric hooks consumed by OutboxWorker.
//
// It intentionally mirrors monitoring.MetricsCollector's signatures so the
// collector satisfies this interface without an adapter.
type OutboxMetricsRecorder interface {
	// RecordOutboxEventProcessed records the terminal disposition of an event.
	// result ∈ {"succeeded", "failed_retry", "dead_letter", "no_handler"}.
	RecordOutboxEventProcessed(eventType, result string)

	// RecordOutboxHandlerFailure increments once per handler-reported error,
	// independent of whether retry or dead-letter follows.
	RecordOutboxHandlerFailure(eventType string)

	// RecordOutboxDeadLetter increments on each transition into dead_letter.
	RecordOutboxDeadLetter(eventType string)

	// RecordOutboxNoHandler increments when dispatcher has no handler for an
	// event type — surfaces no-consumer / untracked-event conditions.
	RecordOutboxNoHandler(eventType string)

	// RecordOutboxProcessingDuration observes per-event handler latency.
	RecordOutboxProcessingDuration(result string, d time.Duration)

	// RecordOutboxRetryAttemptsAtTerminal observes attempt-count distribution at
	// success or DLQ transition. terminal ∈ {"succeeded", "dead_letter"}.
	RecordOutboxRetryAttemptsAtTerminal(terminal string, attempts int)

	// RecordOutboxStuckEventsRecovered records replay/stuck recovery batches.
	RecordOutboxStuckEventsRecovered(count int)

	// SetWorkerRunning and RecordWorkerHeartbeat expose worker liveness.
	SetWorkerRunning(workerName string, running bool)
	RecordWorkerHeartbeat(workerName string)
}

// ProjectionMetricsRecorder is the subset consumed by ProjectionWorker.
type ProjectionMetricsRecorder interface {
	RecordProjectionEventProcessed(result string)
	SetWorkerRunning(workerName string, running bool)
	RecordWorkerHeartbeat(workerName string)
}

// WorkerLivenessRecorder is the minimal liveness sink consumed by workers that
// only need running/heartbeat visibility (no per-event metrics). Workers call
// SetWorkerRunning on Start/Stop and RecordWorkerHeartbeat only after a
// successful cycle completes — errors intentionally suppress the heartbeat so
// staleness of the timestamp exposes stuck-but-alive vs truly-dead workers.
type WorkerLivenessRecorder interface {
	SetWorkerRunning(workerName string, running bool)
	RecordWorkerHeartbeat(workerName string)
}

// =============================================================================
// Stable worker_name labels.
// =============================================================================
// These are used as Prometheus label values. They are stable across restarts
// (no UUID suffix) so dashboards / alerts can match them deterministically.

const (
	WorkerNameOutbox            = "outbox"
	WorkerNameProjection        = "projection"
	WorkerNameAlertDetection    = "alert_detection"
	WorkerNameAuctionEnd        = "auction_end"
	WorkerNameOrderAutoComplete = "order_auto_complete"
)

// =============================================================================
// Result label values (closed enums).
// =============================================================================

const (
	OutboxResultSucceeded   = "succeeded"
	OutboxResultFailedRetry = "failed_retry"
	OutboxResultDeadLetter  = "dead_letter"
	OutboxResultNoHandler   = "no_handler"

	ProjectionResultProcessed = "processed"
	ProjectionResultSkipped   = "skipped"
	ProjectionResultFailed    = "failed"
)
