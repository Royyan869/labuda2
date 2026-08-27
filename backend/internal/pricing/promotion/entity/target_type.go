package entity

import "fmt"

// TargetType represents the type of object being promoted.
// Only these three types are supported for promotion.
type TargetType string

const (
	// TargetTypeForSale promotes a fixed-price sale.
	TargetTypeForSale TargetType = "for_sale"

	// TargetTypeAuction promotes an auction.
	TargetTypeAuction TargetType = "auction"

	// TargetTypeExternalProduct promotes an external product (URL).
	TargetTypeExternalProduct TargetType = "external_product"
)

// IsValid returns true if the target type is valid.
func (t TargetType) IsValid() bool {
	switch t {
	case TargetTypeForSale, TargetTypeAuction, TargetTypeExternalProduct:
		return true
	default:
		return false
	}
}

// RequiresTargetID returns true if this target type needs a target_id reference.
// External products now point at external_products.id so they can be hydrated
// from the reviewable product/media model.
func (t TargetType) RequiresTargetID() bool {
	return t == TargetTypeForSale || t == TargetTypeAuction || t == TargetTypeExternalProduct
}

// IsPublicPromotable returns true when this target type can be shown on public discovery surfaces.
func (t TargetType) IsPublicPromotable() bool {
	return t == TargetTypeForSale || t == TargetTypeAuction || t == TargetTypeExternalProduct
}

// String returns the string representation of the target type.
func (t TargetType) String() string {
	return string(t)
}

// InvalidTargetTypeError is returned when an invalid target type is provided.
type InvalidTargetTypeError struct {
	TargetType TargetType
}

func (e *InvalidTargetTypeError) Error() string {
	return fmt.Sprintf("invalid target type: %s", e.TargetType)
}
