package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	platformevent "github.com/labuda/backend/internal/platform/event"
	dbpkg "github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// BNRStrikeHandler records a strike row in buyer_bnr_strikes when
// auction_bnr_detected fires. Idempotent via UNIQUE(auction_id).
//
// SCOPE: recording only. No enforcement, no decay, no notification.
type BNRStrikeHandler struct {
	db  dbpkg.Transactor
	log *zap.Logger
}

// NewBNRStrikeHandler creates the handler.
func NewBNRStrikeHandler(db dbpkg.Transactor, log *zap.Logger) *BNRStrikeHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &BNRStrikeHandler{db: db, log: log}
}

// Handle processes an auction_bnr_detected event.
//
// Payload shape (emitted by AuctionSettlementWorker):
//
//	{ "auction_id": "...", "winner_id": "...", "seller_id": "...", "timestamp": "..." }
func (h *BNRStrikeHandler) Handle(ctx context.Context, event platformevent.OutboxEvent) error {
	var p AuctionBNRDetectedEvent
	if err := json.Unmarshal(event.Payload, &p); err != nil {
		h.log.Error("bnr_strike: invalid payload",
			zap.String("event_id", event.ID.String()),
			zap.Error(err),
		)
		// Nil → don't retry on malformed payload.
		return nil
	}

	if p.AuctionID == uuid.Nil || p.WinnerID == uuid.Nil {
		h.log.Error("bnr_strike: missing auction_id or winner_id",
			zap.String("event_id", event.ID.String()),
		)
		return nil
	}

	err := h.db.WithTx(ctx, func(tx dbpkg.Tx) error {
		return h.insertStrike(ctx, tx, p.WinnerID, p.AuctionID)
	})
	if err != nil {
		return fmt.Errorf("bnr_strike: insert: %w", err)
	}

	h.log.Info("bnr_strike: recorded",
		zap.String("buyer_id", p.WinnerID.String()),
		zap.String("auction_id", p.AuctionID.String()),
	)
	return nil
}

// insertStrike inserts a single strike row. ON CONFLICT DO NOTHING makes
// replays safe — the same auction can only produce one strike.
func (h *BNRStrikeHandler) insertStrike(ctx context.Context, tx dbpkg.Tx, buyerID, auctionID uuid.UUID) error {
	const q = `
		INSERT INTO buyer_bnr_strikes (buyer_id, auction_id)
		VALUES ($1, $2)
		ON CONFLICT (auction_id) DO NOTHING
	`
	_, err := tx.Exec(ctx, q, buyerID, auctionID)
	return err
}


