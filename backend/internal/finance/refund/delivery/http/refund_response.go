// Package http provides HTTP response DTOs for refund operations.
package http

import (
	"time"

	"github.com/google/uuid"
)

// RefundResponse represents the JSON response for a refund.
// All field names use snake_case for consistent API naming.
type RefundResponse struct {
	ID         uuid.UUID `json:"id"`
	OrderID    uuid.UUID `json:"order_id"`
	BuyerID    uuid.UUID `json:"buyer_id"`
	SellerID   uuid.UUID `json:"seller_id"`

	Reason        string   `json:"reason"`
	Description   *string  `json:"description,omitempty"`
	EvidenceURLs  []string `json:"evidence_urls,omitempty"`

	Status string `json:"status"`

	// Amount fields (in Rupiah, full unit - no cents for IDR)
	RequestedAmount     *int64 `json:"requested_amount,omitempty"`
	SellerApprovedAmount *int64 `json:"seller_approved_amount,omitempty"`
	AdminApprovedAmount  *int64 `json:"admin_approved_amount,omitempty"`
	FinalRefundAmount   *int64 `json:"final_refund_amount,omitempty"`

	// Percentage fields (0-100)
	SellerApprovedPercent *int `json:"seller_approved_percent,omitempty"`
	AdminApprovedPercent  *int `json:"admin_approved_percent,omitempty"`

	// Decision metadata
	SellerNotes      *string    `json:"seller_notes,omitempty"`
	SellerReviewedAt *time.Time `json:"seller_reviewed_at,omitempty"`
	AdminNotes       *string    `json:"admin_notes,omitempty"`
	ReviewedBy       *uuid.UUID `json:"reviewed_by,omitempty"`
	AdminReviewedAt  *time.Time `json:"admin_reviewed_at,omitempty"`

	// Timestamps
	OpenedAt  time.Time  `json:"opened_at"`
	ApprovedAt *time.Time `json:"approved_at,omitempty"`
	RejectedAt *time.Time `json:"rejected_at,omitempty"`
	RefundedAt *time.Time `json:"refunded_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// RefundListResponse represents a paginated list of refunds.
type RefundListResponse struct {
	Data     []*RefundResponse `json:"data"`
	Total    *int              `json:"total,omitempty"`
	Page     *int              `json:"page,omitempty"`
	PageSize *int              `json:"page_size,omitempty"`
}

// NewRefundResponse creates a RefundResponse from a Refund entity.
func NewRefundResponse(
	id uuid.UUID,
	orderID uuid.UUID,
	buyerID uuid.UUID,
	sellerID uuid.UUID,
	reason string,
	description *string,
	evidenceURLs []string,
	status string,
	requestedAmount *int64,
	sellerApprovedAmount *int64,
	adminApprovedAmount *int64,
	finalRefundAmount *int64,
	sellerApprovedPercent *int,
	adminApprovedPercent *int,
	sellerNotes *string,
	sellerReviewedAt *time.Time,
	adminNotes *string,
	reviewedBy *uuid.UUID,
	adminReviewedAt *time.Time,
	openedAt time.Time,
	approvedAt *time.Time,
	rejectedAt *time.Time,
	refundedAt *time.Time,
	createdAt time.Time,
	updatedAt time.Time,
) *RefundResponse {
	return &RefundResponse{
		ID:                   id,
		OrderID:              orderID,
		BuyerID:              buyerID,
		SellerID:             sellerID,
		Reason:               reason,
		Description:          description,
		EvidenceURLs:         evidenceURLs,
		Status:               status,
		RequestedAmount:      requestedAmount,
		SellerApprovedAmount: sellerApprovedAmount,
		AdminApprovedAmount:  adminApprovedAmount,
		FinalRefundAmount:    finalRefundAmount,
		SellerApprovedPercent: sellerApprovedPercent,
		AdminApprovedPercent:  adminApprovedPercent,
		SellerNotes:          sellerNotes,
		SellerReviewedAt:     sellerReviewedAt,
		AdminNotes:           adminNotes,
		ReviewedBy:           reviewedBy,
		AdminReviewedAt:      adminReviewedAt,
		OpenedAt:             openedAt,
		ApprovedAt:           approvedAt,
		RejectedAt:           rejectedAt,
		RefundedAt:           refundedAt,
		CreatedAt:            createdAt,
		UpdatedAt:            updatedAt,
	}
}


