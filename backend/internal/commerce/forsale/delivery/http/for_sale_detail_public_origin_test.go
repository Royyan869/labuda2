package http

import (
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pkg/sellerdisplay"
)

// OWNER TRUTH: a buyer must be able to see where the goods ship from, so the
// for_sale DETAIL payload carries the buyer-facing origin summary
// ("City, Province") resolved from the listing's sender address.
//
// NEGATIVE CONTRACT: the origin is DETAIL-ONLY. Discovery/list payloads render
// cards, not a seller block, so emitting the key there would create a second
// transport for the same fact. Auction pins the identical contract in
// auction_detail_public_origin_test.go.
func TestForSaleDetailPayload_CarriesBuyerFacingOriginOnly(t *testing.T) {
	for_sale := testForSale(uuid.New())
	sellerInfo := sellerdisplay.Info{
		Username:           "seller_user",
		FarmName:           "Acme Farm",
		AccountStatus:      "active",
		SubscriptionStatus: "active",
	}

	const origin = "Magelang, Jawa Tengah"

	detail := forSaleToDetailResponseWithViewerCapabilities(for_sale, sellerInfo, origin, nil)
	if detail["public_origin_line"] != origin {
		t.Fatalf("detail public_origin_line = %#v, want %q", detail["public_origin_line"], origin)
	}

	list := for_saleToResponseWithSeller(for_sale, sellerInfo, nil)
	if _, ok := list["public_origin_line"]; ok {
		t.Fatalf("LIST payload must not carry public_origin_line: %#v", list["public_origin_line"])
	}
}
