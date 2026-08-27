package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/pricing/promotion/entity"
	promotionRepo "github.com/labuda/backend/internal/pricing/promotion/repository"
	"github.com/labuda/backend/pkg/db"
)

// PromotionEventRepositoryImpl implements PromotionEventRepository using PostgreSQL.
type PromotionEventRepositoryImpl struct{}

// NewPromotionEventRepository creates a new PromotionEventRepositoryImpl.
func NewPromotionEventRepository() *PromotionEventRepositoryImpl {
	return &PromotionEventRepositoryImpl{}
}

// RecordEvent inserts a promotion analytics event into promotion_events.
// The table has no FK constraint so this INSERT always succeeds if the event
// struct is well-formed.
func (r *PromotionEventRepositoryImpl) RecordEvent(ctx context.Context, tx db.Tx, ev *entity.PromotionEvent) error {
	const query = `
		INSERT INTO promotion_events
			(id, instance_id, target_type, target_id, owner_user_id, viewer_user_id, event_type, surface, occurred_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := tx.Exec(ctx, query,
		ev.ID,
		ev.InstanceID,
		string(ev.TargetType),
		ev.TargetID, // *uuid.UUID — pgx encodes nil pointer as SQL NULL
		ev.OwnerUserID,
		ev.ViewerUserID,
		string(ev.EventType),
		string(ev.Surface),
		ev.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf("promotion event insert: %w", err)
	}
	return nil
}

// GetCampaignAnalytics aggregates the append-only promotion_events log for a campaign.
func (r *PromotionEventRepositoryImpl) GetCampaignAnalytics(
	ctx context.Context,
	tx db.Tx,
	instanceID uuid.UUID,
	from *time.Time,
	to *time.Time,
) (*promotionRepo.PromotionEventAnalyticsSummary, error) {
	query := `
		SELECT
			COUNT(*) FILTER (WHERE event_type = 'impression') AS impressions_total,
			COUNT(*) FILTER (WHERE event_type = 'click') AS clicks_total,
			COUNT(*) FILTER (WHERE event_type = 'impression' AND surface = 'feed') AS feed_impressions,
			COUNT(*) FILTER (WHERE event_type = 'click' AND surface = 'feed') AS feed_clicks,
			COUNT(*) FILTER (WHERE event_type = 'impression' AND surface = 'search') AS search_impressions,
			COUNT(*) FILTER (WHERE event_type = 'click' AND surface = 'search') AS search_clicks,
			COUNT(*) FILTER (WHERE event_type = 'impression' AND surface = 'explore') AS explore_impressions,
			COUNT(*) FILTER (WHERE event_type = 'click' AND surface = 'explore') AS explore_clicks
		FROM promotion_events
		WHERE instance_id = $1
		  AND ($2::timestamptz IS NULL OR occurred_at >= $2)
		  AND ($3::timestamptz IS NULL OR occurred_at <= $3)
	`

	var impressionsTotal, clicksTotal int
	var feedImpressions, feedClicks int
	var searchImpressions, searchClicks int
	var exploreImpressions, exploreClicks int

	err := tx.QueryRow(ctx, query, instanceID, from, to).Scan(
		&impressionsTotal,
		&clicksTotal,
		&feedImpressions,
		&feedClicks,
		&searchImpressions,
		&searchClicks,
		&exploreImpressions,
		&exploreClicks,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return &promotionRepo.PromotionEventAnalyticsSummary{
				InstanceID:         instanceID,
				WindowFrom:         from,
				WindowTo:           to,
				ImpressionsTotal:   0,
				ClicksTotal:        0,
				CTR:                0,
				FeedImpressions:    0,
				FeedClicks:         0,
				SearchImpressions:  0,
				SearchClicks:       0,
				ExploreImpressions: 0,
				ExploreClicks:      0,
			}, nil
		}
		return nil, fmt.Errorf("promotion analytics aggregate: %w", err)
	}

	summary := &promotionRepo.PromotionEventAnalyticsSummary{
		InstanceID:         instanceID,
		WindowFrom:         from,
		WindowTo:           to,
		ImpressionsTotal:   impressionsTotal,
		ClicksTotal:        clicksTotal,
		FeedImpressions:    feedImpressions,
		FeedClicks:         feedClicks,
		SearchImpressions:  searchImpressions,
		SearchClicks:       searchClicks,
		ExploreImpressions: exploreImpressions,
		ExploreClicks:      exploreClicks,
	}
	if impressionsTotal > 0 {
		summary.CTR = float64(clicksTotal) / float64(impressionsTotal)
	}
	return summary, nil
}
