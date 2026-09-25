package http

import (
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/forsale/entity"
	commerceshared "github.com/labuda/backend/internal/commerce/shared"
	"github.com/labuda/backend/internal/pkg/sellerdisplay"
)

// forSaleToDetailResponseWithViewerCapabilities renders the canonical
// GET /api/v1/for-sale/:id detail response: the shared sale serializer PLUS
// the viewer-scoped capability block. The capability block is intentionally
// detail-only — list/search/write responses use for_saleToResponseWithSeller
// without it so generic cache/list contracts stay viewer-agnostic.
func forSaleToDetailResponseWithViewerCapabilities(
	l *entity.ForSale,
	seller sellerdisplay.Info,
	viewerID *uuid.UUID,
) map[string]interface{} {
	resp := for_saleToResponseWithSeller(l, seller)
	resp["viewer_capabilities"] = buildForSaleViewerCapabilities(l, seller, viewerID)
	return resp
}

func buildForSaleViewerCapabilities(
	l *entity.ForSale,
	seller sellerdisplay.Info,
	viewerID *uuid.UUID,
) commerceshared.ViewerCapabilities {
	sellerProjection, ok := buildForSaleSellerProjection(
		l.SellerID,
		seller.Username,
		seller.AvatarURL,
		seller.FarmName,
		seller.AccountStatus,
		seller.IsDeleted,
		seller.SubscriptionStatus,
		seller.Tier,
	)
	sellerTrustActive := ok &&
		sellerProjection.Seller.Lifecycle != nil &&
		*sellerProjection.Seller.Lifecycle == "active"

	viewerUUID := uuid.Nil
	if viewerID != nil {
		viewerUUID = *viewerID
	}

	return commerceshared.EvaluateForSaleViewerCapabilities(commerceshared.ForSaleViewerCapabilitiesInput{
		ViewerID:           viewerUUID,
		SellerID:           l.SellerID,
		ProductID:          l.ProductID,
		Status:             string(l.Status),
		QuantityAvailable:  l.QuantityAvailable,
		NegotiationEnabled: l.NegotiationEnabled,
		SellerTrustActive:  sellerTrustActive,
	})
}

// Media wire rendering is CONVERGED into commerceshared.MediaWireItems
// (one wire shape for both Product surfaces). See commerce/shared/media_wire.go.
