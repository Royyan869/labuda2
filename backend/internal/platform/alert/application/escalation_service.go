package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/alert/entity"
	"github.com/labuda/backend/internal/platform/alert/repository"
	"go.uber.org/zap"
)

// EscalationPolicy defines how alerts should be escalated.
type EscalationPolicy struct {
	// CriticalAlerts are escalated immediately
	CriticalImmediately bool
	// WarningThresholds defines warning escalation thresholds
	WarningThresholds map[string]int // alert_type -> count
	// WarningWindowMinutes is the time window for warning threshold
	WarningWindowMinutes int
	// InfoLogOnly means info alerts are only logged
	InfoLogOnly bool
}

// DefaultEscalationPolicy returns the default escalation policy.
func DefaultEscalationPolicy() *EscalationPolicy {
	return &EscalationPolicy{
		CriticalImmediately:  true,
		WarningThresholds:    map[string]int{},
		WarningWindowMinutes: 60, // 1 hour
		InfoLogOnly:          true,
	}
}

// EscalationService handles alert escalation logic.
type EscalationService struct {
	alertService *AlertService
	policy       *EscalationPolicy
	log          *zap.Logger
}

// NewEscalationService creates a new EscalationService.
func NewEscalationService(
	alertService *AlertService,
	policy *EscalationPolicy,
	log *zap.Logger,
) *EscalationService {
	if log == nil {
		log = zap.NewNop()
	}
	if policy == nil {
		policy = DefaultEscalationPolicy()
	}
	return &EscalationService{
		alertService: alertService,
		policy:       policy,
		log:          log,
	}
}

// ShouldEscalate determines if an alert should be escalated.
func (s *EscalationService) ShouldEscalate(alert *entity.Alert) bool {
	// Critical alerts always escalate
	if alert.IsCritical() && s.policy.CriticalImmediately {
		return true
	}

	// Warning alerts escalate based on thresholds
	if alert.IsWarning() {
		threshold, exists := s.policy.WarningThresholds[string(alert.AlertType)]
		if exists {
			// Check if occurrence count exceeds threshold
			if count, ok := alert.Metadata["occurrence_count"].(int); ok {
				return count >= threshold
			}
		}
		// Default: warning alerts escalate
		return true
	}

	// Info alerts never escalate (log only)
	if alert.IsInfo() && s.policy.InfoLogOnly {
		return false
	}

	// Legacy severity levels
	if alert.Severity == entity.SeverityHigh || alert.Severity == entity.SeverityMedium {
		return true
	}

	// Low severity legacy alerts don't escalate
	if alert.Severity == entity.SeverityLow {
		return false
	}

	return false
}

// EscalationAction represents an escalation action.
type EscalationAction struct {
	ShouldEscalate bool
	Reason         string
	Channel        string
	Priority       int
}

// GetEscalationAction returns the escalation action for an alert.
func (s *EscalationService) GetEscalationAction(alert *entity.Alert) *EscalationAction {
	if !s.ShouldEscalate(alert) {
		return &EscalationAction{
			ShouldEscalate: false,
			Reason:         "Log only - does not meet escalation criteria",
			Channel:        "log",
			Priority:       0,
		}
	}

	if alert.IsCritical() {
		return &EscalationAction{
			ShouldEscalate: true,
			Reason:         "Critical severity - immediate escalation",
			Channel:        "immediate",
			Priority:       100,
		}
	}

	if alert.IsWarning() {
		threshold, exists := s.policy.WarningThresholds[string(alert.AlertType)]
		if exists {
			if count, ok := alert.Metadata["occurrence_count"].(int); ok && count >= threshold {
				return &EscalationAction{
					ShouldEscalate: true,
					Reason:         fmt.Sprintf("Warning severity - threshold met (%d/%d)", count, threshold),
					Channel:        "threshold_based",
					Priority:       50,
				}
			}
		}
		return &EscalationAction{
			ShouldEscalate: true,
			Reason:         "Warning severity - standard escalation",
			Channel:        "standard",
			Priority:       50,
		}
	}

	// Legacy severity levels
	if alert.Severity == entity.SeverityHigh {
		return &EscalationAction{
			ShouldEscalate: true,
			Reason:         "High severity - standard escalation",
			Channel:        "standard",
			Priority:       70,
		}
	}

	if alert.Severity == entity.SeverityMedium {
		return &EscalationAction{
			ShouldEscalate: true,
			Reason:         "Medium severity - standard escalation",
			Channel:        "standard",
			Priority:       40,
		}
	}

	return &EscalationAction{
		ShouldEscalate: false,
		Reason:         "Unknown severity - log only",
		Channel:        "log",
		Priority:       0,
	}
}

// CreateAlertWithEscalation creates an alert and returns escalation action.
func (s *EscalationService) CreateAlertWithEscalation(
	ctx context.Context,
	alertType entity.AlertType,
	severity entity.AlertSeverity,
	entityType string,
	entityID uuid.UUID,
	message string,
	metadata entity.AlertMetadata,
	groupKey *string,
) (*CreateAlertResult, *EscalationAction, error) {
	result, err := s.alertService.CreateAlert(
		ctx,
		alertType,
		severity,
		entityType,
		entityID,
		message,
		metadata,
		groupKey,
	)

	if err != nil {
		return nil, nil, err
	}

	action := s.GetEscalationAction(result.Alert)

	s.log.Info("Alert escalation evaluated",
		zap.String("alert_id", result.Alert.ID.String()),
		zap.String("alert_type", string(alertType)),
		zap.String("severity", string(severity)),
		zap.Bool("should_escalate", action.ShouldEscalate),
		zap.String("reason", action.Reason),
		zap.String("channel", action.Channel),
		zap.Int("priority", action.Priority),
	)

	return result, action, nil
}

// SetWarningThreshold sets a warning threshold for an alert type.
func (s *EscalationService) SetWarningThreshold(alertType entity.AlertType, threshold int) {
	if s.policy.WarningThresholds == nil {
		s.policy.WarningThresholds = map[string]int{}
	}
	s.policy.WarningThresholds[string(alertType)] = threshold

	s.log.Info("Warning threshold updated",
		zap.String("alert_type", string(alertType)),
		zap.Int("threshold", threshold),
	)
}

// RemoveWarningThreshold removes a warning threshold.
func (s *EscalationService) RemoveWarningThreshold(alertType entity.AlertType) {
	delete(s.policy.WarningThresholds, string(alertType))

	s.log.Info("Warning threshold removed",
		zap.String("alert_type", string(alertType)),
	)
}

// GetEscalationStats returns escalation statistics for a time window.
type EscalationStats struct {
	TotalAlerts      int
	EscalatedAlerts  int
	CriticalAlerts   int
	WarningAlerts    int
	InfoAlerts       int
	WindowStart      time.Time
	WindowEnd        time.Time
}

// GetEscalationStats returns statistics for alerts within a time window.
func (s *EscalationService) GetEscalationStats(
	ctx context.Context,
	windowStart time.Time,
	windowEnd time.Time,
) (*EscalationStats, error) {
	filters := repository.AlertFilters{
		DateFrom: &windowStart,
		DateTo:   &windowEnd,
		Limit:    1000,
	}

	response, err := s.alertService.ListAlerts(ctx, filters)
	if err != nil {
		return nil, err
	}

	stats := &EscalationStats{
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
	}

	for _, alert := range response.Alerts {
		stats.TotalAlerts++

		if alert.IsCritical() {
			stats.CriticalAlerts++
		} else if alert.IsWarning() {
			stats.WarningAlerts++
		} else if alert.IsInfo() {
			stats.InfoAlerts++
		}

		if s.ShouldEscalate(alert) {
			stats.EscalatedAlerts++
		}
	}

	return stats, nil
}


