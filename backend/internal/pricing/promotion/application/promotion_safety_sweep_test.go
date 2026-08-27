package application_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pricing/promotion/application"
	"github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========================================================================
// P5B-B: SAFETY WORKER PAUSE/RESUME CONVERSION TESTS
//
// These tests verify the classification logic and entity-level behavior
// that the safety worker sweep relies on. They regression-lock:
// - Reversible reasons → Pause (no finalization)
// - Permanent reasons → Stop + finalize
// - Paused + operable → Resume
// - Paused + permanent → Stop + finalize
// ========================================================================

// ========================================================================
// REVERSIBLE/PERMANENT CLASSIFICATION TESTS
// ========================================================================

func TestP5BB_IsReversibleReason_SellerInactive(t *testing.T) {
	assert.True(t, application.IsReversibleReason("seller_inactive"),
		"seller_inactive (subscription expired) should be reversible")
}

func TestP5BB_IsReversibleReason_ForSaleHidden(t *testing.T) {
	assert.True(t, application.IsReversibleReason("for_sale_hidden"),
		"for_sale_hidden (withdrawn) should be reversible")
}

func TestP5BB_IsReversibleReason_ForSaleModerated(t *testing.T) {
	assert.True(t, application.IsReversibleReason("for_sale_moderated"),
		"for_sale_moderated should be reversible")
}

func TestP5BB_IsPermanent_SellerRemoved(t *testing.T) {
	assert.False(t, application.IsReversibleReason("seller_removed"),
		"seller_removed should be permanent")
}

func TestP5BB_IsPermanent_SellerAccountInactive(t *testing.T) {
	assert.False(t, application.IsReversibleReason("seller_account_inactive"),
		"seller_account_inactive should be permanent")
}

func TestP5BB_IsPermanent_ForSaleSold(t *testing.T) {
	assert.False(t, application.IsReversibleReason("for_sale_sold"),
		"for_sale_sold should be permanent")
}

func TestP5BB_IsPermanent_AuctionEnded(t *testing.T) {
	assert.False(t, application.IsReversibleReason("auction_ended"),
		"auction_ended should be permanent")
}

func TestP5BB_IsPermanent_AuctionCancelled(t *testing.T) {
	assert.False(t, application.IsReversibleReason("auction_cancelled"),
		"auction_cancelled should be permanent")
}

func TestP5BB_IsPermanent_UserRemoved(t *testing.T) {
	assert.False(t, application.IsReversibleReason("user_removed"),
		"user_removed should be permanent")
}

// ========================================================================
// ACTIVE SWEEP: REVERSIBLE CONDITION → PAUSE ENTITY BEHAVIOR
// ========================================================================

func TestP5BB_ActiveSweep_ReversibleCondition_PausesInstance(t *testing.T) {
	// Simulates what the safety worker does when it finds an active instance
	// with a reversible inoperability reason: calls entity Pause().
	instance := createP5BBActiveInstance(t, 3*time.Second)
	pauseTime := time.Now()

	// Entity Pause (what sweep does for reversible conditions)
	err := instance.Pause(pauseTime)
	require.NoError(t, err)

	assert.Equal(t, entity.InstanceStatusPaused, instance.Status,
		"Reversible condition should PAUSE, not stop")
	assert.False(t, instance.Finalized,
		"Paused instance must NOT be finalized")
	assert.Nil(t, instance.FinalizedAt,
		"FinalizedAt must be nil")
	assert.Equal(t, 0, instance.FinalizedSeconds,
		"FinalizedSeconds must be 0 (no ownership bake)")
	assert.NotNil(t, instance.PausedAt,
		"PausedAt must be set")
	assert.False(t, instance.Status.IsTerminal(),
		"Paused must NOT be terminal")
}

func TestP5BB_ActiveSweep_ReversibleCondition_NoDurationBake(t *testing.T) {
	// Key accounting invariant: pause does NOT bake duration into ownership.
	// Duration stays live on the instance and freezes at PausedAt.
	instance := createP5BBActiveInstance(t, 5*time.Second)

	consumedBefore := instance.GetConsumedDurationSecondsAt(time.Now())
	assert.Greater(t, consumedBefore, 0)

	pauseTime := time.Now()
	err := instance.Pause(pauseTime)
	require.NoError(t, err)

	// After pause: consumed duration should freeze, not grow
	futureTime := time.Now().Add(10 * time.Second)
	consumedAfterPause := instance.GetConsumedDurationSecondsAt(futureTime)
	assert.InDelta(t, consumedBefore, consumedAfterPause, 2,
		"Consumed duration should freeze at pause time")

	// Finalization fields must remain untouched
	assert.False(t, instance.Finalized)
	assert.Equal(t, 0, instance.FinalizedSeconds)
}

// ========================================================================
// ACTIVE SWEEP: PERMANENT CONDITION → STOP + FINALIZE
// ========================================================================

func TestP5BB_ActiveSweep_PermanentCondition_StopsAndFinalizes(t *testing.T) {
	// Simulates what the safety worker does for permanent conditions:
	// canonical 4-step Stop → Snapshot → (Bake) → Persist
	instance := createP5BBActiveInstance(t, 5*time.Second)
	stopTime := time.Now()

	// 1. Stop
	err := instance.Stop(entity.StopReasonForSaleSold, stopTime)
	require.NoError(t, err)

	assert.Equal(t, entity.InstanceStatusCancelled, instance.Status)
	assert.True(t, instance.Status.IsTerminal())

	// 2. Snapshot
	consumed := instance.SnapshotConsumedDuration(stopTime)
	assert.Greater(t, consumed, 0)

	// 3. Verify finalization
	assert.True(t, instance.Finalized,
		"Permanent condition must finalize")
	assert.Equal(t, consumed, instance.FinalizedSeconds,
		"FinalizedSeconds must match snapshot")
	assert.NotNil(t, instance.FinalizedAt)

	// 4. GetConsumedDurationSeconds returns 0 after finalization (double-count guard)
	assert.Equal(t, 0, instance.GetConsumedDurationSeconds(time.Now()))
}

// ========================================================================
// PAUSED SWEEP: OPERABLE AGAIN → RESUME
// ========================================================================

func TestP5BB_PausedSweep_OperableAgain_ResumesInstance(t *testing.T) {
	// Simulates: instance was paused by safety worker, condition cleared,
	// next sweep resumes it.
	instance := createP5BBActiveInstance(t, 3*time.Second)

	// Pause
	pauseTime := time.Now()
	err := instance.Pause(pauseTime)
	require.NoError(t, err)
	assert.Equal(t, entity.InstanceStatusPaused, instance.Status)

	// Resume (what sweep does when target is operable again)
	resumeTime := pauseTime.Add(10 * time.Second)
	err = instance.Resume(resumeTime)
	require.NoError(t, err)

	assert.Equal(t, entity.InstanceStatusActive, instance.Status,
		"Should resume to active")
	assert.Nil(t, instance.PausedAt,
		"PausedAt should be cleared")
	assert.GreaterOrEqual(t, instance.TotalPausedDuration, 9,
		"TotalPausedDuration should accumulate pause period")
	assert.LessOrEqual(t, instance.TotalPausedDuration, 11,
		"TotalPausedDuration should be approximately 10 seconds")
	assert.False(t, instance.Finalized,
		"Resumed instance must NOT be finalized")
}

func TestP5BB_PausedSweep_ResumeExcludesPausedDuration(t *testing.T) {
	// After resume, consumed duration must exclude the paused period.
	now := time.Now()
	activateTime := now.Add(-20 * time.Second) // 20 seconds ago

	instance := createP5BBInstanceAt(t, activateTime)

	// Pause at t+5s (active for 5s)
	pauseTime := activateTime.Add(5 * time.Second)
	err := instance.Pause(pauseTime)
	require.NoError(t, err)

	// Resume at t+15s (paused for 10s)
	resumeTime := activateTime.Add(15 * time.Second)
	err = instance.Resume(resumeTime)
	require.NoError(t, err)

	// Check consumed at t+20s (active for 5 more seconds after resume)
	checkTime := activateTime.Add(20 * time.Second)
	consumed := instance.GetConsumedDurationSecondsAt(checkTime)

	// Wall clock: 20s, paused: 10s, expected consumed: 10s
	assert.Equal(t, 10, consumed,
		"Consumed should be wall_clock(20) - paused(10) = 10")
}

// ========================================================================
// PAUSED SWEEP: PERMANENT CONDITION DETECTED → STOP + FINALIZE
// ========================================================================

func TestP5BB_PausedSweep_PermanentCondition_StopsAndFinalizes(t *testing.T) {
	// Instance was paused (e.g., seller subscription expired).
	// While paused, for_sale was sold — permanent condition.
	// Safety worker should terminally stop and finalize.
	instance := createP5BBActiveInstance(t, 5*time.Second)

	// Pause first
	pauseTime := time.Now()
	err := instance.Pause(pauseTime)
	require.NoError(t, err)

	// Now stop terminally (for_sale sold while paused)
	stopTime := pauseTime.Add(10 * time.Second)
	err = instance.Stop(entity.StopReasonForSaleSold, stopTime)
	require.NoError(t, err)

	assert.Equal(t, entity.InstanceStatusCancelled, instance.Status)
	assert.True(t, instance.Status.IsTerminal())

	// Snapshot should capture consumed duration correctly
	consumed := instance.SnapshotConsumedDuration(stopTime)
	// Duration consumed = time before pause only (paused time excluded)
	assert.Greater(t, consumed, 0,
		"Should have consumed time before pause")
	assert.True(t, instance.Finalized,
		"Must finalize after terminal stop")
}

// ========================================================================
// PAUSED SWEEP: STILL INOPERABLE → NO ACTION
// ========================================================================

func TestP5BB_PausedSweep_StillReversiblyInoperable_KeepsPaused(t *testing.T) {
	// If a paused instance is still inoperable with a reversible reason,
	// the sweep should take NO action (keep paused).
	instance := createP5BBActiveInstance(t, 3*time.Second)

	pauseTime := time.Now()
	err := instance.Pause(pauseTime)
	require.NoError(t, err)

	// Verify the instance stays paused — no entity method should be called.
	// This test documents that the sweep's "else" branch is a no-op.
	assert.Equal(t, entity.InstanceStatusPaused, instance.Status)
	assert.False(t, instance.Finalized)
	assert.NotNil(t, instance.PausedAt)
}

// ========================================================================
// SELLER GOVERNANCE SPLIT: VERIFICATION SUSPENDED vs REVOKED
// ========================================================================

// ========================================================================
// REGRESSION: PAUSED STATUS IS NOT TERMINAL
// ========================================================================

func TestP5BB_PausedNotTerminal_CanTransitionToActive(t *testing.T) {
	assert.False(t, entity.InstanceStatusPaused.IsTerminal(),
		"Paused must not be terminal")
	assert.True(t, entity.InstanceStatusPaused.CanActivate(),
		"Paused must allow re-activation")
}

func TestP5BB_PausedNotTerminal_CanTransitionToCancelled(t *testing.T) {
	// Paused instance can be terminally stopped
	instance := createP5BBActiveInstance(t, 2*time.Second)
	err := instance.Pause(time.Now())
	require.NoError(t, err)

	err = instance.Stop(entity.StopReasonSellerGovernance, time.Now())
	require.NoError(t, err)

	assert.Equal(t, entity.InstanceStatusCancelled, instance.Status)
	assert.True(t, instance.Status.IsTerminal())
}

// ========================================================================
// CLASSIFICATION COMPLETENESS: ALL KNOWN REASONS
// ========================================================================

func TestP5BB_ClassificationCompleteness(t *testing.T) {
	// Regression lock: enumerate all known operability reasons
	// and verify each is classified correctly.
	reversible := map[string]bool{
		"seller_inactive":    true,
		"for_sale_hidden":    true,
		"for_sale_moderated": true,
	}

	permanent := []string{
		"seller_removed",
		"seller_account_inactive",
		"seller_not_found",
		"for_sale_sold",
		"for_sale_not_found",
		"for_sale_unavailable",
		"for_sale_expired",
		"for_sale_deleted",
		"auction_ended",
		"auction_cancelled",
		"auction_deleted",
		"auction_moderated",
		"auction_draft_not_promotable",
		"auction_unavailable",
		"user_removed",
		"user_account_inactive",
		"user_not_found",
	}

	for reason, expected := range reversible {
		assert.Equal(t, expected, application.IsReversibleReason(reason),
			"Reason %q should be reversible=%v", reason, expected)
	}

	for _, reason := range permanent {
		assert.False(t, application.IsReversibleReason(reason),
			"Reason %q should be permanent (not reversible)", reason)
	}
}

// ========================================================================
// HELPERS
// ========================================================================

func createP5BBActiveInstance(t *testing.T, elapsed time.Duration) *entity.PromotionInstance {
	t.Helper()
	activateTime := time.Now().Add(-elapsed)
	return createP5BBInstanceAt(t, activateTime)
}

func createP5BBInstanceAt(t *testing.T, activateTime time.Time) *entity.PromotionInstance {
	t.Helper()
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

	return instance
}
