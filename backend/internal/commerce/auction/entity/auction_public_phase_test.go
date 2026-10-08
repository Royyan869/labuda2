package entity

import "testing"

// CANONICAL PUBLIC AUCTION PHASE VOCABULARY.
//
// The conversation product reference consumes Status.PublicPhase() as the
// auction lifecycle. This test locks the closed public vocabulary and the
// lapsed→cancelled coarsening (lapsed is an internal never-live state that must
// never surface as a distinct public card state).
func TestStatus_PublicPhase_Vocabulary(t *testing.T) {
	cases := []struct {
		status Status
		want   string
	}{
		{StatusScheduled, "scheduled"},
		{StatusActive, "active"},
		{StatusWaitingSettlement, "waiting_settlement"},
		{StatusEnded, "ended"},
		{StatusCancelled, "cancelled"},
		{StatusLapsed, "cancelled"}, // hidden internal state coarsens to cancelled
		{Status("unknown"), "cancelled"},
	}
	for _, tc := range cases {
		t.Run(string(tc.status), func(t *testing.T) {
			if got := tc.status.PublicPhase(); got != tc.want {
				t.Fatalf("Status(%q).PublicPhase() = %q, want %q", tc.status, got, tc.want)
			}
		})
	}
}
