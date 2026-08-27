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
	"fmt"
	"strings"

	"github.com/google/uuid"
	orderapp "github.com/labuda/backend/internal/commerce/order/application"
	orderRepoImpl "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	subscriptionapp "github.com/labuda/backend/internal/commerce/subscription/application"
	walletApp "github.com/labuda/backend/internal/core/wallet/application"
	financeApp "github.com/labuda/backend/internal/finance/application"
	billingapp "github.com/labuda/backend/internal/finance/billing/application"
	billingentity "github.com/labuda/backend/internal/finance/billing/entity"
	billingrepo "github.com/labuda/backend/internal/finance/billing/infrastructure/repository"
	refundapp "github.com/labuda/backend/internal/finance/refund/application"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	alertentity "github.com/labuda/backend/internal/platform/alert/entity"
	promotionapp "github.com/labuda/backend/internal/pricing/promotion/application"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/midtrans"
	"go.uber.org/zap"
)

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
// WIRING RULE: OrderService and WalletService MUST be injected from
// dependencies_core.go. Never construct them here. Building local copies
// produces nil-deps and causes settlement to crash inside MarkPaid.
type PaymentWebhookService struct {
	db                           *db.DB
	midtransClient               *midtrans.Client
	settlementService            *repository.PaymentSettlementService
	paymentRepo                  *repository.PaymentRepository
	paymentAttemptRepo           *repository.PaymentAttemptRepository // BNR Phase 1: Payment attempt tracking
	orderService                 *orderapp.OrderService
	walletService                *walletApp.WalletService
	orderRepo                    *orderRepoImpl.OrderRepository
	canonicalFinalizationService *CanonicalFinalizationService
	billingService               *billingapp.BillingService
	promotionService             *promotionapp.PromotionService
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
	// alertService (PASS_18T) raises an operator alert when a gateway
	// success notification arrives for a payment the platform already
	// expired. Optional: nil leaves the event durably recorded as
	// captured_after_expiry but skips the alert, logged loudly so the gap
	// is visible rather than silent.
	alertService RecoveryAlertService
	log          *zap.Logger
}

// NewPaymentWebhookService creates a new PaymentWebhookService.
//
// orderService and walletService MUST be the canonical instances built in
// dependencies_core.go. Do not pass freshly-constructed instances.
//
// subscriptionPaymentService should be set via SetSubscriptionPaymentService
// after initialization.
func NewPaymentWebhookService(
	db *db.DB,
	midtransClient *midtrans.Client,
	orderService *orderapp.OrderService,
	walletService *walletApp.WalletService,
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
		walletService:      walletService,
		orderRepo:          orderRepoImpl.NewOrderRepository(),
		billingService:     billingapp.NewBillingService(roleChecker, accountStatusChecker),
		billingRepo:        billingrepo.NewBillingRepository(),
		log:                log,
	}
}

// SetPromotionService wires the canonical PromotionService built with
// OperabilityCheckerImpl. MUST be called before any billing webhook
// for promotion_package can succeed; the branch is fail-closed when nil.
func (s *PaymentWebhookService) SetPromotionService(service *promotionapp.PromotionService) {
	s.promotionService = service
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
// expired (PASS_18T). Optional: leaving this unset still records the event
// durably as captured_after_expiry, but the gap is only visible via logs.
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
// 1. INSERT webhook event FIRST (status=pending) - captures ALL events for audit
// 2. Duplicate event_id = idempotent return (UNIQUE constraint)
// 3. Signature verification AFTER insert (prevents replay attack bypass)
// 4. State machine enforcement: pending -> processing -> succeeded/failed
// 5. db.WithTx handles DEADLOCK RETRY for PostgreSQL serialization errors
// 6. DOUBLE-CHECK payment status before ledger call (bulletproof guard)
//
// Returns error if processing fails (non-idempotent failures)
func (s *PaymentWebhookService) HandleWebhook(
	ctx context.Context,
	notification *midtrans.NotificationPayload,
	clientIP string,
) error {
	// Use db.WithTx for automatic retry on serialization/deadlock errors
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		return s.handleWebhookInTransaction(ctx, tx, notification, clientIP)
	})
}

// ReplayVerifiedWebhookFromGateway replays a verified Midtrans success payload
// through the canonical webhook flow. It is intended for dev-only recovery
// paths when the public notification URL is unreachable or stale.
func (s *PaymentWebhookService) ReplayVerifiedWebhookFromGateway(
	ctx context.Context,
	paymentID uuid.UUID,
	clientIP string,
) (*midtrans.NotificationPayload, error) {
	var payment *repository.Payment
	if err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		payment, err = s.paymentRepo.GetByID(ctx, tx, paymentID)
		return err
	}); err != nil {
		return nil, fmt.Errorf("load payment: %w", err)
	}
	if payment == nil {
		return nil, fmt.Errorf("payment not found: %s", paymentID)
	}
	if payment.MidtransOrderID == "" {
		return nil, fmt.Errorf("payment %s has no Midtrans order id", paymentID)
	}

	gatewayStatus, err := s.midtransClient.GetTransactionStatus(payment.MidtransOrderID)
	if err != nil {
		return nil, fmt.Errorf("query midtrans status: %w", err)
	}

	if !s.midtransClient.IsTransactionSuccess(gatewayStatus.TransactionStatus) {
		return nil, fmt.Errorf("gateway status is not successful: %s", gatewayStatus.TransactionStatus)
	}
	if strings.EqualFold(gatewayStatus.TransactionStatus, string(midtrans.StatusCapture)) &&
		strings.ToLower(strings.TrimSpace(gatewayStatus.FraudStatus)) != "accept" {
		return nil, fmt.Errorf("capture status is not safe to activate: fraud_status=%s", gatewayStatus.FraudStatus)
	}

	// MONEY UNIT (PASS_18H): payment.GrossAmount is a Rupiah integer — Labuda's
	// canonical money unit — compared directly against the gateway's amount,
	// which is also whole Rupiah (Midtrans's ".00" suffix is decimal
	// formatting, not a cents subunit). No scaling in either direction.
	expectedGross := payment.GrossAmount.Int64()
	actualGross := parseGrossAmount(gatewayStatus.GrossAmount)
	if actualGross != expectedGross {
		return nil, fmt.Errorf("gateway amount mismatch: payment=%d gateway=%d", expectedGross, actualGross)
	}
	if strings.TrimSpace(gatewayStatus.OrderID) != payment.MidtransOrderID {
		return nil, fmt.Errorf("gateway order mismatch: payment=%s gateway=%s", payment.MidtransOrderID, gatewayStatus.OrderID)
	}

	signed := &midtrans.NotificationPayload{
		TransactionTime:   gatewayStatus.TransactionTime,
		TransactionStatus: gatewayStatus.TransactionStatus,
		TransactionID:     gatewayStatus.TransactionID,
		StatusMessage:     gatewayStatus.StatusMessage,
		StatusCode:        gatewayStatus.StatusCode,
		PaymentType:       gatewayStatus.PaymentType,
		OrderID:           gatewayStatus.OrderID,
		MerchantID:        gatewayStatus.MerchantID,
		GrossAmount:       gatewayStatus.GrossAmount,
		FraudStatus:       gatewayStatus.FraudStatus,
		Currency:          gatewayStatus.Currency,
	}
	signed.SignatureKey = s.midtransClient.BuildWebhookSignature(signed)

	eventExists, err := s.webhookEventExists(ctx, signed.TransactionID)
	if err != nil {
		return nil, fmt.Errorf("check replay event existence: %w", err)
	}
	if eventExists {
		s.log.Info("dev webhook replay found existing event; repairing payment state",
			zap.String("payment_id", paymentID.String()),
			zap.String("transaction_id", signed.TransactionID),
		)
		if repairErr := s.repairVerifiedReplayPayment(ctx, paymentID, signed); repairErr != nil {
			return nil, repairErr
		}
		return signed, nil
	}

	if err := s.HandleWebhook(ctx, signed, clientIP); err != nil {
		return nil, fmt.Errorf("replay webhook: %w", err)
	}

	return signed, nil
}

func (s *PaymentWebhookService) webhookEventExists(ctx context.Context, eventID string) (bool, error) {
	var exists bool
	if err := s.db.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM payment_webhook_events WHERE event_id = $1
			)
		`, eventID).Scan(&exists)
	}); err != nil {
		return false, err
	}
	return exists, nil
}

// repairVerifiedReplayPayment finishes a replay when the webhook event row
// already exists but the payment row is still pending. This keeps the dev
// replay idempotent after the first canonical event insert.
func (s *PaymentWebhookService) repairVerifiedReplayPayment(
	ctx context.Context,
	paymentID uuid.UUID,
	notification *midtrans.NotificationPayload,
) error {
	if notification == nil {
		return fmt.Errorf("notification cannot be nil")
	}

	return s.db.WithTx(ctx, func(tx db.Tx) error {
		payment, err := s.paymentRepo.GetByIDForUpdate(ctx, tx, paymentID)
		if err != nil {
			return fmt.Errorf("lock payment for repair: %w", err)
		}

		if payment.IsPending() {
			if strings.EqualFold(notification.TransactionStatus, string(midtrans.StatusCapture)) {
				if err := s.paymentRepo.MarkAsCapture(ctx, tx, payment.ID, notification.TransactionID, notification.PaymentType); err != nil {
					return fmt.Errorf("repair capture payment status: %w", err)
				}
			} else {
				if err := s.settlementService.SettlePaymentByID(ctx, tx, payment.ID, notification.TransactionID, notification.PaymentType); err != nil {
					return fmt.Errorf("repair settlement payment status: %w", err)
				}
			}
		}

		if payment.ReferenceType == repository.ReferenceTypeSubscription && s.subscriptionPaymentService != nil {
			if err := s.subscriptionPaymentService.ProcessSuccessfulPaymentTx(
				ctx,
				tx,
				payment.ID,
				payment.UserID,
				notification.TransactionID,
			); err != nil {
				return fmt.Errorf("repair subscription activation: %w", err)
			}
		}

		return nil
	})
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
	// This captures ALL incoming webhooks BEFORE any validation
	// Idempotency: duplicate event_id will fail UNIQUE constraint
	eventID := notification.TransactionID
	payload, _ := json.Marshal(notification)

	inserted, err := s.insertWebhookEvent(ctx, tx, eventID, notification.OrderID, notification.SignatureKey, payload)
	if err != nil {
		return fmt.Errorf("failed to insert webhook event: %w", err)
	}
	if !inserted {
		// Idempotent duplicate: the event_id already exists (a prior webhook
		// for this transaction). The insert was a clean ON CONFLICT no-op, so
		// the transaction is healthy and the outer commit will succeed.
		s.log.Info("Webhook already processed (idempotent)",
			zap.String("event_id", eventID),
			zap.String("transaction_id", notification.TransactionID),
		)
		return nil
	}

	// STEP 2: SIGNATURE VERIFICATION (AFTER insert)
	if !s.midtransClient.VerifySignature(notification) {
		s.log.Warn("Invalid webhook signature",
			zap.String("event_id", eventID),
			zap.String("order_id", notification.OrderID),
			zap.String("client_ip", clientIP),
		)
		// Update event status to failed
		_ = s.updateWebhookEventStatus(ctx, tx, eventID, "failed", nil, strPtr("invalid signature"))
		return fmt.Errorf("invalid signature")
	}

	// STEP 3: Transition to processing (state machine enforcement)
	if err := s.updateWebhookEventStatus(ctx, tx, eventID, "processing", nil, nil); err != nil {
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
	if s.midtransClient.IsRefundNotification(notification.TransactionStatus) {
		if s.refundService == nil {
			s.log.Warn("refund_webhook_unwired",
				zap.String("event_id", eventID),
				zap.String("order_id", notification.OrderID),
				zap.String("transaction_status", notification.TransactionStatus),
			)
			_ = s.updateWebhookEventStatus(ctx, tx, eventID, "succeeded", nil, nil)
			return nil
		}
		if err := s.refundService.HandleGatewayRefundAck(ctx, tx, notification); err != nil {
			s.log.Error("refund_webhook_dispatch_failed",
				zap.String("event_id", eventID),
				zap.String("order_id", notification.OrderID),
				zap.Error(err),
			)
			errMsg := fmt.Sprintf("refund ack dispatch failed: %v", err)
			_ = s.updateWebhookEventStatus(ctx, tx, eventID, "failed", nil, strPtr(errMsg))
			return fmt.Errorf("refund ack dispatch failed: %w", err)
		}
		_ = s.updateWebhookEventStatus(ctx, tx, eventID, "succeeded", nil, nil)
		return nil
	}

	// STEP 4: FIND PAYMENT with FOR UPDATE lock (prevents race condition)
	payment, err := s.paymentRepo.GetForUpdate(ctx, tx, notification.OrderID)
	if err != nil {
		if err.Error() == "no rows in result set" {
			s.log.Warn("Webhook orphaned: payment not found",
				zap.String("event_id", eventID),
				zap.String("order_id", notification.OrderID),
			)
			_ = s.updateWebhookEventStatus(ctx, tx, eventID, "orphaned", nil, strPtr("payment not found"))
			return nil // Return success to stop Midtrans retry
		}
		_ = s.updateWebhookEventStatus(ctx, tx, eventID, "failed", nil, strPtr(fmt.Sprintf("failed to find payment: %v", err)))
		return fmt.Errorf("failed to find payment: %w", err)
	}

	// STEP 5: Link event to payment
	if err := s.linkWebhookEventToPayment(ctx, tx, eventID, payment.ID); err != nil {
		s.log.Error("Failed to link webhook to payment", zap.Error(err))
	}

	// STEP 6: AMOUNT VALIDATION
	// MONEY UNIT (PASS_18H): payment.GrossAmount is a Rupiah integer —
	// Labuda's canonical money unit. Compared directly against the gateway's
	// webhook amount, which is also whole Rupiah. No scaling in either direction.
	expectedAmount := payment.GrossAmount.Int64()
	webhookAmount := parseGrossAmount(notification.GrossAmount)
	if webhookAmount != expectedAmount {
		s.log.Warn("Webhook amount mismatch",
			zap.String("payment_id", payment.ID.String()),
			zap.Int64("expected", expectedAmount),
			zap.Int64("received", webhookAmount),
		)
		errMsg := fmt.Sprintf("amount mismatch: expected %d, got %d", expectedAmount, webhookAmount)
		_ = s.updateWebhookEventStatus(ctx, tx, eventID, "failed", nil, strPtr(errMsg))
		return fmt.Errorf("amount validation failed")
	}

	// STEP 7: FIRST payment status check (early exit for already processed)
	if !payment.IsPending() {
		// PASS_18T: a gateway success notification for a payment the platform
		// already expired (PaymentExpiryWorker) is NOT an ordinary idempotent
		// replay — money may be captured at the gateway with no platform-side
		// reconciliation. Do not label it "succeeded"; the canonical state
		// machine (SettlePaymentByID) already hard-blocks settling an expired
		// order, so recovery-to-paid is deliberately not attempted here.
		if payment.IsExpired() && s.midtransClient.IsTransactionSuccess(notification.TransactionStatus) {
			s.recordCapturedAfterExpiry(ctx, tx, eventID, payment, notification)
			return nil
		}

		s.log.Info("Payment already processed, skipping",
			zap.String("payment_id", payment.ID.String()),
			zap.String("status", payment.Status),
		)
		// Still mark succeeded - we did our job
		_ = s.updateWebhookEventStatus(ctx, tx, eventID, "succeeded", &payment.ID, nil)
		return nil
	}

	s.log.Info("Processing webhook notification",
		zap.String("payment_id", payment.ID.String()),
		zap.String("event_id", eventID),
		zap.String("transaction_status", notification.TransactionStatus),
	)

	// STEP 8: BUSINESS LOGIC
	if s.midtransClient.IsTransactionSuccess(notification.TransactionStatus) {
		// PAYMENT SUCCESS: settlement or capture

		// STEP 8a: BULLETPROOF GUARD - Double-check payment status BEFORE settlement call
		// This ensures ledger NEVER executes twice even if another transaction
		// slipped through the initial check during a race condition
		currentStatus, err := s.paymentRepo.GetStatus(ctx, tx, payment.ID)
		if err == nil && currentStatus != repository.PaymentStatusPending {
			s.log.Warn("Payment status changed before settlement, aborting (bulletproof guard)",
				zap.String("payment_id", payment.ID.String()),
				zap.String("status", currentStatus),
			)
			_ = s.updateWebhookEventStatus(ctx, tx, eventID, "succeeded", &payment.ID, nil)
			return nil // Safe exit - another transaction processed it
		}

		// STEP 8b: Canonical order-payment finalization
		// This owns payment settlement, gateway-funded escrow creation,
		// and the order-paid transition in one reusable service.
		if payment.ReferenceType == "order" && payment.ReferenceID != nil && *payment.ReferenceID != uuid.Nil {
			if s.canonicalFinalizationService == nil {
				s.log.Error("CRITICAL: CanonicalFinalizationService not wired",
					zap.String("payment_id", payment.ID.String()),
					zap.String("order_id", (*payment.ReferenceID).String()),
				)
				errMsg := "CRITICAL: canonical finalization service not wired"
				_ = s.updateWebhookEventStatus(ctx, tx, eventID, "failed", &payment.ID, strPtr(errMsg))
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
				_ = s.updateWebhookEventStatus(ctx, tx, eventID, "failed", &payment.ID, strPtr(errMsg))
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
				_ = s.updateWebhookEventStatus(ctx, tx, eventID, "failed", &payment.ID, strPtr(errMsg))
				return fmt.Errorf("CRITICAL: failed to get billing: %w", err)
			}

			newlyPaid, err := s.billingService.MarkPaid(ctx, tx, billingID)
			if err != nil {
				s.log.Error("CRITICAL: Failed to mark billing as paid",
					zap.String("payment_id", payment.ID.String()),
					zap.String("billing_id", billingID.String()),
					zap.Error(err),
				)
				errMsg := fmt.Sprintf("CRITICAL: failed to mark billing as paid: %v", err)
				_ = s.updateWebhookEventStatus(ctx, tx, eventID, "failed", &payment.ID, strPtr(errMsg))
				return fmt.Errorf("CRITICAL: failed to mark billing as paid: %w", err)
			}

			if !newlyPaid {
				// Billing was already paid by a previous webhook — skip all post-payment
				// side-effects to prevent duplicate ownership creation.
				s.log.Info("Billing already paid; skipping post-payment side-effects (idempotent)",
					zap.String("payment_id", payment.ID.String()),
					zap.String("billing_id", billingID.String()),
				)
				_ = s.updateWebhookEventStatus(ctx, tx, eventID, "succeeded", &payment.ID, nil)
				return nil
			}

			s.log.Info("Billing transaction marked as paid",
				zap.String("payment_id", payment.ID.String()),
				zap.String("billing_id", billingID.String()),
				zap.String("billing_type", string(billing.Type)),
			)

			// STEP 8e: PROMOTION PACKAGE - Create ownership after payment
			// For promotion_package billing type, create the promotion ownership.
			// This is the ONLY way ownership can be created (server-authoritative).
			// newlyPaid=true guarantees this runs exactly once per billing transaction.
			if billing.Type == billingentity.TypePromotionPackage {
				if s.promotionService == nil {
					s.log.Error("CRITICAL: PromotionService not wired; refusing to create unvalidated ownership",
						zap.String("payment_id", payment.ID.String()),
						zap.String("billing_id", billingID.String()),
					)
					errMsg := "CRITICAL: promotion service not wired"
					_ = s.updateWebhookEventStatus(ctx, tx, eventID, "failed", &payment.ID, strPtr(errMsg))
					return fmt.Errorf("CRITICAL: promotion service not wired")
				}

				// The TargetID in billing contains the package ID.
				// BillingID is threaded through so the ownership can record its source
				// and the DB unique constraint prevents any concurrent duplicate.
				_, err := s.promotionService.PurchasePackage(ctx, tx, promotionapp.PurchasePackageInput{
					UserID:    billing.PayerID,
					PackageID: billing.TargetID, // Package ID stored as target_id
					BillingID: billingID,        // source traceability + DB-level duplicate guard
				})
				if err != nil {
					s.log.Error("CRITICAL: Failed to create promotion ownership",
						zap.String("payment_id", payment.ID.String()),
						zap.String("billing_id", billingID.String()),
						zap.String("user_id", billing.PayerID.String()),
						zap.String("package_id", billing.TargetID.String()),
						zap.Error(err),
					)
					errMsg := fmt.Sprintf("CRITICAL: failed to create promotion ownership: %v", err)
					_ = s.updateWebhookEventStatus(ctx, tx, eventID, "failed", &payment.ID, strPtr(errMsg))
					return fmt.Errorf("CRITICAL: failed to create promotion ownership: %w", err)
				}

				s.log.Info("Promotion ownership created after payment",
					zap.String("payment_id", payment.ID.String()),
					zap.String("billing_id", billingID.String()),
					zap.String("user_id", billing.PayerID.String()),
					zap.String("package_id", billing.TargetID.String()),
				)
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
				_ = s.updateWebhookEventStatus(ctx, tx, eventID, "failed", &payment.ID, strPtr(errMsg))
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
					_ = s.updateWebhookEventStatus(ctx, tx, eventID, "failed", &payment.ID, strPtr(errMsg))
					return fmt.Errorf("CRITICAL: failed to mark subscription payment as capture: %w", err)
				}
			} else {
				if err := s.settlementService.SettlePaymentByID(ctx, tx, payment.ID, notification.TransactionID, notification.PaymentType); err != nil {
					s.log.Error("CRITICAL: Failed to mark subscription payment as settlement",
						zap.String("payment_id", payment.ID.String()),
						zap.Error(err),
					)
					errMsg := fmt.Sprintf("CRITICAL: failed to mark subscription payment as settlement: %v", err)
					_ = s.updateWebhookEventStatus(ctx, tx, eventID, "failed", &payment.ID, strPtr(errMsg))
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
				_ = s.updateWebhookEventStatus(ctx, tx, eventID, "failed", &payment.ID, strPtr(errMsg))
				return fmt.Errorf("CRITICAL: failed to process subscription payment: %w", err)
			}

			s.log.Info("Subscription payment processed successfully",
				zap.String("payment_id", payment.ID.String()),
				zap.String("user_id", userID.String()),
			)
		}

	} else if s.midtransClient.IsTransactionFailed(notification.TransactionStatus) {
		// PAYMENT FAILED: deny, cancel, expire
		if err := s.settlementService.FailPayment(ctx, tx, notification.OrderID, notification.TransactionStatus); err != nil {
			s.log.Error("Failed to mark payment as failed",
				zap.String("payment_id", payment.ID.String()),
				zap.Error(err),
			)
			// Don't fail the webhook - payment status update is not critical
		} else {
			s.log.Info("Payment marked as failed",
				zap.String("payment_id", payment.ID.String()),
				zap.String("status", notification.TransactionStatus),
			)
		}

		// BNR Phase 1: Mark payment attempt as failed
		if payment.ReferenceType == "order" && payment.ReferenceID != nil {
			s.updatePaymentAttemptFailed(ctx, tx, *payment.ReferenceID, notification.TransactionStatus)
		}
	} else if s.midtransClient.IsTransactionPending(notification.TransactionStatus) {
		// Still pending - no action needed
		_ = s.updateWebhookEventStatus(ctx, tx, eventID, "succeeded", &payment.ID, nil)
		return nil
	} else {
		// Unknown gateway status: do not silently mark success.
		// Route to manual review so ops can inspect the provider payload.
		errMsg := fmt.Sprintf("unknown transaction status: %s", notification.TransactionStatus)
		s.log.Warn("Unknown webhook transaction status",
			zap.String("payment_id", payment.ID.String()),
			zap.String("event_id", eventID),
			zap.String("transaction_status", notification.TransactionStatus),
		)
		_ = s.updateWebhookEventStatus(ctx, tx, eventID, repository.PaymentWebhookEventStatusManualReview, &payment.ID, &errMsg)
		return nil
	}

	// STEP 9: MARK EVENT AS SUCCEEDED (final state)
	if err := s.updateWebhookEventStatus(ctx, tx, eventID, "succeeded", &payment.ID, nil); err != nil {
		s.log.Error("Failed to mark webhook event as succeeded", zap.Error(err))
		// Non-critical - payment already updated
	}

	return nil
}

// recordCapturedAfterExpiry handles a gateway success notification that
// arrives after PaymentExpiryWorker has already closed out the payment (and,
// for order payments, expired the order). It does NOT attempt to recover the
// payment/order to a paid state: the canonical state machine forbids it
// (PaymentSettlementService.SettlePaymentByID hard-blocks settlement once
// order.IsExpired()), and this pass does not weaken that guard. Instead the
// event is durably recorded as captured_after_expiry — never "succeeded",
// which would falsely imply the platform reconciled the money — and an
// operator alert is raised so the unreconciled capture is visible and
// actionable (manual reconciliation or refund) rather than silently lost.
//
// Idempotent: the same webhook retried by Midtrans never reaches this method
// twice (the event_id UNIQUE constraint at STEP 1 already returns early).
// Repeated late notifications for the same payment reuse payment.ID as both
// the alert entity and the dedup group key, so AlertService's dedup window
// merges them instead of paging on every retry.
func (s *PaymentWebhookService) recordCapturedAfterExpiry(
	ctx context.Context,
	tx db.Tx,
	eventID string,
	payment *repository.Payment,
	notification *midtrans.NotificationPayload,
) {
	errMsg := fmt.Sprintf(
		"gateway reported %s for payment %s after the platform already expired it; order/payment NOT marked paid; requires manual reconciliation or refund",
		notification.TransactionStatus, payment.ID,
	)
	s.log.Error("payment_captured_after_expiry",
		zap.String("payment_id", payment.ID.String()),
		zap.String("event_id", eventID),
		zap.String("reference_type", payment.ReferenceType),
		zap.String("transaction_status", notification.TransactionStatus),
		zap.Int64("gross_amount", payment.GrossAmount.Int64()),
	)

	if err := s.updateWebhookEventStatus(ctx, tx, eventID, repository.PaymentWebhookEventStatusCapturedAfterExpiry, &payment.ID, &errMsg); err != nil {
		s.log.Error("Failed to mark webhook event as captured_after_expiry", zap.Error(err))
	}

	if s.alertService == nil {
		s.log.Warn("payment_captured_after_expiry_alert_not_wired",
			zap.String("payment_id", payment.ID.String()),
		)
		return
	}

	metadata := alertentity.AlertMetadata{
		"required_action":    "manual_reconciliation_or_refund",
		"issue_type":         "payment_captured_after_expiry",
		"event_id":           eventID,
		"payment_id":         payment.ID.String(),
		"reference_type":     payment.ReferenceType,
		"transaction_status": notification.TransactionStatus,
		"midtrans_order_id":  notification.OrderID,
		"gross_amount":       payment.GrossAmount.Int64(),
	}
	if payment.ReferenceID != nil {
		metadata["reference_id"] = payment.ReferenceID.String()
	}

	groupKey := fmt.Sprintf("payment_captured_after_expiry:%s", payment.ID.String())
	if _, err := s.alertService.CreateAlert(
		ctx,
		alertentity.AlertTypePaymentCapturedAfterExpiry,
		alertentity.SeverityCritical,
		"payment",
		payment.ID,
		errMsg,
		metadata,
		&groupKey,
	); err != nil {
		s.log.Warn("Failed to create payment_captured_after_expiry alert",
			zap.String("payment_id", payment.ID.String()),
			zap.Error(err),
		)
	}
}

// insertWebhookEvent inserts a new webhook event with status=pending.
//
// Returns (true, nil) when a new row was inserted, (false, nil) when the
// event_id already exists (idempotent duplicate). It uses ON CONFLICT DO
// NOTHING so a duplicate delivery is a clean no-op rather than a SQL error —
// this keeps the surrounding transaction healthy (a unique-violation error
// would abort the tx, making the subsequent Commit fail with
// ErrTxCommitRollback even when the duplicate is treated as idempotent).
func (s *PaymentWebhookService) insertWebhookEvent(
	ctx context.Context,
	tx db.Tx,
	eventID string,
	midtransOrderID string,
	signatureKey string,
	payload []byte,
) (bool, error) {
	query := `
		INSERT INTO payment_webhook_events
			(id, provider, event_id, midtrans_order_id, signature_key, payload, status, received_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		ON CONFLICT (event_id) DO NOTHING
	`

	tag, err := tx.Exec(ctx, query,
		uuid.New(),
		"midtrans",
		eventID,
		midtransOrderID,
		signatureKey,
		payload,
		"pending", // Always start as pending
	)
	if err != nil {
		return false, err
	}

	// ON CONFLICT DO NOTHING reports 0 rows affected when the event_id already
	// exists — that is the idempotent duplicate case.
	return tag.RowsAffected() == 1, nil
}

// updateWebhookEventStatus updates the status of a webhook event
func (s *PaymentWebhookService) updateWebhookEventStatus(
	ctx context.Context,
	tx db.Tx,
	eventID string,
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
		WHERE event_id = $4
	`

	_, err := tx.Exec(ctx, query, status, paymentID, errorMsg, eventID)
	return err
}

// linkWebhookEventToPayment links a webhook event to its payment
func (s *PaymentWebhookService) linkWebhookEventToPayment(
	ctx context.Context,
	tx db.Tx,
	eventID string,
	paymentID uuid.UUID,
) error {
	query := `
		UPDATE payment_webhook_events
		SET payment_id = $1
		WHERE event_id = $2
	`

	_, err := tx.Exec(ctx, query, paymentID, eventID)
	return err
}

// parseGrossAmount parses the gross amount string from Midtrans to int64 (IDR, no cents)
// Midtrans sends amount as string like "10000.00" (IDR, no fractional cents)
func parseGrossAmount(amountStr string) int64 {
	// Parse as float first, then convert to int64
	var amount float64
	fmt.Sscanf(amountStr, "%f", &amount)
	return int64(amount)
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
