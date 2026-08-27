package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/interaction/saved_item/entity"
	"github.com/labuda/backend/internal/interaction/saved_item/repository"
	"github.com/labuda/backend/pkg/db"
)

// savedItemRepositoryImpl implements the SavedItemRepository interface
type savedItemRepositoryImpl struct {
	db *db.DB
}

// NewSavedItemRepository creates a new SavedItemRepository
func NewSavedItemRepository(database *db.DB) repository.SavedItemRepository {
	return &savedItemRepositoryImpl{
		db: database,
	}
}

// Create creates a new saved item
func (r *savedItemRepositoryImpl) Create(ctx context.Context, item *entity.SavedItem) error {
	query := `
		INSERT INTO saved_items (id, user_id, target_type, target_id, intent_type, seller_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id, target_type, target_id) DO NOTHING
	`

	_, err := r.db.Pool().Exec(ctx, query,
		item.ID,
		item.UserID,
		item.TargetType,
		item.TargetID,
		item.IntentType,
		item.SellerID,
		item.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create saved item: %w", err)
	}

	return nil
}

// GetByUser retrieves all saved items for a user
func (r *savedItemRepositoryImpl) GetByUser(ctx context.Context, userID uuid.UUID) ([]*entity.SavedItem, error) {
	query := `
		SELECT id, user_id, target_type, target_id, intent_type, seller_id, created_at
		FROM saved_items
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Pool().Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get saved items: %w", err)
	}
	defer rows.Close()

	var items []*entity.SavedItem
	for rows.Next() {
		item := &entity.SavedItem{}
		err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.TargetType,
			&item.TargetID,
			&item.IntentType,
			&item.SellerID,
			&item.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan saved item: %w", err)
		}
		items = append(items, item)
	}

	return items, nil
}

// GetByUserAndTarget retrieves a specific saved item
func (r *savedItemRepositoryImpl) GetByUserAndTarget(ctx context.Context, userID uuid.UUID, targetType entity.TargetType, targetID uuid.UUID) (*entity.SavedItem, error) {
	query := `
		SELECT id, user_id, target_type, target_id, intent_type, seller_id, created_at
		FROM saved_items
		WHERE user_id = $1 AND target_type = $2 AND target_id = $3
	`

	item := &entity.SavedItem{}
	err := r.db.Pool().QueryRow(ctx, query, userID, targetType, targetID).Scan(
		&item.ID,
		&item.UserID,
		&item.TargetType,
		&item.TargetID,
		&item.IntentType,
		&item.SellerID,
		&item.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get saved item: %w", err)
	}

	return item, nil
}

// Delete removes a saved item
func (r *savedItemRepositoryImpl) Delete(ctx context.Context, userID uuid.UUID, targetType entity.TargetType, targetID uuid.UUID) error {
	query := `
		DELETE FROM saved_items
		WHERE user_id = $1 AND target_type = $2 AND target_id = $3
	`

	_, err := r.db.Pool().Exec(ctx, query, userID, targetType, targetID)
	if err != nil {
		return fmt.Errorf("failed to delete saved item: %w", err)
	}

	return nil
}

// DeleteByType removes all saved items of a type for a user
func (r *savedItemRepositoryImpl) DeleteByType(ctx context.Context, userID uuid.UUID, targetType entity.TargetType) error {
	query := `
		DELETE FROM saved_items
		WHERE user_id = $1 AND target_type = $2
	`

	_, err := r.db.Pool().Exec(ctx, query, userID, targetType)
	if err != nil {
		return fmt.Errorf("failed to delete saved items by type: %w", err)
	}

	return nil
}

// DeleteAll removes all saved items for a user
func (r *savedItemRepositoryImpl) DeleteAll(ctx context.Context, userID uuid.UUID) error {
	query := `DELETE FROM saved_items WHERE user_id = $1`

	_, err := r.db.Pool().Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete all saved items: %w", err)
	}

	return nil
}

// Count returns the number of saved items for a user
func (r *savedItemRepositoryImpl) Count(ctx context.Context, userID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM saved_items WHERE user_id = $1`

	var count int
	err := r.db.Pool().QueryRow(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count saved items: %w", err)
	}

	return count, nil
}

// CountByType returns the number of saved items by type
func (r *savedItemRepositoryImpl) CountByType(ctx context.Context, userID uuid.UUID, targetType entity.TargetType) (int, error) {
	query := `SELECT COUNT(*) FROM saved_items WHERE user_id = $1 AND target_type = $2`

	var count int
	err := r.db.Pool().QueryRow(ctx, query, userID, targetType).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count saved items by type: %w", err)
	}

	return count, nil
}

// GetByUserWithForSales retrieves saved forSales with forSale info.
//
// Fixed-price sale title/description/media live on products (canonical item
// authority); price/quantity/status live on for_sales. The legacy
// `forSales` table is write-dead — nothing inserts or updates it anymore, so
// hydrating from it silently returned null title/price/media for every real
// saved forSale. for_sales has no for_sale_type/visibility columns:
// for_sale_type is always "fixed_price" for this target type (saved items
// distinguish 'for_sale' vs 'auction' via target_type already), and
// visibility is derived the same way ForSaleRepositoryImpl derives
// it — active status with a non-nil published_at.
func (r *savedItemRepositoryImpl) GetByUserWithForSales(ctx context.Context, userID uuid.UUID) ([]*entity.SavedItemWithForSale, error) {
	query := `
		SELECT
			si.id, si.user_id, si.target_type, si.target_id, si.intent_type, si.seller_id, si.created_at,
			p.title as for_sale_title,
			fps.price_per_unit as for_sale_price,
			fps.quantity_available as quantity_available,
			fps.status as for_sale_status,
			fps.published_at as for_sale_published_at,
			p.media_urls as for_sale_media_urls
		FROM saved_items si
		LEFT JOIN for_sales fps ON si.target_id = fps.id
		LEFT JOIN products p ON p.id = fps.product_id
		WHERE si.user_id = $1 AND si.target_type = 'for_sale'
		ORDER BY si.created_at DESC
	`

	rows, err := r.db.Pool().Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get saved items with forSales: %w", err)
	}
	defer rows.Close()

	var items []*entity.SavedItemWithForSale
	for rows.Next() {
		item := &entity.SavedItemWithForSale{}
		var publishedAt *time.Time
		err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.TargetType,
			&item.TargetID,
			&item.IntentType,
			&item.SellerID,
			&item.CreatedAt,
			&item.ForSaleTitle,
			&item.ForSalePrice,
			&item.QuantityAvailable,
			&item.ForSaleStatus,
			&publishedAt,
			&item.ForSaleMediaURLs,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan saved item with forSale: %w", err)
		}
		item.ForSaleType = "fixed_price"
		if item.ForSaleStatus == "active" && publishedAt != nil {
			item.ForSaleVisibility = "public"
		} else {
			item.ForSaleVisibility = "private"
		}
		items = append(items, item)
	}

	return items, nil
}

// GetByUserWithAuctions retrieves saved auctions with auction info
func (r *savedItemRepositoryImpl) GetByUserWithAuctions(ctx context.Context, userID uuid.UUID) ([]*entity.SavedItemWithAuction, error) {
	query := `
		SELECT
			si.id, si.user_id, si.target_type, si.target_id, si.intent_type, si.seller_id, si.created_at,
			p.title as auction_title,
			a.status as auction_status,
			a.start_price as start_price,
			a.current_bid as current_bid,
			a.end_at as end_at
		FROM saved_items si
		LEFT JOIN auctions a ON si.target_id = a.id
		LEFT JOIN products p ON p.id = a.product_id
		WHERE si.user_id = $1 AND si.target_type = 'auction'
		ORDER BY si.created_at DESC
	`

	rows, err := r.db.Pool().Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get saved items with auctions: %w", err)
	}
	defer rows.Close()

	var items []*entity.SavedItemWithAuction
	for rows.Next() {
		item := &entity.SavedItemWithAuction{}
		err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.TargetType,
			&item.TargetID,
			&item.IntentType,
			&item.SellerID,
			&item.CreatedAt,
			&item.AuctionTitle,
			&item.AuctionStatus,
			&item.StartPrice,
			&item.CurrentBid,
			&item.EndAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan saved item with auction: %w", err)
		}
		items = append(items, item)
	}

	return items, nil
}


