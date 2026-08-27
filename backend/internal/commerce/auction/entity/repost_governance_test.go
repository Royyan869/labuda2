package entity

// Repost governance regression lock for auction Status.IsRepostable().
//
// FIX-2 (2026-05-28): validateAuctionTarget() in content_service previously
// checked status == "closed" which never matches any real auction status.
// IsRepostable() is now the single source of truth.

import "testing"

// TestAuctionStatus_IsRepostable pins the exact set of statuses that are/are not
// repostable. Any change must be reviewed for feed/search SQL alignment.
func TestAuctionStatus_IsRepostable(t *testing.T) {
	cases := []struct {
		status     Status
		repostable bool
	}{
		{StatusScheduled, true},
		{StatusActive, true},
		// Terminal states — all must return false.
		{StatusEnded, false},
		{StatusCancelled, false},
		{StatusExpiredBNR, false},
		{StatusWaitingSettlement, false},
		{StatusDraft, false},
		// The old "closed" string — must also be false (never existed).
		{Status("closed"), false},
		{Status("unknown"), false},
		{Status(""), false},
	}
	for _, tc := range cases {
		t.Run(string(tc.status), func(t *testing.T) {
			got := tc.status.IsRepostable()
			if got != tc.repostable {
				t.Errorf("Status(%q).IsRepostable() = %v; want %v", tc.status, got, tc.repostable)
			}
		})
	}
}

// TestAuctionStatus_ClosedNeverRepostable regression-locks that the old "closed"
// string is NOT repostable. This was the exact string used in the broken
// validateAuctionTarget check before FIX-2.
func TestAuctionStatus_ClosedNeverRepostable(t *testing.T) {
	if Status("closed").IsRepostable() {
		t.Error("Status(\"closed\") must not be repostable — this status does not exist in the enum")
	}
}


