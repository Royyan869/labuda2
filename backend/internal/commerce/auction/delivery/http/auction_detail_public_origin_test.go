package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/auction/entity"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/internal/pkg/sellerdisplay"
)

// OWNER TRUTH (parity with the for_sale channel): the auction DETAIL payload
// carries the buyer-facing origin summary ("City, Province") resolved from the
// listing's sender address, so a buyer can see where the goods ship from.
//
// NEGATIVE CONTRACT: origin is DETAIL-ONLY — discovery payloads do not carry
// it (for_sale pins the same rule in for_sale_detail_public_origin_test.go).
func TestAuctionDetailPayload_CarriesBuyerFacingOriginOnly(t *testing.T) {
	now := time.Now().UTC()
	auction := &entity.Auction{
		ID:           uuid.New(),
		SellerID:     uuid.New(),
		ProductID:    uuid.New(),
		StartPrice:   50000,
		BidIncrement: 5000,
		StartAt:      now,
		EndAt:        now.Add(24 * time.Hour),
		Status:       entity.StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	sellerCard := publiccard.SellerCard{
		User: publiccard.UserCard{ID: auction.SellerID, Username: "seller_user"},
	}
	sellerInfo := sellerdisplay.Info{
		Username:           "seller_user",
		FarmName:           "Acme Farm",
		AccountStatus:      "active",
		SubscriptionStatus: "active",
	}

	const origin = "Magelang, Jawa Tengah"

	detail := auctionToDetailResponseWithSeller(auction, sellerCard, sellerInfo, nil, origin, nil)
	if detail["public_origin_line"] != origin {
		t.Fatalf("detail public_origin_line = %#v, want %q", detail["public_origin_line"], origin)
	}

	list := auctionToResponseWithSeller(auction, nil, sellerInfo, nil)
	if _, ok := list["public_origin_line"]; ok {
		t.Fatalf("LIST payload must not carry public_origin_line: %#v", list["public_origin_line"])
	}
}
