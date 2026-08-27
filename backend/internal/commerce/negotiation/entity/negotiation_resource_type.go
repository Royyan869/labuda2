package entity

// NegotiationResourceType represents the type of resource being negotiated.
type NegotiationResourceType string

const (
	// NegotiationResourceForSale - Fixed-price-sale-based negotiations
	NegotiationResourceForSale NegotiationResourceType = "for_sale"
)

// String returns the string representation of the resource type.
func (t NegotiationResourceType) String() string {
	return string(t)
}

// IsValid returns true if the resource type is valid.
func (t NegotiationResourceType) IsValid() bool {
	return t == NegotiationResourceForSale
}


