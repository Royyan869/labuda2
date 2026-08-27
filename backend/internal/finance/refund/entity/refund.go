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

const (
	// RefundStatusPendingSellerReview means buyer requested refund, awaiting seller response.
	RefundStatusPendingSellerReview RefundStatus = "pending_seller_review"
	// RefundStatusSellerApproved means seller approved the refund request.
	RefundStatusSellerApproved RefundStatus = "seller_approved"
	// RefundStatusSellerRejected means seller rejected the refund request.
	RefundStatusSellerRejected RefundStatus = "seller_rejected"
	// RefundStatusEscalatedToAdmin means buyer escalated to dispute after seller rejection.
	RefundStatusEscalatedToAdmin RefundStatus = "escalated_to_admin"
	// RefundStatusAdminRefunded means admin refunded to buyer (from escalated dispute).
	RefundStatusAdminRefunded RefundStatus = "admin_refunded"
	// RefundStatusAdminReleased means admin released to seller (from escalated dispute).
	RefundStatusAdminReleased RefundStatus = "admin_released"
	// RefundStatusRefunded means funds were returned to buyer.
	RefundStatusRefunded RefundStatus = "refunded"
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

// IsRefunded returns true if the refund was completed.
func (r *Refund) IsRefunded() bool {
	return r.Status == RefundStatusRefunded
}

// IsTerminal returns true if the refund is in a terminal state.
func (r *Refund) IsTerminal() bool {
	return r.Status == RefundStatusRefunded ||
		r.Status == RefundStatusAdminRefunded ||
		r.Status == RefundStatusAdminReleased
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
		if r.IsTerminal() {
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
		if r.IsTerminal() {
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
		if r.IsTerminal() {
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

// AdminRelease transitions the refund to admin_released status.
// This is called when dispute is resolved in favor of seller.
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

// CompleteRefund transitions the refund to refunded status.
// This is called after the ledger transaction completes.
func (r *Refund) CompleteRefund(refundAmount int64, now time.Time) error {
	if r.Status != RefundStatusSellerApproved && r.Status != RefundStatusAdminRefunded {
		return &InvalidTransitionError{
			CurrentStatus: r.Status,
			TargetStatus:  RefundStatusRefunded,
		}
	}

	r.Status = RefundStatusRefunded
	r.FinalRefundAmount = &refundAmount
	r.RefundedAt = &now
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

// NewSystemRefund creates a refund initiated by the platform itself (dispute
// resolution, timeout cancellation, expire-with-escrow, manual admin refund).
// The refund row is created directly in admin_refunded state — there is no
// buyer/seller negotiation phase for system-initiated refunds, the platform
// has authority to settle.
//
// Caller is responsible for synchronously dispatching the gateway refund
// (RefundService.InitiateGatewayRefund) in the same tx so the refund row
// transitions to gateway_status=pending.
func NewSystemRefund(
	orderID uuid.UUID,
	buyerID uuid.UUID,
	sellerID uuid.UUID,
	adminID uuid.UUID,
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
		Status:            RefundStatusAdminRefunded,
		RequestedAmount:   cashRefund,
		AdminApprovedAmount: &cashRefund,
		ReviewedBy:        adminID,
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


