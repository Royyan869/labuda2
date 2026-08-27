package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	orderRepo "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

// PaymentSettlementService handles payment settlement.
//
// FINANCIAL AUTHORITY: WalletService is the ONLY authority for money operations.
// Payment settlement ONLY updates payment status - no money movement happens here.
// All money is held/released/refunded through WalletService.
//
// CRITICAL IDEMPOTENCY GUARANTEES:
// 1. Row-level locking (FOR UPDATE) prevents race conditions
// 2. Status check before any state transition
type PaymentSettlementService struct {
	paymentRepo *PaymentRepository
	orderRepo   *orderRepo.OrderRepository // PHASE 5: For order expiry validation
	auditService interface { // Minimal interface to avoid circular import
		PaymentSettled(ctx context.Context, tx db.Tx, paymentID uuid.UUID, amount int64)
		PaymentFailed(ctx context.Context, tx db.Tx, paymentID uuid.UUID, reason string)
	}
}

// NewPaymentSettlementService creates a new PaymentSettlementService.
func NewPaymentSettlementService() *PaymentSettlementService {
	return &PaymentSettlementService{
		paymentRepo: NewPaymentRepository(),
		orderRepo:   orderRepo.NewOrderRepository(), // PHASE 5: Initialize order repo
	}
}

// SetAuditService sets the audit service for audit logging.
// This is called during dependency injection to enable audit events.
func (s *PaymentSettlementService) SetAuditService(auditService interface { // Minimal interface to avoid circular import
	PaymentSettled(ctx context.Context, tx db.Tx, paymentID uuid.UUID, amount int64)
	PaymentFailed(ctx context.Context, tx db.Tx, paymentID uuid.UUID, reason string)
}) {
	s.auditService = auditService
}

// SettlePayment processes a successful payment by:
// 1. Locking the order first (if payment references an order)
// 2. Locking the payment row (FOR UPDATE)
// 3. Updating payment status to settlement
//
// PHASE 6: CONSISTENT LOCK ORDER - LOCK ORDER FIRST (IF APPLICABLE)
// Prevents race condition between payment webhook and expiry worker:
// - Both paths now lock ORDER → PAYMENT (consistent lock order)
// - Only one path can win (settle OR expire, never both)
//
// NOTE: Money is NOT moved here. All money operations are handled by WalletService:
// - HoldForOrder: Called during order creation to hold buyer funds
// - ReleaseEscrow: Called during order completion to release to seller
// - RefundEscrow: Called during order cancellation to refund buyer
//
// This method MUST be called within a transaction:
//
//	db.WithTx(ctx, func(tx db.Tx) error {
//	    return settlementService.SettlePayment(ctx, tx, midtransOrderID, transactionID, paymentType)
//	})
func (s *PaymentSettlementService) SettlePayment(
	ctx context.Context,
	tx db.Tx,
	midtransOrderID string,
	transactionID string,
	paymentType string,
) error {
	// ============================================================
	// STEP 0: FETCH PAYMENT WITHOUT LOCK (to get reference info)
	// ============================================================
	// We need to know if this payment references an order before locking
	// This is a lightweight read without row lock
	payment, err := s.paymentRepo.GetByMidtransOrderID(ctx, tx, midtransOrderID)
	if err != nil {
		return fmt.Errorf("failed to get payment: %w", err)
	}

	// ============================================================
	// STEP 1: LOCK ORDER FIRST (PHASE 6: CONSISTENT LOCK ORDER)
	// ============================================================
	// 🔥 CRITICAL LOCK ORDER: ORDER → PAYMENT (prevents deadlock)
	// - Must lock ORDER before PAYMENT to avoid race with expiry worker
	// - Expiry worker locks: ORDER → PAYMENT
	// - Payment settlement locks: ORDER → PAYMENT (same order)
	// - Result: No deadlock, single writer guarantee
	//
	// If payment references an order, lock it FIRST before locking payment
	// This ensures that expiry worker and settlement cannot run concurrently
	if payment.ReferenceType == ReferenceTypeOrder && payment.ReferenceID != nil {
		order, err := s.orderRepo.GetForUpdate(ctx, tx, *payment.ReferenceID)
		if err != nil {
			return fmt.Errorf("failed to get order for locking: %w", err)
		}

		// ============================================================
		// PHASE 5: HARD BLOCK PAYMENT AFTER EXPIRY (CRITICAL)
		// ============================================================
		// Check expiry AFTER lock (atomic check-and-set)
		// This prevents payment settlement after the payment window has closed
		if order.IsExpired() {
			return fmt.Errorf("order expired - reject settlement: order_id=%s, payment_expires_at=%v",
				order.ID, order.PaymentExpiresAt)
		}
	}

	// ============================================================
	// STEP 2: LOCK PAYMENT (AFTER ORDER)
	// ============================================================
	// Now lock payment (after order is locked if applicable)
	payment, err = s.paymentRepo.GetForUpdate(ctx, tx, midtransOrderID)
	if err != nil {
		return fmt.Errorf("failed to get payment for update: %w", err)
	}

	// ============================================================
	// STEP 3: IDEMPOTENCY CHECK - if already settled, return success
	// ============================================================
	if payment.IsSettled() {
		return nil // Idempotent - already processed
	}

	// ============================================================
	// STEP 4: Validate status before proceeding
	// ============================================================
	if !payment.IsPending() {
		return fmt.Errorf("payment is not pending: current status=%s", payment.Status)
	}

	// ============================================================
	// STEP 5: Update payment status to settlement
	// ============================================================
	// NOTE: Money is NOT moved here. All money operations are handled by WalletService.
	// Funds were already held when order was created via WalletService.HoldForOrder.
	if err := s.paymentRepo.MarkAsSettlement(ctx, tx, payment.ID, transactionID, paymentType); err != nil {
		return fmt.Errorf("failed to mark payment as settlement: %w", err)
	}

	// AUDIT: Emit payment.settled event AFTER successful settlement
	if s.auditService != nil {
		s.auditService.PaymentSettled(ctx, tx, payment.ID, payment.GrossAmount.Int64())
	}

	return nil
}

// SettlePaymentByID processes a successful payment by payment ID.
// Similar to SettlePayment but uses payment ID instead of Midtrans order ID.
//
// CRITICAL: This method is idempotent by payment_id.
// Multiple calls with the same payment_id will safely return success.
//
// PHASE 2: SINGLE WRITER GUARANTEE - LOCK ORDER FIRST
// Prevents race condition between payment webhook and expiry worker:
// - Both paths now lock ORDER → PAYMENT (consistent lock order)
// - Only one path can win (settle OR expire, never both)
//
// PHASE 5 EXPIRY GUARD: Rejects payment settlement if the associated order is expired.
func (s *PaymentSettlementService) SettlePaymentByID(
	ctx context.Context,
	tx db.Tx,
	paymentID uuid.UUID,
	transactionID string,
	paymentType string,
) error {
	// ============================================================
	// STEP 0: FETCH PAYMENT WITHOUT LOCK (to get reference info)
	// ============================================================
	// We need to know if this payment references an order before locking
	// This is a lightweight read without row lock
	payment, err := s.paymentRepo.GetByID(ctx, tx, paymentID)
	if err != nil {
		return fmt.Errorf("failed to get payment: %w", err)
	}

	// ============================================================
	// STEP 1: LOCK ORDER FIRST (PHASE 2: SINGLE WRITER GUARANTEE)
	// ============================================================
	// 🔥 CRITICAL LOCK ORDER: ORDER → PAYMENT (prevents deadlock)
	// - Must lock ORDER before PAYMENT to avoid race with expiry worker
	// - Expiry worker locks: ORDER → PAYMENT
	// - Payment settlement locks: ORDER → PAYMENT (same order)
	// - Result: No deadlock, single writer guarantee
	//
	// If payment references an order, lock it FIRST before locking payment
	// This ensures that expiry worker and settlement cannot run concurrently
	if payment.ReferenceType == ReferenceTypeOrder && payment.ReferenceID != nil {
		order, err := s.orderRepo.GetForUpdate(ctx, tx, *payment.ReferenceID)
		if err != nil {
			return fmt.Errorf("failed to get order for locking: %w", err)
		}

		// ============================================================
		// PHASE 5: HARD BLOCK PAYMENT AFTER EXPIRY (CRITICAL)
		// ============================================================
		// Check expiry AFTER lock (atomic check-and-set)
		// This prevents payment settlement after the payment window has closed
		if order.IsExpired() {
			return fmt.Errorf("order expired - reject settlement: order_id=%s, payment_expires_at=%v",
				order.ID, order.PaymentExpiresAt)
		}
	}

	// ============================================================
	// STEP 2: LOCK PAYMENT (AFTER ORDER)
	// ============================================================
	// Now lock payment (after order is locked if applicable)
	payment, err = s.paymentRepo.GetByIDForUpdate(ctx, tx, paymentID)
	if err != nil {
		return fmt.Errorf("failed to get payment for update: %w", err)
	}

	// ============================================================
	// STEP 3: IDEMPOTENCY CHECK - if already settled, return success
	// ============================================================
	// This is the PRIMARY defense against double settlement
	if payment.IsSettled() {
		return nil // Idempotent - already processed
	}

	// ============================================================
	// STEP 4: Validate status before proceeding
	// ============================================================
	if !payment.IsPending() {
		return fmt.Errorf("payment is not pending: current status=%s", payment.Status)
	}

	// ============================================================
	// STEP 5: Update payment status to settlement
	// ============================================================
	// NOTE: Money is NOT moved here. All money operations are handled by WalletService.
	if err := s.paymentRepo.MarkAsSettlement(ctx, tx, payment.ID, transactionID, paymentType); err != nil {
		return fmt.Errorf("failed to mark payment as settlement: %w", err)
	}

	// AUDIT: Emit payment.settled event AFTER successful settlement
	if s.auditService != nil {
		s.auditService.PaymentSettled(ctx, tx, payment.ID, payment.GrossAmount.Int64())
	}

	return nil
}

// IsPaymentSettled checks if a payment has been settled without locking.
// Useful for pre-flight checks.
func (s *PaymentSettlementService) IsPaymentSettled(
	ctx context.Context,
	tx db.Tx,
	paymentID uuid.UUID,
) (bool, error) {
	status, err := s.paymentRepo.GetStatus(ctx, tx, paymentID)
	if err != nil {
		return false, err
	}
	return status == PaymentStatusSettlement || status == PaymentStatusCapture, nil
}

// SettlePaymentWithOutbox processes a payment settlement and creates an outbox event.
// This is used for order payments that need to trigger order completion.
func (s *PaymentSettlementService) SettlePaymentWithOutbox(
	ctx context.Context,
	tx db.Tx,
	midtransOrderID string,
	transactionID string,
	paymentType string,
) (*Payment, error) {
	// Step 1: Get payment with row lock
	payment, err := s.paymentRepo.GetForUpdate(ctx, tx, midtransOrderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get payment for update: %w", err)
	}

	// Step 2: Idempotency check
	if payment.IsSettled() {
		return payment, nil
	}

	// Step 3: Validate status
	if !payment.IsPending() {
		return nil, fmt.Errorf("payment is not pending: current status=%s", payment.Status)
	}

	// Step 4: Update payment status
	// NOTE: Money is NOT moved here. All money operations are handled by WalletService.
	if err := s.paymentRepo.MarkAsSettlement(ctx, tx, payment.ID, transactionID, paymentType); err != nil {
		return nil, fmt.Errorf("failed to mark payment as settlement: %w", err)
	}

	return payment, nil
}

// FailPayment marks a payment as failed (deny, cancel, or expire).
// No ledger entries are created for failed payments.
func (s *PaymentSettlementService) FailPayment(
	ctx context.Context,
	tx db.Tx,
	midtransOrderID string,
	failedStatus string,
) error {
	// Validate failed status
	if failedStatus != PaymentStatusDeny &&
		failedStatus != PaymentStatusCancel &&
		failedStatus != PaymentStatusExpire {
		return fmt.Errorf("invalid failed status: %s", failedStatus)
	}

	// Get payment with row lock
	payment, err := s.paymentRepo.GetForUpdate(ctx, tx, midtransOrderID)
	if err != nil {
		return fmt.Errorf("failed to get payment for update: %w", err)
	}

	// Idempotency check
	if payment.IsFailed() || payment.IsSettled() {
		return nil // Already processed
	}

	// Update payment status
	if err := s.paymentRepo.MarkAsFailed(ctx, tx, payment.ID, failedStatus); err != nil {
		return fmt.Errorf("failed to mark payment as failed: %w", err)
	}

	// AUDIT: Emit payment.failed event AFTER failure handling
	if s.auditService != nil {
		s.auditService.PaymentFailed(ctx, tx, payment.ID, failedStatus)
	}

	return nil
}

// ValidatePaymentAmount validates that the webhook amount matches the payment amount.
// Returns nil if amounts match, error otherwise.
func (s *PaymentSettlementService) ValidatePaymentAmount(
	ctx context.Context,
	tx db.Tx,
	midtransOrderID string,
	webhookAmount money.Money,
) error {
	paymentAmount, err := s.paymentRepo.GetGrossAmountByMidtransOrderID(ctx, tx, midtransOrderID)
	if err != nil {
		return fmt.Errorf("failed to get payment amount: %w", err)
	}

	if !paymentAmount.Equal(webhookAmount) {
		return fmt.Errorf("amount mismatch: payment=%d, webhook=%d",
			paymentAmount.Int64(), webhookAmount.Int64())
	}

	return nil
}

// GetPayment retrieves a payment without locking.
func (s *PaymentSettlementService) GetPayment(
	ctx context.Context,
	tx db.Tx,
	midtransOrderID string,
) (*Payment, error) {
	return s.paymentRepo.GetByMidtransOrderID(ctx, tx, midtransOrderID)
}

// GetPaymentForUpdate retrieves a payment with row locking.
func (s *PaymentSettlementService) GetPaymentForUpdate(
	ctx context.Context,
	tx db.Tx,
	midtransOrderID string,
) (*Payment, error) {
	return s.paymentRepo.GetForUpdate(ctx, tx, midtransOrderID)
}

// GetPaymentStatus retrieves the current status of a payment.
func (s *PaymentSettlementService) GetPaymentStatus(
	ctx context.Context,
	tx db.Tx,
	midtransOrderID string,
) (string, error) {
	payment, err := s.paymentRepo.GetByMidtransOrderID(ctx, tx, midtransOrderID)
	if err != nil {
		return "", err
	}
	return payment.Status, nil
}


