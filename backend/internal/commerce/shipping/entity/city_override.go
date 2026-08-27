package entity

import (
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/money"
)

// CityOverride represents city-specific rate and availability overrides.
// Nil values mean "use parent coverage default".
type CityOverride struct {
	ID                 uuid.UUID
	ShippingCoverageID uuid.UUID
	CityCode           string
	CityName           string
	Rate               *money.Money // nil = use province_rate
	EstimatedDays      *string      // nil = use coverage default, value = override (e.g., "same day")
	IsAvailable        *bool        // nil = use coverage default
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// NewCityOverride creates a new city override with all values using defaults (nil).
func NewCityOverride(
	shippingCoverageID uuid.UUID,
	cityCode string,
	cityName string,
) *CityOverride {
	now := time.Now()
	return &CityOverride{
		ID:                 uuid.New(),
		ShippingCoverageID: shippingCoverageID,
		CityCode:           cityCode,
		CityName:           cityName,
		Rate:               nil,
		EstimatedDays:      nil,
		IsAvailable:        nil,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

// SetRate sets a specific rate for this city.
func (co *CityOverride) SetRate(rate money.Money) {
	co.Rate = &rate
	co.UpdatedAt = time.Now()
}

// ClearRate removes rate override (uses province_rate).
func (co *CityOverride) ClearRate() {
	co.Rate = nil
	co.UpdatedAt = time.Now()
}

// SetEstimatedDays sets estimated delivery time for this city.
func (co *CityOverride) SetEstimatedDays(days string) {
	co.EstimatedDays = &days
	co.UpdatedAt = time.Now()
}

// ClearEstimatedDays removes estimated days override (uses coverage default).
func (co *CityOverride) ClearEstimatedDays() {
	co.EstimatedDays = nil
	co.UpdatedAt = time.Now()
}

// SetAvailable overrides availability for this city.
func (co *CityOverride) SetAvailable(available bool) {
	co.IsAvailable = &available
	co.UpdatedAt = time.Now()
}

// ClearAvailableOverride removes availability override (uses coverage default).
func (co *CityOverride) ClearAvailableOverride() {
	co.IsAvailable = nil
	co.UpdatedAt = time.Now()
}

// GetEffectiveRate returns the rate to use (override or province rate).
func (co *CityOverride) GetEffectiveRate(provinceRate money.Money) money.Money {
	if co.Rate != nil {
		return *co.Rate
	}
	return provinceRate
}

// GetEffectiveEstimatedDays returns the estimated days to use (override or default).
func (co *CityOverride) GetEffectiveEstimatedDays(defaultDays string) string {
	if co.EstimatedDays != nil {
		return *co.EstimatedDays
	}
	return defaultDays
}

// GetEffectiveAvailability returns availability to use (override or default).
func (co *CityOverride) GetEffectiveAvailability(defaultAvailable bool) bool {
	if co.IsAvailable != nil {
		return *co.IsAvailable
	}
	return defaultAvailable
}


