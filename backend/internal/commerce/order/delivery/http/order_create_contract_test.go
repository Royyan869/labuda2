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
