// OrderOverdueReminderWorker — default ON via DISABLE_ORDER_OVERDUE_REMINDER_WORKER env gate.
// Uses the canonical schema's order_overdue_reminders table.
package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/order/entity"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// =============================================================================
// ORDER OVERDUE REMINDER WORKER - GRADED ENFORCEMENT NOTIFICATIONS
// =============================================================================
//
// BUSINESS TRUTH:
// - Orders that are paid, not shipped, and past ready_to_ship_by are overdue
// - This worker sends tiered reminders to sellers and buyers
// - Uses a reminder tracking table to prevent spam
//
// GRADED NOTIFICATION POLICY:
// - Tier 1 (Day 0 overdue): Notify seller only
// - Tier 2 (Day 3 overdue): Notify seller again + notify buyer
// - Tier 3 (Day 7 overdue): Strong notification to buyer + notify seller
//
// DEDUPLICATION:
// - Each order+tier combination is tracked in order_overdue_reminders table
// - Reminders are sent at most once per tier
// - Uses deterministic idempotency keys for outbox events
//
// PROHIBITED:
// - Do NOT change order status
// - Do NOT auto-cancel / auto-refund
// - Do NOT send spam notifications
// =============================================================================

const (
	// DefaultOverdueReminderPollInterval is how often to check for overdue orders
	DefaultOverdueReminderPollInterval = 1 * time.Hour
	// DefaultOverdueReminderBatchSize is the default number of orders to process per batch
	DefaultOverdueReminderBatchSize = 100
)

// ReminderTier represents the tier of overdue reminder
type ReminderTier string

const (
	ReminderTier1 ReminderTier = "tier1" // Day 0: Just became overdue
	ReminderTier2 ReminderTier = "tier2" // Day 3: Severely overdue
	ReminderTier3 ReminderTier = "tier3" // Day 7: Critically overdue
)

// OverdueReminderRepository defines the interface for tracking overdue reminders.
type OverdueReminderRepository interface {
	// FindOrdersNeedingReminder finds orders that need a reminder for the given tier.
	// Returns order IDs that are overdue but haven't had this tier's reminder sent yet.
	FindOrdersNeedingReminder(ctx context.Context, tx db.Tx, tier ReminderTier, limit int) ([]uuid.UUID, error)

	// MarkReminderSent records that a reminder was sent for an order at a specific tier.
	MarkReminderSent(ctx context.Context, tx db.Tx, orderID uuid.UUID, tier ReminderTier) error

	// GetByIDForUpdate locks an order row for update.
	GetByIDForUpdate(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.Order, error)
}

// OrderOverdueReminderWorker sends tiered notifications for overdue orders.
//
// This worker enforces the graded notification policy:
// - Tier 1 (Day 0 overdue): Notify seller
// - Tier 2 (Day 3 overdue): Notify seller + buyer
// - Tier 3 (Day 7 overdue): Notify seller + buyer (stronger)
//
// Each order+tier combination is tracked to prevent duplicate notifications.
type OrderOverdueReminderWorker struct {
	db           Transactor
	reminderRepo OverdueReminderRepository
	outbox       OutboxInserter
	log          *zap.Logger
	batchSize    int

	// Worker lifecycle
	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

	// Context for shutdown
	shutdownCtx context.Context
	cancelFn    context.CancelFunc

	// Worker identifier for logging
	workerID string

	// Poll interval for periodic processing
	pollInterval time.Duration
}

// NewOrderOverdueReminderWorker creates a new overdue reminder worker.
func NewOrderOverdueReminderWorker(
	db Transactor,
	reminderRepo OverdueReminderRepository,
	outbox OutboxInserter,
	log *zap.Logger,
) *OrderOverdueReminderWorker {
	if log == nil {
		log = zap.NewNop()
	}

	workerID := fmt.Sprintf("order-overdue-reminder-worker-%s", uuid.New().String()[:8])

	return &OrderOverdueReminderWorker{
		db:           db,
		reminderRepo: reminderRepo,
		outbox:       outbox,
		log:          log,
		batchSize:    DefaultOverdueReminderBatchSize,
		pollInterval: DefaultOverdueReminderPollInterval,
		stopCh:       make(chan struct{}),
		workerID:     workerID,
	}
}

// Start begins processing overdue reminders in the background.
func (w *OrderOverdueReminderWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("OrderOverdueReminderWorker already running",
			zap.String("worker_id", w.workerID),
		)
		return
	}

	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.running = true

	w.wg.Add(1)
	go w.run()

	w.log.Info("OrderOverdueReminderWorker started",
		zap.String("worker_id", w.workerID),
		zap.Duration("poll_interval", w.pollInterval),
		zap.Int("batch_size", w.batchSize),
	)
}

// Stop gracefully shuts down the worker.
func (w *OrderOverdueReminderWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.log.Info("OrderOverdueReminderWorker stopping",
		zap.String("worker_id", w.workerID),
	)

	// Signal shutdown
	w.cancelFn()
	close(w.stopCh)

	// Wait for run loop to exit
	w.wg.Wait()

	w.running = false

	w.log.Info("OrderOverdueReminderWorker stopped",
		zap.String("worker_id", w.workerID),
	)
}

// IsRunning returns true if the worker is currently running.
func (w *OrderOverdueReminderWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// run is the main worker loop.
func (w *OrderOverdueReminderWorker) run() {
	defer w.wg.Done()

	// Create ticker for periodic sweeps
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	// Do immediate sweep on startup
	w.processSweep()

	for {
		select {
		case <-w.shutdownCtx.Done():
			w.log.Debug("OrderOverdueReminderWorker shutdown requested",
				zap.String("worker_id", w.workerID),
			)
			return

		case <-ticker.C:
			w.processSweep()

		case <-w.stopCh:
			w.log.Debug("OrderOverdueReminderWorker stop signal received",
				zap.String("worker_id", w.workerID),
			)
			return
		}
	}
}

// processSweep executes one sweep of all reminder tiers.
func (w *OrderOverdueReminderWorker) processSweep() {
	w.log.Debug("OrderOverdueReminderWorker starting sweep",
		zap.String("worker_id", w.workerID),
	)

	startTime := time.Now()

	// Process all tiers
	ctx := w.shutdownCtx
	if err := w.ProcessTier1Reminders(ctx); err != nil {
		w.log.Error("Tier 1 reminder processing failed", zap.Error(err))
	}
	if err := w.ProcessTier2Reminders(ctx); err != nil {
		w.log.Error("Tier 2 reminder processing failed", zap.Error(err))
	}
	if err := w.ProcessTier3Reminders(ctx); err != nil {
		w.log.Error("Tier 3 reminder processing failed", zap.Error(err))
	}

	duration := time.Since(startTime)

	w.log.Debug("OrderOverdueReminderWorker sweep completed",
		zap.String("worker_id", w.workerID),
		zap.Duration("duration", duration),
	)
}

// ProcessTier1Reminders processes Tier 1 (Day 0 overdue) reminders.
// Notifies seller that order has just passed ready_to_ship_by.
func (w *OrderOverdueReminderWorker) ProcessTier1Reminders(ctx context.Context) error {
	return w.processTier(ctx, ReminderTier1, w.processTier1One)
}

// ProcessTier2Reminders processes Tier 2 (Day 3 overdue) reminders.
// Notifies seller again + notifies buyer.
func (w *OrderOverdueReminderWorker) ProcessTier2Reminders(ctx context.Context) error {
	return w.processTier(ctx, ReminderTier2, w.processTier2One)
}

// ProcessTier3Reminders processes Tier 3 (Day 7 overdue) reminders.
// Strong notification to buyer + seller.
func (w *OrderOverdueReminderWorker) ProcessTier3Reminders(ctx context.Context) error {
	return w.processTier(ctx, ReminderTier3, w.processTier3One)
}

// processTier is the generic tier processor.
func (w *OrderOverdueReminderWorker) processTier(
	ctx context.Context,
	tier ReminderTier,
	processFunc func(context.Context, db.Tx, uuid.UUID) error,
) error {
	// Find orders needing this tier's reminder
	var orderIDs []uuid.UUID
	var err error

	// Fetch IDs in a short transaction
	err = w.db.WithTx(ctx, func(tx db.Tx) error {
		orderIDs, err = w.reminderRepo.FindOrdersNeedingReminder(ctx, tx, tier, w.batchSize)
		return err
	})
	if err != nil {
		return fmt.Errorf("find orders needing tier %s reminder failed: %w", tier, err)
	}

	if len(orderIDs) == 0 {
		return nil // No work to do
	}

	w.log.Info("Processing overdue reminders",
		zap.String("tier", string(tier)),
		zap.Int("count", len(orderIDs)),
	)

	// Process each order in its own transaction (1 entity = 1 transaction)
	for _, orderID := range orderIDs {
		if err := w.db.WithTx(ctx, func(tx db.Tx) error {
			return processFunc(ctx, tx, orderID)
		}); err != nil {
			w.log.Error("Failed to process overdue reminder",
				zap.String("tier", string(tier)),
				zap.String("order_id", orderID.String()),
				zap.Error(err),
			)
			// Continue processing other orders
		}
	}

	return nil
}

// processTier1One processes a single Tier 1 reminder (seller notification only).
func (w *OrderOverdueReminderWorker) processTier1One(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) error {
	// Lock order row
	order, err := w.reminderRepo.GetByIDForUpdate(ctx, tx, orderID)
	if err != nil {
		return fmt.Errorf("lock order failed: %w", err)
	}
	if order == nil {
		return nil // Order deleted, skip
	}

	// Verify still eligible (paid + overdue)
	overdueInfo := order.CalculateOverdueInfo()
	if !overdueInfo.IsOverdue || order.Status != entity.StatusPaid {
		return nil // No longer eligible, skip
	}

	// Verify still in Tier 1 (0-2 days overdue)
	if overdueInfo.Tier != entity.OverdueTier1 {
		return nil // Moved to different tier, skip
	}

	// Mark reminder as sent
	if err := w.reminderRepo.MarkReminderSent(ctx, tx, orderID, ReminderTier1); err != nil {
		return fmt.Errorf("mark reminder sent failed: %w", err)
	}

	// Emit seller notification event
	idempotencyKey := fmt.Sprintf("order.overdue.tier1.seller.%s", orderID)
	payload := map[string]any{
		"order_id":         orderID,
		"seller_id":        order.SellerID,
		"buyer_id":         order.BuyerID,
		"tier":             string(ReminderTier1),
		"days_overdue":     overdueInfo.DaysOverdue,
		"ready_to_ship_by": order.ReadyToShipBy,
	}

	if err := w.outbox.InsertTx(ctx, tx, "order.overdue_reminder.seller", payload, idempotencyKey); err != nil {
		return fmt.Errorf("insert outbox event failed: %w", err)
	}

	w.log.Debug("Tier 1 overdue reminder sent to seller",
		zap.String("order_id", orderID.String()),
		zap.String("seller_id", order.SellerID.String()),
	)

	return nil
}

// processTier2One processes a single Tier 2 reminder (seller + buyer notification).
func (w *OrderOverdueReminderWorker) processTier2One(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) error {
	// Lock order row
	order, err := w.reminderRepo.GetByIDForUpdate(ctx, tx, orderID)
	if err != nil {
		return fmt.Errorf("lock order failed: %w", err)
	}
	if order == nil {
		return nil // Order deleted, skip
	}

	// Verify still eligible (paid + overdue)
	overdueInfo := order.CalculateOverdueInfo()
	if !overdueInfo.IsOverdue || order.Status != entity.StatusPaid {
		return nil // No longer eligible, skip
	}

	// Verify still in Tier 2 (3-6 days overdue)
	if overdueInfo.Tier != entity.OverdueTier2 {
		return nil // Moved to different tier, skip
	}

	// Mark reminder as sent
	if err := w.reminderRepo.MarkReminderSent(ctx, tx, orderID, ReminderTier2); err != nil {
		return fmt.Errorf("mark reminder sent failed: %w", err)
	}

	// Emit seller notification event
	sellerIdempotencyKey := fmt.Sprintf("order.overdue.tier2.seller.%s", orderID)
	sellerPayload := map[string]any{
		"order_id":         orderID,
		"seller_id":        order.SellerID,
		"buyer_id":         order.BuyerID,
		"tier":             string(ReminderTier2),
		"days_overdue":     overdueInfo.DaysOverdue,
		"ready_to_ship_by": order.ReadyToShipBy,
	}

	if err := w.outbox.InsertTx(ctx, tx, "order.overdue_reminder.seller", sellerPayload, sellerIdempotencyKey); err != nil {
		return fmt.Errorf("insert seller outbox event failed: %w", err)
	}

	// Emit buyer notification event
	buyerIdempotencyKey := fmt.Sprintf("order.overdue.tier2.buyer.%s", orderID)
	buyerPayload := map[string]any{
		"order_id":         orderID,
		"buyer_id":         order.BuyerID,
		"seller_id":        order.SellerID,
		"tier":             string(ReminderTier2),
		"days_overdue":     overdueInfo.DaysOverdue,
		"ready_to_ship_by": order.ReadyToShipBy,
	}

	if err := w.outbox.InsertTx(ctx, tx, "order.overdue_reminder.buyer", buyerPayload, buyerIdempotencyKey); err != nil {
		return fmt.Errorf("insert buyer outbox event failed: %w", err)
	}

	w.log.Debug("Tier 2 overdue reminder sent to seller and buyer",
		zap.String("order_id", orderID.String()),
		zap.String("seller_id", order.SellerID.String()),
		zap.String("buyer_id", order.BuyerID.String()),
	)

	return nil
}

// processTier3One processes a single Tier 3 reminder (strong seller + buyer notification).
func (w *OrderOverdueReminderWorker) processTier3One(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) error {
	// Lock order row
	order, err := w.reminderRepo.GetByIDForUpdate(ctx, tx, orderID)
	if err != nil {
		return fmt.Errorf("lock order failed: %w", err)
	}
	if order == nil {
		return nil // Order deleted, skip
	}

	// Verify still eligible (paid + overdue)
	overdueInfo := order.CalculateOverdueInfo()
	if !overdueInfo.IsOverdue || order.Status != entity.StatusPaid {
		return nil // No longer eligible, skip
	}

	// Verify still in Tier 3 (7+ days overdue)
	if overdueInfo.Tier != entity.OverdueTier3 {
		return nil // Not in Tier 3, skip
	}

	// Mark reminder as sent
	if err := w.reminderRepo.MarkReminderSent(ctx, tx, orderID, ReminderTier3); err != nil {
		return fmt.Errorf("mark reminder sent failed: %w", err)
	}

	// Emit seller notification event (stronger wording)
	sellerIdempotencyKey := fmt.Sprintf("order.overdue.tier3.seller.%s", orderID)
	sellerPayload := map[string]any{
		"order_id":         orderID,
		"seller_id":        order.SellerID,
		"buyer_id":         order.BuyerID,
		"tier":             string(ReminderTier3),
		"days_overdue":     overdueInfo.DaysOverdue,
		"ready_to_ship_by": order.ReadyToShipBy,
	}

	if err := w.outbox.InsertTx(ctx, tx, "order.overdue_reminder.seller", sellerPayload, sellerIdempotencyKey); err != nil {
		return fmt.Errorf("insert seller outbox event failed: %w", err)
	}

	// Emit buyer notification event (stronger wording + support prompt)
	buyerIdempotencyKey := fmt.Sprintf("order.overdue.tier3.buyer.%s", orderID)
	buyerPayload := map[string]any{
		"order_id":         orderID,
		"buyer_id":         order.BuyerID,
		"seller_id":        order.SellerID,
		"tier":             string(ReminderTier3),
		"days_overdue":     overdueInfo.DaysOverdue,
		"ready_to_ship_by": order.ReadyToShipBy,
		"support_cta":      true, // Include support contact CTA
	}

	if err := w.outbox.InsertTx(ctx, tx, "order.overdue_reminder.buyer", buyerPayload, buyerIdempotencyKey); err != nil {
		return fmt.Errorf("insert buyer outbox event failed: %w", err)
	}

	w.log.Debug("Tier 3 overdue reminder sent to seller and buyer",
		zap.String("order_id", orderID.String()),
		zap.String("seller_id", order.SellerID.String()),
		zap.String("buyer_id", order.BuyerID.String()),
	)

	return nil
}

// SetBatchSize sets the batch size for processing.
func (w *OrderOverdueReminderWorker) SetBatchSize(size int) {
	if size > 0 {
		w.batchSize = size
	}
}

// =============================================================================
// OVERDUE REMINDER REPOSITORY IMPLEMENTATION
// =============================================================================

// orderOverdueReminderRepository implements the OverdueReminderRepository interface.
// Uses the order_overdue_reminders table for deduplication and tier tracking.
type orderOverdueReminderRepository struct{}

// NewOrderOverdueReminderRepository creates a new overdue reminder repository.
func NewOrderOverdueReminderRepository() OverdueReminderRepository {
	return &orderOverdueReminderRepository{}
}

// FindOrdersNeedingReminder finds orders that need a reminder for the given tier.
//
// SQL Logic:
// - Find paid orders where ready_to_ship_by < NOW
// - Calculate days overdue
// - Filter by tier (0-2 days for tier1, 3-6 for tier2, 7+ for tier3)
// - Exclude orders that already have this tier's reminder sent
//
// The CTE keeps ordering, locking, and limiting on canonical order rows while
// avoiding DISTINCT, which would otherwise conflict with ORDER BY on PostgreSQL.
func (r *orderOverdueReminderRepository) FindOrdersNeedingReminder(
	ctx context.Context,
	tx db.Tx,
	tier ReminderTier,
	limit int,
) ([]uuid.UUID, error) {
	// Calculate day range for the tier
	var minDays, maxDays int
	switch tier {
	case ReminderTier1:
		minDays, maxDays = 0, 2
	case ReminderTier2:
		minDays, maxDays = 3, 6
	case ReminderTier3:
		minDays, maxDays = 7, 9999 // Effectively unbounded
	default:
		return nil, fmt.Errorf("invalid reminder tier: %s", tier)
	}

	// Query for orders needing reminder.
	// The CTE isolates canonical order rows, so we can deterministically order
	// by ready_to_ship_by, lock the underlying order rows, and skip locked rows
	// without needing DISTINCT for deduplication.
	query := `
		WITH candidate_orders AS (
			SELECT o.id, o.ready_to_ship_by
			FROM orders o
			WHERE o.status = 'paid'
			  AND o.ready_to_ship_by IS NOT NULL
			  AND o.ready_to_ship_by < NOW()
			  AND NOT EXISTS (
				SELECT 1
				FROM order_overdue_reminders r
				WHERE r.order_id = o.id
				  AND r.tier = $1
			  )
			  AND EXTRACT(DAY FROM (NOW() - o.ready_to_ship_by)) BETWEEN $2 AND $3
			ORDER BY o.ready_to_ship_by ASC, o.id ASC
			FOR UPDATE SKIP LOCKED
			LIMIT $4
		)
		SELECT id
		FROM candidate_orders
		ORDER BY ready_to_ship_by ASC, id ASC
	`

	rows, err := tx.Query(ctx, query, string(tier), minDays, maxDays, limit)
	if err != nil {
		return nil, fmt.Errorf("find orders needing reminder failed: %w", err)
	}
	defer rows.Close()

	var orderIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan order id failed: %w", err)
		}
		orderIDs = append(orderIDs, id)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate order ids failed: %w", rows.Err())
	}

	return orderIDs, nil
}

// MarkReminderSent records that a reminder was sent for an order at a specific tier.
// Uses ON CONFLICT to prevent duplicate records.
func (r *orderOverdueReminderRepository) MarkReminderSent(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	tier ReminderTier,
) error {
	query := `
		INSERT INTO order_overdue_reminders (order_id, tier, sent_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (order_id, tier) DO NOTHING
	`

	_, err := tx.Exec(ctx, query, orderID, string(tier))
	if err != nil {
		return fmt.Errorf("mark reminder sent failed: %w", err)
	}

	return nil
}

// GetByIDForUpdate locks an order row and returns the minimal fields needed
// for overdue reminder processing: Status, ReadyToShipBy, BuyerID, SellerID.
//
// This is a purpose-built minimal query — NOT a full order hydration.
// The reminder worker only needs these fields to verify eligibility
// (CalculateOverdueInfo) and build notification payloads.
func (r *orderOverdueReminderRepository) GetByIDForUpdate(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (*entity.Order, error) {
	var orderID, buyerID, sellerID uuid.UUID
	var status string
	var readyToShipBy *time.Time

	err := tx.QueryRow(ctx, `
		SELECT id, buyer_id, seller_id, status, ready_to_ship_by
		FROM orders
		WHERE id = $1
		FOR UPDATE
	`, id).Scan(&orderID, &buyerID, &sellerID, &status, &readyToShipBy)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil // Order not found
		}
		return nil, fmt.Errorf("get order for update failed: %w", err)
	}

	return &entity.Order{
		ID:            orderID,
		BuyerID:       buyerID,
		SellerID:      sellerID,
		Status:        entity.Status(status),
		ReadyToShipBy: readyToShipBy,
	}, nil
}
