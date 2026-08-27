package entity

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Alert represents a system alert for anomaly detection.
//
// DESIGN PRINCIPLES:
// - Alert domain is OBSERVATIONAL only - does not modify other domains
// - Alerts are created by workers detecting anomalies
// - Alerts can be resolved by admins
// - Metadata JSONB stores alert-specific data flexibly
// - DedupKey prevents duplicate alerts within time window
type Alert struct {
	ID          uuid.UUID          `json:"id"`
	AlertType   AlertType          `json:"alert_type"`
	Severity    AlertSeverity      `json:"severity"`
	EntityType  string             `json:"entity_type"`
	EntityID    uuid.UUID          `json:"entity_id"`
	Message     string             `json:"message"`
	Metadata    AlertMetadata      `json:"metadata"`
	Status      AlertStatus        `json:"status"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
	ResolvedAt  *time.Time         `json:"resolved_at,omitempty"`
	ResolvedBy  *uuid.UUID         `json:"resolved_by,omitempty"`
	GroupKey    *string            `json:"group_key,omitempty"` // For grouping similar alerts
	DedupKey    string             `json:"dedup_key"`           // Auto-generated: type+entity_id for deduplication
	DedupWindow *int               `json:"dedup_window,omitempty"` // Time window in minutes for dedup
}

// AlertType defines the type of alert.
type AlertType string

const (
	// AlertTypePaymentFailureSpike detects sudden increase in payment failures
	AlertTypePaymentFailureSpike AlertType = "payment_failure_spike"
	// AlertTypePaymentStuck detects payments stuck in pending state
	AlertTypePaymentStuck AlertType = "payment_stuck"
	// AlertTypeDisputeSpike detects sudden increase in disputes
	AlertTypeDisputeSpike AlertType = "dispute_spike"
	// AlertTypeSellerRisk detects sellers with high risk metrics
	AlertTypeSellerRisk AlertType = "seller_risk"
	// AlertTypeCoinsAnomaly detects unusual coin activity
	AlertTypeCoinsAnomaly AlertType = "coins_anomaly"
	// AlertTypeWithdrawalAnomaly detects suspicious withdrawal patterns
	AlertTypeWithdrawalAnomaly AlertType = "withdrawal_anomaly"
	// AlertTypeReconciliationDrift detects ledger/account balance mismatches
	AlertTypeReconciliationDrift AlertType = "reconciliation_drift"
	// AlertTypeRefundGatewayFailed detects gateway refund dispatch/ack failures
	AlertTypeRefundGatewayFailed AlertType = "refund_gateway_failed"
	// AlertTypeStaleDisputeFreeze detects active dispute freezes older than threshold
	AlertTypeStaleDisputeFreeze AlertType = "stale_dispute_freeze"
	// AlertTypeOutboxDLQSpike detects sudden increase in outbox dead-letter events
	AlertTypeOutboxDLQSpike AlertType = "outbox_dlq_spike"
	// AlertTypeOutboxStuck detects outbox events stuck in processing state
	AlertTypeOutboxStuck AlertType = "outbox_stuck"
	// AlertTypeSellerNonShipment detects sellers with repeated non-shipment (cancelled_timeout)
	AlertTypeSellerNonShipment AlertType = "seller_non_shipment"
	// AlertTypeEscrowStuck detects orders where escrow has been in holding state longer than threshold
	AlertTypeEscrowStuck AlertType = "escrow_stuck"
	// AlertTypeOrderPaidStuck detects orders stuck in paid state (awaiting shipment) longer than threshold
	AlertTypeOrderPaidStuck AlertType = "order_paid_stuck"
	// AlertTypeOrderShippedStuck detects orders stuck in shipped state (awaiting delivery confirmation) longer than threshold
	AlertTypeOrderShippedStuck AlertType = "order_shipped_stuck"
	// AlertTypeDisputeOpenStuck detects disputes stuck in under_review state longer than threshold
	AlertTypeDisputeOpenStuck AlertType = "dispute_open_stuck"
	// AlertTypeSubscriptionOrphanedPayment detects settled subscription payments with no matching subscription row
	AlertTypeSubscriptionOrphanedPayment AlertType = "subscription_orphaned_payment"
	// AlertTypeSubscriptionConversionRate detects low payment→subscription conversion rates
	AlertTypeSubscriptionConversionRate AlertType = "subscription_conversion_rate"
	// AlertTypeSubscriptionLifecycle detects active subscriptions stuck past their expiry date
	AlertTypeSubscriptionLifecycle AlertType = "subscription_lifecycle"
	// AlertTypePaymentCapturedAfterExpiry detects a gateway success webhook
	// arriving for a payment/order the platform already expired — money may
	// be captured at the gateway with no platform-side reconciliation.
	AlertTypePaymentCapturedAfterExpiry AlertType = "payment_captured_after_expiry"
)

// AlertSeverity defines the severity level of an alert.
type AlertSeverity string

const (
	// SeverityLow is for informational alerts
	SeverityLow AlertSeverity = "low"
	// SeverityMedium requires attention
	SeverityMedium AlertSeverity = "medium"
	// SeverityHigh requires urgent attention
	SeverityHigh AlertSeverity = "high"
	// SeverityCritical requires immediate action
	SeverityCritical AlertSeverity = "critical"
	// SeverityWarning requires attention (used by detection rules)
	SeverityWarning AlertSeverity = "warning"
	// SeverityInfo is for informational only (used by detection rules)
	SeverityInfo AlertSeverity = "info"
)

// AlertStatus defines the status of an alert.
type AlertStatus string

const (
	// StatusActive is for unresolved alerts (pre-migration-000177 value; rows may carry this)
	StatusActive AlertStatus = "active"
	// StatusOpen is for unresolved alerts (migration-000177 canonical value; NewAlert() uses this)
	StatusOpen AlertStatus = "open"
	// StatusAcknowledged is for alerts seen by admins but not resolved
	StatusAcknowledged AlertStatus = "acknowledged"
	// StatusResolved is for alerts that have been addressed
	StatusResolved AlertStatus = "resolved"
	// StatusFalsePositive is for alerts that were invalid
	StatusFalsePositive AlertStatus = "false_positive"
)

// AlertMetadata is a flexible JSON metadata container.
type AlertMetadata map[string]interface{}

// NewAlert creates a new Alert with automatic dedup_key generation.
func NewAlert(
	alertType AlertType,
	severity AlertSeverity,
	entityType string,
	entityID uuid.UUID,
	message string,
	metadata AlertMetadata,
	groupKey *string,
) *Alert {
	now := time.Now()

	// Generate automatic dedup_key from alert_type + entity_type + entity_id
	dedupKey := generateDedupKey(alertType, entityType, entityID)

	return &Alert{
		ID:         uuid.New(),
		AlertType:  alertType,
		Severity:   severity,
		EntityType: entityType,
		EntityID:   entityID,
		Message:    message,
		Metadata:   metadata,
		Status:     StatusOpen,
		CreatedAt:  now,
		UpdatedAt:  now,
		GroupKey:   groupKey,
		DedupKey:   dedupKey,
	}
}

// generateDedupKey creates a unique deduplication key from alert components.
func generateDedupKey(alertType AlertType, entityType string, entityID uuid.UUID) string {
	return fmt.Sprintf("%s:%s:%s", string(alertType), entityType, entityID.String())
}

// NewAlertWithDedupWindow creates a new Alert with custom deduplication window.
func NewAlertWithDedupWindow(
	alertType AlertType,
	severity AlertSeverity,
	entityType string,
	entityID uuid.UUID,
	message string,
	metadata AlertMetadata,
	groupKey *string,
	dedupWindowMinutes int,
) *Alert {
	alert := NewAlert(alertType, severity, entityType, entityID, message, metadata, groupKey)
	alert.DedupWindow = &dedupWindowMinutes
	return alert
}

// Acknowledge marks the alert as acknowledged.
func (a *Alert) Acknowledge(resolvedBy uuid.UUID) {
	a.Status = StatusAcknowledged
	a.UpdatedAt = time.Now()
	a.ResolvedBy = &resolvedBy
}

// Resolve marks the alert as resolved.
func (a *Alert) Resolve(resolvedBy uuid.UUID) {
	now := time.Now()
	a.Status = StatusResolved
	a.UpdatedAt = now
	a.ResolvedAt = &now
	a.ResolvedBy = &resolvedBy
}

// MarkAsFalsePositive marks the alert as a false positive.
func (a *Alert) MarkAsFalsePositive(resolvedBy uuid.UUID) {
	now := time.Now()
	a.Status = StatusFalsePositive
	a.UpdatedAt = now
	a.ResolvedAt = &now
	a.ResolvedBy = &resolvedBy
}

// IsActive returns true if the alert is still active (open or active).
func (a *Alert) IsActive() bool {
	return a.Status == StatusActive || a.Status == StatusOpen
}

// IsResolved returns true if the alert is resolved or false positive.
func (a *Alert) IsResolved() bool {
	return a.Status == StatusResolved || a.Status == StatusFalsePositive
}

// IsCritical returns true if the alert is critical severity.
func (a *Alert) IsCritical() bool {
	return a.Severity == SeverityCritical
}

// IsWarning returns true if the alert is warning severity.
func (a *Alert) IsWarning() bool {
	return a.Severity == SeverityWarning
}

// IsInfo returns true if the alert is info severity.
func (a *Alert) IsInfo() bool {
	return a.Severity == SeverityInfo
}

// RequiresImmediateAction returns true if the alert requires immediate action (critical).
func (a *Alert) RequiresImmediateAction() bool {
	return a.IsCritical()
}

// RequiresEscalation returns true if the alert should be escalated (warning or critical).
func (a *Alert) RequiresEscalation() bool {
	return a.IsCritical() || a.IsWarning()
}

// IsLogOnly returns true if the alert is for logging purposes only (info).
func (a *Alert) IsLogOnly() bool {
	return a.IsInfo()
}

// ToJSON converts metadata to JSON bytes.
func (m AlertMetadata) ToJSON() []byte {
	if m == nil {
		return []byte("{}")
	}
	data, _ := json.Marshal(m)
	return data
}

// FromJSON parses JSON bytes into AlertMetadata.
func FromJSON(data []byte) AlertMetadata {
	if len(data) == 0 {
		return AlertMetadata{}
	}
	var result AlertMetadata
	_ = json.Unmarshal(data, &result)
	return result
}

// ValidTransitions defines valid status transitions.
var ValidTransitions = map[AlertStatus][]AlertStatus{
	StatusActive:        {StatusAcknowledged, StatusResolved, StatusFalsePositive},
	StatusOpen:          {StatusAcknowledged, StatusResolved, StatusFalsePositive}, // Alias for active
	StatusAcknowledged:  {StatusResolved, StatusFalsePositive, StatusActive, StatusOpen},
	StatusResolved:      {}, // Terminal state
	StatusFalsePositive: {}, // Terminal state
}

// CanTransition checks if a status transition is valid.
func (a *Alert) CanTransition(newStatus AlertStatus) bool {
	validStates, ok := ValidTransitions[a.Status]
	if !ok {
		return false
	}
	for _, valid := range validStates {
		if valid == newStatus {
			return true
		}
	}
	return false
}


