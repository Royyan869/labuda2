package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	platformevent "github.com/labuda/backend/internal/platform/event"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap/zaptest"
)

// =============================================================================
// Test helpers
// =============================================================================

// bnrMockTx captures Exec calls for assertion.
type bnrMockTx struct {
	execCalls []struct {
		sql  string
		args []any
	}
	execErr error
}

func (m *bnrMockTx) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	m.execCalls = append(m.execCalls, struct {
		sql  string
		args []any
	}{sql, args})
	return pgconn.NewCommandTag("INSERT 0 1"), m.execErr
}

func (m *bnrMockTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, errors.New("not implemented")
}

func (m *bnrMockTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return nil
}

func (m *bnrMockTx) Commit(_ context.Context) error  { return nil }
func (m *bnrMockTx) Rollback(_ context.Context) error { return nil }

// bnrMockDB wraps a mock tx into a Transactor.
type bnrMockDB struct {
	tx    *bnrMockTx
	txErr error
}

func (m *bnrMockDB) WithTx(_ context.Context, fn func(db.Tx) error) error {
	if m.txErr != nil {
		return m.txErr
	}
	return fn(m.tx)
}

// =============================================================================
// Tests
// =============================================================================

func TestBNRStrikeHandler_RecordsStrike(t *testing.T) {
	tx := &bnrMockTx{}
	mdb := &bnrMockDB{tx: tx}
	h := NewBNRStrikeHandler(mdb, zaptest.NewLogger(t))

	auctionID := uuid.New()
	winnerID := uuid.New()
	sellerID := uuid.New()

	payload, _ := json.Marshal(map[string]interface{}{
		"auction_id": auctionID.String(),
		"winner_id":  winnerID.String(),
		"seller_id":  sellerID.String(),
		"timestamp":  "2026-05-26T12:00:00Z",
	})

	event := platformevent.OutboxEvent{
		ID:          uuid.New(),
		AggregateID: auctionID,
		EventType:   "auction_bnr_detected",
		Payload:     payload,
	}

	err := h.Handle(context.Background(), event)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if len(tx.execCalls) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(tx.execCalls))
	}

	call := tx.execCalls[0]
	if len(call.args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(call.args))
	}
	if call.args[0] != winnerID {
		t.Errorf("arg[0] (buyer_id) = %v, want %v", call.args[0], winnerID)
	}
	if call.args[1] != auctionID {
		t.Errorf("arg[1] (auction_id) = %v, want %v", call.args[1], auctionID)
	}
}

func TestBNRStrikeHandler_MalformedPayload_NilError(t *testing.T) {
	mdb := &bnrMockDB{tx: &bnrMockTx{}}
	h := NewBNRStrikeHandler(mdb, zaptest.NewLogger(t))

	event := platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "auction_bnr_detected",
		Payload:   []byte(`{not valid json`),
	}

	err := h.Handle(context.Background(), event)
	if err != nil {
		t.Fatalf("expected nil error for malformed payload, got: %v", err)
	}

	if len(mdb.tx.execCalls) != 0 {
		t.Error("no SQL should be executed for malformed payload")
	}
}

func TestBNRStrikeHandler_NilAuctionID_NilError(t *testing.T) {
	mdb := &bnrMockDB{tx: &bnrMockTx{}}
	h := NewBNRStrikeHandler(mdb, zaptest.NewLogger(t))

	payload, _ := json.Marshal(map[string]interface{}{
		"auction_id": uuid.Nil.String(),
		"winner_id":  uuid.New().String(),
	})

	event := platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "auction_bnr_detected",
		Payload:   payload,
	}

	err := h.Handle(context.Background(), event)
	if err != nil {
		t.Fatalf("expected nil error for nil auction_id, got: %v", err)
	}
}

func TestBNRStrikeHandler_NilWinnerID_NilError(t *testing.T) {
	mdb := &bnrMockDB{tx: &bnrMockTx{}}
	h := NewBNRStrikeHandler(mdb, zaptest.NewLogger(t))

	payload, _ := json.Marshal(map[string]interface{}{
		"auction_id": uuid.New().String(),
		"winner_id":  uuid.Nil.String(),
	})

	event := platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "auction_bnr_detected",
		Payload:   payload,
	}

	err := h.Handle(context.Background(), event)
	if err != nil {
		t.Fatalf("expected nil error for nil winner_id, got: %v", err)
	}
}

func TestBNRStrikeHandler_DBError_Propagates(t *testing.T) {
	tx := &bnrMockTx{execErr: errors.New("db connection lost")}
	mdb := &bnrMockDB{tx: tx}
	h := NewBNRStrikeHandler(mdb, zaptest.NewLogger(t))

	payload, _ := json.Marshal(map[string]interface{}{
		"auction_id": uuid.New().String(),
		"winner_id":  uuid.New().String(),
		"seller_id":  uuid.New().String(),
	})

	event := platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "auction_bnr_detected",
		Payload:   payload,
	}

	err := h.Handle(context.Background(), event)
	if err == nil {
		t.Fatal("expected error when DB fails")
	}
}

func TestBNRStrikeHandler_NilLogger_NoPanic(t *testing.T) {
	h := NewBNRStrikeHandler(&bnrMockDB{tx: &bnrMockTx{}}, nil)
	if h == nil {
		t.Fatal("NewBNRStrikeHandler with nil logger returned nil")
	}
}

// TestBNRStrikeHandler_SQL_ContainsOnConflict verifies the INSERT uses
// ON CONFLICT (auction_id) DO NOTHING for replay idempotency.
func TestBNRStrikeHandler_SQL_ContainsOnConflict(t *testing.T) {
	tx := &bnrMockTx{}
	mdb := &bnrMockDB{tx: tx}
	h := NewBNRStrikeHandler(mdb, zaptest.NewLogger(t))

	payload, _ := json.Marshal(map[string]interface{}{
		"auction_id": uuid.New().String(),
		"winner_id":  uuid.New().String(),
		"seller_id":  uuid.New().String(),
	})

	event := platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "auction_bnr_detected",
		Payload:   payload,
	}

	_ = h.Handle(context.Background(), event)

	if len(tx.execCalls) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(tx.execCalls))
	}

	sql := tx.execCalls[0].sql
	if !containsSubstring(sql, "ON CONFLICT") {
		t.Error("SQL missing ON CONFLICT clause")
	}
	if !containsSubstring(sql, "DO NOTHING") {
		t.Error("SQL missing DO NOTHING clause")
	}
	if !containsSubstring(sql, "buyer_bnr_strikes") {
		t.Error("SQL not targeting buyer_bnr_strikes table")
	}
}

// TestBNRStrikeHandler_ReplayProof verifies that calling Handle twice with
// the same auction produces exactly 2 Exec calls (the DB's UNIQUE constraint
// makes the second a no-op via ON CONFLICT DO NOTHING).
func TestBNRStrikeHandler_ReplayProof(t *testing.T) {
	tx := &bnrMockTx{}
	mdb := &bnrMockDB{tx: tx}
	h := NewBNRStrikeHandler(mdb, zaptest.NewLogger(t))

	auctionID := uuid.New()
	winnerID := uuid.New()

	payload, _ := json.Marshal(map[string]interface{}{
		"auction_id": auctionID.String(),
		"winner_id":  winnerID.String(),
		"seller_id":  uuid.New().String(),
	})

	event := platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "auction_bnr_detected",
		Payload:   payload,
	}

	// First call
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("first Handle: %v", err)
	}

	// Second call (replay) — same auction_id
	event.ID = uuid.New()
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("second Handle: %v", err)
	}

	// Both calls should have executed INSERT (the DB constraint handles dedup).
	// The handler itself does NOT skip — it trusts ON CONFLICT DO NOTHING.
	if len(tx.execCalls) != 2 {
		t.Fatalf("expected 2 exec calls (both go to DB, dedup at SQL level), got %d", len(tx.execCalls))
	}

	// Both should target same auction_id
	if tx.execCalls[0].args[1] != auctionID {
		t.Error("first call: wrong auction_id")
	}
	if tx.execCalls[1].args[1] != auctionID {
		t.Error("second call: wrong auction_id")
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsHelper(s, sub))
}

func containsHelper(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}


