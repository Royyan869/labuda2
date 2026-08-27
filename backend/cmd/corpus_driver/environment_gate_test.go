package main

import (
	"testing"
)

func TestCheckEnvironmentGate(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		env     string
		wantErr bool
	}{
		// RUNTIME_MUTATION — only development/unset is safe
		{"d11 dev explicit", "scenario-d11", "development", false},
		{"d11 env unset", "scenario-d11", "", false},
		{"d11 production blocked", "scenario-d11", "production", true},
		{"d11 staging blocked", "scenario-d11", "staging", true},

		// Non-RUNTIME_MUTATION — all environments pass
		{"list-services production", "list-services", "production", false},
		{"projection staging", "scenario-projection", "staging", false},
		{"governance-content production", "scenario-governance-content", "production", false},
		{"governance-feed production", "scenario-governance-feed", "production", false},
		{"governance-detail production", "scenario-governance-detail", "production", false},

		// Unknown mode — not in the RUNTIME_MUTATION set, so not blocked
		{"unknown production", "unknown-mode", "production", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkEnvironmentGate(tc.mode, tc.env)
			if (err != nil) != tc.wantErr {
				t.Errorf("checkEnvironmentGate(%q, %q) err=%v, wantErr=%v", tc.mode, tc.env, err, tc.wantErr)
			}
		})
	}
}
