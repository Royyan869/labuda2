package application

// MaxCoinsUsagePercentage is the canonical cap for coin usage on an order.
// The basis is the discounted product value PD = subtotal - discount.
const MaxCoinsUsagePercentage = 20

// MaxCoinsAllowedForDiscountedProduct returns floor(20% × PD).
// Shipping, commission, and payment fees are intentionally excluded.
func MaxCoinsAllowedForDiscountedProduct(discountedProduct int64) int64 {
	if discountedProduct <= 0 {
		return 0
	}
	return discountedProduct * MaxCoinsUsagePercentage / 100
}

// ResolveOrderRedemption is the SINGLE canonical order-time coin authority.
//
// It turns the buyer's checkout intent (use_coins) into the canonical K that
// gets persisted on pricing_tokens.coins_used at Order creation:
//   - useCoins == false          -> K = 0 (coins are optional)
//   - useCoins == true           -> K = min(availableBalance, maxCoinsAllowed)
//
// maxCoinsAllowed is the pricing token's pre-computed 20%-of-PD ceiling
// (PD = subtotal - seller-funded discount; shipping, commission and payment
// fees are excluded). K can never exceed the buyer's actual balance, so an
// order is never created with more coins than the buyer can fund.
//
// Payment MUST derive K from the persisted token snapshot and MUST NOT accept
// an independent client-supplied K.
func ResolveOrderRedemption(availableBalance, maxCoinsAllowed int64, useCoins bool) int64 {
	if !useCoins || maxCoinsAllowed <= 0 || availableBalance <= 0 {
		return 0
	}
	if availableBalance < maxCoinsAllowed {
		return availableBalance
	}
	return maxCoinsAllowed
}
