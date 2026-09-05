package http

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/forsale/entity"
	mediaentity "github.com/labuda/backend/internal/commerce/media/entity"
	commerceshared "github.com/labuda/backend/internal/commerce/shared"
	"github.com/labuda/backend/internal/pkg/sellerdisplay"
	"github.com/labuda/backend/internal/platform/mediaresolve")

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

func resolveReadableForSaleMediaReference(value string) string {
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

func for_saleResponseMediaItems(l *entity.ForSale) []mediaentity.Media {
	if l == nil {
		return []mediaentity.Media{}
	}
	if l.Product != nil && len(l.Product.MediaURLs) > 0 {
		items, err := mediaentity.NewListFromReferences(l.Product.MediaURLs, l.CreatedAt)
		if err == nil {
			return items
		}
	}
	return []mediaentity.Media{}
}

func renderForSaleMediaWire(item mediaentity.Media) map[string]interface{} {
	return map[string]interface{}{
		"id":            item.ID.String(),
		"type":          item.Type.String(),
		"url":           resolveReadableForSaleMediaReference(item.URL),
		"position":      item.Position,
		"thumbnail_url": resolveReadableForSaleMediaReference(stringValue(item.ThumbnailURL)),
		"width":         item.Width,
		"height":        item.Height,
		"duration":      item.Duration,
		"created_at":    item.CreatedAt.Format(time.RFC3339),
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
