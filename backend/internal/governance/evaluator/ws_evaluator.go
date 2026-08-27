package evaluator

import "github.com/labuda/backend/internal/governance/viewercontext"

// WSSubscribeDecision is the output of EvaluateWSSubscribe.
// WebSocket subscribe is fail-CLOSED: DENY on any non-active lifecycle or missing membership.
type WSSubscribeDecision string

const (
	WSSubscribeAllow WSSubscribeDecision = "allow"
	WSSubscribeDeny  WSSubscribeDecision = "deny"
)

// WSSubscribeDenyReason classifies subscribe denial for logging and metrics.
type WSSubscribeDenyReason string

const (
	WSSubscribeDenyReasonNone         WSSubscribeDenyReason = ""
	WSSubscribeDenyReasonSuspended    WSSubscribeDenyReason = "user_suspended"
	WSSubscribeDenyReasonBanned       WSSubscribeDenyReason = "user_banned"
	WSSubscribeDenyReasonRemoved      WSSubscribeDenyReason = "user_removed"
	WSSubscribeDenyReasonUnauthorized WSSubscribeDenyReason = "room_unauthorized"
	WSSubscribeDenyReasonUnknown      WSSubscribeDenyReason = "unknown"
)

// EvaluateWSSubscribe is the pure WS subscribe governance gate.
//
// Inputs are caller-hydrated; this function performs no IO.
// Fail-CLOSED per governance-constitution.md §5: DENY on any non-active lifecycle
// or when room membership is not granted.
//
// Caller hydration sequence (in subscribe_gate.go):
//  1. roomAuth.CanSubscribeToRoom() → membershipGranted
//  2. statusChecker.GetStatus() + CoarsenLifecycle() → lifecycle
//  3. Call EvaluateWSSubscribe(lifecycle, membershipGranted) → decision
func EvaluateWSSubscribe(
	lifecycle viewercontext.PublicLifecycleState,
	membershipGranted bool,
) (WSSubscribeDecision, WSSubscribeDenyReason) {
	switch lifecycle {
	case viewercontext.PublicLifecycleStateActive:
		// proceed to membership check
	case viewercontext.PublicLifecycleStateUnavailable:
		// unavailable = suspended or banned; both deny subscribe
		return WSSubscribeDeny, WSSubscribeDenyReasonSuspended
	case viewercontext.PublicLifecycleStateRemoved:
		return WSSubscribeDeny, WSSubscribeDenyReasonRemoved
	default:
		// unknown lifecycle state → fail-closed
		return WSSubscribeDeny, WSSubscribeDenyReasonUnknown
	}

	if !membershipGranted {
		return WSSubscribeDeny, WSSubscribeDenyReasonUnauthorized
	}

	return WSSubscribeAllow, WSSubscribeDenyReasonNone
}

// WSBroadcastDecision is the per-subscriber output of EvaluateWSBroadcast.
// Broadcast is fail-CLOSED: DROP when lifecycle is non-active.
type WSBroadcastDecision string

const (
	WSBroadcastAllow WSBroadcastDecision = "allow"
	WSBroadcastDrop  WSBroadcastDecision = "drop"
)

// EvaluateWSBroadcast is the pure per-subscriber WS broadcast governance gate.
//
// Inputs are caller-hydrated; this function performs no IO.
// Fail-CLOSED per governance-constitution.md §5: DROP on any non-active lifecycle.
// The Dispatcher constructs a fresh lifecycle per subscriber at broadcast time;
// subscribe-time lifecycle MUST NOT be reused here (ADR-005).
//
// Block overlay at broadcast time is CHAT-4 scope. The minimal-envelope WS frame
// carries no message payload; the client re-fetches via REST which applies block
// enforcement canonically (governance-constitution.md §2.2 interim relaxation).
func EvaluateWSBroadcast(lifecycle viewercontext.PublicLifecycleState) WSBroadcastDecision {
	if lifecycle == viewercontext.PublicLifecycleStateActive {
		return WSBroadcastAllow
	}
	return WSBroadcastDrop
}


