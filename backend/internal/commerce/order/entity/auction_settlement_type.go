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

// IsBuyNow returns true if this is a buy_now settlement.
func (a AuctionSettlementType) IsBuyNow() bool {
	return a == AuctionSettlementBuyNow
}

// IsBidWin returns true if this is a bid_win settlement.
func (a AuctionSettlementType) IsBidWin() bool {
	return a == AuctionSettlementBidWin
}

// AllowsDiscounts returns true if this settlement type allows promotional discounts.
// Both buy_now and bid_win settlements allow discounts.
func (a AuctionSettlementType) AllowsDiscounts() bool {
	return a == AuctionSettlementBuyNow || a == AuctionSettlementBidWin
}

// AllowsCoins returns true if this settlement type allows coins discount.
// Both buy_now and bid_win settlements allow coins (owner canonical 2026-06-16).
func (a AuctionSettlementType) AllowsCoins() bool {
	return a == AuctionSettlementBuyNow || a == AuctionSettlementBidWin
}

// Ptr returns a pointer to this AuctionSettlementType.
func (a AuctionSettlementType) Ptr() *AuctionSettlementType {
	return &a
}

// AuctionSettlementBuyNowPtr returns a pointer to AuctionSettlementBuyNow.
func AuctionSettlementBuyNowPtr() *AuctionSettlementType {
	buyNow := AuctionSettlementBuyNow
	return &buyNow
}

// AuctionSettlementBidWinPtr returns a pointer to AuctionSettlementBidWin.
func AuctionSettlementBidWinPtr() *AuctionSettlementType {
	bidWin := AuctionSettlementBidWin
	return &bidWin
}


