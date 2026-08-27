package evaluator

import (
	"strings"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/labuda/backend/internal/governance/viewercontext"
	feedentity "github.com/labuda/backend/internal/social/feed/entity"
)

// PHASE 3M — /feed evaluator further-restrict enforcement.
//
// EnforceFeed is the synchronous, handler-side counterpart to the
// fire-and-forget FeedShadowRunner. It consumes the SAME caller-
// hydrated `*viewercontext.ViewerContext` + `*viewercontext.TargetContext`
// + raw legacy FeedItem slice, runs the SAME pure EvaluateFeedItem
// decision per row, and returns a filtered slice for the handler to
// serialize. The shadow runner continues to fire AFTER the handler
// writes the response so the existing shadow telemetry remains
// comparable across the shadow→enforce flip (dual emission).
//
// F1-W3A — the evaluator package no longer holds a DB pool, no longer
// executes SQL, and no longer owns hydration. The handler boundary
// (backend/internal/social/feed/delivery/http/feed_viewercontext.go)
// is the sole hydration site; the evaluator reads pre-hydrated inputs
// only.
//
// CANONICAL PILOT CONTRACT (Batch 3M, audit verdict in 3L,
// re-canonicalized in F1-W3A):
//
//   - Mode-gated. In FeedEvaluatorModeShadow the helper returns the
//     input slice unchanged and emits no enforcement counters; the
//     wire shape is byte-identical to the pre-W3A behaviour. Rollback
//     is a single env-var flip (FEED_EVALUATOR_MODE=shadow).
//   - Further-restrict only. The helper NEVER recovers rows the
//     legacy SQL excluded (follow JOIN + status='active' + bidirectional
//     block + F1-W1 is_hidden / deleted_at filters remain SQL
//     authority). Enforcement is strictly safer than the legacy path —
//     it only narrows what already passed projection.
//   - UNKNOWN fail-OPEN. If hydration errors degrade an item to
//     UNKNOWN the item is KEPT and counted under unknown_fail_open;
//     the SQL authority remains the visible answer until overlays
//     recover. This is the inverse of /search/content (fail-CLOSED) —
//     feed is high-traffic and a hydration outage must not blank Home.
//   - TOMBSTONE → "removed", REDACT → "unavailable" lifecycle
//     overrides land in the LifecycleOverrides map (mirror of
//     /search/content's enforcement.LifecycleOverrides). Pre-W3A
//     collapse-to-drop is retired; the handler renders the override on
//     the wire.
//   - The shadow runner MUST still be dispatched fire-and-forget AFTER
//     this helper runs so the existing decision_total /
//     would_enforce_decision_total / divergence_total telemetry
//     continues to fire under both modes. The shadow runner receives
//     the ORIGINAL (pre-filter) item slice and the same hydrated (vc,
//     tc) so divergence metrics stay denominator-comparable.

// FeedEvaluatorMode is the bounded enforce-mode enum. Defaults safely
// to shadow via NormalizeFeedEvaluatorMode (safety default).
type FeedEvaluatorMode string

const (
	// FeedEvaluatorModeShadow is the default operating mode. The
	// enforcement helper short-circuits to identity passthrough and
	// emits no enforcement counters.
	FeedEvaluatorModeShadow FeedEvaluatorMode = "shadow"

	// FeedEvaluatorModeEnforce activates the synchronous further-
	// restrict pass. ALLOW rows pass through; DENY / TOMBSTONE /
	// REDACT rows are dropped or lifecycle-overridden; UNKNOWN rows
	// are kept (fail-open) and counted.
	FeedEvaluatorModeEnforce FeedEvaluatorMode = "enforce"
)

// IsValid reports whether m is a recognized FeedEvaluatorMode.
func (m FeedEvaluatorMode) IsValid() bool {
	switch m {
	case FeedEvaluatorModeShadow, FeedEvaluatorModeEnforce:
		return true
	}
	return false
}

// NormalizeFeedEvaluatorMode parses an environment / config string into
// a canonical FeedEvaluatorMode. Any unrecognized or empty value falls
// safely to FeedEvaluatorModeShadow — enforce mode is opt-in only.
func NormalizeFeedEvaluatorMode(raw string) FeedEvaluatorMode {
	switch FeedEvaluatorMode(strings.ToLower(strings.TrimSpace(raw))) {
	case FeedEvaluatorModeEnforce:
		return FeedEvaluatorModeEnforce
	default:
		return FeedEvaluatorModeShadow
	}
}

// FeedEnforcementAction is the bounded label set for the
// feed_enforcement_applied_total counter. Cardinality is intentionally
// narrow — no UUID, no decision-string, no per-author/per-content
// dimension. The label discipline mirrors the existing shadow
// telemetry (shadow_telemetry.go) and the /search/content
// enforcement_applied_total action vocabulary (search_content_enforce.go).
type FeedEnforcementAction string

const (
	// FeedEnforcementActionDrop is emitted once per row removed from
	// the response by the enforce pass. Under C1 convergence this is
	// reserved for DENY outcomes only — TOMBSTONE / REDACT now
	// coarsen to "lifecycle_override" instead of collapsing to drop.
	FeedEnforcementActionDrop FeedEnforcementAction = "drop"

	// FeedEnforcementActionLifecycleOverride is emitted once per row
	// whose card lifecycle was coarsened (TOMBSTONE → "removed" or
	// REDACT → "unavailable"). The row is still emitted in the
	// response; only its card.Lifecycle (and the top-level lifecycle
	// key) changes.
	FeedEnforcementActionLifecycleOverride FeedEnforcementAction = "lifecycle_override"

	// FeedEnforcementActionUnknownFailOpen is emitted once per row
	// the evaluator classified as UNKNOWN but the helper kept anyway.
	// Counts hydration-degradation incidents per request.
	FeedEnforcementActionUnknownFailOpen FeedEnforcementAction = "unknown_fail_open"
)

// FeedLifecycleRemoved / FeedLifecycleUnavailable are declared in
// feed_adapter.go (authoritative). The override map values are the
// canonical PublicLifecycleState strings so the handler can pipe
// straight to ContentCard.Lifecycle without re-coarsening.

// FeedEnforcementResult is the value returned by EnforceFeed. Shadow-
// mode callers receive Filtered = input slice unchanged, nil
// LifecycleOverrides, and zero counts.
type FeedEnforcementResult struct {
	// Filtered is the post-enforcement subset of items in the original
	// order. In shadow mode this is the input slice unchanged (same
	// backing array, no copy).
	Filtered []*feedentity.FeedItem

	// LifecycleOverrides maps a FeedItem.ID → coarsened public
	// lifecycle string ("unavailable" / "removed") for rows the
	// adapter classified as TOMBSTONE or REDACT but Include=true.
	// The handler MUST apply the override to the emitted
	// ContentCard.Lifecycle (and the top-level lifecycle key) for
	// those rows; rows without a key are emitted with their existing
	// lifecycle. Nil in shadow mode and when no row took the override
	// path.
	LifecycleOverrides map[uuid.UUID]string

	// DroppedCount is the number of items the enforce pass removed
	// (DENY only under C1 convergence; TOMBSTONE / REDACT now count
	// under OverriddenCount instead).
	DroppedCount int

	// OverriddenCount is the number of rows whose lifecycle was
	// coarsened (TOMBSTONE / REDACT outcomes).
	OverriddenCount int

	// UnknownFailOpenCount is the number of items classified UNKNOWN
	// that were kept anyway.
	UnknownFailOpenCount int
}

// feedEnforcementApplied is the bounded Prometheus counter for per-row
// enforce actions. Bounded label set; cardinality discipline matches
// the existing shadow telemetry. NEVER include item id / author id /
// decision text. Action vocabulary: drop | lifecycle_override |
// unknown_fail_open (mirror of search/content's
// enforcement_applied_total). Namespace + name preserved verbatim
// across the F1-W3A rebuild so existing dashboards are unaffected.
var feedEnforcementApplied = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "labuda_evaluator_feed",
	Name:      "enforcement_applied_total",
	Help:      "Per-row enforcement actions taken by the /feed evaluator in enforce mode. Bounded labels: action in {drop, lifecycle_override, unknown_fail_open}. Always zero in shadow mode.",
}, []string{"action"})

// C1 — Feed evaluator promotion-prerequisite counters, mirroring the
// /search/content adapter telemetry pair (search_shadow_telemetry.go).
// Both keep cardinality bounded; values come from the small
// FeedEvaluatorMode + FeedDecisionReason enumerations declared above.
// Preserved verbatim across the F1-W3A rebuild.
var (
	feedEvaluatorWouldEnforceDecisionTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "labuda_evaluator_feed",
		Name:      "would_enforce_decision_total",
		Help:      "Per-row count of how the adapter WOULD classify the /feed decision if running in enforce mode. Emitted unconditionally in shadow mode for promotion safety telemetry (C1 convergence). Labels are bounded to the FeedDecisionReason enum.",
	}, []string{"adapter_reason"})

	feedEvaluatorEnforceModeTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "labuda_evaluator_feed",
		Name:      "enforce_mode_total",
		Help:      "Per-request count of the operating mode (shadow|enforce) of the /feed evaluator integration. Used to correlate would_enforce_decision_total rates with the route's current operating mode (C1 convergence).",
	}, []string{"mode"})
)

// recordFeedWouldEnforceDecision emits the per-row adapter
// classification (would_enforce_decision_total) for feed. Called from
// the shadow runner once per evaluated row alongside the existing
// decision_total emission. The dominant ALLOW pass-through is
// intentionally not counted; the agreement cell is captured by
// labuda_evaluator_shadow_divergence_total.
func recordFeedWouldEnforceDecision(reason FeedDecisionReason) {
	if reason == FeedDecisionReasonNone {
		return
	}
	feedEvaluatorWouldEnforceDecisionTotal.WithLabelValues(string(reason)).Inc()
}

// recordFeedEnforceMode emits enforce_mode_total once per shadow run
// with the configured operating-mode label.
func recordFeedEnforceMode(mode FeedEvaluatorMode) {
	if !mode.IsValid() {
		mode = FeedEvaluatorModeShadow
	}
	feedEvaluatorEnforceModeTotal.WithLabelValues(string(mode)).Inc()
}

// EnforceFeed runs the synchronous further-restrict pass over a /feed
// page slice. See the package docstring above for the full contract.
//
// F1-W3A — canonical signature. Consumes pre-hydrated
// `*viewercontext.ViewerContext` (carrying viewer lifecycle + bidirectional
// block overlay) and `*viewercontext.TargetContext` (carrying per-row
// author lifecycle + per-row content moderation). No DB access, no
// IO, no pool reference.
//
// In FeedEvaluatorModeShadow:
//   - Returns Filtered = items (input slice, unchanged),
//     LifecycleOverrides = nil, and zero counts.
//   - Emits no feed_enforcement_applied_total counters.
//
// In FeedEvaluatorModeEnforce:
//   - Runs EvaluateFeedItem + AdaptFeedDecision per row.
//   - ALLOW       → keep.
//   - DENY        → drop, emit action="drop".
//   - TOMBSTONE   → keep + override lifecycle="removed",
//                   emit action="lifecycle_override".
//   - REDACT      → keep + override lifecycle="unavailable",
//                   emit action="lifecycle_override".
//   - UNKNOWN     → keep (fail-open), emit action="unknown_fail_open".
//
// Nil-safety: a nil items slice returns an empty result. A nil vc or
// tc causes every row to be classified UNKNOWN/input_invalid by
// EvaluateFeedItem; the fail-open policy then keeps every row — the
// safe-default that preserves the legacy SQL answer if the caller
// failed to construct the viewer/target context.
func EnforceFeed(
	mode FeedEvaluatorMode,
	vc *viewercontext.ViewerContext,
	tc *viewercontext.TargetContext,
	items []*feedentity.FeedItem,
) FeedEnforcementResult {
	if mode != FeedEvaluatorModeEnforce {
		return FeedEnforcementResult{Filtered: items}
	}
	if len(items) == 0 {
		return FeedEnforcementResult{Filtered: items}
	}

	out := make([]*feedentity.FeedItem, 0, len(items))
	overrides := make(map[uuid.UUID]string)
	var dropped, overridden, unknownKept int
	for _, item := range items {
		if item == nil {
			continue
		}
		decision, reason := EvaluateFeedItem(vc, tc, item)
		adapted := AdaptFeedDecision(decision, reason, mode)
		if !adapted.Include {
			dropped++
			feedEnforcementApplied.WithLabelValues(string(FeedEnforcementActionDrop)).Inc()
			continue
		}
		if adapted.LifecycleOverride != nil {
			overrides[item.ID] = *adapted.LifecycleOverride
			overridden++
			feedEnforcementApplied.WithLabelValues(string(FeedEnforcementActionLifecycleOverride)).Inc()
			out = append(out, item)
			continue
		}
		if adapted.Reason == FeedDecisionReasonUnknownFailOpen {
			unknownKept++
			feedEnforcementApplied.WithLabelValues(string(FeedEnforcementActionUnknownFailOpen)).Inc()
		}
		out = append(out, item)
	}

	var overrideMap map[uuid.UUID]string
	if len(overrides) > 0 {
		overrideMap = overrides
	}

	return FeedEnforcementResult{
		Filtered:             out,
		LifecycleOverrides:   overrideMap,
		DroppedCount:         dropped,
		OverriddenCount:      overridden,
		UnknownFailOpenCount: unknownKept,
	}
}

// WithMode returns a clone of r with the given operating mode applied.
// Invalid input is silently coerced to FeedEvaluatorModeShadow per the
// NormalizeFeedEvaluatorMode safety contract. Safe to call on a nil
// receiver (returns nil) — when the runner is disabled, mode is moot.
func (r *FeedShadowRunner) WithMode(mode FeedEvaluatorMode) *FeedShadowRunner {
	if r == nil {
		return nil
	}
	clone := *r
	if !mode.IsValid() {
		clone.mode = FeedEvaluatorModeShadow
	} else {
		clone.mode = mode
	}
	return &clone
}

// Mode returns the runner's currently-configured operating mode. A nil
// receiver returns FeedEvaluatorModeShadow (defensive — a disabled
// runner is treated as shadow, never enforce).
func (r *FeedShadowRunner) Mode() FeedEvaluatorMode {
	if r == nil {
		return FeedEvaluatorModeShadow
	}
	if !r.mode.IsValid() {
		return FeedEvaluatorModeShadow
	}
	return r.mode
}


