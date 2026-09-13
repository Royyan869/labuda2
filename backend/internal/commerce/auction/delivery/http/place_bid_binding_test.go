// PLACE BID CANONICAL NUMERIC CONVERGENCE: locks PlaceBidRequest's JSON
// binding contract.
//
// The mobile Place Bid chain now emits integer amounts only
// (PlaceBidDto.amount is int; the input boundary rejects fractional input
// explicitly). These tests prove the backend side of that single canonical
// representation:
//
//   - integer JSON amount binds into Go int64;
//   - fractional JSON amount ("amount": 1000000.0) is REJECTED at the
//     binding — no alias accepts the competing double representation;
//   - amount and idempotency_key are required (min=1).
//
// As with create_auction_binding_test.go, full handler construction is
// unnecessary: c.ShouldBindJSON runs before auctionService/db are touched
// in PlaceBid, so exercising the request struct's binding tags in isolation
// is the correct, lighter-weight boundary.
package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func bindPlaceBidRequest(body string) (captured *PlaceBidRequest, statusCode int, responseBody string) {
	router := gin.New()
	router.POST("/auctions/:id/bid", func(c *gin.Context) {
		var req PlaceBidRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		captured = &req
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auctions/11111111-1111-1111-1111-111111111111/bid", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	return captured, w.Code, w.Body.String()
}

func TestPlaceBidRequest_IntegerAmount_BindsIntoInt64(t *testing.T) {
	body := `{
		"amount": 1000000,
		"idempotency_key": "bid-test-0001"
	}`

	captured, code, _ := bindPlaceBidRequest(body)

	require.NotNil(t, captured, "integer amount must pass the binding boundary")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, int64(1000000), captured.Amount)
	assert.Equal(t, "bid-test-0001", captured.IdempotencyKey)
}

func TestPlaceBidRequest_FractionalAmountLiteral_IsRejected(t *testing.T) {
	// "amount": 1000000.0 — a JSON number with a fractional part must NOT
	// bind into int64. This is the competing representation the mobile
	// chain was converged away from; the binding proves there is no alias
	// that still accepts it.
	body := `{
		"amount": 1000000.0,
		"idempotency_key": "bid-test-0002"
	}`

	captured, code, respBody := bindPlaceBidRequest(body)

	assert.Nil(t, captured, "fractional amount must fail binding before the handler ever sees a populated request")
	assert.Equal(t, http.StatusBadRequest, code)
	// The binding error itself is the proof: it names the competing
	// representation (1000000.0) and the canonical type (int64).
	assert.Contains(t, respBody, "cannot unmarshal number 1000000.0")
	assert.Contains(t, respBody, "type int64")
}

func TestPlaceBidRequest_SmallFractionalAmount_IsRejected(t *testing.T) {
	body := `{
		"amount": 1000000.9,
		"idempotency_key": "bid-test-0003"
	}`

	captured, code, _ := bindPlaceBidRequest(body)

	assert.Nil(t, captured)
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestPlaceBidRequest_MissingAmount_Returns400(t *testing.T) {
	body := `{
		"idempotency_key": "bid-test-0004"
	}`

	captured, code, _ := bindPlaceBidRequest(body)

	assert.Nil(t, captured)
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestPlaceBidRequest_MissingIdempotencyKey_Returns400(t *testing.T) {
	body := `{
		"amount": 1000000
	}`

	captured, code, _ := bindPlaceBidRequest(body)

	assert.Nil(t, captured)
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestPlaceBidRequest_JSONMarshalOfCanonicalInt_MatchesWireForm(t *testing.T) {
	// Proves the canonical wire literal produced when a Go int64 amount is
	// serialized: an integer JSON number, no fractional part.
	wire, err := json.Marshal(map[string]int64{"amount": 1000000})
	require.NoError(t, err)
	assert.Equal(t, `{"amount":1000000}`, string(wire))
}
