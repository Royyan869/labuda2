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

// safeCallbackHealth is a healthy payment-callback view, so tests that only
// exercise worker/payout degradation keep their INFRA-2 expectations unchanged.
func safeCallbackHealth() PaymentCallbackHealth {
	return PaymentCallbackHealth{
		State:           callbackStateReady,
		Degraded:        false,
		Configured:      true,
		Host:            "labuda-dev.example.com",
		Path:            canonicalPaymentWebhookPath,
		RouteMounted:    true,
		DNSResolves:     true,
		OutboundProbe:   callbackOutboundReachable,
		PublicIngress:   notVerifiableFromProcess,
		GatewayDelivery: notVerifiableFromProcess,
	}
}

// degradedCallbackHealth is a callback defect that must behave exactly like the
// other money-path degradations: visible in development, blocking elsewhere.
func degradedCallbackHealth() PaymentCallbackHealth {
	return PaymentCallbackHealth{
		State:           callbackStateHostUnresolved,
		Degraded:        true,
		Reason:          "test: callback host does not resolve",
		Configured:      true,
		Host:            "labuda-dev.example.com",
		RouteMounted:    true,
		OutboundProbe:   callbackOutboundSkipped,
		PublicIngress:   notVerifiableFromProcess,
		GatewayDelivery: notVerifiableFromProcess,
	}
}

// TestEvaluateReadiness_AllHealthy_NoDarkWorkers proves the baseline healthy
// case reports ready=true, degraded=false regardless of environment.
func TestEvaluateReadiness_AllHealthy_NoDarkWorkers(t *testing.T) {
	statuses := []worker.CriticalWorkerStatus{activeStatus("A"), shadowStatus("B")}

	for _, env := range []string{"development", "staging", "production"} {
		cfg := &config.Config{Server: config.ServerConfig{Env: env}}
		ready, degraded := evaluateReadiness(cfg, true, true, statuses, safePayoutStatus(), safeCallbackHealth())
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

		if ready, _ := evaluateReadiness(cfg, false, true, statuses, safePayoutStatus(), safeCallbackHealth()); ready {
			t.Errorf("env=%s: expected ready=false when DB is down", env)
		}
		if ready, _ := evaluateReadiness(cfg, true, false, statuses, safePayoutStatus(), safeCallbackHealth()); ready {
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

	ready, degraded := evaluateReadiness(cfg, true, true, statuses, safePayoutStatus(), safeCallbackHealth())

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
		ready, degraded := evaluateReadiness(cfg, true, true, statuses, safePayoutStatus(), safeCallbackHealth())

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
		ready, degraded := evaluateReadiness(cfg, true, true, statuses, safePayoutStatus(), safeCallbackHealth())

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

	ready, degraded := evaluateReadiness(nil, true, true, statuses, safePayoutStatus(), safeCallbackHealth())

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

	ready, degraded := evaluateReadiness(cfg, true, true, statuses, unsafePayoutStatus(), safeCallbackHealth())

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
		ready, degraded := evaluateReadiness(cfg, true, true, statuses, unsafePayoutStatus(), safeCallbackHealth())

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
		ready, degraded := evaluateReadiness(cfg, true, true, statuses, safeConfigured, safeCallbackHealth())

		if !ready {
			t.Errorf("env=%s: configured completion path must not fail readiness", env)
		}
		if degraded {
			t.Errorf("env=%s: configured completion path must not report degraded", env)
		}

		ready, degraded = evaluateReadiness(cfg, true, true, statuses, safePayoutStatus(), safeCallbackHealth())
		if !ready || degraded {
			t.Errorf("env=%s: disabled payout worker must not degrade or fail readiness", env)
		}
	}
}

// --- INFRA-2: payment callback dependency in readiness ---

// TestEvaluateReadiness_CallbackDegraded_DevelopmentStaysReady proves a broken
// payment callback path stays visible but does not block local development.
func TestEvaluateReadiness_CallbackDegraded_DevelopmentStaysReady(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{Env: "development"}}
	statuses := []worker.CriticalWorkerStatus{activeStatus("A")}

	ready, degraded := evaluateReadiness(cfg, true, true, statuses, safePayoutStatus(), degradedCallbackHealth())

	if !ready {
		t.Error("development must remain ready=true even when the payment callback is undeliverable (must not block local boot)")
	}
	if !degraded {
		t.Error("degraded must be true so the callback defect is visible in the response")
	}
}

// TestEvaluateReadiness_CallbackDegraded_StagingAndProductionFailReadiness is
// the INFRA-2 core requirement: a callback path the gateway cannot deliver to
// must not be reported as a healthy runtime outside development.
func TestEvaluateReadiness_CallbackDegraded_StagingAndProductionFailReadiness(t *testing.T) {
	statuses := []worker.CriticalWorkerStatus{activeStatus("A")}

	for _, env := range []string{"staging", "production"} {
		cfg := &config.Config{Server: config.ServerConfig{Env: env}}
		ready, degraded := evaluateReadiness(cfg, true, true, statuses, safePayoutStatus(), degradedCallbackHealth())

		if ready {
			t.Errorf("env=%s: expected ready=false when the payment callback is undeliverable", env)
		}
		if !degraded {
			t.Errorf("env=%s: expected degraded=true", env)
		}
	}
}

// TestEvaluateReadiness_CallbackHealthy_NeverDegrades proves a healthy callback
// view never degrades or fails readiness in any environment.
func TestEvaluateReadiness_CallbackHealthy_NeverDegrades(t *testing.T) {
	statuses := []worker.CriticalWorkerStatus{activeStatus("A")}

	for _, env := range []string{"development", "staging", "production"} {
		cfg := &config.Config{Server: config.ServerConfig{Env: env}}
		ready, degraded := evaluateReadiness(cfg, true, true, statuses, safePayoutStatus(), safeCallbackHealth())

		if !ready {
			t.Errorf("env=%s: a healthy callback view must not fail readiness", env)
		}
		if degraded {
			t.Errorf("env=%s: a healthy callback view must not report degraded", env)
		}
	}
}

// TestEvaluateReadiness_OutboundProbeUnreachable_DoesNotGateReadiness proves the
// deliberate INFRA-2 severity choice: an outbound-probe failure is reported as
// evidence and must never, by itself, gate readiness. A process-local outbound
// failure cannot distinguish blocked egress from a real ingress outage, so it is
// not a sound readiness gate.
func TestEvaluateReadiness_OutboundProbeUnreachable_DoesNotGateReadiness(t *testing.T) {
	statuses := []worker.CriticalWorkerStatus{activeStatus("A")}
	callback := safeCallbackHealth()
	callback.OutboundProbe = callbackOutboundUnreachable
	callback.OutboundDetail = "test: dial tcp: connection refused"

	for _, env := range []string{"development", "staging", "production"} {
		cfg := &config.Config{Server: config.ServerConfig{Env: env}}
		ready, degraded := evaluateReadiness(cfg, true, true, statuses, safePayoutStatus(), callback)

		if !ready {
			t.Errorf("env=%s: an unreachable outbound probe must not fail readiness on its own", env)
		}
		if degraded {
			t.Errorf("env=%s: an unreachable outbound probe must not degrade readiness on its own", env)
		}
	}
}

// TestEvaluateReadiness_ConfigInvalidCallback_StagingFailsReadiness proves that
// configuration invalidity is a distinct, blocking callback state outside
// development.
func TestEvaluateReadiness_ConfigInvalidCallback_StagingFailsReadiness(t *testing.T) {
	statuses := []worker.CriticalWorkerStatus{activeStatus("A")}
	callback := safeCallbackHealth()
	callback.State = callbackStateConfigInvalid
	callback.Degraded = true
	callback.ConfigError = "MIDTRANS_NOTIFICATION_URL must use https"
	callback.Reason = "callback configuration is structurally invalid"

	cfg := &config.Config{Server: config.ServerConfig{Env: "staging"}}
	ready, degraded := evaluateReadiness(cfg, true, true, statuses, safePayoutStatus(), callback)

	if ready {
		t.Error("expected ready=false when callback configuration is invalid in staging")
	}
	if !degraded {
		t.Error("expected degraded=true")
	}
}
