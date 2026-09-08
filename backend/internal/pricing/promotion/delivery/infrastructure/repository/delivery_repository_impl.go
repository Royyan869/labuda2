package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/pricing/promotion/delivery/entity"
	deliveryRepo "github.com/labuda/backend/internal/pricing/promotion/delivery/repository"
	promoentity "github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/labuda/backend/pkg/db"
)

// DeliveryRepositoryImpl persists canonical delivery tickets and qualified
// impressions.
type DeliveryRepositoryImpl struct {
	db *db.DB
}

// NewDeliveryRepository creates a new DeliveryRepositoryImpl. The db handle
// is used only for the pre-tx ReadTicket pool read (target-eligibility gate);
// every money-critical read/write runs on the caller-provided transaction.
func NewDeliveryRepository(dbConn *db.DB) *DeliveryRepositoryImpl {
	return &DeliveryRepositoryImpl{db: dbConn}
}

var _ deliveryRepo.Repository = (*DeliveryRepositoryImpl)(nil)

// GetDBTime returns the current database time (DB clock authority).
func (r *DeliveryRepositoryImpl) GetDBTime(ctx context.Context, tx db.Tx) (time.Time, error) {
	var dbTime time.Time
	if err := tx.QueryRow(ctx, `SELECT NOW()`).Scan(&dbTime); err != nil {
		return time.Time{}, fmt.Errorf("get database time: %w", err)
	}
	return dbTime, nil
}

func (r *DeliveryRepositoryImpl) CreateTicket(ctx context.Context, tx db.Tx, t *entity.DeliveryTicket) error {
	if t == nil {
		return fmt.Errorf("delivery ticket is nil")
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO promotion_delivery_tickets (
			id, contract_id, target_type, target_id, viewer_id,
			status, issued_at, expires_at, consumed_at, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`,
		t.ID,
		t.ContractID,
		string(t.TargetType),
		t.TargetID,
		t.ViewerID,
		string(t.Status),
		t.IssuedAt,
		t.ExpiresAt,
		t.ConsumedAt,
		t.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create delivery ticket failed: %w", err)
	}
	return nil
}

func (r *DeliveryRepositoryImpl) GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.DeliveryTicket, error) {
	return r.scanTicket(ctx, tx, id, false)
}

// ReadTicket reads a ticket through the pool (no transaction). Used only by
// the qualification pre-flight eligibility gate — see the repository
// interface documentation for why this must not run inside the tx.
func (r *DeliveryRepositoryImpl) ReadTicket(ctx context.Context, id uuid.UUID) (*entity.DeliveryTicket, error) {
	var t entity.DeliveryTicket
	var targetType, status string
	err := r.db.Pool().QueryRow(ctx, `
		SELECT id, contract_id, target_type, target_id, viewer_id,
		       status, issued_at, expires_at, consumed_at, created_at
		FROM promotion_delivery_tickets
		WHERE id = $1
	`, id).Scan(
		&t.ID, &t.ContractID, &targetType, &t.TargetID, &t.ViewerID,
		&status, &t.IssuedAt, &t.ExpiresAt, &t.ConsumedAt, &t.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows || err.Error() == "no rows in result set" {
			return nil, deliveryRepo.ErrTicketNotFound
		}
		return nil, fmt.Errorf("read delivery ticket failed: %w", err)
	}
	t.TargetType = promoentity.TargetType(targetType)
	t.Status = entity.TicketStatus(status)
	return &t, nil
}

func (r *DeliveryRepositoryImpl) GetForUpdate(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.DeliveryTicket, error) {
	return r.scanTicket(ctx, tx, id, true)
}

func (r *DeliveryRepositoryImpl) scanTicket(ctx context.Context, tx db.Tx, id uuid.UUID, forUpdate bool) (*entity.DeliveryTicket, error) {
	query := `
		SELECT id, contract_id, target_type, target_id, viewer_id,
		       status, issued_at, expires_at, consumed_at, created_at
		FROM promotion_delivery_tickets
		WHERE id = $1
	`
	if forUpdate {
		query += ` FOR UPDATE`
	}

	var t entity.DeliveryTicket
	var targetType, status string
	err := tx.QueryRow(ctx, query, id).Scan(
		&t.ID, &t.ContractID, &targetType, &t.TargetID, &t.ViewerID,
		&status, &t.IssuedAt, &t.ExpiresAt, &t.ConsumedAt, &t.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows || err.Error() == "no rows in result set" {
			return nil, deliveryRepo.ErrTicketNotFound
		}
		return nil, fmt.Errorf("get delivery ticket failed: %w", err)
	}
	t.TargetType = promoentity.TargetType(targetType)
	t.Status = entity.TicketStatus(status)
	return &t, nil
}

func (r *DeliveryRepositoryImpl) MarkConsumed(ctx context.Context, tx db.Tx, id uuid.UUID, consumedAt time.Time) error {
	tag, err := tx.Exec(ctx, `
		UPDATE promotion_delivery_tickets
		SET status = 'consumed',
		    consumed_at = $2
		WHERE id = $1
		  AND status = 'issued'
	`, id, consumedAt)
	if err != nil {
		return fmt.Errorf("mark delivery ticket consumed failed: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("mark delivery ticket consumed: ticket %s is not in issued state", id)
	}
	return nil
}

func (r *DeliveryRepositoryImpl) InvalidateIssuedForContract(ctx context.Context, tx db.Tx, contractID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		UPDATE promotion_delivery_tickets
		SET status = 'invalidated'
		WHERE contract_id = $1
		  AND status = 'issued'
	`, contractID)
	if err != nil {
		return fmt.Errorf("invalidate issued delivery tickets for contract %s: %w", contractID, err)
	}
	return nil
}

func (r *DeliveryRepositoryImpl) CountQualifiedImpressions(ctx context.Context, tx db.Tx, contractID uuid.UUID) (int64, error) {
	var count int64
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM promotion_qualified_impressions WHERE contract_id = $1
	`, contractID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count qualified impressions failed: %w", err)
	}
	return count, nil
}

func (r *DeliveryRepositoryImpl) CreateQualifiedImpression(ctx context.Context, tx db.Tx, qi *entity.QualifiedImpression) error {
	if qi == nil {
		return fmt.Errorf("qualified impression is nil")
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO promotion_qualified_impressions (
			id, ticket_id, contract_id, allocation_account_id,
			target_type, target_id, sequence_n, charge_rupiah,
			server_occurred_at, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`,
		qi.ID,
		qi.TicketID,
		qi.ContractID,
		qi.AllocationAccountID,
		string(qi.TargetType),
		qi.TargetID,
		qi.SequenceN,
		qi.ChargeRupiah,
		qi.ServerOccurredAt,
		qi.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create qualified impression failed: %w", err)
	}
	return nil
}
