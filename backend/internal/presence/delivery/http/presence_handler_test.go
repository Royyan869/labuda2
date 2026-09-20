package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/presence"
	"github.com/labuda/backend/pkg/db"
	pkgredis "github.com/labuda/backend/pkg/redis"
	"github.com/labuda/backend/pkg/testdb"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func newTestPresenceHandler(t *testing.T) (*Handler, *testdb.TestDB, *pkgredis.Client) {
	t.Helper()
	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)
	client := &pkgredis.Client{Client: goredis.NewClient(&goredis.Options{
		Addr:         "localhost:6379",
		DB:           15,
		DialTimeout:  2 * 1e9,
		ReadTimeout:  2 * 1e9,
		WriteTimeout: 2 * 1e9,
	})}
	require.NoError(t, client.Ping(context.Background()).Err())
	t.Cleanup(func() { client.Client.Close() })
	repo := presence.NewRedisRepository(client, zaptest.NewLogger(t))
	svc := presence.NewService(db.NewFromPool(tdb.Pool()), repo, presence.NewDBRepository(db.NewFromPool(tdb.Pool())), zaptest.NewLogger(t))
	h := NewHandler(svc, zaptest.NewLogger(t))
	return h, tdb, client
}

func seedUserForPresence(t *testing.T, tdb *testdb.TestDB) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		id = uuid.New()
		_, err := tx.Exec(ctx, `INSERT INTO users (id, firebase_uid, email, email_verified_at, phone_verified, account_status, created_at, updated_at) VALUES ($1,$2,$3,NOW(),true,'active',NOW(),NOW())`, id, id.String(), id.String()+"@test.invalid")
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO user_profiles (user_id, username, privacy, created_at, updated_at) VALUES ($1,$2,'{}'::jsonb,NOW(),NOW())`, id, "u"+id.String()[:8])
		return err
	})
	require.NoError(t, err)
	return id
}

func setPresenceOnline(t *testing.T, client *pkgredis.Client, userID uuid.UUID) int64 {
	t.Helper()
	repo := presence.NewRedisRepository(client, zaptest.NewLogger(t))
	res, err := repo.ResumeLease(context.Background(), userID, uuid.NewString(), timeNow())
	require.NoError(t, err)
	return res.Version
}

func timeNow() time.Time { return time.Now().UTC() }

func TestGetPresence_Unauthenticated(t *testing.T) {
	h, _, _ := newTestPresenceHandler(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/users/presence?user_ids="+uuid.NewString(), nil)
	// no userID set
	h.GetPresence(c)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetPresence_InvalidUUID(t *testing.T) {
	h, _, _ := newTestPresenceHandler(t)
	viewer := uuid.New()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/users/presence?user_ids=not-a-uuid", nil)
	c.Set("userID", viewer)
	h.GetPresence(c)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetPresence_NoTargets(t *testing.T) {
	h, _, _ := newTestPresenceHandler(t)
	viewer := uuid.New()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/users/presence", nil)
	c.Set("userID", viewer)
	h.GetPresence(c)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetPresence_SingleTarget(t *testing.T) {
	h, tdb, client := newTestPresenceHandler(t)
	viewer := seedUserForPresence(t, tdb)
	target := seedUserForPresence(t, tdb)
	// make target online
	repo := presence.NewRedisRepository(client, zaptest.NewLogger(t))
	_, err := repo.ResumeLease(context.Background(), target, uuid.NewString(), timeNow())
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/users/presence?user_ids="+target.String(), nil)
	c.Set("userID", viewer)
	h.GetPresence(c)
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	presences, ok := data["presences"].([]any)
	require.True(t, ok)
	require.Len(t, presences, 1)
	item := presences[0].(map[string]any)
	require.Equal(t, target.String(), item["user_id"])
	require.Equal(t, true, item["is_online"])
	require.NotNil(t, item["version"])
}

func TestGetPresence_Batch(t *testing.T) {
	h, tdb, client := newTestPresenceHandler(t)
	viewer := seedUserForPresence(t, tdb)
	t1 := seedUserForPresence(t, tdb)
	t2 := seedUserForPresence(t, tdb)
	t3 := seedUserForPresence(t, tdb)
	repo := presence.NewRedisRepository(client, zaptest.NewLogger(t))
	_, _ = repo.ResumeLease(context.Background(), t1, uuid.NewString(), timeNow())
	_, _ = repo.ResumeLease(context.Background(), t2, uuid.NewString(), timeNow())
	// t3 remains offline

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	ids := t1.String() + "," + t2.String() + "," + t3.String()
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/users/presence?user_ids="+ids, nil)
	c.Set("userID", viewer)
	h.GetPresence(c)
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	presences := data["presences"].([]any)
	require.Len(t, presences, 3)
}

func TestGetPresence_ViewerBlock_Offline(t *testing.T) {
	h, tdb, client := newTestPresenceHandler(t)
	viewer := seedUserForPresence(t, tdb)
	target := seedUserForPresence(t, tdb)
	// target online
	repo := presence.NewRedisRepository(client, zaptest.NewLogger(t))
	_, err := repo.ResumeLease(context.Background(), target, uuid.NewString(), timeNow())
	require.NoError(t, err)
	// viewer blocks target
	ctx := context.Background()
	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO user_blocks (blocker_id, blocked_id, created_at) VALUES ($1,$2,NOW())`, viewer, target)
		return err
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/users/presence?user_ids="+target.String(), nil)
	c.Set("userID", viewer)
	h.GetPresence(c)
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	item := data["presences"].([]any)[0].(map[string]any)
	// block → fail-closed offline, no last_seen
	require.Equal(t, false, item["is_online"])
	require.Nil(t, item["last_seen_at"])
}

func TestGetPresence_UnknownTarget(t *testing.T) {
	h, tdb, _ := newTestPresenceHandler(t)
	viewer := seedUserForPresence(t, tdb)
	unknown := uuid.New()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/users/presence?user_ids="+unknown.String(), nil)
	c.Set("userID", viewer)
	h.GetPresence(c)
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	item := data["presences"].([]any)[0].(map[string]any)
	require.Equal(t, unknown.String(), item["user_id"])
	require.Equal(t, false, item["is_online"])
}

func TestGetPresence_VersionContinuity(t *testing.T) {
	h, tdb, client := newTestPresenceHandler(t)
	viewer := seedUserForPresence(t, tdb)
	target := seedUserForPresence(t, tdb)
	repo := presence.NewRedisRepository(client, zaptest.NewLogger(t))
	// Create online state with known connID
	cid := uuid.NewString()
	res1, err := repo.ResumeLease(context.Background(), target, cid, timeNow())
	require.NoError(t, err)
	v1 := res1.Version
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/users/presence?user_ids="+target.String(), nil)
	c.Set("userID", viewer)
	h.GetPresence(c)
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	initialVersion := resp["data"].(map[string]any)["presences"].([]any)[0].(map[string]any)["version"].(float64)
	require.Equal(t, float64(v1), initialVersion)

	// Transition offline via same connID → version should bump to v1+1, same authority
	leaveRes, err := repo.LeaveLease(context.Background(), target, cid, timeNow())
	require.NoError(t, err)
	require.True(t, leaveRes.Transitioned)
	require.Equal(t, v1+1, leaveRes.Version)

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/api/v1/users/presence?user_ids="+target.String(), nil)
	c2.Set("userID", viewer)
	h.GetPresence(c2)
	require.Equal(t, http.StatusOK, w2.Code)
	var resp2 map[string]any
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp2))
	offVersion := resp2["data"].(map[string]any)["presences"].([]any)[0].(map[string]any)["version"].(float64)
	require.Equal(t, float64(leaveRes.Version), offVersion)
	// Prove initial and realtime (leaveRes) share same version counter
	require.Equal(t, initialVersion+1, offVersion)
}

func TestGetPresence_BatchLimit(t *testing.T) {
	h, _, _ := newTestPresenceHandler(t)
	viewer := uuid.New()
	ids := make([]string, 101)
	for i := range ids {
		ids[i] = uuid.NewString()
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/users/presence?user_ids="+join(ids, ","), nil)
	c.Set("userID", viewer)
	h.GetPresence(c)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func join(arr []string, sep string) string {
	out := ""
	for i, s := range arr {
		if i > 0 {
			out += sep
		}
		out += s
	}
	return out
}
