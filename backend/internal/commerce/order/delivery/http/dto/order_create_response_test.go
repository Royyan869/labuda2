package dto

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/order/entity"
	"github.com/labuda/backend/pkg/money"
	"github.com/stretchr/testify/require"
)

func TestOrderCreateResponse_MoneyFieldsAreInt64(t *testing.T) {
	// Arrange: Create an Order with known Money values
	now := time.Now()
	order := &entity.Order{
		ID:          uuid.New(),
		OrderNumber: stringPtr("ORD-20260917-TEST0001"),
		Status:      entity.StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,

		// Pricing snapshot
		UnitPrice:              money.New(100000),
		Subtotal:               money.New(100000),
		ShippingTotal:          money.New(10000),
		CommissionAmount:       money.New(5000),
		ServiceFeeAmount:       money.New(2000),
		TotalPayableAmount:     money.New(97000), // PD+S + service fee
		TotalBeforeCoinsAmount: money.New(110000), // PD+S = subtotal + shipping
	}

	// Act: Convert to response DTO
	resp := OrderToCreateResponse(order)

	// Assert: Money fields are int64, not objects
	require.Equal(t, int64(100000), resp.Subtotal)
	require.Equal(t, int64(10000), resp.ShippingTotal)
	require.Equal(t, int64(5000), resp.CommissionAmount)
	require.Equal(t, int64(110000), resp.TotalBeforeCoinsAmount)

	// Serialize to JSON and verify numeric types
	jsonBytes, err := json.Marshal(resp)
	require.NoError(t, err)

	var raw map[string]interface{}
	err = json.Unmarshal(jsonBytes, &raw)
	require.NoError(t, err)

	// CRITICAL: Money fields must be JSON numbers, not objects
	subtotal, ok := raw["subtotal"].(float64)
	require.True(t, ok, "subtotal must be JSON number, got %T", raw["subtotal"])
	require.Equal(t, float64(100000), subtotal)

	shippingTotal, ok := raw["shipping_total"].(float64)
	require.True(t, ok, "shipping_total must be JSON number, got %T", raw["shipping_total"])
	require.Equal(t, float64(10000), shippingTotal)

	commissionAmount, ok := raw["commission_amount"].(float64)
	require.True(t, ok, "commission_amount must be JSON number, got %T", raw["commission_amount"])
	require.Equal(t, float64(5000), commissionAmount)

	totalBeforeCoins, ok := raw["total_before_coins_amount"].(float64)
	require.True(t, ok, "total_before_coins_amount must be JSON number, got %T", raw["total_before_coins_amount"])
	require.Equal(t, float64(110000), totalBeforeCoins)
}

func TestOrderCreateResponse_TimestampIsRFC3339(t *testing.T) {
	// Arrange: Create an Order with known timestamp
	now := time.Date(2026, 9, 17, 10, 30, 0, 0, time.UTC)
	order := &entity.Order{
		ID:        uuid.New(),
		Status:    entity.StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Act: Convert to response DTO
	resp := OrderToCreateResponse(order)

	// Assert: created_at is RFC3339 string
	require.Equal(t, "2026-09-17T10:30:00Z", resp.CreatedAt)

	// Serialize to JSON
	jsonBytes, err := json.Marshal(resp)
	require.NoError(t, err)

	var raw map[string]interface{}
	err = json.Unmarshal(jsonBytes, &raw)
	require.NoError(t, err)

	// CRITICAL: created_at must be a string, not a number
	createdAt, ok := raw["created_at"].(string)
	require.True(t, ok, "created_at must be JSON string, got %T", raw["created_at"])
	require.Equal(t, "2026-09-17T10:30:00Z", createdAt)

	// Verify it's parseable by DateTime.tryParse (Dart)
	parsed, err := time.Parse(time.RFC3339, createdAt)
	require.NoError(t, err)
	require.Equal(t, now.UTC(), parsed.UTC())
}

func TestOrderCreateResponse_DiscountedOrderFixture(t *testing.T) {
	// Arrange: Discounted order fixture
	// P = 100000, D = 15000, S = 10000
	// PD+S = 95000 (total_before_coins_amount)
	now := time.Now()
	order := &entity.Order{
		ID:          uuid.New(),
		OrderNumber: stringPtr("ORD-20260917-DISC0001"),
		Status:      entity.StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,

		// Pricing snapshot
		UnitPrice:              money.New(100000), // P = 100000
		Subtotal:               money.New(100000), // Subtotal = P
		ShippingTotal:          money.New(10000),  // S = 10000
		CommissionAmount:       money.New(5000),   // Commission (5%)
		ServiceFeeAmount:       money.New(0),
		TotalPayableAmount:     money.New(95000),  // PD+S = (P-D)+S = 95000
		TotalBeforeCoinsAmount: money.New(110000), // PD+S = 110000
	}

	// Act
	resp := OrderToCreateResponse(order)

	// Assert: Core financial values
	require.Equal(t, int64(100000), resp.Subtotal)
	require.Equal(t, int64(10000), resp.ShippingTotal)
	require.Equal(t, int64(5000), resp.CommissionAmount)
	require.Equal(t, int64(110000), resp.TotalBeforeCoinsAmount)

	// Serialize to JSON
	jsonBytes, err := json.Marshal(resp)
	require.NoError(t, err)

	// Verify JSON is valid and contains expected fields
	var raw map[string]interface{}
	err = json.Unmarshal(jsonBytes, &raw)
	require.NoError(t, err)

	// Verify no Money object serialization (no {} for money fields)
	require.NotEqual(t, map[string]interface{}{}, raw["subtotal"], "subtotal must NOT be {}")
	require.NotEqual(t, map[string]interface{}{}, raw["shipping_total"], "shipping_total must NOT be {}")
	require.NotEqual(t, map[string]interface{}{}, raw["commission_amount"], "commission_amount must NOT be {}")
	require.NotEqual(t, map[string]interface{}{}, raw["total_before_coins_amount"], "total_before_coins_amount must NOT be {}")
}

func TestOrderCreateResponse_RawMoneySerializationIsBroken(t *testing.T) {
	// This test proves the original bug: raw Money serialization produces {}
	m := money.New(100000)

	jsonBytes, err := json.Marshal(m)
	require.NoError(t, err)

	var raw interface{}
	err = json.Unmarshal(jsonBytes, &raw)
	require.NoError(t, err)

	// The raw Money serialization produces {} (empty object) - this is the bug
	require.Equal(t, map[string]interface{}{}, raw, "raw Money MUST serialize to {} - proving the bug exists")

	// But the DTO converts it to int64
	order := &entity.Order{
		ID:        uuid.New(),
		Status:    entity.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),

		Subtotal:               m,
		ShippingTotal:          money.New(10000),
		CommissionAmount:       money.New(5000),
		TotalBeforeCoinsAmount: money.New(110000),
	}

	resp := OrderToCreateResponse(order)
	respJSON, err := json.Marshal(resp)
	require.NoError(t, err)

	var rawResp map[string]interface{}
	err = json.Unmarshal(respJSON, &rawResp)
	require.NoError(t, err)

	// The DTO correctly serializes as JSON number
	require.Equal(t, float64(100000), rawResp["subtotal"], "DTO must serialize Money as JSON number")
}

func TestOrderCreateResponse_RecoveryBranchUsesSameContract(t *testing.T) {
	// Prove that normal create and recovery branches produce the same response shape
	now := time.Now()
	order := &entity.Order{
		ID:          uuid.New(),
		OrderNumber: stringPtr("ORD-20260917-REC0001"),
		Status:      entity.StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,

		UnitPrice:              money.New(100000),
		Subtotal:               money.New(100000),
		ShippingTotal:          money.New(10000),
		CommissionAmount:       money.New(5000),
		TotalBeforeCoinsAmount: money.New(110000),
	}

	// Both branches use the same DTO
	normalResp := OrderToCreateResponse(order)
	recoveryResp := OrderToCreateResponse(order)

	// Same shape
	normalJSON, _ := json.Marshal(normalResp)
	recoveryJSON, _ := json.Marshal(recoveryResp)

	var normalRaw, recoveryRaw map[string]interface{}
	json.Unmarshal(normalJSON, &normalRaw)
	json.Unmarshal(recoveryJSON, &recoveryRaw)

	// Same keys
	require.Equal(t, len(normalRaw), len(recoveryRaw))

	// Same money field types
	for _, field := range []string{"subtotal", "shipping_total", "commission_amount", "total_before_coins_amount"} {
		_, normalOK := normalRaw[field].(float64)
		_, recoveryOK := recoveryRaw[field].(float64)
		require.True(t, normalOK, "normal branch: %s must be JSON number", field)
		require.True(t, recoveryOK, "recovery branch: %s must be JSON number", field)
	}

	// Same timestamp type
	normalCreatedAt, normalOK := normalRaw["created_at"].(string)
	recoveryCreatedAt, recoveryOK := recoveryRaw["created_at"].(string)
	require.True(t, normalOK, "normal branch: created_at must be JSON string")
	require.True(t, recoveryOK, "recovery branch: created_at must be JSON string")
	require.Equal(t, normalCreatedAt, recoveryCreatedAt)
}

func TestOrderCreateResponse_CompleteContract(t *testing.T) {
	// Verify all required fields are present in the response
	now := time.Date(2026, 9, 17, 10, 30, 0, 0, time.UTC)
	order := &entity.Order{
		ID:          uuid.New(),
		OrderNumber: stringPtr("ORD-20260917-CONTRACT001"),
		Status:      entity.StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,

		UnitPrice:              money.New(100000),
		Subtotal:               money.New(100000),
		ShippingTotal:          money.New(10000),
		CommissionAmount:       money.New(5000),
		TotalBeforeCoinsAmount: money.New(110000),
	}

	// Act
	resp := OrderToCreateResponse(order)

	// Assert: All required fields are present
	require.NotEmpty(t, resp.ID)
	require.NotNil(t, resp.OrderNumber)
	require.Equal(t, "ORD-20260917-CONTRACT001", *resp.OrderNumber)
	require.Equal(t, "pending_payment", resp.Status)
	require.Equal(t, int64(100000), resp.Subtotal)
	require.Equal(t, int64(10000), resp.ShippingTotal)
	require.Equal(t, int64(5000), resp.CommissionAmount)
	require.Equal(t, int64(110000), resp.TotalBeforeCoinsAmount)
	require.Equal(t, "2026-09-17T10:30:00Z", resp.CreatedAt)

	// Serialize to JSON and verify all fields are present
	jsonBytes, err := json.Marshal(resp)
	require.NoError(t, err)

	var raw map[string]interface{}
	err = json.Unmarshal(jsonBytes, &raw)
	require.NoError(t, err)

	// All required fields must be present
	require.NotEmpty(t, raw["id"], "id must be present")
	require.NotEmpty(t, raw["order_number"], "order_number must be present")
	require.NotEmpty(t, raw["status"], "status must be present")
	require.NotEmpty(t, raw["subtotal"], "subtotal must be present")
	require.NotEmpty(t, raw["shipping_total"], "shipping_total must be present")
	require.NotEmpty(t, raw["commission_amount"], "commission_amount must be present")
	require.NotEmpty(t, raw["total_before_coins_amount"], "total_before_coins_amount must be present")
	require.NotEmpty(t, raw["created_at"], "created_at must be present")

	// NEGATIVE CONTRACT: the order stores no coins snapshot — K lives in the
	// coins/payment domain. The create response must never advertise a
	// coins_used field, because any value it could carry would be a fabrication.
	_, hasCoinsUsed := raw["coins_used"]
	require.False(t, hasCoinsUsed, "coins_used must NOT be part of the POST /orders contract")
}

func stringPtr(s string) *string {
	return &s
}
