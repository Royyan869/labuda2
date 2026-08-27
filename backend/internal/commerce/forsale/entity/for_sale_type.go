package entity

// ForSaleType represents the type of a fixed-price for_sale.
//
// PASS_21C: the "auction" value was removed. ForSale (for_sale) and
// Auction are canonical siblings under Product — a ForSale can never
// be an auction, so this type is intentionally single-valued. It is kept
// (rather than deleted outright) because the field is still threaded through
// CreateForSaleInput and many existing tests; removing the concept
// entirely is tracked as low-priority hygiene debt, not an architecture risk,
// since there is no longer any symbol capable of representing "for_sale type
// = auction".
type ForSaleType string

const (
	// ForSaleTypeFixedPrice is a standard fixed-price for_sale.
	ForSaleTypeFixedPrice ForSaleType = "fixed_price"
)

// IsValid checks if the for_sale type is valid.
func (t ForSaleType) IsValid() bool {
	switch t {
	case ForSaleTypeFixedPrice:
		return true
	default:
		return false
	}
}

// String returns the string representation of the for_sale type.
func (t ForSaleType) String() string {
	return string(t)
}
