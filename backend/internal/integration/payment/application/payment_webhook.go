// ⚠️ INTEGRATION LAYER:
// This module is an external payment adapter.
// It does NOT contain business logic or money mutation.
//
// ⚠️ Payment domain does NOT handle money.
// It only handles gateway status and webhook processing.
package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	orderapp "github.com/labuda/backend/internal/commerce/order/application"
	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
	orderRepoImpl "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	subscriptionapp "github.com/labuda/backend/internal/commerce/subscription/application"
	escrowApp "github.com/labuda/backend/internal/core/escrow/application"
	financeApp "github.com/labuda/backend/internal/finance/application"
	billingapp "github.com/labuda/backend/internal/finance/billing/application"
	billingentity "github.com/labuda/backend/internal/finance/billing/entity"
	billingrepo "github.com/labuda/backend/internal/finance/billing/infrastructure/repository"
	refundapp "github.com/labuda/backend/internal/finance/refund/application"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	alertapp "github.com/labuda/backend/internal/platform/alert/application"
	alertentity "github.com/labuda/backend/internal/platform/alert/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/midtrans"
	"go.uber.org/zap"
)

// =============================================================================
// REC-1: PAYMENT WEBHOOK FAILURE DURABILITY
// =============================================================================

// ErrWebhookSignatureInvalid marks a webhook rejected at the signature gate,
// BEFORE any processing happened.
//
// It is a SECURITY rejection of unauthenticated input, not a processing failure,
// and is deliberately excluded from durable failure recording: persisting rows
// keyed by attacker-controlled event ids (and carrying attacker-controlled
// payloads) from an unauthenticated endpoint would let anyone grow the
// payment_webhook_events table without bound. Nothing was processed, so nothing
// needs recovering.
var ErrWebhookSignatureInvalid = errors.New("invalid webhook signature")

// webhookFailureRecordTimeout bounds the independent failure-recording write so
// a durable record can never hang the webhook request.
const webhookFailureRecordTimeout = 5 * time.Second

// webhookEventIsReprocessable reports whether an EXISTING payment_webhook_events
// row with this status must be processed again, instead of being treated as an
// idempotent duplicate.
//
// Only 'failed' is reprocessable. Before REC-1 recorded failures durably, a
// failed delivery left no row at all, so a redelivery was processed again; this
// predicate preserves that exact idempotency semantic now that a failed event
// does leave a row. Every other recorded status is terminal for redelivery
// purposes and short-circuits as it always has.
func webhookEventIsReprocessable(existingStatus string) bool {
	return existingStatus == repository.PaymentWebhookEventStatusFailed
}

// systemRoleChecker is a minimal RoleChecker implementation for system operations.
// For webhooks and other system-initiated operations, we use a simplified checker
// that only recognizes the system caller as having admin privileges.
type systemRoleChecker struct{}

func (s *systemRoleChecker) IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error) {
	return auth.IsSystemCaller(userID), nil
}

func (s *systemRoleChecker) IsSeller(ctx context.Context, userID uuid.UUID) (bool, error) {
	return auth.IsSystemCaller(userID), nil
}

func (s *systemRoleChecker) HasActiveSellerCapability(ctx context.Context, userID uuid.UUID) (bool, error) {
	return auth.IsSystemCaller(userID), nil
}

func (s *systemRoleChecker) HasSellerProfile(ctx context.Context, userID uuid.UUID) (bool, error) {
	return auth.IsSystemCaller(userID), nil
}

// systemAccountStatusChecker is a no-op AccountStatusChecker for system operations.
// Webhooks and workers use SystemCallerID, which bypasses account status checks.
type systemAccountStatusChecker struct{}

func (s *systemAccountStatusChecker) EnsureActive(ctx context.Context, userID uuid.UUID) error {
	return nil // System operations bypass account status checks
}

func (s *systemAccountStatusChecker) GetStatus(ctx context.Context, userID uuid.UUID) (string, error) {
	return "active", nil // System operations bypass account status checks
}

func (s *systemAccountStatusChecker) IsBanned(ctx context.Context, userID uuid.UUID) (bool, error) {
	return false, nil // System operations bypass account status checks
}

// PaymentWebhookService handles Midtrans webhook notifications
// Uses pgx-based repositories and DB layer (NO GORM)
//
// WIRING RULE: OrderService and EscrowService MUST be injected from
// dependencies_core.go. Never construct them here. Building local copies
// produces nil-deps and causes settlement to crash inside MarkPaid.
type PaymentWebhookService struct {
	db                           *db.DB
	midtransClient               *midtrans.Client
	settlementService            *repository.PaymentSettlementService
	paymentRepo                  *repository.PaymentRepository
	paymentAttemptRepo           *repository.PaymentAttemptRepository // BNR Phase 1: Payment attempt tracking
	orderService                 *orderapp.OrderService
	escrowService                *escrowApp.EscrowService
	orderRepo                    *orderRepoImpl.OrderRepository
	canonicalFinalizationService *CanonicalFinalizationService
	billingService               *billingapp.BillingService
	billingRepo                  *billingrepo.BillingRepository
	subscriptionPaymentService   *subscriptionapp.SellerSubscriptionPaymentService
	// refundService handles gateway refund acknowledgement webhooks.
	// Optional: nil leaves the refund branch as a structured-log no-op so
	// the webhook returns 200 to Midtrans (Phase 1 wiring is opt-in).
	refundService *refundapp.RefundService
	// financeService books the settlement funding ledger transaction
	// (DR GATEWAY_CLEARING / CR BANK_SETTLEMENT) inside the same webhook tx
	// as MarkAsSettlement + CreateEscrowFromGatewaySettlement (TASK 39E).
	// Optional only for transitional wiring; nil makes the order branch
	// fail-closed below to prevent escrow creation without funding.
	financeService *financeApp.FinanceService
	// alertService raises an operator alert for recovery anomalies.
	// Optional: nil disables alerting.
	alertService RecoveryAlertService
	log          *zap.Logger
}

// RecoveryAlertService is the sink-only alert contract consumed by the webhook
// service. It never influences processing decisions.
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

// NewPaymentWebhookService creates a new PaymentWebhookService.
//
// orderService and escrowService MUST be the canonical instances built in
// dependencies_core.go. Do not pass freshly-constructed instances.
//
// subscriptionPaymentService should be set via SetSubscriptionPaymentService
// after initialization.
func NewPaymentWebhookService(
	db *db.DB,
	midtransClient *midtrans.Client,
	orderService *orderapp.OrderService,
	escrowService *escrowApp.EscrowService,
	log *zap.Logger,
) *PaymentWebhookService {
	roleChecker := &systemRoleChecker{}
	accountStatusChecker := &systemAccountStatusChecker{}

	// Create settlement service
	settlementSvc := repository.NewPaymentSettlementService()

	// Create payment repo
	paymentRepo := repository.NewPaymentRepository()

	return &PaymentWebhookService{
		db:                 db,
		midtransClient:     midtransClient,
		settlementService:  settlementSvc,
		paymentRepo:        paymentRepo,
		paymentAttemptRepo: repository.NewPaymentAttemptRepository(log), // BNR Phase 1
		orderService:       orderService,
		escrowService:      escrowService,
		orderRepo:          orderRepoImpl.NewOrderRepository(),
		billingService:     billingapp.NewBillingService(roleChecker, accountStatusChecker),
		billingRepo:        billingrepo.NewBillingRepository(),
		log:                log,
	}
}

// SetSubscriptionPaymentService sets the subscription payment service.
// This is called during dependency injection after the subscription module is initialized.
func (s *PaymentWebhookService) SetSubscriptionPaymentService(service *subscriptionapp.SellerSubscriptionPaymentService) {
	s.subscriptionPaymentService = service
}

// SetRefundService wires the gateway-aware refund service used to dispatch
// refund acknowledgement webhooks (TASK 33 / Phase 1).
//
// Wiring is opt-in: leaving this unset keeps refund webhooks logged-and-
// ignored, which is the safe default while the kill-switch is still active
// for legacy refund paths.
func (s *PaymentWebhookService) SetRefundService(service *refundapp.RefundService) {
	s.refundService = service
}

// SetFinanceService wires the canonical FinanceService used to book the
// payment-settlement funding ledger transaction (TASK 39E). Wiring is
// MANDATORY for the gateway-funded settlement model: when unset, the order
// branch refuses to proceed to escrow creation rather than risk creating an
// unfunded escrow.
func (s *PaymentWebhookService) SetFinanceService(service *financeApp.FinanceService) {
	s.financeService = service
}

// SetCanonicalFinalizationService wires the shared canonical order-payment
// finalization service used by webhook, recovery, admin resync, and
// reconciliation entrypoints.
func (s *PaymentWebhookService) SetCanonicalFinalizationService(service *CanonicalFinalizationService) {
	s.canonicalFinalizationService = service
}

// SetAlertService wires the operator alert sink used to surface a gateway
// success notification that arrives for a payment/order the platform already
// SetAlertService wires the alert service for recovery anomaly alerting.
// Optional: nil disables alerting.
func (s *PaymentWebhookService) SetAlertService(service RecoveryAlertService) {
	s.alertService = service
}

// SetAuditService sets the audit service for audit logging.
// This is called during dependency injection to enable audit events.
func (s *PaymentWebhookService) SetAuditService(auditService interface { // Minimal interface to avoid circular import
	PaymentSettled(ctx context.Context, tx db.Tx, paymentID uuid.UUID, amount int64)
	PaymentFailed(ctx context.Context, tx db.Tx, paymentID uuid.UUID, reason string)
	PaymentCreated(ctx context.Context, tx db.Tx, paymentID, userID uuid.UUID, amount int64)
}) {
	s.settlementService.SetAuditService(auditService)
	s.paymentRepo.SetAuditService(auditService)
}

// HandleWebhook processes Midtrans webhook notification with financial-grade security
//
// CRITICAL SECURITY PATTERN:
//  1. INSERT webhook event FIRST (status=pending) - captures ALL events for audit
//  2. Duplicate notification = idempotent return (unique notification_key; a
//     DIFFERENT notification of the same gateway transaction is a distinct event
//     and is processed — REC-3)
//  3. Signature verification AFTER insert (prevents replay attack bypass)
//  4. State machine enforcement: pending -> processing -> succeeded/failed
//  5. db.WithTx handles DEADLOCK RETRY for PostgreSQL serialization errors
//  6. DOUBLE-CHECK payment status before ledger call (bulletproof guard)
//
// FAILURE DURABILITY (REC-1): processing happens inside db.WithTx, which
// ROLLS BACK on any error returned by the transaction function — including the
// payment_webhook_events row and the status='failed' write the transaction just
// made. A failure record therefore cannot be produced from inside that
// transaction. On failure this method performs a SECOND, independent write
// (recordWebhookFailureDurably) that runs after the rollback, so the fact that
// the webhook arrived and failed is never silently lost.
//
// Returns error if processing fails (non-idempotent failures)
func (s *PaymentWebhookService) HandleWebhook(
	ctx context.Context,
	notification *midtrans.NotificationPayload,
	clientIP string,
) error {
	// Use db.WithTx for automatic retry on serialization/deadlock errors
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		return s.handleWebhookInTransaction(ctx, tx, notification, clientIP)
	})
	if err == nil {
		return nil
	}

	// A signature rejection is a security rejection of unverified input, not a
	// processing failure: nothing was processed and nothing may be persisted for
	// an unauthenticated caller (see ErrWebhookSignatureInvalid).
	if errors.Is(err, ErrWebhookSignatureInvalid) {
		return err
	}

	// Every other failure is a genuine processing failure whose record was just
	// rolled back. Re-record it durably before reporting the error.
	s.recordWebhookFailureDurably(ctx, notification, clientIP, err)
	return err
}

// recordWebhookFailureDurably writes the failure record in its OWN transaction.
//
// WHY A SEPARATE TRANSACTION: db.WithTx always rolls back when the transaction
// function returns an error (pkg/db.withRetry). The rolled-back transaction
// contains both the event row insert and its status='failed' update, so neither
// survives. Only a fresh transaction on a separate pooled connection can leave a
// durable record — proven by
// TestWebhookFailureDurability_RollbackErasesInTransactionRecord in
// payment_webhook_failure_durability_integration_test.go.
//
// CONCURRENCY / CONSISTENCY:
//   - Runs strictly AFTER the processing transaction has rolled back, so it can
//     never observe or interfere with a half-applied processing transaction.
//   - Idempotent (upsert keyed on notification_key) and guarded so it can never
//     overwrite a committed terminal non-failure row: a concurrent successful
//     delivery of the same notification always wins the audit record.
//   - Uses a cancellation-detached context with a bounded timeout: the gateway
//     disconnecting its request must not be able to erase the evidence, and the
//     write must never hang the request either.
//
// A failure of this write is NEVER swallowed — it is logged at ERROR level
// because an unrecorded webhook failure is an audit gap.
func (s *PaymentWebhookService) recordWebhookFailureDurably(
	ctx context.Context,
	notification *midtrans.NotificationPayload,
	clientIP string,
	cause error,
) {
	if notification == nil {
		return
	}
	if s.paymentRepo == nil {
		s.log.Error("webhook_failure_record_unavailable",
			zap.String("order_id", notification.OrderID),
			zap.String("transaction_id", notification.TransactionID),
			zap.Error(cause),
		)
		return
	}

	eventID := notification.TransactionID
	notificationKey := midtrans.NotificationIdentity(notification)
	payload, _ := json.Marshal(notification)
	errorMessage := fmt.Sprintf("webhook processing failed: %v", cause)

	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), webhookFailureRecordTimeout)
	defer cancel()

	writeErr := s.db.WithTx(writeCtx, func(tx db.Tx) error {
		recorded, err := s.paymentRepo.RecordWebhookEventFailure(
			writeCtx,
			tx,
			eventID,
			notificationKey,
			notification.OrderID,
			notification.SignatureKey,
			payload,
			nil,
			errorMessage,
		)
		if err != nil {
			return err
		}
		if !recorded {
			s.log.Warn("webhook_failure_record_skipped",
				zap.String("notification_key", notificationKey),
				zap.String("order_id", notification.OrderID),
				zap.String("reason", "event already recorded in a terminal non-failure state"),
			)
		}
		return nil
	})
	if writeErr != nil {
		s.log.Error("webhook_failure_record_write_failed",
			zap.String("notification_key", notificationKey),
			zap.String("order_id", notification.OrderID),
			zap.String("transaction_id", notification.TransactionID),
			zap.Error(cause),
			zap.Error(writeErr),
		)
		return
	}

	s.log.Error("webhook_processing_failed",
		zap.String("notification_key", notificationKey),
		zap.String("order_id", notification.OrderID),
		zap.String("transaction_id", notification.TransactionID),
		zap.String("transaction_status", notification.TransactionStatus),
		zap.String("client_ip", clientIP),
		zap.Error(cause),
		zap.String("durable_record", "payment_webhook_events.status=failed"),
	)
}

// handleWebhookInTransaction contains the core webhook processing logic
// MUST be called within db.WithTx for retry support
func (s *PaymentWebhookService) handleWebhookInTransaction(
	ctx context.Context,
	tx db.Tx,
	notification *midtrans.NotificationPayload,
	clientIP string,
) error {
	// STEP 1: INSERT webhook event FIRST with status=pending
	// This captures EVERY incoming notification BEFORE any validation.
	//
	// IDENTITY (REC-3): event_id holds the Midtrans gateway TRANSACTION reference
	// and is NOT unique — one transaction emits several notifications as its
	// status advances. notification_key is the canonical identity of ONE
	// notification (derived from its signal-defining fields) and is the
	// idempotency authority: an exact redelivery dedups, while a different status
	// transition or refund of the same transaction is stored and processed as its
	// own event.
	midtransTransactionID := notification.TransactionID
	notificationKey := midtrans.NotificationIdentity(notification)
	payload, _ := json.Marshal(notification)

	inserted, err := s.insertWebhookEvent(ctx, tx, midtransTransactionID, notificationKey, notification.OrderID, notification.SignatureKey, payload)
	if err != nil {
		return fmt.Errorf("failed to insert webhook event: %w", err)
	}
	if !inserted {
		// This exact notification is already stored. Its recorded status decides
		// what happens next.
		existingStatus, statusErr := s.paymentRepo.GetWebhookEventStatus(ctx, tx, notificationKey)
		if statusErr != nil {
			return fmt.Errorf("failed to read existing webhook event status: %w", statusErr)
		}

		if !webhookEventIsReprocessable(existingStatus) {
			// Idempotent duplicate: the event was already processed (or otherwise
			// recorded in a terminal state). The insert was a clean ON CONFLICT
			// no-op, so the transaction is healthy and the outer commit succeeds.
			s.log.Info("Webhook already processed (idempotent)",
				zap.String("notification_key", notificationKey),
				zap.String("transaction_id", notification.TransactionID),
				zap.String("existing_status", existingStatus),
			)
			return nil
		}

		// REC-1: a previously FAILED delivery never completed processing — its
		// transaction rolled back — and the only reason a row exists now is the
		// independent failure record. Re-processing is therefore the behaviour
		// that existed before failures were recorded durably. Treating the
		// recorded failure as a permanent tombstone would silently change
		// idempotency semantics and strand the event forever.
		s.log.Info("Reprocessing webhook event previously recorded as failed",
			zap.String("notification_key", notificationKey),
			zap.String("transaction_id", notification.TransactionID),
		)
	}

	// STEP 2: SIGNATURE VERIFICATION (AFTER insert)
	if !s.midtransClient.VerifySignature(notification) {
		s.log.Warn("Invalid webhook signature",
			zap.String("notification_key", notificationKey),
			zap.String("order_id", notification.OrderID),
			zap.String("client_ip", clientIP),
		)
		// No status write here: this transaction always rolls back on the error
		// returned below, so the write would be erased, and a signature rejection
		// of UNVERIFIED input must not be persisted at all (REC-1).
		return fmt.Errorf("%w", ErrWebhookSignatureInvalid)
	}

	// STEP 3: Transition to processing (state machine enforcement)
	if err := s.updateWebhookEventStatus(ctx, tx, notificationKey, "processing", nil, nil); err != nil {
		return fmt.Errorf("failed to update webhook event to processing: %w", err)
	}

	// STEP 3.5: REFUND WEBHOOK BRANCH (TASK 33 / Phase 1)
	//
	// Refund acks have a fundamentally different shape from payment acks:
	// gross_amount is the original payment, refund_amount carries the actual
	// refunded value, and the refund row (not the payment row) owns the
	// state transition. Dispatch them to RefundService and stop here so
	// the payment-side amount validation below never runs against a
	// refund payload.
	if midtrans.IsRefundNotification(notification.TransactionStatus) {
		if s.refundService == nil {
			s.log.Warn("refund_webhook_unwired",
				zap.String("notification_key", notificationKey),
				zap.String("order_id", notification.OrderID),
				zap.String("transaction_status", notification.TransactionStatus),
			)
			_ = s.updateWebhookEventStatus(ctx, tx, notificationKey, "succeeded", nil, nil)
			return nil
		}
		if err := s.refundService.HandleGatewayRefundAck(ctx, tx, notification); err != nil {
			s.log.Error("refund_webhook_dispatch_failed",
				zap.String("notification_key", notificationKey),
				zap.String("order_id", notification.OrderID),
				zap.Error(err),
			)
			errMsg := fmt.Sprintf("refund ack dispatch failed: %v", err)
			_ = s.updateWebhookEventStatus(ctx, tx, notificationKey, "failed", nil, strPtr(errMsg))
			return fmt.Errorf("refund ack dispatch failed: %w", err)
		}
		_ = s.updateWebhookEventStatus(ctx, tx, notificationKey, "succeeded", nil, nil)
		return nil
	}

	// STEP 4: FIND PAYMENT with FOR UPDATE lock (prevents race condition)
	payment, err := s.paymentRepo.GetForUpdate(ctx, tx, notification.OrderID)
	if err != nil {
		if err.Error() == "no rows in result set" {
			s.log.Warn("Webhook orphaned: payment not found",
				zap.String("notification_key", notificationKey),
				zap.String("order_id", notification.OrderID),
			)
			_ = s.updateWebhookEventStatus(ctx, tx, notificationKey, "orphaned", nil, strPtr("payment not found"))
			return nil // Return success to stop Midtrans retry
		}
		_ = s.updateWebhookEventStatus(ctx, tx, notificationKey, "failed", nil, strPtr(fmt.Sprintf("failed to find payment: %v", err)))
		return fmt.Errorf("failed to find payment: %w", err)
	}

	// STEP 5: Link event to payment
	if err := s.linkWebhookEventToPayment(ctx, tx, notificationKey, payment.ID); err != nil {
		s.log.Error("Failed to link webhook to payment", zap.Error(err))
	}

	// STEP 6: AMOUNT VALIDATION
	// MONEY UNIT (PASS_18H): payment.GrossAmount is a Rupiah integer —
	// Labuda's canonical money unit. Compared directly against the gateway's
	// webhook amount, which is also whole Rupiah. No scaling in either direction.
	expectedAmount := payment.GrossAmount.Int64()
	webhookAmount := midtrans.ParseGrossAmount(notification.GrossAmount)
	if webhookAmount != expectedAmount {
		s.log.Warn("Webhook amount mismatch",
			zap.String("payment_id", payment.ID.String()),
			zap.Int64("expected", expectedAmount),
			zap.Int64("received", webhookAmount),
		)
		errMsg := fmt.Sprintf("amount mismatch: expected %d, got %d", expectedAmount, webhookAmount)
		_ = s.updateWebhookEventStatus(ctx, tx, notificationKey, "failed", nil, strPtr(errMsg))
		return fmt.Errorf("amount validation failed")
	}

	// STEP 7: FIRST payment status check (early exit for already processed)
	if !payment.IsPending() {
		// REC-6 SLICE 1: a late gateway SUCCESS for a payment whose order can no
		// longer be finalized must still produce the canonical refund intent.
		// This uses the SAME order-validity authority as the discovery and
		// orphan-recovery paths (orderentity.IsInvalidForPaymentFinalization).
		if notification.ProviderState() == midtrans.ProviderStateSettled &&
			payment.ReferenceType == "order" && payment.ReferenceID != nil && *payment.ReferenceID != uuid.Nil {
			invalid, orderErr := orderInvalidForPaymentFinalization(ctx, tx, *payment.ReferenceID)
			if orderErr != nil {
				s.log.Error("rec6_order_load_failed",
					zap.String("payment_id", payment.ID.String()),
					zap.String("order_id", payment.ReferenceID.String()),
					zap.Error(orderErr),
				)
				errMsg := fmt.Sprintf("REC-6: failed to load order: %v", orderErr)
				_ = s.updateWebhookEventStatus(ctx, tx, notificationKey, "failed", &payment.ID, strPtr(errMsg))
				return fmt.Errorf("REC-6: failed to load order: %w", orderErr)
			}
			if invalid {
				return s.createRec6RefundIntent(ctx, tx, notificationKey, payment, notification)
			}
		}

		s.log.Info("Payment already processed, skipping",
			zap.String("payment_id", payment.ID.String()),
			zap.String("status", payment.Status),
		)
		// Still mark succeeded - we did our job
		_ = s.updateWebhookEventStatus(ctx, tx, notificationKey, "succeeded", &payment.ID, nil)
		return nil
	}

	s.log.Info("Processing webhook notification",
		zap.String("payment_id", payment.ID.String()),
		zap.String("notification_key", notificationKey),
		zap.String("transaction_status", notification.TransactionStatus),
	)

	// STEP 8: BUSINESS LOGIC
	if notification.ProviderState() == midtrans.ProviderStateSettled {
		// PAYMENT SUCCESS: canonical provider state SETTLED (settlement, or a
		// capture the fraud gate accepted)

		// STEP 8a: BULLETPROOF GUARD - Double-check payment status BEFORE settlement call
		// This ensures ledger NEVER executes twice even if another transaction
		// slipped through the initial check during a race condition
		currentStatus, err := s.paymentRepo.GetStatus(ctx, tx, payment.ID)
		if err == nil && currentStatus != repository.PaymentStatusPending {
			s.log.Warn("Payment status changed before settlement, aborting (bulletproof guard)",
				zap.String("payment_id", payment.ID.String()),
				zap.String("status", currentStatus),
			)
			_ = s.updateWebhookEventStatus(ctx, tx, notificationKey, "succeeded", &payment.ID, nil)
			return nil // Safe exit - another transaction processed it
		}

		// STEP 8b: Canonical order-payment finalization
		// This owns payment settlement, gateway-funded escrow creation,
		// and the order-paid transition in one reusable service.
		if payment.ReferenceType == "order" && payment.ReferenceID != nil && *payment.ReferenceID != uuid.Nil {
			// REC-6 SLICE 1: before finalizing, evaluate the SINGLE canonical
			// order-validity authority (expired by time, or already transitioned
			// out of pending_payment). A gateway success for an invalid order
			// becomes a canonical refund intent instead of a finalization. This is
			// the same predicate used by discovery and orphan recovery.
			invalid, orderErr := orderInvalidForPaymentFinalization(ctx, tx, *payment.ReferenceID)
			if orderErr != nil {
				s.log.Error("rec6_order_load_failed",
					zap.String("payment_id", payment.ID.String()),
					zap.String("order_id", payment.ReferenceID.String()),
					zap.Error(orderErr),
				)
				errMsg := fmt.Sprintf("REC-6: failed to load order: %v", orderErr)
				_ = s.updateWebhookEventStatus(ctx, tx, notificationKey, "failed", &payment.ID, strPtr(errMsg))
				return fmt.Errorf("REC-6: failed to load order: %w", orderErr)
			}
			if invalid {
				s.log.Warn("rec6_order_terminal_gateway_success",
					zap.String("payment_id", payment.ID.String()),
					zap.String("order_id", payment.ReferenceID.String()),
				)
				return s.createRec6RefundIntent(ctx, tx, notificationKey, payment, notification)
			}

			if s.canonicalFinalizationService == nil {
				s.log.Error("CRITICAL: CanonicalFinalizationService not wired",
					zap.String("payment_id", payment.ID.String()),
					zap.String("order_id", (*payment.ReferenceID).String()),
				)
				errMsg := "CRITICAL: canonical finalization service not wired"
				_ = s.updateWebhookEventStatus(ctx, tx, notificationKey, "failed", &payment.ID, strPtr(errMsg))
				return fmt.Errorf("CRITICAL: canonical finalization service not wired")
			}

			if err := s.canonicalFinalizationService.FinalizeOrderPayment(
				ctx,
				tx,
				payment,
				notification.TransactionID,
				notification.PaymentType,
			); err != nil {
				s.log.Error("CRITICAL: Failed to finalize order payment",
					zap.String("payment_id", payment.ID.String()),
					zap.String("order_id", (*payment.ReferenceID).String()),
					zap.Error(err),
				)
				errMsg := fmt.Sprintf("CRITICAL: failed to finalize order payment: %v", err)
				_ = s.updateWebhookEventStatus(ctx, tx, notificationKey, "failed", &payment.ID, strPtr(errMsg))
				return fmt.Errorf("CRITICAL: failed to finalize order payment: %w", err)
			}

			// BNR Phase 1: Mark payment attempt as success
			s.updatePaymentAttemptSuccess(ctx, tx, *payment.ReferenceID)
		}

		// STEP 8d: BILLING PAYMENT COMPLETION - Mark billing transaction as paid
		// For billing reference types (promotion_package)
		// This handles non-order payments without touching the order domain
		if payment.ReferenceType == "billing" && payment.ReferenceID != nil && *payment.ReferenceID != uuid.Nil {
			billingID := *payment.ReferenceID

			// Get billing details to check type and target ID
			billing, err := s.billingRepo.GetByID(ctx, tx, billingID)
			if err != nil {
				s.log.Error("CRITICAL: Failed to get billing for payment completion",
					zap.String("payment_id", payment.ID.String()),
					zap.String("billing_id", billingID.String()),
					zap.Error(err),
				)
				errMsg := fmt.Sprintf("CRITICAL: failed to get billing: %v", err)
				_ = s.updateWebhookEventStatus(ctx, tx, notificationKey, "failed", &payment.ID, strPtr(errMsg))
				return fmt.Errorf("CRITICAL: failed to get billing: %w", err)
			}

			// PASS_18V: Pass payment-method fee info for fee carving.
			var paymentMethodCode *string
			serviceFeeAmount := int64(0)
			if payment.PaymentMethodCode != nil {
				paymentMethodCode = payment.PaymentMethodCode
				serviceFeeAmount = payment.ServiceFeeAmount.Int64()
			}
			newlyPaid, err := s.billingService.MarkPaidWithPayment(ctx, tx, billingID, &payment.ID, paymentMethodCode, serviceFeeAmount)
			if err != nil {
				s.log.Error("CRITICAL: Failed to mark billing as paid",
					zap.String("payment_id", payment.ID.String()),
					zap.String("billing_id", billingID.String()),
					zap.Error(err),
				)
				errMsg := fmt.Sprintf("CRITICAL: failed to mark billing as paid: %v", err)
				_ = s.updateWebhookEventStatus(ctx, tx, notificationKey, "failed", &payment.ID, strPtr(errMsg))
				return fmt.Errorf("CRITICAL: failed to mark billing as paid: %w", err)
			}

			if !newlyPaid {
				// Billing was already paid by a previous webhook — skip all post-payment
				// side-effects to prevent duplicate ownership creation.
				s.log.Info("Billing already paid; skipping post-payment side-effects (idempotent)",
					zap.String("payment_id", payment.ID.String()),
					zap.String("billing_id", billingID.String()),
				)
				_ = s.updateWebhookEventStatus(ctx, tx, notificationKey, "succeeded", &payment.ID, nil)
				return nil
			}

			s.log.Info("Billing transaction marked as paid",
				zap.String("payment_id", payment.ID.String()),
				zap.String("billing_id", billingID.String()),
				zap.String("billing_type", string(billing.Type)),
			)

			// PROMOTION PACKAGE PURCHASE PURGED — hard convergence (§28).
			if billing.Type == billingentity.TypePromotionPackage {
				s.log.Error("promotion_package billing forbidden — use promotion_contracts",
					zap.String("payment_id", payment.ID.String()),
					zap.String("billing_id", billingID.String()),
				)
				errMsg := "promotion package purchase purged — use promotion_contracts"
				_ = s.updateWebhookEventStatus(ctx, tx, notificationKey, "failed", &payment.ID, strPtr(errMsg))
				return fmt.Errorf("promotion package purchase forbidden")
			}
		}

		// STEP 8f: SUBSCRIPTION PAYMENT COMPLETION - Activate subscription
		// For subscription reference types, call the subscription payment service
		// This creates the subscription record and enables seller capability
		if payment.ReferenceType == repository.ReferenceTypeSubscription {
			// For subscription payments, use payment.UserID directly
			// ReferenceID may be nil or contain a different identifier
			userID := payment.UserID

			if s.subscriptionPaymentService == nil {
				s.log.Error("CRITICAL: Subscription payment service not configured",
					zap.String("payment_id", payment.ID.String()),
					zap.String("user_id", userID.String()),
				)
				errMsg := "CRITICAL: subscription payment service not configured"
				_ = s.updateWebhookEventStatus(ctx, tx, notificationKey, "failed", &payment.ID, strPtr(errMsg))
				return fmt.Errorf("CRITICAL: subscription payment service not configured")
			}

			// Subscription payments still need the payment row marked paid.
			// Preserve gateway semantics: capture stays capture, everything else
			// uses settlement so the row leaves pending before activation.
			if strings.EqualFold(notification.TransactionStatus, string(midtrans.StatusCapture)) {
				if err := s.paymentRepo.MarkAsCapture(ctx, tx, payment.ID, notification.TransactionID, notification.PaymentType); err != nil {
					s.log.Error("CRITICAL: Failed to mark subscription payment as capture",
						zap.String("payment_id", payment.ID.String()),
						zap.Error(err),
					)
					errMsg := fmt.Sprintf("CRITICAL: failed to mark subscription payment as capture: %v", err)
					_ = s.updateWebhookEventStatus(ctx, tx, notificationKey, "failed", &payment.ID, strPtr(errMsg))
					return fmt.Errorf("CRITICAL: failed to mark subscription payment as capture: %w", err)
				}
			} else {
				if err := s.settlementService.SettlePaymentByID(ctx, tx, payment.ID, notification.TransactionID, notification.PaymentType); err != nil {
					s.log.Error("CRITICAL: Failed to mark subscription payment as settlement",
						zap.String("payment_id", payment.ID.String()),
						zap.Error(err),
					)
					errMsg := fmt.Sprintf("CRITICAL: failed to mark subscription payment as settlement: %v", err)
					_ = s.updateWebhookEventStatus(ctx, tx, notificationKey, "failed", &payment.ID, strPtr(errMsg))
					return fmt.Errorf("CRITICAL: failed to mark subscription payment as settlement: %w", err)
				}
			}

			if err := s.subscriptionPaymentService.ProcessSuccessfulPaymentTx(
				ctx,
				tx,
				payment.ID,
				userID,
				notification.TransactionID,
			); err != nil {
				s.log.Error("CRITICAL: Failed to process subscription payment",
					zap.String("payment_id", payment.ID.String()),
					zap.String("user_id", userID.String()),
					zap.Error(err),
				)
				errMsg := fmt.Sprintf("CRITICAL: failed to process subscription payment: %v", err)
				_ = s.updateWebhookEventStatus(ctx, tx, notificationKey, "failed", &payment.ID, strPtr(errMsg))
				return fmt.Errorf("CRITICAL: failed to process subscription payment: %w", err)
			}

			s.log.Info("Subscription payment processed successfully",
				zap.String("payment_id", payment.ID.String()),
				zap.String("user_id", userID.String()),
			)
		}

	} else if notification.ProviderState() == midtrans.ProviderStateFailed {
		// PAYMENT FAILED: deny, cancel, expire
		if err := s.settlementService.FailPayment(ctx, tx, notification.OrderID, notification.TransactionStatus); err != nil {
			s.log.Error("Failed to mark payment as failed",
				zap.String("payment_id", payment.ID.String()),
				zap.Error(err),
			)
			// REC-1: this error must NOT be swallowed. Swallowing it let the flow
			// continue to STEP 9 and commit the event as 'succeeded' — a durable
			// record claiming the event was handled while the payment status update
			// had actually failed. Returning the error rolls the transaction back
			// and records the failure durably instead.
			errMsg := fmt.Sprintf("failed to mark payment as failed: %v", err)
			_ = s.updateWebhookEventStatus(ctx, tx, notificationKey, "failed", &payment.ID, strPtr(errMsg))
			return fmt.Errorf("failed to mark payment as failed: %w", err)
		}
		s.log.Info("Payment marked as failed",
			zap.String("payment_id", payment.ID.String()),
			zap.String("status", notification.TransactionStatus),
		)

		// BNR Phase 1: Mark payment attempt as failed
		if payment.ReferenceType == "order" && payment.ReferenceID != nil {
			s.updatePaymentAttemptFailed(ctx, tx, *payment.ReferenceID, notification.TransactionStatus)
		}
	} else if notification.ProviderState() == midtrans.ProviderStatePending {
		// Still pending - no action needed
		_ = s.updateWebhookEventStatus(ctx, tx, notificationKey, "succeeded", &payment.ID, nil)
		return nil
	} else {
		// Unknown gateway status: do not silently mark success.
		// Route to manual review so ops can inspect the provider payload.
		errMsg := fmt.Sprintf("unknown transaction status: %s", notification.TransactionStatus)
		s.log.Warn("Unknown webhook transaction status",
			zap.String("payment_id", payment.ID.String()),
			zap.String("notification_key", notificationKey),
			zap.String("transaction_status", notification.TransactionStatus),
		)
		_ = s.updateWebhookEventStatus(ctx, tx, notificationKey, repository.PaymentWebhookEventStatusManualReview, &payment.ID, &errMsg)
		return nil
	}

	// STEP 9: MARK EVENT AS SUCCEEDED (final state)
	if err := s.updateWebhookEventStatus(ctx, tx, notificationKey, "succeeded", &payment.ID, nil); err != nil {
		s.log.Error("Failed to mark webhook event as succeeded", zap.Error(err))
		// Non-critical - payment already updated
	}

	return nil
}

// orderInvalidForPaymentFinalization loads the two canonical order columns and
// evaluates the shared domain authority
// (orderentity.IsInvalidForPaymentFinalization). It is the SINGLE order-validity
// decision used by both webhook ingestion and orphan recovery, so neither path
// can drift from the canonical payment-finalization guards.
func orderInvalidForPaymentFinalization(ctx context.Context, tx db.Tx, orderID uuid.UUID) (bool, error) {
	var status string
	var paymentExpiresAt time.Time
	if err := tx.QueryRow(ctx, `SELECT status, payment_expires_at FROM orders WHERE id = $1`, orderID).Scan(&status, &paymentExpiresAt); err != nil {
		return false, err
	}
	return orderentity.IsInvalidForPaymentFinalization(orderentity.Status(status), paymentExpiresAt), nil
}

// createRec6RefundIntent is the REC-6 webhook-side entry point for creating
// a canonical refund intent when a gateway success notification arrives for
// a payment whose order is in a terminal state (expired, cancelled, etc.).
//
// It loads the order to obtain the seller_id, delegates to RefundService
// (the canonical refund-intent authority), and marks the webhook event as
// succeeded — the refund row is the durable evidence, not the webhook event.
func (s *PaymentWebhookService) createRec6RefundIntent(
	ctx context.Context,
	tx db.Tx,
	notificationKey string,
	payment *repository.Payment,
	notification *midtrans.NotificationPayload,
) error {
	if s.refundService == nil {
		s.log.Error("rec6_refund_service_not_wired",
			zap.String("payment_id", payment.ID.String()),
		)
		errMsg := "CRITICAL: REC-6 refund service not wired"
		_ = s.updateWebhookEventStatus(ctx, tx, notificationKey, "failed", &payment.ID, strPtr(errMsg))
		return fmt.Errorf("CRITICAL: REC-6 refund service not wired")
	}

	if payment.ReferenceID == nil || *payment.ReferenceID == uuid.Nil {
		s.log.Error("rec6_no_order_reference",
			zap.String("payment_id", payment.ID.String()),
		)
		errMsg := "REC-6: payment has no valid order reference"
		_ = s.updateWebhookEventStatus(ctx, tx, notificationKey, "failed", &payment.ID, strPtr(errMsg))
		return fmt.Errorf("REC-6: payment has no valid order reference")
	}

	// Load order to get seller_id for the refund row.
	var sellerID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT seller_id FROM orders WHERE id = $1`, *payment.ReferenceID).Scan(&sellerID); err != nil {
		s.log.Error("rec6_order_load_failed",
			zap.String("payment_id", payment.ID.String()),
			zap.String("order_id", payment.ReferenceID.String()),
			zap.Error(err),
		)
		errMsg := fmt.Sprintf("REC-6: failed to load order: %v", err)
		_ = s.updateWebhookEventStatus(ctx, tx, notificationKey, "failed", &payment.ID, strPtr(errMsg))
		return fmt.Errorf("REC-6: failed to load order: %w", err)
	}

	refund, err := s.refundService.CreateRefundIntentForInvalidOrder(ctx, tx, refundapp.Rec6RefundIntentInput{
		PaymentID:              payment.ID,
		OrderID:                *payment.ReferenceID,
		BuyerID:                payment.UserID,
		SellerID:               sellerID,
		GrossAmount:            payment.GrossAmount.Int64(),
		PaymentMidtransOrderID: notification.OrderID,
	})
	if err != nil {
		s.log.Error("rec6_refund_intent_create_failed",
			zap.String("payment_id", payment.ID.String()),
			zap.String("order_id", payment.ReferenceID.String()),
			zap.Error(err),
		)
		errMsg := fmt.Sprintf("REC-6: refund intent create failed: %v", err)
		_ = s.updateWebhookEventStatus(ctx, tx, notificationKey, "failed", &payment.ID, strPtr(errMsg))
		return fmt.Errorf("REC-6: refund intent create failed: %w", err)
	}

	s.log.Info("rec6_refund_intent_created",
		zap.String("payment_id", payment.ID.String()),
		zap.String("order_id", payment.ReferenceID.String()),
		zap.String("refund_id", refund.ID.String()),
		zap.Int64("gross_amount", payment.GrossAmount.Int64()),
	)

	// Webhook event marked succeeded — the refund row is the durable evidence.
	_ = s.updateWebhookEventStatus(ctx, tx, notificationKey, "succeeded", &payment.ID, nil)

	return nil
}

// insertWebhookEvent inserts a new notification row with status=pending.
//
// Returns (true, nil) when a new row was inserted, (false, nil) when THIS
// notification is already stored (idempotent duplicate). It uses ON CONFLICT DO
// NOTHING so a duplicate delivery is a clean no-op rather than a SQL error —
// this keeps the surrounding transaction healthy (a unique-violation error
// would abort the tx, making the subsequent Commit fail with
// ErrTxCommitRollback even when the duplicate is treated as idempotent).
//
// IDENTITY (REC-3): the conflict target is notification_key — the identity of
// one notification. midtransTransactionID is stored as event_id (the gateway
// transaction reference) and is intentionally NOT unique: one transaction
// produces several notifications, each of which must be stored and processed.
func (s *PaymentWebhookService) insertWebhookEvent(
	ctx context.Context,
	tx db.Tx,
	midtransTransactionID string,
	notificationKey string,
	midtransOrderID string,
	signatureKey string,
	payload []byte,
) (bool, error) {
	query := `
		INSERT INTO payment_webhook_events
			(id, provider, event_id, notification_key, midtrans_order_id, signature_key, payload, status, received_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		ON CONFLICT (notification_key) DO NOTHING
	`

	tag, err := tx.Exec(ctx, query,
		uuid.New(),
		"midtrans",
		midtransTransactionID,
		notificationKey,
		midtransOrderID,
		signatureKey,
		payload,
		"pending", // Always start as pending
	)
	if err != nil {
		return false, err
	}

	// ON CONFLICT DO NOTHING reports 0 rows affected when this notification is
	// already stored — that is the idempotent duplicate case.
	return tag.RowsAffected() == 1, nil
}

// updateWebhookEventStatus updates the status of ONE stored notification.
//
// Keyed on notification_key (REC-3): keying on the gateway transaction
// reference would write one notification's state onto every row stored for that
// transaction.
func (s *PaymentWebhookService) updateWebhookEventStatus(
	ctx context.Context,
	tx db.Tx,
	notificationKey string,
	status string,
	paymentID *uuid.UUID,
	errorMsg *string,
) error {
	// FIX: $1 was used both as enum (status = $1) and text ($1 IN (...)),
	// causing SQLSTATE 42P08 "inconsistent types deduced for parameter $1".
	// Add explicit casts so the parser sees a single, consistent input type (text)
	// at both call sites; the SET cast resolves it to enum once.
	query := `
		UPDATE payment_webhook_events
		SET status = $1::payment_webhook_status_enum,
		    payment_id = $2,
		    error_message = $3,
		    processed_at = CASE WHEN $1::text IN ('succeeded', 'failed', 'orphaned', 'manual_review', 'quarantined', 'terminal_review') THEN NOW() ELSE NULL END
		WHERE notification_key = $4
	`

	_, err := tx.Exec(ctx, query, status, paymentID, errorMsg, notificationKey)
	return err
}

// linkWebhookEventToPayment links one stored notification to its payment
// (keyed on notification_key — see updateWebhookEventStatus).
func (s *PaymentWebhookService) linkWebhookEventToPayment(
	ctx context.Context,
	tx db.Tx,
	notificationKey string,
	paymentID uuid.UUID,
) error {
	query := `
		UPDATE payment_webhook_events
		SET payment_id = $1
		WHERE notification_key = $2
	`

	_, err := tx.Exec(ctx, query, paymentID, notificationKey)
	return err
}

// strPtr returns a pointer to the given string
func strPtr(s string) *string {
	return &s
}

// =============================================================================
// BNR PHASE 1: PAYMENT ATTEMPT TRACKING
// =============================================================================

// updatePaymentAttemptSuccess marks a payment attempt as successful based on webhook
func (s *PaymentWebhookService) updatePaymentAttemptSuccess(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) {
	if s.paymentAttemptRepo == nil {
		return
	}

	// Find the latest payment attempt for this order
	attempt, err := s.paymentAttemptRepo.GetLatestPaymentAttemptByOrderID(ctx, tx, orderID)
	if err != nil {
		s.log.Warn("Failed to get payment attempt for success update",
			zap.String("order_id", orderID.String()),
			zap.Error(err),
		)
		return
	}

	// Only update if still pending
	if attempt.IsPending() {
		if err := s.paymentAttemptRepo.MarkSuccess(ctx, tx, attempt.ID, nil, nil); err != nil {
			s.log.Warn("Failed to mark payment attempt as success",
				zap.String("payment_attempt_id", attempt.ID.String()),
				zap.Error(err),
			)
		} else {
			s.log.Info("payment_attempt success",
				zap.String("payment_attempt_id", attempt.ID.String()),
				zap.String("order_id", orderID.String()),
			)
		}
	}
}

// updatePaymentAttemptFailed marks a payment attempt as failed based on webhook
func (s *PaymentWebhookService) updatePaymentAttemptFailed(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	transactionStatus string,
) {
	if s.paymentAttemptRepo == nil {
		return
	}

	// Map Midtrans status to normalized failure reason
	var failureReason string
	switch transactionStatus {
	case "deny":
		failureReason = repository.FailureReasonGatewayDenied
	case "cancel":
		failureReason = repository.FailureReasonUserCancelled
	case "expire":
		failureReason = repository.FailureReasonTimeout
	default:
		failureReason = repository.FailureReasonUnknown
	}

	// Find the latest payment attempt for this order
	attempt, err := s.paymentAttemptRepo.GetLatestPaymentAttemptByOrderID(ctx, tx, orderID)
	if err != nil {
		s.log.Warn("Failed to get payment attempt for failure update",
			zap.String("order_id", orderID.String()),
			zap.Error(err),
		)
		return
	}

	// Only update if still pending
	if attempt.IsPending() {
		if err := s.paymentAttemptRepo.MarkFailed(ctx, tx, attempt.ID, failureReason, nil); err != nil {
			s.log.Warn("Failed to mark payment attempt as failed",
				zap.String("payment_attempt_id", attempt.ID.String()),
				zap.Error(err),
			)
		} else {
			s.log.Info("payment_attempt failed",
				zap.String("payment_attempt_id", attempt.ID.String()),
				zap.String("order_id", orderID.String()),
				zap.String("failure_reason", failureReason),
			)
		}
	}
}
