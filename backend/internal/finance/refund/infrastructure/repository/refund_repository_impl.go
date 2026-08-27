// Package repository provides the PostgreSQL implementation of the refund repository.
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance/refund/entity"
	refundRepo "github.com/labuda/backend/internal/finance/refund/repository"
	"github.com/labuda/backend/pkg/db"
)

// RefundRepositoryImpl implements the refund repository using PostgreSQL.
type RefundRepositoryImpl struct{}

// NewRefundRepository creates a new RefundRepositoryImpl.
func NewRefundRepository() refundRepo.RefundRepository {
	return &RefundRepositoryImpl{}
}

// Create creates a new refund within a transaction.
//
// Includes the additive gateway-pipeline columns (000129). For freshly created
// refunds those default to ('unsubmitted', 0, NULL, NULL, NULL, NULL).
func (r *RefundRepositoryImpl) Create(
	ctx context.Context,
	tx db.Tx,
	refund *entity.Refund,
) error {
	gatewayStatus := refund.GatewayStatus
	if gatewayStatus == "" {
		gatewayStatus = entity.GatewayRefundUnsubmitted
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO refunds (
			id, order_id, buyer_id, seller_id,
			reason, description, status,
			requested_amount,
			seller_approved_percent, seller_approved_amount, seller_notes, seller_reviewed_at,
			admin_approved_percent, admin_approved_amount, admin_notes, reviewed_by, admin_reviewed_at,
			final_refund_amount,
			refunded_product_amount, refunded_shipping_amount, coins_refunded_amount,
			opened_at, approved_at, rejected_at, refunded_at,
			created_at, updated_at,
			gateway_refund_id, gateway_status, gateway_attempts, last_gateway_error,
			gateway_idempotency_key, gateway_requested_at, gateway_acknowledged_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24,
			$25, $26, $27, $28, $29, $30, $31, $32, $33, $34
		)
	`,
		refund.ID,
		refund.OrderID,
		refund.BuyerID,
		refund.SellerID,
		string(refund.Reason),
		refund.Description,
		string(refund.Status),
		refund.RequestedAmount,
		refund.SellerApprovedPercent,
		refund.SellerApprovedAmount,
		refund.SellerNotes,
		refund.SellerReviewedAt,
		refund.AdminApprovedPercent,
		refund.AdminApprovedAmount,
		refund.AdminNotes,
		refund.ReviewedBy,
		refund.AdminReviewedAt,
		refund.FinalRefundAmount,
		refund.RefundedProductAmount,
		refund.RefundedShippingAmount,
		refund.CoinsRefundedAmount,
		refund.OpenedAt,
		refund.ApprovedAt,
		refund.RejectedAt,
		refund.RefundedAt,
		refund.CreatedAt,
		refund.UpdatedAt,
		refund.GatewayRefundID,
		string(gatewayStatus),
		refund.GatewayAttempts,
		refund.LastGatewayError,
		refund.GatewayIdempotencyKey,
		refund.GatewayRequestedAt,
		refund.GatewayAcknowledgedAt,
	)

	if err != nil {
		return fmt.Errorf("create refund failed: %w", err)
	}

	return nil
}

// refundFullColumns is the canonical SELECT column list including the
// gateway pipeline columns added in migration 000129. Used by single-row
// fetchers (GetByID, GetByOrderID, GetForUpdate, GetByGateway*).
const refundFullColumns = `
	id, order_id, buyer_id, seller_id, reason, description, status,
	requested_amount,
	seller_approved_percent, seller_approved_amount, seller_notes, seller_reviewed_at,
	admin_approved_percent, admin_approved_amount, admin_notes, reviewed_by, admin_reviewed_at,
	final_refund_amount,
	refunded_product_amount, refunded_shipping_amount, coins_refunded_amount,
	opened_at, approved_at, rejected_at, refunded_at,
	created_at, updated_at,
	gateway_refund_id, gateway_status, gateway_attempts, last_gateway_error,
	gateway_idempotency_key, gateway_requested_at, gateway_acknowledged_at
`

// scanRefund reads a single refund row using the refundFullColumns layout.
// It does not load evidence URLs — that requires a separate query.
func scanRefund(row interface {
	Scan(dest ...any) error
}) (*entity.Refund, error) {
	var (
		id, orderID, buyerID, sellerID uuid.UUID
		reason, status                 string
		description                    sql.NullString
		requestedAmount                int64
		sellerApprovedPercent          sql.NullInt64
		sellerApprovedAmount           sql.NullInt64
		sellerNotes                    sql.NullString
		sellerReviewedAt               sql.NullTime
		adminApprovedPercent           sql.NullInt64
		adminApprovedAmount            sql.NullInt64
		adminNotes                     sql.NullString
		reviewedBy                     sql.NullString
		adminReviewedAt                sql.NullTime
		finalRefundAmount              sql.NullInt64
		refundedProductAmount          sql.NullInt64
		refundedShippingAmount         sql.NullInt64
		coinsRefundedAmount            sql.NullInt64
		openedAt, approvedAt           sql.NullTime
		rejectedAt, refundedAt         sql.NullTime
		createdAt, updatedAt           time.Time
		gatewayRefundID                sql.NullString
		gatewayStatus                  string
		gatewayAttempts                int
		lastGatewayError               sql.NullString
		gatewayIdempotencyKey          sql.NullString
		gatewayRequestedAt             sql.NullTime
		gatewayAcknowledgedAt          sql.NullTime
	)
	err := row.Scan(
		&id, &orderID, &buyerID, &sellerID, &reason, &description, &status,
		&requestedAmount,
		&sellerApprovedPercent, &sellerApprovedAmount, &sellerNotes, &sellerReviewedAt,
		&adminApprovedPercent, &adminApprovedAmount, &adminNotes, &reviewedBy, &adminReviewedAt,
		&finalRefundAmount,
		&refundedProductAmount, &refundedShippingAmount, &coinsRefundedAmount,
		&openedAt, &approvedAt, &rejectedAt, &refundedAt,
		&createdAt, &updatedAt,
		&gatewayRefundID, &gatewayStatus, &gatewayAttempts, &lastGatewayError,
		&gatewayIdempotencyKey, &gatewayRequestedAt, &gatewayAcknowledgedAt,
	)
	if err != nil {
		return nil, err
	}
	var reviewedByID uuid.UUID
	if reviewedByIDPtr := db.ToUUIDPtr(reviewedBy); reviewedByIDPtr != nil {
		reviewedByID = *reviewedByIDPtr
	}
	return &entity.Refund{
		ID:                     id,
		OrderID:                orderID,
		BuyerID:                buyerID,
		SellerID:               sellerID,
		Reason:                 entity.RefundReason(reason),
		Description:            db.ToStringPtr(description),
		Status:                 entity.RefundStatus(status),
		RequestedAmount:        requestedAmount,
		SellerApprovedPercent:  db.ToIntPtr(sellerApprovedPercent),
		SellerApprovedAmount:   db.ToInt64Ptr(sellerApprovedAmount),
		SellerNotes:            db.ToStringPtr(sellerNotes),
		SellerReviewedAt:       db.ToTimePtr(sellerReviewedAt),
		AdminApprovedPercent:   db.ToIntPtr(adminApprovedPercent),
		AdminApprovedAmount:    db.ToInt64Ptr(adminApprovedAmount),
		AdminNotes:             db.ToStringPtr(adminNotes),
		ReviewedBy:             reviewedByID,
		AdminReviewedAt:        db.ToTimePtr(adminReviewedAt),
		FinalRefundAmount:      db.ToInt64Ptr(finalRefundAmount),
		RefundedProductAmount:  db.ToInt64Ptr(refundedProductAmount),
		RefundedShippingAmount: db.ToInt64Ptr(refundedShippingAmount),
		CoinsRefundedAmount:    db.ToInt64Ptr(coinsRefundedAmount),
		OpenedAt:               *db.ToTimePtr(openedAt),
		ApprovedAt:             db.ToTimePtr(approvedAt),
		RejectedAt:             db.ToTimePtr(rejectedAt),
		RefundedAt:             db.ToTimePtr(refundedAt),
		CreatedAt:              createdAt,
		UpdatedAt:              updatedAt,
		GatewayRefundID:        db.ToStringPtr(gatewayRefundID),
		GatewayStatus:          entity.GatewayRefundStatus(gatewayStatus),
		GatewayAttempts:        gatewayAttempts,
		LastGatewayError:       db.ToStringPtr(lastGatewayError),
		GatewayIdempotencyKey:  db.ToStringPtr(gatewayIdempotencyKey),
		GatewayRequestedAt:     db.ToTimePtr(gatewayRequestedAt),
		GatewayAcknowledgedAt:  db.ToTimePtr(gatewayAcknowledgedAt),
	}, nil
}

// GetByID retrieves a refund by ID without locking.
func (r *RefundRepositoryImpl) GetByID(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (*entity.Refund, error) {
	row := tx.QueryRow(ctx, `SELECT `+refundFullColumns+` FROM refunds WHERE id = $1`, id)
	refund, err := scanRefund(row)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("get refund by id failed: %w", err)
	}
	evidenceURLs, err := r.ListEvidence(ctx, tx, id)
	if err != nil {
		return nil, fmt.Errorf("get refund evidence failed: %w", err)
	}
	refund.EvidenceURLs = evidenceURLs
	return refund, nil
}

// GetByOrderID retrieves a refund by order ID without locking.
func (r *RefundRepositoryImpl) GetByOrderID(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) (*entity.Refund, error) {
	row := tx.QueryRow(ctx, `SELECT `+refundFullColumns+` FROM refunds WHERE order_id = $1 ORDER BY created_at DESC LIMIT 1`, orderID)
	refund, err := scanRefund(row)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("get refund by order_id failed: %w", err)
	}
	evidenceURLs, err := r.ListEvidence(ctx, tx, refund.ID)
	if err != nil {
		return nil, fmt.Errorf("get refund evidence failed: %w", err)
	}
	refund.EvidenceURLs = evidenceURLs
	return refund, nil
}

// GetByGatewayIdempotencyKey looks up a refund by the idempotency key used
// when calling the gateway. Returns nil if not found.
func (r *RefundRepositoryImpl) GetByGatewayIdempotencyKey(
	ctx context.Context,
	tx db.Tx,
	key string,
) (*entity.Refund, error) {
	row := tx.QueryRow(ctx, `SELECT `+refundFullColumns+` FROM refunds WHERE gateway_idempotency_key = $1 LIMIT 1`, key)
	refund, err := scanRefund(row)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("get refund by gateway idempotency key failed: %w", err)
	}
	return refund, nil
}

// GetByGatewayRefundID looks up a refund by the gateway-issued refund id.
// Returns nil if not found.
func (r *RefundRepositoryImpl) GetByGatewayRefundID(
	ctx context.Context,
	tx db.Tx,
	gatewayRefundID string,
) (*entity.Refund, error) {
	row := tx.QueryRow(ctx, `SELECT `+refundFullColumns+` FROM refunds WHERE gateway_refund_id = $1 LIMIT 1`, gatewayRefundID)
	refund, err := scanRefund(row)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("get refund by gateway refund id failed: %w", err)
	}
	return refund, nil
}

// GetSuccessfulRefundTotalByOrder returns the cumulative amount of successful
// refunds already recorded for an order. Gateway success is authoritative;
// final_refund_amount is used as the stored canonical amount once success lands.
func (r *RefundRepositoryImpl) GetSuccessfulRefundTotalByOrder(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	excludeRefundID *uuid.UUID,
) (int64, error) {
	var total int64
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(final_refund_amount), 0)
		FROM refunds
		WHERE order_id = $1
		  AND gateway_status = $2
		  AND final_refund_amount IS NOT NULL
		  AND ($3::uuid IS NULL OR id <> $3)
	`, orderID, string(entity.GatewayRefundSucceeded), excludeRefundID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("get successful refund total by order failed: %w", err)
	}
	return total, nil
}

// GetCumulativeProductRefundByOrder returns the cumulative product-portion
// refunded across all gateway-succeeded refunds for the order, excluding the
// given refund ID (typically the in-flight row being processed).
//
// S2C2: refunded_product_amount is the canonical product split once recorded;
// legacy rows (written before migration 000040) fall back to
// final_refund_amount.
func (r *RefundRepositoryImpl) GetCumulativeProductRefundByOrder(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	excludeRefundID *uuid.UUID,
) (int64, error) {
	var total int64
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(COALESCE(refunded_product_amount, final_refund_amount)), 0)
		FROM refunds
		WHERE order_id = $1
		  AND gateway_status = 'succeeded'
		  AND ($2::uuid IS NULL OR id != $2)
	`, orderID, excludeRefundID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("get cumulative product refund by order failed: %w", err)
	}
	return total, nil
}

// GetCumulativeShippingRefundByOrder returns the cumulative shipping-portion
// refunded across all gateway-succeeded refunds for the order, excluding the
// given refund ID (typically the in-flight row being processed).
//
// S2C2: refunded_shipping_amount is the canonical shipping split once
// recorded; legacy rows default to 0.
func (r *RefundRepositoryImpl) GetCumulativeShippingRefundByOrder(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	excludeRefundID *uuid.UUID,
) (int64, error) {
	var total int64
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(COALESCE(refunded_shipping_amount, 0)), 0)
		FROM refunds
		WHERE order_id = $1
		  AND gateway_status = 'succeeded'
		  AND ($2::uuid IS NULL OR id != $2)
	`, orderID, excludeRefundID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("get cumulative shipping refund by order failed: %w", err)
	}
	return total, nil
}

// GetCumulativeCoinsRefundedByOrder returns the cumulative coins restored
// across all gateway-succeeded refunds for the order, excluding the given
// refund ID (typically the in-flight row being processed).
//
// S2C2: coins_refunded_amount is the canonical coin delta once recorded;
// legacy rows default to 0.
func (r *RefundRepositoryImpl) GetCumulativeCoinsRefundedByOrder(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	excludeRefundID *uuid.UUID,
) (int64, error) {
	var total int64
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(COALESCE(coins_refunded_amount, 0)), 0)
		FROM refunds
		WHERE order_id = $1
		  AND gateway_status = 'succeeded'
		  AND ($2::uuid IS NULL OR id != $2)
	`, orderID, excludeRefundID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("get cumulative coins refunded by order failed: %w", err)
	}
	return total, nil
}

// GetForUpdate retrieves a refund with FOR UPDATE lock.
func (r *RefundRepositoryImpl) GetForUpdate(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (*entity.Refund, error) {
	row := tx.QueryRow(ctx, `SELECT `+refundFullColumns+` FROM refunds WHERE id = $1 FOR UPDATE`, id)
	refund, err := scanRefund(row)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, fmt.Errorf("refund not found: %s", id)
		}
		return nil, fmt.Errorf("get refund for update failed: %w", err)
	}
	evidenceURLs, err := r.ListEvidence(ctx, tx, id)
	if err != nil {
		return nil, fmt.Errorf("get refund evidence failed: %w", err)
	}
	refund.EvidenceURLs = evidenceURLs
	return refund, nil
}

// Update updates an existing refund within a transaction.
//
// Writes both legacy buyer/seller-negotiation columns and the additive
// gateway-pipeline columns (000129) atomically.
func (r *RefundRepositoryImpl) Update(
	ctx context.Context,
	tx db.Tx,
	refund *entity.Refund,
) error {
	gatewayStatus := refund.GatewayStatus
	if gatewayStatus == "" {
		gatewayStatus = entity.GatewayRefundUnsubmitted
	}
	_, err := tx.Exec(ctx, `
		UPDATE refunds
		SET status = $2,
		    seller_approved_percent = $3,
		    seller_approved_amount = $4,
		    seller_notes = $5,
		    seller_reviewed_at = $6,
		    admin_approved_percent = $7,
		    admin_approved_amount = $8,
		    admin_notes = $9,
		    reviewed_by = $10,
		    admin_reviewed_at = $11,
		    final_refund_amount = $12,
		    approved_at = $13,
		    rejected_at = $14,
		    refunded_at = $15,
		    updated_at = $16,
		    gateway_refund_id = $17,
		    gateway_status = $18,
		    gateway_attempts = $19,
		    last_gateway_error = $20,
		    gateway_idempotency_key = $21,
		    gateway_requested_at = $22,
		    gateway_acknowledged_at = $23,
		    refunded_product_amount = $24,
		    refunded_shipping_amount = $25,
		    coins_refunded_amount = $26
		WHERE id = $1
	`,
		refund.ID,
		string(refund.Status),
		refund.SellerApprovedPercent,
		refund.SellerApprovedAmount,
		refund.SellerNotes,
		refund.SellerReviewedAt,
		refund.AdminApprovedPercent,
		refund.AdminApprovedAmount,
		refund.AdminNotes,
		nullableUUID(refund.ReviewedBy),
		refund.AdminReviewedAt,
		refund.FinalRefundAmount,
		refund.ApprovedAt,
		refund.RejectedAt,
		refund.RefundedAt,
		refund.UpdatedAt,
		refund.GatewayRefundID,
		string(gatewayStatus),
		refund.GatewayAttempts,
		refund.LastGatewayError,
		refund.GatewayIdempotencyKey,
		refund.GatewayRequestedAt,
		refund.GatewayAcknowledgedAt,
		refund.RefundedProductAmount,
		refund.RefundedShippingAmount,
		refund.CoinsRefundedAmount,
	)

	if err != nil {
		return fmt.Errorf("update refund failed: %w", err)
	}

	return nil
}

// ListByBuyer retrieves refunds for a buyer with pagination.
func (r *RefundRepositoryImpl) ListByBuyer(
	ctx context.Context,
	tx db.Tx,
	buyerID uuid.UUID,
	limit int,
	offset int64,
) ([]*entity.Refund, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, order_id, seller_id, reason, description, status,
		       requested_amount,
		       seller_approved_percent, seller_approved_amount, seller_notes, seller_reviewed_at,
		       admin_approved_percent, admin_approved_amount, admin_notes, reviewed_by, admin_reviewed_at,
		       final_refund_amount,
		       opened_at, approved_at, rejected_at, refunded_at,
		       created_at, updated_at
		FROM refunds
		WHERE buyer_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, buyerID, limit, offset)

	if err != nil {
		return nil, fmt.Errorf("list refunds by buyer failed: %w", err)
	}
	defer rows.Close()

	var refunds []*entity.Refund
	for rows.Next() {
		var id, orderID, sellerID uuid.UUID
		var reason, status string
		var description *string
		var requestedAmount int64
		var sellerApprovedPercent *int
		var sellerApprovedAmount *int64
		var sellerNotes *string
		var sellerReviewedAt *time.Time
		var adminApprovedPercent *int
		var adminApprovedAmount *int64
		var adminNotes *string
		var reviewedBy *uuid.UUID
		var adminReviewedAt *time.Time
		var finalRefundAmount *int64
		var openedAt, approvedAt, rejectedAt, refundedAt *time.Time
		var createdAt, updatedAt time.Time

		if err := rows.Scan(
			&id, &orderID, &sellerID, &reason, &description, &status,
			&requestedAmount,
			&sellerApprovedPercent, &sellerApprovedAmount, &sellerNotes, &sellerReviewedAt,
			&adminApprovedPercent, &adminApprovedAmount, &adminNotes, &reviewedBy, &adminReviewedAt,
			&finalRefundAmount,
			&openedAt, &approvedAt, &rejectedAt, &refundedAt,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan refund failed: %w", err)
		}

		// Handle nullable reviewedBy
		var reviewedByID uuid.UUID
		if reviewedBy != nil {
			reviewedByID = *reviewedBy
		}

		refund := &entity.Refund{
			ID:                    id,
			OrderID:               orderID,
			BuyerID:               buyerID,
			SellerID:              sellerID,
			Reason:                entity.RefundReason(reason),
			Description:           description,
			EvidenceURLs:          []string{}, // Not loaded in list view
			Status:                entity.RefundStatus(status),
			RequestedAmount:       requestedAmount,
			SellerApprovedPercent: sellerApprovedPercent,
			SellerApprovedAmount:  sellerApprovedAmount,
			SellerNotes:           sellerNotes,
			SellerReviewedAt:      sellerReviewedAt,
			AdminApprovedPercent:  adminApprovedPercent,
			AdminApprovedAmount:   adminApprovedAmount,
			AdminNotes:            adminNotes,
			ReviewedBy:            reviewedByID,
			AdminReviewedAt:       adminReviewedAt,
			FinalRefundAmount:     finalRefundAmount,
			OpenedAt:              *openedAt,
			ApprovedAt:            approvedAt,
			RejectedAt:            rejectedAt,
			RefundedAt:            refundedAt,
			CreatedAt:             createdAt,
			UpdatedAt:             updatedAt,
		}

		refunds = append(refunds, refund)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate refunds failed: %w", err)
	}

	return refunds, nil
}

// ListBySeller retrieves refunds for a seller with pagination.
func (r *RefundRepositoryImpl) ListBySeller(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
	limit int,
	offset int64,
) ([]*entity.Refund, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, order_id, buyer_id, reason, description, status,
		       requested_amount,
		       seller_approved_percent, seller_approved_amount, seller_notes, seller_reviewed_at,
		       admin_approved_percent, admin_approved_amount, admin_notes, reviewed_by, admin_reviewed_at,
		       final_refund_amount,
		       opened_at, approved_at, rejected_at, refunded_at,
		       created_at, updated_at
		FROM refunds
		WHERE seller_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, sellerID, limit, offset)

	if err != nil {
		return nil, fmt.Errorf("list refunds by seller failed: %w", err)
	}
	defer rows.Close()

	var refunds []*entity.Refund
	for rows.Next() {
		var id, orderID, buyerID uuid.UUID
		var reason, status string
		var description *string
		var requestedAmount int64
		var sellerApprovedPercent *int
		var sellerApprovedAmount *int64
		var sellerNotes *string
		var sellerReviewedAt *time.Time
		var adminApprovedPercent *int
		var adminApprovedAmount *int64
		var adminNotes *string
		var reviewedBy *uuid.UUID
		var adminReviewedAt *time.Time
		var finalRefundAmount *int64
		var openedAt, approvedAt, rejectedAt, refundedAt *time.Time
		var createdAt, updatedAt time.Time

		if err := rows.Scan(
			&id, &orderID, &buyerID, &reason, &description, &status,
			&requestedAmount,
			&sellerApprovedPercent, &sellerApprovedAmount, &sellerNotes, &sellerReviewedAt,
			&adminApprovedPercent, &adminApprovedAmount, &adminNotes, &reviewedBy, &adminReviewedAt,
			&finalRefundAmount,
			&openedAt, &approvedAt, &rejectedAt, &refundedAt,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan refund failed: %w", err)
		}

		// Handle nullable reviewedBy
		var reviewedByID uuid.UUID
		if reviewedBy != nil {
			reviewedByID = *reviewedBy
		}

		refund := &entity.Refund{
			ID:                    id,
			OrderID:               orderID,
			BuyerID:               buyerID,
			SellerID:              sellerID,
			Reason:                entity.RefundReason(reason),
			Description:           description,
			EvidenceURLs:          []string{}, // Not loaded in list view
			Status:                entity.RefundStatus(status),
			RequestedAmount:       requestedAmount,
			SellerApprovedPercent: sellerApprovedPercent,
			SellerApprovedAmount:  sellerApprovedAmount,
			SellerNotes:           sellerNotes,
			SellerReviewedAt:      sellerReviewedAt,
			AdminApprovedPercent:  adminApprovedPercent,
			AdminApprovedAmount:   adminApprovedAmount,
			AdminNotes:            adminNotes,
			ReviewedBy:            reviewedByID,
			AdminReviewedAt:       adminReviewedAt,
			FinalRefundAmount:     finalRefundAmount,
			OpenedAt:              *openedAt,
			ApprovedAt:            approvedAt,
			RejectedAt:            rejectedAt,
			RefundedAt:            refundedAt,
			CreatedAt:             createdAt,
			UpdatedAt:             updatedAt,
		}

		refunds = append(refunds, refund)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate refunds failed: %w", err)
	}

	return refunds, nil
}

// HasActiveRefundByOrderID returns true if the order has a refund in a
// non-terminal status. Terminal = 'refunded' or 'admin_released'.
// H2-F2a: Used by auto-complete guard to prevent releasing escrow while
// refund negotiation or gateway settlement is in flight.
func (r *RefundRepositoryImpl) HasActiveRefundByOrderID(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM refunds
			WHERE order_id = $1
			  AND status NOT IN ('refunded', 'admin_released')
		)
	`, orderID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check active refund by order_id failed: %w", err)
	}
	return exists, nil
}

// CreateEvidence creates an evidence attachment for a refund.
func (r *RefundRepositoryImpl) CreateEvidence(
	ctx context.Context,
	tx db.Tx,
	refundID uuid.UUID,
	mediaURL string,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO refund_evidence (id, refund_id, media_url, created_at)
		VALUES ($1, $2, $3, NOW())
	`, uuid.New(), refundID, mediaURL)

	if err != nil {
		return fmt.Errorf("create refund evidence failed: %w", err)
	}

	return nil
}

// ListEvidence retrieves all evidence URLs for a refund.
func (r *RefundRepositoryImpl) ListEvidence(
	ctx context.Context,
	tx db.Tx,
	refundID uuid.UUID,
) ([]string, error) {
	rows, err := tx.Query(ctx, `
		SELECT media_url
		FROM refund_evidence
		WHERE refund_id = $1
		ORDER BY created_at ASC
	`, refundID)

	if err != nil {
		return nil, fmt.Errorf("list refund evidence failed: %w", err)
	}
	defer rows.Close()

	var evidenceURLs []string
	for rows.Next() {
		var mediaURL string
		if err := rows.Scan(&mediaURL); err != nil {
			return nil, fmt.Errorf("scan refund evidence failed: %w", err)
		}
		evidenceURLs = append(evidenceURLs, mediaURL)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate refund evidence failed: %w", err)
	}

	return evidenceURLs, nil
}

// ListByOrderID retrieves refunds for a specific order using keyset pagination.
func (r *RefundRepositoryImpl) ListByOrderID(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	limit int,
	cursor *refundRepo.OrderRefundCursor,
) ([]*entity.Refund, error) {
	var rows interface{ Close(); Next() bool; Scan(...any) error }
	var err error
	if cursor == nil {
		rows, err = tx.Query(ctx, `SELECT `+refundFullColumns+` FROM refunds WHERE order_id = $1 ORDER BY created_at DESC, id DESC LIMIT $2`, orderID, limit)
	} else {
		rows, err = tx.Query(ctx, `SELECT `+refundFullColumns+` FROM refunds WHERE order_id = $1 AND (created_at, id) < ($2, $3) ORDER BY created_at DESC, id DESC LIMIT $4`, orderID, cursor.CreatedAt, cursor.ID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list refunds by order: %w", err)
	}
	defer rows.Close()
	return scanRefunds(rows)
}

func scanRefunds(rows interface{ Next() bool; Scan(...any) error }) ([]*entity.Refund, error) {
	var refunds []*entity.Refund
	for rows.Next() {
		r, err := scanRefund(rows)
		if err != nil {
			return nil, err
		}
		refunds = append(refunds, r)
	}
	return refunds, nil
}

// nullableUUID converts a UUID to *uuid.UUID for nullable database fields.
func nullableUUID(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}
