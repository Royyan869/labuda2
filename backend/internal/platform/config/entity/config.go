package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Config represents a platform-wide configuration key-value pair.
// Values are snapshot at domain entity creation time (order, subscription, etc.).
// This ensures historical transactions are not affected by config changes.
type Config struct {
	Key       string
	ValueNum  *decimal.Decimal
	ValueText *string
	UpdatedBy *uuid.UUID
	UpdatedAt time.Time
}

// ConfigNotFoundError is returned when a requested config key does not exist.
type ConfigNotFoundError struct {
	Key string
}

func (e *ConfigNotFoundError) Error() string {
	return fmt.Sprintf("config key not found: %s", e.Key)
}

// InvalidConfigTypeError is returned when a config value has the wrong type.
type InvalidConfigTypeError struct {
	Key          string
	ExpectedType string
}

func (e *InvalidConfigTypeError) Error() string {
	return fmt.Sprintf("config key %s has invalid type (expected %s)", e.Key, e.ExpectedType)
}

// NumericValue returns the numeric value or an error if not set.
func (c *Config) NumericValue() (decimal.Decimal, error) {
	if c.ValueNum == nil {
		return decimal.Decimal{}, &InvalidConfigTypeError{
			Key:          c.Key,
			ExpectedType: "numeric",
		}
	}
	return *c.ValueNum, nil
}

// TextValue returns the text value or an error if not set.
func (c *Config) TextValue() (string, error) {
	if c.ValueText == nil {
		return "", &InvalidConfigTypeError{
			Key:          c.Key,
			ExpectedType: "text",
		}
	}
	return *c.ValueText, nil
}

// NewNumericConfig creates a new config with a numeric value.
func NewNumericConfig(key string, value decimal.Decimal, updatedBy uuid.UUID) *Config {
	now := time.Now()
	return &Config{
		Key:       key,
		ValueNum:  &value,
		ValueText: nil,
		UpdatedBy: &updatedBy,
		UpdatedAt: now,
	}
}

// NewTextConfig creates a new config with a text value.
func NewTextConfig(key string, value string, updatedBy uuid.UUID) *Config {
	now := time.Now()
	return &Config{
		Key:       key,
		ValueNum:  nil,
		ValueText: &value,
		UpdatedBy: &updatedBy,
		UpdatedAt: now,
	}
}


