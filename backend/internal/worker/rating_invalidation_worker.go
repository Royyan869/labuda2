package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	ratingApp "github.com/labuda/backend/internal/commerce/order/rating/application"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

const (
	// DefaultRatingInvalidationInterval is how often the worker scans for missed invalidations.
	DefaultRatingInvalidationInterval = 5 * time.Minute

	// DefaultRatingInvalidationBatchSize is the max orders processed per cycle.
	DefaultRatingInvalidationBatchSize = 100
)

// RatingInvalidationWorker is the eventual-consistency safety net for rating invalidation.
//
// PRIMARY AUTHORITY PATH:
//
//	OrderCompletionService calls ratingMutator.InvalidateForOrder at refund time for all
//	three refund paths (full refund, partial refund, dispute refund). That call is best-effort:
//	if it fails, the refund still completes and the failure is logged as CRITICAL.
//
// THIS WORKER:
//
//	Scans every 5 minutes for orders with status IN ('refunded', 'partially_refunded')
//	that still have a valid rating (invalidated_at IS NULL). It catches any orders
//	where the primary-path invalidation failed or was skipped.
//
// INVARIANTS:
//   - Does NOT mutate order state or ledger entries.
//   - Idempotent: the SQL uses WHERE invalidated_at IS NULL; re-running is always safe.
//   - Status guard: re-verifies order status inside the mutation transaction to protect
//     against concurrent state changes between scan and mutation.
//   - Per-order errors are logged and skipped; one failure does not block others.
type RatingInvalidationWorker struct {
	db            Transactor
	logger        *zap.Logger
	ratingMutator ratingApp.RatingMutator
	checkInterval time.Duration
	batchSize     int

	mu          sync.RWMutex
	running     bool
	shutdownCtx context.Context
	cancelFn    context.CancelFunc
	wg          sync.WaitGroup
}

// NewRatingInvalidationWorker creates a new rating invalidation safety-net worker.
//
// Dependencies:
//   - db: transaction executor (Transactor interface — testable, not *db.DB)
//   - logger: nil is safe (falls back to zap.NewNop)
//   - ratingMutator: write interface from the rating domain factory
func NewRatingInvalidationWorker(
	db Transactor,
	logger *zap.Logger,
	ratingMutator ratingApp.RatingMutator,
) *RatingInvalidationWorker {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &RatingInvalidationWorker{
		db:            db,
		logger:        logger,
		ratingMutator: ratingMutator,
		checkInterval: DefaultRatingInvalidationInterval,
		batchSize:     DefaultRatingInvalidationBatchSize,
	}
}

// Start begins the rating invalidation safety-net loop.
// Idempotent: calling Start on an already-running worker is a no-op.
func (w *RatingInvalidationWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.logger.Warn("RatingInvalidationWorker already running")
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.wg.Add(1)
	go w.run()

	w.logger.Info("RatingInvalidationWorker started",
		zap.Duration("check_interval", w.checkInterval),
		zap.Int("batch_size", w.batchSize),
	)
}

// Stop signals the worker to stop and waits for the current cycle to finish.
// Safe to call before Start or after Stop.
func (w *RatingInvalidationWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.cancelFn()
	w.wg.Wait()
	w.running = false

	w.logger.Info("RatingInvalidationWorker stopped")
}

// IsRunning returns true if the worker is currently active.
func (w *RatingInvalidationWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

func (w *RatingInvalidationWorker) run() {
	defer w.wg.Done()

	ticker := time.NewTicker(w.checkInterval)
	defer ticker.Stop()

	// Immediate scan on startup to drain any backlog accumulated before this
	// worker instance was started (e.g., after a redeploy).
	w.cleanupOnce(w.shutdownCtx)

	for {
		select {
		case <-w.shutdownCtx.Done():
			return
		case <-ticker.C:
			w.cleanupOnce(w.shutdownCtx)
		}
	}
}

// cleanupOnce performs a single scan-and-invalidate cycle.
func (w *RatingInvalidationWorker) cleanupOnce(ctx context.Context) {
	orderIDs, err := w.findOrdersNeedingInvalidation(ctx)
	if err != nil {
		w.logger.Error("rating invalidation: scan failed", zap.Error(err))
		return
	}
	if len(orderIDs) == 0 {
		return
	}

	w.logger.Info("rating invalidation: processing safety-net batch",
		zap.Int("count", len(orderIDs)),
	)

	for _, orderID := range orderIDs {
		if err := w.invalidateRatingForOrder(ctx, orderID); err != nil {
			w.logger.Error("rating invalidation: failed for order",
				zap.String("order_id", orderID.String()),
				zap.Error(err),
			)
			// Per-order failure is logged and skipped; other orders are not affected.
		}
	}
}

// findOrdersNeedingInvalidation returns order IDs for refunded orders with valid ratings.
//
// SQL CONTRACT:
//
//	SELECT DISTINCT o.id
//	FROM orders o
//	INNER JOIN order_ratings r ON r.order_id = o.id
//	WHERE o.status IN ('refunded', 'partially_refunded')
//	  AND r.invalidated_at IS NULL
//	ORDER BY o.updated_at DESC
//	LIMIT $1
//
// The partial index idx_order_ratings_valid covers the invalidated_at IS NULL filter.
func (w *RatingInvalidationWorker) findOrdersNeedingInvalidation(ctx context.Context) ([]uuid.UUID, error) {
	var orderIDs []uuid.UUID

	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT DISTINCT o.id
			FROM orders o
			INNER JOIN order_ratings r ON r.order_id = o.id
			WHERE o.status IN ('refunded', 'partially_refunded')
			  AND r.invalidated_at IS NULL
			ORDER BY o.updated_at DESC
			LIMIT $1
		`, w.batchSize)
		if err != nil {
			return fmt.Errorf("scan query failed: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				return fmt.Errorf("scan row failed: %w", err)
			}
			orderIDs = append(orderIDs, id)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("findOrdersNeedingInvalidation: %w", err)
	}
	return orderIDs, nil
}

// invalidateRatingForOrder invalidates the rating for a specific order.
//
// Status guard: re-verifies the order is still refunded/partially_refunded inside the
// mutation transaction. This protects against a race between the scan and the mutation
// (e.g., an order that was erroneously scanned or concurrently changed).
//
// Idempotent: the underlying repo SQL is:
//
//	UPDATE order_ratings SET invalidated_at = NOW()
//	WHERE order_id = $1 AND invalidated_at IS NULL
//
// Calling this for an already-invalidated order is always a safe no-op.
func (w *RatingInvalidationWorker) invalidateRatingForOrder(ctx context.Context, orderID uuid.UUID) error {
	return w.db.WithTx(ctx, func(tx db.Tx) error {
		var status string
		err := tx.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&status)
		if err != nil {
			return fmt.Errorf("verify order status: %w", err)
		}

		if status != "refunded" && status != "partially_refunded" {
			// Race condition guard: order status changed between scan and mutation.
			w.logger.Warn("rating invalidation: order status changed since scan, skipping",
				zap.String("order_id", orderID.String()),
				zap.String("current_status", status),
			)
			return nil
		}

		if err := w.ratingMutator.InvalidateForOrder(ctx, tx, orderID); err != nil {
			return fmt.Errorf("InvalidateForOrder: %w", err)
		}

		w.logger.Info("rating invalidated via safety-net path",
			zap.String("order_id", orderID.String()),
			zap.String("order_status", status),
		)
		return nil
	})
}

// TriggerManualCleanup runs one cleanup cycle synchronously.
// Intended for admin tools and one-off recovery operations.
func (w *RatingInvalidationWorker) TriggerManualCleanup(ctx context.Context) error {
	orderIDs, err := w.findOrdersNeedingInvalidation(ctx)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	for _, orderID := range orderIDs {
		if err := w.invalidateRatingForOrder(ctx, orderID); err != nil {
			w.logger.Error("manual cleanup failed for order",
				zap.String("order_id", orderID.String()),
				zap.Error(err),
			)
		}
	}

	w.logger.Info("manual rating invalidation cleanup completed",
		zap.Int("processed", len(orderIDs)),
	)
	return nil
}


