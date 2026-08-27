package presence

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/events"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type failingLastSeenWriter struct {
	err error
}

func (f *failingLastSeenWriter) UpsertLastSeen(context.Context, db.Tx, uuid.UUID, time.Time) error {
	return f.err
}

type recordedLastSeenOutboxEvent struct {
	eventType      string
	idempotencyKey string
	payload        LastSeenRecordPayload
}

type recordingLastSeenOutbox struct {
	mu     sync.Mutex
	events []recordedLastSeenOutboxEvent
	err    error
}

func (r *recordingLastSeenOutbox) InsertTx(_ context.Context, _ db.Tx, eventType string, payload any, idempotencyKey string) error {
	typed, ok := payload.(LastSeenRecordPayload)
	if !ok {
		return fmt.Errorf("unexpected payload type %T", payload)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, recordedLastSeenOutboxEvent{
		eventType:      eventType,
		idempotencyKey: idempotencyKey,
		payload:        typed,
	})
	return r.err
}

func (r *recordingLastSeenOutbox) snapshot() []recordedLastSeenOutboxEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]recordedLastSeenOutboxEvent, len(r.events))
	copy(out, r.events)
	return out
}

func seedPresenceUserCommitted(t *testing.T, tdb *testdb.TestDB, label string) uuid.UUID {
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
		`, userID, userID.String(), fmt.Sprintf("%s-%s@test.invalid", label, userID.String()))
		return err
	})
	require.NoError(t, err)
	return userID
}

func TestService_PersistLastSeen_DirectWriteSuccess(t *testing.T) {
	tdb, _ := testdb.SetupDB(t)
	defer tdb.Pool().Close()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	outbox := &recordingLastSeenOutbox{}
	repo := NewDBRepository(nil)
	svc := NewService(appDB, nil, repo, zaptest.NewLogger(t), outbox)
	userID := seedPresenceUserCommitted(t, tdb, "direct-write")
	occurredAt := time.Date(2026, time.July, 29, 15, 0, 0, 0, time.UTC)

	require.NoError(t, svc.PersistLastSeen(ctx, userID, occurredAt, 7))
	require.Empty(t, outbox.snapshot())

	seen := readPresenceLastSeen(t, tdb, userID)
	require.NotNil(t, seen)
	require.True(t, occurredAt.Equal(*seen))
}

func TestService_PersistLastSeen_FallsBackToOutboxOnWriteFailure(t *testing.T) {
	tdb, _ := testdb.SetupDB(t)
	defer tdb.Pool().Close()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	outbox := &recordingLastSeenOutbox{}
	failWriter := &failingLastSeenWriter{err: errors.New("postgres unavailable")}
	svc := NewService(appDB, nil, failWriter, zaptest.NewLogger(t), outbox)
	userID := seedPresenceUserCommitted(t, tdb, "fallback")
	occurredAt := time.Date(2026, time.July, 29, 16, 0, 0, 0, time.UTC)

	require.NoError(t, svc.PersistLastSeen(ctx, userID, occurredAt, 11))

	recorded := outbox.snapshot()
	require.Len(t, recorded, 1)
	require.Equal(t, events.EventUserPresenceLastSeenRecord, recorded[0].eventType)
	require.Equal(t, fmt.Sprintf("%s.%d", userID.String(), 11), recorded[0].idempotencyKey)
	require.Equal(t, userID, recorded[0].payload.UserID)
	require.Equal(t, occurredAt.UTC().Format(time.RFC3339), recorded[0].payload.LastSeenAt)
	require.Equal(t, int64(11), recorded[0].payload.Version)

	var count int
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM user_presence WHERE user_id = $1`, userID).Scan(&count))
	require.Equal(t, 0, count)
}

func TestService_PersistLastSeen_OutboxFailureReturnsError(t *testing.T) {
	tdb, _ := testdb.SetupDB(t)
	defer tdb.Pool().Close()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	outbox := &recordingLastSeenOutbox{err: errors.New("outbox failure")}
	failWriter := &failingLastSeenWriter{err: errors.New("postgres unavailable")}
	svc := NewService(appDB, nil, failWriter, zaptest.NewLogger(t), outbox)
	userID := seedPresenceUserCommitted(t, tdb, "outbox-failure")
	occurredAt := time.Date(2026, time.July, 29, 17, 0, 0, 0, time.UTC)

	err := svc.PersistLastSeen(ctx, userID, occurredAt, 13)
	require.Error(t, err)
	require.Contains(t, err.Error(), "enqueue retry failed")

	recorded := outbox.snapshot()
	require.Len(t, recorded, 1)
	require.Equal(t, events.EventUserPresenceLastSeenRecord, recorded[0].eventType)
}
