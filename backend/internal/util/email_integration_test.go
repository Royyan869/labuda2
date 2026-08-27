package util

import (
	"testing"
)

// TestEmailNilHandling verifies the nil handling behavior for email normalization
// This ensures empty/whitespace emails are handled correctly without causing DB constraint violations
func TestEmailNilHandling(t *testing.T) {
	tests := []struct {
		name          string
		input         *string
		expectNil     bool
		expectValue   string
		description   string
	}{
		{
			name:        "nil_input_returns_nil",
			input:       nil,
			expectNil:   true,
			description: "nil input should return nil - no email set",
		},
		{
			name:        "empty_string_returns_nil",
			input:       strPtr(""),
			expectNil:   true,
			description: "empty string input should return nil - no valid email",
		},
		{
			name:        "whitespace_only_returns_nil",
			input:       strPtr("   "),
			expectNil:   true,
			description: "whitespace-only input should return nil - no valid email",
		},
		{
			name:        "valid_email_normalized",
			input:       strPtr("Test@Example.com"),
			expectNil:   false,
			expectValue: "test@example.com",
			description: "valid email should be normalized (lowercase, trimmed)",
		},
		{
			name:        "email_with_spaces_normalized",
			input:       strPtr("  test@example.com  "),
			expectNil:   false,
			expectValue: "test@example.com",
			description: "email with spaces should be trimmed and normalized",
		},
		{
			name:        "mixed_case_email_normalized",
			input:       strPtr("UsEr@ExAmPlE.cOm"),
			expectNil:   false,
			expectValue: "user@example.com",
			description: "mixed case email should be converted to lowercase",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeEmail(tt.input)

			// Check nil expectation
			if tt.expectNil {
				if result != nil {
					t.Errorf("Expected nil result, got %v", *result)
				}
				// If result is nil, we're done
				return
			}

			// Check non-nil expectation
			if result == nil {
				t.Errorf("Expected non-nil result with value %s, got nil", tt.expectValue)
				return
			}

			// Check value
			if *result != tt.expectValue {
				t.Errorf("Expected %s, got %s", tt.expectValue, *result)
			}
		})
	}
}

// TestEmailRepositoryBehavior simulates how the repository should handle email normalization
func TestEmailRepositoryBehavior(t *testing.T) {
	tests := []struct {
		name        string
		originalEmail string
		normalizedEmail string
		shouldModify bool
	}{
		{
			name:        "empty_email_stays_empty",
			originalEmail: "",
			normalizedEmail: "",
			shouldModify: false,
		},
		{
			name:        "valid_email_gets_normalized",
			originalEmail: "Test@Example.com",
			normalizedEmail: "test@example.com",
			shouldModify: true,
		},
		{
			name:        "already_normalized_no_change",
			originalEmail: "test@example.com",
			normalizedEmail: "test@example.com",
			shouldModify: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate repository behavior
			userEmail := tt.originalEmail

			// Apply normalization (as repository does)
			normalized := NormalizeEmail(&userEmail)
			if normalized != nil {
				userEmail = *normalized
			}
			// If normalized is nil, userEmail stays as-is (could be "" from source)

			// Verify result
			if userEmail != tt.normalizedEmail {
				t.Errorf("Expected email %s, got %s", tt.normalizedEmail, userEmail)
			}
		})
	}
}


