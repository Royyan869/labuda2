package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/governance/verification/entity"
	"github.com/labuda/backend/pkg/db"
)

// VerificationDocumentRepository handles verification document persistence.
type VerificationDocumentRepository struct{}

// NewVerificationDocumentRepository creates a new VerificationDocumentRepository.
func NewVerificationDocumentRepository() *VerificationDocumentRepository {
	return &VerificationDocumentRepository{}
}

// GetByID retrieves a verification document by ID.
// Returns nil if not found.
func (r *VerificationDocumentRepository) GetByID(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (*entity.VerificationDocument, error) {
	var doc entity.VerificationDocument
	var reviewedAt *time.Time
	var reviewedBy *uuid.UUID
	var rejectionNote *string
	var storageKey *string

	err := tx.QueryRow(ctx, `
		SELECT id, user_id, document_type, storage_key, document_name,
		       status, rejection_note, submitted_at, reviewed_at, reviewed_by,
		       created_at, updated_at
		FROM verification_documents
		WHERE id = $1
	`, id).Scan(
		&doc.ID, &doc.UserID, &doc.DocumentType, &storageKey, &doc.DocumentName,
		&doc.Status, &rejectionNote, &doc.SubmittedAt, &reviewedAt, &reviewedBy,
		&doc.CreatedAt, &doc.UpdatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("verification_document: get by id failed: %w", err)
	}

	doc.ReviewedAt = reviewedAt
	doc.ReviewedBy = reviewedBy
	doc.RejectionNote = rejectionNote
	if storageKey != nil {
		doc.StorageKey = *storageKey
	}

	return &doc, nil
}

// GetByUserID retrieves all verification documents for a user.
func (r *VerificationDocumentRepository) GetByUserID(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) ([]*entity.VerificationDocument, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, user_id, document_type, storage_key, document_name,
		       status, rejection_note, submitted_at, reviewed_at, reviewed_by,
		       created_at, updated_at
		FROM verification_documents
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("verification_document: get by user failed: %w", err)
	}
	defer rows.Close()

	var docs []*entity.VerificationDocument
	for rows.Next() {
		doc, scanErr := r.scanRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		docs = append(docs, doc)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("verification_document: scan failed: %w", rows.Err())
	}

	return docs, nil
}

// GetByUserIDAndType retrieves a verification document by user ID and document type.
// Returns nil if not found.
func (r *VerificationDocumentRepository) GetByUserIDAndType(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	docType entity.DocumentType,
) (*entity.VerificationDocument, error) {
	var doc entity.VerificationDocument
	var reviewedAt *time.Time
	var reviewedBy *uuid.UUID
	var rejectionNote *string
	var storageKey *string

	err := tx.QueryRow(ctx, `
		SELECT id, user_id, document_type, storage_key, document_name,
		       status, rejection_note, submitted_at, reviewed_at, reviewed_by,
		       created_at, updated_at
		FROM verification_documents
		WHERE user_id = $1 AND document_type = $2
		ORDER BY created_at DESC
		LIMIT 1
	`, userID, docType).Scan(
		&doc.ID, &doc.UserID, &doc.DocumentType, &storageKey, &doc.DocumentName,
		&doc.Status, &rejectionNote, &doc.SubmittedAt, &reviewedAt, &reviewedBy,
		&doc.CreatedAt, &doc.UpdatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("verification_document: get by user and type failed: %w", err)
	}

	doc.ReviewedAt = reviewedAt
	doc.ReviewedBy = reviewedBy
	doc.RejectionNote = rejectionNote
	if storageKey != nil {
		doc.StorageKey = *storageKey
	}

	return &doc, nil
}

// GetPendingDocuments retrieves all pending verification documents for admin review.
func (r *VerificationDocumentRepository) GetPendingDocuments(
	ctx context.Context,
	tx db.Tx,
) ([]*entity.VerificationDocument, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, user_id, document_type, storage_key, document_name,
		       status, rejection_note, submitted_at, reviewed_at, reviewed_by,
		       created_at, updated_at
		FROM verification_documents
		WHERE status = 'pending'
		ORDER BY submitted_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("verification_document: get pending failed: %w", err)
	}
	defer rows.Close()

	var docs []*entity.VerificationDocument
	for rows.Next() {
		doc, scanErr := r.scanRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		docs = append(docs, doc)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("verification_document: scan failed: %w", rows.Err())
	}

	return docs, nil
}

// Create creates a new verification document.
func (r *VerificationDocumentRepository) Create(
	ctx context.Context,
	tx db.Tx,
	doc *entity.VerificationDocument,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO verification_documents (
			id, user_id, document_type, storage_key, document_name,
			status, rejection_note, submitted_at, reviewed_at, reviewed_by,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, doc.ID, doc.UserID, doc.DocumentType,
		nullableString(doc.StorageKey), doc.DocumentName,
		doc.Status, doc.RejectionNote, doc.SubmittedAt, doc.ReviewedAt, doc.ReviewedBy,
		doc.CreatedAt, doc.UpdatedAt)

	if err != nil {
		return fmt.Errorf("verification_document: create failed: %w", err)
	}

	return nil
}

// Update updates an existing verification document.
func (r *VerificationDocumentRepository) Update(
	ctx context.Context,
	tx db.Tx,
	doc *entity.VerificationDocument,
) error {
	result, err := tx.Exec(ctx, `
		UPDATE verification_documents
		SET status = $1,
		    rejection_note = $2,
		    reviewed_at = $3,
		    reviewed_by = $4,
		    updated_at = $5
		WHERE id = $6
	`, doc.Status, doc.RejectionNote, doc.ReviewedAt, doc.ReviewedBy,
		doc.UpdatedAt, doc.ID)

	if err != nil {
		return fmt.Errorf("verification_document: update failed: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("verification_document: update failed, record not found")
	}

	return nil
}

// Delete deletes a verification document.
func (r *VerificationDocumentRepository) Delete(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) error {
	result, err := tx.Exec(ctx, `
		DELETE FROM verification_documents
		WHERE id = $1
	`, id)

	if err != nil {
		return fmt.Errorf("verification_document: delete failed: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("verification_document: delete failed, record not found")
	}

	return nil
}

// HasApprovedIdentityDocument checks if user has at least one approved identity
// document (KTP or selfie). Used to determine payout-authority eligibility
// at the document level (seller-level gate is seller_verifications.status).
func (r *VerificationDocumentRepository) HasApprovedIdentityDocument(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) (bool, error) {
	var count int
	err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM verification_documents
		WHERE user_id = $1
		  AND document_type IN ('identity_ktp', 'identity_selfie')
		  AND status = 'approved'
	`, userID).Scan(&count)

	if err != nil {
		return false, fmt.Errorf("verification_document: check identity failed: %w", err)
	}

	return count > 0, nil
}

// scanRow scans a verification document from a pgx.Rows cursor.
func (r *VerificationDocumentRepository) scanRow(rows pgx.Rows) (*entity.VerificationDocument, error) {
	var doc entity.VerificationDocument
	var reviewedAt *time.Time
	var reviewedBy *uuid.UUID
	var rejectionNote *string
	var storageKey *string

	err := rows.Scan(
		&doc.ID, &doc.UserID, &doc.DocumentType, &storageKey, &doc.DocumentName,
		&doc.Status, &rejectionNote, &doc.SubmittedAt, &reviewedAt, &reviewedBy,
		&doc.CreatedAt, &doc.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("verification_document: scan row failed: %w", err)
	}

	doc.ReviewedAt = reviewedAt
	doc.ReviewedBy = reviewedBy
	doc.RejectionNote = rejectionNote
	if storageKey != nil {
		doc.StorageKey = *storageKey
	}

	return &doc, nil
}

// nullableString converts an empty string to nil for nullable TEXT columns.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}


