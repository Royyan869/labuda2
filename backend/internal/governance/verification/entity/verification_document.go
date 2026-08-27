package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DocumentType represents the type of verification document.
//
// KYC scope (owner decision): individual identity only.
//   - identity_ktp: Indonesian KTP/e-KTP photo.
//   - identity_selfie: Selfie or selfie-with-KTP (proof-of-person).
//
// Business documents (NPWP/SIUP/NIB/business_other) are NOT part of launch
// KYC and have been removed. They are not stored in the DB enum either
// (see migration 000205_kyc_minimal_scope.up.sql).
type DocumentType string

const (
	// DocumentTypeIdentityKTP is the Indonesian national identity card photo.
	DocumentTypeIdentityKTP DocumentType = "identity_ktp"
	// DocumentTypeIdentitySelfie is a selfie / selfie-with-KTP proving the
	// submitter is the same person as the KTP holder.
	DocumentTypeIdentitySelfie DocumentType = "identity_selfie"
)

// AllowedKYCDocumentTypes is the canonical set of document types accepted
// at the submission endpoint. Any type outside this set is rejected.
var AllowedKYCDocumentTypes = map[DocumentType]bool{
	DocumentTypeIdentityKTP:    true,
	DocumentTypeIdentitySelfie: true,
}

// ReviewStatus represents the review status of a verification document.
type ReviewStatus string

const (
	// ReviewStatusNotSubmitted means no verification has been submitted.
	ReviewStatusNotSubmitted ReviewStatus = "not_submitted"
	// ReviewStatusPending means verification is under review.
	ReviewStatusPending ReviewStatus = "pending"
	// ReviewStatusApproved means verification is approved.
	ReviewStatusApproved ReviewStatus = "approved"
	// ReviewStatusRejected means verification was rejected.
	ReviewStatusRejected ReviewStatus = "rejected"
)

// VerificationDocument represents an uploaded verification document.
//
// KYC PII note: StorageKey is the raw S3 object key
// (e.g. kyc/{userID}/identity_ktp/{ts}.jpg). Documents live in a private S3
// bucket. Admin reads generate short-lived presigned GET URLs from StorageKey;
// no permanent public URL is stored or exposed (migration 000206 dropped
// the legacy document_url column).
type VerificationDocument struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	DocumentType  DocumentType
	StorageKey    string // S3 object key; admin reads generate presigned GET URLs
	DocumentName  string
	Status        ReviewStatus
	RejectionNote *string
	SubmittedAt   time.Time
	ReviewedAt    *time.Time
	ReviewedBy    *uuid.UUID
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// NewVerificationDocument creates a new verification document.
// storageKey is the raw S3 object key; document_url is stored as "" because
// KYC docs live in a private S3 bucket — admin reads use presigned GET URLs
// generated on demand from the storage_key (see AdminVerificationHandler).
func NewVerificationDocument(
	userID uuid.UUID,
	docType DocumentType,
	storageKey string,
	documentName string,
) *VerificationDocument {
	now := time.Now()
	return &VerificationDocument{
		ID:           uuid.New(),
		UserID:       userID,
		DocumentType: docType,
		StorageKey:   storageKey,
		DocumentName: documentName,
		Status:       ReviewStatusPending,
		SubmittedAt:  now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// AlreadyVerifiedError is returned when attempting to approve a document
// that has already been approved (document-side terminal state).
type AlreadyVerifiedError struct {
	SellerID uuid.UUID
}

func (e *AlreadyVerifiedError) Error() string {
	return fmt.Sprintf("verification: document for user %s is already approved", e.SellerID)
}

// Approve marks the document as approved.
func (d *VerificationDocument) Approve(adminID uuid.UUID) error {
	if d.Status == ReviewStatusApproved {
		return &AlreadyVerifiedError{SellerID: d.UserID}
	}
	now := time.Now()
	d.Status = ReviewStatusApproved
	d.ReviewedAt = &now
	d.ReviewedBy = &adminID
	d.RejectionNote = nil
	d.UpdatedAt = now
	return nil
}

// Reject marks the document as rejected.
func (d *VerificationDocument) Reject(adminID uuid.UUID, reason string) error {
	now := time.Now()
	d.Status = ReviewStatusRejected
	d.ReviewedAt = &now
	d.ReviewedBy = &adminID
	d.RejectionNote = &reason
	d.UpdatedAt = now
	return nil
}

// IsPending returns true if the document is pending review.
func (d *VerificationDocument) IsPending() bool {
	return d.Status == ReviewStatusPending
}

// IsApproved returns true if the document is approved.
func (d *VerificationDocument) IsApproved() bool {
	return d.Status == ReviewStatusApproved
}

// IsIdentityDocument returns true if this is an identity document.
func (d *VerificationDocument) IsIdentityDocument() bool {
	return d.DocumentType == DocumentTypeIdentityKTP || d.DocumentType == DocumentTypeIdentitySelfie
}

// InvalidDocumentTypeError is returned for invalid document types.
type InvalidDocumentTypeError struct {
	DocumentType DocumentType
}

func (e *InvalidDocumentTypeError) Error() string {
	return fmt.Sprintf("verification: invalid document type: %s", e.DocumentType)
}

// DocumentSummary is a simplified document representation for status responses.
type DocumentSummary struct {
	ID           uuid.UUID    `json:"id"`
	DocumentType DocumentType `json:"document_type"`
	Status       ReviewStatus `json:"status"`
	DocumentName string       `json:"document_name"`
	SubmittedAt  string       `json:"submitted_at"`
}

// ToSummary converts a VerificationDocument to DocumentSummary.
func (d *VerificationDocument) ToSummary() DocumentSummary {
	return DocumentSummary{
		ID:           d.ID,
		DocumentType: d.DocumentType,
		Status:       d.Status,
		DocumentName: d.DocumentName,
		SubmittedAt:  d.SubmittedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}


