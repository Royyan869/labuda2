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
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"

	"github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/labuda/backend/pkg/db"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ============================================================================
// Test helpers
// ============================================================================

// noopAuditLogger satisfies audit.AdminAuditLogger without any I/O.
type noopAuditLogger struct{}

func (n *noopAuditLogger) Log(_ context.Context, _ uuid.UUID, _, _ string, _ uuid.UUID, _ map[string]interface{}) error {
	return nil
}
func (n *noopAuditLogger) LogSafe(_ context.Context, _ uuid.UUID, _, _ string, _ uuid.UUID, _ map[string]interface{}) {
}
func (n *noopAuditLogger) LogTx(_ context.Context, _ db.Tx, _ uuid.UUID, _, _ string, _ uuid.UUID, _ map[string]interface{}) error {
	return nil
}

// newTestHandler builds a handler with nil db/service — safe for validation-layer tests.
func newTestHandler(t *testing.T) *PlatformConfigHandler {
	t.Helper()
	return &PlatformConfigHandler{
		log:              zaptest.NewLogger(t),
		adminAuditLogger: &noopAuditLogger{},
	}
}

// makeActor creates a minimal actor with the given capabilities.
func makeActor(caps ...string) *capabilityEntity.Actor {
	return &capabilityEntity.Actor{
		ID:           uuid.New(),
		Role:         "admin",
		Capabilities: caps,
	}
}

// makeUpdateCtx wires up a gin context for PUT /admin/config/:key.
func makeUpdateCtx(t *testing.T, key, value string, actor *capabilityEntity.Actor) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body, _ := json.Marshal(map[string]string{"value": value})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/config/"+key, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	if actor != nil {
		req = req.WithContext(capability.WithActor(req.Context(), actor))
	}

	c.Request = req
	c.Params = gin.Params{{Key: "key", Value: key}}

	return c, w
}

// ============================================================================
// Capability tests
// ============================================================================

// TestUpdateConfig_CommissionPercent_RequiresFinancialCap verifies both commission
// keys require financial capability.
func TestUpdateConfig_CommissionPercent_RequiresFinancialCap(t *testing.T) {
	keys := []string{"for_sale_commission_percent", "auction_commission_percent"}
	actor := makeActor(capability.CapConfigUpdateGeneral.String())
	h := newTestHandler(t)

	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			c, w := makeUpdateCtx(t, key, "5", actor)
			h.UpdateConfig(c)
			assert.Equal(t, http.StatusForbidden, w.Code)
		})
	}
}

// ============================================================================
// Value validation tests
// ============================================================================

// TestUpdateConfig_CommissionPercent_TooHigh rejects values > 100.
func TestUpdateConfig_CommissionPercent_TooHigh(t *testing.T) {
	actor := makeActor(capability.CapConfigUpdateFinancial.String())
	h := newTestHandler(t)

	cases := []struct{ key, value string }{
		{"for_sale_commission_percent", "101"},
		{"for_sale_commission_percent", "100.01"},
		{"auction_commission_percent", "200"},
	}
	for _, tc := range cases {
		t.Run(tc.key+"/"+tc.value, func(t *testing.T) {
			c, w := makeUpdateCtx(t, tc.key, tc.value, actor)
			h.UpdateConfig(c)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

// TestUpdateConfig_CommissionPercent_Negative rejects negative values.
func TestUpdateConfig_CommissionPercent_Negative(t *testing.T) {
	actor := makeActor(capability.CapConfigUpdateFinancial.String())
	h := newTestHandler(t)
	c, w := makeUpdateCtx(t, "for_sale_commission_percent", "-1", actor)

	h.UpdateConfig(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestUpdateConfig_CommissionPercent_ValidBoundary accepts 0 and 100.
func TestUpdateConfig_CommissionPercent_ValidBoundary(t *testing.T) {
	actor := makeActor(capability.CapConfigUpdateFinancial.String())
	h := newTestHandler(t)

	for _, v := range []string{"0", "100"} {
		t.Run(v, func(t *testing.T) {
			assert.Panics(t, func() {
				c, _ := makeUpdateCtx(t, "for_sale_commission_percent", v, actor)
				h.UpdateConfig(c)
			}, "expected panic at nil db — value %s passed validation", v)
		})
	}
}

// ============================================================================
// Future-only key rejection tests
// ============================================================================

// TestUpdateConfig_FutureOnlyKeys_Rejected verifies withdrawal keys are rejected
// with 400 regardless of capability (Option A: not currently editable).
func TestUpdateConfig_FutureOnlyKeys_Rejected(t *testing.T) {
	// Even with full financial capability, future-only keys must be rejected.
	actor := makeActor(capability.CapConfigUpdateFinancial.String())
	h := newTestHandler(t)

	keys := []string{"min_withdrawal", "max_withdrawal", "withdrawal_threshold"}
	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			c, w := makeUpdateCtx(t, key, "100000", actor)
			h.UpdateConfig(c)
			assert.Equal(t, http.StatusBadRequest, w.Code, "key %q must be rejected as not editable", key)
		})
	}
}

// ============================================================================
// Unit tests for standalone validator functions
// ============================================================================

func TestValidatePercent(t *testing.T) {
	ok := []string{"0", "5", "50", "100"}
	for _, v := range ok {
		d, _ := decimal.NewFromString(v)
		assert.NoError(t, validatePercent(d), "expected %s to be valid", v)
	}

	bad := []string{"-0.01", "100.01", "200"}
	for _, v := range bad {
		d, _ := decimal.NewFromString(v)
		assert.Error(t, validatePercent(d), "expected %s to be invalid", v)
	}
}

func TestValidatePositiveAmount(t *testing.T) {
	ok := []string{"0.01", "1", "500000"}
	for _, v := range ok {
		d, _ := decimal.NewFromString(v)
		assert.NoError(t, validatePositiveAmount(d), "expected %s to be valid", v)
	}

	bad := []string{"0", "-1", "-0.01"}
	for _, v := range bad {
		d, _ := decimal.NewFromString(v)
		assert.Error(t, validatePositiveAmount(d), "expected %s to be invalid", v)
	}
}

func TestValidatePositiveInt(t *testing.T) {
	ok := []string{"1", "365", "1000"}
	for _, v := range ok {
		d, _ := decimal.NewFromString(v)
		assert.NoError(t, validatePositiveInt(d), "expected %s to be valid", v)
	}

	bad := []string{"0", "-1", "1.5", "365.9"}
	for _, v := range bad {
		d, _ := decimal.NewFromString(v)
		assert.Error(t, validatePositiveInt(d), "expected %s to be invalid", v)
	}
}

// TestNotEditableConfigKeys_CompleteCoverage is a regression lock — any key
// added to notEditableConfigKeys must remain rejected by the handler.
func TestNotEditableConfigKeys_CompleteCoverage(t *testing.T) {
	expected := []string{
		"min_withdrawal",
		"max_withdrawal",
		"withdrawal_threshold",
	}
	for _, key := range expected {
		_, inMap := notEditableConfigKeys[key]
		assert.True(t, inMap, "key %q must be in notEditableConfigKeys", key)
	}
	assert.Len(t, notEditableConfigKeys, len(expected), "notEditableConfigKeys size changed — update this test")
}
