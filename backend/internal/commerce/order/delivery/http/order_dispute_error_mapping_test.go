package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	orderEntity "github.com/labuda/backend/internal/commerce/order/entity"
	"github.com/labuda/backend/internal/governance/dispute/application"
	disputeEntity "github.com/labuda/backend/internal/governance/dispute/entity"
	"github.com/stretchr/testify/require"
)

func TestCreateDisputeErrorMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		err        error
		wantCode   int
		wantStatus string
	}{
		{
			name:       "unauthorized access",
			err:        &orderEntity.ErrUnauthorizedDisputeAccess{OrderID: uuid.New(), UserID: uuid.New(), BuyerID: uuid.New(), SellerID: uuid.New()},
			wantCode:   http.StatusForbidden,
			wantStatus: "You are not authorized to access this dispute",
		},
		{
			name:       "missing reason code",
			err:        &disputeEntity.ErrMissingReasonCode{},
			wantCode:   http.StatusBadRequest,
			wantStatus: "Reason code is required",
		},
		{
			name:       "duplicate dispute",
			err:        application.ErrDisputeOpenAlreadyHasActive,
			wantCode:   http.StatusConflict,
			wantStatus: "Order already has an active dispute",
		},
		{
			name:       "closed dispute",
			err:        application.ErrDisputeOpenAfterCompletion,
			wantCode:   http.StatusConflict,
			wantStatus: "Cannot open dispute after order completion. Please negotiate directly with the seller outside the app.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			handler := &OrderHandler{}
			require.True(t, handler.writeCreateDisputeError(c, tc.err))
			require.Equal(t, tc.wantCode, w.Code)
			require.Contains(t, w.Body.String(), tc.wantStatus)
		})
	}
}
