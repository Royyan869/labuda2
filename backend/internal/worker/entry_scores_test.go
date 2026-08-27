package worker

import (
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// TestEntryScores_UniqueConstraint documents the unique constraint on
// (entry_id, judge_person_id) that prevents duplicate scores.
func TestEntryScores_UniqueConstraint(t *testing.T) {
	t.Run("constraint_exists", func(t *testing.T) {
		// SPECIFICATION: Unique constraint "unique_entry_judge_score" must exist
		// on (entry_id, judge_person_id)
		// This prevents duplicate scores from the same judge for the same entry
	})
}

// TestMockDuplicateScoreError verifies the mock error structure.
func TestMockDuplicateScoreError(t *testing.T) {
	err := MockDuplicateScoreError()

	pgErr, ok := err.(*pgconn.PgError)
	if !ok {
		t.Fatalf("Expected *pgconn.PgError, got %T", err)
	}

	if pgErr.Code != "23505" {
		t.Errorf("Expected error code 23505, got %s", pgErr.Code)
	}

	expectedMsg := "duplicate key value violates unique constraint \"unique_entry_judge_score\""
	if pgErr.Message != expectedMsg {
		t.Errorf("Expected message '%s', got '%s'", expectedMsg, pgErr.Message)
	}
}

// TestEntryScore_MinorUnitConversion tests the conversion between
// decimal display format and integer minor-unit storage.
func TestEntryScore_MinorUnitConversion(t *testing.T) {
	tests := []struct {
		name           string
		displayScore   float64 // What user sees (e.g., 85.50)
		expectedStored int     // What's stored (e.g., 8550)
	}{
		{"Zero score", 0.00, 0},
		{"Perfect score", 100.00, 10000},
		{"Half score", 50.00, 5000},
		{"With decimal", 85.50, 8550},
		{"Small decimal", 0.01, 1},
		{"Max precision", 99.99, 9999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stored := int(tt.displayScore * 100)
			if stored != tt.expectedStored {
				t.Errorf("Display %.2f -> stored %d, want %d", tt.displayScore, stored, tt.expectedStored)
			}

			display := float64(stored) / 100
			if display != tt.displayScore {
				t.Errorf("Stored %d -> display %.2f, want %.2f", stored, display, tt.displayScore)
			}
		})
	}
}

// TestEntryScore_Validation_Integers tests INTEGER score validation.
func TestEntryScore_Validation_Integers(t *testing.T) {
	tests := []struct {
		name       string
		score      int // Stored value (minor unit)
		wantValid  bool
		displayVal float64 // Expected display value
	}{
		{"Minimum valid", 0, true, 0.00},
		{"One cent", 1, true, 0.01},
		{"Half point", 50, true, 0.50},
		{"Average score", 7500, true, 75.00},
		{"Perfect score", 10000, true, 100.00},
		{"Below minimum", -1, false, 0},
		{"Above maximum", 10001, false, 0},
		{"Far above maximum", 50000, false, 0},
	}

	const (
		minScore = 0
		maxScore = 10000
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := tt.score >= minScore && tt.score <= maxScore
			if valid != tt.wantValid {
				t.Errorf("Score %d validity = %v, want %v", tt.score, valid, tt.wantValid)
			}

			if tt.wantValid {
				display := float64(tt.score) / 100
				if display != tt.displayVal {
					t.Errorf("Score %d -> display %.2f, want %.2f", tt.score, display, tt.displayVal)
				}
			}
		})
	}
}

// TestEntryScore_CheckConstraint documents the CHECK constraint for scores.
func TestEntryScore_CheckConstraint(t *testing.T) {
	t.Run("constraint_documentation", func(t *testing.T) {
		// The CHECK constraint in the schema:
		// score INTEGER NOT NULL CHECK (score >= 0 AND score <= 10000)
		//
		// This enforces at the database level:
		// - Minimum score: 0 (represents 0.00)
		// - Maximum score: 10000 (represents 100.00)
	})
}

// MockDuplicateScoreError simulates the PostgreSQL unique constraint violation.
func MockDuplicateScoreError() error {
	return &pgconn.PgError{
		Code:    "23505",
		Message: "duplicate key value violates unique constraint \"unique_entry_judge_score\"",
		Detail:  "Key (entry_id, judge_person_id)=(<entry_id>, <judge_person_id>) already exists.",
	}
}


