// Package repository provides the PostgreSQL implementation of the dispute repository.
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/dispute/entity"
	disputeRepo "github.com/labuda/backend/internal/governance/dispute/repository"
	"github.com/labuda/backend/pkg/db"
)

// DisputeRepositoryImpl implements the dispute repository using PostgreSQL.
type DisputeRepositoryImpl struct{}

// NewDisputeRepository creates a new DisputeRepositoryImpl.
func NewDisputeRepository() disputeRepo.DisputeRepository {
	return &DisputeRepositoryImpl{}
}

// Create creates a new dispute within a transaction.
func (r *DisputeRepositoryImpl) Create(
	ctx context.Context,
	tx db.Tx,
	dispute *entity.Dispute,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO disputes (
			id, order_id, buyer_id, seller_id,
			reason, description, status,
			opened_at, resolved_at, resolved_by, resolution_notes,
			timeout_days, is_overdue, overdue_marked_at,
			auto_resolved_at, auto_resolution_type,
			caller_id, reason_code,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
	`,
		dispute.ID,
		dispute.OrderID,
		dispute.BuyerID,
		dispute.SellerID,
		dispute.Reason,
		dispute.Description,
		string(dispute.Status),
		dispute.OpenedAt,
		dispute.ResolvedAt,
		dispute.ResolvedBy,
		dispute.ResolutionNotes,
		dispute.TimeoutDays,
		dispute.IsOverdue,
		dispute.OverdueMarkedAt,
		dispute.AutoResolvedAt,
		dispute.AutoResolutionType,
		dispute.CallerID,
		dispute.ReasonCode,
		dispute.CreatedAt,
		dispute.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("create dispute failed: %w", err)
	}

	// Store evidence URLs if present
	if len(dispute.EvidenceURLs) > 0 {
		for _, evidenceURL := range dispute.EvidenceURLs {
			_, err := tx.Exec(ctx, `
				INSERT INTO dispute_media (id, dispute_id, media_url, created_at)
				VALUES ($1, $2, $3, NOW())
			`, uuid.New(), dispute.ID, evidenceURL)

			if err != nil {
				return fmt.Errorf("create dispute evidence failed: %w", err)
			}
		}
	}

	return nil
}

// GetByOrderID retrieves a dispute by order ID without locking.
func (r *DisputeRepositoryImpl) GetByOrderID(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) (*entity.Dispute, error) {
	var id, buyerID, sellerID, resolvedBy, callerID uuid.UUID
	var reason, status string
	var description, resolutionNotes, autoResolutionType, reasonCode *string
	var openedAt, resolvedAt, overdueMarkedAt, autoResolvedAt *time.Time
	var createdAt, updatedAt time.Time
	var timeoutDays int
	var isOverdue bool

	err := tx.QueryRow(ctx, `
		SELECT id, buyer_id, seller_id, reason, description, status,
		       opened_at, resolved_at, resolved_by, resolution_notes,
		       timeout_days, is_overdue, overdue_marked_at,
		       auto_resolved_at, auto_resolution_type,
		       caller_id, reason_code,
		       created_at, updated_at
		FROM disputes
		WHERE order_id = $1
	`, orderID).Scan(
		&id, &buyerID, &sellerID, &reason, &description, &status,
		&openedAt, &resolvedAt, &resolvedBy, &resolutionNotes,
		&timeoutDays, &isOverdue, &overdueMarkedAt,
		&autoResolvedAt, &autoResolutionType,
		&callerID, &reasonCode,
		&createdAt, &updatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil // No dispute for this order
		}
		return nil, fmt.Errorf("get dispute by order_id failed: %w", err)
	}

	// Handle nullable resolved_by - treat UUID.Nil as null
	var resolvedByPtr *uuid.UUID
	if resolvedBy != uuid.Nil {
		resolvedByPtr = &resolvedBy
	}

	// Handle nullable caller_id - treat UUID.Nil as null
	var callerIDPtr *uuid.UUID
	if callerID != uuid.Nil {
		callerIDPtr = &callerID
	}

	dispute := &entity.Dispute{
		ID:                 id,
		OrderID:            orderID,
		BuyerID:            buyerID,
		SellerID:           sellerID,
		Reason:             reason,
		Description:        description,
		Status:             entity.DisputeStatus(status),
		OpenedAt:           *openedAt,
		ResolvedAt:         resolvedAt,
		ResolvedBy:         resolvedByPtr,
		ResolutionNotes:    resolutionNotes,
		TimeoutDays:        timeoutDays,
		IsOverdue:          isOverdue,
		OverdueMarkedAt:    overdueMarkedAt,
		AutoResolvedAt:     autoResolvedAt,
		AutoResolutionType: autoResolutionType,
		CallerID:           callerIDPtr,
		ReasonCode:         reasonCode,
		EvidenceURLs:       nil, // Loaded separately to avoid complex scanning
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
	}

	return dispute, nil
}

// GetForUpdate retrieves a dispute with FOR UPDATE lock.
func (r *DisputeRepositoryImpl) GetForUpdate(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (*entity.Dispute, error) {
	var orderID, buyerID, sellerID, resolvedBy, callerID uuid.UUID
	var reason, status string
	var description, resolutionNotes, autoResolutionType, reasonCode *string
	var openedAt time.Time
	var resolvedAt, overdueMarkedAt, autoResolvedAt *time.Time
	var createdAt, updatedAt time.Time
	var timeoutDays int
	var isOverdue bool

	err := tx.QueryRow(ctx, `
		SELECT id, order_id, buyer_id, seller_id, reason, description, status,
		       opened_at, resolved_at, resolved_by, resolution_notes,
		       timeout_days, is_overdue, overdue_marked_at,
		       auto_resolved_at, auto_resolution_type,
		       caller_id, reason_code,
		       created_at, updated_at
		FROM disputes
		WHERE id = $1
		FOR UPDATE
	`, id).Scan(
		&id, &orderID, &buyerID, &sellerID, &reason, &description, &status,
		&openedAt, &resolvedAt, &resolvedBy, &resolutionNotes,
		&timeoutDays, &isOverdue, &overdueMarkedAt,
		&autoResolvedAt, &autoResolutionType,
		&callerID, &reasonCode,
		&createdAt, &updatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, fmt.Errorf("dispute not found: %s", id)
		}
		return nil, fmt.Errorf("get dispute for update failed: %w", err)
	}

	// Handle nullable resolved_by - treat UUID.Nil as null
	var resolvedByPtr *uuid.UUID
	if resolvedBy != uuid.Nil {
		resolvedByPtr = &resolvedBy
	}

	// Handle nullable caller_id - treat UUID.Nil as null
	var callerIDPtr *uuid.UUID
	if callerID != uuid.Nil {
		callerIDPtr = &callerID
	}

	dispute := &entity.Dispute{
		ID:                 id,
		OrderID:            orderID,
		BuyerID:            buyerID,
		SellerID:           sellerID,
		Reason:             reason,
		Description:        description,
		Status:             entity.DisputeStatus(status),
		OpenedAt:           openedAt,
		ResolvedAt:         resolvedAt,
		ResolvedBy:         resolvedByPtr,
		ResolutionNotes:    resolutionNotes,
		TimeoutDays:        timeoutDays,
		IsOverdue:          isOverdue,
		OverdueMarkedAt:    overdueMarkedAt,
		AutoResolvedAt:     autoResolvedAt,
		AutoResolutionType: autoResolutionType,
		CallerID:           callerIDPtr,
		ReasonCode:         reasonCode,
		EvidenceURLs:       nil, // Loaded separately to avoid complex scanning
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
	}

	return dispute, nil
}

// Update updates an existing dispute within a transaction.
func (r *DisputeRepositoryImpl) Update(
	ctx context.Context,
	tx db.Tx,
	dispute *entity.Dispute,
) error {
	_, err := tx.Exec(ctx, `
		UPDATE disputes
		SET status = $2,
		    resolved_at = $3,
		    resolved_by = $4,
		    resolution_notes = $5,
		    is_overdue = $6,
		    overdue_marked_at = $7,
		    auto_resolved_at = $8,
		    auto_resolution_type = $9,
		    updated_at = $10
		WHERE id = $1
	`,
		dispute.ID,
		string(dispute.Status),
		dispute.ResolvedAt,
		dispute.ResolvedBy,
		dispute.ResolutionNotes,
		dispute.IsOverdue,
		dispute.OverdueMarkedAt,
		dispute.AutoResolvedAt,
		dispute.AutoResolutionType,
		dispute.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("update dispute failed: %w", err)
	}

	return nil
}

// CreateMedia creates a media attachment for a dispute.
func (r *DisputeRepositoryImpl) CreateMedia(
	ctx context.Context,
	tx db.Tx,
	disputeID uuid.UUID,
	mediaURL string,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO dispute_media (id, dispute_id, media_url, created_at)
		VALUES ($1, $2, $3, NOW())
	`, uuid.New(), disputeID, mediaURL)

	if err != nil {
		return fmt.Errorf("create dispute media failed: %w", err)
	}

	return nil
}

// ListMedia retrieves all media URLs for a dispute.
func (r *DisputeRepositoryImpl) ListMedia(
	ctx context.Context,
	tx db.Tx,
	disputeID uuid.UUID,
) ([]string, error) {
	rows, err := tx.Query(ctx, `
		SELECT media_url
		FROM dispute_media
		WHERE dispute_id = $1
		ORDER BY created_at ASC
	`, disputeID)

	if err != nil {
		return nil, fmt.Errorf("list dispute media failed: %w", err)
	}
	defer rows.Close()

	var mediaURLs []string
	for rows.Next() {
		var mediaURL string
		if err := rows.Scan(&mediaURL); err != nil {
			return nil, fmt.Errorf("scan dispute media failed: %w", err)
		}
		mediaURLs = append(mediaURLs, mediaURL)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dispute media failed: %w", err)
	}

	return mediaURLs, nil
}

// ============================================================================
// ADMIN QUERY METHODS
// ============================================================================

// ListAll retrieves all disputes with optional filters for admin.
func (r *DisputeRepositoryImpl) ListAll(
	ctx context.Context,
	tx db.Tx,
	filters disputeRepo.DisputeListFilters,
) ([]*entity.Dispute, int64, error) {
	// Validate pagination
	if filters.Page < 1 {
		filters.Page = 1
	}
	if filters.PageSize < 1 || filters.PageSize > 100 {
		filters.PageSize = 20
	}

	// Build base query
	baseQuery := `
		FROM disputes
		WHERE 1=1
	`

	args := []interface{}{}
	argIdx := 1

	// Add status filter
	if filters.Status != nil {
		baseQuery += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, *filters.Status)
		argIdx++
	}

	// Add date range filters
	if filters.DateFrom != nil {
		baseQuery += fmt.Sprintf(" AND opened_at >= $%d", argIdx)
		args = append(args, *filters.DateFrom)
		argIdx++
	}

	if filters.DateTo != nil {
		baseQuery += fmt.Sprintf(" AND opened_at <= $%d", argIdx)
		args = append(args, *filters.DateTo)
		argIdx++
	}

	// Get total count
	countQuery := "SELECT COUNT(*) " + baseQuery
	var total int64
	err := tx.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count disputes failed: %w", err)
	}

	// Get paginated results
	offset := (filters.Page - 1) * filters.PageSize
	dataQuery := `
		SELECT id, order_id, buyer_id, seller_id, reason, description, status,
		       opened_at, resolved_at, resolved_by, resolution_notes,
		       timeout_days, is_overdue, overdue_marked_at,
		       auto_resolved_at, auto_resolution_type,
		       created_at, updated_at
		` + baseQuery + `
		ORDER BY opened_at DESC
		LIMIT $` + fmt.Sprintf("%d", argIdx) + ` OFFSET $` + fmt.Sprintf("%d", argIdx+1)

	args = append(args, filters.PageSize, offset)

	rows, err := tx.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list disputes failed: %w", err)
	}
	defer rows.Close()

	disputes := []*entity.Dispute{}
	for rows.Next() {
		var id, orderID, buyerID, sellerID, resolvedBy uuid.UUID
		var reason, status string
		var description, resolutionNotes, autoResolutionType *string
		var openedAt time.Time
		var resolvedAt, overdueMarkedAt, autoResolvedAt *time.Time
		var createdAt, updatedAt time.Time
		var timeoutDays int
		var isOverdue bool

		err := rows.Scan(
			&id, &orderID, &buyerID, &sellerID, &reason, &description, &status,
			&openedAt, &resolvedAt, &resolvedBy, &resolutionNotes,
			&timeoutDays, &isOverdue, &overdueMarkedAt,
			&autoResolvedAt, &autoResolutionType,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan dispute failed: %w", err)
		}

		// Handle nullable resolved_by
		var resolvedByPtr *uuid.UUID
		if resolvedBy != uuid.Nil {
			resolvedByPtr = &resolvedBy
		}

		disputes = append(disputes, &entity.Dispute{
			ID:                 id,
			OrderID:            orderID,
			BuyerID:            buyerID,
			SellerID:           sellerID,
			Reason:             reason,
			Description:        description,
			Status:             entity.DisputeStatus(status),
			OpenedAt:           openedAt,
			ResolvedAt:         resolvedAt,
			ResolvedBy:         resolvedByPtr,
			ResolutionNotes:    resolutionNotes,
			TimeoutDays:        timeoutDays,
			IsOverdue:          isOverdue,
			OverdueMarkedAt:    overdueMarkedAt,
			AutoResolvedAt:     autoResolvedAt,
			AutoResolutionType: autoResolutionType,
			CreatedAt:          createdAt,
			UpdatedAt:          updatedAt,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate disputes failed: %w", err)
	}

	return disputes, total, nil
}

// GetByID retrieves a dispute by ID without locking (for read-only admin view).
func (r *DisputeRepositoryImpl) GetByID(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (*entity.Dispute, error) {
	var orderID, buyerID, sellerID, resolvedBy uuid.UUID
	var reason, status string
	var description, resolutionNotes, autoResolutionType *string
	var openedAt time.Time
	var resolvedAt, overdueMarkedAt, autoResolvedAt *time.Time
	var createdAt, updatedAt time.Time
	var timeoutDays int
	var isOverdue bool

	err := tx.QueryRow(ctx, `
		SELECT id, order_id, buyer_id, seller_id, reason, description, status,
		       opened_at, resolved_at, resolved_by, resolution_notes,
		       timeout_days, is_overdue, overdue_marked_at,
		       auto_resolved_at, auto_resolution_type,
		       created_at, updated_at
		FROM disputes
		WHERE id = $1
	`, id).Scan(
		&id, &orderID, &buyerID, &sellerID, &reason, &description, &status,
		&openedAt, &resolvedAt, &resolvedBy, &resolutionNotes,
		&timeoutDays, &isOverdue, &overdueMarkedAt,
		&autoResolvedAt, &autoResolutionType,
		&createdAt, &updatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil // Dispute not found
		}
		return nil, fmt.Errorf("get dispute by id failed: %w", err)
	}

	// Handle nullable resolved_by
	var resolvedByPtr *uuid.UUID
	if resolvedBy != uuid.Nil {
		resolvedByPtr = &resolvedBy
	}

	dispute := &entity.Dispute{
		ID:                 id,
		OrderID:            orderID,
		BuyerID:            buyerID,
		SellerID:           sellerID,
		Reason:             reason,
		Description:        description,
		Status:             entity.DisputeStatus(status),
		OpenedAt:           openedAt,
		ResolvedAt:         resolvedAt,
		ResolvedBy:         resolvedByPtr,
		ResolutionNotes:    resolutionNotes,
		TimeoutDays:        timeoutDays,
		IsOverdue:          isOverdue,
		OverdueMarkedAt:    overdueMarkedAt,
		AutoResolvedAt:     autoResolvedAt,
		AutoResolutionType: autoResolutionType,
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
	}

	return dispute, nil
}

// =============================================================================
// DEADLOCK PREVENTION METHODS
// =============================================================================

// FindOverdueCandidates retrieves disputes that should be marked as overdue.
// Uses FOR UPDATE SKIP LOCKED for concurrent worker safety.
func (r *DisputeRepositoryImpl) FindOverdueCandidates(
	ctx context.Context,
	tx db.Tx,
	limit int,
) ([]uuid.UUID, error) {
	query := `
		SELECT id
		FROM disputes
		WHERE status = 'under_review'
		  AND is_overdue = false
		  AND opened_at < (NOW() - INTERVAL '3 days')
		ORDER BY opened_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT $1
	`

	rows, err := tx.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("find overdue candidates failed: %w", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan dispute id failed: %w", err)
		}
		ids = append(ids, id)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate dispute ids failed: %w", rows.Err())
	}

	return ids, nil
}

// =============================================================================
// ABUSE DETECTION METHODS
// =============================================================================

// GetCallerDisputeCount returns the number of disputes opened by callerID at or after since.
// All statuses included: caller intent (how many disputes filed) is what matters for abuse detection.
func (r *DisputeRepositoryImpl) GetCallerDisputeCount(
	ctx context.Context,
	tx db.Tx,
	callerID uuid.UUID,
	since time.Time,
) (int, error) {
	var count int
	err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM disputes
		WHERE caller_id = $1
		  AND opened_at >= $2
	`, callerID, since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("GetCallerDisputeCount: %w", err)
	}
	return count, nil
}

// GetCallerDisputeCountAgainstParty returns the number of disputes opened by callerID
// against partyID (the other side: buyer_id or seller_id) at or after since.
func (r *DisputeRepositoryImpl) GetCallerDisputeCountAgainstParty(
	ctx context.Context,
	tx db.Tx,
	callerID uuid.UUID,
	partyID uuid.UUID,
	since time.Time,
) (int, error) {
	var count int
	err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM disputes
		WHERE caller_id = $1
		  AND (buyer_id = $2 OR seller_id = $2)
		  AND opened_at >= $3
	`, callerID, partyID, since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("GetCallerDisputeCountAgainstParty: %w", err)
	}
	return count, nil
}

// FindTimeoutCandidates retrieves disputes that should be auto-resolved.
// Uses FOR UPDATE SKIP LOCKED for concurrent worker safety.
func (r *DisputeRepositoryImpl) FindTimeoutCandidates(
	ctx context.Context,
	tx db.Tx,
	limit int,
) ([]uuid.UUID, error) {
	query := `
		SELECT d.id
		FROM disputes d
		WHERE d.status = 'under_review'
		  AND d.auto_resolved_at IS NULL
		  AND d.opened_at < (NOW() - (d.timeout_days || ' days')::INTERVAL)
		ORDER BY d.opened_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT $1
	`

	rows, err := tx.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("find timeout candidates failed: %w", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan dispute id failed: %w", err)
		}
		ids = append(ids, id)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate dispute ids failed: %w", rows.Err())
	}

	return ids, nil
}


