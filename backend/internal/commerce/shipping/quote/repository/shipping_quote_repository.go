package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/shipping/quote/entity"
	"github.com/labuda/backend/pkg/db"
)

// ShippingQuoteRepository defines the interface for shipping quote persistence.
type ShippingQuoteRepository interface {
	// Create persists a new shipping quote within a transaction.
	Create(ctx context.Context, tx db.Tx, quote *entity.ShippingQuote) error

	// GetLatestByChatAndSource retrieves the current ACTIVE unsuperseded quote
	// for a canonical commercial context.
	GetLatestByChatAndSource(ctx context.Context, tx db.Tx, chatID, productID uuid.UUID, sourceType string, sourceID, sellerID, buyerID uuid.UUID) (*entity.ShippingQuote, error)

	// GetLatestRevisionByChatAndSource retrieves the latest quote revision for a
	// canonical commercial context regardless of lifecycle state.
	GetLatestRevisionByChatAndSource(ctx context.Context, tx db.Tx, chatID, productID uuid.UUID, sourceType string, sourceID, sellerID, buyerID uuid.UUID) (*entity.ShippingQuote, error)

	// GetByID retrieves a shipping quote by ID.
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.ShippingQuote, error)

	// GetByIDs retrieves shipping quotes by ID in ONE batch query, keyed by
	// quote id. Missing ids are simply absent from the result. This is the
	// canonical batch read used to project quote actionability onto a
	// conversation message page without N+1.
	GetByIDs(ctx context.Context, tx db.Tx, ids []uuid.UUID) (map[uuid.UUID]*entity.ShippingQuote, error)

	// GetByIDForUpdate retrieves a shipping quote by ID with FOR UPDATE lock.
	// CRITICAL: Use this in order creation to prevent race condition where
	// the same quote could be used by multiple concurrent checkouts.
	GetByIDForUpdate(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.ShippingQuote, error)

	// UpdateStatus updates the status and related timestamp fields of a shipping quote.
	// Used for marking quotes as USED or EXPIRED (TASK C - Lifecycle).
	UpdateStatus(ctx context.Context, tx db.Tx, quoteID uuid.UUID, status entity.QuoteStatus, usedAt *interface{}) error

	// ReactivateQuote reactivates a USED quote back to ACTIVE status.
	// - Increments reactivation_count
	// - Resets used_at to NULL
	// - Resets expires_at to 24 hours from now
	ReactivateQuote(ctx context.Context, tx db.Tx, quoteID uuid.UUID) error

	// GetCurrentActiveByChatAndSource retrieves the current ACTIVE unsuperseded
	// quote for a canonical commercial context without applying expiry freshness.
	GetCurrentActiveByChatAndSource(ctx context.Context, tx db.Tx, chatID, productID uuid.UUID, sourceType string, sourceID, sellerID, buyerID uuid.UUID) (*entity.ShippingQuote, error)

	// SupersedeCurrentQuotes marks every unsuperseded quote in a canonical
	// commercial context as superseded by the provided new quote.
	SupersedeCurrentQuotes(ctx context.Context, tx db.Tx, chatID, productID uuid.UUID, sourceType string, sourceID, sellerID, buyerID, supersededByID uuid.UUID) (int64, error)

	// InvalidateQuotesByProduct marks all ACTIVE quotes for a product as INVALID.
	// Called when a product becomes unavailable (sold, withdrawn, deleted).
	InvalidateQuotesByProduct(ctx context.Context, tx db.Tx, productID uuid.UUID) error
}
