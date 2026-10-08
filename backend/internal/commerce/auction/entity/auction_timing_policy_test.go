package entity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// ValidateAuctionTiming
// ============================================================================

func TestValidateAuctionTiming(t *testing.T) {
	now := time.Now()

	t.Run("rejects end_at not after start_at", func(t *testing.T) {
		err := ValidateAuctionTiming(now, now)
		assert.ErrorIs(t, err, ErrEndAtNotAfterStartAt)
	})

	t.Run("rejects end_at before start_at", func(t *testing.T) {
		err := ValidateAuctionTiming(now, now.Add(-time.Hour))
		assert.ErrorIs(t, err, ErrEndAtNotAfterStartAt)
	})

	t.Run("rejects duration below minimum", func(t *testing.T) {
		err := ValidateAuctionTiming(now, now.Add(MinAuctionDuration-time.Second))
		var durErr *ErrAuctionDurationOutOfRange
		require.ErrorAs(t, err, &durErr)
	})

	t.Run("rejects duration above maximum", func(t *testing.T) {
		err := ValidateAuctionTiming(now, now.Add(MaxAuctionDuration+time.Second))
		var durErr *ErrAuctionDurationOutOfRange
		require.ErrorAs(t, err, &durErr)
	})

	t.Run("accepts duration at minimum boundary (1 day)", func(t *testing.T) {
		err := ValidateAuctionTiming(now, now.Add(MinAuctionDuration))
		assert.NoError(t, err)
	})

	t.Run("accepts duration at maximum boundary (7 days)", func(t *testing.T) {
		err := ValidateAuctionTiming(now, now.Add(MaxAuctionDuration))
		assert.NoError(t, err)
	})

	t.Run("accepts duration in the middle of the range (3 days)", func(t *testing.T) {
		err := ValidateAuctionTiming(now, now.Add(3*24*time.Hour))
		assert.NoError(t, err)
	})
}

// ============================================================================
// ResolveAuctionTiming
// ============================================================================

func TestResolveAuctionTiming_Now(t *testing.T) {
	now := time.Now()

	t.Run("immediate start uses server now, end = now + duration", func(t *testing.T) {
		startAt, endAt, err := ResolveAuctionTiming(StartModeNow, nil, 24*time.Hour, now)
		require.NoError(t, err)
		assert.Equal(t, now, startAt)
		assert.Equal(t, now.Add(24*time.Hour), endAt)
	})

	t.Run("immediate start ignores a stale client-provided scheduled_start_at", func(t *testing.T) {
		past := now.Add(-48 * time.Hour)
		startAt, _, err := ResolveAuctionTiming(StartModeNow, &past, 24*time.Hour, now)
		require.NoError(t, err)
		assert.Equal(t, now, startAt, "start_mode=now must use server time, never client input")
	})

	t.Run("rejects duration too short", func(t *testing.T) {
		_, _, err := ResolveAuctionTiming(StartModeNow, nil, MinAuctionDuration-time.Minute, now)
		var durErr *ErrAuctionDurationOutOfRange
		require.ErrorAs(t, err, &durErr)
	})

	t.Run("rejects duration too long", func(t *testing.T) {
		_, _, err := ResolveAuctionTiming(StartModeNow, nil, MaxAuctionDuration+time.Minute, now)
		var durErr *ErrAuctionDurationOutOfRange
		require.ErrorAs(t, err, &durErr)
	})
}

func TestResolveAuctionTiming_Scheduled(t *testing.T) {
	now := time.Now()

	t.Run("scheduled start in the future succeeds", func(t *testing.T) {
		future := now.Add(48 * time.Hour)
		startAt, endAt, err := ResolveAuctionTiming(StartModeScheduled, &future, 72*time.Hour, now)
		require.NoError(t, err)
		assert.Equal(t, future, startAt)
		assert.Equal(t, future.Add(72*time.Hour), endAt)
	})

	t.Run("requires scheduled_start_at", func(t *testing.T) {
		_, _, err := ResolveAuctionTiming(StartModeScheduled, nil, 24*time.Hour, now)
		assert.ErrorIs(t, err, ErrScheduledStartRequired)
	})

	t.Run("rejects scheduled start in the past", func(t *testing.T) {
		past := now.Add(-time.Hour)
		_, _, err := ResolveAuctionTiming(StartModeScheduled, &past, 24*time.Hour, now)
		var futureErr *ErrScheduledStartMustBeFuture
		require.ErrorAs(t, err, &futureErr)
	})

	t.Run("tolerates small clock skew (near-now scheduled start)", func(t *testing.T) {
		nearNow := now.Add(-30 * time.Second) // within scheduledStartClockSkewTolerance (1m)
		_, _, err := ResolveAuctionTiming(StartModeScheduled, &nearNow, 24*time.Hour, now)
		assert.NoError(t, err)
	})

	t.Run("rejects duration out of range even with a valid future start", func(t *testing.T) {
		future := now.Add(48 * time.Hour)
		_, _, err := ResolveAuctionTiming(StartModeScheduled, &future, MaxAuctionDuration+time.Hour, now)
		var durErr *ErrAuctionDurationOutOfRange
		require.ErrorAs(t, err, &durErr)
	})
}

func TestResolveAuctionTiming_InvalidMode(t *testing.T) {
	_, _, err := ResolveAuctionTiming(StartMode("bogus"), nil, 24*time.Hour, time.Now())
	assert.ErrorIs(t, err, ErrInvalidStartMode)
}

// ============================================================================
// RequireFutureAuctionEnd (fix #2: schedule gate on a past end_at)
// ============================================================================

func TestRequireFutureAuctionEnd(t *testing.T) {
	now := time.Now()

	t.Run("rejects end_at in the past", func(t *testing.T) {
		err := RequireFutureAuctionEnd(now.Add(-time.Hour), now)
		var endErr *ErrAuctionEndAlreadyPassed
		require.ErrorAs(t, err, &endErr)
		assert.Equal(t, now.Add(-time.Hour), endErr.EndAt)
	})

	t.Run("accepts end_at exactly now (inside skew window)", func(t *testing.T) {
		// now is 0s before now — within the 1m skew tolerance, same as the
		// near-now scheduled start case above.
		err := RequireFutureAuctionEnd(now, now)
		assert.NoError(t, err)
	})

	t.Run("tolerates end_at within clock-skew tolerance", func(t *testing.T) {
		// 30s before now is inside scheduledStartClockSkewTolerance (1m).
		err := RequireFutureAuctionEnd(now.Add(-30*time.Second), now)
		assert.NoError(t, err)
	})

	t.Run("accepts end_at beyond tolerance", func(t *testing.T) {
		assert.NoError(t, RequireFutureAuctionEnd(now.Add(24*time.Hour), now))
	})

	t.Run("rejects end_at beyond tolerance in the past", func(t *testing.T) {
		err := RequireFutureAuctionEnd(now.Add(-2*time.Minute), now)
		assert.Error(t, err)
	})
}

// ============================================================================
// UpdateScheduled timing enforcement (PASS_18C)
// ============================================================================

func TestUpdateScheduled_RejectsInvalidTiming(t *testing.T) {
	t.Run("rejects end_at <= start_at", func(t *testing.T) {
		auction := createTestAuction()
		auction.Status = StatusScheduled
		start := time.Now().Add(time.Hour)
		err := auction.UpdateScheduled(start, start)
		assert.ErrorIs(t, err, ErrEndAtNotAfterStartAt)
	})

	t.Run("rejects duration below 1 day", func(t *testing.T) {
		auction := createTestAuction()
		auction.Status = StatusScheduled
		start := time.Now().Add(time.Hour)
		err := auction.UpdateScheduled(start, start.Add(23*time.Hour))
		var durErr *ErrAuctionDurationOutOfRange
		assert.ErrorAs(t, err, &durErr)
	})

	t.Run("rejects duration above 7 days", func(t *testing.T) {
		auction := createTestAuction()
		auction.Status = StatusScheduled
		start := time.Now().Add(time.Hour)
		err := auction.UpdateScheduled(start, start.Add(8*24*time.Hour))
		var durErr *ErrAuctionDurationOutOfRange
		assert.ErrorAs(t, err, &durErr)
	})

	t.Run("rejects a past start_at", func(t *testing.T) {
		auction := createTestAuction()
		auction.Status = StatusScheduled
		past := time.Now().Add(-time.Hour)
		err := auction.UpdateScheduled(past, past.Add(48*time.Hour))
		var futureErr *ErrScheduledStartMustBeFuture
		assert.ErrorAs(t, err, &futureErr)
	})

	t.Run("accepts valid future timing within bounds", func(t *testing.T) {
		auction := createTestAuction()
		auction.Status = StatusScheduled
		start := time.Now().Add(2 * time.Hour)
		err := auction.UpdateScheduled(start, start.Add(3*24*time.Hour))
		assert.NoError(t, err)
	})
}
