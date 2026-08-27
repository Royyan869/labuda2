package main

import (
	"testing"

	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/internal/worker"
)

func activeStatus(name string) worker.CriticalWorkerStatus {
	return worker.CriticalWorkerStatus{Name: name, Enabled: true, ShadowMode: false, Critical: true, Status: "active"}
}

func shadowStatus(name string) worker.CriticalWorkerStatus {
	return worker.CriticalWorkerStatus{Name: name, Enabled: true, ShadowMode: true, Critical: true, Status: "shadow"}
}

func darkStatus(name string) worker.CriticalWorkerStatus {
	return worker.CriticalWorkerStatus{Name: name, Enabled: false, ShadowMode: true, Critical: true, Status: "dark"}
}

// safePayoutStatus is a neutral (non-degraded) payout safety status used by
// tests that are only exercising worker-status degradation, not payout
// safety specifically.
func safePayoutStatus() config.PayoutCompletionSafety {
	return config.PayoutCompletionSafety{}
}

func unsafePayoutStatus() config.PayoutCompletionSafety {
	return config.PayoutCompletionSafety{
		PayoutWorkerEnabled: true,
		Degraded:            true,
		Reason:              "test: no completion path configured",
	}
}

// TestEvaluateReadiness_AllHealthy_NoDarkWorkers proves the baseline healthy
// case reports ready=true, degraded=false regardless of environment.
func TestEvaluateReadiness_AllHealthy_NoDarkWorkers(t *testing.T) {
	statuses := []worker.CriticalWorkerStatus{activeStatus("A"), shadowStatus("B")}

	for _, env := range []string{"development", "staging", "production"} {
		cfg := &config.Config{Server: config.ServerConfig{Env: env}}
		ready, degraded := evaluateReadiness(cfg, true, true, statuses, safePayoutStatus())
		if !ready {
			t.Errorf("env=%s: expected ready=true when infra OK and no dark workers", env)
		}
		if degraded {
			t.Errorf("env=%s: expected degraded=false when no dark workers", env)
		}
	}
}

// TestEvaluateReadiness_InfraDown_AlwaysFailsReadiness proves DB/Redis
// failures fail readiness in every environment, independent of worker state.
func TestEvaluateReadiness_InfraDown_AlwaysFailsReadiness(t *testing.T) {
	statuses := []worker.CriticalWorkerStatus{activeStatus("A")}

	for _, env := range []string{"development", "staging", "production"} {
		cfg := &config.Config{Server: config.ServerConfig{Env: env}}

		if ready, _ := evaluateReadiness(cfg, false, true, statuses, safePayoutStatus()); ready {
			t.Errorf("env=%s: expected ready=false when DB is down", env)
		}
		if ready, _ := evaluateReadiness(cfg, true, false, statuses, safePayoutStatus()); ready {
			t.Errorf("env=%s: expected ready=false when Redis is down", env)
		}
	}
}

// TestEvaluateReadiness_DarkCriticalWorker_DevelopmentStaysReady is the
// PASS_18R requirement that local/dev must remain usable (not block boot)
// even when a critical detector is dark, but the degradation must still be
// visible in the response.
func TestEvaluateReadiness_DarkCriticalWorker_DevelopmentStaysReady(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{Env: "development"}}
	statuses := []worker.CriticalWorkerStatus{darkStatus("EscrowIntegrityWorker")}

	ready, degraded := evaluateReadiness(cfg, true, true, statuses, safePayoutStatus())

	if !ready {
		t.Error("development must remain ready=true even with a dark critical worker (must not block local boot)")
	}
	if !degraded {
		t.Error("degraded must be true so the dark worker is still visible in the response")
	}
}

// TestEvaluateReadiness_DarkCriticalWorker_StagingAndProductionFailReadiness
// is the PASS_18R core fix: readiness must not falsely imply "all safe" when
// a critical money detector is dark in a production-like environment.
func TestEvaluateReadiness_DarkCriticalWorker_StagingAndProductionFailReadiness(t *testing.T) {
	statuses := []worker.CriticalWorkerStatus{darkStatus("TotalMoneyInvariantWorker")}

	for _, env := range []string{"staging", "production"} {
		cfg := &config.Config{Server: config.ServerConfig{Env: env}}
		ready, degraded := evaluateReadiness(cfg, true, true, statuses, safePayoutStatus())

		if ready {
			t.Errorf("env=%s: expected ready=false when a critical detector is dark", env)
		}
		if !degraded {
			t.Errorf("env=%s: expected degraded=true", env)
		}
	}
}

// TestEvaluateReadiness_ShadowModeIsNotDark proves shadow mode (enabled,
// alerts suppressed) is a deliberate staged-activation state and must NOT by
// itself degrade or fail readiness in any environment.
func TestEvaluateReadiness_ShadowModeIsNotDark(t *testing.T) {
	statuses := []worker.CriticalWorkerStatus{shadowStatus("EscrowIntegrityWorker"), shadowStatus("TotalMoneyInvariantWorker")}

	for _, env := range []string{"development", "staging", "production"} {
		cfg := &config.Config{Server: config.ServerConfig{Env: env}}
		ready, degraded := evaluateReadiness(cfg, true, true, statuses, safePayoutStatus())

		if !ready {
			t.Errorf("env=%s: shadow mode must not fail readiness", env)
		}
		if degraded {
			t.Errorf("env=%s: shadow mode must not report degraded", env)
		}
	}
}

// TestEvaluateReadiness_NilConfig_DoesNotPanic proves a nil config (should
// never happen in practice, but defensive) does not panic and is treated as
// non-development (fail-closed) for degradation purposes.
func TestEvaluateReadiness_NilConfig_DoesNotPanic(t *testing.T) {
	statuses := []worker.CriticalWorkerStatus{darkStatus("EscrowIntegrityWorker")}

	ready, degraded := evaluateReadiness(nil, true, true, statuses, safePayoutStatus())

	if ready {
		t.Error("expected ready=false with nil config and a dark critical worker (fail-closed)")
	}
	if !degraded {
		t.Error("expected degraded=true")
	}
}

// --- PASS_18S: payout completion-loop safety in readiness ---

// TestEvaluateReadiness_UnsafePayoutLoop_DevelopmentStaysReady proves an
// unsafe payout completion loop (PayoutWorker enabled, no webhook/reconciliation)
// does not block local boot, but is visible via degraded=true.
func TestEvaluateReadiness_UnsafePayoutLoop_DevelopmentStaysReady(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{Env: "development"}}
	statuses := []worker.CriticalWorkerStatus{activeStatus("A")}

	ready, degraded := evaluateReadiness(cfg, true, true, statuses, unsafePayoutStatus())

	if !ready {
		t.Error("development must remain ready=true even with an unsafe payout loop (must not block local boot)")
	}
	if !degraded {
		t.Error("degraded must be true so the unsafe payout loop is still visible in the response")
	}
}

// TestEvaluateReadiness_UnsafePayoutLoop_StagingAndProductionFailReadiness is
// the PASS_18S core fix: readiness must not falsely imply "all safe" when
// PayoutWorker is submitting payout requests with no completion path, in a
// production-like environment.
func TestEvaluateReadiness_UnsafePayoutLoop_StagingAndProductionFailReadiness(t *testing.T) {
	statuses := []worker.CriticalWorkerStatus{activeStatus("A")}

	for _, env := range []string{"staging", "production"} {
		cfg := &config.Config{Server: config.ServerConfig{Env: env}}
		ready, degraded := evaluateReadiness(cfg, true, true, statuses, unsafePayoutStatus())

		if ready {
			t.Errorf("env=%s: expected ready=false when payout completion loop is unsafe", env)
		}
		if !degraded {
			t.Errorf("env=%s: expected degraded=true", env)
		}
	}
}

// TestEvaluateReadiness_SafePayoutLoop_NeverDegrades proves a payout loop
// with a configured completion path (or PayoutWorker disabled entirely)
// never degrades or fails readiness, in any environment.
func TestEvaluateReadiness_SafePayoutLoop_NeverDegrades(t *testing.T) {
	statuses := []worker.CriticalWorkerStatus{activeStatus("A")}
	safeConfigured := config.PayoutCompletionSafety{
		PayoutWorkerEnabled:     true,
		PayoutWebhookConfigured: true,
		CompletionPathAvailable: true,
		Degraded:                false,
	}

	for _, env := range []string{"development", "staging", "production"} {
		cfg := &config.Config{Server: config.ServerConfig{Env: env}}
		ready, degraded := evaluateReadiness(cfg, true, true, statuses, safeConfigured)

		if !ready {
			t.Errorf("env=%s: configured completion path must not fail readiness", env)
		}
		if degraded {
			t.Errorf("env=%s: configured completion path must not report degraded", env)
		}

		ready, degraded = evaluateReadiness(cfg, true, true, statuses, safePayoutStatus())
		if !ready || degraded {
			t.Errorf("env=%s: disabled payout worker must not degrade or fail readiness", env)
		}
	}
}
