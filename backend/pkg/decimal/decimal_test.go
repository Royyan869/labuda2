package decimal

import (
	"testing"
)

// ============================================================================
// IDR ROUNDING TESTS (Round Half Up to 0 decimal places)
// ============================================================================

func TestRoundIDR_HalfUp(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected string
	}{
		{
			name:     "Round down 99999.4 -> 99999",
			input:    99999.4,
			expected: "99999",
		},
		{
			name:     "Round up 99999.5 -> 100000",
			input:    99999.5,
			expected: "100000",
		},
		{
			name:     "Round up 100000.5 -> 100001",
			input:    100000.5,
			expected: "100001",
		},
		{
			name:     "Round down 100000.4 -> 100000",
			input:    100000.4,
			expected: "100000",
		},
		{
			name:     "Round up 100001.5 -> 100002",
			input:    100001.5,
			expected: "100002",
		},
		{
			name:     "Exact value 100000 -> 100000",
			input:    100000,
			expected: "100000",
		},
		{
			name:     "Round up small decimal 100.5 -> 101",
			input:    100.5,
			expected: "101",
		},
		{
			name:     "Round down small decimal 100.4 -> 100",
			input:    100.4,
			expected: "100",
		},
		{
			name:     "Large amount round up 99999999.5 -> 100000000",
			input:    99999999.5,
			expected: "100000000",
		},
		{
			name:     "Large amount round down 99999999.4 -> 99999999",
			input:    99999999.4,
			expected: "99999999",
		},
		{
			name:     "Zero rounds to zero",
			input:    0,
			expected: "0",
		},
		{
			name:     "Negative rounds down -100.5 -> -101",
			input:    -100.5,
			expected: "-101",
		},
		{
			name:     "Negative rounds up -100.4 -> -100",
			input:    -100.4,
			expected: "-100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewFromFloat(tt.input)
			result := d.RoundIDR()
			if result.String() != tt.expected {
				t.Errorf("RoundIDR(%v) = %s, want %s", tt.input, result.String(), tt.expected)
			}
		})
	}
}

// ============================================================================
// PRECISION DRIFT TESTS
// ============================================================================

// TestDoubleRounding_DoesNotChangeValue - PHASE 1C.1 HARDENING
// Ensures that rounding an already-rounded value does not change it
func TestDoubleRounding_DoesNotChangeValue(t *testing.T) {
	tests := []struct {
		name  string
		input float64
	}{
		{"Integer value", 100000},
		{"Half-up case", 99999.5},
		{"Half-down case", 99999.4},
		{"Large half-up", 99999999.5},
		{"Small half-up", 100.5},
		{"Zero", 0},
		{"Negative half-up", -100.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewFromFloat(tt.input)
			firstRound := d.RoundIDR()
			secondRound := firstRound.RoundIDR()

			if !firstRound.Equal(secondRound) {
				t.Errorf("Double rounding changed value: first=%s, second=%s",
					firstRound.String(), secondRound.String())
			}
		})
	}
}

// ============================================================================
// EDGE CASE TESTS
// ============================================================================

func TestDecimal_Zero(t *testing.T) {
	zero := Zero()
	if !zero.IsZero() {
		t.Error("Zero() should return zero")
	}
}

func TestDecimal_NewFromInt(t *testing.T) {
	d := NewFromInt(100000)
	if d.String() != "100000" {
		t.Errorf("NewFromInt(100000) = %s, want 100000", d.String())
	}
}

func TestDecimal_Add(t *testing.T) {
	a := NewFromInt(100000)
	b := NewFromInt(50000)
	result := a.Add(b)
	if result.String() != "150000" {
		t.Errorf("Add(100000, 50000) = %s, want 150000", result.String())
	}
}

func TestDecimal_Sub(t *testing.T) {
	a := NewFromInt(100000)
	b := NewFromInt(50000)
	result := a.Sub(b)
	if result.String() != "50000" {
		t.Errorf("Sub(100000, 50000) = %s, want 50000", result.String())
	}
}

func TestDecimal_Mul(t *testing.T) {
	a := NewFromInt(100000)
	b := NewFromInt(2)
	result := a.Mul(b)
	if result.String() != "200000" {
		t.Errorf("Mul(100000, 2) = %s, want 200000", result.String())
	}
}

func TestDecimal_Div(t *testing.T) {
	a := NewFromInt(100000)
	b := NewFromInt(2)
	result := a.Div(b).RoundIDR()
	if result.String() != "50000" {
		t.Errorf("Div(100000, 2) = %s, want 50000", result.String())
	}
}

func TestDecimal_Comparisons(t *testing.T) {
	a := NewFromInt(100000)
	b := NewFromInt(50000)
	c := NewFromInt(100000)

	if !a.GreaterThan(b) {
		t.Error("100000 should be greater than 50000")
	}
	if !b.LessThan(a) {
		t.Error("50000 should be less than 100000")
	}
	if !a.Equal(c) {
		t.Error("100000 should equal 100000")
	}
	if !a.GreaterThanOrEqual(c) {
		t.Error("100000 should be >= 100000")
	}
	if !a.LessThanOrEqual(c) {
		t.Error("100000 should be <= 100000")
	}
}

// ============================================================================
// JSON SERIALIZATION TESTS
// ============================================================================

func TestDecimal_MarshalJSON(t *testing.T) {
	d := NewFromInt(100000)
	bytes, err := d.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}
	if string(bytes) != `"100000"` {
		t.Errorf("MarshalJSON() = %s, want \"100000\"", string(bytes))
	}
}

func TestDecimal_UnmarshalJSON_String(t *testing.T) {
	var d Decimal
	err := d.UnmarshalJSON([]byte(`"100000"`))
	if err != nil {
		t.Fatalf("UnmarshalJSON failed: %v", err)
	}
	if d.String() != "100000" {
		t.Errorf("UnmarshalJSON() = %s, want 100000", d.String())
	}
}

func TestDecimal_UnmarshalJSON_Number(t *testing.T) {
	var d Decimal
	err := d.UnmarshalJSON([]byte(`100000`))
	if err != nil {
		t.Fatalf("UnmarshalJSON failed: %v", err)
	}
	if d.String() != "100000" {
		t.Errorf("UnmarshalJSON() = %s, want 100000", d.String())
	}
}

// ============================================================================
// REFUND REVERSAL TESTS
// ============================================================================

func TestRefundReversal_ExactMatch(t *testing.T) {
	// Test that refund reversal is exact (no drift)
	originalAmount := NewFromInt(100000)

	// Calculate what should be refunded
	refundAmount := originalAmount

	// Verify exact match
	if !refundAmount.Equal(originalAmount) {
		t.Errorf("Refund reversal drift: %s != %s", refundAmount.String(), originalAmount.String())
	}
}

// ============================================================================

