package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/negotiation/entity"
	"github.com/labuda/backend/pkg/db"
)

// Repository defines the interface for negotiation persistence.
//
// NEGOTIATION → CHAT UNIFICATION:
// - Message-related methods (CreateMessage, ListMessages) have been removed
// - Messages are now handled by ChatService
// - This repository only manages session state
type Repository interface {
	// CreateSession persists a new negotiation session within a transaction.
	CreateSession(ctx context.Context, tx db.Tx, session *entity.NegotiationSession) error

	// GetSession retrieves a session without locking (for read-only operations).
	GetSession(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.NegotiationSession, error)

	// GetSessionForUpdate retrieves a session with FOR UPDATE lock.
	// This prevents concurrent modifications and must be used within a transaction.
	GetSessionForUpdate(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.NegotiationSession, error)

	// GetActiveSessionByResourceAndBuyer retrieves the active session for a given fixed-price sale and buyer.
	// Returns nil if no active session exists.
	GetActiveSessionByResourceAndBuyer(
		ctx context.Context,
		tx db.Tx,
		resourceType entity.NegotiationResourceType,
		resourceID, buyerID uuid.UUID,
	) (*entity.NegotiationSession, error)

	// GetAcceptedSessionByResourceAndBuyer retrieves an accepted negotiation session for a given fixed-price sale and buyer.
	// Returns nil if no accepted session exists.
	GetAcceptedSessionByResourceAndBuyer(
		ctx context.Context,
		tx db.Tx,
		resourceType entity.NegotiationResourceType,
		resourceID, buyerID uuid.UUID,
	) (*entity.NegotiationSession, error)

	// GetAcceptedSessionByChatRoomID retrieves an accepted negotiation session for a given chat room.
	// Returns nil if no accepted session exists.
	// Used for chat-centric order creation.
	GetAcceptedSessionByChatRoomID(
		ctx context.Context,
		tx db.Tx,
		chatRoomID uuid.UUID,
	) (*entity.NegotiationSession, error)

	// GetAcceptedSessionByChatRoomIDForUpdate retrieves an accepted negotiation session with FOR UPDATE lock.
	// Used for chat-centric order creation to prevent race conditions.
	// Returns nil if no accepted session exists.
	GetAcceptedSessionByChatRoomIDForUpdate(
		ctx context.Context,
		tx db.Tx,
		chatRoomID uuid.UUID,
	) (*entity.NegotiationSession, error)

	// GetLatestSessionByChatRoomID retrieves the most recently updated negotiation session
	// for a given chat room, regardless of status. Returns nil if no session exists.
	// Used for chat-centric negotiation status queries.
	GetLatestSessionByChatRoomID(
		ctx context.Context,
		tx db.Tx,
		chatRoomID uuid.UUID,
	) (*entity.NegotiationSession, error)

	// UpdateSession persists session changes within a transaction.
	UpdateSession(ctx context.Context, tx db.Tx, session *entity.NegotiationSession) error

	// UpdateOrderID sets the order_id for a negotiation within a transaction.
	// This is called during order creation to prevent double-order race condition.
	UpdateOrderID(ctx context.Context, tx db.Tx, negotiationID, orderID uuid.UUID) error

	// GetExpiredSessions retrieves active negotiations that have passed their expires_at time.
	// Uses FOR UPDATE SKIP LOCKED for concurrent worker support.
	// Returns up to limit sessions that should be expired.
	GetExpiredSessions(ctx context.Context, tx db.Tx, limit int) ([]*entity.NegotiationSession, error)

	// PRICE SECURITY AUDIT: Price history methods

	// CreatePriceHistoryEntry creates a new price history entry.
	CreatePriceHistoryEntry(ctx context.Context, tx db.Tx, history *entity.NegotiationPriceHistory) error

	// GetPriceHistoryBySession retrieves all price history entries for a session, ordered by created_at DESC.
	GetPriceHistoryBySession(ctx context.Context, tx db.Tx, sessionID uuid.UUID) ([]*entity.NegotiationPriceHistory, error)

	// GetPriceHistoryByUser retrieves all price history entries for a user, ordered by created_at DESC.
	GetPriceHistoryByUser(ctx context.Context, tx db.Tx, userID uuid.UUID, limit int) ([]*entity.NegotiationPriceHistory, error)

	// GetActiveNegotiationsByForSaleExcludingBuyer retrieves all active or accepted
	// negotiations for a fixed-price sale, excluding a specific buyer.
	// Used by ForSaleSoldEventHandler to notify other buyers that the item is sold.
	GetActiveNegotiationsByForSaleExcludingBuyer(
		ctx context.Context,
		tx db.Tx,
		forSaleID uuid.UUID,
		excludeBuyerID uuid.UUID,
	) ([]*entity.NegotiationSession, error)

	// BulkCancelAcceptedByForSaleNoOrder cancels all accepted negotiations for a fixed-price sale
	// that have no associated order (order_id IS NULL).
	// Called on for_sale.sold to clean up stale accepted state for buyers who did not
	// complete checkout. Bypasses the entity state machine intentionally (system event).
	// Returns the number of rows updated.
	BulkCancelAcceptedByForSaleNoOrder(
		ctx context.Context,
		tx db.Tx,
		forSaleID uuid.UUID,
	) (int, error)
}
