package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/hishumi/backend/internal/interaction/chat/entity"
	chatRepo "github.com/hishumi/backend/internal/interaction/chat/repository"
	"github.com/hishumi/backend/pkg/db"
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

	// Support rooms have no agent participant: an unset participant_b is stored
	// as SQL NULL (migration 000104). All other room types always carry a real
	// participant_b.
	var participantBArg interface{}
	if room.ParticipantB != uuid.Nil {
		participantBArg = room.ParticipantB
	}

	_, err := toTx(tx).Exec(ctx, query,
		room.ID, room.RoomType, room.ParticipantA, participantBArg,
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
		SELECT id, room_type, participant_a,
		       COALESCE(participant_b, '00000000-0000-0000-0000-000000000000'::uuid) AS participant_b,
		       linked_order_id, created_at, updated_at, last_message_at
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
		SELECT id, room_type, participant_a,
		       COALESCE(participant_b, '00000000-0000-0000-0000-000000000000'::uuid) AS participant_b,
		       linked_order_id, created_at, updated_at, last_message_at
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
		SELECT id, room_type, participant_a,
		       COALESCE(participant_b, '00000000-0000-0000-0000-000000000000'::uuid) AS participant_b,
		       linked_order_id, created_at, updated_at, last_message_at
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
		SELECT id, room_type, participant_a,
		       COALESCE(participant_b, '00000000-0000-0000-0000-000000000000'::uuid) AS participant_b,
		       linked_order_id, created_at, updated_at, last_message_at
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
		SELECT id, room_type, participant_a,
		       COALESCE(participant_b, '00000000-0000-0000-0000-000000000000'::uuid) AS participant_b,
		       linked_order_id, created_at, updated_at, last_message_at
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

	// Idempotent insert: when the same sender already persisted this
	// idempotency key, DO NOTHING instead of raising a unique violation.
	// A raised unique violation would abort the enclosing transaction
	// (PostgreSQL), making the subsequent replay lookup impossible in the
	// same transaction — ON CONFLICT DO NOTHING keeps the transaction
	// usable so the caller can converge on the winner row.
	query := `
		INSERT INTO chat_messages (id, room_id, sender_id, message_type, body, attachment_json, idempotency_key, command_fingerprint, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (sender_id, idempotency_key) DO NOTHING
	`

	tag, err := toTx(tx).Exec(ctx, query,
		message.ID, message.RoomID, message.SenderID, message.MessageType,
		message.Body, attachmentJSONBytes, message.IdempotencyKey, message.CommandFingerprint, message.CreatedAt,
	)

	if err != nil {
		if db.IsUniqueViolation(err) {
			return chatRepo.ErrDuplicateMessage
		}
		return fmt.Errorf("create message failed: %w", err)
	}

	if tag.RowsAffected() == 0 {
		// The (sender_id, idempotency_key) row already exists.
		return chatRepo.ErrDuplicateMessage
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

// GetMessageByIdempotencyKey retrieves a message by (sender_id, idempotency_key).
//
// The lookup is actor-scoped to match the canonical uniqueness authority
// UNIQUE(sender_id, idempotency_key) (migration 000032). A global
// idempotency_key lookup is intentionally NOT used: it would let one sender
// observe/replay another sender's message whenever two senders happened to
// use the same opaque key.
func (r *ChatRepositoryImpl) GetMessageByIdempotencyKey(ctx context.Context, tx interface{}, senderID uuid.UUID, idempotencyKey string) (*entity.ChatMessage, error) {
	query := `
		SELECT id, room_id, sender_id, message_type, body, attachment_json, idempotency_key, command_fingerprint, created_at,
	       deleted_at, deleted_by, deletion_reason
		FROM chat_messages
		WHERE sender_id = $1 AND idempotency_key = $2
	`

	var message entity.ChatMessage
	var attachmentJSONBytes []byte

	err := toTx(tx).QueryRow(ctx, query, senderID, idempotencyKey).Scan(
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
// Thin delegation to the canonical batch authority — no independent SQL.
func (r *ChatRepositoryImpl) GetUnreadCountByRoomAndUser(
	ctx context.Context,
	tx interface{},
	roomID, userID uuid.UUID,
) (int, error) {
	counts, err := r.GetUnreadCountsByRoomIDs(ctx, tx, []uuid.UUID{roomID}, userID)
	if err != nil {
		return 0, err
	}
	return counts[roomID], nil
}

// GetUnreadCountsByRoomIDs is the SINGLE canonical unread-count authority.
//
// Formula: for each room, count messages that are
//   - visible (deleted_at IS NULL), and
//   - created after the viewer's last_read_at (all when no read state), and
//   - authored by someone the viewer has NOT muted.
//
// Batch by construction: one query for N rooms (no N+1).
func (r *ChatRepositoryImpl) GetUnreadCountsByRoomIDs(
	ctx context.Context,
	tx interface{},
	roomIDs []uuid.UUID,
	userID uuid.UUID,
) (map[uuid.UUID]int, error) {
	out := make(map[uuid.UUID]int, len(roomIDs))
	if len(roomIDs) == 0 {
		return out, nil
	}
	for _, roomID := range roomIDs {
		out[roomID] = 0
	}

	const q = `
		WITH target_rooms AS (
			SELECT UNNEST($1::uuid[]) AS room_id
		),
		room_read_states AS (
			SELECT room_id, last_read_at
			FROM chat_read_states
			WHERE user_id = $2 AND room_id = ANY($1)
		)
		SELECT
			tr.room_id,
			COALESCE(COUNT(m.id), 0) AS unread_count
		FROM target_rooms tr
		LEFT JOIN room_read_states rs ON rs.room_id = tr.room_id
		LEFT JOIN chat_messages m ON
			m.room_id = tr.room_id
			AND m.deleted_at IS NULL
			AND (rs.last_read_at IS NULL OR m.created_at > rs.last_read_at)
			AND m.sender_id NOT IN (
				SELECT muted_id FROM user_mutes WHERE muter_id = $2
			)
		GROUP BY tr.room_id
	`

	rows, err := toTx(tx).Query(ctx, q, roomIDs, userID)
	if err != nil {
		return nil, fmt.Errorf("get unread counts failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			roomID uuid.UUID
			count  int
		)
		if err := rows.Scan(&roomID, &count); err != nil {
			return nil, fmt.Errorf("scan unread count failed: %w", err)
		}
		out[roomID] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate unread counts failed: %w", err)
	}

	return out, nil
}

// ========================================================================
// MEDIA ASSET OPERATIONS
// ========================================================================

// scanMediaAsset reads one chat media asset row in the canonical column order.
func scanMediaAsset(scan func(dest ...interface{}) error) (*entity.ChatMediaAsset, error) {
	var asset entity.ChatMediaAsset
	err := scan(
		&asset.ID, &asset.RoomID, &asset.UploaderID, &asset.MediaType, &asset.ContentType, &asset.StorageKey,
		&asset.ThumbnailStorageKey, &asset.ByteSize, &asset.Width, &asset.Height, &asset.DurationMs,
		&asset.Status, &asset.ExpiresAt, &asset.CreatedAt, &asset.FinalizedAt,
		&asset.DeletedAt, &asset.DeletedBy, &asset.DeletionReason,
	)
	if err != nil {
		return nil, err
	}
	return &asset, nil
}

// CreateMediaAsset registers a PENDING room-scoped media asset. The row is the
// register step of register → upload → attach: it exists before any byte
// reaches S3, which is what lets the sweep drop abandoned uploads.
func (r *ChatRepositoryImpl) CreateMediaAsset(ctx context.Context, tx interface{}, asset *entity.ChatMediaAsset) error {
	query := `
		INSERT INTO chat_media_assets (
			id, room_id, uploader_id, media_type, content_type, storage_key,
			thumbnail_storage_key, byte_size, width, height, duration_ms,
			status, expires_at, created_at, finalized_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
	`

	_, err := toTx(tx).Exec(ctx, query,
		asset.ID, asset.RoomID, asset.UploaderID, asset.MediaType, asset.ContentType, asset.StorageKey,
		asset.ThumbnailStorageKey, asset.ByteSize, asset.Width, asset.Height, asset.DurationMs,
		asset.Status, asset.ExpiresAt, asset.CreatedAt, asset.FinalizedAt,
	)
	if err != nil {
		return fmt.Errorf("create chat media asset failed: %w", err)
	}
	return nil
}

// GetMediaAssetsByIDs fetches assets by id. Missing ids are simply absent.
func (r *ChatRepositoryImpl) GetMediaAssetsByIDs(ctx context.Context, tx interface{}, ids []uuid.UUID) ([]*entity.ChatMediaAsset, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	query := `
		SELECT id, room_id, uploader_id, media_type, content_type, storage_key,
		       thumbnail_storage_key, byte_size, width, height, duration_ms,
		       status, expires_at, created_at, finalized_at, deleted_at, deleted_by, deletion_reason
		FROM chat_media_assets
		WHERE id = ANY($1)
	`

	rows, err := toTx(tx).Query(ctx, query, ids)
	if err != nil {
		return nil, fmt.Errorf("get chat media assets failed: %w", err)
	}
	defer rows.Close()

	var assets []*entity.ChatMediaAsset
	for rows.Next() {
		asset, err := scanMediaAsset(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("scan chat media asset failed: %w", err)
		}
		assets = append(assets, asset)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chat media assets failed: %w", err)
	}

	return assets, nil
}

// FinalizeMediaAsset flips pending → finalized and extends the read lifetime.
//
// The `status = 'pending'` guard IS the concurrency control: a racing attach or
// a sweep that already claimed the row makes this UPDATE affect zero rows, and
// the caller's transaction fails with ErrMediaAssetNotAttachable instead of
// double-attaching one asset.
func (r *ChatRepositoryImpl) FinalizeMediaAsset(ctx context.Context, tx interface{}, assetID uuid.UUID, finalizedAt time.Time) error {
	query := `
		UPDATE chat_media_assets
		SET status = $2, finalized_at = $3, expires_at = $4
		WHERE id = $1 AND status = 'pending'
	`

	tag, err := toTx(tx).Exec(ctx, query,
		assetID,
		entity.ChatMediaAssetStatusFinalized,
		finalizedAt,
		finalizedAt.Add(entity.PermanentAssetTTL),
	)
	if err != nil {
		return fmt.Errorf("finalize chat media asset failed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return chatRepo.ErrMediaAssetNotAttachable
	}
	return nil
}

// LinkMediaAssetToMessage records message ↔ asset with its display order.
// ON CONFLICT DO NOTHING keeps a retried attach idempotent.
func (r *ChatRepositoryImpl) LinkMediaAssetToMessage(ctx context.Context, tx interface{}, messageID, assetID uuid.UUID, sortOrder int) error {
	query := `
		INSERT INTO chat_message_media_assets (message_id, media_asset_id, sort_order)
		VALUES ($1,$2,$3)
		ON CONFLICT (message_id, media_asset_id) DO NOTHING
	`

	if _, err := toTx(tx).Exec(ctx, query, messageID, assetID, sortOrder); err != nil {
		return fmt.Errorf("link chat media asset failed: %w", err)
	}
	return nil
}

// MarkMessageHasMedia sets chat_messages.has_media.
func (r *ChatRepositoryImpl) MarkMessageHasMedia(ctx context.Context, tx interface{}, messageID uuid.UUID) error {
	const query = `UPDATE chat_messages SET has_media = true WHERE id = $1`
	if _, err := toTx(tx).Exec(ctx, query, messageID); err != nil {
		return fmt.Errorf("mark chat message has_media failed: %w", err)
	}
	return nil
}

// ListMediaAssetsByMessageIDs batch-fetches the media of a page of messages.
//
// One query for the whole page (no N+1). The asset's SortOrder is hydrated from
// the LINK row: ordering is a property of "this asset inside that message", and
// chat_media_assets itself has no sort_order column. Deleted assets are
// excluded so a swept upload can never render.
func (r *ChatRepositoryImpl) ListMediaAssetsByMessageIDs(ctx context.Context, tx interface{}, messageIDs []uuid.UUID) (map[uuid.UUID][]*entity.ChatMediaAsset, error) {
	out := make(map[uuid.UUID][]*entity.ChatMediaAsset, len(messageIDs))
	if len(messageIDs) == 0 {
		return out, nil
	}

	query := `
		SELECT a.id, a.room_id, a.uploader_id, a.media_type, a.content_type, a.storage_key,
		       a.thumbnail_storage_key, a.byte_size, a.width, a.height, a.duration_ms,
		       a.status, a.expires_at, a.created_at, a.finalized_at, a.deleted_at, a.deleted_by, a.deletion_reason,
		       l.message_id, l.sort_order
		FROM chat_message_media_assets l
		JOIN chat_media_assets a ON a.id = l.media_asset_id
		WHERE l.message_id = ANY($1) AND a.status <> 'deleted'
		ORDER BY l.message_id, l.sort_order
	`

	rows, err := toTx(tx).Query(ctx, query, messageIDs)
	if err != nil {
		return nil, fmt.Errorf("list chat message media failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			asset     entity.ChatMediaAsset
			messageID uuid.UUID
			sortOrder int
		)
		if err := rows.Scan(
			&asset.ID, &asset.RoomID, &asset.UploaderID, &asset.MediaType, &asset.ContentType, &asset.StorageKey,
			&asset.ThumbnailStorageKey, &asset.ByteSize, &asset.Width, &asset.Height, &asset.DurationMs,
			&asset.Status, &asset.ExpiresAt, &asset.CreatedAt, &asset.FinalizedAt,
			&asset.DeletedAt, &asset.DeletedBy, &asset.DeletionReason,
			&messageID, &sortOrder,
		); err != nil {
			return nil, fmt.Errorf("scan chat message media failed: %w", err)
		}
		asset.SortOrder = sortOrder
		out[messageID] = append(out[messageID], &asset)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chat message media failed: %w", err)
	}

	return out, nil
}

// CleanupExpiredPendingMediaAssets marks expired PENDING assets deleted. Only
// pending rows are eligible — finalized assets are message content and are
// never swept by TTL.
func (r *ChatRepositoryImpl) CleanupExpiredPendingMediaAssets(ctx context.Context, tx interface{}, now time.Time) (int64, error) {
	const query = `
		UPDATE chat_media_assets
		SET status = $1, deleted_at = $2, deletion_reason = 'expired'
		WHERE status = 'pending' AND expires_at <= $2
	`

	tag, err := toTx(tx).Exec(ctx, query, entity.ChatMediaAssetStatusDeleted, now)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired chat media assets failed: %w", err)
	}
	return tag.RowsAffected(), nil
}
