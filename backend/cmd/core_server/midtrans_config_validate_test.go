package main

import (
	"strings"
	"testing"

	"github.com/labuda/backend/internal/config"
)

func cfgWith(env, server, client string) *config.Config {
	return &config.Config{
		Midtrans: config.MidtransConfig{
			Environment: env,
			ServerKey:   server,
			ClientKey:   client,
		},
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
