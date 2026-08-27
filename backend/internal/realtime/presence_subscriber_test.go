package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/presence"
	"github.com/labuda/backend/pkg/db"
	pkgredis "github.com/labuda/backend/pkg/redis"
	"github.com/labuda/backend/pkg/testdb"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestMarshalPresenceChanged_UsesCanonicalEnvelope(t *testing.T) {
	seenAt := time.Date(2026, time.August, 11, 10, 0, 0, 0, time.UTC)
	payload := marshalPresenceChanged(presence.State{
		UserID:     uuid.New(),
		IsOnline:   true,
		LastSeenAt: &seenAt,
		Version:    42,
	})

	var envelope map[string]any
	require.NoError(t, json.Unmarshal(payload, &envelope))
	require.Equal(t, "presence.changed", envelope["type"])
	state, ok := envelope["state"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, state["is_online"])
	require.Equal(t, float64(42), state["version"])
}

func newRealtimePresenceRepo(t *testing.T) (*testdb.TestDB, *presence.RedisRepository) {
	t.Helper()

	tdb, _ := testdb.SetupDB(t)
	t.Cleanup(func() { tdb.Pool().Close() })

	client := &pkgredis.Client{Client: goredis.NewClient(&goredis.Options{
		Addr:         "localhost:6379",
		DB:           14,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, client.Ping(ctx).Err())
	t.Cleanup(func() { client.Client.Close() })

	return tdb, presence.NewRedisRepository(client, zaptest.NewLogger(t))
}

func seedRealtimePresenceUser(t *testing.T, tdb *testdb.TestDB, label string) uuid.UUID {
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
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO user_profiles (user_id, username, privacy, created_at, updated_at)
			VALUES ($1, $2, '{}'::jsonb, NOW(), NOW())
		`, userID, "presence-"+userID.String()[:8])
		return err
	})
	require.NoError(t, err)
	return userID
}

func waitForPresenceSubscriber(t *testing.T, s *PresenceSubscriber) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		running := s.running && s.pubsub != nil
		s.mu.Unlock()
		if running {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("subscriber did not connect in time")
}

func TestPresenceSubscriber_ShouldDeliver_DedupesVersions(t *testing.T) {
	sub := NewPresenceSubscriber(nil, nil, zaptest.NewLogger(t))
	userID := uuid.New()

	require.True(t, sub.shouldDeliver(userID, 1))
	require.False(t, sub.shouldDeliver(userID, 1))
	require.False(t, sub.shouldDeliver(userID, 0))
	require.True(t, sub.shouldDeliver(userID, 2))
}

func TestNextPresenceBackoff_CapsAtThirtySeconds(t *testing.T) {
	require.Equal(t, presence.PresenceRetryBackoff, nextPresenceBackoff(0))
	require.Equal(t, 10*time.Second, nextPresenceBackoff(5*time.Second))
	require.Equal(t, 30*time.Second, nextPresenceBackoff(30*time.Second))
	require.Equal(t, 30*time.Second, nextPresenceBackoff(45*time.Second))
}

func TestPresenceSubscriber_DistributesPresenceChangedAcrossInstances(t *testing.T) {
	tdb, redisRepo := newRealtimePresenceRepo(t)
	ctx := context.Background()
	repo := presence.NewDBRepository(nil)
	svc := presence.NewService(db.NewFromPool(tdb.Pool()), redisRepo, repo, zaptest.NewLogger(t))

	targetID := seedRealtimePresenceUser(t, tdb, "target")
	viewerA := seedRealtimePresenceUser(t, tdb, "viewer-a")

	hubA := NewHub(zaptest.NewLogger(t))
	hubB := NewHub(zaptest.NewLogger(t))
	subA := NewPresenceSubscriber(svc, hubA, zaptest.NewLogger(t))
	subB := NewPresenceSubscriber(svc, hubB, zaptest.NewLogger(t))
	subA.Start()
	subB.Start()
	defer subA.Stop()
	defer subB.Stop()
	waitForPresenceSubscriber(t, subA)
	waitForPresenceSubscriber(t, subB)

	connA := &Connection{
		ID:     uuid.NewString(),
		UserID: viewerA,
		Send:   make(chan []byte, 4),
		Rooms:  make(map[uuid.UUID]struct{}),
		hub:    hubA,
	}
	hubA.Register(connA)

	connB := &Connection{
		ID:     uuid.NewString(),
		UserID: targetID,
		Send:   make(chan []byte, 4),
		Rooms:  make(map[uuid.UUID]struct{}),
		hub:    hubB,
	}
	hubB.Register(connB)

	lease, err := redisRepo.ResumeLease(ctx, targetID, "presence-conn", time.Now().UTC())
	require.NoError(t, err)
	require.True(t, lease.IsOnline)

	require.NoError(t, svc.PublishChanged(ctx, presence.State{
		UserID:   targetID,
		IsOnline: true,
		Version:  1,
	}))

	select {
	case msg := <-connB.Send:
		var envelope map[string]any
		require.NoError(t, json.Unmarshal(msg, &envelope))
		require.Equal(t, "presence.changed", envelope["type"])
	case <-time.After(5 * time.Second):
		t.Fatal("watcher on instance B did not receive presence.changed")
	}

	select {
	case msg := <-connA.Send:
		t.Fatalf("non-watcher on instance A unexpectedly received message: %s", string(msg))
	case <-time.After(300 * time.Millisecond):
	}

	require.NoError(t, svc.PublishChanged(ctx, presence.State{
		UserID:   targetID,
		IsOnline: true,
		Version:  1,
	}))
	select {
	case msg := <-connB.Send:
		t.Fatalf("duplicate version should have been ignored, got: %s", string(msg))
	case <-time.After(300 * time.Millisecond):
	}

	require.NoError(t, svc.PublishChanged(ctx, presence.State{
		UserID:   targetID,
		IsOnline: false,
		Version:  2,
	}))

	select {
	case msg := <-connB.Send:
		var envelope map[string]any
		require.NoError(t, json.Unmarshal(msg, &envelope))
		require.Equal(t, "presence.changed", envelope["type"])
	case <-time.After(5 * time.Second):
		t.Fatal("watcher on instance B did not receive newer presence.changed")
	}
}

func TestPresenceSubscriber_StopCancelsSubscription(t *testing.T) {
	tdb, redisRepo := newRealtimePresenceRepo(t)
	svc := presence.NewService(db.NewFromPool(tdb.Pool()), redisRepo, presence.NewDBRepository(nil), zaptest.NewLogger(t))
	hub := NewHub(zaptest.NewLogger(t))
	sub := NewPresenceSubscriber(svc, hub, zaptest.NewLogger(t))

	sub.Start()
	waitForPresenceSubscriber(t, sub)
	require.True(t, sub.IsRunning())
	sub.Stop()
	require.False(t, sub.IsRunning())
}
