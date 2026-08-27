package database

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Test DBMetrics tracking
func TestRecordTxStart(t *testing.T) {
	// Reset metrics
	GetMetrics().Reset()

	initialTxTotal := GetMetrics().txTotal.Load()

	RecordTxStart()

	assert.Equal(t, initialTxTotal+1, GetMetrics().txTotal.Load())
}

func TestRecordTxCommit(t *testing.T) {
	GetMetrics().Reset()

	initialTxCommit := GetMetrics().txCommit.Load()

	RecordTxCommit()

	assert.Equal(t, initialTxCommit+1, GetMetrics().txCommit.Load())
}

func TestRecordTxRollback(t *testing.T) {
	GetMetrics().Reset()

	initialTxRollback := GetMetrics().txRollback.Load()

	RecordTxRollback()

	assert.Equal(t, initialTxRollback+1, GetMetrics().txRollback.Load())
}

func TestRecordTxContextError(t *testing.T) {
	GetMetrics().Reset()

	initialTxContextErr := GetMetrics().txContextErr.Load()

	RecordTxContextError()

	assert.Equal(t, initialTxContextErr+1, GetMetrics().txContextErr.Load())
}

// Test error recording
func TestRecordCreateError(t *testing.T) {
	GetMetrics().Reset()

	initialDBErrorTotal := GetMetrics().dbErrorTotal.Load()
	initialDBErrorCreate := GetMetrics().dbErrorCreate.Load()

	RecordCreateError()

	assert.Equal(t, initialDBErrorTotal+1, GetMetrics().dbErrorTotal.Load())
	assert.Equal(t, initialDBErrorCreate+1, GetMetrics().dbErrorCreate.Load())
}

func TestRecordUpdateError(t *testing.T) {
	GetMetrics().Reset()

	initialDBErrorTotal := GetMetrics().dbErrorTotal.Load()
	initialDBErrorUpdate := GetMetrics().dbErrorUpdate.Load()

	RecordUpdateError()

	assert.Equal(t, initialDBErrorTotal+1, GetMetrics().dbErrorTotal.Load())
	assert.Equal(t, initialDBErrorUpdate+1, GetMetrics().dbErrorUpdate.Load())
}

func TestRecordDeleteError(t *testing.T) {
	GetMetrics().Reset()

	initialDBErrorTotal := GetMetrics().dbErrorTotal.Load()
	initialDBErrorDelete := GetMetrics().dbErrorDelete.Load()

	RecordDeleteError()

	assert.Equal(t, initialDBErrorTotal+1, GetMetrics().dbErrorTotal.Load())
	assert.Equal(t, initialDBErrorDelete+1, GetMetrics().dbErrorDelete.Load())
}

func TestRecordSelectError(t *testing.T) {
	GetMetrics().Reset()

	initialDBErrorTotal := GetMetrics().dbErrorTotal.Load()
	initialDBErrorSelect := GetMetrics().dbErrorSelect.Load()

	RecordSelectError()

	assert.Equal(t, initialDBErrorTotal+1, GetMetrics().dbErrorTotal.Load())
	assert.Equal(t, initialDBErrorSelect+1, GetMetrics().dbErrorSelect.Load())
}

func TestRecordLockError(t *testing.T) {
	GetMetrics().Reset()

	initialDBErrorTotal := GetMetrics().dbErrorTotal.Load()
	initialDBErrorLock := GetMetrics().dbErrorLock.Load()

	RecordLockError()

	assert.Equal(t, initialDBErrorTotal+1, GetMetrics().dbErrorTotal.Load())
	assert.Equal(t, initialDBErrorLock+1, GetMetrics().dbErrorLock.Load())
}

func TestRecordSlowQuery(t *testing.T) {
	GetMetrics().Reset()

	initialSlowQueryCount := GetMetrics().slowQueryCount.Load()

	RecordSlowQuery()

	assert.Equal(t, initialSlowQueryCount+1, GetMetrics().slowQueryCount.Load())
}

// Test Snapshot
func TestSnapshot_EmptyMetrics(t *testing.T) {
	GetMetrics().Reset()

	snapshot := GetMetrics().Snapshot()

	assert.Equal(t, int64(0), snapshot.TxTotal)
	assert.Equal(t, int64(0), snapshot.TxCommit)
	assert.Equal(t, int64(0), snapshot.TxRollback)
	assert.Equal(t, int64(0), snapshot.TxContextErr)
	assert.Equal(t, int64(0), snapshot.DBErrorTotal)
	assert.Equal(t, int64(0), snapshot.DBErrorCreate)
	assert.Equal(t, int64(0), snapshot.DBErrorUpdate)
	assert.Equal(t, int64(0), snapshot.DBErrorDelete)
	assert.Equal(t, int64(0), snapshot.DBErrorSelect)
	assert.Equal(t, int64(0), snapshot.DBErrorLock)
	assert.Equal(t, int64(0), snapshot.SlowQueryCount)
	assert.Equal(t, float64(0), snapshot.TxRollbackRate)
	assert.GreaterOrEqual(t, snapshot.UptimeSeconds, int64(0)) // Can be 0 immediately after Reset
	assert.False(t, snapshot.Timestamp.IsZero())
}

func TestSnapshot_WithData(t *testing.T) {
	GetMetrics().Reset()

	// Record some metrics
	RecordTxStart()
	RecordTxStart()
	RecordTxCommit()
	RecordTxRollback()
	RecordCreateError()
	RecordSlowQuery()

	snapshot := GetMetrics().Snapshot()

	assert.Equal(t, int64(2), snapshot.TxTotal)
	assert.Equal(t, int64(1), snapshot.TxCommit)
	assert.Equal(t, int64(1), snapshot.TxRollback)
	assert.Equal(t, int64(1), snapshot.DBErrorCreate)
	assert.Equal(t, int64(1), snapshot.SlowQueryCount)
	assert.Equal(t, float64(0.5), snapshot.TxRollbackRate) // 1 rollback / 2 total
}

func TestSnapshot_RollbackRateCalculation(t *testing.T) {
	GetMetrics().Reset()

	// Test with zero transactions
	snapshot := GetMetrics().Snapshot()
	assert.Equal(t, float64(0), snapshot.TxRollbackRate)

	// Record some transactions
	for i := 0; i < 10; i++ {
		RecordTxStart()
	}
	for i := 0; i < 7; i++ {
		RecordTxCommit()
	}
	for i := 0; i < 3; i++ {
		RecordTxRollback()
	}

	snapshot = GetMetrics().Snapshot()
	assert.Equal(t, float64(0.3), snapshot.TxRollbackRate) // 3/10 = 0.3
}

// Test Reset
func TestReset(t *testing.T) {
	GetMetrics().Reset()

	// Record some data
	RecordTxStart()
	RecordTxCommit()
	RecordCreateError()
	RecordSlowQuery()

	// Reset
	GetMetrics().Reset()

	snapshot := GetMetrics().Snapshot()

	assert.Equal(t, int64(0), snapshot.TxTotal)
	assert.Equal(t, int64(0), snapshot.TxCommit)
	assert.Equal(t, int64(0), snapshot.TxRollback)
	assert.Equal(t, int64(0), snapshot.DBErrorTotal)
	assert.Equal(t, int64(0), snapshot.SlowQueryCount)
}

// Test HealthThresholds
func TestDefaultHealthThresholds(t *testing.T) {
	thresholds := DefaultHealthThresholds()

	assert.Equal(t, float64(0.10), thresholds.MaxTxRollbackRate)
	assert.Equal(t, int64(100), thresholds.MaxDBErrors)
	assert.Equal(t, int64(50), thresholds.MaxSlowQueries)
}

// Test CheckHealth
func TestCheckHealth_AllHealthy(t *testing.T) {
	GetMetrics().Reset()

	thresholds := DefaultHealthThresholds()
	status := GetMetrics().CheckHealth(thresholds)

	assert.Equal(t, "healthy", status.Status)
	assert.True(t, status.TxHealthy)
	assert.True(t, status.DBHealthy)
	assert.Empty(t, status.Message)
}

func TestCheckHealth_HighRollbackRate(t *testing.T) {
	GetMetrics().Reset()

	// Create high rollback rate (>10%)
	for i := 0; i < 100; i++ {
		RecordTxStart()
	}
	for i := 0; i < 85; i++ {
		RecordTxCommit()
	}
	for i := 0; i < 15; i++ {
		RecordTxRollback()
	}

	thresholds := DefaultHealthThresholds()
	status := GetMetrics().CheckHealth(thresholds)

	assert.Equal(t, "degraded", status.Status)
	assert.False(t, status.TxHealthy)
	assert.True(t, status.DBHealthy)
	assert.Contains(t, status.Message, "rollback rate")
}

func TestCheckHealth_ExcessiveDBErrors(t *testing.T) {
	GetMetrics().Reset()

	// Create excessive errors (>100)
	for i := 0; i < 101; i++ {
		RecordCreateError()
	}

	thresholds := DefaultHealthThresholds()
	status := GetMetrics().CheckHealth(thresholds)

	assert.Equal(t, "unhealthy", status.Status)
	assert.True(t, status.TxHealthy)
	assert.False(t, status.DBHealthy)
	assert.Contains(t, status.Message, "database errors")
}

func TestCheckHealth_HighSlowQueries(t *testing.T) {
	GetMetrics().Reset()

	// Create high slow query count (>50)
	for i := 0; i < 51; i++ {
		RecordSlowQuery()
	}

	thresholds := DefaultHealthThresholds()
	status := GetMetrics().CheckHealth(thresholds)

	assert.Equal(t, "degraded", status.Status)
	assert.Contains(t, status.Message, "slow query")
}

func TestCheckHealth_MultipleIssues(t *testing.T) {
	GetMetrics().Reset()

	// Create multiple issues
	for i := 0; i < 100; i++ {
		RecordTxStart()
	}
	for i := 0; i < 80; i++ {
		RecordTxCommit()
	}
	for i := 0; i < 20; i++ {
		RecordTxRollback()
	}
	for i := 0; i < 101; i++ {
		RecordCreateError()
	}

	thresholds := DefaultHealthThresholds()
	status := GetMetrics().CheckHealth(thresholds)

	// Unhealthy takes precedence over degraded
	assert.Equal(t, "unhealthy", status.Status)
	assert.False(t, status.TxHealthy)
	assert.False(t, status.DBHealthy)
}

// Test CheckAlerts
func TestCheckAlerts_NoAlerts(t *testing.T) {
	GetMetrics().Reset()

	alerts := GetMetrics().CheckAlerts()

	assert.Empty(t, alerts)
}

func TestCheckAlerts_HighRollbackRate(t *testing.T) {
	GetMetrics().Reset()

	// Create rollback rate >5% with at least 10 transactions
	for i := 0; i < 20; i++ {
		RecordTxStart()
	}
	for i := 0; i < 17; i++ {
		RecordTxCommit()
	}
	for i := 0; i < 3; i++ {
		RecordTxRollback()
	}

	alerts := GetMetrics().CheckAlerts()

	assert.Len(t, alerts, 1)
	assert.Equal(t, "tx_rollback_rate", alerts[0].Type)
	assert.True(t, alerts[0].Triggered)
	assert.Equal(t, float64(0.05), alerts[0].Threshold)
	assert.Contains(t, alerts[0].Message, "rollback rate")
}

func TestCheckAlerts_ExcessiveDBErrors(t *testing.T) {
	GetMetrics().Reset()

	// Create >50 errors
	for i := 0; i < 51; i++ {
		RecordCreateError()
	}

	alerts := GetMetrics().CheckAlerts()

	assert.Len(t, alerts, 1)
	assert.Equal(t, "db_error_count", alerts[0].Type)
	assert.True(t, alerts[0].Triggered)
	assert.Equal(t, float64(50), alerts[0].Threshold)
}

func TestCheckAlerts_LockErrors(t *testing.T) {
	GetMetrics().Reset()

	// Create >10 lock errors
	for i := 0; i < 11; i++ {
		RecordLockError()
	}

	alerts := GetMetrics().CheckAlerts()

	assert.Len(t, alerts, 1)
	assert.Equal(t, "db_lock_errors", alerts[0].Type)
	assert.True(t, alerts[0].Triggered)
	assert.Equal(t, float64(10), alerts[0].Threshold)
}

func TestCheckAlerts_BelowThreshold(t *testing.T) {
	GetMetrics().Reset()

	// Create rollback rate <5%
	for i := 0; i < 100; i++ {
		RecordTxStart()
	}
	for i := 0; i < 97; i++ {
		RecordTxCommit()
	}
	for i := 0; i < 3; i++ {
		RecordTxRollback()
	}

	alerts := GetMetrics().CheckAlerts()

	// Should not trigger alert (3% < 5%)
	var rollbackAlert *AlertCondition
	for _, alert := range alerts {
		if alert.Type == "tx_rollback_rate" {
			rollbackAlert = &alert
			break
		}
	}
	assert.Nil(t, rollbackAlert)
}

func TestCheckAlerts_MultipleAlerts(t *testing.T) {
	GetMetrics().Reset()

	// Create multiple alert conditions
	for i := 0; i < 20; i++ {
		RecordTxStart()
	}
	for i := 0; i < 17; i++ {
		RecordTxCommit()
	}
	for i := 0; i < 3; i++ {
		RecordTxRollback()
	}
	for i := 0; i < 51; i++ {
		RecordCreateError()
	}
	for i := 0; i < 11; i++ {
		RecordLockError()
	}

	alerts := GetMetrics().CheckAlerts()

	assert.Len(t, alerts, 3)
}

// Test SetSlowQueryThreshold
func TestSetSlowQueryThreshold(t *testing.T) {
	GetMetrics().Reset()

	newThreshold := 5 * time.Second
	SetSlowQueryThreshold(newThreshold)

	assert.Equal(t, newThreshold, GetMetrics().slowQueryThreshold)
}

// Test concurrent access
func TestConcurrentMetricRecording(t *testing.T) {
	GetMetrics().Reset()

	var wg sync.WaitGroup
	numGoroutines := 100
	recordsPerGoroutine := 100

	// Concurrently record metrics
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < recordsPerGoroutine; j++ {
				RecordTxStart()
				RecordTxCommit()
				RecordCreateError()
				RecordSlowQuery()
			}
		}()
	}

	wg.Wait()

	snapshot := GetMetrics().Snapshot()

	expectedCount := int64(numGoroutines * recordsPerGoroutine)
	assert.Equal(t, expectedCount, snapshot.TxTotal)
	assert.Equal(t, expectedCount, snapshot.TxCommit)
	assert.Equal(t, expectedCount, snapshot.DBErrorCreate)
	assert.Equal(t, expectedCount, snapshot.SlowQueryCount)
}
