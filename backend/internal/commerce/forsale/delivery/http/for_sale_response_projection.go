package http

import (
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/forsale/entity"
	negotiationEntity "github.com/labuda/backend/internal/commerce/negotiation/entity"
	commerceshared "github.com/labuda/backend/internal/commerce/shared"
	"github.com/labuda/backend/internal/pkg/sellerdisplay"
)

// forSaleToDetailResponseWithViewerCapabilities renders the canonical
// GET /api/v1/for-sale/:id detail response: the shared sale serializer PLUS
// the viewer-scoped capability block and the buyer-facing listing origin. The
// capability block is intentionally detail-only — list/search/write responses
// use for_saleToResponseWithSeller without it so generic cache/list contracts
// stay viewer-agnostic. `public_origin_line` (city, province of the sender
// address) is likewise detail-only: it belongs to the seller card on the
// detail surface, not to discovery cards.
func forSaleToDetailResponseWithViewerCapabilities(
	l *entity.ForSale,
	seller sellerdisplay.Info,
	publicOriginLine string,
	viewerID *uuid.UUID,
) map[string]interface{} {
	resp := for_saleToResponseWithSeller(l, seller, viewerID)
	resp["public_origin_line"] = publicOriginLine
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

// viewerNegotiationBinding returns the session id when [s] is a settleable
// DEAL for the viewer (accepted, unexpired, unsettled — entity CanSettle).
// Nil otherwise: a binding is never fabricated from a stale, expired or
// already-settled session.
func viewerNegotiationBinding(s *negotiationEntity.NegotiationSession) *uuid.UUID {
	if s == nil || !s.CanSettle() {
		return nil
	}
	id := s.ID
	return &id
}

// Media wire rendering is CONVERGED into commerceshared.MediaWireItems
// (one wire shape for both Product surfaces). See commerce/shared/media_wire.go.
