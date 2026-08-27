package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// mockTx is a mock transaction for testing.
type mockTx struct {
	execFunc    func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	queryFunc   func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	queryRowFunc func(ctx context.Context, sql string, args ...any) pgx.Row
	commitFunc   func(ctx context.Context) error
	rollbackFunc func(ctx context.Context) error

	execCalled    int
	queryCalled   int
	queryRowCalled int
	commitCalled  int
	rollbackCalled int
}

func (m *mockTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	m.execCalled++
	if m.execFunc != nil {
		return m.execFunc(ctx, sql, args...)
	}
	return pgconn.NewCommandTag("SELECT 1"), nil
}

func (m *mockTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	m.queryCalled++
	if m.queryFunc != nil {
		return m.queryFunc(ctx, sql, args...)
	}
	return &mockRows{}, nil
}

func (m *mockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	m.queryRowCalled++
	if m.queryRowFunc != nil {
		return m.queryRowFunc(ctx, sql, args...)
	}
	return &mockRow{}
}

func (m *mockTx) Commit(ctx context.Context) error {
	m.commitCalled++
	if m.commitFunc != nil {
		return m.commitFunc(ctx)
	}
	return nil
}

func (m *mockTx) Rollback(ctx context.Context) error {
	m.rollbackCalled++
	if m.rollbackFunc != nil {
		return m.rollbackFunc(ctx)
	}
	return nil
}

// mockRows is a mock implementation of pgx.Rows.
type mockRows struct {
	values   [][]any
	closeErr error
	err      error
	index    int
}

func (m *mockRows) Close() {
	m.index = 0
}

func (m *mockRows) Err() error {
	return m.err
}

func (m *mockRows) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}

func (m *mockRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (m *mockRows) Next() bool {
	if m.values == nil {
		return false
	}
	m.index++
	return m.index <= len(m.values)
}

func (m *mockRows) Scan(dest ...any) error {
	if m.values == nil || m.index > len(m.values) || m.index < 1 {
		return pgx.ErrNoRows
	}
	row := m.values[m.index-1]
	for i, val := range row {
		if i < len(dest) {
			if b, ok := dest[i].(*[]byte); ok {
				if bytes, ok := val.([]byte); ok {
					*b = bytes
				}
			} else if ptr, ok := dest[i].(*uuid.UUID); ok {
				if u, ok := val.(uuid.UUID); ok {
					*ptr = u
				}
			} else if ptr, ok := dest[i].(*string); ok {
				if s, ok := val.(string); ok {
					*ptr = s
				}
			} else if ptr, ok := dest[i].(*time.Time); ok {
				if t, ok := val.(time.Time); ok {
					*ptr = t
				}
			} else if ptr, ok := dest[i].(*int); ok {
				if i, ok := val.(int); ok {
					*ptr = i
				}
			}
		}
	}
	return nil
}

func (m *mockRows) Values() ([]any, error) {
	return nil, nil
}

func (m *mockRows) RawValues() [][]byte {
	return nil
}

func (m *mockRows) Conn() *pgx.Conn {
	return nil
}

// mockRow is a mock implementation of pgx.Row.
type mockRow struct {
	scanFunc func(dest ...any) error
	scanErr  error
}

func (m *mockRow) Scan(dest ...any) error {
	if m.scanFunc != nil {
		return m.scanFunc(dest...)
	}
	if m.scanErr != nil {
		return m.scanErr
	}
	return nil
}

// mockPgError is a mock implementation of pgconn.PgError for testing.
// It implements all methods of pgconn.PgError to work with errors.As.
type mockPgError struct {
	code       string
	message    string
	constraint string
}

func (e *mockPgError) Code() string {
	return e.code
}

func (e *mockPgError) Message() string {
	return e.message
}

func (e *mockPgError) Constraint() string {
	return e.constraint
}

func (e *mockPgError) Column() string {
	return ""
}

func (e *mockPgError) DataType() string {
	return ""
}

func (e *mockPgError) Table() string {
	return ""
}

func (e *mockPgError) Schema() string {
	return ""
}

func (e *mockPgError) Severity() string {
	return ""
}

func (e *mockPgError) SQLState() string {
	return e.code
}

func (e *mockPgError) Detail() string {
	return ""
}

func (e *mockPgError) Hint() string {
	return ""
}

func (e *mockPgError) InternalPosition() string {
	return ""
}

func (e *mockPgError) InternalQuery() string {
	return ""
}

func (e *mockPgError) Where() string {
	return ""
}

func (e *mockPgError) SchemaName() string {
	return ""
}

func (e *mockPgError) TableName() string {
	return ""
}

func (e *mockPgError) ColumnName() string {
	return ""
}

func (e *mockPgError) DataTypeName() string {
	return ""
}

func (e *mockPgError) Position() int32 {
	return 0
}

func (e *mockPgError) InternalPositionInt() int32 {
	return 0
}

func (e *mockPgError) Error() string {
	return e.message
}

// Verify mockTx implements the interface
var _ interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Commit(context.Context) error
	Rollback(context.Context) error
} = (*mockTx)(nil)

// TestNewOutboxRepository tests the constructor.
func TestNewOutboxRepository(t *testing.T) {
	t.Run("creates repository", func(t *testing.T) {
		repo := NewOutboxRepository(nil)
		if repo == nil {
			t.Fatal("expected non-nil repository")
		}
		if repo.db != nil {
			t.Error("expected db to be nil")
		}
	})
}

// TestInsertEvent tests the InsertEvent method.
func TestInsertEvent(t *testing.T) {
	ctx := context.Background()
	aggregateID := uuid.New()
	payload := []byte(`{"test": "data"}`)

	t.Run("insert success", func(t *testing.T) {
		tx := &mockTx{
			execFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("INSERT 0 1"), nil
			},
		}
		repo := NewOutboxRepository(nil)

		err := repo.InsertEvent(ctx, tx, "payment.completed", aggregateID, payload)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if tx.execCalled != 1 {
			t.Errorf("expected Exec to be called once, got %d", tx.execCalled)
		}
	})

	t.Run("idempotent on unique violation", func(t *testing.T) {
		// Create a real pgconn.PgError with the unique constraint violation code
		pgErr := &pgconn.PgError{
			Code:    "23505",
			Message: "duplicate key value violates unique constraint",
		}
		tx := &mockTx{
			execFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.CommandTag{}, pgErr
			},
		}
		repo := NewOutboxRepository(nil)

		err := repo.InsertEvent(ctx, tx, "payment.completed", aggregateID, payload)
		if err != nil {
			t.Errorf("expected no error on unique violation, got %v", err)
		}
	})

	t.Run("returns error on non-unique violation", func(t *testing.T) {
		tx := &mockTx{
			execFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.CommandTag{}, errors.New("database connection failed")
			},
		}
		repo := NewOutboxRepository(nil)

		err := repo.InsertEvent(ctx, tx, "payment.completed", aggregateID, payload)
		if err == nil {
			t.Error("expected error, got nil")
		}
		if !errors.Is(err, errors.New("database connection failed")) && err.Error() != "failed to insert outbox event: database connection failed" {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// TestFetchPendingBatch tests the FetchPendingBatch method.
func TestFetchPendingBatch(t *testing.T) {
	ctx := context.Background()

	t.Run("fetches pending events", func(t *testing.T) {
		eventID1 := uuid.New()
		eventID2 := uuid.New()
		aggregateID := uuid.New()
		now := time.Now()

		rows := &mockRows{
			values: [][]any{
				{eventID1, "payment", aggregateID, "payment.completed", []byte(`{"test": "data"}`), StatusPending, 0, now, now},
				{eventID2, "order", aggregateID, "order.created", []byte(`{"test": "data2"}`), StatusPending, 0, now, now},
			},
		}

		tx := &mockTx{
			queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
				return rows, nil
			},
		}
		repo := NewOutboxRepository(nil)

		events, err := repo.FetchPendingBatch(ctx, tx, 10)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if len(events) != 2 {
			t.Errorf("expected 2 events, got %d", len(events))
		}
		if events[0].ID != eventID1 {
			t.Errorf("expected event ID %v, got %v", eventID1, events[0].ID)
		}
		if events[1].ID != eventID2 {
			t.Errorf("expected event ID %v, got %v", eventID2, events[1].ID)
		}
	})

	t.Run("returns empty batch when no events", func(t *testing.T) {
		rows := &mockRows{
			values: [][]any{},
		}

		tx := &mockTx{
			queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
				return rows, nil
			},
		}
		repo := NewOutboxRepository(nil)

		events, err := repo.FetchPendingBatch(ctx, tx, 10)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if len(events) != 0 {
			t.Errorf("expected 0 events, got %d", len(events))
		}
	})

	t.Run("respects limit parameter", func(t *testing.T) {
		rows := &mockRows{
			values: [][]any{
				{uuid.New(), "payment", uuid.New(), "payment.completed", []byte(`{}`), StatusPending, 0, time.Now(), time.Now()},
			},
		}

		tx := &mockTx{
			queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
				if len(args) < 3 {
					t.Errorf("expected at least 3 args, got %d", len(args))
				}
				limit, ok := args[2].(int)
				if !ok || limit != 5 {
					t.Errorf("expected limit to be 5, got %v", args[2])
				}
				return rows, nil
			},
		}
		repo := NewOutboxRepository(nil)

		events, err := repo.FetchPendingBatch(ctx, tx, 5)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if len(events) != 1 {
			t.Errorf("expected 1 event, got %d", len(events))
		}
	})

	t.Run("returns error on query failure", func(t *testing.T) {
		tx := &mockTx{
			queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
				return nil, errors.New("query failed")
			},
		}
		repo := NewOutboxRepository(nil)

		_, err := repo.FetchPendingBatch(ctx, tx, 10)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

// TestMarkProcessing tests the MarkProcessing method.
func TestMarkProcessing(t *testing.T) {
	ctx := context.Background()
	eventID := uuid.New()

	t.Run("marks pending event as processing", func(t *testing.T) {
		tx := &mockTx{
			execFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("UPDATE 1"), nil
			},
		}
		repo := NewOutboxRepository(nil)

		err := repo.MarkProcessing(ctx, tx, eventID)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("returns ErrInvalidStatusTransition when not pending", func(t *testing.T) {
		tx := &mockTx{
			execFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				// Simulate no rows matched (event not in pending state)
				return pgconn.NewCommandTag("UPDATE 0"), nil
			},
		}
		repo := NewOutboxRepository(nil)

		err := repo.MarkProcessing(ctx, tx, eventID)
		if err != ErrInvalidStatusTransition {
			t.Errorf("expected ErrInvalidStatusTransition, got %v", err)
		}
	})

	t.Run("returns error on exec failure", func(t *testing.T) {
		tx := &mockTx{
			execFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.CommandTag{}, errors.New("exec failed")
			},
		}
		repo := NewOutboxRepository(nil)

		err := repo.MarkProcessing(ctx, tx, eventID)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

// TestMarkSucceeded tests the MarkSucceeded method.
func TestMarkSucceeded(t *testing.T) {
	ctx := context.Background()
	eventID := uuid.New()

	t.Run("marks event as succeeded", func(t *testing.T) {
		tx := &mockTx{
			execFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("UPDATE 1"), nil
			},
		}
		repo := NewOutboxRepository(nil)

		err := repo.MarkSucceeded(ctx, tx, eventID)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("returns ErrEventNotFound when event not found", func(t *testing.T) {
		tx := &mockTx{
			execFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("UPDATE 0"), nil
			},
		}
		repo := NewOutboxRepository(nil)

		err := repo.MarkSucceeded(ctx, tx, eventID)
		if err != ErrEventNotFound {
			t.Errorf("expected ErrEventNotFound, got %v", err)
		}
	})

	t.Run("returns error on exec failure", func(t *testing.T) {
		tx := &mockTx{
			execFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.CommandTag{}, errors.New("exec failed")
			},
		}
		repo := NewOutboxRepository(nil)

		err := repo.MarkSucceeded(ctx, tx, eventID)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

// TestIsUniqueViolation tests the isUniqueViolation helper function.
func TestIsUniqueViolation(t *testing.T) {
	t.Run("returns true for unique violation", func(t *testing.T) {
		err := &pgconn.PgError{Code: "23505", Message: "duplicate key"}
		if !isUniqueViolation(err) {
			t.Error("expected true for unique violation")
		}
	})

	t.Run("returns false for other errors", func(t *testing.T) {
		err := errors.New("generic error")
		if isUniqueViolation(err) {
			t.Error("expected false for generic error")
		}
	})

	t.Run("returns false for nil", func(t *testing.T) {
		if isUniqueViolation(nil) {
			t.Error("expected false for nil error")
		}
	})

	t.Run("returns false for non-unique constraint errors", func(t *testing.T) {
		err := &pgconn.PgError{Code: "40001", Message: "serialization failure"}
		if isUniqueViolation(err) {
			t.Error("expected false for serialization failure")
		}
	})
}

// TestEventStatus tests the EventStatus constants.
func TestEventStatus(t *testing.T) {
	t.Run("status constants have correct values", func(t *testing.T) {
		if StatusPending != "pending" {
			t.Errorf("expected 'pending', got %q", StatusPending)
		}
		if StatusProcessing != "processing" {
			t.Errorf("expected 'processing', got %q", StatusProcessing)
		}
		if StatusSucceeded != "succeeded" {
			t.Errorf("expected 'succeeded', got %q", StatusSucceeded)
		}
		if StatusFailed != "failed" {
			t.Errorf("expected 'failed', got %q", StatusFailed)
		}
		if StatusDeadLetter != "dead_letter" {
			t.Errorf("expected 'dead_letter', got %q", StatusDeadLetter)
		}
	})
}

// TestEvent tests the Event struct.
func TestEvent(t *testing.T) {
	t.Run("creates valid event", func(t *testing.T) {
		id := uuid.New()
		aggregateID := uuid.New()
		payload := []byte(`{"test": "data"}`)
		now := time.Now()

		event := Event{
			ID:            id,
			AggregateType: "payment",
			AggregateID:   aggregateID,
			EventType:     "payment.completed",
			Payload:       payload,
			Status:        StatusPending,
			RetryCount:    0,
			CreatedAt:     now,
			UpdatedAt:     now,
		}

		if event.ID != id {
			t.Errorf("expected ID %v, got %v", id, event.ID)
		}
		if event.AggregateType != "payment" {
			t.Errorf("expected AggregateType 'payment', got %q", event.AggregateType)
		}
		if event.EventType != "payment.completed" {
			t.Errorf("expected EventType 'payment.completed', got %q", event.EventType)
		}
	})
}


