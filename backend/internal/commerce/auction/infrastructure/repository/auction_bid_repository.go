package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/commerce/auction/entity"
	"github.com/labuda/backend/pkg/db"
)

// AuctionBidRepository handles auction bid persistence using pgx-based DB layer.
type AuctionBidRepository struct{}

// NewAuctionBidRepository creates a new AuctionBidRepository.
func NewAuctionBidRepository() *AuctionBidRepository {
	return &AuctionBidRepository{}
}

// CreateTx persists a new auction bid within a transaction.
func (r *AuctionBidRepository) CreateTx(
	ctx context.Context,
	tx db.Tx,
	bid *entity.AuctionBid,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO auction_bids (
			id, auction_id, bidder_id, amount, idempotency_key, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`,
		bid.ID,
		bid.AuctionID,
		bid.BidderID,
		bid.Amount,
		bid.IdempotencyKey,
		bid.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("create auction bid failed: %w", err)
	}

	return nil
}

// GetByID retrieves an auction bid without locking.
func (r *AuctionBidRepository) GetByID(
	ctx context.Context,
	tx db.Tx,
	bidID uuid.UUID,
) (*entity.AuctionBid, error) {
	var id, auctionID, bidderID uuid.UUID
	var amount int64
	var idempotencyKey string
	var createdAt time.Time

	err := tx.QueryRow(ctx, `
		SELECT id, auction_id, bidder_id, amount, idempotency_key, created_at
		FROM auction_bids
		WHERE id = $1
	`, bidID).Scan(
		&id, &auctionID, &bidderID, &amount, &idempotencyKey, &createdAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, fmt.Errorf("auction bid not found: %s", bidID)
		}
		return nil, fmt.Errorf("get auction bid failed: %w", err)
	}

	return &entity.AuctionBid{
		ID:             id,
		AuctionID:      auctionID,
		BidderID:       bidderID,
		Amount:         amount,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      createdAt,
	}, nil
}

// GetByAuctionAndIdempotencyKey retrieves a bid by auction, bidder, and idempotency key.
// Scoped to bidder so two different bidders using the same key string are independent.
// Returns nil if not found (used for idempotency check).
func (r *AuctionBidRepository) GetByAuctionAndIdempotencyKey(
	ctx context.Context,
	tx db.Tx,
	auctionID uuid.UUID,
	bidderID uuid.UUID,
	idempotencyKey string,
) (*entity.AuctionBid, error) {
	var id uuid.UUID
	var amount int64
	var createdAt time.Time

	err := tx.QueryRow(ctx, `
		SELECT id, amount, created_at
		FROM auction_bids
		WHERE auction_id = $1 AND bidder_id = $2 AND idempotency_key = $3
	`, auctionID, bidderID, idempotencyKey).Scan(
		&id, &amount, &createdAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil // No bid with this idempotency key for this bidder
		}
		return nil, fmt.Errorf("get auction bid by idempotency key failed: %w", err)
	}

	return &entity.AuctionBid{
		ID:             id,
		AuctionID:      auctionID,
		BidderID:       bidderID,
		Amount:         amount,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      createdAt,
	}, nil
}

// ListByAuction retrieves all bids for an auction, ordered by creation time.
func (r *AuctionBidRepository) ListByAuction(
	ctx context.Context,
	tx db.Tx,
	auctionID uuid.UUID,
	limit int,
) ([]*entity.AuctionBid, error) {
	query := `
		SELECT id, auction_id, bidder_id, amount, idempotency_key, created_at
		FROM auction_bids
		WHERE auction_id = $1
		ORDER BY created_at DESC
	`
	if limit > 0 {
		query += " LIMIT $2"
		rows, err := tx.Query(ctx, query, auctionID, limit)
		if err != nil {
			return nil, fmt.Errorf("list auction bids failed: %w", err)
		}
		defer rows.Close()
		return r.collectRows(rows, auctionID)
	}

	rows, err := tx.Query(ctx, query, auctionID)
	if err != nil {
		return nil, fmt.Errorf("list auction bids failed: %w", err)
	}
	defer rows.Close()

	return r.collectRows(rows, auctionID)
}

// collectRows scans rows into AuctionBid entities.
func (r *AuctionBidRepository) collectRows(rows pgx.Rows, auctionID uuid.UUID) ([]*entity.AuctionBid, error) {
	bids, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*entity.AuctionBid, error) {
		var id, bidderID uuid.UUID
		var amount int64
		var idempotencyKey string
		var createdAt time.Time

		err := row.Scan(&id, &auctionID, &bidderID, &amount, &idempotencyKey, &createdAt)
		if err != nil {
			return nil, err
		}

		return &entity.AuctionBid{
			ID:             id,
			AuctionID:      auctionID,
			BidderID:       bidderID,
			Amount:         amount,
			IdempotencyKey: idempotencyKey,
			CreatedAt:      createdAt,
		}, nil
	})

	if err != nil {
		return nil, fmt.Errorf("scan auction bids failed: %w", err)
	}

	return bids, nil
}

// ListAuctionIDsByBidder lists distinct auctions where user has placed bids,
// ordered by most recent bid activity.
func (r *AuctionBidRepository) ListAuctionIDsByBidder(
	ctx context.Context,
	tx db.Tx,
	bidderID uuid.UUID,
) ([]uuid.UUID, error) {
	query := `
		SELECT auction_id
		FROM (
			SELECT auction_id, MAX(created_at) as last_bid_at
			FROM auction_bids
			WHERE bidder_id = $1
			GROUP BY auction_id
		) t
		ORDER BY last_bid_at DESC
	`

	rows, err := tx.Query(ctx, query, bidderID)
	if err != nil {
		return nil, fmt.Errorf("list auction IDs by bidder failed: %w", err)
	}
	defer rows.Close()

	auctionIDs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (uuid.UUID, error) {
		var auctionID uuid.UUID
		err := row.Scan(&auctionID)
		if err != nil {
			return uuid.Nil, err
		}
		return auctionID, nil
	})

	if err != nil {
		return nil, fmt.Errorf("scan auction IDs failed: %w", err)
	}

	return auctionIDs, nil
}

// GetUserLastBidForAuction retrieves the user's highest bid for an auction.
// Returns nil if the user has not bid on this auction.
func (r *AuctionBidRepository) GetUserLastBidForAuction(
	ctx context.Context,
	tx db.Tx,
	bidderID uuid.UUID,
	auctionID uuid.UUID,
) (*entity.AuctionBid, error) {
	var id, bidAuctionID, bidBidderID uuid.UUID
	var amount int64
	var idempotencyKey string
	var createdAt time.Time

	err := tx.QueryRow(ctx, `
		SELECT id, auction_id, bidder_id, amount, idempotency_key, created_at
		FROM auction_bids
		WHERE bidder_id = $1
		  AND auction_id = $2
		ORDER BY amount DESC, created_at DESC
		LIMIT 1
	`, bidderID, auctionID).Scan(
		&id, &bidAuctionID, &bidBidderID, &amount, &idempotencyKey, &createdAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil // No bid found
		}
		return nil, fmt.Errorf("get user last bid for auction failed: %w", err)
	}

	return &entity.AuctionBid{
		ID:             id,
		AuctionID:      bidAuctionID,
		BidderID:       bidBidderID,
		Amount:         amount,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      createdAt,
	}, nil
}


