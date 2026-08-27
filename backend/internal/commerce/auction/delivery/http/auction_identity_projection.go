package http

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/auction/entity"
	"github.com/labuda/backend/internal/governance/viewercontext"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/pkg/db"
)

type auctionIdentityProjectionRow struct {
	ID                 uuid.UUID
	UserFound          bool
	Username           string
	AvatarURL          string
	FarmName           string
	AccountStatus      string
	IsDeleted          bool
	SubscriptionStatus string
	Tier               string
}

func hydrateAuctionSellerCards(
	ctx context.Context,
	tx db.Tx,
	sellerIDs []uuid.UUID,
) (map[uuid.UUID]publiccard.SellerCard, error) {
	rows, err := fetchAuctionIdentityRows(ctx, tx, sellerIDs)
	if err != nil {
		return nil, err
	}

	out := make(map[uuid.UUID]publiccard.SellerCard, len(rows))
	for id, row := range rows {
		out[id] = projectAuctionSellerCard(row)
	}
	return out, nil
}

func hydrateAuctionSellerCard(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
) (publiccard.SellerCard, error) {
	if sellerID == uuid.Nil {
		return blankAuctionSellerCard(sellerID), nil
	}
	cards, err := hydrateAuctionSellerCards(ctx, tx, []uuid.UUID{sellerID})
	if err != nil {
		return publiccard.SellerCard{}, err
	}
	if card, ok := cards[sellerID]; ok {
		return card, nil
	}
	return blankAuctionSellerCard(sellerID), nil
}

func hydrateAuctionBidderLifecycleCards(
	ctx context.Context,
	tx db.Tx,
	bids []*entity.AuctionBid,
) (map[uuid.UUID]publiccard.UserCard, error) {
	ids := make([]uuid.UUID, 0, len(bids))
	seen := make(map[uuid.UUID]struct{}, len(bids))
	for _, bid := range bids {
		if bid == nil || bid.BidderID == uuid.Nil {
			continue
		}
		if _, ok := seen[bid.BidderID]; ok {
			continue
		}
		seen[bid.BidderID] = struct{}{}
		ids = append(ids, bid.BidderID)
	}

	rows, err := fetchAuctionIdentityRows(ctx, tx, ids)
	if err != nil {
		return nil, err
	}

	out := make(map[uuid.UUID]publiccard.UserCard, len(rows))
	for id, row := range rows {
		out[id] = projectAuctionBidderCard(row)
	}
	return out, nil
}

func hydrateAuctionBidderCard(
	ctx context.Context,
	tx db.Tx,
	bidderID uuid.UUID,
) (publiccard.UserCard, error) {
	if bidderID == uuid.Nil {
		return publiccard.UserCard{}, nil
	}
	rows, err := fetchAuctionIdentityRows(ctx, tx, []uuid.UUID{bidderID})
	if err != nil {
		return publiccard.UserCard{}, err
	}
	if row, ok := rows[bidderID]; ok {
		return projectAuctionBidderCard(row), nil
	}
	return publiccard.UserCard{ID: bidderID}, nil
}

func fetchAuctionIdentityRows(
	ctx context.Context,
	tx db.Tx,
	ids []uuid.UUID,
) (map[uuid.UUID]auctionIdentityProjectionRow, error) {
	out := make(map[uuid.UUID]auctionIdentityProjectionRow, len(ids))
	if tx == nil || len(ids) == 0 {
		return out, nil
	}

	deduped := dedupeAuctionIdentityIDs(ids)
	if len(deduped) == 0 {
		return out, nil
	}

	const q = `
		WITH requested(id) AS (
			SELECT unnest($1::uuid[])
		)
		SELECT
			r.id,
			(u.id IS NOT NULL) AS user_found,
			COALESCE(up.username, '')   AS username,
			COALESCE(up.avatar_url, '') AS avatar_url,
			COALESCE(sp.store_name, '') AS farm_name,
			COALESCE(u.account_status::text, '') AS account_status,
			COALESCE((u.deleted_at IS NOT NULL), false) AS is_deleted,
			COALESCE(ss.status::text, '') AS subscription_status,
			COALESCE(sp.tier::text, '') AS tier
		FROM requested r
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
	`

	rows, err := tx.Query(ctx, q, deduped)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var row auctionIdentityProjectionRow
		if err := rows.Scan(
			&row.ID,
			&row.UserFound,
			&row.Username,
			&row.AvatarURL,
			&row.FarmName,
			&row.AccountStatus,
			&row.IsDeleted,
			&row.SubscriptionStatus,
			&row.Tier,
		); err != nil {
			return nil, err
		}
		out[row.ID] = row
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func projectAuctionSellerCard(row auctionIdentityProjectionRow) publiccard.SellerCard {
	user := projectAuctionBidderCard(row)
	card := publiccard.SellerCard{
		User:      user,
		AvatarURL: user.AvatarURL,
	}
	if farmName := strings.TrimSpace(row.FarmName); farmName != "" {
		card.FarmName = &farmName
	}

	userLifecycle := ""
	if user.Lifecycle != nil {
		userLifecycle = *user.Lifecycle
	}
	trustLifecycle := string(viewercontext.CoarsenSellerTrust(row.SubscriptionStatus))
	if trustLifecycle != "" {
		card.Lifecycle = lifecyclePtr(trustLifecycle)
	}
	card.Tier = publiccard.GatedSellerTier(strings.TrimSpace(row.Tier), userLifecycle, trustLifecycle)
	return card
}

func projectAuctionBidderCard(row auctionIdentityProjectionRow) publiccard.UserCard {
	return projectAuctionUserCard(row.ID, row.UserFound, row.Username, row.AvatarURL, row.AccountStatus, row.IsDeleted)
}

func projectAuctionUserCard(
	id uuid.UUID,
	userFound bool,
	username string,
	avatarURL string,
	accountStatus string,
	isDeleted bool,
) publiccard.UserCard {
	if id == uuid.Nil {
		return publiccard.UserCard{}
	}

	lifecycle := viewercontext.PublicLifecycleStateRemoved
	if userFound {
		lifecycle = viewercontext.CoarsenLifecycle(accountStatus, isDeleted)
	}

	switch lifecycle {
	case viewercontext.PublicLifecycleStateActive:
		trimmedUsername := strings.TrimSpace(username)
		if trimmedUsername == "" {
			return redactedAuctionUserCard(id, viewercontext.PublicLifecycleStateUnavailable)
		}
		card := publiccard.UserCard{
			ID:       id,
			Username: trimmedUsername,
		}
		if trimmedAvatar := strings.TrimSpace(avatarURL); trimmedAvatar != "" {
			card.AvatarURL = &trimmedAvatar
		}
		card.Lifecycle = lifecyclePtr(string(viewercontext.PublicLifecycleStateActive))
		return card
	case viewercontext.PublicLifecycleStateRemoved:
		return redactedAuctionUserCard(id, viewercontext.PublicLifecycleStateRemoved)
	default:
		return redactedAuctionUserCard(id, viewercontext.PublicLifecycleStateUnavailable)
	}
}

func redactedAuctionUserCard(id uuid.UUID, lifecycle viewercontext.PublicLifecycleState) publiccard.UserCard {
	card := publiccard.UserCard{ID: id}
	if lifecycle != "" {
		card.Lifecycle = lifecyclePtr(string(lifecycle))
	}
	return card
}

func blankAuctionSellerCard(id uuid.UUID) publiccard.SellerCard {
	return publiccard.SellerCard{
		User: publiccard.UserCard{ID: id},
	}
}

func dedupeAuctionIdentityIDs(ids []uuid.UUID) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ids))
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func auctionBidderIDs(bids []*entity.AuctionBid) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(bids))
	seen := make(map[uuid.UUID]struct{}, len(bids))
	for _, bid := range bids {
		if bid == nil || bid.BidderID == uuid.Nil {
			continue
		}
		if _, ok := seen[bid.BidderID]; ok {
			continue
		}
		seen[bid.BidderID] = struct{}{}
		ids = append(ids, bid.BidderID)
	}
	return ids
}

func lifecyclePtr(value string) *string {
	if value == "" {
		return nil
	}
	v := value
	return &v
}
