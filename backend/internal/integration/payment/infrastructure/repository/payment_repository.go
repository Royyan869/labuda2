package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

// ErrInvalidStatusTransition is returned when attempting to transition a payment
// from an unexpected current status. This indicates the payment state has changed
// since it was read, providing database-level idempotency and concurrency safety.
var ErrInvalidStatusTransition = errors.New("invalid payment status transition")

// ErrPaymentAlreadyExists is returned when attempting to create a duplicate payment
// for an order that already has an active payment.
var ErrPaymentAlreadyExists = errors.New("payment already exists for this order")

// ErrAmountMismatch is returned when the payment amount doesn't match the expected amount.
var ErrAmountMismatch = errors.New("payment amount mismatch")

// PaymentRepository handles payment data access using pgx-based DB layer.
// It enforces row locking with FOR UPDATE to prevent race conditions.
type PaymentRepository struct {
	auditService interface { // Minimal interface to avoid circular import
		PaymentCreated(ctx context.Context, tx db.Tx, paymentID, userID uuid.UUID, amount int64)
	}
}

// CreatePaymentInput holds the parameters for creating a new payment.
type CreatePaymentInput struct {
	UserID           uuid.UUID
	PaymentNumber    string
	MidtransOrderID  string
	GrossAmount      money.Money
	ServiceFeeAmount money.Money
	CoinsToUse       int
	ReferenceType    string
	ReferenceID      *uuid.UUID
	PriceSnapshotID  *uuid.UUID
	ExpiredAt        time.Time
	// PaymentMethodCode is the canonical method the buyer selected (order
	// payments only; nil for billing/subscription payments).
	PaymentMethodCode *string
}

// CreatePayment creates a new payment with strict validation.
//
// VALIDATION RULES:
// 1. reference_type must be "trade" or "billing"
// 2. reference_id must NOT be null
// 3. All amount fields must be non-negative
//
// PHASE 7: PAYMENT EXPIRY SYNC (CRITICAL)
// 🔥 When creating a payment for an order, MUST sync expiry:
//
//	payment.ExpiredAt = order.PaymentExpiresAt
//
// This ensures:
// - No drift between payment and order expiry
// - Both worker paths use the same expiry time
// - Consistent behavior across the system
//
// IMPLEMENTATION NOTE:
// - Caller is responsible for fetching order.PaymentExpiresAt
// - Pass it as input.ExpiredAt when calling this method
// - For order payments, input.ExpiredAt MUST equal order.PaymentExpiresAt
//
// Returns ErrReferenceIDRequired if reference_id is nil.
func (r *PaymentRepository) CreatePayment(
	ctx context.Context,
	tx db.Tx,
	input CreatePaymentInput,
) (*Payment, error) {
	// VALIDATE: Reference type must be order, billing or subscription
	if input.ReferenceType != ReferenceTypeOrder && input.ReferenceType != ReferenceTypeBilling && input.ReferenceType != ReferenceTypeSubscription {
		return nil, fmt.Errorf("invalid reference_type: %s (must be 'order', 'billing' or 'subscription')", input.ReferenceType)
	}

	// VALIDATE: Reference ID is required
	if input.ReferenceID == nil {
		return nil, ErrReferenceIDRequired
	}

	paymentID := uuid.New()
	coinsToUse := input.CoinsToUse
	coinDiscountAmount := money.New(int64(coinsToUse))

	query := `
		INSERT INTO payments (
			id, user_id, payment_number, midtrans_order_id,
			gross_amount, service_fee_amount, coins_to_use, coin_discount_amount,
			status, reference_type, reference_id, price_snapshot_id,
			expired_at, created_at, updated_at, payment_method_code
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8,
			$9, $10, $11, $12,
			$13, NOW(), NOW(), $14
		)
		RETURNING id, user_id, payment_number, midtrans_order_id,
		          gross_amount, service_fee_amount, coins_to_use, coin_discount_amount,
		          status, reference_type, reference_id, price_snapshot_id,
		          payment_url, transaction_id, payment_type,
		          paid_at, expired_at, created_at, updated_at, payment_method_code
	`

	row := tx.QueryRow(ctx, query,
		paymentID,
		input.UserID,
		input.PaymentNumber,
		input.MidtransOrderID,
		input.GrossAmount.Int64(),
		input.ServiceFeeAmount.Int64(),
		coinsToUse,
		coinDiscountAmount.Int64(),
		PaymentStatusPending,
		input.ReferenceType,
		input.ReferenceID,
		input.PriceSnapshotID,
		input.ExpiredAt,
		input.PaymentMethodCode,
	)

	payment, err := scanPayment(row)
	if err != nil {
		return nil, err
	}

	// AUDIT: Emit payment.created event AFTER successful creation
	if r.auditService != nil {
		r.auditService.PaymentCreated(ctx, tx, payment.ID, payment.UserID, payment.GrossAmount.Int64())
	}

	return payment, nil
}

// =============================================================================
// PAYMENT DOMAIN CRITICAL HARDENING METHODS
// =============================================================================

// FindExistingPaymentForOrder checks if a payment already exists for an order.
// Returns the existing payment if found, nil if not found.
//
// CRITICAL: This method enforces the single payment per order constraint.
// It must be called BEFORE CreatePayment to prevent duplicate payments.
//
// The check looks for payments with status IN (pending, settlement, capture)
// to allow retry on failed payments.
func (r *PaymentRepository) FindExistingPaymentForOrder(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) (*Payment, error) {
	query := `
		SELECT id, user_id, payment_number, midtrans_order_id,
		       gross_amount, service_fee_amount, coins_to_use, coin_discount_amount,
		       status, reference_type, reference_id, price_snapshot_id,
		       payment_url, transaction_id, payment_type,
		       paid_at, expired_at, created_at, updated_at, payment_method_code
		FROM payments
		WHERE reference_type = $1
		  AND reference_id = $2
		  AND status IN ('pending', 'settlement', 'capture')
		LIMIT 1
	`

	row := tx.QueryRow(ctx, query, ReferenceTypeOrder, orderID)
	return scanPayment(row)
}

// FindPendingSubscriptionPayment retrieves a pending subscription payment for a user.
// Returns the existing pending payment if found (for idempotency), nil if not found.
// Used by subscription initiation to avoid creating duplicate pending payments.
func (r *PaymentRepository) FindPendingSubscriptionPayment(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) (*Payment, error) {
	query := `
		SELECT id, user_id, payment_number, midtrans_order_id,
		       gross_amount, service_fee_amount, coins_to_use, coin_discount_amount,
		       status, reference_type, reference_id, price_snapshot_id,
		       payment_url, transaction_id, payment_type,
		       paid_at, expired_at, created_at, updated_at, payment_method_code
		FROM payments
		WHERE reference_type = $1
		  AND user_id = $2
		  AND status = 'pending'
		  AND expired_at > NOW()
		ORDER BY created_at DESC
		LIMIT 1
	`

	row := tx.QueryRow(ctx, query, ReferenceTypeSubscription, userID)
	return scanPayment(row)
}

// UpdatePaymentURL sets the payment_url field after Midtrans Snap token generation.
func (r *PaymentRepository) UpdatePaymentURL(
	ctx context.Context,
	tx db.Tx,
	paymentID uuid.UUID,
	paymentURL string,
) error {
	query := `
		UPDATE payments
		SET payment_url = $1, updated_at = NOW()
		WHERE id = $2
	`

	result, err := tx.Exec(ctx, query, paymentURL, paymentID)
	if err != nil {
		return fmt.Errorf("failed to update payment URL: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("payment not found: %s", paymentID)
	}

	return nil
}

// GetOrCreateForOrder attempts to find an existing payment for an order,
// or creates a new one if none exists.
//
// This is the PRIMARY method for order payment creation, ensuring idempotency.
// Returns the payment (existing or new) and a boolean indicating if it was created.
//
// CRITICAL: Must be called within a transaction for atomicity.
func (r *PaymentRepository) GetOrCreateForOrder(
	ctx context.Context,
	tx db.Tx,
	input CreatePaymentInput,
) (*Payment, bool, error) {
	// VALIDATE: Reference type must be order
	if input.ReferenceType != ReferenceTypeOrder {
		return nil, false, fmt.Errorf("invalid reference_type: %s (must be 'order')", input.ReferenceType)
	}

	// VALIDATE: Reference ID is required
	if input.ReferenceID == nil {
		return nil, false, ErrReferenceIDRequired
	}

	// STEP 1: Check for existing payment
	existing, err := r.FindExistingPaymentForOrder(ctx, tx, *input.ReferenceID)
	if err == nil {
		// Existing payment found - return it
		return existing, false, nil
	}

	// If error is not "no rows", return the error
	if err.Error() != "no rows in result set" {
		return nil, false, fmt.Errorf("failed to check for existing payment: %w", err)
	}

	// STEP 2: No existing payment - create new one
	payment, err := r.CreatePayment(ctx, tx, input)
	if err != nil {
		// Check if this is a unique constraint violation
		// This can happen in a race condition where another payment was created
		// between our check and the insert
		if IsUniqueViolation(err) {
			// Retry the lookup to get the existing payment
			existing, err := r.FindExistingPaymentForOrder(ctx, tx, *input.ReferenceID)
			if err == nil {
				return existing, false, nil
			}
		}
		return nil, false, err
	}

	return payment, true, nil
}

// =============================================================================
// ORPHAN WEBHOOK RECOVERY METHODS
// =============================================================================

// OrphanWebhookEvent represents an orphaned webhook event that needs recovery.
type OrphanWebhookEvent struct {
	ID              uuid.UUID
	EventID         string
	MidtransOrderID string
	Payload         []byte
	ReceivedAt      time.Time
	ErrorMessage    *string
}

// GetOrphanedWebhookEvents retrieves orphaned webhook events for recovery.
// Returns events that are marked as 'orphaned' status.
//
// Used by the recovery worker to retry processing of orphaned webhooks.
func (r *PaymentRepository) GetOrphanedWebhookEvents(
	ctx context.Context,
	tx db.Tx,
	limit int,
) ([]OrphanWebhookEvent, error) {
	query := `
		SELECT id, event_id, midtrans_order_id, payload, received_at, error_message
		FROM payment_webhook_events
		WHERE status = 'orphaned'
		ORDER BY received_at DESC
		LIMIT $1
	`

	rows, err := tx.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get orphaned webhook events: %w", err)
	}
	defer rows.Close()

	var events []OrphanWebhookEvent
	for rows.Next() {
		var e OrphanWebhookEvent
		if err := rows.Scan(&e.ID, &e.EventID, &e.MidtransOrderID, &e.Payload, &e.ReceivedAt, &e.ErrorMessage); err != nil {
			return nil, fmt.Errorf("failed to scan orphaned webhook event: %w", err)
		}
		events = append(events, e)
	}

	return events, nil
}

// MarkWebhookEventForRetry marks an orphaned webhook event for retry processing.
// Changes status from 'orphaned' back to 'pending' for the recovery worker.
func (r *PaymentRepository) MarkWebhookEventForRetry(
	ctx context.Context,
	tx db.Tx,
	eventID string,
) error {
	query := `
		UPDATE payment_webhook_events
		SET status = 'pending',
		    error_message = NULL,
		    processed_at = NULL
		WHERE event_id = $1 AND status = 'orphaned'
	`

	result, err := tx.Exec(ctx, query, eventID)
	if err != nil {
		return fmt.Errorf("failed to mark webhook event for retry: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("webhook event not found or not orphaned: %s", eventID)
	}

	return nil
}

// GetPendingWebhookEvents retrieves webhook events that are pending processing.
// Used by the webhook processing worker.
func (r *PaymentRepository) GetPendingWebhookEvents(
	ctx context.Context,
	tx db.Tx,
	limit int,
) ([]OrphanWebhookEvent, error) {
	query := `
		SELECT id, event_id, midtrans_order_id, payload, received_at, error_message
		FROM payment_webhook_events
		WHERE status = 'pending'
		ORDER BY received_at ASC
		LIMIT $1
	`

	rows, err := tx.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending webhook events: %w", err)
	}
	defer rows.Close()

	var events []OrphanWebhookEvent
	for rows.Next() {
		var e OrphanWebhookEvent
		if err := rows.Scan(&e.ID, &e.EventID, &e.MidtransOrderID, &e.Payload, &e.ReceivedAt, &e.ErrorMessage); err != nil {
			return nil, fmt.Errorf("failed to scan pending webhook event: %w", err)
		}
		events = append(events, e)
	}

	return events, nil
}

// GetPaymentByReference retrieves a payment by reference type and ID.
// Used by webhook recovery to find payment after it was created.
func (r *PaymentRepository) GetPaymentByReference(
	ctx context.Context,
	tx db.Tx,
	referenceType string,
	referenceID uuid.UUID,
) (*Payment, error) {
	query := `
		SELECT id, user_id, payment_number, midtrans_order_id,
		       gross_amount, service_fee_amount, coins_to_use, coin_discount_amount,
		       status, reference_type, reference_id, price_snapshot_id,
		       payment_url, transaction_id, payment_type,
		       paid_at, expired_at, created_at, updated_at, payment_method_code
		FROM payments
		WHERE reference_type = $1 AND reference_id = $2
	`

	row := tx.QueryRow(ctx, query, referenceType, referenceID)
	return scanPayment(row)
}

// NewPaymentRepository creates a new PaymentRepository.
func NewPaymentRepository() *PaymentRepository {
	return &PaymentRepository{}
}

// SetAuditService sets the audit service for audit logging.
// This is called during dependency injection to enable audit events.
func (r *PaymentRepository) SetAuditService(auditService interface { // Minimal interface to avoid circular import
	PaymentCreated(ctx context.Context, tx db.Tx, paymentID, userID uuid.UUID, amount int64)
}) {
	r.auditService = auditService
}

// FindLatestSubscriptionPayment retrieves the most recent subscription payment for a user,
// regardless of status (pending, settlement, capture, deny, cancel, expire).
// Returns nil if no subscription payment exists for the user.
// Used by the seller sync route to inspect the latest payment state.
func (r *PaymentRepository) FindLatestSubscriptionPayment(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) (*Payment, error) {
	query := `
		SELECT id, user_id, payment_number, midtrans_order_id,
		       gross_amount, service_fee_amount, coins_to_use, coin_discount_amount,
		       status, reference_type, reference_id, price_snapshot_id,
		       payment_url, transaction_id, payment_type,
		       paid_at, expired_at, created_at, updated_at, payment_method_code
		FROM payments
		WHERE reference_type = $1
		  AND user_id = $2
		ORDER BY created_at DESC
		LIMIT 1
	`

	row := tx.QueryRow(ctx, query, ReferenceTypeSubscription, userID)
	p, err := scanPayment(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

// GetByMidtransOrderID retrieves a payment by Midtrans order ID without locking.
// Use this for read-only operations.
func (r *PaymentRepository) GetByMidtransOrderID(
	ctx context.Context,
	tx db.Tx,
	midtransOrderID string,
) (*Payment, error) {
	query := `
		SELECT id, user_id, payment_number, midtrans_order_id,
		       gross_amount, service_fee_amount, coins_to_use, coin_discount_amount,
		       status, reference_type, reference_id, price_snapshot_id,
		       payment_url, transaction_id, payment_type,
		       paid_at, expired_at, created_at, updated_at, payment_method_code
		FROM payments
		WHERE midtrans_order_id = $1
	`

	row := tx.QueryRow(ctx, query, midtransOrderID)
	return scanPayment(row)
}

// GetForUpdate retrieves a payment by Midtrans order ID with row locking.
// The FOR UPDATE lock prevents concurrent modifications, ensuring
// the payment status cannot change between read and write.
//
// CRITICAL: This method MUST be called within a transaction:
//
//	db.WithTx(ctx, func(tx db.Tx) error {
//	    payment, err := repo.GetForUpdate(ctx, tx, midtransOrderID)
//	    if err != nil {
//	        return err
//	    }
//	    // ... modify payment ...
//	    return repo.MarkAsSettlement(ctx, tx, payment.ID, transactionID, paymentType)
//	})
func (r *PaymentRepository) GetForUpdate(
	ctx context.Context,
	tx db.Tx,
	midtransOrderID string,
) (*Payment, error) {
	query := `
		SELECT id, user_id, payment_number, midtrans_order_id,
		       gross_amount, service_fee_amount, coins_to_use, coin_discount_amount,
		       status, reference_type, reference_id, price_snapshot_id,
		       payment_url, transaction_id, payment_type,
		       paid_at, expired_at, created_at, updated_at, payment_method_code
		FROM payments
		WHERE midtrans_order_id = $1
		FOR UPDATE
	`

	row := tx.QueryRow(ctx, query, midtransOrderID)
	return scanPayment(row)
}

// GetByID retrieves a payment by ID without locking.
// Use this for read-only operations where concurrent access is acceptable.
func (r *PaymentRepository) GetByID(
	ctx context.Context,
	tx db.Tx,
	paymentID uuid.UUID,
) (*Payment, error) {
	query := `
		SELECT id, user_id, payment_number, midtrans_order_id,
		       gross_amount, service_fee_amount, coins_to_use, coin_discount_amount,
		       status, reference_type, reference_id, price_snapshot_id,
		       payment_url, transaction_id, payment_type,
		       paid_at, expired_at, created_at, updated_at, payment_method_code
		FROM payments
		WHERE id = $1
	`

	row := tx.QueryRow(ctx, query, paymentID)
	return scanPayment(row)
}

// GetByIDForUpdate retrieves a payment by ID with row locking.
// Use this when you need to update the payment and prevent race conditions.
func (r *PaymentRepository) GetByIDForUpdate(
	ctx context.Context,
	tx db.Tx,
	paymentID uuid.UUID,
) (*Payment, error) {
	query := `
		SELECT id, user_id, payment_number, midtrans_order_id,
		       gross_amount, service_fee_amount, coins_to_use, coin_discount_amount,
		       status, reference_type, reference_id, price_snapshot_id,
		       payment_url, transaction_id, payment_type,
		       paid_at, expired_at, created_at, updated_at, payment_method_code
		FROM payments
		WHERE id = $1
		FOR UPDATE
	`

	row := tx.QueryRow(ctx, query, paymentID)
	return scanPayment(row)
}

// MarkAsSettlement updates a payment to settlement status.
// This is used when a payment webhook confirms successful payment.
//
// PHASE 3: CONDITIONAL UPDATE - ONLY update from pending status
// Ensures atomic transition: pending → settlement
// Prevents race conditions between webhook and expiry worker
//
// The method:
// 1. Sets status to 'settlement' ONLY if current status is 'pending'
// 2. Records transaction_id and payment_type from gateway
// 3. Sets paid_at timestamp
//
// Returns nil if the update succeeds.
// Returns an error if:
// - Payment is not found
// - Payment is not in pending status (conditional update failed)
// - Database error occurs
func (r *PaymentRepository) MarkAsSettlement(
	ctx context.Context,
	tx db.Tx,
	paymentID uuid.UUID,
	transactionID string,
	paymentType string,
) error {
	now := time.Now()

	// ============================================================
	// PHASE 3: CONDITIONAL UPDATE (ATOMIC TRANSITION)
	// ============================================================
	// 🔥 CRITICAL: Only update if status = 'pending'
	// This ensures only ONE path can win:
	// - Webhook: pending → settlement (wins)
	// - Worker: pending → expire (wins)
	// - Never: settlement → expire OR expire → settlement
	//
	// WHERE id = $5 AND status = 'pending' ensures atomicity
	query := `
		UPDATE payments
		SET status = $1,
		    transaction_id = $2,
		    payment_type = $3,
		    paid_at = $4,
		    updated_at = NOW()
		WHERE id = $5
		  AND status = 'pending'
	`

	result, err := tx.Exec(ctx, query,
		PaymentStatusSettlement,
		transactionID,
		paymentType,
		now,
		paymentID,
	)

	if err != nil {
		return fmt.Errorf("failed to mark payment as settlement: %w", err)
	}

	// Check if row was updated
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		// ============================================================
		// PHASE 3: CONDITIONAL UPDATE FAILED
		// ============================================================
		// Either:
		// 1. Payment not found OR
		// 2. Payment not in pending status (already settled/expired)
		//
		// Check current status to provide clear error
		currentPayment, err := r.GetByID(ctx, tx, paymentID)
		if err != nil {
			return fmt.Errorf("payment not found or status check failed: %s", paymentID)
		}
		return fmt.Errorf("payment not in pending status (current: %s): %s", currentPayment.Status, paymentID)
	}

	return nil
}

// MarkAsCapture updates a payment to capture status.
// Some payment gateways use 'capture' instead of 'settlement'.
func (r *PaymentRepository) MarkAsCapture(
	ctx context.Context,
	tx db.Tx,
	paymentID uuid.UUID,
	transactionID string,
	paymentType string,
) error {
	now := time.Now()

	query := `
		UPDATE payments
		SET status = $1,
		    transaction_id = $2,
		    payment_type = $3,
		    paid_at = $4,
		    updated_at = NOW()
		WHERE id = $5
	`

	result, err := tx.Exec(ctx, query,
		PaymentStatusCapture,
		transactionID,
		paymentType,
		now,
		paymentID,
	)

	if err != nil {
		return fmt.Errorf("failed to mark payment as capture: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("payment not found: %s", paymentID)
	}

	return nil
}

// MarkAsFailed updates a payment to failed status.
// Used for deny, cancel, or expire statuses.
//
// PHASE 3: CONDITIONAL UPDATE - ONLY update from pending status
// Ensures atomic transition: pending → failed (deny/cancel/expire)
// Prevents race conditions between webhook and expiry worker
func (r *PaymentRepository) MarkAsFailed(
	ctx context.Context,
	tx db.Tx,
	paymentID uuid.UUID,
	status string,
) error {
	// Validate status is a failed status
	if status != PaymentStatusDeny &&
		status != PaymentStatusCancel &&
		status != PaymentStatusExpire {
		return fmt.Errorf("invalid failed status: %s", status)
	}

	// ============================================================
	// PHASE 3: CONDITIONAL UPDATE (ATOMIC TRANSITION)
	// ============================================================
	// 🔥 CRITICAL: Only update if status = 'pending'
	// This ensures only ONE path can win:
	// - Webhook: pending → settlement (wins)
	// - Worker: pending → expire (wins)
	// - Never: settlement → expire OR expire → settlement
	//
	// WHERE id = $2 AND status = 'pending' ensures atomicity
	query := `
		UPDATE payments
		SET status = $1,
		    updated_at = NOW()
		WHERE id = $2
		  AND status = 'pending'
	`

	result, err := tx.Exec(ctx, query, status, paymentID)

	if err != nil {
		return fmt.Errorf("failed to mark payment as failed: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		// ============================================================
		// PHASE 3: CONDITIONAL UPDATE FAILED
		// ============================================================
		// Either:
		// 1. Payment not found OR
		// 2. Payment not in pending status (already settled/expired)
		//
		// Check current status to provide clear error
		currentPayment, err := r.GetByID(ctx, tx, paymentID)
		if err != nil {
			return fmt.Errorf("payment not found or status check failed: %s", paymentID)
		}
		return fmt.Errorf("payment not in pending status (current: %s): %s", currentPayment.Status, paymentID)
	}

	return nil
}

// UpdateStatus updates the payment status with a database-level state guard.
//
// The update is conditional on the current status matching fromStatus, providing:
// - Database-level state machine enforcement
// - Concurrency safety without read-before-write
// - Idempotency for retry scenarios
//
// Returns ErrInvalidStatusTransition if the payment is not in the expected state.
// Returns an error if the payment is not found or the update fails.
func (r *PaymentRepository) UpdateStatus(
	ctx context.Context,
	tx db.Tx,
	paymentID uuid.UUID,
	fromStatus string,
	toStatus string,
) error {
	query := `
		UPDATE payments
		SET status = $1,
		    updated_at = NOW()
		WHERE id = $2
		  AND status = $3
	`

	result, err := tx.Exec(ctx, query, toStatus, paymentID, fromStatus)

	if err != nil {
		return fmt.Errorf("failed to update payment status: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrInvalidStatusTransition
	}

	return nil
}

// UpdateStatusWithCheck updates the payment status only if the current status matches expectedStatus.
// Returns (rowsAffected, error) where rowsAffected indicates if the update was applied.
// This provides database-level idempotency for state transitions.
func (r *PaymentRepository) UpdateStatusWithCheck(
	ctx context.Context,
	tx db.Tx,
	paymentID uuid.UUID,
	newStatus string,
	expectedStatus string,
) (int64, error) {
	query := `
		UPDATE payments
		SET status = $1,
		    updated_at = NOW()
		WHERE id = $2
		  AND status = $3
	`

	result, err := tx.Exec(ctx, query, newStatus, paymentID, expectedStatus)

	if err != nil {
		return 0, fmt.Errorf("failed to update payment status: %w", err)
	}

	rowsAffected := result.RowsAffected()
	return rowsAffected, nil
}

// GetGrossAmount retrieves the gross amount for a payment.
// Useful for validation without loading the entire payment.
func (r *PaymentRepository) GetGrossAmount(
	ctx context.Context,
	tx db.Tx,
	paymentID uuid.UUID,
) (money.Money, error) {
	var amount int64

	err := tx.QueryRow(ctx,
		`SELECT gross_amount FROM payments WHERE id = $1`, paymentID,
	).Scan(&amount)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return money.Zero(), fmt.Errorf("payment not found: %s", paymentID)
		}
		return money.Zero(), fmt.Errorf("failed to get gross amount: %w", err)
	}

	return money.New(amount), nil
}

// GetGrossAmountByMidtransOrderID retrieves the gross amount by Midtrans order ID.
// Useful for webhook validation.
func (r *PaymentRepository) GetGrossAmountByMidtransOrderID(
	ctx context.Context,
	tx db.Tx,
	midtransOrderID string,
) (money.Money, error) {
	var amount int64

	err := tx.QueryRow(ctx,
		`SELECT gross_amount FROM payments WHERE midtrans_order_id = $1`, midtransOrderID,
	).Scan(&amount)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return money.Zero(), fmt.Errorf("payment not found: %s", midtransOrderID)
		}
		return money.Zero(), fmt.Errorf("failed to get gross amount: %w", err)
	}

	return money.New(amount), nil
}

// GetStatus retrieves the current status of a payment.
// Useful for idempotency checks.
func (r *PaymentRepository) GetStatus(
	ctx context.Context,
	tx db.Tx,
	paymentID uuid.UUID,
) (string, error) {
	var status string

	err := tx.QueryRow(ctx,
		`SELECT status FROM payments WHERE id = $1`, paymentID,
	).Scan(&status)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return "", fmt.Errorf("payment not found: %s", paymentID)
		}
		return "", fmt.Errorf("failed to get payment status: %w", err)
	}

	return status, nil
}

// CreatePaymentWebhookEvent records a payment webhook event for idempotency.
// This should be called first when processing a webhook to capture all events.
func (r *PaymentRepository) CreatePaymentWebhookEvent(
	ctx context.Context,
	tx db.Tx,
	eventID string,
	provider string,
	midtransOrderID *string,
	signatureKey string,
	payload []byte,
) error {
	query := `
		INSERT INTO payment_webhook_events
			(id, provider, event_id, midtrans_order_id, signature_key, payload, status, received_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
	`

	eventIDUUID := uuid.New()
	_, err := tx.Exec(ctx, query,
		eventIDUUID,
		provider,
		eventID,
		midtransOrderID,
		signatureKey,
		payload,
		PaymentWebhookEventStatusPending, // Always start as pending
	)

	if err != nil {
		return fmt.Errorf("failed to create webhook event: %w", err)
	}

	return nil
}

// UpdateWebhookEventStatus updates the status of a webhook event.
// Used for state machine transitions: pending -> processing -> succeeded/failed.
func (r *PaymentRepository) UpdateWebhookEventStatus(
	ctx context.Context,
	tx db.Tx,
	eventID string,
	status string,
	paymentID *uuid.UUID,
	errorMsg *string,
) error {
	query := `
		UPDATE payment_webhook_events
		SET status = $1,
		    payment_id = $2,
		    error_message = $3,
		    processed_at = CASE WHEN $1 IN ('succeeded', 'failed', 'orphaned', 'manual_review', 'quarantined', 'terminal_review') THEN NOW() ELSE NULL END
		WHERE event_id = $4
	`

	_, err := tx.Exec(ctx, query, status, paymentID, errorMsg, eventID)

	if err != nil {
		return fmt.Errorf("failed to update webhook event status: %w", err)
	}

	return nil
}

// LinkWebhookEventToPayment links a webhook event to its payment.
func (r *PaymentRepository) LinkWebhookEventToPayment(
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

	if err != nil {
		return fmt.Errorf("failed to link webhook event to payment: %w", err)
	}

	return nil
}

// IsUniqueViolation checks if error is PostgreSQL unique constraint violation (23505).
func IsUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "23505") ||
		contains(errStr, "duplicate key") ||
		contains(errStr, "UNIQUE constraint")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
