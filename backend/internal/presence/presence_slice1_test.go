package presence

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	pkgredis "github.com/labuda/backend/pkg/redis"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func newSlice1RedisRepo(t *testing.T) (*RedisRepository, *pkgredis.Client) {
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
	repo := NewRedisRepository(client, zaptest.NewLogger(t))
	return repo, client
}

// cleanPresenceKeys removes all presence keys for a user to ensure isolation.
func cleanPresenceKeys(ctx context.Context, client *pkgredis.Client, userID uuid.UUID) {
	_ = client.Del(ctx, RedisLeaseKeyPrefix+userID.String(), RedisStateKeyPrefix+userID.String()).Err()
	_ = client.ZRem(ctx, RedisDeadlineKey, userID.String()).Err()
}

func TestSlice1_SingleSession_Acquire(t *testing.T) {
	repo, client := newSlice1RedisRepo(t)
	ctx := context.Background()
	userID := uuid.New()
	connID := uuid.NewString()
	cleanPresenceKeys(ctx, client, userID)
	defer cleanPresenceKeys(ctx, client, userID)

	res, err := repo.ResumeLease(ctx, userID, connID, time.Now().UTC())
	require.NoError(t, err)
	require.NotNil(t, res)
	require.True(t, res.IsOnline, "first ResumeLease should make user online")
	require.True(t, res.Transitioned, "first acquire should transition offline->online")
	require.Equal(t, int64(1), res.ActiveLeaseCount)
	require.Equal(t, int64(1), res.Version, "first online transition version should be 1")
	require.NotNil(t, res.NextDeadline, "online lease must have next deadline")
	require.Nil(t, res.LastSeenAt, "online transition should have no LastSeen")

	// Verify via GetStatesBatch
	states, err := repo.GetStatesBatch(ctx, []uuid.UUID{userID})
	require.NoError(t, err)
	st := states[userID]
	require.True(t, st.IsOnline)
	require.Equal(t, int64(1), st.Version)
}

func TestSlice1_SingleSession_Release(t *testing.T) {
	repo, client := newSlice1RedisRepo(t)
	ctx := context.Background()
	userID := uuid.New()
	connID := uuid.NewString()
	cleanPresenceKeys(ctx, client, userID)
	defer cleanPresenceKeys(ctx, client, userID)

	// Acquire
	acq, err := repo.ResumeLease(ctx, userID, connID, time.Now().UTC())
	require.NoError(t, err)
	require.True(t, acq.IsOnline)
	require.Equal(t, int64(1), acq.Version)

	// Release
	rel, err := repo.LeaveLease(ctx, userID, connID, time.Now().UTC())
	require.NoError(t, err)
	require.NotNil(t, rel)
	require.False(t, rel.IsOnline, "after LeaveLease user must be offline")
	require.True(t, rel.Transitioned, "offline transition must be true when last lease removed")
	require.Equal(t, int64(0), rel.ActiveLeaseCount)
	require.Equal(t, int64(2), rel.Version, "offline transition should increment version 1->2")
	require.NotNil(t, rel.LastSeenAt, "offline transition must expose LastSeenAt")
	require.NotNil(t, rel.State.LastSeenAt)
	require.Nil(t, rel.NextDeadline, "offline has no deadline")

	states, err := repo.GetStatesBatch(ctx, []uuid.UUID{userID})
	require.NoError(t, err)
	require.False(t, states[userID].IsOnline)
	require.Equal(t, int64(2), states[userID].Version)
}

func TestSlice1_MultiSession_OneCloseRemainsOnline(t *testing.T) {
	repo, client := newSlice1RedisRepo(t)
	ctx := context.Background()
	userID := uuid.New()
	c1 := uuid.NewString()
	c2 := uuid.NewString()
	cleanPresenceKeys(ctx, client, userID)
	defer cleanPresenceKeys(ctx, client, userID)

	// c1 acquire
	r1, err := repo.ResumeLease(ctx, userID, c1, time.Now().UTC())
	require.NoError(t, err)
	require.True(t, r1.IsOnline)
	require.True(t, r1.Transitioned)
	require.Equal(t, int64(1), r1.Version)
	require.Equal(t, int64(1), r1.ActiveLeaseCount)

	// c2 acquire - should remain online, no new transition
	r2, err := repo.ResumeLease(ctx, userID, c2, time.Now().UTC())
	require.NoError(t, err)
	require.True(t, r2.IsOnline)
	require.False(t, r2.Transitioned, "second lease should not cause transition when already online")
	require.Equal(t, int64(1), r2.Version, "version must not bump on second session while already online")
	require.Equal(t, int64(2), r2.ActiveLeaseCount)

	// Leave c1 - should remain online
	rel1, err := repo.LeaveLease(ctx, userID, c1, time.Now().UTC())
	require.NoError(t, err)
	require.True(t, rel1.IsOnline, "user remains online while c2 still active")
	require.False(t, rel1.Transitioned, "removing one of two leases must not transition")
	require.Equal(t, int64(1), rel1.ActiveLeaseCount)
	require.Equal(t, int64(1), rel1.Version)
	require.Nil(t, rel1.LastSeenAt)

	// Leave c2 - should go offline
	rel2, err := repo.LeaveLease(ctx, userID, c2, time.Now().UTC())
	require.NoError(t, err)
	require.False(t, rel2.IsOnline)
	require.True(t, rel2.Transitioned)
	require.Equal(t, int64(0), rel2.ActiveLeaseCount)
	require.Equal(t, int64(2), rel2.Version)
	require.NotNil(t, rel2.LastSeenAt)
}

func TestSlice1_Renew_SameConnectionExtendsDeadlineNoVersionBump(t *testing.T) {
	repo, client := newSlice1RedisRepo(t)
	ctx := context.Background()
	userID := uuid.New()
	connID := uuid.NewString()
	cleanPresenceKeys(ctx, client, userID)
	defer cleanPresenceKeys(ctx, client, userID)

	now1 := time.Now().UTC()
	r1, err := repo.ResumeLease(ctx, userID, connID, now1)
	require.NoError(t, err)
	require.True(t, r1.IsOnline)
	require.Equal(t, int64(1), r1.Version)
	require.True(t, r1.Transitioned)
	require.NotNil(t, r1.NextDeadline)
	d1 := *r1.NextDeadline

	// Small delay to ensure now2 > now1 and expiryMs grows
	time.Sleep(15 * time.Millisecond)
	now2 := time.Now().UTC()
	r2, err := repo.ResumeLease(ctx, userID, connID, now2)
	require.NoError(t, err)
	require.True(t, r2.IsOnline)
	require.False(t, r2.Transitioned, "renew with same connectionID must not transition")
	require.Equal(t, int64(1), r2.Version, "renew must not bump version")
	require.Equal(t, int64(1), r2.ActiveLeaseCount)
	require.NotNil(t, r2.NextDeadline)
	d2 := *r2.NextDeadline
	require.True(t, d2.After(d1), "renew must extend deadline: d2=%v d1=%v", d2, d1)
	require.Nil(t, r2.LastSeenAt)
}

func TestSlice1_Service_WrapsRedis_MultiSession(t *testing.T) {
	// Proves the canonical authority path: WS Connection -> presence.Service -> RedisRepository -> Lua
	// No DB needed for lease path; use nil DB to avoid migration overhead.
	repo, client := newSlice1RedisRepo(t)
	svc := NewService(nil, repo, nil, zaptest.NewLogger(t))
	ctx := context.Background()
	userID := uuid.New()
	c1 := uuid.NewString()
	c2 := uuid.NewString()
	cleanPresenceKeys(ctx, client, userID)
	defer cleanPresenceKeys(ctx, client, userID)

	// Via Service
	r1, err := svc.ResumeLease(ctx, userID, c1)
	require.NoError(t, err)
	require.True(t, r1.IsOnline)
	r2, err := svc.ResumeLease(ctx, userID, c2)
	require.NoError(t, err)
	require.True(t, r2.IsOnline)
	require.Equal(t, int64(1), r2.Version)

	rel1, err := svc.LeaveLease(ctx, userID, c1)
	require.NoError(t, err)
	require.True(t, rel1.IsOnline, "service: one close must keep user online")
	require.False(t, rel1.Transitioned)

	rel2, err := svc.LeaveLease(ctx, userID, c2)
	require.NoError(t, err)
	require.False(t, rel2.IsOnline)
	require.True(t, rel2.Transitioned)
	require.Equal(t, int64(2), rel2.Version)
	require.NotNil(t, rel2.LastSeenAt)
}
