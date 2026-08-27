package worker

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	alertapp "github.com/labuda/backend/internal/platform/alert/application"
	alertentity "github.com/labuda/backend/internal/platform/alert/entity"
	"github.com/labuda/backend/internal/finance/entity"
	"github.com/labuda/backend/internal/finance/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

// mockTx implements db.Tx for testing
type mockTx struct {
	queryRowCalls int
	queryCalls    int
	execCalls     int
	QueryRowFunc  func(ctx context.Context, sql string, args ...any) pgx.Row
	QueryFunc     func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	ExecFunc      func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	CommitFunc    func(ctx context.Context) error
	RollbackFunc  func(ctx context.Context) error
}

func (m *mockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	m.queryRowCalls++
	if m.QueryRowFunc != nil {
		return m.QueryRowFunc(ctx, sql, args...)
	}
	return &mockRow{err: errors.New("no mock configured")}
}

func (m *mockTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	m.queryCalls++
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, sql, args...)
	}
	return &mockRows{}, nil
}

func (m *mockTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	m.execCalls++
	if m.ExecFunc != nil {
		return m.ExecFunc(ctx, sql, args...)
	}
	return pgconn.NewCommandTag("1"), nil
}

func (m *mockTx) Commit(ctx context.Context) error {
	if m.CommitFunc != nil {
		return m.CommitFunc(ctx)
	}
	return nil
}

func (m *mockTx) Rollback(ctx context.Context) error {
	if m.RollbackFunc != nil {
		return m.RollbackFunc(ctx)
	}
	return nil
}

// mockDB implements the Transactor interface for testing
type mockDB struct {
	WithTxFunc func(ctx context.Context, fn func(tx db.Tx) error) error
}

func (m *mockDB) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	if m.WithTxFunc != nil {
		return m.WithTxFunc(ctx, fn)
	}
	return fn(&mockTx{})
}

// mockRow implements pgx.Row for testing
type mockRow struct {
	values []any
	err    error
}

func (r *mockRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(r.values) != len(dest) {
		return fmt.Errorf("scan argument count mismatch: have %d, want %d", len(r.values), len(dest))
	}
	for i, v := range r.values {
		switch d := dest[i].(type) {
		case *uuid.UUID:
			if id, ok := v.(uuid.UUID); ok {
				*d = id
			}
		case *string:
			if s, ok := v.(string); ok {
				*d = s
			}
		case *int:
			if i, ok := v.(int); ok {
				*d = i
			}
		case *int64:
			if i, ok := v.(int64); ok {
				*d = i
			}
		case *money.Money:
			if m, ok := v.(money.Money); ok {
				*d = m
			}
		case *time.Time:
			if t, ok := v.(time.Time); ok {
				*d = t
			}
		case *float64:
			if f, ok := v.(float64); ok {
				*d = f
			}
		case *bool:
			if b, ok := v.(bool); ok {
				*d = b
			}
		default:
			return fmt.Errorf("unsupported type %T for mock scan", dest[i])
		}
	}
	return nil
}

// mockRows implements pgx.Rows for testing
type mockRows struct {
	rows    [][]any
	current int
}

func (r *mockRows) Next() bool {
	r.current++
	return r.current <= len(r.rows)
}

func (r *mockRows) Err() error {
	return nil
}

func (r *mockRows) Close() {}

func (r *mockRows) Scan(dest ...any) error {
	if r.current > len(r.rows) {
		return errors.New("no more rows")
	}
	if len(r.rows) == 0 {
		return errors.New("no rows")
	}
	row := r.rows[r.current-1]
	if len(row) != len(dest) {
		return fmt.Errorf("scan argument count mismatch: have %d, want %d", len(row), len(dest))
	}
	for i, v := range row {
		switch d := dest[i].(type) {
		case *uuid.UUID:
			if id, ok := v.(uuid.UUID); ok {
				*d = id
			}
		case *string:
			if s, ok := v.(string); ok {
				*d = s
			}
		case *int:
			if i, ok := v.(int); ok {
				*d = i
			}
		case *int64:
			if i, ok := v.(int64); ok {
				*d = i
			}
		case *money.Money:
			if m, ok := v.(money.Money); ok {
				*d = m
			}
		case *time.Time:
			if t, ok := v.(time.Time); ok {
				*d = t
			}
		case *float64:
			if f, ok := v.(float64); ok {
				*d = f
			}
		case *bool:
			if b, ok := v.(bool); ok {
				*d = b
			}
		default:
			return fmt.Errorf("unsupported type %T for mock scan", dest[i])
		}
	}
	return nil
}

func (r *mockRows) CommandTag() pgconn.CommandTag {
	return pgconn.NewCommandTag(fmt.Sprintf("%d", len(r.rows)))
}

func (r *mockRows) Fields() []pgconn.FieldDescription {
	return nil
}

func (r *mockRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (r *mockRows) RawValues() [][]byte {
	return nil
}

func (r *mockRows) Values() ([]any, error) {
	if r.current > len(r.rows) || r.current < 1 {
		return nil, errors.New("no current row")
	}
	return r.rows[r.current-1], nil
}

func (r *mockRows) Conn() *pgx.Conn {
	return nil
}

// MockReconciliationRepository implements repository.ReconciliationRepository for testing
type MockReconciliationRepository struct {
	results []*entity.ReconciliationResult
	mu      sync.Mutex
}

func NewMockReconciliationRepository() *MockReconciliationRepository {
	return &MockReconciliationRepository{
		results: make([]*entity.ReconciliationResult, 0),
	}
}

func (m *MockReconciliationRepository) Create(ctx context.Context, tx interface{}, result *entity.ReconciliationResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.results = append(m.results, result)
	return nil
}

func (m *MockReconciliationRepository) GetByID(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.ReconciliationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, result := range m.results {
		if result.ID == id {
			return result, nil
		}
	}

	return nil, sql.ErrNoRows
}

func (m *MockReconciliationRepository) List(ctx context.Context, tx interface{}, filters repository.ReconciliationFilters) ([]*entity.ReconciliationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Simple filtering implementation
	start := filters.Offset
	end := len(m.results)

	if filters.Limit > 0 {
		end = start + filters.Limit
		if end > len(m.results) {
			end = len(m.results)
		}
	}

	if start >= len(m.results) {
		return []*entity.ReconciliationResult{}, nil
	}

	return m.results[start:end], nil
}

func (m *MockReconciliationRepository) Count(ctx context.Context, tx interface{}, filters repository.ReconciliationFilters) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return int64(len(m.results)), nil
}

func (m *MockReconciliationRepository) GetLatest(ctx context.Context, tx interface{}) (*entity.ReconciliationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.results) == 0 {
		return nil, sql.ErrNoRows
	}

	return m.results[len(m.results)-1], nil
}

func (m *MockReconciliationRepository) GetResults() []*entity.ReconciliationResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.results
}

func (m *MockReconciliationRepository) GetLatestBySeverity(ctx context.Context, tx interface{}, severity entity.ReconcileSeverity) (*entity.ReconciliationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Return most recent result with matching severity
	for i := len(m.results) - 1; i >= 0; i-- {
		if m.results[i].Severity == severity {
			return m.results[i], nil
		}
	}

	return nil, sql.ErrNoRows
}

func (m *MockReconciliationRepository) DeleteOld(ctx context.Context, tx interface{}, olderThan time.Duration) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)
	initialCount := len(m.results)

	filtered := make([]*entity.ReconciliationResult, 0, len(m.results))
	for _, result := range m.results {
		if result.CheckedAt.After(cutoff) {
			filtered = append(filtered, result)
		}
	}

	m.results = filtered
	deleted := initialCount - len(filtered)

	return deleted, nil
}

// MockAlertService implements alert service for testing
type MockAlertService struct {
	mu              sync.Mutex
	alerts          []alertentity.Alert
	dedupWindow     int
	shouldFail      bool
	escalationCount int
}

func NewMockAlertService() *MockAlertService {
	return &MockAlertService{
		alerts:      make([]alertentity.Alert, 0),
		dedupWindow: 60,
		shouldFail:  false,
	}
}

func (m *MockAlertService) CreateAlert(ctx context.Context, alertType alertentity.AlertType, severity alertentity.AlertSeverity, entityType string, entityID uuid.UUID, message string, metadata alertentity.AlertMetadata, groupKey *string) (*alertapp.CreateAlertResult, error) {
	if m.shouldFail {
		return nil, fmt.Errorf("alert service failure")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	dedupKey := fmt.Sprintf("%s:%s:%s", alertType, entityType, entityID.String())

	// Check for duplicate alerts within dedup window
	for i, existing := range m.alerts {
		if existing.DedupKey == dedupKey {
			if time.Since(existing.CreatedAt) < time.Duration(m.dedupWindow)*time.Minute {
				// Update existing alert
				m.alerts[i].UpdatedAt = time.Now()
				if count, ok := existing.Metadata["occurrence_count"].(int); ok {
					m.alerts[i].Metadata["occurrence_count"] = count + 1
				} else {
					m.alerts[i].Metadata["occurrence_count"] = 1
				}

				return &alertapp.CreateAlertResult{
					Created:    false,
					Reason:     "Updated existing alert (dedup)",
					ExistingID: &existing.ID,
					Alert:      &m.alerts[i],
				}, nil
			}
		}
	}

	// Create new alert
	alert := alertentity.Alert{
		ID:          uuid.New(),
		AlertType:   alertType,
		Severity:    severity,
		EntityType:  entityType,
		EntityID:    entityID,
		Message:     message,
		Status:      alertentity.StatusOpen,
		DedupKey:    dedupKey,
		DedupWindow: &m.dedupWindow,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Metadata:    metadata,
	}

	if metadata["occurrence_count"] == nil {
		metadata["occurrence_count"] = 1
	}

	m.alerts = append(m.alerts, alert)

	return &alertapp.CreateAlertResult{
		Created: true,
		Reason:  "New alert created",
		Alert:   &m.alerts[len(m.alerts)-1],
	}, nil
}

func (m *MockAlertService) CreateAlertWithDedupWindow(ctx context.Context, alertType alertentity.AlertType, severity alertentity.AlertSeverity, entityType string, entityID uuid.UUID, message string, metadata alertentity.AlertMetadata, groupKey *string, dedupWindow int) (*alertapp.CreateAlertResult, error) {
	m.dedupWindow = dedupWindow
	return m.CreateAlert(ctx, alertType, severity, entityType, entityID, message, metadata, groupKey)
}

func (m *MockAlertService) CreateAlertWithEscalation(ctx context.Context, alertType alertentity.AlertType, severity alertentity.AlertSeverity, entityType string, entityID uuid.UUID, message string, metadata alertentity.AlertMetadata, groupKey *string) (*alertapp.CreateAlertResult, *alertapp.EscalationAction, error) {
	result, err := m.CreateAlert(ctx, alertType, severity, entityType, entityID, message, metadata, groupKey)
	if err != nil {
		return nil, nil, err
	}

	action := &alertapp.EscalationAction{}
	if severity == alertentity.SeverityCritical {
		action.ShouldEscalate = true
		action.Priority = 100
		action.Channel = "immediate"
		action.Reason = "Critical severity"
		m.escalationCount++
	}

	return result, action, nil
}

func (m *MockAlertService) GetEscalationStats(ctx context.Context, windowStart time.Time, windowEnd time.Time) (*alertapp.EscalationStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	stats := &alertapp.EscalationStats{
		TotalAlerts:     len(m.alerts),
		CriticalAlerts:  0,
		WarningAlerts:   0,
		InfoAlerts:      0,
		EscalatedAlerts: m.escalationCount,
		WindowStart:     windowStart,
		WindowEnd:       windowEnd,
	}

	for _, alert := range m.alerts {
		switch alert.Severity {
		case alertentity.SeverityCritical:
			stats.CriticalAlerts++
		case alertentity.SeverityWarning:
			stats.WarningAlerts++
		case alertentity.SeverityInfo:
			stats.InfoAlerts++
		}
	}

	return stats, nil
}

func (m *MockAlertService) GetAlertsCreated() []alertentity.Alert {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.alerts
}

func (m *MockAlertService) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alerts = make([]alertentity.Alert, 0)
	m.escalationCount = 0
}

func (m *MockAlertService) SetFailure(shouldFail bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldFail = shouldFail
}

func (m *MockAlertService) CreateReconciliationDriftAlert(ctx context.Context, severity alertentity.AlertSeverity, mismatchedAccounts int, totalAccounts int, details alertentity.AlertMetadata) (*alertapp.CreateAlertResult, error) {
	entityID := uuid.New()
	message := fmt.Sprintf("Reconciliation drift: %d/%d accounts mismatched", mismatchedAccounts, totalAccounts)
	return m.CreateAlert(ctx, alertentity.AlertType("reconciliation_drift"), severity, "reconciliation", entityID, message, details, nil)
}



