package entity_test

import (
	"testing"

	"github.com/labuda/backend/internal/commerce/order/entity"
)

// Regression lock: owner canonical 2026-06-16 — both settlement types allow coins.
func TestAuctionSettlementType_AllowsCoins(t *testing.T) {
	if !entity.AuctionSettlementBuyNow.AllowsCoins() {
		t.Fatal("AuctionSettlementBuyNow must allow coins")
	}
	if !entity.AuctionSettlementBidWin.AllowsCoins() {
		t.Fatal("AuctionSettlementBidWin must allow coins (owner canonical 2026-06-16)")
	}
}

func TestAuctionSettlementType_AllowsDiscounts(t *testing.T) {
	if !entity.AuctionSettlementBuyNow.AllowsDiscounts() {
		t.Fatal("AuctionSettlementBuyNow must allow discounts")
	}
	if !entity.AuctionSettlementBidWin.AllowsDiscounts() {
		t.Fatal("AuctionSettlementBidWin must allow discounts")
	}
}

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


