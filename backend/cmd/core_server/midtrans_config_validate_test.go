package main

import (
	"strings"
	"testing"

	"github.com/labuda/backend/internal/config"
)

// canonicalTestCallbackURL is composed from the SAME constant the route mount
// and the validator use, so these tests cannot drift from the real route path.
var canonicalTestCallbackURL = "https://labuda-dev.example.com" + canonicalPaymentWebhookPath

func cfgWith(env, server, client string) *config.Config {
	return &config.Config{
		Midtrans: config.MidtransConfig{
			Environment:     env,
			ServerKey:       server,
			ClientKey:       client,
			NotificationURL: canonicalTestCallbackURL,
		},
	}
}

// TestCanonicalPaymentWebhookPath_IsSingleAuthority proves the callback path is
// defined once and that the mounted route and the validator consume the same
// value — a second literal would let the configured target point at a route
// that does not exist.
func TestCanonicalPaymentWebhookPath_IsSingleAuthority(t *testing.T) {
	if canonicalPaymentWebhookPath != "/webhooks/payment/midtrans" {
		t.Fatalf("canonical payment callback path changed: got %q", canonicalPaymentWebhookPath)
	}
	if got := webhooksGroupPrefix + paymentWebhookRoutePath; got != canonicalPaymentWebhookPath {
		t.Fatalf("mounted webhook path %q != canonical callback path %q", got, canonicalPaymentWebhookPath)
	}
}

func TestValidateMidtransConfig_AcceptsSandboxWithKeys(t *testing.T) {
	cfg := cfgWith("sandbox", "SB-Mid-server-abc", "SB-Mid-client-xyz")
	if err := validateMidtransConfig(cfg); err != nil {
		t.Fatalf("expected no error for valid sandbox config, got %v", err)
	}
}

func TestValidateMidtransConfig_RejectsProduction(t *testing.T) {
	cfg := cfgWith("production", "Mid-server-abc", "Mid-client-xyz")
	err := validateMidtransConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "production") {
		t.Errorf("expected production rejection, got %v", err)
	}
}

func TestValidateMidtransConfig_RejectsEmptyEnv(t *testing.T) {
	cfg := cfgWith("", "x", "y")
	if err := validateMidtransConfig(cfg); err == nil {
		t.Errorf("expected error on empty environment")
	}
}

func TestValidateMidtransConfig_RejectsUnknownEnv(t *testing.T) {
	cfg := cfgWith("staging", "x", "y")
	if err := validateMidtransConfig(cfg); err == nil {
		t.Errorf("expected error on unknown environment")
	}
}

func TestValidateMidtransConfig_RejectsEmptyServerKey(t *testing.T) {
	cfg := cfgWith("sandbox", "", "SB-Mid-client-xyz")
	err := validateMidtransConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "MIDTRANS_SERVER_KEY") {
		t.Errorf("expected ServerKey rejection, got %v", err)
	}
}

func TestValidateMidtransConfig_RejectsEmptyClientKey(t *testing.T) {
	cfg := cfgWith("sandbox", "SB-Mid-server-abc", "")
	err := validateMidtransConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "MIDTRANS_CLIENT_KEY") {
		t.Errorf("expected ClientKey rejection, got %v", err)
	}
}

func TestValidateMidtransConfig_AcceptsSandboxWithoutPrefix(t *testing.T) {
	// Prefix mismatch is a warning, not a fatal.
	cfg := cfgWith("sandbox", "anykey", "anykey")
	if err := validateMidtransConfig(cfg); err != nil {
		t.Errorf("prefix mismatch must be warn-only, got error %v", err)
	}
}

// TestValidateMidtransConfig_RejectsMissingCallbackURL proves the Midtrans
// merchant-dashboard notification URL is NOT a fallback authority: an empty
// per-transaction target is a boot failure.
func TestValidateMidtransConfig_RejectsMissingCallbackURL(t *testing.T) {
	cfg := cfgWith("sandbox", "SB-Mid-server-abc", "SB-Mid-client-xyz")
	cfg.Midtrans.NotificationURL = ""
	err := validateMidtransConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "MIDTRANS_NOTIFICATION_URL") {
		t.Errorf("expected callback-target rejection for empty NotificationURL, got %v", err)
	}
}

func TestValidateMidtransNotificationURL(t *testing.T) {
	const stable = "https://labuda-dev.example.com"
	const path = canonicalPaymentWebhookPath

	cases := []struct {
		name    string
		raw     string
		wantErr string // substring; empty means the target must be accepted
	}{
		{"accepts canonical stable URL", stable + path, ""},
		{"accepts one trailing slash", stable + path + "/", ""},
		{"accepts surrounding whitespace", "  " + stable + path + "  ", ""},
		{"accepts deep subdomain", "https://pay.dev.labuda.example.com" + path, ""},

		{"rejects empty value", "", "required"},
		{"rejects whitespace only", "   ", "required"},
		{"rejects unparsable URL", "://missing-scheme", "not a valid URL"},
		{"rejects relative URL", path, "absolute"},
		{"rejects prefix-relative URL", "labuda.example.com" + path, "absolute"},
		{"rejects http scheme", "http://labuda-dev.example.com" + path, "https"},
		{"rejects localhost", "https://localhost" + path, "localhost"},
		{"rejects loopback IP literal", "https://127.0.0.1" + path, "IP literal"},
		{"rejects private IP literal", "https://10.0.0.5" + path, "IP literal"},
		{"rejects single-label host", "https://labuda" + path, "fully-qualified"},

		{"rejects root path", stable, "must be"},
		{"rejects missing path", stable + "/", "must be"},
		{"rejects wrong webhook path", stable + "/webhooks/payment", "must be"},
		{"rejects suffix-only path", stable + "/webhooks/payment/midtransx", "must be"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Bare validateMidtransNotificationURL is the production-like strict path.
			err := validateMidtransNotificationURL(tc.raw)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected %q to be accepted, got error %v", tc.raw, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected %q to be rejected (want error containing %q)", tc.raw, tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected %q to be rejected with %q, got %v", tc.raw, tc.wantErr, err)
			}
		})
	}
}
