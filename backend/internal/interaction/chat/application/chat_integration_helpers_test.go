//go:build integration

package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/require"
)

// noopChatOutbox is a no-op OutboxInserter for tests that don't need outbox queries.
type noopChatOutbox struct{}

func (n noopChatOutbox) InsertTx(context.Context, db.Tx, string, any, string) error {
	return nil
}

func insertChatAuthorityUser(t *testing.T, ctx context.Context, appDB *db.DB, username string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := appDB.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at)
		VALUES ($1, $2, $3, 'active', NOW(), NOW())
		ON CONFLICT (id) DO NOTHING
	`, id, id.String(), id.String()+"@test.local")
	require.NoError(t, err)
	_, err = appDB.Pool().Exec(ctx, `
		INSERT INTO user_profiles (user_id, username, created_at, updated_at)
		VALUES ($1, $2, NOW(), NOW())
		ON CONFLICT (user_id) DO NOTHING
	`, id, id.String())
	require.NoError(t, err)
	return id
}

func insertChatAuthorityRoom(t *testing.T, ctx context.Context, appDB *db.DB, participantA, participantB uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	// Sort participants (same as NewChatRoom)
	a, b := participantA, participantB
	if a.String() > b.String() {
		a, b = b, a
	}
	_, err := appDB.Pool().Exec(ctx, `
		INSERT INTO chat_rooms (id, room_type, participant_a, participant_b, created_at, updated_at, last_message_at)
		VALUES ($1, 'direct', $2, $3, NOW(), NOW(), NOW())
		ON CONFLICT (id) DO NOTHING
	`, id, a, b)
	require.NoError(t, err)
	return id
}
