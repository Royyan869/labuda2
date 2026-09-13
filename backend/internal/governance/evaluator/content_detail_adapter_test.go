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
			got := AdaptContentDetailDecision(tc.decision, tc.reason)
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



// TestEnforceContentDetail_EnforceUnknownFailsClosed confirms the detail
// doctrine inversion vs. feed: nil inputs → UNKNOWN/input_invalid →
// Allow=false (404), not fail-open keep.
func TestEnforceContentDetail_EnforceUnknownFailsClosed(t *testing.T) {
	got := EnforceContentDetail(nil, nil, nil)
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


