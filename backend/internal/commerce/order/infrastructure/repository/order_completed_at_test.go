package repository

import (
	"os"
	"strings"
	"testing"
)

// =============================================================================
// REGRESSION LOCK: BUG-P7-3 — UpdateStatusTx must persist completed_at.
// The bug: order_completion_service.go sets order.CompletedAt = &now, but
// UpdateStatusTx's SET clause omitted completed_at → always NULL in DB.
// =============================================================================

// TestUpdateStatusTx_IncludesCompletedAt proves the UpdateStatusTx SQL
// includes completed_at in its SET clause so completed orders have a
// non-NULL completed_at in the database.
func TestUpdateStatusTx_IncludesCompletedAt(t *testing.T) {
	src, err := os.ReadFile("order_repository.go")
	if err != nil {
		t.Fatalf("read order_repository.go: %v", err)
	}
	source := string(src)

	// The UpdateStatusTx function must contain completed_at in its SET clause
	if !strings.Contains(source, "completed_at = $12") {
		t.Fatal("UpdateStatusTx SET clause must include 'completed_at = $12'. " +
			"Without this, order.CompletedAt set in order_completion_service.go " +
			"is never persisted — completed_at remains NULL in DB after completion.")
	}

	// updated_at must have shifted to $13 (accounting for the new $12 slot)
	if !strings.Contains(source, "updated_at = $13") {
		t.Fatal("UpdateStatusTx must have 'updated_at = $13' after completed_at was inserted " +
			"as $12. Param numbering mismatch would cause SQL bind errors.")
	}

	// order.CompletedAt must be passed as a parameter
	if !strings.Contains(source, "order.CompletedAt,") {
		t.Fatal("UpdateStatusTx must pass order.CompletedAt as a query parameter " +
			"to actually persist the completion timestamp.")
	}
}

// TestUpdateStatusTx_ParamCount proves UpdateStatusTx passes exactly 13
// positional parameters (WHERE $1 + 12 SET params) to prevent bind errors.
func TestUpdateStatusTx_ParamCount(t *testing.T) {
	src, err := os.ReadFile("order_repository.go")
	if err != nil {
		t.Fatalf("read order_repository.go: %v", err)
	}

	// Find the UpdateStatusTx function body
	funcStart := strings.Index(string(src), "func (r *OrderRepository) UpdateStatusTx(")
	if funcStart == -1 {
		t.Fatal("UpdateStatusTx not found in order_repository.go")
	}
	body := string(src)[funcStart:]

	// $13 must exist (updated_at after completed_at inserted as $12)
	if !strings.Contains(body, "$13") {
		t.Fatal("UpdateStatusTx must reference $13 — updated_at shifted from $12 " +
			"after completed_at was added as $12")
	}

	// $14 must NOT exist (would indicate extra unintended params)
	if strings.Contains(body, "$14") {
		t.Fatal("UpdateStatusTx must NOT reference $14 — only 13 positional params expected")
	}
}
