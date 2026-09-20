package entity

// AuctionSettlementType represents how an auction order was settled.
//
// BUSINESS RULE (Owner canonical 2026-06-16):
// - buy_now: Fixed-price checkout path, eligible for promo discounts and coins
// - bid_win: Competitive bidding path, eligible for promo discounts AND coins
//
// Coins are Labuda platform usage rights, not money. Both settlement types
// go through the same backend pricing authority (20% cap, commission safety,
// balance check) so coins are permitted on bid-win claims.
type AuctionSettlementType string

const (
	// AuctionSettlementBuyNow indicates the auction was settled via Buy Now option.
	// Treated as fixed-price checkout - promo discounts and coins are ALLOWED.
	AuctionSettlementBuyNow AuctionSettlementType = "buy_now"

	// AuctionSettlementBidWin indicates the auction was settled via winning bid/claim.
	// Treated as competitive final price - promo discounts and coins are ALLOWED.
	AuctionSettlementBidWin AuctionSettlementType = "bid_win"
)

// IsValid checks if the auction settlement type is valid.
func (a AuctionSettlementType) IsValid() bool {
	switch a {
	case AuctionSettlementBuyNow, AuctionSettlementBidWin:
		return true
	default:
		return false
	}
}

// String returns the string representation of the auction settlement type.
func (a AuctionSettlementType) String() string {
	return string(a)
}



