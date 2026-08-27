package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/governance/verification/entity"
	"github.com/labuda/backend/pkg/db"
)

// SellerVerificationRepository handles seller verification persistence operations.
// All mutations use FOR UPDATE to prevent race conditions.
// No soft delete - records are permanent.
type SellerVerificationRepository struct{}

func NewSellerVerificationRepository() *SellerVerificationRepository {
	return &SellerVerificationRepository{}
}

// scanReviewedBankAccountIDs extracts a []uuid.UUID from a raw interface value
// returned by pgx when scanning a UUID[] column. pgx/v5 returns UUID[] columns
// as []interface{} where each element is a [16]byte. We normalise that here so
// the rest of the codebase always works with []uuid.UUID.
func scanReviewedBankAccountIDs(raw interface{}) []uuid.UUID {
	if raw == nil {
		return []uuid.UUID{}
	}
	switch v := raw.(type) {
	case []uuid.UUID:
		return v
	case []interface{}:
		ids := make([]uuid.UUID, 0, len(v))
		for _, elem := range v {
			switch e := elem.(type) {
			case uuid.UUID:
				ids = append(ids, e)
			case [16]byte:
				ids = append(ids, uuid.UUID(e))
			case string:
				if id, err := uuid.Parse(e); err == nil {
					ids = append(ids, id)
				}
			}
		}
		return ids
	case []string:
		ids := make([]uuid.UUID, 0, len(v))
		for _, s := range v {
			if id, err := uuid.Parse(s); err == nil {
				ids = append(ids, id)
			}
		}
		return ids
	}
	return []uuid.UUID{}
}

// GetBySellerID retrieves a verification record by seller ID.
// Returns nil if not found (treat as not_submitted).
func (r *SellerVerificationRepository) GetBySellerID(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
) (*entity.SellerVerification, error) {
	var v entity.SellerVerification
	var submittedAt, reviewedAt *time.Time
	var reason *string
	var reviewedBy *uuid.UUID
	var reviewedBankRaw interface{}

	err := tx.QueryRow(ctx, `
		SELECT id, seller_id, status, submitted_at, reviewed_at, reviewed_by,
		       reason, reviewed_bank_account_ids, created_at, updated_at
		FROM seller_verifications
		WHERE seller_id = $1
	`, sellerID).Scan(
		&v.ID, &v.SellerID, &v.Status, &submittedAt, &reviewedAt, &reviewedBy,
		&reason, &reviewedBankRaw, &v.CreatedAt, &v.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("verification: get by seller failed: %w", err)
	}

	v.SubmittedAt = submittedAt
	v.ReviewedAt = reviewedAt
	v.ReviewedBy = reviewedBy
	v.Reason = reason
	v.ReviewedBankAccountIDs = scanReviewedBankAccountIDs(reviewedBankRaw)

	return &v, nil
}

// GetForUpdate retrieves a verification record by seller ID with FOR UPDATE lock.
// Returns nil if not found.
func (r *SellerVerificationRepository) GetForUpdate(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
) (*entity.SellerVerification, error) {
	var v entity.SellerVerification
	var submittedAt, reviewedAt *time.Time
	var reason *string
	var reviewedBy *uuid.UUID
	var reviewedBankRaw interface{}

	err := tx.QueryRow(ctx, `
		SELECT id, seller_id, status, submitted_at, reviewed_at, reviewed_by,
		       reason, reviewed_bank_account_ids, created_at, updated_at
		FROM seller_verifications
		WHERE seller_id = $1
		FOR UPDATE
	`, sellerID).Scan(
		&v.ID, &v.SellerID, &v.Status, &submittedAt, &reviewedAt, &reviewedBy,
		&reason, &reviewedBankRaw, &v.CreatedAt, &v.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("verification: get for update failed: %w", err)
	}

	v.SubmittedAt = submittedAt
	v.ReviewedAt = reviewedAt
	v.ReviewedBy = reviewedBy
	v.Reason = reason
	v.ReviewedBankAccountIDs = scanReviewedBankAccountIDs(reviewedBankRaw)

	return &v, nil
}

// Create creates a new verification record.
// Fails if seller_id already exists (UNIQUE constraint).
func (r *SellerVerificationRepository) Create(
	ctx context.Context,
	tx db.Tx,
	v *entity.SellerVerification,
) error {
	ids := v.ReviewedBankAccountIDs
	if ids == nil {
		ids = []uuid.UUID{}
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO seller_verifications (
			id, seller_id, status, submitted_at, reviewed_at, reviewed_by,
			reason, reviewed_bank_account_ids, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, v.ID, v.SellerID, v.Status, v.SubmittedAt, v.ReviewedAt, v.ReviewedBy,
		v.Reason, ids, v.CreatedAt, v.UpdatedAt)

	if err != nil {
		return fmt.Errorf("verification: create failed: %w", err)
	}
	return nil
}

// Update updates an existing verification record including reviewed_bank_account_ids.
// Use GetForUpdate before calling to prevent race conditions.
func (r *SellerVerificationRepository) Update(
	ctx context.Context,
	tx db.Tx,
	v *entity.SellerVerification,
) error {
	ids := v.ReviewedBankAccountIDs
	if ids == nil {
		ids = []uuid.UUID{}
	}
	result, err := tx.Exec(ctx, `
		UPDATE seller_verifications
		SET status = $1,
		    submitted_at = $2,
		    reviewed_at = $3,
		    reviewed_by = $4,
		    reason = $5,
		    reviewed_bank_account_ids = $6,
		    updated_at = $7
		WHERE id = $8
	`, v.Status, v.SubmittedAt, v.ReviewedAt, v.ReviewedBy,
		v.Reason, ids, v.UpdatedAt, v.ID)

	if err != nil {
		return fmt.Errorf("verification: update failed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("verification: update failed, record not found")
	}
	return nil
}

// Upsert inserts or updates a verification record.
func (r *SellerVerificationRepository) Upsert(
	ctx context.Context,
	tx db.Tx,
	v *entity.SellerVerification,
) (bool, error) {
	err := r.Create(ctx, tx, v)
	if err == nil {
		return true, nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		updateErr := r.Update(ctx, tx, v)
		if updateErr != nil {
			return false, updateErr
		}
		return false, nil
	}

	return false, err
}

// PendingVerificationRow carries the canonical verification fields plus the
// seller's public username (from user_profiles).
type PendingVerificationRow struct {
	entity.SellerVerification
	SellerUsername *string
	SellerFarmName *string
}

// VerificationIdentityRow carries the canonical verification lifecycle row
// plus the seller's public username and farm/store name for admin display.
type VerificationIdentityRow struct {
	entity.SellerVerification
	SellerUsername *string
	SellerFarmName *string
}

func (r *SellerVerificationRepository) ListPendingWithUsername(
	ctx context.Context,
	tx db.Tx,
) ([]PendingVerificationRow, error) {
	rows, err := tx.Query(ctx, `
		SELECT sv.id, sv.seller_id, sv.status, sv.submitted_at, sv.reviewed_at,
		       sv.reviewed_by, sv.reason, sv.reviewed_bank_account_ids,
		       sv.created_at, sv.updated_at,
		       up.username, sp.store_name
		FROM seller_verifications sv
		LEFT JOIN user_profiles up ON up.user_id = sv.seller_id
		LEFT JOIN seller_profiles sp ON sp.user_id = sv.seller_id
		WHERE sv.status = 'pending_review'
		ORDER BY sv.created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("verification: list pending with username failed: %w", err)
	}
	defer rows.Close()

	var result []PendingVerificationRow
	for rows.Next() {
		var v entity.SellerVerification
		var submittedAt, reviewedAt *time.Time
		var reason *string
		var reviewedBy *uuid.UUID
		var reviewedBankRaw interface{}
		var username, farmName *string

		if err := rows.Scan(
			&v.ID, &v.SellerID, &v.Status, &submittedAt, &reviewedAt,
			&reviewedBy, &reason, &reviewedBankRaw, &v.CreatedAt, &v.UpdatedAt,
			&username, &farmName,
		); err != nil {
			return nil, fmt.Errorf("verification: scan pending row failed: %w", err)
		}

		v.SubmittedAt = submittedAt
		v.ReviewedAt = reviewedAt
		v.ReviewedBy = reviewedBy
		v.Reason = reason
		v.ReviewedBankAccountIDs = scanReviewedBankAccountIDs(reviewedBankRaw)

		result = append(result, PendingVerificationRow{
			SellerVerification: v,
			SellerUsername:     username,
			SellerFarmName:     farmName,
		})
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("verification: scan failed: %w", rows.Err())
	}
	return result, nil
}

func (r *SellerVerificationRepository) ListByStatusWithUsername(
	ctx context.Context,
	tx db.Tx,
	status entity.Status,
) ([]PendingVerificationRow, error) {
	rows, err := tx.Query(ctx, `
		SELECT sv.id, sv.seller_id, sv.status, sv.submitted_at, sv.reviewed_at,
		       sv.reviewed_by, sv.reason, sv.reviewed_bank_account_ids,
		       sv.created_at, sv.updated_at,
		       up.username, sp.store_name
		FROM seller_verifications sv
		LEFT JOIN user_profiles up ON up.user_id = sv.seller_id
		LEFT JOIN seller_profiles sp ON sp.user_id = sv.seller_id
		WHERE sv.status = $1
		ORDER BY sv.created_at DESC
	`, status)
	if err != nil {
		return nil, fmt.Errorf("verification: list by status with username failed: %w", err)
	}
	defer rows.Close()

	var result []PendingVerificationRow
	for rows.Next() {
		var v entity.SellerVerification
		var submittedAt, reviewedAt *time.Time
		var reason *string
		var reviewedBy *uuid.UUID
		var reviewedBankRaw interface{}
		var username, farmName *string

		if err := rows.Scan(
			&v.ID, &v.SellerID, &v.Status, &submittedAt, &reviewedAt,
			&reviewedBy, &reason, &reviewedBankRaw, &v.CreatedAt, &v.UpdatedAt,
			&username, &farmName,
		); err != nil {
			return nil, fmt.Errorf("verification: scan row with username failed: %w", err)
		}

		v.SubmittedAt = submittedAt
		v.ReviewedAt = reviewedAt
		v.ReviewedBy = reviewedBy
		v.Reason = reason
		v.ReviewedBankAccountIDs = scanReviewedBankAccountIDs(reviewedBankRaw)

		result = append(result, PendingVerificationRow{
			SellerVerification: v,
			SellerUsername:     username,
			SellerFarmName:     farmName,
		})
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("verification: scan failed: %w", rows.Err())
	}
	return result, nil
}

func (r *SellerVerificationRepository) GetBySellerIDWithUsername(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
) (*VerificationIdentityRow, error) {
	var v entity.SellerVerification
	var submittedAt, reviewedAt *time.Time
	var reason *string
	var reviewedBy *uuid.UUID
	var reviewedBankRaw interface{}
	var username, farmName *string

	err := tx.QueryRow(ctx, `
		SELECT sv.id, sv.seller_id, sv.status, sv.submitted_at, sv.reviewed_at,
		       sv.reviewed_by, sv.reason, sv.reviewed_bank_account_ids,
		       sv.created_at, sv.updated_at,
		       up.username, sp.store_name
		FROM seller_verifications sv
		LEFT JOIN user_profiles up ON up.user_id = sv.seller_id
		LEFT JOIN seller_profiles sp ON sp.user_id = sv.seller_id
		WHERE sv.seller_id = $1
	`, sellerID).Scan(
		&v.ID, &v.SellerID, &v.Status, &submittedAt, &reviewedAt,
		&reviewedBy, &reason, &reviewedBankRaw, &v.CreatedAt, &v.UpdatedAt,
		&username, &farmName,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("verification: get by seller with username failed: %w", err)
	}

	v.SubmittedAt = submittedAt
	v.ReviewedAt = reviewedAt
	v.ReviewedBy = reviewedBy
	v.Reason = reason
	v.ReviewedBankAccountIDs = scanReviewedBankAccountIDs(reviewedBankRaw)

	return &VerificationIdentityRow{
		SellerVerification: v,
		SellerUsername:     username,
		SellerFarmName:     farmName,
	}, nil
}

func (r *SellerVerificationRepository) ListByStatus(
	ctx context.Context,
	tx db.Tx,
	status entity.Status,
) ([]*entity.SellerVerification, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, seller_id, status, submitted_at, reviewed_at, reviewed_by,
		       reason, reviewed_bank_account_ids, created_at, updated_at
		FROM seller_verifications
		WHERE status = $1
		ORDER BY created_at DESC
	`, status)
	if err != nil {
		return nil, fmt.Errorf("verification: list by status failed: %w", err)
	}
	defer rows.Close()

	var verifications []*entity.SellerVerification
	for rows.Next() {
		v, scanErr := r.scanRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		verifications = append(verifications, v)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("verification: scan failed: %w", rows.Err())
	}
	return verifications, nil
}

func (r *SellerVerificationRepository) scanRow(rows pgx.Rows) (*entity.SellerVerification, error) {
	var v entity.SellerVerification
	var submittedAt, reviewedAt *time.Time
	var reason *string
	var reviewedBy *uuid.UUID
	var reviewedBankRaw interface{}

	err := rows.Scan(
		&v.ID, &v.SellerID, &v.Status, &submittedAt, &reviewedAt, &reviewedBy,
		&reason, &reviewedBankRaw, &v.CreatedAt, &v.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("verification: scan row failed: %w", err)
	}

	v.SubmittedAt = submittedAt
	v.ReviewedAt = reviewedAt
	v.ReviewedBy = reviewedBy
	v.Reason = reason
	v.ReviewedBankAccountIDs = scanReviewedBankAccountIDs(reviewedBankRaw)

	return &v, nil
}


