package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// POSITIVE: a scheduled auction lapses when the seller's market authority
// expires before activation (owner decision Oct 2026: lapse, not cancel).
func TestLapse_FromScheduled(t *testing.T) {
	a := createTestAuction()
	require.NoError(t, a.Lapse())
	assert.Equal(t, StatusLapsed, a.Status)
}

// NEGATIVE: lapse is reachable from scheduled only — a running auction must
// continue to completion, and terminal states stay terminal.
func TestLapse_RejectsNonScheduledStatuses(t *testing.T) {
	for _, status := range []Status{
		StatusActive,
		StatusWaitingSettlement,
		StatusEnded,
		StatusCancelled,
		StatusLapsed,
	} {
		t.Run(string(status), func(t *testing.T) {
			a := createTestAuction()
			a.Status = status

			err := a.Lapse()

			require.Error(t, err)
			assert.IsType(t, &InvalidTransitionError{}, err)
			assert.Equal(t, status, a.Status, "status must be untouched on rejection")
		})
	}
}

// SURFACE LOCK: a lapsed auction is invisible to discovery, dead on the
// public wire, not cancellable and not repostable — inert until relisted.
func TestLapsed_IsHiddenAndInert(t *testing.T) {
	a := createTestAuction()
	require.NoError(t, a.Lapse())

	assert.False(t, a.Status.IsPublicDiscoverable())
	assert.Equal(t, "unavailable", a.Status.PublicLifecycle())
	assert.Equal(t, "cancelled", a.Status.PublicPhase(),
		"public phase must render dead — lapsed never reaches public discovery")
	assert.False(t, a.Status.IsRepostable())
	assert.False(t, a.CanCancel())
}

// STATE MACHINE LOCK: lapsed has exactly one outgoing edge — back to
// scheduled via the relist path (wired in the relist stage). Cancellation
// and settlement paths must not touch it.
func TestLapsed_OnlyExposesScheduledRelistPath(t *testing.T) {
	assert.Equal(t, []Status{StatusScheduled}, transitionAllowed[StatusLapsed],
		"lapsed may only ever lead back to scheduled (relist)")
}
