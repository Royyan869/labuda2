package entity

import "testing"

// Scope B — CancelReason routing authority lock.
//
// Only subscription_expired auto-cancels notify the seller (system-initiated;
// the seller took no action). Self-cancels need no echo, moderation/admin
// outcomes travel their canonical channels, and the legacy zero value (old
// reasonless events) must fail closed as a no-op.
func TestCancelReasonNotifiesSeller(t *testing.T) {
	cases := []struct {
		reason CancelReason
		want   bool
	}{
		{CancelReasonSeller, false},
		{CancelReasonSubscriptionExpired, true},
		{CancelReasonModeration, false},
		{CancelReasonAdmin, false},
		{CancelReasonLegacy, false}, // fail-closed for reasonless legacy events
		{CancelReason("anything_else"), false},
	}

	for _, tc := range cases {
		if got := tc.reason.NotifiesSeller(); got != tc.want {
			t.Errorf("CancelReason(%q).NotifiesSeller() = %v, want %v", tc.reason, got, tc.want)
		}
	}
}
