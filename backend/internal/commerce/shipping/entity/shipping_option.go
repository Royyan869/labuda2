// DOMAIN: COMMERCE
// NOTE: Seller-configured shipping options for commerce fulfillment

package entity

import (
	"time"

	"github.com/google/uuid"
)

// ShippingOption represents a seller-configured shipping option.
// Contains metadata only - pricing and delivery times are defined per location (coverage/override).
type ShippingOption struct {
	ID             uuid.UUID
	SellerID       uuid.UUID
	Name           string
	TransportType  TransportType
	ExpeditionName *string // Optional expedition/company name (free text)
	IsActive       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// NewShippingOption creates a new active shipping option.
// expeditionName can be empty/nil for generic transport types.
func NewShippingOption(
	sellerID uuid.UUID,
	name string,
	transportType TransportType,
	expeditionName string,
) *ShippingOption {
	var expName *string
	if expeditionName != "" {
		expName = &expeditionName
	}

	now := time.Now()
	return &ShippingOption{
		ID:             uuid.New(),
		SellerID:       sellerID,
		Name:           name,
		TransportType:  transportType,
		ExpeditionName: expName,
		IsActive:       true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// Deactivate marks the shipping option as inactive.
func (so *ShippingOption) Deactivate() {
	so.IsActive = false
	so.UpdatedAt = time.Now()
}

// Activate marks the shipping option as active.
func (so *ShippingOption) Activate() {
	so.IsActive = true
	so.UpdatedAt = time.Now()
}

// SetExpeditionName updates the expedition name.
// Pass empty string to clear the expedition name.
func (so *ShippingOption) SetExpeditionName(name string) {
	if name == "" {
		so.ExpeditionName = nil
	} else {
		so.ExpeditionName = &name
	}
	so.UpdatedAt = time.Now()
}

// GetExpeditionName returns the expedition name or empty string if not set.
func (so *ShippingOption) GetExpeditionName() string {
	if so.ExpeditionName == nil {
		return ""
	}
	return *so.ExpeditionName
}


