package worker

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/event"
	"github.com/labuda/backend/internal/presence"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func seedWorkerPresenceUser(t *testing.T, tdb *testdb.TestDB, label string) uuid.UUID {
	t.Helper()

	ctx := context.Background()
	var userID uuid.UUID
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		userID = uuid.New()
		_, err := tx.Exec(ctx, `
			INSERT INTO users (
				id, firebase_uid, email, email_verified_at, phone_verified,
				account_status, created_at, updated_at
			)
			VALUES ($1, $2, $3, NOW(), true, 'active', NOW(), NOW())
		`, userID, userID.String(), label+"-"+userID.String()+"@test.invalid")
		return err
	})
	require.NoError(t, err)
	return userID
}

func TestPresenceLastSeenHandler_RejectsMalformedPayload(t *testing.T) {
	handler := NewPresenceLastSeenHandler(nil, zaptest.NewLogger(t))
	err := handler.Handle(context.Background(), event.OutboxEvent{Payload: []byte(`{"user_id":"bad"}`)})
	require.Error(t, err)
}

func TestPresenceLastSeenHandler_ReplaysMonotonicUpsert(t *testing.T) {
	tdb, _ := testdb.SetupDB(t)
	defer tdb.Pool().Close()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	svc := presence.NewService(appDB, nil, presence.NewDBRepository(nil), zaptest.NewLogger(t))
	handler := NewPresenceLastSeenHandler(svc, zaptest.NewLogger(t))
	userID := seedWorkerPresenceUser(t, tdb, "handler")
	first := time.Date(2026, time.July, 29, 18, 0, 0, 0, time.UTC)
	second := first.Add(10 * time.Minute)

	firstPayload, err := json.Marshal(presence.LastSeenRecordPayload{
		UserID:     userID,
		LastSeenAt: first.Format(time.RFC3339),
		Version:    1,
	})
	require.NoError(t, err)
	secondPayload, err := json.Marshal(presence.LastSeenRecordPayload{
		UserID:     userID,
		LastSeenAt: second.Format(time.RFC3339),
		Version:    2,
	})
	require.NoError(t, err)

	require.NoError(t, handler.Handle(ctx, event.OutboxEvent{Payload: firstPayload}))
	require.NoError(t, handler.Handle(ctx, event.OutboxEvent{Payload: firstPayload}))
	require.NoError(t, handler.Handle(ctx, event.OutboxEvent{Payload: secondPayload}))

	var seen time.Time
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT last_seen_at
		FROM user_presence
		WHERE user_id = $1
	`, userID).Scan(&seen))
	require.True(t, second.Equal(seen.UTC()))
}
