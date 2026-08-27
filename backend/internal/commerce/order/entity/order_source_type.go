package entity

// OrderSourceType represents the origin of an order in the unified commerce model.
//
// Valid source types:
// - for_sale: Direct purchase from a fixed-price sale surface
// - negotiation: Order created from an accepted negotiation session
// - auction: Order created from an auction winning bid
//
// MUST match database enum: order_source_enum
type OrderSourceType string

const (
	// OrderSourceForSale represents an order created from a fixed-price listing via direct purchase.
	// This includes fixed-price purchases (NOT negotiated).
	OrderSourceForSale OrderSourceType = "for_sale"

	// OrderSourceNegotiation represents an order created from an accepted negotiation session.
	// Used when a buyer completes checkout after a seller accepts their negotiated price.
	OrderSourceNegotiation OrderSourceType = "negotiation"

	// OrderSourceAuction represents an order created from an auction winning bid.
	// Created automatically when auction ends with a valid winner.
	OrderSourceAuction OrderSourceType = "auction"
)

// IsValid checks if the order source type is valid.
func (o OrderSourceType) IsValid() bool {
	switch o {
	case OrderSourceForSale, OrderSourceNegotiation, OrderSourceAuction:
		return true
	default:
		return false
	}
}

// String returns the string representation of the order source type.
func (o OrderSourceType) String() string {
	return string(o)
}


