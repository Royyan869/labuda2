package presence

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/db"
	pkgredis "github.com/labuda/backend/pkg/redis"
	"github.com/labuda/backend/pkg/testdb"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func newPresenceRedisTestRepo(t *testing.T) *RedisRepository {
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
	t.Cleanup(func() { client.Client.Close() })
	return NewRedisRepository(client, zaptest.NewLogger(t))
}

func seedProfileRow(t *testing.T, ctx context.Context, tx db.Tx, userID uuid.UUID, privacy any) {
	t.Helper()

	var privacyJSON any
	if privacy != nil {
		b, err := json.Marshal(privacy)
		require.NoError(t, err)
		privacyJSON = string(b)
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO user_profiles (user_id, username, privacy, created_at, updated_at)
		VALUES ($1, $2, $3::jsonb, NOW(), NOW())
		ON CONFLICT (user_id) DO UPDATE
		SET username = EXCLUDED.username,
			privacy = EXCLUDED.privacy,
			updated_at = NOW()
	`, userID, "presence-"+userID.String()[:8], privacyJSON)
	require.NoError(t, err)
}

func seedPresenceTarget(t *testing.T, tdb *testdb.TestDB, label string, privacy any, status string, deleted bool) uuid.UUID {
	t.Helper()

	ctx := context.Background()
	var userID uuid.UUID
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		userID = uuid.New()
		_, err := tx.Exec(ctx, `
			INSERT INTO users (
				id, firebase_uid, email, email_verified_at, phone_verified,
				account_status, created_at, updated_at, deleted_at
			)
			VALUES ($1, $2, $3, NOW(), true, $4, NOW(), NOW(), CASE WHEN $5 THEN NOW() ELSE NULL END)
		`, userID, userID.String(), fmt.Sprintf("%s-%s@test.invalid", label, userID.String()), status, deleted)
		if err != nil {
			return err
		}
		seedProfileRow(t, ctx, tx, userID, privacy)
		return nil
	})
	require.NoError(t, err)
	return userID
}

func TestService_BuildSnapshot_SelfSeesActualStateAndLastSeen(t *testing.T) {
	tdb, _ := testdb.SetupDB(t)
	defer tdb.Pool().Close()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	redisRepo := newPresenceRedisTestRepo(t)
	repo := NewDBRepository(nil)
	svc := NewService(appDB, redisRepo, repo, zaptest.NewLogger(t))
	userID := seedPresenceTarget(t, tdb, "self", map[string]any{"show_activity_status": false}, "active", false)
	lastSeen := time.Date(2026, time.July, 29, 18, 30, 0, 0, time.UTC)

	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		return repo.UpsertLastSeen(ctx, tx, userID, lastSeen)
	}))
	require.NoError(t, func() error {
		lease, err := redisRepo.ResumeLease(ctx, userID, "conn-self", time.Now().UTC())
		if err != nil {
			return err
		}
		require.True(t, lease.IsOnline)
		require.True(t, lease.Transitioned)
		return nil
	}())

	states, err := svc.BuildSnapshot(ctx, userID, []uuid.UUID{userID})
	require.NoError(t, err)
	require.Len(t, states, 1)
	require.True(t, states[0].IsOnline)
	require.NotNil(t, states[0].LastSeenAt)
	require.True(t, lastSeen.Equal(*states[0].LastSeenAt))
	require.Equal(t, int64(1), states[0].Version)
}

func TestService_BuildSnapshot_ShowActivityTrueShowsActualState(t *testing.T) {
	tdb, _ := testdb.SetupDB(t)
	defer tdb.Pool().Close()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	redisRepo := newPresenceRedisTestRepo(t)
	repo := NewDBRepository(nil)
	svc := NewService(appDB, redisRepo, repo, zaptest.NewLogger(t))
	viewerID := seedPresenceTarget(t, tdb, "viewer", nil, "active", false)
	targetID := seedPresenceTarget(t, tdb, "target", map[string]any{"show_activity_status": true}, "active", false)
	lastSeen := time.Date(2026, time.July, 29, 19, 0, 0, 0, time.UTC)

	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		return repo.UpsertLastSeen(ctx, tx, targetID, lastSeen)
	}))
	require.NoError(t, func() error {
		_, err := redisRepo.ResumeLease(ctx, targetID, "conn-target", time.Now().UTC())
		return err
	}())

	states, err := svc.BuildSnapshot(ctx, viewerID, []uuid.UUID{targetID})
	require.NoError(t, err)
	require.Len(t, states, 1)
	require.True(t, states[0].IsOnline)
	require.NotNil(t, states[0].LastSeenAt)
	require.True(t, lastSeen.Equal(*states[0].LastSeenAt))
}

func TestService_BuildSnapshot_ShowActivityFalseHidesState(t *testing.T) {
	tdb, _ := testdb.SetupDB(t)
	defer tdb.Pool().Close()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	redisRepo := newPresenceRedisTestRepo(t)
	repo := NewDBRepository(nil)
	svc := NewService(appDB, redisRepo, repo, zaptest.NewLogger(t))
	viewerID := seedPresenceTarget(t, tdb, "viewer", nil, "active", false)
	targetID := seedPresenceTarget(t, tdb, "target", map[string]any{"show_activity_status": false}, "active", false)
	lastSeen := time.Date(2026, time.July, 29, 19, 30, 0, 0, time.UTC)

	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		return repo.UpsertLastSeen(ctx, tx, targetID, lastSeen)
	}))
	require.NoError(t, func() error {
		_, err := redisRepo.ResumeLease(ctx, targetID, "conn-target", time.Now().UTC())
		return err
	}())

	states, err := svc.BuildSnapshot(ctx, viewerID, []uuid.UUID{targetID})
	require.NoError(t, err)
	require.Len(t, states, 1)
	require.False(t, states[0].IsOnline)
	require.Nil(t, states[0].LastSeenAt)
}

func TestService_BuildSnapshot_MissingPrivacyDefaultsToTrue(t *testing.T) {
	tdb, _ := testdb.SetupDB(t)
	defer tdb.Pool().Close()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	redisRepo := newPresenceRedisTestRepo(t)
	repo := NewDBRepository(nil)
	svc := NewService(appDB, redisRepo, repo, zaptest.NewLogger(t))
	viewerID := seedPresenceTarget(t, tdb, "viewer", nil, "active", false)
	targetID := seedPresenceTarget(t, tdb, "target", nil, "active", false)

	require.NoError(t, func() error {
		_, err := redisRepo.ResumeLease(ctx, targetID, "conn-target", time.Now().UTC())
		return err
	}())

	states, err := svc.BuildSnapshot(ctx, viewerID, []uuid.UUID{targetID})
	require.NoError(t, err)
	require.Len(t, states, 1)
	require.True(t, states[0].IsOnline)
}

func TestService_BuildSnapshot_BlockedOrInactiveSubjectsAreHidden(t *testing.T) {
	tdb, _ := testdb.SetupDB(t)
	defer tdb.Pool().Close()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	redisRepo := newPresenceRedisTestRepo(t)
	repo := NewDBRepository(nil)
	svc := NewService(appDB, redisRepo, repo, zaptest.NewLogger(t))
	viewerID := seedPresenceTarget(t, tdb, "viewer", nil, "active", false)
	blockedID := seedPresenceTarget(t, tdb, "blocked", map[string]any{"show_activity_status": true}, "active", false)
	suspendedID := seedPresenceTarget(t, tdb, "suspended", map[string]any{"show_activity_status": true}, "suspended", false)

	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO user_blocks (blocker_id, blocked_id, created_at) VALUES ($1, $2, NOW())`, viewerID, blockedID)
		return err
	}))
	require.NoError(t, func() error {
		_, err := redisRepo.ResumeLease(ctx, blockedID, "conn-blocked", time.Now().UTC())
		if err != nil {
			return err
		}
		_, err = redisRepo.ResumeLease(ctx, suspendedID, "conn-suspended", time.Now().UTC())
		return err
	}())

	states, err := svc.BuildSnapshot(ctx, viewerID, []uuid.UUID{blockedID, suspendedID})
	require.NoError(t, err)
	require.Len(t, states, 2)
	for _, state := range states {
		require.False(t, state.IsOnline)
		require.Nil(t, state.LastSeenAt)
	}
}

func TestService_BuildSnapshot_DedupesAndSortsTargets(t *testing.T) {
	ids := []uuid.UUID{uuid.Nil, uuid.New(), uuid.New(), uuid.New()}
	ids[2] = ids[1]
	deduped := dedupeAndSort(ids)
	require.Len(t, deduped, 2)
	require.True(t, deduped[0].String() <= deduped[1].String())
}

func TestVisibleStateForViewer_CoarsensLifecycle(t *testing.T) {
	viewerID := uuid.New()
	targetID := uuid.New()
	now := time.Now().UTC()
	svc := &Service{}
	state := svc.visibleStateForViewer(viewerID, targetID, subjectSnapshot{
		SubjectFacts: SubjectFacts{
			UserID:             targetID,
			AccountStatus:      "suspended",
			ShowActivityStatus: true,
			LastSeenAt:         &now,
			Version:            9,
			IsOnline:           true,
		},
	})

	require.Equal(t, targetID, state.UserID)
	require.False(t, state.IsOnline)
	require.Nil(t, state.LastSeenAt)
	require.Equal(t, int64(0), state.Version)
}
