package money

import (
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name   string
		amount int64
		want   int64
	}{
		{"zero", 0, 0},
		{"positive", 1000, 1000},
		{"negative", -500, -500},
		{"large", 9223372036854775807, 9223372036854775807},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(tt.amount)
			if got := m.Int64(); got != tt.want {
				t.Errorf("New(%d).Int64() = %d, want %d", tt.amount, got, tt.want)
			}
		})
	}
}

func TestZero(t *testing.T) {
	m := Zero()
	if !m.IsZero() {
		t.Errorf("Zero().IsZero() = false, want true")
	}
	if m.Int64() != 0 {
		t.Errorf("Zero().Int64() = %d, want 0", m.Int64())
	}
}

func TestAdd(t *testing.T) {
	tests := []struct {
		name     string
		a        int64
		b        int64
		expected int64
	}{
		{"positive + positive", 100, 200, 300},
		{"positive + zero", 100, 0, 100},
		{"zero + zero", 0, 0, 0},
		{"positive + negative", 200, -50, 150},
		{"negative + negative", -100, -200, -300},
		{"negative + positive", -200, 100, -100},
		{"result zero", 100, -100, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := New(tt.a)
			b := New(tt.b)
			result := a.Add(b)
			if result.Int64() != tt.expected {
				t.Errorf("Add() = %d, want %d", result.Int64(), tt.expected)
			}
			// Verify original is unchanged (immutability)
			if a.Int64() != tt.a {
				t.Errorf("Add() modified a: %d, want %d", a.Int64(), tt.a)
			}
			if b.Int64() != tt.b {
				t.Errorf("Add() modified b: %d, want %d", b.Int64(), tt.b)
			}
		})
	}
}

func TestSub(t *testing.T) {
	tests := []struct {
		name     string
		a        int64
		b        int64
		expected int64
	}{
		{"positive - positive", 300, 100, 200},
		{"positive - zero", 100, 0, 100},
		{"zero - zero", 0, 0, 0},
		{"positive - larger positive", 100, 200, -100},
		{"negative - negative", -100, -200, 100},
		{"negative - positive", -100, 50, -150},
		{"result zero", 100, 100, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := New(tt.a)
			b := New(tt.b)
			result := a.Sub(b)
			if result.Int64() != tt.expected {
				t.Errorf("Sub() = %d, want %d", result.Int64(), tt.expected)
			}
			// Verify original is unchanged (immutability)
			if a.Int64() != tt.a {
				t.Errorf("Sub() modified a: %d, want %d", a.Int64(), tt.a)
			}
			if b.Int64() != tt.b {
				t.Errorf("Sub() modified b: %d, want %d", b.Int64(), tt.b)
			}
		})
	}
}

func TestNeg(t *testing.T) {
	tests := []struct {
		name     string
		amount   int64
		expected int64
	}{
		{"positive", 100, -100},
		{"negative", -100, 100},
		{"zero", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(tt.amount)
			result := m.Neg()
			if result.Int64() != tt.expected {
				t.Errorf("Neg() = %d, want %d", result.Int64(), tt.expected)
			}
			// Verify original is unchanged
			if m.Int64() != tt.amount {
				t.Errorf("Neg() modified original: %d, want %d", m.Int64(), tt.amount)
			}
		})
	}
}

func TestMul(t *testing.T) {
	tests := []struct {
		name     string
		amount   int64
		factor   int64
		expected int64
	}{
		{"positive * positive", 100, 3, 300},
		{"positive * zero", 100, 0, 0},
		{"zero * anything", 0, 100, 0},
		{"negative * positive", -100, 3, -300},
		{"positive * negative", 100, -3, -300},
		{"negative * negative", -100, -3, 300},
		{"multiply by one", 100, 1, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(tt.amount)
			result := m.Mul(tt.factor)
			if result.Int64() != tt.expected {
				t.Errorf("Mul() = %d, want %d", result.Int64(), tt.expected)
			}
			// Verify original is unchanged
			if m.Int64() != tt.amount {
				t.Errorf("Mul() modified original: %d, want %d", m.Int64(), tt.amount)
			}
		})
	}
}

func TestEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        int64
		b        int64
		expected bool
	}{
		{"equal positive", 100, 100, true},
		{"equal negative", -100, -100, true},
		{"equal zero", 0, 0, true},
		{"not equal positive", 100, 200, false},
		{"not equal signs", 100, -100, false},
		{"not equal zero and non-zero", 0, 100, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := New(tt.a)
			b := New(tt.b)
			if got := a.Equal(b); got != tt.expected {
				t.Errorf("Equal() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGreaterThan(t *testing.T) {
	tests := []struct {
		name     string
		a        int64
		b        int64
		expected bool
	}{
		{"greater", 200, 100, true},
		{"less", 100, 200, false},
		{"equal", 100, 100, false},
		{"negative less than positive", -100, 100, false},
		{"positive greater than negative", 100, -100, true},
		{"negative numbers", -50, -100, true},
		{"zero less than positive", 0, 100, false},
		{"zero greater than negative", 0, -100, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := New(tt.a)
			b := New(tt.b)
			if got := a.GreaterThan(b); got != tt.expected {
				t.Errorf("GreaterThan() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestLessThan(t *testing.T) {
	tests := []struct {
		name     string
		a        int64
		b        int64
		expected bool
	}{
		{"less", 100, 200, true},
		{"greater", 200, 100, false},
		{"equal", 100, 100, false},
		{"positive greater than negative", 100, -100, false},
		{"negative less than positive", -100, 100, true},
		{"negative numbers", -100, -50, true},
		{"zero greater than negative", 0, -100, false},
		{"zero less than positive", 0, 100, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := New(tt.a)
			b := New(tt.b)
			if got := a.LessThan(b); got != tt.expected {
				t.Errorf("LessThan() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsZero(t *testing.T) {
	tests := []struct {
		name     string
		amount   int64
		expected bool
	}{
		{"zero amount", 0, true},
		{"positive amount", 100, false},
		{"negative amount", -100, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(tt.amount)
			if got := m.IsZero(); got != tt.expected {
				t.Errorf("IsZero() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsNegative(t *testing.T) {
	tests := []struct {
		name     string
		amount   int64
		expected bool
	}{
		{"negative amount", -100, true},
		{"zero amount", 0, false},
		{"positive amount", 100, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(tt.amount)
			if got := m.IsNegative(); got != tt.expected {
				t.Errorf("IsNegative() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestInt64(t *testing.T) {
	tests := []struct {
		name   string
		amount int64
		want   int64
	}{
		{"zero", 0, 0},
		{"positive", 100, 100},
		{"negative", -100, -100},
		{"max int64", 9223372036854775807, 9223372036854775807},
		{"min int64", -9223372036854775808, -9223372036854775808},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(tt.amount)
			if got := m.Int64(); got != tt.want {
				t.Errorf("Int64() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestImmutability(t *testing.T) {
	// Ensure that operations return new Money instances
	original := New(100)

	_ = original.Add(New(50))
	if original.Int64() != 100 {
		t.Errorf("Add() modified original: %d", original.Int64())
	}

	_ = original.Sub(New(30))
	if original.Int64() != 100 {
		t.Errorf("Sub() modified original: %d", original.Int64())
	}

	_ = original.Neg()
	if original.Int64() != 100 {
		t.Errorf("Neg() modified original: %d", original.Int64())
	}

	_ = original.Mul(2)
	if original.Int64() != 100 {
		t.Errorf("Mul() modified original: %d", original.Int64())
	}

	// All original values should remain unchanged
	if original.Int64() != 100 {
		t.Errorf("Original was modified: %d", original.Int64())
	}
}

func TestChaining(t *testing.T) {
	// Test method chaining
	start := New(100)
	result := start.Add(New(50)).Sub(New(30)).Mul(2)
	// (100 + 50 - 30) * 2 = 120 * 2 = 240
	expected := int64(240)
	if result.Int64() != expected {
		t.Errorf("Chaining result = %d, want %d", result.Int64(), expected)
	}
	// Original should be unchanged
	if start.Int64() != 100 {
		t.Errorf("Chaining modified original: %d", start.Int64())
	}
}

func TestAddOverflow(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Add() did not panic on overflow")
		} else if r != "money: addition overflow" {
			t.Errorf("Add() panic = %v, want 'money: addition overflow'", r)
		}
	}()

	maxInt64 := New(9223372036854775807) // math.MaxInt64
	_ = maxInt64.Add(New(1)) // Should panic: MaxInt64 + 1 overflows
}

func TestSubUnderflow(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Sub() did not panic on underflow")
		} else if r != "money: subtraction underflow" {
			t.Errorf("Sub() panic = %v, want 'money: subtraction underflow'", r)
		}
	}()

	minInt64 := New(-9223372036854775808) // math.MinInt64
	_ = minInt64.Sub(New(1)) // Should panic: MinInt64 - 1 underflows
}

func TestNegOverflow(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Neg() did not panic on overflow")
		} else if r != "money: negation overflow" {
			t.Errorf("Neg() panic = %v, want 'money: negation overflow'", r)
		}
	}()

	minInt64 := New(-9223372036854775808) // math.MinInt64
	_ = minInt64.Neg() // Should panic: -MinInt64 overflows
}

func TestMulOverflow(t *testing.T) {
	tests := []struct {
		name   string
		amount int64
		factor int64
	}{
		{"large positive * 2", 5000000000000000000, 2},
		{"max * 2", 9223372036854775807, 2},
		{"min * 2", -9223372036854775807, 2},
		{"large positive * large positive", 3000000000, 4000000000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("Mul() did not panic on overflow")
				} else if r != "money: multiplication overflow" {
					t.Errorf("Mul() panic = %v, want 'money: multiplication overflow'", r)
				}
			}()
			m := New(tt.amount)
			_ = m.Mul(tt.factor)
		})
	}
}
