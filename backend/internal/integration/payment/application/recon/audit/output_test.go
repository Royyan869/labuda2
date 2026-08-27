package audit

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/labuda/backend/internal/integration/payment/application/recon"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

var (
	fixedStart = time.Date(2026, time.May, 12, 9, 0, 0, 0, time.UTC)
	fixedEnd   = time.Date(2026, time.May, 12, 9, 0, 12, 500_000_000, time.UTC)
	orderUUID  = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	refundUUID = uuid.MustParse("55555555-5555-5555-5555-555555555555")
)

func sampleReport() *Report {
	oid := orderUUID
	rid := refundUUID
	return &Report{
		StartedAt:          fixedStart,
		FinishedAt:         fixedEnd,
		DurationSeconds:    fixedEnd.Sub(fixedStart).Seconds(),
		CandidatesScanned:  50,
		SnapshotsResolved:  48,
		GatewayQueriesMade: 47,
		GatewayBudget:      100,
		SuppressedNoOrphan: 44,
		Thresholds: recon.Thresholds{
			PendingPaymentGrace:       3 * time.Minute,
			OrphanRecoveryGrace:       2 * time.Minute,
			StuckRefundGrace:          5 * time.Minute,
			PendingPaymentExpiryGrace: 1 * time.Minute,
		},
		Findings: []recon.Finding{
			{
				DriftClass:      recon.DriftD4PartialRefundMismatch,
				Severity:        recon.SeverityHigh,
				OrderID:         &oid,
				MidtransOrderID: "ORDER-LABUDA-001",
				DetectedAt:      fixedStart,
				IdempotencyKey:  "recon|D4_partial_refund_mismatch|order=" + oid.String() + "|d=20260512",
				SuggestedAction: "reconcile refund rows via operator /admin/refunds/{id}/resync-from-gateway",
				Notes:           "gateway successful refund total=50000, local successful refund total=30000",
				GatewayObservedAmount: 50_000,
				LocalObservedAmount:   30_000,
			},
			{
				DriftClass:      recon.DriftD14LedgerEntryOutboxMissing,
				Severity:        recon.SeverityHigh,
				OrderID:         &oid,
				MidtransOrderID: "ORDER-LABUDA-001",
				RefundID:        &rid,
				DetectedAt:      fixedStart,
				IdempotencyKey:  "recon|D14_ledger_entry_outbox_missing|order=" + oid.String() + "|refund=" + rid.String() + "|d=20260512",
				SuggestedAction: "replay money.refund_succeeded outbox emission via operator endpoint",
				Notes:           "refund_reversal_<refund_id> exists but money.refund_succeeded outbox is absent or dead-letter",
			},
		},
		Errors: []RecordedError{
			{OrderID: oid, Stage: "gateway", Message: "midtrans status http: context deadline exceeded"},
		},
		FindingsByClass: map[string]int{
			string(recon.DriftD4PartialRefundMismatch):     1,
			string(recon.DriftD14LedgerEntryOutboxMissing): 1,
		},
		FindingsBySeverity: map[string]int{
			string(recon.SeverityHigh): 2,
		},
	}
}

// ---------------------------------------------------------------------------
// Mode dispatch
// ---------------------------------------------------------------------------

func TestParseOutputMode(t *testing.T) {
	for _, c := range []struct {
		in   string
		want OutputMode
		ok   bool
	}{
		{"summary", OutputSummary, true},
		{"DETAILED", OutputDetailed, true},
		{"  json  ", OutputJSON, true},
		{"yaml", "", false},
		{"", "", false},
	} {
		got, err := ParseOutputMode(c.in)
		if (err == nil) != c.ok {
			t.Errorf("ParseOutputMode(%q): err=%v want_ok=%v", c.in, err, c.ok)
		}
		if c.ok && got != c.want {
			t.Errorf("ParseOutputMode(%q): got=%q want=%q", c.in, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Determinism: each render is byte-identical across repeated invocations.
// ---------------------------------------------------------------------------

func TestRender_DeterministicAcrossInvocations(t *testing.T) {
	report := sampleReport()
	for _, mode := range []OutputMode{OutputSummary, OutputDetailed, OutputJSON} {
		mode := mode
		t.Run(string(mode), func(t *testing.T) {
			first := renderToBytes(t, report, mode)
			for i := 0; i < 32; i++ {
				again := renderToBytes(t, report, mode)
				if !bytes.Equal(first, again) {
					t.Fatalf("non-deterministic %s output at iteration %d", mode, i)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Summary structural checks (no exact-text golden — fields stay stable but
// human-readable text may evolve; verify presence of every key signal).
// ---------------------------------------------------------------------------

func TestRender_SummaryContainsKeyFields(t *testing.T) {
	out := renderToString(t, sampleReport(), OutputSummary)
	must := []string{
		"Gateway Payment Reconciliation",
		"Candidates scanned:     50",
		"Snapshots resolved:     48",
		"Gateway queries:        47",
		"Total findings:         2",
		"Findings by drift class",
		string(recon.DriftD4PartialRefundMismatch),
		string(recon.DriftD14LedgerEntryOutboxMissing),
		"Findings by severity",
		"high",
		"Orders with zero findings: 44",
		"Resolver errors:           1",
	}
	for _, m := range must {
		if !strings.Contains(out, m) {
			t.Errorf("summary missing fragment %q\nfull output:\n%s", m, out)
		}
	}
}

func TestRender_DetailedContainsPerFindingFields(t *testing.T) {
	out := renderToString(t, sampleReport(), OutputDetailed)
	must := []string{
		"Finding #1",
		"Finding #2",
		string(recon.DriftD4PartialRefundMismatch),
		string(recon.DriftD14LedgerEntryOutboxMissing),
		"ORDER-LABUDA-001",
		orderUUID.String(),
		refundUUID.String(),
		"Gateway amount:  50000",
		"Local amount:    30000",
		"Resolver errors:",
		"context deadline exceeded",
	}
	for _, m := range must {
		if !strings.Contains(out, m) {
			t.Errorf("detailed missing fragment %q\nfull output:\n%s", m, out)
		}
	}
}

func TestRender_JSON_IsValidAndCarriesEveryFinding(t *testing.T) {
	out := renderToString(t, sampleReport(), OutputJSON)
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\noutput:\n%s", err, out)
	}
	if got, want := int(parsed["candidates_scanned"].(float64)), 50; got != want {
		t.Errorf("candidates_scanned: got=%d want=%d", got, want)
	}
	findings, ok := parsed["findings"].([]any)
	if !ok {
		t.Fatalf("findings not a JSON array: %T", parsed["findings"])
	}
	if len(findings) != 2 {
		t.Errorf("expected 2 findings, got %d", len(findings))
	}
	errors, ok := parsed["errors"].([]any)
	if !ok {
		t.Fatalf("errors not a JSON array: %T", parsed["errors"])
	}
	if len(errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(errors))
	}
}

// ---------------------------------------------------------------------------
// Empty / zero-findings rendering
// ---------------------------------------------------------------------------

func TestRender_ZeroFindings(t *testing.T) {
	empty := &Report{
		StartedAt:          fixedStart,
		FinishedAt:         fixedEnd,
		Findings:           []recon.Finding{},
		Errors:             []RecordedError{},
		FindingsByClass:    map[string]int{},
		FindingsBySeverity: map[string]int{},
	}
	for _, mode := range []OutputMode{OutputSummary, OutputDetailed, OutputJSON} {
		mode := mode
		t.Run(string(mode), func(t *testing.T) {
			out := renderToString(t, empty, mode)
			if mode == OutputJSON {
				var parsed map[string]any
				if err := json.Unmarshal([]byte(out), &parsed); err != nil {
					t.Fatalf("invalid JSON: %v", err)
				}
				return
			}
			if !strings.Contains(out, "Gateway Payment Reconciliation") {
				t.Errorf("missing header in zero-findings %s output", mode)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Gateway-translation correctness (pure unit test, no HTTP)
// ---------------------------------------------------------------------------

func TestTranslateGatewayResponse_BasicSettlement(t *testing.T) {
	queriedAt := time.Date(2026, time.May, 12, 10, 0, 0, 0, time.UTC)
	raw := gatewayStatusResponse{
		OrderID:           "ORDER-LABUDA-001",
		TransactionStatus: "Settlement", // mixed case to verify normalisation
		TransactionTime:   "2026-05-12 09:30:00",
		SettlementTime:    "2026-05-12 09:30:12",
		GrossAmount:       "100000.00",
		TransactionID:     "MT-001",
		StatusCode:        "200",
		PaymentType:       "bank_transfer",
		RefundChargebackHistory: []gatewayRefundHistoryRow{
			{RefundKey: "key-1", RefundChargebackID: "MT-RF-1", RefundAmount: "40000.00", Status: "Success", CreatedAt: "2026-05-12 09:45:00"},
			{RefundKey: "key-2", RefundChargebackID: "MT-RF-2", RefundAmount: "10000", Status: "pending", CreatedAt: "2026-05-12 09:50:00"},
		},
	}
	got := translateGatewayResponse(raw, queriedAt)

	if !got.Available {
		t.Error("Available must be true on a valid 200 response")
	}
	if got.TransactionStatus != "settlement" {
		t.Errorf("transaction_status must be normalised to lowercase, got %q", got.TransactionStatus)
	}
	if got.GrossAmount != 100_000 {
		t.Errorf("gross amount: got=%d want=100000", got.GrossAmount)
	}
	if got.SettlementTime == nil {
		t.Error("SettlementTime must be non-nil when raw settlement_time parses")
	}
	if n := len(got.RefundChargebackHistory); n != 2 {
		t.Fatalf("refund history length: got=%d want=2", n)
	}
	if got.RefundChargebackHistory[0].Amount != 40_000 || got.RefundChargebackHistory[0].Status != "success" {
		t.Errorf("refund[0]: %+v", got.RefundChargebackHistory[0])
	}
	if got.RefundChargebackHistory[1].Amount != 10_000 || got.RefundChargebackHistory[1].Status != "pending" {
		t.Errorf("refund[1]: %+v", got.RefundChargebackHistory[1])
	}
	if got.QueriedAt != queriedAt {
		t.Error("queried_at must echo the input")
	}
}

func TestTranslateGatewayResponse_EmptyRefundHistory(t *testing.T) {
	queriedAt := time.Date(2026, time.May, 12, 10, 0, 0, 0, time.UTC)
	raw := gatewayStatusResponse{
		OrderID:           "ORDER-LABUDA-002",
		TransactionStatus: "pending",
		TransactionTime:   "2026-05-12 09:30:00",
		GrossAmount:       "75000",
	}
	got := translateGatewayResponse(raw, queriedAt)
	if got.Available != true {
		t.Error("Available must be true")
	}
	if got.SettlementTime != nil {
		t.Error("SettlementTime must be nil when raw is absent")
	}
	if len(got.RefundChargebackHistory) != 0 {
		t.Errorf("expected empty refund history, got %d entries", len(got.RefundChargebackHistory))
	}
}

func TestParseGatewayAmount_EdgeCases(t *testing.T) {
	cases := map[string]int64{
		"":             0,
		"   ":          0,
		"abc":          0,
		"100000":       100_000,
		"100000.00":    100_000,
		"  50000  ":    50_000,
		"-10":          -10,
		"99999999.99":  99_999_999,
	}
	for in, want := range cases {
		if got := parseGatewayAmount(in); got != want {
			t.Errorf("parseGatewayAmount(%q): got=%d want=%d", in, got, want)
		}
	}
}

func TestParseGatewayTime_EdgeCases(t *testing.T) {
	if got := parseGatewayTime(""); !got.IsZero() {
		t.Error("empty must return zero time")
	}
	if got := parseGatewayTime("not-a-date"); !got.IsZero() {
		t.Error("garbage must return zero time")
	}
	got := parseGatewayTime("2026-05-12 09:30:00")
	if got.Year() != 2026 || got.Month() != time.May || got.Day() != 12 {
		t.Errorf("midtrans format mis-parsed: %v", got)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func renderToBytes(t *testing.T, r *Report, mode OutputMode) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := Render(&buf, r, mode); err != nil {
		t.Fatalf("Render(%s): %v", mode, err)
	}
	return append([]byte(nil), buf.Bytes()...)
}

func renderToString(t *testing.T, r *Report, mode OutputMode) string {
	return string(renderToBytes(t, r, mode))
}


