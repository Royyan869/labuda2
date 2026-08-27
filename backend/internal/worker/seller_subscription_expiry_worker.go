package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	sellerRepo "github.com/labuda/backend/internal/commerce/seller/repository"
	"github.com/labuda/backend/internal/commerce/subscription/entity"
	"github.com/labuda/backend/internal/commerce/subscription/repository"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// DefaultSubscriptionExpiryInterval is the default tick interval for the
// subscription expiry worker. Subscription duration is 365d, and the renewal
// reminder runs 7 days before expiry, so hourly resolution is sufficient.
const DefaultSubscriptionExpiryInterval = 1 * time.Hour

// SellerSubscriptionExpiryWorker handles subscription status transitions.
//
// This worker enforces the subscription lifecycle:
// - active -> expired (when expires_at <= NOW)
//
// Renewal reminders are emitted at the canonical reminder threshold only.
//
// Complies with Worker Architecture: 1 entity = 1 transaction.
type SellerSubscriptionExpiryWorker struct {
	db         Transactor
	repo       repository.SellerSubscriptionRepository
	sellerRepo sellerRepo.SellerRepository
	outbox     OutboxRepository
	log        *zap.Logger
	batchSize  int

	// Lifecycle fields — mirror SubscriptionReconciliationWorker.
	mu          sync.RWMutex
	running     bool
	interval    time.Duration
	stopCh      chan struct{}
	wg          sync.WaitGroup
	shutdownCtx context.Context
	cancelFn    context.CancelFunc
}

// OutboxRepository defines the interface for emitting outbox events.
// This allows the worker to work with both real outbox and mocks.
type OutboxRepository interface {
	InsertTx(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error
}

// NewSellerSubscriptionExpiryWorker creates a new subscription expiry worker.
func NewSellerSubscriptionExpiryWorker(
	db Transactor,
	repo repository.SellerSubscriptionRepository,
	sellerRepo sellerRepo.SellerRepository,
	outbox OutboxRepository,
	log *zap.Logger,
) *SellerSubscriptionExpiryWorker {
	if log == nil {
		log = zap.NewNop()
	}

	return &SellerSubscriptionExpiryWorker{
		db:         db,
		repo:       repo,
		sellerRepo: sellerRepo,
		outbox:     outbox,
		log:        log,
		batchSize:  DefaultBatchSize,
		interval:   DefaultSubscriptionExpiryInterval,
		stopCh:     make(chan struct{}),
	}
}

// SetInterval overrides the default tick interval. Must be called before Start.
func (w *SellerSubscriptionExpiryWorker) SetInterval(interval time.Duration) {
	if interval > 0 {
		w.interval = interval
	}
}

// Start begins periodic reminder emission and expiry transitions in the background.
// Implements serverboot.Worker.
func (w *SellerSubscriptionExpiryWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("seller subscription expiry worker already running")
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.stopCh = make(chan struct{})

	w.wg.Add(1)
	go w.run()

	w.log.Info("subscription_expiry_worker_started",
		zap.Duration("interval", w.interval),
		zap.Int("batch_size", w.batchSize),
	)
}

// Stop gracefully shuts down the worker. Cancels the shutdown context, waits
// for the run loop to drain, and bounds the wait at 10s to avoid hanging the
// process on shutdown.
func (w *SellerSubscriptionExpiryWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.log.Info("stopping seller subscription expiry worker")

	w.cancelFn()
	close(w.stopCh)

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.log.Info("seller subscription expiry worker stopped gracefully")
	case <-time.After(10 * time.Second):
		w.log.Warn("seller subscription expiry worker shutdown timeout")
	}

	w.running = false
}

// IsRunning returns true if the worker loop is active.
func (w *SellerSubscriptionExpiryWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// run is the main loop. Executes one tick immediately on start, then on each
// interval. Returns when the shutdown context is cancelled or stopCh is closed.
func (w *SellerSubscriptionExpiryWorker) run() {
	defer w.wg.Done()

	w.runOnce()

	for {
		select {
		case <-w.shutdownCtx.Done():
			return
		case <-w.stopCh:
			return
		case <-time.After(w.interval):
			w.runOnce()
		}
	}
}

// expiringThreshold defines a reminder threshold for upcoming subscription expiry.
type expiringThreshold struct {
	Days     int
	Duration time.Duration
}

// expiringThresholds are the pre-expiry reminder points. Each active
// subscription gets at most one outbox event per threshold, deduplicated
// by the deterministic idempotency key.
var expiringThresholds = []expiringThreshold{
	{7, 7 * 24 * time.Hour},
}

// runOnce executes one pair of transition sweeps. Errors are logged and
// swallowed so a single tick failure does not kill the loop.
func (w *SellerSubscriptionExpiryWorker) runOnce() {
	ctx := w.shutdownCtx
	if ctx == nil {
		ctx = context.Background()
	}

	// Expiring reminders run BEFORE transitions so a subscription that is about
	// to expire gets its reminder before the expiry transition runs.
	if err := w.ProcessExpiringReminders(ctx); err != nil {
		w.log.Error("expiring_reminders_failed", zap.Error(err))
	} else {
		w.log.Debug("expiring_reminders_processed")
	}

	if err := w.ProcessActiveToExpired(ctx); err != nil {
		w.log.Error("active_to_expired_processed_failed", zap.Error(err))
	} else {
		w.log.Debug("active_to_expired_processed")
	}
}

// ProcessActiveToExpired transitions expired subscriptions from active to expired.
//
// Process for each subscription:
// 1. Start transaction (WithTx)
// 2. Lock row with GetByIDForUpdate (FOR UPDATE)
// 3. Validate status == active
// 4. Update status to expired (with guard: WHERE id=X AND status='active')
// 5. Insert outbox event
// 6. Commit
//
// Each subscription is processed in its own transaction (1 entity = 1 transaction).
func (w *SellerSubscriptionExpiryWorker) ProcessActiveToExpired(ctx context.Context) error {
	now := time.Now()

	// Fetch batch IDs WITHOUT lock - we'll lock each row individually in its own transaction
	// Use a nil transaction since FetchActiveExpiredBatchIDs doesn't need one for just IDs
	subscriptionIDs, err := w.fetchActiveExpiredBatchIDs(ctx, now)
	if err != nil {
		return fmt.Errorf("fetch active expired batch failed: %w", err)
	}

	if len(subscriptionIDs) == 0 {
		return nil // No work to do
	}

	w.log.Info("Processing subscriptions active -> expired", zap.Int("count", len(subscriptionIDs)))

	// Process each subscription in its own transaction
	for _, subID := range subscriptionIDs {
		if err := w.db.WithTx(ctx, func(tx db.Tx) error {
			return w.processActiveToExpiredOne(ctx, tx, subID)
		}); err != nil {
			w.log.Error("Failed to process subscription active -> expired",
				zap.String("subscription_id", subID.String()),
				zap.Error(err),
			)
			// Continue processing other subscriptions
		}
	}

	return nil
}

// processActiveToExpiredOne processes a single subscription transition within a transaction.
func (w *SellerSubscriptionExpiryWorker) processActiveToExpiredOne(
	ctx context.Context,
	tx db.Tx,
	subID uuid.UUID,
) error {
	// Lock row FOR UPDATE
	sub, err := w.repo.GetByIDForUpdate(ctx, tx, subID)
	if err != nil {
		return fmt.Errorf("lock subscription failed: %w", err)
	}
	if sub == nil {
		return nil // Subscription deleted, skip
	}

	// Validate current status is active
	if sub.Status != entity.StatusActive {
		return fmt.Errorf("subscription %s: expected status active, got %s", subID, sub.Status)
	}

	// Update status with guard: WHERE id = X AND status = 'active'
	if err := w.repo.UpdateStatusTx(ctx, tx, subID, entity.StatusActive, entity.StatusExpired); err != nil {
		return fmt.Errorf("update status failed: %w", err)
	}

	// Insert outbox event with deterministic idempotency key
	idempotencyKey := fmt.Sprintf("seller.subscription.expired.%s", subID)
	payload := map[string]any{
		"subscription_id": subID,
		"user_id":         sub.UserID,
		"expires_at":      sub.ExpiresAt,
	}

	if err := w.outbox.InsertTx(ctx, tx, "seller.subscription.expired", payload, idempotencyKey); err != nil {
		return fmt.Errorf("insert outbox event failed: %w", err)
	}

	w.log.Debug("Subscription transitioned to expired",
		zap.String("subscription_id", subID.String()),
		zap.String("user_id", sub.UserID.String()),
	)

	return nil
}

// fetchActiveExpiredBatchIDs fetches subscription IDs that need transition to expired.
// Uses a temporary transactionless context for the ID-only query.
func (w *SellerSubscriptionExpiryWorker) fetchActiveExpiredBatchIDs(
	ctx context.Context,
	now time.Time,
) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	var err error

	// Use WithTx with a read-only approach to fetch IDs
	// The fetch methods don't modify data, so we can use a minimal transaction
	err = w.db.WithTx(ctx, func(tx db.Tx) error {
		ids, err = w.repo.FetchActiveExpiredBatchIDs(ctx, tx, now, w.batchSize)
		return err
	})

	return ids, err
}

// ProcessExpiringReminders emits seller.subscription.expiring outbox events
// for active subscriptions approaching their expires_at date.
//
// One threshold: 7d. Each subscription receives at most one
// event per threshold — the deterministic idempotency key
// "seller.subscription.expiring.{subID}.{days}d" and the outbox
// ON CONFLICT DO NOTHING guarantee deduplication.
//
// No status transition. No row lock. Notification only.
func (w *SellerSubscriptionExpiryWorker) ProcessExpiringReminders(ctx context.Context) error {
	now := time.Now()

	for _, t := range expiringThresholds {
		if err := w.processExpiringForThreshold(ctx, now, t); err != nil {
			w.log.Error("expiring_threshold_failed",
				zap.Int("days", t.Days),
				zap.Error(err),
			)
			// Continue to next threshold
		}
	}
	return nil
}

// expiringSubscription holds the fields needed for an expiring reminder.
type expiringSubscription struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	ExpiresAt time.Time
}

// processExpiringForThreshold emits outbox events for a single threshold.
func (w *SellerSubscriptionExpiryWorker) processExpiringForThreshold(
	ctx context.Context,
	now time.Time,
	t expiringThreshold,
) error {
	deadline := now.Add(t.Duration)

	// Fetch active subscriptions expiring within this threshold window.
	var subs []expiringSubscription
	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		rows, qErr := tx.Query(ctx, `
			SELECT id, user_id, expires_at
			FROM seller_subscriptions
			WHERE status = 'active'
			  AND expires_at > $1
			  AND expires_at <= $2
			LIMIT $3
		`, now, deadline, w.batchSize)
		if qErr != nil {
			return fmt.Errorf("query expiring subscriptions failed: %w", qErr)
		}
		defer rows.Close()

		for rows.Next() {
			var s expiringSubscription
			if err := rows.Scan(&s.ID, &s.UserID, &s.ExpiresAt); err != nil {
				return fmt.Errorf("scan expiring subscription failed: %w", err)
			}
			subs = append(subs, s)
		}
		return rows.Err()
	})
	if err != nil {
		return err
	}

	if len(subs) == 0 {
		return nil
	}

	w.log.Debug("Processing expiring reminders",
		zap.Int("days", t.Days),
		zap.Int("count", len(subs)),
	)

	// Emit one outbox event per subscription. Each in its own tx so a single
	// failure does not block the rest. ON CONFLICT DO NOTHING deduplicates.
	for _, s := range subs {
		if err := w.db.WithTx(ctx, func(tx db.Tx) error {
			idempotencyKey := fmt.Sprintf("%s.%dd", s.ID, t.Days)
			payload := map[string]any{
				"subscription_id":   s.ID,
				"user_id":           s.UserID,
				"expires_at":        s.ExpiresAt,
				"days_until_expiry": t.Days,
			}
			return w.outbox.InsertTx(ctx, tx, "seller.subscription.expiring", payload, idempotencyKey)
		}); err != nil {
			w.log.Error("expiring_reminder_emit_failed",
				zap.String("subscription_id", s.ID.String()),
				zap.Int("days", t.Days),
				zap.Error(err),
			)
		}
	}

	return nil
}

// SetBatchSize sets the batch size for processing.
func (w *SellerSubscriptionExpiryWorker) SetBatchSize(size int) {
	if size > 0 {
		w.batchSize = size
	}
}
