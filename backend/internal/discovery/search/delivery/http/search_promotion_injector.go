package http

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/governance/viewercontext"
	promotionApp "github.com/labuda/backend/internal/pricing/promotion/application"
	promoentity "github.com/labuda/backend/internal/pricing/promotion/entity"
	"go.uber.org/zap"
)

type promotionQueryPool interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// searchMaxPromotedPerPage is the maximum promoted items per search page.
const searchMaxPromotedPerPage = 1

// searchMinOrganicForInjection is the minimum organic results before injection fires.
const searchMinOrganicForInjection = 3

// searchInjectAtIndex is the 0-based organic position before which the promoted
// item is inserted on the client. Value 2 means the promoted card appears after
// the 2nd organic result.
const searchInjectAtIndex = 2

// SearchPromotionInjector builds a promoted items sidecar for search responses.
// Unlike the feed injector (which interleaves into the organic array), the search
// injector returns a separate promoted_items array with inject_at positions. This
// preserves organic pagination (total/offset/limit) accurately.
//
// FAIL-OPEN: Any error returns nil (empty sidecar). Organic results are never affected.
type SearchPromotionInjector struct {
	discoveryService *promotionApp.DiscoveryService
	db               promotionQueryPool
	log              *zap.Logger
}

// NewSearchPromotionInjector creates a new search injector.
func NewSearchPromotionInjector(
	discoveryService *promotionApp.DiscoveryService,
	database promotionQueryPool,
	log *zap.Logger,
) *SearchPromotionInjector {
	if log == nil {
		log = zap.NewNop()
	}
	return &SearchPromotionInjector{
		discoveryService: discoveryService,
		db:               database,
		log:              log,
	}
}

// searchHydratedPromotion is an intermediate struct holding hydrated card data.
type searchHydratedPromotion struct {
	Instance *promoentity.PromotionInstance
	SellerID uuid.UUID
	Response map[string]interface{}
}

// GetPromotedSidecar fetches active promotions, hydrates card data, applies
// dedup against organic results, and returns the sidecar array. Each element
// carries an "inject_at" field telling the mobile client where to insert.
//
// organicIDs: IDs of organic fixed-price-sale/auction results on this page (for target dedup).
// organicSellerIDs: seller IDs of organic results (for seller dedup).
//
// Returns nil when there is nothing to inject (no promotions, too few organic,
// errors, or all candidates filtered).
func (inj *SearchPromotionInjector) GetPromotedSidecar(
	ctx context.Context,
	organicIDs []uuid.UUID,
	organicSellerIDs []uuid.UUID,
) []map[string]interface{} {
	if inj == nil || inj.discoveryService == nil {
		return nil
	}

	if len(organicIDs) < searchMinOrganicForInjection {
		return nil
	}

	// Fetch more candidates than needed for filtering headroom.
	candidates, err := inj.discoveryService.GetPromotedItems(ctx, searchMaxPromotedPerPage*3)
	if err != nil {
		inj.log.Warn("search promotion: discovery fetch failed, fail-open",
			zap.Error(err))
		return nil
	}
	if len(candidates) == 0 {
		return nil
	}

	hydrated, err := inj.hydrateSearchPromotedItems(ctx, candidates)
	if err != nil {
		inj.log.Warn("search promotion: hydration failed, fail-open",
			zap.Error(err))
		return nil
	}
	if len(hydrated) == 0 {
		return nil
	}

	// Build organic lookup sets for dedup.
	organicIDSet := make(map[uuid.UUID]bool, len(organicIDs))
	for _, id := range organicIDs {
		organicIDSet[id] = true
	}
	organicSellerSet := make(map[uuid.UUID]bool, len(organicSellerIDs))
	for _, id := range organicSellerIDs {
		organicSellerSet[id] = true
	}

	filtered := searchApplySlotPolicy(hydrated, organicIDSet, organicSellerSet)
	if len(filtered) == 0 {
		return nil
	}

	// Build sidecar array.
	sidecar := make([]map[string]interface{}, 0, len(filtered))
	for _, item := range filtered {
		item.Response["inject_at"] = searchInjectAtIndex
		sidecar = append(sidecar, item.Response)
	}
	return sidecar
}

// ---------- Hydration ----------

func (inj *SearchPromotionInjector) hydrateSearchPromotedItems(
	ctx context.Context,
	instances []*promoentity.PromotionInstance,
) ([]searchHydratedPromotion, error) {
	var forSaleIDs, auctionIDs, externalProductIDs []uuid.UUID
	var sellerIDs []uuid.UUID
	for _, inst := range instances {
		if !inst.TargetType.IsPublicPromotable() {
			continue
		}
		if inst.TargetType == promoentity.TargetTypeForSale && inst.TargetID != nil {
			forSaleIDs = append(forSaleIDs, *inst.TargetID)
			sellerIDs = append(sellerIDs, inst.UserID)
		} else if inst.TargetType == promoentity.TargetTypeAuction && inst.TargetID != nil {
			auctionIDs = append(auctionIDs, *inst.TargetID)
			sellerIDs = append(sellerIDs, inst.UserID)
		} else if inst.TargetType == promoentity.TargetTypeExternalProduct && inst.TargetID != nil {
			externalProductIDs = append(externalProductIDs, *inst.TargetID)
			sellerIDs = append(sellerIDs, inst.UserID)
		}
	}

	forSaleCards := make(map[uuid.UUID]*searchForSaleCard)
	if len(forSaleIDs) > 0 {
		cards, err := inj.fetchSearchForSaleCards(ctx, forSaleIDs)
		if err != nil {
			return nil, err
		}
		forSaleCards = cards
	}

	auctionCards := make(map[uuid.UUID]*searchAuctionCard)
	if len(auctionIDs) > 0 {
		cards, err := inj.fetchSearchAuctionCards(ctx, auctionIDs)
		if err != nil {
			return nil, err
		}
		auctionCards = cards
	}

	externalProductCards := make(map[uuid.UUID]*searchExternalProductCard)
	if len(externalProductIDs) > 0 {
		cards, err := inj.fetchSearchExternalProductCards(ctx, externalProductIDs)
		if err != nil {
			return nil, err
		}
		externalProductCards = cards
	}

	sellerInfos := make(map[uuid.UUID]*searchSellerInfo)
	if len(sellerIDs) > 0 {
		infos, err := inj.fetchSearchSellerInfos(ctx, sellerIDs)
		if err != nil {
			return nil, err
		}
		sellerInfos = infos
	}

	var result []searchHydratedPromotion
	for _, inst := range instances {
		seller := sellerInfos[inst.UserID]
		sellerUsername := ""
		sellerFarmName := ""
		sellerLifecycle := "active"
		if seller != nil {
			sellerUsername = seller.Username
			sellerFarmName = seller.FarmName
			sellerLifecycle = string(seller.Lifecycle)
		}

		switch inst.TargetType {
		case promoentity.TargetTypeForSale:
			if inst.TargetID == nil {
				continue
			}
			card, ok := forSaleCards[*inst.TargetID]
			if !ok {
				continue
			}
			result = append(result, searchHydratedPromotion{
				Instance: inst,
				SellerID: inst.UserID,
				Response: searchBuildForSaleResponse(
					inst, card, sellerUsername, sellerFarmName, sellerLifecycle,
				),
			})

		case promoentity.TargetTypeAuction:
			if inst.TargetID == nil {
				continue
			}
			card, ok := auctionCards[*inst.TargetID]
			if !ok {
				continue
			}
			result = append(result, searchHydratedPromotion{
				Instance: inst,
				SellerID: inst.UserID,
				Response: searchBuildAuctionResponse(
					inst, card, sellerUsername, sellerFarmName, sellerLifecycle,
				),
			})

		case promoentity.TargetTypeExternalProduct:
			if inst.TargetID == nil {
				continue
			}
			card, ok := externalProductCards[*inst.TargetID]
			if !ok {
				continue
			}
			result = append(result, searchHydratedPromotion{
				Instance: inst,
				SellerID: inst.UserID,
				Response: searchBuildExternalResponse(
					inst, card, sellerUsername, sellerFarmName, sellerLifecycle,
				),
			})
		}
	}

	return result, nil
}

// ---------- Card data structs ----------

type searchForSaleCard struct {
	ID           uuid.UUID
	Title        string
	PricePerUnit int64
	ImageURL     string
}

type searchAuctionCard struct {
	ID          uuid.UUID
	Title       string
	StartPrice  int64
	CurrentBid  *int64
	BuyNowPrice *int64
	EndAt       time.Time
	BidCount    int
	Status      string
	ImageURL    string
}

type searchExternalProductCard struct {
	ID           uuid.UUID
	OwnerUserID  uuid.UUID
	Title        string
	Description  *string
	ExternalURL  string
	MediaURL     *string
	MediaType    *string
	ThumbnailURL *string
}

type searchSellerInfo struct {
	UserID    uuid.UUID
	Username  string
	FarmName  string
	Lifecycle viewercontext.PublicLifecycleState
}

// ---------- Batch fetch helpers ----------

func (inj *SearchPromotionInjector) fetchSearchForSaleCards(
	ctx context.Context,
	ids []uuid.UUID,
) (map[uuid.UUID]*searchForSaleCard, error) {
	// for_sales holds the sale surface; products holds title and media.
	query := `
		SELECT fps.id, p.title, fps.price_per_unit, p.media_urls
		FROM for_sales fps
		JOIN products p ON p.id = fps.product_id
		WHERE fps.id = ANY($1)
		  AND fps.status = 'active'
		  AND fps.quantity_available > 0
	`
	rows, err := inj.db.Query(ctx, query, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[uuid.UUID]*searchForSaleCard)
	for rows.Next() {
		var card searchForSaleCard
		var mediaURLsRaw json.RawMessage
		if err := rows.Scan(&card.ID, &card.Title, &card.PricePerUnit, &mediaURLsRaw); err != nil {
			continue
		}
		card.ImageURL = searchExtractFirstMediaURL(mediaURLsRaw)
		result[card.ID] = &card
	}
	return result, nil
}

func (inj *SearchPromotionInjector) fetchSearchAuctionCards(
	ctx context.Context,
	ids []uuid.UUID,
) (map[uuid.UUID]*searchAuctionCard, error) {
	// auctions.listing_id was a legacy column (never set for product-based
	// auctions) dropped entirely by migration 000010 (PASS_21C). Canonical
	// media + content source is products joined via auctions.product_id.
	query := `
		SELECT a.id, p.title, a.start_price, a.current_bid, a.buy_now_price,
		       a.end_at, a.status,
		       p.media_urls,
		       (SELECT COUNT(*) FROM auction_bids ab WHERE ab.auction_id = a.id)
		FROM auctions a
		LEFT JOIN products p ON p.id = a.product_id
		WHERE a.id = ANY($1)
		  AND a.status IN ('scheduled', 'active')
	`
	rows, err := inj.db.Query(ctx, query, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[uuid.UUID]*searchAuctionCard)
	for rows.Next() {
		var card searchAuctionCard
		var mediaURLsRaw json.RawMessage
		if err := rows.Scan(
			&card.ID, &card.Title, &card.StartPrice, &card.CurrentBid,
			&card.BuyNowPrice, &card.EndAt, &card.Status,
			&mediaURLsRaw, &card.BidCount,
		); err != nil {
			continue
		}
		card.ImageURL = searchExtractFirstMediaURL(mediaURLsRaw)
		result[card.ID] = &card
	}
	return result, nil
}

func (inj *SearchPromotionInjector) fetchSearchExternalProductCards(
	ctx context.Context,
	ids []uuid.UUID,
) (map[uuid.UUID]*searchExternalProductCard, error) {
	if inj == nil || inj.db == nil {
		return map[uuid.UUID]*searchExternalProductCard{}, nil
	}
	query := `
		SELECT p.id, p.owner_user_id, p.title, p.description,
		       p.normalized_external_url,
		       m.url, m.media_type, m.thumbnail_url
		FROM external_products p
		LEFT JOIN LATERAL (
			SELECT url, media_type, thumbnail_url
			FROM external_product_media
			WHERE external_product_id = p.id
			  AND deleted_at IS NULL
			  AND media_type IN ('image', 'video')
			ORDER BY sort_order ASC, created_at ASC
			LIMIT 1
		) m ON true
		WHERE p.id = ANY($1)
		  AND p.review_status = 'approved'
		  AND p.deleted_at IS NULL
	`
	rows, err := inj.db.Query(ctx, query, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[uuid.UUID]*searchExternalProductCard)
	for rows.Next() {
		var card searchExternalProductCard
		var mediaURL, mediaType, thumbnailURL *string
		if err := rows.Scan(
			&card.ID, &card.OwnerUserID, &card.Title, &card.Description,
			&card.ExternalURL, &mediaURL, &mediaType, &thumbnailURL,
		); err != nil {
			continue
		}
		card.MediaURL = mediaURL
		card.MediaType = mediaType
		card.ThumbnailURL = thumbnailURL
		if card.MediaURL == nil || *card.MediaURL == "" {
			continue
		}
		result[card.ID] = &card
	}
	return result, nil
}

func (inj *SearchPromotionInjector) fetchSearchSellerInfos(
	ctx context.Context,
	userIDs []uuid.UUID,
) (map[uuid.UUID]*searchSellerInfo, error) {
	query := `
		SELECT u.id,
		       COALESCE(up.username, '') AS seller_username,
		       COALESCE(sp.store_name, '') AS seller_farm_name,
		       u.account_status,
		       (u.deleted_at IS NOT NULL)
		FROM users u
		LEFT JOIN user_profiles up ON up.user_id = u.id
		LEFT JOIN seller_profiles sp ON sp.user_id = u.id
		WHERE u.id = ANY($1)
	`
	rows, err := inj.db.Query(ctx, query, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[uuid.UUID]*searchSellerInfo)
	for rows.Next() {
		var info searchSellerInfo
		var accountStatus string
		var isDeleted bool
		if err := rows.Scan(
			&info.UserID,
			&info.Username,
			&info.FarmName,
			&accountStatus,
			&isDeleted,
		); err != nil {
			continue
		}
		info.Lifecycle = viewercontext.CoarsenLifecycle(accountStatus, isDeleted)
		result[info.UserID] = &info
	}
	return result, nil
}

// searchExtractFirstMediaURL parses a JSONB array and returns the first URL.
func searchExtractFirstMediaURL(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var urls []string
	if err := json.Unmarshal(raw, &urls); err != nil {
		var items []struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(raw, &items); err != nil {
			return ""
		}
		if len(items) > 0 {
			return items[0].URL
		}
		return ""
	}
	if len(urls) > 0 {
		return urls[0]
	}
	return ""
}

// ---------- Response builders ----------

func searchBuildForSaleResponse(
	inst *promoentity.PromotionInstance,
	card *searchForSaleCard,
	sellerUsername, sellerFarmName, sellerLifecycle string,
) map[string]interface{} {
	return map[string]interface{}{
		"type":                  "promoted_for_sale",
		"promotion_instance_id": inst.ID.String(),
		"target_type":           "for_sale",
		"for_sale_id":   card.ID.String(),
		"title":                 card.Title,
		"price_per_unit":        card.PricePerUnit,
		"image_url":             card.ImageURL,
		"seller_username":       sellerUsername,
		"seller_farm_name":      sellerFarmName,
		"seller_lifecycle":      sellerLifecycle,
	}
}

func searchBuildAuctionResponse(
	inst *promoentity.PromotionInstance,
	card *searchAuctionCard,
	sellerUsername, sellerFarmName, sellerLifecycle string,
) map[string]interface{} {
	resp := map[string]interface{}{
		"type":                  "promoted_auction",
		"promotion_instance_id": inst.ID.String(),
		"target_type":           "auction",
		"auction_id":            card.ID.String(),
		"title":                 card.Title,
		"start_price":           card.StartPrice,
		"image_url":             card.ImageURL,
		"end_at":                card.EndAt.Format(time.RFC3339),
		"bid_count":             card.BidCount,
		"status":                card.Status,
		"seller_username":       sellerUsername,
		"seller_farm_name":      sellerFarmName,
		"seller_lifecycle":      sellerLifecycle,
	}
	if card.CurrentBid != nil {
		resp["current_bid"] = *card.CurrentBid
	}
	if card.BuyNowPrice != nil {
		resp["buy_now_price"] = *card.BuyNowPrice
	}
	return resp
}

func searchBuildExternalResponse(
	inst *promoentity.PromotionInstance,
	card *searchExternalProductCard,
	sellerUsername, sellerFarmName, sellerLifecycle string,
) map[string]interface{} {
	mediaURL := ""
	if card != nil && card.MediaURL != nil {
		mediaURL = *card.MediaURL
	}
	resp := map[string]interface{}{
		"type":                  "promoted_external",
		"promotion_instance_id": inst.ID.String(),
		"target_type":           "external_product",
		"target_id": func() string {
			if inst.TargetID != nil {
				return inst.TargetID.String()
			}
			return ""
		}(),
		"promoted": true,
	}
	if card != nil {
		resp["title"] = card.Title
		resp["description"] = card.Description
		resp["external_url"] = card.ExternalURL
		resp["image_url"] = mediaURL
		resp["external_media_url"] = mediaURL
		if card.MediaType != nil {
			resp["media_type"] = *card.MediaType
		}
		if card.ThumbnailURL != nil {
			resp["thumbnail_url"] = *card.ThumbnailURL
		}
	}
	resp["seller_username"] = sellerUsername
	resp["seller_farm_name"] = sellerFarmName
	resp["seller_lifecycle"] = sellerLifecycle
	return resp
}

// ---------- Slot policy ----------

// searchApplySlotPolicy filters hydrated promotions against organic results:
// - Skip if promoted target is already in organic results (target dedup)
// - Skip if promoted seller is already in organic sellers (seller dedup)
// - Skip duplicate targets within promoted candidates
// - Cap at searchMaxPromotedPerPage
func searchApplySlotPolicy(
	items []searchHydratedPromotion,
	organicIDs map[uuid.UUID]bool,
	organicSellerIDs map[uuid.UUID]bool,
) []searchHydratedPromotion {
	seenTargets := make(map[string]bool)
	seenSellers := make(map[uuid.UUID]bool)
	var result []searchHydratedPromotion

	for _, item := range items {
		// Organic target dedup: skip if this target is already in organic results.
		if item.Instance.TargetID != nil && organicIDs[*item.Instance.TargetID] {
			continue
		}

		// Organic seller dedup: skip if this seller is already in organic results.
		if organicSellerIDs[item.SellerID] {
			continue
		}

		// Within-promoted target dedup.
		targetKey := item.Instance.TargetType.String()
		if item.Instance.TargetID != nil {
			targetKey += ":" + item.Instance.TargetID.String()
		}
		if seenTargets[targetKey] {
			continue
		}

		// Within-promoted seller dedup.
		if seenSellers[item.SellerID] {
			continue
		}

		seenTargets[targetKey] = true
		seenSellers[item.SellerID] = true
		result = append(result, item)

		if len(result) >= searchMaxPromotedPerPage {
			break
		}
	}
	return result
}
