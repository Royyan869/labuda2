// Package entity provides the dispute domain entity.
// This is the canonical dispute system (V1) with proper escrow integration.
package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DisputeStatus represents the state of a dispute.
type DisputeStatus string

const (
	// DisputeStatusUnderReview means the dispute is being reviewed.
	DisputeStatusUnderReview DisputeStatus = "under_review"
	// DisputeStatusResolvedRefund means the dispute was resolved with a refund to the buyer.
	DisputeStatusResolvedRefund DisputeStatus = "resolved_refund"
	// DisputeStatusResolvedRelease means the dispute was resolved with release to the seller.
	DisputeStatusResolvedRelease DisputeStatus = "resolved_release"
	// DisputeStatusResolvedPartial means the dispute was resolved with partial split (buyer gets item refund, seller gets shipping).
	DisputeStatusResolvedPartial DisputeStatus = "resolved_partial"
)

const (
	// DefaultDisputeTimeoutDays is the default number of days before auto-resolution.
	DefaultDisputeTimeoutDays = 14
	// DisputeOverdueThresholdDays is the number of days before marking a dispute as overdue.
	DisputeOverdueThresholdDays = 3
)

// Valid dispute reason codes for standardized categorization
const (
	// ReasonCodeItemNotReceived - Buyer didn't receive the item
	ReasonCodeItemNotReceived = "item_not_received"
	// ReasonCodeItemNotAsDescribed - Item doesn't match description
	ReasonCodeItemNotAsDescribed = "item_not_as_described"
	// ReasonCodeShippingDamage - Item was damaged during shipping
	ReasonCodeShippingDamage = "shipping_damage"
	// ReasonCodeBuyerNotResponding - Buyer not responding to seller
	ReasonCodeBuyerNotResponding = "buyer_not_responding"
	// ReasonCodeSellerNotShipping - Seller not shipping the item
	ReasonCodeSellerNotShipping = "seller_not_shipping"
	// ReasonCodePaymentIssue - Payment-related issues
	ReasonCodePaymentIssue = "payment_issue"
	// ReasonCodeOther - Other reasons
	ReasonCodeOther = "other"
)

// ValidReasonCodes is a map of valid reason codes
var ValidReasonCodes = map[string]bool{
	ReasonCodeItemNotReceived:    true,
	ReasonCodeItemNotAsDescribed: true,
	ReasonCodeShippingDamage:     true,
	ReasonCodeBuyerNotResponding: true,
	ReasonCodeSellerNotShipping:  true,
	ReasonCodePaymentIssue:       true,
	ReasonCodeOther:              true,
}

// BuyerReasonCodes are reason codes that can only be used by buyers
var BuyerReasonCodes = map[string]bool{
	ReasonCodeItemNotReceived:    true,
	ReasonCodeItemNotAsDescribed: true,
	ReasonCodeShippingDamage:     true,
	ReasonCodePaymentIssue:       true,
	ReasonCodeOther:              true,
}

// SellerReasonCodes are reason codes that can only be used by sellers
var SellerReasonCodes = map[string]bool{
	ReasonCodeBuyerNotResponding: true,
	ReasonCodeSellerNotShipping:  true,
	ReasonCodeOther:              true,
}

// Dispute represents an order dispute between buyer and seller.
// This is the V1 dispute entity with proper escrow integration and deadlock prevention.
type Dispute struct {
	ID       uuid.UUID
	OrderID  uuid.UUID
	BuyerID  uuid.UUID
	SellerID uuid.UUID

	Reason      string
	Description *string

	Status DisputeStatus

	OpenedAt   time.Time
	ResolvedAt *time.Time

	// ResolvedBy contains the admin user ID who resolved this dispute.
	// This provides auditability for financial dispute decisions.
	ResolvedBy *uuid.UUID

	// ResolutionNotes contains the admin's reasoning for the resolution decision.
	// This provides context for audit trail and dispute history.
	ResolutionNotes *string

	// =============================================================================
	// FAIRNESS & ABUSE PREVENTION FIELDS
	// =============================================================================

	// CallerID tracks who opened the dispute (buyer or seller).
	// This is used for abuse detection and fair policy enforcement.
	CallerID *uuid.UUID

	// ReasonCode is a standardized reason code for the dispute.
	// Required for all disputes to enable better analytics and policy enforcement.
	// Valid values: "item_not_received", "item_not_as_described", "shipping_damage",
	//               "buyer_not_responding", "seller_not_shipping", "payment_issue", "other"
	ReasonCode *string

	// EvidenceURLs contains URLs to evidence supporting the dispute claim.
	// For buyers: Required (video evidence)
	// For sellers: Optional but recommended for faster resolution
	EvidenceURLs []string

	// DEADLOCK PREVENTION FIELDS

	// TimeoutDays is the number of days after which the dispute is auto-resolved.
	// Default is DefaultDisputeTimeoutDays (14).
	TimeoutDays int

	// IsOverdue indicates if the dispute has exceeded the escalation threshold
	// and needs admin attention. Set to true after DisputeOverdueThresholdDays (3).
	IsOverdue bool

	// OverdueMarkedAt is the timestamp when the dispute was marked as overdue.
	OverdueMarkedAt *time.Time

	// AutoResolvedAt is the timestamp when the dispute was auto-resolved by the timeout worker.
	AutoResolvedAt *time.Time

	// AutoResolutionType stores the resolution type for auto-resolved disputes.
	// Either "release" (to seller) or "refund" (to buyer).
	AutoResolutionType *string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// InvalidTransitionError is returned when attempting an invalid dispute state transition.
type InvalidTransitionError struct {
	CurrentStatus DisputeStatus
	TargetStatus  DisputeStatus
}

func (e *InvalidTransitionError) Error() string {
	return fmt.Sprintf("invalid dispute status transition: %s -> %s", e.CurrentStatus, e.TargetStatus)
}

// ErrAlreadyResolved is returned when attempting to resolve an already resolved dispute.
type ErrAlreadyResolved struct {
	DisputeID     uuid.UUID
	CurrentStatus DisputeStatus
}

func (e *ErrAlreadyResolved) Error() string {
	return fmt.Sprintf("dispute already resolved: %s (status: %s)", e.DisputeID, e.CurrentStatus)
}

// ErrInvalidReasonCode is returned when an invalid reason code is provided.
type ErrInvalidReasonCode struct {
	ReasonCode string
	ValidCodes []string
}

func (e *ErrInvalidReasonCode) Error() string {
	return fmt.Sprintf("invalid reason code: %s (valid codes: %v)", e.ReasonCode, e.ValidCodes)
}

// ErrReasonCodeNotAuthorized is returned when a user tries to use a reason code they're not authorized for.
type ErrReasonCodeNotAuthorized struct {
	ReasonCode string
	UserType   string // "buyer" or "seller"
}

func (e *ErrReasonCodeNotAuthorized) Error() string {
	return fmt.Sprintf("reason code '%s' not authorized for %s", e.ReasonCode, e.UserType)
}

// ErrMissingReasonCode is returned when a dispute is created without a reason code.
type ErrMissingReasonCode struct{}

func (e *ErrMissingReasonCode) Error() string {
	return "reason code is required for all disputes"
}

// ErrInsufficientEvidence is returned when a dispute lacks sufficient evidence.
type ErrInsufficientEvidence struct {
	DisputeID uuid.UUID
	Reason    string
}

func (e *ErrInsufficientEvidence) Error() string {
	return fmt.Sprintf("insufficient evidence for dispute %s: %s", e.DisputeID, e.Reason)
}

// IsUnderReview returns true if the dispute is still under review.
func (d *Dispute) IsUnderReview() bool {
	return d.Status == DisputeStatusUnderReview
}

// IsResolved returns true if the dispute is in a terminal resolved state.
func (d *Dispute) IsResolved() bool {
	return d.Status == DisputeStatusResolvedRefund ||
		d.Status == DisputeStatusResolvedRelease ||
		d.Status == DisputeStatusResolvedPartial
}

// CanResolve returns true if the dispute can be resolved.
func (d *Dispute) CanResolve() bool {
	return d.Status == DisputeStatusUnderReview
}

// ResolveRefund transitions the dispute to resolved_refund status.
// This is called when the dispute is resolved in favor of the buyer.
// The adminID and notes are stored for audit trail purposes.
func (d *Dispute) ResolveRefund(now time.Time, adminID uuid.UUID, notes *string) error {
	if !d.CanResolve() {
		if d.IsResolved() {
			return &ErrAlreadyResolved{
				DisputeID:     d.ID,
				CurrentStatus: d.Status,
			}
		}
		return &InvalidTransitionError{
			CurrentStatus: d.Status,
			TargetStatus:  DisputeStatusResolvedRefund,
		}
	}

	d.Status = DisputeStatusResolvedRefund
	d.ResolvedAt = &now
	d.ResolvedBy = &adminID
	d.ResolutionNotes = notes
	d.UpdatedAt = now
	return nil
}

// ResolveRelease transitions the dispute to resolved_release status.
// This is called when the dispute is resolved in favor of the seller.
// The adminID and notes are stored for audit trail purposes.
func (d *Dispute) ResolveRelease(now time.Time, adminID uuid.UUID, notes *string) error {
	if !d.CanResolve() {
		if d.IsResolved() {
			return &ErrAlreadyResolved{
				DisputeID:     d.ID,
				CurrentStatus: d.Status,
			}
		}
		return &InvalidTransitionError{
			CurrentStatus: d.Status,
			TargetStatus:  DisputeStatusResolvedRelease,
		}
	}

	d.Status = DisputeStatusResolvedRelease
	d.ResolvedAt = &now
	d.ResolvedBy = &adminID
	d.ResolutionNotes = notes
	d.UpdatedAt = now
	return nil
}

// ResolvePartialSplit transitions the dispute to resolved_partial status.
// This is called when the dispute is resolved with partial split:
// - Buyer gets refund for item price (subtotal)
// - Seller gets release for shipping fee (shipping_total)
//
// The adminID and notes are stored for audit trail purposes.
func (d *Dispute) ResolvePartialSplit(now time.Time, adminID uuid.UUID, notes *string) error {
	if !d.CanResolve() {
		if d.IsResolved() {
			return &ErrAlreadyResolved{
				DisputeID:     d.ID,
				CurrentStatus: d.Status,
			}
		}
		return &InvalidTransitionError{
			CurrentStatus: d.Status,
			TargetStatus:  DisputeStatusResolvedPartial,
		}
	}

	d.Status = DisputeStatusResolvedPartial
	d.ResolvedAt = &now
	d.ResolvedBy = &adminID
	d.ResolutionNotes = notes
	d.UpdatedAt = now
	return nil
}

// NewDispute creates a new dispute for an order.
// The reason must be provided by the buyer, description is optional.
func NewDispute(
	orderID uuid.UUID,
	buyerID uuid.UUID,
	sellerID uuid.UUID,
	reason string,
	description *string,
) *Dispute {
	now := time.Now()
	return &Dispute{
		ID:                 uuid.New(),
		OrderID:            orderID,
		BuyerID:             buyerID,
		SellerID:           sellerID,
		Reason:             reason,
		Description:        description,
		Status:             DisputeStatusUnderReview,
		OpenedAt:           now,
		TimeoutDays:        DefaultDisputeTimeoutDays,
		IsOverdue:          false,
		OverdueMarkedAt:    nil,
		AutoResolvedAt:     nil,
		AutoResolutionType: nil,
		CallerID:           nil,
		ReasonCode:         nil,
		EvidenceURLs:       nil,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

// NewDisputeWithDetails creates a new dispute with enhanced details for fairness.
// This is the preferred constructor for new disputes.
//
// Parameters:
// - orderID: The order being disputed
// - buyerID: The buyer's user ID
// - sellerID: The seller's user ID
// - callerID: The user opening the dispute (buyer or seller)
// - reason: Free-text description of the dispute
// - description: Optional detailed description
// - reasonCode: Required standardized reason code
// - evidenceURLs: Optional list of evidence URLs
//
// Returns an error if validation fails (invalid reason code, unauthorized reason code, etc.)
func NewDisputeWithDetails(
	orderID uuid.UUID,
	buyerID uuid.UUID,
	sellerID uuid.UUID,
	callerID uuid.UUID,
	reason string,
	description *string,
	reasonCode string,
	evidenceURLs []string,
) (*Dispute, error) {
	now := time.Now()

	// Validate reason code
	if !ValidReasonCodes[reasonCode] {
		validCodes := make([]string, 0, len(ValidReasonCodes))
		for code := range ValidReasonCodes {
			validCodes = append(validCodes, code)
		}
		return nil, &ErrInvalidReasonCode{
			ReasonCode: reasonCode,
			ValidCodes: validCodes,
		}
	}

	// Validate reason code authorization based on caller type
	var callerType string
	if callerID == buyerID {
		callerType = "buyer"
		if !BuyerReasonCodes[reasonCode] {
			return nil, &ErrReasonCodeNotAuthorized{
				ReasonCode: reasonCode,
				UserType:   callerType,
			}
		}
	} else if callerID == sellerID {
		callerType = "seller"
		if !SellerReasonCodes[reasonCode] {
			return nil, &ErrReasonCodeNotAuthorized{
				ReasonCode: reasonCode,
				UserType:   callerType,
			}
		}
	}

	return &Dispute{
		ID:               uuid.New(),
		OrderID:          orderID,
		BuyerID:          buyerID,
		SellerID:         sellerID,
		Reason:           reason,
		Description:      description,
		Status:           DisputeStatusUnderReview,
		OpenedAt:         now,
		TimeoutDays:      DefaultDisputeTimeoutDays,
		IsOverdue:        false,
		OverdueMarkedAt:  nil,
		AutoResolvedAt:   nil,
		AutoResolutionType: nil,
		CallerID:           &callerID,
		ReasonCode:         &reasonCode,
		EvidenceURLs:       evidenceURLs,
		CreatedAt:          now,
		UpdatedAt:          now,
	}, nil
}

// =============================================================================
// DEADLOCK PREVENTION METHODS
// =============================================================================

// ShouldBeOverdue returns true if the dispute has been open longer than
// the overdue threshold and is still under review.
func (d *Dispute) ShouldBeOverdue() bool {
	if d.Status != DisputeStatusUnderReview {
		return false
	}
	if d.IsOverdue {
		return false // Already marked
	}
	return time.Since(d.OpenedAt) > time.Duration(DisputeOverdueThresholdDays)*24*time.Hour
}

// MarkAsOverdue marks the dispute as overdue for escalation.
func (d *Dispute) MarkAsOverdue(now time.Time) error {
	if d.IsOverdue {
		return fmt.Errorf("dispute already marked as overdue")
	}
	if d.Status != DisputeStatusUnderReview {
		return fmt.Errorf("cannot mark non-under_review dispute as overdue")
	}
	d.IsOverdue = true
	d.OverdueMarkedAt = &now
	d.UpdatedAt = now
	return nil
}

// ShouldAutoResolve returns true if the dispute has exceeded its timeout period.
func (d *Dispute) ShouldAutoResolve() bool {
	if d.Status != DisputeStatusUnderReview {
		return false
	}
	if d.AutoResolvedAt != nil {
		return false // Already auto-resolved
	}
	timeoutDuration := time.Duration(d.TimeoutDays) * 24 * time.Hour
	return time.Since(d.OpenedAt) > timeoutDuration
}



// DaysOpen returns the number of days the dispute has been open.
func (d *Dispute) DaysOpen() int {
	return int(time.Since(d.OpenedAt).Hours() / 24)
}

// IsNearTimeout returns true if the dispute is approaching its timeout
// (within 2 days of auto-resolution).
func (d *Dispute) IsNearTimeout() bool {
	if d.Status != DisputeStatusUnderReview {
		return false
	}
	timeoutDuration := time.Duration(d.TimeoutDays) * 24 * time.Hour
	elapsed := time.Since(d.OpenedAt)
	return elapsed > timeoutDuration-(2*24*time.Hour)
}

// =============================================================================
// FAIRNESS & ABUSE PREVENTION METHODS
// =============================================================================

// IsOpenedBySeller returns true if the dispute was opened by the seller.
func (d *Dispute) IsOpenedBySeller() bool {
	if d.CallerID == nil {
		return false
	}
	return *d.CallerID == d.SellerID
}

// IsOpenedByBuyer returns true if the dispute was opened by the buyer.
func (d *Dispute) IsOpenedByBuyer() bool {
	if d.CallerID == nil {
		return false
	}
	return *d.CallerID == d.BuyerID
}

// HasSufficientEvidence returns true if the dispute has sufficient evidence.
// For buyer disputes: Requires at least one evidence URL
// For seller disputes: Evidence is optional but recommended
func (d *Dispute) HasSufficientEvidence() bool {
	// Buyer disputes require evidence
	if d.IsOpenedByBuyer() {
		return len(d.EvidenceURLs) > 0
	}
	// Seller disputes: evidence is optional
	return true
}

// ValidateForSeller performs additional validation for seller disputes.
// Returns an error if the seller dispute doesn't meet requirements.
func (d *Dispute) ValidateForSeller() error {
	if !d.IsOpenedBySeller() {
		return nil // Not a seller dispute
	}

	// Seller disputes must have a valid reason code
	if d.ReasonCode == nil {
		return &ErrMissingReasonCode{}
	}

	// Seller disputes should have evidence for faster resolution
	// (optional but recommended)
	return nil
}

// ValidateForBuyer performs additional validation for buyer disputes.
// Returns an error if the buyer dispute doesn't meet requirements.
func (d *Dispute) ValidateForBuyer() error {
	if !d.IsOpenedByBuyer() {
		return nil // Not a buyer dispute
	}

	// Buyer disputes must have evidence (video required by policy)
	if !d.HasSufficientEvidence() {
		return &ErrInsufficientEvidence{
			DisputeID: d.ID,
			Reason:    "buyer disputes require evidence (video)",
		}
	}

	return nil
}


