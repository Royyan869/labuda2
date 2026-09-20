package entity_test

import (
	"testing"

	"github.com/labuda/backend/internal/commerce/order/entity"
)

// The auction settlement type is a creation-time validation input: the pricing
// token and the order-creation path reject anything that is not buy_now or
// bid_win. It is deliberately NOT persisted on the order (see
// NewOrderFromSource), so this type carries no order-lifecycle behavior.
func TestAuctionSettlementType_IsValid(t *testing.T) {
	if !entity.AuctionSettlementBuyNow.IsValid() {
		t.Fatal("buy_now must be valid")
	}
	if !entity.AuctionSettlementBidWin.IsValid() {
		t.Fatal("bid_win must be valid")
	}
	if entity.AuctionSettlementType("unknown").IsValid() {
		t.Fatal("unknown value must not be valid")
	}
}
