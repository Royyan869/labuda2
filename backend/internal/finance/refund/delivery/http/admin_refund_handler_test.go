// Tests for the admin gateway refund handler (TASK 34 / Phase 2a).
//
// These tests pin down the *handler* invariants — feature flag enforcement
// and bad-input rejection — that don't require a database, Midtrans
// sandbox, or live gateway client. The orchestration / outbox / state
// machine invariants are already covered by the Phase 1 tests in the
// entity and application packages.
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/labuda/backend/internal/audit"
	refundapp "github.com/labuda/backend/internal/finance/refund/application"
	"github.com/labuda/backend/internal/finance/refund/entity"
	"github.com/labuda/backend/pkg/db"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestAdminRefundHandler_FeatureFlagOff_Returns503(t *testing.T) {
	// nil refundService + nil database is fine here: the handler must
	// short-circuit on the flag BEFORE any of those deps are touched.
	h := NewAdminRefundHandler(nil, nil, false, nil, nil)

	router := gin.New()
	router.POST("/admin/refunds/:refund_id/gateway/initiate", h.InitiateGatewayRefund)

	body := mustJSON(t, map[string]any{
		"amount":          10_000,
		"reason":          "test",
		"idempotency_key": "key-1",
	})
	req := httptest.NewRequest(http.MethodPost,
		"/admin/refunds/"+uuid.New().String()+"/gateway/initiate",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code,
		"flag-off must short-circuit before touching refundService/database")
	assert.Contains(t, rec.Body.String(), "FEATURE_DISABLED")
}

func TestAdminRefundHandler_BadRefundID_Returns400(t *testing.T) {
	h := NewAdminRefundHandler(nil, nil, true, nil, nil)

	router := gin.New()
	router.POST("/admin/refunds/:refund_id/gateway/initiate", h.InitiateGatewayRefund)

	body := mustJSON(t, map[string]any{
		"amount":          10_000,
		"reason":          "test",
		"idempotency_key": "key-1",
	})
	req := httptest.NewRequest(http.MethodPost,
		"/admin/refunds/not-a-uuid/gateway/initiate",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid refund_id")
}

func TestAdminRefundHandler_MissingRequiredFields_Returns400(t *testing.T) {
	h := NewAdminRefundHandler(nil, nil, true, nil, nil)

	router := gin.New()
	router.POST("/admin/refunds/:refund_id/gateway/initiate", h.InitiateGatewayRefund)

	cases := []struct {
		name string
		body map[string]any
	}{
		{"missing amount", map[string]any{"reason": "x", "idempotency_key": "k"}},
		{"missing reason", map[string]any{"amount": 100, "idempotency_key": "k"}},
		{"missing idempotency_key", map[string]any{"amount": 100, "reason": "x"}},
		{"zero amount", map[string]any{"amount": 0, "reason": "x", "idempotency_key": "k"}},
		{"negative amount", map[string]any{"amount": -1, "reason": "x", "idempotency_key": "k"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := mustJSON(t, tc.body)
			req := httptest.NewRequest(http.MethodPost,
				"/admin/refunds/"+uuid.New().String()+"/gateway/initiate",
				bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusBadRequest, rec.Code,
				"binding validation must reject %s before touching service", tc.name)
		})
	}
}

func TestAdminRefundHandler_MissingActor_Returns401(t *testing.T) {
	h := NewAdminRefundHandler(nil, nil, true, nil, nil)

	router := gin.New()
	router.POST("/admin/refunds/:refund_id/gateway/initiate", h.InitiateGatewayRefund)

	body := mustJSON(t, map[string]any{
		"amount":          10_000,
		"reason":          "test",
		"idempotency_key": "key-1",
	})
	req := httptest.NewRequest(http.MethodPost,
		"/admin/refunds/"+uuid.New().String()+"/gateway/initiate",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "User not authenticated")
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

// --- fakes for the P5-01 admin audit logging tests below ---
//
// These avoid a real database and a real Midtrans client entirely: the
// interfaces (gatewayRefundInitiator, db.Transactor, audit.AdminAuditLogger)
// exist specifically so the handler's own logic — including the new
// LogSafe wiring — can be exercised without either dependency.

// fakeGatewayRefundInitiator returns a canned InitiateGatewayRefund result.
type fakeGatewayRefundInitiator struct {
	refund *entity.Refund
	err    error
}

func (f *fakeGatewayRefundInitiator) InitiateGatewayRefund(_ context.Context, _ db.Tx, _ refundapp.InitiateGatewayRefundInput) (*entity.Refund, error) {
	return f.refund, f.err
}

// fakeTransactor runs fn against a nil Tx (the fakes above never touch it).
type fakeTransactor struct{}

func (f *fakeTransactor) WithTx(_ context.Context, fn func(db.Tx) error) error {
	return fn(nil)
}

// auditCall records one LogSafe/Log/LogTx invocation for assertions.
type auditCall struct {
	actorID    uuid.UUID
	actionType string
	targetType string
	targetID   uuid.UUID
	metadata   map[string]interface{}
}

// fakeAdminAuditLogger implements audit.AdminAuditLogger in-memory.
type fakeAdminAuditLogger struct {
	calls []auditCall
}

func (f *fakeAdminAuditLogger) Log(_ context.Context, actorID uuid.UUID, actionType, targetType string, targetID uuid.UUID, metadata map[string]interface{}) error {
	f.calls = append(f.calls, auditCall{actorID, actionType, targetType, targetID, metadata})
	return nil
}

func (f *fakeAdminAuditLogger) LogSafe(_ context.Context, actorID uuid.UUID, actionType, targetType string, targetID uuid.UUID, metadata map[string]interface{}) {
	f.calls = append(f.calls, auditCall{actorID, actionType, targetType, targetID, metadata})
}

func (f *fakeAdminAuditLogger) LogTx(_ context.Context, _ db.Tx, actorID uuid.UUID, actionType, targetType string, targetID uuid.UUID, metadata map[string]interface{}) error {
	f.calls = append(f.calls, auditCall{actorID, actionType, targetType, targetID, metadata})
	return nil
}

// withAuthenticatedAdmin injects the "user_id" context value so
// middleware.MustGetUserIDFromContext resolves to adminID, mirroring what
// UserLookupMiddleware would set in production.
func withAuthenticatedAdmin(adminID uuid.UUID, next gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user_id", adminID)
		next(c)
	}
}

func TestAdminRefundHandler_SuccessfulGatewayInitiate_WritesAdminAuditLog(t *testing.T) {
	refundID := uuid.New()
	orderID := uuid.New()
	adminID := uuid.New()
	gwRefundID := "gw-ref-123"

	auditLogger := &fakeAdminAuditLogger{}
	h := &AdminRefundHandler{
		refundService: &fakeGatewayRefundInitiator{
			refund: &entity.Refund{
				ID:              refundID,
				OrderID:         orderID,
				GatewayStatus:   entity.GatewayRefundPending,
				GatewayAttempts: 1,
				GatewayRefundID: &gwRefundID,
			},
		},
		database:         &fakeTransactor{},
		flagEnabled:      true,
		adminAuditLogger: auditLogger,
		log:              zap.NewNop(),
	}

	router := gin.New()
	router.POST("/admin/refunds/:refund_id/gateway/initiate",
		withAuthenticatedAdmin(adminID, h.InitiateGatewayRefund))

	body := mustJSON(t, map[string]any{
		"amount":          10_000,
		"reason":          "buyer_request",
		"idempotency_key": "key-1",
	})
	req := httptest.NewRequest(http.MethodPost,
		"/admin/refunds/"+refundID.String()+"/gateway/initiate",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Len(t, auditLogger.calls, 1,
		"a successful gateway refund initiation must write exactly one admin audit log entry")

	call := auditLogger.calls[0]
	assert.Equal(t, adminID, call.actorID)
	assert.Equal(t, audit.ActionRefundGatewayInitiated, call.actionType)
	assert.Equal(t, audit.TargetTypeRefund, call.targetType)
	assert.Equal(t, refundID, call.targetID)
	assert.Equal(t, orderID.String(), call.metadata["order_id"])
	assert.Equal(t, "buyer_request", call.metadata["reason"])
	assert.Equal(t, "pending", call.metadata["gateway_status"])

	// No gateway secrets/credentials should ever be logged — only business
	// identifiers the admin themselves supplied or the gateway's own
	// (non-secret) refund reference id.
	for key := range call.metadata {
		assert.NotContains(t, []string{"gateway_credentials", "api_key", "server_key", "signature"}, key)
	}
}

func TestAdminRefundHandler_GatewayDeclined_StillWritesAdminAuditLog(t *testing.T) {
	refundID := uuid.New()
	orderID := uuid.New()
	adminID := uuid.New()

	auditLogger := &fakeAdminAuditLogger{}
	h := &AdminRefundHandler{
		refundService: &fakeGatewayRefundInitiator{
			refund: &entity.Refund{
				ID:            refundID,
				OrderID:       orderID,
				GatewayStatus: entity.GatewayRefundFailed,
			},
			err: assert.AnError,
		},
		database:         &fakeTransactor{},
		flagEnabled:      true,
		adminAuditLogger: auditLogger,
		log:              zap.NewNop(),
	}

	router := gin.New()
	router.POST("/admin/refunds/:refund_id/gateway/initiate",
		withAuthenticatedAdmin(adminID, h.InitiateGatewayRefund))

	body := mustJSON(t, map[string]any{
		"amount":          5_000,
		"reason":          "other",
		"idempotency_key": "key-2",
	})
	req := httptest.NewRequest(http.MethodPost,
		"/admin/refunds/"+refundID.String()+"/gateway/initiate",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String(),
		"gateway decline is still a completed orchestration, not a server error")
	require.Len(t, auditLogger.calls, 1,
		"a gateway-declined-but-orchestrated initiation must still be audit logged")
	assert.Equal(t, "failed", auditLogger.calls[0].metadata["gateway_status"])
}


