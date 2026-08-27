package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance/billing/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

// BillingRepository handles billing transaction persistence using pgx-based DB layer.
// It enforces row-level locking for concurrent state transitions.
type BillingRepository struct{}

// NewBillingRepository creates a new BillingRepository.
func NewBillingRepository() *BillingRepository {
	return &BillingRepository{}
}

// CreateBillingTransaction persists a new billing transaction within a transaction.
func (r *BillingRepository) CreateBillingTransaction(
	ctx context.Context,
	tx db.Tx,
	billing *entity.BillingTransaction,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO billing_transactions (
			id, payer_id, target_id, type, gross_amount,
			platform_fee_percent, platform_fee_amount, net_amount,
			status, event_date, unlock_date, unlocked_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`,
		billing.ID,
		billing.PayerID,
		billing.TargetID,
		string(billing.Type),
		billing.GrossAmount.Int64(),
		billing.PlatformFeePercent,
		billing.PlatformFeeAmount.Int64(),
		billing.NetAmount.Int64(),
		string(billing.Status),
		billing.EventDate,
		billing.UnlockDate,
		billing.UnlockedAt,
		billing.CreatedAt,
		billing.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("create billing transaction failed: %w", err)
	}

	return nil
}

// GetByID retrieves a billing transaction without locking (for read-only operations).
func (r *BillingRepository) GetByID(
	ctx context.Context,
	tx db.Tx,
	billingID uuid.UUID,
) (*entity.BillingTransaction, error) {
	return r.scanBillingTransaction(ctx, tx, billingID, false)
}

// GetForUpdate retrieves a billing transaction with FOR UPDATE lock.
// This prevents concurrent modifications and must be used within a transaction.
func (r *BillingRepository) GetForUpdate(
	ctx context.Context,
	tx db.Tx,
	billingID uuid.UUID,
) (*entity.BillingTransaction, error) {
	return r.scanBillingTransaction(ctx, tx, billingID, true)
}

// scanBillingTransaction is a helper that scans a billing transaction from the database.
// If forUpdate is true, it uses FOR UPDATE lock.
func (r *BillingRepository) scanBillingTransaction(
	ctx context.Context,
	tx db.Tx,
	billingID uuid.UUID,
	forUpdate bool,
) (*entity.BillingTransaction, error) {
	var id, payerID, targetID uuid.UUID
	var grossAmount, platformFeePercent, platformFeeAmount, netAmount int64
	var billingType, status string
	var eventDate, unlockDate, unlockedAt *time.Time
	var createdAt, updatedAt time.Time

	query := `
		SELECT id, payer_id, target_id, type, gross_amount,
		       platform_fee_percent, platform_fee_amount, net_amount,
		       status, event_date, unlock_date, unlocked_at, created_at, updated_at
		FROM billing_transactions
		WHERE id = $1
	`
	if forUpdate {
		query += " FOR UPDATE"
	}

	err := tx.QueryRow(ctx, query, billingID).Scan(
		&id, &payerID, &targetID, &billingType, &grossAmount,
		&platformFeePercent, &platformFeeAmount, &netAmount,
		&status, &eventDate, &unlockDate, &unlockedAt, &createdAt, &updatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, fmt.Errorf("billing transaction not found: %s", billingID)
		}
		return nil, fmt.Errorf("get billing transaction failed: %w", err)
	}

	billing := &entity.BillingTransaction{
		ID:                 id,
		PayerID:            payerID,
		TargetID:           targetID,
		Type:               entity.Type(billingType),
		GrossAmount:        money.New(grossAmount),
		PlatformFeePercent: platformFeePercent,
		PlatformFeeAmount:  money.New(platformFeeAmount),
		NetAmount:          money.New(netAmount),
		Status:             entity.Status(status),
		EventDate:          eventDate,
		UnlockDate:         unlockDate,
		UnlockedAt:         unlockedAt,
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
	}

	return billing, nil
}

// UpdateStatus persists billing transaction status changes.
func (r *BillingRepository) UpdateStatus(
	ctx context.Context,
	tx db.Tx,
	billing *entity.BillingTransaction,
) error {
	now := time.Now()

	_, err := tx.Exec(ctx, `
		UPDATE billing_transactions
		SET status = $2, event_date = $3, unlock_date = $4, updated_at = $5
		WHERE id = $1
	`,
		billing.ID,
		string(billing.Status),
		billing.EventDate,
		billing.UnlockDate,
		now,
	)

	if err != nil {
		return fmt.Errorf("update billing transaction status failed: %w", err)
	}

	return nil
}

// GetStatus retrieves the current status of a billing transaction.
// Useful for idempotency checks.
func (r *BillingRepository) GetStatus(
	ctx context.Context,
	tx db.Tx,
	billingID uuid.UUID,
) (entity.Status, error) {
	var status string

	err := tx.QueryRow(ctx,
		`SELECT status FROM billing_transactions WHERE id = $1`, billingID,
	).Scan(&status)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return "", fmt.Errorf("billing transaction not found: %s", billingID)
		}
		return "", fmt.Errorf("get billing transaction status failed: %w", err)
	}

	return entity.Status(status), nil
}

// GetByPayerID retrieves all billing transactions for a payer, ordered by created_at desc.
func (r *BillingRepository) GetByPayerID(
	ctx context.Context,
	tx db.Tx,
	payerID uuid.UUID,
	limit int,
) ([]*entity.BillingTransaction, error) {
	query := `
		SELECT id, payer_id, target_id, type, gross_amount,
		       platform_fee_percent, platform_fee_amount, net_amount,
		       status, event_date, unlock_date, unlocked_at, created_at, updated_at
		FROM billing_transactions
		WHERE payer_id = $1
		ORDER BY created_at DESC
	`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := tx.Query(ctx, query, payerID)
	if err != nil {
		return nil, fmt.Errorf("query billing transactions failed: %w", err)
	}
	defer rows.Close()

	var transactions []*entity.BillingTransaction
	for rows.Next() {
		var id, targetID uuid.UUID
		var grossAmount, platformFeePercent, platformFeeAmount, netAmount int64
		var billingType, status string
		var eventDate, unlockDate, unlockedAt *time.Time
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&id, &payerID, &targetID, &billingType, &grossAmount,
			&platformFeePercent, &platformFeeAmount, &netAmount,
			&status, &eventDate, &unlockDate, &unlockedAt, &createdAt, &updatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("scan billing transaction failed: %w", err)
		}

		billing := &entity.BillingTransaction{
			ID:                 id,
			PayerID:            payerID,
			TargetID:           targetID,
			Type:               entity.Type(billingType),
			GrossAmount:        money.New(grossAmount),
			PlatformFeePercent: platformFeePercent,
			PlatformFeeAmount:  money.New(platformFeeAmount),
			NetAmount:          money.New(netAmount),
			Status:             entity.Status(status),
			EventDate:          eventDate,
			UnlockDate:         unlockDate,
			UnlockedAt:         unlockedAt,
			CreatedAt:          createdAt,
			UpdatedAt:          updatedAt,
		}

		transactions = append(transactions, billing)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate billing transactions failed: %w", err)
	}

	return transactions, nil
}

// GetByTargetID retrieves billing transactions by target ID.
func (r *BillingRepository) GetByTargetID(
	ctx context.Context,
	tx db.Tx,
	targetID uuid.UUID,
) ([]*entity.BillingTransaction, error) {
	query := `
		SELECT id, payer_id, target_id, type, gross_amount,
		       platform_fee_percent, platform_fee_amount, net_amount,
		       status, event_date, unlock_date, unlocked_at, created_at, updated_at
		FROM billing_transactions
		WHERE target_id = $1
		ORDER BY created_at DESC
	`

	rows, err := tx.Query(ctx, query, targetID)
	if err != nil {
		return nil, fmt.Errorf("query billing transactions by target failed: %w", err)
	}
	defer rows.Close()

	var transactions []*entity.BillingTransaction
	for rows.Next() {
		var id, payerID uuid.UUID
		var grossAmount, platformFeePercent, platformFeeAmount, netAmount int64
		var billingType, status string
		var eventDate, unlockDate, unlockedAt *time.Time
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&id, &payerID, &targetID, &billingType, &grossAmount,
			&platformFeePercent, &platformFeeAmount, &netAmount,
			&status, &eventDate, &unlockDate, &unlockedAt, &createdAt, &updatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("scan billing transaction failed: %w", err)
		}

		billing := &entity.BillingTransaction{
			ID:                 id,
			PayerID:            payerID,
			TargetID:           targetID,
			Type:               entity.Type(billingType),
			GrossAmount:        money.New(grossAmount),
			PlatformFeePercent: platformFeePercent,
			PlatformFeeAmount:  money.New(platformFeeAmount),
			NetAmount:          money.New(netAmount),
			Status:             entity.Status(status),
			EventDate:          eventDate,
			UnlockDate:         unlockDate,
			UnlockedAt:         unlockedAt,
			CreatedAt:          createdAt,
			UpdatedAt:          updatedAt,
		}

		transactions = append(transactions, billing)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate billing transactions failed: %w", err)
	}

	return transactions, nil
}



