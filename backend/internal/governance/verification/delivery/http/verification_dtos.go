package http

import (
	"time"

	"github.com/google/uuid"
	verificationEntity "github.com/labuda/backend/internal/governance/verification/entity"
)

// ============================================================================
// Seller-facing request DTOs
// ============================================================================

// UploadURLRequest is the body for
// POST /api/v1/seller/verification/documents/upload-url.
//
// The mobile client requests a short-lived presigned S3 PUT URL before
// uploading a KYC document. The backend generates the storage_key (so the
// client never constructs bucket paths) and embeds credentials in the URL
// (so no AWS key ever reaches the mobile app).
type UploadURLRequest struct {
	// DocumentType must be identity_ktp or identity_selfie.
	DocumentType verificationEntity.DocumentType `json:"document_type" binding:"required"`
	// ContentType must be image/jpeg, image/png, or image/webp.
	ContentType string `json:"content_type" binding:"required"`
}

// UploadURLResponse is returned by the upload-url endpoint.
type UploadURLResponse struct {
	// StorageKey is the S3 object key the client must PUT to. Keep this and
	// pass it as ktp_storage_key / selfie_storage_key to SubmitKYC.
	StorageKey string `json:"storage_key"`
	// UploadURL is the presigned PUT URL. Expires in ~15 min.
	// The PUT request must include Content-Type matching the requested type.
	UploadURL string `json:"upload_url"`
	// ExpiresAt is the UTC expiry timestamp for the presigned URL.
	ExpiresAt time.Time `json:"expires_at"`
}

// SubmitKYCRequest is the body for POST /api/v1/seller/verification/submit.
//
// KYC scope (owner decision): individual identity only — KTP + selfie.
// Both documents must be uploaded to S3 via the upload-url endpoint before
// calling this endpoint. Pass the storage_key values returned by upload-url.
// No document URLs are accepted or stored — admin reads generate presigned
// GET URLs on demand.
type SubmitKYCRequest struct {
	FullName         string `json:"full_name" binding:"required,min=2,max=100"`
	NationalID       string `json:"national_id" binding:"required,min=10,max=20"`
	KTPStorageKey    string `json:"ktp_storage_key" binding:"required"`
	SelfieStorageKey string `json:"selfie_storage_key" binding:"required"`
}

// ============================================================================
// Seller-facing response DTOs
// ============================================================================

// SubmitKYCResponse is returned after a successful KYC submission.
// Both KTP + selfie documents are created atomically; the seller lifecycle
// is driven to pending_review in the same transaction.
type SubmitKYCResponse struct {
	SellerStatus verificationEntity.Status `json:"seller_status"`
	Message      string                    `json:"message"`
}

// VerificationStatusResponse is the canonical status payload returned by
// GET /api/v1/seller/verification/status. The top-level `status` field is
// the seller-lifecycle state (8 canonical values); `documents` is the
// uploaded evidence array.
type VerificationStatusResponse struct {
	SellerID    uuid.UUID                  `json:"seller_id"`
	Status      verificationEntity.Status  `json:"status"` // canonical lifecycle status
	SubmittedAt *time.Time                 `json:"submitted_at,omitempty"`
	ReviewedAt  *time.Time                 `json:"reviewed_at,omitempty"`
	Reason      *string                    `json:"reason,omitempty"`
	Documents   []VerificationDocumentItem `json:"documents"`
}

// VerificationDocumentItem is a single document summary line in the status
// response. We deliberately do NOT expose document_url to non-admin callers
// at this layer (the seller already knows the URL they uploaded; admin views
// can be added in a later phase).
type VerificationDocumentItem struct {
	ID            uuid.UUID                       `json:"id"`
	DocumentType  verificationEntity.DocumentType `json:"document_type"`
	DocumentName  string                          `json:"document_name"`
	Status        verificationEntity.ReviewStatus `json:"status"`
	RejectionNote *string                         `json:"rejection_note,omitempty"`
	SubmittedAt   time.Time                       `json:"submitted_at"`
	ReviewedAt    *time.Time                      `json:"reviewed_at,omitempty"`
}

// buildVerificationStatusResponse composes the status response from the
// canonical lifecycle row and the document evidence list. lifecycle may be
// nil for users with no Become-Seller history (treated as not_submitted);
// docs may be empty.
func buildVerificationStatusResponse(
	sellerID uuid.UUID,
	lifecycle *verificationEntity.SellerVerification,
	docs []*verificationEntity.VerificationDocument,
) VerificationStatusResponse {
	resp := VerificationStatusResponse{
		SellerID:  sellerID,
		Status:    verificationEntity.StatusNotSubmitted,
		Documents: []VerificationDocumentItem{},
	}
	if lifecycle != nil {
		resp.Status = lifecycle.Status
		resp.SubmittedAt = lifecycle.SubmittedAt
		resp.ReviewedAt = lifecycle.ReviewedAt
		resp.Reason = lifecycle.Reason
	}
	for _, d := range docs {
		resp.Documents = append(resp.Documents, VerificationDocumentItem{
			ID:            d.ID,
			DocumentType:  d.DocumentType,
			DocumentName:  d.DocumentName,
			Status:        d.Status,
			RejectionNote: d.RejectionNote,
			SubmittedAt:   d.SubmittedAt,
			ReviewedAt:    d.ReviewedAt,
		})
	}
	return resp
}

// ============================================================================
// Admin request / response DTOs
// ============================================================================

// AdminReviewDecisionRequest is the body for reject and request-resubmission
// routes. Reason is mandatory per doctrine
// (docs/flows/doctrine/verification-review-governance.md §"Mandatory Properties").
type AdminReviewDecisionRequest struct {
	Reason string `json:"reason" binding:"required,min=3,max=500"`
}

// AdminApproveRequest is the body for the approve route. Reason is optional
// per doctrine (recommended but not mandatory on approve).
type AdminApproveRequest struct {
	Reason string `json:"reason,omitempty"`
}

// AdminVerificationListItem is one entry in the admin review queue.
// SellerUsername is the seller's public display handle (from user_profiles).
// It is omitted when the seller has no profile row. Email, phone, and
// firebase_uid are never included.
type AdminVerificationListItem struct {
	ID             uuid.UUID                 `json:"id"`
	SellerID       uuid.UUID                 `json:"seller_id"`
	SellerUsername *string                   `json:"seller_username,omitempty"`
	SellerFarmName *string                   `json:"seller_farm_name,omitempty"`
	Status         verificationEntity.Status `json:"status"`
	SubmittedAt    *time.Time                `json:"submitted_at,omitempty"`
	CreatedAt      time.Time                 `json:"created_at"`
	UpdatedAt      time.Time                 `json:"updated_at"`
}

// AdminVerificationDocumentItem extends VerificationDocumentItem with a
// view_url field (short-lived presigned S3 GET URL), which is intentionally
// omitted from the seller-facing response but exposed to admin reviewers.
// No permanent document URL is stored — view_url is generated on demand.
type AdminVerificationDocumentItem struct {
	ID            uuid.UUID                       `json:"id"`
	DocumentType  verificationEntity.DocumentType `json:"document_type"`
	DocumentName  string                          `json:"document_name"`
	ViewURL       string                          `json:"view_url"`
	Status        verificationEntity.ReviewStatus `json:"status"`
	RejectionNote *string                         `json:"rejection_note,omitempty"`
	SubmittedAt   time.Time                       `json:"submitted_at"`
	ReviewedAt    *time.Time                      `json:"reviewed_at,omitempty"`
}

// AdminBankAccountInfo is a single registered payout destination shown in the
// admin verification detail so the reviewer can cross-check the declared
// identity against the bank account holder name.
type AdminBankAccountInfo struct {
	ID                string `json:"id"`
	BankName          string `json:"bank_name"`
	BankCode          string `json:"bank_code"`
	AccountNumber     string `json:"account_number"`
	AccountHolderName string `json:"account_holder_name"`
	IsDefault         bool   `json:"is_default"`
	// IsReviewedForPayout is true when this account ID was captured in
	// reviewed_bank_account_ids at KYC approval time.
	// false = added post-approval; GUARD 5 blocks withdrawal to this account
	// until a re-approval snapshots it.
	IsReviewedForPayout bool `json:"is_reviewed_for_payout"`
}

// AdminVerificationDetailResponse is returned by GET
// /api/v1/admin/seller-verifications/:seller_id. It mirrors
// VerificationStatusResponse but exposes view_url (presigned GET) to the
// reviewer and includes the seller's registered bank accounts for identity
// cross-check.
type AdminVerificationDetailResponse struct {
	SellerID       uuid.UUID                       `json:"seller_id"`
	SellerUsername *string                         `json:"seller_username,omitempty"`
	SellerFarmName *string                         `json:"seller_farm_name,omitempty"`
	Status         verificationEntity.Status       `json:"status"`
	SubmittedAt    *time.Time                      `json:"submitted_at,omitempty"`
	ReviewedAt     *time.Time                      `json:"reviewed_at,omitempty"`
	Reason         *string                         `json:"reason,omitempty"`
	Documents      []AdminVerificationDocumentItem `json:"documents"`
	BankAccounts   []AdminBankAccountInfo          `json:"bank_accounts"`
}

// DocumentViewURLResponse is returned by the admin document view-url endpoint.
// The view_url is a short-lived presigned S3 GET URL (default 5 min TTL).
type DocumentViewURLResponse struct {
	ViewURL   string    `json:"view_url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// buildAdminVerificationDetailResponseWithDocs composes the admin detail
// response when the caller has already resolved document URLs (e.g. presigned
// GET URLs). Use this variant from GetDetail where presigning happens in the
// handler before calling this function.
func buildAdminVerificationDetailResponseWithDocs(
	sellerID uuid.UUID,
	sellerUsername *string,
	sellerFarmName *string,
	lifecycle *verificationEntity.SellerVerification,
	docs []AdminVerificationDocumentItem,
	bankAccounts []AdminBankAccountInfo,
) AdminVerificationDetailResponse {
	if bankAccounts == nil {
		bankAccounts = []AdminBankAccountInfo{}
	}
	if docs == nil {
		docs = []AdminVerificationDocumentItem{}
	}
	resp := AdminVerificationDetailResponse{
		SellerID:       sellerID,
		SellerUsername: sellerUsername,
		SellerFarmName: sellerFarmName,
		Status:         verificationEntity.StatusNotSubmitted,
		Documents:      docs,
		BankAccounts:   bankAccounts,
	}
	if lifecycle != nil {
		resp.Status = lifecycle.Status
		resp.SubmittedAt = lifecycle.SubmittedAt
		resp.ReviewedAt = lifecycle.ReviewedAt
		resp.Reason = lifecycle.Reason
	}
	return resp
}

// buildAdminVerificationDetailResponse composes the admin detail response from
// the canonical lifecycle row, the document evidence list, and the seller's
// registered bank accounts. ViewURL is left empty here; use
// buildAdminVerificationDetailResponseWithDocs when presigned GET URLs are
// available (i.e. from GetDetail after presigning each document).
func buildAdminVerificationDetailResponse(
	sellerID uuid.UUID,
	sellerUsername *string,
	sellerFarmName *string,
	lifecycle *verificationEntity.SellerVerification,
	docs []*verificationEntity.VerificationDocument,
	bankAccounts []AdminBankAccountInfo,
) AdminVerificationDetailResponse {
	if bankAccounts == nil {
		bankAccounts = []AdminBankAccountInfo{}
	}
	resp := AdminVerificationDetailResponse{
		SellerID:       sellerID,
		SellerUsername: sellerUsername,
		SellerFarmName: sellerFarmName,
		Status:         verificationEntity.StatusNotSubmitted,
		Documents:      []AdminVerificationDocumentItem{},
		BankAccounts:   bankAccounts,
	}
	if lifecycle != nil {
		resp.Status = lifecycle.Status
		resp.SubmittedAt = lifecycle.SubmittedAt
		resp.ReviewedAt = lifecycle.ReviewedAt
		resp.Reason = lifecycle.Reason
	}
	for _, d := range docs {
		resp.Documents = append(resp.Documents, AdminVerificationDocumentItem{
			ID:            d.ID,
			DocumentType:  d.DocumentType,
			DocumentName:  d.DocumentName,
			ViewURL:       "", // presigned on demand; call GetDocumentViewURL endpoint
			Status:        d.Status,
			RejectionNote: d.RejectionNote,
			SubmittedAt:   d.SubmittedAt,
			ReviewedAt:    d.ReviewedAt,
		})
	}
	return resp
}


