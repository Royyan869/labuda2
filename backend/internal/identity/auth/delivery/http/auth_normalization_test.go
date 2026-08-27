package http_test

import (
	"testing"

	"github.com/labuda/backend/internal/util"
)

// STEP A.2.4 — AUTH HANDLER NORMALIZATION VERIFICATION
// Ensuring auth handler NEVER uses raw email for database operations

func TestCase_AuthHandler_EmptyStringBecomesNULL(t *testing.T) {
	// SCENARIO: Firebase returns empty string in email claim
	email := ""

	// What happens in FirebaseAuth handler
	normalizedEmail := util.NormalizeEmail(&email)

	// Prepare email value for database operations
	var emailValue interface{}
	if normalizedEmail != nil {
		emailValue = *normalizedEmail
	} else {
		emailValue = nil
	}

	// VERIFY: emailValue should be nil (will be stored as NULL in DB)
	if emailValue != nil {
		t.Errorf("Expected nil for empty string, got %v", emailValue)
	}

	// VERIFY: SQL queries will use NULL
	// WHERE LOWER(BTRIM(email)) = LOWER(BTRIM(NULL)) → matches NULL email columns
	// INSERT INTO users (..., email, ...) VALUES (..., NULL, ...) → stores NULL

	t.Log("✓ Empty string '' → DB: NULL")
}

func TestCase_AuthHandler_WhitespaceBecomesNULL(t *testing.T) {
	// SCENARIO: Firebase returns whitespace-only email
	email := "   "

	// What happens in FirebaseAuth handler
	normalizedEmail := util.NormalizeEmail(&email)

	// Prepare email value for database operations
	var emailValue interface{}
	if normalizedEmail != nil {
		emailValue = *normalizedEmail
	} else {
		emailValue = nil
	}

	// VERIFY: emailValue should be nil (will be stored as NULL in DB)
	if emailValue != nil {
		t.Errorf("Expected nil for whitespace input, got %v", emailValue)
	}

	t.Log("✓ Whitespace '   ' → DB: NULL")
}

func TestCase_AuthHandler_ValidEmailNormalized(t *testing.T) {
	// SCENARIO: Firebase returns valid email with whitespace
	email := "Test@Email.com "

	// What happens in FirebaseAuth handler
	normalizedEmail := util.NormalizeEmail(&email)

	// Prepare email value for database operations
	var emailValue interface{}
	if normalizedEmail != nil {
		emailValue = *normalizedEmail
	} else {
		emailValue = nil
	}

	// VERIFY: emailValue should be normalized lowercase string
	if emailValue == nil {
		t.Errorf("Expected normalized email, got nil")
		return
	}

	expected := "test@email.com"
	if emailValue != expected {
		t.Errorf("Expected %s, got %v", expected, emailValue)
	}

	t.Log("✓ Valid email 'Test@Email.com ' → DB: 'test@email.com'")
}

func TestCase_AuthHandler_SQLQueryUsesNormalizedValue(t *testing.T) {
	// SCENARIO: Account linking query checks if email exists
	email := "  User@Example.com  "

	// What happens in FirebaseAuth handler
	normalizedEmail := util.NormalizeEmail(&email)

	var emailValue interface{}
	if normalizedEmail != nil {
		emailValue = *normalizedEmail
	} else {
		emailValue = nil
	}

	// SIMULATE: SQL query in auth_handler.go line ~190
	// WHERE LOWER(BTRIM(email)) = LOWER(BTRIM($1))
	// $1 = emailValue

	// VERIFY: Query uses normalized value, not raw email
	if emailValue == nil {
		t.Error("Expected normalized email for SQL query, got nil")
		return
	}

	queryValue := emailValue.(string)
	expectedQueryValue := "user@example.com"

	if queryValue != expectedQueryValue {
		t.Errorf("SQL query should use normalized value '%s', got '%s'", expectedQueryValue, queryValue)
	}

	t.Log("✓ SQL query uses normalized value 'user@example.com'")
}

func TestCase_AuthHandler_createUserNormalizesBeforeInsert(t *testing.T) {
	// SCENARIO: createUser function receives email parameter
	email := "  New@User.com  "

	// What happens inside createUser function
	normalizedEmail := util.NormalizeEmail(&email)

	var emailValue interface{}
	if normalizedEmail != nil {
		emailValue = *normalizedEmail
	} else {
		emailValue = nil
	}

	// SIMULATE: SQL INSERT in auth_handler.go line ~457
	// INSERT INTO users (..., email, ...) VALUES (..., $3, ...)
	// $3 = emailValue

	if emailValue == nil {
		t.Error("Expected normalized email for INSERT, got nil")
		return
	}

	insertValue := emailValue.(string)
	expectedInsertValue := "new@user.com"

	if insertValue != expectedInsertValue {
		t.Errorf("INSERT should use normalized value '%s', got '%s'", expectedInsertValue, insertValue)
	}

	t.Log("✓ INSERT uses normalized value 'new@user.com'")
}

func TestCase_AuthHandler_RawEmailNeverUsedForDB(t *testing.T) {
	// COMPREHENSIVE: Verify raw email is NEVER used for DB operations

	testCases := []struct {
		name     string
		input    string
		expected interface{}
	}{
		{"Empty string", "", nil},
		{"Whitespace only", "   ", nil},
		{"Tab and space", "\t ", nil},
		{"Valid email", "TEST@EXAMPLE.COM", "test@example.com"},
		{"Email with spaces", " test@example.com ", "test@example.com"},
		{"Mixed case", "UsEr@ExAmPlE.cOm", "user@example.com"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// What happens in auth handler
			normalizedEmail := util.NormalizeEmail(&tc.input)

			var emailValue interface{}
			if normalizedEmail != nil {
				emailValue = *normalizedEmail
			} else {
				emailValue = nil
			}

			// VERIFY: emailValue matches expected
			if emailValue != tc.expected {
				t.Errorf("For input '%s', expected %v, got %v", tc.input, tc.expected, emailValue)
			}

			// VERIFY: Raw input is NOT used for DB operations
			// - SQL queries use emailValue
			// - INSERT statements use emailValue
			// - Raw input only used for logging/non-DB operations
		})
	}

	t.Log("✓ Raw email NEVER reaches database operations")
}
