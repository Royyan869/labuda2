package entity

import (
	"testing"
)

func TestNegotiationResourceTypeString(t *testing.T) {
	tests := []struct {
		resourceType NegotiationResourceType
		expected     string
	}{
		{NegotiationResourceForSale, "for_sale"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.resourceType.String(); got != tt.expected {
				t.Errorf("NegotiationResourceType.String() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestNegotiationResourceTypeIsValid(t *testing.T) {
	tests := []struct {
		name     string
		resource NegotiationResourceType
		expected bool
	}{
		{"for_sale type", NegotiationResourceForSale, true},
		{"invalid type", NegotiationResourceType("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.resource.IsValid(); got != tt.expected {
				t.Errorf("NegotiationResourceType.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}


