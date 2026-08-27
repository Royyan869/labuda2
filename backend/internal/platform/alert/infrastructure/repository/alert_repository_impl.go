package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/platform/alert/entity"
	"github.com/labuda/backend/internal/platform/alert/repository"
	"github.com/labuda/backend/pkg/db"
)

// AlertRepositoryImpl handles alert persistence using pgx-based DB layer.
type AlertRepositoryImpl struct{}

// NewAlertRepository creates a new AlertRepository.
func NewAlertRepository() repository.AlertRepository {
	return &AlertRepositoryImpl{}
}

// Create persists a new alert within a transaction.
func (r *AlertRepositoryImpl) Create(
	ctx context.Context,
	tx interface{},
	alertEntity *entity.Alert,
) error {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	var resolvedBy *uuid.UUID
	if alertEntity.ResolvedBy != nil {
		resolvedBy = alertEntity.ResolvedBy
	}

	var dedupWindow *int
	if alertEntity.DedupWindow != nil {
		dedupWindow = alertEntity.DedupWindow
	}

	_, err := dbTx.Exec(ctx, `
		INSERT INTO system_alerts (
			id, alert_type, severity, entity_type, entity_id,
			message, metadata_json, status, created_at, updated_at,
			resolved_at, resolved_by, group_key, dedup_key, dedup_window
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`,
		alertEntity.ID,
		string(alertEntity.AlertType),
		string(alertEntity.Severity),
		alertEntity.EntityType,
		alertEntity.EntityID,
		alertEntity.Message,
		alertEntity.Metadata.ToJSON(),
		string(alertEntity.Status),
		alertEntity.CreatedAt,
		alertEntity.UpdatedAt,
		alertEntity.ResolvedAt,
		resolvedBy,
		alertEntity.GroupKey,
		alertEntity.DedupKey,
		dedupWindow,
	)

	if err != nil {
		return fmt.Errorf("create alert failed: %w", err)
	}

	return nil
}

// GetByID retrieves an alert without locking (for read-only operations).
func (r *AlertRepositoryImpl) GetByID(
	ctx context.Context,
	tx interface{},
	alertID uuid.UUID,
) (*entity.Alert, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	var alertType, severity, entityType, status, dedupKey string
	var entityID uuid.UUID
	var message string
	var metadataJSON []byte
	var createdAt, updatedAt time.Time
	var resolvedAt *time.Time
	var resolvedBy *uuid.UUID
	var groupKey *string
	var dedupWindow *int

	err := dbTx.QueryRow(ctx, `
		SELECT id, alert_type, severity, entity_type, entity_id,
		       message, metadata_json, status, created_at, updated_at,
		       resolved_at, resolved_by, group_key, dedup_key, dedup_window
		FROM system_alerts
		WHERE id = $1
	`, alertID).Scan(
		&alertID, &alertType, &severity, &entityType, &entityID,
		&message, &metadataJSON, &status, &createdAt, &updatedAt,
		&resolvedAt, &resolvedBy, &groupKey, &dedupKey, &dedupWindow,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, fmt.Errorf("alert not found: %s", alertID)
		}
		return nil, fmt.Errorf("get alert failed: %w", err)
	}

	return &entity.Alert{
		ID:          alertID,
		AlertType:   entity.AlertType(alertType),
		Severity:    entity.AlertSeverity(severity),
		EntityType:  entityType,
		EntityID:    entityID,
		Message:     message,
		Metadata:    entity.FromJSON(metadataJSON),
		Status:      entity.AlertStatus(status),
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
		ResolvedAt:  resolvedAt,
		ResolvedBy:  resolvedBy,
		GroupKey:    groupKey,
		DedupKey:    dedupKey,
		DedupWindow: dedupWindow,
	}, nil
}

// GetForUpdate retrieves an alert with FOR UPDATE lock.
func (r *AlertRepositoryImpl) GetForUpdate(
	ctx context.Context,
	tx interface{},
	alertID uuid.UUID,
) (*entity.Alert, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	var alertType, severity, entityType, status, dedupKey string
	var entityID uuid.UUID
	var message string
	var metadataJSON []byte
	var createdAt, updatedAt time.Time
	var resolvedAt *time.Time
	var resolvedBy *uuid.UUID
	var groupKey *string
	var dedupWindow *int

	err := dbTx.QueryRow(ctx, `
		SELECT id, alert_type, severity, entity_type, entity_id,
		       message, metadata_json, status, created_at, updated_at,
		       resolved_at, resolved_by, group_key, dedup_key, dedup_window
		FROM system_alerts
		WHERE id = $1
		FOR UPDATE
	`, alertID).Scan(
		&alertID, &alertType, &severity, &entityType, &entityID,
		&message, &metadataJSON, &status, &createdAt, &updatedAt,
		&resolvedAt, &resolvedBy, &groupKey, &dedupKey, &dedupWindow,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, fmt.Errorf("alert not found: %s", alertID)
		}
		return nil, fmt.Errorf("get alert for update failed: %w", err)
	}

	return &entity.Alert{
		ID:          alertID,
		AlertType:   entity.AlertType(alertType),
		Severity:    entity.AlertSeverity(severity),
		EntityType:  entityType,
		EntityID:    entityID,
		Message:     message,
		Metadata:    entity.FromJSON(metadataJSON),
		Status:      entity.AlertStatus(status),
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
		ResolvedAt:  resolvedAt,
		ResolvedBy:  resolvedBy,
		GroupKey:    groupKey,
		DedupKey:    dedupKey,
		DedupWindow: dedupWindow,
	}, nil
}

// Update persists alert changes within a transaction.
func (r *AlertRepositoryImpl) Update(
	ctx context.Context,
	tx interface{},
	alertEntity *entity.Alert,
) error {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	var resolvedBy *uuid.UUID
	if alertEntity.ResolvedBy != nil {
		resolvedBy = alertEntity.ResolvedBy
	}

	var dedupWindow *int
	if alertEntity.DedupWindow != nil {
		dedupWindow = alertEntity.DedupWindow
	}

	_, err := dbTx.Exec(ctx, `
		UPDATE system_alerts
		SET status = $2,
		    severity = $3,
		    updated_at = $4,
		    resolved_at = $5,
		    resolved_by = $6,
		    metadata_json = $7,
		    dedup_window = $8
		WHERE id = $1
	`,
		alertEntity.ID,
		string(alertEntity.Status),
		string(alertEntity.Severity),
		alertEntity.UpdatedAt,
		alertEntity.ResolvedAt,
		resolvedBy,
		alertEntity.Metadata.ToJSON(),
		dedupWindow,
	)

	if err != nil {
		return fmt.Errorf("update alert failed: %w", err)
	}

	return nil
}

// List retrieves alerts with filtering and pagination.
func (r *AlertRepositoryImpl) List(
	ctx context.Context,
	tx interface{},
	filters repository.AlertFilters,
) ([]*entity.Alert, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	query, args := r.buildListQuery(filters)

	rows, err := dbTx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list alerts failed: %w", err)
	}
	defer rows.Close()

	var alerts []*entity.Alert
	for rows.Next() {
		alert, err := r.scanRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan alert failed: %w", err)
		}
		alerts = append(alerts, alert)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("list alerts scan failed: %w", rows.Err())
	}

	return alerts, nil
}

// Count returns the total count of alerts matching filters.
func (r *AlertRepositoryImpl) Count(
	ctx context.Context,
	tx interface{},
	filters repository.AlertFilters,
) (int64, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return 0, fmt.Errorf("invalid transaction type")
	}

	var conditions []string
	var args []interface{}
	argIdx := 1

	if len(filters.Statuses) > 0 {
		placeholders := make([]string, len(filters.Statuses))
		for i, s := range filters.Statuses {
			placeholders[i] = fmt.Sprintf("$%d", argIdx)
			args = append(args, string(s))
			argIdx++
		}
		conditions = append(conditions, "status IN ("+strings.Join(placeholders, ", ")+")")
	} else if filters.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, string(*filters.Status))
		argIdx++
	}
	if filters.Severity != nil {
		conditions = append(conditions, fmt.Sprintf("severity = $%d", argIdx))
		args = append(args, string(*filters.Severity))
		argIdx++
	}
	if filters.AlertType != nil {
		conditions = append(conditions, fmt.Sprintf("alert_type = $%d", argIdx))
		args = append(args, string(*filters.AlertType))
		argIdx++
	}
	if filters.EntityType != nil {
		conditions = append(conditions, fmt.Sprintf("entity_type = $%d", argIdx))
		args = append(args, *filters.EntityType)
		argIdx++
	}
	if filters.EntityID != nil {
		conditions = append(conditions, fmt.Sprintf("entity_id = $%d", argIdx))
		args = append(args, *filters.EntityID)
		argIdx++
	}
	if filters.GroupKey != nil {
		conditions = append(conditions, fmt.Sprintf("group_key = $%d", argIdx))
		args = append(args, *filters.GroupKey)
		argIdx++
	}
	if filters.DateFrom != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIdx))
		args = append(args, *filters.DateFrom)
		argIdx++
	}
	if filters.DateTo != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIdx))
		args = append(args, *filters.DateTo)
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + joinConditions(conditions)
	}

	query := fmt.Sprintf("SELECT COUNT(*) FROM system_alerts %s", whereClause)

	var count int64
	err := dbTx.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count alerts failed: %w", err)
	}

	return count, nil
}

// FindActiveByGroupKey finds active alerts with the given group key.
func (r *AlertRepositoryImpl) FindActiveByGroupKey(
	ctx context.Context,
	tx interface{},
	groupKey string,
) ([]*entity.Alert, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	// Query for both 'active' (legacy) and 'open' (current standard) statuses.
	// NewAlert() creates with StatusOpen; StatusActive is the legacy value from
	// pre-migration 000177 rows. Both represent unresolved alerts.
	query := `
		SELECT id, alert_type, severity, entity_type, entity_id,
		       message, metadata_json, status, created_at, updated_at,
		       resolved_at, resolved_by, group_key, dedup_key, dedup_window
		FROM system_alerts
		WHERE group_key = $1 AND status = ANY($2)
		ORDER BY created_at DESC
	`

	activeStatuses := []entity.AlertStatus{entity.StatusActive, entity.StatusOpen}
	rows, err := dbTx.Query(ctx, query, groupKey, activeStatuses)
	if err != nil {
		return nil, fmt.Errorf("find active by group key failed: %w", err)
	}
	defer rows.Close()

	var alerts []*entity.Alert
	for rows.Next() {
		alert, err := r.scanRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan alert failed: %w", err)
		}
		alerts = append(alerts, alert)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("find active by group key scan failed: %w", rows.Err())
	}

	return alerts, nil
}

// FindByDedupKeyInWindow finds alerts with the same dedup_key within time window.
func (r *AlertRepositoryImpl) FindByDedupKeyInWindow(
	ctx context.Context,
	tx interface{},
	dedupKey string,
	minutes int,
) ([]*entity.Alert, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	window := time.Now().Add(-time.Duration(minutes) * time.Minute)

	query := `
		SELECT id, alert_type, severity, entity_type, entity_id,
		       message, metadata_json, status, created_at, updated_at,
		       resolved_at, resolved_by, group_key, dedup_key, dedup_window
		FROM system_alerts
		WHERE dedup_key = $1 AND created_at >= $2
		ORDER BY created_at DESC
	`

	rows, err := dbTx.Query(ctx, query, dedupKey, window)
	if err != nil {
		return nil, fmt.Errorf("find by dedup key in window failed: %w", err)
	}
	defer rows.Close()

	var alerts []*entity.Alert
	for rows.Next() {
		alert, err := r.scanRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan alert failed: %w", err)
		}
		alerts = append(alerts, alert)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("find by dedup key in window scan failed: %w", rows.Err())
	}

	return alerts, nil
}

// DeleteOld deletes resolved alerts older than the given duration (in days).
func (r *AlertRepositoryImpl) DeleteOld(
	ctx context.Context,
	tx interface{},
	olderThan int,
) (int, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return 0, fmt.Errorf("invalid transaction type")
	}

	cutoffDate := time.Now().AddDate(0, 0, -olderThan)

	result, err := dbTx.Exec(ctx, `
		DELETE FROM system_alerts
		WHERE status IN ($1, $2)
		AND resolved_at < $3
	`, entity.StatusResolved, entity.StatusFalsePositive, cutoffDate)

	if err != nil {
		return 0, fmt.Errorf("delete old alerts failed: %w", err)
	}

	rowsAffected := result.RowsAffected()
	return int(rowsAffected), nil
}

// buildListQuery builds the SQL query for listing alerts based on filters.
func (r *AlertRepositoryImpl) buildListQuery(filters repository.AlertFilters) (string, []interface{}) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	if len(filters.Statuses) > 0 {
		placeholders := make([]string, len(filters.Statuses))
		for i, s := range filters.Statuses {
			placeholders[i] = fmt.Sprintf("$%d", argIdx)
			args = append(args, string(s))
			argIdx++
		}
		conditions = append(conditions, "status IN ("+strings.Join(placeholders, ", ")+")")
	} else if filters.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, string(*filters.Status))
		argIdx++
	}
	if filters.Severity != nil {
		conditions = append(conditions, fmt.Sprintf("severity = $%d", argIdx))
		args = append(args, string(*filters.Severity))
		argIdx++
	}
	if filters.AlertType != nil {
		conditions = append(conditions, fmt.Sprintf("alert_type = $%d", argIdx))
		args = append(args, string(*filters.AlertType))
		argIdx++
	}
	if filters.EntityType != nil {
		conditions = append(conditions, fmt.Sprintf("entity_type = $%d", argIdx))
		args = append(args, *filters.EntityType)
		argIdx++
	}
	if filters.EntityID != nil {
		conditions = append(conditions, fmt.Sprintf("entity_id = $%d", argIdx))
		args = append(args, *filters.EntityID)
		argIdx++
	}
	if filters.GroupKey != nil {
		conditions = append(conditions, fmt.Sprintf("group_key = $%d", argIdx))
		args = append(args, *filters.GroupKey)
		argIdx++
	}
	if filters.DateFrom != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIdx))
		args = append(args, *filters.DateFrom)
		argIdx++
	}
	if filters.DateTo != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIdx))
		args = append(args, *filters.DateTo)
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + joinConditions(conditions)
	}

	args = append(args, filters.Limit)
	args = append(args, filters.Offset)

	query := fmt.Sprintf(`
		SELECT id, alert_type, severity, entity_type, entity_id,
		       message, metadata_json, status, created_at, updated_at,
		       resolved_at, resolved_by, group_key, dedup_key, dedup_window
		FROM system_alerts
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)

	return query, args
}

// scanRow scans an alert from a database row.
func (r *AlertRepositoryImpl) scanRow(rows pgx.Rows) (*entity.Alert, error) {
	var id, entityID uuid.UUID
	var alertType, severity, entityType, status, dedupKey string
	var message string
	var metadataJSON []byte
	var createdAt, updatedAt time.Time
	var resolvedAt *time.Time
	var resolvedBy *uuid.UUID
	var groupKey *string
	var dedupWindow *int

	err := rows.Scan(
		&id, &alertType, &severity, &entityType, &entityID,
		&message, &metadataJSON, &status, &createdAt, &updatedAt,
		&resolvedAt, &resolvedBy, &groupKey, &dedupKey, &dedupWindow,
	)

	if err != nil {
		return nil, fmt.Errorf("scan row failed: %w", err)
	}

	return &entity.Alert{
		ID:          id,
		AlertType:   entity.AlertType(alertType),
		Severity:    entity.AlertSeverity(severity),
		EntityType:  entityType,
		EntityID:    entityID,
		Message:     message,
		Metadata:    entity.FromJSON(metadataJSON),
		Status:      entity.AlertStatus(status),
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
		ResolvedAt:  resolvedAt,
		ResolvedBy:  resolvedBy,
		GroupKey:    groupKey,
		DedupKey:    dedupKey,
		DedupWindow: dedupWindow,
	}, nil
}

// joinConditions joins SQL conditions with AND.
func joinConditions(conditions []string) string {
	if len(conditions) == 0 {
		return ""
	}
	result := conditions[0]
	for i := 1; i < len(conditions); i++ {
		result += " AND " + conditions[i]
	}
	return result
}


