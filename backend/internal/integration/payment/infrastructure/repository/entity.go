package repository

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/money"
)

// Payment represents a payment row from the payments table.
// This is a pure pgx-based entity (no GORM tags).
type Payment struct {
	ID                 uuid.UUID
	UserID             uuid.UUID
	PaymentNumber      string
	MidtransOrderID    string
	GrossAmount        money.Money
	ServiceFeeAmount   money.Money
	CoinsToUse         int
	CoinDiscountAmount money.Money
	Status             string
	ReferenceType      string
	ReferenceID        *uuid.UUID
	PriceSnapshotID    *uuid.UUID
	// PaymentMethodCode is the canonical method the buyer selected before
	// this payment was created (PASS_18V). NULL for non-order payments
	// (billing/subscription), which are out of scope for the method-based
	// fee model.
	PaymentMethodCode *string

	// Midtrans response fields
	PaymentURL    *string
	TransactionID *string
	PaymentType   *string

	// Timestamps
	PaidAt    *time.Time
	ExpiredAt time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PaymentStatus constants
const (
	PaymentStatusPending    = "pending"
	PaymentStatusSettlement = "settlement"
	PaymentStatusCapture    = "capture"
	PaymentStatusDeny       = "deny"
	PaymentStatusCancel     = "cancel"
	PaymentStatusExpire     = "expire"
)

// PaymentWebhookEventStatus constants mirror payment_webhook_status_enum.
const (
	PaymentWebhookEventStatusPending        = "pending"
	PaymentWebhookEventStatusProcessing     = "processing"
	PaymentWebhookEventStatusSucceeded      = "succeeded"
	PaymentWebhookEventStatusFailed         = "failed"
	PaymentWebhookEventStatusOrphaned       = "orphaned"
	PaymentWebhookEventStatusManualReview   = "manual_review"
	PaymentWebhookEventStatusQuarantined    = "quarantined"
	PaymentWebhookEventStatusTerminalReview = "terminal_review"
	// PaymentWebhookEventStatusCapturedAfterExpiry (PASS_18T) marks a webhook
	// that reported a successful gateway transaction (settlement/capture) for
	// a payment the platform already expired. It must never be conflated with
	// "succeeded" (which means the platform's own state was updated) — this
	// value keeps captured-but-unreconciled money durably distinguishable so
	// it is not silently indistinguishable from a normal idempotent replay.
	PaymentWebhookEventStatusCapturedAfterExpiry = "captured_after_expiry"
)

// ReferenceType constants
const (
	// ReferenceTypeOrder is for order payments
	ReferenceTypeOrder = "order"
	// ReferenceTypeBilling is for billing payments (promotion package, etc.)
	ReferenceTypeBilling = "billing"
	// ReferenceTypeSubscription is for seller subscription payments
	ReferenceTypeSubscription = "subscription"
)

// ErrReferenceIDRequired is returned when reference_id is nil for a payment
var ErrReferenceIDRequired = fmt.Errorf("payment reference_id is required")

// IsPending returns true if payment status is pending.
func (p *Payment) IsPending() bool {
	return p.Status == PaymentStatusPending
}

// IsSettled returns true if payment is settled or captured.
func (p *Payment) IsSettled() bool {
	return p.Status == PaymentStatusSettlement || p.Status == PaymentStatusCapture
}

// IsFailed returns true if payment has failed.
func (p *Payment) IsFailed() bool {
	return p.Status == PaymentStatusDeny ||
		p.Status == PaymentStatusCancel ||
		p.Status == PaymentStatusExpire
}

// IsExpired returns true if the payment was closed out by PaymentExpiryWorker
// (as opposed to deny/cancel, which come from the gateway itself). A gateway
// success notification arriving after this point is a capture-after-expiry
// event, not an ordinary already-processed replay.
func (p *Payment) IsExpired() bool {
	return p.Status == PaymentStatusExpire
}

// scanPayment scans a pgx row into a Payment struct.
func scanPayment(row scanner) (*Payment, error) {
	var p Payment
	var referenceID, priceSnapshotID *uuid.UUID
	var paymentURL, transactionID, paymentType, paymentMethodCode *string
	var paidAt *time.Time

	err := row.Scan(
		&p.ID,
		&p.UserID,
		&p.PaymentNumber,
		&p.MidtransOrderID,
		&p.GrossAmount,
		&p.ServiceFeeAmount,
		&p.CoinsToUse,
		&p.CoinDiscountAmount,
		&p.Status,
		&p.ReferenceType,
		&referenceID,
		&priceSnapshotID,
		&paymentURL,
		&transactionID,
		&paymentType,
		&paidAt,
		&p.ExpiredAt,
		&p.CreatedAt,
		&p.UpdatedAt,
		&paymentMethodCode,
	)

	if err != nil {
		return nil, err
	}

	p.ReferenceID = referenceID
	p.PriceSnapshotID = priceSnapshotID
	p.PaymentURL = paymentURL
	p.TransactionID = transactionID
	p.PaymentType = paymentType
	p.PaidAt = paidAt
	p.PaymentMethodCode = paymentMethodCode

	return &p, nil
}

// scanner is an interface that matches pgx.Row and pgx.Rows.
// This allows scanPayment to work with both QueryRow and Query.
type scanner interface {
	Scan(dest ...any) error
}

// =============================================================================
// PAYMENT ATTEMPT ENTITY (BNR Phase 1)
// =============================================================================

// PaymentAttempt represents a payment attempt row from the payment_attempts table.
// BNR Phase 1: Tracks user payment intent to distinguish between non-payers
// and failed payment attempts. Real signals only - no heuristics.
type PaymentAttempt struct {
	ID                    uuid.UUID
	OrderID               uuid.UUID
	UserID                uuid.UUID
	AttemptAt             time.Time
	CheckoutStarted       bool
	PaymentMethodSelected *string
	GatewayReached        bool
	Status                string
	FailureReason         *string
	GatewayProvider       string
	GatewayTransactionID  *string
	TimeToCheckoutSeconds *int
	TimeInPaymentSeconds  *int
	UserAgent             *string
	IPAddress             *string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// PaymentAttemptStatus constants
const (
	PaymentAttemptStatusInitiated = "initiated"
	PaymentAttemptStatusPending   = "pending"
	PaymentAttemptStatusSuccess   = "success"
	PaymentAttemptStatusFailed    = "failed"
	PaymentAttemptStatusCancelled = "cancelled"
	PaymentAttemptStatusTimeout   = "timeout"
)

// PaymentAttemptFailureReason constants
const (
	FailureReasonUserCancelled = "user_cancelled"
	FailureReasonGatewayDenied = "gateway_denied"
	FailureReasonNetworkError  = "network_error"
	FailureReasonTimeout       = "timeout"
	FailureReasonUnknown       = "unknown"
)

// IsCompleted returns true if the payment attempt reached a final state.
func (p *PaymentAttempt) IsCompleted() bool {
	return p.Status == PaymentAttemptStatusSuccess ||
		p.Status == PaymentAttemptStatusFailed ||
		p.Status == PaymentAttemptStatusCancelled ||
		p.Status == PaymentAttemptStatusTimeout
}

// IsPending returns true if the payment attempt is still in progress.
func (p *PaymentAttempt) IsPending() bool {
	return p.Status == PaymentAttemptStatusInitiated ||
		p.Status == PaymentAttemptStatusPending
}

// scanPaymentAttempt scans a pgx row into a PaymentAttempt struct.
func scanPaymentAttempt(row scanner) (*PaymentAttempt, error) {
	var p PaymentAttempt
	err := row.Scan(
		&p.ID,
		&p.OrderID,
		&p.UserID,
		&p.AttemptAt,
		&p.CheckoutStarted,
		&p.PaymentMethodSelected,
		&p.GatewayReached,
		&p.Status,
		&p.FailureReason,
		&p.GatewayProvider,
		&p.GatewayTransactionID,
		&p.TimeToCheckoutSeconds,
		&p.TimeInPaymentSeconds,
		&p.UserAgent,
		&p.IPAddress,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}
