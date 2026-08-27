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

// Thresholds for outbox stuck-event detection.
const (
	// OutboxStuckAgeMinutes is how long an event can remain in 'processing'
	// before it is considered stuck (likely worker crash).
	OutboxStuckAgeMinutes = 10

	// OutboxStuckThreshold is the minimum count of stuck events to trigger
	// a WARNING alert.
	OutboxStuckThreshold = 10

	// OutboxStuckCriticalThreshold escalates to CRITICAL.
	OutboxStuckCriticalThreshold = 50

	// OutboxStuckScanLimit caps sample rows per detection cycle.
	OutboxStuckScanLimit = 20
)

// outboxStuckRow holds one stuck-processing row from the query.
type outboxStuckRow struct {
	ID         uuid.UUID
	EventType  string
	RetryCount int
	UpdatedAt  time.Time
}

// OutboxStuckRule detects outbox events stuck in 'processing' state longer
// than the configured age threshold, indicating a crashed or hung worker.
//
// READ-ONLY: This rule does NOT reset, delete, or modify stuck events.
// It only creates an alert in system_alerts.
type OutboxStuckRule struct {
	db  db.Transactor
	log *zap.Logger
}

// NewOutboxStuckRule creates a new outbox stuck-event detection rule.
func NewOutboxStuckRule(db db.Transactor, log *zap.Logger) *OutboxStuckRule {
	return &OutboxStuckRule{db: db, log: log}
}

func (r *OutboxStuckRule) Name() string {
	return "outbox_stuck"
}

// Detect queries the outbox table for events in 'processing' state older
// than the age threshold.
func (r *OutboxStuckRule) Detect(ctx context.Context, tx db.Tx) (bool, *AnomalyFinding, error) {
	cutoff := time.Now().Add(-OutboxStuckAgeMinutes * time.Minute)

	rows, err := tx.Query(ctx, `
		SELECT id, event_type, retry_count, updated_at
		FROM outbox
		WHERE status = 'processing'
		  AND updated_at < $1
		ORDER BY updated_at ASC
		LIMIT $2
	`, cutoff, OutboxStuckScanLimit)
	if err != nil {
		return false, nil, fmt.Errorf("query outbox stuck events: %w", err)
	}
	defer rows.Close()

	var stuckEvents []outboxStuckRow
	for rows.Next() {
		var e outboxStuckRow
		if err := rows.Scan(&e.ID, &e.EventType, &e.RetryCount, &e.UpdatedAt); err != nil {
			continue
		}
		stuckEvents = append(stuckEvents, e)
	}

	if len(stuckEvents) < OutboxStuckThreshold {
		return false, nil, nil
	}

	// Severity escalation
	severity := alertentity.SeverityWarning
	if len(stuckEvents) >= OutboxStuckCriticalThreshold {
		severity = alertentity.SeverityCritical
	}

	// Count by event_type
	typeCounts := map[string]int{}
	for _, e := range stuckEvents {
		typeCounts[e.EventType]++
	}

	// Oldest stuck event for reference
	oldest := stuckEvents[0]
	ageMinutes := int(time.Since(oldest.UpdatedAt).Minutes())

	// Build sample list (up to 10)
	sampleLimit := 10
	if len(stuckEvents) < sampleLimit {
		sampleLimit = len(stuckEvents)
	}
	samples := make([]map[string]interface{}, 0, sampleLimit)
	for i := 0; i < sampleLimit; i++ {
		e := stuckEvents[i]
		samples = append(samples, map[string]interface{}{
			"event_id":    e.ID.String(),
			"event_type":  e.EventType,
			"retry_count": e.RetryCount,
			"updated_at":  e.UpdatedAt.UTC().Format(time.RFC3339),
			"age_minutes": int(time.Since(e.UpdatedAt).Minutes()),
		})
	}

	groupKey := "outbox_stuck:processing"

	// Stable entity ID for dedup across cycles
	entityID := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	metadata := alertentity.AlertMetadata{
		"stuck_count":    len(stuckEvents),
		"age_threshold":  OutboxStuckAgeMinutes,
		"threshold":      OutboxStuckThreshold,
		"type_breakdown": typeCounts,
		"oldest_event": map[string]interface{}{
			"event_id":    oldest.ID.String(),
			"event_type":  oldest.EventType,
			"retry_count": oldest.RetryCount,
			"age_minutes": ageMinutes,
		},
		"sample_events": samples,
		"detected_at":   time.Now().UTC().Format(time.RFC3339),
	}

	return true, &AnomalyFinding{
		AlertType:  alertentity.AlertTypeOutboxStuck,
		Severity:   severity,
		EntityType: "outbox",
		EntityID:   entityID,
		Message: fmt.Sprintf("%d outbox event(s) stuck in processing for >%d minutes (oldest: %d min, type %s)",
			len(stuckEvents), OutboxStuckAgeMinutes, ageMinutes, oldest.EventType),
		Metadata: metadata,
		GroupKey: &groupKey,
	}, nil
}


