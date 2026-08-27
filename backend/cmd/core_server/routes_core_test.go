package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	goredis "github.com/redis/go-redis/v9"

	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/internal/platform/logger"
	"github.com/labuda/backend/pkg/database"
	pkgRedis "github.com/labuda/backend/pkg/redis"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// loadEnvFromParents walks up from the test binary's working directory
// (backend/cmd/core_server) to find backend/.env, mirroring pkg/testdb's
// helper so `go test ./cmd/core_server/...` finds config the same way it
// would from backend/.
func loadEnvFromParents(t *testing.T) {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, ".env")
		if _, statErr := os.Stat(candidate); statErr == nil {
			_ = godotenv.Load(candidate)
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		dir = parent
	}
}

func performReadinessRequest(t *testing.T, db *database.DB, redisClient *pkgRedis.Client) (int, map[string]interface{}) {
	t.Helper()

	router := gin.New()
	router.GET("/health/ready", readinessHandler(&config.Config{}, db, redisClient))

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode readiness response body: %v", err)
	}
	return rec.Code, body
}

// TestReadinessHandler_ReadyWhenDBAndRedisOK proves the happy path still
// reports ready=true/200 once a real Redis ping is performed (previously
// the Redis branch was a no-op that never influenced the result either way).
func TestReadinessHandler_ReadyWhenDBAndRedisOK(t *testing.T) {
	loadEnvFromParents(t)
	cfg, err := config.Load()
	if err != nil {
		t.Skipf("skipping: config load failed: %v", err)
	}
	log, err := logger.NewDevelopment()
	if err != nil {
		t.Fatalf("logger init failed: %v", err)
	}

	db, err := database.NewPostgresDB(&cfg.Database, log)
	if err != nil {
		t.Skipf("skipping: local Postgres unavailable: %v", err)
	}
	defer database.CloseDB(db, log)

	redisClient, err := pkgRedis.NewRedisClient(&cfg.Redis, log)
	if err != nil {
		t.Skipf("skipping: local Redis unavailable: %v", err)
	}
	defer redisClient.Client.Close()

	code, body := performReadinessRequest(t, db, redisClient)
	if code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (body=%v)", code, body)
	}
	if ready, _ := body["ready"].(bool); !ready {
		t.Fatalf("expected ready=true, got body=%v", body)
	}
}

// TestReadinessHandler_NotReadyWhenRedisPingFails proves the fixed readiness
// handler actually fails closed when Redis is unreachable, instead of the
// prior no-op stub that always reported ready regardless of Redis state.
func TestReadinessHandler_NotReadyWhenRedisPingFails(t *testing.T) {
	loadEnvFromParents(t)
	cfg, err := config.Load()
	if err != nil {
		t.Skipf("skipping: config load failed: %v", err)
	}
	log, err := logger.NewDevelopment()
	if err != nil {
		t.Fatalf("logger init failed: %v", err)
	}

	db, err := database.NewPostgresDB(&cfg.Database, log)
	if err != nil {
		t.Skipf("skipping: local Postgres unavailable: %v", err)
	}
	defer database.CloseDB(db, log)

	// Point at a port nothing listens on so Ping fails fast and deterministically.
	unreachable := &pkgRedis.Client{Client: goredis.NewClient(&goredis.Options{
		Addr:        "127.0.0.1:1",
		DialTimeout: 200 * time.Millisecond,
	})}
	defer unreachable.Client.Close()

	code, body := performReadinessRequest(t, db, unreachable)
	if code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when Redis ping fails, got %d (body=%v)", code, body)
	}
	if ready, _ := body["ready"].(bool); ready {
		t.Fatalf("expected ready=false when Redis ping fails, got body=%v", body)
	}
}

// TestReadinessHandler_NilRedisClientSkipsCheck proves the nil-client design
// (Redis not wired at all) is unchanged: readiness is decided by DB alone.
func TestReadinessHandler_NilRedisClientSkipsCheck(t *testing.T) {
	code, body := performReadinessRequest(t, nil, nil)
	if code != http.StatusOK {
		t.Fatalf("expected 200 OK when db and redis are both nil, got %d (body=%v)", code, body)
	}
	if ready, _ := body["ready"].(bool); !ready {
		t.Fatalf("expected ready=true when checks are skipped, got body=%v", body)
	}
}
