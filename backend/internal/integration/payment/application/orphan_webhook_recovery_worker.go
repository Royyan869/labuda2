package application

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	alertapp "github.com/labuda/backend/internal/platform/alert/application"
	alertentity "github.com/labuda/backend/internal/platform/alert/entity"
	"github.com/labuda/backend/internal/worker"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/midtrans"
	"go.uber.org/zap"
)

// OrphanWebhookRecoveryWorker handles recovery of orphaned webhook events.
//
// ORPHAN WEBHOOK SCENARIO:
// Webhook arrives BEFORE payment is committed to database (race condition).
// This can happen when:
// 1. Payment is created in transaction T1
// 2. Webhook arrives immediately after Midtrans creates the payment
// 3. Webhook processing in transaction T2 can't find the payment (T1 not committed yet)
// 4. Webhook is marked as "orphaned"
//
// RECOVERY STRATEGY:
// 1. Periodically scan for orphaned webhook events
// 2. Retry payment lookup with delay (allowing transaction to commit)
// 3. If payment found, process the webhook normally
// 4. If payment still not found after max retries, keep for manual reconciliation
type OrphanWebhookRecoveryWorker struct {
	db                           *db.DB
	midtransClient               *midtrans.Client
	settlementService            *repository.PaymentSettlementService
	paymentRepo                  *repository.PaymentRepository
	webhookService               *PaymentWebhookService
	canonicalFinalizationService *CanonicalFinalizationService
	alertService                 RecoveryAlertService
	metrics                      worker.OrphanWebhookRecoveryMetricsRecorder
	log                          *zap.Logger
	enabled                      bool

	// Recovery configuration
	maxRetries    int           // Maximum retry attempts for orphaned webhooks
	retryInterval time.Duration // Delay between retry attempts
	batchSize     int           // Number of orphans to process per run
	scanInterval  time.Duration // How often to scan for orphans
	minRetryDelay time.Duration // Minimum delay before a retry is eligible
}

// RecoveryAlertService is the sink-only alert contract used by the orphan
// webhook recovery worker. It never influences recovery decisions.
type RecoveryAlertService interface {
	CreateAlert(
		ctx context.Context,
		alertType alertentity.AlertType,
		severity alertentity.AlertSeverity,
		entityType string,
		entityID uuid.UUID,
		message string,
		metadata alertentity.AlertMetadata,
		groupKey *string,
	) (*alertapp.CreateAlertResult, error)
}

// OrphanWebhookRecoveryConfig controls worker cadence and activation posture.
type OrphanWebhookRecoveryConfig struct {
	Enabled       bool
	MaxRetries    int
	RetryInterval time.Duration
	BatchSize     int
	ScanInterval  time.Duration
	MinRetryDelay time.Duration
}

// DefaultOrphanWebhookRecoveryConfig returns the safe default posture.
func DefaultOrphanWebhookRecoveryConfig() OrphanWebhookRecoveryConfig {
	return OrphanWebhookRecoveryConfig{
		Enabled:       false,
		MaxRetries:    5,
		RetryInterval: 5 * time.Second,
		BatchSize:     100,
		ScanInterval:  30 * time.Second,
		MinRetryDelay: 2 * time.Second,
	}
}

// LoadOrphanWebhookRecoveryConfigFromEnv builds the worker config from env.
// The default posture is OFF; DISABLE_ORPHAN_WEBHOOK_RECOVERY_WORKER=false
// enables the worker.
func LoadOrphanWebhookRecoveryConfigFromEnv() OrphanWebhookRecoveryConfig {
	cfg := DefaultOrphanWebhookRecoveryConfig()

	raw := strings.TrimSpace(os.Getenv("DISABLE_ORPHAN_WEBHOOK_RECOVERY_WORKER"))
	switch strings.ToLower(raw) {
	case "0", "false", "no", "off":
		cfg.Enabled = true
	case "1", "true", "yes", "on":
		cfg.Enabled = false
	}

	if v := parseEnvDuration("ORPHAN_WEBHOOK_RECOVERY_SCAN_INTERVAL"); v > 0 {
		cfg.ScanInterval = v
	}
	if v := parseEnvDuration("ORPHAN_WEBHOOK_RECOVERY_RETRY_INTERVAL"); v > 0 {
		cfg.RetryInterval = v
	}
	if v := parseEnvDuration("ORPHAN_WEBHOOK_RECOVERY_MIN_RETRY_DELAY"); v > 0 {
		cfg.MinRetryDelay = v
	}
	if v := parseEnvInt("ORPHAN_WEBHOOK_RECOVERY_BATCH_SIZE"); v > 0 {
		cfg.BatchSize = v
	}
	if v := parseEnvInt("ORPHAN_WEBHOOK_RECOVERY_MAX_RETRIES"); v > 0 {
		cfg.MaxRetries = v
	}

	return cfg
}

// NewOrphanWebhookRecoveryWorker creates a new recovery worker.
func NewOrphanWebhookRecoveryWorker(
	db *db.DB,
	midtransClient *midtrans.Client,
	webhookService *PaymentWebhookService,
	canonicalFinalizationService *CanonicalFinalizationService,
	log *zap.Logger,
	cfg OrphanWebhookRecoveryConfig,
) *OrphanWebhookRecoveryWorker {
	if log == nil {
		log = zap.NewNop()
	}
	cfg = normalizeOrphanWebhookRecoveryConfig(cfg)
	return &OrphanWebhookRecoveryWorker{
		db:                           db,
		midtransClient:               midtransClient,
		settlementService:            repository.NewPaymentSettlementService(),
		paymentRepo:                  repository.NewPaymentRepository(),
		webhookService:               webhookService,
		canonicalFinalizationService: canonicalFinalizationService,
		log:                          log,
		enabled:                      cfg.Enabled,
		maxRetries:                   cfg.MaxRetries,
		retryInterval:                cfg.RetryInterval,
		batchSize:                    cfg.BatchSize,
		scanInterval:                 cfg.ScanInterval,
		minRetryDelay:                cfg.MinRetryDelay,
	}
}

func normalizeOrphanWebhookRecoveryConfig(cfg OrphanWebhookRecoveryConfig) OrphanWebhookRecoveryConfig {
	defaults := DefaultOrphanWebhookRecoveryConfig()

	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = defaults.MaxRetries
	}
	if cfg.RetryInterval <= 0 {
		cfg.RetryInterval = defaults.RetryInterval
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = defaults.BatchSize
	}
	if cfg.ScanInterval <= 0 {
		cfg.ScanInterval = defaults.ScanInterval
	}
	if cfg.MinRetryDelay <= 0 {
		cfg.MinRetryDelay = defaults.MinRetryDelay
	}

	return cfg
}

// SetMetricsRecorder attaches an optional sink-only metrics recorder.
func (w *OrphanWebhookRecoveryWorker) SetMetricsRecorder(metrics worker.OrphanWebhookRecoveryMetricsRecorder) {
	w.metrics = metrics
}

// SetAlertService attaches an optional alert service for recovery anomalies.
func (w *OrphanWebhookRecoveryWorker) SetAlertService(alertService RecoveryAlertService) {
	w.alertService = alertService
}

// Start begins the orphan webhook recovery process.
// This should be run as a background goroutine.
func (w *OrphanWebhookRecoveryWorker) Start(ctx context.Context) {
	if !w.enabled {
		w.log.Info("Orphan webhook recovery worker disabled",
			zap.Duration("scan_interval", w.scanInterval),
			zap.Int("batch_size", w.batchSize),
			zap.Int("max_retries", w.maxRetries),
		)
		return
	}

	ticker := time.NewTicker(w.scanInterval)
	defer ticker.Stop()

	if w.metrics != nil {
		w.metrics.SetWorkerRunning(worker.WorkerNameOrphanWebhookRecovery, true)
		defer w.metrics.SetWorkerRunning(worker.WorkerNameOrphanWebhookRecovery, false)
	}

	w.log.Info("Orphan webhook recovery worker started",
		zap.Int("max_retries", w.maxRetries),
		zap.Duration("retry_interval", w.retryInterval),
		zap.Duration("scan_interval", w.scanInterval),
		zap.Duration("min_retry_delay", w.minRetryDelay),
	)

	for {
		select {
		case <-ctx.Done():
			w.log.Info("Orphan webhook recovery worker stopped")
			return
		case <-ticker.C:
			if err := w.RecoverOrphans(ctx); err != nil {
				w.log.Error("Failed to recover orphaned webhooks", zap.Error(err))
			}
		}
	}
}

// RecoverOrphans processes orphaned webhook events.
// Attempts to find the payment and process the webhook.
func (w *OrphanWebhookRecoveryWorker) RecoverOrphans(ctx context.Context) error {
	start := time.Now()
	outcome := worker.OrphanWebhookOutcomeIdle
	defer func() {
		if w.metrics != nil {
			w.metrics.RecordWorkerHeartbeat(worker.WorkerNameOrphanWebhookRecovery)
			w.metrics.RecordOrphanWebhookProcessingDuration(outcome, time.Since(start))
		}
	}()

	// Get batch of orphaned webhook events (use transaction for query)
	var orphanedEvents []repository.OrphanWebhookEvent
	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		orphanedEvents, err = w.paymentRepo.GetOrphanedWebhookEvents(ctx, tx, w.batchSize)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to get orphaned webhook events: %w", err)
	}

	if len(orphanedEvents) == 0 {
		if w.metrics != nil {
			w.metrics.SetOrphanWebhookBacklog(0)
		}
		return nil // No orphans to process
	}

	outcome = worker.OrphanWebhookOutcomeRecovered
	if w.metrics != nil {
		w.metrics.SetOrphanWebhookBacklog(len(orphanedEvents))
	}

	w.log.Info("Processing orphaned webhook events",
		zap.Int("count", len(orphanedEvents)),
	)

	// Process each orphaned event
	for _, event := range orphanedEvents {
		if err := w.recoverSingleOrphan(ctx, event); err != nil {
			w.log.Error("Failed to recover orphaned webhook",
				zap.String("event_id", event.EventID),
				zap.String("midtrans_order_id", event.MidtransOrderID),
				zap.Error(err),
			)
		}
	}

	return nil
}

// recoverSingleOrphan attempts to recover a single orphaned webhook event.
func (w *OrphanWebhookRecoveryWorker) recoverSingleOrphan(
	ctx context.Context,
	event repository.OrphanWebhookEvent,
) error {
	// STEP 1: Try to find payment by midtrans_order_id
	var payment *repository.Payment
	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		payment, err = w.paymentRepo.GetByMidtransOrderID(ctx, tx, event.MidtransOrderID)
		return err
	})
	if err == nil {
		// Payment found! Process the webhook now.
		w.log.Info("Payment found for orphaned webhook, processing",
			zap.String("event_id", event.EventID),
			zap.String("payment_id", payment.ID.String()),
		)

		// Parse the webhook payload
		var notification midtrans.NotificationPayload
		if err := json.Unmarshal(event.Payload, &notification); err != nil {
			if quarantineErr := w.quarantineOrphanWebhookPayload(
				ctx,
				event,
				&payment.ID,
				"malformed_payload",
				"Malformed orphan webhook payload quarantined for manual review",
				alertentity.SeverityWarning,
				alertentity.AlertMetadata{
					"error":             err.Error(),
					"required_action":   "manual_review",
					"issue_type":        "malformed_payload",
					"event_id":          event.EventID,
					"midtrans_order_id": event.MidtransOrderID,
				},
				fmt.Sprintf("malformed orphan payload: %v", err),
				"Malformed orphan webhook payload quarantined",
				err,
			); quarantineErr != nil {
				return quarantineErr
			}
			return nil
		}

		if validationErr := validateRecoveredWebhookPayload(event, payment, &notification); validationErr != nil {
			if quarantineErr := w.quarantineOrphanWebhookPayload(
				ctx,
				event,
				&payment.ID,
				validationErr.issueType,
				"Malformed orphan webhook payload quarantined for manual review",
				alertentity.SeverityWarning,
				validationErr.metadata,
				validationErr.reason,
				"Malformed orphan webhook payload quarantined",
				nil,
			); quarantineErr != nil {
				return quarantineErr
			}
			return nil
		}

		// Process the webhook normally
		// Use the existing webhook service for consistency
		return w.processRecoveredWebhook(ctx, &notification, payment.ID)
	}

	// STEP 2: Payment still not found - check retry count
	if err.Error() != "no rows in result set" {
		return fmt.Errorf("failed to lookup payment: %w", err)
	}

	// STEP 3: MINIMUM DELAY CHECK - ensure minimum delay before first retry
	elapsed := time.Since(event.ReceivedAt)
	minRetryDelay := w.minRetryDelay
	if minRetryDelay <= 0 {
		minRetryDelay = 2 * time.Second
	}

	if elapsed < minRetryDelay {
		// Too soon for retry - skip and let next scan cycle handle it
		w.log.Debug("Orphan webhook too soon for retry, skipping",
			zap.String("event_id", event.EventID),
			zap.String("midtrans_order_id", event.MidtransOrderID),
			zap.Duration("elapsed", elapsed),
			zap.Duration("min_retry_delay", minRetryDelay),
		)
		return nil // Skip this time, will be retried in next scan cycle
	}

	// STEP 4: Calculate retry count based on received_at
	retries := int(elapsed / w.retryInterval)
	if retries >= w.maxRetries {
		// Max retries reached - move to terminal review queue for manual reconciliation
		errMsg := fmt.Sprintf("max retries reached: %d", retries)
		if markErr := w.markWebhookEventStatus(ctx, event.EventID, repository.PaymentWebhookEventStatusTerminalReview, nil, &errMsg); markErr != nil {
			return fmt.Errorf("failed to move webhook to terminal review: %w", markErr)
		}
		w.recordOutcome(worker.OrphanWebhookOutcomeTerminalFailure, 1)
		w.emitRecoveryAlert(
			ctx,
			"terminal_retry_exhausted",
			alertentity.SeverityHigh,
			event,
			nil,
			"Orphan webhook exhausted retries and entered terminal review",
			alertentity.AlertMetadata{
				"required_action":   "manual_review",
				"issue_type":        "terminal_retry_exhausted",
				"event_id":          event.EventID,
				"midtrans_order_id": event.MidtransOrderID,
				"retries":           retries,
				"max_retries":       w.maxRetries,
			},
		)
		w.log.Warn("Orphan webhook exceeded max retries, moved to terminal review",
			zap.String("event_id", event.EventID),
			zap.String("midtrans_order_id", event.MidtransOrderID),
			zap.Int("retries", retries),
			zap.Int("max_retries", w.maxRetries),
		)
		return nil
	}

	// STEP 5: Not yet max retries - schedule for retry
	// Mark as pending for next processing cycle
	err = w.db.WithTx(ctx, func(tx db.Tx) error {
		return w.paymentRepo.MarkWebhookEventForRetry(ctx, tx, event.EventID)
	})
	if err != nil {
		return fmt.Errorf("failed to mark webhook for retry: %w", err)
	}

	w.recordOutcome(worker.OrphanWebhookOutcomeRetry, 1)
	w.log.Info("Orphan webhook scheduled for retry",
		zap.String("event_id", event.EventID),
		zap.String("midtrans_order_id", event.MidtransOrderID),
		zap.Int("retry_attempt", retries+1),
	)

	return nil
}

// processRecoveredWebhook processes a webhook that was previously orphaned.
func (w *OrphanWebhookRecoveryWorker) processRecoveredWebhook(
	ctx context.Context,
	notification *midtrans.NotificationPayload,
	paymentID uuid.UUID,
) error {
	// Use db.WithTx for automatic retry on serialization errors
	return w.db.WithTx(ctx, func(tx db.Tx) error {
		// Update webhook event status to processing
		eventID := notification.TransactionID
		if err := w.paymentRepo.UpdateWebhookEventStatus(ctx, tx, eventID, repository.PaymentWebhookEventStatusProcessing, &paymentID, nil); err != nil {
			return fmt.Errorf("failed to update webhook event to processing: %w", err)
		}

		// Get payment for update to prevent race conditions
		payment, err := w.paymentRepo.GetByIDForUpdate(ctx, tx, paymentID)
		if err != nil {
			return fmt.Errorf("failed to get payment for update: %w", err)
		}

		// Check if payment is already processed
		if !payment.IsPending() {
			w.log.Info("Payment already processed, marking webhook as succeeded",
				zap.String("payment_id", payment.ID.String()),
				zap.String("status", payment.Status),
			)
			// Mark webhook as succeeded
			_ = w.paymentRepo.UpdateWebhookEventStatus(ctx, tx, eventID, repository.PaymentWebhookEventStatusSucceeded, &paymentID, nil)
			w.recordOutcome(worker.OrphanWebhookOutcomeRecovered, 1)
			return nil
		}

		// Process the payment based on transaction status
		if w.midtransClient.IsTransactionSuccess(notification.TransactionStatus) {
			// Double-check payment status before canonical finalization
			currentStatus, err := w.paymentRepo.GetStatus(ctx, tx, payment.ID)
			if err == nil && currentStatus != repository.PaymentStatusPending {
				w.log.Info("Payment status changed during recovery, aborting",
					zap.String("payment_id", payment.ID.String()),
					zap.String("status", currentStatus),
				)
				_ = w.paymentRepo.UpdateWebhookEventStatus(ctx, tx, eventID, repository.PaymentWebhookEventStatusSucceeded, &paymentID, nil)
				w.recordOutcome(worker.OrphanWebhookOutcomeRecovered, 1)
				return nil
			}

			// Canonical order-payment finalization
			transactionID := notification.TransactionID
			paymentType := notification.PaymentType

			if payment.ReferenceType == repository.ReferenceTypeOrder && payment.ReferenceID != nil && *payment.ReferenceID != uuid.Nil {
				if w.canonicalFinalizationService == nil {
					errMsg := "CRITICAL: canonical finalization service not wired"
					_ = w.paymentRepo.UpdateWebhookEventStatus(ctx, tx, eventID, repository.PaymentWebhookEventStatusFailed, &paymentID, &errMsg)
					w.recordOutcome(worker.OrphanWebhookOutcomeFailed, 1)
					w.emitRecoveryAlert(
						ctx,
						"canonical_finalization_not_wired",
						alertentity.SeverityCritical,
						repository.OrphanWebhookEvent{EventID: eventID, MidtransOrderID: notification.OrderID},
						&paymentID,
						"Canonical finalization service is not wired",
						alertentity.AlertMetadata{
							"required_action":   "wire_canonical_finalization_service",
							"issue_type":        "canonical_finalization_not_wired",
							"event_id":          eventID,
							"payment_id":        payment.ID.String(),
							"midtrans_order_id": notification.OrderID,
						},
					)
					return fmt.Errorf("CRITICAL: canonical finalization service not wired")
				}

				if err := w.canonicalFinalizationService.FinalizeOrderPayment(
					ctx,
					tx,
					payment,
					transactionID,
					paymentType,
				); err != nil {
					errMsg := fmt.Sprintf("CRITICAL: failed to finalize order payment: %v", err)
					_ = w.paymentRepo.UpdateWebhookEventStatus(ctx, tx, eventID, repository.PaymentWebhookEventStatusFailed, &paymentID, &errMsg)
					w.recordOutcome(worker.OrphanWebhookOutcomeFailed, 1)
					w.emitRecoveryAlert(
						ctx,
						"canonical_finalization_failed",
						alertentity.SeverityHigh,
						repository.OrphanWebhookEvent{EventID: eventID, MidtransOrderID: notification.OrderID},
						&paymentID,
						"Canonical finalization failed during orphan recovery",
						alertentity.AlertMetadata{
							"required_action":    "investigate_canonical_finalization_failure",
							"issue_type":         "canonical_finalization_failed",
							"event_id":           eventID,
							"payment_id":         payment.ID.String(),
							"midtrans_order_id":  notification.OrderID,
							"transaction_status": notification.TransactionStatus,
						},
					)
					return fmt.Errorf("CRITICAL: failed to finalize order payment: %w", err)
				}

				w.log.Info("Orphaned webhook recovered and order payment finalized",
					zap.String("payment_id", payment.ID.String()),
				)
				w.recordOutcome(worker.OrphanWebhookOutcomeRecovered, 1)

			} else {
				if err := w.settlementService.SettlePaymentByID(ctx, tx, payment.ID, transactionID, paymentType); err != nil {
					errMsg := fmt.Sprintf("CRITICAL: failed to settle payment: %v", err)
					_ = w.paymentRepo.UpdateWebhookEventStatus(ctx, tx, eventID, repository.PaymentWebhookEventStatusFailed, &paymentID, &errMsg)
					w.recordOutcome(worker.OrphanWebhookOutcomeFailed, 1)
					return fmt.Errorf("CRITICAL: failed to settle payment: %w", err)
				}

				w.log.Info("Orphaned webhook recovered and payment settled",
					zap.String("payment_id", payment.ID.String()),
				)
				w.recordOutcome(worker.OrphanWebhookOutcomeRecovered, 1)
			}
		} else if w.midtransClient.IsTransactionFailed(notification.TransactionStatus) {
			// Mark payment as failed
			if err := w.settlementService.FailPayment(ctx, tx, notification.OrderID, notification.TransactionStatus); err != nil {
				w.log.Error("Failed to mark payment as failed during recovery",
					zap.String("payment_id", payment.ID.String()),
					zap.Error(err),
				)
			}
			w.recordOutcome(worker.OrphanWebhookOutcomeFailed, 1)
		} else if w.midtransClient.IsTransactionPending(notification.TransactionStatus) {
			// Still pending - no action needed
			_ = w.paymentRepo.UpdateWebhookEventStatus(ctx, tx, eventID, repository.PaymentWebhookEventStatusSucceeded, &paymentID, nil)
			w.recordOutcome(worker.OrphanWebhookOutcomeRecovered, 1)
			return nil
		} else {
			// Unknown gateway status: manual review, never silently succeed.
			errMsg := fmt.Sprintf("unknown transaction status: %s", notification.TransactionStatus)
			_ = w.paymentRepo.UpdateWebhookEventStatus(ctx, tx, eventID, repository.PaymentWebhookEventStatusManualReview, &paymentID, &errMsg)
			w.recordOutcome(worker.OrphanWebhookOutcomeManualReview, 1)
			w.emitRecoveryAlert(
				ctx,
				"unknown_gateway_status",
				alertentity.SeverityWarning,
				repository.OrphanWebhookEvent{EventID: eventID, MidtransOrderID: notification.OrderID},
				&paymentID,
				"Unknown gateway transaction status routed to manual review",
				alertentity.AlertMetadata{
					"required_action":    "manual_review",
					"issue_type":         "unknown_gateway_status",
					"event_id":           eventID,
					"payment_id":         payment.ID.String(),
					"midtrans_order_id":  notification.OrderID,
					"transaction_status": notification.TransactionStatus,
				},
			)
			w.log.Warn("Unknown gateway status routed to manual review",
				zap.String("payment_id", payment.ID.String()),
				zap.String("event_id", eventID),
				zap.String("transaction_status", notification.TransactionStatus),
			)
			return nil
		}

		// Mark webhook event as succeeded
		if err := w.paymentRepo.UpdateWebhookEventStatus(ctx, tx, eventID, repository.PaymentWebhookEventStatusSucceeded, &paymentID, nil); err != nil {
			w.log.Error("Failed to mark webhook event as succeeded", zap.Error(err))
			// Non-critical - payment already updated
		}

		return nil
	})
}

// GetOrphanWebhookStats returns statistics about orphaned webhooks.
// Useful for monitoring and alerting.
func (w *OrphanWebhookRecoveryWorker) GetOrphanWebhookStats(ctx context.Context) (*OrphanWebhookStats, error) {
	var orphanedEvents []repository.OrphanWebhookEvent
	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		orphanedEvents, err = w.paymentRepo.GetOrphanedWebhookEvents(ctx, tx, 1000) // Get up to 1000 for stats
		return err
	})
	if err != nil {
		return nil, err
	}

	stats := &OrphanWebhookStats{
		TotalOrphans: len(orphanedEvents),
	}

	// Categorize by age
	now := time.Now()
	for _, event := range orphanedEvents {
		age := now.Sub(event.ReceivedAt)
		if age < 5*time.Minute {
			stats.RecentOrphans++
		} else if age < 1*time.Hour {
			stats.StaleOrphans++
		} else {
			stats.AncientOrphans++
		}
	}

	if w.metrics != nil {
		w.metrics.SetOrphanWebhookBacklog(stats.TotalOrphans)
	}

	return stats, nil
}

// OrphanWebhookStats contains statistics about orphaned webhooks.
type OrphanWebhookStats struct {
	TotalOrphans   int // Total orphaned webhook events
	RecentOrphans  int // Orphans less than 5 minutes old
	StaleOrphans   int // Orphans between 5 minutes and 1 hour old
	AncientOrphans int // Orphans more than 1 hour old (needs attention)
}

func (w *OrphanWebhookRecoveryWorker) markWebhookEventStatus(
	ctx context.Context,
	eventID string,
	status string,
	paymentID *uuid.UUID,
	errorMsg *string,
) error {
	return w.db.WithTx(ctx, func(tx db.Tx) error {
		return w.paymentRepo.UpdateWebhookEventStatus(ctx, tx, eventID, status, paymentID, errorMsg)
	})
}

type orphanWebhookPayloadValidationError struct {
	issueType string
	reason    string
	metadata  alertentity.AlertMetadata
}

func (e *orphanWebhookPayloadValidationError) Error() string {
	return e.reason
}

func validateRecoveredWebhookPayload(
	event repository.OrphanWebhookEvent,
	payment *repository.Payment,
	notification *midtrans.NotificationPayload,
) *orphanWebhookPayloadValidationError {
	if notification == nil {
		return &orphanWebhookPayloadValidationError{
			issueType: "malformed_payload",
			reason:    "notification payload is nil",
			metadata: alertentity.AlertMetadata{
				"required_action":   "manual_review",
				"issue_type":        "malformed_payload",
				"event_id":          event.EventID,
				"midtrans_order_id": event.MidtransOrderID,
			},
		}
	}

	trimmedTransactionID := strings.TrimSpace(notification.TransactionID)
	if trimmedTransactionID == "" {
		return newOrphanWebhookPayloadValidationError(event, payment, "missing_transaction_id", "transaction_id is required")
	}
	trimmedOrderID := strings.TrimSpace(notification.OrderID)
	if trimmedOrderID == "" {
		return newOrphanWebhookPayloadValidationError(event, payment, "missing_order_id", "order_id is required")
	}
	trimmedPaymentType := strings.TrimSpace(notification.PaymentType)
	if trimmedPaymentType == "" {
		return newOrphanWebhookPayloadValidationError(event, payment, "missing_required_field", "payment_type is required")
	}
	trimmedStatus := strings.TrimSpace(notification.TransactionStatus)
	if trimmedStatus == "" {
		return newOrphanWebhookPayloadValidationError(event, payment, "missing_required_field", "transaction_status is required")
	}

	if payment == nil || payment.ID == uuid.Nil {
		return newOrphanWebhookPayloadValidationError(event, payment, "missing_payment_identifier", "payment identifier is required")
	}

	referenceType := strings.TrimSpace(payment.ReferenceType)
	switch referenceType {
	case repository.ReferenceTypeOrder, repository.ReferenceTypeBilling, repository.ReferenceTypeSubscription:
	default:
		return newOrphanWebhookPayloadValidationError(event, payment, "invalid_reference_type", "invalid payment reference_type")
	}

	if referenceType == repository.ReferenceTypeOrder {
		if payment.ReferenceID == nil || *payment.ReferenceID == uuid.Nil {
			return newOrphanWebhookPayloadValidationError(event, payment, "missing_payment_identifier", "order payment reference_id is required")
		}
	}

	if strings.TrimSpace(event.MidtransOrderID) == "" {
		return newOrphanWebhookPayloadValidationError(event, payment, "malformed_payload", "midtrans order id is required")
	}

	if event.MidtransOrderID != trimmedOrderID {
		return newOrphanWebhookPayloadValidationError(event, payment, "malformed_payload", "payload order_id does not match orphan event")
	}

	return nil
}

func newOrphanWebhookPayloadValidationError(
	event repository.OrphanWebhookEvent,
	payment *repository.Payment,
	issueType string,
	message string,
) *orphanWebhookPayloadValidationError {
	metadata := alertentity.AlertMetadata{
		"required_action":   "manual_review",
		"issue_type":        issueType,
		"event_id":          event.EventID,
		"midtrans_order_id": event.MidtransOrderID,
	}
	if payment != nil {
		metadata["payment_id"] = payment.ID.String()
		metadata["reference_type"] = payment.ReferenceType
	}

	return &orphanWebhookPayloadValidationError{
		issueType: issueType,
		reason:    message,
		metadata:  metadata,
	}
}

func (w *OrphanWebhookRecoveryWorker) quarantineOrphanWebhookPayload(
	ctx context.Context,
	event repository.OrphanWebhookEvent,
	paymentID *uuid.UUID,
	issueType string,
	alertMessage string,
	severity alertentity.AlertSeverity,
	metadata alertentity.AlertMetadata,
	errMessage string,
	logMessage string,
	err error,
) error {
	if errMessage == "" {
		errMessage = alertMessage
	}
	if markErr := w.markWebhookEventStatus(ctx, event.EventID, repository.PaymentWebhookEventStatusQuarantined, paymentID, &errMessage); markErr != nil {
		return fmt.Errorf("failed to quarantine malformed payload: %w", markErr)
	}
	w.recordOutcome(worker.OrphanWebhookOutcomeQuarantined, 1)
	w.emitRecoveryAlert(
		ctx,
		issueType,
		severity,
		event,
		paymentID,
		alertMessage,
		metadata,
	)
	if err != nil {
		w.log.Warn(logMessage,
			zap.String("event_id", event.EventID),
			zap.String("payment_id", paymentIDString(paymentID)),
			zap.Error(err),
		)
		return nil
	}
	w.log.Warn(logMessage,
		zap.String("event_id", event.EventID),
		zap.String("payment_id", paymentIDString(paymentID)),
	)
	return nil
}

func paymentIDString(paymentID *uuid.UUID) string {
	if paymentID == nil {
		return ""
	}
	return paymentID.String()
}

func (w *OrphanWebhookRecoveryWorker) recordOutcome(outcome string, count int) {
	if w.metrics == nil || count <= 0 {
		return
	}

	switch outcome {
	case worker.OrphanWebhookOutcomeRecovered:
		w.metrics.RecordOrphanWebhookRecovered(count)
	case worker.OrphanWebhookOutcomeRetry:
		w.metrics.RecordOrphanWebhookRetry(count)
	case worker.OrphanWebhookOutcomeFailed:
		w.metrics.RecordOrphanWebhookFailed(count)
	case worker.OrphanWebhookOutcomeManualReview:
		w.metrics.RecordOrphanWebhookManualReview(count)
	case worker.OrphanWebhookOutcomeQuarantined:
		w.metrics.RecordOrphanWebhookQuarantined(count)
	case worker.OrphanWebhookOutcomeTerminalFailure:
		w.metrics.RecordOrphanWebhookTerminalFailure(count)
	}
}

func (w *OrphanWebhookRecoveryWorker) emitRecoveryAlert(
	ctx context.Context,
	issueType string,
	severity alertentity.AlertSeverity,
	event repository.OrphanWebhookEvent,
	paymentID *uuid.UUID,
	message string,
	metadata alertentity.AlertMetadata,
) {
	if w.alertService == nil {
		return
	}

	entitySeed := "orphan-webhook:" + event.EventID
	if paymentID != nil {
		entitySeed += ":" + paymentID.String()
	}
	entityID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(entitySeed))

	if metadata == nil {
		metadata = alertentity.AlertMetadata{}
	}
	metadata["issue_type"] = issueType
	metadata["event_id"] = event.EventID
	metadata["midtrans_order_id"] = event.MidtransOrderID
	if paymentID != nil {
		metadata["payment_id"] = paymentID.String()
	}

	if _, err := w.alertService.CreateAlert(
		ctx,
		alertentity.AlertTypeReconciliationDrift,
		severity,
		"payment_webhook_event",
		entityID,
		message,
		metadata,
		nil,
	); err != nil {
		w.log.Warn("Failed to create recovery alert",
			zap.String("issue_type", issueType),
			zap.String("event_id", event.EventID),
			zap.Error(err),
		)
	}
}

func parseEnvDuration(key string) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0
	}
	return d
}

func parseEnvInt(key string) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return v
}


