package evaluator_test

import (
	"testing"

	"github.com/labuda/backend/internal/governance/evaluator"
)

// PHASE 3A — adapter mapping tests. These tests pin the
// AdaptSearchContentDecision mapping table so the future Batch 3B
// enforcement wiring can rely on it. NONE of these tests exercise IO,
// DB state, ViewerContext hydration, or response shape — the adapter is
// strictly pure and operates on the canonical evaluator return tuple.

func ptr(s string) *string { return &s }

// TestAdapter_AllowPassesThroughClean asserts the ALLOW happy path:
// Include=true, no lifecycle override, empty Reason.
func TestAdapter_AllowPassesThroughClean(t *testing.T) {
	got := evaluator.AdaptSearchContentDecision(
		evaluator.ShadowDecisionAllow,
		evaluator.SearchUnknownReasonNone,
		evaluator.SearchExposureSemanticAllow,
		evaluator.SearchContentAdapterModeShadow,
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
// excluded row with no lifecycle override — the row would be dropped
// from the response in enforce mode.
func TestAdapter_DenyExcludesNoOverride(t *testing.T) {
	got := evaluator.AdaptSearchContentDecision(
		evaluator.ShadowDecisionDeny,
		evaluator.SearchUnknownReasonNone,
		evaluator.SearchExposureSemanticUnknownShadowOnly,
		evaluator.SearchContentAdapterModeShadow,
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
		evaluator.SearchContentAdapterModeShadow,
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
		evaluator.SearchContentAdapterModeShadow,
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
// results once Batch 3B flips enforce mode on.
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
				evaluator.SearchContentAdapterModeShadow,
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
// or nil row from the handler) is treated as fail-CLOSED — the row
// would be excluded in enforce mode. Nil ViewerContext is a handler
// construction defect, NOT a hydration race; promoting authority means
// surfacing those defects, not silently allowing them past.
//
// This pairs with viewer-context-contract.md §8.1 — the caller is
// responsible for constructing ViewerContext; nil at the evaluator
// boundary indicates a construction bug.
func TestAdapter_UnknownInputInvalidFailsClosed(t *testing.T) {
	got := evaluator.AdaptSearchContentDecision(
		evaluator.ShadowDecisionUnknown,
		evaluator.SearchUnknownReasonInputInvalid,
		evaluator.SearchExposureSemanticUnknownShadowOnly,
		evaluator.SearchContentAdapterModeShadow,
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

// TestAdapter_ModeIsPassive asserts the canonical contract that mode
// NEVER changes the mapping. The adapter's job is to produce a single
// decision; the caller decides whether to ACT on it based on mode.
//
// Failing this test would mean an unsafe mode-conditional path slipped
// into the adapter — the kind of drift that makes promotion unsafe
// because the same input could yield different decisions in shadow vs
// enforce mode.
func TestAdapter_ModeIsPassive(t *testing.T) {
	inputs := []struct {
		name     string
		decision evaluator.ShadowDecision
		reason   evaluator.SearchUnknownReason
	}{
		{"allow", evaluator.ShadowDecisionAllow, evaluator.SearchUnknownReasonNone},
		{"deny", evaluator.ShadowDecisionDeny, evaluator.SearchUnknownReasonNone},
		{"tombstone", evaluator.ShadowDecisionTombstone, evaluator.SearchUnknownReasonNone},
		{"redact", evaluator.ShadowDecisionRedact, evaluator.SearchUnknownReasonNone},
		{"unknown_overlay", evaluator.ShadowDecisionUnknown, evaluator.SearchUnknownReasonViewerOverlayMissing},
		{"unknown_invalid", evaluator.ShadowDecisionUnknown, evaluator.SearchUnknownReasonInputInvalid},
	}
	for _, in := range inputs {
		t.Run(in.name, func(t *testing.T) {
			shadow := evaluator.AdaptSearchContentDecision(in.decision, in.reason, evaluator.SearchExposureSemanticUnknownShadowOnly, evaluator.SearchContentAdapterModeShadow)
			enforce := evaluator.AdaptSearchContentDecision(in.decision, in.reason, evaluator.SearchExposureSemanticUnknownShadowOnly, evaluator.SearchContentAdapterModeEnforce)
			if shadow.Include != enforce.Include {
				t.Errorf("%s: Include differs across modes (shadow=%v enforce=%v); mapping MUST be mode-independent",
					in.name, shadow.Include, enforce.Include)
			}
			if !equalLifecycleOverride(shadow.LifecycleOverride, enforce.LifecycleOverride) {
				t.Errorf("%s: LifecycleOverride differs across modes (shadow=%v enforce=%v); mapping MUST be mode-independent",
					in.name, shadow.LifecycleOverride, enforce.LifecycleOverride)
			}
			if shadow.Reason != enforce.Reason {
				t.Errorf("%s: Reason differs across modes (shadow=%q enforce=%q); mapping MUST be mode-independent",
					in.name, shadow.Reason, enforce.Reason)
			}
		})
	}
}

func equalLifecycleOverride(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
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
		evaluator.SearchContentAdapterModeShadow,
	)
	failOpen := evaluator.AdaptSearchContentDecision(
		evaluator.ShadowDecisionUnknown,
		evaluator.SearchUnknownReasonViewerOverlayMissing,
		evaluator.SearchExposureSemanticUnknownShadowOnly,
		evaluator.SearchContentAdapterModeShadow,
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

// TestNormalizeSearchContentAdapterMode asserts the safety-default
// contract: any unrecognized / empty env value MUST resolve to shadow
// mode. Enforce mode is opt-in only; a typo or missing env var must
// never trigger enforcement.
func TestNormalizeSearchContentAdapterMode(t *testing.T) {
	cases := []struct {
		in   string
		want evaluator.SearchContentAdapterMode
	}{
		{"", evaluator.SearchContentAdapterModeShadow},
		{"shadow", evaluator.SearchContentAdapterModeShadow},
		{"enforce", evaluator.SearchContentAdapterModeEnforce},
		{"SHADOW", evaluator.SearchContentAdapterModeShadow}, // case-sensitive — caller must lowercase before passing
		{"production", evaluator.SearchContentAdapterModeShadow},
		{"ENFORCE", evaluator.SearchContentAdapterModeShadow},
		{"true", evaluator.SearchContentAdapterModeShadow},
	}
	for _, c := range cases {
		if got := evaluator.NormalizeSearchContentAdapterMode(c.in); got != c.want {
			t.Errorf("Normalize(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

// TestSearchContentAdapterModeIsValid asserts the IsValid predicate
// recognizes ONLY the two canonical modes.
func TestSearchContentAdapterModeIsValid(t *testing.T) {
	if !evaluator.SearchContentAdapterModeShadow.IsValid() {
		t.Errorf("shadow.IsValid() = false; want true")
	}
	if !evaluator.SearchContentAdapterModeEnforce.IsValid() {
		t.Errorf("enforce.IsValid() = false; want true")
	}
	if (evaluator.SearchContentAdapterMode("")).IsValid() {
		t.Errorf("empty.IsValid() = true; want false")
	}
	if (evaluator.SearchContentAdapterMode("ENFORCE")).IsValid() {
		t.Errorf("uppercase.IsValid() = true; want false")
	}
}


