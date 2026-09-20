package main

import (
	"context"
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

// fixedPaymentCallbackHealth is the injected payment-callback readiness view
// used by handler tests: it keeps readiness deterministic and offline. The real
// probe (DNS + outbound HTTP) is exercised separately by
// payment_callback_health_test.go and by the runtime proof.
func fixedPaymentCallbackHealth() PaymentCallbackHealth {
	return PaymentCallbackHealth{
		State:           callbackStateReady,
		Degraded:        false,
		Configured:      true,
		Host:            "labuda-dev.example.com",
		Path:            canonicalPaymentWebhookPath,
		RouteMounted:    true,
		DNSResolves:     true,
		OutboundProbe:   callbackOutboundReachable,
		OutboundDetail:  "http_status=404",
		PublicIngress:   notVerifiableFromProcess,
		GatewayDelivery: notVerifiableFromProcess,
	}
}

func performReadinessRequest(t *testing.T, db *database.DB, redisClient *pkgRedis.Client) (int, map[string]interface{}) {
	t.Helper()

	router := gin.New()
	router.GET("/health/ready", readinessHandler(&config.Config{}, db, redisClient,
		func(_ context.Context, _ *config.Config) PaymentCallbackHealth { return fixedPaymentCallbackHealth() }))

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

// --- INFRA-2: payment callback readiness in the canonical readiness surface ---

// TestReadinessHandler_ExposesPaymentCallbackState proves the canonical
// readiness response carries the factual payment-callback view, and that the
// two facts this process cannot establish are labelled unverifiable rather than
// reported as healthy.
func TestReadinessHandler_ExposesPaymentCallbackState(t *testing.T) {
	code, body := performReadinessRequest(t, nil, nil)
	if code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (body=%v)", code, body)
	}

	raw, ok := body["payment_callback"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected a payment_callback object in the readiness body, got %v", body)
	}
	if raw["state"] != callbackStateReady {
		t.Errorf("expected state=%s, got %v", callbackStateReady, raw["state"])
	}
	if raw["public_ingress"] != notVerifiableFromProcess {
		t.Errorf("public_ingress must stay explicitly unverifiable, got %v", raw["public_ingress"])
	}
	if raw["gateway_delivery"] != notVerifiableFromProcess {
		t.Errorf("gateway_delivery must stay explicitly unverifiable, got %v", raw["gateway_delivery"])
	}
}

// TestReadinessHandler_DegradedCallbackFailsReadinessOutsideDevelopment proves
// the fail-closed verdict reaches HTTP: a callback defect is 503 outside
// development, while development stays 200 with degraded=true so local work is
// never blocked.
func TestReadinessHandler_DegradedCallbackFailsReadinessOutsideDevelopment(t *testing.T) {
	degraded := fixedPaymentCallbackHealth()
	degraded.State = callbackStateHostUnresolved
	degraded.Degraded = true
	degraded.DNSResolves = false
	degraded.OutboundProbe = callbackOutboundSkipped
	degraded.Reason = "test: callback host does not resolve"

	render := func(env string, callback PaymentCallbackHealth) (int, map[string]interface{}) {
		t.Helper()
		router := gin.New()
		router.GET("/health/ready", readinessHandler(
			&config.Config{Server: config.ServerConfig{Env: env}}, nil, nil,
			func(_ context.Context, _ *config.Config) PaymentCallbackHealth { return callback }))

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health/ready", nil))

		var body map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to decode readiness response body: %v", err)
		}
		return rec.Code, body
	}

	code, body := render("development", degraded)
	if code != http.StatusOK {
		t.Fatalf("development must stay 200, got %d (body=%v)", code, body)
	}
	if ready, _ := body["ready"].(bool); !ready {
		t.Errorf("development must stay ready=true, got body=%v", body)
	}
	if isDegraded, _ := body["degraded"].(bool); !isDegraded {
		t.Errorf("development must still report degraded=true, got body=%v", body)
	}

	for _, env := range []string{"staging", "production"} {
		code, body := render(env, degraded)
		if code != http.StatusServiceUnavailable {
			t.Errorf("env=%s: expected 503 when the payment callback is undeliverable, got %d (body=%v)", env, code, body)
		}
		if ready, _ := body["ready"].(bool); ready {
			t.Errorf("env=%s: expected ready=false, got body=%v", env, body)
		}
	}

	// A healthy callback view must keep non-development readiness green: this
	// bounds the fail-closed behavior to genuine defects.
	for _, env := range []string{"staging", "production"} {
		code, body := render(env, fixedPaymentCallbackHealth())
		if code != http.StatusOK {
			t.Errorf("env=%s: a healthy callback view must not fail readiness, got %d (body=%v)", env, code, body)
		}
	}
}
