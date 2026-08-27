package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/db"
)

// BNRRestrictionResult holds the outcome of a BNR strike check.
type BNRRestrictionResult struct {
	Allowed          bool
	ActiveStrikes    int
	PermanentBan     bool
	RestrictionUntil *time.Time // nil when allowed or permanently banned
	Warning          string     // non-empty for 1-strike warning
}

// BNRStrikeChecker queries buyer_bnr_strikes to determine auction restriction.
//
// RULES (owner-approved):
//
//	0 strikes → allow
//	1 strike  → allow + warning
//	2 strikes → deny if last_struck_at + 14d > now
//	3 strikes → deny if last_struck_at + 90d > now
//	4+ strikes → deny permanent
//
// SCOPE: auction bidding only. Does not affect listing purchase, chat, social.
type BNRStrikeChecker struct{}

// NewBNRStrikeChecker creates a new checker.
func NewBNRStrikeChecker() *BNRStrikeChecker {
	return &BNRStrikeChecker{}
}

// Check queries active BNR strikes for a buyer and returns the restriction result.
// The check runs in the caller's transaction (same tx as PlaceBid).
func (c *BNRStrikeChecker) Check(ctx context.Context, tx db.Tx, buyerID uuid.UUID) (*BNRRestrictionResult, error) {
	count, lastStruckAt, err := c.queryActiveStrikes(ctx, tx, buyerID)
	if err != nil {
		return nil, fmt.Errorf("bnr_restriction: query strikes: %w", err)
	}

	return c.evaluate(count, lastStruckAt, time.Now()), nil
}

// evaluate applies the owner-approved punishment table.
// Separated from DB for testability.
func (c *BNRStrikeChecker) evaluate(count int, lastStruckAt *time.Time, now time.Time) *BNRRestrictionResult {
	switch {
	case count == 0:
		return &BNRRestrictionResult{Allowed: true}

	case count == 1:
		return &BNRRestrictionResult{
			Allowed:       true,
			ActiveStrikes: 1,
			Warning:       "Anda memiliki 1 pelanggaran lelang yang tidak dibayar. Pelanggaran berikutnya akan mengakibatkan pembatasan.",
		}

	case count == 2:
		until := lastStruckAt.Add(14 * 24 * time.Hour)
		if now.Before(until) {
			return &BNRRestrictionResult{
				Allowed:          false,
				ActiveStrikes:    2,
				RestrictionUntil: &until,
			}
		}
		// 14d elapsed — allow with warning
		return &BNRRestrictionResult{
			Allowed:       true,
			ActiveStrikes: 2,
			Warning:       "Pembatasan lelang 14 hari Anda telah berakhir. Pelanggaran berikutnya akan mengakibatkan pembatasan 90 hari.",
		}

	case count == 3:
		until := lastStruckAt.Add(90 * 24 * time.Hour)
		if now.Before(until) {
			return &BNRRestrictionResult{
				Allowed:          false,
				ActiveStrikes:    3,
				RestrictionUntil: &until,
			}
		}
		// 90d elapsed — allow with warning
		return &BNRRestrictionResult{
			Allowed:       true,
			ActiveStrikes: 3,
			Warning:       "Pembatasan lelang 90 hari Anda telah berakhir. Pelanggaran berikutnya akan mengakibatkan larangan permanen.",
		}

	default: // 4+
		return &BNRRestrictionResult{
			Allowed:       false,
			ActiveStrikes: count,
			PermanentBan:  true,
		}
	}
}

// queryActiveStrikes returns the count of active (non-decayed, non-reset) strikes
// and the most recent struck_at timestamp for a buyer.
func (c *BNRStrikeChecker) queryActiveStrikes(ctx context.Context, tx db.Tx, buyerID uuid.UUID) (int, *time.Time, error) {
	const q = `
		SELECT COUNT(*), MAX(struck_at)
		FROM buyer_bnr_strikes
		WHERE buyer_id = $1
		  AND decayed_at IS NULL
		  AND admin_reset = FALSE
	`
	var count int
	var lastStruckAt *time.Time
	err := tx.QueryRow(ctx, q, buyerID).Scan(&count, &lastStruckAt)
	if err != nil {
		return 0, nil, err
	}
	return count, lastStruckAt, nil
}


