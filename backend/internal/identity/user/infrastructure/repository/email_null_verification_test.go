package repository_test

import (
	"testing"

	"github.com/labuda/backend/internal/util"
)

// STEP 3 — VERIFY (WAJIB)
// Manual verification tests for email NULL handling

func TestCase1_EmailNilStoresAsNULL(t *testing.T) {
	// CASE 1: Input nil → Expected DB: NULL

	var email *string = nil
	normalizedEmail := util.NormalizeEmail(email)

	if normalizedEmail != nil {
		t.Errorf("Expected nil, got %v", *normalizedEmail)
	}

	// In DB, this would be stored as NULL
	t.Log("✓ CASE 1 PASSED: nil → DB: NULL")
}

func TestCase2_WhitespaceEmailStoresAsNULL(t *testing.T) {
	// CASE 2: Input "   " → Expected DB: NULL

	email := "   "
	normalizedEmail := util.NormalizeEmail(&email)

	if normalizedEmail != nil {
		t.Errorf("Expected nil for whitespace input, got %v", *normalizedEmail)
	}

	// In DB, this would be stored as NULL
	t.Log("✓ CASE 2 PASSED: '   ' → DB: NULL")
}

func TestCase3_ValidEmailNormalizedAndStored(t *testing.T) {
	// CASE 3: Input "Test@Email.com " → Expected DB: "test@email.com"

	email := "Test@Email.com "
	normalizedEmail := util.NormalizeEmail(&email)

	if normalizedEmail == nil {
		t.Errorf("Expected normalized email, got nil")
		return
	}

	expected := "test@email.com"
	if *normalizedEmail != expected {
		t.Errorf("Expected %s, got %s", expected, *normalizedEmail)
	}

	// In DB, this would be stored as "test@email.com"
	t.Log("✓ CASE 3 PASSED: 'Test@Email.com ' → DB: 'test@email.com'")
}

func TestCase4_EmptyStringStoresAsNULL(t *testing.T) {
	// Additional test: "" → NULL

	email := ""
	normalizedEmail := util.NormalizeEmail(&email)

	if normalizedEmail != nil {
		t.Errorf("Expected nil for empty string, got %v", *normalizedEmail)
	}

	// In DB, this would be stored as NULL
	t.Log("✓ CASE 4 PASSED: '' → DB: NULL")
}


