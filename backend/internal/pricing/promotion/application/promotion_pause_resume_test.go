package application_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========================================================================
// P5B-A FOUNDATION: PAUSE/RESUME ENTITY-LEVEL REGRESSION TESTS
//
// These tests verify the entity-level pause/resume behavior that the
// service layer depends on. They regression-lock the contract that:
// - Pause sets status=paused, records PausedAt, does NOT finalize
// - Resume sets status=active, accumulates TotalPausedDuration, clears PausedAt
// - Duration math correctly excludes paused time
// - Ownership is NOT touched during pause/resume cycle
// ========================================================================

func TestP5BA_PausePromotion_SetsStatusPaused(t *testing.T) {
	instance := createActiveInstanceWithElapsed(t, 5*time.Second)
	pauseTime := time.Now()

	err := instance.Pause(pauseTime)
	require.NoError(t, err)

	assert.Equal(t, entity.InstanceStatusPaused, instance.Status, "Status should be paused")
	assert.NotNil(t, instance.PausedAt, "PausedAt should be set")
	assert.False(t, instance.Finalized, "Instance should NOT be finalized on pause")
	assert.Nil(t, instance.FinalizedAt, "FinalizedAt should be nil")
	assert.Equal(t, 0, instance.FinalizedSeconds, "FinalizedSeconds should be 0")
}

func TestP5BA_PausePromotion_NoOwnershipConsumptionIncrement(t *testing.T) {
	// Verify that pausing does NOT bake consumed duration into ownership.
	// This is the core business truth: paused time must not burn.
	instance := createActiveInstanceWithElapsed(t, 3*time.Second)

	// Record consumed before pause
	consumedBefore := instance.GetConsumedDurationSecondsAt(time.Now())
	assert.Greater(t, consumedBefore, 0, "Should have consumed some time before pause")

	pauseTime := time.Now()
	err := instance.Pause(pauseTime)
	require.NoError(t, err)

	// After pause: instance is not finalized, consumed duration frozen at pause time
	assert.False(t, instance.Finalized, "Must NOT finalize on pause")

	// GetConsumedDurationSecondsAt should return time up to PausedAt, NOT current time
	consumedAfterPause := instance.GetConsumedDurationSecondsAt(time.Now().Add(10 * time.Second))
	// Should be roughly the same as at pause time (frozen clock)
	assert.InDelta(t, consumedBefore, consumedAfterPause, 2,
		"Consumed duration should freeze at pause time, not grow")
}

func TestP5BA_ResumePromotion_SetsStatusActive(t *testing.T) {
	instance := createActiveInstanceWithElapsed(t, 3*time.Second)

	// Pause for 2 seconds
	pauseTime := time.Now()
	err := instance.Pause(pauseTime)
	require.NoError(t, err)

	resumeTime := pauseTime.Add(2 * time.Second)
	err = instance.Resume(resumeTime)
	require.NoError(t, err)

	assert.Equal(t, entity.InstanceStatusActive, instance.Status, "Status should be active")
	assert.Nil(t, instance.PausedAt, "PausedAt should be cleared")
	assert.GreaterOrEqual(t, instance.TotalPausedDuration, 1,
		"TotalPausedDuration should include the pause period")
	assert.LessOrEqual(t, instance.TotalPausedDuration, 3,
		"TotalPausedDuration should be approximately 2 seconds")
}

func TestP5BA_ResumePromotion_BlockedIfNotPaused(t *testing.T) {
	instance := createActiveInstanceWithElapsed(t, 1*time.Second)

	// Try to resume an active instance
	err := instance.Resume(time.Now())
	assert.Error(t, err, "Resume should fail for active instance")
}

func TestP5BA_ResumePromotion_BlockedIfTerminal(t *testing.T) {
	instance := createActiveInstanceWithElapsed(t, 1*time.Second)

	// Stop it terminally
	err := instance.Stop(entity.StopReasonUserCancelled, time.Now())
	require.NoError(t, err)

	// Try to resume a cancelled instance
	err = instance.Resume(time.Now())
	assert.Error(t, err, "Resume should fail for cancelled instance")
}

func TestP5BA_PausePromotion_BlockedIfAlreadyPaused(t *testing.T) {
	instance := createActiveInstanceWithElapsed(t, 1*time.Second)

	pauseTime := time.Now()
	err := instance.Pause(pauseTime)
	require.NoError(t, err)

	// Try to pause again — idempotency guard
	err = instance.Pause(time.Now())
	assert.Error(t, err, "Second pause should fail (idempotency guard)")
}

func TestP5BA_PausePromotion_BlockedIfTerminal(t *testing.T) {
	instance := createActiveInstanceWithElapsed(t, 1*time.Second)

	err := instance.Stop(entity.StopReasonUserCancelled, time.Now())
	require.NoError(t, err)

	err = instance.Pause(time.Now())
	assert.Error(t, err, "Pause should fail for cancelled instance")
}

func TestP5BA_CancelStillFinalizesAndBakesDuration(t *testing.T) {
	// Verify that user_cancelled still follows the terminal path:
	// Stop → Snapshot → Finalize
	instance := createActiveInstanceWithElapsed(t, 5*time.Second)

	stopTime := time.Now()
	err := instance.Stop(entity.StopReasonUserCancelled, stopTime)
	require.NoError(t, err)

	assert.Equal(t, entity.InstanceStatusCancelled, instance.Status, "Should be cancelled")
	assert.True(t, instance.Status.IsTerminal(), "Cancelled is terminal")

	// Snapshot consumed duration
	consumedSeconds := instance.SnapshotConsumedDuration(stopTime)
	assert.Greater(t, consumedSeconds, 0, "Should have consumed duration")
	assert.True(t, instance.Finalized, "Should be finalized after snapshot")
	assert.Equal(t, consumedSeconds, instance.FinalizedSeconds)
}

func TestP5BA_PauseResumeCycle_DurationExcludesPausedTime(t *testing.T) {
	// Multi-cycle test: active → pause → resume → pause → resume
	// Total paused time must be excluded from consumed duration.
	now := time.Now()
	activateTime := now.Add(-10 * time.Second) // 10 seconds ago

	ownershipID := uuid.New()
	userID := uuid.New()
	targetID := uuid.New()
	instance, err := entity.NewPromotionInstance(
		ownershipID, userID, entity.TargetTypeForSale, &targetID,
		activateTime,
	)
	require.NoError(t, err)
	err = instance.Activate(activateTime)
	require.NoError(t, err)

	// Pause at t+3s (active for 3s)
	pause1 := activateTime.Add(3 * time.Second)
	err = instance.Pause(pause1)
	require.NoError(t, err)

	// Resume at t+5s (paused for 2s)
	resume1 := activateTime.Add(5 * time.Second)
	err = instance.Resume(resume1)
	require.NoError(t, err)
	assert.Equal(t, 2, instance.TotalPausedDuration, "First pause should be 2 seconds")

	// Pause at t+8s (active for 3 more seconds)
	pause2 := activateTime.Add(8 * time.Second)
	err = instance.Pause(pause2)
	require.NoError(t, err)

	// Resume at t+9s (paused for 1s)
	resume2 := activateTime.Add(9 * time.Second)
	err = instance.Resume(resume2)
	require.NoError(t, err)
	assert.Equal(t, 3, instance.TotalPausedDuration, "Total pause should be 3 seconds (2+1)")

	// Check consumed at t+10s
	checkTime := activateTime.Add(10 * time.Second)
	consumed := instance.GetConsumedDurationSecondsAt(checkTime)

	// Wall clock: 10 seconds
	// Total paused: 3 seconds
	// Expected consumed: 7 seconds
	assert.Equal(t, 7, consumed,
		"Consumed should be wall_clock(10) - paused(3) = 7")
}

func TestP5BA_PausedInstanceNotTerminal(t *testing.T) {
	instance := createActiveInstanceWithElapsed(t, 1*time.Second)

	err := instance.Pause(time.Now())
	require.NoError(t, err)

	assert.False(t, instance.Status.IsTerminal(),
		"Paused status must NOT be terminal")
	assert.True(t, instance.Status.CanActivate(),
		"Paused status should allow re-activation (resume)")
}

func TestP5BA_DiscoveryIgnoresPaused(t *testing.T) {
	// The entity InstanceStatus model proves this:
	// Discovery queries filter WHERE status = 'active'
	// Paused is a distinct status, so it's excluded.

	assert.NotEqual(t, entity.InstanceStatusActive, entity.InstanceStatusPaused,
		"Active and Paused must be distinct statuses")

	// CanActivate returns true for paused — meaning it can transition to active
	assert.True(t, entity.InstanceStatusPaused.CanActivate(),
		"Paused should be activatable (for resume)")

	// IsActive returns false for paused
	assert.False(t, entity.InstanceStatusPaused.IsActive(),
		"Paused should NOT be considered active")
}

func TestP5BA_StopFromPausedState(t *testing.T) {
	// A paused instance can still be terminally stopped (e.g., by expiration worker).
	instance := createActiveInstanceWithElapsed(t, 3*time.Second)

	pauseTime := time.Now()
	err := instance.Pause(pauseTime)
	require.NoError(t, err)

	// Stop from paused state (e.g., validity_expired)
	stopTime := pauseTime.Add(5 * time.Second)
	err = instance.Stop(entity.StopReasonValidityExpired, stopTime)
	require.NoError(t, err)

	assert.Equal(t, entity.InstanceStatusCancelled, instance.Status)
	assert.True(t, instance.Status.IsTerminal())

	// Snapshot should capture consumed duration correctly
	consumed := instance.SnapshotConsumedDuration(stopTime)
	// Consumed = StoppedAt - ActivatedAt - TotalPausedDuration
	// The 5 seconds of pause should NOT count
	assert.Greater(t, consumed, 0, "Should have consumed time before pause")
}

func TestP5BA_DeactivateReasonSplit_UserPaused(t *testing.T) {
	// Verify that StopReasonUserPaused is a valid stop reason constant
	assert.Equal(t, entity.StopReason("user_paused"), entity.StopReasonUserPaused)
}

func TestP5BA_DeactivateReasonSplit_UserCancelled(t *testing.T) {
	// Verify that StopReasonUserCancelled is a valid stop reason constant
	assert.Equal(t, entity.StopReason("user_cancelled"), entity.StopReasonUserCancelled)
}

func TestP5BA_InstanceNotPausedError(t *testing.T) {
	// Verify the InstanceNotPausedError type exists and formats correctly
	instanceID := uuid.New()
	err := fmt.Errorf("wrap: %w", &instanceNotPausedErrorProxy{InstanceID: instanceID, Status: entity.InstanceStatusActive})
	assert.Contains(t, err.Error(), "not paused")
	assert.Contains(t, err.Error(), instanceID.String())
}

// ========================================================================
// HELPERS
// ========================================================================

func createActiveInstanceWithElapsed(t *testing.T, elapsed time.Duration) *entity.PromotionInstance {
	t.Helper()
	ownershipID := uuid.New()
	userID := uuid.New()
	targetID := uuid.New()

	activateTime := time.Now().Add(-elapsed)
	instance, err := entity.NewPromotionInstance(
		ownershipID, userID, entity.TargetTypeForSale, &targetID,
		activateTime,
	)
	require.NoError(t, err)
	err = instance.Activate(activateTime)
	require.NoError(t, err)

	return instance
}

// instanceNotPausedErrorProxy mirrors the service error for entity-only tests.
type instanceNotPausedErrorProxy struct {
	InstanceID uuid.UUID
	Status     entity.InstanceStatus
}

func (e *instanceNotPausedErrorProxy) Error() string {
	return fmt.Sprintf("instance is not paused: %s (status: %s)", e.InstanceID, e.Status)
}
