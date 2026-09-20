// Package entity provides the refund domain entity.
// Refund = buyer <-> seller negotiation (before dispute escalation).
package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RefundStatus represents the state of a refund request.
type RefundStatus string

// CANONICAL REFUND DECISION MODEL (one axis, two questions):
//
//  1. RefundStatus is the REFUND DECISION axis. It records WHO decided and
//     WHETHER that decision is final. It never records money movement.
//  2. GatewayRefundStatus (+ RefundedAt / FinalRefundAmount) is the FINANCIAL
//     SETTLEMENT axis. It records whether the gateway actually reversed the
//     payment. Settlement never rewrites the decision.
//
// Decision authority (locked business truth):
//   - seller ACCEPT  = FINAL decision  -> seller_approved
//   - seller REJECT  = NOT final       -> seller_rejected (buyer may escalate)
//   - ADMIN decision = FINAL decision  -> admin_refunded (buyer wins)
//                                         admin_released (seller wins)
//   - platform/system triggered refund (no buyer negotiation, no admin
//     decision)                       -> system_refunded
const (
	// RefundStatusPendingSellerReview means buyer requested refund, awaiting seller response.
	RefundStatusPendingSellerReview RefundStatus = "pending_seller_review"
	// RefundStatusSellerApproved means the seller ACCEPTED the refund request.
	// This is a FINAL refund decision; gateway settlement is tracked separately.
	RefundStatusSellerApproved RefundStatus = "seller_approved"
	// RefundStatusSellerRejected means seller rejected the refund request.
	// NOT final: the buyer may still escalate to admin.
	RefundStatusSellerRejected RefundStatus = "seller_rejected"
	// RefundStatusEscalatedToAdmin means buyer escalated to dispute after seller rejection.
	// The admin is now the final decision authority for this refund request.
	RefundStatusEscalatedToAdmin RefundStatus = "escalated_to_admin"
	// RefundStatusAdminRefunded means the ADMIN finally decided the buyer wins
	// (escalated refund only). Gateway settlement is tracked separately.
	RefundStatusAdminRefunded RefundStatus = "admin_refunded"
	// RefundStatusAdminReleased means the ADMIN finally decided the seller wins
	// (escalated refund only). No money moves to the buyer.
	RefundStatusAdminReleased RefundStatus = "admin_released"
	// RefundStatusSystemRefunded means the PLATFORM itself initiated the refund
	// (payment expiry with escrow, buyer overdue cancel, ban refund, manual admin
	// refund, or a buyer-wins dispute that had no refund request). There is no
	// buyer/seller negotiation and no admin refund decision to record.
	RefundStatusSystemRefunded RefundStatus = "system_refunded"
)

// GatewayRefundStatus tracks the asynchronous Midtrans refund pipeline.
//
// This is a SECONDARY state machine that runs in parallel with RefundStatus.
// RefundStatus represents the buyer/seller negotiation; GatewayRefundStatus
// represents the platform's conversation with the payment gateway about
// physically reversing the original payment. Phase 1 builds this pipeline
// without yet tying it back into RefundStatus / ledger / escrow / order.
type GatewayRefundStatus string

const (
	// GatewayRefundUnsubmitted means the refund row exists but the gateway
	// has not yet been called. Default state for newly-created refunds.
	GatewayRefundUnsubmitted GatewayRefundStatus = "unsubmitted"
	// GatewayRefundPending means the gateway accepted the refund request
	// (Midtrans HTTP 200) but we are awaiting the asynchronous webhook
	// acknowledgement that the funds actually reversed.
	GatewayRefundPending GatewayRefundStatus = "pending"
	// GatewayRefundSucceeded means the gateway webhook confirmed the refund
	// landed at the buyer's payment instrument.
	GatewayRefundSucceeded GatewayRefundStatus = "succeeded"
	// GatewayRefundFailed means either the gateway rejected the refund
	// synchronously (HTTP error) or the async webhook reported failure.
	GatewayRefundFailed GatewayRefundStatus = "failed"
)

// ============================================================================
// CANONICAL REFUND STATUS SETS — SQL MIRROR OF BlocksOrderRelease
// ============================================================================
//
// Every SQL predicate that asks "does a refund block order release?" MUST
// compose these lists (they are the SQL projection of the status sets used by
// Refund.BlocksOrderRelease / Refund.OwesBuyerRefund). Never spell the statuses
// out again: the two queries that ask this question live in different packages
// (refund repository + order auto-complete query) and previously drifted.
const (
	// RefundOwesBuyerStatusList lists the final decisions where the buyer
	// receives money (settlement tracked separately by gateway_status).
	RefundOwesBuyerStatusList = `('seller_approved','admin_refunded','system_refunded')`
	// RefundDecisionOpenStatusList lists decisions that are not final yet.
	RefundDecisionOpenStatusList = `('pending_seller_review','seller_rejected','escalated_to_admin')`
)

// IsValid reports whether the gateway refund status is a known value.
func (s GatewayRefundStatus) IsValid() bool {
	switch s {
	case GatewayRefundUnsubmitted, GatewayRefundPending,
		GatewayRefundSucceeded, GatewayRefundFailed:
		return true
	}
	return false
}

// IsTerminal reports whether the gateway refund is in a final state.
func (s GatewayRefundStatus) IsTerminal() bool {
	return s == GatewayRefundSucceeded || s == GatewayRefundFailed
}

// ErrInvalidGatewayTransition is returned for illegal sub-state transitions.
type ErrInvalidGatewayTransition struct {
	From GatewayRefundStatus
	To   GatewayRefundStatus
}

func (e *ErrInvalidGatewayTransition) Error() string {
	return fmt.Sprintf("invalid gateway refund status transition: %s -> %s", e.From, e.To)
}

// RefundReason represents the reason for refund request.
type RefundReason string

const (
	RefundReasonItemNotReceived    RefundReason = "item_not_received"
	RefundReasonItemNotAsDescribed RefundReason = "item_not_as_described"
	RefundReasonItemDamaged        RefundReason = "item_damaged"
	RefundReasonDefectiveItem      RefundReason = "defective_item"
	RefundReasonWrongItem          RefundReason = "wrong_item"
	RefundReasonChangeOfMind       RefundReason = "change_of_mind"
	RefundReasonDeliveryDelay      RefundReason = "delivery_delay"
	RefundReasonOther              RefundReason = "other"
	// RefundReasonGatewayCapturedAfterOrderInvalid is REC-6: gateway reported
	// success (settlement/capture) for a payment whose order entered a terminal
	// state (expired, cancelled, etc.) that prevents normal payment finalization.
	// Platform-initiated refund of the full gateway-captured amount.
	RefundReasonGatewayCapturedAfterOrderInvalid RefundReason = "gateway_captured_after_order_invalid"
)

// Refund represents a buyer-initiated refund request.
// This is the buyer <-> seller negotiation phase before dispute escalation.
type Refund struct {
	ID            uuid.UUID
	OrderID       uuid.UUID
	BuyerID       uuid.UUID
	SellerID      uuid.UUID

	Reason        RefundReason
	Description   *string
	EvidenceURLs  []string // Optional evidence attachments

	Status        RefundStatus

	// Requested amount (buyer's claim), in Rupiah integer — Labuda's
	// canonical money unit (PASS_18J). No cents/sen subunit.
	RequestedAmount int64

	// Seller decision fields
	SellerApprovedPercent *int   // Percentage seller agrees to refund (0-100)
	SellerApprovedAmount  *int64 // Actual amount seller agrees to refund
	SellerNotes           *string
	SellerReviewedAt      *time.Time

	// Admin decision fields (after escalation)
	AdminApprovedPercent *int
	AdminApprovedAmount  *int64
	AdminNotes           *string
	ReviewedBy           uuid.UUID // Admin user ID
	AdminReviewedAt      *time.Time

	// Final outcome
	FinalRefundAmount *int64

	// CANONICAL REFUND COMPONENTS (S2C2):
	// Split refund into product and shipping for cumulative math.
	// Commission is seller-side, coin restoration is product-proportional.
	RefundedProductAmount  *int64 // Rpd
	RefundedShippingAmount *int64 // Rs
	CoinsRefundedAmount    *int64 // Coin delta this event

	OpenedAt  time.Time
	ApprovedAt *time.Time
	RejectedAt *time.Time
	RefundedAt *time.Time

	// Gateway-aware refund pipeline (Phase 1: orchestration + ack only).
	//
	// These fields capture the platform's async conversation with Midtrans
	// about reversing the original payment. They run IN PARALLEL with the
	// fields above, which still represent the buyer/seller negotiation.
	//
	// Phase 1 invariant: mutating these fields does NOT mutate ledger,
	// seller payable, escrow, or order status.
	GatewayRefundID        *string
	GatewayStatus          GatewayRefundStatus
	GatewayAttempts        int
	LastGatewayError       *string
	GatewayIdempotencyKey  *string
	GatewayRequestedAt     *time.Time
	GatewayAcknowledgedAt  *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

// InvalidTransitionError is returned when attempting an invalid refund state transition.
type InvalidTransitionError struct {
	CurrentStatus RefundStatus
	TargetStatus  RefundStatus
}

func (e *InvalidTransitionError) Error() string {
	return fmt.Sprintf("invalid refund status transition: %s -> %s", e.CurrentStatus, e.TargetStatus)
}

// ErrAlreadyResolved is returned when attempting to modify an already resolved refund.
type ErrAlreadyResolved struct {
	RefundID     uuid.UUID
	CurrentStatus RefundStatus
}

func (e *ErrAlreadyResolved) Error() string {
	return fmt.Sprintf("refund already resolved: %s (status: %s)", e.RefundID, e.CurrentStatus)
}

// IsPending returns true if the refund is awaiting seller review.
func (r *Refund) IsPending() bool {
	return r.Status == RefundStatusPendingSellerReview
}

// IsRejected returns true if the refund was rejected by seller.
func (r *Refund) IsRejected() bool {
	return r.Status == RefundStatusSellerRejected
}

// IsEscalated returns true if the refund was escalated to admin (dispute).
func (r *Refund) IsEscalated() bool {
	return r.Status == RefundStatusEscalatedToAdmin
}

// IsDecisionFinal reports whether the refund DECISION is final — i.e. the
// decision authority (seller via ACCEPT, admin via an escalated decision, or
// the platform itself for system-initiated refunds) has decided and no further
// refund decision can be made on this request.
//
// This is a decision-axis question only. It says NOTHING about whether the
// money has moved; see IsSettlementPending.
func (r *Refund) IsDecisionFinal() bool {
	switch r.Status {
	case RefundStatusSellerApproved,
		RefundStatusAdminRefunded,
		RefundStatusAdminReleased,
		RefundStatusSystemRefunded:
		return true
	}
	return false
}

// AwaitsDecision reports whether the refund decision is still open (no final
// decision has been recorded yet).
func (r *Refund) AwaitsDecision() bool {
	return !r.IsDecisionFinal()
}

// OwesBuyerRefund reports whether the final decision is that the buyer receives
// money back. admin_released (seller wins) does NOT owe the buyer anything.
func (r *Refund) OwesBuyerRefund() bool {
	switch r.Status {
	case RefundStatusSellerApproved,
		RefundStatusAdminRefunded,
		RefundStatusSystemRefunded:
		return true
	}
	return false
}

// IsSettlementPending reports whether money owed to the buyer has not yet been
// settled at the gateway (still unsubmitted/in-flight, or failed and awaiting
// an operator retry). Releasing escrow to the seller while this is true would
// pay the same money twice, so it must fail closed.
func (r *Refund) IsSettlementPending() bool {
	if !r.OwesBuyerRefund() {
		return false
	}
	return r.GatewayStatus != GatewayRefundSucceeded
}

// BlocksOrderRelease is THE canonical predicate for "this refund must be
// respected before the order lifecycle may release money to the seller".
//
// It is consumed by:
//   - the order completion guard (buyer acceptance + auto-complete worker),
//   - the auto-complete candidate query (SQL mirror of this predicate),
//   - the order read path's has_active_refund / CTA gating.
//
// refundWindowOpen is owned by the ORDER domain (Order.IsRefundWindowOpen): only
// the order lifecycle knows whether the buyer's refund/escalation opportunity is
// still inside its own window. Once that window closes, an undecided refund no
// longer blocks the order — the normal lifecycle owns the outcome (locked
// business truth: seller reject + no escalation may end via normal completion).
func (r *Refund) BlocksOrderRelease(refundWindowOpen bool) bool {
	if r.IsSettlementPending() {
		return true
	}
	return r.AwaitsDecision() && refundWindowOpen
}

// CanEscalate returns true if buyer can escalate to dispute.
func (r *Refund) CanEscalate() bool {
	return r.Status == RefundStatusSellerRejected
}

// SellerApprove transitions the refund to seller_approved status.
// This is called when the seller agrees to refund the buyer.
// approvedAmount is the amount the seller agrees to refund (may equal RequestedAmount).
func (r *Refund) SellerApprove(approvedAmount int64, notes *string, now time.Time) error {
	if r.Status != RefundStatusPendingSellerReview {
		if r.IsDecisionFinal() {
			return &ErrAlreadyResolved{
				RefundID:      r.ID,
				CurrentStatus: r.Status,
			}
		}
		return &InvalidTransitionError{
			CurrentStatus: r.Status,
			TargetStatus:  RefundStatusSellerApproved,
		}
	}

	r.Status = RefundStatusSellerApproved
	r.SellerApprovedAmount = &approvedAmount
	if r.RequestedAmount > 0 {
		pct := int(approvedAmount * 100 / r.RequestedAmount)
		r.SellerApprovedPercent = &pct
	}
	r.SellerNotes = notes
	r.SellerReviewedAt = &now
	r.ApprovedAt = &now
	r.UpdatedAt = now
	return nil
}

// SellerReject transitions the refund to seller_rejected status.
// This is called when the seller declines the buyer's refund request.
func (r *Refund) SellerReject(notes *string, now time.Time) error {
	if r.Status != RefundStatusPendingSellerReview {
		if r.IsDecisionFinal() {
			return &ErrAlreadyResolved{
				RefundID:      r.ID,
				CurrentStatus: r.Status,
			}
		}
		return &InvalidTransitionError{
			CurrentStatus: r.Status,
			TargetStatus:  RefundStatusSellerRejected,
		}
	}

	r.Status = RefundStatusSellerRejected
	r.SellerNotes = notes
	r.SellerReviewedAt = &now
	r.RejectedAt = &now
	r.UpdatedAt = now
	return nil
}

// EscalateToAdmin transitions the refund to escalated_to_admin status.
// This is called when buyer opens a dispute after seller rejection.
func (r *Refund) EscalateToAdmin(now time.Time) error {
	if !r.CanEscalate() {
		if r.IsDecisionFinal() {
			return &ErrAlreadyResolved{
				RefundID:     r.ID,
				CurrentStatus: r.Status,
			}
		}
		return &InvalidTransitionError{
			CurrentStatus: r.Status,
			TargetStatus:  RefundStatusEscalatedToAdmin,
		}
	}

	r.Status = RefundStatusEscalatedToAdmin
	r.UpdatedAt = now
	return nil
}

// AdminRefund records the ADMIN's final decision that the buyer wins an
// escalated refund. Decision only — the gateway settlement that actually moves
// the money happens on this same refund row via InitiateGatewayRefund.
func (r *Refund) AdminRefund(adminID uuid.UUID, approvedAmount int64, notes *string, now time.Time) error {
	if r.Status != RefundStatusEscalatedToAdmin {
		return &InvalidTransitionError{
			CurrentStatus: r.Status,
			TargetStatus:  RefundStatusAdminRefunded,
		}
	}

	r.Status = RefundStatusAdminRefunded
	r.AdminApprovedAmount = &approvedAmount
	r.AdminNotes = notes
	r.ReviewedBy = adminID
	r.AdminReviewedAt = &now
	// Provisional decision amount. The gateway acknowledgement overwrites this
	// with the authoritative computed cash refund once settlement lands.
	r.FinalRefundAmount = &approvedAmount
	r.UpdatedAt = now
	return nil
}

// AdminRelease records the ADMIN's final decision that the seller wins an
// escalated refund. This is the FINAL decision for the refund process; the
// order lifecycle (escrow release + completion) is driven separately by the
// dispute resolution path in the same transaction.
func (r *Refund) AdminRelease(adminID uuid.UUID, notes *string, now time.Time) error {
	if r.Status != RefundStatusEscalatedToAdmin {
		return &InvalidTransitionError{
			CurrentStatus: r.Status,
			TargetStatus:  RefundStatusAdminReleased,
		}
	}

	r.Status = RefundStatusAdminReleased
	r.AdminNotes = notes
	r.ReviewedBy = adminID
	r.AdminReviewedAt = &now
	// No refund to buyer - amount is 0
	zeroAmount := int64(0)
	r.FinalRefundAmount = &zeroAmount
	r.UpdatedAt = now
	return nil
}

// NewRefund creates a new refund request.
func NewRefund(
	orderID uuid.UUID,
	buyerID uuid.UUID,
	sellerID uuid.UUID,
	reason RefundReason,
	description *string,
	requestedAmount int64,
) *Refund {
	now := time.Now()
	return &Refund{
		ID:              uuid.New(),
		OrderID:         orderID,
		BuyerID:         buyerID,
		SellerID:        sellerID,
		Reason:          reason,
		Description:     description,
		EvidenceURLs:    []string{},
		Status:          RefundStatusPendingSellerReview,
		RequestedAmount: requestedAmount,
		OpenedAt:        now,
		CreatedAt:       now,
		UpdatedAt:       now,
		GatewayStatus:   GatewayRefundUnsubmitted,
		GatewayAttempts: 0,
	}
}

// NewSystemRefund creates a refund initiated by the platform itself (timeout
// cancellation, expire-with-escrow, manual admin refund, ban refund, or a
// buyer-wins dispute that had no refund request).
//
// The row is created directly in system_refunded state: there is no
// buyer/seller negotiation phase and no admin refund decision to record, the
// platform itself has authority to settle.
//
// ATTRIBUTION: reviewerID is recorded as refunds.reviewed_by only when a human
// admin made the decision. Automatic platform refunds have no human reviewer
// and MUST pass uuid.Nil, which persists as NULL. auth.SystemCallerID is an
// authorization/audit sentinel and must never be stored as a users.id.
//
// Caller is responsible for synchronously dispatching the gateway refund
// (RefundService.InitiateGatewayRefund) in the same tx so the refund row
// transitions to gateway_status=pending.
func NewSystemRefund(
	orderID uuid.UUID,
	buyerID uuid.UUID,
	sellerID uuid.UUID,
	reviewerID uuid.UUID,
	reason RefundReason,
	productAmount int64,
	shippingAmount int64,
	description *string,
) *Refund {
	now := time.Now()
	cashRefund := productAmount + shippingAmount
	return &Refund{
		ID:                uuid.New(),
		OrderID:           orderID,
		BuyerID:           buyerID,
		SellerID:          sellerID,
		Reason:            reason,
		Description:       description,
		EvidenceURLs:      []string{},
		Status:            RefundStatusSystemRefunded,
		RequestedAmount:   cashRefund,
		AdminApprovedAmount: &cashRefund,
		ReviewedBy:        reviewerID,
		AdminReviewedAt:   &now,
		FinalRefundAmount: &cashRefund,
			RefundedProductAmount: &productAmount,
			RefundedShippingAmount: &shippingAmount,
		OpenedAt:          now,
		CreatedAt:         now,
		UpdatedAt:         now,
		GatewayStatus:     GatewayRefundUnsubmitted,
		GatewayAttempts:   0,
	}
}

// MarkGatewayDispatched records that we successfully called the gateway
// refund API. Transitions gateway_status from unsubmitted/failed -> pending,
// stores the idempotency key and gateway-side refund id, and bumps attempts.
//
// Phase 1: this MUST NOT touch RefundStatus, ledger, escrow, or order state.
func (r *Refund) MarkGatewayDispatched(idempotencyKey string, gatewayRefundID *string, now time.Time) error {
	if r.GatewayStatus == GatewayRefundSucceeded {
		return &ErrInvalidGatewayTransition{From: r.GatewayStatus, To: GatewayRefundPending}
	}
	r.GatewayStatus = GatewayRefundPending
	r.GatewayAttempts++
	r.GatewayIdempotencyKey = &idempotencyKey
	if gatewayRefundID != nil && *gatewayRefundID != "" {
		r.GatewayRefundID = gatewayRefundID
	}
	r.GatewayRequestedAt = &now
	r.LastGatewayError = nil
	r.UpdatedAt = now
	return nil
}

// MarkGatewayRequestFailed records a synchronous gateway dispatch failure.
// Transitions gateway_status to failed and stores the error string. Does
// not advance any other state. Caller may retry via MarkGatewayDispatched.
//
// Phase 1: this MUST NOT touch RefundStatus, ledger, escrow, or order state.
func (r *Refund) MarkGatewayRequestFailed(errMsg string, now time.Time) error {
	if r.GatewayStatus == GatewayRefundSucceeded {
		return &ErrInvalidGatewayTransition{From: r.GatewayStatus, To: GatewayRefundFailed}
	}
	r.GatewayStatus = GatewayRefundFailed
	r.GatewayAttempts++
	r.LastGatewayError = &errMsg
	r.UpdatedAt = now
	return nil
}

// MarkGatewayAckSucceeded records a successful refund webhook acknowledgement.
// Idempotent: a second call when already succeeded returns nil and is a no-op.
//
// Phase 1: this MUST NOT touch RefundStatus, ledger, escrow, or order state.
// The actual financial reversal happens in a later phase.
func (r *Refund) MarkGatewayAckSucceeded(gatewayRefundID string, now time.Time) error {
	if r.GatewayStatus == GatewayRefundSucceeded {
		return nil
	}
	if r.GatewayStatus != GatewayRefundPending && r.GatewayStatus != GatewayRefundFailed {
		return &ErrInvalidGatewayTransition{From: r.GatewayStatus, To: GatewayRefundSucceeded}
	}
	r.GatewayStatus = GatewayRefundSucceeded
	if gatewayRefundID != "" {
		r.GatewayRefundID = &gatewayRefundID
	}
	r.GatewayAcknowledgedAt = &now
	r.LastGatewayError = nil
	r.UpdatedAt = now
	return nil
}

// MarkGatewayAckFailed records an asynchronous refund failure webhook.
// Idempotent: a second call when already failed updates only the error msg.
// Refuses to overwrite an already-succeeded gateway refund.
//
// Phase 1: this MUST NOT touch RefundStatus, ledger, escrow, or order state.
func (r *Refund) MarkGatewayAckFailed(errMsg string, now time.Time) error {
	if r.GatewayStatus == GatewayRefundSucceeded {
		return &ErrInvalidGatewayTransition{From: r.GatewayStatus, To: GatewayRefundFailed}
	}
	r.GatewayStatus = GatewayRefundFailed
	r.LastGatewayError = &errMsg
	r.GatewayAcknowledgedAt = &now
	r.UpdatedAt = now
	return nil
}


