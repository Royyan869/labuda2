package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/alert/entity"
	"github.com/labuda/backend/internal/platform/alert/repository"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// Transactor represents the ability to execute functions within transactions.
type Transactor interface {
	WithTx(ctx context.Context, fn func(tx db.Tx) error) error
}

// AlertService handles alert business logic.
type AlertService struct {
	db   Transactor
	repo repository.AlertRepository
	log  *zap.Logger
}

// NewAlertService creates a new AlertService.
func NewAlertService(
	db Transactor,
	repo repository.AlertRepository,
	log *zap.Logger,
) *AlertService {
	if log == nil {
		log = zap.NewNop()
	}
	return &AlertService{
		db:   db,
		repo: repo,
		log:  log,
	}
}

// CreateAlertResult represents the result of creating an alert.
type CreateAlertResult struct {
	Alert      *entity.Alert
	Created    bool
	Reason     string
	ExistingID *uuid.UUID
}

// CreateAlert creates a new alert with duplicate prevention and grouping.
//
// DUPLICATE PREVENTION:
// - Checks for alerts with the same dedup_key within time window
// - If found, updates existing alert instead of creating duplicate
//
// TIME WINDOW:
// - Default dedup window: 60 minutes
// - Can be customized via NewAlertWithDedupWindow
//
// GROUPING:
// - Uses group_key for manual grouping of related alerts
// - Uses dedup_key for automatic duplicate prevention
func (s *AlertService) CreateAlert(
	ctx context.Context,
	alertType entity.AlertType,
	severity entity.AlertSeverity,
	entityType string,
	entityID uuid.UUID,
	message string,
	metadata entity.AlertMetadata,
	groupKey *string,
) (*CreateAlertResult, error) {
	return s.CreateAlertWithDedupWindow(
		ctx,
		alertType,
		severity,
		entityType,
		entityID,
		message,
		metadata,
		groupKey,
		60, // Default 60 minute dedup window
	)
}

// CreateAlertWithDedupWindow creates a new alert with custom dedup window.
func (s *AlertService) CreateAlertWithDedupWindow(
	ctx context.Context,
	alertType entity.AlertType,
	severity entity.AlertSeverity,
	entityType string,
	entityID uuid.UUID,
	message string,
	metadata entity.AlertMetadata,
	groupKey *string,
	dedupWindowMinutes int,
) (*CreateAlertResult, error) {
	var result *CreateAlertResult

	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		// Generate dedup_key for this alert
		dedupKey := s.generateDedupKey(alertType, entityType, entityID)

		// Check for existing alert with same dedup_key within time window
		existing, err := s.repo.FindByDedupKeyInWindow(ctx, tx, dedupKey, dedupWindowMinutes)
		if err != nil {
			return fmt.Errorf("check existing alerts failed: %w", err)
		}

		if len(existing) > 0 {
			// Found existing alert within time window - update instead of creating duplicate
			existingAlert := existing[0]

			// Only update if the existing alert is still active/open
			if existingAlert.IsActive() {
				// Merge in the latest occurrence's domain details (e.g. reconciliation
				// account_mismatches, totals) so admins see current state rather than a
				// stale first-occurrence snapshot, while preserving dedup bookkeeping.
				s.mergeOccurrenceMetadata(existingAlert, metadata)

				// Update severity if new alert is more severe
				if s.isMoreSevere(severity, existingAlert.Severity) {
					existingAlert.Severity = severity
				}

				existingAlert.UpdatedAt = time.Now()

				if err := s.repo.Update(ctx, tx, existingAlert); err != nil {
					return fmt.Errorf("update existing alert failed: %w", err)
				}

				result = &CreateAlertResult{
					Alert:      existingAlert,
					Created:    false,
					Reason:     "Updated existing alert (dedup)",
					ExistingID: &existingAlert.ID,
				}

				s.log.Info("Alert deduplicated",
					zap.String("alert_type", string(alertType)),
					zap.String("dedup_key", dedupKey),
					zap.String("existing_id", existingAlert.ID.String()),
					zap.Int("window_minutes", dedupWindowMinutes),
				)

				return nil
			}
		}

		// Check for manual group_key grouping (for backward compatibility)
		if groupKey != nil {
			groupedAlerts, err := s.repo.FindActiveByGroupKey(ctx, tx, *groupKey)
			if err != nil {
				return fmt.Errorf("check grouped alerts failed: %w", err)
			}

			if len(groupedAlerts) > 0 {
				// Found active alert with same group key
				existingAlert := groupedAlerts[0]

				// Merge in the latest occurrence's domain details (see dedup-key
				// branch above) so admins see current state, not a stale snapshot.
				s.mergeOccurrenceMetadata(existingAlert, metadata)

				// Update severity if new alert is more severe
				if s.isMoreSevere(severity, existingAlert.Severity) {
					existingAlert.Severity = severity
				}

				existingAlert.UpdatedAt = time.Now()

				if err := s.repo.Update(ctx, tx, existingAlert); err != nil {
					return fmt.Errorf("update existing alert failed: %w", err)
				}

				result = &CreateAlertResult{
					Alert:      existingAlert,
					Created:    false,
					Reason:     "Updated existing alert (group_key)",
					ExistingID: &existingAlert.ID,
				}

				s.log.Info("Alert grouped with existing alert",
					zap.String("alert_type", string(alertType)),
					zap.String("group_key", *groupKey),
					zap.String("existing_id", existingAlert.ID.String()),
				)

				return nil
			}
		}

		// Create new alert
		alert := entity.NewAlertWithDedupWindow(
			alertType,
			severity,
			entityType,
			entityID,
			message,
			metadata,
			groupKey,
			dedupWindowMinutes,
		)

		// Initialize occurrence count
		if alert.Metadata == nil {
			alert.Metadata = entity.AlertMetadata{}
		}
		alert.Metadata["occurrence_count"] = 1
		alert.Metadata["first_occurrence"] = time.Now()

		if err := s.repo.Create(ctx, tx, alert); err != nil {
			return fmt.Errorf("create alert failed: %w", err)
		}

		result = &CreateAlertResult{
			Alert:   alert,
			Created: true,
			Reason:  "New alert created",
		}

		s.log.Info("Alert created",
			zap.String("alert_id", alert.ID.String()),
			zap.String("alert_type", string(alertType)),
			zap.String("severity", string(severity)),
			zap.String("entity_type", entityType),
			zap.String("entity_id", entityID.String()),
			zap.String("dedup_key", alert.DedupKey),
		)

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// generateDedupKey creates a deduplication key from alert components.
func (s *AlertService) generateDedupKey(alertType entity.AlertType, entityType string, entityID uuid.UUID) string {
	return fmt.Sprintf("%s:%s:%s", string(alertType), entityType, entityID.String())
}

// AcknowledgeAlert marks an alert as acknowledged.
func (s *AlertService) AcknowledgeAlert(
	ctx context.Context,
	alertID uuid.UUID,
	adminID uuid.UUID,
) error {
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		alert, err := s.repo.GetForUpdate(ctx, tx, alertID)
		if err != nil {
			return err
		}

		if !alert.CanTransition(entity.StatusAcknowledged) {
			return fmt.Errorf("cannot transition from %s to acknowledged", alert.Status)
		}

		alert.Acknowledge(adminID)

		if err := s.repo.Update(ctx, tx, alert); err != nil {
			return err
		}

		s.log.Info("Alert acknowledged",
			zap.String("alert_id", alertID.String()),
			zap.String("admin_id", adminID.String()),
		)

		return nil
	})
}

// ResolveAlert marks an alert as resolved.
func (s *AlertService) ResolveAlert(
	ctx context.Context,
	alertID uuid.UUID,
	adminID uuid.UUID,
) error {
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		alert, err := s.repo.GetForUpdate(ctx, tx, alertID)
		if err != nil {
			return err
		}

		if !alert.CanTransition(entity.StatusResolved) {
			return fmt.Errorf("cannot transition from %s to resolved", alert.Status)
		}

		alert.Resolve(adminID)

		if err := s.repo.Update(ctx, tx, alert); err != nil {
			return err
		}

		s.log.Info("Alert resolved",
			zap.String("alert_id", alertID.String()),
			zap.String("admin_id", adminID.String()),
		)

		return nil
	})
}

// MarkAsFalsePositive marks an alert as a false positive.
func (s *AlertService) MarkAsFalsePositive(
	ctx context.Context,
	alertID uuid.UUID,
	adminID uuid.UUID,
) error {
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		alert, err := s.repo.GetForUpdate(ctx, tx, alertID)
		if err != nil {
			return err
		}

		if !alert.CanTransition(entity.StatusFalsePositive) {
			return fmt.Errorf("cannot transition from %s to false_positive", alert.Status)
		}

		alert.MarkAsFalsePositive(adminID)

		if err := s.repo.Update(ctx, tx, alert); err != nil {
			return err
		}

		s.log.Info("Alert marked as false positive",
			zap.String("alert_id", alertID.String()),
			zap.String("admin_id", adminID.String()),
		)

		return nil
	})
}

// GetAlert retrieves a single alert by ID.
func (s *AlertService) GetAlert(ctx context.Context, alertID uuid.UUID) (*entity.Alert, error) {
	var alert *entity.Alert
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		alert, err = s.repo.GetByID(ctx, tx, alertID)
		return err
	})
	return alert, err
}

// ListAlertsResponse represents the response for listing alerts.
type ListAlertsResponse struct {
	Alerts []*entity.Alert `json:"alerts"`
	Total  int64           `json:"total"`
}

// ListAlerts retrieves alerts with filtering and pagination.
func (s *AlertService) ListAlerts(
	ctx context.Context,
	filters repository.AlertFilters,
) (*ListAlertsResponse, error) {
	var result *ListAlertsResponse

	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		alerts, err := s.repo.List(ctx, tx, filters)
		if err != nil {
			return err
		}

		total, err := s.repo.Count(ctx, tx, filters)
		if err != nil {
			return err
		}

		result = &ListAlertsResponse{
			Alerts: alerts,
			Total:  total,
		}
		return nil
	})

	return result, err
}

// CleanupOldAlerts deletes resolved alerts older than the specified duration.
func (s *AlertService) CleanupOldAlerts(ctx context.Context, olderThanDays int) (int, error) {
	var deleted int
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		deleted, err = s.repo.DeleteOld(ctx, tx, olderThanDays)
		return err
	})

	if err != nil {
		return 0, err
	}

	s.log.Info("Old alerts cleaned up",
		zap.Int("deleted", deleted),
		zap.Int("older_than_days", olderThanDays),
	)

	return deleted, nil
}

// isMoreSevere checks if newSeverity is more severe than existing.
// mergeOccurrenceMetadata folds a new occurrence's domain metadata into an
// existing (deduplicated) alert's metadata, so the alert reflects the latest
// known state rather than a stale first-occurrence snapshot. Dedup bookkeeping
// keys (occurrence_count, first_occurrence, last_occurrence) are preserved and
// updated independently of whatever the caller passes in.
func (s *AlertService) mergeOccurrenceMetadata(existingAlert *entity.Alert, latest entity.AlertMetadata) {
	if existingAlert.Metadata == nil {
		existingAlert.Metadata = entity.AlertMetadata{}
	}

	firstOccurrence := existingAlert.Metadata["first_occurrence"]
	count, _ := existingAlert.Metadata["occurrence_count"].(int)

	for k, v := range latest {
		existingAlert.Metadata[k] = v
	}

	existingAlert.Metadata["occurrence_count"] = count + 1
	existingAlert.Metadata["last_occurrence"] = time.Now()
	if firstOccurrence != nil {
		existingAlert.Metadata["first_occurrence"] = firstOccurrence
	}
}

func (s *AlertService) isMoreSevere(newSeverity, existingSeverity entity.AlertSeverity) bool {
	severityOrder := map[entity.AlertSeverity]int{
		entity.SeverityLow:      0,
		entity.SeverityMedium:   1,
		entity.SeverityHigh:     2,
		entity.SeverityCritical: 3,
	}

	return severityOrder[newSeverity] > severityOrder[existingSeverity]
}

// CreatePaymentFailureSpikeAlert creates a payment failure spike alert.
func (s *AlertService) CreatePaymentFailureSpikeAlert(
	ctx context.Context,
	failureCount int,
	windowMinutes int,
	threshold int,
) (*CreateAlertResult, error) {
	groupKey := fmt.Sprintf("payment_failure_spike:%d", windowMinutes)

	metadata := entity.AlertMetadata{
		"failure_count":  failureCount,
		"window_minutes": windowMinutes,
		"threshold":      threshold,
		"detected_at":    time.Now(),
	}

	severity := entity.SeverityMedium
	if failureCount > threshold*2 {
		severity = entity.SeverityHigh
	}
	if failureCount > threshold*5 {
		severity = entity.SeverityCritical
	}

	return s.CreateAlert(
		ctx,
		entity.AlertTypePaymentFailureSpike,
		severity,
		"system",
		uuid.Nil, // No specific entity
		fmt.Sprintf("Payment failure spike detected: %d failures in %d minutes (threshold: %d)",
			failureCount, windowMinutes, threshold),
		metadata,
		&groupKey,
	)
}

// CreateDisputeSpikeAlert creates a dispute spike alert.
func (s *AlertService) CreateDisputeSpikeAlert(
	ctx context.Context,
	disputeCount int,
	windowMinutes int,
	threshold int,
) (*CreateAlertResult, error) {
	groupKey := fmt.Sprintf("dispute_spike:%d", windowMinutes)

	metadata := entity.AlertMetadata{
		"dispute_count":  disputeCount,
		"window_minutes": windowMinutes,
		"threshold":      threshold,
		"detected_at":    time.Now(),
	}

	severity := entity.SeverityMedium
	if disputeCount > threshold*2 {
		severity = entity.SeverityHigh
	}
	if disputeCount > threshold*5 {
		severity = entity.SeverityCritical
	}

	return s.CreateAlert(
		ctx,
		entity.AlertTypeDisputeSpike,
		severity,
		"system",
		uuid.Nil,
		fmt.Sprintf("Dispute spike detected: %d disputes in %d minutes (threshold: %d)",
			disputeCount, windowMinutes, threshold),
		metadata,
		&groupKey,
	)
}

// CreateSellerRiskAlert creates a seller risk alert.
func (s *AlertService) CreateSellerRiskAlert(
	ctx context.Context,
	sellerID uuid.UUID,
	riskReason string,
	metadata entity.AlertMetadata,
) (*CreateAlertResult, error) {
	groupKey := fmt.Sprintf("seller_risk:%s", sellerID.String())

	metadata["risk_reason"] = riskReason
	metadata["detected_at"] = time.Now()

	return s.CreateAlert(
		ctx,
		entity.AlertTypeSellerRisk,
		entity.SeverityHigh,
		"seller",
		sellerID,
		fmt.Sprintf("Seller risk detected: %s", riskReason),
		metadata,
		&groupKey,
	)
}

// CreateCoinsAnomalyAlert creates a coins anomaly alert.
func (s *AlertService) CreateCoinsAnomalyAlert(
	ctx context.Context,
	userID uuid.UUID,
	anomalyType string,
	metadata entity.AlertMetadata,
) (*CreateAlertResult, error) {
	groupKey := fmt.Sprintf("coins_anomaly:%s:%s", userID.String(), anomalyType)

	metadata["anomaly_type"] = anomalyType
	metadata["detected_at"] = time.Now()

	return s.CreateAlert(
		ctx,
		entity.AlertTypeCoinsAnomaly,
		entity.SeverityHigh,
		"user",
		userID,
		fmt.Sprintf("Coins anomaly detected: %s", anomalyType),
		metadata,
		&groupKey,
	)
}

// CreateReconciliationDriftAlert creates a reconciliation drift alert.
// This is called when the reconciliation worker detects balance mismatches.
func (s *AlertService) CreateReconciliationDriftAlert(
	ctx context.Context,
	severity entity.AlertSeverity,
	mismatchedAccounts int,
	totalAccounts int,
	details entity.AlertMetadata,
) (*CreateAlertResult, error) {
	groupKey := "reconciliation_drift:active"

	details["mismatched_accounts"] = mismatchedAccounts
	details["total_accounts"] = totalAccounts
	details["detected_at"] = time.Now()

	return s.CreateAlert(
		ctx,
		entity.AlertTypeReconciliationDrift,
		severity,
		"system",
		uuid.Nil,
		fmt.Sprintf("Reconciliation drift detected: %d/%d accounts with mismatches",
			mismatchedAccounts, totalAccounts),
		details,
		&groupKey,
	)
}
