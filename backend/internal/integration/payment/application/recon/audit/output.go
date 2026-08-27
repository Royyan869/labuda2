package audit

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// OutputMode selects which formatter the CLI uses.
type OutputMode string

const (
	OutputSummary  OutputMode = "summary"
	OutputDetailed OutputMode = "detailed"
	OutputJSON     OutputMode = "json"
)

// ParseOutputMode validates an external string against the supported modes.
func ParseOutputMode(s string) (OutputMode, error) {
	switch OutputMode(strings.ToLower(strings.TrimSpace(s))) {
	case OutputSummary:
		return OutputSummary, nil
	case OutputDetailed:
		return OutputDetailed, nil
	case OutputJSON:
		return OutputJSON, nil
	}
	return "", fmt.Errorf("unsupported output mode %q (expected: summary|detailed|json)", s)
}

// Render emits the report to w in the requested mode. Returns an error only
// if w fails; the report itself never produces a formatting error.
func Render(w io.Writer, report *Report, mode OutputMode) error {
	switch mode {
	case OutputSummary:
		return renderSummary(w, report)
	case OutputDetailed:
		return renderDetailed(w, report)
	case OutputJSON:
		return renderJSON(w, report)
	}
	return fmt.Errorf("unsupported output mode %q", mode)
}

// renderSummary emits a compact human-readable report: run header, count
// breakdowns by drift class and severity, error count. No per-finding rows.
func renderSummary(w io.Writer, r *Report) error {
	b := &strings.Builder{}
	writeHeader(b, r)
	b.WriteString("\n")

	b.WriteString("Findings by drift class:\n")
	for _, k := range sortedKeys(r.FindingsByClass) {
		fmt.Fprintf(b, "  %-44s %4d\n", k, r.FindingsByClass[k])
	}
	if len(r.FindingsByClass) == 0 {
		b.WriteString("  (none)\n")
	}
	b.WriteString("\n")

	b.WriteString("Findings by severity:\n")
	for _, k := range sortedKeys(r.FindingsBySeverity) {
		fmt.Fprintf(b, "  %-20s %4d\n", k, r.FindingsBySeverity[k])
	}
	if len(r.FindingsBySeverity) == 0 {
		b.WriteString("  (none)\n")
	}
	b.WriteString("\n")

	fmt.Fprintf(b, "Orders with zero findings: %d\n", r.SuppressedNoOrphan)
	fmt.Fprintf(b, "Resolver errors:           %d\n", len(r.Errors))
	if len(r.Errors) > 0 {
		errByStage := map[string]int{}
		for _, e := range r.Errors {
			errByStage[e.Stage]++
		}
		for _, k := range sortedKeys(errByStage) {
			fmt.Fprintf(b, "  %-16s %d\n", k, errByStage[k])
		}
	}

	_, err := w.Write([]byte(b.String()))
	return err
}

// renderDetailed emits one block per finding plus the summary header.
func renderDetailed(w io.Writer, r *Report) error {
	b := &strings.Builder{}
	writeHeader(b, r)
	b.WriteString("\n")

	if len(r.Findings) == 0 {
		b.WriteString("No findings.\n")
	}
	for i, f := range r.Findings {
		fmt.Fprintf(b, "Finding #%d  —  %s  (%s)\n", i+1, f.DriftClass, f.Severity)
		if f.OrderID != nil {
			fmt.Fprintf(b, "  Order:           %s\n", f.OrderID.String())
		}
		if f.MidtransOrderID != "" {
			fmt.Fprintf(b, "  Midtrans:        %s\n", f.MidtransOrderID)
		}
		if f.RefundID != nil {
			fmt.Fprintf(b, "  Refund:          %s\n", f.RefundID.String())
		}
		fmt.Fprintf(b, "  Detected at:     %s\n", f.DetectedAt.Format(time.RFC3339))
		fmt.Fprintf(b, "  Idempotency:     %s\n", f.IdempotencyKey)
		if f.GatewayObservedAmount != 0 || f.LocalObservedAmount != 0 {
			fmt.Fprintf(b, "  Gateway amount:  %d\n", f.GatewayObservedAmount)
			fmt.Fprintf(b, "  Local amount:    %d\n", f.LocalObservedAmount)
		}
		if f.Notes != "" {
			fmt.Fprintf(b, "  Notes:           %s\n", f.Notes)
		}
		if f.SuggestedAction != "" {
			fmt.Fprintf(b, "  Suggested:       %s\n", f.SuggestedAction)
		}
		b.WriteString("\n")
	}

	if len(r.Errors) > 0 {
		b.WriteString("Resolver errors:\n")
		for _, e := range r.Errors {
			fmt.Fprintf(b, "  order=%s stage=%s — %s\n", e.OrderID.String(), e.Stage, e.Message)
		}
		b.WriteString("\n")
	}

	_, err := w.Write([]byte(b.String()))
	return err
}

// renderJSON emits the Report struct directly. Map keys are sorted by Go's
// encoding/json (alphabetical), and Findings/Errors are already sorted by
// Orchestrator.finalize — so the byte output is stable across runs over
// identical data.
func renderJSON(w io.Writer, r *Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(r)
}

// writeHeader is shared between summary and detailed modes.
func writeHeader(b *strings.Builder, r *Report) {
	b.WriteString("Gateway Payment Reconciliation — Audit Report\n")
	b.WriteString(strings.Repeat("=", 47) + "\n")
	fmt.Fprintf(b, "Started:                %s\n", r.StartedAt.Format(time.RFC3339))
	fmt.Fprintf(b, "Finished:               %s\n", r.FinishedAt.Format(time.RFC3339))
	fmt.Fprintf(b, "Duration:               %.3fs\n", r.DurationSeconds)
	fmt.Fprintf(b, "Candidates scanned:     %d\n", r.CandidatesScanned)
	fmt.Fprintf(b, "Snapshots resolved:     %d\n", r.SnapshotsResolved)
	fmt.Fprintf(b, "Gateway queries:        %d", r.GatewayQueriesMade)
	if r.GatewayBudget > 0 {
		fmt.Fprintf(b, " (budget %d)", r.GatewayBudget)
	}
	if r.SkipGateway {
		b.WriteString(" [SKIPPED — local-only mode]")
	}
	b.WriteString("\n")
	fmt.Fprintf(b, "Total findings:         %d\n", len(r.Findings))
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}


