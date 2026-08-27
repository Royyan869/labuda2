package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	alertentity "github.com/labuda/backend/internal/platform/alert/entity"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// Seller non-shipment detection thresholds
const (
	// SellerNonShipmentWindowDays is the rolling window for non-shipment detection
	SellerNonShipmentWindowDays = 30

	// SellerNonShipmentCountThreshold is the minimum cancelled_timeout orders to trigger alert
	SellerNonShipmentCountThreshold = 3

	// SellerNonShipmentRateThreshold is the maximum fulfillment rate (below this triggers alert)
	SellerNonShipmentRateThreshold = 80 // percent

	// SellerNonShipmentRateMinOrders is the minimum terminal orders required for rate-based detection
	SellerNonShipmentRateMinOrders = 10
)

// sellerShipmentMetrics holds per-seller order metrics for non-shipment detection.
type sellerShipmentMetrics struct {
	SellerID              uuid.UUID
	CancelledTimeoutCount int
	FulfilledCount        int
	TotalTerminalOrders   int
	FulfillmentRate       float64
}

// SellerNonShipmentRule detects sellers with repeated non-shipment patterns.
//
// BUSINESS RULE:
// Alert (no automatic consequence) when seller has:
//   - >=3 cancelled_timeout orders in last 30 days  OR
//   - fulfillment_rate < 80% in last 30 days AND total terminal orders >= 10
//
// Queries the orders table directly for 30-day granularity.
type SellerNonShipmentRule struct {
	db  db.Transactor
	log *zap.Logger
}

// NewSellerNonShipmentRule creates a new seller non-shipment detection rule.
func NewSellerNonShipmentRule(db db.Transactor, log *zap.Logger) *SellerNonShipmentRule {
	return &SellerNonShipmentRule{db: db, log: log}
}

func (r *SellerNonShipmentRule) Name() string {
	return "seller_non_shipment"
}

func (r *SellerNonShipmentRule) Detect(ctx context.Context, tx db.Tx) (bool, *AnomalyFinding, error) {
	windowStart := time.Now().AddDate(0, 0, -SellerNonShipmentWindowDays)

	// Single query: compute per-seller cancelled_timeout count, fulfilled count,
	// and total terminal orders in the 30-day window.
	//
	// Terminal statuses: completed, delivered, cancelled_timeout, refunded, partially_refunded, cancelled, expired
	// Fulfilled statuses: completed, delivered, partially_refunded (seller shipped, buyer received)
	rows, err := tx.Query(ctx, `
		SELECT
			seller_id,
			COUNT(*) FILTER (WHERE status = 'cancelled_timeout') AS cancelled_timeout_count,
			COUNT(*) FILTER (WHERE status IN ('completed', 'delivered', 'partially_refunded')) AS fulfilled_count,
			COUNT(*) AS total_terminal_orders
		FROM orders
		WHERE status IN ('completed', 'delivered', 'cancelled_timeout', 'refunded', 'partially_refunded', 'cancelled', 'expired')
		  AND updated_at >= $1
		GROUP BY seller_id
		HAVING
			COUNT(*) FILTER (WHERE status = 'cancelled_timeout') >= $2
			OR (
				COUNT(*) >= $3
				AND COUNT(*) FILTER (WHERE status IN ('completed', 'delivered', 'partially_refunded')) * 100.0 / COUNT(*) < $4
			)
		ORDER BY COUNT(*) FILTER (WHERE status = 'cancelled_timeout') DESC
		LIMIT 10
	`, windowStart, SellerNonShipmentCountThreshold, SellerNonShipmentRateMinOrders, SellerNonShipmentRateThreshold)

	if err != nil {
		return false, nil, fmt.Errorf("query seller non-shipment: %w", err)
	}
	defer rows.Close()

	var flaggedSellers []sellerShipmentMetrics
	for rows.Next() {
		var m sellerShipmentMetrics
		if err := rows.Scan(&m.SellerID, &m.CancelledTimeoutCount, &m.FulfilledCount, &m.TotalTerminalOrders); err != nil {
			continue
		}
		if m.TotalTerminalOrders > 0 {
			m.FulfillmentRate = float64(m.FulfilledCount) * 100.0 / float64(m.TotalTerminalOrders)
		}
		flaggedSellers = append(flaggedSellers, m)
	}

	if len(flaggedSellers) == 0 {
		return false, nil, nil
	}

	// Alert for worst offender
	seller := flaggedSellers[0]
	groupKey := fmt.Sprintf("seller_non_shipment:%s", seller.SellerID.String())

	metadata := alertentity.AlertMetadata{
		"seller_id":               seller.SellerID.String(),
		"cancelled_timeout_count": seller.CancelledTimeoutCount,
		"fulfilled_count":         seller.FulfilledCount,
		"fulfillment_rate":        seller.FulfillmentRate,
		"total_terminal_orders":   seller.TotalTerminalOrders,
		"window_days":             SellerNonShipmentWindowDays,
		"count_threshold":         SellerNonShipmentCountThreshold,
		"rate_threshold":          SellerNonShipmentRateThreshold,
		"rate_min_orders":         SellerNonShipmentRateMinOrders,
		"all_flagged_sellers":     flaggedSellers,
		"detected_at":             time.Now(),
	}

	severity := alertentity.SeverityWarning
	if seller.CancelledTimeoutCount >= SellerNonShipmentCountThreshold*2 {
		severity = alertentity.SeverityHigh
	}

	return true, &AnomalyFinding{
		AlertType:  alertentity.AlertTypeSellerNonShipment,
		Severity:   severity,
		EntityType: "seller",
		EntityID:   seller.SellerID,
		Message: fmt.Sprintf("Seller non-shipment: %d cancelled_timeout orders, fulfillment rate %.1f%% (%d/%d terminal) in %d days",
			seller.CancelledTimeoutCount, seller.FulfillmentRate, seller.FulfilledCount, seller.TotalTerminalOrders, SellerNonShipmentWindowDays),
		Metadata: metadata,
		GroupKey: &groupKey,
	}, nil
}


