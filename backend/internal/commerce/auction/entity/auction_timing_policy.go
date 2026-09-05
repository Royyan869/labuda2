package entity

import (
	"fmt"
	"time"
)

const (
	// MinAuctionDuration is the minimum time an auction may run (owner-approved).
	MinAuctionDuration = 24 * time.Hour
	// MaxAuctionDuration is the maximum time an auction may run (owner-approved).
	MaxAuctionDuration = 7 * 24 * time.Hour

	// AntiSnipingWindow is how close to EndAt a bid must land to trigger a
	// soft-close extension (owner-approved).
	AntiSnipingWindow = 5 * time.Minute
	// AntiSnipingExtension is how far EndAt is pushed back per triggering bid.
	AntiSnipingExtension = 5 * time.Minute
	// MaxAntiSnipingTotalExtension caps the cumulative soft-close extension an
	// auction may receive, regardless of how many late bids land.
	MaxAntiSnipingTotalExtension = 30 * time.Minute

	// scheduledStartClockSkewTolerance absorbs request/network latency so a
	// scheduled start requested for "now" isn't rejected for landing a few
	// hundred milliseconds in the past by the time it reaches the server.
	scheduledStartClockSkewTolerance = 1 * time.Minute

	// MaxScheduledStartHorizon is the maximum time in the future a scheduled
	// auction may be set to start, measured from the authoritative server time
	// supplied to the timing resolution path. Owner-approved: exactly 30 days
	// (720 hours). This is the single canonical domain authority for the
	// scheduled-start horizon — no handler, service, or worker may duplicate
	// this policy.
	MaxScheduledStartHorizon = 30 * 24 * time.Hour
)

// StartMode selects how a newly created auction begins its lifecycle.
type StartMode string

const (
	// StartModeNow starts the auction immediately using server time.
	StartModeNow StartMode = "now"
	// StartModeScheduled starts the auction at a seller-chosen future time.
	StartModeScheduled StartMode = "scheduled"
)

// ErrEndAtNotAfterStartAt is returned when end_at does not strictly follow start_at.
var ErrEndAtNotAfterStartAt = fmt.Errorf("end_at must be after start_at")

// ErrScheduledStartRequired is returned when start_mode=scheduled but no
// scheduled start time was provided.
var ErrScheduledStartRequired = fmt.Errorf("scheduled_start_at is required when start_mode is scheduled")

// ErrInvalidStartMode is returned for an unrecognized start_mode value.
var ErrInvalidStartMode = fmt.Errorf(`invalid start_mode: must be "now" or "scheduled"`)

// ErrAuctionDurationOutOfRange is returned when an auction's duration
// (end_at - start_at) falls outside the owner-approved bounds.
type ErrAuctionDurationOutOfRange struct {
	Duration time.Duration
	Min      time.Duration
	Max      time.Duration
}

func (e *ErrAuctionDurationOutOfRange) Error() string {
	return fmt.Sprintf("auction duration %s is out of range: must be between %s and %s", e.Duration, e.Min, e.Max)
}

// ErrScheduledStartMustBeFuture is returned when a scheduled auction start
// time is not in the future (beyond a small clock-skew tolerance).
type ErrScheduledStartMustBeFuture struct {
	ScheduledStartAt time.Time
	Now              time.Time
}

func (e *ErrScheduledStartMustBeFuture) Error() string {
	return fmt.Sprintf("scheduled start_at %s must be in the future (now: %s)",
		e.ScheduledStartAt.Format(time.RFC3339), e.Now.Format(time.RFC3339))
}

// ErrScheduledStartBeyondHorizon is returned when a scheduled auction start
// time exceeds the owner-approved maximum horizon (30 days from server now).
type ErrScheduledStartBeyondHorizon struct {
	ScheduledStartAt time.Time
	Now              time.Time
	MaxHorizon       time.Duration
}

func (e *ErrScheduledStartBeyondHorizon) Error() string {
	return fmt.Sprintf("scheduled start_at %s exceeds maximum horizon of %s from now (%s)",
		e.ScheduledStartAt.Format(time.RFC3339), e.MaxHorizon, e.Now.Format(time.RFC3339))
}

// ValidateAuctionTiming enforces the owner-approved auction timing invariants
// that must hold for any create or update of start_at/end_at:
//   - end_at must be strictly after start_at;
//   - the resulting duration must be within [MinAuctionDuration, MaxAuctionDuration].
//
// This is the single source of truth for timing bounds — it is called from
// ResolveAuctionTiming (create path) and directly from the UpdateDraft/
// UpdateScheduled entity methods, so no create/update path can bypass it by
// calling a service method directly. UI validation is a convenience only.
func ValidateAuctionTiming(startAt, endAt time.Time) error {
	if !endAt.After(startAt) {
		return ErrEndAtNotAfterStartAt
	}
	duration := endAt.Sub(startAt)
	if duration < MinAuctionDuration || duration > MaxAuctionDuration {
		return &ErrAuctionDurationOutOfRange{Duration: duration, Min: MinAuctionDuration, Max: MaxAuctionDuration}
	}
	return nil
}

// RequireFutureScheduledStart validates that startAt is in the future (beyond
// a small clock-skew tolerance). Shared by ResolveAuctionTiming (create) and
// UpdateScheduled (entity method) so a scheduled auction's start can never be
// edited into the past.
func RequireFutureScheduledStart(startAt, now time.Time) error {
	if !startAt.After(now.Add(-scheduledStartClockSkewTolerance)) {
		return &ErrScheduledStartMustBeFuture{ScheduledStartAt: startAt, Now: now}
	}
	return nil
}

// RequireScheduledStartWithinHorizon validates that startAt does not exceed
// the owner-approved 30-day horizon from the authoritative server time.
func RequireScheduledStartWithinHorizon(startAt, now time.Time) error {
	if startAt.After(now.Add(MaxScheduledStartHorizon)) {
		return &ErrScheduledStartBeyondHorizon{ScheduledStartAt: startAt, Now: now, MaxHorizon: MaxScheduledStartHorizon}
	}
	return nil
}

// ResolveAuctionTiming computes start_at/end_at from the seller's chosen
// start mode and duration, validating both against the owner-approved
// duration bounds and (for scheduled starts) the future-start invariant.
//
// now must always be the server clock — never client-supplied — so an
// immediate-start request cannot be backdated by a client with a skewed
// clock, and duration is always measured from a trustworthy origin.
func ResolveAuctionTiming(mode StartMode, scheduledStartAt *time.Time, duration time.Duration, now time.Time) (startAt, endAt time.Time, err error) {
	var start time.Time
	switch mode {
	case StartModeNow:
		start = now
	case StartModeScheduled:
		if scheduledStartAt == nil {
			return time.Time{}, time.Time{}, ErrScheduledStartRequired
		}
		if err := RequireFutureScheduledStart(*scheduledStartAt, now); err != nil {
			return time.Time{}, time.Time{}, err
		}
		if err := RequireScheduledStartWithinHorizon(*scheduledStartAt, now); err != nil {
			return time.Time{}, time.Time{}, err
		}
		start = *scheduledStartAt
	default:
		return time.Time{}, time.Time{}, ErrInvalidStartMode
	}

	end := start.Add(duration)
	if err := ValidateAuctionTiming(start, end); err != nil {
		return time.Time{}, time.Time{}, err
	}
	return start, end, nil
}
