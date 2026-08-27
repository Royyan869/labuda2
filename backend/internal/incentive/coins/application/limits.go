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
