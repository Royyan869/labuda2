package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	negotiationEntity "github.com/labuda/backend/internal/commerce/negotiation/entity"
	negotiationRepo "github.com/labuda/backend/internal/commerce/negotiation/repository"
	"github.com/labuda/backend/pkg/db"
)

// NegotiationRepositoryImpl handles negotiation persistence using pgx-based DB layer.
//
// NEGOTIATION → CHAT UNIFICATION:
// - Message-related methods removed (handled by ChatService)
// - chat_room_id added to all queries
//
// PRICE SECURITY HARDENING:
// - proposal_sequence added for optimistic locking
// - price_history methods for audit trail
type NegotiationRepositoryImpl struct{}

// NewNegotiationRepository creates a new NegotiationRepository.
func NewNegotiationRepository() negotiationRepo.Repository {
	return &NegotiationRepositoryImpl{}
}

// CreateSession persists a new negotiation session within a transaction.
func (r *NegotiationRepositoryImpl) CreateSession(
	ctx context.Context,
	tx db.Tx,
	session *negotiationEntity.NegotiationSession,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO negotiation_sessions (
			id, resource_type, for_sale_id, buyer_id, seller_id, chat_room_id,
			status, expires_at, current_price, accepted_price, accepted_at,
			proposal_sequence, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`,
		session.ID,
		string(session.ResourceType),
		session.ForSaleID,
		session.BuyerID,
		session.SellerID,
		session.ChatRoomID,
		string(session.Status),
		session.ExpiresAt,
		session.CurrentPrice,
		session.AcceptedPrice,
		session.AcceptedAt,
		session.ProposalSequence,
		session.CreatedAt,
		session.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("create negotiation session failed: %w", err)
	}

	return nil
}

// GetSession retrieves a session without locking (for read-only operations).
func (r *NegotiationRepositoryImpl) GetSession(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (*negotiationEntity.NegotiationSession, error) {
	var session negotiationEntity.NegotiationSession
	var resourceType, status string

	err := tx.QueryRow(ctx, `
		SELECT id, resource_type, for_sale_id, buyer_id, seller_id,
		       chat_room_id, status, order_id, expires_at, current_price, accepted_price, accepted_at,
		       proposal_sequence, created_at, updated_at
		FROM negotiation_sessions
		WHERE id = $1
	`, id).Scan(
		&session.ID, &resourceType, &session.ForSaleID,
		&session.BuyerID, &session.SellerID, &session.ChatRoomID,
		&status, &session.OrderID, &session.ExpiresAt,
		&session.CurrentPrice, &session.AcceptedPrice, &session.AcceptedAt,
		&session.ProposalSequence,
		&session.CreatedAt, &session.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("negotiation session not found: %s", id)
		}
		return nil, fmt.Errorf("get negotiation session failed: %w", err)
	}

	session.ResourceType = negotiationEntity.NegotiationResourceType(resourceType)
	session.Status = negotiationEntity.NegotiationStatus(status)

	return &session, nil
}

// GetSessionForUpdate retrieves a session with FOR UPDATE lock.
func (r *NegotiationRepositoryImpl) GetSessionForUpdate(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (*negotiationEntity.NegotiationSession, error) {
	var session negotiationEntity.NegotiationSession
	var resourceType, status string

	err := tx.QueryRow(ctx, `
		SELECT id, resource_type, for_sale_id, buyer_id, seller_id,
		       chat_room_id, status, order_id, expires_at, current_price, accepted_price, accepted_at,
		       proposal_sequence, created_at, updated_at
		FROM negotiation_sessions
		WHERE id = $1
		FOR UPDATE
	`, id).Scan(
		&session.ID, &resourceType, &session.ForSaleID,
		&session.BuyerID, &session.SellerID, &session.ChatRoomID,
		&status, &session.OrderID, &session.ExpiresAt,
		&session.CurrentPrice, &session.AcceptedPrice, &session.AcceptedAt,
		&session.ProposalSequence,
		&session.CreatedAt, &session.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("negotiation session not found: %s", id)
		}
		return nil, fmt.Errorf("get negotiation session for update failed: %w", err)
	}

	session.ResourceType = negotiationEntity.NegotiationResourceType(resourceType)
	session.Status = negotiationEntity.NegotiationStatus(status)

	return &session, nil
}

// GetActiveSessionByResourceAndBuyer retrieves the active/accepted-unsettled session for a given resource and buyer.
// Canonical slot predicate: status IN ('active','accepted') AND order_id IS NULL.
// This matches the partial unique index ux_negotiation_one_active_per_buyer_for_sale and is the
// early/fast-path check; the index remains the final concurrency authority.
func (r *NegotiationRepositoryImpl) GetActiveSessionByResourceAndBuyer(
	ctx context.Context,
	tx db.Tx,
	resourceType negotiationEntity.NegotiationResourceType,
	resourceID, buyerID uuid.UUID,
) (*negotiationEntity.NegotiationSession, error) {
	var session negotiationEntity.NegotiationSession
	var status string

	err := tx.QueryRow(ctx, `
		SELECT id, resource_type, for_sale_id, buyer_id, seller_id,
		       chat_room_id, status, order_id, expires_at, current_price, accepted_price, accepted_at,
		       proposal_sequence, created_at, updated_at
		FROM negotiation_sessions
		WHERE resource_type = $1
		  AND for_sale_id = $2
		  AND buyer_id = $3
		  AND status IN ('active', 'accepted')
		  AND order_id IS NULL
	`, string(resourceType), resourceID, buyerID).Scan(
		&session.ID, &session.ResourceType, &session.ForSaleID,
		&session.BuyerID, &session.SellerID, &session.ChatRoomID,
		&status, &session.OrderID, &session.ExpiresAt,
		&session.CurrentPrice, &session.AcceptedPrice, &session.AcceptedAt,
		&session.ProposalSequence,
		&session.CreatedAt, &session.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // No active session found
		}
		return nil, fmt.Errorf("get active session by resource and buyer failed: %w", err)
	}

	session.Status = negotiationEntity.NegotiationStatus(status)

	return &session, nil
}

// UpdateSession persists session changes within a transaction.
func (r *NegotiationRepositoryImpl) UpdateSession(
	ctx context.Context,
	tx db.Tx,
	session *negotiationEntity.NegotiationSession,
) error {
	_, err := tx.Exec(ctx, `
		UPDATE negotiation_sessions
		SET chat_room_id = $2, status = $3, order_id = $4, expires_at = $5,
		    current_price = $6, accepted_price = $7, accepted_at = $8,
		    proposal_sequence = $9, updated_at = $10
		WHERE id = $1
	`,
		session.ID,
		session.ChatRoomID,
		string(session.Status),
		session.OrderID,
		session.ExpiresAt,
		session.CurrentPrice,
		session.AcceptedPrice,
		session.AcceptedAt,
		session.ProposalSequence,
		session.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("update negotiation session failed: %w", err)
	}

	return nil
}

// GetExpiredSessions retrieves active and accepted negotiations that have passed their expires_at time.
// Uses FOR UPDATE SKIP LOCKED for concurrent worker support.
// Returns up to limit sessions that should be expired.
//
// N5 LIFECYCLE FINALIZATION: Settled negotiations (order_id IS NOT NULL) are terminal
// and must not be expired. Candidate predicate is the single expiry authority.
// NEGOTIATION EXPIRY CONSISTENCY: Includes both active and accepted (unsettled) sessions to prevent
// "accepted but expired" state which allows checkout of stale agreements.
func (r *NegotiationRepositoryImpl) GetExpiredSessions(
	ctx context.Context,
	tx db.Tx,
	limit int,
) ([]*negotiationEntity.NegotiationSession, error) {
	// Use FOR UPDATE SKIP LOCKED to allow concurrent workers
	// Each worker will get different rows to process
	rows, err := tx.Query(ctx, `
		SELECT id, resource_type, for_sale_id, buyer_id, seller_id,
		       chat_room_id, status, order_id, expires_at, current_price, accepted_price, accepted_at,
		       proposal_sequence, created_at, updated_at
		FROM negotiation_sessions
		WHERE status IN ('active', 'accepted')
		  AND order_id IS NULL
		  AND expires_at IS NOT NULL
		  AND expires_at < NOW()
		FOR UPDATE SKIP LOCKED
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("get expired sessions failed: %w", err)
	}
	defer rows.Close()

	var sessions []*negotiationEntity.NegotiationSession
	for rows.Next() {
		var session negotiationEntity.NegotiationSession
		var resourceType, status string

		err := rows.Scan(
			&session.ID, &resourceType, &session.ForSaleID,
			&session.BuyerID, &session.SellerID, &session.ChatRoomID,
			&status, &session.OrderID, &session.ExpiresAt,
			&session.CurrentPrice, &session.AcceptedPrice, &session.AcceptedAt,
			&session.ProposalSequence,
			&session.CreatedAt, &session.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan expired session failed: %w", err)
		}

		session.ResourceType = negotiationEntity.NegotiationResourceType(resourceType)
		session.Status = negotiationEntity.NegotiationStatus(status)

		sessions = append(sessions, &session)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate expired sessions failed: %w", rows.Err())
	}

	return sessions, nil
}

// ============================================================================
// PRICE SECURITY AUDIT: Price history methods
// ============================================================================

// CreatePriceHistoryEntry creates a new price history entry.
func (r *NegotiationRepositoryImpl) CreatePriceHistoryEntry(
	ctx context.Context,
	tx db.Tx,
	history *negotiationEntity.NegotiationPriceHistory,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO negotiation_price_history (
			id, session_id, proposal_sequence, old_price, new_price,
			changed_by_user_id, change_reason, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`,
		history.ID,
		history.SessionID,
		history.ProposalSequence,
		history.OldPrice,
		history.NewPrice,
		history.ChangedByUserID,
		history.ChangeReason,
		history.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("create price history entry failed: %w", err)
	}

	return nil
}

// GetPriceHistoryBySession retrieves all price history entries for a session,
// ordered by created_at DESC (most recent first).
func (r *NegotiationRepositoryImpl) GetPriceHistoryBySession(
	ctx context.Context,
	tx db.Tx,
	sessionID uuid.UUID,
) ([]*negotiationEntity.NegotiationPriceHistory, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, session_id, proposal_sequence, old_price, new_price,
		       changed_by_user_id, change_reason, created_at
		FROM negotiation_price_history
		WHERE session_id = $1
		ORDER BY created_at DESC
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get price history by session failed: %w", err)
	}
	defer rows.Close()

	var history []*negotiationEntity.NegotiationPriceHistory
	for rows.Next() {
		var h negotiationEntity.NegotiationPriceHistory
		err := rows.Scan(
			&h.ID, &h.SessionID, &h.ProposalSequence,
			&h.OldPrice, &h.NewPrice,
			&h.ChangedByUserID, &h.ChangeReason, &h.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan price history entry failed: %w", err)
		}
		history = append(history, &h)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate price history failed: %w", rows.Err())
	}

	return history, nil
}

// GetPriceHistoryByUser retrieves all price history entries for a user,
// ordered by created_at DESC (most recent first).
func (r *NegotiationRepositoryImpl) GetPriceHistoryByUser(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	limit int,
) ([]*negotiationEntity.NegotiationPriceHistory, error) {
	rows, err := tx.Query(ctx, `
		SELECT ph.id, ph.session_id, ph.proposal_sequence, ph.old_price, ph.new_price,
		       ph.changed_by_user_id, ph.change_reason, ph.created_at
		FROM negotiation_price_history ph
		WHERE ph.changed_by_user_id = $1
		ORDER BY ph.created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("get price history by user failed: %w", err)
	}
	defer rows.Close()

	var history []*negotiationEntity.NegotiationPriceHistory
	for rows.Next() {
		var h negotiationEntity.NegotiationPriceHistory
		err := rows.Scan(
			&h.ID, &h.SessionID, &h.ProposalSequence,
			&h.OldPrice, &h.NewPrice,
			&h.ChangedByUserID, &h.ChangeReason, &h.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan price history entry failed: %w", err)
		}
		history = append(history, &h)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate price history failed: %w", rows.Err())
	}

	return history, nil
}

// GetAcceptedSessionByResourceAndBuyer retrieves an accepted negotiation session for a given resource and buyer.
// Returns nil if no accepted session exists.
func (r *NegotiationRepositoryImpl) GetAcceptedSessionByResourceAndBuyer(
	ctx context.Context,
	tx db.Tx,
	resourceType negotiationEntity.NegotiationResourceType,
	resourceID, buyerID uuid.UUID,
) (*negotiationEntity.NegotiationSession, error) {
	var session negotiationEntity.NegotiationSession
	var status string

	err := tx.QueryRow(ctx, `
		SELECT id, resource_type, for_sale_id, buyer_id, seller_id,
		       chat_room_id, status, order_id, expires_at, current_price, accepted_price, accepted_at,
		       proposal_sequence, created_at, updated_at
		FROM negotiation_sessions
		WHERE resource_type = $1
		  AND for_sale_id = $2
		  AND buyer_id = $3
		  AND status = 'accepted'
	`, string(resourceType), resourceID, buyerID).Scan(
		&session.ID, &session.ResourceType, &session.ForSaleID,
		&session.BuyerID, &session.SellerID, &session.ChatRoomID,
		&status, &session.OrderID, &session.ExpiresAt,
		&session.CurrentPrice, &session.AcceptedPrice, &session.AcceptedAt,
		&session.ProposalSequence,
		&session.CreatedAt, &session.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // No accepted session found
		}
		return nil, fmt.Errorf("get accepted session by resource and buyer failed: %w", err)
	}

	session.Status = negotiationEntity.NegotiationStatus(status)
	return &session, nil
}

// GetActiveNegotiationsByForSaleExcludingBuyer retrieves all active or accepted
// negotiations for a fixed-price listing, excluding a specific buyer.
// Used by ForSaleSoldEventHandler to notify other buyers that the item is sold.
func (r *NegotiationRepositoryImpl) GetActiveNegotiationsByForSaleExcludingBuyer(
	ctx context.Context,
	tx db.Tx,
	forSaleID uuid.UUID,
	excludeBuyerID uuid.UUID,
) ([]*negotiationEntity.NegotiationSession, error) {
	// Query: Get all active or accepted negotiations for this fixed-price listing (excluding the buyer who purchased)
	rows, err := tx.Query(ctx, `
		SELECT id, resource_type, for_sale_id, buyer_id, seller_id,
		       chat_room_id, status, order_id, expires_at, current_price, accepted_price, accepted_at,
		       proposal_sequence, created_at, updated_at
		FROM negotiation_sessions
		WHERE for_sale_id = $1
		  AND buyer_id != $2
		  AND status IN ('active', 'accepted')
	`, forSaleID, excludeBuyerID)
	if err != nil {
		return nil, fmt.Errorf("get active negotiations by fixed-price listing failed: %w", err)
	}
	defer rows.Close()

	var sessions []*negotiationEntity.NegotiationSession
	for rows.Next() {
		var session negotiationEntity.NegotiationSession
		var resourceType, status string

		err := rows.Scan(
			&session.ID, &resourceType, &session.ForSaleID,
			&session.BuyerID, &session.SellerID, &session.ChatRoomID,
			&status, &session.OrderID, &session.ExpiresAt,
			&session.CurrentPrice, &session.AcceptedPrice, &session.AcceptedAt,
			&session.ProposalSequence,
			&session.CreatedAt, &session.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan negotiation session failed: %w", err)
		}

		session.ResourceType = negotiationEntity.NegotiationResourceType(resourceType)
		session.Status = negotiationEntity.NegotiationStatus(status)

		sessions = append(sessions, &session)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate negotiation sessions failed: %w", rows.Err())
	}

	return sessions, nil
}

// BulkCancelAcceptedByForSaleNoOrder cancels all accepted negotiations for a fixed-price listing
// that have no associated order. Bypasses entity state machine — system event only.
func (r *NegotiationRepositoryImpl) BulkCancelAcceptedByForSaleNoOrder(
	ctx context.Context,
	tx db.Tx,
	forSaleID uuid.UUID,
) (int, error) {
	result, err := tx.Exec(ctx, `
		UPDATE negotiation_sessions
		SET status = 'cancelled', updated_at = NOW()
		WHERE for_sale_id = $1
		  AND status = 'accepted'
		  AND order_id IS NULL
	`, forSaleID)
	if err != nil {
		return 0, fmt.Errorf("bulk cancel accepted negotiations failed: %w", err)
	}
	return int(result.RowsAffected()), nil
}

// GetLatestSessionByChatRoomID retrieves the most recently updated negotiation session
// for a given chat room, regardless of status. Returns nil if no session exists.
func (r *NegotiationRepositoryImpl) GetLatestSessionByChatRoomID(
	ctx context.Context,
	tx db.Tx,
	chatRoomID uuid.UUID,
) (*negotiationEntity.NegotiationSession, error) {
	var session negotiationEntity.NegotiationSession
	var status string

	err := tx.QueryRow(ctx, `
		SELECT id, resource_type, for_sale_id, buyer_id, seller_id,
		       chat_room_id, status, order_id, expires_at, current_price, accepted_price, accepted_at,
		       proposal_sequence, created_at, updated_at
		FROM negotiation_sessions
		WHERE chat_room_id = $1
		ORDER BY updated_at DESC
		LIMIT 1
	`, chatRoomID).Scan(
		&session.ID, &session.ResourceType, &session.ForSaleID,
		&session.BuyerID, &session.SellerID, &session.ChatRoomID,
		&status, &session.OrderID, &session.ExpiresAt,
		&session.CurrentPrice, &session.AcceptedPrice, &session.AcceptedAt,
		&session.ProposalSequence,
		&session.CreatedAt, &session.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest session by chat room failed: %w", err)
	}

	session.ResourceType = negotiationEntity.NegotiationResourceType(session.ResourceType)
	session.Status = negotiationEntity.NegotiationStatus(status)

	return &session, nil
}

