package http

import (
	"time"

	"github.com/google/uuid"
	auctionentity "github.com/labuda/backend/internal/commerce/auction/entity"
	"github.com/labuda/backend/internal/discovery/search/entity"
	"github.com/labuda/backend/internal/pkg/mediaref"
	"github.com/labuda/backend/internal/pkg/publiccard"
	contentApp "github.com/labuda/backend/internal/social/content/application"
)

// searchProjectionAdapter centralizes the current search wire assembly
// without changing emitted JSON. The legacy wrapper functions in
// search_handler.go delegate to this adapter so the handler surface stays
// stable while the projection code becomes easier to audit.
type searchProjectionAdapter struct{}

func newSearchProjectionAdapter() searchProjectionAdapter {
	return searchProjectionAdapter{}
}

func (searchProjectionAdapter) forSalePreviewsToResponse(
	forSales []*entity.ForSalePreview,
	sellerUserLifecycleByID map[uuid.UUID]string,
) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(forSales))
	for _, l := range forSales {
		projection, ok := buildSearchCommerceSellerProjection(
			l.SellerID,
			l.SellerUsername,
			l.SellerAvatarURL,
			l.SellerFarmName,
			l.SellerAccountStatus,
			l.SellerIsDeleted,
			l.SellerSubscriptionStatus,
		)
		if !ok {
			continue
		}

		media := buildMediaRefs(l.MediaURLs)

		var thumbnail *string
		if len(l.MediaURLs) > 0 {
			t := l.MediaURLs[0]
			thumbnail = &t
		}

		seller := projection.Seller
		result = append(result, map[string]interface{}{
			"id":                l.ID.String(),
			"title":             l.Title,
			"description":       l.Description,
			"variety":           l.Variety,
			"price":             l.Price,
			"media_urls":        l.MediaURLs,
			"seller_id":         l.SellerID.String(),
			"created_at":        l.CreatedAt.Format(time.RFC3339),
			"seller_username":   projection.Author.Username,
			"seller_farm_name":  projection.FarmName,
			"seller_avatar_url": projection.Author.AvatarURL,
			"seller_lifecycle":   seller.Lifecycle,
			"author":            projection.Author,
			"media":             media,
			"for_sale": publiccard.NewForSaleCard(
				l.ID,
				l.Title,
				thumbnail,
				l.Price,
				nil,
				"",
				&seller,
			),
		})
	}
	return result
}

func (searchProjectionAdapter) contentPreviewsToResponse(
	contents []*entity.ContentPreview,
	lifecycleOverrides map[uuid.UUID]string,
	authorLifecycleByID map[uuid.UUID]string,
) []map[string]interface{} {
	return searchProjectionAdapter{}.contentPreviewsToResponseWithProjections(contents, lifecycleOverrides, authorLifecycleByID, nil)
}

func (searchProjectionAdapter) contentPreviewsToResponseWithProjections(
	contents []*entity.ContentPreview,
	lifecycleOverrides map[uuid.UUID]string,
	authorLifecycleByID map[uuid.UUID]string,
	projections map[uuid.UUID]*contentApp.ContentResourceProjection,
) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(contents))
	for _, c := range contents {
		media := buildMediaRefs(c.MediaURLs)

		var authorLifecycle string
		if authorLifecycleByID != nil {
			authorLifecycle = authorLifecycleByID[c.AuthorID]
		}
		authorCard := publiccard.NewWithLifecycle(
			c.AuthorID,
			c.AuthorUsername,
			c.AuthorAvatarURL,
			authorLifecycle,
		)

		var contentCaptionPtr *string
		if c.Caption != "" {
			cap := c.Caption
			contentCaptionPtr = &cap
		}

		lifecycle := ""
		if lifecycleOverrides != nil {
			if v, ok := lifecycleOverrides[c.ID]; ok {
				lifecycle = v
			}
		}

		contentCard := publiccard.NewContentCard(
			c.ID,
			c.Type,
			contentCaptionPtr,
			c.MediaURLs,
			lifecycle,
			c.CreatedAt,
			&authorCard,
		)

		item := map[string]interface{}{
			"id":         c.ID.String(),
			"author_id":  c.AuthorID.String(),
			"type":       c.Type,
			"caption":    c.Caption,
			"media_urls": c.MediaURLs,
			"created_at": c.CreatedAt.Format(time.RFC3339),
			"author":     authorCard,
			"media":      media,
			"card":       contentCard,
		}

		if projections != nil {
			if projection := projections[c.ID]; projection != nil {
				item["resource_projection"] = projection
			}
		}

		result = append(result, item)
	}
	return result
}

func (searchProjectionAdapter) userPreviewsToResponse(users []*entity.UserPreview) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(users))
	for _, u := range users {
		result = append(result, map[string]interface{}{
			"id":                          u.ID.String(),
			"username":                    u.Username,
			"avatar_url":                  u.AvatarURL,
			"is_followed_by_current_user": u.IsFollowedByCurrentUser,
		})
	}
	return result
}

func (searchProjectionAdapter) auctionPreviewsToResponse(
	auctions []*entity.AuctionPreview,
	sellerUserLifecycleByID map[uuid.UUID]string,
) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(auctions))
	thumbnailKind := "thumbnail"
	for _, a := range auctions {
		projection, ok := buildSearchCommerceSellerProjection(
			a.SellerID,
			a.SellerUsername,
			a.SellerAvatarURL,
			a.SellerFarmName,
			a.SellerAccountStatus,
			a.SellerIsDeleted,
			a.SellerSubscriptionStatus,
		)
		if !ok {
			continue
		}

		media := make([]mediaref.MediaRef, 0, 1)
		if a.ThumbnailURL != nil && *a.ThumbnailURL != "" {
			localKind := thumbnailKind
			media = append(media, mediaref.MediaRef{
				URL:  *a.ThumbnailURL,
				Kind: &localKind,
			})
		}

		auctionLifecycle := auctionentity.Status(a.Status).PublicLifecycle()
		seller := projection.Seller

		result = append(result, map[string]interface{}{
			"id":                a.ID.String(),
			"seller_id":         a.SellerID.String(),
			"product_id":        a.ProductID.String(),
			"title":             a.Title,
			"description":       a.Description,
			"start_price":       a.StartPrice,
			"current_bid":       a.CurrentBid,
			"buy_now_price":     a.BuyNowPrice,
			"start_at":          a.StartAt.Format(time.RFC3339),
			"end_at":            a.EndAt.Format(time.RFC3339),
			"status":            a.Status,
			"thumbnail_url":     a.ThumbnailURL,
			"bid_count":         a.BidCount,
			"created_at":        a.CreatedAt.Format(time.RFC3339),
			"seller_username":   projection.Author.Username,
			"seller_farm_name":  projection.FarmName,
			"seller_avatar_url": projection.Author.AvatarURL,
			"seller_lifecycle":   seller.Lifecycle,
			"author":            projection.Author,
			"media":             media,
			"auction": publiccard.NewAuctionCard(
				a.ID,
				a.Title,
				a.ThumbnailURL,
				a.CurrentBid,
				a.BuyNowPrice,
				a.EndAt.Format(time.RFC3339),
				auctionLifecycle,
				&seller,
			),
		})
	}
	return result
}

func buildMediaRefs(urls []string) []mediaref.MediaRef {
	media := make([]mediaref.MediaRef, 0, len(urls))
	for _, u := range urls {
		media = append(media, mediaref.MediaRef{URL: u})
	}
	return media
}
