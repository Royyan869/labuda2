package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/forsale/entity"
	"github.com/labuda/backend/pkg/db"
)

// SearchFilters holds filter criteria for for_sale search.
type SearchFilters struct {
	Query    string     // Full-text search query
	PriceMin *int64     // Minimum price
	PriceMax *int64     // Maximum price
	Variety  *string    // Koi variety filter
	SellerID *uuid.UUID // Filter by seller
	Cursor   *time.Time // Cursor for pagination
	Limit    int        // Max results
	SortBy   string     // Sort order: relevance, price, created_at
	SortDir  string     // Sort direction: asc, desc
}

// ForSaleRepository defines the interface for for_sale persistence.
// All stock mutations must use FOR UPDATE to prevent concurrent modifications.
type ForSaleRepository interface {
	// Create persists a new for_sale within a transaction.
	Create(ctx context.Context, tx db.Tx, for_sale *entity.ForSale) error

	// GetByID retrieves a for_sale by its ForSale UUID without locking (for read-only operations).
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.ForSale, error)

	// GetByProductID retrieves a for_sale by its Product UUID without locking (for read-only operations).
	// Used when only the Product UUID is available (e.g., from auction.product_id).
	GetByProductID(ctx context.Context, tx db.Tx, productID uuid.UUID) (*entity.ForSale, error)

	// GetForUpdate retrieves a for_sale with FOR UPDATE lock.
	// This prevents concurrent modifications and must be used within a transaction.
	// REQUIRED for all stock mutations.
	GetForUpdate(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.ForSale, error)

	// Update persists changes to a for_sale within a transaction.
	Update(ctx context.Context, tx db.Tx, for_sale *entity.ForSale) error

	// UpdateStock updates quantity and status atomically within a transaction.
	// Uses FOR UPDATE to prevent concurrent modifications.
	UpdateStock(ctx context.Context, tx db.Tx, for_sale *entity.ForSale) error

	// UpdateStatus updates only the status field within a transaction.
	UpdateStatus(ctx context.Context, tx db.Tx, for_sale *entity.ForSale) error

	// Delete soft-deletes a for_sale (marks as withdrawn) within a transaction.
	Delete(ctx context.Context, tx db.Tx, id uuid.UUID) error

	// GetBySellerID retrieves a seller's full history (draft/active/sold;
	// withdrawn optional) — OWNER-ONLY inventory authority. Never expose this
	// through a public/anon path; public seller pages must use
	// GetPublicBySellerID.
	GetBySellerID(ctx context.Context, tx db.Tx, sellerID uuid.UUID, includeWithdrawn bool) ([]*entity.ForSale, error)

	// GetBySellerIDPaginated retrieves a seller's full history with
	// SQL-based pagination — OWNER-ONLY inventory authority. Never expose
	// this through a public/anon path; public seller pages must use
	// GetPublicBySellerID.
	GetBySellerIDPaginated(ctx context.Context, tx db.Tx, sellerID uuid.UUID, limit, offset int, includeWithdrawn bool) ([]*entity.ForSale, error)

	// GetPublicBySellerID retrieves publicly discoverable (active + in-stock)
	// for_sales of one seller for the public browsable seller page. Read-only.
	// This is NOT the seller-inventory query (use GetBySellerIDPaginated).
	GetPublicBySellerID(ctx context.Context, tx db.Tx, sellerID uuid.UUID, limit, offset int) ([]*entity.ForSale, error)

	// GetPublic retrieves public discoverable for_sales: active + in-stock
	// (quantity_available > 0) + seller account visible (read-only).
	GetPublic(ctx context.Context, tx db.Tx, limit, offset int) ([]*entity.ForSale, error)

	// Search performs full-text search on for_sales with cursor pagination.
	Search(ctx context.Context, tx db.Tx, filters SearchFilters) ([]*entity.ForSale, *time.Time, error)
}




