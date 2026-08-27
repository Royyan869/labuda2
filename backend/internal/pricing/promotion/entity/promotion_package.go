package entity

import (
	"time"

	"github.com/google/uuid"
)

// PromotionPackage represents a purchasable promotion package.
// This is the product catalog - read-only after creation.
//
// Business truth: Packages define what users can purchase.
// They contain total duration, validity window, and allowed target types.
// No placement tier — owner decision: promotion has no tier.
type PromotionPackage struct {
	// Identity
	ID uuid.UUID

	// Package definition
	Name                string       // "Promote Basic (3 Days)"
	TotalDurationHours  int          // 72, 168, 336
	ValidityWindowHours int          // 336 (14 days max to use)
	PriceAmount         int          // One-time purchase price, Rupiah integer
	AllowedTargetTypes  []TargetType // ['for_sale'], ['for_sale', 'auction']

	// State
	IsActive bool

	// Timestamps
	CreatedAt time.Time
}

// NewPromotionPackage creates a new promotion package with validation.
func NewPromotionPackage(
	name string,
	totalDurationHours int,
	validityWindowHours int,
	priceAmount int,
	allowedTargetTypes []TargetType,
) (*PromotionPackage, error) {
	if name == "" {
		return nil, &ValidationError{Field: "name", Message: "name is required"}
	}
	if totalDurationHours <= 0 {
		return nil, &ValidationError{Field: "total_duration_hours", Message: "must be positive"}
	}
	if validityWindowHours <= 0 {
		return nil, &ValidationError{Field: "validity_window_hours", Message: "must be positive"}
	}
	if priceAmount < 0 {
		return nil, &ValidationError{Field: "price_amount", Message: "cannot be negative"}
	}
	if len(allowedTargetTypes) == 0 {
		return nil, &ValidationError{Field: "allowed_target_types", Message: "at least one required"}
	}

	// Validate all target types
	for _, tt := range allowedTargetTypes {
		if !tt.IsValid() {
			return nil, &ValidationError{Field: "allowed_target_types", Message: "invalid target type"}
		}
	}

	return &PromotionPackage{
		ID:                  uuid.New(),
		Name:                name,
		TotalDurationHours:  totalDurationHours,
		ValidityWindowHours: validityWindowHours,
		PriceAmount:         priceAmount,
		AllowedTargetTypes:  allowedTargetTypes,
		IsActive:            true,
		CreatedAt:           time.Now(),
	}, nil
}

// AllowsTargetType returns true if the package allows promoting the given target type.
func (p *PromotionPackage) AllowsTargetType(targetType TargetType) bool {
	for _, allowed := range p.AllowedTargetTypes {
		if allowed == targetType {
			return true
		}
	}
	return false
}

// CalculateExpiry returns the expiry timestamp for an ownership purchased at the given time.
func (p *PromotionPackage) CalculateExpiry(purchasedAt time.Time) time.Time {
	return purchasedAt.Add(time.Duration(p.ValidityWindowHours) * time.Hour)
}

// ValidationError is returned when validation fails.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}
