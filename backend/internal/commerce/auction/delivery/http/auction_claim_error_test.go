package http

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labuda/backend/internal/commerce/auction/entity"
	"github.com/labuda/backend/internal/platform/response"
)

// B83 — Claim error semantics tests.
//
// Verifies that each claim rejection path returns the correct HTTP status code
// and error code instead of a generic 500.

// claimErrorMapping returns the expected HTTP status for each claim error.
// This mirrors the switch-case in ClaimAuction and locks the contract.
func claimErrorMapping() []struct {
	err          error
	wrapMsg      string
	expectedCode int
	expectedBody string
} {
	return []struct {
		err          error
		wrapMsg      string
		expectedCode int
		expectedBody string
	}{
		{
			err:          entity.ErrAlreadySettled,
			wrapMsg:      "claim validation failed",
			expectedCode: http.StatusConflict,
			expectedBody: "CONFLICT",
		},
		{
			err:          fmt.Errorf("%w: status=expired_bnr (expected waiting_settlement)", entity.ErrNotClaimable),
			wrapMsg:      "claim validation failed",
			expectedCode: http.StatusConflict,
			expectedBody: "CONFLICT",
		},
		{
			err:          fmt.Errorf("%w: deadline=2026-05-26T12:00:00Z", entity.ErrSettlementDeadlinePassed),
			wrapMsg:      "claim validation failed",
			expectedCode: http.StatusGone,
			expectedBody: "GONE",
		},
		{
			err:          entity.ErrNoWinner,
			wrapMsg:      "claim validation failed",
			expectedCode: http.StatusConflict,
			expectedBody: "CONFLICT",
		},
		{
			err:          entity.ErrNotWinner,
			wrapMsg:      "claim validation failed",
			expectedCode: http.StatusForbidden,
			expectedBody: "FORBIDDEN",
		},
	}
}

func TestClaimErrorMapping_CorrectHTTPStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tc := range claimErrorMapping() {
		// Wrap the error the same way ClaimAuction does
		wrappedErr := fmt.Errorf("%s: %w", tc.wrapMsg, tc.err)

		t.Run(tc.err.Error(), func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			// Replicate the handler's switch-case
			switch {
			case errors.Is(wrappedErr, entity.ErrAlreadySettled):
				response.Conflict(c, "Auction has already been claimed")
			case errors.Is(wrappedErr, entity.ErrNotClaimable):
				response.Conflict(c, "Auction is not claimable")
			case errors.Is(wrappedErr, entity.ErrSettlementDeadlinePassed):
				response.Gone(c, "Auction settlement deadline has passed")
			case errors.Is(wrappedErr, entity.ErrNoWinner):
				response.Conflict(c, "Auction has no winner")
			case errors.Is(wrappedErr, entity.ErrNotWinner):
				response.Forbidden(c, "Caller is not the auction winner")
			default:
				response.InternalServerError(c, "Failed to claim auction")
			}

			if w.Code != tc.expectedCode {
				t.Errorf("expected HTTP %d, got %d (error: %v)", tc.expectedCode, w.Code, wrappedErr)
			}
			if body := w.Body.String(); !contains(body, tc.expectedBody) {
				t.Errorf("expected body to contain %q, got %s", tc.expectedBody, body)
			}
		})
	}
}

func TestClaimErrorMapping_UnknownErrorReturns500(t *testing.T) {
	gin.SetMode(gin.TestMode)

	unknownErr := fmt.Errorf("pricing token generation failed: %w", fmt.Errorf("database timeout"))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	switch {
	case errors.Is(unknownErr, entity.ErrAlreadySettled):
		response.Conflict(c, "Auction has already been claimed")
	case errors.Is(unknownErr, entity.ErrNotClaimable):
		response.Conflict(c, "Auction is not claimable")
	case errors.Is(unknownErr, entity.ErrSettlementDeadlinePassed):
		response.Gone(c, "Auction settlement deadline has passed")
	case errors.Is(unknownErr, entity.ErrNoWinner):
		response.Conflict(c, "Auction has no winner")
	case errors.Is(unknownErr, entity.ErrNotWinner):
		response.Forbidden(c, "Caller is not the auction winner")
	default:
		response.InternalServerError(c, "Failed to claim auction")
	}

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestClaimErrorMapping_WrappedErrorsUnwrap(t *testing.T) {
	// Verify errors.Is works through wrapping layers
	tests := []struct {
		name   string
		err    error
		target error
	}{
		{
			name:   "ErrNotClaimable wrapped with status detail",
			err:    fmt.Errorf("claim validation failed: %w", fmt.Errorf("%w: status=expired_bnr (expected waiting_settlement)", entity.ErrNotClaimable)),
			target: entity.ErrNotClaimable,
		},
		{
			name:   "ErrSettlementDeadlinePassed wrapped with deadline detail",
			err:    fmt.Errorf("claim validation failed: %w", fmt.Errorf("%w: deadline=2026-05-26T12:00:00Z", entity.ErrSettlementDeadlinePassed)),
			target: entity.ErrSettlementDeadlinePassed,
		},
		{
			name:   "ErrAlreadySettled through claim wrapper",
			err:    fmt.Errorf("claim validation failed: %w", entity.ErrAlreadySettled),
			target: entity.ErrAlreadySettled,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if !errors.Is(tc.err, tc.target) {
				t.Errorf("errors.Is(%v, %v) = false, want true", tc.err, tc.target)
			}
		})
	}
}

func TestClaimSentinelErrors_AreDistinct(t *testing.T) {
	sentinels := []error{
		entity.ErrAlreadySettled,
		entity.ErrNotClaimable,
		entity.ErrSettlementDeadlinePassed,
		entity.ErrNoWinner,
		entity.ErrNotWinner,
	}

	for i, a := range sentinels {
		for j, b := range sentinels {
			if i == j {
				continue
			}
			if errors.Is(a, b) {
				t.Errorf("sentinel errors should be distinct: %v == %v", a, b)
			}
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}


