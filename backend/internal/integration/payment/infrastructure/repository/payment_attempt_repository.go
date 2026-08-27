package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// PaymentAttemptRepository handles payment attempt data access.
// BNR Phase 1: Tracks user payment intent for BNR (Buyer No-Response) analysis.
type PaymentAttemptRepository struct {
	log *zap.Logger
}

// NewPaymentAttemptRepository creates a new PaymentAttemptRepository.
func NewPaymentAttemptRepository(log *zap.Logger) *PaymentAttemptRepository {
	if log == nil {
		log = zap.NewNop()
	}
	return &PaymentAttemptRepository{
		log: log,
	}
}

// =============================================================================
// CREATE - Track a new payment attempt
// =============================================================================

// CreatePaymentAttemptInput holds the parameters for creating a payment attempt.
type CreatePaymentAttemptInput struct {
	OrderID               uuid.UUID
	UserID                uuid.UUID
	UserAgent             *string
	IPAddress             *string
	PaymentMethodSelected *string
}

// Create creates a new payment attempt record.
// This should be called when an order is created to track checkout start.
func (r *PaymentAttemptRepository) Create(
	ctx context.Context,
	tx db.Tx,
	input CreatePaymentAttemptInput,
) (*PaymentAttempt, error) {
	attemptID := uuid.New()
	now := time.Now()

	query := `
		INSERT INTO payment_attempts (
			id, order_id, user_id, attempt_at,
			checkout_started, payment_method_selected, gateway_reached,
			status, gateway_provider, user_agent, ip_address,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7,
			$8, $9, $10, $11,
			NOW(), NOW()
		)
		RETURNING id, order_id, user_id, attempt_at,
		          checkout_started, payment_method_selected, gateway_reached,
		          status, failure_reason, gateway_provider, gateway_transaction_id,
		          time_to_checkout_seconds, time_in_payment_seconds,
		          user_agent, ip_address, created_at, updated_at
	`

	row := tx.QueryRow(ctx, query,
		attemptID,
		input.OrderID,
		input.UserID,
		now,
		true,                              // checkout_started = true (EVENT 1)
		input.PaymentMethodSelected,       // EVENT 2 (optional)
		false,                             // gateway_reached = false initially
		PaymentAttemptStatusInitiated,     // status = initiated
		"midtrans",                        // gateway_provider
		input.UserAgent,
		input.IPAddress,
	)

	attempt, err := scanPaymentAttempt(row)
	if err != nil {
		return nil, fmt.Errorf("failed to create payment attempt: %w", err)
	}

	// Log the payment attempt creation
	r.log.Info("payment_attempt recorded",
		zap.String("attempt_id", attempt.ID.String()),
		zap.String("order_id", attempt.OrderID.String()),
		zap.String("user_id", attempt.UserID.String()),
		zap.String("status", attempt.Status),
		zap.Bool("checkout_started", attempt.CheckoutStarted),
	)

	return attempt, nil
}

// =============================================================================
// UPDATE - Track payment progress events
// =============================================================================

// MarkGatewayReached marks that the payment gateway was reached (EVENT 3).
// This is called when the payment URL is generated/redirect happens.
func (r *PaymentAttemptRepository) MarkGatewayReached(
	ctx context.Context,
	tx db.Tx,
	attemptID uuid.UUID,
	gatewayTransactionID string,
) error {
	query := `
		UPDATE payment_attempts
		SET gateway_reached = true,
		    gateway_transaction_id = $2,
		    status = $3,
		    updated_at = NOW()
		WHERE id = $1
	`

	result, err := tx.Exec(ctx, query,
		attemptID,
		gatewayTransactionIdOrNull(gatewayTransactionID),
		PaymentAttemptStatusPending,
	)

	if err != nil {
		return fmt.Errorf("failed to mark gateway reached: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("payment attempt not found: %s", attemptID)
	}

	r.log.Info("payment_attempt gateway_reached",
		zap.String("attempt_id", attemptID.String()),
		zap.String("gateway_transaction_id", gatewayTransactionID),
	)

	return nil
}

// MarkSuccess marks a payment attempt as successful.
// This is called when payment webhook confirms successful payment.
func (r *PaymentAttemptRepository) MarkSuccess(
	ctx context.Context,
	tx db.Tx,
	attemptID uuid.UUID,
	timeToCheckoutSeconds *int,
	timeInPaymentSeconds *int,
) error {
	query := `
		UPDATE payment_attempts
		SET status = $1,
		    time_to_checkout_seconds = $2,
		    time_in_payment_seconds = $3,
		    updated_at = NOW()
		WHERE id = $4
	`

	result, err := tx.Exec(ctx, query,
		PaymentAttemptStatusSuccess,
		timeToCheckoutSeconds,
		timeInPaymentSeconds,
		attemptID,
	)

	if err != nil {
		return fmt.Errorf("failed to mark payment attempt as success: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("payment attempt not found: %s", attemptID)
	}

	r.log.Info("payment_attempt success",
		zap.String("attempt_id", attemptID.String()),
		zap.String("status", PaymentAttemptStatusSuccess),
	)

	return nil
}

// MarkFailed marks a payment attempt as failed with a normalized failure reason.
// This is called when payment webhook indicates payment failure.
func (r *PaymentAttemptRepository) MarkFailed(
	ctx context.Context,
	tx db.Tx,
	attemptID uuid.UUID,
	failureReason string,
	timeInPaymentSeconds *int,
) error {
	query := `
		UPDATE payment_attempts
		SET status = $1,
		    failure_reason = $2,
		    time_in_payment_seconds = $3,
		    updated_at = NOW()
		WHERE id = $4
	`

	result, err := tx.Exec(ctx, query,
		PaymentAttemptStatusFailed,
		failureReasonOrNull(failureReason),
		timeInPaymentSeconds,
		attemptID,
	)

	if err != nil {
		return fmt.Errorf("failed to mark payment attempt as failed: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("payment attempt not found: %s", attemptID)
	}

	r.log.Info("payment_attempt failed",
		zap.String("attempt_id", attemptID.String()),
		zap.String("status", PaymentAttemptStatusFailed),
		zap.String("failure_reason", failureReason),
	)

	return nil
}

// MarkCancelled marks a payment attempt as cancelled by user.
// This is called when user explicitly cancels the payment.
func (r *PaymentAttemptRepository) MarkCancelled(
	ctx context.Context,
	tx db.Tx,
	attemptID uuid.UUID,
) error {
	query := `
		UPDATE payment_attempts
		SET status = $1,
		    failure_reason = $2,
		    updated_at = NOW()
		WHERE id = $3
	`

	result, err := tx.Exec(ctx, query,
		PaymentAttemptStatusCancelled,
		FailureReasonUserCancelled,
		attemptID,
	)

	if err != nil {
		return fmt.Errorf("failed to mark payment attempt as cancelled: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("payment attempt not found: %s", attemptID)
	}

	r.log.Info("payment_attempt cancelled",
		zap.String("attempt_id", attemptID.String()),
		zap.String("status", PaymentAttemptStatusCancelled),
	)

	return nil
}

// MarkTimeout marks a payment attempt as timed out.
// This is called when payment expires without completion.
func (r *PaymentAttemptRepository) MarkTimeout(
	ctx context.Context,
	tx db.Tx,
	attemptID uuid.UUID,
) error {
	query := `
		UPDATE payment_attempts
		SET status = $1,
		    failure_reason = $2,
		    updated_at = NOW()
		WHERE id = $3
	`

	result, err := tx.Exec(ctx, query,
		PaymentAttemptStatusTimeout,
		FailureReasonTimeout,
		attemptID,
	)

	if err != nil {
		return fmt.Errorf("failed to mark payment attempt as timeout: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("payment attempt not found: %s", attemptID)
	}

	r.log.Info("payment_attempt timeout",
		zap.String("attempt_id", attemptID.String()),
		zap.String("status", PaymentAttemptStatusTimeout),
	)

	return nil
}

// =============================================================================
// QUERY - Retrieve payment attempts
// =============================================================================

// GetPaymentAttemptsByOrderID retrieves all payment attempts for an order,
// ordered by attempt_at descending (newest first).
func (r *PaymentAttemptRepository) GetPaymentAttemptsByOrderID(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) ([]*PaymentAttempt, error) {
	query := `
		SELECT id, order_id, user_id, attempt_at,
		       checkout_started, payment_method_selected, gateway_reached,
		       status, failure_reason, gateway_provider, gateway_transaction_id,
		       time_to_checkout_seconds, time_in_payment_seconds,
		       user_agent, ip_address, created_at, updated_at
		FROM payment_attempts
		WHERE order_id = $1
		ORDER BY attempt_at DESC
	`

	rows, err := tx.Query(ctx, query, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get payment attempts: %w", err)
	}
	defer rows.Close()

	var attempts []*PaymentAttempt
	for rows.Next() {
		attempt, err := scanPaymentAttempt(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan payment attempt: %w", err)
		}
		attempts = append(attempts, attempt)
	}

	return attempts, nil
}

// GetLatestPaymentAttemptByOrderID retrieves the most recent payment attempt for an order.
func (r *PaymentAttemptRepository) GetLatestPaymentAttemptByOrderID(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) (*PaymentAttempt, error) {
	query := `
		SELECT id, order_id, user_id, attempt_at,
		       checkout_started, payment_method_selected, gateway_reached,
		       status, failure_reason, gateway_provider, gateway_transaction_id,
		       time_to_checkout_seconds, time_in_payment_seconds,
		       user_agent, ip_address, created_at, updated_at
		FROM payment_attempts
		WHERE order_id = $1
		ORDER BY attempt_at DESC
		LIMIT 1
	`

	row := tx.QueryRow(ctx, query, orderID)
	return scanPaymentAttempt(row)
}

// GetByID retrieves a payment attempt by ID.
func (r *PaymentAttemptRepository) GetByID(
	ctx context.Context,
	tx db.Tx,
	attemptID uuid.UUID,
) (*PaymentAttempt, error) {
	query := `
		SELECT id, order_id, user_id, attempt_at,
		       checkout_started, payment_method_selected, gateway_reached,
		       status, failure_reason, gateway_provider, gateway_transaction_id,
		       time_to_checkout_seconds, time_in_payment_seconds,
		       user_agent, ip_address, created_at, updated_at
		FROM payment_attempts
		WHERE id = $1
	`

	row := tx.QueryRow(ctx, query, attemptID)
	return scanPaymentAttempt(row)
}

// GetByGatewayTransactionID retrieves a payment attempt by gateway transaction ID.
// Used for webhook reconciliation.
func (r *PaymentAttemptRepository) GetByGatewayTransactionID(
	ctx context.Context,
	tx db.Tx,
	gatewayTransactionID string,
) (*PaymentAttempt, error) {
	query := `
		SELECT id, order_id, user_id, attempt_at,
		       checkout_started, payment_method_selected, gateway_reached,
		       status, failure_reason, gateway_provider, gateway_transaction_id,
		       time_to_checkout_seconds, time_in_payment_seconds,
		       user_agent, ip_address, created_at, updated_at
		FROM payment_attempts
		WHERE gateway_transaction_id = $1
		ORDER BY attempt_at DESC
		LIMIT 1
	`

	row := tx.QueryRow(ctx, query, gatewayTransactionID)
	return scanPaymentAttempt(row)
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// gatewayTransactionIdOrNull returns the string pointer or nil if empty.
func gatewayTransactionIdOrNull(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// failureReasonOrNull validates and returns the failure reason pointer.
func failureReasonOrNull(reason string) interface{} {
	// Validate against allowed values
	switch reason {
	case FailureReasonUserCancelled,
	     FailureReasonGatewayDenied,
	     FailureReasonNetworkError,
	     FailureReasonTimeout,
	     FailureReasonUnknown:
		return reason
	default:
		// Default to unknown if invalid
		return FailureReasonUnknown
	}
}


