package presence

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	pkgredis "github.com/labuda/backend/pkg/redis"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func newSweeperRedisRepo(t *testing.T, db int) (*RedisRepository, *pkgredis.Client) {
	t.Helper()
	client := &pkgredis.Client{Client: goredis.NewClient(&goredis.Options{
		Addr:         "localhost:6379",
		DB:           db,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, client.Ping(ctx).Err())
	t.Cleanup(func() { _ = client.Client.Close() })
	return NewRedisRepository(client, zaptest.NewLogger(t)), client
}

func cleanSweeperKeys(ctx context.Context, client *pkgredis.Client, userID uuid.UUID) {
	_ = client.Del(ctx, RedisLeaseKeyPrefix+userID.String(), RedisStateKeyPrefix+userID.String()).Err()
	_ = client.ZRem(ctx, RedisDeadlineKey, userID.String()).Err()
}

// TestSweeper_ExpiredLease_TransitionsToOfflineAndPublishes verifies A: expired lease → OFFLINE
func TestSweeper_ExpiredLease_TransitionsToOfflineAndPublishes(t *testing.T) {
	repo, client := newSweeperRedisRepo(t, 15)
	svc := NewService(nil, repo, nil, zaptest.NewLogger(t))
	sweeper := NewSweeperWithConfig(svc, zaptest.NewLogger(t), 10*time.Millisecond, 200)
	ctx := context.Background()
	// Isolate DB from other tests
	require.NoError(t, client.FlushDB(ctx).Err())
	userID := uuid.New()
	c1 := uuid.NewString()
	cleanSweeperKeys(ctx, client, userID)
	defer cleanSweeperKeys(ctx, client, userID)

	// Create lease that expired 10s ago
	past := time.Now().UTC().Add(-100 * time.Second)
	_, err := repo.ResumeLease(ctx, userID, c1, past)
	require.NoError(t, err)

	// Verify online before sweep
	states, err := repo.GetStatesBatch(ctx, []uuid.UUID{userID})
	require.NoError(t, err)
	require.True(t, states[userID].IsOnline)
	require.Equal(t, int64(1), states[userID].Version)

	// Subscribe to events before sweep
	subCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	pubsub, err := repo.Subscribe(subCtx)
	require.NoError(t, err)
	defer pubsub.Close()
	ch := pubsub.Channel()

	// Sweep via sweeper (ClaimDueUsers→SweepUser→PublishChanged)
	sweeper.SweepOnce(ctx)

	// Should have transitioned to offline
	states2, err := repo.GetStatesBatch(ctx, []uuid.UUID{userID})
	require.NoError(t, err)
	require.False(t, states2[userID].IsOnline)
	require.Equal(t, int64(2), states2[userID].Version)

	// Verify presence.changed published with correct state (filter by userID in case other due users in shared DB)
	timeout := time.After(2 * time.Second)
	for {
		select {
		case msg := <-ch:
			var ev Event
			if err := json.Unmarshal([]byte(msg.Payload), &ev); err != nil {
				continue
			}
			if ev.State.UserID != userID {
				continue
			}
			require.Equal(t, "presence.changed", ev.Type)
			require.False(t, ev.State.IsOnline)
			require.Equal(t, int64(2), ev.State.Version)
			require.NotNil(t, ev.State.LastSeenAt)
			return
		case <-timeout:
			t.Fatal("expected presence.changed for user after sweep")
		}
	}
}

// TestSweeper_MultipleSessions_OneExpiresRemainsOnline verifies B: c1 expires but c2 active → no offline
func TestSweeper_MultipleSessions_OneExpiresRemainsOnline(t *testing.T) {
	repo, client := newSweeperRedisRepo(t, 15)
	svc := NewService(nil, repo, nil, zaptest.NewLogger(t))
	sweeper := NewSweeperWithConfig(svc, zaptest.NewLogger(t), 10*time.Millisecond, 200)
	ctx := context.Background()
	require.NoError(t, client.FlushDB(ctx).Err())
	userID := uuid.New()
	c1 := uuid.NewString()
	c2 := uuid.NewString()
	cleanSweeperKeys(ctx, client, userID)
	defer cleanSweeperKeys(ctx, client, userID)

	past := time.Now().UTC().Add(-100 * time.Second)
	_, err := repo.ResumeLease(ctx, userID, c1, past)
	require.NoError(t, err)
	// c2 lease is fresh (now), so not expired
	_, err = repo.ResumeLease(ctx, userID, c2, time.Now().UTC())
	require.NoError(t, err)

	states, _ := repo.GetStatesBatch(ctx, []uuid.UUID{userID})
	require.True(t, states[userID].IsOnline)
	require.Equal(t, int64(1), states[userID].Version)

	// Sweep should NOT transition to offline because c2 still valid after ZREMRANGEBYSCORE removes only expired c1
	sweeper.SweepOnce(ctx)

	states2, _ := repo.GetStatesBatch(ctx, []uuid.UUID{userID})
	require.True(t, states2[userID].IsOnline, "user remains online while c2 active")
	require.Equal(t, int64(1), states2[userID].Version, "no version bump when still online")
	require.Equal(t, int64(1), states2[userID].Version)

	// No offline transition, so checking that we didn't publish offline event is tricky without capturing.
	// Instead verify that a second sweep after c2 also expires does transition
	past2 := time.Now().UTC().Add(-100 * time.Second)
	// Force c2 to be expired by re-adding with past (overwrites ZADD score)
	_, err = repo.ResumeLease(ctx, userID, c2, past2)
	require.NoError(t, err)
	// Need to ensure deadlines now past again: after above, deadlines = past2+90s = now-10s, so claimable
	sweeper.SweepOnce(ctx)
	states3, _ := repo.GetStatesBatch(ctx, []uuid.UUID{userID})
	require.False(t, states3[userID].IsOnline)
	require.Equal(t, int64(2), states3[userID].Version)
}

// TestSweeper_NoTransition_NoPublish verifies D: no effective transition → no event
func TestSweeper_NoTransition_NoPublish(t *testing.T) {
	repo, client := newSweeperRedisRepo(t, 15)
	svc := NewService(nil, repo, nil, zaptest.NewLogger(t))
	ctx := context.Background()
	userID := uuid.New()
	cleanSweeperKeys(ctx, client, userID)
	defer cleanSweeperKeys(ctx, client, userID)

	// No lease at all, ensure no due users
	due, err := repo.ClaimDueUsers(ctx, 10, time.Now().UTC())
	require.NoError(t, err)
	require.Empty(t, due)

	subCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	pubsub, err := repo.Subscribe(subCtx)
	require.NoError(t, err)
	defer pubsub.Close()
	ch := pubsub.Channel()

	sweeper := NewSweeperWithConfig(svc, zaptest.NewLogger(t), 10*time.Millisecond, 200)
	sweeper.SweepOnce(ctx)

	select {
	case msg := <-ch:
		t.Fatalf("unexpected publish when no transition: %s", msg.Payload)
	case <-time.After(200 * time.Millisecond):
		// expected no publish
	}
}

// TestSweeper_VersionSemantics verifies version increments only on transition
func TestSweeper_VersionSemantics(t *testing.T) {
	repo, client := newSweeperRedisRepo(t, 15)
	svc := NewService(nil, repo, nil, zaptest.NewLogger(t))
	ctx := context.Background()
	userID := uuid.New()
	c1 := uuid.NewString()
	cleanSweeperKeys(ctx, client, userID)
	defer cleanSweeperKeys(ctx, client, userID)

	// offline -> online
	past := time.Now().UTC().Add(-100 * time.Second)
	r1, _ := repo.ResumeLease(ctx, userID, c1, past)
	require.Equal(t, int64(1), r1.Version)
	// renew same c1 fresher should not bump version (but our past vs now test already)
	fresh := time.Now().UTC()
	r2, _ := repo.ResumeLease(ctx, userID, c1, fresh)
	require.Equal(t, int64(1), r2.Version, "renew unchanged")

	// online -> offline via sweep
	sweeper := NewSweeperWithConfig(svc, zaptest.NewLogger(t), 10*time.Millisecond, 200)
	// Make c1 expired again
	_, _ = repo.ResumeLease(ctx, userID, c1, past)
	sweeper.SweepOnce(ctx)
	states, _ := repo.GetStatesBatch(ctx, []uuid.UUID{userID})
	require.Equal(t, int64(2), states[userID].Version, "offline transition +1")
}

// TestSweeper_StartStop_Shutdown verifies lifecycle
func TestSweeper_StartStop_Shutdown(t *testing.T) {
	repo, _ := newSweeperRedisRepo(t, 15)
	svc := NewService(nil, repo, nil, zaptest.NewLogger(t))
	sweeper := NewSweeperWithConfig(svc, zaptest.NewLogger(t), 50*time.Millisecond, 10)
	require.False(t, sweeper.IsRunning())
	sweeper.Start()
	require.True(t, sweeper.IsRunning())
	// double start is no-op
	sweeper.Start()
	require.True(t, sweeper.IsRunning())
	sweeper.Stop()
	require.False(t, sweeper.IsRunning())
	// double stop no panic
	sweeper.Stop()
	require.False(t, sweeper.IsRunning())
}
