package http

import (
	"testing"

	"github.com/google/uuid"
	commerceshared "github.com/labuda/backend/internal/commerce/shared"
	"github.com/labuda/backend/internal/pkg/sellerdisplay"
)

// CANONICAL TRUTH (owner-locked): for_sale and auction emit the SAME Product
// content key set on BOTH list and detail payloads
// (shared.ProductContentWireKeys). This test pins the for_sale side of that
// contract; auction_list_content_parity_test.go pins the auction side, so a
// channel that drops a content key breaks the contract instead of silently
// rendering an empty card.
func TestForSaleListResponse_CarriesCanonicalProductContentBlock(t *testing.T) {
	for_sale := testForSale(uuid.New())
	for_sale.Product.PreparationTime = "1_3_days"
	sellerInfo := sellerdisplay.Info{
		Username:           "seller_user",
		FarmName:           "Acme Farm",
		AccountStatus:      "active",
		IsDeleted:          false,
		SubscriptionStatus: "active",
		Tier:               "pro",
	}

	resp := for_saleToResponseWithSeller(for_sale, sellerInfo, nil)

	for _, key := range commerceshared.ProductContentWireKeys {
		if _, ok := resp[key]; !ok {
			t.Fatalf("for_sale LIST payload missing canonical Product content key %q", key)
		}
	}

	if _, ok := resp["farm_address_id"]; ok {
		t.Fatalf("LIST payload must not carry farm_address_id: %#v", resp["farm_address_id"])
	}
	if resp["preparation_time"] != "1_3_days" {
		t.Fatalf("preparation_time = %#v, want short", resp["preparation_time"])
	}

	// viewer_capabilities is DETAIL-ONLY by contract (both channels).
	if _, ok := resp["viewer_capabilities"]; ok {
		t.Fatalf("LIST payload must not carry viewer_capabilities: %#v", resp["viewer_capabilities"])
	}

	detail := forSaleToDetailResponseWithViewerCapabilities(for_sale, sellerInfo, "", nil)
	for _, key := range commerceshared.ProductContentWireKeys {
		if _, ok := detail[key]; !ok {
			t.Fatalf("for_sale DETAIL payload missing canonical Product content key %q", key)
		}
	}
	if _, ok := detail["viewer_capabilities"]; !ok {
		t.Fatalf("DETAIL payload must carry viewer_capabilities")
	}
}
