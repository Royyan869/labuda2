package evaluator_test

import (
	"testing"

	"github.com/labuda/backend/internal/governance/evaluator"
	"github.com/labuda/backend/internal/governance/viewercontext"
)

// =============================================================================
// WS SUBSCRIBE EVALUATOR TESTS
// =============================================================================

func TestWSSubscribe_ActiveMember_Allow(t *testing.T) {
	decision, reason := evaluator.EvaluateWSSubscribe(
		viewercontext.PublicLifecycleStateActive, true,
	)
	if decision != evaluator.WSSubscribeAllow {
		t.Errorf("active + member: got %q, want allow", decision)
	}
	if reason != evaluator.WSSubscribeDenyReasonNone {
		t.Errorf("active + member: deny reason = %q, want none", reason)
	}
}

func TestWSSubscribe_ActiveNonMember_Deny(t *testing.T) {
	decision, reason := evaluator.EvaluateWSSubscribe(
		viewercontext.PublicLifecycleStateActive, false,
	)
	if decision != evaluator.WSSubscribeDeny {
		t.Errorf("active + non-member: got %q, want deny", decision)
	}
	if reason != evaluator.WSSubscribeDenyReasonUnauthorized {
		t.Errorf("deny reason = %q, want room_unauthorized", reason)
	}
}

func TestWSSubscribe_RemovedUser_Deny(t *testing.T) {
	decision, reason := evaluator.EvaluateWSSubscribe(
		viewercontext.PublicLifecycleStateRemoved, true,
	)
	if decision != evaluator.WSSubscribeDeny {
		t.Errorf("removed user: got %q, want deny", decision)
	}
	if reason != evaluator.WSSubscribeDenyReasonRemoved {
		t.Errorf("deny reason = %q, want user_removed", reason)
	}
}

func TestWSSubscribe_UnavailableUser_Deny(t *testing.T) {
	decision, reason := evaluator.EvaluateWSSubscribe(
		viewercontext.PublicLifecycleStateUnavailable, true,
	)
	if decision != evaluator.WSSubscribeDeny {
		t.Errorf("unavailable user: got %q, want deny", decision)
	}
	if reason != evaluator.WSSubscribeDenyReasonSuspended {
		t.Errorf("deny reason = %q, want user_suspended", reason)
	}
}

func TestWSSubscribe_UnknownLifecycle_DenyFailClosed(t *testing.T) {
	decision, reason := evaluator.EvaluateWSSubscribe(
		viewercontext.PublicLifecycleState("unknown"), true,
	)
	if decision != evaluator.WSSubscribeDeny {
		t.Errorf("unknown lifecycle: got %q, want deny (fail-closed)", decision)
	}
	if reason != evaluator.WSSubscribeDenyReasonUnknown {
		t.Errorf("deny reason = %q, want unknown", reason)
	}
}

// =============================================================================
// WS BROADCAST EVALUATOR TESTS
// =============================================================================

func TestWSBroadcast_Active_Allow(t *testing.T) {
	decision := evaluator.EvaluateWSBroadcast(viewercontext.PublicLifecycleStateActive)
	if decision != evaluator.WSBroadcastAllow {
		t.Errorf("active: got %q, want allow", decision)
	}
}

func TestWSBroadcast_Removed_Drop(t *testing.T) {
	decision := evaluator.EvaluateWSBroadcast(viewercontext.PublicLifecycleStateRemoved)
	if decision != evaluator.WSBroadcastDrop {
		t.Errorf("removed: got %q, want drop", decision)
	}
}

func TestWSBroadcast_Unavailable_Drop(t *testing.T) {
	decision := evaluator.EvaluateWSBroadcast(viewercontext.PublicLifecycleStateUnavailable)
	if decision != evaluator.WSBroadcastDrop {
		t.Errorf("unavailable: got %q, want drop", decision)
	}
}

func TestWSBroadcast_UnknownLifecycle_DropFailClosed(t *testing.T) {
	decision := evaluator.EvaluateWSBroadcast(viewercontext.PublicLifecycleState("unknown"))
	if decision != evaluator.WSBroadcastDrop {
		t.Errorf("unknown lifecycle: got %q, want drop (fail-closed)", decision)
	}
}

// =============================================================================
// COARSENING → EVALUATOR INTEGRATION (end-to-end vocabulary proof)
// =============================================================================

// TestRemovedUserEndToEnd proves the full chain:
// GetStatus() returns "removed" → CoarsenLifecycle("removed", false) → removed
// → EvaluateWSSubscribe → deny
// → EvaluateWSBroadcast → drop
//
// This is the regression test for the stale-socket bypass bug where
// "removed" was not recognized by CoarsenLifecycle.
func TestRemovedUserEndToEnd(t *testing.T) {
	// Simulate GetStatus() returning "removed" for a soft-deleted user
	statusFromGetStatus := "removed"
	lifecycle := viewercontext.CoarsenLifecycle(statusFromGetStatus, false)

	if lifecycle != viewercontext.PublicLifecycleStateRemoved {
		t.Fatalf("CoarsenLifecycle(%q, false) = %q; want removed — vocabulary gap still open",
			statusFromGetStatus, lifecycle)
	}

	// Subscribe must deny
	subDecision, subReason := evaluator.EvaluateWSSubscribe(lifecycle, true)
	if subDecision != evaluator.WSSubscribeDeny {
		t.Errorf("subscribe: got %q, want deny", subDecision)
	}
	if subReason != evaluator.WSSubscribeDenyReasonRemoved {
		t.Errorf("subscribe deny reason = %q, want user_removed", subReason)
	}

	// Broadcast must drop
	bcastDecision := evaluator.EvaluateWSBroadcast(lifecycle)
	if bcastDecision != evaluator.WSBroadcastDrop {
		t.Errorf("broadcast: got %q, want drop", bcastDecision)
	}
}


