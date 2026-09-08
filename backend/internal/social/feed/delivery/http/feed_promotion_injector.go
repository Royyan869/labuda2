package http

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/governance/viewercontext"
	contractApp "github.com/labuda/backend/internal/pricing/promotion/contract/application"
	deliveryApp "github.com/labuda/backend/internal/pricing/promotion/delivery/application"
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
const firstSlotIndex = 2

// secondSlotIndex is the 0-based position for the second promoted item.
const secondSlotIndex = 5

// CanonicalPromotionHandoff is the canonical delivery source accepted at the
// feed bridge. It is the production consumer boundary of the canonical
// contract selection pipeline (promotion_contracts + queue authority).
type CanonicalPromotionHandoff interface {
	SelectForDelivery(ctx context.Context, limit int) ([]contractApp.DeliveryCandidate, error)
}

// CanonicalDeliveryMeasurement is the canonical delivery measurement source
// accepted at the feed bridge (canonical_promotion_delivery_events,
// contract_id authority).
type CanonicalDeliveryMeasurement interface {
	RecordIncluded(ctx context.Context, inclusions []deliveryApp.DeliveryInclusion) (map[uuid.UUID]uuid.UUID, error)
}

// FeedPromotionInjector handles fetching, hydrating, and interleaving promoted
// items into the organic feed response. All operations are fail-open.
// Canonical-only: promotion_contracts is the sole authority.
type FeedPromotionInjector struct {
	canonicalHandoff CanonicalPromotionHandoff
	canonicalMeasure CanonicalDeliveryMeasurement
	db               promotionQueryPool
	log              *zap.Logger
}

// NewFeedPromotionInjector creates a canonical-only injector.
func NewFeedPromotionInjector(
	canonicalHandoff CanonicalPromotionHandoff,
	canonicalMeasure CanonicalDeliveryMeasurement,
	database promotionQueryPool,
	log *zap.Logger,
) *FeedPromotionInjector {
	if log == nil {
		log = zap.NewNop()
	}
	return &FeedPromotionInjector{
		canonicalHandoff: canonicalHandoff,
		canonicalMeasure: canonicalMeasure,
		db:               database,
		log:              log,
	}
}

// promotedTargetView is the neutral identity/target surface the hydration
// layer consumes.
type promotedTargetView struct {
	DeliveryItemID uuid.UUID
	SellerUserID   uuid.UUID
	TargetType     promoentity.TargetType
	TargetID       *uuid.UUID
}

// viewFromCandidate maps a canonical delivery candidate onto the neutral surface.
func viewFromCandidate(cand contractApp.DeliveryCandidate) promotedTargetView {
	targetID := cand.TargetID
	return promotedTargetView{
		DeliveryItemID: cand.ContractID,
		SellerUserID:   cand.SellerID,
		TargetType:     promoentity.TargetType(cand.TargetType),
		TargetID:       &targetID,
	}
}

// hydratedPromotion is an intermediate struct holding hydrated card data.
type hydratedPromotion struct {
	View     *promotedTargetView
	SellerID uuid.UUID
	Response map[string]interface{}
	Canonical bool
}

// InjectPromotions fetches promoted items, hydrates card data, and interleaves
// them into the organic feed items. Returns the merged items slice.
// viewerID is the authenticated feed viewer.
// FAIL-OPEN: Any error at any stage returns organicItems unchanged.
// MEASUREMENT (canonical only): after the response is built, every canonical
// card that was actually placed into the merged response is reported to the
// canonical measurement sink as an 'included' observation.
func (inj *FeedPromotionInjector) InjectPromotions(
	ctx context.Context,
	viewerID uuid.UUID,
	organicItems []map[string]interface{},
) []map[string]interface{} {
	return inj.InjectPromotionsWithGeography(ctx, viewerID, "", false, organicItems)
}

// InjectPromotionsWithGeography is the canonical geo-aware injection path.
// viewerCityID/viewerHasPrimary come from ViewerContext GeographyOverlay (primary address).
func (inj *FeedPromotionInjector) InjectPromotionsWithGeography(
	ctx context.Context,
	viewerID uuid.UUID,
	viewerCityID string,
	viewerHasPrimary bool,
	organicItems []map[string]interface{},
) []map[string]interface{} {
	if inj == nil || inj.canonicalHandoff == nil {
		return organicItems
	}
	if len(organicItems) < minOrganicForInjection {
		return organicItems
	}
	candidates, err := inj.canonicalHandoff.SelectForDelivery(ctx, maxPromotedPerPage*2)
	if err != nil {
		inj.log.Warn("feed promotion: canonical handoff failed, fail-open",
			zap.Error(err))
		return organicItems
	}
	if len(candidates) == 0 {
		return organicItems
	}
	// Geographic eligibility — empty geography = nationwide
	candidates = inj.filterCandidatesByGeography(ctx, candidates, viewerCityID, viewerHasPrimary)
	if len(candidates) == 0 {
		return organicItems
	}
	hydrated, err := inj.hydrateCanonicalDeliveryItems(ctx, candidates)
	if err != nil {
		inj.log.Warn("feed promotion: canonical hydration failed, fail-open",
			zap.Error(err))
		return organicItems
	}
	if len(hydrated) == 0 {
		return organicItems
	}
	filtered := applySlotPolicy(hydrated)
	if len(filtered) == 0 {
		return organicItems
	}
	merged := interleavePromotions(organicItems, filtered)
	if inj.canonicalMeasure != nil {
		inclusions := make([]deliveryApp.DeliveryInclusion, 0, len(filtered))
		for _, item := range filtered {
			if !item.Canonical || item.View == nil || item.View.TargetID == nil {
				continue
			}
			inclusions = append(inclusions, deliveryApp.DeliveryInclusion{
				ContractID: item.View.DeliveryItemID,
				TargetType: string(item.View.TargetType),
				TargetID:   *item.View.TargetID,
				ViewerID:   viewerID,
			})
		}
		if len(inclusions) > 0 {
			exposures, err := inj.canonicalMeasure.RecordIncluded(ctx, inclusions)
			if err != nil {
				inj.log.Warn("feed promotion: canonical measurement failed, fail-open",
					zap.Error(err))
			} else {
				for _, item := range filtered {
					if !item.Canonical || item.View == nil {
						continue
					}
					if exposureID, ok := exposures[item.View.DeliveryItemID]; ok {
						item.Response["canonical_exposure_id"] = exposureID.String()
					}
				}
			}
		}
	}
	return merged
}

func (inj *FeedPromotionInjector) filterCandidatesByGeography(ctx context.Context, candidates []contractApp.DeliveryCandidate, viewerCityID string, viewerHasPrimary bool) []contractApp.DeliveryCandidate {
	if inj.db == nil {
		return candidates
	}
	if len(candidates) == 0 {
		return candidates
	}
	ids := make([]uuid.UUID, 0, len(candidates))
	for _, c := range candidates {
		ids = append(ids, c.ContractID)
	}
	rows, err := inj.db.Query(ctx, `SELECT contract_id, city_id FROM promotion_contract_geographies WHERE contract_id = ANY($1)`, ids)
	if err != nil {
		return candidates
	}
	defer rows.Close()
	contractCities := make(map[uuid.UUID]map[string]struct{})
	contractHasRestriction := make(map[uuid.UUID]bool)
	for rows.Next() {
		var cid uuid.UUID
		var cityID string
		if err := rows.Scan(&cid, &cityID); err != nil {
			continue
		}
		if _, ok := contractCities[cid]; !ok {
			contractCities[cid] = make(map[string]struct{})
		}
		contractCities[cid][cityID] = struct{}{}
		contractHasRestriction[cid] = true
	}
	var out []contractApp.DeliveryCandidate
	for _, c := range candidates {
		cid := c.ContractID
		hasRestriction := contractHasRestriction[cid]
		if !hasRestriction {
			out = append(out, c)
			continue
		}
		if !viewerHasPrimary || viewerCityID == "" {
			continue
		}
		if _, ok := contractCities[cid][viewerCityID]; ok {
			out = append(out, c)
		}
	}
	return out
}

// hydrateCanonicalDeliveryItems is the canonical feed bridge.
func (inj *FeedPromotionInjector) hydrateCanonicalDeliveryItems(
	ctx context.Context,
	candidates []contractApp.DeliveryCandidate,
) ([]hydratedPromotion, error) {
	views := make([]promotedTargetView, 0, len(candidates))
	for _, cand := range candidates {
		views = append(views, viewFromCandidate(cand))
	}
	return inj.hydrateViews(ctx, views, true)
}

// hydrateViews is the shared hydration core over the neutral delivery surface.
func (inj *FeedPromotionInjector) hydrateViews(
	ctx context.Context,
	views []promotedTargetView,
	canonicalSource bool,
) ([]hydratedPromotion, error) {
	var forSaleIDs, auctionIDs, externalProductIDs []uuid.UUID
	var sellerIDs []uuid.UUID
	for i := range views {
		view := &views[i]
		if !view.TargetType.IsPublicPromotable() {
			continue
		}
		if view.TargetType == promoentity.TargetTypeForSale && view.TargetID != nil {
			forSaleIDs = append(forSaleIDs, *view.TargetID)
			sellerIDs = append(sellerIDs, view.SellerUserID)
		} else if view.TargetType == promoentity.TargetTypeAuction && view.TargetID != nil {
			auctionIDs = append(auctionIDs, *view.TargetID)
			sellerIDs = append(sellerIDs, view.SellerUserID)
		} else if view.TargetType == promoentity.TargetTypeExternalProduct && view.TargetID != nil {
			externalProductIDs = append(externalProductIDs, *view.TargetID)
			sellerIDs = append(sellerIDs, view.SellerUserID)
		}
	}
	forSaleCards := make(map[uuid.UUID]*forSaleCardData)
	if len(forSaleIDs) > 0 {
		cards, err := inj.fetchForSaleCards(ctx, forSaleIDs)
		if err != nil {
			return nil, err
		}
		forSaleCards = cards
	}
	auctionCards := make(map[uuid.UUID]*auctionCardData)
	if len(auctionIDs) > 0 {
		cards, err := inj.fetchAuctionCards(ctx, auctionIDs)
		if err != nil {
			return nil, err
		}
		auctionCards = cards
	}
	externalProductCards := make(map[uuid.UUID]*externalProductCardData)
	if len(externalProductIDs) > 0 {
		cards, err := inj.fetchExternalProductCards(ctx, externalProductIDs)
		if err != nil {
			return nil, err
		}
		externalProductCards = cards
	}
	sellerInfos := make(map[uuid.UUID]*sellerInfo)
	if len(sellerIDs) > 0 {
		infos, err := inj.fetchSellerInfos(ctx, sellerIDs)
		if err != nil {
			return nil, err
		}
		sellerInfos = infos
	}
	var result []hydratedPromotion
	for i := range views {
		view := &views[i]
		seller := sellerInfos[view.SellerUserID]
		sellerUsername := ""
		sellerFarmName := ""
		sellerLifecycle := "active"
		if seller != nil {
			sellerUsername = seller.Username
			sellerFarmName = seller.FarmName
			sellerLifecycle = string(seller.Lifecycle)
		}
		switch view.TargetType {
		case promoentity.TargetTypeForSale:
			if view.TargetID == nil {
				continue
			}
			card, ok := forSaleCards[*view.TargetID]
			if !ok {
				continue
			}
			result = append(result, hydratedPromotion{
				View:      view,
				SellerID:  view.SellerUserID,
				Canonical: canonicalSource,
				Response: buildPromotedForSaleResponse(
					view, card, sellerUsername, sellerFarmName, sellerLifecycle,
				),
			})
		case promoentity.TargetTypeAuction:
			if view.TargetID == nil {
				continue
			}
			card, ok := auctionCards[*view.TargetID]
			if !ok {
				continue
			}
			result = append(result, hydratedPromotion{
				View:      view,
				SellerID:  view.SellerUserID,
				Canonical: canonicalSource,
				Response: buildPromotedAuctionResponse(
					view, card, sellerUsername, sellerFarmName, sellerLifecycle,
				),
			})
		case promoentity.TargetTypeExternalProduct:
			if view.TargetID == nil {
				continue
			}
			card, ok := externalProductCards[*view.TargetID]
			if !ok {
				continue
			}
			result = append(result, hydratedPromotion{
				View:      view,
				SellerID:  view.SellerUserID,
				Canonical: canonicalSource,
				Response: buildPromotedExternalResponse(
					view, card, sellerUsername, sellerFarmName, sellerLifecycle,
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
	ImageURL     string
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
	ImageURL    string
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

func extractFirstMediaURL(raw json.RawMessage) string {
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

func buildPromotedForSaleResponse(
	view *promotedTargetView,
	card *forSaleCardData,
	sellerUsername, sellerFarmName, sellerLifecycle string,
) map[string]interface{} {
	return map[string]interface{}{
		"type":        "promoted_for_sale",
		"contract_id": view.DeliveryItemID.String(),
		"target_type": "for_sale",
		"for_sale_id":           card.ID.String(),
		"title":                 card.Title,
		"price_per_unit":        card.PricePerUnit,
		"image_url":             card.ImageURL,
		"seller_username":       sellerUsername,
		"seller_farm_name":      sellerFarmName,
		"seller_lifecycle":      sellerLifecycle,
	}
}

func buildPromotedAuctionResponse(
	view *promotedTargetView,
	card *auctionCardData,
	sellerUsername, sellerFarmName, sellerLifecycle string,
) map[string]interface{} {
	resp := map[string]interface{}{
		"type":        "promoted_auction",
		"contract_id": view.DeliveryItemID.String(),
		"target_type": "auction",
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
	view *promotedTargetView,
	card *externalProductCardData,
	sellerUsername, sellerFarmName, sellerLifecycle string,
) map[string]interface{} {
	mediaURL := ""
	if card != nil && card.MediaURL != nil {
		mediaURL = *card.MediaURL
	}
	resp := map[string]interface{}{
		"type":        "promoted_external",
		"contract_id": view.DeliveryItemID.String(),
		"target_type": "external_product",
		"target_id": func() string {
			if view.TargetID != nil {
				return view.TargetID.String()
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

func applySlotPolicy(items []hydratedPromotion) []hydratedPromotion {
	seenTargets := make(map[string]bool)
	seenSellers := make(map[uuid.UUID]bool)
	var result []hydratedPromotion
	for _, item := range items {
		targetKey := item.View.TargetType.String()
		if item.View.TargetID != nil {
			targetKey += ":" + item.View.TargetID.String()
		}
		if seenTargets[targetKey] {
			continue
		}
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

func interleavePromotions(
	organic []map[string]interface{},
	promoted []hydratedPromotion,
) []map[string]interface{} {
	if len(promoted) == 0 {
		return organic
	}
	insertBeforeOrganic := []int{firstSlotIndex}
	if len(promoted) > 1 {
		insertBeforeOrganic = append(insertBeforeOrganic, secondSlotIndex)
	}
	result := make([]map[string]interface{}, 0, len(organic)+len(promoted))
	promoIdx := 0
	for i, item := range organic {
		if promoIdx < len(promoted) && promoIdx < len(insertBeforeOrganic) && i == insertBeforeOrganic[promoIdx] {
			result = append(result, promoted[promoIdx].Response)
			promoIdx++
		}
		result = append(result, item)
	}
	for ; promoIdx < len(promoted); promoIdx++ {
		result = append(result, promoted[promoIdx].Response)
	}
	return result
}
