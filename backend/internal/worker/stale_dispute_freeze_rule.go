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

// Thresholds for stale dispute freeze detection.
const (
	// StaleDisputeFreezeThresholdHours is the age (hours) after which an active
	// dispute freeze is considered stale and triggers an operator alert.
	StaleDisputeFreezeThresholdHours = 48

	// StaleDisputeFreezeCriticalHours is the age (hours) after which the alert
	// escalates from WARNING to CRITICAL.
	StaleDisputeFreezeCriticalHours = 72

	// StaleDisputeFreezeScanLimit caps the number of stale freezes returned per
	// detection cycle to avoid unbounded result sets.
	StaleDisputeFreezeScanLimit = 50
)

// staleFreezeRow holds one row from the stale-freeze query.
type staleFreezeRow struct {
	ID           uuid.UUID
	DisputeID    uuid.UUID
	OrderID      uuid.UUID
	SellerID     uuid.UUID
	FrozenAmount int64
	CreatedAtMs  int64 // Unix epoch milliseconds
}

// StaleDisputeFreezeRule detects active dispute_freezes that have been active
// for longer than the configured threshold.
//
// O2: Closes the silent-failure gap where sellers can have funds frozen
// indefinitely with zero operator visibility.
//
// READ-ONLY: This rule does NOT release freezes, resolve disputes, or create
// debt. It only creates an alert in system_alerts.
type StaleDisputeFreezeRule struct {
	db  db.Transactor
	log *zap.Logger
}

// NewStaleDisputeFreezeRule creates a new stale dispute freeze detection rule.
func NewStaleDisputeFreezeRule(db db.Transactor, log *zap.Logger) *StaleDisputeFreezeRule {
	return &StaleDisputeFreezeRule{db: db, log: log}
}

func (r *StaleDisputeFreezeRule) Name() string {
	return "stale_dispute_freeze"
}

// Detect queries for active dispute_freezes older than the threshold.
//
// NOTE: dispute_freezes.created_at is BIGINT (Unix epoch milliseconds).
func (r *StaleDisputeFreezeRule) Detect(ctx context.Context, tx db.Tx) (bool, *AnomalyFinding, error) {
	thresholdMs := time.Now().Add(-StaleDisputeFreezeThresholdHours * time.Hour).UnixMilli()

	rows, err := tx.Query(ctx, `
		SELECT id, dispute_id, order_id, seller_id, frozen_amount, created_at
		FROM dispute_freezes
		WHERE status = 'active'
		  AND created_at < $1
		ORDER BY created_at ASC
		LIMIT $2
	`, thresholdMs, StaleDisputeFreezeScanLimit)
	if err != nil {
		return false, nil, fmt.Errorf("query stale dispute freezes: %w", err)
	}
	defer rows.Close()

	var staleFreezes []staleFreezeRow
	for rows.Next() {
		var f staleFreezeRow
		if err := rows.Scan(&f.ID, &f.DisputeID, &f.OrderID, &f.SellerID, &f.FrozenAmount, &f.CreatedAtMs); err != nil {
			continue
		}
		staleFreezes = append(staleFreezes, f)
	}

	if len(staleFreezes) == 0 {
		return false, nil, nil
	}

	// Use the oldest freeze as the primary entity for the alert.
	oldest := staleFreezes[0]
	ageHours := int(time.Since(time.UnixMilli(oldest.CreatedAtMs)).Hours())

	severity := alertentity.SeverityWarning
	if ageHours >= StaleDisputeFreezeCriticalHours {
		severity = alertentity.SeverityCritical
	}

	// Build sample list (up to 10 for metadata readability).
	sampleLimit := 10
	if len(staleFreezes) < sampleLimit {
		sampleLimit = len(staleFreezes)
	}
	sampleFreezes := make([]map[string]interface{}, 0, sampleLimit)
	for i := 0; i < sampleLimit; i++ {
		f := staleFreezes[i]
		fAge := int(time.Since(time.UnixMilli(f.CreatedAtMs)).Hours())
		sampleFreezes = append(sampleFreezes, map[string]interface{}{
			"freeze_id":  f.ID.String(),
			"dispute_id": f.DisputeID.String(),
			"order_id":   f.OrderID.String(),
			"seller_id":  f.SellerID.String(),
			"amount":     f.FrozenAmount,
			"age_hours":  fAge,
		})
	}

	groupKey := "stale_dispute_freeze:active"

	metadata := alertentity.AlertMetadata{
		"stale_count":     len(staleFreezes),
		"threshold_hours": StaleDisputeFreezeThresholdHours,
		"oldest_freeze": map[string]interface{}{
			"freeze_id":  oldest.ID.String(),
			"dispute_id": oldest.DisputeID.String(),
			"order_id":   oldest.OrderID.String(),
			"seller_id":  oldest.SellerID.String(),
			"amount":     oldest.FrozenAmount,
			"created_at": time.UnixMilli(oldest.CreatedAtMs).UTC().Format(time.RFC3339),
			"age_hours":  ageHours,
		},
		"sample_freezes": sampleFreezes,
		"detected_at":    time.Now(),
	}

	return true, &AnomalyFinding{
		AlertType:  alertentity.AlertTypeStaleDisputeFreeze,
		Severity:   severity,
		EntityType: "dispute_freeze",
		EntityID:   oldest.ID,
		Message: fmt.Sprintf("%d active dispute freeze(s) older than %dh (oldest: %dh, seller %s)",
			len(staleFreezes), StaleDisputeFreezeThresholdHours, ageHours, oldest.SellerID),
		Metadata: metadata,
		GroupKey: &groupKey,
	}, nil
}


