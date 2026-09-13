package evaluator

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/labuda/backend/internal/governance/viewercontext"
	contententity "github.com/labuda/backend/internal/social/content/entity"
)

// F1-W3B — /contents/:id evaluator fail-CLOSED enforcement helper,
// canonical signature.
//
// EnforceContentDetail is the synchronous, handler-side counterpart to
// the fire-and-forget ContentDetailShadowRunner. It consumes the same
// pre-hydrated *viewercontext.ViewerContext + *viewercontext.TargetContext
// + *contententity.Content the handler builds at the boundary, runs the
// SAME pure EvaluateContentDetail decision, translates the result
// through AdaptContentDetailDecision, and returns the allow/deny verdict
// to the handler so non-allow can be converted to HTTP 404 BEFORE
// response serialization.
//
// CANONICAL CONTRACT (D1 / F1-W3B):
//
//   - Unconditional enforcement. There is no mode parameter: the helper
//     always runs the fail-CLOSED pass.
//   - Further-restrict only. The helper NEVER allows a row the legacy
//     handler would have 404'd — the legacy gate runs first; this helper
//     only converts a legacy-200 into a 404 when the evaluator decision
//     disagrees in enforce mode.
//   - Fail-CLOSED on UNKNOWN — doctrine §8.5. The handler emits 404 when
//     overlays did not hydrate.
//   - The shadow runner's async dispatch remains in place AFTER the
//     handler writes the response as pure observability, so the existing
//     shadow_* telemetry continues to fire. The shadow runner and this
//     helper run two passes of the same pure evaluator over the same row;
//     the cost is negligible.
//   - F1-W3B — neither the helper nor the runner touches the DB. Both
//     consume pre-hydrated canonical overlays from the handler boundary.

// ContentDetailEnforcementAction is the bounded label set for the
// content_detail_enforcement_applied_total counter. Cardinality
// discipline mirrors the feed / search enforcement counters.
type ContentDetailEnforcementAction string

const (
	// ContentDetailEnforcementActionDeny404 is emitted once per request the
	// enforce pass converted from a legacy-200 to a 404 because the
	// evaluator returned DENY / TOMBSTONE / REDACT (block, viewer-side
	// lifecycle, owner-side lifecycle, hidden, deleted).
	ContentDetailEnforcementActionDeny404 ContentDetailEnforcementAction = "deny_404"

	// ContentDetailEnforcementActionUnknownFailClosed404 is emitted once
	// per request the enforce pass converted from a legacy-200 to a 404
	// because the evaluator returned UNKNOWN (overlay hydration failure /
	// input-invalid). Distinct from deny_404 so ops can monitor the
	// hydration-degradation rate separately from intentional denies.
	ContentDetailEnforcementActionUnknownFailClosed404 ContentDetailEnforcementAction = "unknown_fail_closed_404"
)

// ContentDetailEnforcementResult is the value returned by
// EnforceContentDetail.
type ContentDetailEnforcementResult struct {
	// Allow reports whether the handler should write its successful
	// response. Allow is the adapter's Include flag.
	Allow bool

	// Reason is the bounded telemetry-safe label carried back to the
	// handler so it can decide which 404 reason to emit (or no-op when
	// Allow=true).
	Reason ContentDetailDecisionReason

	// ShadowDecision passes through the raw evaluator decision for the
	// handler's structured logs.
	ShadowDecision ShadowDecision
}

// EnforceContentDetail runs the synchronous enforcement pass on one
// /contents/:id request. See the package docstring above for the full
// contract.
//
// F1-W3B — canonical signature consuming pre-hydrated
// *viewercontext.ViewerContext + *viewercontext.TargetContext +
// *contententity.Content. No DB access, no IO, no pool reference.
//
// Enforcement is unconditional:
//   - Runs EvaluateContentDetail + AdaptContentDetailDecision.
//   - Returns Allow=false on every non-ALLOW decision.
//   - Emits one content_detail_enforcement_applied_total{action} per
//     non-allow outcome.
//
// Nil-safety: a nil vc / tc / content causes EvaluateContentDetail to
// classify the request as UNKNOWN/input_invalid. Under fail-CLOSED
// doctrine this becomes Allow=false with reason=unknown_fail_closed.
// The safe-default behaviour prevents a construction defect from
// silently exposing unmoderated content.
func EnforceContentDetail(
	vc *viewercontext.ViewerContext,
	tc *viewercontext.TargetContext,
	content *contententity.Content,
) ContentDetailEnforcementResult {
	decision, reason := EvaluateContentDetail(vc, tc, content)
	adapted := AdaptContentDetailDecision(decision, reason)
	if !adapted.Include {
		switch adapted.Reason {
		case ContentDetailDecisionReasonDeny:
			contentDetailEnforcementApplied.
				WithLabelValues(string(ContentDetailEnforcementActionDeny404)).Inc()
		case ContentDetailDecisionReasonUnknownFailClosed:
			contentDetailEnforcementApplied.
				WithLabelValues(string(ContentDetailEnforcementActionUnknownFailClosed404)).Inc()
		}
	}
	return ContentDetailEnforcementResult{
		Allow:          adapted.Include,
		Reason:         adapted.Reason,
		ShadowDecision: adapted.ShadowDecision,
	}
}

// D1 — Bounded telemetry counters. Mirror of the
// labuda_evaluator_feed_* trio. Namespace + name + label sets preserved
// VERBATIM across the F1-W3B rebuild so existing dashboards are
// unaffected.

var (
	contentDetailEnforcementApplied = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "labuda_evaluator_content_detail",
		Name:      "enforcement_applied_total",
		Help:      "Per-request enforcement actions taken by the /contents/:id evaluator. Bounded labels: action in {deny_404, unknown_fail_closed_404}.",
	}, []string{"action"})

	contentDetailWouldEnforceDecisionTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "labuda_evaluator_content_detail",
		Name:      "would_enforce_decision_total",
		Help:      "Per-request adapter classification emitted by the /contents/:id observability runner. Labels are bounded to the ContentDetailDecisionReason enum.",
	}, []string{"adapter_reason"})

	contentDetailEnforceModeTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "labuda_evaluator_content_detail",
		Name:      "enforce_mode_total",
		Help:      "Per-request count of the /contents/:id evaluator integration. Always labeled mode=enforce.",
	}, []string{"mode"})
)

// recordContentDetailWouldEnforceDecision emits the per-request adapter
// classification. The dominant ALLOW pass-through is intentionally not
// counted; the agreement cell is captured by
// labuda_evaluator_shadow_divergence_total{surface="content_detail"}.
func recordContentDetailWouldEnforceDecision(reason ContentDetailDecisionReason) {
	if reason == ContentDetailDecisionReasonNone {
		return
	}
	contentDetailWouldEnforceDecisionTotal.WithLabelValues(string(reason)).Inc()
}

// recordContentDetailEnforceMode emits enforce_mode_total once per
// observability run.
func recordContentDetailEnforceMode() {
	contentDetailEnforceModeTotal.WithLabelValues("enforce").Inc()
}


