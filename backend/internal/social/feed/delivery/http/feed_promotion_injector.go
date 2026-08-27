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

// maxPromotedPerPage is the maximum number of promoted items injected per feed page.
const maxPromotedPerPage = 2

// minOrganicForInjection is the minimum organic items required before injection triggers.
const minOrganicForInjection = 3

// firstSlotIndex is the 0-based position after which the first promoted item is inserted.
// With value 2, promoted item appears after the 2nd organic item (i.e., at index 2).
const firstSlotIndex = 2

// secondSlotIndex is the 0-based position for the second promoted item.
const secondSlotIndex = 5

// FeedPromotionInjector handles fetching, hydrating, and interleaving promoted
// items into the organic feed response. All operations are fail-open — if any
// step errors, the organic feed is returned unchanged.
type FeedPromotionInjector struct {
	discoveryService *promotionApp.DiscoveryService
	db               promotionQueryPool
	log              *zap.Logger
}

// NewFeedPromotionInjector creates a new injector. All params are required;
// a nil discoveryService disables injection entirely.
func NewFeedPromotionInjector(
	discoveryService *promotionApp.DiscoveryService,
	database promotionQueryPool,
	log *zap.Logger,
) *FeedPromotionInjector {
	if log == nil {
		log = zap.NewNop()
	}
	return &FeedPromotionInjector{
		discoveryService: discoveryService,
		db:               database,
		log:              log,
	}
}

// hydratedPromotion is an intermediate struct holding hydrated card data.
type hydratedPromotion struct {
	Instance *promoentity.PromotionInstance
	SellerID uuid.UUID // for same-seller dedup
	Response map[string]interface{}
}

// InjectPromotions fetches promoted items, hydrates card data, and interleaves
// them into the organic feed items. Returns the merged items slice.
//
// FAIL-OPEN: Any error at any stage returns organicItems unchanged.
// The organic feed is NEVER degraded by promotion failures.
func (inj *FeedPromotionInjector) InjectPromotions(
	ctx context.Context,
	organicItems []map[string]interface{},
) []map[string]interface{} {
	// Nil receiver or nil discovery service → no injection.
	if inj == nil || inj.discoveryService == nil {
		return organicItems
	}

	// Policy: only inject if enough organic items.
	if len(organicItems) < minOrganicForInjection {
		return organicItems
	}

	// Fetch candidates (request more than we need for filtering headroom).
	candidates, err := inj.discoveryService.GetPromotedItems(ctx, maxPromotedPerPage*2)
	if err != nil {
		inj.log.Warn("feed promotion: discovery fetch failed, fail-open",
			zap.Error(err))
		return organicItems
	}
	if len(candidates) == 0 {
		return organicItems
	}

	// Hydrate forSale/auction target data for card rendering.
	hydrated, err := inj.hydratePromotedItems(ctx, candidates)
	if err != nil {
		inj.log.Warn("feed promotion: hydration failed, fail-open",
			zap.Error(err))
		return organicItems
	}
	if len(hydrated) == 0 {
		return organicItems
	}

	// Apply slot policy: dedup targets, same-seller filter, cap.
	filtered := applySlotPolicy(hydrated)
	if len(filtered) == 0 {
		return organicItems
	}

	// Interleave into organic items.
	return interleavePromotions(organicItems, filtered)
}

// hydratePromotedItems batch-hydrates forSale, auction, and external product
// card data for all promotion instances.
func (inj *FeedPromotionInjector) hydratePromotedItems(
	ctx context.Context,
	instances []*promoentity.PromotionInstance,
) ([]hydratedPromotion, error) {
	// Partition by target type.
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

	// Batch-fetch forSale card data.
	forSaleCards := make(map[uuid.UUID]*forSaleCardData)
	if len(forSaleIDs) > 0 {
		cards, err := inj.fetchForSaleCards(ctx, forSaleIDs)
		if err != nil {
			return nil, err
		}
		forSaleCards = cards
	}

	// Batch-fetch auction card data.
	auctionCards := make(map[uuid.UUID]*auctionCardData)
	if len(auctionIDs) > 0 {
		cards, err := inj.fetchAuctionCards(ctx, auctionIDs)
		if err != nil {
			return nil, err
		}
		auctionCards = cards
	}

	// Batch-fetch external product card data.
	externalProductCards := make(map[uuid.UUID]*externalProductCardData)
	if len(externalProductIDs) > 0 {
		cards, err := inj.fetchExternalProductCards(ctx, externalProductIDs)
		if err != nil {
			return nil, err
		}
		externalProductCards = cards
	}

	// Batch-fetch seller info (username + lifecycle).
	sellerInfos := make(map[uuid.UUID]*sellerInfo)
	if len(sellerIDs) > 0 {
		infos, err := inj.fetchSellerInfos(ctx, sellerIDs)
		if err != nil {
			return nil, err
		}
		sellerInfos = infos
	}

	// Build hydrated items.
	var result []hydratedPromotion
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
			result = append(result, hydratedPromotion{
				Instance: inst,
				SellerID: inst.UserID,
				Response: buildPromotedForSaleResponse(
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
			result = append(result, hydratedPromotion{
				Instance: inst,
				SellerID: inst.UserID,
				Response: buildPromotedAuctionResponse(
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
			result = append(result, hydratedPromotion{
				Instance: inst,
				SellerID: inst.UserID,
				Response: buildPromotedExternalResponse(
					inst, card, sellerUsername, sellerFarmName, sellerLifecycle,
				),
			})
		}
	}

	return result, nil
}

// ---------- Card data structs ----------

type forSaleCardData struct {
	ID           uuid.UUID
	Title        string
	PricePerUnit int64
	ImageURL     string // first media URL
}

type auctionCardData struct {
	ID          uuid.UUID
	Title       string
	StartPrice  int64
	CurrentBid  *int64
	BuyNowPrice *int64
	EndAt       time.Time
	BidCount    int
	Status      string
	ImageURL    string // from forSale media_urls
}

type externalProductCardData struct {
	ID           uuid.UUID
	OwnerUserID  uuid.UUID
	Title        string
	Description  *string
	ExternalURL  string
	MediaURL     *string
	MediaType    *string
	ThumbnailURL *string
}

type sellerInfo struct {
	UserID    uuid.UUID
	Username  string
	FarmName  string
	Lifecycle viewercontext.PublicLifecycleState
}

// ---------- Batch fetch helpers ----------

func (inj *FeedPromotionInjector) fetchForSaleCards(
	ctx context.Context,
	ids []uuid.UUID,
) (map[uuid.UUID]*forSaleCardData, error) {
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

	result := make(map[uuid.UUID]*forSaleCardData)
	for rows.Next() {
		var card forSaleCardData
		var mediaURLsRaw json.RawMessage
		if err := rows.Scan(&card.ID, &card.Title, &card.PricePerUnit, &mediaURLsRaw); err != nil {
			continue
		}
		card.ImageURL = extractFirstMediaURL(mediaURLsRaw)
		result[card.ID] = &card
	}
	return result, nil
}

func (inj *FeedPromotionInjector) fetchAuctionCards(
	ctx context.Context,
	ids []uuid.UUID,
) (map[uuid.UUID]*auctionCardData, error) {
	// auctions.for_sale_id was a legacy column (never set for product-based
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

	result := make(map[uuid.UUID]*auctionCardData)
	for rows.Next() {
		var card auctionCardData
		var mediaURLsRaw json.RawMessage
		if err := rows.Scan(
			&card.ID, &card.Title, &card.StartPrice, &card.CurrentBid,
			&card.BuyNowPrice, &card.EndAt, &card.Status,
			&mediaURLsRaw, &card.BidCount,
		); err != nil {
			continue
		}
		card.ImageURL = extractFirstMediaURL(mediaURLsRaw)
		result[card.ID] = &card
	}
	return result, nil
}

func (inj *FeedPromotionInjector) fetchExternalProductCards(
	ctx context.Context,
	ids []uuid.UUID,
) (map[uuid.UUID]*externalProductCardData, error) {
	if inj == nil || inj.db == nil {
		return map[uuid.UUID]*externalProductCardData{}, nil
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

	result := make(map[uuid.UUID]*externalProductCardData)
	for rows.Next() {
		var card externalProductCardData
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

func (inj *FeedPromotionInjector) fetchSellerInfos(
	ctx context.Context,
	userIDs []uuid.UUID,
) (map[uuid.UUID]*sellerInfo, error) {
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

	result := make(map[uuid.UUID]*sellerInfo)
	for rows.Next() {
		var info sellerInfo
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

// extractFirstMediaURL parses a JSONB array of media URLs and returns the first one.
func extractFirstMediaURL(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var urls []string
	if err := json.Unmarshal(raw, &urls); err != nil {
		// Try as array of objects with "url" field
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

func buildPromotedForSaleResponse(
	inst *promoentity.PromotionInstance,
	card *forSaleCardData,
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

func buildPromotedAuctionResponse(
	inst *promoentity.PromotionInstance,
	card *auctionCardData,
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

func buildPromotedExternalResponse(
	inst *promoentity.PromotionInstance,
	card *externalProductCardData,
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

// applySlotPolicy filters hydrated promotions: no duplicate targets on the same
// page, no two items from the same seller, capped at maxPromotedPerPage.
func applySlotPolicy(items []hydratedPromotion) []hydratedPromotion {
	seenTargets := make(map[string]bool)
	seenSellers := make(map[uuid.UUID]bool)
	var result []hydratedPromotion

	for _, item := range items {
		// Dedup by target.
		targetKey := item.Instance.TargetType.String()
		if item.Instance.TargetID != nil {
			targetKey += ":" + item.Instance.TargetID.String()
		}
		if seenTargets[targetKey] {
			continue
		}

		// Same-seller dedup.
		if seenSellers[item.SellerID] {
			continue
		}

		seenTargets[targetKey] = true
		seenSellers[item.SellerID] = true
		result = append(result, item)

		if len(result) >= maxPromotedPerPage {
			break
		}
	}
	return result
}

// interleavePromotions merges promoted items into the organic feed at
// predetermined slot positions. Promoted items are inserted BEFORE the
// organic item at the slot index. For example, with firstSlotIndex=2:
//
//	[organic0, organic1, PROMO1, organic2, organic3, ...]
//
// If a slot index exceeds the organic count, the promoted item is appended.
func interleavePromotions(
	organic []map[string]interface{},
	promoted []hydratedPromotion,
) []map[string]interface{} {
	if len(promoted) == 0 {
		return organic
	}

	// Slot positions in terms of organic indices. A promoted item is inserted
	// before organic[slot]. When there are two promoted items, the second slot
	// index accounts for the first promoted item already being in the output.
	insertBeforeOrganic := []int{firstSlotIndex}
	if len(promoted) > 1 {
		insertBeforeOrganic = append(insertBeforeOrganic, secondSlotIndex)
	}

	result := make([]map[string]interface{}, 0, len(organic)+len(promoted))
	promoIdx := 0

	for i, item := range organic {
		// Insert promoted item before this organic index if it matches a slot.
		if promoIdx < len(promoted) && promoIdx < len(insertBeforeOrganic) && i == insertBeforeOrganic[promoIdx] {
			result = append(result, promoted[promoIdx].Response)
			promoIdx++
		}
		result = append(result, item)
	}

	// Append any remaining promoted items at the end (when organic is shorter
	// than the slot position).
	for ; promoIdx < len(promoted); promoIdx++ {
		result = append(result, promoted[promoIdx].Response)
	}

	return result
}
