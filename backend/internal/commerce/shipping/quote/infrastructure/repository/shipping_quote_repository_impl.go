package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/commerce/shipping/quote/entity"
	shippingQuoteRepo "github.com/labuda/backend/internal/commerce/shipping/quote/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

var (
	errShippingQuoteInvalidPersistedState = errors.New("invalid shipping quote persisted state")
	errShippingQuoteStateConflict         = errors.New("shipping quote state conflict")
)

// ShippingQuoteRepositoryImpl handles shipping quote persistence using pgx-based DB layer.
type ShippingQuoteRepositoryImpl struct{}

// NewShippingQuoteRepository creates a new ShippingQuoteRepositoryImpl.
func NewShippingQuoteRepository() *ShippingQuoteRepositoryImpl {
	return &ShippingQuoteRepositoryImpl{}
}

var _ shippingQuoteRepo.ShippingQuoteRepository = (*ShippingQuoteRepositoryImpl)(nil)

// Create persists a new shipping quote within a transaction.
func (r *ShippingQuoteRepositoryImpl) Create(
	ctx context.Context,
	tx db.Tx,
	quote *entity.ShippingQuote,
) error {
	if quote.ExpiresAt == nil {
		return fmt.Errorf("create shipping quote failed: %w", errShippingQuoteInvalidPersistedState)
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO shipping_quotes (
			id, chat_id, product_id, source_type, source_id, auction_id, seller_id, buyer_id,
			cost, note, status, destination_city_id, destination_province_id,
			created_at, expires_at, reactivation_count, max_reuse
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`,
		quote.ID,
		quote.ChatID,
		quote.ProductID,
		quote.SourceType,
		quote.SourceID,
		quote.AuctionID,
		quote.SellerID,
		quote.BuyerID,
		quote.Cost.Int64(),
		quote.Note,
		quote.Status,
		quote.DestinationCityID,
		quote.DestinationProvinceID,
		quote.CreatedAt,
		quote.ExpiresAt,
		quote.ReactivationCount,
		quote.MaxReuse,
	)
	if err != nil {
		return fmt.Errorf("create shipping quote failed: %w", err)
	}
	return nil
}

func scanShippingQuote(
	row pgx.Row,
) (*entity.ShippingQuote, error) {
	var quote entity.ShippingQuote
	var note *string
	var cost int64
	var status string
	var sourceType *string
	var sourceID *uuid.UUID
	var auctionID *uuid.UUID
	var supersededAt *time.Time
	var supersededByID *uuid.UUID
	var destCityID, destProvinceID *string
	var usedAt, expiresAt *time.Time
	var reactivationCount, maxReuse int

	if err := row.Scan(
		&quote.ID,
		&quote.ChatID,
		&quote.ProductID,
		&sourceType,
		&sourceID,
		&auctionID,
		&quote.SellerID,
		&quote.BuyerID,
		&cost,
		&note,
		&status,
		&supersededAt,
		&supersededByID,
		&destCityID,
		&destProvinceID,
		&usedAt,
		&expiresAt,
		&quote.CreatedAt,
		&reactivationCount,
		&maxReuse,
	); err != nil {
		return nil, err
	}

	quote.Cost = money.New(cost)
	quote.Note = note
	quote.Status = entity.QuoteStatus(status)
	quote.SourceType = sourceType
	quote.SourceID = sourceID
	quote.AuctionID = auctionID
	quote.SupersededAt = supersededAt
	quote.SupersededByID = supersededByID
	quote.DestinationCityID = destCityID
	quote.DestinationProvinceID = destProvinceID
	quote.UsedAt = usedAt
	quote.ExpiresAt = expiresAt
	quote.ReactivationCount = reactivationCount
	quote.MaxReuse = maxReuse

	if quote.ExpiresAt == nil {
		return nil, fmt.Errorf("%w: quote %s has NULL expires_at", errShippingQuoteInvalidPersistedState, quote.ID)
	}

	return &quote, nil
}

const shippingQuoteSelectColumns = `
		SELECT id, chat_id, product_id, source_type, source_id, auction_id, seller_id, buyer_id,
		       cost, note, status, superseded_at, superseded_by_id,
		       destination_city_id, destination_province_id,
		       used_at, expires_at, created_at, reactivation_count, max_reuse
`

// GetLatestByChatAndSource retrieves the current ACTIVE unsuperseded quote
// for a canonical commercial context.
func (r *ShippingQuoteRepositoryImpl) GetLatestByChatAndSource(
	ctx context.Context,
	tx db.Tx,
	chatID, productID uuid.UUID,
	sourceType string,
	sourceID, sellerID, buyerID uuid.UUID,
) (*entity.ShippingQuote, error) {
	row := tx.QueryRow(ctx, shippingQuoteSelectColumns+`
		FROM shipping_quotes
		WHERE chat_id = $1
		  AND product_id = $2
		  AND source_type = $3
		  AND source_id = $4
		  AND seller_id = $5
		  AND buyer_id = $6
		  AND status = 'ACTIVE'
		  AND superseded_at IS NULL
		  AND used_at IS NULL
		  AND expires_at > CURRENT_TIMESTAMP
		ORDER BY created_at DESC
		LIMIT 1
	`, chatID, productID, sourceType, sourceID, sellerID, buyerID)

	quote, err := scanShippingQuote(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest shipping quote failed: %w", err)
	}

	return quote, nil
}

// GetLatestRevisionByChatAndSource retrieves the latest quote revision for a
// canonical commercial context regardless of lifecycle state.
func (r *ShippingQuoteRepositoryImpl) GetLatestRevisionByChatAndSource(
	ctx context.Context,
	tx db.Tx,
	chatID, productID uuid.UUID,
	sourceType string,
	sourceID, sellerID, buyerID uuid.UUID,
) (*entity.ShippingQuote, error) {
	row := tx.QueryRow(ctx, shippingQuoteSelectColumns+`
		FROM shipping_quotes
		WHERE chat_id = $1
		  AND product_id = $2
		  AND source_type = $3
		  AND source_id = $4
		  AND seller_id = $5
		  AND buyer_id = $6
		ORDER BY created_at DESC
		LIMIT 1
	`, chatID, productID, sourceType, sourceID, sellerID, buyerID)

	quote, err := scanShippingQuote(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest shipping quote revision failed: %w", err)
	}

	return quote, nil
}

// GetCurrentActiveByChatAndSource retrieves the current ACTIVE unsuperseded
// quote for a canonical commercial context without applying expiry freshness.
func (r *ShippingQuoteRepositoryImpl) GetCurrentActiveByChatAndSource(
	ctx context.Context,
	tx db.Tx,
	chatID, productID uuid.UUID,
	sourceType string,
	sourceID, sellerID, buyerID uuid.UUID,
) (*entity.ShippingQuote, error) {
	row := tx.QueryRow(ctx, shippingQuoteSelectColumns+`
		FROM shipping_quotes
		WHERE chat_id = $1
		  AND product_id = $2
		  AND source_type = $3
		  AND source_id = $4
		  AND seller_id = $5
		  AND buyer_id = $6
		  AND status = 'ACTIVE'
		  AND superseded_at IS NULL
		ORDER BY created_at DESC
		LIMIT 1
	`, chatID, productID, sourceType, sourceID, sellerID, buyerID)

	quote, err := scanShippingQuote(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get current active shipping quote failed: %w", err)
	}

	return quote, nil
}

// GetByID retrieves a shipping quote by ID.
func (r *ShippingQuoteRepositoryImpl) GetByID(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (*entity.ShippingQuote, error) {
	row := tx.QueryRow(ctx, shippingQuoteSelectColumns+`
		FROM shipping_quotes
		WHERE id = $1
	`, id)

	quote, err := scanShippingQuote(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get shipping quote by id failed: %w", err)
	}

	return quote, nil
}

// GetByIDForUpdate retrieves a shipping quote by ID with FOR UPDATE lock.
func (r *ShippingQuoteRepositoryImpl) GetByIDForUpdate(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (*entity.ShippingQuote, error) {
	row := tx.QueryRow(ctx, shippingQuoteSelectColumns+`
		FROM shipping_quotes
		WHERE id = $1
		FOR UPDATE
	`, id)

	quote, err := scanShippingQuote(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get shipping quote by id for update failed: %w", err)
	}

	return quote, nil
}

// UpdateStatus updates the status and related timestamp fields of a shipping quote.
func (r *ShippingQuoteRepositoryImpl) UpdateStatus(
	ctx context.Context,
	tx db.Tx,
	quoteID uuid.UUID,
	status entity.QuoteStatus,
	usedAt *interface{},
) error {
	now := time.Now()
	query := `
		UPDATE shipping_quotes
		SET status = $1
	`
	args := []interface{}{status}
	argIdx := 2

	if usedAt != nil {
		query += fmt.Sprintf(", used_at = $%d", argIdx)
		args = append(args, now)
		argIdx++
	}

	query += fmt.Sprintf(" WHERE id = $%d", argIdx)
	args = append(args, quoteID)

	tag, err := tx.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update shipping quote status failed: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("%w: update shipping quote status affected %d rows for quote %s", errShippingQuoteStateConflict, tag.RowsAffected(), quoteID)
	}
	return nil
}

// ReactivateQuote reactivates a USED quote back to ACTIVE status.
func (r *ShippingQuoteRepositoryImpl) ReactivateQuote(
	ctx context.Context,
	tx db.Tx,
	quoteID uuid.UUID,
) error {
	newExpiry := time.Now().Add(24 * time.Hour)
	tag, err := tx.Exec(ctx, `
		UPDATE shipping_quotes
		SET
			status = 'ACTIVE',
			used_at = NULL,
			expires_at = $1,
			reactivation_count = reactivation_count + 1
		WHERE id = $2
		  AND status = 'USED'
		  AND superseded_at IS NULL
		  AND reactivation_count < max_reuse
	`, newExpiry, quoteID)
	if err != nil {
		return fmt.Errorf("reactivate shipping quote failed: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("%w: reactivate shipping quote affected %d rows for quote %s", errShippingQuoteStateConflict, tag.RowsAffected(), quoteID)
	}
	return nil
}

// SupersedeCurrentQuotes marks all unsuperseded quotes for a canonical
// commercial context as superseded by the provided quote.
func (r *ShippingQuoteRepositoryImpl) SupersedeCurrentQuotes(
	ctx context.Context,
	tx db.Tx,
	chatID, productID uuid.UUID,
	sourceType string,
	sourceID, sellerID, buyerID, supersededByID uuid.UUID,
) (int64, error) {
	tag, err := tx.Exec(ctx, `
		UPDATE shipping_quotes
		SET superseded_at = NOW(),
		    superseded_by_id = $7
		WHERE chat_id = $1
		  AND product_id = $2
		  AND source_type = $3
		  AND source_id = $4
		  AND seller_id = $5
		  AND buyer_id = $6
		  AND superseded_at IS NULL
		  AND id <> $7
	`, chatID, productID, sourceType, sourceID, sellerID, buyerID, supersededByID)
	if err != nil {
		return 0, fmt.Errorf("supersede current quotes failed: %w", err)
	}
	return tag.RowsAffected(), nil
}

// InvalidateQuotesByProduct marks all ACTIVE quotes for a product as INVALID.
func (r *ShippingQuoteRepositoryImpl) InvalidateQuotesByProduct(
	ctx context.Context,
	tx db.Tx,
	productID uuid.UUID,
) error {
	tag, err := tx.Exec(ctx, `
		UPDATE shipping_quotes
		SET status = 'INVALID'
		WHERE product_id = $1 AND status = 'ACTIVE' AND superseded_at IS NULL
	`, productID)
	if err != nil {
		return fmt.Errorf("invalidate quotes by product failed: %w", err)
	}
	_ = tag.RowsAffected()
	return nil
}
