package http

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/forsale/entity"
	mediaentity "github.com/labuda/backend/internal/commerce/media/entity"
	commerceshared "github.com/labuda/backend/internal/commerce/shared"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/internal/pkg/sellerdisplay"
	"github.com/labuda/backend/internal/platform/mediaresolve"
)

// for_saleToResponseWithSellerProjection renders fixed-price sale JSON using
// the fixed-price-local seller identity projection. It preserves the legacy
// flat seller fields for compatibility, but the nested for_sale block
// is now assembled from the canonical non-synthetic seller projection.
func for_saleToResponseWithSellerProjection(
	l *entity.ForSale,
	seller sellerdisplay.Info,
) map[string]interface{} {
	product := l.Product
	title := l.Title
	description := l.Description
	variety := l.Variety
	sizeCM := l.SizeCM
	ageMonths := l.AgeMonths
	gender := l.Gender
	breeder := l.Breeder
	bloodline := l.Bloodline
	certificates := l.Certificates
	preparationTime := l.PreparationTime
	preparationNote := l.PreparationNote
	if product != nil {
		title = product.Title
		description = product.Description
		variety = product.Variety
		sizeCM = product.SizeCm
		ageMonths = product.AgeMonths
		gender = product.Gender
		breeder = product.Breeder
		bloodline = product.Bloodline
		certificates = product.Certificates
		preparationTime = entity.PreparationTime(product.PreparationTime)
		preparationNote = product.PreparationNote
	}

	mediaItems := for_saleResponseMediaItems(l)
	mediaURLs := mediaentity.FlattenURLs(mediaItems)
	if len(mediaURLs) == 0 {
		mediaURLs = []string{}
	}
	for i, raw := range mediaURLs {
		mediaURLs[i] = resolveReadableForSaleMediaReference(raw)
	}
	renderedMedia := make([]map[string]interface{}, 0, len(mediaItems))
	for _, item := range mediaItems {
		renderedMedia = append(renderedMedia, renderForSaleMediaWire(item))
	}

	var thumbnail *string
	if len(mediaURLs) > 0 {
		t := mediaURLs[0]
		thumbnail = &t
	}

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

	var sellerCard *publiccard.SellerCard
	var sellerUsername string
	var sellerFarmName string
	var sellerAvatarURL string
	if ok {
		sellerCard = &sellerProjection.Seller
		sellerUsername = sellerProjection.Author.Username
		sellerFarmName = sellerProjection.FarmName
		if sellerProjection.Author.AvatarURL != nil {
			sellerAvatarURL = resolveReadableForSaleMediaReference(*sellerProjection.Author.AvatarURL)
		}
	}

	forSaleCard := publiccard.NewForSaleCard(
		l.ID,
		title,
		thumbnail,
		l.PricePerUnit.Int64(),
		nil,
		l.Status.PublicLifecycle(),
		sellerCard,
	)

	return map[string]interface{}{
		"id":                  l.ID.String(),
		"product_id":          l.ProductID.String(),
		"seller_id":           l.SellerID.String(),
		"title":               title,
		"description":         description,
		"media":               renderedMedia,
		"media_urls":          mediaURLs,
		"variety":             variety,
		"size_cm":             sizeCM,
		"age_months":          ageMonths,
		"gender":              gender,
		"breeder":             breeder,
		"bloodline":           bloodline,
		"certificates":        certificates,
		"price":               l.PricePerUnit.Int64(),
		"quantity":            l.QuantityAvailable,
		"negotiation_enabled": l.NegotiationEnabled,
		"visibility":          string(l.Visibility),
		"status":              string(l.Status),
		"lifecycle":           l.Status.PublicLifecycle(),
		"preparation_time":    preparationTime,
		"preparation_note":    preparationNote,
		"published_at":        l.PublishedAt,
		"sold_at":             l.SoldAt,
		"withdrawn_at":        l.WithdrawnAt,
		"created_at":          l.CreatedAt.Format(time.RFC3339),
		"updated_at":          l.UpdatedAt.Format(time.RFC3339),
		"seller_username":     sellerUsername,
		"seller_farm_name":    sellerFarmName,
		"seller_avatar_url":   sellerAvatarURL,
		"for_sale":            forSaleCard,
	}
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

func for_saleToDetailResponseWithSellerProjection(
	l *entity.ForSale,
	seller sellerdisplay.Info,
	viewerID *uuid.UUID,
) map[string]interface{} {
	resp := for_saleToResponseWithSellerProjection(l, seller)
	resp["seller_identity"] = sellerdisplay.ProjectionMap(
		seller,
		resolveReadableForSaleMediaReference,
	)
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
		items, err := mediaentity.NewLegacyImageListFromReferences(l.Product.MediaURLs, l.CreatedAt)
		if err == nil {
			return items
		}
	}
	if len(l.MediaURLs) > 0 && string(l.MediaURLs) != "null" {
		var mediaURLs []string
		if err := json.Unmarshal(l.MediaURLs, &mediaURLs); err == nil {
			items, err := mediaentity.NewLegacyImageListFromReferences(mediaURLs, l.CreatedAt)
			if err == nil {
				return items
			}
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
