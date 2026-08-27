// Admin-side handler for the seller verification lifecycle.
//
// Endpoints (mounted under /api/v1/admin in routes_core.go and gated by the
// CapSellerVerificationReview capability):
//   - GET  /api/v1/admin/seller-verifications/pending
//   - GET  /api/v1/admin/seller-verifications/:seller_id
//   - POST /api/v1/admin/seller-verifications/:seller_id/approve
//   - POST /api/v1/admin/seller-verifications/:seller_id/reject
//   - POST /api/v1/admin/seller-verifications/:seller_id/request-resubmission
//   - POST /api/v1/admin/seller-verifications/:seller_id/suspend
//   - POST /api/v1/admin/seller-verifications/:seller_id/revoke
//   - POST /api/v1/admin/seller-verifications/:seller_id/investigate
//   - POST /api/v1/admin/seller-verifications/:seller_id/restore
//   - POST /api/v1/admin/seller-verifications/:seller_id/bank-accounts/:bank_account_id/mark-reviewed
//   - GET  /api/v1/admin/seller-verifications/:seller_id/documents/:document_id/view-url
package http

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	bankaccountEntity "github.com/labuda/backend/internal/finance/bankaccount/entity"
	verificationApp "github.com/labuda/backend/internal/governance/verification/application"
	verificationEntity "github.com/labuda/backend/internal/governance/verification/entity"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
)

// AdminDocViewTTL is the lifetime of a KYC document presigned GET URL.
// 5 minutes is short enough to limit exposure if the URL leaks.
const AdminDocViewTTL = 5 * time.Minute

// BankAccountsReader is the minimal interface the admin verification handler
// needs to fetch payout destinations for display alongside KYC documents.
// The concrete implementation is *bankaccountrepo.BankAccountRepository.
type BankAccountsReader interface {
	ListActiveAccountsBySeller(ctx context.Context, tx db.Tx, sellerID uuid.UUID) ([]*bankaccountEntity.BankAccount, error)
}

// AdminVerificationHandler is the admin-facing handler for the seller
// verification lifecycle. Capability enforcement is delegated to the route
// group middleware (RequireCapability("seller.verification.review")).
type AdminVerificationHandler struct {
	sellerService *verificationApp.VerificationService
	docService    *verificationApp.VerificationDocumentService
	log           *zap.Logger

	// Optional — wired via SetBankAccountReader after construction so that
	// existing tests remain unaffected. When nil, bank_accounts is returned
	// as an empty array rather than causing a 500.
	bankAcctDB     db.Transactor
	bankAcctReader BankAccountsReader

	// Optional — wired via SetPresigner after construction. When nil,
	// view_url in GetDetail responses is empty string; GetDocumentViewURL
	// returns 503. Wire via SetPresigner in serverboot.
	presigner S3Presigner
}

// SetBankAccountReader wires the optional bank account reader dependency.
// Call this in serverboot after NewAdminVerificationHandler.
func (h *AdminVerificationHandler) SetBankAccountReader(transactor db.Transactor, reader BankAccountsReader) {
	h.bankAcctDB = transactor
	h.bankAcctReader = reader
}

// SetPresigner wires the S3 presigner dependency.
// Call this in serverboot after NewAdminVerificationHandler.
func (h *AdminVerificationHandler) SetPresigner(p S3Presigner) {
	h.presigner = p
}

// NewAdminVerificationHandler creates the admin-facing verification handler.
func NewAdminVerificationHandler(
	sellerService *verificationApp.VerificationService,
	docService *verificationApp.VerificationDocumentService,
	log *zap.Logger,
) *AdminVerificationHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &AdminVerificationHandler{
		sellerService: sellerService,
		docService:    docService,
		log:           log,
	}
}

// validAdminListStatuses is the set of statuses accepted by the ?status= query
// parameter on the list endpoint. not_submitted is excluded because those are
// placeholder rows the admin cannot meaningfully review.
var validAdminListStatuses = map[verificationEntity.Status]bool{
	verificationEntity.StatusPendingReview:      true,
	verificationEntity.StatusApproved:           true,
	verificationEntity.StatusNeedsResubmission:  true,
	verificationEntity.StatusRejected:           true,
	verificationEntity.StatusSuspended:          true,
	verificationEntity.StatusRevoked:            true,
	verificationEntity.StatusUnderInvestigation: true,
}

// ListPending handles GET /api/v1/admin/seller-verifications/pending.
//
// Accepts an optional ?status= query parameter to filter by any canonical
// verification status. When absent, defaults to pending_review (preserving
// backward-compatible behavior). Invalid status values return 400.
func (h *AdminVerificationHandler) ListPending(c *gin.Context) {
	ctx := c.Request.Context()

	// Determine target status: explicit ?status= or default to pending_review.
	statusParam := c.Query("status")
	targetStatus := verificationEntity.StatusPendingReview
	if statusParam != "" {
		candidate := verificationEntity.Status(statusParam)
		if !validAdminListStatuses[candidate] {
			response.BadRequest(c, "Invalid status filter. Valid values: pending_review, approved, needs_resubmission, rejected, suspended, revoked, under_investigation")
			return
		}
		targetStatus = candidate
	}

	pending, err := h.sellerService.ListVerificationsByStatusWithUsername(ctx, targetStatus)
	if err != nil {
		h.log.Error("verification admin: list by status failed",
			zap.String("status", string(targetStatus)), zap.Error(err))
		response.InternalServerError(c, "Failed to load verifications")
		return
	}

	items := make([]AdminVerificationListItem, 0, len(pending))
	for _, v := range pending {
		items = append(items, AdminVerificationListItem{
			ID:             v.ID,
			SellerID:       v.SellerID,
			SellerUsername: v.SellerUsername,
			SellerFarmName: v.SellerFarmName,
			Status:         v.Status,
			SubmittedAt:    v.SubmittedAt,
			CreatedAt:      v.CreatedAt,
			UpdatedAt:      v.UpdatedAt,
		})
	}
	response.Success(c, gin.H{"items": items, "count": len(items)})
}

// GetDetail handles GET /api/v1/admin/seller-verifications/:seller_id.
//
// Returns the full verification lifecycle row plus submitted documents.
// When a presigner is configured, document_url in each AdminVerificationDocumentItem
// is replaced with a short-lived presigned GET URL (AdminDocViewTTL) so KYC
// evidence is never served via permanent public URLs. For documents that lack
// a storage_key (legacy rows), document_url falls back to the stored value.
func (h *AdminVerificationHandler) GetDetail(c *gin.Context) {
	ctx := c.Request.Context()
	sellerID, err := h.parseSellerID(c)
	if err != nil {
		return
	}

	identity, err := h.sellerService.GetVerificationWithUsername(ctx, sellerID)
	if err != nil {
		if msg := err.Error(); containsAny(msg, "no record found") {
			response.NotFound(c, "Seller verification record not found")
			return
		}
		h.log.Error("verification admin: get detail failed",
			zap.String("seller_id", sellerID.String()), zap.Error(err))
		response.InternalServerError(c, "Failed to load verification detail")
		return
	}
	if identity == nil {
		response.NotFound(c, "Seller verification record not found")
		return
	}

	docs, err := h.docService.ListDocuments(ctx, sellerID)
	if err != nil {
		h.log.Warn("verification admin: list documents failed",
			zap.String("seller_id", sellerID.String()), zap.Error(err))
		docs = nil
	}

	// Fetch registered bank accounts for identity cross-check (fail-open).
	var bankAccounts []AdminBankAccountInfo
	if h.bankAcctDB != nil && h.bankAcctReader != nil {
		_ = h.bankAcctDB.WithTx(ctx, func(tx db.Tx) error {
			accounts, err := h.bankAcctReader.ListActiveAccountsBySeller(ctx, tx, sellerID)
			if err != nil {
				h.log.Warn("verification admin: list bank accounts failed",
					zap.String("seller_id", sellerID.String()), zap.Error(err))
				return nil // non-fatal
			}
			for _, ba := range accounts {
				bankAccounts = append(bankAccounts, AdminBankAccountInfo{
					ID:                  ba.ID.String(),
					BankName:            ba.BankName,
					BankCode:            ba.BankCode,
					AccountNumber:       ba.AccountNumber,
					AccountHolderName:   ba.AccountHolderName,
					IsDefault:           ba.IsDefault,
					IsReviewedForPayout: identity.HasReviewedBankAccount(ba.ID),
				})
			}
			return nil
		})
	}

	// Generate presigned GET URLs for documents (fail-open: view_url is empty
	// string when presigner unavailable or storage_key missing; admin can
	// refresh via GetDocumentViewURL).
	var docItems []AdminVerificationDocumentItem
	for _, d := range docs {
		var docURL string
		if h.presigner != nil && d.StorageKey != "" {
			if url, presignErr := h.presigner.PresignGET(d.StorageKey, AdminDocViewTTL); presignErr == nil {
				docURL = url
			} else {
				h.log.Warn("verification admin: presign GET failed",
					zap.String("doc_id", d.ID.String()), zap.Error(presignErr))
			}
		}
		docItems = append(docItems, AdminVerificationDocumentItem{
			ID:            d.ID,
			DocumentType:  d.DocumentType,
			DocumentName:  d.DocumentName,
			ViewURL:       docURL,
			Status:        d.Status,
			RejectionNote: d.RejectionNote,
			SubmittedAt:   d.SubmittedAt,
			ReviewedAt:    d.ReviewedAt,
		})
	}
	if docItems == nil {
		docItems = []AdminVerificationDocumentItem{}
	}

	resp := buildAdminVerificationDetailResponseWithDocs(
		sellerID,
		identity.SellerUsername,
		identity.SellerFarmName,
		&identity.SellerVerification,
		docItems,
		bankAccounts,
	)
	response.Success(c, resp)
}

// GetDocumentViewURL handles
// GET /api/v1/admin/seller-verifications/:seller_id/documents/:document_id/view-url.
//
// Returns a fresh short-lived presigned GET URL for a single KYC document.
// Useful when the URL embedded in GetDetail has expired (AdminDocViewTTL = 5 min).
func (h *AdminVerificationHandler) GetDocumentViewURL(c *gin.Context) {
	ctx := c.Request.Context()
	sellerID, err := h.parseSellerID(c)
	if err != nil {
		return
	}

	docIDStr := c.Param("document_id")
	docID, err := uuid.Parse(docIDStr)
	if err != nil {
		response.BadRequest(c, "Invalid document_id")
		return
	}

	if h.presigner == nil {
		h.log.Error("verification admin: view-url: presigner not configured")
		response.Error(c, 503, "PRESIGNER_NOT_CONFIGURED", "Document view URL service not configured")
		return
	}

	// Fetch the document to verify it belongs to the seller and get storage_key.
	docs, err := h.docService.ListDocuments(ctx, sellerID)
	if err != nil {
		h.log.Error("verification admin: view-url: list documents failed",
			zap.String("seller_id", sellerID.String()), zap.Error(err))
		response.InternalServerError(c, "Failed to fetch document")
		return
	}

	var storageKey string
	for _, d := range docs {
		if d.ID == docID {
			storageKey = d.StorageKey
			break
		}
	}
	if storageKey == "" {
		response.NotFound(c, "Document not found or has no storage key")
		return
	}

	// Guard: storage_key must be inside the kyc/ namespace. ListDocuments above
	// already enforces seller ownership; this check additionally prevents
	// presigning non-KYC paths in case of future data corruption or model drift.
	if !strings.HasPrefix(storageKey, "kyc/") {
		h.log.Error("verification admin: view-url: storage key outside KYC namespace",
			zap.String("doc_id", docIDStr))
		response.Error(c, 403, "INVALID_KEY_NAMESPACE", "Document is not in the KYC storage namespace")
		return
	}

	viewURL, err := h.presigner.PresignGET(storageKey, AdminDocViewTTL)
	if err != nil {
		h.log.Error("verification admin: view-url: presign failed",
			zap.String("doc_id", docIDStr), zap.Error(err))
		response.InternalServerError(c, "Failed to generate view URL")
		return
	}

	response.Success(c, DocumentViewURLResponse{
		ViewURL:   viewURL,
		ExpiresAt: time.Now().Add(AdminDocViewTTL),
	})
}

// Approve handles POST /api/v1/admin/seller-verifications/:seller_id/approve.
// Reason is optional; if provided it is recorded in the audit log metadata
// but the state-machine transition is approve-only (no rejection_reason on
// approved rows).
func (h *AdminVerificationHandler) Approve(c *gin.Context) {
	ctx := c.Request.Context()
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}
	sellerID, err := h.parseSellerID(c)
	if err != nil {
		return
	}

	// Body optional — bind best-effort and ignore EOF / malformed.
	var req AdminApproveRequest
	_ = c.ShouldBindJSON(&req)

	if err := h.sellerService.ApproveVerification(ctx, sellerID, adminID); err != nil {
		h.handleTransitionError(c, sellerID, adminID, "approve", err)
		return
	}

	response.SuccessWithMessage(c, "Seller verification approved", gin.H{
		"seller_id": sellerID,
		"status":    verificationEntity.StatusApproved,
	})
}

// Reject handles POST /api/v1/admin/seller-verifications/:seller_id/reject.
// Reason is MANDATORY.
func (h *AdminVerificationHandler) Reject(c *gin.Context) {
	ctx := c.Request.Context()
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}
	sellerID, err := h.parseSellerID(c)
	if err != nil {
		return
	}

	var req AdminReviewDecisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "REASON_REQUIRED", "Reason is required for rejection")
		return
	}

	if err := h.sellerService.RejectVerification(ctx, sellerID, adminID, req.Reason); err != nil {
		h.handleTransitionError(c, sellerID, adminID, "reject", err)
		return
	}

	response.SuccessWithMessage(c, "Seller verification rejected", gin.H{
		"seller_id": sellerID,
		"status":    verificationEntity.StatusRejected,
		"reason":    req.Reason,
	})
}

// RequestResubmission handles POST /api/v1/admin/seller-verifications/:seller_id/request-resubmission.
// Reason is MANDATORY.
func (h *AdminVerificationHandler) RequestResubmission(c *gin.Context) {
	ctx := c.Request.Context()
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}
	sellerID, err := h.parseSellerID(c)
	if err != nil {
		return
	}

	var req AdminReviewDecisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "REASON_REQUIRED", "Reason is required for resubmission request")
		return
	}

	if err := h.sellerService.RequestResubmission(ctx, sellerID, adminID, req.Reason); err != nil {
		h.handleTransitionError(c, sellerID, adminID, "request_resubmission", err)
		return
	}

	response.SuccessWithMessage(c, "Resubmission requested", gin.H{
		"seller_id": sellerID,
		"status":    verificationEntity.StatusNeedsResubmission,
		"reason":    req.Reason,
	})
}

// Suspend handles POST /api/v1/admin/seller-verifications/:seller_id/suspend.
// Reason is MANDATORY. Reversible trust pause — closes selling + payout authority.
func (h *AdminVerificationHandler) Suspend(c *gin.Context) {
	ctx := c.Request.Context()
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}
	sellerID, err := h.parseSellerID(c)
	if err != nil {
		return
	}

	var req AdminReviewDecisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "REASON_REQUIRED", "Reason is required for suspension")
		return
	}

	if err := h.sellerService.SuspendVerification(ctx, sellerID, adminID, req.Reason); err != nil {
		h.handleTransitionError(c, sellerID, adminID, "suspend", err)
		return
	}

	response.SuccessWithMessage(c, "Seller verification suspended", gin.H{
		"seller_id": sellerID,
		"status":    verificationEntity.StatusSuspended,
		"reason":    req.Reason,
	})
}

// Revoke handles POST /api/v1/admin/seller-verifications/:seller_id/revoke.
// Reason is MANDATORY. Terminal trust withdrawal — no recovery path.
func (h *AdminVerificationHandler) Revoke(c *gin.Context) {
	ctx := c.Request.Context()
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}
	sellerID, err := h.parseSellerID(c)
	if err != nil {
		return
	}

	var req AdminReviewDecisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "REASON_REQUIRED", "Reason is required for revocation")
		return
	}

	if err := h.sellerService.RevokeVerification(ctx, sellerID, adminID, req.Reason); err != nil {
		h.handleTransitionError(c, sellerID, adminID, "revoke", err)
		return
	}

	response.SuccessWithMessage(c, "Seller verification revoked", gin.H{
		"seller_id": sellerID,
		"status":    verificationEntity.StatusRevoked,
		"reason":    req.Reason,
	})
}

// Investigate handles POST /api/v1/admin/seller-verifications/:seller_id/investigate.
// Reason is MANDATORY. Payout authority closed; selling authority preserved (Option C).
func (h *AdminVerificationHandler) Investigate(c *gin.Context) {
	ctx := c.Request.Context()
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}
	sellerID, err := h.parseSellerID(c)
	if err != nil {
		return
	}

	var req AdminReviewDecisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "REASON_REQUIRED", "Reason is required for investigation")
		return
	}

	if err := h.sellerService.InvestigateVerification(ctx, sellerID, adminID, req.Reason); err != nil {
		h.handleTransitionError(c, sellerID, adminID, "investigate", err)
		return
	}

	response.SuccessWithMessage(c, "Seller verification under investigation", gin.H{
		"seller_id": sellerID,
		"status":    verificationEntity.StatusUnderInvestigation,
		"reason":    req.Reason,
	})
}

// Restore handles POST /api/v1/admin/seller-verifications/:seller_id/restore.
// Reason is optional. Restores from suspended or under_investigation to approved.
func (h *AdminVerificationHandler) Restore(c *gin.Context) {
	ctx := c.Request.Context()
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}
	sellerID, err := h.parseSellerID(c)
	if err != nil {
		return
	}

	// Body optional — bind best-effort and ignore EOF / malformed.
	var req AdminApproveRequest
	_ = c.ShouldBindJSON(&req)

	if err := h.sellerService.RestoreVerification(ctx, sellerID, adminID); err != nil {
		h.handleTransitionError(c, sellerID, adminID, "restore", err)
		return
	}

	response.SuccessWithMessage(c, "Seller verification restored", gin.H{
		"seller_id": sellerID,
		"status":    verificationEntity.StatusApproved,
	})
}

// MarkBankAccountReviewed handles
// POST /api/v1/admin/seller-verifications/:seller_id/bank-accounts/:bank_account_id/mark-reviewed.
//
// Appends bank_account_id to the seller's reviewed_bank_account_ids, allowing
// payout to that account without a full re-KYC cycle. Use this when a seller
// adds a new bank account post-approval and an admin has verified that the
// account belongs to the same KYC-approved identity.
//
// Gate: seller must be in approved status; account must be active + own by seller.
// Idempotent: marking an already-reviewed account returns 200 (no-op, no duplicate audit).
func (h *AdminVerificationHandler) MarkBankAccountReviewed(c *gin.Context) {
	ctx := c.Request.Context()
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}
	sellerID, err := h.parseSellerID(c)
	if err != nil {
		return
	}

	bankAccountIDStr := c.Param("bank_account_id")
	bankAccountID, err := uuid.Parse(bankAccountIDStr)
	if err != nil {
		response.BadRequest(c, "Invalid bank_account_id")
		return
	}

	if err := h.sellerService.MarkBankAccountReviewed(ctx, sellerID, bankAccountID, adminID); err != nil {
		var authErr *verificationApp.ErrVerificationAuthorityRequired
		if errors.As(err, &authErr) {
			response.Forbidden(c, "Missing seller.verification.review capability")
			return
		}
		var notApproved *verificationApp.ErrMarkReviewedNotApproved
		if errors.As(err, &notApproved) {
			response.Error(c, 409, "SELLER_NOT_APPROVED",
				"Seller must be in approved status to mark bank accounts as reviewed for payout")
			return
		}
		var notFound *verificationApp.ErrBankAccountNotFoundForSeller
		if errors.As(err, &notFound) {
			response.NotFound(c, "Bank account not found or not active for this seller")
			return
		}
		if msg := err.Error(); containsAny(msg, "no record found") {
			response.NotFound(c, "Seller verification record not found")
			return
		}
		h.log.Error("verification admin: mark bank account reviewed failed",
			zap.String("seller_id", sellerID.String()),
			zap.String("bank_account_id", bankAccountID.String()),
			zap.String("admin_id", adminID.String()),
			zap.Error(err))
		response.InternalServerError(c, "Failed to mark bank account as reviewed for payout")
		return
	}

	response.SuccessWithMessage(c, "Bank account marked as reviewed for payout", gin.H{
		"seller_id":       sellerID,
		"bank_account_id": bankAccountID,
	})
}

// parseSellerID extracts the :seller_id path parameter; emits 400 and logs
// nothing if parse fails. Returns ok=false (via err non-nil) to let callers
// short-circuit.
func (h *AdminVerificationHandler) parseSellerID(c *gin.Context) (uuid.UUID, error) {
	raw := c.Param("seller_id")
	id, err := uuid.Parse(raw)
	if err != nil {
		response.BadRequest(c, "Invalid seller_id")
		return uuid.Nil, err
	}
	return id, nil
}

// handleTransitionError maps service-layer errors from approve / reject /
// request_resubmission to canonical HTTP status codes.
//
//   - WrongStatusError  → 409 INVALID_STATE  (e.g. trying to approve a row
//     that is not pending_review)
//   - MissingReasonError → 400 REASON_REQUIRED (defensive — handler already
//     binds on the body but the entity also enforces)
//   - "no record found" → 404 NOT_FOUND
//   - anything else      → 500
func (h *AdminVerificationHandler) handleTransitionError(
	c *gin.Context,
	sellerID, adminID uuid.UUID,
	op string,
	err error,
) {
	var authErr *verificationApp.ErrVerificationAuthorityRequired
	if errors.As(err, &authErr) {
		response.Forbidden(c, "Missing seller.verification.review capability")
		return
	}
	var wrong *verificationEntity.WrongStatusError
	if errors.As(err, &wrong) {
		response.Error(c, 409, "INVALID_STATE",
			"Cannot "+op+" from current status: "+string(wrong.CurrentStatus))
		return
	}
	var invalidTransition *verificationEntity.InvalidTransitionError
	if errors.As(err, &invalidTransition) {
		response.Error(c, 409, "INVALID_STATE", invalidTransition.Error())
		return
	}
	var missingReason *verificationEntity.MissingReasonError
	if errors.As(err, &missingReason) {
		response.Error(c, 400, "REASON_REQUIRED", missingReason.Error())
		return
	}

	if msg := err.Error(); containsAny(msg, "no record found") {
		response.NotFound(c, "Seller verification record not found")
		return
	}

	h.log.Error("verification admin: transition failed",
		zap.String("op", op),
		zap.String("seller_id", sellerID.String()),
		zap.String("admin_id", adminID.String()),
		zap.Error(err))
	response.InternalServerError(c, "Failed to "+op+" verification")
}


