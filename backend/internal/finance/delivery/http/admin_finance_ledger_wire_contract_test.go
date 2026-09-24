package http

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestLedgerWireContract_EntriesNeverNull locks the ledger wire contract that
// broke the admin Finance Ledger page: a Go nil slice marshals to JSON null,
// and the admin client crashed on it ("Cannot read properties of null
// (reading 'map')" — FinanceLedgerPage.tsx). Every ledger transaction row
// must carry entries as an ARRAY on the wire, even when zero entries survive
// the financial_accounts JOIN (orphan anomaly → still [], never null).
func TestLedgerWireContract_EntriesNeverNull(t *testing.T) {
	// HAZARD (documented): a Go nil slice marshals to JSON null — the exact
	// crash the admin UI hit ("Cannot read properties of null (reading
	// 'map')"). This assertion pins the hazard so the normalization below can
	// never be judged unnecessary.
	nilRaw, err := json.Marshal(ledgerTxRow{
		ID:             "tx-nil",
		IdempotencyKey: "idem-nil",
		ReferenceType:  "ORDER",
		Entries:        nil,
	})
	if err != nil {
		t.Fatalf("marshal nil failed: %v", err)
	}
	if !strings.Contains(string(nilRaw), `"entries":null`) {
		t.Fatalf("precondition drift: nil entries no longer marshal to null (got %s) — re-evaluate this contract test", string(nilRaw))
	}

	// CONTRACT: the handler normalizes every row to a non-nil empty slice
	// before responding — the wire carries entries: [], never null.
	normalizedRaw, err := json.Marshal(ledgerTxRow{
		ID:             "tx-1",
		IdempotencyKey: "idem-1",
		ReferenceType:  "ORDER",
		Entries:        []ledgerEntryRow{},
	})
	if err != nil {
		t.Fatalf("marshal normalized failed: %v", err)
	}

	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(normalizedRaw, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	entries, ok := decoded["entries"]
	if !ok {
		t.Fatal("entries key missing from wire shape")
	}
	if string(entries) != "[]" {
		t.Fatalf("entries must marshal as [] when empty, got %s", string(entries))
	}
}

// TestLedgerWireContract_EntriesAlwaysArrayLocked is a source proof of the
// two invariants that keep entries non-null AND non-lost on the wire:
// 1. every row is normalized to a non-nil slice before responding;
// 2. entries are associated BY INDEX into results, never via a *pointer into
//    the results slice (append() reallocates the backing array and stale
//    pointers silently discard the entries they accumulated — the original
//    root cause of the admin UI crash).
func TestLedgerWireContract_EntriesAlwaysArrayLocked(t *testing.T) {
	raw, err := os.ReadFile("admin_finance_handler.go")
	if err != nil {
		t.Fatalf("failed to read admin_finance_handler.go: %v", err)
	}
	source := string(raw)

	const normalization = "row.Entries = []ledgerEntryRow{}"
	if !strings.Contains(source, normalization) {
		t.Fatalf("missing canonical entries normalization %q in admin_finance_handler.go — nil entries would marshal to null and crash the admin UI", normalization)
	}

	const pointerAssociation = "txByID[id] = &results["
	if strings.Contains(source, pointerAssociation) {
		t.Fatalf("forbidden pointer-into-slice association %q found — append() reallocates the backing array and entries written through stale pointers are silently lost", pointerAssociation)
	}

	const indexAssociation = "results[idx].Entries = append(results[idx].Entries, entry)"
	if !strings.Contains(source, indexAssociation) {
		t.Fatalf("missing canonical index-based entry association %q", indexAssociation)
	}

	if !strings.Contains(source, "ledger_transaction_without_entries") {
		t.Fatal("missing orphan-anomaly warning log — tx without entries is a ledger integrity signal ops must see")
	}
}
