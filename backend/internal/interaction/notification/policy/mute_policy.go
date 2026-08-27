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

// MuteMode controls whether mute suppression is active or shadow-only.
type MuteMode string

const (
	// MuteShadow evaluates mute and emits divergence telemetry but always delivers.
	// This is the safe default — no user-visible behavior change.
	MuteShadow MuteMode = "shadow"

	// MuteEnforce evaluates mute and suppresses delivery when muted.
	// Requires explicit opt-in via MUTE_CHAT_NOTIFICATION_ENFORCE=true.
	MuteEnforce MuteMode = "enforce"
)

// MuteAction represents the mute policy evaluation result.
type MuteAction struct {
	WouldSuppress bool   // true if mute relationship exists and would suppress delivery
	Suppressed    bool   // true if actually suppressed (enforce mode + muted)
	PolicyError   bool   // true if mute checker returned an error
	Reason        string // for telemetry and logging
}

// MutePolicy evaluates mute relationships for notification delivery.
//
// SHADOW-FIRST: Default mode is MuteShadow. Suppression requires explicit MuteEnforce.
// SCOPE: Notification delivery surface only. REST and WebSocket are unaffected.
// DIRECTION: Only recipient-muted-sender suppresses delivery.
//
//	Sender-muted-recipient has no delivery effect.
type MutePolicy struct {
	muteChecker MuteChecker
	mode        MuteMode
}

// NewMutePolicy creates a new MutePolicy.
// If mode is empty, it defaults to MuteShadow.
func NewMutePolicy(checker MuteChecker, mode MuteMode) *MutePolicy {
	if mode == "" {
		mode = MuteShadow
	}
	return &MutePolicy{muteChecker: checker, mode: mode}
}

// Mode returns the current enforcement mode.
func (p *MutePolicy) Mode() MuteMode {
	return p.mode
}

// ShouldApplyMute evaluates mute policy for notification delivery.
//
// Only recipient-muted-sender semantics apply. Sender-muted-recipient has no effect.
//
// Shadow mode: WouldSuppress=true, Suppressed=false → deliver + emit divergence telemetry.
// Enforce mode: WouldSuppress=true, Suppressed=true → suppress both in-app and push.
//
// FAIL-OPEN: mute is a preference, not a safety boundary.
// On checker error: PolicyError=true, deliver (fail-open), telemetry emitted.
func (p *MutePolicy) ShouldApplyMute(
	ctx context.Context,
	senderID, recipientID uuid.UUID,
) MuteAction {
	if p.muteChecker == nil {
		return MuteAction{WouldSuppress: false, Suppressed: false, Reason: "no_mute_checker"}
	}

	// Check only the recipient-muted-sender direction: recipientID is the muter, senderID is muted.
	muted, err := p.muteChecker.ExistsMute(ctx, recipientID, senderID)
	if err != nil {
		// FAIL-OPEN: mute is a preference, not a safety boundary — uncertain state means deliver.
		return MuteAction{
			WouldSuppress: false,
			Suppressed:    false,
			PolicyError:   true,
			Reason:        fmt.Sprintf("mute_policy_error: %v", err),
		}
	}

	if !muted {
		return MuteAction{WouldSuppress: false, Suppressed: false, Reason: "not_muted"}
	}

	if p.mode == MuteEnforce {
		return MuteAction{WouldSuppress: true, Suppressed: true, Reason: "mute_enforced_drop"}
	}

	// Shadow mode: mute relationship observed but delivery proceeds.
	return MuteAction{WouldSuppress: true, Suppressed: false, Reason: "mute_shadow_deliver"}
}


