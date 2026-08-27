package http

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labuda/backend/internal/platform/response"
	promotionApp "github.com/labuda/backend/internal/pricing/promotion/application"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandlePromotionError_ExternalProductReviewRequired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	handled := handlePromotionError(ctx, &promotionApp.ExternalProductPromotionReviewRequiredError{})

	require.True(t, handled)
	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), response.ErrCodeForbidden)
}

func TestHandlePromotionError_NonPromotionErrorReturnsFalse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	handled := handlePromotionError(ctx, errors.New("boom"))

	require.False(t, handled)
	assert.Equal(t, http.StatusOK, recorder.Code)
}
