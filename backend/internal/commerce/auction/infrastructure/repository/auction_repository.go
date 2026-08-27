package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/commerce/auction/entity"
	productEntity "github.com/labuda/backend/internal/commerce/product/entity"
	"github.com/labuda/backend/pkg/db"
)

// AuctionRepository handles auction persistence using pgx-based DB layer.
// All mutations must use GetForUpdate() for row-level locking.
type AuctionRepository struct{}

// NewAuctionRepository creates a new AuctionRepository.
func NewAuctionRepository() *AuctionRepository {
	return &AuctionRepository{}
}

// CreateTx persists a new auction within a transaction.
func (r *AuctionRepository) CreateTx(
	ctx context.Context,
	tx db.Tx,
	auction *entity.Auction,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO auctions (
			id, seller_id, product_id, order_id, settlement_deadline,
			start_price, bid_increment, buy_now_price,
			start_at, end_at, current_bid, current_winner_id,
			status, created_at, updated_at, anti_snipe_extension_seconds
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`,
		auction.ID,
		auction.SellerID,
		auction.ProductID,
		auction.OrderID,
		auction.SettlementDeadline,
		auction.StartPrice,
		auction.BidIncrement,
		auction.BuyNowPrice,
		auction.StartAt,
		auction.EndAt,
		auction.CurrentBid,
		auction.CurrentWinnerID,
		string(auction.Status),
		auction.CreatedAt,
		auction.UpdatedAt,
		int64(auction.AntiSnipeExtensionTotal/time.Second),
	)

	if err != nil {
		return fmt.Errorf("create auction failed: %w", err)
	}

	return nil
}

// joinedAuctionColumns selects auction columns plus the joined Product columns.
// Product is the canonical authority for title, description, media, koi
// attributes, preparation and farm address — auction reads it read-only.
const joinedAuctionColumns = `a.id, a.seller_id, a.product_id, a.order_id, a.settlement_deadline,
	a.start_price, a.bid_increment, a.buy_now_price,
	a.start_at, a.end_at, a.current_bid, a.current_winner_id,
	a.status, a.created_at, a.updated_at, a.anti_snipe_extension_seconds,
	p.id, p.seller_id, p.title, p.description, p.media_urls,
	p.variety, p.size_cm, p.age_months, p.gender, p.breeder, p.bloodline, p.certificates,
	p.farm_address_id, p.preparation_time, p.preparation_note,
	p.created_at, p.updated_at`

// scanJoinedAuction scans a row produced by joinedAuctionColumns into an
// Auction entity with the canonical Product attached.
func scanJoinedAuction(row interface {
	Scan(dest ...any) error
}) (*entity.Auction, error) {
	var a entity.Auction
	var p productEntity.Product
	var orderID *uuid.UUID
	var settlementDeadline *time.Time
	var currentWinnerID *uuid.UUID
	var startPrice, bidIncrement int64
	var buyNowPrice, currentBid *int64
	var status string
	var startAt, endAt, createdAt, updatedAt time.Time
	var antiSnipeExtensionSeconds int64
	var mediaURLsRaw json.RawMessage
	var sizeCM, ageMonths *int
	var gender, breeder, bloodline, preparationNote *string
	var certificates []string
	var farmAddressID *uuid.UUID
	var productCreatedAt, productUpdatedAt time.Time

	err := row.Scan(
		&a.ID, &a.SellerID, &a.ProductID, &orderID, &settlementDeadline,
		&startPrice, &bidIncrement, &buyNowPrice,
		&startAt, &endAt, &currentBid, &currentWinnerID,
		&status, &createdAt, &updatedAt, &antiSnipeExtensionSeconds,
		&p.ID, &p.SellerID, &p.Title, &p.Description, &mediaURLsRaw,
		&p.Variety, &sizeCM, &ageMonths, &gender, &breeder, &bloodline, &certificates,
		&farmAddressID, &p.PreparationTime, &preparationNote,
		&productCreatedAt, &productUpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	var mediaURLs []string
	if len(mediaURLsRaw) > 0 && string(mediaURLsRaw) != "null" {
		if err := json.Unmarshal(mediaURLsRaw, &mediaURLs); err != nil {
			return nil, fmt.Errorf("unmarshal product media urls failed: %w", err)
		}
	}

	a.OrderID = orderID
	a.SettlementDeadline = settlementDeadline
	a.StartPrice = startPrice
	a.BidIncrement = bidIncrement
	a.BuyNowPrice = buyNowPrice
	a.StartAt = startAt
	a.EndAt = endAt
	a.CurrentBid = currentBid
	a.CurrentWinnerID = currentWinnerID
	a.Status = entity.Status(status)
	a.CreatedAt = createdAt
	a.UpdatedAt = updatedAt
	a.AntiSnipeExtensionTotal = time.Duration(antiSnipeExtensionSeconds) * time.Second

	p.MediaURLs = mediaURLs
	p.SizeCm = sizeCM
	p.AgeMonths = ageMonths
	p.Gender = gender
	p.Breeder = breeder
	p.Bloodline = bloodline
	p.Certificates = certificates
	p.FarmAddressID = farmAddressID
	p.PreparationNote = preparationNote
	p.CreatedAt = productCreatedAt
	p.UpdatedAt = productUpdatedAt
	a.Product = &p

	return &a, nil
}

// GetByID retrieves an auction without locking (for read-only operations).
func (r *AuctionRepository) GetByID(
	ctx context.Context,
	tx db.Tx,
	auctionID uuid.UUID,
) (*entity.Auction, error) {
	row := tx.QueryRow(ctx, `
		SELECT `+joinedAuctionColumns+`
		FROM auctions a
		JOIN products p ON p.id = a.product_id
		WHERE a.id = $1
	`, auctionID)

	auction, err := scanJoinedAuction(row)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, fmt.Errorf("auction not found: %s", auctionID)
		}
		return nil, fmt.Errorf("get auction failed: %w", err)
	}
	return auction, nil
}

// GetForUpdate retrieves an auction with FOR UPDATE lock.
// This prevents concurrent modifications and must be used within a transaction.
func (r *AuctionRepository) GetForUpdate(
	ctx context.Context,
	tx db.Tx,
	auctionID uuid.UUID,
) (*entity.Auction, error) {
	row := tx.QueryRow(ctx, `
		SELECT `+joinedAuctionColumns+`
		FROM auctions a
		JOIN products p ON p.id = a.product_id
		WHERE a.id = $1
		FOR UPDATE
	`, auctionID)

	auction, err := scanJoinedAuction(row)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, fmt.Errorf("auction not found: %s", auctionID)
		}
		return nil, fmt.Errorf("get auction for update failed: %w", err)
	}
	return auction, nil
}

// UpdateTx persists auction state changes within a transaction.
// Must be called after GetForUpdate within the same transaction.
// Product content (title, description, koi attributes, media) is updated
// through the Product repository — never through the auctions table.
func (r *AuctionRepository) UpdateTx(
	ctx context.Context,
	tx db.Tx,
	auction *entity.Auction,
) error {
	_, err := tx.Exec(ctx, `
		UPDATE auctions
		SET order_id = $2, settlement_deadline = $3,
		    start_price = $4, bid_increment = $5, buy_now_price = $6,
		    start_at = $7, end_at = $8,
		    current_bid = $9, current_winner_id = $10,
		    status = $11, updated_at = $12, anti_snipe_extension_seconds = $13
		WHERE id = $1
	`,
		auction.ID,
		auction.OrderID,
		auction.SettlementDeadline,
		auction.StartPrice,
		auction.BidIncrement,
		auction.BuyNowPrice,
		auction.StartAt,
		auction.EndAt,
		auction.CurrentBid,
		auction.CurrentWinnerID,
		string(auction.Status),
		auction.UpdatedAt,
		int64(auction.AntiSnipeExtensionTotal/time.Second),
	)

	if err != nil {
		return fmt.Errorf("update auction failed: %w", err)
	}

	return nil
}

// ListActiveToEnd retrieves active auctions that have ended.
// Uses FOR UPDATE SKIP LOCKED for concurrent-safe batch processing.
// This is used by the auction end worker.
func (r *AuctionRepository) ListActiveToEnd(
	ctx context.Context,
	tx db.Tx,
	now time.Time,
	limit int,
) ([]*entity.Auction, error) {
	query := `
		SELECT ` + joinedAuctionColumns + `
		FROM auctions a
		JOIN products p ON p.id = a.product_id
		WHERE a.status = 'active'
		  AND a.end_at <= $1
		FOR UPDATE SKIP LOCKED
		LIMIT $2
	`

	rows, err := tx.Query(ctx, query, now, limit)
	if err != nil {
		return nil, fmt.Errorf("list active to end failed: %w", err)
	}
	defer rows.Close()

	auctions, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*entity.Auction, error) {
		return scanJoinedAuction(row)
	})

	if err != nil {
		return nil, fmt.Errorf("scan auctions failed: %w", err)
	}

	return auctions, nil
}

// GetByIdempotencyKey retrieves an auction by idempotency key without locking.
// Returns nil if no auction found with the given key.
// This is used for idempotent bid operations.
func (r *AuctionRepository) GetByAuctionAndIdempotencyKey(
	ctx context.Context,
	tx db.Tx,
	auctionID uuid.UUID,
	idempotencyKey string,
) (*entity.AuctionBid, error) {
	// This actually queries auction_bids, not auctions
	// But for simplicity, let's put this in AuctionBidRepository
	return nil, fmt.Errorf("use AuctionBidRepository.GetByAuctionAndIdempotencyKey instead")
}

// AuctionFilter holds filter criteria for listing auctions.
type AuctionFilter struct {
	Status   *entity.Status // Filter by status (optional)
	SellerID *uuid.UUID     // Filter by seller ID (optional)
	Cursor   *time.Time     // Cursor for pagination (created_at based)
	Limit    int            // Max results (default 20, max 50)
}

// List retrieves auctions with filtering and cursor-based pagination.
// This is a read-only query with no locks.
// Orders by created_at DESC for newest-first pagination.
//
// Seller governance: JOIN users enforces that only auctions from active,
// non-deleted sellers are returned. This mirrors the listing browse pattern
// (for_sale_repository_impl.go) and prevents discovery of content from
// suspended/banned/deleted sellers.
func (r *AuctionRepository) List(
	ctx context.Context,
	tx db.Tx,
	filter AuctionFilter,
) ([]*entity.Auction, error) {
	// Set default limit
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	// Build query with dynamic WHERE clause.
	// JOIN users enforces seller governance at SQL level.
	query := `
		SELECT ` + joinedAuctionColumns + `
		FROM auctions a
		JOIN products p ON p.id = a.product_id
		JOIN users u ON u.id = a.seller_id
	`
	// Seller governance conditions — always applied.
	conditions := []string{
		"u.account_status = 'active'",
		"u.deleted_at IS NULL",
	}
	var args []interface{}
	argIdx := 1

	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("a.status = $%d", argIdx))
		args = append(args, string(*filter.Status))
		argIdx++
	} else {
		// Default browse (no explicit status filter) is public discovery:
		// only pre-sale/live-sale states. Draft, cancelled, waiting_settlement,
		// ended (settled/no-winner) and expired_bnr are owned/historical
		// surfaces and must not surface in anonymous public browse.
		conditions = append(conditions, "a.status IN ('scheduled', 'active')")
	}

	if filter.SellerID != nil {
		conditions = append(conditions, fmt.Sprintf("a.seller_id = $%d", argIdx))
		args = append(args, *filter.SellerID)
		argIdx++
	}

	if filter.Cursor != nil {
		conditions = append(conditions, fmt.Sprintf("a.created_at < $%d", argIdx))
		args = append(args, *filter.Cursor)
		argIdx++
	}

	// WHERE clause always present (seller governance conditions are mandatory).
	query += " WHERE " + conditions[0]
	for i := 1; i < len(conditions); i++ {
		query += " AND " + conditions[i]
	}

	// Add ORDER BY and LIMIT
	// Use LIMIT + 1 to detect if there are more results
	query += " ORDER BY a.created_at DESC LIMIT $" + fmt.Sprintf("%d", argIdx)
	args = append(args, limit+1)

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list auctions failed: %w", err)
	}
	defer rows.Close()

	auctions, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*entity.Auction, error) {
		return scanJoinedAuction(row)
	})

	if err != nil {
		return nil, fmt.Errorf("scan auctions failed: %w", err)
	}

	return auctions, nil
}

// GetAuctionStatusByProductID retrieves the status of an auction for a product.
// Returns nil if no auction exists for this product.
func (r *AuctionRepository) GetAuctionStatusByProductID(
	ctx context.Context,
	tx db.Tx,
	productID uuid.UUID,
) (*entity.Status, error) {
	var status entity.Status

	err := tx.QueryRow(ctx, `
		SELECT status
		FROM auctions
		WHERE product_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, productID).Scan(&status)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil // No auction found for this product
		}
		return nil, fmt.Errorf("get auction status failed: %w", err)
	}

	return &status, nil
}
