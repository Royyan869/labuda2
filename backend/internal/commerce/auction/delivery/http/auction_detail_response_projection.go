package http

import (
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/auction/entity"
	productEntity "github.com/labuda/backend/internal/commerce/product/entity"
	commerceshared "github.com/labuda/backend/internal/commerce/shared"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/internal/pkg/sellerdisplay"
)

// auctionToDetailResponseWithSeller renders the canonical detail wire:
// the shared auction serializer (auctionToResponseWithSeller — which owns
// the Product content block for BOTH list and detail) PLUS the viewer-scoped
// capability block. The capability block is intentionally detail-only; the
// content block is NOT detail-only — list payloads carry the identical
// Product projection (for_sale parity).
func auctionToDetailResponseWithSeller(
	a *entity.Auction,
	seller publiccard.SellerCard,
	sellerInfo sellerdisplay.Info,
	product *productEntity.Product,
	viewerID *uuid.UUID,
) map[string]interface{} {
	resp := auctionToResponseWithSeller(a, product, sellerInfo, viewerID)
	sellerTrustActive := seller.Lifecycle != nil && *seller.Lifecycle == "active"
	resp["viewer_capabilities"] = commerceshared.EvaluateAuctionViewerCapabilities(
		commerceshared.AuctionViewerCapabilitiesInput{
			ViewerID:          uuidValue(viewerID),
			SellerID:          a.SellerID,
			Status:            string(a.Status),
			SellerTrustActive: sellerTrustActive,
			BuyNowPrice:       a.BuyNowPrice,
		},
	)
	return resp
}

func uuidStrings(values []uuid.UUID) []string {
	if len(values) == 0 {
		return []string{}
	}

	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == uuid.Nil {
			continue
		}
		result = append(result, value.String())
	}
	return result
}

func uuidValue(value *uuid.UUID) uuid.UUID {
	if value == nil {
		return uuid.Nil
	}
	return *value
}
