package policy

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// MuteChecker defines the interface for checking mute relationships.
// Implementations are responsible for managing their own DB connection;
// the notification policy layer has no transaction context to pass.
type MuteChecker interface {
	ExistsMute(ctx context.Context, muterID, mutedID uuid.UUID) (bool, error)
}

// MuteAction represents the mute policy evaluation result.
type MuteAction struct {
	Suppressed  bool   // true if a recipient-muted-sender relationship exists and delivery is suppressed
	PolicyError bool   // true if the mute checker returned an error
	Reason      string // for telemetry and logging
}

// MutePolicy evaluates mute relationships for notification delivery.
//
// CANONICAL BUSINESS TRUTH: mute is ENFORCED. When the recipient has muted
// the sender, chat notifications are suppressed on every channel (in-app and
// push). There is no shadow/observe mode and no environment knob — mute is
// normal business behavior, not a staged rollout.
//
// SCOPE: Notification delivery surface only. REST and WebSocket are unaffected.
// DIRECTION: Only recipient-muted-sender suppresses delivery.
//
//	Sender-muted-recipient has no delivery effect.
type MutePolicy struct {
	muteChecker MuteChecker
}

// NewMutePolicy creates a new MutePolicy bound to the given checker.
// There is a single construction model: enforcement is always on.
func NewMutePolicy(checker MuteChecker) *MutePolicy {
	return &MutePolicy{muteChecker: checker}
}

// ShouldApplyMute evaluates mute policy for notification delivery.
//
// When the recipient has muted the sender: Suppressed=true → suppress both
// in-app and push.
//
// FAIL-OPEN: mute is a preference, not a safety boundary. A missing checker
// or a checker error leaves delivery unaffected and reports the reason.
func (p *MutePolicy) ShouldApplyMute(
	ctx context.Context,
	senderID, recipientID uuid.UUID,
) MuteAction {
	if p.muteChecker == nil {
		return MuteAction{Reason: "no_mute_checker"}
	}

	// Check only the recipient-muted-sender direction: recipientID is the muter, senderID is muted.
	muted, err := p.muteChecker.ExistsMute(ctx, recipientID, senderID)
	if err != nil {
		// FAIL-OPEN: mute is a preference, not a safety boundary — uncertain state means deliver.
		return MuteAction{
			PolicyError: true,
			Reason:      fmt.Sprintf("mute_policy_error: %v", err),
		}
	}

	if !muted {
		return MuteAction{Reason: "not_muted"}
	}

	return MuteAction{Suppressed: true, Reason: "mute_enforced_drop"}
}
