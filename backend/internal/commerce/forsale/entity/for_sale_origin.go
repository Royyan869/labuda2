package entity

// ForSaleOrigin represents the source/context of how a for_sale was created.
// This is for tracking and analytics only - NOT for business logic branching.
type ForSaleOrigin string

const (
	// ForSaleOriginDirectCreate - Seller created for_sale directly from seller dashboard
	ForSaleOriginDirectCreate ForSaleOrigin = "direct_create"

	// ForSaleOriginRequestContext - Seller created for_sale in response to a buyer request
	ForSaleOriginRequestContext ForSaleOrigin = "request_context"

	// ForSaleOriginChatContext - Seller created for_sale during/after a chat conversation
	ForSaleOriginChatContext ForSaleOrigin = "chat_context"
)

// String returns the string representation of the origin.
func (o ForSaleOrigin) String() string {
	return string(o)
}

// IsValid returns true if the origin is valid.
func (o ForSaleOrigin) IsValid() bool {
	switch o {
	case ForSaleOriginDirectCreate, ForSaleOriginRequestContext, ForSaleOriginChatContext:
		return true
	default:
		return false
	}
}




