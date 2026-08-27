package worker

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func unsetCriticalWorkerEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"DISABLE_ESCROW_INTEGRITY_WORKER",
		"ESCROW_INTEGRITY_SHADOW_MODE",
		"DISABLE_TOTAL_MONEY_INVARIANT_WORKER",
		"TOTAL_MONEY_INVARIANT_SHADOW_MODE",
	} {
		os.Unsetenv(k)
	}
}

// TestEscrowIntegrityWorkerStatus_DefaultIsShadowNotDark is the PASS_18R
// regression test: with no env vars set at all, the detector must report as
// "shadow" (enabled, alerts suppressed) — never "dark" (disabled). Before
// this pass, the default was Disabled=true, which would have produced
// status="dark" here.
func TestEscrowIntegrityWorkerStatus_DefaultIsShadowNotDark(t *testing.T) {
	unsetCriticalWorkerEnv(t)

	status := EscrowIntegrityWorkerStatus()

	assert.Equal(t, "EscrowIntegrityWorker", status.Name)
	require.True(t, status.Enabled, "must be enabled by default")
	assert.True(t, status.ShadowMode, "must default to shadow mode")
	assert.True(t, status.Critical)
	assert.Equal(t, "shadow", status.Status)
	assert.NotEqual(t, "dark", status.Status)
}

// TestTotalMoneyInvariantWorkerStatus_DefaultIsShadowNotDark mirrors the
// escrow test for TotalMoneyInvariantWorker.
func TestTotalMoneyInvariantWorkerStatus_DefaultIsShadowNotDark(t *testing.T) {
	unsetCriticalWorkerEnv(t)

	status := TotalMoneyInvariantWorkerStatus()

	assert.Equal(t, "TotalMoneyInvariantWorker", status.Name)
	require.True(t, status.Enabled, "must be enabled by default")
	assert.True(t, status.ShadowMode, "must default to shadow mode")
	assert.True(t, status.Critical)
	assert.Equal(t, "shadow", status.Status)
	assert.NotEqual(t, "dark", status.Status)
}

func TestCriticalWorkerStatusFrom_AllThreeStates(t *testing.T) {
	cases := []struct {
		name       string
		disabled   bool
		shadowMode bool
		wantStatus string
	}{
		{"disabled wins regardless of shadow flag", true, true, "dark"},
		{"disabled wins even with shadow off", true, false, "dark"},
		{"enabled + shadow = shadow", false, true, "shadow"},
		{"enabled + no shadow = active", false, false, "active"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := criticalWorkerStatusFrom("TestWorker", tc.disabled, tc.shadowMode)
			assert.Equal(t, tc.wantStatus, got.Status)
			assert.Equal(t, !tc.disabled, got.Enabled)
			assert.True(t, got.Critical)
			assert.NotEmpty(t, got.Reason)
		})
	}
}

func TestAnyCriticalWorkerDark(t *testing.T) {
	activeStatus := criticalWorkerStatusFrom("A", false, false)
	shadowStatus := criticalWorkerStatusFrom("B", false, true)
	darkStatus := criticalWorkerStatusFrom("C", true, true)

	assert.False(t, AnyCriticalWorkerDark([]CriticalWorkerStatus{activeStatus, shadowStatus}),
		"active+shadow must not count as dark")
	assert.True(t, AnyCriticalWorkerDark([]CriticalWorkerStatus{activeStatus, darkStatus}),
		"any disabled worker must count as dark")
	assert.False(t, AnyCriticalWorkerDark(nil), "empty set must not be dark")
}

// TestCriticalWorkerStatuses_ExplicitDisable proves an operator explicitly
// opting out via DISABLE_*=true is correctly reported as dark — this also
// regression-locks the "true"/"1"/"yes"/"on" branch added to the parse
// switch when the default flipped to enabled (PASS_18R).
func TestCriticalWorkerStatuses_ExplicitDisable(t *testing.T) {
	unsetCriticalWorkerEnv(t)
	t.Setenv("DISABLE_ESCROW_INTEGRITY_WORKER", "true")
	t.Setenv("DISABLE_TOTAL_MONEY_INVARIANT_WORKER", "true")

	statuses := CriticalWorkerStatuses()
	require.Len(t, statuses, 2)
	for _, s := range statuses {
		assert.False(t, s.Enabled, "%s: explicit DISABLE=true must be honored", s.Name)
		assert.Equal(t, "dark", s.Status)
	}
	assert.True(t, AnyCriticalWorkerDark(statuses))
}
