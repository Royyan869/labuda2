package http

import (
	"strings"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/auction/entity"
	productEntity "github.com/labuda/backend/internal/commerce/product/entity"
	commerceshared "github.com/labuda/backend/internal/commerce/shared"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/internal/pkg/sellerdisplay"
	"github.com/labuda/backend/internal/platform/mediaresolve"
)

func auctionToDetailResponseWithSeller(
	a *entity.Auction,
	seller publiccard.SellerCard,
	sellerInfo sellerdisplay.Info,
	mediaURLs []string,
	product *productEntity.Product,
	viewerID *uuid.UUID,
) map[string]interface{} {
	resp := auctionToResponseWithSeller(a, product, sellerInfo)
	resp["seller_identity"] = sellerdisplay.ProjectionMap(
		sellerInfo,
		resolveReadableAuctionMediaReference,
	)
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
	if product != nil {
		resp["title"] = product.Title
		resp["description"] = product.Description
		resp["media_urls"] = product.MediaURLs
		resp["variety"] = product.Variety
		resp["size_cm"] = product.SizeCm
		resp["age_months"] = product.AgeMonths
		resp["gender"] = product.Gender
		resp["breeder"] = product.Breeder
		resp["bloodline"] = product.Bloodline
		resp["certificates"] = product.Certificates
		resp["preparation_time"] = product.PreparationTime
		resp["preparation_note"] = product.PreparationNote
	}
	return resp
}

func resolveReadableAuctionMediaReference(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	resolved, err := mediaresolve.ResolveMediaReadURL(trimmed)
	if err != nil {
		return trimmed
	}
	return resolved
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
