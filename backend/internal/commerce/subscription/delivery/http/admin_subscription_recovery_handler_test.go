package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ============================================================================
// Stubs
// ============================================================================

type noopAuditLoggerRecovery struct{}

func (n *noopAuditLoggerRecovery) Log(_ context.Context, _ uuid.UUID, _, _ string, _ uuid.UUID, _ map[string]interface{}) error {
	return nil
}
func (n *noopAuditLoggerRecovery) LogSafe(_ context.Context, _ uuid.UUID, _, _ string, _ uuid.UUID, _ map[string]interface{}) {
}
func (n *noopAuditLoggerRecovery) LogTx(_ context.Context, _ db.Tx, _ uuid.UUID, _, _ string, _ uuid.UUID, _ map[string]interface{}) error {
	return nil
}

// stubTx satisfies db.Tx; only QueryRow is used.
type stubTx struct {
	row pgx.Row
}

func (s *stubTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (s *stubTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}
func (s *stubTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return s.row
}
func (s *stubTx) Commit(_ context.Context) error   { return nil }
func (s *stubTx) Rollback(_ context.Context) error { return nil }

// stubRow satisfies pgx.Row.
type stubRow struct {
	vals []interface{}
	err  error
}

func (r *stubRow) Scan(dest ...interface{}) error {
	if r.err != nil {
		return r.err
	}
	for i, d := range dest {
		if i >= len(r.vals) {
			break
		}
		switch v := d.(type) {
		case *uuid.UUID:
			*v = r.vals[i].(uuid.UUID)
		case *string:
			*v = r.vals[i].(string)
		}
	}
	return nil
}

// stubTransactor satisfies db.Transactor; calls fn with the provided tx.
type stubTransactor struct {
	tx db.Tx
}

func (s *stubTransactor) WithTx(_ context.Context, fn func(db.Tx) error) error {
	return fn(s.tx)
}

// errorTransactor satisfies db.Transactor; returns a fixed error.
type errorTransactor struct{ err error }

func (e *errorTransactor) WithTx(_ context.Context, _ func(db.Tx) error) error { return e.err }

// ============================================================================
// Helpers
// ============================================================================

func makeRecoveryActor(caps ...string) *capabilityEntity.Actor {
	return &capabilityEntity.Actor{
		ID:           uuid.New(),
		Role:         "admin",
		Capabilities: caps,
	}
}

func makeRecoveryCtx(
	t *testing.T,
	paymentID string,
	actor *capabilityEntity.Actor,
) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/admin/seller-subscriptions/recover/"+paymentID, nil)
	if actor != nil {
		req = req.WithContext(capability.WithActor(req.Context(), actor))
	}
	c.Request = req
	c.Params = gin.Params{{Key: "payment_id", Value: paymentID}}
	return c, w
}

func buildTestHandler(t *testing.T, transactor db.Transactor) *AdminSubscriptionRecoveryHandler {
	t.Helper()
	return &AdminSubscriptionRecoveryHandler{
		paymentService: nil, // not reached in validation-layer tests
		db:             transactor,
		log:            zaptest.NewLogger(t),
		auditLogger:    &noopAuditLoggerRecovery{},
	}
}

// ============================================================================
// Tests
// ============================================================================

// errMsg extracts body["error"]["message"] from a JSON response.
func errMsg(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	errInfo, ok := body["error"].(map[string]interface{})
	require.True(t, ok, "response body has no 'error' object: %v", body)
	msg, _ := errInfo["message"].(string)
	return msg
}

// TestRecovery_NoCapability verifies that missing capability returns 403.
func TestRecovery_NoCapability(t *testing.T) {
	h := buildTestHandler(t, nil) // db never reached
	actor := makeRecoveryActor()  // no capabilities
	c, w := makeRecoveryCtx(t, uuid.New().String(), actor)

	h.Recover(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, errMsg(t, w), "seller.subscription.recover")
}

// TestRecovery_InvalidUUID verifies that a non-UUID path param returns 400.
func TestRecovery_InvalidUUID(t *testing.T) {
	h := buildTestHandler(t, nil) // db never reached
	actor := makeRecoveryActor(capability.CapSellerSubscriptionRecover.String())
	c, w := makeRecoveryCtx(t, "not-a-uuid", actor)

	h.Recover(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, errMsg(t, w), "payment_id")
}

// TestRecovery_PaymentNotFound verifies that a missing payment returns 404.
func TestRecovery_PaymentNotFound(t *testing.T) {
	transactor := &errorTransactor{err: pgx.ErrNoRows}
	h := buildTestHandler(t, transactor)
	actor := makeRecoveryActor(capability.CapSellerSubscriptionRecover.String())
	c, w := makeRecoveryCtx(t, uuid.New().String(), actor)

	h.Recover(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestRecovery_WrongReferenceType verifies that a non-subscription payment returns 400.
func TestRecovery_WrongReferenceType(t *testing.T) {
	userID := uuid.New()
	row := &stubRow{vals: []interface{}{userID, "settlement", "order"}}
	transactor := &stubTransactor{tx: &stubTx{row: row}}
	h := buildTestHandler(t, transactor)
	actor := makeRecoveryActor(capability.CapSellerSubscriptionRecover.String())
	c, w := makeRecoveryCtx(t, uuid.New().String(), actor)

	h.Recover(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, errMsg(t, w), "reference_type")
}

// TestRecovery_UnsettledPayment verifies that a pending payment returns 400.
func TestRecovery_UnsettledPayment(t *testing.T) {
	userID := uuid.New()
	row := &stubRow{vals: []interface{}{userID, "pending", "subscription"}}
	transactor := &stubTransactor{tx: &stubTx{row: row}}
	h := buildTestHandler(t, transactor)
	actor := makeRecoveryActor(capability.CapSellerSubscriptionRecover.String())
	c, w := makeRecoveryCtx(t, uuid.New().String(), actor)

	h.Recover(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, errMsg(t, w), "status")
}


