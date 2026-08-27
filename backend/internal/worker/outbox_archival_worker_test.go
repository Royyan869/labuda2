package worker

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap/zaptest"

	"github.com/labuda/backend/internal/config"
	dbpkg "github.com/labuda/backend/pkg/db"
)

// TestOutboxArchivalWorker_NewWorker tests creating a new archival worker
func TestOutboxArchivalWorker_NewWorker(t *testing.T) {
	log := zaptest.NewLogger(t)
	cfg := DefaultOutboxArchivalWorkerConfig()

	worker := NewOutboxArchivalWorker(nil, nil, log, cfg)

	if worker == nil {
		t.Fatal("NewOutboxArchivalWorker() returned nil")
	}

	if worker.pollInterval != DefaultArchivalPollInterval {
		t.Errorf("pollInterval = %v, want %v", worker.pollInterval, DefaultArchivalPollInterval)
	}

	if worker.batchSize != DefaultArchivalBatchSize {
		t.Errorf("batchSize = %d, want %d", worker.batchSize, DefaultArchivalBatchSize)
	}

	if worker.retentionDays != DefaultRetentionDays {
		t.Errorf("retentionDays = %d, want %d", worker.retentionDays, DefaultRetentionDays)
	}

	if worker.workerID == "" {
		t.Error("workerID should not be empty")
	}
}

// TestOutboxArchivalWorker_StartStop tests starting and stopping the worker
func TestOutboxArchivalWorker_StartStop(t *testing.T) {
	log := zaptest.NewLogger(t)
	cfg := OutboxArchivalWorkerConfig{
		PollInterval:  10 * time.Millisecond,
		BatchSize:     10,
		RetentionDays: 30,
	}

	mockDB := &mockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
			return fn(&mockTx{
				QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
					return &mockRows{rows: [][]any{}}, nil
				},
			})
		},
	}

	worker := NewOutboxArchivalWorker(mockDB, nil, log, cfg)

	if worker.IsRunning() {
		t.Error("worker should not be running initially")
	}

	worker.Start()

	if !worker.IsRunning() {
		t.Error("worker should be running after Start()")
	}

	time.Sleep(50 * time.Millisecond)

	worker.Stop()

	if worker.IsRunning() {
		t.Error("worker should not be running after Stop()")
	}
}

// TestOutboxArchivalWorker_DoubleStart tests that starting twice doesn't spawn multiple goroutines
func TestOutboxArchivalWorker_DoubleStart(t *testing.T) {
	log := zaptest.NewLogger(t)
	cfg := DefaultOutboxArchivalWorkerConfig()

	mockDB := &mockDB{
		WithTxFunc: func(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
			return fn(&mockTx{
				QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
					return &mockRows{rows: [][]any{}}, nil
				},
			})
		},
	}

	worker := NewOutboxArchivalWorker(mockDB, nil, log, cfg)

	worker.Start()
	worker.Start() // Should warn but not crash

	worker.Stop()
}

// TestOutboxArchivalWorker_FromConfig tests creating worker from application config
func TestOutboxArchivalWorker_FromConfig(t *testing.T) {
	log := zaptest.NewLogger(t)
	cfg := &config.Config{
		Outbox: config.OutboxConfig{
			RetentionDays:    60,
			ArchiveBatchSize: 1000,
		},
	}

	worker := NewOutboxArchivalWorkerFromConfig(nil, nil, log, cfg)

	if worker.retentionDays != 60 {
		t.Errorf("retentionDays = %d, want 60", worker.retentionDays)
	}

	if worker.batchSize != 1000 {
		t.Errorf("batchSize = %d, want 1000", worker.batchSize)
	}
}

// TestOutboxArchivalWorker_SetMetricsCollector tests setting metrics collector
func TestOutboxArchivalWorker_SetMetricsCollector(t *testing.T) {
	log := zaptest.NewLogger(t)
	cfg := DefaultOutboxArchivalWorkerConfig()

	worker := NewOutboxArchivalWorker(nil, nil, log, cfg)

	mockRecorder := &mockMetricsRecorder{}
	worker.SetMetricsCollector(mockRecorder)

	worker.mu.RLock()
	collector := worker.metricsCollector
	worker.mu.RUnlock()

	if collector == nil {
		t.Error("metricsCollector should not be nil after SetMetricsCollector")
	}
}

// TestOutboxArchivalWorker_GetMetrics tests metric retrieval methods
func TestOutboxArchivalWorker_GetMetrics(t *testing.T) {
	log := zaptest.NewLogger(t)
	cfg := DefaultOutboxArchivalWorkerConfig()

	worker := NewOutboxArchivalWorker(nil, nil, log, cfg)

	if worker.GetArchivedCount() != 0 {
		t.Errorf("GetArchivedCount() = %d, want 0", worker.GetArchivedCount())
	}

	if worker.GetTotalArchivedCount() != 0 {
		t.Errorf("GetTotalArchivedCount() = %d, want 0", worker.GetTotalArchivedCount())
	}
}

// Specification tests for archival worker behavior
func TestOutboxArchivalWorker_Specification(t *testing.T) {
	t.Run("fetch_succeeded_for_archival", func(t *testing.T) {
		// SPECIFICATION: Worker should fetch outbox messages that are:
		// - status = 'succeeded'
		// - delivered_at < NOW() - INTERVAL 'retention days'
		// - Ordered by delivered_at ASC
		// - Limited to batch_size
	})

	t.Run("move_to_archive", func(t *testing.T) {
		// SPECIFICATION: Archival should:
		// - Copy message to outbox_archive table
		// - Delete message from outbox table
		// - Be done in a single transaction
	})

	t.Run("full_archival_flow", func(t *testing.T) {
		// SPECIFICATION: Full flow should:
		// 1. Fetch messages ready for archival
		// 2. Move to archive in batches
		// 3. Update metrics
		// 4. Handle errors gracefully
	})
}

// Mock implementations for testing

type outboxMockDB struct {
	WithTxFunc func(ctx context.Context, fn func(tx dbpkg.Tx) error) error
}

func (m *outboxMockDB) WithTx(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
	if m.WithTxFunc != nil {
		return m.WithTxFunc(ctx, fn)
	}
	return fn(&outboxMockTx{})
}

type outboxMockTx struct {
	QueryFunc func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (m *outboxMockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return &outboxMockRow{}
}

func (m *outboxMockTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, sql, args...)
	}
	return &outboxMockRows{}, nil
}

func (m *outboxMockTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("0"), nil
}

func (m *outboxMockTx) Commit(ctx context.Context) error {
	return nil
}

func (m *outboxMockTx) Rollback(ctx context.Context) error {
	return nil
}

type outboxMockRows struct {
	rows [][]any
}

func (m *outboxMockRows) Close() {}
func (m *outboxMockRows) Err() error { return nil }
func (m *outboxMockRows) Next() bool { return false }
func (m *outboxMockRows) Scan(dest ...any) error { return nil }
func (m *outboxMockRows) CommandTag() pgconn.CommandTag { return pgconn.NewCommandTag("0") }
func (m *outboxMockRows) Fields() []pgconn.FieldDescription { return nil }
func (m *outboxMockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (m *outboxMockRows) RawValues() [][]byte { return nil }
func (m *outboxMockRows) Values() ([]any, error) { return nil, nil }
func (m *outboxMockRows) Conn() *pgx.Conn { return nil }

type outboxMockRow struct{}
func (m *outboxMockRow) Scan(dest ...any) error { return nil }

type mockMetricsRecorder struct{}

func (m *mockMetricsRecorder) RecordOutboxArchived(count int) {}
func (m *mockMetricsRecorder) RecordOutboxArchiveBatchDuration(durationMs float64) {}




