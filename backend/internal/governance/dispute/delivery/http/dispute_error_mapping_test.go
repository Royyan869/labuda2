package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/governance/dispute/application"
	"github.com/stretchr/testify/require"
)

func TestResolveDisputeErrorMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		err        error
		wantCode   int
		wantStatus string
	}{
		{
			name:       "not found",
			err:        pgx.ErrNoRows,
			wantCode:   http.StatusNotFound,
			wantStatus: "Dispute not found",
		},
		{
			name:       "missing capability",
			err:        application.ErrDisputeResolutionCapabilityRequired,
			wantCode:   http.StatusForbidden,
			wantStatus: "finance.dispute.resolve capability required",
		},
		{
			name:       "invalid state",
			err:        application.ErrDisputeResolveInvalidState,
			wantCode:   http.StatusConflict,
			wantStatus: "Dispute cannot be resolved in current state",
		},
		{
			name:       "closed after completion",
			err:        application.ErrDisputeResolveAfterCompletion,
			wantCode:   http.StatusConflict,
			wantStatus: "Cannot resolve dispute after order completion. Please negotiate directly with the seller outside the app.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			handler := &DisputeHandler{}
			require.True(t, handler.writeResolveDisputeError(c, tc.err))
			require.Equal(t, tc.wantCode, w.Code)
			require.Contains(t, w.Body.String(), tc.wantStatus)
		})
	}
}
