package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	orderApp "github.com/labuda/backend/internal/commerce/order/application"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

const (
	// DefaultPaymentTimeoutPollInterval is how often the worker checks for
	// orphan pending_payment orders (no payment row ever created).
	DefaultPaymentTimeoutPollInterval = 2 * time.Minute

	// DefaultPaymentTimeoutBatchSize is max orders to process per poll.
	DefaultPaymentTimeoutBatchSize = 50
)

// OrderPaymentTimeoutWorker expires orders stuck in pending_payment after
// payment_expires_at when NO payment row was ever created by the buyer.
//
// BACKGROUND: The existing PaymentExpiryWorker scans the payments table for
// expired pending payments. If a buyer creates an order but never calls the
// payment endpoint, no payment row exists and PaymentExpiryWorker cannot
// detect it. This worker closes that gap by scanning the orders table directly.
//
// DESIGN:
//   - Scans orders WHERE status='pending_payment' AND payment_expires_at <= NOW()
//     AND no active payment row exists.
//   - Delegates to OrderService.Expire() for all business logic (stock restore,
//     coins refund, outbox events). Does NOT duplicate any business logic.
//   - Uses FOR UPDATE SKIP LOCKED for concurrent-safe batch processing.
//   - Idempotent: OrderService.Expire() validates status transition internally.
//
// COMPLEMENTARY: PaymentExpiryWorker remains enabled and owns the path where
// a payment row exists. This worker owns the path where no payment row exists.
type OrderPaymentTimeoutWorker struct {
	db           Transactor
	orderService *orderApp.OrderService
	log          *zap.Logger
	pollInterval time.Duration
	batchSize    int

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

	shutdownCtx context.Context
	cancelFn    context.CancelFunc
}

// OrderPaymentTimeoutConfig holds worker configuration.
type OrderPaymentTimeoutConfig struct {
	PollInterval time.Duration
	BatchSize    int
}

// DefaultOrderPaymentTimeoutConfig returns default configuration.
func DefaultOrderPaymentTimeoutConfig() OrderPaymentTimeoutConfig {
	return OrderPaymentTimeoutConfig{
		PollInterval: DefaultPaymentTimeoutPollInterval,
		BatchSize:    DefaultPaymentTimeoutBatchSize,
	}
}

// NewOrderPaymentTimeoutWorker creates a new worker.
//
// orderService must be the fully-wired OrderService from dependencies.go.
// The worker delegates Expire() to OrderService which handles stock restore,
// coins refund, outbox events, and shipping quote reactivation.
func NewOrderPaymentTimeoutWorker(
	db Transactor,
	orderService *orderApp.OrderService,
	log *zap.Logger,
	cfg OrderPaymentTimeoutConfig,
) *OrderPaymentTimeoutWorker {
	if log == nil {
		log = zap.NewNop()
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = DefaultPaymentTimeoutPollInterval
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = DefaultPaymentTimeoutBatchSize
	}

	return &OrderPaymentTimeoutWorker{
		db:           db,
		orderService: orderService,
		log:          log,
		pollInterval: cfg.PollInterval,
		batchSize:    cfg.BatchSize,
		stopCh:       make(chan struct{}),
	}
}

// Start begins the background polling loop.
func (w *OrderPaymentTimeoutWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("OrderPaymentTimeoutWorker already running")
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.stopCh = make(chan struct{})

	w.wg.Add(1)
	go w.run()

	w.log.Info("OrderPaymentTimeoutWorker started",
		zap.Duration("poll_interval", w.pollInterval),
		zap.Int("batch_size", w.batchSize),
	)
}

// Stop gracefully shuts down the worker.
func (w *OrderPaymentTimeoutWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.log.Info("Stopping OrderPaymentTimeoutWorker...")

	w.cancelFn()
	close(w.stopCh)

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.log.Info("OrderPaymentTimeoutWorker stopped gracefully")
	case <-time.After(10 * time.Second):
		w.log.Warn("OrderPaymentTimeoutWorker shutdown timeout")
	}

	w.running = false
}

// IsRunning returns true if the worker is currently running.
func (w *OrderPaymentTimeoutWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

func (w *OrderPaymentTimeoutWorker) run() {
	defer w.wg.Done()

	w.checkOrphanOrders()

	for {
		select {
		case <-w.shutdownCtx.Done():
			w.log.Info("OrderPaymentTimeoutWorker shutdown requested")
			return
		case <-time.After(w.pollInterval):
			w.checkOrphanOrders()
		case <-w.stopCh:
			return
		}
	}
}

// checkOrphanOrders finds and expires pending_payment orders with no active
// payment row whose payment window has closed.
//
// Two-phase processing:
// Phase 1: Short transaction to fetch candidate IDs with SKIP LOCKED.
// Phase 2: Process each order in its own transaction for failure isolation.
func (w *OrderPaymentTimeoutWorker) checkOrphanOrders() {
	ctx := context.Background()

	orderIDs, err := w.findOrphanExpiredOrderIDs(ctx, w.batchSize)
	if err != nil {
		w.log.Error("Failed to find orphan expired orders", zap.Error(err))
		return
	}

	if len(orderIDs) == 0 {
		return
	}

	w.log.Info("Found orphan expired orders", zap.Int("count", len(orderIDs)))

	for _, orderID := range orderIDs {
		if err := w.expireOrphanOrder(ctx, orderID); err != nil {
			w.log.Error("Failed to expire orphan order",
				zap.String("order_id", orderID.String()),
				zap.Error(err),
			)
		}
	}
}

// findOrphanExpiredOrderIDs returns IDs of pending_payment orders whose
// payment_expires_at has passed and that have no active payment row.
//
// The NOT EXISTS subquery ensures this worker does NOT process orders that
// already have a payment row — those are owned by PaymentExpiryWorker.
func (w *OrderPaymentTimeoutWorker) findOrphanExpiredOrderIDs(
	ctx context.Context,
	limit int,
) ([]uuid.UUID, error) {
	var orderIDs []uuid.UUID

	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		query := `
			SELECT o.id
			FROM orders o
			WHERE o.status = 'pending_payment'
			  AND o.payment_expires_at <= NOW()
			  AND NOT EXISTS (
			      SELECT 1 FROM payments p
			      WHERE p.reference_type = 'order'
			        AND p.reference_id = o.id
			        AND p.status IN ('pending', 'settlement', 'capture')
			  )
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		`

		rows, err := tx.Query(ctx, query, limit)
		if err != nil {
			return fmt.Errorf("query orphan expired orders failed: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				return fmt.Errorf("scan order id failed: %w", err)
			}
			orderIDs = append(orderIDs, id)
		}

		return rows.Err()
	})

	return orderIDs, err
}

// expireOrphanOrder expires a single orphan order within its own transaction.
// Delegates entirely to OrderService.Expire() for business logic.
func (w *OrderPaymentTimeoutWorker) expireOrphanOrder(
	ctx context.Context,
	orderID uuid.UUID,
) error {
	return w.db.WithTx(ctx, func(tx db.Tx) error {
		if err := w.orderService.Expire(ctx, tx, orderID); err != nil {
			// OrderService.Expire() returns error if order cannot transition
			// (e.g. already expired, paid, cancelled). This is expected for
			// idempotent re-processing and race conditions with PaymentExpiryWorker.
			w.log.Debug("Orphan order expiry skipped (invalid transition)",
				zap.String("order_id", orderID.String()),
				zap.Error(err),
			)
			return nil
		}

		w.log.Info("Orphan order expired (no payment row)",
			zap.String("order_id", orderID.String()),
		)
		return nil
	})
}


