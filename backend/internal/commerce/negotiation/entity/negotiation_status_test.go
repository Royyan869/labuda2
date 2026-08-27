package entity

import (
	"testing"
)

func TestNegotiationStatusString(t *testing.T) {
	tests := []struct {
		status   NegotiationStatus
		expected string
	}{
		{NegotiationStatusActive, "active"},
		{NegotiationStatusAccepted, "accepted"},
		{NegotiationStatusCancelled, "cancelled"},
		{NegotiationStatusExpired, "expired"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.status.String(); got != tt.expected {
				t.Errorf("NegotiationStatus.String() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestNegotiationStatusIsActive(t *testing.T) {
	tests := []struct {
		status   NegotiationStatus
		expected bool
	}{
		{NegotiationStatusActive, true},
		{NegotiationStatusAccepted, false},
		{NegotiationStatusCancelled, false},
		{NegotiationStatusExpired, false},
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			if got := tt.status.IsActive(); got != tt.expected {
				t.Errorf("NegotiationStatus.IsActive() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNegotiationStatusIsTerminal(t *testing.T) {
	tests := []struct {
		status   NegotiationStatus
		expected bool
	}{
		{NegotiationStatusActive, false},
		{NegotiationStatusAccepted, false},
		{NegotiationStatusCancelled, true},
		{NegotiationStatusExpired, true},
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			if got := tt.status.IsTerminal(); got != tt.expected {
				t.Errorf("NegotiationStatus.IsTerminal() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNegotiationStatusCanTransition(t *testing.T) {
	tests := []struct {
		name     string
		from     NegotiationStatus
		to       NegotiationStatus
		expected bool
	}{
		// From active
		{"active -> accepted", NegotiationStatusActive, NegotiationStatusAccepted, true},
		{"active -> cancelled", NegotiationStatusActive, NegotiationStatusCancelled, true},
		{"active -> expired", NegotiationStatusActive, NegotiationStatusExpired, true},
		{"active -> active", NegotiationStatusActive, NegotiationStatusActive, false},

		// From terminal states (no transitions allowed)
		{"accepted -> active", NegotiationStatusAccepted, NegotiationStatusActive, false},
		{"accepted -> cancelled", NegotiationStatusAccepted, NegotiationStatusCancelled, false},
		{"cancelled -> active", NegotiationStatusCancelled, NegotiationStatusActive, false},
		{"expired -> active", NegotiationStatusExpired, NegotiationStatusActive, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.from.CanTransition(tt.to); got != tt.expected {
				t.Errorf("NegotiationStatus.CanTransition() = %v, want %v", got, tt.expected)
			}
		})
	}
}


