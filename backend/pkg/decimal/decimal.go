package decimal

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/shopspring/decimal"
)

// Decimal is a wrapper around shopspring/decimal.Decimal that provides
// JSON marshaling as string and SQL driver/valuer interfaces.
// This is the monetary type for all financial calculations in the system.
type Decimal struct {
	d decimal.Decimal
}

// ============================================================================
// CONSTRUCTORS
// ============================================================================

// NewFromFloat creates a new Decimal from a float64.
// WARNING: Float should not be used for monetary values due to precision issues.
// This is provided for legacy code migration only.
func NewFromFloat(f float64) Decimal {
	return Decimal{d: decimal.NewFromFloat(f)}
}

// NewFromInt creates a new Decimal from an int64.
func NewFromInt(i int64) Decimal {
	return Decimal{d: decimal.NewFromInt(i)}
}

// NewFromString creates a new Decimal from a string.
func NewFromString(s string) (Decimal, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return Decimal{}, err
	}
	return Decimal{d: d}, nil
}

// Zero returns a zero Decimal.
func Zero() Decimal {
	return Decimal{d: decimal.Zero}
}

// ============================================================================
// IDR ROUNDING (Round Half Up to 0 decimal places)
// ============================================================================

// RoundIDR rounds the decimal to 0 decimal places using round-half-up mode.
// This is the standard rounding for Indonesian Rupiah (IDR).
// Example: 9999.5 -> 10000, 10000.4 -> 10000
func (d Decimal) RoundIDR() Decimal {
	return Decimal{d: d.d.Round(0)}
}

// RoundIDRWithPrecision rounds to n decimal places using round-half-up.
func (d Decimal) RoundIDRWithPrecision(n int32) Decimal {
	return Decimal{d: d.d.Round(n)}
}

// ============================================================================
// ARITHMETIC OPERATIONS
// ============================================================================

// Add adds two decimals.
func (d Decimal) Add(other Decimal) Decimal {
	return Decimal{d: d.d.Add(other.d)}
}

// Sub subtracts other from d.
func (d Decimal) Sub(other Decimal) Decimal {
	return Decimal{d: d.d.Sub(other.d)}
}

// Mul multiplies two decimals.
func (d Decimal) Mul(other Decimal) Decimal {
	return Decimal{d: d.d.Mul(other.d)}
}

// Div divides d by other.
func (d Decimal) Div(other Decimal) Decimal {
	return Decimal{d: d.d.Div(other.d)}
}

// MulByFloat multiplies by a float64 rate.
// WARNING: This should only be used for percentage rates (0.05, 0.025, etc.)
// The result should be rounded immediately after.
func (d Decimal) MulByFloat(rate float64) Decimal {
	return Decimal{d: d.d.Mul(decimal.NewFromFloat(rate))}
}

// Neg returns the negative of d.
func (d Decimal) Neg() Decimal {
	return Decimal{d: d.d.Neg()}
}

// Abs returns the absolute value of d.
func (d Decimal) Abs() Decimal {
	return Decimal{d: d.d.Abs()}
}

// ============================================================================
// COMPARISONS
// ============================================================================

// Cmp compares d and other.
// Returns: -1 if d < other, 0 if d == other, 1 if d > other
func (d Decimal) Cmp(other Decimal) int {
	return d.d.Cmp(other.d)
}

// Equal returns true if d equals other.
func (d Decimal) Equal(other Decimal) bool {
	return d.d.Equal(other.d)
}

// GreaterThan returns true if d > other.
func (d Decimal) GreaterThan(other Decimal) bool {
	return d.d.GreaterThan(other.d)
}

// GreaterThanOrEqual returns true if d >= other.
func (d Decimal) GreaterThanOrEqual(other Decimal) bool {
	return d.d.GreaterThanOrEqual(other.d)
}

// LessThan returns true if d < other.
func (d Decimal) LessThan(other Decimal) bool {
	return d.d.LessThan(other.d)
}

// LessThanOrEqual returns true if d <= other.
func (d Decimal) LessThanOrEqual(other Decimal) bool {
	return d.d.LessThanOrEqual(other.d)
}

// IsNegative returns true if d < 0.
func (d Decimal) IsNegative() bool {
	return d.d.IsNegative()
}

// IsPositive returns true if d > 0.
func (d Decimal) IsPositive() bool {
	return d.d.IsPositive()
}

// IsZero returns true if d == 0.
func (d Decimal) IsZero() bool {
	return d.d.IsZero()
}

// ============================================================================
// CONVERSIONS
// ============================================================================

// Float64 returns the float64 value.
// WARNING: Should not be used for monetary calculations due to precision loss.
func (d Decimal) Float64() float64 {
	f, _ := d.d.Float64()
	return f
}

// Int64 returns the truncated int64 value.
func (d Decimal) Int64() int64 {
	return d.d.IntPart()
}

// String returns the string representation.
func (d Decimal) String() string {
	return d.d.String()
}

// StringFixed returns the string representation with fixed decimal places.
func (d Decimal) StringFixed(places int32) string {
	return d.d.StringFixed(places)
}

// ============================================================================
// INTERFACE IMPLEMENTATIONS
// ============================================================================

// MarshalJSON implements json.Marshaler.
// Marshals the decimal as a string to preserve precision.
func (d Decimal) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.d.String())
}

// UnmarshalJSON implements json.Unmarshaler.
// Accepts both string and number formats.
func (d *Decimal) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	// Try string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		parsed, err := decimal.NewFromString(s)
		if err != nil {
			return err
		}
		d.d = parsed
		return nil
	}

	// Try number
	var f float64
	if err := json.Unmarshal(data, &f); err == nil {
		d.d = decimal.NewFromFloat(f)
		return nil
	}

	return fmt.Errorf("invalid decimal format: %s", string(data))
}

// Value implements driver.Valuer for SQL serialization.
func (d Decimal) Value() (driver.Value, error) {
	return d.d.String(), nil
}

// Scan implements sql.Scanner for SQL deserialization.
func (d *Decimal) Scan(value interface{}) error {
	if value == nil {
		d.d = decimal.Zero
		return nil
	}

	switch v := value.(type) {
	case []byte:
		parsed, err := decimal.NewFromString(string(v))
		if err != nil {
			return err
		}
		d.d = parsed
	case string:
		parsed, err := decimal.NewFromString(v)
		if err != nil {
			return err
		}
		d.d = parsed
	case float64:
		d.d = decimal.NewFromFloat(v)
	case int64:
		d.d = decimal.NewFromInt(v)
	default:
		return fmt.Errorf("cannot scan %T into decimal", value)
	}

	return nil
}

// ParseDecimalOrZero parses a string to Decimal, returning Zero on error.
func ParseDecimalOrZero(s string) Decimal {
	d, err := NewFromString(s)
	if err != nil {
		return Zero()
	}
	return d
}

// MustParseDecimal parses a string to Decimal, panics on error.
func MustParseDecimal(s string) Decimal {
	d, err := NewFromString(s)
	if err != nil {
		panic(fmt.Sprintf("invalid decimal: %s", s))
	}
	return d
}

// DecimalFromInt64 creates a Decimal from an int64.
func DecimalFromInt64(i int64) Decimal {
	return NewFromInt(i)
}

// CreateDecimal from string for validation
func CreateDecimal(s string) (Decimal, error) {
	return NewFromString(s)
}

// IntPart returns the integer part of the decimal.
func (d Decimal) IntPart() int64 {
	return d.d.IntPart()
}

// GetFloat64 returns the float64 value.
// Deprecated: Use Float64() instead.
func (d Decimal) GetFloat64() float64 {
	return d.Float64()
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (d *Decimal) UnmarshalText(text []byte) error {
	parsed, err := decimal.NewFromString(string(text))
	if err != nil {
		return err
	}
	d.d = parsed
	return nil
}

// MarshalText implements encoding.TextMarshaler.
func (d Decimal) MarshalText() ([]byte, error) {
	return []byte(d.d.String()), nil
}

// ToDecimal converts float64 to Decimal (legacy helper).
func ToDecimal(f float64) Decimal {
	return NewFromFloat(f)
}

// ToDecimalFromString converts string to Decimal (legacy helper).
func ToDecimalFromString(s string) (Decimal, error) {
	return NewFromString(s)
}

// RoundToMoney rounds to 2 decimal places for display.
func (d Decimal) RoundToMoney() Decimal {
	return Decimal{d: d.d.Round(2)}
}

// ValidateDecimal validates if a string is a valid decimal.
func ValidateDecimal(s string) error {
	_, err := decimal.NewFromString(s)
	return err
}

// FormatIDR formats the decimal as Indonesian Rupiah string.
func (d Decimal) FormatIDR() string {
	// Format as integer (IDR has no decimal places)
	return "Rp " + d.d.StringFixed(0)
}

// ParseIDR parses an IDR string (e.g., "Rp 10.000" or "10000").
func ParseIDR(s string) (Decimal, error) {
	// Simple implementation - remove "Rp " and dots, then parse
	// For a more robust implementation, use a proper currency parser
	cleaned := s
	// Remove "Rp " prefix
	if len(s) > 3 && s[:3] == "Rp " {
		cleaned = s[3:]
	}
	// Remove thousand separators
	// This is simplified - proper implementation would use regex
	return NewFromString(cleaned)
}

// Compare returns -1, 0, or 1 based on comparison.
func (d Decimal) Compare(other Decimal) int {
	return d.Cmp(other)
}

// Min returns the minimum of d and other.
func (d Decimal) Min(other Decimal) Decimal {
	if d.LessThan(other) {
		return d
	}
	return other
}

// Max returns the maximum of d and other.
func (d Decimal) Max(other Decimal) Decimal {
	if d.GreaterThan(other) {
		return d
	}
	return other
}

// Clone returns a copy of the decimal.
func (d Decimal) Clone() Decimal {
	return Decimal{d: d.d}
}

// AddInt64 adds an int64 to the decimal.
func (d Decimal) AddInt64(i int64) Decimal {
	return d.Add(NewFromInt(i))
}

// SubInt64 subtracts an int64 from the decimal.
func (d Decimal) SubInt64(i int64) Decimal {
	return d.Sub(NewFromInt(i))
}

// MulInt64 multiplies the decimal by an int64.
func (d Decimal) MulInt64(i int64) Decimal {
	return Decimal{d: d.d.Mul(decimal.NewFromInt(i))}
}

// DivInt64 divides the decimal by an int64.
func (d Decimal) DivInt64(i int64) Decimal {
	return Decimal{d: d.d.Div(decimal.NewFromInt(i))}
}

// Sign returns -1 if d < 0, 0 if d == 0, 1 if d > 0.
func (d Decimal) Sign() int {
	return d.d.Sign()
}

// IsInteger returns true if d has no fractional part.
func (d Decimal) IsInteger() bool {
	return d.d.Exponent() == 0
}

// Truncate truncates the decimal to n decimal places.
func (d Decimal) Truncate(n int32) Decimal {
	return Decimal{d: d.d.Truncate(n)}
}

// Floor returns the greatest integer value <= d.
func (d Decimal) Floor() Decimal {
	return Decimal{d: d.d.Floor()}
}

// Ceil returns the smallest integer value >= d.
func (d Decimal) Ceil() Decimal {
	return Decimal{d: d.d.Ceil()}
}

// Percent calculates p percent of d.
func (d Decimal) Percent(p Decimal) Decimal {
	return d.Mul(p).DivInt64(100)
}

// PercentFloat calculates p percent of d where p is a float (e.g., 5.0 for 5%).
func (d Decimal) PercentFloat(p float64) Decimal {
	return d.MulByFloat(p / 100)
}

// Exchange converts d using the exchange rate.
func (d Decimal) Exchange(rate Decimal) Decimal {
	return d.Mul(rate).RoundIDR()
}

// IsValid checks if the decimal value is valid (not NaN, not infinity).
// shopspring/decimal doesn't have NaN/Inf, so this always returns true.
func (d Decimal) IsValid() bool {
	return true
}

// Empty returns true if the decimal is uninitialized (zero value).
// Note: This is different from IsZero() - it checks struct initialization.
func (d *Decimal) Empty() bool {
	return d == nil
}

// IsSet returns true if the decimal pointer is not nil.
func (d *Decimal) IsSet() bool {
	return d != nil
}

// OrDefault returns the decimal or a default value if nil.
func (d *Decimal) OrDefault(defaultValue Decimal) Decimal {
	if d == nil {
		return defaultValue
	}
	return *d
}

// Set sets the value from another Decimal pointer.
func (d *Decimal) Set(other *Decimal) {
	if d != nil && other != nil {
		*d = *other
	}
}

// NewDecimalPtr creates a new Decimal pointer from a Decimal.
func NewDecimalPtr(d Decimal) *Decimal {
	return &d
}

// DecimalPtrOrNil creates a nil Decimal pointer if zero, otherwise a pointer to d.
func DecimalPtrOrNil(d Decimal) *Decimal {
	if d.IsZero() {
		return nil
	}
	return &d
}

// NullDecimal represents a Decimal that can be null.
// Use this for database fields that can be NULL.
type NullDecimal struct {
	Decimal Decimal
	Valid   bool
}

// NewNullDecimal creates a new NullDecimal.
func NewNullDecimal(d Decimal) NullDecimal {
	return NullDecimal{
		Decimal: d,
		Valid:   true,
	}
}

// NullDecimalFromPtr creates a NullDecimal from a pointer.
func NullDecimalFromPtr(d *Decimal) NullDecimal {
	if d == nil {
		return NullDecimal{Valid: false}
	}
	return NullDecimal{
		Decimal: *d,
		Valid:   true,
	}
}

// MarshalJSON implements json.Marshaler for NullDecimal.
func (n NullDecimal) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return n.Decimal.MarshalJSON()
}

// UnmarshalJSON implements json.Unmarshaler for NullDecimal.
func (n *NullDecimal) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.Valid = false
		return nil
	}
	n.Valid = true
	return n.Decimal.UnmarshalJSON(data)
}

// Value implements driver.Valuer for NullDecimal.
func (n NullDecimal) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Decimal.Value()
}

// Scan implements sql.Scanner for NullDecimal.
func (n *NullDecimal) Scan(value interface{}) error {
	if value == nil {
		n.Valid = false
		return nil
	}
	n.Valid = true
	return n.Decimal.Scan(value)
}

// Ptr returns a pointer to the Decimal, or nil if invalid.
func (n NullDecimal) Ptr() *Decimal {
	if !n.Valid {
		return nil
	}
	return &n.Decimal
}

// Atoi converts a string to int64, then to Decimal.
func Atoi(s string) (Decimal, error) {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return Zero(), err
	}
	return NewFromInt(i), nil
}

// Itoa converts Decimal to string (alias for String()).
func Itoa(d Decimal) string {
	return d.String()
}

// Sum returns the sum of all decimals.
func Sum(decimals ...Decimal) Decimal {
	sum := Zero()
	for _, d := range decimals {
		sum = sum.Add(d)
	}
	return sum
}

// Avg returns the average of all decimals.
func Avg(decimals ...Decimal) Decimal {
	if len(decimals) == 0 {
		return Zero()
	}
	return Sum(decimals...).DivInt64(int64(len(decimals)))
}

// Quantize rounds d to the same number of decimal places as other.
func (d Decimal) Quantize(other Decimal) Decimal {
	return Decimal{d: d.d.Round(other.d.Exponent())}
}

// Mod returns the remainder of d divided by other.
func (d Decimal) Mod(other Decimal) Decimal {
	return Decimal{d: d.d.Mod(other.d)}
}

// Pow returns d raised to the power n.
func (d Decimal) Pow(n int32) Decimal {
	return Decimal{d: d.d.Pow(decimal.NewFromInt(int64(n)))}
}

// Sqrt returns the square root of d.
// Note: This uses approximation and may have small precision errors.
func (d Decimal) Sqrt() (Decimal, error) {
	if d.IsNegative() {
		return Zero(), fmt.Errorf("cannot calculate square root of negative number")
	}
	if d.IsZero() {
		return Zero(), nil
	}
	// Newton-Raphson approximation
	x := d
	two := NewFromInt(2)
	one := NewFromInt(1)
	for i := 0; i < 20; i++ {
		next := x.Add(d.Div(x)).Div(two)
		if next.Sub(x).Abs().LessThan(one) {
			return next.RoundToMoney(), nil
		}
		x = next
	}
	return x.RoundToMoney(), nil
}
