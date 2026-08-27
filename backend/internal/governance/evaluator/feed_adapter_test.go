package evaluator

import "testing"

// TestAdaptFeedDecision_Mapping exercises the canonical mapping table for
// the C1 feed adapter convergence. The table mirrors AdaptSearchContentDecision
// one-for-one except for the UNKNOWN policy — feed fail-OPEN, no
// fail-closed branch on input_invalid.
func TestAdaptFeedDecision_Mapping(t *testing.T) {
	cases := []struct {
		name            string
		decision        ShadowDecision
		reason          UnknownReason
		wantInclude     bool
		wantOverride    *string
		wantReason      FeedDecisionReason
	}{
		{
			name:        "allow",
			decision:    ShadowDecisionAllow,
			wantInclude: true,
			wantReason:  FeedDecisionReasonNone,
		},
		{
			name:        "deny",
			decision:    ShadowDecisionDeny,
			wantInclude: false,
			wantReason:  FeedDecisionReasonDeny,
		},
		{
			name:         "tombstone_maps_to_removed_override",
			decision:     ShadowDecisionTombstone,
			wantInclude:  true,
			wantOverride: ptr(FeedLifecycleRemoved),
			wantReason:   FeedDecisionReasonTombstone,
		},
		{
			name:         "redact_maps_to_unavailable_override",
			decision:     ShadowDecisionRedact,
			wantInclude:  true,
			wantOverride: ptr(FeedLifecycleUnavailable),
			wantReason:   FeedDecisionReasonRedact,
		},
		{
			name:        "unknown_input_invalid_fails_open",
			decision:    ShadowDecisionUnknown,
			reason:      UnknownReasonInputInvalid,
			wantInclude: true,
			wantReason:  FeedDecisionReasonUnknownFailOpen,
		},
		{
			name:        "unknown_viewer_overlay_missing_fails_open",
			decision:    ShadowDecisionUnknown,
			reason:      UnknownReasonViewerOverlayMissing,
			wantInclude: true,
			wantReason:  FeedDecisionReasonUnknownFailOpen,
		},
		{
			name:        "unknown_target_overlay_missing_fails_open",
			decision:    ShadowDecisionUnknown,
			reason:      UnknownReasonTargetOverlayMissing,
			wantInclude: true,
			wantReason:  FeedDecisionReasonUnknownFailOpen,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := AdaptFeedDecision(tc.decision, tc.reason, FeedEvaluatorModeEnforce)
			if got.Include != tc.wantInclude {
				t.Errorf("Include = %v, want %v", got.Include, tc.wantInclude)
			}
			if tc.wantOverride == nil {
				if got.LifecycleOverride != nil {
					t.Errorf("LifecycleOverride = %v, want nil", *got.LifecycleOverride)
				}
			} else {
				if got.LifecycleOverride == nil {
					t.Errorf("LifecycleOverride = nil, want %q", *tc.wantOverride)
				} else if *got.LifecycleOverride != *tc.wantOverride {
					t.Errorf("LifecycleOverride = %q, want %q", *got.LifecycleOverride, *tc.wantOverride)
				}
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

// TestAdaptFeedDecision_ModeIsPassive confirms the adapter mapping is the
// same in shadow and enforce mode. Mode is a passive label; only callers
// react differently.
func TestAdaptFeedDecision_ModeIsPassive(t *testing.T) {
	shadow := AdaptFeedDecision(ShadowDecisionTombstone, UnknownReasonNone, FeedEvaluatorModeShadow)
	enforce := AdaptFeedDecision(ShadowDecisionTombstone, UnknownReasonNone, FeedEvaluatorModeEnforce)
	if shadow.Include != enforce.Include ||
		shadow.Reason != enforce.Reason ||
		(shadow.LifecycleOverride == nil) != (enforce.LifecycleOverride == nil) ||
		(shadow.LifecycleOverride != nil && *shadow.LifecycleOverride != *enforce.LifecycleOverride) {
		t.Errorf("adapter mapping must be mode-invariant; shadow=%+v enforce=%+v", shadow, enforce)
	}
}

func ptr(s string) *string { return &s }


