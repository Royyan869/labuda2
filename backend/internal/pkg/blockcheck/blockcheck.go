// Package blockcheck provides shared bidirectional block checking for handlers.
//
// Pattern: inline SQL on user_blocks table, matching the canonical approach
// used throughout the codebase (ContentHandler.checkBidirectionalBlock,
// auction_bids_viewercontext.hydrateBidsBlockedSet, etc.).
//
// Block is BIDIRECTIONAL: if A blocked B or B blocked A, both see the block.
// Anonymous viewers (uuid.Nil) are never blocked.
package blockcheck

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/db"
)

// IsBidirectionallyBlocked checks if a block exists in either direction between
// viewerID and targetID. Returns false for anonymous viewers (uuid.Nil).
// Fail-open: returns false on infrastructure error (logged by caller).
func IsBidirectionallyBlocked(ctx context.Context, tx db.Tx, viewerID, targetID uuid.UUID) (bool, error) {
	if viewerID == uuid.Nil {
		return false, nil
	}
	if targetID == uuid.Nil {
		return false, nil
	}
	const query = `
		SELECT EXISTS(
			SELECT 1 FROM user_blocks
			WHERE (blocker_id = $1 AND blocked_id = $2)
			   OR (blocker_id = $2 AND blocked_id = $1)
		)
	`
	rows, err := tx.Query(ctx, query, viewerID, targetID)
	if err != nil {
		return false, fmt.Errorf("block check query failed: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		var blocked bool
		if err := rows.Scan(&blocked); err != nil {
			return false, fmt.Errorf("block check scan failed: %w", err)
		}
		return blocked, nil
	}
	return false, rows.Err()
}

// BlockedSet returns the set of target IDs (from candidates) that have a
// bidirectional block relationship with viewerID. Used for batch filtering
// in list/search endpoints. Returns empty set for anonymous viewers.
func BlockedSet(ctx context.Context, tx db.Tx, viewerID uuid.UUID, candidates []uuid.UUID) (map[uuid.UUID]bool, error) {
	result := make(map[uuid.UUID]bool)
	if viewerID == uuid.Nil || len(candidates) == 0 {
		return result, nil
	}
	const query = `
		SELECT DISTINCT
			CASE
				WHEN blocker_id = $1 THEN blocked_id
				ELSE blocker_id
			END AS blocked_author
		FROM user_blocks
		WHERE (blocker_id = $1 AND blocked_id = ANY($2))
		   OR (blocker_id = ANY($2) AND blocked_id = $1)
	`
	rows, err := tx.Query(ctx, query, viewerID, candidates)
	if err != nil {
		return result, fmt.Errorf("blocked set query failed: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return result, fmt.Errorf("blocked set scan failed: %w", err)
		}
		result[id] = true
	}
	return result, rows.Err()
}


