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

const searchMaxPromotedPerPage = 1
const searchMinOrganicForInjection = 3
const searchInjectAtIndex = 2

// CanonicalSearchHandoff is the canonical contract delivery source for search.
// Mirrors feed injector's CanonicalPromotionHandoff — single authority: promotion_contracts.
type CanonicalSearchHandoff interface {
	SelectForDelivery(ctx context.Context, limit int) ([]contractApp.DeliveryCandidate, error)
}

// SearchPromotionInjector builds a promoted items sidecar for search responses.
// Canonical authority: promotion_contracts → queue → Delivery Ticket →
// Qualified Impression → CPM charge (the single canonical delivery/billing
// path — same TicketIssuer authority as the feed bridge).
//
// VIEWER BINDING: a Delivery Ticket requires a bound viewer identity
// (structural self-delivery detection + qualification), so anonymous search
// viewers receive NO promoted delivery.
type SearchPromotionInjector struct {
	canonicalHandoff CanonicalSearchHandoff
	issuer           deliveryApp.TicketIssuer
	db               promotionQueryPool
	log              *zap.Logger
}

func NewSearchPromotionInjector(
	canonicalHandoff CanonicalSearchHandoff,
	issuer deliveryApp.TicketIssuer,
	database promotionQueryPool,
	log *zap.Logger,
) *SearchPromotionInjector {
	if log == nil {
		log = zap.NewNop()
	}
	return &SearchPromotionInjector{
		canonicalHandoff: canonicalHandoff,
		issuer:           issuer,
		db:               database,
		log:              log,
	}
}

type searchHydratedPromotion struct {
	Candidate *contractApp.DeliveryCandidate
	SellerID  uuid.UUID
	Response  map[string]interface{}
}

func (inj *SearchPromotionInjector) GetPromotedSidecar(
	ctx context.Context,
	organicIDs []uuid.UUID,
	organicSellerIDs []uuid.UUID,
) []map[string]interface{} {
	return inj.GetPromotedSidecarWithGeography(ctx, organicIDs, organicSellerIDs, uuid.Nil, "", false)
}

func (inj *SearchPromotionInjector) GetPromotedSidecarWithGeography(
	ctx context.Context,
	organicIDs []uuid.UUID,
	organicSellerIDs []uuid.UUID,
	viewerID uuid.UUID,
	viewerCityID string,
	viewerHasPrimary bool,
) []map[string]interface{} {
	if inj == nil || inj.canonicalHandoff == nil {
		return nil
	}
	if len(organicIDs) < searchMinOrganicForInjection {
		return nil
	}
	// Canonical viewer binding: a Delivery Ticket requires a bound viewer —
	// anonymous search viewers receive no promoted delivery at all.
	if viewerID == uuid.Nil {
		return nil
	}
	candidates, err := inj.canonicalHandoff.SelectForDelivery(ctx, searchMaxPromotedPerPage*3)
	if err != nil {
		// Selection carries the canonical delivery gate: when promotion
		// delivery is platform-disabled it returns ErrDeliveryDisabled and
		// NOTHING is distributed — no promoted cards, no tickets, no charges.
		inj.log.Warn("search promotion: canonical handoff failed, fail-open", zap.Error(err))
		return nil
	}
	if len(candidates) == 0 {
		return nil
	}
	hydrated, err := inj.hydrateSearchPromotedCandidates(ctx, candidates)
	if err != nil {
		inj.log.Warn("search promotion: hydration failed, fail-open", zap.Error(err))
		return nil
	}
	if len(hydrated) == 0 {
		return nil
	}
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
	// CANONICAL DELIVERY TICKET ISSUANCE — one server-issued, viewer-bound
	// ticket per card actually about to be delivered (full delivery gate:
	// platform enablement, active contract, no seller self-delivery,
	// geographic eligibility, pacing envelope). A card that cannot be
	// authorized is NOT delivered (fail-closed for delivery).
	filtered = inj.issueDeliveryTickets(ctx, filtered, viewerID, viewerCityID, viewerHasPrimary)
	if len(filtered) == 0 {
		return nil
	}
	sidecar := make([]map[string]interface{}, 0, len(filtered))
	for _, item := range filtered {
		item.Response["inject_at"] = searchInjectAtIndex
		sidecar = append(sidecar, item.Response)
	}
	// CANONICAL QUALIFICATION — the ONLY billing path: every delivered card is
	// server-qualified into the immutable Qualified Impression + exact CPM
	// charge. Errors never fail the search response (fail-open presentation;
	// money safety lives inside the qualification transaction).
	inj.qualifyDeliveredTickets(ctx, filtered)
	return sidecar
}

// issueDeliveryTickets issues the canonical viewer-bound Delivery Ticket for
// each candidate card; unauthorized cards are dropped.
func (inj *SearchPromotionInjector) issueDeliveryTickets(
	ctx context.Context,
	items []searchHydratedPromotion,
	viewerID uuid.UUID,
	viewerCityID string,
	viewerHasPrimary bool,
) []searchHydratedPromotion {
	if inj.issuer == nil {
		// No ticket authority wired: cards can be presented but never bill.
		// Narrowly-scoped composition tests only; production wiring always
		// supplies the canonical issuer.
		return items
	}
	issued := make([]searchHydratedPromotion, 0, len(items))
	for _, item := range items {
		if item.Candidate == nil {
			continue
		}
		ticket, err := inj.issuer.IssueTicket(ctx, deliveryApp.IssueTicketInput{
			ContractID:       item.Candidate.ContractID,
			TargetType:       promoentity.TargetType(item.Candidate.TargetType),
			TargetID:         item.Candidate.TargetID,
			ViewerID:         viewerID,
			ViewerCityID:     viewerCityID,
			ViewerHasPrimary: viewerHasPrimary,
		})
		if err != nil {
			inj.log.Warn("search promotion: delivery ticket not issued, card dropped",
				zap.String("contract_id", item.Candidate.ContractID.String()),
				zap.Error(err),
			)
			continue
		}
		item.Response["canonical_ticket_id"] = ticket.ID.String()
		issued = append(issued, item)
	}
	return issued
}

// qualifyDeliveredTickets runs the canonical server-side qualification for
// every delivered card — the ONLY path that produces a billable Qualified
// Impression and books the exact CPM charge to the ledger.
func (inj *SearchPromotionInjector) qualifyDeliveredTickets(ctx context.Context, items []searchHydratedPromotion) {
	if inj.issuer == nil {
		return
	}
	for _, item := range items {
		if item.Candidate == nil {
			continue
		}
		ticketIDRaw, ok := item.Response["canonical_ticket_id"]
		if !ok {
			continue
		}
		ticketIDStr, ok := ticketIDRaw.(string)
		if !ok {
			continue
		}
		ticketID, err := uuid.Parse(ticketIDStr)
		if err != nil {
			continue
		}
		qi, err := inj.issuer.QualifyTicket(ctx, deliveryApp.QualifyTicketInput{TicketID: ticketID})
		if err != nil {
			inj.log.Warn("search promotion: qualification did not produce a billable impression",
				zap.String("contract_id", item.Candidate.ContractID.String()),
				zap.String("ticket_id", ticketID.String()),
				zap.Error(err),
			)
			continue
		}
		inj.log.Info("search promotion: qualified impression recorded",
			zap.String("contract_id", qi.ContractID.String()),
			zap.String("ticket_id", ticketID.String()),
			zap.Int64("charge_rupiah", qi.ChargeRupiah),
		)
	}
}

// ---------- Hydration over canonical candidates ----------

func (inj *SearchPromotionInjector) hydrateSearchPromotedCandidates(
	ctx context.Context,
	candidates []contractApp.DeliveryCandidate,
) ([]searchHydratedPromotion, error) {
	var forSaleIDs, auctionIDs, externalProductIDs []uuid.UUID
	var sellerIDs []uuid.UUID
	for i := range candidates {
		c := &candidates[i]
		tt := promoentity.TargetType(c.TargetType)
		if !tt.IsPublicPromotable() {
			continue
		}
		switch tt {
		case promoentity.TargetTypeForSale:
			forSaleIDs = append(forSaleIDs, c.TargetID)
			sellerIDs = append(sellerIDs, c.SellerID)
		case promoentity.TargetTypeAuction:
			auctionIDs = append(auctionIDs, c.TargetID)
			sellerIDs = append(sellerIDs, c.SellerID)
		case promoentity.TargetTypeExternalProduct:
			externalProductIDs = append(externalProductIDs, c.TargetID)
			sellerIDs = append(sellerIDs, c.SellerID)
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
	for i := range candidates {
		c := &candidates[i]
		tt := promoentity.TargetType(c.TargetType)
		seller := sellerInfos[c.SellerID]
		sellerUsername, sellerFarmName, sellerLifecycle := "", "", "active"
		if seller != nil {
			sellerUsername = seller.Username
			sellerFarmName = seller.FarmName
			sellerLifecycle = string(seller.Lifecycle)
		}
		switch tt {
		case promoentity.TargetTypeForSale:
			card, ok := forSaleCards[c.TargetID]
			if !ok {
				continue
			}
			result = append(result, searchHydratedPromotion{
				Candidate: c,
				SellerID:  c.SellerID,
				Response: searchBuildForSaleResponseCanonical(
					c, card, sellerUsername, sellerFarmName, sellerLifecycle,
				),
			})
		case promoentity.TargetTypeAuction:
			card, ok := auctionCards[c.TargetID]
			if !ok {
				continue
			}
			result = append(result, searchHydratedPromotion{
				Candidate: c,
				SellerID:  c.SellerID,
				Response: searchBuildAuctionResponseCanonical(
					c, card, sellerUsername, sellerFarmName, sellerLifecycle,
				),
			})
		case promoentity.TargetTypeExternalProduct:
			card, ok := externalProductCards[c.TargetID]
			if !ok {
				continue
			}
			result = append(result, searchHydratedPromotion{
				Candidate: c,
				SellerID:  c.SellerID,
				Response: searchBuildExternalResponseCanonical(
					c, card, sellerUsername, sellerFarmName, sellerLifecycle,
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

// ---------- Response builders (canonical) ----------

func searchBuildForSaleResponseCanonical(
	c *contractApp.DeliveryCandidate,
	card *searchForSaleCard,
	sellerUsername, sellerFarmName, sellerLifecycle string,
) map[string]interface{} {
	cid := c.ContractID
	return map[string]interface{}{
		"type":           "promoted_for_sale",
		"contract_id":    cid.String(),
		"target_type":    "for_sale",
		"for_sale_id":    card.ID.String(),
		"title":          card.Title,
		"price_per_unit": card.PricePerUnit,
		"image_url":      card.ImageURL,
		"seller_username":  sellerUsername,
		"seller_farm_name": sellerFarmName,
		"seller_lifecycle": sellerLifecycle,
	}
}

func searchBuildAuctionResponseCanonical(
	c *contractApp.DeliveryCandidate,
	card *searchAuctionCard,
	sellerUsername, sellerFarmName, sellerLifecycle string,
) map[string]interface{} {
	cid := c.ContractID
	resp := map[string]interface{}{
		"type":           "promoted_auction",
		"contract_id":    cid.String(),
		"target_type":    "auction",
		"auction_id":     card.ID.String(),
		"title":          card.Title,
		"start_price":    card.StartPrice,
		"image_url":      card.ImageURL,
		"end_at":         card.EndAt.Format(time.RFC3339),
		"bid_count":      card.BidCount,
		"status":         card.Status,
		"seller_username":  sellerUsername,
		"seller_farm_name": sellerFarmName,
		"seller_lifecycle": sellerLifecycle,
	}
	if card.CurrentBid != nil {
		resp["current_bid"] = *card.CurrentBid
	}
	if card.BuyNowPrice != nil {
		resp["buy_now_price"] = *card.BuyNowPrice
	}
	return resp
}

func searchBuildExternalResponseCanonical(
	c *contractApp.DeliveryCandidate,
	card *searchExternalProductCard,
	sellerUsername, sellerFarmName, sellerLifecycle string,
) map[string]interface{} {
	cid := c.ContractID
	mediaURL := ""
	if card != nil && card.MediaURL != nil {
		mediaURL = *card.MediaURL
	}
	resp := map[string]interface{}{
		"type":        "promoted_external",
		"contract_id": cid.String(),
		"target_type": "external_product",
		"target_id":   c.TargetID.String(),
		"promoted":    true,
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

func searchApplySlotPolicy(
	items []searchHydratedPromotion,
	organicIDs map[uuid.UUID]bool,
	organicSellerIDs map[uuid.UUID]bool,
) []searchHydratedPromotion {
	seenTargets := make(map[string]bool)
	seenSellers := make(map[uuid.UUID]bool)
	var result []searchHydratedPromotion
	for _, item := range items {
		if item.Candidate != nil && organicIDs[item.Candidate.TargetID] {
			continue
		}
		if organicSellerIDs[item.SellerID] {
			continue
		}
		targetKey := ""
		if item.Candidate != nil {
			targetKey = item.Candidate.TargetType + ":" + item.Candidate.TargetID.String()
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
		if len(result) >= searchMaxPromotedPerPage {
			break
		}
	}
	return result
}
