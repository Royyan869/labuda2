// Tests for PASS_18S: payout completion-loop safety.
//
// EvaluatePayoutCompletionSafety, ValidatePayoutCompletionPath, and the
// environment-aware branch added to ValidatePayoutGatewayProvider must
// together guarantee that PayoutWorker can never silently submit payout
// requests with no way to reach a confirmed terminal state.
package config

import "testing"

// --- EvaluatePayoutCompletionSafety ---

func TestEvaluatePayoutCompletionSafety_WorkerDisabled_NeverDegraded(t *testing.T) {
	cfg := &Config{Payout: PayoutConfig{EnableWorker: false}}

	safety := cfg.EvaluatePayoutCompletionSafety()

	if safety.Degraded {
		t.Fatal("payout worker disabled must never be reported as degraded — no submission loop is running")
	}
	if safety.PayoutWorkerEnabled {
		t.Fatal("expected PayoutWorkerEnabled=false")
	}
}

// TestEvaluatePayoutCompletionSafety_EnabledNoCompletionPath_Unsafe is the
// PASS_18S regression test for the exact runtime gap found in PASS_18Q/R:
// PayoutWorker enabled, PAYOUT_SECRET_KEY unset, reconciliation disabled.
func TestEvaluatePayoutCompletionSafety_EnabledNoCompletionPath_Unsafe(t *testing.T) {
	cfg := &Config{Payout: PayoutConfig{
		EnableWorker:         true,
		SecretKey:            "",
		EnableReconciliation: false,
	}}

	safety := cfg.EvaluatePayoutCompletionSafety()

	if !safety.Degraded {
		t.Fatal("expected Degraded=true: worker enabled with no webhook secret and no reconciliation")
	}
	if safety.CompletionPathAvailable {
		t.Fatal("expected CompletionPathAvailable=false")
	}
	if safety.SafeForRealMoney {
		t.Fatal("expected SafeForRealMoney=false")
	}
	if safety.Reason == "" {
		t.Fatal("expected a human-readable reason to be set")
	}
}

// TestEvaluatePayoutCompletionSafety_WebhookConfigured_Safe proves a
// configured webhook secret alone is recognized as a valid completion path.
func TestEvaluatePayoutCompletionSafety_WebhookConfigured_Safe(t *testing.T) {
	cfg := &Config{Payout: PayoutConfig{
		EnableWorker:    true,
		SecretKey:       "test-only-local-dev-secret",
		GatewayProvider: "midtrans_payout",
	}}

	safety := cfg.EvaluatePayoutCompletionSafety()

	if safety.Degraded {
		t.Fatal("expected Degraded=false when a webhook secret is configured")
	}
	if !safety.CompletionPathAvailable {
		t.Fatal("expected CompletionPathAvailable=true")
	}
	if !safety.SafeForRealMoney {
		t.Fatal("expected SafeForRealMoney=true (webhook configured + non-sandbox gateway)")
	}
}

// TestEvaluatePayoutCompletionSafety_ReconciliationAlone_StillUnsafe is the
// PASS_18S "do not fake it" regression test: PayoutReconciliationService is
// a stub (QueryGatewayStatus never calls the real gateway, MarkPayoutStuck
// never transitions state), so enabling PAYOUT_ENABLE_RECONCILIATION alone
// must NOT be treated as a real completion path.
func TestEvaluatePayoutCompletionSafety_ReconciliationAlone_StillUnsafe(t *testing.T) {
	cfg := &Config{Payout: PayoutConfig{
		EnableWorker:         true,
		SecretKey:            "",
		EnableReconciliation: true,
	}}

	safety := cfg.EvaluatePayoutCompletionSafety()

	if !safety.PayoutReconciliationEnabled {
		t.Fatal("expected PayoutReconciliationEnabled=true to be reported (it IS enabled)")
	}
	if safety.CompletionPathAvailable {
		t.Fatal("reconciliation-enabled-but-non-functional must NOT count as a completion path (do not fake safety)")
	}
	if !safety.Degraded {
		t.Fatal("expected Degraded=true even with reconciliation enabled, since it cannot resolve a stuck payout")
	}
}

// TestEvaluatePayoutCompletionSafety_SandboxGateway_NeverSafeForRealMoney
// proves the fake sandbox gateway can never be reported safe for real
// money, even with a webhook configured.
func TestEvaluatePayoutCompletionSafety_SandboxGateway_NeverSafeForRealMoney(t *testing.T) {
	cfg := &Config{Payout: PayoutConfig{
		EnableWorker:    true,
		SecretKey:       "test-only-local-dev-secret",
		GatewayProvider: "sandbox",
	}}

	safety := cfg.EvaluatePayoutCompletionSafety()

	if safety.SafeForRealMoney {
		t.Fatal("the fake AlwaysSucceed sandbox gateway must never be reported safe for real money")
	}
	// The completion path (webhook) is still "available" — it's the gateway
	// that disqualifies real-money safety, not the completion loop itself.
	if !safety.CompletionPathAvailable {
		t.Fatal("expected CompletionPathAvailable=true (webhook is configured)")
	}
}

// --- ValidatePayoutCompletionPath ---

func TestValidatePayoutCompletionPath_Development_NeverBlocksEvenIfUnsafe(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Env: "development"},
		Payout: PayoutConfig{EnableWorker: true, SecretKey: ""},
	}

	if err := cfg.ValidatePayoutCompletionPath(); err != nil {
		t.Fatalf("development must never block boot on payout completion safety, got: %v", err)
	}
}

// TestValidatePayoutCompletionPath_StagingAndProduction_BlocksWhenUnsafe is
// the PASS_18S core fix: a production-like environment must refuse to boot
// with PayoutWorker enabled and no completion path.
func TestValidatePayoutCompletionPath_StagingAndProduction_BlocksWhenUnsafe(t *testing.T) {
	for _, env := range []string{"staging", "production"} {
		cfg := &Config{
			Server: ServerConfig{Env: env},
			Payout: PayoutConfig{EnableWorker: true, SecretKey: ""},
		}

		if err := cfg.ValidatePayoutCompletionPath(); err == nil {
			t.Errorf("env=%s: expected an error when PayoutWorker is enabled with no completion path", env)
		}
	}
}

// TestValidatePayoutCompletionPath_StagingAndProduction_PassesWhenSafe
// proves a genuinely configured webhook unblocks staging/production boot.
func TestValidatePayoutCompletionPath_StagingAndProduction_PassesWhenSafe(t *testing.T) {
	for _, env := range []string{"staging", "production"} {
		cfg := &Config{
			Server: ServerConfig{Env: env},
			Payout: PayoutConfig{
				EnableWorker:    true,
				SecretKey:       "configured-secret",
				GatewayProvider: "midtrans_payout",
			},
		}

		if err := cfg.ValidatePayoutCompletionPath(); err != nil {
			t.Errorf("env=%s: expected no error with a configured webhook secret, got: %v", env, err)
		}
	}
}

// TestValidatePayoutCompletionPath_WorkerDisabled_NeverBlocks proves a
// disabled payout worker never fails this check, in any environment.
func TestValidatePayoutCompletionPath_WorkerDisabled_NeverBlocks(t *testing.T) {
	for _, env := range []string{"development", "staging", "production"} {
		cfg := &Config{
			Server: ServerConfig{Env: env},
			Payout: PayoutConfig{EnableWorker: false},
		}

		if err := cfg.ValidatePayoutCompletionPath(); err != nil {
			t.Errorf("env=%s: disabled payout worker must never block boot, got: %v", env, err)
		}
	}
}

// --- ValidatePayoutGatewayProvider environment-aware empty-provider guard ---

func TestValidatePayoutGatewayProvider_EmptyInDevelopment_DefaultsToSandbox(t *testing.T) {
	cfg := &Config{Server: ServerConfig{Env: "development"}, Payout: PayoutConfig{GatewayProvider: ""}}

	cfg.ValidatePayoutGatewayProvider() // must not panic

	if cfg.Payout.GatewayProvider != "sandbox" {
		t.Fatalf("expected empty provider to default to sandbox in development, got %q", cfg.Payout.GatewayProvider)
	}
}

// TestValidatePayoutGatewayProvider_EmptyInProductionLikeEnv_Panics is the
// PASS_18S gateway-safety fix: an unset PAYOUT_GATEWAY_PROVIDER must not
// silently become the fake sandbox gateway in staging/production.
func TestValidatePayoutGatewayProvider_EmptyInProductionLikeEnv_Panics(t *testing.T) {
	for _, env := range []string{"staging", "production"} {
		cfg := &Config{Server: ServerConfig{Env: env}, Payout: PayoutConfig{GatewayProvider: ""}}

		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("env=%s: expected panic on empty PAYOUT_GATEWAY_PROVIDER", env)
				}
			}()
			cfg.ValidatePayoutGatewayProvider()
		}()
	}
}

func TestValidatePayoutGatewayProvider_UnknownProvider_AlwaysPanics(t *testing.T) {
	for _, env := range []string{"development", "staging", "production"} {
		cfg := &Config{Server: ServerConfig{Env: env}, Payout: PayoutConfig{GatewayProvider: "totally-fake-gateway"}}

		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("env=%s: expected panic on unknown gateway provider", env)
				}
			}()
			cfg.ValidatePayoutGatewayProvider()
		}()
	}
}

func TestValidatePayoutGatewayProvider_ValidProvider_NeverPanics(t *testing.T) {
	for _, env := range []string{"development", "staging", "production"} {
		for _, provider := range []string{"sandbox", "midtrans_payout", "MIDTRANS_PAYOUT"} {
			cfg := &Config{Server: ServerConfig{Env: env}, Payout: PayoutConfig{GatewayProvider: provider}}
			cfg.ValidatePayoutGatewayProvider() // must not panic
		}
	}
}
