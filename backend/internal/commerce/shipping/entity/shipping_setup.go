// DOMAIN: COMMERCE
// NOTE: Seller-configured shipping options for commerce fulfillment

package entity

import (
	"time"

	"github.com/google/uuid"
)

// ShippingSetup represents a seller-configured shipping setup.
// Contains metadata only - pricing and delivery times are defined per location (coverage/override).
// NOTE: expedition_name was dropped by migration 000014.
type ShippingSetup struct {
	ID            uuid.UUID
	SellerID      uuid.UUID
	Name          string
	TransportType TransportType
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// NewShippingSetup creates a new active shipping option.
func NewShippingSetup(
	sellerID uuid.UUID,
	name string,
	transportType TransportType,
) *ShippingSetup {
	now := time.Now()
	return &ShippingSetup{
		ID:            uuid.New(),
		SellerID:      sellerID,
		Name:          name,
		TransportType: transportType,
		IsActive:      true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// Deactivate marks the shipping option as inactive.
func (so *ShippingSetup) Deactivate() {
	so.IsActive = false
	so.UpdatedAt = time.Now()
}

// Activate marks the shipping option as active.
func (so *ShippingSetup) Activate() {
	so.IsActive = true
	so.UpdatedAt = time.Now()
}




