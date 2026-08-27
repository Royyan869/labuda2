package serverboot

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	fpsEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	commerceshared "github.com/labuda/backend/internal/commerce/shared"
	"github.com/labuda/backend/internal/governance/viewercontext"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/internal/pkg/blockcheck"
	"github.com/labuda/backend/internal/platform/mediaresolve"
	"github.com/labuda/backend/pkg/db"
)

type forSaleProjectionBatchResolver struct {
	db forSaleProjectionDB
}

type forSaleProjectionDB interface {
	WithTx(ctx context.Context, fn func(db.Tx) error) error
}

type forSaleSourceRow struct {
	saleID             uuid.UUID
	productID          uuid.UUID
	sellerID           uuid.UUID
	title              string
	pricePerUnit       int64
	quantityAvailable  int
	negotiationEnabled bool
	status             string
	publishedAt        sql.NullTime
	soldAt             sql.NullTime
	withdrawnAt        sql.NullTime
	createdAt          time.Time
	updatedAt          time.Time
	productMediaURLs   json.RawMessage
	username           sql.NullString
	storeName          sql.NullString
	storeImageURL      sql.NullString
	accountStatus      string
	deletedAt          sql.NullTime
	subscriptionStatus string
}

func newForSaleProjectionBatchResolver(database forSaleProjectionDB) *forSaleProjectionBatchResolver {
	return &forSaleProjectionBatchResolver{db: database}
}

var _ chatApp.ResourceProjectionResolver = (*forSaleProjectionBatchResolver)(nil)

func (r *forSaleProjectionBatchResolver) ResolveResourceProjections(
	ctx context.Context,
	viewerID uuid.UUID,
	occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
) (map[uuid.UUID]*chatApp.ResourceProjection, error) {
	return r.ResolveForSales(ctx, viewerID, occurrences)
}

func (r *forSaleProjectionBatchResolver) ResolveForSales(
	ctx context.Context,
	viewerID uuid.UUID,
	occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
) (map[uuid.UUID]*chatApp.ResourceProjection, error) {
	if len(occurrences) == 0 {
		return map[uuid.UUID]*chatApp.ResourceProjection{}, nil
	}
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("chat: fixed price sale projection resolver not configured")
	}

	saleIDs := make([]uuid.UUID, 0, len(occurrences))
	seenSaleIDs := make(map[uuid.UUID]struct{}, len(occurrences))
	occurrenceByMsgID := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, len(occurrences))

	for messageID, occ := range occurrences {
		if occ == nil {
			return nil, fmt.Errorf("chat: nil occurrence for message %s", messageID)
		}
		if !occ.ResourceType().IsValid() {
			return nil, fmt.Errorf("chat: malformed occurrence identity for message %s", messageID)
		}
		if occ.ResourceType() != chatEntity.ResourceOccurrenceResourceTypeForSale {
			return nil, fmt.Errorf("chat: unsupported resource type %q in fixed price sale resolver", occ.ResourceType())
		}
		if occ.ForSaleSourceID == nil {
			return nil, fmt.Errorf("chat: fixed price sale occurrence for message %s requires non-nil fixed price sale source id", messageID)
		}
		if _, seen := seenSaleIDs[*occ.ForSaleSourceID]; !seen {
			saleIDs = append(saleIDs, *occ.ForSaleSourceID)
			seenSaleIDs[*occ.ForSaleSourceID] = struct{}{}
		}
		occurrenceByMsgID[messageID] = occ
	}

	if len(saleIDs) == 0 {
		return map[uuid.UUID]*chatApp.ResourceProjection{}, nil
	}

	var result map[uuid.UUID]*chatApp.ResourceProjection
	err := r.db.WithTx(ctx, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT
				fps.id,
				fps.product_id,
				fps.seller_id,
				p.title,
				fps.price_per_unit,
				fps.quantity_available,
				fps.negotiation_enabled,
				fps.status,
				fps.published_at,
				fps.sold_at,
				fps.withdrawn_at,
				fps.created_at,
				fps.updated_at,
				COALESCE(p.media_urls, '[]'::jsonb) AS product_media_urls,
				COALESCE(up.username, '') AS username,
				COALESCE(sp.store_name, '') AS store_name,
				COALESCE(sp.store_image_url, '') AS store_image_url,
				u.account_status,
				u.deleted_at,
				COALESCE(ss.status::text, '') AS subscription_status
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
		`, saleIDs)
		if err != nil {
			return fmt.Errorf("chat: fixed price sale source batch query failed: %w", err)
		}
		defer rows.Close()

		byID := make(map[uuid.UUID]forSaleSourceRow, len(saleIDs))
		sellerIDs := make([]uuid.UUID, 0, len(saleIDs))
		seenSellerIDs := make(map[uuid.UUID]struct{}, len(saleIDs))
		for rows.Next() {
			var row forSaleSourceRow
			if err := rows.Scan(
				&row.saleID,
				&row.productID,
				&row.sellerID,
				&row.title,
				&row.pricePerUnit,
				&row.quantityAvailable,
				&row.negotiationEnabled,
				&row.status,
				&row.publishedAt,
				&row.soldAt,
				&row.withdrawnAt,
				&row.createdAt,
				&row.updatedAt,
				&row.productMediaURLs,
				&row.username,
				&row.storeName,
				&row.storeImageURL,
				&row.accountStatus,
				&row.deletedAt,
				&row.subscriptionStatus,
			); err != nil {
				return fmt.Errorf("chat: fixed price sale source batch scan failed: %w", err)
			}
			byID[row.saleID] = row
			if row.sellerID == uuid.Nil {
				return fmt.Errorf("chat: fixed price sale source row missing seller id for %s", row.saleID)
			}
			if _, seen := seenSellerIDs[row.sellerID]; !seen {
				sellerIDs = append(sellerIDs, row.sellerID)
				seenSellerIDs[row.sellerID] = struct{}{}
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("chat: fixed price sale source batch iteration failed: %w", err)
		}

		for _, saleID := range saleIDs {
			if _, ok := byID[saleID]; !ok {
				return fmt.Errorf("chat: fixed price sale source row missing for %s", saleID)
			}
		}

		blockedSet := map[uuid.UUID]bool{}
		if viewerID != uuid.Nil && len(sellerIDs) > 0 {
			blockedSet, err = blockcheck.BlockedSet(ctx, tx, viewerID, sellerIDs)
			if err != nil {
				return fmt.Errorf("chat: fixed price sale block batch query failed: %w", err)
			}
		}

		result = make(map[uuid.UUID]*chatApp.ResourceProjection, len(occurrenceByMsgID))
		for messageID, occ := range occurrenceByMsgID {
			sourceID := occ.SourceID()
			row := byID[sourceID]
			sellerLifecycle := viewercontext.CoarsenLifecycle(row.accountStatus, row.deletedAt.Valid)
			blocked := viewerID != uuid.Nil && viewerID != row.sellerID && blockedSet[row.sellerID]
			var publishedAt *time.Time
			if row.publishedAt.Valid {
				publishedAt = &row.publishedAt.Time
			}
			visibility := fpsEntity.DeriveVisibility(fpsEntity.ForSaleStatus(row.status), publishedAt)

			if !commerceshared.EvaluateForSaleViewAccess(commerceshared.ForSaleViewAccessInput{
				ViewerID:   viewerID,
				SellerID:   row.sellerID,
				Status:     row.status,
				Visibility: string(visibility),
				Blocked:    blocked,
				Seller: commerceshared.SellerAccessSnapshot{
					AccountStatus:      row.accountStatus,
					IsDeleted:          row.deletedAt.Valid,
					SubscriptionStatus: row.subscriptionStatus,
				},
			}) {
				proj, projErr := chatApp.NewTombstoneProjection(chatEntity.ResourceOccurrenceResourceTypeForSale)
				if projErr != nil {
					return projErr
				}
				result[messageID] = &proj
				continue
			}

			imageURL := firstResolvedURLFromJSONStrings(row.productMediaURLs)
			sellerTrustActive := viewercontext.CoarsenSellerTrust(row.subscriptionStatus) == viewercontext.PublicLifecycleStateActive
			forSaleCaps := commerceshared.EvaluateForSaleViewerCapabilities(commerceshared.ForSaleViewerCapabilitiesInput{
				ViewerID:           viewerID,
				SellerID:           row.sellerID,
				ProductID:          row.productID,
				Status:             row.status,
				QuantityAvailable:  row.quantityAvailable,
				NegotiationEnabled: row.negotiationEnabled,
				SellerTrustActive:  sellerTrustActive,
			})
			commerceActions := buildForSaleCommerceActions(forSaleCaps)
			viewerCaps := chatApp.ProjectionViewerCapabilities{
				CanView:            true,
				CanInteract:        commerceActions.CanBuy || commerceActions.CanNegotiate,
				BlockedByTombstone: false,
			}

			payload := chatApp.ForSaleLivePayload{
				Title:             row.title,
				ImageURL:          imageURL,
				Price:             chatApp.ForSaleLivePrice{Amount: row.pricePerUnit, Currency: "IDR"},
				Status:            row.status,
				Seller:            buildForSaleLiveSeller(row, sellerLifecycle),
				QuantityAvailable: row.quantityAvailable,
			}

			proj, projErr := chatApp.NewLiveProjection(
				chatEntity.ResourceOccurrenceResourceTypeForSale,
				sourceID,
				payload,
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

func buildForSaleCommerceActions(
	caps commerceshared.ViewerCapabilities,
) chatApp.CommerceActionCapabilities {
	return chatApp.CommerceActionCapabilities{
		Role:         caps.Role,
		CanChat:      caps.CanChat,
		CanNegotiate: caps.CanNegotiate,
		CanBuy:       caps.CanBuy,
		CanBid:       caps.CanBid,
		CanManage:    caps.CanManage,
	}
}

func buildForSaleLiveSeller(row forSaleSourceRow, lifecycle viewercontext.PublicLifecycleState) chatApp.ForSaleLiveSeller {
	seller := chatApp.ForSaleLiveSeller{
		ID:        row.sellerID,
		StoreName: strings.TrimSpace(row.storeName.String),
		Username:  strings.TrimSpace(row.username.String),
		Lifecycle: string(lifecycle),
	}
	if row.storeImageURL.Valid {
		if trimmed := strings.TrimSpace(row.storeImageURL.String); trimmed != "" {
			resolved := resolveReadableSaleMediaReference(trimmed)
			seller.StoreImage = &resolved
		}
	}
	return seller
}

func firstResolvedURLFromJSONStrings(raw json.RawMessage) *string {
	for _, value := range decodeJSONStringSlice(raw) {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			resolved := resolveReadableSaleMediaReference(trimmed)
			if resolved != "" {
				return &resolved
			}
		}
	}
	return nil
}

func decodeJSONStringSlice(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return []string{}
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return []string{}
	}
	return values
}

func resolveReadableSaleMediaReference(value string) string {
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
