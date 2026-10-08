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
	// this payment was created. Every flow that carries a payment-method fee
	// sets it: order (PASS_18V), billing / promote balance top-up (PASS_18V),
	// and seller subscription (PMF-02).
	PaymentMethodCode *string

	// SubscriptionDurationDays is the purchased seller-subscription entitlement
	// length, snapshotted at initiation so a later config change cannot alter
	// an already-purchased subscription. NON-NULL exactly for subscription
	// payments; NULL for every other reference type.
	SubscriptionDurationDays *int

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
	// ReferenceTypeBilling is for billing payments (Promote Balance top-up, etc.)
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
	return IsSettledStatus(p.Status)
}

// settledPaymentStatuses is THE canonical definition of "the money is ours".
//
// ONE DEFINITION, TWO ACCESSORS: IsSettledStatus answers the question for a
// single status in Go, SettledPaymentStatuses supplies the same set for SQL
// (`p.status = ANY($n::text[])`). Never restate this set as SQL literals: the
// subscription recovery selector and the admin recovery surface each used to
// spell out 'settlement' by hand, so a capture-status payment could be
// settled for the domain and invisible to recovery at the same time.
var settledPaymentStatuses = []string{PaymentStatusSettlement, PaymentStatusCapture}

// SettledPaymentStatuses returns a copy of the canonical settled status set so
// SQL consumers can express the same truth without restating it.
func SettledPaymentStatuses() []string {
	return append([]string(nil), settledPaymentStatuses...)
}

// IsSettledStatus reports whether a raw payment status counts as settled:
// the gateway money is in, and no reversal has happened (deny / cancel /
// expire / pending are all NOT settled).
func IsSettledStatus(status string) bool {
	for _, s := range settledPaymentStatuses {
		if status == s {
			return true
		}
	}
	return false
}

// canonicalWireStatus maps the payments-table status — the gateway vocabulary
// this system persists (settlement / capture / deny / cancel / expire / pending)
// — onto the canonical payment vocabulary exposed on the wire.
//
// The translation lives HERE, in the package that owns the payment state,
// because the status has exactly one authority. Every consumer that used to
// restate the gateway words (or, worse, silently degrade an unknown value to
// "pending") was a second source of truth for the same money question.
func canonicalWireStatus(raw string) (string, bool) {
	switch raw {
	case PaymentStatusPending:
		return "pending", true
	case PaymentStatusSettlement, PaymentStatusCapture:
		return "paid", true
	case PaymentStatusDeny:
		return "failed", true
	case PaymentStatusCancel:
		return WireStatusNoVerdict, false
	case PaymentStatusExpire:
		return "expired", true
	default:
		return "", false
	}
}

// CanonicalWireStatusPtr is canonicalWireStatus for the `payment_status` field
// carried alongside an order: it returns nil when there is no payment row or
// the persisted status is outside this contract, so the wire omits the field
// instead of inventing a money state the client would render as truth.
// WireStatusNoVerdict is the wire value for "this payment row carries no
// buyer-facing verdict" — no payment row, a cancelled/void row, or a status
// outside the canonical vocabulary. It is deliberately NOT "pending": silently
// degrading that into a money state is the lie this vocabulary exists to prevent.
//
// A CANCELLED payment row belongs here, not with the failures: in this system it
// is the shadow of an order-level cancellation, and reporting it as "failed"
// would blame the buyer for a payment nobody asked them to retry.
const WireStatusNoVerdict = ""

// CanonicalWireStatus is the plain-string form for untyped response maps
// (gin.H), where a key cannot simply be omitted.
func CanonicalWireStatus(raw string) string {
	canonical, ok := canonicalWireStatus(raw)
	if !ok {
		return WireStatusNoVerdict
	}
	return canonical
}

// CanonicalWireStatusPtr is canonicalWireStatus for the `payment_status` field
func CanonicalWireStatusPtr(raw *string) *string {
	if raw == nil {
		return nil
	}
	canonical, ok := canonicalWireStatus(*raw)
	if !ok {
		return nil
	}
	return &canonical
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
	var subscriptionDurationDays *int
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
		&subscriptionDurationDays,
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
	p.SubscriptionDurationDays = subscriptionDurationDays

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
