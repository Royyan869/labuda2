package entity

import "time"

// ForSaleVisibility represents the visibility of a for_sale.
//
// ═══════════════════════════════════════════════════════════════════════════════
// HARD RULE: STATUS → VISIBILITY MAPPING
// ═══════════════════════════════════════════════════════════════════════════════
// - Draft for_sales: MUST BE private (workspace-only)
// - Active for_sales: MUST BE public (ACTIVE = PUBLIC ONLY invariant)
// - Terminal states (sold/withdrawn): visibility field is irrelevant
//
// The Publish() method enforces this by automatically setting Visibility=Public.
// No manual setting of visibility for active for_sales is allowed.
type ForSaleVisibility string

const (
	// ForSaleVisibilityPublic is visible to all users.
	// This is the ONLY valid visibility for active for_sales.
	ForSaleVisibilityPublic ForSaleVisibility = "public"
	// ForSaleVisibilityPrivate is only visible to the seller.
	// This is the ONLY valid visibility for draft for_sales.
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
// publish timestamp. Active for_sales become public only once published; every
// other state remains private.
func DeriveVisibility(status ForSaleStatus, publishedAt *time.Time) ForSaleVisibility {
	if status == ForSaleStatusActive && publishedAt != nil {
		return ForSaleVisibilityPublic
	}
	return ForSaleVisibilityPrivate
}
