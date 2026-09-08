package application

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ============================================================================
// DURATION AUTHORITY — SINGLE CANONICAL INPUT (FINDING B)
//
// Structural proof that the seller duration (DurationDays) is the ONLY
// creation input that can influence planned_finish: CreatePromotionInput
// carries no planned_finish / planned_start field, so no canonical creation
// path can combine duration_days A with a planned_finish corresponding to
// duration B. planned_finish is server-derived from the same validated
// DurationDays used by the minimum-budget branch.
// ============================================================================

func TestCreatePromotionInput_HasSingleDurationAuthority(t *testing.T) {
	tpe := reflect.TypeOf(CreatePromotionInput{})
	require.Equal(t, reflect.Struct, tpe.Kind())

	for i := 0; i < tpe.NumField(); i++ {
		require.NotEqualf(t, "PlannedFinish", tpe.Field(i).Name,
			"CreatePromotionInput must not carry an independently writable planned_finish")
		require.NotEqualf(t, "PlannedStart", tpe.Field(i).Name,
			"CreatePromotionInput must not carry an independently writable planned_start")
	}

	fields := map[string]bool{}
	for i := 0; i < tpe.NumField(); i++ {
		fields[tpe.Field(i).Name] = true
	}
	require.True(t, fields["DurationDays"], "DurationDays must remain the seller duration input")
	require.True(t, fields["BudgetRupiah"], "BudgetRupiah must remain the seller budget input")
}

// ============================================================================
// TECHNICAL OVERFLOW BOUND (FINDING A)
//
// Proves that the only remaining duration bound is the actual time.Duration
// representation constraint (~292 years), derived from the implementation's
// arithmetic — NOT an invented business/product duration maximum such as the
// removed 3650-day cap.
// ============================================================================

func TestMaxContractDurationDays_IsTechnicalOverflowBound(t *testing.T) {
	// Exact derivation from the implementation constraint: planned_finish =
	// planned_start + duration_days x 24h runs in a Go time.Duration (int64
	// nanoseconds), so the largest safe whole-day duration is MaxInt64 / 24h.
	require.Equal(t, math.MaxInt64/int64(24*time.Hour), maxContractDurationDays)

	// The technical bound is far above the previously invented 3650-day cap —
	// durations between 3651 and the bound are accepted (see the real-DB
	// proof), so 3650 was never a genuine safety constraint.
	require.Greater(t, maxContractDurationDays, int64(3650))

	// At the bound, duration x 24h is exactly representable (positive).
	d := time.Duration(maxContractDurationDays) * 24 * time.Hour
	require.Greater(t, int64(d), int64(0))

	// One day past the bound the SAME arithmetic silently wraps at runtime
	// (days is a runtime variable so no constant folding hides the wrap).
	// This is the genuine overflow point the guard protects: without the
	// rejection a contract could be persisted with a wrong/backwards
	// planned_finish. The guard is a technical safety constraint, not policy.
	days := maxContractDurationDays
	days++
	wrapped := time.Duration(days) * 24 * time.Hour
	require.Less(t, int64(wrapped), int64(0),
		"one day past the technical bound the duration x 24h multiplication must wrap")
}

func TestMulNonNeg_FailsClosedOnOverflow(t *testing.T) {
	// Normal multiplication is exact.
	got, overflow := mulNonNeg(10_000, 4000)
	require.False(t, overflow)
	require.Equal(t, int64(40_000_000), got)

	// min_daily_budget x duration_days overflow is REPORTED, never wrapped.
	_, overflow = mulNonNeg(math.MaxInt64/2, 3)
	require.True(t, overflow, "int64 overflow must be reported so creation fails closed")

	// Boundary precision: MaxInt64 x 1 is fine, MaxInt64 x 2 overflows.
	_, overflow = mulNonNeg(math.MaxInt64, 1)
	require.False(t, overflow)
	_, overflow = mulNonNeg(math.MaxInt64, 2)
	require.True(t, overflow)
}
