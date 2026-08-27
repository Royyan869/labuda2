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

// Constants for subscription detection thresholds
const (
	// SubscriptionOrphanedPaymentThreshold is the number of orphaned payments to trigger an alert
	SubscriptionOrphanedPaymentThreshold = 1

	// SubscriptionConversionRateThreshold is the minimum conversion rate (0-1) before alert
	SubscriptionConversionRateThreshold = 0.95

	// SubscriptionStuckThresholdDays is how long a subscription can be in wrong state before alert
	SubscriptionStuckThresholdDays = 1
)

// SubscriptionOrphanedPaymentRule detects subscription payments without matching subscriptions.
type SubscriptionOrphanedPaymentRule struct {
	db  db.Transactor
	log *zap.Logger
}

// NewSubscriptionOrphanedPaymentRule creates a new orphaned subscription payment rule.
func NewSubscriptionOrphanedPaymentRule(db db.Transactor, log *zap.Logger) *SubscriptionOrphanedPaymentRule {
	return &SubscriptionOrphanedPaymentRule{db: db, log: log}
}

func (r *SubscriptionOrphanedPaymentRule) Name() string {
	return "subscription_orphaned_payment"
}

func (r *SubscriptionOrphanedPaymentRule) Detect(ctx context.Context, tx db.Tx) (bool, *AnomalyFinding, error) {
	// Find payments with reference_type='subscription' and status='settlement'
	// but no matching subscription record
	const query = `
		SELECT COUNT(*)
		FROM payments p
		WHERE p.reference_type = 'subscription'
		  AND p.status = 'settlement'
		  AND NOT EXISTS (
		    SELECT 1 FROM seller_subscriptions s WHERE s.payment_id = p.id
		  );
	`

	var orphanedCount int
	err := tx.QueryRow(ctx, query).Scan(&orphanedCount)
	if err != nil {
		return false, nil, fmt.Errorf("query orphaned payments: %w", err)
	}

	if orphanedCount >= SubscriptionOrphanedPaymentThreshold {
		// Get detailed info for the most recent orphaned payments
		detailQuery := `
			SELECT p.id, p.user_id, p.payment_number, p.paid_at
			FROM payments p
			WHERE p.reference_type = 'subscription'
			  AND p.status = 'settlement'
			  AND NOT EXISTS (
			    SELECT 1 FROM seller_subscriptions s WHERE s.payment_id = p.id
			  )
			ORDER BY p.paid_at DESC
			LIMIT 5;
		`

		rows, err := tx.Query(ctx, detailQuery)
		if err != nil {
			return false, nil, fmt.Errorf("query orphaned payment details: %w", err)
		}
		defer rows.Close()

		type orphanedPayment struct {
			ID            uuid.UUID
			UserID        uuid.UUID
			PaymentNumber string
			PaidAt        time.Time
		}

		var orphanedPayments []orphanedPayment
		for rows.Next() {
			var op orphanedPayment
			if err := rows.Scan(&op.ID, &op.UserID, &op.PaymentNumber, &op.PaidAt); err != nil {
				continue
			}
			orphanedPayments = append(orphanedPayments, op)
		}

		groupKey := fmt.Sprintf("subscription_orphaned_payment:%d", time.Now().Unix()/60) // Group by minute

		metadata := alertentity.AlertMetadata{
			"orphaned_count": orphanedCount,
			"threshold":      SubscriptionOrphanedPaymentThreshold,
			"detected_at":    time.Now(),
		}

		if len(orphanedPayments) > 0 {
			metadata["recent_orphans"] = orphanedPayments
		}

		severity := alertentity.SeverityCritical
		message := fmt.Sprintf("CRITICAL: %d subscription payments without matching subscriptions (payment webhook integration broken)",
			orphanedCount)

		return true, &AnomalyFinding{
			AlertType:  alertentity.AlertTypeSubscriptionOrphanedPayment,
			Severity:   severity,
			EntityType: "system",
			EntityID:   uuid.Nil,
			Message:    message,
			Metadata:   metadata,
			GroupKey:   &groupKey,
		}, nil
	}

	return false, nil, nil
}

// SubscriptionConversionRateRule detects low subscription payment conversion rates.
type SubscriptionConversionRateRule struct {
	db  db.Transactor
	log *zap.Logger
}

// NewSubscriptionConversionRateRule creates a new subscription conversion rate rule.
func NewSubscriptionConversionRateRule(db db.Transactor, log *zap.Logger) *SubscriptionConversionRateRule {
	return &SubscriptionConversionRateRule{db: db, log: log}
}

func (r *SubscriptionConversionRateRule) Name() string {
	return "subscription_conversion_rate"
}

func (r *SubscriptionConversionRateRule) Detect(ctx context.Context, tx db.Tx) (bool, *AnomalyFinding, error) {
	const query = `
		WITH payment_stats AS (
		  SELECT
		    COUNT(*) FILTER (WHERE status = 'settlement') as total_settlement,
		    COUNT(*) FILTER (
		      WHERE status = 'settlement'
		      AND EXISTS (SELECT 1 FROM seller_subscriptions s WHERE s.payment_id = payments.id)
		    ) as converted
		  FROM payments
		  WHERE reference_type = 'subscription'
		)
		SELECT
		  COALESCE(total_settlement, 0) as total,
		  COALESCE(converted, 0) as converted
		FROM payment_stats;
	`

	var total, converted int
	err := tx.QueryRow(ctx, query).Scan(&total, &converted)
	if err != nil {
		return false, nil, fmt.Errorf("query conversion rate: %w", err)
	}

	if total == 0 {
		// No payments yet, no alert
		return false, nil, nil
	}

	conversionRate := float64(converted) / float64(total)
	if conversionRate < SubscriptionConversionRateThreshold {
		groupKey := fmt.Sprintf("subscription_conversion_rate:%d", time.Now().Unix()/300) // Group by 5 minutes

		metadata := alertentity.AlertMetadata{
			"total_payments":       total,
			"converted_payments":   converted,
			"conversion_rate":      conversionRate,
			"threshold":            SubscriptionConversionRateThreshold,
			"missing_subscriptions": total - converted,
			"detected_at":          time.Now(),
		}

		severity := alertentity.SeverityCritical
		if conversionRate > 0.90 {
			severity = alertentity.SeverityHigh
		} else if conversionRate > 0.80 {
			severity = alertentity.SeverityMedium
		}

		message := fmt.Sprintf("Low subscription conversion rate: %.2f%% (%d/%d payments converted, %d missing subscriptions)",
			conversionRate*100, converted, total, total-converted)

		return true, &AnomalyFinding{
			AlertType:  alertentity.AlertTypeSubscriptionConversionRate,
			Severity:   severity,
			EntityType: "system",
			EntityID:   uuid.Nil,
			Message:    message,
			Metadata:   metadata,
			GroupKey:   &groupKey,
		}, nil
	}

	return false, nil, nil
}

// SubscriptionLifecycleRule detects subscriptions stuck in wrong state.
type SubscriptionLifecycleRule struct {
	db  db.Transactor
	log *zap.Logger
}

// NewSubscriptionLifecycleRule creates a new subscription lifecycle rule.
func NewSubscriptionLifecycleRule(db db.Transactor, log *zap.Logger) *SubscriptionLifecycleRule {
	return &SubscriptionLifecycleRule{db: db, log: log}
}

func (r *SubscriptionLifecycleRule) Name() string {
	return "subscription_lifecycle"
}

func (r *SubscriptionLifecycleRule) Detect(ctx context.Context, tx db.Tx) (bool, *AnomalyFinding, error) {
	// Check for active subscriptions that should have expired
	const query = `
		SELECT COUNT(*)
		FROM seller_subscriptions
		WHERE status = 'active'
		  AND expires_at < NOW() - INTERVAL '1 day';
	`

	var stuckCount int
	err := tx.QueryRow(ctx, query).Scan(&stuckCount)
	if err != nil {
		return false, nil, fmt.Errorf("query stuck subscriptions: %w", err)
	}

	if stuckCount > 0 {
		groupKey := fmt.Sprintf("subscription_lifecycle:%d", time.Now().Unix()/60) // Group by minute

		metadata := alertentity.AlertMetadata{
			"stuck_count":     stuckCount,
			"threshold_days":  SubscriptionStuckThresholdDays,
			"detected_at":     time.Now(),
		}

		severity := alertentity.SeverityMedium
		if stuckCount > 10 {
			severity = alertentity.SeverityHigh
		}
		if stuckCount > 50 {
			severity = alertentity.SeverityCritical
		}

		message := fmt.Sprintf("%d subscriptions stuck in 'active' state past expiry (expiry worker may be down)",
			stuckCount)

		return true, &AnomalyFinding{
			AlertType:  alertentity.AlertTypeSubscriptionLifecycle,
			Severity:   severity,
			EntityType: "system",
			EntityID:   uuid.Nil,
			Message:    message,
			Metadata:   metadata,
			GroupKey:   &groupKey,
		}, nil
	}

	return false, nil, nil
}


