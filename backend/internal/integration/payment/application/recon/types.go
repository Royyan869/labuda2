// Package recon is the Phase 1A pure deterministic reconciliation classifier
// for the Gateway Payment Reconciliation Worker.
//
// CONSTITUTIONAL ROLE (RUNTIME-INVARIANTS §7.1 + ADR-002):
// This package is VERIFY-ESCALATE-OBSERVE only.
//
//   - Classify is a pure function.
//   - No DB writes.
//   - No gateway calls inside the classifier.
//   - No logging side-effects.
//   - No clock reads (Now is injected via Snapshot).
//   - No randomness, no map iteration without sorted keys.
//
// Caller responsibilities (worker layer, NOT this package):
//   - Resolve the snapshot from DB + gateway.
//   - Persist findings.
//   - Emit alerts / outbox observability events.
//   - Route operator action requests.
//
// Drift classes D1–D14 are derived from the approved forensic audit + owner
// decisions (2026-05-11). Severity values reflect owner overrides: D8 and D13
// upgraded to HIGH; D14 introduced as HIGH.
package recon

import (
	"time"

	"github.com/google/uuid"
)

// DriftClass enumerates the deterministic drift signatures the classifier
// recognises. Values are stable identifiers persisted into observation tables
// and emitted as outbox metadata; do not rename existing entries.
type DriftClass string

const (
	DriftD1GatewaySettledLocalUnpaid       DriftClass = "D1_gateway_settled_local_unpaid"
	DriftD2LocalPaidGatewayTerminalFailure DriftClass = "D2_local_paid_gateway_terminal_failure"
	DriftD3GatewayRefundedLocalHolding     DriftClass = "D3_gateway_refunded_local_holding"
	DriftD4PartialRefundMismatch           DriftClass = "D4_partial_refund_mismatch"
	DriftD5DuplicateSettlement             DriftClass = "D5_duplicate_settlement"
	DriftD6MissingWebhookDelivery          DriftClass = "D6_missing_webhook_delivery"
	DriftD7WebhookProcessedLedgerAbsent    DriftClass = "D7_webhook_processed_ledger_absent"
	DriftD8EscrowStateMismatch             DriftClass = "D8_escrow_state_mismatch"
	DriftD9RefundFullCoinsNotRefunded      DriftClass = "D9_refund_full_coins_not_refunded"
	DriftD10OrderCompletedReleaseAbsent    DriftClass = "D10_order_completed_release_absent"
	DriftD11StuckPendingRefund             DriftClass = "D11_stuck_pending_refund"
	DriftD12PendingPaymentPastExpiry       DriftClass = "D12_pending_payment_past_expiry"
	DriftD13ProjectionNoneEscrowExists     DriftClass = "D13_projection_none_escrow_exists"
	DriftD14LedgerEntryOutboxMissing       DriftClass = "D14_ledger_entry_outbox_missing"
	DriftD15EscrowPresentPaymentAbsent     DriftClass = "D15_escrow_present_payment_absent"
)

// Severity is the alert-routing band. Values match AlertService /
// EscalationService thresholds in internal/platform/alert.
//
// SeverityCriticalSecurity is a distinct routing class reserved for findings
// that imply gateway compromise, webhook forgery, or replay attack (currently
// D2 only). Operator runbook for this class diverges from operational
// CRITICAL — security oncall, not finance ops.
type Severity string

const (
	SeverityCritical         Severity = "critical"
	SeverityCriticalSecurity Severity = "critical_security"
	SeverityHigh             Severity = "high"
	SeverityMedium           Severity = "medium"
	SeverityLow              Severity = "low"
	SeverityInfo             Severity = "info"
)

// Gateway transaction_status values mirrored from Midtrans Core API.
const (
	GatewayStatusPending    = "pending"
	GatewayStatusSettlement = "settlement"
	GatewayStatusCapture    = "capture"
	GatewayStatusDeny       = "deny"
	GatewayStatusCancel     = "cancel"
	GatewayStatusExpire     = "expire"
	GatewayStatusChallenge  = "challenge"
)

// Local payment.status values (subset of payment_status_enum) the classifier
// reasons about by name. Other values pass through opaquely.
const (
	LocalPaymentStatusPending    = "pending"
	LocalPaymentStatusSettlement = "settlement"
	LocalPaymentStatusCapture    = "capture"
	LocalPaymentStatusPaid       = "paid"
	LocalPaymentStatusFailed     = "failed"
	LocalPaymentStatusCancelled  = "cancelled"
	LocalPaymentStatusExpire     = "expire"
)

// Local escrow.status canonical values (3-state model post wallet-hold
// demolition).
const (
	EscrowStatusHolding  = "holding"
	EscrowStatusReleased = "released"
	EscrowStatusRefunded = "refunded"
)

// orders.escrow_status projection values.
const (
	OrderEscrowStatusNone     = "none"
	OrderEscrowStatusHolding  = "holding"
	OrderEscrowStatusReleased = "released"
	OrderEscrowStatusRefunded = "refunded"
)

// Local order.status values the classifier names. Other values pass through.
const (
	OrderStatusPendingPayment = "pending_payment"
	OrderStatusPaid           = "paid"
	OrderStatusShipped        = "shipped"
	OrderStatusDelivered      = "delivered"
	OrderStatusCompleted      = "completed"
	OrderStatusCancelled      = "cancelled"
	OrderStatusRefunded       = "refunded"
	OrderStatusPartiallyRef   = "partially_refunded"
	OrderStatusDisputeOpen    = "dispute_open"
	OrderStatusExpired        = "expired"
)

// Refund gateway pipeline states (refund_gateway pipeline, migration 000129).
const (
	GatewayRefundStatusUnsubmitted = "unsubmitted"
	GatewayRefundStatusPending     = "pending"
	GatewayRefundStatusSucceeded   = "succeeded"
	GatewayRefundStatusFailed      = "failed"
)

// Gateway refund history entry status values.
const (
	GatewayRefundHistorySuccess = "success"
	GatewayRefundHistoryPending = "pending"
	GatewayRefundHistoryFailed  = "failed"
)

// payment_webhook_events.status values.
const (
	WebhookStatusPending        = "pending"
	WebhookStatusProcessing     = "processing"
	WebhookStatusSucceeded      = "succeeded"
	WebhookStatusFailed         = "failed"
	WebhookStatusOrphaned       = "orphaned"
	WebhookStatusManualReview   = "manual_review"
	WebhookStatusQuarantined    = "quarantined"
	WebhookStatusTerminalReview = "terminal_review"
)

// GatewaySnapshot is the Midtrans-side truth at QueriedAt. If Available is
// false the gateway query was skipped (circuit open, timeout, etc.) and the
// classifier suppresses any drift class that requires gateway data.
type GatewaySnapshot struct {
	MidtransOrderID         string
	Available               bool
	TransactionStatus       string
	StatusCode              string
	TransactionID           string
	PaymentType             string
	GrossAmount             int64
	TransactionTime         time.Time
	SettlementTime          *time.Time
	RefundChargebackHistory []GatewayRefundEntry
	QueriedAt               time.Time
}

// GatewayRefundEntry mirrors a single entry from Midtrans
// refund_chargeback_history. Status uses GatewayRefundHistory* constants.
type GatewayRefundEntry struct {
	RefundKey string // merchant idempotency key (gateway_idempotency_key)
	RefundID  string // gateway-assigned id (gateway_refund_id)
	Amount    int64
	Status    string
	CreatedAt time.Time
}

// PaymentRow is the canonical projection of one row from payments.
type PaymentRow struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	MidtransOrderID string
	TransactionID   string // empty when nullable column is NULL
	GrossAmount     int64
	Status          string
	ReferenceType   string // 'order' or 'billing'
	ReferenceID     uuid.UUID
	PaidAt          *time.Time
	ExpiredAt       time.Time
	CreatedAt       time.Time
}

// OrderRow is the canonical projection of one row from orders.
type OrderRow struct {
	ID           uuid.UUID
	Status       string
	EscrowStatus string // denormalized projection (orders.escrow_status)
	// GrossAmount is the canonical buyer-funded escrow base
	// (orders.total_before_coins_amount = PD + S). orders.escrow_amount is
	// NOT authoritative (never persisted).
	GrossAmount  int64
	HasDispute   bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// EscrowRow is the canonical projection of one row from escrows. Single row
// per order (UNIQUE(order_id)).
type EscrowRow struct {
	ID         uuid.UUID
	OrderID    uuid.UUID
	Status     string
	Amount     int64
	CreatedAt  time.Time
	ReleasedAt *time.Time
	RefundedAt *time.Time
}

// RefundRow is the canonical projection of one row from refunds. Multiple
// rows per order are allowed (sequential partials).
type RefundRow struct {
	ID                    uuid.UUID
	OrderID               uuid.UUID
	RequestedAmount       int64
	Status                string // refund_status_enum
	GatewayStatus         string // 'unsubmitted','pending','succeeded','failed'
	GatewayRefundID       string
	GatewayIdempotencyKey string
	GatewayRequestedAt    *time.Time
	GatewayAcknowledgedAt *time.Time
}

// WebhookEventRef is the classifier's lightweight view of a row from
// payment_webhook_events.
//
// Status is the row processing status (payment_webhook_status_enum), not the
// gateway's payload status. TransactionStatus + TransactionID are extracted
// from the JSONB payload at resolve time and feed lifecycle-aware D5
// duplicate detection. Empty strings mean the payload did not carry the
// field (older webhooks, malformed payloads).
type WebhookEventRef struct {
	EventID           string
	MidtransOrderID   string
	Status            string
	TransactionStatus string // payload->>'transaction_status'
	TransactionID     string // payload->>'transaction_id'
	ReceivedAt        time.Time
	ProcessedAt       *time.Time
}

// LedgerLookup carries the pre-resolved answers to "does the canonical ledger
// entry for this business event exist?". The caller resolves each boolean
// from financial_transactions by idempotency_key. The classifier MUST NOT
// know specific key formats — those belong to FinanceService.
type LedgerLookup struct {
	// Order-side
	BuyerSettlementExists bool // entry for payment settlement booked
	OrderReleaseExists    bool // entry for escrow release to seller booked
	OrderReleaseAmount    int64

	// Refund-side — keyed by refund.ID; absent map entries imply false.
	RefundReversalExistsByRefundID map[uuid.UUID]bool
}

// OutboxLookup carries the pre-resolved answers to "is the required outbox
// event row present AND alive?".
//
// Liveness semantics (owner-locked):
//   - status ∈ {pending, processing, succeeded}  → ALIVE  → true
//   - status = failed (with retries remaining)   → ALIVE  → true
//   - status = dead_letter                       → DEAD   → false
//   - row absent                                 → DEAD   → false
//
// "Failed with retries remaining" is operationally indistinguishable from
// in-flight delivery; only a dead-letter terminal state means the consumer
// will never receive the event. The resolver in the worker layer collapses
// these states into the boolean before invoking Classify.
type OutboxLookup struct {
	// Order-side
	MoneyReleasedAliveByOrderID map[uuid.UUID]bool

	// Refund-side
	MoneyRefundSucceededAliveByRefundID map[uuid.UUID]bool
	CoinsRefundRequiredAliveByOrderID   map[uuid.UUID]bool
}

// Thresholds carry injected timing knobs so the classifier remains a pure
// function. Zero values mean "do not apply this threshold gate".
type Thresholds struct {
	// PendingPaymentGrace is the minimum age a pending payment must reach
	// before D1 can fire. Guards against racing the in-flight webhook.
	PendingPaymentGrace time.Duration

	// OrphanRecoveryGrace is the window within which an orphaned webhook
	// event suppresses D6 (orphan_webhook_recovery_worker is still trying).
	OrphanRecoveryGrace time.Duration

	// StuckRefundGrace is the minimum age of a refund pinned at
	// gateway_status='pending' before D11 can fire.
	StuckRefundGrace time.Duration

	// PendingPaymentExpiryGrace is the minimum age past expired_at before
	// D12 can fire (gives the gateway/webhook a chance to deliver expire).
	PendingPaymentExpiryGrace time.Duration
}

// Snapshot bundles every input the classifier needs for one logical
// reconciliation target (typically one order or one refund). All time
// comparisons in the classifier are anchored to Snapshot.Now; the classifier
// MUST NOT call time.Now() itself.
type Snapshot struct {
	Now        time.Time
	Gateway    GatewaySnapshot
	Payment    *PaymentRow // nil if no local payment row resolved
	Order      *OrderRow   // nil if no local order row resolved
	Escrow     *EscrowRow  // nil if no escrow row exists for this order
	Refunds    []RefundRow
	Webhooks   []WebhookEventRef
	Ledger     LedgerLookup
	Outbox     OutboxLookup
	Thresholds Thresholds
}

// Finding is one structured drift signal emitted by the classifier. It is a
// pure data record: persisting it, alerting on it, and routing operator
// action are caller concerns.
//
// IdempotencyKey is deterministic and bucketed by UTC date so daily
// reoccurrences can be detected without alert spam — the worker upserts on
// this key and increments occurrences.
type Finding struct {
	DriftClass            DriftClass
	Severity              Severity
	OrderID               *uuid.UUID
	MidtransOrderID       string
	RefundID              *uuid.UUID
	SuggestedAction       string
	DetectedAt            time.Time
	IdempotencyKey        string
	GatewayObservedAmount int64
	LocalObservedAmount   int64
	Notes                 string
}


