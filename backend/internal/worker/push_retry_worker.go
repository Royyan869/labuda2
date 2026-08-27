package worker

// PushRetryWorker — reliability layer for failed FCM push notifications.
//
// Business truth: push delivery may fail (FCM outage, token churn, network).
// Important notifications (chat/order/payment/withdrawal) must have retry
// capability. Retry must not become infinite spam.
//
// Design:
//   - Polls push_retry_queue every 30 seconds.
//   - Exponential backoff: 1m → 5m → 15m → 1h → 4h → 8h.
//   - Max 10 attempts or 24-hour window, whichever comes first.
//   - On success: row deleted.
//   - On terminal failure: row deleted, delivery logged as "failed".
//   - Duplicate enqueue: idempotent via partial-unique index on
//     (notification_id, fcm_token) where expires_at > NOW().
//   - Policy: push eligibility is evaluated upstream (PushService); the
//     retry worker re-sends the already-approved token. No policy bypass.

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	dbpkg "github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

const (
	// maxPushRetryAttempts is the maximum number of retry attempts for a push notification.
	maxPushRetryAttempts = 10

	// pushRetryWindow is the maximum time window for retry attempts.
	pushRetryWindow = 24 * time.Hour

	// pushBatchSize is the number of push retries to process in one batch.
	pushBatchSize = 100

	// DefaultPushRetryPollInterval is the default poll interval for PushRetryWorker.
	DefaultPushRetryPollInterval = 30 * time.Second
)

// DBPushRetryQueue is the database-backed implementation of service.PushRetryQueue.
// It wraps EnqueuePushRetry so callers can inject it via the interface.
// Duplicate enqueue is safe: the partial-unique index on (notification_id, fcm_token)
// silently ignores re-inserts for active entries.
type DBPushRetryQueue struct {
	db *dbpkg.DB
}

// NewDBPushRetryQueue creates a DB-backed push retry queue.
func NewDBPushRetryQueue(db *dbpkg.DB) *DBPushRetryQueue {
	return &DBPushRetryQueue{db: db}
}

// EnqueuePushRetry implements service.PushRetryQueue.
// Idempotent: ON CONFLICT DO NOTHING ensures duplicate enqueue is a no-op.
func (q *DBPushRetryQueue) EnqueuePushRetry(
	ctx context.Context,
	notificationID, recipientID uuid.UUID,
	fcmToken, title, body string,
	data map[string]interface{},
) error {
	return EnqueuePushRetry(ctx, q.db, notificationID, recipientID, fcmToken, title, body, data)
}

// PushRetryWorker retries failed FCM push notifications with exponential backoff.
//
// Lifecycle: Start() / Stop() / IsRunning() satisfy serverboot.Worker.
// The worker owns its context; callers do not pass one.
type PushRetryWorker struct {
	db             *dbpkg.DB
	sender         PushSender
	deliveryLogger DeliveryLogger // Optional: for audit trail
	log            *zap.Logger

	pollInterval time.Duration
	batchSize    int

	mu          sync.RWMutex
	running     bool
	shutdownCtx context.Context
	cancelFn    context.CancelFunc
	wg          sync.WaitGroup
}

// NewPushRetryWorker creates a new PushRetryWorker.
func NewPushRetryWorker(db *dbpkg.DB, sender PushSender, log *zap.Logger) *PushRetryWorker {
	if log == nil {
		log = zap.NewNop()
	}
	return &PushRetryWorker{
		db:           db,
		sender:       sender,
		log:          log,
		pollInterval: DefaultPushRetryPollInterval,
		batchSize:    pushBatchSize,
	}
}

// SetDeliveryLogger wires an audit-trail logger. Optional; call before Start.
func (w *PushRetryWorker) SetDeliveryLogger(logger DeliveryLogger) {
	w.deliveryLogger = logger
}

// Start begins the push retry worker loop in the background.
// Idempotent: calling Start on an already-running worker is a no-op.
func (w *PushRetryWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("PushRetryWorker already running")
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.wg.Add(1)
	go w.run()

	w.log.Info("PushRetryWorker started",
		zap.Duration("poll_interval", w.pollInterval),
		zap.Int("batch_size", w.batchSize),
		zap.Int("max_attempts", maxPushRetryAttempts),
		zap.Duration("retry_window", pushRetryWindow),
	)
}

// Stop signals the worker to stop and waits for the current cycle to finish.
func (w *PushRetryWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.cancelFn()
	w.wg.Wait()
	w.running = false

	w.log.Info("PushRetryWorker stopped")
}

// IsRunning returns true if the worker loop is active.
func (w *PushRetryWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

func (w *PushRetryWorker) run() {
	defer w.wg.Done()

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.shutdownCtx.Done():
			return
		case <-ticker.C:
			if err := w.processBatch(w.shutdownCtx); err != nil {
				w.log.Error("Failed to process push retry batch", zap.Error(err))
			}
		}
	}
}

// processBatch processes a batch of pending push retries.
func (w *PushRetryWorker) processBatch(ctx context.Context) error {
	entries, err := w.fetchPendingRetries(ctx)
	if err != nil {
		return fmt.Errorf("fetch pending retries: %w", err)
	}

	if len(entries) == 0 {
		return nil
	}

	w.log.Debug("Processing push retry batch", zap.Int("count", len(entries)))

	for _, entry := range entries {
		if err := w.processEntry(ctx, entry); err != nil {
			w.log.Warn("Failed to process push retry entry",
				zap.String("entry_id", entry.ID.String()),
				zap.Error(err),
			)
		}
	}

	return nil
}

// pushRetryQueueEntry represents a pending push retry entry.
type pushRetryQueueEntry struct {
	ID             uuid.UUID
	NotificationID uuid.UUID
	RecipientID    uuid.UUID
	FCMToken       string
	Attempts       int
	NextAttemptAt  time.Time
	ExpiresAt      time.Time
	Title          string
	Body           string
	Data           map[string]interface{}
}

// fetchPendingRetries retrieves ready-to-retry entries from the queue.
func (w *PushRetryWorker) fetchPendingRetries(ctx context.Context) ([]pushRetryQueueEntry, error) {
	query := `
		SELECT
			id, notification_id, recipient_id, fcm_token,
			attempts, next_attempt_at, expires_at,
			title, body, data
		FROM push_retry_queue
		WHERE next_attempt_at <= NOW()
			AND expires_at > NOW()
		ORDER BY next_attempt_at ASC
		LIMIT $1
	`

	rows, err := w.db.Pool().Query(ctx, query, w.batchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []pushRetryQueueEntry
	for rows.Next() {
		var e pushRetryQueueEntry
		if err := rows.Scan(
			&e.ID, &e.NotificationID, &e.RecipientID, &e.FCMToken,
			&e.Attempts, &e.NextAttemptAt, &e.ExpiresAt,
			&e.Title, &e.Body, &e.Data,
		); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}

	return entries, rows.Err()
}

// processEntry attempts delivery for one retry entry.
func (w *PushRetryWorker) processEntry(ctx context.Context, entry pushRetryQueueEntry) error {
	if w.sender == nil {
		// No push sender (Firebase unavailable). Drop to avoid stale accumulation.
		w.logRetryDelivery(entry.NotificationID, entry.RecipientID, "skipped", entry.Attempts, "no push sender")
		return w.deleteEntry(ctx, entry.ID)
	}

	notif := map[string]interface{}{
		"id":           entry.NotificationID,
		"recipient_id": entry.RecipientID.String(),
		"type":         "push_retry",
	}

	err := w.sender.SendNotification(ctx, nil, notif, entry.Title, entry.Body)

	if err == nil {
		// Success — remove from queue.
		w.logRetryDelivery(entry.NotificationID, entry.RecipientID, "sent", entry.Attempts+1, "")
		return w.deleteEntry(ctx, entry.ID)
	}

	nextAttempt := w.calculateNextAttempt(entry.Attempts + 1)

	// Terminal: max attempts reached or next retry would exceed window.
	if entry.Attempts+1 >= maxPushRetryAttempts || nextAttempt.After(entry.ExpiresAt) {
		reason := fmt.Sprintf("exhausted after %d attempts", entry.Attempts+1)
		if nextAttempt.After(entry.ExpiresAt) {
			reason = "retry window exceeded"
		}
		w.logRetryDelivery(entry.NotificationID, entry.RecipientID, "failed", entry.Attempts+1, reason)
		w.log.Warn("PushRetryWorker: entry terminal",
			zap.String("entry_id", entry.ID.String()),
			zap.Int("attempts", entry.Attempts+1),
			zap.String("reason", reason),
		)
		return w.deleteEntry(ctx, entry.ID)
	}

	// Schedule next retry.
	w.logRetryDelivery(entry.NotificationID, entry.RecipientID, "retrying", entry.Attempts+1, err.Error())
	return w.updateRetryEntry(ctx, entry.ID, entry.Attempts+1, nextAttempt, err.Error())
}

// logRetryDelivery emits a delivery log event asynchronously (non-blocking).
func (w *PushRetryWorker) logRetryDelivery(
	notificationID, recipientID uuid.UUID,
	status string,
	attempt int,
	reason string,
) {
	if w.deliveryLogger == nil {
		return
	}

	metadata := map[string]interface{}{
		"retry_attempt": attempt,
		"max_attempts":  maxPushRetryAttempts,
	}

	go func() {
		w.deliveryLogger.LogDelivery(
			context.Background(),
			notificationID, recipientID,
			"push_retry", status,
			reason, metadata,
		)
	}()
}

// calculateNextAttempt returns the next retry time using exponential backoff with ±20% jitter.
func (w *PushRetryWorker) calculateNextAttempt(attempt int) time.Time {
	var delay time.Duration
	switch attempt {
	case 1:
		delay = 1 * time.Minute
	case 2:
		delay = 5 * time.Minute
	case 3:
		delay = 15 * time.Minute
	case 4:
		delay = 1 * time.Hour
	case 5:
		delay = 4 * time.Hour
	default:
		delay = 8 * time.Hour
	}

	// ±20% jitter to avoid thundering herd.
	jitter := time.Duration(float64(delay) * 0.2 * (2.0*float64(time.Now().UnixNano()%1000)/1000.0 - 1.0))
	return time.Now().Add(delay + jitter)
}

func (w *PushRetryWorker) deleteEntry(ctx context.Context, id uuid.UUID) error {
	_, err := w.db.Pool().Exec(ctx, `DELETE FROM push_retry_queue WHERE id = $1`, id)
	return err
}

func (w *PushRetryWorker) updateRetryEntry(ctx context.Context, id uuid.UUID, attempts int, nextAttempt time.Time, lastError string) error {
	query := `
		UPDATE push_retry_queue
		SET attempts = $2,
		    next_attempt_at = $3,
		    last_error = $4,
		    updated_at = NOW()
		WHERE id = $1
	`
	_, err := w.db.Pool().Exec(ctx, query, id, attempts, nextAttempt, lastError)
	return err
}

// EnqueuePushRetry inserts a new entry into the push_retry_queue.
// Called by PushService on initial FCM send failure.
// Idempotent: ON CONFLICT DO NOTHING prevents duplicate retry entries
// for the same (notification_id, fcm_token) within the 24h window.
func EnqueuePushRetry(
	ctx context.Context,
	db *dbpkg.DB,
	notificationID, recipientID uuid.UUID,
	fcmToken, title, body string,
	data map[string]interface{},
) error {
	query := `
		INSERT INTO push_retry_queue (
			notification_id, recipient_id, fcm_token,
			attempts, max_attempts, next_attempt_at, expires_at,
			title, body, data
		) VALUES ($1, $2, $3, 0, $4, NOW(), NOW() + INTERVAL '24 hours', $5, $6, $7)
		ON CONFLICT DO NOTHING
	`

	_, err := db.Pool().Exec(ctx, query,
		notificationID, recipientID, fcmToken,
		maxPushRetryAttempts, title, body, data,
	)
	return err
}


