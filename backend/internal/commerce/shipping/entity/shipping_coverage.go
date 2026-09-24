package entity

import (
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/money"
)

// ShippingCoverage defines geographic coverage for a shipping option at province level.
// Contains province-level rate and availability.
// NOTE: estimated_days was dropped by migration 000014.
// CityOverrides carries the city-level qualification rows for this coverage.
// It is populated by the application layer for read paths that need it
// (seller edit form); persistence layers do not hydrate it implicitly.
type ShippingCoverage struct {
	ID               uuid.UUID
	ShippingSetupID uuid.UUID
	ProvinceCode     string
	ProvinceName     string
	ProvinceRate     money.Money
	IsAvailable      bool
	CityOverrides    []*CityOverride
	CreatedAt        time.Time
}

// NewShippingCoverage creates a new shipping coverage.
// By default, coverage is available with zero rate.
func NewShippingCoverage(
	shippingSetupID uuid.UUID,
	provinceCode string,
	provinceName string,
) *ShippingCoverage {
	return &ShippingCoverage{
		ID:               uuid.New(),
		ShippingSetupID: shippingSetupID,
		ProvinceCode:     provinceCode,
		ProvinceName:     provinceName,
		ProvinceRate:     money.New(0),
		IsAvailable:      true,
		CreatedAt:        time.Now(),
	}
}

// WithRate sets the province rate.
func (sc *ShippingCoverage) WithRate(rate money.Money) *ShippingCoverage {
	sc.ProvinceRate = rate
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




