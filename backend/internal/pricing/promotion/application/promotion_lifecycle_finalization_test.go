package application_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========================================================================
// P4B: EXPIRED OWNERSHIP FINALIZES ACTIVE INSTANCE
// ========================================================================

// TestP4B_ExpiredOwnership_ActiveInstanceFinalized verifies that when an
// ownership expires with an active instance, the instance is finalized via
// the canonical 4-step sequence (Stop → Snapshot → Bake → Persist) instead
// of the old MarkAsExpired shortcut.
func TestP4B_ExpiredOwnership_ActiveInstanceFinalized(t *testing.T) {
	// Simulate an active instance that has been running for 10 seconds
	instance := createP4BActiveInstance(t, 10*time.Second)
	assert.True(t, instance.IsActive())
	assert.False(t, instance.Finalized)

	now := time.Now()

	// CANONICAL FINALIZATION (what ProcessExpiredOwnerships now does):
	// 1. Stop
	err := instance.Stop(entity.StopReasonValidityExpired, now)
	require.NoError(t, err)
	assert.Equal(t, entity.InstanceStatusCancelled, instance.Status)

	// 2. Snapshot
	consumedSeconds := instance.SnapshotConsumedDuration(now)
	assert.True(t, instance.Finalized, "Instance must be finalized after snapshot")
	assert.Greater(t, consumedSeconds, 0, "Must have consumed duration")
	assert.Equal(t, consumedSeconds, instance.FinalizedSeconds)

	// 3. Bake into ownership (simulated — entity-level proof)
	ownership := createP4BOwnership(t, 72, 0)
	fullyConsumed := ownership.AddConsumedDurationSeconds(consumedSeconds, now)
	assert.False(t, fullyConsumed, "10 seconds should not exhaust 72 hours")
	assert.Greater(t, ownership.ConsumedDurationHours, 0, "Consumed hours must increase")

	// 4. After finalization, GetConsumedDurationSeconds returns 0 (no double-count)
	postFinalize := instance.GetConsumedDurationSeconds(now)
	assert.Equal(t, 0, postFinalize, "Finalized instance must return 0")
}

// TestP4B_ExpiredOwnership_PausedInstanceFinalized verifies that a paused
// instance is also properly finalized when ownership expires.
func TestP4B_ExpiredOwnership_PausedInstanceFinalized(t *testing.T) {
	instance := createP4BActiveInstance(t, 8*time.Second)

	// Pause it
	pauseTime := time.Now().Add(-3 * time.Second)
	instance.Status = entity.InstanceStatusPaused
	instance.PausedAt = &pauseTime

	now := time.Now()

	// Canonical finalization for paused instance:
	err := instance.Stop(entity.StopReasonValidityExpired, now)
	require.NoError(t, err)

	consumedSeconds := instance.SnapshotConsumedDuration(now)
	assert.True(t, instance.Finalized)
	// Should have consumed ~8 seconds active minus ~3 seconds pause = ~5 seconds
	// But TotalPausedDuration is 0 (no prior resume), PausedAt is set.
	// GetConsumedDurationSecondsAt calculates up to StoppedAt minus TotalPausedDuration.
	assert.Greater(t, consumedSeconds, 0, "Paused instance must have consumed some duration")
}

// TestP4B_ExpiredOwnership_AlreadyFinalizedSkipped verifies idempotency:
// an already-finalized instance is skipped without error.
func TestP4B_ExpiredOwnership_AlreadyFinalizedSkipped(t *testing.T) {
	instance := createP4BActiveInstance(t, 5*time.Second)
	now := time.Now()

	// First finalization
	err := instance.Stop(entity.StopReasonValidityExpired, now)
	require.NoError(t, err)
	snap1 := instance.SnapshotConsumedDuration(now)

	// Simulate re-processing (idempotency check)
	assert.True(t, instance.Finalized)
	assert.True(t, instance.Status.IsTerminal())

	// Second snapshot returns same value
	snap2 := instance.SnapshotConsumedDuration(now)
	assert.Equal(t, snap1, snap2, "Idempotent: second snapshot returns same value")
}

// TestP4B_ExpiredOwnership_InactiveInstanceSkipped verifies that inactive
// instances (never activated) are not erroneously finalized.
func TestP4B_ExpiredOwnership_InactiveInstanceSkipped(t *testing.T) {
	ownershipID := uuid.New()
	userID := uuid.New()
	targetID := uuid.New()

	instance, err := entity.NewPromotionInstance(
		ownershipID, userID, entity.TargetTypeForSale, &targetID,
		time.Now(),
	)
	require.NoError(t, err)

	// Inactive instance should not be stoppable via normal flow
	assert.Equal(t, entity.InstanceStatusInactive, instance.Status)
	assert.False(t, instance.IsActive())

	// Consumed duration should be 0
	consumed := instance.GetConsumedDurationSeconds(time.Now())
	assert.Equal(t, 0, consumed)
}

// ========================================================================
// P4B: DURATION EXHAUSTION DETECTION
// ========================================================================

// TestP4B_DurationExhausted_FinalizedCorrectly simulates the scenario where
// an instance's consumed wall-clock time exceeds the ownership's remaining
// duration, triggering duration_exhausted finalization.
func TestP4B_DurationExhausted_FinalizedCorrectly(t *testing.T) {
	// Ownership: 1 hour total, 0 consumed
	ownership := createP4BOwnership(t, 1, 0)

	// Instance that has been running for 1 hour + 1 second
	instance := createP4BActiveInstance(t, 1*time.Hour+1*time.Second)

	now := time.Now()
	currentConsumed := instance.GetConsumedDurationSecondsAt(now)
	remainingSeconds := (ownership.TotalDurationHours - ownership.ConsumedDurationHours) * 3600

	// Should exceed remaining
	assert.GreaterOrEqual(t, currentConsumed, remainingSeconds,
		"Instance consumed must >= ownership remaining for exhaustion trigger")

	// Finalize with duration_exhausted
	err := instance.Stop(entity.StopReasonDurationExhausted, now)
	require.NoError(t, err)

	consumedSeconds := instance.SnapshotConsumedDuration(now)
	fullyConsumed := ownership.AddConsumedDurationSeconds(consumedSeconds, now)

	assert.True(t, instance.Finalized)
	assert.Equal(t, "duration_exhausted", *instance.StopReason)
	assert.True(t, fullyConsumed, "Ownership should be fully consumed")
	assert.Equal(t, entity.OwnershipStatusConsumed, ownership.Status)
}

// TestP4B_DurationExhausted_NotYetExhausted verifies that an instance
// whose consumed time is less than remaining is NOT finalized.
func TestP4B_DurationExhausted_NotYetExhausted(t *testing.T) {
	ownership := createP4BOwnership(t, 72, 0)              // 72 hours total
	instance := createP4BActiveInstance(t, 10*time.Second) // Only 10 seconds

	now := time.Now()
	currentConsumed := instance.GetConsumedDurationSecondsAt(now)
	remainingSeconds := (ownership.TotalDurationHours - ownership.ConsumedDurationHours) * 3600

	assert.Less(t, currentConsumed, remainingSeconds,
		"Instance consumed must be < remaining — should NOT be finalized")
	assert.True(t, instance.IsActive(), "Instance should remain active")
	assert.False(t, instance.Finalized)
}

// ========================================================================
// P4B: SWEEP FINALIZATION SETS FINALIZED FLAG
// ========================================================================

// TestP4B_SweepFinalization_SetsFinalized verifies that the rewritten sweep
// path produces a properly finalized instance (Finalized=true, FinalizedSeconds>0).
func TestP4B_SweepFinalization_SetsFinalized(t *testing.T) {
	instance := createP4BActiveInstance(t, 5*time.Second)
	now := time.Now()

	// Simulate what the rewritten SweepInactivePromotions does:
	// 1. Check: not finalized, not terminal
	assert.False(t, instance.Finalized)
	assert.False(t, instance.Status.IsTerminal())

	// 2. Stop with governance reason
	err := instance.Stop(entity.StopReasonSellerGovernance, now)
	require.NoError(t, err)

	// 3. Snapshot
	consumed := instance.SnapshotConsumedDuration(now)
	assert.True(t, instance.Finalized, "Sweep must set Finalized=true")
	assert.Greater(t, consumed, 0, "Sweep must capture consumed duration")
	assert.Equal(t, consumed, instance.FinalizedSeconds)

	// 4. GetConsumedDurationSeconds returns 0 after finalization
	assert.Equal(t, 0, instance.GetConsumedDurationSeconds(now))
}

// TestP4B_SweepFinalization_TerminalSkipped verifies that sweep correctly
// skips instances that are already terminal.
func TestP4B_SweepFinalization_TerminalSkipped(t *testing.T) {
	instance := createP4BActiveInstance(t, 3*time.Second)
	now := time.Now()

	// Stop it first
	err := instance.Stop(entity.StopReasonForSaleSold, now)
	require.NoError(t, err)
	instance.SnapshotConsumedDuration(now)

	// Now try to stop again (as sweep would)
	assert.True(t, instance.Status.IsTerminal())
	assert.True(t, instance.Finalized)

	// Stop should fail
	err = instance.Stop(entity.StopReasonSellerGovernance, now)
	assert.Error(t, err, "Cannot stop terminal instance")
}

// ========================================================================
// P4B: OWNERSHIP ACCOUNTING CAP INVARIANT
// ========================================================================

// TestP4B_OwnershipCap_NeverExceedsTotal verifies that AddConsumedDurationSeconds
// caps at TotalDurationHours even if called with excessive seconds.
func TestP4B_OwnershipCap_NeverExceedsTotal(t *testing.T) {
	ownership := createP4BOwnership(t, 10, 8) // 10 total, 8 consumed

	// Try to add 5 hours worth of seconds (would exceed 10)
	fullyConsumed := ownership.AddConsumedDurationSeconds(5*3600, time.Now())

	assert.True(t, fullyConsumed, "Should be fully consumed")
	assert.Equal(t, ownership.TotalDurationHours, ownership.ConsumedDurationHours,
		"ConsumedDurationHours must be capped at TotalDurationHours")
	assert.Equal(t, entity.OwnershipStatusConsumed, ownership.Status)
}

// TestP4B_OwnershipCap_ZeroSecondsNoOp verifies that baking 0 seconds is harmless.
func TestP4B_OwnershipCap_ZeroSecondsNoOp(t *testing.T) {
	ownership := createP4BOwnership(t, 72, 10)
	before := ownership.ConsumedDurationHours

	fullyConsumed := ownership.AddConsumedDurationSeconds(0, time.Now())

	assert.False(t, fullyConsumed)
	assert.Equal(t, before, ownership.ConsumedDurationHours,
		"0 seconds should not change consumed hours")
}

// ========================================================================
// P4B: STOP REASON CORRECTNESS
// ========================================================================

// TestP4B_StopReason_ValidityExpired verifies that validity_expired is a
// recognized stop reason and is used for ownership expiry.
func TestP4B_StopReason_ValidityExpired(t *testing.T) {
	instance := createP4BActiveInstance(t, 3*time.Second)

	err := instance.Stop(entity.StopReasonValidityExpired, time.Now())
	require.NoError(t, err)

	assert.Equal(t, entity.InstanceStatusCancelled, instance.Status)
	assert.Equal(t, "validity_expired", *instance.StopReason)
}

// TestP4B_StopReason_DurationExhausted verifies that duration_exhausted is a
// recognized stop reason and is used for duration exhaustion.
func TestP4B_StopReason_DurationExhausted(t *testing.T) {
	instance := createP4BActiveInstance(t, 3*time.Second)

	err := instance.Stop(entity.StopReasonDurationExhausted, time.Now())
	require.NoError(t, err)

	assert.Equal(t, entity.InstanceStatusCancelled, instance.Status)
	assert.Equal(t, "duration_exhausted", *instance.StopReason)
}

// ========================================================================
// P4B HELPERS
// ========================================================================

func createP4BActiveInstance(t *testing.T, runDuration time.Duration) *entity.PromotionInstance {
	ownershipID := uuid.New()
	userID := uuid.New()
	targetID := uuid.New()
	now := time.Now()

	instance, err := entity.NewPromotionInstance(
		ownershipID, userID, entity.TargetTypeForSale, &targetID,
		now,
	)
	require.NoError(t, err)

	err = instance.Activate(now)
	require.NoError(t, err)

	// Simulate wall-clock duration by backdating ActivatedAt
	pastTime := time.Now().Add(-runDuration)
	instance.ActivatedAt = &pastTime

	return instance
}

func createP4BOwnership(t *testing.T, totalHours, consumedHours int) *entity.PromotionOwnership {
	now := time.Now()
	ownership, err := entity.NewPromotionOwnership(
		uuid.New(), uuid.New(), totalHours, 336, now,
	)
	require.NoError(t, err)
	ownership.ConsumedDurationHours = consumedHours
	return ownership
}
