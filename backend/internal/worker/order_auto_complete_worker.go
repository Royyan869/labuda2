package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	orderApp "github.com/labuda/backend/internal/commerce/order/application"
	orderRepo "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	orderrepository "github.com/labuda/backend/internal/commerce/order/repository"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

const (
	// DefaultAutoCompletePollInterval is how often the worker checks for orders to auto-complete
	DefaultAutoCompletePollInterval = 1 * time.Minute

	// DefaultAutoCompleteBatchSize is max orders to process per batch
	DefaultAutoCompleteBatchSize = 50
)

// OrderAutoCompleteWorker automatically completes orders after the auto-release deadline.
//
// BUSINESS RULE: Auto-complete timer starts when seller marks order as shipped.
// Buyer has 5 days to confirm or dispute.
//
// Buyer may extend once (+3 days) near expiry.
//
// CRITICAL SAFETY GUARDS (multi-layer protection):
//
// LAYER 1 - Database Query (repository):
// - has_dispute = false (excludes disputed orders at DB level)
// - escrow_status = 'holding' (only releasable escrows)
// - status IN ('shipped', 'delivered') (timer starts at shipped)
//
// LAYER 2 - Entity Guards (order.Complete):
// - Returns error if Status not in ("shipped", "delivered")
// - Returns error if HasDispute = true (DisputeActiveError)
// - Returns error if EscrowStatus != "holding" (InvalidEscrowStatusError)
//
// LAYER 3 - Service Idempotency (OrderCompletionService.Complete):
// - Returns success if already completed (no-op on re-execution)
// - Ledger idempotency key "order_release_<order_id>" prevents double-release
//
// This multi-layer approach prevents auto-completing disputed orders even under race conditions.
type OrderAutoCompleteWorker struct {
	db           Transactor
	orderService *orderApp.OrderService
	orderRepo    orderrepository.OrderRepository
	log          *zap.Logger
	pollInterval time.Duration
	batchSize    int
	workerID     string // Unique identifier for this worker instance

	metrics WorkerLivenessRecorder // optional sink; nil = no-op

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

	// Context for shutdown
	shutdownCtx context.Context
	cancelFn    context.CancelFunc
}

// OrderAutoCompleteConfig holds worker configuration
type OrderAutoCompleteConfig struct {
	PollInterval time.Duration // How often to check for completable orders
	BatchSize    int           // Max orders to process per batch
}

// DefaultOrderAutoCompleteConfig returns default configuration
func DefaultOrderAutoCompleteConfig() OrderAutoCompleteConfig {
	return OrderAutoCompleteConfig{
		PollInterval: DefaultAutoCompletePollInterval,
		BatchSize:    DefaultAutoCompleteBatchSize,
	}
}

// NewOrderAutoCompleteWorker creates a new order auto-complete worker
func NewOrderAutoCompleteWorker(
	db Transactor,
	orderService *orderApp.OrderService,
	log *zap.Logger,
	cfg OrderAutoCompleteConfig,
) *OrderAutoCompleteWorker {
	if log == nil {
		log = zap.NewNop()
	}

	if cfg.PollInterval == 0 {
		cfg.PollInterval = DefaultAutoCompletePollInterval
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = DefaultAutoCompleteBatchSize
	}

	// Generate unique worker ID for logging
	workerID := uuid.New().String()[:8]

	return &OrderAutoCompleteWorker{
		db:           db,
		orderService: orderService,
		orderRepo:    orderRepo.NewOrderRepository(),
		log:          log,
		pollInterval: cfg.PollInterval,
		batchSize:    cfg.BatchSize,
		workerID:     workerID,
		stopCh:       make(chan struct{}),
	}
}

// SetMetricsRecorder attaches an optional liveness sink. Must be called before
// Start(). The recorder is sink-only and never influences completion decisions.
func (w *OrderAutoCompleteWorker) SetMetricsRecorder(r WorkerLivenessRecorder) {
	w.metrics = r
}

// Start begins processing auto-complete orders in the background
func (w *OrderAutoCompleteWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("Order auto-complete worker already running",
			zap.String("worker_id", w.workerID),
		)
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.stopCh = make(chan struct{})

	if w.metrics != nil {
		w.metrics.SetWorkerRunning(WorkerNameOrderAutoComplete, true)
	}

	w.wg.Add(1)
	go w.run()

	w.log.Info("Order auto-complete worker started",
		zap.String("worker_id", w.workerID),
		zap.Duration("poll_interval", w.pollInterval),
		zap.Int("batch_size", w.batchSize),
	)
}

// Stop gracefully shuts down the worker
func (w *OrderAutoCompleteWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.log.Info("Stopping order auto-complete worker...",
		zap.String("worker_id", w.workerID),
	)

	w.cancelFn()
	close(w.stopCh)

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.log.Info("Order auto-complete worker stopped gracefully",
			zap.String("worker_id", w.workerID),
		)
	case <-time.After(10 * time.Second):
		w.log.Warn("Order auto-complete worker shutdown timeout",
			zap.String("worker_id", w.workerID),
		)
	}

	w.running = false
	if w.metrics != nil {
		w.metrics.SetWorkerRunning(WorkerNameOrderAutoComplete, false)
	}
}

// IsRunning returns true if the worker is currently running
func (w *OrderAutoCompleteWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// run is the main worker loop
func (w *OrderAutoCompleteWorker) run() {
	defer w.wg.Done()

	// Initial processing on startup
	w.processAutoCompleteOrders()

	for {
		select {
		case <-w.shutdownCtx.Done():
			w.log.Info("Worker shutdown requested",
				zap.String("worker_id", w.workerID),
			)
			return

		case <-time.After(w.pollInterval):
			w.processAutoCompleteOrders()

		case <-w.stopCh:
			return
		}
	}
}

// processAutoCompleteOrders finds and completes orders that are due for auto-completion.
// Uses FOR UPDATE SKIP LOCKED for concurrent worker support.
func (w *OrderAutoCompleteWorker) processAutoCompleteOrders() {
	start := time.Now()
	ctx := context.Background()

	// Find orders due for auto-completion
	orderIDs, err := w.findOrdersForAutoComplete(ctx, w.batchSize)
	if err != nil {
		w.log.Error("worker_error",
			zap.String("worker", "order_auto_complete"),
			zap.String("worker_id", w.workerID),
			zap.Error(err),
		)
		return
	}

	if len(orderIDs) == 0 {
		return
	}

	w.log.Info("Processing auto-complete orders",
		zap.String("worker_id", w.workerID),
		zap.Int("count", len(orderIDs)),
	)

	// Process each order in its own transaction for isolation
	var processed, skipped, errors int

	for _, orderID := range orderIDs {
		// Enforce max loop duration - break if exceeded
		if time.Since(start) > maxWorkerLoopDuration {
			w.log.Info("worker_loop_max_duration_reached",
				zap.String("worker", "order_auto_complete"),
				zap.String("worker_id", w.workerID),
				zap.Duration("duration", time.Since(start)),
				zap.Int("remaining", len(orderIDs)-processed-skipped-errors),
			)
			break
		}

		result, err := w.processOrder(ctx, orderID)
		if err != nil {
			w.log.Error("worker_error",
				zap.String("worker", "order_auto_complete"),
				zap.String("worker_id", w.workerID),
				zap.String("order_id", orderID.String()),
				zap.Error(err),
			)
			errors++
		} else {
			switch result {
			case "success":
				processed++
			case "skip":
				skipped++
			}
		}
	}

	// Log worker run metrics
	durationMs := int(time.Since(start).Milliseconds())
	w.log.Info("worker_run",
		zap.String("worker", "order_auto_complete"),
		zap.String("worker_id", w.workerID),
		zap.Int("batch_size", w.batchSize),
		zap.Int("processed_count", processed),
		zap.Int("skipped_count", skipped),
		zap.Int("errors", errors),
		zap.Int("duration_ms", durationMs),
	)

	// Heartbeat after a successful polling cycle (order ID fetch succeeded).
	// Intentionally placed here (not defer) so a fetch error that returns
	// early does NOT advance the heartbeat.
	if w.metrics != nil {
		w.metrics.RecordWorkerHeartbeat(WorkerNameOrderAutoComplete)
	}
}

// findOrdersForAutoComplete returns IDs of orders that are due for auto-completion.
// Uses repository method which uses FOR UPDATE SKIP LOCKED to support concurrent workers.
//
// Query conditions (in repository):
// - status IN ('shipped', 'delivered')
// - escrow_status = 'holding'
// - has_dispute = false (CRITICAL SAFETY - prevents race with dispute creation)
// - auto_release_at <= NOW()
func (w *OrderAutoCompleteWorker) findOrdersForAutoComplete(
	ctx context.Context,
	limit int,
) ([]uuid.UUID, error) {
	var orderIDs []uuid.UUID

	// Use repository method to fetch order IDs with locking
	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		orderIDs, err = w.orderRepo.FindOrdersForAutoComplete(ctx, tx, limit)
		return err
	})

	return orderIDs, err
}

// processOrder processes a single order for auto-completion.
// Returns result status: "success", "skip", or error.
// Runs within its own transaction with db.WithTx for retry support.
//
// MULTI-LAYER SAFETY (defended in depth):
// 1. Query layer: Disputed orders already excluded by has_dispute = false
// 2. Entity layer: order.Complete() checks HasDispute and EscrowStatus
// 3. Service layer: Complete() is idempotent - returns success if already completed
//
// System caller ID bypasses ownership check for auto-completion.
func (w *OrderAutoCompleteWorker) processOrder(ctx context.Context, orderID uuid.UUID) (string, error) {
	var result string
	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		// Rollout-visibility snapshot at pickup time. Non-authoritative — the
		// canonical decision still lives in OrderCompletionService.Complete,
		// which re-locks the row via GetForUpdate. Logged once per pickup,
		// not on every tick, so this is not metrics spam.
		if order, err := w.orderRepo.GetByID(ctx, tx, orderID); err == nil && order != nil {
			autoReleaseAt := ""
			if order.AutoReleaseAt != nil {
				autoReleaseAt = order.AutoReleaseAt.UTC().Format(time.RFC3339Nano)
			}
			w.log.Info("auto_release_eligible",
				zap.String("worker", "order_auto_complete"),
				zap.String("worker_id", w.workerID),
				zap.String("order_id", orderID.String()),
				zap.String("auto_release_at", autoReleaseAt),
				zap.String("escrow_status", string(order.EscrowStatus)),
				zap.String("order_status", string(order.Status)),
				zap.Bool("has_dispute", order.HasDispute),
			)
		}

		// OrderService.Complete includes all safety guards:
		// - LAYER 1: Query already excluded has_dispute = true orders
		// - LAYER 2: order.Complete() checks HasDispute and EscrowStatus
		// - LAYER 3: Service returns success if already completed (idempotent)
		//
		// System caller ID bypasses ownership check for auto-completion
		// B4A: Empty idempotency key for system caller (uses ledger-level idempotency)
		if err := w.orderService.Complete(ctx, tx, auth.SystemCallerID, orderID, ""); err != nil {
			return fmt.Errorf("failed to complete order: %w", err)
		}

		result = "success"

		w.log.Debug("Order auto-completed successfully",
			zap.String("worker_id", w.workerID),
			zap.String("order_id", orderID.String()),
		)

		return nil
	})

	return result, err
}

// ManualProcess triggers immediate processing of orders due for auto-completion.
// Useful for testing or manual intervention.
func (w *OrderAutoCompleteWorker) ManualProcess(ctx context.Context) error {
	orderIDs, err := w.findOrdersForAutoComplete(ctx, w.batchSize)
	if err != nil {
		return fmt.Errorf("failed to find orders for auto-complete: %w", err)
	}

	if len(orderIDs) == 0 {
		w.log.Info("No orders found for auto-complete",
			zap.String("worker_id", w.workerID),
		)
		return nil
	}

	w.log.Info("Manual auto-complete processing",
		zap.String("worker_id", w.workerID),
		zap.Int("count", len(orderIDs)),
	)

	for _, orderID := range orderIDs {
		if _, err := w.processOrder(ctx, orderID); err != nil {
			w.log.Error("Failed to auto-complete order",
				zap.String("worker_id", w.workerID),
				zap.String("order_id", orderID.String()),
				zap.Error(err),
			)
		}
	}

	return nil
}


