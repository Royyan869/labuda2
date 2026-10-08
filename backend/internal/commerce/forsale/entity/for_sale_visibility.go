package entity

import "time"

// ForSaleVisibility represents the visibility of a for_sale.
//
// ═══════════════════════════════════════════════════════════════════════════════
// HARD RULE: STATUS → VISIBILITY MAPPING
// ═══════════════════════════════════════════════════════════════════════════════
// - Active for_sales: MUST BE public (ACTIVE = PUBLIC ONLY invariant)
// - Terminal states (sold/withdrawn): visibility field is irrelevant
//
// There is no draft state (create = publish), so there is no private
// workspace for_sale either: NewForSaleSurface sets Visibility=Public
// unconditionally. The private value survives only as defensive read
// vocabulary for unknown/unpublished rows.
type ForSaleVisibility string

const (
	// ForSaleVisibilityPublic is visible to all users.
	// This is the ONLY valid visibility for active for_sales.
	ForSaleVisibilityPublic ForSaleVisibility = "public"
	// ForSaleVisibilityPrivate is only visible to the seller.
	// Never set for a created for_sale (there is no private active state).
	ForSaleVisibilityPrivate ForSaleVisibility = "private"
)

// IsValid checks if the visibility is valid.
func (v ForSaleVisibility) IsValid() bool {
	switch v {
	case ForSaleVisibilityPublic, ForSaleVisibilityPrivate:
		return true
	default:
		return false
	}
}

// String returns the string representation of the visibility.
func (v ForSaleVisibility) String() string {
	return string(v)
}

// DeriveVisibility returns the canonical for_sale visibility from status and
// publish timestamp. A for_sale is published at create, so every known status
// derives public; only an unknown/unpublished row can derive private.
func DeriveVisibility(status ForSaleStatus, publishedAt *time.Time) ForSaleVisibility {
	if publishedAt != nil {
		switch status {
		case ForSaleStatusActive, ForSaleStatusSold, ForSaleStatusWithdrawn:
			return ForSaleVisibilityPublic
		}
	}
	return ForSaleVisibilityPrivate
}
