package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/discovery/search/entity"
	"github.com/labuda/backend/pkg/db"
)

// SearchRepository defines the search data access interface.
// Search domain queries existing domains without duplicating data.
type SearchRepository interface {
	// Search History
	AddSearchHistory(ctx context.Context, tx db.Tx, userID uuid.UUID, query string) error
	GetSearchHistory(ctx context.Context, tx db.Tx, userID uuid.UUID, limit int) ([]*entity.SearchHistory, error)
	ClearSearchHistory(ctx context.Context, tx db.Tx, userID uuid.UUID) error
	DeleteSearchHistory(ctx context.Context, tx db.Tx, id uuid.UUID, userID uuid.UUID) error
	TrimSearchHistory(ctx context.Context, tx db.Tx, userID uuid.UUID) error

	// ForSale Search
	SearchForSales(ctx context.Context, tx db.Tx, filters entity.SearchFilters) ([]*entity.ForSalePreview, int, error)

	// Content Search
	SearchContent(ctx context.Context, tx db.Tx, filters entity.SearchFilters) ([]*entity.ContentPreview, int, error)

	// User Search
	SearchUsers(ctx context.Context, tx db.Tx, filters entity.SearchFilters) ([]*entity.UserPreview, int, error)

	// Auction Search
	// AUCTION SEARCH ELIGIBILITY (Phase 3.5):
	// Only searches auctions with status IN ('scheduled', 'active', 'ended')
	// Draft and cancelled auctions are NOT searchable
	SearchAuctions(ctx context.Context, tx db.Tx, filters entity.SearchFilters) ([]*entity.AuctionPreview, int, error)
}
