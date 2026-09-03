package serverboot

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	commerceshared "github.com/labuda/backend/internal/commerce/shared"
	"github.com/labuda/backend/internal/governance/viewercontext"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/internal/pkg/blockcheck"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/internal/platform/mediaresolve"
	"github.com/labuda/backend/pkg/db"
)

type auctionProjectionBatchResolver struct {
	db auctionProjectionDB
}

type auctionProjectionDB interface {
	WithTx(ctx context.Context, fn func(db.Tx) error) error
}

type auctionProjectionSourceRow struct {
	auctionID          uuid.UUID
	sellerID           uuid.UUID
	productID          uuid.UUID
	orderID            *uuid.UUID
	startPrice         int64
	bidIncrement       int64
	buyNowPrice        *int64
	startAt            time.Time
	endAt              time.Time
	currentBid         *int64
	currentWinnerID    *uuid.UUID
	status             string
	createdAt          time.Time
	updatedAt          time.Time
	productTitle       string
	productMediaURLs   json.RawMessage
	productVariety     string
	productSizeCm      *int
	productAgeMonths   *int
	productGender      *string
	productBreeder     *string
	productBloodline   *string
	productPreparation string
	productNote        *string
}

type auctionProjectionSellerRow struct {
	found              bool
	username           string
	farmName           string
	storeImageURL      string
	avatarURL          string
	accountStatus      string
	isDeleted          bool
	subscriptionStatus string
	tier               string
}

func newAuctionProjectionBatchResolver(database auctionProjectionDB) *auctionProjectionBatchResolver {
	return &auctionProjectionBatchResolver{db: database}
}

var _ chatApp.ResourceProjectionResolver = (*auctionProjectionBatchResolver)(nil)

func (r *auctionProjectionBatchResolver) ResolveResourceProjections(
	ctx context.Context,
	viewerID uuid.UUID,
	occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
) (map[uuid.UUID]*chatApp.ResourceProjection, error) {
	return r.ResolveAuctions(ctx, viewerID, occurrences)
}

// ResolveAuctions resolves only auction occurrences. Unsupported occurrence
// types are rejected rather than silently omitted or tombstoned.
func (r *auctionProjectionBatchResolver) ResolveAuctions(
	ctx context.Context,
	viewerID uuid.UUID,
	occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
) (map[uuid.UUID]*chatApp.ResourceProjection, error) {
	if len(occurrences) == 0 {
		return map[uuid.UUID]*chatApp.ResourceProjection{}, nil
	}
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("chat: auction projection resolver not configured")
	}

	auctionIDs := make([]uuid.UUID, 0, len(occurrences))
	seenAuctionIDs := make(map[uuid.UUID]struct{}, len(occurrences))
	occurrenceByMsgID := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, len(occurrences))

	for messageID, occ := range occurrences {
		if occ == nil {
			return nil, fmt.Errorf("chat: nil occurrence for message %s", messageID)
		}
		if !occ.ResourceType().IsValid() {
			return nil, fmt.Errorf("chat: malformed occurrence identity for message %s", messageID)
		}
		if occ.ResourceType() != chatEntity.ResourceOccurrenceResourceTypeAuction {
			return nil, fmt.Errorf("chat: unsupported resource type %q in auction resolver", occ.ResourceType())
		}
		if occ.AuctionSourceID == nil {
			return nil, fmt.Errorf("chat: auction occurrence for message %s requires non-nil auction source id", messageID)
		}

		if _, seen := seenAuctionIDs[*occ.AuctionSourceID]; !seen {
			auctionIDs = append(auctionIDs, *occ.AuctionSourceID)
			seenAuctionIDs[*occ.AuctionSourceID] = struct{}{}
		}
		occurrenceByMsgID[messageID] = occ
	}

	if len(auctionIDs) == 0 {
		return map[uuid.UUID]*chatApp.ResourceProjection{}, nil
	}

	var result map[uuid.UUID]*chatApp.ResourceProjection
	err := r.db.WithTx(ctx, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `
			WITH requested(id) AS (
				SELECT unnest($1::uuid[])
			)
			SELECT
				a.id,
				a.seller_id,
				a.product_id,
				a.order_id,
				a.start_price,
				a.bid_increment,
				a.buy_now_price,
				a.start_at,
				a.end_at,
				a.current_bid,
				a.current_winner_id,
				a.status,
				a.created_at,
				a.updated_at,
				p.title AS product_title,
				COALESCE(p.media_urls, '[]'::jsonb) AS product_media_urls,
				p.variety,
				p.size_cm,
				p.age_months,
				p.gender,
				p.breeder,
				p.bloodline,
				p.preparation_time AS product_preparation_time,
				p.preparation_note AS product_preparation_note
			FROM requested r
			JOIN auctions a ON a.id = r.id
			JOIN products p ON p.id = a.product_id
		`, auctionIDs)
		if err != nil {
			return fmt.Errorf("chat: auction source batch query failed: %w", err)
		}
		defer rows.Close()

		byID := make(map[uuid.UUID]auctionProjectionSourceRow, len(auctionIDs))
		sellerIDs := make([]uuid.UUID, 0, len(auctionIDs))
		seenSellerIDs := make(map[uuid.UUID]struct{}, len(auctionIDs))
		for rows.Next() {
			var row auctionProjectionSourceRow
			if err := rows.Scan(
				&row.auctionID,
				&row.sellerID,
				&row.productID,
				&row.orderID,
				&row.startPrice,
				&row.bidIncrement,
				&row.buyNowPrice,
				&row.startAt,
				&row.endAt,
				&row.currentBid,
				&row.currentWinnerID,
				&row.status,
				&row.createdAt,
				&row.updatedAt,
				&row.productTitle,
				&row.productMediaURLs,
				&row.productVariety,
				&row.productSizeCm,
				&row.productAgeMonths,
				&row.productGender,
				&row.productBreeder,
				&row.productBloodline,
				&row.productPreparation,
				&row.productNote,
			); err != nil {
				return fmt.Errorf("chat: auction source batch scan failed: %w", err)
			}
			byID[row.auctionID] = row
			if row.sellerID == uuid.Nil {
				return fmt.Errorf("chat: auction source row missing seller id for %s", row.auctionID)
			}
			if _, seen := seenSellerIDs[row.sellerID]; !seen {
				sellerIDs = append(sellerIDs, row.sellerID)
				seenSellerIDs[row.sellerID] = struct{}{}
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("chat: auction source batch iteration failed: %w", err)
		}

		for _, auctionID := range auctionIDs {
			if _, ok := byID[auctionID]; !ok {
				return fmt.Errorf("chat: auction source row missing for %s", auctionID)
			}
		}

		sellerRows, err := loadAuctionProjectionSellerRows(ctx, tx, sellerIDs)
		if err != nil {
			return err
		}

		blockedSet := map[uuid.UUID]bool{}
		if viewerID != uuid.Nil && len(sellerIDs) > 0 {
			blockedSet, err = blockcheck.BlockedSet(ctx, tx, viewerID, sellerIDs)
			if err != nil {
				return fmt.Errorf("chat: auction block batch query failed: %w", err)
			}
		}

		result = make(map[uuid.UUID]*chatApp.ResourceProjection, len(occurrenceByMsgID))
		for messageID, occ := range occurrenceByMsgID {
			sourceID := occ.SourceID()
			row := byID[sourceID]
			sellerRow, ok := sellerRows[row.sellerID]
			if !ok || !sellerRow.found {
				return fmt.Errorf("chat: auction seller row missing for %s", row.sellerID)
			}

			blocked := viewerID != uuid.Nil && viewerID != row.sellerID && blockedSet[row.sellerID]
			if !commerceshared.EvaluateAuctionViewAccess(commerceshared.AuctionViewAccessInput{
				ViewerID: viewerID,
				SellerID: row.sellerID,
				Status:   row.status,
				Blocked:  blocked,
				Seller: commerceshared.SellerAccessSnapshot{
					AccountStatus:      sellerRow.accountStatus,
					IsDeleted:          sellerRow.isDeleted,
					SubscriptionStatus: sellerRow.subscriptionStatus,
				},
			}) {
				proj, projErr := chatApp.NewTombstoneProjection(chatEntity.ResourceOccurrenceResourceTypeAuction)
				if projErr != nil {
					return projErr
				}
				result[messageID] = &proj
				continue
			}

			auctionCaps := commerceshared.EvaluateAuctionViewerCapabilities(commerceshared.AuctionViewerCapabilitiesInput{
				ViewerID:          viewerID,
				SellerID:          row.sellerID,
				Status:            row.status,
				SellerTrustActive: viewercontext.CoarsenSellerTrust(sellerRow.subscriptionStatus) == viewercontext.PublicLifecycleStateActive,
				BuyNowPrice:       row.buyNowPrice,
			})
			commerceActions := buildAuctionCommerceActions(auctionCaps)
			viewerCaps := chatApp.ProjectionViewerCapabilities{
				CanView:            true,
				CanInteract:        commerceActions.CanBid || commerceActions.CanBuy,
				BlockedByTombstone: false,
			}
			sellerCard := buildAuctionLiveSellerCard(row.sellerID, sellerRow)
			thumbnail := firstResolvedAuctionURLFromJSONStrings(row.productMediaURLs)
			lifecycle := auctionEntity.Status(row.status).PublicLifecycle()

			proj, projErr := chatApp.NewLiveProjection(
				chatEntity.ResourceOccurrenceResourceTypeAuction,
				sourceID,
				chatApp.AuctionLivePayload{
					Title:       row.productTitle,
					Thumbnail:   thumbnail,
					CurrentBid:  row.currentBid,
					BuyNowPrice: row.buyNowPrice,
					EndAt:       row.endAt.Format(time.RFC3339),
					Lifecycle:   &lifecycle,
					Seller:      sellerCard,
				},
				viewerCaps,
				&commerceActions,
			)
			if projErr != nil {
				return projErr
			}
			result[messageID] = &proj
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func buildAuctionLiveSellerCard(
	sellerID uuid.UUID,
	row auctionProjectionSellerRow,
) *publiccard.SellerCard {
	if sellerID == uuid.Nil {
		return nil
	}

	userLifecycle := viewercontext.CoarsenLifecycle(row.accountStatus, row.isDeleted)
	user := publiccard.UserCard{ID: sellerID}

	switch userLifecycle {
	case viewercontext.PublicLifecycleStateActive:
		trimmedUsername := strings.TrimSpace(row.username)
		if trimmedUsername == "" {
			user.Lifecycle = lifecyclePtr(string(viewercontext.PublicLifecycleStateUnavailable))
			break
		}
		user.Username = trimmedUsername
		if trimmedAvatar := strings.TrimSpace(row.avatarURL); trimmedAvatar != "" {
			user.AvatarURL = &trimmedAvatar
		}
		user.Lifecycle = lifecyclePtr(string(viewercontext.PublicLifecycleStateActive))
	case viewercontext.PublicLifecycleStateRemoved:
		user.Lifecycle = lifecyclePtr(string(viewercontext.PublicLifecycleStateRemoved))
	default:
		user.Lifecycle = lifecyclePtr(string(viewercontext.PublicLifecycleStateUnavailable))
	}

	seller := publiccard.SellerCard{
		User:      user,
		AvatarURL: user.AvatarURL,
	}
	if farmName := strings.TrimSpace(row.farmName); farmName != "" {
		seller.FarmName = &farmName
	}

	trustLifecycle := string(viewercontext.CoarsenSellerTrust(row.subscriptionStatus))
	if trustLifecycle != "" {
		seller.Lifecycle = lifecyclePtr(trustLifecycle)
	}

	userLifecycleValue := ""
	if user.Lifecycle != nil {
		userLifecycleValue = *user.Lifecycle
	}
	seller.Tier = publiccard.GatedSellerTier(strings.TrimSpace(row.tier), userLifecycleValue, trustLifecycle)
	return &seller
}

func lifecyclePtr(value string) *string {
	if value == "" {
		return nil
	}
	v := value
	return &v
}

func loadAuctionProjectionSellerRows(
	ctx context.Context,
	tx db.Tx,
	sellerIDs []uuid.UUID,
) (map[uuid.UUID]auctionProjectionSellerRow, error) {
	out := make(map[uuid.UUID]auctionProjectionSellerRow, len(sellerIDs))
	if len(sellerIDs) == 0 {
		return out, nil
	}

	rows, err := tx.Query(ctx, `
		SELECT
			r.id,
			(u.id IS NOT NULL) AS user_found,
			COALESCE(up.username, '') AS username,
			COALESCE(sp.store_name, '') AS farm_name,
			COALESCE(sp.store_image_url, '') AS store_image_url,
			COALESCE(up.avatar_url, '') AS avatar_url,
			COALESCE(u.account_status::text, '') AS account_status,
			(u.deleted_at IS NOT NULL) AS is_deleted,
			COALESCE(ss.status::text, '') AS subscription_status,
			COALESCE(sp.tier::text, '') AS tier
		FROM (SELECT unnest($1::uuid[]) AS id) r
		LEFT JOIN users u ON u.id = r.id
		LEFT JOIN user_profiles up ON up.user_id = u.id
		LEFT JOIN seller_profiles sp ON sp.user_id = u.id
		LEFT JOIN LATERAL (
			SELECT status
			FROM seller_subscriptions
			WHERE user_id = u.id
			ORDER BY created_at DESC
			LIMIT 1
		) ss ON true
	`, sellerIDs)
	if err != nil {
		return nil, fmt.Errorf("chat: auction seller batch query failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var row auctionProjectionSellerRow
		var rowKey uuid.UUID
		if err := rows.Scan(
			&rowKey,
			&row.found,
			&row.username,
			&row.farmName,
			&row.storeImageURL,
			&row.avatarURL,
			&row.accountStatus,
			&row.isDeleted,
			&row.subscriptionStatus,
			&row.tier,
		); err != nil {
			return nil, fmt.Errorf("chat: auction seller batch scan failed: %w", err)
		}
		out[rowKey] = row
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chat: auction seller batch iteration failed: %w", err)
	}

	for _, sellerID := range sellerIDs {
		if row, ok := out[sellerID]; !ok || !row.found {
			return nil, fmt.Errorf("chat: auction seller row missing for %s", sellerID)
		}
	}

	return out, nil
}

func buildAuctionCommerceActions(caps commerceshared.ViewerCapabilities) chatApp.CommerceActionCapabilities {
	return chatApp.CommerceActionCapabilities{
		Role:         caps.Role,
		CanChat:      caps.CanChat,
		CanNegotiate: false,
		CanBuy:       caps.CanBuyNow,
		CanBid:       caps.CanBid,
		CanManage:    caps.CanManage,
	}
}

func resolveReadableAuctionMediaReference(value string) string {
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

func firstResolvedAuctionURLFromJSONStrings(raw json.RawMessage) *string {
	for _, value := range decodeJSONStringSlice(raw) {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			resolved := resolveReadableAuctionMediaReference(trimmed)
			if resolved != "" {
				return &resolved
			}
		}
	}
	return nil
}
