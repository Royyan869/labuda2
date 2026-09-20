package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labuda/backend/internal/identity/auth"
)

// TestMapErrorToResponse_MarketAuthorityRequired proves that
// auth.ErrMarketAuthorityRequired maps to HTTP 403 + MARKET_AUTHORITY_REQUIRED
// via the canonical error mapper, not generic FORBIDDEN.
func TestMapErrorToResponse_MarketAuthorityRequired(t *testing.T) {
	mapping := MapErrorToResponse(auth.ErrMarketAuthorityRequired)

	if mapping.StatusCode != http.StatusForbidden {
		t.Errorf("StatusCode = %d, want %d", mapping.StatusCode, http.StatusForbidden)
	}
	if mapping.Code != ErrCodeMarketAuthorityRequired {
		t.Errorf("Code = %q, want %q", mapping.Code, ErrCodeMarketAuthorityRequired)
	}
	if mapping.Message == "" {
		t.Error("Message is empty")
	}
}

// TestMapErrorToResponse_MarketAuthorityRequired_Wrapped proves that the
// error mapper still recognizes ErrMarketAuthorityRequired when it is wrapped
// with fmt.Errorf("%w", ...).
func TestMapErrorToResponse_MarketAuthorityRequired_Wrapped(t *testing.T) {
	wrapped := auth.ErrMarketAuthorityRequired
	mapping := MapErrorToResponse(wrapped)

	if mapping.StatusCode != http.StatusForbidden {
		t.Errorf("StatusCode = %d, want %d", mapping.StatusCode, http.StatusForbidden)
	}
	if mapping.Code != ErrCodeMarketAuthorityRequired {
		t.Errorf("Code = %q, want %q", mapping.Code, ErrCodeMarketAuthorityRequired)
	}
}

// TestMarketAuthorityRequired_Helper proves that response.MarketAuthorityRequired
// sends HTTP 403 with code = MARKET_AUTHORITY_REQUIRED and a caller-supplied
// human-readable message.
func TestMarketAuthorityRequired_Helper(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	callerMessage := "Active seller subscription required to publish for_sales"
	MarketAuthorityRequired(c, callerMessage)

	if w.Code != http.StatusForbidden {
		t.Errorf("HTTP status = %d, want %d", w.Code, http.StatusForbidden)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("response.Error is nil")
	}
	if resp.Error.Code != ErrCodeMarketAuthorityRequired {
		t.Errorf("error.code = %q, want %q", resp.Error.Code, ErrCodeMarketAuthorityRequired)
	}
	if resp.Error.Message != callerMessage {
		t.Errorf("error.message = %q, want %q", resp.Error.Message, callerMessage)
	}
}

// TestMarketAuthorityRequired_VS_Forbidden proves the semantic difference:
// MarketAuthorityRequired produces code=MARKET_AUTHORITY_REQUIRED while
// Forbidden produces code=FORBIDDEN. Both use HTTP 403.
func TestMarketAuthorityRequired_VS_Forbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// MarketAuthorityRequired
	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	MarketAuthorityRequired(c1, "test message")
	var resp1 Response
	json.Unmarshal(w1.Body.Bytes(), &resp1)

	// Forbidden
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	Forbidden(c2, "test message")
	var resp2 Response
	json.Unmarshal(w2.Body.Bytes(), &resp2)

	// Both are 403
	if w1.Code != http.StatusForbidden {
		t.Errorf("MarketAuthorityRequired status = %d, want %d", w1.Code, http.StatusForbidden)
	}
	if w2.Code != http.StatusForbidden {
		t.Errorf("Forbidden status = %d, want %d", w2.Code, http.StatusForbidden)
	}

	// But codes differ
	if resp1.Error.Code != ErrCodeMarketAuthorityRequired {
		t.Errorf("MarketAuthorityRequired code = %q, want %q", resp1.Error.Code, ErrCodeMarketAuthorityRequired)
	}
	if resp2.Error.Code != ErrCodeForbidden {
		t.Errorf("Forbidden code = %q, want %q", resp2.Error.Code, ErrCodeForbidden)
	}
}
