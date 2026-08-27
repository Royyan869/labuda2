package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	infraRepo "github.com/labuda/backend/internal/interaction/chat/infrastructure/repository"
	chatRepo "github.com/labuda/backend/internal/interaction/chat/repository"
	"github.com/labuda/backend/internal/realtime"
	socialRepo "github.com/labuda/backend/internal/social/graph"
	infraSocialRepo "github.com/labuda/backend/internal/social/graph/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/rate"
	"go.uber.org/zap"
)

const (
	// MaxMessageBodyLength is the maximum allowed length for a message body.
	MaxMessageBodyLength = 5000
)

// AccountStatusChecker is the service-layer interface for account status enforcement.
// The service re-checks independently of middleware — neither layer alone is sufficient.
type AccountStatusChecker interface {
	EnsureActive(ctx context.Context, userID uuid.UUID) error
}

// ChatModerationRoomUpdater is the moderation-facing service contract.
//
// The moderation worker calls into this service so hide/restore mutations can
// keep room-list projections in sync with a single transaction boundary.
type ChatModerationRoomUpdater interface {
	SoftHideForModeration(ctx context.Context, tx db.Tx, messageID uuid.UUID, deletedBy uuid.UUID, reason, moderationKey string) error
	RestoreFromModeration(ctx context.Context, tx db.Tx, messageID uuid.UUID, moderationKey string) error
}

// ChatMetrics defines the interface for chat-related metrics.
type ChatMetrics interface {
	RecordChatMessage()
	RecordChatRateLimited()
}

// ForSaleChecker defines the interface for checking fixed-price sale existence.
// This allows attachment validation without importing the full fixed-price sale domain.
type ForSaleChecker interface {
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (interface{}, error)
}

// AuctionChecker defines the interface for checking auction existence.
// This allows attachment validation without importing the full auction domain.
type AuctionChecker interface {
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (interface{}, error)
}

// OrderOwnershipReader is the minimal order-domain contract needed to
// validate order/room ownership before a manual order-link mutation
// (LinkOrderToChat). Kept narrow — buyer/seller IDs only — to preserve the
// chat/order domain boundary described in STRICT BOUNDARY RULES above.
type OrderOwnershipReader interface {
	GetOrderParticipants(ctx context.Context, tx db.Tx, orderID uuid.UUID) (buyerID, sellerID uuid.UUID, err error)
}

// Service handles chat domain business logic.
//
// STRICT BOUNDARY RULES:
// - NO direct financial mutations
// - NO ledger modifications
// - NO trade/offer/withdraw mutations
// - Emits outbox events for notification delivery
// - Idempotent message sending
// - Cursor-based pagination only (NO OFFSET)
type Service struct {
	db                    Transactor
	repo                  chatRepo.Repository
	socialRepo            socialRepo.SocialRepository
	outboxRepo            OutboxInserter
	rateLimiter           *rate.RateLimiter
	metrics               ChatMetrics           // Optional metrics collector
	forSaleChecker ForSaleChecker // For attachment validation
	auctionChecker        AuctionChecker        // For attachment validation
	statusChecker         AccountStatusChecker  // Account status enforcement (service-layer authority)
	orderReader           OrderOwnershipReader  // Order buyer/seller lookup for LinkOrderToChat authorization
	log                   *zap.Logger           // Optional logger for warnings
}

// OutboxInserter defines the interface for inserting outbox events.
type OutboxInserter interface {
	InsertTx(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error
}

// Transactor represents the ability to execute functions within transactions.
type Transactor interface {
	WithTx(ctx context.Context, fn func(tx db.Tx) error) error
}

// NewService creates a new chat service.
func NewService(
	db Transactor,
	repo chatRepo.Repository,
	socialRepo socialRepo.SocialRepository,
	outboxRepo OutboxInserter,
	rateLimiter *rate.RateLimiter,
	metrics ChatMetrics,
	forSaleChecker ForSaleChecker,
	auctionChecker AuctionChecker,
	statusChecker AccountStatusChecker,
	orderReader OrderOwnershipReader,
	log *zap.Logger,
) *Service {
	return &Service{
		db:                    db,
		repo:                  repo,
		socialRepo:            socialRepo,
		outboxRepo:            outboxRepo,
		rateLimiter:           rateLimiter,
		metrics:               metrics,
		forSaleChecker: forSaleChecker,
		auctionChecker:        auctionChecker,
		statusChecker:         statusChecker,
		orderReader:           orderReader,
		log:                   log,
	}
}

// NewServiceWithDefaults creates a chat service with the default repository.
func NewServiceWithDefaults(
	db Transactor,
	outboxRepo OutboxInserter,
	rateLimiter *rate.RateLimiter,
	metrics ChatMetrics,
	forSaleChecker ForSaleChecker,
	auctionChecker AuctionChecker,
	statusChecker AccountStatusChecker,
	orderReader OrderOwnershipReader,
	log *zap.Logger,
) *Service {
	return NewService(
		db,
		infraRepo.NewChatRepository(),
		infraSocialRepo.NewSocialRepository(),
		outboxRepo,
		rateLimiter,
		metrics,
		forSaleChecker,
		auctionChecker,
		statusChecker,
		orderReader,
		log,
	)
}

// ========================================================================
// ROOM OPERATIONS
// ========================================================================

// GetOrCreateDirectRoom gets or creates a direct room between two users.
//
// Transaction flow:
// 1. BEGIN
// 2. Try to get existing direct room
// 3. If not found, create new room with context if provided
// 4. If found and context provided, update context if room has no context
// 5. COMMIT
//
// Business rules:
// - Users must be different (no self-chat)
// - One room per user pair for direct type
// - Participants are stored in sorted order
// - Context is optional commerce metadata (fixed-price sale, auction, etc.)
// - BLOCK ENFORCEMENT: If creating room WITHOUT commerce context and target user has blocked requester, return error
func (s *Service) GetOrCreateDirectRoom(
	ctx context.Context,
	userA, userB uuid.UUID,
	contextJSON json.RawMessage,
	contextSetBy uuid.UUID,
) (*chatEntity.ChatRoom, error) {
	if userA == userB {
		return nil, chatRepo.ErrSelfChat
	}

	// Account status enforcement: requester must be active.
	// Service re-checks independently of RequireActiveAccount middleware.
	if s.statusChecker != nil {
		if err := s.statusChecker.EnsureActive(ctx, userA); err != nil {
			return nil, err
		}
	}

	// Rate limit: 10 room creations per minute per user
	key := fmt.Sprintf("chat:room:%s", userA.String())
	if !s.rateLimiter.Allow(key, 10, 1*time.Minute) {
		return nil, chatRepo.ErrRateLimited
	}

	// Block enforcement: bidirectional. Applies regardless of commerce context.
	// If either user has blocked the other, room creation is denied.
	// A block prevents ALL new social contact — including fixed-price-sale-initiated rooms.
	var blocked bool
	blockErr := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		blocked, err = s.socialRepo.ExistsBlock(ctx, tx, userA, userB)
		return err
	})
	if blockErr != nil {
		return nil, fmt.Errorf("failed to check block: %w", blockErr)
	}
	if blocked {
		return nil, chatRepo.ErrUserBlocked
	}

	var room *chatEntity.ChatRoom
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		var created bool
		room, created, err = s.getOrCreateDirectRoomTx(ctx, tx, userA, userB, contextJSON, contextSetBy)
		if err != nil {
			return err
		}
		if created {
			if err := s.emitChatRoomCreatedEvents(ctx, tx, room); err != nil {
				return err
			}
		}
		return err
	})

	if err != nil {
		return nil, err
	}

	return room, nil
}

// getOrCreateDirectRoomTx is the internal transaction-aware version of GetOrCreateDirectRoom.
// This allows external services to call this method within their own transaction.
//
// IMPORTANT: This method does NOT manage its own transaction.
// The caller must provide a valid tx from an ongoing transaction.
//
// CONTEXT SEMANTICS:
// - New room: Creates with provided context
// - Existing room without context: Updates with provided context
// - Existing room with context: Keeps existing context (NOT overwritten)
func (s *Service) getOrCreateDirectRoomTx(
	ctx context.Context,
	tx db.Tx,
	userA, userB uuid.UUID,
	contextJSON json.RawMessage,
	contextSetBy uuid.UUID,
) (*chatEntity.ChatRoom, bool, error) {
	// Try to get existing room first
	existingRoom, err := s.repo.GetDirectRoom(ctx, tx, userA, userB)
	if err == nil {
		// Room exists - update context if room has no context and context was provided
		if len(contextJSON) > 0 && !existingRoom.HasContext() {
			if err := s.repo.UpdateRoomContext(ctx, tx, existingRoom.ID, contextJSON, contextSetBy); err != nil {
				return nil, false, fmt.Errorf("failed to update room context: %w", err)
			}
			// Reload room to get updated context
			updatedRoom, err := s.repo.GetDirectRoom(ctx, tx, userA, userB)
			if err != nil {
				return nil, false, fmt.Errorf("failed to reload room: %w", err)
			}
			return updatedRoom, false, nil
		}
		return existingRoom, false, nil
	}
	if err != chatRepo.ErrRoomNotFound {
		return nil, false, fmt.Errorf("failed to get direct room: %w", err)
	}

	// Room doesn't exist, create new one
	var newRoom *chatEntity.ChatRoom
	if len(contextJSON) > 0 && contextSetBy != uuid.Nil {
		newRoom = chatEntity.NewChatRoomWithContext(
			chatEntity.RoomTypeDirect,
			userA, userB,
			contextJSON,
			contextSetBy,
		)
	} else {
		newRoom = chatEntity.NewChatRoom(chatEntity.RoomTypeDirect, userA, userB)
	}

	if err := s.repo.CreateRoom(ctx, tx, newRoom); err != nil {
		// CRITICAL: Handle race condition - if unique violation occurred,
		// another transaction created the same room. Fetch and return it.
		if err == chatRepo.ErrDuplicateRoom {
			existingRoom, fetchErr := s.repo.GetDirectRoom(ctx, tx, userA, userB)
			if fetchErr != nil {
				return nil, false, fmt.Errorf("room created by another request but fetch failed: %w", fetchErr)
			}
			// Update context if room has no context and context was provided
			if len(contextJSON) > 0 && !existingRoom.HasContext() {
				if updErr := s.repo.UpdateRoomContext(ctx, tx, existingRoom.ID, contextJSON, contextSetBy); updErr != nil {
					return nil, false, fmt.Errorf("failed to update room context: %w", updErr)
				}
				// Reload room to get updated context
				updatedRoom, reloadErr := s.repo.GetDirectRoom(ctx, tx, userA, userB)
				if reloadErr != nil {
					return nil, false, fmt.Errorf("failed to reload room: %w", reloadErr)
				}
				return updatedRoom, false, nil
			}
			return existingRoom, false, nil
		}
		return nil, false, fmt.Errorf("failed to create room: %w", err)
	}

	return newRoom, true, nil
}

// NOTE: GetOrCreateNegotiationRoom (and its private helper
// getOrCreateNegotiationRoomTx) were removed in PASS_12B. As of PASS_8A,
// negotiation proposal/counter messages route directly into the buyer/seller's
// existing direct room via chat_room_id persisted on the negotiation session
// (see negotiation_service.go) — no separate room_type='negotiation' room is
// ever created anymore. The removed methods had zero production callers.
// RoomTypeNegotiation and the Postgres 'negotiation' enum value are
// intentionally retained to correctly read back any pre-existing rows.

// GetOrCreateSupportRoom gets or creates a support room for a user.
//
// Transaction flow:
// 1. BEGIN
// 2. Try to get existing support room
// 3. If not found, create new room
// 4. COMMIT
//
// Business rules:
// - One support room per user
// - participant_a = user, participant_b = system UUID (Nil)
// - room_type = 'support'
func (s *Service) GetOrCreateSupportRoom(ctx context.Context, userID uuid.UUID) (*chatEntity.ChatRoom, error) {
	var room *chatEntity.ChatRoom
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		var created bool
		room, created, err = s.getOrCreateSupportRoomTx(ctx, tx, userID, nil)
		if err != nil {
			return err
		}
		if created {
			if err := s.emitChatRoomCreatedEvents(ctx, tx, room); err != nil {
				return err
			}
		}
		return err
	})

	if err != nil {
		return nil, err
	}

	return room, nil
}

// GetOrCreateSupportRoomWithContext gets or creates a support room for a user with context.
//
// Transaction flow:
// 1. BEGIN
// 2. Try to get existing support room
// 3. If not found, create new room with context
// 4. If found and no context, update with provided context
// 5. COMMIT
//
// Business rules:
// - One support room per user
// - participant_a = user, participant_b = system UUID (Nil)
// - room_type = 'support'
// - Context is optional (e.g., linked_order_id for support tickets)
func (s *Service) GetOrCreateSupportRoomWithContext(ctx context.Context, userID uuid.UUID, contextJSON json.RawMessage) (*chatEntity.ChatRoom, error) {
	var room *chatEntity.ChatRoom
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		var created bool
		room, created, err = s.getOrCreateSupportRoomTx(ctx, tx, userID, contextJSON)
		if err != nil {
			return err
		}
		if created {
			if err := s.emitChatRoomCreatedEvents(ctx, tx, room); err != nil {
				return err
			}
		}
		return err
	})

	if err != nil {
		return nil, err
	}

	return room, nil
}

// getOrCreateSupportRoomTx is the internal transaction-aware version of GetOrCreateSupportRoom.
//
// IMPORTANT: This method does NOT manage its own transaction.
// The caller must provide a valid tx from an ongoing transaction.
//
// CONTEXT SEMANTICS:
// - New room: Creates with provided context
// - Existing room without context: Updates with provided context
// - Existing room with context: Keeps existing context (NOT overwritten)
func (s *Service) getOrCreateSupportRoomTx(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	contextJSON json.RawMessage,
) (*chatEntity.ChatRoom, bool, error) {
	// Try to get existing support room first
	existingRoom, err := s.repo.GetSupportRoom(ctx, tx, userID)
	if err == nil {
		// Room exists - update context if room has no context and context was provided
		if len(contextJSON) > 0 && !existingRoom.HasContext() {
			if err := s.repo.UpdateRoomContext(ctx, tx, existingRoom.ID, contextJSON, uuid.Nil); err != nil {
				return nil, false, fmt.Errorf("failed to update room context: %w", err)
			}
			// Reload room to get updated context
			updatedRoom, err := s.repo.GetSupportRoom(ctx, tx, userID)
			if err != nil {
				return nil, false, fmt.Errorf("failed to reload room: %w", err)
			}
			return updatedRoom, false, nil
		}
		return existingRoom, false, nil
	}
	if err != chatRepo.ErrRoomNotFound {
		return nil, false, fmt.Errorf("failed to get support room: %w", err)
	}

	// Room doesn't exist, create new one
	var newRoom *chatEntity.ChatRoom
	if len(contextJSON) > 0 {
		// Create with context (context_set_by = system/Nil for support rooms)
		newRoom = chatEntity.NewChatRoomWithContext(
			chatEntity.RoomTypeSupport,
			userID,
			uuid.Nil, // System UUID as participant_b
			contextJSON,
			uuid.Nil, // Context set by system
		)
	} else {
		newRoom = chatEntity.NewChatRoom(
			chatEntity.RoomTypeSupport,
			userID,
			uuid.Nil, // System UUID as participant_b
		)
	}

	if err := s.repo.CreateRoom(ctx, tx, newRoom); err != nil {
		// CRITICAL: Handle race condition - if unique violation occurred,
		// another transaction created the same room. Fetch and return it.
		if err == chatRepo.ErrDuplicateRoom {
			existingRoom, fetchErr := s.repo.GetSupportRoom(ctx, tx, userID)
			if fetchErr != nil {
				return nil, false, fmt.Errorf("room created by another request but fetch failed: %w", fetchErr)
			}
			// Update context if room has no context and context was provided
			if len(contextJSON) > 0 && !existingRoom.HasContext() {
				if updErr := s.repo.UpdateRoomContext(ctx, tx, existingRoom.ID, contextJSON, uuid.Nil); updErr != nil {
					return nil, false, fmt.Errorf("failed to update room context: %w", updErr)
				}
				// Reload room to get updated context
				updatedRoom, reloadErr := s.repo.GetSupportRoom(ctx, tx, userID)
				if reloadErr != nil {
					return nil, false, fmt.Errorf("failed to reload room: %w", reloadErr)
				}
				return updatedRoom, false, nil
			}
			return existingRoom, false, nil
		}
		return nil, false, fmt.Errorf("failed to create support room: %w", err)
	}

	return newRoom, true, nil
}

// SendSystemMessage sends a system message to a room.
//
// Transaction flow:
// 1. BEGIN
// 2. Verify room exists
// 3. Create system message (sender_id = Nil, message_type = 'system')
// 4. Update room's last_message_at
// 5. COMMIT
//
// Business rules:
// - No rate limit (system messages)
// - No block checking (system messages)
// - sender_id is always Nil (system)
// - message_type is always 'system'
func (s *Service) SendSystemMessage(ctx context.Context, roomID uuid.UUID, body string) error {
	// Validate body
	if body == "" {
		return fmt.Errorf("system message body cannot be empty")
	}
	if len(body) > MaxMessageBodyLength {
		return chatRepo.ErrMessageBodyTooLong
	}

	return s.db.WithTx(ctx, func(tx db.Tx) error {
		// Verify room exists
		_, err := s.repo.GetRoomByID(ctx, tx, roomID)
		if err != nil {
			return fmt.Errorf("room not found: %w", err)
		}

		// Create system message
		// Use a deterministic idempotency key for system messages
		idempotencyKey := fmt.Sprintf("system:%s:%d", roomID.String(), time.Now().UnixNano())
		systemMessage := chatEntity.NewSystemMessage(roomID, body, idempotencyKey)

		if err := s.repo.CreateMessage(ctx, tx, systemMessage); err != nil {
			return fmt.Errorf("failed to create system message: %w", err)
		}

		// Update room's last_message_at
		if err := s.repo.UpdateRoomLastMessageAt(ctx, tx, roomID, systemMessage.CreatedAt); err != nil {
			return fmt.Errorf("failed to update room last_message_at: %w", err)
		}

		return nil
	})
}

// GetRoom retrieves a room by ID.
func (s *Service) GetRoom(ctx context.Context, roomID uuid.UUID) (*chatEntity.ChatRoom, error) {
	var room *chatEntity.ChatRoom
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		room, err = s.repo.GetRoomByID(ctx, tx, roomID)
		return err
	})

	if err != nil {
		return nil, err
	}

	return room, nil
}

// GetRoomByOrderID retrieves a room by linked order ID.
func (s *Service) GetRoomByOrderID(ctx context.Context, orderID uuid.UUID) (*chatEntity.ChatRoom, error) {
	var room *chatEntity.ChatRoom
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		room, err = s.repo.GetRoomByOrderID(ctx, tx, orderID)
		return err
	})

	if err != nil {
		return nil, err
	}

	return room, nil
}

// ListRoomsByUser lists all rooms where the user is a participant.
//
// Uses cursor-based pagination on (last_message_at, id).
// Returns rooms ordered by last_message_at DESC.
func (s *Service) ListRoomsByUser(
	ctx context.Context,
	userID uuid.UUID,
	cursorLastMessageAt *time.Time,
	cursorID *uuid.UUID,
	limit int,
) ([]*chatEntity.ChatRoom, error) {
	var rooms []*chatEntity.ChatRoom
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		rooms, err = s.repo.ListRoomsByUser(ctx, tx, userID, cursorLastMessageAt, cursorID, limit)
		return err
	})

	if err != nil {
		return nil, err
	}

	return rooms, nil
}

// AutoLinkOrderToDirectRoom ensures the canonical buyer↔seller direct room
// exists and sets its linked_order_id to the new order (LATEST ACTIVE ORDER
// RULE). This is the consumer-side counterpart of the
// order.chat_link_requested outbox event — see
// internal/commerce/order/application/order_creation_service.go.
//
// CONTRACT:
//   - Idempotent end-state. Replaying the same event produces the same row
//     state. UNIQUE (participant_a, participant_b, room_type) prevents
//     duplicate rooms; setting linked_order_id to the same value is a no-op.
//   - Single internal transaction. Either both room ensure + link succeed, or
//     neither. Caller (consumer handler) does NOT need to wrap this in its own
//     tx.
//   - Returns nil on success. On error, the outbox worker treats it as a
//     retryable failure and re-delivers per the retry / DLQ policy.
//
// NOTE: This is the ONLY entry point for order-driven direct-room linkage.
// Order creation MUST NOT mutate chat_rooms directly; doing so would cross
// the commerce/chat domain authority boundary (RUNTIME-INVARIANTS §1.2).
func (s *Service) AutoLinkOrderToDirectRoom(
	ctx context.Context,
	buyerID, sellerID, orderID uuid.UUID,
) (*chatEntity.ChatRoom, error) {
	if buyerID == sellerID {
		return nil, chatRepo.ErrSelfChat
	}

	var room *chatEntity.ChatRoom
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		// Step 1: Ensure the canonical direct room exists. We pass no context
		// here — context is set by the chat surface that opened the room, not
		// by an asynchronous auto-link consumer.
		ensured, created, err := s.getOrCreateDirectRoomTx(ctx, tx, buyerID, sellerID, nil, uuid.Nil)
		if err != nil {
			return fmt.Errorf("ensure direct room: %w", err)
		}

		// Step 2: Set linked_order_id. Idempotent: same orderID → same state.
		// LATEST ACTIVE ORDER RULE is preserved because this overwrites any
		// previous linkage; that is the same semantics the inline path had.
		if err := s.repo.UpdateRoomLinkedOrderId(ctx, tx, ensured.ID, &orderID); err != nil {
			return fmt.Errorf("update linked_order_id: %w", err)
		}

		// Reload to return the post-update state.
		reloaded, err := s.repo.GetRoomByID(ctx, tx, ensured.ID)
		if err != nil {
			return fmt.Errorf("reload room: %w", err)
		}
		room = reloaded

		if created {
			if err := s.emitChatRoomCreatedEvents(ctx, tx, room); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return room, nil
}

func (s *Service) emitChatRoomCreatedEvents(ctx context.Context, tx db.Tx, room *chatEntity.ChatRoom) error {
	for _, recipientID := range []uuid.UUID{room.ParticipantA, room.ParticipantB} {
		if recipientID == uuid.Nil {
			continue
		}

		payload := buildChatRoomCreatedOutboxPayload(room, recipientID, recipientID)
		idempotencyKey := fmt.Sprintf("chat.room.created.%s.%s", room.ID.String(), recipientID.String())
		if err := s.outboxRepo.InsertTx(ctx, tx, realtime.EventTypeChatRoomCreated, payload, idempotencyKey); err != nil {
			return fmt.Errorf("insert room.created outbox event failed: %w", err)
		}
	}
	return nil
}

// LinkOrderToChat links an order to a chat room for commerce continuity.
//
// Transaction flow:
// 1. BEGIN
// 2. Verify room exists and requesting user is a participant
// 3. Verify order exists and requesting user is its buyer or seller
// 4. Verify the room's two participants exactly match the order's buyer/seller
//    (rejects linking an unrelated order into an unrelated room)
// 5. Verify the order isn't already linked to a different room
// 6. Update room's linked_order_id
// 7. COMMIT
//
// Business rules:
// - Order must belong to a room participant AND the room's counterparty must
//   be the order's other party — the room and order must describe the same
//   buyer/seller pair. This is the authorization boundary for this mutation;
//   see PASS_6A / F1.
// - Supports linking orders created from chat or navigating from order detail
// - LATEST ACTIVE ORDER RULE: a room's linked_order_id may be replaced by a
//   newer order between the same pair over time, but at any instant an order
//   is linked to at most one room.
func (s *Service) LinkOrderToChat(
	ctx context.Context,
	roomID, orderID, requestingUserID uuid.UUID,
) (*chatEntity.ChatRoom, error) {
	var room *chatEntity.ChatRoom
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		room, err = s.repo.GetRoomByID(ctx, tx, roomID)
		if err != nil {
			return fmt.Errorf("room not found: %w", err)
		}

		// Verify requesting user is a participant
		if !room.HasParticipant(requestingUserID) {
			return chatRepo.ErrParticipantMismatch
		}

		if s.orderReader == nil {
			return fmt.Errorf("chat: order ownership reader not configured")
		}
		buyerID, sellerID, err := s.orderReader.GetOrderParticipants(ctx, tx, orderID)
		if err != nil {
			return chatRepo.ErrOrderNotFound
		}

		// Requesting user must be the buyer or seller of the order being linked.
		if requestingUserID != buyerID && requestingUserID != sellerID {
			return chatRepo.ErrOrderOwnershipMismatch
		}

		// The room's two participants must exactly match the order's buyer and
		// seller — a room with an unrelated third party must not be able to
		// claim an order it has no relationship to.
		if !room.HasParticipant(buyerID) || !room.HasParticipant(sellerID) {
			return chatRepo.ErrOrderRoomParticipantMismatch
		}

		// An order links to at most one room at a time. If it's already linked
		// to a different room, reject rather than silently creating a second
		// mapping (which would make order→room lookup non-deterministic).
		if existingRoom, err := s.repo.GetRoomByOrderID(ctx, tx, orderID); err == nil && existingRoom != nil && existingRoom.ID != roomID {
			return chatRepo.ErrOrderAlreadyLinkedElsewhere
		}

		// Update linked_order_id (LATEST ACTIVE ORDER RULE)
		if err := s.repo.UpdateRoomLinkedOrderId(ctx, tx, roomID, &orderID); err != nil {
			return fmt.Errorf("failed to link order: %w", err)
		}

		// Reload room to get updated state
		room, err = s.repo.GetRoomByID(ctx, tx, roomID)
		return err
	})

	if err != nil {
		return nil, err
	}

	return room, nil
}

// ========================================================================
// MESSAGE OPERATIONS
// ========================================================================

// SendMessage sends a message to a room.
//
// Transaction flow:
// 1. BEGIN
// 2. Validate room exists and user is participant
// 3. Check for duplicate by idempotency key
// 4. Create message
// 5. Update room's last_message_at
// 6. Upsert read state for sender
// 7. Emit outbox event
// 8. COMMIT
//
// Business rules:
// - Idempotency key is required
// - Sender must be a room participant
// - Returns existing message if duplicate (idempotent)
// - BLOCK ENFORCEMENT: Direct rooms WITHOUT context are subject to recipient block check
func (s *Service) SendMessage(
	ctx context.Context,
	roomID, senderID uuid.UUID,
	messageType chatEntity.MessageType,
	body *string,
	attachmentJSON map[string]interface{},
	idempotencyKey string,
) (*chatEntity.ChatMessage, error) {
	if idempotencyKey == "" {
		return nil, chatRepo.ErrInvalidIdempotencyKey
	}

	// Rate limit: 5 messages per 5 seconds (short-term burst protection)
	shortTermKey := fmt.Sprintf("chat:msg:%s", senderID.String())
	if !s.rateLimiter.Allow(shortTermKey, 5, 5*time.Second) {
		if s.metrics != nil {
			s.metrics.RecordChatRateLimited()
		}
		return nil, chatRepo.ErrRateLimited
	}

	// Rate limit: 60 messages per minute (long-term guard)
	longTermKey := fmt.Sprintf("chat:msg:min:%s", senderID.String())
	if !s.rateLimiter.Allow(longTermKey, 60, 1*time.Minute) {
		if s.metrics != nil {
			s.metrics.RecordChatRateLimited()
		}
		return nil, chatRepo.ErrRateLimited
	}

	if !messageType.IsValid() {
		return nil, chatRepo.ErrInvalidMessageType
	}

	// Validate body length for text messages
	if messageType == chatEntity.MessageTypeText && body != nil {
		if len(*body) == 0 {
			return nil, fmt.Errorf("text message body cannot be empty")
		}
		if len(*body) > MaxMessageBodyLength {
			return nil, chatRepo.ErrMessageBodyTooLong
		}
	}

	// For text messages, body is required
	if messageType == chatEntity.MessageTypeText && body == nil {
		return nil, fmt.Errorf("text message requires a body")
	}

	// Account status enforcement: sender must be active before any persistence.
	// Service re-checks independently of RequireActiveAccount middleware.
	if s.statusChecker != nil {
		if err := s.statusChecker.EnsureActive(ctx, senderID); err != nil {
			return nil, err
		}
	}

	var message *chatEntity.ChatMessage
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		// Verify room exists
		room, err := s.repo.GetRoomByID(ctx, tx, roomID)
		if err != nil {
			return fmt.Errorf("room not found: %w", err)
		}

		// Verify sender is a participant
		if !room.HasParticipant(senderID) {
			return chatRepo.ErrParticipantMismatch
		}

		// Block enforcement: direct and negotiation rooms are blocked unless
		// order-linked (commerce continuity). Support rooms are always exempt.
		// HasOrderContext() is the explicit carve-out; HasContext() (UI hint) is NOT a bypass.
		if room.RoomType != chatEntity.RoomTypeSupport && !room.HasOrderContext() {
			recipientID := room.OtherParticipant(senderID)
			blocked, err := s.socialRepo.ExistsBlock(ctx, tx, senderID, recipientID)
			if err != nil {
				return fmt.Errorf("failed to check block: %w", err)
			}
			if blocked {
				return chatRepo.ErrUserBlocked
			}
		}

		// Validate attachment references exist
		if attachmentJSON != nil {
			if err := s.validateAttachmentReferences(ctx, tx, senderID, attachmentJSON); err != nil {
				return err
			}
		}

		// Check for existing message by idempotency key
		existingMessage, err := s.repo.GetMessageByIdempotencyKey(ctx, tx, idempotencyKey)
		if err == nil {
			message = existingMessage
			return nil // Return existing message (idempotent)
		}
		if err != chatRepo.ErrMessageNotFound {
			return fmt.Errorf("failed to check idempotency: %w", err)
		}

		// Create new message
		newMessage := chatEntity.NewChatMessage(
			roomID,
			senderID,
			messageType,
			body,
			attachmentJSON,
			idempotencyKey,
		)

		if err := s.repo.CreateMessage(ctx, tx, newMessage); err != nil {
			return fmt.Errorf("failed to create message: %w", err)
		}

		// Update room's last_message_at
		if err := s.repo.UpdateRoomLastMessageAt(ctx, tx, roomID, newMessage.CreatedAt); err != nil {
			return fmt.Errorf("failed to update room last_message_at: %w", err)
		}

		// Mark message as read for sender immediately
		readState := chatEntity.NewChatReadStateWithTimestamp(roomID, senderID, newMessage.CreatedAt)
		if err := s.repo.UpsertReadState(ctx, tx, readState); err != nil {
			return fmt.Errorf("failed to upsert read state: %w", err)
		}

		// Emit outbox event for notification delivery
		// Recipient is the other participant
		recipientID := room.OtherParticipant(senderID)
		payload := map[string]any{
			"room_id":      roomID.String(),
			"message_id":   newMessage.ID.String(),
			"sender_id":    senderID.String(),
			"recipient_id": recipientID.String(),
			"message_type": string(newMessage.MessageType),
			"created_at":   newMessage.CreatedAt.UTC().Format(time.RFC3339),
		}

		// Idempotency key: chat.message.sent.{messageID}
		outboxIdempotencyKey := fmt.Sprintf("chat.message.sent.%s", newMessage.ID.String())
		if err := s.outboxRepo.InsertTx(ctx, tx, "chat.message.sent", payload, outboxIdempotencyKey); err != nil {
			return fmt.Errorf("insert outbox event failed: %w", err)
		}

		// Emit room-list update events for each real participant.
		// The summary is viewer-scoped so unread_count and other_user_id
		// match the REST room-list shape for the receiving client.
		for _, viewerID := range []uuid.UUID{room.ParticipantA, room.ParticipantB} {
			if viewerID == uuid.Nil {
				continue
			}

			unreadCount, err := s.repo.GetUnreadCountByRoomAndUser(ctx, tx, roomID, viewerID)
			if err != nil {
				return fmt.Errorf("failed to get unread count for room.updated: %w", err)
			}

			roomPayload := buildChatRoomUpdatedOutboxPayload(room, viewerID, viewerID, newMessage, unreadCount, false)
			roomKey := fmt.Sprintf("chat.room.updated.%s.%s", newMessage.ID.String(), viewerID.String())
			if err := s.outboxRepo.InsertTx(ctx, tx, realtime.EventTypeChatRoomUpdated, roomPayload, roomKey); err != nil {
				return fmt.Errorf("insert room.updated outbox event failed: %w", err)
			}
		}

		// SUPPORT USER REPLY HOOK: When a real user (not system) sends a message
		// in a support room, emit an event so the support service can transition
		// the ticket status from waiting_user → in_progress.
		if room.RoomType == chatEntity.RoomTypeSupport && senderID != uuid.Nil {
			supportPayload := map[string]any{
				"room_id":    roomID.String(),
				"sender_id":  senderID.String(),
				"message_id": newMessage.ID.String(),
			}
			supportKey := fmt.Sprintf("support.user_replied.%s", newMessage.ID.String())
			if err := s.outboxRepo.InsertTx(ctx, tx, "support.user_replied", supportPayload, supportKey); err != nil {
				// Non-fatal: ticket transition is best-effort, message delivery is primary
				if s.log != nil {
					s.log.Warn("failed to emit support.user_replied event",
						zap.String("room_id", roomID.String()),
						zap.Error(err),
					)
				}
			}
		}

		message = newMessage
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Record metrics for successful message send
	if s.metrics != nil {
		s.metrics.RecordChatMessage()
	}

	return message, nil
}

// SendTextMessage sends a text message (convenience method).
func (s *Service) SendTextMessage(
	ctx context.Context,
	roomID, senderID uuid.UUID,
	body string,
	idempotencyKey string,
) (*chatEntity.ChatMessage, error) {
	return s.SendMessage(
		ctx,
		roomID,
		senderID,
		chatEntity.MessageTypeText,
		&body,
		nil,
		idempotencyKey,
	)
}

// ListMessages lists messages in a room.
//
// Uses cursor-based pagination on (created_at, id).
// Returns messages ordered by created_at DESC (newest first).
//
// Business rules:
// - User must be a room participant
func (s *Service) ListMessages(
	ctx context.Context,
	roomID, userID uuid.UUID,
	cursorCreatedAt *time.Time,
	cursorID *uuid.UUID,
	limit int,
) ([]*chatEntity.ChatMessage, error) {
	var messages []*chatEntity.ChatMessage
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		// Verify room exists and user is participant
		room, err := s.repo.GetRoomByID(ctx, tx, roomID)
		if err != nil {
			return fmt.Errorf("room not found: %w", err)
		}

		if !room.HasParticipant(userID) {
			return chatRepo.ErrParticipantMismatch
		}

		// List messages
		messages, err = s.repo.ListMessagesByRoom(ctx, tx, roomID, cursorCreatedAt, cursorID, limit)
		return err
	})

	if err != nil {
		return nil, err
	}

	return messages, nil
}

// ========================================================================
// READ STATE OPERATIONS
// ========================================================================

// MarkAsRead marks all messages in a room as read for a user.
//
// Transaction flow:
//  1. BEGIN
//  2. Verify room exists and user is participant
//  3. Read existing read state
//  4. Upsert read state when timestamp advances
//  5. Recompute unread count and emit room.updated only when the viewer's
//     room-list state actually changes
//  6. COMMIT
//
// Business rules:
// - User must be a room participant
// - Idempotent (upsert operation)
func (s *Service) MarkAsRead(
	ctx context.Context,
	roomID, userID uuid.UUID,
	timestamp time.Time,
) error {
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		// Verify room exists and user is participant
		room, err := s.repo.GetRoomByID(ctx, tx, roomID)
		if err != nil {
			return fmt.Errorf("room not found: %w", err)
		}

		if !room.HasParticipant(userID) {
			return chatRepo.ErrParticipantMismatch
		}

		beforeUnreadCount, err := s.repo.GetUnreadCountByRoomAndUser(ctx, tx, roomID, userID)
		if err != nil {
			return fmt.Errorf("failed to get unread count before read update: %w", err)
		}

		existingReadState, err := s.repo.GetReadState(ctx, tx, roomID, userID)
		if err != nil && err != chatRepo.ErrReadStateNotFound {
			return fmt.Errorf("failed to get read state: %w", err)
		}
		if err == nil && !timestamp.After(existingReadState.LastReadAt) {
			return nil
		}

		// Upsert read state
		readState := chatEntity.NewChatReadStateWithTimestamp(roomID, userID, timestamp)
		if err := s.repo.UpsertReadState(ctx, tx, readState); err != nil {
			return fmt.Errorf("failed to upsert read state: %w", err)
		}

		afterUnreadCount, err := s.repo.GetUnreadCountByRoomAndUser(ctx, tx, roomID, userID)
		if err != nil {
			return fmt.Errorf("failed to get unread count after read update: %w", err)
		}

		if afterUnreadCount == beforeUnreadCount {
			return nil
		}

		latestMessages, err := s.repo.ListMessagesByRoom(ctx, tx, roomID, nil, nil, 1)
		if err != nil {
			return fmt.Errorf("failed to load latest room message: %w", err)
		}

		var latestMessage *chatEntity.ChatMessage
		if len(latestMessages) > 0 {
			latestMessage = latestMessages[0]
		}

		payload := buildChatRoomUpdatedOutboxPayload(room, userID, userID, latestMessage, afterUnreadCount, true)
		outboxKey := fmt.Sprintf("chat.room.updated.read.%s.%s.%d", roomID.String(), userID.String(), timestamp.UTC().UnixNano())
		if err := s.outboxRepo.InsertTx(ctx, tx, realtime.EventTypeChatRoomUpdated, payload, outboxKey); err != nil {
			return fmt.Errorf("insert room.updated read-state outbox event failed: %w", err)
		}

		return nil
	})
}

// GetReadState retrieves the read state for a user in a room.
func (s *Service) GetReadState(ctx context.Context, roomID, userID uuid.UUID) (*chatEntity.ChatReadState, error) {
	var state *chatEntity.ChatReadState
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		state, err = s.repo.GetReadState(ctx, tx, roomID, userID)
		return err
	})

	if err != nil {
		return nil, err
	}

	return state, nil
}

// GetUnreadCount calculates the unread message count for a user in a room.
//
// IMPORTANT: This is a read-only computation.
// We do NOT store unread_count in the database.
//
// MUTE SUPPRESSION (C6C): Messages from senders that the user has muted are
// excluded from the unread count. The messages still exist and are visible
// when the user opens the room — only the count computation is affected.
// This aligns with Option B mute semantics: suppress notifications + badge,
// but preserve message history.
//
// Returns 0 if there's no read state (all messages are unread).
func (s *Service) GetUnreadCount(ctx context.Context, roomID, userID uuid.UUID) (int, error) {
	var count int
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		// Verify room exists and user is participant
		room, err := s.repo.GetRoomByID(ctx, tx, roomID)
		if err != nil {
			return fmt.Errorf("room not found: %w", err)
		}

		if !room.HasParticipant(userID) {
			return chatRepo.ErrParticipantMismatch
		}

		// Mute exclusion subquery: exclude messages from senders the user has muted.
		// Uses NOT IN with a correlated subquery against user_mutes.
		// If user has no mutes, the subquery returns empty set (no filtering).
		const muteExclusion = `AND sender_id NOT IN (SELECT muted_id FROM user_mutes WHERE muter_id = `

		// Get read state
		readState, err := s.repo.GetReadState(ctx, tx, roomID, userID)
		if err != nil {
			if err == chatRepo.ErrReadStateNotFound {
				// No read state means all messages are unread
				// Count all messages in room (excluding muted senders)
				query := `SELECT COUNT(*) FROM chat_messages WHERE room_id = $1 AND deleted_at IS NULL ` + muteExclusion + `$2)`
				return tx.QueryRow(ctx, query, roomID, userID).Scan(&count)
			}
			return fmt.Errorf("failed to get read state: %w", err)
		}

		// Count messages created after last_read_at (excluding muted senders)
		query := `SELECT COUNT(*) FROM chat_messages WHERE room_id = $1 AND deleted_at IS NULL AND created_at > $2 ` + muteExclusion + `$3)`
		return tx.QueryRow(ctx, query, roomID, readState.LastReadAt, userID).Scan(&count)
	})

	if err != nil {
		return 0, err
	}

	return count, nil
}

// SoftHideForModeration soft-hides a chat message and emits room-list updates.
//
// The moderation key is used for outbox idempotency so retries do not double
// publish room.updated events.
func (s *Service) SoftHideForModeration(
	ctx context.Context,
	tx db.Tx,
	messageID uuid.UUID,
	deletedBy uuid.UUID,
	reason string,
	moderationKey string,
) error {
	message, err := s.repo.GetMessageByID(ctx, tx, messageID)
	if err != nil {
		return err
	}

	room, err := s.repo.GetRoomByID(ctx, tx, message.RoomID)
	if err != nil {
		return err
	}

	if err := s.repo.SoftHideForModeration(ctx, tx, messageID, deletedBy, reason); err != nil {
		return err
	}

	return s.emitModerationRoomUpdated(ctx, tx, room, moderationKey, "hidden")
}

// RestoreFromModeration restores a soft-hidden chat message and emits
// room-list updates.
func (s *Service) RestoreFromModeration(
   ctx context.Context,
   tx db.Tx,
   messageID uuid.UUID,
   moderationKey string,
) error {
   message, err := s.repo.GetMessageByID(ctx, tx, messageID)
   if err != nil {
           return err
   }

   // Restore is only meaningful for moderation-hidden messages. If the
   // message is already visible, treat this as a deterministic no-op so the
   // caller cannot broaden state through a restore retry.
   if message.DeletedAt == nil {
           return nil
   }

   room, err := s.repo.GetRoomByID(ctx, tx, message.RoomID)
   if err != nil {
           return err
   }

	if err := s.repo.RestoreFromModeration(ctx, tx, messageID); err != nil {
		return err
	}

	return s.emitModerationRoomUpdated(ctx, tx, room, moderationKey, "restored")
}

func (s *Service) emitModerationRoomUpdated(
	ctx context.Context,
	tx db.Tx,
	room *chatEntity.ChatRoom,
	moderationKey string,
	action string,
) error {
	latestMessages, err := s.repo.ListMessagesByRoom(ctx, tx, room.ID, nil, nil, 1)
	if err != nil {
		return fmt.Errorf("failed to load latest room message for moderation update: %w", err)
	}

	var latestMessage *chatEntity.ChatMessage
	if len(latestMessages) > 0 {
		latestMessage = latestMessages[0]
	}

	for _, viewerID := range []uuid.UUID{room.ParticipantA, room.ParticipantB} {
		if viewerID == uuid.Nil {
			continue
		}

		unreadCount, err := s.repo.GetUnreadCountByRoomAndUser(ctx, tx, room.ID, viewerID)
		if err != nil {
			return fmt.Errorf("failed to get unread count for moderation room.updated: %w", err)
		}

		payload := buildChatRoomUpdatedOutboxPayload(room, viewerID, viewerID, latestMessage, unreadCount, true)
		idempotencyKey := fmt.Sprintf("chat.room.updated.moderation.%s.%s.%s.%s", action, moderationKey, room.ID.String(), viewerID.String())
		if err := s.outboxRepo.InsertTx(ctx, tx, realtime.EventTypeChatRoomUpdated, payload, idempotencyKey); err != nil {
			return fmt.Errorf("insert moderation room.updated outbox event failed: %w", err)
		}
	}

	return nil
}

// validateAttachmentReferences validates that attachment references exist.
// Only checks fixed-price sale and auction attachments for data integrity.
// Skip validation if checker is not provided (backward compatibility).
func (s *Service) validateAttachmentReferences(
	ctx context.Context,
	tx db.Tx,
	senderID uuid.UUID,
	attachmentJSON map[string]interface{},
) error {
	attachmentTypeRaw, hasType := attachmentJSON["type"]
	if !hasType {
		// Type should be validated by attachment_validator before reaching here
		return nil
	}

	attachmentType, ok := attachmentTypeRaw.(string)
	if !ok {
		return nil
	}

	data, _ := attachmentJSON["data"].(map[string]interface{})
	if data == nil {
		// Structural validation should already reject missing/invalid data.
		return nil
	}

	switch attachmentType {
	case "reference":
		targetType, _ := data["target_type"].(string)
		targetIDStr, _ := data["target_id"].(string)
		if targetType == "" || targetIDStr == "" {
			return nil
		}
		targetID, err := uuid.Parse(targetIDStr)
		if err != nil {
			return nil
		}
		switch targetType {
		case "for_sale":
			return s.validateForSaleReferenceExists(ctx, tx, targetID)
		case "auction":
			return s.validateAuctionReferenceExists(ctx, tx, targetID)
		case "post":
			return s.validateContentReferenceExists(ctx, tx, targetID, "post", chatRepo.ErrAttachmentPostNotFound)
		case "request":
			return s.validateContentReferenceExists(ctx, tx, targetID, "request", chatRepo.ErrAttachmentRequestNotFound)
		case "profile":
			return s.validateProfileReferenceExists(ctx, tx, senderID, targetID)
		}

	case "for_sale":
		// Validate fixed-price sale exists
		forSaleIDRaw, hasID := data["for_sale_id"]
		if !hasID {
			return nil // Missing ID should be caught by structural validation
		}
		forSaleIDStr, ok := forSaleIDRaw.(string)
		if !ok {
			return nil
		}
		forSaleID, err := uuid.Parse(forSaleIDStr)
		if err != nil {
			return nil // Invalid UUID should be caught by structural validation
		}
		return s.validateForSaleReferenceExists(ctx, tx, forSaleID)

	case "auction":
		// Validate auction exists
		auctionIDRaw, hasID := data["auction_id"]
		if !hasID {
			return nil // Missing ID should be caught by structural validation
		}
		auctionIDStr, ok := auctionIDRaw.(string)
		if !ok {
			return nil
		}
		auctionID, err := uuid.Parse(auctionIDStr)
		if err != nil {
			return nil // Invalid UUID should be caught by structural validation
		}
		return s.validateAuctionReferenceExists(ctx, tx, auctionID)
	}

	return nil
}

func (s *Service) validateForSaleReferenceExists(ctx context.Context, tx db.Tx, forSaleID uuid.UUID) error {
	if s.forSaleChecker == nil {
		if s.log != nil {
			s.log.Warn("attachment validation skipped: missing fixed-price sale checker")
		}
		return nil
	}
	_, err := s.forSaleChecker.GetByID(ctx, tx, forSaleID)
	if err != nil {
		return chatRepo.ErrAttachmentForSaleNotFound
	}
	return nil
}

func (s *Service) validateAuctionReferenceExists(ctx context.Context, tx db.Tx, auctionID uuid.UUID) error {
	if s.auctionChecker == nil {
		if s.log != nil {
			s.log.Warn("attachment validation skipped: missing auction checker")
		}
		return nil
	}
	_, err := s.auctionChecker.GetByID(ctx, tx, auctionID)
	if err != nil {
		return chatRepo.ErrAttachmentAuctionNotFound
	}
	return nil
}

func (s *Service) validateContentReferenceExists(
	ctx context.Context,
	tx db.Tx,
	contentID uuid.UUID,
	contentType string,
	notFoundErr error,
) error {
	var foundID uuid.UUID
	err := tx.QueryRow(
		ctx,
		`SELECT id FROM contents WHERE id = $1 AND type = $2 AND deleted_at IS NULL`,
		contentID,
		contentType,
	).Scan(&foundID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return notFoundErr
		}
		return err
	}
	return nil
}

func (s *Service) validateProfileReferenceExists(
	ctx context.Context,
	tx db.Tx,
	senderID, profileID uuid.UUID,
) error {
	blocked, err := s.socialRepo.ExistsBlock(ctx, tx, senderID, profileID)
	if err != nil {
		return err
	}
	if blocked {
		return chatRepo.ErrAttachmentProfileNotFound
	}

	var foundID uuid.UUID
	err = tx.QueryRow(
		ctx,
		`SELECT id FROM users WHERE id = $1 AND deleted_at IS NULL`,
		profileID,
	).Scan(&foundID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return chatRepo.ErrAttachmentProfileNotFound
		}
		return err
	}

	return nil
}
