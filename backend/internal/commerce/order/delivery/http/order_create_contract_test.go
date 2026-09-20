package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestCreateOrder_RejectsLegacyListingID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewOrderHandler(nil, nil, nil, nil, nil, nil, nil, zap.NewNop())

	body := map[string]string{
		"listing_id":         uuid.New().String(),
		"product_id":         uuid.New().String(),
		"source_type":        "for_sale",
		"source_id":          uuid.New().String(),
		"pricing_token":      uuid.New().String(),
		"address_id":         uuid.New().String(),
		"shipping_option_id": uuid.New().String(),
	}
	payload, err := json.Marshal(body)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, err := http.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(payload))
	require.NoError(t, err)
	c.Request = req
	c.Set("userID", uuid.New())

	handler.CreateOrder(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateOrderRequest_BindsCanonicalFixedPriceShape(t *testing.T) {
	raw := []byte(`{
		"product_id":"` + uuid.New().String() + `",
		"source_type":"for_sale",
		"source_id":"` + uuid.New().String() + `",
		"pricing_token":"` + uuid.New().String() + `",
		"quantity":1,
		"address_id":"` + uuid.New().String() + `",
		"shipping_option_id":"` + uuid.New().String() + `"
	}`)

	var req CreateOrderRequest
	require.NoError(t, json.Unmarshal(raw, &req))
	require.NotEmpty(t, req.ProductID)
	require.Equal(t, "for_sale", req.SourceType)
	require.NotEmpty(t, req.SourceID)
	require.NotEmpty(t, req.PricingToken)
}

func TestCreateOrderRequest_BindsCanonicalAuctionShape(t *testing.T) {
	raw := []byte(`{
		"product_id":"` + uuid.New().String() + `",
		"source_type":"auction",
		"source_id":"` + uuid.New().String() + `",
		"pricing_token":"` + uuid.New().String() + `",
		"quantity":1,
		"address_id":"` + uuid.New().String() + `",
		"shipping_option_id":"` + uuid.New().String() + `"
	}`)

	var req CreateOrderRequest
	require.NoError(t, json.Unmarshal(raw, &req))
	require.NotEmpty(t, req.ProductID)
	require.Equal(t, "auction", req.SourceType)
	require.NotEmpty(t, req.SourceID)
	require.NotEmpty(t, req.PricingToken)
}

// TestCreateOrderRequest_BindsCoinIntentAsUseCoins pins the ONE order-time coin
// intent representation. The client expresses intent with the boolean
// `use_coins`; it never ships a coin count, because the backend resolves K
// (min(live balance, token.MaxCoinsAllowed)) and persists it on the pricing
// token. The legacy client-authored `coins_to_use` must not bind here.
func TestCreateOrderRequest_BindsCoinIntentAsUseCoins(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want bool
	}{
		{
			name: "absent_intent_defaults_off",
			raw:  `{"pricing_token":"` + uuid.New().String() + `"}`,
			want: false,
		},
		{
			name: "explicit_true",
			raw:  `{"pricing_token":"` + uuid.New().String() + `","use_coins":true}`,
			want: true,
		},
		{
			name: "explicit_false",
			raw:  `{"pricing_token":"` + uuid.New().String() + `","use_coins":false}`,
			want: false,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var req CreateOrderRequest
			require.NoError(t, json.Unmarshal([]byte(tc.raw), &req))
			require.Equal(t, tc.want, req.UseCoins)
		})
	}

	// A stale client coin count must never bind into the order request: a
	// payment-time or order-time client K would be a second coin authority.
	for _, legacy := range []string{"coins_to_use", "coins_used", "coins", "coin_amount", "coin_discount"} {
		raw := `{"pricing_token":"` + uuid.New().String() + `","` + legacy + `":500}`
		var req CreateOrderRequest
		require.NoErrorf(t, json.Unmarshal([]byte(raw), &req), "legacy key %q must be ignored, not fatal", legacy)
		require.Falsef(t, req.UseCoins, "legacy key %q must not enable coins", legacy)
	}
}
