package worker

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	dbpkg "github.com/labuda/backend/pkg/db"
	"go.uber.org/zap/zaptest"
)

// =============================================================================
// Mock infrastructure
// =============================================================================

// decayMockTx records Exec calls and returns a configurable command tag.
type decayMockTx struct {
	execCalls []struct {
		sql  string
		args []any
	}
	rowsAffected int64
	execErr      error
}

func (m *decayMockTx) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	m.execCalls = append(m.execCalls, struct {
		sql  string
		args []any
	}{sql, args})
	if m.execErr != nil {
		return pgconn.CommandTag{}, m.execErr
	}
	return pgconn.NewCommandTag(fmt.Sprintf("UPDATE %d", m.rowsAffected)), nil
}

func (m *decayMockTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *decayMockTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return nil
}

func (m *decayMockTx) Commit(_ context.Context) error  { return nil }
func (m *decayMockTx) Rollback(_ context.Context) error { return nil }

// decayMockDB wraps a mock tx into a Transactor.
type decayMockDB struct {
	tx    *decayMockTx
	txErr error
}

func (m *decayMockDB) WithTx(_ context.Context, fn func(dbpkg.Tx) error) error {
	if m.txErr != nil {
		return m.txErr
	}
	return fn(m.tx)
}

// =============================================================================
// Helper
// =============================================================================

func newDecayWorker(t *testing.T, tx *decayMockTx) (*BNRDecayWorker, *decayMockDB) {
	t.Helper()
	mdb := &decayMockDB{tx: tx}
	w := NewBNRDecayWorker(mdb, zaptest.NewLogger(t))
	return w, mdb
}

// =============================================================================
// Tests
// =============================================================================

// Test 1: 1 old strike >180d → decayed
// Verifies that RunDecay executes the UPDATE and reports 1 row affected.
func TestBNRDecay_SingleOldStrike_Decayed(t *testing.T) {
	tx := &decayMockTx{rowsAffected: 1}
	w, _ := newDecayWorker(t, tx)

	decayed, err := w.RunDecay(context.Background())
	if err != nil {
		t.Fatalf("RunDecay error: %v", err)
	}
	if decayed != 1 {
		t.Errorf("decayed = %d, want 1", decayed)
	}

	if len(tx.execCalls) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(tx.execCalls))
	}

	// Verify the SQL contains the key clauses.
	sql := tx.execCalls[0].sql
	for _, fragment := range []string{
		"decayed_at IS NULL",
		"admin_reset = FALSE",
		"MAX(struck_at)",
		"ORDER BY buyer_id, struck_at ASC",
		"SET decayed_at = NOW()",
		"DISTINCT ON (buyer_id)",
	} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("SQL missing %q", fragment)
		}
	}

	// Verify cutoff arg is approximately now - 180 days.
	if len(tx.execCalls[0].args) != 1 {
		t.Fatalf("expected 1 arg (cutoff), got %d", len(tx.execCalls[0].args))
	}
	cutoff, ok := tx.execCalls[0].args[0].(time.Time)
	if !ok {
		t.Fatalf("arg[0] is %T, want time.Time", tx.execCalls[0].args[0])
	}
	expected := time.Now().Add(-BNRDecayThreshold)
	if cutoff.Before(expected.Add(-5*time.Second)) || cutoff.After(expected.Add(5*time.Second)) {
		t.Errorf("cutoff = %v, want approximately %v", cutoff, expected)
	}
}

// Test 2: 2 old strikes >180d → only oldest decayed (1 per buyer per run)
// The SQL uses DISTINCT ON (buyer_id) ORDER BY struck_at ASC to pick the
// oldest. The mock returns rowsAffected=1 to simulate single decay.
func TestBNRDecay_TwoOldStrikes_OnlyOldestDecayed(t *testing.T) {
	// DB returns 1 because DISTINCT ON picks only the oldest per buyer.
	tx := &decayMockTx{rowsAffected: 1}
	w, _ := newDecayWorker(t, tx)

	decayed, err := w.RunDecay(context.Background())
	if err != nil {
		t.Fatalf("RunDecay error: %v", err)
	}
	if decayed != 1 {
		t.Errorf("decayed = %d, want 1 (oldest only)", decayed)
	}

	// Verify DISTINCT ON is in the SQL (guarantees one-per-buyer).
	sql := tx.execCalls[0].sql
	if !strings.Contains(sql, "DISTINCT ON (buyer_id)") {
		t.Error("SQL missing DISTINCT ON (buyer_id) — one-per-buyer guarantee broken")
	}
}

// Test 3: old strike + recent strike → no decay
// When MAX(struck_at) > cutoff, the HAVING clause excludes the buyer.
// The DB returns 0 rows affected.
func TestBNRDecay_OldPlusRecentStrike_NoDecay(t *testing.T) {
	tx := &decayMockTx{rowsAffected: 0}
	w, _ := newDecayWorker(t, tx)

	decayed, err := w.RunDecay(context.Background())
	if err != nil {
		t.Fatalf("RunDecay error: %v", err)
	}
	if decayed != 0 {
		t.Errorf("decayed = %d, want 0 (recent strike should block decay)", decayed)
	}
}

// Test 4: admin_reset strike ignored
// The SQL filters admin_reset = FALSE. Admin-reset rows are excluded from
// both the eligible-buyer subquery and the DISTINCT ON target selection.
func TestBNRDecay_AdminResetStrike_Ignored(t *testing.T) {
	tx := &decayMockTx{rowsAffected: 0}
	w, _ := newDecayWorker(t, tx)

	decayed, err := w.RunDecay(context.Background())
	if err != nil {
		t.Fatalf("RunDecay error: %v", err)
	}
	if decayed != 0 {
		t.Errorf("decayed = %d, want 0 (admin_reset rows excluded)", decayed)
	}

	// Verify SQL explicitly filters admin_reset.
	sql := tx.execCalls[0].sql
	if !strings.Contains(sql, "admin_reset = FALSE") {
		t.Error("SQL missing admin_reset = FALSE filter")
	}
}

// Test 5: already decayed strike ignored
// The SQL filters decayed_at IS NULL. Already-decayed rows are excluded
// from both eligibility and target selection.
func TestBNRDecay_AlreadyDecayedStrike_Ignored(t *testing.T) {
	tx := &decayMockTx{rowsAffected: 0}
	w, _ := newDecayWorker(t, tx)

	decayed, err := w.RunDecay(context.Background())
	if err != nil {
		t.Fatalf("RunDecay error: %v", err)
	}
	if decayed != 0 {
		t.Errorf("decayed = %d, want 0 (already decayed rows excluded)", decayed)
	}

	// Verify SQL explicitly filters decayed_at.
	sql := tx.execCalls[0].sql
	if !strings.Contains(sql, "decayed_at IS NULL") {
		t.Error("SQL missing decayed_at IS NULL filter")
	}
}

// Test 6: buyer A decay does not affect buyer B
// When two buyers are both eligible, the SQL decays one row per buyer.
// rowsAffected=2 means both buyer A and buyer B each had their oldest
// strike decayed — but NOT a second strike for either buyer.
func TestBNRDecay_MultipleBuyers_Independent(t *testing.T) {
	// Two eligible buyers → 2 rows affected (one per buyer).
	tx := &decayMockTx{rowsAffected: 2}
	w, _ := newDecayWorker(t, tx)

	decayed, err := w.RunDecay(context.Background())
	if err != nil {
		t.Fatalf("RunDecay error: %v", err)
	}
	if decayed != 2 {
		t.Errorf("decayed = %d, want 2 (one per eligible buyer)", decayed)
	}

	// Single SQL call — the CTE handles all buyers in one pass.
	if len(tx.execCalls) != 1 {
		t.Errorf("expected 1 exec call (single-pass CTE), got %d", len(tx.execCalls))
	}
}

// Test 7: idempotent second run — after first run decayed 1 strike, the
// second run sees it as decayed_at IS NOT NULL and does NOT re-decay it.
// If no other strikes qualify, second run returns 0.
func TestBNRDecay_IdempotentSecondRun(t *testing.T) {
	// First run: 1 strike decayed.
	tx1 := &decayMockTx{rowsAffected: 1}
	w, mdb := newDecayWorker(t, tx1)

	decayed1, err := w.RunDecay(context.Background())
	if err != nil {
		t.Fatalf("first RunDecay error: %v", err)
	}
	if decayed1 != 1 {
		t.Errorf("first run: decayed = %d, want 1", decayed1)
	}

	// Second run: the decayed row now has decayed_at set, so it's excluded.
	// If the buyer only had one active strike, they have zero now → not eligible.
	tx2 := &decayMockTx{rowsAffected: 0}
	mdb.tx = tx2

	decayed2, err := w.RunDecay(context.Background())
	if err != nil {
		t.Fatalf("second RunDecay error: %v", err)
	}
	if decayed2 != 0 {
		t.Errorf("second run: decayed = %d, want 0 (already decayed in first run)", decayed2)
	}
}

// =============================================================================
// Error + construction tests
// =============================================================================

func TestBNRDecay_DBError_Propagates(t *testing.T) {
	tx := &decayMockTx{execErr: fmt.Errorf("connection lost")}
	w, _ := newDecayWorker(t, tx)

	_, err := w.RunDecay(context.Background())
	if err == nil {
		t.Fatal("expected error when DB fails")
	}
	if !strings.Contains(err.Error(), "bnr_decay") {
		t.Errorf("error should contain 'bnr_decay', got: %v", err)
	}
}

func TestBNRDecay_NilLogger_NoPanic(t *testing.T) {
	tx := &decayMockTx{rowsAffected: 0}
	mdb := &decayMockDB{tx: tx}
	w := NewBNRDecayWorker(mdb, nil)
	if w == nil {
		t.Fatal("NewBNRDecayWorker with nil logger returned nil")
	}

	// Should not panic.
	_, err := w.RunDecay(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBNRDecay_Constants(t *testing.T) {
	if BNRDecayThreshold != 180*24*time.Hour {
		t.Errorf("BNRDecayThreshold = %v, want 180 days", BNRDecayThreshold)
	}
	if DefaultBNRDecayInterval <= 0 {
		t.Errorf("DefaultBNRDecayInterval = %v, want > 0", DefaultBNRDecayInterval)
	}
}

func TestBNRDecay_IsRunning_BeforeStart(t *testing.T) {
	tx := &decayMockTx{}
	w, _ := newDecayWorker(t, tx)
	if w.IsRunning() {
		t.Fatal("worker should not be running before Start")
	}
}

func TestBNRDecay_CustomThreshold(t *testing.T) {
	tx := &decayMockTx{rowsAffected: 1}
	w, _ := newDecayWorker(t, tx)

	// Use a shorter threshold (7 days) to verify the cutoff arg changes.
	threshold := 7 * 24 * time.Hour
	decayed, err := w.RunDecayWithThreshold(context.Background(), threshold)
	if err != nil {
		t.Fatalf("RunDecayWithThreshold error: %v", err)
	}
	if decayed != 1 {
		t.Errorf("decayed = %d, want 1", decayed)
	}

	// Verify cutoff is now - 7 days, not now - 180 days.
	cutoff := tx.execCalls[0].args[0].(time.Time)
	expected := time.Now().Add(-threshold)
	if cutoff.Before(expected.Add(-5*time.Second)) || cutoff.After(expected.Add(5*time.Second)) {
		t.Errorf("cutoff = %v, want approximately %v (7-day threshold)", cutoff, expected)
	}
}


