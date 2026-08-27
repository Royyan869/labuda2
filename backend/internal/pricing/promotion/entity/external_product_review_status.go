package entity

// ExternalProductReviewStatus represents the moderation lifecycle of an external product.
type ExternalProductReviewStatus string

const (
	ExternalProductReviewStatusDraft          ExternalProductReviewStatus = "draft"
	ExternalProductReviewStatusPendingReview  ExternalProductReviewStatus = "pending_review"
	ExternalProductReviewStatusApproved       ExternalProductReviewStatus = "approved"
	ExternalProductReviewStatusRejected       ExternalProductReviewStatus = "rejected"
	ExternalProductReviewStatusRequestChanges ExternalProductReviewStatus = "request_changes"
	ExternalProductReviewStatusHidden         ExternalProductReviewStatus = "hidden"
)

// IsValid returns true if the review status is canonical.
func (s ExternalProductReviewStatus) IsValid() bool {
	switch s {
	case ExternalProductReviewStatusDraft,
		ExternalProductReviewStatusPendingReview,
		ExternalProductReviewStatusApproved,
		ExternalProductReviewStatusRejected,
		ExternalProductReviewStatusRequestChanges,
		ExternalProductReviewStatusHidden:
		return true
	default:
		return false
	}
}

// IsPubliclyEligibleReviewStatus returns true when the product may appear publicly after activation.
func (s ExternalProductReviewStatus) IsPubliclyEligibleReviewStatus() bool {
	return s == ExternalProductReviewStatusApproved
}

// CanSubmit returns true if an owner may submit this state for review.
func (s ExternalProductReviewStatus) CanSubmit() bool {
	return s == ExternalProductReviewStatusDraft
}

// CanResubmit returns true if an owner may resubmit after rejection or a request-changes decision.
func (s ExternalProductReviewStatus) CanResubmit() bool {
	return s == ExternalProductReviewStatusRejected || s == ExternalProductReviewStatusRequestChanges
}

// CanMaterialEdit returns true if an owner may edit the content without admin intervention.
func (s ExternalProductReviewStatus) CanMaterialEdit() bool {
	switch s {
	case ExternalProductReviewStatusDraft,
		ExternalProductReviewStatusRejected,
		ExternalProductReviewStatusRequestChanges,
		ExternalProductReviewStatusApproved:
		return true
	default:
		return false
	}
}

// CanRequestChanges returns true when an admin may issue a request-changes decision.
func (s ExternalProductReviewStatus) CanRequestChanges() bool {
	return s == ExternalProductReviewStatusPendingReview
}

// CanApprove returns true when an admin may approve the current state.
func (s ExternalProductReviewStatus) CanApprove() bool {
	return s == ExternalProductReviewStatusPendingReview
}

// CanReject returns true when an admin may reject the current state.
func (s ExternalProductReviewStatus) CanReject() bool {
	return s == ExternalProductReviewStatusPendingReview
}

// CanHide returns true when an admin may hide the current state.
func (s ExternalProductReviewStatus) CanHide() bool {
	switch s {
	case ExternalProductReviewStatusPendingReview,
		ExternalProductReviewStatusApproved,
		ExternalProductReviewStatusRejected,
		ExternalProductReviewStatusRequestChanges:
		return true
	default:
		return false
	}
}

// String returns the string representation.
func (s ExternalProductReviewStatus) String() string {
	return string(s)
}
