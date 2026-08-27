package evaluator

import "testing"

// TestAdaptContentDetailDecision_Mapping exercises the canonical mapping
// table for the D1 detail adapter convergence. Mirrors AdaptFeedDecision /
// AdaptSearchContentDecision but with the documented doctrine inversion —
// detail surface fail-CLOSED on UNKNOWN and collapse-to-404 on TOMBSTONE/
// REDACT (no in-wire lifecycle override).
func TestAdaptContentDetailDecision_Mapping(t *testing.T) {
	cases := []struct {
		name        string
		decision    ShadowDecision
		reason      UnknownReason
		wantInclude bool
		wantReason  ContentDetailDecisionReason
	}{
		{
			name:        "allow",
			decision:    ShadowDecisionAllow,
			wantInclude: true,
			wantReason:  ContentDetailDecisionReasonNone,
		},
		{
			name:        "deny_collapses_to_404",
			decision:    ShadowDecisionDeny,
			wantInclude: false,
			wantReason:  ContentDetailDecisionReasonDeny,
		},
		{
			name:        "tombstone_collapses_to_404",
			decision:    ShadowDecisionTombstone,
			wantInclude: false,
			wantReason:  ContentDetailDecisionReasonDeny,
		},
		{
			name:        "redact_collapses_to_404",
			decision:    ShadowDecisionRedact,
			wantInclude: false,
			wantReason:  ContentDetailDecisionReasonDeny,
		},
		{
			name:        "unknown_input_invalid_fails_closed",
			decision:    ShadowDecisionUnknown,
			reason:      UnknownReasonInputInvalid,
			wantInclude: false,
			wantReason:  ContentDetailDecisionReasonUnknownFailClosed,
		},
		{
			name:        "unknown_viewer_overlay_missing_fails_closed",
			decision:    ShadowDecisionUnknown,
			reason:      UnknownReasonViewerOverlayMissing,
			wantInclude: false,
			wantReason:  ContentDetailDecisionReasonUnknownFailClosed,
		},
		{
			name:        "unknown_target_overlay_missing_fails_closed",
			decision:    ShadowDecisionUnknown,
			reason:      UnknownReasonTargetOverlayMissing,
			wantInclude: false,
			wantReason:  ContentDetailDecisionReasonUnknownFailClosed,
		},
		{
			name:        "unknown_hydration_error_fails_closed",
			decision:    ShadowDecisionUnknown,
			reason:      UnknownReasonHydrationError,
			wantInclude: false,
			wantReason:  ContentDetailDecisionReasonUnknownFailClosed,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := AdaptContentDetailDecision(tc.decision, tc.reason, ContentDetailEvaluatorModeEnforce)
			if got.Include != tc.wantInclude {
				t.Errorf("Include = %v, want %v", got.Include, tc.wantInclude)
			}
			if got.Reason != tc.wantReason {
				t.Errorf("Reason = %q, want %q", got.Reason, tc.wantReason)
			}
			if got.ShadowDecision != tc.decision {
				t.Errorf("ShadowDecision = %q, want %q", got.ShadowDecision, tc.decision)
			}
		})
	}
}

// TestAdaptContentDetailDecision_ModeIsPassive confirms the adapter
// mapping is mode-invariant; only callers react differently between
// shadow and enforce.
func TestAdaptContentDetailDecision_ModeIsPassive(t *testing.T) {
	shadow := AdaptContentDetailDecision(ShadowDecisionUnknown, UnknownReasonHydrationError, ContentDetailEvaluatorModeShadow)
	enforce := AdaptContentDetailDecision(ShadowDecisionUnknown, UnknownReasonHydrationError, ContentDetailEvaluatorModeEnforce)
	if shadow.Include != enforce.Include || shadow.Reason != enforce.Reason {
		t.Errorf("adapter mapping must be mode-invariant; shadow=%+v enforce=%+v", shadow, enforce)
	}
}

// TestNormalizeContentDetailEvaluatorMode tolerates whitespace and casing
// and falls safe to shadow on every unrecognised input.
func TestNormalizeContentDetailEvaluatorMode(t *testing.T) {
	cases := []struct {
		in   string
		want ContentDetailEvaluatorMode
	}{
		{"", ContentDetailEvaluatorModeShadow},
		{"shadow", ContentDetailEvaluatorModeShadow},
		{"SHADOW", ContentDetailEvaluatorModeShadow},
		{" shadow ", ContentDetailEvaluatorModeShadow},
		{"enforce", ContentDetailEvaluatorModeEnforce},
		{"ENFORCE", ContentDetailEvaluatorModeEnforce},
		{" enforce ", ContentDetailEvaluatorModeEnforce},
		{"weird", ContentDetailEvaluatorModeShadow},
		{"true", ContentDetailEvaluatorModeShadow},
		{"on", ContentDetailEvaluatorModeShadow},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			if got := NormalizeContentDetailEvaluatorMode(tc.in); got != tc.want {
				t.Errorf("NormalizeContentDetailEvaluatorMode(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestEnforceContentDetail_ShadowAllowsAll confirms that in shadow mode
// the helper short-circuits to Allow=true regardless of the underlying
// decision — the legacy handler's gate remains the sole authority.
func TestEnforceContentDetail_ShadowAllowsAll(t *testing.T) {
	// F1-W3B — nil inputs would fail-CLOSED in enforce mode but must
	// short-circuit to Allow=true in shadow mode.
	got := EnforceContentDetail(ContentDetailEvaluatorModeShadow, nil, nil, nil)
	if !got.Allow {
		t.Errorf("shadow mode must Allow=true regardless of decision; got %+v", got)
	}
	if got.Reason != ContentDetailDecisionReasonNone {
		t.Errorf("shadow mode must emit Reason=none; got %q", got.Reason)
	}
}

// TestEnforceContentDetail_EnforceUnknownFailsClosed confirms the detail
// doctrine inversion vs. feed: nil inputs → UNKNOWN/input_invalid →
// Allow=false (404), not fail-open keep.
func TestEnforceContentDetail_EnforceUnknownFailsClosed(t *testing.T) {
	got := EnforceContentDetail(ContentDetailEvaluatorModeEnforce, nil, nil, nil)
	if got.Allow {
		t.Errorf("enforce mode UNKNOWN must Allow=false (fail-closed); got %+v", got)
	}
	if got.Reason != ContentDetailDecisionReasonUnknownFailClosed {
		t.Errorf("enforce mode UNKNOWN must emit Reason=unknown_fail_closed; got %q", got.Reason)
	}
	if got.ShadowDecision != ShadowDecisionUnknown {
		t.Errorf("ShadowDecision passthrough broken; got %q", got.ShadowDecision)
	}
}

// TestContentDetailShadowRunner_ModeNilSafe verifies the mode accessor is
// safe on nil receiver.
func TestContentDetailShadowRunner_ModeNilSafe(t *testing.T) {
	var r *ContentDetailShadowRunner
	if got := r.Mode(); got != ContentDetailEvaluatorModeShadow {
		t.Errorf("nil runner Mode() = %q, want %q", got, ContentDetailEvaluatorModeShadow)
	}
	if got := r.WithMode(ContentDetailEvaluatorModeEnforce); got != nil {
		t.Errorf("nil runner WithMode must return nil, got %+v", got)
	}
}


