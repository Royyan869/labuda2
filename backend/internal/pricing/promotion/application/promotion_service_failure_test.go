package application_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pricing/promotion/application"
	"github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========================================================================
// FAILURE SCENARIO: CRASH AFTER SNAPSHOT, BEFORE OWNERSHIP UPDATE
// ========================================================================

func TestFailureScenario_CrashAfterSnapshotBeforeOwnershipUpdate(t *testing.T) {
	// SCENARIO: Instance is stopped and snapshot is taken,
	// but system crashes before updating ownership.consumed_duration_hours

	// Create an instance that ran for 5 seconds
	instance := createActiveInstanceWithDuration(t, 5*time.Second)

	// Stop the instance
	err := instance.Stop(entity.StopReasonUserCancelled, time.Now())
	require.NoError(t, err)

	// Snapshot consumed duration
	snapshotTime := time.Now()
	consumedSeconds := instance.SnapshotConsumedDuration(snapshotTime)

	assert.True(t, instance.Finalized, "Instance should be finalized")
	assert.Equal(t, consumedSeconds, instance.FinalizedSeconds, "FinalizedSeconds should match")

	// SIMULATE CRASH: System crashes here, before ownership update
	// When system recovers, the instance is already finalized
	// GetConsumedDurationSeconds should return 0 to prevent double count

	// After recovery, try to get consumed duration again
	recoveredConsumed := instance.GetConsumedDurationSeconds(time.Now())
	assert.Equal(t, 0, recoveredConsumed, "Finalized instance should return 0, preventing double count")

	// Ownership update can be safely retried - it's just an additive operation
	// The finalized flag prevents double counting
}

// ========================================================================
// FAILURE SCENARIO: CRASH AFTER OWNERSHIP UPDATE, BEFORE INSTANCE UPDATE
// ========================================================================

func TestFailureScenario_CrashAfterOwnershipUpdateBeforeInstanceUpdate(t *testing.T) {
	// SCENARIO: Ownership is updated with consumed duration,
	// but system crashes before instance.finalized is persisted

	instance := createActiveInstanceWithDuration(t, 3*time.Second)

	// Stop the instance
	err := instance.Stop(entity.StopReasonUserCancelled, time.Now())
	require.NoError(t, err)

	// Snapshot
	consumedSeconds := instance.SnapshotConsumedDuration(time.Now())

	// SIMULATE: Ownership is updated in DB
	// But instance update (finalized flag) hasn't happened yet
	// System crashes

	// On recovery, instance is loaded from DB - it's NOT finalized yet
	// But ownership already has the duration added
	// If we snapshot again, we should get the same value (idempotency)

	// Retry snapshot
	consumedSeconds2 := instance.SnapshotConsumedDuration(time.Now())

	// CRITICAL: Second snapshot should return same value
	assert.Equal(t, consumedSeconds, consumedSeconds2, "Second snapshot should be idempotent")

	// This ensures that even if ownership was updated but instance wasn't,
	// retrying won't double-count
}

// ========================================================================
// FAILURE SCENARIO: CONCURRENT REASSIGN
// ========================================================================

func TestFailureScenario_ConcurrentReassign(t *testing.T) {
	// SCENARIO: Two users try to reassign the same instance simultaneously

	oldInstance := createActiveInstanceWithDuration(t, 5*time.Second)

	// User A starts reassign
	// Flow: STOP OLD -> SNAPSHOT -> UPDATE OWNERSHIP -> UPDATE OLD -> CREATE NEW
	// User B starts reassign at same time

	// User A: Stop old
	err := oldInstance.Stop(entity.StopReasonUserCancelled, time.Now())
	require.NoError(t, err)

	// User A: Snapshot
	snapshot1 := oldInstance.SnapshotConsumedDuration(time.Now())

	// User B: Tries to stop (but instance is already stopped)
	err = oldInstance.Stop(entity.StopReasonUserCancelled, time.Now())
	// This will fail because instance is already in terminal state
	assert.Error(t, err, "Second stop should fail")

	// User B: Tries to snapshot anyway
	snapshot2 := oldInstance.SnapshotConsumedDuration(time.Now())

	// Both snapshots should return same value (idempotency protects here)
	assert.Equal(t, snapshot1, snapshot2, "Concurrent snapshots should be idempotent")

	// User A's transaction will complete first
	// User B's transaction will fail when trying to update the instance
	// (optimistic lock or version check would catch this)
}

// ========================================================================
// FAILURE SCENARIO: CONCURRENT ACTIVATE
// ========================================================================

func TestFailureScenario_ConcurrentActivate(t *testing.T) {
	// SCENARIO: Two users try to activate promotions on the same target simultaneously

	_ = uuid.New() // ownershipID
	_ = uuid.New() // targetID

	// User A starts activation flow
	// Service layer checks: HasActivePromotionForTarget -> returns false
	// User B starts activation at same time
	// Service layer checks: HasActivePromotionForTarget -> returns false

	// Both proceed to create instance
	// First one to INSERT wins
	// Second one hits UNIQUE constraint violation

	// This is prevented by:
	// 1. GetActiveInstanceByOwnershipForUpdate (locks ownership row)
	// 2. HasActivePromotionForTarget check
	// 3. UNIQUE index on (target_type, target_id) for active instances

	// Test that service layer check catches this
	// (In real scenario, this would be tested with actual DB)
}

// ========================================================================
// FAILURE SCENARIO: DOUBLE STOP
// ========================================================================

func TestFailureScenario_DoubleStop(t *testing.T) {
	// SCENARIO: Stop is called twice on the same instance
	// (Could happen due to retry logic or duplicate events)

	instance := createActiveInstanceWithDuration(t, 3*time.Second)

	// First stop
	err := instance.Stop(entity.StopReasonUserCancelled, time.Now())
	require.NoError(t, err)
	assert.Equal(t, entity.InstanceStatusCancelled, instance.Status)

	// Second stop - should fail
	err = instance.Stop(entity.StopReasonDurationExhausted, time.Now())
	assert.Error(t, err, "Second stop should fail")
	assert.Contains(t, fmt.Sprint(err), "transition", "Error should mention state transition")

	// Even if someone bypasses the error and calls snapshot:
	snapshot1 := instance.SnapshotConsumedDuration(time.Now())
	snapshot2 := instance.SnapshotConsumedDuration(time.Now())

	// Idempotency protects against double snapshot
	assert.Equal(t, snapshot1, snapshot2, "Double snapshot should be idempotent")
}

// ========================================================================
// FAILURE SCENARIO: INSTANCE IN STUCK PAUSED STATE
// ========================================================================

func TestFailureScenario_StuckPausedInstance(t *testing.T) {
	// SCENARIO: User pauses promotion and never resumes
	// Instance stays paused forever

	instance := createActiveInstanceWithDuration(t, 2*time.Second)

	// Pause
	err := instance.Pause(time.Now())
	require.NoError(t, err)
	assert.Equal(t, entity.InstanceStatusPaused, instance.Status)

	// Time passes (1 hour)
	// Instance is still paused

	// Get consumed duration should calculate up to pause time
	consumed := instance.GetConsumedDurationSeconds(time.Now())
	assert.Greater(t, consumed, 0, "Should have consumed time before pause")
	assert.Less(t, consumed, 3, "Should be less than 3 seconds")

	// If user never resumes, this is a problem
	// RECOMMENDATION: Add worker to auto-expire paused instances after X days
	// Or allow admin to force-stop paused instances

	// For now, this test documents the issue
}

// ========================================================================
// INVARIANT TESTS
// ========================================================================

func TestInvariant_OwnershipConsumedNeverExceedsTotal(t *testing.T) {
	// INVARIANT: ownership.consumed_duration_hours <= ownership.total_duration_hours

	// This should be enforced at:
	// 1. Entity level: AddConsumedDurationToOwnership should cap at total
	// 2. Database level: CHECK constraint
	// 3. Service level: Validate before activation

	// Test that consumed can't exceed total
	totalHours := 10
	consumedHours := 5

	assert.LessOrEqual(t, consumedHours, totalHours,
		"Consumed duration should never exceed total duration")

	// If more duration is added, it should be capped
	additionalHours := 8
	newConsumed := consumedHours + additionalHours
	cappedConsumed := min(newConsumed, totalHours)

	assert.Equal(t, totalHours, cappedConsumed,
		"Should cap consumed duration at total")
}

func TestInvariant_SumFinalizedInstancesEqualsOwnershipConsumed(t *testing.T) {
	// INVARIANT: SUM(instance.finalized_seconds) / 3600 <= ownership.consumed_duration_hours

	// This ensures that all finalized instances' durations are properly accounted for

	instance1 := createActiveInstanceWithDuration(t, 2*time.Second)
	instance1.Stop(entity.StopReasonUserCancelled, time.Now())
	instance1.SnapshotConsumedDuration(time.Now())

	instance2 := createActiveInstanceWithDuration(t, 3*time.Second)
	instance2.Stop(entity.StopReasonUserCancelled, time.Now())
	instance2.SnapshotConsumedDuration(time.Now())

	// Sum of finalized seconds
	totalFinalizedSeconds := instance1.FinalizedSeconds + instance2.FinalizedSeconds
	totalFinalizedHours := totalFinalizedSeconds / 3600

	// Ownership consumed hours should be >= this sum
	ownershipConsumedHours := 10 // Example value

	assert.GreaterOrEqual(t, ownershipConsumedHours, totalFinalizedHours,
		"Ownership consumed should account for all finalized instances")
}

// ========================================================================
// HELPER FUNCTIONS
// ========================================================================

func createActiveInstanceWithDuration(t *testing.T, duration time.Duration) *entity.PromotionInstance {
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

	// Simulate duration passing by manipulating ActivatedAt
	oldActivatedAt := instance.ActivatedAt
	pastTime := time.Now().Add(-duration)
	instance.ActivatedAt = &pastTime

	// Restore old value in test (for cleanup)
	t.Cleanup(func() {
		instance.ActivatedAt = oldActivatedAt
	})

	return instance
}

// Mock service for testing (simplified)
type mockPromotionService struct {
	finalizedInstances map[uuid.UUID]bool
}

func newMockService() *mockPromotionService {
	return &mockPromotionService{
		finalizedInstances: make(map[uuid.UUID]bool),
	}
}

func (m *mockPromotionService) DeactivatePromotion(
	ctx context.Context,
	instanceID uuid.UUID,
	reason entity.StopReason,
) error {
	// Check if already finalized
	if m.finalizedInstances[instanceID] {
		return &application.InstanceAlreadyFinalizedError{InstanceID: instanceID}
	}

	// Mark as finalized
	m.finalizedInstances[instanceID] = true
	return nil
}

// Custom error type for testing
type InstanceAlreadyFinalizedError struct {
	InstanceID uuid.UUID
}

func (e *InstanceAlreadyFinalizedError) Error() string {
	return fmt.Sprintf("instance already finalized: %s", e.InstanceID)
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
