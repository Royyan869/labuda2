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

// Thresholds for outbox dead-letter spike detection.
const (
	// OutboxDLQSpikeWindowMinutes is the lookback window for counting recent
	// dead-letter transitions.
	OutboxDLQSpikeWindowMinutes = 10

	// OutboxDLQSpikeThreshold is the minimum number of dead-letter events
	// within the window to trigger a WARNING alert.
	OutboxDLQSpikeThreshold = 3

	// OutboxDLQSpikeCriticalThreshold escalates to CRITICAL when this many
	// events hit dead-letter within the window.
	OutboxDLQSpikeCriticalThreshold = 10

	// OutboxDLQSpikeScanLimit caps the sample rows returned per cycle.
	OutboxDLQSpikeScanLimit = 20
)

// outboxDLQRow holds one dead-letter row from the query.
type outboxDLQRow struct {
	ID        uuid.UUID
	EventType string
	UpdatedAt time.Time
}

// OutboxDLQSpikeRule detects a spike of outbox events transitioning to
// dead_letter status within a recent time window.
//
// READ-ONLY: This rule does NOT replay, delete, or modify outbox events.
// It only creates an alert in system_alerts.
type OutboxDLQSpikeRule struct {
	db  db.Transactor
	log *zap.Logger
}

// NewOutboxDLQSpikeRule creates a new outbox dead-letter spike detection rule.
func NewOutboxDLQSpikeRule(db db.Transactor, log *zap.Logger) *OutboxDLQSpikeRule {
	return &OutboxDLQSpikeRule{db: db, log: log}
}

func (r *OutboxDLQSpikeRule) Name() string {
	return "outbox_dlq_spike"
}

// Detect queries the outbox table for events that transitioned to dead_letter
// within the lookback window.
func (r *OutboxDLQSpikeRule) Detect(ctx context.Context, tx db.Tx) (bool, *AnomalyFinding, error) {
	windowStart := time.Now().Add(-OutboxDLQSpikeWindowMinutes * time.Minute)

	rows, err := tx.Query(ctx, `
		SELECT id, event_type, updated_at
		FROM outbox
		WHERE status = 'dead_letter'
		  AND updated_at >= $1
		ORDER BY updated_at DESC
		LIMIT $2
	`, windowStart, OutboxDLQSpikeScanLimit)
	if err != nil {
		return false, nil, fmt.Errorf("query outbox dead_letter spike: %w", err)
	}
	defer rows.Close()

	var dlqEvents []outboxDLQRow
	for rows.Next() {
		var e outboxDLQRow
		if err := rows.Scan(&e.ID, &e.EventType, &e.UpdatedAt); err != nil {
			continue
		}
		dlqEvents = append(dlqEvents, e)
	}

	if len(dlqEvents) < OutboxDLQSpikeThreshold {
		return false, nil, nil
	}

	// Severity escalation
	severity := alertentity.SeverityWarning
	if len(dlqEvents) >= OutboxDLQSpikeCriticalThreshold {
		severity = alertentity.SeverityCritical
	}

	// Count by event_type for metadata
	typeCounts := map[string]int{}
	for _, e := range dlqEvents {
		typeCounts[e.EventType]++
	}

	// Build sample list (up to 10)
	sampleLimit := 10
	if len(dlqEvents) < sampleLimit {
		sampleLimit = len(dlqEvents)
	}
	samples := make([]map[string]interface{}, 0, sampleLimit)
	for i := 0; i < sampleLimit; i++ {
		e := dlqEvents[i]
		samples = append(samples, map[string]interface{}{
			"event_id":   e.ID.String(),
			"event_type": e.EventType,
			"updated_at": e.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}

	groupKey := "outbox_dlq_spike:recent"

	// Use a stable entity ID so dedup works across cycles
	entityID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	metadata := alertentity.AlertMetadata{
		"dlq_count":      len(dlqEvents),
		"window_minutes": OutboxDLQSpikeWindowMinutes,
		"threshold":      OutboxDLQSpikeThreshold,
		"type_breakdown": typeCounts,
		"sample_events":  samples,
		"detected_at":    time.Now().UTC().Format(time.RFC3339),
	}

	return true, &AnomalyFinding{
		AlertType:  alertentity.AlertTypeOutboxDLQSpike,
		Severity:   severity,
		EntityType: "outbox",
		EntityID:   entityID,
		Message: fmt.Sprintf("%d outbox event(s) moved to dead_letter in last %d minutes",
			len(dlqEvents), OutboxDLQSpikeWindowMinutes),
		Metadata: metadata,
		GroupKey: &groupKey,
	}, nil
}


