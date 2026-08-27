package entity

import (
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/money"
)

// ShippingCoverage defines geographic coverage for a shipping option at province level.
// Contains province-level rate and estimated delivery time.
type ShippingCoverage struct {
	ID               uuid.UUID
	ShippingOptionID uuid.UUID
	ProvinceCode     string
	ProvinceName     string
	ProvinceRate     money.Money
	EstimatedDays    *string // e.g., "1-2 hari", "3-5 hari" (nil = not specified)
	IsAvailable      bool
	CreatedAt        time.Time
}

// NewShippingCoverage creates a new shipping coverage.
// By default, coverage is available with zero rate.
func NewShippingCoverage(
	shippingOptionID uuid.UUID,
	provinceCode string,
	provinceName string,
) *ShippingCoverage {
	return &ShippingCoverage{
		ID:               uuid.New(),
		ShippingOptionID: shippingOptionID,
		ProvinceCode:     provinceCode,
		ProvinceName:     provinceName,
		ProvinceRate:     money.New(0),
		EstimatedDays:    nil,
		IsAvailable:      true,
		CreatedAt:        time.Now(),
	}
}

// WithRate sets the province rate.
func (sc *ShippingCoverage) WithRate(rate money.Money) *ShippingCoverage {
	sc.ProvinceRate = rate
	return sc
}

// WithEstimatedDays sets the estimated delivery time.
func (sc *ShippingCoverage) WithEstimatedDays(days string) *ShippingCoverage {
	sc.EstimatedDays = &days
	return sc
}

// MarkUnavailable marks this province as unavailable for shipping.
func (sc *ShippingCoverage) MarkUnavailable() {
	sc.IsAvailable = false
}

// MarkAvailable marks this province as available for shipping.
func (sc *ShippingCoverage) MarkAvailable() {
	sc.IsAvailable = true
}

// GetEstimatedDays returns the estimated days or empty string if not set.
func (sc *ShippingCoverage) GetEstimatedDays() string {
	if sc.EstimatedDays == nil {
		return ""
	}
	return *sc.EstimatedDays
}


