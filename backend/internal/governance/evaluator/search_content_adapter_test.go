package evaluator_test

import (
	"testing"

	"github.com/labuda/backend/internal/governance/evaluator"
)

// PHASE 3A — adapter mapping tests. These tests pin the
// AdaptSearchContentDecision mapping table. NONE of these tests exercise
// IO, DB state, ViewerContext hydration, or response shape — the adapter
// is strictly pure and operates on the canonical evaluator return tuple,
// and its mapping is unconditional (no mode parameter).

func ptr(s string) *string { return &s }

// TestAdapter_AllowPassesThroughClean asserts the ALLOW happy path:
// Include=true, no lifecycle override, empty Reason.
func TestAdapter_AllowPassesThroughClean(t *testing.T) {
	got := evaluator.AdaptSearchContentDecision(
		evaluator.ShadowDecisionAllow,
		evaluator.SearchUnknownReasonNone,
		evaluator.SearchExposureSemanticAllow,
	)
	if !got.Include {
		t.Errorf("ALLOW: Include = false; want true")
	}
	if got.LifecycleOverride != nil {
		t.Errorf("ALLOW: LifecycleOverride = %v; want nil", got.LifecycleOverride)
	}
	if got.Reason != evaluator.SearchContentDecisionReasonNone {
		t.Errorf("ALLOW: Reason = %q; want empty", got.Reason)
	}
	if got.ShadowDecision != evaluator.ShadowDecisionAllow {
		t.Errorf("ALLOW: ShadowDecision = %q; want allow", got.ShadowDecision)
	}
}

// TestAdapter_DenyExcludesNoOverride asserts that DENY produces an
// excluded row with no lifecycle override — the row is dropped from the
// response.
func TestAdapter_DenyExcludesNoOverride(t *testing.T) {
	got := evaluator.AdaptSearchContentDecision(
		evaluator.ShadowDecisionDeny,
		evaluator.SearchUnknownReasonNone,
		evaluator.SearchExposureSemanticUnknownShadowOnly,
	)
	if got.Include {
		t.Errorf("DENY: Include = true; want false")
	}
	if got.LifecycleOverride != nil {
		t.Errorf("DENY: LifecycleOverride = %v; want nil", got.LifecycleOverride)
	}
	if got.Reason != evaluator.SearchContentDecisionReasonDeny {
		t.Errorf("DENY: Reason = %q; want deny", got.Reason)
	}
}

// TestAdapter_TombstoneRemovedLifecycle asserts TOMBSTONE preserves
// Include=true but coarsens the card lifecycle to "removed".
func TestAdapter_TombstoneRemovedLifecycle(t *testing.T) {
	got := evaluator.AdaptSearchContentDecision(
		evaluator.ShadowDecisionTombstone,
		evaluator.SearchUnknownReasonNone,
		evaluator.SearchExposureSemanticUnknownShadowOnly,
	)
	if !got.Include {
		t.Errorf("TOMBSTONE: Include = false; want true (degraded card path)")
	}
	if got.LifecycleOverride == nil || *got.LifecycleOverride != evaluator.SearchContentLifecycleRemoved {
		t.Errorf("TOMBSTONE: LifecycleOverride = %v; want %q", got.LifecycleOverride, evaluator.SearchContentLifecycleRemoved)
	}
	if got.Reason != evaluator.SearchContentDecisionReasonTombstone {
		t.Errorf("TOMBSTONE: Reason = %q; want tombstone", got.Reason)
	}
}

// TestAdapter_RedactUnavailableLifecycle asserts REDACT preserves
// Include=true but coarsens the card lifecycle to "unavailable".
func TestAdapter_RedactUnavailableLifecycle(t *testing.T) {
	got := evaluator.AdaptSearchContentDecision(
		evaluator.ShadowDecisionRedact,
		evaluator.SearchUnknownReasonNone,
		evaluator.SearchExposureSemanticUnknownShadowOnly,
	)
	if !got.Include {
		t.Errorf("REDACT: Include = false; want true")
	}
	if got.LifecycleOverride == nil || *got.LifecycleOverride != evaluator.SearchContentLifecycleUnavailable {
		t.Errorf("REDACT: LifecycleOverride = %v; want %q", got.LifecycleOverride, evaluator.SearchContentLifecycleUnavailable)
	}
	if got.Reason != evaluator.SearchContentDecisionReasonRedact {
		t.Errorf("REDACT: Reason = %q; want redact", got.Reason)
	}
}

// TestAdapter_UnknownOverlayMissingFailsOpen asserts the audit doctrine:
// overlay-missing UNKNOWN is NOT proof of denial. The legacy authority
// is preserved (Include=true) and a bounded telemetry reason is emitted.
//
// This is the "incomplete overlay ≠ proof of denial" rule from the
// Batch 3 audit. Without this fail-open behavior, a transient overlay
// hydration error would silently exclude legitimate items from search
// results.
func TestAdapter_UnknownOverlayMissingFailsOpen(t *testing.T) {
	cases := []struct {
		name   string
		reason evaluator.SearchUnknownReason
	}{
		{"viewer_overlay_missing", evaluator.SearchUnknownReasonViewerOverlayMissing},
		{"target_overlay_missing", evaluator.SearchUnknownReasonTargetOverlayMissing},
		{"hydration_error", evaluator.SearchUnknownReasonHydrationError},
		{"candidate_set_incomplete", evaluator.SearchUnknownReasonCandidateSetIncomplete},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := evaluator.AdaptSearchContentDecision(
				evaluator.ShadowDecisionUnknown,
				tc.reason,
				evaluator.SearchExposureSemanticUnknownShadowOnly,
			)
			if !got.Include {
				t.Errorf("%s: Include = false; want true (fail-open doctrine)", tc.name)
			}
			if got.LifecycleOverride != nil {
				t.Errorf("%s: LifecycleOverride = %v; want nil", tc.name, got.LifecycleOverride)
			}
			if got.Reason != evaluator.SearchContentDecisionReasonUnknownFailOpen {
				t.Errorf("%s: Reason = %q; want unknown_fail_open", tc.name, got.Reason)
			}
		})
	}
}

// TestAdapter_UnknownInputInvalidFailsClosed asserts that an UNKNOWN
// outcome caused by an input-invalid classification (nil ViewerContext
// or nil row from the handler) is treated as fail-CLOSED — the row is
// excluded. Nil ViewerContext is a handler construction defect, NOT a
// hydration race.
//
// This pairs with viewer-context-contract.md §8.1 — the caller is
// responsible for constructing ViewerContext; nil at the evaluator
// boundary indicates a construction bug.
func TestAdapter_UnknownInputInvalidFailsClosed(t *testing.T) {
	got := evaluator.AdaptSearchContentDecision(
		evaluator.ShadowDecisionUnknown,
		evaluator.SearchUnknownReasonInputInvalid,
		evaluator.SearchExposureSemanticUnknownShadowOnly,
	)
	if got.Include {
		t.Errorf("UNKNOWN/input_invalid: Include = true; want false (fail-closed doctrine)")
	}
	if got.LifecycleOverride != nil {
		t.Errorf("UNKNOWN/input_invalid: LifecycleOverride = %v; want nil", got.LifecycleOverride)
	}
	if got.Reason != evaluator.SearchContentDecisionReasonUnknownFailClosed {
		t.Errorf("UNKNOWN/input_invalid: Reason = %q; want unknown_fail_closed", got.Reason)
	}
}

// TestAdapter_ShadowMetricsAreNotProofOfSafety encodes the Batch 3 audit
// finding "Undefined Denominator Rule" (docs/05-rollout/search-shadow-
// seam-architecture.md §3.1). The shadow runner observes only legacy-
// allowed rows; UNKNOWN outcomes are NOT evidence that the legacy
// filter is sound for rows the legacy excluded.
//
// The adapter pin: an UNKNOWN-with-overlay-missing reason is classified
// as `unknown_fail_open` — a distinct, bounded reason — NOT silently
// folded into "allow". Downstream telemetry can therefore distinguish:
//
//   - true ALLOW (evaluator decided)            → Reason=""
//   - fail-open by choice (overlay missing)     → Reason="unknown_fail_open"
//   - fail-closed by choice (input invalid)     → Reason="unknown_fail_closed"
//
// If a future PR collapses unknown_fail_open into "allow" silently,
// this test will fail — preventing the regression of treating
// projection / hydration absence as proof of evaluator agreement.
func TestAdapter_ShadowMetricsAreNotProofOfSafety(t *testing.T) {
	allow := evaluator.AdaptSearchContentDecision(
		evaluator.ShadowDecisionAllow,
		evaluator.SearchUnknownReasonNone,
		evaluator.SearchExposureSemanticAllow,
	)
	failOpen := evaluator.AdaptSearchContentDecision(
		evaluator.ShadowDecisionUnknown,
		evaluator.SearchUnknownReasonViewerOverlayMissing,
		evaluator.SearchExposureSemanticUnknownShadowOnly,
	)

	// Both Include=true today, but the Reason MUST distinguish them so
	// downstream telemetry can compute an honest agreement rate
	// excluding fail-open noise.
	if !allow.Include || !failOpen.Include {
		t.Fatalf("expected both Include=true; got allow=%v failOpen=%v", allow.Include, failOpen.Include)
	}
	if allow.Reason == failOpen.Reason {
		t.Errorf("ALLOW and fail-open Reasons must differ (both = %q); "+
			"silent collapse would let projection/hydration absence "+
			"masquerade as evaluator agreement (Undefined Denominator Rule).",
			allow.Reason)
	}
	if allow.Reason != evaluator.SearchContentDecisionReasonNone {
		t.Errorf("ALLOW Reason = %q; want empty", allow.Reason)
	}
	if failOpen.Reason != evaluator.SearchContentDecisionReasonUnknownFailOpen {
		t.Errorf("fail-open Reason = %q; want unknown_fail_open", failOpen.Reason)
	}
}


