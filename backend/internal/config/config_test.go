// Tests for P5-03: environment fail-closed defaults.
//
// These pin down two invariants:
//  1. An unset/empty ENV must never behave as "development" — it must
//     default to a safe non-development value so dev-only routes
//     (gated by cfg.IsDevelopment() at the router layer) stay unmounted.
//  2. ValidateProductionSafety must reject any ENV value outside
//     {development, staging, production}, unconditionally — not just
//     when Env=="production".
package config

import (
	"os"
	"testing"
)

// withEnv sets key=value for the duration of the test and restores the
// prior value (or unsets it if it wasn't set) afterward.
func withEnv(t *testing.T, key, value string) {
	t.Helper()
	prev, existed := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("failed to set %s: %v", key, err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(key, prev)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

// withUnsetEnv ensures key is unset for the duration of the test and
// restores the prior value afterward.
func withUnsetEnv(t *testing.T, key string) {
	t.Helper()
	prev, existed := os.LookupEnv(key)
	_ = os.Unsetenv(key)
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(key, prev)
		}
	})
}

// baseRequiredEnv sets the env vars Load() hard-requires (DB_NAME plus an
// explicit non-placeholder DB_PASSWORD) so these tests can isolate ENV
// behavior without a real database.
func baseRequiredEnv(t *testing.T) {
	t.Helper()
	withEnv(t, "DB_NAME", "hishumi_test_config")
	withEnv(t, "DB_PASSWORD", "hishumi_test_cfg_pw")
}

func TestLoad_UnsetEnv_DoesNotBehaveAsDevelopment(t *testing.T) {
	baseRequiredEnv(t)
	withUnsetEnv(t, "ENV")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.IsDevelopment() {
		t.Fatal("unset ENV must not behave as development — dev-only routes would mount unintentionally")
	}
	if cfg.Server.Env != "production" {
		t.Fatalf("expected unset ENV to fail-closed default to 'production', got %q", cfg.Server.Env)
	}
}

func TestLoad_EmptyEnv_DoesNotBehaveAsDevelopment(t *testing.T) {
	baseRequiredEnv(t)
	withEnv(t, "ENV", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.IsDevelopment() {
		t.Fatal("empty ENV must not behave as development")
	}
}

func TestLoad_ExplicitDevelopment_MountsDevRoutes(t *testing.T) {
	baseRequiredEnv(t)
	withEnv(t, "ENV", "development")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if !cfg.IsDevelopment() {
		t.Fatal("explicit ENV=development must mount dev-only routes (IsDevelopment() must be true)")
	}
}

func TestLoad_ExplicitProduction_DoesNotMountDevRoutes(t *testing.T) {
	baseRequiredEnv(t)
	withEnv(t, "ENV", "production")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.IsDevelopment() {
		t.Fatal("explicit ENV=production must not mount dev-only routes")
	}
	if !cfg.IsProduction() {
		t.Fatal("explicit ENV=production must report IsProduction()=true")
	}
}

func TestValidateProductionSafety_InvalidEnv_Panics(t *testing.T) {
	cfg := &Config{Server: ServerConfig{Env: "protuction"}} // typo, not a valid value

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected ValidateProductionSafety to panic on an invalid ENV value")
		}
	}()
	cfg.ValidateProductionSafety()
}

func TestValidateProductionSafety_EmptyEnv_Panics(t *testing.T) {
	cfg := &Config{Server: ServerConfig{Env: ""}}

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected ValidateProductionSafety to panic on an empty ENV value")
		}
	}()
	cfg.ValidateProductionSafety()
}

func TestValidateProductionSafety_Development_DoesNotPanicOnEnvCheck(t *testing.T) {
	// Development is a valid ENV value; ValidateProductionSafety must pass
	// the ENV-validity switch and then no-op (production-only checks below
	// it must not run for a development environment).
	cfg := &Config{Server: ServerConfig{Env: "development"}}
	cfg.ValidateProductionSafety() // must not panic
}

func TestValidateProductionSafety_Staging_DoesNotPanicOnEnvCheck(t *testing.T) {
	cfg := &Config{Server: ServerConfig{Env: "staging"}}
	cfg.ValidateProductionSafety() // must not panic
}

// PASS_SECURITY: Dev flags must never be active in production.

func TestValidateProductionSafety_MockFirebaseAuth_PanicsInProduction(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Env: "production", GinMode: "release"},
		Database: DatabaseConfig{SSLMode: "require"},
		Dev: DevConfig{MockFirebaseAuth: true},
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when DEV_MOCK_FIREBASE_AUTH=true in production")
		}
	}()
	cfg.ValidateProductionSafety()
}

func TestValidateProductionSafety_AutoApproveVerification_PanicsInProduction(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Env: "production", GinMode: "release"},
		Database: DatabaseConfig{SSLMode: "require"},
		Dev: DevConfig{AutoApproveVerification: true},
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when DEV_AUTO_APPROVE_VERIFICATION=true in production")
		}
	}()
	cfg.ValidateProductionSafety()
}

func TestValidateProductionSafety_SkipPaymentGateway_PanicsInProduction(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Env: "production", GinMode: "release"},
		Database: DatabaseConfig{SSLMode: "require"},
		Dev: DevConfig{SkipPaymentGateway: true},
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when DEV_SKIP_PAYMENT_GATEWAY=true in production")
		}
	}()
	cfg.ValidateProductionSafety()
}

func TestValidateProductionSafety_DevFlagsFalse_DoesNotPanic(t *testing.T) {
	// Dev flags false (default) should not trigger production guard
	cfg := &Config{
		Server: ServerConfig{Env: "production", GinMode: "release"},
		Database: DatabaseConfig{SSLMode: "require"},
		Dev: DevConfig{MockFirebaseAuth: false, AutoApproveVerification: false, SkipPaymentGateway: false},
	}
	// This may still panic on other production checks (CORS, payout),
	// but should NOT panic on Dev flag checks
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Dev flags false should not cause panic, got: %v", r)
		}
	}()
	cfg.ValidateProductionSafety()
}

// H.4.4-B: DB_PASSWORD is required and fail-closed. There is no silent weak
// default: Load() must reject a missing password and well-known insecure
// placeholders instead of booting with a guessable credential.

func TestLoad_MissingDBPassword_FailsClosed(t *testing.T) {
	withEnv(t, "DB_NAME", "hishumi_test_config")
	withUnsetEnv(t, "DB_PASSWORD")

	if _, err := Load(); err == nil {
		t.Fatal("Load() must fail when DB_PASSWORD is not set")
	}
}

func TestLoad_PlaceholderDBPassword_FailsClosed(t *testing.T) {
	withEnv(t, "DB_NAME", "hishumi_test_config")

	for _, placeholder := range []string{"changeme", "change_me", "CHANGE_ME_THIS_IS_NOT_SECURE", "password", "postgres"} {
		withEnv(t, "DB_PASSWORD", placeholder)
		if _, err := Load(); err == nil {
			t.Fatalf("Load() must fail when DB_PASSWORD is the insecure placeholder %q", placeholder)
		}
	}
}

func TestLoad_ExplicitDBPassword_Passes(t *testing.T) {
	withEnv(t, "DB_NAME", "hishumi_test_config")
	withEnv(t, "DB_PASSWORD", "hishumi_test_cfg_pw")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed with explicit DB_PASSWORD: %v", err)
	}
	if cfg.Database.Password == "" {
		t.Fatal("explicit DB_PASSWORD must be retained in config")
	}
}
