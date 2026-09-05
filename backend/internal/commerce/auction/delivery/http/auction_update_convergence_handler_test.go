package http

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bindUpdateAuctionRequest exercises exactly the UpdateAuction binding guard.
func bindUpdateAuctionRequest(body string) (captured *UpdateAuctionRequest, statusCode int, responseBody string, rejected bool) {
	router := gin.New()
	router.PUT("/auctions/:id", func(c *gin.Context) {
		rawBody, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// replicate guard from AuctionHandler.UpdateAuction
		if bytes.Contains(rawBody, []byte(`"images"`)) ||
			bytes.Contains(rawBody, []byte(`"category"`)) ||
			bytes.Contains(rawBody, []byte(`"condition"`)) ||
			bytes.Contains(rawBody, []byte(`"auto_extend"`)) ||
			bytes.Contains(rawBody, []byte(`"autoExtend"`)) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported field: images/category/condition/auto_extend are not supported for auction update"})
			rejected = true
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(rawBody))
		var req UpdateAuctionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		captured = &req
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/auctions/11111111-1111-1111-1111-111111111111", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	return captured, w.Code, w.Body.String(), rejected
}

func TestUpdateAuctionRequest_TitleAndDescription_Bind(t *testing.T) {
	body := `{"title":"New Title","description":"New desc","start_price":1500000}`
	captured, code, _, rejected := bindUpdateAuctionRequest(body)
	require.False(t, rejected)
	require.NotNil(t, captured)
	assert.Equal(t, http.StatusOK, code)
	require.NotNil(t, captured.Title)
	assert.Equal(t, "New Title", *captured.Title)
	require.NotNil(t, captured.Description)
	assert.Equal(t, "New desc", *captured.Description)
	require.NotNil(t, captured.StartPrice)
	assert.Equal(t, int64(1500000), *captured.StartPrice)
}

func TestUpdateAuctionRequest_RejectsImagesField(t *testing.T) {
	body := `{"title":"New Title","images":["https://cdn.example.com/a.jpg"]}`
	_, code, respBody, rejected := bindUpdateAuctionRequest(body)
	assert.True(t, rejected)
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, respBody, "unsupported field")
}

func TestUpdateAuctionRequest_RejectsCategoryField(t *testing.T) {
	body := `{"category":"Kohaku"}`
	_, code, respBody, rejected := bindUpdateAuctionRequest(body)
	assert.True(t, rejected)
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, respBody, "unsupported field")
}

func TestUpdateAuctionRequest_RejectsConditionField(t *testing.T) {
	body := `{"condition":"new"}`
	_, code, respBody, rejected := bindUpdateAuctionRequest(body)
	assert.True(t, rejected)
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, respBody, "unsupported field")
}

func TestUpdateAuctionRequest_RejectsAutoExtendField(t *testing.T) {
	body := `{"auto_extend":true}`
	_, code, respBody, rejected := bindUpdateAuctionRequest(body)
	assert.True(t, rejected)
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, respBody, "unsupported field")
}

func TestUpdateAuctionRequest_AcceptsTimingFields(t *testing.T) {
	body := `{"start_at":"2026-09-10T10:00:00Z","end_at":"2026-09-11T10:00:00Z"}`
	captured, code, _, rejected := bindUpdateAuctionRequest(body)
	require.False(t, rejected)
	require.NotNil(t, captured)
	assert.Equal(t, http.StatusOK, code)
	require.NotNil(t, captured.StartAt)
	require.NotNil(t, captured.EndAt)
}
