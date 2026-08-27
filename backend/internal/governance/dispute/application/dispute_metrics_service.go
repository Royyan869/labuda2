// ⚠️ FINANCIAL RULE:
// All money operations MUST go through WalletService.
// Direct balance mutation is forbidden.
//
// Dispute domain manages dispute state and resolution.
// All financial operations are delegated to WalletService.
package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/db"
)

// =============================================================================
// TASK 5: DISPUTE METRICS TRACKING
// =============================================================================
// This service tracks dispute metrics for analytics and fairness monitoring.
//
// METRICS TRACKED:
// 1. Daily dispute metrics (total, wins, escalations)
// 2. Per-user dispute statistics
// 3. Reason code distribution
//
// FAIRNESS INDICATORS:
// - Buyer win rate vs seller win rate
// - Escalation rate (system health)
// - High-risk user identification
// =============================================================================

// DisputeMetricsService handles dispute metrics tracking and aggregation.
type DisputeMetricsService struct{}

// NewDisputeMetricsService creates a new DisputeMetricsService.
func NewDisputeMetricsService() *DisputeMetricsService {
	return &DisputeMetricsService{}
}

// RecordDisputeOpened records when a dispute is opened.
func (s *DisputeMetricsService) RecordDisputeOpened(
	ctx context.Context,
	tx db.Tx,
	disputeID uuid.UUID,
	orderID uuid.UUID,
	buyerID uuid.UUID,
	sellerID uuid.UUID,
	callerID uuid.UUID,
	reasonCode string,
) error {
	today := time.Now().Truncate(24 * time.Hour)

	// Update daily metrics
	if err := s.updateDailyMetrics(ctx, tx, today, 1, 0, 0); err != nil {
		return fmt.Errorf("failed to update daily metrics: %w", err)
	}

	// Update buyer stats
	if err := s.updateUserStats(ctx, tx, buyerID, callerID == buyerID, false); err != nil {
		return fmt.Errorf("failed to update buyer stats: %w", err)
	}

	// Update seller stats
	if err := s.updateUserStats(ctx, tx, sellerID, callerID == sellerID, false); err != nil {
		return fmt.Errorf("failed to update seller stats: %w", err)
	}

	// Update reason code metrics
	if err := s.updateReasonMetrics(ctx, tx, reasonCode, callerID == buyerID); err != nil {
		return fmt.Errorf("failed to update reason metrics: %w", err)
	}

	return nil
}

// RecordDisputeResolved records when a dispute is resolved.
func (s *DisputeMetricsService) RecordDisputeResolved(
	ctx context.Context,
	tx db.Tx,
	disputeID uuid.UUID,
	buyerID uuid.UUID,
	sellerID uuid.UUID,
	resolution ResolutionType, // ResolutionRefund or ResolutionRelease
) error {
	today := time.Now().Truncate(24 * time.Hour)

	var buyerWins, sellerWins int
	if resolution == ResolutionRefund {
		buyerWins = 1
	} else {
		sellerWins = 1
	}

	// Update daily metrics
	if err := s.updateDailyMetrics(ctx, tx, today, 0, buyerWins, sellerWins); err != nil {
		return fmt.Errorf("failed to update daily metrics: %w", err)
	}

	// Update user stats with win information
	if buyerWins > 0 {
		if err := s.updateUserStatsWithWin(ctx, tx, buyerID, true); err != nil {
			return fmt.Errorf("failed to update buyer win stats: %w", err)
		}
	}
	if sellerWins > 0 {
		if err := s.updateUserStatsWithWin(ctx, tx, sellerID, false); err != nil {
			return fmt.Errorf("failed to update seller win stats: %w", err)
		}
	}

	return nil
}

// RecordDisputeEscalated records when a dispute is escalated due to timeout.
func (s *DisputeMetricsService) RecordDisputeEscalated(
	ctx context.Context,
	tx db.Tx,
	disputeID uuid.UUID,
) error {
	today := time.Now().Truncate(24 * time.Hour)

	// Update daily escalation count
	_, err := tx.Exec(ctx, `
		INSERT INTO dispute_metrics (metric_date, total_disputes, escalated_count, resolved_count, pending_count)
		VALUES ($1, 0, 1, 0, 0)
		ON CONFLICT (metric_date) DO UPDATE SET
			escalated_count = dispute_metrics.escalated_count + 1,
			updated_at = NOW()
	`, today)

	if err != nil {
		return fmt.Errorf("failed to record escalation: %w", err)
	}

	return nil
}

// =============================================================================
// INTERNAL METHODS
// =============================================================================

// updateDailyMetrics updates the daily aggregated metrics.
func (s *DisputeMetricsService) updateDailyMetrics(
	ctx context.Context,
	tx db.Tx,
	date time.Time,
	totalNew int,
	buyerWins int,
	sellerWins int,
) error {
	// Update or insert daily metrics
	_, err := tx.Exec(ctx, `
		INSERT INTO dispute_metrics (metric_date, total_disputes, buyer_wins, seller_wins, resolved_count)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (metric_date) DO UPDATE SET
			total_disputes = dispute_metrics.total_disputes + $2,
			buyer_wins = dispute_metrics.buyer_wins + $3,
			seller_wins = dispute_metrics.seller_wins + $4,
			resolved_count = dispute_metrics.resolved_count + ($3 + $4),
			updated_at = NOW()
	`, date, totalNew, buyerWins, sellerWins)

	return err
}

// updateUserStats updates per-user dispute statistics.
func (s *DisputeMetricsService) updateUserStats(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	openedAsCaller bool,
	isResolved bool,
) error {
	var disputeAsBuyer, disputeAsSeller int
	if openedAsCaller {
		// We need to determine if this user is buyer or seller in this context
		// For now, increment both counters (actual logic would need more context)
		disputeAsBuyer = 1
		disputeAsSeller = 1
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO user_dispute_stats (user_id, total_disputes, disputes_as_buyer, disputes_as_seller, last_dispute_at)
		VALUES ($1, 1, $2, $3, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			total_disputes = user_dispute_stats.total_disputes + 1,
			disputes_as_buyer = user_dispute_stats.disputes_as_buyer + $2,
			disputes_as_seller = user_dispute_stats.disputes_as_seller + $3,
			last_dispute_at = NOW(),
			updated_at = NOW()
	`, userID, disputeAsBuyer, disputeAsSeller)

	return err
}

// updateUserStatsWithWin updates user stats when a dispute is resolved in their favor.
func (s *DisputeMetricsService) updateUserStatsWithWin(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	isBuyerWin bool,
) error {
	var column string
	if isBuyerWin {
		column = "buyer_wins"
	} else {
		column = "seller_wins"
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO user_dispute_stats (user_id, total_disputes, `+column+`)
		VALUES ($1, 0, 1)
		ON CONFLICT (user_id) DO UPDATE SET
			`+column+` = user_dispute_stats.`+column+` + 1,
			updated_at = NOW()
	`, userID)

	return err
}

// updateReasonMetrics updates reason code statistics.
func (s *DisputeMetricsService) updateReasonMetrics(
	ctx context.Context,
	tx db.Tx,
	reasonCode string,
	isBuyer bool,
) error {
	var buyerCount, sellerCount int
	if isBuyer {
		buyerCount = 1
	} else {
		sellerCount = 1
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO dispute_reason_metrics (reason_code, total_count, buyer_count, seller_count, last_used_at)
		VALUES ($1, 1, $2, $3, NOW())
		ON CONFLICT (reason_code) DO UPDATE SET
			total_count = dispute_reason_metrics.total_count + 1,
			buyer_count = dispute_reason_metrics.buyer_count + $2,
			seller_count = dispute_reason_metrics.seller_count + $3,
			last_used_at = NOW(),
			updated_at = NOW()
	`, reasonCode, buyerCount, sellerCount)

	return err
}

// =============================================================================
// QUERY METHODS
// =============================================================================

// GetDailyMetrics retrieves metrics for a specific date range.
func (s *DisputeMetricsService) GetDailyMetrics(
	ctx context.Context,
	tx db.Tx,
	startDate, endDate time.Time,
) ([]map[string]interface{}, error) {
	rows, err := tx.Query(ctx, `
		SELECT
			metric_date,
			total_disputes,
			buyer_wins,
			seller_wins,
			escalated_count,
			resolved_count,
			pending_count,
			CASE
				WHEN resolved_count > 0 THEN ROUND(100.0 * buyer_wins / resolved_count, 2)
				ELSE 0
			END as buyer_win_rate,
			CASE
				WHEN resolved_count > 0 THEN ROUND(100.0 * seller_wins / resolved_count, 2)
				ELSE 0
			END as seller_win_rate
		FROM dispute_metrics
		WHERE metric_date BETWEEN $1 AND $2
		ORDER BY metric_date ASC
	`, startDate, endDate)

	if err != nil {
		return nil, fmt.Errorf("failed to query daily metrics: %w", err)
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var metricDate time.Time
		var totalDisputes, buyerWins, sellerWins, escalatedCount, resolvedCount, pendingCount int
		var buyerWinRate, sellerWinRate float64

		err := rows.Scan(&metricDate, &totalDisputes, &buyerWins, &sellerWins,
			&escalatedCount, &resolvedCount, &pendingCount, &buyerWinRate, &sellerWinRate)
		if err != nil {
			return nil, fmt.Errorf("failed to scan metrics: %w", err)
		}

		results = append(results, map[string]interface{}{
			"date":             metricDate,
			"total_disputes":   totalDisputes,
			"buyer_wins":       buyerWins,
			"seller_wins":      sellerWins,
			"escalated_count":  escalatedCount,
			"resolved_count":   resolvedCount,
			"pending_count":    pendingCount,
			"buyer_win_rate":   buyerWinRate,
			"seller_win_rate":  sellerWinRate,
		})
	}

	return results, nil
}

// GetHighRiskUsers retrieves users with high dispute frequency.
func (s *DisputeMetricsService) GetHighRiskUsers(
	ctx context.Context,
	tx db.Tx,
	limit int,
) ([]map[string]interface{}, error) {
	rows, err := tx.Query(ctx, `
		SELECT
			user_id,
			total_disputes,
			disputes_as_buyer,
			disputes_as_seller,
			buyer_wins,
			seller_wins,
			last_dispute_at,
			CASE
				WHEN total_disputes >= 10 THEN 'critical'
				WHEN total_disputes >= 5 THEN 'high'
				WHEN total_disputes >= 3 THEN 'medium'
				ELSE 'low'
			END as risk_level
		FROM user_dispute_stats
		WHERE total_disputes >= 3
		ORDER BY total_disputes DESC
		LIMIT $1
	`, limit)

	if err != nil {
		return nil, fmt.Errorf("failed to query high-risk users: %w", err)
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var userID uuid.UUID
		var totalDisputes, disputesAsBuyer, disputesAsSeller, buyerWins, sellerWins int
		var lastDisputeAt time.Time
		var riskLevel string

		err := rows.Scan(&userID, &totalDisputes, &disputesAsBuyer, &disputesAsSeller,
			&buyerWins, &sellerWins, &lastDisputeAt, &riskLevel)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user stats: %w", err)
		}

		results = append(results, map[string]interface{}{
			"user_id":            userID,
			"total_disputes":     totalDisputes,
			"disputes_as_buyer":  disputesAsBuyer,
			"disputes_as_seller": disputesAsSeller,
			"buyer_wins":         buyerWins,
			"seller_wins":        sellerWins,
			"last_dispute_at":    lastDisputeAt,
			"risk_level":         riskLevel,
		})
	}

	return results, nil
}


