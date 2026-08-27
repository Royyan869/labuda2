// PASS_21B negative guard: for-sale creation takes inline product
// fields only — there is no "attach to existing listing" shape, old or new.
// This locks in the explicit legacy listing_id/listingId rejection added in
// PASS_21B, mirroring the equivalent guard already proven for order
// creation and auction creation. The rejected field is intentionally still
// named "listing_id" because it is the legacy wire field under test.
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
)

func TestCreateForSale_RejectsLegacyListingID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewForSaleHandler(nil, nil, nil, nil)

	body := map[string]interface{}{
		"listing_id":  uuid.New().String(),
		"title":       "Showa For Sale",
		"description": "A for-sale item",
		"price":       10000,
		"visibility":  "public",
	}
	payload, err := json.Marshal(body)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, err := http.NewRequest(http.MethodPost, "/api/v1/for-sale", bytes.NewReader(payload))
	require.NoError(t, err)
	c.Request = req
	c.Set("userID", uuid.New())

	handler.CreateForSale(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateForSale_RejectsLegacyListingIdCamelCase(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewForSaleHandler(nil, nil, nil, nil)

	body := map[string]interface{}{
		"listingId":   uuid.New().String(),
		"title":       "Showa For Sale",
		"description": "A for-sale item",
		"price":       10000,
		"visibility":  "public",
	}
	payload, err := json.Marshal(body)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, err := http.NewRequest(http.MethodPost, "/api/v1/for-sale", bytes.NewReader(payload))
	require.NoError(t, err)
	c.Request = req
	c.Set("userID", uuid.New())

	handler.CreateForSale(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
}