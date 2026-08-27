package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/identity/auth"
	orderApp "github.com/labuda/backend/internal/commerce/order/application"
	orderRepo "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	orderrepository "github.com/labuda/backend/internal/commerce/order/repository"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

const (
	// DefaultOverdueCancelPollInterval is how often the worker checks for orders to auto-cancel
	DefaultOverdueCancelPollInterval = 1 * time.Hour

	// DefaultOverdueCancelBatchSize is max orders to process per batch
	DefaultOverdueCancelBatchSize = 50
)

// OrderOverdueCancelWorker automatically cancels orders that exceed the shipment deadline.
//
// 🔥 PHASE 2: AUTO-CANCEL (CRITICAL)
//
// This worker enforces the fulfillment deadline by auto-cancelling orders
// that have not been shipped within ReadyToShipBy + grace period.
//
// BUSINESS RULE:
// - Orders with status = PAID
// - Where ready_to_ship_by + grace_period < NOW()
// - Auto-cancel and refund escrow to buyer
//
// SAFETY:
// - Atomic transactions (cancel + refund)
// - Idempotent (won't double refund)
// - Only processes paid orders with escrow in holding state
//
// This prevents orders from being stuck indefinitely and protects buyers from
// sellers who never ship.
type OrderOverdueCancelWorker struct {
	db           Transactor
	orderService *orderApp.OrderService
	orderRepo    orderrepository.OrderRepository
	log          *zap.Logger
	pollInterval time.Duration
	batchSize    int
	workerID     string // Unique identifier for this worker instance

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

	// Context for shutdown
	shutdownCtx context.Context
	cancelFn    context.CancelFunc
}

// OrderOverdueCancelConfig holds worker configuration
type OrderOverdueCancelConfig struct {
	PollInterval time.Duration // How often to check for overdue orders
	BatchSize    int           // Max orders to process per batch
}

// DefaultOrderOverdueCancelConfig returns default configuration
func DefaultOrderOverdueCancelConfig() OrderOverdueCancelConfig {
	return OrderOverdueCancelConfig{
		PollInterval: DefaultOverdueCancelPollInterval,
		BatchSize:    DefaultOverdueCancelBatchSize,
	}
}

// NewOrderOverdueCancelWorker creates a new order overdue cancel worker
func NewOrderOverdueCancelWorker(
	db Transactor,
	orderService *orderApp.OrderService,
	log *zap.Logger,
	cfg OrderOverdueCancelConfig,
) *OrderOverdueCancelWorker {
	if log == nil {
		log = zap.NewNop()
	}

	if cfg.PollInterval == 0 {
		cfg.PollInterval = DefaultOverdueCancelPollInterval
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = DefaultOverdueCancelBatchSize
	}

	// Generate unique worker ID for logging
	workerID := uuid.New().String()[:8]

	return &OrderOverdueCancelWorker{
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

// Start begins processing overdue orders in the background
func (w *OrderOverdueCancelWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("Order overdue cancel worker already running",
			zap.String("worker_id", w.workerID),
		)
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.stopCh = make(chan struct{})

	w.wg.Add(1)
	go w.run()

	w.log.Info("Order overdue cancel worker started",
		zap.String("worker_id", w.workerID),
		zap.Duration("poll_interval", w.pollInterval),
		zap.Int("batch_size", w.batchSize),
	)
}

// Stop gracefully shuts down the worker
func (w *OrderOverdueCancelWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.log.Info("Stopping order overdue cancel worker...",
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
		w.log.Info("Order overdue cancel worker stopped gracefully",
			zap.String("worker_id", w.workerID),
		)
	case <-time.After(10 * time.Second):
		w.log.Warn("Order overdue cancel worker shutdown timeout",
			zap.String("worker_id", w.workerID),
		)
	}

	w.running = false
}

// IsRunning returns true if the worker is currently running
func (w *OrderOverdueCancelWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// run is the main worker loop
func (w *OrderOverdueCancelWorker) run() {
	defer w.wg.Done()

	// Initial processing on startup
	w.processOverdueOrders()

	for {
		select {
		case <-w.shutdownCtx.Done():
			w.log.Info("Worker shutdown requested",
				zap.String("worker_id", w.workerID),
			)
			return

		case <-time.After(w.pollInterval):
			w.processOverdueOrders()

		case <-w.stopCh:
			return
		}
	}
}

// processOverdueOrders finds and cancels orders that are overdue for shipment.
// Uses FOR UPDATE SKIP LOCKED for concurrent worker support.
func (w *OrderOverdueCancelWorker) processOverdueOrders() {
	start := time.Now()
	ctx := context.Background()

	// Find orders that are overdue for shipment
	orderIDs, err := w.findOverdueOrders(ctx, w.batchSize)
	if err != nil {
		w.log.Error("worker_error",
			zap.String("worker", "order_overdue_cancel"),
			zap.String("worker_id", w.workerID),
			zap.Error(err),
		)
		return
	}

	if len(orderIDs) == 0 {
		return
	}

	w.log.Info("Processing overdue orders for cancellation",
		zap.String("worker_id", w.workerID),
		zap.Int("count", len(orderIDs)),
	)

	// Process each order in its own transaction for isolation
	var processed, skipped, errors int

	for _, orderID := range orderIDs {
		// Enforce max loop duration - break if exceeded
		if time.Since(start) > maxWorkerLoopDuration {
			w.log.Info("worker_loop_max_duration_reached",
				zap.String("worker", "order_overdue_cancel"),
				zap.String("worker_id", w.workerID),
				zap.Duration("duration", time.Since(start)),
				zap.Int("remaining", len(orderIDs)-processed-skipped-errors),
			)
			break
		}

		result, err := w.processOrder(ctx, orderID)
		if err != nil {
			w.log.Error("worker_error",
				zap.String("worker", "order_overdue_cancel"),
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
		zap.String("worker", "order_overdue_cancel"),
		zap.String("worker_id", w.workerID),
		zap.Int("batch_size", w.batchSize),
		zap.Int("processed_count", processed),
		zap.Int("skipped_count", skipped),
		zap.Int("errors", errors),
		zap.Int("duration_ms", durationMs),
	)
}

// findOverdueOrders returns IDs of orders that are overdue for shipment.
// Uses repository method which uses FOR UPDATE SKIP LOCKED to support concurrent workers.
//
// Query conditions (in repository):
// - status = 'paid'
// - escrow_status = 'holding'
// - ready_to_ship_by IS NOT NULL
// - ready_to_ship_by + grace_period < NOW()
func (w *OrderOverdueCancelWorker) findOverdueOrders(
	ctx context.Context,
	limit int,
) ([]uuid.UUID, error) {
	var orderIDs []uuid.UUID

	// Use repository method to fetch order IDs with locking
	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		orderIDs, err = w.orderRepo.FindOverdueOrdersForCancel(ctx, tx, limit)
		return err
	})

	return orderIDs, err
}

// processOrder processes a single order for overdue cancellation.
// Returns result status: "success", "skip", or error.
// Runs within its own transaction with db.WithTx for retry support.
//
// CRITICAL: This operation is atomic - both order cancellation and escrow refund
// happen within the same transaction. This prevents the "cancelled + escrow held" state.
func (w *OrderOverdueCancelWorker) processOrder(ctx context.Context, orderID uuid.UUID) (string, error) {
	var result string
	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		// Get order first to check current state
		order, err := w.orderRepo.GetByID(ctx, tx, orderID)
		if err != nil {
			return fmt.Errorf("failed to get order: %w", err)
		}

		// Check if still eligible (might have been shipped/cancelled since query)
		if order.Status != "paid" {
			result = "skip"
			return nil // Order already processed, skip
		}

		// CancelOverdue: gateway refund + escrow flip + coins refund + cancelled_timeout status.
		// System caller ID bypasses ownership check for auto-cancellation.
		if err := w.orderService.CancelOverdue(ctx, tx, orderID, "worker_overdue_cancel_"+w.workerID, auth.SystemCallerID); err != nil {
			return fmt.Errorf("failed to cancel overdue order: %w", err)
		}

		result = "success"

		w.log.Debug("Order auto-cancelled due to shipment timeout",
			zap.String("worker_id", w.workerID),
			zap.String("order_id", orderID.String()),
		)

		return nil
	})

	return result, err
}

// ManualProcess triggers immediate processing of overdue orders.
// Useful for testing or manual intervention.
func (w *OrderOverdueCancelWorker) ManualProcess(ctx context.Context) error {
	orderIDs, err := w.findOverdueOrders(ctx, w.batchSize)
	if err != nil {
		return fmt.Errorf("failed to find overdue orders: %w", err)
	}

	if len(orderIDs) == 0 {
		w.log.Info("No overdue orders found",
			zap.String("worker_id", w.workerID),
		)
		return nil
	}

	w.log.Info("Manual overdue order processing",
		zap.String("worker_id", w.workerID),
		zap.Int("count", len(orderIDs)),
	)

	for _, orderID := range orderIDs {
		if _, err := w.processOrder(ctx, orderID); err != nil {
			w.log.Error("Failed to auto-cancel order",
				zap.String("worker_id", w.workerID),
				zap.String("order_id", orderID.String()),
				zap.Error(err),
			)
		}
	}

	return nil
}


