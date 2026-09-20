package http

import (
	"strings"
	"time"

	"github.com/google/uuid"
	auctionentity "github.com/labuda/backend/internal/commerce/auction/entity"
	"github.com/labuda/backend/internal/discovery/search/entity"
	"github.com/labuda/backend/internal/pkg/mediaref"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/internal/platform/mediaresolve"
	contentApp "github.com/labuda/backend/internal/social/content/application"
)

// searchProjectionAdapter centralizes the current search wire assembly.
// The legacy wrapper functions in search_handler.go delegate to this adapter
// so the handler surface stays stable while the projection code becomes easier
// to audit.
//
// The ONLY intentional wire change here is Content media read resolution:
// /search/content rows carry persisted `content_media.media_url` references,
// and every media surface on the row is now projected through the shared
// mediaresolve authority (see resolveSearchContentMediaReadURLs). Commerce
// (for-sale / auction) rows keep their existing assembly untouched.
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

func (searchProjectionAdapter) contentPreviewsToResponseWithProjections(
	contents []*entity.ContentPreview,
	lifecycleOverrides map[uuid.UUID]string,
	authorLifecycleByID map[uuid.UUID]string,
	projections map[uuid.UUID]*contentApp.ContentResourceProjection,
) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(contents))
	for _, c := range contents {
		// MEDIA READ RESOLUTION: SearchContent loads the persisted
		// content_media.media_url values. They are projected through the shared
		// mediaresolve authority — the same authority the converged Content /
		// feed read surfaces use — and that ONE resolved list feeds all three
		// media surfaces on the row (flat `media_urls`, canonical `media` refs,
		// `card.media`), so they can never disagree.
		//
		// Search-specific semantics preserved verbatim: the DISTINCT +
		// ORDER BY media_url ordering produced by SearchContent is untouched,
		// and an unresolvable/empty reference is emitted as-is rather than
		// dropped or erased (fail-open, matching the converged surfaces).
		mediaURLs := resolveSearchContentMediaReadURLs(c.MediaURLs)
		media := buildMediaRefs(mediaURLs)

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
			mediaURLs,
			lifecycle,
			c.CreatedAt,
			&authorCard,
		)

		item := map[string]interface{}{
			"id":         c.ID.String(),
			"author_id":  c.AuthorID.String(),
			"type":       c.Type,
			"caption":    c.Caption,
			"media_urls": mediaURLs,
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

// resolveSearchContentMediaReadURLs projects persisted Content media
// references (content_media.media_url, as loaded by SearchContent) onto the
// canonical readable URL using the shared mediaresolve authority.
//
// This is the Search Content read surface's adoption of the already-locked
// Content media authority — no new resolver, URL builder, or media-type
// detector is introduced here.
//
// Fail-open (identical posture to the converged Content / feed surfaces): an
// empty reference stays empty and an unresolvable reference is emitted trimmed
// and unchanged, so a persisted reference is never erased and the emitted array
// keeps the exact length/order SearchContent produced.
func resolveSearchContentMediaReadURLs(urls []string) []string {
	if len(urls) == 0 {
		return urls
	}
	resolved := make([]string, 0, len(urls))
	for _, raw := range urls {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			resolved = append(resolved, "")
			continue
		}
		value := trimmed
		if readURL, err := mediaresolve.ResolveMediaReadURL(trimmed); err == nil {
			value = readURL
		}
		resolved = append(resolved, value)
	}
	return resolved
}
