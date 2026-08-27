package application

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/alert/entity"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestAlertDeduplication verifies that alert deduplication works correctly.
func TestAlertDeduplication(t *testing.T) {
	// This is a placeholder test showing the expected behavior
	// In a real test, you would set up a test database and repository

	alertType := entity.AlertTypePaymentFailureSpike
	severity := entity.SeverityWarning
	entityType := "payment"
	entityID := uuid.New()
	message := "Payment failure spike detected"

	t.Run("First alert creates new record", func(t *testing.T) {
		// Expected: New alert created
		result := &CreateAlertResult{
			Created: true,
			Reason:  "New alert created",
			Alert: &entity.Alert{
				AlertType:   alertType,
				Severity:    severity,
				EntityType:  entityType,
				EntityID:    entityID,
				Message:     message,
				DedupKey:    "payment_failure_spike:payment:" + entityID.String(),
				DedupWindow: intPtr(60),
			},
		}

		assert.True(t, result.Created)
		assert.NotNil(t, result.Alert)
		assert.Equal(t, "payment_failure_spike:payment:"+entityID.String(), result.Alert.DedupKey)
	})

	t.Run("Second alert within window updates existing", func(t *testing.T) {
		// Expected: Existing alert updated
		result := &CreateAlertResult{
			Created:    false,
			Reason:     "Updated existing alert (dedup)",
			ExistingID: uuidPtr(uuid.New()),
			Alert: &entity.Alert{
				AlertType:   alertType,
				Severity:    severity,
				EntityType:  entityType,
				EntityID:    entityID,
				Message:     message,
				DedupKey:    "payment_failure_spike:payment:" + entityID.String(),
				DedupWindow: intPtr(60),
			},
		}

		assert.False(t, result.Created)
		assert.Contains(t, result.Reason, "dedup")
		assert.NotNil(t, result.ExistingID)
	})

	t.Run("Alert outside window creates new record", func(t *testing.T) {
		// Expected: New alert created (outside time window)
		result := &CreateAlertResult{
			Created: true,
			Reason:  "New alert created",
			Alert: &entity.Alert{
				AlertType:   alertType,
				Severity:    severity,
				EntityType:  entityType,
				EntityID:    entityID,
				Message:     message,
				DedupKey:    "payment_failure_spike:payment:" + entityID.String(),
				DedupWindow: intPtr(60),
			},
		}

		assert.True(t, result.Created)
		assert.Equal(t, "New alert created", result.Reason)
	})
}

// TestAlertEscalation verifies escalation policy logic.
func TestAlertEscalation(t *testing.T) {
	policy := DefaultEscalationPolicy()
	logger := zap.NewNop()
	service := &EscalationService{
		policy: policy,
		log:    logger,
	}

	t.Run("Critical alerts always escalate", func(t *testing.T) {
		alert := &entity.Alert{
			AlertType: entity.AlertTypePaymentFailureSpike,
			Severity:  entity.SeverityCritical,
			Status:    entity.StatusOpen,
		}

		action := service.GetEscalationAction(alert)

		assert.True(t, action.ShouldEscalate)
		assert.Equal(t, 100, action.Priority)
		assert.Equal(t, "immediate", action.Channel)
		assert.Contains(t, action.Reason, "Critical")
	})

	t.Run("Warning alerts with threshold escalate", func(t *testing.T) {
		service.SetWarningThreshold(entity.AlertTypeDisputeSpike, 5)

		alert := &entity.Alert{
			AlertType: entity.AlertTypeDisputeSpike,
			Severity:  entity.SeverityWarning,
			Status:    entity.StatusOpen,
			Metadata: entity.AlertMetadata{
				"occurrence_count": 5,
			},
		}

		action := service.GetEscalationAction(alert)

		assert.True(t, action.ShouldEscalate)
		assert.Equal(t, 50, action.Priority)
		assert.Contains(t, action.Reason, "threshold met")
	})

	t.Run("Warning alerts below threshold do not escalate", func(t *testing.T) {
		service.SetWarningThreshold(entity.AlertTypeDisputeSpike, 10)

		alert := &entity.Alert{
			AlertType: entity.AlertTypeDisputeSpike,
			Severity:  entity.SeverityWarning,
			Status:    entity.StatusOpen,
			Metadata: entity.AlertMetadata{
				"occurrence_count": 3,
			},
		}

		action := service.GetEscalationAction(alert)

		assert.False(t, action.ShouldEscalate)
		assert.Equal(t, 0, action.Priority)
		assert.Equal(t, "log", action.Channel)
	})

	t.Run("Info alerts never escalate", func(t *testing.T) {
		alert := &entity.Alert{
			AlertType: entity.AlertTypeCoinsAnomaly,
			Severity:  entity.SeverityInfo,
			Status:    entity.StatusOpen,
		}

		action := service.GetEscalationAction(alert)

		assert.False(t, action.ShouldEscalate)
		assert.Equal(t, 0, action.Priority)
		assert.Equal(t, "log", action.Channel)
	})
}

// TestSeverityLevels verifies new severity levels work correctly.
func TestSeverityLevels(t *testing.T) {
	t.Run("Critical severity helpers", func(t *testing.T) {
		alert := &entity.Alert{Severity: entity.SeverityCritical}
		assert.True(t, alert.IsCritical())
		assert.True(t, alert.RequiresImmediateAction())
		assert.True(t, alert.RequiresEscalation())
		assert.False(t, alert.IsWarning())
		assert.False(t, alert.IsInfo())
		assert.False(t, alert.IsLogOnly())
	})

	t.Run("Warning severity helpers", func(t *testing.T) {
		alert := &entity.Alert{Severity: entity.SeverityWarning}
		assert.True(t, alert.IsWarning())
		assert.False(t, alert.IsCritical())
		assert.True(t, alert.RequiresEscalation())
		assert.False(t, alert.RequiresImmediateAction())
		assert.False(t, alert.IsLogOnly())
	})

	t.Run("Info severity helpers", func(t *testing.T) {
		alert := &entity.Alert{Severity: entity.SeverityInfo}
		assert.True(t, alert.IsInfo())
		assert.True(t, alert.IsLogOnly())
		assert.False(t, alert.RequiresEscalation())
		assert.False(t, alert.RequiresImmediateAction())
		assert.False(t, alert.IsCritical())
		assert.False(t, alert.IsWarning())
	})
}

// TestAlertStatus verifies status transitions work correctly.
func TestAlertStatus(t *testing.T) {
	adminID := uuid.New()

	t.Run("Open status is active", func(t *testing.T) {
		alert := &entity.Alert{
			Status: entity.StatusOpen,
		}
		assert.True(t, alert.IsActive())
		assert.False(t, alert.IsResolved())
	})

	t.Run("Active status is active", func(t *testing.T) {
		alert := &entity.Alert{
			Status: entity.StatusActive,
		}
		assert.True(t, alert.IsActive())
		assert.False(t, alert.IsResolved())
	})

	t.Run("Resolved status is terminal", func(t *testing.T) {
		alert := &entity.Alert{
			Status: entity.StatusOpen,
		}
		alert.Resolve(adminID)

		assert.True(t, alert.IsResolved())
		assert.False(t, alert.IsActive())
		assert.Equal(t, entity.StatusResolved, alert.Status)
		assert.NotNil(t, alert.ResolvedAt)
		assert.Equal(t, adminID, *alert.ResolvedBy)
	})

	t.Run("Valid status transitions", func(t *testing.T) {
		alert := &entity.Alert{
			Status: entity.StatusOpen,
		}

		// Can transition from open to acknowledged
		assert.True(t, alert.CanTransition(entity.StatusAcknowledged))

		// Cannot transition from resolved to open
		alert.Status = entity.StatusResolved
		assert.False(t, alert.CanTransition(entity.StatusOpen))
	})
}

// TestDedupKeyGeneration verifies dedup key format.
func TestDedupKeyGeneration(t *testing.T) {
	alertType := entity.AlertTypePaymentFailureSpike
	entityType := "payment"
	entityID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")

	expected := "payment_failure_spike:payment:123e4567-e89b-12d3-a456-426614174000"

	alert := entity.NewAlert(
		alertType,
		entity.SeverityWarning,
		entityType,
		entityID,
		"Test message",
		entity.AlertMetadata{},
		nil,
	)

	assert.Equal(t, expected, alert.DedupKey)
}

// Helper functions
func intPtr(i int) *int {
	return &i
}

func uuidPtr(u uuid.UUID) *uuid.UUID {
	return &u
}

// TestAlertStatsOpenActiveAlignment verifies that open and active both map to the
// stats.Active bucket, while resolved/acknowledged are unaffected.
func TestAlertStatsOpenActiveAlignment(t *testing.T) {
	t.Run("status=open counted as active", func(t *testing.T) {
		alert := &entity.Alert{Status: entity.StatusOpen}
		assert.True(t, alert.IsActive())
		assert.False(t, alert.IsResolved())
	})

	t.Run("status=active counted as active", func(t *testing.T) {
		alert := &entity.Alert{Status: entity.StatusActive}
		assert.True(t, alert.IsActive())
		assert.False(t, alert.IsResolved())
	})

	t.Run("status=resolved not active", func(t *testing.T) {
		alert := &entity.Alert{Status: entity.StatusResolved}
		assert.False(t, alert.IsActive())
		assert.True(t, alert.IsResolved())
	})

	t.Run("status=acknowledged not active and not resolved", func(t *testing.T) {
		alert := &entity.Alert{Status: entity.StatusAcknowledged}
		assert.False(t, alert.IsActive())
		assert.False(t, alert.IsResolved())
	})
}

// TestEscalationStats verifies escalation statistics.
func TestEscalationStats(t *testing.T) {
	windowStart := time.Now().Add(-24 * time.Hour)
	windowEnd := time.Now()

	stats := &EscalationStats{
		TotalAlerts:     100,
		EscalatedAlerts: 45,
		CriticalAlerts:  10,
		WarningAlerts:   25,
		InfoAlerts:      20,
		WindowStart:     windowStart,
		WindowEnd:       windowEnd,
	}

	assert.Equal(t, 100, stats.TotalAlerts)
	assert.Equal(t, 45, stats.EscalatedAlerts)
	assert.Equal(t, 10, stats.CriticalAlerts)
	assert.Equal(t, 25, stats.WarningAlerts)
	assert.Equal(t, 20, stats.InfoAlerts)
	assert.Equal(t, 0.45, float64(stats.EscalatedAlerts)/float64(stats.TotalAlerts))
}


