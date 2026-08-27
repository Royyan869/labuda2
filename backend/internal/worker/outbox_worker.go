// ⚠️ STANDBY WORKER
// DO NOT ENABLE WITHOUT BUSINESS VALIDATION
// This worker is intentionally disabled pending business validation of outbox processing.
// Worker is initialized but not started in dependencies_core.go.
//
// To enable: Remove this comment and uncomment .Start() call in dependencies_core.go
package worker

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
	orderApp "github.com/labuda/backend/internal/commerce/order/application"
	disputeApp "github.com/labuda/backend/internal/governance/dispute/application"
	coinsApp "github.com/labuda/backend/internal/incentive/coins/application"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatConsumer "github.com/labuda/backend/internal/interaction/chat/consumer"
	platformevent "github.com/labuda/backend/internal/platform/event"
	"github.com/labuda/backend/internal/platform/events"
	"github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/internal/presence"
	dbpkg "github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

const (
	// DefaultOutboxPollInterval is how often the worker checks for pending outbox events
	DefaultOutboxPollInterval = 1 * time.Minute

	// DefaultOutboxBatchSize is max events to process per batch
	DefaultOutboxBatchSize = 100

	// MaxOutboxAttempts is maximum retry attempts before dead letter
	MaxOutboxAttempts = 20

	// OutboxBaseBackoff is the base backoff time for retries (1 second)
	OutboxBaseBackoff = 1 * time.Second

	// OutboxMaxBackoff is the maximum backoff time (1 hour)
	OutboxMaxBackoff = 1 * time.Hour

	// DefaultProcessingTimeout is how long an event can stay in 'processing' before being reset
	DefaultProcessingTimeout = 5 * time.Minute

	// DefaultStuckEventCheckInterval is how often to check for stuck events
	DefaultStuckEventCheckInterval = 1 * time.Minute
)

// =============================================================================
// OUTBOX WORKER
// =============================================================================

// OutboxWorker processes outbox events for reliable event delivery.
//
// PROCESSING FLOW (1 event = 1 transaction):
// 1. Fetch batch of event IDs in short transaction
// 2. For each event:
//   - Start new transaction
//   - Mark as processing
//   - Dispatch to handler
//   - On success: Mark as succeeded
//   - On failure: Mark with retry OR move to dead letter
//   - Commit transaction
//
// SAFETY:
// - Each event processed in its own transaction
// - FOR UPDATE SKIP LOCKED for concurrent worker support
// - Exponential backoff: base * 2^(attempt-1), capped at max
// - Handler registry pattern (no switch-case)
type OutboxWorker struct {
	db                      *dbpkg.DB
	outboxRepo              *repository.OutboxRepository
	dispatcher              *OutboxDispatcher
	log                     *zap.Logger
	pollInterval            time.Duration
	batchSize               int
	maxAttempts             int
	baseBackoff             time.Duration
	maxBackoff              time.Duration
	processingTimeout       time.Duration
	stuckEventCheckInterval time.Duration

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

	// Context for shutdown
	shutdownCtx context.Context
	cancelFn    context.CancelFunc

	// Worker identifier for logging (per-instance UUID, NOT a metric label).
	workerID string

	// metrics is an optional sink-only recorder. nil → silently skip metric emission.
	// Set via SetMetricsRecorder. The recorder MUST NOT influence retry/DLQ
	// decisions; it is invoked AFTER the canonical decision is made.
	metrics OutboxMetricsRecorder
}

// OutboxWorkerConfig holds worker configuration
type OutboxWorkerConfig struct {
	PollInterval            time.Duration // How often to check for pending events
	BatchSize               int           // Max events to process per batch
	MaxAttempts             int           // Maximum retry attempts before dead letter
	BaseBackoff             time.Duration // Base backoff for retry calculation
	MaxBackoff              time.Duration // Maximum backoff cap
	ProcessingTimeout       time.Duration // How long before resetting stuck 'processing' events
	StuckEventCheckInterval time.Duration // How often to check for stuck events
}

// DefaultOutboxWorkerConfig returns default configuration
func DefaultOutboxWorkerConfig() OutboxWorkerConfig {
	return OutboxWorkerConfig{
		PollInterval:            DefaultOutboxPollInterval,
		BatchSize:               DefaultOutboxBatchSize,
		MaxAttempts:             MaxOutboxAttempts,
		BaseBackoff:             OutboxBaseBackoff,
		MaxBackoff:              OutboxMaxBackoff,
		ProcessingTimeout:       DefaultProcessingTimeout,
		StuckEventCheckInterval: DefaultStuckEventCheckInterval,
	}
}

// NewOutboxWorker creates a new outbox worker
func NewOutboxWorker(
	db *dbpkg.DB,
	log *zap.Logger,
	cfg OutboxWorkerConfig,
) *OutboxWorker {
	if log == nil {
		log = zap.NewNop()
	}

	if cfg.PollInterval == 0 {
		cfg.PollInterval = DefaultOutboxPollInterval
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = DefaultOutboxBatchSize
	}
	if cfg.MaxAttempts == 0 {
		cfg.MaxAttempts = MaxOutboxAttempts
	}
	if cfg.BaseBackoff == 0 {
		cfg.BaseBackoff = OutboxBaseBackoff
	}
	if cfg.MaxBackoff == 0 {
		cfg.MaxBackoff = OutboxMaxBackoff
	}
	if cfg.ProcessingTimeout == 0 {
		cfg.ProcessingTimeout = DefaultProcessingTimeout
	}
	if cfg.StuckEventCheckInterval == 0 {
		cfg.StuckEventCheckInterval = DefaultStuckEventCheckInterval
	}

	workerID := fmt.Sprintf("worker-%s", uuid.New().String()[:8])

	return &OutboxWorker{
		db:                      db,
		outboxRepo:              repository.NewOutboxRepository(db),
		dispatcher:              NewOutboxDispatcher(log),
		log:                     log,
		pollInterval:            cfg.PollInterval,
		batchSize:               cfg.BatchSize,
		maxAttempts:             cfg.MaxAttempts,
		baseBackoff:             cfg.BaseBackoff,
		maxBackoff:              cfg.MaxBackoff,
		processingTimeout:       cfg.ProcessingTimeout,
		stuckEventCheckInterval: cfg.StuckEventCheckInterval,
		stopCh:                  make(chan struct{}),
		workerID:                workerID,
	}
}

// SetMetricsRecorder attaches an optional metrics sink. Safe to call before
// Start. Passing nil disables emission. The recorder is sink-only and never
// influences canonical retry / dead-letter decisions.
func (w *OutboxWorker) SetMetricsRecorder(m OutboxMetricsRecorder) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.metrics = m
}

// Start begins processing outbox events in the background
func (w *OutboxWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("Outbox worker already running")
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.stopCh = make(chan struct{})

	if w.metrics != nil {
		w.metrics.SetWorkerRunning(WorkerNameOutbox, true)
	}

	w.wg.Add(1)
	go w.run()

	w.log.Info("Outbox worker started",
		zap.Duration("poll_interval", w.pollInterval),
		zap.Int("batch_size", w.batchSize),
		zap.Int("max_attempts", w.maxAttempts),
		zap.Duration("base_backoff", w.baseBackoff),
		zap.Duration("max_backoff", w.maxBackoff),
		zap.Duration("processing_timeout", w.processingTimeout),
		zap.Duration("stuck_event_check_interval", w.stuckEventCheckInterval),
		zap.String("worker_id", w.workerID),
	)
}

// Stop gracefully shuts down the worker
func (w *OutboxWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.log.Info("Stopping outbox worker...")

	w.cancelFn()
	close(w.stopCh)

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.log.Info("Outbox worker stopped gracefully")
	case <-time.After(10 * time.Second):
		w.log.Warn("Outbox worker shutdown timeout")
	}

	w.running = false
	if w.metrics != nil {
		w.metrics.SetWorkerRunning(WorkerNameOutbox, false)
	}
}

// IsRunning returns true if the worker is currently running
func (w *OutboxWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// run is the main worker loop
func (w *OutboxWorker) run() {
	defer w.wg.Done()

	w.processOutboxBatch()

	// Use a persistent poll ticker so new events continue to be checked
	// after startup. This avoids recreating one-shot timers inside the select loop.
	pollTicker := time.NewTicker(w.pollInterval)
	defer pollTicker.Stop()

	// Start ticker for stuck event recovery
	stuckEventTicker := time.NewTicker(w.stuckEventCheckInterval)
	defer stuckEventTicker.Stop()

	for {
		select {
		case <-w.shutdownCtx.Done():
			w.log.Info("Worker shutdown requested")
			return

		case <-pollTicker.C:
			w.processOutboxBatch()

		case <-stuckEventTicker.C:
			w.recoverStuckEvents()

		case <-w.stopCh:
			return
		}
	}
}

// processOutboxBatch processes a batch of pending outbox events.
//
// TRANSACTION MODEL: 1 event = 1 transaction
// - Step 1: Fetch event IDs in short transaction (with row locks)
// - Step 2: For each event, process in its own transaction
func (w *OutboxWorker) processOutboxBatch() {
	ctx := context.Background()

	// Heartbeat AFTER every poll attempt, regardless of outcome, so staleness of
	// the heartbeat timestamp distinguishes "idle worker" from "stuck worker".
	defer func() {
		if w.metrics != nil {
			w.metrics.RecordWorkerHeartbeat(WorkerNameOutbox)
		}
	}()

	// Step 1: Fetch events in a short transaction
	var events []repository.Event
	err := w.db.WithTx(ctx, func(tx dbpkg.Tx) error {
		var err error
		// NO SQL in worker - use repository method
		events, err = w.outboxRepo.FetchPendingBatch(ctx, tx, w.batchSize)
		return err
	})

	if err != nil {
		w.log.Error("Failed to fetch pending events", zap.Error(err))
		return
	}

	if len(events) == 0 {
		return
	}

	w.log.Debug("Processing outbox batch",
		zap.Int("count", len(events)),
		zap.String("worker_id", w.workerID),
	)

	// Step 2: Process each event in its own transaction
	var succeeded, failed, deadLetter int

	for _, event := range events {
		s, f, d := w.processSingleEvent(ctx, event)
		succeeded += s
		failed += f
		deadLetter += d
	}

	// Log batch stats
	w.log.Info("Outbox batch processed",
		zap.Int("processed", len(events)),
		zap.Int("succeeded", succeeded),
		zap.Int("failed", failed),
		zap.Int("dead_letter", deadLetter),
	)
}

// processSingleEvent processes a single event in its own transaction.
//
// Returns (succeeded, failed, deadLetter) counts.
//
// METRICS: This function is the single point where we know both (a) the event
// type and (b) the terminal disposition the worker assigned. Metric emission
// happens here AFTER the transaction commits, so a rolled-back transaction
// (e.g. infra error) does NOT pollute the counters with phantom outcomes.
func (w *OutboxWorker) processSingleEvent(
	ctx context.Context,
	event repository.Event,
) (succeeded, failed, deadLetter int) {
	start := time.Now()

	var disposition string
	var attemptsAtTerminal int

	err := w.db.WithTx(ctx, func(tx dbpkg.Tx) error {
		d, a, err := w.processEventInTx(ctx, tx, event)
		disposition = d
		attemptsAtTerminal = a
		return err
	})

	duration := time.Since(start)

	if err != nil {
		// Transaction itself failed (e.g. DB infra). The canonical row state has
		// been rolled back, so the event remains in its prior status. We DO NOT
		// emit a terminal-result metric here — that would over-count.
		w.log.Error("Transaction failed for event",
			zap.String("event_id", event.ID.String()),
			zap.String("event_type", event.EventType),
			zap.Error(err),
		)
		return 0, 0, 0
	}

	// Disposition is set by processEventInTx for every committed path.
	if w.metrics != nil && disposition != "" {
		w.metrics.RecordOutboxEventProcessed(event.EventType, disposition)
		w.metrics.RecordOutboxProcessingDuration(disposition, duration)

		switch disposition {
		case OutboxResultSucceeded:
			w.metrics.RecordOutboxRetryAttemptsAtTerminal(OutboxResultSucceeded, attemptsAtTerminal)
		case OutboxResultDeadLetter:
			w.metrics.RecordOutboxDeadLetter(event.EventType)
			w.metrics.RecordOutboxRetryAttemptsAtTerminal(OutboxResultDeadLetter, attemptsAtTerminal)
		case OutboxResultFailedRetry:
			w.metrics.RecordOutboxHandlerFailure(event.EventType)
		case OutboxResultNoHandler:
			w.metrics.RecordOutboxNoHandler(event.EventType)
		}
	}

	switch disposition {
	case OutboxResultSucceeded, OutboxResultNoHandler:
		return 1, 0, 0
	case OutboxResultFailedRetry:
		return 0, 1, 0
	case OutboxResultDeadLetter:
		return 0, 0, 1
	default:
		// Skipped (e.g. invalid status transition — another worker raced us).
		return 0, 0, 0
	}
}

// processEventInTx processes an event within a single transaction.
//
// All operations are atomic within this transaction:
// - MarkProcessing
// - Dispatch
// - MarkSucceeded OR MarkFailedWithRetry OR MoveToDeadLetter
//
// Returns:
//   - disposition: the terminal label assigned to this event in this transaction.
//     "" means skipped (e.g. lost the race to another worker).
//     OutboxResultSucceeded / OutboxResultFailedRetry / OutboxResultDeadLetter /
//     OutboxResultNoHandler for the four committed outcomes.
//   - attemptsAtTerminal: retry_count snapshot at the moment of disposition. For
//     succeeded/no_handler this is the row's prior retry_count; for failed/DLQ
//     it is the new retry_count (prior + 1).
//   - error: only non-nil for infra failures that should roll the tx back.
//     Handler failures are absorbed into the failed_retry/dead_letter paths.
func (w *OutboxWorker) processEventInTx(
	ctx context.Context,
	tx dbpkg.Tx,
	event repository.Event,
) (string, int, error) {
	// Step 1: Mark as processing
	if err := w.outboxRepo.MarkProcessing(ctx, tx, event.ID); err != nil {
		if errors.Is(err, repository.ErrInvalidStatusTransition) {
			// Event already being processed by another worker - skip
			return "", 0, nil
		}
		return "", 0, fmt.Errorf("mark processing failed: %w", err)
	}

	// Step 2: Dispatch to handler
	dispatchResult, dispatchErr := w.dispatcher.DispatchWithResult(ctx, event)
	if dispatchErr != nil {
		// Handler failed - handle with retry logic. handleFailureInTx returns the
		// disposition (failed_retry or dead_letter) and the new attempt count.
		return w.handleFailureInTx(ctx, tx, event, dispatchErr)
	}

	// Step 3: Success - mark as succeeded (covers both handler-success and
	// no-handler-registered cases; both are canonical "delivered" outcomes per
	// the dispatcher's existing semantics).
	if err := w.outboxRepo.MarkSucceeded(ctx, tx, event.ID); err != nil {
		return "", 0, fmt.Errorf("mark succeeded failed: %w", err)
	}

	w.log.Debug("Event processed successfully",
		zap.String("event_id", event.ID.String()),
		zap.String("event_type", event.EventType),
		zap.String("dispatch_result", string(dispatchResult)),
	)

	if dispatchResult == DispatchResultNoHandler {
		return OutboxResultNoHandler, event.RetryCount, nil
	}
	return OutboxResultSucceeded, event.RetryCount, nil
}

// handleFailureInTx handles event processing failure within transaction.
//
// Decides between retry and dead letter based on attempt count.
//
// Returns the disposition (failed_retry | dead_letter), the new attempt count,
// and any infra error that should roll back the transaction.
func (w *OutboxWorker) handleFailureInTx(
	ctx context.Context,
	tx dbpkg.Tx,
	event repository.Event,
	processErr error,
) (string, int, error) {
	newRetryCount := event.RetryCount + 1

	// Check if should move to dead letter
	if newRetryCount >= w.maxAttempts {
		if err := w.outboxRepo.MoveToDeadLetter(ctx, tx, event.ID); err != nil {
			return "", 0, fmt.Errorf("move to dead letter failed: %w", err)
		}

		// Structured high-severity log keeps last_error attribution even though
		// the outbox schema does not store it. Paired with outbox_dead_letter_total
		// counter emission in processSingleEvent.
		w.log.Warn("Event moved to dead letter",
			zap.String("event_id", event.ID.String()),
			zap.String("event_type", event.EventType),
			zap.String("aggregate_type", event.AggregateType),
			zap.Int("attempts", newRetryCount),
			zap.Error(processErr),
		)

		return OutboxResultDeadLetter, newRetryCount, nil
	}

	// Calculate backoff: base * 2^(attempt-1), capped at max
	backoff := w.calculateBackoff(newRetryCount)
	nextAttemptAt := time.Now().Add(backoff)

	// Mark as failed with retry info
	if err := w.outboxRepo.MarkFailedWithRetry(ctx, tx, event.ID, newRetryCount, nextAttemptAt); err != nil {
		return "", 0, fmt.Errorf("mark failed failed: %w", err)
	}

	w.log.Info("Event processing failed, will retry",
		zap.String("event_id", event.ID.String()),
		zap.String("event_type", event.EventType),
		zap.Int("attempt", newRetryCount),
		zap.Duration("backoff", backoff),
		zap.Error(processErr),
	)

	return OutboxResultFailedRetry, newRetryCount, nil
}

// calculateBackoff calculates exponential backoff for retry.
//
// Formula: base * 2^(attempt-1), capped at maxBackoff
//
// Examples (with base=1s):
// - Attempt 1: 1s * 2^0 = 1s
// - Attempt 2: 1s * 2^1 = 2s
// - Attempt 3: 1s * 2^2 = 4s
// - Attempt 4: 1s * 2^3 = 8s
// - ...capped at maxBackoff (1 hour)
func (w *OutboxWorker) calculateBackoff(attempt int) time.Duration {
	// Calculate: base * 2^(attempt-1)
	// Use math.Pow for clarity and safety (no bit shift overflow)
	multiplier := math.Pow(2, float64(attempt-1))
	backoff := float64(w.baseBackoff) * multiplier

	// Cap at maxBackoff
	if backoff > float64(w.maxBackoff) {
		backoff = float64(w.maxBackoff)
	}

	return time.Duration(backoff)
}

// ManualProcess triggers immediate processing of pending events.
// Useful for testing or manual intervention.
func (w *OutboxWorker) ManualProcess(ctx context.Context) error {
	w.processOutboxBatch()
	return nil
}

// recoverStuckEvents resets events that have been stuck in 'processing' status.
//
// SELF-HEALING MECHANISM:
// This function runs periodically to detect and recover events that were
// left in 'processing' status due to worker crashes or failures.
//
// SAFETY:
// - Only resets events that have been stuck longer than processingTimeout
// - Increments retry_count to track recovery attempts
// - Logs recovery metrics for monitoring
func (w *OutboxWorker) recoverStuckEvents() {
	ctx := context.Background()

	count, err := w.outboxRepo.ResetStuckEvents(ctx, w.processingTimeout)
	if err != nil {
		w.log.Error("Failed to recover stuck events",
			zap.Error(err),
			zap.Duration("timeout", w.processingTimeout),
		)
		return
	}

	if count > 0 {
		w.log.Info("Recovered stuck events",
			zap.Int("recovered_count", count),
			zap.Duration("processing_timeout", w.processingTimeout),
			zap.String("worker_id", w.workerID),
		)
		if w.metrics != nil {
			// Replay-storm visibility: cumulative count of resets, alertable on
			// rate.
			w.metrics.RecordOutboxStuckEventsRecovered(count)
		}
	}
}

// =============================================================================
// EVENT HANDLER INTERFACE
// =============================================================================

// EventHandler handles a specific outbox event type.
//
// Implementations should be idempotent and handle their own error recovery.
type EventHandler interface {
	// Handle processes the event. Return nil for success, error for retry.
	Handle(ctx context.Context, event platformevent.OutboxEvent) error
}

// =============================================================================
// OUTBOX DISPATCHER (REGISTRY PATTERN)
// =============================================================================

// OutboxDispatcher routes events to appropriate handlers using a registry pattern.
//
// NO SWITCH-CASE - uses handler registry for extensibility.
type OutboxDispatcher struct {
	handlers map[string]EventHandler
	log      *zap.Logger
}

// NewOutboxDispatcher creates a new outbox dispatcher
func NewOutboxDispatcher(log *zap.Logger) *OutboxDispatcher {
	if log == nil {
		log = zap.NewNop()
	}
	return &OutboxDispatcher{
		handlers: make(map[string]EventHandler),
		log:      log,
	}
}

// Register registers a handler for a specific event type.
//
// DUPLICATE GUARD: If a handler is already registered for the same event type,
// Register panics at startup. Use RegisterFanout() to deliberately wire
// multiple handlers for the same event type.
func (d *OutboxDispatcher) Register(eventType string, handler EventHandler) {
	if existing, ok := d.handlers[eventType]; ok {
		// Allow re-registration of the exact same handler (idempotent Setup* calls).
		if existing == handler {
			return
		}
		panic(fmt.Sprintf(
			"DISPATCHER_DUPLICATE_REGISTRATION: event type %q already has a handler; "+
				"use RegisterFanout() to wire multiple handlers for the same event",
			eventType,
		))
	}
	d.handlers[eventType] = handler
	d.log.Debug("Registered event handler",
		zap.String("event_type", eventType),
	)
}

// RegisterMultiple registers a handler for multiple event types.
func (d *OutboxDispatcher) RegisterMultiple(eventTypes []string, handler EventHandler) {
	for _, eventType := range eventTypes {
		d.Register(eventType, handler)
	}
}

// RegisterFanout registers multiple handlers for a single event type.
// Handlers execute sequentially in the order provided. If any handler returns
// an error, execution stops and the error propagates (triggering outbox retry).
//
// This is the ONLY way to wire multiple consumers for the same event type.
// Register() will panic if a handler already exists for the event type.
func (d *OutboxDispatcher) RegisterFanout(eventType string, handlers ...EventHandler) {
	if len(handlers) == 0 {
		return
	}
	if len(handlers) == 1 {
		d.handlers[eventType] = handlers[0]
		d.log.Debug("Registered event handler (fanout with single handler)",
			zap.String("event_type", eventType),
		)
		return
	}
	d.handlers[eventType] = &fanoutHandler{handlers: handlers}
	d.log.Info("Registered fanout handler",
		zap.String("event_type", eventType),
		zap.Int("handler_count", len(handlers)),
	)
}

// fanoutHandler executes multiple EventHandlers sequentially for a single event.
// If any handler fails, execution stops and the error is returned (outbox retry
// will re-invoke ALL handlers; each handler MUST be idempotent).
type fanoutHandler struct {
	handlers []EventHandler
}

func (f *fanoutHandler) Handle(ctx context.Context, event platformevent.OutboxEvent) error {
	for i, h := range f.handlers {
		if err := h.Handle(ctx, event); err != nil {
			return fmt.Errorf("fanout handler %d/%d failed: %w", i+1, len(f.handlers), err)
		}
	}
	return nil
}

// DispatchResult describes which branch the dispatcher took.
// It exists so the outbox worker can distinguish "handler delivered" from
// "no handler registered" without changing the at-least-once delivery contract
// (both are still treated as canonical success at the row-status layer).
type DispatchResult string

const (
	// DispatchResultHandled means a registered handler returned nil.
	DispatchResultHandled DispatchResult = "handled"
	// DispatchResultNoHandler means no handler was registered for the event
	// type. The event is marked succeeded (existing behaviour) but the worker
	// emits a separate observability signal so this is not silent.
	DispatchResultNoHandler DispatchResult = "no_handler"
)

// Dispatch routes an event to the registered handler.
//
// Returns nil if:
// - Handler processed successfully
// - No handler registered (logs warning, treats as success)
//
// Returns error if:
// - Handler failed (triggers retry logic)
//
// Kept for callers that don't care about the no-handler distinction.
func (d *OutboxDispatcher) Dispatch(ctx context.Context, event repository.Event) error {
	_, err := d.DispatchWithResult(ctx, event)
	return err
}

// DispatchWithResult is the same as Dispatch but also reports which branch was
// taken so the worker can attribute the outcome in metrics.
func (d *OutboxDispatcher) DispatchWithResult(ctx context.Context, event repository.Event) (DispatchResult, error) {
	d.log.Debug("Dispatching event",
		zap.String("event_id", event.ID.String()),
		zap.String("event_type", event.EventType),
		zap.String("aggregate_type", event.AggregateType),
	)

	handler, exists := d.handlers[event.EventType]
	if !exists {
		// Unknown event type - log warning but don't fail. The worker pairs this
		// branch with an outbox_no_handler_total counter increment so this is
		// observable, not silent.
		d.log.Warn("No handler registered for event type, skipping",
			zap.String("event_type", event.EventType),
			zap.String("event_id", event.ID.String()),
		)
		return DispatchResultNoHandler, nil
	}

	// Convert repository.Event to OutboxEvent
	outboxEvent := platformevent.OutboxEvent{
		ID:            event.ID,
		AggregateType: event.AggregateType,
		AggregateID:   event.AggregateID,
		EventType:     event.EventType,
		Payload:       event.Payload,
	}

	// Delegate to handler
	if err := handler.Handle(ctx, outboxEvent); err != nil {
		return DispatchResultHandled, fmt.Errorf("handler failed for event type %s: %w", event.EventType, err)
	}

	return DispatchResultHandled, nil
}

// =============================================================================
// DEFAULT NO-OP HANDLERS
// These handlers safely acknowledge events without side effects.
// They prevent "unknown event type" warnings while workflows are inactive.
// =============================================================================

// PaymentEventHandler handles payment-related events
type PaymentEventHandler struct {
	log *zap.Logger
}

func NewPaymentEventHandler(log *zap.Logger) *PaymentEventHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &PaymentEventHandler{log: log}
}

func (h *PaymentEventHandler) Handle(ctx context.Context, event platformevent.OutboxEvent) error {
	h.log.Debug("Handling payment event",
		zap.String("event_type", event.EventType),
		zap.String("event_id", event.ID.String()),
	)
	// NO-OP HANDLER: Payment events are currently logged only.
	// Future: Add notification/webhook logic when payment workflows are active.
	return nil
}

// UserEventHandler handles user-related events
type UserEventHandler struct {
	log *zap.Logger
}

func NewUserEventHandler(log *zap.Logger) *UserEventHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &UserEventHandler{log: log}
}

func (h *UserEventHandler) Handle(ctx context.Context, event platformevent.OutboxEvent) error {
	h.log.Debug("Handling user event",
		zap.String("event_type", event.EventType),
		zap.String("event_id", event.ID.String()),
	)
	// NO-OP HANDLER: User events are currently logged only.
	// Future: Add email/analytics/sync logic when user workflows are active.
	return nil
}

// =============================================================================
// WORKER SETUP HELPER
// =============================================================================

// SetupDefaultHandlers registers all default event handlers with the dispatcher.
//
// This is a convenience method for worker initialization.
// Custom handlers can be registered after calling this.
func (w *OutboxWorker) SetupDefaultHandlers() {
	w.dispatcher.RegisterMultiple([]string{
		"payment.completed",
		"payment.expired",
		"payment.failed",
	}, NewPaymentEventHandler(w.log))

	w.dispatcher.RegisterMultiple([]string{
		"user.created",
		"user.role_changed",
	}, NewUserEventHandler(w.log))

	w.log.Info("Default event handlers registered")
}

// SetupPresenceLastSeenHandler registers the canonical durable retry handler
// for presence.last_seen_record.
func (w *OutboxWorker) SetupPresenceLastSeenHandler(presenceService *presence.Service) *OutboxWorker {
	w.dispatcher.Register(events.EventUserPresenceLastSeenRecord, NewPresenceLastSeenHandler(presenceService, w.log))
	w.log.Info("Presence last_seen retry handler registered")
	return w
}

// SetupModerationHandlers registers moderation event handlers.
//
// This must be called after worker creation and requires:
//   - ContentService for content soft-deletes
//   - CommentService for comment soft-deletes
//   - ForSaleService for for_sale withdrawal
//   - AuctionService for auction cancellation
//   - UserRepository for user suspension
//   - notifHandler: the NotificationEventHandler (returned by SetupNotificationHandlers).
//     Must be called AFTER SetupNotificationHandlers so notifHandler is non-nil.
//
// Fanout ordering (enforcement events):
//
//	ModerationEventHandler (enforcement) → NotificationEventHandler (notification)
//
// SetupModerationWSEvictionHandler, if called after this, composes WS eviction
// as a third handler for moderation.user.suspended only.
//
// Usage:
//
//	import contentapp "github.com/labuda/backend/internal/social/content/application"
//	import forsaleapp "github.com/labuda/backend/internal/commerce/forsale/application"
//	import auctionapp "github.com/labuda/backend/internal/commerce/auction/application"
//	import userrepo "github.com/labuda/backend/internal/identity/user/repository"
//	worker.SetupModerationHandlers(db, contentService, commentService, forSaleService, auctionService, userRepo, notifHandler)
func (w *OutboxWorker) SetupModerationHandlers(
	db *dbpkg.DB,
	contentService interface{},
	commentService interface{},
	forSaleService interface{},
	auctionService interface{},
	userRepo interface{},
	chatMessageStore ChatMessageModerationService,
	notifHandler EventHandler,
) *OutboxWorker {
	// The contentService is expected to be *contentapp.ContentService
	// The commentService is expected to be *contentapp.CommentService
	// The forSaleService is expected to be *forsaleapp.ForSaleService
	// The auctionService is expected to be *auctionapp.AuctionService
	// The userRepo is expected to be userrepo.UserRepository
	// The chatMessageStore is expected to satisfy ChatMessageModerationStore
	// Type assertion will happen in NewModerationEventHandler
	handler := NewModerationEventHandler(db, contentService, commentService, forSaleService, auctionService, userRepo, chatMessageStore, w.log)

	// Enforcement-only: chat_message has no seller-facing notification.
	w.dispatcher.RegisterMultiple([]string{
		"moderation.chat_message.hidden",
		"moderation.chat_message.restored",
	}, handler)

	// Enforcement + notification fanout: enforcement FIRST, notification SECOND.
	// SetupModerationWSEvictionHandler composes WS eviction as THIRD for .suspended.
	for _, eventType := range []string{
		"moderation.content.removed",
		"moderation.comment.removed",
		"moderation.for_sale.removed",
		"moderation.auction.removed",
		"moderation.user.suspended",
		"moderation.content.restored",
		"moderation.comment.restored",
		"moderation.for_sale.restored",
		"moderation.auction.restored",
		"moderation.user.restored",
	} {
		w.dispatcher.RegisterFanout(eventType, handler, notifHandler)
	}

	w.log.Info("Moderation event handlers registered (with enforcement + notification fanout)")
	return w
}

// SetupPromotionHandlers registers promotion event handlers.
//
// This must be called after worker creation and requires the PromotionService
// for handling promotion auto-stop/pause/resume when targets become non-operable.
//
// ORDERING CONSTRAINT: Must be called AFTER SetupNotificationHandlers,
// SetupModerationHandlers, and SetupSellerSubscriptionExpiredHandler so that
// seller governance and moderation events can be composed via fanout with
// their existing handlers.
//
// Events handled:
//   - for_sale.sold/withdrawn/updated: target-level pause/stop/resume
//   - auction.ended/cancelled: target-level stop
//   - seller.subscription.activated: resume user's paused promotions
//   - seller.subscription.expired: pause user's active promotions (fanout)
//   - moderation.for_sale.restored: resume paused promotions for fixed-price sale (fanout)
//
// Usage:
//
//	import promotionApp "github.com/labuda/backend/internal/pricing/promotion/application"
//	worker.SetupPromotionHandlers(db, promotionService)
func (w *OutboxWorker) SetupPromotionHandlers(db *dbpkg.DB, promotionService interface{}) *OutboxWorker {
	promotionApp := NewPromotionEventHandlerWrapper(db, promotionService, w.log)

	// Target-level events — promotion handler is sole consumer.
	w.dispatcher.RegisterMultiple([]string{
		"for_sale.sold",
		"for_sale.withdrawn",
		"for_sale.updated",
	}, promotionApp)

	// auction.cancelled — promotion handler is sole consumer.
	w.dispatcher.Register("auction.cancelled", promotionApp)

	// auction.ended — compose via fanout with notification handler (P14: seller
	// notified when auction closes with no winner). Notification handler is
	// registered first by SetupNotificationHandlers (ordering constraint above).
	{
		const eventType = "auction.ended"
		if existing, ok := w.dispatcher.handlers[eventType]; ok {
			w.dispatcher.handlers[eventType] = &fanoutHandler{
				handlers: []EventHandler{existing, promotionApp},
			}
		} else {
			w.dispatcher.Register(eventType, promotionApp)
		}
	}

	// Seller subscription activated — no existing handler (was NoHandlerAuditOnly).
	w.dispatcher.Register("seller.subscription.activated", promotionApp)

	// Seller governance events — compose via fanout with existing handlers
	// (notification handler registered by SetupNotificationHandlers,
	// auction cancellation handler by SetupSellerSubscriptionExpiredHandler).
	for _, eventType := range []string{
		"seller.subscription.expired",
	} {
		if existing, ok := w.dispatcher.handlers[eventType]; ok {
			w.dispatcher.handlers[eventType] = &fanoutHandler{
				handlers: []EventHandler{existing, promotionApp},
			}
		} else {
			w.dispatcher.Register(eventType, promotionApp)
		}
	}

	// moderation.for_sale.restored — compose via fanout with existing handlers
	// (enforcement + notification fanout registered by SetupModerationHandlers).
	{
		const eventType = "moderation.for_sale.restored"
		if existing, ok := w.dispatcher.handlers[eventType]; ok {
			w.dispatcher.handlers[eventType] = &fanoutHandler{
				handlers: []EventHandler{existing, promotionApp},
			}
		} else {
			w.dispatcher.Register(eventType, promotionApp)
		}
	}

	w.log.Info("Promotion event handlers registered (target + seller governance + moderation)")
	return w
}

// SetupRefundFailedAlertHandler registers the money.refund_failed event handler.
//
// O1A: Converts gateway refund failures into CRITICAL operator alerts.
// Previously audit-only (NoHandlerAuditOnly); now surfaces in system_alerts
// table for admin visibility.
//
// FANOUT-READY: If SetupNotificationHandlers was called before this (i.e.
// money.refund_failed already has a notification handler), this method
// composes both handlers via fanout so neither overwrites the other.
// Alert creation runs FIRST, then admin notification fanout.
//
// IDEMPOTENCY: AlertService deduplicates by refund_id within 60-min window.
func (w *OutboxWorker) SetupRefundFailedAlertHandler(alertService RefundFailedAlertCreator) *OutboxWorker {
	alertHandler := NewRefundFailedAlertHandler(alertService, w.log)

	const eventType = "money.refund_failed"
	if existing, ok := w.dispatcher.handlers[eventType]; ok {
		// Notification handler already registered — compose via fanout.
		// Alert creation runs first (domain side-effect), then notification.
		w.dispatcher.handlers[eventType] = &fanoutHandler{
			handlers: []EventHandler{alertHandler, existing},
		}
		w.log.Info("Refund failed alert handler composed with existing notification handler (fanout)")
	} else {
		w.dispatcher.Register(eventType, alertHandler)
		w.log.Info("Refund failed alert handler registered (O1A: operator visibility)")
	}
	return w
}

// SetupNegotiationHandlers registers negotiation event handlers for chat integration.
//
// ❗ CRITICAL: Chat domain consumes negotiation events to create:
// - negotiation.started: Creates chat room + initial proposal message
// - negotiation.message_sent: Creates proposal message in existing room
//
// IDEMPOTENCY STRATEGY:
// - Uses event ID as idempotency key
// - Database constraints prevent duplicates
// - Safe for retry without creating duplicates
//
// NO CROSS-DOMAIN WRITE:
// - Chat handler does NOT write to negotiation_sessions table
// - Only creates chat_rooms and chat_messages
//
// Usage:
//
//	import chatApp "github.com/labuda/backend/internal/interaction/chat/application"
//	worker.SetupNegotiationHandlers(db, chatService, notifHandler)
func (w *OutboxWorker) SetupNegotiationHandlers(
	db *dbpkg.DB,
	chatService *chatApp.Service,
	notificationHandler EventHandler,
) *OutboxWorker {
	// Create handlers for both event types
	negotiationStartedHandler := chatConsumer.NewNegotiationStartedHandler(db, chatService, w.log)
	negotiationMessageSentHandler := chatConsumer.NewNegotiationMessageSentHandler(db, chatService, w.log)

	// FANOUT: negotiation.started and negotiation.message_sent need BOTH:
	// 1. Chat-domain handler (creates room / sends proposal message) — runs FIRST
	// 2. Notification handler (sends push notification to counterparty) — runs SECOND
	//
	// Domain side-effect (chat room) must exist before notification fires,
	// because the notification data.chatRoomId reference is only valid after
	// the room is created.
	w.dispatcher.RegisterFanout("negotiation.started",
		negotiationStartedHandler,
		notificationHandler,
	)
	w.dispatcher.RegisterFanout("negotiation.message_sent",
		negotiationMessageSentHandler,
		notificationHandler,
	)

	w.log.Info("Negotiation event handlers registered (chat + notification fanout)")
	return w
}

// SetupOrderChatLinkHandler registers the order.chat_link_requested handler.
//
// This handler exists so chat_rooms mutation is OUT of the canonical order
// transaction (RUNTIME-INVARIANTS §1.2 — a transaction MUST NOT span two
// domain authorities). The order tx emits order.chat_link_requested in the
// same commit as order.created; this consumer performs the linkage
// idempotently after the order has been durably committed.
//
// Failure semantics: eventual consistency. Linkage failure does NOT roll back
// the order. Outbox retries the consumer; persistent failure surfaces via the
// outbox_dead_letter_total metric.
//
// Usage:
//
//	import chatApp "github.com/labuda/backend/internal/interaction/chat/application"
//	worker.SetupOrderChatLinkHandler(chatService)
func (w *OutboxWorker) SetupOrderChatLinkHandler(
	chatService *chatApp.Service,
) *OutboxWorker {
	handler := chatConsumer.NewOrderChatLinkHandler(chatService, w.log)

	w.dispatcher.Register(events.EventOrderChatLinkRequested, handler)

	w.log.Info("Order chat-link event handler registered (decoupled chat linkage)")
	return w
}

// SetupBNRStrikeHandler registers the canonical BNR strike recorder.
//
// Consumes auction_bnr_detected events (emitted by AuctionSettlementWorker)
// and inserts a row into buyer_bnr_strikes. Idempotent via UNIQUE(auction_id).
//
// SCOPE: recording only. No enforcement, no decay, no notification.
func (w *OutboxWorker) SetupBNRStrikeHandler(db dbpkg.Transactor) *OutboxWorker {
	handler := NewBNRStrikeHandler(db, w.log)
	w.dispatcher.Register("auction_bnr_detected", handler)
	w.log.Info("BNR strike handler registered (canonical strike recorder)")
	return w
}

// SetupBNRHandlers registers the BNR strike recorder AND notification handler
// as a fanout pair for auction_bnr_detected events.
//
// Order: strike recording runs first, then notifications.
// Both handlers are idempotent (strike via UNIQUE(auction_id), notifications
// via ON CONFLICT (recipient_id, actor_id, type, entity_id) DO NOTHING).
func (w *OutboxWorker) SetupBNRHandlers(db dbpkg.Transactor, notifHandler *NotificationEventHandler) *OutboxWorker {
	strikeHandler := NewBNRStrikeHandler(db, w.log)
	w.dispatcher.RegisterFanout("auction_bnr_detected", strikeHandler, notifHandler)
	w.log.Info("BNR handlers registered (strike recorder + notification fanout)")
	return w
}

// SetupUserBanHandler registers the user.banned event handler.
//
// MODERATION DOMAIN HARD LOCK - SESSION 3:
// - Processes user.banned events
// - Finds all active orders for the banned user
// - Applies safe refund logic based on shipment evidence
// - Ensures idempotency via processed_ban_events table
//
// Usage:
//
//	import orderApp "github.com/labuda/backend/internal/commerce/order/application"
//	import disputeApp "github.com/labuda/backend/internal/workflow/dispute/application"
//	worker.SetupUserBanHandler(db, orderService, disputeService)
func (w *OutboxWorker) SetupUserBanHandler(
	db *dbpkg.DB,
	orderService *orderApp.OrderService,
	disputeService *disputeApp.DisputeService,
) *OutboxWorker {
	handler := NewUserBanEventHandler(db, orderService, disputeService, w.log)

	w.dispatcher.Register("user.banned", handler)

	w.log.Info("User ban event handler registered")
	return w
}

// SetupWSEvictionHandler registers the user.banned → WS session eviction handler.
//
// CHAT-3: event-driven eviction. When a user is banned the outbox worker picks
// up the user.banned event and calls hub.EvictUser() to close their active WS
// sessions immediately (ADR-005 — NOT polling-based).
//
// FANOUT-READY: If SetupUserBanHandler was called before this (i.e. user.banned
// already has a handler), this method uses RegisterFanout to wire both handlers.
// Otherwise it uses Register for the sole handler. This means re-enabling
// UserBanEventHandler requires NO dispatcher changes — just flip the env gate.
func (w *OutboxWorker) SetupWSEvictionHandler(hub WSHub) *OutboxWorker {
	evictionHandler := NewWSEvictionHandler(hub, w.log)

	if existing, ok := w.dispatcher.handlers["user.banned"]; ok {
		// UserBanEventHandler already registered — compose via fanout.
		// UserBanEventHandler runs first (order refunds), then WS eviction.
		w.dispatcher.handlers["user.banned"] = &fanoutHandler{
			handlers: []EventHandler{existing, evictionHandler},
		}
		w.log.Info("WS eviction handler composed with existing user.banned handler (fanout)")
	} else {
		w.dispatcher.Register("user.banned", evictionHandler)
		w.log.Info("WS eviction handler registered for user.banned events")
	}
	return w
}

// SetupModerationWSEvictionHandler registers WS session eviction for
// moderation.user.suspended events. Mirrors SetupWSEvictionHandler (which
// handles user.banned) but parses the moderation payload format.
//
// FANOUT-READY: If SetupModerationHandlers was called before this (i.e.
// moderation.user.suspended already has a handler), this method composes
// both handlers via fanout so neither overwrites the other.
func (w *OutboxWorker) SetupModerationWSEvictionHandler(hub WSHub) *OutboxWorker {
	evictionHandler := NewModerationWSEvictionHandler(hub, w.log)

	const eventType = "moderation.user.suspended"
	if existing, ok := w.dispatcher.handlers[eventType]; ok {
		// ModerationEventHandler already registered — compose via fanout.
		// Suspension handler runs first (sets account_status), then WS eviction.
		w.dispatcher.handlers[eventType] = &fanoutHandler{
			handlers: []EventHandler{existing, evictionHandler},
		}
		w.log.Info("WS eviction handler composed with moderation.user.suspended handler (fanout)")
	} else {
		w.dispatcher.Register(eventType, evictionHandler)
		w.log.Info("WS eviction handler registered for moderation.user.suspended events")
	}
	return w
}

// SetupCoinsRefundRequiredHandler registers the coins.refund_required event handler.
//
// CRITICAL: This is the SINGLE ENTRY POINT for all coins refunds.
// All refund logic flows through this handler to ensure:
// 1. Idempotency (no double refunds)
// 2. Transaction-based refunds (not order.snapshot based)
// 3. Traceability (all refunds logged)
// 4. Failure recovery (coins.refund_failed event on error)
//
// This handler REPLACES all direct calls to RefundCoinsInternal() across the codebase.
//
// Usage:
//
//	import coinsApp "github.com/labuda/backend/internal/incentive/coins/application"
//	worker.SetupCoinsRefundRequiredHandler(db, coinsService)
func (w *OutboxWorker) SetupCoinsRefundRequiredHandler(
	db *dbpkg.DB,
	coinsService *coinsApp.CoinsService,
) *OutboxWorker {
	handler := NewCoinsRefundRequiredHandler(db, coinsService, w.log)

	w.dispatcher.Register("coins.refund_required", handler)

	w.log.Info("Coins refund required event handler registered")
	return w
}
