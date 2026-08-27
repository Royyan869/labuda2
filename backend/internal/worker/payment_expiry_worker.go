package worker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	orderApp "github.com/labuda/backend/internal/commerce/order/application"
	orderEntity "github.com/labuda/backend/internal/commerce/order/entity"
	paymentRepo "github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// Transactor defines the interface for executing transactions.
// This allows the worker to work with both real DB and mocks.
type Transactor interface {
	WithTx(ctx context.Context, fn func(tx db.Tx) error) error
}

const (
	// DefaultPollInterval is how often the worker checks for expired payments
	DefaultPollInterval = 1 * time.Minute

	// DefaultBatchSize is max expired payments to process per poll
	DefaultBatchSize = 100
)

// PaymentExpiryWorker detects and marks expired pending payments.
// It processes each payment atomically within a transaction using SKIP LOCKED
// for concurrent-safe batch processing.
type PaymentExpiryWorker struct {
	db           Transactor
	paymentRepo  *paymentRepo.PaymentRepository
	orderService *orderApp.OrderService
	log          *zap.Logger
	pollInterval time.Duration
	batchSize    int

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

	// Context for shutdown
	shutdownCtx context.Context
	cancelFn    context.CancelFunc
}

// Config holds worker configuration
type Config struct {
	PollInterval time.Duration // How often to check for expired payments
	BatchSize    int           // Max payments to process per poll
}

// DefaultConfig returns default configuration
func DefaultConfig() Config {
	return Config{
		PollInterval: DefaultPollInterval,
		BatchSize:    DefaultBatchSize,
	}
}

// NewPaymentExpiryWorker creates a new payment expiry worker.
//
// orderService must be the fully-wired OrderService from dependencies.go
// (with walletService, coinsService, etc. non-nil). The worker delegates
// Expire() to OrderService which needs these deps for gateway-funded refund.
func NewPaymentExpiryWorker(
	db Transactor,
	orderService *orderApp.OrderService,
	log *zap.Logger,
	cfg Config,
) *PaymentExpiryWorker {
	if log == nil {
		log = zap.NewNop()
	}

	if cfg.PollInterval == 0 {
		cfg.PollInterval = DefaultPollInterval
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = DefaultBatchSize
	}

	return &PaymentExpiryWorker{
		db:           db,
		paymentRepo:  paymentRepo.NewPaymentRepository(),
		orderService: orderService,
		log:          log,
		pollInterval: cfg.PollInterval,
		batchSize:    cfg.BatchSize,
		stopCh:       make(chan struct{}),
	}
}

// Start begins detecting expired payments in the background
func (w *PaymentExpiryWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("Payment expiry worker already running")
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.stopCh = make(chan struct{}) // Always create a new stopCh

	w.wg.Add(1)
	go w.run()

	w.log.Info("Payment expiry worker started",
		zap.Duration("poll_interval", w.pollInterval),
		zap.Int("batch_size", w.batchSize),
	)
}

// Stop gracefully shuts down the worker
func (w *PaymentExpiryWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.log.Info("Stopping payment expiry worker...")

	w.cancelFn()
	close(w.stopCh)

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.log.Info("Payment expiry worker stopped gracefully")
	case <-time.After(10 * time.Second):
		w.log.Warn("Payment expiry worker shutdown timeout")
	}

	w.running = false
}

// IsRunning returns true if the worker is currently running
func (w *PaymentExpiryWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// run is the main worker loop
func (w *PaymentExpiryWorker) run() {
	defer w.wg.Done()

	w.checkExpiredPayments()

	for {
		select {
		case <-w.shutdownCtx.Done():
			w.log.Info("Worker shutdown requested")
			return

		case <-time.After(w.pollInterval):
			w.checkExpiredPayments()

		case <-w.stopCh:
			return
		}
	}
}

// checkExpiredPayments finds and marks expired pending payments.
//
// PATTERN:
// Phase 1: Short transaction to fetch IDs with SKIP LOCKED
// Phase 2: Process each payment in its own transaction
func (w *PaymentExpiryWorker) checkExpiredPayments() {
	ctx := context.Background()

	// Phase 1: Short tx just to fetch IDs with SKIP LOCKED
	// Locks are released immediately after this transaction commits
	paymentIDs, err := w.findExpiredPendingPaymentIDs(ctx, w.batchSize)
	if err != nil {
		w.log.Error("Failed to find expired payments", zap.Error(err))
		return
	}

	if len(paymentIDs) == 0 {
		return
	}

	w.log.Info("Found expired payments", zap.Int("count", len(paymentIDs)))

	// Phase 2: Process each payment in its own transaction
	// Failure is isolated per entity - no cascade rollback
	for _, paymentID := range paymentIDs {
		if err := w.expirePayment(ctx, paymentID); err != nil {
			w.log.Error("Failed to expire payment",
				zap.String("payment_id", paymentID.String()),
				zap.Error(err),
			)
		}
	}
}

// findExpiredPendingPaymentIDs returns IDs of pending payments that have expired.
// Phase 1: Short transaction to fetch IDs with SKIP LOCKED.
func (w *PaymentExpiryWorker) findExpiredPendingPaymentIDs(
	ctx context.Context,
	limit int,
) ([]uuid.UUID, error) {
	var paymentIDs []uuid.UUID

	// Short transaction to fetch and lock IDs
	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		query := `
			SELECT id
			FROM payments
			WHERE status = $1
			  AND expired_at < NOW()
			FOR UPDATE SKIP LOCKED
			LIMIT $2
		`

		rows, err := tx.Query(ctx, query, paymentRepo.PaymentStatusPending, limit)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				return err
			}
			paymentIDs = append(paymentIDs, id)
		}

		return rows.Err()
	})

	return paymentIDs, err
}

// expirePayment marks a single payment as expired within its own transaction.
// Phase 2: Each payment is processed in a separate transaction.
func (w *PaymentExpiryWorker) expirePayment(
	ctx context.Context,
	paymentID uuid.UUID,
) error {
	return w.db.WithTx(ctx, func(tx db.Tx) error {
		// Lock payment row with FOR UPDATE
		payment, err := w.paymentRepo.GetByIDForUpdate(ctx, tx, paymentID)
		if err != nil {
			return fmt.Errorf("failed to lock payment row: %w", err)
		}

		// Verify still expired (double-check after lock)
		if time.Now().Before(payment.ExpiredAt) {
			w.log.Debug("Payment no longer expired, skipping",
				zap.String("payment_id", payment.ID.String()),
			)
			return nil
		}

		// Update payment status to expired with database-level guard
		// Only updates if current status is 'pending' - provides idempotency
		err = w.paymentRepo.UpdateStatus(
			ctx, tx, paymentID,
			paymentRepo.PaymentStatusPending,
			paymentRepo.PaymentStatusExpire,
		)
		if errors.Is(err, paymentRepo.ErrInvalidStatusTransition) {
			// Payment was already processed by another worker/webhook (idempotent skip)
			w.log.Debug("Payment no longer pending, skipping",
				zap.String("payment_id", paymentID.String()),
				zap.String("status", payment.Status),
			)
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to mark payment as expired: %w", err)
		}

		// Payment successfully expired
		w.log.Info("Payment marked expired",
			zap.String("payment_id", payment.ID.String()),
		)

		// Handle reference-specific cleanup based on reference_type
		if payment.ReferenceType == paymentRepo.ReferenceTypeOrder && payment.ReferenceID != nil {
			if err := w.cancelOrderForExpiredPayment(ctx, tx, *payment.ReferenceID); err != nil {
				return fmt.Errorf("failed to cancel order after payment expiry: %w", err)
			}
		}

		return nil
	})
}

// cancelOrderForExpiredPayment expires an order when its payment expires.
// Uses Expire() instead of Cancel() to properly track payment expiry.
//
// PASS_20B: previously this treated EVERY error from Expire() identically —
// logged at Debug and swallowed as "not critical." That masked genuine
// failures (e.g. the D2 auction-restore bug: restoreForSaleStock erroring on
// an auction order aborted the whole Expire() transaction, and this method
// silently reported success anyway, leaving the order/payment stuck in
// pending_payment forever with the worker retrying every minute with zero
// operator visibility). Order.MarkExpired()'s own state-machine guard
// (order not pending anymore — already paid/cancelled by something else) is
// the ONLY expected, harmless case; everything else is a real failure that
// must be visible and must roll back so the worker retries instead of
// leaving a split-brain state (payment expired, order not).
func (w *PaymentExpiryWorker) cancelOrderForExpiredPayment(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) error {
	err := w.orderService.Expire(ctx, tx, orderID)
	if err == nil {
		w.log.Info("Order expired due to payment timeout",
			zap.String("order_id", orderID.String()),
		)
		return nil
	}

	if isOrderAlreadyProcessedError(err) {
		// Order is no longer pending (already paid/cancelled/expired by
		// something else) — expected, harmless, idempotent no-op.
		w.log.Debug("Order expiry after payment timeout skipped: order no longer pending",
			zap.String("order_id", orderID.String()),
			zap.Error(err),
		)
		return nil
	}

	// Real, non-recoverable failure (e.g. stock/auction restore error) —
	// surface it loudly and propagate so the enclosing transaction rolls
	// back and the worker retries next poll, instead of silently leaving
	// the order/payment stuck.
	w.log.Error("Order expiry after payment timeout failed non-recoverably",
		zap.String("order_id", orderID.String()),
		zap.Error(err),
	)
	return fmt.Errorf("order expire failed for order %s: %w", orderID, err)
}

// isOrderAlreadyProcessedError reports whether err is an
// entity.InvalidTransitionError — Order.MarkExpired()'s state-machine guard,
// meaning the order is no longer pending (already paid/cancelled/expired by
// something else). This is the ONLY expected, harmless outcome from
// Expire(); everything else is a real failure. Extracted as a pure function
// so this classification is unit-testable without a live OrderService/DB.
func isOrderAlreadyProcessedError(err error) bool {
	var invalidTransitionErr *orderEntity.InvalidTransitionError
	return errors.As(err, &invalidTransitionErr)
}
