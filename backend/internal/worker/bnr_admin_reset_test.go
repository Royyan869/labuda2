package worker

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	dbpkg "github.com/labuda/backend/pkg/db"
	"go.uber.org/zap/zaptest"
)

// =============================================================================
// Mock infrastructure (BNR admin reset)
// =============================================================================

type resetMockTx struct {
	execCalls []struct {
		sql  string
		args []any
	}
	rowsAffected int64
	execErr      error
}

func (m *resetMockTx) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	m.execCalls = append(m.execCalls, struct {
		sql  string
		args []any
	}{sql, args})
	if m.execErr != nil {
		return pgconn.CommandTag{}, m.execErr
	}
	return pgconn.NewCommandTag(fmt.Sprintf("UPDATE %d", m.rowsAffected)), nil
}

func (m *resetMockTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *resetMockTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return nil
}

func (m *resetMockTx) Commit(_ context.Context) error  { return nil }
func (m *resetMockTx) Rollback(_ context.Context) error { return nil }

type resetMockDB struct {
	tx    *resetMockTx
	txErr error
}

func (m *resetMockDB) WithTx(_ context.Context, fn func(dbpkg.Tx) error) error {
	if m.txErr != nil {
		return m.txErr
	}
	return fn(m.tx)
}

// =============================================================================
// Tests: ResetAllForBuyer
// =============================================================================

// Test 1: admin can reset all active strikes for buyer
func TestBNRAdminReset_AllForBuyer_ResetsActiveStrikes(t *testing.T) {
	tx := &resetMockTx{rowsAffected: 3}
	mdb := &resetMockDB{tx: tx}
	r := NewBNRAdminResetter(mdb, zaptest.NewLogger(t))

	buyerID := uuid.New()
	actorID := uuid.New()
	count, err := r.ResetAllForBuyer(context.Background(), buyerID, actorID)
	if err != nil {
		t.Fatalf("ResetAllForBuyer error: %v", err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}

	if len(tx.execCalls) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(tx.execCalls))
	}

	// Verify SQL
	sql := tx.execCalls[0].sql
	for _, fragment := range []string{
		"admin_reset = TRUE",
		"buyer_id = $1",
		"decayed_at IS NULL",
		"admin_reset = FALSE",
	} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("SQL missing %q", fragment)
		}
	}

	// Verify buyer_id arg
	if tx.execCalls[0].args[0] != buyerID {
		t.Errorf("arg[0] = %v, want %v", tx.execCalls[0].args[0], buyerID)
	}
}

// Test 2: decayed strikes not affected (SQL filters decayed_at IS NULL)
func TestBNRAdminReset_DecayedStrikesNotAffected(t *testing.T) {
	// If buyer only has decayed strikes, rowsAffected = 0
	tx := &resetMockTx{rowsAffected: 0}
	mdb := &resetMockDB{tx: tx}
	r := NewBNRAdminResetter(mdb, zaptest.NewLogger(t))

	count, err := r.ResetAllForBuyer(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("ResetAllForBuyer error: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0 (decayed strikes should be excluded)", count)
	}

	// Verify SQL filter present
	if !strings.Contains(tx.execCalls[0].sql, "decayed_at IS NULL") {
		t.Error("SQL missing decayed_at IS NULL filter")
	}
}

// Test 3: already reset strikes not counted again
func TestBNRAdminReset_AlreadyResetNotCounted(t *testing.T) {
	// All strikes already reset → 0 affected
	tx := &resetMockTx{rowsAffected: 0}
	mdb := &resetMockDB{tx: tx}
	r := NewBNRAdminResetter(mdb, zaptest.NewLogger(t))

	count, err := r.ResetAllForBuyer(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("ResetAllForBuyer error: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0 (already reset strikes excluded)", count)
	}

	// Verify SQL filter present
	if !strings.Contains(tx.execCalls[0].sql, "admin_reset = FALSE") {
		t.Error("SQL missing admin_reset = FALSE filter")
	}
}

// Test 4: DB error propagates
func TestBNRAdminReset_AllForBuyer_DBError(t *testing.T) {
	tx := &resetMockTx{execErr: fmt.Errorf("connection lost")}
	mdb := &resetMockDB{tx: tx}
	r := NewBNRAdminResetter(mdb, zaptest.NewLogger(t))

	_, err := r.ResetAllForBuyer(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error when DB fails")
	}
	if !strings.Contains(err.Error(), "bnr_admin_reset") {
		t.Errorf("error should contain 'bnr_admin_reset', got: %v", err)
	}
}

// =============================================================================
// Tests: ResetStrike (single)
// =============================================================================

// Test 5: single strike reset succeeds
func TestBNRAdminReset_SingleStrike_Updated(t *testing.T) {
	tx := &resetMockTx{rowsAffected: 1}
	mdb := &resetMockDB{tx: tx}
	r := NewBNRAdminResetter(mdb, zaptest.NewLogger(t))

	strikeID := uuid.New()
	updated, err := r.ResetStrike(context.Background(), strikeID, uuid.New())
	if err != nil {
		t.Fatalf("ResetStrike error: %v", err)
	}
	if !updated {
		t.Error("expected updated = true")
	}

	if len(tx.execCalls) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(tx.execCalls))
	}

	// Verify SQL targets single ID
	sql := tx.execCalls[0].sql
	if !strings.Contains(sql, "id = $1") {
		t.Error("SQL missing id = $1 (single-strike target)")
	}
	if tx.execCalls[0].args[0] != strikeID {
		t.Errorf("arg[0] = %v, want %v", tx.execCalls[0].args[0], strikeID)
	}
}

// Test 6: single strike not found or already reset → updated = false
func TestBNRAdminReset_SingleStrike_NotFound(t *testing.T) {
	tx := &resetMockTx{rowsAffected: 0}
	mdb := &resetMockDB{tx: tx}
	r := NewBNRAdminResetter(mdb, zaptest.NewLogger(t))

	updated, err := r.ResetStrike(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("ResetStrike error: %v", err)
	}
	if updated {
		t.Error("expected updated = false for non-existent/already-reset strike")
	}
}

// Test 7: nil logger is safe
func TestBNRAdminReset_NilLogger(t *testing.T) {
	tx := &resetMockTx{rowsAffected: 0}
	mdb := &resetMockDB{tx: tx}
	r := NewBNRAdminResetter(mdb, nil)
	if r == nil {
		t.Fatal("NewBNRAdminResetter with nil logger returned nil")
	}
	_, err := r.ResetAllForBuyer(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}


