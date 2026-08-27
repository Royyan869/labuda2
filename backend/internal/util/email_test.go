package util

import (
	"testing"
)

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		name     string
		input    *string
		expected *string
	}{
		{
			name:     "Trim spaces and lowercase",
			input:    strPtr(" Test@Email.com "),
			expected: strPtr("test@email.com"),
		},
		{
			name:     "Already normalized",
			input:    strPtr("test@email.com"),
			expected: strPtr("test@email.com"),
		},
		{
			name:     "All uppercase",
			input:    strPtr("TEST@EMAIL.COM"),
			expected: strPtr("test@email.com"),
		},
		{
			name:     "Mixed case with spaces",
			input:    strPtr("  UsEr@ExAmPlE.CoM  "),
			expected: strPtr("user@example.com"),
		},
		{
			name:     "Nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "Empty string after trim",
			input:    strPtr("   "),
			expected: nil,
		},
		{
			name:     "Empty string",
			input:    strPtr(""),
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeEmail(tt.input)
			if tt.expected == nil {
				if result != nil {
					t.Errorf("Expected nil, got %v", result)
				}
				return
			}
			if result == nil {
				t.Errorf("Expected %v, got nil", *tt.expected)
				return
			}
			if *result != *tt.expected {
				t.Errorf("Expected %v, got %v", *tt.expected, *result)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}


