package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/pricing/promotion/contract/entity"
	"github.com/labuda/backend/pkg/db"
)

// FundingIntentRepository persists promotion funding intents.
type FundingIntentRepository struct{}

// NewFundingIntentRepository creates a new repository.
func NewFundingIntentRepository() *FundingIntentRepository {
	return &FundingIntentRepository{}
}

// UpsertIntent atomically inserts a funding intent or returns the existing one
// if a row with the same (seller_id, kind, budget_rupiah, duration_days) already
// exists. This is the canonical race-safe idempotency mechanism:
//
//   - INSERT ... ON CONFLICT DO NOTHING RETURNING id → insertedID is set
//   - If conflict → insertedID is zero → caller reads the existing row
//
// This guarantees that concurrent duplicate requests produce at most one
// intent + one billing obligation.
//
// Returns (insertedID, nil) on success, (uuid.Nil, nil) on conflict.
func (r *FundingIntentRepository) UpsertIntent(ctx context.Context, tx db.Tx, intent *entity.FundingIntent) (uuid.UUID, error) {
	var insertedID uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO promotion_funding_intents
			(id, seller_id, kind, budget_rupiah, duration_days, city_ids, shortage_amount, billing_transaction_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (seller_id, kind, budget_rupiah, duration_days) DO NOTHING
		RETURNING id`,
		intent.ID, intent.SellerID, intent.Kind, intent.BudgetRupiah,
		intent.DurationDays, intent.CityIDs, intent.ShortageAmount,
		intent.BillingTransactionID, intent.CreatedAt,
	).Scan(&insertedID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Conflict — row already exists. Return zero UUID to signal caller.
			return uuid.Nil, nil
		}
		return uuid.Nil, err
	}
	return insertedID, nil
}

// GetByID reads a funding intent by id.
func (r *FundingIntentRepository) GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.FundingIntent, error) {
	var intent entity.FundingIntent
	err := tx.QueryRow(ctx, `
		SELECT id, seller_id, kind, budget_rupiah, duration_days, city_ids,
		       shortage_amount, billing_transaction_id, created_at
		FROM promotion_funding_intents
		WHERE id = $1`, id,
	).Scan(
		&intent.ID, &intent.SellerID, &intent.Kind, &intent.BudgetRupiah,
		&intent.DurationDays, &intent.CityIDs, &intent.ShortageAmount,
		&intent.BillingTransactionID, &intent.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &intent, nil
}

// LinkBilling sets the billing_transaction_id on a funding intent.
func (r *FundingIntentRepository) LinkBilling(ctx context.Context, tx db.Tx, intentID uuid.UUID, billingID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		UPDATE promotion_funding_intents
		SET billing_transaction_id = $1
		WHERE id = $2 AND billing_transaction_id IS NULL`,
		billingID, intentID,
	)
	return err
}

// LatestIntentForSeller returns the most recent intent for a seller, if any.
// Returns nil, nil when no intent exists. Used for idempotency: if the
// latest intent has matching params, it is returned without creating a
// new billing transaction.
func (r *FundingIntentRepository) LatestIntentForSeller(ctx context.Context, tx db.Tx, sellerID uuid.UUID) (*entity.FundingIntent, error) {
	var intent entity.FundingIntent
	err := tx.QueryRow(ctx, `
		SELECT id, seller_id, kind, budget_rupiah, duration_days, city_ids,
		       shortage_amount, billing_transaction_id, created_at
		FROM promotion_funding_intents
		WHERE seller_id = $1
		ORDER BY created_at DESC
		LIMIT 1`, sellerID,
	).Scan(
		&intent.ID, &intent.SellerID, &intent.Kind, &intent.BudgetRupiah,
		&intent.DurationDays, &intent.CityIDs, &intent.ShortageAmount,
		&intent.BillingTransactionID, &intent.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &intent, nil
}
