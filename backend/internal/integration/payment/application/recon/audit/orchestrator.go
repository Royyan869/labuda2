package audit

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/labuda/backend/internal/integration/payment/application/recon"
)

// Orchestrator runs one audit pass: for each candidate order it resolves a
// Snapshot, calls recon.Classify, and accumulates findings + per-stage
// errors into a Report.
//
// Orchestrator is single-threaded by design. Phase 1B is a validation
// exercise — concurrency goes against "one shot" and risks hitting Midtrans
// quota faster than humans can interpret.
type Orchestrator struct {
	resolver *Resolver
	clock    func() time.Time
}

// NewOrchestrator constructs an Orchestrator. clockFn is injectable for
// deterministic tests; production callers should pass time.Now.UTC.
func NewOrchestrator(resolver *Resolver, clockFn func() time.Time) *Orchestrator {
	if clockFn == nil {
		clockFn = func() time.Time { return time.Now().UTC() }
	}
	return &Orchestrator{resolver: resolver, clock: clockFn}
}

// ResolveError-bearing entries the orchestrator captured, surfaced via
// Report.Errors. Each carries the order it failed on and the resolve stage
// that broke; gateway-stage failures are accompanied by a partial Snapshot
// with Available=false so the local-only drift classes still run.
type RecordedError struct {
	OrderID uuid.UUID `json:"order_id"`
	Stage   string    `json:"stage"`
	Message string    `json:"message"`
}

// Report bundles every input the output formatters need.
type Report struct {
	StartedAt           time.Time        `json:"started_at"`
	FinishedAt          time.Time        `json:"finished_at"`
	DurationSeconds     float64          `json:"duration_seconds"`
	CandidatesScanned   int              `json:"candidates_scanned"`
	SnapshotsResolved   int              `json:"snapshots_resolved"`
	GatewayQueriesMade  int              `json:"gateway_queries_made"`
	GatewayBudget       int              `json:"gateway_budget"`
	SkipGateway         bool             `json:"skip_gateway"`
	Findings            []recon.Finding  `json:"findings"`
	Errors              []RecordedError  `json:"errors"`
	FindingsByClass     map[string]int   `json:"findings_by_class"`
	FindingsBySeverity  map[string]int   `json:"findings_by_severity"`
	SuppressedNoOrphan  int              `json:"orders_clean_no_findings"`
	Thresholds          recon.Thresholds `json:"thresholds"`
}

// RunSpec configures one audit run.
type RunSpec struct {
	// OrderIDs, if non-empty, fixes the candidate set. Otherwise the
	// orchestrator calls Resolver.CandidateScan(Since, Limit).
	OrderIDs []uuid.UUID

	Since time.Time // candidate scan window lower bound (inclusive)
	Limit int       // candidate scan upper bound; ignored if OrderIDs set
}

// Run executes one audit pass and returns the assembled Report. It does NOT
// emit alerts, write to outbox, or persist findings — those concerns belong
// to a future worker, not Phase 1B.
//
// Run is cancel-safe: when ctx is cancelled mid-iteration the partial Report
// is still returned with whatever findings were already collected.
func (o *Orchestrator) Run(ctx context.Context, spec RunSpec) (*Report, error) {
	report := &Report{
		StartedAt:          o.clock(),
		Findings:           make([]recon.Finding, 0),
		Errors:             make([]RecordedError, 0),
		FindingsByClass:    map[string]int{},
		FindingsBySeverity: map[string]int{},
		GatewayBudget:      o.resolver.gatewayBudget,
		SkipGateway:        o.resolver.skipGateway,
		Thresholds:         o.resolver.thresholds,
	}

	candidates := spec.OrderIDs
	if len(candidates) == 0 {
		scanned, err := o.resolver.CandidateScan(ctx, spec.Since, spec.Limit)
		if err != nil {
			return o.finalize(report), err
		}
		candidates = scanned
	}
	report.CandidatesScanned = len(candidates)

	for _, oid := range candidates {
		if ctx.Err() != nil {
			break
		}

		snap, err := o.resolver.ResolveOrder(ctx, oid, o.clock())
		if err != nil {
			var rerr *ResolveError
			if errors.As(err, &rerr) {
				report.Errors = append(report.Errors, RecordedError{
					OrderID: rerr.OrderID,
					Stage:   rerr.Stage,
					Message: rerr.Err.Error(),
				})
				// Gateway-stage errors yield a partial snapshot (Available=
				// false) and the resolver still returned it — fall through
				// to classify local-only drift. Order-stage errors mean we
				// have no Snapshot.Order, so skip.
				if rerr.Stage == "order" {
					continue
				}
			} else {
				report.Errors = append(report.Errors, RecordedError{
					OrderID: oid,
					Stage:   "unknown",
					Message: err.Error(),
				})
				continue
			}
		}

		report.SnapshotsResolved++
		findings := recon.Classify(snap)
		if len(findings) == 0 {
			report.SuppressedNoOrphan++
			continue
		}
		report.Findings = append(report.Findings, findings...)
		for _, f := range findings {
			report.FindingsByClass[string(f.DriftClass)]++
			report.FindingsBySeverity[string(f.Severity)]++
		}
	}

	report.GatewayQueriesMade = o.resolver.GatewayCallsUsed()
	return o.finalize(report), nil
}

// finalize stamps end-of-run metadata and canonicalises ordering so the
// JSON output is byte-stable across runs over identical data.
func (o *Orchestrator) finalize(report *Report) *Report {
	report.FinishedAt = o.clock()
	report.DurationSeconds = report.FinishedAt.Sub(report.StartedAt).Seconds()

	sort.SliceStable(report.Findings, func(i, j int) bool {
		if report.Findings[i].DriftClass != report.Findings[j].DriftClass {
			return report.Findings[i].DriftClass < report.Findings[j].DriftClass
		}
		return report.Findings[i].IdempotencyKey < report.Findings[j].IdempotencyKey
	})
	sort.SliceStable(report.Errors, func(i, j int) bool {
		if report.Errors[i].OrderID != report.Errors[j].OrderID {
			return report.Errors[i].OrderID.String() < report.Errors[j].OrderID.String()
		}
		return report.Errors[i].Stage < report.Errors[j].Stage
	})
	return report
}


