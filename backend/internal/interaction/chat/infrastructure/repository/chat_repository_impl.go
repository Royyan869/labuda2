package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/interaction/chat/entity"
	chatRepo "github.com/labuda/backend/internal/interaction/chat/repository"
	"github.com/labuda/backend/pkg/db"
)

// ChatRepositoryImpl implements the chat repository using pgx.
type ChatRepositoryImpl struct{}

// NewChatRepository creates a new ChatRepository.
func NewChatRepository() chatRepo.Repository {
	return &ChatRepositoryImpl{}
}

// toTx wraps the transaction interface for type casting.
func toTx(tx interface{}) db.Tx {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		panic(fmt.Sprintf("invalid transaction type: %T", tx))
	}
	return dbTx
}

// ========================================================================
// ROOM OPERATIONS
// ========================================================================

// CreateRoom creates a new chat room.
func (r *ChatRepositoryImpl) CreateRoom(ctx context.Context, tx interface{}, room *entity.ChatRoom) error {
	query := `
		INSERT INTO chat_rooms (id, room_type, participant_a, participant_b, linked_order_id, created_at, updated_at, last_message_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := toTx(tx).Exec(ctx, query,
		room.ID, room.RoomType, room.ParticipantA, room.ParticipantB,
		room.LinkedOrderID,
		room.CreatedAt, room.UpdatedAt, room.LastMessageAt,
	)

	if err != nil {
		if db.IsUniqueViolation(err) {
			return chatRepo.ErrDuplicateRoom
		}
		return fmt.Errorf("create room failed: %w", err)
	}

	return nil
}

// GetRoomByID retrieves a room by ID.
func (r *ChatRepositoryImpl) GetRoomByID(ctx context.Context, tx interface{}, roomID uuid.UUID) (*entity.ChatRoom, error) {
	query := `
		SELECT id, room_type, participant_a, participant_b, linked_order_id, created_at, updated_at, last_message_at
		FROM chat_rooms
		WHERE id = $1
	`

	var room entity.ChatRoom
	err := toTx(tx).QueryRow(ctx, query, roomID).Scan(
		&room.ID, &room.RoomType, &room.ParticipantA, &room.ParticipantB,
		&room.LinkedOrderID,
		&room.CreatedAt, &room.UpdatedAt, &room.LastMessageAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, chatRepo.ErrRoomNotFound
		}
		return nil, fmt.Errorf("get room by id failed: %w", err)
	}

	return &room, nil
}

// GetRoomByIDForUpdate retrieves a room by ID and locks the row for the
// duration of the transaction.
func (r *ChatRepositoryImpl) GetRoomByIDForUpdate(ctx context.Context, tx interface{}, roomID uuid.UUID) (*entity.ChatRoom, error) {
	query := `
		SELECT id, room_type, participant_a, participant_b, linked_order_id, created_at, updated_at, last_message_at
		FROM chat_rooms
		WHERE id = $1
		FOR UPDATE
	`

	var room entity.ChatRoom
	err := toTx(tx).QueryRow(ctx, query, roomID).Scan(
		&room.ID, &room.RoomType, &room.ParticipantA, &room.ParticipantB,
		&room.LinkedOrderID,
		&room.CreatedAt, &room.UpdatedAt, &room.LastMessageAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, chatRepo.ErrRoomNotFound
		}
		return nil, fmt.Errorf("get room by id for update failed: %w", err)
	}

	return &room, nil
}

// GetDirectRoom retrieves a direct room between two users.
func (r *ChatRepositoryImpl) GetDirectRoom(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (*entity.ChatRoom, error) {
	// Sort participants to match the stored order
	var participantA, participantB uuid.UUID
	if userA.String() < userB.String() {
		participantA = userA
		participantB = userB
	} else {
		participantA = userB
		participantB = userA
	}

	query := `
		SELECT id, room_type, participant_a, participant_b, linked_order_id, created_at, updated_at, last_message_at
		FROM chat_rooms
		WHERE participant_a = $1 AND participant_b = $2 AND room_type = 'direct'
	`

	var room entity.ChatRoom
	err := toTx(tx).QueryRow(ctx, query, participantA, participantB).Scan(
		&room.ID, &room.RoomType, &room.ParticipantA, &room.ParticipantB,
		&room.LinkedOrderID,
		&room.CreatedAt, &room.UpdatedAt, &room.LastMessageAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, chatRepo.ErrRoomNotFound
		}
		return nil, fmt.Errorf("get direct room failed: %w", err)
	}

	return &room, nil
}

// GetSupportRoom retrieves a support room for a user.
//
// Support rooms are identified by:
// - room_type = 'support'
// - One participant is the user, the other is uuid.Nil (system)
// - Participants are sorted deterministically by NewChatRoom (uuid.Nil is always smallest)
func (r *ChatRepositoryImpl) GetSupportRoom(ctx context.Context, tx interface{}, userID uuid.UUID) (*entity.ChatRoom, error) {
	// Sort participants to match the stored order (same as GetDirectRoom).
	// NewChatRoom sorts by UUID string; uuid.Nil ("00000000-...") is always participant_a.
	systemUUID := uuid.Nil
	var participantA, participantB uuid.UUID
	if userID.String() < systemUUID.String() {
		participantA = userID
		participantB = systemUUID
	} else {
		participantA = systemUUID
		participantB = userID
	}

	query := `
		SELECT id, room_type, participant_a, participant_b, linked_order_id, created_at, updated_at, last_message_at
		FROM chat_rooms
		WHERE participant_a = $1 AND participant_b = $2 AND room_type = 'support'
	`

	var room entity.ChatRoom
	err := toTx(tx).QueryRow(ctx, query, participantA, participantB).Scan(
		&room.ID, &room.RoomType, &room.ParticipantA, &room.ParticipantB,
		&room.LinkedOrderID,
		&room.CreatedAt, &room.UpdatedAt, &room.LastMessageAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, chatRepo.ErrRoomNotFound
		}
		return nil, fmt.Errorf("get support room failed: %w", err)
	}

	return &room, nil
}

// ListRoomsByUser lists all rooms where the user is a participant.
// Uses cursor-based pagination on last_message_at.
func (r *ChatRepositoryImpl) ListRoomsByUser(
	ctx context.Context,
	tx interface{},
	userID uuid.UUID,
	cursorLastMessageAt *time.Time,
	cursorID *uuid.UUID,
	limit int,
) ([]*entity.ChatRoom, error) {
	// Query rooms where user is either participant_a or participant_b
	// Ordered by last_message_at DESC, id DESC for cursor pagination
	baseQuery := `
		SELECT id, room_type, participant_a, participant_b, linked_order_id, created_at, updated_at, last_message_at
		FROM chat_rooms
		WHERE participant_a = $1 OR participant_b = $1
	`

	args := []interface{}{userID}
	argIdx := 2

	// Add cursor conditions if provided
	if cursorLastMessageAt != nil && cursorID != nil {
		baseQuery += fmt.Sprintf(" AND (last_message_at, id) < ($%d, $%d)", argIdx, argIdx+1)
		args = append(args, *cursorLastMessageAt, *cursorID)
		argIdx += 2
	}

	// Add ordering and limit
	baseQuery += fmt.Sprintf(" ORDER BY last_message_at DESC, id DESC LIMIT $%d", argIdx)
	args = append(args, limit)

	rows, err := toTx(tx).Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("list rooms by user failed: %w", err)
	}
	defer rows.Close()

	var rooms []*entity.ChatRoom
	for rows.Next() {
		var room entity.ChatRoom
		err := rows.Scan(
			&room.ID, &room.RoomType, &room.ParticipantA, &room.ParticipantB,
			&room.LinkedOrderID,
			&room.CreatedAt, &room.UpdatedAt, &room.LastMessageAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan room failed: %w", err)
		}
		rooms = append(rooms, &room)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("list rooms scan failed: %w", rows.Err())
	}

	return rooms, nil
}

// GetRoomByOrderID retrieves a room by linked order ID.
func (r *ChatRepositoryImpl) GetRoomByOrderID(ctx context.Context, tx interface{}, orderID uuid.UUID) (*entity.ChatRoom, error) {
	query := `
		SELECT id, room_type, participant_a, participant_b, linked_order_id, created_at, updated_at, last_message_at
		FROM chat_rooms
		WHERE linked_order_id = $1
	`

	var room entity.ChatRoom
	err := toTx(tx).QueryRow(ctx, query, orderID).Scan(
		&room.ID, &room.RoomType, &room.ParticipantA, &room.ParticipantB,
		&room.LinkedOrderID,
		&room.CreatedAt, &room.UpdatedAt, &room.LastMessageAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, chatRepo.ErrRoomNotFound
		}
		return nil, fmt.Errorf("get room by order id failed: %w", err)
	}

	return &room, nil
}

// UpdateRoomLastMessageAt updates the last_message_at timestamp.
func (r *ChatRepositoryImpl) UpdateRoomLastMessageAt(ctx context.Context, tx interface{}, roomID uuid.UUID, timestamp time.Time) error {
	query := `
		UPDATE chat_rooms
		SET last_message_at = $1, updated_at = $2
		WHERE id = $3
	`

	result, err := toTx(tx).Exec(ctx, query, timestamp, timestamp, roomID)
	if err != nil {
		return fmt.Errorf("update room last_message_at failed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return chatRepo.ErrRoomNotFound
	}

	return nil
}

// UpdateRoomLinkedOrderId updates the room's linked order ID.
//
// This is used to link an order to a chat for commerce continuity.
// When an order is created or when navigating from order detail to chat,
// this creates the connection between order and chat.
func (r *ChatRepositoryImpl) UpdateRoomLinkedOrderId(
	ctx context.Context,
	tx interface{},
	roomID uuid.UUID,
	linkedOrderID *uuid.UUID,
) error {
	query := `
		UPDATE chat_rooms
		SET linked_order_id = $1, updated_at = $2
		WHERE id = $3
	`

	now := time.Now()
	result, err := toTx(tx).Exec(ctx, query, linkedOrderID, now, roomID)
	if err != nil {
		return fmt.Errorf("update room linked_order_id failed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return chatRepo.ErrRoomNotFound
	}

	return nil
}

// ========================================================================
// MESSAGE OPERATIONS
// ========================================================================

// CreateMessage creates a new chat message.
func (r *ChatRepositoryImpl) CreateMessage(ctx context.Context, tx interface{}, message *entity.ChatMessage) error {
	var attachmentJSONBytes []byte
	if message.AttachmentJSON != nil {
		var err error
		attachmentJSONBytes, err = json.Marshal(message.AttachmentJSON)
		if err != nil {
			return fmt.Errorf("marshal attachment_json failed: %w", err)
		}
	}

	query := `
		INSERT INTO chat_messages (id, room_id, sender_id, message_type, body, attachment_json, idempotency_key, command_fingerprint, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := toTx(tx).Exec(ctx, query,
		message.ID, message.RoomID, message.SenderID, message.MessageType,
		message.Body, attachmentJSONBytes, message.IdempotencyKey, message.CommandFingerprint, message.CreatedAt,
	)

	if err != nil {
		if db.IsUniqueViolation(err) {
			return chatRepo.ErrDuplicateMessage
		}
		return fmt.Errorf("create message failed: %w", err)
	}

	return nil
}

// GetMessageByID retrieves a message by ID.
func (r *ChatRepositoryImpl) GetMessageByID(ctx context.Context, tx interface{}, messageID uuid.UUID) (*entity.ChatMessage, error) {
	query := `
		SELECT id, room_id, sender_id, message_type, body, attachment_json, idempotency_key, command_fingerprint, created_at,
	       deleted_at, deleted_by, deletion_reason
		FROM chat_messages
		WHERE id = $1
	`

	var message entity.ChatMessage
	var attachmentJSONBytes []byte

	err := toTx(tx).QueryRow(ctx, query, messageID).Scan(
		&message.ID, &message.RoomID, &message.SenderID, &message.MessageType,
		&message.Body, &attachmentJSONBytes, &message.IdempotencyKey, &message.CommandFingerprint, &message.CreatedAt,
		&message.DeletedAt, &message.DeletedBy, &message.DeletionReason,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, chatRepo.ErrMessageNotFound
		}
		return nil, fmt.Errorf("get message by id failed: %w", err)
	}

	// Unmarshal attachment_json if present
	if attachmentJSONBytes != nil {
		if err := json.Unmarshal(attachmentJSONBytes, &message.AttachmentJSON); err != nil {
			return nil, fmt.Errorf("unmarshal attachment_json failed: %w", err)
		}
	}

	return &message, nil
}

// ListMessagesByRoom lists messages in a room.
// Uses cursor-based pagination on (created_at, id).
func (r *ChatRepositoryImpl) ListMessagesByRoom(
	ctx context.Context,
	tx interface{},
	roomID uuid.UUID,
	cursorCreatedAt *time.Time,
	cursorID *uuid.UUID,
	limit int,
) ([]*entity.ChatMessage, error) {
	baseQuery := `
		SELECT id, room_id, sender_id, message_type, body, attachment_json, idempotency_key, command_fingerprint, created_at,
	       deleted_at, deleted_by, deletion_reason
		FROM chat_messages
		WHERE room_id = $1
	`

	args := []interface{}{roomID}
	argIdx := 2

	// Add cursor conditions if provided
	if cursorCreatedAt != nil && cursorID != nil {
		baseQuery += fmt.Sprintf(" AND (created_at, id) < ($%d, $%d)", argIdx, argIdx+1)
		args = append(args, *cursorCreatedAt, *cursorID)
		argIdx += 2
	}

	// Add ordering and limit
	baseQuery += fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d", argIdx)
	args = append(args, limit)

	rows, err := toTx(tx).Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("list messages by room failed: %w", err)
	}
	defer rows.Close()

	var messages []*entity.ChatMessage
	for rows.Next() {
		var message entity.ChatMessage
		var attachmentJSONBytes []byte

		err := rows.Scan(
			&message.ID, &message.RoomID, &message.SenderID, &message.MessageType,
			&message.Body, &attachmentJSONBytes, &message.IdempotencyKey, &message.CommandFingerprint, &message.CreatedAt,
			&message.DeletedAt, &message.DeletedBy, &message.DeletionReason,
		)
		if err != nil {
			return nil, fmt.Errorf("scan message failed: %w", err)
		}

		// Unmarshal attachment_json if present
		if attachmentJSONBytes != nil {
			if err := json.Unmarshal(attachmentJSONBytes, &message.AttachmentJSON); err != nil {
				return nil, fmt.Errorf("unmarshal attachment_json failed: %w", err)
			}
		}

		messages = append(messages, &message)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("list messages scan failed: %w", rows.Err())
	}

	return messages, nil
}

// GetMessageByIdempotencyKey retrieves a message by idempotency key.
func (r *ChatRepositoryImpl) GetMessageByIdempotencyKey(ctx context.Context, tx interface{}, idempotencyKey string) (*entity.ChatMessage, error) {
	query := `
		SELECT id, room_id, sender_id, message_type, body, attachment_json, idempotency_key, command_fingerprint, created_at,
	       deleted_at, deleted_by, deletion_reason
		FROM chat_messages
		WHERE idempotency_key = $1
	`

	var message entity.ChatMessage
	var attachmentJSONBytes []byte

	err := toTx(tx).QueryRow(ctx, query, idempotencyKey).Scan(
		&message.ID, &message.RoomID, &message.SenderID, &message.MessageType,
		&message.Body, &attachmentJSONBytes, &message.IdempotencyKey, &message.CommandFingerprint, &message.CreatedAt,
		&message.DeletedAt, &message.DeletedBy, &message.DeletionReason,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, chatRepo.ErrMessageNotFound
		}
		return nil, fmt.Errorf("get message by idempotency key failed: %w", err)
	}

	// Unmarshal attachment_json if present
	if attachmentJSONBytes != nil {
		if err := json.Unmarshal(attachmentJSONBytes, &message.AttachmentJSON); err != nil {
			return nil, fmt.Errorf("unmarshal attachment_json failed: %w", err)
		}
	}

	return &message, nil
}

// ========================================================================
// MODERATION OPERATIONS
// ========================================================================

// SoftHideForModeration sets deleted_at/deleted_by/deletion_reason on a message.
// Idempotent: if message already hidden (deleted_at IS NOT NULL), returns nil.
func (r *ChatRepositoryImpl) SoftHideForModeration(
	ctx context.Context,
	tx interface{},
	messageID uuid.UUID,
	deletedBy uuid.UUID,
	reason string,
) error {
	query := `
		UPDATE chat_messages
		SET deleted_at = NOW(), deleted_by = $2, deletion_reason = $3
		WHERE id = $1 AND deleted_at IS NULL
	`

	_, err := toTx(tx).Exec(ctx, query, messageID, deletedBy, reason)
	if err != nil {
		return fmt.Errorf("soft-hide message for moderation failed: %w", err)
	}

	return nil
}

// RestoreFromModeration clears deleted_at/deleted_by/deletion_reason on a message.
// Idempotent: if message not hidden (deleted_at IS NULL), returns nil.
func (r *ChatRepositoryImpl) RestoreFromModeration(
	ctx context.Context,
	tx interface{},
	messageID uuid.UUID,
) error {
	query := `
		UPDATE chat_messages
		SET deleted_at = NULL, deleted_by = NULL, deletion_reason = NULL
		WHERE id = $1 AND deleted_at IS NOT NULL
	`

	_, err := toTx(tx).Exec(ctx, query, messageID)
	if err != nil {
		return fmt.Errorf("restore message from moderation failed: %w", err)
	}

	return nil
}

// ========================================================================
// READ STATE OPERATIONS
// ========================================================================

// CreateReadState creates a new read state.
func (r *ChatRepositoryImpl) CreateReadState(ctx context.Context, tx interface{}, state *entity.ChatReadState) error {
	query := `
		INSERT INTO chat_read_states (room_id, user_id, last_read_at)
		VALUES ($1, $2, $3)
	`

	_, err := toTx(tx).Exec(ctx, query, state.RoomID, state.UserID, state.LastReadAt)

	if err != nil {
		if db.IsUniqueViolation(err) {
			return chatRepo.ErrDuplicateReadState
		}
		return fmt.Errorf("create read state failed: %w", err)
	}

	return nil
}

// GetReadState retrieves the read state for a room and user.
func (r *ChatRepositoryImpl) GetReadState(ctx context.Context, tx interface{}, roomID, userID uuid.UUID) (*entity.ChatReadState, error) {
	query := `
		SELECT room_id, user_id, last_read_at
		FROM chat_read_states
		WHERE room_id = $1 AND user_id = $2
	`

	var state entity.ChatReadState
	err := toTx(tx).QueryRow(ctx, query, roomID, userID).Scan(
		&state.RoomID, &state.UserID, &state.LastReadAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, chatRepo.ErrReadStateNotFound
		}
		return nil, fmt.Errorf("get read state failed: %w", err)
	}

	return &state, nil
}

// UpdateReadState updates the last_read_at timestamp.
func (r *ChatRepositoryImpl) UpdateReadState(ctx context.Context, tx interface{}, state *entity.ChatReadState) error {
	query := `
		UPDATE chat_read_states
		SET last_read_at = $1
		WHERE room_id = $2 AND user_id = $3
	`

	result, err := toTx(tx).Exec(ctx, query, state.LastReadAt, state.RoomID, state.UserID)
	if err != nil {
		return fmt.Errorf("update read state failed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return chatRepo.ErrReadStateNotFound
	}

	return nil
}

// UpsertReadState creates or updates a read state.
func (r *ChatRepositoryImpl) UpsertReadState(ctx context.Context, tx interface{}, state *entity.ChatReadState) error {
	query := `
		INSERT INTO chat_read_states (room_id, user_id, last_read_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (room_id, user_id) DO UPDATE
		SET last_read_at = EXCLUDED.last_read_at
	`

	_, err := toTx(tx).Exec(ctx, query, state.RoomID, state.UserID, state.LastReadAt)
	if err != nil {
		return fmt.Errorf("upsert read state failed: %w", err)
	}

	return nil
}

// ListReadStatesByRoom lists all read states for a room.
func (r *ChatRepositoryImpl) ListReadStatesByRoom(ctx context.Context, tx interface{}, roomID uuid.UUID) ([]*entity.ChatReadState, error) {
	query := `
		SELECT room_id, user_id, last_read_at
		FROM chat_read_states
		WHERE room_id = $1
	`

	rows, err := toTx(tx).Query(ctx, query, roomID)
	if err != nil {
		return nil, fmt.Errorf("list read states by room failed: %w", err)
	}
	defer rows.Close()

	var states []*entity.ChatReadState
	for rows.Next() {
		var state entity.ChatReadState
		err := rows.Scan(&state.RoomID, &state.UserID, &state.LastReadAt)
		if err != nil {
			return nil, fmt.Errorf("scan read state failed: %w", err)
		}
		states = append(states, &state)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("list read states scan failed: %w", rows.Err())
	}

	return states, nil
}

// GetUnreadCountByRoomAndUser calculates the unread count for a single room/user pair.
//
// Mirrors the room-list unread projection, including mute suppression.
func (r *ChatRepositoryImpl) GetUnreadCountByRoomAndUser(
	ctx context.Context,
	tx interface{},
	roomID, userID uuid.UUID,
) (int, error) {
	const q = `
		WITH room_read_state AS (
			SELECT last_read_at
			FROM chat_read_states
			WHERE room_id = $1 AND user_id = $2
		)
		SELECT COALESCE(COUNT(m.id), 0) AS unread_count
		FROM chat_messages m
		LEFT JOIN room_read_state rs ON TRUE
		WHERE m.room_id = $1
			AND m.deleted_at IS NULL
			AND (rs.last_read_at IS NULL OR m.created_at > rs.last_read_at)
			AND m.sender_id NOT IN (
				SELECT muted_id FROM user_mutes WHERE muter_id = $2
			)
	`

	var count int
	err := toTx(tx).QueryRow(ctx, q, roomID, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("get unread count failed: %w", err)
	}

	return count, nil
}
