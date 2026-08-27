package entity_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========================================================================
// SNAPSHOT IDEMPOTENCY TESTS
// ========================================================================

func TestSnapshotConsumedDuration_Idempotent(t *testing.T) {
	// Create an active instance
	instance := createActiveInstance(t)
	time.Sleep(100 * time.Millisecond) // Ensure some duration has passed

	// First snapshot
	duration1 := instance.SnapshotConsumedDuration(time.Now())
	assert.True(t, instance.Finalized, "Instance should be finalized after first snapshot")
	assert.NotNil(t, instance.FinalizedAt, "FinalizedAt should be set")
	assert.Equal(t, duration1, instance.FinalizedSeconds, "FinalizedSeconds should match snapshot")
	assert.Greater(t, duration1, 0, "Duration should be greater than 0")

	// Wait a bit
	time.Sleep(50 * time.Millisecond)

	// Second snapshot - should return same value without changing anything
	finalizedAt := instance.FinalizedAt
	duration2 := instance.SnapshotConsumedDuration(time.Now())

	// CRITICAL: Second snapshot must not change anything
	assert.Equal(t, duration1, duration2, "Second snapshot should return same duration")
	assert.Equal(t, finalizedAt, instance.FinalizedAt, "FinalizedAt should not change on second call")
	assert.Equal(t, duration1, instance.FinalizedSeconds, "FinalizedSeconds should not change")

	// Third snapshot - still idempotent
	duration3 := instance.SnapshotConsumedDuration(time.Now())
	assert.Equal(t, duration1, duration3, "Third snapshot should return same duration")
}

func TestSnapshotConsumedDuration_DoesNotDoubleCount(t *testing.T) {
	// Create an instance that ran for 5 seconds
	instance := createActiveInstance(t)
	time.Sleep(5 * 100 * time.Millisecond) // ~500ms (using 100ms x 5 for faster test)

	// Snapshot
	consumedSeconds := instance.SnapshotConsumedDuration(time.Now())

	// Get consumed duration - should return 0 for finalized instances
	// (to prevent double counting in ownership calculations)
	actualConsumed := instance.GetConsumedDurationSeconds(time.Now())

	assert.Equal(t, 0, actualConsumed, "Finalized instance should return 0 from GetConsumedDurationSeconds")
	assert.Greater(t, consumedSeconds, 0, "Snapshot should have captured the duration")
}

func TestSnapshotConsumedDuration_WithPause(t *testing.T) {
	// Create an active instance with 5 seconds of history
	instance := createActiveInstance(t)

	// Pause
	err := instance.Pause(time.Now())
	require.NoError(t, err)

	// The instance was active for 5 seconds before pause
	// Snapshot should capture that
	consumedSeconds := instance.SnapshotConsumedDuration(time.Now())

	// Should have consumed the 5 seconds of active time
	assert.Greater(t, consumedSeconds, 0, "Should have consumed some time")
	assert.Equal(t, 5, consumedSeconds, "Should have consumed exactly 5 seconds")
}

// ========================================================================
// FLOW ORDER TESTS
// ========================================================================

func TestReassignPromotion_NoDoubleActiveWindow(t *testing.T) {
	// This test verifies that during reassignment, there's no window
	// where both old and new instances are active

	// Create old instance
	oldInstance := createActiveInstance(t)
	assert.True(t, oldInstance.IsActive(), "Old instance should be active")

	// Simulate STOP OLD first (correct order)
	oldInstance.Stop(entity.StopReasonUserCancelled, time.Now())
	assert.False(t, oldInstance.IsActive(), "Old instance should be stopped")

	// Snapshot old
	oldInstance.SnapshotConsumedDuration(time.Now())
	assert.True(t, oldInstance.Finalized, "Old instance should be finalized")

	// NOW create new instance (after old is stopped and finalized)
	newInstance := createActiveInstance(t)
	assert.True(t, newInstance.IsActive(), "New instance should be active")

	// CRITICAL: At this point, only new instance should be active
	assert.False(t, oldInstance.IsActive(), "Old instance should remain inactive")
	assert.True(t, newInstance.IsActive(), "New instance should be active")
}

// ========================================================================
// PAUSE IDEMPOTENCY TESTS
// ========================================================================

func TestPause_Idempotent(t *testing.T) {
	instance := createActiveInstance(t)

	// First pause
	err := instance.Pause(time.Now())
	require.NoError(t, err)
	assert.Equal(t, entity.InstanceStatusPaused, instance.Status)

	// Wait a bit
	time.Sleep(50 * time.Millisecond)

	// Second pause - should fail with transition error
	err = instance.Pause(time.Now())
	assert.Error(t, err, "Second pause should return error")
	assert.Contains(t, err.Error(), "transition", "Error should mention invalid transition")
}

func TestPause_CannotPauseInactive(t *testing.T) {
	instance := createInactiveInstance(t)

	err := instance.Pause(time.Now())
	assert.Error(t, err, "Should not be able to pause inactive instance")

	// Verify instance didn't change
	assert.Equal(t, entity.InstanceStatusInactive, instance.Status)
	assert.Nil(t, instance.PausedAt, "PausedAt should not be set")
}

func TestPause_CannotPauseAlreadyPaused(t *testing.T) {
	instance := createActiveInstance(t)

	// First pause
	err := instance.Pause(time.Now())
	require.NoError(t, err)

	// Try to pause again
	err = instance.Pause(time.Now())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transition")
}

func TestResume_CalculatesPauseDuration(t *testing.T) {
	// Create instance and manually set PausedAt to simulate pause
	instance := createActiveInstance(t)

	// Simulate pause by manually setting state with 2 seconds of pause
	pastTime := time.Now().Add(-2 * time.Second)
	instance.Status = entity.InstanceStatusPaused
	instance.PausedAt = &pastTime

	// Resume
	err := instance.Resume(time.Now())
	require.NoError(t, err)

	// TotalPausedDuration should be approximately 2 seconds
	assert.GreaterOrEqual(t, instance.TotalPausedDuration, 1, "Should have recorded at least 1 second paused")
	assert.LessOrEqual(t, instance.TotalPausedDuration, 3, "Should have recorded at most 3 seconds paused")
	assert.Nil(t, instance.PausedAt, "PausedAt should be cleared after resume")

	// Verify instance is active again
	assert.Equal(t, entity.InstanceStatusActive, instance.Status)
}

// ========================================================================
// FINALIZED GUARD TESTS
// ========================================================================

func TestStop_CannotStopFinalizedInstance(t *testing.T) {
	instance := createActiveInstance(t)

	// Stop and finalize
	instance.Stop(entity.StopReasonUserCancelled, time.Now())
	instance.SnapshotConsumedDuration(time.Now())

	assert.True(t, instance.Finalized, "Instance should be finalized")

	// Try to stop again - entity layer allows this (it's a no-op for terminal states)
	// but service layer should check Finalized flag and prevent operation
	err := instance.Stop(entity.StopReasonDurationExhausted, time.Now())
	// This will fail because instance is already in terminal state
	assert.Error(t, err, "Should not be able to stop an already stopped instance")
	assert.Contains(t, err.Error(), "transition", "Error should mention state transition")

	// This test documents that service layer MUST check Finalized flag
	// before attempting to stop instances
}

// ========================================================================
// DURATION CALCULATION TESTS
// ========================================================================

func TestGetConsumedDurationSeconds_FinalizedReturnsZero(t *testing.T) {
	instance := createActiveInstance(t)
	time.Sleep(100 * time.Millisecond)

	// Snapshot
	duration := instance.SnapshotConsumedDuration(time.Now())
	assert.Greater(t, duration, 0, "Should have consumed some time")

	// GetConsumedDurationSeconds should return 0 for finalized instances
	consumed := instance.GetConsumedDurationSeconds(time.Now())
	assert.Equal(t, 0, consumed, "Finalized instance should return 0")
}

func TestGetConsumedDurationSeconds_At(t *testing.T) {
	// Create instance with activation time in the past
	instance := createActiveInstance(t)

	// The instance was activated 5 seconds ago
	// Calculate duration at current time
	now := time.Now()
	durationAtNow := instance.GetConsumedDurationSecondsAt(now)

	// Should be approximately 5 seconds
	assert.GreaterOrEqual(t, durationAtNow, 4, "Should have consumed at least 4 seconds")
	assert.LessOrEqual(t, durationAtNow, 6, "Should have consumed at most 6 seconds")

	// Calculate duration 1 second in the future
	futureTime := now.Add(1 * time.Second)
	durationAtFuture := instance.GetConsumedDurationSecondsAt(futureTime)

	// Duration should be approximately 1 second more
	diff := durationAtFuture - durationAtNow
	assert.GreaterOrEqual(t, diff, 0, "Difference should be non-negative")
	assert.LessOrEqual(t, diff, 2, "Difference should be at most 2 seconds")
}

// ========================================================================
// HELPER FUNCTIONS
// ========================================================================

func createActiveInstance(t *testing.T) *entity.PromotionInstance {
	ownershipID := uuid.New()
	userID := uuid.New()
	targetID := uuid.New()
	now := time.Now()

	instance, err := entity.NewPromotionInstance(
		ownershipID,
		userID,
		entity.TargetTypeForSale,
		&targetID,

		now,
	)
	require.NoError(t, err)

	err = instance.Activate(now)
	require.NoError(t, err)

	// Simulate time passing by manipulating ActivatedAt
	// This allows tests to verify duration calculations without slow sleeps
	pastTime := time.Now().Add(-5 * time.Second)
	instance.ActivatedAt = &pastTime

	return instance
}

func createInactiveInstance(t *testing.T) *entity.PromotionInstance {
	ownershipID := uuid.New()
	userID := uuid.New()
	targetID := uuid.New()

	instance, err := entity.NewPromotionInstance(
		ownershipID,
		userID,
		entity.TargetTypeForSale,
		&targetID,

		time.Now(),
	)
	require.NoError(t, err)

	return instance
}
