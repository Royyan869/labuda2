package money

import "fmt"

// Money represents a monetary value using int64 as the smallest unit.
// The amount field is private to ensure immutability.
type Money struct {
	amount int64
}

// New creates a new Money value with the specified amount in smallest unit.
func New(amount int64) Money {
	return Money{amount: amount}
}

// Zero creates a Money value with zero amount.
func Zero() Money {
	return Money{amount: 0}
}

// Add returns a new Money with the sum of this and other.
// Panics on overflow as it represents a programmer error / financial invariant violation.
func (m Money) Add(other Money) Money {
	result := m.amount + other.amount
	// Check for signed overflow:
	// - If both operands are positive and result is negative → overflow
	// - If both operands are negative and result is positive → overflow
	if (other.amount > 0 && result < m.amount) || (other.amount < 0 && result > m.amount) {
		panic("money: addition overflow")
	}
	return Money{amount: result}
}

// Sub returns a new Money with the difference of this minus other.
// Panics on underflow as it represents a programmer error / financial invariant violation.
func (m Money) Sub(other Money) Money {
	result := m.amount - other.amount
	// Check for signed underflow:
	// - If subtracting positive and result > original → underflow
	// - If subtracting negative and result < original → overflow
	if (other.amount > 0 && result > m.amount) || (other.amount < 0 && result < m.amount) {
		panic("money: subtraction underflow")
	}
	return Money{amount: result}
}

// Neg returns a new Money with the negated amount.
// Panics if amount is MinInt64 as negation would overflow.
func (m Money) Neg() Money {
	if m.amount == -9223372036854775808 { // math.MinInt64
		panic("money: negation overflow")
	}
	return Money{amount: -m.amount}
}

// Mul returns a new Money multiplied by the given integer factor.
// Panics on overflow as it represents a programmer error / financial invariant violation.
func (m Money) Mul(n int64) Money {
	result := m.amount * n
	// Check for signed overflow using division verification
	// If n != 0 and result / n != original, overflow occurred
	if n != 0 && result/n != m.amount {
		panic("money: multiplication overflow")
	}
	// Edge case: -1 * MinInt64 overflows (but division check misses it)
	if n == -1 && m.amount == -9223372036854775808 {
		panic("money: multiplication overflow")
	}
	return Money{amount: result}
}

// Equal returns true if this Money equals other.
func (m Money) Equal(other Money) bool {
	return m.amount == other.amount
}

// GreaterThan returns true if this Money is greater than other.
func (m Money) GreaterThan(other Money) bool {
	return m.amount > other.amount
}

// LessThan returns true if this Money is less than other.
func (m Money) LessThan(other Money) bool {
	return m.amount < other.amount
}

// IsZero returns true if this Money has zero amount.
func (m Money) IsZero() bool {
	return m.amount == 0
}

// IsNegative returns true if this Money has a negative amount.
func (m Money) IsNegative() bool {
	return m.amount < 0
}

// Int64 returns the amount as int64 for database/storage purposes.
func (m Money) Int64() int64 {
	return m.amount
}

// Value implements driver.Valuer for SQL serialization.
// Returns Money as int64 for BIGINT storage.
func (m Money) Value() (interface{}, error) {
	return m.amount, nil
}

// Scan implements sql.Scanner for SQL deserialization.
// Scans BIGINT value into Money.
func (m *Money) Scan(value interface{}) error {
	if value == nil {
		m.amount = 0
		return nil
	}

	switch v := value.(type) {
	case int64:
		m.amount = v
	case []byte:
		var i int64
		_, err := fmt.Sscanf(string(v), "%d", &i)
		if err != nil {
			return err
		}
		m.amount = i
	case string:
		var i int64
		_, err := fmt.Sscanf(v, "%d", &i)
		if err != nil {
			return err
		}
		m.amount = i
	default:
		return fmt.Errorf("cannot scan %T into money.Money", value)
	}

	return nil
}
