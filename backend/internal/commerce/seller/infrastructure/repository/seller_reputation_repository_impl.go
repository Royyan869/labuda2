package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	sellerEntity "github.com/labuda/backend/internal/commerce/seller/entity"
	"github.com/labuda/backend/pkg/db"
)

// UpsertReputationStateTx creates or overwrites the live reputation state for a seller.
//
// Uses INSERT ... ON CONFLICT (seller_id) DO UPDATE so repeated calls are safe.
// The caller (SellerReputationRecomputeWorker) runs this inside a per-seller
// transaction alongside UpdateTierTx, ensuring reputation state and tier badge
// are always written atomically.
func (r *SellerRepositoryImpl) UpsertReputationStateTx(
	ctx context.Context,
	tx db.Tx,
	state *sellerEntity.SellerReputationState,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO seller_reputation_state (
			seller_id,
			window_days,
			window_start,
			window_end,
			rolling_completed_orders,
			rolling_cancelled_timeout,
			rolling_rating_average,
			rolling_rating_count,
			rolling_dispute_loss_count,
			rolling_fulfillment_rate,
			current_tier,
			tier_last_evaluated_at,
			reputation_updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (seller_id) DO UPDATE SET
			window_days               = EXCLUDED.window_days,
			window_start              = EXCLUDED.window_start,
			window_end                = EXCLUDED.window_end,
			rolling_completed_orders  = EXCLUDED.rolling_completed_orders,
			rolling_cancelled_timeout = EXCLUDED.rolling_cancelled_timeout,
			rolling_rating_average    = EXCLUDED.rolling_rating_average,
			rolling_rating_count      = EXCLUDED.rolling_rating_count,
			rolling_dispute_loss_count = EXCLUDED.rolling_dispute_loss_count,
			rolling_fulfillment_rate  = EXCLUDED.rolling_fulfillment_rate,
			current_tier              = EXCLUDED.current_tier,
			tier_last_evaluated_at    = EXCLUDED.tier_last_evaluated_at,
			reputation_updated_at     = EXCLUDED.reputation_updated_at
	`,
		state.SellerID,
		state.WindowDays,
		state.WindowStart,
		state.WindowEnd,
		state.RollingCompletedOrders,
		state.RollingCancelledTimeout,
		state.RollingRatingAverage,
		state.RollingRatingCount,
		state.RollingDisputeLossCount,
		state.RollingFulfillmentRate,
		string(state.CurrentTier),
		state.TierLastEvaluatedAt,
		state.ReputationUpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert seller reputation state failed: %w", err)
	}
	return nil
}

// GetReputationStateForUpdate retrieves the current live reputation state
// with a row-level FOR UPDATE lock. Returns nil if the seller has no state
// row yet (first recompute has not run for this seller).
func (r *SellerRepositoryImpl) GetReputationStateForUpdate(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
) (*sellerEntity.SellerReputationState, error) {
	var (
		windowDays              int
		windowStart, windowEnd  time.Time
		completedOrders         int
		cancelledTimeout        int
		ratingAvg               float64
		ratingCount             int
		disputeLoss             int
		fulfillmentRate         float64
		currentTier             string
		tierEvaluatedAt         *time.Time
		reputationUpdatedAt     time.Time
	)

	err := tx.QueryRow(ctx, `
		SELECT
			window_days,
			window_start,
			window_end,
			rolling_completed_orders,
			rolling_cancelled_timeout,
			rolling_rating_average,
			rolling_rating_count,
			rolling_dispute_loss_count,
			rolling_fulfillment_rate,
			current_tier,
			tier_last_evaluated_at,
			reputation_updated_at
		FROM seller_reputation_state
		WHERE seller_id = $1
		FOR UPDATE
	`, sellerID).Scan(
		&windowDays,
		&windowStart,
		&windowEnd,
		&completedOrders,
		&cancelledTimeout,
		&ratingAvg,
		&ratingCount,
		&disputeLoss,
		&fulfillmentRate,
		&currentTier,
		&tierEvaluatedAt,
		&reputationUpdatedAt,
	)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("get seller reputation state for update failed: %w", err)
	}

	return &sellerEntity.SellerReputationState{
		SellerID:                sellerID,
		WindowDays:              windowDays,
		WindowStart:             windowStart,
		WindowEnd:               windowEnd,
		RollingCompletedOrders:  completedOrders,
		RollingCancelledTimeout: cancelledTimeout,
		RollingRatingAverage:    ratingAvg,
		RollingRatingCount:      ratingCount,
		RollingDisputeLossCount: disputeLoss,
		RollingFulfillmentRate:  fulfillmentRate,
		CurrentTier:             sellerEntity.Tier(currentTier),
		TierLastEvaluatedAt:     tierEvaluatedAt,
		ReputationUpdatedAt:     reputationUpdatedAt,
	}, nil
}


