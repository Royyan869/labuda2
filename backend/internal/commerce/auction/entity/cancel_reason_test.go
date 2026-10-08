package entity

import "testing"

// CancelReason is a pure outbox-audit vocabulary (no notification routing
// exists on it — the subscription-expired direction was purged Oct 2026 when
// subscription expiry started LAPPSING auctions instead of cancelling them).
// This lock prevents the vocabulary from silently drifting: payload consumers
// (audit, dashboards) rely on these exact wire values.
func TestCancelReasonVocabulary(t *testing.T) {
	cases := []struct {
		reason CancelReason
		want   string
	}{
		{CancelReasonSeller, "seller"},
		{CancelReasonModeration, "moderation"},
		{CancelReasonAdmin, "admin"},
		{CancelReasonLegacy, ""},
	}

	for _, tc := range cases {
		if got := string(tc.reason); got != tc.want {
			t.Errorf("CancelReason wire value = %q, want %q", got, tc.want)
		}
	}
}
