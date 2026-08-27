package worker

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/commerce/subscription/entity"
	db "github.com/labuda/backend/pkg/db"
	"go.uber.org/zap/zaptest"
)

// =============================================================================
// SV1B: ProcessExpiringReminders UNIT TESTS
// =============================================================================

// mockOutboxForExpiry captures outbox insert calls.
type mockOutboxForExpiry struct {
	mu      sync.Mutex
	inserts []mockOutboxInsert
}

type mockOutboxInsert struct {
	EventType      string
	IdempotencyKey string
}

func (m *mockOutboxForExpiry) InsertTx(_ context.Context, _ db.Tx, eventType string, _ any, idempotencyKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inserts = append(m.inserts, mockOutboxInsert{
		EventType:      eventType,
		IdempotencyKey: idempotencyKey,
	})
	return nil
}

// mockRepoForExpiry stubs the SellerSubscriptionRepository interface.
type mockRepoForExpiry struct{}

func (m *mockRepoForExpiry) InsertTx(context.Context, db.Tx, *entity.SellerSubscription) error {
	return nil
}
func (m *mockRepoForExpiry) UpdateStatusTx(context.Context, db.Tx, uuid.UUID, entity.Status, entity.Status) error {
	return nil
}
func (m *mockRepoForExpiry) GetByIDForUpdate(context.Context, db.Tx, uuid.UUID) (*entity.SellerSubscription, error) {
	return nil, nil
}
func (m *mockRepoForExpiry) GetByID(context.Context, db.Tx, uuid.UUID) (*entity.SellerSubscription, error) {
	return nil, nil
}
func (m *mockRepoForExpiry) GetLatestByUserID(context.Context, db.Tx, uuid.UUID) (*entity.SellerSubscription, error) {
	return nil, nil
}
func (m *mockRepoForExpiry) GetLatestByUserIDForUpdate(context.Context, db.Tx, uuid.UUID) (*entity.SellerSubscription, error) {
	return nil, nil
}
func (m *mockRepoForExpiry) FetchActiveExpiredBatch(context.Context, db.Tx, time.Time, int) ([]*entity.SellerSubscription, error) {
	return nil, nil
}
func (m *mockRepoForExpiry) FetchActiveExpiredBatchIDs(context.Context, db.Tx, time.Time, int) ([]uuid.UUID, error) {
	return nil, nil
}
func (m *mockRepoForExpiry) GetActiveByUserID(context.Context, db.Tx, uuid.UUID) (*entity.SellerSubscription, error) {
	return nil, nil
}
func (m *mockRepoForExpiry) ExistsActiveByUserID(context.Context, db.Tx, uuid.UUID) (bool, error) {
	return false, nil
}
func (m *mockRepoForExpiry) GetActiveConfig(context.Context, db.Tx) (*entity.SellerSubscriptionConfig, error) {
	return nil, nil
}
func (m *mockRepoForExpiry) UpdateConfigTx(context.Context, db.Tx, uuid.UUID, int64, int, int, bool) error {
	return nil
}

// mockTransactorForExpiry returns a mock tx that yields canned subscription rows.
type mockTransactorForExpiry struct {
	sub *expiringSubscription // nil = no results
}

func (m *mockTransactorForExpiry) WithTx(_ context.Context, fn func(db.Tx) error) error {
	return fn(&mockTxForExpiry{sub: m.sub})
}

// mockTxForExpiry implements db.Tx with a Query that returns canned rows.
type mockTxForExpiry struct {
	sub *expiringSubscription
}

func (m *mockTxForExpiry) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("0"), nil
}

func (m *mockTxForExpiry) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &mockRowForNotification{}
}

func (m *mockTxForExpiry) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	if m.sub == nil {
		return &mockExpiryRows{}, nil
	}
	return &mockExpiryRows{sub: m.sub, consumed: false}, nil
}

func (m *mockTxForExpiry) Commit(_ context.Context) error   { return nil }
func (m *mockTxForExpiry) Rollback(_ context.Context) error { return nil }

// mockExpiryRows implements pgx.Rows for the expiry worker query.
type mockExpiryRows struct {
	sub      *expiringSubscription
	consumed bool
}

func (r *mockExpiryRows) Next() bool {
	if r.sub == nil || r.consumed {
		return false
	}
	r.consumed = true
	return true
}

func (r *mockExpiryRows) Scan(dest ...any) error {
	if r.sub == nil {
		return pgx.ErrNoRows
	}
	if len(dest) != 3 {
		return fmt.Errorf("expected 3 scan targets, got %d", len(dest))
	}
	*dest[0].(*uuid.UUID) = r.sub.ID
	*dest[1].(*uuid.UUID) = r.sub.UserID
	*dest[2].(*time.Time) = r.sub.ExpiresAt
	return nil
}

func (r *mockExpiryRows) Close()                                       {}
func (r *mockExpiryRows) Err() error                                   { return nil }
func (r *mockExpiryRows) CommandTag() pgconn.CommandTag                { return pgconn.NewCommandTag("0") }
func (r *mockExpiryRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *mockExpiryRows) RawValues() [][]byte                          { return nil }
func (r *mockExpiryRows) Values() ([]any, error)                       { return nil, nil }
func (r *mockExpiryRows) Conn() *pgx.Conn                              { return nil }

// TestProcessExpiringReminders_EmitsOutboxEvents verifies that the worker
// emits the canonical 7-day reminder exactly once per subscription.
func TestProcessExpiringReminders_EmitsOutboxEvents(t *testing.T) {
	subID := uuid.New()
	userID := uuid.New()
	expiresAt := time.Now().Add(5 * 24 * time.Hour) // 5 days from now

	outbox := &mockOutboxForExpiry{}

	worker := NewSellerSubscriptionExpiryWorker(
		&mockTransactorForExpiry{sub: &expiringSubscription{
			ID: subID, UserID: userID, ExpiresAt: expiresAt,
		}},
		&mockRepoForExpiry{},
		nil,
		outbox,
		zaptest.NewLogger(t),
	)

	err := worker.ProcessExpiringReminders(context.Background())
	if err != nil {
		t.Fatalf("ProcessExpiringReminders() error = %v", err)
	}

	outbox.mu.Lock()
	defer outbox.mu.Unlock()

	// Mock always returns the sub for every threshold query (no SQL filtering).
	// In production the DB WHERE clause filters; here we verify the single reminder threshold.
	expectedCount := 1
	if len(outbox.inserts) != expectedCount {
		t.Errorf("outbox inserts = %d, want %d", len(outbox.inserts), expectedCount)
		for i, ins := range outbox.inserts {
			t.Logf("  insert[%d]: type=%s key=%s", i, ins.EventType, ins.IdempotencyKey)
		}
		return
	}

	for _, ins := range outbox.inserts {
		if ins.EventType != "seller.subscription.expiring" {
			t.Errorf("event type = %s, want seller.subscription.expiring", ins.EventType)
		}
	}

	key7 := fmt.Sprintf("%s.7d", subID)
	if outbox.inserts[0].IdempotencyKey != key7 {
		t.Errorf("key[0] = %s, want %s", outbox.inserts[0].IdempotencyKey, key7)
	}
}

// TestProcessExpiringReminders_NoSubscriptions verifies no events emitted
// when no subscriptions are expiring.
func TestProcessExpiringReminders_NoSubscriptions(t *testing.T) {
	outbox := &mockOutboxForExpiry{}

	worker := NewSellerSubscriptionExpiryWorker(
		&mockTransactorForExpiry{}, // no subscription
		&mockRepoForExpiry{},
		nil,
		outbox,
		zaptest.NewLogger(t),
	)

	err := worker.ProcessExpiringReminders(context.Background())
	if err != nil {
		t.Fatalf("ProcessExpiringReminders() error = %v", err)
	}

	outbox.mu.Lock()
	defer outbox.mu.Unlock()

	if len(outbox.inserts) != 0 {
		t.Errorf("outbox inserts = %d, want 0", len(outbox.inserts))
	}
}
