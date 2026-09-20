package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/event"
	"github.com/labuda/backend/internal/presence"
	outboxRepoPkg "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	pkgredis "github.com/labuda/backend/pkg/redis"
	"github.com/labuda/backend/pkg/testdb"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func newDurableRedisRepoForWorker(t *testing.T) (*presence.RedisRepository, *pkgredis.Client) {
	t.Helper()
	client := &pkgredis.Client{Client: goredis.NewClient(&goredis.Options{
		Addr:         "localhost:6379",
		DB:           15,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, client.Ping(ctx).Err())
	t.Cleanup(func() { _ = client.Client.Close() })
	return presence.NewRedisRepository(client, zaptest.NewLogger(t)), client
}

func seedDurableUserWorker(t *testing.T, tdb *testdb.TestDB) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var userID uuid.UUID
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		userID = uuid.New()
		_, err := tx.Exec(ctx, `INSERT INTO users (id, firebase_uid, email, email_verified_at, phone_verified, account_status, created_at, updated_at) VALUES ($1,$2,$3,NOW(),true,'active',NOW(),NOW())`, userID, userID.String(), userID.String()+"@test.invalid")
		return err
	})
	require.NoError(t, err)
	return userID
}

func readDurableLastSeenWorker(t *testing.T, tdb *testdb.TestDB, userID uuid.UUID) *time.Time {
	t.Helper()
	var ts *time.Time
	err := tdb.Pool().QueryRow(context.Background(), `SELECT last_seen_at FROM user_presence WHERE user_id=$1`, userID).Scan(&ts)
	if err != nil {
		return nil
	}
	return ts
}

// A. WS single-session close → durable last_seen
func TestDurable_WS_SingleSession_ProducesLastSeen(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	repo, client := newDurableRedisRepoForWorker(t)
	outboxRepo := outboxRepoPkg.NewOutboxRepository(db.NewFromPool(tdb.Pool()))
	svc := presence.NewService(db.NewFromPool(tdb.Pool()), repo, presence.NewDBRepository(db.NewFromPool(tdb.Pool())), zaptest.NewLogger(t), outboxRepo)
	ctx := context.Background()
	require.NoError(t, client.FlushDB(ctx).Err())
	userID := seedDurableUserWorker(t, tdb)
	c1 := uuid.NewString()
	r1, err := svc.ResumeLease(ctx, userID, c1)
	require.NoError(t, err)
	require.True(t, r1.IsOnline)
	res, err := svc.LeaveLease(ctx, userID, c1)
	require.NoError(t, err)
	require.True(t, res.Transitioned)
	require.NotNil(t, res.LastSeenAt)
	require.NoError(t, svc.HandleOfflineTransition(ctx, res))
	var count int
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM outbox WHERE idempotency_key=$1`, "presence.last_seen_record."+userID.String()+".2").Scan(&count))
	require.Equal(t, 1, count)
	handler := NewPresenceLastSeenHandler(svc, zaptest.NewLogger(t))
	var payloadBytes []byte
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT payload FROM outbox WHERE idempotency_key=$1`, "presence.last_seen_record."+userID.String()+".2").Scan(&payloadBytes))
	require.NoError(t, handler.Handle(ctx, event.OutboxEvent{Payload: payloadBytes}))
	seen := readDurableLastSeenWorker(t, tdb, userID)
	require.NotNil(t, seen)
}

// B. Multi-session: c1 close no event, c2 close one event
func TestDurable_MultiSession_OnlyFinalProduces(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	repo, client := newDurableRedisRepoForWorker(t)
	outboxRepo := outboxRepoPkg.NewOutboxRepository(db.NewFromPool(tdb.Pool()))
	svc := presence.NewService(db.NewFromPool(tdb.Pool()), repo, presence.NewDBRepository(db.NewFromPool(tdb.Pool())), zaptest.NewLogger(t), outboxRepo)
	ctx := context.Background()
	require.NoError(t, client.FlushDB(ctx).Err())
	userID := seedDurableUserWorker(t, tdb)
	c1 := uuid.NewString()
	c2 := uuid.NewString()
	_, err := svc.ResumeLease(ctx, userID, c1)
	require.NoError(t, err)
	_, err = svc.ResumeLease(ctx, userID, c2)
	require.NoError(t, err)
	r1, _ := svc.LeaveLease(ctx, userID, c1)
	require.True(t, r1.IsOnline)
	require.False(t, r1.Transitioned)
	require.NoError(t, svc.HandleOfflineTransition(ctx, r1))
	var c1Count int
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM outbox WHERE idempotency_key LIKE $1`, "presence.last_seen_record."+userID.String()+".%").Scan(&c1Count))
	require.Equal(t, 0, c1Count)
	r2, _ := svc.LeaveLease(ctx, userID, c2)
	require.False(t, r2.IsOnline)
	require.True(t, r2.Transitioned)
	require.NoError(t, svc.HandleOfflineTransition(ctx, r2))
	var c2Count int
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM outbox WHERE idempotency_key LIKE $1`, "presence.last_seen_record."+userID.String()+".%").Scan(&c2Count))
	require.Equal(t, 1, c2Count)
}

// C. Sweeper expiry produces durable
func TestDurable_SweeperExpiry_ProducesLastSeen(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	repo, client := newDurableRedisRepoForWorker(t)
	outboxRepo := outboxRepoPkg.NewOutboxRepository(db.NewFromPool(tdb.Pool()))
	svc := presence.NewService(db.NewFromPool(tdb.Pool()), repo, presence.NewDBRepository(db.NewFromPool(tdb.Pool())), zaptest.NewLogger(t), outboxRepo)
	sweeper := presence.NewSweeperWithConfig(svc, zaptest.NewLogger(t), 10*time.Millisecond, 200)
	ctx := context.Background()
	require.NoError(t, client.FlushDB(ctx).Err())
	userID := seedDurableUserWorker(t, tdb)
	c1 := uuid.NewString()
	past := time.Now().UTC().Add(-100 * time.Second)
	_, err := repo.ResumeLease(ctx, userID, c1, past)
	require.NoError(t, err)
	sweeper.SweepOnce(ctx)
	var count int
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM outbox WHERE idempotency_key LIKE $1`, "presence.last_seen_record."+userID.String()+".%").Scan(&count))
	require.Equal(t, 1, count)
	var payloadBytes []byte
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT payload FROM outbox WHERE idempotency_key LIKE $1 LIMIT 1`, "presence.last_seen_record."+userID.String()+".%").Scan(&payloadBytes))
	handler := NewPresenceLastSeenHandler(svc, zaptest.NewLogger(t))
	require.NoError(t, handler.Handle(ctx, event.OutboxEvent{Payload: payloadBytes}))
	seen := readDurableLastSeenWorker(t, tdb, userID)
	require.NotNil(t, seen)
}

// D. Renew does not produce
func TestDurable_Renew_DoesNotProduce(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	repo, client := newDurableRedisRepoForWorker(t)
	outboxRepo := outboxRepoPkg.NewOutboxRepository(db.NewFromPool(tdb.Pool()))
	svc := presence.NewService(db.NewFromPool(tdb.Pool()), repo, presence.NewDBRepository(db.NewFromPool(tdb.Pool())), zaptest.NewLogger(t), outboxRepo)
	ctx := context.Background()
	require.NoError(t, client.FlushDB(ctx).Err())
	userID := seedDurableUserWorker(t, tdb)
	c1 := uuid.NewString()
	_, err := svc.ResumeLease(ctx, userID, c1)
	require.NoError(t, err)
	r2, err := svc.ResumeLease(ctx, userID, c1)
	require.NoError(t, err)
	require.False(t, r2.Transitioned)
	require.NoError(t, svc.HandleOfflineTransition(ctx, r2))
	var count int
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM outbox WHERE idempotency_key LIKE $1`, "presence.last_seen_record."+userID.String()+".%").Scan(&count))
	require.Equal(t, 0, count)
}

// E. Replay safe (same version twice)
func TestDurable_Replay_Safe(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	repo, client := newDurableRedisRepoForWorker(t)
	outboxRepo := outboxRepoPkg.NewOutboxRepository(db.NewFromPool(tdb.Pool()))
	svc := presence.NewService(db.NewFromPool(tdb.Pool()), repo, presence.NewDBRepository(db.NewFromPool(tdb.Pool())), zaptest.NewLogger(t), outboxRepo)
	ctx := context.Background()
	require.NoError(t, client.FlushDB(ctx).Err())
	userID := seedDurableUserWorker(t, tdb)
	c1 := uuid.NewString()
	_, _ = svc.ResumeLease(ctx, userID, c1)
	res, _ := svc.LeaveLease(ctx, userID, c1)
	require.NoError(t, svc.HandleOfflineTransition(ctx, res))
	require.NoError(t, svc.EnqueueLastSeen(ctx, userID, *res.LastSeenAt, res.Version))
	var cnt int
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM outbox WHERE idempotency_key=$1`, "presence.last_seen_record."+userID.String()+"."+fmt.Sprintf("%d", res.Version)).Scan(&cnt))
	require.Equal(t, 1, cnt)
	var payloadBytes []byte
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT payload FROM outbox WHERE idempotency_key=$1`, "presence.last_seen_record."+userID.String()+"."+fmt.Sprintf("%d", res.Version)).Scan(&payloadBytes))
	handler := NewPresenceLastSeenHandler(svc, zaptest.NewLogger(t))
	require.NoError(t, handler.Handle(ctx, event.OutboxEvent{Payload: payloadBytes}))
	firstSeen := readDurableLastSeenWorker(t, tdb, userID)
	require.NoError(t, handler.Handle(ctx, event.OutboxEvent{Payload: payloadBytes}))
	secondSeen := readDurableLastSeenWorker(t, tdb, userID)
	require.True(t, firstSeen.Equal(*secondSeen))
}

// F. Out-of-order older version cannot overwrite newer
func TestDurable_OutOfOrder_OlderIgnored(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	svc := presence.NewService(db.NewFromPool(tdb.Pool()), nil, presence.NewDBRepository(db.NewFromPool(tdb.Pool())), zaptest.NewLogger(t), nil)
	ctx := context.Background()
	userID := seedDurableUserWorker(t, tdb)
	newer := time.Date(2026, time.July, 29, 20, 0, 0, 0, time.UTC)
	older := newer.Add(-1 * time.Hour)
	require.NoError(t, svc.PersistLastSeen(ctx, userID, newer, 2))
	require.NoError(t, svc.PersistLastSeen(ctx, userID, older, 1))
	seen := readDurableLastSeenWorker(t, tdb, userID)
	require.NotNil(t, seen)
	require.True(t, newer.Equal(*seen))
}

// G. Failure path: handler retry safe
func TestDurable_HandlerFailure_IsRetryable(t *testing.T) {
	handler := NewPresenceLastSeenHandler(nil, zaptest.NewLogger(t))
	payloadBytes, _ := json.Marshal(presence.LastSeenRecordPayload{UserID: uuid.New(), LastSeenAt: time.Now().UTC().Format(time.RFC3339), Version: 1})
	err := handler.Handle(context.Background(), event.OutboxEvent{Payload: payloadBytes})
	require.Error(t, err)
	require.Contains(t, err.Error(), "presence service unavailable")
}
