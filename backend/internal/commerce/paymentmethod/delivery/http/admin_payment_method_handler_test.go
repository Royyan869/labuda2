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
	"go.uber.org/zap/zaptest"

	"github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/labuda/backend/pkg/db"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ============================================================================
// Test helpers (mirrors platform_config_handler_test.go conventions)
// ============================================================================

type noopAuditLogger struct{}

func (n *noopAuditLogger) Log(_ context.Context, _ uuid.UUID, _, _ string, _ uuid.UUID, _ map[string]interface{}) error {
	return nil
}
func (n *noopAuditLogger) LogSafe(_ context.Context, _ uuid.UUID, _, _ string, _ uuid.UUID, _ map[string]interface{}) {
}
func (n *noopAuditLogger) LogTx(_ context.Context, _ db.Tx, _ uuid.UUID, _, _ string, _ uuid.UUID, _ map[string]interface{}) error {
	return nil
}

// newTestHandler builds a handler with a nil db/repo — safe for
// validation-layer and capability-layer tests, and for PreviewFee (which
// never touches the DB at all).
func newTestHandler(t *testing.T) *AdminPaymentMethodHandler {
	t.Helper()
	return &AdminPaymentMethodHandler{
		log:              zaptest.NewLogger(t),
		adminAuditLogger: &noopAuditLogger{},
	}
}

func makeActor(caps ...string) *capabilityEntity.Actor {
	return &capabilityEntity.Actor{
		ID:           uuid.New(),
		Role:         "admin",
		Capabilities: caps,
	}
}

func makeCtx(t *testing.T, method, path string, code string, body interface{}, actor *capabilityEntity.Actor) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	var reader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if actor != nil {
		req = req.WithContext(capability.WithActor(req.Context(), actor))
	}

	c.Request = req
	if code != "" {
		c.Params = gin.Params{{Key: "code", Value: code}}
	}

	return c, w
}

func validUpdateBody() map[string]interface{} {
	return map[string]interface{}{
		"display_name":       "Transfer Bank",
		"enabled":            true,
		"fee_type":           "flat",
		"flat_amount_rupiah": 4000,
		"percent_bps":        0,
		"midtrans_channels":  []string{"bca_va"},
		"sort_order":         10,
		"rate_source":        "public_baseline",
	}
}

// ============================================================================
// Capability gate tests
// ============================================================================

func TestListMethods_NoActor_Unauthorized(t *testing.T) {
	h := newTestHandler(t)
	c, w := makeCtx(t, http.MethodGet, "/api/v1/admin/payment-methods", "", nil, nil)
	h.ListMethods(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListMethods_WrongCapability_Forbidden(t *testing.T) {
	h := newTestHandler(t)
	actor := makeActor("governance.dashboard.view")
	c, w := makeCtx(t, http.MethodGet, "/api/v1/admin/payment-methods", "", nil, actor)
	h.ListMethods(c)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetMethod_WrongCapability_Forbidden(t *testing.T) {
	h := newTestHandler(t)
	actor := makeActor("governance.dashboard.view")
	c, w := makeCtx(t, http.MethodGet, "/api/v1/admin/payment-methods/qris", "qris", nil, actor)
	h.GetMethod(c)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUpdateMethod_ViewOnlyCapability_Forbidden(t *testing.T) {
	h := newTestHandler(t)
	actor := makeActor(capability.CapFinancePaymentMethodView.String())
	c, w := makeCtx(t, http.MethodPut, "/api/v1/admin/payment-methods/bank_transfer", "bank_transfer", validUpdateBody(), actor)
	h.UpdateMethod(c)
	assert.Equal(t, http.StatusForbidden, w.Code, "view capability alone must not allow mutation")
}

func TestPreviewFee_WrongCapability_Forbidden(t *testing.T) {
	h := newTestHandler(t)
	actor := makeActor("governance.dashboard.view")
	body := map[string]interface{}{"fee_type": "flat", "flat_amount_rupiah": 4000, "base_amount_rupiah": 100000}
	c, w := makeCtx(t, http.MethodPost, "/api/v1/admin/payment-methods/bank_transfer/preview", "bank_transfer", body, actor)
	h.PreviewFee(c)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ============================================================================
// UpdateMethod: validation must reject BEFORE any DB call (nil db is safe)
// ============================================================================

func TestUpdateMethod_InvalidConfig_RejectedBeforeDB(t *testing.T) {
	actor := makeActor(capability.CapFinancePaymentMethodManage.String())

	cases := map[string]map[string]interface{}{
		"empty display name": {
			"display_name": "  ", "enabled": true, "fee_type": "flat",
			"flat_amount_rupiah": 4000, "midtrans_channels": []string{"bca_va"},
		},
		"unknown fee_type": {
			"display_name": "X", "enabled": true, "fee_type": "bogus",
			"midtrans_channels": []string{"bca_va"},
		},
		"negative flat amount": {
			"display_name": "X", "enabled": true, "fee_type": "flat",
			"flat_amount_rupiah": -1, "midtrans_channels": []string{"bca_va"},
		},
		"negative percent bps": {
			"display_name": "X", "enabled": true, "fee_type": "percent",
			"percent_bps": -1, "midtrans_channels": []string{"bca_va"},
		},
		"percent bps too high": {
			"display_name": "X", "enabled": true, "fee_type": "percent",
			"percent_bps": 5000, "midtrans_channels": []string{"bca_va"},
		},
		"min exceeds max": {
			"display_name": "X", "enabled": true, "fee_type": "flat",
			"flat_amount_rupiah": 4000, "min_fee_rupiah": 5000, "max_fee_rupiah": 4000,
			"midtrans_channels": []string{"bca_va"},
		},
		"enabled with no channels": {
			"display_name": "X", "enabled": true, "fee_type": "flat",
			"flat_amount_rupiah": 4000,
		},
		"unknown midtrans channel": {
			"display_name": "X", "enabled": true, "fee_type": "flat",
			"flat_amount_rupiah": 4000, "midtrans_channels": []string{"totally_fake_channel"},
		},
		"forbidden shopeepay channel": {
			"display_name": "X", "enabled": true, "fee_type": "percent",
			"percent_bps": 150, "midtrans_channels": []string{"shopeepay"}, "rate_source": "public_baseline",
		},
		"forbidden kredivo channel": {
			"display_name": "X", "enabled": true, "fee_type": "flat",
			"flat_amount_rupiah": 0, "midtrans_channels": []string{"kredivo"}, "rate_source": "public_baseline",
		},
		"forbidden akulaku channel": {
			"display_name": "X", "enabled": true, "fee_type": "flat",
			"flat_amount_rupiah": 0, "midtrans_channels": []string{"akulaku"}, "rate_source": "public_baseline",
		},
		"missing rate_source": {
			"display_name": "X", "enabled": true, "fee_type": "flat",
			"flat_amount_rupiah": 4000, "midtrans_channels": []string{"bca_va"},
		},
		"unknown rate_source": {
			"display_name": "X", "enabled": true, "fee_type": "flat",
			"flat_amount_rupiah": 4000, "midtrans_channels": []string{"bca_va"}, "rate_source": "bogus",
		},
		"merchant_verified without note": {
			"display_name": "X", "enabled": true, "fee_type": "flat",
			"flat_amount_rupiah": 4000, "midtrans_channels": []string{"bca_va"}, "rate_source": "merchant_verified",
		},
		// PASS_19C: an enabled method with explicit JSON null (not just an
		// omitted field) for midtrans_channels must be a clean 400, never a
		// DB error — nil map value marshals to JSON `null`, which unmarshals
		// to a nil Go slice, same as omission.
		"enabled with null channels": {
			"display_name": "X", "enabled": true, "fee_type": "flat",
			"flat_amount_rupiah": 4000, "midtrans_channels": nil, "rate_source": "public_baseline",
		},
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			h := newTestHandler(t) // nil db: would panic if the handler reached it
			c, w := makeCtx(t, http.MethodPut, "/api/v1/admin/payment-methods/bank_transfer", "bank_transfer", body, actor)
			h.UpdateMethod(c)
			assert.Equal(t, http.StatusBadRequest, w.Code, "case %q must be rejected with 400", name)
		})
	}
}

// TestUpdateMethod_ValidConfig_ReachesDB proves a well-formed request clears
// every validation gate and reaches the DB layer (nil db => panic here,
// exactly like platform_config_handler_test.go's ValidBoundary case).
func TestUpdateMethod_ValidConfig_ReachesDB(t *testing.T) {
	actor := makeActor(capability.CapFinancePaymentMethodManage.String())
	h := newTestHandler(t)

	assert.Panics(t, func() {
		c, _ := makeCtx(t, http.MethodPut, "/api/v1/admin/payment-methods/bank_transfer", "bank_transfer", validUpdateBody(), actor)
		h.UpdateMethod(c)
	}, "expected panic at nil db — valid body passed validation")
}

// TestUpdateMethod_DisabledWithOmittedChannels_ReachesDB (PASS_19C) proves a
// disabled method with midtrans_channels entirely omitted from the JSON body
// clears validation (matching entity.ValidateConfig's disabled-with-no-
// channels rule) and reaches the DB layer — not rejected with 400, and not a
// 500 either (the nil-vs-NULL fix lives in the repository, past this point).
func TestUpdateMethod_DisabledWithOmittedChannels_ReachesDB(t *testing.T) {
	actor := makeActor(capability.CapFinancePaymentMethodManage.String())
	h := newTestHandler(t)

	body := map[string]interface{}{
		"display_name": "QRIS (disabled)",
		"enabled":      false,
		"fee_type":     "percent",
		"percent_bps":  70,
		"rate_source":  "public_baseline",
		// midtrans_channels intentionally absent.
	}

	assert.Panics(t, func() {
		c, _ := makeCtx(t, http.MethodPut, "/api/v1/admin/payment-methods/qris", "qris", body, actor)
		h.UpdateMethod(c)
	}, "expected panic at nil db — disabled+omitted-channels body passed validation")
}

// TestUpdateMethod_DisabledWithNullChannels_ReachesDB (PASS_19C) is the same
// proof for an explicit JSON `null` (not just an omitted key).
func TestUpdateMethod_DisabledWithNullChannels_ReachesDB(t *testing.T) {
	actor := makeActor(capability.CapFinancePaymentMethodManage.String())
	h := newTestHandler(t)

	body := map[string]interface{}{
		"display_name":      "QRIS (disabled)",
		"enabled":           false,
		"fee_type":          "percent",
		"percent_bps":       70,
		"rate_source":       "public_baseline",
		"midtrans_channels": nil,
	}

	assert.Panics(t, func() {
		c, _ := makeCtx(t, http.MethodPut, "/api/v1/admin/payment-methods/qris", "qris", body, actor)
		h.UpdateMethod(c)
	}, "expected panic at nil db — disabled+null-channels body passed validation")
}

// TestUpdateMethod_MissingCode_BadRequest proves the code path param is required.
func TestUpdateMethod_MissingCode_BadRequest(t *testing.T) {
	actor := makeActor(capability.CapFinancePaymentMethodManage.String())
	h := newTestHandler(t)
	c, w := makeCtx(t, http.MethodPut, "/api/v1/admin/payment-methods/", "", validUpdateBody(), actor)
	h.UpdateMethod(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================================
// PreviewFee: pure computation, fully testable end-to-end (no DB)
// ============================================================================

type previewResponse struct {
	Data struct {
		MethodCode      string `json:"method_code"`
		BaseAmount      int64  `json:"base_amount_rupiah"`
		BuyerPaymentFee int64  `json:"buyer_payment_fee_rupiah"`
		GrossAmount     int64  `json:"gross_amount_rupiah"`
		Clamped         bool   `json:"clamped"`
		Formula         string `json:"formula"`
	} `json:"data"`
}

func decodePreview(t *testing.T, w *httptest.ResponseRecorder) previewResponse {
	t.Helper()
	var resp previewResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

func TestPreviewFee_Flat(t *testing.T) {
	h := newTestHandler(t)
	actor := makeActor(capability.CapFinancePaymentMethodView.String())
	body := map[string]interface{}{
		"fee_type": "flat", "flat_amount_rupiah": 4000, "base_amount_rupiah": 103000,
	}
	c, w := makeCtx(t, http.MethodPost, "/api/v1/admin/payment-methods/bank_transfer/preview", "bank_transfer", body, actor)
	h.PreviewFee(c)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	resp := decodePreview(t, w)
	assert.Equal(t, int64(4000), resp.Data.BuyerPaymentFee)
	assert.Equal(t, int64(107000), resp.Data.GrossAmount)
	assert.False(t, resp.Data.Clamped)
}

func TestPreviewFee_Percent_CeilingRounding(t *testing.T) {
	h := newTestHandler(t)
	actor := makeActor(capability.CapFinancePaymentMethodView.String())
	body := map[string]interface{}{
		"fee_type": "percent", "percent_bps": 70, "base_amount_rupiah": 100001,
	}
	c, w := makeCtx(t, http.MethodPost, "/api/v1/admin/payment-methods/qris/preview", "qris", body, actor)
	h.PreviewFee(c)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	resp := decodePreview(t, w)
	// ceil(100001 * 70 / 10000) = ceil(700.007) = 701 — never truncate.
	assert.Equal(t, int64(701), resp.Data.BuyerPaymentFee)
	assert.Equal(t, int64(100702), resp.Data.GrossAmount)
}

func TestPreviewFee_PercentPlusFlat_WithMaxClamp(t *testing.T) {
	h := newTestHandler(t)
	actor := makeActor(capability.CapFinancePaymentMethodView.String())
	body := map[string]interface{}{
		"fee_type": "percent_plus_flat", "percent_bps": 290, "flat_amount_rupiah": 2000,
		"max_fee_rupiah": 10000, "base_amount_rupiah": 10_000_000,
	}
	c, w := makeCtx(t, http.MethodPost, "/api/v1/admin/payment-methods/credit_card/preview", "credit_card", body, actor)
	h.PreviewFee(c)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	resp := decodePreview(t, w)
	// unclamped = ceil(10_000_000*290/10000) + 2000 = 290000 + 2000 = 292000 -> clamps to 10000
	assert.Equal(t, int64(10000), resp.Data.BuyerPaymentFee)
	assert.True(t, resp.Data.Clamped)
	assert.Equal(t, int64(10_010_000), resp.Data.GrossAmount)
}

func TestPreviewFee_UnknownFeeType_Rejected(t *testing.T) {
	h := newTestHandler(t)
	actor := makeActor(capability.CapFinancePaymentMethodView.String())
	body := map[string]interface{}{"fee_type": "bogus", "base_amount_rupiah": 100000}
	c, w := makeCtx(t, http.MethodPost, "/api/v1/admin/payment-methods/x/preview", "x", body, actor)
	h.PreviewFee(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPreviewFee_MissingBaseAmount_Rejected(t *testing.T) {
	h := newTestHandler(t)
	actor := makeActor(capability.CapFinancePaymentMethodView.String())
	body := map[string]interface{}{"fee_type": "flat", "flat_amount_rupiah": 4000}
	c, w := makeCtx(t, http.MethodPost, "/api/v1/admin/payment-methods/x/preview", "x", body, actor)
	h.PreviewFee(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPreviewFee_NegativeBps_Rejected(t *testing.T) {
	h := newTestHandler(t)
	actor := makeActor(capability.CapFinancePaymentMethodView.String())
	body := map[string]interface{}{"fee_type": "percent", "percent_bps": -5, "base_amount_rupiah": 100000}
	c, w := makeCtx(t, http.MethodPost, "/api/v1/admin/payment-methods/x/preview", "x", body, actor)
	h.PreviewFee(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
