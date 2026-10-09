package main

import (
	"os"
	"strings"
	"testing"
)

// TestAuctionClaimRouteAbsent_PurgeMustHold guards the reconstruction of the
// auction bid-win purchase flow.
//
// The legacy POST /api/v1/auctions/:id/claim mixed auction settlement with
// buyer checkout and order creation, and landed winners on Payment Result
// instead of the canonical Order machine. Owner canonical:
//
//	Auction END → settlement window (end_at + 24h) → shared Checkout
//	→ POST /orders (bid-win branch: binds auction.OrderID + ResolveShipping
//	in the order-creation transaction) → Order Detail → PaymentMethodPicker
//	→ POST /payments.
//
// The claim endpoint has NO remaining business responsibility and must never
// be re-registered; bid-win order creation lives only in POST /orders.
func TestAuctionClaimRouteAbsent_PurgeMustHold(t *testing.T) {
	src, err := os.ReadFile("routes_core.go")
	if err != nil {
		t.Fatalf("read routes_core.go: %v", err)
	}

	code := string(src)

	if strings.Contains(code, `auctionRoutes.POST("/:id/claim"`) {
		t.Fatal("regression: auction claim route POST /auctions/:id/claim must not be registered — bid-win flows through POST /orders")
	}
	if strings.Contains(code, "ClaimAuction") {
		t.Fatal("regression: ClaimAuction handler must not be referenced in route registration")
	}
}
