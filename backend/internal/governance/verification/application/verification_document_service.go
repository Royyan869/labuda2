package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/verification/entity"
	"github.com/labuda/backend/internal/governance/verification/infrastructure/repository"
	outboxrepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
)

// VerificationDocumentService handles verification document operations.
//
// KYC scope: identity_ktp + identity_selfie only.
// Business documents have been removed per owner decision (migration 000205).
//
// NOTIFICATION CONTINUITY:
// Emits truthful outbox events for verification decision outcomes.
// All events are registered in notification_worker.go and produce
// CommerceCritical (push-capable) notifications to the seller.
type VerificationDocumentService struct {
	db         Transactor
	docRepo    *repository.VerificationDocumentRepository
	sellerRepo *repository.SellerVerificationRepository
	outboxRepo *outboxrepo.OutboxRepository
}

// NewVerificationDocumentService creates a new VerificationDocumentService.
func NewVerificationDocumentService(
	db Transactor,
	docRepo *repository.VerificationDocumentRepository,
	sellerRepo *repository.SellerVerificationRepository,
	outboxRepo *outboxrepo.OutboxRepository,
) *VerificationDocumentService {
	return &VerificationDocumentService{
		db:         db,
		docRepo:    docRepo,
		sellerRepo: sellerRepo,
		outboxRepo: outboxRepo,
	}
}

// SubmitKYCDocuments submits the canonical minimal KYC evidence set —
// an identity_ktp document AND an identity_selfie document — in a single
// atomic transaction.
//
// Both documents must be uploaded to S3 via backend-issued presigned PUT URLs
// (POST /seller/verification/documents/upload-url) before calling this method.
// The caller supplies only the S3 object keys (storage_key fields); no
// document_url is stored — admin reads generate short-lived presigned GET
// URLs on demand from the storage_key.
//
// The seller-level lifecycle is driven to pending_review inside the same tx.
// Allowed source states: not_submitted, rejected, needs_resubmission.
// States pending_review / approved / suspended / revoked / under_investigation
// all reject new submissions (returned as InvalidTransitionError → 409 HTTP).
//
// Emits seller.verification.submitted.
func (s *VerificationDocumentService) SubmitKYCDocuments(
	ctx context.Context,
	userID uuid.UUID,
	fullName string,
	nationalID string,
	ktpStorageKey string,
	selfieStorageKey string,
) error {
	// Validate storage key ownership + document-type namespace before opening
	// a transaction. The backend is the sole key generator
	// (POST /seller/verification/documents/upload-url embeds userID and docType
	// in the path), so a string-prefix check is sufficient to reject
	// client-constructed keys or cross-user keys.
	ktpPrefix := fmt.Sprintf("kyc/%s/identity_ktp/", userID.String())
	selfiePrefix := fmt.Sprintf("kyc/%s/identity_selfie/", userID.String())
	if !strings.HasPrefix(ktpStorageKey, ktpPrefix) {
		return fmt.Errorf("verification: ktp_storage_key namespace or ownership invalid")
	}
	if !strings.HasPrefix(selfieStorageKey, selfiePrefix) {
		return fmt.Errorf("verification: selfie_storage_key namespace or ownership invalid")
	}

	return s.db.WithTx(ctx, func(tx db.Tx) error {
		// Check for existing pending/approved KTP — reject duplicate submissions.
		existingKTP, err := s.docRepo.GetByUserIDAndType(ctx, tx, userID, entity.DocumentTypeIdentityKTP)
		if err != nil {
			return fmt.Errorf("verification: check existing KTP failed: %w", err)
		}
		if existingKTP != nil && (existingKTP.IsPending() || existingKTP.IsApproved()) {
			return fmt.Errorf("verification: identity verification already %s", existingKTP.Status)
		}

		// Drive the seller-level lifecycle BEFORE persisting documents so
		// any invalid-transition error short-circuits without leaving orphan rows.
		if err := s.driveSellerLifecycleSubmitTx(ctx, tx, userID); err != nil {
			return err
		}

		// Create KTP document (document_url stored as ""; admin reads via presigned GET).
		ktpDoc := entity.NewVerificationDocument(
			userID,
			entity.DocumentTypeIdentityKTP,
			ktpStorageKey,
			fmt.Sprintf("KTP_%s", fullName),
		)
		if err := s.docRepo.Create(ctx, tx, ktpDoc); err != nil {
			return fmt.Errorf("verification: create KTP document failed: %w", err)
		}

		// Create selfie document.
		selfieDoc := entity.NewVerificationDocument(
			userID,
			entity.DocumentTypeIdentitySelfie,
			selfieStorageKey,
			fmt.Sprintf("Selfie_%s", fullName),
		)
		if err := s.docRepo.Create(ctx, tx, selfieDoc); err != nil {
			return fmt.Errorf("verification: create selfie document failed: %w", err)
		}

		return nil
	})
}

// ListDocuments retrieves all verification documents for a user.
func (s *VerificationDocumentService) ListDocuments(
	ctx context.Context,
	userID uuid.UUID,
) ([]*entity.VerificationDocument, error) {
	var docs []*entity.VerificationDocument
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		docs, err = s.docRepo.GetByUserID(ctx, tx, userID)
		return err
	})

	if err != nil {
		return nil, err
	}

	return docs, nil
}

// DeleteDocument deletes a verification document.
// Only allows deletion of documents that are not approved.
func (s *VerificationDocumentService) DeleteDocument(
	ctx context.Context,
	userID uuid.UUID,
	documentID uuid.UUID,
) error {
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		doc, err := s.docRepo.GetByID(ctx, tx, documentID)
		if err != nil {
			return fmt.Errorf("verification: get document failed: %w", err)
		}

		if doc == nil || doc.UserID != userID {
			return fmt.Errorf("verification: document not found")
		}

		if doc.IsApproved() {
			return fmt.Errorf("verification: cannot delete approved document")
		}

		return s.docRepo.Delete(ctx, tx, documentID)
	})
}

// ApproveDocument approves a verification document (admin only).
//
// NOTE: users.is_id_verified / users.is_farm_verified are NOT updated here.
// Those legacy columns are written by no gate and read by no gate.
// The canonical payout authority gate is seller_verifications.status == 'approved'
// (see VerificationService.HasPayoutAuthority). Do not reinstate the sync.
func (s *VerificationDocumentService) ApproveDocument(
	ctx context.Context,
	documentID uuid.UUID,
	adminID uuid.UUID,
) error {
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		doc, err := s.docRepo.GetByID(ctx, tx, documentID)
		if err != nil {
			return fmt.Errorf("verification: get document failed: %w", err)
		}

		if doc == nil {
			return fmt.Errorf("verification: document not found")
		}

		if err := doc.Approve(adminID); err != nil {
			return err
		}

		if err := s.docRepo.Update(ctx, tx, doc); err != nil {
			return err
		}

		// NOTIFICATION CONTINUITY: emit non-fatal document-level event.
		if s.outboxRepo != nil {
			payload := map[string]interface{}{
				"document_id":   documentID.String(),
				"user_id":       doc.UserID.String(),
				"document_type": string(doc.DocumentType),
				"approved_by":   adminID.String(),
			}
			payloadBytes, _ := json.Marshal(payload)
			_ = s.outboxRepo.InsertEvent(ctx, tx, "verification.document.approved", documentID, payloadBytes)
		}

		return nil
	})
}

// RejectDocument rejects a verification document (admin only).
func (s *VerificationDocumentService) RejectDocument(
	ctx context.Context,
	documentID uuid.UUID,
	adminID uuid.UUID,
	reason string,
) error {
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		doc, err := s.docRepo.GetByID(ctx, tx, documentID)
		if err != nil {
			return fmt.Errorf("verification: get document failed: %w", err)
		}

		if doc == nil {
			return fmt.Errorf("verification: document not found")
		}

		if err := doc.Reject(adminID, reason); err != nil {
			return err
		}

		if err := s.docRepo.Update(ctx, tx, doc); err != nil {
			return err
		}

		// OUTBOX ATOMIC — emit in the same transaction.
		if s.outboxRepo != nil {
			payload := map[string]interface{}{
				"document_id":   documentID.String(),
				"user_id":       doc.UserID.String(),
				"document_type": string(doc.DocumentType),
				"rejected_by":   adminID.String(),
				"reason":        reason,
			}
			payloadBytes, _ := json.Marshal(payload)
			if err := s.outboxRepo.InsertEvent(ctx, tx, "verification.document.rejected", documentID, payloadBytes); err != nil {
				return fmt.Errorf("outbox verification.document.rejected: %w", err)
			}
		}

		return nil
	})
}

// ListPendingDocuments retrieves all pending documents for admin review.
func (s *VerificationDocumentService) ListPendingDocuments(
	ctx context.Context,
) ([]*entity.VerificationDocument, error) {
	var docs []*entity.VerificationDocument
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		docs, err = s.docRepo.GetPendingDocuments(ctx, tx)
		return err
	})

	if err != nil {
		return nil, err
	}

	return docs, nil
}

// driveSellerLifecycleSubmitTx transitions the seller_verifications row to
// pending_review via the canonical entity.Submit state machine and emits
// seller.verification.submitted in the same tx.
//
// Allowed source states: not_submitted, rejected, needs_resubmission.
// Any other state returns InvalidTransitionError → 409 at the HTTP layer.
//
// If the seller has no lifecycle row (new seller), creates one defensively.
func (s *VerificationDocumentService) driveSellerLifecycleSubmitTx(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
) error {
	v, err := s.sellerRepo.GetForUpdate(ctx, tx, sellerID)
	if err != nil {
		return fmt.Errorf("verification: get seller lifecycle for update: %w", err)
	}

	if v == nil {
		v = entity.NewSellerVerification(sellerID)
		if err := v.Submit(); err != nil {
			return err
		}
		if err := s.sellerRepo.Create(ctx, tx, v); err != nil {
			return err
		}
	} else {
		if err := v.Submit(); err != nil {
			return err
		}
		if err := s.sellerRepo.Update(ctx, tx, v); err != nil {
			return err
		}
	}

	if s.outboxRepo != nil {
		payload := map[string]interface{}{
			"seller_id": sellerID.String(),
			"status":    string(v.Status),
		}
		payloadBytes, _ := json.Marshal(payload)
		_ = s.outboxRepo.InsertEvent(ctx, tx, "seller.verification.submitted", sellerID, payloadBytes)
	}

	return nil
}


