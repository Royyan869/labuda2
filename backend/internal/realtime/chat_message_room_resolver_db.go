package realtime

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	dbpkg "github.com/labuda/backend/pkg/db"
)

type DBChatMessageRoomResolver struct {
	db *dbpkg.DB
}

func NewDBChatMessageRoomResolver(db *dbpkg.DB) *DBChatMessageRoomResolver {
	return &DBChatMessageRoomResolver{db: db}
}

func (r *DBChatMessageRoomResolver) ResolveRoomIDByMessageID(
	ctx context.Context,
	messageID uuid.UUID,
) (uuid.UUID, error) {
	var roomID uuid.UUID
	err := r.db.Pool().QueryRow(
		ctx,
		`SELECT room_id FROM chat_messages WHERE id = $1`,
		messageID,
	).Scan(&roomID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("query room_id for message failed: %w", err)
	}
	return roomID, nil
}


