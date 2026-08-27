package entity

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ExternalProduct is the reviewable content record for a promotion target.
type ExternalProduct struct {
	ID                    uuid.UUID
	OwnerUserID           uuid.UUID
	Title                 string
	Description           *string
	ExternalURL           string
	NormalizedExternalURL string
	ReviewStatus          ExternalProductReviewStatus
	RejectionReason       *string
	UnsafeURLFlag         bool
	SubmittedAt           *time.Time
	ApprovedAt            *time.Time
	RejectedAt            *time.Time
	HiddenAt              *time.Time
	LastReviewedBy        *uuid.UUID
	CreatedAt             time.Time
	UpdatedAt             time.Time
	DeletedAt             *time.Time
}

// ExternalProductUpdateInput carries owner-editable fields.
type ExternalProductUpdateInput struct {
	Title       *string
	Description *string
	ExternalURL *string
}

// NewExternalProductDraft creates a validated draft record.
func NewExternalProductDraft(
	ownerUserID uuid.UUID,
	title string,
	description *string,
	externalURL string,
	dbTime time.Time,
) (*ExternalProduct, error) {
	normalizedURL, err := ValidateExternalURL(externalURL)
	if err != nil {
		return nil, err
	}

	normalizedTitle, err := ValidateTitle(title)
	if err != nil {
		return nil, err
	}

	return &ExternalProduct{
		ID:                    uuid.New(),
		OwnerUserID:           ownerUserID,
		Title:                 normalizedTitle,
		Description:           description,
		ExternalURL:           strings.TrimSpace(externalURL),
		NormalizedExternalURL: normalizedURL,
		ReviewStatus:          ExternalProductReviewStatusDraft,
		UnsafeURLFlag:         false,
		CreatedAt:             dbTime,
		UpdatedAt:             dbTime,
	}, nil
}

// IsEditableByOwner returns true when the owner may edit the record directly.
func (p *ExternalProduct) IsEditableByOwner() bool {
	if p == nil || p.DeletedAt != nil {
		return false
	}
	return p.ReviewStatus.CanMaterialEdit()
}

// IsPubliclyEligibleReviewStatus returns true when the product is in an approved state.
func (p *ExternalProduct) IsPubliclyEligibleReviewStatus() bool {
	if p == nil {
		return false
	}
	return p.ReviewStatus.IsPubliclyEligibleReviewStatus()
}

// CanSubmit returns true when the owner may submit the product for review.
func (p *ExternalProduct) CanSubmit() bool {
	if p == nil {
		return false
	}
	return p.ReviewStatus.CanSubmit()
}

// CanResubmit returns true when the owner may resubmit after rejection.
func (p *ExternalProduct) CanResubmit() bool {
	if p == nil {
		return false
	}
	return p.ReviewStatus.CanResubmit()
}

// CanMaterialEdit returns true when the owner may make edits.
func (p *ExternalProduct) CanMaterialEdit() bool {
	if p == nil {
		return false
	}
	return p.ReviewStatus.CanMaterialEdit()
}

// Submit moves a draft record into pending review.
func (p *ExternalProduct) Submit(dbTime time.Time) error {
	if p == nil {
		return fmt.Errorf("external product is nil")
	}
	if !p.ReviewStatus.CanSubmit() {
		return fmt.Errorf("external product is not in a submittable state")
	}

	p.transitionToPendingReview(dbTime)
	p.ReviewStatus = ExternalProductReviewStatusPendingReview
	return nil
}

// Resubmit moves a rejected record back into pending review.
func (p *ExternalProduct) Resubmit(dbTime time.Time) error {
	if p == nil {
		return fmt.Errorf("external product is nil")
	}
	if !p.ReviewStatus.CanResubmit() {
		return fmt.Errorf("external product is not in a resubmittable state")
	}

	p.transitionToPendingReview(dbTime)
	p.ReviewStatus = ExternalProductReviewStatusPendingReview
	return nil
}

// ApplyOwnerUpdate applies editable fields and transitions approved content back to review.
func (p *ExternalProduct) ApplyOwnerUpdate(input ExternalProductUpdateInput, dbTime time.Time) error {
	if p == nil {
		return fmt.Errorf("external product is nil")
	}
	if !p.IsEditableByOwner() {
		return fmt.Errorf("external product is not editable in state %s", p.ReviewStatus)
	}

	if input.Title != nil {
		title, err := ValidateTitle(*input.Title)
		if err != nil {
			return err
		}
		p.Title = title
	}
	if input.Description != nil {
		p.Description = input.Description
	}
	if input.ExternalURL != nil {
		normalizedURL, err := ValidateExternalURL(*input.ExternalURL)
		if err != nil {
			return err
		}
		p.ExternalURL = strings.TrimSpace(*input.ExternalURL)
		p.NormalizedExternalURL = normalizedURL
	}

	if p.ReviewStatus == ExternalProductReviewStatusApproved {
		p.transitionToPendingReview(dbTime)
		p.ReviewStatus = ExternalProductReviewStatusPendingReview
	}

	p.UpdatedAt = dbTime
	return nil
}

// ApproveByAdmin approves a pending external product.
func (p *ExternalProduct) ApproveByAdmin(adminID uuid.UUID, dbTime time.Time) error {
	if p == nil {
		return fmt.Errorf("external product is nil")
	}
	if adminID == uuid.Nil {
		return fmt.Errorf("admin_id is required")
	}
	if !p.ReviewStatus.CanApprove() {
		return fmt.Errorf("external product is not approvable in state %s", p.ReviewStatus)
	}

	p.ReviewStatus = ExternalProductReviewStatusApproved
	p.ApprovedAt = &dbTime
	p.RejectedAt = nil
	p.HiddenAt = nil
	p.LastReviewedBy = &adminID
	p.RejectionReason = nil
	p.UpdatedAt = dbTime
	return nil
}

// RejectByAdmin rejects a pending external product with a reason.
func (p *ExternalProduct) RejectByAdmin(adminID uuid.UUID, reason string, dbTime time.Time) error {
	if p == nil {
		return fmt.Errorf("external product is nil")
	}
	if adminID == uuid.Nil {
		return fmt.Errorf("admin_id is required")
	}
	if !p.ReviewStatus.CanReject() {
		return fmt.Errorf("external product is not rejectable in state %s", p.ReviewStatus)
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return fmt.Errorf("reason is required")
	}

	p.ReviewStatus = ExternalProductReviewStatusRejected
	p.ApprovedAt = nil
	p.RejectedAt = &dbTime
	p.HiddenAt = nil
	p.LastReviewedBy = &adminID
	p.RejectionReason = &reason
	p.UpdatedAt = dbTime
	return nil
}

// RequestChangesByAdmin records a request-changes decision for a pending external product.
// Unlike RejectByAdmin, this sets a distinct ReviewStatus that allows the owner to resubmit
// while making it clear that admin review identified required improvements (not a hard rejection).
func (p *ExternalProduct) RequestChangesByAdmin(adminID uuid.UUID, reason string, dbTime time.Time) error {
	if p == nil {
		return fmt.Errorf("external product is nil")
	}
	if adminID == uuid.Nil {
		return fmt.Errorf("admin_id is required")
	}
	if !p.ReviewStatus.CanRequestChanges() {
		return fmt.Errorf("external product is not in a request-changeable state (current: %s)", p.ReviewStatus)
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return fmt.Errorf("reason is required for request_changes")
	}

	p.ReviewStatus = ExternalProductReviewStatusRequestChanges
	p.ApprovedAt = nil
	p.RejectedAt = &dbTime
	p.HiddenAt = nil
	p.LastReviewedBy = &adminID
	p.RejectionReason = &reason
	p.UpdatedAt = dbTime
	return nil
}

// HideByAdmin hides an external product after review.
func (p *ExternalProduct) HideByAdmin(adminID uuid.UUID, reason string, dbTime time.Time) error {
	if p == nil {
		return fmt.Errorf("external product is nil")
	}
	if adminID == uuid.Nil {
		return fmt.Errorf("admin_id is required")
	}
	if !p.ReviewStatus.CanHide() {
		return fmt.Errorf("external product is not hideable in state %s", p.ReviewStatus)
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return fmt.Errorf("reason is required")
	}

	p.ReviewStatus = ExternalProductReviewStatusHidden
	p.HiddenAt = &dbTime
	p.LastReviewedBy = &adminID
	p.RejectionReason = &reason
	p.UpdatedAt = dbTime
	return nil
}

func (p *ExternalProduct) transitionToPendingReview(dbTime time.Time) {
	p.SubmittedAt = &dbTime
	p.ApprovedAt = nil
	p.RejectedAt = nil
	p.HiddenAt = nil
	p.LastReviewedBy = nil
	p.RejectionReason = nil
	p.UpdatedAt = dbTime
}

// ValidateTitle validates and normalizes a public title.
func ValidateTitle(title string) (string, error) {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return "", fmt.Errorf("title is required")
	}
	if len([]rune(trimmed)) > 200 {
		return "", fmt.Errorf("title must be 200 characters or fewer")
	}
	return trimmed, nil
}

// ValidateExternalURL validates a basic http/https URL and returns a normalized value.
func ValidateExternalURL(raw string) (string, error) {
	normalized, err := NormalizeExternalURL(raw)
	if err != nil {
		return "", err
	}
	return normalized, nil
}

// NormalizeExternalURL trims and canonicalizes a URL for storage.
func NormalizeExternalURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("external_url is required")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid external_url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("external_url must use http or https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("external_url must include a host")
	}

	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""

	return parsed.String(), nil
}
