package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/interaction/saved_item/entity"
)

// SavedItemRepository defines the interface for saved item persistence operations
type SavedItemRepository interface {
	// Create creates a new saved item
	Create(ctx context.Context, item *entity.SavedItem) error

	// GetByUser retrieves all saved items for a user
	GetByUser(ctx context.Context, userID uuid.UUID) ([]*entity.SavedItem, error)

	// GetByUserAndTarget retrieves a specific saved item
	// Returns nil if not found
	GetByUserAndTarget(ctx context.Context, userID uuid.UUID, targetType entity.TargetType, targetID uuid.UUID) (*entity.SavedItem, error)

	// Delete removes a saved item
	Delete(ctx context.Context, userID uuid.UUID, targetType entity.TargetType, targetID uuid.UUID) error

	// DeleteByType removes all saved items of a type for a user
	DeleteByType(ctx context.Context, userID uuid.UUID, targetType entity.TargetType) error

	// DeleteAll removes all saved items for a user
	DeleteAll(ctx context.Context, userID uuid.UUID) error

	// Count returns the number of saved items for a user
	Count(ctx context.Context, userID uuid.UUID) (int, error)

	// CountByType returns the number of saved items by type
	CountByType(ctx context.Context, userID uuid.UUID, targetType entity.TargetType) (int, error)

	// GetByUserWithForSales retrieves saved forSales with forSale info
	GetByUserWithForSales(ctx context.Context, userID uuid.UUID) ([]*entity.SavedItemWithForSale, error)

	// GetByUserWithAuctions retrieves saved auctions with auction info
	GetByUserWithAuctions(ctx context.Context, userID uuid.UUID) ([]*entity.SavedItemWithAuction, error)
}


