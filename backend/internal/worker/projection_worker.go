// Package worker provides background workers for various async tasks.
// This file contains the projection worker that maintains read models.
//
// ⚠️  SCHEMA-ALIGNED STANDBY WORKER
// Schema and code are consistent as of migration 000142. Worker remains
// disabled pending dev/staging smoke-test validation.
//
// Canonical projection scope:
//   - order_summaries  — order list for buyer, seller, admin
//   - account_balances — DORMANT; no readers until finance dashboard is built
//   - projection_tracker — idempotency guard for event consumption
//
// To enable: set env PROJECTION_WORKER=true (or omit DISABLE_PROJECTION_WORKER).
// See serverboot/dependencies.go for the env gate.
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/projection"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/internal/platform/events"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

const (
	// DefaultProjectionPollInterval is how often the worker checks for new events
	DefaultProjectionPollInterval = 30 * time.Second

	// DefaultProjectionBatchSize is max events to process per poll
	DefaultProjectionBatchSize = 50
)

// =============================================================================
// PROJECTION WORKER
// =============================================================================

// ProjectionWorker maintains read models by consuming outbox events.
//
// DESIGN PRINCIPLES:
// - No business validation (events are already validated by domain)
// - No state transitions (projections are one-way: write model → read model)
// - No financial recalculation (copies data from write model)
// - Pure projection: denormalizes data for fast reads
//
// FLOW:
// 1. Poll outbox for succeeded events (events already delivered to external systems)
// 2. For each event, RE-QUERY write model for fresh data
// 3. Update the appropriate read model table
// 4. Mark event as processed in projection_tracker
//
// IDEMPOTENCY:
// - Single source of truth: projection_tracker table
// - Replaying same event yields same result (overwrite-based upsert)
// - 1 row per entity: projection is last-known-state only
type ProjectionWorker struct {
	db             *db.DB
	outboxRepo     *outboxRepo.OutboxRepository
	projectionRepo *projection.Repository
	log            *zap.Logger
	pollInterval   time.Duration
	batchSize      int

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
	metrics ProjectionMetricsRecorder
}

// ProjectionWorkerConfig holds worker configuration
type ProjectionWorkerConfig struct {
	PollInterval time.Duration // How often to check for new events
	BatchSize    int           // Max events to process per batch
}

// DefaultProjectionWorkerConfig returns default configuration
func DefaultProjectionWorkerConfig() ProjectionWorkerConfig {
	return ProjectionWorkerConfig{
		PollInterval: DefaultProjectionPollInterval,
		BatchSize:    DefaultProjectionBatchSize,
	}
}

// NewProjectionWorker creates a new projection worker
func NewProjectionWorker(
	database *db.DB,
	log *zap.Logger,
	cfg ProjectionWorkerConfig,
) *ProjectionWorker {
	if log == nil {
		log = zap.NewNop()
	}

	if cfg.PollInterval == 0 {
		cfg.PollInterval = DefaultProjectionPollInterval
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = DefaultProjectionBatchSize
	}

	workerID := fmt.Sprintf("projection-worker-%s", uuid.New().String()[:8])

	return &ProjectionWorker{
		db:             database,
		outboxRepo:     outboxRepo.NewOutboxRepository(database),
		projectionRepo: projection.NewRepository(database),
		log:            log.With(zap.String("worker", workerID)),
		pollInterval:   cfg.PollInterval,
		batchSize:      cfg.BatchSize,
		stopCh:         make(chan struct{}),
		workerID:       workerID,
	}
}

// SetMetricsRecorder attaches an optional metrics sink. Safe to call before Start.
// The recorder is sink-only and never influences projection state.
func (w *ProjectionWorker) SetMetricsRecorder(m ProjectionMetricsRecorder) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.metrics = m
}

// Start begins processing outbox events in the background
func (w *ProjectionWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("Projection worker already running")
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.stopCh = make(chan struct{})

	if w.metrics != nil {
		w.metrics.SetWorkerRunning(WorkerNameProjection, true)
	}

	w.wg.Add(1)
	go w.run()

	w.log.Info("Projection worker started",
		zap.Duration("poll_interval", w.pollInterval),
		zap.Int("batch_size", w.batchSize),
	)
}

// Stop gracefully shuts down the worker
func (w *ProjectionWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.log.Info("Stopping projection worker...")

	w.cancelFn()
	close(w.stopCh)

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.log.Info("Projection worker stopped gracefully")
	case <-time.After(10 * time.Second):
		w.log.Warn("Projection worker shutdown timeout")
	}

	w.running = false
	if w.metrics != nil {
		w.metrics.SetWorkerRunning(WorkerNameProjection, false)
	}
}

// IsRunning returns true if the worker is currently running
func (w *ProjectionWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// run is the main worker loop
func (w *ProjectionWorker) run() {
	defer w.wg.Done()

	w.processBatch()

	for {
		select {
		case <-w.shutdownCtx.Done():
			w.log.Info("Worker shutdown requested")
			return

		case <-time.After(w.pollInterval):
			w.processBatch()

		case <-w.stopCh:
			return
		}
	}
}

// processBatch processes a batch of succeeded outbox events
func (w *ProjectionWorker) processBatch() {
	ctx := context.Background()

	// Heartbeat after every poll attempt (including empty batches and fetch
	// errors). Staleness of this timestamp exposes a stuck projection worker.
	defer func() {
		if w.metrics != nil {
			w.metrics.RecordWorkerHeartbeat(WorkerNameProjection)
		}
	}()

	// Step 1: Fetch succeeded events that haven't been processed yet
	events, err := w.fetchUnprocessedEvents(ctx, w.batchSize)
	if err != nil {
		w.log.Error("Failed to fetch unprocessed events", zap.Error(err))
		return
	}

	if len(events) == 0 {
		return
	}

	w.log.Debug("Processing projection batch",
		zap.Int("count", len(events)),
		zap.String("worker_id", w.workerID),
	)

	// Step 2: Process each event
	var processed, skipped, failed int

	for _, event := range events {
		if err := w.processEvent(ctx, event); err != nil {
			if errors.Is(err, ErrAlreadyProcessed) {
				skipped++
				if w.metrics != nil {
					w.metrics.RecordProjectionEventProcessed(ProjectionResultSkipped)
				}
				continue
			}
			w.log.Error("Failed to process event",
				zap.String("event_id", event.ID.String()),
				zap.String("event_type", event.EventType),
				zap.Error(err),
			)
			failed++
			if w.metrics != nil {
				w.metrics.RecordProjectionEventProcessed(ProjectionResultFailed)
			}
			continue
		}
		processed++
		if w.metrics != nil {
			w.metrics.RecordProjectionEventProcessed(ProjectionResultProcessed)
		}
	}

	if processed > 0 || skipped > 0 || failed > 0 {
		w.log.Info("Projection batch processed",
			zap.Int("processed", processed),
			zap.Int("skipped", skipped),
			zap.Int("failed", failed),
		)
	}
}

// ErrAlreadyProcessed is returned when an event has already been processed
var ErrAlreadyProcessed = errors.New("event already processed")

// fetchUnprocessedEvents fetches succeeded outbox events that haven't been projected yet
func (w *ProjectionWorker) fetchUnprocessedEvents(
	ctx context.Context,
	limit int,
) ([]outboxRepo.Event, error) {
	var events []outboxRepo.Event

	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		// Fetch succeeded events that are not in projection_tracker
		query := `
			SELECT o.id, o.aggregate_type, o.aggregate_id, o.event_type, o.payload,
			       o.status, o.retry_count, o.next_attempt_at, o.created_at, o.updated_at
			FROM outbox o
			WHERE o.status = $1
			  AND NOT EXISTS (
				  SELECT 1 FROM projection_tracker pt
				  WHERE pt.outbox_event_id = o.id
			  )
			ORDER BY o.created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT $2
		`

		rows, err := tx.Query(ctx, query, outboxRepo.StatusSucceeded, limit)
		if err != nil {
			return fmt.Errorf("fetch events failed: %w", err)
		}
		defer rows.Close()

		events, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (outboxRepo.Event, error) {
			var e outboxRepo.Event
			err := row.Scan(
				&e.ID, &e.AggregateType, &e.AggregateID, &e.EventType, &e.Payload,
				&e.Status, &e.RetryCount, &e.NextAttemptAt, &e.CreatedAt, &e.UpdatedAt,
			)
			return e, err
		})

		return err
	})

	return events, err
}

// processEvent processes a single outbox event and updates the appropriate read model
func (w *ProjectionWorker) processEvent(
	ctx context.Context,
	event outboxRepo.Event,
) error {
	return w.db.WithTx(ctx, func(tx db.Tx) error {
		// Double-check if already processed (concurrency safety)
		alreadyProcessed, err := w.projectionRepo.IsEventProcessed(ctx, tx, event.ID)
		if err != nil {
			return fmt.Errorf("check event processed failed: %w", err)
		}
		if alreadyProcessed {
			return ErrAlreadyProcessed
		}

		// Route to appropriate handler based on event type
		// Handlers will RE-QUERY the write model for fresh data
		if err := w.handleEvent(ctx, tx, event); err != nil {
			return fmt.Errorf("handle event failed: %w", err)
		}

		// Mark event as processed
		if err := w.projectionRepo.MarkEventProcessed(ctx, tx, event.ID, event.EventType); err != nil {
			return fmt.Errorf("mark event processed failed: %w", err)
		}

		return nil
	})
}

// handleEvent routes events to the appropriate projection handler
func (w *ProjectionWorker) handleEvent(
	ctx context.Context,
	tx db.Tx,
	event outboxRepo.Event,
) error {
	switch {
	case isOrderEvent(event.EventType):
		return w.handleOrderEvent(ctx, tx, event)

	case isDisputeEvent(event.EventType):
		return w.handleDisputeEvent(ctx, tx, event)

	case isLedgerEvent(event.EventType):
		return w.handleLedgerEvent(ctx, tx, event)

	default:
		// Unknown event type - log but don't fail
		w.log.Debug("Unknown event type for projection, skipping",
			zap.String("event_type", event.EventType),
			zap.String("event_id", event.ID.String()),
		)
		return nil
	}
}

// =============================================================================
// ORDER EVENT HANDLERS
// =============================================================================

// isOrderEvent checks if an event is an order-related event
func isOrderEvent(eventType string) bool {
	switch eventType {
	case events.EventOrderCreated,
	     events.EventOrderPaid,
	     "order.shipped",
	     events.EventOrderCompleted,
	     "order.cancelled",
	     "order.expired",
	     "order.refunded",
	     "order.partially_refunded":
		return true
	default:
		return false
	}
}

// handleOrderEvent processes order events by RE-QUERYING the write model.
// The event's aggregate_id contains the order_id to query.
// Fresh data is fetched from the orders table to ensure accuracy.
func (w *ProjectionWorker) handleOrderEvent(
	ctx context.Context,
	tx db.Tx,
	event outboxRepo.Event,
) error {
	// Extract order_id from event's aggregate_id (the entity being projected)
	orderID := event.AggregateID

	// RE-QUERY write model for fresh data
	// This ensures projection has accurate data regardless of payload version
	var summary projection.OrderSummary

	query := `
		SELECT o.id, o.buyer_id, o.seller_id, o.source_type, o.source_id,
		       o.status, o.escrow_status, o.has_dispute,
		       d.status as dispute_status, d.reason as dispute_reason,
		       d.opened_at as dispute_opened_at, d.resolved_at as dispute_resolved_at,
		       o.subtotal, o.shipping_total, o.commission_amount,
		       o.total_before_coins_amount, o.refunded_amount,
		       o.shipping_option_name, o.shipping_transport_type,
		       o.auto_release_at, o.created_at, o.updated_at
		FROM orders o
		LEFT JOIN disputes d ON d.order_id = o.id
		WHERE o.id = $1
	`

	err := tx.QueryRow(ctx, query, orderID).Scan(
		&summary.ID, &summary.BuyerID, &summary.SellerID, &summary.SourceType, &summary.SourceID,
		&summary.Status, &summary.EscrowStatus, &summary.HasDispute,
		&summary.DisputeStatus, &summary.DisputeReason, &summary.DisputeOpenedAt, &summary.DisputeResolvedAt,
		&summary.Subtotal, &summary.ShippingTotal, &summary.CommissionAmount,
		&summary.EscrowAmount, &summary.RefundedAmount,
		&summary.ShippingSetupName, &summary.ShippingTransportType,
		&summary.AutoReleaseAt, &summary.CreatedAt, &summary.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			// Order may have been deleted - skip projection
			w.log.Debug("Order not found in write model, skipping projection",
				zap.String("order_id", orderID.String()),
			)
			return nil
		}
		return fmt.Errorf("query order from write model failed: %w", err)
	}

	// Upsert to read model (overwrite-based)
	if err := w.projectionRepo.UpsertOrderSummary(ctx, tx, &summary); err != nil {
		return fmt.Errorf("upsert order summary failed: %w", err)
	}

	w.log.Debug("Order summary projected",
		zap.String("order_id", orderID.String()),
		zap.String("event_type", event.EventType),
		zap.String("status", summary.Status),
	)

	return nil
}

// =============================================================================
// DISPUTE EVENT HANDLERS
// =============================================================================

// isDisputeEvent checks if an event is a dispute-related event
func isDisputeEvent(eventType string) bool {
	switch eventType {
	case "dispute.opened", "dispute.resolved":
		return true
	default:
		return false
	}
}

// handleDisputeEvent processes dispute events by RE-QUERYING both disputes and orders.
// The dispute event's aggregate_id contains the dispute_id.
// We need to update the order_summaries table with dispute information.
func (w *ProjectionWorker) handleDisputeEvent(
	ctx context.Context,
	tx db.Tx,
	event outboxRepo.Event,
) error {
	// Extract dispute_id from event's aggregate_id
	disputeID := event.AggregateID

	// RE-QUERY write model for fresh data
	// Join disputes with orders to get full information for order_summaries
	var summary projection.OrderSummary

	query := `
		SELECT o.id, o.buyer_id, o.seller_id, o.source_type, o.source_id,
		       o.status, o.escrow_status, o.has_dispute,
		       d.status as dispute_status, d.reason as dispute_reason,
		       d.opened_at as dispute_opened_at, d.resolved_at as dispute_resolved_at,
		       o.subtotal, o.shipping_total, o.commission_amount,
		       o.total_before_coins_amount, o.refunded_amount,
		       o.shipping_option_name, o.shipping_transport_type,
		       o.auto_release_at, o.created_at, o.updated_at
		FROM disputes d
		INNER JOIN orders o ON o.id = d.order_id
		WHERE d.id = $1
	`

	err := tx.QueryRow(ctx, query, disputeID).Scan(
		&summary.ID, &summary.BuyerID, &summary.SellerID, &summary.SourceType, &summary.SourceID,
		&summary.Status, &summary.EscrowStatus, &summary.HasDispute,
		&summary.DisputeStatus, &summary.DisputeReason, &summary.DisputeOpenedAt, &summary.DisputeResolvedAt,
		&summary.Subtotal, &summary.ShippingTotal, &summary.CommissionAmount,
		&summary.EscrowAmount, &summary.RefundedAmount,
		&summary.ShippingSetupName, &summary.ShippingTransportType,
		&summary.AutoReleaseAt, &summary.CreatedAt, &summary.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			// Dispute or order may have been deleted - skip projection
			w.log.Debug("Dispute or order not found in write model, skipping projection",
				zap.String("dispute_id", disputeID.String()),
			)
			return nil
		}
		return fmt.Errorf("query dispute/order from write model failed: %w", err)
	}

	// Upsert to read model (overwrite-based)
	if err := w.projectionRepo.UpsertOrderSummary(ctx, tx, &summary); err != nil {
		return fmt.Errorf("upsert order summary failed: %w", err)
	}

	w.log.Debug("Order summary projected from dispute event",
		zap.String("dispute_id", disputeID.String()),
		zap.String("order_id", summary.ID.String()),
		zap.String("event_type", event.EventType),
	)

	if summary.DisputeStatus != nil {
		w.log.Debug("Dispute status",
			zap.String("status", *summary.DisputeStatus),
		)
	}

	return nil
}

// =============================================================================
// LEDGER EVENT HANDLERS
// =============================================================================

// isLedgerEvent checks if an event is a ledger-related event
func isLedgerEvent(eventType string) bool {
	switch eventType {
	case "ledger.transaction.completed":
		return true
	default:
		return false
	}
}

// handleLedgerEvent processes ledger events by RE-QUERYING financial_accounts.
// This mirrors the write model account balances without recalculation.
func (w *ProjectionWorker) handleLedgerEvent(
	ctx context.Context,
	tx db.Tx,
	event outboxRepo.Event,
) error {
	// Extract affected account_ids from event payload
	var payload struct {
		AccountIDs []uuid.UUID `json:"account_ids"`
	}

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		// If payload doesn't contain account_ids, try to extract from aggregate_id
		// Some events may only reference a single account
		payload.AccountIDs = []uuid.UUID{event.AggregateID}
	}

	if len(payload.AccountIDs) == 0 {
		return fmt.Errorf("ledger event payload missing account_ids")
	}

	// RE-QUERY each account from financial_accounts (write model)
	// This ensures projection mirrors the actual account balance
	for _, accountID := range payload.AccountIDs {
		var balance projection.AccountBalance

		query := `
			SELECT id, user_id, account_type, balance,
			       COALESCE(currency, 'IDR') as currency,
			       NOW() as updated_at
			FROM financial_accounts
			WHERE id = $1
		`

		err := tx.QueryRow(ctx, query, accountID).Scan(
			&balance.ID, &balance.UserID, &balance.AccountType,
			&balance.Balance, &balance.Currency, &balance.UpdatedAt,
		)

		if err != nil {
			if err == pgx.ErrNoRows {
				// Account may have been deleted - skip
				w.log.Debug("Account not found in write model, skipping projection",
					zap.String("account_id", accountID.String()),
				)
				continue
			}
			return fmt.Errorf("query account from write model failed: %w", err)
		}

		// Upsert to read model (overwrite-based)
		if err := w.projectionRepo.UpsertAccountBalance(ctx, tx, &balance); err != nil {
			return fmt.Errorf("upsert account balance failed for account %s: %w",
				accountID, err)
		}
	}

	w.log.Debug("Account balances projected",
		zap.String("event_id", event.ID.String()),
		zap.Int("accounts_updated", len(payload.AccountIDs)),
	)

	return nil
}

// =============================================================================
// MANUAL PROCESSING
// =============================================================================

// ManualProcess triggers immediate processing of pending events.
// Useful for testing or manual catch-up after downtime.
func (w *ProjectionWorker) ManualProcess(ctx context.Context) error {
	w.processBatch()
	return nil
}

// ReprocessEvent reprocesses a specific event.
// Useful for rebuilding projections after data fix.
func (w *ProjectionWorker) ReprocessEvent(
	ctx context.Context,
	eventID uuid.UUID,
) error {
	return w.db.WithTx(ctx, func(tx db.Tx) error {
		// Fetch the event
		var event outboxRepo.Event
		query := `
			SELECT id, aggregate_type, aggregate_id, event_type, payload,
			       status, retry_count, next_attempt_at, created_at, updated_at
			FROM outbox WHERE id = $1
		`

		err := tx.QueryRow(ctx, query, eventID).Scan(
			&event.ID, &event.AggregateType, &event.AggregateID, &event.EventType, &event.Payload,
			&event.Status, &event.RetryCount, &event.NextAttemptAt, &event.CreatedAt, &event.UpdatedAt,
		)
		if err != nil {
			if err == pgx.ErrNoRows {
				return fmt.Errorf("event not found: %s", eventID)
			}
			return fmt.Errorf("fetch event failed: %w", err)
		}

		// Delete from projection_tracker to allow reprocessing
		_, err = tx.Exec(ctx, `DELETE FROM projection_tracker WHERE outbox_event_id = $1`, eventID)
		if err != nil {
			return fmt.Errorf("delete from tracker failed: %w", err)
		}

		// Process the event (will re-query write model)
		if err := w.handleEvent(ctx, tx, event); err != nil {
			return fmt.Errorf("handle event failed: %w", err)
		}

		// Mark as processed
		if err := w.projectionRepo.MarkEventProcessed(ctx, tx, event.ID, event.EventType); err != nil {
			return fmt.Errorf("mark event processed failed: %w", err)
		}

		w.log.Info("Event reprocessed",
			zap.String("event_id", eventID.String()),
			zap.String("event_type", event.EventType),
		)

		return nil
	})
}

// RebuildAll rebuilds projections from scratch by querying all write model data.
// This is a heavy operation - use sparingly.
//
// RACE CONDITION SAFETY:
// - Records max(outbox.id) BEFORE rebuilding
// - Only marks events ≤ recorded max as processed
// - Events that come in during rebuild will be processed by worker normally
func (w *ProjectionWorker) RebuildAll(ctx context.Context) error {
	w.log.Info("Starting full projection rebuild...")

	// Step 0: Record the max outbox event ID BEFORE any changes
	// This prevents race condition where new events arrive during rebuild
	var maxOutboxID uuid.UUID
	err := w.db.Pool().QueryRow(ctx, `
		SELECT COALESCE(
			(SELECT id FROM outbox WHERE status = 'succeeded' ORDER BY created_at DESC LIMIT 1),
			'00000000-0000-0000-0000-000000000000'::uuid
		)
	`).Scan(&maxOutboxID)
	if err != nil {
		return fmt.Errorf("get max outbox id failed: %w", err)
	}

	w.log.Info("Recorded max outbox ID for rebuild",
		zap.String("max_outbox_id", maxOutboxID.String()),
	)

	// Step 1: Clear projection_tracker
	if err := w.truncateProjectionTracker(ctx); err != nil {
		return fmt.Errorf("clear tracker failed: %w", err)
	}

	// Step 2: Rebuild order_summaries from orders table
	if err := w.rebuildOrderSummaries(ctx); err != nil {
		return fmt.Errorf("rebuild order summaries failed: %w", err)
	}

	// Step 3: Rebuild account_balances from financial_accounts table
	if err := w.rebuildAccountBalances(ctx); err != nil {
		return fmt.Errorf("rebuild account balances failed: %w", err)
	}

	// Step 4: Mark only events EXISTING at rebuild time as processed
	// New events that arrived during rebuild will be processed by worker normally
	if err := w.markOutboxProcessedUpTo(ctx, maxOutboxID); err != nil {
		return fmt.Errorf("mark outbox processed failed: %w", err)
	}

	w.log.Info("Full projection rebuild completed")
	return nil
}

func (w *ProjectionWorker) truncateProjectionTracker(ctx context.Context) error {
	_, err := w.db.Pool().Exec(ctx, `TRUNCATE TABLE projection_tracker`)
	return err
}

func (w *ProjectionWorker) rebuildOrderSummaries(ctx context.Context) error {
	w.log.Info("Rebuilding order_summaries...")

	// Query all orders joined with disputes and insert into order_summaries.
	// LEFT JOIN ensures orders without disputes are included (dispute columns = NULL).
	query := `
		INSERT INTO order_summaries (
			id, buyer_id, seller_id, source_type, source_id,
			status, escrow_status, has_dispute,
			dispute_status, dispute_reason, dispute_opened_at, dispute_resolved_at,
			subtotal, shipping_total, commission_amount,
			escrow_amount, refunded_amount,
			shipping_option_name, shipping_transport_type,
			auto_release_at, created_at, updated_at
		)
		SELECT o.id, o.buyer_id, o.seller_id, o.source_type, o.source_id,
		       o.status, o.escrow_status, o.has_dispute,
		       d.status, d.reason, d.opened_at, d.resolved_at,
		       o.subtotal, o.shipping_total, o.commission_amount,
		       o.total_before_coins_amount, o.refunded_amount,
		       o.shipping_option_name, o.shipping_transport_type,
		       o.auto_release_at, o.created_at, o.updated_at
		FROM orders o
		LEFT JOIN disputes d ON d.order_id = o.id
		ON CONFLICT (id) DO UPDATE SET
			buyer_id = EXCLUDED.buyer_id,
			seller_id = EXCLUDED.seller_id,
			source_type = EXCLUDED.source_type,
			source_id = EXCLUDED.source_id,
			status = EXCLUDED.status,
			escrow_status = EXCLUDED.escrow_status,
			has_dispute = EXCLUDED.has_dispute,
			dispute_status = EXCLUDED.dispute_status,
			dispute_reason = EXCLUDED.dispute_reason,
			dispute_opened_at = EXCLUDED.dispute_opened_at,
			dispute_resolved_at = EXCLUDED.dispute_resolved_at,
			subtotal = EXCLUDED.subtotal,
			shipping_total = EXCLUDED.shipping_total,
			commission_amount = EXCLUDED.commission_amount,
			escrow_amount = EXCLUDED.escrow_amount,
			refunded_amount = EXCLUDED.refunded_amount,
			shipping_option_name = EXCLUDED.shipping_option_name,
			shipping_transport_type = EXCLUDED.shipping_transport_type,
			auto_release_at = EXCLUDED.auto_release_at,
			updated_at = EXCLUDED.updated_at
	`

	_, err := w.db.Pool().Exec(ctx, query)
	return err
}

func (w *ProjectionWorker) rebuildAccountBalances(ctx context.Context) error {
	w.log.Info("Rebuilding account_balances...")

	query := `
		INSERT INTO account_balances (
			id, user_id, account_type, balance, currency, updated_at
		)
		SELECT id, user_id, account_type, balance,
		       COALESCE(currency, 'IDR') as currency,
		       NOW() as updated_at
		FROM financial_accounts
		ON CONFLICT (id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			account_type = EXCLUDED.account_type,
			balance = EXCLUDED.balance,
			currency = EXCLUDED.currency,
			updated_at = EXCLUDED.updated_at
	`

	_, err := w.db.Pool().Exec(ctx, query)
	return err
}

func (w *ProjectionWorker) markCurrentOutboxProcessed(ctx context.Context) error {
	w.log.Info("Marking current outbox events as processed...")

	query := `
		INSERT INTO projection_tracker (outbox_event_id, event_type, processed_at)
		SELECT id, event_type, NOW()
		FROM outbox
		WHERE status = 'succeeded'
		ON CONFLICT (outbox_event_id) DO NOTHING
	`

	_, err := w.db.Pool().Exec(ctx, query)
	return err
}

// markOutboxProcessedUpTo marks only events with id <= maxID as processed.
// This prevents race condition where new events arrive during rebuild.
func (w *ProjectionWorker) markOutboxProcessedUpTo(ctx context.Context, maxID uuid.UUID) error {
	w.log.Info("Marking outbox events as processed",
		zap.String("up_to_id", maxID.String()),
	)

	query := `
		INSERT INTO projection_tracker (outbox_event_id, event_type, processed_at)
		SELECT id, event_type, NOW()
		FROM outbox
		WHERE status = 'succeeded'
		  AND id <= $1
		ON CONFLICT (outbox_event_id) DO NOTHING
	`

	_, err := w.db.Pool().Exec(ctx, query, maxID)
	return err
}

// GetProjectionStatus returns the current status of the projection
func (w *ProjectionWorker) GetProjectionStatus(
	ctx context.Context,
) (*ProjectionStatus, error) {
	var status ProjectionStatus

	// Get last processed event time
	err := w.db.Pool().QueryRow(ctx, `
		SELECT COALESCE(MAX(processed_at), '1970-01-01'::timestamptz)
		FROM projection_tracker
	`).Scan(&status.LastProcessedAt)
	if err != nil {
		return nil, fmt.Errorf("get last processed time failed: %w", err)
	}

	// Count unprocessed events
	err = w.db.Pool().QueryRow(ctx, `
		SELECT COUNT(*)
		FROM outbox o
		WHERE o.status = $1
		  AND NOT EXISTS (
			  SELECT 1 FROM projection_tracker pt
			  WHERE pt.outbox_event_id = o.id
		  )
	`, outboxRepo.StatusSucceeded).Scan(&status.PendingCount)
	if err != nil {
		return nil, fmt.Errorf("get pending count failed: %w", err)
	}

	// Count total processed
	err = w.db.Pool().QueryRow(ctx, `
		SELECT COUNT(*) FROM projection_tracker
	`).Scan(&status.ProcessedCount)
	if err != nil {
		return nil, fmt.Errorf("get processed count failed: %w", err)
	}

	// Count projection table sizes
	err = w.db.Pool().QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM order_summaries) as orders,
			(SELECT COUNT(*) FROM account_balances) as accounts
	`).Scan(&status.OrderCount, &status.AccountCount)
	if err != nil {
		return nil, fmt.Errorf("get projection counts failed: %w", err)
	}

	return &status, nil
}

// ProjectionStatus represents the current status of the projection
type ProjectionStatus struct {
	LastProcessedAt time.Time
	PendingCount    int
	ProcessedCount  int
	OrderCount      int
	AccountCount    int
}


