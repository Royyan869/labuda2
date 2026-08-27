package evaluator

import (
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/discovery/search/entity"
	"github.com/labuda/backend/internal/governance/viewercontext"
)

// PHASE 3B — /search/content evaluator enforcement helper.
//
// EnforceSearchContent is the synchronous, handler-side counterpart to
// the existing fire-and-forget SearchContentShadowRunner. It consumes the
// SAME caller-hydrated ViewerContext + TargetContext + content-preview
// slice, runs the SAME pure EvaluateSearchContent decision per row, and
// translates the result through the SAME AdaptSearchContentDecision
// mapping table. The only difference vs. the shadow runner is that this
// helper RETURNS its filtered output to the handler so the response can
// be restricted (DENY → drop) or coarsened (TOMBSTONE/REDACT → lifecycle
// override) before serialization.
//
// CANONICAL PILOT CONTRACT (Batch 3B):
//
//   - Mode-gated. In SearchContentAdapterModeShadow the helper returns
//     the input slice unchanged and a nil override map; the handler's
//     wire shape is byte-identical to the pre-Batch-3B behavior. Roll-
//     back to shadow is a single env-var flip; no schema rollback, no
//     migration rollback.
//   - Further-restrict only. The helper NEVER recovers rows the legacy
//     SQL excluded (hidden/deleted are physically absent from the input
//     slice per search_repository_impl.go projection). Enforcement is
//     therefore strictly safer than the legacy path — it only narrows
//     what already passed projection.
//   - Pure aside from telemetry. The helper performs no DB reads, no IO
//     beyond the bounded Prometheus counters declared in
//     search_shadow_telemetry.go (enforcement_applied_total).
//   - The handler MUST still dispatch the shadow runner (fire-and-
//     forget) AFTER calling this helper so the existing
//     would_enforce_decision_total / decision_total / divergence_total
//     telemetry continues to fire under both modes. The shadow runner
//     and this helper run two passes of the same pure evaluator over
//     the same row set; the cost is negligible and the dual emission
//     guarantees telemetry continuity if the enforcement flip is rolled
//     back.

// SearchContentEnforcementAction is the bounded label set for the
// enforcement_applied_total counter. Each label corresponds to a
// concrete row-level decision the handler took on behalf of the
// evaluator.
type SearchContentEnforcementAction string

const (
	// SearchContentEnforcementActionNone is the sentinel for "no action
	// taken" — used by callers that want to short-circuit the metric
	// emission. NEVER actually written to the counter.
	SearchContentEnforcementActionNone SearchContentEnforcementAction = ""

	// SearchContentEnforcementActionDrop is emitted once per row that
	// would-include=false caused to be filtered from the response.
	SearchContentEnforcementActionDrop SearchContentEnforcementAction = "drop"

	// SearchContentEnforcementActionLifecycleOverride is emitted once
	// per row whose card lifecycle was coarsened (TOMBSTONE → "removed"
	// or REDACT → "unavailable"). The row is still emitted in the
	// response; only its card.Lifecycle field changes.
	SearchContentEnforcementActionLifecycleOverride SearchContentEnforcementAction = "lifecycle_override"
)

// SearchContentEnforcementResult is the value returned by
// EnforceSearchContent. It carries the post-enforcement slice and a
// per-content-ID lifecycle override map; both are nil-safe for
// shadow-mode callers (Filtered is the input slice unchanged;
// LifecycleOverrides is empty).
type SearchContentEnforcementResult struct {
	// Filtered is the post-enforcement subset of contents in the
	// original order. In shadow mode this is the input slice unchanged
	// (same backing array, no copy).
	Filtered []*entity.ContentPreview

	// LifecycleOverrides maps a ContentPreview.ID → coarsened public
	// lifecycle string ("unavailable" / "removed") for rows the adapter
	// classified as TOMBSTONE or REDACT but Include=true. The handler
	// MUST apply the override to the emitted card's Lifecycle field;
	// rows without a key are emitted with their existing lifecycle.
	LifecycleOverrides map[uuid.UUID]string

	// DroppedCount is the number of rows filtered out (Include=false).
	// Aggregated separately from the metric so handler-side logging /
	// response shaping can use it without re-counting.
	DroppedCount int

	// OverriddenCount is the number of rows whose lifecycle was
	// coarsened. Aggregated for the same reason as DroppedCount.
	OverriddenCount int
}

// EnforceSearchContent runs the synchronous enforcement pass over a
// /search/content candidate slice. See the package docstring above for
// the full contract.
//
// In SearchContentAdapterModeShadow:
//   - Returns Filtered = contents (input slice, unchanged)
//   - Returns LifecycleOverrides = nil
//   - Returns DroppedCount = 0, OverriddenCount = 0
//   - Emits no enforcement_applied_total counters
//
// In SearchContentAdapterModeEnforce:
//   - Runs EvaluateSearchContent + AdaptSearchContentDecision per row.
//   - Drops rows where adapter.Include == false.
//   - Records lifecycle overrides for rows where adapter.LifecycleOverride != nil.
//   - Emits one enforcement_applied_total{action="drop"} per dropped row.
//   - Emits one enforcement_applied_total{action="lifecycle_override"} per overridden row.
//
// Nil-safety: a nil contents slice returns an empty result; a nil vc
// causes every row to be classified as UNKNOWN/input_invalid and
// (per adapter doctrine) FAIL-CLOSED — every row is dropped. This is
// the safe-default behavior; a construction defect in the handler must
// not silently expose unmoderated content.
func EnforceSearchContent(
	mode SearchContentAdapterMode,
	vc *viewercontext.ViewerContext,
	tc *viewercontext.TargetContext,
	contents []*entity.ContentPreview,
) SearchContentEnforcementResult {
	if mode != SearchContentAdapterModeEnforce {
		return SearchContentEnforcementResult{
			Filtered: contents,
		}
	}

	// Enforce path. Allocate output capacity equal to input length — the
	// upper bound on Filtered size.
	out := make([]*entity.ContentPreview, 0, len(contents))
	overrides := make(map[uuid.UUID]string)
	var dropped, overridden int

	metrics := newSearchShadowMetrics()
	for _, c := range contents {
		if c == nil {
			continue
		}
		decision, reason, _, semantic := EvaluateSearchContent(vc, tc, c)
		adapted := AdaptSearchContentDecision(decision, reason, semantic, mode)
		if !adapted.Include {
			dropped++
			metrics.recordEnforcementApplied(SearchContentEnforcementActionDrop)
			continue
		}
		if adapted.LifecycleOverride != nil {
			overrides[c.ID] = *adapted.LifecycleOverride
			overridden++
			metrics.recordEnforcementApplied(SearchContentEnforcementActionLifecycleOverride)
		}
		out = append(out, c)
	}

	// Collapse empty override map to nil so callers can rely on the
	// zero-cost nil-map iteration semantics.
	var overrideMap map[uuid.UUID]string
	if len(overrides) > 0 {
		overrideMap = overrides
	}

	return SearchContentEnforcementResult{
		Filtered:           out,
		LifecycleOverrides: overrideMap,
		DroppedCount:       dropped,
		OverriddenCount:    overridden,
	}
}


