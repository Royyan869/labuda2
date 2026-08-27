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

// Constants for order stuck state detection thresholds.
const (
	// EscrowStuckThresholdDays is how long an escrow can remain in holding before warning.
	EscrowStuckThresholdDays = 7
	// EscrowStuckCriticalDays is how long before escalating to critical.
	EscrowStuckCriticalDays = 14

	// OrderPaidStuckHours is how long an order can remain in paid state before warning.
	OrderPaidStuckHours = 24
	// OrderPaidStuckCriticalCount is the count above which severity escalates to critical.
	OrderPaidStuckCriticalCount = 5

	// OrderShippedStuckDays is how long an order can remain in shipped state before warning.
	OrderShippedStuckDays = 7
	// OrderShippedStuckCriticalCount is the count above which severity escalates to critical.
	OrderShippedStuckCriticalCount = 5

	// DisputeOpenStuckWarnHours is how long a dispute can be under_review before warning.
	DisputeOpenStuckWarnHours = 48
	// DisputeOpenStuckCriticalHours is how long before escalating to critical.
	DisputeOpenStuckCriticalHours = 72
)

// =============================================================================
// EscrowStuckRule
// =============================================================================

// EscrowStuckRule detects escrows stuck in holding state longer than the threshold.
// Observational only: does not modify other domains.
type EscrowStuckRule struct {
	db  db.Transactor
	log *zap.Logger
}

// NewEscrowStuckRule creates a new escrow stuck rule.
func NewEscrowStuckRule(db db.Transactor, log *zap.Logger) *EscrowStuckRule {
	return &EscrowStuckRule{db: db, log: log}
}

func (r *EscrowStuckRule) Name() string {
	return "escrow_stuck"
}

func (r *EscrowStuckRule) Detect(ctx context.Context, tx db.Tx) (bool, *AnomalyFinding, error) {
	warnCutoff := time.Now().Add(-time.Duration(EscrowStuckThresholdDays) * 24 * time.Hour)
	criticalCutoff := time.Now().Add(-time.Duration(EscrowStuckCriticalDays) * 24 * time.Hour)

	// Count escrows stuck in holding beyond warn threshold, excluding terminal order states.
	// Escrows table uses created_at (no updated_at column).
	// The representative order_id is selected from the oldest escrow row by
	// escrow created_at, with order_id as a deterministic tie-breaker.
	var stuckCount int
	var oldestOrderID string
	var oldestAgeHours float64
	err := tx.QueryRow(ctx, `
		WITH stuck_escrows AS (
			SELECT e.order_id, e.created_at
			FROM escrows e
			JOIN orders o ON o.id = e.order_id
			WHERE e.status = 'holding'
			  AND o.status NOT IN ('cancelled', 'cancelled_timeout', 'expired', 'pending_payment')
			  AND e.created_at < $1
		)
		SELECT
			COUNT(*) AS stuck_count,
			COALESCE((
				SELECT se.order_id::TEXT
				FROM stuck_escrows se
				ORDER BY se.created_at ASC, se.order_id ASC
				LIMIT 1
			), '') AS oldest_order_id,
			COALESCE(EXTRACT(EPOCH FROM (NOW() - MIN(created_at))) / 3600, 0) AS oldest_age_hours
		FROM stuck_escrows
	`, warnCutoff).Scan(&stuckCount, &oldestOrderID, &oldestAgeHours)
	if err != nil {
		return false, nil, fmt.Errorf("escrow_stuck: query failed: %w", err)
	}

	if stuckCount == 0 {
		return false, nil, nil
	}

	severity := alertentity.SeverityWarning
	if oldestAgeHours >= float64(EscrowStuckCriticalDays*24) {
		severity = alertentity.SeverityCritical
	}

	// Also count the critical subset for metadata.
	var criticalCount int
	_ = tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM escrows e
		JOIN orders o ON o.id = e.order_id
		WHERE e.status = 'holding'
		  AND o.status NOT IN ('cancelled', 'cancelled_timeout', 'expired', 'pending_payment')
		  AND e.created_at < $1
	`, criticalCutoff).Scan(&criticalCount)

	today := time.Now().UTC().Format("20060102")
	groupKey := fmt.Sprintf("escrow_stuck:%s", today)

	return true, &AnomalyFinding{
		AlertType:  alertentity.AlertTypeEscrowStuck,
		Severity:   severity,
		EntityType: "system",
		EntityID:   uuid.Nil,
		Message: fmt.Sprintf(
			"Escrow stuck in holding: %d orders (≥%dd warn threshold), oldest %.1fh ago",
			stuckCount, EscrowStuckThresholdDays, oldestAgeHours,
		),
		Metadata: alertentity.AlertMetadata{
			"stuck_count":      stuckCount,
			"critical_count":   criticalCount,
			"oldest_order_id":  oldestOrderID,
			"oldest_age_hours": oldestAgeHours,
			"threshold_days":   EscrowStuckThresholdDays,
			"critical_days":    EscrowStuckCriticalDays,
		},
		GroupKey: &groupKey,
	}, nil
}

// =============================================================================
// OrderPaidStuckRule — FIX-3
// =============================================================================

// OrderPaidStuckRule detects orders stuck in paid state awaiting shipment.
type OrderPaidStuckRule struct {
	db  db.Transactor
	log *zap.Logger
}

// NewOrderPaidStuckRule creates a new order paid stuck rule.
func NewOrderPaidStuckRule(db db.Transactor, log *zap.Logger) *OrderPaidStuckRule {
	return &OrderPaidStuckRule{db: db, log: log}
}

func (r *OrderPaidStuckRule) Name() string {
	return "order_paid_stuck"
}

func (r *OrderPaidStuckRule) Detect(ctx context.Context, tx db.Tx) (bool, *AnomalyFinding, error) {
	cutoff := time.Now().Add(-time.Duration(OrderPaidStuckHours) * time.Hour)

	type stuckRow struct {
		count         int
		sampleOrderID *string
	}

	var stuckCount int
	err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM orders
		WHERE status = 'paid'
		  AND updated_at < $1
	`, cutoff).Scan(&stuckCount)
	if err != nil {
		return false, nil, fmt.Errorf("order_paid_stuck: query failed: %w", err)
	}

	if stuckCount == 0 {
		return false, nil, nil
	}

	// Collect up to 5 sample order IDs for operator context.
	rows, err := tx.Query(ctx, `
		SELECT id::TEXT
		FROM orders
		WHERE status = 'paid'
		  AND updated_at < $1
		ORDER BY updated_at ASC
		LIMIT 5
	`, cutoff)
	if err != nil {
		return false, nil, fmt.Errorf("order_paid_stuck: sample query failed: %w", err)
	}
	defer rows.Close()

	var sampleIDs []string
	for rows.Next() {
		var id string
		if scanErr := rows.Scan(&id); scanErr == nil {
			sampleIDs = append(sampleIDs, id)
		}
	}

	severity := alertentity.SeverityWarning
	if stuckCount > OrderPaidStuckCriticalCount {
		severity = alertentity.SeverityCritical
	}

	today := time.Now().UTC().Format("20060102")
	groupKey := fmt.Sprintf("order_paid_stuck:%s", today)

	return true, &AnomalyFinding{
		AlertType:  alertentity.AlertTypeOrderPaidStuck,
		Severity:   severity,
		EntityType: "system",
		EntityID:   uuid.Nil,
		Message: fmt.Sprintf(
			"Orders stuck in paid state: %d orders (threshold: >%dh without shipment)",
			stuckCount, OrderPaidStuckHours,
		),
		Metadata: alertentity.AlertMetadata{
			"stuck_count":      stuckCount,
			"sample_order_ids": sampleIDs,
			"threshold_hours":  OrderPaidStuckHours,
			"critical_count":   OrderPaidStuckCriticalCount,
		},
		GroupKey: &groupKey,
	}, nil
}

// =============================================================================
// OrderShippedStuckRule — FIX-3
// =============================================================================

// OrderShippedStuckRule detects orders stuck in shipped state awaiting delivery confirmation.
type OrderShippedStuckRule struct {
	db  db.Transactor
	log *zap.Logger
}

// NewOrderShippedStuckRule creates a new order shipped stuck rule.
func NewOrderShippedStuckRule(db db.Transactor, log *zap.Logger) *OrderShippedStuckRule {
	return &OrderShippedStuckRule{db: db, log: log}
}

func (r *OrderShippedStuckRule) Name() string {
	return "order_shipped_stuck"
}

func (r *OrderShippedStuckRule) Detect(ctx context.Context, tx db.Tx) (bool, *AnomalyFinding, error) {
	cutoff := time.Now().Add(-time.Duration(OrderShippedStuckDays) * 24 * time.Hour)

	var stuckCount int
	err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM orders
		WHERE status = 'shipped'
		  AND updated_at < $1
	`, cutoff).Scan(&stuckCount)
	if err != nil {
		return false, nil, fmt.Errorf("order_shipped_stuck: query failed: %w", err)
	}

	if stuckCount == 0 {
		return false, nil, nil
	}

	rows, err := tx.Query(ctx, `
		SELECT id::TEXT
		FROM orders
		WHERE status = 'shipped'
		  AND updated_at < $1
		ORDER BY updated_at ASC
		LIMIT 5
	`, cutoff)
	if err != nil {
		return false, nil, fmt.Errorf("order_shipped_stuck: sample query failed: %w", err)
	}
	defer rows.Close()

	var sampleIDs []string
	for rows.Next() {
		var id string
		if scanErr := rows.Scan(&id); scanErr == nil {
			sampleIDs = append(sampleIDs, id)
		}
	}

	severity := alertentity.SeverityWarning
	if stuckCount > OrderShippedStuckCriticalCount {
		severity = alertentity.SeverityCritical
	}

	today := time.Now().UTC().Format("20060102")
	groupKey := fmt.Sprintf("order_shipped_stuck:%s", today)

	return true, &AnomalyFinding{
		AlertType:  alertentity.AlertTypeOrderShippedStuck,
		Severity:   severity,
		EntityType: "system",
		EntityID:   uuid.Nil,
		Message: fmt.Sprintf(
			"Orders stuck in shipped state: %d orders (threshold: >%dd without confirmation)",
			stuckCount, OrderShippedStuckDays,
		),
		Metadata: alertentity.AlertMetadata{
			"stuck_count":      stuckCount,
			"sample_order_ids": sampleIDs,
			"threshold_days":   OrderShippedStuckDays,
			"critical_count":   OrderShippedStuckCriticalCount,
		},
		GroupKey: &groupKey,
	}, nil
}

// =============================================================================
// DisputeOpenStuckRule — FIX-3
// =============================================================================

// DisputeOpenStuckRule detects disputes stuck in under_review state longer than threshold.
type DisputeOpenStuckRule struct {
	db  db.Transactor
	log *zap.Logger
}

// NewDisputeOpenStuckRule creates a new dispute open stuck rule.
func NewDisputeOpenStuckRule(db db.Transactor, log *zap.Logger) *DisputeOpenStuckRule {
	return &DisputeOpenStuckRule{db: db, log: log}
}

func (r *DisputeOpenStuckRule) Name() string {
	return "dispute_open_stuck"
}

func (r *DisputeOpenStuckRule) Detect(ctx context.Context, tx db.Tx) (bool, *AnomalyFinding, error) {
	warnCutoff := time.Now().Add(-time.Duration(DisputeOpenStuckWarnHours) * time.Hour)

	var disputeCount int
	var oldestDisputeID string
	var oldestAgeHours float64
	err := tx.QueryRow(ctx, `
		SELECT
			COUNT(*) AS dispute_count,
			COALESCE(MIN(id)::TEXT, '') AS oldest_dispute_id,
			COALESCE(EXTRACT(EPOCH FROM (NOW() - MIN(opened_at))) / 3600, 0) AS oldest_age_hours
		FROM disputes
		WHERE status = 'under_review'
		  AND opened_at < $1
	`, warnCutoff).Scan(&disputeCount, &oldestDisputeID, &oldestAgeHours)
	if err != nil {
		return false, nil, fmt.Errorf("dispute_open_stuck: query failed: %w", err)
	}

	if disputeCount == 0 {
		return false, nil, nil
	}

	severity := alertentity.SeverityWarning
	if oldestAgeHours >= float64(DisputeOpenStuckCriticalHours) {
		severity = alertentity.SeverityCritical
	}

	today := time.Now().UTC().Format("20060102")
	groupKey := fmt.Sprintf("dispute_open_stuck:%s", today)

	return true, &AnomalyFinding{
		AlertType:  alertentity.AlertTypeDisputeOpenStuck,
		Severity:   severity,
		EntityType: "system",
		EntityID:   uuid.Nil,
		Message: fmt.Sprintf(
			"Disputes stuck in under_review: %d disputes (oldest %.1fh, warn threshold: %dh)",
			disputeCount, oldestAgeHours, DisputeOpenStuckWarnHours,
		),
		Metadata: alertentity.AlertMetadata{
			"dispute_count":        disputeCount,
			"oldest_dispute_id":    oldestDisputeID,
			"oldest_age_hours":     oldestAgeHours,
			"warn_threshold_h":     DisputeOpenStuckWarnHours,
			"critical_threshold_h": DisputeOpenStuckCriticalHours,
		},
		GroupKey: &groupKey,
	}, nil
}
