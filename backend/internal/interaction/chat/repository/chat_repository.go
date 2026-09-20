package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/interaction/chat/entity"
)

// Repository defines the persistence interface for chat domain.
//
// DESIGN PRINCIPLES:
// - All mutations use db.Tx for transaction safety
// - Cursor-based pagination only (NO OFFSET)
// - Idempotent operations supported
// - No SQL in service layer
type Repository interface {
	// ========================================================================
	// ROOM OPERATIONS
	// ========================================================================

	// CreateRoom creates a new chat room.
	// Returns ErrDuplicateRoom if a room with the same participants and type exists.
	CreateRoom(ctx context.Context, tx interface{}, room *entity.ChatRoom) error

	// GetRoomByID retrieves a room by ID.
	// Returns ErrRoomNotFound if not found.
	GetRoomByID(ctx context.Context, tx interface{}, roomID uuid.UUID) (*entity.ChatRoom, error)

	// GetRoomByIDForUpdate retrieves a room by ID with a FOR UPDATE lock.
	// Use this to serialize commerce-context mutations that hang off chat.
	GetRoomByIDForUpdate(ctx context.Context, tx interface{}, roomID uuid.UUID) (*entity.ChatRoom, error)

	// GetDirectRoom retrieves a direct room between two users.
	// Returns ErrRoomNotFound if not found.
	GetDirectRoom(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (*entity.ChatRoom, error)

	// ListRoomsByUser lists all rooms where the user is a participant.
	// Uses cursor-based pagination on last_message_at.
	// Returns rooms ordered by last_message_at DESC.
	ListRoomsByUser(
		ctx context.Context,
		tx interface{},
		userID uuid.UUID,
		cursorLastMessageAt *time.Time,
		cursorID *uuid.UUID,
		limit int,
	) ([]*entity.ChatRoom, error)

	// GetRoomByOrderID retrieves a room by linked order ID.
	// Returns ErrRoomNotFound if not found.
	GetRoomByOrderID(ctx context.Context, tx interface{}, orderID uuid.UUID) (*entity.ChatRoom, error)

	// UpdateRoomLastMessageAt updates the last_message_at timestamp.
	UpdateRoomLastMessageAt(ctx context.Context, tx interface{}, roomID uuid.UUID, timestamp time.Time) error

	// UpdateRoomLinkedOrderId updates the room's linked order ID for commerce continuity.
	// This links an order to a chat, enabling order↔chat alignment.
	UpdateRoomLinkedOrderId(ctx context.Context, tx interface{}, roomID uuid.UUID, linkedOrderID *uuid.UUID) error

	// ========================================================================
	// MESSAGE OPERATIONS
	// ========================================================================

	// CreateMessage creates a new chat message.
	// Returns ErrDuplicateMessage if idempotency_key already exists.
	CreateMessage(ctx context.Context, tx interface{}, message *entity.ChatMessage) error

	// GetMessageByID retrieves a message by ID.
	// Returns ErrMessageNotFound if not found.
	GetMessageByID(ctx context.Context, tx interface{}, messageID uuid.UUID) (*entity.ChatMessage, error)

	// ListMessagesByRoom lists messages in a room.
	// Uses cursor-based pagination on (created_at, id).
	// Returns messages ordered by created_at DESC, id DESC.
	ListMessagesByRoom(
		ctx context.Context,
		tx interface{},
		roomID uuid.UUID,
		cursorCreatedAt *time.Time,
		cursorID *uuid.UUID,
		limit int,
	) ([]*entity.ChatMessage, error)

	// GetMessageByIdempotencyKey retrieves a message by (sender_id, idempotency_key).
	//
	// AUTHORITY: actor-scoped. The idempotency key is only meaningful within
	// the sender's own command space — UNIQUE(sender_id, idempotency_key)
	// (migration 000032). A key used by another sender must never be returned.
	// Returns ErrMessageNotFound if not found.
	GetMessageByIdempotencyKey(ctx context.Context, tx interface{}, senderID uuid.UUID, idempotencyKey string) (*entity.ChatMessage, error)

	// ========================================================================
	// MODERATION OPERATIONS
	// ========================================================================

	// SoftHideForModeration sets deleted_at/deleted_by/deletion_reason on a message.
	// Idempotent: if message already hidden, returns nil.
	SoftHideForModeration(ctx context.Context, tx interface{}, messageID uuid.UUID, deletedBy uuid.UUID, reason string) error

	// RestoreFromModeration clears deleted_at/deleted_by/deletion_reason.
	// Idempotent: if message not hidden, returns nil.
	RestoreFromModeration(ctx context.Context, tx interface{}, messageID uuid.UUID) error

	// ========================================================================
	// READ STATE OPERATIONS
	// ========================================================================

	// CreateReadState creates a new read state.
	// Returns ErrDuplicateReadState if (room_id, user_id) already exists.
	CreateReadState(ctx context.Context, tx interface{}, state *entity.ChatReadState) error

	// GetReadState retrieves the read state for a room and user.
	// Returns ErrReadStateNotFound if not found.
	GetReadState(ctx context.Context, tx interface{}, roomID, userID uuid.UUID) (*entity.ChatReadState, error)

	// UpdateReadState updates the last_read_at timestamp.
	UpdateReadState(ctx context.Context, tx interface{}, state *entity.ChatReadState) error

	// UpsertReadState creates or updates a read state.
	UpsertReadState(ctx context.Context, tx interface{}, state *entity.ChatReadState) error

	// ListReadStatesByRoom lists all read states for a room.
	ListReadStatesByRoom(ctx context.Context, tx interface{}, roomID uuid.UUID) ([]*entity.ChatReadState, error)

	// GetUnreadCountByRoomAndUser calculates the unread count for one room and one user.
	//
	// This is a thin delegation to GetUnreadCountsByRoomIDs so that exactly ONE
	// unread-count SQL formula exists in the codebase.
	GetUnreadCountByRoomAndUser(ctx context.Context, tx interface{}, roomID, userID uuid.UUID) (int, error)

	// GetUnreadCountsByRoomIDs calculates the unread count for several rooms for
	// one viewer in a single batch query.
	//
	// CANONICAL unread-count authority: unread = visible (deleted_at IS NULL)
	// messages after the viewer's last_read_at, excluding muted senders. Every
	// unread consumer (single-room endpoint, send/read flows, room-list badges)
	// resolves through this method.
	GetUnreadCountsByRoomIDs(ctx context.Context, tx interface{}, roomIDs []uuid.UUID, userID uuid.UUID) (map[uuid.UUID]int, error)
}

// ========================================================================
// DOMAIN ERRORS
// ========================================================================

var (
	// ErrRoomNotFound is returned when a room is not found.
	ErrRoomNotFound = errorString("room not found")

	// ErrDuplicateRoom is returned when a room with the same participants and type exists.
	ErrDuplicateRoom = errorString("room already exists")

	// ErrMessageNotFound is returned when a message is not found.
	ErrMessageNotFound = errorString("message not found")

	// ErrDuplicateMessage is returned when a message with the same idempotency key exists.
	ErrDuplicateMessage = errorString("message already exists")

	// ErrIdempotencyKeyConflict is returned when the same sender reuses an
	// idempotency key for a different command (different command_fingerprint).
	// Same sender + same key + same command is an idempotent replay; same
	// sender + same key + different command is a conflict.
	ErrIdempotencyKeyConflict = errorString("idempotency key already used with a different command")

	// ErrReadStateNotFound is returned when a read state is not found.
	ErrReadStateNotFound = errorString("read state not found")

	// ErrDuplicateReadState is returned when a read state with the same room_id and user_id exists.
	ErrDuplicateReadState = errorString("read state already exists")

	// ErrInvalidRoomType is returned when an invalid room type is provided.
	ErrInvalidRoomType = errorString("invalid room type")

	// ErrInvalidMessageType is returned when an invalid message type is provided.
	ErrInvalidMessageType = errorString("invalid message type")

	// ErrInvalidIdempotencyKey is returned when the idempotency key is empty.
	ErrInvalidIdempotencyKey = errorString("idempotency key cannot be empty")

	// ErrInvalidResourceOccurrence is returned when a message resource
	// occurrence reference violates the communication contract (invalid
	// operation/type, nil id, or a direct-commerce operation on a
	// non-commerce resource type).
	ErrInvalidResourceOccurrence = errorString("invalid resource occurrence")

	// ErrParticipantMismatch is returned when a user is not a participant in a room.
	ErrParticipantMismatch = errorString("user is not a participant in this room")

	// ErrSelfChat is returned when attempting to create a room with the same user twice.
	ErrSelfChat = errorString("cannot create room with same user as both participants")

	// ErrMessageBodyTooLong is returned when the message body exceeds the maximum allowed length.
	ErrMessageBodyTooLong = errorString("message body exceeds maximum allowed length")

	// ErrRateLimited is returned when a rate limit is exceeded.
	ErrRateLimited = errorString("rate limit exceeded")

	// ErrUserBlocked is returned when the recipient has blocked the sender.
	ErrUserBlocked = errorString("cannot send message: recipient has blocked you")

	// ErrAttachmentForSaleNotFound is returned when a fixed-price sale attachment references a non-existent sale.
	ErrAttachmentForSaleNotFound = errorString("attachment fixed-price sale not found")

	// ErrAttachmentAuctionNotFound is returned when an auction attachment references a non-existent auction.
	ErrAttachmentAuctionNotFound = errorString("attachment auction not found")

	// ErrAttachmentPostNotFound is returned when a reference attachment points to a non-existent post.
	ErrAttachmentPostNotFound = errorString("attachment post not found")

	// ErrAttachmentRequestNotFound is returned when a reference attachment points to a non-existent request.
	ErrAttachmentRequestNotFound = errorString("attachment request not found")

	// ErrAttachmentProfileNotFound is returned when a reference attachment points to a non-existent or hidden profile.
	ErrAttachmentProfileNotFound = errorString("attachment profile not found")

	// ErrOrderNotFound is returned when LinkOrderToChat references an order that does not exist.
	ErrOrderNotFound = errorString("order not found")

	// ErrOrderOwnershipMismatch is returned when the requesting user is neither
	// the buyer nor the seller of the order being linked.
	ErrOrderOwnershipMismatch = errorString("user is not the buyer or seller of this order")

	// ErrOrderRoomParticipantMismatch is returned when the chat room's two
	// participants do not exactly match the order's buyer and seller.
	ErrOrderRoomParticipantMismatch = errorString("chat room participants do not match order buyer/seller")

	// ErrOrderAlreadyLinkedElsewhere is returned when the order is already
	// linked to a different chat room (an order links to at most one room at a time).
	ErrOrderAlreadyLinkedElsewhere = errorString("order is already linked to a different chat room")

	// Resource-authority sentinels are shared across chat adapters and the
	// application layer for resource-occurrence authorization decisions.
	ErrResourceNotFound      = errorString("resource not found")
	ErrResourceNotAccessible = errorString("resource not accessible")
	ErrResourceNotPromotable = errorString("resource not promotable")
)

// errorString is a string type that implements error.
type errorString string

func (e errorString) Error() string {
	return string(e)
}
