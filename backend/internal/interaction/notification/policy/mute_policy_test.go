package policy

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
)

// --- test doubles ---

type stubMuteChecker struct {
	muted bool
	err   error
}

func (s *stubMuteChecker) ExistsMute(_ context.Context, _, _ uuid.UUID) (bool, error) {
	return s.muted, s.err
}

// stubMuteCheckerDirectional returns muted=true only for the exact (muterID, mutedID) pair.
type stubMuteCheckerDirectional struct {
	muterID uuid.UUID
	mutedID uuid.UUID
}

func (s *stubMuteCheckerDirectional) ExistsMute(_ context.Context, muterID, mutedID uuid.UUID) (bool, error) {
	return muterID == s.muterID && mutedID == s.mutedID, nil
}

// --- A: not muted → deliver ---

func TestShouldApplyMute_NotMuted_Delivers(t *testing.T) {
	p := NewMutePolicy(&stubMuteChecker{muted: false})
	action := p.ShouldApplyMute(context.Background(), uuid.New(), uuid.New())

	if action.Suppressed {
		t.Errorf("Suppressed = true, want false")
	}
	if action.PolicyError {
		t.Errorf("PolicyError = true, want false")
	}
	if action.Reason != "not_muted" {
		t.Errorf("Reason = %q, want %q", action.Reason, "not_muted")
	}
}

// --- B: muted → suppress (canonical enforced behavior) ---

func TestShouldApplyMute_Muted_Suppresses(t *testing.T) {
	p := NewMutePolicy(&stubMuteChecker{muted: true})
	action := p.ShouldApplyMute(context.Background(), uuid.New(), uuid.New())

	if !action.Suppressed {
		t.Errorf("Suppressed = false, want true (mute is enforced)")
	}
	if action.PolicyError {
		t.Errorf("PolicyError = true, want false")
	}
	if action.Reason != "mute_enforced_drop" {
		t.Errorf("Reason = %q, want %q", action.Reason, "mute_enforced_drop")
	}
}

// --- C: direction — only recipient-muted-sender suppresses ---

func TestShouldApplyMute_SenderMutedRecipient_DoesNotSuppress(t *testing.T) {
	senderID := uuid.New()
	recipientID := uuid.New()

	// Mute exists in sender→recipient direction only; policy checks recipient→sender.
	p := NewMutePolicy(&stubMuteCheckerDirectional{muterID: senderID, mutedID: recipientID})
	action := p.ShouldApplyMute(context.Background(), senderID, recipientID)

	if action.Suppressed {
		t.Errorf("Suppressed = true: wrong direction must not suppress")
	}
}

func TestShouldApplyMute_RecipientMutedSender_Suppresses(t *testing.T) {
	senderID := uuid.New()
	recipientID := uuid.New()

	// Mute exists in recipient→sender direction; policy checks this direction.
	p := NewMutePolicy(&stubMuteCheckerDirectional{muterID: recipientID, mutedID: senderID})
	action := p.ShouldApplyMute(context.Background(), senderID, recipientID)

	if !action.Suppressed {
		t.Errorf("Suppressed = false: recipient-muted-sender must suppress")
	}
}

// --- D: error path — fail-open ---

func TestShouldApplyMute_CheckerError_FailOpen(t *testing.T) {
	p := NewMutePolicy(&stubMuteChecker{err: fmt.Errorf("db timeout")})
	action := p.ShouldApplyMute(context.Background(), uuid.New(), uuid.New())

	if action.Suppressed {
		t.Errorf("Suppressed = true: mute must fail-open on error")
	}
	if !action.PolicyError {
		t.Errorf("PolicyError = false, want true on checker error")
	}
}

// --- E: nil checker ---

func TestShouldApplyMute_NilChecker_Delivers(t *testing.T) {
	p := NewMutePolicy(nil)
	action := p.ShouldApplyMute(context.Background(), uuid.New(), uuid.New())

	if action.Suppressed {
		t.Errorf("Suppressed = true, want false (nil checker = no filtering)")
	}
	if action.Reason != "no_mute_checker" {
		t.Errorf("Reason = %q, want %q", action.Reason, "no_mute_checker")
	}
}

// --- F: regression — "invalid transaction type" must never appear ---

func TestShouldApplyMute_ReasonNeverContainsInvalidTxType(t *testing.T) {
	cases := []struct {
		name  string
		muted bool
		err   error
	}{
		{"not_muted", false, nil},
		{"muted", true, nil},
		{"error", false, fmt.Errorf("some db error")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := NewMutePolicy(&stubMuteChecker{muted: tc.muted, err: tc.err})
			action := p.ShouldApplyMute(context.Background(), uuid.New(), uuid.New())
			if action.Reason == "invalid transaction type" {
				t.Errorf("Reason = %q: nil-tx bug still present", action.Reason)
			}
		})
	}
}
