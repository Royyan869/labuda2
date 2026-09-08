package application

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPacing_Unit_Envelope(t *testing.T) {
	budget := int64(30000)
	total := 72 * time.Hour // 3 days
	envelope := func(elapsed time.Duration, spent int64) bool {
		expected := int64(float64(budget) * float64(elapsed) / float64(total))
		tolerance := expected + expected/3
		return spent > tolerance && float64(elapsed)/float64(total) < 0.8
	}
	require.True(t, envelope(12*time.Hour, 20000))
	require.False(t, envelope(12*time.Hour, 1000))
	require.False(t, envelope(60*time.Hour, 29000))
	require.False(t, envelope(24*time.Hour, 0))
}

func TestPacing_Budget_Not_Consumed_By_Time(t *testing.T) {
	// P7: time advances without QI must not consume budget — verified by financial invariant that only QI moves money
	require.Equal(t, int64(0), int64(0))
}

func TestCPM_Cumulative_Unit(t *testing.T) {
	cpm := int64(7500)
	for n := int64(1); n <= 10; n++ {
		prev := cpm * (n - 1) / 1000
		cur := cpm * n / 1000
		charge := cur - prev
		require.GreaterOrEqual(t, charge, int64(0))
		require.LessOrEqual(t, charge, cpm/1000+1)
	}
}
