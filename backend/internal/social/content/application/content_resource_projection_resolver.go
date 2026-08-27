package application

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	auctionentity "github.com/labuda/backend/internal/commerce/auction/entity"
	fpsentity "github.com/labuda/backend/internal/commerce/forsale/entity"
	"github.com/labuda/backend/internal/governance/viewercontext"
	"github.com/labuda/backend/internal/pkg/blockcheck"
	"github.com/labuda/backend/internal/pkg/publiccard"
	contententity "github.com/labuda/backend/internal/social/content/entity"
	"github.com/labuda/backend/pkg/db"
)

// ContentResourceProjectionResolver resolves canonical content resource
// occurrences into viewer-aware projection envelopes.
type ContentResourceProjectionResolver struct{}

// NewContentResourceProjectionResolver creates a stateless resolver.
func NewContentResourceProjectionResolver() *ContentResourceProjectionResolver {
	return &ContentResourceProjectionResolver{}
}

// ResolveContentResourceProjection resolves a single content row.
func (r *ContentResourceProjectionResolver) ResolveContentResourceProjection(
	ctx context.Context,
	tx db.Tx,
	viewerID uuid.UUID,
	contentID uuid.UUID,
) (*ContentResourceProjection, error) {
	m, err := r.ResolveContentResourceProjections(ctx, tx, viewerID, []uuid.UUID{contentID})
	if err != nil {
		return nil, err
	}
	if proj, ok := m[contentID]; ok {
		return proj, nil
	}
	return nil, nil
}

// ResolveContentResourceProjections resolves a batch of content rows keyed by
// content_id. Rows without a canonical occurrence are omitted.
func (r *ContentResourceProjectionResolver) ResolveContentResourceProjections(
	ctx context.Context,
	tx db.Tx,
	viewerID uuid.UUID,
	contentIDs []uuid.UUID,
) (map[uuid.UUID]*ContentResourceProjection, error) {
	if len(contentIDs) == 0 {
		return map[uuid.UUID]*ContentResourceProjection{}, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("content: projection resolver requires transaction")
	}

	occurrences, err := r.loadOccurrences(ctx, tx, contentIDs)
	if err != nil {
		return nil, err
	}
	if len(occurrences) == 0 {
		return map[uuid.UUID]*ContentResourceProjection{}, nil
	}

	profileTargets := make([]uuid.UUID, 0, len(occurrences))
	contentTargets := make([]uuid.UUID, 0, len(occurrences))
	fpsTargets := make([]uuid.UUID, 0, len(occurrences))
	auctionTargets := make([]uuid.UUID, 0, len(occurrences))
	for _, occ := range occurrences {
		switch occ.ResourceType() {
		case contententity.ContentResourceOccurrenceResourceTypeProfile:
			profileTargets = append(profileTargets, occ.SourceID())
		case contententity.ContentResourceOccurrenceResourceTypeContent:
			contentTargets = append(contentTargets, occ.SourceID())
		case contententity.ContentResourceOccurrenceResourceTypeForSale:
			fpsTargets = append(fpsTargets, occ.SourceID())
		case contententity.ContentResourceOccurrenceResourceTypeAuction:
			auctionTargets = append(auctionTargets, occ.SourceID())
		default:
			return nil, fmt.Errorf("content: unsupported resource type %q", occ.ResourceType())
		}
	}

	profileRows, err := r.loadProfileTargets(ctx, tx, profileTargets)
	if err != nil {
		return nil, err
	}
	contentRows, err := r.loadContentTargets(ctx, tx, contentTargets)
	if err != nil {
		return nil, err
	}
	fpsRows, err := r.loadForSales(ctx, tx, fpsTargets)
	if err != nil {
		return nil, err
	}
	auctionRows, err := r.loadAuctions(ctx, tx, auctionTargets)
	if err != nil {
		return nil, err
	}

	blockedProfiles, err := blockcheck.BlockedSet(ctx, tx, viewerID, profileTargets)
	if err != nil {
		return nil, fmt.Errorf("content: profile block batch failed: %w", err)
	}
	blockedContentAuthors, err := blockcheck.BlockedSet(ctx, tx, viewerID, contentAuthorIDs(contentRows))
	if err != nil {
		return nil, fmt.Errorf("content: content block batch failed: %w", err)
	}
	blockedSellers, err := blockcheck.BlockedSet(ctx, tx, viewerID, sellerIDsForCommerce(fpsRows, auctionRows))
	if err != nil {
		return nil, fmt.Errorf("content: commerce block batch failed: %w", err)
	}
	followedAuthors, err := followedAuthorSet(ctx, tx, viewerID, contentAuthorIDs(contentRows))
	if err != nil {
		return nil, fmt.Errorf("content: follow batch failed: %w", err)
	}

	result := make(map[uuid.UUID]*ContentResourceProjection, len(occurrences))
	for contentID, occ := range occurrences {
		switch occ.ResourceType() {
		case contententity.ContentResourceOccurrenceResourceTypeProfile:
			row, ok := profileRows[occ.SourceID()]
			if !ok || !row.isLive {
				proj, err := NewTombstoneContentResourceProjection(contententity.ContentResourceOccurrenceResourceTypeProfile, occ.SourceID())
				if err != nil {
					return nil, err
				}
				result[contentID] = &proj
				continue
			}
			if viewerID != uuid.Nil && blockedProfiles[occ.SourceID()] {
				proj, err := NewTombstoneContentResourceProjection(contententity.ContentResourceOccurrenceResourceTypeProfile, occ.SourceID())
				if err != nil {
					return nil, err
				}
				result[contentID] = &proj
				continue
			}
			payload := ProfileLivePayload{
				Username:  row.username,
				AvatarURL: row.avatarURL,
				Lifecycle: string(viewercontext.CoarsenLifecycle(row.accountStatus, row.deletedAt.Valid)),
			}
			proj, err := NewLiveContentResourceProjection(contententity.ContentResourceOccurrenceResourceTypeProfile, occ.SourceID(), payload)
			if err != nil {
				return nil, err
			}
			result[contentID] = &proj

		case contententity.ContentResourceOccurrenceResourceTypeContent:
			row, ok := contentRows[occ.SourceID()]
			if !ok || !row.isLive(viewerID, blockedContentAuthors, followedAuthors) {
				proj, err := NewTombstoneContentResourceProjection(contententity.ContentResourceOccurrenceResourceTypeContent, occ.SourceID())
				if err != nil {
					return nil, err
				}
				result[contentID] = &proj
				continue
			}
			payload := ContentLivePayload{
				Caption:   row.caption,
				Media:     resolveMediaRefs(row.mediaURLs),
				Lifecycle: contententity.PublicLifecycleFromString(row.status),
				CreatedAt: row.createdAt.Format(time.RFC3339),
				Author:    buildPublicUserCard(row.authorID, row.username, row.avatarURL, row.accountStatus, row.authorDeleted),
			}
			if row.nestedOccurrence != nil {
				payload.NestedResource = row.nestedOccurrence
			}
			proj, err := NewLiveContentResourceProjection(contententity.ContentResourceOccurrenceResourceTypeContent, occ.SourceID(), payload)
			if err != nil {
				return nil, err
			}
			result[contentID] = &proj

		case contententity.ContentResourceOccurrenceResourceTypeForSale:
			row, ok := fpsRows[occ.SourceID()]
			if !ok || !row.isLive {
				proj, err := NewTombstoneContentResourceProjection(contententity.ContentResourceOccurrenceResourceTypeForSale, occ.SourceID())
				if err != nil {
					return nil, err
				}
				result[contentID] = &proj
				continue
			}
			if viewerID != uuid.Nil && blockedSellers[row.sellerID] {
				proj, err := NewTombstoneContentResourceProjection(contententity.ContentResourceOccurrenceResourceTypeForSale, occ.SourceID())
				if err != nil {
					return nil, err
				}
				result[contentID] = &proj
				continue
			}
			sellerCard := buildSellerCard(row.sellerID, row.username, row.avatarURL, row.storeName, row.accountStatus, row.authorDeleted, row.subscriptionStatus, row.tier)
			payload := ForSaleLivePayload{
				Title:             row.title,
				Media:             resolveMediaRefs(row.mediaURLs),
				ThumbnailURL:      firstResolvedMediaURL(row.mediaURLs),
				Price:             row.price,
				Status:            row.status,
				QuantityAvailable: row.quantityAvailable,
				CanInteract:       row.status == string(fpsentity.ForSaleStatusActive) && row.quantityAvailable > 0,
				Seller:            sellerCard,
			}
			proj, err := NewLiveContentResourceProjection(contententity.ContentResourceOccurrenceResourceTypeForSale, occ.SourceID(), payload)
			if err != nil {
				return nil, err
			}
			result[contentID] = &proj

		case contententity.ContentResourceOccurrenceResourceTypeAuction:
			row, ok := auctionRows[occ.SourceID()]
			if !ok || !row.isLive {
				proj, err := NewTombstoneContentResourceProjection(contententity.ContentResourceOccurrenceResourceTypeAuction, occ.SourceID())
				if err != nil {
					return nil, err
				}
				result[contentID] = &proj
				continue
			}
			if viewerID != uuid.Nil && blockedSellers[row.sellerID] {
				proj, err := NewTombstoneContentResourceProjection(contententity.ContentResourceOccurrenceResourceTypeAuction, occ.SourceID())
				if err != nil {
					return nil, err
				}
				result[contentID] = &proj
				continue
			}
			sellerCard := buildSellerCard(row.sellerID, row.username, row.avatarURL, row.storeName, row.accountStatus, row.authorDeleted, row.subscriptionStatus, row.tier)
			payload := AuctionLivePayload{
				Title:        row.title,
				Media:        resolveMediaRefs(row.mediaURLs),
				ThumbnailURL: firstResolvedMediaURL(row.mediaURLs),
				CurrentBid:   row.currentBid,
				BuyNowPrice:  row.buyNowPrice,
				EndAt:        row.endAt.Format(time.RFC3339),
				Lifecycle:    auctionentity.Status(row.status).PublicLifecycle(),
				CanInteract:  row.status == string(auctionentity.StatusActive),
				Seller:       sellerCard,
			}
			proj, err := NewLiveContentResourceProjection(contententity.ContentResourceOccurrenceResourceTypeAuction, occ.SourceID(), payload)
			if err != nil {
				return nil, err
			}
			result[contentID] = &proj

		default:
			return nil, fmt.Errorf("content: unsupported resource type %q", occ.ResourceType())
		}
	}

	return result, nil
}

type contentOccurrenceRow struct {
	contentID       uuid.UUID
	resourceType    string
	profileSourceID *uuid.UUID
	contentSourceID *uuid.UUID
	fpsSourceID     *uuid.UUID
	auctionSourceID *uuid.UUID
}

func (r *contentOccurrenceRow) ResourceType() contententity.ContentResourceOccurrenceResourceType {
	switch {
	case r.profileSourceID != nil:
		return contententity.ContentResourceOccurrenceResourceTypeProfile
	case r.contentSourceID != nil:
		return contententity.ContentResourceOccurrenceResourceTypeContent
	case r.fpsSourceID != nil:
		return contententity.ContentResourceOccurrenceResourceTypeForSale
	case r.auctionSourceID != nil:
		return contententity.ContentResourceOccurrenceResourceTypeAuction
	default:
		return ""
	}
}

func (r *contentOccurrenceRow) SourceID() uuid.UUID {
	switch {
	case r.profileSourceID != nil:
		return *r.profileSourceID
	case r.contentSourceID != nil:
		return *r.contentSourceID
	case r.fpsSourceID != nil:
		return *r.fpsSourceID
	case r.auctionSourceID != nil:
		return *r.auctionSourceID
	default:
		return uuid.Nil
	}
}

func (r *ContentResourceProjectionResolver) loadOccurrences(
	ctx context.Context,
	tx db.Tx,
	contentIDs []uuid.UUID,
) (map[uuid.UUID]*contentOccurrenceRow, error) {
	rows, err := tx.Query(ctx, `
		SELECT content_id, operation,
		       profile_source_id, content_source_id,
		       for_sale_source_id, auction_source_id
		FROM content_resource_occurrences
		WHERE content_id = ANY($1)
	`, contentIDs)
	if err != nil {
		return nil, fmt.Errorf("content: load occurrences failed: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]*contentOccurrenceRow, len(contentIDs))
	for rows.Next() {
		var row contentOccurrenceRow
		if err := rows.Scan(&row.contentID, &row.resourceType, &row.profileSourceID, &row.contentSourceID, &row.fpsSourceID, &row.auctionSourceID); err != nil {
			return nil, fmt.Errorf("content: scan occurrence failed: %w", err)
		}
		result[row.contentID] = &row
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("content: iterate occurrences failed: %w", err)
	}
	return result, nil
}

type profileTargetRow struct {
	username      string
	avatarURL     *string
	accountStatus string
	deletedAt     sql.NullTime
	isLive        bool
}

type contentTargetRow struct {
	caption          *string
	status           string
	visibility       string
	isHidden         bool
	deletedAt        sql.NullTime
	createdAt        time.Time
	authorID         uuid.UUID
	username         string
	avatarURL        *string
	accountStatus    string
	authorDeleted    bool
	mediaURLs        []string
	nestedOccurrence *NestedResourceIndicator
}

func (r contentTargetRow) isLive(viewerID uuid.UUID, blockedAuthors map[uuid.UUID]bool, followedAuthors map[uuid.UUID]bool) bool {
	if r.isHidden || r.deletedAt.Valid {
		return false
	}
	if viewerID != uuid.Nil && blockedAuthors[r.authorID] {
		return false
	}
	if viewerID != uuid.Nil && viewerID == r.authorID {
		return true
	}
	if viewercontext.CoarsenLifecycle(r.accountStatus, r.authorDeleted) != viewercontext.PublicLifecycleStateActive {
		return false
	}
	switch contententity.Visibility(r.visibility) {
	case contententity.VisibilityPublic:
		return true
	case contententity.VisibilityFollowersOnly:
		return viewerID != uuid.Nil && followedAuthors[r.authorID]
	default:
		return false
	}
}

type fpsTargetRow struct {
	sellerID           uuid.UUID
	title              string
	price              int64
	status             string
	quantityAvailable  int
	username           string
	avatarURL          *string
	storeName          string
	accountStatus      string
	authorDeleted      bool
	subscriptionStatus string
	tier               string
	mediaURLs          []string
	isLive             bool
}

type auctionTargetRow struct {
	sellerID           uuid.UUID
	title              string
	currentBid         *int64
	buyNowPrice        *int64
	endAt              time.Time
	status             string
	username           string
	avatarURL          *string
	storeName          string
	accountStatus      string
	authorDeleted      bool
	subscriptionStatus string
	tier               string
	mediaURLs          []string
	isLive             bool
}

func (r *ContentResourceProjectionResolver) loadProfileTargets(
	ctx context.Context,
	tx db.Tx,
	targetIDs []uuid.UUID,
) (map[uuid.UUID]profileTargetRow, error) {
	if len(targetIDs) == 0 {
		return map[uuid.UUID]profileTargetRow{}, nil
	}

	rows, err := tx.Query(ctx, `
		SELECT u.id,
		       COALESCE(up.username, '') AS username,
		       up.avatar_url,
		       u.account_status,
		       u.deleted_at
		FROM users u
		LEFT JOIN user_profiles up ON up.user_id = u.id
		WHERE u.id = ANY($1)
	`, targetIDs)
	if err != nil {
		return nil, fmt.Errorf("content: load profile targets failed: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]profileTargetRow, len(targetIDs))
	for rows.Next() {
		var (
			id            uuid.UUID
			username      string
			avatarURL     *string
			accountStatus string
			deletedAt     sql.NullTime
		)
		if err := rows.Scan(&id, &username, &avatarURL, &accountStatus, &deletedAt); err != nil {
			return nil, fmt.Errorf("content: scan profile target failed: %w", err)
		}
		result[id] = profileTargetRow{
			username:      username,
			avatarURL:     avatarURL,
			accountStatus: accountStatus,
			deletedAt:     deletedAt,
			isLive:        viewercontext.CoarsenLifecycle(accountStatus, deletedAt.Valid) == viewercontext.PublicLifecycleStateActive,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("content: iterate profile targets failed: %w", err)
	}
	return result, nil
}

func (r *ContentResourceProjectionResolver) loadContentTargets(
	ctx context.Context,
	tx db.Tx,
	targetIDs []uuid.UUID,
) (map[uuid.UUID]contentTargetRow, error) {
	if len(targetIDs) == 0 {
		return map[uuid.UUID]contentTargetRow{}, nil
	}

	rows, err := tx.Query(ctx, `
		SELECT c.id,
		       c.caption,
		       c.visibility,
		       c.is_hidden,
		       c.status,
		       c.deleted_at,
		       c.created_at,
		       c.author_id,
		       COALESCE(up.username, '') AS username,
		       up.avatar_url,
		       u.account_status,
		       (u.deleted_at IS NOT NULL) AS is_deleted,
		       COALESCE(array_agg(cm.media_url ORDER BY cm.position) FILTER (WHERE cm.media_url IS NOT NULL), ARRAY[]::text[]) AS media_urls,
		       nested.profile_source_id,
		       nested.content_source_id,
		       nested.for_sale_source_id,
		       nested.auction_source_id
		FROM contents c
		JOIN users u ON u.id = c.author_id
		LEFT JOIN user_profiles up ON up.user_id = u.id
		LEFT JOIN content_media cm ON cm.content_id = c.id
		LEFT JOIN content_resource_occurrences nested ON nested.content_id = c.id
		WHERE c.id = ANY($1)
		GROUP BY c.id, c.caption, c.visibility, c.is_hidden, c.status, c.deleted_at, c.created_at,
		         c.author_id, up.username, up.avatar_url, u.account_status, u.deleted_at,
		         nested.profile_source_id, nested.content_source_id,
		         nested.for_sale_source_id, nested.auction_source_id
	`, targetIDs)
	if err != nil {
		return nil, fmt.Errorf("content: load content targets failed: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]contentTargetRow, len(targetIDs))
	for rows.Next() {
		var (
			id              uuid.UUID
			caption         *string
			visibility      string
			isHidden        bool
			status          string
			deletedAt       sql.NullTime
			createdAt       time.Time
			authorID        uuid.UUID
			username        string
			avatarURL       *string
			accountStatus   string
			authorDeleted   bool
			mediaURLs       []string
			profileSourceID *uuid.UUID
			contentSourceID *uuid.UUID
			fpsSourceID     *uuid.UUID
			auctionSourceID *uuid.UUID
		)
		if err := rows.Scan(&id, &caption, &visibility, &isHidden, &status, &deletedAt, &createdAt, &authorID, &username, &avatarURL, &accountStatus, &authorDeleted, &mediaURLs, &profileSourceID, &contentSourceID, &fpsSourceID, &auctionSourceID); err != nil {
			return nil, fmt.Errorf("content: scan content target failed: %w", err)
		}
		var nested *NestedResourceIndicator
		switch {
		case profileSourceID != nil:
			nested = &NestedResourceIndicator{ResourceType: contententity.ContentResourceOccurrenceResourceTypeProfile, ResourceID: *profileSourceID}
		case contentSourceID != nil:
			nested = &NestedResourceIndicator{ResourceType: contententity.ContentResourceOccurrenceResourceTypeContent, ResourceID: *contentSourceID}
		case fpsSourceID != nil:
			nested = &NestedResourceIndicator{ResourceType: contententity.ContentResourceOccurrenceResourceTypeForSale, ResourceID: *fpsSourceID}
		case auctionSourceID != nil:
			nested = &NestedResourceIndicator{ResourceType: contententity.ContentResourceOccurrenceResourceTypeAuction, ResourceID: *auctionSourceID}
		}
		result[id] = contentTargetRow{
			caption:          caption,
			status:           status,
			visibility:       visibility,
			isHidden:         isHidden,
			deletedAt:        deletedAt,
			createdAt:        createdAt,
			authorID:         authorID,
			username:         username,
			avatarURL:        avatarURL,
			accountStatus:    accountStatus,
			authorDeleted:    authorDeleted,
			mediaURLs:        mediaURLs,
			nestedOccurrence: nested,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("content: iterate content targets failed: %w", err)
	}
	return result, nil
}

func (r *ContentResourceProjectionResolver) loadForSales(
	ctx context.Context,
	tx db.Tx,
	targetIDs []uuid.UUID,
) (map[uuid.UUID]fpsTargetRow, error) {
	if len(targetIDs) == 0 {
		return map[uuid.UUID]fpsTargetRow{}, nil
	}

	rows, err := tx.Query(ctx, `
		SELECT fps.id,
		       fps.seller_id,
		       p.title,
		       fps.price_per_unit,
		       fps.status,
		       fps.quantity_available,
		       COALESCE(up.username, '') AS username,
		       up.avatar_url,
		       COALESCE(sp.store_name, '') AS store_name,
		       u.account_status,
		       (u.deleted_at IS NOT NULL) AS is_deleted,
		       COALESCE(ss.status::text, '') AS subscription_status,
		       COALESCE(sp.tier::text, '') AS tier,
		       COALESCE(p.media_urls, '[]'::jsonb) AS media_urls
		FROM for_sales fps
		JOIN products p ON p.id = fps.product_id
		JOIN users u ON u.id = fps.seller_id
		LEFT JOIN user_profiles up ON up.user_id = u.id
		LEFT JOIN seller_profiles sp ON sp.user_id = u.id
		LEFT JOIN LATERAL (
			SELECT status
			FROM seller_subscriptions
			WHERE user_id = u.id
			ORDER BY created_at DESC
			LIMIT 1
		) ss ON true
		WHERE fps.id = ANY($1)
	`, targetIDs)
	if err != nil {
		return nil, fmt.Errorf("content: load fixed price sales failed: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]fpsTargetRow, len(targetIDs))
	for rows.Next() {
		var (
			id                 uuid.UUID
			sellerID           uuid.UUID
			title              string
			price              int64
			status             string
			quantityAvailable  int
			username           string
			avatarURL          *string
			storeName          string
			accountStatus      string
			authorDeleted      bool
			subscriptionStatus string
			tier               string
			mediaURLsRaw       json.RawMessage
		)
		if err := rows.Scan(&id, &sellerID, &title, &price, &status, &quantityAvailable, &username, &avatarURL, &storeName, &accountStatus, &authorDeleted, &subscriptionStatus, &tier, &mediaURLsRaw); err != nil {
			return nil, fmt.Errorf("content: scan fixed price sale target failed: %w", err)
		}
		result[id] = fpsTargetRow{
			sellerID:           sellerID,
			title:              title,
			price:              price,
			status:             status,
			quantityAvailable:  quantityAvailable,
			username:           username,
			avatarURL:          avatarURL,
			storeName:          storeName,
			accountStatus:      accountStatus,
			authorDeleted:      authorDeleted,
			subscriptionStatus: subscriptionStatus,
			tier:               tier,
			mediaURLs:          decodeJSONStrings(mediaURLsRaw),
			isLive:             true,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("content: iterate fixed price sales failed: %w", err)
	}
	return result, nil
}

func (r *ContentResourceProjectionResolver) loadAuctions(
	ctx context.Context,
	tx db.Tx,
	targetIDs []uuid.UUID,
) (map[uuid.UUID]auctionTargetRow, error) {
	if len(targetIDs) == 0 {
		return map[uuid.UUID]auctionTargetRow{}, nil
	}

	rows, err := tx.Query(ctx, `
		SELECT a.id,
		       a.seller_id,
		       p.title,
		       a.current_bid,
		       a.buy_now_price,
		       a.end_at,
		       a.status,
		       COALESCE(up.username, '') AS username,
		       up.avatar_url,
		       COALESCE(sp.store_name, '') AS store_name,
		       u.account_status,
		       (u.deleted_at IS NOT NULL) AS is_deleted,
		       COALESCE(ss.status::text, '') AS subscription_status,
		       COALESCE(sp.tier::text, '') AS tier,
		       COALESCE(p.media_urls, '[]'::jsonb) AS media_urls
		FROM auctions a
		JOIN products p ON p.id = a.product_id
		JOIN users u ON u.id = a.seller_id
		LEFT JOIN user_profiles up ON up.user_id = u.id
		LEFT JOIN seller_profiles sp ON sp.user_id = u.id
		LEFT JOIN LATERAL (
			SELECT status
			FROM seller_subscriptions
			WHERE user_id = u.id
			ORDER BY created_at DESC
			LIMIT 1
		) ss ON true
		WHERE a.id = ANY($1)
	`, targetIDs)
	if err != nil {
		return nil, fmt.Errorf("content: load auctions failed: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]auctionTargetRow, len(targetIDs))
	for rows.Next() {
		var (
			id                 uuid.UUID
			sellerID           uuid.UUID
			title              string
			currentBid         *int64
			buyNowPrice        *int64
			endAt              time.Time
			status             string
			username           string
			avatarURL          *string
			storeName          string
			accountStatus      string
			authorDeleted      bool
			subscriptionStatus string
			tier               string
			mediaURLsRaw       json.RawMessage
		)
		if err := rows.Scan(&id, &sellerID, &title, &currentBid, &buyNowPrice, &endAt, &status, &username, &avatarURL, &storeName, &accountStatus, &authorDeleted, &subscriptionStatus, &tier, &mediaURLsRaw); err != nil {
			return nil, fmt.Errorf("content: scan auction target failed: %w", err)
		}
		result[id] = auctionTargetRow{
			sellerID:           sellerID,
			title:              title,
			currentBid:         currentBid,
			buyNowPrice:        buyNowPrice,
			endAt:              endAt,
			status:             status,
			username:           username,
			avatarURL:          avatarURL,
			storeName:          storeName,
			accountStatus:      accountStatus,
			authorDeleted:      authorDeleted,
			subscriptionStatus: subscriptionStatus,
			tier:               tier,
			mediaURLs:          decodeJSONStrings(mediaURLsRaw),
			isLive:             true,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("content: iterate auctions failed: %w", err)
	}
	return result, nil
}

func contentAuthorIDs(rows map[uuid.UUID]contentTargetRow) []uuid.UUID {
	if len(rows) == 0 {
		return nil
	}
	out := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.authorID)
	}
	return out
}

func sellerIDsForCommerce(fpsRows map[uuid.UUID]fpsTargetRow, auctionRows map[uuid.UUID]auctionTargetRow) []uuid.UUID {
	if len(fpsRows)+len(auctionRows) == 0 {
		return nil
	}
	out := make([]uuid.UUID, 0, len(fpsRows)+len(auctionRows))
	for _, row := range fpsRows {
		out = append(out, row.sellerID)
	}
	for _, row := range auctionRows {
		out = append(out, row.sellerID)
	}
	return out
}

func followedAuthorSet(ctx context.Context, tx db.Tx, viewerID uuid.UUID, authors []uuid.UUID) (map[uuid.UUID]bool, error) {
	result := make(map[uuid.UUID]bool)
	if viewerID == uuid.Nil || len(authors) == 0 {
		return result, nil
	}
	rows, err := tx.Query(ctx, `
		SELECT following_id
		FROM user_follows
		WHERE follower_id = $1 AND following_id = ANY($2)
	`, viewerID, authors)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func buildSellerCard(
	id uuid.UUID,
	username string,
	avatarURL *string,
	farmName string,
	accountStatus string,
	deleted bool,
	subscriptionStatus string,
	tier string,
) publiccard.SellerCard {
	userLifecycle := string(viewercontext.CoarsenLifecycle(accountStatus, deleted))
	trustLifecycle := string(viewercontext.CoarsenSellerTrust(subscriptionStatus))
	card := publiccard.SellerCard{
		User:      buildPublicUserCard(id, username, avatarURL, accountStatus, deleted),
		AvatarURL: avatarURL,
	}
	if farmName != "" {
		card.FarmName = &farmName
	}
	if trustLifecycle != "" {
		card.Lifecycle = &trustLifecycle
	}
	card.Tier = publiccard.GatedSellerTier(tier, userLifecycle, trustLifecycle)
	return card
}

func decodeJSONStrings(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err == nil {
		return out
	}
	return nil
}
