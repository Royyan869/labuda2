package worker

// BNRAdminResetter performs admin-initiated BNR strike resets.
//
// Reset sets admin_reset = TRUE on active strikes. Rows are preserved
// for audit — never deleted. Decayed and already-reset rows are excluded.

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/identity/auth"
	dbpkg "github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// BNRAdminResetter handles admin reset of BNR strikes.
type BNRAdminResetter struct {
	db  dbpkg.Transactor
	log *zap.Logger
}

// ErrBNRAdminActorRequired is returned when the caller provenance is absent
// or not an explicit admin actor.
type ErrBNRAdminActorRequired struct {
	Operation string
}

func (e *ErrBNRAdminActorRequired) Error() string {
	return "bnr admin reset requires explicit admin actor: " + e.Operation
}

// NewBNRAdminResetter creates a new resetter.
func NewBNRAdminResetter(db dbpkg.Transactor, log *zap.Logger) *BNRAdminResetter {
	if log == nil {
		log = zap.NewNop()
	}
	return &BNRAdminResetter{db: db, log: log}
}

// ResetAllForBuyer sets admin_reset = TRUE on all active strikes for a buyer.
// Active = decayed_at IS NULL AND admin_reset = FALSE.
// Returns the number of strikes reset.
func (r *BNRAdminResetter) ResetAllForBuyer(ctx context.Context, buyerID, actorID uuid.UUID) (int64, error) {
	if actorID == uuid.Nil || auth.IsSystemCaller(actorID) {
		return 0, &ErrBNRAdminActorRequired{Operation: "reset_all_for_buyer"}
	}
	var count int64
	err := r.db.WithTx(ctx, func(tx dbpkg.Tx) error {
		const q = `
			UPDATE buyer_bnr_strikes
			SET admin_reset = TRUE
			WHERE buyer_id = $1
			  AND decayed_at IS NULL
			  AND admin_reset = FALSE
		`
		tag, err := tx.Exec(ctx, q, buyerID)
		if err != nil {
			return fmt.Errorf("bnr_admin_reset: update all: %w", err)
		}
		count = tag.RowsAffected()
		return nil
	})
	if err != nil {
		return 0, err
	}

	r.log.Info("bnr_admin_reset: buyer strikes reset",
		zap.String("buyer_id", buyerID.String()),
		zap.String("actor_id", actorID.String()),
		zap.Int64("count", count),
	)
	return count, nil
}

// ResetStrike sets admin_reset = TRUE on a single strike by ID.
// Returns true if the strike was updated, false if it was already reset,
// decayed, or does not exist.
func (r *BNRAdminResetter) ResetStrike(ctx context.Context, strikeID, actorID uuid.UUID) (bool, error) {
	if actorID == uuid.Nil || auth.IsSystemCaller(actorID) {
		return false, &ErrBNRAdminActorRequired{Operation: "reset_single_strike"}
	}
	var updated bool
	err := r.db.WithTx(ctx, func(tx dbpkg.Tx) error {
		const q = `
			UPDATE buyer_bnr_strikes
			SET admin_reset = TRUE
			WHERE id = $1
			  AND decayed_at IS NULL
			  AND admin_reset = FALSE
		`
		tag, err := tx.Exec(ctx, q, strikeID)
		if err != nil {
			return fmt.Errorf("bnr_admin_reset: update single: %w", err)
		}
		updated = tag.RowsAffected() > 0
		return nil
	})
	if err == nil {
		r.log.Info("bnr_admin_reset: strike reset",
			zap.String("strike_id", strikeID.String()),
			zap.String("actor_id", actorID.String()),
			zap.Bool("updated", updated),
		)
	}
	return updated, err
}


