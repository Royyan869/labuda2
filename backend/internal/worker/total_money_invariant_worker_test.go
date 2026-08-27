package worker

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
)

// ============================================================================
// TOTAL MONEY INVARIANT WORKER — UNIT TESTS
// ============================================================================
// These tests verify the worker lifecycle and config parsing without a database.
// The underlying checker logic is tested in wallet/application/total_money_invariant_checker_test.go.

// --- Start/Stop exits clean ---

func TestTotalMoneyInvariantWorker_StartStop_ExitsClean(t *testing.T) {
	w := NewTotalMoneyInvariantWorker(nil, zaptest.NewLogger(t), 100*time.Millisecond, true)
	require.False(t, w.IsRunning())

	w.Start()
	assert.True(t, w.IsRunning())

	w.Stop()
	assert.False(t, w.IsRunning(), "worker must not be running after Stop")
}

func TestTotalMoneyInvariantWorker_DoubleStart_Idempotent(t *testing.T) {
	w := NewTotalMoneyInvariantWorker(nil, zaptest.NewLogger(t), 100*time.Millisecond, true)

	w.Start()
	w.Start() // second call is no-op
	assert.True(t, w.IsRunning())

	w.Stop()
	assert.False(t, w.IsRunning())
}

func TestTotalMoneyInvariantWorker_DoubleStop_Safe(t *testing.T) {
	w := NewTotalMoneyInvariantWorker(nil, zaptest.NewLogger(t), 100*time.Millisecond, true)

	w.Start()
	w.Stop()
	w.Stop() // second call is safe no-op
	assert.False(t, w.IsRunning())
}

func TestTotalMoneyInvariantWorker_StopBeforeStart_Safe(t *testing.T) {
	w := NewTotalMoneyInvariantWorker(nil, zap.NewNop(), 100*time.Millisecond, true)
	w.Stop() // safe to call before Start
	assert.False(t, w.IsRunning())
}

// --- Ticker invokes checker (shadow logs don't panic) ---

func TestTotalMoneyInvariantWorker_CheckOnce_NilChecker_NoPanic(t *testing.T) {
	// When checker is nil, checkOnce should not panic — it logs a warn and returns.
	w := NewTotalMoneyInvariantWorker(nil, zaptest.NewLogger(t), 100*time.Millisecond, true)

	w.Start()
	// Give the ticker one chance to fire
	time.Sleep(50 * time.Millisecond)
	w.Stop()
	// If we get here without panic, the test passes.
}

// --- alert_suppressed log field reflects real shadow mode (PASS_18R) ---

// TestTotalMoneyInvariantWorker_LogCheckCompleted_AlertSuppressedReflectsShadowMode
// mirrors the escrow worker's PASS_18R regression test: the alert_suppressed
// log field must track the worker's real shadowMode value, never be
// hardcoded true.
func TestTotalMoneyInvariantWorker_LogCheckCompleted_AlertSuppressedReflectsShadowMode(t *testing.T) {
	t.Run("shadow mode true reports alert_suppressed=true", func(t *testing.T) {
		core, observed := observer.New(zap.InfoLevel)
		w := NewTotalMoneyInvariantWorker(nil, zap.New(core), time.Minute, true)

		w.logCheckCompleted(false, time.Millisecond)

		entries := observed.TakeAll()
		require.Len(t, entries, 1)
		suppressed, ok := entries[0].ContextMap()["alert_suppressed"].(bool)
		require.True(t, ok, "alert_suppressed field must be present and boolean")
		assert.True(t, suppressed)
	})

	t.Run("shadow mode false reports alert_suppressed=false", func(t *testing.T) {
		core, observed := observer.New(zap.InfoLevel)
		w := NewTotalMoneyInvariantWorker(nil, zap.New(core), time.Minute, false)

		w.logCheckCompleted(false, time.Millisecond)

		entries := observed.TakeAll()
		require.Len(t, entries, 1)
		suppressed, ok := entries[0].ContextMap()["alert_suppressed"].(bool)
		require.True(t, ok, "alert_suppressed field must be present and boolean")
		assert.False(t, suppressed, "alert_suppressed must not be hardcoded true when shadow mode is off")
	})
}

// --- Constructor defaults ---

func TestTotalMoneyInvariantWorker_NilLogger_Defaults(t *testing.T) {
	w := NewTotalMoneyInvariantWorker(nil, nil, DefaultTotalMoneyInvariantInterval, true)
	require.NotNil(t, w.logger, "nil logger must fall back to zap.NewNop()")
}

func TestTotalMoneyInvariantWorker_ZeroInterval_Defaults(t *testing.T) {
	w := NewTotalMoneyInvariantWorker(nil, nil, 0, true)
	assert.Equal(t, DefaultTotalMoneyInvariantInterval, w.interval,
		"zero interval must fall back to default")
}

func TestTotalMoneyInvariantWorker_NegativeInterval_Defaults(t *testing.T) {
	w := NewTotalMoneyInvariantWorker(nil, nil, -5*time.Minute, true)
	assert.Equal(t, DefaultTotalMoneyInvariantInterval, w.interval,
		"negative interval must fall back to default")
}

// --- Config parsing ---

// TestParseTotalMoneyInvariantConfig_Defaults is the PASS_18R regression
// test: this money-safety detector must default to ENABLED (in shadow
// mode), not disabled. Before this pass, the default silently produced a
// fully dark detector with no drift protection at all.
func TestParseTotalMoneyInvariantConfig_Defaults(t *testing.T) {
	// Ensure env vars are unset for this test
	os.Unsetenv("DISABLE_TOTAL_MONEY_INVARIANT_WORKER")
	os.Unsetenv("TOTAL_MONEY_INVARIANT_INTERVAL_MINUTES")
	os.Unsetenv("TOTAL_MONEY_INVARIANT_SHADOW_MODE")

	cfg := ParseTotalMoneyInvariantConfig()

	assert.False(t, cfg.Disabled, "default must be enabled (PASS_18R: detector must not be silently dormant)")
	assert.Equal(t, DefaultTotalMoneyInvariantInterval, cfg.Interval, "default interval must be 15m")
	assert.True(t, cfg.ShadowMode, "default must be shadow mode")
}

func TestParseTotalMoneyInvariantConfig_Enabled(t *testing.T) {
	t.Setenv("DISABLE_TOTAL_MONEY_INVARIANT_WORKER", "false")
	t.Setenv("TOTAL_MONEY_INVARIANT_INTERVAL_MINUTES", "30")
	t.Setenv("TOTAL_MONEY_INVARIANT_SHADOW_MODE", "false")

	cfg := ParseTotalMoneyInvariantConfig()

	assert.False(t, cfg.Disabled, "DISABLE=false should enable")
	assert.Equal(t, 30*time.Minute, cfg.Interval, "interval should be 30 minutes")
	assert.False(t, cfg.ShadowMode, "shadow=false should disable shadow mode")
}

func TestParseTotalMoneyInvariantConfig_IntervalParsing(t *testing.T) {
	tests := []struct {
		name     string
		envVal   string
		expected time.Duration
	}{
		{"valid_5", "5", 5 * time.Minute},
		{"valid_60", "60", 60 * time.Minute},
		{"zero_fallback", "0", DefaultTotalMoneyInvariantInterval},
		{"negative_fallback", "-1", DefaultTotalMoneyInvariantInterval},
		{"invalid_fallback", "abc", DefaultTotalMoneyInvariantInterval},
		{"empty_fallback", "", DefaultTotalMoneyInvariantInterval},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv("DISABLE_TOTAL_MONEY_INVARIANT_WORKER")
			os.Unsetenv("TOTAL_MONEY_INVARIANT_SHADOW_MODE")
			if tt.envVal == "" {
				os.Unsetenv("TOTAL_MONEY_INVARIANT_INTERVAL_MINUTES")
			} else {
				t.Setenv("TOTAL_MONEY_INVARIANT_INTERVAL_MINUTES", tt.envVal)
			}

			cfg := ParseTotalMoneyInvariantConfig()
			assert.Equal(t, tt.expected, cfg.Interval, "interval mismatch for %q", tt.envVal)
		})
	}
}

func TestParseTotalMoneyInvariantConfig_DisabledVariants(t *testing.T) {
	variants := map[string]bool{
		"true":  true,
		"TRUE":  true,
		"1":     true,
		"yes":   true,
		"false": false,
		"FALSE": false,
		"0":     false,
		"no":    false,
		"off":   false,
	}

	for envVal, expectDisabled := range variants {
		t.Run(envVal, func(t *testing.T) {
			t.Setenv("DISABLE_TOTAL_MONEY_INVARIANT_WORKER", envVal)
			os.Unsetenv("TOTAL_MONEY_INVARIANT_INTERVAL_MINUTES")
			os.Unsetenv("TOTAL_MONEY_INVARIANT_SHADOW_MODE")

			cfg := ParseTotalMoneyInvariantConfig()
			assert.Equal(t, expectDisabled, cfg.Disabled, "DISABLE=%s", envVal)
		})
	}
}

func TestParseTotalMoneyInvariantConfig_ShadowVariants(t *testing.T) {
	variants := map[string]bool{
		"true":  true,
		"TRUE":  true,
		"1":     true,
		"yes":   true,
		"false": false,
		"FALSE": false,
		"0":     false,
		"no":    false,
		"off":   false,
	}

	for envVal, expectShadow := range variants {
		t.Run(envVal, func(t *testing.T) {
			os.Unsetenv("DISABLE_TOTAL_MONEY_INVARIANT_WORKER")
			os.Unsetenv("TOTAL_MONEY_INVARIANT_INTERVAL_MINUTES")
			t.Setenv("TOTAL_MONEY_INVARIANT_SHADOW_MODE", envVal)

			cfg := ParseTotalMoneyInvariantConfig()
			assert.Equal(t, expectShadow, cfg.ShadowMode, "SHADOW=%s", envVal)
		})
	}
}


