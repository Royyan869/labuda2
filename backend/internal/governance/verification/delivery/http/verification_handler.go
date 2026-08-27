// Package http exposes the canonical seller-facing verification surface.
//
// Endpoints (mounted under /api/v1/seller in routes_core.go):
//   - POST /api/v1/seller/verification/submit
//   - GET  /api/v1/seller/verification/status
//
// Both require the user to already hold seller authority (the seller
// route group already gates that via RequireSellerMiddleware). Submitting
// verification opens the *payout* authority sub-gate, not selling — see
// docs/flows/doctrine/seller-authority-separation.md.
package http

import (
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	verificationApp "github.com/labuda/backend/internal/governance/verification/application"
	verificationEntity "github.com/labuda/backend/internal/governance/verification/entity"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/response"
)

// S3Presigner generates short-lived presigned S3 URLs without exposing AWS
// credentials to the caller. Implemented by the s3PresignerAdapter in serverboot.
type S3Presigner interface {
	// PresignPUT returns a URL the client can PUT to with Content-Type: contentType.
	PresignPUT(key, contentType string, ttl time.Duration) (string, error)
	// PresignGET returns a short-lived read URL for a private S3 object.
	PresignGET(key string, ttl time.Duration) (string, error)
}

// KYCUploadTTL is the lifetime of a KYC presigned PUT URL.
// 15 minutes gives the client ample time to upload after requesting the URL.
const KYCUploadTTL = 15 * time.Minute

// allowedKYCContentTypes is the set of MIME types accepted for KYC uploads.
var allowedKYCContentTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

// VerificationHandler is the seller-facing handler for verification lifecycle.
// The admin-side handler lives in admin_verification_handler.go alongside.
type VerificationHandler struct {
	docService    *verificationApp.VerificationDocumentService
	sellerService *verificationApp.VerificationService
	log           *zap.Logger

	// Optional presigner — wired via SetPresigner after construction so that
	// existing tests remain unaffected. When nil, the upload-url endpoint
	// returns 503 (not configured).
	presigner S3Presigner
}

// SetPresigner wires the S3 presigner dependency.
// Call this in serverboot after NewVerificationHandler.
func (h *VerificationHandler) SetPresigner(p S3Presigner) {
	h.presigner = p
}

// NewVerificationHandler creates the seller-facing verification handler.
func NewVerificationHandler(
	docService *verificationApp.VerificationDocumentService,
	sellerService *verificationApp.VerificationService,
	log *zap.Logger,
) *VerificationHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &VerificationHandler{
		docService:    docService,
		sellerService: sellerService,
		log:           log,
	}
}

// RequestUploadURL handles POST /api/v1/seller/verification/documents/upload-url.
//
// Issues a short-lived presigned S3 PUT URL so the mobile client can upload a
// KYC document without holding any AWS credentials. The backend owns the
// storage_key namespace (kyc/{userID}/{docType}/{ts}_{ext}).
//
// The client must:
//  1. POST here to get {storage_key, upload_url}.
//  2. PUT the file bytes to upload_url with Content-Type matching the requested type.
//  3. Pass storage_key to SubmitKYC as ktp_storage_key / selfie_storage_key.
//
// Returns:
//   - 200: storage_key + upload_url + expires_at
//   - 400: invalid document_type or content_type
//   - 503: presigner not configured
func (h *VerificationHandler) RequestUploadURL(c *gin.Context) {
	ctx := c.Request.Context()
	userID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	if h.presigner == nil {
		h.log.Error("verification: upload-url: presigner not configured")
		response.Error(c, 503, "PRESIGNER_NOT_CONFIGURED", "Upload URL service not configured")
		return
	}

	var req UploadURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	// Validate document type.
	if !verificationEntity.AllowedKYCDocumentTypes[req.DocumentType] {
		response.Error(c, 400, "INVALID_DOCUMENT_TYPE",
			"document_type must be identity_ktp or identity_selfie")
		return
	}

	// Validate content type.
	ext, ok := allowedKYCContentTypes[req.ContentType]
	if !ok {
		response.Error(c, 400, "INVALID_CONTENT_TYPE",
			"content_type must be image/jpeg, image/png, or image/webp")
		return
	}

	// Build storage key: kyc/{userID}/{docType}/{timestamp}{ext}
	// Example: kyc/550e8400-e29b-41d4-a716-446655440000/identity_ktp/1749600000000.jpg
	_ = ctx // reserved for future DB-side checks
	ts := time.Now().UnixMilli()
	storageKey := fmt.Sprintf("kyc/%s/%s/%d%s", userID, string(req.DocumentType), ts, ext)

	uploadURL, err := h.presigner.PresignPUT(storageKey, req.ContentType, KYCUploadTTL)
	if err != nil {
		h.log.Error("verification: upload-url: presign failed",
			zap.String("user_id", userID.String()),
			zap.String("doc_type", string(req.DocumentType)),
			zap.Error(err))
		response.InternalServerError(c, "Failed to generate upload URL")
		return
	}

	expiresAt := time.Now().Add(KYCUploadTTL)
	response.Success(c, UploadURLResponse{
		StorageKey: storageKey,
		UploadURL:  uploadURL,
		ExpiresAt:  expiresAt,
	})
}

// SubmitKYC handles POST /api/v1/seller/verification/submit.
//
// Atomically persists both the KTP and selfie documents AND flips
// seller_verifications.status to pending_review (via
// VerificationDocumentService.SubmitKYCDocuments, which delegates to
// the entity state machine). Emits seller.verification.submitted inside
// the same tx.
//
// Both S3 uploads must be completed via the upload-url endpoint before
// calling this. Pass the storage_key values returned by upload-url.
// No document URLs are accepted — admin reads use presigned GET URLs.
//
// Returns:
//   - 201: KTP + selfie persisted, seller in pending_review
//   - 400: invalid body / missing fields
//   - 401: not authenticated
//   - 409: lifecycle state forbids new submission (pending_review / approved /
//          suspended / revoked / under_investigation), or KTP already pending/approved
//   - 500: server error
func (h *VerificationHandler) SubmitKYC(c *gin.Context) {
	ctx := c.Request.Context()
	userID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	var req SubmitKYCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	err := h.docService.SubmitKYCDocuments(
		ctx, userID,
		req.FullName, req.NationalID,
		req.KTPStorageKey,
		req.SelfieStorageKey,
	)
	if err != nil {
		h.handleSubmitError(c, userID, err)
		return
	}

	c.JSON(201, gin.H{
		"success": true,
		"data": SubmitKYCResponse{
			SellerStatus: verificationEntity.StatusPendingReview,
			Message:      "KYC documents submitted; awaiting admin review.",
		},
	})
}

// GetStatus handles GET /api/v1/seller/verification/status.
//
// Canonical: seller_verifications.status is the authoritative status. The
// returned object also includes a documents summary array (uploaded
// evidence) so the mobile UI can render both the lifecycle state and the
// list of submitted artefacts in one call.
//
// Flat user booleans (users.is_id_verified / is_farm_verified) are
// deliberately NOT consulted here — those are residual legacy fields that
// no authority gate reads.
func (h *VerificationHandler) GetStatus(c *gin.Context) {
	ctx := c.Request.Context()
	userID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Canonical lifecycle row (one per seller).
	lifecycle, err := h.sellerService.GetVerification(ctx, userID)
	if err != nil {
		h.log.Error("verification status: get lifecycle failed",
			zap.String("user_id", userID.String()), zap.Error(err))
		response.InternalServerError(c, "Failed to load verification status")
		return
	}

	// Documents (evidence list). Errors here are non-fatal: the caller still
	// gets the canonical lifecycle row.
	docs, err := h.docService.ListDocuments(ctx, userID)
	if err != nil {
		h.log.Warn("verification status: list documents failed",
			zap.String("user_id", userID.String()), zap.Error(err))
		docs = nil
	}

	resp := buildVerificationStatusResponse(userID, lifecycle, docs)
	response.Success(c, resp)
}

// handleSubmitError maps service-layer errors from a submit path to the
// canonical HTTP status codes. The entity state machine surfaces
// InvalidTransitionError when a seller tries to submit from a forbidden
// source state (pending_review / approved / suspended / revoked /
// under_investigation); that maps to 409.
func (h *VerificationHandler) handleSubmitError(c *gin.Context, userID uuid.UUID, err error) {
	var invalidTransition *verificationEntity.InvalidTransitionError
	if errors.As(err, &invalidTransition) {
		response.Error(c, 409, "INVALID_STATE",
			"Cannot submit verification from current status: "+string(invalidTransition.CurrentStatus))
		return
	}

	// Document-side duplicate (pending / approved doc of same type) is
	// surfaced as a plain wrapped error from the document service. Detect by
	// substring on the well-known message rather than introducing a new
	// typed error in this phase.
	msg := err.Error()
	if containsAny(msg, "verification: identity verification already") {
		response.Error(c, 409, "ALREADY_SUBMITTED", msg)
		return
	}

	h.log.Error("verification submit failed",
		zap.String("user_id", userID.String()), zap.Error(err))
	response.InternalServerError(c, "Failed to submit verification")
}

func containsAny(s string, parts ...string) bool {
	for _, p := range parts {
		if len(p) > 0 && len(s) >= len(p) {
			for i := 0; i+len(p) <= len(s); i++ {
				if s[i:i+len(p)] == p {
					return true
				}
			}
		}
	}
	return false
}


