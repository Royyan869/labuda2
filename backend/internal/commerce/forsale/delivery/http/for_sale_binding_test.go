package http

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func bindCreateForSaleRequest(body string) (captured *CreateForSaleRequest, statusCode int, responseBody string) {
	router := gin.New()
	router.POST("/for_sales", func(c *gin.Context) {
		var req CreateForSaleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		captured = &req
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/for_sales", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	return captured, w.Code, w.Body.String()
}

func TestCreateForSaleRequest_IgnoresTypedMediaPayload(t *testing.T) {
	body := `{
		"title": "Kohaku ForSale",
		"description": "A test for_sale",
		"price": 10000,
		"visibility": "public",
		"media": [
			{"type":"image","url":"https://cdn.example.com/a.jpg"},
			{"type":"video","url":"https://cdn.example.com/b.mp4","duration":12}
		]
	}`

	captured, code, _ := bindCreateForSaleRequest(body)

	require.NotNil(t, captured)
	assert.Equal(t, http.StatusOK, code)
	assert.Empty(t, captured.MediaURLs)
}

func TestCreateForSaleRequest_BindsLegacyMediaURLs(t *testing.T) {
	body := `{
		"title": "Kohaku ForSale",
		"description": "A test for_sale",
		"price": 10000,
		"visibility": "public",
		"media_urls": [
			"https://cdn.example.com/a.jpg"
		]
	}`

	captured, code, _ := bindCreateForSaleRequest(body)

	require.NotNil(t, captured)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, []string{"https://cdn.example.com/a.jpg"}, captured.MediaURLs)
}
