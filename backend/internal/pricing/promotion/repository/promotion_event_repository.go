package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/labuda/backend/pkg/db"
)

// PromotionEventRepository is the analytics store for promotion interaction events.
// It is intentionally separate from PromotionRepository because:
//   - Events are append-only and never mutated after insert.
//   - Summary reads are derived directly from the append-only event log.
//   - The handler records events in a lightweight single-statement transaction.
type PromotionEventRepository interface {
	// RecordEvent appends an analytics event to promotion_events.
	RecordEvent(ctx context.Context, tx db.Tx, event *entity.PromotionEvent) error

	// GetCampaignAnalytics aggregates click/impression metrics for a campaign.
	// from/to are optional inclusive window bounds; nil means all-time.
	GetCampaignAnalytics(
		ctx context.Context,
		tx db.Tx,
		instanceID uuid.UUID,
		from *time.Time,
		to *time.Time,
	) (*PromotionEventAnalyticsSummary, error)
}

// PromotionEventAnalyticsSummary is the minimal read model for promotion analytics.
type PromotionEventAnalyticsSummary struct {
	InstanceID         uuid.UUID  `json:"instance_id"`
	WindowFrom         *time.Time `json:"window_from,omitempty"`
	WindowTo           *time.Time `json:"window_to,omitempty"`
	ImpressionsTotal   int        `json:"impressions_total"`
	ClicksTotal        int        `json:"clicks_total"`
	CTR                float64    `json:"ctr"`
	FeedImpressions    int        `json:"feed_impressions"`
	FeedClicks         int        `json:"feed_clicks"`
	SearchImpressions  int        `json:"search_impressions"`
	SearchClicks       int        `json:"search_clicks"`
	ExploreImpressions int        `json:"explore_impressions"`
	ExploreClicks      int        `json:"explore_clicks"`
}
