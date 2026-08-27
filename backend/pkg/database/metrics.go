package database

import (
	"sync/atomic"
	"time"
)

// DBMetrics tracks database operation metrics for monitoring
type DBMetrics struct {
	// Transaction metrics
	txTotal      atomic.Int64
	txCommit     atomic.Int64
	txRollback   atomic.Int64
	txContextErr atomic.Int64

	// Error metrics
	dbErrorTotal   atomic.Int64
	dbErrorCreate  atomic.Int64
	dbErrorUpdate  atomic.Int64
	dbErrorDelete  atomic.Int64
	dbErrorSelect  atomic.Int64
	dbErrorLock    atomic.Int64

	// Slow query tracking
	slowQueryThreshold time.Duration
	slowQueryCount     atomic.Int64

	startTime time.Time
}

// Global metrics instance
var globalMetrics = &DBMetrics{
	startTime:           time.Now(),
	slowQueryThreshold:  2 * time.Second,
}

// GetMetrics returns the global metrics instance
func GetMetrics() *DBMetrics {
	return globalMetrics
}

// SetSlowQueryThreshold sets the threshold for slow query tracking
func SetSlowQueryThreshold(d time.Duration) {
	globalMetrics.slowQueryThreshold = d
}

// Transaction tracking methods

// RecordTxStart records a transaction start
func RecordTxStart() {
	globalMetrics.txTotal.Add(1)
}

// RecordTxCommit records a transaction commit
func RecordTxCommit() {
	globalMetrics.txCommit.Add(1)
}

// RecordTxRollback records a transaction rollback
func RecordTxRollback() {
	globalMetrics.txRollback.Add(1)
}

// RecordTxContextError records a context cancellation error
func RecordTxContextError() {
	globalMetrics.txContextErr.Add(1)
}

// Error tracking methods

// RecordDBError records a generic database error
func RecordDBError() {
	globalMetrics.dbErrorTotal.Add(1)
}

// RecordCreateError records a create operation error
func RecordCreateError() {
	globalMetrics.dbErrorTotal.Add(1)
	globalMetrics.dbErrorCreate.Add(1)
}

// RecordUpdateError records an update operation error
func RecordUpdateError() {
	globalMetrics.dbErrorTotal.Add(1)
	globalMetrics.dbErrorUpdate.Add(1)
}

// RecordDeleteError records a delete operation error
func RecordDeleteError() {
	globalMetrics.dbErrorTotal.Add(1)
	globalMetrics.dbErrorDelete.Add(1)
}

// RecordSelectError records a select operation error
func RecordSelectError() {
	globalMetrics.dbErrorTotal.Add(1)
	globalMetrics.dbErrorSelect.Add(1)
}

// RecordLockError records a row locking error
func RecordLockError() {
	globalMetrics.dbErrorTotal.Add(1)
	globalMetrics.dbErrorLock.Add(1)
}

// RecordSlowQuery records a slow query
func RecordSlowQuery() {
	globalMetrics.slowQueryCount.Add(1)
}

// MetricsSnapshot represents a point-in-time snapshot of metrics
type MetricsSnapshot struct {
	Timestamp time.Time `json:"timestamp"`

	// Transaction metrics
	TxTotal      int64 `json:"tx_total"`
	TxCommit     int64 `json:"tx_commit"`
	TxRollback   int64 `json:"tx_rollback"`
	TxContextErr int64 `json:"tx_context_error"`
	TxRollbackRate float64 `json:"tx_rollback_rate"`

	// Error metrics
	DBErrorTotal  int64 `json:"db_error_total"`
	DBErrorCreate int64 `json:"db_error_create"`
	DBErrorUpdate int64 `json:"db_error_update"`
	DBErrorDelete int64 `json:"db_error_delete"`
	DBErrorSelect int64 `json:"db_error_select"`
	DBErrorLock   int64 `json:"db_error_lock"`

	// Slow query metrics
	SlowQueryCount int64 `json:"slow_query_count"`

	// Uptime
	UptimeSeconds int64 `json:"uptime_seconds"`
}

// Snapshot returns a point-in-time snapshot of all metrics
func (m *DBMetrics) Snapshot() MetricsSnapshot {
	txTotal := m.txTotal.Load()
	txCommit := m.txCommit.Load()
	txRollback := m.txRollback.Load()

	rollbackRate := 0.0
	if txTotal > 0 {
		rollbackRate = float64(txRollback) / float64(txTotal)
	}

	return MetricsSnapshot{
		Timestamp:     time.Now(),
		TxTotal:       txTotal,
		TxCommit:      txCommit,
		TxRollback:    txRollback,
		TxContextErr:  m.txContextErr.Load(),
		TxRollbackRate: rollbackRate,
		DBErrorTotal:  m.dbErrorTotal.Load(),
		DBErrorCreate: m.dbErrorCreate.Load(),
		DBErrorUpdate: m.dbErrorUpdate.Load(),
		DBErrorDelete: m.dbErrorDelete.Load(),
		DBErrorSelect: m.dbErrorSelect.Load(),
		DBErrorLock:   m.dbErrorLock.Load(),
		SlowQueryCount: m.slowQueryCount.Load(),
		UptimeSeconds: int64(time.Since(m.startTime).Seconds()),
	}
}

// Reset resets all metrics to zero (useful for testing)
func (m *DBMetrics) Reset() {
	m.txTotal.Store(0)
	m.txCommit.Store(0)
	m.txRollback.Store(0)
	m.txContextErr.Store(0)
	m.dbErrorTotal.Store(0)
	m.dbErrorCreate.Store(0)
	m.dbErrorUpdate.Store(0)
	m.dbErrorDelete.Store(0)
	m.dbErrorSelect.Store(0)
	m.dbErrorLock.Store(0)
	m.slowQueryCount.Store(0)
	m.startTime = time.Now()
}

// HealthStatus returns the health status based on metrics
type HealthStatus struct {
	Status    string  `json:"status"`
	TxHealthy bool    `json:"tx_healthy"`
	DBHealthy bool    `json:"db_healthy"`
	Message   string  `json:"message,omitempty"`
}

// CheckHealth evaluates metrics and returns health status
func (m *DBMetrics) CheckHealth(thresholds HealthThresholds) HealthStatus {
	snapshot := m.Snapshot()

	status := "healthy"
	messages := []string{}

	// Check transaction rollback rate
	txHealthy := true
	if snapshot.TxRollbackRate > thresholds.MaxTxRollbackRate {
		status = "degraded"
		txHealthy = false
		messages = append(messages, "High transaction rollback rate")
	}

	// Check database error rate
	dbHealthy := true
	if snapshot.DBErrorTotal > thresholds.MaxDBErrors {
		status = "unhealthy"
		dbHealthy = false
		messages = append(messages, "Excessive database errors")
	}

	// Check slow queries
	if snapshot.SlowQueryCount > thresholds.MaxSlowQueries {
		status = "degraded"
		messages = append(messages, "High slow query count")
	}

	msg := ""
	if len(messages) > 0 {
		msg = messages[0]
	}

	return HealthStatus{
		Status:    status,
		TxHealthy: txHealthy,
		DBHealthy: dbHealthy,
		Message:   msg,
	}
}

// HealthThresholds defines thresholds for health checks
type HealthThresholds struct {
	MaxTxRollbackRate float64 // Maximum acceptable rollback rate (0.0 - 1.0)
	MaxDBErrors       int64   // Maximum acceptable database errors
	MaxSlowQueries    int64   // Maximum acceptable slow queries
}

// DefaultHealthThresholds returns sensible default thresholds
func DefaultHealthThresholds() HealthThresholds {
	return HealthThresholds{
		MaxTxRollbackRate: 0.10, // 10% rollback rate
		MaxDBErrors:       100,  // 100 errors total
		MaxSlowQueries:    50,   // 50 slow queries
	}
}

// AlertCondition represents a condition that should trigger an alert
type AlertCondition struct {
	Type        string  `json:"type"`         // "tx_rollback", "db_error", "slow_query"
	Triggered   bool    `json:"triggered"`
	Value       float64 `json:"value"`
	Threshold   float64 `json:"threshold"`
	Message     string  `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
}

// CheckAlerts checks all alert conditions and returns triggered alerts
func (m *DBMetrics) CheckAlerts() []AlertCondition {
	snapshot := m.Snapshot()
	alerts := []AlertCondition{}

	// Check for high rollback rate
	rollbackRate := snapshot.TxRollbackRate
	if rollbackRate > 0.05 && snapshot.TxTotal > 10 { // 5% rate with at least 10 transactions
		alerts = append(alerts, AlertCondition{
			Type:        "tx_rollback_rate",
			Triggered:   true,
			Value:       rollbackRate,
			Threshold:   0.05,
			Message:     "Transaction rollback rate exceeds 5%",
			Timestamp:   time.Now(),
		})
	}

	// Check for excessive database errors
	if snapshot.DBErrorTotal > 50 {
		alerts = append(alerts, AlertCondition{
			Type:        "db_error_count",
			Triggered:   true,
			Value:       float64(snapshot.DBErrorTotal),
			Threshold:   50,
			Message:     "Database error count exceeds threshold",
			Timestamp:   time.Now(),
		})
	}

	// Check for lock errors
	if snapshot.DBErrorLock > 10 {
		alerts = append(alerts, AlertCondition{
			Type:        "db_lock_errors",
			Triggered:   true,
			Value:       float64(snapshot.DBErrorLock),
			Threshold:   10,
			Message:     "Database lock errors exceed threshold",
			Timestamp:   time.Now(),
		})
	}

	return alerts
}
